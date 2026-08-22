---
phase: 6
slug: typed-operator-renderer
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-16
---

# Phase 6 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (`go test`) — no assertion library |
| **Config file** | none — `go.mod` alone (`go 1.26.3`) |
| **Quick run command** | `go test ./cmd/engram/... -run TestFieldSet -v` |
| **Full suite command** | `task test` (lint + `test:go` + `test:python`) |
| **Estimated runtime** | ~90 seconds (package-scoped quick run: ~10s) |

---

## Sampling Rate

- **After every task commit:** Run the converted report's own `-run` subset plus `go test ./cmd/engram/... -run TestFieldSet -v` (the mechanism's self-test — cheap, always run)
- **After every plan wave:** Run `go test ./cmd/engram/... -v`
- **Before `/gsd-verify-work`:** `task test` must be green
- **Max feedback latency:** 90 seconds

**False-green trap (record `bsbsvn4hbc`):** every `-run` pattern in this file MUST be re-resolved
against `go test -list` when the plan lands. A `-run` that matches nothing exits 0 with
`ok … [no tests to run]` and reports a false green forever. Prove execution with `-v` RUN/PASS
pairs, never a package-level `ok`.

---

## Per-Task Verification Map

| Task ID | Plan | Requirement | Test Type | Automated Command | Status |
|---------|------|-------------|-----------|-------------------|--------|
| 6-01 | 06-01 | REQ-operator-renderer-typed | unit | `go test ./cmd/engram/ -run TestRenderOperatorTextAndJSON -count=1` | ✅ green |
| 6-02 | 06-02 | REQ-operator-renderer-typed | unit | `go test ./cmd/engram/ -run TestOperatorOutputFormat -count=1` | ✅ green |
| 6-03 | 06-03 | REQ-operator-renderer-typed | unit | `go test ./cmd/engram/ -run TestOperatorViewFixturesCoverEveryOperatorCommand -count=1` | ✅ green |
| 6-04 | 06-04 | REQ-operator-renderer-typed | unit | `go test ./cmd/engram/ -run TestOperatorDocsAreHandDeclared -count=1` | ✅ green |
| 6-05 | 06-05 | REQ-operator-renderer-typed | unit | `go test ./cmd/engram/ -run TestOperatorViewIdentityAcrossEveryOperatorCommand -count=1` | ✅ green |
| 6-06 | 06-06 | REQ-operator-renderer-typed | unit | `go test ./cmd/engram/ -run TestOperatorViewFixturesHaveNoUnsanitizedNesting -count=1` | ✅ green |
| 6-07 | 06-07 | REQ-operator-renderer-typed | unit | `go test ./cmd/engram/ -run TestEveryOperatorCommandRejectsInvalidOutput -count=1` | ✅ green |
| 6-RE | validate-phase | REQ-operator-renderer-typed | integration | `go test ./internal/store/ -run TestRedEvidencePatchesAreLive -count=1` | ✅ green |
| 6-NV | 06-01 | REQ-operator-renderer-typed | unit (comparator non-vacuity) | `go test ./cmd/engram/ -run TestSetDiffDetectsDivergence -count=1` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Re-derived by `/gsd-validate-phase` on 2026-08-22 from the seven shipped SUMMARYs, not transcribed
from the seeded `6-TBD` row (record `1xe3ze1v9s`). Every `-run` pattern above was re-resolved by
execution: 20 subtest PASS lines, 0 FAIL, never a bare package-level `ok`.*

---

## Wave 0 Requirements

- [x] The shared operator-view mechanism and its own test file — shipped as `cmd/engram/operator_view.go` + `operator_view_test.go`, **not** as the drafted `cmd/engram/fieldset.go` / `fieldset_test.go`
- [x] A non-vacuity self-test for the set comparator — shipped as `TestSetDiffDetectsDivergence` (`operator_output_test.go:247`), covering missing / extra / disjoint / identical / empty
- [x] A bidirectional registry test proving the fixture set is set-equal to `operatorCommands()` in both directions — shipped as `TestOperatorViewFixturesCoverEveryOperatorCommand` (`:215`)

