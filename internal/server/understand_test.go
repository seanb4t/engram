// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/decide"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/understand"
)

// understandWireQuestion and understandWireRequest are the minimal subset
// of jev's wire request shape this file needs to decode: the type
// discriminates how the fixture server answers, and Criteria carries a
// choice question's option map for the "none plus 0.01 per other option"
// answering scheme.
type understandWireQuestion struct {
	Type     string          `json:"type"`
	Criteria json.RawMessage `json:"criteria"`
}

type understandWireRequest struct {
	Model     string                            `json:"model"`
	State     map[string]any                    `json:"state"`
	Questions map[string]understandWireQuestion `json:"questions"`
}

// newUnderstandTracerServer builds an httptest decisions server that decodes
// every request, records it, and answers EVERY asked question in Jev's wire
// shape: category_decision noul 0.95, every other noul 0.10, and any choice
// question {"choice":"none","probabilities":{"none":0.97, <other>:0.01}} —
// so the test stays valid once plan 06-02 adds the time-window and scope
// questions. Returns the server and accessors for a thread-safe snapshot of
// the requests it has seen so far.
func newUnderstandTracerServer(t *testing.T) (srv *httptest.Server, count func() int) {
	t.Helper()
	var mu sync.Mutex
	var reqs []understandWireRequest

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body understandWireRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		reqs = append(reqs, body)
		mu.Unlock()

		answers := make(map[string]any, len(body.Questions))
		for name, q := range body.Questions {
			switch q.Type {
			case "noul":
				v := 0.10
				if name == "category_decision" {
					v = 0.95
				}
				answers[name] = map[string]any{"type": "noul", "noul": v}
			case "choice":
				var options map[string]string
				_ = json.Unmarshal(q.Criteria, &options)
				probs := map[string]float64{"none": 0.97}
				for opt := range options {
					if opt == "none" {
						continue
					}
					probs[opt] = 0.01
				}
				answers[name] = map[string]any{"type": "choice", "choice": "none", "probabilities": probs}
			}
		}
		resp := map[string]any{
			"model":    "typesafe/jev-1.13-20260917",
			"answers":  answers,
			"usage":    map[string]any{"input_tokens": 100, "output_tokens": 10, "cost": 0.00001},
			"id":       "gen-dec-test-001",
			"provider": "TypeSafe",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))

	count = func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(reqs)
	}
	// lastState/lastQuestions read the most recent recorded request under
	// lock, for the "on" subtest's shape assertions.
	return srv, count
}

// understandTestClient mounts d over a fresh httptest Connect server and
// returns an engramv1connect client against it.
func understandTestClient(t *testing.T, d *deps) engramv1connect.EngramServiceClient {
	t.Helper()
	mux := http.NewServeMux()
	if err := d.mountConnect(mux, csrfStubResolve, csrfTestVerify, nil); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
}

