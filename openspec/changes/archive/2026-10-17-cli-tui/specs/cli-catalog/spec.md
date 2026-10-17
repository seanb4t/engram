# Spec Delta

## Purpose

Gives agents and scripts one machine-readable description of the engram binary: every command, every flag, each command's blast radius, and the exit-code table, derived from the running binary and read through `engram catalog`.

## ADDED Requirements

### Requirement: The catalog command emits one JSON document

`engram catalog` SHALL write exactly one JSON document to stdout, write nothing to stderr, and exit 0. It SHALL behave the same whether or not stdin or stdout is a terminal, and it SHALL need no server, credential or network access.

#### Scenario: Agent reads the catalog through a pipe

- **WHEN** a caller runs `engram catalog` with stdout connected to a pipe
- **THEN** stdout parses in its entirety as one JSON object
- **AND** stderr is empty
- **AND** the process exits 0

#### Scenario: Human reads the catalog in a terminal

- **WHEN** a caller runs `engram catalog` with stdin and stdout connected to a terminal
- **THEN** stdout is the same JSON document a piped invocation writes, and the process exits 0

#### Scenario: No server is configured

- **WHEN** a caller runs `engram catalog` with no `--server` flag and no `ENGRAM_SERVER_URL` set
- **THEN** the command still writes the full document and exits 0, without making any network request

### Requirement: The catalog keeps the self-describe document shape

The document SHALL carry the top-level keys `binary`, `version`, `commands`, `exit_codes` and `notes`, with the same meaning, nesting and snake_case key names that a bare `engram` invocation emitted before this change. Each `commands` entry SHALL carry `name`, `summary`, `flags` and `blast_radius`.

#### Scenario: A consumer of the old bare-invocation document switches command

- **WHEN** a script that parsed bare `engram` output before this change runs `engram catalog` instead
- **THEN** every key it read before is present with the same type and meaning
- **AND** `blast_radius` still carries `read_only`, `destructive`, `idempotent` and `open_world`

### Requirement: The catalog is derived from the live binary

The catalog SHALL list every visible command at any depth, by its qualified name, with every flag a caller may pass to it, including flags inherited from a parent command. It SHALL omit hidden commands and the framework's own `help` and `completion` commands. A command added or removed in a later release SHALL appear or disappear without a separate edit to the catalog.

#### Scenario: The new commands appear in the catalog

- **WHEN** a caller runs `engram catalog`
- **THEN** `commands` contains an entry named `catalog` and an entry named `tui`
- **AND** each of the two entries has a `blast_radius` with `read_only` true, `destructive` false, `idempotent` true and `open_world` false

#### Scenario: Nested commands are listed

- **WHEN** a caller runs `engram catalog`
- **THEN** `commands` contains entries for nested commands such as `spine-review scan`, alongside top-level commands such as `search`

### Requirement: The catalog command takes no arguments

`engram catalog` SHALL reject any positional argument as a usage error: it SHALL write nothing to stdout, write one error line to stderr, and exit with the usage code 2.

#### Scenario: Caller passes an argument

- **WHEN** a caller runs `engram catalog search`
- **THEN** stdout is empty
- **AND** stderr carries one error line
- **AND** the process exits 2

### Requirement: Human help stays separate from the catalog

`engram --help` SHALL keep printing ordinary human help text, never the JSON catalog.

#### Scenario: Caller asks for help

- **WHEN** a caller runs `engram --help`
- **THEN** stdout contains the human usage text and does not parse as JSON
- **AND** the command list in that text includes `catalog` and `tui`
