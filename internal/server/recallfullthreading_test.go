// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves 04-06's own share of REQ-list-contract-unchanged: a
// caller's projection choice (full=true/false) selects what the STORE
// fetches on every list lane this phase's Task 1 threads it through — the
// MCP list_memory tool and the Connect ListMemories RPC (both wired in
// 04-05's own Rule 2 fix) and, newly here, list_rules — the one list caller
// outside the typed core (04-RESEARCH.md Pattern 6, step 4). Never how each
// transport SHAPES its response, which is unchanged and re-asserted here as
// a regression guard.
//
// Real Qdrant only (rule m45p2b4bp7): a mutex-guarded
// grpc.UnaryClientInterceptor records each Scroll RPC's payload selector,
// mirroring internal/store/recallview_oversized_test.go's recallViewRecorder
// — the server package's equivalent, kept minimal to this file's own needs
// (direction only, not limit/id-set shape, which the store package's own
// test already proves at the store's entry point).
package server

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)

// scrollSelectorCall is one intercepted Scroll RPC's payload-selector
// direction and whether its filter carries a top-level Must has_id
// condition — the shape a backfillNoSummaryContent id-set fetch (D-04, plan
// 04-05) always produces and a page fetch never does, letting this file's
// tests tell the two apart without re-deriving the store's own filter logic
// (mirrors internal/store/recallview_oversized_test.go's
// recallViewCallShape/filterHasTopLevelHasID).
type scrollSelectorCall struct {
	exclude bool // true = summary-view (content/citations excluded) selector
	idSet   bool // true = a top-level Must has_id filter (a backfill fetch)
}

func filterHasTopLevelHasID(f *qdrant.Filter) bool {
	if f == nil {
		return false
	}
	for _, cond := range f.GetMust() {
		if cond.GetHasId() != nil {
			return true
		}
	}
	return false
}

// scrollSelectorRecorder is a mutex-guarded recording grpc.UnaryClientInterceptor
// scoped to methods ending "/Scroll", recording each call's selector
// direction and id-set shape — the server package's equivalent of
// internal/store/recallview_oversized_test.go's recallViewRecorder, kept
// minimal to this file's own needs (no per-call limit).
type scrollSelectorRecorder struct {
	calls []scrollSelectorCall
	muCh  chan struct{}
}

func newScrollSelectorRecorder() *scrollSelectorRecorder {
	r := &scrollSelectorRecorder{muCh: make(chan struct{}, 1)}
	r.muCh <- struct{}{}
	return r
}

func (r *scrollSelectorRecorder) lock()   { <-r.muCh }
func (r *scrollSelectorRecorder) unlock() { r.muCh <- struct{}{} }

func (r *scrollSelectorRecorder) intercept(
	ctx context.Context, method string, req, reply any,
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
) error {
	if len(method) < len("/Scroll") || method[len(method)-len("/Scroll"):] != "/Scroll" {
		return invoker(ctx, method, req, reply, cc, opts...)
	}
	var call scrollSelectorCall
	if sp, ok := req.(*qdrant.ScrollPoints); ok {
		call.exclude = sp.GetWithPayload().GetExclude() != nil
		call.idSet = filterHasTopLevelHasID(sp.GetFilter())
	}
	err := invoker(ctx, method, req, reply, cc, opts...)
	r.lock()
	r.calls = append(r.calls, call)
	r.unlock()
	return err
}

func (r *scrollSelectorRecorder) reset() {
	r.lock()
	r.calls = nil
	r.unlock()
}

func (r *scrollSelectorRecorder) snapshot() []scrollSelectorCall {
	r.lock()
	out := make([]scrollSelectorCall, len(r.calls))
	copy(out, r.calls)
	r.unlock()
	return out
}

// pageCalls filters out backfill (id-set) calls, returning only the page
// fetches whose selector reflects the caller's own projection choice — a
// backfill fetch's selector is fixed by what it needs (content), not by the
// caller's flag, so mixing it into a direction check would be a false
// negative on the very call the D-04 fallback deliberately overrides.
func pageCalls(calls []scrollSelectorCall) []scrollSelectorCall {
	out := make([]scrollSelectorCall, 0, len(calls))
	for _, c := range calls {
		if !c.idSet {
			out = append(out, c)
		}
	}
	return out
}

