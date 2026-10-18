# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Sean Brandt

"""Tests for the session-start-memory-recall hook (subprocess + real git fixtures)."""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import time
from pathlib import Path

import pytest

HOOK = Path(__file__).resolve().parent.parent / "session-start-memory-recall"
# Resolved before the bin_dir fixture narrows PATH.
SLEEP = shutil.which("sleep")


def run_hook(cwd: str) -> subprocess.CompletedProcess:
    return subprocess.run(
        [sys.executable, str(HOOK)],
        input=json.dumps({"cwd": cwd}),
        capture_output=True,
        text=True,
        timeout=15,
        check=False,
    )


@pytest.fixture(autouse=True)
def bin_dir(tmp_path: Path, monkeypatch) -> Path:
    """PATH holds only git, plus any stub engram a test adds, so the developer's
    own engram binary never runs; git reads no global or system config."""
    d = tmp_path / "bin"
    d.mkdir()
    (d / "git").symlink_to(shutil.which("git"))
    monkeypatch.setenv("PATH", str(d))
    monkeypatch.setenv("GIT_CONFIG_GLOBAL", os.devnull)
    monkeypatch.setenv("GIT_CONFIG_NOSYSTEM", "1")
    return d


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
        # commit.gpgsign=false keeps the fixture hermetic: a developer's global
        # signing config (e.g. 1Password's gpg.ssh.program) otherwise makes this
        # commit fail ("failed to write commit object", exit 128) whenever the
        # signer can't sign — locked vault, headless drain, CI.
        "-c",
        "commit.gpgsign=false",
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
    assert "mcp__engram__list_memory" in ctx
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
    assert "mcp__engram__list_memory" in ctx
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
        check=False,
    )
    assert proc.returncode == 0
    assert proc.stdout.strip() == ""


def test_rules_index_instruction_present(git_repo: Path, monkeypatch):
    monkeypatch.delenv("ENGRAM_PROJECT", raising=False)
    result = run_hook(str(git_repo))
    assert result.returncode == 0
    ctx = json.loads(result.stdout)["hookSpecificOutput"]["additionalContext"]
    # Rules-index instruction names list_rules and the rule scope for the spine.
    assert "list_rules" in ctx
    assert "rule:repo:github.com/org/repo" in ctx
    # Ordering lock (ADR engram-d386): the index must render ABOVE the recall
    # digest. Pin the directive so a future reword cannot silently drop it.
    assert "ABOVE the recall digest" in ctx
    # No project scope when ENGRAM_PROJECT is unset.
    assert "rule:project:" not in ctx


def test_rules_index_includes_project_scope_when_configured(
    git_repo: Path, monkeypatch
):
    monkeypatch.setenv("ENGRAM_PROJECT", "selfhosted-cluster")
    ctx = json.loads(run_hook(str(git_repo)).stdout)["hookSpecificOutput"][
        "additionalContext"
    ]
    assert "rule:project:selfhosted-cluster" in ctx


PRIMER = """# engram session primer

Memory spine scope: repo:github.com/org/repo
Spine scope source: git origin remote
Rule scopes: rule:repo:github.com/org/repo

## Rules

- abcdefgh12 — MUST pass an explicit pathspec [git]
"""


def stub_engram(bin_dir: Path, body: str) -> Path:
    stub = bin_dir / "engram"
    stub.write_text("#!/bin/sh\n" + body)
    stub.chmod(0o755)
    return stub


def context_of(result: subprocess.CompletedProcess) -> str:
    assert result.returncode == 0, result.stderr
    return json.loads(result.stdout)["hookSpecificOutput"]["additionalContext"]


def assert_fallback(ctx: str) -> None:
    assert "Memory spine scope: repo:github.com/org/repo" in ctx
    assert "REQUIRED FIRST ACTION" in ctx
    assert "mcp__engram__list_memory" in ctx
    assert "mcp__engram__list_rules" in ctx


def test_primer_injected_when_prime_succeeds(git_repo: Path, bin_dir: Path):
    log = bin_dir.parent / "prime-log"
    log.mkdir()
    stub_engram(
        bin_dir,
        f"""printf '%s\\n' "$*" > "{log}/args"
pwd -P > "{log}/cwd"
if [ -p /dev/stdin ]; then echo pipe; else echo other; fi > "{log}/stdin"
printf '%s' '{PRIMER}'
""",
    )
    ctx = context_of(run_hook(str(git_repo)))
    assert ctx.startswith(PRIMER.rstrip())
    assert "already recalled" in ctx
    assert "MUST-follow" in ctx
    assert "Memory store precedence" in ctx
    assert "curating-memory" in ctx
    assert "REQUIRED FIRST ACTION" not in ctx
    assert "mcp__engram__list_memory" not in ctx
    assert "mcp__engram__list_rules" not in ctx
    assert (log / "args").read_text().split() == ["prime", "--timeout", "3s"]
    assert (log / "cwd").read_text().strip() == str(git_repo.resolve())
    assert (log / "stdin").read_text().strip() == "other"


def test_fallback_when_engram_missing(git_repo: Path):
    assert_fallback(context_of(run_hook(str(git_repo))))


@pytest.mark.parametrize(
    "body",
    [
        "echo 'Error: the saved login is no longer valid' >&2\nexit 3\n",
        'echo \'Error: unknown command "prime" for "engram"\' >&2\nexit 1\n',
        "exit 0\n",
    ],
    ids=["no credential", "older binary without prime", "empty output"],
)
def test_fallback_when_prime_fails(git_repo: Path, bin_dir: Path, body: str):
    stub_engram(bin_dir, body)
    result = run_hook(str(git_repo))
    assert_fallback(context_of(result))
    assert "using the instruction text" in result.stderr


def test_fallback_when_prime_does_not_finish(git_repo: Path, bin_dir: Path):
    # sleep runs as a child of the stub shell, not exec'd, so it keeps the
    # stdout pipe open unless the hook kills the whole process group.
    stub_engram(bin_dir, f'"{SLEEP}" 30\n')
    start = time.monotonic()
    result = run_hook(str(git_repo))
    elapsed = time.monotonic() - start
    assert_fallback(context_of(result))
    assert "did not finish" in result.stderr
    assert elapsed < 10, f"hook took {elapsed:.1f} s, over its 10 s limit"
