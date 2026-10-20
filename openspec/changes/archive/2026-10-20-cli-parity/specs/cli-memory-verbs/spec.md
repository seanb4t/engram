# Spec Delta

## Purpose

The client-tier verbs of the engram CLI that read and write memory on a remote engram server
over Connect, so a person, a script, or an agent without an MCP session can recall and curate
memory with the same capabilities the MCP tools give.

## ADDED Requirements

### Requirement: Every client-tier verb shares one contract

Each verb named in this capability SHALL accept the shared client flags (`--server`,
`--token-file`, `--insecure`, `--output`, `--timeout`), SHALL choose its server and credential in
the order `cli-authentication` specifies, SHALL issue exactly one request per invocation with no
retry, SHALL write data to stdout and every diagnostic to stderr, and SHALL map a server
rejection to the exit-code table in the CLI guide.

#### Scenario: Not-found maps to exit 4

- **WHEN** `engram related nope123456` names an id the server reports as not found
- **THEN** the command writes the server's message to stderr, writes nothing to stdout, and exits with code 4

#### Scenario: Server rejection keeps the envelope

- **WHEN** the server rejects a verb's input with `field=scope hint=required`
- **THEN** stderr carries the `field=scope hint=required` envelope unchanged and the command exits with code 2

#### Scenario: JSON and text lanes carry the same value

- **WHEN** a verb runs once with `--output json` and once with `--output text`
- **THEN** the text rendering shows no field the JSON document lacks, and the JSON document mirrors the Connect response's field names

### Requirement: Related memories are read by id

`engram related <id>` SHALL call `RelatedMemories` with the id (full UUID or `short_id`),
optional `--k` and `--full`, and SHALL render the anchor, each related record with its edge type
and evidence, and the `truncated` flag. It SHALL apply the same recall rules the
`related_memories` MCP tool applies.

#### Scenario: Neighbourhood rendered

- **WHEN** the caller owns a record with one supersession edge and two tag edges
- **THEN** the output lists three related records, each with its edge type and evidence, and `truncated` is `false`

#### Scenario: Anchor the caller does not own

- **WHEN** the id names a record owned by another caller and not shared
- **THEN** the command exits with code 4 and never echoes the record's UUID

#### Scenario: Derived states of neighbours

- **WHEN** a neighbour is archived, superseded, expired or scheduled
- **THEN** it is excluded from or included in the neighbourhood exactly as the `related_memories` tool excludes or includes it, with no CLI-specific filter

#### Scenario: Missing id

- **WHEN** `engram related` runs with no id argument
- **THEN** the command writes a usage error naming the id argument to stderr and exits with code 2 without sending a request

### Requirement: Tag counts are read for a scope or every readable scope

`engram tags` SHALL call `ListTags` with optional `--scope` and `--limit`, and SHALL render each
tag with its count, most-used first, plus the `more` flag. Counts SHALL cover recall-visible
records only: never archived, superseded, expired or not-yet-active records, and never another
caller's private records.

#### Scenario: Scope omitted

- **WHEN** `engram tags` runs with no `--scope`
- **THEN** the counts span every scope the caller can read

#### Scenario: Limit above the maximum

- **WHEN** `--limit 1001` is given
- **THEN** stderr carries `field=limit hint=out_of_range` and the command exits with code 2

#### Scenario: Truncated tag set in text mode

- **WHEN** more distinct tags exist than `--limit` allows
- **THEN** text output ends with a line reporting `more: true`, and JSON output carries `"more": true`

### Requirement: Rules are listed as an index or in full

`engram rules` SHALL call `ListRules` with optional repeated `--scope`, repeated `--tag` and
`--full`, and SHALL render the rules oldest-first in the compact index shape by default. With no
`--scope` it SHALL list every readable rule scope and SHALL report the covered scopes the same way
`engram list --cross-spine` reports `searched_scopes`.

#### Scenario: All readable rule scopes

- **WHEN** `engram rules` runs with no `--scope`
- **THEN** output carries every readable rule scope's rules, and text mode appends the `searched_scopes` footer

#### Scenario: Scope with the wrong prefix

- **WHEN** `--scope repo:example` is given
- **THEN** stderr carries `field=scopes hint=prefix` and the command exits with code 2

