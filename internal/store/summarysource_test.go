// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import "testing"

// TestSummarySourceConstants pins the canonical wire/payload strings the
// SummarySource constants emit (see the type doc for why they are locked).
func TestSummarySourceConstants(t *testing.T) {
	cases := []struct {
		name string
		got  SummarySource
		want string
	}{
		{"client", SummarySourceClient, "client"},
		{"auto", SummarySourceAuto, "auto"},
		{"none", SummarySourceNone, ""},
	}
	for _, c := range cases {
		if string(c.got) != c.want {
			t.Errorf("%s: got %q, want %q", c.name, string(c.got), c.want)
		}
	}
	// The zero value must be "none" so a freshly constructed Memory reads as
	// no-summary-source without an explicit assignment.
	var zero SummarySource
	if zero != SummarySourceNone {
		t.Errorf("zero SummarySource = %q, want %q (SummarySourceNone)", zero, SummarySourceNone)
	}
}
