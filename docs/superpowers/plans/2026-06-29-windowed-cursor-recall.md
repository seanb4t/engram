<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Server-Side Windowed + Cursor Recall Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Qdrant payload indexes (owner/scope/created_at) and rebuild engram's recall path to a true server-side query — date-window filtering + cursor paging on all recall tools — retiring the in-memory `scanCap`/`approximate` slice.

**Architecture:** Three stacked layers. (1) Idempotent Qdrant payload indexes created on every boot. (2) `store.List`/`Search`/`ListScheduled` rebuilt to filter/order/count server-side via the indexes. (3) Date-window params (`created_after`/`created_before`) + a boundary-id-set cursor surfaced on the Connect wire and the MCP tools. No data migration; existing RFC3339 `created_at` strings light up the datetime index on boot.

**Tech Stack:** Go 1.26, `github.com/qdrant/go-client v1.18.2` (Scroll/Query/Count/CreateFieldIndex/OrderBy/DatetimeRange), connect-go + buf (proto), modelcontextprotocol/go-sdk (MCP tools), testcontainers Qdrant for store integration tests, `task` runner.

**Spec:** `docs/superpowers/specs/2026-06-29-windowed-cursor-recall-design.md` (design-reviewer READY).

**Grounded correction to the spec (§6):** the spec calls the `list_memory` output reshape "the one non-additive break." The code already returns structured `map[string]any{"memories": mems}` (`internal/server/tools.go:728-730`), so adding `next_cursor` is **additive** — there is no breaking change. Task 9 corrects the spec/CLAUDE.md wording.

**Conventions every task follows:**

- VCS is jj (colocated). Commit with `jj commit -m "<conventional message>"` per `references/vcs-preamble.md`. Every commit message ends with the `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>` trailer. Never push to `main`.
- Every new `.go` file carries the Apache-2.0 SPDX header (run `task license:add` if unsure):

  ```go
  // SPDX-License-Identifier: Apache-2.0
  // Copyright 2026 Sean Brandt
  ```

- Run `task test` (lint + test) is the full gate; for tight loops use `go test ./internal/store/ -run <Name> -v`. Store tests need Docker (testcontainers Qdrant via `testStore(t)`).
- `task proto:gen` regenerates the committed `gen/` tree; `task proto:lint` runs buf lint + breaking.

---

## File structure

| File | Responsibility | Tasks |
|------|----------------|-------|
| `internal/store/store.go` | index creation, `ListOptions`, `List`/`Search`/`ListScheduled` server-side rebuild, date-range + order_by helpers | 1,2,3,5,6 |
| `internal/store/cursor.go` (new) | boundary-id-set cursor token encode/decode | 4 |
| `internal/store/cursor_test.go` (new) | cursor codec unit tests | 4 |
| `internal/store/store_test.go` | index/date-window/cursor/pagination integration tests | 1,2,3,4,5 |
| `internal/store/summarize.go` | correct the stale "not Qdrant-rangeable" comment | 9 |
| `proto/engram/v1/engram.proto` | additive request/response fields | 7 |
| `gen/**` | regenerated buf output (committed) | 7 |
| `internal/server/connectapi.go` | `ListMemories`/`SearchMemories` wire the new fields | 8 |
| `internal/server/tools.go` | MCP arg structs + handlers + `next_cursor` output | 8 |
| `internal/server/tools_test.go`, `connectapi_test.go` | handler tests | 8 |
| `CLAUDE.md`, `docs-site/src/content/docs/reference/tools.md`, `skill/engram/**` | docs + skill recall examples | 9 |

---

## Task 1: Payload indexes created idempotently on every boot

**Why:** `owner`/`scope`/`created_at` are unindexed, forcing in-memory scans. `ensureCollection` currently returns early when the collection exists (`store.go:173-175`), so indexes would never be created for existing deployments. Restructure it to **always** ensure the three indexes after the collection exists.

**Files:**

- Modify: `internal/store/store.go` (`ensureCollection`, ~169-183)
- Test: `internal/store/store_test.go` (new `TestEnsureCollectionCreatesIndexes`)

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go`:

```go
// TestEnsureCollectionCreatesIndexes pins that EnsureCollection provisions the
// owner/scope/created_at payload indexes and is idempotent on a second call.
func TestEnsureCollectionCreatesIndexes(t *testing.T) {
	s := testStore(t) // testStore already calls EnsureCollection(ctx, 3) once
	ctx := context.Background()

	info, err := s.client.GetCollectionInfo(ctx, s.collection)
	if err != nil {
		t.Fatalf("GetCollectionInfo: %v", err)
	}
	schema := info.GetPayloadSchema()
	for _, field := range []string{"owner", "scope", "created_at"} {
		if _, ok := schema[field]; !ok {
			t.Errorf("payload index missing for %q; have %v", field, keysOf(schema))
		}
	}

	// Idempotent: a second EnsureCollection must not error on existing indexes.
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("second EnsureCollection: %v", err)
	}
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// recordIDs extracts ids in slice order (later tasks assert order/coverage with
// it). NOTE: the existing TestSearchAndListTagsFilter defines a *local* `ids`
// closure — there is no package-level `ids`, so this shared helper is added here.
func recordIDs(ms []Memory) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return out
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestEnsureCollectionCreatesIndexes -v`
Expected: FAIL — `payload index missing for "owner"` (indexes not created yet).

- [ ] **Step 3: Implement index creation in `ensureCollection`**

Replace `ensureCollection` in `internal/store/store.go` (currently lines 169-183) with:

```go
func (s *Store) ensureCollection(ctx context.Context, name string, dim uint64) error {
	exists, err := s.client.CollectionExists(ctx, name)
	if err != nil {
		return err
	}
	if !exists {
		if err := s.client.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: name,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size: dim, Distance: qdrant.Distance_Cosine,
			}),
		}); err != nil {
			return err
		}
	}
	// Indexes are ensured on every boot (idempotently) so existing collections
	// gain them without a data migration: Qdrant backfills the index over the
	// already-stored RFC3339 created_at strings and keyword payloads.
	return s.ensureIndexes(ctx, name)
}

// ensureIndexes idempotently creates the recall payload indexes. owner is a
// tenant-optimized keyword (authz key), scope a keyword, created_at a datetime
// (enables server-side DatetimeRange + order_by). A re-create of an existing
// index with identical schema is a no-op in Qdrant; we additionally tolerate an
// "already exists" error defensively so boot never fails on a pre-indexed field.
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
			if strings.Contains(strings.ToLower(err.Error()), "already exists") {
				continue
			}
			return fmt.Errorf("ensure index %q: %w", ix.field, err)
		}
	}
	return nil
}
```

Confirm `strings` and `fmt` are imported in `store.go` (both already are — `fmt` is used in `ListScheduled`, `strings` may need adding; if `go build` reports `strings` unused/missing, add it to the import block).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestEnsureCollectionCreatesIndexes -v`
Expected: PASS.

