// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/surfaces"
)

// cobraExposedFields returns the flag-name set collectFlags derives for
// cmd — the SAME live-tree walk buildCatalog already uses (catalog.go),
// never a second traversal.
func cobraExposedFields(cmd *cobra.Command) []string {
	flags := collectFlags(cmd)
	out := make([]string, len(flags))
	for i, f := range flags {
		out[i] = f.Name
	}
	return out
}

func containsSurface(list []surfaces.Surface, want surfaces.Surface) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// TestSurfaceConformanceCobraUsage is D-05's cobra-Usage gate: for every
// non-hidden command whose OWN flag set exposes every field a declared
// rule names, at least one of that command's flag Usage strings must
// state the rule's canonical Sentence — referenced from the registry,
// never retyped here.
func TestSurfaceConformanceCobraUsage(t *testing.T) {
	commands := nonHiddenCommands()

	// unionExposed is every field ANY non-hidden command's flags expose,
	// across the whole cobra tree — this is the cobra_usage surface's
	// COMPLETE exposed field set, the same flat shape ApplicableSurfaces
	// expects (D-08). A rule whose fields do not ALL appear somewhere in
	// this union genuinely does not apply to cobra_usage at all (e.g. the
	// paging trio's cursor_mode/offset/page_token, or schedule_memory's
	// not_before/not_after — schedule_memory has no CLI command), and that
	// is not a violation: it is the same "resolves empty on this surface"
	// outcome D-08's worked example names for MCP jsonschema. Skipping it
	// here is what keeps this test from demanding rule text on a command
	// that structurally cannot carry it.
	unionExposed := make([]string, 0, len(commands)*8)
	for _, cmd := range commands {
		unionExposed = append(unionExposed, cobraExposedFields(cmd)...)
	}

	for _, rule := range surfaces.Rules() {
		applicableAtAll := surfaces.ApplicableSurfaces(rule, map[surfaces.Surface][]string{
			surfaces.SurfaceCobraUsage: unionExposed,
		})
		if !containsSurface(applicableAtAll, surfaces.SurfaceCobraUsage) {
			continue // this rule's fields don't exist anywhere in the cobra tree — not applicable, not a violation
		}

		matched := 0
		for _, cmd := range commands {
			exposed := map[surfaces.Surface][]string{
				surfaces.SurfaceCobraUsage: cobraExposedFields(cmd),
			}
			applicable := surfaces.ApplicableSurfaces(rule, exposed)
			if !containsSurface(applicable, surfaces.SurfaceCobraUsage) {
				continue
			}
			matched++

			found := false
			for _, f := range collectFlags(cmd) {
				if strings.Contains(f.Usage, rule.Sentence) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("rule=%s surface=cobra_usage command=%s: no flag Usage string contains %q", rule.ID, cmd.Name(), rule.Sentence)
			}
		}
		if matched == 0 {
			t.Errorf("rule=%s surface=cobra_usage: no command's flag set exposed this rule's fields", rule.ID)
		}
	}
}

// nonHiddenCommands returns EVERY non-hidden, non-help, non-completion
// command in rootCmd's whole tree, at any depth — walkCommands(rootCmd,
// commandWalkSkip), the same shared walker and skip predicate buildCatalog
// uses, factored out so the union pass and the per-command pass iterate the
// identical set. Widening this to the whole tree (not just rootCmd's direct
// children) is what makes TestSurfaceConformanceCobraUsage's unionExposed
// actually reach a nested leaf's flags: a single-level nonHiddenCommands
// would make every rule this phase registers on a spine-review leaf
// resolve "not applicable to cobra_usage" via the ApplicableSurfaces skip
// below, with the suite staying green (T-03-23).
func nonHiddenCommands() []*cobra.Command {
	return walkCommands(rootCmd, commandWalkSkip)
}

// TestNonHiddenCommandsReachesNestedLeaves is the behavioral gate behind
// the walker-conversion grep: it asserts the commandKey set of
// nonHiddenCommands() equals the commandKey set of walkCommands(rootCmd,
// commandWalkSkip) directly (they should be literally the same call, so
// this also catches nonHiddenCommands drifting to a different skip
// predicate), AND that at least one member contains a space — proof the
// union actually reaches a nested leaf, not merely a stub function that
// happens to delegate today. This is what would go red on exactly the miss
// that lets TestSurfaceConformanceCobraUsage silently skip every rule this
// phase registers on a spine-review leaf (T-03-23).
func TestNonHiddenCommandsReachesNestedLeaves(t *testing.T) {
	keySet := func(cmds []*cobra.Command) map[string]bool {
		m := make(map[string]bool, len(cmds))
		for _, c := range cmds {
			m[commandKey(c)] = true
		}
		return m
	}

	got := keySet(nonHiddenCommands())
	want := keySet(walkCommands(rootCmd, commandWalkSkip))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("nonHiddenCommands() keys = %v, want %v", got, want)
	}

	nested := false
	for k := range got {
		if strings.Contains(k, " ") {
			nested = true
			break
		}
	}
	if !nested {
		t.Errorf("nonHiddenCommands() = %v, want at least one nested leaf (a key containing a space)", got)
	}
}
