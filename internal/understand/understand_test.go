// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package understand

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/seanb4t/engram/internal/decide"
)

// TestNewRequestCategories proves NewRequest's D-06 question-set shape: one
// Noul question per unanswered category, filtered by applied.Categories,
// each with non-empty instructions/criteria, a valid Request, and the query
// state truncated to MaxQueryChars runes.
func TestNewRequestCategories(t *testing.T) {
	t.Run("no applied categories asks all four", func(t *testing.T) {
		req := NewRequest("what did we decide", Applied{})
		var got []string
		for name := range req.Questions {
			if _, ok := req.Questions[name]; ok {
				got = append(got, name)
			}
		}
		if len(got) != 4 {
			t.Fatalf("len(Questions) = %d, want 4: %v", len(got), got)
		}
		for _, cat := range Categories {
			name := CategoryQuestion(cat)
			q, ok := req.Questions[name]
			if !ok {
				t.Errorf("missing question %q", name)
				continue
			}
			if q.Type != decide.QuestionNoul {
				t.Errorf("question %q type = %q, want noul", name, q.Type)
			}
			if q.Instructions == "" || q.WhenTrue == "" || q.WhenFalse == "" {
				t.Errorf("question %q has an empty instructions/whenTrue/whenFalse field", name)
			}
		}
		if err := req.Validate(); err != nil {
			t.Errorf("req.Validate() = %v, want nil", err)
		}
		if len(req.State) != 1 {
			t.Fatalf("len(State) = %d, want 1", len(req.State))
		}
		if _, ok := req.State["query"]; !ok {
			t.Error(`State["query"] missing`)
		}
	})

	t.Run("some applied categories filters the question set", func(t *testing.T) {
		req := NewRequest("what did we decide", Applied{Categories: []string{"decision", "gotcha"}})
		if _, ok := req.Questions[CategoryQuestion("decision")]; ok {
			t.Error("category_decision present, want filtered out (applied)")
		}
		if _, ok := req.Questions[CategoryQuestion("gotcha")]; ok {
			t.Error("category_gotcha present, want filtered out (applied)")
		}
		if _, ok := req.Questions[CategoryQuestion("convention")]; !ok {
			t.Error("category_convention missing, want present (not applied)")
		}
		if _, ok := req.Questions[CategoryQuestion("preference")]; !ok {
			t.Error("category_preference missing, want present (not applied)")
		}
	})

	t.Run("all four applied asks nothing", func(t *testing.T) {
		req := NewRequest("what did we decide", Applied{Categories: append([]string{}, Categories...)})
		if len(req.Questions) != 0 {
			t.Errorf("len(Questions) = %d, want 0", len(req.Questions))
		}
	})

	t.Run("query truncated to MaxQueryChars runes, valid UTF-8", func(t *testing.T) {
		// A 2100-rune query built from multi-byte runes (each "é" is 2
		// bytes), so a byte-based truncation would produce different
		// results than a rune-based one.
		long := ""
		for i := 0; i < 2100; i++ {
			long += "é"
		}
		req := NewRequest(long, Applied{})
		q, _ := req.State["query"].(string)
		if n := utf8.RuneCountInString(q); n != MaxQueryChars {
			t.Errorf("truncated query has %d runes, want %d", n, MaxQueryChars)
		}
		if !utf8.ValidString(q) {
			t.Error("truncated query is not valid UTF-8")
		}
	})
}

// TestFromResponseCategoryThreshold proves FromResponse's D-05 threshold
// boundary and malformed-response handling.
func TestFromResponseCategoryThreshold(t *testing.T) {
	req := NewRequest("what did we decide", Applied{})

	t.Run("threshold boundary", func(t *testing.T) {
		cases := []struct {
			name string
			p    float64
			want bool
		}{
			{"at threshold", 0.9, true},
			{"just below threshold", 0.8999, false},
			{"maximum", 1.0, true},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				resp := decide.Response{Answers: map[string]decide.Answer{
					CategoryQuestion("convention"): {Type: decide.QuestionNoul, Probability: 0.10},
					CategoryQuestion("gotcha"):     {Type: decide.QuestionNoul, Probability: 0.10},
					CategoryQuestion("decision"):   {Type: decide.QuestionNoul, Probability: tc.p},
					CategoryQuestion("preference"): {Type: decide.QuestionNoul, Probability: 0.10},
				}}
				got, err := FromResponse(resp, req)
				if err != nil {
					t.Fatalf("FromResponse: %v", err)
				}
				var found bool
				for _, s := range got {
					if s.Kind == KindCategory && s.Value == "decision" {
						found = true
					}
				}
				if found != tc.want {
					t.Errorf("decision suggested = %v, want %v (p=%v)", found, tc.want, tc.p)
				}
			})
		}
	})

	t.Run("malformed answers", func(t *testing.T) {
		cases := []struct {
			name   string
			answer *decide.Answer
		}{
			{"missing", nil},
			{"wrong type", &decide.Answer{Type: decide.QuestionChoice, Choice: "x"}},
			{"NaN", &decide.Answer{Type: decide.QuestionNoul, Probability: math.NaN()}},
			{"below zero", &decide.Answer{Type: decide.QuestionNoul, Probability: -0.1}},
			{"above one", &decide.Answer{Type: decide.QuestionNoul, Probability: 1.1}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				answers := map[string]decide.Answer{}
				if tc.answer != nil {
					answers[CategoryQuestion("decision")] = *tc.answer
				}
				resp := decide.Response{Answers: answers}
				got, err := FromResponse(resp, req)
				if err == nil {
					t.Fatal("FromResponse err = nil, want a malformed-response error")
				}
				if !errors.Is(err, decide.ErrDecisionMalformedResponse) {
					t.Errorf("err = %v, want errors.Is(err, decide.ErrDecisionMalformedResponse)", err)
				}
				if got != nil {
					t.Errorf("got = %v, want nil on error", got)
				}
			})
		}
	})

	t.Run("output order follows Categories", func(t *testing.T) {
		resp := decide.Response{Answers: map[string]decide.Answer{
			CategoryQuestion("convention"): {Type: decide.QuestionNoul, Probability: 0.95},
			CategoryQuestion("gotcha"):     {Type: decide.QuestionNoul, Probability: 0.10},
			CategoryQuestion("decision"):   {Type: decide.QuestionNoul, Probability: 0.95},
			CategoryQuestion("preference"): {Type: decide.QuestionNoul, Probability: 0.10},
		}}
		got, err := FromResponse(resp, req)
		if err != nil {
			t.Fatalf("FromResponse: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len(got) = %d, want 2: %v", len(got), got)
		}
		if got[0].Value != "convention" || got[1].Value != "decision" {
			t.Errorf("order = %v, want [convention, decision] (Categories order)", got)
		}
	})
}

