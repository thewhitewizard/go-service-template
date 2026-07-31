---
name: open-pr
description: Opens a PR cleanly — checks "one PR at a time", make check, size (≤ 400 lines of Go, tests included), branch and commit naming, then gh pr create. Never merges. Run when the code is ready and reviewed.
---

# Open a pull request

Run in order, **stop at the first failure** and report it to the user.

## 1. One PR at a time

`gh pr list --state open`. If a PR is already open, stop: the current one must be
reviewed, green and merged before the next one opens. Two open PRs on a solo project
means two review contexts held at once, and the second always gets the shallower pass.

## 2. Branch

Not on `main`. The name matches `type/short-description` — same type vocabulary as the
commits.

## 3. Green locally

`make check` (vet + lint + test). On failure, stop: never open a red PR. CI will find the
same thing five minutes later, having cost a round trip and a notification to everyone.

## 4. Size

Sum the additions of:

```
git diff --numstat origin/main...HEAD -- '*.go'
```

**Tests count.** The exclusion list must stay **identical** to the one in
`.claude/hooks/check-pr-size.sh` and to the command quoted in `CLAUDE.md`: three
diverging commands return three contradicting verdicts on the same rule. Today nothing is
excluded — add generated code here, in the hook, and in `CLAUDE.md`, in the same commit.

Above 400: do not open. Propose a split into vertical functional slices, and check the
production/test ratio — far above 1:1 signals redundant cases rather than extra coverage.

## 5. Commits

`git log origin/main..HEAD --format=%s` — every subject must be a Conventional Commit
(`feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `ci`, optional scope).

## 6. Create the PR

`gh pr create`, with:

- a title that is itself a Conventional Commit;
- a body containing: one or two sentences of context, a link to the relevant
  `docs/SPEC.md` phase and to any governing ADR, a **"How to test"** section with the
  actual commands, and the standard PR signature.

## 7. Hand back

Remind the user that the final review and the merge are theirs. **Never** run
`gh pr merge` and never delete the branch — both are denied in `.claude/settings.json`,
so attempting them fails loudly rather than quietly succeeding.
