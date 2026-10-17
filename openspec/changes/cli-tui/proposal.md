# Proposal

## Why

Running `engram` with no arguments writes the self-describe catalog (D-15) to the terminal: one large JSON document that is useful to an agent and unreadable to a person. A human who types `engram` gets a wall of JSON instead of a way to work with their memories. This change comes from #819, filed after an explore session on 2026-10-17.

## What Changes

- **BREAKING:** the self-describe catalog moves from bare `engram` to a new `engram catalog` command. The document keeps its shape and is still derived from the live command tree. It gains entries only for commands that are new in this release, including `catalog` itself.
- When stdin and stdout are both terminals, bare `engram` opens a terminal user interface (TUI) for the memory store.
- When either one is not a terminal (an agent, a pipe, a script), bare `engram` never starts the TUI and never waits for input. It writes one line to stderr that names `engram catalog` and exits with the usage code (`exitUsage`, 2).
- The first version of the TUI reads only. It shows search and list results, a record's full detail with its derived state, related memories, tags, rules and scheduled records, across the caller's readable scopes. It uses the same Connect API, server setting and credential order as the other client commands.
- `engram --help` and every other command are unchanged.

## Capabilities

### New Capabilities

- `cli-catalog`: the machine-readable self-describe catalog: the command that emits it, its document shape, and how it is derived from the live binary.
- `terminal-console`: the interactive terminal interface that bare `engram` opens, and how bare `engram` behaves when it is not attached to a terminal.

### Modified Capabilities

None. No existing spec covers the CLI.

## Impact

- **Wire contract:** the CLI's catalog contract moves to a new command. This is a compatibility decision. It breaks any external agent or script that runs bare `engram` to read the catalog, so the commit is a `feat!`. Nothing in this repository runs bare `engram`: the skills, hooks and `engram setup` were checked on 2026-10-17. The non-terminal pointer exists so a broken caller learns the new command from the failure. No proto field, MCP input schema, error hint code, `ENGRAM_` key or stored payload key changes.
- **Code:** `cmd/engram` (`root.go` RunE, a new `catalog` command, and the TUI), `internal/surfaces` (a blast-radius row for `catalog`), and the `help.golden` and `catalog.golden` fixtures.
- **Server:** deliberately unchanged.
- **MCP tool schemas:** deliberately unchanged.
- **Connect proto:** deliberately unchanged. The TUI uses the RPCs the console already calls.
- **CLI:** bare `engram` changes behaviour; `engram catalog` is new.
- **Console (`ui/`):** deliberately unchanged. The TUI is a separate client and does not share code with the web console.
- **Helm chart:** deliberately unchanged.
- **docs-site:** `guides/cli.md` (the self-describe catalog section becomes `engram catalog`, plus a TUI section) and `contributing/architecture.md` (the catalog golden description).
- **engram skill:** deliberately unchanged. No skill text runs bare `engram`.
- **CLAUDE.md memory contract:** deliberately unchanged.
- **Dependencies:** new Go dependencies for the TUI: Bubble Tea v2 (`charm.land/bubbletea/v2`) and the libraries it pairs with for components and styling. Design.md confirms the exact modules and that each one is the maintained upstream.
- **Credentials:** the TUI works with `ENGRAM_TOKEN` and `--token-file` today. It picks up the login cache automatically once change `cli-login` (#818) lands, because both use the same credential chain. This change does not depend on `cli-login`.
- **Out of scope:** curation writes from the TUI (store, update, supersede, archive, restore, delete), which can follow in a later change; the agent session primer, which is change `cli-prime` (#820); and any change to `engram --help`.
