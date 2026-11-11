package management

import (
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

type authActivity struct {
	ID       int64  `json:"id"`
	Action   string `json:"action"`
	TargetID string `json:"targetId,omitempty"`
	At       string `json:"at"`
}

func rolePermissions(role string) []string {
	base := []string{"auth:read", "auth:write", "websites:read", "policies:read", "events:read", "configuration:read"}
	switch role {
	case "admin":
		return append(base,
			"users:read", "users:write",
			"websites:write", "policies:write",
			"settings:read", "settings:write",
			"certificates:read", "certificates:write",
			"configuration:apply", "backup:read", "backup:write",
		)
	case "operator":
		return append(base,
			"websites:write", "policies:write", "configuration:apply", "certificates:read",
		)
	default:
		return base
	}
}

func (s *Server) setupStatus(w http.ResponseWriter, r *http.Request) {
	var count int
	if err := s.store.db.QueryRowContext(r.Context(), "SELECT count(*) FROM users").Scan(&count); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	respond(w, r, 200, map[string]bool{"enrollmentRequired": count == 0})
}

func (s *Server) enroll(w http.ResponseWriter, r *http.Request) {
	var body struct{ EnrollmentToken, Username, DisplayName, Password string }
	if !decode(w, r, &body) {
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if len(key) < 1 || len(key) > 128 {
		failure(w, r, 400, "IDEMPOTENCY_KEY_REQUIRED")
		return
	}
	if !validUsername(body.Username) || len(body.DisplayName) < 1 || len(body.DisplayName) > 256 || len(body.EnrollmentToken) != 43 {
		failure(w, r, 422, "VALIDATION_FAILED")
		return
	}
	tx, err := s.store.db.BeginTx(r.Context(), nil)
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	// Include the high-entropy bootstrap identity; never persist enrollment credentials.
	keyHash := tokenHash("enroll:" + body.EnrollmentToken + ":" + key)
	requestHash := tokenHash(fmt.Sprintf("%q%q%q%q", body.EnrollmentToken, body.Username, body.DisplayName, body.Password))
	var previousHash, userID string
	err = tx.QueryRowContext(r.Context(), "SELECT request_hash,user_id FROM enrollment_retries WHERE key_hash=? AND expires_at>?", keyHash, now).Scan(&previousHash, &userID)
	if err == nil {
		if previousHash != requestHash {
			failure(w, r, 409, "IDEMPOTENCY_CONFLICT")
			return
		}
		u, err := scanUser(tx.QueryRowContext(r.Context(), "SELECT "+userColumns+" FROM users WHERE id=?", userID))
		if err != nil {
			failure(w, r, 503, "STORAGE_UNAVAILABLE")
			return
		}
		w.Header().Set("Location", "/api/v1/users/"+u.ID)
		respond(w, r, 201, u)
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	var count int
	if err := tx.QueryRowContext(r.Context(), "SELECT count(*) FROM users").Scan(&count); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if count > 0 {
		failure(w, r, 409, "SETUP_COMPLETE")
		return
	}
	var expires int64
	err = tx.QueryRowContext(r.Context(), "SELECT expires_at FROM enrollment_tokens WHERE hash=?", tokenHash(body.EnrollmentToken)).Scan(&expires)
	if errors.Is(err, sql.ErrNoRows) || err == nil && expires <= now {
		failure(w, r, 401, "INVALID_CREDENTIALS")
		return
	}
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	passwordHash, err := hashPassword(body.Password)
	if err != nil {
		failure(w, r, 422, "VALIDATION_FAILED")
		return
	}
	u := User{ID: randomToken(), Username: body.Username, DisplayName: body.DisplayName, Role: "admin", Enabled: true, Version: 1}
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO users(id,username,display_name,password_hash,role) VALUES (?,?,?,?,?)", u.ID, u.Username, u.DisplayName, passwordHash, u.Role); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM enrollment_tokens"); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO enrollment_retries VALUES (?,?,?,?)", keyHash, requestHash, u.ID, now+86400); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err = audit(r.Context(), tx, u.ID, "setup.enroll", u.ID); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err = tx.Commit(); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	w.Header().Set("Location", "/api/v1/users/"+u.ID)
	respond(w, r, 201, u)
}

// reserveLogin records a bounded attempt before expensive hashing, so concurrent
// requests cannot race past the limit. Forwarding headers are deliberately ignored.
func (s *Server) reserveLogin(r *http.Request, username string) (int64, error) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	tx, err := s.store.db.BeginTx(r.Context(), nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM login_attempts WHERE at<=?", now-900); err != nil {
		return 0, err
	}
	var global, scoped int
	if err = tx.QueryRowContext(r.Context(), "SELECT count(*),coalesce(sum(CASE WHEN account=? OR ip=? THEN 1 ELSE 0 END),0) FROM login_attempts", strings.ToLower(username), ip).Scan(&global, &scoped); err != nil {
		return 0, err
	}
	if global >= 100 || scoped >= 5 {
		return 0, nil
	}
	result, err := tx.ExecContext(r.Context(), "INSERT INTO login_attempts(account,ip,at) VALUES (?,?,?)", strings.ToLower(username), ip, now)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct{ Username, Password string }
	if !decode(w, r, &body) {
		return
	}
	if len(body.Username) > 64 || len(body.Password) > 512 {
		failure(w, r, 401, "INVALID_CREDENTIALS")
		return
	}
	attempt, err := s.reserveLogin(r, body.Username)
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if attempt == 0 {
		w.Header().Set("Retry-After", "900")
		failure(w, r, 429, "RATE_LIMITED")
		return
	}
	var id, hash string
	var enabled bool
	err = s.store.db.QueryRowContext(r.Context(), "SELECT id,password_hash,enabled FROM users WHERE username=?", body.Username).Scan(&id, &hash, &enabled)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if hash == "" {
		hash = s.dummyPassword
	}
	valid := verifyPassword(hash, body.Password)
	if !valid || id == "" || !enabled {
		failure(w, r, 401, "INVALID_CREDENTIALS")
		return
	}
	tx, err := s.store.db.BeginTx(r.Context(), nil)
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	defer tx.Rollback()
	u, err := scanUser(tx.QueryRowContext(r.Context(), "SELECT "+userColumns+" FROM users WHERE id=? AND enabled=1 AND password_hash=?", id, hash))
	if errors.Is(err, sql.ErrNoRows) {
		failure(w, r, 401, "INVALID_CREDENTIALS")
		return
	}
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM login_attempts WHERE id=?", attempt); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	// A new login invalidates the browser's previous session, preventing fixation.
	if c, err := r.Cookie(s.cookieName); err == nil {
		if _, err = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE hash=?", tokenHash(c.Value)); err != nil {
			failure(w, r, 503, "STORAGE_UNAVAILABLE")
			return
		}
	}
	token, err := s.newSession(r, tx, id, 0, time.Now().Add(12*time.Hour).Unix())
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err = audit(r.Context(), tx, id, "auth.login", id); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err = tx.Commit(); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	s.setCookie(w, s.cookieName, token, 43200)
	s.setCookie(w, s.preauthName, "", -1)
	respond(w, r, 200, u)
}