#### Scenario: Archived rule

- **WHEN** a rule in the scope is archived
- **THEN** it is absent from the output, as it is from `list_rules`

### Requirement: Scheduled records are listed by state

`engram scheduled` SHALL call `ListScheduled` with `--scope` (required unless `--cross-spine`),
optional `--state` (`scheduled` default, `expired`, `all`), `--limit`, `--created-after`,
`--created-before` and `--cursor`, and SHALL return only the caller's own windowed records with
`next_page_token` for paging (the Connect response's field name, which the JSON lane mirrors;
`--cursor` sends it back as `page_token`).

#### Scenario: Scope missing without cross-spine

- **WHEN** `engram scheduled` runs with neither `--scope` nor `--cross-spine`
- **THEN** stderr carries `field=scope hint=conditional_required` and the command exits with code 2

#### Scenario: Unknown state

- **WHEN** `--state later` is given
- **THEN** stderr carries `field=state hint=enum` and the command exits with code 2

#### Scenario: Expired record under the default state

- **WHEN** a record's window has lapsed and `--state` is omitted
- **THEN** the record is absent; it appears only under `--state expired` or `--state all`

### Requirement: Discoveries are searched by query

`engram search-discovery` SHALL call `SearchDiscoveries` with `--query`, `--scope` (a
`discovery:` scope) and optional `--k`, and SHALL render the discoveries with their `kind`,
citations and `score`. Archived and superseded discoveries SHALL be absent, as they are from
`search_discovery`.

#### Scenario: Discovery search

- **WHEN** `engram search-discovery --query "how auth works" --scope discovery:repo:x` runs
- **THEN** stdout carries the matching discoveries in rank order with a `score` on each

#### Scenario: Non-discovery scope

- **WHEN** `--scope repo:x` is given
- **THEN** stderr carries `field=scope hint=prefix` and the command exits with code 2

#### Scenario: Missing query

- **WHEN** `--query` is empty
- **THEN** the command writes a usage error naming `--query` to stderr and exits with code 2 without sending a request

### Requirement: Records are archived and restored by id over Connect

`engram archive <id>...` and `engram restore <id>...` SHALL call `ArchiveMemory` and
`RestoreMemory` with the ids in the given order, and SHALL render one outcome per id in that
order (`archived`, `already_archived`, `restored`, `not_archived`, `not_found`). A record the
caller does not own SHALL read `not_found`. These verbs are distinct from the operator-tier
`spine-review archive` and `spine-review restore`, which stay unchanged.

#### Scenario: Mixed outcomes

- **WHEN** `engram archive a1 b2 c3` names an owned live record, an owned archived record, and a record owned by someone else
- **THEN** the outcomes are `archived`, `already_archived`, `not_found` in that order, the command exits with code 0, and the third record's UUID never appears

#### Scenario: Too many ids

- **WHEN** 1001 ids are given
- **THEN** stderr carries `field=ids hint=too_many` and the command exits with code 2

#### Scenario: No ids

- **WHEN** `engram restore` runs with no id argument
- **THEN** the command writes a usage error to stderr and exits with code 2 without sending a request

### Requirement: An update previews by default

Because `update_memory` is classified destructive (it overwrites content in place), `engram
update <id>` SHALL preview by default, fetching the record and naming the fields that would
change without writing, and SHALL call `UpdateMemory` once only under `--apply`.

#### Scenario: Preview

- **WHEN** `engram update <id> --content "new text"` runs without `--apply` on an owned record
- **THEN** stdout names `content` as the field that would change and shows the current record, nothing is written, and the command exits with code 0

### Requirement: A record is updated in place

`engram update <id>` SHALL build an update mask from the flags given: `--content`, `--summary`,
`--tags` (repeatable; an explicit empty value clears the set), and `--shared` or `--private`. At
least one of these flags SHALL be given. Omitted flags SHALL leave their fields unchanged.

#### Scenario: Content and tags replaced

- **WHEN** `engram update <id> --content "new text" --tags a --tags b --apply` runs on an owned record
- **THEN** the record carries the new content and exactly the tags `a` and `b`, and stdout carries its `id` and `short_id`

