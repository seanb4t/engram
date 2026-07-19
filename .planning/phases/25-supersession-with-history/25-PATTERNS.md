# Phase 25: Supersession with History - Pattern Map

**Mapped:** 2026-07-19
**Files analyzed:** 4 (all modified, no new files)
**Analogs found:** 4 / 4 (all exact — every seam this phase touches has a verbatim in-repo precedent)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/store/store.go` — `Store.Supersede` method | service (store-layer mutation) | CRUD (payload-only partial update) | `Store.SetVisibility` (store.go:1693) | exact |
| `internal/store/store.go` — `Memory` struct + `payload()`/`fromPayload()` | model / codec | transform | `NotBefore`/`NotAfter` fields + codec (store.go:164-167, 438-443, 525-532) | exact |
| `internal/store/store.go` — recall-gate additions (`Search`/`List`) | service (query filter assembly) | CRUD (read-path filter) | `activeWindowConditions` call sites (store.go:791, 1010) | exact |
| `internal/store/store.go` — `ErrAlreadySuperseded` sentinel | model (error type) | n/a | `ErrIdempotencyConflict` (store.go:84-90) | exact |
| `internal/server/tools.go` — `supersedeArgs` struct | model (tool args) | request-response | `scheduleArgs` (tools.go:441-449, embeds `storeArgs`) | exact |
| `internal/server/tools.go` — `deps.supersedeMemory` handler | controller (MCP handler) | request-response | `deps.setVisibility` (tools.go:1201-1232) + `deps.storeMemory` (tools.go:712) | exact |
| `internal/server/tools.go` — `supersede_memory` tool registration | route (MCP tool registration) | request-response | `set_visibility` registration (tools.go:1423-1431) | exact |
| `internal/server/connecterror.go` — new `case` in `connectError` switch | middleware (error-to-code mapper) | transform | `store.ErrIdempotencyConflict` case (connecterror.go:60-65) | exact |
| `internal/store/store_test.go` — `TestSupersede*` cases | test | CRUD / request-response | `TestSetVisibilityOwnerGate` (store_test.go:797), `TestSetVisibilityTOCTOU` (store_test.go:848), `TestListDateWindow`/`TestSearchDateWindow` (store_test.go:2441/2480) | exact |
| `internal/server/tools_test.go` (or equivalent) — `TestSupersedeMemory` | test | request-response | existing `set_visibility` handler-level test | role-match (exact location TBC by planner) |

## Pattern Assignments

### `internal/store/store.go` — `Store.Supersede` (service, CRUD)

**Analog:** `Store.SetVisibility` (store.go:1685-1719)

**Full analog body** (store.go:1685-1719):
```go
// SetVisibility flips a record's shared flag without re-embedding (uses
// SetPayload, preserving the vector), only if owned by subj.
//
// TOCTOU note: if the record is deleted between the getWritable ownership gate
// and the SetPayload call, Qdrant's point-ID-selector SetPayload returns a
// NotFound gRPC error (verified against v1.18.2). That error propagates
// unchanged, so SetVisibility is fail-closed with respect to concurrent
// deletion — no additional re-fetch is required.
func (s *Store) SetVisibility(ctx context.Context, id string, subj Subject, shared bool) (err error) {
	ctx, span := tracer.Start(ctx, "store.SetVisibility",
		trace.WithAttributes(attribute.String("engram.owner", ownerOf(subj))))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "SetVisibility", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	if _, err := s.getWritable(ctx, id, subj, authz.ActionShare); err != nil {
		return err
	}
	vis := ""
	if shared {
		vis = visibilityShared
	}
	_, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"visibility": vis}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	})
	return err
}
```

**What to copy:** the exact shape — span+telemetry wrapper, `getWritable(ctx, id, subj, action)` gate, single-key `SetPayload` with `PointsSelectorIDs`, bare `return err` (fail-closed TOCTOU, no re-fetch), and the doc-comment TOCTOU note. For `Supersede`, swap `authz.ActionShare` → `authz.ActionWrite`, the `getWritable` target is `target` (not the id being created), add the D-05 already-superseded pre-check (`targetRec.SupersededBy != nil && *targetRec.SupersededBy != ""` → `ErrAlreadySuperseded`) between the gate and the `SetPayload`, and precede the back-stamp with a normal `s.Upsert(ctx, newMem, vec)` for the new record (order matters — create first, back-stamp second, per Pitfall 1). See RESEARCH.md "Recommended Method Signatures" for the full composed method — it is copy-ready.

**getWritable (unchanged, reuse verbatim)** (store.go:1426-1439):
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
	if !s.decideRecord(owner, kind, action, m.Owner, m.Category, m.Visibility, m.Scope).Allow {
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return m, nil
}
```

