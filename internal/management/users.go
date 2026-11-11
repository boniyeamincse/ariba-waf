package management

import (
	"crypto/hmac"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func etag(u User) string { return fmt.Sprintf(`"%s-v%d"`, u.ID, u.Version) }

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, true); !ok {
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 200 {
			failure(w, r, 400, "INVALID_QUERY")
			return
		}
		limit = n
	}
	cursor := r.URL.Query().Get("cursor")
	if len(cursor) > 256 {
		failure(w, r, 400, "INVALID_QUERY")
		return
	}
	var since int64
	var after string
	if cursor != "" {
		parts := strings.Split(cursor, ".")
		if len(parts) != 2 || !hmac.Equal([]byte(parts[1]), []byte(s.proof("users:"+parts[0]))) {
			failure(w, r, 400, "INVALID_QUERY")
			return
		}
		decoded, err := base64.RawURLEncoding.DecodeString(parts[0])
		fields := strings.Split(string(decoded), ":")
		if err != nil || len(fields) != 2 {
			failure(w, r, 400, "INVALID_QUERY")
			return
		}
		since, err = strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			failure(w, r, 400, "INVALID_QUERY")
			return
		}
		after = fields[1]
	}
	rows, err := s.store.db.QueryContext(r.Context(), "SELECT "+userColumns+",created_at FROM users WHERE created_at>? OR (created_at=? AND id>?) ORDER BY created_at,id LIMIT ?", since, since, after, limit+1)
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	defer rows.Close()
	users := []User{}
	var times []int64
	for rows.Next() {
		var u User
		var created int64
		err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Enabled, &u.Version, &created)
		if err != nil {
			failure(w, r, 503, "STORAGE_UNAVAILABLE")
			return
		}
		users = append(users, u)
		times = append(times, created)
	}
	if rows.Err() != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	var next any
	if len(users) > limit {
		users = users[:limit]
		payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%s", times[limit-1], users[limit-1].ID)))
		next = payload + "." + s.proof("users:"+payload)
	}
	respondWithMeta(w, r, 200, users, map[string]any{"nextCursor": next})
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, true); !ok {
		return
	}
	u, err := scanUser(s.store.db.QueryRowContext(r.Context(), "SELECT "+userColumns+" FROM users WHERE id=?", r.PathValue("id")))
	if errors.Is(err, sql.ErrNoRows) {
		failure(w, r, 404, "NOT_FOUND")
		return
	}
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	w.Header().Set("ETag", etag(u))
	respond(w, r, 200, u)
}

// adminTransaction repeats authorization while holding the write transaction,
// so concurrent demotion/revocation cannot authorize a later account mutation.
func (s *Server) adminTransaction(w http.ResponseWriter, r *http.Request, i identity) (*sql.Tx, bool) {
	return s.roleTransaction(w, r, i, true)
}

func (s *Server) roleTransaction(w http.ResponseWriter, r *http.Request, i identity, adminOnly bool) (*sql.Tx, bool) {
	tx, err := s.store.db.BeginTx(r.Context(), nil)
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return nil, false
	}
	var count int
	now := time.Now().Unix()
	err = tx.QueryRowContext(r.Context(), `SELECT count(*) FROM sessions s JOIN users u ON s.user_id=u.id
WHERE s.id=? AND (u.role='admin' OR (?=0 AND u.role='operator')) AND u.enabled=1 AND s.expires_at>? AND s.last_seen>?`, i.SessionID, adminOnly, now, now-1800).Scan(&count)
	if err != nil {
		tx.Rollback()
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return nil, false
	}
	if count != 1 {
		tx.Rollback()
		failure(w, r, 403, "FORBIDDEN")
		return nil, false
	}
	return tx, true
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	i, ok := s.authenticate(w, r, true)
	if !ok {
		return
	}
	var body struct{ Username, DisplayName, Role, Password string }
	if !decode(w, r, &body) {
		return
	}
	if !validUsername(body.Username) || len(body.DisplayName) < 1 || len(body.DisplayName) > 256 || !validRole(body.Role) {
		failure(w, r, 422, "VALIDATION_FAILED")
		return
	}
	hash, err := hashPassword(body.Password)
	if err != nil {
		failure(w, r, 422, "VALIDATION_FAILED")
		return
	}
	tx, ok := s.adminTransaction(w, r, i)
	if !ok {
		return
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRowContext(r.Context(), "SELECT count(*) FROM users WHERE username=?", body.Username).Scan(&count); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if count > 0 {
		failure(w, r, 409, "USERNAME_CONFLICT")
		return
	}
	u := User{ID: randomToken(), Username: body.Username, DisplayName: body.DisplayName, Role: body.Role, Enabled: true, Version: 1}
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO users(id,username,display_name,role,password_hash) VALUES (?,?,?,?,?)", u.ID, u.Username, u.DisplayName, u.Role, hash); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err = audit(r.Context(), tx, i.User.ID, "users.create", u.ID); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err = tx.Commit(); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	w.Header().Set("Location", "/api/v1/users/"+u.ID)
	w.Header().Set("ETag", etag(u))
	respond(w, r, 201, u)
}

