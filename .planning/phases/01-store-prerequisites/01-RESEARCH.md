# Phase 1: Store Prerequisites - Research

**Researched:** 2026-09-25
**Domain:** Go store-layer authz (Cedar via `internal/authz`) + Qdrant query composition (`internal/store`, go-client v1.19.2)
**Confidence:** HIGH

## Summary

This phase adds three pure `internal/store` methods — an authz-gated Archive/Restore pair,
`RelatedMemories`, and `ListTags` — with zero proto/RPC/UI surface. Every pattern needed already
exists in the codebase and was read directly this session: the owner-write gate (`getWritable`),
the recall-gate filter composition (`ownerScopeFilter` + `activeWindowConditions` +
`IsEmpty(superseded_by)` + `IsEmpty(archived_at)`), the query-by-id sub-query precedent
(`NearDuplicates`), the two-phase fetch that makes authz enforcement structural rather than a
post-filter (`fetchPayloadsByID`), and a Facet-with-filter precedent that deliberately has no
filter yet (`MigrateStatus`). Nothing here requires inventing a new Qdrant idiom.

The one genuine risk this research surfaces beyond what CONTEXT.md already names: the recall-gate
test file (`internal/store/schemaversion_recallgate_test.go`) enforces its "no gap" guarantee
through **three independent, differently-scoped lists**, not one. CONTEXT.md's D-15 names only the
dynamic interceptor's allowlist (`recognizedFilterCarryingRequestMethods`) and its type switch. A
**second**, purely static list (`recallEmissionMethods`, the AST-scan vocabulary) and a **third**
(`recallEntryPointSeeds`, the caller-facing entry-point roster that `ListTags` should join) also
gate on the exact method name `"Facet"` and the exact enclosing-function name `"Store.ListTags"`.
Widening only the two D-15 names will compile and pass CI green — but only because the static
completeness scan (`TestRecallEmissionSetIsCompleteAndClassified`) never notices the new call site
at all, which is precisely the silent-blind-spot failure mode that test file's own doc comment
warns about. See Common Pitfalls below for the exact three edits.

**Primary recommendation:** implement `ArchiveAs`/`RestoreAs` (or planner-chosen names) as thin
`getWritable(ctx, id, subj, authz.ActionArchive)`-gated wrappers that, once authorized, call the
existing unlocked-write internals `Archive`/`Restore` already use — never duplicate the lock/resolve
logic. Implement `RelatedMemories` as one method building one shared authz filter (`ownerScopeFilter`
+ recall-gate conditions) and reusing it across all three query-based edge types (vector, tag,
citation), with the supersession-chain edge fetched separately via `GetReadable` per id (never
filtered — chain members are soft-hidden by design). Implement `ListTags` as a filtered
`s.client.Facet` call modeled on `MigrateStatus` but WITH the authz filter `MigrateStatus`
deliberately omits.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Archive/Restore authz gate | Database/Storage (`internal/store`) | — | DEC-cgb: all per-actor authorization is enforced inside `internal/store` via Qdrant filters/owner gates, never in a handler — this phase adds no handler at all |
| Cedar action vocabulary | Database/Storage (`internal/authz`, consumed only by `internal/store`) | — | `internal/authz` is a library the store calls; it has no independent tier of its own in this app's shape |
| RelatedMemories edge composition | Database/Storage (`internal/store`) | — | Every sub-query's filter must carry the caller's read predicate; composing it anywhere else (a future RPC handler) would be the exact anti-pattern ARCHITECTURE.md names |
| ListTags facet counts | Database/Storage (`internal/store`) | — | Same reasoning: Facet's `Filter` field is the enforcement point, and it must be built by the same code that builds every other recall filter |
| Payload index (`tags`) | Database/Storage (`internal/store.ensureIndexes`) | — | Idempotent boot-time index creation, the existing mechanism for every prior payload index |
| Recall-gate test allowlists | Database/Storage (test-only, `internal/store` package) | — | The three lists this phase must widen all live inside `internal/store`'s own test files, not a separate test-infra package |

No Browser/Client, Frontend Server, CDN, or API/Backend (Connect/MCP) tier is touched this phase —
Phase 3 owns wrapping these store methods in RPCs/tools.

## User Constraints (from CONTEXT.md)

<user_constraints>

### Locked Decisions

**Phase Boundary:** `internal/store` gains three pure store methods — an authz-gated Archive/Restore
path, `RelatedMemories`, and `ListTags` — plus the test infrastructure they need (the `tags` payload
index, the recall-gate allowlist widened for a filtered `Facet`). No proto, no Connect RPC, no MCP
tool, no UI: those are Phase 3 onward. Requirements: STORE-01, STORE-02, STORE-03.

**Archive / Restore authz (STORE-01)**
- **D-01:** Add one new Cedar action, `ActionArchive` (`"archive"`), used by both the gated archive
  and the gated restore. Do not reuse `ActionWrite`; do not split archive and restore into two
  actions. `own_records.cedar` already permits every action for the owner, and `shared_read.cedar`
  permits only `read`, so no policy change is needed — but the action must be added to
  `internal/authz/schema.json` and the `allActions`/write-action lists in
  `internal/authz/policy_corpus_test.go`. — **Reversibility:** costly — once Phase 3 exposes it, the
  action name is part of the policy vocabulary operators may write policies against.
- **D-02:** Method shape: new gated wrappers (e.g. `ArchiveAs(ctx, id, subj)` /
  `RestoreAs(ctx, id, subj)` — planner names them) that run `getWritable(ctx, id, subj,
  authz.ActionArchive)` INSIDE the per-id `s.locker.Lock`, then share one core with the existing
  subject-less `Archive`/`Restore`. The existing `Archive(ctx, id)` / `Restore(ctx, id)` signatures
  and behaviour stay unchanged so the CLI `spine-review archive/restore` path keeps working (SC2).
  Never introduce a nil-Subject-means-operator sentinel — nil Subject stays fail-closed.
- **D-03:** The gated path ALLOWS the owner to archive and restore a rule (`category == "rule"`).
  Archive is reversible and owner-only, so it is permitted — unlike supersede and `set_visibility`,
  which reject rules. (Whether the Phase 4 Rules view exposes an archive affordance is a UI decision
  for that phase; CUR-03 currently says delete only.)
- **D-04:** A gated archive/restore of a record the caller can read but does not own (a shared
  record) returns `getWritable`'s `ErrNotFound`, indistinguishable from a nonexistent id —
  exactly as Delete/Update/Supersede reject it (SC1). `ArchiveResult.Outcome` is `NotFound`.
- **D-05:** The authz-in-store test (owner can, shared-reader cannot, anonymous vs owned records)
  is the phase's FIRST test, per SC1.

**RelatedMemories — edge semantics (STORE-02)**
- **D-06:** One result entry per candidate record, carrying the list of every edge type that
  reached it plus per-type evidence: cosine score for a vector edge, the shared tags (and their
  weight) for a tag edge, the shared citation refs for a citation edge, direction/depth for a
  supersession edge. This is the documented rule for a candidate reachable by more than one edge
  type. A candidate is never returned twice.
- **D-07:** Shared-tag edges are rarity-weighted: each shared tag contributes by inverse frequency,
  computed from the same `tags` facet `ListTags` uses, and only the top-N tag neighbours are kept.
  A ubiquitous tag (e.g. `engram`) contributes almost nothing; a rare one (e.g. `gh-618`) is a
  strong edge.
- **D-08:** A shared citation means the same citation `kind` + `ref` (file / URL / commit),
  regardless of `locator`, `pin`, or `excerpt`.
- **D-09:** Supersession edges walk the full chain in both directions — `superseded_by` forward to
  the live head, `supersedes` backward through predecessors — under a depth cap.

**RelatedMemories — reach and bounds (STORE-02)**
- **D-10:** Neighbours come from every scope the caller can read (cross-spine reach), not only the
  anchor's scope; each entry carries the neighbour's scope. The caller's read predicate is composed
  into every Qdrant sub-query's filter and the final payload fetch — never post-filtered.
- **D-11:** Supersession-chain members appear even when soft-hidden (superseded is what they are).
  Vector, tag, and citation edges follow the recall gate: archived, superseded, expired, and
  not-yet-active scheduled records are excluded from those edge types.
- **D-12:** Bounds: each edge type has its own small default cap (planner picks; roughly 8), the
  caller's `k` adjusts only the vector cap and is subject to the shared maximum of 1000
  (`rejectOverMaximum`), the supersession walk has a depth cap, and the merged result has a hard
  total ceiling. Scores are not comparable across edge types, so no single type may crowd out the
  others.

**ListTags (STORE-03)**
- **D-13:** Counts cover recall-visible records only — the same gate search/list apply (no
  archived, superseded, expired, or not-yet-active scheduled records) under the caller's read
  filter, so a tag showing count N yields N results when used as a filter.
- **D-14:** An empty scope means all readable scopes, reusing the cross-spine read filter.
- **D-15:** Exact counts (`FacetCounts.Exact = true`), ordered by count descending, with a limit
  (planner picks; at most the shared 1000 maximum) and a `more bool` truncation signal shaped like
  `ListScopes`'s — never a silent cut. Unlike `MigrateStatus`'s Facet, this one carries a Filter, so
  `recognizedFilterCarryingRequestMethods` and the interceptor type switch must recognise `Facet`.
- **D-16:** A `tags` keyword payload index is added in `ensureIndexes`.

