---
phase: "2"
slug: "error-classification-resourceexhausted-mapping"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-19"
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`, table-driven) + real-Qdrant integration via `internal/store/storetest` |
| **Config file** | none — `go.mod` / `Taskfile.yaml` |
| **Quick run command** | `go test ./internal/store/... ./internal/server/... ./cmd/engram/... -short -count=1` |
| **Full suite command** | `task` |
| **Estimated runtime** | ~300 seconds (full, real Qdrant) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/store/... ./internal/server/... ./cmd/engram/... -short -count=1`
- **After every plan wave:** Run `ENGRAM_REQUIRE_QDRANT=1 task`
- **Before `/gsd-verify-work`:** `task` must be green, plus a live-observed RED/GREEN pair for each lane's new test (store, Connect, MCP, CLI)
- **Max feedback latency:** 120 seconds (quick run)

---

## Per-Task Verification Map

Refined by the planner to the four plans' task IDs and concrete test names (one test or one
shared-prefix group per row, so every command is copy-paste runnable without regex alternation).
Re-resolve each `-run` against `go test -list` when auditing, and prove execution with `-v`
RUN/PASS pairs (durable record `bsbsvn4hbc`). Real-Qdrant rows run with `ENGRAM_REQUIRE_QDRANT=1`
so a missing Qdrant fails instead of skipping.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 02-01-T1 | 02-01 | 1 | REQ-exhausted-sentinel | T-02-01-04 | real overflow classified at the named limit; never a success-shaped page | integration (real Qdrant, both fixture shapes) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestStoreListOverflowIsResponseTooLarge$' -v -count=1` | ❌ W0 | ⬜ pending |
| 02-01-T2 | 02-01 | 1 | REQ-exhausted-sentinel | T-02-01-01, T-02-01-02 | non-matching ResourceExhausted returned as the identical value | unit (synthetic statuses: 4 receive shapes, 8 pass-through, nil; idempotency; concurrency) | `go test ./internal/store/ -run '^TestClassifyResponseTooLarge' -race -v -count=1` | ❌ W0 | ⬜ pending |
| 02-01-T2 | 02-01 | 1 | REQ-exhausted-sentinel | T-02-01-01 | caller interceptor sits inside the classifier; server-sent ResourceExhausted unrelabeled | integration (real client chain) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestResponseTooLargeClassifierSitsInsideCallerChain$' -v -count=1` | ❌ W0 | ⬜ pending |
| 02-02-T1 | 02-02 | 2 | REQ-exhausted-connect | T-02-02-01, T-02-02-04 | no raw gRPC/Qdrant text, no byte ceiling on the Connect wire; raw error logged once | integration (Connect over httptest + real Qdrant) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestConnectListMemoriesResponseTooLarge$' -v -count=1` | ❌ W0 | ⬜ pending |
| 02-02-T2 | 02-02 | 2 | REQ-exhausted-mcp | T-02-02-02, T-02-02-04 | envelope-only tool result; raw error logged once | integration (MCP in-memory transport + real Qdrant) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestMCPListMemoryResponseTooLarge$' -v -count=1` | ❌ W0 | ⬜ pending |
| 02-02-T2 | 02-02 | 2 | REQ-exhausted-mcp | T-02-02-02 | mapper registered innermost in the one production middleware call | source gate (go/parser) | `go test ./internal/server/ -run '^TestRegisterInstallsToolMiddleware$' -v -count=1` | ❌ W0 | ⬜ pending |
| 02-02-T3 | 02-02 | 2 | REQ-exhausted-connect | T-02-02-03 | non-relabeled ResourceExhausted still scrubbed to internal; new code distinct | unit (existing table, extended) | `go test ./internal/server/ -run '^TestConnectError$' -v -count=1` | ✅ (rows new) | ⬜ pending |
| 02-02-T3 | 02-02 | 2 | REQ-exhausted-mcp | T-02-02-05 | every other tool error keeps its exact text (D-09) | unit | `go test ./internal/server/ -run '^TestMapResponseTooLargePassesOtherResultsThrough$' -v -count=1` | ❌ W0 | ⬜ pending |
| 02-02-T3 | 02-02 | 2 | REQ-exhausted-connect, REQ-exhausted-mcp | T-02-02-01 | envelope carries no number, no upstream text, no false remedy | unit | `go test ./internal/server/ -run '^TestResponseTooLargeEnvelopeShape$' -v -count=1` | ❌ W0 | ⬜ pending |
| 02-03-T1 | 02-03 | 3 | REQ-exhausted-cli-docs | T-02-03-01 | N/A | CLI command path (stub Connect server) | `go test ./cmd/engram/ -run '^TestExitCodeBaseline$' -v -count=1` | ✅ (rows new) | ⬜ pending |
| 02-03-T1 | 02-03 | 3 | REQ-exhausted-cli-docs | T-02-03-02 | N/A | unit | `go test ./cmd/engram/ -run '^TestExitCodeTooLargeDistinct$' -v -count=1` | ❌ W0 | ⬜ pending |
| 02-03-T1 | 02-03 | 3 | REQ-exhausted-cli-docs | T-02-03-02 | N/A | unit (existing table, row edited) | `go test ./cmd/engram/ -run '^TestExitCodeForConnectErrTable$' -v -count=1` | ✅ | ⬜ pending |
| 02-03-T1 | 02-03 | 3 | REQ-exhausted-cli-docs | T-02-03-02 | N/A | unit (existing mechanical gate) | `go test ./cmd/engram/ -run '^TestCatalogExitCodesMatchMapper$' -v -count=1` | ✅ | ⬜ pending |
| 02-03-T1 | 02-03 | 3 | REQ-exhausted-cli-docs | T-02-03-02 | N/A | unit (existing gate, `wantExitCodes` edited) | `go test ./cmd/engram/ -run '^TestCatalogListsEveryExitCode$' -v -count=1` | ✅ | ⬜ pending |
| 02-03-T2 | 02-03 | 3 | REQ-exhausted-cli-docs | T-02-03-01 | N/A | unit (existing distinct-set test, extended) | `go test ./cmd/engram/ -run '^TestClassifyOperatorErrCodesAreDistinct$' -v -count=1` | ✅ | ⬜ pending |
| 02-03-T3 | 02-03 | 3 | REQ-exhausted-cli-docs | T-02-03-03 | N/A | doc gate (errors.md vs argerror.go) | `go test ./internal/server/ -run '^TestErrorsDocHintCodesMatchArgErrorConstants$' -v -count=1` | ❌ W0 | ⬜ pending |
| 02-03-T3 | 02-03 | 3 | REQ-exhausted-cli-docs | T-02-03-03 | N/A | unit (doc-gate parser self-test) | `go test ./internal/server/ -run '^TestParseHintCodeTable$' -v -count=1` | ❌ W0 | ⬜ pending |
| 02-04-T1/T2 | 02-04 | 4 | all four | T-02-04-01..03 | N/A | red-evidence harness (12 confirmed REDs) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -v -count=1 -timeout 30m` | ✅ | ⬜ pending |

The research-seeded `internal/e2e` `TestCLIExitCodes` row is intentionally dropped: `engram serve`
dials Qdrant with no named receive limit until Phase 5's REQ-recv-limit-backstop, so a binary-level
overflow would rest on grpc-go's default limit (rule `m45p2b4bp7`). The chain is covered in two halves
joined at the Connect wire (02-02-T1 real overflow → `resource_exhausted`; 02-03-T1 `resource_exhausted`
→ exit 10 through the real command path).

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/store/responsetoolarge_oversized_test.go` — real-overflow `Store.List` test (02-01-T1)
- [ ] `internal/store/responsetoolarge_test.go` — classifier unit, idempotency, concurrency, chain-position tests (02-01-T2)
- [ ] `internal/server/responsetoolarge_test.go` — Connect-lane and MCP-lane overflow tests, registration gate, pass-through and envelope-shape units (02-02)
- [ ] `internal/server/hintcodedocs_test.go` — errors.md ↔ `argerror.go` doc gate (02-03-T3)
- [ ] this phase's `redEvidenceDirs` entry + eight hand-verified patches (02-04)

---

## Manual-Only Verifications

| Behavior | Why manual | Where |
|----------|------------|-------|
| Remedy prose in `reference/errors.md`, `guides/cli.md` and `guides/upgrade.md` §14 correctly describes the flags and fields (edge probe row REQ-exhausted-cli-docs was `unclassified`; flagged assumption A-E1) | Prose correctness is not structurally checkable beyond the doc gate's table, count-word, envelope and anchor checks | 02-03-PLAN.md `<surfaced_assumptions>` |

Every other phase behavior has automated verification.

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
