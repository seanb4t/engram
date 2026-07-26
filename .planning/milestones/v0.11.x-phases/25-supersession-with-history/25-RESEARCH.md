# Phase 25: Supersession with History - Research

**Researched:** 2026-07-19
**Domain:** Go / Qdrant payload-mutation semantics, MCP tool surface design, authz write-gate reuse
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Stamp mechanism (how the target's `superseded_by` is written)**
- **D-01:** Back-stamp the target with **`SetPayload` (partial payload update)**, following the
  existing `SetVisibility` pattern (`store.go:1693`): `getWritable(target, subj, ActionWrite)`
  then a payload-only, vector-preserving `SetPayload` of the single `superseded_by` key.
  **Rationale / deviation note:** ROADMAP.md phrases the Phase 24 dependency as "reuses
  idempotency's payload-only **re-Upsert** mechanism." We refine that to `SetPayload` because a
  full re-Upsert *replaces the entire payload* and therefore inherits the CR-01 lost-write hazard
  documented in engram memory `m43h2yt97m` (a concurrent same-point Upsert drops a co-written
  key). `SetPayload` merges the one key and is race-safe for this stamp. The payload-only intent
  is preserved; only the primitive changes. **Planner/researcher: confirm this against the
  Phase 24 mechanism and flag if the roadmap intended a literal re-Upsert.**
- **D-02:** `SetVisibility`'s TOCTOU property carries over: if the target is deleted between the
  ownership gate and `SetPayload`, Qdrant returns NotFound and the error propagates unchanged
  (fail-closed) — no extra re-fetch needed.

**Entry point (tool surface)**
- **D-03:** Expose a **dedicated MCP tool/verb** (`supersede_memory`), NOT an overloaded field on
  `store_memory`. Follows the DEC-90w precedent (`schedule_memory` is a distinct verb rather than
  a `store_memory` flag). The tool stores the new/correcting memory (normal write path, caller
  owns it) AND back-stamps the target — one atomic-intent operation from the caller's view.
- **D-04:** The new record carries `supersedes = <target_id>`; the target gets `superseded_by =
  <new_record_id>`. The new record is created through the normal write path (already write-gated
  as the caller's own record); only the target back-stamp needs the additional `getWritable` gate.

**Single-hop model & cycle rejection (write-time validation)**
- **D-05:** Reject at write time if the target is **already superseded** (`superseded_by`
  non-empty) → a typed error (e.g. `store.ErrAlreadySuperseded`). This keeps a single live head
  and makes cycles structurally impossible.
- **D-06:** **Forward chains are allowed** — superseding the current live head is how history
  accumulates (C supersedes A which superseded B → chain C→A→B, all fetchable by id). Only
  superseding a non-head (already-superseded) record is rejected.
- **D-07:** **No automatic superseding** — supersession never fires on a similarity threshold or
  any write-through path (SC4). It is only ever the explicit `supersede_memory` call.

**Link representation (payload shape)**
- **D-08:** Add two **optional `*string`** payload keys to `store.Memory`: `supersedes` and
  `superseded_by` (single-id each, matching the single-hop linear model; not arrays). Optional
  pointer follows the repo's `*time.Time`-for-optional convention (never zero-value +
  `omitempty`). Absent on every pre-feature record → unchanged behavior.
- **D-09:** Recall-gate integration: add a `superseded_by IS EMPTY` condition to the
  Search/List recall filter, alongside `activeWindowConditions` (`store.go:727`). `get_memory`
  (`Store.Get`, id-addressed, ungated) is deliberately left untouched so superseded records stay
  fetchable (SC2). Same soft-hide-at-recall-gate shape as the DEC-ufz scheduling window.

### Claude's Discretion
- Exact error type names/wire mapping (`ErrAlreadySuperseded` → Connect/MCP code), tool argument
  names, and whether `supersede_memory` shares `storeArgs` with `store_memory` — left to the
  planner to fit existing conventions.

### Deferred Ideas (OUT OF SCOPE)
- **Un-supersede / restore** — reversing a supersession link is a separate capability; not in
  REQ-supersession-links. Note for a future phase if needed.
- **Multi-parent / merge supersession** (one record superseding several) — the single-hop
  `*string` model excludes it by design; revisit only if a real need appears.
- **Surfacing supersession chains in the operator console / Connect read lane** — this phase is
  MCP-store-only; UI/Connect exposure is its own phase.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-supersession-links | A memory can supersede another via additive `supersedes` / `superseded_by` payload links. Superseded records are soft-hidden from recall (reusing the `DEC-ufz` recall gate) but remain fetchable by id (`get_memory`), and the supersede operation routes through the ownership **write** gate (`getWritable`/`OwnedOrAbsent`), never a read grant. Correction is explicit and preserves history — it never deletes or silently overwrites. | Resolved D-01 (`SetPayload` vs re-Upsert) with HIGH confidence via Qdrant docs + this repo's own `UpdatePayload`/`persistAndEnqueue` doc comments (see Summary, Pattern 1, State of the Art). Full `Store.Supersede` method signature, `supersede_memory` MCP tool registration, recall-gate dual-call-site extension, `ErrAlreadySuperseded` sentinel + `connectError` mapping, and 7 Wave-0 test cases specified (see Architecture Patterns, Common Pitfalls, Validation Architecture). |

</phase_requirements>

## Summary

This phase adds nothing new to the stack — no new dependency, no new external service, no new
Qdrant feature. It is a pure composition of four already-shipped primitives in
`internal/store/store.go`: the owner-only write gate (`getWritable`), the payload-only mutation
primitive (`SetPayload`, as used by `SetVisibility`), the recall-gate filter assembly
(`activeWindowConditions`), and the optional-pointer payload-field pattern (`NotBefore`/`NotAfter`).
The one open technical question CONTEXT.md flagged as highest-value — whether `SetPayload` (D-01)
or a literal re-Upsert (ROADMAP wording) is correct for the target back-stamp — is **resolved with
HIGH confidence in favor of `SetPayload`**, verified against both the Qdrant Go client's proto
field semantics, official Qdrant documentation, and — most importantly — this repository's own
prior art: `internal/store/store.go:1571-1580` already documents, in the `UpdatePayload` doc
comment, the exact lost-write hazard CONTEXT.md's D-01 rationale describes, and explicitly says
"Do not reintroduce a whole-payload write here unless a real optimistic-concurrency/version (CAS)
mechanism is added first." The Phase 24 `persistAndEnqueue` comment (`internal/server/tools.go:748-757`)
independently documents the same Upsert-replaces-whole-payload hazard for the CR-01 concurrent-racer
case. D-01 is not just plausible — it is the codebase's own established convention, restated by two
independent doc comments authored in two different phases.

**Primary recommendation:** Implement `Store.Supersede` as a new store method that (1) resolves and
authz-gates the target via `getWritable(target, subj, authz.ActionWrite)`, (2) rejects if the
target's `SupersededBy` is already non-empty (`store.ErrAlreadySuperseded`), (3) stores the new
record through the existing `Upsert` path (normal create, caller already owns it), and (4)
back-stamps the target with a single-key `SetPayload({"superseded_by": newID})` — mirroring
`SetVisibility` almost verbatim, action swapped to `ActionWrite`, key swapped to `superseded_by`.
Expose it via a dedicated `supersede_memory` MCP tool (D-03) that composes `d.storeMemory`-style
embed+persist for the new record with the store-level back-stamp call. Add `superseded_by IS EMPTY`
to `activeWindowConditions`'s call sites (`Search` and `List`), leave `Get` untouched.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| New-record creation (the correcting memory) | API / Backend (`internal/server` handler → `internal/store.Upsert`) | — | Identical to `store_memory`'s existing write path; caller owns the new record from creation |
| Target back-stamp (`superseded_by`) | Database / Storage (`internal/store.Supersede` via `SetPayload`) | API / Backend (owner-gate call site) | Payload-only mutation, vector-preserving — store-layer primitive, gated by the API-layer caller identity |
| Write authorization (owner-only) | API / Backend (`internal/authz` PDP via `getWritable`) | — | Existing Cedar-backed gate (Phase 22/23); no new policy needed — `own_records.cedar` already grants "any action" to the resource owner |
| Recall-gate exclusion (soft-hide) | Database / Storage (`internal/store` filter assembly) | — | Same tier as the existing `not_before`/`not_after` gate it extends |
| Fetch-by-id (ungated) | Database / Storage (`internal/store.Get`) | — | Deliberately untouched — superseded records must stay fetchable |
| Cycle/single-hop rejection | Database / Storage (`internal/store.Supersede`, pre-mutation check) | — | Write-time validation belongs next to the mutation it guards, not the handler |
| MCP tool surface | API / Backend (`internal/server` `supersede_memory` registration) | — | Mirrors `schedule_memory`/`set_visibility` registration pattern exactly |
| Connect/HTTP parity | Out of scope this phase | — | See "Connect Lane Parity" below — confirmed not required |

## Standard Stack

No new libraries. This phase is 100% composition of existing internal packages:

### Core (existing, reused verbatim)
| Package | Version | Purpose | Why reused |
|---------|---------|---------|--------------|
| `github.com/qdrant/go-client` | v1.18.3 `[VERIFIED: go.mod]` | `SetPayload`/`Upsert`/`NewIsEmpty` primitives | Already the store's only Qdrant client; `SetPayload` and `NewIsEmpty` are already exercised by `SetVisibility` and `activeWindowConditions` respectively |
| `internal/authz` (cedar-go v1.8.0, Phase 22) | in-repo | Owner-only write gate via `getWritable`/`ActionWrite` | `own_records.cedar` (`permit(principal, action, resource) when { resource.owner == principal.owner }`) already grants **any** action to the owner — zero new Cedar policy needed |

### Supporting
None — no new supporting packages required.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `SetPayload` single-key back-stamp (D-01) | Full re-Upsert of the target's whole payload (ROADMAP wording) | **Rejected.** A full `Upsert` on the target replaces its ENTIRE payload map from a snapshot taken before the mutation; any concurrent write to a *different* key on that same point (e.g. a concurrent `update_memory` touching `content`/`tags`, or a concurrent `set_visibility`) is silently dropped once the re-Upsert lands. This is the exact CR-01 lost-write class already documented at `store.go:1571-1580` and independently at `internal/server/tools.go:748-757` (Phase 24). `SetPayload` sends only the changed key(s) and Qdrant merges them into the existing payload, so a concurrent single-key writer to a *different* key is unaffected. |
| Dedicated `supersede_memory` tool (D-03) | `supersedes` field on `store_memory` | **Rejected per CONTEXT.md D-03**, DEC-90w precedent (`schedule_memory` as distinct verb). Overloading `store_memory` would force every keyed-idempotency/window/citation combination to also reason about supersede semantics. |

**Installation:** None — no `go get`/`go.mod` changes required for this phase.

**Version verification:** `go.mod:18` pins `github.com/qdrant/go-client v1.18.3`. `SetPayloadPoints`,
`UpsertPoints`, and `NewIsEmpty` are all already used successfully against this exact pinned version
elsewhere in `store.go` — no version-compat risk.

## Package Legitimacy Audit

**Not applicable.** This phase adds zero new external dependencies. All primitives used
(`qdrant.SetPayloadPoints`, `qdrant.NewIsEmpty`, `internal/authz.ActionWrite`) already ship in the
repo's `go.mod` and are exercised by existing, tested code paths (`SetVisibility`,
`activeWindowConditions`). No `npm install`/`pip install`/`cargo add` of any kind. Skip the
Package Legitimacy Gate protocol for this phase — there is nothing to audit.

