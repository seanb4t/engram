---
phase: 3
slug: migration-foundation-registry-invariants-sweep
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-13
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (Go 1.x, stdlib testing) |
| **Config file** | none — `Taskfile.yaml` wraps lint + test as `task` |
| **Quick run command** | `go test ./internal/migrate/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~1s (migrate leaf, pure) · ~40s (full suite, testcontainers Qdrant) |

**Note on the two tiers.** `internal/migrate` is stdlib-only and has no Qdrant dependency (SC1),
so its tests are sub-second and can run after every task commit with no container cost. The
`internal/store` sweep tests require a real pinned Qdrant via testcontainers and belong to the
per-wave tier.

**Re-resolve every `-run` before trusting it.** `go test -run X` that matches nothing exits 0 with
`ok … [no tests to run]`. This repo has been bitten by that false green across two milestones
(STATE.md Blockers/Concerns; durable record `bsbsvn4hbc`). Every `-run` in this file must be
re-resolved against `go test -list` at execution time and proven with `-v` RUN/PASS pairs, never a
package-level `ok`.

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/migrate/...`
- **After every plan wave:** Run `go test ./internal/migrate/... ./internal/store/...`
- **Before `/gsd-verify-work`:** Full suite (`go test ./...`) must be green
- **Max feedback latency:** ~45 seconds

---

## Per-Task Verification Map

> Seeded at plan time from the phase success criteria. Task IDs are provisional until the planner
> emits PLAN.md files; `/gsd-validate-phase` reconciles them.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 3-01-* | 01 | 1 | REQ-migration-step-registry | — | N/A | unit | `go test ./internal/migrate/ -run TestValidate -v` | ❌ W0 | ⬜ pending |
| 3-01-* | 01 | 1 | REQ-migration-step-reversibility | — | N/A | unit | `go test ./internal/migrate/ -run TestIrreversible -v` | ❌ W0 | ⬜ pending |
| 3-0?-* | ?? | 2 | REQ-migration-additive-only-gated | — | N/A | unit | `go test ./internal/migrate/ -run TestAdditiveOnly -v` | ❌ W0 | ⬜ pending |
| 3-0?-* | ?? | 2+ | REQ-migrate-partial-failure-resume | — | N/A | integration | `go test ./internal/store/ -run TestMigratePartialFailure -v` | ❌ W0 | ⬜ pending |
| 3-0?-* | ?? | 2+ | REQ-migrate-converges-without-lock | — | N/A | integration | `go test ./internal/store/ -run TestMigrateConverges -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/migrate/migrate_test.go` — fixture steps exercising every registry invariant
      (conforming additive step, irreversible step with a stated reason, a key-removing step, and a
      step whose actual adds diverge from its declared `addsKeys`)
- [ ] `internal/store/migrate_sweep_test.go` — sweep tests against a real pinned Qdrant
- [ ] gRPC fault-injection interceptor, extending the existing seam in
      `internal/store/schemaversion_recallgate_test.go` (D-10) — do NOT resurrect the rejected
      `setPayloadKeys` test-hook field pattern

*Framework install: not required — `go test` and the testcontainers Qdrant harness already exist.*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

Every success criterion in this phase is machine-checkable: SC1 by an import/dependency assertion,
SC2/SC3 by compile-failure and registry-invariant tests, SC4/SC5 by integration tests against a
real pinned Qdrant with deterministic fault injection. Nothing here needs a human to look at a
screen.

---

## Non-Vacuity Requirements (phase-specific)

This phase's proofs are gates, and this milestone has already shipped a vacuous gate once
(Phase 01; durable record `x6v6qxqd6f`). Every gate below carries an explicit anti-vacuity
assertion, and these are validation requirements, not suggestions:

- [ ] The additive-only key-set diff (D-04) asserts a **non-zero fixture count** — a table that
      exercises zero fixtures is vacuously green (D-05).
- [ ] The additive-only diff asserts **set equality in both directions**
      (`before ⊆ after` AND `after − before == declared`), never a subset or `len(x) > 0` check.
- [ ] The backlog-derivation test seeds a record whose `schema_version` key is **genuinely absent**
      (raw payload injection bypassing `payload()`), because a test seeding only stamped records
      cannot distinguish the correct `Should:[Range, IsEmpty]` filter from a bare `Range` that
      silently derives an empty backlog (durable record `4syx1ggfxk`).
- [ ] Each RED cycle is proven by an exact reversible patch, committed under `red-evidence/`, with
      `git diff --exit-code` clean after revert — matching Phase 2's established practice.

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 45s
- [ ] Every `-run` re-resolved against `go test -list` and proven with `-v` RUN/PASS pairs
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
