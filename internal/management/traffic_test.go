package management

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func responseData(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var result struct{ Data map[string]any }
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result.Data
}

func createPolicy(t *testing.T, b *browser, name string) (string, string) {
	t.Helper()
	w := b.request("POST", "/api/v1/policies", fmt.Sprintf(`{"name":%q}`, name), nil)
	expect(t, w, 201)
	return responseData(t, w)["id"].(string), w.Header().Get("ETag")
}

func siteBody(policy, domain, host string) string {
	return fmt.Sprintf(`{"name":"Demo","domains":[%q],"policyId":%q,"upstreams":[{"scheme":"http","host":%q,"port":8081}]}`, domain, policy, host)
}

func allowCIDRs(t *testing.T, b *browser, cidrs ...string) *httptest.ResponseRecorder {
	t.Helper()
	w := b.request("GET", "/api/v1/settings/proxy", "", nil)
	expect(t, w, 200)
	encoded, _ := json.Marshal(cidrs)
	w = b.request("PATCH", "/api/v1/settings/proxy", `{"allowedCidrs":`+string(encoded)+`}`, map[string]string{"If-Match": w.Header().Get("ETag")})
	expect(t, w, 200)
	return w
}

func desiredGeneration(t *testing.T, b *browser) float64 {
	t.Helper()
	w := b.request("GET", "/api/v1/configuration/status", "", nil)
	expect(t, w, 200)
	data := responseData(t, w)
	if data["appliedRevisionId"] != nil || data["activationExecutorAvailable"] != false || data["currentJob"] != nil {
		t.Fatalf("desired state misreported as active: %v", data)
	}
	return data["desiredGeneration"].(float64)
}

func TestTrafficCRUDGenerationAndReferences(t *testing.T) {
	s, _, b, _ := fixture(t)
	loginAs(t, b, "admin", testPassword)
	if desiredGeneration(t, b) != 0 {
		t.Fatal("initial generation is not zero")
	}
	policy, policyTag := createPolicy(t, b, "Detection")
	// No destinations are permitted until an Admin explicitly grants a network.
	expect(t, b.request("POST", "/api/v1/websites", siteBody(policy, "app.example.test", "10.24.20.50"), nil), 422)
	if desiredGeneration(t, b) != 1 {
		t.Fatal("rejected write changed generation")
	}
	allowCIDRs(t, b, "10.24.20.0/24")
	w := b.request("POST", "/api/v1/websites", siteBody(policy, "App.Example.Test.", "10.24.20.50"), nil)
	expect(t, w, 201)
	data := responseData(t, w)
	id := data["id"].(string)
	tag := w.Header().Get("ETag")
	if data["desiredGeneration"] != float64(3) || data["appliedRevisionId"] != nil || data["pendingChanges"] != true || data["domains"].([]any)[0] != "app.example.test" {
		t.Fatalf("incorrect website state: %v", data)
	}
	expect(t, b.request("DELETE", "/api/v1/policies/"+policy, "", map[string]string{"If-Match": policyTag}), 409)
	expect(t, b.request("PATCH", "/api/v1/websites/"+id, `{"name":"Changed"}`, nil), 428)
	expect(t, b.request("PATCH", "/api/v1/websites/"+id, `{"name":"Changed"}`, map[string]string{"If-Match": "stale"}), 412)
	w = b.request("PATCH", "/api/v1/websites/"+id, `{"name":"Changed","timeouts":{"readSeconds":120}}`, map[string]string{"If-Match": tag})
	expect(t, w, 200)
	data = responseData(t, w)
	tag = w.Header().Get("ETag")
	if data["timeouts"].(map[string]any)["connectSeconds"] != float64(5) || data["timeouts"].(map[string]any)["readSeconds"] != float64(120) {
		t.Fatal("PATCH lost nested fields")
	}
	if desiredGeneration(t, b) != 4 {
		t.Fatal("wrong generation after patch")
	}
	expect(t, b.request("GET", "/api/v1/websites/"+id, "", nil), 200)
	expect(t, b.request("GET", "/api/v1/websites", "", nil), 200)
	expect(t, b.request("DELETE", "/api/v1/websites/"+id, "", map[string]string{"If-Match": tag}), 204)
	expect(t, b.request("GET", "/api/v1/websites/"+id, "", nil), 404)
	expect(t, b.request("DELETE", "/api/v1/policies/"+policy, "", map[string]string{"If-Match": policyTag}), 204)
	if desiredGeneration(t, b) != 6 {
		t.Fatal("wrong generation after deletes")
	}
	var audits int
	if err := s.db.QueryRow("SELECT count(*) FROM audit_entries WHERE action LIKE 'policies.%' OR action LIKE 'websites.%' OR action='settings.proxy'").Scan(&audits); err != nil || audits != 6 {
		t.Fatalf("audit count=%d, err=%v", audits, err)
	}
}

