// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/surfaces"
)

// cobraExposedFields returns the flag-name set collectFlags derives for
// cmd — the SAME live-tree walk buildCatalog already uses (catalog.go),
// never a second traversal.
func cobraExposedFields(cmd *cobra.Command) []string {
	flags := collectFlags(rootCmd, cmd)
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
			for _, f := range collectFlags(rootCmd, cmd) {
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

// nonHiddenCommands returns rootCmd's non-hidden, non-help, non-completion
// subcommands — the exact predicate TestSurfaceConformanceCobraUsage's loop
// already applied inline; factored out so the union pass and the per-command
// pass iterate the identical set.
func nonHiddenCommands() []*cobra.Command {
	var out []*cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Hidden || cmd.Name() == "help" || cmd.Name() == "completion" {
			continue
		}
		out = append(out, cmd)
	}
	return out
}
