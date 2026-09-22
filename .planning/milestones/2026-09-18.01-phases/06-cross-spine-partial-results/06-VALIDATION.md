---
phase: "06"
slug: "cross-spine-partial-results"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-20"
validated: "2026-09-20"
---

# Phase 06 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

Seeded without a RESEARCH.md (research skipped — the milestone's own `ARCHITECTURE.md` already mapped
all four call sites and the design). The seed carried NO invented test names: every new test was left
`TO-RESOLVE`, because a `-run` pattern matching nothing exits 0 with `no tests to run` and reports a
permanent false green (`gfh6q1ack4`, `bsbsvn4hbc`; Phase 4 hit this with two of five seeded names).
**All seven names below were resolved against `go test -list` on 2026-09-20 and their match counts
verified (5 and 2).**

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib). This phase's proofs need NO live Qdrant — the `ListScopes` failure is injected with a single-method store double (the `lostRaceStore` template) |
| **Config file** | none — `Taskfile.yaml` `test:*` tasks are the entry points |
| **Quick run command** | `go test -short ./internal/server/... ./cmd/engram/...` |
| **Full suite command** | `task` (lint + whole-repo test) |
| **Actual runtime** | `internal/server` 52.6 s · `cmd/engram` 5.2 s · lint 0 issues |

---

## Sampling Rate

- **After every task commit:** `go test -short ./internal/server/... ./cmd/engram/...`
- **After every plan:** `go test ./internal/server/... ./cmd/engram/... ./internal/keylinks/ -count=1`
- **Phase gate:** `task lint` + the three packages above, all verified green.
- **Proto note:** a proto change requires `task proto:gen` and committing the regenerated `gen/go`, `gen/ts` and `ui/src/lib/gen` trees; CI checks for drift.
- **`internal/store` caveat:** the full package is NOT reliably green on a loaded dev machine — see the Environmental Limitation below. It needs `-timeout 180m` locally, and this phase changes only `redevidence_harness_test.go` within it.

---

## Per-Requirement Verification Map

REQ-cross-spine-partial is this phase's only requirement; the rows decompose it by behavior.

| Behavior proven | Type | Automated Command | Tests matched | Status |
|-----------------|------|-------------------|---------------|--------|
| Hits are RETURNED, not discarded, when `ListScopes` fails after a cross-spine recall produced them — on all four surfaces (MCP `search_memory`/`list_memory`, Connect `SearchMemories`/`ListMemories`) | integration | `go test ./internal/server/ -run '^(TestCrossSpineCoverageUnknownConnectSearch\|TestCrossSpineCoverageUnknownConnectList\|TestCrossSpineCoverageUnknownMCPSearch\|TestCrossSpineCoverageUnknownMCPList)$' -count=1 -v` | 4 | ✅ green |
| The three states stay distinguishable on BOTH transports: non-cross-spine (no keys) · coverage known (`searched_scopes` + `scopes_truncated`) · coverage unknown (`scopes_unknown` true, `searched_scopes` **ABSENT**, never an empty list) | integration | `go test ./internal/server/ -run '^TestCrossSpineCoverageThreeStates$' -count=1 -v` | 1 | ✅ green |
| `renderCoverageFooter` prints `scopes_unknown: true` with NO count, on both verbs, via a full CLI round trip | integration | `go test ./cmd/engram/ -run '^(TestClientListCoverageUnknownFooter\|TestClientSearchCoverageUnknownFooter)$' -count=1 -v` | 2 | ✅ green |
| The forbidden fix is refused: making `ListScopes` swallow its own error turns the contract test RED | red-evidence | `06-01-helper-swallows-listscopes-error.patch`, registered in `redEvidenceDirs`; hand-executed apply→RED→revert during verification | 1 patch | ✅ green |
| The proto change is additive and the generated trees are in sync | structural | `buf breaking` (FILE mode) in the `buf` CI job; `connectdescriptor_test.go`'s per-field tables (bumped to 7 and 4); `git diff --exit-code` over `gen/` after `task proto:gen` | existing gates | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**Regression set that had to stay green and did:** `TestSearchedScopesReporting`, `TestListMemoryCrossSpineIsolation`, `TestSearchMemoryCrossSpineIsolation`, `TestSearchMemoriesConnectCrossSpine`, `TestCrossSpineResultScope`, `TestConnectCrossSpineNotInferred`, `TestConnectCrossSpineScopeRequired`, `TestClientListFooterUnchangedWithoutCrossSpine`, `TestClientSearchNoFooterWithoutCrossSpine`.

---

## Wave 0 Requirements

All satisfied:

- [x] An injectable `ListScopes` failure — solved with the `lostRaceStore` single-method-override template, wired via `&deps{st: wrapper}` (MCP) and `&engramAPI{d: d}` (Connect). No live Qdrant needed.
- [x] Per-surface partial-result tests (4), in `internal/server/crossspinecoverage_test.go`
- [x] The three-state distinguishability test (1)
- [x] The two CLI round-trip footer tests
- [x] Five red-evidence patches registered (harness 53 → 58)
- [x] Framework install: none required

---

## Manual-Only Verifications

*None.* Every behavior is reachable from `internal/server` and `cmd/engram` tests, and the failure
injection needs no live backend.

---

## Environmental Limitation (not a coverage gap)

The full-package `internal/store` run is not currently reproducible-green on this machine. At load
~290 the Qdrant **testcontainer** died mid-run with `connection refused / code = Unavailable` in
`TestSummarizeMissingBoundedOverGRPCLimit` (669 s), and a bare `task` run separately hit Go's 600 s
default package timeout (603 s). **The same test passes in 6.35 s in isolation**, so this is
environmental, not a defect — and this phase changes exactly one file in that package
(`redevidence_harness_test.go`).

Worth distinguishing from #497/#498: those fixed the **CI** path, replacing four testcontainers on a
2-vCPU runner with one shared `services:` container. This is the **local** testcontainer path, which
that fix never covered, and this milestone's fixtures made the run long enough to expose it.
Recorded as WINDOWS entry 14.

---

## Validation Audit 2026-09-20

| Metric | Count |
|--------|-------|
| Requirements audited | 1 |
| COVERED | 1 |
| PARTIAL | 0 |
| MISSING | 0 |
| Gaps filled this audit | 0 |
| Escalated to manual-only | 0 |

**`TO-RESOLVE` resolution.** The seed invented no names. All seven tests were resolved against
`go test -list` and their match counts verified (`internal/server` → 5, `cmd/engram` → 2) before
being written into this map.
