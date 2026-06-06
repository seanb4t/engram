<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Per-actor Memory Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an engram-wide authorization layer so each actor sees and mutates only their own memories, with opt-in per-record sharing — closing engram-ir1 (cross-actor overwrite) and engram-2kw (cross-actor reads).

**Architecture:** Authorization is enforced **in the store**. Reads compose an owner/shared subclause into the Qdrant query filter; id-addressed mutations gate on a fetch-then-compare against the caller's stable OIDC `sub`. The `sub` is read from the verified token (`TokenInfo.Extra["sub"]`) in the server layer and passed as an explicit parameter into every store method. A one-time CLI command backfills `owner` onto pre-isolation records.

**Tech Stack:** Go, Qdrant (`github.com/qdrant/go-client/qdrant`), MCP go-sdk (`github.com/modelcontextprotocol/go-sdk`), cobra. Source spec: `docs/superpowers/specs/2026-06-06-per-actor-memory-isolation-design.md`.

---

## File Structure

| File | Responsibility | Change |
|------|----------------|--------|
| `internal/store/store.go` | Qdrant persistence + queries | `Owner`/`Visibility` fields, `payload`/`fromPayload`, `ErrNotFound`, read-filter helpers, `getWritable`, owner-aware `Search`/`List`/`SearchDiscovery`/`GetReadable`/`Update`/`Delete`/`SetVisibility`/`DeleteAll`, `OwnedOrAbsent`, `MigrateSetOwner` |
| `internal/store/store_test.go` | Store integration tests | Two-owner isolation matrix, migration test |
| `internal/server/tools.go` | MCP tool handlers | `ownerFromContext`, thread `sub`, owner-stamp on writes, `updateArgs.Shared *bool`, `set_visibility` tool, `StoreFromEnv` |
| `internal/server/tools_test.go` | Handler integration tests | `sub==""` wiring + `shared *bool` semantics |
| `cmd/engram/migrate.go` | **New** — `migrate-set-owner` CLI command | self-registers via `init()` |
| `README.md`, `CLAUDE.md` | Memory contract docs | document `owner`/`visibility`, isolation invariant |

**Build-green discipline:** every store signature change and its handler/test call-sites are updated in the **same task**, so `go build ./...` stays green at every commit.

**Test posture:** store/handler integration tests skip unless `MEM_QDRANT_TEST_ADDR` (host:port, gRPC 6334) points at a live Qdrant — the existing convention (`testStore`/`testDeps`). Run them with, e.g., `MEM_QDRANT_TEST_ADDR=localhost:6334`. The go-sdk's token-context key is unexported, so handler tests cannot inject a `sub`; they exercise the `sub==""` path. The full two-owner matrix lives in store tests, which take `sub` as an explicit argument.

---

### Task 1: Data model — `Owner`/`Visibility` fields, payload round-trip, owner-stamp on writes

**Files:**

- Modify: `internal/store/store.go` (`Memory`, `payload`, `fromPayload`)
- Modify: `internal/server/tools.go` (`ownerFromContext`, `storeMemory`, `storeDiscovery`)
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go`:

```go
func TestOwnerVisibilityRoundtrip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	m := Memory{
		ID:         "aaaaaaaa-0000-0000-0000-000000000001",
		Content:    "owned + shared record",
		Scope:      "iso-test:project:r",
		Owner:      "sub-abc",
		Visibility: "shared",
		CreatedAt:  time.Now().UTC().Truncate(time.Second),
	}
	defer func() { _, _ = s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelector(qdrant.NewID(m.ID)),
	}) }()
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := s.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Owner != "sub-abc" || got.Visibility != "shared" {
		t.Errorf("round-trip lost owner/visibility: owner=%q visibility=%q", got.Owner, got.Visibility)
	}
}
```

(The cleanup uses the raw Qdrant client, not `s.Delete`, so this test is stable across the `Delete` signature change in Task 5.)

- [ ] **Step 2: Run test to verify it fails**

Run: `MEM_QDRANT_TEST_ADDR=localhost:6334 go test ./internal/store/ -run TestOwnerVisibilityRoundtrip -v`
Expected: FAIL — `got.Owner`/`got.Visibility` are empty (fields not persisted yet).

- [ ] **Step 3: Add the fields to `Memory`**

In `internal/store/store.go`, in the `Memory` struct, replace the `Actor` line and the line after it:

```go
	// Actor is the verified caller identity (email/username/subject) taken from
	// the validated OIDC token — never client-supplied. Empty when auth is off.
	Actor string `json:"actor"`
	// Owner is the stable OIDC subject (`sub`) of the caller — the authorization
	// key. Server-set from the validated token, never client-supplied. Empty when
	// auth is disabled (the single anonymous bucket).
	Owner string `json:"owner"`
	// Visibility gates cross-actor reads: "" (private, default) or "shared"
	// (readable by any authenticated caller). Writes always require ownership.
	Visibility string    `json:"visibility,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
```

- [ ] **Step 4: Write both keys in `payload`**

In `payload`, add two entries to the map literal, after `"actor": m.Actor,`:

```go
		"actor":         m.Actor,
		"owner":         m.Owner,
		"visibility":    m.Visibility,
		"created_at":    m.CreatedAt.Format(time.RFC3339),
```

- [ ] **Step 5: Read both keys in `fromPayload`**

In `fromPayload`, after the `actor` block, add:

```go
	if v, ok := p["actor"]; ok {
		m.Actor = v.GetStringValue()
	}
	if v, ok := p["owner"]; ok {
		m.Owner = v.GetStringValue()
	}
	if v, ok := p["visibility"]; ok {
		m.Visibility = v.GetStringValue()
	}
```

- [ ] **Step 6: Add `ownerFromContext` and stamp `owner` on writes**

In `internal/server/tools.go`, add next to `actorFromContext`:

```go
// ownerFromContext returns the stable OIDC subject (the authorization key)
// injected by the RequireBearerToken middleware, or "" when auth is disabled.
// Never client-supplied — it is the validated token's `sub`, which
// auth.TokenVerifier places in TokenInfo.Extra["sub"].
func ownerFromContext(ctx context.Context) string {
	if ti := mcpauth.TokenInfoFromContext(ctx); ti != nil {
		if sub, ok := ti.Extra["sub"].(string); ok {
			return sub
		}
	}
	return ""
}
```

In `storeMemory`, add `Owner` after the `Actor` line:

```go
		Actor:     actorFromContext(ctx),
		Owner:     ownerFromContext(ctx),
		CreatedAt: time.Now().UTC(),
```

