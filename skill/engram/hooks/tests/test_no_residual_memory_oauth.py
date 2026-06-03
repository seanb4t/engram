# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Sean Brandt

"""Guards on the *shipped* bundle: no pre-rebrand branding, no private hosts.

Scope is the shipped bundle (skills, hooks, manifests, commands).
Only the ``hooks/tests/`` scaffolding is excluded — these guards and
``test_plugin_config`` must name the banned strings in order to assert against
them, and tests are never shipped to consumers. The exclusion is pinned to
exactly ``hooks/tests/`` so a future shipped subtree that merely contains a
``tests`` path segment is still scanned.
"""

from __future__ import annotations

from pathlib import Path

BUNDLE = Path(__file__).resolve().parents[2]  # skill/engram/
# Built non-literally so this guard file is itself clean of the strings it bans.
OLD_IDS = ("memory" + "_oauth", "memory" + "-curator", "memory" + "_curator")
# Private deployment hosts must never ship — the url is consumer-configured via
# ${ENGRAM_MCP_URL}; a gateway like litellm is the deployer's own choice.
PRIVATE_HOSTS = ("fzymgc", "lite" + "llm")


def _scan_shipped(needles: tuple[str, ...]) -> list[str]:
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
        hits = [n for n in needles if n in text]
        if hits:
            offenders.append(f"{p.relative_to(BUNDLE)} ({', '.join(hits)})")
    return offenders


def test_no_residual_old_branding():
    offenders = _scan_shipped(OLD_IDS)
    assert not offenders, (
        f"residual pre-rebrand branding in shipped bundle: {offenders}"
    )


def test_no_private_hosts_in_shipped_bundle():
    offenders = _scan_shipped(PRIVATE_HOSTS)
    assert not offenders, f"private deployment host in shipped bundle: {offenders}"
