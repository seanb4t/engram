// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"testing"

	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/seanb4t/engram/internal/store"
)

// wantExitCode asserts err carries ExitCode() == want via errors.As.
func wantExitCode(t *testing.T, err error, want int) {
	t.Helper()
	var ec interface{ ExitCode() int }
	if !errors.As(err, &ec) {
		t.Fatalf("classified error has no ExitCode(): %v", err)
	}
	if got := ec.ExitCode(); got != want {
		t.Errorf("ExitCode() = %d, want %d (err=%v)", got, want, err)
	}
}

// TestClassifyOperatorErr exercises every behavior row from the plan's
// <behavior> table.
func TestClassifyOperatorErr(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := classifyOperatorErr(nil); got != nil {
			t.Errorf("classifyOperatorErr(nil) = %v, want nil", got)
		}
	})

	t.Run("store.ErrNotFound", func(t *testing.T) {
		err := fmt.Errorf("get %s: %w", "abc123", store.ErrNotFound)
		wantExitCode(t, classifyOperatorErr(err), exitNotFound)
	})

	t.Run("store.ErrInvalidArgument", func(t *testing.T) {
		err := fmt.Errorf("bad arg: %w", store.ErrInvalidArgument)
		wantExitCode(t, classifyOperatorErr(err), exitUsage)
	})

	t.Run("store.ErrAmbiguousShortID", func(t *testing.T) {
		err := fmt.Errorf("ambiguous: %w", store.ErrAmbiguousShortID)
		wantExitCode(t, classifyOperatorErr(err), exitUsage)
	})

	t.Run("store.ErrShortIDExhausted", func(t *testing.T) {
		// Chosen code: exitUnavailable. See operror.go's doc comment on this
		// arm and 01-05-SUMMARY.md for the recorded rationale.
		err := fmt.Errorf("mint exhausted: %w", store.ErrShortIDExhausted)
		wantExitCode(t, classifyOperatorErr(err), exitUnavailable)
	})

	t.Run("store.ErrIdempotencyConflict", func(t *testing.T) {
		// Chosen code: exitUsage (no live trigger from these four commands
		// today; kept exhaustive per connectError's own precedent). See
		// operror.go's doc comment and 01-05-SUMMARY.md.
		err := fmt.Errorf("conflict: %w", store.ErrIdempotencyConflict)
		wantExitCode(t, classifyOperatorErr(err), exitUsage)
	})

	t.Run("store.ErrAlreadySuperseded", func(t *testing.T) {
		// Chosen code: exitUsage (no live trigger from these four commands
		// today; kept exhaustive per connectError's own precedent). See
		// operror.go's doc comment and 01-05-SUMMARY.md.
		err := fmt.Errorf("superseded: %w", store.ErrAlreadySuperseded)
		wantExitCode(t, classifyOperatorErr(err), exitUsage)
	})

	t.Run("config-load/validate error", func(t *testing.T) {
		// A representative, unsentineled config/parse error, exactly the
		// shape server.StoreAndSummarizerFromEnv / storeFromConfig raise --
		// no gRPC status, no store sentinel. classifyOperatorErr's own
		// default leaves this untouched (D-02, proven by the "unrecognized
		// error" row below); classifyOperatorErrConstruction is the
		// construction-call-site fallback that classifies it exitUsage --
		// see its doc comment in operror.go.
		err := errors.New("ENGRAM_SUMMARY_MODEL is empty: auto-summary is disabled")
		wantExitCode(t, classifyOperatorErrConstruction(err), exitUsage)
	})

	t.Run("grpc codes.Unavailable", func(t *testing.T) {
		err := fmt.Errorf("EnsureCollection: %w", status.Error(grpccodes.Unavailable, "no connection"))
		wantExitCode(t, classifyOperatorErr(err), exitUnavailable)
	})

	t.Run("raw dial failure", func(t *testing.T) {
		// A non-status net.Error reaching the classifier directly (never
		// normalized into a *status.Status), wrapped exactly the way
		// store.Reindex wraps a check-source failure.
		err := fmt.Errorf("reindex: check source %q: %w", "t", &net.OpError{
			Op: "dial", Net: "tcp", Err: errors.New("connection refused"),
		})
		wantExitCode(t, classifyOperatorErr(err), exitUnavailable)
	})

	t.Run("context.DeadlineExceeded", func(t *testing.T) {
		wantExitCode(t, classifyOperatorErr(context.DeadlineExceeded), exitUnavailable)
	})

	t.Run("context.Canceled", func(t *testing.T) {
		wantExitCode(t, classifyOperatorErr(context.Canceled), exitUnavailable)
	})

	t.Run("already classified is idempotent", func(t *testing.T) {
		original := &cliError{code: exitUsage, err: errors.New("boom")}
		got := classifyOperatorErr(original)
		if got != error(original) {
			t.Errorf("classifyOperatorErr(already-classified) = %v, want the same value unchanged", got)
		}
		wantExitCode(t, got, exitUsage)
	})

	t.Run("unrecognized error stays unclassified", func(t *testing.T) {
		err := errors.New("totally unrelated failure")
		got := classifyOperatorErr(err)
		if got != err {
			t.Errorf("classifyOperatorErr(unrecognized) = %v, want the same value unchanged", got)
		}
		var ec interface{ ExitCode() int }
		if errors.As(got, &ec) {
			t.Errorf("unrecognized error unexpectedly carries ExitCode() = %d", ec.ExitCode())
		}
	})
}

// TestClassifyOperatorErrCodesAreDistinct pins the per-sentinel exit codes
// as a SET, mirroring TestCatalogExitCodesMatchMapper's set-equality
// discipline (catalog_test.go). A test asserting only "not exit code 1"
// would still pass if the switch in operror.go collapsed every sentinel
// into a single code by an ordering mistake; this proves the classes stay
// separated.
func TestClassifyOperatorErrCodesAreDistinct(t *testing.T) {
	sentinels := map[string]error{
		"ErrNotFound":            store.ErrNotFound,
		"ErrInvalidArgument":     store.ErrInvalidArgument,
		"ErrAmbiguousShortID":    store.ErrAmbiguousShortID,
		"ErrShortIDExhausted":    store.ErrShortIDExhausted,
		"ErrIdempotencyConflict": store.ErrIdempotencyConflict,
		"ErrAlreadySuperseded":   store.ErrAlreadySuperseded,
	}

	got := make(map[int]bool)
	for name, sentinel := range sentinels {
		classified := classifyOperatorErr(fmt.Errorf("%s: %w", name, sentinel))
		var ec interface{ ExitCode() int }
		if !errors.As(classified, &ec) {
			t.Fatalf("sentinel %s: classifyOperatorErr returned an unclassified error: %v", name, classified)
		}
		got[ec.ExitCode()] = true
	}

	want := map[int]bool{exitNotFound: true, exitUsage: true, exitUnavailable: true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sentinel exit codes = {%s}, want {%s}", sortedIntKeys(got), sortedIntKeys(want))
	}
}
