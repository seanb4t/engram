// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file pins the text content of engram's five read tools to their
// structured result. MCP 2026-07-28 (server/tools § Structured Content) says a
// tool returning structured content SHOULD also return the serialized JSON in a
// TextContent block, and go-sdk v1.8.0's AddTool does exactly that when a
// handler returns a nil *CallToolResult. A client that forwards `content` in
// preference to `structuredContent` (hermes-agent does) otherwise sees only a
// count such as "2 hits" and loses the records. Every assertion goes through a
// real mcp.ClientSession, so it proves the wire result, not a handler's return.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/seanb4t/engram/internal/store"
)

// callToolTextJSON calls tool and returns its structured result and the JSON
// its single TextContent block decodes to, failing unless the result is a
// non-error with exactly one TextContent.
func callToolTextJSON(ctx context.Context, t *testing.T, cs *mcp.ClientSession, tool string, args map[string]any) (structured map[string]any, text string) {
	t.Helper()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", tool, err)
	}
	if res.IsError {
		t.Fatalf("CallTool(%s): IsError = true, want false (content: %+v)", tool, res.Content)
	}
	if len(res.Content) != 1 {
		t.Fatalf("CallTool(%s): len(Content) = %d, want 1 (content: %+v)", tool, len(res.Content), res.Content)
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("CallTool(%s): Content[0] is %T, want *mcp.TextContent", tool, res.Content[0])
	}
	m, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("CallTool(%s): StructuredContent is %T, want map[string]any", tool, res.StructuredContent)
	}
	return m, tc.Text
}

// assertTextIsStructuredJSON fails unless text decodes as JSON to a value
// deep-equal to a JSON round-trip of structured.
func assertTextIsStructuredJSON(t *testing.T, tool, text string, structured map[string]any) {
	t.Helper()
	var fromText any
	if err := json.Unmarshal([]byte(text), &fromText); err != nil {
		t.Fatalf("%s: text content %q is not JSON: %v (want the serialized structured result)", tool, text, err)
	}
	raw, err := json.Marshal(structured)
	if err != nil {
		t.Fatalf("%s: marshal structured: %v", tool, err)
	}
	var fromStructured any
	if err := json.Unmarshal(raw, &fromStructured); err != nil {
		t.Fatalf("%s: unmarshal structured: %v", tool, err)
	}
	if !reflect.DeepEqual(fromText, fromStructured) {
		t.Fatalf("%s: text content decodes to\n%v\nwant the structured result\n%v", tool, fromText, fromStructured)
	}
}

// requireNonEmptyList fails unless structured[key] is a non-empty array, so a
// vacuous empty result can never satisfy the text/structured comparison.
func requireNonEmptyList(t *testing.T, tool string, structured map[string]any, key string) {
	t.Helper()
	arr, ok := structured[key].([]any)
	if !ok || len(arr) == 0 {
		t.Fatalf("%s: StructuredContent[%q] = %v, want a non-empty array", tool, key, structured[key])
	}
}

func TestRecallTextJSON(t *testing.T) {
	d := testDeps(t)
	owner := "textjson-owner-" + uuid.NewString()
	scope := "iso-test:project:textjson-" + uuid.NewString()
	scheduledScope := "iso-test:project:textjson-sched-" + uuid.NewString()
	discoveryScope := "discovery:repo:textjson-" + uuid.NewString()
	ruleScope := "rule:repo:textjson-" + uuid.NewString()
	ctx := authedContext(t, owner)
	c := callerFor(ctx, t)
	t.Cleanup(func() {
		bg := context.Background()
		cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(bg, scope, store.Authenticated(owner)))
		cleanupErr(t, "DeleteAll "+scheduledScope, d.st.DeleteAll(bg, scheduledScope, store.Authenticated(owner)))
		cleanupErr(t, "DeleteAll "+discoveryScope, d.st.DeleteAll(bg, discoveryScope, store.Authenticated(owner)))
		cleanupErr(t, "DeleteAll "+ruleScope, d.st.DeleteAll(bg, ruleScope, store.Anonymous()))
	})

	for i := range 2 {
		if _, _, err := d.storeMemory(ctx, c, storeArgs{
			Content: fmt.Sprintf("textjson memory %d", i), Scope: scope, Source: "agent-inferred", Category: "decision",
		}); err != nil {
			t.Fatalf("seed memory %d: %v", i, err)
		}
	}
	future := time.Now().Add(365 * 24 * time.Hour).UTC().Format(time.RFC3339)
	if _, _, err := d.scheduleMemory(ctx, c, scheduleArgs{
		storeArgs: storeArgs{Content: "textjson scheduled", Scope: scheduledScope, Source: "agent-inferred", Category: "decision"},
		NotBefore: future,
	}); err != nil {
		t.Fatalf("seed scheduled: %v", err)
	}
	if _, _, err := d.storeDiscovery(ctx, c, storeDiscoveryArgs{
		Content: "textjson discovery", Kind: "fact", Scope: discoveryScope, Citations: []citationArg{{Kind: "file", Ref: "f"}},
	}); err != nil {
		t.Fatalf("seed discovery: %v", err)
	}
	if _, _, err := d.storeRule(ctx, c, storeRuleArgs{
		Content: "textjson rule", Scope: ruleScope, Summary: "textjson rule",
	}); err != nil {
		t.Fatalf("seed rule: %v", err)
	}

	cs, tctx := newOutOfRangeMCPSession(t, d, owner)

	cases := []struct {
		tool string
		args map[string]any
		key  string
	}{
		{"search_memory", map[string]any{"scope": scope, "query": "textjson memory"}, "memories"},
		{"list_memory", map[string]any{"scope": scope}, "memories"},
		{"list_scheduled", map[string]any{"scope": scheduledScope}, "memories"},
		{"search_discovery", map[string]any{"scope": discoveryScope, "query": "textjson discovery"}, "discoveries"},
		{"list_rules", map[string]any{"scopes": []string{ruleScope}}, "rules"},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			structured, text := callToolTextJSON(tctx, t, cs, tc.tool, tc.args)
			requireNonEmptyList(t, tc.tool, structured, tc.key)
			assertTextIsStructuredJSON(t, tc.tool, text, structured)
		})
	}
}

