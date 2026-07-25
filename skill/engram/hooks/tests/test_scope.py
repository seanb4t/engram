# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Sean Brandt

"""Tests for the engram scope-derivation library (hooks/lib/scope.py)."""

from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))  # hooks/
from lib import scope


def make_fake_run(table):
    """table: list of (prefix_list, return_value). First prefix match wins."""
    calls = []

    def fake_run(args, cwd):
        calls.append(args)
        for prefix, value in table:
            if args[: len(prefix)] == prefix:
                return value
        return None

    fake_run.calls = calls
    return fake_run


def test_normalize_remote_scp_and_https():
    assert (
        scope._normalize_remote("git@github.com:org/repo.git") == "github.com/org/repo"
    )
    assert (
        scope._normalize_remote("https://github.com/org/repo.git")
        == "github.com/org/repo"
    )
    assert (
        scope._normalize_remote("ssh://git@github.com/org/repo")
        == "github.com/org/repo"
    )


def test_normalize_remote_explicit_port_dropped():
    # A numeric port after the host must be dropped, not treated as an scp path.
    assert (
        scope._normalize_remote("https://github.com:443/org/repo.git")
        == "github.com/org/repo"
    )


def test_git_repo_uses_origin(monkeypatch):
    fake = make_fake_run(
        [
            (["git", "remote", "get-url", "origin"], "https://github.com/org/repo.git"),
        ]
    )
    monkeypatch.setattr(scope, "_run", fake)
    assert scope._repo_id("/x") == "github.com/org/repo"


def test_repo_id_prefers_origin_over_common_dir(monkeypatch):
    # When origin resolves, the common-dir fallback must not be consulted.
    fake = make_fake_run(
        [
            (["git", "remote", "get-url", "origin"], "git@github.com:org/repo.git"),
            (["git", "rev-parse", "--git-common-dir"], "/wrong/.git"),
        ]
    )
    monkeypatch.setattr(scope, "_run", fake)
    assert scope._repo_id("/repo") == "github.com/org/repo"


def test_no_repo_returns_none(monkeypatch):
    fake = make_fake_run([])  # everything returns None
    monkeypatch.setattr(scope, "_run", fake)
    assert scope._repo_id("/nowhere") is None


def test_non_repo_returns_none_pair(monkeypatch):
    monkeypatch.setattr(scope, "_run", make_fake_run([]))
    assert scope.derive_scopes("/nowhere") == (None, None)


def test_remoteless_git_uses_common_dir_parent(monkeypatch):
    # No origin -> fall back to the parent of the shared .git.
    fake = make_fake_run(
        [
            (["git", "remote", "get-url", "origin"], None),
            (["git", "rev-parse", "--git-common-dir"], ".git"),
        ]
    )
    monkeypatch.setattr(scope, "_run", fake)
    assert scope._repo_id("/home/u/myrepo") == "myrepo"


def test_git_primary_worktree_no_overlay(monkeypatch):
    fake = make_fake_run(
        [
            (["git", "remote", "get-url", "origin"], "git@github.com:org/repo.git"),
            (["git", "rev-parse", "--git-common-dir"], "/repo/.git"),
            (["git", "rev-parse", "--show-toplevel"], "/repo"),
        ]
    )
    monkeypatch.setattr(scope, "_run", fake)
    assert scope.derive_scopes("/repo") == ("repo:github.com/org/repo", None)


def test_git_linked_worktree_overlay_basename(monkeypatch):
    fake = make_fake_run(
        [
            (["git", "remote", "get-url", "origin"], "git@github.com:org/repo.git"),
            (["git", "rev-parse", "--git-common-dir"], "/repo/.git"),
            (["git", "rev-parse", "--show-toplevel"], "/repo/_wt/feat"),
        ]
    )
    monkeypatch.setattr(scope, "_run", fake)
    spine, overlay = scope.derive_scopes("/repo/_wt/feat")
    assert spine == "repo:github.com/org/repo"
    assert overlay == "repo:github.com/org/repo:ws:feat"


def test_spine_identical_across_worktrees(monkeypatch):
    # Same repo, primary checkout vs a linked worktree -> identical spine,
    # None vs basename overlay. The spine must be stable across worktrees.
    def fake_for(toplevel):
        return make_fake_run(
            [
                (["git", "remote", "get-url", "origin"], "git@github.com:org/repo.git"),
                (["git", "rev-parse", "--git-common-dir"], "/repo/.git"),
                (["git", "rev-parse", "--show-toplevel"], toplevel),
            ]
        )

    monkeypatch.setattr(scope, "_run", fake_for("/repo"))
    spine_primary, ov_primary = scope.derive_scopes("/repo")
    monkeypatch.setattr(scope, "_run", fake_for("/repo/_wt/feat"))
    spine_feat, ov_feat = scope.derive_scopes("/repo/_wt/feat")
    assert spine_primary == spine_feat
    assert ov_primary is None
    assert ov_feat == "repo:github.com/org/repo:ws:feat"


def test_run_logs_abnormal_failure(capsys, tmp_path):
    # A missing binary (OSError) is an abnormal failure: return None AND log, so a
    # silent git timeout that drops the scope is diagnosable.
    assert scope._run(["engram-no-such-binary-xyz"], str(tmp_path)) is None
    assert "probe failed" in capsys.readouterr().err


def test_run_silent_on_clean_nonzero(capsys, tmp_path):
    # A real tool exiting nonzero (git outside a repo) is the expected "not a git
    # repo" negative — it must return None WITHOUT logging, or every session spams.
    assert scope._run(["git", "rev-parse", "--show-toplevel"], str(tmp_path)) is None
    assert capsys.readouterr().err == ""
