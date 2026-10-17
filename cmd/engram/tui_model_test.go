// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
)

// consoleHarness drives the console model against a stub server: it sends
// messages to the model and runs every command the model returns, feeding
// each result back, so a test sees what a session would.
type consoleHarness struct {
	t *testing.T
	m *consoleModel
}

func newConsoleHarness(t *testing.T, svc *stubEngramService, scopes ...string) *consoleHarness {
	t.Helper()
	svc.readOnly = t
	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, startStubServer(t, svc))
	resp := &engramv1.ListScopesResponse{}
	for _, s := range scopes {
		resp.Scopes = append(resp.Scopes, &engramv1.ScopeCount{Scope: s, Count: 1})
	}
	h := &consoleHarness{t: t, m: newConsoleModel(context.Background(), client, 5*time.Second, resp)}
	h.send(tea.WindowSizeMsg{Width: 200, Height: 40})
	h.run(h.m.Init())
	return h
}

func (h *consoleHarness) update(msg tea.Msg) tea.Cmd {
	_, cmd := h.m.Update(msg)
	return cmd
}

func (h *consoleHarness) send(msg tea.Msg) { h.run(h.update(msg)) }

func (h *consoleHarness) run(cmd tea.Cmd) {
	for _, msg := range runConsoleCmd(cmd) {
		h.send(msg)
	}
}

// runConsoleCmd runs cmd and returns the messages it produced, with
// batches flattened.
func runConsoleCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	switch msg := cmd().(type) {
	case nil:
		return nil
	case tea.BatchMsg:
		var out []tea.Msg
		for _, c := range msg {
			out = append(out, runConsoleCmd(c)...)
		}
		return out
	default:
		return []tea.Msg{msg}
	}
}

func consoleKey(k string) tea.KeyPressMsg {
	switch k {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
}

func (h *consoleHarness) press(keys ...string) {
	for _, k := range keys {
		h.send(consoleKey(k))
	}
}

func (h *consoleHarness) typeText(s string) {
	for _, r := range s {
		h.send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

var ansiCSI = regexp.MustCompile("\x1b\\[[0-9;:?]*[ -/]*[@-~]")

func (h *consoleHarness) view() string {
	return ansiCSI.ReplaceAllString(h.m.View().Content, "")
}

func (h *consoleHarness) wantView(substrs ...string) {
	h.t.Helper()
	v := h.view()
	for _, s := range substrs {
		if !strings.Contains(v, s) {
			h.t.Errorf("view does not contain %q:\n%s", s, v)
		}
	}
}

// wantField checks a detail-view line: the field name, then its value.
func (h *consoleHarness) wantField(name, value string) {
	h.t.Helper()
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `:\s+` + regexp.QuoteMeta(value) + `\s*$`)
	if v := h.view(); !re.MatchString(v) {
		h.t.Errorf("view has no %s: %s line:\n%s", name, value, v)
	}
}

// recorder keeps every request one RPC received.
type recorder[Req any] struct {
	mu   sync.Mutex
	reqs []*Req
}

func (r *recorder[Req]) add(req *Req) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reqs = append(r.reqs, req)
}

func (r *recorder[Req]) all() []*Req {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*Req(nil), r.reqs...)
}

func (r *recorder[Req]) last(t *testing.T) *Req {
	t.Helper()
	all := r.all()
	if len(all) == 0 {
		t.Fatal("no request received")
	}
	return all[len(all)-1]
}

func stringPtr(s string) *string { return &s }

func testMemory(shortID, summary string) *engramv1.Memory {
	return &engramv1.Memory{
		Id: "id-" + shortID, ShortId: shortID, Summary: summary, Content: summary + " in full",
		Category: "decision", Scope: "repo:example/one", Visibility: "private", Owner: "me@example.com",
	}
}

func listOf(mems ...*engramv1.Memory) func(context.Context, *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
	return func(context.Context, *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
		return &engramv1.ListMemoriesResponse{Memories: mems}, nil
	}
}