### Claude's Discretion
- Exact method names, result struct names, and field names for all three methods.
- Numeric defaults: per-type caps, total ceiling, supersession depth cap, ListTags limit.
- Rarity-weighting formula (IDF-style) and the population it is computed over (the caller's
  readable, recall-visible set is the natural choice).
- How the `tags` index reaches existing collections (the idempotent `ensureIndexes` on startup is
  the existing mechanism).
- Behaviour when the anchor itself is soft-hidden (`Get` is not recall-gated, so a readable hidden
  anchor should still work) and when the anchor is unreadable (`ErrNotFound`).

### Deferred Ideas (OUT OF SCOPE)
- Edge labels from the consolidate verdict (same fact / contradicts / similar) — already GRAPH-04,
  future milestone.
- "Security-review then install the three design/a11y registry skills" (Reviewed Todo, not
  folded — belongs to DSYS-03, Phase 4).

</user_constraints>

## Phase Requirements

<phase_requirements>

| ID | Description | Research Support |
|----|-------------|------------------|
| STORE-01 | `internal/store` gains authz-gated `Archive`/`Restore` paths that pass through the same owner-write gate as `Delete`/`Update`/`Supersede`, while the CLI's subject-less path keeps working unchanged | `getWritable` gate confirmed byte-for-byte (store.go:2195-2208); `Archive`/`Restore` core confirmed (spine.go:773-876); CLI caller confirmed unchanged-signature dependency (spine_review_archive.go:211,242); Cedar action addition path confirmed functionally schema-free (entities.go:79-81, authz.go:22-29) |
| STORE-02 | `internal/store` gains `RelatedMemories(subj, id)` returning typed edges — supersession chain, shared tags, shared citations, vector neighbours — with the caller's read predicate composed into the Qdrant filter, never post-filtered, bounded, with a documented multi-edge-type rule | `NearDuplicates`' `NewQueryID` sub-query pattern confirmed (spine.go:632-644); `ownerScopeFilter`/recall-gate composition confirmed (store.go:1038-1045,1223-1249); `fetchPayloadsByID` enforcement point confirmed (searchfetch.go:73-124); `qdrant.NewNestedFilter` confirmed available for citation matching (go-client conditions.go:253-262); Citation struct fields confirmed (store.go:398-404) |
| STORE-03 | `internal/store` gains `ListTags(subj, scope)` facet counts over a new `tags` payload index under the caller's read filter, with the recall-gate test allowlist widened for a filtered `Facet`, bounded | `ensureIndexes` idxs slice confirmed missing `tags` (store.go:657-687); `qdrant.FacetCounts.Filter` field and `GetFilter()` confirmed present (go-client points.pb.go:7899-7973); `MigrateStatus`'s unfiltered precedent confirmed (migrate_status.go:143-148); the THREE recall-gate lists needing widening confirmed by direct read (schemaversion_recallgate_test.go:868-872,893-903,334-340,355-362) — this is a finding beyond CONTEXT.md's own D-15 wording |

</phase_requirements>

## Standard Stack

No new external dependency is needed. This phase is pure `internal/store`/`internal/authz` Go code
against libraries already vendored and already in production use.

### Core (already in go.mod — no version change)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/qdrant/go-client` | v1.19.2 [VERIFIED: go.mod] | Qdrant gRPC client — `Facet`, `Query`/`QueryBatch`, filter/condition builders | Already the sole Qdrant client in the codebase; this phase adds new call shapes (`Facet` with a filter, an authz-composed `NewQueryID` sub-query), not a new client |
| `github.com/cedar-policy/cedar-go` | v1.8.0 [VERIFIED: go.mod] | Policy decision point behind `internal/authz.PDP` | Already the sole authz engine; adding an action is a one-line const addition, no schema/engine change (action entities are built dynamically — `entities.go:79-81` — schema.json is documentation only, not parsed at runtime per its own header comment) |

### Package Legitimacy Audit

**No new external packages are introduced by this phase.** The milestone brief itself states "zero
new Go dependencies expected" for the whole milestone (`.planning/REQUIREMENTS.md` research-basis
note), and this phase's three methods are built entirely from `internal/store`'s and
`internal/authz`'s existing primitives. The Package Legitimacy Gate protocol is not applicable —
skip the audit table.

## Architecture Patterns

### System Architecture Diagram

```
                     internal/authz (Cedar PDP, unchanged engine)
                              │
                 DecideRecord(owner, kind, action, memoryOwner, category, visibility, scope)
                              │ Allow/Deny
                              ▼
   ┌─────────────────────────────────────────────────────────────────────────┐
   │ internal/store (this phase's whole surface — no handler, no RPC)        │
   │                                                                          │
   │  ArchiveAs(ctx,id,subj) ──┐                                             │
   │  RestoreAs(ctx,id,subj) ──┤─▶ s.locker.Lock(id) ─▶ getWritable(         │
   │                            │      ctx,id,subj,authz.ActionArchive)      │
   │                            │      Allow ─▶ shared core ─▶ SetPayload/   │
   │                            │                              DeletePayload │
   │                            │      Deny  ─▶ ErrNotFound (D-04)           │
   │                            └─▶ unlock()                                 │
   │                                                                          │
   │  RelatedMemories(ctx,id,subj) ─▶ GetReadable(anchor) [ErrNotFound if    │
   │      │                            caller cannot even read the anchor]   │
   │      ├─▶ f := ownerScopeFilter(ctx,"",subj) + recall-gate conditions    │
   │      │        (activeWindowConditions, IsEmpty(superseded_by),         │
   │      │         IsEmpty(archived_at)) — ONE filter, reused below         │
   │      ├─▶ vector edge:  QueryPoints{Query: NewQueryID(id), Filter: f}    │
   │      ├─▶ tag edge:     Scroll/Query{Filter: f + NewMatch("tags",t)…}    │
   │      ├─▶ citation edge:Scroll/Query{Filter: f + NewNestedFilter(       │
   │      │                     "citations", kind==k AND ref==r)…}           │
   │      ├─▶ supersession edge: GetReadable per id in Supersedes/           │
   │      │        SupersededBy — NOT filtered by f (D-11: soft-hidden       │
   │      │        chain members must still appear)                         │
   │      └─▶ union ids ─▶ fetchPayloadsByID(ctx, f, view, ids)  ◀── the     │
   │              ONE enforcement point: an id the caller can't read is      │
   │              never fetched, so there is nothing to post-filter          │
   │                                                                          │
   │  ListTags(ctx,subj,scope) ─▶ f := ownerScopeFilter(ctx,scope,subj) +    │
   │      recall-gate conditions ─▶ s.client.Facet(FacetCounts{Key:"tags",   │
   │      Filter: f, Exact:true, Limit:N+1}) ─▶ sort desc, more = len>N      │
   └─────────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                     Qdrant (single collection, `tags` keyword index added
                     idempotently by ensureIndexes at boot)
```

Phase 3 (out of scope here) wraps each of these three methods in a Connect RPC / MCP tool; no line
in this diagram crosses into `internal/server`.

### Recommended file layout for new code

No new package. Follow the existing convention of grouping by concern within `internal/store`:

```
internal/store/
├── spine.go            # Archive/Restore core already lives here — add ArchiveAs/RestoreAs
│                        # beside Archive/Restore, sharing the core (D-02)
├── store.go             # ensureIndexes gets the new tags index (D-16); getWritable/
│                        # ownerScopeFilter are consumed, not modified
├── relatedmemories.go   # NEW FILE (suggested) — RelatedMemories + its edge-building
│                        # helpers; keeps store.go/spine.go from growing further
├── listtags.go          # NEW FILE (suggested) — ListTags + its FacetHit decoding
internal/authz/
├── authz.go             # add ActionArchive to the Action const block (authz.go:22-29)
├── schema.json           # add "archive" to the actions map (doc-only, not CI-gated)
└── policy_corpus_test.go # widen allActions (line 14) AND the inline shared-read-deny
                          # action list (line 38) to include ActionArchive
```

`store_test.go` is already 7500+ lines; consider new test files
(`archive_authz_test.go`, `relatedmemories_test.go`, `listtags_test.go`) rather than growing it
further — this is a suggestion, not a locked decision (Claude's Discretion covers file
organization implicitly).

### Pattern 1: Gated wrapper shares an unlocked core with the subject-less method (D-02)

**What:** `Delete`, `Update` (via `FetchForUpdate`), and `SetVisibility` all call `getWritable`
first, then perform the mutation. Archive/Restore currently have NO subject parameter at all —
they are Subject-less operator-tier methods, matching `reindex`/`migrate-remap-owner`/
`prune-expired`. The new gated methods must add the `getWritable` gate INSIDE the same
`s.locker.Lock` window Archive/Restore already take, then perform the identical write.

**Why the lock ordering matters:** Archive/Restore's own doc comment (verified below) explains
that a lock-free write can land inside `Update`'s in-lock re-read/Upsert window and be silently
erased — the exact CR-04 failure class. The gated wrapper must not introduce a second, unlocked
window between its own `getWritable` check and the write.

