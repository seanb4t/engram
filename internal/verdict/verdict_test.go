// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package verdict

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/seanb4t/engram/internal/decide"
)

// TestRelationQuestionSet pins Relations' D-05 order, RelationCriteria's
// exact key set and updates' verbatim D-06 sentence, both functions'
// fresh-copy-per-call contract, and NewRequest's exact state/question
// shape (D-06) — including that the built request passes decide's own
// structural validation.
func TestRelationQuestionSet(t *testing.T) {
	want := []string{Duplicate, Contradicts, Updates, Related, Unrelated}
	got := Relations()
	if !slices.Equal(got, want) {
		t.Errorf("Relations() = %v, want %v", got, want)
	}
	got[0] = "mutated"
	if again := Relations(); !slices.Equal(again, want) {
		t.Errorf("Relations() after mutating a previous result = %v, want unaffected %v", again, want)
	}

	criteria := RelationCriteria()
	if len(criteria) != 5 {
		t.Fatalf("len(RelationCriteria()) = %d, want 5: %v", len(criteria), criteria)
	}
	for _, name := range want {
		if _, ok := criteria[name]; !ok {
			t.Errorf("RelationCriteria() is missing %q: %v", name, criteria)
		}
	}
	const wantUpdates = "B is a newer state or a more complete version of the same fact."
	if criteria[Updates] != wantUpdates {
		t.Errorf("RelationCriteria()[updates] = %q, want %q", criteria[Updates], wantUpdates)
	}
	criteria[Duplicate] = "mutated"
	if again := RelationCriteria(); again[Duplicate] == "mutated" {
		t.Error("RelationCriteria() after mutating a previous result reflects the mutation, want unaffected")
	}

	req := NewRequest("x", "y")
	wantState := []string{"context", "record_a", "record_b"}
	if len(req.State) != len(wantState) {
		t.Fatalf("len(req.State) = %d, want %d: %v", len(req.State), len(wantState), req.State)
	}
	for _, k := range wantState {
		if _, ok := req.State[k]; !ok {
			t.Errorf("req.State is missing key %q: %v", k, req.State)
		}
	}
	if len(req.Questions) != 2 {
		t.Fatalf("len(req.Questions) = %d, want 2: %v", len(req.Questions), req.Questions)
	}
	relationQ, ok := req.Questions[QuestionRelation]
	if !ok || relationQ.Type != decide.QuestionChoice || len(relationQ.Options) != 5 {
		t.Errorf("req.Questions[relation] = %+v, want a choice question with 5 options", relationQ)
	}
	sameSubjectQ, ok := req.Questions[QuestionSameSubject]
	if !ok || sameSubjectQ.Type != decide.QuestionNoul {
		t.Errorf("req.Questions[same_subject] = %+v, want a noul question", sameSubjectQ)
	}
	if err := req.Validate(); err != nil {
		t.Errorf("NewRequest(...).Validate() = %v, want nil", err)
	}
}

// TestStateTruncation pins State's D-09 composition (summary, blank line,
// content — or content alone) and truncation: exact rune-count truncation
// to maxChars, never splitting a multi-byte UTF-8 sequence, replacing an
// invalid UTF-8 byte with U+FFFD, and a non-positive maxChars falling back
// to DefaultStateChars.
func TestStateTruncation(t *testing.T) {
	if got, want := State("summary", "content", 1500), "summary\n\ncontent"; got != want {
		t.Errorf("State(summary, content, 1500) = %q, want %q", got, want)
	}
	if got, want := State("", "content only", 1500), "content only"; got != want {
		t.Errorf(`State("", content, 1500) = %q, want %q`, got, want)
	}

	long := strings.Repeat("x", 2000)
	if got := State("", long, 1500); utf8.RuneCountInString(got) != 1500 {
		t.Errorf("RuneCountInString(State(2000-char content, 1500)) = %d, want 1500", utf8.RuneCountInString(got))
	}

	// A cut landing on a multi-byte rune (é, then a 4-byte emoji) must
	// still produce valid UTF-8 with exactly maxChars runes.
	multiByte := "a" + strings.Repeat("é", 3) + strings.Repeat("😀", 3)
	for n := 1; n <= utf8.RuneCountInString(multiByte); n++ {
		got := State("", multiByte, n)
		if !utf8.ValidString(got) {
			t.Fatalf("State(multiByte, %d) = %q, not valid UTF-8", n, got)
		}
		if c := utf8.RuneCountInString(got); c != n {
			t.Errorf("RuneCountInString(State(multiByte, %d)) = %d, want %d", n, c, n)
		}
	}

	invalid := "valid" + string([]byte{0xff, 0xfe}) + "tail"
	if got := State("", invalid, 100); !utf8.ValidString(got) {
		t.Fatalf("State(invalid UTF-8, 100) = %q, not valid UTF-8", got)
	}

	for _, maxChars := range []int{0, -1} {
		got := State("", long, maxChars)
		if n := utf8.RuneCountInString(got); n != DefaultStateChars {
			t.Errorf("RuneCountInString(State(2000-char content, %d)) = %d, want DefaultStateChars %d", maxChars, n, DefaultStateChars)
		}
	}
}

