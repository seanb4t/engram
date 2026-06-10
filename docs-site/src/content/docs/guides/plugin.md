---
title: Claude Code Plugin
description: Install the engram Claude Code plugin and register your self-hosted server via /engram-setup.
---

The engram Claude Code plugin lives in `skill/engram/` and ships:

- A `/engram-setup` command to register your server
- A `SessionStart` hook that surfaces stored memories at the start of every session
- A `PostToolUse` hook that nudges memory capture after you change repo state

**There is no bundled MCP server.** engram is self-hosted and OAuth-gated; the plugin ships no connection definition. `/engram-setup` is the only registration path.

## Install the plugin

```sh
claude plugin install https://github.com/seanb4t/engram
```

Or, if you have a local clone:

```sh
claude plugin install /path/to/engram/skill/engram
```

## Register your server with /engram-setup

Run `/engram-setup` in Claude Code (with an optional URL argument):

```
/engram-setup https://engram.example.com
```

The command determines your server URL and auth mode, then runs the matching `claude mcp add` invocation. For example, for OAuth (direct OIDC) or no-auth:

```sh
claude mcp add --transport http engram <url> --scope user
```

This writes a **user-scope** server entry (available in every project). The `--scope user` flag is intentional: a self-hosted, OAuth-gated server outranks any per-project `.mcp.json` definition.

### Auth modes

`/engram-setup` supports four modes:

| Mode | Command run |
|------|------------|
| OAuth (direct or gateway-fronted) | `claude mcp add --transport http engram <url> --scope user` |
| Pre-registered OAuth client | `claude mcp add --transport http engram <url> --scope user --client-id <id> --client-secret --callback-port 8765` |
| Bearer token | `claude mcp add --transport http engram <url> --scope user --header "Authorization: Bearer <token>"` |
| None (local / no-auth) | `claude mcp add --transport http engram <url> --scope user` |

After running the command, complete the OAuth flow via `/mcp` (select `engram` → Authenticate) if using OAuth.

To change the URL later:

```sh
claude mcp remove engram --scope user
# then re-run /engram-setup
```

## SessionStart hook — memory recall

At the start of every session the `session-start-memory-recall` hook computes the two-tier memory scope (spine + optional workspace overlay) from the local git/jj context and injects an instruction for Claude to call `list_memory` over its own OAuth-authenticated MCP connection. Recall is model-mediated because hooks cannot hold OAuth tokens.

If engram returns `401`/`403`, Claude reports once that memories are unavailable and continues — it does not block.

## PostToolUse hook — capture nudge

After a mutating tool fires (Edit/Write/NotebookEdit), the `posttooluse-memory-capture-nudge` hook emits a silent `additionalContext` reminder to capture any durable facts established this session. The nudge fires at most once per session (throttled by a `session_id`-keyed marker file in `$TMPDIR`).

Use the `curating-memory` skill to store memories correctly: search before store, supersede on contradiction, default scope to the spine.
