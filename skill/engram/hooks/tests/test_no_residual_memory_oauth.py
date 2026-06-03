# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Sean Brandt

"""Guard: no residual pre-rebrand branding survives in the shipped bundle.

Checks both the old server id (``memory_oauth``) and the old plugin name
(``memory-curator`` / ``memory_curator``). Scope is the *shipped* bundle
(skills, hooks, manifests, .mcp.json). Only the ``hooks/tests/`` scaffolding is
excluded — this guard and ``test_plugin_config`` must name the old identifiers
in order to assert against them, and tests are never shipped to consumers. The
exclusion is pinned to exactly ``hooks/tests/`` so a future shipped subtree that
merely contains a ``tests`` path segment is still scanned.
"""

from __future__ import annotations

from pathlib import Path

BUNDLE = Path(__file__).resolve().parents[2]  # skill/engram/
# Built non-literally so this guard file is itself clean of the strings it bans.
OLD_IDS = ("memory" + "_oauth", "memory" + "-curator", "memory" + "_curator")


def test_no_residual_old_branding():
    offenders = []
    for p in BUNDLE.rglob("*"):
        if not p.is_file() or "__pycache__" in p.parts:
            continue
        if p.relative_to(BUNDLE).parts[:2] == ("hooks", "tests"):
            continue
        try:
            text = p.read_text(encoding="utf-8")
        except (UnicodeDecodeError, OSError):
            continue
        hits = [old for old in OLD_IDS if old in text]
        if hits:
            offenders.append(f"{p.relative_to(BUNDLE)} ({', '.join(hits)})")
    assert not offenders, (
        f"residual pre-rebrand branding in shipped bundle: {offenders}"
    )
