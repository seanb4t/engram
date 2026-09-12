// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import "strings"

// safeArgRunes is D-02's minimal POSIX safe set: a word made ONLY of these
// runes renders bare in Action.Command()/Plan.Display(). This is the
// display half of issue #523 (02-SECURITY.md R-02-01) — argv form
// (Args, exec.CommandContext) is the execution half, closed in apply.go.
// Ordinary URLs (scheme, host, path segments, query) are entirely within
// this set, so the previewed command stays byte-identical to Phase 2's
// hand-authored strings for every URL this binary has ever pinned in a
// test or doc. A space, a shell metacharacter (; | $ ` &), or any rune
// outside this set forces single-quoting.
const safeArgRunes = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
	"abcdefghijklmnopqrstuvwxyz" +
	"0123456789" +
	"_@%+=:,./-"

// isSafeArgRune reports whether r may appear in a bare (unquoted) rendered
// word.
func isSafeArgRune(r rune) bool {
	return strings.ContainsRune(safeArgRunes, r)
}

// quoteWord renders one argv element for display: bare when every rune is
// in safeArgRunes, otherwise wrapped in single quotes with each embedded
// single quote replaced by the four-character escape sequence `'\”` —
// the standard POSIX shell idiom for "close the quote, emit an escaped
// quote, reopen the quote". The empty string renders as an explicitly
// quoted empty word (”), never as nothing, so an empty argv element is
// never silently absorbed into the surrounding whitespace when a human
// reads the rendered line.
func quoteWord(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for _, r := range s {
		if !isSafeArgRune(r) {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// quoteArgs renders a full argv as a single display string: each element
// quoted independently via quoteWord, joined by a single space. This is
// the sole implementation Action.Command() and Plan.Display() delegate to
// (plan.go) — display is a pure function of Args, never independently
// authored (D-01).
func quoteArgs(args []string) string {
	words := make([]string, len(args))
	for i, a := range args {
		words[i] = quoteWord(a)
	}
	return strings.Join(words, " ")
}