// TestUnderstandQueryTracer proves the UnderstandQuery RPC end to end
// (Task 2): with understanding on, a real Jev client reaches a decisions
// server through the production resolver and one committed prose query
// yields exactly one DECIDED category suggestion; with understanding off,
// nothing is called; unauthenticated and oversized requests are rejected
// before any decision call.
func TestUnderstandQueryTracer(t *testing.T) {
	srv, reqCount := newUnderstandTracerServer(t)
	defer srv.Close()

	onConfig := func() *config.Config {
		cfg := &config.Config{}
		cfg.Decisions.Provider = "jev"
		cfg.Decisions.BaseURL = srv.URL + "/api"
		cfg.Decisions.APIKey = "k"
		return cfg
	}

	t.Run("on", func(t *testing.T) {
		before := reqCount()

		cfg := onConfig()
		dec, err := understandDecider(cfg)
		if err != nil {
			t.Fatalf("understandDecider: %v", err)
		}
		if dec == nil {
			t.Fatal("understandDecider returned nil for provider=jev, understanding unset")
		}

		d, sp := newSpyDeps()
		d.understandDec = dec
		client := understandTestClient(t, d)

		req := connect.NewRequest(&engramv1.UnderstandQueryRequest{Query: "what did we decide about tags"})
		req.Header().Set("X-Test-Actor", "actor-A")
		resp, err := client.UnderstandQuery(context.Background(), req)
		if err != nil {
			t.Fatalf("UnderstandQuery: %v", err)
		}
		if !resp.Msg.GetEnabled() {
			t.Fatal("Enabled = false, want true")
		}
		if got := len(resp.Msg.GetSuggestions()); got != 1 {
			t.Fatalf("len(Suggestions) = %d, want 1: %v", got, resp.Msg.GetSuggestions())
		}
		sug := resp.Msg.GetSuggestions()[0]
		if sug.GetCategory() != "decision" {
			t.Errorf("Category = %q, want %q", sug.GetCategory(), "decision")
		}
		if sug.GetSource() != engramv1.SuggestionSource_SUGGESTION_SOURCE_DECIDED {
			t.Errorf("Source = %v, want SUGGESTION_SOURCE_DECIDED", sug.GetSource())
		}

		after := reqCount()
		if after-before != 1 {
			t.Fatalf("decisions server saw %d new requests, want exactly 1", after-before)
		}
		// D-08/D-09 (plan 06-02): understandQuery reads the caller's own
		// readable scopes for the scope Choice's options (no scope applied)
		// and its own tag vocabulary for local matching — two store calls,
		// not "no store call at all" (06-01's original assertion,
		// superseded by this behavior).
		callLog := sp.callLog()
		if len(callLog) != 2 || callLog[0].Method != "ListScopes" || callLog[1].Method != "ListTags" {
			t.Errorf("spy store call log = %v, want exactly [ListScopes, ListTags]", callLog)
		}
	})

	t.Run("off", func(t *testing.T) {
		before := reqCount()

		cfg := onConfig()
		cfg.Search.Understanding = "off"
		dec, err := understandDecider(cfg)
		if err != nil {
			t.Fatalf("understandDecider: %v", err)
		}
		if dec != nil {
			t.Fatal("understandDecider returned non-nil for understanding=off")
		}

		d, sp := newSpyDeps()
		d.understandDec = dec
		client := understandTestClient(t, d)

		req := connect.NewRequest(&engramv1.UnderstandQueryRequest{Query: "what did we decide about tags"})
		req.Header().Set("X-Test-Actor", "actor-A")
		resp, err := client.UnderstandQuery(context.Background(), req)
		if err != nil {
			t.Fatalf("UnderstandQuery: %v", err)
		}
		if resp.Msg.GetEnabled() {
			t.Error("Enabled = true, want false")
		}
		if got := len(resp.Msg.GetSuggestions()); got != 0 {
			t.Errorf("len(Suggestions) = %d, want 0", got)
		}

		after := reqCount()
		if after != before {
			t.Errorf("decisions server saw %d new requests, want 0", after-before)
		}
		if len(sp.callLog()) != 0 {
			t.Errorf("spy store call log = %v, want empty", sp.callLog())
		}
	})

	t.Run("unauthenticated", func(t *testing.T) {
		d, _ := newSpyDeps()
		client := understandTestClient(t, d)

		req := connect.NewRequest(&engramv1.UnderstandQueryRequest{Query: "two words"})
		// no X-Test-Actor header
		_, err := client.UnderstandQuery(context.Background(), req)
		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Errorf("code = %v, want CodeUnauthenticated: %v", connect.CodeOf(err), err)
		}
	})

	t.Run("oversized", func(t *testing.T) {
		before := reqCount()

		cfg := onConfig()
		dec, err := understandDecider(cfg)
		if err != nil {
			t.Fatalf("understandDecider: %v", err)
		}
		d, _ := newSpyDeps()
		d.understandDec = dec
		client := understandTestClient(t, d)

		tags1001 := make([]string, 1001)
		for i := range tags1001 {
			tags1001[i] = fmt.Sprintf("t%d", i)
		}
		req := connect.NewRequest(&engramv1.UnderstandQueryRequest{Query: "irrelevant", Tags: tags1001})
		req.Header().Set("X-Test-Actor", "actor-A")
		_, err = client.UnderstandQuery(context.Background(), req)
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("code = %v, want CodeInvalidArgument: %v", connect.CodeOf(err), err)
		}
		var ce *connect.Error
		if !errors.As(err, &ce) {
			t.Fatalf("err is not a *connect.Error: %v", err)
		}
		if !strings.HasPrefix(ce.Message(), "field=tags hint=out_of_range") {
			t.Errorf("message = %q, want prefix %q", ce.Message(), "field=tags hint=out_of_range")
		}

		after := reqCount()
		if after != before {
			t.Errorf("decisions server saw a request for the oversized call, want none")
		}

		req2 := connect.NewRequest(&engramv1.UnderstandQueryRequest{Query: "what did we decide about tags", Tags: tags1001[:1000]})
		req2.Header().Set("X-Test-Actor", "actor-A")
		if _, err := client.UnderstandQuery(context.Background(), req2); err != nil {
			t.Errorf("1000 tags: %v, want success", err)
		}
	})
}

