# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Sean Brandt

"""Validate the engram bundled skill-plugin config."""

from __future__ import annotations

import json
from pathlib import Path

PLUGIN_ROOT = Path(__file__).resolve().parents[2]  # skill/engram/
REPO_ROOT = PLUGIN_ROOT.parents[1]  # engram/


def load(path: Path) -> dict:
    with path.open() as fh:
        return json.load(fh)


def test_no_bundled_mcp_server():
    # The plugin ships no .mcp.json. A static localhost default produced a
    # permanently-failing duplicate server for remote/gateway deployments and
    # was redundant with the user-scope server /engram-setup registers (which
    # always outranks a plugin def). Registration is via /engram-setup only.
    assert not (PLUGIN_ROOT / ".mcp.json").exists()


def test_plugin_manifest():
    manifest = load(PLUGIN_ROOT / ".claude-plugin" / "plugin.json")
    assert manifest["name"] == "engram"
    assert manifest["hooks"] == "./hooks/hooks.json"
    assert "mcpServers" not in manifest  # no bundled server; see /engram-setup


def test_marketplace_points_at_bundle():
    mkt = load(REPO_ROOT / ".claude-plugin" / "marketplace.json")
    names = {p["name"]: p for p in mkt["plugins"]}
    assert "engram" in names
    assert names["engram"]["source"] == "./skill/engram"


def test_hooks_register_sessionstart_and_posttooluse():
    hooks = load(PLUGIN_ROOT / "hooks" / "hooks.json")["hooks"]
    assert set(hooks) == {"SessionStart", "PostToolUse"}
    assert "Stop" not in hooks  # the blocking Stop nudge was removed
    ss = hooks["SessionStart"][0]
    assert ss["matcher"] == "startup|clear|compact"
    assert "session-start-memory-recall" in ss["hooks"][0]["command"]
    ptu = hooks["PostToolUse"][0]
    assert ptu["matcher"] == "Edit|Write|NotebookEdit"
    assert "posttooluse-memory-capture-nudge" in ptu["hooks"][0]["command"]


def test_engram_setup_command_shape():
    cmd = (PLUGIN_ROOT / "commands" / "engram-setup.md").read_text(encoding="utf-8")
    assert cmd.startswith("---\n")  # YAML frontmatter on line 1 (loader requires it)
    frontmatter = cmd.split("---", 2)[1]
    assert "description:" in frontmatter
    assert "argument-hint:" in frontmatter
    assert "disable-model-invocation: true" in frontmatter  # explicit /-invoke only
    # The body registers a user-scope engram server via the supported CLI.
    assert "claude mcp add --transport http engram" in cmd
    assert "--scope user" in cmd
