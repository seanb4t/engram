// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package skills embeds the shipped curation skills into the binary and
// installs them for a runtime, either natively or via an AGENTS.md
// delimited-block fallback.
//
// frontmatter.go parses the one-line index entry each shipped SKILL.md
// authors at the Agent Skills specification's own `metadata` extension
// point (D-14). It is the sole reason this package carries a third-party
// import at all: go.yaml.in/yaml/v3 is the maintained successor to the
// archived gopkg.in/yaml.v3 (unmaintained April 2025), already resolved
// in go.sum and reached transitively today by cobra/doc and the buf
// tool. Promoting it to a direct dependency here is a deliberate,
// user-locked decision against repo rule xvqj44e5mk, which prefers an
// established upstream over a hand-rolled parser — the standing "zero
// new Go dependencies" constraint is about supply-chain surface and
// shipped-binary linkage, not a mandate to reimplement a solved problem.
// gopkg.in/yaml.v3 must never be promoted alongside it.
package skills

import (
	"bytes"
	"fmt"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// MetadataSummaryKey is the frontmatter `metadata` map key whose value IS
// a shipped skill's index entry, verbatim — authored, never derived by
// truncating `description` (D-14). Exported as a constant so this file's
// test and the AGENTS.md renderer share one definition rather than
// restating the literal "engram-summary" in two places.
const MetadataSummaryKey = "engram-summary"

// MaxSummaryBytes bounds an index entry's length, mirroring
// ENGRAM_MEMORY_MAX_SUMMARY_BYTES's default (CLAUDE.md § "Memory
// contract") — the precedent this bound follows. Exported for the same
// share-not-restate reason as MetadataSummaryKey.
const MaxSummaryBytes = 512

// fenceLine is the exact three-hyphen line that opens and closes a
// SKILL.md's YAML frontmatter block.
const fenceLine = "---"

// frontmatterNameDoc and frontmatterMetadataDoc are each unmarshaled from
// an ISOLATED single-top-level-key fragment of the frontmatter document
// (see extractTopLevelKeyBlock), never from the whole document at once.
// This split is deliberate, not incidental: at least one shipped skill's
// `description` value contains ": " (a colon immediately followed by a
// space) in ordinary prose ("This skill judges and proposes: it never
// mutates..."), which YAML's plain-scalar grammar treats as an ambiguous
// mapping-key indicator and refuses to parse — confirmed live against
// go.yaml.in/yaml/v3 on this exact file. `description` must stay
// byte-identical to what the plugin ships (D-14's own prohibition), so
// this file never feeds it to the parser at all. Extracting and parsing
// only the `name` and `metadata` fragments means a pre-existing
// formatting quirk anywhere else in the frontmatter can never break the
// one piece of structure this package actually depends on.
type frontmatterNameDoc struct {
	Name string `yaml:"name"`
}

type frontmatterMetadataDoc struct {
	Metadata map[string]string `yaml:"metadata"`
}

// ParseFrontmatter locates the leading `---`-fenced YAML frontmatter
// block in content and returns the skill's `name` and its authored
// `metadata.engram-summary` index entry.
//
// A missing opening fence, a missing closing fence, or invalid YAML
// within the `name` or `metadata` fragments is an error naming path. A
// frontmatter document with no `metadata` map, or a `metadata` map with
// no MetadataSummaryKey entry, is NOT an error — summary is returned
// empty, and the caller (this package's own test suite, via
// TestEverySkillCarriesIndexSummary) decides what to do with an absent
// entry. This split is deliberate: a missing index entry must fail
// engram's own build, never a user's install.
//
// A PRESENT summary containing a newline, or exceeding MaxSummaryBytes,
// IS an error naming path (WR-04): D-14's "authored, single-line" contract
// was previously enforced only by TestEverySkillCarriesIndexSummary
// against whatever the currently embedded inventory happens to contain —
// a YAML block-scalar `summary: |` value would parse successfully and
// pass straight through to RenderBlock, silently emitting a malformed
// multi-line bullet into the spliced AGENTS.md index block. Rejecting it
// here makes the invariant structurally impossible to embed, rather than
// merely tested if the gate happens to run before the binary is built.
func ParseFrontmatter(path string, content []byte) (name string, summary string, err error) {
	allLines := bytes.Split(content, []byte("\n"))
	if len(allLines) == 0 || string(bytes.TrimRight(allLines[0], "\r")) != fenceLine {
		return "", "", fmt.Errorf("skills: %s: does not begin with a %q frontmatter fence", path, fenceLine)
	}

	closeIdx := -1
	for i := 1; i < len(allLines); i++ {
		if string(bytes.TrimRight(allLines[i], "\r")) == fenceLine {
			closeIdx = i
			break
		}
	}
	if closeIdx == -1 {
		return "", "", fmt.Errorf("skills: %s: frontmatter fence opened but never closed", path)
	}

	docLines := allLines[1:closeIdx]

	if nameBlock := extractTopLevelKeyBlock(docLines, "name"); nameBlock != nil {
		var doc frontmatterNameDoc
		if err := yaml.Unmarshal(nameBlock, &doc); err != nil {
			return "", "", fmt.Errorf("skills: %s: parse frontmatter %q field: %w", path, "name", err)
		}
		name = doc.Name
	}

	if metaBlock := extractTopLevelKeyBlock(docLines, "metadata"); metaBlock != nil {
		var doc frontmatterMetadataDoc
		if err := yaml.Unmarshal(metaBlock, &doc); err != nil {
			return "", "", fmt.Errorf("skills: %s: parse frontmatter %q field: %w", path, "metadata", err)
		}
		if doc.Metadata != nil {
			summary = doc.Metadata[MetadataSummaryKey]
		}
	}

	if strings.Contains(summary, "\n") {
		return "", "", fmt.Errorf("skills: %s: %s %q value contains a newline — it must be authored as a single line (D-14)", path, MetadataSummaryKey, summary)
	}
	if len(summary) > MaxSummaryBytes {
		return "", "", fmt.Errorf("skills: %s: %s is %d bytes, want at most %d (D-14)", path, MetadataSummaryKey, len(summary), MaxSummaryBytes)
	}

	return name, summary, nil
}

// extractTopLevelKeyBlock scans lines (already stripped of the enclosing
// `---` fences) for a top-level (column-0) key line matching exactly
// "key:" or beginning with "key: ", and returns that line plus every
// following line that is either blank or indented (part of the same
// block, per ordinary YAML block-mapping structure) — stopping at the
// next column-0 line or the end of lines. Returns nil when key is not
// found at all. The returned block is a minimal, standalone YAML
// fragment safe to unmarshal on its own, independent of whatever any
// other top-level key's value contains.
func extractTopLevelKeyBlock(lines [][]byte, key string) []byte {
	prefix := []byte(key + ":")

	startIdx := -1
	for i, line := range lines {
		if bytes.HasPrefix(line, prefix) && (len(line) == len(prefix) || line[len(prefix)] == ' ') {
			startIdx = i
			break
		}
	}
	if startIdx == -1 {
		return nil
	}

	endIdx := len(lines)
	for i := startIdx + 1; i < len(lines); i++ {
		l := lines[i]
		if len(l) == 0 {
			continue
		}
		if l[0] != ' ' && l[0] != '\t' {
			endIdx = i
			break
		}
	}

	return bytes.Join(lines[startIdx:endIdx], []byte("\n"))
}
