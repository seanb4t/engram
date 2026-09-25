// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package relevance is the search-path relevance question set shared by the
// server's Jev rank hook and the retrieval eval: spike 004's noul-per-
// candidate request, the D-08 budgeted candidate state, and mapping a
// decide.Response to a per-id probability map. It performs no I/O of its
// own beyond the decide.Decider it is handed, and it never filters, drops
// or acts on a probability — callers do that.
package relevance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/seanb4t/engram/internal/decide"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/verdict"
)

// StateQuery is the decide.Request state key carrying the search query text.
const StateQuery = "query"

// WhenTrue and WhenFalse are spike 004's noul criteria, verbatim, shared by
// every per-candidate question this package builds.
const (
	WhenTrue  = "The record directly answers what the query asks."
	WhenFalse = "The record is about something else, or only shares vocabulary with the query."
)

// D-08 budget constants. DefaultCandidateChars is the starting per-candidate
// truncation length in Unicode code points; MinCandidateChars is the floor
// the total-state guard refuses to shrink below; MaxQueryChars bounds the
// query state; DefaultTokenBudget is the estimated-request ceiling;
// BytesPerToken is the UTF-8-bytes-per-token divisor EstimateTokens uses.
const (
	DefaultCandidateChars = 600
	MinCandidateChars     = 100
	MaxQueryChars         = 2000
	DefaultTokenBudget    = 28000
	BytesPerToken         = 4
)

// Budget holds the D-08 budget knobs NewRequest and EstimateTokens use.
type Budget struct {
	CandidateChars    int
	MinCandidateChars int
	QueryChars        int
	TokenBudget       int
}

// DefaultBudget returns the D-08 default Budget.
func DefaultBudget() Budget {
	return Budget{
		CandidateChars:    DefaultCandidateChars,
		MinCandidateChars: MinCandidateChars,
		QueryChars:        MaxQueryChars,
		TokenBudget:       DefaultTokenBudget,
	}
}

// ErrNoCandidates is returned by NewRequest when given zero hits.
var ErrNoCandidates = errors.New("relevance: no candidates")

// ErrStateBudget is returned by NewRequest when the total-state guard
// cannot shrink the per-candidate truncation enough to fit TokenBudget
// without going below MinCandidateChars.
var ErrStateBudget = errors.New("relevance: request exceeds token budget")

// Instructions returns the spike 004 instructions text for candidate key.
func Instructions(key string) string {
	return "Does memory record " + key + " directly answer the query?"
}

// CandidateKey returns the zero-padded two-digit candidate key for index i
// (c00, c07, c99), matching every candidate's position in the lexical-rank
// order NewRequest builds the request in.
func CandidateKey(i int) string {
	return fmt.Sprintf("c%02d", i)
}

// estimateBytesOf sums the UTF-8 byte length of every state key/string
// value plus every question's name, Instructions, WhenTrue and WhenFalse —
// the raw byte total EstimateTokens divides down to a token estimate.
func estimateBytesOf(req decide.Request) int {
	total := 0
	for k, v := range req.State {
		total += len(k)
		if s, ok := v.(string); ok {
			total += len(s)
		}
	}
	for name, q := range req.Questions {
		total += len(name) + len(q.Instructions) + len(q.WhenTrue) + len(q.WhenFalse)
	}
	return total
}

// EstimateTokens estimates req's token cost as UTF-8 bytes / BytesPerToken,
// rounded up using integer arithmetic only.
func EstimateTokens(req decide.Request) int {
	b := estimateBytesOf(req)
	return (b + BytesPerToken - 1) / BytesPerToken
}

// candidateBytesAt sums the UTF-8 byte length of every candidate state
// value in req, built at per code points — the per-dependent share of
// estimateBytesOf's total.
func candidateBytesAt(req decide.Request, n int) int {
	total := 0
	for i := 0; i < n; i++ {
		if s, ok := req.State[CandidateKey(i)].(string); ok {
			total += len(s)
		}
	}
	return total
}

// buildRequest builds the D-08 decide.Request for query against hits, with
// every candidate's state truncated to per code points.
func buildRequest(query string, hits []store.Memory, b Budget, per int) decide.Request {
	state := decide.State{
		StateQuery: verdict.State("", query, b.QueryChars),
	}
	questions := make(map[string]decide.Question, len(hits))
	for i, h := range hits {
		key := CandidateKey(i)
		state[key] = verdict.State(h.Summary, h.Content, per)
		questions[key] = decide.Noul(Instructions(key), WhenTrue, WhenFalse)
	}
	return decide.Request{State: state, Questions: questions}
}