## Architecture Patterns

### System Architecture Diagram

```
                 MCP client
                     |
                     v
        supersede_memory(new_content, ..., supersedes=<target_id>)
                     |
                     v
       +-------------------------------+
       | internal/server: supersedeMemory handler |
       +-------------------------------+
                     |
        1. resolve target id (ResolvePointID, short_id or UUID)
                     |
        2. embed new_content -> vec  (existing d.em.Embed)
                     |
                     v
       +--------------------------------------------+
       | internal/store.Supersede(ctx, newMem, vec,  |
       |                          targetID, subj)     |
       +--------------------------------------------+
                     |
        a. getWritable(targetID, subj, ActionWrite)  --> Cedar PDP (own_records.cedar)
                     |         |
                     |    ErrNotFound (not owner / doesn't exist) --> propagate, abort
                     v
        b. if target.SupersededBy != "" --> ErrAlreadySuperseded --> abort (single-hop D-05)
                     |
                     v
        c. Upsert(newMem{Supersedes: &targetID, ...}, vec)   -- normal create, full payload OK
                     |         (new record has no prior payload to lose keys from)
                     v
        d. SetPayload(targetID, {"superseded_by": newMem.ID})  -- single-key merge, vector preserved
                     |
                     v
              both records persisted; target's vector/content/tags/visibility UNTOUCHED

  --- separately, at recall time ---

  search_memory / list_memory
                     |
                     v
       Search()/List() filter assembly
                     |
        f.Must = ownerScopeFilter + activeWindowConditions(now) + NEW: IsEmpty("superseded_by") + tags + ...
                     |
                     v
              Qdrant Query/Scroll  --> superseded records excluded

  get_memory(id)  --> Store.Get(id)  --> UNGATED, no filter --> superseded records still fetchable
```

