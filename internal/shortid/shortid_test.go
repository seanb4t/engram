// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package shortid

import (
	"strings"
	"testing"
)

func TestNewShapeAndAlphabet(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 1000; i++ {
		s, err := New()
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if len(s) != Length {
			t.Fatalf("len=%d want %d (%q)", len(s), Length, s)
		}
		for _, r := range s {
			if !strings.ContainsRune(alphabet, r) {
				t.Fatalf("char %q not in alphabet (%q)", r, s)
			}
		}
		seen[s] = struct{}{}
	}
	if len(seen) != 1000 { // 50 bits over 1000 draws: collisions astronomically unlikely
		t.Fatalf("expected 1000 distinct ids, got %d", len(seen))
	}
}

func TestCanonicalFoldsGlyphsAndCase(t *testing.T) {
	cases := map[string]string{
		"  J7K2M9P4X0 ": "j7k2m9p4x0",
		"OIL":           "011", // O->0, I->1, L->1
		"abcXYZ":        "abcxyz",
	}
	for in, want := range cases {
		if got := Canonical(in); got != want {
			t.Fatalf("Canonical(%q)=%q want %q", in, got, want)
		}
	}
}
