# Phase 1: Store Prerequisites - Pattern Map

**Mapped:** 2026-09-25
**Files analyzed:** 9 (2 new store files, 2 new-suggested test files beyond that, 5 modified existing files)
**Analogs found:** 9 / 9

All analogs re-verified directly this session (`Read`/`grep -n`) against the working tree, not
taken on RESEARCH.md's word alone — line numbers below match current source. Every analog path is
git-tracked (`git ls-files` confirmed for all pre-existing files below); the five new files have no
existing path to check.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/store/spine.go` (add `ArchiveAs`/`RestoreAs`) | service (store method) | CRUD (gated single-record write) | `internal/store/spine.go` `Archive`/`Restore` (same file, lines 773-876) + `internal/store/store.go` `getWritable` (2195-2208) | exact — same file, same core, adds the gate `Delete`/`SetVisibility` already use |
| `internal/authz/authz.go` (add `ActionArchive` const) | config (policy vocabulary) | CRUD | `internal/authz/authz.go` lines 22-29 (existing `Action` const block) | exact — one-line addition to an existing const block |
| `internal/authz/schema.json` (add `"archive"` to actions, doc-only) | config | — | same file, existing action list | exact — mechanical doc update, not CI-gated |
| `internal/authz/policy_corpus_test.go` (widen `allActions` + inline deny list) | test | request-response (policy decision) | same file, lines 14 and 34-40 | exact — widen two existing lists in place |
| `internal/store/relatedmemories.go` (NEW) — `RelatedMemories` | service (store method) | CRUD / transform (multi-edge query + merge) | `internal/store/spine.go` `NearDuplicates` (576-660, query-by-id sub-query) + `internal/store/store.go` `Search` (1190-1249, authz filter composition) + `internal/store/searchfetch.go` `fetchPayloadsByID` (enforcement fetch) | role-match — no single existing method combines all three shapes, but each ingredient exists verbatim |
| `internal/store/listtags.go` (NEW) — `ListTags` | service (store method) | CRUD (facet aggregation) | `internal/store/migrate_status.go` (Facet call, ~line 143) + `internal/store/store.go` `ListScopes` (2037-2072, `more`-flag shape + authz filter) | role-match — Facet shape from one analog, filter+truncation shape from another |
| `internal/store/store.go` (`ensureIndexes`, add `tags` index) | config (bootstrap) | batch (idempotent index creation) | same function, lines 657-678 | exact — append one `idx` struct literal |
| `internal/store/schemaversion_recallgate_test.go` (widen 4 lists) | test | event-driven (AST/interceptor completeness gate) | same file: `recognizedFilterCarryingRequestMethods` (868), `recallCaptureInterceptor` (891-905), `recallEmissionMethods` (334-340), `recallEntryPointSeeds` (355-362), `recallTransmitters` (486-511) | exact — widen four existing lists/switches in place, model the new rows on the `ListScopes` rows |
| `internal/store/archive_authz_test.go` (NEW, suggested) | test | request-response (authz regression) | `internal/store/store_test.go` `TestDeleteOwnerGate` (836-858) | exact — same shape: owner allow, shared-non-owner deny, record-unchanged assertions |
| `internal/store/relatedmemories_test.go` (NEW, suggested) | test | CRUD / transform | `internal/store/store_test.go` `TestDeleteOwnerGate` (836-858) for the authz half; no existing multi-edge test to model the edge-composition half on (new ground) | role-match for authz half only |
| `internal/store/listtags_test.go` (NEW, suggested) | test | CRUD | `internal/store/store_test.go` `TestDeleteOwnerGate` (836-858) for authz shape; `ListScopes`'s own (unexported) test coverage for the `more`-flag shape — grep at plan time | role-match |
| `cmd/engram/spine_review_archive.go` | (unmodified — compile-time constraint only) | — | — | N/A — no edit; SC2 is enforced by this file's existing `func(context.Context, string) (store.ArchiveResult, error)` call-site typing at lines 211/242, confirmed unchanged |

## Pattern Assignments

### `internal/store/spine.go` — `ArchiveAs`/`RestoreAs` (service, CRUD)

**Analog:** `internal/store/spine.go` `Archive` (verified lines 773-813, re-read this session — identical to RESEARCH.md's excerpt, reproduced here for the planner):

```go
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
(`Restore` at lines 836-876 is the structural mirror — `deletePayloadKeys` instead of `SetPayload`.)

