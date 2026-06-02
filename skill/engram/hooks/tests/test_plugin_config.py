"""Validate memory-curator plugin config files."""

from __future__ import annotations

import json
from pathlib import Path

PLUGIN_ROOT = Path(__file__).resolve().parents[2]  # memory-curator/
HOOKS_DIR = PLUGIN_ROOT / "hooks"


def load(path: Path) -> dict:
    with path.open() as fh:
        return json.load(fh)


def test_mcp_declares_oauth_server():
    cfg = load(PLUGIN_ROOT / ".mcp.json")
    server = cfg["mcpServers"]["memory_oauth"]
    assert server["type"] == "http"
    assert server["url"] == "https://litellm.fzymgc.house/mcp/memory_oauth"
    assert "headers" not in server  # OAuth: no static secret
    assert server["oauth"]["callbackPort"] == 8765


def test_plugin_manifest_minimal():
    manifest = load(PLUGIN_ROOT / "plugin.json")
    assert manifest["name"] == "memory-curator"
    assert manifest["description"]


def test_hooks_register_sessionstart_and_posttooluse():
    hooks = load(HOOKS_DIR / "hooks.json")["hooks"]
    assert "SessionStart" in hooks and "PostToolUse" in hooks
    # No blocking Stop hook: it rendered as a "Stop hook error" every session.
    assert "Stop" not in hooks
    ss = hooks["SessionStart"][0]
    assert ss["matcher"] == "startup|clear|compact"
    assert "session-start-memory-recall" in ss["hooks"][0]["command"]
    # PostToolUse nudge is scoped to mutating tools (regex on tool_name).
    ptu = hooks["PostToolUse"][0]
    assert ptu["matcher"] == "Edit|Write|NotebookEdit"
    assert "posttooluse-memory-capture-nudge" in ptu["hooks"][0]["command"]


def test_no_stale_session_end_hook():
    assert not (HOOKS_DIR / "session-end-memory-capture").exists()
