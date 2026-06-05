<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Discovery Memory Type Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a citation-backed, aging-aware `discovery` memory type — a separate
on-demand pool of agent-earned codebase understanding — plus dedicated MCP tools
and a `discovering` capture skill.

**Architecture:** Additive change to the single Qdrant-backed `Memory` record
(`discovery` becomes a 5th `category` with optional `Kind` / `Citations` /
`Summary` fields). A new `Store.SearchDiscovery` builds a compound exact-match
filter (`category=discovery` [+`scope`] [+`kind`]) from the existing `NewMatch`
primitive — no prefix/wildcard machinery. Two dedicated MCP tools
(`store_discovery`, `search_discovery`) wrap it with pure validation helpers, and
a new plugin skill drives capture. The curated four categories and their tools
are untouched.

**Tech Stack:** Go 1.x, `github.com/qdrant/go-client` (gRPC, payload via
`NewValueMap`), `github.com/modelcontextprotocol/go-sdk/mcp`, jj VCS, `task` runner.

**Spec:** `docs/superpowers/specs/2026-06-05-discovery-memory-type-design.md`
**Design bead:** engram-4v1

---

## File Structure

| File | Responsibility | Action |
|------|----------------|--------|
| `internal/store/store.go` | `Citation` type, discovery fields on `Memory`, `payload`/`fromPayload` nesting, `SearchDiscovery` | Modify |
| `internal/store/store_test.go` | discovery round-trip + search-isolation integration tests | Modify |
| `internal/server/tools.go` | `store_discovery`/`search_discovery` args, validation helpers, handlers, registration | Modify |
| `internal/server/tools_test.go` | unit tests for the pure validation helpers (no Qdrant) | Create |
| `skill/engram/skills/discovering/SKILL.md` | the capture-side skill | Create |
| `README.md` | tool table: list the two discovery tools | Modify |
| `CLAUDE.md` | Memory-contract section: name the discovery tools + type | Modify |

**Test posture (important):** the store tests are **integration tests** that
`t.Skip()` unless `MEM_QDRANT_TEST_ADDR` is set (see existing `testStore`); CI's
`go test ./...` skips them. The validation logic therefore lives in **pure
functions** unit-tested in `tools_test.go`, which run everywhere. Do not refactor
`deps.st` into an interface — keep the integration/unit split the repo already uses.

---

### Task 1: Store — `Citation` type, discovery fields, payload round-trip

**Files:**