**Verified existing code (spine.go:773-876, read this session):**
```go
// [VERIFIED: internal/store/spine.go:773-813]
func (s *Store) Archive(ctx context.Context, id string) (res ArchiveResult, err error) {
	ctx, span := tracer.Start(ctx, "store.Archive",
		trace.WithAttributes(attribute.String("engram.id", id)))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "Archive", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.String("engram.archive.outcome", string(res.Outcome)))
		}
	}()

	unlock, lerr := s.locker.Lock(ctx, id)
	if lerr != nil {
		return ArchiveResult{ID: id}, lerr
	}
	defer unlock()

	cur, gerr := s.Get(ctx, id)
	if gerr != nil {
		if errors.Is(gerr, ErrNotFound) {
			return ArchiveResult{ID: id, Outcome: ArchiveOutcomeNotFound}, gerr
		}
		return ArchiveResult{ID: id}, gerr
	}
	if cur.ArchivedAt != nil {
		return ArchiveResult{ID: id, Outcome: ArchiveOutcomeAlready}, nil
	}

	now := s.now()
	if _, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"archived_at": now.Unix()}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	}); err != nil {
		return ArchiveResult{ID: id}, err
	}
	return ArchiveResult{ID: id, Outcome: ArchiveOutcomeChanged}, nil
}
```

The gated wrapper's shape (planner names it; sketch only, not verbatim source):
```go
// SKETCH — not verified source, illustrates D-02's required shape only.
func (s *Store) ArchiveAs(ctx context.Context, id string, subj Subject) (res ArchiveResult, err error) {
	unlock, lerr := s.locker.Lock(ctx, id)
	if lerr != nil {
		return ArchiveResult{ID: id}, lerr
	}
	defer unlock()

	if _, err := s.getWritable(ctx, id, subj, authz.ActionArchive); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ArchiveResult{ID: id, Outcome: ArchiveOutcomeNotFound}, err
		}
		return ArchiveResult{ID: id}, err
	}
	// ... identical archived-at-already-set check and SetPayload as Archive above,
	// factored into a shared unexported core both Archive and ArchiveAs call.
}
```

**Verified `getWritable` gate (store.go:2195-2208, read this session):**
```go
// [VERIFIED: internal/store/store.go:2195-2208]
func (s *Store) getWritable(ctx context.Context, id string, subj Subject, action authz.Action) (Memory, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return Memory{}, err
	}
	owner, kind, ok := principalParams(subj)
	if !ok {
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if !s.decideRecord(ctx, owner, kind, action, m.Owner, m.Category, m.Visibility, m.Scope).Allow {
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return m, nil
}
```

`getWritable` calls `s.Get(ctx, id)` internally — calling it a second time after the shared core's
own `Get` would be a redundant Qdrant round-trip inside the lock. The planner should decide whether
the gated wrapper's shared core takes the already-fetched `Memory` from `getWritable` (mirroring how
`Update` takes `cur Memory` from `FetchForUpdate` rather than re-fetching) or accepts one extra
`Get` for simplicity — this is an implementation-detail tradeoff, not a locked decision.

### Pattern 2: Adding a Cedar action costs one const line, not a schema/engine change

**Verified (authz.go:22-29, read this session):**
```go
// [VERIFIED: internal/authz/authz.go:22-29]
// The five engram authorization actions.
const (
	ActionRead     Action = "read"
	ActionWrite    Action = "write"
	ActionDelete   Action = "delete"
	ActionShare    Action = "share"
	ActionSchedule Action = "schedule"
)
```

**Verified: action entities are built dynamically, never schema-validated at runtime
(entities.go:79-81):**
```go
// [VERIFIED: internal/authz/entities.go:79-81]
func actionEntityUID(action Action) cedar.EntityUID {
	return cedar.NewEntityUID(actionEntityType, cedar.String(action))
}
```

`schema.json`'s own header comment (verified, quoted in full): `"REFERENCE-ONLY Cedar JSON schema
(D-06). ... it is NOT parsed at runtime and NOT CI-gated ... The hand-authored Go entity structs in
entities.go are the actual runtime source of truth."` [VERIFIED: internal/authz/schema.json:2]
— so adding `"archive"` there is documentation hygiene per D-01, not a functional requirement.

**Verified: the two Cedar policy files that decide the new action's behavior need NO edits**
(own_records.cedar, read in full):
```cedar
// [VERIFIED: internal/authz/policies/own_records.cedar:1-20, full file]
// own-records: the principal may perform any action on a resource it owns.
permit (
  principal,
  action,
  resource
)
when {
  resource.owner == principal.owner
};
```
```cedar
// [VERIFIED: internal/authz/policies/shared_read.cedar:1-14, full file]
// shared-read: a shared record is readable by any authenticated (non-empty
// owner) caller. READ ONLY (DEC-kyz).
permit (
  principal,
  action == Action::"read",
  resource
)
when {
  resource.visibility == "shared" &&
  principal.owner != ""
};
```
`own_records.cedar` matches `action` unconstrained (any action, including the new one, for the
owner) and `shared_read.cedar` matches only `Action::"read"` — `ActionArchive` therefore falls
through to Deny for a shared, non-owned record exactly like `write`/`delete`/`share`/`schedule` do
today. This is the code-level confirmation behind D-01's "no policy change is needed."

**What DOES need to change (three test-file edits, all in `internal/authz/policy_corpus_test.go`,
read in full this session):**
```go
// [VERIFIED: internal/authz/policy_corpus_test.go:14] — add ActionArchive here
var allActions = []Action{ActionRead, ActionWrite, ActionDelete, ActionShare, ActionSchedule}
```
```go
// [VERIFIED: internal/authz/policy_corpus_test.go:38] — this is CONTEXT.md D-01's
// "write-action list": the inline non-read action slice inside
// TestPolicyCorpus_SharedReadOnly that proves a shared record denies every
// action except read. ActionArchive must join it so SC1 (shared record can't
// be archived) has a permanent regression backstop at the policy-corpus layer,
// not just a store-level test.
for _, action := range []Action{ActionWrite, ActionDelete, ActionShare, ActionSchedule} {
	if got := pdp.DecideRecord("alice", "human", action, "bob", "note", "shared", ""); got.Allow {
		t.Fatalf("DecideRecord(owner=alice, action=%s, resource=bob/shared) = Allow, want Deny (DEC-kyz)", action)
	}
}
```

### Pattern 3: RelatedMemories reuses NearDuplicates' sub-query shape, adds the authz filter it lacks

