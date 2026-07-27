---
phase: 25-supersession-with-history
verified: 2026-07-19T17:54:08Z
status: passed
score: 9/9 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 25: Supersession with History Verification Report

**Phase Goal:** A memory can supersede another via additive `supersedes`/`superseded_by` links —
correction is explicit and preserves history, never deleting or silently overwriting. Superseded
records are soft-hidden from recall (search_memory/list_memory) but remain fetchable by id
(get_memory).
**Verified:** 2026-07-19T17:54:08Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| SC1 | Storing a memory with a `supersedes` link stamps the target's `superseded_by` back via a single-key `SetPayload`, WITHOUT touching content/vector | ✓ VERIFIED | `internal/store/store.go:1780-1818` `Store.Supersede`: step 4 uses `s.client.SetPayload(...Payload: qdrant.NewValueMap(map[string]any{"superseded_by": newMem.ID})...)` — a single-key payload merge, never a full re-`Upsert`. `Upsert(ctx, newMem, vec)` (step 3, new record) happens strictly before the back-stamp (step 4). `TestSupersedeStamp` (`internal/store/store_test.go`) asserts target content/tags/visibility survive — PASS (real Qdrant, `go test ./internal/store/... -run TestSupersedeStamp` green). |
| SC1b | New correcting record carries `supersedes = target id` | ✓ VERIFIED | `store.go:1264` handler sets `m.Supersedes = &targetID` before calling `Supersede`; `TestSupersedeMemory` asserts `newRec.Supersedes == targetID` — PASS. |
| SC2 | A superseded record is excluded from BOTH `search_memory` and `list_memory` but still returned by `get_memory` | ✓ VERIFIED | `rg -c 'NewIsEmpty("superseded_by")' internal/store/store.go` = 2 (Search call site store.go:826, List call site store.go:1049), each a sibling condition to `activeWindowConditions`, not folded into it. `Store.Get` (store.go:1345-1372) is unmodified — no filter applied, id-addressed. `TestSupersedeRecallGate` and `TestSupersedeMemory` (handler-level: target absent from `list_memory`/`search_memory`, present via `get_memory` with unchanged content) — both PASS on real Qdrant. |
| SC3 | Supersede routes through the ownership WRITE gate; a read/shared-only caller cannot supersede another owner's record; 404-indistinguishable | ✓ VERIFIED | `Store.Supersede` step 1: `s.getWritable(ctx, target, subj, authz.ActionWrite)` — never `GetReadable`/`ActionShare`. `getWritable` (store.go:1465-1478) returns the identical `ErrNotFound` for both "not owner" and "doesn't exist" (fail-closed, no existence leak). Handler (`tools.go:1272-1281`) re-wraps `store.ErrNotFound` with the caller's ORIGINAL `a.Supersedes` input, never the resolved UUID. `TestSupersedeOwnerGate` (store-level) and `TestSupersedeMemory`'s cross-owner subtest (handler-level, asserts error echoes `targetSID` and does NOT contain the resolved `targetID`) — both PASS. |
| SC4 | No auto/similarity supersede path; an already-superseded target (and self/cycle) is rejected at write time via `store.ErrAlreadySuperseded` → `connect.CodeFailedPrecondition` | ✓ VERIFIED | `Store.Supersede` step 2 (store.go:1799-1803) rejects a non-nil/non-empty `targetRec.SupersededBy` with `ErrAlreadySuperseded` before any mutation — a single live head structurally prevents cycles. `Supersede` is only ever invoked from the explicit `supersedeMemory` handler (no similarity/write-through call site found via `rg -n 'Supersede(' internal/`). `connecterror.go:70-76` maps `store.ErrAlreadySuperseded` → `connect.CodeFailedPrecondition`, enumerated in the doc comment (:38-41). `TestSupersedeAlreadySuperseded` (store) and `TestConnectError/already_superseded` (server) — both PASS. |

