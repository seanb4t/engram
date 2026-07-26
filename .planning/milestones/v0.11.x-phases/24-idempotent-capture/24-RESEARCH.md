# Phase 24: Idempotent Capture - Research

**Researched:** 2026-07-18
**Domain:** Deterministic-ID upsert idempotency on a Go + Qdrant memory-capture write path
**Confidence:** HIGH

## Summary

This phase makes `store_memory` (and, via the shared `storeArgs`/`toMemory` seam, `schedule_memory`)
safely re-runnable by adding one optional field, `idempotency_key`, and swapping the point-ID
minting strategy for keyed calls from a random `uuid.NewString()` to a deterministic
`uuid.NewSHA1` derived from `(owner, scope, key)`. The entire mechanism was already fully
specified in `.planning/phases/24-idempotent-capture/24-CONTEXT.md` (D-01 through D-13) and
grounded in `.planning/research/{STACK,PITFALLS}.md`; this research's job was to verify every code
anchor CONTEXT.md cites against the live tree, resolve a few mechanical ambiguities CONTEXT.md
left open, and surface two facts CONTEXT.md doesn't state explicitly but the planner needs.

All code anchors CONTEXT.md cites were confirmed byte-for-byte against the current tree
(`tools.go`, `store.go`) — see Code Seam Verification. Two non-obvious findings surfaced during
verification:

1. **The Connect write lane already funnels through `storeArgs`.** `connectapi.go`'s `StoreMemory`
   RPC calls `storeMemoryRequestToArgs` (`protoconv.go:90`), which builds a `storeArgs{}` literal
   field-by-field from the proto request — and the `engramv1.StoreMemoryRequest` proto has no
   `idempotency_key` field. Because `storeMemoryRequestToArgs` constructs the struct positionally
   (not via struct-copy), the new `IdempotencyKey` field is **structurally unreachable** from
   Connect with zero extra guard code — D-13's "Connect write-lane idempotency is out of scope" is
   satisfied for free, not by an explicit exclusion the planner needs to write.
2. **`Store.Upsert` has no internal authz check at all** — it is a bare Qdrant point write
   (`store.go:546-568`). D-09's phrase "the existing store-layer write authz (Cedar PDP +
   `getWritable`) still guards the `Upsert` as defense-in-depth" slightly overstates the current
   mechanism: `getWritable`/`OwnedOrAbsent` are handler-layer gates called *before* `Upsert`, and
   plain `store_memory` today calls neither (a fresh create needs no ownership check — the owner is
   always stamped from the verified `Subject`, never client-supplied). For the keyed path, safety
   is structural for the same reason `storeDiscovery`'s new-ID branch is safe: the deterministic ID
   can only ever resolve to a point this caller's own `(owner, scope, key)` hash produced, so the
   raw `d.st.Get` is safe without any additional gate — no Cedar call site exists to "wire into" and
   none needs to be added. This doesn't change the design, only its description; see
   Code Seam Verification.

**Primary recommendation:** Add `IdempotencyKey string` to `storeArgs` (jsonschema per D-11); at the
top of `storeMemory`/`scheduleMemory`, before `toMemory`+`Embed`, branch on a non-empty key into a
shared check-before-embed helper that derives the deterministic point ID via
`uuid.NewSHA1(engramIdempotencyNS, injectiveEncode(owner, scope, key))`, does a raw `d.st.Get`, and
either returns the existing `(id, short_id)` unchanged (fingerprint match), rejects with a new
distinct sentinel (fingerprint mismatch), or falls through to today's embed→`persistAndEnqueue` tail
with the deterministic ID and a freshly computed fingerprint stamped on the record (absent). No new
Qdrant index, no new store method, no new dependency, no proto/Connect change.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-idempotent-capture | `store_memory` accepts an optional idempotency key with strict replay-safety: same key+owner+content → original record unchanged; same key+owner, different content → rejected with explicit mismatch error; owner-scoped; race-safe (no duplicate Qdrant records under concurrency) | Fully addressed by D-01–D-13 (locked in CONTEXT.md) + this research's code-seam verification, deterministic-ID/fingerprint mechanics (Architecture Patterns), and the SC1–SC5 test matrix (Validation Architecture) |

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01 (no content-hash fallback):** When `idempotency_key` is absent/empty, `toMemory` keeps
  minting `uuid.NewString()` — a fresh record every time, today's behavior exactly. The
  research-proposed keyless content-hash fallback (`sha256(owner+scope+category+content)`) is
  **rejected for this phase**: it would make a repeat keyless call idempotent, violating locked SC5.
- **D-02 (derivation):** When a key is supplied, the record's Qdrant point ID is
  `uuid.NewSHA1(engramIdempotencyNS, injectiveEncode(owner, scope, key))` (UUIDv5) instead of
  `uuid.NewString()`. Same `(owner, scope, key)` ⇒ same point ID ⇒ the second `store_memory` is a
  native Qdrant upsert-replace of the first (SC1); different owners ⇒ different ID ⇒ no collision
  (SC3); concurrent identical calls ⇒ same ID ⇒ one point (SC4).
- **D-03 (tuple = owner + scope + key):** The hash input is `(owner, scope, key)` — scope
  included, not STACK.md §(a)'s narrower `owner+key`.
- **D-04 (injective encoding):** The three components are combined with an injective encoding
  (NUL-delimited or length-prefixed, mirroring `internal/auth`'s `namespacedOwner`
  `len:claim:len:value` discipline), never bare string concatenation. A fixed, committed
  `engramIdempotencyNS` UUID constant is the `uuid.NewSHA1` namespace. Exact byte layout is
  planner's discretion provided it is injective.
- **D-05 (fingerprint is a get, not a search):** Detect same-key/different-content by a point get
  at the deterministic ID + a fingerprint compare — never a filtered `Search`/`Scroll` and never a
  payload index. No new Qdrant index, no ledger/second collection.
- **D-06 (payload-only fingerprint field):** Store a payload-only, unindexed content fingerprint on
  the record, reusing the exact shape of the shipped embedder-config-identity stamp
  (`embedderIdentityKey`, `json:"-"`, written by `payload()` / read by `fromPayload()`). Field name
  is planner's discretion.
