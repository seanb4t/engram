// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// crossFieldHints is D-02's four structural markers. A call to the generic
// argErrf/argErrFieldsf constructors carrying one of these hint codes is the
// exact residual gap D-04's compiler-enforced conditionalErrf cannot close
// on its own: nothing stops a NEW rejection from being hand-typed here
// instead of referencing a declared surfaces.ConditionalRule. This sweep is
// the narrow backstop that catches it.
var crossFieldHints = map[string]bool{
	"HintConditionalRequired": true,
	"HintMutuallyExclusive":   true,
	"HintNotApplicable":       true,
	"HintOrdering":            true,
}

// conformanceExcludedSites is D-02's "AND NOT this specific site" clause
// made explicit: the ONE deliberate, documented exception to the sweep
// below. Keyed by "<file>:<enclosing function>" — a stable site identifier,
// not a line number, so an unrelated edit above the call does not silently
// move the exclusion (and does not silently re-admit a site whose function
// was renamed or removed).
//
// Do not add a second entry quietly: TestConformanceExcludedSitesStaysAtOne
// fails the moment this map holds more than one entry, and any addition
// needs its rationale argued in this plan's lineage (02-03-PLAN.md), not
// slipped in as a side effect of an unrelated change.
var conformanceExcludedSites = map[string]string{
	// tools.go's parseWindow: "not_after must be in the future" compares
	// the caller's not_after against time.Now(), not a second caller-
	// supplied field. It is single-field and clock-dependent — a wall-clock
	// state constraint, not an interface-legible relational rule — so it is
	// carved out by name rather than swept in by a bare hint-code-in-set
	// predicate, which would wrongly demand this wall-clock text appear in
	// cobra Usage and jsonschema tags, where no such text exists or should.
	"tools.go:parseWindow": "not_after-must-be-in-the-future is single-field/clock-dependent (D-02's one explicit registry exclusion)",
}

// TestNoUnregisteredConditionalRejection is D-04's residual backstop: parses
// every non-test .go file in this package with go/parser (the same
// mechanism cmd/engram's TestClientFilesImportBoundary uses for its own
// source-scanning gate) and fails for any argErrf/argErrFieldsf call whose
// hint argument is one of the four cross-field codes, UNLESS its enclosing
// site is named in conformanceExcludedSites. conditionalErrf itself is
// exempt by construction — it forwards to argErrFieldsf internally, but
// this sweep only inspects direct call sites in source, so
// conditionalErrf's own body (declared once, in conditionalerr.go) is
// never itself flagged as a violation of the invariant it enforces.
func TestNoUnregisteredConditionalRejection(t *testing.T) {
	violations := scanForUnregisteredConditionalRejections(t)
	for _, v := range violations {
		t.Error(v)
	}
}

// scanForUnregisteredConditionalRejections is the sweep's production code
// path — the same function TestNoUnregisteredConditionalRejection calls for
// the real gate and the fail-first/carve-out-growth proofs below call
// against synthetic conditions, so no test exercises a parallel copy of the
// scan itself.
func scanForUnregisteredConditionalRejections(t *testing.T) []string {
	t.Helper()

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no .go files found in internal/server — sweep cannot exercise anything")
	}

	var violations []string
	fset := token.NewFileSet()
	scanned := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		scanned++
		af, err := parser.ParseFile(fset, f, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", f, err)
		}
		violations = append(violations, scanFileForUnregisteredConditionalRejections(f, af)...)
	}
	if scanned == 0 {
		t.Fatal("no non-test .go files scanned — sweep cannot exercise anything")
	}
	return violations
}

// scanFileForUnregisteredConditionalRejections walks af's declarations,
// tracking the enclosing top-level function for every call expression it
// finds, and reports one violation per disallowed argErrf/argErrFieldsf
// call. Both constructors take the hint code as their second positional
// argument (argErrf(class, hint, field, format, ...a) /
// argErrFieldsf(class, hint, fields, detail)), so index 1 is checked
// uniformly regardless of which constructor is called.
func scanFileForUnregisteredConditionalRejections(filename string, af *ast.File) []string {
	var violations []string
	for _, decl := range af.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		funcName := fn.Name.Name
		ast.Inspect(fn, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			ident, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			if ident.Name != "argErrf" && ident.Name != "argErrFieldsf" {
				return true
			}
			if len(call.Args) < 2 {
				return true // malformed call — not this sweep's concern, go vet/build catches it
			}
			hintIdent, ok := call.Args[1].(*ast.Ident)
			if !ok || !crossFieldHints[hintIdent.Name] {
				return true
			}
			site := filename + ":" + funcName
			if _, excluded := conformanceExcludedSites[site]; excluded {
				return true
			}
			violations = append(violations, fmt.Sprintf(
				"site=%s call=%s(%s, ...): a cross-field hint code was constructed via the generic constructor, not conditionalErrf against a declared surfaces.ConditionalRule — either convert this site or add it BY NAME to conformanceExcludedSites with a written reason",
				site, ident.Name, hintIdent.Name))
			return true
		})
	}
	return violations
}

// TestConformanceExcludedSitesStaysAtOne is D-02's "the carve-out cannot
// silently grow" guard: conformanceExcludedSites holds exactly the one
// documented exception today, and this test fails the moment a second entry
// appears, directing a future author to justify any addition in this plan's
// lineage (02-03-PLAN.md) rather than adding it quietly.
func TestConformanceExcludedSitesStaysAtOne(t *testing.T) {
	if len(conformanceExcludedSites) != 1 {
		t.Fatalf("conformanceExcludedSites has %d entries, want exactly 1 — a new exclusion must be justified in 02-03-PLAN.md's lineage, not added quietly: %v",
			len(conformanceExcludedSites), conformanceExcludedSites)
	}
	if _, ok := conformanceExcludedSites["tools.go:parseWindow"]; !ok {
		t.Fatalf("conformanceExcludedSites' one entry is not the documented not_after-must-be-in-the-future carve-out: %v", conformanceExcludedSites)
	}
}
