<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->
<!-- markdownlint-disable MD013 -->

# Relocate memory-curator into engram — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use dev-flow:subagent-driven-development (recommended) or dev-flow:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the `memory-curator` client plugin from `fzymgc-house-skills` into
`engram` as a bundled skill-plugin under `skill/engram/`, rebranded to `engram`,
then remove it from the old repo.

**Architecture:** A `.claude-plugin/plugin.json` inside `skill/engram/` makes the
folder a bundled skill-plugin (`engram@skills-dir`) carrying both skills,
`hooks/hooks.json`, and `.mcp.json`. A repo-root `.claude-plugin/marketplace.json`
also exposes it for `/plugin install engram`. The MCP server id and all tool
prefixes rebrand `memory_oauth` → `engram`. engram CI gains a Python lane and
Apache-2.0/SPDX headers cover the bundle.

**Tech Stack:** Claude Code plugin/skill system; `uv`-run Python 3.11 hooks
(pytest + ruff); `task` (Taskfile); `skywalking-eyes` (license-eye); GitHub
Actions; jj (colocated).

**Spec:** `docs/superpowers/specs/2026-06-02-relocate-memory-curator-into-engram-design.md`

**Grounding (Rule 7, recorded inline — bd unavailable, see note):**

- Source bundle = `fzymgc-house-skills` `origin/main` @ PR #135 / `v1.20.1`:
  `skills/{curating-memory,promoting-memory}/SKILL.md`; `hooks/`
  = `hooks.json` (SessionStart + PostToolUse), `session-start-memory-recall`,
  `posttooluse-memory-capture-nudge`, `lib/scope.py`, `tests/` (4 test modules +
  `__init__.py`); `.mcp.json`; `plugin.json`. **Two** hooks, two skills.
- engram tooling: `Taskfile.yaml` (`lint` deps `[go,yaml,actions,markdown]`,
  `test`=`go test ./...`, `fmt` deps `[go,dprint,yaml]`, `license:check`=
  `license-eye header check`); `.github/workflows/ci.yaml` jobs `[test, golangci,
  license, chart, workflows]`; `.licenserc.yaml` paths `[cmd/**, internal/**,
  docs/**, *.md]`, Markdown=AngleBracket. **No Python lane** — added here.
- `skywalking-eyes` `language` block supports a **`filenames`** key (not only
  `extensions`) + `comment_style_id: Hashtag` + `license-location-threshold`
  (shebang offset) — so the extensionless `uv` hook scripts are licensable by
  filename without renaming. (deepwiki: apache/skywalking-eyes)
- Bundled skill-plugin `plugin.json` accepts `hooks` and `mcpServers` as path
  refs (`./hooks/hooks.json`, `./.mcp.json`) — same field names as marketplace
  plugins. (code.claude.com/docs/en/plugins-reference.md)

> **bd note:** This plan was authored with engram's bd workspace resolution
> broken (adding the `relocate-memory-curator` jj workspace confused bd's
> worktree detection; `bd where` → "No active beads workspace found"). The
> design bead `engram-w8b` and its notes are intact (`.beads` is not jj-tracked).
> Resolve bd resolution before running `plan-to-beads`; until then, this plan is
> the source of truth.

---

## File structure

**Created in engram (PR-1):**

- `skill/engram/.claude-plugin/plugin.json` — bundled skill-plugin manifest.
- `skill/engram/skills/curating-memory/SKILL.md`, `…/promoting-memory/SKILL.md`
  — the two skills (copied, rebranded).
- `skill/engram/hooks/{hooks.json, session-start-memory-recall,
  posttooluse-memory-capture-nudge, lib/scope.py}` — copied, rebranded.
- `skill/engram/hooks/tests/{__init__.py, test_scope.py,
  test_session_start_memory_recall.py, test_posttooluse_memory_capture_nudge.py,
  test_plugin_config.py}` — copied, rebranded + retargeted.