- [ ] **Step 5: Run the package to check no regressions**

Run: `go test ./internal/store/ -run 'TestList|TestSearch|TestRecall' -v`
Expected: PASS (existing recall tests unaffected by index creation).

- [ ] **Step 6: Commit**

`jj commit -m "feat(store): create owner/scope/created_at payload indexes on boot (engram-nx2t)"` (with the Co-Authored-By trailer).

---

## Task 2: Date-window filter on `List` and `Search`

**Why:** Add the half-open `[created_after, created_before)` range as a server-side `DatetimeRange` condition, composed onto the existing authz filter. This task adds the option fields, the helper, and wires both recall paths; `List`'s server-side rebuild (order_by/Count) is Task 3.

**Files:**

- Modify: `internal/store/store.go` (`ListOptions` ~540-546; add `createdRangeCondition`; `List` filter ~611; `Search` signature + filter ~452-481)
- Modify: existing `Search` call sites (compile fix)
- Test: `internal/store/store_test.go` (new `TestListDateWindow`, `TestSearchDateWindow`)

- [ ] **Step 1: Add `CreatedAfter`/`CreatedBefore` to `ListOptions`**

In `internal/store/store.go`, extend the `ListOptions` struct (currently 540-546):

```go
ListOptions struct {
	Limit      uint64
	Offset     uint64
	Categories []string // empty = all
	Visibility string   // "" = all | "private" | "shared"
	Tags       []string // empty = all; non-empty = records carrying ALL listed tags
	// Half-open creation-time window. Zero value = unbounded on that side.
	// CreatedAfter is inclusive (gte); CreatedBefore is exclusive (lt).
	CreatedAfter  time.Time
	CreatedBefore time.Time
	Cursor        string // "" = offset mode; non-empty = cursor mode (Task 4). Mutually exclusive with Offset>0.
}
```

- [ ] **Step 2: Add the `createdRangeCondition` helper**

Add near `activeWindowConditions` in `internal/store/store.go`:

```go
// createdRangeCondition builds a half-open [after, before) DatetimeRange on the
// created_at datetime index: after→Gte (inclusive), before→Lt (exclusive). A
// zero bound is omitted. Returns nil when both bounds are zero (no filter).
func createdRangeCondition(after, before time.Time) *qdrant.Condition {
	if after.IsZero() && before.IsZero() {
		return nil
	}
	dr := &qdrant.DatetimeRange{}
	if !after.IsZero() {
		dr.Gte = timestamppb.New(after.UTC())
	}
	if !before.IsZero() {
		dr.Lt = timestamppb.New(before.UTC())
	}
	return qdrant.NewDatetimeRange("created_at", dr)
}
```

Add `"google.golang.org/protobuf/types/known/timestamppb"` to the imports if absent.

- [ ] **Step 3: Write the failing tests**

Add to `internal/store/store_test.go`:

```go
// TestListDateWindow pins half-open [after, before): a record AT created_after is
// included (gte); a record AT created_before is excluded (lt).
func TestListDateWindow(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "win-test:project:list"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	t0 := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	mk := func(id string, at time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A", CreatedAt: at}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert %s: %v", id, err)
		}
	}
	mk("a0000000-0000-0000-0000-000000000001", t0.Add(-time.Hour)) // before window
	mk("a0000000-0000-0000-0000-000000000002", t0)                 // == after  -> included
	mk("a0000000-0000-0000-0000-000000000003", t0.Add(time.Hour))  // inside
	mk("a0000000-0000-0000-0000-000000000004", t0.Add(2*time.Hour))// == before -> excluded

	subj := Authenticated("sub-A")
	items, total, _, err := s.List(ctx, scope, subj, ListOptions{
		Limit: 10, CreatedAfter: t0, CreatedBefore: t0.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := recordIDs(items)
	want := []string{
		"a0000000-0000-0000-0000-000000000003",
		"a0000000-0000-0000-0000-000000000002",
	} // CreatedAt desc
	if !slices.Equal(got, want) {
		t.Errorf("window: got %v want %v", got, want)
	}
	if total != 2 {
		t.Errorf("window total: got %d want 2", total)
	}
}

// TestSearchDateWindow pins the same half-open window as a Search pre-filter.
func TestSearchDateWindow(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "win-test:project:search"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	t0 := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	mk := func(id string, at time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A", CreatedAt: at}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert %s: %v", id, err)
		}
	}
	mk("b0000000-0000-0000-0000-000000000001", t0.Add(-time.Hour))
	mk("b0000000-0000-0000-0000-000000000002", t0.Add(time.Hour))

	hits, err := s.Search(ctx, scope, Authenticated("sub-A"),
		[]float32{0.1, 0.2, 0.3}, 10, nil, t0, time.Time{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if got := recordIDs(hits); !slices.Equal(got, []string{"b0000000-0000-0000-0000-000000000002"}) {
		t.Errorf("search window: got %v want [..002]", got)
	}
}
```

> NOTE: `cleanupErr(...)`, `Authenticated(...)`, `Anonymous()` are package-level and already exist. `recordIDs(...)` is added in Task 1 Step 1 (the existing `ids` is a *local closure* in `TestSearchAndListTagsFilter`, not package-level). The record-writer is `s.Upsert(ctx, m, vec)` (`store.go:327`, signature `func (s *Store) Upsert(ctx context.Context, m Memory, vec []float32) error`).

- [ ] **Step 4: Run tests to verify they fail**

Run: `go test ./internal/store/ -run 'TestListDateWindow|TestSearchDateWindow' -v`
Expected: FAIL to compile — `Search` takes 6 args, test passes 8 (signature not updated yet).

- [ ] **Step 5: Wire the range into `List` and `Search`**

In `List` (store.go ~611), after `f := listFilter(scope, subj, opts)` and the `activeWindowConditions` append, add:

```go
	if c := createdRangeCondition(opts.CreatedAfter, opts.CreatedBefore); c != nil {
		f.Must = append(f.Must, c)
	}
```

Change `Search`'s signature and filter (store.go 452):

```go
func (s *Store) Search(ctx context.Context, scope string, subj Subject, vec []float32, k uint64, tags []string, after, before time.Time) (out []Memory, err error) {
```

and after the `tagMatchConditions` append inside `Search`:

```go
	if c := createdRangeCondition(after, before); c != nil {
		f.Must = append(f.Must, c)
	}
```

- [ ] **Step 6: Fix `Search` call sites**

`Search` now needs two extra args at every caller. Update each to pass `time.Time{}, time.Time{}` (or real bounds):

