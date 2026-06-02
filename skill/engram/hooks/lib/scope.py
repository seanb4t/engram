"""Two-tier memory scope derivation (local git/jj only — no network, no auth).

Public API: derive_scopes(cwd) -> Scopes(spine, overlay).
  spine   = "repo:<repo-id>"                      (always, when in a repo)
  overlay = "repo:<repo-id>:ws:<workspace>" | None (None for the primary checkout)
  Returns Scopes(None, None) when cwd is not inside any recognised repo.
"""

from __future__ import annotations

import re
import subprocess
from pathlib import Path
from typing import NamedTuple


class Scopes(NamedTuple):
    spine: str | None
    overlay: str | None


def _run(args: list[str], cwd: str) -> str | None:
    """Run a command; return stripped stdout, or None on any failure."""
    try:
        proc = subprocess.run(args, cwd=cwd, capture_output=True, text=True, timeout=5)
    except (OSError, subprocess.SubprocessError, ValueError):
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
    if u.endswith(".git"):
        u = u[:-4]
    return u.strip("/")


def _origin_from_jj(remotes: str | None) -> str | None:
    if not remotes:
        return None
    for line in remotes.splitlines():
        parts = line.split()
        if len(parts) >= 2 and parts[0] == "origin":
            return parts[1]
    return None


def _jj_primary_root(workspace_root: str | None) -> str | None:
    """Resolve the primary workspace root via the .jj/repo store pointer."""
    if not workspace_root:
        return None
    pointer = Path(workspace_root) / ".jj" / "repo"
    if pointer.is_dir():
        return workspace_root  # default workspace: .jj/repo is the store dir
    if pointer.is_file():
        try:
            target = pointer.read_text().strip()
        except OSError:
            return None
        tp = Path(target)
        if not tp.is_absolute():
            tp = (pointer.parent / target).resolve()
        if tp.name == "repo" and tp.parent.name == ".jj":
            return str(tp.parent.parent)
    return None


def _repo_id(cwd: str) -> str | None:
    # jj-first: a jj workspace is NOT a git repo, so git remote fails there.
    jj_root = _run(["jj", "--no-pager", "root"], cwd)
    if jj_root is not None:
        origin = _origin_from_jj(
            _run(["jj", "--no-pager", "git", "remote", "list"], cwd)
        )
        if origin:
            return _normalize_remote(origin)
        primary = _jj_primary_root(jj_root)
        return Path(primary).name if primary else None
    # pure git (including linked worktrees, which share origin)
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
    """Per-workspace name for the overlay; None for the primary checkout."""
    jj_root = _run(["jj", "--no-pager", "root"], cwd)
    if jj_root is not None:
        wc = _run(
            [
                "jj",
                "--no-pager",
                "log",
                "-r",
                "@",
                "--no-graph",
                "-T",
                "working_copies",
            ],
            cwd,
        )
        if wc:
            parts = wc.split("@")[0].split()
            name = parts[0] if parts else ""
            if name and name != "default":
                return name
        return None
    # git worktree: primary -> None; linked -> toplevel basename
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
