---
title: Headless CLI Client
description: Use engram search, engram list, and engram store from a shell — an agent with no MCP client, a CI step, or a cron loop — including credentials, TLS, exit codes, and the machine-readable self-describe catalog.
---

`engram` is one binary that is both the server (`engram serve`) and a client for
that server. The three client verbs — `engram search`, `engram list`, and
`engram store` — let anything with a shell talk to a **remote** engram server
over the Connect API, with no MCP client involved: a subagent with a closed
tool list, a CI step, or a cron loop.

The server must be running with the Connect lane mounted (`connect.headless:
true`, or a UI-enabled deployment) — see [Configuration](/guides/configure/).

## The three verbs

| Command | Purpose |
|---------|---------|
| `engram search --server <url> --query <text> --scope <scope>` | Vector search over stored memories |
| `engram list --server <url> --scope <scope>` | Paged, filtered recall |
| `engram store --server <url> --content <text> --scope <scope>` | Write a memory (the only write verb) |

Run `engram <verb> --help` for the full flag list of each command — every
flag mirrors a field on the corresponding Connect request message.

## Shared flags

All three commands accept the same five flags, each resolved flag-then-
environment-then-default through the same `internal/config` registry the
server side uses:

| Flag | Purpose |
|------|---------|
| `--server` | Server base URL. Falls back to `ENGRAM_SERVER_URL` if unset. Required (from one or the other) — there is no localhost default. |
| `--token-file` | Path to a file containing the bearer credential. Falls back to `ENGRAM_TOKEN` (env wins over file if both are set). |
| `--insecure` | Skip TLS certificate verification. Always prints an unconditional warning to stderr — this cannot be suppressed and **has no environment fallback**, deliberately: it cannot be silently enabled by an inherited environment variable. |
| `--output` | Force `"json"` or `"text"`. Default: JSON when stdout is not a terminal, a human table when it is. |
| `--timeout` | Bounds the RPC call. Falls back to `ENGRAM_TIMEOUT`. Default `30s`. `0` is rejected as a usage error — it does not mean unbounded. See [Request timeout](#request-timeout) below. |

**Credential precedence, in order:** `ENGRAM_TOKEN` environment variable, then
the file named by `--token-file`. **There is no `--token` flag.** This is
deliberate: a credential must never be able to reach `argv`, `ps` output, or
shell history. Omitting both is legal — an anonymous call against a
no-issuer server is a normal request, not an error.

## Recall scope selection

`engram search` and `engram list` each require exactly one of two flags — there
is no default that silently picks one for you:

| Flag | Purpose |
|------|---------|
| `--scope` | Limit recall to one scope; <!-- engram:rule:start scope-required-unless-cross-spine -->scope is required unless cross_spine is true<!-- engram:rule:end scope-required-unless-cross-spine -->; omit and pass `--cross-spine` to span every scope you can read; mutually exclusive with `--cross-spine`. |
| `--cross-spine` | Span every scope you can read; mutually exclusive with `--scope`. |

Passing neither, or passing both, is rejected by the CLI itself — before any
network call — with exit `2`. The server would in fact accept `--scope`
together with `cross_spine=true`, silently discarding the scope and logging
the discard at Info on the server side, where the calling agent never sees
it. The CLI is deliberately stricter than the server on this one combination:
an explicitly-typed filter being discarded without the caller learning about
it is exactly the kind of surprise this interface is designed to avoid, so
the client rejects the pair outright rather than forwarding it.

## Paging `engram list`

`engram list` supports two independent paging styles — offset-for-UI and
cursor paging — never combined:

| Flag | Purpose |
|------|---------|
| `--offset` | Offset-for-UI paging; <!-- engram:rule:start paging-trio-mutually-exclusive -->cursor_mode, offset, and page_token are mutually exclusive<!-- engram:rule:end paging-trio-mutually-exclusive -->. |
| `--cursor-mode` | Opt into cursor paging on the first (tokenless) page; mutually exclusive with `--offset` and `--page-token`. |
| `--page-token` | Opaque cursor from a previous response's `next_page_token`; mutually exclusive with `--offset` and `--cursor-mode`. |

Passing more than one of the three is rejected by the CLI itself — before any
network call — with exit `2`, via a declared cobra flag group (the same
mutual-exclusion enforcement mechanism as `--scope`/`--cross-spine` above).

## Output contract

Data goes to stdout as one JSON object per invocation, mirroring the Connect
response's own field names (`memories`, `total`, `next_page_token`, `id`,
`short_id`, ...). Every diagnostic — warnings, the `--insecure` notice,
errors — goes to stderr, so `engram search ... | jq` is always safe. An empty
result set is a success: `engram search` and `engram list` exit `0` with
`"memories":[]`, never `null`.

