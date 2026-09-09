---
status: testing
phase: 03-runtime-registration
source: [03-VERIFICATION.md]
started: 2026-09-09T17:03:30Z
updated: 2026-09-09T17:03:30Z
---

## Current Test

number: 1
name: claude-code converges — `--apply` twice, then a third time after manual deletion
expected: |
  Run 1: outcome=wrote. Run 2 (state unchanged): outcome=already-correct.
  Between runs 1 and 2 there is a real (tolerant-remove-then-fatal-add) window where the
  registration briefly does not exist — this is by design, not a bug.
awaiting: user response

## Tests

### 1. claude-code converges — `--apply` twice, then a third time after manual deletion

Run `engram setup --apply --runtime claude-code` twice in a row against a real Claude Code install
with no prior engram registration, then a third time after manually deleting the entry.

expected: Run 1: outcome=wrote. Run 2 (state unchanged): outcome=already-correct. Between runs 1 and 2 there is a real (tolerant-remove-then-fatal-add) window where the registration briefly does not exist — this is by design, not a bug.
result: [pending]

### 2. codex and opencode converge

Run `engram setup --apply --runtime codex` twice, then `engram setup --apply --runtime opencode`
twice, against real installs.

expected: codex: run 1 wrote, run 2 already-correct (its `mcp add` overwrites silently and `mcp get --json` is a pure local read, so already-correct should be the common case). opencode: run 2 is expected to report wrote far more often than already-correct, because `opencode mcp list` dials every registered server live and any one flip differs the two probe captures — this is the documented safe-direction-only degradation, not a defect.
result: [pending]

### 3. Drifted flag surface fails legibly

Point `--runtime` at a claude/codex/opencode binary that has since removed or renamed a flag this
package depends on (e.g. an intentionally broken PATH entry pointing at a wrapper script that
rejects `--transport` or `--bearer-token-env-var`), then run `engram setup --apply`.

expected: outcome=failed with Reason naming the runtime, the exact argv issued, the nonzero exit code, and the runtime's stderr verbatim — never a silent no-op.
result: [pending]

## Summary

total: 3
passed: 0
issues: 0
pending: 3
skipped: 0
blocked: 0

## Gaps