*Reconciled 2026-08-22. The drafted `FieldSet` + placeholder-parser design was written before any
PLAN.md existed and was never built; the phase took the walk-the-marshaled-bytes route instead
(`viewFields`, `operator_view.go:45`). The drafted file names are retained above only as the record
of what changed — they are not outstanding work.*

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Adding a new operator-report field touches exactly one `FieldSet`-builder function (SC3) | REQ-operator-renderer-typed | SC3 is a property about FUTURE ergonomics (Phase 7's six record-state fields), not a runtime-checkable invariant within this phase | At phase close, pick one converted report, add a throwaway field to its `FieldSet` builder only, and confirm it appears correctly in BOTH `--output text` and `--output json` with no second call site edited. Revert. Record the observation — do not claim it in prose alone (record `1xe3ze1v9s`: prose-only non-vacuity claims retain no artifact) |

---

## Red-Evidence Requirement

This milestone treats a gate that cannot go red as no gate at all (records `fqznw5nc1g`,
`bqpfcnrnjs`, `tm0s0h3wgy`, `01mdq5qq9j`). Phase 6's central claim — field-set identity holds *by
construction* — is exactly the kind of claim that can be vacuously "proven".

**Satisfied 2026-08-22.** Three patches under `red-evidence/`, all registered in `redEvidenceDirs`
and all confirmed RED by the live harness:

| Patch | Reddens | Direction |
|-------|---------|-----------|
| `06-red-1-empty-field-line-suppressed.patch` | `TestOperatorViewIdentityAcrossEveryOperatorCommand` | json key present, text suppressed |
| `06-red-2-text-lane-only-field.patch` | `TestOperatorViewIdentityAcrossEveryOperatorCommand` | text field with no json counterpart |
| `06-red-3-operator-command-dropped.patch` | `TestOperatorViewFixturesCoverEveryOperatorCommand` | command dropped from the enumerated registry |

All three are exact set/order-equality failures, never count or partition identities (record
`01mdq5qq9j`: a partition check was invariant under the very mutation it appeared to guard).

**Correction to this contract's original bullet 1.** The draft demanded a patch that adds "a
json-only field to a converted report's doc". **That mutation cannot redden any gate, by
construction, and no such patch can exist.** `viewFields` (`cmd/engram/operator_view.go:45-46`)
performs a single `json.Marshal(doc)` and the text lane walks those very bytes, so a field added to
a document struct widens *both* lanes identically and the identity gate correctly stays green.
That is the D-01/D-02 design working as intended, not a hole. The drafted mutation presumed the
`FieldSet` + placeholder-parser architecture, which was never built. The retained patches attack
the property that genuinely can break — the single-derivation invariant itself — in both
directions.

Phase 06 is registered in `redEvidenceDirs` (`internal/store/redevidence_harness_test.go`). That
map fails closed within a listed directory but fails OPEN across directories, so registration was
a hard prerequisite: before it, a patch would never have been executed (issue #501, record
`bqpfcnrnjs`).

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (reconciled — the drafted design was never built)
- [x] No watch-mode flags
- [x] Feedback latency < 90s (package-scoped run: 0.38s)
- [x] Every `-run` pattern re-resolved by execution — 20 subtest PASS lines, 0 FAIL, never a bare package-level `ok`
- [x] Red-evidence patches retained (3) and Phase 06 listed in `redEvidenceDirs`
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-08-22 by `/gsd-validate-phase 06`

---

## Validation Audit 2026-08-22

| Metric | Count |
|--------|-------|
| Gaps found | 2 |
| Resolved | 2 |
| Escalated | 0 |

Gaps closed: (1) no red-evidence patch for phase 06; (2) phase 06 absent from `redEvidenceDirs`,
where the map fails open across directories. Both compounded — a patch without registration is
never executed, and registration without a patch trips the zero-applicability guard.

One finding recorded rather than silently worked around: this contract's original red-evidence
bullet named a mutation that is impossible under the shipped architecture. See the correction in
the Red-Evidence Requirement section above.

Verified by `go test ./internal/store/ -run TestRedEvidencePatchesAreLive -count=1 -v` — the
phase-06 subtest executes and passes; all three patches log `confirmed RED`; the working tree
reverts clean.
