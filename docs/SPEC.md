# Specification — [SERVICE NAME]

> **[TO BE DEFINED — run `/bootstrap-spec`]**
>
> Sections §2, §4, §5, §7, §8 and §10 ship filled in: they describe the template and
> carry over to your service as they are. Sections **§1, §3, §6 and §9** are yours to
> define. `/bootstrap-spec` interviews you and fills them.
>
> This file is not documentation for its own sake — three agents read it:
> `qa-engineer` checks a test establishes the *Definition of done* of the slice under
> review, `/plan-work` locates that slice in §6 before splitting it, and the ADR rule comes
> from §7. Leaving §6 vague disarms the review panel downstream.

---

## 1. Context and goals

**[TO BE DEFINED — run `/bootstrap-spec`]**

What problem the service solves, who calls it, and what it explicitly does not do.

---

## 2. Stack

- **Go 1.26**, module layout per §5.
- **HTTP**: Fiber v3 (fasthttp), confined to `internal/transport/http` — see
  [ADR-0001](./adr/0001-fiber-over-net-http.md).
- **Logging**: `log/slog`, JSON handler, request ID propagated.
- **Metrics**: `prometheus/client_golang`, confined to `internal/observability` — see
  [ADR-0002](./adr/0002-metrics-label-cardinality.md).
- **Tests**: `stretchr/testify`, table-driven, `httptest` for HTTP.
- **Lint**: `golangci-lint` v2, configuration in `.golangci.yml`.
- **Container**: multi-stage build, `distroless/static`, non-root, base images pinned by
  digest.
- **Local stack**: `docker compose` — service + Prometheus + Grafana, provisioned as code
  under `deploy/`.

Add a dependency here **and** in an ADR when it is not obvious. Every dependency,
transitive included, is surface.

---

## 3. Architecture decisions (ADR)

Non-obvious decisions are recorded in `docs/adr/`. Accepted so far:

- [ADR-0001 — Fiber v3 rather than net/http, confined to the transport layer](./adr/0001-fiber-over-net-http.md)
- [ADR-0002 — Bound the `route` metric label with an allowlist](./adr/0002-metrics-label-cardinality.md)

**Backlog — [TO BE DEFINED — run `/bootstrap-spec`]**

Declare here, with the §6 slice each one governs, the decisions your service must make before
the code assumes an answer. Declaring is not deciding: an ADR is written with
`## Decision` empty and becomes `Accepted` only when you fill it in.

---

## 4. Framework pitfalls to handle from day one

These are the real difficulty of the HTTP layer. Each one deserves a dedicated test.

- **fasthttp memory reuse**: `c.Body()`, `c.Params()` and `c.Get()` return data
  invalidated once the handler returns. Copy before any asynchronous use. This is the
  most likely bug class in this layer.
- **Context propagation**: take a `context.Context` at the top of the handler and pass
  only that one downstream. The API changed between Fiber v2 and v3 — check the exact
  names against the installed version before writing code.
- **`WriteTimeout` and long responses**: a finite `WriteTimeout` cuts a long stream
  mid-flight. Set it to 0 if the service ever streams.
- **Attribution of `context.Canceled`**: distinguish a client disconnection from a server
  shutdown (`baseCtx`). Confusing the two makes a normal deploy look like a wave of client
  errors.
- **Releasing `defer`s and asynchronous callbacks**: a callback that runs after the handler
  returns outlives a `defer` placed in the handler. The resource leaks on a recovered
  panic.
- **Route pattern is not bounded on a 404** — see ADR-0002. Never label a metric with a
  caller-controlled value.
- **Graceful shutdown**: cancel in-flight work *before* the server stops accepting, and
  build the shutdown budget on a fresh context. Reusing the already-cancelled signal
  context gives the drain no time at all.

---

## 5. Target tree

```
cmd/service/            entry point, wiring, signal handling
internal/
  config/               env -> struct, validated once at startup
  domain/               core types and error sentinels; knows no HTTP, no framework
  observability/        Prometheus registry and collectors (sole owner of client_golang)
  transport/http/       Fiber server, the only layer that knows the framework
    handlers/           handlers + the single domain-error -> HTTP-status mapping
    middleware/         request ID, structured logging, metrics recording
  arch/                 architecture tests (import rules) + negative test of the engine
docs/
  SPEC.md               this file
  adr/                  architecture decision records
deploy/
  prometheus/           scrape config
  grafana/provisioning/ datasource and dashboards, as code
.claude/                agents, skills and hooks — see §10
```

Add a package here when you add one. A tree that no longer matches reality is worse than
no tree, because it is still believed.

---

## 6. Work

**An ordered list of slices. One slice = one PR, ≤ 400 added lines of Go, tests included.**

There is no intermediate grouping. A need becomes an ordered list of slices, and each
slice becomes a pull request — nothing sits in between, because nothing in between decides
anything.

Two rules per entry, and they are what the review agents downstream depend on:

- **The title says what an outside caller can do**, not which layer is touched. Apply the
  test: *what can a caller do once this is merged that they could not before?* "Nothing —
  it enables the next one" means it is a layer, and it must be folded into the slice it
  serves. A title named after a directory — *configuration*, *client*, *store*,
  *middleware*, *the provider interface* — is the smell.
- **The *Definition of done* is falsifiable**: a criterion that would fail if the
  implementation were wrong. "The service works" or "the endpoint is implemented" are not.
  `qa-engineer` reads this item and checks a test establishes it, so a vague one disarms
  the review panel.