// TestPairRequestOrdersNewerSecond proves PairRequest's ordering rule
// (D-06): the later CreatedAt is record_b whichever argument order it
// arrives in, and an exact CreatedAt tie puts the lexically larger ID
// second.
func TestPairRequestOrdersNewerSecond(t *testing.T) {
	earlier := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	later := earlier.Add(time.Hour)

	x := Side{ID: "id-x", CreatedAt: earlier, Content: "content-x"}
	y := Side{ID: "id-y", CreatedAt: later, Content: "content-y"}

	for _, req := range []decide.Request{PairRequest(x, y, 100), PairRequest(y, x, 100)} {
		if got, want := req.State["record_b"], State("", "content-y", 100); got != want {
			t.Errorf("record_b = %v, want %v (later CreatedAt)", got, want)
		}
		if got, want := req.State["record_a"], State("", "content-x", 100); got != want {
			t.Errorf("record_a = %v, want %v", got, want)
		}
	}

	// Equal CreatedAt: the lexically larger ID becomes record_b.
	same := earlier
	small := Side{ID: "id-a", CreatedAt: same, Content: "content-small"}
	large := Side{ID: "id-z", CreatedAt: same, Content: "content-large"}
	for _, args := range [][2]Side{{small, large}, {large, small}} {
		req := PairRequest(args[0], args[1], 100)
		if got, want := req.State["record_b"], State("", "content-large", 100); got != want {
			t.Errorf("record_b (tie) = %v, want %v (lexically larger ID)", got, want)
		}
	}
}

// relationAnswer builds a well-formed choice answer over the five D-05
// relations for FromResult tests below.
func relationAnswer(choice string, p Probabilities) decide.Answer {
	return decide.Answer{
		Type:   decide.QuestionChoice,
		Choice: choice,
		Probabilities: map[string]float64{
			Duplicate: p.Duplicate, Contradicts: p.Contradicts, Updates: p.Updates,
			Related: p.Related, Unrelated: p.Unrelated,
		},
	}
}

// TestFromResultThresholdBoundary pins D-08's strict-inequality boundary:
// p == threshold is never needs_review; one ulp below threshold always
// is; threshold 0 never flags; threshold 1 flags everything under 1.0.
func TestFromResultThresholdBoundary(t *testing.T) {
	resultAt := func(p float64) decide.Result {
		return decide.Result{Response: decide.Response{Answers: map[string]decide.Answer{
			QuestionRelation:    relationAnswer(Duplicate, Probabilities{Duplicate: p, Unrelated: 1 - p}),
			QuestionSameSubject: {Type: decide.QuestionNoul, Probability: 0.5},
		}}}
	}

	if v := FromResult(resultAt(0.9), 0.9); v.NeedsReview {
		t.Error("p == threshold (0.9) => needs_review true, want false")
	}
	below := math.Nextafter(0.9, 0)
	if v := FromResult(resultAt(below), 0.9); !v.NeedsReview {
		t.Errorf("p == %v (one ulp below threshold) => needs_review false, want true", below)
	}
	if v := FromResult(resultAt(0), 0); v.NeedsReview {
		t.Error("threshold 0, p=0 => needs_review true, want false (threshold 0 never flags)")
	}
	if v := FromResult(resultAt(0.999999), 1); !v.NeedsReview {
		t.Error("threshold 1, p=0.999999 => needs_review false, want true")
	}
	if v := FromResult(resultAt(1.0), 1); v.NeedsReview {
		t.Error("threshold 1, p=1.0 => needs_review true, want false")
	}
}

// TestFromResultCarriesValuesVerbatim proves probabilities that do not sum
// to 1 are carried unchanged (never renormalized), and SameSubject/Model
// are copied exactly as sent.
func TestFromResultCarriesValuesVerbatim(t *testing.T) {
	want := Probabilities{Duplicate: 0.1, Contradicts: 0.2, Updates: 0.3, Related: 0.35, Unrelated: 0.03}
	res := decide.Result{Response: decide.Response{
		Model: "typesafe/jev-1.13-20260917",
		Answers: map[string]decide.Answer{
			QuestionRelation:    relationAnswer(Related, want),
			QuestionSameSubject: {Type: decide.QuestionNoul, Probability: 0.87},
		},
	}}
	v := FromResult(res, 0.9)
	if v.Failed() {
		t.Fatalf("FromResult failed: %+v", v)
	}
	if v.Probabilities != want {
		t.Errorf("Probabilities = %+v, want %+v", v.Probabilities, want)
	}
	if v.SameSubject != 0.87 {
		t.Errorf("SameSubject = %v, want 0.87", v.SameSubject)
	}
	if v.Model != "typesafe/jev-1.13-20260917" {
		t.Errorf("Model = %q, want typesafe/jev-1.13-20260917", v.Model)
	}
}

