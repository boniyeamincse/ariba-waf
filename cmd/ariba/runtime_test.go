package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestManagementListenIsolation(t *testing.T) {
	for _, tc := range []struct {
		address      string
		allow, valid bool
	}{
		{"127.0.0.1:9000", false, true}, {"[::1]:9000", false, true},
		{"0.0.0.0:9000", false, false}, {"0.0.0.0:9000", true, true},
		{"192.168.1.2:9000", false, false}, {"192.168.1.2:9000", true, true},
		{"localhost:9000", false, false}, {":9000", true, false},
		{"127.0.0.1:0", false, false}, {"127.0.0.1:65536", false, false},
		{"127.0.0.1:http", false, false}, {"invalid", true, false},
	} {
		t.Run(tc.address, func(t *testing.T) {
			if _, err := validateListenAddress(tc.address, tc.allow); (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}

func TestLivenessProbeStatusAndProxyIsolation(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable, http.StatusFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if status == http.StatusFound {
					w.Header().Set("Location", "/health/live")
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			if err := probeLiveness(server.URL + "/health/live"); (err == nil) != (status == http.StatusOK) {
				t.Fatalf("status=%d, error=%v", status, err)
			}
		})
	}
}
