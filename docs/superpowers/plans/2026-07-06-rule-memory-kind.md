<!--
SPDX-License-Identifier: Apache-2.0
Copyright 2026 Sean Brandt
-->
# `rule` Memory Kind Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a normative, user-blessed, always-shared `rule` memory kind that is enumerable as a complete set and surfaced at session start as a one-line-per-rule index (progressive disclosure).

**Architecture:** `rule` is a 6th `category` value in the existing single Qdrant collection (no schema change, no migration), isolated by a `rule:repo:*` / `rule:project:*` scope prefix. Two dedicated MCP tools (`store_rule`, `list_rules`) mirror the discovery tool-pair precedent (ADR engram-0gy); by-id ops reuse `get_memory` / `update_memory` / `delete_memory`. The session-start hook fetches the rule set and injects a compact index. All new Go server code lives in a focused `internal/server/rules.go` file; the only store change is one additive `ListOptions.Ascending` field.

**Tech Stack:** Go 1.x (`internal/server`, `internal/store`), Qdrant go-client v1.18.3, modelcontextprotocol/go-sdk (`mcp.AddTool`), Python 3.11 uv-script hook, `task` runner.

---

## Prerequisites (READ FIRST)

**This plan depends on `engram-c0yl` (short_id handles) being merged to `main` first.** The spec's Sequencing section is explicit: the rule index and the by-id correction loop want `short_id` from day one. Concretely, this plan references two artifacts that **do not exist in `main` today** and are delivered by c0yl:

1. **`store.Memory.ShortID string` field** — c0yl adds `ShortID string \`json:"short_id,omitempty"\`` to the `Memory` struct (`internal/store/store.go`, per c0yl spec `docs/superpowers/specs/2026-07-06-short-id-handle-design.md:82`) and populates it on Upsert. `list_rules`'s compact shape surfaces it.
2. **Post-c0yl `set_visibility` handler calling `ResolvePointID`** — c0yl's plan (`docs/superpowers/plans/2026-07-06-short-id-handle.md`, Task 8) rewrites the `set_visibility` handler to resolve `a.ID` (UUID or short_id) to a canonical UUID `pid` via `d.st.ResolvePointID(ctx, a.ID)` before calling `d.st.SetVisibility`. Task 6 below inserts the rule guard into that post-c0yl handler.

**Verification gate before starting:** confirm both landed on `main`:

```bash
rg -n "ShortID string" internal/store/store.go        # expect a hit
rg -n "ResolvePointID" internal/server/tools.go        # expect a hit in the set_visibility handler
```

If either returns nothing, **stop** — c0yl has not merged; this plan is blocked. The bead epic for this plan carries a `bd dep` on `engram-c0yl` to enforce ordering.

Every other symbol referenced below (`store.List`, `ListOptions`, `resolveSummaryUpdate`, `FetchForUpdate`, `GetReadable`, `Upsert`, `OwnedOrAbsent`, `EmbedText`, `textResult`, `derive_scopes`) exists in `main` today at the cited locations.

---

## File Structure

| File | Responsibility | New/Modify |
|------|----------------|------------|
| `internal/store/store.go` | Add `ListOptions.Ascending`; honor it in the offset-mode `Scroll` OrderBy direction | Modify |
| `internal/store/store_test.go` | Ascending-order List test | Modify |
| `internal/server/rules.go` | All rule server logic: arg types, `validateStoreRule`, byte-cap constants, `storeRule` + `listRules` handlers, `ruleView` compact shape, rule-guard helpers | **Create** |
| `internal/server/rules_test.go` | `validateStoreRule` unit tests; store_rule/list_rules/set_visibility/update rule integration tests | **Create** |
| `internal/server/tools.go` | Register `store_rule` + `list_rules`; call rule guard inside `updateMemory` and the `set_visibility` handler | Modify |
| `skill/engram/hooks/session-start-memory-recall` | Emit the rules-index instruction block; read `ENGRAM_PROJECT` for the project scope | Modify |
| `skill/engram/hooks/tests/test_session_start_memory_recall.py` | Assert the rules-index instruction + project-scope behavior | Modify |
| `docs-site/src/content/docs/reference/tools.md` | Document `store_rule` / `list_rules` | Modify |
| `CLAUDE.md` | Add `store_rule` / `list_rules` to the memory-contract tool list | Modify |
| `skill/engram/skills/curating-memory/SKILL.md` | Document the user-blessed-only `store_rule` contract | Modify |

Every new Go/Markdown file needs the Apache-2.0 SPDX header (`task license:check` enforces it; `task license:add` applies it).

---

## Task 1: Store — additive `ListOptions.Ascending`

**Model:** sonnet

**Files:**

