// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import (
	"errors"
	"os"
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
