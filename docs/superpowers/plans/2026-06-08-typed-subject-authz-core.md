<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Typed `Subject` authz-core refactor — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the bare `sub string` caller-identity parameter threaded through the engram authz core with a sealed `store.Subject` sum type (`Anonymous` | `Authenticated(sub)`), so the anonymous/authenticated distinction is a type and an ignored extraction error fails closed by construction.

**Architecture:** A sealed interface `Subject` lives in `internal/store` (the authorization enforcement point, per ADR engram-cgb) with unexported variants `anonymous{}` / `authenticated{sub}` and exported constructors `Anonymous()` / `Authenticated(sub)`. Its zero value is `nil` (not anonymous), so discarding the extraction error fails closed. Store methods switch over the variants with a `default`-deny arm; `server` produces the `Subject` from the request context via `subjectFromContext`. The refactor is **semantics-preserving** — identical owner/shared/anonymous logic, only the carrier type changes; no Qdrant payload, MCP API, or behavior change.

**Tech Stack:** Go, Qdrant (`github.com/qdrant/go-client/qdrant`), MCP go-sdk (`github.com/modelcontextprotocol/go-sdk`). Tests run against an ephemeral Qdrant via testcontainers (existing `TestMain` harness). Quality gate: `task` (= lint + test); formatting gate `gofmt -l` (the repo's golangci-lint config does NOT run gofmt, so `gofmt -l` must be checked separately before every commit).

**Build-green invariant:** Every task leaves `go build ./...`, `gofmt -l` (empty), `go vet ./...`, `golangci-lint run`, and `go test ./...` all passing. Because a store signature change breaks every caller until updated, each conversion task converts a store method **together with its handler call sites and its tests** in one commit. New symbols are introduced only in the task that first references them (an unused unexported func/type fails golangci-lint's `unused` check), except where a unit test in the same task references them.

---

## File structure

| File | Responsibility | Tasks |
|------|----------------|-------|
| `internal/store/subject.go` (new) | The `Subject` sealed type, variants, constructors, `Owner()`, and `matchNothing()` filter helper | 1, 3 |
| `internal/store/subject_test.go` (new) | Unit tests for the type (constructors, `Owner()`, sealed-ness) | 1 |
| `internal/store/store.go` | Convert the 11 sub-gated methods + `ownerScopeFilter` to take `Subject`; switch-with-`default`-deny | 3,4,5,6,7 |
| `internal/store/store_test.go` | Migrate ~70 `sub`-string test literals; add nil-`Subject` default-deny tests | 3,4,5,6,7,9 |
| `internal/server/tools.go` | Add `subjectFromContext`; convert handlers; stamp `Memory.Owner = subj.Owner()`; retire `ownerFromContext` | 2,3,4,5,6,7,8 |
| `internal/server/tools_test.go` | `TestSubjectFromContextNoToken`; migrate handler-test literals; drop `TestOwnerFromContextNoToken` | 2,3,4,5,6,8 |
| `internal/server/instrument.go` | Convert `identityForLog` to `subjectFromContext` (display-only, nil-safe) | 8 |

---

### Task 1: Introduce the `Subject` type

**Files:**

- Create: `internal/store/subject.go`
- Test: `internal/store/subject_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/store/subject_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import "testing"

func TestSubjectOwner(t *testing.T) {
	if got := Anonymous().Owner(); got != "" {
		t.Errorf("Anonymous().Owner() = %q, want \"\"", got)
	}
	if got := Authenticated("sub-A").Owner(); got != "sub-A" {
		t.Errorf("Authenticated(sub-A).Owner() = %q, want \"sub-A\"", got)
	}
}

// TestSubjectZeroValueIsNil documents the load-bearing property: the zero value
// of the Subject interface is nil, NOT Anonymous — so discarding the extraction
// error (subj, _ := subjectFromContext(...)) yields nil, which fails closed at
// the store default arm rather than silently granting the anonymous bucket.
func TestSubjectZeroValueIsNil(t *testing.T) {
	var zero Subject
	if zero != nil {
		t.Errorf("zero Subject = %v, want nil", zero)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run 'TestSubject' -v`
Expected: FAIL — compile error `undefined: Anonymous`, `undefined: Authenticated`, `undefined: Subject`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/store/subject.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

// Subject is the verified caller identity used for authorization. It is a sealed
// sum: exactly Anonymous (auth disabled — the owner=="" bucket) or Authenticated
// (a verified, non-empty OIDC sub). The concrete variants are unexported, so the
// union cannot be extended or constructed outside this package; callers use the
// Anonymous()/Authenticated() constructors. The zero value is nil (not
// Anonymous): a discarded extraction error yields nil, which fails closed at the
// store default arm rather than silently granting the anonymous bucket.
type Subject interface {
	isSubject()
	// Owner is the persistence/stamping accessor: the owner string this subject
	// writes onto Memory.Owner ("" for anonymous, sub for authenticated). It is
	// NOT an enforcement accessor — read filters and write gates use the
	// exhaustive type switch (with its default-deny arm), never Owner(). Calling
	// Owner() on a nil Subject panics (loud), never a silent anonymous grant.
	Owner() string
}

type anonymous struct{}

func (anonymous) isSubject()    {}
func (anonymous) Owner() string { return "" }

type authenticated struct{ sub string }

func (authenticated) isSubject()      {}
func (a authenticated) Owner() string { return a.sub }

// Anonymous is the caller when auth is disabled (the owner=="" bucket).
func Anonymous() Subject { return anonymous{} }

// Authenticated is a caller carrying a verified, non-empty OIDC sub.
func Authenticated(sub string) Subject { return authenticated{sub: sub} }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run 'TestSubject' -v`
Expected: PASS (both tests).

- [ ] **Step 5: Verify gates and commit**

Run: `gofmt -l internal/store/ && go vet ./internal/store/ && golangci-lint run ./internal/store/...`
Expected: empty gofmt output, no vet/lint errors. (`unused` does not fire: `subject_test.go` references `Anonymous`, `Authenticated`, `Owner`, `Subject`.)

Commit (jj): `jj commit -m "feat(store): sealed Subject type (Anonymous | Authenticated) (engram-6tl.5)"`

---

### Task 2: Add `subjectFromContext`

**Files:**

- Modify: `internal/server/tools.go` (add after `ownerFromContext`, ~line 339)
- Test: `internal/server/tools_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/server/tools_test.go`:

```go
// TestSubjectFromContextNoToken pins the auth-disabled half of the contract: no
// token in context yields (Anonymous, nil), NOT an error — otherwise a no-issuer
// deployment would reject every request. The fail-closed path (a validated token
// lacking a non-empty sub → error) cannot be unit-tested here because the go-sdk
// stores TokenInfo under an unexported context key, so there is no way to inject
// one; it is exercised through the handler middleware helper authedContext.
func TestSubjectFromContextNoToken(t *testing.T) {
	subj, err := subjectFromContext(context.Background())
	if err != nil {
		t.Fatalf("no token: unexpected error %v", err)
	}
	if subj == nil || subj.Owner() != "" {
		t.Errorf("no token: want non-nil Anonymous (Owner==\"\"), got %#v", subj)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run 'TestSubjectFromContextNoToken' -v`
Expected: FAIL — compile error `undefined: subjectFromContext`.

- [ ] **Step 3: Write minimal implementation**

Add to `internal/server/tools.go` immediately after `ownerFromContext` (keep `ownerFromContext` for now — other handlers still call it; it is retired in Task 8):

```go
// subjectFromContext returns the verified caller as a store.Subject. No token in
// context → Anonymous (auth disabled, the owner=="" bucket). A validated token
// carrying a non-empty `sub` → Authenticated(sub). A validated-but-malformed
// token (no non-empty sub) is a fail-closed error, never silently collapsed into
// the anonymous bucket. This replaces ownerFromContext: its nil-on-error return
// (vs ""-on-error) means a discarded error fails closed at the store default arm.
func subjectFromContext(ctx context.Context) (store.Subject, error) {
	ti := mcpauth.TokenInfoFromContext(ctx)
	if ti == nil {
		return store.Anonymous(), nil
	}
	if sub, ok := ti.Extra["sub"].(string); ok && sub != "" {
		return store.Authenticated(sub), nil
	}
	return nil, fmt.Errorf("validated token missing subject")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -run 'TestSubjectFromContextNoToken' -v`
Expected: PASS.

- [ ] **Step 5: Verify gates and commit**

Run: `gofmt -l internal/server/ && go vet ./internal/server/ && golangci-lint run ./internal/server/...`
Expected: clean. (`subjectFromContext` is referenced by its test, so `unused` does not fire; `ownerFromContext` is still referenced by the un-converted handlers.)

Commit (jj): `jj commit -m "feat(server): subjectFromContext returning store.Subject (engram-6tl.5)"`

---

### Task 3: Convert the read-filter cluster (`ownerOrSharedCondition`, `ownerScopeFilter`, `Search`, `List`, `SearchDiscovery`)

These four read paths flow through `ownerScopeFilter` / `ownerOrSharedCondition`, so converting that helper's signature forces all of them in one commit. Adds `matchNothing()` (first production use here, so it is introduced now).

**Files:**

- Modify: `internal/store/subject.go` (add `matchNothing`)
- Modify: `internal/store/store.go:214-320` (`ownerOrSharedCondition`, `ownerScopeFilter`, `Search`, `SearchDiscovery`, `List`)
- Modify: `internal/server/tools.go` (`searchMemory`, `listMemory`, `searchDiscovery` handlers)
- Test: `internal/store/store_test.go`, `internal/server/tools_test.go`

- [ ] **Step 1: Convert `ownerOrSharedCondition` and add `matchNothing`**

In `internal/store/store.go`, replace `ownerOrSharedCondition` (currently lines 223-234):

```go
// ownerOrSharedCondition matches records the subject may READ.
//
// Authenticated: owner==sub OR visibility=="shared".
// Anonymous: owner=="" ONLY — shared records require an authenticated subject;
// the anonymous bucket is not a back-door to all shared records.
// nil/unknown (a discarded extraction error): matchNothing — fail closed.
func ownerOrSharedCondition(subj Subject) *qdrant.Condition {
	switch s := subj.(type) {
	case authenticated:
		return qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
			qdrant.NewMatch("owner", s.sub),
			qdrant.NewMatch("visibility", visibilityShared),
		}})
	case anonymous:
		return qdrant.NewFilterAsCondition(&qdrant.Filter{Must: []*qdrant.Condition{
			qdrant.NewMatch("owner", ""),
		}})
	default:
		return matchNothing()
	}
}
```

In `internal/store/subject.go`, append:

```go
// matchNothing returns a condition no record can satisfy (owner==x AND owner!=x).
// It backs the fail-closed default arm of read-filter switches when the Subject
// is nil/unknown — a query then returns zero rows rather than over-returning.
func matchNothing() *qdrant.Condition {
	const x = "\x00engram-no-such-owner"
	return qdrant.NewFilterAsCondition(&qdrant.Filter{
		Must:    []*qdrant.Condition{qdrant.NewMatch("owner", x)},
		MustNot: []*qdrant.Condition{qdrant.NewMatch("owner", x)},
	})
}
```

Add the qdrant import to `subject.go` (it currently has none):

```go
import "github.com/qdrant/go-client/qdrant"
```

- [ ] **Step 2: Convert `ownerScopeFilter`, `Search`, `List`, `SearchDiscovery` signatures**

In `internal/store/store.go`, change the four signatures and the `ownerScopeFilter` parameter, replacing `sub string` with `subj Subject` and the call `ownerOrSharedCondition(sub)` with `ownerOrSharedCondition(subj)`:

```go
func (s *Store) ownerScopeFilter(scope string, subj Subject) *qdrant.Filter {
	return &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		ownerOrSharedCondition(subj),
	}}
}

