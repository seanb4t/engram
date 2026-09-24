// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package relevance

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/seanb4t/engram/internal/decide"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/verdict"
)

// repeatToBytes returns unit repeated enough times that the result's UTF-8
// byte length is the largest multiple of len(unit) not exceeding
// targetBytes — used to build fixture content/query text of an
// approximate target size without ever splitting a multi-byte rune.
func repeatToBytes(unit string, targetBytes int) string {
	n := targetBytes / len(unit)
	return strings.Repeat(unit, n)
}

// fixtureHits builds n short-content hits (ids id-000.. id-0NN, distinct
// summary/content per hit) — the shape most Task 1 tests that don't care
// about content size use.
func fixtureHits(n int) []store.Memory {
	hits := make([]store.Memory, n)
	for i := range hits {
		hits[i] = store.Memory{
			ID:      fmt.Sprintf("id-%03d", i),
			Summary: fmt.Sprintf("summary %03d", i),
			Content: fmt.Sprintf("content %03d", i),
		}
	}
	return hits
}

// TestNewRequestShape pins RANK-05's shape truth at n=1 and n=100: state
// keys are exactly query plus c00..c(n-1), each candidate state matches
// verdict.State verbatim for short (unshrunk) input, every question is a
// verbatim spike-004 noul, and the resulting request validates.
func TestNewRequestShape(t *testing.T) {
	for _, n := range []int{1, 100} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			hits := fixtureHits(n)
			req, err := NewRequest("what is the deploy process", hits, DefaultBudget())
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			if len(req.State) != n+1 {
				t.Fatalf("len(req.State) = %d, want %d (query + %d candidates)", len(req.State), n+1, n)
			}
			if _, ok := req.State[StateQuery]; !ok {
				t.Fatal("req.State missing the query key")
			}
			if len(req.Questions) != n {
				t.Fatalf("len(req.Questions) = %d, want %d", len(req.Questions), n)
			}
			for i, h := range hits {
				key := CandidateKey(i)
				stateVal, ok := req.State[key].(string)
				if !ok {
					t.Fatalf("req.State[%q] missing or not a string", key)
				}
				want := verdict.State(h.Summary, h.Content, DefaultCandidateChars)
				if stateVal != want {
					t.Errorf("req.State[%q] = %q, want %q", key, stateVal, want)
				}
				q, ok := req.Questions[key]
				if !ok {
					t.Fatalf("req.Questions[%q] missing", key)
				}
				wantQ := decide.Noul(Instructions(key), WhenTrue, WhenFalse)
				if !reflect.DeepEqual(q, wantQ) {
					t.Errorf("req.Questions[%q] = %+v, want %+v", key, q, wantQ)
				}
			}
			if err := req.Validate(); err != nil {
				t.Errorf("req.Validate() = %v, want nil", err)
			}
		})
	}
}

// TestNewRequestBudgetASCII pins the RANK-05 recall-maximum truth for ASCII
// content: 100 candidates of 64 KiB ASCII content plus a 512-byte summary
// and a 2000-character query stay at exactly DefaultCandidateChars per
// candidate (no shrink needed) and the estimate stays within
// DefaultTokenBudget.
func TestNewRequestBudgetASCII(t *testing.T) {
	const n = 100
	content := repeatToBytes("a", 64*1024)
	summary := repeatToBytes("s", 512)
	hits := make([]store.Memory, n)
	for i := range hits {
		hits[i] = store.Memory{ID: fmt.Sprintf("id-%03d", i), Summary: summary, Content: content}
	}
	query := repeatToBytes("q", 2000)

	req, err := NewRequest(query, hits, DefaultBudget())
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	for i := range hits {
		key := CandidateKey(i)
		s, ok := req.State[key].(string)
		if !ok {
			t.Fatalf("req.State[%q] missing or not a string", key)
		}
		if got := utf8.RuneCountInString(s); got != DefaultCandidateChars {
			t.Errorf("candidate %s rune count = %d, want %d", key, got, DefaultCandidateChars)
		}
	}
	if got := EstimateTokens(req); got > DefaultTokenBudget {
		t.Errorf("EstimateTokens = %d, want <= %d", got, DefaultTokenBudget)
	}
}