- `internal/server/connectapi.go:118` (`SearchMemories`) → handled fully in Task 8; for now pass `time.Time{}, time.Time{}`.
- Test call sites in `internal/store/store_test.go`: `TestSearchListOwnerIsolation`, `TestSearchAndListTagsFilter`, `TestTagsFilterComposesWithWindow`, `TestRecallWindowGate`, `TestNilSubjectFailsClosed`. Find them with `mcp__probe__grep "s.Search(ctx" internal/store/store_test.go` and append `, time.Time{}, time.Time{}` before the closing paren.
- `internal/store/instrument_test.go:55` — `st.Search(context.Background(), "repo:spans", anonymous{}, make([]float32, 3), 5, nil)` also needs `, time.Time{}, time.Time{}` appended. Confirm with `mcp__probe__grep "Search(" internal/store/instrument_test.go`.

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./internal/store/ -run 'TestListDateWindow|TestSearchDateWindow|TestSearchListOwnerIsolation|TestSearchAndListTagsFilter|TestTagsFilterComposesWithWindow' -v`
Expected: PASS.

- [ ] **Step 8: Commit**

`jj commit -m "feat(store): half-open created_at window on List and Search (engram-nx2t)"`.

---

## Task 3: `List` server-side rebuild — order_by + exact Count + offset mode

**Why:** Replace scroll-to-`scanCap` → in-memory sort → slice with a server-side `order_by created_at desc` scroll and an exact `Count(filter)`. This retires `scanCap` and the `approximate` flag for `List`. The 3rd return value changes from `approximate bool` to `nextCursor string` (empty in offset mode; populated by Task 4's cursor mode) — chosen because most callers ignore it (`lst, _, _, err`), so churn is limited to `total`/`approximate` consumers.

**Files:**

- Modify: `internal/store/store.go` (`List`, 593-637)
- Modify: `internal/server/connectapi.go` (`ListMemories`, 84-102) — compile fix
- Modify: `internal/store/store_test.go` (`TestListPagination`) — compile fix
- Test: `internal/store/store_test.go` (new `TestListExactTotalPastOldCap`)

- [ ] **Step 1: Write the failing test (exact total beyond the old 1000 cap)**

Add to `internal/store/store_test.go`:

```go
// TestListExactTotalPastOldCap proves the scanCap ceiling is gone: with > 1000
// readable records, List returns an exact total (Count), not a capped 1000.
func TestListExactTotalPastOldCap(t *testing.T) {
	if testing.Short() {
		t.Skip("writes 1001 points; skipped in -short")
	}
	s := testStore(t)
	ctx := context.Background()
	scope := "cap-test:project:x"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	const n = 1001
	for i := 0; i < n; i++ {
		m := Memory{
			ID:        fmt.Sprintf("c0000000-0000-0000-0000-%012d", i),
			Content:   "c", Scope: scope, Owner: "sub-A",
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert %d: %v", i, err)
		}
	}
	_, total, _, err := s.List(ctx, scope, Authenticated("sub-A"), ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != n {
		t.Errorf("exact total: got %d want %d (scanCap not retired?)", total, n)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestListExactTotalPastOldCap -v`
Expected: FAIL — `got 1000 want 1001` (old scanCap truncates).

- [ ] **Step 3: Rebuild `List`**

Replace the body of `List` (store.go 593-637) with:

```go
func (s *Store) List(ctx context.Context, scope string, subj Subject, opts ListOptions) (items []Memory, total uint64, nextCursor string, err error) {
	ctx, span := tracer.Start(ctx, "store.List", trace.WithAttributes(
		attribute.String("engram.scope", scope),
		attribute.String("engram.owner", ownerOf(subj)),
	))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "List", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int("engram.result_count", len(items)))
		}
	}()

	if opts.Cursor != "" && opts.Offset > 0 {
		return nil, 0, "", fmt.Errorf("list: cursor and offset are mutually exclusive")
	}

	f := listFilter(scope, subj, opts)
	f.Must = append(f.Must, activeWindowConditions(s.now())...)
	if c := createdRangeCondition(opts.CreatedAfter, opts.CreatedBefore); c != nil {
		f.Must = append(f.Must, c)
	}

	// Exact total over the filtered set (replaces the scanCap approximation).
	total, err = s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: s.collection, Filter: f, Exact: qdrant.PtrOf(true),
	})
	if err != nil {
		return nil, 0, "", err
	}

	if opts.Cursor != "" {
		items, nextCursor, err = s.listByCursor(ctx, f, opts) // Task 4
		return items, total, nextCursor, err
	}

	// Offset mode: Qdrant has no numeric OFFSET, so scroll offset+limit ordered
	// records and return the trailing limit.
	fetch := opts.Offset + opts.Limit
	if opts.Limit == 0 {
		fetch = total // limit 0 = "all" (preserves prior behavior)
	}
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         f,
		Limit:          qdrant.PtrOf(uint32(fetch)),
		OrderBy:        &qdrant.OrderBy{Key: "created_at", Direction: qdrant.PtrOf(qdrant.Direction_Desc)},
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, 0, "", err
	}
	all := make([]Memory, 0, len(pts))
	for _, p := range pts {
		all = append(all, fromPayload(p.Id.GetUuid(), p.Payload))
	}
	if opts.Offset >= uint64(len(all)) {
		return []Memory{}, total, "", nil
	}
	return all[opts.Offset:], total, "", nil
}
```

> NOTE: `listByCursor` is introduced in Task 4. Until then, add a temporary stub so the package compiles: `func (s *Store) listByCursor(_ context.Context, _ *qdrant.Filter, _ ListOptions) ([]Memory, string, error) { return nil, "", fmt.Errorf("cursor mode not yet implemented") }` — Task 4 replaces it.

- [ ] **Step 4: Fix the `ListMemories` handler (3rd return is now a cursor)**

In `internal/server/connectapi.go` (84-102), change:

```go
	ms, total, _, err := a.d.st.List(ctx, req.Msg.Scope, subj, store.ListOptions{
		Limit:      req.Msg.Limit,
		Offset:     req.Msg.Offset,
		Categories: req.Msg.Categories,
		Visibility: req.Msg.Visibility,
		Tags:       req.Msg.Tags,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&engramv1.ListMemoriesResponse{
		Memories: shapeProtoMemories(ms, req.Msg.Full, a.d.summaryMaxChars), Total: total, Approximate: false,
	}), nil
```

(Date/cursor fields are wired in Task 8 once the proto has them. `Approximate` is now always `false`.)

- [ ] **Step 5: Fix `TestListPagination`**

In `internal/store/store_test.go`, `TestListPagination` has three `s.List` calls but only the **first** (~line 1492) binds the 3rd return as `approx`; the other two already blank it. Change that first call from `got, total, approx, err := ...` to `got, total, _, err := ...` and remove the now-dangling `approx` assertion (the `if approx { ... }` / approximate check). Keep the `total` and offset-slice assertions. The other two calls are unchanged.

- [ ] **Step 6: Run tests**

Run: `go test ./internal/store/ -run 'TestListExactTotalPastOldCap|TestListPagination|TestListDateWindow' -v` then `go build ./...`
Expected: PASS; build clean.

- [ ] **Step 7: Commit**

`jj commit -m "feat(store): rebuild List server-side (order_by + exact Count), retire scanCap (engram-nx2t)"`.

---

## Task 4: Boundary-id-set cursor

**Why:** Deterministic, migration-free paging that does not depend on Qdrant's intra-timestamp order stability. The cursor carries the oldest emitted `created_at` (`c`) and the set of ids already returned at exactly `c`; resume over-fetches `limit + len(seen)` (Qdrant scroll caps at `limit`), drops `seen` by id membership, takes `limit`.

**Files:**

- Create: `internal/store/cursor.go`, `internal/store/cursor_test.go`
- Modify: `internal/store/store.go` (replace the `listByCursor` stub from Task 3)
- Test: `internal/store/store_test.go` (new `TestListCursorTraversal`)

- [ ] **Step 1: Write the cursor codec unit test**

Create `internal/store/cursor_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"slices"
	"testing"
)