**Score:** 9/9 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/store/store.go` — `Memory.Supersedes`/`SupersededBy *string` fields | Optional pointer link fields, plain (wire-visible) json tags | ✓ VERIFIED | Lines 179/184: `Supersedes *string \`json:"supersedes,omitempty"\`` / `SupersededBy *string \`json:"superseded_by,omitempty"\`` — plain tags (not `json:"-"`), matching the `NotBefore`/`NotAfter` convention. |
| `internal/store/store.go` — `payload()`/`fromPayload()` codec | Encode only when non-nil (string values); decode defensively | ✓ VERIFIED | Lines 461-465 (encode), 558/562 (decode) — string values, not epoch ints; comma-ok idiom matching `NotBefore`/`NotAfter`. |
| `internal/store/store.go` — `store.ErrAlreadySuperseded` sentinel | Named sentinel error | ✓ VERIFIED | Line 97: `var ErrAlreadySuperseded = errors.New("target is already superseded")`, doc comment at :92-96 naming D-05/D-06. |
| `internal/store/store.go` — `Store.Supersede` method | Owner-gated, single-hop, vector-preserving back-stamp | ✓ VERIFIED | Lines 1780-1818, full implementation reviewed — matches D-01/D-02/D-04/D-05/D-06 exactly. |
| `internal/store/store_test.go` — six `TestSupersede*` cases | RecallGate/Stamp/OwnerGate/AlreadySuperseded/ForwardChain/TOCTOU | ✓ VERIFIED | All six present and green: `go test ./internal/store/... -run TestSupersede -v -count=1` — 6/6 PASS (real Qdrant via testcontainers). |
| `internal/server/tools.go` — `supersedeArgs` struct | Embeds `storeArgs`, adds `Supersedes` field | ✓ VERIFIED | Lines 492-495: `type supersedeArgs struct { storeArgs; Supersedes string ... }`. |
| `internal/server/tools.go` — `deps.supersedeMemory` handler | Resolve target → embed → mint short id → `Store.Supersede` → 404-rewrap → enqueue | ✓ VERIFIED | Lines 1256-1285, matches the plan's 7-step sequence exactly. |
| `internal/server/tools.go` — `supersede_memory` MCP registration | Registered tool, id/short_id result shape | ✓ VERIFIED | Lines 1486-1494. |
| `internal/server/connecterror.go` — `ErrAlreadySuperseded` case | Maps to `CodeFailedPrecondition`, enumerated in doc comment | ✓ VERIFIED | Lines 38-41 (doc), 70-76 (switch case). |
| `internal/server/tools_test.go` — `TestSupersedeMemory` | Handler stores + back-stamps + owner-gate re-wrap | ✓ VERIFIED | Lines 1865-1957, exercises SC1-SC4 end-to-end; PASS. |
| `internal/server/connecterror_test.go` — `already_superseded` row | Table-driven case | ✓ VERIFIED | `TestConnectError/already_superseded` present and PASS. |
| `internal/server/store_iface.go` + `fakestore_test.go` — `memStore.Supersede` + `spyStore.Supersede` | Interface extension + scripted fake (undocumented in plan, added as auto-fix) | ✓ VERIFIED | store_iface.go:40, fakestore_test.go:209-230 — required for `go vet`'s compile-time interface assertion; correctly scripted to mirror real `Store.Supersede` semantics. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `Store.Supersede` | `getWritable(target, subj, ActionWrite)` | Cedar PDP (`decideRecord`) | ✓ WIRED | store.go:1795 — `authz.ActionWrite`, never `ActionShare`/`GetReadable`. |
| `Store.Supersede` | `Upsert` then `SetPayload` | create-first ordering | ✓ WIRED | store.go:1806 (`Upsert`) precedes :1812 (`SetPayload`) — confirmed by reading, not just grep. |
| Search + List filter assembly | `qdrant.NewIsEmpty("superseded_by")` | sibling condition to `activeWindowConditions` | ✓ WIRED | store.go:826 (Search), :1049 (List) — both present, `activeWindowConditions` body unmodified. |
| `get_memory` | `Store.Get` | deliberately unfiltered | ✓ WIRED (by omission) | store.go:1345-1372 — no `superseded_by` filter present. |
| `supersede_memory` registration | `deps.supersedeMemory` | `callerFromContext` → handler | ✓ WIRED | tools.go:1487-1494. |
| `deps.supersedeMemory` | `d.st.Supersede(...)` | `memStore` interface | ✓ WIRED | tools.go:1272, store_iface.go:40. |
| `connectError` | `store.ErrAlreadySuperseded` | `connect.CodeFailedPrecondition` | ✓ WIRED | connecterror.go:70-76. |

