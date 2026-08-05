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