#### Scenario: No field flag

- **WHEN** `engram update <id>` runs with none of the field flags
- **THEN** the command writes a usage error naming the field flags to stderr and exits with code 2 without sending a request

#### Scenario: Both visibility flags

- **WHEN** `--shared` and `--private` are both given
- **THEN** the command rejects the combination as a usage error and exits with code 2

#### Scenario: Record the caller does not own

- **WHEN** the id names another caller's record
- **THEN** the command exits with code 4 and never echoes the record's UUID

#### Scenario: Un-sharing a rule

- **WHEN** `--private` targets a rule
- **THEN** the server rejects the call, stderr says to delete the rule instead, and the command exits with code 2

### Requirement: Visibility is set by id

`engram set-visibility <id>` SHALL call `SetVisibility` with exactly one of `--shared` or
`--private`, and SHALL render the record's `id` and `short_id`.

#### Scenario: Shared

- **WHEN** `engram set-visibility <id> --shared` runs on an owned private record
- **THEN** the record becomes readable by every authenticated caller and the command exits with code 0

#### Scenario: Neither flag

- **WHEN** neither `--shared` nor `--private` is given
- **THEN** the command writes a usage error naming both flags to stderr and exits with code 2 without sending a request

#### Scenario: Rule target

- **WHEN** the id names a rule
- **THEN** the server rejects the call, stderr says to delete the rule instead, and the command exits with code 2

### Requirement: A memory is scheduled with a validity window

`engram schedule` SHALL call `ScheduleMemory` with the `engram store` flag set plus
`--not-before` and/or `--not-after` (RFC3339) and optional `--idempotency-key`, and SHALL render
the new record's `id` and `short_id`.

#### Scenario: Deferred reveal

- **WHEN** `engram schedule --content ... --scope ... --source ... --not-before 2027-01-01T00:00:00Z` runs
- **THEN** the record is stored, is hidden from `engram search` and `engram list` until that instant, and is fetchable by `engram get` at once

#### Scenario: No window

- **WHEN** neither `--not-before` nor `--not-after` is given
- **THEN** the command writes a usage error naming both flags to stderr and exits with code 2 without sending a request

#### Scenario: Malformed timestamp

- **WHEN** `--not-after tomorrow` is given
- **THEN** stderr carries `field=not_after hint=format` and the command exits with code 2

#### Scenario: Discovery category

- **WHEN** `--category discovery` is given
- **THEN** stderr carries `field=category hint=enum` and the command exits with code 2

### Requirement: A record supersedes one or more predecessors

`engram supersede` SHALL call `SupersedeMemory` with the `engram store` flag set plus repeated
`--supersedes <id>`, optional `--citation`, optional `--idempotency-key` and `--validate-only`,
and SHALL render the response (`id`, `short_id`, `validated`, `supersedes`, `targets`). With
`--validate-only` it SHALL write nothing on the server and SHALL report the resolved targets or
the exact rejection a real call would produce.

#### Scenario: Validate before merging

- **WHEN** `engram supersede --supersedes a1 --supersedes b2 --validate-only ...` runs and both targets are owned live heads
- **THEN** stdout carries `validated: true` and the two resolved targets, and no record is written

#### Scenario: Rejected set

- **WHEN** one target is a rule, or is owned by another caller, or is already superseded
- **THEN** the server rejects the whole call naming every offending target of one failure class, nothing is written, and the command exits with code 2 for a rule or already-superseded target and code 4 for a not-owned or nonexistent target

#### Scenario: No targets

- **WHEN** `--supersedes` is not given
- **THEN** stderr carries `field=supersedes hint=required` and the command exits with code 2

#### Scenario: Replay with the same key

- **WHEN** the same `--idempotency-key` and identical content are sent twice
- **THEN** the second call returns the first record unchanged and creates no duplicate

### Requirement: A discovery is stored with citations

`engram store-discovery` SHALL call `StoreDiscovery` with `--content`, `--kind` (`map` or
`fact`), repeated `--citation`, `--scope` (a `discovery:` scope), optional `--tags`, `--summary`
and `--id` (replace in place), and SHALL render the `id` and `short_id`.

#### Scenario: Discovery stored