- `skill/engram/hooks/tests/test_no_residual_memory_oauth.py` — new completeness test.
- `skill/engram/.mcp.json` — rebranded server id + url.
- `.claude-plugin/marketplace.json` — repo-root marketplace.

**Modified in engram (PR-1):**

- `.licenserc.yaml` — cover `skill/**`; Python + extensionless-script language maps.
- `Taskfile.yaml` — `lint:python`, `test:python`, `fmt:python` targets + wiring.
- `.github/workflows/ci.yaml` — a `python` job.

**Modified/removed in fzymgc-house-skills (PR-2):**

- Remove `memory-curator/`, `plugins/memory-curator/`, both marketplace entries,
  Taskfile memory-curator gates; add README redirect. Design docs/ADRs left.

---

## PR-1 — engram import + stand-up

> Work in the `relocate-memory-curator` jj workspace
> (`/Users/sean/Code/github.com/seanb4t/engram_worktrees/relocate-memory-curator`),
> bookmark `engram-w8b-design`, based on `engram` `main`. Set
> `SRC=/Volumes/Code/github.com/fzymgc-house/fzymgc-house-skills/memory-curator`
> (the refreshed fzymgc working copy, == `origin/main`).

### Task 1: Scaffold `skill/engram/` and import the source bundle

**Files:**

- Create: `skill/engram/` (tree), copied from `$SRC`.

- [ ] **Step 1: Create the bundle directories**

```bash
cd /Users/sean/Code/github.com/seanb4t/engram_worktrees/relocate-memory-curator
mkdir -p skill/engram/.claude-plugin skill/engram/skills
```

- [ ] **Step 2: Copy skills, hooks, and the MCP declaration verbatim**

```bash
SRC=/Volumes/Code/github.com/fzymgc-house/fzymgc-house-skills/memory-curator
cp -Rf "$SRC/skills/curating-memory"  skill/engram/skills/
cp -Rf "$SRC/skills/promoting-memory" skill/engram/skills/
cp -Rf "$SRC/hooks"                   skill/engram/hooks
cp -f  "$SRC/.mcp.json"               skill/engram/.mcp.json
# Drop any pycache the copy may have pulled in:
find skill/engram -name '__pycache__' -type d -prune -exec rm -rf {} +
```

- [ ] **Step 3: Verify the imported tree**

Run:

```bash
find skill/engram -type f -not -name '*.pyc' | sort
```

Expected (exact set, before adding plugin.json/marketplace/headers):

```text
skill/engram/.mcp.json
skill/engram/hooks/hooks.json
skill/engram/hooks/lib/__init__.py
skill/engram/hooks/lib/scope.py
skill/engram/hooks/posttooluse-memory-capture-nudge
skill/engram/hooks/session-start-memory-recall
skill/engram/hooks/tests/__init__.py
skill/engram/hooks/tests/test_plugin_config.py
skill/engram/hooks/tests/test_posttooluse_memory_capture_nudge.py
skill/engram/hooks/tests/test_scope.py
skill/engram/hooks/tests/test_session_start_memory_recall.py
skill/engram/skills/curating-memory/SKILL.md
skill/engram/skills/promoting-memory/SKILL.md
```

- [ ] **Step 4: Confirm the hook scripts stay executable**

```bash
chmod +x skill/engram/hooks/session-start-memory-recall skill/engram/hooks/posttooluse-memory-capture-nudge
test -x skill/engram/hooks/session-start-memory-recall && echo OK
```

Expected: `OK`

- [ ] **Step 5: Commit**

```bash
jj describe -m "feat(skill): import memory-curator bundle into skill/engram (engram-w8b)"
jj new   # start a fresh change for the next task
```

### Task 2: Rebrand `memory_oauth` → `engram`

The MCP server id and every tool prefix change. This is a pure string rewrite
across the bundle; Task 7 adds a test that asserts zero residual strings.

**Files:**

