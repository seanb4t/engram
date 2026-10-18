# session-primer Specification

## Purpose

Defines `engram prime`, the session primer that gives an agent its memory scope, the repository's rules index and a bounded recall digest in one read, and how the plugin's session-start hook uses it and falls back when it cannot.

## Requirements

### Requirement: Prime emits a session primer

`engram prime` SHALL write a session primer to stdout with, in this order: a scope header, a rules index, a recall digest and a pointer to the machine-readable command catalog. It SHALL only read: it SHALL NOT write to the store, start a login, prompt, or read stdin.

#### Scenario: Primer in a repository with rules and memories

- **WHEN** `engram prime` runs in a clone whose `origin` remote is `https://github.com/org/repo.git`, and the caller has rules in `rule:repo:github.com/org/repo` and memories in `repo:github.com/org/repo`
- **THEN** stdout holds the scope header, then the rules index, then the recall digest, then a line naming the catalog command, and the command exits 0

#### Scenario: Prime never waits for input

- **WHEN** `engram prime` runs with stdin attached to an open pipe that never closes
- **THEN** the command completes without reading stdin

### Requirement: Scope header states the scope and its source

The primer SHALL start with a line `Memory spine scope: <scope>` and a line naming the source of that scope: `--scope`, `ENGRAM_SCOPE`, `git origin remote`, or the repository-directory-name fallback. When an overlay is derived, it SHALL add `Memory workspace scope: <scope>` and its source. It SHALL name the rule scopes it read.

#### Scenario: Scope derived from the origin remote

- **WHEN** `engram prime` runs without `--scope` or `ENGRAM_SCOPE` in a clone whose `origin` remote is `git@github.com:org/repo.git`
- **THEN** the header contains `Memory spine scope: repo:github.com/org/repo` and names `git origin remote` as its source

#### Scenario: Directory-name fallback is flagged

- **WHEN** `engram prime` runs without an override in a clone named `myrepo` that has no `origin` remote
- **THEN** the header contains `Memory spine scope: repo:myrepo`, names the repository directory name as its source, and states that this is probably not a scope on the server

#### Scenario: Linked worktree

- **WHEN** `engram prime` runs without an override in the linked worktree `feat` of that clone
- **THEN** the header also contains `Memory workspace scope: repo:github.com/org/repo:ws:feat` and names the linked worktree as its source

### Requirement: Scope override precedence

`--scope` SHALL take precedence over `ENGRAM_SCOPE`, and either SHALL take precedence over the derived scope. An overridden scope SHALL be used exactly as given, with no overlay derived. When the scope has the form `repo:<repo-id>[...]`, the repository rule scope SHALL be `rule:repo:<repo-id>`.

#### Scenario: Flag wins over the environment

- **WHEN** `ENGRAM_SCOPE` is `repo:a/b` and `engram prime --scope repo:c/d` runs in a linked worktree
- **THEN** the spine scope is `repo:c/d`, its source is `--scope`, and the primer has no workspace scope

#### Scenario: Environment variable alone

- **WHEN** `ENGRAM_SCOPE` is `repo:a/b` and `engram prime` runs without `--scope`
- **THEN** the spine scope is `repo:a/b` and its source is `ENGRAM_SCOPE`

#### Scenario: Override names a workspace scope

- **WHEN** `engram prime --scope repo:github.com/org/repo:ws:feat` runs
- **THEN** the spine scope is `repo:github.com/org/repo:ws:feat` and the repository rule scope is `rule:repo:github.com/org/repo`

#### Scenario: Override outside a repository

- **WHEN** `engram prime --scope repo:github.com/org/repo` runs outside any git repository
- **THEN** the primer uses `repo:github.com/org/repo` and includes the rules index and the recall digest

### Requirement: Rejected scope values

Before any server call, `engram prime` SHALL reject an empty or blank `--scope`, and a `--scope` or `ENGRAM_SCOPE` that starts with `rule:` or `discovery:`, with exit code 2 and a message that names the flag or variable. A scope the server rejects SHALL be reported with the server's `field=` and `hint=` envelope unchanged.

#### Scenario: Explicitly empty flag

- **WHEN** `engram prime --scope ""` runs
- **THEN** the command exits 2, its stderr names `--scope`, and stdout is empty

#### Scenario: Rule scope passed as the memory scope

- **WHEN** `engram prime --scope rule:repo:github.com/org/repo` runs
- **THEN** the command exits 2, its stderr names `--scope` and says that the rule scope is derived from the memory scope, and no server call is made

#### Scenario: Rule scope in the environment

- **WHEN** `ENGRAM_SCOPE` is `discovery:repo:github.com/org/repo` and `engram prime` runs without `--scope`
- **THEN** the command exits 2 and its stderr names `ENGRAM_SCOPE`

#### Scenario: Server rejects the scope

- **WHEN** the server rejects the scope that `engram prime` sends with `field=scope` and a hint code
- **THEN** stderr carries that `field=scope hint=<code>` envelope unchanged, stdout is empty, and the command exits 2

### Requirement: Rules index

The primer SHALL list every rule in `rule:repo:<repo-id>` and, when `ENGRAM_PROJECT` is set, in `rule:project:<ENGRAM_PROJECT>`, one line per rule with its full `short_id`, its summary and its tags, and no rule content. When no rule is found, it SHALL omit the rules section.

#### Scenario: Repository and project rules

- **WHEN** `ENGRAM_PROJECT` is `selfhosted-cluster` and both rule scopes hold rules
- **THEN** the rules index has one line per rule from both scopes, each with the rule's full `short_id`, summary and tags