### Recommended Method Signatures

```go
// internal/store/store.go — new sentinel, alongside ErrIdempotencyConflict (store.go:90)
var ErrAlreadySuperseded = errors.New("target is already superseded")

// Supersede stores newMem (a normal, caller-owned create — same shape as Upsert) and
// back-stamps target's superseded_by link, atomically-intent from the caller's view
// (two Qdrant ops, not a transaction — same non-atomicity accepted by SetVisibility's
// own TOCTOU note). newMem.Supersedes MUST already be set to target's resolved id by
// the caller before this is invoked (mirrors OwnedOrAbsent's contract: caller threads
// resolved ids in, store methods do not resolve short ids).
func (s *Store) Supersede(ctx context.Context, newMem Memory, vec []float32, target string, subj Subject) (err error) {
	// 1. Owner-only write gate on the TARGET (not the new record — that's a normal
	//    create, already write-gated by construction).
	targetRec, err := s.getWritable(ctx, target, subj, authz.ActionWrite)
	if err != nil {
		return err // ErrNotFound: not owner, or doesn't exist — fail-closed, no leak
	}
	// 2. Single-hop / cycle rejection (D-05/D-06): reject if target is already a
	//    non-head record. Self-reference (newMem superseding itself) is structurally
	//    impossible pre-create since newMem.ID is freshly minted, but guard anyway.
	if targetRec.SupersededBy != nil && *targetRec.SupersededBy != "" {
		return fmt.Errorf("%w: %s", ErrAlreadySuperseded, target)
	}
	// 3. Store the new record (normal Upsert — fresh ID, no concurrent-writer risk).
	if err := s.Upsert(ctx, newMem, vec); err != nil {
		return err
	}
	// 4. Back-stamp the target: SetPayload, single key, vector-preserving — mirrors
	//    SetVisibility exactly (store.go:1693), action swapped Share->Write, key
	//    swapped visibility->superseded_by.
	_, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"superseded_by": newMem.ID}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(target)}),
	})
	// TOCTOU note (identical to SetVisibility, store.go:1688-1692): if target is
	// deleted between getWritable and this SetPayload, Qdrant's point-ID-selector
	// SetPayload returns NotFound and propagates unchanged — fail-closed, no re-fetch
	// needed. BUT: this leaves the new record persisted with a Supersedes link to a
	// now-nonexistent target. Planner must decide: accept as an orphaned-but-harmless
	// forward link (the new record is still valid, fetchable, and in recall), or
	// treat step-4 failure as a hard error requiring rollback of step 3 (no
	// transaction primitive exists in this codebase for that — Qdrant has no
	// multi-point transactional write here). RECOMMENDATION: accept the orphan;
	// document it exactly as SetVisibility documents its own TOCTOU acceptance.
	return err
}
```

```go
// internal/server/tools.go — dedicated tool args, mirrors setVisibilityArgs (tools.go:581)
// and storeArgs (tools.go:424). Field naming: Claude's Discretion per CONTEXT.md.
type supersedeArgs struct {
	Supersedes string   `json:"supersedes" jsonschema:"id (full UUID or short_id) of the memory this new record corrects/replaces"`
	Content    string   `json:"content" jsonschema:"the corrected/replacing memory text"`
	Scope      string   `json:"scope"`
	Source     string   `json:"source"`
	Category   string   `json:"category"`
	Tags       []string `json:"tags,omitempty"`
	Repo       string   `json:"repo,omitempty"`
	Workspace  string   `json:"workspace,omitempty"`
	Worktree   string   `json:"worktree_path,omitempty"`
	BaseDir    string   `json:"base_dir,omitempty"`
	Summary    string   `json:"summary,omitempty"`
	// NOTE: whether supersede_memory also accepts idempotency_key is Claude's
	// Discretion (CONTEXT.md) — if storeArgs is embedded verbatim (like scheduleArgs
	// embeds it, tools.go:445-449) it comes along "for free" via Go field promotion,
	// same IN-01 cross-tool-namespace caveat would then apply. RECOMMEND: embed
	// storeArgs (not hand-roll a parallel struct) to inherit idempotency_key,
	// content, scope, etc. consistently — reduces drift risk (the repo already flags
	// exactly this kind of duplication as a past defect, tools.go:734-736).
}

// supersedeMemory: resolve target -> embed new content -> store.Supersede.
// Mirrors storeMemory's shape (tools.go:712-732) with an extra target-resolve step
// borrowed from setVisibility's ResolvePointID + GetReadable-for-category-check
// pattern (tools.go:1201-1224) — EXCEPT supersede uses getWritable (via
// store.Supersede), not GetReadable, since this is a write-gated mutation of the
// target, not a read.
func (d *deps) supersedeMemory(ctx context.Context, c caller, a supersedeArgs) (string, string, error) {
	targetID, err := d.st.ResolvePointID(ctx, a.Supersedes)
	if err != nil {
		return "", "", err
	}
	owner := c.Subj.Owner()
	m := a.storeArgs.toMemory(owner, c.Actor, d.clock()) // if storeArgs embedded
	m.EmbedderIdentity = d.embedderIdentity
	m.Supersedes = &targetID
	vec, err := d.em.Embed(ctx, store.EmbedText(m.Content, m.Tags))
	if err != nil {
		return "", "", err
	}
	if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil {
		return "", "", err
	}
	if err := d.st.Supersede(ctx, m, vec, targetID, c.Subj); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Re-wrap with caller's ORIGINAL input (a.Supersedes), not the
			// resolved targetID — same 404-indistinguishability discipline as
			// setVisibility (tools.go:1213-1219) and storeDiscovery
			// (tools.go:852-858).
			return "", "", fmt.Errorf("%w: %s", store.ErrNotFound, a.Supersedes)
		}
		return "", "", err
	}
	d.summaryQueue.tryEnqueue(m.ID) // if summary-on-write applies to supersede too — Claude's Discretion
	return m.ID, m.ShortID, nil
}
```

