# Design

## Context

See proposal.md for why. The current state that shapes the approach:

- The console lives in `cmd/engram/tui.go`, `tui_model.go` and `tui_view.go`. It is a pointer Bubble Tea model with one screen per tab (memories, tags, rules, scheduled) and a stack of detail and related screens. Each RPC runs as a command under the client timeout, and its result carries a sequence number so a stale result is dropped. Rendering reuses `memoryStateWords`, `renderCoverageFooter` and `renderRecallHiddenFooter`. Every server string passes through `consoleText`, which replaces control runes.
- Model tests drive the model through a harness that runs each returned command inline against `stubEngramService`, whose write handlers fail the test. The text input uses the real terminal cursor, so no command waits on a timer.
- List and search responses are compact (content cleared). Search is top-k with no page token. Listing uses cursor paging at 50 rows.
- Modules in use: `charm.land/bubbletea/v2` v2.1.0, `charm.land/bubbles/v2` v2.2.1, `charm.land/lipgloss/v2` v2.0.6.
- Client settings live in the `client.*` rows of the `internal/config` registry. They are validated by `config.ValidateClient` and read once in `clientFromFlags`. No client row has a Helm value.
- The engram config directory is resolved by `credentialsPath` in `credcache.go`: `$XDG_CONFIG_HOME/engram`, else `os.UserConfigDir()/engram`.
- The direction comes from an explore session on 2026-10-17 (engram memories `pfk15jg0kr` and `1sm0d2m2j7`). That session surveyed the Charm ecosystem (gh-dash, Crush, Glow, Soft Serve, sesh), best-in-class TUIs (Posting 3, lazygit, k9s, Harlequin, Helix, television, yazi, Atuin) and list-and-preview tools (fzf, telescope, snacks, aerc, lf). The decisions below cite the precedent each one follows.

## Goals / Non-Goals

**Goals:**

- Meet every requirement in `specs/terminal-console/spec.md` with model-level tests and fixed-size view snapshots, never a pseudo-terminal or a wall-clock wait.
- Keep one source of truth for keys: the bindings drive the footer, the help overlay and the palette.
- Keep the console's startup path, exit codes and terminal guard exactly as they are.

**Non-Goals:**

- No change to how records are fetched on the server, and no new RPC.
- No code shared with the web console. The terminal console copies its colour values, words and keys.
- No remappable key bindings, and no theme file format a person can author.

## Decisions

### 1. Layout tiers chosen from the terminal size

| Tier | When | Arrangement |
|---|---|---|
| Side by side | width ≥ 100 | list 50% (never under 48 columns), one `│` divider, preview the rest |
| Stacked | width < 100 and height ≥ 32 | list on top at 55% of the body height, a `─` divider, preview below |
| Single | otherwise | list only; Enter opens the record full screen and Esc returns |

`p` hides or shows the preview in either split tier. `z` maximises the focused pane and restores it. The layout is recomputed from every `WindowSizeMsg`. The selection is held as a record id, not a row index, so a resize or reload keeps it.

Precedents: gh-dash moves its preview below when the list would drop under 80 columns. sesh only splits at 100 columns or more and keeps the list at 40–60 columns. Helix hides its preview at 72 columns or less. lazygit's portrait mode applies at ≤84×≥46. The spec's scenario sizes (160×40, 90×45, 70×24) fall clearly inside one tier each.

Alternatives considered:

- **Always split.** Rejected: a preview narrower than about 40 columns wraps markdown into noise.
- **Three panes with a scope sidebar.** Rejected in the explore session as out of scope.

### 2. Borderless chrome, one divider, an accent bar for selection

There are no boxes around panes. A single divider separates list and preview, and it takes the accent colour when the preview has focus. The selected row carries a `▌` bar in the accent colour in a column every row reserves, so text never shifts, and its summary is bold. When the list loses focus, the bar and row use the theme's blurred-selection colour. There is no reverse video.

Precedents: Crush, gh-dash, Glow and Soft Serve. Chosen in the explore session (memory `1sm0d2m2j7`).

Alternatives considered:

