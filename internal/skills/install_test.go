// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeInstallEnv builds an in-memory Environment backed by store, so no
// test in this file ever touches a real home directory (repo rule
// m45p2b4bp7). MkdirAll is a no-op — the in-memory map has no directory
// concept to create.
func fakeInstallEnv(store map[string][]byte) Environment {
	return Environment{
		ReadFile: func(name string) ([]byte, error) {
			b, ok := store[name]
			if !ok {
				return nil, os.ErrNotExist
			}
			return b, nil
		},
		WriteFile: func(name string, data []byte, _ os.FileMode) error {
			cp := make([]byte, len(data))
			copy(cp, data)
			store[name] = cp
			return nil
		},
		MkdirAll: func(string, os.FileMode) error { return nil },
	}
}

// testSkills is a small, fixed two-skill inventory this file's tests
// drive Install against directly — never the real embedded Inventory(),
// so these tests exercise Install's own logic independent of whatever the
// vendored tree happens to contain.
func testSkills() []Skill {
	return []Skill{
		{Name: "alpha", Files: []File{{Path: "SKILL.md", Content: []byte("alpha content")}}},
		{Name: "beta", Files: []File{
			{Path: "SKILL.md", Content: []byte("beta content")},
			{Path: "references/notes.md", Content: []byte("beta notes")},
		}},
	}
}

// TestInstallNativeConverges proves D-08's byte-compare convergence: a
// first install writes every discovered file, and a second install
// against the SAME destination map reports every file AlreadyCorrect with
// zero writes.
func TestInstallNativeConverges(t *testing.T) {
	store := make(map[string][]byte)
	env := fakeInstallEnv(store)
	target := Target{Format: FormatNative, Dir: "/home/fake/.claude/skills"}
	skillsList := testSkills()

	first := Install(env, target, skillsList)
	if first.Err != nil {
		t.Fatalf("first Install: unexpected error: %v", first.Err)
	}
	if len(first.Wrote) != 3 {
		t.Fatalf("first Install: len(Wrote) = %d, want 3 (one per file across both skills)", len(first.Wrote))
	}
	if len(first.AlreadyCorrect) != 0 {
		t.Fatalf("first Install: len(AlreadyCorrect) = %d, want 0", len(first.AlreadyCorrect))
	}

	t.Run("second_install_converges", func(t *testing.T) {
		second := Install(env, target, skillsList)
		if second.Err != nil {
			t.Fatalf("second Install: unexpected error: %v", second.Err)
		}
		if len(second.Wrote) != 0 {
			t.Fatalf("second Install: len(Wrote) = %d, want 0 (nothing changed)", len(second.Wrote))
		}
		if len(second.AlreadyCorrect) != 3 {
			t.Fatalf("second Install: len(AlreadyCorrect) = %d, want 3", len(second.AlreadyCorrect))
		}
	})
}

// TestInstallOverwritesDifferingDestination proves a destination whose
// existing bytes differ from the embedded content is overwritten
// unconditionally and reported on Wrote (D-08) — never refused, never
// left alone.
func TestInstallOverwritesDifferingDestination(t *testing.T) {
	store := make(map[string][]byte)
	env := fakeInstallEnv(store)
	target := Target{Format: FormatNative, Dir: "/home/fake/.claude/skills"}
	skillsList := testSkills()

	if r := Install(env, target, skillsList); r.Err != nil {
		t.Fatalf("seed Install: unexpected error: %v", r.Err)
	}

	dest := target.Dir + "/alpha/SKILL.md"
	store[dest] = []byte("hand-edited, stale content")

	second := Install(env, target, skillsList)
	if second.Err != nil {
		t.Fatalf("second Install: unexpected error: %v", second.Err)
	}
	var sawWrote bool
	for _, p := range second.Wrote {
		if p == dest {
			sawWrote = true
		}
	}
	if !sawWrote {
		t.Errorf("second Install: Wrote = %v, want it to contain %q (differing destination must be overwritten)", second.Wrote, dest)
	}
	if got := string(store[dest]); got != "alpha content" {
		t.Errorf("store[%q] = %q, want the embedded content to have overwritten the stale bytes", dest, got)
	}
}

