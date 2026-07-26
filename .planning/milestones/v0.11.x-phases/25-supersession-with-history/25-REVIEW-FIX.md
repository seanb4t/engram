---
phase: 25-supersession-with-history
fixed_at: 2026-07-19T20:10:00Z
review_path: .planning/phases/25-supersession-with-history/25-REVIEW.md
iteration: 2
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 25: Code Review Fix Report

**Fixed at:** 2026-07-19T20:10:00Z
**Source review:** .planning/phases/25-supersession-with-history/25-REVIEW.md
**Iteration:** 2

**Summary (iteration 2):**
- Findings in scope: 4 (1 critical/blocker, 1 warning, 2 info)
- Fixed: 4
- Skipped: 0

This iteration re-reviews the fixes applied in iteration 1 (below, unchanged)
and addresses the 4 new findings surfaced by that re-review: CR-04
(BLOCKER), WR-04 (WARNING), and IN-03/IN-04 (INFO).

## Fixed Issues (iteration 2)

### CR-04: `Store.Update`'s whole-payload write silently reverts a concurrent `Supersede` back-stamp

**Files modified:** `internal/store/store.go`, `internal/store/store_test.go`
**Commit:** `d84378fb`
**Applied fix:** `Store.Update` re-Upserted a `cur` snapshot wholesale; if that snapshot was fetched (via
`FetchForUpdate`) before a concurrent `Supersede` landed its `superseded_by` back-stamp, `Update`'s Upsert
silently erased it — reproduced against real Qdrant during the re-review. Fixed both aspects the finding called
out:
1. **Serialize:** `Store.Update` now acquires the SAME per-target lock `Store.Supersede` uses
   (`s.locker.Lock(ctx, cur.ID)`), released via `defer unlock()`, so `Update` and `Supersede` on the same target
   id can never interleave.
2. **Preserve:** inside the lock, `Update` re-`Get`s the record and copies its current `Supersedes`/`SupersededBy`
   into `cur` before the Upsert, so even a just-landed back-stamp survives regardless of which of the two racing
   calls wins the lock first. A concurrent `Delete` (the `Get` returning `store.ErrNotFound`) is left as
   pre-existing, out-of-scope behavior — `Update` never existence-checked `cur` before this fix either, so this
   doesn't regress or newly introduce that gap.
3. **Sibling scan:** searched every `.Upsert(ctx, ...)` and `SetPayload`/`OverwritePayload` call against an
   EXISTING record in `internal/store/`. `UpdatePayload`, `SetVisibility`, `IncrementAccess`,
   `BackfillShortIDs`, `RemapOwner`, and `Supersede`'s own back-stamp all already use a targeted, single/few-key
   `SetPayload` (never a whole-payload `payload(m)`-built Upsert), so none of them can erase `superseded_by` the
   same way. `Reindex`'s per-point `s.client.Upsert` writes into a DIFFERENT target collection during an offline
   embedder migration, copying the source's raw payload map verbatim (only the `embedder_identity` key is
   touched) — a separate, already-documented migration-tool code path, not an in-place same-collection write
   racing `Supersede`, so it was left out of scope. `Store.Update` was the only in-scope offender.
