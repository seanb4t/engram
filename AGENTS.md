# AGENTS.md

This repository's agent/contributor guidance lives in [CLAUDE.md](./CLAUDE.md)
(layout, conventions, the memory contract, and auth). AGENTS.md is intentionally
a thin pointer so there is a single source of truth.

Quick orientation: Go MCP server (`cmd/engram` + `internal/`), Helm chart
(`charts/engram`), Task-based workflow (`task` = lint + test), cobra CLI (no
viper), Conventional Commits, Apache-2.0 SPDX headers on every source file.
