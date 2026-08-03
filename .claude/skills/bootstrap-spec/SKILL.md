---
name: bootstrap-spec
description: One-time project start — a few questions, then three sentences of context and the first slice in docs/SPEC.md, plus the CLAUDE.md and README headers. Deliberately minimal: everything else accretes as slices get built. Run once on a fresh clone, then go straight to /plan-work.
argument-hint: "<what the service does, in a sentence or two>"
---

# Start a new service from this template

Run **once**, on a fresh clone. Then `/plan-work` takes over, one slice at a time.

**This skill is deliberately small, and that is the design.** A specification is worth what
you already know, and at hour zero you know least. Writing the whole context, the whole ADR
backlog and the whole slice list before a line of code exists produces the heaviest document
at the moment of maximum ignorance — and it costs a morning before the first pull request.
So: enough to start slice 1, and nothing more. The rest accretes as slices get built.

**Target: under five minutes and one round of questions.** If you find yourself drafting a
fourth section or a second ADR, you have left the scope of this skill.

## ⓿ Switch to plan mode

- **Already in plan mode** → go to ①.
- **Otherwise** → call `EnterPlanMode` **immediately, without asking**. The tool requests
  the user's consent itself; asking beforehand would ask the same question twice.

In plan mode only the plan file is writable, so this skill creates **no** file in the
repository. Everything below is drafted in the plan file and written after `ExitPlanMode`.

## ① Ask only what blocks slice 1

**One `AskUserQuestion` call, at most four questions.** Not five topics — four questions,
chosen because you cannot write the first slice without their answers. Usually:

- **Who calls this service**, and does the first slice need to be protected?
- **What is the one thing it must do first**, stated as something a caller can observe?
- **Which external dependency does that need**, if any?
- **Anything the first slice must deliberately not do?**

Do not ask what `CLAUDE.md`, `docs/SPEC.md` or an existing ADR already answers. Do not ask
about the stack, the logging format or the metrics backend — the template settled those.
Do not ask about anything beyond slice 1: those questions are `/plan-work`'s, asked when the
slice that needs them comes up, with the codebase in front of you.

## ② Draft three sections, and stop

**§1 — Context and goals.** Three sentences: what the service does, who calls it, what it
explicitly does not do. Not a page.

**§6 — Work.** One entry under *Next*, and one line each under *Later*:

```markdown
### Next

### 1. <what a caller can do that they could not before>

- **Size**: ~<n> lines of Go, tests included
- **Definition of done**: <a criterion that would FAIL if the implementation were wrong>

### Later — intent, not plan

- <one line>
- <one line>
```

Two rules on that single entry, and they are the whole quality bar of this skill:

- **The title names an outcome, not a layer.** *What can an outside caller do once this is
  merged that they could not before?* "Nothing — it enables the next one" means it is a
  layer. A title taken from the directory tree — *configuration*, *client*, *store*,
  *middleware* — is the smell. "Load the config" is a layer; "return the current head, end
  to end" is an outcome, and it pulls in exactly the configuration it needs.
- **The *Definition of done* is falsifiable.** "The service works" is not. Something a test
  fails on is. Keep latency and availability targets out of it — those are SLOs, they belong
  in §9 if you ever want that section, and they cannot fail today.

Everything under *Later* stays one line: no size, no DoD. Those get written when the entry
moves up to *Next*, by `/plan-work`, against the code as it is then.

**Headers.** Replace the placeholder header of `CLAUDE.md` (title and the two-line pitch)
and the "what this is" section of `README.md`. Cheap, one-time, and it stops the repository
describing the template.

## ③ No ADR here, and no critique here

**Do not draft an ADR.** A decision is recorded by `/plan-work`, when the slice that depends
on it is planned — because that is when you know enough to name the options, and because an
ADR drafted now is a gate the user must clear before any code, for a decision the code has
not yet forced.

If the interview surfaced something structural, write it as **one line under *Later***:
`decision to take: <the question>`. That records it without turning it into a queue.

**Do not spawn `tech-lead`.** There is nothing here worth five minutes of critique: three
sentences and one slice. Its lenses are about the split, and the split is `/plan-work`'s
output — that is where it earns its cost, and where it will run.

## ④ Hand back

Show the three drafts, then say plainly what happens after approval:

1. write `docs/SPEC.md` §1 and §6, and the two headers;
2. `make check` — must be green before any of your own code;
3. `/plan-work "<the slice 1 title>"` — and that is where the questions, the split and the
   critique happen.

Then `ExitPlanMode`.