func TestTUIQuitKeys(t *testing.T) {
	m := newConsoleModel(context.Background(), nil, time.Second, &engramv1.ListScopesResponse{})
	for _, k := range []string{"q", "ctrl+c"} {
		_, cmd := m.Update(consoleKey(k))
		if cmd == nil {
			t.Fatalf("%s returned no command", k)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%s did not return the quit command", k)
		}
	}
	// While typing a search, q is text and Ctrl-C still quits.
	m.input.Focus()
	if _, cmd := m.Update(consoleKey("q")); cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Error("q quit while typing a search")
		}
	}
	if _, cmd := m.Update(consoleKey("ctrl+c")); cmd == nil {
		t.Error("ctrl+c did not quit while typing a search")
	} else if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl+c did not quit while typing a search")
	}
}

func TestTUIListScopeAndPaging(t *testing.T) {
	var lists recorder[engramv1.ListMemoriesRequest]
	svc := &stubEngramService{
		listFn: func(_ context.Context, req *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			lists.add(req)
			if req.GetPageToken() == "page-2" {
				return &engramv1.ListMemoriesResponse{Memories: []*engramv1.Memory{testMemory("bbbbbbbbb2", "second page")}}, nil
			}
			return &engramv1.ListMemoriesResponse{
				Memories:      []*engramv1.Memory{testMemory("aaaaaaaaa1", "first page")},
				NextPageToken: "page-2",
			}, nil
		},
	}
	h := newConsoleHarness(t, svc, "repo:example/one", "repo:example/two")

	first := lists.last(t)
	if !first.GetCrossSpine() || first.GetScope() != "" || !first.GetCursorMode() || first.GetPageToken() != "" {
		t.Errorf("first page = %v, want cross_spine and cursor_mode with no scope or token", first)
	}
	if l := first.GetLimit(); l == 0 || l >= 1000 {
		t.Errorf("page size = %d, want well under 1000", l)
	}

	h.press("n")
	next := lists.last(t)
	if next.GetPageToken() != "page-2" || next.GetCursorMode() {
		t.Errorf("next page = %v, want page_token page-2 without cursor_mode", next)
	}
	h.wantView("first page", "second page")

	// Filters: three include toggles, every readable scope, then each scope.
	h.press("f", "j", "j", "j", "j", "j", "enter")
	scoped := lists.last(t)
	if scoped.GetScope() != "repo:example/two" || scoped.GetCrossSpine() || !scoped.GetCursorMode() {
		t.Errorf("after choosing a scope = %v, want scope repo:example/two on a fresh first page", scoped)
	}
	h.wantView("scope: repo:example/two")
}

func TestTUISearchRunsOnEnter(t *testing.T) {
	var searches recorder[engramv1.SearchMemoriesRequest]
	svc := &stubEngramService{
		listFn: listOf(),
		searchFn: func(_ context.Context, req *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			searches.add(req)
			return &engramv1.SearchMemoriesResponse{Memories: []*engramv1.Memory{testMemory("ccccccccc3", "a hit")}}, nil
		},
	}
	h := newConsoleHarness(t, svc, "repo:example/one")

	h.press("/")
	h.typeText("deploy notes")
	if n := len(searches.all()); n != 0 {
		t.Fatalf("typing sent %d searches, want 0", n)
	}
	h.press("enter")
	all := searches.all()
	if len(all) != 1 {
		t.Fatalf("enter sent %d searches, want 1", len(all))
	}
	if all[0].GetQuery() != "deploy notes" || !all[0].GetCrossSpine() || all[0].GetScope() != "" {
		t.Errorf("search = %v, want query \"deploy notes\" across every readable scope", all[0])
	}
	h.wantView(`1 hits for "deploy notes"`, "a hit")

	h.press("f", "j", "j", "j", "j", "enter")
	if last := searches.last(t); last.GetScope() != "repo:example/one" || last.GetCrossSpine() || last.GetQuery() != "deploy notes" {
		t.Errorf("search after choosing a scope = %v, want the same query in repo:example/one", last)
	}
}

