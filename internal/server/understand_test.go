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
		// D-08 (plan 06-02): understandQuery reads the caller's own readable
		// scopes for the scope Choice's options when no scope is applied —
		// a single ListScopes call, not "no store call at all" (06-01's
		// original assertion, superseded by this behavior).
		callLog := sp.callLog()
		if len(callLog) != 1 || callLog[0].Method != "ListScopes" {
			t.Errorf("spy store call log = %v, want exactly one ListScopes call", callLog)
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
// every category noul at 0.1 (never a category suggestion) and the "scope"
// choice with Choice "repo:a/x" at probability 0.93 (>= Threshold) — the
// fixed response TestUnderstandQueryScopeSuggestion's behavior block
// specifies.
func newUnderstandScopeScriptedDecider() *scriptedDecider {
	return &scriptedDecider{resp: decide.Response{Answers: map[string]decide.Answer{
		understand.CategoryQuestion("convention"): {Type: decide.QuestionNoul, Probability: 0.1},
		understand.CategoryQuestion("gotcha"):     {Type: decide.QuestionNoul, Probability: 0.1},
		understand.CategoryQuestion("decision"):   {Type: decide.QuestionNoul, Probability: 0.1},
		understand.CategoryQuestion("preference"): {Type: decide.QuestionNoul, Probability: 0.1},
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
