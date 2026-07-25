# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Sean Brandt

"""Two-tier memory scope derivation (local git only — no network, no auth).

Public API: derive_scopes(cwd) -> Scopes(spine, overlay).
  spine   = "repo:<repo-id>"                      (always, when in a repo)
  overlay = "repo:<repo-id>:ws:<workspace>" | None (None for the primary checkout)
  Returns Scopes(None, None) when cwd is not inside any recognised repo.
"""

from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path
from typing import NamedTuple


class Scopes(NamedTuple):
    spine: str | None
    overlay: str | None


def _run(args: list[str], cwd: str) -> str | None:
    """Run a command; return stripped stdout, or None when the probe doesn't resolve.

    Two None cases are deliberately collapsed and treated the same by callers:
    a clean negative (nonzero exit — e.g. ``git rev-parse`` outside a git repo)
    and a failure (missing binary, timeout, permission error). They are
    indistinguishable at a bare ``rev-parse`` probe, so the safe contract is to
    degrade conservatively (no scope, or spine-only) rather than guess a tier.

    Abnormal failures (the exception branch) are logged to stderr because they are
    rare and worth diagnosing — a git timeout silently dropping the overlay is
    otherwise invisible. Nonzero exits are NOT logged: they are the expected
    "not a git repo" negative and would spam every session.
    """
    try:
        proc = subprocess.run(
            args, cwd=cwd, capture_output=True, text=True, timeout=5, check=False
        )
    except (OSError, subprocess.SubprocessError, ValueError) as exc:
        print(
            f"engram scope: {args[0]} probe failed ({exc}); degrading", file=sys.stderr
        )
        return None
    if proc.returncode != 0:
        return None
    return proc.stdout.strip()


def _normalize_remote(url: str) -> str:
    """Normalise a remote URL to a canonical host/org/repo key.

    Examples:
      git@github.com:org/repo.git        -> github.com/org/repo
      https://github.com/org/repo.git    -> github.com/org/repo
      https://github.com:443/org/repo    -> github.com/org/repo  (port dropped)
      ssh://git@github.com/org/repo      -> github.com/org/repo
    """
    u = url.strip()
    for scheme in ("https://", "http://", "ssh://", "git://"):
        if u.startswith(scheme):
            u = u[len(scheme) :]
            break
    head = u.split("/", 1)[0]
    if "@" in head:
        u = u.split("@", 1)[1]
        head = u.split("/", 1)[0]
    # After stripping the scheme (and optional user@), head may be "host:port"
    # (numeric port → drop it) or "host:path" (scp-style → replace : with /).
    # Only apply the scp replacement when the part after ':' is not all-digits.
    colon_idx = head.find(":")
    if colon_idx != -1:
        after_colon = head[colon_idx + 1 :]
        if re.match(r"^\d+$", after_colon):
            # Numeric port: strip "host:port" → "host", re-attach the rest of path
            rest = u[len(head) :]  # e.g. "/org/repo.git"
            u = head[:colon_idx] + rest
        else:
            # scp-style host:path → host/path
            u = u.replace(":", "/", 1)
    u = u.removesuffix(".git")
    return u.strip("/")


def _repo_id(cwd: str) -> str | None:
    # git origin, incl. linked worktrees (which share the origin remote)
    origin = _run(["git", "remote", "get-url", "origin"], cwd)
    if origin:
        return _normalize_remote(origin)
    common = _run(["git", "rev-parse", "--git-common-dir"], cwd)
    if common:
        p = Path(common)
        if not p.is_absolute():
            p = (Path(cwd) / p).resolve()
        return p.parent.name  # parent of the shared .git == main repo root
    return None


def _workspace(cwd: str) -> str | None:
    """Per-workspace name for the overlay; None for the primary checkout.

    git worktree: primary -> None; linked -> toplevel basename.
    """
    common = _run(["git", "rev-parse", "--git-common-dir"], cwd)
    toplevel = _run(["git", "rev-parse", "--show-toplevel"], cwd)
    if common and toplevel:
        cp = Path(common)
        if not cp.is_absolute():
            cp = (Path(cwd) / cp).resolve()
        if cp.parent.resolve() == Path(toplevel).resolve():
            return None  # primary worktree
        return Path(toplevel).name
    return None


def derive_scopes(cwd: str) -> Scopes:
    rid = _repo_id(cwd)
    if rid is None:
        return Scopes(None, None)
    spine = f"repo:{rid}"
    ws = _workspace(cwd)
    overlay = f"{spine}:ws:{ws}" if ws else None
    return Scopes(spine, overlay)
