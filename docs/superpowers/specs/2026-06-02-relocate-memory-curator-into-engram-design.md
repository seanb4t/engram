<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Relocate the `memory-curator` plugin into engram

**Date:** 2026-06-02
**Status:** Design
**Design bead:** engram-w8b

## Context

The `engram` server (this repo) is the self-hosted, OAuth-secured memory MCP
backend: `cmd/engram`, `internal/{auth,embed,store,server}`, and the
`charts/engram` Helm chart. The **client** that drives it — the
`memory-curator` plugin (two skills, two hooks — `SessionStart` + `PostToolUse`
— and an MCP server declaration) —
lives in a *different* repo, `fzymgc-house-skills`. The two are semantically
coupled: the plugin's scope convention, tool names, and auth flow are all
contracts with this server. Splitting them across repos lets a server change
silently break the client, and has already produced drift (the
`/mcp`-vs-tool-based-auth gotcha; tool-name assumptions).

This design relocates the client bundle into engram so server and client live
in one repo and version in lockstep under engram's existing `cog` tag-only
release flow. The move also adopts Claude Code's **bundled skill-plugin** model
(`.claude-plugin/plugin.json` inside a skill folder), which is a better fit than
the marketplace-only pattern: `memory-curator` *is* "skills that need an MCP
server and hooks", exactly the shape that model was built for.

The plugin is simultaneously **rebranded** to `engram` — plugin name, MCP server
id, and tool prefixes all become `engram`, so the whole memory system shares one
name.

### Decisions (from brainstorming, recorded on engram-w8b)

1. **Rebrand to `engram`** — plugin name, the MCP server id (`memory_oauth` →
   `engram`), and every tool reference (`mcp__memory_oauth__*` →
   `mcp__engram__*`).
2. **Bundled skill-plugin layout** under a new top-level `skill/` directory:
   `skill/engram/` is the single source tree.
3. **Two consumption paths from one source** — a repo-root
   `.claude-plugin/marketplace.json` for `/plugin install engram`, and a symlink
   into `~/.claude/skills/engram` for zero-install personal use.
4. **Claude-only** — no Codex `.agents/` wrapper for now. engram's existing
   `.agents/skills/beads` and `.codex/` are out of scope and untouched.
5. **Clean cutover via two sequenced PRs** — engram import first, then removal
   from `fzymgc-house-skills`. Design docs/ADRs are **left behind** in
   `fzymgc-house-skills` as historical record; this fresh spec is the only
   migration doc.

## Goals / Non-goals

## Goals

- Co-locate the client bundle with the server it drives, versioned in lockstep.
- Adopt the bundled skill-plugin model with both install paths working.
- Complete, verifiable rebrand to `engram` with zero residual `memory_oauth`.
- Grow engram CI to gate the bundle (Python hooks + markdown) under Apache-2.0.

## Non-goals

- No Codex / cross-platform wrapper (deferred).
- No change to the engram server, store schema, or scope semantics.
- No migration of stored memories (server-side, scope-keyed, unaffected).
- No ADR/spec history migration; `fzymgc-house-skills` docs stay put.

## Grounding provenance

This spec describes the source bundle as it exists on **`fzymgc-house-skills`
`origin/main` at PR #135 / tag `v1.20.1`**: hooks are `SessionStart` +
`PostToolUse` (`session-start-memory-recall`, `posttooluse-memory-capture-nudge`),
**not** the older `Stop`/`session-end-memory-capture` pair. A primary checkout
left parented before #135 shows stale files; ground any inspection against
`main`/`origin/main` (e.g. `jj file show -r main …`), not the working copy.

## Source layout (`engram/skill/engram/`)

```text
engram/
├── skill/
│   └── engram/                          # bundled skill-plugin → engram@skills-dir
│       ├── .claude-plugin/
│       │   └── plugin.json              # name "engram"; hooks + mcpServers paths
│       ├── skills/
│       │   ├── curating-memory/SKILL.md
│       │   └── promoting-memory/SKILL.md
│       ├── hooks/
│       │   ├── hooks.json               # SessionStart + PostToolUse
│       │   ├── session-start-memory-recall
│       │   ├── posttooluse-memory-capture-nudge
│       │   ├── lib/scope.py
│       │   └── tests/                   # pytest suite, moved intact
│       └── .mcp.json                    # server id "engram", url …/mcp/engram
├── .claude-plugin/
│   └── marketplace.json                 # NEW: plugin "engram", source ./skill/engram
└── docs/superpowers/specs/2026-06-02-relocate-memory-curator-into-engram-design.md
```

The `skill/engram/` folder carries `.claude-plugin/plugin.json` and bundles both
skills under `skills/`. Per the Claude Code plugin reference, a folder with
`.claude-plugin/plugin.json` is a plugin that bundles its own skills/hooks/MCP —
it does **not** require a top-level `SKILL.md`.

## Consumption

| Path | How | Notes |
|------|-----|-------|
| Marketplace | `/plugin marketplace add seanb4t/engram` → `/plugin install engram` | Versioned with engram's git tags. |
| Skills-dir | `ln -s <clone>/skill/engram ~/.claude/skills/engram` | Loads as `engram@skills-dir`; tracks HEAD; zero-install. |