func (s *Server) newSession(r *http.Request, tx *sql.Tx, userID string, recent, expires int64) (string, error) {
	now := time.Now().Unix()
	if _, err := tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE expires_at<=? OR last_seen<=?", now, now-1800); err != nil {
		return "", err
	}
	// Bound live sessions per user; the oldest is evicted on the eleventh login.
	if _, err := tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE id IN (SELECT id FROM sessions WHERE user_id=? ORDER BY created_at DESC,id DESC LIMIT -1 OFFSET 9)", userID); err != nil {
		return "", err
	}
	token := randomToken()
	_, err := tx.ExecContext(r.Context(), "INSERT INTO sessions(id,hash,user_id,created_at,last_seen,expires_at,reauthenticated_at) VALUES (?,?,?,?,?,?,?)", randomToken(), tokenHash(token), userID, now, now, expires, recent)
	return token, err
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	i, ok := s.authenticate(w, r, false)
	if !ok {
		return
	}
	respond(w, r, 200, map[string]any{"user": i.User, "expiresAt": time.Unix(i.ExpiresAt, 0).UTC().Format(time.RFC3339)})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	i, ok := s.authenticate(w, r, false)
	if !ok {
		return
	}
	tx, err := s.store.db.BeginTx(r.Context(), nil)
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE id=?", i.SessionID); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err = audit(r.Context(), tx, i.User.ID, "auth.logout", i.User.ID); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err = tx.Commit(); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	s.setCookie(w, s.cookieName, "", -1)
	w.WriteHeader(204)
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	i, ok := s.authenticate(w, r, false)
	if !ok {
		return
	}
	tx, err := s.store.db.BeginTx(r.Context(), nil)
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE id=?", i.SessionID); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	token, err := s.newSession(r, tx, i.User.ID, i.RecentAuth, time.Now().Add(12*time.Hour).Unix())
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err = audit(r.Context(), tx, i.User.ID, "auth.refresh", i.User.ID); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err = tx.Commit(); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	s.setCookie(w, s.cookieName, token, 43200)
	respond(w, r, 200, map[string]int{"expiresInSeconds": 43200})
}

