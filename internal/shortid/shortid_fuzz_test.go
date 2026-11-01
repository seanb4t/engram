// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package shortid

import (
	"strings"
	"testing"
	"unicode"
)

func FuzzCanonical(f *testing.F) {
	for _, seed := range []string{"", "  0123456789  ", "ILOilo", "abcdefghjk", "ẞ"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out := Canonical(s)
		if out != strings.TrimSpace(out) {
			t.Fatalf("Canonical(%q) = %q keeps surrounding whitespace", s, out)
		}
		for _, r := range out {
			if unicode.ToLower(r) != r || r == 'i' || r == 'l' || r == 'o' {
				t.Fatalf("Canonical(%q) = %q holds %q", s, out, r)
			}
		}
	})
}
