package middleware

import (
	"time"

	"github.com/gofiber/fiber/v3"

	// Both live in the transport layer, and the metrics middleware genuinely needs
	// to know how an error becomes a status: handlers owns that mapping, so the
	// dependency points that way. handlers does not import middleware, so there is
	// no cycle — keep it that way.
	"github.com/thewhitewizard/go-service-template/internal/transport/http/handlers"
)

// RequestRecorder records one finished HTTP request.
//
// The interface is declared here, in the consumer, so the transport layer never
// imports the metrics implementation and therefore never imports prometheus.
// internal/observability.Metrics satisfies it.
type RequestRecorder interface {
	ObserveRequest(method, route string, status int, d time.Duration)
}

// UnmatchedRoute is the single "route" label value used for requests that
// matched no registered route.
//
// This constant is load-bearing, and the reason is a Fiber detail worth knowing:
// Ctx.Route() falls back to the RAW REQUEST PATH when nothing matched —
//
//	func (c *DefaultCtx) Route() *Route {
//	    if c.route == nil {
//	        // Fallback for fasthttp error handler
//	        return &Route{path: c.pathOriginal, Path: c.pathOriginal, ...}
//	    }
//	    return c.route
//	}
//
// so labelling with Route().Path alone is NOT bounded. Spraying /a1, /a2, /a3…
// against a service would create one time series per path and eventually take
// the metrics backend down — a denial of service that costs the attacker one
// HTTP request per series. The allowlist below is what actually bounds the label;
// using the route pattern is only half the fix.
const UnmatchedRoute = "unmatched"

// RouteAllowlist bounds the set of values the "route" label may take.
//
// Set is called once during server construction, after the routes are declared
// and before the server accepts traffic. The map is only read afterwards, so no
// synchronisation is needed — do not call Set on a serving server.
type RouteAllowlist struct {
	paths map[string]struct{}
}

// NewRouteAllowlist returns an empty allowlist. Until Set is called every route
// is reported as UnmatchedRoute: failing closed keeps cardinality bounded even
// if the wiring is wrong.
func NewRouteAllowlist() *RouteAllowlist {
	return &RouteAllowlist{paths: make(map[string]struct{})}
}

// Set replaces the allowed route patterns.
func (a *RouteAllowlist) Set(paths []string) {
	next := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		next[p] = struct{}{}
	}

	a.paths = next
}

// Label returns path if it is an allowed route pattern, and UnmatchedRoute
// otherwise.
func (a *RouteAllowlist) Label(path string) string {
	if _, ok := a.paths[path]; ok {
		return path
	}

	return UnmatchedRoute
}

// Metrics records every finished request through rec, labelling it with the
// registered route pattern rather than the request path.
func Metrics(rec RequestRecorder, allowed *RouteAllowlist) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		route := UnmatchedRoute
		if r := c.Route(); r != nil {
			route = allowed.Label(r.Path)
		}

		rec.ObserveRequest(c.Method(), route, statusOf(c, err), time.Since(start))

		return err
	}
}

// statusOf reports the status code the client will actually receive.
//
// c.Response().StatusCode() is not enough on the error path: when a handler
// returns an error, Fiber runs its ErrorHandler *after* this middleware has
// returned, so the response still carries the pre-error status. Reading it
// blindly would record a 200 for every failed request and make the error rate
// permanently flat — the one metric an alert depends on.
//
// The mapping goes through handlers.StatusFor, the same function the server's
// ErrorHandler uses to write the response. Duplicating the logic here is how a
// dashboard ends up reporting 500s for requests the client saw as 404s: the two
// would drift the first time a sentinel is added on one side only.
func statusOf(c fiber.Ctx, err error) int {
	if err == nil {
		return c.Response().StatusCode()
	}

	return handlers.StatusFor(err)
}