```go
// Registration — mirrors set_visibility (tools.go:1423-1430) exactly.
mcp.AddTool(s, &mcp.Tool{Name: "supersede_memory", Description: "Correct a memory you own by superseding it: stores a new record and marks the target superseded_by the new one. The target is soft-hidden from search_memory/list_memory but remains fetchable via get_memory — history is preserved, nothing is deleted or overwritten. Rejects if the target is already superseded (single live head per chain). The target id may be the full UUID or short_id."},
	func(ctx context.Context, _ *mcp.CallToolRequest, a supersedeArgs) (*mcp.CallToolResult, any, error) {
		c, err := callerFromContext(ctx)
		if err != nil {
			return nil, nil, err
		}
		id, sid, err := d.supersedeMemory(ctx, c, a)
		return textResult(fmt.Sprintf("stored %s, superseding %s", id, a.Supersedes)), map[string]string{"id": id, "short_id": sid}, err
	})
```

### Pattern 1: Payload-only, vector-preserving back-stamp (D-01)
**What:** Mutate a single Qdrant payload key on an existing point via `SetPayload`, never
`Upsert`/`OverwritePayload` on an existing record.
**When to use:** Any mutation that changes ONE conceptual field on a record whose content/vector
must survive untouched, and where concurrent writers may be touching OTHER keys on the same point.
**Example (existing, verbatim template):**
```go
// Source: internal/store/store.go:1693 (SetVisibility) — same shape, target back-stamp
// swaps ActionShare->ActionWrite and the "visibility" key -> "superseded_by".
_, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
	CollectionName: s.collection, Wait: qdrant.PtrOf(true),
	Payload:        qdrant.NewValueMap(map[string]any{"visibility": vis}),
	PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
})
```

### Pattern 2: Recall-gate soft-hide via IsEmpty condition
**What:** Add a `NewIsEmpty(key)` disjunct/conjunct to the recall filter's `Must` slice so a
non-empty payload key excludes a record from `Search`/`List` without touching `Get`.
**When to use:** Any "soft delete"/"soft hide" semantics where the record must remain individually
fetchable.
**Example:**
```go
// Source: internal/store/store.go:727-741 (activeWindowConditions) — the existing
// not_before/not_after gate this phase's superseded_by gate sits alongside.
// New condition to append at BOTH call sites (store.go:791 Search, store.go:1010 List):
qdrant.NewIsEmpty("superseded_by")
```
Note `activeWindowConditions` uses a `Should`-wrapped-in-`Filter` idiom for the OR of
(range-match OR absent); `superseded_by` needs no OR — it is a simple `Must`-level
`NewIsEmpty` condition (no "supersede window," just presence/absence), so it can be appended
directly to `f.Must` alongside `activeWindowConditions(...)`'s output, not folded into that
helper's return slice (keep the helper's name/scope accurate — it is about the *time* window,
not supersession; add a sibling call, not a signature change).

