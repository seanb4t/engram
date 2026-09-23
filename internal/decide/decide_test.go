// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package decide

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestQuestionConstructors(t *testing.T) {
	n := Noul("does it match?", "matches", "does not match")
	if n.Type != QuestionNoul || n.Instructions != "does it match?" || n.WhenTrue != "matches" || n.WhenFalse != "does not match" {
		t.Fatalf("Noul: unexpected fields: %+v", n)
	}
	if len(n.Options) != 0 || len(n.Scale) != 0 {
		t.Fatalf("Noul: non-noul fields set: %+v", n)
	}

	c := Choice("which one?", map[string]string{"a": "option a", "b": "option b"})
	if c.Type != QuestionChoice || c.Instructions != "which one?" {
		t.Fatalf("Choice: unexpected fields: %+v", c)
	}
	if len(c.Options) != 2 || c.WhenTrue != "" || c.WhenFalse != "" || len(c.Scale) != 0 {
		t.Fatalf("Choice: other-type fields set: %+v", c)
	}

	s := Score("how strong?", []string{"weak", "medium", "strong"})
	if s.Type != QuestionScore || s.Instructions != "how strong?" {
		t.Fatalf("Score: unexpected fields: %+v", s)
	}
	if len(s.Scale) != 3 || s.WhenTrue != "" || s.WhenFalse != "" || len(s.Options) != 0 {
		t.Fatalf("Score: other-type fields set: %+v", s)
	}
}

func manyOptions(n int) map[string]string {
	opts := make(map[string]string, n)
	for i := range n {
		opts[fmt.Sprintf("opt%d", i)] = "a description"
	}
	return opts
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		req     Request
		wantErr error // additionally checked against ErrDecisionInvalidRequest
	}{
		{
			name: "valid noul",
			req:  Request{Questions: map[string]Question{"q": Noul("i", "t", "f")}},
		},
		{
			name: "valid choice",
			req:  Request{Questions: map[string]Question{"q": Choice("i", map[string]string{"a": "d"})}},
		},
		{
			name: "valid score",
			req:  Request{Questions: map[string]Question{"q": Score("i", []string{"lo", "hi"})}},
		},
		{
			name: "255 options pass",
			req:  Request{Questions: map[string]Question{"q": Choice("i", manyOptions(255))}},
		},
		{
			name:    "256 options fail",
			req:     Request{Questions: map[string]Question{"q": Choice("i", manyOptions(256))}},
			wantErr: ErrTooManyChoices,
		},
		{
			name:    "choice no options",
			req:     Request{Questions: map[string]Question{"q": Choice("i", nil)}},
			wantErr: ErrEmptyCriteria,
		},
		{
			name:    "choice empty option description",
			req:     Request{Questions: map[string]Question{"q": Choice("i", map[string]string{"a": ""})}},
			wantErr: ErrEmptyCriteria,
		},
		{
			name:    "noul empty WhenFalse",
			req:     Request{Questions: map[string]Question{"q": {Type: QuestionNoul, Instructions: "i", WhenTrue: "t", WhenFalse: ""}}},
			wantErr: ErrEmptyCriteria,
		},
		{
			name:    "score no levels",
			req:     Request{Questions: map[string]Question{"q": Score("i", nil)}},
			wantErr: ErrEmptyCriteria,
		},
		{
			name:    "score empty level",
			req:     Request{Questions: map[string]Question{"q": Score("i", []string{"lo", ""})}},
			wantErr: ErrEmptyCriteria,
		},
		{
			name:    "unknown type",
			req:     Request{Questions: map[string]Question{"q": {Type: "bogus", Instructions: "i"}}},
			wantErr: ErrUnknownQuestionType,
		},
		{
			name:    "zero-value question",
			req:     Request{Questions: map[string]Question{"q": {}}},
			wantErr: ErrUnknownQuestionType,
		},
		{
			name:    "empty instructions",
			req:     Request{Questions: map[string]Question{"q": Noul("", "t", "f")}},
			wantErr: ErrEmptyInstructions,
		},
		{
			name:    "empty questions map",
			req:     Request{},
			wantErr: ErrNoQuestions,
		},
		{
			name:    "empty question name",
			req:     Request{Questions: map[string]Question{"": Noul("i", "t", "f")}},
			wantErr: ErrEmptyQuestionName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() = %v, want errors.Is match for %v", err, tt.wantErr)
			}
			if !errors.Is(err, ErrDecisionInvalidRequest) {
				t.Errorf("Validate() = %v, want errors.Is match for ErrDecisionInvalidRequest", err)
			}
		})
	}

	t.Run("two bad questions joined in sorted order", func(t *testing.T) {
		req := Request{Questions: map[string]Question{
			"b": {Type: "bogus"},
			"a": {Type: "bogus"},
		}}
		err := req.Validate()
		if err == nil {
			t.Fatal("Validate() = nil, want error")
		}
		if !errors.Is(err, ErrUnknownQuestionType) || !errors.Is(err, ErrDecisionInvalidRequest) {
			t.Fatalf("Validate() = %v, want both sentinels matchable", err)
		}
		text := err.Error()
		aIdx := strings.Index(text, `"a"`)
		bIdx := strings.Index(text, `"b"`)
		if aIdx == -1 || bIdx == -1 || aIdx > bIdx {
			t.Fatalf("Validate() text = %q, want %q before %q", text, "a", "b")
		}
	})
}