func TestCursorRoundTrip(t *testing.T) {
	in := listCursor{C: "2026-06-27T12:00:00Z", Seen: []string{"id-1", "id-2"}}
	tok := encodeCursor(in)
	if tok == "" {
		t.Fatal("encodeCursor returned empty")
	}
	out, err := decodeCursor(tok)
	if err != nil {
		t.Fatalf("decodeCursor: %v", err)
	}
	if out.C != in.C || !slices.Equal(out.Seen, in.Seen) {
		t.Errorf("round-trip mismatch: got %+v want %+v", out, in)
	}
}

func TestDecodeCursorRejectsGarbage(t *testing.T) {
	if _, err := decodeCursor("!!!not-base64!!!"); err == nil {
		t.Error("decodeCursor accepted non-base64 garbage; want error")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/store/ -run TestCursor -v`
Expected: FAIL — `undefined: listCursor / encodeCursor / decodeCursor`.

- [ ] **Step 3: Implement the codec**

Create `internal/store/cursor.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// listCursor is the opaque keyset cursor for List. C is the oldest created_at
// (RFC3339) emitted so far; Seen is every record id already returned at exactly
// C. Resume drops Seen by id membership, making page boundaries independent of
// Qdrant's intra-timestamp order. See the spec §3.
type listCursor struct {
	C    string   `json:"c"`
	Seen []string `json:"seen"`
}

func encodeCursor(c listCursor) string {
	b, _ := json.Marshal(c) // listCursor has only string fields; marshal cannot fail
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(tok string) (listCursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil {
		return listCursor{}, fmt.Errorf("decode cursor: %w", err)
	}
	var c listCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return listCursor{}, fmt.Errorf("decode cursor: %w", err)
	}
	return c, nil
}
```

- [ ] **Step 4: Run to verify codec passes**

Run: `go test ./internal/store/ -run TestCursor -v`
Expected: PASS.

- [ ] **Step 5: Write the failing traversal test (the finding-1 pin)**

Add to `internal/store/store_test.go`:

```go
// TestListCursorTraversal pins order-independent cursor paging at limit=1: N
// records sharing ONE timestamp plus M with distinct timestamps, paged to
// exhaustion, must yield the full set with no duplicates and no skips. At limit=1
// this only passes if resume over-fetches limit+len(seen).
func TestListCursorTraversal(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "cursor-test:project:x"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	tie := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	want := map[string]bool{}
	mk := func(id string, at time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A", CreatedAt: at}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert %s: %v", id, err)
		}
		want[id] = true
	}
	// 4 records share `tie`; 2 have distinct (older) timestamps.
	mk("d0000000-0000-0000-0000-000000000001", tie)
	mk("d0000000-0000-0000-0000-000000000002", tie)
	mk("d0000000-0000-0000-0000-000000000003", tie)
	mk("d0000000-0000-0000-0000-000000000004", tie)
	mk("d0000000-0000-0000-0000-000000000005", tie.Add(-time.Hour))
	mk("d0000000-0000-0000-0000-000000000006", tie.Add(-2*time.Hour))

	subj := Authenticated("sub-A")
	seen := map[string]int{}
	cursor := ""
	for steps := 0; steps < 100; steps++ {
		// CursorMode:true makes page 1 (Cursor:"") route through listByCursor, which
		// emits a nextCursor; without it the offset path runs and returns "" → break.
		items, _, next, err := s.List(ctx, scope, subj, ListOptions{Limit: 1, Cursor: cursor, CursorMode: true})
		if err != nil {
			t.Fatalf("List page: %v", err)
		}
		for _, m := range items {
			seen[m.ID]++
		}
		if next == "" {
			break
		}
		cursor = next
	}
	if len(seen) != len(want) {
		t.Errorf("traversal coverage: got %d distinct want %d", len(seen), len(want))
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("record %s returned %d times (want 1) — dup/skip bug", id, n)
		}
		if !want[id] {
			t.Errorf("unexpected id %s", id)
		}
	}
	for id := range want {
		if seen[id] == 0 {
			t.Errorf("record %s never returned — skip bug", id)
		}
	}
}
```

> NOTE: every `List` call sets `CursorMode: true` (the MCP default). With it, the first call (`Cursor: ""`) routes through `listByCursor` via the Step 7 routing condition `opts.Cursor != "" || (opts.Offset == 0 && opts.Limit > 0 && opts.CursorMode)`, so page 1 emits a `nextCursor` and the loop runs until `listByCursor` returns `""` (fewer than `limit` fresh records remain). The offset path is never taken in this test.

- [ ] **Step 6: Add `CursorMode` to `ListOptions`, then implement `listByCursor`**

First add the cursor-mode opt-in to the `ListOptions` struct (so the routing below compiles):

```go
	CursorMode bool // true = boundary-id-set cursor paging (MCP default); false = offset paging (UI)