// TestInstallAccumulatesFailuresAndAttemptsEverySkill proves D-07: a
// write failure on one skill's file does not prevent a later skill (or a
// later file within the SAME skill) from being attempted, and every
// failure's message survives in the joined error.
func TestInstallAccumulatesFailuresAndAttemptsEverySkill(t *testing.T) {
	store := make(map[string][]byte)
	env := fakeInstallEnv(store)
	env.WriteFile = func(name string, data []byte, _ os.FileMode) error {
		if strings.Contains(name, "alpha") {
			return errors.New("boom: disk full")
		}
		store[name] = append([]byte(nil), data...)
		return nil
	}
	target := Target{Format: FormatNative, Dir: "/home/fake/.claude/skills"}

	list := []Skill{
		{Name: "alpha", Files: []File{
			{Path: "SKILL.md", Content: []byte("alpha 1")},
			{Path: "references/x.md", Content: []byte("alpha 2")},
		}},
		{Name: "beta", Files: []File{{Path: "SKILL.md", Content: []byte("beta content")}}},
	}

	report := Install(env, target, list)
	if report.Err == nil {
		t.Fatal("Install: Err = nil, want a non-nil joined error naming both alpha failures")
	}
	if got := len(report.Wrote); got != 1 {
		t.Fatalf("Install: len(Wrote) = %d, want 1 (beta's file still attempted and succeeded)", got)
	}
	if report.Wrote[0] != target.Dir+"/beta/SKILL.md" {
		t.Errorf("Install: Wrote = %v, want it to contain beta's destination", report.Wrote)
	}
	msg := report.Err.Error()
	if strings.Count(msg, "boom: disk full") != 2 {
		t.Errorf("Install: Err = %q, want it to contain both of alpha's failures (one per file)", msg)
	}
}

// TestInstallAgentsMdConverges drives Install's FormatAgentsMD branch
// across every non-symlink bullet in 04-02-PLAN.md Task 3's behavior
// block using the in-memory fake seam: create, replace, already-correct
// on a second run, and the malformed case.
func TestInstallAgentsMdConverges(t *testing.T) {
	const indexFile = "/home/fake/AGENTS.md"
	target := Target{Format: FormatAgentsMD, Dir: "/home/fake/.claude/skills", IndexFile: indexFile}

	t.Run("create: index file does not exist yet", func(t *testing.T) {
		store := make(map[string][]byte)
		env := fakeInstallEnv(store)
		list := testSkills()

		report := Install(env, target, list)
		if report.Err != nil {
			t.Fatalf("Install: unexpected error: %v", report.Err)
		}
		if len(report.Wrote) != 4 { // 3 skill files + the index file
			t.Fatalf("len(Wrote) = %d, want 4 (3 skill files + index)", len(report.Wrote))
		}
		var sawIndex bool
		for _, p := range report.Wrote {
			if p == indexFile {
				sawIndex = true
			}
		}
		if !sawIndex {
			t.Fatalf("Wrote = %v, want it to contain the index file %q", report.Wrote, indexFile)
		}
		body := string(store[indexFile])
		if !strings.Contains(body, BlockStartMarker) || !strings.Contains(body, BlockEndMarker) {
			t.Errorf("index file content = %q, want it to contain both markers", body)
		}
	})

	t.Run("replace: a well-formed pre-existing block is replaced in place", func(t *testing.T) {
		store := make(map[string][]byte)
		env := fakeInstallEnv(store)
		list := testSkills()

		if r := Install(env, target, list); r.Err != nil {
			t.Fatalf("seed Install: unexpected error: %v", r.Err)
		}
		// Hand-alter the pre-existing block's interior to prove a re-run
		// replaces rather than appends a second copy.
		store[indexFile] = []byte(strings.Replace(string(store[indexFile]), "alpha", "ALPHA-STALE", 1))

		second := Install(env, target, list)
		if second.Err != nil {
			t.Fatalf("second Install: unexpected error: %v", second.Err)
		}
		var sawIndexWrote bool
		for _, p := range second.Wrote {
			if p == indexFile {
				sawIndexWrote = true
			}
		}
		if !sawIndexWrote {
			t.Errorf("second Install: Wrote = %v, want it to contain the index file", second.Wrote)
		}
		if strings.Contains(string(store[indexFile]), "ALPHA-STALE") {
			t.Error("second Install did not replace the stale interior")
		}
		if strings.Count(string(store[indexFile]), BlockStartMarker) != 1 {
			t.Errorf("index file has %d start markers, want exactly 1 (replaced, not appended)", strings.Count(string(store[indexFile]), BlockStartMarker))
		}
	})

	t.Run("already-correct: a second run with nothing changed writes nothing to the index", func(t *testing.T) {
		store := make(map[string][]byte)
		env := fakeInstallEnv(store)
		list := testSkills()

		if r := Install(env, target, list); r.Err != nil {
			t.Fatalf("seed Install: unexpected error: %v", r.Err)
		}

		second := Install(env, target, list)
		if second.Err != nil {
			t.Fatalf("second Install: unexpected error: %v", second.Err)
		}
		for _, p := range second.Wrote {
			if p == indexFile {
				t.Errorf("second Install: Wrote = %v, want the index file NOT on Wrote", second.Wrote)
			}
		}
		var sawIndexAlreadyCorrect bool
		for _, p := range second.AlreadyCorrect {
			if p == indexFile {
				sawIndexAlreadyCorrect = true
			}
		}
		if !sawIndexAlreadyCorrect {
			t.Errorf("second Install: AlreadyCorrect = %v, want it to contain the index file", second.AlreadyCorrect)
		}
	})

	t.Run("malformed: the index write is skipped but every skill file still installs", func(t *testing.T) {
		store := make(map[string][]byte)
		env := fakeInstallEnv(store)
		list := testSkills()

		malformed := []byte(BlockEndMarker + "\nno matching start\n")
		store[indexFile] = append([]byte(nil), malformed...)

		report := Install(env, target, list)
		if report.Err == nil {
			t.Fatal("Install: Err = nil, want a non-nil error naming the malformed index")
		}
		if !strings.Contains(report.Err.Error(), indexFile) {
			t.Errorf("Install: Err = %q, want it to name the index path %q", report.Err.Error(), indexFile)
		}
		if !bytes.Equal(store[indexFile], malformed) {
			t.Errorf("store[%q] = %q, want it UNCHANGED — zero bytes written to a malformed index", indexFile, store[indexFile])
		}
		for _, p := range report.Wrote {
			if p == indexFile {
				t.Error("Wrote contains the index file, want zero writes to it on a malformed input")
			}
		}
		if len(report.Wrote) != 3 {
			t.Errorf("len(Wrote) = %d, want 3 (every skill file still installs)", len(report.Wrote))
		}
	})
}

