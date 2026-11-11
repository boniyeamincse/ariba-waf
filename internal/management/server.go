package management

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Config struct {
	Origin string
	// DevHTTP permits insecure cookies only for a loopback browser origin.
	// The caller must bind to loopback or explicitly isolate a container listener
	// behind a host-loopback-only port publication.
	DevHTTP bool
	// Resolver defaults to the system resolver; injectable for isolated tests.
	Resolver HostResolver
}

type HostResolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type Server struct {
	store         *Store
	origin        string
	cookieName    string
	preauthName   string
	csrfSecret    string
	dummyPassword string
	secure        bool
	work          chan struct{}
	resolver      HostResolver
}

// NewHandler returns the versioned management routes. Health probes are separate.
func NewHandler(store *Store, cfg Config) (http.Handler, error) {
	u, err := url.Parse(cfg.Origin)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, errors.New("origin must be an exact HTTP(S) origin without a path")
	}
	if !cfg.DevHTTP && u.Scheme != "https" {
		return nil, errors.New("HTTP origin requires explicit development mode")
	}
	if cfg.DevHTTP && u.Hostname() != "localhost" {
		ip := net.ParseIP(u.Hostname())
		if ip == nil || !ip.IsLoopback() {
			return nil, errors.New("development cookies require a loopback origin")
		}
	}
	s := &Server{store: store, origin: cfg.Origin, secure: !cfg.DevHTTP, cookieName: "__Host-ariba_session", preauthName: "__Host-ariba_preauth", work: make(chan struct{}, 2)}
	s.resolver = cfg.Resolver
	if s.resolver == nil {
		s.resolver = net.DefaultResolver
	}
	if cfg.DevHTTP {
		s.cookieName = "ariba_dev_session"
		s.preauthName = "ariba_dev_preauth"
	}
	if err := store.db.QueryRow("SELECT value FROM secrets WHERE name = 'csrf'").Scan(&s.csrfSecret); err != nil {
		return nil, err
	}
	s.dummyPassword, err = hashPassword(randomToken())
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/setup/status", s.setupStatus)
	mux.HandleFunc("GET /api/v1/auth/csrf", s.csrf)
	mux.HandleFunc("POST /api/v1/setup/enroll", s.enroll)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/refresh", s.refresh)
	mux.HandleFunc("GET /api/v1/auth/me", s.me)
	mux.HandleFunc("GET /api/v1/auth/permissions", s.permissions)
	mux.HandleFunc("GET /api/v1/auth/activity", s.activity)
	mux.HandleFunc("POST /api/v1/auth/logout", s.logout)
	mux.HandleFunc("POST /api/v1/auth/password", s.changePassword)
	mux.HandleFunc("POST /api/v1/auth/reauthenticate", s.reauthenticate)
	mux.HandleFunc("GET /api/v1/auth/sessions", s.listSessions)
	mux.HandleFunc("POST /api/v1/auth/sessions/revoke-all", s.revokeAllSessions)
	mux.HandleFunc("DELETE /api/v1/auth/sessions/{id}", s.deleteSession)
	mux.HandleFunc("GET /api/v1/users", s.listUsers)
	mux.HandleFunc("GET /api/v1/users/{id}", s.getUser)
	mux.HandleFunc("POST /api/v1/users", s.createUser)
	mux.HandleFunc("PATCH /api/v1/users/{id}", s.patchUser)
	mux.HandleFunc("DELETE /api/v1/users/{id}", s.deleteUser)
	mux.HandleFunc("GET /api/v1/events", s.listEvents)
	mux.HandleFunc("GET /api/v1/metrics/overview", s.metricsOverview)
	mux.HandleFunc("GET /api/v1/audit-logs", s.auditLogs)
	s.registerTrafficRoutes(mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		requestID := randomToken()
		w.Header().Set("X-Request-ID", requestID)
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey{}, requestID))
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if r.Header.Get("Origin") != s.origin {
				failure(w, r, 403, "ORIGIN_NOT_ALLOWED")
				return
			}
			if !s.validCSRF(r) {
				failure(w, r, 403, "CSRF_INVALID")
				return
			}
		}
		// Bound expensive password work and DB waiters even under unauthenticated load.
		select {
		case s.work <- struct{}{}:
			defer func() { <-s.work }()
		default:
			w.Header().Set("Retry-After", "1")
			failure(w, r, 429, "RATE_LIMITED")
			return
		}
		mux.ServeHTTP(w, r)
	}), nil
}

type requestIDKey struct{}

func respond(w http.ResponseWriter, r *http.Request, status int, data any) {
	respondWithMeta(w, r, status, data, nil)
}

func respondWithMeta(w http.ResponseWriter, r *http.Request, status int, data any, extra map[string]any) {
	meta := map[string]any{"requestId": r.Context().Value(requestIDKey{})}
	for key, value := range extra {
		meta[key] = value
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "meta": meta})
}

