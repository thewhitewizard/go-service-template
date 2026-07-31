---
name: plan-work
description: Frames one unit of work — asks the questions the need leaves open, locates the SPEC phase, splits into slices of ≤ 400 lines with an out-of-scope section and a falsifiable Definition of done, drafts the ADR of any structural decision (Decision left empty), then has the plan critiqued by tech-lead. Run in plan mode, before writing code.
argument-hint: "<the need, in a sentence or two>"
---

# Frame a unit of work

## ⓿ Switch to plan mode

- **Already in plan mode** → go to ①.
- **Otherwise** → call `EnterPlanMode` **immediately, without asking**. The tool
  requests the user's consent itself; asking beforehand would ask the same question
  twice for one decision. Do not start ① until plan mode is active.

Two reasons to be there, not one:

- plan mode guarantees **nothing is written** during framing — no code, no ADR laid down
  before it is decided;
- it provides the **plan file** for the split and the drafted ADRs, plus the approval
  gate (`ExitPlanMode`) that materialises the user's agreement.

**If the user declines plan mode**, still run ① to ⑤ but **write no file**: hand
everything back in the conversation, and say that there will be no written trace, so the
absence is a choice and not a side effect.

**Constraint**: in plan mode only the plan file is writable. This skill creates **no**
file in the repository — ADRs are drafted in the plan file and written to `docs/adr/`
as the first action after `ExitPlanMode`.

## ① Ask the questions first

**This is the main step.** A precisely described need always has holes, and those holes
are what make the implementation diverge from the intent.

Read the need, explore the code concerned, then list what the need **does not settle**:
exact semantics of an operation, behaviour at the boundaries, effect on related
entities, backward compatibility of the exposed surface, behaviour when a dependency is
down.

Ask through `AskUserQuestion` — grouped, at most 4 per call. Do not ask anything already
answered by `CLAUDE.md`, `docs/SPEC.md` or an ADR: that is noise, and it teaches the
user to skim the questions that matter.

**Write nothing before you have the answers.** Producing markdown without having asked
anything skips the only step this skill adds over plain plan mode.

## ② Locate

- Derive the **phase** from `docs/SPEC.md` §6, and take its *Definition of done*.
- Take the **skills for that phase** from the mapping in §10 — they go into the
  developer's brief.
- List the **existing ADRs** (`docs/adr/`) governing the area touched, **with their
  `Status`**. An ADR still `Proposed` blocks the slices that depend on it.

## ③ Split

One slice = **one PR of ≤ 400 added lines of Go, tests included**. Split by **vertical**
functional slice (one endpoint and its tests, never a whole layer).

One section per slice:

```markdown
### Slice 1 — <name>

- **Files**: <paths>
- **Estimate**: ~<n> lines of Go, tests included
- **Out of scope**: <what this slice deliberately does NOT do>
- **Definition of done**: <a criterion that would fail if the implementation were wrong>
- **Governing ADR**: ADR-000N (Accepted) | ADR-000M (Proposed — must be decided first)
- **Skills for this phase (§10)**: <list>
```

Two requirements per slice:

- **Out of scope must not be empty.** It is what stops slice 1 from eating slice 2 and
  crossing the limit. Name what the slice does **not** do, including the tempting parts.
- **The Definition of done must be falsifiable.** A criterion no test could contradict is
  not one. "The endpoint is implemented" is worthless; "a field not supplied stays
  unchanged" is testable — and `qa-engineer` will check a test establishes it.

**Proportionality.** If the need fits in one slice, produce **one** slice. Do not invent
a split, do not manufacture an ADR. Over-ceremonialising a small need is this skill's
failure mode: it ends up unused.

## ④ Draft the ADR of any structural decision

`docs/SPEC.md` §7: an ADR for any non-obvious decision. If framing surfaced a decision
the plan would settle implicitly — deletion semantics, schema choice, behaviour during a
dependency outage, a responsibility boundary — record it.

Draft it **in full, in the plan file**, from `docs/adr/0000-template.md`, with the next
free number in `docs/adr/`:

- `**Status**: Proposed`, `**Phase**` filled in.
- `## Context`, `## Options considered`, `## Consequences`, `## Mitigations`,
  `## Verification` written.
- Each option names **what is lost**, not only what is gained.
- `## Decision` **left empty**:

```markdown
## Decision

<!-- To be decided by the architect. -->
```

**An ADR that arrives pre-decided is a failure**: the architectural decision belongs to
the user, exactly as the merge does.

If a slice depends on an ADR that is still `Proposed` — the one just drafted, or a
pre-existing one — say so in the slice: it is not implementable before the decision.

## ⑤ Have it critiqued

Spawn `tech-lead` (Agent tool, `subagent_type: tech-lead`), **one call**, in **mode
"slices"** — say so explicitly in the brief, it changes which lenses it applies. Pass:

- the original need and the answers from step ①;
- the drafted plan (slices, estimates, out-of-scope, DoD);
- the phase concerned and the governing ADRs with their status.

Wait for its verdict before handing back.

## ⑥ Hand back

In this order:

1. **The `tech-lead` verdict** and its findings. On `[LEAD]: BLOCKING`, propose the
   corrected plan rather than asking the user to arbitrate a problem already diagnosed.
2. The plan: context, questions and answers, slices, drafted ADRs.
3. If an ADR was drafted, **state explicitly** that writing it to `docs/adr/` is the
   first action after approval, before any line of code, and that it must be decided and
   moved to `Status: Accepted` before implementing the slices that depend on it.

Then hand back for approval (`ExitPlanMode`).

After approval, implement **one slice at a time**: brief `go-developer` with the plan
file path and the slice number — including its out-of-scope section — then
`go-test-writer`, then `/end-phase`, then `/open-pr`.
