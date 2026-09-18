---
title: Agent Setup
description: Preview MCP registration, choose runtimes and authentication, and install engram curation skills.
---

Connect your agent to a running engram server with `engram setup`.

:::note[Requires engram v0.16.0 or later]
Follow [Install](/guides/install/) to obtain the released binary, then check
`engram version --output json`. Upgrade older binaries before using setup.
:::

:::note[Available since v0.17.0]
`--header`, plugin-first skill delivery, the `preserved` outcome, the apply
gate described below, and the OAuth re-login note shipped in v0.17.0 and were
verified against the installed release on 2026-09-18.
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
local registration state.

For Claude Code and Codex, preview also compares the existing engram
registration it reads — URL, auth mode, and header names with their
environment-variable references — against what setup would write, and
classifies the row as `already-correct`, `would-write`, or `preserved`.
opencode's registration is not compared: its `mcp list` prints a table setup
does not parse, so a present opencode always reads `would-write`.

`--apply` makes the same comparison before writing: an `already-correct` or
`preserved` row runs no registration command — on Claude Code, not even
`claude mcp remove` — so repeating setup on a converged Claude Code or Codex
installation is a no-op that never touches an existing OAuth login. Only a
`would-write` row is written, then read back.

Preview is not an offline-only operation or proof of a successful connection.

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
If a Claude Code row was rewritten, read its `notes` field first: the [OAuth
section](#oauth) below describes when re-authentication is required.

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

| Runtime | `oauth` | `oauth-client` | `bearer` | `none` | `--header` |
| --- | --- | --- | --- | --- | --- |
| `claude-code` | Supported | Supported | Supported | Supported | Supported |
| `codex` | Supported | Supported | Supported | Supported | Unsupported |
| `opencode` | Supported | Unsupported | Supported | Supported | Supported |
| `generic` | Manual config | Unsupported | Manual config | Manual config | Manual config |

Unsupported combinations — including `--header` on `codex` — appear as failed
rows with a reason. Inspect them even when other runtimes succeed. The
installed runtime must also accept the commands shown in the preview; an
older runtime may reject an option.

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

On Claude Code, a `would-write` row is rewritten with `claude mcp remove` then
`claude mcp add`. If the existing registration carries no `Authorization`
header, setup treats it as OAuth-authenticated and replacing the entry discards that login.

A rewritten row's `notes` field states, in both preview and apply, that this OAuth-authenticated registration means you will need to log in again after `--apply`.

Read the preview's `notes` before applying — `--apply` does not pause for
this, and there is no flag to suppress the rewrite.

### Bearer token

Make `ENGRAM_TOKEN` available in the agent runtime's environment at connection
time. Native setup registers a reference to this variable; it does not put the
token value in command arguments or configuration. Keep a displayed
`${ENGRAM_TOKEN}` reference literal, including its single quotes in shell commands.
Do not expand it into a credential value before running a command.

`--token-file` applies only to `--runtime generic`. Passing it to a native runtime
does not configure that runtime's credential; its result reports
`token_file=ignored`.

### Gateway headers

Some deployments sit behind a gateway that requires an additional header —
for example a gateway's own `x-gateway-api-key`:

```sh
engram setup --url https://engram.example.com/mcp --auth oauth --header x-gateway-api-key=GATEWAY_KEY
```

`--header NAME=ENVVAR` adds a header alongside whatever `--auth` produces, is
repeatable (or comma-separated), and works with every mode. `ENVVAR` is the
NAME of an environment variable the runtime resolves at connection time —
never a value — so it must be a POSIX-shell identifier (ASCII letters, digits,
and underscore, not starting with a digit); any `Authorization` name is also
rejected — that header is owned by `--auth` (use `--auth bearer`).

Each runtime renders the header in its own syntax. The auth header (if any)
renders first, and extra headers sort by name, identically in the preview,
the JSON `headers` field (a comma-separated `NAME=ENVVAR` string), and the
generated `/engram-setup` prose:

| Runtime | Rendering |
| --- | --- |
| Claude Code | `--header 'x-gateway-api-key: ${GATEWAY_KEY}'` |
| opencode | `--header 'x-gateway-api-key={env:GATEWAY_KEY}'` |
| Generic | `"headers": {"x-gateway-api-key": "${GATEWAY_KEY}"}` |

Codex has no custom-header flag (`codex mcp add` exposes only
`--bearer-token-env-var`), so a `--header` run reports a `failed` row for
`codex` naming the header — drop `--header` or select the other runtimes with
`--runtime claude-code,opencode`; setup never writes Codex's configuration
for you. Codex documents its own per-server header configuration in its
config file — configure it there yourself if you need it.

Keep the `${GATEWAY_KEY}` reference literal, including its single quotes, and
never paste the value.

### No authentication

Choose `none` for a server configured to accept unauthenticated requests, such as
a local deployment without an OIDC issuer. This choice configures the client;
it does not change the server's authentication policy.

## Registration and curation skills

`--apply` registers the MCP server and delivers the curation skills
plugin-first. A Claude Code or Codex whose own plugin CLI works receives the
skills, session hooks, and the `/engram-setup` command through its plugin
system — engram's own marketplace and plugin only (`seanb4t/engram`, `engram@engram`).
The marketplace is added when absent, the plugin installed
when absent, updated when outdated, and left alone when current, with the
exact plugin commands shown in the preview. Every other runtime — opencode,
generic, or a Claude Code/Codex without a working plugin CLI — receives the
native skills copy carried in the binary, per the table below. Each present
runtime's row reports registration, plugin (absent, outdated, current, or
unavailable with a reason), and skills separately, alongside one aggregated
outcome. Inspect all three: successful registration does not mean skill or
plugin installation succeeded.