The plugin's loaded identity differs by path — `engram` via marketplace install,
`engram@skills-dir` via the symlink — but both namespace the bundled skills the
same way: `engram:curating-memory` and `engram:promoting-memory`.

**Operational caveat:** the two paths must not be active on the same machine at
once — identical plugin/skill ids would collide. This is documented in the
plugin README; not enforced in code.

## The rebrand (mechanical rename)

A single, total rename pass with a completeness check:

- `.mcp.json`: server key `memory_oauth` → `engram`; `url`
  `…/mcp/memory_oauth` → `…/mcp/engram`.
- Both `SKILL.md` files and both hook scripts: every `mcp__memory_oauth__*`
  reference → `mcp__engram__*`; prose references to the server name updated.
- Hook tests updated to the new ids.
- **Completeness gate:** a test (and a CI grep) asserting **zero** residual
  `memory_oauth` / `mcp__memory_oauth__` strings anywhere under `skill/engram/`.

### External prerequisite (flagged, out of this repo)

The live endpoint path `https://litellm.fzymgc.house/mcp/memory_oauth` is a
**server-side litellm route**. Renaming the client to `…/mcp/engram` breaks auth
unless the litellm gateway route is renamed or aliased to `engram` **before**
PR-1 ships. This is the only cross-system coupling and is a hard prerequisite,
not an in-repo change. If the route cannot be renamed in time, the `.mcp.json`
`url` stays `…/mcp/memory_oauth` while the client-side server id still becomes
`engram` (the id and the route path are independent); this fallback is noted so
the rebrand is not blocked on infra.

## CI, licensing, and tooling (engram)

engram's CI is Go-only today (`go test`, `golangci-lint`, `skywalking-eyes`
license headers, `helm lint`), driven partly through `Taskfile.yaml`; releases
via `goreleaser` + chart push under `cog`.

- **New Python lane** in `ci.yaml` + `Taskfile.yaml` (`lint`/`test`/`fmt`):
  `uv`-run `ruff check`, `ruff format --check`, and `pytest` over
  `skill/engram/hooks/tests/`, plus `rumdl` over the bundle's markdown — mirroring
  the gate style proven in `fzymgc-house-skills`.
- **Apache-2.0 + SPDX headers** on the moved hooks and functional markdown.
  `skywalking-eyes` enforces this; `.licenserc.yaml` is extended to cover
  `skill/**`. (The source plugin shipped UNLICENSED; relicensing to engram's
  Apache-2.0 is part of the move.) **Note on the hook entrypoints:** the three
  hooks (`session-start-memory-recall`, `posttooluse-memory-capture-nudge`) are
  **extensionless** `uv` scripts, not `*.py` files — only `lib/scope.py` and the
  `tests/*.py` carry the `.py` extension. `skywalking-eyes`' extension-based
  matching will not catch the extensionless scripts by default, so the
  `.licenserc.yaml` update must match them explicitly (by `skill/**` path with an
  appropriate hash-comment style) and each script gets its SPDX header in the
  leading comment block beneath the shebang/`uv` preamble. Verify license-eye is
  green on the extensionless scripts as part of PR-1, not just the `.py` files.
- The new `marketplace.json` and `plugin.json` are added to engram's JSON
  validation gate.

## Cutover plan (two PRs, sequenced)

**PR-1 — engram (import + stand-up).** Create `skill/engram/` with the rebranded
bundle, `.claude-plugin/marketplace.json`, the CI Python lane, SPDX headers, and
the spec. Acceptance: green CI; `/plugin install engram` and the
`~/.claude/skills` symlink both load the skills + register the `engram` MCP
server; a live `search_memory`/`store_memory` round-trip succeeds against the
`engram` route — **or**, if the litellm fallback is active (route not yet
renamed), against the existing `…/mcp/memory_oauth` route. The round-trip target
follows whichever route the shipped `.mcp.json` points at; PR-1 is not blocked on
the gateway rename.

**PR-2 — fzymgc-house-skills (removal + redirect).** Remove `memory-curator/`,
`plugins/memory-curator/` (the existing Codex wrapper), both marketplace entries,
and the `memory-curator` Taskfile gates. Add a README/CHANGELOG redirect to
engram. **Leave** `docs/adr/*memory*`, `docs/adr/fhsk-p07*`, and the
`docs/superpowers/specs|plans/*memory-curator*` files in place as historical
record. Sequenced only after PR-1 is verified.

## Testing

- The existing pytest suite moves intact: `test_scope`,
  `test_session_start_memory_recall`, `test_posttooluse_memory_capture_nudge`,
  `test_plugin_config`.
- `test_plugin_config` updated to assert the new `engram` ids/paths and
  `marketplace.json` `source: ./skill/engram`.
- New rebrand-completeness test (no residual `memory_oauth`).
- Manual gate in PR-1: install via both paths; real MCP round-trip.

## Risks

- **litellm route rename** (covered above) — the one external dependency; has a
  documented fallback so the rebrand is not blocked.
- **Dual-path id collision** — documented operational caveat.
- **Skills-dir project-scope nuance** — `@skills-dir` plugins load only from the
  `.claude/skills` of the launch directory and do not walk up to the repo root;
  irrelevant to the symlink-into-`~/.claude/skills` path but worth noting for
  anyone using engram's own `.claude/skills/` for project-scope dev.