func TestTUIIncludeTogglesAndState(t *testing.T) {
	var lists recorder[engramv1.ListMemoriesRequest]
	both := testMemory("ddddddddd4", "archived and superseded")
	both.ArchivedAt = timestamppb.Now()
	both.SupersededBy = stringPtr("id-eeeeeeeee5")
	svc := &stubEngramService{
		listFn: func(_ context.Context, req *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			lists.add(req)
			return &engramv1.ListMemoriesResponse{
				Memories:         []*engramv1.Memory{both},
				SearchedScopes:   []string{"repo:example/one", "repo:example/two"},
				ScopesTruncated:  true,
				RecallGateHidden: &engramv1.RecallGateHidden{Total: 4, Archived: 1, Superseded: 2, Expired: 3, Scheduled: 4},
			}, nil
		},
	}
	h := newConsoleHarness(t, svc, "repo:example/one")

	def := lists.last(t)
	if def.GetIncludeArchived() || def.GetIncludeSuperseded() || def.GetIncludeScheduled() {
		t.Errorf("default request = %v, want every include option off", def)
	}
	h.wantView("archived superseded",
		"recall_gate_hidden: 4  archived: 1  superseded: 2  expired: 3  scheduled: 4",
		"searched_scopes: 2  scopes_truncated: true")

	h.press("f", "space", "esc")
	got := lists.last(t)
	if !got.GetIncludeArchived() || got.GetIncludeSuperseded() || got.GetIncludeScheduled() {
		t.Errorf("after the archived toggle = %v, want only include_archived", got)
	}
	h.wantView("include: archived")
}

func TestTUIDetail(t *testing.T) {
	superseded := testMemory("fffffffff6", "an old decision")
	superseded.SupersededBy = stringPtr("id-ggggggggg7")
	shared := testMemory("hhhhhhhhh8", "a shared fact")
	shared.Visibility, shared.Owner = "shared", "other@example.com"
	records := map[string]*engramv1.Memory{superseded.GetId(): superseded, shared.GetId(): shared}
	svc := &stubEngramService{
		listFn: listOf(superseded, shared),
		getFn: func(_ context.Context, req *engramv1.GetMemoryRequest) (*engramv1.GetMemoryResponse, error) {
			if r, ok := records[req.GetId()]; ok {
				return &engramv1.GetMemoryResponse{Memory: r}, nil
			}
			return nil, connect.NewError(connect.CodeNotFound, errors.New("memory not found"))
		},
	}
	h := newConsoleHarness(t, svc)

	h.wantView("shared · other@example.com")
	h.press("enter")
	h.wantField("superseded_by", "id-ggggggggg7")
	h.wantField("state", "superseded")
	h.wantView("an old decision in full")

	h.press("esc", "j", "enter")
	h.wantField("visibility", "shared")
	h.wantField("owner", "other@example.com")

	// An unknown id and another actor's private id both read not found
	// from the server, and the console words them the same.
	for _, id := range []string{"zzzzzzzzz9", "00000000-0000-4000-8000-000000000000"} {
		h.press("/")
		h.typeText(id)
		h.press("enter")
		h.wantView(consoleNotFound)
	}
}

func TestTUIRelatedView(t *testing.T) {
	var related recorder[engramv1.RelatedMemoriesRequest]
	anchor := testMemory("jjjjjjjjj1", "the anchor")
	svc := &stubEngramService{
		listFn: listOf(anchor),
		relatedFn: func(_ context.Context, req *engramv1.RelatedMemoriesRequest) (*engramv1.RelatedMemoriesResponse, error) {
			related.add(req)
			return &engramv1.RelatedMemoriesResponse{Anchor: anchor, Related: []*engramv1.RelatedMemory{{
				Memory: testMemory("kkkkkkkkk2", "a neighbour"),
				Edges: []*engramv1.RelatedEdge{
					{Type: engramv1.EdgeType_EDGE_TYPE_SUPERSESSION},
					{Type: engramv1.EdgeType_EDGE_TYPE_TAG},
				},
			}}}, nil
		},
	}
	h := newConsoleHarness(t, svc)
	h.press("r")
	if got := related.last(t).GetId(); got != anchor.GetId() {
		t.Errorf("RelatedMemories id = %q, want %q", got, anchor.GetId())
	}
	h.wantView("1 related to jjjjjjjjj1", "supersession, shared tag", "a neighbour")
}

