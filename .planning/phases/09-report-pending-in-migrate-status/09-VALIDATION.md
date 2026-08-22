---
phase: 9
slug: report-pending-in-migrate-status
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-22
---

# Phase 9 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go stdlib `testing`; no external test framework) |
| **Config file** | none — `go test` reads package files directly |
| **Quick run command** | `go test ./cmd/engram/... -run 'TestMigrateFamilyStatusReportDocKeyOrder\|TestMigrateFamilyStatusReportDocNeverMarshalsNull\|TestOperatorViewIdentityAcrossEveryOperatorCommand\|TestOperatorDocsAreHandDeclared\|TestOperatorOutputEmpty' -v -count=1 > out.log 2>&1; status=$?; cat out.log; exit $status` |
| **Full suite command** | `task test:go` (`go test ./...`) |
| **Estimated runtime** | ~1 second (quick run measured at 0.301s on 2026-08-22 over the four pre-existing tests, 30 RUN/PASS lines, exit 0 — a real run, not `[no tests to run]`). Full suite unmeasured; Qdrant-backed integration tests dominate it. |

**Redirect-to-file shape is deliberate, not stylistic.** `cmd 2>&1 \| tail -N; echo $?` reports
`tail`'s exit status, not the command's, and a gate shaped that way passes unconditionally. Separately,
`go test -run <NonexistentName>` exits **0** with `[no tests to run]`, and agent-rendered Bash output
drops `=== RUN` lines — so a green console is not evidence the test ran. Capture to a file, test
`$status` directly, and count RUN/PASS lines in the file before believing a pass.

---

## Sampling Rate

- **After every task commit:** Run the quick run command above.
- **After every plan wave:** Run `task test:go`.
- **Before `/gsd-verify-work`:** Full suite must be green, plus `task` (lint + test).
- **Max feedback latency:** ~1 second for the quick run.

---

## Per-Task Verification Map

Task IDs are assigned by the planner; this table is seeded with the behaviors the research pass
mapped to requirements. The planner fills `Task ID` / `Plan` / `Wave` when PLAN.md files are written.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| {planner} | {planner} | {planner} | REQ-migrate-status-histogram | — | N/A | unit | `go test ./internal/store/... -run 'TestMigrateStatusResultPending$\|TestMigrateStatusResultPendingZeroValue' -v` (pre-existing; proves the source-of-truth method, unaffected by this phase) | ✅ `internal/store/migrate_status_test.go:29,49` | ⬜ pending |
| {planner} | {planner} | {planner} | REQ-migrate-status-histogram | — | N/A | unit | new discriminating-fixture test: `migrateStatusReportDoc.Pending` equals `res.Pending()` where three naive re-derivations each yield a different number | ❌ W0 | ⬜ pending |
| {planner} | {planner} | {planner} | REQ-migrate-status-histogram | — | N/A | unit | `go test ./cmd/engram/... -run TestMigrateFamilyStatusReportDocKeyOrder -v` (existing; extended with `"pending"` last) | ✅ `cmd/engram/migrate_family_test.go:420` | ⬜ pending |
| {planner} | {planner} | {planner} | REQ-migrate-status-histogram | — | N/A | unit | `go test ./cmd/engram/... -run TestMigrateFamilyStatusReportDocNeverMarshalsNull -v` (existing, unchanged — regression guard) | ✅ `cmd/engram/migrate_family_test.go:409` | ⬜ pending |
| {planner} | {planner} | {planner} | REQ-migrate-status-histogram | — | N/A | unit | `go test ./cmd/engram/... -run TestOperatorViewIdentityAcrossEveryOperatorCommand -v` (existing; picks up the new field via the shared fixture) | ✅ | ⬜ pending |
| {planner} | {planner} | {planner} | REQ-migrate-status-histogram | — | N/A | unit | `go test ./cmd/engram/... -run TestOperatorDocsAreHandDeclared -v` (existing; scalar field does not trip the embedding check) | ✅ | ⬜ pending |
| {planner} | {planner} | {planner} | REQ-docs-record-state | — | N/A | doc gate (Go test over file content) | new zero-occurrence gate: `the equivalent number from` and `Connect lane only` each absent from `docs-site/`; both verified at exactly 1 occurrence today, both at `guides/migrate.md:279` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] New discriminating unit test for D-05, co-located with `TestMigrateFamilyStatusReportDocKeyOrder`
      in `cmd/engram/migrate_family_test.go`. Version numbers MUST be built `cur`-relative
      (`cur := int(migrate.CurrentVersion)`), never as literals — the convention
      `internal/store/migrate_status_test.go:29-45` already follows. `migrate.CurrentVersion` is `1`,
      so the fixture needs a bucket **strictly below** current; a fixture whose only bucket sits *at*
      current never exercises `Pending()`'s bucket loop and a plain `pending = Absent` re-derivation
      would pass it.
- [ ] New W3 doc-content gate for D-07. Locate this repo's existing docs-content-assertion tests first
      (`rg -l "docs-site" cmd/engram/*_test.go`); if none exists, a new plain Go test file is fine —
      `docs-site/**` markdown is hand-authored, not tool-generated, so the "never invent structure in
      a tool-owned file" convention does not apply here.
- [ ] Positive control for the D-07 gate: inject `the equivalent number from` (and separately
      `Connect lane only`) into a scratch copy and assert the gate goes RED. A gate observed only
      passing is not evidence.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Rendered `engram migrate status --output text` field table shows a `Pending` row last | REQ-migrate-status-histogram | The reflection-driven renderer is already covered by the view-identity test; a human read confirms the label `humanizeKey("pending")` produces reads correctly in context | Run `engram migrate status --output text` against any collection and confirm `Pending` appears as the final field row with the same value as `--output json`'s `pending` key |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