- Modify: `skill/engram/.mcp.json`
- Modify: `skill/engram/skills/*/SKILL.md`
- Modify: `skill/engram/hooks/{session-start-memory-recall, posttooluse-memory-capture-nudge}`
- Modify: `skill/engram/hooks/tests/*.py`
- [ ] **Step 1: Rewrite the `.mcp.json` server id and url**

Replace the file contents with:

```json
{
  "mcpServers": {
    "engram": {
      "type": "http",
      "url": "https://litellm.fzymgc.house/mcp/engram",
      "oauth": { "callbackPort": 8765 }
    }
  }
}
```

> **Prerequisite (external, from the spec):** the litellm gateway route
> `memory_oauth` must be renamed/aliased to `engram` before this ships, or the
> `url` stays `…/mcp/memory_oauth` (the client server id `engram` and the route
> path are independent — see spec "External prerequisite"). Pick one and keep
> Task 8's round-trip target consistent with it.

- [ ] **Step 2: Rewrite tool prefixes and server-name prose in skills + hooks**

```bash
cd skill/engram
# tool-call prefix
grep -rlZ 'mcp__memory_oauth__' skills hooks | xargs -0 sed -i '' 's/mcp__memory_oauth__/mcp__engram__/g'
# bare server-id references in prose / .mcp.json key mentions
grep -rlZ 'memory_oauth' skills hooks | xargs -0 sed -i '' 's/memory_oauth/engram/g'
cd ../..
```

- [ ] **Step 3: Verify zero residual references**

Run:

```bash
grep -rn 'memory_oauth' skill/engram || echo "CLEAN"
```

Expected: `CLEAN`

- [ ] **Step 4: Sanity-run the hooks (scope derivation still works)**

```bash
echo '{"cwd": "'"$PWD"'", "session_id": "rebrand-smoke"}' | uv run skill/engram/hooks/session-start-memory-recall \
  | python3 -c 'import json,sys; c=json.load(sys.stdin)["hookSpecificOutput"]["additionalContext"]; assert "mcp__engram__list_memory" in c, c; print("OK")'
rm -f "${TMPDIR:-/tmp}/memory-curator-capture-nudge-rebrand-smoke" 2>/dev/null || true
```

Expected: `OK`

- [ ] **Step 5: Commit**

```bash
jj describe -m "refactor(skill): rebrand memory_oauth -> engram across the bundle (engram-w8b)"
jj new
```

### Task 3: Write the bundled skill-plugin manifest

**Files:**

- Create: `skill/engram/.claude-plugin/plugin.json`

- [ ] **Step 1: Write `plugin.json`**

```json
{
  "name": "engram",
  "description": "Self-hosted, correctable, OAuth-secured memory for coding agents: session-start recall, curation discipline, and a two-tier per-workspace memory scope, wired to the engram MCP server.",
  "hooks": "./hooks/hooks.json",
  "mcpServers": "./.mcp.json"
}
```

- [ ] **Step 2: Verify JSON validity**

Run: `jq empty skill/engram/.claude-plugin/plugin.json && echo OK`
Expected: `OK`

- [ ] **Step 3: Commit**

```bash
jj describe -m "feat(skill): add engram bundled skill-plugin manifest (engram-w8b)"
jj new
```

### Task 4: Write the repo-root marketplace

**Files:**

- Create: `.claude-plugin/marketplace.json`

- [ ] **Step 1: Write `marketplace.json`**

```json
{
  "name": "engram",
  "owner": { "name": "Sean Brandt", "url": "https://github.com/seanb4t" },
  "plugins": [
    {
      "name": "engram",
      "description": "Self-hosted, correctable, OAuth-secured memory for coding agents (skills + hooks + MCP).",
      "source": "./skill/engram"
    }
  ]
}
```

- [ ] **Step 2: Verify JSON validity**

Run: `jq empty .claude-plugin/marketplace.json && echo OK`
Expected: `OK`

- [ ] **Step 3: Commit**

```bash
jj describe -m "feat: expose engram plugin via repo-root marketplace (engram-w8b)"
jj new
```

