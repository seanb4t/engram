# terminal-console Specification

## Purpose

Gives a person at a terminal an interactive, read-only way to browse and inspect their engram memories, opened by a bare `engram` or `engram tui`, and makes sure a caller without a terminal is pointed at the machine-readable catalog instead.

## Requirements

### Requirement: Bare engram opens the terminal console on a terminal

When stdin and stdout are both terminals, a bare `engram` invocation SHALL open the terminal console, exactly as `engram tui` with no flags does.

#### Scenario: Human types engram

- **WHEN** a person runs `engram` with no arguments in an interactive terminal, with `ENGRAM_SERVER_URL` set to a reachable server
- **THEN** the terminal console opens and shows the caller's memories

### Requirement: Bare engram without a terminal points to the catalog

When stdin or stdout is not a terminal, a bare `engram` invocation SHALL NOT open the console, read stdin or wait for input. It SHALL write nothing to stdout, write one line to stderr that names `engram catalog` as the machine-readable form, and exit with the usage code 2.

#### Scenario: Agent runs bare engram through a pipe

- **WHEN** a caller runs `engram` with no arguments and stdout connected to a pipe
- **THEN** stdout is empty
- **AND** stderr is one line that contains `engram catalog`
- **AND** the process exits 2 without waiting for input

#### Scenario: Stdin is not a terminal

- **WHEN** a caller runs `engram` with no arguments, stdout connected to a terminal and stdin redirected from a file
- **THEN** the console does not open, stderr is one line that contains `engram catalog`, and the process exits 2

### Requirement: The tui command opens the console with client settings

`engram tui` SHALL open the terminal console and accept the shared client settings `--server` (or `ENGRAM_SERVER_URL`), `--token-file`, `--insecure` and `--timeout` (or `ENGRAM_TIMEOUT`), with the same meaning and validation they have on `engram search`. It SHALL take no positional arguments.

#### Scenario: Caller names the server with a flag

- **WHEN** a person runs `engram tui --server https://engram.example` in an interactive terminal
- **THEN** the console opens against that server

#### Scenario: The tui command without a terminal

- **WHEN** a caller runs `engram tui` with stdout connected to a pipe
- **THEN** stdout is empty, stderr is one line stating that `engram tui` needs a terminal and naming `engram catalog`, and the process exits 2

#### Scenario: Caller passes an argument

- **WHEN** a caller runs `engram tui extra`
- **THEN** the console does not open, stderr carries one error line, and the process exits 2

### Requirement: The console uses the client credential order

The console SHALL choose its credential with the same order every client command uses, and it SHALL NOT accept a credential value on the command line. With `--insecure`, it SHALL write the TLS warning to stderr before the console takes over the screen.

#### Scenario: Token from the environment

- **WHEN** `ENGRAM_TOKEN` is set and a person opens the console
- **THEN** every request the console sends carries that token as its bearer credential

#### Scenario: Insecure transport

- **WHEN** a person runs `engram tui --insecure` in an interactive terminal
- **THEN** stderr carries the TLS-verification warning before the console opens

### Requirement: Startup failures exit before the console opens

Before it takes over the screen, the console SHALL check its settings and make one read request to the server. A failure SHALL leave the terminal as it was, write the same stderr message a client command writes for that failure, and exit with that failure's exit code.

#### Scenario: No server configured

- **WHEN** a person runs `engram` in an interactive terminal with no `--server` and no `ENGRAM_SERVER_URL`
- **THEN** the console does not open, stderr names `--server` and `ENGRAM_SERVER_URL`, and the process exits 2

#### Scenario: Credential rejected

- **WHEN** a person opens the console against a server that rejects the caller's credential
- **THEN** the console does not open, stderr carries the authentication error, and the process exits 3

#### Scenario: Server unreachable

- **WHEN** a person opens the console against a server that does not answer
- **THEN** the console does not open, stderr carries the transport error, and the process exits 5, or 6 when the `--timeout` deadline passed first

### Requirement: The console is read-only