- **lazygit-style boxed panes.** Rejected: the borders cost two columns and two rows per pane, and copying text picks up box characters.
- **Reverse video** (today's look). Rejected: it reads as a terminal default and hides colour.

### 3. One-line rows with width-driven columns

Columns from left to right:

1. The selection bar.
2. The category marker `●` (`*` in ASCII mode) and the category word, padded to 10 columns.
3. The summary, ellipsized.
4. The state words in canonical order.
5. Tags, then `+N`.
6. The scope, only when rows in view come from more than one scope.
7. The age, right-aligned in compact units: `3h`, `2d`, `5mo`.

When width runs short, tags drop first, then scope. State words and age always stay. Rows with a past state use the theme's muted foreground, not terminal faint. The web console found that dimming by opacity fails contrast, and terminals render faint inconsistently.

Precedent: the web console's results list (`sketch-findings-engram`, recall surface), plus gh-dash's relative dates.

Alternative considered: **two-line rows** (Glow, Soft Serve). Rejected in the explore session: they halve the rows on screen.

### 4. The preview loads in two tiers, debounced, cancelled and cached

**First tier, no I/O.** The preview draws at once from the selected row, so it is never blank: the summary in bold, then a metadata line (category · scope · visibility and owner · short_id · age), then the tags and the state.

**Second tier, the full record.**

- **Debounce.** A move starts a 100 ms `tea.Tick` that carries the selected id. When the tick arrives and the id still matches, the model fetches the record with `GetMemory` under the client timeout.
- **Cancel.** The previous fetch's context is cancelled.
- **Loading text.** A second tick at 200 ms shows `loading…` only if the record has not arrived by then.
- **First load.** The first preview, with nothing yet shown, fetches with no debounce.
- **Stale results.** A result for an id that is no longer selected is kept in the cache but never drawn. The existing sequence check covers this too.
- **Prefetch.** After a settled fetch, the rows directly above and below are fetched one at a time.

**Caches.**

- Fetched records: an LRU of 128, keyed by id, cleared on any reload.
- Rendered markdown: keyed by id, width and theme.

Tests deliver the tick messages directly, so nothing in a test waits on a timer.

Precedents:

- aerc debounces its preview by 100 ms.
- lazygit's task manager kills the previous task and writes `loading...` only after 200 ms.
- fzf runs only the latest preview request.
- tv and lf cache previews per item, and yazi and lf preload neighbours.

Alternatives considered:

- **List and search with `full=true`.** Rejected: 50 rows of up to 64 KiB each can reach `response_too_large`, and listing is compact by design.
- **Fetch on every move.** Rejected: holding `j` would send a request per row.

### 5. Markdown through Glamour v2, sanitised first, rendered off the update loop

Content passes through `consoleText` (control runes replaced, line breaks kept) before `charm.land/glamour/v2` renders it. The renderer is built from the active theme's `ansi.StyleConfig` and wraps at the smaller of the preview width minus 2 and 100 columns.

Rendering runs inside the fetch command, not in `Update`. Glamour v2 renderers are not safe for concurrent `Render` calls, so each command builds or borrows a renderer for its (width, theme) under a mutex. The rendered string goes into the markdown cache. A resize or theme change re-renders from the cached record.

`charm.land/glamour/v2` (v2.0.1) is the maintained `charm.land` vanity path for `github.com/charmbracelet/glamour`. The old path's latest tag, v1.0.0, is the v1 line, which rule `xvqj44e5mk` warns about.

**Measurements to take before shipping:** binary size before and after, as task 3.1 did for #832; render time for a 64 KiB record.

Alternatives considered:

- **Plain wrapped text.** Rejected: memory content is mostly markdown with code fences.
- **goldmark with our own renderer.** Rejected by rule `xvqj44e5mk`.

### 6. Themes are semantic roles with light and dark values

A theme is a struct of roles:

- `fg`, `muted`, `faint`
- `accent`, `onAccent`
- `divider`, `dividerFocus`
- `selection`, `selectionBlurred`
- `warning`, `error`, `success`
- six category colours (convention, gotcha, decision, preference, discovery, rule)

Every role holds a light and a dark value.

| Theme name | Source |
|---|---|
| `engram` (default) | the web console's tokens in `ui/src/app.css` (the category colours, `--primary`, `--warning`, the surfaces) |
| `terminal` | ANSI indices 0–15 only, so the person's terminal palette shows |
| `catppuccin` | Mocha (dark) and Latte (light) |
| `tokyo-night` | Night (dark) and Day (light) |

**Choosing light or dark.**

- At start the model asks for the terminal background (`tea.RequestBackgroundColor`) and picks light or dark from `BackgroundColorMsg.IsDark()`.
- It uses dark until the answer arrives, as Glow does.

**Colour depth and `NO_COLOR`.**

- Bubble Tea's colour profile downsamples truecolour values to 256 or 16 colours on its own.
- When `NO_COLOR` is set to a non-empty value, the console passes an ASCII colour profile to the program itself. It does not rely on the library for this.

**Edge-type letters.** S, C, T and V show in every theme.

**The picker** is an overlay over the live view. Moving through it applies each theme at once; Enter keeps the theme and saves it, and Esc restores the theme the person started with.

Precedents:

- Crush's named roles.
- gh-dash's ANSI-indexed defaults.
- Posting's live theme preview with Esc to revert.
- Atuin's semantic "meanings".

Alternatives considered:

- **One fixed palette.** Rejected in the explore session in favour of themes with a picker.
- **A user-authored theme file.** Out of scope.

### 7. Settings resolve flag, then environment, then saved preference, then default

**Two new registry rows:**

| Key | Env | Flag | Default | Valid values |
|---|---|---|---|---|
| `client.tui_theme` | `ENGRAM_TUI_THEME` | `theme` | `""` | `""` or a theme name |
| `client.tui_mouse` | `ENGRAM_TUI_MOUSE` | `mouse` | `""` | `""` or anything `strconv.ParseBool` accepts |

- **Validation.** A console-only `config.ValidateConsole` checks both, kept apart from `config.ValidateClient` so an `ENGRAM_TUI_THEME` meant for the console can never fail `engram search`. A bad value is a usage error (exit 2) that names the setting and the valid values, before the console opens.
- **Helm.** No Helm value, like every other `client.*` row: the chart deploys only the server.
- **Flags.** `engram tui` binds `--theme` and `--mouse`, with `--mouse` a bool flag as `--insecure` is. Bare `engram` takes no flags, so it reads the environment and the saved preference.
- **Resolution order.** The empty default means "not set", so the model falls back in order: flag, environment, saved preference, then `engram` and mouse off.
- **Picker while the variable is set.** If a person changes the theme in the picker while `ENGRAM_TUI_THEME` is set, the choice is saved. The confirmation message says the environment variable still wins at the next start.

**The preferences file** is `console.json` in the engram config directory, found the same way as `credentials.json`. It holds `{"theme": "...", "mouse": false}`.

- It is written only when the person changes a setting inside the console.
- It is written to a temporary file and renamed, with mode 0600, matching the directory's existing files.
- A missing, unreadable or invalid file yields the defaults. The console still opens and shows a one-line note.

Alternatives considered:

- **A preferences file only, with no `ENGRAM_` key.** Rejected: this repo's settings are env-first through the registry, and a script or an SSH session needs a way to set the theme without writing a file.
- **Flags only.** Rejected: bare `engram` takes no flags.

### 8. Overlays are Lip Gloss compositor layers

**The overlays:**

- the command palette
- the help sheet
- the scope and filters panel
- the theme picker
- the full-screen record in the single tier

Each overlay is a `lipgloss.Layer` placed over the base view by a `lipgloss.Compositor`. It is drawn with a rounded border in the accent colour and a title rule. Esc closes the topmost overlay, and one key event goes to the topmost overlay only.

Precedent: Crush's dialog stack.

Alternative considered: **replacing the base view** (today's filters panel). Rejected: the person loses the context the overlay acts on.

### 9. Fuzzy matching through `github.com/sahilm/fuzzy`

The palette and local narrowing both use `github.com/sahilm/fuzzy` v0.1.3. It is maintained, last pushed in 2026-06, and Bubbles v2 itself requires it for its list filter. It ranks matches and reports the matched character positions, which the palette underlines.

**What narrowing matches.**

- It matches the loaded rows' summary, category, tags and `short_id`.
- It never sends a request.
- The header states `7 of 50 loaded`.
- Esc clears the narrowing.

Alternative considered: **substring matching.** Rejected: rule `xvqj44e5mk` prefers the established library, and fuzzy ranking is what palette users expect.

### 10. Keys

The console uses the web console's key wherever one exists. It never reuses a web-console write key (`a`, `A`, `S`, `s`, `e`, `#`, `x`, `X`) for anything else, so a person moving between the two is never surprised.

| Key | Action | Precedent |
|---|---|---|
| `j`/`k`, `↓`/`↑`, PgDn/PgUp, Home/End | move | web console |
| Enter | open the record full screen; in the related view, walk to that record | web console, Harlequin F10 |
| Esc | tiered back: overlay, then narrowing, then focus, then screen | web console, Frogmouth |
| Tab / Shift-Tab | move focus between list and preview | nap, Textual |
| `J`/`K`, Ctrl-d/Ctrl-u | scroll the preview from the list | yazi, gh-dash, telescope |
| `/` | search box (server search on Enter) | web console |
| `=` | narrow the loaded rows | eilmeldung, fzf |
| `f` | scope and filters panel | cli-tui |
| `1`–`4` | memories, tags, rules, scheduled | cli-tui |
| `r` | related memories of the selection | web console |
| `[` | walk back along the related trail | web console |
| `space` | in the tags view, choose a tag | VisiData |
| `w` | in the scheduled view, switch scheduled and expired (moved off Tab) | — |
| `n` | load the next page now | cli-tui |
| `p` / `z` | hide the preview / maximise the focused pane | gh-dash, lazygit |
| `c` / `C` | copy `short_id` / full id | web console |
| `T` | theme picker | — |
| Ctrl-K | command palette | web console |
| `?` | help overlay | web console |
| `q`, Ctrl-C | quit | cli-tui |

Every action is a `key.Binding` in one key map. The footer renders the focused pane's short help, trimmed from the right at narrow widths and always ending in `? help`. The help overlay renders the grouped full help. The palette lists every binding with its key. So a key cannot appear in one place and be missing from another.

Precedents: Posting and Zellij generate their footers from bindings; Zellij's v0.31 fix records the drift this prevents.

### 11. Search hint, paging and the position indicator

**Search hint.** The classifier from cli-tui decides the hint shown at the right of the search box:

- `enter: open` for a single UUID- or `short_id`-shaped token;
- `enter: list` for an empty query;
- `enter: search · 1 embedding` for anything else.

**Paging.**

- The next page is requested when the selection comes within one screen of the last loaded row, the server returned a page token, and no page request is in flight.
- The rows are appended and the selection stays on the same id.
- A sentinel row reads `loading 50 more…`, `end · 137 memories`, or `couldn't load more · n to retry`.

**Position indicator.** It reads `12/50+` while a page token exists and `12/137` once the list is complete. Search shows `12/50 · top 50`.

Precedents: gh-dash fetches when you navigate past the last loaded item. Harlequin stopped reporting a truncated fetch as an exact total.

### 12. Secondary views

**Related.**

- Lanes in the order supersession, citation, tag, vector.
- Each lane heading reads `S supersession (2)`.
- Rows carry evidence from the existing edge messages:
  - supersession: direction and depth;
  - citation: the shared citation ref;
  - tag: the shared tags;
  - vector: the score as two decimals and a bar.
- Enter walks to the selected record and pushes it on a trail; `[` and Esc walk back.

**Tags.**

- Bars sized against the largest count, with each count right-aligned.
- `space` chooses a tag, shown as `✓`, and Enter returns to memories with the chosen tags as the `tags` filter.
- The filter line reads `all of #helm #oci`, because the filter is an AND.

**Scheduled.** Each row ends with its window in words: `reveals in 3d` or `expired 2d ago`, from `not_before` and `not_after`, using the same compact units as row ages.

**Rules.** One index line per rule. The preview shows the rule's full text.

Precedents: the web console's lanes and `S`/`C`/`T`/`V` glyphs, VisiData's frequency table, and the web console's tag bars.

### 13. Mouse, clipboard and messages

**Mouse.** When it is on, the view sets `MouseMode` to cell motion. A click is mapped to a row or pane through the compositor's hit test, and the wheel scrolls the pane under the pointer. When it is off, the console requests no mouse mode, so the terminal's own selection works.

Alternative considered: **bubblezone.** Rejected: its README warns that it may not work with the Lip Gloss v2 compositor.

**Clipboard.** `c` and `C` send `tea.SetClipboard`, OSC 52, which also works over SSH. The confirmation names what was copied.

**Messages.** Confirmations and notes show for 3 s in a one-line message slot at the bottom right. A `tea.Tick` carrying a sequence number clears them, and tests deliver that tick directly.

**Errors.** Request errors stay in the status line with the envelope unchanged, as today, and are never put in a message that clears itself.

### 14. Code stays in `cmd/engram`, split by concern

The redesign still needs `clientFromFlags`, `wrapRPCError`, `memoryStateWords` and the footer helpers from `package main`, so it stays there. The files are split by concern:

- `tui_model.go`: the root model and routing
- `tui_layout.go`: tiers and sizes
- `tui_list.go`: rows, paging and narrowing
- `tui_preview.go`: tiers, cache and markdown
- `tui_views.go`: related, tags, scheduled and rules
- `tui_overlay.go`: palette, help, filters and theme picker
- `tui_theme.go`: roles and themes
- `tui_prefs.go`: the preferences file and resolution order
- `tui_keys.go`: the key map
- `tui_view.go`: composition

Alternative considered: **an `internal/console` package.** Rejected again for the reason cli-tui gave: it would first need the client constructor, error mapper and state helpers moved out of `main`. That is a refactor of its own.

### 15. Tests

- **Model tests** keep the existing harness and the fail-on-write stub. Every requirement scenario in the spec delta maps to at least one model test.
- **View snapshots.**
  - `View()` output is compared at 160×40, 90×45 and 70×24 under an ASCII colour profile, so the text is stable, plus one truecolour snapshot of the `engram` theme.
  - Golden files live in `cmd/engram/testdata/tui/` and are regenerated with the package's existing `-update` flag.
- **Timers.** Tick-driven behaviour (debounce, delayed loading text, message expiry) is tested by delivering the tick message, never by waiting.
- **Startup.** The settings resolution order and the invalid-value exits are tested through `runClient` with `forceTerminal`, as the startup tests are today.
- **Scope.** No test asserts how Bubble Tea, Bubbles, Lip Gloss, Glamour or fuzzy behave (rule `m45p2b4bp7`).

## Risks / Trade-offs

- **[Glamour and chroma grow the binary.]** → Measure before and after, and record both in the PR, as #832 did.
- **[Rendering a 64 KiB record may be slow.]** → It runs off the update loop and is cached. Measure render time before shipping. If it is slow, render the first screen first.
- **[Some terminals do not answer the background query.]** → Default to dark, as Glow does. The `terminal` theme and `ENGRAM_TUI_THEME` give a fixed choice.
- **[Some terminals ignore OSC 52.]** → The confirmation still shows. The full-screen record view lets a person select the id with the terminal's own selection when the mouse is off.
- **[Snapshot goldens churn on every visual tweak.]** → Keep them to three sizes, and review the diffs as part of the change. They catch layout regressions that model tests cannot.
- **[The change is large.]** → Tasks are grouped so each group lands working and tested on its own: settings first, then layout and rows, preview, views, overlays, then themes and mouse.
- **[A new `ENGRAM_` key is a compatibility surface.]** → Both keys are additive and client-side only, and an empty value means "not set". No existing key changes.

## Migration Plan

1. Land the change on a branch with a `feat` commit. Nothing breaks: the startup path, exit codes and the catalog are unchanged.
2. On its first run, the console finds no preferences file and uses its defaults.
3. Rollback is a revert. A leftover `console.json` is ignored by older binaries, which never read it.

## Open Questions

- The exact Catppuccin and Tokyo Night values for the six category colours. These can be chosen during implementation from each palette's published accent colours without changing a requirement.
