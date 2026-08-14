// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package migrate

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// noopApply is a placeholder ApplyFunc for fixtures in this file where the
// point under test is Validate/StepsFrom's ORDERING logic, never the
// transformation itself.
func noopApply(payload map[string]any) (map[string]any, error) {
	return payload, nil
}

// TestValidateRejectsOrderingAndUniquenessViolations exercises Validate's
// three rules — transition uniqueness, advance, contiguity — each with a
// fixture in which that rule's OWN message is the observed one. Renamed
// from ...AndIdempotencyViolations after review cycle 1: the first rule is
// TRANSITION UNIQUENESS, not idempotence — Validate never calls a step's
// ApplyFunc, so it says nothing about whether applying a step twice is
// safe. Calling it idempotency here would repeat the exact overclaim the
// production doc comment was corrected for.
func TestValidateRejectsOrderingAndUniquenessViolations(t *testing.T) {
	cases := []struct {
		name           string
		steps          []Step
		wantErr        bool
		wantSubstrings []string
		wantAbsent     []string
	}{
		{
			name:    "empty registry",
			steps:   nil,
			wantErr: false,
		},
		{
			name:    "single valid step",
			steps:   []Step{NewStep(0, 1, nil, Irreversible("x"), noopApply)},
			wantErr: false,
		},
		{
			name: "valid chain",
			steps: []Step{
				NewStep(0, 1, nil, Irreversible("x"), noopApply),
				NewStep(1, 2, nil, Irreversible("x"), noopApply),
				NewStep(2, 3, nil, Irreversible("x"), noopApply),
			},
			wantErr: false,
		},
		{
			name: "duplicate from",
			steps: []Step{
				NewStep(0, 1, nil, Irreversible("x"), noopApply),
				NewStep(0, 2, nil, Irreversible("x"), noopApply),
			},
			wantErr:        true,
			wantSubstrings: []string{"transition uniqueness violated", "From=0"},
		},
		{
			name: "duplicate to",
			steps: []Step{
				NewStep(0, 2, nil, Irreversible("x"), noopApply),
				NewStep(1, 2, nil, Irreversible("x"), noopApply),
			},
			wantErr:        true,
			wantSubstrings: []string{"transition uniqueness violated", "To=2"},
		},
		{
			name:           "non-advancing step",
			steps:          []Step{NewStep(1, 1, nil, Irreversible("x"), noopApply)},
			wantErr:        true,
			wantSubstrings: []string{"does not advance the version"},
		},
		{
			name:           "backwards step",
			steps:          []Step{NewStep(2, 1, nil, Irreversible("x"), noopApply)},
			wantErr:        true,
			wantSubstrings: []string{"does not advance the version"},
		},
		{
			name: "broken contiguity",
			steps: []Step{
				NewStep(0, 1, nil, Irreversible("x"), noopApply),
				NewStep(2, 3, nil, Irreversible("x"), noopApply),
			},
			wantErr:        true,
			wantSubstrings: []string{"is not contiguous with step"},
			wantAbsent:     []string{"transition uniqueness violated", "does not advance the version"},
		},
		{
			// Proves errors.Join ACCUMULATES rather than short-circuits: a
			// first-violation-wins implementation would fail this row while
			// passing every other one above.
			name: "multiple simultaneous violations",
			steps: []Step{
				NewStep(1, 1, nil, Irreversible("x"), noopApply),
				NewStep(3, 2, nil, Irreversible("x"), noopApply),
			},
			wantErr:        true,
			wantSubstrings: []string{"does not advance the version", "is not contiguous with step"},
		},
	}
	if len(cases) == 0 {
		t.Fatal("zero test cases defined in TestValidateRejectsOrderingAndUniquenessViolations — this table must never be empty")
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.steps)
			if tc.wantErr && err == nil {
				t.Fatalf("Validate(%s) = nil error, want an error containing: %v", tc.name, tc.wantSubstrings)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate(%s) = %v, want no error", tc.name, err)
			}
			if err == nil {
				return
			}
			msg := err.Error()
			for _, sub := range tc.wantSubstrings {
				if !strings.Contains(msg, sub) {
					t.Errorf("Validate(%s) error = %q, want it to contain %q — this row exists to prove that specific rule's own message, not merely that SOME error was returned", tc.name, msg, sub)
				}
			}
			for _, sub := range tc.wantAbsent {
				if strings.Contains(msg, sub) {
					t.Errorf("Validate(%s) error = %q, want it to NOT contain %q — this rule must not have fired on this fixture, proving accumulation reports precisely the rules that fired rather than blanket-flagging every rule", tc.name, msg, sub)
				}
			}
		})
	}

	// The shipped state this phase must leave behind: the real, empty
	// production registry passes Validate with no error.
	if err := Validate(Registry); err != nil {
		t.Fatalf("Validate(Registry) = %v, want no error — migrate.Registry ships EMPTY this phase (Phase 4 registers the first step)", err)
	}
}