The console SHALL send only read requests. It SHALL offer no action that stores, updates, supersedes, archives, restores, deletes or changes the visibility of a record, or that stores a rule or discovery.

#### Scenario: A person browses and quits

- **WHEN** a person searches, lists, opens records and quits the console
- **THEN** the server received no write request from the console during the session

### Requirement: Recall views honour the recall gate

Search and list views SHALL show only records the server's recall gate returns. By default they SHALL hide archived, superseded, expired and scheduled records. The person SHALL be able to include archived, superseded, and scheduled-or-expired records, each on its own. When the server reports how many records it hid, the view SHALL show that count by state.

#### Scenario: Default view hides gated records

- **WHEN** a person lists a scope that holds an archived, a superseded, an expired and a scheduled record, with no include option turned on
- **THEN** none of the four records is shown
- **AND** the view shows the server's hidden-record count, broken down as archived, superseded, expired and scheduled

#### Scenario: Including archived records

- **WHEN** the person turns on the option to include archived records
- **THEN** the archived record is shown, marked `archived`, and the superseded, expired and scheduled records stay hidden

#### Scenario: Including scheduled records reveals expired ones too

- **WHEN** the person turns on the option to include scheduled records
- **THEN** both the scheduled record and the expired record are shown, marked `scheduled` and `expired` respectively

### Requirement: Rows and detail show derived state in canonical order

Every record the console shows SHALL carry its derived-state words in the canonical order `archived`, `superseded`, `expired`, `scheduled`. `expired` SHALL suppress `scheduled`.

#### Scenario: Record that is archived and superseded

- **WHEN** the console shows a record that is both archived and superseded
- **THEN** its state reads `archived superseded`, in that order

### Requirement: Detail view fetches any record by id

Selecting a record SHALL fetch it by id and show, in the preview or the full-screen record view, its full content, summary, scope, category, tags, visibility, owner, creation time and derived state. Fetch by id is not recall-gated, so the console SHALL show an archived, superseded, expired or scheduled record, and for a superseded record it SHALL show the id of its successor.

#### Scenario: Opening a superseded record

- **WHEN** a person opens a superseded record from a view that includes superseded records
- **THEN** the detail view shows its full content, the state word `superseded`, and the successor's id

#### Scenario: Previewing a record in each derived state

- **WHEN** the selection moves onto an archived, an expired and then a scheduled record, in a view that includes them
- **THEN** the preview of each shows its full content and its state word, `archived`, `expired` or `scheduled` respectively

#### Scenario: Previewing another actor's shared record

- **WHEN** the selection moves onto a `shared` record owned by another actor
- **THEN** the preview shows that owner and the visibility `shared`

### Requirement: Records the caller does not own

The console SHALL show another actor's `shared` record read-only, with its owner and its `shared` visibility visible. A record that is private to another actor SHALL be reported as not found, in the same words as a record that does not exist.

#### Scenario: Shared record from another actor

- **WHEN** a search returns a `shared` record owned by another actor
- **THEN** the row and the detail view show that owner and the visibility `shared`

#### Scenario: Another actor's private record

- **WHEN** a person opens an id that belongs to another actor's private record
- **THEN** the console reports the record as not found, with the same message it gives for an id that does not exist

### Requirement: Related, tags, rules and scheduled views

The console SHALL let a person view, for a selected record, its related memories grouped by edge type with a count per group; the tags of the current scope with their counts; the rules of the caller's readable rule scopes; and the caller's own scheduled and expired records with each record's window stated in words.

#### Scenario: Related memories of an open record

- **WHEN** a person asks for the related memories of an open record
- **THEN** the console lists each related record with its edge type (supersession, shared tag, shared citation or vector neighbour)

#### Scenario: Related memories are grouped by edge type

- **WHEN** a record's related memories include two supersession edges and five shared-tag edges
- **THEN** the console shows a supersession group headed with the count 2 and a shared-tag group headed with the count 5, in the order supersession, shared citation, shared tag, vector neighbour
- **AND** each group heading carries a letter (S, C, T, V) as well as any colour

