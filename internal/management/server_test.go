package management

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const testOrigin = "http://localhost:3000"
const testPassword = "local test password 123"

type browser struct {
	t       *testing.T
	h       http.Handler
	cookies map[string]*http.Cookie
	csrf    string
}

func newBrowser(t *testing.T, h http.Handler) *browser {
	t.Helper()
	b := &browser{t: t, h: h, cookies: map[string]*http.Cookie{}}
	b.refreshCSRF()
	return b
}

func (b *browser) request(method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	b.t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("Origin", testOrigin)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", b.csrf)
	for _, c := range b.cookies {
		r.AddCookie(c)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	b.h.ServeHTTP(w, r)
	for _, c := range w.Result().Cookies() {
		if c.MaxAge < 0 {
			delete(b.cookies, c.Name)
		} else {
			b.cookies[c.Name] = c
		}
	}
	return w
}

func (b *browser) refreshCSRF() {
	b.t.Helper()
	w := b.request("GET", "/api/v1/auth/csrf", "", nil)
	expect(b.t, w, 200)
	var body struct {
		Data struct {
			CSRFToken string `json:"csrfToken"`
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		b.t.Fatal(err)
	}
	b.csrf = body.Data.CSRFToken
}

func expect(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status=%d, want %d: %s", w.Code, status, w.Body.String())
	}
}

func fixture(t *testing.T) (*Store, http.Handler, *browser, string) {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "data", "ariba.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	h, err := NewHandler(s, Config{Origin: testOrigin, DevHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	token, err := s.CreateEnrollmentToken(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	b := newBrowser(t, h)
	body := fmt.Sprintf(`{"enrollmentToken":%q,"username":"admin","displayName":"Admin","password":%q}`, token, testPassword)
	w := b.request("POST", "/api/v1/setup/enroll", body, map[string]string{"Idempotency-Key": "enroll-1"})
	expect(t, w, 201)
	var result struct{ Data User }
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return s, h, b, result.Data.ID
}

func loginAs(t *testing.T, b *browser, username, password string) {
	t.Helper()
	b.refreshCSRF()
	w := b.request("POST", "/api/v1/auth/login", fmt.Sprintf(`{"username":%q,"password":%q}`, username, password), nil)
	expect(t, w, 200)
	b.refreshCSRF()
}

func TestSetupLoginLogoutAndPersistence(t *testing.T) {
	s, _, b, id := fixture(t)
	expect(t, b.request("GET", "/api/v1/auth/me", "", nil), 401)
	if _, err := s.CreateEnrollmentToken(t.Context()); err == nil {
		t.Fatal("setup token issued after enrollment")
	}
	loginAs(t, b, "ADMIN", testPassword)
	w := b.request("GET", "/api/v1/auth/me", "", nil)
	expect(t, w, 200)
	if !strings.Contains(w.Body.String(), id) {
		t.Fatal("wrong authenticated identity")
	}
	c := b.cookies["ariba_dev_session"]
	if c == nil || !c.HttpOnly || c.Secure || c.SameSite != http.SameSiteStrictMode || len(c.Value) != 43 {
		t.Fatal("incorrect development cookie")
	}
	var hash, passwordHash string
	if err := s.db.QueryRow("SELECT hash FROM sessions").Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if hash == c.Value || hash != tokenHash(c.Value) {
		t.Fatal("raw session token persisted")
	}
	if err := s.db.QueryRow("SELECT password_hash FROM users").Scan(&passwordHash); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(passwordHash, passwordPrefix) || strings.Contains(passwordHash, testPassword) {
		t.Fatal("password not hashed")
	}
	// Closing and reopening checks the migration, secrets and session persistence.
	var dbPath string
	if err := s.db.QueryRow("SELECT file FROM pragma_database_list WHERE name='main'").Scan(&dbPath); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	h, err := NewHandler(reopened, Config{Origin: testOrigin, DevHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	b.h = h
	expect(t, b.request("GET", "/api/v1/auth/me", "", nil), 200)
	old := *c
	expect(t, b.request("POST", "/api/v1/auth/logout", "", nil), 204)
	b.cookies[old.Name] = &old
	expect(t, b.request("GET", "/api/v1/auth/me", "", nil), 401)
}

func TestCSRFOriginAndJSONValidation(t *testing.T) {
	_, _, b, _ := fixture(t)
	body := fmt.Sprintf(`{"username":"admin","password":%q}`, testPassword)
	for _, tc := range []struct {
		name    string
		headers map[string]string
		body    string
		status  int
	}{
		{"missing token", map[string]string{"X-CSRF-Token": ""}, body, 403},
		{"wrong origin", map[string]string{"Origin": "http://evil.example"}, body, 403},
		{"missing origin", map[string]string{"Origin": ""}, body, 403},
		{"wrong token", map[string]string{"X-CSRF-Token": "fake"}, body, 403},
		{"wrong media", map[string]string{"Content-Type": "text/plain"}, body, 415},
		{"unknown field", nil, `{"username":"admin","password":"test","role":"admin"}`, 400},
		{"duplicate field", nil, `{"username":"admin","username":"other"}`, 400},
		{"trailing JSON", nil, body + `{}`, 400},
		{"null JSON", nil, `null`, 400},
		{"oversize", nil, strings.Repeat(" ", 1<<20) + body, 413},
		{"nesting", nil, `{"x":` + strings.Repeat("[", 33) + `0` + strings.Repeat("]", 33) + `}`, 400},
	} {
		t.Run(tc.name, func(t *testing.T) { expect(t, b.request("POST", "/api/v1/auth/login", tc.body, tc.headers), tc.status) })
	}
	preauthCSRF := b.csrf
	loginAs(t, b, "admin", testPassword)
	expect(t, b.request("POST", "/api/v1/auth/logout", "", map[string]string{"X-CSRF-Token": preauthCSRF}), 403)
}

func TestConcurrentEnrollmentAndIdempotency(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "private", "db")
	s, err := Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h, err := NewHandler(s, Config{Origin: testOrigin, DevHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	token, err := s.CreateEnrollmentToken(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// Separate connections model the running API and a concurrent local process.
	s2, err := Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	h2, err := NewHandler(s2, Config{Origin: testOrigin, DevHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	a, b := newBrowser(t, h), newBrowser(t, h2)
	body := fmt.Sprintf(`{"enrollmentToken":%q,"username":"first","displayName":"First","password":%q}`, token, testPassword)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for n, client := range []*browser{a, b} {
		wg.Add(1)
		go func(n int, b *browser) {
			defer wg.Done()
			codes <- b.request("POST", "/api/v1/setup/enroll", body, map[string]string{"Idempotency-Key": fmt.Sprint(n)}).Code
		}(n, client)
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[201] != 1 || counts[409] != 1 {
		t.Fatalf("concurrent enrollment outcomes: %v", counts)
	}
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM users").Scan(&count); err != nil || count != 1 {
		t.Fatalf("user count=%d, err=%v", count, err)
	}
	var winningKey string
	for _, key := range []string{"0", "1"} {
		var n int
		s.db.QueryRow("SELECT count(*) FROM enrollment_retries WHERE key_hash=?", tokenHash("enroll:"+token+":"+key)).Scan(&n)
		if n == 1 {
			winningKey = key
		}
	}
	expect(t, a.request("POST", "/api/v1/setup/enroll", body, map[string]string{"Idempotency-Key": winningKey}), 201)
	expect(t, a.request("POST", "/api/v1/setup/enroll", strings.Replace(body, "First", "Changed", 1), map[string]string{"Idempotency-Key": winningKey}), 409)
}

func TestEnrollmentExpiryAndReplacement(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "private", "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h, err := NewHandler(s, Config{Origin: testOrigin, DevHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	b := newBrowser(t, h)
	first, _ := s.CreateEnrollmentToken(t.Context())
	second, err := s.CreateEnrollmentToken(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{first, second} {
		if token == second {
			if _, err := s.db.Exec("UPDATE enrollment_tokens SET expires_at=?", time.Now().Add(-time.Second).Unix()); err != nil {
				t.Fatal(err)
			}
		}
		body := fmt.Sprintf(`{"enrollmentToken":%q,"username":"admin","displayName":"Admin","password":%q}`, token, testPassword)
		expect(t, b.request("POST", "/api/v1/setup/enroll", body, map[string]string{"Idempotency-Key": token}), 401)
	}
}

func TestRolesVersionChecksAndLastAdmin(t *testing.T) {
	_, h, admin, adminID := fixture(t)
	loginAs(t, admin, "admin", testPassword)
	anonymous := newBrowser(t, h)
	expect(t, anonymous.request("GET", "/api/v1/users", "", nil), 401)
	for _, role := range []string{"viewer", "operator"} {
		w := admin.request("POST", "/api/v1/users", fmt.Sprintf(`{"username":%q,"displayName":%q,"role":%q,"password":%q}`, role, role, role, testPassword), nil)
		expect(t, w, 201)
		var response struct{ Data User }
		json.Unmarshal(w.Body.Bytes(), &response)
		user := newBrowser(t, h)
		loginAs(t, user, role, testPassword)
		expect(t, user.request("GET", "/api/v1/auth/me", "", nil), 200)
		expect(t, user.request("GET", "/api/v1/users", "", nil), 403)
		expect(t, user.request("POST", "/api/v1/users", `{}`, nil), 403)
		adminSession := currentSessionID(t, admin)
		expect(t, user.request("DELETE", "/api/v1/auth/sessions/"+adminSession, "", nil), 404)
		expect(t, user.request("PATCH", "/api/v1/users/"+response.Data.ID, `{"role":"admin"}`, map[string]string{"If-Match": etag(response.Data)}), 403)
		expect(t, admin.request("PATCH", "/api/v1/users/"+response.Data.ID, `{"displayName":"Renamed"}`, nil), 428)
		expect(t, admin.request("PATCH", "/api/v1/users/"+response.Data.ID, `{"displayName":"Renamed"}`, map[string]string{"If-Match": "stale"}), 412)
		newRole := "viewer"
		if role == "viewer" {
			newRole = "operator"
		}
		w = admin.request("PATCH", "/api/v1/users/"+response.Data.ID, fmt.Sprintf(`{"role":%q}`, newRole), map[string]string{"If-Match": etag(response.Data)})
		expect(t, w, 200)
		expect(t, user.request("GET", "/api/v1/auth/me", "", nil), 401)
		loginAs(t, user, role, testPassword)
		w = admin.request("PATCH", "/api/v1/users/"+response.Data.ID, `{"enabled":false}`, map[string]string{"If-Match": w.Header().Get("ETag")})
		expect(t, w, 200)
		expect(t, user.request("GET", "/api/v1/auth/me", "", nil), 401)
		expect(t, admin.request("PATCH", "/api/v1/users/"+response.Data.ID, `{"enabled":null}`, map[string]string{"If-Match": w.Header().Get("ETag")}), 422)
		expect(t, admin.request("DELETE", "/api/v1/users/"+response.Data.ID, "", map[string]string{"If-Match": w.Header().Get("ETag")}), 204)
	}
	w := admin.request("GET", "/api/v1/users/"+adminID, "", nil)
	expect(t, w, 200)
	for _, body := range []string{`{"role":"viewer"}`, `{"enabled":false}`} {
		expect(t, admin.request("PATCH", "/api/v1/users/"+adminID, body, map[string]string{"If-Match": w.Header().Get("ETag")}), 409)
	}
	expect(t, admin.request("DELETE", "/api/v1/users/"+adminID, "", map[string]string{"If-Match": w.Header().Get("ETag")}), 409)
	expect(t, admin.request("POST", "/api/v1/users", fmt.Sprintf(`{"username":"ADMIN","displayName":"duplicate","role":"viewer","password":%q}`, testPassword), nil), 409)
}

func TestConcurrentLastAdminProtection(t *testing.T) {
	s, h, a, firstID := fixture(t)
	loginAs(t, a, "admin", testPassword)
	w := a.request("POST", "/api/v1/users", fmt.Sprintf(`{"username":"second","displayName":"Second","role":"admin","password":%q}`, testPassword), nil)
	expect(t, w, 201)
	var result struct{ Data User }
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	b := newBrowser(t, h)
	loginAs(t, b, "second", testPassword)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for n, client := range []*browser{a, b} {
		id := firstID
		if n == 1 {
			id = result.Data.ID
		}
		wg.Go(func() {
			codes <- client.request("PATCH", "/api/v1/users/"+id, `{"role":"viewer"}`, map[string]string{"If-Match": fmt.Sprintf(`"%s-v1"`, id)}).Code
		})
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatalf("concurrent admin outcomes: %v", counts)
	}
	var admins int
	if err := s.db.QueryRow("SELECT count(*) FROM users WHERE role='admin' AND enabled=1").Scan(&admins); err != nil || admins != 1 {
		t.Fatalf("enabled admins=%d, err=%v", admins, err)
	}
}

// Read the public session IDs through the owner's API, never session secrets.
func currentSessionID(t *testing.T, owner *browser) string {
	t.Helper()
	w := owner.request("GET", "/api/v1/auth/sessions", "", nil)
	expect(t, w, 200)
	var body struct {
		Data []struct {
			ID      string
			Current bool
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, session := range body.Data {
		if session.Current {
			return session.ID
		}
	}
	t.Fatal("current session missing")
	return ""
}

func TestUserPaginationAndConcurrentVersionChecks(t *testing.T) {
	_, h, a, id := fixture(t)
	loginAs(t, a, "admin", testPassword)
	expect(t, a.request("POST", "/api/v1/users", fmt.Sprintf(`{"username":"viewer","displayName":"Viewer","role":"viewer","password":%q}`, testPassword), nil), 201)
	seen := map[string]bool{}
	cursor := ""
	for page := 0; page < 2; page++ {
		w := a.request("GET", "/api/v1/users?limit=1&cursor="+cursor, "", nil)
		expect(t, w, 200)
		var result struct {
			Data []User
			Meta struct {
				NextCursor *string `json:"nextCursor"`
			}
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Data) != 1 || seen[result.Data[0].ID] {
			t.Fatal("pagination duplicated or lost users")
		}
		seen[result.Data[0].ID] = true
		if page == 0 {
			if result.Meta.NextCursor == nil {
				t.Fatal("missing cursor")
			}
			cursor = *result.Meta.NextCursor
		} else if result.Meta.NextCursor != nil {
			t.Fatal("unexpected extra page")
		}
	}
	expect(t, a.request("GET", "/api/v1/users?cursor="+cursor+"x", "", nil), 400)
	b := newBrowser(t, h)
	loginAs(t, b, "admin", testPassword)
	w := a.request("GET", "/api/v1/users/"+id, "", nil)
	expect(t, w, 200)
	version := w.Header().Get("ETag")
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for _, client := range []*browser{a, b} {
		wg.Go(func() {
			codes <- client.request("PATCH", "/api/v1/users/"+id, `{"displayName":"Renamed"}`, map[string]string{"If-Match": version}).Code
		})
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[200] != 1 || counts[412] != 1 {
		t.Fatalf("concurrent version outcomes: %v", counts)
	}
}

func TestRateLimitsAndSessionExpiry(t *testing.T) {
	s, h, b, _ := fixture(t)
	for n := 0; n < 5; n++ {
		expect(t, b.request("POST", "/api/v1/auth/login", `{"username":"admin","password":"wrong"}`, nil), 401)
	}
	expect(t, b.request("POST", "/api/v1/auth/login", fmt.Sprintf(`{"username":"admin","password":%q}`, testPassword), map[string]string{"X-Forwarded-For": "192.0.2.123"}), 429)
	if _, err := s.db.Exec("DELETE FROM login_attempts"); err != nil {
		t.Fatal(err)
	}
	loginAs(t, b, "admin", testPassword)
	// An idle timeout and an absolute expiry both reject otherwise valid cookies.
	for _, column := range []string{"last_seen", "expires_at"} {
		if _, err := s.db.Exec("UPDATE sessions SET "+column+"=?", time.Now().Add(-time.Hour).Unix()); err != nil {
			t.Fatal(err)
		}
		expect(t, b.request("GET", "/api/v1/auth/me", "", nil), 401)
		b = newBrowser(t, h)
		loginAs(t, b, "admin", testPassword)
	}
}

func TestPasswordChangeReauthenticationAndOwnedSessions(t *testing.T) {
	s, h, a, _ := fixture(t)
	loginAs(t, a, "admin", testPassword)
	expect(t, a.request("GET", "/api/v1/auth/permissions", "", nil), 200)
	expect(t, a.request("GET", "/api/v1/auth/activity", "", nil), 200)
	expect(t, a.request("POST", "/api/v1/auth/refresh", `{}`, nil), 200)
	a.refreshCSRF()
	b := newBrowser(t, h)
	loginAs(t, b, "admin", testPassword)
	oldCookie := *a.cookies["ariba_dev_session"]
	expect(t, a.request("POST", "/api/v1/auth/reauthenticate", fmt.Sprintf(`{"password":%q}`, testPassword), nil), 200)
	if oldCookie.Value == a.cookies[oldCookie.Name].Value {
		t.Fatal("reauthentication did not rotate session")
	}
	a.refreshCSRF()
	stale := newBrowser(t, h)
	stale.cookies[oldCookie.Name] = &oldCookie
	expect(t, stale.request("GET", "/api/v1/auth/me", "", nil), 401)
	var sessionID string
	if err := s.db.QueryRow("SELECT id FROM sessions WHERE hash=?", tokenHash(b.cookies["ariba_dev_session"].Value)).Scan(&sessionID); err != nil {
		t.Fatal(err)
	}
	expect(t, a.request("GET", "/api/v1/auth/sessions", "", nil), 200)
	expect(t, a.request("DELETE", "/api/v1/auth/sessions/"+sessionID, "", nil), 204)
	expect(t, b.request("GET", "/api/v1/auth/me", "", nil), 401)
	loginAs(t, b, "admin", testPassword)
	expect(t, a.request("POST", "/api/v1/auth/password", fmt.Sprintf(`{"currentPassword":%q,"newPassword":"new local password 123"}`, testPassword), nil), 204)
	expect(t, b.request("GET", "/api/v1/auth/me", "", nil), 401)
	loginAs(t, a, "admin", "new local password 123")
	b2 := newBrowser(t, h)
	loginAs(t, b2, "admin", "new local password 123")
	expect(t, a.request("POST", "/api/v1/auth/sessions/revoke-all", `{}`, nil), 204)
	expect(t, b2.request("GET", "/api/v1/auth/me", "", nil), 401)
}

func TestObservabilityEndpoints(t *testing.T) {
	_, h, admin, _ := fixture(t)
	loginAs(t, admin, "admin", testPassword)

	// Any authenticated role can access events/metrics.
	expect(t, admin.request("GET", "/api/v1/events", "", nil), 200)
	expect(t, admin.request("GET", "/api/v1/metrics/overview", "", nil), 200)
	expect(t, admin.request("GET", "/api/v1/audit-logs", "", nil), 200)

	// Non-admin cannot read audit logs.
	w := admin.request("POST", "/api/v1/users", fmt.Sprintf(`{"username":"viewer_obs","displayName":"Viewer Obs","role":"viewer","password":%q}`, testPassword), nil)
	expect(t, w, 201)

	viewer := newBrowser(t, h)
	loginAs(t, viewer, "viewer_obs", testPassword)
	expect(t, viewer.request("GET", "/api/v1/events", "", nil), 200)
	expect(t, viewer.request("GET", "/api/v1/metrics/overview", "", nil), 200)
	expect(t, viewer.request("GET", "/api/v1/audit-logs", "", nil), 403)
}

func TestConfigurationAndStoragePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	path := filepath.Join(dir, "db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, path := range []string{dir, path} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0077 != 0 {
			t.Fatalf("insecure permissions: %s", path)
		}
	}
	for _, cfg := range []Config{
		{Origin: "http://localhost:3000"},
		{Origin: "http://192.0.2.1:3000", DevHTTP: true},
		{Origin: "https://localhost:9443/path"},
		{Origin: "https://user:password@localhost:9443"},
	} {
		if _, err := NewHandler(s, cfg); err == nil {
			t.Fatalf("accepted unsafe config %+v", cfg)
		}
	}
	h, err := NewHandler(s, Config{Origin: "https://localhost:9443"})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/auth/csrf", nil))
	expect(t, w, 200)
	c := w.Result().Cookies()[0]
	if c.Name != "__Host-ariba_preauth" || !c.Secure || !c.HttpOnly || c.Path != "/" || c.Domain != "" {
		t.Fatal("incorrect secure cookie")
	}
}

func TestSetupStatusLifecycle(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data", "ariba.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	h, err := NewHandler(s, Config{Origin: testOrigin, DevHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	b := newBrowser(t, h)

	var status struct {
		Data struct {
			EnrollmentRequired bool `json:"enrollmentRequired"`
		} `json:"data"`
	}
	w := b.request("GET", "/api/v1/setup/status", "", nil)
	expect(t, w, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if !status.Data.EnrollmentRequired {
		t.Fatal("fresh instance must require enrollment")
	}

	// Observability routes are authenticated even before setup.
	expect(t, b.request("GET", "/api/v1/events", "", nil), 401)
	expect(t, b.request("GET", "/api/v1/metrics/overview", "", nil), 401)

	token, err := s.CreateEnrollmentToken(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"enrollmentToken":%q,"username":"admin","displayName":"Admin","password":%q}`, token, testPassword)
	expect(t, b.request("POST", "/api/v1/setup/enroll", body, map[string]string{"Idempotency-Key": "setup-status"}), 201)

	w = b.request("GET", "/api/v1/setup/status", "", nil)
	expect(t, w, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Data.EnrollmentRequired {
		t.Fatal("enrollment must close after the first Admin exists")
	}
}

func TestObservabilityPayloadContracts(t *testing.T) {
	_, _, admin, _ := fixture(t)
	loginAs(t, admin, "admin", testPassword)

	var events struct {
		Data []eventRecord `json:"data"`
	}
	w := admin.request("GET", "/api/v1/events", "", nil)
	expect(t, w, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &events); err != nil {
		t.Fatal(err)
	}
	if events.Data == nil || len(events.Data) != 0 {
		t.Fatalf("expected empty event list, got %#v", events.Data)
	}

	var metrics struct {
		Data struct {
			Counters  map[string]int64 `json:"counters"`
			Freshness struct {
				Status string `json:"status"`
			} `json:"freshness"`
		} `json:"data"`
	}
	w = admin.request("GET", "/api/v1/metrics/overview", "", nil)
	expect(t, w, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &metrics); err != nil {
		t.Fatal(err)
	}
	if metrics.Data.Freshness.Status != "unavailable" {
		t.Fatalf("freshness status=%q, want unavailable", metrics.Data.Freshness.Status)
	}
	for _, key := range []string{"requests", "detections", "blocks"} {
		if _, ok := metrics.Data.Counters[key]; !ok {
			t.Fatalf("missing counter %q: %#v", key, metrics.Data.Counters)
		}
	}
}
