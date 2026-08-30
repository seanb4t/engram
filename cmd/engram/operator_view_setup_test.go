// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

// This file holds this plan's (02-01) setup command fixtures, modeled on
// operator_view_flat_test.go: it is merged into operatorViewFixtures()
// (operator_output_test.go) and gates the merged key set against
// operatorCommands() in BOTH directions — adding a command without adding
// its fixture here is exactly what that gate exists to catch.

import (
	"fmt"
	"testing"
)

// setupViewFixtures returns one setupReportDoc per report shape —
// all-present, all-absent, and mixed — built from the real
// setupRuntimeRow/setupReportDoc types this plan declares in setup.go,
// never a hand-rolled map, keyed by commandKey as operatorCommands()
// produces it.
func setupViewFixtures() map[string][]any {
	present := setupRuntimeRow{
		Name:    "claude-code",
		Present: true,
		Outcome: "would-write",
		Command: "claude mcp add --transport http engram https://engram.example.com/mcp --scope user",
	}
	absent := setupRuntimeRow{
		Name:    "claude-code",
		Present: false,
		Outcome: "not-present",
	}
	codexPresent := setupRuntimeRow{
		Name:    "codex",
		Present: true,
		Outcome: "would-write",
		Command: "codex mcp add engram --url https://engram.example.com/mcp",
	}
	openCodeAbsent := setupRuntimeRow{
		Name:    "opencode",
		Present: false,
		Outcome: "not-present",
	}
	bearerMode := setupRuntimeRow{
		Name:    "claude-code",
		Present: true,
		Outcome: "would-write",
		Command: `claude mcp add --transport http engram https://engram.example.com/mcp --scope user --header "Authorization: Bearer <from /home/u/.engram/token>"`,
	}
	unsupportedMode := setupRuntimeRow{
		Name:    "opencode",
		Present: true,
		Outcome: "failed",
		Reason:  "opencode: auth mode \"oauth-client\": setup: auth mode is not supported by this runtime",
	}
	// applyAttemptedFailed and applyNotPresent model the --apply-shaped
	// document setupApplyRun renders (02-02 Task 3): every present runtime
	// is marked failed (Apply is stubbed, D-09), and a not-present runtime
	// keeps its OutcomeNotPresent row untouched (D-07).
	applyAttemptedFailed := setupRuntimeRow{
		Name:    "claude-code",
		Present: true,
		Outcome: "failed",
		Reason:  "setup: --apply is not implemented yet (Phase 3) — run `engram setup` (without --apply) to preview the invocation it would issue",
	}
	applyNotPresent := setupRuntimeRow{
		Name:    "codex",
		Present: false,
		Outcome: "not-present",
	}

	return map[string][]any{
		"setup": {
			setupReportDoc{Runtimes: []setupRuntimeRow{present}},         // all-present
			setupReportDoc{Runtimes: []setupRuntimeRow{absent}},          // all-absent
			setupReportDoc{Runtimes: []setupRuntimeRow{present, absent}}, // mixed (two)
			// Task 2: the three-runtime and mixed-presence shapes over
			// every registered runtime.
			setupReportDoc{Runtimes: []setupRuntimeRow{present, codexPresent, openCodeAbsent}},
			// Task 3: a bearer-mode fixture (credential redacted by
			// provenance, D-16), an unsupported-mode fixture (opencode x
			// oauth-client, outcome=failed), and an apply-shaped mixed
			// not-present/failed fixture covering the apply lane.
			setupReportDoc{Runtimes: []setupRuntimeRow{bearerMode}},
			setupReportDoc{Runtimes: []setupRuntimeRow{unsupportedMode}},
			setupReportDoc{Runtimes: []setupRuntimeRow{applyAttemptedFailed, applyNotPresent}},
		},
	}
}

// TestSetupViewIdentity runs the shared identity gate (assertViewIdentity,
// operator_view_test.go) over every setupViewFixtures() document.
func TestSetupViewIdentity(t *testing.T) {
	for name, docs := range setupViewFixtures() {
		for i, doc := range docs {
			t.Run(fmt.Sprintf("%s/%d", name, i), func(t *testing.T) {
				assertViewIdentity(t, name, doc)
			})
		}
	}
}
