# API Coverage — Qdrant Go client (points surface)

> Full coverage by default. Opt-outs are explicit, reasoned decisions.

**Scope note.** The detector fired on two signals. One is a false positive: `connect` + `mcp` matched
`03-06-PLAN.md`'s "the MCP-visible / Connect-omitted split", which is engram's **own** served
surface (`EngramService` in `proto/engram/v1/`), not a consumed one — same shape as the v0.12.x
Phase 7 declaration. The other is **real**: `03-05-PLAN.md` names Qdrant's Search Matrix API, and
this phase reaches genuinely new vendor capability in `github.com/qdrant/go-client v1.18.3` —
`QueryBatch` with `NewQueryID` landed in commit `a4949639`, the first use of batched
query-by-stored-ID in this repo.

The surface below is the vendored client's **points** API (`qdrant/points.go`), which is the whole
blast radius of the `spine-review` tier. Collection-lifecycle and snapshot APIs are out of scope:
they are owned by the store bootstrap (`internal/store/store.go:421`), not the curation tier, and
this phase does not touch them.

**On the Search Matrix rejection.** `SearchMatrixPairs`/`SearchMatrixOffsets` return exactly the
`(A, B, score)` shape D-15 wants, in a single RPC — an attractive shortcut. They are opted out
because the sampling is not documented as deterministic: the vendored client's doc comment says only
"Calculates the distances between a random sample of points", the `Sample` field comment says "How
many points to select and search within. Default is 10", and the REST reference does not state
whether the sample covers the whole filtered set. A `consolidate` report built on that would make
*absence of a pair* indistinguishable from *non-similarity* — the same false-green failure class this
repo has already been bitten by. `QueryBatch` sweeps the filtered set exhaustively instead. Full
analysis in `03-RESEARCH.md` § Common Pitfalls, Pitfall 4.

Decisions are recorded from the full-coverage baseline so the un-built surface is a recorded
decision rather than an invisible hole.

| capability | decision | reason |
|---|---|---|
| `QueryBatch` (+ `NewQueryID`) | INTEGRATE | this phase, 03-05 — near-duplicate pairs over **already-stored** vectors, batched per page, no re-embedding (`internal/store/spine.go:589`) |
| `ScrollAndOffset` | INTEGRATE | bounded cursor paging for preview enumeration and apply-time re-derivation (`internal/store/spine.go:49`) |
| `Count` | INTEGRATE | per-eligibility-class blast-radius counts driving the preview manifest (`internal/store/spine.go:128`) |
| `SetPayload` | INTEGRATE | this phase, 03-06 — stamps the orthogonal `archived_at` key for `archive`/`restore` (`internal/store/spine.go:750`) |
| `Delete` | INTEGRATE | this phase, 03-07 — `purge --apply` deletes only the preview ∩ re-derive intersection (`internal/store/spine.go:1380`) |
| `Query` (single) | INTEGRATE | pre-existing recall ranking; unchanged by this phase but inside the blast radius (same collection) |
| `Get` | INTEGRATE | pre-existing fetch-by-id backing `get_memory`; unchanged by this phase |
| `Upsert` | INTEGRATE | pre-existing write path; unchanged by this phase |
| `SearchMatrixPairs` / `SearchMatrixOffsets` | OPT-OUT | `Sample` semantics undocumented — a sampled report makes absence indistinguishable from non-similarity. Rejected as primary mechanism in `03-RESEARCH.md` § Pitfall 4; see scope note above. |
| `QueryGroups` | OPT-OUT | groups by a payload key — the clustering D-15 deliberately rejected. `consolidate` reports ranked pairs with no cluster verdict label in either output form. |
| `Facet` | OPT-OUT | would add a second counting path alongside the per-eligibility-class `Count` filters. One path keeps preview and `--apply` re-derivation comparable. |
| `Scroll` / `ScrollAll` | OPT-OUT | superseded by `ScrollAndOffset`, the only variant returning a next-page offset. Bounded paging keeps a large preview from becoming an unbounded in-memory read. |
| `ClearPayload` / `DeletePayload` / `OverwritePayload` | OPT-OUT | archive stamps one key (`archived_at`) and must stay orthogonal to `superseded_by` (D-12). Whole-payload mutation could clear a supersession link as a side effect. |
| `UpdateBatch` | OPT-OUT | `purge --apply` re-derives at apply time and deletes only the intersection with the preview; interleaving other mutation kinds would make that intersection non-auditable. |
| `UpdateVectors` / `DeleteVectors` / `CreateVectorName` / `DeleteVectorName` | OPT-OUT | the collection uses a single unnamed vector (`qdrant.NewVectorsConfig(&qdrant.VectorParams{…})`, `internal/store/store.go:423`) — there is no named vector to manage. |
| `CreateFieldIndex` / `DeleteFieldIndex` | OPT-OUT | payload-index schema is owned by the store bootstrap (`internal/store/store.go:455-462`), created idempotently at connect. A curation verb mutating it would make collection shape order-dependent. |