```

Then replace the Task-3 stub in `internal/store/store.go` with:

```go
// listByCursor implements boundary-id-set keyset paging over the already-built
// filter f. opts.Cursor may be empty (first page); a non-empty cursor resumes at
// its created_at boundary, dropping ids already emitted at that exact timestamp.
func (s *Store) listByCursor(ctx context.Context, f *qdrant.Filter, opts ListOptions) ([]Memory, string, error) {
	limit := opts.Limit
	if limit == 0 {
		limit = 20
	}
	var startFrom *qdrant.StartFrom
	seen := map[string]bool{}
	var boundary string
	if opts.Cursor != "" {
		c, err := decodeCursor(opts.Cursor)
		if err != nil {
			return nil, "", err
		}
		boundary = c.C
		startFrom = qdrant.NewStartFromDatetime(c.C)
		for _, id := range c.Seen {
			seen[id] = true
		}
	}

	// Over-fetch by len(seen) so >= limit fresh candidates survive the drop.
	fetch := limit + uint64(len(seen)) + 1
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         f,
		Limit:          qdrant.PtrOf(uint32(fetch)),
		OrderBy: &qdrant.OrderBy{
			Key:       "created_at",
			Direction: qdrant.PtrOf(qdrant.Direction_Desc),
			StartFrom: startFrom,
		},
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, "", err
	}

	out := make([]Memory, 0, limit)
	for _, p := range pts {
		m := fromPayload(p.Id.GetUuid(), p.Payload)
		ts := m.CreatedAt.UTC().Format(time.RFC3339)
		if ts == boundary && seen[m.ID] {
			continue // already emitted at this exact timestamp
		}
		out = append(out, m)
		if uint64(len(out)) == limit {
			break
		}
	}

	if uint64(len(out)) < limit {
		return out, "", nil // exhausted: no next page
	}

	// Build next cursor from the last emitted record: c = its created_at, seen =
	// every emitted id sharing that timestamp (so the next page drops them).
	last := out[len(out)-1]
	nextC := last.CreatedAt.UTC().Format(time.RFC3339)
	nextSeen := make([]string, 0, 4)
	// Carry forward prior seen ids if the boundary did not advance.
	if nextC == boundary {
		nextSeen = append(nextSeen, idsAtBoundary(seen)...)
	}
	for _, m := range out {
		if m.CreatedAt.UTC().Format(time.RFC3339) == nextC {
			nextSeen = append(nextSeen, m.ID)
		}
	}
	return out, encodeCursor(listCursor{C: nextC, Seen: dedup(nextSeen)}), nil
}

// idsAtBoundary returns the keys of a seen-set as a slice (order irrelevant).
func idsAtBoundary(seen map[string]bool) []string {
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	return out
}

func dedup(in []string) []string {
	m := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !m[s] {
			m[s] = true
			out = append(out, s)
		}
	}
	return out
}
```

**Routing for the first page:** `CursorMode` (added in Step 6) distinguishes the MCP path (cursor default) from the Connect UI path (offset). Update the cursor-branch condition in `List` (the `if opts.Cursor != "" {` block from Task 3) to also route a CursorMode first page (`Cursor == "" && Offset == 0`) through `listByCursor`:

```go
	if opts.Cursor != "" || (opts.Offset == 0 && opts.Limit > 0 && opts.CursorMode) {
		items, nextCursor, err = s.listByCursor(ctx, f, opts)
		return items, total, nextCursor, err
	}
```

The MCP `list_memory` handler sets `CursorMode: true` (Task 8); the Connect UI path leaves it false → offset scroll. This keeps offset mode (UI) and cursor mode (MCP) cleanly separated and makes the first-page-cursor behavior explicit, not inferred.

- [ ] **Step 7: Confirm `ListOptions` final shape compiles**

`ListOptions` now carries: `Limit, Offset, Categories, Visibility, Tags, CreatedAfter, CreatedBefore, Cursor, CursorMode`. Run `go build ./internal/store/` and confirm it compiles before running the tests.

Run: `go build ./internal/store/`
Expected: clean.

- [ ] **Step 8: Run the traversal + codec tests**

Run: `go test ./internal/store/ -run 'TestListCursorTraversal|TestCursor' -v`
Expected: PASS (full coverage, no dup/skip at limit=1).

- [ ] **Step 9: Run the whole store package**

Run: `go test ./internal/store/ -v`
Expected: PASS.

- [ ] **Step 10: Commit**

`jj commit -m "feat(store): boundary-id-set cursor paging for List (engram-nx2t)"`.

---

## Task 5: `ListScheduled` server-side order_by + date window, retire its scanCap

**Why:** Bring `ListScheduled` (store.go 689-732) onto the indexes: `order_by created_at desc` bounded by `limit` (no in-memory `scanCap` scroll-and-sort) plus the optional date window. It keeps limit-only paging.

**Files:**

- Modify: `internal/store/store.go` (`ListScheduled`)
- Test: `internal/store/store_test.go` (new `TestListScheduledDateWindow`; existing scheduled tests must stay green)

- [ ] **Step 1: Write the failing test**

```go
// TestListScheduledDateWindow pins the created_at window on the scheduled view.
func TestListScheduledDateWindow(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "sched-win:project:x"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	future := s.now().Add(48 * time.Hour)
	t0 := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	mk := func(id string, created time.Time) {
		nb := future
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A",
			NotBefore: &nb, CreatedAt: created}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert %s: %v", id, err)
		}
	}
	mk("e0000000-0000-0000-0000-000000000001", t0.Add(-time.Hour)) // before window
	mk("e0000000-0000-0000-0000-000000000002", t0.Add(time.Hour))  // inside

	got, err := s.ListScheduled(ctx, scope, Authenticated("sub-A"),
		ScheduledPending, ListOptions{Limit: 10, CreatedAfter: t0})
	if err != nil {
		t.Fatalf("ListScheduled: %v", err)
	}
	if len(got) != 1 || got[0].ID != "e0000000-0000-0000-0000-000000000002" {
		t.Errorf("scheduled window: got %v want just ..002", recordIDs(got))
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/store/ -run TestListScheduledDateWindow -v`
Expected: FAIL — both records returned (window ignored).

- [ ] **Step 3: Rebuild `ListScheduled`'s scroll**

In `ListScheduled` (store.go 689-732), replace the `const scanCap = 1000` block and the scroll/sort/truncate tail with:

```go
	limit := opts.Limit
	if limit == 0 {
		limit = 20
	}
	f := &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		ownerOnlyCondition(subj),
		scheduledStateCondition(state, s.now()),
	}}
	if c := createdRangeCondition(opts.CreatedAfter, opts.CreatedBefore); c != nil {
		f.Must = append(f.Must, c)
	}
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection, Filter: f,
		Limit:       qdrant.PtrOf(uint32(limit)),
		OrderBy:     &qdrant.OrderBy{Key: "created_at", Direction: qdrant.PtrOf(qdrant.Direction_Desc)},
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	items = make([]Memory, 0, len(pts))
	for _, p := range pts {
		items = append(items, fromPayload(p.Id.GetUuid(), p.Payload))
	}
	return items, nil
