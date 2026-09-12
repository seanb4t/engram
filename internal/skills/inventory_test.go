// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import (
	"testing"
	"testing/fstest"
)

// fakeSkillMD builds a minimal, well-formed SKILL.md fixture (a valid
// frontmatter fence, a name, and an authored metadata.engram-summary
// entry) for a fake in-memory skill named name — walkSkills now requires
// every discovered skill's SKILL.md to parse via ParseFrontmatter (Task
// 1), so a bare content string is no longer a valid fixture.
func fakeSkillMD(name string) string {
	return "---\nname: " + name + "\nmetadata:\n  engram-summary: \"fake index entry for " + name + "\"\n---\n# " + name + "\n"
}

// TestInventoryIsStructural proves the walk is a structural predicate,
// never an enumeration (D-04): layering an additional skill directory and
// an additional nested file over an in-memory filesystem view — the real
// embedded FS cannot be extended at runtime to prove this directly —
// makes both appear with no Go code change. walkSkills is the unexported
// function Inventory() itself calls against the embedded FS; driving it
// here exercises the SAME code path the binary does.
func TestInventoryIsStructural(t *testing.T) {
	fsys := fstest.MapFS{
		"data/alpha/SKILL.md":            &fstest.MapFile{Data: []byte(fakeSkillMD("alpha"))},
		"data/alpha/references/notes.md": &fstest.MapFile{Data: []byte("alpha reference notes")},
		"data/zeta/SKILL.md":             &fstest.MapFile{Data: []byte(fakeSkillMD("zeta"))},
	}

	discovered, err := walkSkills(fsys)
	if err != nil {
		t.Fatalf("walkSkills: %v", err)
	}

	var alpha, zeta *Skill
	for i := range discovered {
		switch discovered[i].Name {
		case "alpha":
			alpha = &discovered[i]
		case "zeta":
			zeta = &discovered[i]
		}
	}

	if zeta == nil {
		t.Fatal("walkSkills did not discover the newly-added skill directory — the walk is not structural (an enumeration cannot be made to pass this)")
	}
	if alpha == nil {
		t.Fatal("walkSkills did not discover skill \"alpha\"")
	}
	var sawNestedFile bool
	for _, f := range alpha.Files {
		if f.Path == "references/notes.md" {
			sawNestedFile = true
		}
	}
	if !sawNestedFile {
		t.Error("walkSkills did not discover the newly-added nested file inside an existing skill directory")
	}
}

// TestInventoryIncludesUnderscoreAndDotPrefixedContent asserts, against
// an in-memory filesystem, that a file whose name begins with an
// underscore and one whose name begins with a dot are both collected —
// the walk itself applies no filtering of its own; only the `all:` prefix
// on the //go:embed directive (embed.go) is what prevents such content
// from being silently excluded at compile time. It then asserts, against
// the REAL embedded FS, that the total collected file count is greater
// than zero and that a known deep path — the first discovered skill's own
// SKILL.md, located STRUCTURALLY rather than by a hardcoded skill name —
// is present with non-empty content.
func TestInventoryIncludesUnderscoreAndDotPrefixedContent(t *testing.T) {
	fsys := fstest.MapFS{
		"data/alpha/SKILL.md":          &fstest.MapFile{Data: []byte(fakeSkillMD("alpha"))},
		"data/alpha/_shared/helper.md": &fstest.MapFile{Data: []byte("shared helper content")},
		"data/alpha/.hidden.md":        &fstest.MapFile{Data: []byte("hidden content")},
	}
	discovered, err := walkSkills(fsys)
	if err != nil {
		t.Fatalf("walkSkills: %v", err)
	}

	var sawUnderscorePrefixed, sawDotPrefixed bool
	for _, s := range discovered {
		for _, f := range s.Files {
			if f.Path == "_shared/helper.md" {
				sawUnderscorePrefixed = true
			}
			if f.Path == ".hidden.md" {
				sawDotPrefixed = true
			}
		}
	}
	if !sawUnderscorePrefixed {
		t.Error("walkSkills did not collect an underscore-prefixed nested path")
	}
	if !sawDotPrefixed {
		t.Error("walkSkills did not collect a dot-prefixed nested path")
	}

	inv, err := Inventory()
	if err != nil {
		t.Fatalf("Inventory(): %v", err)
	}
	if len(inv) == 0 {
		t.Fatal("Inventory() returned zero skills")
	}
	totalFiles := 0
	for _, s := range inv {
		totalFiles += len(s.Files)
	}
	if totalFiles == 0 {
		t.Fatal("Inventory() discovered zero files across every skill")
	}

	first := inv[0]
	var skillMD *File
	for i := range first.Files {
		if first.Files[i].Path == "SKILL.md" {
			skillMD = &first.Files[i]
		}
	}
	if skillMD == nil {
		t.Fatalf("skill %q (structurally the first discovered skill) has no SKILL.md file", first.Name)
	}
	if len(skillMD.Content) == 0 {
		t.Errorf("skill %q's SKILL.md has empty content", first.Name)
	}
}

// TestInventoryIsDeterministic proves two consecutive Inventory() calls
// return identical skill order and identical per-skill file order.
func TestInventoryIsDeterministic(t *testing.T) {
	first, err := Inventory()
	if err != nil {
		t.Fatalf("Inventory() (first call): %v", err)
	}
	second, err := Inventory()
	if err != nil {
		t.Fatalf("Inventory() (second call): %v", err)
	}

	if len(first) != len(second) {
		t.Fatalf("Inventory() returned %d skills then %d skills, want the same count both times", len(first), len(second))
	}
	for i := range first {
		if first[i].Name != second[i].Name {
			t.Errorf("skill order differs at index %d: %q then %q", i, first[i].Name, second[i].Name)
		}
		if first[i].Summary != second[i].Summary {
			t.Errorf("skill %q summary differs across calls: %q then %q", first[i].Name, first[i].Summary, second[i].Summary)
		}
		if len(first[i].Files) != len(second[i].Files) {
			t.Fatalf("skill %q has %d files then %d files, want the same count both times", first[i].Name, len(first[i].Files), len(second[i].Files))
		}
		for j := range first[i].Files {
			if first[i].Files[j].Path != second[i].Files[j].Path {
				t.Errorf("skill %q file order differs at index %d: %q then %q", first[i].Name, j, first[i].Files[j].Path, second[i].Files[j].Path)
			}
		}
	}
}

// TestDigestIsStableAndTruncated asserts Digest returns exactly twelve
// lowercase hexadecimal characters, that it is stable across calls for
// the same skill, and that changing one byte of one file changes the
// digest.
func TestDigestIsStableAndTruncated(t *testing.T) {
	s := Skill{Name: "example", Files: []File{{Path: "SKILL.md", Content: []byte("hello world")}}}

	first := Digest(s)
	second := Digest(s)
	if first != second {
		t.Errorf("Digest(s) = %q then %q, want the same value both times", first, second)
	}
	if len(first) != 12 {
		t.Fatalf("Digest(s) = %q, len = %d, want exactly 12", first, len(first))
	}
	for _, r := range first {
		isLowerHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
		if !isLowerHex {
			t.Errorf("Digest(s) = %q contains non-lowercase-hex rune %q", first, r)
		}
	}

	changed := Skill{Name: "example", Files: []File{{Path: "SKILL.md", Content: []byte("hellp world")}}}
	if Digest(changed) == first {
		t.Error("changing one byte of one file's content did not change the digest")
	}
}
