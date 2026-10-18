// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package scope

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// cases mirrors testdata/cases.json, which skill/engram/hooks/tests/test_scope.py
// also reads.
type cases struct {
	Remotes []struct {
		Name     string `json:"name"`
		Remote   string `json:"remote"`
		Identity string `json:"identity"`
	} `json:"remotes"`
	Layouts []struct {
		Name    string  `json:"name"`
		Kind    string  `json:"kind"`
		Remote  *string `json:"remote"`
		Spine   *string `json:"spine"`
		Overlay *string `json:"overlay"`
	} `json:"layouts"`
}

func loadCases(t *testing.T) cases {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c cases
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	if len(c.Remotes) == 0 || len(c.Layouts) == 0 {
		t.Fatal("testdata/cases.json has an empty remotes or layouts list")
	}
	return c
}

func TestNormalizeRemote(t *testing.T) {
	for _, c := range loadCases(t).Remotes {
		t.Run(c.Name, func(t *testing.T) {
			if got := NormalizeRemote(c.Remote); got != c.Identity {
				t.Errorf("NormalizeRemote(%q) = %q, want %q", c.Remote, got, c.Identity)
			}
		})
	}
}

// hermeticGit keeps the developer's git configuration and any repository
// above the temporary root out of every git command the test runs,
// including Derive's own probes.
func hermeticGit(t *testing.T, root string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	ceiling, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", ceiling)
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// buildLayout builds one layout kind under root, following cases.json's
// layout_conventions, and returns the directory to derive from.
func buildLayout(t *testing.T, root, kind string, remote *string) string {
	t.Helper()
	if kind == "not_a_repository" {
		dir := filepath.Join(root, "scratch")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	repo := filepath.Join(root, "myrepo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "init", "-q", "-b", "main")
	if remote != nil {
		git(t, repo, "remote", "add", "origin", *remote)
	}
	git(t, repo, "commit", "-q", "--allow-empty", "-m", "init")
	switch kind {
	case "primary_checkout":
		return repo
	case "subdirectory":
		dir := filepath.Join(repo, "cmd", "tool")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		return dir
	case "linked_worktree":
		wt := filepath.Join(root, "wt", "feat")
		git(t, repo, "worktree", "add", "-q", wt)
		return wt
	}
	t.Fatalf("unknown layout kind %q", kind)
	return ""
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func TestDeriveLayouts(t *testing.T) {
	for _, c := range loadCases(t).Layouts {
		t.Run(c.Name, func(t *testing.T) {
			root := t.TempDir()
			hermeticGit(t, root)
			dir := buildLayout(t, root, c.Kind, c.Remote)
			got, err := Derive(context.Background(), dir)
			if err != nil {
				t.Fatalf("Derive: %v", err)
			}
			if got.Spine() != deref(c.Spine) || got.Overlay() != deref(c.Overlay) {
				t.Errorf("Derive(%s) = spine %q overlay %q, want spine %q overlay %q",
					c.Kind, got.Spine(), got.Overlay(), deref(c.Spine), deref(c.Overlay))
			}
			if wantOrigin := c.Remote != nil && c.Spine != nil; got.FromOrigin != wantOrigin {
				t.Errorf("FromOrigin = %v, want %v", got.FromOrigin, wantOrigin)
			}
		})
	}
}

func TestDeriveWithoutGit(t *testing.T) {
	root := t.TempDir()
	hermeticGit(t, root)
	remote := "git@github.com:org/repo.git"
	dir := buildLayout(t, root, "primary_checkout", &remote)
	t.Setenv("PATH", t.TempDir())
	got, err := Derive(context.Background(), dir)
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	if got != (Scopes{}) {
		t.Errorf("Derive with no git on PATH = %+v, want no scope", got)
	}
}

func TestDeriveReturnsContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Derive(ctx, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Errorf("Derive with a cancelled context: err = %v, want context.Canceled", err)
	}
}
