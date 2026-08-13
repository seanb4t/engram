// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package keylinks

import (
	"path/filepath"
	"strings"
	"testing"
)

// runFixture parses name (a file under testdata/), validates every link's
// pattern, and returns the formatted offender lines — collecting all,
// never returning on the first (D-07).
func runFixture(t *testing.T, name string) []string {
	t.Helper()
	links, err := ParsePlanKeyLinks(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("ParsePlanKeyLinks(%s): %v", name, err)
	}

	var lines []string
	for _, link := range links {
		if _, off := ValidatePattern(link.Pattern); off != nil {
			off.File = link.File
			off.Line = link.Line
			lines = append(lines, OffenderLine(*off))
		}
	}
	return lines
}

// TestFixturePairEscaping is D-06's fail-first proof for the escaping
// shape: the committed known-good fixture must be silent, and the
// committed known-corrupted fixture must be loud, naming the file, the
// escaping shape, and the corrected character-class form — both in one
// test run.
func TestFixturePairEscaping(t *testing.T) {
	good := runFixture(t, "good_key_links.md")
	if len(good) != 0 {
		t.Errorf("good_key_links.md: expected zero offenders, got %d:\n%s", len(good), strings.Join(good, "\n"))
	}

	bad := runFixture(t, "bad_key_links.md")
	if len(bad) == 0 {
		t.Fatal("bad_key_links.md: expected at least one offender, got none")
	}
	joined := strings.Join(bad, "\n")
	if !strings.Contains(joined, "bad_key_links.md") {
		t.Errorf("offender output does not name bad_key_links.md:\n%s", joined)
	}
	if !strings.Contains(joined, string(ShapeEscaping)) {
		t.Errorf("offender output does not name the escaping shape:\n%s", joined)
	}
	if !strings.Contains(joined, "[.]") {
		t.Errorf("offender output does not name the corrected character-class form:\n%s", joined)
	}
}

// checkFixtureLink runs the full two-check pipeline (ValidatePattern, then
// CheckSatisfiable when the pattern validated cleanly) over one KeyLink,
// exactly as ScanPlans does under ModeSatisfiability.
func checkFixtureLink(t *testing.T, link KeyLink, repoRoot string) *Offender {
	t.Helper()
	re, off := ValidatePattern(link.Pattern)
	if off != nil {
		off.File = link.File
		off.Line = link.Line
		return off
	}
	return CheckSatisfiable(link, re, repoRoot)
}

// fixtureRepoRoot resolves the module root from this package's directory
// (internal/keylinks), so fixture from:/to: values — which are written
// relative to the repo root — resolve to real files on disk.
func fixtureRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	return root
}

