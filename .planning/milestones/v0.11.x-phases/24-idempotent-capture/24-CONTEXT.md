# Phase 24: Idempotent Capture - Context

**Gathered:** 2026-07-18
**Status:** Ready for planning
**Mode:** --auto --all --chain (all gray areas auto-selected and auto-resolved to the
research-recommended default, grounded in `.planning/research/{STACK,PITFALLS,SUMMARY}.md`, the
locked ROADMAP/REQUIREMENTS idempotency contract, and the shipped `storeDiscovery` /
embedder-config-identity precedents)

<domain>
## Phase Boundary

`store_memory` becomes safely re-runnable via an **optional** `idempotency_key` that provides
**strict replay-safety** (not upsert):

- Same key + same owner + same content → returns the **original** record/result unchanged, no
  duplicate, no side-effect (no re-embed, no new `short_id`, no summary re-enqueue).
- Same key + same owner + **different** content → **rejected** with an explicit, distinct mismatch
  error (never a silent overwrite — matches CLAUDE.md's "explicit, zero-junk, correctable" and
  Pitfall 4's Stripe-style conflict convention).
- The key is **owner-scoped** — two owners may reuse an identical key value without colliding
  (owner is baked into the deterministic point-ID hash, so cross-owner collision is *structurally*
  impossible, not filter-enforced).
- Concurrent identical retries resolve to **exactly one** Qdrant point (race-safe via
  deterministic-ID `Upsert`, never a search-then-insert TOCTOU check).
- **Omitting the key preserves today's behavior byte-for-byte** — a fresh random `uuid.NewString()`
  record is created every time (locked SC5).

Requirement: `REQ-idempotent-capture` (single requirement, single phase). This is a **small,
surgical write-path change**: no new store method, no new Qdrant collection, no new payload index,
no new dependency, no lock. The entire mechanism is "make the point ID a deterministic function of
`(owner, scope, key)` when a key is supplied" + one payload-only content-fingerprint field for
mismatch detection.

**Explicitly NOT this phase:**
- **Content-hash fallback for no-key calls** (STACK.md §(a) proposed hashing `owner+scope+category+content`
  to dedup byte-identical *keyless* submissions) — **rejected**: it directly contradicts locked SC5
  ("omitting the key → fresh record every time"). The ROADMAP decision line pins deterministic IDs
  to "**only when a key is supplied**." See D-01.
- **Idempotency on the Connect write lane / other write RPCs** — deferred (REQUIREMENTS.md Deferred:
  "v0.11.x lands these on the MCP `store_memory` path first; Connect parity follows"). See D-13.
- **A payload index / filtered search on the key** (deterministic ID makes idempotency a point
  *get*, not a *search* — DEC-ef28 indexes stay owner/scope/created_at only). See D-05.
- **A separate idempotency ledger / Redis / second collection** — violates DEC-2bv. See D-05.
- Supersession (Phase 25 — reuses this phase's re-`Upsert` mechanism) and citations (Phase 26).

</domain>

<decisions>
## Implementation Decisions

### Omitted-key behavior (SC5 — the load-bearing scope decision)
- **D-01 (no content-hash fallback):** When `idempotency_key` is absent/empty, `toMemory` keeps
  minting `uuid.NewString()` — a fresh record every time, today's behavior exactly. The
  research-proposed keyless content-hash fallback (`sha256(owner+scope+category+content)`) is
  **rejected for this phase**: it would make a repeat keyless call idempotent, violating locked SC5.
  The ROADMAP decision is explicit — deterministic derivation applies "only when a key is supplied."
  (Noted in Deferred as an opt-in the milestone did not ask for.)

### Deterministic point-ID derivation (SC1, SC3, SC4)
- **D-02 (derivation):** When a key is supplied, the record's Qdrant point ID is
  `uuid.NewSHA1(engramIdempotencyNS, injectiveEncode(owner, scope, key))` (UUIDv5) instead of
  `uuid.NewString()`. Same `(owner, scope, key)` ⇒ same point ID ⇒ the second `store_memory` is a
  native Qdrant upsert-replace of the first (SC1); different owners ⇒ different ID ⇒ no collision
  (SC3, Pitfall 2 "structurally impossible"); concurrent identical calls ⇒ same ID ⇒ one point (SC4,
  Pitfall 3 — no lock, no existence check).
- **D-03 (tuple = owner + scope + key):** The hash input is `(owner, scope, key)` — **scope
  included**, matching the ROADMAP decision line and Pitfall 2 (`(owner, scope, idempotency_key)`),
  not STACK.md §(a)'s narrower `owner+key`. Rationale: same key under two different scopes should
  seed two records, and scope-inclusion tightens the uniqueness domain at zero cost.
- **D-04 (injective encoding — anti-collision discipline):** The three components are combined with
  an **injective** encoding (NUL-delimited or length-prefixed, mirroring `internal/auth`'s
  `namespacedOwner` `len:claim:len:value` discipline), never bare string concatenation (which would
  let `owner="a"|scope="bc"` and `owner="ab"|scope="c"` collide). A fixed, committed
  `engramIdempotencyNS` UUID constant is the `uuid.NewSHA1` namespace. Exact byte layout is
  planner's discretion provided it is injective.

### Mismatch detection — stored content fingerprint (SC2, Pitfall 4)
- **D-05 (fingerprint is a get, not a search):** Detect same-key/different-content by a **point
  get** at the deterministic ID + a fingerprint compare — never a filtered `Search`/`Scroll` and
  never a payload index. Deterministic-ID derivation makes idempotency O(1) get-by-id. No new Qdrant
  index (DEC-ef28 unchanged), no ledger/second collection (DEC-2bv unchanged).
- **D-06 (payload-only fingerprint field):** Store a **payload-only, unindexed** content fingerprint
  on the record, reusing the exact shape of the shipped embedder-config-identity stamp
  (`embedderIdentityKey`, `json:"-"`, written by `payload()` / read by `fromPayload()` at
  `internal/store/store.go:193-207,409,500`). It is server-set, never client-supplied, never
  recall-filtered. Field name (e.g. `idempotency_fingerprint`) is planner's discretion.
- **D-07 (fingerprint over client-authored fields, computed at write time):** The fingerprint is
  `sha256` over the **client-authored identity fields as submitted** — content + category + tags +
  source + repo/workspace/worktree/base_dir + client-supplied summary (if any). It is computed from
  the **incoming args** at write time and **stored**; on replay it is **recomputed from the new
  incoming args and compared to the stored value**. This deliberately decouples the fingerprint from
  server-side mutations — the async summary fill (`ENGRAM_SUMMARY_ON_WRITE`), access-count bumps
  (D-04 bump path), and short-id minting never touch it, so a legitimate replay after an async
  summary fill still matches. Canonical field ordering is planner's discretion; it must be stable
  and injective.

### Replay control flow & side-effect suppression (SC1)
- **D-08 (check-before-embed ordering, mirrors `storeDiscovery`):** For a keyed `store_memory`,
  resolve the deterministic ID and `Get` the existing record **before** embedding — the same
  check-first ordering `storeDiscovery` uses (`tools.go:718-746` resolve/own-check happen before
  `Embed` at `:748`). This avoids a wasted embed call on every replay.
  - **Absent** → embed → `Upsert` at the deterministic ID → mint `short_id` → `tryEnqueue` (today's
    `persistAndEnqueue` tail, unchanged; the only change is the ID is deterministic not random).
  - **Present + fingerprint match** → return the **existing** `(id, short_id)` with **zero
    side-effects**: no embed, no `Upsert`, no `MintShortID`, no summary enqueue (SC1 "unchanged").
  - **Present + fingerprint mismatch** → reject **before** embedding with D-09's sentinel.
- **D-09 (owner isolation is structural, not a second filter):** A `Get` at the deterministic ID can
  only ever return the caller's own record because owner is inside the hash (Pitfall 2). No separate
  owner filter / `OwnedOrAbsent` gate is needed on the happy path — unlike `storeDiscovery`, which
  needs one because it accepts an arbitrary client-supplied ID. The existing store-layer write authz
  (Cedar PDP + `getWritable`) still guards the `Upsert` as defense-in-depth (DEC-cgb/DEC-cdr1).

### Error surface (SC2)
- **D-10 (distinct sentinel, surfaced verbatim):** A new distinct store sentinel error
  (e.g. `store.ErrIdempotencyConflict`; name is planner's discretion) is returned on
  same-key/different-content. It is surfaced to the MCP caller as an explicit, self-describing
  message that an agent can distinguish from `ErrNotFound` (DEC-xa6's uniform 404), from input
  validation errors, and from embed/store failures. It does **not** reuse the not-found path — a
  key conflict is a real, reportable condition, not an existence-hiding case.
- **D-11 (tool description documents the contract — Pitfall 4 warning sign):** The `store_memory`
  MCP tool description / `idempotency_key` field schema MUST state what happens on a
  same-key/different-content replay (returns original on match, rejects on mismatch, fresh record
  when omitted), so agents never rely on accidental semantics. The jsonschema for the new optional
  arg spells out "owner-scoped; omit for a fresh record every time."

### Concurrency honesty (SC4 vs SC2 boundary)
- **D-12 (no-duplicate absolute; strict-reject best-effort under a simultaneous mismatch race):** The deterministic-ID `Upsert` guarantees **no duplicate is ever created** under arbitrary
  concurrency (SC4 holds unconditionally). The mismatch **reject** (SC2) is a read-compare: two
  *truly simultaneous* same-key/**different**-content calls can both see "absent" and both upsert the
  same point (last-writer-wins → still exactly one record), so that one exact race converges without
  the reject firing. This is acceptable and correct — it is a caller bug, no duplicate results, and
  the *next* replay of either content deterministically detects the mismatch against the stored
  fingerprint. This honest boundary is captured so it is never mistaken for a defect later, and the
  concurrency test asserts the no-duplicate invariant (not reject-under-simultaneous-race).

### Tool coverage
- **D-13 (shared `storeArgs`; both MCP write tools; Connect deferred):** The optional
  `idempotency_key` is added to the **shared `storeArgs`** so both `store_memory` and
  `schedule_memory` gain it via the shared `toMemory` → `persistAndEnqueue` path (the same IN-01
  reasoning that created `persistAndEnqueue` to stop the two handlers diverging). The keyed
  check-before-embed branch (D-08) lives at the shared seam. **Connect write-lane idempotency is
  out of scope** (REQUIREMENTS Deferred: MCP-first). `store_discovery` / `store_rule` already have
  their own deterministic-ID/replace paths and are untouched.

### Claude's Discretion
- Exact Go signatures/seam: whether the keyed branch lives inline in `storeMemory`/`scheduleMemory`
  or in a new shared helper alongside `persistAndEnqueue`; whether `toMemory` takes the derived ID
  as a param or a dedicated `mintID(owner, scope, key)` helper is introduced.
- The fixed `engramIdempotencyNS` namespace UUID value and where the constant lives
  (`internal/store` vs `internal/server`).
- Exact injective byte encoding of `(owner, scope, key)` (NUL-delimited vs length-prefixed) and the
  canonical field ordering fed into the content fingerprint.
- The payload field name/key for the stored fingerprint and the sentinel error's name.
- Whether the raw key is also stored in the payload (NOT required — nothing reads it back; the key's
  job ends once hashed into the ID and fingerprint). Recommend fingerprint-only to keep it minimal.
- Test-file organization and the precise shape of the concurrency test (SC4) and the
  same-key/two-owners matrix test (SC3, Pitfall 2).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase definition & requirements
- `.planning/ROADMAP.md` — Phase 24 entry (§383-410): goal, 5 success criteria, and the locked
  decision line ("same-key/different-content → **reject**"; "deterministic point-ID via
  `uuid.NewSHA1` over `(owner, scope, key)` replaces `uuid.NewString()` **only when a key is
  supplied**"). Also §29-30: the "capture trio in strict internal order (idempotency → supersession
  → citations, since supersession reuses idempotency's re-Upsert mechanism)".
- `.planning/REQUIREMENTS.md` — `REQ-idempotent-capture` (§62-66); the **Deferred** entry
  (idempotency on the Connect lane follows MCP-first, §106-108); the **Out of Scope** row (no new
  collection / no separate authz store — DEC-2bv, §121).

### Milestone research (v0.11.x) — the design is already specified here
- `.planning/research/STACK.md` **§(a) Idempotency-key / upsert against Qdrant** — the load-bearing
  recommendation: deterministic `uuid.NewSHA1` point ID; Qdrant `Upsert` already replaces-in-place;
  `github.com/google/uuid` v1.6.0 already exposes `NewSHA1`; the payload-only content-fingerprint
  stamp mirrors the Phase-13 embedder-config-identity field; the "Alternatives Considered" /
  "What NOT to Use" rows (no payload index, no ledger, no Redis, no second collection). **Note the
  one deviation this CONTEXT locks:** STACK.md's keyless content-hash *fallback* is rejected by SC5
  (see D-01), and its hash tuple is corrected to include `scope` per Pitfall 2 (see D-03).
- `.planning/research/PITFALLS.md` — **Pitfall 2** (key collides/leaks across owners → owner-in-hash,
  D-02/D-03), **Pitfall 3** (concurrent identical writes race a check-then-insert → deterministic-ID
  Upsert, no lock, D-02/D-12), **Pitfall 4** (same-key/different-content is an undefined contract →
  store a fingerprint, reject on mismatch, document in the tool description, D-05/D-06/D-07/D-10/D-11).
  These three pitfalls are the phase's verification oracle.
- `.planning/research/SUMMARY.md` — executive summary of the milestone; idempotency is the
  self-contained, auth-orthogonal member of the capture trio.

### Locked ADRs / decisions governing this phase
- `docs/adr/engram-2bv-*.md` (DEC-2bv) — single Memory collection; the idempotency fingerprint is a
  payload field on it, never a new collection/store.
- DEC-ef28 (owner/scope/created_at are the only payload indexes) — the idempotency key is a
  deterministic-ID *get*, so it adds **no** index.
- `docs/adr/engram-cgb-*.md` (DEC-cgb) + `docs/adr/engram-cdr1-*.md` (DEC-cdr1) — the store is the
  sole authz chokepoint; the keyed `Upsert` still passes the Phase-22 Cedar write gate
  (defense-in-depth behind the owner-in-hash structural guarantee, D-09).
- `docs/adr/engram-xa6-*.md` (DEC-xa6) — uniform not-found; the idempotency-conflict error is a
  **distinct** sentinel, deliberately NOT folded into the 404 path (D-10).

### Prior phase context
- `.planning/phases/22-cedar-authz-foundation-store-enforcement/22-CONTEXT.md` — the write-gate /
  `getWritable(authz.Action)` shape the keyed `Upsert` flows through.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/server/tools.go:626` `storeArgs.toMemory(owner, actor, createdAt)` — the ONE place
  `ID: uuid.NewString()` is minted (`:632`). D-02 swaps this for the deterministic ID when a key is
  present. Shared by both `store_memory` and `schedule_memory` (D-13).
- `internal/server/tools.go:670` `persistAndEnqueue` — the shared `MintShortID → Upsert →
  tryEnqueue` tail. The keyed match-path (D-08) must short-circuit BEFORE this (no mint/upsert/
  enqueue on an idempotent replay).
- `internal/server/tools.go:713` `storeDiscovery` — **the precedent to mirror**: it already does
  resolve-ID → owner-check → `Get`-existing → build-with-same-ID → `Upsert`, and it resolves/checks
  **before** `Embed` (`:748`). The keyed `store_memory` branch is the same check-first shape, minus
  the arbitrary-client-ID resolution (owner is in the hash, D-09).
- `internal/store/store.go:546` `Store.Upsert` — "same ID replaces in place"; keys solely on
  `qdrant.NewID(m.ID)` (`:562`). Deterministic `m.ID` is the entire idempotency mechanism — no new
  store method.
- `internal/store/store.go:193-207,409,500` `EmbedderIdentity` / `embedderIdentityKey` — the
  **payload-only server-set stamp** precedent (`json:"-"`, written by `payload()`, read by
  `fromPayload()`). The content fingerprint (D-06) copies this shape exactly.
- `github.com/google/uuid` v1.6.0 (already vendored) — `uuid.NewSHA1(ns, name)` for UUIDv5;
  `crypto/sha256` (stdlib) for the content fingerprint. **No new dependency.**
- `internal/auth` `namespacedOwner` `len:claim:len:value` injective encoding — the discipline D-04
  borrows for the `(owner, scope, key)` hash input.

### Established Patterns
- Deterministic-ID + `Upsert`-replace for a client-addressed record is already shipped
  (`storeDiscovery`) — idempotent `store_memory` is that pattern with a *server-derived* ID.
- Payload-only, unindexed, server-set identity stamps are already shipped (embedder-config-identity,
  Phase 13) — the fingerprint is a second instance of the same idea.
- Dual-write-tool parity via a shared helper (`toMemory` / `persistAndEnqueue`, IN-01) is the seam
  where the key is threaded so `store_memory` and `schedule_memory` never diverge.

### Integration Points
- `internal/server/tools.go` — new optional `IdempotencyKey` on `storeArgs` (jsonschema per D-11);
  keyed branch at the shared seam (D-08); deterministic ID in/around `toMemory` (D-02).
- `internal/store/store.go` — new payload-only fingerprint field in the `Memory` struct + `payload()`/
  `fromPayload()` marshal pair (D-06); the fingerprint helper + sentinel error (D-07/D-10). The
  deterministic-ID helper may live here or in `internal/server` (Claude's discretion).
- No `internal/config` change — idempotency has no operator knobs; it is a per-call arg.
- No proto/Connect change — Connect idempotency is deferred (D-13).

</code_context>

<specifics>
## Specific Ideas

- The reject error message should name the condition plainly (e.g. "idempotency key reused with
  different content") so a coding agent reads it and corrects, per CLAUDE.md's "correctable" intent.
- The idempotent match-path returns the ORIGINAL `(id, short_id)` — an agent that retries a store
  and gets back the same `short_id` it saw the first time is the observable proof of replay-safety.
- Keep the fingerprint canonicalization deterministic across process restarts and Go map iteration
  order (sort tags; fixed field order) — a non-deterministic fingerprint would false-positive a
  mismatch on a legitimate replay.
- SC verification oracle: SC1 (identical replay → same record, no dup), SC2 (mismatch → distinct
  error), SC3 (two owners, same key → two independent records — Pitfall 2 matrix test), SC4
  (concurrent identical → exactly one point — Pitfall 3 concurrency test), SC5 (no key → fresh every
  time — a keyless-repeat test asserting two distinct IDs).

</specifics>

<deferred>
## Deferred Ideas

- **Keyless content-hash dedup fallback** (STACK.md §(a)'s `sha256(owner+scope+category+content)`
  → deterministic ID for *keyless* byte-identical resubmission) — rejected here because it violates
  locked SC5. If a future milestone wants opt-in keyless dedup, it must be a distinct, explicitly
  flagged mode, never the default (D-01).
- **Idempotency on the Connect write lane & other write RPCs** — MCP `store_memory`/`schedule_memory`
  first (REQUIREMENTS Deferred); Connect parity is a later milestone item (D-13).
- **Persisting/indexing the raw idempotency key for audit or "list my keys"** — not needed for the
  contract (nothing reads it back); a future observability feature could add it, but it would
  require the payload index this phase deliberately avoids.
- **A configurable idempotency-key TTL / expiry** (Stripe expires keys after 24h) — engram records
  are durable, not request-scoped; no expiry this phase. Revisit only if key-space growth ever
  becomes a concern.

</deferred>

---

*Phase: 24-Idempotent Capture*
*Context gathered: 2026-07-18 (auto mode)*