// allExclude reports whether at least one page call was recorded and every
// one of them carried the exclude-shaped (summary) selector.
func allExclude(calls []scrollSelectorCall) bool {
	pages := pageCalls(calls)
	if len(pages) == 0 {
		return false
	}
	for _, c := range pages {
		if !c.exclude {
			return false
		}
	}
	return true
}

// noneExclude reports whether at least one page call was recorded and none
// of them carried the exclude-shaped (summary) selector — i.e. every page
// call asked for the full view.
func noneExclude(calls []scrollSelectorCall) bool {
	pages := pageCalls(calls)
	if len(pages) == 0 {
		return false
	}
	for _, c := range pages {
		if c.exclude {
			return false
		}
	}
	return true
}

// testDepsWithRecorder builds a Qdrant-backed deps exactly like
// testDepsWithStore (tools_test.go), except its dial carries the given
// interceptor so a test can observe the Scroll selector every recall in this
// file issues.
func testDepsWithRecorder(t *testing.T, rec *scrollSelectorRecorder) *deps {
	t.Helper()
	c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(rec.intercept))
	st := newTestStore(t, c, testCollection("recallfull_"+uuid.NewString()))
	if err := st.EnsureCollection(context.Background(), 3); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	// summaryMaxChars mirrors summaryMaxChars(cfg)'s production default (280,
	// tools.go): the bare &deps{} literal other tests use leaves this 0,
	// which would truncate every no-summary fallback to a bare "…" — this
	// file's tests need the real fallback text to build a reliable lookup key.
	return &deps{st: st, em: fakeEmbedder{}, summaryMaxChars: 280}
}

