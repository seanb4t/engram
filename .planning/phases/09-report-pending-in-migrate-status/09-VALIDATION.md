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
| **Post-phase quick run** | `go test ./cmd/engram/... -run 'TestMigrateFamilyStatusReportDocPendingNeverRederived\|TestMigrateFamilyStatusSummaryPendingClause\|TestMigrateFamilyStatusReportDocKeyOrder\|TestMigrateFamilyStatusReportDocNeverMarshalsNull\|TestMigrateGuidePendingRow\|TestOperatorViewIdentityAcrossEveryOperatorCommand\|TestOperatorDocsAreHandDeclared' -v -count=1 > /tmp/gsd-09-quick.log 2>&1; status=$?; PASSES=$(rg -o -- '--- PASS' /tmp/gsd-09-quick.log \| wc -l \| tr -d ' '); FAILS=$(rg -o -- '--- FAIL' /tmp/gsd-09-quick.log \| wc -l \| tr -d ' '); echo "exit=$status passes=$PASSES fails=$FAILS"; test "$status" -eq 0 && test "$FAILS" -eq 0` — the seeded quick run above names only pre-existing tests and stays valid as a baseline; this one is what the phase's own gates run against. |
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

Filled by the planner on 2026-08-22 from `09-01-PLAN.md` and `09-02-PLAN.md`. Task IDs are
`{plan}-T{n}`.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 09-01-T1 | 09-01 | 1 | REQ-migrate-status-histogram | — | N/A | unit | `go test ./internal/store/... -run 'TestMigrateStatusResultPending$\|TestMigrateStatusResultPendingZeroValue' -v` (pre-existing; proves the source-of-truth method, unaffected by this phase — reference only, not re-verified) | ✅ `internal/store/migrate_status_test.go:29,49` | ⬜ pending |
| 09-01-T1 | 09-01 | 1 | REQ-migrate-status-histogram | — | N/A | unit | `go test ./cmd/engram/... -run TestMigrateFamilyStatusReportDocPendingNeverRederived -v` — five subtests: `equals_store_pending`, `fixture_discriminates_every_naive_rederivation`, `zero_value_marshals_pending_zero`, `converter_is_pure`, `renders_last_in_both_lanes` | ❌ W0 | ⬜ pending |
| 09-01-T1 | 09-01 | 1 | REQ-migrate-status-histogram | — | N/A | unit | `go test ./cmd/engram/... -run TestMigrateFamilyStatusReportDocKeyOrder -v` (existing; `want` extended with `"pending"` last, D-06) | ✅ `cmd/engram/migrate_family_test.go:420` | ⬜ pending |
| 09-01-T1 | 09-01 | 1 | REQ-migrate-status-histogram | — | N/A | unit | `go test ./cmd/engram/... -run TestMigrateFamilyStatusReportDocNeverMarshalsNull -v` (existing, unchanged — regression guard) | ✅ `cmd/engram/migrate_family_test.go:409` | ⬜ pending |
| 09-01-T1 | 09-01 | 1 | REQ-migrate-status-histogram | — | N/A | unit | `go test ./cmd/engram/... -run 'TestOperatorViewIdentityAcrossEveryOperatorCommand\|TestMigrateViewIdentity' -v` (existing; picks up the new field via the shared fixture, D-08) | ✅ | ⬜ pending |
| 09-01-T1 | 09-01 | 1 | REQ-migrate-status-histogram | T-09-01 | Operator report stays hand-declared, so record content is unreachable by construction | unit | `go test ./cmd/engram/... -run TestOperatorDocsAreHandDeclared -v` (existing; a `uint64` scalar does not trip the embedding check) | ✅ | ⬜ pending |
| 09-01-T2 | 09-01 | 1 | REQ-migrate-status-histogram | — | N/A | unit | `go test ./cmd/engram/... -run TestMigrateFamilyStatusSummaryPendingClause -v` — three subtests: `ordering_tokens_are_unambiguous`, `states_pending_value_between_buckets_and_future`, `emitted_unconditionally_at_zero` (D-03) | ❌ W0 | ⬜ pending |
| 09-02-T1 | 09-02 | 2 | REQ-docs-record-state | T-09-05 | The gate that closes a vacuous-gate finding is itself proven discriminating | doc gate (Go test over file content) | `go test ./cmd/engram/... -run TestMigrateGuidePendingRow -v` — expected NON-zero exit at this task: `TestMigrateGuidePendingRowIsAccurate` RED against the live guide while `TestMigrateGuidePendingRowGateFiresOnInjectedViolation` passes all seven cases | ❌ W0 | ⬜ pending |
| 09-02-T2 | 09-02 | 2 | REQ-docs-record-state | T-09-06 | Corrected row names the `future` exclusion an operator is most likely to get wrong | doc gate (Go test over file content) | `go test ./cmd/engram/... -run TestMigrateGuidePendingRow -v` (now green) plus the scoped zero-occurrence sweep `rg -o -F <anchor> docs-site/ \| wc -l` returning 0 for each of the two anchors | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] **Owner: 09-01-T1** — `TestMigrateFamilyStatusReportDocPendingNeverRederived`. New discriminating unit test for D-05, co-located with `TestMigrateFamilyStatusReportDocKeyOrder`
      in `cmd/engram/migrate_family_test.go`. Version numbers MUST be built `cur`-relative
      (`cur := int(migrate.CurrentVersion)`), never as literals — the convention
      `internal/store/migrate_status_test.go:29-45` already follows. `migrate.CurrentVersion` is `1`,
      so the fixture needs a bucket **strictly below** current; a fixture whose only bucket sits *at*
      current never exercises `Pending()`'s bucket loop and a plain `pending = Absent` re-derivation
      would pass it.