- **D-07 (fingerprint over client-authored fields, computed at write time):** The fingerprint is
  `sha256` over the client-authored identity fields as submitted — content + category + tags +
  source + repo/workspace/worktree/base_dir + client-supplied summary (if any). Computed from the
  incoming args at write time and stored; on replay recomputed from the new incoming args and
  compared to the stored value. Deliberately decoupled from server-side mutations (async summary
  fill, access-count bumps, short-id minting never touch it). Canonical field ordering is planner's
  discretion; must be stable and injective.
- **D-08 (check-before-embed ordering, mirrors `storeDiscovery`):** For a keyed `store_memory`,
  resolve the deterministic ID and `Get` the existing record before embedding.
  - Absent → embed → `Upsert` at the deterministic ID → mint `short_id` → `tryEnqueue` (today's
    `persistAndEnqueue` tail, unchanged; only the ID is deterministic not random).
  - Present + fingerprint match → return the existing `(id, short_id)` with zero side-effects: no
    embed, no `Upsert`, no `MintShortID`, no summary enqueue.
  - Present + fingerprint mismatch → reject before embedding with D-09's sentinel.
- **D-09 (owner isolation is structural, not a second filter):** A `Get` at the deterministic ID can
  only ever return the caller's own record because owner is inside the hash. No separate owner
  filter / `OwnedOrAbsent` gate is needed on the happy path. The existing store-layer write authz
  (Cedar PDP + `getWritable`) still guards the `Upsert` as defense-in-depth. *(Research note: see
  Code Seam Verification for a mechanism-level correction — the outcome is unchanged, but there is
  no literal Cedar/`getWritable` call site "guarding" `Upsert` today to point to.)*
