# Design

## Context

See proposal.md for why. The current state that shapes the approach:

- `rootCmd` (`cmd/engram/root.go`) has `RunE: runSelfDescribe` and `Args: cobra.NoArgs`. `runSelfDescribe` (`catalog.go`) encodes `buildCatalog(cmd.Root())` to stdout. D-15 (`.planning/milestones/v0.12.x-phases/02-headless-cli-client/02-CONTEXT.md`) made the bare invocation the agent's discovery path; D-16 kept `--help` as human output.
- `buildCatalog` walks the live tree with `walkCommands(root, commandWalkSkip)` and panics on any command with no `internal/surfaces` blast-radius row. `testdata/catalog.golden` is built from `buildCatalog(rootCmd)` directly, not from a bare invocation, so moving the RunE does not touch how the golden is produced. `task surfaces:gen` regenerates both goldens.
- Tests that run the bare invocation and expect the catalog: `TestRootBareInvocationEmitsCatalog`, `TestCatalogGoesToStdoutNotStderr`, `TestHelpAndCatalogAreDifferentOutputs` (`catalog_test.go`), and the `root/self-describe` row of `exitcode_baseline_test.go` (exit 0 today).
- Client commands share `addClientFlags` and `clientFromFlags` (`client_common.go`): the registry load, validation, credential order (`ENGRAM_TOKEN`, then `--token-file`), the `--insecure` warning, and the timeout. `wrapRPCError` maps a Connect error to the exit-code taxonomy. `isTerminal` tests `os.ModeCharDevice`. `--token-file` and `--insecure` have no environment form, by design (D-13, D-14).
- `memoryStateWords` (`memory_state.go`) already renders derived state in canonical order for `engram list`/`search` text output, and `renderRecallHiddenFooter` renders the hidden-record counts.
- The Connect API already has every read the console needs: `ListScopes`, `ListMemories`, `SearchMemories`, `GetMemory`, `RelatedMemories`, `ListTags`, `ListRules`, `ListScheduled`. Their `include_archived`, `include_superseded` and `include_scheduled` fields default to the recall gate.

## Goals / Non-Goals

**Goals:**

- A bare `engram` is useful to a person and impossible for an agent to misread: the console on a terminal, a one-line pointer and exit 2 everywhere else.
- The catalog contract survives the move with its shape unchanged.
- The console reuses the client lane's settings, credential order, error mapping and state rendering instead of growing a second copy.

**Non-Goals:**

- No new `ENGRAM_` setting, config key, Helm value or server behaviour.
- No mouse support, theming options or saved console state in this version.
- No sharing of code or tokens with the web console in `ui/`.

## Decisions

### 1. Move the catalog to `engram catalog`, and give bare `engram` to the console

`catalogCmd` gets the current `runSelfDescribe` as its RunE. `rootCmd.RunE` becomes the console entry point. The catalog document and `buildCatalog` do not change.

Alternatives considered:

- **Detect the terminal and keep JSON for pipes.** Rejected. One invocation would carry two contracts chosen by the environment. Some agent runtimes run commands inside a pseudo-terminal, so an agent there would get a full-screen program instead of JSON. That breaks the correct-by-reading principle (engram memory `4aksmneehh`): the caller cannot tell from the command what it will get.
- **Put the catalog in `engram prime`.** Rejected. `prime` becomes the session primer in #820, so one name would carry two contracts, and the agent contract would break twice.
- **Keep bare JSON and add only `engram tui`.** Rejected. It leaves the reported problem in place: a person who types `engram` still gets a wall of JSON.
- **Bare `engram` prints `--help` on a terminal.** Rejected. `--help` already exists and gives no way to work with memories.
- **Keep JSON for one release with a deprecation warning on stderr.** Rejected. That window is the terminal-detection contract above under another name. engram is pre-1.0, and the stderr pointer is the migration aid.

### 2. The no-terminal guard checks stdin and stdout, before anything else

Bare `engram` and `engram tui` both start by checking `isTerminal(os.Stdin) && isTerminal(os.Stdout)`, before loading configuration or dialling. A failed check returns `usageErrorf` with a single line that names `engram catalog` (and `engram --help`), so `Execute` prints it on stderr and exits 2. The terminal check is a package-level function value so a test can force the terminal branch without a pseudo-terminal.