#### Scenario: Scheduled records

- **WHEN** a person opens the scheduled view
- **THEN** the console lists the caller's own not-yet-active records, and can switch to the caller's expired records

#### Scenario: Scheduled windows in words

- **WHEN** the scheduled view lists a record that becomes active in three days, and the person switches to expired records that include one which expired two days ago
- **THEN** the first record reads "reveals in 3d" and the second reads "expired 2d ago"

### Requirement: Request errors during a session are shown, not fatal

After the console has opened, a failed request SHALL be shown in the console with the server's message unchanged, including any `field=<name> hint=<code>` envelope, and the session SHALL continue.

#### Scenario: Response too large

- **WHEN** a request the console sends is rejected with `field=response hint=response_too_large`
- **THEN** the console shows that envelope and its text unchanged
- **AND** the person can keep using the console

### Requirement: Quitting restores the terminal

Quitting the console SHALL restore the terminal to its previous state and exit 0.

#### Scenario: Person quits

- **WHEN** a person presses the quit key, or sends an interrupt with Ctrl-C
- **THEN** the terminal returns to its previous screen and the process exits 0

### Requirement: Results and preview side by side on a wide terminal

On a terminal wide enough for both, the console SHALL show the results list and a preview of the selected record side by side, with the list on the left.

#### Scenario: Wide terminal

- **WHEN** a person opens the console in a terminal 160 columns wide and 40 rows high
- **THEN** the results list and the preview of the selected record are both visible, side by side

### Requirement: Layout adapts to the terminal size

The console SHALL stack the preview below the list on a terminal too narrow for two columns but tall enough for both. On a terminal too small for either arrangement it SHALL show only the list, and Enter SHALL open the selected record full screen. A resize SHALL re-lay the console out without losing the selection.

#### Scenario: Narrow and tall terminal

- **WHEN** the console runs in a terminal 90 columns wide and 45 rows high
- **THEN** the preview appears below the list

#### Scenario: Small terminal

- **WHEN** the console runs in a terminal 70 columns wide and 24 rows high and the person presses Enter on a row
- **THEN** only the list was visible before Enter, and the record now fills the screen until Esc returns to the list with the same row selected

#### Scenario: Resize keeps the selection

- **WHEN** a person selects the fifth row and resizes the terminal from 160 to 70 columns
- **THEN** the fifth row is still selected

### Requirement: The preview follows the selection

Moving the selection SHALL update the preview without a further key press. The preview SHALL show what the row already carries at once, and SHALL show the record's full content once it has loaded. A preview SHALL never show a different record's content than the selected one.

#### Scenario: Moving through rows

- **WHEN** a person moves the selection down three rows and stops
- **THEN** the preview shows the summary of the row now selected straight away, and then its full content

#### Scenario: A slow load does not show stale content

- **WHEN** the full record for row A is still loading and the person moves to row B
- **THEN** the preview never shows row A's content while row B is selected

### Requirement: The person can maximise or hide the preview

The console SHALL offer one key that toggles the preview between hidden and shown, and one key that maximises the focused pane and restores it.

#### Scenario: Hiding the preview

- **WHEN** a person presses the hide-preview key on a wide terminal
- **THEN** the list takes the full width, and pressing it again shows the preview

### Requirement: Rows fit on one line and carry their state

Each result row SHALL take one line and show the record's category, summary, derived-state words, tags (with a `+N` count for those that do not fit) and age. A row for an archived, superseded or expired record SHALL be drawn muted; a row for a scheduled record SHALL NOT. The selected row SHALL stay identifiable when the list does not have focus.

#### Scenario: Row content

- **WHEN** the list shows a decision record created two days ago with five tags, in a width that fits two of them
- **THEN** its row reads the word `decision`, the summary, the first two tags, `+3`, and `2d`

#### Scenario: Past states are muted

- **WHEN** a view that includes them lists an archived, a superseded, an expired and a scheduled record
- **THEN** the archived, superseded and expired rows are drawn muted and carry their state words, and the scheduled row is not muted and carries `scheduled`

