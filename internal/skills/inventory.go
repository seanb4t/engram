// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

// dataRoot is the embedded FS's own root directory name — the ONE place
// that string is written literally in this package (embed.go's directive
// and this constant are the only two spellings of "data" anywhere here).
const dataRoot = "data"

// File is one file inside a skill's own directory: Path is
// slash-separated and RELATIVE TO THE SKILL'S OWN DIRECTORY (e.g.
// "SKILL.md", never "data/curating-memory/SKILL.md"), and Content is the
// file's verbatim bytes as embedded.
type File struct {
	Path    string
	Content []byte
}

// Skill is one discovered skill: Name is the vendored directory's own
// name, and Files is every regular file beneath it, sorted by Path.
type Skill struct {
	Name  string
	Files []File
}

// Inventory walks the embedded skill tree structurally: every immediate
// child directory of data is one skill, and every regular file beneath it
// (at any depth) belongs to that skill. Nothing here, or in this
// package's tests, names a count or a fixed list of skill names (D-04) —
// a sixth skill directory, or a new file inside an existing one, is
// discovered with zero code change here, and D-02's drift gate covers the
// vendor step that puts it there.
//
// An empty result is an ERROR, never an empty success: a walk matching
// nothing means the vendor step or the //go:embed directive is broken,
// and installing nothing while reporting success would be a silent
// failure exactly like the class D-04's own inventory-canary discussion
// warns against.
func Inventory() ([]Skill, error) {
	entries, err := fs.ReadDir(skillsFS, dataRoot)
	if err != nil {
		return nil, fmt.Errorf("skills: read embedded %q: %w", dataRoot, err)
	}

	var out []Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillName := entry.Name()
		skillRoot := dataRoot + "/" + skillName

		var files []File
		walkErr := fs.WalkDir(skillsFS, skillRoot, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			content, readErr := fs.ReadFile(skillsFS, p)
			if readErr != nil {
				return fmt.Errorf("read embedded %q: %w", p, readErr)
			}
			rel := strings.TrimPrefix(p, skillRoot+"/")
			files = append(files, File{Path: rel, Content: content})
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("skills: walk embedded skill %q: %w", skillName, walkErr)
		}

		sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
		out = append(out, Skill{Name: skillName, Files: files})
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("skills: embedded inventory under %q is empty — the vendor step (task skills:vendor) or the //go:embed directive is broken", dataRoot)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Digest returns the first 12 lowercase hexadecimal characters of a
// SHA-256 over s's own files, computed by writing, for each file in
// sorted Path order, the path bytes, a NUL byte, the content length in
// decimal, a NUL byte, the content, and a NUL byte — length-prefixed and
// NUL-separated so no path/content boundary is ambiguous (a file named
// "a" with content "b\x00c" cannot collide with a file named "a\x00b"
// with content "c").
//
// The 12-character truncation is deliberate: this digest is a display and
// cross-machine comparison aid rendered into an operator report row
// (D-03), never a security boundary — collision resistance at full
// SHA-256 width is not a property anything here depends on.
func Digest(s Skill) string {
	files := make([]File, len(s.Files))
	copy(files, s.Files)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	h := sha256.New()
	for _, f := range files {
		h.Write([]byte(f.Path))
		h.Write([]byte{0})
		h.Write([]byte(strconv.Itoa(len(f.Content))))
		h.Write([]byte{0})
		h.Write(f.Content)
		h.Write([]byte{0})
	}
	sum := hex.EncodeToString(h.Sum(nil))
	return sum[:12]
}

// TotalBytes sums the content length of every file in every skill.
func TotalBytes(skills []Skill) int {
	total := 0
	for _, s := range skills {
		for _, f := range s.Files {
			total += len(f.Content)
		}
	}
	return total
}

// ContentJSON returns a MINIFIED, single-line JSON object keyed by
// "<skill-name>/<file-path>", whose value is the file's verbatim content
// as a JSON STRING — never a nested object, array, or json.RawMessage
// value, so this can never bypass the operator renderer's scalar-only
// sanitizing branch (cmd/engram/operator_view.go's sanitizeViewValue),
// exactly as Plan.Config (internal/setup/plan.go) already does.
func ContentJSON(skills []Skill) (string, error) {
	doc := make(map[string]string)
	for _, s := range skills {
		for _, f := range s.Files {
			doc[s.Name+"/"+f.Path] = string(f.Content)
		}
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("skills: marshal content json: %w", err)
	}
	return string(b), nil
}
