// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package decide

import (
	"errors"
	"sort"
)

// MaxChoices bounds the number of options a single choice question may
// carry (D-09) — the one cardinality the spike verified live: 255 options
// validate, 256 are rejected by Validate before any network call.
const MaxChoices = 255

// Validate performs D-09's cheap, no-I/O structural checks: it returns nil
// when r is valid, else the errors.Join of *Error values (one per violated
// rule), visiting question names in sorted order so the joined error text is
// deterministic. No token counting happens here (the 32k-context limit is
// left to callers and the server's own max_tokens_exceeded response).
func (r Request) Validate() error {
	if len(r.Questions) == 0 {
		return &Error{Kind: ErrNoQuestions}
	}

	names := make([]string, 0, len(r.Questions))
	for name := range r.Questions {
		names = append(names, name)
	}
	sort.Strings(names)

	var errs []error
	for _, name := range names {
		q := r.Questions[name]
		if name == "" {
			errs = append(errs, &Error{Kind: ErrEmptyQuestionName})
			continue
		}
		switch q.Type {
		case QuestionNoul:
			errs = append(errs, validateInstructions(name, q)...)
			if q.WhenTrue == "" || q.WhenFalse == "" {
				errs = append(errs, &Error{Kind: ErrEmptyCriteria, Question: name})
			}
		case QuestionChoice:
			errs = append(errs, validateInstructions(name, q)...)
			errs = append(errs, validateChoiceOptions(name, q.Options)...)
		case QuestionScore:
			errs = append(errs, validateInstructions(name, q)...)
			errs = append(errs, validateScale(name, q.Scale)...)
		default:
			errs = append(errs, &Error{Kind: ErrUnknownQuestionType, Question: name})
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

func validateInstructions(name string, q Question) []error {
	if q.Instructions == "" {
		return []error{&Error{Kind: ErrEmptyInstructions, Question: name}}
	}
	return nil
}

func validateChoiceOptions(name string, options map[string]string) []error {
	var errs []error
	if len(options) == 0 {
		errs = append(errs, &Error{Kind: ErrEmptyCriteria, Question: name})
	} else {
		for _, description := range options {
			if description == "" {
				errs = append(errs, &Error{Kind: ErrEmptyCriteria, Question: name})
				break
			}
		}
	}
	if len(options) > MaxChoices {
		errs = append(errs, &Error{Kind: ErrTooManyChoices, Question: name})
	}
	return errs
}

func validateScale(name string, scale []string) []error {
	if len(scale) == 0 {
		return []error{&Error{Kind: ErrEmptyCriteria, Question: name}}
	}
	for _, level := range scale {
		if level == "" {
			return []error{&Error{Kind: ErrEmptyCriteria, Question: name}}
		}
	}
	return nil
}

// Add appends one named question to r, initializing a nil Questions map on
// the first call. An empty name returns ErrEmptyQuestionName. A name already
// present returns ErrDuplicateQuestion and leaves the existing question
// untouched — this is what makes D-09's duplicate-name rule enforceable
// against D-10's map[name]Question shape.
func (r *Request) Add(name string, q Question) error {
	if name == "" {
		return &Error{Kind: ErrEmptyQuestionName}
	}
	if r.Questions == nil {
		r.Questions = make(map[string]Question)
	}
	if _, exists := r.Questions[name]; exists {
		return &Error{Kind: ErrDuplicateQuestion, Question: name}
	}
	r.Questions[name] = q
	return nil
}