- Modify: `internal/store/store.go` (struct ~17-32, `payload` ~62-80, `fromPayload` ~82-124)
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go`:

```go
func TestDiscoveryRoundtrip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	m := Memory{
		ID:       "44444444-4444-4444-4444-444444444444",
		Content:  "retry logic lives in client.go, keyed off ctx deadline",
		Scope:    "discovery:repo:github.com/seanb4t/engram",
		Repo:     "github.com/seanb4t/engram",
		Source:   "agent-inferred",
		Category: "discovery",
		Kind:     "fact",
		Summary:  "retry = ctx deadline",
		Citations: []Citation{
			{Kind: "file", Ref: "internal/client.go", Locator: "200-240", Pin: "sha256:abc", Excerpt: "for ... { select { <-ctx.Done() } }"},
			{Kind: "repo", Ref: "github.com/qdrant/go-client", Pin: "@v1.18.2"},
		},
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := s.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Kind != "fact" || got.Summary != "retry = ctx deadline" {
		t.Errorf("kind/summary mismatch: %+v", got)
	}
	if len(got.Citations) != 2 {
		t.Fatalf("citations: got %d want 2: %+v", len(got.Citations), got.Citations)
	}
	c0 := got.Citations[0]
	if c0.Kind != "file" || c0.Ref != "internal/client.go" || c0.Locator != "200-240" || c0.Pin != "sha256:abc" || c0.Excerpt == "" {
		t.Errorf("citation[0] mismatch: %+v", c0)
	}
	if got.Citations[1].Ref != "github.com/qdrant/go-client" || got.Citations[1].Pin != "@v1.18.2" {
		t.Errorf("citation[1] mismatch: %+v", got.Citations[1])
	}
	_ = s.Delete(ctx, m.ID)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `MEM_QDRANT_TEST_ADDR=localhost:6334 go test ./internal/store/ -run TestDiscoveryRoundtrip -v`
Expected: compile error — `Memory` has no field `Kind`/`Summary`/`Citations`, `Citation` undefined.
(If `MEM_QDRANT_TEST_ADDR` is unset the test skips — start a local Qdrant on
gRPC 6334 to run it, e.g. `docker run -p 6334:6334 qdrant/qdrant:v1.18.2`.)

- [ ] **Step 3: Add the `Citation` type and discovery fields**

In `internal/store/store.go`, extend the `Memory` struct (after `CreatedAt`) and
add the `Citation` type below it:

```go
type Memory struct {
	ID        string   `json:"id"`
	Content   string   `json:"content"`
	Scope     string   `json:"scope"`
	Repo      string   `json:"repo"`
	Workspace string   `json:"workspace"`
	Worktree  string   `json:"worktree_path"`
	BaseDir   string   `json:"base_dir"`
	Source    string   `json:"source"`
	Category  string   `json:"category"`
	Tags      []string `json:"tags"`
	Actor     string   `json:"actor"`
	CreatedAt time.Time `json:"created_at"`

	// Discovery-only (zero-valued for the curated four categories).
	Kind      string     `json:"kind,omitempty"`      // "map" | "fact"
	Citations []Citation `json:"citations,omitempty"` // >= 1 for discoveries
	Summary   string     `json:"summary,omitempty"`
}

// Citation anchors a discovery to a source so it can be verified and aged.
type Citation struct {
	Kind    string `json:"kind"`              // file | commit | url | repo
	Ref     string `json:"ref"`               // path / repo URL / doc URL
	Locator string `json:"locator,omitempty"` // e.g. "200-240" line range
	Pin     string `json:"pin,omitempty"`     // aging anchor captured at store time
	Excerpt string `json:"excerpt,omitempty"` // cached substance
}
```

- [ ] **Step 4: Write the payload write path (guarded)**

In `payload(m Memory)`, before the final `return`, add the discovery keys only
for discovery records so curated payloads are byte-for-byte unchanged:

```go
func payload(m Memory) map[string]any {
	tags := make([]any, len(m.Tags))
	for i, t := range m.Tags {
		tags[i] = t
	}
	p := map[string]any{
		"content":       m.Content,
		"scope":         m.Scope,
		"repo":          m.Repo,
		"workspace":     m.Workspace,
		"worktree_path": m.Worktree,
		"base_dir":      m.BaseDir,
		"source":        m.Source,
		"category":      m.Category,
		"tags":          tags,
		"actor":         m.Actor,
		"created_at":    m.CreatedAt.Format(time.RFC3339),
	}
	if m.Category == "discovery" {
		p["kind"] = m.Kind
		p["summary"] = m.Summary
		cites := make([]any, len(m.Citations))
		for i, c := range m.Citations {
			cites[i] = map[string]any{
				"kind": c.Kind, "ref": c.Ref, "locator": c.Locator,
				"pin": c.Pin, "excerpt": c.Excerpt,
			}
		}
		p["citations"] = cites
	}
	return p
}
```

- [ ] **Step 5: Write the `fromPayload` read path**

In `fromPayload`, after the `created_at` block and before `return m`, add the
nested read (accessor chain confirmed against qdrant go-client):

```go
	if v, ok := p["kind"]; ok {
		m.Kind = v.GetStringValue()
	}
	if v, ok := p["summary"]; ok {
		m.Summary = v.GetStringValue()
	}
	if v, ok := p["citations"]; ok {
		if lv := v.GetListValue(); lv != nil {
			for _, item := range lv.GetValues() {
				f := item.GetStructValue().GetFields()
				m.Citations = append(m.Citations, Citation{
					Kind:    f["kind"].GetStringValue(),
					Ref:     f["ref"].GetStringValue(),
					Locator: f["locator"].GetStringValue(),
					Pin:     f["pin"].GetStringValue(),
					Excerpt: f["excerpt"].GetStringValue(),
				})
			}
		}
	}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `MEM_QDRANT_TEST_ADDR=localhost:6334 go test ./internal/store/ -run TestDiscoveryRoundtrip -v`
Expected: PASS. Also run `go build ./...` (Expected: success) and
`go test ./...` (Expected: store tests SKIP without the env var, everything else PASS).

- [ ] **Step 7: Commit**

```bash
jj commit -m "feat(store): add discovery fields and Citation to Memory"
```

---

### Task 2: Store — `SearchDiscovery` compound filter

**Files:**

- Modify: `internal/store/store.go` (after `Search`, ~157)
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go`:

```go
func TestSearchDiscoveryFilters(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scopeA := "discovery:repo:A"
	scopeB := "discovery:repo:B"
	curated := "repo:A"
	defer func() { _ = s.DeleteAll(ctx, scopeA); _ = s.DeleteAll(ctx, scopeB); _ = s.DeleteAll(ctx, curated) }()

	mk := func(id, scope, cat, kind string, vec []float32) {
		m := Memory{ID: id, Content: "x", Scope: scope, Category: cat, Kind: kind,
			Source: "agent-inferred", CreatedAt: time.Now().UTC()}
		if cat == "discovery" {
			m.Citations = []Citation{{Kind: "file", Ref: "f.go"}}
		}
		if err := s.Upsert(ctx, m, vec); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	mk("55555555-5555-5555-5555-555555555555", scopeA, "discovery", "map", []float32{0.9, 0.1, 0.0})
	mk("66666666-6666-6666-6666-666666666666", scopeA, "discovery", "fact", []float32{0.8, 0.2, 0.0})
	mk("77777777-7777-7777-7777-777777777777", scopeB, "discovery", "map", []float32{0.7, 0.3, 0.0})
	mk("88888888-8888-8888-8888-888888888888", curated, "preference", "", []float32{0.9, 0.1, 0.0})

	q := []float32{0.9, 0.1, 0.0}

	// scope-constrained: only scopeA discoveries, not curated, not scopeB.
	hits, err := s.SearchDiscovery(ctx, scopeA, "", q, 10)
	if err != nil {
		t.Fatalf("search scopeA: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("scopeA: got %d want 2", len(hits))
	}
	for _, h := range hits {
		if h.Category != "discovery" || h.Scope != scopeA {
			t.Errorf("leaked non-discovery/other-scope: %+v", h)
		}
	}

	// kind filter.
	maps, err := s.SearchDiscovery(ctx, scopeA, "map", q, 10)
	if err != nil {
		t.Fatalf("search kind=map: %v", err)
	}
	if len(maps) != 1 || maps[0].Kind != "map" {
		t.Fatalf("kind=map: got %d %+v", len(maps), maps)
	}

	// cross-spine: empty scope spans scopeA + scopeB discoveries (3), still no curated.
	all, err := s.SearchDiscovery(ctx, "", "", q, 10)
	if err != nil {
		t.Fatalf("search cross-spine: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("cross-spine: got %d want 3", len(all))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `MEM_QDRANT_TEST_ADDR=localhost:6334 go test ./internal/store/ -run TestSearchDiscoveryFilters -v`
Expected: compile error — `s.SearchDiscovery` undefined.

- [ ] **Step 3: Implement `SearchDiscovery`**

In `internal/store/store.go`, after the `Search` method add:

```go
// SearchDiscovery runs a top-k vector search constrained to discovery records.
// Empty scope spans all discovery scopes (the cross_spine case); empty kind
// matches both map and fact. Builds a compound exact-match filter from the same
// NewMatch primitive scopeFilter uses — no prefix matching.
func (s *Store) SearchDiscovery(ctx context.Context, scope, kind string, vec []float32, k uint64) ([]Memory, error) {
	must := []*qdrant.Condition{qdrant.NewMatch("category", "discovery")}
	if scope != "" {
		must = append(must, qdrant.NewMatch("scope", scope))
	}
	if kind != "" {
		must = append(must, qdrant.NewMatch("kind", kind))
	}
	res, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collection, Query: qdrant.NewQuery(vec...),
		Filter: &qdrant.Filter{Must: must}, Limit: qdrant.PtrOf(k),
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Memory, 0, len(res))
	for _, p := range res {
		out = append(out, fromPayload(p.Id.GetUuid(), p.Payload))
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `MEM_QDRANT_TEST_ADDR=localhost:6334 go test ./internal/store/ -run TestSearchDiscoveryFilters -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(store): add SearchDiscovery compound-filter query"
```

---

### Task 3: MCP `store_discovery` tool + validation

**Files:**

- Modify: `internal/server/tools.go` (args ~83-117, handlers ~119-160, `Register` ~176-223)
- Create: `internal/server/tools_test.go`

- [ ] **Step 1: Write the failing unit test**

Create `internal/server/tools_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import "testing"

func TestValidateStoreDiscovery(t *testing.T) {
	good := storeDiscoveryArgs{
		Content: "x", Kind: "map", Scope: "discovery:repo:X",
		Citations: []citationArg{{Kind: "file", Ref: "f.go"}},
	}
	if err := validateStoreDiscovery(good); err != nil {
		t.Errorf("valid args rejected: %v", err)
	}
	bad := []struct {
		name string
		a    storeDiscoveryArgs
	}{
		{"bad kind", storeDiscoveryArgs{Content: "x", Kind: "blob", Scope: "s", Citations: []citationArg{{Kind: "file", Ref: "f"}}}},
		{"empty kind", storeDiscoveryArgs{Content: "x", Kind: "", Scope: "s", Citations: []citationArg{{Kind: "file", Ref: "f"}}}},
		{"no citations", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: "s"}},
		{"empty content", storeDiscoveryArgs{Content: "", Kind: "fact", Scope: "s", Citations: []citationArg{{Kind: "file", Ref: "f"}}}},
		{"empty scope", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: "", Citations: []citationArg{{Kind: "file", Ref: "f"}}}},
	}
	for _, tc := range bad {
		if err := validateStoreDiscovery(tc.a); err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestValidateStoreDiscovery -v`
Expected: compile error — `storeDiscoveryArgs`, `citationArg`, `validateStoreDiscovery` undefined.

- [ ] **Step 3: Add args types + validation helper**

In `internal/server/tools.go`, after the existing arg structs (~117) add:

```go
type citationArg struct {
	Kind    string `json:"kind" jsonschema:"file|commit|url|repo"`
	Ref     string `json:"ref" jsonschema:"path, repo URL, or doc URL"`
	Locator string `json:"locator,omitempty" jsonschema:"e.g. 200-240 line range"`
	Pin     string `json:"pin,omitempty" jsonschema:"commit SHA, content-hash, @rev, or fetched-at"`
	Excerpt string `json:"excerpt,omitempty" jsonschema:"cached substance (<= ~50 lines)"`
}

type storeDiscoveryArgs struct {
	Content   string        `json:"content" jsonschema:"the understanding to cache (embedded + searched)"`
	Kind      string        `json:"kind" jsonschema:"map (orientation) or fact (pinned checkable claim)"`
	Citations []citationArg `json:"citations" jsonschema:">= 1 source anchor"`
	Scope     string        `json:"scope" jsonschema:"discovery:repo:<repo>"`
	Tags      []string      `json:"tags,omitempty"`
	Summary   string        `json:"summary,omitempty"`
	ID        string        `json:"id,omitempty" jsonschema:"omit to create; supply to replace in place"`
}

type searchDiscoveryArgs struct {
	Query      string `json:"query"`
	Scope      string `json:"scope,omitempty" jsonschema:"required unless cross_spine"`
	Kind       string `json:"kind,omitempty" jsonschema:"map|fact filter"`
	K          uint64 `json:"k,omitempty"`
	CrossSpine bool   `json:"cross_spine,omitempty" jsonschema:"span all discovery scopes (ignores scope)"`
}

func validateStoreDiscovery(a storeDiscoveryArgs) error {
	if a.Content == "" {
		return fmt.Errorf("content is required")
	}
	if a.Kind != "map" && a.Kind != "fact" {
		return fmt.Errorf("kind must be \"map\" or \"fact\", got %q", a.Kind)
	}
	if len(a.Citations) == 0 {
		return fmt.Errorf("at least one citation is required")
	}
	if a.Scope == "" {
		return fmt.Errorf("scope is required")
	}
	return nil
}
```

- [ ] **Step 4: Add the `storeDiscovery` handler**

After `storeMemory` (~139) add:

```go
func (d *deps) storeDiscovery(ctx context.Context, a storeDiscoveryArgs) (string, error) {
	if err := validateStoreDiscovery(a); err != nil {
		return "", err
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
		CreatedAt: time.Now().UTC(),
	}
	return m.ID, d.st.Upsert(ctx, m, vec)
}
```

- [ ] **Step 5: Register the tool**

In `Register`, after the `store_memory` registration (~183) add:

```go
	mcp.AddTool(s, &mcp.Tool{Name: "store_discovery", Description: "Cache agent-earned codebase understanding with citations. kind=map|fact; >=1 citation; scope discovery:repo:<repo>."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a storeDiscoveryArgs) (*mcp.CallToolResult, any, error) {
			id, err := d.storeDiscovery(ctx, a)
			return textResult(fmt.Sprintf("stored %s", id)), map[string]string{"id": id}, err
		})
```

- [ ] **Step 6: Run test + build**

Run: `go test ./internal/server/ -run TestValidateStoreDiscovery -v`
Expected: PASS.
Run: `go build ./...`
Expected: success.

- [ ] **Step 7: Commit**

```bash
jj commit -m "feat(server): add store_discovery MCP tool with validation"
```

---

### Task 4: MCP `search_discovery` tool + scope rule

**Files:**

- Modify: `internal/server/tools.go` (handler after `searchMemory` ~160, `Register` after the `store_discovery` block)
- Test: `internal/server/tools_test.go`

- [ ] **Step 1: Write the failing unit test**

Append to `internal/server/tools_test.go`:

```go
func TestEffectiveDiscoveryScope(t *testing.T) {
	// cross_spine=false requires a scope.
	if _, err := effectiveDiscoveryScope(searchDiscoveryArgs{CrossSpine: false, Scope: ""}); err == nil {
		t.Error("expected error: scope required when cross_spine=false")
	}
	got, err := effectiveDiscoveryScope(searchDiscoveryArgs{CrossSpine: false, Scope: "discovery:repo:X"})
	if err != nil || got != "discovery:repo:X" {
		t.Errorf("scoped: got %q err %v", got, err)
	}
	// cross_spine=true spans all scopes (effective scope empty), scope ignored.
	got, err = effectiveDiscoveryScope(searchDiscoveryArgs{CrossSpine: true, Scope: "discovery:repo:X"})
	if err != nil || got != "" {
		t.Errorf("cross_spine: got %q err %v", got, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestEffectiveDiscoveryScope -v`
Expected: compile error — `effectiveDiscoveryScope` undefined.

- [ ] **Step 3: Add the scope helper + handler**

In `internal/server/tools.go`, after `searchMemory` (~160) add:

```go
// effectiveDiscoveryScope resolves the scope filter for a discovery search:
// "" means span all discovery scopes (cross_spine). cross_spine ignores any
// supplied scope; otherwise a scope is mandatory.
func effectiveDiscoveryScope(a searchDiscoveryArgs) (string, error) {
	if a.CrossSpine {
		return "", nil
	}
	if a.Scope == "" {
		return "", fmt.Errorf("scope is required unless cross_spine is true")
	}
	return a.Scope, nil
}

func (d *deps) searchDiscovery(ctx context.Context, a searchDiscoveryArgs) ([]store.Memory, error) {
	scope, err := effectiveDiscoveryScope(a)
	if err != nil {
		return nil, err
	}
	if a.CrossSpine && a.Scope != "" {
		log.Printf("search_discovery: cross_spine=true ignores scope %q", a.Scope)
	}
	if a.K == 0 {
		a.K = 8
	}
	vec, err := d.em.Embed(ctx, a.Query)
	if err != nil {
		return nil, err
	}
	return d.st.SearchDiscovery(ctx, scope, a.Kind, vec, a.K)
}
```

- [ ] **Step 4: Register the tool**

In `Register`, after the `store_discovery` block add:

```go
	mcp.AddTool(s, &mcp.Tool{Name: "search_discovery", Description: "Semantic search over the discovery pool. scope required unless cross_spine=true; optional kind=map|fact. Results carry citations + created_at (aging signals)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a searchDiscoveryArgs) (*mcp.CallToolResult, any, error) {
			hits, err := d.searchDiscovery(ctx, a)
			return textResult(fmt.Sprintf("%d hits", len(hits))), map[string]any{"discoveries": hits}, err
		})
```

- [ ] **Step 5: Run tests + build + full suite**

Run: `go test ./internal/server/ -v`
Expected: PASS (both validation tests).
Run: `go build ./... && go test ./...`
Expected: success; store integration tests SKIP without `MEM_QDRANT_TEST_ADDR`.

- [ ] **Step 6: Commit**

```bash
jj commit -m "feat(server): add search_discovery MCP tool with scope rule"
```

---

### Task 5: The `discovering` skill

**Files:**

- Create: `skill/engram/skills/discovering/SKILL.md`

- [ ] **Step 1: Write the skill file**

Create `skill/engram/skills/discovering/SKILL.md` with frontmatter on line 1
(no leading SPDX comment — `.licenserc.yaml` exempts `skill/**/SKILL.md`):

```markdown
---
name: discovering
description: Use when mapping or investigating a repository/codebase to cache agent-earned understanding as citation-backed discoveries via engram's store_discovery tool. Trigger on "map this repo", "help me understand this codebase", onboarding to unfamiliar third-party code, or before substantial work in an unmapped area. Pairs with search_discovery for on-demand recall.
---

# Discovering

A **discovery** caches understanding you earned by reading code — the expensive
re-derivation you would otherwise repeat next session. Its value is the work it
saves, so the bar is simple: **store a discovery only when re-deriving it would
cost meaningful tokens.** Discoveries are separate from the curated four memory
types (decision / preference / convention / gotcha) and never load at session
start — they are pulled on demand.

## When to capture

- Tracing how an unfamiliar subsystem or third-party dependency works.
- Orientation worth keeping: where things live, how a flow connects.
- A behavioral fact that is costly to re-derive and risky to get wrong.

Do **not** capture: anything trivially re-read in one file, transient state,
secrets, or restating the curated four. Capture is explicit — never
auto-extract.

## kind: map vs fact

- **map** — orientation: structure, where things live, how flows connect.
  Broader; commit-SHA pins are enough.
- **fact** — a pinned, checkable behavioral claim. Tighter; pin a content-hash
  of the cited region so a later reader can detect that *those exact lines*
  changed.

## Citations are mandatory

Every discovery carries **>= 1 citation** — that is what makes it trustworthy
and ageable. For each citation capture:

- `kind`: file | commit | url | repo
- `ref`: path / repo URL / doc URL
- `locator`: line range for files
- `pin`: the aging anchor captured now — content-hash (fact files), commit SHA
  (map files), `@rev` (repo), or fetched-at (url)
- `excerpt`: the cached substance — keep the few lines worth not re-fetching.
  Soft cap **~50 lines**; exceed only with explicit reason.

## Workflow

1. **search-before-store.** Run `search_discovery` for the area first (a
   natural-language description — it is semantic). If a near-duplicate exists,
   call `store_discovery` with that record's `id` to replace it rather than
   adding a duplicate.
2. Explore breadth-first; for each meaningful unit decide map vs fact.
3. Capture citations (pins + excerpts) as you read.
4. `store_discovery(content, kind, citations[], scope="discovery:repo:<repo>", summary?, tags?)`.

## Recall (the other half)

When entering mapped territory later, issue a targeted `search_discovery` scoped
to `discovery:repo:<repo>`. Pass `cross_spine=true` only when you deliberately
want to span every discovery scope. The result carries each citation's `pin` and
the record's `created_at` — render trust from those (age, pinned commit, whether
the cited code has since moved); the server stores signals, never a verdict.
```

- [ ] **Step 2: Verify frontmatter + lint**

Run: `head -1 skill/engram/skills/discovering/SKILL.md`
Expected: `---` (frontmatter on line 1, no SPDX comment).
Run: `rumdl check skill/engram/skills/discovering/SKILL.md`
Expected: no issues (run `rumdl fmt` on it if MD058/MD040/MD031/MD032 appear;
do NOT use `rumdl fmt`'s MD004 autofix on prose).

- [ ] **Step 3: Commit**

```bash
jj commit -m "feat(skill): add discovering capture skill"
```

---

### Task 6: Documentation — README + CLAUDE.md

**Files:**

- Modify: `README.md` (tool table ~29-36)
- Modify: `CLAUDE.md` (Memory contract ~38-42)

- [ ] **Step 1: Add the discovery tools to the README table**

In `README.md`, in the tool table, after the `store_memory` row add:

```markdown
| `store_discovery(content, kind, citations[], scope, …)` | Cache citation-backed codebase understanding (kind=map\|fact) |
| `search_discovery(query, scope?, kind?, cross_spine?)` | On-demand semantic search over the discovery pool |
```

Then extend the record-fields paragraph below the table with a sentence:

```markdown
A **discovery** record (category `discovery`) additionally carries `kind`
(`map` | `fact`), `citations[]` (each `kind`/`ref`/`locator`/`pin`/`excerpt`),
and an optional `summary`; it lives in a `discovery:repo:<repo>` scope and is
recalled on demand, never at session start.
```

- [ ] **Step 2: Update the CLAUDE.md memory contract**

In `CLAUDE.md`, in the "Memory contract (stable)" section, after the existing
tool list add a sentence:

```markdown
Discovery tools: `store_discovery` / `search_discovery`. A discovery is a 5th
`category` carrying `kind` (`map`|`fact`), `citations` (with aging `pin`s), and
`summary`; it lives in a separate `discovery:repo:*` scope, is recalled on
demand (never at session start), and is captured via the `discovering` skill.
Design intent unchanged: explicit, citation-backed, no auto-extraction.
```

- [ ] **Step 3: Lint the docs**

Run: `rumdl check README.md CLAUDE.md`
Expected: no issues. (CLAUDE.md's beads block is already wrapped in
`rumdl-disable` comments; do not edit inside the BEADS markers.)

- [ ] **Step 4: Commit**

```bash
jj commit -m "docs: document discovery tools and memory type"
```

---

## Final verification

- [ ] **Full gate:** `task` (lint + test). Expected: exit 0. Store integration
  tests SKIP without `MEM_QDRANT_TEST_ADDR`; with a local Qdrant on 6334 they PASS.
- [ ] **License headers:** `task license:check`. Expected: pass (Go files carry
  SPDX; `tools_test.go` gets the header from Step; SKILL.md is exempt).
- [ ] **Manual smoke (optional, needs Qdrant + a running server):** call
  `store_discovery` then `search_discovery` and confirm the returned record
  carries `citations` with `pin` and a `created_at`.
<!-- adr-capture: sha256=c89eb1f23de701a2; session=cli; ts=2026-06-06T00:02:59Z; adrs=engram-2bv,engram-3l0,engram-0gy -->
