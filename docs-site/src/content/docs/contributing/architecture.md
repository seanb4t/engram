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

## Pinned interface surfaces

Six surfaces state the same conditional-requirement rules today: cobra `Usage` strings, the MCP jsonschema struct tags, the MCP tool `Description` prose, the proto field comments, `docs-site/`, and `skill/engram/`. For each rule, exactly one source exists — the declared value in `internal/surfaces` (id, canonical sentence, bound fields) — and everything else is either generated from it directly or composed from it in Go source. Inside a generated region, marked by an `engram:rule:start` / `engram:rule:end` anchor pair, the text is regenerated on every run and any hand-edit is silently reverted; authors reword freely everywhere OUTSIDE the anchors.

Two more artifacts are pinned the same way, from the same registry: every command's `--help` output (`cmd/engram/testdata/help.golden`, walked from the live cobra tree) and the bare `engram` catalog JSON (`cmd/engram/testdata/catalog.golden`), which now includes each command's `blast_radius` classification (see [the CLI guide](/guides/cli/#blast-radius) for its shape).

**One command regenerates every pinned artifact:** `task surfaces:gen` — the anchored rule regions, the `--help` golden, and the catalog-JSON golden, in one run. One CI job (`surfaces`) re-runs the same regeneration in a throwaway checkout and fails the build on a dirty tree, mirroring the contract the committed `gen/` tree already lives under (`task proto:gen` + the `buf` drift job).

**What "unreviewed" means here.** `REQ-help-output-pinned` requires `--help` output to change only "reviewed, not unreviewed." This phase interprets that as a mechanical guarantee: CI fails whenever the committed golden does not match what the live tree actually produces, so any help-text change — however small — forces a regeneration commit whose diff shows the exact before/after wording in a normal PR review. CODEOWNERS gating on these paths was considered and rejected: on a single-maintainer repo it adds a required-reviewer rule with nobody else to review, which is process theater rather than an actual second set of eyes. The CI drift check is the real gate.

**Regeneration is never automatic.** `surfaces:gen` is not a dependency of `task default`, `task test`, or `task lint` — running the normal gates never silently rewrites a golden or a generated region. The `surfaces` CI job itself holds no repository write access and never pushes; it only regenerates in its own checkout and diffs against what was committed.