func TestTUITagsView(t *testing.T) {
	var tags recorder[engramv1.ListTagsRequest]
	svc := &stubEngramService{
		listFn: listOf(),
		listTagsFn: func(_ context.Context, req *engramv1.ListTagsRequest) (*engramv1.ListTagsResponse, error) {
			tags.add(req)
			return &engramv1.ListTagsResponse{Tags: []*engramv1.TagCount{{Tag: "deploy", Count: 7}}, More: true}, nil
		},
	}
	h := newConsoleHarness(t, svc)
	h.press("2")
	if req := tags.last(t); req.GetScope() != "" || req.GetLimit() == 0 {
		t.Errorf("ListTags = %v, want every readable scope with a limit", req)
	}
	h.wantView("      7  deploy", "more: true")
}

func TestTUIRulesView(t *testing.T) {
	var rules recorder[engramv1.ListRulesRequest]
	svc := &stubEngramService{
		listFn: listOf(),
		listRulesFn: func(_ context.Context, req *engramv1.ListRulesRequest) (*engramv1.ListRulesResponse, error) {
			rules.add(req)
			rule := testMemory("mmmmmmmmm3", "never push to main")
			rule.Category, rule.Scope = "rule", "rule:repo:example/one"
			return &engramv1.ListRulesResponse{Rules: []*engramv1.Memory{rule}, SearchedScopes: []string{"rule:repo:example/one"}}, nil
		},
	}
	h := newConsoleHarness(t, svc)
	h.press("3")
	if req := rules.last(t); len(req.GetScopes()) != 0 {
		t.Errorf("ListRules scopes = %v, want every readable rule scope", req.GetScopes())
	}
	h.wantView("never push to main", "rule:repo:example/one", "searched_scopes: 1")
}

func TestTUIScheduledView(t *testing.T) {
	var scheduled recorder[engramv1.ListScheduledRequest]
	svc := &stubEngramService{
		listFn: listOf(),
		listScheduledFn: func(_ context.Context, req *engramv1.ListScheduledRequest) (*engramv1.ListScheduledResponse, error) {
			scheduled.add(req)
			r := testMemory("nnnnnnnnn4", req.GetState()+" record")
			if req.GetState() == "expired" {
				r.NotAfter = timestamppb.New(time.Now().Add(-time.Hour))
			} else {
				r.NotBefore = timestamppb.New(time.Now().Add(time.Hour))
			}
			return &engramv1.ListScheduledResponse{Memories: []*engramv1.Memory{r}}, nil
		},
	}
	h := newConsoleHarness(t, svc)
	h.press("4")
	if req := scheduled.last(t); req.GetState() != "scheduled" || !req.GetCrossSpine() {
		t.Errorf("ListScheduled = %v, want state scheduled across every readable scope", req)
	}
	h.wantView("scheduled record")
	h.press("tab")
	if req := scheduled.last(t); req.GetState() != "expired" {
		t.Errorf("ListScheduled state after tab = %q, want expired", req.GetState())
	}
	h.wantView("expired record", "1 expired records")
}

func TestTUIRequestErrorStaysInSession(t *testing.T) {
	const envelope = "field=response hint=response_too_large: the result is too large to return in one response; retry with a smaller limit or k, or omit full"
	var tags recorder[engramv1.ListTagsRequest]
	svc := &stubEngramService{
		listFn: func(context.Context, *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			return nil, connect.NewError(connect.CodeResourceExhausted, errors.New(envelope))
		},
		listTagsFn: func(_ context.Context, req *engramv1.ListTagsRequest) (*engramv1.ListTagsResponse, error) {
			tags.add(req)
			return &engramv1.ListTagsResponse{}, nil
		},
	}
	h := newConsoleHarness(t, svc)
	h.wantView(envelope)
	h.press("2")
	if len(tags.all()) != 1 {
		t.Error("the key press after the error was not handled")
	}
}