In `storeDiscovery`, add `Owner` after its `Actor` line:

```go
		Actor:     actorFromContext(ctx),
		Owner:     ownerFromContext(ctx),
		CreatedAt: time.Now().UTC(),
```

- [ ] **Step 7: Run test to verify it passes + build**

Run: `go build ./... && MEM_QDRANT_TEST_ADDR=localhost:6334 go test ./internal/store/ -run TestOwnerVisibilityRoundtrip -v`
Expected: build OK; PASS.

- [ ] **Step 8: Commit**

```bash
jj commit -m "feat(store): add Owner/Visibility fields + owner-stamp on writes (engram-99z)"
```

---

### Task 2: Read isolation for `Search` and `List`

**Files:**

- Modify: `internal/store/store.go` (`ownerOrSharedCondition`, `ownerScopeFilter`, `Search`, `List`)
- Modify: `internal/server/tools.go` (`searchMemory`, `list_memory` handler)
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go`:

```go
func TestSearchListOwnerIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:search"
	defer func() { _ = s.DeleteAllRaw(ctx, scope) }()

	mk := func(id, owner, vis string) {
		m := Memory{ID: id, Content: "x", Scope: scope, Owner: owner, Visibility: vis,
			CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	mk("bbbbbbbb-0000-0000-0000-000000000001", "sub-A", "")       // A private
	mk("bbbbbbbb-0000-0000-0000-000000000002", "sub-B", "")       // B private
	mk("bbbbbbbb-0000-0000-0000-000000000003", "sub-B", "shared") // B shared

	// A sees only A-private + B-shared (2), never B-private.
	hits, err := s.Search(ctx, scope, "sub-A", []float32{0.1, 0.2, 0.3}, 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("Search: got %d want 2", len(hits))
	}
	for _, h := range hits {
		if h.Owner == "sub-B" && h.Visibility != "shared" {
			t.Errorf("leaked B's private record: %s", h.ID)
		}
	}
	// List honors the same filter.
	lst, err := s.List(ctx, scope, "sub-A", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lst) != 2 {
		t.Errorf("List: got %d want 2", len(lst))
	}
}
```

Add a raw test cleanup helper at the bottom of `internal/store/store_test.go` (used by isolation tests; bypasses owner gating):

```go
// DeleteAllRaw removes every point in scope regardless of owner — test cleanup only.
func (s *Store) DeleteAllRaw(ctx context.Context, scope string) error {
	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelectorFilter(&qdrant.Filter{
			Must: []*qdrant.Condition{qdrant.NewMatch("scope", scope)},
		}),
	})
	return err
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./... 2>&1 | head`
Expected: FAIL — compile error: `too few arguments in call to s.Search` (signature not yet changed).

- [ ] **Step 3: Add the read-filter helpers**

In `internal/store/store.go`, replace `scopeFilter` with the two helpers (keep `scopeFilter` for now — `DeleteAll` still uses it until Task 9):

```go
func (s *Store) scopeFilter(scope string) *qdrant.Filter {
	return &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("scope", scope)}}
}

// ownerOrSharedCondition matches records the caller may READ: owned by sub OR
// marked shared. Sharing grants read, never write.
func ownerOrSharedCondition(sub string) *qdrant.Condition {
	return qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
		qdrant.NewMatch("owner", sub),
		qdrant.NewMatch("visibility", "shared"),
	}})
}