func (s *Server) permissions(w http.ResponseWriter, r *http.Request) {
	i, ok := s.authenticate(w, r, false)
	if !ok {
		return
	}
	respond(w, r, 200, map[string]any{
		"role":        i.User.Role,
		"permissions": rolePermissions(i.User.Role),
	})
}

func (s *Server) activity(w http.ResponseWriter, r *http.Request) {
	i, ok := s.authenticate(w, r, false)
	if !ok {
		return
	}
	rows, err := s.store.db.QueryContext(
		r.Context(),
		"SELECT id, action, target_id, at FROM audit_entries WHERE actor_id=? ORDER BY id DESC LIMIT 100",
		i.User.ID,
	)
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	defer rows.Close()
	items := []authActivity{}
	for rows.Next() {
		var row authActivity
		var ts int64
		if err := rows.Scan(&row.ID, &row.Action, &row.TargetID, &ts); err != nil {
			failure(w, r, 503, "STORAGE_UNAVAILABLE")
			return
		}
		row.At = time.Unix(ts, 0).UTC().Format(time.RFC3339)
		items = append(items, row)
	}
	if rows.Err() != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	respond(w, r, 200, items)
}

func (s *Server) credential(w http.ResponseWriter, r *http.Request, i identity, password string) (string, bool) {
	attempt, err := s.reserveLogin(r, i.User.Username)
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return "", false
	}
	if attempt == 0 {
		w.Header().Set("Retry-After", "900")
		failure(w, r, 429, "RATE_LIMITED")
		return "", false
	}
	var hash string
	if err = s.store.db.QueryRowContext(r.Context(), "SELECT password_hash FROM users WHERE id=?", i.User.ID).Scan(&hash); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return "", false
	}
	if !verifyPassword(hash, password) {
		failure(w, r, 401, "INVALID_CREDENTIALS")
		return "", false
	}
	if _, err = s.store.db.ExecContext(r.Context(), "DELETE FROM login_attempts WHERE id=?", attempt); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return "", false
	}
	return hash, true
}

func (s *Server) reauthenticate(w http.ResponseWriter, r *http.Request) {
	i, ok := s.authenticate(w, r, false)
	if !ok {
		return
	}
	var body struct{ Password string }
	if !decode(w, r, &body) {
		return
	}
	hash, ok := s.credential(w, r, i, body.Password)
	if !ok {
		return
	}
	tx, err := s.store.db.BeginTx(r.Context(), nil)
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	defer tx.Rollback()
	if !sessionStillValid(w, r, tx, i, hash) {
		return
	}
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE id=?", i.SessionID); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	token, err := s.newSession(r, tx, i.User.ID, time.Now().Unix(), i.ExpiresAt)
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err = audit(r.Context(), tx, i.User.ID, "auth.reauthenticate", i.User.ID); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err = tx.Commit(); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	s.setCookie(w, s.cookieName, token, max(1, int(i.ExpiresAt-time.Now().Unix())))
	respond(w, r, 200, map[string]int{"recentAuthSeconds": 300})
}