// scriptedDecider is a fake decide.Decider whose Decide/DecideMany calls
// are mutex-guarded counters, returning a scripted (resp, err) pair and
// recording the last Request handed to Decide — used both as d.decider
// (the consolidate client) in TestUnderstandQueryOffNeverDecides to prove
// UnderstandQuery never reaches it when d.understandDec is nil, and as
// d.understandDec in tests that need to script a whole decide.Response and
// inspect the captured request's questions/options.
type scriptedDecider struct {
	mu              sync.Mutex
	decideCalls     int
	decideManyCalls int
	lastReq         decide.Request
	resp            decide.Response
	err             error
}

func (s *scriptedDecider) Decide(_ context.Context, req decide.Request) (decide.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.decideCalls++
	s.lastReq = req
	return s.resp, s.err
}

func (s *scriptedDecider) DecideMany(_ context.Context, reqs []decide.Request) []decide.Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.decideManyCalls++
	out := make([]decide.Result, len(reqs))
	for i := range reqs {
		out[i] = decide.Result{Response: s.resp, Err: s.err}
	}
	return out
}

func (s *scriptedDecider) counts() (decideCalls, decideManyCalls int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.decideCalls, s.decideManyCalls
}

// lastRequest returns the most recently captured Decide request, under
// lock.
func (s *scriptedDecider) lastRequest() decide.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastReq
}

// TestUnderstandQueryOffNeverDecides proves D-04's isolation from the
// consolidate client: a scriptedDecider set as d.decider (never
// d.understandDec) is never called by deps.understandQuery, and the spy
// store's call log stays empty.
func TestUnderstandQueryOffNeverDecides(t *testing.T) {
	d, sp := newSpyDeps()
	scripted := &scriptedDecider{}
	d.decider = scripted
	// d.understandDec deliberately left nil.

	res, err := d.understandQuery(context.Background(), caller{}, understandArgs{Query: "what did we decide"})
	if err != nil {
		t.Fatalf("understandQuery: %v", err)
	}
	if res.Enabled {
		t.Error("Enabled = true, want false")
	}
	decideCalls, decideManyCalls := scripted.counts()
	if decideCalls != 0 || decideManyCalls != 0 {
		t.Errorf("scriptedDecider saw %d Decide, %d DecideMany calls, want 0, 0", decideCalls, decideManyCalls)
	}
	if len(sp.callLog()) != 0 {
		t.Errorf("spy store call log = %v, want empty", sp.callLog())
	}
}

// understandSeedScope upserts one readable record for owner in scope,
// directly on the embedded spy — enough for spyStore.ListScopes (which
// derives its scope set from readable records) to surface it.
func understandSeedScope(t *testing.T, sp *spyStore, owner, scope, visibility string) {
	t.Helper()
	if err := sp.Upsert(context.Background(), store.Memory{
		ID:         uuid.NewString(),
		Content:    "x",
		Scope:      scope,
		Owner:      owner,
		Visibility: visibility,
		CreatedAt:  time.Now().UTC(),
	}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed %s/%s: %v", owner, scope, err)
	}
}

// newUnderstandScopeScriptedDecider returns a scriptedDecider that answers
// every category noul at 0.1 (never a category suggestion), the
// "time_window" choice with "none" (Task 2 also asks this question — this
// fixture stays scoped to proving the scope suggestion alone), and the
// "scope" choice with Choice "repo:a/x" at probability 0.93 (>= Threshold)
// — the fixed response TestUnderstandQueryScopeSuggestion's behavior block
// specifies.
func newUnderstandScopeScriptedDecider() *scriptedDecider {
	return &scriptedDecider{resp: decide.Response{Answers: map[string]decide.Answer{
		understand.CategoryQuestion("convention"): {Type: decide.QuestionNoul, Probability: 0.1},
		understand.CategoryQuestion("gotcha"):     {Type: decide.QuestionNoul, Probability: 0.1},
		understand.CategoryQuestion("decision"):   {Type: decide.QuestionNoul, Probability: 0.1},
		understand.CategoryQuestion("preference"): {Type: decide.QuestionNoul, Probability: 0.1},
		understand.QuestionTimeWindow: {
			Type:   decide.QuestionChoice,
			Choice: "none",
			Probabilities: map[string]float64{
				"none":       0.97,
				"today":      0.01,
				"past_week":  0.01,
				"past_month": 0.005,
				"past_year":  0.005,
			},
		},
		understand.QuestionScope: {
			Type:   decide.QuestionChoice,
			Choice: "repo:a/x",
			Probabilities: map[string]float64{
				"repo:a/x": 0.93,
				"none":     0.05,
				"repo:a/y": 0.02,
			},
		},
	}}}
}

