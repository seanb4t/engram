---
title: Claude Code Plugin
description: Install the engram Claude Code plugin and register your self-hosted server via /engram-setup.
---

The engram Claude Code plugin lives in `skill/engram/` and ships:

- A `/engram-setup` command to register your server
- A `SessionStart` hook that surfaces stored memories at the start of every session
- A `PostToolUse` hook that nudges memory capture after you change repo state

**There is no bundled MCP server.** Connect the plugin to your own engram
deployment. [Install](/guides/install/) covers obtaining the binary;
[Agent Setup](/guides/agent-setup/) covers MCP registration and curation skills
across supported runtimes. The standalone plugin adds the hooks described below.

## Install the plugin

```sh
claude plugin marketplace add seanb4t/engram
claude plugin install engram@engram
```

Or, if you have a local clone, add its repository root (which contains the
marketplace manifest):

```sh
claude plugin marketplace add /path/to/engram
claude plugin install engram@engram
```

## Register your server with /engram-setup

Run `/engram-setup` in Claude Code (with an optional URL argument):

```text
/engram-setup https://engram.example.com/mcp
```

Supply the complete MCP endpoint, including its configured path. The command
gathers the endpoint and one of four auth choices: OAuth, pre-registered OAuth
client, bearer token, or none.

:::caution[Binary delegation requires unreleased setup]
As of September 12, 2026, the published **v0.15.1 binary lacks `setup`**.
The current source command delegates whenever it finds `engram` on `PATH`; it
does not fall back automatically when that binary lacks the command. Installing
v0.15.1 through Homebrew does not enable delegation. To use it, follow the
[source-build route](/guides/install/#build-unreleased-setup-from-source) and make
that executable available on `PATH`.
:::

With a setup-capable binary present, `/engram-setup` previews across detected
runtimes, shows every registration and skills result, and asks for confirmation
before applying the same inputs. It reports failures without automatically
retrying or switching to fallback. See [Agent Setup](/guides/agent-setup/) for
runtime support and credential requirements.

With **no `engram` binary on `PATH`**, the standalone command retains its
Claude-only fallback through Claude's own CLI. That registers a user-scope server
available in every project. It does not perform the binary's cross-runtime skill
installation. Exact native commands and the fallback procedure live in the
[generated setup command reference](https://github.com/seanb4t/engram/blob/main/skill/engram/commands/engram-setup.md#generated-command-reference).

For a pre-registered OAuth client, provide the non-secret client ID and make
`MCP_CLIENT_SECRET` available in the scripted Claude process's inherited
environment; there is no interactive stdin. For bearer mode, supply `ENGRAM_TOKEN`
in the runtime environment and keep command references literal. Do not paste
secret values into commands or chat. Native registration does not use
`--token-file`.

After successful OAuth registration, run `/mcp`, select `engram`, and authenticate
in the browser. Registration alone does not complete that login.

To change the URL later on the standalone fallback path:

```sh
claude mcp remove engram --scope user
# then re-run /engram-setup
```

## SessionStart hook — memory recall

At the start of every session the `session-start-memory-recall` hook computes the two-tier memory scope (spine + optional workspace overlay) from the local git context and injects an instruction for Claude to call `list_memory` over its own OAuth-authenticated MCP connection. Recall is model-mediated because hooks cannot hold OAuth tokens.

If engram returns `401`/`403`, Claude reports once that memories are unavailable and continues — it does not block.

## PostToolUse hook — capture nudge

After a mutating tool fires (Edit/Write/NotebookEdit), the `posttooluse-memory-capture-nudge` hook emits a silent `additionalContext` reminder to capture any durable facts established this session. The nudge fires at most once per session (throttled by a `session_id`-keyed marker file in `$TMPDIR`).

Use the `curating-memory` skill to store memories correctly: search before store, supersede on contradiction, default scope to the spine.
