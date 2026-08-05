---
name: critique
description: Has tech-lead attack the plan you just wrote — layer-shaped slices, ordering that forces rework, a split that will not hold, a structural decision left untracked, a contradiction with an ADR or with the layering rules. Run in plan mode once your plan is written, before writing code. Its counterpart on code is /review.
---

# Have your plan attacked

`/critique` before the code exists. `/review` after. Same idea both times: a reader who did
not write the thing.

**You write the plan. This skill does not write it for you**, and that is the point. The
person who holds the model of the system is the one who should shape the work — an agent
producing the plan and another agent critiquing it is two agents talking to each other, and
every hop between them is a re-encoding with loss. What an agent is genuinely good at here
is attacking a plan it had no part in.

## 1. Find the plan

- **In plan mode** — read the plan file. That is the plan.
- **Not in plan mode** — the plan is what the user just wrote in the conversation. If there
  is no plan to be found, say so and stop; do not invent one to have something to critique.

Do not rewrite, complete or tidy it first. Pass it as it is: a plan improved before the
critic sees it hides the very gaps worth finding.

## 2. Spawn `tech-lead`

Agent tool, `subagent_type: tech-lead`, **one call**. Pass:

- the plan verbatim;
- what the user is trying to achieve, if the plan does not say it;
- the ADRs in `docs/adr/` that govern the area, **with their `Status`** — an ADR still
  `Proposed` blocks the slices depending on it;
- that it must verify the plan's claims about the current code itself rather than trust them.

It reads `CLAUDE.md`, `docs/SPEC.md` and the files concerned on its own.

## 3. Report

Give the verdict line and the findings, sorted by severity, unchanged. On
`[LEAD]: BLOCKING`, propose the corrected plan rather than handing back a list — the problem
is already diagnosed, and arbitrating it again is work the user does not need to do twice.

**The user decides each disposition, one by one — taken, declined, or extended.** Do not
settle them yourself, and treat a declined finding as an answer rather than a disagreement to
re-argue: he holds context the plan does not carry.

**Then rewrite the plan file with the dispositions that were taken, and say in one line that
you did.** Both halves matter:

- *Rewrite it.* In plan mode the plan file is the only writable artefact, so it is where the
  agreed state belongs. Leaving corrections in the conversation while the file keeps the old
  plan is how someone implements from a stale document — and the file is what he will read
  tomorrow, not this exchange.
- *Say so.* Otherwise there is no way to tell a rewritten plan from a reported one without
  grepping for a marker that should have changed. If the answer to "was the plan updated?" is
  not visible, the step is not finished.

If `tech-lead` says the plan is fine, say that in one line and rewrite nothing. A critic whose
verdict is always BLOCKING stops carrying information, and so does a skill that reports one.

## 4. When a finding says a decision is untracked

`tech-lead` will sometimes point at a decision the plan settles implicitly — a schema, a
deletion semantics, behaviour during a dependency outage, a responsibility boundary. It
names it; **it does not write the ADR.** You do, from `docs/adr/0000-template.md`, and this
bar decides whether it is worth the effort at all:

> **Would reversing this decision later be expensive?**

Yes for a persisted shape, a wire contract, a dependency to unpick from call sites,
behaviour under failure, a responsibility boundary. No for two options that both work and
swap in an hour, the shape of a config file, a field name, anything a review comment
settles — record those as a comment where the code makes the choice, or as one line under
`docs/SPEC.md` §6 *Later*.

An ADR is a gate that has to be cleared before the slices depending on it can start.
Inflating the count turns that gate into a queue.

## A note on estimates, which are the thing this repository gets wrong

When the plan claims a slice fits under the 400-line cap, `tech-lead` will check it — and it
is the check that misses most often, always in the same direction. Three estimates in a row
came in at about 2.5× their figure.

The cause is arithmetic, and **it depends entirely on which quantity was estimated.** Two
different numbers are in play, and applying the wrong multiplier to the wrong one produces
nonsense in either direction:

- **A — production logic lines**: what you will actually write, excluding comments, blank
  lines and tests.
- **B — added lines in the diff**: what `git diff --numstat -- '*.go'` counts. Comments and
  tests included. This is the number the 400 cap applies to.

In this repository comment density runs 25–40% on production files, so a production *file* is
about `1.4 A`; and production:test file lines come out near 1:1. Hence:

> **B ≈ 2.8 A.** 130 lines of logic is already a 365-line diff.

**Apply 2.8 only to A.** If the estimate is already expressed in added lines — a table with
`prod` and `test` columns is B, not A — then 2.8 does not apply, and multiplying by it again
inflates the figure by nearly threefold.

For an estimate already made in B, there is no clean multiplier, only observed drift: in this
codebase, estimates expressed directly in added lines have come in **1.5× to 2.5× low**, every
time, without exception. Treat such an estimate as a **floor**, not a figure — and say which
of A or B you estimated, so the next reader knows which correction applies.

The only reliable calibration is measurement: `git log --oneline -20`, then
`git show --stat <sha>` on a merged PR of comparable scope. In a fresh repository there are
none, so the arithmetic above is the fallback and the real count before `/open-pr` is what
settles it.
