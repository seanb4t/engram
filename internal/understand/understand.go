// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package understand is the query-understanding core for the console
// (milestone 2026-09-25.01 Phase 6): it builds one budgeted decide.Request
// from a prose search query, maps the typed-decision answers to advisory
// filter suggestions, and performs no I/O beyond the decide.Decider it is
// handed. A suggestion is advisory only — it is surfaced, never applied by
// this package or anything downstream of it.
package understand

import (
	"context"
	"math"
	"slices"
	"time"

	"github.com/seanb4t/engram/internal/decide"
	"github.com/seanb4t/engram/internal/verdict"
)

// StateQuery is the decide.Request state key carrying the query text —
// never record content (D-05).
const StateQuery = "query"

// Threshold is the D-05 fixed probability boundary: a decided suggestion is
// emitted only when its answer's Probability is at or above this value.
// Not a config knob.
const Threshold = 0.9

// MaxQueryChars bounds the query state sent to the provider, in Unicode
// code points.
const MaxQueryChars = 2000

// MaxScopeOptions is the largest number of scopes D-08's scope Choice
// question may offer — one below decide.MaxChoices, reserving one slot for
// the "none" option.
const MaxScopeOptions = decide.MaxChoices - 1

// The three Suggest outcomes.
const (
	// OutcomeDecided means the decision call succeeded and Suggestions
	// reflects its answers (possibly empty).
	OutcomeDecided = "decided"
	// OutcomeFallback means the decision call or its response failed;
	// FallbackClass names why and Suggestions is empty.
	OutcomeFallback = "fallback"
	// OutcomeSkipped means no question needed asking (every category
	// already applied) or no Decider was configured; no Decide call was
	// made.
	OutcomeSkipped = "skipped"
)

// Categories is the console's category vocabulary, in display order
// (ui/src/lib/queries.ts CATEGORIES) — the D-06 Noul-per-category set.
var Categories = []string{"convention", "gotcha", "decision", "preference"}

// Kind names which chip a Suggestion is for.
type Kind string

// The four Kind values — one per FilterSuggestion oneof case.
const (
	KindCategory   Kind = "category"
	KindTimeWindow Kind = "time_window"
	KindScope      Kind = "scope"
	KindTag        Kind = "tag"
)

// Source names how a Suggestion was produced.
type Source string

// The two Source values.
const (
	SourceDecided Source = "decided"
	SourceMatched Source = "matched"
)

// Suggestion is one advisory, unapplied filter chip. Value carries the
// category name, scope, tag, or time-bucket name depending on Kind;
// CreatedAfter/CreatedBefore/Label are set only for KindTimeWindow.
type Suggestion struct {
	Kind          Kind
	Value         string
	CreatedAfter  string
	CreatedBefore string
	Label         string
	Source        Source
}

// Applied is the set of filters already applied to the caller's search —
// NewRequest skips a question whose answer is already applied.
type Applied struct {
	Scope         string
	Categories    []string
	Tags          []string
	CreatedAfter  string
	CreatedBefore string
}

// Input is Suggest's argument: the query text, the currently-applied
// filters, and the request time (a future time-window bucket computation
// consumes Now; unused by this plan's category-only slice).
type Input struct {
	Query   string
	Applied Applied
	Now     time.Time
}

// Result is Suggest's outcome (D-17): Outcome is one of the three
// constants above, FallbackClass is set only for OutcomeFallback, and
// QuestionsAsked is the number of questions the built request carried,
// regardless of outcome.
type Result struct {
	Suggestions    []Suggestion
	Outcome        string
	FallbackClass  string
	QuestionsAsked int
}

// CategoryQuestion returns the decide.Request question name for cat.
func CategoryQuestion(cat string) string {
	return "category_" + cat
}

// categoryWhenTrue is the D-06 per-category noul criterion for "true".
var categoryWhenTrue = map[string]string{
	"convention": "The query asks for an agreed way of doing things: a naming, structure, style or workflow convention.",
	"gotcha":     "The query asks about a pitfall, trap, surprising failure or workaround.",
	"decision":   "The query asks what was decided, or why a choice was made.",
	"preference": "The query asks about a stated personal or team preference.",
}

// categoryWhenFalse is the shared "false" criterion for every category
// question (D-06).
const categoryWhenFalse = "The query does not specifically ask for that kind of memory."

// NewRequest builds the D-05/D-06 decide.Request for query: the query
// state (truncated to MaxQueryChars runes) plus one Noul question per
// category in Categories not already present in applied.Categories. Zero
// applied categories asks all four; every category applied asks none.
func NewRequest(query string, applied Applied) decide.Request {
	questions := make(map[string]decide.Question, len(Categories))
	for _, cat := range Categories {
		if slices.Contains(applied.Categories, cat) {
			continue
		}
		questions[CategoryQuestion(cat)] = decide.Noul(
			"Is the person searching specifically for a "+cat+" memory?",
			categoryWhenTrue[cat],
			categoryWhenFalse,
		)
	}
	return decide.Request{
		State: decide.State{
			StateQuery: verdict.State("", query, MaxQueryChars),
		},
		Questions: questions,
	}
}

// FromResponse maps resp to the decided category suggestions, in
// Categories order: for every category whose question is present in
// req.Questions, resp.Answers must carry a present, noul-typed, finite
// [0, 1] answer — else FromResponse returns a *decide.Error with Kind
// decide.ErrDecisionMalformedResponse naming the offending question, never
// a partial result. A category is suggested (KindCategory, Source
// SourceDecided) when its Probability is at or above Threshold.
func FromResponse(resp decide.Response, req decide.Request) ([]Suggestion, error) {
	var out []Suggestion
	for _, cat := range Categories {
		name := CategoryQuestion(cat)
		if _, asked := req.Questions[name]; !asked {
			continue
		}
		ans, ok := resp.Answers[name]
		if !ok || ans.Type != decide.QuestionNoul {
			return nil, &decide.Error{Kind: decide.ErrDecisionMalformedResponse, Question: name}
		}
		p := ans.Probability
		if math.IsNaN(p) || math.IsInf(p, 0) || p < 0 || p > 1 {
			return nil, &decide.Error{Kind: decide.ErrDecisionMalformedResponse, Question: name}
		}
		if p >= Threshold {
			out = append(out, Suggestion{Kind: KindCategory, Value: cat, Source: SourceDecided})
		}
	}
	return out, nil
}

// Suggest builds one Request from in, asks dec exactly once (never
// DecideMany), and maps the response to suggestions. Suggest never returns
// an error — a decision failure is "no decision" (rwtzp3m7y8): zero
// questions in the built request or a nil dec skips the call entirely
// (OutcomeSkipped); a Decide error or a FromResponse error both yield
// OutcomeFallback with FallbackClass naming why.
func Suggest(ctx context.Context, dec decide.Decider, in Input) Result {
	req := NewRequest(in.Query, in.Applied)
	n := len(req.Questions)
	if n == 0 || dec == nil {
		return Result{Outcome: OutcomeSkipped, QuestionsAsked: n}
	}
	resp, err := dec.Decide(ctx, req)
	if err != nil {
		return Result{Outcome: OutcomeFallback, FallbackClass: decide.Status(err), QuestionsAsked: n}
	}
	suggestions, err := FromResponse(resp, req)
	if err != nil {
		return Result{Outcome: OutcomeFallback, FallbackClass: decide.Status(err), QuestionsAsked: n}
	}
	return Result{Suggestions: suggestions, Outcome: OutcomeDecided, QuestionsAsked: n}
}
