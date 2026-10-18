// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package scope derives the spine and workspace-overlay memory scopes for a
// working directory from local git alone: no network call and no `gh`.
//
// It is the Go port of the plugin hook's skill/engram/hooks/lib/scope.py
// and must derive the same values. testdata/cases.json is the case table
// both test suites read.
package scope

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
)

// Scopes is the derivation result for one directory. The zero value means
// no scope: the directory is outside a git repository, or git could not
// run.
type Scopes struct {
	// RepoID is the repository identity, e.g. "github.com/org/repo", or
	// the main repository root's directory name when there is no origin
	// remote.
	RepoID string
	// FromOrigin is true when RepoID came from the origin remote, and
	// false when it fell back to the directory name.
	FromOrigin bool
	// Workspace is the linked worktree's top-level directory name, and
	// empty in the primary checkout.
	Workspace string
}

// Spine returns "repo:<RepoID>", or "" when no scope was derived.
func (s Scopes) Spine() string {
	if s.RepoID == "" {
		return ""
	}
	return "repo:" + s.RepoID
}

// Overlay returns "<spine>:ws:<Workspace>" in a linked worktree, and ""
// otherwise.
func (s Scopes) Overlay() string {
	if s.RepoID == "" || s.Workspace == "" {
		return ""
	}
	return s.Spine() + ":ws:" + s.Workspace
}

// remoteSchemes are the URL schemes NormalizeRemote strips, as scope.py does.
var remoteSchemes = []string{"https://", "http://", "ssh://", "git://"}

// NormalizeRemote reduces a git remote URL to its host/path identity: it
// removes the scheme, user info, a numeric port, a trailing ".git" and
// surrounding slashes, and replaces the colon of an scp-style host:path
// with "/". A path segment of digits after an scp-style colon
// (git@host:1234/repo.git) is read as a port, as scope.py reads it.
func NormalizeRemote(url string) string {
	u := strings.TrimSpace(url)
	for _, scheme := range remoteSchemes {
		if rest, ok := strings.CutPrefix(u, scheme); ok {
			u = rest
			break
		}
	}
	head, _, _ := strings.Cut(u, "/")
	if strings.Contains(head, "@") {
		_, u, _ = strings.Cut(u, "@")
		head, _, _ = strings.Cut(u, "/")
	}
	if host, after, ok := strings.Cut(head, ":"); ok {
		if isDigits(after) {
			u = host + u[len(head):]
		} else {
			u = strings.Replace(u, ":", "/", 1)
		}
	}
	u = strings.TrimSuffix(u, ".git")
	return strings.Trim(u, "/")
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Derive returns the scopes for dir. A git failure, or a dir outside any
// repository, yields the zero Scopes and a nil error. The only error is
// ctx's, when ctx ends before the git probes finish.
func Derive(ctx context.Context, dir string) (Scopes, error) {
	origin, err := probe(ctx, dir, "remote", "get-url", "origin")
	if err != nil {
		return Scopes{}, err
	}
	common, err := probe(ctx, dir, "rev-parse", "--git-common-dir")
	if err != nil {
		return Scopes{}, err
	}
	var s Scopes
	switch {
	case origin != "":
		s.RepoID, s.FromOrigin = NormalizeRemote(origin), true
	case common != "":
		// The parent of the shared .git is the main repository root, the
		// same from every subdirectory and linked worktree.
		s.RepoID = filepath.Base(filepath.Dir(commonPath(dir, common)))
	default:
		return Scopes{}, nil
	}
	toplevel, err := probe(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return Scopes{}, err
	}
	if common != "" && toplevel != "" && realpath(filepath.Dir(commonPath(dir, common))) != realpath(toplevel) {
		s.Workspace = filepath.Base(toplevel)
	}
	return s, nil
}

// probe runs one git command in dir and returns its trimmed stdout, or ""
// when git fails or prints nothing.
func probe(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", ctxErr
	}
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

// commonPath returns the shared .git path that --git-common-dir printed.
// git prints it relative to its physical working directory (../../.git
// from a subdirectory), so a relative path joins the resolved dir, not the
// lexical one. An absolute path is used as printed, as scope.py uses it.
func commonPath(dir, common string) string {
	if filepath.IsAbs(common) {
		return filepath.Clean(common)
	}
	return realpath(filepath.Join(realpath(dir), common))
}

// realpath resolves p's symlinks, or returns p cleaned and absolute when it
// cannot.
func realpath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return filepath.Clean(p)
}
