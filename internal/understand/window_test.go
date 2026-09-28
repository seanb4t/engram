// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package understand

import (
	"testing"
	"time"
)

// TestWindow proves Window's D-07/D-11 day-aligned bucket-to-window
// conversion: each bucket's after/label pair, a non-UTC now resolving to
// the correct UTC calendar day, and an unrecognized bucket reporting ok
// false.
func TestWindow(t *testing.T) {
	now := time.Date(2026, 9, 28, 14, 23, 5, 0, time.UTC)

	cases := []struct {
		bucket    string
		wantAfter string
		wantLabel string
	}{
		{"today", "2026-09-28T00:00:00Z", "today"},
		{"past_week", "2026-09-21T00:00:00Z", "past week"},
		{"past_month", "2026-08-28T00:00:00Z", "past month"},
		{"past_year", "2025-09-28T00:00:00Z", "past year"},
	}
	for _, tc := range cases {
		t.Run(tc.bucket, func(t *testing.T) {
			after, before, label, ok := Window(tc.bucket, now)
			if !ok {
				t.Fatal("ok = false, want true")
			}
			if after != tc.wantAfter {
				t.Errorf("after = %q, want %q", after, tc.wantAfter)
			}
			if before != "" {
				t.Errorf("before = %q, want empty", before)
			}
			if label != tc.wantLabel {
				t.Errorf("label = %q, want %q", label, tc.wantLabel)
			}
		})
	}

	t.Run("non-UTC now resolves to the UTC calendar day", func(t *testing.T) {
		// UTC-7 at 20:00 local on 2026-09-27 is 2026-09-28T03:00:00Z — the
		// UTC day is the 28th, not the 27th a naive local-day read would give.
		loc := time.FixedZone("UTC-7", -7*60*60)
		localNow := time.Date(2026, 9, 27, 20, 0, 0, 0, loc)
		after, _, _, ok := Window("today", localNow)
		if !ok {
			t.Fatal("ok = false, want true")
		}
		if after != "2026-09-28T00:00:00Z" {
			t.Errorf("after = %q, want %q", after, "2026-09-28T00:00:00Z")
		}
	})

	t.Run("unrecognized buckets report ok false", func(t *testing.T) {
		for _, bucket := range []string{"none", "yesterday"} {
			t.Run(bucket, func(t *testing.T) {
				_, _, _, ok := Window(bucket, now)
				if ok {
					t.Errorf("ok = true, want false for bucket %q", bucket)
				}
			})
		}
	})
}