func TestRequestAdd(t *testing.T) {
	var r Request
	if err := r.Add("q1", Noul("i", "t", "f")); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if r.Questions == nil || len(r.Questions) != 1 {
		t.Fatalf("first Add: nil map not initialized: %+v", r.Questions)
	}

	if err := r.Add("q1", Noul("other", "t", "f")); !errors.Is(err, ErrDuplicateQuestion) {
		t.Fatalf("second Add same name: err = %v, want ErrDuplicateQuestion", err)
	}
	if r.Questions["q1"].Instructions != "i" {
		t.Fatalf("second Add: first question was overwritten: %+v", r.Questions["q1"])
	}

	if err := r.Add("", Noul("i", "t", "f")); !errors.Is(err, ErrEmptyQuestionName) {
		t.Fatalf("Add(\"\"): err = %v, want ErrEmptyQuestionName", err)
	}
}

func TestErrorUnwrap(t *testing.T) {
	e1 := &Error{Kind: ErrDecisionContextTooLarge, Status: 400}
	if !errors.Is(e1, ErrDecisionContextTooLarge) {
		t.Errorf("e1: want match for ErrDecisionContextTooLarge")
	}
	if !errors.Is(e1, ErrDecisionBadRequest) {
		t.Errorf("e1: want match for ErrDecisionBadRequest (parent class)")
	}

	e2 := &Error{Kind: ErrDecisionTimeout, Err: context.DeadlineExceeded}
	if !errors.Is(e2, ErrDecisionTimeout) {
		t.Errorf("e2: want match for ErrDecisionTimeout")
	}
	if !errors.Is(e2, context.DeadlineExceeded) {
		t.Errorf("e2: want match for context.DeadlineExceeded")
	}

	validationKinds := []error{
		ErrNoQuestions, ErrEmptyQuestionName, ErrDuplicateQuestion,
		ErrUnknownQuestionType, ErrEmptyInstructions, ErrEmptyCriteria,
		ErrTooManyChoices,
	}
	for _, kind := range validationKinds {
		e := &Error{Kind: kind, Question: "q"}
		if !errors.Is(e, ErrDecisionInvalidRequest) {
			t.Errorf("Kind=%v: want match for ErrDecisionInvalidRequest", kind)
		}
	}

	e3 := &Error{Kind: ErrDecisionBadRequest, Status: 400, Detail: "upstream said no", Question: "q1"}
	text := e3.Error()
	if !strings.Contains(text, "400") {
		t.Errorf("Error() = %q, want status 400", text)
	}
	if !strings.Contains(text, "upstream said no") {
		t.Errorf("Error() = %q, want Detail", text)
	}
	if strings.Contains(text, "some instructions never appear") {
		t.Errorf("Error() = %q, must never carry a Question's instructions text", text)
	}
}

func TestStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, "ok"},
		{"auth", &Error{Kind: ErrDecisionAuth}, "auth"},
		{"bad request", &Error{Kind: ErrDecisionBadRequest}, "bad_request"},
		{"context too large", &Error{Kind: ErrDecisionContextTooLarge}, "context_too_large"},
		{"rate limited", &Error{Kind: ErrDecisionRateLimited}, "rate_limited"},
		{"unavailable", &Error{Kind: ErrDecisionUnavailable}, "unavailable"},
		{"timeout", &Error{Kind: ErrDecisionTimeout}, "timeout"},
		{"response too large", &Error{Kind: ErrDecisionResponseTooLarge}, "response_too_large"},
		{"malformed response", &Error{Kind: ErrDecisionMalformedResponse}, "malformed_response"},
		{"invalid request", &Error{Kind: ErrNoQuestions}, "invalid_request"},
		{"canceled", context.Canceled, "canceled"},
		{"other", errors.New("boom"), "error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Status(tt.err); got != tt.want {
				t.Errorf("Status(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}
