---
phase: 3
slug: spine-curation-structural-cli
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-06
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

*Seeded as `draft` — task IDs are assigned by the planner. `/gsd-validate-phase 3` fills this table
against the written PLAN.md files and flips `status: validated`.*

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| *pending* | — | — | — | — | — | — | — | — | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Requirement → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists |
|--------|----------|-----------|-------------------|-------------|
| REQ-spine-scan | `scan` reports inventory/health, zero mutation on any path | integration (testcontainers) | `go test ./internal/store/... -run TestScanSpine -v` | ❌ W0 |
| REQ-citation-drift-verify | excerpt-anchored tier classification (valid / moved / broken-split-by-cause / unverifiable) | unit (pure function) | `go test ./cmd/engram/... -run TestVerifyFileCitation -v` | ❌ W0 |
| REQ-near-duplicate-report | ranked `(A, B, score)` pairs, no mutation, no re-embedding | integration (testcontainers) | `go test ./internal/store/... -run TestNearDuplicates -v` | ❌ W0 |
| REQ-purge-extract-gated | manifest forgery rejected at compile time; intersection-only delete; eligibility re-derived at apply | unit (forgery, pure) + integration (delete) | `go test ./internal/store/... -run 'TestPurgeManifest\|TestApplyPurgeIntersection' -v` | ❌ W0 |
| REQ-archive-tier | `archived_at` observably distinct from `not_after` and `superseded_by`; visible via `get_memory`; hidden from `Search`/`List` | integration (testcontainers) | `go test ./internal/store/... -run 'TestArchive\|TestRestore' -v` | ❌ W0 |
| REQ-destructive-preview-default | `--apply` required for every command the blast-radius table marks destructive — derived, not declared | unit (table-driven over the operations table) | `go test ./cmd/engram/... -run TestDestructiveCommandsRequireApply -v` | ❌ W0 |
| REQ-operator-output-flag | `--output json\|text` on all six operator commands with `--timeout` semantics untouched | unit (golden + table-driven) | `go test ./cmd/engram/... -run 'TestHelpGolden\|TestCatalogGolden\|TestOperatorOutputFlag' -v` | ✅ goldens exist; new test names are W0 |

---

## Wave 0 Requirements

- [ ] `internal/store/spine_test.go` — new file; covers REQ-spine-scan, REQ-near-duplicate-report,
      REQ-purge-extract-gated (manifest + apply), REQ-archive-tier
- [ ] `cmd/engram/spine_review_test.go` — new file; covers the citation-verification pure function,
      per-leaf `--output` / `--apply` flag wiring, and `resetCommandFlagState` pairing for every new
      leaf command
- [ ] `cmd/engram/catalog_test.go` / `golden_test.go` extension — a recursive `walkCommands` helper
      shared by `buildCatalog` and the golden walker; new fixtures covering all six `spine-review`
      leaves
- [ ] `internal/surfaces/toolclass_test.go` extension — six qualified-path-keyed operation rows plus
      a table-driven assertion that `Class.Destructive` on the `purge` row alone drives `--apply`
      gating (the "derived, not declared" proof for REQ-destructive-preview-default)
- [ ] `internal/surfaces/rules_test.go` extension — rules for D-14's `--fail-on` and D-10's
      filter-path scope requirement, each asserting `ApplicableSurfaces` resolves non-empty
- Framework install: none — `testing` + `testcontainers-go` already present.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `prune-expired`'s breaking default flip reads correctly to an operator upgrading | REQ-destructive-preview-default | The deliverable is prose comprehension in `docs-site/src/content/docs/guides/upgrade.md`; no assertion proves an operator understood it | Read the upgrade guide's `prune-expired` entry cold and confirm it states (a) the old behavior, (b) the new behavior, (c) the exact flag to restore deletion, alongside Phase 1's exit-code migration |
| `--output text` renders legibly at a real TTY | REQ-operator-output-flag | TTY auto-detection resolves differently under a captured pipe than at a terminal; the unit test pins the mapping, not the rendering | Run each of the six operator commands at an interactive terminal and confirm text (not JSON) is emitted, then pipe each to `cat` and confirm the format flips |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] Golden/catalog tests stressed across multiple `-shuffle=<seed>` runs (not one green run)
- [ ] `go clean -testcache && task test` green at the phase gate
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
