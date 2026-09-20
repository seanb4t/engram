---
phase: "05"
slug: "operator-sweeps-ci-backstop"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-20"
---

# Phase 05 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

**Seeded without a RESEARCH.md** (research skipped — the mechanism is already built and proven by
Phases 3–4). Every test name below in the "existing" column was resolved against `go test -list`
on 2026-09-20 and EXISTS. Names for tests this phase will write are deliberately left as
`TO-RESOLVE` rather than guessed: a `-run` pattern matching nothing exits 0 with `no tests to run`
and reports a permanent false green (durable records `gfh6q1ack4`, `bsbsvn4hbc`, and Phase 4's own
recurrence, where two of five seeded names never existed). `/gsd-validate-phase 5` resolves every
`TO-RESOLVE` against `go test -list` and rewrites this map.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib); real Qdrant via `internal/store/storetest` |
| **Config file** | none — `Taskfile.yaml` `test:*` tasks are the entry points |
| **Quick run command** | `go test -short ./internal/store/...` |
| **Full suite command** | `task` (lint + whole-repo test, includes the real-Qdrant `internal/store` suite) |
| **Estimated runtime** | ~60 s quick · ~5 min full |

---

## Sampling Rate

- **After every task commit:** `go test -short ./internal/store/...`
- **After every plan:** `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -count=1`
- **Phase gate:** full `task` green
- **Max feedback latency:** 90 seconds

---

## Per-Requirement Verification Map

| Requirement | Behavior to prove | Existing coverage (verified to exist) | New test | Status |
|-------------|-------------------|----------------------------------------|----------|--------|
| REQ-sweeps-bounded | Each of `migrate`, `migrate revert`, `summarize-missing`, `spine-review scan`/`verify`/`purge`, `reindex` completes over a scope whose 256-record pages would exceed the receive limit | — (new behavior) | `TO-RESOLVE` — one real-Qdrant regression per sweep, both `storetest.SeedOversized` shapes | ⬜ pending |
| REQ-bounded-read-mechanism | Every inventory site routes through a shared primitive or carries a written exemption; `unbudgetedView` has no callers | Inventory closing checks (a)–(d) in `03-INVENTORY.md`; `TestRecallEmissionSetIsCompleteAndClassified` (exists) | `TO-RESOLVE` — the zero-count gate for check (b) | ⬜ pending |
| REQ-recv-limit-backstop | `NewQdrantClient` passes the configured `MaxCallRecvMsgSize` into its dial options; `storetest.Dial` passes `RecvLimit`. **Pass-through only** — never that gRPC enforces a ceiling (D-06, rule `m45p2b4bp7`) | `TestQdrantClientConstructedOnlyByNewQdrantClient` (exists), `TestQdrantClientIsHeldOnlyByStorePackage` (exists) | `TO-RESOLVE` — the dial-option pass-through assertion | ⬜ pending |
| REQ-ci-store-green | #497 closed on #498's existing evidence (D-07) — no new test; this is a recorded reasoning step, not a behavior | `TestQdrantImageMatchesCIService` (exists) | none by design | ⬜ pending |
| **Semantics preserved (D-02)** | The four migrated own-loop sweeps behave identically; any break must be explainable as an expected change | `TestMigrateFullBacklogProjection`, `TestMigrateManifestIntersection`, `TestMigrateManifestSparedDeletedRecord`, `TestMigrateManifestBacklogAppeared`, `TestMigrateDryRunAndManifestMutuallyExclusive`, `TestMigrateRevertPartialFailureReconciliation`, `TestMigrateRevertFixtureInjectionConverges`, plus the full `migrate_test.go` (14), `revert_test.go` (9), `summarize_test.go` (8), `reindex_test.go` (17), `spine_test.go` (35) — **all verified to exist** | none — the in-place suites ARE the characterization (D-02) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Per-sweep real-Qdrant regressions (five commands), `package store_test`, both fixture shapes
- [ ] The dial-option pass-through assertion for the backstop
- [ ] The `unbudgetedView` zero-caller gate (inventory closing check (b))
- [ ] Framework install: none — the harness exists from Phase 1

---

## Manual-Only Verifications

*None expected — every behavior this phase changes is reachable from `internal/store`'s own suite.
If a sweep's oversized regression proves impractical to seed, record it here rather than dropping it.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Every `TO-RESOLVE` replaced with a name resolved against `go test -list`
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] No watch-mode flags
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