- **D-10 (distinct sentinel, surfaced verbatim):** A new distinct store sentinel error (e.g.
  `store.ErrIdempotencyConflict`; name is planner's discretion) is returned on
  same-key/different-content. Surfaced to the MCP caller as an explicit, self-describing message. It
  does NOT reuse the not-found path.
- **D-11 (tool description documents the contract):** The `store_memory` MCP tool description /
  `idempotency_key` field schema MUST state what happens on a same-key/different-content replay.
  The jsonschema for the new optional arg spells out "owner-scoped; omit for a fresh record every
  time."
- **D-12 (no-duplicate is absolute; strict-reject is best-effort under a simultaneous mismatch
  race):** The deterministic-ID `Upsert` guarantees no duplicate is ever created under arbitrary
  concurrency (SC4 holds unconditionally). The mismatch reject (SC2) is a read-compare: two truly
  simultaneous same-key/different-content calls can both see "absent" and both upsert the same
  point (last-writer-wins → still exactly one record), so that one exact race converges without the
  reject firing. This is acceptable and correct. The concurrency test asserts the no-duplicate
  invariant, not reject-under-simultaneous-race.
- **D-13 (shared `storeArgs`; both MCP write tools; Connect deferred):** The optional
  `idempotency_key` is added to the shared `storeArgs` so both `store_memory` and `schedule_memory`
  gain it via the shared `toMemory` → `persistAndEnqueue` path. Connect write-lane idempotency is
  out of scope. `store_discovery`/`store_rule` already have their own deterministic-ID/replace
  paths and are untouched.

### Claude's Discretion

- Exact Go signatures/seam: whether the keyed branch lives inline in `storeMemory`/`scheduleMemory`
  or in a new shared helper alongside `persistAndEnqueue`; whether `toMemory` takes the derived ID
  as a param or a dedicated `mintID(owner, scope, key)` helper is introduced.
- The fixed `engramIdempotencyNS` namespace UUID value and where the constant lives
  (`internal/store` vs `internal/server`).
- Exact injective byte encoding of `(owner, scope, key)` (NUL-delimited vs length-prefixed) and the
  canonical field ordering fed into the content fingerprint.
- The payload field name/key for the stored fingerprint and the sentinel error's name.
- Whether the raw key is also stored in the payload (NOT required; nothing reads it back).
  Recommend fingerprint-only to keep it minimal.
- Test-file organization and the precise shape of the concurrency test (SC4) and the
  same-key/two-owners matrix test (SC3).

### Deferred Ideas (OUT OF SCOPE)

- **Keyless content-hash dedup fallback** — rejected here because it violates locked SC5. If a
  future milestone wants opt-in keyless dedup, it must be a distinct, explicitly flagged mode, never
  the default.
- **Idempotency on the Connect write lane & other write RPCs** — MCP `store_memory`/`schedule_memory`
  first; Connect parity is a later milestone item.
- **Persisting/indexing the raw idempotency key for audit or "list my keys"** — not needed for the
  contract; would require the payload index this phase deliberately avoids.
- **A configurable idempotency-key TTL/expiry** — engram records are durable, not request-scoped; no
  expiry this phase.

</user_constraints>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `idempotency_key` MCP arg + jsonschema doc | API/Backend (`internal/server`, MCP tool surface) | — | New optional field on `storeArgs`, mirrors `Summary`'s jsonschema pattern |
| Deterministic point-ID derivation (owner, scope, key) | API/Backend (`internal/server` or `internal/store` helper) | Database/Storage (Qdrant point identity is what makes it work) | Pure function, no I/O; owner/scope are already resolved server-side by the time it's called |
| Content-fingerprint compute + compare | API/Backend (compute at write time, compare on replay) | Database/Storage (fingerprint is a payload field, persisted via `payload()`/`fromPayload()`) | Compute is pure Go; storage/round-trip reuses the existing `Memory` struct + payload codec |
| Check-before-embed replay short-circuit | API/Backend (`storeMemory`/`scheduleMemory` handlers) | — | Must happen before the `embed.Embed` call (external HTTP call to the embedder) to avoid wasted cost on every replay |
| Race-safety under concurrent identical writes | Database/Storage (Qdrant's atomic per-point `Upsert`) | API/Backend (deterministic ID is what routes concurrent calls to the same point) | Qdrant's point-identity semantics provide the atomicity; no application-level lock is added or needed |
| Distinct mismatch error surfaced to caller | API/Backend (new sentinel + MCP error text) | — | MCP `AddTool` handlers return `error` directly; the SDK renders it as the tool's error text, same as today's `errRuleImmutable`/`errStaleSummary` |
| Connect write-lane exclusion | API/Backend (structural: `protoconv.go`'s field-by-field mapping) | — | Already achieved with zero new code — see Summary finding 1 |

This is a single self-hosted Go binary (no browser/SSR/CDN tiers) — every capability in this phase
lives in `internal/server` and `internal/store`.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/google/uuid` | v1.6.0 (`go.mod`, already pinned) | `uuid.NewSHA1(space, data)` → deterministic UUIDv5 point ID; `uuid.MustParse` for the fixed namespace constant | Already vendored; `NewSHA1` signature confirmed via `go doc` against the local module: `func NewSHA1(space UUID, data []byte) UUID` [VERIFIED: local go.mod module cache via `go doc`] |
| `crypto/sha256` (stdlib) | Go 1.26 toolchain (pinned) | Content-fingerprint hash | Stdlib, zero dependency; already the recommended primitive in STACK.md §(a) |
| `encoding/hex` (stdlib) | Go 1.26 toolchain | Encode the fingerprint hash to a stable string for the payload field | Standard pairing with `sha256.Sum256`; no alternative needed |

### Supporting

None — this phase needs no new supporting library. `internal/store`'s existing `payload()`/
`fromPayload()` codec and `Store.Get`/`Store.Upsert` cover every I/O need.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Deterministic `uuid.NewSHA1` point ID | Client-supplied idempotency key stored as a searchable payload field + new Qdrant index + filtered search-then-upsert | Extra round-trip, new index violates DEC-ef28, TOCTOU race (Pitfall 3) — rejected in STACK.md's own Alternatives Considered table |
| `uuid.NewSHA1` (SHA1-based UUIDv5) | `uuid.NewMD5` (MD5-based UUIDv3) | Both are deterministic-hash UUID variants already exposed by the vendored `google/uuid`; SHA1/v5 is the modern preferred variant (MD5/v3 is legacy per RFC 4122) — no functional difference for this use case, but v5 is the conventional choice and is what CONTEXT.md's D-02 already locks |

**Installation:**

```bash
# No new dependency — google/uuid v1.6.0 is already in go.mod/go.sum,
# crypto/sha256 and encoding/hex are Go 1.26 stdlib.
```

**Version verification:** `go doc github.com/google/uuid.NewSHA1` against the local module cache
confirms the exact signature `func NewSHA1(space UUID, data []byte) UUID` is available in the
pinned v1.6.0. `go doc github.com/google/uuid.MustParse` confirms `func MustParse(s string) UUID`
("simplifies safe initialization of global variables holding compiled UUIDs" — the doc comment
itself recommends exactly the `var engramIdempotencyNS = uuid.MustParse("...")` pattern this phase
needs) [VERIFIED: local go.mod module cache via `go doc`, 2026-07-18].

## Package Legitimacy Audit

**Not applicable — this phase installs zero new packages.** Every symbol used
(`github.com/google/uuid.NewSHA1`/`MustParse`, `crypto/sha256`, `encoding/hex`) is either already
pinned in `go.mod` or Go stdlib. No `go get`/`npm install`/`pip install` step exists in this phase's
plan.

## Architecture Patterns

### System Architecture Diagram

```
storeArgs (MCP wire args)
    │  Content, Scope, Category, Tags, Source, Repo/Workspace/Worktree/BaseDir,
    │  Summary, + NEW: IdempotencyKey
    ▼
storeMemory(ctx, caller, storeArgs)  /  scheduleMemory(ctx, caller, scheduleArgs)
    │
    ├─ IdempotencyKey == "" ──────────────────────────────────────────┐
    │                                                                  │
    ├─ IdempotencyKey != "" ──► pointID := deterministicID(owner,      │
    │                              scope, key)                        │
    │                          existing, err := d.st.Get(ctx, pointID)│
    │                              │                                  │
    │                              ├─ ErrNotFound ────────────────────┤ (falls through
    │                              │                                  │  to embed path
    │                              ├─ found, fingerprint MATCH ──► return existing.(ID, ShortID)
    │                              │                                  no embed / no Upsert /
    │                              │                                  no MintShortID / no enqueue
    │                              │                                  (SC1)
    │                              │
    │                              └─ found, fingerprint MISMATCH ──► return
    │                                    ErrIdempotencyConflict (SC2) — before embed
    │                                                                  │
    │                                                                  ▼
    │                                                     m := storeArgs.toMemory(owner, actor, now)
    │                                                       m.ID = pointID (deterministic) or
    │                                                             uuid.NewString() (no key)
    │                                                       m.IdempotencyFingerprint = fp (or "")
    │                                                       m.EmbedderIdentity = ...
    │                                                                  │
    │                                                                  ▼
    │                                              vec := d.em.Embed(EmbedText(m.Content, m.Tags))
    │                                                        (external HTTP call — never
    │                                                         reached on a replay match/mismatch)
    │                                                                  │
    └──────────────────────────────────────────────────────────────► persistAndEnqueue(m, vec)
                                                                        MintShortID → Upsert
                                                                        (Qdrant point write,
                                                                         keyed on m.ID — same
                                                                         pointID = replace-in-place,
                                                                         SC4 atomicity) → tryEnqueue
```

The deterministic ID makes "idempotent write" a **point identity** decision, not a filter/search
decision — everything downstream of `pointID` (the `Get` check and the `Upsert` write) is the exact
same store primitives already used for every other write, just keyed differently.

### Recommended Project Structure

No new files. Changes land in the existing:
```
internal/server/tools.go   # storeArgs (+IdempotencyKey), toMemory, storeMemory, scheduleMemory,
                            # new check-before-embed helper, new sentinel error, jsonschema doc
internal/store/store.go    # Memory struct (+ fingerprint field), payload()/fromPayload() codec,
                            # (optionally) the deterministic-ID helper if placed here
```

### Pattern 1: Deterministic point ID replaces random UUID (D-02/D-03/D-04)

**What:** Derive the Qdrant point ID from a UUIDv5 hash of `(owner, scope, key)` instead of a
random UUIDv4, using an injective (length-prefixed) encoding so no two distinct tuples can ever
collide onto the same encoded string.

**When to use:** Only when the caller supplies a non-empty `idempotency_key` (D-01 — omission
preserves `uuid.NewString()` exactly).

**Example — mirrors `internal/auth`'s `namespacedOwner` discipline (`auth.go:138`):**
```go
// Source: internal/auth/auth.go:138 (namespacedOwner) — same length-prefix
// discipline extended from 2 components to 3.
//
// var engramIdempotencyNS = uuid.MustParse("<fixed, committed UUID>")
// (go doc: "MustParse ... simplifies safe initialization of global variables
// holding compiled UUIDs" — google/uuid v1.6.0, verified via `go doc`)

func idempotencyPointID(owner, scope, key string) uuid.UUID {
	name := fmt.Sprintf("%d:%s:%d:%s:%d:%s",
		len(owner), owner, len(scope), scope, len(key), key)
	return uuid.NewSHA1(engramIdempotencyNS, []byte(name))
}
```
`uuid.NewSHA1(space UUID, data []byte) UUID` — confirmed via `go doc github.com/google/uuid.NewSHA1`
against the pinned v1.6.0 module [VERIFIED: local module cache].

### Pattern 2: Check-before-embed replay short-circuit (D-08, mirrors `storeDiscovery`)

**What:** Resolve the deterministic ID and `Get` the existing record BEFORE calling `d.em.Embed`
(an external HTTP call to the embedder) — exactly the ordering `storeDiscovery` already uses
(`tools.go:718-746` resolve/own-check happen before `Embed` at `:748`).

**When to use:** Any keyed `store_memory`/`schedule_memory` call.

**Example:**
```go
// Source: mirrors internal/server/tools.go's storeDiscovery check-before-embed
// shape (tools.go:713-751), minus the arbitrary-client-ID resolution — owner
// is baked into the hash (D-09), so no OwnedOrAbsent gate is needed here.
func (d *deps) checkIdempotentReplay(ctx context.Context, owner string, a storeArgs) (id, shortID string, done bool, err error) {
	if a.IdempotencyKey == "" {
		return "", "", false, nil
	}
	pointID := idempotencyPointID(owner, a.Scope, a.IdempotencyKey).String()
	existing, gerr := d.st.Get(ctx, pointID)
	switch {
	case errors.Is(gerr, store.ErrNotFound):
		return pointID, "", false, nil // fall through: absent, proceed to embed+persist
	case gerr != nil:
		return "", "", false, gerr
	}
	if contentFingerprint(a) == existing.IdempotencyFingerprint {
		return existing.ID, existing.ShortID, true, nil // SC1: zero side-effects
	}
	return "", "", false, fmt.Errorf(
		"idempotency key reused with different content: %w", store.ErrIdempotencyConflict)
}
```
Note: the `pointID` returned on the fall-through-absent arm still needs to reach `toMemory` so the
subsequent `Upsert` targets the SAME deterministic ID computed here — pass it through explicitly
(don't recompute in `toMemory`) to avoid the two computations ever silently diverging.

### Pattern 3: Payload-only server-set fingerprint stamp (D-06, mirrors `EmbedderIdentity`)

**What:** A `json:"-"` field on `Memory`, written/read exclusively through `payload()`/
`fromPayload()`, never crossing any JSON wire.

**Example — the exact precedent this phase copies:**
```go
// Source: internal/store/store.go:193-210 (EmbedderIdentity / embedderIdentityKey)
// EmbedderIdentity string `json:"-"`
// const embedderIdentityKey = "embedder_identity"
// p[embedderIdentityKey] = m.EmbedderIdentity          // payload(), :409
// m.EmbedderIdentity = v.GetStringValue()               // fromPayload(), :499-500

// New field, same shape:
IdempotencyFingerprint string `json:"-"`
const idempotencyFingerprintKey = "idempotency_fingerprint"
// payload():    p[idempotencyFingerprintKey] = m.IdempotencyFingerprint
// fromPayload(): if v, ok := p[idempotencyFingerprintKey]; ok { m.IdempotencyFingerprint = v.GetStringValue() }
```
Write unconditionally (empty string for non-keyed records), matching `embedderIdentityKey`'s
always-write pattern rather than a conditional `if m.Category == "discovery"` block.

### Anti-Patterns to Avoid

- **Search-then-insert for the idempotency check:** never `Search`/`Scroll` by a key payload field
  — that's the TOCTOU race Pitfall 3 documents. Always a `Get` at a pre-derived deterministic ID.
- **Bare string concatenation for the hash input:** `owner + scope + key` (no delimiter/length
  prefix) lets `owner="a", scope="bc"` and `owner="ab", scope="c"` collide onto the same hash input
  — this is exactly the ambiguity `namespacedOwner`'s doc comment calls out and fixes.
- **Recomputing the deterministic ID independently in two places** (once in the check, once in
  `toMemory`) without sharing the same helper function — a future edit to the encoding in one
  call site and not the other silently breaks SC1.
- **Reusing `ErrNotFound` for the mismatch case:** D-10 is explicit — the conflict is a real,
  reportable condition, not an existence-hiding case; folding it into `ErrNotFound` would make an
  agent think its own key just doesn't exist yet, and retry into a loop.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Uniqueness enforcement under concurrent writes | An in-process mutex/lock keyed on `(owner, scope, key)`, or a "check flag" table | Qdrant's atomic per-point `Upsert` on a deterministic ID | A lock only protects one process; Qdrant's point-identity atomicity is server-side and correct under any number of engram replicas/processes — no coordination needed |
| Deterministic ID hashing | A hand-rolled hash-then-format-as-UUID routine | `uuid.NewSHA1` (already vendored, RFC 4122 UUIDv5-compliant) | Produces a wire-valid UUID string other tooling (short-id minting, `ResolvePointID`'s `uuid.Parse` fast path) already expects — a custom scheme risks producing something `uuid.Parse` rejects |
| Cross-owner/cross-scope collision prevention | A second payload field + filtered lookup to double-check "is this really my record" | Owner (and scope) baked directly into the hash input, so the ID itself IS the collision-prevention mechanism | Structural guarantees (can't be bypassed by a missed filter clause) are strictly stronger than a second runtime check that could itself have a bug |

**Key insight:** every "hard part" of idempotency here (uniqueness, atomicity, cross-tenant
isolation) reduces to a property Qdrant's point-identity model already provides for free once the ID
is derived correctly — the phase's actual engineering work is entirely in getting that derivation
(and the separate mismatch-detection fingerprint) right, not in building new infrastructure.

## Code Seam Verification

Every anchor CONTEXT.md's `<code_context>` section cites was re-read against the current tree on
2026-07-18 and confirmed accurate:

| CONTEXT.md claim | Verified against live code | Status |
|---|---|---|
| `tools.go:626` `storeArgs.toMemory` mints `ID: uuid.NewString()` at `:632` | `toMemory` at line 626, `ID: uuid.NewString()` at line 632, exact match | CONFIRMED |
| `tools.go:670` `persistAndEnqueue` — shared `MintShortID → Upsert → tryEnqueue` tail | Function at line 670; body is exactly `MintShortID` (671), `Upsert` (674), `tryEnqueue` (679) | CONFIRMED |
| `tools.go:713` `storeDiscovery` — resolve→own-check→`Get`→build-with-same-ID→`Upsert`, all before `Embed` | Function at line 713; `ResolvePointID`/`OwnedOrAbsent`/`Get` at 718-746, `Embed` at line 748 | CONFIRMED |
| `store.go:546` `Store.Upsert` — "same ID replaces in place," keys on `qdrant.NewID(m.ID)` at `:562` | `Upsert` at line 546; `Id: qdrant.NewID(m.ID)` at line 562, single-point `UpsertPoints` call, no read-modify-write | CONFIRMED |
| `store.go:193-207,409,500` `EmbedderIdentity`/`embedderIdentityKey` payload-only stamp | Field at 193-203 (`json:"-"`), const at 210, write at `payload()` line 409, read at `fromPayload()` lines 499-500 | CONFIRMED |
| `github.com/google/uuid` v1.6.0 exposes `uuid.NewSHA1` | `go doc github.com/google/uuid.NewSHA1` → `func NewSHA1(space UUID, data []byte) UUID`, resolved against the pinned module | CONFIRMED [VERIFIED] |
| `internal/auth` `namespacedOwner` `len:claim:len:value` encoding | `auth.go:138-140`: `fmt.Sprintf("%d:%s:%d:%s", len(claim), claim, len(value), value)` | CONFIRMED |

**New findings not in CONTEXT.md (surfaced by this verification pass):**

- **Connect write-lane auto-exclusion is structural, not a guard to write.** `connectapi.go`'s
  `StoreMemory` → `protoconv.go:90` `storeMemoryRequestToArgs` builds `storeArgs{}` field-by-field
  from `*engramv1.StoreMemoryRequest`; the proto has no `idempotency_key` field, so
  `IdempotencyKey` is always `""` for any Connect-originated call regardless of what the shared
  `storeMemory` handler does with it. The planner does not need an explicit `if fromConnect { … }`
  branch — D-13's Connect exclusion is already true by construction once the field is added only to
  `storeArgs`/the MCP tool schema.
- **`Store.Upsert` performs no internal authz check** (`store.go:546-568` is a bare
  `s.client.Upsert` call). `getWritable`/`OwnedOrAbsent` are handler-level gates called *before*
  `Upsert` in mutation paths (`update_memory`, `delete_memory`, `storeDiscovery`'s replace branch);
  plain `store_memory` today calls neither, because a fresh create's ownership is structural (the
  `Owner` field is always stamped from the verified `Subject`, never client-writable). D-09's
  "Cedar PDP + `getWritable`... guards the `Upsert` as defense-in-depth" should be read as "the
  broader Cedar/authz architecture makes cross-owner writes structurally impossible elsewhere in
  the codebase," not as a literal call site the keyed idempotent path passes through — there is
  none, and the design doesn't need one: the `Get` at a hash that can only ever encode this
  caller's own `(owner, scope, key)` is the entire safety argument, exactly as D-09 also (correctly)
  states in its second sentence. This is a documentation-precision note, not a design change.
- **`storeArgs` has no existing content-size validation** (unlike `storeDiscoveryArgs`'s
  `maxDiscoveryContentBytes` 64KB cap, checked in `validateStoreDiscovery`). There is currently no
  `validateStoreArgs` function for `store_memory` at all. This phase doesn't need to add one, but if
  the planner wants to bound `idempotency_key` length (recommended hygiene — an unbounded key
  string feeds directly into the hash and, if the raw key is ever stored for debugging, the payload),
  there's no existing validation seam to hook into; a small inline length check in the new
  check-before-embed helper is the simplest option. Not required by any locked SC — flagged as a
  discretionary hardening item, not a requirement.
- **`scheduleArgs` embeds `storeArgs` via Go field promotion** (`tools.go:441-445`), so adding
  `IdempotencyKey` to `storeArgs` flows automatically to `scheduleArgs`/`schedule_memory` with zero
  additional field declaration — confirming D-13's "both MCP write tools" is achieved by touching
  exactly one struct.
- **The fingerprint's field list (D-07) doesn't mention `scheduleArgs`'s `NotBefore`/`NotAfter`.**
  D-07 enumerates content/category/tags/source/repo/workspace/worktree/base_dir/summary — all
  `storeArgs` fields — but says nothing about the schedule window fields `scheduleArgs` adds. Taken
  literally, a `schedule_memory` replay with identical content but a *different* `not_before`/
  `not_after` window would still fingerprint-match and return the original record with its
  **original** window unchanged (the new window is silently ignored on replay). This is a plausible
  reading of D-07 as locked, but it's worth the planner explicitly confirming this is intended
  before implementing (see Open Questions) — it's the one place D-07's "client-authored identity
  fields" phrase is ambiguous about scope.

## Common Pitfalls

(Distilled from `.planning/research/PITFALLS.md` Pitfalls 2/3/4 — the phase's stated verification
oracle — with this phase's concrete mitigation restated against the verified code seam.)

### Pitfall 1 (source Pitfall 2): Idempotency key collides or leaks across owners

**What goes wrong:** Two different owners submit `store_memory` with the same idempotency key; if
uniqueness isn't scoped to the caller's owner, one owner's write silently overwrites/returns the
other's record, or is falsely rejected as a duplicate of a record it can't even see.

**Why it happens:** Idempotency is naturally modeled as "look up by key" — easy to build as a
global lookup unless deliberately carried over from engram's owner-scoping discipline.

**How to avoid:** Owner is baked directly into the UUIDv5 hash input (D-02/D-03) — cross-owner
collision is structurally impossible, not filter-enforced. No separate owner condition needed.

**Warning signs:** A `Search`/`Scroll` by key with no owner term; any idempotency code path that
doesn't route through the deterministic-ID `Get`.

### Pitfall 2 (source Pitfall 3): Concurrent identical writes race past a check-then-insert

**What goes wrong:** Two near-simultaneous calls with the same key both pass "does this exist?" and
both insert, producing two Qdrant points for one logical record.

**Why it happens:** Qdrant has no application-filter-level unique constraint; only per-point-ID
`Upsert` is atomic. "Search by key, then insert if absent" is a textbook TOCTOU race.

**How to avoid:** The deterministic point ID makes the write itself the atomic operation — there is
no separate "does it exist" step gating the `Upsert`; concurrent calls to the same key race onto the
same Qdrant point, and Qdrant's point-identity `Upsert` resolves them to exactly one point (D-12).

**Warning signs:** No concurrency test exists for the idempotency path; two separate store calls
("exists?" then "insert") with no atomic tie.

### Pitfall 3 (source Pitfall 4): "Same key, different content" left an undefined contract

**What goes wrong:** A retry with the same key but different content silently overwrites the
original — not idempotency (safe retry) but silent mutation-by-replay.

**Why it happens:** "Idempotency" and "upsert" get conflated without an explicit decision.

**How to avoid:** Already locked — D-05/D-06/D-07 store a fingerprint and D-10 rejects on mismatch
with a distinct sentinel, documented in the tool description (D-11).

**Warning signs:** No fingerprint field stored; no same-key/different-content test; tool description
silent on the behavior.

## Code Examples

### Deterministic ID + injective encoding (D-02, D-03, D-04)
```go
// Source: pattern mirrors internal/auth/auth.go:138 namespacedOwner, extended
// from (claim, value) to (owner, scope, key).
var engramIdempotencyNS = uuid.MustParse("EE450AC8-D047-494D-8FCD-BDEB88C3668B") // any fixed, committed UUID — example value, not load-bearing

func idempotencyPointID(owner, scope, key string) string {
	name := fmt.Sprintf("%d:%s:%d:%s:%d:%s",
		len(owner), owner, len(scope), scope, len(key), key)
	return uuid.NewSHA1(engramIdempotencyNS, []byte(name)).String()
}
```

### Content fingerprint (D-06, D-07)
```go
// Source: sha256 over the client-authored fields D-07 enumerates, same
// length-prefix injective discipline as the ID derivation. Tags sorted for
// determinism (Go map/slice iteration order is not stable across calls, and
// a caller could legitimately submit tags in a different order on replay).
func contentFingerprint(a storeArgs) string {
	tags := append([]string(nil), a.Tags...)
	sort.Strings(tags)
	var b strings.Builder
	for _, f := range []string{
		a.Content, a.Category, strings.Join(tags, "\x1f"),
		a.Source, a.Repo, a.Workspace, a.Worktree, a.BaseDir, a.Summary,
	} {
		fmt.Fprintf(&b, "%d:%s:", len(f), f)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
```
Note: the `\x1f`-joined tags are length-prefixed as a single field here, which is a pragmatic
simplification (tags are short curated keywords, not arbitrary content) rather than fully injective
per-tag length-prefixing. If stricter rigor is wanted, length-prefix each tag individually before
joining — flagged as planner discretion, not a correctness requirement for this phase's SCs.

### jsonschema field addition (D-11), mirrors the existing `Summary` field
```go
// Source: internal/server/tools.go:434 (storeArgs.Summary) — same jsonschema
// tag convention for a new optional field.
type storeArgs struct {
	// ... existing fields ...
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional; owner-scoped replay-safety key — a repeat call with the same key and identical content returns the original record unchanged; same key with different content is rejected; omit for a fresh record every time"`
}
```

### Sentinel error (D-10), two viable precedents — see discretion note below
```go
// Precedent A (internal/store, mirrors ErrNotFound/ErrInvalidArgument/ErrAmbiguousShortID,
// store.go:62-71): a store-layer sentinel that connectError could later switch on.
// var ErrIdempotencyConflict = errors.New("idempotency key reused with different content")

// Precedent B (internal/server, mirrors errRuleImmutable/errStaleSummary,
// identity.go:129 / summary.go:16): a handler-seam sentinel, since the fingerprint
// compare happens entirely in internal/server, not inside any internal/store method.
// var errIdempotencyConflict = errors.New("idempotency key reused with different content")
```
Both precedents exist in the codebase today (see Code Seam Verification). `store.Err*` sentinels are
all currently switched on by `connectError` (`connecterror.go`); `internal/server`-local sentinels
(`errRuleImmutable`, `errStaleSummary`) are not universally switched there but ARE asserted directly
via `errors.Is` in unit tests (`rules_test.go:254-255`, `connectapi_write_parity_test.go:443-444`).
Since Connect can never actually trigger this error this phase (idempotency_key is unreachable from
Connect — see Code Seam Verification), either location is currently equivalent in observable
behavior; `internal/store` is the safer default if Connect parity is picked up later (D-13's
"Connect parity follows" deferred item), since it's then already wired into `connectError` for free.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `store_memory`/`schedule_memory` always mint `uuid.NewString()` (`tools.go:632`) | Deterministic `uuid.NewSHA1(owner,scope,key)` when `idempotency_key` is supplied; `uuid.NewString()` unchanged when omitted | This phase | Repeat calls become safely re-runnable without any client-side dedup logic; zero behavior change for existing callers who never pass the new field |

**Deprecated/outdated:** Nothing in this phase deprecates existing behavior — it is purely additive
(new optional field, new conditional branch gated on that field being non-empty).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The schedule-window fields (`not_before`/`not_after`) are excluded from the D-07 content fingerprint, so a `schedule_memory` replay with identical content but a different window returns the original record with its original (unchanged) window | Code Seam Verification (new finding), Open Questions | LOW-MEDIUM — a caller retrying `schedule_memory` to *correct* a window (not just retry identically) would get silently ignored rather than updated; not a data-loss risk (nothing is corrupted), but a caller-surprise risk. Confirm intent with the user/planner before implementing if schedule-window corrections via replay are a plausible use case. |
| A2 | `internal/store` is the recommended location for the new sentinel error (over `internal/server`), specifically to pre-position for eventual Connect-lane parity | Code Examples | LOW — purely a code-organization choice; either location satisfies every locked SC this phase, and moving it later is a mechanical refactor, not a behavior change |
| A3 | The example `\x1f`-joined-tags fingerprint encoding (rather than per-tag length-prefixing) is an acceptable simplification for this phase's scope | Code Examples | LOW — tags are curated short keywords per the existing memory contract (not arbitrary user content), so a `\x1f` collision is exceptionally unlikely in practice; a per-tag length-prefixed version is a drop-in upgrade if stricter rigor is later wanted |

**If this table is empty:** N/A — see entries above. All three are LOW-to-MEDIUM risk, non-blocking
observations from verification, not load-bearing design decisions (those are all locked in
CONTEXT.md D-01–D-13 and were not re-litigated here).

## Open Questions

1. **Does a `schedule_memory` replay need the schedule window in its fingerprint?**
   - What we know: D-07 (locked) enumerates only `storeArgs` fields for the fingerprint; D-13 says
     the keyed branch lives at the shared `toMemory`/`persistAndEnqueue` seam so both tools reuse it
     unmodified.
   - What's unclear: whether a caller retrying `schedule_memory` with the same key but a corrected
     `not_before`/`not_after` should be treated as a content mismatch (reject) or silently ignored
     (current literal reading of D-07 — replay returns the original window unchanged).
   - Recommendation: implement per the literal D-07 field list (schedule window excluded from the
     fingerprint) as the default, but flag it explicitly in the plan's task description so it's a
     conscious choice, not an oversight; a one-line addendum to D-07 during planning can resolve this
     in either direction at negligible cost since it's still pre-implementation.

2. **Should the fixed `engramIdempotencyNS` UUID constant be generated fresh at implementation time,
   or is a specific value load-bearing?**
   - What we know: any fixed, committed UUID works — it only needs to never change once shipped
     (changing it would silently break every previously-computed deterministic ID, effectively
     un-deduping every existing idempotent record on the next replay).
   - What's unclear: nothing functionally; this is purely "pick one and commit it."
   - Recommendation: generate one fresh via `uuidgen`/`go run` at implementation time (the example
     value in this document, `EE450AC8-D047-494D-8FCD-BDEB88C3668B`, is illustrative only — not
     required) and add a code comment noting it must never be changed post-ship.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (no assertion library actively used — `testify` is present in `go.mod` only as an indirect transitive dependency) |
| Config file | none — plain `go test`, gated by `internal/server/tools_test.go`'s `requireQdrant`/`failOrSkipNoQdrant`/`TestMain` for integration-tier tests |
| Quick run command | `go test ./internal/server/... ./internal/store/...` (unit-tier tests using `spyStore`/in-memory fakes run without Qdrant; integration-tier tests self-skip cleanly if Qdrant is unavailable locally) |
| Full suite command | `task test` (lint + `go test ./...`); CI sets `ENGRAM_REQUIRE_QDRANT=true` so integration tests fail rather than silently skip |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-idempotent-capture (SC1) | Same key + same owner + identical content → returns original `(id, short_id)` unchanged, no duplicate, no side-effects (no re-embed/no new short_id/no summary re-enqueue) | integration (real Qdrant via `testDepsWithStore`, mirrors `TestStoreDiscoveryMintsThenReplacePreservesShortID` shape) | `go test -run TestStoreMemoryIdempotentReplayReturnsOriginal ./internal/server/...` | ❌ Wave 0 — new test |
| REQ-idempotent-capture (SC2) | Same key + same owner + different content → rejected with distinct mismatch sentinel, not silently overwritten/not folded into `ErrNotFound` | unit (`errors.Is` assertion, mirrors `rules_test.go:254-255`'s `errRuleImmutable` pattern) | `go test -run TestStoreMemoryIdempotentReplayRejectsMismatch ./internal/server/...` | ❌ Wave 0 — new test |
| REQ-idempotent-capture (SC3) | Two different owners, identical key → two independent records, neither visible to the other (matrix test, mirrors Pitfall 2's prescribed test) | integration (real Qdrant, two distinct `Subject`s) | `go test -run TestStoreMemoryIdempotentKeyScopedPerOwner ./internal/server/...` | ❌ Wave 0 — new test |
| REQ-idempotent-capture (SC4) | N concurrent identical (same key, same content) `store_memory` calls resolve to exactly one Qdrant point (race-safety, no duplicate under concurrency) | integration + `-race` (real Qdrant, `sync.WaitGroup` fan-out, mirrors `TestStoreMemoryReturnsWhenSummarizerHangs`'s `go func(){...}` pattern at `tools_test.go:767`) | `go test -race -run TestStoreMemoryIdempotentConcurrentIdenticalOnePoint ./internal/server/...` | ❌ Wave 0 — new test |
| REQ-idempotent-capture (SC5) | Omitting `idempotency_key` preserves today's behavior exactly — repeat calls with no key each mint a fresh random ID (two distinct IDs) | unit or integration (either `spyStore` or real store both prove distinctness) | `go test -run TestStoreMemoryNoKeyAlwaysFresh ./internal/server/...` | ❌ Wave 0 — new test |

**SC4 concurrency-test approach (concrete shape):** spin up N (e.g. 10-20) goroutines each calling
`d.storeMemory(ctx, caller, storeArgs{..., IdempotencyKey: "same-key"})` with byte-identical content
against a REAL `*store.Store` (`testDepsWithStore(t)` — the in-memory `spyStore` fake is
mutex-protected at the Go level and proves the application logic doesn't introduce its own race, but
only a real Qdrant instance proves the actual point-`Upsert` atomicity claim D-02/D-12 depend on);
collect all returned `(id, shortID, err)` tuples via a buffered channel; assert (a) every returned
`id` is identical, (b) `d.st.List(ctx, scope, subj, opts)` in that scope reports `total == 1`, and
(c) run with `go test -race` since this is new concurrent test code the existing test suite has no
prior `-race` convention for (first `-race` usage in this codebase per this research's Taskfile/CI
scan — flag for the planner that this is new CI-invocation surface, not an existing convention to
extend).

**SC4-vs-SC2 honest boundary (D-12, must be reflected in the SC4 test's assertions):** the
concurrency test asserts the no-duplicate invariant (exactly one point survives N concurrent
IDENTICAL-content calls) — it must NOT assert that a concurrent race of same-key-DIFFERENT-content
calls always fires the SC2 reject; D-12 documents that a truly simultaneous mismatch race can
converge to one record via last-writer-wins without the reject firing, and that is accepted,
correct behavior, not a defect to test against.

### Sampling Rate
- **Per task commit:** `go test ./internal/server/... ./internal/store/...`
- **Per wave merge:** `task test` (lint + full `go test ./... `)
- **Phase gate:** Full suite green (including the new `-race` concurrency test) before
  `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/server/tools_test.go` — five new tests (SC1-SC5) per the map above; no new test
  file needed, this phase's tests fit the existing `tools_test.go` convention (`storeDiscovery`'s
  idempotent-replace tests already live there: `TestStoreDiscoveryMintsThenReplacePreservesShortID`
  at line 1195, `TestStoreDiscoveryCrossOwnerShortIDDoesNotLeakUUID` at line 1228 — direct
  structural precedents for SC1/SC3 respectively)
- [ ] `internal/store/store_test.go` — if the fingerprint/deterministic-ID helper lands in
  `internal/store`, its own pure-function unit tests (injectivity spot-checks, e.g. `owner="a",
  scope="bc"` vs `owner="ab", scope="c"` must NOT collide) belong here, mirroring
  `TestMintShortIDUnique`/`TestMintShortIDRetriesOnCollision`'s style
- [ ] No framework install needed — `go test` + `-race` are both already available in the toolchain
  (`-race` requires `CGO_ENABLED=1` for the race detector itself, which conflicts with this
  project's `CGO_ENABLED=0` distroless build config for the SHIPPED binary — but does NOT conflict
  with running `go test -race` locally/in CI, since test binaries are built separately from the
  release binary; confirm the CI test job doesn't force `CGO_ENABLED=0` for `go test` invocations
  specifically, only for `goreleaser` release builds)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Unchanged — idempotency operates entirely downstream of the existing OIDC/static-token auth chain (Phase 22/23), consuming only the already-verified `c.Subj.Owner()` |
| V3 Session Management | no | Not touched by this phase |
| V4 Access Control | yes | Owner-in-hash structural isolation (D-02/D-03/D-09) — the deterministic point ID is itself the access-control boundary for the idempotent-replay `Get`; no new Cedar policy needed since no new access decision is introduced (a caller can only ever derive a hash for their own resolved owner) |
| V5 Input Validation | yes | `idempotency_key` is an opaque string fed through `sha256`/`uuid.NewSHA1` — no injection surface (never interpolated into a Qdrant filter string, SQL, or shell); no length bound exists today for `storeArgs` fields in general (see Code Seam Verification finding), so this phase is consistent with existing practice rather than introducing a new gap, but a modest length cap is a reasonable discretionary hardening addition |
| V6 Cryptography | yes (informational) | `sha256`/`uuid.NewSHA1` (SHA1-based UUIDv5) are used for **content-addressing/collision-avoidance**, not for a security/authentication guarantee — SHA1's cryptographic weaknesses (collision attacks) are not a threat here because the hash space is `(owner, scope, key)` triples under this project's control, not adversary-supplied data being defended against a forgery attack. This is the same threat-model reasoning `uuid.NewSHA1`'s own RFC 4122 UUIDv5 usage already assumes (it's a deterministic-ID scheme, not a MAC). No action needed. |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Idempotency key collision/leak across owners (Pitfall 2) | Information Disclosure / Tampering | Owner baked into the UUIDv5 hash input — structurally impossible, not filter-enforced (D-02/D-03) |
| Concurrent duplicate-write race (Pitfall 3) | Tampering (data integrity) | Deterministic-ID atomic `Upsert`, no application-level lock, no TOCTOU window (D-02/D-12) |
| Silent content overwrite via key replay (Pitfall 4) | Tampering / Repudiation | Stored fingerprint + explicit reject-on-mismatch sentinel (D-05/D-06/D-07/D-10), documented in the tool schema (D-11) so an agent can't accidentally rely on silent-overwrite semantics |

## Sources

### Primary (HIGH confidence)
- Direct codebase read, 2026-07-18: `internal/server/tools.go` (`storeArgs`, `toMemory`,
  `storeMemory`, `scheduleMemory`, `persistAndEnqueue`, `storeDiscovery`, `Register`/`AddTool`
  wiring, `connectapi.go`/`protoconv.go` Connect write-lane mapping), `internal/store/store.go`
  (`Memory` struct, `payload()`/`fromPayload()`, `Upsert`, `Get`, `ResolvePointID`,
  `getWritable`/`OwnedOrAbsent`, `ErrNotFound`/`ErrInvalidArgument`), `internal/auth/auth.go`
  (`namespacedOwner`), `internal/server/connecterror.go` (sentinel-to-Connect-code mapping),
  `internal/server/{rules_test.go,summary.go,identity.go}` (`errRuleImmutable`/`errStaleSummary`
  precedent), `internal/server/tools_test.go` (`requireQdrant`, `testDepsWithStore`,
  `TestStoreMemoryReturnsWhenSummarizerHangs` goroutine pattern, `TestStoreDiscoveryMintsThen...`/
  `TestStoreDiscoveryCrossOwnerShortIDDoesNotLeakUUID`), `Taskfile.yaml` (test targets, no existing
  `-race` usage repo-wide)
- `go doc github.com/google/uuid.NewSHA1` / `go doc github.com/google/uuid.MustParse` against the
  local pinned module (v1.6.0), 2026-07-18 [VERIFIED: local go.mod module cache]
- `.planning/phases/24-idempotent-capture/24-CONTEXT.md` — locked decisions D-01–D-13, discretion
  areas, deferred ideas (all copied verbatim into `<user_constraints>` above)
- `.planning/research/STACK.md` §(a) Idempotency-key / upsert against Qdrant — the load-bearing
  milestone-level recommendation this phase's design is grounded in
- `.planning/research/PITFALLS.md` Pitfalls 2, 3, 4 — the phase's verification oracle
- `.planning/REQUIREMENTS.md` — `REQ-idempotent-capture`, Deferred/Out-of-Scope sections
- `.planning/STATE.md` — v0.11.x build-order rationale (idempotency lands before supersession
  because supersession reuses this phase's re-`Upsert` mechanism)

### Secondary (MEDIUM confidence)
None required this phase — every claim was resolvable against the local codebase/toolchain
directly; no web search was needed or performed.

### Tertiary (LOW confidence)
None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies; exact API signatures confirmed via `go doc` against
  the pinned module, not training-data recall
- Architecture: HIGH — every code anchor CONTEXT.md cited was re-verified line-by-line against the
  live tree; the design pattern (`storeDiscovery`'s check-before-embed, `EmbedderIdentity`'s
  payload-only stamp) are shipped precedents in this exact codebase, not external analogies
- Pitfalls: HIGH — sourced from the milestone's own dedicated PITFALLS.md research (Pitfalls 2/3/4),
  itself grounded in direct reads of this codebase's authz/store invariants

**Research date:** 2026-07-18
**Valid until:** Effectively indefinite for the core design (deterministic-ID upsert is a stable
Qdrant/UUID pattern, not a fast-moving API surface) — but re-verify the code-line anchors if
Phase 25 (Supersession) or any other change lands in `tools.go`/`store.go` before this phase is
planned, since Phase 25 is documented as reusing this phase's re-`Upsert` mechanism and could shift
line numbers.