On a `--cross-spine` call, text-mode output for both `engram search` and
`engram list` appends a coverage footer after the table:

```text
searched_scopes: 3
```

or, when the server reports the authorized span was truncated:

```text
searched_scopes: 3  scopes_truncated: true
```

The footer reports a **count** of the scopes searched, never the scope names
themselves. It prints only on a `--cross-spine` call — output for every other
invocation is unchanged, byte-for-byte, from before this capability existed.
The JSON lane already carried `searched_scopes` and `scopes_truncated` on
every response before this release and is unaffected by this change.

### Operator commands

Every operator command — `reindex`, `prune-expired`, `summarize-missing`,
`backfill-short-ids`, `migrate-remap-owner`, its deprecated alias
`migrate-set-owner`, and every `engram spine-review` leaf (currently `scan`) —
also accepts `--output`:

| Value | Behavior |
|-------|----------|
| `json` | Write exactly one JSON document to stdout. |
| `text` | Write the pre-existing human-readable summary line, unchanged. |
| *(absent)* | Detect from the command's own configured output writer: a human
terminal renders `text`; anything else (a pipe, a file redirect) renders `json`. |
| anything else | Rejected as a usage error (exit `2`), naming `--output` and its
legal values — the same validator the three client verbs use. |

As with the client tier, the JSON document goes to stdout and any warning or
diagnostic goes to stderr, so `engram <operator-command> --output json | jq .`
is always safe. A sweep that affected zero records still emits zero-valued
counters and `[]` for any list-shaped field — never `null` — and exits `0`.
Every fact an operator command's `text` line states also appears as a field
in its `json` document; a preview is always distinguished from an applied
mutation by an explicit boolean field plus separate count fields, never by
prose alone.

### Destructive commands

