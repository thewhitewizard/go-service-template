// Package http assembles the Fiber server: it is the only layer that knows the
// web framework. The domain and every future service package deal only with
// context.Context and domain structs, which internal/arch enforces.
package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gofiber/fiber/v3/middleware/recover"

	"github.com/thewhitewizard/go-service-template/internal/config"
	"github.com/thewhitewizard/go-service-template/internal/transport/http/handlers"
	"github.com/thewhitewizard/go-service-template/internal/transport/http/middleware"
)

// Deps groups the server dependencies. A struct rather than a parameter list
// because the next layers will add to it.
type Deps struct {
	Config  config.Config
	Log     *slog.Logger
	Metrics Recorder

	// Probes are the readiness probes, one per external dependency. Empty is
	// valid and produces a startup warning.
	Probes []handlers.Probe
}

// Recorder is what the server needs from the metrics implementation: record a
// request, and serve the exposition endpoint. Declared here so this package does
// not import internal/observability, and therefore never reaches prometheus.
type Recorder interface {
	middleware.RequestRecorder

	Handler() http.Handler
}

// Server wraps the Fiber application and its lifecycle.
type Server struct {
	app *fiber.App
	cfg config.Config

	// baseCtx is the root of any long-lived work started by a handler. It is
	// cancelled on Shutdown so in-flight work is released before the server stops
	// accepting.
	//nolint:containedctx // long-lived root for in-flight work, cancelled on Shutdown
	baseCtx    context.Context
	baseCancel context.CancelFunc
}

// New wires middlewares, routes and handlers onto the dependencies.
//
// A missing dependency panics here, at wiring time. Moving to a struct loses the
// check the compiler gave us on a parameter list: without this, a nil field would
// only surface on the first request to the affected route, as a generic 500
// produced by the recover middleware — far from its cause.
func New(deps Deps) *Server {
	switch {
	case deps.Log == nil:
		panic("http: Deps.Log is nil")
	case deps.Metrics == nil:
		panic("http: Deps.Metrics is nil")
	}

	baseCtx, baseCancel := context.WithCancel(context.Background())

	app := fiber.New(fiber.Config{
		ReadTimeout:  deps.Config.ReadTimeout,
		IdleTimeout:  deps.Config.IdleTimeout,
		WriteTimeout: deps.Config.WriteTimeout,
		AppName:      "go-service-template",
	})

	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(_ fiber.Ctx, e any) {
			deps.Log.Error("recovered panic", slog.Any("panic", e), slog.String("stack", string(debug.Stack())))
		},
		PanicHandler: func(_ fiber.Ctx, _ any) error {
			// Deliberately opaque: a panic value can quote request data or a
			// secret, and it must not reach the client.
			return fiber.NewError(fiber.StatusInternalServerError, "internal error")
		},
	}))
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(deps.Log))

	// The allowlist is created before the routes exist and filled in below, once
	// they are all declared. Until then it reports every route as unmatched,
	// which fails closed on cardinality.
	routes := middleware.NewRouteAllowlist()
	app.Use(middleware.Metrics(deps.Metrics, routes))

	health := handlers.NewHealth(deps.Log, deps.Probes...)

	app.Get(handlers.PathHealthz, health.Live)
	app.Get(handlers.PathReadyz, health.Ready)
	app.Get(handlers.PathMetrics, adaptor.HTTPHandler(deps.Metrics.Handler()))

	// Add your routes above this line.

	routes.Set(registeredPaths(app))

	if health.ProbeCount() == 0 {
		deps.Log.Warn("no readiness probes registered: /readyz reports liveness only",
			slog.String("fix", "register one handlers.Probe per external dependency in cmd/service/main.go"),
		)
	}

	return &Server{
		app:        app,
		cfg:        deps.Config,
		baseCtx:    baseCtx,
		baseCancel: baseCancel,
	}
}

// registeredPaths returns the route patterns declared on the application.
//
// It is what bounds the "route" metric label: see middleware.UnmatchedRoute for
// why the pattern reported by Fiber cannot be trusted on its own.
func registeredPaths(app *fiber.App) []string {
	all := app.GetRoutes(true)

	paths := make([]string, 0, len(all))
	for _, r := range all {
		paths = append(paths, r.Path)
	}

	return paths
}

// Listen starts the server and blocks until it stops.
func (s *Server) Listen() error {
	if err := s.app.Listen(s.cfg.ListenAddr, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
		return fmt.Errorf("http: listen on %s: %w", s.cfg.ListenAddr, err)
	}

	return nil
}

// Shutdown cancels in-flight work, then stops the server gracefully.
//
// The order matters: cancelling baseCtx first lets handlers unwind on their own,
// so ShutdownWithContext waits for work that is already finishing rather than for
// work that has no reason to stop.
func (s *Server) Shutdown(ctx context.Context) error {
	s.baseCancel()

	if err := s.app.ShutdownWithContext(ctx); err != nil {
		return fmt.Errorf("http: graceful shutdown: %w", err)
	}

	return nil
}
