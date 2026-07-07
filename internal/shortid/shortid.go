// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package shortid generates and canonicalizes the short, human/agent-friendly
// handle carried alongside a memory's UUID. The handle is a fixed-length
// Crockford base32 token: case-insensitive and free of the confusable glyphs
// I, L, O, U — so an LLM cannot corrupt it by case-folding or glyph-swapping.
package shortid

import (
	"crypto/rand"
	"strings"
	"unicode"
)

// alphabet is Crockford base32, lowercase, excluding i, l, o, u.
const alphabet = "0123456789abcdefghjkmnpqrstvwxyz"

// Length is the fixed handle length. 10 symbols × 5 bits = 50 bits of entropy.
const Length = 10

// New returns a fresh handle drawn uniformly from crypto/rand. len(alphabet)==32
// divides 256, so byte%32 has no modulo bias. Uniqueness is the caller's concern
// (see Store.MintShortID).
func New() (string, error) {
	b := make([]byte, Length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, Length)
	for i, c := range b {
		out[i] = alphabet[int(c)%len(alphabet)]
	}
	return string(out), nil
}

// Canonical normalizes a caller-supplied handle to the stored form for exact
// comparison: trims whitespace, folds Crockford's confusable glyphs (i/I/l/L → 1,
// o/O → 0), and lowercases everything else.
func Canonical(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case 'i', 'I', 'l', 'L':
			b.WriteByte('1')
		case 'o', 'O':
			b.WriteByte('0')
		default:
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}
