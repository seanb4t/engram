---
phase: 25-supersession-with-history
plan: 01
subsystem: database
tags: [go, qdrant, store, authz, supersession]

# Dependency graph
requires:
  - phase: 24-idempotent-capture
    provides: "SetPayload single-key-merge precedent (SetVisibility) and the payload-only-mutation lost-write hazard doc comments (UpdatePayload, persistAndEnqueue) that motivated D-01's SetPayload-not-re-Upsert refinement"
provides:
  - "Memory.Supersedes / Memory.SupersededBy optional *string link fields with a working payload()/fromPayload() codec"
  - "store.ErrAlreadySuperseded sentinel (single-hop guard)"
  - "Store.Supersede(ctx, newMem, vec, target, subj) — owner-gated, single-hop, vector-preserving back-stamp"
  - "recall-gate soft-hide (qdrant.NewIsEmpty(\"superseded_by\")) at both Search and List call sites; Store.Get stays ungated"
affects: [25-02-supersede-memory-tool, connecterror-sentinel-switch]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Store-layer link fields follow the NotBefore/NotAfter optional-pointer codec pattern (plain json tags, encode-if-non-nil, decode-defensively)"
    - "Payload-only back-stamp mutations use SetPayload (single-key merge), never a full Upsert, on an existing record — avoids the CR-01 lost-write hazard"
    - "Recall-gate soft-hide conditions are added as sibling qdrant.NewIsEmpty(key) entries at each independent call site (Search, List), never folded into activeWindowConditions"

key-files:
  created: []
  modified:
    - internal/store/store.go
    - internal/store/store_test.go

key-decisions:
  - "D-01 confirmed: target back-stamp uses SetPayload (single-key merge), not a full re-Upsert — avoids CR-01 lost-write hazard"
  - "Recall-gate condition added independently at both Search (store.go) and List (store.go) call sites, not folded into activeWindowConditions"
  - "Store.Get left deliberately ungated so superseded records stay fetchable by id"

patterns-established:
  - "Pattern: SetVisibility-shaped mutation (span+telemetry wrapper, getWritable gate, single-key SetPayload, bare fail-closed return) is the template for any future payload-only, vector-preserving, owner-gated mutation"

requirements-completed: [REQ-supersession-links]

coverage:
  - id: D1
    description: "Memory.Supersedes/SupersededBy optional *string fields with payload codec round-trip (nil stays nil, non-nil round-trips)"
    requirement: "REQ-supersession-links"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestSupersedeRecallGate"
        status: pass
    human_judgment: false
  - id: D2
    description: "Superseded records excluded from both search_memory (Store.Search) and list_memory (Store.List) recall, but still fetchable via get_memory (Store.Get)"
    requirement: "REQ-supersession-links"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestSupersedeRecallGate"
        status: pass
      - kind: unit
        ref: "internal/store/store_test.go#TestSupersedeForwardChain"
        status: pass
    human_judgment: false
  - id: D3
    description: "Store.Supersede back-stamps target.superseded_by via SetPayload (vector/content-preserving) and the new record carries supersedes == target id"
    requirement: "REQ-supersession-links"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestSupersedeStamp"
        status: pass
    human_judgment: false
  - id: D4
    description: "Non-owner caller cannot supersede a target they don't own (getWritable/ActionWrite gate, ErrNotFound on denial)"
    requirement: "REQ-supersession-links"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestSupersedeOwnerGate"
        status: pass
    human_judgment: false
  - id: D5
    description: "Superseding an already-superseded target is rejected with store.ErrAlreadySuperseded (single live head, no cycles)"
    requirement: "REQ-supersession-links"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestSupersedeAlreadySuperseded"
        status: pass
    human_judgment: false
  - id: D6
    description: "Target deleted between the ownership gate and the back-stamp fails closed (TOCTOU, D-02)"
    requirement: "REQ-supersession-links"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestSupersedeTOCTOU"
        status: pass
    human_judgment: false

# Metrics
duration: 4min
completed: 2026-07-19
status: complete
---

# Phase 25 Plan 01: Store-Layer Supersession Primitive Summary

**Store.Supersede owner-gates a target via getWritable/ActionWrite, back-stamps its `superseded_by` with a single-key SetPayload (never a re-Upsert), and the recall gate soft-hides superseded records from Search/List while `Store.Get` stays fetchable.**

## Performance

- **Duration:** 4 min (task-commit span; excludes context-loading)
- **Started:** 2026-07-19T17:35:13Z
- **Completed:** 2026-07-19T17:39:29Z
- **Tasks:** 2 (both `tdd="true"`, RED then GREEN each)
- **Files modified:** 2 (`internal/store/store.go`, `internal/store/store_test.go`)