func (s *Store) Search(ctx context.Context, scope string, subj Subject, vec []float32, k uint64) ([]Memory, error) {
	res, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collection, Query: qdrant.NewQuery(vec...),
		Filter: s.ownerScopeFilter(scope, subj), Limit: qdrant.PtrOf(k), WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	return memoriesFromPoints(res), nil
}

func (s *Store) List(ctx context.Context, scope string, subj Subject, limit uint64) ([]Memory, error) {
	const scanCap = 1000
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         s.ownerScopeFilter(scope, subj),
		Limit:          qdrant.PtrOf(uint32(scanCap)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Memory, 0, len(pts))
	for _, p := range pts {
		out = append(out, fromPayload(p.Id.GetUuid(), p.Payload))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if limit > 0 && uint64(len(out)) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Store) SearchDiscovery(ctx context.Context, scope, kind string, subj Subject, vec []float32, k uint64) ([]Memory, error) {
	must := []*qdrant.Condition{qdrant.NewMatch("category", "discovery")}
	if scope != "" {
		must = append(must, qdrant.NewMatch("scope", scope))
	}
	if kind != "" {
		must = append(must, qdrant.NewMatch("kind", kind))
	}
	must = append(must, ownerOrSharedCondition(subj))
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

- [ ] **Step 3: Convert the three handlers**

In `internal/server/tools.go`, in `searchMemory`, `listMemory`, `searchDiscovery`, replace the `owner, err := ownerFromContext(ctx)` extraction with `subj, err := subjectFromContext(ctx)` and pass `subj` to the store call. Concretely:

`listMemory` (lines 341-350) becomes:

```go
func (d *deps) listMemory(ctx context.Context, a listArgs) ([]store.Memory, error) {
	if a.Limit == 0 {
		a.Limit = 20
	}
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return d.st.List(ctx, a.Scope, subj, a.Limit)
}
```

`searchMemory` (lines 352-365): replace `owner, err := ownerFromContext(ctx)` with `subj, err := subjectFromContext(ctx)` and the final call with `return d.st.Search(ctx, a.Scope, subj, vec, a.K)`.

`searchDiscovery` (lines 380-402): replace `owner, err := ownerFromContext(ctx)` with `subj, err := subjectFromContext(ctx)` and the final call with `return d.st.SearchDiscovery(ctx, scope, a.Kind, subj, vec, a.K)`.

- [ ] **Step 4: Migrate the tests for these paths**

**Migration completeness (applies to the test migration in Tasks 3-7).** The signature change makes every un-converted call site a compile error (`cannot use "" (string) as store.Subject`), so the build-green gate enforces completeness — you cannot miss one. Find them all up front rather than discovering them via failed builds:

```text
rg -n '\.(Search|List|SearchDiscovery|GetReadable|getWritable|FetchForUpdate|SetVisibility|Delete|OwnedOrAbsent|DeleteAll)\(' internal/store/store_test.go internal/server/tools_test.go
```

Named tests are the primary ones, but the sweep also catches the anonymous-bucket tests (`TestAnonBucketReadIsolation`, `TestAnonBucketWriteSemantics`, `TestAnonBucketDiscoveryReadIsolation`) and `TestSearchAndDeleteAll`, which also pass bare `""` / `"sub-X"` literals. Convert every flagged site for the method(s) in scope for the current task.

In `internal/store/store_test.go`, the store-level tests calling `Search` / `List` / `SearchDiscovery` are `TestSearchListOwnerIsolation`, `TestSearchDiscoveryFilters`, `TestSearchDiscoveryOwnerIsolation`, and the anonymous-read tests. Apply the mechanical migration **at every call to these three methods**:

- a `""` sub argument → `store.Anonymous()` (in-package: `Anonymous()`)
- a `"sub-X"` sub argument → `Authenticated("sub-X")`

Example — in `TestSearchListOwnerIsolation`, a call that read:

```go
hits, err := s.Search(ctx, scope, "sub-A", q, 10)
```

becomes:

```go
hits, err := s.Search(ctx, scope, Authenticated("sub-A"), q, 10)
```

and an anonymous call `s.List(ctx, scope, "", 20)` becomes `s.List(ctx, scope, Anonymous(), 20)`.

In `internal/server/tools_test.go`, the handler tests exercising these paths (`TestAnonReadIsolationHandlers`, `TestAuthedCrossActorSharedReadHandlers`, and any `searchDiscovery`/`listMemory`/`searchMemory` handler test) drive through `context.Background()` / `authedContext`, so they need **no literal change** — the handler now calls `subjectFromContext` internally. Confirm they still compile and pass.

- [ ] **Step 5: Run the gates**

Run: `gofmt -l internal/ && go vet ./... && golangci-lint run && go test ./internal/store/ ./internal/server/ -v`
Expected: clean; all migrated tests PASS. Anonymous-read isolation and cross-actor shared-read assertions unchanged in outcome (semantics-preserving).

- [ ] **Step 6: Commit**

`jj commit -m "refactor(store): thread Subject through read filters (Search/List/SearchDiscovery) (engram-6tl.5)"`

---

### Task 4: Convert the read gate (`GetReadable`)

**Files:**

- Modify: `internal/store/store.go:344-361`
- Modify: `internal/server/tools.go` (`get_memory` handler, lines 456-464)
- Test: `internal/store/store_test.go` (`TestGetReadableOwnerGate`)

- [ ] **Step 1: Convert `GetReadable`**

Replace `internal/store/store.go` lines 344-361:

```go
func (s *Store) GetReadable(ctx context.Context, id string, subj Subject) (Memory, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return Memory{}, err
	}
	switch sj := subj.(type) {
	case authenticated:
		if m.Owner != sj.sub && m.Visibility != visibilityShared {
			return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return m, nil
	case anonymous:
		if m.Owner != "" {
			return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return m, nil
	default:
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
}
```

- [ ] **Step 2: Convert the `get_memory` handler**

In `internal/server/tools.go`, the `get_memory` tool closure (lines 457-463) becomes:

```go
func(ctx context.Context, _ *mcp.CallToolRequest, a idArgs) (*mcp.CallToolResult, any, error) {
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return nil, nil, err
	}
	m, err := d.st.GetReadable(ctx, a.ID, subj)
	return textResult(m.Content), m, err
}
```

- [ ] **Step 3: Migrate `TestGetReadableOwnerGate`**

In `internal/store/store_test.go`, `TestGetReadableOwnerGate` calls `s.GetReadable(ctx, id, "sub-B")` / `"sub-A"`. Apply: `"sub-B"` → `Authenticated("sub-B")`, `"sub-A"` → `Authenticated("sub-A")`. Example:

```go
if _, err := s.GetReadable(ctx, priv.ID, Authenticated("sub-B")); err != nil {
	t.Errorf("owner denied own record: %v", err)
}
```

- [ ] **Step 4: Run the gates**

Run: `gofmt -l internal/ && go vet ./... && golangci-lint run && go test ./internal/store/ ./internal/server/ -run 'GetReadable|get_memory|Handler' -v`
Expected: clean; PASS.

- [ ] **Step 5: Commit**

`jj commit -m "refactor(store): GetReadable takes Subject with default-deny (engram-6tl.5)"`

---

### Task 5: Convert the write-gate cluster (`getWritable`, `FetchForUpdate`, `SetVisibility`, `Delete`)

All flow through `getWritable`, so they convert together.

**Files:**

- Modify: `internal/store/store.go:373-467` (`getWritable`, `FetchForUpdate`, `SetVisibility`, `Delete`)
- Modify: `internal/server/tools.go` (`updateMemory`, `delete_memory`, `set_visibility` handlers)
- Test: `internal/store/store_test.go` (`TestDeleteOwnerGate`, `TestUpdateOwnerGateAndSharedFlag`, `TestSetVisibilityOwnerGate`, `TestSetVisibilityTOCTOU`, `TestFetchForUpdate*`)

- [ ] **Step 1: Convert `getWritable`**

Replace `internal/store/store.go` lines 373-382:

```go
func (s *Store) getWritable(ctx context.Context, id string, subj Subject) (Memory, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return Memory{}, err
	}
	switch sj := subj.(type) {
	case authenticated:
		if m.Owner != sj.sub {
			return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return m, nil
	case anonymous:
		if m.Owner != "" {
			return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return m, nil
	default:
		return Memory{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
}
```

- [ ] **Step 2: Convert `FetchForUpdate`, `SetVisibility`, `Delete` signatures**

In `internal/store/store.go`:

```go
func (s *Store) FetchForUpdate(ctx context.Context, id string, subj Subject) (Memory, error) {
	return s.getWritable(ctx, id, subj)
}

func (s *Store) SetVisibility(ctx context.Context, id string, subj Subject, shared bool) error {
	if _, err := s.getWritable(ctx, id, subj); err != nil {
		return err
	}
	vis := ""
	if shared {
		vis = visibilityShared
	}
	_, err := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"visibility": vis}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	})
	return err
}

func (s *Store) Delete(ctx context.Context, id string, subj Subject) error {
	if _, err := s.getWritable(ctx, id, subj); err != nil {
		return err
	}
	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelector(qdrant.NewID(id)),
	})
	return err
}
```

- [ ] **Step 3: Convert the three handlers**

In `internal/server/tools.go`:

`updateMemory` (lines 404-422): replace `owner, err := ownerFromContext(ctx)` with `subj, err := subjectFromContext(ctx)`, and `cur, err := d.st.FetchForUpdate(ctx, a.ID, owner)` with `cur, err := d.st.FetchForUpdate(ctx, a.ID, subj)`.

`delete_memory` closure (lines 473-480): replace extraction with `subj, err := subjectFromContext(ctx)` and call with `err = d.st.Delete(ctx, a.ID, subj)`.

`set_visibility` closure (lines 505-512): replace extraction with `subj, err := subjectFromContext(ctx)` and call with `err = d.st.SetVisibility(ctx, a.ID, subj, a.Shared)`.

- [ ] **Step 4: Migrate the write-gate tests**

In `internal/store/store_test.go`, migrate every `getWritable`/`FetchForUpdate`/`SetVisibility`/`Delete` call literal in `TestDeleteOwnerGate`, `TestUpdateOwnerGateAndSharedFlag`, `TestSetVisibilityOwnerGate`, `TestSetVisibilityTOCTOU`, and the `FetchForUpdate` test(s). Rule: `""` → `Anonymous()`, `"sub-X"` → `Authenticated("sub-X")`. Example:

```go
if err := s.Delete(ctx, m.ID, Authenticated("sub-A")); !errors.Is(err, ErrNotFound) {
	t.Errorf("non-owner delete: want ErrNotFound, got %v", err)
}
```

Handler tests (`updateMemory`/`delete_memory`/`set_visibility` via `authedContext`) need no literal change; confirm they pass.

- [ ] **Step 5: Run the gates**

Run: `gofmt -l internal/ && go vet ./... && golangci-lint run && go test ./internal/store/ ./internal/server/ -v`
Expected: clean; PASS. The TOCTOU contract (SetPayload on a deleted point errors) is unchanged.

- [ ] **Step 6: Commit**

`jj commit -m "refactor(store): thread Subject through write gates (getWritable/Delete/SetVisibility/FetchForUpdate) (engram-6tl.5)"`

---

### Task 6: Convert `OwnedOrAbsent` + the store/discovery stamping handlers

**Files:**

- Modify: `internal/store/store.go:388-400` (`OwnedOrAbsent`)
- Modify: `internal/server/tools.go` (`storeMemory`, `storeDiscovery`)
- Test: `internal/store/store_test.go` (`TestOwnedOrAbsent`), `internal/server/tools_test.go` (store/discovery handler tests)

- [ ] **Step 1: Convert `OwnedOrAbsent`**

Replace `internal/store/store.go` lines 388-400:

```go
func (s *Store) OwnedOrAbsent(ctx context.Context, id string, subj Subject) error {
	m, err := s.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	switch sj := subj.(type) {
	case authenticated:
		if m.Owner != sj.sub {
			return fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return nil
	case anonymous:
		if m.Owner != "" {
			return fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
}
```

- [ ] **Step 2: Convert `storeMemory` (stamp via `Owner()`)**

In `internal/server/tools.go`, `storeMemory` (lines 243-268): replace `owner, err := ownerFromContext(ctx)` with `subj, err := subjectFromContext(ctx)`, and the struct field `Owner: owner,` with `Owner: subj.Owner(),`. `Actor: actorFromContext(ctx),` is unchanged.

- [ ] **Step 3: Convert `storeDiscovery` (gate + stamp)**

In `internal/server/tools.go`, `storeDiscovery` (lines 270-310): replace `sub, err := ownerFromContext(ctx)` with `subj, err := subjectFromContext(ctx)`; the gate `d.st.OwnedOrAbsent(ctx, a.ID, sub)` with `d.st.OwnedOrAbsent(ctx, a.ID, subj)`; the struct field `Owner: sub,` with `Owner: subj.Owner(),`.

- [ ] **Step 4: Migrate `TestOwnedOrAbsent`**

In `internal/store/store_test.go`, `TestOwnedOrAbsent` calls `s.OwnedOrAbsent(ctx, id, "sub-A")` / `"sub-B"`. Apply: `"sub-A"` → `Authenticated("sub-A")`, `"sub-B"` → `Authenticated("sub-B")`. Example:

```go
if err := s.OwnedOrAbsent(ctx, id, Authenticated("sub-B")); !errors.Is(err, ErrNotFound) {
	t.Errorf("cross-owner overwrite: want ErrNotFound, got %v", err)
}
```

Handler tests for `store_memory` / `store_discovery` drive through context and need no literal change; confirm they pass.

- [ ] **Step 5: Run the gates**

Run: `gofmt -l internal/ && go vet ./... && golangci-lint run && go test ./internal/store/ ./internal/server/ -v`
Expected: clean; PASS.

- [ ] **Step 6: Commit**

`jj commit -m "refactor(store): OwnedOrAbsent takes Subject; stamp Owner via subj.Owner() (engram-6tl.5)"`

---

### Task 7: Convert `DeleteAll` (filter-based, default returns error)

**Files:**

- Modify: `internal/store/store.go:471-481` (`DeleteAll`)
- Modify: `internal/server/tools.go` (`delete_all` handler)
- Test: `internal/store/store_test.go` (`TestDeleteAllOwnerScoped`)

- [ ] **Step 1: Convert `DeleteAll`**

Replace `internal/store/store.go` lines 471-481. The default arm returns an error and deletes nothing (a bulk delete must not proceed on an unknown subject):

```go
// DeleteAll removes the subject's OWN records in scope (never another owner's,
// and never another owner's shared records). A nil/unknown Subject is rejected
// without deleting anything — fail closed.
func (s *Store) DeleteAll(ctx context.Context, scope string, subj Subject) error {
	var owner string
	switch sj := subj.(type) {
	case authenticated:
		owner = sj.sub
	case anonymous:
		owner = ""
	default:
		return fmt.Errorf("%w: nil subject", ErrNotFound)
	}
	filter := &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		qdrant.NewMatch("owner", owner),
	}}
	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelectorFilter(filter),
	})
	return err
}
```

- [ ] **Step 2: Convert the `delete_all` handler**

In `internal/server/tools.go`, the `delete_all` closure (lines 483-490): replace extraction with `subj, err := subjectFromContext(ctx)` and call with `err = d.st.DeleteAll(ctx, a.Scope, subj)`.

- [ ] **Step 3: Migrate `TestDeleteAllOwnerScoped`**

In `internal/store/store_test.go`, `TestDeleteAllOwnerScoped` calls `s.DeleteAll(ctx, scope, "sub-A")` / `""`. Apply: `"sub-A"` → `Authenticated("sub-A")`, `""` → `Anonymous()`. Example:

```go
if err := s.DeleteAll(ctx, scope, Authenticated("sub-A")); err != nil {
	t.Fatalf("DeleteAll: %v", err)
}
```

- [ ] **Step 4: Run the gates**

Run: `gofmt -l internal/ && go vet ./... && golangci-lint run && go test ./internal/store/ ./internal/server/ -run 'DeleteAll|delete_all' -v`
Expected: clean; PASS.

- [ ] **Step 5: Commit**

`jj commit -m "refactor(store): DeleteAll takes Subject with fail-closed default (engram-6tl.5)"`

---

### Task 8: Retire `ownerFromContext`; convert `identityForLog`

At this point the only remaining caller of `ownerFromContext` is `identityForLog`.

**Files:**

- Modify: `internal/server/instrument.go:77-81` (`identityForLog`)
- Modify: `internal/server/tools.go` (delete `ownerFromContext`, lines 322-339)
- Test: `internal/server/tools_test.go` (delete `TestOwnerFromContextNoToken`)

- [ ] **Step 1: Convert `identityForLog` (display-only, nil-safe)**

Replace `internal/server/instrument.go` lines 73-81:

```go
// identityForLog extracts the verified actor (human-readable) and owner (sub)
// from context for log attribution. Both are "" when auth is disabled. The owner
// is read via subjectFromContext for DISPLAY ONLY — a nil/error subject degrades
// to "" rather than failing the log; this is never an enforcement decision.
func identityForLog(ctx context.Context) (actor, owner string) {
	actor = actorFromContext(ctx)
	if subj, err := subjectFromContext(ctx); err == nil && subj != nil {
		owner = subj.Owner()
	}
	return actor, owner
}
```

- [ ] **Step 2: Delete `ownerFromContext`**

In `internal/server/tools.go`, delete the entire `ownerFromContext` function (lines 322-339, the doc comment through the closing brace).

- [ ] **Step 3: Delete the obsolete test**

In `internal/server/tools_test.go`, delete `TestOwnerFromContextNoToken` (its coverage is replaced by `TestSubjectFromContextNoToken` from Task 2).

- [ ] **Step 4: Run the gates**

Run: `gofmt -l internal/ && go vet ./... && golangci-lint run && go test ./...`
Expected: clean; PASS. `golangci-lint`'s `unused` confirms `ownerFromContext` has no remaining references (build would fail otherwise).

- [ ] **Step 5: Commit**

`jj commit -m "refactor(server): retire ownerFromContext; identityForLog uses Subject (engram-6tl.5)"`

---

### Task 9: Add fail-closed `default`-deny tests (the new guarantee)

These directly assert the property the whole refactor buys: a `nil` Subject denies on every path. Previously unrepresentable (the param was a string).

**Files:**

- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go` (integration test — uses the testcontainers `testStore` like its neighbors). It passes a `nil` Subject to each converted method:

```go
// TestNilSubjectFailsClosed pins the core guarantee of the typed-Subject
// refactor: a nil Subject (what a discarded subjectFromContext error yields)
// denies on every authz path — empty reads, ErrNotFound id-gates, and a rejected
// bulk delete — rather than silently resolving to the anonymous bucket.
func TestNilSubjectFailsClosed(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:nil-subject"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	// Seed an ownerless (anonymous-bucket) record and an owned record.
	anon := Memory{ID: "a0a0a0a0-0000-0000-0000-000000000001", Content: "anon", Scope: scope, Owner: "", CreatedAt: time.Now().UTC()}
	owned := Memory{ID: "a0a0a0a0-0000-0000-0000-000000000002", Content: "owned", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
	for _, m := range []Memory{anon, owned} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}

	var nilSubj Subject // zero value == nil: the discarded-error case

	// Reads return nothing.
	if hits, err := s.Search(ctx, scope, nilSubj, []float32{0.1, 0.2, 0.3}, 10); err != nil || len(hits) != 0 {
		t.Errorf("Search(nil): want 0 hits nil err, got %d hits, %v", len(hits), err)
	}
	if mems, err := s.List(ctx, scope, nilSubj, 20); err != nil || len(mems) != 0 {
		t.Errorf("List(nil): want 0 mems nil err, got %d, %v", len(mems), err)
	}
	if hits, err := s.SearchDiscovery(ctx, scope, "", nilSubj, []float32{0.1, 0.2, 0.3}, 10); err != nil || len(hits) != 0 {
		t.Errorf("SearchDiscovery(nil): want 0 hits nil err, got %d, %v", len(hits), err)
	}

	// Id-gates return ErrNotFound (even for the ownerless record).
	if _, err := s.GetReadable(ctx, anon.ID, nilSubj); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetReadable(nil): want ErrNotFound, got %v", err)
	}
	if _, err := s.FetchForUpdate(ctx, anon.ID, nilSubj); !errors.Is(err, ErrNotFound) {
		t.Errorf("FetchForUpdate(nil): want ErrNotFound, got %v", err)
	}
	if err := s.Delete(ctx, anon.ID, nilSubj); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete(nil): want ErrNotFound, got %v", err)
	}
	if err := s.SetVisibility(ctx, anon.ID, nilSubj, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("SetVisibility(nil): want ErrNotFound, got %v", err)
	}
	if err := s.OwnedOrAbsent(ctx, anon.ID, nilSubj); !errors.Is(err, ErrNotFound) {
		t.Errorf("OwnedOrAbsent(nil) on existing id: want ErrNotFound, got %v", err)
	}

	// Bulk delete is rejected and removes nothing.
	if err := s.DeleteAll(ctx, scope, nilSubj); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteAll(nil): want ErrNotFound, got %v", err)
	}
	if _, err := s.Get(ctx, anon.ID); err != nil {
		t.Errorf("DeleteAll(nil) must not delete: record gone, %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test ./internal/store/ -run 'TestNilSubjectFailsClosed' -v`
Expected: PASS (the `default` arms added in Tasks 3-7 already implement this; this task locks it under test).

> Note: this is a regression-locking test for behavior built in prior tasks, so it passes on first run — there is no red phase. That is the documented TDD exception for codifying an existing invariant.

- [ ] **Step 3: Run the full gate**

Run: `task && gofmt -l .`
Expected: `task` (lint + test) exits 0; `gofmt -l` empty.

- [ ] **Step 4: Commit**

`jj commit -m "test(store): nil-Subject fail-closed guarantee across all authz paths (engram-6tl.5)"`

---

## Final verification (after Task 9)

- [ ] Run `task` — lint + full test suite green.
- [ ] Run `gofmt -l .` — empty.
- [ ] Run `task license:check` — all files carry the SPDX header (new `subject.go` / `subject_test.go` included).
- [ ] Confirm `rg -n 'ownerFromContext' internal/` returns nothing (fully retired).
- [ ] Confirm `rg -n '\bsub string\b' internal/store/store.go` returns nothing for the converted methods (only `MigrateSetOwner(owner string)` — an admin string — remains).
- [ ] Confirm no externally observable behavior change: the existing isolation suites (`TestSearchListOwnerIsolation`, `TestSearchDiscoveryOwnerIsolation`, `TestGetReadableOwnerGate`, `TestDeleteOwnerGate`, `TestUpdateOwnerGateAndSharedFlag`, `TestSetVisibilityOwnerGate`, `TestSetVisibilityTOCTOU`, `TestDeleteAllOwnerScoped`, `TestOwnedOrAbsent`, `TestAnonReadIsolationHandlers`, `TestAuthedCrossActorSharedReadHandlers`) all pass unchanged in intent.

## Out of scope

The sharing model (kyz), the `owner` Qdrant payload representation, the `ownerlessFilter` / `migrate-set-owner` pre-isolation path, and any MCP tool signature. None are touched.
<!-- adr-capture: sha256=cc859809d80343ac; session=cli; ts=2026-06-08T15:32:51Z; adrs=engram-12c -->