// TestFullSelectsFetchView drives the real MCP list_memory tool (over an
// in-memory transport, through registerTools — not the shared core
// directly) and the real Connect ListMemories RPC (through engramAPI) against
// a store dialed with a selector recorder. With the projection flag unset,
// every recorded Scroll carries the summary (exclude) selector; with it set,
// every recorded Scroll carries the full selector. Both lanes' response
// shaping stays exactly as it is today: the compact shape never carries
// content/citations and the full shape always does, in both runs — proving
// 04-05's Rule 2 wiring end to end through the real entry points, not the
// typed core.
func TestFullSelectsFetchView(t *testing.T) {
	rec := newScrollSelectorRecorder()
	d := testDepsWithRecorder(t, rec)
	owner := "recallfull-owner-" + uuid.NewString()
	scope := "iso-test:project:recallfull-" + uuid.NewString()
	ctx := authedContext(t, owner)
	c := callerFor(ctx, t)
	t.Cleanup(func() {
		cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(context.Background(), scope, store.Authenticated(owner)))
	})

	withSummaryContent := "recallfull with-summary content, never fetched by a default list"
	if _, _, err := d.storeMemory(ctx, c, storeArgs{
		Content: withSummaryContent, Scope: scope, Source: "agent-inferred",
		Category: "decision", Summary: "with-summary",
	}); err != nil {
		t.Fatalf("seed with-summary: %v", err)
	}
	// D-04's no-summary truncation fallback (Phase 3) must still see content
	// on every lane this test drives, regardless of which view the store
	// fetched — the compact shape's summary is DERIVED from content when no
	// summary was stored.
	noSummaryContent := "recallfull no-summary content that must still echo, truncated, in the compact shape"
	if _, _, err := d.storeMemory(ctx, c, storeArgs{
		Content: noSummaryContent, Scope: scope, Source: "agent-inferred", Category: "decision",
	}); err != nil {
		t.Fatalf("seed no-summary: %v", err)
	}

	s := mcp.NewServer(&mcp.Implementation{Name: "engram-test", Version: "test"}, nil)
	if err := registerTools(s, d); err != nil {
		t.Fatalf("registerTools: %v", err)
	}
	tctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ss, err := s.Connect(authedContext(t, owner), serverTransport, nil)
	if err != nil {
		t.Fatalf("server Connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	mc := mcp.NewClient(&mcp.Implementation{Name: "engram-test-client", Version: "test"}, nil)
	cs, err := mc.Connect(tctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client Connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	t.Run("MCP list_memory", func(t *testing.T) {
		rec.reset()
		compact, err := cs.CallTool(tctx, &mcp.CallToolParams{Name: "list_memory", Arguments: map[string]any{"scope": scope, "limit": 10}})
		if err != nil || compact.IsError {
			t.Fatalf("CallTool list_memory full=false: err=%v isError=%v", err, compact.IsError)
		}
		if snap := rec.snapshot(); !allExclude(snap) {
			t.Errorf("list_memory full=false: recorded Scroll selector(s) did not all exclude content/citations — want the summary view; snapshot=%+v", snap)
		}
		compactMems := mustMemoriesField(t, compact.StructuredContent)
		// noSummaryContent is short enough that truncateForRecall (summary.go)
		// returns it unchanged, so the compact shape's derived summary equals
		// the raw content verbatim — a reliable lookup key distinct from the
		// with-summary record's own (different) summary text.
		compactNoSummary := findByField(t, compactMems, "summary", noSummaryContent)
		if _, has := compactNoSummary["content"]; has {
			t.Errorf("list_memory full=false: compact record carries a content field, want none: %+v", compactNoSummary)
		}
		if summary, _ := compactNoSummary["summary"].(string); summary == "" {
			t.Errorf("list_memory full=false: no-summary record's compact summary is empty, want the truncated-content fallback")
		}

		rec.reset()
		full, err := cs.CallTool(tctx, &mcp.CallToolParams{Name: "list_memory", Arguments: map[string]any{"scope": scope, "limit": 10, "full": true}})
		if err != nil || full.IsError {
			t.Fatalf("CallTool list_memory full=true: err=%v isError=%v", err, full.IsError)
		}
		if !noneExclude(rec.snapshot()) {
			t.Errorf("list_memory full=true: recorded Scroll selector(s) excluded content/citations — want the full view")
		}
		fullMems := mustMemoriesField(t, full.StructuredContent)
		fullNoSummary := findByField(t, fullMems, "id", compactNoSummary["id"])
		if got, _ := fullNoSummary["content"].(string); got != noSummaryContent {
			t.Errorf("list_memory full=true: no-summary record content = %q, want %q", got, noSummaryContent)
		}
	})

	api := &engramAPI{d: d}
	actx := withConnectTokenInfo(context.Background(), &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": owner}})

	t.Run("Connect ListMemories", func(t *testing.T) {
		rec.reset()
		compact, err := api.ListMemories(actx, connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope, Limit: 10, Full: false}))
		if err != nil {
			t.Fatalf("ListMemories full=false: %v", err)
		}
		if !allExclude(rec.snapshot()) {
			t.Errorf("ListMemories full=false: recorded Scroll selector(s) did not all exclude content/citations — want the summary view")
		}
		compactNoSummary := findProtoBySummary(t, compact.Msg.Memories, noSummaryContent)
		if compactNoSummary.Content != "" {
			t.Errorf("ListMemories full=false: Content = %q, want cleared", compactNoSummary.Content)
		}
		if len(compactNoSummary.Citations) != 0 {
			t.Errorf("ListMemories full=false: Citations = %+v, want empty", compactNoSummary.Citations)
		}

		rec.reset()
		full, err := api.ListMemories(actx, connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope, Limit: 10, Full: true}))
		if err != nil {
			t.Fatalf("ListMemories full=true: %v", err)
		}
		if !noneExclude(rec.snapshot()) {
			t.Errorf("ListMemories full=true: recorded Scroll selector(s) excluded content/citations — want the full view")
		}
		fullNoSummary := findProtoByID(t, full.Msg.Memories, compactNoSummary.Id)
		if fullNoSummary.Content != noSummaryContent {
			t.Errorf("ListMemories full=true: no-summary record content = %q, want %q", fullNoSummary.Content, noSummaryContent)
		}
	})
}

