# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Sean Brandt

"""Guard: no residual memory_oauth strings survive the rebrand.

Scope is the *shipped* bundle (skills, hooks, manifests, .mcp.json). The
``tests/`` subtree is excluded: this guard and ``test_plugin_config`` must
name the old identifier in order to assert against it, and that scaffolding
is never shipped to plugin consumers.
"""

from __future__ import annotations

from pathlib import Path

BUNDLE = Path(__file__).resolve().parents[2]  # skill/engram/
OLD_ID = "memory" + "_oauth"  # avoid a literal so this file is self-clean too


def test_no_residual_memory_oauth():
    offenders = []
    for p in BUNDLE.rglob("*"):
        if not p.is_file() or "__pycache__" in p.parts:
            continue
        if "tests" in p.relative_to(BUNDLE).parts:
            continue
        try:
            text = p.read_text(encoding="utf-8")
        except (UnicodeDecodeError, OSError):
            continue
        if OLD_ID in text:
            offenders.append(str(p.relative_to(BUNDLE)))
    assert not offenders, f"residual {OLD_ID} in shipped bundle: {offenders}"