func TestTypedTrafficValidationIsAtomic(t *testing.T) {
	_, _, b, _ := fixture(t)
	loginAs(t, b, "admin", testPassword)
	policy, tag := createPolicy(t, b, "Detection")
	allowCIDRs(t, b, "10.0.0.0/8")
	for _, body := range []string{
		`{"name":null}`, `{"engineMode":"unknown"}`, `{"crs":{"paranoiaLevel":5}}`,
		`{"crs":{"inboundAnomalyThreshold":0}}`, `{"inspection":{"requestBodyLimitBytes":0}}`,
		`{"inspection":{"requestBodyNoFilesLimitBytes":104857601}}`, `{"inspection":{"responseBodyEnabled":true}}`,
		`{"rateLimit":{"requestsPerSecond":10001}}`, `{"rateLimit":{"burst":-1}}`,
		`{"crs":null}`, `{"inspection":{"requestBodyEnabled":null}}`, `{}`,
	} {
		t.Run(body, func(t *testing.T) {
			expect(t, b.request("PATCH", "/api/v1/policies/"+policy, body, map[string]string{"If-Match": tag}), 422)
		})
	}
	expect(t, b.request("PATCH", "/api/v1/policies/"+policy, `{"id":"chosen"}`, map[string]string{"If-Match": tag}), 400)
	expect(t, b.request("PATCH", "/api/v1/policies/"+policy, `{"crs":{"rawDirective":"evil"}}`, map[string]string{"If-Match": tag}), 400)
	for _, body := range []string{
		strings.Replace(siteBody(policy, "app.example.test", "10.0.0.2"), `"name":"Demo"`, `"name":null`, 1),
		strings.Replace(siteBody(policy, "app.example.test", "10.0.0.2"), `"policyId":"`+policy+`"`, `"policyId":null`, 1),
		strings.Replace(siteBody(policy, "app.example.test", "10.0.0.2"), `"domains":["app.example.test"]`, `"domains":["app.example.test","APP.example.test."]`, 1),
		strings.Replace(siteBody(policy, "app.example.test", "10.0.0.2"), `"upstreams":`, `"listeners":{"http":false},"upstreams":`, 1),
		strings.Replace(siteBody(policy, "app.example.test", "10.0.0.2"), `"upstreams":`, `"listeners":{"https":true},"upstreams":`, 1),
		strings.Replace(siteBody(policy, "app.example.test", "10.0.0.2"), `"upstreams":`, `"upstreamTls":{"verify":false},"upstreams":`, 1),
		strings.Replace(siteBody(policy, "app.example.test", "10.0.0.2"), `"upstreams":`, `"upstreamTls":{"trustSource":"bundle","trustBundleId":"missing"},"upstreams":`, 1),
		strings.Replace(siteBody(policy, "app.example.test", "10.0.0.2"), `"port":8081`, `"port":0`, 1),
		strings.Replace(siteBody(policy, "app.example.test", "10.0.0.2"), `"scheme":"http"`, `"scheme":"file"`, 1),
		siteBody(policy, "*.example.test", "10.0.0.2"), siteBody(policy, "127.0.0.1", "10.0.0.2"), siteBody(policy, "app.example.test", "user@origin.test"),
	} {
		expect(t, b.request("POST", "/api/v1/websites", body, nil), 422)
	}
	if desiredGeneration(t, b) != 2 {
		t.Fatal("invalid input advanced generation")
	}
	w := b.request("PATCH", "/api/v1/policies/"+policy, `{"engineMode":"blocking"}`, map[string]string{"If-Match": tag})
	expect(t, w, 200)
	expect(t, b.request("POST", "/api/v1/websites", siteBody(policy, "app.example.test", "10.0.0.2"), nil), 422)
	if desiredGeneration(t, b) != 3 {
		t.Fatal("rejected initial blocking policy changed state")
	}
}

