<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-50b; do not edit manually; use `/adr update engram-50b` -->

# engram plugin ships no bundled MCP server; /engram-setup is the sole registration path

**Date:** 2026-06-03
**Status:** Accepted
**Decision:** engram-50b
**Deciders:** Sean Brandt

## Context

The skill/engram Claude Code plugin bundled a `.mcp.json` declaring an `engram` MCP server at `${ENGRAM_MCP_URL:-http://localhost:8080}`. A plugin `.mcp.json` is static — it cannot prompt for a URL or auth mode, so it must bake in a default. engram's deployment posture is self-hosted, OAuth-gated, and usually behind a gateway, so the localhost default is wrong for most deployers. In `/mcp` it produced a permanent, confusing `plugin:engram:engram · failed / needs-auth` entry, while being strictly redundant with the user-scope server `/engram-setup` already registers — and user scope ALWAYS outranks a plugin-namespaced definition, so the bundle never actually won name resolution.

## Decision

Ship no bundled MCP server. `/engram-setup` is the single canonical registration path; it runs `claude mcp add --transport http engram <url> --scope user [...]` to write a USER-SCOPE server (a plugin command cannot programmatically write MCP config itself, hence the generator command). Concretely: delete `skill/engram/.mcp.json`, drop the `mcpServers` key from `plugin.json`, and update the validating tests, README, and the `/engram-setup` prose. Implemented in PR #16, released in v0.3.0.

## Rationale

Removes a permanently-failing, confusing duplicate entry; gives one obvious setup path that adapts to any URL + auth mode (OAuth, pre-registered client, bearer, none); and eliminates the fragile `${VAR:-default}` env-interpolation contract. Because user scope always outranks a plugin def, the bundle provided no functional value it did not already get from `/engram-setup`.

## Alternatives Considered

(1) Keep the bundled `.mcp.json` as-is — rejected: a permanent failed entry for the common remote/gateway deployment. (2) Keep the def but drop the localhost fallback (`${ENGRAM_MCP_URL}` with no default) — rejected: an unset/empty var resolves to a malformed URL that fails worse, since Claude Code does not reliably treat an empty URL as "disabled". (3) Generalize the bundled `.mcp.json` via env interpolation (the prior 2026-06-02-generalize-engram-client-config design) — superseded: it still ships a wrong-for-most default and the failed entry.

## Consequences

No more confusing failed/duplicate entry; a single obvious setup path. No zero-config localhost path — even local users run `/engram-setup` (pointing it at `http://localhost:8080`). A marketplace copy installed before v0.3.0 keeps its stale `.mcp.json` until refreshed. Supersedes the `2026-06-02-generalize-engram-client-config` design; those specs/plans remain as dated historical records.