func readUpdateUser(w http.ResponseWriter, r *http.Request, tx *sql.Tx) (User, bool) {
	u, err := scanUser(tx.QueryRowContext(r.Context(), "SELECT "+userColumns+" FROM users WHERE id=?", r.PathValue("id")))
	if errors.Is(err, sql.ErrNoRows) {
		failure(w, r, 404, "NOT_FOUND")
		return u, false
	}
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return u, false
	}
	if r.Header.Get("If-Match") == "" {
		failure(w, r, 428, "PRECONDITION_REQUIRED")
		return u, false
	}
	if r.Header.Get("If-Match") != etag(u) {
		failure(w, r, 412, "VERSION_CONFLICT")
		return u, false
	}
	return u, true
}

func preserveAdmin(w http.ResponseWriter, r *http.Request, tx *sql.Tx, u User) bool {
	if !u.Enabled || u.Role != "admin" {
		return true
	}
	var count int
	if err := tx.QueryRowContext(r.Context(), "SELECT count(*) FROM users WHERE enabled=1 AND role='admin'").Scan(&count); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return false
	}
	if count <= 1 {
		failure(w, r, 409, "LAST_ADMIN")
		return false
	}
	return true
}

func (s *Server) patchUser(w http.ResponseWriter, r *http.Request) {
	i, ok := s.authenticate(w, r, true)
	if !ok {
		return
	}
	// Raw fields distinguish omitted fields from forbidden null values.
	var body map[string]any
	if !decode(w, r, &body) {
		return
	}
	if len(body) == 0 {
		failure(w, r, 422, "VALIDATION_FAILED")
		return
	}
	for k, v := range body {
		switch k {
		case "displayName":
			name, ok := v.(string)
			if !ok || len(name) < 1 || len(name) > 256 {
				failure(w, r, 422, "VALIDATION_FAILED")
				return
			}
		case "role":
			role, ok := v.(string)
			if !ok || !validRole(role) {
				failure(w, r, 422, "VALIDATION_FAILED")
				return
			}
		case "enabled":
			if _, ok := v.(bool); !ok {
				failure(w, r, 422, "VALIDATION_FAILED")
				return
			}
		default:
			failure(w, r, 400, "UNKNOWN_FIELD")
			return
		}
	}
	tx, ok := s.adminTransaction(w, r, i)
	if !ok {
		return
	}
	defer tx.Rollback()
	u, ok := readUpdateUser(w, r, tx)
	if !ok {
		return
	}
	old := u
	if v, ok := body["displayName"]; ok {
		u.DisplayName = v.(string)
	}
	if v, ok := body["role"]; ok {
		u.Role = v.(string)
	}
	if v, ok := body["enabled"]; ok {
		u.Enabled = v.(bool)
	}
	if (u.Role != "admin" || !u.Enabled) && !preserveAdmin(w, r, tx, old) {
		return
	}
	u.Version++
	if _, err := tx.ExecContext(r.Context(), "UPDATE users SET display_name=?,role=?,enabled=?,version=? WHERE id=?", u.DisplayName, u.Role, u.Enabled, u.Version, u.ID); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if u.Role != old.Role || !u.Enabled {
		if _, err := tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE user_id=?", u.ID); err != nil {
			failure(w, r, 503, "STORAGE_UNAVAILABLE")
			return
		}
	}
	if err := audit(r.Context(), tx, i.User.ID, "users.update", u.ID); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err := tx.Commit(); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	w.Header().Set("ETag", etag(u))
	respond(w, r, 200, u)
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	i, ok := s.authenticate(w, r, true)
	if !ok {
		return
	}
	tx, ok := s.adminTransaction(w, r, i)
	if !ok {
		return
	}
	defer tx.Rollback()
	u, ok := readUpdateUser(w, r, tx)
	if !ok || !preserveAdmin(w, r, tx, u) {
		return
	}
	if _, err := tx.ExecContext(r.Context(), "DELETE FROM users WHERE id=?", u.ID); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err := audit(r.Context(), tx, i.User.ID, "users.delete", u.ID); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	if err := tx.Commit(); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	w.WriteHeader(204)
}
