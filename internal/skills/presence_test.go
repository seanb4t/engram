// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeFileInfo is a minimal os.FileInfo implementation for
// fakeLstatEnv's Lstat closure — only Mode() is ever consulted by
// DetectPresence, so every other method returns an arbitrary,
// unused zero value.
type fakeFileInfo struct {
	name string
	mode os.FileMode
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeFileInfo) Sys() any           { return nil }

// fakeLstatEnv builds an in-memory Environment for TestDetectPresence:
// ReadFile serves store (missing → os.ErrNotExist, matching
// errors.Is(err, fs.ErrNotExist) exactly like OSEnvironment.ReadFile's
// contract), Lstat reports a fakeFileInfo for any path present in
// entries (missing → os.ErrNotExist), and WriteFile/MkdirAll are left
// nil — DetectPresence must never call them, so any accidental call
// panics this test immediately (the read-only prohibition, mechanically
// enforced rather than merely asserted).
func fakeLstatEnv(store map[string][]byte, entries map[string]os.FileMode) Environment {
	return Environment{
		ReadFile: func(name string) ([]byte, error) {
			b, ok := store[name]
			if !ok {
				return nil, os.ErrNotExist
			}
			return b, nil
		},
		Lstat: func(name string) (os.FileInfo, error) {
			mode, ok := entries[name]
			if !ok {
				return nil, os.ErrNotExist
			}
			return fakeFileInfo{name: filepath.Base(name), mode: mode}, nil
		},
	}
}