- Modify: `internal/store/store.go:614-630` (`ListOptions` struct), `internal/store/store.go:741-747` (offset-mode `Scroll`)
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go` (integration; needs Qdrant, follows the `testStore(t)` idiom used throughout the file):

```go
func TestListAscendingOrder(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "rule:repo:asc-order-test"
	subj := Authenticated("owner-asc")
	// Three records with distinct, increasing created_at.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mk := func(id string, mins int) {
		m := Memory{
			ID: id, Content: "c", Scope: scope, Category: "rule",
			Owner: "owner-asc", Visibility: "shared",
			Source: "user-said", Summary: "s", SummarySource: SummarySourceClient,
			CreatedAt: base.Add(time.Duration(mins) * time.Minute),
		}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	mk("c1000000-0000-0000-0000-000000000001", 0)
	mk("c1000000-0000-0000-0000-000000000002", 10)
	mk("c1000000-0000-0000-0000-000000000003", 20)
	t.Cleanup(func() { cleanupErr(t, "DeleteAll", s.DeleteAll(ctx, scope, subj)) })

	got, _, _, err := s.List(ctx, scope, subj, ListOptions{Ascending: true})
	if err != nil {
		t.Fatalf("List ascending: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d records, want 3", len(got))
	}
	if got[0].ID != "c1000000-0000-0000-0000-000000000001" ||
		got[2].ID != "c1000000-0000-0000-0000-000000000003" {
		t.Errorf("not ascending by created_at: first=%s last=%s", got[0].ID, got[2].ID)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestListAscendingOrder -v`
Expected: FAIL — `ListOptions` has no field `Ascending` (compile error), or (once the field is added but unused) order is descending so `got[0]` is the newest record.

- [ ] **Step 3: Add the field**

In `internal/store/store.go`, add to the `ListOptions` struct (after the `CursorMode bool` field at :629):

```go
	// Ascending flips the offset-mode created_at ordering from the default
	// descending (recency-first recall) to ascending (oldest-first). Honored
	// only on the offset/all path used by list_rules; cursor mode is unaffected
	// (rules do not paginate). Zero value preserves the existing desc behavior.
	Ascending bool
```

- [ ] **Step 4: Honor the field in the offset-mode Scroll**

In `internal/store/store.go`, replace the hardcoded `OrderBy` at :745 (inside the offset-mode `s.client.Scroll` call) with a computed direction:

```go
	dir := qdrant.Direction_Desc
	if opts.Ascending {
		dir = qdrant.Direction_Asc
	}
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         f,
		Limit:          qdrant.PtrOf(uint32(fetch)),
		OrderBy:        &qdrant.OrderBy{Key: "created_at", Direction: qdrant.PtrOf(dir)},
		WithPayload:    qdrant.NewWithPayload(true),
	})
```

(Insert the `dir` computation on the line immediately before the existing `pts, err := s.client.Scroll(...)` at :741 and edit that call's `OrderBy` line. Leave the cursor-mode path at :803-805 untouched — `list_rules` never uses it.)

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestListAscendingOrder -v`
Expected: PASS

- [ ] **Step 6: Guard against regressions in existing List tests**

Run: `go test ./internal/store/ -run 'TestList|TestSearchList|TestRecallWindow' -v`
Expected: PASS (default `Ascending:false` preserves desc order everywhere).

- [ ] **Step 7: Commit**

```bash
jj commit -m "feat(store): add ListOptions.Ascending for oldest-first listing (engram-3jo0)"
```

---

## Task 2: `validateStoreRule` + byte-cap constants (pure, unit-tested)

**Model:** haiku

**Files:**

- Create: `internal/server/rules.go`
- Create: `internal/server/rules_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/server/rules_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"strings"
	"testing"
)

func TestValidateStoreRule(t *testing.T) {
	good := storeRuleArgs{
		Content: "never push to main directly; open a PR",
		Scope:   "rule:repo:github.com/seanb4t/engram",
		Summary: "never push to main directly; PRs only",
	}
	if err := validateStoreRule(good); err != nil {
		t.Errorf("valid args rejected: %v", err)
	}
	if err := validateStoreRule(storeRuleArgs{
		Content: "x", Scope: "rule:project:selfhosted-cluster", Summary: "s",
	}); err != nil {
		t.Errorf("valid project-scope args rejected: %v", err)
	}

	bad := []struct {
		name string
		a    storeRuleArgs
	}{
		{"empty content", storeRuleArgs{Content: "", Scope: "rule:repo:x", Summary: "s"}},
		{"empty scope", storeRuleArgs{Content: "x", Scope: "", Summary: "s"}},
		{"non-rule scope", storeRuleArgs{Content: "x", Scope: "repo:x", Summary: "s"}},
		{"discovery scope", storeRuleArgs{Content: "x", Scope: "discovery:repo:x", Summary: "s"}},
		{"rule prefix no tier", storeRuleArgs{Content: "x", Scope: "rule:repo:", Summary: "s"}},
		{"rule bad tier", storeRuleArgs{Content: "x", Scope: "rule:widget:x", Summary: "s"}},
		{"empty summary", storeRuleArgs{Content: "x", Scope: "rule:repo:x", Summary: ""}},
		{"summary newline", storeRuleArgs{Content: "x", Scope: "rule:repo:x", Summary: "line1\nline2"}},
		{"summary carriage return", storeRuleArgs{Content: "x", Scope: "rule:repo:x", Summary: "line1\rline2"}},
		{"summary too long", storeRuleArgs{Content: "x", Scope: "rule:repo:x", Summary: strings.Repeat("a", maxRuleSummaryBytes+1)}},
		{"content too large", storeRuleArgs{Content: strings.Repeat("a", maxRuleContentBytes+1), Scope: "rule:repo:x", Summary: "s"}},
	}
	for _, tc := range bad {
		if err := validateStoreRule(tc.a); err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestValidateStoreRule -v`