Entries here stay **coarse**: outcome, rough size, governing decision. The implementable
detail — exact files, exact out-of-scope, refined estimate — is produced by `/plan-work`
immediately before the slice is built. A slice specified today against a codebase that
will have changed by then is specified wrong, and planning slice 12 now is waste.

Keep only the next few slices specified. Anything further belongs under *Later*, as intent
rather than plan.

### Done

**0. The service starts, exposes its operational contract, and defends its own boundaries**
*(template baseline)*

**Definition of done** — satisfied, and every item is checked by something that runs:
`make check` green; `go test ./internal/arch` fails when a framework import is added to
`internal/domain`; `TestImportRulePrefixesAreRealModules` fails when a rule's prefix is
misspelled; `TestMetricsRouteLabelIsBounded` fails when the route label stops going through
the allowlist; `TestDomainErrorsAreMappedToStatus` fails when the error handler is removed;
`make dev` then `curl /healthz /readyz /metrics` all answer, the Prometheus target reads UP,
and the provisioned Grafana dashboard shows a request rate.

### Next

**[TO BE DEFINED — run `/bootstrap-spec`]**

```markdown
### 1. <what a caller can do that they could not before>

- **Size**: ~<n> lines of Go, tests included
- **Definition of done**: <a criterion that would fail if the implementation were wrong>
- **Governing decision**: ADR-000N (Accepted) | none
```

### Later — intent, not plan

**[TO BE DEFINED — run `/bootstrap-spec`]**

What the service is meant to grow into, one line each. No sizes, no *Definition of done*:
those are written when the slice moves up to *Next*. This section exists so an idea can be
recorded without being paid for now — and so `/plan-work` has somewhere to put an
abstraction it declined to introduce early.

---

## 7. Conventions

- **An ADR for every non-obvious decision.** Start from `docs/adr/0000-template.md`. An
  ADR names what is lost, not only what is gained, and says how the decision will be
  verified.
- **Errors**: an application error type in the domain, mapped to an HTTP status in one
  place (`handlers.StatusFor`). Never a `fiber.Error` outside the transport layer.
- **Tests**: real containers rather than mocks for any external dependency. A mock encodes
  the behaviour you assumed, so it cannot contradict you.
- **Layer boundaries are verified by a test**, not by review: see `internal/arch`. A
  boundary that exists only in prose has already been crossed somewhere.
- **Configuration**: environment → struct, validated once at startup. Any sensitive field
  is listed in `Config.LogValue` as redacted, before it exists in production — logs are not
  retroactively redactable.

---

## 8. README (counts almost as much as the code)

1. The problem solved, in three sentences.
2. Quickstart: `make dev` → a `curl` that answers.
3. The agentic workflow: which agent does what, and which rules are enforced mechanically.
4. Technical decisions, with links to the ADRs.
5. Known limits and roadmap — honesty beats an inflated scope.

---

## 9. Service level objectives

**[TO BE DEFINED — run `/bootstrap-spec`, or delete this section]**

Targets the service is expected to meet in production — latency, availability, freshness of
the data it serves. One line each, with the metric that measures it.

Keep these **out of** the §6 *Definition of done* items. An SLO is a running-system property
measured over a window; a *Definition of done* is something a test can fail on today.
Conflating them produces a DoD that cannot fail for the reason it claims, which is how a
review gate quietly stops gating.

---

## 10. Claude Code tooling — agents and skills

The **`cc-skills-golang@samber`** plugin provides ~45 Go skills. Install it before using
the agents, otherwise their `skills:` frontmatter resolves to nothing:

```
/plugin marketplace add samber/cc
/plugin install cc-skills-golang@samber
```

### Agents (`.claude/agents/`)

| Agent | When | Writes? |
|---|---|---|
| `tech-lead` | Upstream, on a **plan**, before code exists | No |
| `go-developer` | Production code | Yes |
| `go-test-writer` | Tests | Yes |
| `go-reviewer` | Review: correctness, safety, concurrency, layering | No |
| `qa-engineer` | Review: test quality, DoD coverage | No |
| `security-analyst` | Review: secrets, authorization, DoS surface | No |
| `platform-engineer` | Review: image, CI, observability, config — when those paths change | No |

The four review agents are spawned **in parallel** by `/review`, and the **strictest
verdict wins**. Each has an exclusive remit declared in its own file, so the panel returns
one finding per problem instead of four.

### Skills (`.claude/skills/`)

| Skill | Frequency |
|---|---|
| `/bootstrap-spec` | Once per project — three sentences and the first slice, nothing more |
| `/plan-work` | Once per unit of work, in plan mode — questions, split, ADR, and `tech-lead` when warranted |
| `/review` | Once per slice, before the PR |
| `/open-pr` | Once per slice |

### Which Go skills load

Nothing to maintain here. The plugin ships **`golang-how-to`**, an orchestrator that reads
the task at hand and loads the relevant skills — a slice touching the HTTP client pulls
`golang-context` and `golang-concurrency`, one touching authentication pulls
`golang-security`. A hand-written mapping would duplicate that mechanism, do it worse
because it cannot see the diff, and go stale the first time a slice is re-cut.

Each agent preloads the two or three skills it always needs, in its own `skills:`
frontmatter, and loads the rest on demand. That is the whole configuration.