// TestAgentsMdPreservesSymlink is the one test in this package that uses
// a REAL filesystem: the property under test — what the operating system
// does to a symlink under a whole-file write — cannot be exhibited by a
// fake seam. This is engram's OWN write behavior against a real
// filesystem, not third-party behavior, so it is a legitimate automated
// gate under repo rule m45p2b4bp7.
func TestAgentsMdPreservesSymlink(t *testing.T) {
	dir := t.TempDir()
	realFile := filepath.Join(dir, "real-agents.md")
	symlinkFile := filepath.Join(dir, "AGENTS.md")

	if err := os.WriteFile(realFile, []byte("pre-existing dotfile content\n"), 0o644); err != nil {
		t.Fatalf("seed real file: %v", err)
	}
	if err := os.Symlink(realFile, symlinkFile); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	target := Target{
		Format:    FormatAgentsMD,
		Dir:       filepath.Join(dir, "skills"),
		IndexFile: symlinkFile,
	}

	report := Install(OSEnvironment, target, testSkills())
	if report.Err != nil {
		t.Fatalf("Install: unexpected error: %v", report.Err)
	}

	info, err := os.Lstat(symlinkFile)
	if err != nil {
		t.Fatalf("Lstat symlink path: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is no longer a symlink after Install", symlinkFile)
	}

	linkTarget, err := os.Readlink(symlinkFile)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if linkTarget != realFile {
		t.Errorf("Readlink(%s) = %q, want %q — the link's target changed identity", symlinkFile, linkTarget, realFile)
	}

	content, err := os.ReadFile(realFile)
	if err != nil {
		t.Fatalf("read real target file: %v", err)
	}
	if !strings.Contains(string(content), BlockStartMarker) {
		t.Errorf("real target file content = %q, want it to contain the skills index block", content)
	}
	if !strings.Contains(string(content), "pre-existing dotfile content") {
		t.Errorf("real target file content = %q, want the pre-existing content preserved", content)
	}
}
