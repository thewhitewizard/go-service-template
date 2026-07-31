---
name: end-phase
description: Review pass — runs a panel of agents in parallel (go-reviewer, qa-engineer, security-analyst, plus platform-engineer when relevant) on the branch diff vs main, applies the strictest verdict and reports actionable findings (🔴/🟠/🟡). Run before opening a PR, or for an out-of-cycle review.
---

# Review pass

Run in order.

## 1. Scope

Determine the diff with `git diff origin/main...HEAD --stat` (fall back to local `main`
if `origin/main` is absent). If the diff is empty, say so and stop.

Also get the changed file list (`git diff origin/main...HEAD --name-only`): it decides
which agents to spawn in step 2.

## 2. Spawn the panel, in parallel

The panel gives several viewpoints on the **same** diff. Each agent has an exclusive
remit, declared in its own file; an agent straying outside it produces a duplicate.

**Always** — three agents:

| `subagent_type` | Viewpoint |
|---|---|
| `go-reviewer` | Correctness, safety, concurrency, layering |
| `qa-engineer` | Test quality and discriminating power, Definition of done coverage |
| `security-analyst` | Secret leakage, authorization, DoS surface, input validation |

**Conditionally** — a fourth, only if the changed files touch `Dockerfile`,
`.dockerignore`, `compose*`, `.github/workflows/`, `deploy/`, `internal/observability/`,
`internal/config/`, or the route wiring in `internal/transport/http/server.go`:

| `subagent_type` | Viewpoint |
|---|---|
| `platform-engineer` | Image, CI, observability, configuration, graceful shutdown |

**Issue every Agent call before awaiting the first result.** The agents are independent;
running them in series multiplies the wait for nothing.

Each agent's prompt contains:

- the changed file list and **the exact diff command to replay** — never the diff content
  serialised into the prompt: each agent reads what concerns it, which keeps its context
  budget for the reading that matters;
- the phase concerned (derive it from the branch name or `docs/SPEC.md`, otherwise ask
  the user), so it loads the skills from the §10 mapping and can find the phase's
  *Definition of done*;
- a reminder of its exclusive remit, so it does not return another agent's findings;
- the output contract: `[PREFIX]: ...` verdict on the first line, then per finding
  **file:line**, the problem named precisely, the condition under which it breaks, a
  concrete fix, and a label:
  - 🔴 **BLOCKING** — bug, data race, vulnerability, architecture violation; fix before
    merge.
  - 🟠 **IMPORTANT** — a real risk under particular conditions.
  - 🟡 **SUGGESTION** — defensive or stylistic improvement.

## 3. Consolidate

Read the **first-line verdict** of each agent (`[REV]:`, `[QA]:`, `[SEC]:`, `[OPS]:`).

- **Overall verdict = the strictest one.** A single `BLOCKING` makes the overall verdict
  `BLOCKING`, whatever the others say. A `CONCERNS` beats any number of `OK`s.
- **Deduplicate** before reporting: two agents flagging the same line for the same cause
  produce **one** finding, crediting both. Do not stack them — a panel that reports the
  same problem three times gets ignored.
- If an agent failed or returned no readable verdict, **say so explicitly** rather than
  counting it as `OK`. A missing viewpoint is not a satisfied one.

## 4. Report

Report the overall verdict, then the deduplicated findings sorted by severity, naming
for each the agent that raised it. **Do NOT fix the code straight away** — the decision
to fix belongs to the user.

## 5. Learning loop

This is the mechanism that turns a generic checklist into this project's institutional
memory. It must feed **all four** agents, otherwise only the first one ever gets smarter.

Compare the findings against the "priority attention points" of the panel agents
(`.claude/agents/{go-reviewer,qa-engineer,security-analyst,platform-engineer}.md`) and
against previous reviews — search git history for bug classes already fixed:
`git log --oneline --grep=fix`.

If a finding belongs to a class already seen (same kind of bug, not necessarily the same
file), flag it explicitly and **propose** adding it to the checklist of **the agent that
produced the finding** — not systematically to `go-reviewer`'s. Only modify an agent file
with the user's explicit agreement.

If a finding clearly belongs to another agent's remit than the one that raised it, that is
a badly declared boundary: flag it and propose tightening the "Not your remit" section of
both agents concerned.

Some checklist entries live under a `## Rules that activate when …` heading, inactive in
this template. When the corresponding capability arrives (a datastore, streaming,
multi-tenancy), propose moving the relevant entries up into the main checklist.