// NewRequest builds the D-04/D-08 decide.Request for query against hits (in
// their given order — the caller's lexical-rank order): one noul question
// per candidate, keyed by CandidateKey, plus the query state key. Zero hits
// returns ErrNoCandidates. When the built request's EstimateTokens exceeds
// b.TokenBudget, the shared per-candidate truncation length is shrunk
// deterministically (every candidate always shares the same per) and the
// request rebuilt; if the shrink would take per below b.MinCandidateChars,
// NewRequest returns ErrStateBudget instead of ever sending an
// over-budget/under-floor request.
func NewRequest(query string, hits []store.Memory, b Budget) (decide.Request, error) {
	if len(hits) == 0 {
		return decide.Request{}, ErrNoCandidates
	}
	per := b.CandidateChars
	for {
		req := buildRequest(query, hits, b, per)
		if EstimateTokens(req) <= b.TokenBudget {
			return req, nil
		}
		totalBytes := estimateBytesOf(req)
		c := candidateBytesAt(req, len(hits))
		f := totalBytes - c
		target := int64(b.TokenBudget)*int64(BytesPerToken) - int64(f)
		if target <= 0 || c <= 0 {
			return decide.Request{}, ErrStateBudget
		}
		next := int64(per) * target / int64(c)
		if next >= int64(per) {
			next = int64(per) - 1
		}
		if next < int64(b.MinCandidateChars) {
			return decide.Request{}, ErrStateBudget
		}
		per = int(next)
	}
}

// FromResponse maps resp to a per-id relevance-probability map: for every
// hit index i, resp.Answers[CandidateKey(i)] must be present, of type
// decide.QuestionNoul, and carry a finite Probability in [0, 1] — else
// FromResponse returns a *decide.Error with Kind
// decide.ErrDecisionMalformedResponse naming the missing/mistyped/
// out-of-range question (D-03: NaN, +Inf, -Inf, below 0 and above 1 are
// all malformed; exactly 0 and exactly 1 are accepted). On success,
// hits[i].ID maps to the answer's Probability, verbatim — never rounded,
// clamped or renormalized.
func FromResponse(resp decide.Response, hits []store.Memory) (map[string]float64, error) {
	out := make(map[string]float64, len(hits))
	for i, h := range hits {
		key := CandidateKey(i)
		ans, ok := resp.Answers[key]
		if !ok || ans.Type != decide.QuestionNoul {
			return nil, &decide.Error{Kind: decide.ErrDecisionMalformedResponse, Question: key}
		}
		p := ans.Probability
		if math.IsNaN(p) || math.IsInf(p, 0) || p < 0 || p > 1 {
			return nil, &decide.Error{Kind: decide.ErrDecisionMalformedResponse, Question: key}
		}
		out[h.ID] = p
	}
	return out, nil
}

// errClass classifies err for the fallback log line: ErrNoCandidates and
// ErrStateBudget get their own fixed words, everything else goes through
// decide.Status.
func errClass(err error) string {
	switch {
	case errors.Is(err, ErrNoCandidates):
		return "no_candidates"
	case errors.Is(err, ErrStateBudget):
		return "state_budget"
	default:
		return decide.Status(err)
	}
}

// logFallback emits the one WarnContext line a Hook failure produces,
// carrying only the class word from errClass — never the query, a
// candidate state, or err.Error() — and stamps that same class word on the
// ambient search span as store.AttrRerankFallbackClass (#618), so the span
// says why it fell back without a log join. The store stamps the outcome
// itself; only the hook can classify its own failure.
func logFallback(ctx context.Context, err error) {
	class := errClass(err)
	trace.SpanFromContext(ctx).SetAttributes(attribute.String(store.AttrRerankFallbackClass, class))
	slog.WarnContext(ctx, "search rerank fell back to lexical order", "class", class)
}

// Hook builds a store.RankHook over dec, budgeted by b. A nil dec returns a
// nil hook (store.RankWithHook treats a nil hook as the byte-identical
// no-rerank path). The returned hook builds one request via NewRequest,
// calls dec.Decide exactly once (never DecideMany), and maps the response
// via FromResponse; any error along that path is logged via logFallback and
// returned as (nil, err) — the caller (store.applyRankHook) is what
// actually falls back to lexical order.
func Hook(dec decide.Decider, b Budget) store.RankHook {
	if dec == nil {
		return nil
	}
	return func(ctx context.Context, query string, hits []store.Memory) (map[string]float64, error) {
		req, err := NewRequest(query, hits, b)
		if err != nil {
			logFallback(ctx, err)
			return nil, err
		}
		resp, err := dec.Decide(ctx, req)
		if err != nil {
			logFallback(ctx, err)
			return nil, err
		}
		rel, err := FromResponse(resp, hits)
		if err != nil {
			logFallback(ctx, err)
			return nil, err
		}
		return rel, nil
	}
}
