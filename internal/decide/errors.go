// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package decide

import (
	"context"
	"errors"
	"fmt"
)

// The D-12 status-class sentinels. Every backend under internal/decide/*
// classifies its failures into exactly these classes so callers branch with
// errors.Is, never on error message text.
var (
	// ErrDecisionAuth covers HTTP 401, 402 and 403: the credential or
	// account cannot be used, and only an operator action (rotating the key,
	// resolving billing) fixes it.
	ErrDecisionAuth = errors.New("authentication or account error")
	// ErrDecisionBadRequest covers HTTP 400 and any other 4xx not otherwise
	// classified. The wrapping *Error's Detail carries the upstream text.
	ErrDecisionBadRequest = errors.New("bad request")
	// ErrDecisionContextTooLarge covers HTTP 400 max_tokens_exceeded
	// (detected structurally where possible) and 413. It is a refinement of
	// ErrDecisionBadRequest: a *Error with this Kind also matches
	// ErrDecisionBadRequest via Unwrap.
	ErrDecisionContextTooLarge = errors.New("request context too large")
	// ErrDecisionRateLimited covers HTTP 429.
	ErrDecisionRateLimited = errors.New("rate limited")
	// ErrDecisionUnavailable covers HTTP 5xx and transport failures other
	// than timeouts.
	ErrDecisionUnavailable = errors.New("provider unavailable")
	// ErrDecisionTimeout covers the call's deadline being exceeded.
	ErrDecisionTimeout = errors.New("timed out")
	// ErrDecisionResponseTooLarge covers a success body that exceeded its
	// byte bound.
	ErrDecisionResponseTooLarge = errors.New("response too large")
	// ErrDecisionMalformedResponse covers a 200 body that does not decode,
	// or that lacks an answer for a requested question. Not one of D-12's
	// originally listed classes: a decode failure still needs a named class
	// so callers can errors.Is against it instead of matching error text.
	ErrDecisionMalformedResponse = errors.New("malformed response")
)

// The D-09 structural-validation sentinels. Request.Validate returns these,
// joined via errors.Join, with no network call.
var (
	// ErrDecisionInvalidRequest is the umbrella every D-09 validation
	// sentinel below also matches via Unwrap.
	ErrDecisionInvalidRequest = errors.New("invalid request")
	// ErrNoQuestions: a Request with an empty Questions map.
	ErrNoQuestions = errors.New("no questions")
	// ErrEmptyQuestionName: a Questions map key of "".
	ErrEmptyQuestionName = errors.New("empty question name")
	// ErrDuplicateQuestion: Request.Add called twice with the same name.
	ErrDuplicateQuestion = errors.New("duplicate question name")
	// ErrUnknownQuestionType: a Question.Type that is not noul, choice or
	// score — including the zero value.
	ErrUnknownQuestionType = errors.New("unknown question type")
	// ErrEmptyInstructions: a Question with empty Instructions.
	ErrEmptyInstructions = errors.New("empty instructions")
	// ErrEmptyCriteria: a noul missing WhenTrue/WhenFalse, a choice with no
	// options or an empty option description, or a score with no levels or
	// an empty level description.
	ErrEmptyCriteria = errors.New("empty criteria")
	// ErrTooManyChoices: a choice question with more than MaxChoices
	// options.
	ErrTooManyChoices = errors.New("too many choices")
)

// Error is a backend failure classified into one of the D-12/D-09 sentinels
// via Kind. Status is the HTTP status the backend observed (0 when none,
// e.g. a pure validation or transport failure). Detail is bounded upstream
// text (at most 4096 bytes) set by the backend. Question, when set, is a
// question NAME — never its instructions or criteria text.
type Error struct {
	Kind     error
	Status   int
	Detail   string
	Question string
	Err      error
}

// Error implements the error interface: "decide: <Kind text>", then
// ": question %q" if Question is set, ": status %d" if Status is nonzero,
// ": <Detail>" if Detail is set, and ": <Err>" if Err is set.
func (e *Error) Error() string {
	s := "decide: " + e.Kind.Error()
	if e.Question != "" {
		s += fmt.Sprintf(": question %q", e.Question)
	}
	if e.Status != 0 {
		s += fmt.Sprintf(": status %d", e.Status)
	}
	if e.Detail != "" {
		s += ": " + e.Detail
	}
	if e.Err != nil {
		s += ": " + e.Err.Error()
	}
	return s
}

// Unwrap returns the non-nil values among Kind, Kind's parent class (D-12's
// ErrDecisionBadRequest for ErrDecisionContextTooLarge, D-09's
// ErrDecisionInvalidRequest for every validation sentinel) and Err — so a
// caller's errors.Is against either the specific Kind or its class matches.
func (e *Error) Unwrap() []error {
	errs := make([]error, 0, 3)
	if e.Kind != nil {
		errs = append(errs, e.Kind)
	}
	if parent := parentClass(e.Kind); parent != nil {
		errs = append(errs, parent)
	}
	if e.Err != nil {
		errs = append(errs, e.Err)
	}
	return errs
}

// errParents maps a Kind sentinel to its broader class for Unwrap. A map
// lookup (rather than a switch on error values) sidesteps errorlint's
// wrapped-error warning — these are always the exact package-level sentinel
// values, never wrapped, but a switch on error still reads as the unsafe
// pattern to static analysis.
var errParents = map[error]error{
	ErrDecisionContextTooLarge: ErrDecisionBadRequest,
	ErrNoQuestions:             ErrDecisionInvalidRequest,
	ErrEmptyQuestionName:       ErrDecisionInvalidRequest,
	ErrDuplicateQuestion:       ErrDecisionInvalidRequest,
	ErrUnknownQuestionType:     ErrDecisionInvalidRequest,
	ErrEmptyInstructions:       ErrDecisionInvalidRequest,
	ErrEmptyCriteria:           ErrDecisionInvalidRequest,
	ErrTooManyChoices:          ErrDecisionInvalidRequest,
}

// parentClass returns kind's broader class for Unwrap, or nil when kind has
// no parent.
func parentClass(kind error) error {
	return errParents[kind]
}

// Status returns the fixed, lowercase word backends use for the
// engram.decide.status span attribute (D-13). nil maps to "ok";
// context.Canceled maps to "canceled"; each D-12/D-09 class maps to its own
// word; anything else maps to "error". ErrDecisionContextTooLarge is
// checked before ErrDecisionBadRequest since the former's Unwrap chain also
// matches the latter.
func Status(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, ErrDecisionContextTooLarge):
		return "context_too_large"
	case errors.Is(err, ErrDecisionAuth):
		return "auth"
	case errors.Is(err, ErrDecisionBadRequest):
		return "bad_request"
	case errors.Is(err, ErrDecisionRateLimited):
		return "rate_limited"
	case errors.Is(err, ErrDecisionUnavailable):
		return "unavailable"
	case errors.Is(err, ErrDecisionTimeout):
		return "timeout"
	case errors.Is(err, ErrDecisionResponseTooLarge):
		return "response_too_large"
	case errors.Is(err, ErrDecisionMalformedResponse):
		return "malformed_response"
	case errors.Is(err, ErrDecisionInvalidRequest):
		return "invalid_request"
	default:
		return "error"
	}
}