**Auth/Gate pattern** — `getWritable` (verified lines 2195-2208, `internal/store/store.go`):

```go
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

**Required composition (D-02, per CONTEXT.md — not a verbatim analog, the shape the planner must
produce):** factor `Archive`/`Restore`'s post-`Get` body into an unexported core taking an
already-resolved `Memory` — mirroring `FetchForUpdate`'s doc-commented contract (verified
`internal/store/store.go:2255`, `func (s *Store) FetchForUpdate(ctx context.Context, id string, subj Subject) (out Memory, err error)`)
so the update handler performs ownership-gate-then-mutate as a single `Get` round trip, not two.
`ArchiveAs`/`RestoreAs` call `s.locker.Lock` → `getWritable(ctx, id, subj, authz.ActionArchive)` →
shared core (skip its internal `Get`, reuse the `Memory` `getWritable` already fetched) → unlock.
The subject-less `Archive`/`Restore` keep calling `s.Get` directly and are otherwise untouched (SC2).

**Error handling pattern:** `getWritable`'s denial and the subject-less `Get`'s not-found both
collapse to the same `ErrNotFound` (D-04) — `errors.Is(err, ErrNotFound)` is the check, never a
distinguishable error variant for "exists but not owned" vs "doesn't exist" (DEC-xa6).

---

### `internal/authz/authz.go` — `ActionArchive` (config)

**Analog:** same file, verified lines 22-29:

```go
// The five engram authorization actions.
const (
	ActionRead     Action = "read"
	ActionWrite    Action = "write"
	ActionDelete   Action = "delete"
	ActionShare    Action = "share"
	ActionSchedule Action = "schedule"
)
```

Add `ActionArchive Action = "archive"` to this block and update the doc comment to "six." No Cedar
engine change needed — action entities are built dynamically (`actionEntityUID`,
`internal/authz/entities.go:79-81`, confirmed present this session via the RESEARCH.md excerpt and
not independently re-read — low risk, one-line accessor). `schema.json` is reference-only (its own
header comment says so) — add `"archive"` there too for documentation hygiene (D-01), not
functionally required.

**Policy files need NO edits** — verified in full:

```cedar
// internal/authz/policies/own_records.cedar (full file)
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
// internal/authz/policies/shared_read.cedar (full file)
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
`own_records.cedar` matches `action` unconstrained (owner gets every action, including the new
one); `shared_read.cedar` matches only `read` — `ActionArchive` falls through to Deny for a shared
non-owned record automatically.

---

### `internal/authz/policy_corpus_test.go` — widen two lists (test)

**Analog/target, verified lines 14, 34-40 (re-read this session):**

```go
// line 14 — add ActionArchive here
var allActions = []Action{ActionRead, ActionWrite, ActionDelete, ActionShare, ActionSchedule}
```
```go
// TestPolicyCorpus_SharedReadOnly, lines 30-40 — add ActionArchive to this slice
func TestPolicyCorpus_SharedReadOnly(t *testing.T) {
	pdp := MustDefault()
	if got := pdp.DecideRecord("alice", "human", ActionRead, "bob", "note", "shared", ""); !got.Allow {
		t.Fatalf("DecideRecord(owner=alice, action=read, resource=bob/shared) = Deny, want Allow")
	}
	for _, action := range []Action{ActionWrite, ActionDelete, ActionShare, ActionSchedule} {
		if got := pdp.DecideRecord("alice", "human", action, "bob", "note", "shared", ""); got.Allow {
			t.Fatalf("DecideRecord(owner=alice, action=%s, resource=bob/shared) = Allow, want Deny (DEC-kyz)", action)
		}
	}
}
```
Both edits are additive appends to existing `[]Action{...}` literals — no structural change.

---

### `internal/store/relatedmemories.go` (NEW) — `RelatedMemories` (service, transform)

