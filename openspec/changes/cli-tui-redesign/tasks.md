# Tasks

## 1. Baseline and dependencies

- [x] 1.1 Before adding modules, build the binary on the branch base with the release flags (`go build -trimpath -ldflags '-s -w' -o <scratch>/engram-before ./cmd/engram`) and record its size. Verify the size is written down for the PR description.
- [x] 1.2 Add `charm.land/glamour/v2` and `github.com/sahilm/fuzzy` with `go get`, then run `task deps`. Verify `go list -m charm.land/glamour/v2 github.com/sahilm/fuzzy` resolves both, `go.mod` has no direct requirement on `github.com/charmbracelet/glamour`, and a second `task deps` leaves `go.mod` and `go.sum` unchanged.

## 2. Theme and mouse settings

- [x] 2.1 Add the `client.tui_theme` (`ENGRAM_TUI_THEME`, flag `theme`, default empty) and `client.tui_mouse` (`ENGRAM_TUI_MOUSE`, flag `mouse`, default empty) registry rows and their checks in a console-only `config.ValidateConsole`, so a console setting never fails another client command. Verify `go test ./internal/config -count=1` passes with new cases: empty values are valid, an unknown theme and a non-boolean mouse value are rejected with messages naming the setting and the valid values, and `ValidateClient` ignores both settings.
- [x] 2.2 Bind `--theme` and `--mouse` on `engram tui` through the registry rows, then run `task surfaces:gen`. Verify `git diff cmd/engram/testdata/` shows only the two new flags in the `tui` catalog entry and help section.
- [x] 2.3 Add `tui_prefs.go`: read and write `console.json` in the engram config directory (resolved as `credentialsPath` resolves its directory), written by temporary file and rename with mode 0600, only when a setting changes in the console; resolve each setting as flag, then environment, then saved preference, then default. Verify model tests: each step of the resolution order wins over the next; a missing file and an invalid file both give the defaults and the console still opens with a one-line note; reading never writes the file.
- [x] 2.4 Verify the startup path: `engram tui --theme nope` and `ENGRAM_TUI_MOUSE=maybe engram tui`, with the terminal check forced true, exit 2 before the program starts, and the error names `--theme` and the valid theme names, or names `ENGRAM_TUI_MOUSE`.
- [x] 2.5 Document `--theme`, `--mouse`, `ENGRAM_TUI_THEME`, `ENGRAM_TUI_MOUSE`, the resolution order and the preferences file in `docs-site/src/content/docs/guides/cli.md`'s "Terminal console" section. Verify `task lint:markdown` passes and every name the text uses exists in the code.

## 3. Themes

- [x] 3.1 Add `tui_theme.go`: the role struct (design decision 6), the `engram` theme from the web tokens in `ui/src/app.css`, and the `terminal`, `catppuccin` and `tokyo-night` themes, each with light and dark values. Verify a test that every theme sets every role in both variants.
- [x] 3.2 Request the terminal background at start and pick light or dark from `BackgroundColorMsg`, defaulting to dark until it arrives. When `NO_COLOR` is non-empty, pass an ASCII colour profile to the program. Verify model tests: a light `BackgroundColorMsg` switches to light values; with `NO_COLOR` set, the program options carry the ASCII profile, and an ASCII snapshot still shows state words and edge-type letters.

## 4. Layout, rows and the results list

- [x] 4.1 Add `tui_keys.go` with the full key map from design decision 10 as the single source for the footer, the help overlay and the palette; move the scheduled/expired switch from Tab to `w`; update the existing cli-tui model tests whose keys change. Verify `go test ./cmd/engram -run TUI -count=1` passes.
- [x] 4.2 Add `tui_layout.go`: the three tiers (side by side at width ≥ 100, stacked below 100 when height ≥ 32, otherwise single), `p` to hide the preview and `z` to maximise the focused pane, with the selection held by record id. Verify model tests for each spec scenario size (160×40, 90×45, 70×24), resize keeping the selected record, and Enter opening the full-screen record in the single tier with Esc returning to the same row; add golden view snapshots at the three sizes under an ASCII profile in `cmd/engram/testdata/tui/`.
- [x] 4.3 Add `tui_list.go` rows: selection bar in a reserved column with a blurred colour when the list loses focus, category marker and word, summary, state words, tags with `+N`, scope only when rows differ, compact age; tags then scope drop out as width shrinks; past states use the muted role. Verify model tests for the spec's row scenarios, including a truecolour render in which archived, superseded and expired rows use the muted colour and the scheduled row does not.
- [x] 4.4 Add the results header (counts, scopes, query, ordering, hidden count) and the empty state that names what was searched and the keys that widen it, plus the filter line showing scope, include options and chosen tags. Verify model tests for the spec's "Search results", "Empty result" and "Including archived records" scenarios.
- [x] 4.5 Add automatic paging near the end of the loaded rows, the sentinel row and the position indicator (`12/50+`, `12/137`, `12/50 · top 50`). Verify model tests: one next-page request when the selection comes within one screen of the end, none while one is in flight, the selection stays on the same record, and the indicator text for each case.
- [x] 4.6 Add the search hint (`enter: open`, `enter: list`, `enter: search · 1 embedding`) and local narrowing on `=` with `sahilm/fuzzy`. Verify model tests: typing sends no request, the hint text for each input class, `7 of 50 loaded` for the spec's narrowing scenario with no server request, and Esc restoring every loaded row.
- [x] 4.7 Document the layout tiers, the list keys, the search hint, narrowing and paging in the CLI guide. Verify `task lint:markdown` passes and every key the text names exists in the key map.