// TestFromResultMalformed proves every malformed-answer edge FromResult
// must reject: an unrecognized choice, a relation answer missing one of
// the five probabilities, a missing relation or same_subject answer, and
// a same_subject answer of the wrong type. Each must yield ErrorClass
// "malformed_response" and no other populated field. RED against Task 1's
// FromResult, which copies answers verbatim without validating them —
// Task 3 adds the validation this test pins.
func TestFromResultMalformed(t *testing.T) {
	fullProbs := Probabilities{Duplicate: 0.9, Contradicts: 0.02, Updates: 0.02, Related: 0.03, Unrelated: 0.03}
	goodRelation := relationAnswer(Duplicate, fullProbs)
	goodSameSubject := decide.Answer{Type: decide.QuestionNoul, Probability: 0.5}

	incompleteProbs := decide.Answer{
		Type: decide.QuestionChoice, Choice: Duplicate,
		Probabilities: map[string]float64{Duplicate: 0.9, Contradicts: 0.02, Updates: 0.02, Related: 0.03},
	}

	cases := []struct {
		name    string
		answers map[string]decide.Answer
	}{
		{
			name: "unknown choice",
			answers: map[string]decide.Answer{
				QuestionRelation:    relationAnswer("bogus", fullProbs),
				QuestionSameSubject: goodSameSubject,
			},
		},
		{
			name: "missing probability",
			answers: map[string]decide.Answer{
				QuestionRelation:    incompleteProbs,
				QuestionSameSubject: goodSameSubject,
			},
		},
		{
			name: "missing relation answer",
			answers: map[string]decide.Answer{
				QuestionSameSubject: goodSameSubject,
			},
		},
		{
			name: "missing same_subject answer",
			answers: map[string]decide.Answer{
				QuestionRelation: goodRelation,
			},
		},
		{
			name: "same_subject wrong type",
			answers: map[string]decide.Answer{
				QuestionRelation:    goodRelation,
				QuestionSameSubject: {Type: decide.QuestionChoice, Choice: "x"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := decide.Result{Response: decide.Response{Answers: tc.answers}}
			v := FromResult(res, 0.9)
			if !v.Failed() {
				t.Fatalf("FromResult(%s) = %+v, want Failed()", tc.name, v)
			}
			if v.ErrorClass != "malformed_response" {
				t.Errorf("ErrorClass = %q, want malformed_response", v.ErrorClass)
			}
			if v.Relation != "" || v.SameSubject != 0 || v.NeedsReview || v.Model != "" || v.Probabilities != (Probabilities{}) {
				t.Errorf("FromResult(%s) = %+v, want no other populated field", tc.name, v)
			}
		})
	}
}

// TestErrorClass pins ErrorClass's full vocabulary: state_unavailable
// (bare and wrapped), a raw context.DeadlineExceeded mapped to timeout
// (decide.Status alone maps it to "error"), context.Canceled, every
// decide sentinel via its own decide.Status word, and an unknown error
// falling to "error".
func TestErrorClass(t *testing.T) {
	if got := ErrorClass(ErrStateUnavailable); got != "state_unavailable" {
		t.Errorf("ErrorClass(ErrStateUnavailable) = %q, want state_unavailable", got)
	}
	wrapped := fmt.Errorf("wrap: %w", ErrStateUnavailable)
	if got := ErrorClass(wrapped); got != "state_unavailable" {
		t.Errorf("ErrorClass(wrapped ErrStateUnavailable) = %q, want state_unavailable", got)
	}
	if got := ErrorClass(context.DeadlineExceeded); got != "timeout" {
		t.Errorf("ErrorClass(context.DeadlineExceeded) = %q, want timeout", got)
	}
	if got := ErrorClass(context.Canceled); got != "canceled" {
		t.Errorf("ErrorClass(context.Canceled) = %q, want canceled", got)
	}
	for _, sentinel := range []error{
		decide.ErrDecisionAuth, decide.ErrDecisionBadRequest, decide.ErrDecisionContextTooLarge,
		decide.ErrDecisionRateLimited, decide.ErrDecisionUnavailable, decide.ErrDecisionTimeout,
		decide.ErrDecisionResponseTooLarge, decide.ErrDecisionMalformedResponse, decide.ErrDecisionInvalidRequest,
	} {
		derr := &decide.Error{Kind: sentinel}
		if got, want := ErrorClass(derr), decide.Status(derr); got != want {
			t.Errorf("ErrorClass(%v) = %q, want decide.Status's word %q", sentinel, got, want)
		}
	}
	if got := ErrorClass(errors.New("boom")); got != "error" {
		t.Errorf("ErrorClass(unknown) = %q, want error", got)
	}
}
