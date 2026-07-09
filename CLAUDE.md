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
| `cmd/engram/` | cobra CLI: `root`, `serve`, `version` + operator commands (`reindex` embedder migration — see docs-site `guides/reindex`; `migrate-remap-owner`; `prune-expired`; `summarize-missing`; `backfill-short-ids`) (entrypoint only) |
| `internal/server/` | MCP tool registration + handlers (`Register`, `EnvOr`) |
| `internal/store/` | Qdrant-backed memory store |
| `internal/embed/` | embedder (OpenAI-compatible) |
| `internal/auth/` | OIDC bearer-token verifier (go-oidc + go-sdk auth middleware) |
| `internal/config/` | koanf config loader + field registry (single source of truth for ENGRAM_ vars) |
| `charts/engram/` | Helm chart (server + Qdrant), generic/parameterized |
| `proto/engram/v1/` | protobuf schema (`EngramService` v1 read API) — source of truth for codegen |
| `gen/` | committed buf-generated code (connect-go stubs in `gen/go/`, protobuf-es types in `gen/ts/`) |

## Conventions

- **VCS:** git. Branch + PR; never push to `main` directly (protect-main ruleset). Planning/workflow via GSD (`.planning/`, `/gsd-*`).
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
`summary`/`summary_source` (client-authored or auto-generated digest; omit for none),
`actor` (verified caller — server-set, never client-supplied), `owner` (caller's
configured owner-claim value — `ENGRAM_OWNER_CLAIM`, default `email`, the authz
key — server-set), `visibility` (`private` default |
`shared`), `created_at`, and a server-minted `short_id` (10-char Crockford
base32 handle, accepted anywhere an id is accepted; legacy records gain one via
`engram backfill-short-ids`). Recall returns summaries by default with `full=true` opt-in;
full content via `get_memory`. Design intent: explicit, zero-junk, correctable. Do not
add auto-extraction.

**Isolation (authz):** each actor sees/mutates only their own records; `shared`
records are readable (never writable) by any **authenticated** caller — the
shared read grant requires a non-empty owner-claim value. No issuer → single anonymous
bucket (`owner==""`); anonymous callers (auth disabled) see only that bucket and
cannot read other actors' `shared` records. The `set_visibility` tool and
`update_memory`'s `shared` field toggle sharing; `update_memory`'s `tags` field
replaces the tag set (omit to preserve, empty array to clear). `search_memory`
and `list_memory` accept an optional `tags` filter — records must carry **all**
listed tags (AND); on `search_memory` it is a hard pre-filter applied before
vector ranking. `search_memory` / `list_memory` / `list_scheduled` also accept
optional `created_after` / `created_before` (RFC3339, half-open `[after, before)`)
to window recall by creation time; `list_memory` paginates via an opaque `cursor`
arg and returns `{memories, next_cursor}` (empty `next_cursor` = last page).
Pre-isolation records (missing
`owner` key) are invisible to every read until you backfill them with `engram
migrate-remap-owner --from-missing --to <owner>` (the `migrate-set-owner` command
is a deprecated alias). To re-stamp records after an IdP `sub`/claim change, use
`engram migrate-remap-owner --from <old> --to <new>`.

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

Rule tools: `store_rule` / `list_rules`. A rule is a 6th `category`: normative,
user-blessed, always-shared ground truth in a dedicated `rule:repo:*` /
`rule:project:*` scope. `store_rule` is invoked only on explicit user
instruction (never promoted unilaterally); its `summary` must be a single line
(the index entry). `list_rules` returns the complete set for one or more
`rule:*` scopes, oldest-first, compact index shape by default (`full` for
content). Rules surface at session start as a progressive-disclosure index (one
line per rule; full text fetched on demand via `get_memory`). `set_visibility`
is rejected for rules — delete the rule instead. Design intent unchanged:
explicit, user-blessed, no auto-extraction.

## Auth

`--oidc-issuer`/`ENGRAM_OIDC_ISSUER` enables bearer-token enforcement (JWKS
signature + issuer + expiry; optional audience). The verified identity becomes
the memory `actor`. No issuer → validation disabled (logged loudly).

## Issue Tracking

**GitHub Issues** is the tracker (`gh issue list`, `gh issue create`). Beads was retired
2026-07-08: the full export is archived at `.planning/archive/`, active work was migrated to
GitHub Issues (label `from-beads`), and `.planning/BACKLOG.md` indexes it for GSD milestone
promotion (`/gsd-review-backlog`). Do not use markdown TODO lists for durable tracking.

Durable project memory (decisions, conventions, gotchas) → the **engram** MCP store (see
"Memory contract" above) — not `MEMORY.md` files.

## Session Completion

Subordinate to explicit user, repository, and orchestrator instructions.

1. **File follow-ups** — open GitHub issues for remaining work.
2. **Run quality gates** (if code changed) — `task` (lint + test).
3. **Git policy (conservative default)** — do not commit, push, or open PRs unless asked; at
   handoff report changed files, validation, and the exact proposed commands. Never push to
   `main` — branch + PR (protect-main ruleset).
4. **Hand off** — summarize changes, validation, and any blocked step with its exact command and error.
