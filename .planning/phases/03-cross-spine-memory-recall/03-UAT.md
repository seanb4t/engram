---
status: complete
phase: 03-cross-spine-memory-recall
source: 03-01-SUMMARY.md, 03-02-SUMMARY.md, 03-03-SUMMARY.md, 03-04-SUMMARY.md, 03-05-SUMMARY.md
started: 2026-08-02T00:00:00Z
updated: 2026-08-02T00:00:00Z
---

## Current Test

[testing complete]

## Tests

### 1. searched_scopes is documented as coverage, not hit-distribution
expected: All three surfaces state the authorized-to-read/spanned meaning and explicitly negate the produced-hits reading
result: pass
evidence: |
  All three surfaces carry both the positive statement AND the explicit negation:
  - docs-site/.../reference/tools.md:117 (search) — "every scope you can read that the search
    spanned, not the scopes that produced hits"
  - docs-site/.../reference/tools.md:147 (list) — "every scope you can read that the list
    spanned, not the scopes that produced results"
  - skill/engram/skills/curating-memory/SKILL.md:277 — "name the scopes searched under your
    authorization — not the scopes that had results"
  The negation is what matters here: a reader cannot come away with the hit-distribution reading,
  which is the misreading 03-VALIDATION.md's Known Precision Note exists to prevent (a scope where
  every record is superseded contributes zero hits yet still legitimately appears, because the
  field reports the span the query covered).

## Summary

total: 1
passed: 1
issues: 0
pending: 0
skipped: 0
blocked: 0

## Auto-Covered (source: automated)

19 of 20 coverage deliverables across this phase's five plans are deterministically covered by
passing automated tests and are recorded as `result: pass, source: automated` — they are not
presented as checkpoints. Re-verified live during `/gsd-validate-phase 3` on 2026-08-02: 4 store
tests against real Qdrant and 6 server tests, all `--- PASS:`, 0 fail, 0 skip, run under
`ENGRAM_REQUIRE_QDRANT=1` so a silent skip could not pass as success.

## Gaps

<!-- none yet -->