// TestListRulesFullThreaded proves 04-06 Task 1's own fix: the rule
// listing's direct Store.List call (the one list caller outside the typed
// core, internal/server/rules.go) threads a.Full into the store's fetch
// projection, so a full=true rule read pulls the full view and a compact
// read pulls the summary view — exactly the same regression 04-05 already
// fixed for list_memory/ListMemories. The compact (ruleView) and full
// (store.Memory) shapes returned are exactly what they are today.
//
// Rules mandate a non-empty Summary (validateRuleSummary): unlike
// list_memory's no-summary truncation fallback, list_rules' toRuleView
// copies Summary verbatim with no truncation path to exercise, so this test
// asserts the seeded Summary/Content pair instead of a no-summary fallback —
// see the SUMMARY's own note on this.
func TestListRulesFullThreaded(t *testing.T) {
	rec := newScrollSelectorRecorder()
	d := testDepsWithRecorder(t, rec)
	ctx := context.Background()
	scope := "rule:repo:recallfull-threaded-" + uuid.NewString()
	c := callerFor(authedContext(t, "recallfull-rule-owner"), t)
	t.Cleanup(func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) })

	ruleContent := "recallfull rule content, must reach the full shape only when full=true"
	ruleSummary := "recallfull rule summary"
	if _, _, err := d.storeRule(ctx, c, storeRuleArgs{Content: ruleContent, Scope: scope, Summary: ruleSummary}); err != nil {
		t.Fatalf("storeRule: %v", err)
	}

	rec.reset()
	compact, _, err := d.listRules(ctx, c, listRulesArgs{Scopes: []string{scope}})
	if err != nil {
		t.Fatalf("listRules full=false: %v", err)
	}
	if !allExclude(rec.snapshot()) {
		t.Errorf("listRules full=false: recorded Scroll selector(s) did not all exclude content/citations — want the summary view")
	}
	if len(compact) != 1 {
		t.Fatalf("listRules full=false: got %d rules, want 1", len(compact))
	}
	cv, ok := compact[0].(ruleView)
	if !ok {
		t.Fatalf("listRules full=false: compact shape is not ruleView: %T", compact[0])
	}
	if cv.Summary != ruleSummary {
		t.Errorf("listRules full=false: Summary = %q, want %q", cv.Summary, ruleSummary)
	}

	rec.reset()
	full, _, err := d.listRules(ctx, c, listRulesArgs{Scopes: []string{scope}, Full: true})
	if err != nil {
		t.Fatalf("listRules full=true: %v", err)
	}
	if !noneExclude(rec.snapshot()) {
		t.Errorf("listRules full=true: recorded Scroll selector(s) excluded content/citations — want the full view")
	}
	if len(full) != 1 {
		t.Fatalf("listRules full=true: got %d rules, want 1", len(full))
	}
	fm, ok := full[0].(store.Memory)
	if !ok {
		t.Fatalf("listRules full=true: full shape is not store.Memory: %T", full[0])
	}
	if fm.Content != ruleContent {
		t.Errorf("full rule shape lost its content: Content = %q, want %q (the projection flag must have reached the store)", fm.Content, ruleContent)
	}
}

// mustMemoriesField extracts the "memories" array from a CallToolResult's
// StructuredContent (a JSON-decoded map[string]any once round-tripped
// through the in-memory MCP transport) as a slice of maps.
func mustMemoriesField(t *testing.T, structured any) []map[string]any {
	t.Helper()
	m, ok := structured.(map[string]any)
	if !ok {
		t.Fatalf("StructuredContent is %T, want map[string]any: %+v", structured, structured)
	}
	raw, ok := m["memories"].([]any)
	if !ok {
		t.Fatalf("StructuredContent[\"memories\"] is %T, want []any: %+v", m["memories"], m)
	}
	out := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		rm, ok := r.(map[string]any)
		if !ok {
			t.Fatalf("memories entry is %T, want map[string]any: %+v", r, r)
		}
		out = append(out, rm)
	}
	return out
}

// findByField returns the first entry in mems whose named field equals want,
// failing the test if none matches.
func findByField(t *testing.T, mems []map[string]any, field string, want any) map[string]any {
	t.Helper()
	for _, m := range mems {
		if m[field] == want {
			return m
		}
	}
	t.Fatalf("no memory entry with %s = %v found among %+v", field, want, mems)
	return nil
}

func findProtoByID(t *testing.T, ms []*engramv1.Memory, id string) *engramv1.Memory {
	t.Helper()
	for _, m := range ms {
		if m.GetId() == id {
			return m
		}
	}
	t.Fatalf("no proto memory with id %q found among %d entries", id, len(ms))
	return nil
}

// findProtoBySummary returns the first proto memory whose Summary equals
// want, failing the test if none matches.
func findProtoBySummary(t *testing.T, ms []*engramv1.Memory, want string) *engramv1.Memory {
	t.Helper()
	for _, m := range ms {
		if m.GetSummary() == want {
			return m
		}
	}
	t.Fatalf("no proto memory with summary %q found among %d entries", want, len(ms))
	return nil
}
