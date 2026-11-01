// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package shortid

import (
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// spellings are the confusable and mixed-case forms a caller can type for
// one stored rune.
var spellings = map[rune][]string{'1': {"1", "i", "I", "l", "L"}, '0': {"0", "o", "O"}}

func TestPropertyCanonicalHandle(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		handle := rapid.StringOfN(rapid.SampledFrom([]rune(alphabet)), Length, Length, -1).Draw(t, "handle")
		var typed strings.Builder
		typed.WriteString(rapid.SampledFrom([]string{"", " ", "\t", "\n "}).Draw(t, "lead"))
		for i, r := range handle {
			forms, ok := spellings[r]
			if !ok {
				forms = []string{string(r), strings.ToUpper(string(r))}
			}
			typed.WriteString(rapid.SampledFrom(forms).Draw(t, "rune"+string(rune('a'+i))))
		}
		typed.WriteString(rapid.SampledFrom([]string{"", " ", "\r\n"}).Draw(t, "trail"))

		if got := Canonical(typed.String()); got != handle {
			t.Fatalf("Canonical(%q) = %q, want %q", typed.String(), got, handle)
		}
		if got := Canonical(handle); got != handle {
			t.Fatalf("Canonical(%q) changed a stored handle to %q", handle, got)
		}
	})
}
