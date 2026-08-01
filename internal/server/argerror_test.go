// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/seanb4t/engram/internal/store"
)

// TestArgErrorGrammar pins the D-17 checkpoint grammar: field=<fields>
// hint=<code>: <detail>, comma-joined for multiple fields with no space
// after the comma. Per go-sdk@v1.6.1/mcp/server.go:340-354 this string IS
// the entire MCP wire payload for a rejected tool call, so the grammar is
// pinned exactly, not loosely matched.
func TestArgErrorGrammar(t *testing.T) {
	t.Run("single_field", func(t *testing.T) {
		err := argErrf(classMalformed, HintEnum, "kind", "kind must be \"map\" or \"fact\"")
		want := `field=kind hint=enum: kind must be "map" or "fact"`
		if got := err.Error(); got != want {
			t.Fatalf("Error() = %q, want %q", got, want)
		}
		// Exercise the three errors.As accessors (argFieldsOf, argHintOf,
		// argClassOf) here: they exist for the assertions and the future
		// sweep sites (04-04, 04-05) to use, per the artifact list.
		if got := argFieldsOf(err); len(got) != 1 || got[0] != "kind" {
			t.Errorf("argFieldsOf(err) = %v, want [kind]", got)
		}
		if got := argHintOf(err); got != HintEnum {
			t.Errorf("argHintOf(err) = %v, want %v", got, HintEnum)
		}
		if class, ok := argClassOf(err); !ok || class != classMalformed {
			t.Errorf("argClassOf(err) = (%v, %v), want (%v, true)", class, ok, classMalformed)
		}
	})

	t.Run("two_fields", func(t *testing.T) {
		err := argErrFieldsf(classPrecondition, HintOrdering, []string{"not_before", "not_after"}, "not_before must precede not_after")
		want := "field=not_before,not_after hint=ordering: not_before must precede not_after"
		if got := err.Error(); got != want {
			t.Fatalf("Error() = %q, want %q", got, want)
		}
		if got := argFieldsOf(err); len(got) != 2 || got[0] != "not_before" || got[1] != "not_after" {
			t.Errorf("argFieldsOf(err) = %v, want [not_before not_after]", got)
		}
	})

	t.Run("non_argError_accessors_return_zero_values", func(t *testing.T) {
		plain := errors.New("plain error")
		if got := argFieldsOf(plain); got != nil {
			t.Errorf("argFieldsOf(plain) = %v, want nil", got)
		}
		if got := argHintOf(plain); got != "" {
			t.Errorf("argHintOf(plain) = %v, want empty", got)
		}
		if _, ok := argClassOf(plain); ok {
			t.Errorf("argClassOf(plain) ok = true, want false")
		}
	})
}

// TestArgErrorUnwrapsInvalidArgument pins load-bearing back-compat: every
// existing errors.Is(err, store.ErrInvalidArgument) consumer keeps working
// unchanged across the whole D-06 sweep, for all three classes.
func TestArgErrorUnwrapsInvalidArgument(t *testing.T) {
	classes := []struct {
		name  string
		class argClass
	}{
		{"malformed", classMalformed},
		{"out_of_range", classOutOfRange},
		{"precondition", classPrecondition},
	}
	for _, tc := range classes {
		t.Run(tc.name, func(t *testing.T) {
			err := argErrf(tc.class, HintRequired, "field", "detail")
			if !errors.Is(err, store.ErrInvalidArgument) {
				t.Fatalf("errors.Is(err, store.ErrInvalidArgument) = false for class %v, want true", tc.class)
			}
		})
	}
}