// TestDetectPresence exercises DetectPresence's read-only classification
// of a native (or AGENTS.md) destination: how many shipped skills already
// exist there, whether the destination or any skill entry is a symlink,
// and — for FormatAgentsMD only — whether an engram skills index block is
// already present in the operator's own index file.
func TestDetectPresence(t *testing.T) {
	list := testSkills()
	dir := "/home/fake/.agents/skills"
	index := "/home/fake/.codex/AGENTS.md"

	skillPath := func(name string) string { return filepath.Join(dir, name) }

	t.Run("none-present", func(t *testing.T) {
		env := fakeLstatEnv(nil, nil)
		got := DetectPresence(env, Target{Format: FormatNative, Dir: dir}, list)
		if got != (Presence{}) {
			t.Errorf("DetectPresence() = %+v, want zero Presence", got)
		}
	})

	t.Run("copies", func(t *testing.T) {
		entries := map[string]os.FileMode{dir: os.ModeDir}
		for _, s := range list {
			entries[skillPath(s.Name)] = os.ModeDir
		}
		env := fakeLstatEnv(nil, entries)
		got := DetectPresence(env, Target{Format: FormatNative, Dir: dir}, list)
		if got.Skills != len(list) {
			t.Errorf("Skills = %d, want %d", got.Skills, len(list))
		}
		if got.Symlink {
			t.Errorf("Symlink = true, want false (plain directories)")
		}
		if got.IndexBlock {
			t.Errorf("IndexBlock = true, want false (FormatNative never checks it)")
		}
	})

	t.Run("per-skill-symlinks", func(t *testing.T) {
		entries := map[string]os.FileMode{dir: os.ModeDir}
		for _, s := range list {
			entries[skillPath(s.Name)] = os.ModeSymlink
		}
		env := fakeLstatEnv(nil, entries)
		got := DetectPresence(env, Target{Format: FormatNative, Dir: dir}, list)
		if got.Skills != len(list) {
			t.Errorf("Skills = %d, want %d", got.Skills, len(list))
		}
		if !got.Symlink {
			t.Errorf("Symlink = false, want true (the maintainer's own hand-made per-skill symlinks)")
		}
	})

	t.Run("symlinked-dir", func(t *testing.T) {
		entries := map[string]os.FileMode{dir: os.ModeSymlink}
		for _, s := range list {
			entries[skillPath(s.Name)] = os.ModeDir
		}
		env := fakeLstatEnv(nil, entries)
		got := DetectPresence(env, Target{Format: FormatNative, Dir: dir}, list)
		if !got.Symlink {
			t.Errorf("Symlink = false, want true (target.Dir itself is a symlink)")
		}
	})

	t.Run("partial", func(t *testing.T) {
		entries := map[string]os.FileMode{
			dir:                     os.ModeDir,
			skillPath(list[0].Name): os.ModeDir,
		}
		env := fakeLstatEnv(nil, entries)
		got := DetectPresence(env, Target{Format: FormatNative, Dir: dir}, list)
		if got.Skills != 1 {
			t.Errorf("Skills = %d, want 1 (only one skill entry present)", got.Skills)
		}
	})

	t.Run("agents-md-block-present", func(t *testing.T) {
		store := map[string][]byte{
			index: []byte("# Mine\n" + BlockStartMarker + "\n...\n" + BlockEndMarker + "\n"),
		}
		env := fakeLstatEnv(store, nil)
		got := DetectPresence(env, Target{Format: FormatAgentsMD, Dir: dir, IndexFile: index}, list)
		if !got.IndexBlock {
			t.Errorf("IndexBlock = false, want true (well-formed block present)")
		}
	})

	t.Run("agents-md-block-malformed", func(t *testing.T) {
		store := map[string][]byte{
			index: []byte("# Mine\n" + BlockStartMarker + "\n...no end marker\n"),
		}
		env := fakeLstatEnv(store, nil)
		got := DetectPresence(env, Target{Format: FormatAgentsMD, Dir: dir, IndexFile: index}, list)
		if !got.IndexBlock {
			t.Errorf("IndexBlock = false, want true (malformed is still PRESENT, not absent)")
		}
	})

	t.Run("agents-md-no-block", func(t *testing.T) {
		store := map[string][]byte{index: []byte("# Mine\n")}
		env := fakeLstatEnv(store, nil)
		got := DetectPresence(env, Target{Format: FormatAgentsMD, Dir: dir, IndexFile: index}, list)
		if got.IndexBlock {
			t.Errorf("IndexBlock = true, want false (no marker anywhere)")
		}
	})

	t.Run("agents-md-missing-index", func(t *testing.T) {
		env := fakeLstatEnv(nil, nil)
		got := DetectPresence(env, Target{Format: FormatAgentsMD, Dir: dir, IndexFile: index}, list)
		if got.IndexBlock {
			t.Errorf("IndexBlock = true, want false (index file does not exist)")
		}
	})

	t.Run("native-ignores-index", func(t *testing.T) {
		store := map[string][]byte{
			index: []byte(BlockStartMarker + "\n" + BlockEndMarker + "\n"),
		}
		env := fakeLstatEnv(store, nil)
		got := DetectPresence(env, Target{Format: FormatNative, Dir: dir, IndexFile: index}, list)
		if got.IndexBlock {
			t.Errorf("IndexBlock = true, want false (FormatNative never reads IndexFile)")
		}
	})

	t.Run("format-none", func(t *testing.T) {
		env := Environment{
			ReadFile: func(string) ([]byte, error) {
				t.Errorf("ReadFile must not be called for FormatNone")
				return nil, os.ErrNotExist
			},
			Lstat: func(string) (os.FileInfo, error) {
				t.Errorf("Lstat must not be called for FormatNone")
				return nil, os.ErrNotExist
			},
		}
		got := DetectPresence(env, Target{Format: FormatNone}, list)
		if got != (Presence{}) {
			t.Errorf("DetectPresence() = %+v, want zero Presence", got)
		}
	})

	t.Run("lstat-permission-error-counts-absent", func(t *testing.T) {
		entries := map[string]os.FileMode{dir: os.ModeDir}
		for i, s := range list {
			if i == 0 {
				continue // deliberately absent from entries below
			}
			entries[skillPath(s.Name)] = os.ModeDir
		}
		env := Environment{
			ReadFile: fakeLstatEnv(nil, entries).ReadFile,
			Lstat: func(name string) (os.FileInfo, error) {
				if name == skillPath(list[0].Name) {
					return nil, os.ErrPermission
				}
				mode, ok := entries[name]
				if !ok {
					return nil, os.ErrNotExist
				}
				return fakeFileInfo{name: filepath.Base(name), mode: mode}, nil
			},
		}
		got := DetectPresence(env, Target{Format: FormatNative, Dir: dir}, list)
		if got.Skills != len(list)-1 {
			t.Errorf("Skills = %d, want %d (permission-denied skill not counted, no panic)", got.Skills, len(list)-1)
		}
	})

	t.Run("real-filesystem", func(t *testing.T) {
		if OSEnvironment.Lstat == nil {
			t.Fatal("OSEnvironment.Lstat is nil")
		}

		root := t.TempDir()
		realDir := filepath.Join(root, "real")
		for _, s := range list {
			if err := os.MkdirAll(filepath.Join(realDir, s.Name), 0o755); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
		}
		link := filepath.Join(root, "skills")
		if err := os.Symlink(realDir, link); err != nil {
			t.Fatalf("Symlink: %v", err)
		}

		got := DetectPresence(OSEnvironment, Target{Format: FormatNative, Dir: link}, list)
		if got.Skills != len(list) {
			t.Errorf("Skills = %d, want %d", got.Skills, len(list))
		}
		if !got.Symlink {
			t.Errorf("Symlink = false, want true (Dir itself is a symlink)")
		}

		got2 := DetectPresence(OSEnvironment, Target{Format: FormatNative, Dir: realDir}, list)
		if got2.Symlink {
			t.Errorf("Symlink = true, want false (Dir is a real directory)")
		}
	})
}