// TestStepsFromSelectsContiguousChain pins StepsFrom's chain-selection
// behavior over a fixed valid registry (0->1, 1->2, 2->3) in both
// directions: successful selection (as an ORDERED sequence — unlike every
// set comparison elsewhere in this phase, order matters here) and the three
// distinct ways a request can fail to select a chain.
func TestStepsFromSelectsContiguousChain(t *testing.T) {
	reg := []Step{
		NewStep(0, 1, nil, Irreversible("x"), noopApply),
		NewStep(1, 2, nil, Irreversible("x"), noopApply),
		NewStep(2, 3, nil, Irreversible("x"), noopApply),
	}

	type pair struct{ from, to Version }

	cases := []struct {
		name       string
		from, to   Version
		wantErr    bool
		wantErrSub string
		wantPairs  []pair
	}{
		{name: "from equals to", from: 2, to: 2, wantPairs: nil},
		{
			name:      "full chain",
			from:      0,
			to:        3,
			wantPairs: []pair{{0, 1}, {1, 2}, {2, 3}},
		},
		{
			name:      "sub-chain",
			from:      1,
			to:        3,
			wantPairs: []pair{{1, 2}, {2, 3}},
		},
		{
			name:       "unreachable target",
			from:       0,
			to:         5,
			wantErr:    true,
			wantErrSub: "broke at 3",
		},
		{
			name:       "unknown start",
			from:       7,
			to:         8,
			wantErr:    true,
			wantErrSub: "broke at 7",
		},
		{
			// A revert is Phase 4's `migrate revert` verb running declared
			// inverses, never a backwards StepsFrom.
			name:    "backwards request",
			from:    3,
			to:      1,
			wantErr: true,
		},
	}
	if len(cases) == 0 {
		t.Fatal("zero test cases defined in TestStepsFromSelectsContiguousChain — this table must never be empty")
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := StepsFrom(reg, tc.from, tc.to)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("StepsFrom(steps, %d, %d) = nil error, want an error", tc.from, tc.to)
				}
				if tc.wantErrSub != "" && !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("StepsFrom(steps, %d, %d) error = %q, want it to contain %q", tc.from, tc.to, err.Error(), tc.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("StepsFrom(steps, %d, %d) = %v, want no error", tc.from, tc.to, err)
			}
			if len(got) != len(tc.wantPairs) {
				t.Fatalf("StepsFrom(steps, %d, %d) returned %d steps, want %d", tc.from, tc.to, len(got), len(tc.wantPairs))
			}
			for i, p := range tc.wantPairs {
				if got[i].From() != p.from || got[i].To() != p.to {
					t.Errorf("StepsFrom(steps, %d, %d)[%d] = (From=%d To=%d), want (From=%d To=%d) — order matters here", tc.from, tc.to, i, got[i].From(), got[i].To(), p.from, p.to)
				}
			}
		})
	}
}

