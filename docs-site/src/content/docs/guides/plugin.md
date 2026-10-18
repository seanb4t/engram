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

On a Claude Code whose plugin CLI works, `engram setup --apply` installs this plugin for you — engram's own marketplace and plugin (`seanb4t/engram`, `engram@engram`), added when absent, updated when outdated, and left alone when current. See [Agent Setup](/guides/agent-setup/) for installation. The commands below are the by-hand route.

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

:::note[Binary delegation requires engram v0.16.0 or later]
The command delegates whenever it finds `engram` on `PATH`; it does not fall back
automatically when that binary lacks `setup`. Follow [Install](/guides/install/)
to obtain v0.16.0 or later, and check `engram version --output json`. Upgrade
older binaries, including v0.15.1, before using delegation.
:::

:::note[Available since v0.17.0]
Plugin-first installation through `engram setup --apply` and the `preserved`
result described below shipped in v0.17.0 and were verified against the
installed release on 2026-09-18.
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

If `engram setup` reports the registration as `preserved` — it carries something setup did not author and cannot reproduce — setup leaves it untouched and never runs this command for you; the `preserved` row's reason names the exact step. See the [results section](/guides/agent-setup/#read-results-and-repeat-safely) of the setup guide.

## SessionStart hook — memory recall

At the start of every session, the `session-start-memory-recall` hook finds the two-tier memory scope from the local git context. The scope is the spine, plus a workspace overlay in a linked git worktree. Outside a git repository it adds nothing to the session.

In a git repository, the hook takes one of two paths.

**The primer path.** When the `engram` binary is on `PATH`, the hook runs [`engram prime`](/guides/cli/#session-primer-engram-prime) from the session's directory, with a 3-second deadline. prime reads the rules index and the most recent memories with the CLI's own credential. The hook puts that primer in the session context, together with the guidance on store precedence and memory capture. Claude then starts the session with its memory already loaded and does not spend MCP calls on recall.

**The fallback path.** The hook uses the fallback in these cases:

- The binary is not installed.
- prime exits with an error. For example, there is no credential, or the installed binary is too old to have `prime`.
- prime writes nothing.
- prime does not finish within 4 seconds.

The hook then tells Claude to call `list_rules` and `list_memory` over its own OAuth-authenticated MCP connection. A hook cannot hold that OAuth token. If engram returns `401`/`403`, Claude reports once that memories are unavailable. Claude then continues and does not block.

The hook always finishes within its 10-second limit, and it never fails the session start.

To use the primer path:

1. Install the `engram` binary (see [Install](/guides/install/)) so that it is on the `PATH` that Claude Code gives to hooks.
2. Run `engram login --server <url>`. The login saves a credential and records `<url>` as the default server. The hook then needs no other settings. If you use `ENGRAM_TOKEN` or `--token-file` instead of a login, there is no default server. Then set `ENGRAM_SERVER_URL` in the environment that Claude Code runs hooks in.
3. Log in to the CLI as the same user as the MCP connection. If the two logins are different users, the primer shows one user's memories while Claude writes as the other.

`ENGRAM_PROJECT`, when set, adds the `rule:project:<name>` rules on both paths.

## PostToolUse hook — capture nudge

After a mutating tool fires (Edit/Write/NotebookEdit), the `posttooluse-memory-capture-nudge` hook emits a silent `additionalContext` reminder to capture any durable facts established this session. The nudge fires at most once per session (throttled by a `session_id`-keyed marker file in `$TMPDIR`).

Use the `curating-memory` skill to store memories correctly: search before store, supersede on contradiction, default scope to the spine.