- [ ] **Owner: 09-01-T2** — `TestMigrateFamilyStatusSummaryPendingClause`. New unit test for D-03,
      asserting on values and positions rather than the sentence's wording (which carries no
      stability contract per D-04): ordering tokens unambiguous, `40` before `97` before `5`, and a
      pending clause still present for a zero-valued result.
- [ ] **Owner: 09-02-T1** — `cmd/engram/migrate_docs_test.go`. New W3 doc-content gate for D-07.
      Resolved at plan time: the repo's only existing docs-content-assertion test is
      `cmd/engram/docsync_test.go`, which is scoped to the upgrade guide by its constant name and
      test name, so the new gate lands in a sibling file rather than rescoping that one.
      `docs-site/**` markdown is hand-authored, not tool-generated, so the "never invent structure in
      a tool-owned file" convention does not apply here.
- [ ] **Owner: 09-02-T1** — `TestMigrateGuidePendingRowGateFiresOnInjectedViolation`, the positive
      control for the D-07 gate: a seven-case table over in-memory fixtures (never a file mutation)
      driving the same `migrateGuidePendingRowViolations` helper the real-file gate calls. The
      `clean` case is as load-bearing as the six violation cases — a gate that fires on everything
      discriminates nothing. 09-02-T1 additionally observes the gate RED against the real,
      unmodified guide before the fix lands.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| ~~Rendered `engram migrate status --output text` field table shows a `Pending` row last~~ | REQ-migrate-status-histogram | **MECHANIZED at plan time — no longer manual.** 09-01-T1's `renders_last_in_both_lanes` subtest asserts `viewFields`' final element has `Key` `pending` and `Label` `Pending`, renders through `renderOperatorView` and asserts the label and value appear, and encodes the same doc through a `json.Encoder` asserting the same value. Same fixture, both lanes, in-process — no live Qdrant needed. | (retained only as an optional smoke check once the deployed instance is rolled forward: run `engram migrate status --output text` against a real collection and confirm the `Pending` row is last and matches `--output json`'s `pending` key) |

No verification in this phase is manual-only.

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