---

### `internal/store/store.go` — `ErrAlreadySuperseded` sentinel (model)

**Analog:** `ErrIdempotencyConflict` (store.go:84-90):
```go
// ErrIdempotencyConflict is returned when a keyed store_memory/schedule_memory
// ...
var ErrIdempotencyConflict = errors.New("idempotency key reused with different content")
```
Copy shape: `var ErrAlreadySuperseded = errors.New("target is already superseded")`, placed alongside the other sentinels (store.go:62-90 block), with a doc comment referencing D-05/D-06 (single live head, rejects superseding a non-head record).

---

### `internal/store/store.go` — `Memory` struct fields + codec (model, transform)

**Analog:** `NotBefore`/`NotAfter` (store.go:161-167):
```go
Visibility string    `json:"visibility,omitempty"`
CreatedAt  time.Time `json:"created_at"`
// NotBefore gates deferred reveal: the record is hidden from recall until
// now >= NotBefore. nil = always active (no lower gate).
NotBefore *time.Time `json:"not_before,omitempty"`
// NotAfter gates expiry: the record drops out of recall once now >= NotAfter.
// nil = never expires.
NotAfter *time.Time `json:"not_after,omitempty"`
```
New fields (place adjacent, per RESEARCH A3): `Supersedes *string `json:"supersedes,omitempty"`` / `SupersededBy *string `json:"superseded_by,omitempty"`` — plain json tags (NOT `json:"-"`), matching `NotBefore`/`NotAfter`'s wire-visibility class, not `EmbedderIdentity`'s audit-only class.

**Encode side** (store.go:438-443, inside `payload()`):
```go
if m.NotBefore != nil {
	p["not_before"] = m.NotBefore.Unix()
}
if m.NotAfter != nil {
	p["not_after"] = m.NotAfter.Unix()
}
```
New: `if m.Supersedes != nil { p["supersedes"] = *m.Supersedes }` / same for `superseded_by` — string values, not `.Unix()` epoch ints (these are id strings, not times).

**Decode side** (store.go:525-532, inside `fromPayload()`):
```go
if v, ok := p["not_before"]; ok {
	t := time.Unix(v.GetIntegerValue(), 0).UTC()
	m.NotBefore = &t
}
if v, ok := p["not_after"]; ok {
	t := time.Unix(v.GetIntegerValue(), 0).UTC()
	m.NotAfter = &t
}
```
New: `if v, ok := p["supersedes"]; ok { s := v.GetStringValue(); m.Supersedes = &s }` / same for `superseded_by`.

---

### `internal/store/store.go` — recall-gate additions (service, CRUD read-path filter)

**Analog:** `activeWindowConditions` call sites, both must be touched (Pitfall 3):

Search (store.go:790-791):
```go
f := s.ownerScopeFilter(scope, subj)
f.Must = append(f.Must, activeWindowConditions(s.now())...)
```

List (store.go:1009-1010):
```go
f := s.listFilter(scope, subj, opts)
f.Must = append(f.Must, activeWindowConditions(s.now())...)
```

