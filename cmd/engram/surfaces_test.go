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
	for _, rule := range surfaces.Rules() {
		matched := 0
		for _, cmd := range rootCmd.Commands() {
			if cmd.Hidden || cmd.Name() == "help" || cmd.Name() == "completion" {
				continue
			}
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
