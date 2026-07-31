package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thewhitewizard/go-service-template/internal/transport/http/handlers"
)

func newHealthApp(t *testing.T, probes ...handlers.Probe) *fiber.App {
	t.Helper()

	log := slog.New(slog.DiscardHandler)
	h := handlers.NewHealth(log, probes...)

	app := fiber.New()
	app.Get(handlers.PathHealthz, h.Live)
	app.Get(handlers.PathReadyz, h.Ready)

	return app
}

func call(t *testing.T, app *fiber.App, path string) (int, map[string]any) {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)

	resp, err := app.Test(req)
	require.NoError(t, err)

	defer func() { assert.NoError(t, resp.Body.Close()) }()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))

	return resp.StatusCode, body
}

// TestLiveIgnoresProbes pins the difference between liveness and readiness: a
// liveness check that probed a dependency would turn a dependency outage into a
// restart loop, which fixes nothing and destroys what the process still served.
func TestLiveIgnoresProbes(t *testing.T) {
	t.Parallel()

	called := false
	probe := handlers.Probe{
		Name: "must-not-be-called",
		Check: func(context.Context) error {
			called = true

			return errors.New("liveness must not probe dependencies")
		},
	}

	status, body := call(t, newHealthApp(t, probe), handlers.PathHealthz)

	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, "ok", body["status"])
	assert.False(t, called, "Live must not run readiness probes")
}

// TestReadyWithoutProbes documents the template's default state: ready, and the
// probe count exposed so the answer is self-describing rather than a bare 200.
func TestReadyWithoutProbes(t *testing.T) {
	t.Parallel()

	status, body := call(t, newHealthApp(t), handlers.PathReadyz)

	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, "ready", body["status"])
	assert.InDelta(t, 0.0, body["probes"], 0.0)
}

func TestReadyWithPassingProbes(t *testing.T) {
	t.Parallel()

	pass := func(name string) handlers.Probe {
		return handlers.Probe{Name: name, Check: func(context.Context) error { return nil }}
	}

	status, body := call(t, newHealthApp(t, pass("database"), pass("cache")), handlers.PathReadyz)

	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, "ready", body["status"])
	assert.InDelta(t, 2.0, body["probes"], 0.0)
}

// TestReadyNamesTheFailingDependency is the point of naming probes: an operator
// must learn *which* dependency is down from the response, without opening logs.
func TestReadyNamesTheFailingDependency(t *testing.T) {
	t.Parallel()

	ok := handlers.Probe{Name: "database", Check: func(context.Context) error { return nil }}
	broken := handlers.Probe{
		Name:  "cache",
		Check: func(context.Context) error { return errors.New("connection refused") },
	}

	status, body := call(t, newHealthApp(t, ok, broken), handlers.PathReadyz)

	assert.Equal(t, fiber.StatusServiceUnavailable, status)
	assert.Equal(t, "unavailable", body["status"])
	assert.Equal(t, "cache", body["dependency"])
}

// TestReadyProbeGetsADeadline pins the per-dependency budget: each probe receives
// a context with its own deadline, never one already consumed by a slow neighbour.
func TestReadyProbeGetsADeadline(t *testing.T) {
	t.Parallel()

	var deadlines []bool

	record := func(name string) handlers.Probe {
		return handlers.Probe{
			Name: name,
			Check: func(ctx context.Context) error {
				_, ok := ctx.Deadline()
				deadlines = append(deadlines, ok)

				return nil
			},
		}
	}

	status, _ := call(t, newHealthApp(t, record("first"), record("second")), handlers.PathReadyz)

	require.Equal(t, fiber.StatusOK, status)
	require.Len(t, deadlines, 2)
	assert.Equal(t, []bool{true, true}, deadlines, "every probe must get its own deadline")
}