### Task 5: SPDX headers + `.licenserc.yaml` coverage

The hooks ship UNLICENSED from fzymgc; engram is Apache-2.0 with enforced
headers. `.py` files use `#` headers; the two extensionless `uv` scripts are
mapped by `filenames`; markdown uses AngleBracket; JSON cannot carry comments and
is ignored.

**Files:**

- Modify: `.licenserc.yaml`
- Modify: `skill/engram/hooks/**` (headers added by `license-eye header fix`)
- [ ] **Step 1: Extend `.licenserc.yaml`**

Under `header.paths`, add `- "skill/**"`. Under `header.paths-ignore`, add the
comment-incapable JSON files:

```yaml
    - "skill/**/*.json"
```

(`skill/**/*.json` covers `.mcp.json`, `.claude-plugin/plugin.json`, and
`hooks/hooks.json` — all comment-incapable.)

Under `header.language`, add (alongside the existing `Markdown` entry):

```yaml
    Python:
      extensions: [".py"]
      comment_style_id: Hashtag
    UvScript:
      filenames:
        - "session-start-memory-recall"
        - "posttooluse-memory-capture-nudge"
      comment_style_id: Hashtag
```

And, to let the header sit beneath the shebang + PEP 723 preamble, add at the
`header` level:

```yaml
  license-location-threshold: 80
```

- [ ] **Step 2: Add headers, then verify**

Run:

```bash
license-eye header fix
license-eye header check
```

Expected: `header check` exits 0 (all `skill/**` `.py` + extensionless scripts +
`SKILL.md` carry the SPDX header; `.json` ignored).

- [ ] **Step 3: Confirm the shebang is still line 1 of each script**

```bash
head -1 skill/engram/hooks/session-start-memory-recall
head -1 skill/engram/hooks/posttooluse-memory-capture-nudge
```

Expected: both print `#!/usr/bin/env -S uv run --script`. (If `header fix`
displaced the shebang, move the 2-line `# SPDX…` / `# Copyright…` block to
directly beneath the shebang by hand and re-run `header check`.)

- [ ] **Step 4: Commit**

```bash
jj describe -m "chore(skill): Apache-2.0 SPDX headers + license-eye coverage for the bundle (engram-w8b)"
jj new
```

### Task 6: Add the Python CI lane

**Files:**

- Modify: `Taskfile.yaml`
- Modify: `.github/workflows/ci.yaml`
- [ ] **Step 1: Add Python targets to `Taskfile.yaml`**

Add these tasks and wire them into the aggregates. New tasks:

```yaml
  lint:python:
    cmds:
      - uv run --with ruff ruff check skill/engram/hooks
      - uv run --with ruff ruff format --check skill/engram/hooks
  test:python:
    cmds:
      - uv run --with pytest pytest skill/engram/hooks/tests -q
  fmt:python:
    cmds:
      - uv run --with ruff ruff format skill/engram/hooks
```

Wire into the aggregates by editing their `deps`:

- `lint.deps`: add `'lint:python'`
- `test`: change to also run python — make it `deps: ['test:go', 'test:python']`
  with a new `test:go` task holding the existing `go test ./...` cmd. (Mirror the
  `lint` aggregate pattern so `task test` runs both.)
- `fmt.deps`: add `'fmt:python'`
- `lint:markdown` already runs `rumdl check .`, which now also covers the bundle's
  markdown — no change needed.
- [ ] **Step 2: Add a `python` job to `ci.yaml`**

```yaml
  python:
    name: python
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: astral-sh/setup-uv@v8
      - name: ruff
        run: |
          uv run --with ruff ruff check skill/engram/hooks
          uv run --with ruff ruff format --check skill/engram/hooks
      - name: pytest
        run: uv run --with pytest pytest skill/engram/hooks/tests -q
```

- [ ] **Step 3: Verify locally**

Run:

```bash
task lint:python
task test:python
```

Expected: ruff clean; pytest collects and passes the moved suite (count asserted
in Task 7).