**Add at BOTH sites** (do not fold into `activeWindowConditions` itself — that helper's name/doc promise is time-window-specific, per RESEARCH Anti-Patterns):
```go
f.Must = append(f.Must, qdrant.NewIsEmpty("superseded_by"))
```
`activeWindowConditions` definition for reference (store.go:727, unchanged, not modified):
```go
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
```
`Store.Get` (id-addressed fetch) is deliberately left untouched — no filter, superseded records stay fetchable (D-02/SC2).

---

### `internal/server/tools.go` — `supersedeArgs` (model, request-response)

**Analog:** `scheduleArgs` (tools.go:441-449), which embeds `storeArgs` (tools.go:424-439):
```go
type storeArgs struct {
	Content   string   `json:"content" jsonschema:"the memory text to persist"`
	Scope     string   `json:"scope" jsonschema:"run:tier:repo, e.g. eval-2026-05:project:selfhosted-cluster"`
	Source    string   `json:"source" jsonschema:"user-said or agent-inferred"`
	Category  string   `json:"category" jsonschema:"decision|preference|convention|gotcha"`
	Tags      []string `json:"tags,omitempty"`
	Repo      string   `json:"repo,omitempty"`
	Workspace string   `json:"workspace,omitempty"`
	Worktree  string   `json:"worktree_path,omitempty"`
	BaseDir   string   `json:"base_dir,omitempty"`
	Summary   string   `json:"summary,omitempty" jsonschema:"..."`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"..."`
}

// scheduleArgs embeds storeArgs and adds the temporal window.
type scheduleArgs struct {
	storeArgs
	NotBefore string `json:"not_before,omitempty" jsonschema:"RFC3339; hide from recall until this time"`
	NotAfter  string `json:"not_after,omitempty" jsonschema:"RFC3339; drop from recall at this time"`
}
```
**Copy this embedding shape exactly** for `supersedeArgs` (RESEARCH A4 / Pitfall 2 — embed `storeArgs`, don't hand-roll a parallel field list, so `idempotency_key` comes for free):
```go
type supersedeArgs struct {
	storeArgs
	Supersedes string `json:"supersedes" jsonschema:"id (full UUID or short_id) of the memory this new record corrects/replaces"`
}
```

---

### `internal/server/tools.go` — `deps.supersedeMemory` handler (controller, request-response)

**Analog:** `deps.setVisibility` (tools.go:1201-1232):
```go
func (d *deps) setVisibility(ctx context.Context, c caller, a setVisibilityArgs) (mutationResult, error) {
	pid, err := d.st.ResolvePointID(ctx, a.ID)
	if err != nil {
		return mutationResult{}, err
	}
	rec, err := d.st.GetReadable(ctx, pid, c.Subj)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return mutationResult{}, fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
		}
		return mutationResult{}, err
	}
	if rec.Category == "rule" {
		return mutationResult{}, fmt.Errorf("%w — delete the rule instead of changing its visibility", errRuleImmutable)
	}
	if err := d.st.SetVisibility(ctx, pid, c.Subj, a.Shared); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return mutationResult{}, fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
		}
		return mutationResult{}, err
	}
	return mutationResult{ID: rec.ID, ShortID: rec.ShortID}, nil
}
```

**404-indistinguishability pattern to copy:** resolve id (`ResolvePointID`), then on any `store.ErrNotFound` from the downstream store call, re-wrap with the caller's ORIGINAL unresolved input (`a.ID` / `a.Supersedes`), never the resolved UUID — this is repeated discipline across `setVisibility`, `storeDiscovery`, and must apply to `supersedeMemory`'s target-not-found path too.

**Handler composition** — mirrors `storeMemory`'s embed+persist (tools.go:712) fused with `setVisibility`'s resolve+gate shape; full recommended body is in RESEARCH.md "Recommended Method Signatures" (resolve target id via `ResolvePointID`, build `Memory` from embedded `storeArgs`, set `m.Supersedes = &targetID`, embed content, mint short id, call `d.st.Supersede(...)`, re-wrap `ErrNotFound` with `a.Supersedes`).

---

### `internal/server/tools.go` — `supersede_memory` registration (route)

**Analog:** `set_visibility` registration (tools.go:1423-1431):
```go
mcp.AddTool(s, &mcp.Tool{Name: "set_visibility", Description: "Share or unshare a memory you own. shared=true → readable by any authenticated caller (never writable by others); false → private. The id may be the full UUID or the short_id."},
	func(ctx context.Context, _ *mcp.CallToolRequest, a setVisibilityArgs) (*mcp.CallToolResult, any, error) {
		c, err := callerFromContext(ctx)
		if err != nil {
			return nil, nil, err
		}
		_, err = d.setVisibility(ctx, c, a)
		return textResult("visibility updated"), nil, err
	})