## 5. Preview

- [x] 5.1 Add `tui_preview.go`: the first tier drawn from the row, then a 100 ms settle tick carrying the selected id, a `GetMemory` fetch with the previous fetch cancelled, `loading…` only after a 200 ms tick, an LRU of 128 records cleared on reload, and prefetch of the neighbouring rows one at a time. Verify model tests that deliver the ticks directly: moving three rows sends one `GetMemory`, for the final row; a cached record sends none; `loading…` appears only after the 200 ms tick; a late result for a row no longer selected is never drawn.
- [x] 5.2 Render content with Glamour v2 after `consoleText`, off the update loop, with a renderer per width and theme under a mutex and a cache of rendered output by id, width and theme. Verify model tests: a markdown heading and code fence render in the preview, line breaks survive, and the spec's escape-sequence scenario reaches the view with the escape character replaced.
- [x] 5.3 Add focus movement (Tab, Shift-Tab) and preview scrolling from the list (`J`/`K`, Ctrl-d/Ctrl-u), with the footer listing the focused pane's keys. Verify model tests for the scroll keys and the spec's "Help changes with focus" scenario.
- [x] 5.4 Verify the modified detail requirement with model tests: a superseded record's preview shows its successor id; archived, expired and scheduled records show their state words; another actor's `shared` record shows that owner and `shared`; an id that reads not found shows the fixed not-found message.
- [x] 5.5 Measure the render time of a 64 KiB markdown record at 80 and 160 columns and record it for the PR. Verify both numbers are written down.
- [x] 5.6 Document the preview, focus and scroll keys in the CLI guide. Verify `task lint:markdown` passes and every key named exists in the key map.

## 6. Secondary views

- [x] 6.1 Rebuild the related view as lanes in the order supersession, citation, tag, vector, each headed by its letter and count, with evidence per row; Enter walks to a record and `[`/Esc walk back along the trail. Verify model tests for the spec's grouped-lanes scenario and for walking forward and back.
- [x] 6.2 Rebuild the tags view with bars sized against the largest count; `space` chooses a tag and Enter returns to memories with the chosen tags. Verify a model test for the spec's two-tag scenario: the `ListMemories` request carries both tags and the filter line reads `all of #helm #oci`.
- [x] 6.3 Show each scheduled row's window in words and switch scheduled/expired on `w`. Verify a model test for the spec's "Scheduled windows in words" scenario.
- [x] 6.4 Show rules as one index line each with the full text in the preview. Verify a model test that a rule's full text reaches the preview.
- [x] 6.5 Document the related, tags, scheduled and rules views in the CLI guide. Verify `task lint:markdown` passes.

## 7. Overlays

- [x] 7.1 Add `tui_overlay.go`: a compositor overlay stack where Esc closes the topmost overlay and keys go only to it. Verify a model test with two overlays open: one Esc closes only the top one.
- [x] 7.2 Add the `Ctrl-K` palette listing every key-map entry with its key, narrowed with `sahilm/fuzzy`; Enter runs an entry and Esc closes with nothing run. Verify model tests for the spec's palette scenario and that the palette lists every binding in the key map.
- [x] 7.3 Move the help sheet to an overlay grouped by purpose. Verify a model test that it lists every binding in the key map.
- [x] 7.4 Move scope and filters into an overlay with a fuzzy scope list, the cross-spine choice, the three include options and the chosen tags. Verify model tests: choosing a scope and turning on an include option each reload once with the new request fields.
- [x] 7.5 Add the theme picker on `T` and in the palette: moving applies each theme, Enter keeps and saves it, Esc restores the starting theme, and a save while `ENGRAM_TUI_THEME` is set says the variable still wins at the next start. Verify model tests for the spec's "Previewing themes" and "Theme survives a restart" scenarios.
- [x] 7.6 Document the palette, help, filters and theme picker in the CLI guide. Verify `task lint:markdown` passes and every key named exists in the key map.

## 8. Mouse, copy and messages

- [x] 8.1 Add opt-in mouse support: no mouse mode by default; when on, a click selects the row under it through the compositor hit test and the wheel scrolls the pane under the pointer; a palette entry toggles it and saves the preference. Verify model tests: the default view requests no mouse mode, and with it on a click on the third row selects it and the preview follows.
- [x] 8.2 Add `c` and `C` through `tea.SetClipboard`, and a one-line message slot cleared after 3 s by a tick with a sequence guard. Verify model tests: `c` returns the clipboard command for the selected `short_id` and shows a message naming it; the message clears when its tick arrives and survives a stale tick.
- [x] 8.3 Document mouse support and the copy keys in the CLI guide. Verify `task lint:markdown` passes.

## 9. Integration

- [x] 9.1 Build the binary again with the task 1.1 flags and record the size difference with the 5.5 render times in the PR description. Verify both sizes and both times appear there.
- [x] 9.2 Run the console against the local stack (Qdrant in Docker, a stub embedder, `engram serve` with static tokens, as in engram memory `337hf30teq`) in a pseudo-terminal at 160×40, 90×45 and 70×24, step through every view, the palette, the theme picker and the filters, and quit. Verify the screens match `specs/terminal-console/spec.md`, and that the run exits 0 with no write request in the server log.
- [x] 9.3 Run the quality gate `task` (lint and test). Verify it exits 0.

## Workflow follow-up

- Commit with `feat(cli)` types; nothing in this change is breaking.
- Archive the change after review, then confirm the merged `terminal-console` spec passes `task lint:markdown`.
