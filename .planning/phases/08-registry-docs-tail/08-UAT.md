---
status: complete
phase: 08-registry-docs-tail
source: 08-01-SUMMARY.md, 08-02-SUMMARY.md, 08-03-SUMMARY.md, 08-04-SUMMARY.md, 08-05-SUMMARY.md, 08-06-SUMMARY.md
started: 2026-08-22T01:31:23Z
updated: 2026-08-22T02:05:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Sweep-scope rule declared once and composed by all three leaves
expected: RuleSweepScopeOrAllScopesRequired is declared once in internal/surfaces/rules.go and composed by all three sweep leaves in both their rejection path and their --all-scopes help text
result: pass
source: automated
coverage_id: D1

### 2. Zero hand-rolled guard literals remain under cmd/engram/
expected: Zero hand-rolled occurrences of the removed guard literal remain under cmd/engram/
result: pass
source: automated
coverage_id: D2

### 3. surfaces:gen regeneration is a fixed point
expected: task surfaces:gen regeneration is a fixed point and the goldens moved only in the composed --all-scopes Usage text
result: pass
source: automated
coverage_id: D3

### 4. tools.md anchored-region prose reads as a sentence
expected: The prose surrounding the new anchor in docs-site/reference/tools.md reads as a genuine sentence, not an orphaned fragment
result: pass

### 5. memory-record.md documents the full record state
expected: memory-record.md's Field reference table has a row for every wire-visible store.Memory key (8 new rows), a new validity-window section deriving both boundary directions from activeWindowConditions, a new schema-version section stating the three-part forward-compatibility guarantee with its two narrowings, and the Archiving section's false Connect-lane claim corrected with the tracker citation removed
result: pass

### 6. tools.md get_memory state vocabulary corrected
expected: tools.md's get_memory section — expired bullet no longer says not_after is "in the past" (off-by-one fixed to the exclusive at-or-before wording), the four state words are reordered to canonical archived/superseded/expired/scheduled, schema_version is added, and the 08-01 anchored region is untouched
result: pass

### 7. guides/migrate.md documents the migration mechanism end to end
expected: docs-site/guides/migrate.md documents engram migrate, migrate status, migrate revert, and migration-status end to end — mechanism, preview/apply contract, convergence and re-run semantics in the code's own terms, both revert refusal forms, flags, and every json key of the three CLI report structs plus the Connect lane's pending field
result: pass

### 8. guides/upgrade.md section 12 no longer denies the forward sweep
expected: guides/upgrade.md's schema-version release note (section 12) no longer asserts the forward sweep is unavailable; it names engram migrate --apply and links to the new guide while keeping the rollback hazard and its additive-only reasoning intact
result: pass

### 9. CLAUDE.md migrations convention states scope and automation contract
expected: The Conventions list's migrations bullet states what this milestone ships (schema-version-driven registry, additive-only steps, mandatory reversibility, swept by engram migrate), the automation contract (never applies automatically; what IS automatic is the read-only startup MigrateStatus probe), and the deliberate boundary (migrate-remap-owner/summarize-missing/reindex are not version-driven and do not appear in the registry or status histogram)
result: pass

### 10. CLAUDE.md Layout row names all 23 commands with a client/operator tier split
expected: The cmd/engram/ Layout row names every one of the 23 top-level commands in cmd/engram/testdata/catalog.golden, grouped by parent verb, split into client-tier (get, search, list, store, migration-status — reach a running server over Connect) and operator-tier (reindex, migrate family, migrate-remap-owner/migrate-set-owner, prune-expired, summarize-missing, backfill-short-ids, spine-review family — act on Qdrant directly), with migrate-set-owner marked deprecated as the alias of migrate-remap-owner rather than a peer verb
result: pass

### 11. CLAUDE.md Memory contract names schema_version and the archived state
expected: The Memory contract section names schema_version (server-set on every write, absent reads as version 0, never gates recall) and the archived state (archived_at, engram spine-review archive/restore, and the four-word canonical vocabulary archived/superseded/expired/scheduled with the expired-suppresses-scheduled precedence), using the same wording reference/memory-record.md uses, staying prose with no table or list introduced
result: pass

### 12. Layout row marks every deprecated command derived from the goldens
expected: cmd/engram/ Layout row marks every deprecated command derived from the committed goldens (backfill-short-ids and migrate-set-owner), not just one
result: pass
source: automated
coverage_id: D1

### 13. Archived-state paragraph names all four soft-hide recall surfaces
expected: Archived-state paragraph names all four soft-hide recall surfaces, agreeing with the Supersession paragraph and the store's live archived_at gate-site count
result: pass
source: automated
coverage_id: D2

### 14. Sweep rule doc comment claims only enforcement that exists
expected: internal/surfaces/rules.go's RuleSweepScopeOrAllScopesRequired doc comment claims only enforcement that exists (retired present-tense envelope claim removed; replacement gated against code with a non-vacuity control)
result: pass
source: automated
coverage_id: D1

### 15. TestNoHandRolledSweepScopeGuards walks the live command tree
expected: TestNoHandRolledSweepScopeGuards exists, runs under go test ./..., derives the live command set from walkCommands (never a hand-listed set), and refuses to pass on an empty walk
result: pass
source: automated
coverage_id: D2

### 16. Leaf classification declared once at package level
expected: The classification (enforcingSweepLeaves/nonEnforcingSweepLeaves) is declared once at package level; each of the four sweep-review leaf names appears exactly once in cmd/engram/sweep_scope_test.go outside comment lines
result: pass
source: automated
coverage_id: D3

### 17. Gate observed RED against a constructed defect
expected: The gate has been OBSERVED failing against a deliberately constructed defect (throwaway command with inline hand-rolled guard), not merely observed passing, with the RED output recorded verbatim, in a single repeatable script
result: pass
source: automated
coverage_id: D4

### 18. Rule comment reads as contributor-facing explanation
expected: The rewritten rule comment reads as a contributor-facing explanation of a not-yet-wired lane, not an apology or changelog entry
result: pass

## Summary

total: 18
passed: 18
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none — all 18 checkpoints passed]