// TestRecallTextJSONListRulesAdvisory proves list_rules' curation advisory,
// which used to ride only in the text, now lives in the structured result
// under "advisory" — present when a scope is over the threshold, absent when
// none is — so nothing the text carried is lost.
func TestRecallTextJSONListRulesAdvisory(t *testing.T) {
	d := testDeps(t)
	owner := "textjson-adv-owner-" + uuid.NewString()
	overScope := "rule:repo:textjson-adv-over-" + uuid.NewString()
	underScope := "rule:repo:textjson-adv-under-" + uuid.NewString()
	t.Cleanup(func() {
		bg := context.Background()
		cleanupErr(t, "DeleteAll "+overScope, d.st.DeleteAll(bg, overScope, store.Anonymous()))
		cleanupErr(t, "DeleteAll "+underScope, d.st.DeleteAll(bg, underScope, store.Anonymous()))
	})

	seed := func(scope string, n int) {
		t.Helper()
		vec := []float32{0.1, 0.2, 0.3}
		for i := range n {
			m := store.Memory{
				ID:            uuid.NewString(),
				Content:       fmt.Sprintf("rule %d", i),
				Summary:       fmt.Sprintf("rule %d", i),
				Scope:         scope,
				Source:        "user-said",
				Category:      "rule",
				Visibility:    "shared",
				SummarySource: store.SummarySourceClient,
				CreatedAt:     time.Unix(int64(i), 0).UTC(),
			}
			if err := d.st.Upsert(context.Background(), m, vec); err != nil {
				t.Fatalf("seed %s %d: %v", scope, i, err)
			}
		}
	}
	seed(overScope, ruleThreshold+1)
	seed(underScope, 1)

	cs, tctx := newOutOfRangeMCPSession(t, d, owner)

	t.Run("over_threshold", func(t *testing.T) {
		structured, text := callToolTextJSON(tctx, t, cs, "list_rules", map[string]any{"scopes": []string{overScope}})
		adv, ok := structured["advisory"].(string)
		if !ok || !strings.Contains(adv, "curation smell") || !strings.Contains(adv, overScope) {
			t.Fatalf("StructuredContent[\"advisory\"] = %v, want the curation-smell advisory naming %s", structured["advisory"], overScope)
		}
		assertTextIsStructuredJSON(t, "list_rules", text, structured)
	})
	t.Run("under_threshold", func(t *testing.T) {
		structured, text := callToolTextJSON(tctx, t, cs, "list_rules", map[string]any{"scopes": []string{underScope}})
		if v, present := structured["advisory"]; present {
			t.Fatalf("StructuredContent[\"advisory\"] = %v, want the key absent when no scope is over the threshold", v)
		}
		assertTextIsStructuredJSON(t, "list_rules", text, structured)
	})
}

// TestRecallTextJSONWriteToolUnchanged pins that a write tool keeps its own
// text: store_memory still answers "stored <id>" beside its {id, short_id}
// structured result, and gains no second text block.
func TestRecallTextJSONWriteToolUnchanged(t *testing.T) {
	d := testDeps(t)
	owner := "textjson-write-owner-" + uuid.NewString()
	scope := "iso-test:project:textjson-write-" + uuid.NewString()
	t.Cleanup(func() {
		cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(context.Background(), scope, store.Authenticated(owner)))
	})

	cs, tctx := newOutOfRangeMCPSession(t, d, owner)
	structured, text := callToolTextJSON(tctx, t, cs, "store_memory", map[string]any{
		"content": "textjson write", "scope": scope, "source": "agent-inferred", "category": "decision",
	})
	id, ok := structured["id"].(string)
	if !ok || id == "" {
		t.Fatalf("StructuredContent[\"id\"] = %v, want a non-empty id", structured["id"])
	}
	if want := "stored " + id; text != want {
		t.Fatalf("store_memory text = %q, want %q", text, want)
	}
}
