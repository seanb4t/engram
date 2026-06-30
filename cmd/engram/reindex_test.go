// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"strings"
	"testing"

	"github.com/seanb4t/engram/internal/store"
)

func TestReindexSummary(t *testing.T) {
	t.Run("dry run reports the scanned count and is marked dry-run", func(t *testing.T) {
		got := reindexSummary(store.ReindexResult{Scanned: 5}, "new_coll", 1536, true)
		if !strings.HasPrefix(got, "dry-run:") {
			t.Errorf("dry-run summary should be marked as such: %q", got)
		}
		for _, want := range []string{"5 record", "new_coll", "1536"} {
			if !strings.Contains(got, want) {
				t.Errorf("dry-run summary %q missing %q", got, want)
			}
		}
	})

	t.Run("real run reports upserted/scanned, skipped, unchanged, and the cutover hint", func(t *testing.T) {
		got := reindexSummary(store.ReindexResult{Scanned: 5, Upserted: 2, Skipped: 1, Unchanged: 2}, "new_coll", 1536, false)
		if strings.HasPrefix(got, "dry-run:") {
			t.Errorf("non-dry-run summary must not be marked dry-run: %q", got)
		}
		for _, want := range []string{"2/5 record", "1 skipped", "2 unchanged", "at dim 1536", "ENGRAM_QDRANT_COLLECTION=new_coll"} {
			if !strings.Contains(got, want) {
				t.Errorf("summary %q missing %q", got, want)
			}
		}
	})
}