**Analog 1 — query-by-id sub-query shape**, `NearDuplicates` (verified lines 631-644, re-read this
session, matches RESEARCH.md verbatim):

```go
for _, chunk := range chunkIDs(ids, nearDuplicateBatchSize) {
	qp := make([]*qdrant.QueryPoints, len(chunk))
	for i, id := range chunk {
		f := &qdrant.Filter{MustNot: []*qdrant.Condition{qdrant.NewHasID(qdrant.NewID(id))}}
		if scopeMust != nil {
			f.Must = scopeMust
		}
		qp[i] = &qdrant.QueryPoints{
			CollectionName: s.collection,
			Query:          qdrant.NewQueryID(qdrant.NewID(id)),
			Filter:         f,
			Limit:          qdrant.PtrOf(topK),
		}
	}
	batchRes, qErr := s.client.QueryBatch(ctx, &qdrant.QueryBatchPoints{
		CollectionName: s.collection, QueryPoints: qp,
	})
	...
```
`NearDuplicates`' filter `f` is scope-only and subject-less (it is an operator sweep) — copy the
`NewQueryID`/self-exclusion shape, but replace `f` construction with Analog 2 below.

**Analog 2 — authz filter composition**, `Search` (verified lines 1219-1229, re-read this session):

```go
if err := rejectOverMaximum("k", k); err != nil {
	return nil, err
}

f := s.ownerScopeFilter(ctx, scope, subj)
// IncludeScheduled relaxes the ENTIRE activeWindowConditions append as one
// unit (both the not_before and not_after halves) — never split across two
// branches ...
```
Full composition (RESEARCH.md-quoted, structurally present at `store.go:1223-1249` per this
session's line-number confirmation of the function's start):
```go
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
For `RelatedMemories`: `scope = ""` (D-10, cross-spine), and the recall-gate three conditions are
always applied (no `Include*` knobs exposed — D-11) for the vector/tag/citation edges only.

**Analog 3 — enforcement fetch**, `fetchPayloadsByID` (`internal/store/searchfetch.go`, confirmed
git-tracked, function signature re-confirmed this session):
```go
func (s *Store) fetchPayloadsByID(ctx context.Context, f *qdrant.Filter, view readView, ids []string) (map[string]Memory, error) {
	...
	for start := 0; start < len(ids); start += batchSize {
		...
		if err := s.fetchPayloadBatch(ctx, f, view, ids[start:end], out); err != nil {
			return nil, err
		}
	}
	return out, nil
}
```
Pass the SAME `f` every sub-query used. Its doc comment (`searchfetch.go:21-24`, per RESEARCH.md)
is the structural guarantee behind D-10 ("never post-filtered"): a candidate the caller can't read
just never comes back from this call.

**Supersession-chain edge — the one part NOT filtered (D-11):** walk `Memory.Supersedes []string` /
`Memory.SupersededBy *string` (confirmed fields exist on `Memory` — same struct `Search`/`Get`
already decode) via `GetReadable(ctx, id, subj)` (verified signature,
`internal/store/store.go:2155`, `func (s *Store) GetReadable(ctx context.Context, id string, subj Subject) (out Memory, err error)`)
per hop, under a depth cap — never a Qdrant filter for this edge type, since chain members must
surface even when soft-hidden.

**Citation-edge matching:** `Citation` struct (verified — grep-confirmed shape referenced in
RESEARCH.md, fields `Kind`/`Ref`/`Locator`/`Pin`/`Excerpt`); match same `kind`+`ref` via
`qdrant.NewNestedFilter("citations", &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("kind", c.Kind), qdrant.NewMatch("ref", c.Ref)}})`,
OR'd (`Should`) across every anchor citation. `NewNestedFilter` confirmed present in the pinned
`github.com/qdrant/go-client v1.19.2` module (RESEARCH.md verified this directly against the module
cache — trust as HIGH-confidence, not independently re-read this session since it is vendored
third-party code, not project source).

**Error handling / not-found:** anchor fetch uses `GetReadable` (not `Get`) so an unreadable anchor
returns `ErrNotFound` (Claude's Discretion item in CONTEXT.md, resolved this way per RESEARCH.md's
Pattern 3 discussion) — mirrors every other subject-aware store method's `ErrNotFound` contract.

---

### `internal/store/listtags.go` (NEW) — `ListTags` (service, CRUD/facet)

**Analog 1 — Facet call shape**, `migrate_status.go` (confirmed git-tracked; call re-verified
present near the file's documented line 143 per RESEARCH.md, not independently re-read verbatim
this session — the shape is simple enough that RESEARCH.md's direct quote is trusted):

```go
hits, ferr := s.client.Facet(ctx, &qdrant.FacetCounts{
	CollectionName: s.collection,
	Key:            schemaVersionKey,
	Exact:          qdrant.PtrOf(true),
	Limit:          qdrant.PtrOf(migrateStatusFacetLimit),
})
```
`ListTags` uses `Key: "tags"`, adds `Filter: f` (the same `ownerScopeFilter` + recall-gate
composition as `RelatedMemories` above — `MigrateStatus` deliberately omits this, do not copy that
omission), and its OWN limit constant — never `migrateStatusFacetLimit` (that constant is an
operator-diagnostic bound deliberately above `MaxRecallLimit`).

**Analog 2 — `more`-flag / truncation shape**, `ListScopes` (verified lines 2037-2072, re-read this
session in full):

```go
func (s *Store) ListScopes(ctx context.Context, subj Subject) (out []ScopeCount, more bool, err error) {
	...
	const scanCap = 1000
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         &qdrant.Filter{Must: []*qdrant.Condition{s.ownerOrSharedCondition(ctx, subj)}},
		Limit:          qdrant.PtrOf(uint32(scanCap)),
		WithPayload:    qdrant.NewWithPayloadInclude("scope"),
	})
	if err != nil {
		return nil, false, err
	}
	counts := map[string]uint64{}
	for _, p := range pts {
		counts[p.GetPayload()["scope"].GetStringValue()]++
	}
	out = make([]ScopeCount, 0, len(counts))
	for sc, n := range counts {
		out = append(out, ScopeCount{Scope: sc, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Scope < out[j].Scope })
	return out, len(pts) == scanCap, nil
}
```
`FacetResponse` carries no truncation flag of its own (confirmed struct shape has only
`Hits`/`Time`/`Usage`), so mirror this file's `(out, more, err)` return shape and the "request
`Limit: N+1`, `more = len(hits) > N`" idiom (RESEARCH.md Pattern 4) — the request-side over-ask,
not a response field, is the only truncation lever available.

**Bound to reuse, not reinvent** — `MaxRecallLimit`/`rejectOverMaximum` (verified lines 1647,
1656-1660):
```go
const MaxRecallLimit = 1000