// TestFixturePairSubsetAndSatisfiability is D-08's RE2 ∩ JavaScript
// common-subset proof (named groups in both syntaxes, JS-only compile
// rejections) plus D-03/D-11's satisfiability proof (the from-file-first,
// to-file-fallback resolution, and #479's second finding: a well-formed
// pattern that names a symbol present in neither file).
func TestFixturePairSubsetAndSatisfiability(t *testing.T) {
	repoRoot := fixtureRepoRoot(t)

	t.Run("named-group-go-syntax", func(t *testing.T) {
		_, off := ValidatePattern(`(?P<name>foo)`)
		if off == nil || off.Shape != ShapeNamedGroup {
			t.Fatalf("Go named-group syntax: expected ShapeNamedGroup offender, got %+v", off)
		}
	})

	t.Run("named-group-js-syntax", func(t *testing.T) {
		_, off := ValidatePattern(`(?<name>foo)`)
		if off == nil || off.Shape != ShapeNamedGroup {
			t.Fatalf("JS named-group syntax: expected ShapeNamedGroup offender, got %+v", off)
		}
	})

	t.Run("compile-error-lookahead", func(t *testing.T) {
		_, off := ValidatePattern(`(?=foo)`)
		if off == nil || off.Shape != ShapeCompileError {
			t.Fatalf("lookahead: expected ShapeCompileError offender, got %+v", off)
		}
	})

	t.Run("compile-error-negative-lookahead", func(t *testing.T) {
		_, off := ValidatePattern(`(?!foo)`)
		if off == nil || off.Shape != ShapeCompileError {
			t.Fatalf("negative lookahead: expected ShapeCompileError offender, got %+v", off)
		}
	})

	t.Run("compile-error-lookbehind", func(t *testing.T) {
		_, off := ValidatePattern(`(?<=foo)`)
		if off == nil || off.Shape != ShapeCompileError {
			t.Fatalf("lookbehind: expected ShapeCompileError offender, got %+v", off)
		}
	})

	t.Run("backreference-rejected", func(t *testing.T) {
		// A backreference necessarily contains a backslash (`\1`), so
		// D-02's unconditional, first-run escaping check (Pitfall 1)
		// catches it as ShapeEscaping before it ever reaches
		// regexp.Compile. It is still correctly rejected either way.
		_, off := ValidatePattern(`(foo)\1`)
		if off == nil {
			t.Fatal("backreference: expected an offender, got none")
		}
		if off.Shape != ShapeEscaping && off.Shape != ShapeCompileError {
			t.Fatalf("backreference: expected ShapeEscaping or ShapeCompileError, got %s", off.Shape)
		}
	})

	t.Run("good-fixture-zero-offenders-both-checks", func(t *testing.T) {
		links, err := ParsePlanKeyLinks(filepath.Join("testdata", "good_key_links.md"))
		if err != nil {
			t.Fatalf("ParsePlanKeyLinks: %v", err)
		}
		var offenders []string
		for _, link := range links {
			if off := checkFixtureLink(t, link, repoRoot); off != nil {
				offenders = append(offenders, OffenderLine(*off))
			}
		}
		if len(offenders) != 0 {
			t.Errorf("good_key_links.md: expected zero offenders under both checks, got:\n%s", strings.Join(offenders, "\n"))
		}
	})

	t.Run("good-fixture-from-missing-is-silent", func(t *testing.T) {
		links, err := ParsePlanKeyLinks(filepath.Join("testdata", "good_key_links.md"))
		if err != nil {
			t.Fatalf("ParsePlanKeyLinks: %v", err)
		}
		found := false
		for _, link := range links {
			if !strings.Contains(link.From, "does_not_exist_yet") {
				continue
			}
			found = true
			if off := checkFixtureLink(t, link, repoRoot); off != nil {
				t.Errorf("from-missing entry: expected nil offender, got %+v", off)
			}
		}
		if !found {
			t.Fatal("good_key_links.md: expected an entry whose from path does not exist")
		}
	})

	t.Run("good-fixture-to-fallback-matches", func(t *testing.T) {
		links, err := ParsePlanKeyLinks(filepath.Join("testdata", "good_key_links.md"))
		if err != nil {
			t.Fatalf("ParsePlanKeyLinks: %v", err)
		}
		found := false
		for _, link := range links {
			if !strings.Contains(link.Pattern, "GOODFIXTURE_TOFALLBACK_MARKER") {
				continue
			}
			found = true
			if off := checkFixtureLink(t, link, repoRoot); off != nil {
				t.Errorf("to-fallback entry: expected nil offender, got %+v", off)
			}
		}
		if !found {
			t.Fatal("good_key_links.md: expected a to-fallback entry")
		}
	})

	t.Run("bad-fixture-all-shapes-present", func(t *testing.T) {
		links, err := ParsePlanKeyLinks(filepath.Join("testdata", "bad_key_links.md"))
		if err != nil {
			t.Fatalf("ParsePlanKeyLinks: %v", err)
		}
		gotShapes := map[Shape]bool{}
		for _, link := range links {
			if off := checkFixtureLink(t, link, repoRoot); off != nil {
				gotShapes[off.Shape] = true
			}
		}
		for _, want := range []Shape{ShapeEscaping, ShapeNamedGroup, ShapeCompileError, ShapeUnsatisfiable} {
			if !gotShapes[want] {
				t.Errorf("bad_key_links.md: expected an offender of shape %s, found shapes: %v", want, gotShapes)
			}
		}
	})

	t.Run("bad-fixture-unsatisfiable-entry", func(t *testing.T) {
		links, err := ParsePlanKeyLinks(filepath.Join("testdata", "bad_key_links.md"))
		if err != nil {
			t.Fatalf("ParsePlanKeyLinks: %v", err)
		}
		var target *KeyLink
		for i := range links {
			if strings.Contains(links[i].Pattern, "BADFIXTURE_UNSATISFIABLE_SYMBOL_XYZ") {
				target = &links[i]
			}
		}
		if target == nil {
			t.Fatal("bad_key_links.md: expected the unsatisfiable-shape entry")
		}
		off := checkFixtureLink(t, *target, repoRoot)
		if off == nil || off.Shape != ShapeUnsatisfiable {
			t.Fatalf("expected ShapeUnsatisfiable offender, got %+v", off)
		}
	})
}
