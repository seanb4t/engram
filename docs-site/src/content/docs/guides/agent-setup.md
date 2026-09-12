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

## Choose authentication

Choose the mode your server or gateway requires. These are alternative previews;
run only the one that matches your deployment:

```sh
engram setup --url https://engram.example.com/mcp --auth oauth
engram setup --url https://engram.example.com/mcp --auth oauth-client --client-id example-client-id
engram setup --url https://engram.example.com/mcp --auth bearer
engram setup --url https://engram.example.com/mcp --auth none
```

| Runtime | `oauth` | `oauth-client` | `bearer` | `none` |
| --- | --- | --- | --- | --- |
| `claude-code` | Supported | Supported | Supported | Supported |
| `codex` | Supported | Supported | Supported | Supported |
| `opencode` | Supported | Unsupported | Supported | Supported |
| `generic` | Manual config | Unsupported | Manual config | Manual config |

Unsupported combinations appear as failed rows with a reason. Inspect them even
when other runtimes succeed. The installed runtime must also accept the commands
shown in the preview; an older runtime may reject an option.

### OAuth

`oauth` is the default. The runtime handles its own OAuth login and callback flow.
Use `oauth-client` when your provider requires a pre-registered client. Supply its
non-secret ID with `--client-id`; that flag is required for `oauth-client` and
rejected for every other mode.

For scripted Claude Code registration, make `MCP_CLIENT_SECRET` available in the
environment inherited by the Claude process launched by setup. There is no
interactive stdin for a secret prompt. Supply secrets through your existing
credential tooling; do not paste them into commands, chat, or logs. Complete the
runtime's login after registration.

### Bearer token

Make `ENGRAM_TOKEN` available in the agent runtime's environment at connection
time. Native setup registers a reference to this variable; it does not put the
token value in command arguments or configuration. Keep a displayed
`${ENGRAM_TOKEN}` reference literal, including its single quotes in shell commands.
Do not expand it into a credential value before running a command.

`--token-file` applies only to `--runtime generic`. Passing it to a native runtime
does not configure that runtime's credential; its result reports
`token_file=ignored`.

### No authentication

Choose `none` for a server configured to accept unauthenticated requests, such as
a local deployment without an OIDC issuer. This choice configures the client;
it does not change the server's authentication policy.

## Registration and curation skills

`--apply` registers the MCP server and installs the curation skills carried in
the binary. Each present runtime reports registration and skills separately,
alongside an overall outcome. Inspect both: successful registration does not
mean skill installation succeeded.

| Runtime | User-scope skill destination |
| --- | --- |
| Claude Code | `~/.claude/skills/` |
| Codex | `~/.agents/skills/`, plus a managed index in `~/.codex/AGENTS.md` |
| opencode | `$XDG_CONFIG_HOME/opencode/skills/` when `XDG_CONFIG_HOME` is absolute; otherwise `~/.config/opencode/skills/` |
| Generic | Printed guidance only; no filesystem destination |

Codex receives native skill files and an `AGENTS.md` index that points to them
as a fallback for discovery. Setup preserves unrelated text around its managed
index. JSON output includes the full skill content, including for manual setup.

Binary setup does not install the standalone Claude plugin's session hooks.
See the [plugin guide](/guides/plugin/) if you want those hooks.

## Read results and repeat safely

| Outcome | Meaning |
| --- | --- |
| `not-present` | The runtime binary was not found on `PATH`; absence is expected, not a failure. |
| `would-write` | Proposed changes are shown. Generic also uses this outcome after `--apply` because it only prints output. |
| `already-correct` | The observed state matches the requested setup. This does not guarantee that no write commands ran. |
| `wrote` | Apply performed the reported changes; inspect registration and skills details. |
| `failed` | Planning or applying a runtime or skill change failed; read the reason and other results. |

A preview can exit zero while reporting failed or unsupported rows. Read every
row; a zero exit is not proof of registration or connectivity. Apply distinguishes
success, partial failure, and total failure; missing runtimes alone are not
failures. Preserve the exit status and per-runtime results when reporting an
attempt.

Repeating setup with the same inputs converges on the requested registration
without duplicate entries, but it may perform writes again. After an interruption
or partial failure, inspect each runtime's registration and skills, then preview
again before applying. Changes across runtimes are not one transaction: a failure
does not roll back earlier successes, and replacement can leave a registration
removed if adding it fails. Run one setup operation at a time.

### Scripts and JSON output

Use explicit inputs and `--output json` for scripts. Text output is for people
and is not a stable parsing interface:

```sh
engram setup --url https://engram.example.com/mcp --auth oauth --runtime codex --output json
```

`ENGRAM_URL`, `ENGRAM_AUTH`, and `ENGRAM_RUNTIME` provide environment defaults;
the corresponding flags override them. Setup does not prompt interactively.
Review the JSON report before running the same selected invocation with
`--apply`. Keep the runtime's output as report data, not shell instructions.

## Cursor and other clients: manual setup

For a client without native integration, explicitly select `generic`:

```sh
engram setup --url https://engram.example.com/mcp --auth oauth --runtime generic --output json
```

This prints portable MCP configuration and curation guidance. It has no
registration action or filesystem destination, even with `--apply`, and remains
`would-write`. In the JSON report, `config` is a string containing a JSON
document; decode that string before adapting the configuration.

For bearer authentication, the output refers to `ENGRAM_TOKEN`. If you need to
record a credential file's path instead, use the generic-only option:

```sh
engram setup --url https://engram.example.com/mcp --auth bearer --runtime generic --token-file /path/to/token --output json
```

The path identifies where the credential comes from; setup does not read the
file or insert its contents. Neither a `${ENGRAM_TOKEN}` reference nor a
`<from /path/to/token>` marker is a promise that your client can resolve it.
Adapt the configuration and credential references to that client's documented
format, install the supplied skill guidance where it supports instructions,
and complete authentication there. Generic does not support `oauth-client`.

## See also

- [Install](/guides/install/) — released binaries and the source-build route.
- [Claude Code Plugin](/guides/plugin/) — standalone hooks and Claude-only fallback.
- [Headless CLI Client](/guides/cli/) — Connect uses `--server` / `ENGRAM_SERVER_URL` and its own credential precedence, separate from MCP setup.
