# Phase 1: Store Prerequisites - Context

**Gathered:** 2026-09-25
**Status:** Ready for planning

<domain>
## Phase Boundary

`internal/store` gains three pure store methods — an authz-gated Archive/Restore path,
`RelatedMemories`, and `ListTags` — plus the test infrastructure they need (the `tags` payload
index, the recall-gate allowlist widened for a filtered `Facet`). No proto, no Connect RPC, no MCP
tool, no UI: those are Phase 3 onward. Requirements: STORE-01, STORE-02, STORE-03.

</domain>

<decisions>
## Implementation Decisions

### Archive / Restore authz (STORE-01)
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

### RelatedMemories — edge semantics (STORE-02)
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

### RelatedMemories — reach and bounds (STORE-02)
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

### ListTags (STORE-03)
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

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope and research
- `.planning/REQUIREMENTS.md` — STORE-01..03 text; RPC-02/RPC-04 (the Phase 3 consumers); Out of Scope table
- `.planning/ROADMAP.md` § Phase 1: Store Prerequisites — the four success criteria
- `.planning/research/ARCHITECTURE.md` — Gap 1 (Archive/Restore authz), Gap 2 (ListTags index + recall-gate test), the `RelatedMemories` one-method pattern, the post-filtering anti-pattern
- `.planning/research/SUMMARY.md` — Phase 1 rationale and open questions (now resolved above)
- `.planning/research/PITFALLS.md` — authz-in-handler regression risk
- `.planning/notes/console-overhaul-exploration.md` — milestone decisions ("the entry point must not lie")

### Authz
- `internal/authz/authz.go` — Action constants
- `internal/authz/schema.json` — Cedar schema action list (new `archive` action goes here)
- `internal/authz/policies/*.cedar` — own_records / shared_read / defense_empty_owner
- `internal/authz/policy_corpus_test.go` — per-action policy corpus

### Store code
- `internal/store/spine.go` — `Archive`/`Restore` (subject-less, per-id lock), `NearDuplicates` (`NewQueryID` sub-query precedent, subject-less)
- `internal/store/store.go` — `getWritable`, `decideRecord`, `ownerScopeFilter`, `ensureIndexes`, `Search`, `ListScopes` (`more` flag shape), `Supersede`, `Delete`, citation payload encoding
- `internal/store/searchfetch.go` — `fetchPayloadsByID` (the enforcement point: fetch under the query's own filter)
- `internal/store/migrate_status.go` — the only existing `Facet` call (deliberately filter-less; do not copy that aspect)
- `internal/store/schemaversion_recallgate_test.go` — `recognizedFilterCarryingRequestMethods` + `recallCaptureInterceptor`
- `cmd/engram/spine_review_archive.go` — the CLI caller that must keep working unchanged

### Repo rules
- engram rules `m45p2b4bp7` and `3p0zsqrhmb` — tests verify our own behaviour; no tests of Qdrant's documented behaviour, no tests of tests

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `getWritable(ctx, id, subj, action)`: the owner-only write gate; returns `ErrNotFound` for unowned or anonymous-vs-owned.
- `Archive`/`Restore` core: per-id lock, resolve-first via `Get`, idempotent `ArchiveOutcomeAlready`, `deletePayloadKeys` seam for Restore.
- `NearDuplicates`' `qdrant.NewQueryID(qdrant.NewID(id))` sub-query with `MustNot(HasID)` self-exclusion — reuse the shape, but with the authz filter it lacks.
- `fetchPayloadsByID(ctx, f, view, ids)`: fetches payloads under the query's own filter, so a candidate the caller cannot read is never fetched.
- `ListScopes`' `(out, more, err)` return shape for the truncation signal.
- `migrateStatusFacetLimit` comment block: Facet `Limit` defaults to 10 and carries no truncation flag — the same trap applies to `ListTags`.

### Established Patterns
- Authz is compiled into the Qdrant filter in `internal/store` only (DEC-cgb); handlers never filter.
- Every store method opens a `store.<Name>` span and records `telemetry.RecordStoreOp`.
- Write-lane mutations of an existing record take `s.locker.Lock(ctx, id)` (CR-04).
- Recall gate: archived/superseded/expired/scheduled records are soft-hidden from recall, never from `Get`.

### Integration Points
- `ensureIndexes` (`store.go`) — add the `tags` keyword index.
- `internal/authz` — new action constant + schema entry + corpus rows.
- Recall-gate interceptor — add `Facet` to the recognised filter-carrying methods and type switch.
- Phase 3 will wrap these methods in `ArchiveMemory`/`RestoreMemory`/`RelatedMemories`/`ListTags` RPCs and MCP tools.

</code_context>

<specifics>
## Specific Ideas

- The tag-edge rarity weighting exists because nearly every record in this store carries the
  `engram` tag; an unweighted shared-tag edge would connect everything to everything.
- Tag counts must match what filtering by that tag returns — the milestone's "the entry point must
  not lie" rule applied to counts.

</specifics>

<deferred>
## Deferred Ideas

- Edge labels from the consolidate verdict (same fact / contradicts / similar) — already GRAPH-04, future milestone.

### Reviewed Todos (not folded)
- "Security-review then install the three design/a11y registry skills" — belongs to DSYS-03 (Phase 4), not store work.

</deferred>

---

*Phase: 01-store-prerequisites*
*Context gathered: 2026-09-25*