// countingDecider is a fake decide.Decider that counts Decide/DecideMany
// calls and records the last Request handed to Decide, returning a fixed
// (resp, err) pair — mirrors internal/relevance's countingDecider precedent.
type countingDecider struct {
	decideCalls     int
	decideManyCalls int
	lastReq         decide.Request
	resp            decide.Response
	err             error
}

func (d *countingDecider) Decide(_ context.Context, req decide.Request) (decide.Response, error) {
	d.decideCalls++
	d.lastReq = req
	return d.resp, d.err
}

func (d *countingDecider) DecideMany(_ context.Context, reqs []decide.Request) []decide.Result {
	d.decideManyCalls++
	out := make([]decide.Result, len(reqs))
	for i := range reqs {
		out[i] = decide.Result{Response: d.resp, Err: d.err}
	}
	return out
}

// TestSuggestCategoryPaths proves Suggest's D-05/D-17 outcome shapes: a
// successful decision yields OutcomeDecided with exactly one Decide call; a
// decision error yields OutcomeFallback with its classified FallbackClass;
// every category already applied (nothing left to ask) yields
// OutcomeSkipped with zero Decide calls; a nil decider does the same.
func TestSuggestCategoryPaths(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		dec := &countingDecider{resp: decide.Response{Answers: map[string]decide.Answer{
			CategoryQuestion("decision"):   {Type: decide.QuestionNoul, Probability: 0.95},
			CategoryQuestion("convention"): {Type: decide.QuestionNoul, Probability: 0.10},
			CategoryQuestion("gotcha"):     {Type: decide.QuestionNoul, Probability: 0.10},
			CategoryQuestion("preference"): {Type: decide.QuestionNoul, Probability: 0.10},
		}}}
		res := Suggest(context.Background(), dec, Input{Query: "what did we decide", Now: time.Now()})
		if res.Outcome != OutcomeDecided {
			t.Errorf("Outcome = %q, want %q", res.Outcome, OutcomeDecided)
		}
		if dec.decideCalls != 1 {
			t.Errorf("decideCalls = %d, want 1", dec.decideCalls)
		}
		if dec.decideManyCalls != 0 {
			t.Errorf("decideManyCalls = %d, want 0", dec.decideManyCalls)
		}
		if len(res.Suggestions) != 1 || res.Suggestions[0].Value != "decision" {
			t.Errorf("Suggestions = %v, want exactly [decision]", res.Suggestions)
		}
	})

	t.Run("decision error falls back", func(t *testing.T) {
		dec := &countingDecider{err: &decide.Error{Kind: decide.ErrDecisionTimeout}}
		req := NewRequest("what did we decide", Applied{})
		res := Suggest(context.Background(), dec, Input{Query: "what did we decide", Now: time.Now()})
		if res.Outcome != OutcomeFallback {
			t.Errorf("Outcome = %q, want %q", res.Outcome, OutcomeFallback)
		}
		if res.FallbackClass != "timeout" {
			t.Errorf("FallbackClass = %q, want %q", res.FallbackClass, "timeout")
		}
		if len(res.Suggestions) != 0 {
			t.Errorf("Suggestions = %v, want empty", res.Suggestions)
		}
		if res.QuestionsAsked != len(req.Questions) {
			t.Errorf("QuestionsAsked = %d, want %d", res.QuestionsAsked, len(req.Questions))
		}
	})

	t.Run("all categories applied skips the call", func(t *testing.T) {
		dec := &countingDecider{}
		in := Input{
			Query: "what did we decide",
			Applied: Applied{
				Categories:   append([]string{}, Categories...),
				Scope:        "tool:project:x",
				CreatedAfter: "2026-01-01T00:00:00Z",
			},
			Now: time.Now(),
		}
		res := Suggest(context.Background(), dec, in)
		if res.Outcome != OutcomeSkipped {
			t.Errorf("Outcome = %q, want %q", res.Outcome, OutcomeSkipped)
		}
		if dec.decideCalls != 0 {
			t.Errorf("decideCalls = %d, want 0", dec.decideCalls)
		}
	})

	t.Run("nil decider skips the call", func(t *testing.T) {
		res := Suggest(context.Background(), nil, Input{Query: "what did we decide", Now: time.Now()})
		if res.Outcome != OutcomeSkipped {
			t.Errorf("Outcome = %q, want %q", res.Outcome, OutcomeSkipped)
		}
	})
}
