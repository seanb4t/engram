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
	"log/slog"
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

// QuestionScope is the decide.Request question name for the D-08 scope
// Choice.
const QuestionScope = "scope"

// NoneOption is the reserved Choice-question option meaning "the query does
// not name or clearly point to one of the offered options" — shared by the
// D-08 scope question and (plan Task 2) the D-07 time-window question. A
// scope literally named NoneOption is dropped from scopeOptions rather than
// colliding with the reserved meaning.
const NoneOption = "none"

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
// filters, the caller's own readable scopes (D-08 scope Choice options,
// caller-scoped — never a second store read), the caller's own tag
// vocabulary (D-09 local matching — never sent to the decision provider),
// and the request time (D-07's time-window bucket-to-window conversion).
type Input struct {
	Query   string
	Applied Applied
	Scopes  []string
	Tags    []string
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

// scopeOptions builds the D-08 scope Choice's options from scopes: one
// entry per de-duplicated scope (a scope literally named NoneOption is
// dropped — it cannot be distinguished from the reserved option) plus the
// reserved NoneOption entry. The returned map's length is always at least
// 1 (NoneOption alone for a zero-length scopes).
func scopeOptions(scopes []string) map[string]string {
	seen := make(map[string]struct{}, len(scopes))
	opts := make(map[string]string, len(scopes)+1)
	for _, sc := range scopes {
		if sc == NoneOption {
			continue
		}
		if _, dup := seen[sc]; dup {
			continue
		}
		seen[sc] = struct{}{}
		opts[sc] = "Memories stored in the " + sc + " scope."
	}
	opts[NoneOption] = "The query does not name or clearly point to one of these scopes."
	return opts
}

// NewRequest builds the D-05/D-06/D-07/D-08 decide.Request for query: the
// query state (truncated to MaxQueryChars runes), one Noul question per
// category in Categories not already present in applied.Categories (zero
// applied categories asks all four; every category applied asks none), a
// time_window Choice over WindowOptions() when applied.CreatedAfter and
// applied.CreatedBefore are both empty, and — when applied.Scope is empty
// and the de-duplicated scope count (scopes minus any named NoneOption) is
// between 1 and MaxScopeOptions inclusive — one Choice question over
// scopeOptions(scopes). The scope gate counts scopes only, never a store
// scan-cap flag (RESEARCH Pitfall 5): a caller with more scopes than
// MaxScopeOptions readable, or an already-applied scope, asks no scope
// question at all.
func NewRequest(query string, applied Applied, scopes []string) decide.Request {
	questions := make(map[string]decide.Question, len(Categories)+2)
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
	if applied.CreatedAfter == "" && applied.CreatedBefore == "" {
		questions[QuestionTimeWindow] = decide.Choice(
			"When were the memories the person is looking for recorded?",
			WindowOptions(),
		)
	}
	if applied.Scope == "" {
		opts := scopeOptions(scopes)
		n := len(opts) - 1 // exclude the reserved NoneOption entry
		if n >= 1 && n <= MaxScopeOptions {
			questions[QuestionScope] = decide.Choice(
				"Does the query name, or clearly point to, one of these scopes? Choose none if it does not.",
				opts,
			)
		}
	}
	return decide.Request{
		State: decide.State{
			StateQuery: verdict.State("", query, MaxQueryChars),
		},
		Questions: questions,
	}
}

// validateChoiceAnswer checks resp's answer for the choice question named
// name, asked as req.Questions[name] (the caller must have already
// confirmed it was asked): the answer must be present, type choice, its
// Choice must be a key of the asked question's Options, and
// Probabilities[Choice] must be present and finite in [0, 1]. Any
// violation returns a *decide.Error with Kind ErrDecisionMalformedResponse
// naming name — never a partial or best-effort result (D-05).
func validateChoiceAnswer(resp decide.Response, req decide.Request, name string) (choice string, prob float64, err error) {
	q := req.Questions[name]
	ans, ok := resp.Answers[name]
	if !ok || ans.Type != decide.QuestionChoice {
		return "", 0, &decide.Error{Kind: decide.ErrDecisionMalformedResponse, Question: name}
	}
	if _, askedOption := q.Options[ans.Choice]; !askedOption {
		return "", 0, &decide.Error{Kind: decide.ErrDecisionMalformedResponse, Question: name}
	}
	p, ok := ans.Probabilities[ans.Choice]
	if !ok || math.IsNaN(p) || math.IsInf(p, 0) || p < 0 || p > 1 {
		return "", 0, &decide.Error{Kind: decide.ErrDecisionMalformedResponse, Question: name}
	}
	return ans.Choice, p, nil
}

// FromResponse maps resp to the decided suggestions, in emission order:
// categories (Categories order), then time_window, then scope. For every
// category whose question is present in req.Questions, resp.Answers must
// carry a present, noul-typed, finite [0, 1] answer — else FromResponse
// returns a *decide.Error with Kind decide.ErrDecisionMalformedResponse
// naming the offending question, never a partial result. A category is
// suggested (KindCategory, Source SourceDecided) when its Probability is
// at or above Threshold. When the QuestionTimeWindow question was asked,
// its answer is validated like a category (see validateChoiceAnswer) and,
// when the chosen bucket is not NoneOption and its probability is at or
// above Threshold, Window(bucket, now) converts it to a KindTimeWindow
// suggestion. When the QuestionScope question was asked, its answer is
// validated the same way and a KindScope suggestion is emitted when the
// chosen option is not NoneOption and its probability is at or above
// Threshold.
func FromResponse(resp decide.Response, req decide.Request, now time.Time) ([]Suggestion, error) {
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
	if _, asked := req.Questions[QuestionTimeWindow]; asked {
		bucket, p, err := validateChoiceAnswer(resp, req, QuestionTimeWindow)
		if err != nil {
			return nil, err
		}
		if bucket != NoneOption && p >= Threshold {
			after, before, label, ok := Window(bucket, now)
			if ok {
				out = append(out, Suggestion{
					Kind:          KindTimeWindow,
					Value:         bucket,
					CreatedAfter:  after,
					CreatedBefore: before,
					Label:         label,
					Source:        SourceDecided,
				})
			}
		}
	}
	if _, asked := req.Questions[QuestionScope]; asked {
		choice, p, err := validateChoiceAnswer(resp, req, QuestionScope)
		if err != nil {
			return nil, err
		}
		if choice != NoneOption && p >= Threshold {
			out = append(out, Suggestion{Kind: KindScope, Value: choice, Source: SourceDecided})
		}
	}
	return out, nil
}

// Suggest builds one Request from in, asks dec exactly once (never
// DecideMany), and maps the response to suggestions. Tag suggestions
// (D-09) are matched locally against in.Tags and are always included,
// regardless of outcome — matching a tag is never a decision. Suggest
// never returns an error — a decision failure is "no decision"
// (rwtzp3m7y8): zero questions in the built request or a nil dec skips the
// Decide call entirely (OutcomeSkipped, matched tags still returned); a
// Decide error or a FromResponse error both yield OutcomeFallback with
// FallbackClass naming why (matched tags still returned) and one Warn line
// via logFallback (D-05/D-17).
func Suggest(ctx context.Context, dec decide.Decider, in Input) Result {
	matched := MatchTags(in.Query, in.Tags, in.Applied.Tags)
	req := NewRequest(in.Query, in.Applied, in.Scopes)
	n := len(req.Questions)
	if n == 0 || dec == nil {
		return Result{Suggestions: matched, Outcome: OutcomeSkipped, QuestionsAsked: n}
	}
	resp, err := dec.Decide(ctx, req)
	if err != nil {
		class := decide.Status(err)
		logFallback(ctx, class)
		return Result{Suggestions: matched, Outcome: OutcomeFallback, FallbackClass: class, QuestionsAsked: n}
	}
	decided, err := FromResponse(resp, req, in.Now)
	if err != nil {
		class := decide.Status(err)
		logFallback(ctx, class)
		return Result{Suggestions: matched, Outcome: OutcomeFallback, FallbackClass: class, QuestionsAsked: n}
	}
	return Result{Suggestions: append(decided, matched...), Outcome: OutcomeDecided, QuestionsAsked: n}
}

// logFallback emits the one WarnContext line a decision failure or a
// malformed response produces — the class word only, never the query, a
// question, or the error text (D-05/D-17).
func logFallback(ctx context.Context, class string) {
	slog.WarnContext(ctx, "query understanding fell back to no decided suggestions", "class", class)
}