func rejectOverMaximum(field string, count uint64) error {
	if count <= MaxRecallLimit {
		return nil
	}
	return fmt.Errorf("%s exceeds the maximum of %d: %w", field, MaxRecallLimit, ErrInvalidArgument)
}
```
Apply `rejectOverMaximum` to `ListTags`' own caller-supplied `limit` argument, same as `Search`'s
first line does for `k`.

---

### `internal/store/store.go` — `ensureIndexes`, add `tags` index (config)

**Analog:** same function, verified lines 657-678, re-read this session — matches RESEARCH.md
verbatim:

```go
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
Add `{"tags", qdrant.FieldType_FieldTypeKeyword, nil}` to the `idxs` slice literal — no
`KeywordIndexParams` (only `owner` uses `IsTenant`). Idempotent by construction (the
`AlreadyExists`-continue arm), no other code change needed.

---

### `internal/store/schemaversion_recallgate_test.go` — widen 4 lists (test)

**This is the phase's highest-risk file edit.** Four independent lists/switches gate on the exact
strings `"Facet"` / `"Store.ListTags"`; D-15 in CONTEXT.md names only #1 and #2. All four verified
directly this session (`grep -n` + `Read`, matches RESEARCH.md Pitfall 1 exactly):

**1. `recognizedFilterCarryingRequestMethods`** (line 868, verified):
```go
var recognizedFilterCarryingRequestMethods = map[string]bool{
	"Query":  true, // *qdrant.QueryPoints
	"Scroll": true, // *qdrant.ScrollPoints (also covers ScrollAndOffset — see grpcMethodForEmission)
	"Count":  true, // *qdrant.CountPoints
}
```
→ add `"Facet": true`.

