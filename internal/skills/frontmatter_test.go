// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import (
	"strings"
	"testing"
)

// TestParseFrontmatter covers every bullet in 04-02-PLAN.md Task 1's
// behavior block using in-memory fixtures.
func TestParseFrontmatter(t *testing.T) {
	t.Run("well-formed returns name and index entry", func(t *testing.T) {
		content := "---\nname: example\nmetadata:\n  engram-summary: \"an authored index line\"\n---\n# Example\n"
		name, summary, err := ParseFrontmatter("example/SKILL.md", []byte(content))
		if err != nil {
			t.Fatalf("ParseFrontmatter: %v", err)
		}
		if name != "example" {
			t.Errorf("name = %q, want %q", name, "example")
		}
		if summary != "an authored index line" {
			t.Errorf("summary = %q, want %q", summary, "an authored index line")
		}
	})

	t.Run("no metadata map returns empty summary and no error", func(t *testing.T) {
		content := "---\nname: example\ndescription: something\n---\n# Example\n"
		name, summary, err := ParseFrontmatter("example/SKILL.md", []byte(content))
		if err != nil {
			t.Fatalf("ParseFrontmatter: %v", err)
		}
		if name != "example" {
			t.Errorf("name = %q, want %q", name, "example")
		}
		if summary != "" {
			t.Errorf("summary = %q, want empty", summary)
		}
	})

	t.Run("metadata map missing the key returns empty summary and no error", func(t *testing.T) {
		content := "---\nname: example\nmetadata:\n  other-key: value\n---\n# Example\n"
		_, summary, err := ParseFrontmatter("example/SKILL.md", []byte(content))
		if err != nil {
			t.Fatalf("ParseFrontmatter: %v", err)
		}
		if summary != "" {
			t.Errorf("summary = %q, want empty", summary)
		}
	})

	t.Run("no opening fence is an error naming the path", func(t *testing.T) {
		content := "name: example\n---\n# Example\n"
		_, _, err := ParseFrontmatter("example/SKILL.md", []byte(content))
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "example/SKILL.md") {
			t.Errorf("error %q does not name the path", err.Error())
		}
	})

	t.Run("unclosed fence is an error naming the path", func(t *testing.T) {
		content := "---\nname: example\n# Example\n"
		_, _, err := ParseFrontmatter("example/SKILL.md", []byte(content))
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "example/SKILL.md") {
			t.Errorf("error %q does not name the path", err.Error())
		}
	})

	t.Run("invalid YAML is an error naming the path", func(t *testing.T) {
		content := "---\nname: [unterminated\n---\n# Example\n"
		_, _, err := ParseFrontmatter("example/SKILL.md", []byte(content))
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "example/SKILL.md") {
			t.Errorf("error %q does not name the path", err.Error())
		}
	})
}

// TestEverySkillCarriesIndexSummary drives the REAL embedded inventory:
// for every discovered skill, assert the summary is non-empty, contains
// no newline, and is at most MaxSummaryBytes. This is what keeps a sixth
// skill authored without an index entry from ever reaching a user's
// machine — it names no skill and no count.
func TestEverySkillCarriesIndexSummary(t *testing.T) {
	inv, err := Inventory()
	if err != nil {
		t.Fatalf("Inventory(): %v", err)
	}
	if len(inv) == 0 {
		t.Fatal("Inventory() returned zero skills")
	}

	for _, s := range inv {
		if s.Summary == "" {
			t.Errorf("skill %q has an empty %s index entry", s.Name, MetadataSummaryKey)
			continue
		}
		if strings.Contains(s.Summary, "\n") {
			t.Errorf("skill %q's index entry contains a newline: %q", s.Name, s.Summary)
		}
		if len(s.Summary) > MaxSummaryBytes {
			t.Errorf("skill %q's index entry is %d bytes, want at most %d", s.Name, len(s.Summary), MaxSummaryBytes)
		}
	}
}