```
Copy this shape: `callerFromContext(ctx)` → handler call → `textResult(...)`. For `supersede_memory`, the handler returns `(id, shortID, error)` like `storeMemory`/`scheduleMemory` do, so the result-map shape (`map[string]string{"id": id, "short_id": sid}`) follows those, not `set_visibility`'s `nil` result.

---

### `internal/server/connecterror.go` — new sentinel case (middleware, transform)

**Analog:** `store.ErrIdempotencyConflict` case (connecterror.go:60-65):
```go
case errors.Is(err, store.ErrIdempotencyConflict):
	// Pre-positioning only (Phase 24, Plan 01): idempotency_key is
	// structurally unreachable from the Connect write lane this phase
	// (MCP-first per REQUIREMENTS Deferred), so this row cannot yet be
	// triggered — kept here so the sentinel switch stays exhaustive.
	return connect.NewError(connect.CodeAlreadyExists, err)
```
**Copy this "pre-positioning only" pattern exactly** for `store.ErrAlreadySuperseded`, mapped to `connect.CodeFailedPrecondition` (matches `errRuleImmutable`/`errStaleSummary`/`store.ErrAmbiguousShortID` — all "current record state forbids this operation," connecterror.go:54-59):
```go
case errors.Is(err, store.ErrAlreadySuperseded):
	// Pre-positioning only: supersede_memory is MCP-only this phase (no
	// Connect RPC exposes it yet) — kept here so the sentinel switch stays
	// exhaustive per this function's own enumeration discipline.
	return connect.NewError(connect.CodeFailedPrecondition, err)
