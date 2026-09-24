// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package verdict defines the advisory relation-verdict contract that
// spine-review consolidate and the curation eval share: the D-06 question
// set, the D-09 per-record state, pair ordering, and mapping a
// decide.Result to a Verdict. This package performs no I/O and never acts
// on a verdict — callers surface it, never mutate a record because of it.
package verdict

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/seanb4t/engram/internal/decide"
)

// The five D-05/D-06 relation names, in D-05's key order.
const (
	Duplicate   = "duplicate"
	Contradicts = "contradicts"
	Updates     = "updates"
	Related     = "related"
	Unrelated   = "unrelated"
)

// Relations returns a fresh slice of the five relation names in D-05's key
// order. Mutating the returned slice never affects a later call.
func Relations() []string {
	return []string{Duplicate, Contradicts, Updates, Related, Unrelated}
}

// RelationCriteria returns a fresh map of relation name to its D-06
// criteria description: duplicate/contradicts/related/unrelated verbatim
// from the spike blueprint, updates verbatim from D-06. Mutating the
// returned map never affects a later call.
func RelationCriteria() map[string]string {
	return map[string]string{
		Duplicate:   "Both state the same fact; keeping both is redundant (one may be more complete).",
		Contradicts: "They make incompatible claims about the same subject; one corrects or reverses the other.",
		Updates:     "B is a newer state or a more complete version of the same fact.",
		Related:     "Same subject area, but different and compatible facts; both are worth keeping.",
		Unrelated:   "Different subjects.",
	}
}

// Question names, instructions and same_subject criteria (D-06). One
// request per pair carries exactly these two questions.
const (
	QuestionRelation    = "relation"
	QuestionSameSubject = "same_subject"

	RelationInstructions    = "How does record_b relate to record_a?"
	SameSubjectInstructions = "Are both records about the same specific subject?"
	SameSubjectTrue         = "both records are about the same specific subject"
	SameSubjectFalse        = "the records are about different subjects"

	// StateContext is the fixed context sentence sent with every request
	// (D-06). It carries no record content — only the framing a decider
	// needs to interpret record_a/record_b.
	StateContext = "record_a and record_b are two entries from a software team's durable project memory; record_b was created after record_a."
)

// DefaultThreshold is the D-08 default below which a verdict's relation
// probability is flagged needs_review. Plan 03-02 registers the operator
// knob whose default must equal this value.
const DefaultThreshold = 0.9

// DefaultStateChars is the D-09 default per-record state truncation
// length, in Unicode code points. Plan 03-02 registers the operator knob
// whose default must equal this value.
const DefaultStateChars = 1500

// State builds one side's decision-state text (D-09): summary, a blank
// line, then content when summary is non-empty; content alone otherwise.
// The result is truncated to maxChars Unicode code points — truncation
// never splits a multi-byte UTF-8 sequence, and an invalid UTF-8 byte
// becomes U+FFFD, counting as one code point. maxChars at or below zero
// uses DefaultStateChars.
func State(summary, content string, maxChars int) string {
	if maxChars <= 0 {
		maxChars = DefaultStateChars
	}
	full := content
	if summary != "" {
		full = summary + "\n\n" + content
	}
	return truncateRunes(full, maxChars)
}

