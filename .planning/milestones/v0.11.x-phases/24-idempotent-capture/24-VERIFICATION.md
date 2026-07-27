---
phase: 24-idempotent-capture
verified: 2026-07-18T20:05:00Z
status: passed
score: 12/12 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 24: Idempotent Capture Verification Report

**Phase Goal:** `store_memory` is safely re-runnable — a repeat call with the same idempotency key
and owner returns the original record unchanged, a repeat with the same key but different content
is explicitly rejected, and concurrent retries never produce duplicate Qdrant records.
**Verified:** 2026-07-18T20:05:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria + PLAN must_haves)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| SC1 | Keyed replay with identical content returns the ORIGINAL record with zero side-effects | ✓ VERIFIED | `checkIdempotentReplay` (tools.go:671) runs and returns BEFORE `d.em.Embed` (tools.go:691-697, 751-761). `TestStoreMemoryIdempotentReplayReturnsOriginal` run live against real Qdrant (testcontainers, `-race`): embed-call spy stays at 1 after the replay, ids/short_ids identical, `List` total==1. **PASS** (0.01s). |
| SC2 | Same key + different content → `store.ErrIdempotencyConflict`, rejected before embed | ✓ VERIFIED | `checkIdempotentReplay` returns the wrapped sentinel before any embed call on mismatch (tools.go:686). `TestStoreMemoryIdempotentReplayRejectsMismatch` run live: `errors.Is(err, store.ErrIdempotencyConflict)` true, original record's content unmutated. **PASS**. `connecterror.go:60` maps it to `connect.CodeAlreadyExists`, distinct from the `ErrNotFound` row above it. |
| SC3 | Owner-scoped — owner baked into injective hash tuple `(owner, scope, key)` | ✓ VERIFIED | `idempotencyPointID` (idempotency.go:33-37) uses length-prefixed `fmt.Sprintf("%d:%s:%d:%s:%d:%s", ...)` over owner/scope/key, mirroring `namespacedOwner`. `TestStoreMemoryIdempotentKeyScopedPerOwner` (two-owner matrix, same key+content) run live: distinct ids, each record correctly attributed, no cross-owner List leakage. **PASS**. Pure boundary-shift injectivity additionally pinned by `TestIdempotencyPointIDBoundaryShiftInjective` (Plan 01, unit-tier). |
| SC4 | Concurrent identical → exactly one Qdrant point via deterministic-ID Upsert (no search-then-insert) | ✓ VERIFIED | No `Search`/`Scroll` in `checkIdempotentReplay` — only `d.st.Get(ctx, pointID)` (tools.go:676), a point Get. `TestStoreMemoryIdempotentConcurrentIdenticalOnePoint` (n=20 goroutines, same key+content) run under `go test -race` against real Qdrant: all returned ids equal, `List` total==1, no race detected. **PASS** (0.02s). D-12 honestly scoped: test asserts only the no-duplicate invariant, not reject-under-simultaneous-mismatch (confirmed by reading the test — no such assertion present). |
| SC5 | Omit key → fresh `uuid.NewString()` every time (keyless behavior unchanged) | ✓ VERIFIED | `checkIdempotentReplay` no-ops (`a.IdempotencyKey == ""` → immediate return, tools.go:672-674); `toMemory` still calls `uuid.NewString()` unconditionally (tools.go:636). `TestStoreMemoryNoKeyAlwaysFresh` run live: two keyless calls with identical content produce two distinct ids. **PASS**. |
| Schedule-window exclusion (D-13/D-07) | `idempotency_key` on shared `storeArgs`; schedule window excluded from fingerprint, consciously tested | ✓ VERIFIED | `IdempotencyKey` declared once on `storeArgs` (tools.go:438); `scheduleArgs` embeds `storeArgs` (tools.go:445-449) so it is promoted, not re-declared. `contentFingerprint` never reads `NotBefore`/`NotAfter` (idempotency.go:50-63 — only client-authored identity fields). `TestScheduleMemoryIdempotentIgnoresWindowChange` run live: replay with a changed `not_after` returns the original record with its original window intact, with an explicit doc-comment framing this as the conscious Open-Question-1 resolution. **PASS**. |

