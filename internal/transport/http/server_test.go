package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thewhitewizard/go-service-template/internal/config"
	"github.com/thewhitewizard/go-service-template/internal/domain"
	"github.com/thewhitewizard/go-service-template/internal/observability"
	"github.com/thewhitewizard/go-service-template/internal/transport/http/handlers"
)

// testTimeout gives app.Test a generous budget: the default is short enough to
// make a slow CI runner look like a broken handler.
const testTimeout = 5 * time.Second

func testConfig() config.Config {
	return config.Config{
		ListenAddr:      ":0",
		ReadTimeout:     15 * time.Second,
		IdleTimeout:     120 * time.Second,
		WriteTimeout:    30 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		LogLevel:        "info",
	}
}

// newTestServer returns a server plus the buffer holding everything it logged, so
// tests can assert on startup diagnostics as well as on responses.
func newTestServer(t *testing.T, probes ...handlers.Probe) (*Server, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer

	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	srv := New(Deps{
		Config:  testConfig(),
		Log:     log,
		Metrics: observability.NewMetrics(),
		Probes:  probes,
	})

	return srv, &buf
}

func request(t *testing.T, srv *Server, path string) *http.Response {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)

	resp, err := srv.app.Test(req, fiber.TestConfig{Timeout: testTimeout})
	require.NoError(t, err)

	return resp
}

// TestNewPanicsOnMissingDependency pins the wiring contract: moving to a Deps
// struct loses the check the compiler gave us on a parameter list, so a nil field
// must fail at construction. Without this, it would only surface as a generic 500
// on the first request to the affected route.
func TestNewPanicsOnMissingDependency(t *testing.T) {
	t.Parallel()

	tests := map[string]Deps{
		"nil log": {
			Config:  testConfig(),
			Metrics: observability.NewMetrics(),
		},
		"nil metrics": {
			Config: testConfig(),
			Log:    slog.New(slog.DiscardHandler),
		},
	}

	for name, deps := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Panics(t, func() { New(deps) })
		})
	}
}

func TestServiceContractRoutes(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)

	tests := map[string]int{
		handlers.PathHealthz: fiber.StatusOK,
		handlers.PathReadyz:  fiber.StatusOK,
		handlers.PathMetrics: fiber.StatusOK,
	}

	for path, wantStatus := range tests {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			resp := request(t, srv, path)

			defer func() { assert.NoError(t, resp.Body.Close()) }()

			assert.Equal(t, wantStatus, resp.StatusCode)
		})
	}
}

// TestMetricsEndpointServesExposition checks the endpoint actually exposes the
// service's own collectors, not just an empty 200. A /metrics that answers but
// publishes nothing is the same failure as a hardcoded /readyz: a signal that
// looks present and is not.
func TestMetricsEndpointServesExposition(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)

	// One request first, so the HTTP collectors have something to report.
	warm := request(t, srv, handlers.PathHealthz)
	require.NoError(t, warm.Body.Close())

	resp := request(t, srv, handlers.PathMetrics)

	defer func() { assert.NoError(t, resp.Body.Close()) }()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	body := string(raw)

	assert.Contains(t, body, "http_requests_total")
	assert.Contains(t, body, "http_request_duration_seconds")
	assert.Contains(t, body, "go_goroutines", "the Go collector must be registered")
	assert.Contains(t, body, `route="`+handlers.PathHealthz+`"`,
		"a matched route must be labelled with its registered pattern")
}

// TestNewWarnsWhenNoReadinessProbe is the guard on the deliberate compromise of
// this template: /readyz answers 200 with no dependency to probe, which is honest
// only if the situation is stated out loud. Silence here would make the template
// ship the "hardcoded 200" antipattern by default.
func TestNewWarnsWhenNoReadinessProbe(t *testing.T) {
	t.Parallel()

	_, logs := newTestServer(t)

	assert.Contains(t, logs.String(), "no readiness probes registered",
		"a service with no probe must say so at startup")
}

func TestNewDoesNotWarnWhenProbeRegistered(t *testing.T) {
	t.Parallel()

	probe := handlers.Probe{Name: "database", Check: func(context.Context) error { return nil }}

	_, logs := newTestServer(t, probe)

	assert.NotContains(t, logs.String(), "no readiness probes registered")
}

// TestRequestIDIsEchoed covers the middleware chain being wired at all: the header
// only comes back if RequestID ran before the handler.
func TestRequestIDIsEchoed(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)

	resp := request(t, srv, handlers.PathHealthz)

	defer func() { assert.NoError(t, resp.Body.Close()) }()

	assert.NotEmpty(t, resp.Header.Get("X-Request-ID"))
}