Expected: FAIL — `storeRuleArgs`, `validateStoreRule`, `maxRuleSummaryBytes`, `maxRuleContentBytes` undefined (compile error).

- [ ] **Step 3: Write minimal implementation**

Create `internal/server/rules.go` (arg types + validation only for now; handlers arrive in Tasks 3-4):

```go
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"fmt"
	"strings"

	"github.com/seanb4t/engram/internal/store"
)

// Rule size bounds. content is generous full rule text; summary is the one-line
// index entry, capped so one rule ≈ one terminal line in the session-start index.
const (
	maxRuleContentBytes = 8 * 1024 // 8 KiB full rule text
	maxRuleSummaryBytes = 256      // one physical index line
)

type storeRuleArgs struct {
	Content string   `json:"content" jsonschema:"the full rule text (normative constraint agents must follow)"`
	Scope   string   `json:"scope" jsonschema:"rule scope: rule:repo:<repo> or rule:project:<project>"`
	Summary string   `json:"summary" jsonschema:"REQUIRED one-line index entry (single physical line, no newlines)"`
	Tags    []string `json:"tags,omitempty" jsonschema:"concern-area labels e.g. vcs, deploy, authz"`
	ID      string   `json:"id,omitempty" jsonschema:"omit to create; supply to replace in place"`
}

type listRulesArgs struct {
	Scopes []string `json:"scopes" jsonschema:"one or more rule:* scopes to fetch the complete rule set from"`
	Tags   []string `json:"tags,omitempty" jsonschema:"restrict to rules carrying ALL listed tags (AND)"`
	Full   bool     `json:"full,omitempty" jsonschema:"true adds full content; default returns the compact index shape"`
}

// validRuleScope reports whether s is a well-formed rule scope: rule:repo:<tail>
// or rule:project:<tail> with a non-empty tail.
func validRuleScope(s string) bool {
	for _, prefix := range []string{"rule:repo:", "rule:project:"} {
		if strings.HasPrefix(s, prefix) && len(s) > len(prefix) {
			return true
		}
	}
	return false
}

// validateStoreRule enforces the store_rule contract without touching Qdrant:
// rule scope prefix, required content within the byte cap, and a required
// single-line summary within the byte cap. Newlines in the summary are rejected
// (never normalized): the summary is the index line and munging user input hides
// the problem (explicit/correctable ethos).
func validateStoreRule(a storeRuleArgs) error {
	if a.Content == "" {
		return fmt.Errorf("content is required")
	}
	if len(a.Content) > maxRuleContentBytes {
		return fmt.Errorf("content too large: %d bytes (max %d)", len(a.Content), maxRuleContentBytes)
	}
	if a.Scope == "" {
		return fmt.Errorf("scope is required")
	}
	if !validRuleScope(a.Scope) {
		return fmt.Errorf("scope must be rule:repo:<repo> or rule:project:<project>, got %q", a.Scope)
	}
	if err := validateRuleSummary(a.Summary); err != nil {
		return err
	}
	return nil
}

// validateRuleSummary enforces the shared summary contract for rules: non-empty,
// single physical line, within the byte cap. Reused by the update_memory guard.
func validateRuleSummary(summary string) error {
	if summary == "" {
		return fmt.Errorf("summary is required for a rule (it is the one-line index entry)")
	}
	if strings.ContainsAny(summary, "\r\n") {
		return fmt.Errorf("rule summary must be a single line (no newlines); it is the index entry")
	}
	if len(summary) > maxRuleSummaryBytes {
		return fmt.Errorf("summary too long: %d bytes (max %d)", len(summary), maxRuleSummaryBytes)
	}
	return nil
}

// ensure the store import is used even before handlers land (removed once
// storeRule/listRules reference store types in later tasks).
var _ = store.SummarySourceClient
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestValidateStoreRule -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(server): validateStoreRule + rule arg types and byte caps (engram-3jo0)"
```

---

## Task 3: `store_rule` handler + registration

**Model:** sonnet

**Files:**

- Modify: `internal/server/rules.go`
- Modify: `internal/server/tools.go:857` (register alongside the discovery/set_visibility tools, before the final `return nil`)
- Test: `internal/server/rules_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/server/rules_test.go` (integration; uses the same `testDeps(t)` / `d.st` harness the discovery handler tests use — see `TestStoreAndSearchDiscoveryHandlers` in `tools_test.go` for the exact helper names in this package):