Stdin is checked as well as stdout because the console reads key presses from stdin. With stdin redirected, it would either hang or quit on end of file.

Alternatives considered:

- **Exit 0 with the pointer.** Rejected. A script that parsed the old catalog would see empty stdout and a success, and fail later with a confusing JSON error. Exit 2 makes the break visible where it happens.
- **Check stdout only, matching `outputFormatFromConfig`.** Rejected for the reason above. Output format only decides how to write; the console also has to read.

### 3. A visible `engram tui` command carries the client flags; bare `engram` delegates to it

`engram tui` registers `--server`, `--token-file`, `--insecure` and `--timeout` through the same registry rows `addClientFlags` binds. It does not register `--output`, because no output format applies to a full-screen program. Bare `engram` runs the same entry point with no flags, so it reads `ENGRAM_SERVER_URL`, `ENGRAM_TOKEN` and `ENGRAM_TIMEOUT`. `internal/surfaces/toolclass.go` gains CLI-only rows for `catalog` and `tui`, both read-only, non-destructive, idempotent and not open-world. These match the `version` and `search` rows.

Alternatives considered:

- **Put the client flags on `rootCmd`.** Rejected. Root's flags would show in `engram --help` but not in the catalog, which lists subcommands only, so an agent could not discover them. It would also mix client flags into the root help of a binary whose operator commands use different flags.
- **Bare `engram` with environment settings only, and no `tui` command.** Rejected. `--token-file` and `--insecure` have no environment form by design (D-13, D-14), so a person who keeps their token in a file could not use the console until #818 adds the login cache.

This adds a `tui` row to `internal/surfaces`, in addition to the `catalog` row the proposal names.

### 4. Startup checks run before the console takes over the screen

The entry point runs the no-terminal guard, then `clientFromFlags`, then one `ListScopes` call with the client timeout, and only then starts the Bubble Tea program. A failure returns through `wrapRPCError`, so stderr and the exit code match what `engram search` gives for the same failure: 2 for a missing server, 3 for a rejected credential, 5 for an unreachable server, 6 for a deadline. `ListScopes` is used because the scope picker needs it anyway, it is the cheapest read, and it goes through authentication.

Alternative considered: **open the console at once and show failures inside it.** Rejected. A person with a bad credential would be put into a full-screen program that can only tell them to quit, and a wrapper script would lose the standard exit codes.

### 5. Bubble Tea v2, Bubbles v2 and Lip Gloss v2 from `charm.land`

| Module | Version seen on 2026-10-17 | Use |
|---|---|---|
| `charm.land/bubbletea/v2` | v2.1.0 | program loop, key and window messages |
| `charm.land/bubbles/v2` | v2.2.1 | `textinput`, `list`, `viewport`, `help`, `key` |
| `charm.land/lipgloss/v2` | v2.0.6 | layout and styles |

The `charm.land` paths are the vanity import paths for the `github.com/charmbracelet/*` repositories: the `go-import` meta tag of `charm.land/bubbletea` points to `https://github.com/charmbracelet/bubbletea`. The old `github.com/charmbracelet/bubbles` and `github.com/charmbracelet/lipgloss` paths resolve only to the v1 line (v1.0.0 and v1.1.0). That is the vanity-path migration that rule `xvqj44e5mk` warns about, so the change imports only the `charm.land` v2 paths. The Bubbles v2 upgrade guide (fetched through Context7) confirms the `charm.land/bubbles/v2/<component>` import form. Version numbers here record what was resolvable during planning; the module versions are whatever `go get` resolves at apply time.

Alternatives considered:

- **`tview` on `tcell`.** Rejected. It uses a mutable widget tree driven by callbacks. Bubble Tea's model-update-view loop is a pure function of messages, so the console's behaviour can be unit-tested by feeding it messages, with no pseudo-terminal.
- **`gocui`.** Rejected. Lower-level and less actively maintained.
- **Hand-written ANSI handling.** Rejected by rule `xvqj44e5mk`.

