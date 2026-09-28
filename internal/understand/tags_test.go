// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package understand

import (
	"fmt"
	"strings"
	"testing"
)

// TestMatchTags proves MatchTags' D-09 local matching rule: case-
// insensitive whole-tag equality, a hyphen-separated part of 3+ runes
// equalling a query token, vocabulary order, original tag case preserved,
// applied-tag skipping, the MaxTagSuggestions cap, and an empty vocabulary
// yielding no matches.
func TestMatchTags(t *testing.T) {
	vocab := []string{"qdrant", "qdrant-ops", "CI", "go", "go-to", "sqlite"}

	t.Run("hyphen-part and whole-tag matches, vocabulary order, original case", func(t *testing.T) {
		got := MatchTags("Why does Qdrant fail in ci", vocab, nil)
		want := []string{"qdrant", "qdrant-ops", "CI"}
		if len(got) != len(want) {
			t.Fatalf("len(got) = %d, want %d: %v", len(got), len(want), got)
		}
		for i, s := range want {
			if got[i].Value != s {
				t.Errorf("got[%d].Value = %q, want %q", i, got[i].Value, s)
			}
			if got[i].Kind != KindTag {
				t.Errorf("got[%d].Kind = %q, want %q", i, got[i].Kind, KindTag)
			}
			if got[i].Source != SourceMatched {
				t.Errorf("got[%d].Source = %q, want %q", i, got[i].Source, SourceMatched)
			}
		}
	})

	t.Run("whole-token match only; go-to excluded by its short hyphen parts", func(t *testing.T) {
		got := MatchTags("go to the qdrant-ops dashboard", vocab, nil)
		want := []string{"qdrant-ops", "go"}
		if len(got) != len(want) {
			t.Fatalf("len(got) = %d, want %d: %v", len(got), len(want), got)
		}
		for i, s := range want {
			if got[i].Value != s {
				t.Errorf("got[%d].Value = %q, want %q", i, got[i].Value, s)
			}
		}
	})

	t.Run("applied tags are skipped", func(t *testing.T) {
		got := MatchTags("Why does Qdrant fail in ci", vocab, []string{"qdrant"})
		for _, s := range got {
			if s.Value == "qdrant" {
				t.Errorf("got contains applied tag %q, want skipped: %v", s.Value, got)
			}
		}
	})

	t.Run("capped at MaxTagSuggestions", func(t *testing.T) {
		bigVocab := make([]string, 12)
		for i := range bigVocab {
			bigVocab[i] = fmt.Sprintf("tagword%d", i)
		}
		query := strings.Join(bigVocab, " ")
		got := MatchTags(query, bigVocab, nil)
		if len(got) != MaxTagSuggestions {
			t.Fatalf("len(got) = %d, want %d: %v", len(got), MaxTagSuggestions, got)
		}
	})

	t.Run("empty vocab yields no matches", func(t *testing.T) {
		got := MatchTags("anything at all", nil, nil)
		if got != nil {
			t.Errorf("got = %v, want nil", got)
		}
	})
}
