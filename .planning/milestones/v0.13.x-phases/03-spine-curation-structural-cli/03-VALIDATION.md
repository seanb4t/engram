---
phase: 3
slug: spine-curation-structural-cli
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-06
validated: 2026-08-07
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Seeded from `03-RESEARCH.md` § Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (+ `testcontainers-go`, already a dependency via `internal/store/store_test.go`) |
| **Config file** | none — `go test` flags only (`-shuffle`, `-count`, `-run`) |
| **Quick run command** | `go test ./cmd/engram/... ./internal/store/... ./internal/surfaces/...` |
| **Full suite command** | `go clean -testcache && task test` |
| **Estimated runtime** | ~30s quick; full suite dominated by testcontainers Qdrant startup |

**Why the full command carries `go clean -testcache`:** Go's test cache keys on a package's own
inputs, so `task test` replays a cached PASS for `internal/e2e` — which shells out to the built
binary — after a behavior change in `cmd/engram`. This phase changes `cmd/engram` behavior that
`internal/e2e` exercises, so the cache flush is mandatory at any phase-completion or pre-PR gate,
not optional hygiene.

---

## Sampling Rate

- **After every task commit:** `go test ./cmd/engram/... ./internal/store/... ./internal/surfaces/...`
- **After every plan wave:** `task test` (full suite, go + python)
- **Before `/gsd-verify-work`:** `go clean -testcache && task test` must be green
- **Max feedback latency:** ~30 seconds (package-scoped quick run)

**Determinism requirement — golden/snapshot tests over `cmd/engram`:** any snapshot of the cobra
tree is order-dependent (cobra registers a command's own `-h`/`--help` lazily inside `execute()`,
and `rootCmd` + subcommands are shared singletons across the test binary; combined with Go's
nil-vs-empty slice marshalling this produced a golden test that passed in isolation and failed on
2 of 3 `-shuffle` seeds). Every run touching `TestCatalogGolden` / `TestHelpGolden` must be
stressed with several `-shuffle=<seed>` values — a single green run is not evidence. D-01's nested
subcommand tree makes this strictly worse by adding depth, so it is a phase-level gate, not a
per-task nicety.

---

## Per-Task Verification Map

*Filled by `/gsd-validate-phase 3` on 2026-08-07 against the seven merged plans. Verification is
per-plan rather than per-task: every plan's tasks land in one wave and share that wave's suite,
so a per-task row would repeat the same command seven times over.*

| Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | Tests Matched | Status |
|------|------|-------------|------------|-----------------|-----------|-------------------|---------------|--------|
| 03-01 | 1 | REQ-spine-scan | T-03-02 | Subject-less sweep spans every owner; zero mutation on any path | integration (testcontainers) | `go test ./internal/store/... -run TestScanSpine` | 4 | ✅ green |
| 03-02 | 2 | REQ-operator-output-flag | — | `--output` adopted without unifying the client-vs-operator `--timeout` divergence | unit (golden + table-driven) | `go test ./cmd/engram/... -run 'TestHelpGolden\|TestCatalogGolden\|TestOperatorOutput\|TestTimeoutGroupMatrix'` | 10 | ✅ green |
| 03-03 | 3 | REQ-destructive-preview-default | T-03-02 | `registerDestructive` owns `RunE`; membership derived from the blast-radius table, no injectable seam | unit (table-driven over `surfaces.Operations()`) | `go test ./cmd/engram/... -run TestDestructive` | 5 | ✅ green |
| 03-04 | 4 | REQ-citation-drift-verify | T-03-04 | resolved (not lexical) path containment — absolute, `..`, and symlink escape all refused | unit (pure) + integration | `go test ./cmd/engram/... -run TestVerify` | 11 | ✅ green |
| 03-05 | 5 | REQ-near-duplicate-report | — | stored-vector query only (no re-embedding); never merges or mutates | integration (testcontainers) | `go test ./internal/store/... -run TestNearDuplicates` | 14 | ✅ green |
| 03-06 | 6 | REQ-archive-tier | T-03-17 | `archived_at` orthogonal to `not_after`; `Archive`/`Restore` take the same lock as `Update` | integration (testcontainers) | `go test ./internal/store/... -run 'TestArchive\|TestRestore'` | 11 | ✅ green |
| 03-07 | 7 | REQ-purge-extract-gated | T-03-01 | cross-package manifest forgery rejected before any RPC; intersection-only delete | unit (cross-package forgery) + integration | `go test ./internal/store/... -run 'TestPurgeManifest\|TestApplyPurge\|TestCheckExtractGate\|TestPurgeFilterPathActive'` | 6 | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**Counting method.** "Tests Matched" is the number of `--- PASS` lines the command emits under
`-v`, not merely that it exited 0. `go test -run <pattern>` exits **0 with "no tests to run"** when
the pattern matches nothing, so an exit-status check alone would report green for seven entirely
fictional test names. The counts above are what distinguishes "the command succeeded" from "the
tests exist and passed" — the same trap class as the vacuous `rg` gates this phase's convergence
cycles removed.

**Cross-cutting acceptance.** `internal/e2e/spine_review_test.go`'s `TestE2EPhaseAcceptance`
exercises all seven ROADMAP success criteria against a seeded 270-record multi-owner, multi-page
collection, asserting via `store.Get` re-reads rather than exit codes alone.

---