### Pattern 3: Optional pointer payload field (D-08)
**What:** New optional Memory fields use `*T`, encoded only when non-nil, decoded defensively.
**Example:**
```go
// Source: internal/store/store.go:438-443 (payload()) and :525-532 (fromPayload())
// pattern for NotBefore/NotAfter — Supersedes/SupersededBy follow identically but with
// string keys, not epoch-second integers:
if m.NotBefore != nil {
	p["not_before"] = m.NotBefore.Unix()
}
// -->
if m.Supersedes != nil {
	p["supersedes"] = *m.Supersedes
}
if m.SupersededBy != nil {
	p["superseded_by"] = *m.SupersededBy
}
// decode side:
if v, ok := p["supersedes"]; ok {
	s := v.GetStringValue()
	m.Supersedes = &s
}
if v, ok := p["superseded_by"]; ok {
	s := v.GetStringValue()
	m.SupersededBy = &s
}
```
Add the two fields to the `Memory` struct (store.go, recommend placing near `NotBefore`/`NotAfter`
at ~line 162-167, both grouped as "recall-gate-affecting optional links") with **normal json
tags** (`json:"supersedes,omitempty"` / `json:"superseded_by,omitempty"`) — NOT `json:"-"`. This is
a deliberate divergence from `EmbedderIdentity`/`IdempotencyFingerprint` (which ARE `json:"-"` to
prevent an audit-only field leaking onto the wire): `supersedes`/`superseded_by` are exactly the
data the caller needs to see (SC1's whole point is that the caller can observe the link), so they
must cross the wire on `full=true` recall and `get_memory` — same visibility class as
`NotBefore`/`NotAfter`, which also use plain json tags.

### Recommended Project Structure
No new files/packages. All changes land in the four already-identified files:
```
internal/store/store.go        # Memory struct fields, payload()/fromPayload(), Store.Supersede,
                                # ErrAlreadySuperseded, activeWindowConditions call-site additions
internal/store/store_test.go   # TestSupersede* (owner gate, TOCTOU, single-hop, forward-chain)
internal/server/tools.go       # supersedeArgs, deps.supersedeMemory, mcp.AddTool registration
internal/server/connecterror.go # new case in the sentinel switch (see below — exhaustiveness,
                                # NOT because Connect exposes supersede this phase)
```

### Anti-Patterns to Avoid
- **Full re-Upsert of the target to set `superseded_by`:** Reintroduces the exact CR-01 lost-write
  hazard this repo has twice already documented and rejected (`store.go:1571-1580`,
  `tools.go:748-757`). Never do this for an existing record's payload-only field change.
- **Re-embedding the target on back-stamp:** The target's content/vector must not change.
  `SetPayload` never touches vectors (confirmed: `SetVisibility`/`IncrementAccess`/`UpdatePayload`
  all use it precisely because it is vector-preserving).
- **Using `GetReadable` instead of `getWritable` for the target gate:** `GetReadable` permits
  shared-but-not-owned records (SC3 would regress — DEC-kyz "sharing grants read, never write").
  Always `getWritable(target, subj, authz.ActionWrite)`.
- **Folding `superseded_by IS EMPTY` into `activeWindowConditions`:** That helper's name/doc
  promise is specifically about the temporal window; conflating supersession into it makes a
  future reader think superseded-ness is time-based. Add a sibling condition at each of the two
  call sites instead.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Owner-only write authorization | A parallel ad-hoc `if m.Owner != subj.Owner()` check | `getWritable(id, subj, authz.ActionWrite)` | Already Cedar-PDP-backed (Phase 22/23), already handles anonymous-vs-authenticated and TOCTOU-safe fail-closed `ErrNotFound` semantics uniformly across every mutation |
| Partial-payload merge write | A read-modify-write full `Upsert` (fetch, mutate one field in Go, write the whole struct back) | `qdrant.SetPayloadPoints` with only the changed key(s) | Read-modify-write races with ANY concurrent writer to a different key on the same point; `SetPayload` is a server-side merge, immune to that race |
| Soft-hide-but-keep-fetchable | A `deleted`/`archived` boolean + separate "trash" query path | The existing `IsEmpty(key)` recall-filter idiom already used for `not_before`/`not_after` | One well-tested filter-assembly shape covers scheduling AND supersession identically — no new query surface |

**Key insight:** Every piece of this phase already has a proven twin in the codebase
(`SetVisibility` for the mutation, `activeWindowConditions` for the gate, `getWritable` for the
authz, `NotBefore`/`NotAfter` for the optional-field codec). The engineering risk in this phase is
not inventing anything — it's correctly threading four existing patterns together without
introducing a fifth, novel one.

## Common Pitfalls

### Pitfall 1: Treating the two-step Supersede as atomic
**What goes wrong:** A crash/network failure between `Upsert(newMem)` and `SetPayload(target)`
leaves the new record created with `Supersedes` set, but the target's `SupersededBy` never gets
stamped — a forward link with no backward link. The target continues to appear in recall
(uncorrected from the caller's perspective) alongside the "correcting" record.
**Why it happens:** Qdrant has no multi-point transaction primitive this codebase uses anywhere
(confirmed: `SetVisibility`'s own doc comment explicitly accepts non-atomicity for its
single-record two-step SetPayload+DeletePayload sequence, store.go:1596-1598, and this phase's
Supersede has the SAME shape one level up — two separate point mutations).
**How to avoid:** Document this exactly like `SetVisibility` documents its own TOCTOU note — accept
it as a known, bounded failure mode rather than attempting a distributed-transaction fix that has
no precedent in this codebase. Order the two writes so the WORSE failure (an orphaned forward link
with no back-stamp) is the one that happens on error, not a "phantom superseded" target with no
correcting record — i.e., always create the new record FIRST, back-stamp the target SECOND (as
specified above). A caller can retry `supersede_memory`... but see Pitfall 2.
**Warning signs:** A record with non-empty `Supersedes` whose target has empty `SupersededBy`.

### Pitfall 2: Retrying a failed Supersede creates duplicate correcting records
**What goes wrong:** Unlike `store_memory`, `supersede_memory` per D-03/D-04 has no idempotency-key
story specified in CONTEXT.md. If step 4 (back-stamp) fails after step 3 (new-record create)
succeeds, a naive client retry re-runs the WHOLE tool call, creating a second new record.
**Why it happens:** `supersede_memory` composes a create + a mutation; only the create side can
reuse Phase 24's idempotency mechanism, and only if `supersedeArgs` embeds `storeArgs` (Claude's
Discretion per CONTEXT.md).
**How to avoid:** Recommend the planner make this Claude's-Discretion call explicit: embed
`storeArgs` on `supersedeArgs` so `idempotency_key` is available "for free" (same mechanism as
`schedule_memory`). This does not fully solve the two-step problem (idempotency only protects step
3, not step 4), but at minimum prevents duplicate correcting records on a naive retry after a
step-3-succeeded/step-4-failed partial failure. Flag as an Open Question for the planner: whether
step 4's own failure needs a distinct retry-safety story, or whether "the caller can call
`supersede_memory` again with a NEW idempotency key once they see the first attempt failed" is
sufficient (this second attempt would still succeed at step 4 since D-05's already-superseded
check would not yet have been satisfied — target's `SupersededBy` is still empty from the failed
attempt).
**Warning signs:** Multiple records with `Supersedes` pointing at the same target id.

### Pitfall 3: Forgetting the recall-gate addition at BOTH call sites
**What goes wrong:** `activeWindowConditions(s.now())` is called independently at `store.go:791`
(inside `Search`) and `store.go:1010` (inside `List`) — it is NOT a single shared filter object.
Adding `superseded_by IS EMPTY` only to `Search`'s call site would leave `list_memory` still
surfacing superseded records (SC2 partial regression).
**Why it happens:** The two call sites independently build `f.Must` appends; there is no single
"recall filter" constructor both funnel through.
**How to avoid:** Grep both `store.go:791` and `store.go:1010` (or `rg "activeWindowConditions" internal/store/store.go`) and add the new condition at both. Add a test asserting exclusion from
BOTH `Search` and `List` (see `TestListDateWindow`/`TestSearchDateWindow`, store_test.go:2441/2480,
as the existing dual-coverage precedent for `not_before`/`not_after` — mirror that pairing for
`superseded_by`).
**Warning signs:** A superseded record appears in `list_memory` results but not `search_memory`
(or vice versa) during manual verification.

### Pitfall 4: Missing the `connectError` sentinel-switch exhaustiveness convention
**What goes wrong:** `store.ErrAlreadySuperseded` is added to `store.go` but never added to
`internal/server/connecterror.go`'s switch. The switch's own doc comment
(`connecterror.go:16-44`) explicitly enumerates every known sentinel it maps — a reviewer expects
every new `store.Err*` to appear there, even ones the Connect lane cannot currently reach.
**Why it happens:** Easy to assume "Connect doesn't expose `supersede_memory` this phase, so no
Connect-side change is needed" — but Phase 24 sets a DIRECT precedent against that assumption:
`store.ErrIdempotencyConflict` IS mapped in `connectError` (to `CodeAlreadyExists`) with the
explicit comment "Pre-positioning only... kept here so the sentinel switch stays exhaustive"
(`connecterror.go:60-65`) even though idempotency_key is likewise structurally unreachable via
Connect.
**How to avoid:** Add a case for `store.ErrAlreadySuperseded` in `connectError`, mapped to
`connect.CodeFailedPrecondition` (matches the existing precondition-class mappings:
`errRuleImmutable`, `errStaleSummary`, `store.ErrAmbiguousShortID` — all "the record's current
state forbids this operation," the exact shape of "already superseded"). Update the function's doc
comment to list the new case, matching the existing enumeration discipline.
**Warning signs:** A future Connect `SupersedeMemory` RPC (if ever added) falls through to the
generic `CodeInternal` branch instead of a precise code.

## Runtime State Inventory

Not applicable — this is a greenfield feature phase (new optional payload keys, new method, new
tool), not a rename/refactor/migration phase. No existing data needs remapping: every pre-feature
record simply has an absent `supersedes`/`superseded_by` payload key, decoded as `nil` by the
defensive `fromPayload` pattern (identical to how every pre-`NotBefore`/`NotAfter` record already
behaves) — zero backfill required.

## Common Pitfalls
(see above)

## Code Examples

### `getWritable` — the write-gate primitive to reuse unchanged
```go
// Source: internal/store/store.go:1426-1439
func (s *Store) getWritable(ctx context.Context, id string, subj Subject, action authz.Action) (Memory, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return Memory{}, err
	}
	owner, kind, ok := principalParams(subj)
	if !ok {
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if !s.decideRecord(owner, kind, action, m.Owner, m.Category, m.Visibility, m.Scope).Allow {
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return m, nil
}
```

### `activeWindowConditions` — the recall-gate shape being extended
```go
// Source: internal/store/store.go:727-741
func activeWindowConditions(now time.Time) []*qdrant.Condition {
	sec := float64(now.Unix())
	return []*qdrant.Condition{
		qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
			qdrant.NewRange("not_before", &qdrant.Range{Lte: qdrant.PtrOf(sec)}),
			qdrant.NewIsEmpty("not_before"),
		}}),
		qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
			qdrant.NewRange("not_after", &qdrant.Range{Gt: qdrant.PtrOf(sec)}),
			qdrant.NewIsEmpty("not_after"),
		}}),
	}
}
// Call sites to extend (add qdrant.NewIsEmpty("superseded_by") to f.Must alongside):
//   Search: store.go:791  f.Must = append(f.Must, activeWindowConditions(s.now())...)
//   List:   store.go:1010 f.Must = append(f.Must, activeWindowConditions(s.now())...)
```

### `own_records.cedar` — confirms zero new authz policy needed
```
// Source: internal/authz/policies/own_records.cedar (verbatim, full file)
permit (
  principal,
  action,
  resource
)
when {
  resource.owner == principal.owner
};
```
This policy grants the owner **any** `action` — `ActionWrite` for the supersede back-stamp requires
no new Cedar rule; it is already covered by the existing "any action on owned resource" grant.

## State of the Art

Not applicable in the conventional sense (no library/framework version drift risk here — this is
pure internal composition). The one "state of the art" question CONTEXT.md raised was resolved:

| Old Approach (ROADMAP wording) | Current Approach (CONTEXT.md D-01, verified this session) | When Changed | Impact |
|--------------------------------|------------------------------------------------------------|---------------|--------|
| "reuses idempotency's payload-only re-Upsert mechanism" | `SetPayload` single-key merge (not a re-Upsert) | Refined during `/gsd-discuss-phase` context gathering, 2026-07-19 | Avoids the CR-01 lost-write class; ROADMAP wording was directionally correct ("payload-only") but imprecise on the primitive — `Upsert` in this codebase always replaces the WHOLE payload (confirmed: `Store.Upsert`, store.go:599-606, sends `qdrant.NewValueMap(payload(m))` — the full struct-derived map, every call) |

**Deprecated/outdated:** None — no code being removed or replaced by this phase.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `supersede_memory`'s error-message wording ("Correct a memory you own...") and exact tool description string are illustrative, not locked | Code Examples / Pattern registration | Low — cosmetic, planner/implementer can adjust freely; CONTEXT.md explicitly leaves tool naming to Claude's Discretion |
| A2 | `Connect.CodeFailedPrecondition` is the right mapping for `ErrAlreadySuperseded` | Pitfall 4 | Low-Medium — reasoned by analogy to `errRuleImmutable`/`errStaleSummary`/`ErrAmbiguousShortID`, all of which ARE precondition-class rejections on record state; `CodeAlreadyExists` (used for `ErrIdempotencyConflict`) is a plausible alternative if the planner judges "already superseded" closer to a duplicate-request semantics than a state-precondition. Recommend the planner make this call explicitly in the plan rather than deferring it. |
| A3 | Placing `Supersedes`/`SupersededBy` fields near `NotBefore`/`NotAfter` in the `Memory` struct (rather than near `Visibility` or at the end) is purely a readability recommendation | Pattern 3 | None — no functional impact, field order in a Go struct with named JSON tags is irrelevant to correctness |
| A4 | `supersedeArgs` should embed `storeArgs` (inheriting `idempotency_key`) rather than hand-roll a parallel field list | Pitfall 2, Code Examples | Medium — if the planner instead hand-rolls, duplicate-field drift risk reappears (the exact class of bug `tools.go:734-736`'s `persistAndEnqueue` comment says was already fixed once for `storeMemory`/`scheduleMemory`); recommend embedding, but this is explicitly Claude's Discretion per CONTEXT.md |

## Open Questions

1. **Does `supersede_memory` participate in async summary-on-write (`ENGRAM_SUMMARY_ON_WRITE`)?**
   - What we know: `persistAndEnqueue` (the shared `store_memory`/`schedule_memory` tail) calls
     `d.summaryQueue.tryEnqueue(m.ID)` after every successful create. `storeDiscovery`/`storeRule`
     deliberately do NOT call `persistAndEnqueue` because "discoveries and rules own their own
     summaries" (tools.go:741-742).
   - What's unclear: Is the new correcting record conceptually a `store_memory`-shaped write (so it
     SHOULD get async summarization) or is it closer to `storeDiscovery`'s "owns its own summary"
     carve-out because it already carries an explicit relationship (supersedes) the caller is
     presumably being deliberate about?
   - Recommendation: Treat it as `store_memory`-shaped (enqueue for summarization) — the new record
     IS a normal memory with an extra link, not a structurally distinct category like `discovery`/
     `rule`. Planner should confirm this in the plan rather than silently deciding either way.

2. **Should `toRecallView` (the compact summary shape returned by default from `search_memory`/
   `list_memory`) surface `Supersedes`?**
   - What we know: `toRecallView` (`internal/server/summary.go:96-105`) does NOT include
     `NotBefore`/`NotAfter` even though those are plain (non-`json:"-"`) `Memory` fields — the
     compact view is a deliberate allowlist, not "everything except explicitly excluded."
     `SupersededBy` never needs to appear here (a superseded record is excluded from recall
     entirely by construction). `Supersedes` on a LIVE (non-superseded) record, however, WOULD be
     useful recall-time context ("this memory corrects an earlier one") without requiring a
     separate `get_memory` call.
   - What's unclear: Whether this is in scope for Phase 25 (SC1/SC2 only require stamping + hiding
     + fetchability, not compact-view surfacing) or a nice-to-have deferred to a later UX pass.
   - Recommendation: Out of scope for SC1-SC4 as written; leave `toRecallView` untouched (matches
     `NotBefore`/`NotAfter` precedent) unless the planner/user wants it — flag as a possible
     follow-up, not a blocker.

3. **`Store.Supersede`'s exact parameter shape (`newMem, vec, target, subj` vs. a request struct)**
   - What we know: Comparable existing methods (`Update`, `SetVisibility`, `Upsert`) use plain
     positional parameters, not a request struct — this codebase has no `XxxRequest` struct
     convention at the `internal/store` layer (that pattern is proto-generated and lives only in
     `internal/server`/`gen/`).
   - What's unclear: Nothing meaningfully unclear; flagging only so the planner doesn't invent a
     new store-layer convention unnecessarily.
   - Recommendation: Match `Update`'s positional-parameter shape, as sketched in Code Examples
     above.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Qdrant (testcontainer or `ENGRAM_QDRANT_TEST_ADDR`) | `internal/store` test suite (`testStore(t)`) | ✓ (existing test infra) | pinned `qdrantImageTag` / `qdrantTOCTOUVerifiedVersion` in store_test.go | none needed — already required by every existing store test |
| go-client v1.18.3 | `SetPayload`/`NewIsEmpty`/`Upsert` | ✓ | v1.18.3 (go.mod:18) `[VERIFIED: go.mod]` | none needed |

No missing dependencies — this phase requires nothing beyond what every prior store-layer phase
already needed.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (+ `testify`-free assertions, matches repo convention) |
| Config file | none — `go test ./...` via `Taskfile.yaml:34-36` (`task test:go`) |
| Quick run command | `go test ./internal/store/... ./internal/server/... -run TestSupersede` |
| Full suite command | `task test` (runs `go test ./...` + python skill-hook tests) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-supersession-links (SC1) | Storing with `supersedes` stamps target's `superseded_by`, content/vector untouched | unit (store) | `go test ./internal/store/... -run TestSupersedeStamp -v` | ❌ Wave 0 — new test |
| REQ-supersession-links (SC2) | Superseded record excluded from `Search`/`List`, still fetchable via `Get` | unit (store) | `go test ./internal/store/... -run TestSupersedeRecallGate -v` | ❌ Wave 0 — new test (mirror `TestListDateWindow`/`TestSearchDateWindow` pairing, store_test.go:2441/2480) |
| REQ-supersession-links (SC3) | Non-owner (shared/read-only) caller cannot supersede | unit (store) | `go test ./internal/store/... -run TestSupersedeOwnerGate -v` | ❌ Wave 0 — new test (mirror `TestSetVisibilityOwnerGate`, store_test.go:797) |
| REQ-supersession-links (SC4) | Superseding an already-superseded target rejected; no auto-supersede path exists | unit (store) | `go test ./internal/store/... -run TestSupersedeAlreadySuperseded -v` | ❌ Wave 0 — new test |
| REQ-supersession-links (D-02 TOCTOU) | Target deleted between gate and SetPayload fails closed | unit (store, testcontainer-gated) | `go test ./internal/store/... -run TestSupersedeTOCTOU -v` | ❌ Wave 0 — new test (mirror `TestSetVisibilityTOCTOU`, store_test.go:848, including the `qdrantTOCTOUVerifiedVersion` guard) |
| REQ-supersession-links (D-03 tool surface) | `supersede_memory` MCP tool registered, wired through authz | unit (server) | `go test ./internal/server/... -run TestSupersedeMemory -v` | ❌ Wave 0 — new test |
| REQ-supersession-links (D-06 forward chains) | C supersedes A which superseded B — chain fetchable, only B/A hidden | unit (store) | `go test ./internal/store/... -run TestSupersedeForwardChain -v` | ❌ Wave 0 — new test |

### Sampling Rate
- **Per task commit:** `go test ./internal/store/... ./internal/server/... -run TestSupersede`
- **Per wave merge:** `task test`
- **Phase gate:** `task test` (lint+test via `task`) green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/store/store_test.go` — add the 6 `TestSupersede*` cases above (no new file needed;
      this file already hosts `TestSetVisibility*`/`TestListDateWindow`/`TestSearchDateWindow` —
      follow the existing single-file convention, do not split into a new `supersede_test.go`
      unless the planner judges the file has grown too large)
- [ ] `internal/server/tools_test.go` (or equivalent) — `TestSupersedeMemory` handler-level test
      mirroring `set_visibility`'s handler test coverage
- [ ] No framework install needed — `go test` is already wired

*(Existing test infrastructure — `testStore(t)`, the testcontainer/`ENGRAM_QDRANT_TEST_ADDR`
toggle, `Authenticated(sub)`/`anonymous{}` Subject fixtures — covers everything this phase needs;
only new test CASES are required, not new test INFRASTRUCTURE.)*

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no (unchanged) | Existing OIDC bearer-token verification (`internal/auth`) — this phase adds no new auth surface |
| V3 Session Management | no (unchanged) | N/A — MCP tool call, not session-bearing |
| V4 Access Control | **yes** | `getWritable`/`OwnedOrAbsent` owner-only gate (D-01/SC3) via Cedar `own_records.cedar` — reused unchanged, zero new policy |
| V5 Input Validation | yes | `ResolvePointID` (rejects malformed/empty ids), single-hop rejection (`ErrAlreadySuperseded`) as a state-precondition validator |
| V6 Cryptography | no | N/A — no crypto surface in this phase |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Cross-owner supersede (attacker supersedes a victim's record they can only READ via `shared`) | Elevation of Privilege | `getWritable(target, subj, ActionWrite)` — shared visibility grants `ActionRead` only per `own_records.cedar`; DEC-kyz explicitly forbids write via shared-read (SC3, this phase's own success criterion) |
| Existence-leak via error message on a nonexistent/not-owned target | Information Disclosure | `getWritable` returns the SAME `ErrNotFound` for "doesn't exist" and "exists but not yours" (store.go:1413-1424 doc comment) — reused unchanged; handler-level re-wrap with the caller's ORIGINAL (unresolved) input id, matching `setVisibility`'s and `storeDiscovery`'s existing 404-indistinguishability discipline |
| Supersede-chain cycle (A supersedes B supersedes A) | Tampering (data-integrity) | D-05's single-hop rejection: superseding an already-superseded record (`SupersededBy != nil`) is rejected — since a fresh record cannot itself already be superseded at create time, and the target check runs before the new record is persisted, no cycle can ever form |
| TOCTOU delete-during-supersede | Denial of Service / data-integrity (minor) | Fail-closed: `SetPayload` on a deleted point-ID returns `NotFound`, propagated unchanged (identical to `SetVisibility`'s proven-and-tested TOCTOU contract, store_test.go:848) |

## Sources

### Primary (HIGH confidence)
- `internal/store/store.go` (this repo, read directly) — `SetVisibility` (1693), `getWritable`
  (1426), `activeWindowConditions` (727), `Memory` struct (136-235), `payload`/`fromPayload`
  (418-583), `Upsert` (586), `Update`/`UpdatePayload` doc comments (1514-1598), error sentinels
  (62-90)
- `internal/server/tools.go` (this repo, read directly) — tool registration block (1245-1430),
  `checkIdempotentReplay`/`persistAndEnqueue` (661-775), `storeArgs`/`scheduleArgs`/`idArgs`
  (424-516), `setVisibility` handler (1201-1232), `storeDiscovery` handler (832+)
- `internal/server/connecterror.go` (this repo, read directly) — the single sentinel-to-Connect-code
  mapper, its exhaustiveness convention and doc comment
- `internal/authz/policies/own_records.cedar` (this repo, read directly) — confirms zero new
  policy needed
- `internal/authz/authz.go` (this repo, read directly) — confirms `ActionWrite` already exists
- `internal/store/store_test.go` (this repo, read directly) — `TestSetVisibilityOwnerGate` (797),
  `TestSetVisibilityTOCTOU` (848), date-window test pairing (`TestListDateWindow`/
  `TestSearchDateWindow`, 2441/2480)
- Qdrant official docs, `/websites/qdrant_tech` via Context7 — "Set Payload" (partial update,
  preserves other keys) vs "Overwrite Payload" (full replace) vs `PUT /collections/{name}/points`
  Upsert semantics ("the entire point is replaced") — confirms D-01's SetPayload-vs-Upsert
  distinction at the protocol level, independent of this repo's own code
- `go.mod:18` — `github.com/qdrant/go-client v1.18.3` pinned version, matches what `store.go`
  already imports and exercises

### Secondary (MEDIUM confidence)
None required — every claim in this document traced to either a first-party file read in this
session or an official Qdrant documentation fetch via Context7.

### Tertiary (LOW confidence)
None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies; 100% reuse of already-shipped, already-tested
  internal primitives
- Architecture: HIGH — every pattern (payload-only mutation, recall-gate IsEmpty condition,
  owner-write gate, optional-pointer codec) has a verbatim precedent read directly from this
  repo's current `store.go`/`tools.go`
- Pitfalls: HIGH — Pitfalls 1-3 are derived from this repo's own doc comments describing
  identical hazards in adjacent code (`SetVisibility`'s TOCTOU note, `UpdatePayload`'s lost-write
  warning, `persistAndEnqueue`'s CR-01 comment); Pitfall 4 is derived from a directly-observed
  precedent (`ErrIdempotencyConflict`'s "pre-positioning only" comment in `connecterror.go`)

**Research date:** 2026-07-19
**Valid until:** No external dependency drift risk (no new packages) — this research is valid
until the underlying `internal/store`/`internal/server` code it cites is itself refactored;
recommend re-verifying file:line references only if `store.go`/`tools.go` have materially changed
since 2026-07-19.
