// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// migrateGuideRelPath is the path to the migrate guide, relative to this
// package's own directory (cmd/engram). Two levels up reaches the repo
// root (cmd/engram -> cmd -> repo root) -- `go test` always runs with the
// package directory as its working directory, so a plain relative read
// resolves correctly with no repo-root lookup needed. Mirrors
// upgradeGuideRelPath's idiom in docsync_test.go.
const migrateGuideRelPath = "../../docs-site/src/content/docs/guides/migrate.md"

// migrateGuideStaleClaimAnchors are the two false claims audit item W3
// found in the guide's `pending` row. Both must occur zero times in
// docs-site/ once the row is corrected.
var migrateGuideStaleClaimAnchors = []string{
	// "the equivalent number from" carries no verb and therefore cannot
	// inflect. The milestone audit recorded a near-miss where searching for
	// the singular verb form ("derives the equivalent") returned nothing
	// and read as "already fixed", while the actual sentence ("derive the
	// equivalent" -- plural subject, uninflected verb) was fully intact.
	// Anchoring on a verb-free noun phrase closes that false-negative
	// class off entirely.
	"the equivalent number from",
	// "Connect lane only" pins the second false claim: the field became
	// wrong the moment plan 09-01 landed pending in both the CLI's text
	// and json lanes too.
	"Connect lane only",
}

// pendingRowPattern matches the single markdown table row beginning with
// the backticked cell "`pending`", capturing the whole line so callers can
// inspect its content for the required boundary and exclusion tokens.
var pendingRowPattern = regexp.MustCompile("(?m)^\\|\\s*`pending`\\s*\\|.*\\|\\s*$")

// migrateGuidePendingRowViolations checks doc for the five ways the
// `pending` row of the shared `engram migrate status` / `engram
// migration-status` field table can be wrong. It returns one error per
// violation so a caller can report every problem in a single run rather
// than stopping at the first. Both TestMigrateGuidePendingRowIsAccurate
// (against the live guide) and TestMigrateGuidePendingRowGateFiresOnInjectedViolation
// (against in-memory fixtures) call this same function, so the positive
// control exercises the real gate rather than a parallel reimplementation.
func migrateGuidePendingRowViolations(doc string) []error {
	var errs []error

	// Leg 1: zero remaining occurrences of either stale claim anchor.
	// Gated on ZERO, never on a conversion count -- a count gate would
	// silently pass if a new occurrence landed elsewhere after this fix.
	for _, anchor := range migrateGuideStaleClaimAnchors {
		if n := strings.Count(doc, anchor); n != 0 {
			errs = append(errs, fmt.Errorf("%s: stale claim anchor %q still present (%d occurrence(s)) -- the pending row must not restate this false claim", migrateGuideRelPath, anchor, n))
		}
	}

	// Legs 2-4: the row must survive as a row of the table (not be
	// deleted to dodge leg 1), and it must name both the current_version
	// boundary and the future exclusion.
	row := pendingRowPattern.FindString(doc)
	if row == "" {
		errs = append(errs, fmt.Errorf("%s: no `pending` table row found -- the row must be corrected in place, never deleted", migrateGuideRelPath))
	} else {
		if !strings.Contains(row, "current_version") {
			errs = append(errs, fmt.Errorf("%s: `pending` row does not name `current_version` -- the row must state the strictly-below boundary", migrateGuideRelPath))
		}
		if !strings.Contains(row, "future") {
			errs = append(errs, fmt.Errorf("%s: `pending` row does not name the `future` exclusion", migrateGuideRelPath))
		}
	}

	// Leg 5: the protojson uint64-as-string encoding paragraph below the
	// row must survive untouched -- it describes an encoding difference,
	// not a field-presence difference, and stays true after the
	// correction.
	if !strings.Contains(doc, "protojson") || !strings.Contains(doc, "uint64") {
		errs = append(errs, fmt.Errorf("%s: the protojson/uint64 encoding paragraph below the `pending` row is missing", migrateGuideRelPath))
	}

	return errs
}

// TestMigrateGuidePendingRowIsAccurate is the W3 closing documentation
// gate: it asserts the guide's `pending` row no longer claims the field is
// Connect-lane-only and no longer claims the CLI re-derives an equivalent
// number, while the row itself, its boundary/exclusion tokens, and the
// encoding paragraph below it all remain present.
//
// Guarded against a green-but-untested pass: if the guide is absent (a
// trimmed checkout), this test skips explicitly rather than silently
// passing having read nothing; it also fails outright if the file content
// is empty.
func TestMigrateGuidePendingRowIsAccurate(t *testing.T) {
	data, err := os.ReadFile(migrateGuideRelPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("migrate guide not present at %s (trimmed checkout?) -- skipping rather than passing silently", migrateGuideRelPath)
		}
		t.Fatalf("read %s: %v", migrateGuideRelPath, err)
	}
	if len(data) == 0 {
		t.Fatalf("%s is empty", migrateGuideRelPath)
	}

	for _, err := range migrateGuidePendingRowViolations(string(data)) {
		t.Error(err)
	}
}

// TestMigrateGuidePendingRowGateFiresOnInjectedViolation is the D-07
// positive control: it proves migrateGuidePendingRowViolations
// discriminates a clean fixture from each of the six ways this plan's
// correction could go wrong, entirely over in-memory strings so no file is
// ever mutated. The "clean" case is as load-bearing as the violation
// cases -- a gate that fires on everything discriminates nothing.
func TestMigrateGuidePendingRowGateFiresOnInjectedViolation(t *testing.T) {
	rowLine := "| `pending` | `absent` plus every bucket strictly below `current_version` (excluding `future`) -- reported by all three surfaces, all reading the same `MigrateStatusResult.Pending()`. |"
	encodingNote := "The Connect lane differs in encoding: protojson emits `uint64` as a string."
	cleanFixture := rowLine + "\n\n" + encodingNote + "\n"

	cases := []struct {
		name            string
		fixture         string
		expectViolation bool
	}{
		{"clean", cleanFixture, false},
		{"stale_derivation_claim", cleanFixture + migrateGuideStaleClaimAnchors[0], true},
		{"stale_connect_lane_claim", cleanFixture + migrateGuideStaleClaimAnchors[1], true},
		{"row_deleted", "", true},
		{"row_omits_boundary", strings.Replace(cleanFixture, "current_version", "", 1), true},
		{"row_omits_future_exclusion", strings.Replace(cleanFixture, "future", "", 1), true},
		{"encoding_note_removed", rowLine + "\n", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			violations := migrateGuidePendingRowViolations(c.fixture)
			if got := len(violations) > 0; got != c.expectViolation {
				t.Errorf("fixture %q: violations=%v (count %d), want expectViolation=%v", c.name, violations, len(violations), c.expectViolation)
			}
		})
	}
}
