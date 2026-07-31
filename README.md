# go-service-template

A starting point for a Go backend service, and — more importantly — **a way of working
with Claude Code that does not depend on remembering to do the right thing.**

The runtime part is deliberately small: an HTTP server, configuration, structured logging,
the three operational endpoints, a container image and a local observability stack. The
part that took the effort is the other one: seven agents with non-overlapping remits, four
rituals, and a handful of guardrails that a script enforces rather than a convention.

> **[TO BE REPLACED — run `/bootstrap-spec`]** Once you start a real service from this
> template, replace this section with what *your* service does and who calls it.

---

## Quickstart

```bash
make dev          # service + Prometheus + Grafana, provisioned as code
```

Then:

```bash
curl localhost:8080/healthz    # {"status":"ok"}
curl localhost:8080/readyz     # {"probes":0,"status":"ready"}
curl localhost:8080/metrics    # Prometheus exposition
```

- Prometheus: <http://localhost:9090> — the `service` target should read **UP**
- Grafana: <http://localhost:3000> — dashboard **Service — RED**, no login

Ports are overridable when one is already taken, which on a developer machine is most of
the time:

```bash
GRAFANA_PORT=3001 SERVICE_PORT=8081 make dev
```

Without Docker: `make run`, then the same `curl`s.

## Starting a new project

1. **Use this template** on GitHub (the button, not a fork — you get a clean history).
2. `make rename MODULE_NEW=github.com/your-org/your-service` — rewrites the module path
   everywhere and re-tidies. Do this **before the first commit**, or every later diff
   carries the rename noise.
3. `/bootstrap-spec "what your service does"` — interviews you, then fills
   `docs/SPEC.md` (context, phases with a falsifiable *Definition of done*, ADR backlog)
   and rewrites the `CLAUDE.md` and README headers.
4. Decide the drafted ADRs and set them to `Status: Accepted`. A phase whose governing ADR
   is still `Proposed` is not implementable.
5. `make check` — must be green before you write a line of your own.

Then, for each unit of work: `/plan-work` → implement → `/end-phase` → `/open-pr`.

## Requirements

- **Go 1.26**
- **Docker** (for `make dev` and the image)
- **golangci-lint `v2.12.2`** — the exact version, not just "v2". CI pins the same one
  in `.github/workflows/ci.yml`, and it has to stay in sync: linter releases change
  which findings they report, so a developer on an older binary gets a green
  `make check` on code CI rejects. Check yours with `make lint-version`.
- **The `cc-skills-golang@samber` plugin** — required, not optional: without it the
  agents' `skills:` frontmatter resolves to nothing and their reviews are worth much less.

  ```
  /plugin marketplace add samber/cc
  /plugin install cc-skills-golang@samber
  ```

---

## The agentic workflow

### The daily loop

```
/plan-work "<need>"     once per need, in plan mode
    ↓                   asks the open questions, splits into PR-sized slices,
                        drafts the ADR of any structural decision,
                        then tech-lead critiques the plan
implement one slice      go-developer + go-test-writer
/end-phase               panel of 4 agents in parallel, strictest verdict wins
/open-pr                 you review, you merge
    ↺ next slice
```

### Why a panel and not a reviewer

A single reviewer has a single lens. `go-reviewer` is good at Go correctness, concurrency
and layering — and it will not look for a mass-assignment hole on a `PATCH`, nor notice
that a test whose only assertion is `require.NoError` establishes nothing, nor that a new
metric label is fed by caller input.

So the review has four viewpoints, spawned in parallel, each with an **exclusive remit**
declared in its own file so the panel returns one finding per problem instead of four:

| Agent | Lens | Model |
|---|---|---|
| `go-reviewer` | Correctness, safety, concurrency, layering | opus |
| `qa-engineer` | Test discriminating power, *Definition of done* coverage | sonnet |
| `security-analyst` | Secret leakage, authorization, DoS surface | opus |
| `platform-engineer` | Image, CI, observability, configuration — only when those paths change | sonnet |

A single 🔴 makes the overall verdict 🔴, whatever the others say.

### Why framing before code

The panel runs *after* the code exists, so an approach mistake is paid in work thrown
away. `tech-lead` critiques the **plan** instead: does the split hold under the 400-line
limit, does slice 2 force slice 1 to be rewritten, is a structural decision being settled
without being recorded, is the *Definition of done* falsifiable.

Its usefulness rests on an asymmetry: it did not write the plan.

