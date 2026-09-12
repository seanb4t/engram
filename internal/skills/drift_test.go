// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// walkEmbeddedPaths collects every regular file's path in the embedded
// FS, relative to dataRoot ("data"), using forward slashes.
func walkEmbeddedPaths(t *testing.T) map[string][]byte {
	t.Helper()
	out := make(map[string][]byte)
	err := fs.WalkDir(skillsFS, dataRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := p[len(dataRoot)+1:]
		content, readErr := fs.ReadFile(skillsFS, p)
		if readErr != nil {
			return readErr
		}
		out[rel] = content
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded FS: %v", err)
	}
	return out
}

// walkVendorSourcePaths collects every regular file's path under
// ../../skill/engram/skills (relative to that root, forward-slashed) on
// disk — the CANONICAL tree the vendored copy under internal/skills/data
// must match byte-for-byte (D-01, D-02).
func walkVendorSourcePaths(t *testing.T) map[string][]byte {
	t.Helper()
	root := filepath.Join("..", "..", "skill", "engram", "skills")
	out := make(map[string][]byte)
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		content, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		out[filepath.ToSlash(rel)] = content
		return nil
	})
	if err != nil {
		t.Fatalf("walk on-disk vendor source %s: %v", root, err)
	}
	return out
}

// TestSkillsEmbedMatchesVendored proves the vendored copy embedded into
// this binary (internal/skills/data, via //go:embed all:data) is
// byte-identical to skill/engram/skills — the canonical, plugin-authored
// tree (D-01) — asserting SET EQUALITY over discovered paths FIRST (never
// containment, which passes vacuously on a superset — memories
// bqhy5v5hq9/8583e0yqa1), then BYTE EQUALITY for every shared path. This
// is D-02's anti-drift gate: it lives in `go test ./...`, so it fires on
// `task`, on a bare local run, and on every feature-branch push — not
// only at PR time — and it catches an edit made directly to the vendored
// copy, which a one-directional regenerate-then-diff would not.
func TestSkillsEmbedMatchesVendored(t *testing.T) {
	embedded := walkEmbeddedPaths(t)
	vendored := walkVendorSourcePaths(t)

	if len(embedded) == 0 {
		t.Fatal("embedded FS under data/ has zero files — a scan matching nothing is vacuously green, which means the //go:embed directive or the vendor step is broken, not that the trees match")
	}
	if len(vendored) == 0 {
		t.Fatal("on-disk skill/engram/skills has zero files — a scan matching nothing is vacuously green, which means the canonical tree moved or this test's path is wrong, not that the trees match")
	}

	var embeddedOnly, vendoredOnly []string
	for p := range embedded {
		if _, ok := vendored[p]; !ok {
			embeddedOnly = append(embeddedOnly, p)
		}
	}
	for p := range vendored {
		if _, ok := embedded[p]; !ok {
			vendoredOnly = append(vendoredOnly, p)
		}
	}
	sort.Strings(embeddedOnly)
	sort.Strings(vendoredOnly)

	if len(embeddedOnly) > 0 || len(vendoredOnly) > 0 {
		t.Fatalf("internal/skills/data has drifted from skill/engram/skills — run `task skills:vendor`.\n"+
			"present in internal/skills/data only: %v\n"+
			"present in skill/engram/skills only: %v", embeddedOnly, vendoredOnly)
	}

	var differing []string
	for p, embeddedContent := range embedded {
		vendoredContent := vendored[p]
		if string(embeddedContent) != string(vendoredContent) {
			differing = append(differing, p)
		}
	}
	sort.Strings(differing)
	if len(differing) > 0 {
		t.Fatalf("internal/skills/data has drifted from skill/engram/skills — run `task skills:vendor`.\n"+
			"byte-differing path(s): %v", differing)
	}
}
