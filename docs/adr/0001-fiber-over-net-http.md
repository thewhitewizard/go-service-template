# ADR-0001 — Fiber v3 rather than net/http, confined to the transport layer

- **Status**: Accepted
- **Date**: 2026-07-31
- **Phase**: 0 (template baseline)

## Context

The service needs an HTTP layer. Go's standard library is sufficient and has no
dependency; Fiber is faster on high request rates because it sits on fasthttp, but
fasthttp does not implement `net/http` and brings its own object model.

The decision matters beyond the router: whichever is chosen, its types spread through
handlers and middlewares, and if they leak further they reach the domain — at which point
the choice is no longer reversible.

## Decision

Use **Fiber v3**, and confine it — together with its transitive dependency `fasthttp` — to
`internal/transport/http/`. The domain and every service package deal only with
`context.Context` and domain structs. This is enforced by a test, not by a convention:
`internal/arch/arch_test.go`.

## Options considered

- **`net/http` (stdlib)** — no dependency, universal ecosystem, `http.Handler`
  compatible with every middleware ever written in Go. Slower under very high request
  rates. What is lost: a measurable amount of throughput at the top end.
- **Fiber v3 (fasthttp)** — higher throughput, ergonomic API. What is lost: incompatible
  with the `net/http` middleware ecosystem, so third-party handlers need an adaptor;
  `fasthttp` reuses buffers between requests, which introduces a class of bug that does
  not exist in the stdlib; and one more dependency to follow.
- **Fiber unconfined** — the version that costs nothing today and everything later: with
  `fiber.Ctx` in the domain, replacing the framework means rewriting the service.

## Consequences

- Throughput is better than the stdlib on the request-rate dimension.
- **`fasthttp` reuses memory between requests**: `c.Body()`, `c.Params()` and `c.Get()`
  return data invalidated once the handler returns. Any asynchronous use must copy first.
  This is the single most likely bug class in this layer, and it is why it is the first
  entry in `go-reviewer`'s checklist.
- Any `net/http` handler — including `promhttp` for `/metrics` — needs
  `adaptor.HTTPHandler`.
- **The confinement is what keeps the decision reversible.** Swapping to `net/http` stays
  a rewrite of one package instead of a rewrite of the service.

## Mitigations

- The architecture test forbids `github.com/gofiber` **and** `github.com/valyala/fasthttp`
  outside `internal/transport/http`, in test files too — a test that imports the framework
  from the domain crosses the boundary just as effectively as production code, and the
  exemption would be invisible in review.
- The rule covers the whole `github.com/gofiber` namespace, not just the `fiber` module:
  covering only the top-level dependency would leave the boundary escapable through
  `gofiber/utils`.
- `internal/arch/rules_test.go` is the negative test of the rule engine, so a typo in a
  rule cannot stay green.

## Verification

`go test ./internal/arch` fails as soon as a framework import appears outside the
transport layer. Verify by adding an import of `github.com/gofiber/fiber/v3` to
`internal/domain/errors.go`: the test must fail and name the rule.
