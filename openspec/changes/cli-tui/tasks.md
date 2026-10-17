# Tasks

## 1. Move the catalog to `engram catalog`

- [x] 1.1 Add `catalogCmd` (`Use: "catalog"`) in `cmd/engram/catalog.go` with `runSelfDescribe` as its RunE and an `Args` check that returns `usageErrorf` for any positional argument, and register it on `rootCmd`. Verify `go run ./cmd/engram catalog | jq -e '.commands | length > 0'` succeeds and `go run ./cmd/engram catalog search; echo $?` prints `2`.
- [x] 1.2 Add a CLI-only `catalog` row to `internal/surfaces/toolclass.go` (read-only, non-destructive, idempotent, not open-world). Verify `go test ./internal/surfaces -count=1` and `go test ./cmd/engram -run TestCatalogBlastRadiusMatchesToolClasses -count=1` pass.
- [x] 1.3 Retarget `TestRootBareInvocationEmitsCatalog`, `TestCatalogGoesToStdoutNotStderr` and `TestHelpAndCatalogAreDifferentOutputs` in `catalog_test.go` from the bare invocation to `engram catalog`, and add tests for a positional argument (stdout empty, exit 2) and for running with no `--server` or `ENGRAM_SERVER_URL` (full document, exit 0). Verify `go test ./cmd/engram -run 'Catalog' -count=1` passes.
- [x] 1.4 Run `task surfaces:gen`. Verify `git diff cmd/engram/testdata/` shows `catalog.golden` gaining only the `catalog` entry and `help.golden` gaining only the `catalog` help section and its line in the command list.

## 2. Bare `engram` without a terminal

- [x] 2.1 Replace `rootCmd.RunE` with the console entry point, whose first step is a no-terminal guard: a package-level terminal-check function value that tests both `os.Stdin` and `os.Stdout` with `isTerminal`, and on failure returns `usageErrorf` with one line naming `engram catalog` and `engram --help`. Keep `Args: cobra.NoArgs`, and rewrite the D-15 comment block in `root.go` to describe the new contract. Verify with a `runClient` test: stdout empty, stderr one line containing `engram catalog`, exit 2.
- [x] 2.2 Change the `root/self-describe` row of `exitcode_baseline_test.go` to a `root/no-terminal` row with `before: exitOK`, `after: exitUsage`, `changes: true`, and add a `catalog` row with exit 0. Verify `go test ./cmd/engram -run TestExitCodeBaseline -count=1` passes, including `TestExitCodeBaselineRowCount` with its expected count updated.
- [x] 2.3 Confirm `TestRootUnknownSubcommandStillErrors`, `TestRootExistingSubcommandsStillResolve` and `TestHelpFlagStillPrintsHumanHelp` still pass unchanged. Verify `go test ./cmd/engram -run 'TestRoot|TestHelpFlag' -count=1` passes.
- [x] 2.4 Update `docs-site/src/content/docs/guides/cli.md`: the frontmatter description, the "The self-describe catalog" section (now `engram catalog`, with the `jq` example rewritten to `engram catalog | jq ...`), and a sentence saying that a bare `engram` with no terminal exits 2 and names `engram catalog`. Update the catalog-golden sentence in `docs-site/src/content/docs/contributing/architecture.md`. Verify `rg -n 'Run .engram. with no arguments|bare .engram. catalog' docs-site/src` returns nothing and `task lint:markdown` passes.

## 3. Dependencies

- [x] 3.1 Before adding modules, build the binary on `main` with the release flags (`go build -trimpath -ldflags '-s -w' -o <scratch>/engram-before ./cmd/engram`) and record its size. Verify the size is written down for the PR description.
- [x] 3.2 Add `charm.land/bubbletea/v2`, `charm.land/bubbles/v2` and `charm.land/lipgloss/v2` with `go get`, then run `task deps`. Verify `go list -m charm.land/bubbletea/v2 charm.land/bubbles/v2 charm.land/lipgloss/v2` resolves all three, `go.mod` has no direct requirement on `github.com/charmbracelet/bubbles` or `github.com/charmbracelet/lipgloss`, and `git diff --exit-code go.mod go.sum` is clean after a second `task deps`.

## 4. The `tui` command and startup checks

