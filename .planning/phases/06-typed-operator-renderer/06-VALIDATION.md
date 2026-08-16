---
phase: 6
slug: typed-operator-renderer
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
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

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 6-TBD | TBD | TBD | REQ-operator-renderer-typed | — | N/A | unit | TBD — filled when PLAN.md files land | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Task rows are seeded as TBD because no PLAN.md exists yet. `/gsd-validate-phase` resolves them
against the plans' actual task IDs. Per record `1xe3ze1v9s`, seeded `N-TBD` rows do NOT
auto-resolve — they must be re-derived, not transcribed.*

---

## Wave 0 Requirements

- [ ] `cmd/engram/fieldset.go` — `FieldSet`/`Field` types, `validateFieldSet`, the shared placeholder parser, `FieldSet.MarshalJSON`, and the text-render walk (zero regression risk: nothing calls it yet)
- [ ] `cmd/engram/fieldset_test.go` — unit tests for the above, including `TestFieldSetCoverage_CatchesWidening` (the mandatory red-proof mutation test) and its symmetric companion proving a MISSING placeholder for a declared field is also caught
- [ ] A bidirectional registry test proving the report-constructor registry is set-equal to `operatorCommands()` in BOTH directions — authored during the conversion, but MUST NOT replace `TestOperatorOutputParity` until all 15 reports have converted (D-07 retirement is the last step, not a Wave 0 action)

*Framework and per-report test files already exist; the only genuine Wave 0 gap is the new
mechanism's own test file.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Adding a new operator-report field touches exactly one `FieldSet`-builder function (SC3) | REQ-operator-renderer-typed | SC3 is a property about FUTURE ergonomics (Phase 7's six record-state fields), not a runtime-checkable invariant within this phase | At phase close, pick one converted report, add a throwaway field to its `FieldSet` builder only, and confirm it appears correctly in BOTH `--output text` and `--output json` with no second call site edited. Revert. Record the observation — do not claim it in prose alone (record `1xe3ze1v9s`: prose-only non-vacuity claims retain no artifact) |

---

## Red-Evidence Requirement

This milestone treats a gate that cannot go red as no gate at all (records `fqznw5nc1g`,
`bqpfcnrnjs`, `tm0s0h3wgy`, `01mdq5qq9j`). Phase 6's central claim — field-set identity holds *by
construction* — is exactly the kind of claim that can be vacuously "proven".

- `TestFieldSetCoverage_CatchesWidening` MUST be demonstrated RED by a real mutation (add a
  json-only field to a converted report's doc and observe the failure), and the demonstration MUST
  be retained as a checked-in patch under this phase's red-evidence directory — not described in
  prose.
- Phase 06's directory MUST be added to `redEvidenceDirs` in
  `internal/store/redevidence_harness_test.go`. That map is **declared, not discovered**: it fails
  closed within a listed directory and fails OPEN across directories, so an unlisted Phase 06 is
  silently ungated (issue #501, record `bqpfcnrnjs`).
- Prefer exact SET EQUALITY over count/partition identities. Record `01mdq5qq9j` documents a
  partition check that was invariant under the very mutation it appeared to guard.

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] Every `-run` pattern re-resolved against `go test -list` (not transcribed from this draft)
- [ ] Red-evidence patch retained and Phase 06 listed in `redEvidenceDirs`
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
