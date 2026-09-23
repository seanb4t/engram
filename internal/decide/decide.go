// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package decide defines a provider-neutral, advisory-only typed-decision
// contract in System One vocabulary: shared state plus a batch of named
// Choice/Score/Noul questions in, typed probabilities out. This package
// performs no I/O itself — backends that reach a real decision provider over
// the network live in internal/decide/<name> (jev now, an emulator later,
// DEC-F2).
package decide

import "context"

// Decider answers a batch of typed questions against shared state.
//
// A decision failure never fails the surrounding read or sweep: callers treat
// an error as "no decision" and continue.
// Decisions are advisory: callers surface answers and never act on them
// automatically.
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
// returns a single P(true) in [0, 1]. Plan 02-04 adds QuestionChoice and
// QuestionScore.
const QuestionNoul QuestionType = "noul"

// Question is one typed question asked against a Request's State.
// WhenTrue and WhenFalse are the noul criteria descriptions: short prose
// telling the provider what "true" and "false" mean for this question.
type Question struct {
	Type         QuestionType
	Instructions string
	WhenTrue     string
	WhenFalse    string
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

// Request batches every question about one State into a single call —
// questions are answered in parallel by the provider and cannot see each
// other, so batching everything about one State into one Request is both
// cheaper and the only way to get answers that share the same context.
type Request struct {
	State     State
	Questions map[string]Question
}

// Answer is one question's typed result. For a noul question, Probability is
// P(true), carried verbatim from the provider — never rounded or
// renormalized.
type Answer struct {
	Type        QuestionType
	Probability float64
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
