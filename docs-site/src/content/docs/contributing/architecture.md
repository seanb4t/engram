---
title: Architecture
description: Codebase layout and development conventions for the engram server.
---

engram is a self-hosted, correctable, OAuth-secured memory MCP server written in Go, backed by Qdrant.

## Codebase layout

| Path | Responsibility |
|------|----------------|
| `cmd/engram/` | cobra CLI: `root`, `serve`, `version` (entrypoint only) |
| `internal/config/` | koanf config loader + field registry (single source of truth for ENGRAM_ vars) |
| `internal/server/` | MCP tool registration + handlers (`Register`, etc.) |
| `internal/store/` | Qdrant-backed memory store |
| `internal/embed/` | embedder (OpenAI-compatible) |
| `internal/auth/` | OIDC bearer-token verifier (go-oidc + go-sdk auth middleware) |
| `charts/engram/` | Helm chart (server + Qdrant), generic/parameterized |
| `proto/engram/v1/` | protobuf schema (`EngramService` v1: 5 read + 6 write RPCs) — source of truth for codegen |
| `gen/` | committed buf-generated code (connect-go stubs in `gen/go/`, protobuf-es types in `gen/ts/`) |

## Key conventions

**VCS — git.** Use `git` for all VCS operations; branch and open a PR — never push directly to `main`.

**Task runner.** `task` (see `Taskfile.yaml`) is the single entry point. Running `task` alone runs lint + test. Use `task proto:lint` / `task proto:gen` for protobuf work.

**CLI — cobra, config via internal/config.** Configuration is env-first (`ENGRAM_*` variables) with flag overrides, loaded by the koanf-backed `internal/config` package. There is no viper dependency and no config file format.

**Connect/buf API.** The `EngramService` ConnectRPC API is defined in `proto/engram/v1/` and generated via `go tool buf`. The `gen/` tree is committed and CI-checks for drift — never edit generated files by hand.

**Commits.** Conventional Commits format; PR titles are validated in CI via `action-semantic-pull-request`.

**License.** Every Go and Markdown file carries the Apache-2.0 SPDX header. Run `task license:check` to verify; `task license:add` to apply it to new files.

**Lint/format.** `task lint` runs golangci-lint, yamlfmt, actionlint, and rumdl. `task fmt` runs gofmt, dprint, and yamlfmt. Both must be clean before merging.

**Not used here:** database migrations, viper, cocogitto.
