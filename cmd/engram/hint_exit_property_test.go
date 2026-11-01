// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"pgregory.net/rapid"
)

// serverHintCodes reads every HintCode constant from the server's source,
// so a new hint code joins this property without an edit here.
func serverHintCodes(t *testing.T) []string {
	t.Helper()
	af, err := parser.ParseFile(token.NewFileSet(), "../../internal/server/argerror.go", nil, 0)
	if err != nil {
		t.Fatalf("parse argerror.go: %v", err)
	}
	var codes []string
	ast.Inspect(af, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		if id, ok := vs.Type.(*ast.Ident); !ok || id.Name != "HintCode" {
			return true
		}
		for _, v := range vs.Values {
			if lit, ok := v.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				s, _ := strconv.Unquote(lit.Value)
				codes = append(codes, s)
			}
		}
		return true
	})
	if len(codes) == 0 {
		t.Fatal("found no HintCode constants in argerror.go")
	}
	return codes
}

func TestPropertyHintExitCode(t *testing.T) {
	hints := serverHintCodes(t)
	// The server sends an argument rejection under one of three codes,
	// chosen by the failure class, and an oversized response under
	// resource_exhausted.
	argCodes := []connect.Code{connect.CodeInvalidArgument, connect.CodeOutOfRange, connect.CodeFailedPrecondition}
	exitFor := map[string]int{}
	rapid.Check(t, func(t *rapid.T) {
		hint := rapid.SampledFrom(hints).Draw(t, "hint")
		field := rapid.StringMatching(`[a-z][a-z_]{0,20}`).Draw(t, "field")
		code := rapid.SampledFrom(argCodes).Draw(t, "code")
		if hint == "response_too_large" {
			field, code = "response", connect.CodeResourceExhausted
		}
		envelope := fmt.Sprintf("field=%s hint=%s", field, hint)
		err := wrapRPCError(connect.NewError(code, errors.New(envelope+": detail")))

		got := exitCodeFromError(err)
		if prev, seen := exitFor[hint]; seen && prev != got {
			t.Fatalf("hint %s exits %d under %s but %d before", hint, got, code, prev)
		}
		exitFor[hint] = got
		if want := map[bool]int{true: exitTooLarge, false: exitUsage}[hint == "response_too_large"]; got != want {
			t.Fatalf("hint %s under %s exits %d, want %d", hint, code, got, want)
		}
		if stderr := "Error: " + err.Error(); !strings.Contains(stderr, envelope) {
			t.Fatalf("stderr %q does not name %q", stderr, envelope)
		}
	})
}
