package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/boniyeamincse/ariba-waf/internal/api"
	"github.com/boniyeamincse/ariba-waf/internal/management"
)

const listenAddress = "127.0.0.1:9000"

func main() {
	databasePath := flag.String("db", "dev-data/ariba.db", "SQLite database in a private directory")
	origin := flag.String("origin", "https://localhost:9443", "exact browser management origin")
	devHTTP := flag.Bool("dev-http", false, "permit development cookies for a loopback HTTP browser origin")
	enrollmentToken := flag.Bool("enrollment-token", false, "generate a single-use 10-minute setup token and exit")
	listen := flag.String("listen", listenAddress, "management listen address (IP literal)")
	allowRemote := flag.Bool("allow-remote-listen", false, "allow a non-loopback container listener; restrict published ports to host loopback")
	healthcheck := flag.Bool("healthcheck", false, "probe local liveness and exit without opening the database")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if *healthcheck {
		if err := probeLiveness("http://127.0.0.1:9000/health/live"); err != nil {
			logger.Error("liveness check failed", "error", err)
			os.Exit(1)
		}
		return
	}
	address, err := validateListenAddress(*listen, *allowRemote)
	if err != nil {
		logger.Error("management listen configuration invalid", "error", err)
		os.Exit(1)
	}
	store, err := management.Open(*databasePath)
	if err != nil {
		logger.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	if *enrollmentToken {
		token, err := store.CreateEnrollmentToken(context.Background())
		if err != nil {
			logger.Error("enrollment token creation failed", "error", err)
			os.Exit(1)
		}
		fmt.Println(token)
		return
	}
	managementHandler, err := management.NewHandler(store, management.Config{Origin: *origin, DevHTTP: *devHTTP})
	if err != nil {
		logger.Error("management configuration invalid", "error", err)
		os.Exit(1)
	}
	mux := http.NewServeMux()
	mux.Handle("/{$}", api.NewLandingHandler())
	mux.Handle("/api/v1/", managementHandler)
	mux.Handle("/health/", api.NewHealthHandler(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := store.Ping(ctx); err != nil {
			return err
		}
		return errors.New("WAF activation dependencies are not configured")
	}))

	server := &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		logger.Error("management API listener failed", "error", err)
		os.Exit(1)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(shutdown)

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("management API shutdown failed", "error", err)
		}
	}()

	logger.Info("management API starting", "address", address)
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("management API stopped unexpectedly", "error", err)
		os.Exit(1)
	}
	// Serve stops accepting before Shutdown finishes draining existing requests.
	// Keep the account database open until those requests finish.
	<-shutdownDone
}