4. **Test:** added `TestSupersedeVsUpdateConcurrent` (real Qdrant, `-race`), mirroring `TestSupersedeConcurrent`'s
   style: takes a `FetchForUpdate` snapshot before racing a `Supersede` and an `Update` on the same target from two
   goroutines, and asserts both calls succeed and `superseded_by` survives regardless of scheduling order (proven
   by the lock's serialization + the in-lock re-read, not by test-side timing tricks).

### WR-04: `IdempotencyKey json:"-"` shadow does not exclude the field from wire decode

**Files modified:** `internal/server/tools.go`, `internal/server/tools_test.go`
**Commit:** `74e60a1b`
**Applied fix:** Chose the doc+defensive-clear disposition from the finding's two options (restructuring
`supersedeArgs` to stop embedding `storeArgs` was rejected — it would reintroduce the exact hand-rolled parallel
field-list drift risk the embedding was originally added to avoid, per the type's own doc comment).
1. Corrected `supersedeArgs`' doc comment: it previously (incorrectly) claimed the `json:"-"` shadow field
   removes `idempotency_key` from BOTH the schema and the wire decode. It only removes it from the schema — a
   `json:"-"` field has no JSON name, so it never enters `encoding/json`'s same-name shadowing contest, leaving
   the promoted `storeArgs.IdempotencyKey` as the sole decode target. The comment now states this precisely and
   explains why the defensive clear (below) is what actually makes the field inert.
2. Added `a.storeArgs.IdempotencyKey = ""` at the top of `supersedeMemory`, before the field is ever read (it
   already wasn't read anywhere in the function — this is defense-in-depth against a future refactor).
3. Added `TestSupersedeArgsDecodePopulatesPromotedIdempotencyKey`: a direct `json.Unmarshal` into `supersedeArgs`
   asserting the promoted `a.storeArgs.IdempotencyKey` DOES decode (pins the corrected understanding; the prior
   claim was untested and wrong).
4. Added `TestSupersedeMemoryIgnoresIdempotencyKey`: an end-to-end handler test that supplies
   `IdempotencyKey` on a `supersedeArgs` call and asserts a normal supersede happens (no error, correct content),
   then a second call with the SAME key against the now-already-superseded target fails with a normal
   `ErrAlreadySuperseded`-flavored error rather than replaying the first call's id — proving the key is ignored,
   not honored as a replay key.

### IN-03: No direct unit tests for `TargetLocker`/`inProcessTargetLocker`

**Files modified:** `internal/store/locker_test.go` (new)
**Commit:** `92c0b96f`
**Applied fix:** Added a package-local `locker_test.go` with pure (non-Qdrant) unit tests:
`TestInProcessTargetLockerCanceledContextRejected` (an already-canceled `ctx` is rejected up front, without
acquiring the lock), `TestInProcessTargetLockerSameKeySerializes` (a channel-based ordering assertion that a
second `Lock` on the same key blocks until the first unlocks — no sleeps), `TestInProcessTargetLockerDifferentKeysDoNotBlock`
(a different key acquires immediately while an unrelated key's lock is held), and
`TestInProcessTargetLockerConcurrentDistinctKeys` (a `-race` smoke test with many goroutines locking many
distinct keys concurrently). Coverage was previously entirely indirect via `TestSupersedeConcurrent`
(same-key contention only, through the full `Store.Supersede` path).

### IN-04: Rule-immutability guard for supersede lives only in the MCP handler, not in `Store.Supersede`

**Files modified:** none (accepted, no code change)
**Applied fix:** No action taken, per the finding's own disposition ("No action required to ship"). The
re-reviewer confirmed this matches the existing, established convention in this codebase: `Store.Update`,
`Store.UpdatePayload`, and `Store.SetVisibility` likewise leave rule immutability entirely to their respective
handler-layer guards rather than enforcing it in the store, and `Store.Supersede`'s only production caller today
is `supersedeMemory`, which already gates it. Not a new gap introduced by this phase's work — tracked here per
the finding's own suggestion, to be revisited only if/when a second caller of `Store.Supersede` is added (e.g. a
future Connect RPC) without replicating the guard.

## Skipped Issues (iteration 2)

None — all 4 in-scope findings were addressed (3 fixed with code/test changes, 1 accepted with no change per its
own "no action required" disposition).

## Verification (iteration 2)

After each of the 3 code-changing commits: `go build ./...` and `go vet ./...` were confirmed clean, and the
relevant `go test ./internal/{store,server}/... -race` subset was run and passed before moving to the next
finding. Cumulative final state (iteration 1 + iteration 2, all commits applied):

- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./internal/store/... -race` — ok (23.7s), including the new `TestSupersedeVsUpdateConcurrent` and
  `locker_test.go` suite
- `go test ./internal/server/... -race` — ok (7.7s), including the new WR-04 idempotency tests
- Final `task lint:go` and `task license:check` are run against the main checkout by the orchestrator after this
  worktree's commits are merged in (not run from inside the isolated fixer worktree, per this agent's operating
  constraints).

No finding in this iteration required a "requires human verification" flag: CR-04's fix is directly pinned by
`TestSupersedeVsUpdateConcurrent` under `-race` (asserts the invariant holds regardless of which racing call wins
the lock, not just in one lucky ordering); WR-04's fix is pinned by both the decode-path test and the end-to-end
ignore test; IN-03 is a pure additive test file with no production-code change.

---

_Fixed: 2026-07-19T20:10:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_

---

# Iteration 1 (preserved below, unchanged)

_fixed_at: 2026-07-19T19:42:47Z — findings_in_scope: 8 — fixed: 8 — skipped: 0 — status: all_fixed_

## Fixed Issues

### CR-01: Concurrent Supersede calls on the same target race past the "already superseded" guard

**Files modified:** `internal/store/locker.go` (new), `internal/store/store.go`, `internal/store/store_test.go`
**Commit:** `0d7080dc`
**Applied fix:** Per the user-directed disposition (NOT the reviewer's suggested compare-and-swap), added a
`TargetLocker` interface in `internal/store/locker.go` with a default in-process implementation
(`inProcessTargetLocker`, a `sync.Map` of per-target `*sync.Mutex`), wired as `Store`'s default via `New()` and
overridable via the new `WithTargetLocker` option. `Store.Supersede` now acquires `s.locker.Lock(ctx, target)`
BEFORE the `getWritable`/already-superseded check and holds it through the back-stamping `SetPayload` (released
via `defer unlock()`), making the check-then-act atomic in-process — different targets never contend. Added a
doc comment on `Supersede` and on `TargetLocker` itself noting the interface exists so a distributed lock can
later be swapped in without touching `Supersede`'s logic, and that the in-process default only guarantees
single-instance atomicity. Added `TestSupersedeConcurrent`: two goroutines race to supersede the same target;
asserts exactly one succeeds and the other returns `ErrAlreadySuperseded`. Verified under `-race`.

### CR-02: `supersede_memory` has no rule-immutability guard

**Files modified:** `internal/server/tools.go`, `internal/server/tools_test.go`
**Commit:** `db3fbe9c`
**Applied fix:** `deps.supersedeMemory` now rejects a `rule`-category target with
`fmt.Errorf("%w — delete the rule instead of superseding it", errRuleImmutable)`, mirroring `updateMemory`
(tools.go:1116) and `setVisibility` (tools.go:1233). The category check runs on the record fetched by the new
`FetchForUpdate` call added for CR-03 (see below), so no extra store round trip was needed for this guard alone.
Added `TestSupersedeMemoryRejectsRule`: seeds a rule, attempts to supersede it, asserts `errRuleImmutable` is
returned, the rule is not back-stamped, and it remains present in `list_rules`.

### CR-03: `supersede_memory` embeds and mints a short id before the target ownership gate

**Files modified:** `internal/server/tools.go`, `internal/server/tools_test.go`
**Commit:** `db3fbe9c` (combined with CR-02 — both live in the same code edit: the new `FetchForUpdate` call
gates ownership AND supplies the category CR-02 needs, so splitting them into separate commits would have
required an artificial, less-clear diff)
**Applied fix:** `deps.supersedeMemory` now calls `d.st.FetchForUpdate(ctx, targetID, c.Subj)` (the existing
`getWritable`/`ActionWrite` gate, already exposed on the `memStore` interface via `FetchForUpdate`) immediately
after resolving the target id and BEFORE the billable `d.em.Embed` call and the Qdrant-hitting `d.st.MintShortID`
call — mirroring `updateMemory`'s and `storeDiscovery`'s cost-amplification hardening. A non-owner or
nonexistent target is now rejected before any spend. `Store.Supersede` still re-runs its own internal
`getWritable` gate (now under CR-01's lock) as the authoritative atomic check — the handler-level gate is a
cheap early-reject, not a replacement for the store-level one. Added
`TestSupersedeMemoryEmbedNotCalledForNonOwner` using the existing `countingEmbedder` test double, mirroring
`TestUpdateMemoryEmbedNotCalledForNonOwner`: asserts a non-owner's supersede attempt returns `ErrNotFound` with
zero embed calls.

### WR-01: `SearchDiscovery` does not apply the `superseded_by` soft-hide gate

**Files modified:** `internal/store/store.go`, `internal/store/store_test.go`
**Commit:** `1d5c2973`
**Applied fix:** Added `qdrant.NewIsEmpty("superseded_by")` to `SearchDiscovery`'s `must` conditions, matching
`Search`/`List`. Added `TestSearchDiscoverySupersededHidden`, analogous to `TestSupersedeRecallGate`, scoped to
`SearchDiscovery`: a superseded discovery is excluded, a live one remains present.

### WR-02: `ListScheduled` does not apply the `superseded_by` soft-hide gate

**Files modified:** `internal/store/store.go`, `internal/store/store_test.go`
**Commit:** `331d33c6`
**Applied fix:** Added `qdrant.NewIsEmpty("superseded_by")` to `ListScheduled`'s filter, alongside `scope`,
`ownerOnlyCondition`, and `scheduledStateCondition`. Added `TestListScheduledSupersededHidden`: a
scheduled-but-superseded record is excluded from `ListScheduled(ScheduledPending)`; a live scheduled record
remains present.

### WR-03: `supersede_memory` silently no-ops the inherited `idempotency_key` field

**Files modified:** `internal/server/tools.go`, `internal/server/tools_test.go`, `go.mod`, `go.sum`
**Commit:** `49b29750`
**Applied fix:** Per the user-directed disposition (removal, NOT wiring idempotency), `supersedeArgs` now
declares its own `IdempotencyKey string `json:"-"`` field, shadowing the promoted `storeArgs.IdempotencyKey` at
a shallower struct depth. Both `encoding/json`'s decode (dominant-field selection) and `jsonschema-go`'s
`reflect.VisibleFields`-based schema inference resolve same-name fields by shallowest-depth-wins, so this
excludes `idempotency_key` from `supersede_memory`'s wire decode AND its advertised JSON schema. Updated the
`supersedeArgs` doc comment to drop the `idempotency_key` claim and document the exclusion (plan T-25-10:
supersede's idempotency was deliberately deferred, not wired this phase). Added
`TestSupersedeMemorySchemaExcludesIdempotencyKey`, which calls `jsonschema.For[supersedeArgs](nil)` directly and
asserts `idempotency_key` is absent from `schema.Properties` while `content`/`scope`/`supersedes` remain present
(sanity that the exclusion is targeted, not a broken reflection). `github.com/google/jsonschema-go` was promoted
from an indirect to a direct `go.mod` dependency via `go mod tidy` (it was already a transitive dependency of
`modelcontextprotocol/go-sdk`; the test now imports it directly to inspect the reflected schema the same way
`mcp.AddTool` does internally).

**Iteration-2 note:** the re-review found this fix's schema half correct but its doc comment's WIRE-DECODE claim
false (WR-04, fixed above) — the `json:"-"` shadow only excludes the field from the advertised schema, not the
`encoding/json` decode.

### IN-01: No test verifies the target's stored vector survives Supersede

**Files modified:** `internal/store/store_test.go`
**Commit:** `37ef7e71`
**Applied fix:** Added `TestSupersedeVectorPreserved`, using a raw `s.client.Get(..., WithVectors:
qdrant.NewWithVectors(true))` before and after `Supersede` (mirroring `reindex_test.go`'s `scrollPoints`
helper's `GetVectors().GetVector().GetDense().GetData()` pattern — `Store.Get` omits vectors). Note: Qdrant
normalizes vectors on insert for Cosine-distance collections, so the raw stored vector is the normalized form of
the input, not byte-identical to it; the test asserts the raw vector is non-empty before Supersede and
byte-identical between the before/after raw reads, which is the actual claim in Supersede's doc comment
("vector-preserving").

### IN-02: `supersede_memory` handler tests do not exercise rule/discovery targets or the non-owner cost path

**Files modified:** `internal/server/tools_test.go`
**Commit:** `21817437` (the rule-target and non-owner-cost-path cases were already covered by the CR-02/CR-03
tests above — `TestSupersedeMemoryRejectsRule` and `TestSupersedeMemoryEmbedNotCalledForNonOwner` — so only the
discovery-target case remained un-duplicated)
**Applied fix:** Added `TestSupersedeMemoryDiscoveryTarget`: seeds a discovery via `storeDiscovery`, supersedes
it, and asserts the target is back-stamped (still fetchable via `get_memory`, ungated) and soft-hidden from
`search_discovery` — exercising WR-01's gate end to end through the full handler path, not just the store layer.

## Skipped Issues

None — all 8 findings were fixed.

## Verification

After every commit: `go build ./...`, `go vet ./...`, and the relevant `go test ./internal/{store,server}/... -race`
subset were confirmed clean before moving to the next finding. Final state (all 8 commits applied):

- `go build ./...` — clean
- `go vet ./...` — clean
- `task lint:go` — 0 issues
- `task license:check` — all files valid (989 checked, 0 invalid)
- `go test ./internal/store/... -race` — ok (7.6s)
- `go test ./internal/server/... -race` — ok (7.4s)
- `go test ./...` (full suite) — ok, all packages pass

No finding required a "requires human verification" flag: CR-01's concurrency fix is directly pinned by
`TestSupersedeConcurrent` under `-race`; CR-02/CR-03's ordering and rejection are directly pinned by their
respective tests; the remaining findings are mechanical filter/schema additions with direct test coverage.

---

_Fixed: 2026-07-19T19:42:47Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