// TestUnderstandQueryScopeSuggestion proves D-08 end to end (Task 1): the
// caller's own readable scopes (from ListScopes, caller's Subject passed
// straight through) become the scope Choice's options, another actor's
// private scope is never offered, and a DECIDED scope choice at or above
// Threshold becomes exactly one scope suggestion. With a scope already
// applied, no scope question is asked and ListScopes is not called.
func TestUnderstandQueryScopeSuggestion(t *testing.T) {
	t.Run("scope suggested from caller's own readable scopes", func(t *testing.T) {
		d, sp := newSpyDeps()
		understandSeedScope(t, sp, "actor-A", "repo:a/x", "")
		understandSeedScope(t, sp, "actor-A", "repo:a/y", "")
		understandSeedScope(t, sp, "actor-B", "repo:b/secret", "")

		dec := newUnderstandScopeScriptedDecider()
		d.understandDec = dec
		client := understandTestClient(t, d)

		req := connect.NewRequest(&engramv1.UnderstandQueryRequest{Query: "what did we record in the x repo"})
		req.Header().Set("X-Test-Actor", "actor-A")
		resp, err := client.UnderstandQuery(context.Background(), req)
		if err != nil {
			t.Fatalf("UnderstandQuery: %v", err)
		}

		var scopeSuggestions []*engramv1.FilterSuggestion
		for _, s := range resp.Msg.GetSuggestions() {
			if _, ok := s.GetKind().(*engramv1.FilterSuggestion_Scope); ok {
				scopeSuggestions = append(scopeSuggestions, s)
			}
		}
		if len(scopeSuggestions) != 1 {
			t.Fatalf("len(scope suggestions) = %d, want 1: %v", len(scopeSuggestions), resp.Msg.GetSuggestions())
		}
		if got := scopeSuggestions[0].GetScope(); got != "repo:a/x" {
			t.Errorf("scope = %q, want %q", got, "repo:a/x")
		}
		if got := scopeSuggestions[0].GetSource(); got != engramv1.SuggestionSource_SUGGESTION_SOURCE_DECIDED {
			t.Errorf("source = %v, want SUGGESTION_SOURCE_DECIDED", got)
		}

		lastReq := dec.lastRequest()
		q, ok := lastReq.Questions[understand.QuestionScope]
		if !ok {
			t.Fatal("scope question not asked")
		}
		if len(q.Options) != 3 {
			t.Fatalf("len(Options) = %d, want 3 (repo:a/x, repo:a/y, none): %v", len(q.Options), q.Options)
		}
		for _, want := range []string{"repo:a/x", "repo:a/y", "none"} {
			if _, ok := q.Options[want]; !ok {
				t.Errorf("Options missing %q: %v", want, q.Options)
			}
		}
		if _, ok := q.Options["repo:b/secret"]; ok {
			t.Error("Options contains repo:b/secret, want never offered (another actor's private scope)")
		}

		var listScopesCalls int
		for _, c := range sp.callLog() {
			if c.Method == "ListScopes" {
				listScopesCalls++
				if c.Owner != "actor-A" {
					t.Errorf("ListScopes owner = %q, want %q", c.Owner, "actor-A")
				}
			}
		}
		if listScopesCalls != 1 {
			t.Errorf("ListScopes called %d times, want 1", listScopesCalls)
		}
	})

	t.Run("scope already applied: no scope question, no ListScopes call", func(t *testing.T) {
		d, sp := newSpyDeps()
		understandSeedScope(t, sp, "actor-A", "repo:a/x", "")
		understandSeedScope(t, sp, "actor-A", "repo:a/y", "")

		dec := newUnderstandScopeScriptedDecider()
		d.understandDec = dec
		client := understandTestClient(t, d)

		req := connect.NewRequest(&engramv1.UnderstandQueryRequest{
			Query: "what did we record in the x repo",
			Scope: "repo:a/y",
		})
		req.Header().Set("X-Test-Actor", "actor-A")
		if _, err := client.UnderstandQuery(context.Background(), req); err != nil {
			t.Fatalf("UnderstandQuery: %v", err)
		}

		lastReq := dec.lastRequest()
		if _, ok := lastReq.Questions[understand.QuestionScope]; ok {
			t.Error("scope question asked, want none (scope already applied)")
		}
		for _, c := range sp.callLog() {
			if c.Method == "ListScopes" {
				t.Error("ListScopes called, want none (scope already applied)")
			}
		}
	})
}

