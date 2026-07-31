#!/usr/bin/env bash
# PreToolUse hook (Bash matcher): deterministic guards on `git commit`.
# Only acts on commands containing `git commit`.
#
# Two CLAUDE.md rules that would otherwise be purely declarative:
#   1. "One branch per change, never commit directly on main" -> deny.
#   2. "Conventional Commits" (7 allowed types)               -> ask.
#
# Principle: this hook only decides what a script can decide without being wrong.
# On any invocation form it cannot read with certainty (message via -F, interactive
# commit, --amend with no -m, exotic quoting) it emits NO opinion and exits 0. A
# guardrail that blocks wrongly is as broken as one that never blocks.
#
# No jq dependency (absent from Git Bash on Windows): detection uses bash parameter
# expansion on the raw JSON received on stdin, and output is produced with printf.
# Requires only bash and git.

input=$(cat)

# Only concerns commits (substring search on the raw JSON).
case "$input" in
  *"git commit"*) ;;
  *) exit 0 ;;
esac

deny() {
  printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"%s"}}\n' "$1"
  exit 0
}

ask() {
  printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"ask","permissionDecisionReason":"%s"}}\n' "$1"
  exit 0
}

# --- Rule 1: never commit directly on the default branch ----------------------
# Deliberately first: it is the non-negotiable rule, and it needs no parsing of
# the commit message.

# Exception: the very first commit of a repository. With no HEAD yet there is no
# branch to move to, so blocking it would make the rule impossible to satisfy on
# a fresh clone or a `git init`. This is not a fail-open guess — the absence of
# HEAD is a fact the script reads, not an ambiguity it gives up on.
if ! git rev-parse --verify --quiet HEAD >/dev/null 2>&1; then
  exit 0
fi

branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)

case "$branch" in
  main|master)
    deny "Direct commit on '${branch}' is forbidden (CLAUDE.md rule: one branch per change). Create a branch: git checkout -b type/short-description — then commit again."
    ;;
esac

# --- Rule 2: Conventional Commits --------------------------------------------
# Message extraction without jq. In the raw JSON a shell double quote appears
# escaped (\"), hence the patterns below. Any unrecognised form exits 0: saying
# nothing beats saying something wrong.

after=${input#*git commit}

case "$after" in
  *-m*) msg_raw=${after#*-m} ;;
  *) exit 0 ;;  # no -m: nothing to validate here
esac

# Strip leading whitespace.
msg_raw=${msg_raw#"${msg_raw%%[![:space:]]*}"}

case "$msg_raw" in
  '\"'*)
    msg=${msg_raw#'\"'}
    msg=${msg%%'\"'*}
    ;;
  "'"*)
    msg=${msg_raw#"'"}
    msg=${msg%%"'"*}
    ;;
  *)
    exit 0  # unrecognised quoting: no opinion
    ;;
esac

[ -n "$msg" ] || exit 0

# The 7 types allowed by CLAUDE.md, with an optional scope: feat(transport): ...
# Add a type here AND in CLAUDE.md, or the two rules diverge.
case "$msg" in
  feat:*|fix:*|chore:*|docs:*|refactor:*|test:*|ci:*) exit 0 ;;
  feat\(*|fix\(*|chore\(*|docs\(*|refactor\(*|test\(*|ci\(*) exit 0 ;;
esac

ask "Commit message is not a Conventional Commit (CLAUDE.md rule): '${msg}'. Expected types: feat, fix, chore, docs, refactor, test, ci — optional scope, e.g. 'feat(transport): ...'. (Confirm to override.)"
