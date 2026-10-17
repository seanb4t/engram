# Proposal

## Why

The read-only terminal console that bare `engram` and `engram tui` open (#819, PR #832) works, but people will not want to use it. It shows one view at a time, so reading a record means leaving the list. Every line is the same plain text. Rows lead with ids, repeat long scope strings and have no ages. Footers use the CLI's machine form (`recall_gate_hidden: 4  archived: 1 …`). The detail view is a key-and-value dump with unrendered markdown. Enter on a tag does nothing, and related memories are one flat list. This change comes from #834. Its direction comes from an explore session on 2026-10-17 that surveyed about 60 terminal UIs (the Charm and Bubble Tea ecosystem, the best-regarded TUIs in other ecosystems, and list-and-preview search tools).

## What Changes

- **Split layout with a live preview.** Results sit on the left, and a preview of the selected record sits on the right and follows the cursor. A tall, narrow terminal stacks the preview under the list. A small terminal shows one pane, and Enter opens the record full screen. One key maximises the focused pane, and one key hides or shows the preview.
- **Readable rows.** Each record takes one line: a category marker and word, the summary, the derived-state words, tags with a `+N` overflow, and a relative age. Scope appears only when it differs between rows and the width allows it. A coloured left bar marks the selection, and the bar stays visible in grey when the list loses focus.
- **Preview content.** The preview shows the summary, then one metadata line, the tags and the derived state. Below that it renders the content as markdown. The preview draws from the row at once, then loads the full record once the cursor stops moving.
- **Honest headers and footers.** Each view states what it searched in words, for example "41 hits across 9 scopes for "helm" · ranked by cosine · 3 hidden by the recall gate". The bottom line shows the position (`12/41`, or `12/150+` while more pages exist) and the hidden counts by state. Empty results name what was searched and the keys that would widen the search.
- **Search and local narrowing.** `/` runs a server search on Enter, as today. A hint says what Enter will do: open an id, list with the current filters (an empty query), or run a search that costs one embedding. A new key narrows the rows already loaded, as you type, with no server request, and shows how many loaded rows match.
- **Facets in view.** The scope, the cross-spine choice, the three include options and any chosen tags show on one line above the results, and each one has a key.
- **Paging.** The next page loads when the cursor nears the end of the loaded rows. `n` still loads the next page on request.
- **Better secondary views.**
  - Related memories are grouped by edge type (supersession, citation, tag, vector), with a count and a letter for each group, so colour is never the only signal.
  - Tags show as bars sized by count. A person can choose tags and list the memories that carry all of them.
  - Scheduled records show their window in words ("reveals in 3d", "expired 2d ago").
- **Command palette.** `Ctrl+K` opens a fuzzy palette of every view, filter and action, each shown with its key, the same key as the web console.
- **Themes.** Colours come from named roles. By default the console picks light or dark from the terminal's background, using the web console's palette. It also ships a theme that uses only the terminal's own colours, plus a few named themes. A picker previews each theme live, and Esc reverts. The console honours `NO_COLOR`, and every state also has a word or glyph.
- **Mouse, opt in.** When it is turned on, a click selects a row and the wheel scrolls the pane under the pointer. It is off by default, so the terminal's own text selection keeps working.
- **Copy.** `c` copies the selected record's `short_id` and `C` copies its full id, as in the web console, through the terminal clipboard sequence, which also works over SSH.
- **Key help.** The bottom line lists the keys for whatever has focus, built from the key bindings themselves. `?` opens a grouped overlay of every key.
- **Theme and mouse settings.** Two new client settings, `ENGRAM_TUI_THEME` and `ENGRAM_TUI_MOUSE`, with `--theme` and `--mouse` flags on `engram tui`. A choice made inside the console persists in a preferences file in the engram config directory, next to the login cache, and applies whenever neither the flag nor the environment variable is set. Nothing about them is sent to the server.
- The console stays read-only, and every startup check, exit code and terminal guard stays as it is.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `terminal-console`: the detail and secondary-view requirements change to the split layout, the preview, grouped related memories, tag filtering and scheduled windows. New requirements cover the layout tiers, local narrowing, paging, the command palette, themes, mouse, copy, key help, local preferences, and keeping server text from controlling the terminal.

## Impact

- **Code:** `cmd/engram/tui*.go` (the console model, views and rendering) and its tests. Bare `engram` and `engram tui` keep the same entry point, startup checks and exit codes.
- **Dependencies:** a new direct dependency, `charm.land/glamour/v2`, for markdown. It brings a syntax highlighter (chroma) and a markdown parser (goldmark), so the binary grows; the PR records the size before and after. Design.md confirms each new module is the maintained upstream.
- **On-disk state:** a new console preferences file in the engram config directory. It is not a credential and holds no record data.
- **Config keys (compatibility decision: additive):** two new client-side keys, `ENGRAM_TUI_THEME` and `ENGRAM_TUI_MOUSE`, in the `internal/config` registry. They have no Helm value, like every other client key, because the chart deploys only the server. No existing key changes.
- **Wire contract:** otherwise deliberately unchanged. No proto field, MCP input schema, error hint code or stored payload key changes. The console uses only RPCs it already calls, plus the existing `tags` filter on `ListMemories` and `SearchMemories`.
- **Server:** deliberately unchanged.
- **MCP tool schemas:** deliberately unchanged.
- **Connect proto:** deliberately unchanged.
- **CLI:** the console's behaviour and keys change, and `engram tui` gains `--theme` and `--mouse`, so its `--help` text and catalog entry gain those two flags. Every other command is unchanged.
- **Console (`ui/`):** deliberately unchanged. The terminal console reuses its colours, words and keys, but shares no code with it.
- **Helm chart:** deliberately unchanged.
- **docs-site:** `guides/cli.md`'s "Terminal console" section is rewritten for the new layout, keys, themes, mouse and preferences.
- **engram skill:** deliberately unchanged. No skill text drives the console.
- **CLAUDE.md memory contract:** deliberately unchanged.
- **Out of scope:**
  - Curation writes (store, update, supersede, archive, restore, delete, share), which stay a later change.
  - A scope sidebar or three-pane layout.
  - Drawing a node-and-link graph.
  - Inline images.
  - Remappable key bindings.
  - Any server, proto or MCP change.