### The learning loop

`/end-phase` step 5 compares each finding against the checklist of the agent that raised
it, and against bug classes already fixed in git history. When a finding belongs to a class
already seen, it proposes adding it to that agent's checklist — **with your explicit
agreement**.

That is how a generic checklist becomes this project's institutional memory. The entries in
`go-reviewer` about `defer` placement before an asynchronous callback, or about a
constructor returning `(object, error)` both non-nil, are not general wisdom: they are real
bugs that were paid for once.

---

## Enforced mechanically vs advisory

This distinction is what makes the setup credible rather than decorative. **Nothing an
agent says blocks anything** — verdicts inform, you decide. What blocks is only what a
script can decide without being wrong:

| Rule | Mechanism | Effect |
|---|---|---|
| Claude never merges, never deletes a branch | `permissions.deny` | **hard refusal** |
| No direct commit on `main` | `hooks/check-commit.sh` | **deny** |
| Conventional Commits | `hooks/check-commit.sh` | **ask** (override by confirming) |
| PR ≤ 400 lines of Go | `hooks/check-pr-size.sh` on `gh pr create` | **ask**, with the prod/test breakdown |
| Fiber and fasthttp stay in the transport layer | `internal/arch` + CI | **build failure** |
| prometheus stays in `internal/observability` | `internal/arch` + CI | **build failure** |
| Metric labels stay bounded | `TestMetricsRouteLabelIsBounded` | **build failure** |
| `go.mod` matches the real imports | `go mod tidy -diff` in CI | **build failure** |
| The image does not run as root | assertion in CI | **build failure** |

The commit hook is deliberately **fail-open**: on any invocation form it cannot read with
certainty (`-F`, `--amend` with no message, exotic quoting) it emits no opinion. A
guardrail that blocks wrongly is as broken as one that never blocks.

---

## Two design details worth knowing

**`/readyz` with no dependency.** This template has no datastore, so there is nothing real
to probe — and a `/readyz` hardcoded to 200 is the worst of both worlds, because it removes
the signal without saying so. So readiness takes a list of named `handlers.Probe`, empty by
default, and the server **warns at startup**:

```
no readiness probes registered: /readyz reports liveness only
```

Register one probe per dependency in `cmd/service/main.go` and the warning goes away.

**The `route` metric label.** Labelling with the raw path is unbounded, which every guide
says. What they do not say is that in Fiber the usual fix is only half of one:
`Ctx.Route()` **falls back to the raw request path** when no route matched, so a caller
spraying `/a1`, `/a2`, `/a3`… still creates one time series per request. The label therefore
goes through an allowlist of registered patterns, built at construction time and failing
closed. See [ADR-0002](./docs/adr/0002-metrics-label-cardinality.md).

---

## Adding a database (or streaming, or multi-tenancy)

The agents' checklists carry entries that are inactive here, grouped under
`## Rules that activate when …`:

- **a datastore** — no DB mocks, migration applicable during a rolling deploy, separate
  connect and migrate budgets, generated-code drift guard in CI, a `handlers.Probe` for
  `/readyz`;
- **streaming responses** — `Flush()` per event, client disconnection detected, infinite
  `WriteTimeout`;
- **multi-tenancy** — tenant restriction in the `WHERE` clause, explicit public/admin
  surface classification, quota reservation released on error.

When you add the capability, move the relevant entries up into the main checklist of the
agent concerned. `/end-phase` will suggest it too, the first time a finding of that class
shows up.

## Layout

```
cmd/service/            entry point, wiring, signal handling
internal/
  config/               env -> struct, validated once at startup
  domain/               core types and error sentinels; no HTTP, no framework
  observability/        Prometheus registry (sole owner of client_golang)
  transport/http/       Fiber server, the only layer that knows the framework
  arch/                 import rules + the negative test of the rule engine
docs/SPEC.md            the spec three agents read
docs/adr/               architecture decision records
deploy/                 Prometheus scrape config, Grafana provisioning
.claude/                agents, skills, hooks
```

## Known limits

- **No datastore.** Deliberate, see above.
- **No tracing.** Metrics and structured logs only; OpenTelemetry is a decision for your
  service, not for the template.
- **The `docker` CI job builds but does not push.** Publishing is a release concern.
- **Base image digests go stale.** Refresh them deliberately, in a commit of their own:
  `docker buildx imagetools inspect golang:1.26-alpine`.
