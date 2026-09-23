// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package decide defines a provider-neutral, advisory-only typed-decision
// contract in System One vocabulary: shared state plus a batch of named
// Choice/Score/Noul questions in, typed probabilities out. This package
// performs no I/O itself — backends that reach a real decision provider over
// the network live in internal/decide/<name> (jev now, an emulator later,
// DEC-F2).
package decide

import (
	"context"
	"encoding/json"
)

// Decider answers a batch of typed questions against shared state.
//
// A decision failure never fails the surrounding read or sweep: callers treat
// an error as "no decision" and continue.
// Decisions are advisory: callers surface answers and never act on them
// automatically. Backends return the named D-12 failure classes from
// errors.go, so callers branch with errors.Is, never on error message text.
type Decider interface {
	Decide(ctx context.Context, req Request) (Response, error)
}

// State is the shared context every question in a Request is evaluated
// against — arbitrary caller-supplied key/value pairs (e.g. two candidate
// memory records being compared).
type State map[string]any

// QuestionType names the shape of a Question and its matching Answer.
type QuestionType string

// QuestionNoul is a binary (true/false) probability question: the provider
// returns a single P(true) in [0, 1].
const QuestionNoul QuestionType = "noul"

// QuestionChoice is a single-select question over a named set of options:
// the provider returns the chosen option plus a per-option probability
// distribution.
const QuestionChoice QuestionType = "choice"

// QuestionScore is an ordered-level question: the provider returns a
// numeric score plus a per-level probability distribution keyed by level
// index.
const QuestionScore QuestionType = "score"

// Question is one typed question asked against a Request's State. Only the
// fields for Type are sent to the provider: WhenTrue/WhenFalse for noul,
// Options for choice, Scale for score.
type Question struct {
	Type         QuestionType
	Instructions string
	// WhenTrue and WhenFalse are the noul criteria descriptions: short prose
	// telling the provider what "true" and "false" mean for this question.
	WhenTrue  string
	WhenFalse string
	// Options maps a choice question's option name to its description.
	Options map[string]string
	// Scale holds a score question's ordered level descriptions; the slice
	// index is the level.
	Scale []string
}

// Noul builds a noul (binary-probability) Question from its instructions and
// the criteria descriptions for the true and false outcomes.
func Noul(instructions, whenTrue, whenFalse string) Question {
	return Question{
		Type:         QuestionNoul,
		Instructions: instructions,
		WhenTrue:     whenTrue,
		WhenFalse:    whenFalse,
	}
}

// Choice builds a choice (single-select) Question from its instructions and
// a map of option name to description.
func Choice(instructions string, options map[string]string) Question {
	return Question{
		Type:         QuestionChoice,
		Instructions: instructions,
		Options:      options,
	}
}

// Score builds a score (ordered-level) Question from its instructions and
// an ordered slice of level descriptions.
func Score(instructions string, scale []string) Question {
	return Question{
		Type:         QuestionScore,
		Instructions: instructions,
		Scale:        scale,
	}
}

// Request batches every question about one State into a single call —
// questions are answered in parallel by the provider and cannot see each
// other, so batching everything about one State into one Request is both
// cheaper and the only way to get answers that share the same context.
type Request struct {
	State     State
	Questions map[string]Question
}

// Answer is one question's typed result. For a noul question, Probability is
// P(true). For a choice question, Choice is the selected option name and
// Probabilities maps every option name to its probability. For a score
// question, Score is the numeric result and Probabilities maps each level
// index — exactly as the provider sends it ("0", "1", …) — to its
// probability; Legend is the provider's verbatim, uninterpreted score
// legend. Every number here is carried verbatim as float64: never rounded,
// clamped or renormalized (E03). Confidence and Legend are nil when the
// provider omits them, never zero.
type Answer struct {
	Type        QuestionType
	Probability float64
	// Choice is the selected option name (choice answers only).
	Choice string
	// Score is the numeric result (score answers only).
	Score float64
	// Confidence is the provider's confidence in this answer (choice and
	// score answers). Nil when the provider omits it.
	Confidence *float64
	// Probabilities maps option name (choice) or level index (score) to its
	// probability, carried verbatim.
	Probabilities map[string]float64
	// Legend is the provider's verbatim, uninterpreted score legend (score
	// answers only). Nil when absent.
	Legend map[string]json.RawMessage
}

// Result is one Request's outcome from DecideMany: exactly one of Response
// or Err is meaningful, matching the batch element it was produced for.
type Result struct {
	Response Response
	Err      error
}

// Usage reports token and cost accounting for one Decide call. CostUSD is nil
// when the provider omits cost.
type Usage struct {
	InputTokens  int64
	OutputTokens int64
	CostUSD      *float64
}

// Response is the typed result of one Decide call. Model is the dated
// snapshot the provider reports (e.g. "typesafe/jev-1.13-20260917"), distinct
// from the pinned model requested. Usage is nil when the provider omits it.
type Response struct {
	Answers  map[string]Answer
	Model    string
	ID       string
	Provider string
	Usage    *Usage
}
