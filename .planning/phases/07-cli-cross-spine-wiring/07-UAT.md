---
status: complete
phase: 07-cli-cross-spine-wiring
source: 07-01-SUMMARY.md, 07-02-SUMMARY.md, 07-03-SUMMARY.md
started: 2026-08-02T00:00:00Z
updated: 2026-08-02T00:00:00Z
---

## Current Test

[testing complete]

## Tests

### 1. CLI reference makes the scope-or-cross-spine rule learnable by reading
expected: The four facts (scope-or-cross-spine, mutual exclusion, exit code, coverage footer) are all stated, and no example demonstrates a call that exits 2 (D-08)
result: pass
evidence: |
  All four facts are stated explicitly in `docs-site/src/content/docs/guides/cli.md`
  § Recall scope selection:

  1. Scope-or-cross-spine required — "each require exactly one of two flags — there is no default
     that silently picks one for you" (:45)
  2. Mutual exclusion — stated on BOTH table rows, so a reader landing on either flag learns it
     without having to read the other (:50-51)
  3. Exit code — "Passing neither, or passing both, is rejected by the CLI itself — before any
     network call — with exit `2`" (:53)
  4. Coverage footer — "It prints only on a `--cross-spine` call — output for every other
     invocation is unchanged, byte-for-byte" (:85)

  D-08 holds: both invocation examples (:19 search, :20 list) carry `--scope`, so no documented
  example would exit 2.

  The prose also explains WHY the CLI is stricter than the server (the server would accept the
  pair and silently discard the scope, logging at Info where the agent never sees it) — which
  serves the correct-by-reading principle in memory 4aksmneehh: the reader learns the rule and its
  rationale without running anything.

## Summary

total: 1
passed: 1
issues: 0
pending: 0
skipped: 0
blocked: 0

## Auto-Covered (source: automated)

3 coverage deliverables from 07-03 are recorded as `result: pass, source: automated`.

Plans 07-01 and 07-02 carry no `coverage:` block (they use a `decisions:` key), so they fall to
legacy prose extraction. Their recorded decisions are entirely non-observable implementation
notes — where a test file landed, a `resetClientFlags` leak fix, the ordering of an error check
against a footer write — which the workflow's legacy path explicitly skips. Their user-facing
outcomes (the `--cross-spine` flag, the shared guard, the coverage footer) are the same surface
test 1 and 07-03's auto-covered entries verify, so they contribute no additional checkpoint
rather than being silently dropped.

Re-verified live during `/gsd-validate-phase 7` on 2026-08-02: 12 top-level tests `--- PASS:`,
0 fail, 0 skip, plus all six docs gates exact.

## Gaps

<!-- none yet -->
