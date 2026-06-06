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
| `store_discovery(content, kind, citations[], scope, …)` | Cache citation-backed codebase understanding (kind=map\|fact) |
| `search_discovery(query, scope?, kind?, cross_spine?)` | On-demand semantic search over the discovery pool |
| `search_memory(query, scope, k?)` | Semantic search within a scope |
| `list_memory(scope, limit?)` | Most-recent memories in a scope (session bootstrap, no query) |
| `get_memory(id)` | Fetch one memory |
| `update_memory(id, content)` | Replace content in place (re-embeds) |
| `delete_memory(id)` / `delete_all(scope)` | Correct / tear down |

A memory record carries `content`, `scope`, `repo`/`workspace`/`worktree_path`/
`base_dir`, `source` (`user-said` | `agent-inferred`), `category`, `tags`,
`actor` (the verified caller identity — server-set, never client-supplied),
`owner` (the caller's stable OIDC `sub`, the authorization key — server-set),
`visibility` (`private` by default, or `shared`), and `created_at`.

**Isolation:** each actor reads and writes only their **own** records; a record
can be marked `shared` (via `set_visibility` or `update_memory`'s `shared` flag)
to make it readable by any authenticated caller — sharing grants read, never
write. Isolation **requires authentication**: with no `--oidc-issuer`, all
callers share one anonymous bucket. The `owner` is the stable `sub`, so a
changed email never revokes access.

**Upgrading an existing deployment:** records written before isolation carry no
`owner` and become **invisible to every read** (and un-clearable by `delete_all`)
once the new binary starts. The server logs a startup warning when such records
exist. Claim them once with `engram migrate-set-owner --owner <sub>` (using the
`sub` you authenticate as); the command is idempotent and a rerun reports `0`.

A **discovery** record (category `discovery`) additionally carries `kind`
(`map` | `fact`), `citations[]` (each `kind`/`ref`/`locator`/`pin`/`excerpt`),
and an optional `summary`; it lives in a `discovery:repo:<repo>` scope and is
recalled on demand, never at session start.

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

## Connect your coding agent

This repo bundles the **`engram` plugin** for Claude Code (`skill/engram/`) — two
skills (`curating-memory`, `promoting-memory`), and session-start recall + a
silent capture nudge. Install it one of two ways:

```sh
# Marketplace (versioned with engram's git tags)
/plugin marketplace add seanb4t/engram
/plugin install engram

# …or zero-install: symlink the bundle into your personal skills dir
ln -s "$PWD/skill/engram" ~/.claude/skills/engram
```

The plugin ships **no** MCP server of its own — point it at **your** engram
server by running **`/engram-setup`**. It interviews the URL and auth mode and
registers a user-scope `engram` server (available in every project) via the
supported `claude mcp add` CLI. Then run `/mcp` to complete OAuth where required.

| Posture | How |
|---------|-----|
| Direct server, OIDC OAuth | server runs with `--oidc-issuer` + `--oidc-resource-metadata`; `/engram-setup` → OAuth, `/mcp` authenticates on first `401` |
| Behind an OAuth gateway | `/engram-setup` → OAuth, point it at the gateway route; `/mcp` authenticates |
| Local / no auth | `/engram-setup` → None, point it at `http://localhost:8080` |
| Bearer token / CI | `/engram-setup` → bearer mode (static `Authorization` header) |

## Develop

```sh
task           # lint + test
task build     # build ./cmd/engram → bin/engram
task lint      # golangci-lint + yamlfmt + actionlint + rumdl
task fmt       # gofmt + dprint + yamlfmt
```

## License

[Apache License 2.0](./LICENSE).