**2. `recallCaptureInterceptor`'s type switch** (lines 891-905, verified):
```go
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
→ add `case *qdrant.FacetCounts: capture.record("Facet", r.GetFilter())`.

**3. `recallEmissionMethods`** (line 334, verified — the STATIC AST-scan vocabulary, NOT named by
D-15):
```go
var recallEmissionMethods = map[string]bool{
	"Query":           true,
	"QueryBatch":      true,
	"Scroll":          true,
	"ScrollAndOffset": true,
	"Count":           true,
}
```
→ add `"Facet": true`, or `TestRecallEmissionSetIsCompleteAndClassified` never derives
`Store.ListTags` as an emission site at all (silent pass, not a failure).

**4. `recallEntryPointSeeds`** (line 355, verified) + a new `recallTransmitters` row (lines
486-511, model row below) — NOT named by D-15:
```go
var recallEntryPointSeeds = []string{
	"Store.Search",
	"Store.SearchReranked",
	"Store.SearchDiscovery",
	"Store.List",
	"Store.ListScheduled",
	"Store.ListScopes",
}
```
**Model row to copy the shape of** — the existing `ListScopes` entry in `recallTransmitters`
(verified lines 507-510, exact justification style to match):
```go
{
	enclosingFunc: "Store.ListScopes",
	justification: "Emits Scroll (store.go:1616), its own transmission. Serves ListScopes — exposed to callers through BOTH Connect (internal/server/connectapi.go) and MCP (internal/server/tools.go), so D-16's operator-tier exclusion rationale does not reach it.",
},
```
Add `"Store.ListTags"` to `recallEntryPointSeeds` and a matching row to `recallTransmitters` with
justification citing `Facet` + Phase-3's planned Connect+MCP exposure (RPC-04). `RelatedMemories`
needs a `recallTransmitters` row too (it emits `Query`/`Scroll`/`QueryBatch`, method names already
in lists 1-3) — whether it also joins `recallEntryPointSeeds` this phase or defers to Phase 3 is an
open question the planner resolves (RESEARCH.md Open Question 1); either way it needs SOME row in
one of `recallTransmitters`/`operatorMigrationEmitters`/`otherNonRecallEmitters` or the completeness
test fails loudly by design.

**Verification command to run before considering this file's edit done:**
```
go test ./internal/store/... -run TestRecallEmissionSetIsCompleteAndClassified -v
```
confirm `Store.ListTags` (and `Store.RelatedMemories` if it emits reachably) appear in the verbose
subtest output, not just that the suite is green.

---

### `internal/store/archive_authz_test.go` (NEW, suggested) — authz regression test

**Analog:** `TestDeleteOwnerGate` (verified `internal/store/store_test.go:836-858`, re-read this
session):

```go
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
Mirror exactly for `ArchiveAs`/`RestoreAs` (D-05, the phase's required FIRST test): owner can
archive/restore, non-owner-shared gets `ErrNotFound` with record state unchanged
(`cur.ArchivedAt == nil`), and add an `Anonymous()` vs owned-record case (constructors confirmed:
`internal/store/subject.go:35` `func Anonymous() Subject`, `:43` `func Authenticated(sub string) Subject`).

---

## Shared Patterns

### Authz gate (`getWritable`)
**Source:** `internal/store/store.go:2195-2208`
**Apply to:** `ArchiveAs`, `RestoreAs` — the only new gated single-record write paths this phase adds.
```go
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
Never hand-roll an `m.Owner == subj` comparison inline — this is the one DEC-cgb chokepoint.

### Read-scoped authz filter (`ownerScopeFilter` + recall-gate conditions)
**Source:** `internal/store/store.go:1223-1249` (Search's composition)
**Apply to:** `RelatedMemories`' vector/tag/citation edges, `ListTags`' Facet filter.
```go
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
For both new methods, apply the recall-gate three conditions unconditionally (no `Include*` knobs
exposed) and use `scope = ""` for `RelatedMemories` (D-10 cross-spine) or the caller's scope
argument for `ListTags` (empty means all-readable, D-14).

### Two-phase enforcement fetch (`fetchPayloadsByID`)
**Source:** `internal/store/searchfetch.go` (`fetchPayloadsByID`/`fetchPayloadBatch`)
**Apply to:** `RelatedMemories`' final resolution of every vector/tag/citation candidate id.
Pass the identical `*qdrant.Filter` every sub-query used — this is the structural reason no
post-filtering step is ever needed (D-10). A candidate that fails the filter on re-fetch simply
never appears in the result map; never an error, never returned stale.

### Truncation signal without a response-side flag
**Source:** `internal/store/store.go:2037-2072` (`ListScopes`'s `(out, more, err)` shape)
**Apply to:** `ListTags` (`FacetResponse` has no truncation field of its own — confirmed only
`Hits`/`Time`/`Usage` — so request `Limit: N+1` and derive `more = len(hits) > N`, or mirror
`ListScopes`'s `len(pts) == scanCap` idiom with `Limit: N`).

### Shared maximum guard (`rejectOverMaximum`/`MaxRecallLimit`)
**Source:** `internal/store/store.go:1647-1660`
**Apply to:** `RelatedMemories`' caller-supplied vector `k` only (not the per-edge-type caps, not
the supersession depth cap, not the total ceiling — those are planner-chosen constants, never
routed through this caller-facing validation error); `ListTags`' caller-supplied `limit`.
```go
const MaxRecallLimit = 1000

func rejectOverMaximum(field string, count uint64) error {
	if count <= MaxRecallLimit {
		return nil
	}
	return fmt.Errorf("%s exceeds the maximum of %d: %w", field, MaxRecallLimit, ErrInvalidArgument)
}
```

### `ErrNotFound` indistinguishability (DEC-xa6)
**Source:** `getWritable` (above) and `internal/store/store.go:2960` (`Delete`)
**Apply to:** `ArchiveAs`/`RestoreAs` (D-04: shared-non-owned collapses to the same `ErrNotFound` as
nonexistent) and `RelatedMemories`' anchor resolution (unreadable anchor → `ErrNotFound` via
`GetReadable`, never a distinguishable "forbidden" error).

### Telemetry/span wrapper on every store method
**Source:** every method read this session (`Archive`, `Search`, `ListScopes`) opens
`tracer.Start(ctx, "store.<Name>", ...)` and defers `telemetry.RecordStoreOp(ctx, "<Name>", start, err)`.
**Apply to:** `ArchiveAs`, `RestoreAs`, `RelatedMemories`, `ListTags` — every new exported method
needs this same span+telemetry wrapper; copy the exact `defer func(){...}()` shape from `Archive`
(reproduced in full above).

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `internal/store/relatedmemories_test.go` (edge-composition assertions, not the authz half) | test | transform | No existing test exercises a multi-edge-type union-with-per-type-evidence result shape; `TestDeleteOwnerGate` only covers the authz half. Build fixtures per D-06 through D-09 from scratch — RESEARCH.md's Code Examples section and CONTEXT.md's D-06..D-12 are the spec to test against, not an existing test to copy. |

## Metadata

**Analog search scope:** `internal/store/*.go` (non-test and test), `internal/authz/*.go`,
`internal/authz/policies/*.cedar`, `cmd/engram/spine_review_archive.go` — the exact file set
CONTEXT.md's canonical_refs names, confirmed complete by cross-referencing RESEARCH.md's Sources
list (Primary tier).
**Files scanned:** 13 existing files read/grepped this session (7 via direct `Read`, 6 via `grep -n`
line-number confirmation against RESEARCH.md's claims).
**Verification method:** every excerpt above was independently re-confirmed against the current
working tree this session (`grep -n` for line numbers, `Read` for body text) rather than trusted
from RESEARCH.md alone; line numbers match RESEARCH.md's verified citations in every case checked.
**Pattern extraction date:** 2026-09-25