func TestTUIDropsStaleResults(t *testing.T) {
	svc := &stubEngramService{
		listFn: listOf(),
		searchFn: func(_ context.Context, req *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{Memories: []*engramv1.Memory{testMemory("ppppppppp5", "result for "+req.GetQuery())}}, nil
		},
	}
	h := newConsoleHarness(t, svc)

	search := func(q string) tea.Cmd {
		h.press("/")
		h.m.input.SetValue(q)
		return h.update(consoleKey("enter"))
	}
	older, newer := search("older"), search("newer")
	for _, msg := range runConsoleCmd(newer) {
		h.send(msg)
	}
	for _, msg := range runConsoleCmd(older) {
		h.send(msg)
	}
	h.wantView("result for newer")
	if strings.Contains(h.view(), "result for older") {
		t.Error("an older search result replaced a newer one")
	}
}

// TestTUIReadOnlySession runs a session through every view to quit. The
// harness fails the test on any write RPC the server receives.
func TestTUIReadOnlySession(t *testing.T) {
	rec := testMemory("qqqqqqqqq6", "a record")
	svc := &stubEngramService{
		listFn: listOf(rec),
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{Memories: []*engramv1.Memory{rec}}, nil
		},
		getFn: func(context.Context, *engramv1.GetMemoryRequest) (*engramv1.GetMemoryResponse, error) {
			return &engramv1.GetMemoryResponse{Memory: rec}, nil
		},
		relatedFn: func(context.Context, *engramv1.RelatedMemoriesRequest) (*engramv1.RelatedMemoriesResponse, error) {
			return &engramv1.RelatedMemoriesResponse{Anchor: rec}, nil
		},
		listTagsFn: func(context.Context, *engramv1.ListTagsRequest) (*engramv1.ListTagsResponse, error) {
			return &engramv1.ListTagsResponse{}, nil
		},
		listRulesFn: func(context.Context, *engramv1.ListRulesRequest) (*engramv1.ListRulesResponse, error) {
			return &engramv1.ListRulesResponse{}, nil
		},
		listScheduledFn: func(context.Context, *engramv1.ListScheduledRequest) (*engramv1.ListScheduledResponse, error) {
			return &engramv1.ListScheduledResponse{}, nil
		},
	}
	h := newConsoleHarness(t, svc, "repo:example/one")
	h.press("/")
	h.typeText("anything")
	h.press("enter", "enter", "r", "esc", "esc", "?", "?",
		"f", "space", "j", "space", "j", "space", "esc",
		"2", "3", "4", "tab", "1", "n", "j", "k")
	if cmd := h.update(consoleKey("q")); cmd == nil {
		t.Fatal("q returned no command")
	} else if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q did not quit")
	}
	if h.m.statusErr {
		t.Errorf("session ended with an error: %s", h.m.status)
	}
}

// TestTUIServerTextCannotEscape: text from the server (here another
// actor's shared record, a scope name, a tag and an error) never reaches
// the terminal with a control sequence of its own.
func TestTUIServerTextCannotEscape(t *testing.T) {
	const osc52 = "\x1b]52;c;cHduZWQ=\x07"
	const c1CSI = "\u009b2J"
	hostile := "x" + osc52 + c1CSI + "\r"
	rec := testMemory("rrrrrrrrr7", "summary "+hostile)
	rec.Content = "line one " + hostile + "\nline two"
	rec.Visibility, rec.Owner, rec.Scope = "shared", "owner "+hostile, "repo:"+hostile
	rec.Tags = []string{"tag " + hostile}
	svc := &stubEngramService{
		listFn: listOf(rec),
		getFn: func(context.Context, *engramv1.GetMemoryRequest) (*engramv1.GetMemoryResponse, error) {
			return &engramv1.GetMemoryResponse{Memory: rec}, nil
		},
		listTagsFn: func(context.Context, *engramv1.ListTagsRequest) (*engramv1.ListTagsResponse, error) {
			return nil, connect.NewError(connect.CodeInternal, errors.New("boom "+hostile))
		},
	}
	h := newConsoleHarness(t, svc, "repo:"+hostile)
	check := func(where string) {
		t.Helper()
		v := h.m.View().Content
		for _, bad := range []string{"\x1b]", "\x07", "\u009b", "\r"} {
			if strings.Contains(v, bad) {
				t.Errorf("%s: view carries %q from server text", where, bad)
			}
		}
	}
	check("list")
	h.press("f")
	check("filters")
	h.press("esc", "enter")
	check("detail")
	h.wantView("line two")
	h.press("esc", "2")
	check("error status")
}