- [ ] **Step 4: Lint the workflow file**

Run: `actionlint .github/workflows/ci.yaml && echo OK`
Expected: `OK`

- [ ] **Step 5: Commit**

```bash
jj describe -m "ci(skill): add python lane (ruff + pytest) for the engram bundle (engram-w8b)"
jj new
```

### Task 7: Update `test_plugin_config` + add completeness test

**Files:**

- Modify: `skill/engram/hooks/tests/test_plugin_config.py`
- Create: `skill/engram/hooks/tests/test_no_residual_memory_oauth.py`
- [ ] **Step 1: Rewrite the config-validation test for the new layout**

The old test asserted the fzymgc `memory-curator` paths and `memory_oauth` id.
Replace its body so it validates the engram bundle. Key change: `PLUGIN_ROOT` is
now `skill/engram/` and the manifest is `.claude-plugin/plugin.json`.

```python
"""Validate the engram bundled skill-plugin config."""

from __future__ import annotations

import json
from pathlib import Path

PLUGIN_ROOT = Path(__file__).resolve().parents[2]  # skill/engram/
REPO_ROOT = PLUGIN_ROOT.parents[1]                 # engram/


def load(path: Path) -> dict:
    with path.open() as fh:
        return json.load(fh)


def test_mcp_declares_engram_server():
    cfg = load(PLUGIN_ROOT / ".mcp.json")
    server = cfg["mcpServers"]["engram"]
    assert server["type"] == "http"
    assert server["url"].endswith("/mcp/engram") or server["url"].endswith("/mcp/memory_oauth")
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
    assert "Stop" not in hooks
    assert hooks["PostToolUse"][0]["matcher"] == "Edit|Write|NotebookEdit"
```

- [ ] **Step 2: Run it (expect FAIL until paths/ids are right)**

Run: `uv run --with pytest pytest skill/engram/hooks/tests/test_plugin_config.py -q`
Expected: PASS (the bundle from Tasks 1–4 satisfies every assertion).

- [ ] **Step 3: Write the rebrand-completeness test**

```python
"""Guard: no residual memory_oauth strings survive the rebrand."""

from __future__ import annotations

from pathlib import Path

BUNDLE = Path(__file__).resolve().parents[2]  # skill/engram/


def test_no_residual_memory_oauth():
    offenders = []
    for p in BUNDLE.rglob("*"):
        if not p.is_file() or "__pycache__" in p.parts:
            continue
        try:
            text = p.read_text(encoding="utf-8")
        except (UnicodeDecodeError, OSError):
            continue
        if "memory_oauth" in text:
            offenders.append(str(p.relative_to(BUNDLE)))
    assert not offenders, f"residual memory_oauth in: {offenders}"
```

- [ ] **Step 4: Run the full bundle suite**

Run: `uv run --with pytest pytest skill/engram/hooks/tests -q`
Expected: PASS, **5 test modules** — `test_scope`,
`test_session_start_memory_recall`, `test_posttooluse_memory_capture_nudge`
(moved intact), `test_plugin_config` (rewritten), `test_no_residual_memory_oauth`
(new). If `test_plugin_config` from fzymgc referenced `memory_oauth`, Step 1
already removed it.

- [ ] **Step 5: Commit**

```bash
jj describe -m "test(skill): validate engram bundle config + rebrand completeness (engram-w8b)"
jj new
```

### Task 8: Full gate run + manual verification + open PR-1

**Files:** none (verification + PR).

- [ ] **Step 1: Run every local gate**

Run:

```bash
task lint        # go, yaml, actions, markdown, python
task test        # go + python
license-eye header check
jq empty skill/engram/.mcp.json skill/engram/.claude-plugin/plugin.json .claude-plugin/marketplace.json
```

Expected: all exit 0.

- [ ] **Step 2: Rebase onto current main + push the bookmark**

```bash
jj git fetch                                   # ensure local main is current first
jj rebase -s engram-w8b-design -o main
jj git push --bookmark engram-w8b-design --allow-new
```

