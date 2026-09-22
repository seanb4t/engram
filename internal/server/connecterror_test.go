// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/seanb4t/engram/internal/store"
)

// TestConnectError is the mapper's own acceptance table: every mapping arm
// listed on connectError's doc comment, matched on the TYPED sentinel (never a
// string), plus the CodeInternal generic-message no-leak guarantee (T-17-12).
func TestConnectError(t *testing.T) {
	ctx := context.Background()

	t.Run("nil_is_nil", func(t *testing.T) {
		if got := connectError(ctx, nil); got != nil {
			t.Fatalf("connectError(ctx, nil) = %v, want nil", got)
		}
	})

	// Stand-ins for the wrapped ErrInvalidArgument shapes 17-02 produces
	// (finding 5): parseWindow's past-not_after rejection and
	// validateRuleSummary's rejection both wrap store.ErrInvalidArgument.
	pastNotAfter := fmt.Errorf("not_after 2020-01-01T00:00:00Z is not in the future: %w", store.ErrInvalidArgument)
	ruleSummary := fmt.Errorf("rule summary must be a single non-empty line: %w", store.ErrInvalidArgument)

	cases := []struct {
		name string
		err  error
		want connect.Code
	}{
		{"not_found", fmt.Errorf("%w: some-id", store.ErrNotFound), connect.CodeNotFound},
		{"invalid_argument_bare", store.ErrInvalidArgument, connect.CodeInvalidArgument},
		{"invalid_argument_past_not_after", pastNotAfter, connect.CodeInvalidArgument},
		{"invalid_argument_rule_summary", ruleSummary, connect.CodeInvalidArgument},
		{"rule_immutable", fmt.Errorf("%w — delete the rule instead", errRuleImmutable), connect.CodeFailedPrecondition},
		{"stale_summary", errStaleSummary, connect.CodeFailedPrecondition},
		{"ambiguous_short_id", fmt.Errorf("%w: abc123defg", store.ErrAmbiguousShortID), connect.CodeFailedPrecondition},
		{"idempotency_conflict", fmt.Errorf("idempotency key %q reused with different content: %w", "k", store.ErrIdempotencyConflict), connect.CodeAlreadyExists},
		{"already_superseded", fmt.Errorf("%w: some-id", store.ErrAlreadySuperseded), connect.CodeFailedPrecondition},
		// Wrapped ctx sentinels: the arms must match via errors.Is against a
		// wrapped error, not just the bare sentinel (round-8 MED).
		{"canceled_wrapped", fmt.Errorf("embed: %w", context.Canceled), connect.CodeCanceled},
		{"deadline_exceeded_wrapped", fmt.Errorf("store upsert: %w", context.DeadlineExceeded), connect.CodeDeadlineExceeded},
		{"response_too_large_wrapped", fmt.Errorf("list: %w", &store.ResponseTooLargeError{Method: "/qdrant.Points/Scroll", Limit: 4194304}), connect.CodeResourceExhausted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := connectError(ctx, tc.err)
			if got == nil {
				t.Fatalf("connectError(%v) = nil, want code %v", tc.err, tc.want)
			}
			if code := connect.CodeOf(got); code != tc.want {
				t.Fatalf("connectError(%v) code = %v, want %v", tc.err, code, tc.want)
			}
		})
	}

	t.Run("unexpected_error_maps_to_internal_without_leaking", func(t *testing.T) {
		leaky := errors.New("qdrant dial tcp 10.0.0.1:6334: connection refused")
		got := connectError(ctx, leaky)
		if connect.CodeOf(got) != connect.CodeInternal {
			t.Fatalf("code = %v, want CodeInternal", connect.CodeOf(got))
		}
		if strings.Contains(got.Error(), "10.0.0.1") || strings.Contains(got.Error(), "qdrant") {
			t.Fatalf("CodeInternal message leaked internal detail: %v", got)
		}
	})

	t.Run("no_code_aborted_arm", func(t *testing.T) {
		// round-5 LOW: no distinct concurrency/edit-conflict sentinel exists
		// today, so an arbitrary error must NOT map to CodeAborted.
		got := connectError(ctx, errors.New("some arbitrary failure"))
		if connect.CodeOf(got) == connect.CodeAborted {
			t.Fatalf("connectError must never emit CodeAborted (round-5 LOW): got %v", got)
		}
	})

	// response_too_large_scrubbed: the mapped *connect.Error carries ONLY the
	// shared envelope — never the wrapped error's RPC method, byte counts, or
	// upstream grpc-go text (D-04, D-06).
	t.Run("response_too_large_scrubbed", func(t *testing.T) {
		err := fmt.Errorf("list: %w", &store.ResponseTooLargeError{Method: "/qdrant.Points/Scroll", Limit: 4194304})
		got := connectError(ctx, err)
		var cerr *connect.Error
		if !errors.As(got, &cerr) {
			t.Fatalf("connectError(%v) = %v, does not unwrap to *connect.Error", err, got)
		}
		if want := responseTooLargeEnvelope(); cerr.Message() != want {
			t.Errorf("Message() = %q, want %q", cerr.Message(), want)
		}
		if strings.ContainsAny(cerr.Message(), "0123456789") {
			t.Errorf("Message() %q leaked an ASCII digit (a byte ceiling must never reach the wire)", cerr.Message())
		}
	})

	// response_too_large_empty_payload: connectError(ctx, nil) stays nil
	// (asserted by nil_is_nil above), and a *store.ResponseTooLargeError with
	// an empty method and zero counts maps to the IDENTICAL envelope — the
	// wire text never depends on the payload.
	t.Run("response_too_large_empty_payload", func(t *testing.T) {
		got := connectError(ctx, &store.ResponseTooLargeError{})
		var cerr *connect.Error
		if !errors.As(got, &cerr) {
			t.Fatalf("connectError(&store.ResponseTooLargeError{}) = %v, does not unwrap to *connect.Error", got)
		}
		if want := responseTooLargeEnvelope(); cerr.Message() != want {
			t.Errorf("Message() = %q, want %q (the wire text must never depend on the payload)", cerr.Message(), want)
		}
	})

	// server_resource_exhausted_not_relabeled: a server-sent ResourceExhausted
	// that the store-layer classifier did NOT relabel (never wrapped as
	// store.ErrResponseTooLarge) is not an *argError either, so it falls
	// through to the generic default arm — CodeInternal, never
	// CodeResourceExhausted.
	t.Run("server_resource_exhausted_not_relabeled", func(t *testing.T) {
		err := status.Error(grpccodes.ResourceExhausted, "Too many requests")
		got := connectError(ctx, err)
		var cerr *connect.Error
		if !errors.As(got, &cerr) {
			t.Fatalf("connectError(%v) = %v, does not unwrap to *connect.Error", err, got)
		}
		if cerr.Code() != connect.CodeInternal {
			t.Errorf("code = %v, want CodeInternal", cerr.Code())
		}
		if cerr.Message() != "internal error" {
			t.Errorf("Message() = %q, want %q", cerr.Message(), "internal error")
		}
	})

	// response_too_large_distinct_from_arg_classes: durable record 667p88n2be
	// — assert by explicit inequality, not merely "not CodeInternal", so a
	// switch-ordering regression that collapsed this arm into another class
	// cannot pass silently.
	t.Run("response_too_large_distinct_from_arg_classes", func(t *testing.T) {
		got := connect.CodeOf(connectError(ctx, &store.ResponseTooLargeError{}))
		for _, other := range []connect.Code{
			connect.CodeInvalidArgument, connect.CodeOutOfRange,
			connect.CodeFailedPrecondition, connect.CodeInternal,
		} {
			if got == other {
				t.Errorf("response-too-large code (%v) must be DISTINCT from %v, got the same value", got, other)
			}
		}
		if got != connect.CodeResourceExhausted {
			t.Errorf("response-too-large code = %v, want CodeResourceExhausted", got)
		}
	})
}

