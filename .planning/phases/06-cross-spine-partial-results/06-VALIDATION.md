---
phase: "06"
slug: "cross-spine-partial-results"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-20"
---

# Phase 06 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

**Seeded without a RESEARCH.md** (research skipped — the milestone's own `ARCHITECTURE.md` already
maps all four call sites and the design). Every name in the "existing coverage" column was resolved
against `go test -list` on 2026-09-20 and **exists**. Names for tests this phase will write are
left as `TO-RESOLVE` rather than guessed: a `-run` pattern matching nothing exits 0 with
`no tests to run` and reports a permanent false green (`gfh6q1ack4`, `bsbsvn4hbc`; Phase 4 hit this
with two of five seeded names). `/gsd-validate-phase 6` resolves each one and rewrites this map.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib); real Qdrant via `internal/store/storetest` where a cross-spine recall is exercised end to end |
| **Config file** | none — `Taskfile.yaml` `test:*` tasks are the entry points |
| **Quick run command** | `go test -short ./internal/server/... ./cmd/engram/...` |
| **Full suite command** | `task` (lint + whole-repo test) |
| **Estimated runtime** | ~45 s quick · ~5 min full |

---

## Sampling Rate

- **After every task commit:** `go test -short ./internal/server/... ./cmd/engram/...`
- **After every plan:** `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... ./cmd/engram/... -count=1`
- **Phase gate:** full `task` green
- **Proto note:** any proto change requires `task proto:gen` and committing the regenerated `gen/go`, `gen/ts` and `ui/src/lib/gen` trees — CI checks the tree for drift.

---

## Per-Requirement Verification Map

REQ-cross-spine-partial is this phase's only requirement; the rows below decompose it by the behavior each proves.

| Behavior to prove | Existing coverage (verified to exist) | New test | Status |
|-------------------|----------------------------------------|----------|--------|
| Hits are RETURNED, not discarded, when `ListScopes` fails after a cross-spine recall produced them — on all four surfaces (MCP `list_memory`/`search_memory`, Connect `ListMemories`/`SearchMemories`) | `TestSearchedScopesReporting`, `TestListMemoryCrossSpineIsolation`, `TestSearchMemoryCrossSpineIsolation`, `TestSearchMemoriesConnectCrossSpine`, `TestCrossSpineResultScope` — all exist and pin today's cross-spine behavior | `TO-RESOLVE` — one per surface, with an injectable `ListScopes` failure | ⬜ pending |
| The three states stay distinguishable: non-cross-spine (no keys) · coverage known (`searched_scopes` + `scopes_truncated`) · coverage unknown (`scopes_unknown` true, `searched_scopes` ABSENT, never an empty list) | `TestConnectCrossSpineNotInferred`, `TestConnectCrossSpineScopeRequired` (exist) | `TO-RESOLVE` — a three-state table test | ⬜ pending |
| The RPC SUCCEEDS on the coverage-unknown path (D-01), and the `ListScopes` error is logged server-side rather than surfaced on the wire (D-02) | — | `TO-RESOLVE` — assert success + no backend text in the response | ⬜ pending |
| `renderCoverageFooter` prints a third form, `scopes_unknown: true`, with no count (D-05) | `TestClientListFooterUnchangedWithoutCrossSpine`, `TestClientSearchNoFooterWithoutCrossSpine` (exist — these pin the no-footer case and must stay green) | `TO-RESOLVE` — a renderer test for the third form | ⬜ pending |
| The proto change is additive and the generated trees are in sync | `buf breaking` (FILE mode) in the `buf` CI job; `git diff --exit-code` over `gen/` after `task proto:gen` | none — existing gates | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] An injectable `ListScopes` failure usable from `internal/server` tests (the pivot every behavior row depends on — if this proves impractical, say so rather than weakening the assertions)
- [ ] Per-surface partial-result tests (4)
- [ ] The three-state distinguishability test
- [ ] The `renderCoverageFooter` third-form test
- [ ] Framework install: none required

---

## Manual-Only Verifications

*None expected.* Every behavior is reachable from `internal/server` and `cmd/engram` tests. If the
`ListScopes` failure cannot be injected on some surface, record that here rather than dropping the
assertion.

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Every `TO-RESOLVE` replaced with a name resolved against `go test -list`
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] No watch-mode flags
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