**Verified (spine.go:632-644, read this session — NearDuplicates' per-id QueryPoints construction):**
```go
// [VERIFIED: internal/store/spine.go:632-644]
qp[i] = &qdrant.QueryPoints{
	CollectionName: s.collection,
	Query:          qdrant.NewQueryID(qdrant.NewID(id)),
	Filter:         f, // NearDuplicates: scope-only, subject-less
	Limit:          qdrant.PtrOf(topK),
}
batchRes, qErr := s.client.QueryBatch(ctx, &qdrant.QueryBatchPoints{
	CollectionName: s.collection, QueryPoints: qp,
})
```
`f` there is built as `&qdrant.Filter{Must: scopeMust}` plus (per-query) `MustNot(NewHasID(id))` for
self-exclusion — never an authz condition, because `NearDuplicates` is an operator/diagnostic sweep
(classified in `operatorMigrationEmitters`, not `recallTransmitters` — see Pitfall below).
`RelatedMemories` must build `f` the way `Search` does instead:

**Verified (store.go:1223-1249, read this session — Search's filter composition, the shape to
reuse for RelatedMemories' vector/tag/citation edges):**
```go
// [VERIFIED: internal/store/store.go:1223-1249]
f := s.ownerScopeFilter(ctx, scope, subj)
if !opts.IncludeScheduled {
	f.Must = append(f.Must, activeWindowConditions(s.now())...)
}
if !opts.IncludeSuperseded {
	f.Must = append(f.Must, qdrant.NewIsEmpty("superseded_by"))
}
if !opts.IncludeArchived {
	f.Must = append(f.Must, qdrant.NewIsEmpty("archived_at"))
}
```
For `RelatedMemories`, `scope` should be `""` (cross-spine, per D-10 — "neighbours come from every
scope the caller can read") and the three recall-gate relaxation flags should NOT be exposed as
caller options (D-11 always applies the gate to vector/tag/citation edges; there is no
`IncludeArchived`-style knob for this method per CONTEXT.md).

**Citation-edge matching:** `Citations` is stored as a list of nested payload objects
(`kind`/`ref`/`locator`/`pin`/`excerpt` — verified below), so a "same kind+ref" match (D-08) needs
`qdrant.NewNestedFilter`, confirmed present in the pinned client:
```go
// [VERIFIED: go.mod:21 pins github.com/qdrant/go-client v1.19.2;
//  github.com/qdrant/go-client@v1.19.2/qdrant/conditions.go:253-262]
func NewNestedFilter(field string, filter *Filter) *Condition {
	return &Condition{
		ConditionOneOf: &Condition_Nested{
			Nested: &NestedCondition{
				Key:    field,
				Filter: filter,
			},
		},
	}
}
```
One citation-edge condition looks like `qdrant.NewNestedFilter("citations", &qdrant.Filter{Must:
[]*qdrant.Condition{qdrant.NewMatch("kind", c.Kind), qdrant.NewMatch("ref", c.Ref)}})`, ORed
(`Should`) across every citation on the anchor record.

**Verified Citation struct (store.go:398-404):**
```go
// [VERIFIED: internal/store/store.go:398-404]
type Citation struct {
	Kind    string `json:"kind"`              // file | commit | url | repo
	Ref     string `json:"ref"`               // path / repo URL / doc URL
	Locator string `json:"locator,omitempty"` // e.g. "200-240" line range
	Pin     string `json:"pin,omitempty"`     // aging anchor captured at store time
	Excerpt string `json:"excerpt,omitempty"` // cached substance
}
```
Note the comment lists `kind` values as `file | commit | url | repo` — CONTEXT.md D-08 says
"file / URL / commit"; `repo` also exists as a fourth kind. Match on `kind`+`ref` covers all four
uniformly; no special-casing needed.

**Supersession-chain edge is NOT filtered (D-11):** `source.Supersedes` and `source.SupersededBy`
are already decoded onto the fetched anchor `Memory` (no extra query for the anchor's own direct
links). Confirmed field shapes (store.go:222-234):
```go
// [VERIFIED: internal/store/store.go:222-234]
Supersedes []string `json:"supersedes,omitempty"`
SupersededBy *string `json:"superseded_by,omitempty"`
```
Walking the full chain (both directions, under a depth cap per D-09) requires following each
successor/predecessor id's own `Supersedes`/`SupersededBy` in turn via `GetReadable` (per-id
authz-checked fetch — never `Get`, which has no authz), not a Qdrant filter — the chain is a
pointer walk, not a bulk query, and `GetReadable` is exactly the per-record authz-composed fetch
this walk needs.

**Enforcement point — `fetchPayloadsByID` (searchfetch.go, read in full this session):** every
vector/tag/citation candidate id must be resolved to a `Memory` through this SAME helper `Search`
uses, passing the SAME `f` value every sub-query's filter used:
```go
// [VERIFIED: internal/store/searchfetch.go:104-124]
func (s *Store) fetchPayloadsByID(ctx context.Context, f *qdrant.Filter, view readView, ids []string) (map[string]Memory, error) {
	if len(ids) == 0 {
		return map[string]Memory{}, nil
	}
	if !view.budgeted() {
		return nil, fmt.Errorf("fetch payloads by id: view has no byte-derived ceiling: %w", ErrInvalidArgument)
	}
	out := make(map[string]Memory, len(ids))
	batchSize := perRPCLimit(view.maxRecordBytes)
	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		if err := s.fetchPayloadBatch(ctx, f, view, ids[start:end], out); err != nil {
			return nil, err
		}
	}
	return out, nil
}
```
Its doc comment (verified, quoted): *"A record that left visibility between the two phases (deleted,
superseded, archived, expired, or made private) simply fails the re-applied filter and is silently
absent from `fetchPayloadsByID`'s result map — never an error, never returned stale."*
[VERIFIED: internal/store/searchfetch.go:21-24] — this is the exact structural guarantee D-10
("never post-filtered") depends on: a candidate id the caller cannot read simply never comes back
from this call, so `RelatedMemories` has nothing to strip afterward.

### Pattern 4: ListTags is MigrateStatus's Facet call, WITH the filter MigrateStatus deliberately omits

**Verified (migrate_status.go:143-148, read this session):**
```go
// [VERIFIED: internal/store/migrate_status.go:143-148]
hits, ferr := s.client.Facet(ctx, &qdrant.FacetCounts{
	CollectionName: s.collection,
	Key:            schemaVersionKey,
	Exact:          qdrant.PtrOf(true),
	Limit:          qdrant.PtrOf(migrateStatusFacetLimit),
})
```
`ListTags` builds the same call shape but with `Key: "tags"` and `Filter: f` (the same
`ownerScopeFilter` + recall-gate composition as Pattern 3), and its own limit — **do not reuse
`migrateStatusFacetLimit` (1024)**: that constant is an operator-diagnostic bound deliberately
ABOVE `MaxRecallLimit`; D-15 says ListTags' limit must be "at most the shared 1000 maximum"
(`MaxRecallLimit`, confirmed below), a caller-facing bound, not an internal diagnostic one.

**Verified `qdrant.FacetCounts` has a `Filter` field and `GetFilter()` (points.pb.go:7899-7973,
read from the pinned module cache this session, NOT training-data recall):**
```go
// [VERIFIED: github.com/qdrant/go-client@v1.19.2/qdrant/points.pb.go:7899-7920]
type FacetCounts struct {
	// ...
	CollectionName string `protobuf:"bytes,1,opt,name=collection_name,json=collectionName,proto3" json:"collection_name,omitempty"`
	Key string `protobuf:"bytes,2,opt,name=key,proto3" json:"key,omitempty"`
	// Filter conditions - return only those points that satisfy the specified conditions.
	Filter *Filter `protobuf:"bytes,3,opt,name=filter,proto3,oneof" json:"filter,omitempty"`
	Limit *uint64 `protobuf:"varint,4,opt,name=limit,proto3,oneof" json:"limit,omitempty"`
	Exact *bool `protobuf:"varint,5,opt,name=exact,proto3,oneof" json:"exact,omitempty"`
	// ...
}
```
```go
// [VERIFIED: github.com/qdrant/go-client@v1.19.2/qdrant/points.pb.go:7968-7973]
func (x *FacetCounts) GetFilter() *Filter {
	if x != nil {
		return x.Filter
	}
	return nil
}
```

**Verified response shape (points.pb.go:8010-8117, 10405-10414):**
```go
// [VERIFIED: github.com/qdrant/go-client@v1.19.2/qdrant/points.pb.go:8108-8117]
type FacetHit struct {
	Value *FacetValue `protobuf:"bytes,1,opt,name=value,proto3" json:"value,omitempty"`
	Count uint64      `protobuf:"varint,2,opt,name=count,proto3" json:"count,omitempty"`
}
// FacetValue is a oneof: StringValue | IntegerValue | BoolValue (tags is a
// keyword field, so h.GetValue().GetStringValue() is the accessor to use).
```
```go
// [VERIFIED: github.com/qdrant/go-client@v1.19.2/qdrant/points.pb.go:10410-10413]
type FacetResponse struct {
	Hits []*FacetHit `protobuf:"bytes,1,rep,name=hits,proto3" json:"hits,omitempty"`
	Time float64
	Usage *Usage
}
```
There is no truncation flag on `FacetResponse` (confirmed: only `Hits`/`Time`/`Usage`) — exactly
the trap `MigrateStatus`'s own comment names: *"`qdrant.FacetCounts.Limit` is a MAXIMUM, not a
truncation signal, and `FacetResponse` carries no truncation flag"* [VERIFIED:
internal/store/migrate_status.go:100-103]. For D-15's `more bool`, request `Limit: N+1` and set
`more = len(hits) > N` (trim the extra), OR mirror `ListScopes`'s exact idiom
(`len(pts) == scanCap`) with `Limit: N` and treat a full page as possibly-truncated — planner's
choice, but the response carries no signal of its own, so the request-side limit is the only lever.

**Verified `MaxRecallLimit`/`rejectOverMaximum` (store.go:1646-1660):**
```go
// [VERIFIED: internal/store/store.go:1646-1660]
const MaxRecallLimit = 1000

func rejectOverMaximum(field string, count uint64) error {
	if count <= MaxRecallLimit {
		return nil
	}
	return fmt.Errorf("%s exceeds the maximum of %d: %w", field, MaxRecallLimit, ErrInvalidArgument)
}
```

**Verified `ensureIndexes` is missing a `tags` index (store.go:657-687, read in full):**
```go
// [VERIFIED: internal/store/store.go:657-687]
func (s *Store) ensureIndexes(ctx context.Context, name string) error {
	type idx struct {
		field  string
		typ    qdrant.FieldType
		params *qdrant.PayloadIndexParams
	}
	idxs := []idx{
		{"owner", qdrant.FieldType_FieldTypeKeyword,
			qdrant.NewPayloadIndexParamsKeyword(&qdrant.KeywordIndexParams{IsTenant: qdrant.PtrOf(true)})},
		{"scope", qdrant.FieldType_FieldTypeKeyword, nil},
		{"created_at", qdrant.FieldType_FieldTypeDatetime, nil},
		{"short_id", qdrant.FieldType_FieldTypeKeyword, nil},
		{schemaVersionKey, qdrant.FieldType_FieldTypeInteger, nil},
	}
	for _, ix := range idxs {
		req := &qdrant.CreateFieldIndexCollection{
			CollectionName:   name,
			FieldName:        ix.field,
			FieldType:        qdrant.PtrOf(ix.typ),
			FieldIndexParams: ix.params,
			Wait:             qdrant.PtrOf(true),
		}
		if _, err := s.client.CreateFieldIndex(ctx, req); err != nil {
			if st, ok := status.FromError(err); ok && st.Code() == grpccodes.AlreadyExists {
				continue
			}
			return fmt.Errorf("ensure index %q: %w", ix.field, err)
		}
	}
	return nil
}
```
`tags` is a `[]string` payload field (list-of-keyword); add `{"tags", qdrant.FieldType_FieldTypeKeyword, nil}`
to `idxs` — no `KeywordIndexParams` needed (only `owner` uses `IsTenant`). This is additive and
idempotent, mirroring every prior index this function added (the `AlreadyExists` continue makes it
safe on every existing deployment, per the function's own doc comment above line 655, read this
session: *"Indexes are ensured on every boot... so existing collections gain them without a data
migration."* — not directly quoted verbatim here since only the code block above was captured
verbatim; treat this paraphrase as [CITED: internal/store/store.go, ensureIndexes doc comment]
rather than [VERIFIED]).