- [ ] **Step 3: Manual install verification (both paths)**

Document the result in the PR description. Marketplace path:

```text
/plugin marketplace add seanb4t/engram
/plugin install engram
```

Skills-dir path (on a test machine, not alongside the marketplace install):

```bash
ln -s "$PWD/skill/engram" ~/.claude/skills/engram
```

In a fresh session, confirm: skills `engram:curating-memory` /
`engram:promoting-memory` appear; the `engram` MCP server registers; a live
`search_memory` + `store_memory` round-trip succeeds against whichever route
`.mcp.json` points at (renamed `engram`, or the `memory_oauth` fallback).

- [ ] **Step 4: Open the PR**

```bash
gh pr create -R seanb4t/engram --base main --head engram-w8b-design \
  --title "feat(skill): relocate memory-curator into engram as the engram bundled skill-plugin" \
  --body "Implements docs/superpowers/specs/2026-06-02-relocate-memory-curator-into-engram-design.md (engram-w8b). PR-2 (removal from fzymgc-house-skills) follows once this is verified."
```

---

## PR-2 — remove from fzymgc-house-skills + redirect

> Separate repo. Create an isolated worktree off `fzymgc-house-skills` `main`
> via the `dev-flow:using-worktrees` skill. Do NOT start until PR-1 is merged and
> verified.

### Task 9: Remove the plugin and add a redirect

**Files (in fzymgc-house-skills):**

- Delete: `memory-curator/`, `plugins/memory-curator/`
- Modify: `.claude-plugin/marketplace.json`, `.agents/plugins/marketplace.json`,
  `Taskfile.yaml`, `README.md`
- Keep (historical): `docs/adr/*`, `docs/superpowers/specs|plans/*memory-curator*`
- [ ] **Step 1: Remove the plugin source + Codex wrapper**

```bash
rm -rf memory-curator plugins/memory-curator
```

- [ ] **Step 2: Drop the marketplace entries**

In `.claude-plugin/marketplace.json` remove the `memory-curator` object from
`plugins`. In `.agents/plugins/marketplace.json` remove the `memory-curator`
object from `plugins`.

Run after editing: `jq empty .claude-plugin/marketplace.json .agents/plugins/marketplace.json && echo OK`
Expected: `OK`, and neither file mentions `memory-curator`:

```bash
grep -rn memory-curator .claude-plugin .agents/plugins || echo CLEAN
```

Expected: `CLEAN`

- [ ] **Step 3: Remove the memory-curator Taskfile gates**

In `Taskfile.yaml`, remove `memory-curator/...` paths from the `rumdl` markdown
list and the pytest test-paths list (the `vars` block + the `lint`/`test`
commands). After editing:

```bash
grep -n memory-curator Taskfile.yaml || echo CLEAN
task lint && task test
```

Expected: `CLEAN`; both gates pass (memory-curator suite no longer collected).

- [ ] **Step 4: Add a README redirect**

Append a short note to `README.md` (and/or `CHANGELOG`):

```markdown
> **`memory-curator` has moved to [seanb4t/engram](https://github.com/seanb4t/engram)**
> and is now the `engram` bundled skill-plugin. Install via
> `/plugin marketplace add seanb4t/engram` → `/plugin install engram`.
> The design history (spec/plan/ADRs) remains in this repo under `docs/`.
```

- [ ] **Step 5: Commit + open PR**

```bash
jj describe -m "chore: remove memory-curator (relocated to seanb4t/engram)"
# create bookmark, push, gh pr create -R fzymgc-house/fzymgc-house-skills ...
```

---

## Done criteria

- PR-1 merged: `engram` plugin installs (marketplace + skills-dir), MCP server
  registers, live memory round-trip works; engram CI green incl. the Python lane
  and license headers.
- PR-2 merged: `fzymgc-house-skills` no longer ships `memory-curator`; gates
  green; README redirects to engram; design docs retained.