func TestDomainConflictsAndProxyReferenceValidation(t *testing.T) {
	_, _, b, _ := fixture(t)
	loginAs(t, b, "admin", testPassword)
	policy, _ := createPolicy(t, b, "Detection")
	allowCIDRs(t, b, "10.0.0.0/8")
	w := b.request("POST", "/api/v1/websites", siteBody(policy, "bücher.example.test", "10.1.2.3"), nil)
	expect(t, w, 201)
	data := responseData(t, w)
	first := data["id"].(string)
	if data["domains"].([]any)[0] != "xn--bcher-kva.example.test" {
		t.Fatal("IDNA normalization missing")
	}
	expect(t, b.request("POST", "/api/v1/websites", siteBody(policy, "XN--BCHER-KVA.example.test.", "10.1.2.3"), nil), 409)
	disabled := strings.Replace(siteBody(policy, "bücher.example.test", "10.1.2.3"), `"name":"Demo"`, `"name":"Disabled","enabled":false`, 1)
	w = b.request("POST", "/api/v1/websites", disabled, nil)
	expect(t, w, 201)
	second := responseData(t, w)["id"].(string)
	tag := w.Header().Get("ETag")
	expect(t, b.request("PATCH", "/api/v1/websites/"+second, `{"enabled":true}`, map[string]string{"If-Match": tag}), 409)
	proxy := b.request("GET", "/api/v1/settings/proxy", "", nil)
	expect(t, proxy, 200)
	expect(t, b.request("PATCH", "/api/v1/settings/proxy", `{"allowedCidrs":[]}`, map[string]string{"If-Match": proxy.Header().Get("ETag")}), 409)
	expect(t, b.request("PATCH", "/api/v1/settings/proxy", `{"allowedCidrs":["bad"]}`, map[string]string{"If-Match": proxy.Header().Get("ETag")}), 422)
	if desiredGeneration(t, b) != 4 {
		t.Fatal("failed validation changed generation")
	}
	w = b.request("GET", "/api/v1/websites/"+first, "", nil)
	expect(t, w, 200)
	expect(t, b.request("PATCH", "/api/v1/websites/"+first, `{"enabled":false}`, map[string]string{"If-Match": w.Header().Get("ETag")}), 200)
	expect(t, b.request("PATCH", "/api/v1/websites/"+second, `{"enabled":true}`, map[string]string{"If-Match": tag}), 200)
}

type resolverFunc func(context.Context, string, string) ([]netip.Addr, error)

func (f resolverFunc) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return f(ctx, network, host)
}

