// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// testGitRepo builds a git repository named myrepo, with one commit, under
// a fresh symlink-resolved temp root, with origin set to remote unless it
// is empty. It keeps the developer's git configuration and any repository
// above the root out of every git command the test runs, including the
// scope derivation's own probes.
func testGitRepo(t *testing.T, remote string) (root, repo string) {
	t.Helper()
	root = resolvedTempDir(t)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CEILING_DIRECTORIES", root)
	repo = filepath.Join(root, "myrepo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	testGit(t, repo, "init", "-q", "-b", "main")
	if remote != "" {
		testGit(t, repo, "remote", "add", "origin", remote)
	}
	testGit(t, repo, "commit", "-q", "--allow-empty", "-m", "init")
	return root, repo
}

// testGitWorktree adds a linked worktree of repo at root/wt/<name>.
func testGitWorktree(t *testing.T, root, repo, name string) string {
	t.Helper()
	wt := filepath.Join(root, "wt", name)
	testGit(t, repo, "worktree", "add", "-q", wt)
	return wt
}

func testGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
