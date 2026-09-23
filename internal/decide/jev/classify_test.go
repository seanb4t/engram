// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package jev

import (
	"context"
	"errors"
	"net"
	"strings"
	"syscall"
	"testing"

	"github.com/seanb4t/engram/internal/decide"
)

// TestJevErrorClassification proves D-12: every status, both error dialects
// and non-JSON bodies classify into the shared decide.* vocabulary by HTTP
// status alone, never by decoding or substring-matching the error `code` or
// `message` fields for control flow (except the single structural
// max_tokens_exceeded exception D-12 allows).
func TestJevErrorClassification(t *testing.T) {
	type tc struct {
		name       string
		status     int
		body       string
		wantKind   error
		notKind    error
		wantStatus int
		wantDetail string
	}

	cases := []tc{
		{name: "openrouter401", status: 401, body: fixtureOpenRouter401, wantKind: decide.ErrDecisionAuth, wantStatus: 401},
		{name: "litellm401_string_code", status: 401, body: fixtureLiteLLM401, wantKind: decide.ErrDecisionAuth, wantStatus: 401, wantDetail: "Authentication Error, No api key passed in."},
		{name: "litellm403_synthetic", status: 403, body: fixtureLiteLLM403, wantKind: decide.ErrDecisionAuth, wantStatus: 403},
		{name: "numeric402_synthetic", status: 402, body: fixtureOpenRouter402, wantKind: decide.ErrDecisionAuth, wantStatus: 402},
		{name: "openrouter400_choices", status: 400, body: fixtureOpenRouter400Choices, wantKind: decide.ErrDecisionBadRequest, notKind: decide.ErrDecisionContextTooLarge, wantStatus: 400},
		{name: "openrouter400_badtype", status: 400, body: fixtureOpenRouter400BadType, wantKind: decide.ErrDecisionBadRequest, notKind: decide.ErrDecisionContextTooLarge, wantStatus: 400},
		{name: "openrouter400_maxtokens", status: 400, body: fixtureOpenRouter400MaxTokens, wantKind: decide.ErrDecisionContextTooLarge, wantStatus: 400},
		{name: "openrouter400_chatpath", status: 400, body: fixtureChatPath400, wantKind: decide.ErrDecisionBadRequest, notKind: decide.ErrDecisionContextTooLarge, wantStatus: 400},
		{
			name:     "synthetic400_top_level_detail_object",
			status:   400,
			body:     `{"detail":{"error_type":"max_tokens_exceeded"}}`,
			wantKind: decide.ErrDecisionContextTooLarge, wantStatus: 400,
		},
		{
			name:     "synthetic400_prose_no_json_never_substring_matched",
			status:   400,
			body:     `{"error":{"message":"the request hit max_tokens_exceeded somewhere","code":400}}`,
			wantKind: decide.ErrDecisionBadRequest, notKind: decide.ErrDecisionContextTooLarge, wantStatus: 400,
		},
		{
			name:     "synthetic400_string_code_never_reaches_control_flow",
			status:   400,
			body:     `{"error":{"message":"bad request","code":"401"}}`,
			wantKind: decide.ErrDecisionBadRequest, notKind: decide.ErrDecisionContextTooLarge, wantStatus: 400,
		},
		{name: "413_context_too_large", status: 413, body: fixtureOpenRouter413, wantKind: decide.ErrDecisionContextTooLarge, wantStatus: 413},
		{name: "404_other_4xx_is_bad_request", status: 404, body: fixtureOpenRouter404, wantKind: decide.ErrDecisionBadRequest, wantStatus: 404},
		{name: "429_rate_limited", status: 429, body: fixtureOpenRouter429, wantKind: decide.ErrDecisionRateLimited, wantStatus: 429},
		{name: "500_unavailable", status: 500, body: fixtureOpenRouter500, wantKind: decide.ErrDecisionUnavailable, wantStatus: 500},
		{name: "502_unavailable", status: 502, body: fixtureOpenRouter502, wantKind: decide.ErrDecisionUnavailable, wantStatus: 502},
		{name: "503_unavailable", status: 503, body: fixtureOpenRouter503, wantKind: decide.ErrDecisionUnavailable, wantStatus: 503},
		{name: "524_unavailable", status: 524, body: fixtureOpenRouter524, wantKind: decide.ErrDecisionUnavailable, wantStatus: 524},
		{name: "529_unavailable", status: 529, body: fixtureOpenRouter529, wantKind: decide.ErrDecisionUnavailable, wantStatus: 529},
		{name: "html502_nonjson_body", status: 502, body: fixtureHTML502, wantKind: decide.ErrDecisionUnavailable, wantStatus: 502, wantDetail: strings.TrimSpace(fixtureHTML502)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := classifyStatus(c.status, []byte(c.body))
			if !errors.Is(err, c.wantKind) {
				t.Fatalf("classifyStatus(%d, ...) = %v, want errors.Is match for %v", c.status, err, c.wantKind)
			}
			if c.notKind != nil && errors.Is(err, c.notKind) {
				t.Errorf("classifyStatus(%d, ...) unexpectedly matches %v too", c.status, c.notKind)
			}
			var de *decide.Error
			if !errors.As(err, &de) {
				t.Fatalf("classifyStatus(%d, ...) = %v, want *decide.Error", c.status, err)
			}
			if de.Status != c.wantStatus {
				t.Errorf("Status = %d, want %d", de.Status, c.wantStatus)
			}
			if len(de.Detail) > maxErrorBodyBytes {
				t.Errorf("len(Detail) = %d, want <= %d", len(de.Detail), maxErrorBodyBytes)
			}
			if c.wantDetail != "" && de.Detail != c.wantDetail {
				t.Errorf("Detail = %q, want %q", de.Detail, c.wantDetail)
			}
		})
	}

	t.Run("20kib_body_gives_bounded_detail", func(t *testing.T) {
		big := strings.Repeat("e", 20*1024)
		err := classifyStatus(500, []byte(big))
		var de *decide.Error
		if !errors.As(err, &de) {
			t.Fatalf("classifyStatus(500, <20KiB>) = %v, want *decide.Error", err)
		}
		if len(de.Detail) > maxErrorBodyBytes {
			t.Errorf("len(Detail) = %d, want <= %d", len(de.Detail), maxErrorBodyBytes)
		}
		if len(de.Detail) == 0 {
			t.Error("Detail is empty, want a non-empty bounded slice of the body")
		}
	})

	t.Run("transport", func(t *testing.T) {
		t.Run("deadline_exceeded", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 0)
			defer cancel()
			<-ctx.Done()
			err := classifyTransport(ctx, ctx.Err())
			if !errors.Is(err, decide.ErrDecisionTimeout) {
				t.Errorf("got %v, want ErrDecisionTimeout", err)
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("got %v, want it to also match context.DeadlineExceeded", err)
			}
		})
		t.Run("net_error_timeout_true", func(t *testing.T) {
			err := classifyTransport(context.Background(), &net.OpError{Op: "dial", Err: fakeTimeoutErr{}})
			if !errors.Is(err, decide.ErrDecisionTimeout) {
				t.Errorf("got %v, want ErrDecisionTimeout", err)
			}
		})
		t.Run("caller_canceled_passes_through", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err := classifyTransport(ctx, ctx.Err())
			if !errors.Is(err, context.Canceled) {
				t.Errorf("got %v, want context.Canceled", err)
			}
			if errors.Is(err, decide.ErrDecisionTimeout) || errors.Is(err, decide.ErrDecisionUnavailable) {
				t.Errorf("got %v, want no ErrDecision* class on caller cancellation", err)
			}
		})
		t.Run("connection_refused_is_unavailable", func(t *testing.T) {
			err := classifyTransport(context.Background(), &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED})
			if !errors.Is(err, decide.ErrDecisionUnavailable) {
				t.Errorf("got %v, want ErrDecisionUnavailable", err)
			}
		})
	})

	t.Run("retryable", func(t *testing.T) {
		if !isRetryable(&decide.Error{Kind: decide.ErrDecisionRateLimited}) {
			t.Error("RateLimited should be retryable")
		}
		if !isRetryable(&decide.Error{Kind: decide.ErrDecisionUnavailable}) {
			t.Error("Unavailable should be retryable")
		}
		if isRetryable(&decide.Error{Kind: decide.ErrDecisionAuth}) {
			t.Error("Auth should not be retryable")
		}
		if isRetryable(&decide.Error{Kind: decide.ErrDecisionBadRequest}) {
			t.Error("BadRequest should not be retryable")
		}
		if isRetryable(&decide.Error{Kind: decide.ErrDecisionContextTooLarge}) {
			t.Error("ContextTooLarge should not be retryable")
		}
		if isRetryable(&decide.Error{Kind: decide.ErrDecisionTimeout}) {
			t.Error("Timeout should not be retryable")
		}
	})
}

// fakeTimeoutErr implements net.Error with Timeout() true, for the
// net_error_timeout_true transport subtest.
type fakeTimeoutErr struct{}

func (fakeTimeoutErr) Error() string   { return "fake timeout" }
func (fakeTimeoutErr) Timeout() bool   { return true }
func (fakeTimeoutErr) Temporary() bool { return true } // net.Error still requires this method; deprecated but not removed.