## Accomplishments
- `Memory.Supersedes` / `Memory.SupersededBy` optional `*string` link fields, with a payload codec that encodes only when non-nil (string values, not epoch ints) and decodes defensively — mirrors the `NotBefore`/`NotAfter` pattern exactly.
- `store.ErrAlreadySuperseded` sentinel: superseding an already-superseded (non-head) target is rejected, keeping a single live head per chain and making cycles structurally impossible.
- Recall-gate soft-hide: `qdrant.NewIsEmpty("superseded_by")` added as a sibling condition at both independent `f.Must` assembly sites — `Search` and `List` — so a superseded record disappears from both `search_memory` and `list_memory`. `Store.Get` (`get_memory`) is deliberately untouched, so a superseded record stays fetchable by id with its content and vector intact.
- `Store.Supersede(ctx, newMem, vec, target, subj) error`: owner-gates the target via `getWritable(target, subj, authz.ActionWrite)` (never `GetReadable` — shared-read access must not grant write), rejects an already-superseded target, creates the new correcting record first (`Upsert`), then back-stamps the target's `superseded_by` via a single-key `SetPayload` — the exact `SetVisibility` shape, swapped `ActionShare`→`ActionWrite` and `visibility`→`superseded_by`. Fails closed on a mid-op target delete (TOCTOU), with the same accepted-orphan-forward-link non-atomicity `SetVisibility`'s own doc comment already accepts for this codebase.
- Six new tests, all green (including `-race`): `TestSupersedeRecallGate`, `TestSupersedeStamp`, `TestSupersedeOwnerGate`, `TestSupersedeAlreadySuperseded`, `TestSupersedeForwardChain`, `TestSupersedeTOCTOU`.

## Task Commits

Each task followed strict RED → GREEN TDD, each half committed separately:

1. **Task 1: Link fields, payload codec, recall-gate soft-hide, and ErrAlreadySuperseded sentinel**
   - `18deb213` (test) — `TestSupersedeRecallGate`, confirmed failing to compile (Memory had no link fields yet)
   - `1215721a` (feat) — Memory fields + codec + sentinel + recall-gate additions; `TestSupersedeRecallGate` green
2. **Task 2: Store.Supersede — owner-gated, single-hop, vector-preserving back-stamp**
   - `57c3a017` (test) — `TestSupersedeStamp`/`OwnerGate`/`AlreadySuperseded`/`ForwardChain`/`TOCTOU`, confirmed failing to compile (`Store.Supersede` undefined)
   - `3ce68362` (feat) — `Store.Supersede` implementation; all five tests green, including `-race`

**Plan metadata:** (this commit)

## Files Created/Modified
- `internal/store/store.go` — `Memory.Supersedes`/`SupersededBy` fields (adjacent to `NotBefore`/`NotAfter`), `payload()`/`fromPayload()` codec entries, `ErrAlreadySuperseded` sentinel (alongside `ErrIdempotencyConflict`), `Store.Supersede` method (alongside `SetVisibility`), recall-gate `NewIsEmpty("superseded_by")` additions at `Search` and `List`
- `internal/store/store_test.go` — six new `TestSupersede*` cases

## Decisions Made
- D-01 (SetPayload, not re-Upsert) implemented exactly as CONTEXT.md locked it — verified against the codebase's own CR-01 lost-write hazard doc comments (`UpdatePayload`, `persistAndEnqueue`). No deviation from the plan's specified mechanism.
- No new Cedar policy needed — `own_records.cedar`'s "any action to the owner" grant already covers `authz.ActionWrite` on the target.
- Field placement: `Supersedes`/`SupersededBy` placed directly adjacent to `NotBefore`/`NotAfter` in the `Memory` struct (readability grouping per RESEARCH A3, no functional impact).

## Deviations from Plan

None — plan executed exactly as written. All `<read_first>` file:line citations in the plan matched the current codebase state exactly (no drift), so no adaptation was needed. All acceptance criteria and success criteria (SC1–SC4, D-02, D-06) verified as specified.

## Issues Encountered

- Pre-existing, unrelated `task lint:yaml` failure on `Taskfile.yaml` itself (yamlfmt lint complaint), confirmed via `git stash` to exist independent of this plan's changes — out of scope per the deviation rules' scope boundary (only auto-fix issues directly caused by the current task's changes). `task lint:go` (golangci-lint, the Go-scoped lint this plan's changes are actually subject to) is clean; `gofmt -l` reports no unformatted files; `task license:check` is clean.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- The store-layer supersession primitive (`Store.Supersede`, link fields, recall-gate soft-hide) is complete and fully tested — ready for Plan 02 to wire the `supersede_memory` MCP tool handler on top of it (per RESEARCH.md's `supersedeMemory` handler sketch and the `connecterror.go` sentinel-switch exhaustiveness addition for `ErrAlreadySuperseded`).
- No blockers. `go vet ./...` clean across the whole repo (no downstream breakage from the two new `Memory` fields).

---
*Phase: 25-supersession-with-history*
*Completed: 2026-07-19*

## Self-Check: PASSED

All claimed files (`internal/store/store.go`, `internal/store/store_test.go`, this SUMMARY.md) and all four task commit hashes (`18deb213`, `1215721a`, `57c3a017`, `3ce68362`) verified present.