// TestNewRequestBudgetShrinksMultibyte pins the RANK-05 recall-maximum truth
// for multi-byte content: 100 candidates of 64 KiB three-byte (CJK) or
// four-byte (emoji) text shrink to a uniform per-candidate length below
// DefaultCandidateChars and at or above MinCandidateChars, staying valid
// UTF-8 throughout, with the estimate at or under DefaultTokenBudget.
func TestNewRequestBudgetShrinksMultibyte(t *testing.T) {
	cases := []struct {
		name string
		unit string
	}{
		{"cjk", "中"},            // 3-byte
		{"emoji", "\U0001F600"}, // 4-byte
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const n = 100
			content := repeatToBytes(tc.unit, 64*1024)
			hits := make([]store.Memory, n)
			for i := range hits {
				hits[i] = store.Memory{ID: fmt.Sprintf("id-%03d", i), Content: content}
			}

			req, err := NewRequest("query text", hits, DefaultBudget())
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			if got := EstimateTokens(req); got > DefaultTokenBudget {
				t.Errorf("EstimateTokens = %d, want <= %d", got, DefaultTokenBudget)
			}

			length := -1
			for i := range hits {
				key := CandidateKey(i)
				s, ok := req.State[key].(string)
				if !ok {
					t.Fatalf("req.State[%q] missing or not a string", key)
				}
				if !utf8.ValidString(s) {
					t.Errorf("candidate %s state is not valid UTF-8: %q", key, s)
				}
				rl := utf8.RuneCountInString(s)
				if length == -1 {
					length = rl
					continue
				}
				if rl != length {
					t.Errorf("candidate %s rune length = %d, want uniform %d", key, rl, length)
				}
			}
			if length >= DefaultCandidateChars {
				t.Errorf("candidate rune length = %d, want < %d (shrink expected)", length, DefaultCandidateChars)
			}
			if length < MinCandidateChars {
				t.Errorf("candidate rune length = %d, want >= %d", length, MinCandidateChars)
			}
		})
	}
}

// TestNewRequestQueryCap pins the RANK-05 query-cap truth: a 64 KiB emoji
// query yields a query state of exactly MaxQueryChars code points, staying
// valid UTF-8.
func TestNewRequestQueryCap(t *testing.T) {
	query := repeatToBytes("\U0001F600", 64*1024)
	hits := []store.Memory{{ID: "id-000", Content: "short content"}}

	req, err := NewRequest(query, hits, DefaultBudget())
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	qs, ok := req.State[StateQuery].(string)
	if !ok {
		t.Fatal("req.State[StateQuery] missing or not a string")
	}
	if !utf8.ValidString(qs) {
		t.Errorf("query state is not valid UTF-8: %q", qs)
	}
	if got := utf8.RuneCountInString(qs); got != MaxQueryChars {
		t.Errorf("query state rune count = %d, want %d", got, MaxQueryChars)
	}
}

// TestNewRequestBudgetBoundary pins the RANK-05 boundary truth: with E the
// estimate of a fixture that needs no shrink at DefaultBudget, a Budget
// whose TokenBudget equals E returns a request deep-equal to the default
// one (no shrink triggered); TokenBudget E-1 returns a request whose
// estimate is at most E-1, with a shorter candidate state.
func TestNewRequestBudgetBoundary(t *testing.T) {
	const n = 20
	content := strings.Repeat("word ", 200) // 1000 ASCII runes, longer than DefaultCandidateChars
	hits := make([]store.Memory, n)
	for i := range hits {
		hits[i] = store.Memory{ID: fmt.Sprintf("id-%03d", i), Content: content}
	}
	query := "boundary test query"

	base := DefaultBudget()
	reqDefault, err := NewRequest(query, hits, base)
	if err != nil {
		t.Fatalf("NewRequest (default budget): %v", err)
	}
	e := EstimateTokens(reqDefault)

	atE := base
	atE.TokenBudget = e
	reqAtE, err := NewRequest(query, hits, atE)
	if err != nil {
		t.Fatalf("NewRequest (TokenBudget=E): %v", err)
	}
	if !reflect.DeepEqual(reqAtE, reqDefault) {
		t.Error("NewRequest at TokenBudget=E diverged from the default-budget request (no shrink expected)")
	}

	belowE := base
	belowE.TokenBudget = e - 1
	reqBelowE, err := NewRequest(query, hits, belowE)
	if err != nil {
		t.Fatalf("NewRequest (TokenBudget=E-1): %v", err)
	}
	if got := EstimateTokens(reqBelowE); got > e-1 {
		t.Errorf("EstimateTokens at TokenBudget=E-1 = %d, want <= %d", got, e-1)
	}
	c0 := CandidateKey(0)
	defState, _ := reqDefault.State[c0].(string)
	belowState, _ := reqBelowE.State[c0].(string)
	if got, want := utf8.RuneCountInString(belowState), utf8.RuneCountInString(defState); got >= want {
		t.Errorf("candidate state at TokenBudget=E-1 has %d runes, want fewer than the default's %d", got, want)
	}
}