// TestArgErrorConnectCodeTrio asserts ConnectCode() returns exactly the
// expected code per class AND that every class's code is a member of the
// CLI-compatible trio (D-20) as a SET check, so a future fourth class that
// maps outside the trio fails this test rather than silently breaking the
// CLI's exit-code contract.
func TestArgErrorConnectCodeTrio(t *testing.T) {
	trio := map[connect.Code]bool{
		connect.CodeInvalidArgument:    true,
		connect.CodeOutOfRange:         true,
		connect.CodeFailedPrecondition: true,
	}

	cases := []struct {
		name  string
		class argClass
		want  connect.Code
	}{
		{"malformed", classMalformed, connect.CodeInvalidArgument},
		{"out_of_range", classOutOfRange, connect.CodeOutOfRange},
		{"precondition", classPrecondition, connect.CodeFailedPrecondition},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := argErrf(tc.class, HintRequired, "field", "detail")
			var ae *argError
			if !errors.As(err, &ae) {
				t.Fatalf("argErrf did not return an *argError-wrapping error: %T", err)
			}
			got := ae.ConnectCode()
			if got != tc.want {
				t.Fatalf("ConnectCode() = %v, want %v", got, tc.want)
			}
			if !trio[got] {
				t.Fatalf("ConnectCode() = %v is NOT a member of the CLI-compatible trio {InvalidArgument, OutOfRange, FailedPrecondition}", got)
			}
		})
	}
}

// TestMCPErrorCarriesHintCode asserts on err.Error()'s STRING, deliberately —
// per go-sdk@v1.6.1/mcp/server.go:340-354 the string is the entire MCP
// payload for a rejected tool call, so a test that inspected only the Go
// struct would pass while the wire carried nothing.
func TestMCPErrorCarriesHintCode(t *testing.T) {
	const sc = "discovery:repo:X"
	cite := []citationArg{{Kind: "file", Ref: "f"}}

	cases := []struct {
		name      string
		a         storeDiscoveryArgs
		wantField string
		wantHint  string
	}{
		{"empty content", storeDiscoveryArgs{Content: "", Kind: "fact", Scope: sc, Citations: cite}, "content", "required"},
		{"content too large", storeDiscoveryArgs{Content: strings.Repeat("a", maxDiscoveryContentBytes+1), Kind: "fact", Scope: sc, Citations: cite}, "content", "too_long"},
		{"bad kind", storeDiscoveryArgs{Content: "x", Kind: "blob", Scope: sc, Citations: cite}, "kind", "enum"},
		{"empty scope", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: "", Citations: cite}, "scope", "required"},
		{"non-discovery scope", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: "repo:X", Citations: cite}, "scope", "prefix"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStoreDiscovery(tc.a)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			got := err.Error()
			if !strings.Contains(got, "field="+tc.wantField) {
				t.Errorf("Error() = %q, want it to contain %q", got, "field="+tc.wantField)
			}
			if !strings.Contains(got, "hint="+tc.wantHint) {
				t.Errorf("Error() = %q, want it to contain %q", got, "hint="+tc.wantHint)
			}
		})
	}
}

// TestStoreDiscoveryValidationIsNotCodeInternal pins the closed D-11a defect:
// before Task 1's conversion, validateStoreDiscovery's five rejections were
// bare fmt.Errorf values that fell through connectError's switch to
// CodeInternal — a caller's invalid input reported as a server fault. Calls
// connectError directly with context.Background(); the point is the mapper's
// classification, not standing up an RPC (the existing connectError tests
// already establish this direct-call idiom).
func TestStoreDiscoveryValidationIsNotCodeInternal(t *testing.T) {
	const sc = "discovery:repo:X"
	cite := []citationArg{{Kind: "file", Ref: "f"}}

	cases := []struct {
		name string
		a    storeDiscoveryArgs
	}{
		{"empty content", storeDiscoveryArgs{Content: "", Kind: "fact", Scope: sc, Citations: cite}},
		{"content too large", storeDiscoveryArgs{Content: strings.Repeat("a", maxDiscoveryContentBytes+1), Kind: "fact", Scope: sc, Citations: cite}},
		{"bad kind", storeDiscoveryArgs{Content: "x", Kind: "blob", Scope: sc, Citations: cite}},
		{"empty scope", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: "", Citations: cite}},
		{"non-discovery scope", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: "repo:X", Citations: cite}},
	}
	ctx := context.Background()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStoreDiscovery(tc.a)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			got := connect.CodeOf(connectError(ctx, err))
			if got == connect.CodeInternal {
				t.Fatalf("connectError classified a validation rejection as CodeInternal (D-11a): %v", err)
			}
		})
	}
}
