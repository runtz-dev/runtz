package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/runtz-dev/runtz/engine/internal/api"
	"github.com/runtz-dev/runtz/engine/internal/config"
	"github.com/runtz-dev/runtz/engine/internal/telemetry"
	"github.com/runtz-dev/runtz/engine/internal/version"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid runtz engine configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Before api.New: the Mongo client it builds picks up the global tracer
	// provider at construction time, so the providers have to be installed
	// first for database spans to land anywhere.
	shutdownTelemetry, err := telemetry.Setup(ctx, telemetry.Service{
		Name:    "runtz-engine",
		Version: version.Version,
	})
	if err != nil {
		slog.Error("failed to start telemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		// Its own context: ctx is already cancelled by the time we get here,
		// and a cancelled context would drop the final batch of spans.
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(flushCtx); err != nil {
			slog.Warn("failed to flush telemetry", "error", err)
		}
	}()

	server, err := api.New(ctx, cfg)
	if err != nil {
		slog.Error("failed to start runtz engine", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Close(shutdownCtx); err != nil {
			slog.Warn("failed to close mongo connection", "error", err)
		}
	}()

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.Port),
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("runtz engine listening", "addr", httpServer.Addr, "version", version.Version)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("http shutdown failed", "error", err)
		os.Exit(1)
	}
}
