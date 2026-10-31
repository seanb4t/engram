// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"strings"
	"testing"
	"time"
)

func TestDistinctWarnerAdmit(t *testing.T) {
	now := time.Unix(0, 0)
	d := newDistinctWarner(2, 4)
	d.now = func() time.Time { return now }

	type result struct {
		cut           string
		warn, atLimit bool
	}
	admit := func(value string) result {
		cut, warn, atLimit := d.admit(value)
		return result{cut, warn, atLimit}
	}
	steps := []struct {
		value string
		want  result
	}{
		{"aaaaaa", result{"aaaa", true, false}},
		{"aaaa", result{"aaaa", false, false}},
		{"bb", result{"bb", true, true}},
		{"cc", result{"cc", false, false}},
	}
	for _, s := range steps {
		if got := admit(s.value); got != s.want {
			t.Fatalf("admit(%q) = %+v, want %+v", s.value, got, s.want)
		}
	}
	now = now.Add(warnWindow)
	if got, want := admit("cc"), (result{"cc", true, false}); got != want {
		t.Fatalf("admit after the window = %+v, want %+v", got, want)
	}
}

func TestNewDistinctWarnerRefusesZeroBounds(t *testing.T) {
	for _, bounds := range [][2]int{{0, 4}, {2, 0}} {
		func() {
			defer func() {
				if r, _ := recover().(string); !strings.Contains(r, "must both be positive") {
					t.Fatalf("newDistinctWarner(%d, %d) did not panic with the bound error: %v", bounds[0], bounds[1], r)
				}
			}()
			newDistinctWarner(bounds[0], bounds[1])
		}()
	}
}