func sessionStillValid(w http.ResponseWriter, r *http.Request, tx *sql.Tx, i identity, hash string) bool {
	var count int
	now := time.Now().Unix()
	err := tx.QueryRowContext(r.Context(), `SELECT count(*) FROM sessions s JOIN users u ON s.user_id=u.id
WHERE s.id=? AND u.enabled=1 AND u.password_hash=? AND s.expires_at>? AND s.last_seen>?`, i.SessionID, hash, now, now-1800).Scan(&count)
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return false
	}
	if count != 1 {
		failure(w, r, 401, "SESSION_EXPIRED")
		return false
	}
	return true
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	i, ok := s.authenticate(w, r, false)
	if !ok {
		return
	}
	var body struct{ CurrentPassword, NewPassword string }
	if !decode(w, r, &body) {
		return
	}
	old, ok := s.credential(w, r, i, body.CurrentPassword)
	if !ok {
		return
	}
	hash, err := hashPassword(body.NewPassword)
	if err != nil {
		failure(w, r, 422, "VALIDATION_FAILED")
		return
	}
	tx, err := s.store.db.BeginTx(r.Context(), nil)
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	defer tx.Rollback()
	if !sessionStillValid(w, r, tx, i, old) {
		return
	}
	if _, err = tx.ExecContext(r.Context(), "UPDATE users SET password_hash=?,version=version+1 WHERE id=?", hash, i.User.ID); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE user_id=?", i.User.ID); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err = audit(r.Context(), tx, i.User.ID, "auth.password", i.User.ID); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err = tx.Commit(); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	s.setCookie(w, s.cookieName, "", -1)
	w.WriteHeader(204)
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	i, ok := s.authenticate(w, r, false)
	if !ok {
		return
	}
	now := time.Now().Unix()
	rows, err := s.store.db.QueryContext(r.Context(), "SELECT id,created_at,last_seen,expires_at FROM sessions WHERE user_id=? AND expires_at>? AND last_seen>? ORDER BY created_at DESC,id DESC", i.User.ID, now, now-1800)
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id string
		var created, seen, expires int64
		if err := rows.Scan(&id, &created, &seen, &expires); err != nil {
			failure(w, r, 503, "STORAGE_UNAVAILABLE")
			return
		}
		items = append(items, map[string]any{"id": id, "current": id == i.SessionID, "createdAt": time.Unix(created, 0).UTC().Format(time.RFC3339), "lastSeenAt": time.Unix(seen, 0).UTC().Format(time.RFC3339), "expiresAt": time.Unix(expires, 0).UTC().Format(time.RFC3339)})
	}
	if rows.Err() != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	respond(w, r, 200, items)
}

func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	i, ok := s.authenticate(w, r, false)
	if !ok {
		return
	}
	tx, err := s.store.db.BeginTx(r.Context(), nil)
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE id=? AND user_id=?", r.PathValue("id"), i.User.ID)
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	n, err := result.RowsAffected()
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if n == 0 {
		failure(w, r, 404, "NOT_FOUND")
		return
	}
	if err := audit(r.Context(), tx, i.User.ID, "auth.sessionRevoke", r.PathValue("id")); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err := tx.Commit(); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if r.PathValue("id") == i.SessionID {
		s.setCookie(w, s.cookieName, "", -1)
	}
	w.WriteHeader(204)
}

func (s *Server) revokeAllSessions(w http.ResponseWriter, r *http.Request) {
	i, ok := s.authenticate(w, r, false)
	if !ok {
		return
	}
	tx, err := s.store.db.BeginTx(r.Context(), nil)
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE user_id=?", i.User.ID); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err := audit(r.Context(), tx, i.User.ID, "auth.revokeAllSessions", i.User.ID); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err = tx.Commit(); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	s.setCookie(w, s.cookieName, "", -1)
	w.WriteHeader(204)
}