// newUnderstandAllKindsDecider returns a scriptedDecider answering gotcha
// 0.95, decision 0.93, the other two categories at 0.1, time_window
// "past_week" at 0.94, and scope "repo:a/x" at 0.96 — the fixed response
// TestUnderstandQueryAllKinds' behavior block specifies.
func newUnderstandAllKindsDecider() *scriptedDecider {
	return &scriptedDecider{resp: decide.Response{Answers: map[string]decide.Answer{
		understand.CategoryQuestion("convention"): {Type: decide.QuestionNoul, Probability: 0.1},
		understand.CategoryQuestion("gotcha"):     {Type: decide.QuestionNoul, Probability: 0.95},
		understand.CategoryQuestion("decision"):   {Type: decide.QuestionNoul, Probability: 0.93},
		understand.CategoryQuestion("preference"): {Type: decide.QuestionNoul, Probability: 0.1},
		understand.QuestionTimeWindow: {
			Type:   decide.QuestionChoice,
			Choice: "past_week",
			Probabilities: map[string]float64{
				"past_week":  0.94,
				"none":       0.03,
				"today":      0.01,
				"past_month": 0.01,
				"past_year":  0.01,
			},
		},
		understand.QuestionScope: {
			Type:   decide.QuestionChoice,
			Choice: "repo:a/x",
			Probabilities: map[string]float64{
				"repo:a/x": 0.96,
				"none":     0.04,
			},
		},
	}}}
}