// truncateRunes returns s truncated to at most maxChars runes, decoding an
// invalid UTF-8 byte as U+FFFD (one rune) exactly like Go's range over a
// string does — the result is always valid UTF-8.
func truncateRunes(s string, maxChars int) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n >= maxChars {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// Side is one record's identity and content for PairRequest's ordering.
type Side struct {
	ID        string
	CreatedAt time.Time
	Summary   string
	Content   string
}

// PairRequest builds a decide.Request for x and y (D-06): the side with
// the later CreatedAt becomes record_b — on an exact tie, the lexically
// larger ID becomes record_b — so a decider's "updates" criterion ("B is a
// newer state…") is meaningful regardless of the pair's argument order.
func PairRequest(x, y Side, maxChars int) decide.Request {
	a, b := x, y
	if xBecomesB(x, y) {
		a, b = y, x
	}
	return NewRequest(State(a.Summary, a.Content, maxChars), State(b.Summary, b.Content, maxChars))
}

// xBecomesB reports whether x should become record_b ahead of y: a
// strictly later CreatedAt, or — on an exact tie — a lexically larger ID.
func xBecomesB(x, y Side) bool {
	switch {
	case x.CreatedAt.After(y.CreatedAt):
		return true
	case x.CreatedAt.Equal(y.CreatedAt):
		return x.ID > y.ID
	default:
		return false
	}
}

// NewRequest builds the D-06 decide.Request for one pair's already-built
// state text: the fixed context sentence, the two record states, a
// relation choice question over the five criteria, and a same_subject
// noul question. One request per pair (D-06) — never split across calls.
func NewRequest(recordA, recordB string) decide.Request {
	return decide.Request{
		State: decide.State{
			"context":  StateContext,
			"record_a": recordA,
			"record_b": recordB,
		},
		Questions: map[string]decide.Question{
			QuestionRelation:    decide.Choice(RelationInstructions, RelationCriteria()),
			QuestionSameSubject: decide.Noul(SameSubjectInstructions, SameSubjectTrue, SameSubjectFalse),
		},
	}
}

// Probabilities is the D-05 five-key probability distribution over the
// relation choice, carried verbatim from the provider — never rounded,
// clamped or renormalized.
type Probabilities struct {
	Duplicate, Contradicts, Updates, Related, Unrelated float64
}

// Of returns the probability for relation, and false when relation is not
// one of the five D-05 names.
func (p Probabilities) Of(relation string) (float64, bool) {
	switch relation {
	case Duplicate:
		return p.Duplicate, true
	case Contradicts:
		return p.Contradicts, true
	case Updates:
		return p.Updates, true
	case Related:
		return p.Related, true
	case Unrelated:
		return p.Unrelated, true
	default:
		return 0, false
	}
}

// Verdict is one pair's advisory relation verdict (D-05): the decider's
// choice, the full probability distribution, the same_subject probability,
// whether the relation probability fell below threshold, and the model
// snapshot that produced it. ErrorClass is non-empty exactly when the
// verdict failed (see Failed) — every other field is then the zero value.
type Verdict struct {
	Relation      string
	Probabilities Probabilities
	SameSubject   float64
	NeedsReview   bool
	Model         string
	ErrorClass    string
}

// Failed reports whether v carries a failure class instead of a decided
// relation.
func (v Verdict) Failed() bool {
	return v.ErrorClass != ""
}

// ErrStateUnavailable marks a pair whose record state could not be
// fetched, or whose state fetch itself failed.
var ErrStateUnavailable = errors.New("record state unavailable")

// ErrorClass classifies err into decide.Status's vocabulary, plus
// state_unavailable for ErrStateUnavailable and timeout for a raw
// context.DeadlineExceeded — what DecideMany stores for an item it never
// started once its context is done, which decide.Status alone maps to
// "error" rather than "timeout".
func ErrorClass(err error) string {
	switch {
	case errors.Is(err, ErrStateUnavailable):
		return "state_unavailable"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return decide.Status(err)
	}
}

// Unavailable returns a Verdict carrying only err's ErrorClass.
func Unavailable(err error) Verdict {
	return Verdict{ErrorClass: ErrorClass(err)}
}

// FromResult maps one decide.Result to a Verdict (D-05): a non-nil
// res.Err returns Unavailable(res.Err); otherwise the relation answer's
// choice and its five probabilities are copied verbatim (no rounding,
// clamping or renormalizing), SameSubject from the same_subject noul's
// Probability, Model from res.Response.Model, and NeedsReview is
// probabilities[relation] strictly below threshold (D-08) —
// probabilities[relation] is the single source of truth, never re-derived
// locally.
func FromResult(res decide.Result, threshold float64) Verdict {
	if res.Err != nil {
		return Unavailable(res.Err)
	}
	relation := res.Response.Answers[QuestionRelation]
	sameSubject := res.Response.Answers[QuestionSameSubject]
	probs := Probabilities{
		Duplicate:   relation.Probabilities[Duplicate],
		Contradicts: relation.Probabilities[Contradicts],
		Updates:     relation.Probabilities[Updates],
		Related:     relation.Probabilities[Related],
		Unrelated:   relation.Probabilities[Unrelated],
	}
	p := relation.Probabilities[relation.Choice]
	return Verdict{
		Relation:      relation.Choice,
		Probabilities: probs,
		SameSubject:   sameSubject.Probability,
		NeedsReview:   p < threshold,
		Model:         res.Response.Model,
	}
}