### Anti-Patterns to Avoid

- **Post-filtering RelatedMemories/ListTags results in a future handler:** ARCHITECTURE.md names
  this explicitly (`.planning/research/ARCHITECTURE.md:164-167`) and it is the same DEC-cgb
  violation Pitfall 6 in PITFALLS.md describes for the RPC layer. Not reachable this phase (no
  handler exists yet), but the store methods must be built so that Phase 3's handler has nothing to
  filter — see Pattern 3's `fetchPayloadsByID` enforcement point.
- **Reusing `ActionWrite` for archive/restore:** explicitly rejected by D-01. A future policy that
  narrows `write` (e.g., a field-level ABAC rule) would then accidentally also narrow archive/restore
  with no way to distinguish them.
- **A nil-Subject-means-operator sentinel:** explicitly rejected by D-02. `principalParams` already
  fails closed for an unrecognized `Subject` (`ok == false` → `ErrNotFound`) — a magic nil-means-CLI
  path would be a second, divergent authz surface exactly like the handler-level anti-pattern above.
- **Copying `migrateStatusFacetLimit` (1024) for ListTags' limit:** it deliberately exceeds
  `MaxRecallLimit` (1000) because it is an operator diagnostic, not a caller-facing bound — see
  Pattern 4.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Owner-only write gate | A `memory.Owner == subj.sub` comparison inline in the new archive/restore methods | `getWritable(ctx, id, subj, authz.ActionArchive)` | This IS the DEC-cgb chokepoint; a hand-rolled comparison is a second, divergent enforcement point (Pitfall 6's exact failure mode, just one layer earlier than the RPC handler it names) |
| Read-scoped filter for RelatedMemories/ListTags | A bespoke `owner == X OR visibility == "shared"` filter builder | `s.ownerScopeFilter(ctx, scope, subj)` | Already handles anonymous vs authenticated, own vs shared buckets, and PDP consultation (`decideBucket`) — a new builder would need to reproduce all of it correctly |
| Recall-gate exclusions (archived/superseded/scheduled) | New `IsEmpty`/range conditions authored from scratch for the two new methods | `activeWindowConditions(s.now())`, `qdrant.NewIsEmpty("superseded_by")`, `qdrant.NewIsEmpty("archived_at")` — the exact three conditions `Search`/`List` already append | These are the SAME three conditions across four existing call sites (store.go comment: "this idiom appears at four sites in this file"); a fifth hand-rolled copy is exactly the kind of drift the codebase's own comments warn against |
| Query-by-id vector sub-query | A raw `qdrant.NewQuery` reconstructed from the anchor's own vector | `qdrant.NewQueryID(qdrant.NewID(id))` | `NearDuplicates` already established this as the precedent (spine.go:632-644); it queries by the point's OWN stored vector server-side, no re-embedding, no vector round-trip through Go |
| Truncation signaling | A boolean derived from `FacetResponse` fields that don't exist (no truncation flag) | Request `Limit: N+1` and compare `len(hits) > N`, mirroring `ListScopes`'s `len(pts) == scanCap` idiom | `FacetResponse` genuinely carries no truncation signal (confirmed by direct struct read) — the request-side over-ask is the only lever, exactly as `MigrateStatus`'s own doc comment explains for its own (different) truncation problem |

**Key insight:** every primitive this phase needs — the owner gate, the read filter, the recall-gate
conditions, the query-by-id sub-query, the byte-budgeted two-phase fetch — already exists in
`internal/store` and is exercised by `Delete`/`Update`/`Search`/`List`/`NearDuplicates`/
`MigrateStatus` today. This phase is compositional, not novel: the risk is in composing the *right*
existing primitives (authz filter INTO the new methods, not around them), not in inventing new ones.

## Common Pitfalls

### Pitfall 1: The recall-gate test's completeness guarantee is THREE lists, not the two D-15 names

**What goes wrong:** A planner reads CONTEXT.md's D-15 ("recognizedFilterCarryingRequestMethods and
the interceptor type switch must recognise Facet") and widens exactly those two things. CI goes
green. `TestSchemaVersionNeverGatesRecall` (the dynamic, live-capture proof) passes because the
interceptor now decodes `ListTags`' Facet call correctly. But
`TestRecallEmissionSetIsCompleteAndClassified` (the STATIC, AST-derived completeness proof) ALSO
passes — silently, for the wrong reason: it never saw the new call site at all, because its own
maintained method vocabulary doesn't include `"Facet"`.

**Why it happens:** the file's own package doc comment names this exact failure mode as "limit 2":
*"The method vocabulary ... is a MAINTAINED LIST, and the classification CANNOT backstop it: an
emission behind an unenumerated method name produces no subject at all, reaches none of the three
lists, and causes no set difference."* [VERIFIED: internal/store/schemaversion_recallgate_test.go:32-35]
This happened before, for `ScrollAndOffset` — the file's own history names it as "durable record
x6v6qxqd6f."

Three independent maps/lists gate on the string `"Facet"` or the name `"Store.ListTags"`, at these
exact, verified locations:

1. **`recognizedFilterCarryingRequestMethods`** — the dynamic interceptor's allowlist:
   ```go
   // [VERIFIED: internal/store/schemaversion_recallgate_test.go:868-872]
   var recognizedFilterCarryingRequestMethods = map[string]bool{
   	"Query":  true, // *qdrant.QueryPoints
   	"Scroll": true, // *qdrant.ScrollPoints (also covers ScrollAndOffset — see grpcMethodForEmission)
   	"Count":  true, // *qdrant.CountPoints
   }
   ```
   Needs `"Facet": true` added.

2. **`recallCaptureInterceptor`'s type switch** — the same file, the dynamic decoder:
   ```go
   // [VERIFIED: internal/store/schemaversion_recallgate_test.go:891-905]
   func recallCaptureInterceptor(t *testing.T, capture *recallCapture) grpc.UnaryClientInterceptor {
   	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
   		switch r := req.(type) {
   		case *qdrant.QueryPoints:
   			capture.record("Query", r.GetFilter())
   		case *qdrant.ScrollPoints:
   			capture.record("Scroll", r.GetFilter())
   		case *qdrant.CountPoints:
   			capture.record("Count", r.GetFilter())
   		default:
   			if fc, ok := req.(filterCarryingRequest); ok && fc.GetFilter() != nil {
   				t.Fatalf("recallCaptureInterceptor: gRPC method %s (request type %T) carries a *qdrant.Filter but is not in the interceptor's recognized set — widen recognizedFilterCarryingRequestMethods and this type switch", method, req)
   			}
   		}
   		return invoker(ctx, method, req, reply, cc, opts...)
   	}
   }
   ```
   Needs a `case *qdrant.FacetCounts: capture.record("Facet", r.GetFilter())` arm added — confirmed
   `*qdrant.FacetCounts` implements `filterCarryingRequest` (has `GetFilter() *Filter`, verified in
   Pattern 4 above), so WITHOUT this case, `ListTags`' filtered Facet call hits the `default` arm and
   `t.Fatalf`s the moment any recall-path test exercises it live — this is a hard compile-and-pass,
   then FAIL-AT-RUNTIME trap, not a silent gap, for THIS list specifically. (D-15 already names
   these two; #1 and #2 above are NOT new findings, just confirmed by direct read.)

3. **`recallEmissionMethods`** — the STATIC AST-scan vocabulary, a DIFFERENT list in the same file,
   NOT named by D-15:
   ```go
   // [VERIFIED: internal/store/schemaversion_recallgate_test.go:334-340]
   var recallEmissionMethods = map[string]bool{
   	"Query":           true,
   	"QueryBatch":      true,
   	"Scroll":          true,
   	"ScrollAndOffset": true,
   	"Count":           true,
   }
   ```
   Needs `"Facet": true` added, or `TestRecallEmissionSetIsCompleteAndClassified` never derives
   `Store.ListTags` as an emission site at all — it passes green with zero knowledge the method
   exists.

4. **`recallEntryPointSeeds`** — the caller-facing entry-point roster the whole static gate is
   anchored on, ALSO not named by D-15:
   ```go
   // [VERIFIED: internal/store/schemaversion_recallgate_test.go:355-362]
   var recallEntryPointSeeds = []string{
   	"Store.Search",
   	"Store.SearchReranked",
   	"Store.SearchDiscovery",
   	"Store.List",
   	"Store.ListScheduled",
   	"Store.ListScopes",
   }
   ```
   `ListScopes` is IN this list specifically because — quoting its own comment — *"it is served to
   callers through BOTH Connect ... and MCP ..., so D-16's operator-tier exclusion rationale does not
   reach it"* [VERIFIED: internal/store/schemaversion_recallgate_test.go:350-354]. `ListTags` is
   exactly the same shape (Phase 3 will expose it via both Connect and MCP per RPC-04) — it belongs
   in this list as a 7th seed, with `Store.ListTags` added to `recallTransmitters`
   (`schemaversion_recallgate_test.go:486-511`) with its own justification row, NOT to
   `operatorMigrationEmitters` (that bucket is reserved for genuinely operator-only sweeps like
   `MigrateStatus`'s own Facet call, which is explicitly "Never reachable from any
   recallEntryPointSeeds member" — `schemaversion_recallgate_test.go:554`).

**How to avoid:** treat this as a 4-list, not 2-list, edit. Grep for `Facet` inside
`internal/store/schemaversion_recallgate_test.go` before considering this task done, and confirm
`TestRecallEmissionSetIsCompleteAndClassified`'s two subtests both still pass with `Store.ListTags`
now appearing in their derived sets.

**Warning signs:** CI is green but `go test ./internal/store/... -run TestRecallEmissionSetIsCompleteAndClassified -v`
shows the subtest names passed without any mention of `ListTags` in verbose output, or the test's
own `t.Run("reachable emission completeness", ...)` subtest's derived set size is unchanged from
before this phase.

**Note on `RelatedMemories`:** it also emits `Query`/`Scroll`/`QueryBatch` calls carrying the authz
filter — those method names are ALREADY in all the relevant lists (`Query`/`Scroll`/`QueryBatch` are
all pre-existing entries), so `RelatedMemories` only needs a `recallTransmitters` classification row
and (if it should be a caller-facing entry point like `ListScopes`/`ListTags`) a
`recallEntryPointSeeds` entry — no NEW method-name widening the way `Facet` requires. Decide during
planning whether `RelatedMemories` joins `recallEntryPointSeeds` this phase (it will be Connect+MCP
exposed in Phase 3, same reasoning as `ListTags`) or is deferred — either way it needs SOME entry
in one of the three classification lists (`recallTransmitters`/`operatorMigrationEmitters`/
`otherNonRecallEmitters`), or `TestRecallEmissionSetIsCompleteAndClassified`'s "three-way
classification of the remainder" subtest fails loudly (by design — an unclassified reachable site
is a hard failure, not a silent gap, for THIS particular list).

### Pitfall 2: `getWritable` already calls `Get` — a naive gated wrapper double-fetches inside the lock

**What goes wrong:** implementing `ArchiveAs` as "call `getWritable` for the authz check, then call
the existing `Archive`-core logic which itself calls `Get` again" issues two Qdrant `Get`s inside
one lock window for every archive/restore call.

**Why it happens:** `getWritable` (verified above) already does `s.Get(ctx, id)` as its first
statement. The existing `Archive`/`Restore` cores also open with `s.Get(ctx, id)` (verified in
Pattern 1). Composing them naively duplicates the fetch.

**How to avoid:** factor `Archive`/`Restore`'s existing body (past the initial `Get`) into an
unexported core that accepts an already-resolved `Memory` (mirroring how `Update` accepts `cur
Memory` from `FetchForUpdate` rather than re-fetching — verified pattern at store.go:2255-2269,
`FetchForUpdate` doc comment: *"The update handler calls this once as the authoritative ownership
gate BEFORE embedding, then hands the returned record to Update — so the update path performs a
single Qdrant Get instead of two."*). `ArchiveAs`/`RestoreAs` then call `getWritable` once, get the
`Memory` back, and hand it to the shared core; the subject-less `Archive`/`Restore` keep calling
`s.Get` directly since they have no authz check to piggyback on.

**Warning signs:** a benchmark or trace showing two `Get` RPCs per gated archive/restore call; a
test asserting call counts against a spy client showing 2 Gets where Delete/Update show 1.

### Pitfall 3: Rule-category rejection for archive/restore is a non-issue at the store layer (verified, not assumed)

**What goes wrong (that this research rules out):** a planner might assume D-03 ("the gated path
ALLOWS the owner to archive/restore a rule — unlike supersede and set_visibility, which reject
rules") requires adding a `category == "rule"` check somewhere in the new gated methods to make
archive/restore's permissiveness explicit, mirroring how supersede/set_visibility reject rules.

**What is actually true (verified this session):** `Store.SetVisibility` (store.go:2545-2570, read
in full) contains NO `category`/`"rule"` check at all:
```go
// [VERIFIED: internal/store/store.go:2545-2570, full function body]
func (s *Store) SetVisibility(ctx context.Context, id string, subj Subject, shared bool) (err error) {
	// ...
	if _, err := s.getWritable(ctx, id, subj, authz.ActionShare); err != nil {
		return err
	}
	vis := ""
	if shared {
		vis = visibilityShared
	}
	_, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{ /* ... */ })
	return err
}
```
The rule-immutability rejection (`errRuleImmutable`, "rules are always shared") lives ONE LAYER UP,
in `internal/server/tools.go` (the MCP tool handler), not in `internal/store`:
```
// [VERIFIED via rg: internal/server/identity.go:153]
var errRuleImmutable = errors.New("rules are always shared")
// [VERIFIED via rg: internal/server/tools.go:2163,2329,2522 — three call sites,
//  all inside internal/server, none inside internal/store]
```
**Conclusion:** the new gated `ArchiveAs`/`RestoreAs` need NO rule-category special-casing to
satisfy D-03 — the store layer was never rejecting rules for any mutation; that rejection is a
Phase-3-and-later server/MCP-tool-layer concern (out of scope for this phase, and only relevant when
a future Rules view or MCP tool considers whether to add an archive-specific `errRuleImmutable`-style
guard — CONTEXT.md already flags that as CUR-03/Phase-4 territory).

### Pitfall 4: `MaxRecallLimit`/`rejectOverMaximum` guards k, not per-edge-type caps

**What goes wrong:** applying `rejectOverMaximum` to the total `RelatedMemories` result count, or to
each of the four edge-type caps independently, rather than only to the caller-supplied vector `k`
per D-12 ("the caller's k adjusts only the vector cap and is subject to the shared maximum of
1000").

**Why it happens:** `rejectOverMaximum` is currently called at exactly three existing sites, all
guarding a caller-supplied count directly passed to a Qdrant `Limit` (`Search`'s `k`,
`SearchReranked`'s `k`, `SearchDiscovery`'s `k`, `List`/`ListScheduled`'s `limit`) — it is a
request-validation guard, not a general-purpose "cap this number" utility.

**How to avoid:** call `rejectOverMaximum("k", k)` only against the caller's vector-edge `k`
argument (mirroring `Search`'s own first line, store.go:1219); the small per-type caps (tag/citation/
supersession-depth) and the total ceiling are planner-chosen constants (D-12: "planner picks;
roughly 8" / "a hard total ceiling"), never subject to `rejectOverMaximum`'s public-facing error
contract, since a caller never supplies them directly.

## Runtime State Inventory

Not applicable — this is a greenfield-within-package phase (new methods added to an existing
package), not a rename/refactor/migration phase. No stored data, service config, OS-registered
state, secrets, or build artifacts reference anything by a name this phase changes.

## Code Examples

### Owner-vs-shared authz test pattern to mirror for SC1 (D-05's "first test")

**Verified existing precedent (store_test.go:836-858, `TestDeleteOwnerGate`, read in full):**
```go
// [VERIFIED: internal/store/store_test.go:836-858]
func TestDeleteOwnerGate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:del"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	// Even a SHARED record is not deletable by a non-owner.
	m := Memory{ID: "eeeeeeee-0000-0000-0000-000000000001", Content: "s", Scope: scope, Owner: "sub-B", Visibility: "shared", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := s.Delete(ctx, m.ID, Authenticated("sub-A")); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-owner delete: want ErrNotFound, got %v", err)
	}
	if _, err := s.Get(ctx, m.ID); err != nil {
		t.Errorf("record should survive non-owner delete: %v", err)
	}
	if err := s.Delete(ctx, m.ID, Authenticated("sub-B")); err != nil {
		t.Errorf("owner delete failed: %v", err)
	}
	if _, err := s.Get(ctx, m.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("owner delete did not remove record: %v", err)
	}
}
```
The SC1 test (owner can archive/restore; a shared-but-not-owned record's caller gets `ErrNotFound`;
an anonymous caller vs an owned record) should follow this exact shape: `testStore(t)`,
`Authenticated("sub-A")`/`Authenticated("sub-B")`/`Anonymous()` (all confirmed real constructors —
`internal/store/subject.go:24-43`), `Upsert` a fixture record, assert `errors.Is(err, ErrNotFound)`
for the denied path, assert the record's state is unchanged (`Get` still returns/doesn't return
`ArchivedAt`) for both the denied and allowed paths.

### CLI caller that must keep working unchanged (SC2)

**Verified (spine_review_archive.go:211,242, read in full):**
```go
// [VERIFIED: cmd/engram/spine_review_archive.go:211]
results, procErr := spineArchiveOrRestore(ctx, st, spineArchiveIDs, st.Archive)
```
```go
// [VERIFIED: cmd/engram/spine_review_archive.go:242]
results, procErr := spineArchiveOrRestore(ctx, st, spineRestoreIDs, st.Restore)
```
`spineArchiveOrRestore`'s `fn` parameter type is `func(context.Context, string) (store.ArchiveResult, error)`
(verified: `cmd/engram/spine_review_archive.go:114`) — i.e. the CLI is coupled to `Archive`/
`Restore`'s EXACT signature `(ctx, id) (ArchiveResult, error)`. This is the hard constraint behind
D-02's "existing `Archive(ctx, id)` / `Restore(ctx, id)` signatures and behaviour stay unchanged":
changing the signature (e.g., adding a `Subject` parameter with a privileged sentinel value) breaks
this CLI call site's function-value assignment at compile time, not just behaviorally — SC2 is
therefore mechanically enforced by `go build` succeeding, not merely a runtime behavior to test.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Facet calls with no filter (`MigrateStatus`, the only precedent) | Facet calls WITH the caller's authz filter (`ListTags`, this phase) | This phase | First filtered-Facet call site in the codebase; requires the three-list recall-gate widening in Pitfall 1 |
| Five Cedar actions (`read`/`write`/`delete`/`share`/`schedule`) | Six, adding `archive` | This phase (D-01) | The action vocabulary comment at authz.go:22 ("The five engram authorization actions") becomes stale and should be updated to six in the same change |

No other "old vs new" migration applies — this phase adds capability, it does not replace an
existing approach.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `ensureIndexes`' doc comment (paraphrased, not directly quoted in the verified code block) says indexes are idempotently ensured on every boot so existing deployments gain new ones without a data migration | Architecture Patterns, Pattern 4 | Low — the code's own idempotent `AlreadyExists`-continue behavior (which IS verified verbatim) makes this true by construction regardless of the comment's exact wording |
| A2 | The rule identifiers `m45p2b4bp7`, `3p0zsqrhmb`, `xvqj44e5mk`, `xhg7dgmqx4` (referenced in the phase brief) are engram memory-store rules whose exact text was NOT independently re-verified this session — this session had no `engram` MCP tool available to query them; their summarized meaning (test third-party behavior gate, no-tests-for-tests, prefer idiomatic OSS, key_links drift) is taken as given from the invocation prompt | Common Pitfalls (implicit — these rules constrain test design) | Medium if the planner writes a test that violates one of these rules based on a misremembered summary; low-cost mitigation is to re-fetch these specific rule ids via engram at plan time if the exact wording matters for a specific test design decision |
| A3 | `RelatedMemories`' per-edge-type overlap-fraction scoring formula for tag/citation edges (`|intersection|/|union|` or similar) is ARCHITECTURE.md's suggestion, not a locked decision — D-07 locks the RARITY-WEIGHTED formula for tags specifically (IDF-style, computed from the ListTags facet), which supersedes ARCHITECTURE.md's simpler overlap-fraction suggestion for tags. The citation edge's scoring approach is not locked by CONTEXT.md at all (Claude's Discretion) | Architecture Patterns, Pattern 3 | Low — CONTEXT.md's D-07/D-08 are the locked source of truth already reproduced in User Constraints; this note just flags that ARCHITECTURE.md (an earlier research pass) and CONTEXT.md (later, user-reviewed) disagree slightly on tag-edge scoring, and CONTEXT.md wins |

**If this table is empty:** N/A — three items above need awareness, none blocks planning.

## Open Questions

1. **Should `RelatedMemories` and/or the new gated Archive/Restore methods join `recallEntryPointSeeds`
   this phase, or wait for Phase 3?**
   - What we know: `ListTags` clearly should (D-15's own spirit + the `ListScopes` precedent's
     stated rationale — exposed via both Connect and MCP). `RelatedMemories` will ALSO be exposed
     via both lanes per RPC-04, so the same rationale applies to it too, by parallel reasoning.
     Archive/Restore's gated methods are id-addressed single-record writes, not bulk recall paths —
     they may not need a `recallEntryPointSeeds` entry at all (Delete/Update/SetVisibility, the
     methods `getWritable` already gates, are NOT in `recallEntryPointSeeds` either, since that list
     is specifically about BULK recall, not single-id fetch-then-authorize).
   - What's unclear: whether the planner wants to pre-empt Phase 3's exposure decision now or treat
     the `recallEntryPointSeeds`/`recallTransmitters` classification as Phase 3's responsibility
     (since the classification's own stated purpose is "caller-facing recall entry points," and this
     phase adds no caller yet).
   - Recommendation: add `ListTags` to `recallEntryPointSeeds`/`recallTransmitters` THIS phase (its
     Facet call is inherently a recall-shaped bulk read, unlike Archive/Restore's single-id write),
     and add `RelatedMemories` to `recallTransmitters` (required either way, since it's a reachable
     Query/Scroll emission the moment it exists) but leave the `recallEntryPointSeeds` question for
     Phase 3 to decide alongside the RPC exposure — document whichever choice is made in the
     resulting PLAN/SUMMARY so Phase 3's research doesn't have to re-derive it.

2. **Exact numeric defaults for RelatedMemories' per-edge caps, total ceiling, and supersession
   depth cap (D-12), and ListTags' limit (D-15).**
   - What we know: D-12 suggests "roughly 8" per edge type as a starting point; D-15 says "at most
     the shared 1000 maximum" for ListTags.
   - What's unclear: the exact numbers — these are explicitly Claude's Discretion per CONTEXT.md,
     not something this research should lock.
   - Recommendation: planner picks concrete constants (e.g., 8 per edge type, a depth cap of 5,
     a total ceiling of 32, a ListTags default limit of 50-100) and documents the reasoning inline
     as a code comment, following the codebase's own convention of explaining every numeric
     constant's provenance (see `nearDuplicateBatchSize`'s and `defaultNearDuplicateTopK`'s doc
     comments as the house style to match).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Building/testing `internal/store`, `internal/authz` | ✓ | go1.27.1 (module pins `go 1.26.7`) [VERIFIED: `go version`, go.mod:3] | — |
| Docker | `internal/store/storetest`'s Qdrant testcontainer (integration tests) | ✓ | `docker info` succeeded [VERIFIED: this session] | `ENGRAM_QDRANT_TEST_ADDR` pointing at an externally-run Qdrant, or tests skip per `SkipOrFailNoQdrant` when `ENGRAM_REQUIRE_QDRANT` is unset |
| `qdrant/qdrant:v1.19.1` (pinned test image) | Integration tests via `storetest.Run` | Pulled on demand by testcontainers | `v1.19.1` [VERIFIED: internal/store/storetest/storetest.go:49] | None needed — pinned and CI-gated for drift (`TestQdrantImageMatchesCIService`) |
| `task` CLI | `task test`, `task lint`, `task fmt` gates | ✓ | 3.52.0 [VERIFIED: `task --version`, this session] | Direct `go test`/`golangci-lint`/`gofmt` invocations (Taskfile.yaml lines 50,65,104,126,139) |

**Missing dependencies with no fallback:** none.

**Missing dependencies with fallback:** none observed missing in this environment; the one
documented fallback (external Qdrant via `ENGRAM_QDRANT_TEST_ADDR`, or a skip) exists for CI
environments where Docker-in-Docker is unavailable.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib), integration tests against a real Qdrant via testcontainers-go (`internal/store/storetest`) |
| Config file | none — `go test` flags + `ENGRAM_QDRANT_TEST_ADDR`/`ENGRAM_REQUIRE_QDRANT` env vars |
| Quick run command | `go test ./internal/store/... ./internal/authz/... -run '<TestName>' -v` (Docker required, or set `ENGRAM_QDRANT_TEST_ADDR`) |
| Full suite command | `go test ./internal/store/... ./internal/authz/... -count=1` |

### Phase Requirements → Test Map

| Req ID / SC | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| STORE-01 / SC1 | Owner can archive/restore; shared-but-not-owned caller gets `ErrNotFound`, record unchanged; anonymous vs owned records fail closed | integration (real Qdrant) | `go test ./internal/store/... -run TestArchiveAsOwnerGate -v` (planner names the test; mirror `TestDeleteOwnerGate`, store_test.go:836) | ❌ Wave 0 — new test |
| STORE-01 / SC1 (policy layer) | `ActionArchive` denies for a shared, non-owned resource at the Cedar-corpus level (permanent regression backstop, not just store-level) | unit (no Qdrant) | `go test ./internal/authz/... -run TestPolicyCorpus_SharedReadOnly -v` (existing test, widened per Pattern 2) | ✅ existing — widen `allActions` + the inline action list |
| STORE-01 / SC2 | CLI `spine-review archive`/`restore` unchanged | compile-time (signature) + existing CLI tests | `go build ./... && go test ./cmd/engram/... -run 'TestSpineReviewArchive|TestSpineReviewRestore' -v` (exact test names TBD — grep `cmd/engram/*archive*_test.go` at plan time) | ✅ existing test files presumed present — verify at plan time |
| STORE-02 / SC3 | `RelatedMemories` returns supersession/tag/citation/vector edges; caller's read predicate composed into every sub-query; bounded; multi-edge-type candidate rule holds | integration (real Qdrant) | `go test ./internal/store/... -run TestRelatedMemories -v` (planner names; new test file suggested: `relatedmemories_test.go`) | ❌ Wave 0 — new test |
| STORE-02 / SC3 (authz enforcement) | A candidate the caller cannot read is never returned by `RelatedMemories`, even when it is a genuine near neighbour/shared-tag match | integration (real Qdrant), seed both a readable and an unreadable near-duplicate | `go test ./internal/store/... -run TestRelatedMemoriesExcludesUnreadable -v` | ❌ Wave 0 — new test |
| STORE-03 / SC4 | `ListTags` returns exact facet counts under the caller's read filter, ordered desc, bounded with `more` | integration (real Qdrant) | `go test ./internal/store/... -run TestListTags -v` (new file suggested: `listtags_test.go`) | ❌ Wave 0 — new test |
| STORE-03 / SC4 (recall-gate allowlist) | The static AST completeness scan and the dynamic live-capture interceptor both recognize `ListTags`' filtered `Facet` call | unit/AST-scan + integration (real Qdrant) | `go test ./internal/store/... -run TestRecallEmissionSetIsCompleteAndClassified -v` AND `go test ./internal/store/... -run TestSchemaVersionNeverGatesRecall -v` (both existing, widened per Pitfall 1) | ✅ existing — widen the 4 lists in Pitfall 1 |
| STORE-03 / SC4 (tags index) | `tags` keyword payload index exists after boot | integration (real Qdrant) | Existing `ensureIndexes` test coverage, if any — grep `TestEnsureIndexes` at plan time; otherwise assert via `s.client.ListFieldIndexes` or equivalent in the new `ListTags` test's setup | Verify at plan time |

### Sampling Rate
- **Per task commit:** `go test ./internal/store/... ./internal/authz/... -run '<touched test(s)>' -v`
- **Per wave merge:** `go test ./internal/store/... ./internal/authz/... -count=1` (full package, real Qdrant via Docker)
- **Phase gate:** `task` (lint + full test suite) green before `/gsd:verify-work`, per CLAUDE.md's Session Completion contract

### Wave 0 Gaps
- [ ] New test file(s) for the authz-gated Archive/Restore path (SC1) — suggested `archive_authz_test.go`, mirroring `TestDeleteOwnerGate`'s shape exactly
- [ ] New test file for `RelatedMemories` (SC3) — suggested `relatedmemories_test.go`, seeding fixtures for all four edge types plus at least one unreadable near-neighbour to prove the authz-composition claim
- [ ] New test file for `ListTags` (SC4) — suggested `listtags_test.go`
- [ ] Widen `internal/authz/policy_corpus_test.go`'s `allActions` (line 14) and the shared-read-deny action list (line 38) — no new file, existing file edit
- [ ] Widen the 4 recall-gate lists in `internal/store/schemaversion_recallgate_test.go` per Pitfall 1 — no new file, existing file edit, but genuinely easy to under-scope to 2 of the 4
- [ ] No new test framework or fixture infra needed — `testStore`/`dialTestClient`/`storetest.Run`/`Authenticated`/`Anonymous` all pre-exist and cover every fixture shape this phase needs

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | Out of scope — this phase does not touch token validation |
| V3 Session Management | No | Out of scope |
| V4 Access Control | Yes | Cedar PDP (`internal/authz`) consulted via `decideRecord`/`decideBucket`, compiled into Qdrant filters/`getWritable` gates inside `internal/store` only (DEC-cgb) — never a handler-level check, never a hand-rolled ownership comparison |
| V5 Input Validation | Yes (narrow) | `rejectOverMaximum` for the vector `k` argument on `RelatedMemories`; `ResolvePointID`'s existing UUID/short_id parsing handles id-shaped input upstream of this phase's methods (unchanged this phase) |
| V6 Cryptography | No | Not touched this phase |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Handler-level authz drift (a future RPC re-implementing the owner check instead of delegating) | Elevation of Privilege | DEC-cgb: all authz lives in `internal/store`; this phase's methods are the ONLY place the check can correctly live, so there is nothing for a future handler to re-implement if it simply calls these methods |
| Information disclosure via a distinguishable 403-vs-404 (revealing a shared record's existence to a non-owner attempting archive) | Information Disclosure | `getWritable` returns the SAME `ErrNotFound` for "doesn't exist" and "exists but not owned" (DEC-xa6) — D-04 explicitly requires this for the new gated Archive/Restore path |
| Timing/count side-channel via `RelatedMemories` revealing how many unauthorized neighbours exist (e.g., a caller inferring a private record's existence from a truncated/short edge list) | Information Disclosure | `fetchPayloadsByID`'s "drop on disappear" semantics (an unauthorized candidate simply never appears in the returned map — no error, no distinguishable truncation code) is the existing mitigation; ARCHITECTURE.md names this exact concern (`ARCHITECTURE.md:166`) |
| A new Cedar action silently widening more than intended (e.g., a future policy author assumes `archive` implies something about `write`) | Elevation of Privilege | `ActionArchive` is a genuinely distinct action string, evaluated independently by Cedar; `own_records.cedar`'s unconstrained `action` match means "any action for the owner" already covers it correctly, and `shared_read.cedar`'s `action == Action::"read"` constraint already excludes it — no implicit widening exists in either policy (verified by direct read) |

## Sources

### Primary (HIGH confidence — direct `Read`/`Bash cat -n` of source this session)
- `internal/authz/authz.go` (Action consts, DecideRecord/DecideBucket, decision logging)
- `internal/authz/entities.go` (`actionEntityUID` — dynamic action entity construction)
- `internal/authz/schema.json` (full file — confirmed reference-only/not-CI-gated)
- `internal/authz/policy_corpus_test.go` (full file — `allActions`, all 6 corpus tests)
- `internal/authz/policies/*.cedar` (all 4 files, full text)
- `internal/store/spine.go` (Archive/Restore full bodies; NearDuplicates full sub-query construction)
- `internal/store/store.go` (getWritable, decideRecord, ownerScopeFilter, ownerOrSharedCondition,
  ownerOnlyCondition, principalParams, Search's full filter composition, Delete, SetVisibility,
  ListScopes, ensureIndexes, rejectOverMaximum/MaxRecallLimit, Memory struct, Citation struct,
  payload()/fromPayload() codec, FetchForUpdate)
- `internal/store/searchfetch.go` (full file — fetchPayloadsByID, fetchPayloadBatch, includeIDs)
- `internal/store/migrate_status.go` (full file — the Facet-without-filter precedent)
- `internal/store/schemaversion_recallgate_test.go` (package doc comment; recognizedFilterCarryingRequestMethods;
  recallCaptureInterceptor; recallEmissionMethods; recallEntryPointSeeds; recallTransmitters/
  operatorMigrationEmitters/otherNonRecallEmitters; TestRecallEmissionSetIsCompleteAndClassified)
- `internal/store/store_test.go` (TestDeleteOwnerGate, TestUpdateOwnerGateAndSharedFlag,
  testStore/dialTestClient/newTestStore, existing Archive test roster)
- `internal/store/export_test.go`, `internal/store/main_test.go`, `internal/store/storetest/storetest.go`
  (testcontainer lifecycle, `ENGRAM_QDRANT_TEST_ADDR`/`ENGRAM_REQUIRE_QDRANT` contract, pinned image)
- `internal/store/subject.go` (Subject/Authenticated/Anonymous/principalParams)
- `internal/server/identity.go`, `internal/server/tools.go` (grep-confirmed: `errRuleImmutable`'s
  three call sites, none inside `internal/store`)
- `cmd/engram/spine_review_archive.go` (full file — the CLI caller SC2 must not break)
- `github.com/qdrant/go-client@v1.19.2` module cache (`qdrant/points.pb.go`,
  `qdrant/points_service_grpc.pb.go`, `qdrant/conditions.go`) — `FacetCounts`/`FacetHit`/
  `FacetValue`/`FacetResponse` structs, `Facet`'s gRPC method name, `NewNestedFilter`/
  `NewNestedCondition`
- `go.mod` (pinned versions: `github.com/qdrant/go-client v1.19.2`, `github.com/cedar-policy/cedar-go v1.8.0`, `go 1.26.7`)
- `Taskfile.yaml` (test/lint/fmt task definitions)
- Direct environment probes this session: `go version` (go1.27.1), `docker info` (available),
  `task --version` (3.52.0)

### Secondary (MEDIUM confidence — prior research docs, cross-checked against source this session
rather than trusted standalone)
- `.planning/research/ARCHITECTURE.md` (Gap 1, Gap 2, the RelatedMemories one-RPC pattern, the
  post-filtering anti-pattern, suggested build order) — cross-checked against live source; found to
  UNDER-STATE the recall-gate widening scope (misses `recallEmissionMethods`/`recallEntryPointSeeds`,
  same gap CONTEXT.md's D-15 also has) — see Pitfall 1
- `.planning/research/SUMMARY.md`, `.planning/research/PITFALLS.md` (Pitfall 6/7 — DEC-cgb and CSRF
  risk framing, both correctly scoped to Phase 3's RPC layer, not this phase)
- `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md` (phase boundary, success criteria, traceability)
- `.planning/STATE.md` (DEC-cgb standing-invariant summary, carry-forward gotchas)

### Tertiary (LOW confidence / not independently verified this session)
- Engram-memory rule ids `m45p2b4bp7`, `3p0zsqrhmb`, `xvqj44e5mk`, `xhg7dgmqx4` — summarized meaning
  taken from the invocation prompt only; no `engram` MCP tool was available in this session to
  re-fetch their exact text (see Assumption A2)
- `.planning/research/CEDAR.md` — dated 2026-07-16, predates this milestone; covers the original
  cedar-go adoption decision, not action-vocabulary extension; read for background only, not cited
  for any specific claim in this document

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependency; every library version confirmed against `go.mod` and the
  actual vendored module cache source
- Architecture: HIGH — every pattern (getWritable gate, ownerScopeFilter composition, query-by-id
  sub-query, fetchPayloadsByID enforcement point, Facet filter support) confirmed by direct source
  read this session, not training-data recall
- Pitfalls: HIGH for the recall-gate 4-list finding (confirmed by reading the full test file,
  including its own stated design-intent comments) and the rule-immutability layer finding
  (confirmed by grep + read); MEDIUM for numeric-default recommendations (explicitly Claude's
  Discretion, not verifiable facts)

**Research date:** 2026-09-25
**Valid until:** stable — this is internal Go/Qdrant code with no external API drift risk; re-verify
only if `go-client` or `cedar-go` are upgraded before this phase is planned/executed