- [x] 4.1 Add `tuiCmd` (`Use: "tui"`) in `cmd/engram/tui.go` with `--server`, `--token-file`, `--insecure` and `--timeout` bound to the same `client.*` registry rows `addClientFlags` uses (no `--output`), an `Args` check that returns `usageErrorf` for a positional argument, and the same entry point bare `engram` runs. Verify `go run ./cmd/engram tui --help` lists exactly those four flags plus `--help`, and `go run ./cmd/engram tui extra; echo $?` prints `2`.
- [x] 4.2 Add a CLI-only `tui` row to `internal/surfaces/toolclass.go` with the same classification as `catalog`. Verify `go test ./internal/surfaces -count=1` and `go test ./cmd/engram -run TestCatalogBlastRadiusMatchesToolClasses -count=1` pass.
- [x] 4.3 Implement the startup sequence: guard, then `clientFromFlags`, then one `ListScopes` call under the client timeout, then start the program. Route failures through `wrapRPCError`. Verify with tests that force the terminal check true against `stubEngramService`: no server gives exit 2 with stderr naming `--server` and `ENGRAM_SERVER_URL`; a `CodeUnauthenticated` stub gives exit 3; a dead server address gives exit 5; a hung stub past `--timeout`, following `timeout_test.go`'s pattern, gives exit 6; `--insecure` writes the TLS warning to stderr before the program starts.
- [x] 4.4 Verify the guard for `engram tui` itself: with `runClient` (non-terminal buffers), stdout is empty, stderr is one line saying `engram tui` needs a terminal and naming `engram catalog`, and the exit code is 2.
- [x] 4.5 Run `task surfaces:gen`. Verify `git diff cmd/engram/testdata/` shows `catalog.golden` gaining only the `tui` entry and `help.golden` gaining only the `tui` help section and its line in the command list.

## 5. The console

- [x] 5.1 Load the `engram-console-conventions` skill, then add the model skeleton in `cmd/engram/tui_*.go`: one active view, a status line, the key map (`/`, Enter, Esc, `j`/`k` and arrows, `?`, `q`, Ctrl-C) using the web console's key where one exists, and the alternate screen. Verify model tests: `q` and Ctrl-C both return the quit command, and a program run that quits returns nil so the process exits 0.
- [x] 5.2 Add the scope picker built from the startup `ListScopes` result, a cross-spine toggle, and the list view with cursor paging at a page size well under 1000. Verify model tests against the stub: `ListMemories` carries the chosen scope or `cross_spine`, the first page sets `cursor_mode`, and the next page sends the returned `page_token`.
- [x] 5.3 Add search on Enter. Verify a model test: typing a query sends no `SearchMemories` request, and Enter sends exactly one, carrying the typed query and the active scope or `cross_spine`.
- [x] 5.4 Add the three include toggles, `memoryStateWords` on every row, the `recall_gate_hidden` footer by state, and the `searched_scopes` / `scopes_truncated` footer for cross-spine requests. Verify model tests: the default request has `include_archived`, `include_superseded` and `include_scheduled` all false; turning on the archived toggle sets only `include_archived`; a stub record that is archived and superseded renders `archived superseded`; a stub `recall_gate_hidden` renders its archived, superseded, expired and scheduled counts.
- [x] 5.5 Add the detail view through `GetMemory`: content, summary, scope, category, tags, visibility, owner, creation time, derived state and, for a superseded record, the successor id. Verify model tests: a superseded stub record shows its successor id; a `CodeNotFound` reply renders one fixed not-found message; a `shared` record owned by another actor shows that owner and `shared`.
- [x] 5.6 Add the related view (`RelatedMemories`, each edge's type), the tags view (`ListTags`, counts), the rules view (`ListRules`) and the scheduled view (`ListScheduled`, switching between scheduled and expired). Verify one model test per view: the stub receives the expected request, and `View()` shows the stub's records with their edge types or counts.
- [x] 5.7 Show request errors in the status line without leaving the session. Verify a model test: a stub that rejects a list with `field=response hint=response_too_large` produces a status line containing that envelope unchanged, and a following key press is still handled.
- [x] 5.8 Tag every RPC result with a request sequence number and drop stale results. Verify a model test that delivers an older search result after a newer one: the view keeps the newer result.
- [x] 5.9 Give the test stub write handlers (`StoreMemory`, `UpdateMemory`, `DeleteMemory`, `SetVisibility`, `ScheduleMemory`, `ArchiveMemory`, `RestoreMemory`, `SupersedeMemory`, `StoreDiscovery`) that fail the test when called, and run a scripted session through every view to quit. Verify `go test ./cmd/engram -run 'TUI' -count=1` passes.
- [x] 5.10 Add a "Terminal console" section to `docs-site/src/content/docs/guides/cli.md`: how bare `engram` and `engram tui` open it, its four flags, that it is read-only, its keys, and the startup exit codes 2, 3, 5 and 6. Verify `task lint:markdown` passes and every key and flag the section names exists in the code.

## 6. Integration

- [ ] 6.1 Build the binary again with the same flags as task 3.1 and record the size difference in the PR description. Verify both sizes appear there.
- [x] 6.2 Against a reachable engram server, run `engram` in an interactive terminal, open each view, toggle each include option and quit. Then run `engram | cat; echo $?` and `engram catalog | jq -e .binary`. Verify the console behaves as `specs/terminal-console/spec.md` describes, the pipe prints the pointer and `2`, and the catalog parses.
- [x] 6.3 Run the quality gate `task` (lint and test). Verify it exits 0.

## Workflow follow-up

- Commit with a `feat!` type and a `BREAKING CHANGE:` footer that names `engram catalog` as the replacement for the bare-invocation catalog.
- Archive the change after review, then confirm the merged `cli-catalog` and `terminal-console` specs pass `task lint:markdown`.
