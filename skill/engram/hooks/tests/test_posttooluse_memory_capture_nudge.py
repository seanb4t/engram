"""Tests for the posttooluse-memory-capture-nudge hook."""

from __future__ import annotations

import json
import subprocess
import sys
import tempfile
from pathlib import Path

import pytest

HOOK = Path(__file__).resolve().parent.parent / "posttooluse-memory-capture-nudge"


def run_hook(payload: dict) -> subprocess.CompletedProcess:
    return subprocess.run(
        [sys.executable, str(HOOK)],
        input=json.dumps(payload),
        capture_output=True,
        text=True,
        timeout=15,
    )


def git(*args, cwd):
    subprocess.run(["git", *args], cwd=cwd, check=True, capture_output=True, text=True)


def marker_for(session_id: str) -> Path:
    return Path(tempfile.gettempdir()) / f"memory-curator-capture-nudge-{session_id}"


@pytest.fixture()
def git_repo(tmp_path: Path) -> Path:
    repo = tmp_path / "repo"
    repo.mkdir()
    git("init", "-q", "-b", "main", cwd=repo)
    git("remote", "add", "origin", "git@github.com:org/repo.git", cwd=repo)
    git(
        "-c",
        "user.email=t@t",
        "-c",
        "user.name=t",
        "commit",
        "--allow-empty",
        "-m",
        "init",
        cwd=repo,
    )
    return repo


@pytest.fixture()
def session_id(request) -> str:
    sid = f"test-{request.node.name}"
    marker_for(sid).unlink(missing_ok=True)
    yield sid
    marker_for(sid).unlink(missing_ok=True)


def test_nudges_once_with_capture_context(git_repo: Path, session_id: str):
    result = run_hook({"cwd": str(git_repo), "session_id": session_id})
    assert result.returncode == 0
    ctx = json.loads(result.stdout)["hookSpecificOutput"]["additionalContext"]
    assert (
        json.loads(result.stdout)["hookSpecificOutput"]["hookEventName"]
        == "PostToolUse"
    )
    assert "curating-memory" in ctx
    assert "repo:github.com/org/repo" in ctx


def test_throttled_to_once_per_session(git_repo: Path, session_id: str):
    first = run_hook({"cwd": str(git_repo), "session_id": session_id})
    assert first.stdout.strip()  # first call emits
    second = run_hook({"cwd": str(git_repo), "session_id": session_id})
    assert second.returncode == 0
    assert second.stdout.strip() == ""  # marker present → silent


def test_overlay_scope_for_named_workspace(
    git_repo: Path, tmp_path: Path, session_id: str
):
    wt = tmp_path / "wt-feat"
    git("worktree", "add", "-q", str(wt), cwd=git_repo)
    ctx = json.loads(run_hook({"cwd": str(wt), "session_id": session_id}).stdout)[
        "hookSpecificOutput"
    ]["additionalContext"]
    assert ":ws:wt-feat" in ctx


def test_no_session_id_is_silent(git_repo: Path):
    result = run_hook({"cwd": str(git_repo)})
    assert result.returncode == 0
    assert result.stdout.strip() == ""


def test_non_repo_is_silent(tmp_path: Path, session_id: str):
    result = run_hook({"cwd": str(tmp_path), "session_id": session_id})
    assert result.returncode == 0
    assert result.stdout.strip() == ""


def test_malformed_stdin_is_silent():
    proc = subprocess.run(
        [sys.executable, str(HOOK)],
        input="not json",
        capture_output=True,
        text=True,
        timeout=15,
    )
    assert proc.returncode == 0
    assert proc.stdout.strip() == ""
