---
name: bootstrap-spec
description: One-time project bootstrap — interviews you about the service, then fills docs/SPEC.md (context, phases with a falsifiable Definition of done, declared ADR backlog), drafts those ADRs with Decision left empty, and rewrites the CLAUDE.md and README headers. Run once on a fresh clone of the template, before any code.
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

**This is the main step.** Everything downstream — the phases, their *Definition of
done*, the ADR backlog — is only as good as this conversation.

Ask through `AskUserQuestion`, grouped, at most 4 per call. Cover:

- **What the service does**, and **who calls it** (another service, a front end, a
  scheduled job, the public internet). The caller determines the authentication story
  and the denial-of-service surface.
- **External dependencies**: database, cache, queue, third-party API, blockchain node.
  Each one needs a `handlers.Probe` in `/readyz` and shows up in the phases.
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
(declared ADR backlog), **§6** (phases), **§9** (milestones). Leave the others as
shipped, and say so.

For §6, one section per phase:

```markdown
### Phase 1 — <name> (≈ <duration>)

**Goal**: <one sentence: what becomes possible that was not>

- <bullet of work>
- <bullet of work>

**Definition of done**: <a criterion that would FAIL if the implementation were wrong>
```

Three rules for the phases:

- **Phase 1 must be shippable end to end.** A first phase that delivers a layer rather
  than a working slice cannot be validated by anything — so it cannot be wrong either,
  which is the problem.
- **Every *Definition of done* must be falsifiable.** "The service works" or "the
  endpoint is implemented" are not. "A revoked key is rejected in under 5 s, verified by
  an integration test" is. `qa-engineer` reads these items and checks each one has a
  test that establishes it, so a vague DoD disarms the review panel downstream.
- **Each work bullet names an outcome, not a layer.** Apply this test to every bullet:
  *what can an outside caller do once it is delivered that they could not before?* If the
  answer is "nothing — it enables the next bullet", it is a layer and it must be folded
  into the bullet it serves.

  The smell is a bullet titled with a noun from the directory tree — *configuration*,
  *client*, *store*, *middleware*, *the provider interface*. Those describe where code
  goes, not what the service can do. A phase decomposed that way survives every check
  in this skill and then poisons `/plan-work`, which inherits the shape and splits a
  layer into sub-layers.

  Concretely: "load the chain list from configuration" is a layer. "return the current
  block of one hardcoded chain, end to end" is an outcome — and it makes the
  configuration work fall out of it, sized to what is actually needed.

  A slice that genuinely cannot be vertical exists (a dependency bump, a migration with
  no user-visible effect). It is allowed, but it must say so explicitly and it must
  **never be the first bullet of a phase** — the first one decides whether the phase can
  be validated at all.

Keep the existing Phase 0 (the template's baseline) as the worked example of a
satisfied DoD.

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

**Count check.** More ADRs than phases is a signal, not an achievement: re-read them and
apply the test above to each. Seven ADRs on a three-phase project means five of them are
ceremony, and ceremony has a cost — each one is a decision the user must arbitrate before
the slices that depend on it become implementable. Inflating the count converts an
approval gate into a queue.

**Demoted decisions are not lost.** Record them as a short *Decisions taken inline* list
in the relevant `docs/SPEC.md` section, or as a comment where the code makes the choice.
The trace survives; the ceremony does not.

### Drafting

List the surviving decisions in §3 with their target phase, then draft each one **in the
plan file**, from `docs/adr/0000-template.md`, using the next free number in
`docs/adr/`:

- `**Status**: Proposed`, `**Phase**` filled in.
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

## ⑥ Have the phases critiqued

Spawn `tech-lead` (Agent tool, `subagent_type: tech-lead`), **one call**, in **mode
"phases"** — say so explicitly in the brief, it changes which lenses it applies. Pass:

- the original need and the answers from step ②;
- the drafted phases with their *Definition of done*;
- the declared ADR backlog.

Wait for its verdict before handing back.

## ⑦ Hand back

In this order:

1. **The `tech-lead` verdict** and its findings. On `[LEAD]: BLOCKING`, propose the
   corrected phase decomposition rather than asking the user to arbitrate a problem that
   is already diagnosed.
2. The drafted `docs/SPEC.md` sections, the ADRs, and the new headers.
3. **The write order after approval**, explicitly:
   - `make rename MODULE_NEW=…` first;
   - then `docs/SPEC.md`, the ADRs, the `CLAUDE.md` and `README.md` headers;
   - then decide each ADR and set its `Status: Accepted` — a phase whose governing ADR
     is still `Proposed` is not implementable;
   - then `make check`, and only then the first `/plan-work`.

Then hand back for approval (`ExitPlanMode`).
