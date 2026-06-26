// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"strings"
	"testing"

	"github.com/seanb4t/engram/internal/store"
)

func TestSummarizeSummary(t *testing.T) {
	dry := summarizeSummary(store.SummarizeResult{Scanned: 10, Filled: 4, Skipped: 6}, true)
	if !strings.Contains(dry, "dry-run") || !strings.Contains(dry, "4") {
		t.Fatalf("dry-run wording: %q", dry)
	}
	live := summarizeSummary(store.SummarizeResult{Scanned: 10, Filled: 4, Skipped: 5, Failed: 1}, false)
	if strings.Contains(live, "dry-run") || !strings.Contains(live, "1 failed") {
		t.Fatalf("live wording: %q", live)
	}
}
