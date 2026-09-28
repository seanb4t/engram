// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package understand

import (
	"context"
	"errors"
	"fmt"
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
		// CreatedAfter applied so this stays scoped to the category question
		// set alone (Task 2 adds a time_window question when neither
		// CreatedAfter nor CreatedBefore is applied — TestNewRequestShape
		// covers that combined shape).
		req := NewRequest("what did we decide", Applied{CreatedAfter: "2026-01-01T00:00:00Z"}, nil)
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
		req := NewRequest("what did we decide", Applied{Categories: []string{"decision", "gotcha"}}, nil)
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
		// CreatedAfter applied too, so the time_window question (Task 2) is
		// also skipped — "nothing" means nothing, not "no categories".
		req := NewRequest("what did we decide", Applied{
			Categories:   append([]string{}, Categories...),
			CreatedAfter: "2026-01-01T00:00:00Z",
		}, nil)
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
		req := NewRequest(long, Applied{}, nil)
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
	// CreatedAfter applied so the Task 2 time_window question is not asked
	// — this test is scoped to category-answer handling alone, and every
	// scripted resp below carries only category answers.
	req := NewRequest("what did we decide", Applied{CreatedAfter: "2026-01-01T00:00:00Z"}, nil)

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
				got, err := FromResponse(resp, req, time.Now())
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
				got, err := FromResponse(resp, req, time.Now())
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
		got, err := FromResponse(resp, req, time.Now())
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
		// CreatedAfter applied so the Task 2 time_window question is not
		// asked — this scripted resp carries only category answers.
		res := Suggest(context.Background(), dec, Input{
			Query:   "what did we decide",
			Applied: Applied{CreatedAfter: "2026-01-01T00:00:00Z"},
			Now:     time.Now(),
		})
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
		req := NewRequest("what did we decide", Applied{}, nil)
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

// TestScopeQuestionGate proves NewRequest's D-08 scope-question gate: 0
// scopes asks nothing; the option cardinality boundary at MaxScopeOptions
// (254 asks, 255 does not); an already-applied scope skips the question
// regardless of scope count; a scope literally named "none" is dropped from
// the options (the gate counts scopes only, never ListScopes' scan-cap
// flag — RESEARCH Pitfall 5, which is not even in this function's
// signature).
func TestScopeQuestionGate(t *testing.T) {
	t.Run("0 scopes: no scope question", func(t *testing.T) {
		req := NewRequest("q", Applied{}, nil)
		if _, ok := req.Questions[QuestionScope]; ok {
			t.Error("scope question present, want absent")
		}
	})

	t.Run("1 scope: options are {s, none}", func(t *testing.T) {
		req := NewRequest("q", Applied{}, []string{"repo:a/x"})
		q, ok := req.Questions[QuestionScope]
		if !ok {
			t.Fatal("scope question absent, want present")
		}
		if len(q.Options) != 2 {
			t.Fatalf("len(Options) = %d, want 2: %v", len(q.Options), q.Options)
		}
		if _, ok := q.Options["repo:a/x"]; !ok {
			t.Error(`Options["repo:a/x"] missing`)
		}
		if _, ok := q.Options[NoneOption]; !ok {
			t.Error(`Options["none"] missing`)
		}
	})

	t.Run("254 scopes: 255 options, Validate() nil", func(t *testing.T) {
		scopes := make([]string, MaxScopeOptions)
		for i := range scopes {
			scopes[i] = fmt.Sprintf("repo:a/s%d", i)
		}
		req := NewRequest("q", Applied{}, scopes)
		q, ok := req.Questions[QuestionScope]
		if !ok {
			t.Fatal("scope question absent, want present")
		}
		if len(q.Options) != MaxScopeOptions+1 {
			t.Fatalf("len(Options) = %d, want %d", len(q.Options), MaxScopeOptions+1)
		}
		if err := req.Validate(); err != nil {
			t.Errorf("req.Validate() = %v, want nil", err)
		}
	})

	t.Run("255 scopes: no scope question", func(t *testing.T) {
		scopes := make([]string, MaxScopeOptions+1)
		for i := range scopes {
			scopes[i] = fmt.Sprintf("repo:a/s%d", i)
		}
		req := NewRequest("q", Applied{}, scopes)
		if _, ok := req.Questions[QuestionScope]; ok {
			t.Error("scope question present, want absent (over MaxScopeOptions)")
		}
	})

	t.Run("applied scope with 3 scopes: no scope question", func(t *testing.T) {
		req := NewRequest("q", Applied{Scope: "repo:a/x"}, []string{"repo:a/x", "repo:a/y", "repo:a/z"})
		if _, ok := req.Questions[QuestionScope]; ok {
			t.Error("scope question present, want absent (scope already applied)")
		}
	})

	t.Run("a scope literally named none is dropped", func(t *testing.T) {
		req := NewRequest("q", Applied{}, []string{"repo:a/x", "none"})
		q, ok := req.Questions[QuestionScope]
		if !ok {
			t.Fatal("scope question absent, want present")
		}
		if len(q.Options) != 2 {
			t.Fatalf("len(Options) = %d, want 2: %v", len(q.Options), q.Options)
		}
		if _, ok := q.Options["none"]; !ok {
			t.Error(`Options["none"] missing (reserved)`)
		}
	})
}

// TestFromResponseChoices proves FromResponse's choice-answer validation for
// the D-08 scope question (Task 2 extends this test with a "time" subtest
// for the time_window half): a chosen "none" yields no suggestion; a chosen
// scope at or above Threshold is suggested, below is not; a chosen option
// that was never asked, a missing Probabilities entry for the chosen
// option, or a noul-typed answer for "scope" all yield
// decide.ErrDecisionMalformedResponse and zero suggestions.
func TestFromResponseChoices(t *testing.T) {
	scopes := []string{"repo:a/x", "repo:a/y"}

	t.Run("scope", func(t *testing.T) {
		// All four categories AND CreatedAfter applied so only the scope
		// question is asked — this subtest is about the scope choice-answer
		// validation alone.
		req := NewRequest("what did we decide", Applied{
			Categories:   append([]string{}, Categories...),
			CreatedAfter: "2026-01-01T00:00:00Z",
		}, scopes)

		t.Run("none chosen: no suggestion", func(t *testing.T) {
			resp := decide.Response{Answers: map[string]decide.Answer{
				QuestionScope: {Type: decide.QuestionChoice, Choice: NoneOption, Probabilities: map[string]float64{NoneOption: 0.97, "repo:a/x": 0.02, "repo:a/y": 0.01}},
			}}
			got, err := FromResponse(resp, req, time.Now())
			if err != nil {
				t.Fatalf("FromResponse: %v", err)
			}
			for _, s := range got {
				if s.Kind == KindScope {
					t.Errorf("scope suggested = %v, want none", s)
				}
			}
		})

		t.Run("chosen at threshold: suggested", func(t *testing.T) {
			resp := decide.Response{Answers: map[string]decide.Answer{
				QuestionScope: {Type: decide.QuestionChoice, Choice: "repo:a/x", Probabilities: map[string]float64{"repo:a/x": 0.9, NoneOption: 0.1}},
			}}
			got, err := FromResponse(resp, req, time.Now())
			if err != nil {
				t.Fatalf("FromResponse: %v", err)
			}
			var found bool
			for _, s := range got {
				if s.Kind == KindScope && s.Value == "repo:a/x" && s.Source == SourceDecided {
					found = true
				}
			}
			if !found {
				t.Errorf("got = %v, want a scope suggestion for repo:a/x", got)
			}
		})

		t.Run("below threshold: not suggested", func(t *testing.T) {
			resp := decide.Response{Answers: map[string]decide.Answer{
				QuestionScope: {Type: decide.QuestionChoice, Choice: "repo:a/x", Probabilities: map[string]float64{"repo:a/x": 0.8999, NoneOption: 0.1001}},
			}}
			got, err := FromResponse(resp, req, time.Now())
			if err != nil {
				t.Fatalf("FromResponse: %v", err)
			}
			for _, s := range got {
				if s.Kind == KindScope {
					t.Errorf("scope suggested = %v, want none (below threshold)", s)
				}
			}
		})

		t.Run("malformed: chosen option never asked", func(t *testing.T) {
			resp := decide.Response{Answers: map[string]decide.Answer{
				QuestionScope: {Type: decide.QuestionChoice, Choice: "repo:z/never-asked", Probabilities: map[string]float64{"repo:z/never-asked": 0.95}},
			}}
			_, err := FromResponse(resp, req, time.Now())
			if !errors.Is(err, decide.ErrDecisionMalformedResponse) {
				t.Fatalf("err = %v, want ErrDecisionMalformedResponse", err)
			}
		})

		t.Run("malformed: missing probabilities entry for chosen option", func(t *testing.T) {
			resp := decide.Response{Answers: map[string]decide.Answer{
				QuestionScope: {Type: decide.QuestionChoice, Choice: "repo:a/x", Probabilities: map[string]float64{NoneOption: 0.5}},
			}}
			_, err := FromResponse(resp, req, time.Now())
			if !errors.Is(err, decide.ErrDecisionMalformedResponse) {
				t.Fatalf("err = %v, want ErrDecisionMalformedResponse", err)
			}
		})

		t.Run("malformed: noul-typed answer for scope", func(t *testing.T) {
			resp := decide.Response{Answers: map[string]decide.Answer{
				QuestionScope: {Type: decide.QuestionNoul, Probability: 0.95},
			}}
			_, err := FromResponse(resp, req, time.Now())
			if !errors.Is(err, decide.ErrDecisionMalformedResponse) {
				t.Fatalf("err = %v, want ErrDecisionMalformedResponse", err)
			}
		})
	})

	t.Run("time", func(t *testing.T) {
		// All four categories applied, no scopes offered, so only the
		// time_window question is asked.
		req := NewRequest("what did we decide", Applied{Categories: append([]string{}, Categories...)}, nil)
		now := time.Date(2026, 9, 28, 14, 23, 5, 0, time.UTC)

		t.Run("past_week at threshold: one KindTimeWindow suggestion", func(t *testing.T) {
			resp := decide.Response{Answers: map[string]decide.Answer{
				QuestionTimeWindow: {Type: decide.QuestionChoice, Choice: "past_week", Probabilities: map[string]float64{"past_week": 0.95, NoneOption: 0.05}},
			}}
			got, err := FromResponse(resp, req, now)
			if err != nil {
				t.Fatalf("FromResponse: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("len(got) = %d, want 1: %v", len(got), got)
			}
			wantAfter, wantBefore, wantLabel, ok := Window("past_week", now)
			if !ok {
				t.Fatal("Window(past_week) ok = false")
			}
			s := got[0]
			if s.Kind != KindTimeWindow || s.Value != "past_week" || s.Source != SourceDecided {
				t.Errorf("got = %+v, want Kind=time_window Value=past_week Source=decided", s)
			}
			if s.CreatedAfter != wantAfter || s.CreatedBefore != wantBefore || s.Label != wantLabel {
				t.Errorf("got bounds = (%q, %q, %q), want (%q, %q, %q)", s.CreatedAfter, s.CreatedBefore, s.Label, wantAfter, wantBefore, wantLabel)
			}
		})

		t.Run("none chosen: no suggestion", func(t *testing.T) {
			resp := decide.Response{Answers: map[string]decide.Answer{
				QuestionTimeWindow: {Type: decide.QuestionChoice, Choice: NoneOption, Probabilities: map[string]float64{NoneOption: 0.97, "today": 0.01, "past_week": 0.01, "past_month": 0.005, "past_year": 0.005}},
			}}
			got, err := FromResponse(resp, req, now)
			if err != nil {
				t.Fatalf("FromResponse: %v", err)
			}
			for _, s := range got {
				if s.Kind == KindTimeWindow {
					t.Errorf("time_window suggested = %v, want none", s)
				}
			}
		})

		t.Run("below threshold: no suggestion", func(t *testing.T) {
			resp := decide.Response{Answers: map[string]decide.Answer{
				QuestionTimeWindow: {Type: decide.QuestionChoice, Choice: "past_week", Probabilities: map[string]float64{"past_week": 0.8999, NoneOption: 0.1001}},
			}}
			got, err := FromResponse(resp, req, now)
			if err != nil {
				t.Fatalf("FromResponse: %v", err)
			}
			for _, s := range got {
				if s.Kind == KindTimeWindow {
					t.Errorf("time_window suggested = %v, want none (below threshold)", s)
				}
			}
		})
	})
}

// TestNewRequestShape proves NewRequest's Task 2 combined shape: with
// nothing applied, the full question set (four categories, time_window,
// scope) is built, the time_window Choice's options are exactly the D-07
// buckets plus NoneOption with non-empty descriptions, the built Request
// validates, and State carries only "query"; an applied CreatedBefore
// alone (with no CreatedAfter) still skips the time_window question.
func TestNewRequestShape(t *testing.T) {
	t.Run("nothing applied with 3 scopes: full question set", func(t *testing.T) {
		scopes := []string{"repo:a/x", "repo:a/y", "repo:a/z"}
		req := NewRequest("what did we decide", Applied{}, scopes)

		wantNames := []string{
			CategoryQuestion("convention"),
			CategoryQuestion("decision"),
			CategoryQuestion("gotcha"),
			CategoryQuestion("preference"),
			QuestionTimeWindow,
			QuestionScope,
		}
		if len(req.Questions) != len(wantNames) {
			t.Fatalf("len(Questions) = %d, want %d: %v", len(req.Questions), len(wantNames), req.Questions)
		}
		for _, name := range wantNames {
			if _, ok := req.Questions[name]; !ok {
				t.Errorf("missing question %q", name)
			}
		}

		tw, ok := req.Questions[QuestionTimeWindow]
		if !ok {
			t.Fatal("time_window question absent")
		}
		wantOptions := []string{NoneOption, "today", "past_week", "past_month", "past_year"}
		if len(tw.Options) != len(wantOptions) {
			t.Fatalf("len(time_window Options) = %d, want %d: %v", len(tw.Options), len(wantOptions), tw.Options)
		}
		for _, opt := range wantOptions {
			desc, ok := tw.Options[opt]
			if !ok || desc == "" {
				t.Errorf("time_window Options[%q] missing or empty", opt)
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

	t.Run("applied CreatedBefore alone skips time_window", func(t *testing.T) {
		req := NewRequest("what did we decide", Applied{CreatedBefore: "2026-01-01T00:00:00Z"}, nil)
		if _, ok := req.Questions[QuestionTimeWindow]; ok {
			t.Error("time_window question present, want absent (CreatedBefore applied)")
		}
	})
}

// TestSuggestOrderAndSkip proves Suggest's D-05/D-07/D-08/D-09 combined
// suggestion order — categories in Categories (console) order regardless
// of probability, then time_window, then scope, then matched tags — and
// the all-applied skip path (zero Decide calls, OutcomeSkipped, matched
// tags still returned).
func TestSuggestOrderAndSkip(t *testing.T) {
	t.Run("full suggestion set: console category order, then time_window, scope, tags", func(t *testing.T) {
		dec := &countingDecider{resp: decide.Response{Answers: map[string]decide.Answer{
			CategoryQuestion("convention"): {Type: decide.QuestionNoul, Probability: 0.2},
			CategoryQuestion("gotcha"):     {Type: decide.QuestionNoul, Probability: 0.91},
			CategoryQuestion("decision"):   {Type: decide.QuestionNoul, Probability: 0.95},
			CategoryQuestion("preference"): {Type: decide.QuestionNoul, Probability: 0.1},
			QuestionTimeWindow: {
				Type:          decide.QuestionChoice,
				Choice:        "past_month",
				Probabilities: map[string]float64{"past_month": 0.92, NoneOption: 0.08},
			},
			QuestionScope: {
				Type:          decide.QuestionChoice,
				Choice:        "repo:a/x",
				Probabilities: map[string]float64{"repo:a/x": 0.97, NoneOption: 0.03},
			},
		}}}
		now := time.Date(2026, 9, 28, 14, 23, 5, 0, time.UTC)
		res := Suggest(context.Background(), dec, Input{
			Query:  "what did we decide about tag1 and tag2",
			Scopes: []string{"repo:a/x"},
			Tags:   []string{"tag1", "tag2", "unrelated"},
			Now:    now,
		})
		if res.Outcome != OutcomeDecided {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeDecided)
		}
		wantKinds := []Kind{KindCategory, KindCategory, KindTimeWindow, KindScope, KindTag, KindTag}
		wantValues := []string{"gotcha", "decision", "past_month", "repo:a/x", "tag1", "tag2"}
		if len(res.Suggestions) != len(wantValues) {
			t.Fatalf("len(Suggestions) = %d, want %d: %v", len(res.Suggestions), len(wantValues), res.Suggestions)
		}
		for i, s := range res.Suggestions {
			if s.Kind != wantKinds[i] || s.Value != wantValues[i] {
				t.Errorf("Suggestions[%d] = %+v, want Kind=%q Value=%q", i, s, wantKinds[i], wantValues[i])
			}
		}
	})

	t.Run("everything applied: zero Decide calls, matched tags still returned", func(t *testing.T) {
		dec := &countingDecider{}
		res := Suggest(context.Background(), dec, Input{
			Query: "what did we decide about tag1",
			Applied: Applied{
				Categories:   append([]string{}, Categories...),
				Scope:        "repo:a/x",
				CreatedAfter: "2026-01-01T00:00:00Z",
			},
			Tags: []string{"tag1", "unrelated"},
			Now:  time.Now(),
		})
		if res.Outcome != OutcomeSkipped {
			t.Errorf("Outcome = %q, want %q", res.Outcome, OutcomeSkipped)
		}
		if dec.decideCalls != 0 {
			t.Errorf("decideCalls = %d, want 0", dec.decideCalls)
		}
		if len(res.Suggestions) != 1 || res.Suggestions[0].Value != "tag1" {
			t.Errorf("Suggestions = %v, want exactly [tag1]", res.Suggestions)
		}
	})
}