func TestDestinationPolicyChecksEveryAnswer(t *testing.T) {
	s, _, _, _ := fixture(t)
	resolver := resolverFunc(func(ctx context.Context, _, host string) ([]netip.Addr, error) {
		switch host {
		case "good.example.test":
			return []netip.Addr{netip.MustParseAddr("10.1.2.3"), netip.MustParseAddr("fd12::1")}, nil
		case "mixed.example.test":
			return []netip.Addr{netip.MustParseAddr("10.1.2.3"), netip.MustParseAddr("127.0.0.1")}, nil
		case "metadata.example.test":
			return []netip.Addr{netip.MustParseAddr("169.254.169.254")}, nil
		case "outside.example.test":
			return []netip.Addr{netip.MustParseAddr("192.0.2.1")}, nil
		default:
			return nil, errors.New("unavailable")
		}
	})
	h, err := NewHandler(s, Config{Origin: testOrigin, DevHTTP: true, Resolver: resolver})
	if err != nil {
		t.Fatal(err)
	}
	b := newBrowser(t, h)
	loginAs(t, b, "admin", testPassword)
	policy, _ := createPolicy(t, b, "Detection")
	allowCIDRs(t, b, "10.0.0.0/8", "fd12::/16")
	expect(t, b.request("POST", "/api/v1/websites", siteBody(policy, "good.example.test", "GOOD.example.test."), nil), 201)
	for _, host := range []string{"mixed.example.test", "metadata.example.test", "outside.example.test", "missing.example.test"} {
		expect(t, b.request("POST", "/api/v1/websites", siteBody(policy, "new.example.test", host), nil), 422)
	}
	// Even an explicit broad allowlist cannot authorize metadata, loopback or tunnels.
	allowCIDRs(t, b, "0.0.0.0/0", "::/0")
	for _, host := range []string{"127.0.0.1", "::1", "::ffff:127.0.0.1", "169.254.169.254", "100.100.100.200", "168.63.129.16", "0.0.0.0", "224.0.0.1", "fe80::1", "fd00:ec2::254", "64:ff9b::a9fe:a9fe", "2002:a9fe:a9fe::1"} {
		expect(t, b.request("POST", "/api/v1/websites", siteBody(policy, "new.example.test", host), nil), 422)
	}
	for _, port := range []string{"9000", "9443"} {
		body := strings.Replace(siteBody(policy, "new.example.test", "10.1.2.3"), `"port":8081`, `"port":`+port, 1)
		expect(t, b.request("POST", "/api/v1/websites", body, nil), 422)
	}
	if desiredGeneration(t, b) != 4 {
		t.Fatal("forbidden destinations changed state")
	}
}

func TestTrafficRolePermissions(t *testing.T) {
	_, h, admin, _ := fixture(t)
	loginAs(t, admin, "admin", testPassword)
	policy, _ := createPolicy(t, admin, "Detection")
	allowCIDRs(t, admin, "10.0.0.0/8")
	anonymous := newBrowser(t, h)
	for _, path := range []string{"/policies", "/websites", "/configuration/status", "/settings/proxy"} {
		expect(t, anonymous.request("GET", "/api/v1"+path, "", nil), 401)
	}
	for _, role := range []string{"operator", "viewer"} {
		expect(t, admin.request("POST", "/api/v1/users", fmt.Sprintf(`{"username":%q,"displayName":%q,"role":%q,"password":%q}`, role, role, role, testPassword), nil), 201)
		b := newBrowser(t, h)
		loginAs(t, b, role, testPassword)
		for _, path := range []string{"/policies", "/policies/" + policy, "/websites", "/configuration/status"} {
			expect(t, b.request("GET", "/api/v1"+path, "", nil), 200)
		}
		expect(t, b.request("GET", "/api/v1/settings/proxy", "", nil), 403)
		expect(t, b.request("PATCH", "/api/v1/settings/proxy", `{}`, nil), 403)
		if role == "viewer" {
			for _, path := range []string{"/policies", "/websites"} {
				expect(t, b.request("POST", "/api/v1"+path, `{}`, nil), 403)
			}
			expect(t, b.request("PATCH", "/api/v1/policies/"+policy, `{}`, nil), 403)
			expect(t, b.request("DELETE", "/api/v1/policies/"+policy, "", nil), 403)
		} else {
			p, _ := createPolicy(t, b, "Operator policy")
			w := b.request("POST", "/api/v1/websites", siteBody(p, "operator.example.test", "10.0.0.2"), nil)
			expect(t, w, 201)
			id := responseData(t, w)["id"].(string)
			expect(t, b.request("DELETE", "/api/v1/websites/"+id, "", map[string]string{"If-Match": w.Header().Get("ETag")}), 204)
		}
	}
}

