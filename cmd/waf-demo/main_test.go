package main

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/boniyeamincse/ariba-waf/internal/waf"
)

func TestValidateLoopbackAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{name: "IPv4 loopback", address: "127.0.0.1:18080"},
		{name: "IPv6 loopback", address: "[::1]:18080"},
		{name: "all interfaces", address: "0.0.0.0:18080", wantErr: true},
		{name: "private interface", address: "192.168.1.10:18080", wantErr: true},
		{name: "hostname", address: "localhost:18080", wantErr: true},
		{name: "malformed address", address: "not-an-address", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateLoopbackAddress(tt.address)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateLoopbackAddress() error = %v, wantErr %t", err, tt.wantErr)
			}
			if err == nil && got != tt.address {
				t.Errorf("address = %q, want %q", got, tt.address)
			}
		})
	}
}

func TestNewDemoTLSConfig(t *testing.T) {
	config, err := newDemoTLSConfig()
	if err != nil {
		t.Fatalf("newDemoTLSConfig() error = %v", err)
	}
	if config.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %d, want TLS 1.2", config.MinVersion)
	}
	if len(config.Certificates) != 1 || len(config.Certificates[0].Certificate) != 1 {
		t.Fatalf("expected one self-signed certificate")
	}

	certificate, err := x509.ParseCertificate(config.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatalf("parse generated certificate: %v", err)
	}
	if err := certificate.VerifyHostname("localhost"); err != nil {
		t.Errorf("certificate does not cover localhost: %v", err)
	}
	if err := certificate.VerifyHostname("127.0.0.1"); err != nil {
		t.Errorf("certificate does not cover loopback IP: %v", err)
	}
}

func TestHTTPSDemoEnforcesCorazaRules(t *testing.T) {
	var originHits atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originHits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(r.URL.RequestURI()))
	}))
	defer origin.Close()

	target, err := url.Parse(origin.URL)
	if err != nil {
		t.Fatalf("parse origin URL: %v", err)
	}
	handler, err := waf.NewCorazaHandler(httputil.NewSingleHostReverseProxy(target), waf.ModeBlocking)
	if err != nil {
		t.Fatalf("NewCorazaHandler() error = %v", err)
	}

	tlsConfig, err := newDemoTLSConfig()
	if err != nil {
		t.Fatalf("newDemoTLSConfig() error = %v", err)
	}
	certificate, err := x509.ParseCertificate(tlsConfig.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatalf("parse generated certificate: %v", err)
	}
	trustedRoots := x509.NewCertPool()
	trustedRoots.AddCert(certificate)

	server := httptest.NewUnstartedServer(handler)
	server.TLS = tlsConfig
	server.StartTLS()
	defer server.Close()
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: trustedRoots, MinVersion: tls.VersionTLS12},
	}}

	response, err := client.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("GET benign HTTPS request: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("benign HTTPS status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	response, err = client.Get(server.URL + "/?q=%3Cscript%3Ealert%281%29%3C%2Fscript%3E")
	if err != nil {
		t.Fatalf("GET controlled HTTPS request: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("controlled HTTPS status = %d, want %d", response.StatusCode, http.StatusForbidden)
	}
	if got := originHits.Load(); got != 1 {
		t.Errorf("origin hits = %d, want 1 (only the benign request should pass)", got)
	}
}