```

Remove the now-unused `sort` usage if `ListScheduled` was its only consumer (check `go build` — `List` no longer sorts either after Task 3, so `sort` may become unused package-wide; drop the import if `go build` flags it).

- [ ] **Step 4: Run scheduled tests**

Run: `go test ./internal/store/ -run 'TestListScheduled|TestListScheduledDateWindow|TestNotAfterBoundaryInstant' -v`
Expected: PASS (existing `TestListScheduledStates`, `TestListScheduledOwnerIsolation` still green).

- [ ] **Step 5: Commit**

`jj commit -m "feat(store): ListScheduled server-side order_by + window, retire scanCap (engram-nx2t)"`.

---

## Task 6: Proto — additive request/response fields + regen

**Why:** Surface the window + cursor on the Connect wire. All additive → `buf breaking` stays green.

**Files:**

- Modify: `proto/engram/v1/engram.proto`
- Regenerate: `gen/**` (committed)

- [ ] **Step 1: Edit the proto**

In `proto/engram/v1/engram.proto`, extend the three messages:

```proto
message ListMemoriesRequest {
  string scope = 1;
  uint64 limit = 2;
  uint64 offset = 3;
  repeated string categories = 4;
  string visibility = 5;
  repeated string tags = 6;
  bool full = 7;
  string created_after = 8;  // RFC3339; inclusive lower bound on created_at
  string created_before = 9; // RFC3339; exclusive upper bound on created_at
  string page_token = 10;    // opaque cursor; when set, cursor paging (ignores offset)
}

message ListMemoriesResponse {
  repeated Memory memories = 1;
  uint64 total = 2;
  bool approximate = 3 [deprecated = true]; // always false since totals are now exact (Count)
  string next_page_token = 4;               // empty when no further pages (cursor paging)
}

message SearchMemoriesRequest {
  string query = 1;
  string scope = 2;
  uint64 k = 3;
  repeated string tags = 4;
  bool full = 5;
  string created_after = 6;  // RFC3339; inclusive lower bound on created_at
  string created_before = 7; // RFC3339; exclusive upper bound on created_at
}
```

- [ ] **Step 2: Lint + regenerate**

Run: `task proto:lint` then `task proto:gen`
Expected: lint passes (buf breaking green — additive only); `gen/` updates.

- [ ] **Step 3: Build**

Run: `go build ./...`
Expected: clean (new generated accessors `GetCreatedAfter()` etc. available).

- [ ] **Step 4: Commit**

`jj commit -m "feat(proto): add created_after/before + page_token to recall RPCs (engram-nx2t)"` (include the regenerated `gen/` tree in the same commit).

---

## Task 7: Connect handlers wire the new fields

**Why:** `ListMemories`/`SearchMemories` parse RFC3339 windows + page_token and map them to `store` options, returning `next_page_token`. Malformed inputs → `CodeInvalidArgument`.

**Files:**

- Modify: `internal/server/connectapi.go` (`ListMemories`, `SearchMemories`)
- Test: `internal/server/connectapi_test.go` (new window/cursor cases)

- [ ] **Step 1: Add an RFC3339 parse helper**

Add to `internal/server/connectapi.go`:

```go
// parseRFC3339 maps an optional RFC3339 string to a time.Time; empty → zero.
func parseRFC3339(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, s)
}
```

- [ ] **Step 2: Write the failing handler test**

Add to `internal/server/connectapi_test.go` (follow the existing harness in that file — it constructs an `engramAPI` with a test store):

```go
func TestListMemoriesRejectsBadCreatedAfter(t *testing.T) {
	api := &engramAPI{d: testDeps(t)} // testDeps from tools_test.go; engramAPI{d:...} per connectapi_test.go
	ctx := authedContext(t, "sub-A")  // subject-injecting ctx helper used across server tests
	_, err := api.ListMemories(ctx, connect.NewRequest(&engramv1.ListMemoriesRequest{
		Scope: "s:project:x", CreatedAfter: "not-a-date",
	}))
	if err == nil || connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("bad created_after: got %v want InvalidArgument", err)
	}
}
```

> NOTE: `testDeps(t)` (`tools_test.go:169`, skips without Qdrant) and `authedContext(t, "sub")` are the real server-test helpers; handler tests build `api := &engramAPI{d: d}` (`connectapi_test.go:61`). The Connect handler reads its subject via `subjectFromConnectContext`; confirm `authedContext` injects a subject that path can read (it is the documented authenticated handler-path ctx helper at `tools_test.go:207`) — if Connect needs a different ctx shape, match the helper `connectapi_test.go` already uses for an authenticated request.

- [ ] **Step 3: Run to verify it fails**

Run: `go test ./internal/server/ -run TestListMemoriesRejectsBadCreatedAfter -v`
Expected: FAIL (no validation yet — bad date silently ignored).

- [ ] **Step 4: Implement `ListMemories` wiring**

Replace the `ListMemories` body's store-call section:

```go
	after, err := parseRFC3339(req.Msg.CreatedAfter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("created_after: %w", err))
	}
	before, err := parseRFC3339(req.Msg.CreatedBefore)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("created_before: %w", err))
	}
	ms, total, nextToken, err := a.d.st.List(ctx, req.Msg.Scope, subj, store.ListOptions{
		Limit:         req.Msg.Limit,
		Offset:        req.Msg.Offset,
		Categories:    req.Msg.Categories,
		Visibility:    req.Msg.Visibility,
		Tags:          req.Msg.Tags,
		CreatedAfter:  after,
		CreatedBefore: before,
		Cursor:        req.Msg.PageToken,
		CursorMode:    req.Msg.PageToken != "",
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err) // covers bad cursor / offset+cursor
	}
	return connect.NewResponse(&engramv1.ListMemoriesResponse{
		Memories:      shapeProtoMemories(ms, req.Msg.Full, a.d.summaryMaxChars),
		Total:         total,
		Approximate:   false,
		NextPageToken: nextToken,
	}), nil
```

> NOTE: a bad cursor and an offset+cursor conflict both surface from `List` as a plain error; mapping them to `CodeInvalidArgument` is correct, but a Qdrant outage would also be a `List` error and should be `CodeInternal`. Refine if the codebase distinguishes them — minimally, decode the cursor in the handler first (call `store`-exported validation) OR accept that invalid-arg is the dominant case. Keep it simple: invalid-arg here; revisit only if a reviewer flags conflating transport errors.

- [ ] **Step 5: Implement `SearchMemories` wiring**

Replace the `a.d.st.Search(...)` call in `SearchMemories` with:

```go
	after, err := parseRFC3339(req.Msg.CreatedAfter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("created_after: %w", err))
	}
	before, err := parseRFC3339(req.Msg.CreatedBefore)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("created_before: %w", err))
	}
	ms, err := a.d.st.Search(ctx, req.Msg.Scope, subj, vec, k, req.Msg.Tags, after, before)
```

(Replaces the temporary `time.Time{}, time.Time{}` placeholder from Task 2 Step 6.)

- [ ] **Step 6: Run handler tests**

Run: `go test ./internal/server/ -run 'TestListMemories|TestSearchMemories' -v`
Expected: PASS.

- [ ] **Step 7: Commit**

`jj commit -m "feat(server): wire created_after/before + page_token into Connect recall handlers (engram-nx2t)"`.

---

## Task 8: MCP tool args + handlers + `next_cursor` output

**Why:** Surface the window + cursor on `list_memory` (cursor default), and the window on `search_memory`/`list_scheduled`. `list_memory`'s structured output gains `next_cursor` (additive — it already returns `{"memories": ...}`).

**Files:**

- Modify: `internal/server/tools.go` (`listArgs`, `searchArgs`, `listScheduledArgs`, `listMemory`, `searchMemory`, `listScheduled`, the three `mcp.AddTool` registrations)
- Modify: `internal/server/summary.go` only if `shapeRecall` needs touching (it does not — pass-through)
- Test: `internal/server/tools_test.go` (new arg-schema + handler cases)

- [ ] **Step 1: Extend the arg structs**

In `internal/server/tools.go`:

```go
listArgs struct {
	Scope         string   `json:"scope" jsonschema:"the scope to list memories from"`
	Limit         uint64   `json:"limit,omitempty" jsonschema:"max memories to return (default 20)"`
	Tags          []string `json:"tags,omitempty" jsonschema:"optional; restrict to records carrying ALL listed tags"`
	Full          bool     `json:"full,omitempty" jsonschema:"return full content instead of summaries (default false → compact summary view)"`
	CreatedAfter  string   `json:"created_after,omitempty" jsonschema:"optional RFC3339; inclusive lower bound on created_at"`
	CreatedBefore string   `json:"created_before,omitempty" jsonschema:"optional RFC3339; exclusive upper bound on created_at"`
	Cursor        string   `json:"cursor,omitempty" jsonschema:"opaque pagination cursor from a prior next_cursor; omit for the first page"`
}

searchArgs struct {
	Query         string   `json:"query"`
	Scope         string   `json:"scope"`
	K             uint64   `json:"k,omitempty"`
	Tags          []string `json:"tags,omitempty" jsonschema:"optional; restrict to records carrying ALL listed tags"`
	Full          bool     `json:"full,omitempty" jsonschema:"return full content instead of summaries (default false → compact summary view)"`
	CreatedAfter  string   `json:"created_after,omitempty" jsonschema:"optional RFC3339; inclusive lower bound on created_at"`
	CreatedBefore string   `json:"created_before,omitempty" jsonschema:"optional RFC3339; exclusive upper bound on created_at"`
}

listScheduledArgs struct {
	Scope         string `json:"scope" jsonschema:"the scope to list scheduled/expired memories from"`
	State         string `json:"state,omitempty" jsonschema:"scheduled (default, not yet active) | expired | all"`
	Limit         uint64 `json:"limit,omitempty" jsonschema:"max memories to return (default 20)"`
	CreatedAfter  string `json:"created_after,omitempty" jsonschema:"optional RFC3339; inclusive lower bound on created_at"`
	CreatedBefore string `json:"created_before,omitempty" jsonschema:"optional RFC3339; exclusive upper bound on created_at"`
}
```

- [ ] **Step 2: Add a tool-side RFC3339 parser**

Add to `internal/server/tools.go`:

```go
// parseToolTime maps an optional RFC3339 arg to time.Time; empty → zero.
func parseToolTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, s)
}
```

- [ ] **Step 3: Rewrite `listMemory` to return `(memories, nextCursor)`**

```go
func (d *deps) listMemory(ctx context.Context, a listArgs) ([]any, string, error) {
	if a.Limit == 0 {
		a.Limit = 20
	}
	after, err := parseToolTime(a.CreatedAfter)
	if err != nil {
		return nil, "", fmt.Errorf("created_after: %w", err)
	}
	before, err := parseToolTime(a.CreatedBefore)
	if err != nil {
		return nil, "", fmt.Errorf("created_before: %w", err)
	}
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return nil, "", err
	}
	ms, _, next, err := d.st.List(ctx, a.Scope, subj, store.ListOptions{
		Limit:         a.Limit,
		Tags:          a.Tags,
		CreatedAfter:  after,
		CreatedBefore: before,
		Cursor:        a.Cursor,
		CursorMode:    true, // cursor is the MCP default
	})
	if err != nil {
		return nil, "", err
	}
	return shapeRecall(ms, a.Full, d.summaryMaxChars), next, nil
}
```

**Update the 6 existing `listMemory` callers** — `listMemory` now returns 3 values, so the existing call sites in `internal/server/tools_test.go` will not compile. Run `mcp__probe__grep "d.listMemory(" internal/server/tools_test.go` (current lines ~308, 328, 504, 724, 749, 964) and add the cursor return at each:

- `mems, _ := d.listMemory(...)` → `mems, _, _ := d.listMemory(...)`
- `mems, err := d.listMemory(...)` → `mems, _, err := d.listMemory(...)`
- `mems, err = d.listMemory(...)` → `mems, _, err = d.listMemory(...)`

- [ ] **Step 4: Update the `list_memory` registration to emit `next_cursor`**

```go
	mcp.AddTool(s, &mcp.Tool{Name: "list_memory", Description: "List memories in a scope without a query. Most-recent first. Optional `created_after`/`created_before` (RFC3339) window and `cursor` for paging (use the returned next_cursor). Optional `tags` (AND). Returns {memories, next_cursor}; compact summaries by default, `full=true` for full content."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a listArgs) (*mcp.CallToolResult, any, error) {
			mems, next, err := d.listMemory(ctx, a)
			return textResult(fmt.Sprintf("%d memories", len(mems))), map[string]any{"memories": mems, "next_cursor": next}, err
		})
```

- [ ] **Step 5: Wire the window into `searchMemory` and `listScheduled`**

In `searchMemory` (read it first via `mcp__probe__extract_code internal/server/tools.go#searchMemory`), parse `a.CreatedAfter`/`a.CreatedBefore` and pass to `d.st.Search(ctx, a.Scope, subj, vec, a.K, a.Tags, after, before)`.

In `listScheduled`, parse the window and pass into the `store.ListOptions`:

```go
	after, err := parseToolTime(a.CreatedAfter)
	if err != nil {
		return nil, fmt.Errorf("created_after: %w", err)
	}
	before, err := parseToolTime(a.CreatedBefore)
	if err != nil {
		return nil, fmt.Errorf("created_before: %w", err)
	}
	// ... existing state switch + subj ...
	return d.st.ListScheduled(ctx, a.Scope, subj, state,
		store.ListOptions{Limit: a.Limit, CreatedAfter: after, CreatedBefore: before})
```

- [ ] **Step 6: Write a handler test for the new output shape**

Add to `internal/server/tools_test.go`:

```go
func TestListMemoryReturnsNextCursorField(t *testing.T) {
	d := testDeps(t) // tools_test.go:169; skips without Qdrant
	ctx := authedContext(t, "sub-A")
	scope := "tool:project:nextcursor"
	if err := d.st.Upsert(ctx, store.Memory{ID: "f0000000-0000-0000-0000-000000000001",
		Content: "c", Scope: scope, Owner: "sub-A", CreatedAt: d.clock()}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() { _ = d.st.Delete(context.Background(), "f0000000-0000-0000-0000-000000000001", store.Authenticated("sub-A")) })
	mems, next, err := d.listMemory(ctx, listArgs{Scope: scope, Limit: 1})
	if err != nil {
		t.Fatalf("listMemory: %v", err)
	}
	_ = next // contract: the call returns a (possibly empty) next_cursor token
	if mems == nil {
		t.Error("expected memories slice")
	}
}

func TestListMemoryRejectsBadWindow(t *testing.T) {
	d := &deps{} // no Qdrant: parseToolTime("nope") fails before any store call
	if _, _, err := d.listMemory(context.Background(), listArgs{Scope: "tool:project:x", CreatedAfter: "nope"}); err == nil {
		t.Error("bad created_after accepted; want error")
	}
}
```

> NOTE: use `d.clock()` (`tools.go:484`), NOT the raw `d.now` field — `testDeps(t)` leaves `now` nil, and `clock()` is the nil-safe accessor that falls back to `time.Now().UTC()`. The existing `tools_test.go` already exercises `AddTool` schema generation across all tool arg structs, so the extended `listArgs`/`searchArgs`/`listScheduledArgs` are also guarded against a jsonschema panic by that test.

- [ ] **Step 7: Run server tests**

Run: `go test ./internal/server/ -v`
Expected: PASS (including the `AddTool` schema-generation guard with the new fields).

- [ ] **Step 8: Commit**

`jj commit -m "feat(server): MCP recall tools gain created_after/before + cursor; list_memory emits next_cursor (engram-nx2t)"`.

---

## Task 9: Docs, skill, and the stale comment

**Why:** Keep the memory contract, reference docs, skill recall examples, and the `summarize.go` comment consistent with the new capability. Correct the spec's inaccurate "non-additive break" note.

**Files:**

- Modify: `internal/store/summarize.go` (comment ~93)
- Modify: `CLAUDE.md` (Memory contract section)
- Modify: `docs-site/src/content/docs/reference/tools.md`
- Modify: `skill/engram/**` (recall guidance / examples)
- Modify: `docs/superpowers/specs/2026-06-29-windowed-cursor-recall-design.md` (§6 wording)

- [ ] **Step 1: Fix the stale comment**

In `internal/store/summarize.go` (~line 93), replace:

```go
// created_at age filtering is applied in-code (created_at is stored as an RFC3339 string, not a Qdrant-rangeable number).
```

with:

```go
// created_at age filtering is applied in-code here for simplicity; note that
// created_at IS server-side rangeable via the datetime payload index (see
// ensureIndexes) — recall paths use DatetimeRange, this sweep just keeps its
// in-code filter.
```

- [ ] **Step 2: Update `CLAUDE.md` memory contract**

In the "Memory contract" section, add a sentence to the recall paragraph: `search_memory` / `list_memory` / `list_scheduled` accept optional `created_after` / `created_before` (RFC3339, half-open `[after, before)`); `list_memory` paginates via an opaque `cursor` and returns `{memories, next_cursor}`. Keep it terse and consistent with the existing tag-filter sentence.

- [ ] **Step 3: Update `docs-site` reference**

In `docs-site/src/content/docs/reference/tools.md`, document the new `list_memory`/`search_memory`/`list_scheduled` params and the `list_memory` `{memories, next_cursor}` output. Match the file's existing per-tool format.

- [ ] **Step 4: Update the engram skill recall examples**

In `skill/engram/**` (grep for `list_memory` to find the recall examples and the session-start hook), update any example that shows `list_memory` output to the `{memories, next_cursor}` shape and mention the window params where windowed recall is relevant. Do not change the session-start recall semantics (still `list_memory limit 10`).

- [ ] **Step 5: Correct the spec §6 wording**

In `docs/superpowers/specs/2026-06-29-windowed-cursor-recall-design.md` §6, change the "one non-additive break" sentence to note the implementation found the MCP structured output is already `{memories}`, so `next_cursor` is additive (no break). One-sentence correction; keep the rest.

- [ ] **Step 6: Lint docs + license**

Run: `task lint` then `task license:check`
Expected: clean (rumdl/dprint/yamlfmt pass; SPDX headers intact).

- [ ] **Step 7: Commit**

`jj commit -m "docs(engram): document windowed/cursor recall; fix stale summarize comment (engram-nx2t)"`.

---

## Task 10: Full gate + integration sweep

**Why:** Confirm the whole feature is green end-to-end before review.

**Files:** none (verification only)

- [ ] **Step 1: Full build + vet**

Run: `go build ./... && go vet ./...`
Expected: clean.

- [ ] **Step 2: Full test + lint gate**

Run: `task test`
Expected: PASS (lint + the full Go suite, including the new store integration tests against testcontainers Qdrant).

- [ ] **Step 3: Proto drift check**

Run: `task proto:lint && git status --porcelain gen/` (read-only `git status` is allowed under jj)
Expected: buf green; `gen/` has no uncommitted drift (regenerated tree already committed in Task 6).

- [ ] **Step 4: Manual smoke (optional, if a local Qdrant + server is available)**

Start the server, then exercise: `list_memory` with `created_after`/`created_before` and follow `next_cursor` across pages; confirm no dup/skip and that an out-of-range window returns `{memories: [], next_cursor: ""}`.

- [ ] **Step 5: Final commit (if any verification fixups)**

Commit any fixups with a `chore`/`fix` conventional message and the Co-Authored-By trailer.

---

## Self-review checklist (run before plan-reviewer)

- **Spec coverage:** indexes (T1), date window all tools (T2,T5,T8), List server-side rebuild + Count/scanCap retirement (T3), cursor + tie-break (T4), ListScheduled (T5), proto additive (T6), Connect handlers (T7), MCP tools + next_cursor (T8), docs/skill/comment + ADRs context (T9), full gate (T10). ListScopes correctly excluded per spec.
- **Type consistency:** `List` returns `(items, total, nextCursor string, err)` everywhere (T3 handler, T8 tool, tests); `Search` takes `(…, tags, after, before)` everywhere (T2,T7,T8); `ListOptions` carries `CreatedAfter/CreatedBefore/Cursor/CursorMode` (T2,T4); `listCursor{C, Seen}` codec shared (T4).
- **No placeholders:** every code step shows real code; every run step shows the command + expected result.
