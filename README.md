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

## Documentation

Full documentation — quickstart, deploy, configuration, the MCP tool
contract, the memory-record and auth/isolation model, and contributor
guides — lives at **<https://engram-docs.workers.dev>** (source under
[`docs-site/`](./docs-site)).

| Topic | Link |
|-------|------|
| Quickstart | <https://engram-docs.workers.dev/guides/quickstart/> |
| Deploy (Helm / Docker) | <https://engram-docs.workers.dev/guides/deploy/> |
| Configuration (`MEM_*`) | <https://engram-docs.workers.dev/guides/configure/> |
| MCP tools | <https://engram-docs.workers.dev/reference/tools/> |
| Auth & isolation | <https://engram-docs.workers.dev/reference/auth/> |

## License

[Apache License 2.0](./LICENSE).
