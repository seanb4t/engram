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

Opening a record SHALL fetch it by id and show its full content, summary, scope, category, tags, visibility, owner, creation time and derived state. Fetch by id is not recall-gated, so the detail view SHALL show an archived, superseded, expired or scheduled record, and for a superseded record it SHALL show the id of its successor.

#### Scenario: Opening a superseded record

- **WHEN** a person opens a superseded record from a view that includes superseded records
- **THEN** the detail view shows its full content, the state word `superseded`, and the successor's id

### Requirement: Records the caller does not own

The console SHALL show another actor's `shared` record read-only, with its owner and its `shared` visibility visible. A record that is private to another actor SHALL be reported as not found, in the same words as a record that does not exist.

#### Scenario: Shared record from another actor

- **WHEN** a search returns a `shared` record owned by another actor
- **THEN** the row and the detail view show that owner and the visibility `shared`

#### Scenario: Another actor's private record

- **WHEN** a person opens an id that belongs to another actor's private record
- **THEN** the console reports the record as not found, with the same message it gives for an id that does not exist

### Requirement: Related, tags, rules and scheduled views

The console SHALL let a person view, for an open record, its related memories with each edge's type; the tags of a scope with their counts; the rules of the caller's readable rule scopes; and the caller's own scheduled and expired records.

#### Scenario: Related memories of an open record

- **WHEN** a person asks for the related memories of an open record
- **THEN** the console lists each related record with its edge type (supersession, shared tag, shared citation or vector neighbour)

#### Scenario: Scheduled records

- **WHEN** a person opens the scheduled view
- **THEN** the console lists the caller's own not-yet-active records, and can switch to the caller's expired records

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