// TestUnderstandQueryAllKinds proves Task 2's combined end-to-end shape
// (D-05/D-07/D-08/D-09): one prose query yields every suggestion kind in
// order — category gotcha, category decision, time_window, scope, then the
// one matched tag — each with the right oneof case and source; the tag
// vocabulary is read via the shared listTags core scoped to the caller and
// the applied scope, and never reaches the decide request (the vocabulary
// tags never appear in any question instruction, criterion, or option).
func TestUnderstandQueryAllKinds(t *testing.T) {
	t.Run("no scope applied", func(t *testing.T) {
		d, sp := newSpyDeps()
		understandSeedScope(t, sp, "actor-A", "repo:a/x", "")
		sp.tags = []store.TagCount{{Tag: "qdrant-zeta", Count: 9}, {Tag: "omega-ci", Count: 3}}

		dec := newUnderstandAllKindsDecider()
		d.understandDec = dec
		client := understandTestClient(t, d)

		req := connect.NewRequest(&engramv1.UnderstandQueryRequest{Query: "why does qdrant break in repo x lately"})
		req.Header().Set("X-Test-Actor", "actor-A")
		resp, err := client.UnderstandQuery(context.Background(), req)
		if err != nil {
			t.Fatalf("UnderstandQuery: %v", err)
		}

		suggestions := resp.Msg.GetSuggestions()
		if len(suggestions) != 5 {
			t.Fatalf("len(Suggestions) = %d, want 5: %v", len(suggestions), suggestions)
		}

		if got := suggestions[0]; got.GetCategory() != "gotcha" || got.GetSource() != engramv1.SuggestionSource_SUGGESTION_SOURCE_DECIDED {
			t.Errorf("suggestions[0] = %+v, want category=gotcha source=DECIDED", got)
		}
		if got := suggestions[1]; got.GetCategory() != "decision" || got.GetSource() != engramv1.SuggestionSource_SUGGESTION_SOURCE_DECIDED {
			t.Errorf("suggestions[1] = %+v, want category=decision source=DECIDED", got)
		}

		tw := suggestions[2]
		twMsg := tw.GetTimeWindow()
		if twMsg == nil {
			t.Fatalf("suggestions[2] = %+v, want a time_window suggestion", tw)
		}
		if !strings.HasSuffix(twMsg.GetCreatedAfter(), "T00:00:00Z") || len(twMsg.GetCreatedAfter()) != len("2026-09-28T00:00:00Z") {
			t.Errorf("suggestions[2].created_after = %q, want a day-aligned RFC3339 midnight", twMsg.GetCreatedAfter())
		}
		if twMsg.GetCreatedBefore() != "" {
			t.Errorf("suggestions[2].created_before = %q, want empty", twMsg.GetCreatedBefore())
		}
		if twMsg.GetLabel() != "past week" {
			t.Errorf("suggestions[2].label = %q, want %q", twMsg.GetLabel(), "past week")
		}
		if tw.GetSource() != engramv1.SuggestionSource_SUGGESTION_SOURCE_DECIDED {
			t.Errorf("suggestions[2].Source = %v, want DECIDED", tw.GetSource())
		}

		if got := suggestions[3]; got.GetScope() != "repo:a/x" || got.GetSource() != engramv1.SuggestionSource_SUGGESTION_SOURCE_DECIDED {
			t.Errorf("suggestions[3] = %+v, want scope=repo:a/x source=DECIDED", got)
		}
		if got := suggestions[4]; got.GetTag() != "qdrant-zeta" || got.GetSource() != engramv1.SuggestionSource_SUGGESTION_SOURCE_MATCHED {
			t.Errorf("suggestions[4] = %+v, want tag=qdrant-zeta source=MATCHED", got)
		}

		var listTagsCalls int
		for _, c := range sp.callLog() {
			if c.Method == "ListTags" {
				listTagsCalls++
				if c.Owner != "actor-A" {
					t.Errorf("ListTags owner = %q, want %q", c.Owner, "actor-A")
				}
				if scope, _ := c.Args.(string); scope != "" {
					t.Errorf("ListTags scope = %q, want empty (no scope applied)", scope)
				}
			}
		}
		if listTagsCalls != 1 {
			t.Errorf("ListTags called %d times, want 1", listTagsCalls)
		}

		lastReq := dec.lastRequest()
		if len(lastReq.State) != 1 {
			t.Fatalf("len(State) = %d, want 1", len(lastReq.State))
		}
		if _, ok := lastReq.State["query"]; !ok {
			t.Error(`State["query"] missing`)
		}
		for _, forbidden := range []string{"qdrant-zeta", "omega-ci"} {
			for name, q := range lastReq.Questions {
				if strings.Contains(q.Instructions, forbidden) {
					t.Errorf("question %q instructions contain vocabulary tag %q", name, forbidden)
				}
				if strings.Contains(q.WhenTrue, forbidden) || strings.Contains(q.WhenFalse, forbidden) {
					t.Errorf("question %q criteria contain vocabulary tag %q", name, forbidden)
				}
				for opt, desc := range q.Options {
					if strings.Contains(opt, forbidden) || strings.Contains(desc, forbidden) {
						t.Errorf("question %q option %q contains vocabulary tag %q", name, opt, forbidden)
					}
				}
			}
		}
	})

	t.Run("scope applied: ListTags scoped to it", func(t *testing.T) {
		d, sp := newSpyDeps()
		understandSeedScope(t, sp, "actor-A", "repo:a/x", "")
		sp.tags = []store.TagCount{{Tag: "qdrant-zeta", Count: 9}}

		dec := newUnderstandAllKindsDecider()
		d.understandDec = dec
		client := understandTestClient(t, d)

		req := connect.NewRequest(&engramv1.UnderstandQueryRequest{
			Query: "why does qdrant break in repo x lately",
			Scope: "repo:a/x",
		})
		req.Header().Set("X-Test-Actor", "actor-A")
		if _, err := client.UnderstandQuery(context.Background(), req); err != nil {
			t.Fatalf("UnderstandQuery: %v", err)
		}

		var found bool
		for _, c := range sp.callLog() {
			if c.Method == "ListTags" {
				found = true
				if scope, _ := c.Args.(string); scope != "repo:a/x" {
					t.Errorf("ListTags scope = %q, want %q", scope, "repo:a/x")
				}
			}
		}
		if !found {
			t.Error("ListTags not called")
		}
	})
}
