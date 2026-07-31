package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thewhitewizard/go-service-template/internal/observability"
	"github.com/thewhitewizard/go-service-template/internal/transport/http/middleware"
)

// routePaths mirrors what the server does when it fills the allowlist.
func routePaths(app *fiber.App) []string {
	routes := app.GetRoutes(true)

	paths := make([]string, 0, len(routes))
	for _, r := range routes {
		paths = append(paths, r.Path)
	}

	return paths
}

func newMetricsApp(t *testing.T, fillAllowlist bool) (*fiber.App, *observability.Metrics) {
	t.Helper()

	metrics := observability.NewMetrics()
	app := fiber.New()
	allow := middleware.NewRouteAllowlist()

	app.Use(middleware.Metrics(metrics, allow))

	ok := func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }
	app.Get("/healthz", ok)
	app.Get("/items/:id", ok)

	if fillAllowlist {
		allow.Set(routePaths(app))
	}

	return app, metrics
}

func get(t *testing.T, app *fiber.App, path string) {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
}

// exposition renders the /metrics body, which is how we inspect label *values*
// without importing prometheus types outside internal/observability.
func exposition(t *testing.T, metrics *observability.Metrics) string {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", nil)

	metrics.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)

	return string(body)
}

// TestMetricsRouteLabelIsBounded is the negative test of the cardinality
// guardrail. It must fail if anyone:
//
//   - labels with c.Path() instead of the registered route pattern, or
//   - forgets to fill the route allowlist.
//
// Without it, the guardrail is a comment. Both mistakes are silent in production
// until the metrics backend falls over.
func TestMetricsRouteLabelIsBounded(t *testing.T) {
	t.Parallel()

	app, metrics := newMetricsApp(t, true)

	require.Equal(t, 0, metrics.CountSeries("http_requests_total"),
		"no request observed yet")

	// Two requests on the same pattern with different parameters must collapse
	// into a single series.
	get(t, app, "/items/1")
	get(t, app, "/items/2")
	require.Equal(t, 1, metrics.CountSeries("http_requests_total"),
		"/items/1 and /items/2 share one route pattern and must share one series")

	// Three unmatched paths must collapse into the single "unmatched" value.
	// This is the denial-of-service case: one HTTP request per new time series.
	get(t, app, "/nope/1")
	get(t, app, "/nope/2")
	get(t, app, "/nope/3")
	assert.Equal(t, 2, metrics.CountSeries("http_requests_total"),
		"unmatched paths must all fold into %q", middleware.UnmatchedRoute)

	// Counting series is not enough: with no allowlist, matched routes would also
	// report "unmatched" and the count would still be 2. Assert the label values.
	body := exposition(t, metrics)
	assert.Contains(t, body, `route="/items/:id"`,
		"a matched route must be labelled with its registered pattern")
	assert.Contains(t, body, `route="`+middleware.UnmatchedRoute+`"`)
	assert.NotContains(t, body, `route="/items/1"`,
		"the raw request path must never reach a metric label")
	assert.NotContains(t, body, `route="/nope/1"`)
}

// TestMetricsWithoutAllowlistFailsClosed pins the deliberate choice that an
// unfilled allowlist reports every route as unmatched. Losing per-route detail is
// a monitoring gap; an unbounded label is an outage.
func TestMetricsWithoutAllowlistFailsClosed(t *testing.T) {
	t.Parallel()

	app, metrics := newMetricsApp(t, false)

	get(t, app, "/items/1")

	body := exposition(t, metrics)
	assert.Contains(t, body, `route="`+middleware.UnmatchedRoute+`"`)
	assert.NotContains(t, body, `route="/items/:id"`)
}

func TestRouteAllowlistLabel(t *testing.T) {
	t.Parallel()

	allow := middleware.NewRouteAllowlist()
	allow.Set([]string{"/healthz", "/items/:id"})

	tests := map[string]string{
		"/healthz":    "/healthz",
		"/items/:id":  "/items/:id",
		"/items/42":   middleware.UnmatchedRoute,
		"/unknown":    middleware.UnmatchedRoute,
		"":            middleware.UnmatchedRoute,
		"/healthz/..": middleware.UnmatchedRoute,
	}

	for in, want := range tests {
		assert.Equal(t, want, allow.Label(in), "Label(%q)", in)
	}
}

// TestMetricsRecordsErrorStatus pins that a handler returning an error is counted
// with the status the client receives, not with the status the response object
// still carries when the middleware unwinds. Getting this wrong makes the error
// rate permanently flat.
func TestMetricsRecordsErrorStatus(t *testing.T) {
	t.Parallel()

	metrics := observability.NewMetrics()
	app := fiber.New()
	allow := middleware.NewRouteAllowlist()

	app.Use(middleware.Metrics(metrics, allow))
	app.Get("/boom", func(_ fiber.Ctx) error {
		return fiber.NewError(fiber.StatusTeapot, "boom")
	})
	allow.Set(routePaths(app))

	get(t, app, "/boom")

	body := exposition(t, metrics)
	assert.Contains(t, body, `status="418"`, "the recorded status must be the one the client sees")
	assert.NotContains(t, body, `route="/boom",status="200"`,
		"an errored request must not be counted as a success")
}
