"""Tests for the session-start-memory-recall hook (subprocess + real git fixtures)."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

import pytest

HOOK = Path(__file__).resolve().parent.parent / "session-start-memory-recall"


def run_hook(cwd: str) -> subprocess.CompletedProcess:
    return subprocess.run(
        [sys.executable, str(HOOK)],
        input=json.dumps({"cwd": cwd}),
        capture_output=True,
        text=True,
        timeout=15,
    )


def git(*args, cwd):
    subprocess.run(["git", *args], cwd=cwd, check=True, capture_output=True, text=True)


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


def test_primary_checkout_spine_only(git_repo: Path):
    result = run_hook(str(git_repo))
    assert result.returncode == 0
    ctx = json.loads(result.stdout)["hookSpecificOutput"]["additionalContext"]
    assert "Memory spine scope: repo:github.com/org/repo" in ctx
    assert "Memory workspace scope:" not in ctx
    assert "mcp__memory_oauth__list_memory" in ctx
    assert "401" in ctx


def test_includes_capture_expectations(git_repo: Path):
    # The capture discipline lives here now (silent), replacing the Stop nudge.
    ctx = json.loads(run_hook(str(git_repo)).stdout)["hookSpecificOutput"][
        "additionalContext"
    ]
    assert "curating-memory" in ctx
    assert "no end-of-session prompt" in ctx
    assert "semantic" in ctx  # search_memory is vector-backed


def test_linked_worktree_has_overlay(git_repo: Path, tmp_path: Path):
    wt = tmp_path / "wt-feat"
    git("worktree", "add", "-q", str(wt), cwd=git_repo)
    result = run_hook(str(wt))
    ctx = json.loads(result.stdout)["hookSpecificOutput"]["additionalContext"]
    assert "Memory spine scope: repo:github.com/org/repo" in ctx
    assert "Memory workspace scope: repo:github.com/org/repo:ws:wt-feat" in ctx
    assert "mcp__memory_oauth__list_memory" in ctx
    assert "401" in ctx


def test_non_repo_is_silent(tmp_path: Path):
    result = run_hook(str(tmp_path))
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
