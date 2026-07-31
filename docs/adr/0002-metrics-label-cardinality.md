# ADR-0002 — Bound the `route` metric label with an allowlist of registered patterns

- **Status**: Accepted
- **Date**: 2026-07-31
- **Phase**: 0 (template baseline)

## Context

The service exposes RED metrics per HTTP route. The label carrying the route determines
how many time series the service produces, and that number is the difference between a
useful dashboard and a dead monitoring backend.

Labelling with the raw request path is unbounded by construction: `/items/1`,
`/items/2`, … each create their own series. The usual advice is therefore to label with
the *registered route pattern* instead.

**In Fiber that advice is only half a fix, and the missing half is the dangerous one.**
`Ctx.Route()` falls back to the raw request path when no route matched:

```go
func (c *DefaultCtx) Route() *Route {
    if c.route == nil {
        // Fallback for fasthttp error handler
        return &Route{path: c.pathOriginal, Path: c.pathOriginal, ...}
    }
    return c.route
}
```

So an unauthenticated caller spraying `/a1`, `/a2`, `/a3`… creates one time series per
request. That is a denial of service costing the attacker a single HTTP request per
series, against a component that is usually not the one being defended.

## Decision

Label with the registered route pattern **filtered through an allowlist** of the patterns
actually registered on the application, built at construction time from
`app.GetRoutes(true)`. Anything not in the allowlist — every unmatched request — collapses
into the single constant value `unmatched`.

The allowlist starts empty and is filled after the routes are declared, so it **fails
closed**: until it is populated, every route reports `unmatched`.

## Options considered

- **Label with `c.Path()`** — full detail per request. What is lost: any bound on
  cardinality. Rejected outright.
- **Label with `c.Route().Path` alone** — bounded for matched requests, unbounded for
  404s because of the fallback above. What is lost: the guarantee, precisely on the path
  an attacker controls. This is the option that looks correct and is not.
- **Allowlist of registered patterns** (chosen) — bounded by construction, since the value
  set is the finite set of declared routes plus one constant. What is lost: no per-path
  detail on unmatched requests, so a 404 flood shows as volume without showing which paths
  were probed.
- **Drop the route label entirely** — trivially bounded. What is lost: the ability to tell
  which endpoint is slow or failing, which is most of the value of RED metrics.

## Consequences

- Cardinality of `http_requests_total` is bounded by
  `len(routes) + 1` × methods × statuses, all three of which are finite and known.
- **Unmatched paths are indistinguishable from each other in metrics.** Investigating a
  404 flood goes through the logs, which record `c.Path()` — a log line is one event and
  can afford the detail, whereas a metric label multiplies series. The two are
  deliberately not symmetric.
- The allowlist must be refilled if routes are ever registered after construction. Doing
  that on a serving server would also be a data race on the map, which is why `Set` is
  documented as construction-time only.
- `http_request_duration_seconds` carries no `status` label: a histogram costs
  `len(buckets)+2` series per label combination, so multiplying it by the status set buys
  detail nobody queries at a cost everybody pays.

## Mitigations

- `internal/arch` confines `github.com/prometheus` to `internal/observability`, so the
  metric definitions — and their label sets — live in one reviewable place.
- The `route` label value is computed in exactly one function,
  `middleware.RouteAllowlist.Label`.
- Histogram buckets are declared explicitly with the reasoning attached, rather than
  inherited from `prometheus.DefBuckets`, which is calibrated for short requests.

## Verification

`TestMetricsRouteLabelIsBounded` in `internal/transport/http/middleware` fails if either
half of the fix is removed. It asserts both the **series count** (two parameterised paths
collapse to one series, three unmatched paths collapse to one) and the **label values**
(`route="/items/:id"` present, `route="/items/1"` absent) — because the count alone stays
at 2 when the allowlist is missing, and only the value assertion catches that.

Verify by replacing `r.Path` with `c.Path()` in `middleware.Metrics`: the test must fail.
