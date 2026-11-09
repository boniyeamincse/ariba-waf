// Command waf-demo runs an isolated HTTP-only Coraza/CRS prototype.
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

const defaultProxyAddress = "127.0.0.1:18080"

func main() {
	modeFlag := flag.String("mode", string(waf.ModeDetection), "WAF mode: off, detectionOnly, or blocking")
	listenFlag := flag.String("listen", defaultProxyAddress, "loopback listen address for the local demo")
	flag.Parse()

	mode := waf.Mode(*modeFlag)
	if mode != waf.ModeOff && mode != waf.ModeDetection && mode != waf.ModeBlocking {
		fmt.Fprintln(os.Stderr, "invalid -mode; choose off, detectionOnly, or blocking")
		os.Exit(2)
	}
	proxyAddress, err := validateLoopbackAddress(*listenFlag)
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

	proxyListener, err := net.Listen("tcp", proxyAddress)
	if err != nil {
		logger.Error("demo proxy listener failed", "address", proxyAddress, "error", err)
		_ = originServer.Close()
		os.Exit(1)
	}
	proxyServer := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(shutdown)

	serveResult := make(chan error, 1)
	go func() { serveResult <- proxyServer.Serve(proxyListener) }()

	logger.Info("local WAF prototype started", "address", proxyListener.Addr().String(), "mode", mode, "upstream", "in-process demo origin")
	select {
	case sig := <-shutdown:
		logger.Info("shutting down local WAF prototype", "signal", sig.String())
	case err := <-serveResult:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("demo proxy stopped unexpectedly", "error", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := proxyServer.Shutdown(ctx); err != nil {
		logger.Error("demo proxy shutdown failed", "error", err)
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
