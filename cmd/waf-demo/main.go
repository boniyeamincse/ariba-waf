// Command waf-demo runs an isolated HTTP/HTTPS Coraza/CRS prototype.
// It intentionally has no configurable upstream and binds only to loopback.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/boniyeamincse/ariba-waf/internal/waf"
)

const (
	defaultHTTPAddress  = "127.0.0.1:18080"
	defaultHTTPSAddress = "127.0.0.1:18443"
)

func main() {
	modeFlag := flag.String("mode", string(waf.ModeDetection), "WAF mode: off, detectionOnly, or blocking")
	httpListenFlag := flag.String("http-listen", defaultHTTPAddress, "loopback HTTP listen address for the local demo")
	httpsListenFlag := flag.String("https-listen", defaultHTTPSAddress, "loopback HTTPS listen address for the local demo")
	flag.Parse()

	mode := waf.Mode(*modeFlag)
	if mode != waf.ModeOff && mode != waf.ModeDetection && mode != waf.ModeBlocking {
		fmt.Fprintln(os.Stderr, "invalid -mode; choose off, detectionOnly, or blocking")
		os.Exit(2)
	}
	httpAddress, err := validateLoopbackAddress(*httpListenFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	httpsAddress, err := validateLoopbackAddress(*httpsListenFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	originListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		logger.Error("demo origin listener failed", "error", err)
		os.Exit(1)
	}

	originServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = fmt.Fprintf(w, "demo upstream received %s %s\n", r.Method, r.URL.RequestURI())
		}),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	go func() {
		if err := originServer.Serve(originListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("demo origin stopped unexpectedly", "error", err)
		}
	}()

	originURL := &url.URL{Scheme: "http", Host: originListener.Addr().String()}
	upstream := httputil.NewSingleHostReverseProxy(originURL)
	handler, err := waf.NewCorazaHandler(upstream, mode)
	if err != nil {
		logger.Error("WAF initialization failed", "error", err)
		_ = originServer.Close()
		os.Exit(1)
	}

	httpListener, err := net.Listen("tcp", httpAddress)
	if err != nil {
		logger.Error("demo HTTP listener failed", "address", httpAddress, "error", err)
		_ = originServer.Close()
		os.Exit(1)
	}
	httpsListener, err := net.Listen("tcp", httpsAddress)
	if err != nil {
		logger.Error("demo HTTPS listener failed", "address", httpsAddress, "error", err)
		_ = httpListener.Close()
		_ = originServer.Close()
		os.Exit(1)
	}
	tlsConfig, err := newDemoTLSConfig()
	if err != nil {
		logger.Error("demo TLS initialization failed", "error", err)
		_ = httpsListener.Close()
		_ = httpListener.Close()
		_ = originServer.Close()
		os.Exit(1)
	}

	httpServer := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	httpsServer := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
		TLSConfig:         tlsConfig,
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(shutdown)

	serveResult := make(chan error, 2)
	go func() { serveResult <- httpServer.Serve(httpListener) }()
	go func() { serveResult <- httpsServer.ServeTLS(httpsListener, "", "") }()

	logger.Info("local WAF prototype started", "http", httpListener.Addr().String(), "https", httpsListener.Addr().String(), "mode", mode, "upstream", "in-process demo origin", "tlsCertificate", "ephemeral self-signed")
	select {
	case sig := <-shutdown:
		logger.Info("shutting down local WAF prototype", "signal", sig.String())
	case err := <-serveResult:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("demo listener stopped unexpectedly", "error", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("demo HTTP shutdown failed", "error", err)
	}
	if err := httpsServer.Shutdown(ctx); err != nil {
		logger.Error("demo HTTPS shutdown failed", "error", err)
	}
	if err := originServer.Shutdown(ctx); err != nil {
		logger.Error("demo origin shutdown failed", "error", err)
	}
}

func validateLoopbackAddress(address string) (string, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("invalid -listen address: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", errors.New("-listen must use a loopback IP address")
	}
	return address, nil
}
