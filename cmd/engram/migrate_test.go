// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"testing"
)

func TestRemapOwnerFlagValidation(t *testing.T) {
	cases := []struct {
		name          string
		from          string
		missing, anon bool
		to            string
		wantErr       bool
	}{
		{"no source", "", false, false, "x", true},
		{"missing ok", "", true, false, "x", false},
		{"anon ok", "", false, true, "x", false},
		{"from ok", "old", false, false, "x", false},
		{"two sources", "old", true, false, "x", true},
		{"empty to", "", true, false, "", true},
		{"ambiguous empty from", "", false, false, "x", true},
		{"from==to", "x", false, false, "x", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := buildRemapSource(c.from, c.missing, c.anon, c.to)
			if (err != nil) != c.wantErr {
				t.Errorf("buildRemapSource(%+v) err=%v, wantErr=%v", c, err, c.wantErr)
			}
		})
	}
}