func TestTrafficPaginationAndConcurrentDomainWrites(t *testing.T) {
	_, h, a, _ := fixture(t)
	loginAs(t, a, "admin", testPassword)
	policy, _ := createPolicy(t, a, "First")
	createPolicy(t, a, "Second")
	allowCIDRs(t, a, "10.0.0.0/8")
	w := a.request("GET", "/api/v1/policies?limit=1", "", nil)
	expect(t, w, 200)
	var first struct {
		Data []map[string]any
		Meta struct {
			NextCursor string `json:"nextCursor"`
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	w = a.request("GET", "/api/v1/policies?limit=1&cursor="+first.Meta.NextCursor, "", nil)
	expect(t, w, 200)
	var second struct{ Data []map[string]any }
	if err := json.Unmarshal(w.Body.Bytes(), &second); err != nil {
		t.Fatal(err)
	}
	if len(first.Data) != 1 || len(second.Data) != 1 || first.Data[0]["id"] == second.Data[0]["id"] {
		t.Fatal("pagination lost or duplicated policies")
	}
	expect(t, a.request("GET", "/api/v1/websites?cursor="+first.Meta.NextCursor, "", nil), 400)
	b := newBrowser(t, h)
	loginAs(t, b, "admin", testPassword)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for _, client := range []*browser{a, b} {
		wg.Go(func() {
			codes <- client.request("POST", "/api/v1/websites", siteBody(policy, "same.example.test", "10.0.0.2"), nil).Code
		})
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[201] != 1 || counts[409] != 1 {
		t.Fatalf("concurrent domain outcomes: %v", counts)
	}
	if desiredGeneration(t, a) != 4 {
		t.Fatal("conflicting write changed state")
	}
	current := a.request("GET", "/api/v1/policies/"+policy, "", nil)
	expect(t, current, 200)
	tag := current.Header().Get("ETag")
	codes = make(chan int, 2)
	for _, client := range []*browser{a, b} {
		wg.Go(func() {
			codes <- client.request("PATCH", "/api/v1/policies/"+policy, `{"crs":{"paranoiaLevel":2}}`, map[string]string{"If-Match": tag}).Code
		})
	}
	wg.Wait()
	close(codes)
	counts = map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[200] != 1 || counts[412] != 1 {
		t.Fatalf("concurrent policy outcomes: %v", counts)
	}
	if desiredGeneration(t, a) != 5 {
		t.Fatal("stale policy update changed state")
	}
	updated := a.request("GET", "/api/v1/policies/"+policy, "", nil)
	expect(t, updated, 200)
	crs := responseData(t, updated)["crs"].(map[string]any)
	if crs["paranoiaLevel"] != float64(2) || crs["inboundAnomalyThreshold"] != float64(5) {
		t.Fatal("nested policy PATCH lost other fields")
	}
}

func TestTrafficMigrationAndPersistence(t *testing.T) {
	s, _, b, _ := fixture(t)
	loginAs(t, b, "admin", testPassword)
	var path string
	if err := s.db.QueryRow("SELECT file FROM pragma_database_list WHERE name='main'").Scan(&path); err != nil {
		t.Fatal(err)
	}
	// Reconstruct the exact v1 table set, retaining its real user/session data.
	if _, err := s.db.Exec(`DROP TABLE website_domains; DROP TABLE websites; DROP TABLE policies; DROP TABLE proxy_settings; DROP TABLE configuration_state; PRAGMA user_version=1;`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewHandler(reopened, Config{Origin: testOrigin, DevHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	b.h = h
	expect(t, b.request("GET", "/api/v1/auth/me", "", nil), 200)
	policy, _ := createPolicy(t, b, "Persisted")
	allowCIDRs(t, b, "10.0.0.0/8")
	expect(t, b.request("POST", "/api/v1/websites", siteBody(policy, "persist.example.test", "10.0.0.2"), nil), 201)
	reopened.Close()
	final, err := Open(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	defer final.Close()
	h, err = NewHandler(final, Config{Origin: testOrigin, DevHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	b.h = h
	expect(t, b.request("GET", "/api/v1/policies/"+policy, "", nil), 200)
	if desiredGeneration(t, b) != 3 {
		t.Fatal("configuration state lost across restart")
	}
	if _, err := final.db.Exec("PRAGMA user_version=99"); err != nil {
		t.Fatal(err)
	}
	if newer, err := Open(path); err == nil {
		newer.Close()
		t.Fatal("accepted unknown future schema")
	}
}