// TestRegistryIsAPackageLevelVarWithPhase4Marker is the AST gate that makes
// PA-1's Phase 4 obligation enforceable today rather than a doc comment
// Phase 4 could lose. It proves only the PLACEMENT and CONSTRUCTION-SITE
// halves of PA-1: Phase 4 can neither move Registry into a function nor
// hollow the declaration out into a deferred assignment. It does NOT prove
// the init-time panic itself — the registry is empty this phase, and there
// is no Irreversible("") in it to panic; that half remains Phase 4's, once
// it registers a real step.
func TestRegistryIsAPackageLevelVarWithPhase4Marker(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "registry.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse registry.go: %v", err)
	}

	// Non-vacuity guard: a parse that silently returned an empty Decls
	// (wrong path, a read error swallowed somewhere upstream) would satisfy
	// neither check below by making the loop body never run.
	if len(f.Decls) == 0 {
		t.Fatal("parsed registry.go has zero file-scope declarations — a parse that found nothing would satisfy this gate's assertions vacuously; this is a defect in the parse, not evidence Registry is correctly declared")
	}

	// Walk f.Decls directly, NOT ast.Inspect: ast.Inspect would also match
	// a `var Registry` declared inside a function body, defeating the
	// entire point of this gate (a builder function's local var would look
	// identical to the package-level one under a naive full-tree walk).
	var registryDecl *ast.GenDecl
	var registrySpec *ast.ValueSpec
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, s := range gd.Specs {
			vs, ok := s.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range vs.Names {
				if name.Name == "Registry" {
					registryDecl = gd
					registrySpec = vs
				}
			}
		}
	}

	if registryDecl == nil || registrySpec == nil {
		t.Fatal("no file-scope `var Registry` declaration found in registry.go — Phase 4 must keep Registry a package-level var literal, because that placement is what makes a bad Irreversible(\"\") fail at package INIT rather than whenever some builder function happens to run; this is D-03's mechanism, not a style preference")
	}

	// The half review cycle 2 found missing: `var Registry []Step` plus a
	// later `func RegisterSteps() { Registry = []Step{...} }` assignment
	// preserves the placement check above while moving construction OUT of
	// package initialization entirely — voiding D-03. Registry must be
	// initialized by a composite literal AT its declaration. An EMPTY
	// literal ([]Step{}) legitimately passes this phase: what is asserted
	// is that whatever Registry contains is built at the declaration, not
	// that it is non-empty.
	if len(registrySpec.Values) == 0 {
		t.Fatal("`var Registry` has zero Values at its declaration — this is the DEFERRED-INIT shape (`var Registry []Step` with a later `RegisterSteps() { Registry = ... }` assignment) that satisfies the placement check alone while moving construction out of package initialization entirely; Registry must be initialized by a composite literal AT the declaration, even when that literal is empty ([]Step{})")
	}
	if len(registrySpec.Values) != 1 {
		t.Fatalf("`var Registry` declaration has %d values, want exactly 1", len(registrySpec.Values))
	}
	if _, ok := registrySpec.Values[0].(*ast.CompositeLit); !ok {
		t.Fatalf("`var Registry`'s single value is a %T, want *ast.CompositeLit (a []Step{...} literal) — Registry must be constructed at its declaration, not assigned from a function call or any other expression", registrySpec.Values[0])
	}

	doc := registryDecl.Doc
	if doc == nil {
		t.Fatal("`var Registry` declaration has no doc comment — it must carry a `// PHASE4:` marker naming the package-level placement obligation and D-03")
	}
	docText := doc.Text()
	lower := strings.ToLower(docText)
	if !strings.Contains(docText, "PHASE4:") {
		t.Fatalf("`var Registry`'s doc comment does not contain a `// PHASE4:` marker — the obligation marker was removed or reworded past recognition:\n%s", docText)
	}
	if !strings.Contains(lower, "package scope") && !strings.Contains(lower, "package-level") && !strings.Contains(lower, "package level") {
		t.Fatalf("`var Registry`'s `// PHASE4:` marker does not mention the package-level placement obligation:\n%s", docText)
	}
	if !strings.Contains(lower, "d-03") {
		t.Fatalf("`var Registry`'s `// PHASE4:` marker does not mention D-03:\n%s", docText)
	}
}