```go
func TestStoreRuleHandler(t *testing.T) {
	d := testDeps(t) // same harness as TestStoreAndSearchDiscoveryHandlers
	ctx := context.Background()
	scope := "rule:repo:store-rule-handler-test"
	t.Cleanup(func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) })

	id, err := d.storeRule(ctx, storeRuleArgs{
		Content: "never force-push shared branches",
		Scope:   scope,
		Summary: "no force-push on shared branches",
		Tags:    []string{"vcs"},
	})
	if err != nil {
		t.Fatalf("storeRule: %v", err)
	}
	got, err := d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Category != "rule" || got.Source != "user-said" || got.Visibility != "shared" {
		t.Errorf("server-set fields wrong: category=%q source=%q visibility=%q",
			got.Category, got.Source, got.Visibility)
	}
	if got.Summary != "no force-push on shared branches" || got.SummarySource != store.SummarySourceClient {
		t.Errorf("summary not persisted as client: summary=%q source=%q", got.Summary, got.SummarySource)
	}

	// Invalid args are rejected before any write.
	if _, err := d.storeRule(ctx, storeRuleArgs{Content: "x", Scope: "repo:x", Summary: "s"}); err == nil {
		t.Error("expected rejection of non-rule scope")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestStoreRuleHandler -v`
Expected: FAIL — `d.storeRule` undefined.

- [ ] **Step 3: Write the handler**

In `internal/server/rules.go`, remove the `var _ = store.SummarySourceClient` placeholder line and add (mirrors `storeDiscovery` at `tools.go:544-584`, including the id-replace ownership gate via `OwnedOrAbsent`):

```go
func (d *deps) storeRule(ctx context.Context, a storeRuleArgs) (string, error) {
	if err := validateStoreRule(a); err != nil {
		return "", err
	}
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return "", err
	}
	if a.ID != "" {
		if err := d.st.OwnedOrAbsent(ctx, a.ID, subj); err != nil {
			return "", err
		}
	}
	vec, err := d.em.Embed(ctx, store.EmbedText(a.Content, a.Tags))
	if err != nil {
		return "", err
	}
	id := a.ID
	if id == "" {
		id = uuid.NewString()
	}
	m := store.Memory{
		ID:            id,
		Content:       a.Content,
		Scope:         a.Scope,
		Source:        "user-said",
		Category:      "rule",
		Visibility:    "shared",
		Tags:          a.Tags,
		Summary:       a.Summary,
		SummarySource: store.SummarySourceClient,
		Actor:         actorFromContext(ctx),
		Owner:         subj.Owner(),
		CreatedAt:     d.clock(),
	}
	return m.ID, d.st.Upsert(ctx, m, vec)
}
```

Add `"github.com/google/uuid"` to the import block of `rules.go` (same import path used in `tools.go`).

- [ ] **Step 4: Register the tool**

In `internal/server/tools.go`, immediately before the final `return nil` of `Register` (after the `set_visibility` `mcp.AddTool` block ending at :866):

```go
	mcp.AddTool(s, &mcp.Tool{Name: "store_rule", Description: "Persist a NORMATIVE rule (ground truth) for a repo/project. Call ONLY on explicit user instruction — never promote a rule unilaterally; propose it to the user instead. scope=rule:repo:<repo> or rule:project:<project>. summary is REQUIRED and is the one-line index entry (single line). Rules are always shared and user-blessed."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a storeRuleArgs) (*mcp.CallToolResult, any, error) {
			id, err := d.storeRule(ctx, a)
			return textResult(fmt.Sprintf("stored rule %s", id)), map[string]string{"id": id}, err
		})
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestStoreRuleHandler -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
jj commit -m "feat(server): store_rule tool + handler (engram-3jo0)"
```

---

## Task 4: `list_rules` handler + registration

**Model:** sonnet

**Files:**

- Modify: `internal/server/rules.go`
- Modify: `internal/server/tools.go` (register in `Register`)
- Test: `internal/server/rules_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/server/rules_test.go`:

```go
func TestListRulesHandler(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "rule:repo:list-rules-test"
	t.Cleanup(func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) })

	// Seed three rules; created_at ordering is set by d.clock() — store them in
	// a known order and assert ascending oldest-first.
	for i, s := range []string{"rule A", "rule B", "rule C"} {
		if _, err := d.storeRule(ctx, storeRuleArgs{
			Content: s, Scope: scope, Summary: s, Tags: []string{"x"},
		}); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	// Compact (default) shape: complete set, ascending, ruleView fields.
	rules, advisory, err := d.listRules(ctx, listRulesArgs{Scopes: []string{scope}})
	if err != nil {
		t.Fatalf("listRules: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("got %d rules, want 3", len(rules))
	}
	first, ok := rules[0].(ruleView)
	if !ok {
		t.Fatalf("compact shape is not ruleView: %T", rules[0])
	}
	if first.Summary != "rule A" {
		t.Errorf("not ascending oldest-first: first summary=%q want %q", first.Summary, "rule A")
	}
	if advisory != "" {
		t.Errorf("unexpected advisory under threshold: %q", advisory)
	}

	// Tags AND filter: all carry "x", none carry "y".
	none, _, err := d.listRules(ctx, listRulesArgs{Scopes: []string{scope}, Tags: []string{"y"}})
	if err != nil {
		t.Fatalf("listRules tags: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("tags=[y] should match nothing, got %d", len(none))
	}

	// Full shape returns store.Memory (carries content).
	full, _, err := d.listRules(ctx, listRulesArgs{Scopes: []string{scope}, Full: true})
	if err != nil {
		t.Fatalf("listRules full: %v", err)
	}
	if _, ok := full[0].(store.Memory); !ok {
		t.Errorf("full shape is not store.Memory: %T", full[0])
	}

	// Invalid scope rejected.
	if _, _, err := d.listRules(ctx, listRulesArgs{Scopes: []string{"repo:x"}}); err == nil {
		t.Error("expected rejection of non-rule scope")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestListRulesHandler -v`
Expected: FAIL — `d.listRules`, `ruleView` undefined.

- [ ] **Step 3: Write the compact shape + handler**

In `internal/server/rules.go`, add:

```go
// ruleThreshold is the soft rule-count ceiling per scope above which listRules
// returns a curation-smell advisory (textResult only; the {rules} payload is
// unaffected). A rule set is definitionally small.
const ruleThreshold = 50

// ruleView is the compact list_rules result: the one-line index entry plus the
// short handle callers paste into get_memory. ShortID is populated from
// store.Memory.ShortID (added by engram-c0yl); empty until that lands.
type ruleView struct {
	ShortID   string    `json:"short_id,omitempty"`
	ID        string    `json:"id"`
	Summary   string    `json:"summary"`
	Tags      []string  `json:"tags,omitempty"`
	Scope     string    `json:"scope"`
	CreatedAt time.Time `json:"created_at"`
}

func toRuleView(m store.Memory) ruleView {
	return ruleView{
		ShortID: m.ShortID, ID: m.ID, Summary: m.Summary,
		Tags: m.Tags, Scope: m.Scope, CreatedAt: m.CreatedAt,
	}
}

// listRules returns the complete rule set across the given rule:* scopes,
// oldest-first, as compact ruleView values (or full store.Memory when full).
// The second return is a human-readable curation advisory for the tool's
// textResult (empty when under threshold); it never changes the {rules} payload.
func (d *deps) listRules(ctx context.Context, a listRulesArgs) (out []any, advisory string, err error) {
	if len(a.Scopes) == 0 {
		return nil, "", fmt.Errorf("at least one rule scope is required")
	}
	for _, sc := range a.Scopes {
		if !validRuleScope(sc) {
			return nil, "", fmt.Errorf("scope must be rule:repo:<repo> or rule:project:<project>, got %q", sc)
		}
	}
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return nil, "", err
	}
	var over []string
	for _, sc := range a.Scopes {
		// Limit:0 = all; Ascending = oldest-first; Categories pins the rule kind.
		ms, _, _, lerr := d.st.List(ctx, sc, subj, store.ListOptions{
			Limit:      0,
			Ascending:  true,
			Categories: []string{"rule"},
			Tags:       a.Tags,
		})
		if lerr != nil {
			return nil, "", lerr
		}
		if len(ms) > ruleThreshold {
			over = append(over, fmt.Sprintf("%d rules in %s", len(ms), sc))
		}
		for _, m := range ms {
			if a.Full {
				out = append(out, m)
			} else {
				out = append(out, toRuleView(m))
			}
		}
	}
	if len(over) > 0 {
		advisory = "curation smell — " + strings.Join(over, "; ") + " — consider consolidating"
	}
	return out, advisory, nil
}
```

Add `"time"` to the `rules.go` import block.

- [ ] **Step 4: Register the tool**

In `internal/server/tools.go`, after the `store_rule` registration from Task 3:

```go
	mcp.AddTool(s, &mcp.Tool{Name: "list_rules", Description: "List the COMPLETE rule set for one or more rule:* scopes, oldest-first. Compact index shape by default (short_id, summary, tags); full=true adds content. Optional tags filter (AND). Rules are the repo/project's normative ground truth."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a listRulesArgs) (*mcp.CallToolResult, any, error) {
			rules, advisory, err := d.listRules(ctx, a)
			msg := fmt.Sprintf("%d rules", len(rules))
			if advisory != "" {
				msg += " (" + advisory + ")"
			}
			return textResult(msg), map[string]any{"rules": rules}, err
		})
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestListRulesHandler -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
jj commit -m "feat(server): list_rules tool + handler with ascending complete-set recall (engram-3jo0)"
```

---

## Task 5: `update_memory` rule guard

