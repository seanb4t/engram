// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"testing"
	"time"
)

// TestPruneCutoffAppliesGracePeriod pins the prune-expired --older-than arithmetic
// (hr2.9): the cutoff is now minus the grace period, so records whose not_after
// lapsed more recently than the grace window are spared. older-than=0 prunes
// anything already past not_after.
func TestPruneCutoffAppliesGracePeriod(t *testing.T) {
	now := time.Date(2031, 1, 2, 15, 0, 0, 0, time.UTC)

	if got := pruneCutoff(now, 0); !got.Equal(now) {
		t.Errorf("pruneCutoff(now, 0) = %v, want %v (no grace → cutoff is now)", got, now)
	}

	want := now.Add(-24 * time.Hour)
	if got := pruneCutoff(now, 24*time.Hour); !got.Equal(want) {
		t.Errorf("pruneCutoff(now, 24h) = %v, want %v (now minus grace)", got, want)
	}
}