#### Scenario: Selection while the preview has focus

- **WHEN** a person moves focus from the list to the preview
- **THEN** the selected row is still marked in the list

### Requirement: Each results view states what it searched

Each results view SHALL state in words, above the rows, how many records it shows, how many scopes it covered, the query when there is one, how the rows are ordered, and how many records the recall gate hid. An empty result SHALL name what was searched and the keys that would widen it.

#### Scenario: Search results

- **WHEN** a cross-spine search for `helm` returns 41 records from 9 scopes and the recall gate hid 3
- **THEN** the view states 41 hits, 9 scopes, the query `helm`, that the order is by similarity, and 3 hidden by the recall gate

#### Scenario: Empty result

- **WHEN** a scope-confined search returns no records and the recall gate hid 2 archived records
- **THEN** the view states that nothing matched the query in that scope, that 2 were hidden, and names the keys for searching every readable scope and for including archived records

### Requirement: The list shows its position and loads more as needed

The console SHALL show the selected row's position among the loaded rows, marking the count as incomplete while the server reports more pages. When the selection comes near the last loaded row and more pages exist, the console SHALL request the next page without a key press, and SHALL keep the selection where it was.

#### Scenario: Position with more pages

- **WHEN** the first page of 50 rows is loaded, more pages exist and the twelfth row is selected
- **THEN** the position reads `12/50+`

#### Scenario: Reaching the end of the loaded rows

- **WHEN** the person moves the selection to within one screen of the last loaded row and more pages exist
- **THEN** the console requests the next page once, appends its rows, and keeps the same record selected

### Requirement: Search states what Enter will do

While a person types in the search box, the console SHALL state what Enter will do: open a record by id for a single UUID- or `short_id`-shaped token, list with the current filters for an empty query, or run a server search that costs one embedding. It SHALL never send an id-shaped token as a search query.

#### Scenario: Typing text

- **WHEN** a person types `helm upgrade` in the search box
- **THEN** the console states that Enter will run a search costing one embedding, and has sent no search request

#### Scenario: Typing a short_id

- **WHEN** a person types a `short_id` and presses Enter
- **THEN** the console opens that record by id and sends no search request

### Requirement: Local narrowing filters loaded rows without a request

The console SHALL let a person narrow the rows already loaded by typing, with no request to the server, and SHALL state how many loaded rows match out of how many are loaded. Clearing the narrowing SHALL restore every loaded row.

#### Scenario: Narrowing

- **WHEN** 50 rows are loaded and the person narrows by `oci`, which matches 7 of them
- **THEN** the list shows those 7 rows, states `7 of 50 loaded`, and the server received no request for it

### Requirement: Active filters are visible and change the results

The console SHALL show the current scope or cross-spine choice, the include options that are on, and any chosen tags, on one line above the results. Changing any of them SHALL reload the results with the new filters.

#### Scenario: Including archived records

- **WHEN** the person turns on the option to include archived records
- **THEN** the filter line shows it, and the reloaded results include archived records, marked `archived`

### Requirement: Chosen tags filter the memories

The console SHALL let a person choose one or more tags from the tags view and then list the memories that carry all of them. The chosen tags SHALL be sent as the tag filter on the list and search requests, and SHALL show on the filter line until the person clears them.

#### Scenario: Two tags

- **WHEN** the person chooses the tags `helm` and `oci` and returns to the memories view
- **THEN** the list request carries both tags, every listed record carries both, and the filter line shows them

### Requirement: Command palette

`Ctrl+K` SHALL open a palette that lists every view, filter and action the console offers, each with its key, and narrows the list as the person types. Enter SHALL run the chosen entry and Esc SHALL close the palette with nothing run.

#### Scenario: Running an action from the palette

- **WHEN** a person opens the palette, types `tags` and presses Enter on the tags view entry
- **THEN** the console shows the tags view, and the entry showed the same key that opens the tags view directly