**Model:** haiku

**Files:**

- Modify: `internal/server/tools.go:727-761` (`updateMemory`)
- Test: `internal/server/rules_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/server/rules_test.go`:

```go
func TestUpdateMemoryRuleGuard(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "rule:repo:update-rule-guard-test"
	t.Cleanup(func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) })

	id, err := d.storeRule(ctx, storeRuleArgs{
		Content: "original rule text", Scope: scope, Summary: "original summary",
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	newline := "bad\nsummary"
	long := strings.Repeat("a", maxRuleSummaryBytes+1)
	empty := ""
	cases := []struct {
		name string
		a    updateArgs
	}{
		{"newline summary", updateArgs{ID: id, Content: "original rule text", Summary: &newline}},
		{"oversize summary", updateArgs{ID: id, Content: "original rule text", Summary: &long}},
		{"clear summary", updateArgs{ID: id, Content: "original rule text", Summary: &empty}},
	}
	for _, tc := range cases {
		if err := d.updateMemory(ctx, tc.a); err == nil {
			t.Errorf("%s: expected rejection, got nil", tc.name)
		}
	}

	// A valid single-line summary replacement still succeeds.
	okSummary := "revised summary"
	if err := d.updateMemory(ctx, updateArgs{ID: id, Content: "revised rule text", Summary: &okSummary}); err != nil {
		t.Errorf("valid rule update rejected: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestUpdateMemoryRuleGuard -v`
Expected: FAIL — clear/newline/oversize summaries currently succeed (no rule guard yet).

- [ ] **Step 3: Add the guard**

In `internal/server/tools.go`, inside `updateMemory`, insert the rule check right after the `FetchForUpdate` call succeeds (after :737, before the `resolveSummaryUpdate` call at :741):

```go
	// Rule guard: a rule's summary is its index line — it must stay a non-empty
	// single line. Reject a clearing/newline/oversize summary before embed/write.
	// (cur.Category is known for free from the fetch above.)
	if cur.Category == "rule" && a.Summary != nil {
		if err := validateRuleSummary(*a.Summary); err != nil {
			return err
		}
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestUpdateMemoryRuleGuard -v`
Expected: PASS

- [ ] **Step 5: Verify no regression on ordinary update paths**

Run: `go test ./internal/server/ -run 'TestUpdate|TestSummary' -v`
Expected: PASS (guard is scoped to `cur.Category == "rule"`).

- [ ] **Step 6: Commit**

```bash
jj commit -m "feat(server): update_memory rejects invalid rule summaries (engram-3jo0)"
```

---

## Task 6: `set_visibility` rule rejection (post-c0yl handler)

**Model:** opus

**Files:**

- Modify: `internal/server/tools.go` (the `set_visibility` `mcp.AddTool` handler — **post-c0yl shape**, which resolves `a.ID` to `pid` via `ResolvePointID`)
- Test: `internal/server/rules_test.go`

> **Prerequisite reminder:** this task edits the c0yl-rewritten `set_visibility` handler (per c0yl plan Task 8). If `rg -n "ResolvePointID" internal/server/tools.go` returns nothing, c0yl has not merged — stop.

- [ ] **Step 1: Write the failing test**

Add to `internal/server/rules_test.go`:

```go
func TestSetVisibilityRejectsRule(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "rule:repo:set-vis-rule-test"
	t.Cleanup(func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) })

	id, err := d.storeRule(ctx, storeRuleArgs{
		Content: "some rule", Scope: scope, Summary: "some rule",
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	// The set_visibility handler must reject any visibility change on a rule.
	err = d.setVisibility(ctx, setVisibilityArgs{ID: id, Shared: false})
	if err == nil {
		t.Fatal("expected set_visibility on a rule to be rejected")
	}
	if !strings.Contains(err.Error(), "always shared") {
		t.Errorf("expected 'always shared' message, got %v", err)
	}

	// The rule is untouched: still shared.
	got, err := d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Visibility != "shared" {
		t.Errorf("rule visibility mutated to %q", got.Visibility)
	}
}
```

> Note: this test calls `d.setVisibility(...)`. c0yl Task 8 extracts the inline `set_visibility` closure body into a `func (d *deps) setVisibility(ctx, a setVisibilityArgs) error` method. If c0yl left it inline, first extract it into that method (pure refactor, no behavior change) so the guard and test have a seam — then proceed.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestSetVisibilityRejectsRule -v`
Expected: FAIL — no rule guard; the unshare succeeds and mutates visibility (or the test won't compile until `setVisibility` is a method — see the note).

- [ ] **Step 3: Add the guard (category-first, per the spec's implementation-order note)**

In the `setVisibility` handler, after `pid` is resolved via `ResolvePointID` and before the `d.st.SetVisibility(ctx, pid, ...)` call, insert:

```go
	// Rules are always shared: reject any visibility change on a rule. Read the
	// record to learn its category (ResolvePointID returns only the UUID). Run
	// this BEFORE the write-ownership gate so the actionable "always shared"
	// message wins over an owner-only ErrNotFound (spec implementation-order note;
	// not a leak — rules are unconditionally readable).
	rec, err := d.st.GetReadable(ctx, pid, subj)
	if err != nil {
		return err
	}
	if rec.Category == "rule" {
		return fmt.Errorf("rules are always shared — delete the rule instead of changing its visibility")
	}