| Runtime | User-scope skill destination |
| --- | --- |
| Claude Code | `~/.claude/skills/` |
| Codex | `~/.agents/skills/`, plus a managed index in `~/.codex/AGENTS.md` |
| opencode | `$XDG_CONFIG_HOME/opencode/skills/` when `XDG_CONFIG_HOME` is absolute; otherwise `~/.config/opencode/skills/` |
| Generic | Printed guidance only; no filesystem destination |

A native-copy Codex receives native skill files and an `AGENTS.md` index that
points to them as a fallback for discovery. Setup preserves unrelated text
around its managed index. JSON output includes the full skill content,
including for manual setup.

A plugin-delivered runtime receives the session hooks through the plugin; a
native-copy runtime does not. See the [plugin guide](/guides/plugin/) for what
the plugin installs and its standalone Claude-only fallback.

## Read results and repeat safely

| Outcome | Meaning |
| --- | --- |
| `not-present` | The runtime binary was not found on `PATH`; absence is expected, not a failure. |
| `would-write` | Proposed changes are shown. For an existing Claude Code or Codex registration, `facets` names what differs (`url`, `auth-mode`, `header-name`, `header-value-ref`) and `drift` details each difference, for example `x-gateway-api-key: observed <redacted>, would write ${GATEWAY_KEY}`. On Claude Code, `--apply` rewrites with `claude mcp remove` then `claude mcp add` — see the [OAuth section](#oauth) for the re-login note. Generic also uses this outcome after `--apply` because it only prints output. |
| `already-correct` | In preview, the registration read through the runtime's own CLI matches the requested URL, auth mode, and header names and references — a real comparison, not a guess. For Claude Code and Codex, `--apply` makes the same comparison before writing and runs no registration command when the row is `already-correct`; opencode is not compared, so its `--apply` writes and then reads back. |
| `preserved` | The existing registration carries something setup did not author and cannot reproduce — an extra header, an unrecognized field — so setup leaves it untouched and `reason` names it; header values read from a runtime are never shown. Claude Code and Codex both replace the whole entry on write (no partial merge), so `--apply` leaves a `preserved` registration untouched — it runs no registration command, not even Claude Code's `mcp remove` — and will never merge into it. To replace it yourself, clear it with the runtime's own tool (`claude mcp remove engram --scope user`; for Codex, delete the `[mcp_servers.engram]` table from `config.toml`), then run setup again — the row then reads `would-write`. |
| `wrote` | Apply performed the reported changes; inspect registration and skills details. For Claude Code and Codex, `registered` shows the new registration, with header values redacted. |
| `failed` | Planning or applying a runtime or skill change failed; read the reason and other results. |

A preview can exit zero while reporting failed or unsupported rows. Read every
row; a zero exit is not proof of registration or connectivity. Apply distinguishes
success, partial failure, and total failure; missing runtimes alone are not
failures. Preserve the exit status and per-runtime results when reporting an
attempt.

Repeating setup with the same inputs converges on the requested registration
without duplicate entries. For Claude Code and Codex, a converged registration
reads `already-correct` and no registration command runs; opencode is written
again each time. After an interruption or partial failure, inspect each
runtime's registration and skills, then preview again before applying. Changes
across runtimes are not one transaction: a failure does not roll back earlier
successes, and replacement can leave a registration removed if adding it
fails. Run one setup operation at a time.

### Scripts and JSON output

Use explicit inputs and `--output json` for scripts. Text output is for people
and is not a stable parsing interface:

```sh
engram setup --url https://engram.example.com/mcp --auth oauth --runtime codex --output json
```

`ENGRAM_URL`, `ENGRAM_AUTH`, `ENGRAM_RUNTIME`, and `ENGRAM_HEADERS` (a
comma-separated `NAME=ENVVAR` list) provide environment defaults; the
corresponding flags override them — `--header` on the command line replaces
the whole `ENGRAM_HEADERS` list. Setup does not prompt interactively. Review
the JSON report before running the same selected invocation with `--apply`.
Keep the runtime's output as report data, not shell instructions. Each
present runtime's JSON row carries a `headers` string, and for Claude Code and Codex also carries `registered` (a normalized rendering of the registration the runtime's CLI reported — URL, auth state, header names — with every header value redacted), `facets` (the comma-joined differing facets, in a fixed order), `drift` (the per-facet detail lines, or a note that the registration was not compared), and `notes` (the OAuth re-login consequence and other tolerant-step records, when present).

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