### Requirement: Colours follow a theme and are never the only signal

The console SHALL choose light or dark colours from the terminal's background by default, and SHALL offer named themes, including one that uses only the terminal's own colours. A theme picker SHALL preview each theme as the person moves through it, and Esc SHALL restore the previous theme. With `NO_COLOR` set, the console SHALL use no colour. Every state and edge type SHALL also have a word or letter.

#### Scenario: Previewing themes

- **WHEN** a person opens the theme picker, moves to another theme and presses Esc
- **THEN** the console showed the other theme while it was highlighted, and shows the original theme again after Esc

#### Scenario: NO_COLOR

- **WHEN** a person opens the console with `NO_COLOR` set
- **THEN** the console draws no colour, and the derived-state words and edge-type letters still show

### Requirement: Console preferences persist locally

The console SHALL save the theme and mouse setting the person chooses inside it to a preferences file in the engram configuration directory, and SHALL read that file at the next start. It SHALL never send them to the server. A missing, unreadable or invalid preferences file SHALL leave the console on its defaults and SHALL NOT stop it from opening.

#### Scenario: Theme survives a restart

- **WHEN** a person chooses a theme, quits and opens the console again
- **THEN** the console uses the chosen theme, and the server received no write request

#### Scenario: Invalid preferences file

- **WHEN** the preferences file holds invalid content and a person opens the console
- **THEN** the console opens on its default theme with the mouse off

### Requirement: Theme and mouse settings take a flag or the environment first

`engram tui` SHALL accept `--theme` and `--mouse`, and the console SHALL read `ENGRAM_TUI_THEME` and `ENGRAM_TUI_MOUSE`. A flag SHALL win over the environment variable, the environment variable over the saved preference, and the saved preference over the default. An unknown theme name or a mouse value that is not a boolean SHALL be a usage error that exits 2 before the console opens.

#### Scenario: Environment wins over the saved preference

- **WHEN** the saved preference names one theme and `ENGRAM_TUI_THEME` names another
- **THEN** the console uses the theme named by `ENGRAM_TUI_THEME`

#### Scenario: Unknown theme

- **WHEN** a person runs `engram tui --theme nope` in an interactive terminal
- **THEN** the console does not open, stderr names `--theme` and the valid theme names, and the process exits 2

### Requirement: Mouse support is opt-in

Mouse input SHALL be off by default. When the person turns it on, a click on a row SHALL select it and the wheel SHALL scroll the pane under the pointer.

#### Scenario: Default

- **WHEN** a person opens the console with no saved preference
- **THEN** the console does not capture the mouse, so the terminal's own text selection works

#### Scenario: Clicking a row

- **WHEN** mouse support is on and the person clicks the third row
- **THEN** the third row is selected and the preview shows it

### Requirement: Copying ids

`c` SHALL copy the selected record's `short_id` and `C` its full id to the clipboard through the terminal, and the console SHALL confirm the copy in a short message that clears itself.

#### Scenario: Copying a short_id

- **WHEN** a person presses `c` on a selected record
- **THEN** the console sends the record's `short_id` to the terminal clipboard and shows a message naming it

### Requirement: Key help matches the key bindings

The bottom line SHALL list the main keys for whatever has focus, and `?` SHALL open an overlay listing every key, grouped by purpose. Every key shown in either place SHALL do what it says.

#### Scenario: Help changes with focus

- **WHEN** focus moves from the list to the preview
- **THEN** the bottom line lists the preview's scrolling keys in place of the list's

### Requirement: Server text cannot control the terminal

Text that comes from the server, including record content, summaries, owners, scopes, tags and error messages, SHALL reach the terminal with every control character replaced, so it can never send the terminal an escape sequence. Line breaks in a record's content SHALL be kept.

#### Scenario: Another actor's shared record carries an escape sequence

- **WHEN** a `shared` record owned by another actor has content holding a terminal escape sequence and the person previews it
- **THEN** the terminal receives the content with the escape character replaced, and the content's line breaks are kept
