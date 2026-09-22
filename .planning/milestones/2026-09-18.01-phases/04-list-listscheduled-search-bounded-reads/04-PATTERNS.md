# Phase 4: List, ListScheduled & Search Bounded Reads - Pattern Map

**Mapped:** 2026-09-19
**Files analyzed:** 27
**Analogs found:** 25 / 27

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/store/store.go` (`Store.List` offset mode) | service | CRUD (paginated read) | `internal/store/orderedpage.go` (`scrollOrderedPage`) | exact (D-05 loop composes it directly) |
| `internal/store/store.go` (`listByCursor`) | service | CRUD (cursor read) | `internal/store/orderedpage.go` (`scrollOrderedPage`) | exact (thin adapter, Pattern 1) |
| `internal/store/store.go` (`ListScheduled`) | service | CRUD (paginated read) | `internal/store/orderedpage.go` (`scrollOrderedPage`) via the same D-05 loop as offset-mode `List` | exact |
| `internal/store/store.go` (`Search`/`SearchReranked`/`SearchDiscovery`) | service | request-response (two-phase fetch) | `internal/store/orderedpage.go` (`excludeSeen` — INCLUDE-filter sibling) + `store.go:1189` `memoriesFromPoints` (score re-attach) | role-match (new mechanism, proven idiom) |
| `internal/store/searchfetch.go` (new) | service/utility | batch (has_id fetch) | `internal/store/orderedpage.go` (`excludeSeen`, file-doc-comment structure) | role-match |
| `internal/store/boundedread.go` (new `keysView`) | utility | transform (per-view sizing) | `internal/store/boundedread.go` (`fullView`/`summaryView`) — same file, sibling constructor | exact |
| `internal/store/schemaversion_recallgate_test.go` | test | event-driven (AST/reflection gate) | itself — `recallTransmitters`/`otherNonRecallEmitters` entries already exist for `Store.scrollOrderedPage` and every List/Search method | exact |
| `internal/store/*_test.go` (new, per-site regressions) | test | integration (real Qdrant) | `internal/store/listscopes_oversized_test.go` (`TestListScopesFullPayloadsOverGRPCLimit`) | exact |
| `internal/store/export_test.go` | test (shim) | — | itself — `ScrollOrderedPage`/`SetByteBudgets`/`FullView`/`SummaryView` shims | exact |
| `internal/server/tools.go` (`coreListRequest`/`coreSearchRequest` `Full`; `deps.listMemory`/`searchMemory`/`searchDiscovery`/`listScheduled`) | controller (typed core) | request-response | itself — `coreListRequest`/`coreSearchRequest` (`tools.go:1543-1611`) | exact (extend existing struct) |
| `internal/server/rules.go:206-212` | controller | CRUD | `internal/server/tools.go:1621-1644` (`deps.listMemory`, the `Full`-threading precedent) | role-match (direct `Store.List` call bypassing the typed core) |
| `internal/server/connectapi.go` (`ListMemories`/`SearchMemories`/`SearchDiscoveries` max rejection) | controller | request-response | itself — `connectapi.go:235-311` (`ListMemories`'s existing `cursor_mode`/`offset` rejection idiom) | exact |
| `internal/server/argerror.go` (`HintOutOfRange`, rename `HintTooLarge`) | model/const catalog | — | itself — `HintCode` catalog block | exact |
| `internal/server/connecterror.go` | controller (error mapper) | — | itself — `connectError`'s `errors.As(err, &ae)` arm | exact |
| `internal/server/responsetoolarge.go` + tests | utility (shared envelope) | — | itself — `responseTooLargeEnvelope`/`renderHintEnvelope` | exact |
| `internal/server/argattribution_test.go`, `connectargerror_test.go`, `hintcodedocs_test.go`, `responsetoolarge_test.go` | test | — | themselves (existing hint-code conformance suite) | exact |
| `cmd/engram/client_list.go`, `client_search.go` (help text) | CLI/config | — | themselves — existing `--limit`/`--k` flag `Usage` strings | exact |
| `cmd/engram/exitcode_baseline_test.go` | test | — | itself | exact |
| docs-site + proto comments + CLAUDE.md + PROJECT.md | config/docs | — | themselves | exact |
| `internal/store/redevidence_harness_test.go` (rename + Phase 4 entries) | test (registry) | — | itself — `redEvidenceDirs` map | exact |
| `.planning/phases/02-.../red-evidence/02-03-errors-doc-drops-too-large.patch` | — | — | itself (regenerate) | exact |

## Pattern Assignments

### `internal/store/store.go` — `Store.List` offset mode, `Store.ListScheduled` (D-05 loop)

**Analog:** `internal/store/orderedpage.go` (`scrollOrderedPage`) + Phase 3's own illustrative loop already given in RESEARCH.md Pattern 2.

**The primitive's exact contract to compose against** (`internal/store/orderedpage.go:47-70`):
```go
type orderedPage struct {
    Items       []Memory
    Next        listCursor // always the resume position; == from when nothing emitted
    Exhausted   bool       // true only when Qdrant genuinely has no more matches
    CutByBudget bool       // true only when pageByteBudget stopped the page early
    Bytes       int
}

func (s *Store) scrollOrderedPage(ctx context.Context, f *qdrant.Filter, view readView,
    dir qdrant.Direction, from listCursor, limit uint64) (orderedPage, error)
```

**D-05 assembly loop** (illustrative shape confirmed against `scrollOrderedPage`'s contract; exact identifiers are the executor's discretion):
```go
var items []Memory
from := listCursor{} // zero value: first page
for uint64(len(items)) < want {
    page, err := s.scrollOrderedPage(ctx, f, view, dir, from, want-uint64(len(items)))
    if err != nil {
        return nil, err
    }
    items = append(items, page.Items...)
    from = page.Next
    if page.Exhausted {
        break
    }
    // CutByBudget: loop again immediately — D-05 revises Phase 3 D-06:
    // pageByteBudget bounds cursor-mode responses ONLY.
}
```

**What `Store.List`'s offset mode replaces** (today's unbounded shape, `internal/store/store.go:1458-1491`):
```go
// Offset mode: Qdrant has no numeric OFFSET, so scroll offset+limit ordered
// records and return the trailing limit.
fetch := opts.Offset + opts.Limit
if opts.Limit == 0 {
    fetch = total // limit 0 = "all" (preserves prior behavior)  <-- D-01 removes "all"; becomes min(total, maxRecallLimit)
}
...
pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
    CollectionName: s.collection, Filter: f, Limit: qdrant.PtrOf(uint32(fetch)),
    OrderBy: &qdrant.OrderBy{Key: "created_at", Direction: qdrant.PtrOf(dir)},
    WithPayload: qdrant.NewWithPayload(true), // <-- unbounded full payload; D-05 replaces this ONE Scroll with the loop above
})
```
This single unbounded `Scroll` is exactly what the D-05 loop over `scrollOrderedPage` replaces — same `Filter`, same `OrderBy` shape, same `dir` (`qdrant.Direction_Desc`/`Asc` from `opts.Ascending`), but sized per-RPC from `view.maxRecordBytes` instead of one `fetch`-sized request. The trailing `all[opts.Offset:]` slice (`store.go:1484-1491`) is unchanged — it now slices the loop's `items` instead of a single Scroll's `pts`.

**`ListScheduled`'s existing Scroll to rewire onto the same loop** (`internal/store/store.go:1680-1687`; ordering already matches `scrollOrderedPage`'s expectations — desc `created_at`, no `StartFrom` on the first call):
```go
pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
    CollectionName: s.collection, Filter: f,
    Limit:       qdrant.PtrOf(uint32(limit)),
    OrderBy:     &qdrant.OrderBy{Key: "created_at", Direction: qdrant.PtrOf(qdrant.Direction_Desc)},
    WithPayload: qdrant.NewWithPayload(true),
})
```
Per Pitfall 6 / RESEARCH.md: `ListScheduled` has no full/summary distinction to preserve — always request `s.fullView()` here, bounded via the D-05 loop, never a new caller-facing knob.

**Error handling:** unchanged — `scrollOrderedPage` already returns the raw error (including `ErrResponseTooLarge` after its own batch-of-1 fallback exhausts) unwrapped; the D-05 loop propagates it as-is, exactly like today's single-Scroll `if err != nil { return nil, 0, "", err }` (`store.go:1481-1483`).

---

### `internal/store/store.go` — `listByCursor` → thin `scrollOrderedPage` adapter (Pattern 1)

**Analog:** `internal/store/orderedpage.go` (`scrollOrderedPage`'s doc comment, `orderedpage.go:14-20`) — the mapping is correct by construction, not a new branch:

```go
// orderedPage.Next is populated whenever len(Items) > 0 (orderedpage.go:214-217).
// page.Exhausted -> nextCursor = ""
// otherwise      -> nextCursor = encodeCursor(page.Next)   // REGARDLESS of CutByBudget (D-06)
```

**Today's hand-rolled cursor loop to replace** (`internal/store/store.go:1502-1585`, `listByCursor`) already builds `listCursor{C, Seen}` and calls `encodeCursor`/`decodeCursor` (`internal/store/cursor.go`) — those two functions are UNCHANGED; only the fetch-and-boundary-tracking body is replaced by one `s.scrollOrderedPage(ctx, f, view, dir, from, limit)` call. `maxListLimit` (renamed per D-02, see below) still caps `limit` and a decoded cursor's `Seen` set exactly as today (`store.go:1507-1509,1518-1520`).

**Cursor codec, unchanged** (`internal/store/cursor.go:12-36`):
```go
type listCursor struct {
    C    string   `json:"c"`
    Seen []string `json:"seen"`
}
func encodeCursor(c listCursor) string { ... }
func decodeCursor(tok string) (listCursor, error) { ... }
```

---

### `internal/store/searchfetch.go` (new) — two-phase search fetch helper (D-09)

**Analog:** `internal/store/orderedpage.go:72-92` (`excludeSeen`) — the INCLUDE-filter sibling this phase needs is a structural mirror (`Must` instead of `MustNot`):

```go
// excludeSeen — the EXCLUDE-shaped precedent to invert:
func excludeSeen(f *qdrant.Filter, ids []string) *qdrant.Filter {
    if len(ids) == 0 {
        return f
    }
    pointIDs := make([]*qdrant.PointId, len(ids))
    for i, id := range ids {
        pointIDs[i] = qdrant.NewID(id)
    }
    var must []*qdrant.Condition
    if f != nil {
        must = []*qdrant.Condition{qdrant.NewFilterAsCondition(f)}
    }
    return &qdrant.Filter{Must: must, MustNot: []*qdrant.Condition{qdrant.NewHasID(pointIDs...)}}
}
```

**INCLUDE sibling to author** (RESEARCH.md's verified pattern, `internal/store/orderedpage.go:76-92` structural precedent):
```go
func includeIDs(f *qdrant.Filter, ids []string) *qdrant.Filter {
    pointIDs := make([]*qdrant.PointId, len(ids))
    for i, id := range ids {
        pointIDs[i] = qdrant.NewID(id)
    }
    var must []*qdrant.Condition
    if f != nil {
        must = []*qdrant.Condition{qdrant.NewFilterAsCondition(f)}
    }
    must = append(must, qdrant.NewHasID(pointIDs...))
    return &qdrant.Filter{Must: must}
}
```
**Critical constraint (V4 Access Control):** this wraps the caller's filter as a nested `Must` condition — never `Should`/`MustNot` at the top level — so an id from another owner's record cannot pass the AND even if included in the batch. Copy `excludeSeen`'s narrow-only structure exactly; do not hand-roll a parallel filter.

**Score re-attachment precedent** (`internal/store/store.go:1189-1197`, `memoriesFromPoints` — today's single-phase version to adapt):
```go
func memoriesFromPoints(res []*qdrant.ScoredPoint) []Memory {
    out := make([]Memory, 0, len(res))
    for _, p := range res {
        m := fromPayload(p.Id.GetUuid(), p.Payload)
        m.Score = p.Score
        out = append(out, m)
    }
    return out
}
```
Two-phase adaptation: phase 1 (`Query` with `WithPayload: qdrant.NewWithPayload(false)`) builds `map[string]float32` from `[]*ScoredPoint`; phase 2's `fromPayload(id, fetchedPayload)` + a map lookup replaces `p.Score` directly, then `sort.SliceStable` by the held score descending (phase 2's `Scroll` order is NOT rank order).

**Mandatory early-exit** (Pitfall 1 — `internal/store/schemaversion_recallgate_test.go:982-1042`'s six search rows assert `expectCount:1, expectMethods:["Query"]` against a zero-record fixture): the fetch helper's FIRST line must be `if len(ids) == 0 { return nil, nil }` before any RPC — never "fix" the test's expected counts instead.

**Batching:** one `Scroll` per chunk of `perRPCLimit(view)` ids (`internal/store/boundedread.go:194-202`, `perRPCLimit`), in the view selected per Pattern 5 below (`s.fullView()` unconditionally for `SearchReranked`'s ≤100-candidate pool; caller's `full` flag for `SearchDiscovery`).

**File header convention** to copy verbatim in style (from `internal/store/orderedpage.go:1-33`, the Phase 3 precedent this file mirrors):
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file hosts <what/why, referencing 04-CONTEXT.md D-09>...
```

---

### `internal/store/boundedread.go` — new `keysView` (D-07 deep-offset walk)

**Analog:** same file's `fullView`/`summaryView` constructors (`internal/store/boundedread.go:216-228`):
```go
func (s *Store) fullView() readView {
    return readView{selector: qdrant.NewWithPayload(true), maxRecordBytes: fullRecordCeiling(s.RecordCaps())}
}

func (s *Store) summaryView() readView {
    return readView{
        selector:       qdrant.NewWithPayloadExclude("content", "citations"),
        maxRecordBytes: summaryRecordCeiling(s.RecordCaps()),
    }
}
```
**New sibling** (Option A, recommended — `qdrant.NewWithPayloadInclude("created_at")`, reusing `fromPayload`'s existing "created_at" parse, `internal/store/store.go:717-757`, and `scrollOrderedPage`'s existing boundary tracking, `orderedpage.go:188-197`, with NO new decode logic):
```go
func keysView() readView {
    return readView{selector: qdrant.NewWithPayloadInclude("created_at"), maxRecordBytes: keysRecordCeiling}
}
```
`keysRecordCeiling` is a small fixed constant (~256 bytes/record, per RESEARCH.md Pattern 3) — NOT derived from `RecordCaps` (content/tags/citations are excluded entirely), documented the same way `uncappedFieldsAllowance` is documented above it (`boundedread.go:148-155`). Do NOT adopt Option B (`RetrievedPoint.OrderValue`) without a live empirical check first — RESEARCH.md Assumption A1 flags its encoding as unconfirmed.

**Boundary handoff:** the keys-only walk's own `orderedPage.Next` (a `listCursor`) feeds DIRECTLY as the `from` argument to the caller-view `scrollOrderedPage` call — no translation needed (both walks share the identical `created_at`/`seen`-id keyset shape).

**Doc-comment revision required** (D-05 explicitly revises Phase 3 D-06): `boundedread.go`'s file-level doc comment (`internal/store/boundedread.go:1-26`) must be updated to state `pageByteBudget` bounds cursor-mode responses ONLY, per D-05.

---

### `internal/server/tools.go` — `Full` threading through `coreListRequest`/`coreSearchRequest` (Pattern 6)

**Analog:** the structs themselves — extend, don't replace (`internal/server/tools.go:1543-1567,1586-1611`):
```go
type coreListRequest struct {
    Scope         string
    Limit         uint64
    Offset        uint64
    Categories    []string
    Visibility    string
    Tags          []string
    CreatedAfter  time.Time
    CreatedBefore time.Time
    Cursor        string
    CursorMode    bool
    CrossSpine    bool
    IncludeArchived   bool
    IncludeSuperseded bool
    IncludeScheduled  bool
    // Full bool  <-- NEW field this phase adds
}
```
**Wiring precedent for a boolean threaded from BOTH lanes** — the existing `IncludeArchived`/`IncludeSuperseded`/`IncludeScheduled` trio already does exactly this: declared on the core struct (doc comment explicitly says "Connect and the CLI only — the MCP closure below never sets these"), copied straight into `store.ListOptions`/`SearchOptions` inside `deps.listMemory`/`searchMemory` (`tools.go:1626-1638,1708-1716`), and read at both the MCP closure (`tools.go:2627-2638`, sets zero value implicitly) and the Connect handler (`internal/server/connectapi.go:295-306`, `IncludeArchived: req.Msg.IncludeArchived`). `Full` follows the SAME threading shape, except MCP DOES set it (`a.Full`) unlike the trio.

**`deps.listMemory`, the exact call site to extend** (`internal/server/tools.go:1621-1644`):
```go
func (d *deps) listMemory(ctx context.Context, c caller, req coreListRequest) (coreListResult, error) {
    scope, err := effectiveSearchScope(req.Scope, req.CrossSpine)
    if err != nil {
        return coreListResult{}, err
    }
    ms, total, next, err := d.st.List(ctx, scope, c.Subj, store.ListOptions{
        Limit: req.Limit, Offset: req.Offset, Categories: req.Categories,
        Visibility: req.Visibility, Tags: req.Tags,
        CreatedAfter: req.CreatedAfter, CreatedBefore: req.CreatedBefore,
        Cursor: req.Cursor, CursorMode: req.CursorMode,
        IncludeArchived: req.IncludeArchived, IncludeSuperseded: req.IncludeSuperseded,
        IncludeScheduled: req.IncludeScheduled,
        // Full: req.Full,  <-- NEW
    })
    ...
}
```

**MCP closure wiring point** (`internal/server/tools.go:2627-2638`, `list_memory`) — add `Full: a.Full` to the `coreListRequest{}` literal alongside the existing `CrossSpine: a.CrossSpine`.

**Connect handler wiring point** (`internal/server/connectapi.go:274-286`, `ListMemories`) — add `Full: req.Msg.Full` to the `coreListRequest{}` literal; the field is already read separately, one line later, for RESPONSE shaping only (`shapeProtoMemories(res.Memories, req.Msg.Full, ...)`, `connectapi.go:302`) — this phase makes it ALSO select the fetch view.

**`search_memory`'s override (Pattern 5)** — `SearchReranked`'s fetch phase must ALWAYS use `fullView()` regardless of the caller's `full`, because `RerankHits`/`lexicalOverlap` needs `Content` for all ≤100 candidates (`internal/store/rerank.go:16-25,47-56`):
```go
// internal/store/rerank.go:16-25 — the 100-candidate hard ceiling
func candidateK(k uint64) uint64 {
    c := k * 4
    if c < 32 { c = 32 }
    if c > 100 { c = 100 }
    return c
}
```
`search_discovery`/`SearchDiscoveries` has no reranker (`deps.searchDiscovery` calls `Store.SearchDiscovery` directly, `tools.go:1841`) — its fetch view follows the caller's `full` flag directly, same as List.

---

### `internal/server/rules.go:206-212` — the one direct `Store.List` call (Pattern 6 step 4, do-not-miss)

**Analog:** `internal/server/tools.go:1621-1644` (`deps.listMemory`'s `Full` threading, above) — apply the SAME field, but `rules.go` bypasses the typed core entirely, so it must be edited in this same change or it silently regresses to `summaryView()`-only content for `full=true` rule reads.

**Exact call site to edit** (`internal/server/rules.go:206-212`):
```go
ms, _, _, lerr := d.st.List(ctx, sc, c.Subj, store.ListOptions{
    Limit:      0,
    Ascending:  true,
    Categories: []string{"rule"},
    Tags:       a.Tags,
    // Full: a.Full,  <-- NEW — listRulesArgs.Full already exists (rules.go);
    //                     toRuleView (rules.go:181-186) already shapes
    //                     compact-vs-full downstream from a.Full
})
```
`toRuleView` (`internal/server/rules.go:181-186`) already does the compact/full downstream shaping identical to `shapeRecall` — only the FETCH view was missing this wiring.

---

### `internal/server/connectapi.go` — `limit`/`k` `out_of_range` maximum check (D-10)

**Analog:** `ListMemories`'s existing relational-rejection idiom (`internal/server/connectapi.go:255-262`), the closest same-handler precedent for a fail-fast boundary check that hands a classified error straight to `connectError`:
```go
if req.Msg.CursorMode && req.Msg.Offset > 0 {
    rule, _ := surfaces.RuleByID(surfaces.RulePagingMutuallyExclusive)
    return nil, connectError(ctx, conditionalErrf(classPrecondition, rule))
}
```
**D-10's shape to add** (per Pitfall 3 — `classMalformed`, NOT `classOutOfRange`, despite the hint's name):
```go
if req.Msg.Limit > maxRecallLimit {  // renamed maxListLimit, D-02
    return nil, connectError(ctx, argErrf(classMalformed, HintOutOfRange, "limit",
        "limit must not exceed %d", maxRecallLimit))
}
```
Same shape for `k` on `SearchMemories`/`SearchDiscoveries` (`internal/server/connectapi.go:326-336,420-424`) and for `limit`/`k` in the MCP closures/`deps.*` (`tools.go`). Position per V4/DoS guidance (RESEARCH.md Security Domain): before scope resolution/embedding — mirrors `ListMemories`'s own check ordering (this check sits alongside the existing `cursor_mode`/`offset` and `effectiveSearchScope` checks, `connectapi.go:255-273`).

---

### `internal/server/argerror.go` — `HintOutOfRange` + rename `HintTooLarge` (D-10, D-11)

**Analog:** the `HintCode` catalog itself (`internal/server/argerror.go:33-45`):
```go
const (
    HintRequired            HintCode = "required"
    HintConditionalRequired HintCode = "conditional_required"
    HintTooLong             HintCode = "too_long"
    HintTooMany             HintCode = "too_many"
    HintEnum                HintCode = "enum"
    HintFormat              HintCode = "format"
    HintPrefix              HintCode = "prefix"
    HintOrdering            HintCode = "ordering"
    HintMutuallyExclusive   HintCode = "mutually_exclusive"
    HintNotApplicable       HintCode = "not_applicable"
    HintTooLarge            HintCode = "too_large"       // <-- RENAME to HintResponseTooLarge = "response_too_large" (D-11)
    // HintOutOfRange       HintCode = "out_of_range"     // <-- NEW (D-10)
)
```
Both changes ride the same commit per D-11's own instruction: `internal/server/{argerror,connecterror,responsetoolarge}.go` and their tests, `cmd/engram/exitcode_baseline_test.go`, docs, and the red-evidence patch all move together (see Pitfall 4).

**D-10's classification — use `argErrf` exactly as it already exists** (`internal/server/argerror.go:135-145`):
```go
func argErrf(class argClass, hint HintCode, field, format string, a ...any) error {
    return &argError{Fields: []string{field}, Hint: hint, Detail: fmt.Sprintf(format, a...), Class: class}
}
```
Call as `argErrf(classMalformed, HintOutOfRange, "limit", "limit must not exceed %d", maxRecallLimit)` — `classMalformed` (NOT the more "obvious" `classOutOfRange`) per Pitfall 3; `classOutOfRange` already exists in `argClass` (`argerror.go:57-59`) but D-10 explicitly locks `classMalformed` because both classes group under CLI `exitUsage` (2) identically.

---

### `internal/server/connecterror.go` — rename `HintTooLarge`'s reference (D-11)

**Analog:** itself — `connectError`'s `store.ErrResponseTooLarge` arm (`internal/server/connecterror.go:103-105`):
```go
case errors.Is(err, store.ErrResponseTooLarge):
    slog.ErrorContext(ctx, "connect handler: response too large", "error", err)
    return connect.NewError(connect.CodeResourceExhausted, errors.New(responseTooLargeEnvelope()))
```
No structural change — `responseTooLargeEnvelope()` (`responsetoolarge.go`) is the one place that must switch from `HintTooLarge` to `HintResponseTooLarge`; this call site is untouched.

---

### `internal/server/responsetoolarge.go` — rename the envelope constant (D-11)

**Analog:** itself (`internal/server/responsetoolarge.go:30-46`):
```go
const responseTooLargeDetail = "the result is too large to return in one response; retry with a smaller limit or k, or omit full"

func responseTooLargeEnvelope() string {
    return renderHintEnvelope([]string{"response"}, HintTooLarge, responseTooLargeDetail)  // <-- HintTooLarge -> HintResponseTooLarge
}
```
Both `mapResponseTooLarge` (the MCP middleware, same file, lines 48-84) and `connectError` (above) call `responseTooLargeEnvelope()` — one rename here propagates to both lanes, per the file's own design comment ("never a second hand-built string, D-04").

---

### `internal/store/schemaversion_recallgate_test.go` — recall-gate reclassification

**Analog:** itself — `otherNonRecallEmitters`' existing pre-positioned entry (`internal/store/schemaversion_recallgate_test.go:586-589`), which already anticipates this exact move:
```go
{
    enclosingFunc: "Store.scrollOrderedPage",
    justification: "Emits Scroll (orderedpage.go). The shared ordered-page primitive ... is not yet wired into any recall entry point, so it is unreachable from recallEntryPointSeeds. Phase 4 moves it to recallTransmitters in the same change that wires Store.List/listByCursor/ListScheduled onto it; if anything reaches it from a seed before then, reachability pulls it in, it stops matching this entry, and the suite goes RED.",
},
```
**Action:** delete this entry from `otherNonRecallEmitters` (`schemaversion_recallgate_test.go:577-590`) and add a new entry to `recallTransmitters` (`schemaversion_recallgate_test.go:486-511`), in the exact same shape as its existing siblings:
```go
{
    enclosingFunc: "Store.scrollOrderedPage",
    justification: "Emits Scroll (orderedpage.go). Wired this phase into Store.List (offset+cursor), Store.ListScheduled. Reachable from every List/ListScheduled seed.",
},
```
Add a second new `recallTransmitters` entry for the new search fetch helper (`searchfetch.go`), justified the same way, and (if the deep-offset walk gets its own named function) a third for the keys-only walk. `recallEntryPointSeeds` (`schemaversion_recallgate_test.go:355+`) and `recallInvocationRows` (`:982+`) do NOT need new rows for these — they are reached transitively from the existing `Store.List`/`Store.Search`/etc. seed rows; only their CLASSIFICATION changes. Per Pitfall 1, do not touch `recallInvocationRows`' `expectCount`/`expectMethods` for the six search rows unless the fetch helper's empty-batch short-circuit is missing.

---

### `internal/store/*_test.go` (new per-site regressions)

**Analog:** `internal/store/listscopes_oversized_test.go` (full file read above) — copy its exact shape: `package store_test`, dial with `storetest.RecvLimit`, seed via `storetest.SeedOversized` at both `storetest.FewLarge`/`storetest.ManySmall` shapes, subtest per shape named by `Shape.String()`, assert the operation succeeds and every seeded record is accounted for:
```go
package store_test

import (
    "context"
    "testing"

    "github.com/google/uuid"
    "github.com/seanb4t/engram/internal/store"
    "github.com/seanb4t/engram/internal/store/storetest"
)

func TestStoreListOffsetBounded(t *testing.T) {
    shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
    for _, shape := range shapes {
        t.Run(shape.String(), func(t *testing.T) {
            c := storetest.Dial(t, storetest.RecvLimit)
            name := store.PrefixedTestCollection("oversized_list_" + uuid.NewString())
            st := store.NewTestStore(t, c, name)
            ctx := context.Background()
            if err := st.EnsureCollection(ctx, 3); err != nil {
                t.Fatalf("EnsureCollection: %v", err)
            }
            t.Cleanup(func() {
                if err := c.DeleteCollection(ctx, name); err != nil {
                    t.Errorf("DeleteCollection(%q): %v", name, err)
                }
            })
            fx := storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3}})
            items, total, _, err := st.List(ctx, fx.Scope, store.Authenticated(fx.Owner), store.ListOptions{Limit: 0})
            if err != nil {
                t.Fatalf("List: %v (request shape exceeded the %d-byte named receive limit)", err, storetest.RecvLimit)
            }
            if total != uint64(len(fx.IDs)) {
                t.Errorf("total = %d, want %d", total, len(fx.IDs))
            }
            _ = items
        })
    }
}
```
For the D-06 cut-page test (shrunken `pageByteBudget`), use `store.SetByteBudgets` (`export_test.go:89-96`, already a `t.Cleanup`-restoring shim) exactly as Phase 3's own precedent does — see `internal/store/orderedpage_oversized_test.go` (functions `TestScrollOrderedPageByteBudget`, `TestScrollOrderedPageBudgetBoundaries`) for the shrunken-budget assertion idiom (not reproduced in full here — read those two functions directly when authoring the cut-page test).

**Test shim reuse — `export_test.go`** (`internal/store/export_test.go`, full file read above): every new `package store_test` regression reaches internal-only symbols (`ReadView`, `FullView()`, `SummaryView()`, `ScrollOrderedPage`, `SetByteBudgets`, `MaxListLimit`) through this file's existing shims. If the deep-offset keys view or the new fetch helper need test-only exposure, add a new shim HERE, following the exact one-line-doc-comment convention every existing shim uses (e.g. `// KeysView exposes ... to package store_test.`).

---

## Shared Patterns

### License header (every new Go file)
**Source:** every existing file in this map (`orderedpage.go:1-2`, `boundedread.go:1-2`, etc.)
**Apply to:** `internal/store/searchfetch.go` and every new `_test.go` file.
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
```
Per CLAUDE.md: never add this header above a file whose first line must be `---` YAML frontmatter (not applicable to any file in this phase's Go set, but applicable-adjacent for `.planning/**` — do not add it there).

### `package store_test` for storetest-importing tests
**Source:** `internal/store/listscopes_oversized_test.go:1-20`, `internal/store/export_test.go` doc comment (`:13-19`)
**Apply to:** every new real-Qdrant regression test in this phase (List/ListScheduled/Search/SearchDiscovery bounded-read proofs). `internal/store` (package `store`) cannot import `storetest` (import cycle) — a test that dials `storetest.Dial`/seeds via `storetest.SeedOversized` MUST live in `package store_test` and reach internal symbols only via `export_test.go`'s shims.

### `argErrf` + the single `connectError` mapper — never hand-roll a rejection
**Source:** `internal/server/argerror.go:135-145` (`argErrf`), `internal/server/connecterror.go:62-110` (`connectError`)
**Apply to:** every `out_of_range` rejection this phase adds (Connect `ListMemories`/`SearchMemories`/`SearchDiscoveries`, MCP `deps.listMemory`/`listScheduled`/`searchMemory`/`searchDiscovery`, and the store-level `ErrInvalidArgument` backstop in `maxListLimit`'s new rejection path). Never construct a `connect.NewError` directly at a handler boundary — always `argErrf(...)` then hand the result to `connectError(ctx, err)` (or return the plain error from a `deps.*` method, which the MCP lane renders via `(*argError).Error()`'s `renderHintEnvelope`).

### Hint codes declared once, cross-checked mechanically
**Source:** `internal/server/argerror.go:33-45` (`HintCode` catalog), `internal/server/hintcodedocs_test.go` (not fully reproduced — read directly when editing `errors.md`)
**Apply to:** `HintOutOfRange` (new) and `HintResponseTooLarge` (rename). Per Pitfall 4: `hintcodedocs_test.go` only asserts the HEADING's count word and the table's code SET — grep the WHOLE `errors.md` file for "eleven" after the edit (5 occurrences: frontmatter line 3, body lines 106/153/155, heading 111) and fix every one, not just what the test catches.

### One documented maximum, one constant
**Source:** `internal/store/store.go:1494-1497` (`maxListLimit`)
**Apply to:** rename/alias to `maxRecallLimit` (RESEARCH.md's recommended name — reads correctly at List/ListScheduled/Search/SearchDiscovery/list_rules call sites) per D-02. `internal/store/export_test.go:36-39`'s `MaxListLimit` shim must be renamed alongside it (or aliased) so `storetest.ManySmallRecords`'s pin (`listscopes_oversized_test.go:64-68`, `TestManySmallShapeFitsOneListPage`) keeps compiling.

### Red-evidence: one patch per RED direction, registered in `redEvidenceDirs`
**Source:** `internal/store/redevidence_harness_test.go:106+` (`redEvidenceDirs` map), its existing Phase 2 entry: `"02-03-errors-doc-drops-too-large.patch": "TestErrorsDocHintCodesMatchArgErrorConstants"`
**Apply to:** regenerate that patch under the `HintResponseTooLarge` rename (D-11), and add one new patch per Phase 4 RED direction, registered after the phase's last plan and before verification (playbook `f7zdc18tn3` #12-13).

### D-12 regression retargeting — record every retarget in SUMMARY deviations
**Source:** `internal/server/responsetoolarge_test.go:166` and its Connect/MCP siblings (not fully reproduced here — read directly when retargeting), which today exploit `Store.List`'s `Limit:0` → unbounded single-Scroll short-circuit (`store.go:1461-1462`) to prove `ErrResponseTooLarge`. Once D-01/D-05 land, that premise is gone (Pitfall 2). Retarget to the D-07 batch-of-1 fallback path (a single legacy record whose payload alone exceeds `storetest.RecvLimit`) through a normal recall entry point, mirroring `scrollOrderedPage`'s own already-proven batch-of-1 fallback test (`internal/store/store_test.go`, `TestScrollOrderedPageBatchOfOneFallback` — read directly for the exact fixture shape).

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `internal/store/boundedread.go` (`keysView`'s exact byte constant) | utility | transform | No prior keys-only (id + one timestamp field) view exists in this codebase; RESEARCH.md Assumption A2 flags the ~256-byte estimate as unmeasured — self-correcting at runtime via `proto.Size`, not a planning blocker. |

## Metadata

**Analog search scope:** `internal/store/*.go`, `internal/server/{tools,connectapi,rules,argerror,connecterror,responsetoolarge}.go`, `cmd/engram/client_{list,search}.go`, existing `*_oversized_test.go` regressions, `internal/store/export_test.go`, `internal/store/schemaversion_recallgate_test.go`, `internal/store/redevidence_harness_test.go`.
**Files scanned:** ~20 source files read in full or targeted sections (store.go ~600 lines across 2 reads, orderedpage.go/boundedread.go/cursor.go/export_test.go/listscopes_oversized_test.go in full, argerror.go/connecterror.go/responsetoolarge.go in full, tools.go/connectapi.go/rules.go targeted sections, schemaversion_recallgate_test.go targeted sections).
**Pattern extraction date:** 2026-09-19