- **WHEN** `engram store-discovery --content ... --kind fact --citation path:line --scope discovery:repo:x` runs
- **THEN** the discovery is stored and stdout carries its `id` and `short_id`

#### Scenario: No citation

- **WHEN** no `--citation` is given
- **THEN** stderr carries `field=citations hint=required` and the command exits with code 2

#### Scenario: Wrong kind

- **WHEN** `--kind guess` is given
- **THEN** stderr carries `field=kind hint=enum` and the command exits with code 2

### Requirement: A rule is stored only on the user's say-so

`engram store-rule` SHALL call `StoreRule` with `--content`, `--scope` (a `rule:repo:` or
`rule:project:` scope), `--summary` (one line), optional `--tags` and `--id`, and SHALL render
the `id` and `short_id`. The verb SHALL carry no automatic promotion path: it runs only when a
person invokes it.

#### Scenario: Rule stored

- **WHEN** `engram store-rule --content ... --scope rule:repo:x --summary "MUST ..."` runs
- **THEN** the rule is stored as always-shared and stdout carries its `id` and `short_id`

#### Scenario: Multi-line summary

- **WHEN** `--summary` contains a newline
- **THEN** stderr carries `field=summary hint=format` and the command exits with code 2

#### Scenario: Wrong scope prefix

- **WHEN** `--scope repo:x` is given
- **THEN** stderr carries `field=scope hint=prefix` and the command exits with code 2

### Requirement: Deleting a record previews by default

`engram delete <id>` SHALL fetch the record and render what would be deleted without deleting
it, and SHALL delete only under `--apply`. The apply path SHALL call `DeleteMemory` once. A record
the caller does not own SHALL read not found on both paths.

#### Scenario: Preview

- **WHEN** `engram delete <id>` runs without `--apply` on an owned record
- **THEN** stdout carries the record's summary with a headline saying it would be deleted, the record still exists, and the command exits with code 0

#### Scenario: Apply

- **WHEN** `engram delete <id> --apply` runs on an owned record
- **THEN** the record is deleted, `engram get <id>` then exits with code 4, and the command exits with code 0

#### Scenario: Not owned

- **WHEN** the id names another caller's record, with or without `--apply`
- **THEN** the command exits with code 4 and never echoes the record's UUID

### Requirement: No verb deletes every record

The CLI SHALL offer no verb that deletes all of a caller's records, and the CLI guide SHALL
state that the `delete_all` MCP tool has no CLI counterpart.

#### Scenario: Catalog carries no delete-all

- **WHEN** an agent reads `engram catalog`
- **THEN** no command's summary or name describes deleting every record

### Requirement: Every new verb is catalogued with its blast radius

`engram catalog` SHALL list each verb in this capability with a blast radius that holds for
every invocation: the five read verbs are read-only; every write verb, `supersede` included even
under `--validate-only`, is not; `archive`, `restore`, `set-visibility` and `update` are
idempotent; `delete` and `update` are destructive (each shares its MCP tool's row) and therefore
preview by default; none is open-world.

#### Scenario: Read verb row

- **WHEN** an agent reads the `related` entry
- **THEN** it has `read_only: true`, `destructive: false`, `idempotent: true`, `open_world: false`

#### Scenario: Delete row

- **WHEN** an agent reads the `delete` entry
- **THEN** it has `read_only: false`, `destructive: true`, `open_world: false`, and lists the `apply` flag

### Requirement: Text renders keep the shared recall footers

A verb whose response carries `searched_scopes`, `scopes_truncated`, `scopes_unknown` or
`recall_gate_hidden` SHALL append the same text-mode footers `engram search` and `engram list`
append, and SHALL carry the fields under the same names in JSON.

#### Scenario: Coverage footer on a cross-spine scheduled listing

- **WHEN** `engram scheduled --cross-spine --output text` runs
- **THEN** output ends with a `searched_scopes:` footer line, as `engram list --cross-spine` prints

#### Scenario: Nothing to report

- **WHEN** a scoped `engram scheduled` or `engram rules --scope` call runs
- **THEN** no footer line is printed, because the response carries no coverage fields and `ListScheduled` / `ListRules` carry no `recall_gate_hidden`