// TestUnmatchedRouteIsNotFound also documents that a 404 is counted, not dropped:
// the metrics middleware sits above the router.
func TestUnmatchedRouteIsNotFound(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)

	resp := request(t, srv, "/does-not-exist")

	defer func() { assert.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// TestDomainErrorsAreMappedToStatus is the test that makes handlers.StatusFor
// load-bearing rather than decorative.
//
// The mapping existed from the start, but nothing called it and fiber.Config
// left ErrorHandler unset — so Fiber's DefaultErrorHandler applied and every
// domain sentinel became a 500. CLAUDE.md promised a centralised mapping the code
// did not perform, and the first developer to follow the documented rule would
// have got a silently wrong status.
//
// Remove the ErrorHandler from server.go and this test goes red.
func TestDomainErrorsAreMappedToStatus(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err        error
		wantStatus int
	}{
		"not found":     {domain.ErrNotFound, fiber.StatusNotFound},
		"invalid input": {domain.ErrInvalidInput, fiber.StatusBadRequest},
		"unauthorized":  {domain.ErrUnauthorized, fiber.StatusUnauthorized},
		"conflict":      {domain.ErrConflict, fiber.StatusConflict},
		"unavailable":   {domain.ErrUnavailable, fiber.StatusServiceUnavailable},
		// A wrapped sentinel must map the same way: handlers use %w to add context,
		// and errors.Is has to see through it.
		"wrapped not found": {
			fmt.Errorf("loading user 42: %w", domain.ErrNotFound),
			fiber.StatusNotFound,
		},
		// Anything unrecognised is a server fault, not a client one.
		"unknown error": {errors.New("something broke"), fiber.StatusInternalServerError},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			srv, _ := newTestServer(t)
			srv.app.Get("/boom", func(_ fiber.Ctx) error { return tc.err })

			resp := request(t, srv, "/boom")

			defer func() { assert.NoError(t, resp.Body.Close()) }()

			assert.Equal(t, tc.wantStatus, resp.StatusCode)
		})
	}
}

// TestErrorBodyDoesNotLeakTheErrorMessage guards the security half of the same
// fix. Fiber's DefaultErrorHandler writes err.Error() into the response body, so
// a wrapped error carrying a DSN, a token or a signed URL would be handed to the
// caller. Only the generic reason phrase may go out.
func TestErrorBodyDoesNotLeakTheErrorMessage(t *testing.T) {
	t.Parallel()

	// gosec is right that this looks like a credential — that is the payload under
	// test. The assertion below is precisely that a string of this shape must not
	// reach the response body.
	//nolint:gosec // fake credential on purpose: it is what the test checks does not leak
	const secret = "postgres://user:hunter2@db.internal:5432/app"

	srv, logs := newTestServer(t)
	srv.app.Get("/boom", func(_ fiber.Ctx) error {
		return fmt.Errorf("connecting to %s: %w", secret, domain.ErrUnavailable)
	})

	resp := request(t, srv, "/boom")

	defer func() { assert.NoError(t, resp.Body.Close()) }()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)
	assert.NotContains(t, string(raw), "hunter2", "the response body must not carry the error text")
	assert.NotContains(t, string(raw), secret)
	assert.Contains(t, string(raw), http.StatusText(fiber.StatusServiceUnavailable))

	// The operator still needs the real cause: it belongs in the log, which is not
	// handed to the caller.
	assert.Contains(t, logs.String(), "request failed")
	assert.Contains(t, logs.String(), "hunter2", "the full error must reach the logs")
}

// TestMetricStatusMatchesResponseStatus is the anti-divergence test. The response
// status and the recorded status come from the same function on purpose; if one
// side ever computes it independently, they drift and the dashboard reports 500s
// for requests the client saw as 404s.
func TestMetricStatusMatchesResponseStatus(t *testing.T) {
	t.Parallel()

	metrics := observability.NewMetrics()

	var buf bytes.Buffer

	srv := New(Deps{
		Config:  testConfig(),
		Log:     slog.New(slog.NewJSONHandler(&buf, nil)),
		Metrics: metrics,
	})
	srv.app.Get("/boom", func(_ fiber.Ctx) error { return domain.ErrNotFound })

	resp := request(t, srv, "/boom")
	require.NoError(t, resp.Body.Close())
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	metricsResp := request(t, srv, handlers.PathMetrics)

	defer func() { assert.NoError(t, metricsResp.Body.Close()) }()

	raw, err := io.ReadAll(metricsResp.Body)
	require.NoError(t, err)

	assert.Contains(t, string(raw), `status="404"`,
		"the metric must record the status the client received")
	// Not `route="/boom",status="500"`: this route is registered after New filled the
	// allowlist, so its label is "unmatched" and that assertion could never fail.
	// Asserting on the status alone is what discriminates — sabotaging statusOf makes
	// a status="500" series appear here.
	assert.NotContains(t, string(raw), `status="500"`,
		"no request faulted, so no 5xx series may exist")
}

// TestShutdownCancelsBaseContext pins the shutdown order: in-flight work is
// released before the server stops accepting, so ShutdownWithContext waits for
// work that is already unwinding rather than for work with no reason to stop.
func TestShutdownCancelsBaseContext(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)

	require.NoError(t, srv.baseCtx.Err(), "base context must be live before shutdown")

	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()

	require.NoError(t, srv.Shutdown(ctx))
	assert.ErrorIs(t, srv.baseCtx.Err(), context.Canceled)
}

// TestLogsAreStructuredJSON guards the log format the whole pipeline depends on:
// a stray fmt.Println or a text handler would break every downstream parser.
func TestLogsAreStructuredJSON(t *testing.T) {
	t.Parallel()

	srv, logs := newTestServer(t)

	resp := request(t, srv, handlers.PathHealthz)
	require.NoError(t, resp.Body.Close())

	for line := range strings.Lines(strings.TrimSpace(logs.String())) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry map[string]any
		require.NoErrorf(t, json.Unmarshal([]byte(line), &entry), "log line is not JSON: %s", line)
	}
}
