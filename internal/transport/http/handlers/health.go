package handlers

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
)

// probeTimeout bounds each dependency probe, individually.
const probeTimeout = 2 * time.Second

// Probe reports whether one dependency this service needs is usable.
//
// Name appears in the /readyz response so an operator knows which dependency is
// at fault without opening the logs. Check must respect the context it is given:
// a probe that ignores cancellation turns a readiness check into a hang.
type Probe struct {
	Name  string
	Check func(ctx context.Context) error
}

// Health serves the /healthz (liveness) and /readyz (readiness) probes.
type Health struct {
	probes []Probe
	log    *slog.Logger
}

// NewHealth builds the health handler.
//
// Pass one Probe per external dependency the service cannot serve traffic
// without. Passing none is valid — see Ready — but it makes /readyz equivalent to
// /healthz, which the server logs a warning about at startup.
func NewHealth(log *slog.Logger, probes ...Probe) *Health {
	return &Health{probes: probes, log: log}
}

// ProbeCount returns how many readiness probes are registered.
func (h *Health) ProbeCount() int {
	return len(h.probes)
}

// Live reports that the process is running. It depends on nothing.
//
// Liveness must never probe a dependency: if it did, an outage of that dependency
// would fail the liveness check, the orchestrator would restart the pod, and the
// restart would fix nothing while destroying whatever the process still served.
// That is the difference between liveness and readiness, and it is the mistake
// worth not making.
func (h *Health) Live(c fiber.Ctx) error {
	return c.JSON(fiber.Map{statusKey: "ok"})
}

// Ready reports whether the service can serve traffic: every registered probe
// must succeed.
//
// With no probe registered, it reports ready. That is honest for a service with
// no external dependency, and the startup warning makes the situation visible
// rather than silent — a /readyz hardcoded to 200 removes the signal without
// telling anyone.
func (h *Health) Ready(c fiber.Ctx) error {
	for _, probe := range h.probes {
		// One budget per dependency, not a shared one: with a shared budget a slow
		// dependency would consume nearly all of it, and the next probe would fail
		// on an almost-expired context — blaming a dependency that is perfectly
		// healthy.
		ctx, cancel := context.WithTimeout(c.Context(), probeTimeout)

		err := probe.Check(ctx)

		cancel()

		if err != nil {
			h.log.Warn("readiness probe failed",
				slog.String("dependency", probe.Name),
				slog.Any("error", err),
			)

			return h.unavailable(c, probe.Name)
		}
	}

	return c.JSON(fiber.Map{
		statusKey: "ready",
		"probes":  len(h.probes),
	})
}

// unavailable answers 503, naming the dependency at fault.
func (h *Health) unavailable(c fiber.Ctx, dependency string) error {
	return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
		statusKey:    "unavailable",
		"dependency": dependency,
	})
}
