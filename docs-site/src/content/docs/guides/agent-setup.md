---
title: Agent Setup
description: Preview MCP registration, choose runtimes and authentication, and install engram curation skills.
---

Connect your agent to a running engram server with `engram setup`.

:::caution[Setup is unreleased]
As of September 12, 2026, **v0.15.1 does not include `engram setup`**.
These instructions describe unreleased source. Follow the
[local-source build instructions](/guides/install/#build-unreleased-setup-from-source)
before running them, and use that executable. Installing the published binary
alone does not enable setup.
:::

## Before you start

Obtain the **full MCP endpoint** from your server operator. Supply it through
`--url` or `ENGRAM_URL`; setup uses it verbatim without adding or removing a path.
A direct deployment usually uses `https://engram.example.com/mcp`; a gateway
may use a route such as `https://gateway.example.com/mcp/engram`. If the server
sets `ENGRAM_MCP_PATH=/`, use its root endpoint instead.

Install your agent runtime and make its binary available on `PATH`.
Setup detects Claude Code through `claude`, Codex through `codex`, and opencode
through `opencode`. A leftover configuration directory does not count as an
installed runtime. By default, setup considers all three native runtimes and
reports which are absent. The `generic` target is always opt-in.

## Preview, review, then apply

Replace the example URL with your endpoint. This OAuth example previews setup
across detected runtimes:

```sh
engram setup --url https://engram.example.com/mcp --auth oauth
```

Preview writes no registrations or skills. It runs read probes through each
present runtime: Claude Code and opencode may contact the endpoint; Codex reads
local registration state. Preview is not an offline-only operation or proof of
a successful connection.

Inspect **every row**, including absent, unsupported, and failed results, and
both the registration and skills details. When the proposed changes match your
intent, append `--apply` to the same invocation with the same inputs:

```sh
engram setup --url https://engram.example.com/mcp --auth oauth --apply
```

Inspect every apply result too. Registration and OAuth login are separate steps:
complete the runtime's OAuth login after successful registration. In Claude Code,
run `/mcp`, select `engram`, and authenticate in the browser. Then use the agent
to [store and recall your first memory](/guides/quickstart/#store-and-recall-your-first-memory).

### Choose specific runtimes

Use `--runtime` to limit the preview and subsequent apply. The flag accepts a
comma-separated list or repeated values:

```sh
engram setup --url https://engram.example.com/mcp --auth oauth --runtime claude-code,codex
```

Use `engram setup --help` for the current flag list and exit behavior.
