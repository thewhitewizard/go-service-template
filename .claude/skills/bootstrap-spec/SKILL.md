---
name: bootstrap-spec
description: One-time project bootstrap — interviews you about the service, then fills docs/SPEC.md (context, an ordered list of PR-sized slices each with a falsifiable Definition of done, declared ADR backlog), drafts those ADRs with Decision left empty, and rewrites the CLAUDE.md and README headers. Run once on a fresh clone of the template, before any code.
argument-hint: "<what the service does, in a sentence or two>"
---

# Bootstrap a new service from this template

Run **once**, on a fresh clone. After that, `/plan-work` takes over for each unit of
work.

## ⓿ Switch to plan mode

- **Already in plan mode** → go to ①.
- **Otherwise** → call `EnterPlanMode` **immediately, without asking**. The tool
  requests the user's consent itself; asking beforehand would ask the same question
  twice. Do not start ① until plan mode is active.

Plan mode guarantees nothing is written during the interview, and it provides the plan
file where the drafted `docs/SPEC.md` and ADRs live until approval.

**Constraint**: in plan mode only the plan file is writable. This skill therefore
creates **no** file in the repository. Everything it drafts goes into the plan file, and
writing it to disk is the first action after `ExitPlanMode`. Say so explicitly when
handing back.

## ① Read what the template already decided

Do not ask what the repository already answers.

- `docs/SPEC.md` §2, §4, §5, §7, §8, §10 ship filled in: they describe the template's
  stack, its framework pitfalls, its target tree, its conventions and its tooling.
- `docs/adr/` already holds accepted decisions (framework choice, metrics cardinality).
- `CLAUDE.md` holds the layering rules and the contribution workflow.

## ② Interview

**This is the main step.** Everything downstream — the slice list, its *Definition of
done*, the ADR backlog — is only as good as this conversation.

Ask through `AskUserQuestion`, grouped, at most 4 per call. Cover:

- **What the service does**, and **who calls it** (another service, a front end, a
  scheduled job, the public internet). The caller determines the authentication story
  and the denial-of-service surface.
- **External dependencies**: database, cache, queue, third-party API, blockchain node.
  Each one needs a `handlers.Probe` in `/readyz` and shows up in the slice list.
- **The MVP**: the smallest version that is genuinely useful to a caller. Push back on
  an MVP that is a layer rather than a working slice.
- **What must be true to run it in production**: throughput, latency, data durability,
  multi-tenancy, audit. These become *Definition of done* items, not wishes.
- **What is explicitly out of scope**, and for how long.

Do not ask about things the template already settles (web framework, logging format,
metrics backend) unless the answers suggest they are wrong for this service — in which
case say so and propose an ADR that supersedes the existing one.

## ③ Draft `docs/SPEC.md`

Fill only the sections the template left open: **§1** (context and goals), **§3**
(declared ADR backlog), **§6** (the ordered slice list), **§9** (SLOs). Leave the others as
shipped, and say so.

§6 is **an ordered list of slices — one slice, one PR**. There is no phase, no milestone,
no grouping in between: a need becomes an ordered list, and each entry becomes a pull
request. Nothing sits between them because nothing in between decides anything.

One entry per slice, under *Next*:

```markdown
### 1. <what a caller can do that they could not before>

- **Size**: ~<n> lines of Go, tests included
- **Definition of done**: <a criterion that would FAIL if the implementation were wrong>
- **Governing decision**: ADR-000N | none
```

Four rules:

- **The title names an outcome, not a layer.** Apply the test to every entry: *what can an
  outside caller do once it is merged that they could not before?* "Nothing — it enables
  the next one" means it is a layer, and it must be folded into the slice it serves.

  The smell is a title taken from the directory tree — *configuration*, *client*, *store*,
  *middleware*, *the provider interface*. Those say where code goes, not what the service
  can do. A list written that way survives every check in this skill and then poisons
  `/plan-work`, which inherits the shape and splits a layer into sub-layers.

  Concretely: "load the chain list from configuration" is a layer. "return the current
  block of one chain, end to end" is an outcome — and it makes the configuration work fall
  out of it, sized to what is actually needed.

- **The first slice must be shippable end to end.** It decides whether anything can be
  validated at all: a first slice that delivers a layer cannot be checked by anything, so
  it cannot be wrong either, which is the problem.