// ownerScopeFilter restricts to a scope AND the caller's readable set.
func (s *Store) ownerScopeFilter(scope, sub string) *qdrant.Filter {
	return &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		ownerOrSharedCondition(sub),
	}}
}
```

- [ ] **Step 4: Make `Search` and `List` owner-aware**

Change `Search`'s signature and filter:

```go
// Search returns the k nearest readable memories to vec within scope (records
// the caller owns, plus shared records).
func (s *Store) Search(ctx context.Context, scope, sub string, vec []float32, k uint64) ([]Memory, error) {
	res, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collection, Query: qdrant.NewQuery(vec...),
		Filter: s.ownerScopeFilter(scope, sub), Limit: qdrant.PtrOf(k), WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	return memoriesFromPoints(res), nil
}
```

Change `List`'s signature and the `Filter:` line (body otherwise unchanged):

```go
func (s *Store) List(ctx context.Context, scope, sub string, limit uint64) ([]Memory, error) {
	const scanCap = 1000
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         s.ownerScopeFilter(scope, sub),
		Limit:          qdrant.PtrOf(uint32(scanCap)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	// ...rest of the existing body is unchanged...
```

- [ ] **Step 5: Update the handlers**

In `internal/server/tools.go`, `searchMemory`:

```go
	return d.st.Search(ctx, a.Scope, ownerFromContext(ctx), vec, a.K)
```

In `Register`, the `list_memory` handler body:

```go
			mems, err := d.st.List(ctx, a.Scope, ownerFromContext(ctx), a.Limit)
```

- [ ] **Step 6: Update the existing `Search` call-sites in `TestSearchAndDeleteAll`**

That test's records have `Owner == ""`, so passing `sub=""` keeps every assertion valid. In `internal/store/store_test.go`, update both `Search` calls (currently lines ~204 and ~216):

```go
	hits, err := s.Search(ctx, scope, "", []float32{0.9, 0.1, 0.0}, 5)   // was (ctx, scope, []float32{...}, 5)
	// ...later in the same test...
	hits2, err := s.Search(ctx, scope, "", []float32{0.9, 0.1, 0.0}, 5)  // was (ctx, scope, []float32{...}, 5)
```

(The `s.DeleteAll(ctx, scope)` call between them is fixed in Task 9, when `DeleteAll`'s signature changes. It still compiles after this task — only `Search` changed here.)

- [ ] **Step 7: Run tests + build**

Run: `go build ./... && MEM_QDRANT_TEST_ADDR=localhost:6334 go test ./internal/store/ -run 'TestSearchListOwnerIsolation|TestSearchAndDeleteAll' -v`
Expected: build OK; both PASS.

- [ ] **Step 8: Commit**

```bash
jj commit -m "feat(store): owner-isolate Search and List reads (engram-2kw)"
```

---

### Task 3: Read isolation for `SearchDiscovery` (incl. `cross_spine`)

**Files:**

- Modify: `internal/store/store.go` (`SearchDiscovery`)
- Modify: `internal/server/tools.go` (`searchDiscovery`)
- Test: `internal/store/store_test.go` (new test + update existing `TestSearchDiscoveryFilters` call-sites)

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go`:

```go
func TestSearchDiscoveryOwnerIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scopeA := "discovery:repo:isoA"
	scopeB := "discovery:repo:isoB"
	defer func() { _ = s.DeleteAllRaw(ctx, scopeA); _ = s.DeleteAllRaw(ctx, scopeB) }()

	mk := func(id, scope, owner, vis string) {
		m := Memory{ID: id, Content: "d", Scope: scope, Category: "discovery", Kind: "fact",
			Owner: owner, Visibility: vis, CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	mk("cccccccc-0000-0000-0000-000000000001", scopeA, "sub-A", "")       // A private, scopeA
	mk("cccccccc-0000-0000-0000-000000000002", scopeA, "sub-B", "")       // B private, scopeA
	mk("cccccccc-0000-0000-0000-000000000003", scopeB, "sub-A", "")       // A private, scopeB
	mk("cccccccc-0000-0000-0000-000000000004", scopeB, "sub-B", "shared") // B shared, scopeB
	q := []float32{0.1, 0.2, 0.3}

	// Scoped: A in scopeA sees only A's record (1), not B's private.
	hits, err := s.SearchDiscovery(ctx, scopeA, "", "sub-A", q, 10)
	if err != nil {
		t.Fatalf("scoped: %v", err)
	}
	if len(hits) != 1 || hits[0].Owner != "sub-A" {
		t.Fatalf("scoped: got %d %+v want 1 A-owned", len(hits), hits)
	}
	// cross_spine (scope=""): A sees A across both scopes (2) + B's shared (1) = 3, never B's private.
	all, err := s.SearchDiscovery(ctx, "", "", "sub-A", q, 10)
	if err != nil {
		t.Fatalf("cross_spine: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("cross_spine: got %d want 3", len(all))
	}
	for _, h := range all {
		if h.Owner == "sub-B" && h.Visibility != "shared" {
			t.Errorf("cross_spine leaked B's private discovery: %s", h.ID)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./... 2>&1 | head`
Expected: FAIL — compile error: `too few arguments in call to s.SearchDiscovery`.

- [ ] **Step 3: Make `SearchDiscovery` owner-aware**

In `internal/store/store.go`, change the signature and append the owner/shared subclause to `must` (kept even when scope is dropped, so `cross_spine` = *my* discoveries across all scopes plus shared, never everyone's):

```go
func (s *Store) SearchDiscovery(ctx context.Context, scope, kind, sub string, vec []float32, k uint64) ([]Memory, error) {
	must := []*qdrant.Condition{qdrant.NewMatch("category", "discovery")}
	if scope != "" {
		must = append(must, qdrant.NewMatch("scope", scope))
	}
	if kind != "" {
		must = append(must, qdrant.NewMatch("kind", kind))
	}
	must = append(must, ownerOrSharedCondition(sub))
	res, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collection, Query: qdrant.NewQuery(vec...),
		Filter: &qdrant.Filter{Must: must}, Limit: qdrant.PtrOf(k),
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	return memoriesFromPoints(res), nil
}
```

- [ ] **Step 4: Update the handler**

In `internal/server/tools.go`, `searchDiscovery`:

```go
	return d.st.SearchDiscovery(ctx, scope, a.Kind, ownerFromContext(ctx), vec, a.K)
```

- [ ] **Step 5: Update existing `TestSearchDiscoveryFilters` call-sites**

In `internal/store/store_test.go`, that test's records have no owner (Owner == ""), so passing `sub=""` keeps every existing assertion valid. Update the three `SearchDiscovery` calls to add the `sub` argument:

```go
	hits, err := s.SearchDiscovery(ctx, scopeA, "", "", q, 10)   // was (ctx, scopeA, "", q, 10)
	// ...
	maps, err := s.SearchDiscovery(ctx, scopeA, "map", "", q, 10) // was (ctx, scopeA, "map", q, 10)
	// ...
	all, err := s.SearchDiscovery(ctx, "", "", "", q, 10)         // was (ctx, "", "", q, 10)
```

- [ ] **Step 6: Run tests + build**

Run: `go build ./... && MEM_QDRANT_TEST_ADDR=localhost:6334 go test ./internal/store/ -run 'TestSearchDiscovery' -v`
Expected: build OK; both `TestSearchDiscoveryFilters` and `TestSearchDiscoveryOwnerIsolation` PASS.

- [ ] **Step 7: Commit**

```bash
jj commit -m "feat(store): owner-isolate SearchDiscovery incl. cross_spine (engram-2kw/f7h.3)"
```

---

### Task 4: Id read gate — `ErrNotFound` + `GetReadable`

**Files:**

- Modify: `internal/store/store.go` (`ErrNotFound`, `Get`, `GetReadable`)
- Modify: `internal/server/tools.go` (`get_memory` handler)
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go`:

```go
func TestGetReadableOwnerGate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:getr"
	defer func() { _ = s.DeleteAllRaw(ctx, scope) }()
	priv := Memory{ID: "dddddddd-0000-0000-0000-000000000001", Content: "p", Scope: scope, Owner: "sub-B", CreatedAt: time.Now().UTC()}
	shar := Memory{ID: "dddddddd-0000-0000-0000-000000000002", Content: "s", Scope: scope, Owner: "sub-B", Visibility: "shared", CreatedAt: time.Now().UTC()}
	for _, m := range []Memory{priv, shar} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	// Owner reads own private record.
	if _, err := s.GetReadable(ctx, priv.ID, "sub-B"); err != nil {
		t.Errorf("owner denied own record: %v", err)
	}
	// Non-owner reads a shared record.
	if _, err := s.GetReadable(ctx, shar.ID, "sub-A"); err != nil {
		t.Errorf("shared record denied to other actor: %v", err)
	}
	// Non-owner denied a private record — and it looks like not-found.
	_, err := s.GetReadable(ctx, priv.ID, "sub-A")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-actor private read: want ErrNotFound, got %v", err)
	}
}
```

Add `"errors"` to the test file's import block.

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./... 2>&1 | head`
Expected: FAIL — compile error: `undefined: s.GetReadable` and `undefined: ErrNotFound`.

- [ ] **Step 3: Add `ErrNotFound` and route `Get` through it**

In `internal/store/store.go`, add `"errors"` to the import block, then add the sentinel near the top (after the imports):

```go
// ErrNotFound is returned when an id is absent OR not visible to the caller —
// the two are indistinguishable by design, so ownership never leaks across actors.
var ErrNotFound = errors.New("not found")
```

In `Get`, change the not-found return so it wraps the sentinel (the rendered message — `"not found: <id>"` — is unchanged, so existing assertions still hold):

```go
	if len(pts) == 0 {
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
```

- [ ] **Step 4: Add `GetReadable`**

> **Export note:** the spec names the id-path gates in lowercase (`getReadable` /
> `getWritable` / `ownedOrAbsent`). In the implementation, the two primitives the
> server package calls — `GetReadable` (get_memory) and `OwnedOrAbsent` (Task 8,
> store_discovery) — MUST be **exported** (capitalized) because `internal/server`
> cannot reach unexported `store` methods. `getWritable` stays **unexported** — it
> is only used internally by `Delete`/`Update`/`SetVisibility`.

In `internal/store/store.go`, after `Get`:

```go
// GetReadable returns the record only if the caller may READ it (owns it or it
// is shared); otherwise ErrNotFound, so ownership never leaks across actors.
func (s *Store) GetReadable(ctx context.Context, id, sub string) (Memory, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return Memory{}, err
	}
	if m.Owner != sub && m.Visibility != "shared" {
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return m, nil
}
```

- [ ] **Step 5: Update the `get_memory` handler**

In `internal/server/tools.go`, `Register`, the `get_memory` handler:

```go
			func(ctx context.Context, _ *mcp.CallToolRequest, a idArgs) (*mcp.CallToolResult, any, error) {
				m, err := d.st.GetReadable(ctx, a.ID, ownerFromContext(ctx))
				return textResult(m.Content), m, err
			})
```

- [ ] **Step 6: Run tests + build**

Run: `go build ./... && MEM_QDRANT_TEST_ADDR=localhost:6334 go test ./internal/store/ -run TestGetReadableOwnerGate -v`
Expected: build OK; PASS.

- [ ] **Step 7: Commit**

```bash
jj commit -m "feat(store): GetReadable id read-gate + ErrNotFound sentinel (engram-99z)"
```

---

### Task 5: Id write gate — `getWritable` + owner-gated `Delete`

**Files:**

- Modify: `internal/store/store.go` (`getWritable`, `Delete`)
- Modify: `internal/server/tools.go` (`delete_memory` handler)
- Test: `internal/store/store_test.go` (new test + fix existing `TestUpsertGetDeleteRoundtrip`)

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go`:

```go
func TestDeleteOwnerGate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:del"
	defer func() { _ = s.DeleteAllRaw(ctx, scope) }()
	// Even a SHARED record is not deletable by a non-owner.
	m := Memory{ID: "eeeeeeee-0000-0000-0000-000000000001", Content: "s", Scope: scope, Owner: "sub-B", Visibility: "shared", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := s.Delete(ctx, m.ID, "sub-A"); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-owner delete: want ErrNotFound, got %v", err)
	}
	if _, err := s.Get(ctx, m.ID); err != nil {
		t.Errorf("record should survive non-owner delete: %v", err)
	}
	if err := s.Delete(ctx, m.ID, "sub-B"); err != nil {
		t.Errorf("owner delete failed: %v", err)
	}
	if _, err := s.Get(ctx, m.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("owner delete did not remove record: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./... 2>&1 | head`
Expected: FAIL — compile error: `too many arguments in call to s.Delete`.

- [ ] **Step 3: Add `getWritable` and owner-gate `Delete`**

In `internal/store/store.go`, add `getWritable` (above `Get` is fine), and change `Delete`:

```go
// getWritable returns the record only if the caller OWNS it (shared does NOT
// grant write); otherwise ErrNotFound. The mutate primitive.
func (s *Store) getWritable(ctx context.Context, id, sub string) (Memory, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return Memory{}, err
	}
	if m.Owner != sub {
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return m, nil
}

// Delete removes the memory with the given id, only if owned by sub.
func (s *Store) Delete(ctx context.Context, id, sub string) error {
	if _, err := s.getWritable(ctx, id, sub); err != nil {
		return err
	}
	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelector(qdrant.NewID(id)),
	})
	return err
}
```

- [ ] **Step 4: Update the `delete_memory` handler**

In `internal/server/tools.go`, `Register`, the `delete_memory` handler:

```go
			func(ctx context.Context, _ *mcp.CallToolRequest, a idArgs) (*mcp.CallToolResult, any, error) {
				err := d.st.Delete(ctx, a.ID, ownerFromContext(ctx))
				return textResult("deleted"), nil, err
			})
```

- [ ] **Step 5: Fix the existing `Delete` callers**

Both records have `Owner == ""`; pass `sub=""` so the gate (`""==""`) permits it.

In `internal/store/store_test.go`, `TestUpsertGetDeleteRoundtrip` (line ~58):

```go
	if err := s.Delete(ctx, m.ID, ""); err != nil {   // was s.Delete(ctx, m.ID)
		t.Fatalf("delete: %v", err)
	}
```

In `internal/store/store_test.go`, `TestDiscoveryRoundtrip` (line ~104):

```go
	_ = s.Delete(ctx, m.ID, "")   // was _ = s.Delete(ctx, m.ID)
```

- [ ] **Step 6: Run tests + build**

Run: `go build ./... && MEM_QDRANT_TEST_ADDR=localhost:6334 go test ./internal/store/ -run 'TestDeleteOwnerGate|TestUpsertGetDeleteRoundtrip|TestDiscoveryRoundtrip' -v`
Expected: build OK; all PASS.

- [ ] **Step 7: Commit**

```bash
jj commit -m "feat(store): owner-gate Delete via getWritable (engram-ir1)"
```

---

### Task 6: Owner-gated `Update` + `shared *bool` flag

**Files:**

- Modify: `internal/store/store.go` (`Update`)
- Modify: `internal/server/tools.go` (`updateArgs`, `updateMemory`)
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go`:

```go
func TestUpdateOwnerGateAndSharedFlag(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:upd"
	defer func() { _ = s.DeleteAllRaw(ctx, scope) }()
	m := Memory{ID: "ffffffff-0000-0000-0000-000000000001", Content: "v1", Scope: scope, Owner: "sub-B", Visibility: "shared", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	vec := []float32{0.4, 0.5, 0.6}
	// Non-owner cannot update even a shared record.
	if err := s.Update(ctx, m.ID, "sub-A", "hijack", nil, vec); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-owner update: want ErrNotFound, got %v", err)
	}
	// Owner content-only update (shared == nil) PRESERVES visibility.
	if err := s.Update(ctx, m.ID, "sub-B", "v2", nil, vec); err != nil {
		t.Fatalf("owner update: %v", err)
	}
	got, _ := s.Get(ctx, m.ID)
	if got.Content != "v2" || got.Visibility != "shared" {
		t.Errorf("content-only update lost sharing: content=%q visibility=%q", got.Content, got.Visibility)
	}
	// Explicit unshare.
	no := false
	if err := s.Update(ctx, m.ID, "sub-B", "v3", &no, vec); err != nil {
		t.Fatalf("unshare update: %v", err)
	}
	got, _ = s.Get(ctx, m.ID)
	if got.Visibility != "" {
		t.Errorf("unshare failed: visibility=%q", got.Visibility)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./... 2>&1 | head`
Expected: FAIL — compile error: `undefined: s.Update`.

- [ ] **Step 3: Add the `Update` store method**

In `internal/store/store.go`, after `Upsert`:

```go
// Update replaces a record's content (re-embedded via vec), only if owned by
// sub. When shared is non-nil it also sets visibility (true → "shared", false →
// ""); nil leaves visibility unchanged so a content edit never silently unshares.
func (s *Store) Update(ctx context.Context, id, sub, content string, shared *bool, vec []float32) error {
	cur, err := s.getWritable(ctx, id, sub)
	if err != nil {
		return err
	}
	cur.Content = content
	if shared != nil {
		if *shared {
			cur.Visibility = "shared"
		} else {
			cur.Visibility = ""
		}
	}
	return s.Upsert(ctx, cur, vec)
}
```

- [ ] **Step 4: Add `Shared *bool` to `updateArgs` and rewrite the handler**

In `internal/server/tools.go`, change `updateArgs`:

```go
type updateArgs struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Shared  *bool  `json:"shared,omitempty" jsonschema:"omit to keep current visibility; true=shared, false=private"`
}
```

Replace `updateMemory`:

```go
func (d *deps) updateMemory(ctx context.Context, a updateArgs) error {
	vec, err := d.em.Embed(ctx, a.Content)
	if err != nil {
		return err
	}
	return d.st.Update(ctx, a.ID, ownerFromContext(ctx), a.Content, a.Shared, vec)
}
```

- [ ] **Step 5: Run tests + build**

Run: `go build ./... && MEM_QDRANT_TEST_ADDR=localhost:6334 go test ./internal/store/ -run TestUpdateOwnerGateAndSharedFlag -v`
Expected: build OK; PASS.

- [ ] **Step 6: Commit**

```bash
jj commit -m "feat(store): owner-gate Update + shared *bool (engram-ir1)"
```

---

### Task 7: `set_visibility` tool + `SetVisibility` (no re-embed)

**Files:**

- Modify: `internal/store/store.go` (`SetVisibility`)
- Modify: `internal/server/tools.go` (`setVisibilityArgs`, `Register`)
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go`:

```go
func TestSetVisibilityOwnerGate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:vis"
	defer func() { _ = s.DeleteAllRaw(ctx, scope) }()
	m := Memory{ID: "a1a1a1a1-0000-0000-0000-000000000001", Content: "v", Scope: scope, Owner: "sub-B", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Non-owner denied.
	if err := s.SetVisibility(ctx, m.ID, "sub-A", true); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-owner set_visibility: want ErrNotFound, got %v", err)
	}
	// Owner shares, then unshares; vector is preserved (SetPayload, not Upsert).
	if err := s.SetVisibility(ctx, m.ID, "sub-B", true); err != nil {
		t.Fatalf("share: %v", err)
	}
	got, _ := s.Get(ctx, m.ID)
	if got.Visibility != "shared" {
		t.Errorf("share failed: %q", got.Visibility)
	}
	if err := s.SetVisibility(ctx, m.ID, "sub-B", false); err != nil {
		t.Fatalf("unshare: %v", err)
	}
	got, _ = s.Get(ctx, m.ID)
	if got.Visibility != "" {
		t.Errorf("unshare failed: %q", got.Visibility)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./... 2>&1 | head`
Expected: FAIL — compile error: `undefined: s.SetVisibility`.

- [ ] **Step 3: Add `SetVisibility`**

In `internal/store/store.go`, after `Update`:

```go
// SetVisibility flips a record's shared flag without re-embedding (uses
// SetPayload, preserving the vector), only if owned by sub.
func (s *Store) SetVisibility(ctx context.Context, id, sub string, shared bool) error {
	if _, err := s.getWritable(ctx, id, sub); err != nil {
		return err
	}
	vis := ""
	if shared {
		vis = "shared"
	}
	_, err := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"visibility": vis}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	})
	return err
}
```

- [ ] **Step 4: Register the `set_visibility` tool**

In `internal/server/tools.go`, add the args struct near the other arg types:

```go
type setVisibilityArgs struct {
	ID     string `json:"id"`
	Shared bool   `json:"shared" jsonschema:"true = readable by any authenticated caller; false = private"`
}
```

In `Register`, after the `search_discovery` tool registration (the last `AddTool` call):

```go
	mcp.AddTool(s, &mcp.Tool{Name: "set_visibility", Description: "Share or unshare a memory you own. shared=true → readable by any authenticated caller (never writable by others); false → private."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a setVisibilityArgs) (*mcp.CallToolResult, any, error) {
			err := d.st.SetVisibility(ctx, a.ID, ownerFromContext(ctx), a.Shared)
			return textResult("visibility updated"), nil, err
		})
```

- [ ] **Step 5: Run tests + build**

Run: `go build ./... && MEM_QDRANT_TEST_ADDR=localhost:6334 go test ./internal/store/ -run TestSetVisibilityOwnerGate -v`
Expected: build OK; PASS.

- [ ] **Step 6: Commit**

```bash
jj commit -m "feat: set_visibility tool + owner-gated SetVisibility (engram-99z)"
```

---

### Task 8: Discovery overwrite gate — `OwnedOrAbsent`

**Files:**

- Modify: `internal/store/store.go` (`OwnedOrAbsent`)
- Modify: `internal/server/tools.go` (`storeDiscovery`)
- Test: `internal/store/store_test.go`, `internal/server/tools_test.go`

- [ ] **Step 1: Write the failing store test**

Add to `internal/store/store_test.go`:

```go
func TestOwnedOrAbsent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "discovery:repo:owned"
	defer func() { _ = s.DeleteAllRaw(ctx, scope) }()
	id := "b2b2b2b2-0000-0000-0000-000000000001"
	// Absent id → ok (caller will create).
	if err := s.OwnedOrAbsent(ctx, id, "sub-A"); err != nil {
		t.Errorf("absent id should be ok: %v", err)
	}
	m := Memory{ID: id, Content: "d", Scope: scope, Category: "discovery", Kind: "fact", Owner: "sub-A", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Owner re-supplies id → ok (replace).
	if err := s.OwnedOrAbsent(ctx, id, "sub-A"); err != nil {
		t.Errorf("owner replace should be ok: %v", err)
	}
	// Other actor supplies id → ErrNotFound (refuse overwrite).
	if err := s.OwnedOrAbsent(ctx, id, "sub-B"); !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-owner overwrite: want ErrNotFound, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./... 2>&1 | head`
Expected: FAIL — compile error: `undefined: s.OwnedOrAbsent`.

- [ ] **Step 3: Add `OwnedOrAbsent`**

In `internal/store/store.go`, after `getWritable`:

```go
// OwnedOrAbsent permits a client-supplied-id write: nil if the id is absent (new
// record) or already owned by sub (replace in place); ErrNotFound if it exists
// and is owned by another actor (refuse cross-owner overwrite). Transport errors
// surface unchanged.
func (s *Store) OwnedOrAbsent(ctx context.Context, id, sub string) error {
	m, err := s.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if m.Owner != sub {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return nil
}
```

- [ ] **Step 4: Gate the discovery overwrite in the handler**

In `internal/server/tools.go`, `storeDiscovery`, add the gate right after validation (before embedding). The `Owner: ownerFromContext(ctx)` line was already added in Task 1 — reuse the `sub` local here:

```go
func (d *deps) storeDiscovery(ctx context.Context, a storeDiscoveryArgs) (string, error) {
	if err := validateStoreDiscovery(a); err != nil {
		return "", err
	}
	sub := ownerFromContext(ctx)
	if a.ID != "" {
		if err := d.st.OwnedOrAbsent(ctx, a.ID, sub); err != nil {
			return "", err
		}
	}
	vec, err := d.em.Embed(ctx, a.Content)
	if err != nil {
		return "", err
	}
	cites := make([]store.Citation, len(a.Citations))
	for i, c := range a.Citations {
		cites[i] = store.Citation{Kind: c.Kind, Ref: c.Ref, Locator: c.Locator, Pin: c.Pin, Excerpt: c.Excerpt}
	}
	id := a.ID
	if id == "" {
		id = uuid.NewString()
	}
	m := store.Memory{
		ID:        id,
		Content:   a.Content,
		Scope:     a.Scope,
		Source:    "agent-inferred",
		Category:  "discovery",
		Kind:      a.Kind,
		Citations: cites,
		Summary:   a.Summary,
		Tags:      a.Tags,
		Actor:     actorFromContext(ctx),
		Owner:     sub,
		CreatedAt: time.Now().UTC(),
	}
	return m.ID, d.st.Upsert(ctx, m, vec)
}
```

- [ ] **Step 5: Run tests + build**

Run: `go build ./... && MEM_QDRANT_TEST_ADDR=localhost:6334 go test ./internal/store/ -run TestOwnedOrAbsent -v`
Expected: build OK; PASS.

- [ ] **Step 6: Commit**

```bash
jj commit -m "feat(store): OwnedOrAbsent gate for discovery id-overwrite (engram-ir1/f7h.2)"
```

---

### Task 9: Owner-scoped `DeleteAll`

**Files:**

- Modify: `internal/store/store.go` (`DeleteAll`, remove unused `scopeFilter`)
- Modify: `internal/server/tools.go` (`delete_all` handler)
- Test: `internal/store/store_test.go` (new test + fix existing `DeleteAll` call-sites)

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go`:

```go
func TestDeleteAllOwnerScoped(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:delall"
	defer func() { _ = s.DeleteAllRaw(ctx, scope) }()
	a := Memory{ID: "c3c3c3c3-0000-0000-0000-000000000001", Content: "a", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
	b := Memory{ID: "c3c3c3c3-0000-0000-0000-000000000002", Content: "b", Scope: scope, Owner: "sub-B", CreatedAt: time.Now().UTC()}
	for _, m := range []Memory{a, b} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	// A's teardown removes only A's record; B's survives.
	if err := s.DeleteAll(ctx, scope, "sub-A"); err != nil {
		t.Fatalf("deleteAll: %v", err)
	}
	if _, err := s.Get(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("A's record should be gone: %v", err)
	}
	if _, err := s.Get(ctx, b.ID); err != nil {
		t.Errorf("B's record should survive A's teardown: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./... 2>&1 | head`
Expected: FAIL — compile error: `too many arguments in call to s.DeleteAll`.

- [ ] **Step 3: Owner-scope `DeleteAll` and drop `scopeFilter`**

In `internal/store/store.go`, replace `DeleteAll`:

```go
// DeleteAll removes the caller's OWN records in scope (never another owner's,
// and never another owner's shared records).
func (s *Store) DeleteAll(ctx context.Context, scope, sub string) error {
	filter := &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		qdrant.NewMatch("owner", sub),
	}}
	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelectorFilter(filter),
	})
	return err
}
```

`scopeFilter` is now unused (Search/List moved to `ownerScopeFilter` in Task 2; this was its last caller). Delete the `scopeFilter` method to keep the linter clean.

- [ ] **Step 4: Update the `delete_all` handler**

In `internal/server/tools.go`, `Register`, the `delete_all` handler:

```go
			func(ctx context.Context, _ *mcp.CallToolRequest, a scopeArgs) (*mcp.CallToolResult, any, error) {
				err := d.st.DeleteAll(ctx, a.Scope, ownerFromContext(ctx))
				return textResult("scope cleared"), nil, err
			})
```

- [ ] **Step 5: Fix existing `DeleteAll` call-sites**

These tests' records have `Owner == ""`; pass `sub=""` so the owner-scoped delete matches them.

In `internal/store/store_test.go`, `TestSearchDiscoveryFilters`:

```go
	defer func() { _ = s.DeleteAll(ctx, scopeA, ""); _ = s.DeleteAll(ctx, scopeB, ""); _ = s.DeleteAll(ctx, curated, "") }()
```

In `internal/store/store_test.go`, `TestSearchAndDeleteAll` (line ~212):

```go
	if err := s.DeleteAll(ctx, scope, ""); err != nil {   // was s.DeleteAll(ctx, scope)
		t.Fatalf("delete_all: %v", err)
	}
```

In `internal/server/tools_test.go`, `TestStoreAndSearchDiscoveryHandlers`:

```go
	defer func() { _ = d.st.DeleteAll(ctx, scope, "") }()
```

- [ ] **Step 6: Run tests + build**

Run: `go build ./... && MEM_QDRANT_TEST_ADDR=localhost:6334 go test ./internal/store/ ./internal/server/ -run 'TestDeleteAllOwnerScoped|TestSearchDiscoveryFilters|TestStoreAndSearchDiscoveryHandlers|TestSearchAndDeleteAll' -v`
Expected: build OK; all PASS.

- [ ] **Step 7: Commit**

```bash
jj commit -m "feat(store): owner-scope DeleteAll teardown (engram-99z)"
```

---

### Task 10: Migration — `StoreFromEnv` + `MigrateSetOwner` + `migrate-set-owner` CLI

**Files:**

- Modify: `internal/server/tools.go` (`StoreFromEnv`, refactor `buildDepsFromEnv`)
- Modify: `internal/store/store.go` (`MigrateSetOwner`)
- Create: `cmd/engram/migrate.go`
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go`:

```go
func TestMigrateSetOwner(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:migrate"
	defer func() { _ = s.DeleteAllRaw(ctx, scope) }()

	// Refuses empty owner.
	if _, err := s.MigrateSetOwner(ctx, ""); err == nil {
		t.Error("empty owner: expected error")
	}

	// A record written WITHOUT the owner key (simulating a pre-isolation record):
	// build the payload, strip "owner", and upsert raw.
	id := "d4d4d4d4-0000-0000-0000-000000000001"
	p := payload(Memory{ID: id, Content: "legacy", Scope: scope, CreatedAt: time.Now().UTC()})
	delete(p, "owner")
	if _, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: []*qdrant.PointStruct{{
			Id: qdrant.NewID(id), Vectors: qdrant.NewVectors(0.1, 0.2, 0.3),
			Payload: qdrant.NewValueMap(p),
		}},
	}); err != nil {
		t.Fatalf("raw upsert: %v", err)
	}

	n, err := s.MigrateSetOwner(ctx, "sub-OWNER")
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if n == 0 {
		t.Fatal("migrate stamped 0 records, want >= 1")
	}
	got, _ := s.Get(ctx, id)
	if got.Owner != "sub-OWNER" {
		t.Errorf("owner not stamped: %q", got.Owner)
	}
	// Idempotent: a second run stamps nothing (the record now has an owner).
	n2, err := s.MigrateSetOwner(ctx, "sub-OWNER")
	if err != nil {
		t.Fatalf("migrate rerun: %v", err)
	}
	if n2 != 0 {
		t.Errorf("idempotency: rerun stamped %d, want 0", n2)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./... 2>&1 | head`
Expected: FAIL — compile error: `undefined: s.MigrateSetOwner`.

- [ ] **Step 3: Add `MigrateSetOwner`**

In `internal/store/store.go`, at the end of the file:

```go
// MigrateSetOwner backfills owner onto every record that lacks an owner key
// (records written before per-actor isolation). Idempotent: records that already
// carry an owner are not matched. NewIsEmpty matches missing/null keys but not an
// empty string, so auth-disabled records (owner=="") are intentionally left
// alone. Returns the number of records stamped.
func (s *Store) MigrateSetOwner(ctx context.Context, owner string) (uint64, error) {
	if owner == "" {
		return 0, fmt.Errorf("owner must be non-empty")
	}
	missing := &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewIsEmpty("owner")}}
	cnt, err := s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: s.collection, Filter: missing, Exact: qdrant.PtrOf(true),
	})
	if err != nil {
		return 0, err
	}
	if cnt == 0 {
		return 0, nil
	}
	_, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"owner": owner}),
		PointsSelector: qdrant.NewPointsSelectorFilter(missing),
	})
	if err != nil {
		return 0, err
	}
	return cnt, nil
}
```

- [ ] **Step 4: Run the store test to verify it passes**

Run: `MEM_QDRANT_TEST_ADDR=localhost:6334 go test ./internal/store/ -run TestMigrateSetOwner -v`
Expected: PASS.

- [ ] **Step 5: Extract `StoreFromEnv` in the server package**

In `internal/server/tools.go`, add `"fmt"` is already imported. Add `StoreFromEnv` and refactor `buildDepsFromEnv` to reuse it:

```go
// StoreFromEnv builds a Qdrant-backed Store from the MEM_QDRANT_* / MEM_EMBED_DIM
// environment and ensures the collection exists. Shared by the server bootstrap
// and the migrate-set-owner command.
func StoreFromEnv() (*store.Store, error) {
	qdrantAddr := EnvOr("MEM_QDRANT_ADDR", "localhost:6334")
	collection := EnvOr("MEM_QDRANT_COLLECTION", "mem_eval")
	embedDimStr := EnvOr("MEM_EMBED_DIM", "1024")
	embedDim, err := strconv.ParseUint(embedDimStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid MEM_EMBED_DIM %q: %w", embedDimStr, err)
	}
	host, portStr, err := net.SplitHostPort(qdrantAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid MEM_QDRANT_ADDR %q: %w", qdrantAddr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port in MEM_QDRANT_ADDR %q: %w", qdrantAddr, err)
	}
	qc, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: port})
	if err != nil {
		return nil, fmt.Errorf("qdrant client: %w", err)
	}
	st := store.New(qc, collection)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := st.EnsureCollection(ctx, embedDim); err != nil {
		return nil, fmt.Errorf("EnsureCollection: %w", err)
	}
	return st, nil
}
```

Replace the body of `buildDepsFromEnv` with:

```go
func buildDepsFromEnv() *deps {
	st, err := StoreFromEnv()
	if err != nil {
		log.Fatalf("%v", err)
	}
	litellmURL := EnvOr("MEM_LITELLM_URL", "http://localhost:4000")
	litellmKey := EnvOr("MEM_LITELLM_KEY", "")
	embedModel := EnvOr("MEM_EMBED_MODEL", "ollama/bge-m3")
	em := embed.New(litellmURL, litellmKey, embedModel)
	return &deps{st: st, em: em}
}
```

- [ ] **Step 6: Create the `migrate-set-owner` command**

Create `cmd/engram/migrate.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/server"
)

