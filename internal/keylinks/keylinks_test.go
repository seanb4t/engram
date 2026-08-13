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