// TestNewRequestBudgetFloor pins the RANK-05 floor truth: a Budget whose
// TokenBudget only fits candidates at exactly MinCandidateChars is
// accepted at that length; one token less returns ErrStateBudget. The
// exact floor token budget is computed by building directly at
// MinCandidateChars (no shrink involved) rather than hand-derived, so the
// boundary is exact regardless of the shrink loop's internal arithmetic.
func TestNewRequestBudgetFloor(t *testing.T) {
	const n = 10
	content := strings.Repeat("x", 1000) // longer than DefaultCandidateChars, ASCII
	hits := make([]store.Memory, n)
	for i := range hits {
		hits[i] = store.Memory{ID: fmt.Sprintf("id-%03d", i), Content: content}
	}
	query := "floor test query"

	atFloor := Budget{
		CandidateChars:    MinCandidateChars,
		MinCandidateChars: MinCandidateChars,
		QueryChars:        MaxQueryChars,
		TokenBudget:       1 << 30,
	}
	reqAtFloor, err := NewRequest(query, hits, atFloor)
	if err != nil {
		t.Fatalf("NewRequest (direct at floor): %v", err)
	}
	floorTokens := EstimateTokens(reqAtFloor)

	shrinking := Budget{
		CandidateChars:    DefaultCandidateChars,
		MinCandidateChars: MinCandidateChars,
		QueryChars:        MaxQueryChars,
		TokenBudget:       floorTokens,
	}
	reqAccepted, err := NewRequest(query, hits, shrinking)
	if err != nil {
		t.Fatalf("NewRequest (TokenBudget=floorTokens): %v", err)
	}
	acceptedState, ok := reqAccepted.State[CandidateKey(0)].(string)
	if !ok {
		t.Fatal("reqAccepted.State[c00] missing or not a string")
	}
	if got := utf8.RuneCountInString(acceptedState); got != MinCandidateChars {
		t.Errorf("candidate rune length at TokenBudget=floorTokens = %d, want exactly %d", got, MinCandidateChars)
	}

	shrinking.TokenBudget = floorTokens - 1
	if _, err := NewRequest(query, hits, shrinking); !errors.Is(err, ErrStateBudget) {
		t.Errorf("NewRequest (TokenBudget=floorTokens-1) err = %v, want ErrStateBudget", err)
	}
}

// TestNewRequestEmptyAndSingle pins the RANK-05 empty/single truths: nil
// and empty hits both return ErrNoCandidates; one hit builds exactly one
// question, c00.
func TestNewRequestEmptyAndSingle(t *testing.T) {
	if _, err := NewRequest("q", nil, DefaultBudget()); !errors.Is(err, ErrNoCandidates) {
		t.Errorf("NewRequest(nil hits) err = %v, want ErrNoCandidates", err)
	}
	if _, err := NewRequest("q", []store.Memory{}, DefaultBudget()); !errors.Is(err, ErrNoCandidates) {
		t.Errorf("NewRequest(empty hits) err = %v, want ErrNoCandidates", err)
	}

	hits := []store.Memory{{ID: "id-000", Summary: "s", Content: "c"}}
	req, err := NewRequest("q", hits, DefaultBudget())
	if err != nil {
		t.Fatalf("NewRequest(one hit): %v", err)
	}
	if len(req.Questions) != 1 {
		t.Fatalf("len(req.Questions) = %d, want 1", len(req.Questions))
	}
	if _, ok := req.Questions[CandidateKey(0)]; !ok {
		t.Errorf("req.Questions missing %q", CandidateKey(0))
	}
}

// TestNewRequestDuplicateContentStaysSeparate pins the RANK-05 adjacency
// truth: two hits with identical summary and content get two separate keys
// and two separate questions — nothing is merged or deduplicated.
func TestNewRequestDuplicateContentStaysSeparate(t *testing.T) {
	hits := []store.Memory{
		{ID: "id-a", Summary: "same", Content: "same content"},
		{ID: "id-b", Summary: "same", Content: "same content"},
	}
	req, err := NewRequest("q", hits, DefaultBudget())
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if len(req.Questions) != 2 {
		t.Fatalf("len(req.Questions) = %d, want 2", len(req.Questions))
	}
	if CandidateKey(0) == CandidateKey(1) {
		t.Fatal("candidate keys collided despite distinct indices")
	}
	s0, _ := req.State[CandidateKey(0)].(string)
	s1, _ := req.State[CandidateKey(1)].(string)
	if s0 != s1 {
		t.Errorf("candidate states differ despite identical input: %q vs %q", s0, s1)
	}
}

// TestEstimateTokensCeil pins the RANK-05 precision truth: EstimateTokens
// is ceil(UTF-8 bytes / BytesPerToken) using integer arithmetic only.
func TestEstimateTokensCeil(t *testing.T) {
	cases := []struct {
		bytes int
		want  int
	}{
		{0, 0},
		{1, 1},
		{4, 1},
		{5, 2},
		{8, 2},
	}
	for _, tc := range cases {
		t.Run(strconv.Itoa(tc.bytes), func(t *testing.T) {
			req := decide.Request{}
			if tc.bytes > 0 {
				req.State = decide.State{"": strings.Repeat("x", tc.bytes)}
			}
			if got := EstimateTokens(req); got != tc.want {
				t.Errorf("EstimateTokens(%d bytes) = %d, want %d", tc.bytes, got, tc.want)
			}
		})
	}
}

// TestNewRequestAtRecallMaximum pins the RANK-05 recall-maximum pool size:
// the pool NewRequest is exercised against elsewhere in this file is
// store.CandidateK(store.MaxRecallLimit), and that value is 100.
func TestNewRequestAtRecallMaximum(t *testing.T) {
	n := store.CandidateK(store.MaxRecallLimit)
	if n != 100 {
		t.Fatalf("store.CandidateK(store.MaxRecallLimit) = %d, want 100", n)
	}
	hits := fixtureHits(int(n))
	req, err := NewRequest("recall max query", hits, DefaultBudget())
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if got := len(req.Questions); got != int(n) {
		t.Errorf("len(req.Questions) = %d, want %d", got, n)
	}
}