## Requirement → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists |
|--------|----------|-----------|-------------------|-------------|
| REQ-spine-scan | `scan` reports inventory/health, zero mutation on any path | integration (testcontainers) | `go test ./internal/store/... -run TestScanSpine -v` | ✅ 4 tests |
| REQ-citation-drift-verify | excerpt-anchored tier classification (valid / moved / broken-split-by-cause / unverifiable) | unit (pure function) + integration | `go test ./cmd/engram/... -run TestVerify -v` | ✅ 11 tests |
| REQ-near-duplicate-report | ranked `(A, B, score)` pairs, no mutation, no re-embedding | integration (testcontainers) | `go test ./internal/store/... -run TestNearDuplicates -v` | ✅ 14 tests |
| REQ-purge-extract-gated | manifest forgery rejected before any RPC; intersection-only delete; eligibility re-derived at apply | unit (cross-package forgery, pure) + integration (delete) | `go test ./internal/store/... -run 'TestPurgeManifest\|TestApplyPurge\|TestCheckExtractGate\|TestPurgeFilterPathActive' -v` | ✅ 6 tests |
| REQ-archive-tier | `archived_at` observably distinct from `not_after` and `superseded_by`; visible via `get_memory`; hidden from `Search`/`List` | integration (testcontainers) | `go test ./internal/store/... -run 'TestArchive\|TestRestore' -v` | ✅ 11 tests |
| REQ-destructive-preview-default | `--apply` required for every command the blast-radius table marks destructive — derived, not declared | unit (table-driven over the operations table) | `go test ./cmd/engram/... -run TestDestructive -v` | ✅ 5 tests |
| REQ-operator-output-flag | `--output json\|text` on all six operator commands with `--timeout` semantics untouched | unit (golden + table-driven) | `go test ./cmd/engram/... -run 'TestHelpGolden\|TestCatalogGolden\|TestOperatorOutput\|TestTimeoutGroupMatrix' -v` | ✅ 10 tests |

**One correction to the seeded plan.** The seed named
`TestVerifyFileCitation`, `TestApplyPurgeIntersection` and `TestOperatorOutputFlag`; the delivered
test names differ. Since `go test -run` on a non-matching pattern exits 0, keeping the planned
names would have produced a permanently green, permanently vacuous row. The commands above are the
ones that actually match.

---

## Wave 0 Requirements

All delivered — `wave_0_complete: true`.

- [x] `internal/store/spine_test.go` — covers REQ-spine-scan, REQ-near-duplicate-report,
      REQ-purge-extract-gated (manifest + apply), REQ-archive-tier
- [x] `cmd/engram/spine_review_test.go` — citation-verification pure function, per-leaf
      `--output` / `--apply` flag wiring, `resetCommandFlagState` pairing for every new leaf
- [x] `cmd/engram/catalog_test.go` / `golden_test.go` — the recursive `walkCommands` helper is
      genuinely shared: `catalog.go:98` and `golden_test.go:80` both call
      `walkCommands(root, commandWalkSkip)`, so the catalog and the golden walker cannot drift
      apart at depth. Fixtures cover all six `spine-review` leaves.
- [x] `internal/surfaces/toolclass_test.go` — qualified-path-keyed operation rows plus the
      table-driven "derived, not declared" assertion for REQ-destructive-preview-default
- [x] `internal/surfaces/rules_test.go` — rules for `--fail-on` and the filter-path scope
      requirement, each asserting `ApplicableSurfaces` resolves non-empty
- [x] `internal/store/spine_forgery_test.go` — **not in the original seed.** Added because the
      `PurgeManifest` guarantee only holds cross-package: Go forbids setting an unexported field
      from another package, so a same-package literal would prove nothing. Declares
      `package store_test`.
- Framework install: none — `testing` + `testcontainers-go` already present.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `prune-expired`'s breaking default flip reads correctly to an operator upgrading | REQ-destructive-preview-default | The deliverable is prose comprehension in `docs-site/src/content/docs/guides/upgrade.md`; no assertion proves an operator understood it | Read the upgrade guide's `prune-expired` entry cold and confirm it states (a) the old behavior, (b) the new behavior, (c) the exact flag to restore deletion, alongside Phase 1's exit-code migration |
| `--output text` renders legibly at a real TTY | REQ-operator-output-flag | TTY auto-detection resolves differently under a captured pipe than at a terminal; the unit test pins the mapping, not the rendering | Run each of the six operator commands at an interactive terminal and confirm text (not JSON) is emitted, then pipe each to `cat` and confirm the format flips |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references — every file delivered
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] Golden/catalog tests stressed across multiple `-shuffle=<seed>` runs (not one green run) —
      seeds 11 / 29 / 47 at the final gate, plus per-wave runs recorded in each SUMMARY
- [x] `go clean -testcache && task` green at the phase gate
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-08-07

---

## Validation Audit 2026-08-07

| Metric | Count |
|--------|-------|
| Requirements audited | 7 |
| COVERED | 7 |
| PARTIAL | 0 |
| MISSING | 0 |
| Gaps found | 0 |
| Resolved | 0 (none needed) |
| Escalated | 0 |
| Passing tests matched across the seven requirement commands | 61 |

No `gsd-nyquist-auditor` dispatch was needed: the audit found zero MISSING or PARTIAL
requirements, so there were no gaps to fill. Every requirement's coverage was confirmed by
counting `--- PASS` lines rather than trusting exit status, because `go test -run` on a
non-matching pattern exits 0 — an exit-status audit would have passed seven fictional test names.

Two manual-only items remain (unchanged, and independent of Nyquist compliance): the
`upgrade.md` cold-read and real-TTY `--output` rendering. Both were **waived** at phase
verification on 2026-08-07 rather than performed — see `03-VERIFICATION.md`'s
`human_verification_waived` block. They are recorded here as still genuinely unverified.