### Behavioral Spot-Checks / Test Execution

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Store-layer Supersede tests (real Qdrant) | `go test ./internal/store/... -run TestSupersede -v -count=1` | 6/6 PASS: RecallGate, Stamp, OwnerGate, AlreadySuperseded, ForwardChain, TOCTOU | ✓ PASS |
| Server-layer handler + connectError tests (real Qdrant) | `go test ./internal/server/... -run 'TestSupersedeMemory\|TestConnectError' -v -count=1` | TestSupersedeMemory PASS; TestConnectError (14 subtests incl. `already_superseded`) PASS | ✓ PASS |
| Whole-repo build/vet | `go build ./...` / `go vet ./...` | clean, no errors (rules out stale-LSP false "undefined symbol" concerns) | ✓ PASS |
| Full test suite | `task test` (go test ./... + python hook tests) | all packages `ok`, 33/33 python passed | ✓ PASS |
| Lint | `task lint:go` | `0 issues.` | ✓ PASS |
| License headers | `task license:check` | `986 files, valid: 207, invalid: 0` | ✓ PASS |
| Commit provenance | `git cat-file -e <hash>` for all 8 commits cited in both SUMMARYs | all 8 present in history | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| REQ-supersession-links | 25-01, 25-02 | Supersede links, soft-hide recall gate, write-gate authz, explicit-only correction | ✓ SATISFIED | REQUIREMENTS.md line 68-72 marked `[x]`; ROADMAP maps Phase 25 → Complete; store + server implementation and tests confirmed above. No orphaned requirements found for Phase 25. |

### Anti-Patterns Found

None. `rg -in 'TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER'` on all files modified by this phase
(`internal/store/store.go`, `internal/server/tools.go`, `internal/server/connecterror.go`,
`internal/server/store_iface.go`, `internal/server/fakestore_test.go`) returned no matches.
No stub returns, no hardcoded-empty response patterns, no `GetReadable`/`ActionShare` substituted
for the write gate anywhere in the supersede path.

### Threat Model Mitigation Check (25-01 + 25-02, all `high`-severity rows)

| Threat ID | Severity | Disposition | Verified |
|-----------|----------|-------------|----------|
| T-25-01 (EoP, target write gate) | high | mitigate | ✓ `getWritable(..., ActionWrite)` confirmed; `TestSupersedeOwnerGate` PASS |
| T-25-02 (Tampering, single-hop/cycle) | high | mitigate | ✓ `ErrAlreadySuperseded` guard before any mutation; `TestSupersedeAlreadySuperseded` PASS |
| T-25-04 (EoP, cross-tenant) | high | mitigate | ✓ Reuses existing Cedar PDP unchanged (no new policy needed, confirmed no `own_records.cedar` edits) |
| T-25-07 (EoP, handler-level) | high | mitigate | ✓ Handler never calls `GetReadable`; delegates entirely to `Store.Supersede`'s gate; cross-owner subtest PASS |
| T-25-08 (Info disclosure, 404) | medium | mitigate | ✓ Re-wrap with `a.Supersedes` (original), not `targetID`; asserted directly in `TestSupersedeMemory` |
| T-25-09 (Info disclosure, connectError fallthrough) | low | mitigate | ✓ Precise `CodeFailedPrecondition`, not generic `CodeInternal` |
| T-25-05 / T-25-10 (accepted, low/medium) | low | accept | ✓ Documented in doc comments exactly as the threat register specifies (TOCTOU fail-closed via `TestSupersedeTOCTOU`; non-idempotent two-step accepted as bounded/reversible) |
| T-25-06 (accepted, repudiation) | low | accept | ✓ Owner-only, additive-only, `Get` stays fetchable — consistent with implementation |
| T-25-SC (both plans, supply chain) | n/a | accept | ✓ `git diff --stat` on go.mod/go.sum for the phase's commits shows no new dependencies |

### Human Verification Required

None. All must-haves resolved programmatically with passing tests against real Qdrant
(testcontainers), clean `go build`/`go vet`/lint/license checks, and direct source reading of
every claimed code path (not just SUMMARY assertions).

### Gaps Summary

No gaps found. Both plans' locked decisions (D-01 through D-09) and all four ROADMAP success
criteria are implemented exactly as specified, with real (not just claimed) passing test coverage
on real Qdrant infrastructure. The one deviation from the PLAN's stated `<files>` scope (25-02
Task 1 also touched `store_iface.go`/`fakestore_test.go`) is a required, same-shape, non-functional
compile-time necessity, correctly documented as an auto-fix in the SUMMARY and verified to be
correctly scripted.

---

*Verified: 2026-07-19T17:54:08Z*
*Verifier: Claude (gsd-verifier)*
