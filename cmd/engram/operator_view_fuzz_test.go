// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import "testing"

func FuzzSanitizeViewValue(f *testing.F) {
	for _, seed := range []string{"", "plain", "x\ny\r\t\x1b\x7fz", "repo:github.com/o/r", "naïve ✓"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		for _, r := range sanitizeViewValue(s) {
			if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
				t.Fatalf("sanitizeViewValue(%q) kept control rune %U", s, r)
			}
		}
	})
}