- **Every *Definition of done* is falsifiable.** "The service works" or "the endpoint is
  implemented" are not. "A revoked key is rejected in under 5 s, established by an
  integration test" is. `qa-engineer` reads the DoD of the slice under review and checks a
  test establishes it, so a vague one disarms the panel downstream.

  Keep running-system targets — latency percentiles, availability — out of the DoD and in
  §9. An SLO is measured over a window; a DoD is something a test fails on today.
  Conflating them produces a DoD that cannot fail for the reason it claims.

- **Stay coarse, and stay short.** Outcome, rough size, governing decision — nothing more.
  The exact files and the out-of-scope section are `/plan-work`'s job, produced immediately
  before the slice is built, because a slice specified today against a codebase that will
  have changed is specified wrong.

  Specify **the next few slices only**. Everything else goes under *Later* as one line of
  intent, with no size and no DoD. Planning slice 12 today is waste, and a long list reads
  as a commitment nobody made.

A slice that genuinely cannot be vertical exists (a dependency bump, a migration with no
user-visible effect). It is allowed, but say so explicitly, and never make it the first.

Keep the existing entry under *Done* (the template baseline) as the worked example of a
falsifiable DoD.

## ④ Declare the ADR backlog, and draft each ADR

### The test that decides whether something deserves an ADR

`docs/SPEC.md` §7 asks for an ADR on any non-obvious decision, and "non-obvious" is far
too generous a bar on its own. Apply this one instead:

> **Would reversing this decision later be expensive?**

Yes — it earns an ADR:

- it fixes a persisted data shape, a wire contract, or a publicly exposed surface;
- it adds a dependency that would have to be unpicked from call sites;
- it determines behaviour under failure (fail-open vs fail-closed, degrade vs refuse);
- it draws a responsibility boundary between packages or services.

No — it does not, and writing one is a net loss:

- two options that both work and would take an hour to swap;
- the shape of a config file, the name of a field, the spelling of a route;
- anything a code review comment settles.

**Count check.** More ADRs than slices is a signal, not an achievement: re-read them and
apply the test above to each. Seven ADRs on a four-slice project means most of them are
ceremony, and ceremony has a cost — each one is a decision the user must arbitrate before
the slices that depend on it become implementable. Inflating the count converts an
approval gate into a queue.

**Demoted decisions are not lost.** Record them as a short *Decisions taken inline* list
in the relevant `docs/SPEC.md` section, or as a comment where the code makes the choice.
The trace survives; the ceremony does not.

### Drafting

List the surviving decisions in §3 with the slice they govern, then draft each one **in the
plan file**, from `docs/adr/0000-template.md`, using the next free number in
`docs/adr/`:

- `**Status**`, and `**Slice**` set to the §6 entry the decision governs.
- `## Context`, `## Options considered`, `## Consequences`, `## Mitigations`,
  `## Verification` written.
- Each option names **what is lost**, not only what is gained — that is the template's
  instruction and it is what makes an ADR worth reading later.
- `## Decision` **left empty**:

```markdown
## Decision

<!-- To be decided by the architect. -->
```

**An ADR that arrives pre-decided is a failure**: the architectural decision belongs to
the user, exactly as the merge does. Present the options and their consequences, not a
choice.

## ⑤ Rewrite the project identity

Draft, in the plan file:

- The **header of `CLAUDE.md`** — title and the two-line pitch. Nothing else in that
  file describes the project; the rest is rules that carry over as they are.
- The **"What this is" section of `README.md`**, replacing the template's description.
- A reminder to run `make rename MODULE_NEW=github.com/<org>/<name>`, which rewrites the
  module path everywhere and re-tidies. Do it **before** the first commit, otherwise
  every later diff carries the rename noise.

## ⑥ Have the slice list critiqued

Spawn `tech-lead` (Agent tool, `subagent_type: tech-lead`), **one call**, in **mode
Pass:

- the original need and the answers from step ②;
- the drafted slice list with each entry's *Definition of done*;
- the declared ADR backlog.

Wait for its verdict before handing back.

## ⑦ Hand back

In this order:

1. **The `tech-lead` verdict** and its findings. On `[LEAD]: BLOCKING`, propose the
   corrected slice list rather than asking the user to arbitrate a problem that
   is already diagnosed.
2. The drafted `docs/SPEC.md` sections, the ADRs, and the new headers.
3. **The write order after approval**, explicitly:
   - `make rename MODULE_NEW=…` first;
   - then `docs/SPEC.md`, the ADRs, the `CLAUDE.md` and `README.md` headers;
   - then decide each ADR and set its `Status: Accepted` — a slice whose governing ADR
     is still `Proposed` is not implementable;
   - then `make check`, and only then the first `/plan-work`.

Then hand back for approval (`ExitPlanMode`).