```
Also update the function's doc-comment enumeration (connecterror.go:23-37) to list the new case, matching its existing bullet-list discipline.

---

### `internal/store/store_test.go` — `TestSupersede*` (test, CRUD/request-response)

**Analog 1 — owner gate:** `TestSetVisibilityOwnerGate` (store_test.go:797-831):
```go
func TestSetVisibilityOwnerGate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:vis"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	m := Memory{ID: "a1a1a1a1-0000-0000-0000-000000000001", Content: "v", Scope: scope, Owner: "sub-B", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Non-owner denied.
	if err := s.SetVisibility(ctx, m.ID, Authenticated("sub-A"), true); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-owner set_visibility: want ErrNotFound, got %v", err)
	}
	// ... owner succeeds, Get confirms mutation, vector preserved via SetPayload not Upsert
}
```
Copy shape for `TestSupersedeOwnerGate`: two records, non-owner subject attempts `s.Supersede(...)` on a target it doesn't own → expect `ErrNotFound` (not a distinct "forbidden" error — same fail-closed 404-indistinguishability). Owner succeeds → `Get` confirms `SupersededBy` set, `Content`/vector on target unchanged.

**Analog 2 — TOCTOU:** `TestSetVisibilityTOCTOU` (store_test.go:833-866+), including the `qdrantTOCTOUVerifiedVersion` guard block (store_test.go:852-866) — copy the same version-guard pattern for `TestSupersedeTOCTOU` (delete target between `getWritable` gate and `SetPayload`, assert error propagates, not nil).

**Analog 3 — dual recall-gate coverage:** `TestListDateWindow` (store_test.go:2441) / `TestSearchDateWindow` (store_test.go:2480) — the existing precedent for asserting a recall-gate condition excludes at BOTH `Search` and `List`. Mirror this exact pairing for `TestSupersedeRecallGate` (create + supersede a record, assert it's absent from both `Search` and `List` results but present via `Get`).

**New (no direct analog, but small):** `TestSupersedeAlreadySuperseded` (D-05 rejection) and `TestSupersedeForwardChain` (D-06: C supersedes A supersedes B, all three fetchable by id, only A and B excluded from recall — B because A superseded it earlier, A because C superseded it) — construct via repeated `s.Supersede` calls plus `s.Get`/`s.Search` assertions; no existing test to copy verbatim, compose from the owner-gate and recall-gate analogs above.

---

## Shared Patterns

### Owner-only write gate
**Source:** `getWritable` (store.go:1426-1439)
**Apply to:** `Store.Supersede`'s target-side gate. Already Cedar-PDP-backed (no new policy — `own_records.cedar` grants any action to the owner). Never substitute `GetReadable` (that permits shared-but-not-owned reads — would regress SC3).

### Payload-only, vector-preserving mutation
**Source:** `SetVisibility`'s `SetPayload` call (store.go:1713-1717)
**Apply to:** the target back-stamp in `Store.Supersede`. Never use a full `Upsert`/re-Upsert on an EXISTING record's payload — see `UpdatePayload`'s doc comment (store.go:1571-1580) and `persistAndEnqueue`'s CR-01 comment (tools.go:748-757) for the two independent in-repo hazard writeups this avoids.

### 404-indistinguishability re-wrap
**Source:** `setVisibility` handler (tools.go:1213-1219), `storeDiscovery` (tools.go:852-858)
**Apply to:** `supersedeMemory` handler's error path — re-wrap `store.ErrNotFound` with the caller's original unresolved input id (`a.Supersedes`), never the resolved UUID.

### Optional-pointer payload field codec
**Source:** `NotBefore`/`NotAfter` on `Memory` + `payload()`/`fromPayload()` (store.go:164-167, 438-443, 525-532)
**Apply to:** `Supersedes`/`SupersededBy` fields — `*string`, plain json tags (not `json:"-"`), encode-if-non-nil, decode-defensively.

### Sentinel-switch exhaustiveness in `connectError`
**Source:** `store.ErrIdempotencyConflict` "pre-positioning only" case (connecterror.go:60-65)
**Apply to:** `store.ErrAlreadySuperseded` — add even though Connect doesn't expose `supersede_memory` this phase; a reviewer expects every `store.Err*` sentinel enumerated here per the function's own doc comment.

### Dedicated verb over flag-overload
**Source:** `schedule_memory` vs `store_memory` (DEC-90w), tools.go:441-449 embedding pattern
**Apply to:** `supersede_memory` — new dedicated tool, `supersedeArgs` embeds `storeArgs` (not a flag on `storeArgs`/`store_memory`).

## No Analog Found

None — every file/seam this phase touches has a verified, exact in-repo precedent (see Summary in RESEARCH.md: "the engineering risk in this phase is not inventing anything — it's correctly threading four existing patterns together").

## Metadata

**Analog search scope:** `internal/store/store.go`, `internal/store/store_test.go`, `internal/server/tools.go`, `internal/server/connecterror.go`, `internal/authz/policies/own_records.cedar` (confirmed zero new policy needed)
**Files scanned:** 5 (all read directly this session; line numbers re-verified against current HEAD, matching RESEARCH.md's citations exactly — no drift)
**Pattern extraction date:** 2026-07-19
