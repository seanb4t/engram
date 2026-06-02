<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# engram

Self-hosted, correctable, OAuth-secured **memory for coding agents**, exposed over
the [Model Context Protocol](https://modelcontextprotocol.io) (MCP).

`engram` is a small Go server that stores deliberately-chosen, durable facts
(decisions, preferences, conventions, gotchas) as vectors in
[Qdrant](https://qdrant.tech), embeds them with a configurable model via an
OpenAI-compatible endpoint, and serves them to agents (e.g. Claude Code) over
streamable-HTTP MCP. Every memory is a single **engram** — editable and
deletable, so the store stays correct over time rather than accreting junk.

## Why

LLM agents lose context between sessions. A memory layer fixes that — but an
_auto-extracting_ one fills up with transient noise. `engram` is **explicit and
zero-junk by construction**: the agent stores only what's worth keeping, can
search before storing to avoid duplicates, and supersedes stale facts on
contradiction. Writes are attributed to the verified caller, so you always know
_who_ recorded a memory.

## Tools

| Tool | Purpose |
|------|---------|
| `store_memory(content, scope, source, category, tags?, …)` | Persist a durable memory |
| `search_memory(query, scope, k?)` | Semantic search within a scope |
| `list_memory(scope, limit?)` | Most-recent memories in a scope (session bootstrap, no query) |
| `get_memory(id)` | Fetch one memory |
| `update_memory(id, content)` | Replace content in place (re-embeds) |
| `delete_memory(id)` / `delete_all(scope)` | Correct / tear down |

A memory record carries `content`, `scope`, `repo`/`workspace`/`worktree_path`/
`base_dir`, `source` (`user-said` | `agent-inferred`), `category`, `tags`,
`actor` (the verified caller identity — server-set, never client-supplied), and
`created_at`.

## Authentication

`engram` validates an OIDC bearer token on every request when `--oidc-issuer`
(env `MEM_OIDC_ISSUER`) is set: signature (via the issuer's JWKS), issuer, and
expiry are enforced; an optional audience pin is available. The verified caller
identity (email → username → subject) is extracted from the token and recorded
as the memory's `actor`. With no issuer configured, validation is disabled and
every request is accepted — logged loudly so it is never silently open. This is
designed to sit behind a gateway (e.g. LiteLLM) that forwards the user's token.

## Run

```sh
engram serve --listen-addr :8080 \
  --oidc-issuer https://idp.example/application/o/engram/
```

All flags default from `MEM_*` environment variables (so a container can be
configured purely by env, with flags as overrides):

| Flag | Env | Default |
|------|-----|---------|
| `--listen-addr` | `MEM_LISTEN_ADDR` | `:8080` |
| `--oidc-issuer` | `MEM_OIDC_ISSUER` | _(unset → auth disabled)_ |
| `--oidc-audience` | `MEM_OIDC_AUDIENCE` | _(unset → audience not checked)_ |
| `--oidc-resource-metadata` | `MEM_OIDC_RESOURCE_METADATA` | _(unset)_ |

Storage/embedding are configured via `MEM_QDRANT_ADDR`, `MEM_QDRANT_COLLECTION`,
`MEM_LITELLM_URL`, `MEM_LITELLM_KEY`, `MEM_EMBED_MODEL`, `MEM_EMBED_DIM`.

## Deploy (Helm)

```sh
helm install engram oci://ghcr.io/seanb4t/charts/engram --version <X.Y.Z> \
  --values your-values.yaml
```

The chart deploys the server plus a Qdrant instance with a persistent volume.

## Develop

```sh
task           # lint + test
task build     # build ./cmd/engram → bin/engram
task lint      # golangci-lint + yamlfmt + actionlint + rumdl
task fmt       # gofmt + dprint + yamlfmt
```

## License

[Apache License 2.0](./LICENSE).
