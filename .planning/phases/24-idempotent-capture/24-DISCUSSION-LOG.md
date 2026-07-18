# Phase 24: Idempotent Capture - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-18
**Phase:** 24-Idempotent Capture
**Mode:** `--auto --all --chain` — all gray areas auto-selected; every question resolved to the
research-recommended default (no interactive prompts). Selections grounded in
`.planning/research/{STACK,PITFALLS}.md`, the locked ROADMAP/REQUIREMENTS idempotency contract, and
the shipped `storeDiscovery` / embedder-config-identity precedents.
**Areas discussed:** Omitted-key behavior, Deterministic-ID derivation, Mismatch detection, Replay
control-flow, Error surface, Tool coverage

---

## Omitted-key behavior (SC5)

| Option | Description | Selected |
|--------|-------------|----------|
| No fallback — omitted key → `uuid.NewString()` fresh every time | Honors locked SC5 exactly; deterministic ID only when a key is supplied | ✓ |
| Content-hash fallback (STACK.md §a) | `sha256(owner+scope+category+content)` dedups keyless byte-identical resubmissions | |

**Auto-selected:** No fallback (D-01).
**Notes:** The research proposed a keyless content-hash fallback, but locked SC5 ("omitting the key
preserves today's behavior exactly — a fresh record is created every time") and the ROADMAP decision
line ("deterministic derivation only when a key is supplied") forbid it. Fallback moved to Deferred
as an explicit non-default opt-in.

---

## Deterministic-ID derivation (SC1, SC3, SC4)

| Option | Description | Selected |
|--------|-------------|----------|
| `uuid.NewSHA1(engramNS, injective(owner, scope, key))` | UUIDv5 over the `(owner, scope, key)` tuple; scope included per Pitfall 2 + ROADMAP | ✓ |
| `uuid.NewSHA1(engramNS, owner+key)` (STACK.md §a narrower tuple) | Omits scope | |
| Random UUID + separate unique lookup key + conditional upsert/lock | Reintroduces a check-then-insert race | |

**Auto-selected:** deterministic UUIDv5 over `(owner, scope, key)` with injective encoding
(D-02/D-03/D-04).
**Notes:** Scope included (corrects STACK.md's narrower tuple to match Pitfall 2 and the ROADMAP
decision). Injective encoding mirrors `internal/auth`'s `namespacedOwner` discipline. Owner-in-hash
makes cross-owner collision structurally impossible (SC3) and concurrent identical retries converge
without a lock (SC4).

---

## Mismatch detection (SC2, Pitfall 4)

| Option | Description | Selected |
|--------|-------------|----------|
| Stored payload-only content fingerprint | `sha256` over client-authored fields, stored like the embedder-identity stamp; recompute-and-compare on replay | ✓ |
| Compare full stored `Content` string directly | No new field, but couples to server-side mutations (summary fill, bumps) | |
| Filtered search on a stored raw key + payload index | Adds a Qdrant index DEC-ef28 doesn't cover; extra round-trip | |

**Auto-selected:** stored payload-only content fingerprint (D-05/D-06/D-07).
**Notes:** Fingerprint is computed from incoming args at write time and stored; on replay it is
recomputed from the new incoming args and compared to the stored value — decoupling it from async
summary fill and access-count bumps so a legitimate post-fill replay still matches. Detection is a
point `get`, never a search (no new index, no ledger).

---

## Replay control-flow (SC1)

| Option | Description | Selected |
|--------|-------------|----------|
| Check-before-embed; suppress all side-effects on match | Resolve ID → `Get` before `Embed` (mirrors `storeDiscovery`); match returns existing `(id, short_id)` with no embed/upsert/mint/enqueue | ✓ |
| Embed-then-check | Simpler, but wastes an embed call on every replay and risks a summary re-enqueue | |

**Auto-selected:** check-before-embed with side-effect suppression (D-08/D-09).
**Notes:** Mirrors the shipped `storeDiscovery` ordering. Owner-in-hash means the `Get` can only
return the caller's own record, so no separate owner filter is needed on the happy path (D-09); the
Cedar write gate still guards the `Upsert` as defense-in-depth.

---

## Error surface (SC2)

| Option | Description | Selected |
|--------|-------------|----------|
| Distinct store sentinel, surfaced verbatim, documented in tool description | New `ErrIdempotencyConflict`-style error, distinguishable from `ErrNotFound`/validation | ✓ |
| Fold into the uniform 404 (DEC-xa6) | Would hide a real, reportable condition | |

**Auto-selected:** distinct sentinel + documented tool description (D-10/D-11).
**Notes:** A key conflict is a real reportable condition, not an existence-hiding case, so it is NOT
folded into the not-found path. The `store_memory` tool description must state the
match/mismatch/omitted-key contract (Pitfall 4 warning sign).

---

## Tool coverage

| Option | Description | Selected |
|--------|-------------|----------|
| Shared `storeArgs` → both `store_memory` + `schedule_memory`; Connect deferred | Key threaded at the shared `toMemory`/`persistAndEnqueue` seam | ✓ |
| `store_memory` only | Would diverge the two handlers that share the write tail | |
| Include Connect write lane now | Out of scope — REQUIREMENTS is MCP-first | |

**Auto-selected:** shared `storeArgs`, both MCP write tools, Connect deferred (D-13).
**Notes:** Reuses the IN-01 shared-seam reasoning so `store_memory` and `schedule_memory` never
diverge. Connect-lane idempotency stays deferred per REQUIREMENTS.

---

## Claude's Discretion

- Go signatures/seam placement (inline branch vs shared helper; `mintID` helper vs param to `toMemory`).
- The `engramIdempotencyNS` namespace UUID value and constant location.
- Injective byte encoding of `(owner, scope, key)` and canonical field ordering for the fingerprint.
- Payload field name for the fingerprint and the sentinel error's name.
- Whether the raw key is also stored (recommended: fingerprint-only).
- Test-file organization; exact shape of the SC4 concurrency test and the SC3 two-owner matrix test.

## Deferred Ideas

- Keyless content-hash dedup fallback (rejected by SC5; future opt-in mode only).
- Idempotency on the Connect write lane & other write RPCs (MCP-first; later milestone).
- Persisting/indexing the raw key for audit / "list my keys" (would need the avoided payload index).
- Configurable idempotency-key TTL/expiry (engram records are durable, not request-scoped).