#### Scenario: No rules

- **WHEN** neither rule scope holds a rule
- **THEN** the primer has no rules section

### Requirement: Recall digest

The primer SHALL list, per scope, up to the 10 most recent records that recall shows the caller, each as one line with its full `short_id` and its compact summary, grouped as repo-wide and workspace records. A record in both groups SHALL appear only in the repo-wide group. A full group SHALL end with a note that older records exist. An empty group SHALL be omitted.

#### Scenario: Full spine

- **WHEN** the spine scope holds 25 records the caller can recall
- **THEN** the repo-wide group lists the 10 most recent, newest first, and ends with a note that older records are reachable through search or a higher list limit

#### Scenario: Derived states are not shown

- **WHEN** the spine scope holds an archived record, a superseded record, an expired record and a not-yet-active scheduled record
- **THEN** none of the four appears in the digest

#### Scenario: Records the caller does not own

- **WHEN** the spine scope holds another caller's `shared` record and another caller's `private` record
- **THEN** the `shared` record can appear in the digest and the `private` record never does

#### Scenario: Empty workspace overlay

- **WHEN** the overlay scope holds no record the caller can recall
- **THEN** the primer has no workspace group

### Requirement: Prime outside a repository

When no scope override is given and no scope can be derived, `engram prime` SHALL write a primer that states that no scope was derived and that `--scope` sets one, SHALL make no server call, and SHALL exit 0.

#### Scenario: Not a git repository

- **WHEN** `engram prime` runs in `/tmp/scratch` without `--scope` or `ENGRAM_SCOPE`
- **THEN** stdout states that no scope was derived and names `--scope`, there is no rules index and no recall digest, no server call is made, and the command exits 0

### Requirement: Primer output format

`engram prime` SHALL write the text primer by default, whether or not stdout is a terminal. With `--output json` it SHALL write one JSON document that holds the same facts: each scope with its source, the rule scopes, the rules and the digest records. Any other `--output` value SHALL be rejected with exit code 2.

#### Scenario: Piped output stays text

- **WHEN** `engram prime | cat` runs in a repository
- **THEN** the output is the text primer, not JSON

#### Scenario: JSON on request

- **WHEN** `engram prime --output json` runs in a repository
- **THEN** stdout is one JSON document with the spine scope and its source, the overlay scope and its source when one exists, the rule scopes, the rules and the digest records

#### Scenario: Unknown format

- **WHEN** `engram prime --output yaml` runs
- **THEN** the command exits 2 and stderr names `--output`

### Requirement: Prime failures

When a server call fails, `engram prime` SHALL write nothing to stdout, SHALL write the error to stderr, and SHALL exit with the CLI's exit code for that failure class. It SHALL resolve its server and credential the same way as every other client command, and SHALL NOT start a login.

#### Scenario: No credential for a server that requires one

- **WHEN** `engram prime` runs in a repository with no credential against a server that enforces OIDC
- **THEN** stdout is empty, stderr reports the authentication failure, and the command exits 3

#### Scenario: Server unreachable

- **WHEN** the server cannot be reached
- **THEN** stdout is empty and the command exits 5

#### Scenario: Deadline exceeded

- **WHEN** the server does not answer within `--timeout`
- **THEN** stdout is empty and the command exits 6

#### Scenario: No server configured

- **WHEN** `engram prime` runs in a repository with no `--server`, no `ENGRAM_SERVER_URL` and no other configured server
- **THEN** stdout is empty, stderr names `--server` and `ENGRAM_SERVER_URL`, and the command exits 2

### Requirement: Session-start hook uses the primer

In a git repository, when the `engram` binary is on `PATH`, the session-start hook SHALL run `engram prime` from the session's working directory with a bounded timeout. When prime exits 0 with output, the hook SHALL inject that primer together with its guidance on store precedence and memory capture, and SHALL NOT tell the agent to call `list_memory` or `list_rules` at startup.

#### Scenario: Primer available

- **WHEN** a session starts in a repository, `engram` is on `PATH`, and `engram prime` exits 0 with a primer
- **THEN** the injected context holds that primer, including its `Memory spine scope:` line, and the store-precedence and capture guidance, and holds no instruction to call `list_memory` or `list_rules`

### Requirement: Session-start hook falls back

The hook SHALL inject today's instruction text, with scopes from its own derivation, when the `engram` binary is missing, when `engram prime` exits non-zero, when it does not finish within the hook's prime timeout, or when it writes nothing. The hook SHALL finish within its configured time limit and SHALL NOT fail the session start. Outside a git repository it SHALL inject nothing, as before.

#### Scenario: Binary missing

- **WHEN** a session starts in a repository and `engram` is not on `PATH`
- **THEN** the injected context is the instruction text that tells the agent to call `list_memory` and `list_rules`, with the hook's derived scopes

#### Scenario: No credential

- **WHEN** `engram prime` exits 3 because no credential is available
- **THEN** the hook injects the instruction text

#### Scenario: Older binary without prime

- **WHEN** the installed `engram` has no `prime` command and exits non-zero
- **THEN** the hook injects the instruction text

#### Scenario: Prime does not finish in time

- **WHEN** `engram prime` has not finished when the hook's prime timeout expires
- **THEN** the hook stops waiting for it, injects the instruction text, and finishes within the hook's configured time limit

#### Scenario: Outside a repository

- **WHEN** a session starts outside any git repository
- **THEN** the hook injects nothing and exits 0