var migrateOwner string

// migrateSetOwnerCmd backfills the stable OIDC `sub` onto memory records written
// before per-actor isolation (which carry no `owner` key). One-time, idempotent.
// Run it with OIDC enabled and your real `sub`, so enabling auth keeps the
// records yours rather than orphaning them in the anonymous bucket.
var migrateSetOwnerCmd = &cobra.Command{
	Use:   "migrate-set-owner",
	Short: "Backfill owner (OIDC sub) onto pre-isolation memory records",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if migrateOwner == "" {
			return fmt.Errorf("--owner (or MEM_MIGRATE_OWNER) is required and must be a non-empty OIDC sub")
		}
		st, err := server.StoreFromEnv()
		if err != nil {
			return err
		}
		n, err := st.MigrateSetOwner(context.Background(), migrateOwner)
		if err != nil {
			return err
		}
		cmd.Printf("stamped owner=%s onto %d owner-less record(s)\n", migrateOwner, n)
		return nil
	},
}

func init() {
	migrateSetOwnerCmd.Flags().StringVar(&migrateOwner, "owner",
		server.EnvOr("MEM_MIGRATE_OWNER", ""),
		"OIDC sub to stamp onto owner-less records (required, non-empty)")
	rootCmd.AddCommand(migrateSetOwnerCmd)
}
```

(The command self-registers via `init()`; `root.go` needs no edit.)

- [ ] **Step 7: Verify the command wires up + build + full store suite**

Run: `go build ./... && go run ./cmd/engram migrate-set-owner --help`
Expected: build OK; help shows the `--owner` flag.

Run: `MEM_QDRANT_TEST_ADDR=localhost:6334 go test ./internal/store/ -v`
Expected: all store tests PASS.

- [ ] **Step 8: Commit**

```bash
jj commit -m "feat: migrate-set-owner command + StoreFromEnv + MigrateSetOwner (engram-99z)"
```

---

### Task 11: Handler wiring test + docs

**Files:**

- Test: `internal/server/tools_test.go`
- Modify: `README.md`, `CLAUDE.md`

- [ ] **Step 1: Write the handler wiring test**

The go-sdk token-context key is unexported, so handler tests run with `sub==""`. This test confirms the `update_memory` handler threads `Shared *bool` correctly (content-only update preserves visibility) end-to-end through the store. Add to `internal/server/tools_test.go`:

```go
func TestUpdateMemoryPreservesSharingHandler(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "iso-test:project:handler-upd"
	id := "e5e5e5e5-0000-0000-0000-000000000001"
	defer func() { _ = d.st.DeleteAll(ctx, scope, "") }()

	// Seed a shared record owned by the anonymous caller (sub == "").
	m := store.Memory{ID: id, Content: "v1", Scope: scope, Owner: "", Visibility: "shared", CreatedAt: timeNow()}
	if err := d.st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Content-only update (Shared nil) must preserve "shared".
	if err := d.updateMemory(ctx, updateArgs{ID: id, Content: "v2"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := d.st.Get(ctx, id)
	if got.Content != "v2" || got.Visibility != "shared" {
		t.Errorf("handler content-only update lost sharing: content=%q visibility=%q", got.Content, got.Visibility)
	}
}

// timeNow is a tiny indirection so the test reads cleanly; store records require
// a CreatedAt.
func timeNow() time.Time { return time.Now().UTC().Truncate(time.Second) }
```

Add `"time"` to the `internal/server/tools_test.go` import block.

- [ ] **Step 2: Run the handler test**

Run: `MEM_QDRANT_TEST_ADDR=localhost:6334 go test ./internal/server/ -run TestUpdateMemoryPreservesSharingHandler -v`
Expected: PASS.

- [ ] **Step 3: Update the README memory contract**

In `README.md`, the "A memory record carries …" paragraph: add `owner` and `visibility` to the field list and append the isolation note. Replace the existing sentence fragment:

```markdown
A memory record carries `content`, `scope`, `repo`/`workspace`/`worktree_path`/
`base_dir`, `source` (`user-said` | `agent-inferred`), `category`, `tags`,
`actor` (the verified caller identity — server-set, never client-supplied),
`owner` (the caller's stable OIDC `sub`, the authorization key — server-set),
`visibility` (`private` by default, or `shared`), and `created_at`.

**Isolation:** each actor reads and writes only their **own** records; a record
can be marked `shared` (via `set_visibility` or `update_memory`'s `shared` flag)
to make it readable by any authenticated caller — sharing grants read, never
write. Isolation **requires authentication**: with no `--oidc-issuer`, all
callers share one anonymous bucket. The `owner` is the stable `sub`, so a
changed email never revokes access. New deployments with pre-existing records
must run `engram migrate-set-owner --owner <sub>` once.
```

- [ ] **Step 4: Update the CLAUDE.md memory contract**

In `CLAUDE.md`, under "## Memory contract (stable)", append to the record-fields sentence and add an isolation line. Replace:

```markdown
A record carries `content`,
`scope`, repo/workspace/worktree/base_dir, `source`, `category`, `tags`,
`actor` (verified caller — server-set, never client-supplied), `owner` (caller's
stable OIDC `sub`, the authz key — server-set), `visibility` (`private` default |
`shared`), `created_at`. Design intent: explicit, zero-junk, correctable. Do not
add auto-extraction.

**Isolation (authz):** each actor sees/mutates only their own records; `shared`
records are readable (never writable) by any authenticated caller. No issuer →
single anonymous bucket. `set_visibility` and `update_memory --shared` toggle
sharing. Backfill legacy records with `engram migrate-set-owner --owner <sub>`.
```

- [ ] **Step 5: Run lint + the full suite**

Run: `task lint && go build ./... && MEM_QDRANT_TEST_ADDR=localhost:6334 go test ./...`
Expected: lint clean (rumdl/yamlfmt/golangci); build OK; all tests PASS (handler/store tests need the Qdrant addr; others run regardless).

- [ ] **Step 6: Commit**

```bash
jj commit -m "feat: handler wiring test + document owner/visibility isolation (engram-99z)"
```

---

## Verification checklist (after all tasks)

- [ ] `go build ./...` clean.
- [ ] `MEM_QDRANT_TEST_ADDR=localhost:6334 go test ./...` — all isolation tests pass.
- [ ] `task lint` clean (golangci-lint, rumdl, yamlfmt).
- [ ] `task license:check` clean (new `cmd/engram/migrate.go` carries the SPDX header).
- [ ] Manual: two distinct tokens cannot read/update/delete each other's private records; a `shared` record is readable but not writable by the other; `cross_spine` returns own+shared only.
- [ ] `engram migrate-set-owner --owner <sub>` stamps legacy records; rerun reports 0.
