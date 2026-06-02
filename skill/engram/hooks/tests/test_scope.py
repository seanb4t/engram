"""Tests for memory-curator scope derivation."""

from __future__ import annotations

import sys
from pathlib import Path


sys.path.insert(0, str(Path(__file__).resolve().parent.parent))  # hooks/
from lib import scope  # noqa: E402


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


def test_origin_from_jj_picks_origin_when_not_first(monkeypatch):
    """origin must be found even when it is not the first remote listed."""
    fake = make_fake_run(
        [
            (["jj", "--no-pager", "root"], "/repo"),
            (
                ["jj", "--no-pager", "git", "remote", "list"],
                "upstream git@github.com:upstream/repo.git\norigin git@github.com:org/repo.git",
            ),
        ]
    )
    monkeypatch.setattr(scope, "_run", fake)
    assert scope._repo_id("/repo") == "github.com/org/repo"


def test_jj_repo_uses_jj_remote_never_git(monkeypatch):
    fake = make_fake_run(
        [
            (["jj", "--no-pager", "root"], "/repo"),
            (
                ["jj", "--no-pager", "git", "remote", "list"],
                "origin git@github.com:org/repo.git",
            ),
        ]
    )
    monkeypatch.setattr(scope, "_run", fake)
    assert scope._repo_id("/repo") == "github.com/org/repo"
    assert not any(a and a[0] == "git" for a in fake.calls)


def test_git_repo_uses_origin(monkeypatch):
    fake = make_fake_run(
        [
            (["jj", "--no-pager", "root"], None),
            (["git", "remote", "get-url", "origin"], "https://github.com/org/repo.git"),
        ]
    )
    monkeypatch.setattr(scope, "_run", fake)
    assert scope._repo_id("/x") == "github.com/org/repo"


def test_remoteless_jj_default_workspace_uses_basename(monkeypatch, tmp_path):
    (tmp_path / ".jj" / "repo").mkdir(parents=True)  # default ws: repo is a dir
    fake = make_fake_run(
        [
            (["jj", "--no-pager", "root"], str(tmp_path)),
            (["jj", "--no-pager", "git", "remote", "list"], ""),
        ]
    )
    monkeypatch.setattr(scope, "_run", fake)
    assert scope._repo_id(str(tmp_path)) == tmp_path.name


def test_remoteless_jj_secondary_workspace_resolves_pointer(monkeypatch, tmp_path):
    primary = tmp_path / "myrepo"
    (primary / ".jj").mkdir(parents=True)
    ws = tmp_path / "myrepo_worktrees" / "feat"
    (ws / ".jj").mkdir(parents=True)
    (ws / ".jj" / "repo").write_text("../../../myrepo/.jj/repo")  # pointer file
    fake = make_fake_run(
        [
            (["jj", "--no-pager", "root"], str(ws)),
            (["jj", "--no-pager", "git", "remote", "list"], ""),
        ]
    )
    monkeypatch.setattr(scope, "_run", fake)
    assert scope._repo_id(str(ws)) == "myrepo"


def test_no_repo_returns_none(monkeypatch):
    fake = make_fake_run([])  # everything returns None
    monkeypatch.setattr(scope, "_run", fake)
    assert scope._repo_id("/nowhere") is None


def test_jj_named_workspace_overlay(monkeypatch):
    fake = make_fake_run(
        [
            (["jj", "--no-pager", "root"], "/repo/_wt/feat"),
            (
                ["jj", "--no-pager", "git", "remote", "list"],
                "origin git@github.com:org/repo.git",
            ),
            (["jj", "--no-pager", "log", "-r", "@"], "worktree-feat@"),
        ]
    )
    monkeypatch.setattr(scope, "_run", fake)
    spine, overlay = scope.derive_scopes("/repo/_wt/feat")
    assert spine == "repo:github.com/org/repo"
    assert overlay == "repo:github.com/org/repo:ws:worktree-feat"


def test_jj_default_workspace_spine_only(monkeypatch):
    fake = make_fake_run(
        [
            (["jj", "--no-pager", "root"], "/repo"),
            (
                ["jj", "--no-pager", "git", "remote", "list"],
                "origin git@github.com:org/repo.git",
            ),
            (["jj", "--no-pager", "log", "-r", "@"], "default@"),
        ]
    )
    monkeypatch.setattr(scope, "_run", fake)
    spine, overlay = scope.derive_scopes("/repo")
    assert spine == "repo:github.com/org/repo"
    assert overlay is None


def test_spine_identical_across_workspaces(monkeypatch):
    # Same repo, two different workspaces -> same spine, different/None overlay.
    def fake_for(ws_name):
        return make_fake_run(
            [
                (["jj", "--no-pager", "root"], "/repo"),
                (
                    ["jj", "--no-pager", "git", "remote", "list"],
                    "origin git@github.com:org/repo.git",
                ),
                (["jj", "--no-pager", "log", "-r", "@"], ws_name),
            ]
        )

    monkeypatch.setattr(scope, "_run", fake_for("default@"))
    spine_default, ov_default = scope.derive_scopes("/repo")
    monkeypatch.setattr(scope, "_run", fake_for("worktree-feat@"))
    spine_feat, ov_feat = scope.derive_scopes("/repo")
    assert spine_default == spine_feat
    assert ov_default is None
    assert ov_feat == "repo:github.com/org/repo:ws:worktree-feat"


def test_non_repo_returns_none_pair(monkeypatch):
    monkeypatch.setattr(scope, "_run", make_fake_run([]))
    assert scope.derive_scopes("/nowhere") == (None, None)


def test_remoteless_git_uses_common_dir_parent(monkeypatch):
    # jj absent, no origin -> fall back to parent of the shared .git.
    fake = make_fake_run(
        [
            (["jj", "--no-pager", "root"], None),
            (["git", "remote", "get-url", "origin"], None),
            (["git", "rev-parse", "--git-common-dir"], ".git"),
        ]
    )
    monkeypatch.setattr(scope, "_run", fake)
    assert scope._repo_id("/home/u/myrepo") == "myrepo"


def test_git_primary_worktree_no_overlay(monkeypatch):
    fake = make_fake_run(
        [
            (["jj", "--no-pager", "root"], None),
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
            (["jj", "--no-pager", "root"], None),
            (["git", "remote", "get-url", "origin"], "git@github.com:org/repo.git"),
            (["git", "rev-parse", "--git-common-dir"], "/repo/.git"),
            (["git", "rev-parse", "--show-toplevel"], "/repo/_wt/feat"),
        ]
    )
    monkeypatch.setattr(scope, "_run", fake)
    spine, overlay = scope.derive_scopes("/repo/_wt/feat")
    assert spine == "repo:github.com/org/repo"
    assert overlay == "repo:github.com/org/repo:ws:feat"
