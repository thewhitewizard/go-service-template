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

**Do not apply the corrections to any file.** The plan is the user's; so is the decision to
change it.

If `tech-lead` says the plan is fine, say that in one line. A critic whose verdict is always
BLOCKING stops carrying information, and so does a skill that reports one.

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

The cause is arithmetic, not judgement. Comment density here runs 25–40%, and production and
test lines come out near 1:1. So **N lines of actual logic land as roughly 2.8 N added
lines**: 130 lines of logic is already a 365-line diff. Estimating the logic instead of the
diff undershoots by exactly that factor.

The reliable calibration is your own merged PRs — `git log --oneline -20`, then
`git show --stat <sha>` on one of comparable scope. In a fresh repository there are none, and
the 2.8 multiplier is the fallback.