```

The resulting handler body reads: resolve `pid` → `GetReadable(pid)` category check → `store.SetVisibility(pid, subj, shared)`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestSetVisibilityRejectsRule -v`
Expected: PASS

- [ ] **Step 5: Verify no regression on ordinary set_visibility**

Run: `go test ./internal/server/ ./internal/store/ -run 'TestSetVisibility|TestUpdateMemoryPreservesSharing' -v`
Expected: PASS (non-rule records still share/unshare normally).

- [ ] **Step 6: Commit**

```bash
jj commit -m "feat(server): set_visibility rejects rules (always shared) (engram-3jo0)"
```

---

## Task 7: Session-start hook — rules index + `ENGRAM_PROJECT`

**Model:** sonnet

**Files:**

- Modify: `skill/engram/hooks/session-start-memory-recall`
- Test: `skill/engram/hooks/tests/test_session_start_memory_recall.py`

- [ ] **Step 1: Write the failing test**

The hook is a hyphenated, extensionless script — it is NOT importable. The existing tests **subprocess** it via the `run_hook(cwd)` helper and read `additionalContext` from the emitted JSON (see `test_session_start_memory_recall.py:18-25` for `run_hook` and `:32-55` for the `git_repo` fixture that yields spine `repo:github.com/org/repo`). Match that pattern. `run_hook` passes no `env=`, so the subprocess inherits the parent environment — `monkeypatch.setenv/delenv` on `ENGRAM_PROJECT` reaches the hook.

Add to `skill/engram/hooks/tests/test_session_start_memory_recall.py`:

```python
def test_rules_index_instruction_present(git_repo: Path, monkeypatch):
    monkeypatch.delenv("ENGRAM_PROJECT", raising=False)
    result = run_hook(str(git_repo))
    assert result.returncode == 0
    ctx = json.loads(result.stdout)["hookSpecificOutput"]["additionalContext"]
    # Rules-index instruction names list_rules and the rule scope for the spine.
    assert "list_rules" in ctx
    assert "rule:repo:github.com/org/repo" in ctx
    # No project scope when ENGRAM_PROJECT is unset.
    assert "rule:project:" not in ctx


def test_rules_index_includes_project_scope_when_configured(git_repo: Path, monkeypatch):
    monkeypatch.setenv("ENGRAM_PROJECT", "selfhosted-cluster")
    ctx = json.loads(run_hook(str(git_repo)).stdout)["hookSpecificOutput"]["additionalContext"]
    assert "rule:project:selfhosted-cluster" in ctx
```

- [ ] **Step 2: Run test to verify it fails**

Run: `task test:python` (or `cd skill/engram/hooks && uv run pytest tests/test_session_start_memory_recall.py -v`)
Expected: FAIL — `list_rules` / `rule:repo:...` not in the emitted context.

- [ ] **Step 3: Derive the rule scopes and inject the index instruction**

In `skill/engram/hooks/session-start-memory-recall`, change `_build_context` to accept the derived spine's repo id and emit a rules block. First, add a helper and thread the env var. Replace the `_build_context` signature and add the rules block before the final `return "\n".join(lines)`:

```python
def _rule_scopes(spine: str) -> list[str]:
    """Rule scopes to fetch: rule:<repo-tail> from the spine, plus the optional
    project scope from ENGRAM_PROJECT (client-side hook env var, not the Go
    internal/config registry)."""
    scopes: list[str] = []
    # spine is "repo:<repo-id>"; the rule scope mirrors it under the rule: prefix.
    if spine.startswith("repo:"):
        scopes.append("rule:" + spine)  # rule:repo:<repo-id>
    project = os.environ.get("ENGRAM_PROJECT", "").strip()
    if project:
        scopes.append(f"rule:project:{project}")
    return scopes
```

Then, inside `_build_context`, after the capture block (before `return`):

```python
    rule_scopes = _rule_scopes(spine)
    if rule_scopes:
        lines.append("")
        joined = ", ".join(rule_scopes)
        lines.append(
            "Repository rules (normative ground truth) — after the recall digest, "
            f"call mcp__engram__list_rules once with scopes=[{joined}] and render a "
            "compact Rules index: one line per rule as `short_id — summary [tags]`. "
            "This is an INDEX only, not the full rules: do NOT fetch or inline rule "
            "content at startup. When you are about to act in a rule's concern area, "
            "fetch that one rule's full text on demand via get_memory(<short_id>). "
            "If list_rules returns nothing, omit the Rules section entirely. Rules "
            "are always-shared, user-blessed constraints — treat them as MUST-follow."
        )
```

