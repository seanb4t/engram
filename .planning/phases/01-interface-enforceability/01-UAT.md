---
status: complete
phase: 01-interface-enforceability
source: [01-01-SUMMARY.md, 01-02-SUMMARY.md, 01-03-SUMMARY.md, 01-04-SUMMARY.md, 01-05-SUMMARY.md, 01-06-SUMMARY.md, 01-07-SUMMARY.md, 01-08-SUMMARY.md, 01-09-SUMMARY.md]
started: 2026-08-04T02:15:00Z
updated: 2026-08-04T02:15:00Z
---

## Current Test

number: 12
name: Migration prose adequate for a reader upgrading a running deployment
expected: |
  `guides/upgrade.md`'s `## Unreleased` section gives an operator upgrading a live
  deployment enough to act: every changed exit status named, each with who-should-act
  framing, and no surprise left implicit.
awaiting: user response

## Tests

### 1. Paging trio rejected before any dial
expected: `engram list --scope s --offset 5 --page-token x` exits 2 without a network call
result: pass
source: observed (exit 2)

### 2. Zero-value paging combination also rejected
expected: `engram list --scope s --offset 0 --page-token ''` exits 2 — cobra flag groups count a flag being supplied, not its value
result: pass
source: observed (exit 2)

### 3. `--cross-spine=false` with `--scope` rejected
expected: `engram search --scope s --cross-spine=false --query q` exits 2, same supplied-not-value semantics
result: pass
source: observed (exit 2)

### 4. Unknown flag is a usage error
expected: `engram --definitely-not-a-real-flag` exits 2, not the old 1
result: pass
source: observed (exit 2)

### 5. Unknown verb stays on the deliberate exit-1 backstop
expected: `engram definitely-not-a-real-verb` exits 1 — cobra's Find() fails before execute(), structurally unreachable from the interception point
result: pass
source: observed (exit 1)

### 6. migrate-remap-owner requires exactly one source flag
expected: `engram migrate-remap-owner` with no source flag exits 2 (MarkFlagsOneRequired)
result: pass
source: observed (exit 2)

### 7. Client `--timeout 0` rejected
expected: `engram search --scope s --timeout 0 --query q` exits 2 — D-05 requires a finite deadline
result: pass
source: observed (exit 2)

### 8. migrate `--timeout 0` semantic reversal
expected: `migrate-remap-owner --timeout 0` and the hidden `migrate-set-owner --timeout 0` both exit 2; previously 0 meant unbounded
result: pass
source: observed (exit 2 for both)

### 9. Unreachable backend exits 5
expected: `engram search --server http://127.0.0.1:1` exits 5
result: pass
source: observed (exit 5)

### 10. Hung server exits 6, distinct from 5, and returns
expected: against a listener that accepts and never responds, `--timeout 2s` returns within the window and exits 6 — never confused with a closed port's 5
result: pass
source: observed (exit 6, returned in 2s; closed port exit 5 in the same run)
note: this is edge E-11, the backstop marker carried from 01-08's must_haves — confirmed by direct observation rather than a timing-race unit test

### 11. Exit-code taxonomy advertised from the constants
expected: seven constants (0/1/2/3/4/5/6) with `exitTimeout = 6`; catalog built from constants, never a second literal
result: pass
source: automated (TestCatalogListsEveryExitCode, TestCatalogExitCodesMatchMapper, TestCatalogClaimsNoFlagErrorExitsGeneric — all pass)

### 12. Migration prose adequate for a live-deployment upgrade
expected: `## Unreleased` names every changed exit status with who-should-act framing
result: pass
source: user
note: |
  Human judgment — carried from 01-09's coverage block. User verdict: "reads more as
  release notes, but it's readable." Passed with that critique acted on: all seven
  sections already carried `**Who should act:**` (matching the v0.12.0 precedent), so
  the gap was organizational — the guide was ordered by what changed, forcing a reader
  through all seven sections to learn whether any of it touched them. Added a
  "Do you need to act?" triage table above §1 routing each reader class to the sections
  that apply, including an explicit "only run engram interactively → no action" row.
  Existing prose unchanged.

## Summary

Auto-covered by passing tests (7 of 9 SUMMARYs, `all_auto_covered: true`): 01-01, 01-02,
01-03, 01-04, 01-07 coverage blocks; 01-05 and 01-06 are legacy-mode and were covered by
direct CLI observation above (tests 5, 6, 8).

11 of 12 tests pass by direct observation against a binary built at HEAD. 1 pending human
judgment.

## Gaps

None found during UAT. The one gap found earlier by phase verification
(`internal/e2e/cli_exitcode_test.go` asserting the pre-unification exit 1 for an unknown
flag) was fixed and committed in `05991bc7` before this session; test 4 above re-confirms
the corrected behavior from the user's side.