**Score:** 6/6 SC-level truths verified (0 present-but-behavior-unverified — every state-transition/concurrency claim was exercised by a live, passing test against a real Qdrant, `-race` included, not just presence/wiring).

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/store/store.go` | `IdempotencyFingerprint` field + `idempotencyFingerprintKey` const + `ErrIdempotencyConflict` sentinel + codec | ✓ VERIFIED | Field at :224 (`json:"-"`), const at :236, sentinel at :90 (distinct `errors.New`, not aliased to `ErrNotFound` at :64), unconditional write in `payload()` at :436, defensive read in `fromPayload()` at :529-530. |
| `internal/server/idempotency.go` | `engramIdempotencyNS` + `idempotencyPointID` + `contentFingerprint` | ✓ VERIFIED | All three present, pure, no Qdrant/DB calls. `engramIdempotencyNS` fixed at `69fbe3e4-a53b-4d6e-971a-cad2f107e23c` with a "never change" doc comment. |
| `internal/server/connecterror.go` | `ErrIdempotencyConflict` → Connect code row | ✓ VERIFIED | :60, maps to `connect.CodeAlreadyExists`, comment notes pre-positioning (Connect lane unreachable this phase). |
| `internal/server/tools.go` | `storeArgs.IdempotencyKey` + `checkIdempotentReplay` + keyed branch in both handlers + tool descriptions | ✓ VERIFIED | Field :438, helper :671-687, wired in `storeMemory` :691 and `scheduleMemory` :751 (both before `Embed`), tool descriptions at :1204 and :1214 state the match/reject/omit contract. |
| `internal/server/tools_test.go` / `idempotency_test.go` / `store_test.go` | SC1-SC5 + schedule-window + pure-function tests | ✓ VERIFIED | All named tests present and passing (see Behavioral Spot-Checks). |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `checkIdempotentReplay` | `d.st.Get` | point Get at deterministic ID | ✓ WIRED | tools.go:676 — single `Get`, no `Search`/`Scroll` anywhere in the helper or its callers. |
| `storeMemory`/`scheduleMemory` | `checkIdempotentReplay` | called before `d.em.Embed` | ✓ WIRED | tools.go:691 (before :704 Embed) and :751 (before :770 Embed). Match arm (`replay==true`) returns immediately, skipping Embed and `persistAndEnqueue` entirely. |
| `checkIdempotentReplay` resolved `pointID` | `m.ID` on create path | threaded, not recomputed | ✓ WIRED | tools.go:700-703 and :766-769 — `m.ID = pointID` uses the value returned from the helper; `idempotencyPointID` is not called a second time in either handler. |
| `storeArgs.IdempotencyKey` | `scheduleArgs` | Go field embedding (D-13) | ✓ WIRED | tools.go:445-449 — `scheduleArgs` embeds `storeArgs`; no separate field declaration, no proto/Connect field. |
| Connect `storeMemoryRequestToArgs` | `idempotency_key` | deliberately absent | ✓ CONFIRMED ABSENT | protoconv.go:90-103 — no `IdempotencyKey` assignment; Connect lane structurally excluded as required by D-13. |

### Behavioral Spot-Checks (live execution, not presence-only)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| SC1-SC5 + schedule-window test | `ENGRAM_REQUIRE_QDRANT=true go test -race -run 'TestStoreMemoryNoKeyAlwaysFresh\|TestStoreMemoryIdempotentReplayReturnsOriginal\|TestStoreMemoryIdempotentReplayRejectsMismatch\|TestStoreMemoryIdempotentKeyScopedPerOwner\|TestStoreMemoryIdempotentConcurrentIdenticalOnePoint\|TestScheduleMemoryIdempotentIgnoresWindowChange' ./internal/server/...` | All 6 PASS against real testcontainers Qdrant v1.18.2, no race detected | ✓ PASS |
| Payload round-trip (Plan 01) | `go test -run 'TestPayloadRoundTripsIdempotencyFingerprint\|TestPayloadRoundTripsEmbedderIdentity' ./internal/store/...` | PASS | ✓ PASS |
| Pure derivation/fingerprint unit tests (Plan 01) | `go test -run 'TestIdempotencyPointID\|TestContentFingerprint' ./internal/server/...` | PASS (all 7 sub-tests) | ✓ PASS |
| Full package regression | `go test -count=1 ./internal/server/... ./internal/store/...` | ok (5.5s / 4.9s) | ✓ PASS |
| Build/vet/format | `go build ./...`, `go vet ./internal/...`, `gofmt -l` (touched files), `golangci-lint run ./internal/server/... ./internal/store/...` | clean | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| REQ-idempotent-capture | 24-01, 24-02 | Optional idempotency key, strict replay-safety, owner-scoped, race-safe | ✓ SATISFIED | REQUIREMENTS.md:62-66 checked `[x]`; traceability table (line 135) marks Phase 24 "Complete"; all 5 SC + schedule-window decision live-tested and passing. No orphaned requirements found for Phase 24. |

### Prohibition Checks (must_haves.prohibitions, both plans)

| Prohibition | Status | Evidence |
|-------------|--------|----------|
| No new Qdrant payload index for the fingerprint | ✓ HOLDS | `ensureIndexes` (store.go:377-406) lists only `owner`, `scope`, `created_at`, `short_id` (pre-existing) — no `idempotency_fingerprint` index. |
| No second collection/ledger | ✓ HOLDS | No new `Store` method, no new collection name, no external dependency added (`git diff` vs. pre-phase base shows zero go.mod/go.sum change). |
| No IdempotencyFingerprint JSON wire leak | ✓ HOLDS | `json:"-"` tag on the field (store.go:224). |
| No folding conflict into ErrNotFound | ✓ HOLDS | Distinct `errors.New` sentinel (store.go:90), independently allocated from `ErrNotFound` (store.go:64) — `errors.Is` between two independent sentinel errors is false by Go semantics; confirmed via source read (no aliasing/wrapping between the two). |
| Keyless behavior unchanged (SC5) | ✓ HOLDS | `TestStoreMemoryNoKeyAlwaysFresh` passes live. |
| No silent overwrite on mismatch | ✓ HOLDS | `TestStoreMemoryIdempotentReplayRejectsMismatch` passes live; original content unmutated. |
| No search-then-insert / TOCTOU | ✓ HOLDS | Only `d.st.Get` used; no `Search`/`Scroll` call in the keyed path. |
| No Connect-lane widening | ✓ HOLDS | `storeMemoryRequestToArgs` (protoconv.go:90-103) has no `idempotency_key`/`IdempotencyKey` field. |
| No embed/MintShortID/tryEnqueue on match | ✓ HOLDS | Match arm returns immediately (tools.go:695-697, 755-761), before `Embed` and `persistAndEnqueue`; confirmed by embed-call-spy staying at 1 in `TestStoreMemoryIdempotentReplayReturnsOriginal`. |
| No new dependency | ✓ HOLDS | `git diff ab847570 -- go.mod go.sum` empty; only `github.com/google/uuid` (vendored) + stdlib `crypto/sha256`/`encoding/hex`. |
| No new config knob | ✓ HOLDS | No `idempotency` string found under `internal/config/` or `charts/engram/`. |

### Anti-Patterns Found

None. `rg` scan for `TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER` across all phase-touched files returned zero hits. `gofmt -l` and `golangci-lint run` both clean on the touched files.

### Human Verification Required

None. Every truth in this phase is a deterministic, machine-verifiable property (id/short_id identity, sentinel-error identity, point counts, `-race` invariant), and every one was exercised by a live test run against a real Qdrant instance during this verification pass — not inferred from presence/wiring alone.

### Gaps Summary

No gaps. All 5 ROADMAP success criteria, the schedule-window decision (D-13/D-07), all PLAN-frontmatter must-have truths/artifacts/key-links, and all prohibitions from both 24-01-PLAN.md and 24-02-PLAN.md verified directly against the live codebase and passing test runs (including a fresh `-race` run against a real testcontainers Qdrant, not merely SUMMARY.md's claims). REQ-idempotent-capture is satisfied and correctly marked complete in REQUIREMENTS.md.

One minor bookkeeping note (not a gap): `.planning/ROADMAP.md` line 107 still shows the Phase 24 milestone-level checkbox as `[ ]` (unlike Phases 22/23, which show `[x]` with a completion date) even though both plan-level checkboxes at lines 414/418 are `[x]` and 2/2 plans are marked executed. This is administrative/ship-workflow bookkeeping, not a phase-goal gap.

---

*Verified: 2026-07-18T20:05:00Z*
*Verifier: Claude (gsd-verifier)*