### 6. The console lives in `cmd/engram`, in `tui_*.go` files

The console needs `clientFromFlags`, `wrapRPCError`, `memoryStateWords` and the hidden-count rendering, all of which are in package `main`. Putting the console in `cmd/engram` reuses them without moving code. The model holds one view at a time (results, detail, related, tags, rules, scheduled) plus a status line, a scope picker built from `ListScopes`, and three include toggles.

Each RPC runs as a Bubble Tea command under a context with the client timeout. Its result comes back as a message tagged with a request sequence number, and a result older than the view's latest request is dropped, so a slow response cannot overwrite a newer one.

Alternative considered: **a separate `internal/tui` package.** Rejected for now. It would mean moving the client constructor, the error mapper and the state helpers out of `main` first. That refactor can be done later on its own.

### 7. Search runs on Enter, not on every key press

The server embeds every search query through the configured embedding provider. Searching as the person types would send one embedding request per key press, which costs money and can hit provider rate limits. Search therefore runs when the person presses Enter. List, tags, rules and scheduled views load when they are opened. Lists use cursor paging with a page size well under the 1000 maximum, so a large scope does not run into `response_too_large`.

Alternative considered: **search as you type with a debounce.** Rejected for the cost reason above. A debounce lowers the number of requests but does not bound it.

### 8. Recall views send the recall gate's defaults and show what it hid

List and search requests send `include_archived`, `include_superseded` and `include_scheduled` from three toggles that default to off, so the default view is exactly the recall gate. Rows show `memoryStateWords`, and the view footer shows `recall_gate_hidden` by state, plus `searched_scopes` and `scopes_truncated` for a cross-spine request. Detail calls `GetMemory`, which is not gated. A `not_found` from `GetMemory` is shown with one fixed message, whether the id does not exist or belongs to another actor's private record. Owner and visibility come from the record the server returns.

### 9. Keys follow the web console where a key has an equivalent

`/` focuses search, Enter runs it or opens the selected row, Esc goes back, `j`/`k` and the arrow keys move, `?` shows the key help, and `q` or Ctrl-C quits. Where the web console's keyboard model (`engram-console-conventions` skill) already has a key for an action, the console uses the same key.

### 10. Tests drive the model and the stub server, never Bubble Tea itself

- Model tests send messages (key presses, RPC results, window sizes) to the model and assert on `View()` text and on the requests a `stubEngramService` (`clienttest_test.go`) receives. Every write handler on that stub fails the test if it is called, which is the evidence for the read-only requirement.
- The guard and the startup checks are tested through `runClient`. Its buffers are not terminals, which covers the guard branch. The forced-terminal function value covers the startup-failure branches.
- No test asserts how Bubble Tea, Bubbles or Lip Gloss behave (rule `m45p2b4bp7`).

## Risks / Trade-offs

- [An external agent or script that runs bare `engram` breaks.] → It gets exit 2 and a stderr line naming `engram catalog`. The commit carries a `BREAKING CHANGE:` footer, so the release notes say so.
- [An agent runtime that runs commands in a pseudo-terminal opens the console and waits.] → No local fix: that caller has a terminal by every test the binary can make. `guides/cli.md` tells agents to use `engram catalog`, and Ctrl-C exits the console with 0.
- [The three new modules make the binary larger.] → Measure the `engram` binary size before and after on the release build target, and record both in the PR. This is a measurement to take before shipping.
- [Bubble Tea v2 is a recent major version, so its API may still move.] → The versions are pinned in `go.mod`, and Renovate updates them through the normal dependency PRs.
- [A slow response arrives after a newer one.] → Results carry a request sequence number, and stale results are dropped (decision 6).

## Migration Plan

1. Land the change on a branch with a `feat!` commit and a `BREAKING CHANGE:` footer that names `engram catalog`. release-please derives the release from that.
2. Callers move from `engram` to `engram catalog`. The document they parse is unchanged.
3. Rollback is a revert of the change. That restores the bare-invocation catalog, and `engram catalog` and `engram tui` disappear with it.

## Open Questions

- Exact colours and layout proportions. These can be chosen during implementation without changing any requirement.