Note: `spine` already has the `repo:` form (see `lib/scope.py:derive_scopes`, which returns `spine = "repo:<repo-id>"`), so `"rule:" + spine` yields `rule:repo:<repo-id>` — the exact scope `store_rule` writes.

- [ ] **Step 4: Run test to verify it passes**

Run: `task test:python`
Expected: PASS (both new tests + all existing hook tests).

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(hook): inject a progressive-disclosure rules index at session start (engram-3jo0)"
```

---

## Task 8: Docs + skill contract

**Model:** haiku

**Files:**

- Modify: `docs-site/src/content/docs/reference/tools.md`
- Modify: `CLAUDE.md`
- Modify: `skill/engram/skills/curating-memory/SKILL.md`

- [ ] **Step 1: Document the tools in the docs-site reference**

In `docs-site/src/content/docs/reference/tools.md`, add entries for `store_rule` and `list_rules` alongside the existing tool docs (match the surrounding heading level and prose style). Include, for each: purpose, the arg list, server-set fields (`category=rule`, `source=user-said`, `visibility=shared`), the user-blessed-only contract, the `rule:repo:*` / `rule:project:*` scope forms, and the single-line-summary requirement. For `list_rules`: complete-set semantics, oldest-first ordering (MCP-`list_rules`-only), compact vs `full`, the tags AND filter, and that the >50 advisory is a `textResult` note only.

- [ ] **Step 2: Update the memory contract in CLAUDE.md**

In `CLAUDE.md`, under "## Memory contract (stable)", add `store_rule` / `list_rules` to the tool list and a short paragraph: a `rule` is a 6th category (normative, user-blessed, always-shared ground truth) in a dedicated `rule:repo:*` / `rule:project:*` scope, surfaced as a session-start index (progressive disclosure). Note `set_visibility` is rejected for rules and the summary must be a single line. Mirror the phrasing used for the discovery-tools paragraph already in that section.

- [ ] **Step 3: Document the user-blessed-only contract in curating-memory**

In `skill/engram/skills/curating-memory/SKILL.md`, add a short subsection: `store_rule` is invoked ONLY on explicit user instruction. An agent that believes something should be a rule proposes it to the user and never promotes unilaterally. Rules are normative (MUST-follow), always shared, and their summary is the one-line index entry. This complements (does not replace) the decision/preference/convention/gotcha routing already documented.

- [ ] **Step 4: Verify docs lint + license headers**

Run: `task license:check && task lint`
Expected: PASS. If `license:check` flags the new `internal/server/rules.go` / `internal/server/rules_test.go`, run `task license:add`. (Markdown docs edited in place already carry headers; new files do not apply here.)

- [ ] **Step 5: Commit**

```bash
jj commit -m "docs: document store_rule/list_rules + user-blessed rule contract (engram-3jo0)"
```

---

## Final Verification

- [ ] **Full test suite**

Run: `task test`
Expected: PASS (Go + Python).

- [ ] **Full lint + license gate**

Run: `task` (lint + test) — the repo's default gate.
Expected: clean.

- [ ] **Manual smoke (optional, needs a running engram + Qdrant)**

Store a rule, list it, confirm the index shape and the set_visibility rejection:

```text
store_rule(content="never push to main directly; open a PR", scope="rule:repo:github.com/seanb4t/engram", summary="never push to main directly; PRs only", tags=["vcs"])
list_rules(scopes=["rule:repo:github.com/seanb4t/engram"])   # → one ruleView, short_id + summary
set_visibility(id=<that id>, shared=false)                    # → rejected: "rules are always shared"
```

---

## Spec Coverage Map

| Spec requirement | Task |
|------------------|------|
| 6th `category` = `rule`, no schema change | Tasks 3 (Memory assembly) |
| `rule:repo:*` / `rule:project:*` scope isolation | Task 2 (`validRuleScope`), Task 4 (`Categories` filter) |
| Server-set `category`/`source`/`visibility` | Task 3 |
| `store_rule` validation (scope, content cap, single-line summary, summary cap) | Task 2 |
| `list_rules` complete-set, ascending, compact/full, tags-AND, textResult advisory | Task 4 |
| `ListOptions.Ascending` additive store change | Task 1 |
| `update_memory` rule summary guard (newline/oversize/clear) | Task 5 |
| `set_visibility` rejection for rules (handler `GetReadable`, category-first) | Task 6 |
| Session-start rules index (progressive disclosure) | Task 7 |
| `ENGRAM_PROJECT` client-side project scope | Task 7 |
| Docs (tools ref, CLAUDE.md) + user-blessed contract (curating-memory) | Task 8 |
| short_id in index + by-id correction loop | Prerequisite (engram-c0yl) + Tasks 4/6 |
<!-- adr-capture: sha256=b267ee133b37007f; session=cli; ts=2026-07-06T22:18:41Z; adrs=engram-iedk,engram-d386,engram-m4s8 -->
