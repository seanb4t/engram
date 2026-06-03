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


def test_mcp_declares_engram_server():
    cfg = load(PLUGIN_ROOT / ".mcp.json")
    server = cfg["mcpServers"]["engram"]
    assert server["type"] == "http"
    assert server["url"].endswith("/mcp/engram")
    assert "headers" not in server  # OAuth: no static secret
    assert server["oauth"]["callbackPort"] == 8765


def test_plugin_manifest():
    manifest = load(PLUGIN_ROOT / ".claude-plugin" / "plugin.json")
    assert manifest["name"] == "engram"
    assert manifest["hooks"] == "./hooks/hooks.json"
    assert manifest["mcpServers"] == "./.mcp.json"


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
