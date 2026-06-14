<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# CLAUDE.md — engram

AI assistant routing for the `engram` repo: a self-hosted, correctable,
OAuth-secured memory MCP server for coding agents (Go + Qdrant).

## Layout

| Path | Responsibility |
|------|----------------|
| `cmd/engram/` | cobra CLI: `root`, `serve`, `version` (entrypoint only) |
| `internal/server/` | MCP tool registration + handlers (`Register`, `EnvOr`) |
| `internal/store/` | Qdrant-backed memory store |
| `internal/embed/` | embedder (OpenAI-compatible) |
| `internal/auth/` | OIDC bearer-token verifier (go-oidc + go-sdk auth middleware) |
| `internal/config/` | koanf config loader + field registry (single source of truth for ENGRAM_ vars) |
| `charts/engram/` | Helm chart (server + Qdrant), generic/parameterized |
| `proto/engram/v1/` | protobuf schema (`EngramService` v1 read API) — source of truth for codegen |
| `gen/` | committed buf-generated code (connect-go stubs in `gen/go/`, protobuf-es types in `gen/ts/`) |

## Conventions

- **VCS:** jj-colocated. Use jj for VCS ops; never push to `main` directly.
- **Task runner:** `task` (see `Taskfile.yaml`). `task` = lint + test.
- **Protobuf/buf:** the `EngramService` Connect API is defined in `proto/` and
  generated via `go tool buf` (`task proto:lint` / `task proto:gen`); the
  generated `gen/` tree is committed and CI-checked for drift (`buf` job).
- **CLI:** cobra; config is loaded by `internal/config` (koanf): env-first via the `ENGRAM_` prefix with `--flag` overrides; no viper.
- **Commits:** Conventional Commits; PR titles validated in CI
  (action-semantic-pull-request).
- **License:** every Go/Markdown file carries the Apache-2.0 SPDX header
  (`task license:check`). `task license:add` applies it.
- **Lint/format:** `task lint` (golangci-lint, yamlfmt, actionlint, rumdl) and
  `task fmt` (gofmt, dprint, yamlfmt) must be clean.
- **Releases:** release-please-driven (see `RELEASING.md`). Merging the
  release PR cuts the `vX.Y.Z` tag + GitHub Release; the release workflow then
  ships the binary + image (goreleaser) and the OCI Helm chart (`task
  chart:push`). release-please syncs `charts/engram/Chart.yaml`
  (`version`/`appVersion`) and `skill/engram/.claude-plugin/plugin.json`
  (`$.version`); the binary version is ldflags-injected into `main.version`.
- **Not used here:** database migrations, viper, cocogitto.

## Memory contract (stable)

Tools: `store_memory` / `schedule_memory` / `search_memory` / `list_memory` /
`list_scheduled` / `get_memory` / `update_memory` / `delete_memory` /
`delete_all`. A record carries `content`,
`scope`, repo/workspace/worktree/base_dir, `source`, `category`, `tags`,
`actor` (verified caller — server-set, never client-supplied), `owner` (caller's
stable OIDC `sub`, the authz key — server-set), `visibility` (`private` default |
`shared`), `created_at`. Design intent: explicit, zero-junk, correctable. Do not
add auto-extraction.

**Isolation (authz):** each actor sees/mutates only their own records; `shared`
records are readable (never writable) by any **authenticated** caller — the
shared read grant requires a non-empty `sub`. No issuer → single anonymous
bucket (`owner==""`); anonymous callers (auth disabled) see only that bucket and
cannot read other actors' `shared` records. The `set_visibility` tool and
`update_memory`'s `shared` field toggle sharing. Pre-isolation records (missing
`owner` key) are invisible to every read until you backfill them with `engram
migrate-set-owner --owner <sub>`.

Scheduled tools: `schedule_memory` stores a memory with a temporal validity
window — `not_before` (RFC3339; deferred reveal: hidden from recall until then)
and/or `not_after` (RFC3339; expiry: dropped from recall at then). `list_scheduled`
surfaces windowed records the recall gate is hiding (`state` = `scheduled` default
| `expired` | `all`); active windowed records surface normally via
`search_memory`/`list_memory`. Recall is gated; fetch-by-id (`get_memory`) is not.
Operators reclaim lapsed records with `engram prune-expired [--older-than DUR]`.

Discovery tools: `store_discovery` / `search_discovery`. A discovery is a 5th
`category` carrying `kind` (`map`|`fact`), `citations` (with aging `pin`s), and
`summary`; it lives in a separate `discovery:repo:*` scope, is recalled on
demand (never at session start), and is captured via the `discovering` skill.
Design intent unchanged: explicit, citation-backed, no auto-extraction.

## Auth

`--oidc-issuer`/`ENGRAM_OIDC_ISSUER` enables bearer-token enforcement (JWKS
signature + issuer + expiry; optional audience). The verified identity becomes
the memory `actor`. No issuer → validation disabled (logged loudly).

<!-- rumdl-disable MD031 MD032 -->
<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:6cd5cc61 -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/SYNC_CONCEPTS.md for details and anti-patterns.

## Agent Context Profiles

The managed Beads block is task-tracking guidance, not permission to override repository, user, or orchestrator instructions.

- **Conservative (default)**: Use `bd` for task tracking. Do not run git commits, git pushes, or Dolt remote sync unless explicitly asked. At handoff, report changed files, validation, and suggested next commands.
- **Minimal**: Keep tool instruction files as pointers to `bd prime`; use the same conservative git policy unless active instructions say otherwise.
- **Team-maintainer**: Only when the repository explicitly opts in, agents may close beads, run quality gates, commit, and push as part of session close. A current "do not commit" or "do not push" instruction still wins.

## Session Completion

This protocol applies when ending a Beads implementation workflow. It is subordinate to explicit user, repository, and orchestrator instructions.

1. **File issues for remaining work** - Create beads for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Handle git/sync by active profile**:
   ```bash
   # Conservative/minimal/default: report status and proposed commands; wait for approval.
   git status

   # Team-maintainer opt-in only, unless current instructions forbid it:
   git pull --rebase
   git push
   git status
   ```
5. **Hand off** - Summarize changes, validation, issue status, and any blocked sync/commit/push step

**Critical rules:**
- Explicit user or orchestrator instructions override this Beads block.
- Do not commit or push without clear authority from the active profile or the current user request.
- If a required sync or push is blocked, stop and report the exact command and error.
<!-- END BEADS INTEGRATION -->
<!-- rumdl-enable MD031 MD032 -->
