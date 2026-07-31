# [SERVICE NAME] — project guide

> **[TO BE REPLACED — run `/bootstrap-spec`]** Two lines describing what this service
> does and who calls it. Everything below this header is template rules that carry over
> to your service unchanged.

**Reference spec**: [`docs/SPEC.md`](./docs/SPEC.md). **Decisions**:
[`docs/adr/`](./docs/adr/). Any non-obvious decision → a new ADR (start from
`docs/adr/0000-template.md`).

## Contribution workflow — rules

- **One branch per change**, never a direct commit on `main`. Naming:
  `type/short-description` (e.g. `feat/user-lookup`, `fix/readyz-timeout`). Enforced by
  `.claude/hooks/check-commit.sh`.
- **Conventional Commits**: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`,
  `ci:`, optional scope (`feat(transport): ...`). Adding a type means adding it to the
  hook too, or the two rules diverge.
- **Small PRs**: **≤ 400 added lines of Go per PR, tests included** (excluding
  `go.mod`/`go.sum`, markdown, YAML/Makefile config and generated code). Tests **count
  toward the limit**: it protects human review time, and a test file reads like the rest.
  A 300-line production change with 800 lines of tests is not reviewable, whatever the
  merit of those tests.
  Two practical consequences: split by **vertical functional slice** (one endpoint and
  its tests, not a whole layer), and write **discriminating rather than exhaustive**
  tests — one case that would fail if the implementation were wrong beats three variants
  of the same one.
  Count with: `git diff --numstat origin/main...HEAD -- '*.go'`. Keep this command
  identical to the one in `.claude/hooks/check-pr-size.sh` and in
  `.claude/skills/open-pr/SKILL.md`.
- **One PR at a time**: the current PR must be **validated** (reviewed + CI green) and
  merged before the next one opens.
- **Announce the split before writing**: if a need clearly exceeds one PR, propose the
  split into vertical slices and wait for approval **before** writing code. The 400-line
  limit is decided at split time, not at `gh pr create`. This is the rule `/plan-work`
  applies.
- **The merge belongs to the human.** Claude (and its agents) opens the PR and gets CI
  green, then **hands back**; the final review and the `merge` are done by the user.
  Claude never runs `gh pr merge` and never deletes the branch — both are denied in
  `.claude/settings.json`.
- Every PR leaves `main` **compiling and green**: `make check` (vet + lint + test).
- Every non-trivial architecture decision → an ADR in the same PR or just before.
- **Codified rituals**: framing a need via `/plan-work` (in plan mode, before writing
  code); review via `/end-phase`; opening a PR via `/open-pr` (never an ad-hoc
  `gh pr create`).

## Layering — non-negotiable

Each of these is verified by `internal/arch`, in test files too: a test that imports the
framework from the domain crosses the boundary just as effectively as production code, and
the exemption would be invisible in review.

- **Fiber lives only in `internal/transport/http/`.** The domain and every service package
  deal only with `context.Context` and domain structs. No `github.com/gofiber/...`
  anywhere else — and no `github.com/valyala/fasthttp` either, which would be the same
  coupling through the transitive dependency. See
  [ADR-0001](./docs/adr/0001-fiber-over-net-http.md).
- **Errors**: an application error type in the domain → HTTP mapping centralised in
  `handlers.StatusFor`. Never a `fiber.Error` outside `internal/transport/http`.
- **`prometheus/client_golang` lives only in `internal/observability`.** Callers depend on
  narrow interfaces (`middleware.RequestRecorder`), never on prometheus types. See
  [ADR-0002](./docs/adr/0002-metrics-label-cardinality.md).

## Service contract

Every service built from this template exposes three endpoints, and they stay distinct:

- **`/healthz`** — liveness. Depends on **nothing**. A liveness check that probed a
  dependency would turn a dependency outage into a restart loop that fixes nothing.
- **`/readyz`** — readiness. Registers one `handlers.Probe` per external dependency, each
  under its own timeout budget. With no probe registered, the server **warns at startup**:
  a `/readyz` hardcoded to 200 removes the signal without saying so.
- **`/metrics`** — Prometheus. Never a caller-controlled label value.

## Go conventions

- Style and naming: follow the `golang-code-style` and `golang-naming` skills.
- Error handling: wrap with `fmt.Errorf("...: %w", err)`, sentinels via `errors.Is/As`
  (`golang-error-handling`). Never wrap a value carrying a secret into an error that
  reaches the logs.
- Tests: table-driven, `t.Helper()`, `httptest` for HTTP, real containers rather than
  mocks for external dependencies (`golang-testing`, `golang-stretchr-testify`).
- Concurrency: context propagated everywhere, no goroutine without a clear lifecycle
  (`golang-concurrency`).
- Config: environment → struct validated at startup. Any sensitive field listed in
  `Config.LogValue` as redacted.
- Comment the **why**, not the **what**: record the failure a line prevents or the
  alternative rejected. See `internal/transport/http/middleware/metrics.go` for the
  register.

## Commands

```bash
make build      # go build -> ./bin/service
make test       # go test ./...
make lint       # golangci-lint run
make check      # vet + lint + test
make run        # run locally
make dev        # local stack: service + Prometheus + Grafana
make rename MODULE_NEW=github.com/org/name   # rewrite the module path
```

## Skills & agents

The `cc-skills-golang@samber` plugin provides ~45 Go skills — install it, otherwise the
agents' `skills:` frontmatter resolves to nothing. Skills ↔ phases mapping:
`docs/SPEC.md` §10.

- **Framing, upstream**: `tech-lead` critiques a **plan** before the code exists (split,
  slice ordering, untracked structural decision, contradiction with an ADR or with the
  layering rules). Spawned by `/plan-work` and `/bootstrap-spec`.
- **Writing**: `go-developer` (production code), `go-test-writer` (tests).
- **Review, read-only** — the panel `/end-phase` spawns **in parallel**, with the
  **strictest verdict** winning: `go-reviewer` (correctness, safety, concurrency,
  layering), `qa-engineer` (test discriminating power, Definition of done coverage),
  `security-analyst` (secrets, authorization, DoS surface), and `platform-engineer`
  (image, CI, observability, config) only when the diff touches its paths.

Each review agent has an **exclusive remit** declared in its file under "Not your remit":
that is what stops four agents returning the same finding. Some checklist entries sit
under `## Rules that activate when …` and are inactive here — move them up when the
corresponding capability arrives.

## Project status

**[TO BE DEFINED — run `/bootstrap-spec`]** Phase roadmap in `docs/SPEC.md` §6.
