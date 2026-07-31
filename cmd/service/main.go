// Command service starts the HTTP service.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/thewhitewizard/go-service-template/internal/config"
	"github.com/thewhitewizard/go-service-template/internal/observability"
	httptransport "github.com/thewhitewizard/go-service-template/internal/transport/http"
)

func main() {
	if err := run(); err != nil {
		slog.Error("stopping on error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := newLogger(cfg.LogLevel)
	slog.SetDefault(log)

	// signal.NotifyContext: SIGINT/SIGTERM cancel ctx and trigger graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	metrics := observability.NewMetrics()

	srv := httptransport.New(httptransport.Deps{
		Config:  cfg,
		Log:     log,
		Metrics: metrics,

		// Register one probe per external dependency the service cannot serve
		// traffic without, for example:
		//
		//	Probes: []handlers.Probe{
		//	    {Name: "database", Check: store.Ping},
		//	    {Name: "cache", Check: cache.Ping},
		//	},
		//
		// Leaving this empty makes /readyz equivalent to /healthz, and the server
		// logs a warning saying so at startup.
		Probes: nil,
	})

	errCh := make(chan error, 1)

	go func() {
		log.Info("starting service", slog.String("addr", cfg.ListenAddr), slog.Any("config", cfg))

		errCh <- srv.Listen()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutdown signal received, draining")

		// A fresh context on purpose: ctx is already cancelled, so reusing it
		// would give the graceful shutdown no time at all.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		return srv.Shutdown(shutdownCtx)
	}
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level

	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
