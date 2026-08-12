// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"os"
	"strings"
	"testing"
)

const (
	curatingMemorySkillPath = "../../skill/engram/skills/curating-memory/SKILL.md"
	curatingSpineSkillPath  = "../../skill/engram/skills/curating-spine/SKILL.md"
	verbTableHeader         = "| Situation | Tool | Why |"
	// verbTableAnchor uniquely identifies the D-09 supersede/update/delete
	// verb-selection table's first data row, distinguishing it from
	// curating-memory/SKILL.md's other "| Situation | Tool | Why |" table
	// (the unrelated rule-correction table a few hundred lines earlier).
	verbTableAnchor = "The old fact *was* true and is now wrong"
)

// extractVerbTable returns the verb-selection table whose first data row
// contains verbTableAnchor, as a slice of whitespace-trimmed lines starting
// at its header row and continuing until the first line that is not part of
// a markdown table (does not start with "|" once leading indentation is
// trimmed).
func extractVerbTable(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	lines := strings.Split(string(data), "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == verbTableHeader &&
			i+2 < len(lines) && strings.Contains(lines[i+2], verbTableAnchor) {
			start = i
			break
		}
	}
	if start == -1 {
		t.Fatalf("%s: verb-selection table anchored by %q not found", path, verbTableAnchor)
	}
	var table []string
	for _, line := range lines[start:] {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			break
		}
		table = append(table, trimmed)
	}
	return table
}

// TestCuratingSpineVerbTableMatchesCuratingMemory binds
// curating-spine/SKILL.md's "Choosing the verb" table to
// curating-memory/SKILL.md's verb-selection table (D-09 of
// .planning/phases/04-spine-curation-semantic-skill/04-01-PLAN.md requires
// the two stay byte-identical, whitespace-normalized). Without this test the
// two tables can drift silently — an edit to one file's wording is caught
// only by an attentive reviewer noticing during an unrelated diff. Mirrors
// the doc-binding pattern TestSupersedeDocsMatchShippedContract establishes
// for errors.md.
func TestCuratingSpineVerbTableMatchesCuratingMemory(t *testing.T) {
	memoryTable := extractVerbTable(t, curatingMemorySkillPath)
	spineTable := extractVerbTable(t, curatingSpineSkillPath)

	memoryNorm := normalizeWS(strings.Join(memoryTable, "\n"))
	spineNorm := normalizeWS(strings.Join(spineTable, "\n"))

	if memoryNorm != spineNorm {
		t.Errorf("verb-selection table drifted between %s and %s (D-09 requires whitespace-normalized byte-identity):\n%s: %s\n%s: %s",
			curatingMemorySkillPath, curatingSpineSkillPath,
			curatingMemorySkillPath, memoryNorm,
			curatingSpineSkillPath, spineNorm)
	}
}