func failure(w http.ResponseWriter, r *http.Request, status int, code string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": http.StatusText(status)}, "meta": map[string]any{"requestId": r.Context().Value(requestIDKey{})}})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		failure(w, r, 415, "UNSUPPORTED_MEDIA_TYPE")
		return false
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			failure(w, r, 413, "PAYLOAD_TOO_LARGE")
		} else {
			failure(w, r, 400, "INVALID_JSON")
		}
		return false
	}
	// encoding/json otherwise accepts duplicate keys and deep objects.
	if !utf8.Valid(b) {
		failure(w, r, 400, "INVALID_JSON")
		return false
	}
	if err := checkJSON(json.NewDecoder(bytes.NewReader(b)), 0); err != nil {
		failure(w, r, 400, "INVALID_JSON")
		return false
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		code := "INVALID_JSON"
		if strings.HasPrefix(err.Error(), "json: unknown field ") {
			code = "UNKNOWN_FIELD"
		}
		failure(w, r, 400, code)
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		failure(w, r, 400, "INVALID_JSON")
		return false
	}
	return true
}

func checkJSON(d *json.Decoder, depth int) error {
	if depth > 32 {
		return errors.New("JSON nesting limit")
	}
	t, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		if t == nil && depth == 0 {
			return errors.New("null body")
		}
		return nil
	}
	seen := map[string]bool{}
	for d.More() {
		if delim == '{' {
			key, err := d.Token()
			if err != nil {
				return err
			}
			k, ok := key.(string)
			if !ok || seen[k] {
				return errors.New("duplicate or invalid key")
			}
			seen[k] = true
		}
		if err := checkJSON(d, depth+1); err != nil {
			return err
		}
	}
	_, err = d.Token()
	return err
}

func (s *Server) setCookie(w http.ResponseWriter, name, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", Secure: s.secure, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: age})
}

func (s *Server) proof(binding string) string {
	h := hmac.New(sha256.New, []byte(s.csrfSecret))
	h.Write([]byte(binding))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Server) binding(r *http.Request) string {
	if c, err := r.Cookie(s.cookieName); err == nil && len(c.Value) == 43 {
		return "session:" + c.Value
	}
	if c, err := r.Cookie(s.preauthName); err == nil {
		parts := strings.Split(c.Value, ".")
		if len(parts) == 2 && len(parts[1]) == 43 {
			exp, err := strconv.ParseInt(parts[0], 10, 64)
			if err == nil && exp > time.Now().Unix() && exp <= time.Now().Add(10*time.Minute).Unix() {
				return "preauth:" + c.Value
			}
		}
	}
	return ""
}

func (s *Server) validCSRF(r *http.Request) bool {
	binding := s.binding(r)
	return binding != "" && hmac.Equal([]byte(r.Header.Get("X-CSRF-Token")), []byte(s.proof(binding)))
}

func (s *Server) csrf(w http.ResponseWriter, r *http.Request) {
	binding := s.binding(r)
	if binding == "" {
		value := strconv.FormatInt(time.Now().Add(10*time.Minute).Unix(), 10) + "." + randomToken()
		s.setCookie(w, s.preauthName, value, 600)
		binding = "preauth:" + value
	}
	respond(w, r, 200, map[string]string{"csrfToken": s.proof(binding)})
}

type identity struct {
	User       User
	SessionID  string
	ExpiresAt  int64
	RecentAuth int64
}

func (s *Server) authenticate(w http.ResponseWriter, r *http.Request, admin bool) (identity, bool) {
	var i identity
	c, err := r.Cookie(s.cookieName)
	if err != nil || len(c.Value) != 43 {
		failure(w, r, 401, "AUTH_REQUIRED")
		return i, false
	}
	now := time.Now().Unix()
	err = s.store.db.QueryRowContext(r.Context(), `SELECT u.id,u.username,u.display_name,u.role,u.enabled,u.version,s.id,s.expires_at,s.reauthenticated_at
FROM sessions s JOIN users u ON s.user_id=u.id WHERE s.hash=? AND s.expires_at>? AND s.last_seen>? AND u.enabled=1`, tokenHash(c.Value), now, now-1800).
		Scan(&i.User.ID, &i.User.Username, &i.User.DisplayName, &i.User.Role, &i.User.Enabled, &i.User.Version, &i.SessionID, &i.ExpiresAt, &i.RecentAuth)
	if errors.Is(err, sql.ErrNoRows) {
		failure(w, r, 401, "SESSION_EXPIRED")
		return i, false
	}
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return i, false
	}
	if admin && i.User.Role != "admin" {
		failure(w, r, 403, "FORBIDDEN")
		return i, false
	}
	if _, err := s.store.db.ExecContext(r.Context(), "UPDATE sessions SET last_seen=MAX(last_seen,?) WHERE id=?", now, i.SessionID); err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return i, false
	}
	return i, true
}

func validUsername(v string) bool {
	if len(v) < 3 || len(v) > 64 {
		return false
	}
	for _, c := range v {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}
func validRole(v string) bool { return v == "admin" || v == "operator" || v == "viewer" }