// TestConnectErrorStaleSummaryDistinctFromMalformed is 02-03-PLAN.md Task 2's
// required guard: the summary-stale code and a plain malformed-argument code
// must be DISTINCT values, not merely both individually correct and both
// non-internal — the exact assertion weakness that would let connectError's
// load-bearing switch ordering (the *argError errors.As arm MUST stay first,
// ahead of the store.ErrInvalidArgument sentinel arm — see connectError's own
// doc comment and durable record 667p88n2be) silently regress: a test that
// only checks "not CodeInternal" still passes even if a reordering collapsed
// every class back to CodeInvalidArgument.
func TestConnectErrorStaleSummaryDistinctFromMalformed(t *testing.T) {
	ctx := context.Background()
	staleCode := connect.CodeOf(connectError(ctx, errStaleSummary))
	malformedCode := connect.CodeOf(connectError(ctx, argErrf(classMalformed, HintRequired, "content", "content is required")))
	if staleCode == malformedCode {
		t.Fatalf("summary-stale code (%v) and malformed-argument code (%v) must be DISTINCT, got the same value", staleCode, malformedCode)
	}
	if staleCode != connect.CodeFailedPrecondition {
		t.Errorf("summary-stale code = %v, want CodeFailedPrecondition", staleCode)
	}
	if malformedCode != connect.CodeInvalidArgument {
		t.Errorf("malformed-argument code = %v, want CodeInvalidArgument", malformedCode)
	}
}
