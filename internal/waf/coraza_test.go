package waf

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"testing"
)

const controlledCRSPayload = "/?q=%3Cscript%3Ealert%281%29%3C%2Fscript%3E"

func TestCorazaModesWithCRS(t *testing.T) {
	tests := []struct {
		name             string
		mode             Mode
		path             string
		wantStatus       int
		wantUpstreamHits int32
	}{
		{
			name:             "benign request passes in blocking mode",
			mode:             ModeBlocking,
			path:             "/health",
			wantStatus:       http.StatusOK,
			wantUpstreamHits: 1,
		},
		{
			name:             "controlled CRS match passes in detection mode",
			mode:             ModeDetection,
			path:             controlledCRSPayload,
			wantStatus:       http.StatusOK,
			wantUpstreamHits: 1,
		},
		{
			name:       "controlled CRS match is blocked",
			mode:       ModeBlocking,
			path:       controlledCRSPayload,
			wantStatus: http.StatusForbidden,
		},
		{
			name:             "off mode forwards controlled test request",
			mode:             ModeOff,
			path:             controlledCRSPayload,
			wantStatus:       http.StatusOK,
			wantUpstreamHits: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var upstreamHits atomic.Int32
			upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				upstreamHits.Add(1)
				w.WriteHeader(http.StatusOK)
			})

			handler, err := NewCorazaHandler(upstream, tt.mode)
			if err != nil {
				t.Fatalf("NewCorazaHandler() error = %v", err)
			}

			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			request.Host = "waf-test.example"
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if got := upstreamHits.Load(); got != tt.wantUpstreamHits {
				t.Errorf("upstream hits = %d, want %d", got, tt.wantUpstreamHits)
			}
		})
	}
}

func TestNewCorazaHandlerRejectsInvalidConfiguration(t *testing.T) {
	if _, err := NewCorazaHandler(nil, ModeBlocking); err == nil {
		t.Fatal("NewCorazaHandler(nil, ModeBlocking) expected an error")
	}

	if _, err := NewCorazaHandler(http.NotFoundHandler(), Mode("unknown")); err == nil {
		t.Fatal("NewCorazaHandler with an unknown mode expected an error")
	}
}

func TestCorazaHandlerProxiesToDemoOrigin(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(r.URL.RequestURI()))
	}))
	defer origin.Close()

	target, err := url.Parse(origin.URL)
	if err != nil {
		t.Fatalf("parse origin URL: %v", err)
	}
	handler, err := NewCorazaHandler(httputil.NewSingleHostReverseProxy(target), ModeBlocking)
	if err != nil {
		t.Fatalf("NewCorazaHandler() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/health?check=ok", nil)
	request.Host = "waf-test.example"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got, want := recorder.Body.String(), "/health?check=ok"; got != want {
		t.Errorf("upstream request URI = %q, want %q", got, want)
	}
}

func BenchmarkCorazaCRSAllowedRequest(b *testing.B) {
	handler, err := NewCorazaHandler(http.NotFoundHandler(), ModeDetection)
	if err != nil {
		b.Fatalf("NewCorazaHandler() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Host = "waf-test.example"
	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNotFound {
			b.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
		}
	}
}
