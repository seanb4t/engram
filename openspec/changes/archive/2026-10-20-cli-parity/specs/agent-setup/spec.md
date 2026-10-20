# Spec Delta

## Purpose

How `engram setup` registers engram as an MCP server in each agent runtime. This slice covers
the OAuth callback port a Claude Code registration carries; the rest of `engram setup` is
unchanged and not restated here.

## ADDED Requirements

### Requirement: Setup can fix the OAuth callback port

`engram setup` SHALL accept `--callback-port <port>` (`ENGRAM_CALLBACK_PORT`), a TCP port from 1
to 65535, valid with `--auth oauth` and `--auth oauth-client`. For Claude Code the registration
SHALL pass the port as `--callback-port` on `claude mcp add`, so the OAuth redirect URI the
runtime registers is `http://localhost:<port>/callback` on every login.

#### Scenario: OAuth with a fixed port

- **WHEN** `engram setup --url https://engram.example/mcp --auth oauth --callback-port 8765` runs
- **THEN** the Claude Code preview shows `claude mcp add ... --callback-port 8765`, and the same arguments run under `--apply`

#### Scenario: OAuth without a port

- **WHEN** `--callback-port` is not given with `--auth oauth`
- **THEN** the Claude Code registration carries no `--callback-port`, byte-identical to the registration before this change

#### Scenario: Pre-registered client keeps its default

- **WHEN** `--auth oauth-client --client-id c` runs without `--callback-port`
- **THEN** the Claude Code registration carries `--callback-port 8765`, as it did before this change

#### Scenario: Pre-registered client with another port

- **WHEN** `--auth oauth-client --client-id c --callback-port 9000` runs
- **THEN** the Claude Code registration carries `--callback-port 9000`

#### Scenario: Port with a non-OAuth mode

- **WHEN** `--callback-port 8765` is given with `--auth bearer` or `--auth none`
- **THEN** the command writes a usage error saying the flag applies only to the OAuth modes to stderr and exits with code 2 without running any runtime command

#### Scenario: Port out of range

- **WHEN** `--callback-port 70000` or `--callback-port 0` is given
- **THEN** the command writes a usage error naming the valid range to stderr and exits with code 2

### Requirement: Runtimes without a callback-port option report it

For a runtime whose `mcp add` offers no callback-port option, a `--callback-port` run SHALL
report that runtime's row as `failed`, naming the flag, and SHALL leave the other selected
runtimes' rows unaffected, matching how `--header` on Codex is reported today.

#### Scenario: Codex selected with a port

- **WHEN** `engram setup --auth oauth --callback-port 8765 --runtime claude-code,codex` runs
- **THEN** the Claude Code row carries the port, the Codex row reads `failed` naming `--callback-port`, and the exit code follows the partial-failure rule (8)

### Requirement: The setup result names the callback port

The JSON and text setup results SHALL carry, on every present row, the callback port a
registration was given: the caller's `--callback-port`, or the `oauth-client` arm's `8765`.
Claude Code's own `mcp get` does not report the port back (measured on 2.1.304), so the port
is reported as requested and SHALL NOT take part in the `already-correct` comparison; the guide
SHALL say so and tell the reader to re-run with `--apply` after changing the port.

#### Scenario: Preview shows the port

- **WHEN** a preview runs with `--callback-port 8765`
- **THEN** the Claude Code row's output names `8765` as the callback port

#### Scenario: Absent runtime carries no port

- **WHEN** the selected runtime is not installed
- **THEN** its row carries no callback port, as it carries no header facet

### Requirement: The guide documents the headless login

The agent-setup guide SHALL document how to complete the Claude Code OAuth login for engram from
an SSH session with no local browser: the `claude mcp login engram --no-browser` paste-back flow,
and the local-forward flow that needs a fixed callback port and `ssh -t -L <port>:localhost:<port>`.
It SHALL state that the login happens once per host, because the runtime refreshes the token
afterwards.

#### Scenario: Reader over SSH

- **WHEN** a reader with Claude Code on a remote host and a browser only on their own machine follows the guide
- **THEN** they can complete the login without any change on the identity provider
