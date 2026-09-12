// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import "testing"

// TestQuoteWord is D-02's pinned safe-set table: a later "simplification"
// of the safe set fails loudly here rather than silently reopening the
// display half of issue #523.
func TestQuoteWord(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"plain-url", "https://engram.example.com/mcp", "https://engram.example.com/mcp"},
		{"url-with-space", "https://engram.example.com/a b", "'https://engram.example.com/a b'"},
		{"semicolon", "a;b", "'a;b'"},
		{"pipe", "a|b", "'a|b'"},
		{"dollar", "a$b", "'a$b'"},
		{"backtick", "a`b", "'a`b'"},
		{"ampersand", "a&b", "'a&b'"},
		{"single-quote", "it's", `'it'\''s'`},
		{"empty", "", "''"},
		{"safe-set-runes-bare", "A-z_0-9@%+=:,./-", "A-z_0-9@%+=:,./-"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := quoteWord(tc.input); got != tc.want {
				t.Errorf("quoteWord(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestQuoteArgs proves a full argv renders as independently-quoted words
// joined by a single space.
func TestQuoteArgs(t *testing.T) {
	got := quoteArgs([]string{"codex", "mcp", "add", "engram", "--header", "Authorization: Bearer x"})
	want := `codex mcp add engram --header 'Authorization: Bearer x'`
	if got != want {
		t.Errorf("quoteArgs(...) = %q, want %q", got, want)
	}
}

// TestQuoteWordEmptyNeverAbsorbedIntoWhitespace proves the empty string
// renders as an explicitly quoted empty word, never as nothing — an empty
// argv element must never be silently invisible in the rendered line.
func TestQuoteWordEmptyNeverAbsorbedIntoWhitespace(t *testing.T) {
	got := quoteArgs([]string{"a", "", "b"})
	want := "a '' b"
	if got != want {
		t.Errorf("quoteArgs([a, \"\", b]) = %q, want %q", got, want)
	}
}
