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
| `internal/embed/` | embedder (OpenAI-compatible / LiteLLM) |
| `internal/auth/` | OIDC bearer-token verifier (go-oidc + go-sdk auth middleware) |
| `charts/engram/` | Helm chart (server + Qdrant), generic/parameterized |

## Conventions

- **VCS:** jj-colocated. Use jj for VCS ops; never push to `main` directly.
- **Task runner:** `task` (see `Taskfile.yaml`). `task` = lint + test.
- **CLI:** cobra; **no viper** — config is env-first (`MEM_*`) with flag overrides.
- **Commits:** Conventional Commits; validated in CI on PR titles (cocogitto).
- **License:** every Go/Markdown file carries the Apache-2.0 SPDX header
  (`task license:check`). `task license:add` applies it.
- **Lint/format:** `task lint` (golangci-lint, yamlfmt, actionlint, rumdl) and
  `task fmt` (gofmt, dprint, yamlfmt) must be clean.
- **Releases:** tag `vX.Y.Z` → goreleaser image + OCI Helm chart; version is
  ldflags-injected into `main.version`.
- **Not used here:** protobuf/buf, database migrations, viper.

## Memory contract (stable)

Tools: `store_memory` / `search_memory` / `list_memory` / `get_memory` /
`update_memory` / `delete_memory` / `delete_all`. A record carries `content`,
`scope`, repo/workspace/worktree/base_dir, `source`, `category`, `tags`,
`actor` (verified caller — server-set, never client-supplied), `created_at`.
Design intent: explicit, zero-junk, correctable. Do not add auto-extraction.

## Auth

`--oidc-issuer`/`MEM_OIDC_ISSUER` enables bearer-token enforcement (JWKS
signature + issuer + expiry; optional audience). The verified identity becomes
the memory `actor`. No issuer → validation disabled (logged loudly).