<!-- engram:rule:start destructive-requires-apply -->a destructive operator command previews by default and mutates only when apply is set<!-- engram:rule:end destructive-requires-apply -->.
This applies to every command the [blast-radius](#blast-radius) table below
classifies `destructive`: today, `prune-expired` and `migrate-remap-owner`. A
bare invocation reports what the sweep *would* do and exits `0` without
touching the collection; add `--apply` to perform the mutation. A forgotten
`--apply` is therefore a harmless no-op — the command just previews again.

Every other mutating operator command — `reindex`, `summarize-missing`, and
`backfill-short-ids` — is classified **non-destructive** and keeps its
pre-existing opt-in **preview** idiom, `--dry-run`. This is a deliberate,
two-idiom split, not an accident: on a destructive command a forgotten
`--apply` costs nothing (it just previews again), but on a non-destructive
command a forgotten `--dry-run` merely performs the recoverable, additive
thing the operator already asked for. The boundary sits exactly on the
blast-radius table's `Destructive` column and nowhere else.

## Exit codes

Every command in the binary — the three client verbs and all seven operator
commands (`serve`, `reindex`, `prune-expired`, `summarize-missing`,
`backfill-short-ids`, `migrate-remap-owner`, and its deprecated alias
`migrate-set-owner`) — resolves through the same seven codes:

| Code | Meaning |
|------|---------|
| 0 | Success (including an empty result set) |
| 1 | Unclassified internal error — a backstop, not a general-purpose failure code. Reached by exactly two paths (see caution below). |
| 2 | Usage or validation error — a bad flag value, a violated mutually-exclusive flag group, or engram's own semantic validation (a missing `--server`/`ENGRAM_SERVER_URL`, an invalid `--output` value, an empty required flag) |
| 3 | Authentication or authorization failure |
| 4 | Not found |
| 5 | Transport or server unavailable |
| 6 | Request deadline exceeded — the server accepted the request but did not answer within `--timeout` |

:::caution[Only two paths still exit 1]
Framework flag errors — an unknown flag, an unparseable flag value — and a
violated mutually-exclusive flag group all exit `2` now, the same code as
engram's own semantic validation. Previously published guidance said a flag
typo exits `1`, "not `2`"; that guidance is **retracted**. Only two paths
still exit `1`:

- A **mistyped verb** (`engram serach`), rejected during cobra's own command
  resolution before any engram hook runs.
- A **genuinely unclassified internal error**, including `serve`'s own
  `ListenAndServe()` call failing to bind (for example, "address already in
  use"). This one is deliberate, not an oversight: exit `5` means "the
  remote server or Qdrant is unreachable" everywhere else in this taxonomy,
  and a local OS bind failure is a different condition — force-mapping it
  onto `5` would make `5` ambiguous for any caller scripting both `serve`
  and a client verb.

See the
[upgrade guide](/guides/upgrade/#1-framework-flag-errors-now-exit-2-not-1)
for the full migration note.
:::

## Request timeout

Every client verb (`search`, `list`, `store`) bounds its RPC call with
`--timeout` (or `ENGRAM_TIMEOUT`), default `30s`. `0`, a negative value, and
a malformed duration are all rejected as usage errors (exit `2`) **before**
any dial — `--timeout 0` together with an unreachable `--server` still exits
`2`, not `5`. A server that accepts the connection but never answers within
the window reports exit `6`, distinct from exit `5` (the server refused the
connection, or was never reachable at all).

**The operator commands' own `--timeout` is a different flag with different
zero-semantics, and it is not uniform across commands:**

| Commands | `--timeout` meaning | `0` behavior |
|---|---|---|
| `search`, `list`, `store` | Per-RPC-call deadline | **Rejected** (usage error) |
| `reindex`, `prune-expired`, `summarize-missing`, `backfill-short-ids`, `spine-review scan` | Whole-sweep wall-clock budget | Disables the deadline (unbounded), unchanged |
| `migrate-remap-owner`, `migrate-set-owner` | Whole-sweep wall-clock budget | **Rejected** (usage error) — changed this release, see the [upgrade guide](/guides/upgrade/#6-migrate-remap-owner---timeout-0--migrate-set-owner---timeout-0-no-longer-means-unbounded) |

A reader comparing `engram search --help` against `engram reindex --help`
and `engram migrate-remap-owner --help` side by side should not have to
infer this table from the flag's one-line usage text — the three groups
genuinely disagree on what `--timeout 0` does.

## The self-describe catalog

Run `engram` with no arguments. It writes one JSON document to stdout and
exits `0`: every command in the live binary, every flag with its type,
default, and usage, and this same exit-code table — derived from the running
binary's actual command tree and exit-code constants, not maintained by
hand. `engram --help` is unaffected and still prints ordinary human help.

This catalog is the **machine-readable, authoritative** form of everything on
this page. When the two disagree, trust the binary — this page is prose
describing it, not the contract itself.

```sh
engram | jq '.commands[] | select(.name == "search")'
```

### Blast radius

Every command in the catalog carries a `blast_radius` object:

```json
"blast_radius": {
  "read_only": true,
  "destructive": false,
  "idempotent": true,
  "open_world": false
}
```

This is the same taxonomy the [MCP tool annotations](/reference/tools/#blast-radius) publish — `readOnlyHint`, `destructiveHint`, `idempotentHint`, and `openWorldHint` — read from the same table, so an agent that has already learned to branch on one lane's hints applies the identical logic on the other. The JSON keys are snake_case rather than the MCP wire's camelCase, so the two forms rhyme without being byte-identical.

Each hint takes the conservative stance: a value is `true` only if it holds under **every** valid invocation of that command, not merely the common case. `migrate-remap-owner`, for example, is `"destructive": true` because its `--from <value>` form can overwrite an existing, non-empty owner — even though its `--from-missing` form only ever fills an empty one.

This classification is a discoverability aid, not an authorization mechanism — it tells an agent what to expect, it does not gate what the agent is allowed to run.

## See also

- [Configuration](/guides/configure/) — `connect.headless` and the other
  server-side settings the Connect lane depends on.
