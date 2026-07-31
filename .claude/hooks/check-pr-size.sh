#!/usr/bin/env bash
# PreToolUse hook (Bash matcher): guardrail on PR size.
# Only acts on commands containing `gh pr create`.
#
# Counts the Go lines added on the branch vs main, TESTS INCLUDED, and BLOCKS
# (permissionDecision "ask" -> user confirmation) above 400.
# Rule: CLAUDE.md "<= 400 added lines of Go per PR, tests included".
#
# Tests are counted on purpose: the limit protects human review time, and a test
# file reads like the rest. Only generated code is excluded, since nobody reviews
# it. The prod/test breakdown is printed to situate an imbalance, but the TOTAL is
# what decides.
#
# No jq dependency (absent from Git Bash on Windows): detection by grep on the raw
# JSON received on stdin, output produced with printf. Requires only bash, git and
# awk.

LIMIT=400

# Generated code, excluded from the count. Keep this list IDENTICAL to the one in
# .claude/skills/open-pr/SKILL.md and to the command quoted in CLAUDE.md: three
# diverging commands would return three verdicts on the same rule.
# Example once sqlc is in use: EXCLUDES=(':(exclude)internal/store/db/**')
EXCLUDES=()

input=$(cat)

# Only concerns opening a PR (substring search on the raw JSON).
case "$input" in
  *"gh pr create"*) ;;
  *) exit 0 ;;
esac

# Comparison base: origin/main preferably, else local main.
if git rev-parse --verify --quiet origin/main >/dev/null 2>&1; then
  base="origin/main"
elif git rev-parse --verify --quiet main >/dev/null 2>&1; then
  base="main"
else
  exit 0
fi

sum_added() {
  awk '{ if ($1 ~ /^[0-9]+$/) s += $1 } END { print s + 0 }'
}

# Total subject to the limit: every Go line added, excluding generated code.
go_added=$(git diff --numstat "$base"...HEAD -- '*.go' "${EXCLUDES[@]}" 2>/dev/null | sum_added)

# Breakdown, purely informative.
prod=$(git diff --numstat "$base"...HEAD -- '*.go' ':(exclude)*_test.go' \
  "${EXCLUDES[@]}" 2>/dev/null | sum_added)
tests=$(git diff --numstat "$base"...HEAD -- '*_test.go' 2>/dev/null | sum_added)

go_added=${go_added:-0}
prod=${prod:-0}
tests=${tests:-0}

if [ "$go_added" -gt "$LIMIT" ]; then
  reason="PR too large to review: ${go_added} added lines of Go vs ${base} (limit ${LIMIT}, CLAUDE.md rule) — ${prod} production and ${tests} tests. Split into smaller vertical slices, or tighten the tests down to the cases that actually discriminate. (Confirm to override.)"
  printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"ask","permissionDecisionReason":"%s"}}\n' "$reason"
  exit 0
fi

printf '{"systemMessage":"PR size OK: %s added lines of Go (limit %s) — %s production + %s tests."}\n' "$go_added" "$LIMIT" "$prod" "$tests"
exit 0
