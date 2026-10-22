---
title: Headless CLI Client
description: Use the engram client verbs — search, list, get, store and the curation verbs that mirror every MCP tool — from a shell with no MCP client, including credentials, TLS, exit codes, the read-only terminal console, and the machine-readable catalog that engram catalog prints.
---

`engram` is one binary that is both the server (`engram serve`) and a client for
that server. The client verbs — `engram search`, `engram list`, `engram get`,
`engram store` and the curation verbs listed below — let anything with a shell
talk to a **remote** engram server over the Connect API, with no MCP client
involved: a subagent with a closed tool list, a CI step, or a cron loop. Every
MCP tool but `delete_all` has a client verb that reaches the same server core.

Get the binary through [Install](/guides/install/). To register an MCP connection
for an agent, follow [Agent Setup](/guides/agent-setup/) and its release
availability notice. MCP setup uses `--url` / `ENGRAM_URL`; the Connect commands
below use `--server` / `ENGRAM_SERVER_URL` and the credential precedence described
here. Setup's generic-only `--token-file` behavior does not apply to these
Connect commands.

The server must be running with the Connect lane mounted (`connect.headless:
true`, or a UI-enabled deployment) — see [Configuration](/guides/configure/).

## The client verbs

Each verb is the CLI counterpart of one MCP tool and calls the same Connect
RPC the tool's server core sits behind, so a rejection carries the same
`field=<name> hint=<code>` envelope on both lanes (see the
[error reference](/reference/errors/)). A verb that takes an id accepts a
full UUID or a `short_id`.

**Reads**

| Command | MCP tool | Purpose |
|---------|----------|---------|
| `engram search --query <text> --scope <scope>` | `search_memory` | Vector search over stored memories |
| `engram list --scope <scope>` | `list_memory` | Paged, filtered recall |
| `engram get <id>` | `get_memory` | One record's full state, recall-gated or not |
| `engram related <id>` | `related_memories` | One record's neighbourhood: supersession, shared tags, shared citations, vector neighbours; `--k`, `--full` |
| `engram tags [--scope <scope>]` | `list_tags` | Exact tag counts, most-used first; omit `--scope` for every readable scope; `--limit` |
| `engram rules [--scope <rule-scope>...]` | `list_rules` | The complete rule set, oldest-first; omit `--scope` for every readable rule scope; `--tag`, `--full` |
| `engram scheduled --scope <scope>` | `list_scheduled` | Your windowed records the recall gate is hiding; `--state scheduled \| expired \| all`, `--cross-spine`, `--cursor` |
| `engram search-discovery --query <text> --scope discovery:...` | `search_discovery` | Semantic search over the discovery pool |

**Writes that preserve or reverse**

| Command | MCP tool | Purpose |
|---------|----------|---------|
| `engram store --content <text> --scope <scope>` | `store_memory` | Write a memory |
| `engram schedule --content <text> --scope <scope> --not-before <rfc3339>` | `schedule_memory` | Write a memory with a validity window (`--not-before` and/or `--not-after`); `--idempotency-key` |
| `engram supersede --content <text> --scope <scope> --supersedes <id>...` | `supersede_memory` | Correct one or more records with a new one, history preserved; `--validate-only` previews the server's preflight without writing; `--citation kind:ref`, `--idempotency-key` |
| `engram archive <id>...` / `engram restore <id>...` | `archive_memory` / `restore_memory` | Retire or un-retire records you own; one outcome per id, in order; a record you do not own reads `not_found` |
| `engram set-visibility <id> --shared \| --private` | `set_visibility` | Share or unshare a record you own; rejected for a rule |
| `engram store-discovery --content <text> --kind map \| fact --citation kind:ref --scope discovery:...` | `store_discovery` | Store a citation-backed discovery; `--id` replaces in place |
| `engram store-rule --content <text> --scope rule:... --summary <line>` | `store_rule` | Store a user-blessed, always-shared rule; `--id` replaces in place. Run it only when a person asks for it: nothing promotes a rule on its own |

**Destructive writes** — preview by default, write under `--apply` (see
[Destructive commands](#destructive-commands))

| Command | MCP tool | Purpose |
|---------|----------|---------|
| `engram update <id> --content <text> \| --summary <text> \| --tags <t> \| --shared \| --private` | `update_memory` | Replace the named fields in place; the flags given build the update mask, `--tags ""` clears the set |
| `engram delete <id>` | `delete_memory` | Delete one record you own |

The `archive`, `restore` and `set-visibility` verbs act on the caller's own
records through the server's authorization, unlike the operator-tier
`spine-review archive` / `spine-review restore`, which talk to Qdrant directly.

Run `engram <verb> --help` for the full flag list of each command — every
flag mirrors a field on the corresponding Connect request message.

## Shared flags

Every client verb accepts the same five flags, each resolved flag-then-
environment-then-default through the same `internal/config` registry the
server side uses:

| Flag | Purpose |
|------|---------|
| `--server` | Server base URL. Falls back to `ENGRAM_SERVER_URL`, then to the default server that the last [`engram login`](#log-in-with-engram-login) set. With none of the three, the command exits `2` — there is no localhost default. |
| `--token-file` | Path to a file containing the bearer credential. Falls back to `ENGRAM_TOKEN` (env wins over file if both are set). |
| `--insecure` | Skip TLS certificate verification. Always prints an unconditional warning to stderr — this cannot be suppressed and **has no environment fallback**, deliberately: it cannot be silently enabled by an inherited environment variable. |
| `--output` | Force `"json"` or `"text"`. Default: JSON when stdout is not a terminal, a human table when it is. |
| `--timeout` | Bounds the RPC call. Falls back to `ENGRAM_TIMEOUT`. Default `30s`. `0` is rejected as a usage error — it does not mean unbounded. See [Request timeout](#request-timeout) below. |

**Credential precedence.** A client command uses the first of these that is
set, and does not read the ones after it:

1. The `ENGRAM_TOKEN` environment variable.
2. The file that `--token-file` names.
3. The saved login for the server (see
   [Log in with `engram login`](#log-in-with-engram-login)).

**There is no `--token` flag.** This is deliberate: a credential must never be
able to reach `argv`, `ps` output, or shell history. Having none of the three is
legal. An anonymous call against a server with no issuer is a normal request,
not an error.

## Log in with `engram login`

`engram login` gets a credential for one server and saves it, so the client
commands work with no token handling. It works over SSH and on a machine with no
browser.

```sh
engram login --server https://engram.example.com
```

1. The command reads the server's
   [protected-resource metadata](/reference/auth/#protected-resource-metadata-rfc-9728)
   to find its authorization server (the IdP).
2. The first time you log in to a server from a machine, the command registers a
   client at the IdP through dynamic client registration (RFC 7591). Later logins
   to the same server reuse that client.
3. The command prints a URL and a code on stderr. Open the URL in a browser on
   any device, sign in, and approve the login.
4. The command saves the credential and makes this server the default server.

The login asks for the `offline_access` scope, so the IdP issues a refresh token:
you log in once per machine, not once per access-token lifetime. Each client
command refreshes the access token before its request when the token has
expired, and saves the new one.

| Flag | Purpose |
|------|---------|
| `--server` | The server to log in to. Falls back to `ENGRAM_SERVER_URL`. Required. |
| `--client-id` | Use a client that an administrator registered at the IdP in advance, for an IdP without dynamic client registration. Falls back to `ENGRAM_CLIENT_ID`. |
| `--insecure` | Skip TLS certificate verification for the engram server only. Requests to the IdP are always verified. The server's metadata names the IdP, and that metadata now comes over an unverified connection, so the command prints a warning that names the IdP: check that the URL you open is on the IdP host you expect before you approve. |

The IdP and every endpoint it advertises must use `https`. The command accepts
plain `http` only on a loopback host, such as a local test IdP.

**Where login keeps the credential.** In
`$XDG_CONFIG_HOME/engram/credentials.json`, or in
`~/.config/engram/credentials.json` when `XDG_CONFIG_HOME` is not set, on every
platform. The file holds one login per server, and only you can read it.
If other users can read the file, a client command refuses it, exits `2`, and
names the `chmod` command that fixes it.

**Log out.** `engram logout --server <url>` revokes the refresh token at the
IdP, deletes the client that `engram login` registered (never a `--client-id`
client), and removes the saved login. With neither `--server` nor
`ENGRAM_SERVER_URL` it logs out of the default server. If the IdP cannot be
reached, the command prints a warning and removes the saved login anyway.

**Exit codes.**

| Code | `engram login` | A client command that uses a saved login |
|------|----------------|------------------------------------------|
| `2` | No server is given, the server names no authorization server (it accepts anonymous calls), or other users can read the saved-login file. | Other users can read the saved-login file. |
| `3` | The IdP refuses the login: you denied it, the code expired, the IdP has no device grant, or it has no registration and no `--client-id` is set. | The IdP rejects the refresh. Run `engram login` again. |
| `5` | The server or the IdP does not answer. | The IdP does not answer. |

When a client command exits `3` or `5` because of its saved login, it does not
call the engram server.

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
| `--limit` | Max results per page; `0` resolves to the maximum, `1000`; a value above `1000` is rejected. |
| `--offset` | Offset-for-UI paging; <!-- engram:rule:start paging-trio-mutually-exclusive -->cursor_mode, offset, and page_token are mutually exclusive<!-- engram:rule:end paging-trio-mutually-exclusive -->. |
| `--cursor-mode` | Opt into cursor paging on the first (tokenless) page; mutually exclusive with `--offset` and `--page-token`. |
| `--page-token` | Opaque cursor from a previous response's `next_page_token`; mutually exclusive with `--offset` and `--cursor-mode`. |

Passing more than one of the three paging-mode flags is rejected by the CLI
itself — before any network call — with exit `2`, via a declared cobra flag
group (the same mutual-exclusion enforcement mechanism as
`--scope`/`--cross-spine` above). `engram search --k` carries the same
contract as `--limit` above except for its own zero-value default: `0`
resolves to `20`, and a value above `1000` (the shared maximum) is rejected.
Either rejection surfaces as exit `2` (see [Exit codes](#exit-codes)).

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

or, when the server's own coverage-enumeration query failed after hits were
already found — the call still succeeds, but there is no count to report:

```text
scopes_unknown: true
```

The footer reports a **count** of the scopes searched, never the scope names
themselves — except the `scopes_unknown` form, which carries no count
because there is none. It prints only on a `--cross-spine` call — output for
every other invocation is unchanged, byte-for-byte, from before this
capability existed. The JSON lane already carried `searched_scopes` and
`scopes_truncated` on every response before this release, now joined by
`scopes_unknown`, and is unaffected by this change beyond that addition.

Text-mode output for both commands also appends a recall-gate hidden-count
footer, after the coverage footer, whenever the response reports hidden
records:

```text
recall_gate_hidden: 5  archived: 2  superseded: 2  expired: 1  scheduled: 1
```

This one prints on **any** `engram search`/`engram list` text call, not only
`--cross-spine` — a scope-confined call can hide records too. It prints
nothing when no records were hidden or the count is unavailable, and the
JSON lane carries `recall_gate_hidden` under the same name (`{total,
archived, superseded, expired, scheduled}`), so this addition needs no
separate rendering code there.

When the server runs the Jev reranker (`ENGRAM_SEARCH_RANKER=jev`, see
[Search reranking (Jev)](/guides/configure/#search-reranking-jev)),
`engram search` text output gains a RELEVANCE column and JSON output gains a
per-memory `relevance` field — the provider's probability, 0 to 1, that the
record answers the query; values all near zero mean nothing returned answers
it. Without it, output is unchanged byte for byte. `engram list` output
never carries relevance.

### Operator commands

Every operator command — `reindex`, `prune-expired`, `summarize-missing`,
`backfill-short-ids`, `migrate` (plus its `status` and `revert` subcommands;
see the [Migrate guide](/guides/migrate/)), `migrate-remap-owner`, its
deprecated alias `migrate-set-owner`, `setup`, and every `engram spine-review`
leaf (currently `scan`, `verify`, `consolidate`, `archive`, `restore`, and
`purge`) — also accepts `--output`:

| Value | Behavior |
|-------|----------|
| `json` | Write exactly one JSON document to stdout. |
| `text` | Render a one-line prose headline followed by one aligned line per field of the same document `json` emits. This is a human-readable view, not a stable interface, and is not intended to be parsed. |
| *(absent)* | Detect from the command's own configured output writer: a human terminal renders `text`; anything else (a pipe, a file redirect) renders `json`. |
| anything else | Rejected as a usage error (exit `2`), naming `--output` and its legal values — the same validator the client verbs use. |

As with the client tier, the JSON document goes to stdout and any warning or
diagnostic goes to stderr, so `engram <operator-command> --output json | jq .`
is always safe. A sweep that affected zero records still emits zero-valued
counters and `[]` for any list-shaped field — never `null` — and exits `0`.
Both lanes derive from **one serialization**: the `json` document is produced
by `encoding/json` over the command's own report struct, and `text` is a
rendered view of that same value — never a second, independently maintained
format. The `json` document cannot carry a fact `text` omits, and `text`
cannot show a field `json` withheld, because both are read from the same
value. The `json` lane is the contract; the `text` lane may change shape in
any release. A preview is always distinguished from an applied mutation by an
explicit boolean field plus separate count fields, never by prose alone.

### Destructive commands

<!-- engram:rule:start destructive-requires-apply -->a mutating operator command previews by default and mutates only when apply is set<!-- engram:rule:end destructive-requires-apply -->.
This applies to every command routed through the `registerDestructive` choke
point — a wider set than the [blast-radius](#blast-radius) table's
`destructive` column alone. Today that is every command the table classifies
`destructive` (`prune-expired`, `migrate-remap-owner`, `spine-review purge`,
`migrate revert`, and the two client verbs `update` and `delete`, which share
`update_memory`'s and `delete_memory`'s rows), **plus `migrate`**, which is classified
**non-destructive** (its sweep is additive-only) yet still previews by
default and mutates only under `--apply` — the same contract, extended to a
mutating-but-not-destructive command. `migrate status` is read-only and
carries no `--apply` flag at all. A bare invocation of any `--apply`-gated
command reports what the sweep *would* do and exits `0` without touching the
collection; add `--apply` to perform the mutation. A forgotten `--apply` is
therefore a harmless no-op — the command just previews again.

`reindex` and `summarize-missing` are classified **non-destructive** and
keep their pre-existing opt-in **preview** idiom, `--dry-run`. This is a
deliberate two-idiom split, not an accident: on a command routed through
`registerDestructive` (above) a forgotten `--apply` costs nothing (it just
previews again), but on `reindex`/`summarize-missing` a forgotten
`--dry-run` merely performs the recoverable, additive thing the operator
already asked for.

`summarize-missing --scope <scope>` (or `--all-scopes`) sweeps that scope
(or every scope) for records with no summary yet; `--scope` and
`--all-scopes` are mutually exclusive, matching every other operator command
that offers this pair.

**`engram delete <id>`** and **`engram update <id> ...`** are the client
tier's two destructive verbs. A bare `delete` fetches the record and prints
what would be deleted; `--apply` sends one `DeleteMemory` call. A bare
`update` fetches the record and names the fields the flags would replace;
`--apply` sends one `UpdateMemory` call. A record you cannot read exits `4`
on both paths, so the preview reveals nothing a non-owner could not already
see. **The `delete_all` MCP tool has no CLI counterpart**, deliberately and
permanently: no verb deletes every record, and `engram catalog` lists none.

`backfill-short-ids` has MOVED from the `--dry-run` idiom to the `--apply`
one: it is now a thin delegating alias for `engram migrate` — see the
[upgrade guide](/guides/upgrade/)'s `## Unreleased` entry on
`backfill-short-ids --dry-run` removal for the full behavior break. Its
own blast-radius row is still **non-destructive** (the sweep is
additive-only), exactly like `migrate` itself above — the boundary
between the two idioms is therefore no longer purely the blast-radius
table's `Destructive` column; it is "does this command route through
`registerDestructive`", which today is every `Destructive:true` command
plus `migrate` and `backfill-short-ids`.

### `spine-review scan`

`engram spine-review scan --scope <scope>` (or `--all-scopes`) reports an
inventory of the memory spine and never mutates on any path. `--scope` and
`--all-scopes` are mutually exclusive. Alongside the
total and a per-scope/per-category breakdown, it reports the health signals
an operator needs before deciding what to curate: how many records carry a
summary and how many do not, how many are superseded, expired, or archived
(three independently observable states, each read from its own field, so a
record is never double-counted into the wrong bucket), how many carry
citations and how many citations exist in total, and the count of distinct
non-empty owners.

That last number is what makes the Subject-less claim observable rather
than asserted: `scan` is built on the operator tier, never on the
Subject-gated `Search`/`List`, so a sweep that had been silently narrowed
to one caller's bucket would report a single owner even on a collection
holding records from several. The report also carries `scanned_at` — the
one instant the sweep took at the top of the run and evaluated
`expired`/`scheduled` against — so a reader knows exactly what those counts
are relative to.

`scan` counts what recall hides. Superseded and archived records are
soft-hidden from `search_memory`/`list_memory` but still appear here, which
is the point: an inventory that inherited recall's blind spots could not
tell you what needs curating.

### `spine-review verify`

`engram spine-review verify --scope <scope>` (or `--all-scopes`) classifies
every stored citation into one of four tiers: `valid`, `moved` (the cited
excerpt still exists in the same file, at a different byte offset —
ordinary drift from an edit above the cited range, never treated as
breakage), `broken` (the file is missing, or the excerpt is gone from it
entirely), and `unverifiable` (a `commit`/`url`/`repo` citation, an empty
cached excerpt, a citation whose owning record names a different repo than
the working tree, or a `ref` this command refuses to read for safety —
every such citation carries the reason it was not checked, so a clean
report can never be read as coverage the run did not have). `--scope` and
`--all-scopes` are mutually exclusive. It never reads
outside the working tree it was run in — not even through a symlink — and
never widens its search past the file a citation names.

<!-- engram:rule:start verify-fail-on-accepted-values -->fail-on accepts broken, moved, unverifiable, or any; omitted, verify exits 0 regardless of findings<!-- engram:rule:end verify-fail-on-accepted-values -->
`verify` exits `0` by default even when it reports broken citations —
"the command worked" and "the data is healthy" are different questions.
Pass `--fail-on <tier>` to ask the second one from a CI step without
parsing the report: it exits a distinct code (`7`, see below) when the
named tier has at least one entry.

### `spine-review consolidate`

`engram spine-review consolidate --scope <scope>` (or `--all-scopes`)
reports ranked near-duplicate candidate pairs — `(record A, record B,
score)` rows sorted by score, highest first — using each record's
**already-stored vector**: no text is re-embedded and no vector ever
crosses the wire from engram to Qdrant. Exactly one of `--scope` or
`--all-scopes` is required: they are mutually exclusive, and supplying
neither is rejected with exit status `2`, like `spine-review scan`,
`spine-review verify` and `summarize-missing` (#508).

The **structural ranking never merges, never mutates, and never labels a
pair a "duplicate."** It ranks candidates and stops — deciding whether two
records are the same fact is a judgment for the operator (or a future
semantic skill) to make with the ranked list in hand, not something this
structural sweep pre-decides. There is no clustering and no default
similarity threshold: `--min-score` bounds report SIZE, not correctness,
and its absence (the default) means **no filtering at all** — a pair with
a negative cosine score is reported just like any other. `--top-k` bounds
how many neighbours are considered per record (a small default; see the
flag's own `--help` text for the exact number). The optional advisory
verdict described below MAY name any relation, including `duplicate` —
that is the verdict object doing its documented job, not the structural
ranking pre-labelling a pair.

With `--all-scopes`, a candidate pair may span **two different scopes** —
two scopes holding the same fact is exactly the duplication an operator
wants surfaced, so cross-scope pairs are reported, and each row names both
records' scopes (`a_scope`/`b_scope` in JSON) so a within-scope duplicate
is never confused with a cross-scope one.

A scope with fewer than two records reports zero candidates and exits `0`;
the JSON `candidates` array is `[]`, never `null`, in that case.

#### Advisory verdicts

When
[`ENGRAM_DECISIONS_PROVIDER`](/guides/configure/#typed-decisions-jev) is
set (off by default), each candidate pair ALSO gets an **advisory**
relation verdict, surfaced to you and never acted on — consolidate still
never merges or mutates a record because of one. `--no-verdicts` skips
this pass entirely for one run: no record content is sent and no
`verdict` key appears anywhere in the report.

Each request carries, per record, up to
`ENGRAM_DECISIONS_VERDICT_STATE_CHARS` characters total of its summary
followed by its content (default `1500`) — see
[configure's "What leaves your deployment"](/guides/configure/#typed-decisions-jev)
for what that means for your deployment. The decider is asked which of
five relations best describes how the newer record relates to the older
one; the more recently created record of the pair is always sent as
`record_b`, so `updates` means the NEWER record updates the older one,
regardless of which side of the pair (`a`/`b`) it happened to land on in
the structural ranking:

| Relation | Meaning |
|----------|---------|
| `duplicate` | Both state the same fact; keeping both is redundant (one may be more complete). |
| `contradicts` | They make incompatible claims about the same subject; one corrects or reverses the other. |
| `updates` | The newer record (`record_b`) is a newer state or a more complete version of the same fact. |
| `related` | Same subject area, but different and compatible facts; both are worth keeping. |
| `unrelated` | Different subjects. |

A second question asks whether both records are about the same specific
subject — `same_subject`, a probability. A verdict whose own relation
probability falls strictly below `ENGRAM_DECISIONS_VERDICT_THRESHOLD`
(default `0.9`, or `--verdict-threshold` to override it for one run) is
flagged `needs_review`.

A successful verdict's JSON shape:

```json
{
  "relation": "duplicate",
  "probabilities": {"duplicate": 0.93, "contradicts": 0.01, "updates": 0.02, "related": 0.03, "unrelated": 0.01},
  "same_subject": 0.97,
  "needs_review": false,
  "model": "typesafe/jev-1.13-20260917"
}
```

A failed request instead reports only its failure class:

```json
{"error": "timeout"}
```

`--output text` renders the same object as `verdict=duplicate p=0.93
same_subject=0.97
probabilities=duplicate:0.93,contradicts:0.01,updates:0.02,related:0.03,unrelated:0.01
model=typesafe/jev-1.13-20260917`, appending ` [needs review]` when
flagged, or `verdict unavailable (<class>)` for a failure — text is a
rendered view of the same JSON object (D-05), never a second source of
truth.

`<class>` is one of: `auth`, `bad_request`, `context_too_large`,
`rate_limited`, `unavailable`, `timeout`, `response_too_large`,
`malformed_response`, `invalid_request`, `canceled`, `error`, or
`state_unavailable` (the record's stored state could not be fetched from
engram's own store, before any request left for the provider). A failed
verdict is reported per pair and **never changes this command's exit
status** — consolidate still exits `0` even when every verdict fails.
`--timeout` bounds the whole sweep, including the verdict pass: a
deadline reached mid-pass reports every still-outstanding pair `verdict
unavailable (timeout)` rather than aborting the sweep or dropping
candidates.

Before sending anything, consolidate prints one line to stderr naming the
decisions provider, model, endpoint host and the per-record character
bound; after the pass, one summary line with requested/answered/
needs_review/unavailable counts and, on any failure, a per-class
breakdown. Neither line ever reaches stdout — a piped `--output json |
jq .` consumer never sees them, and script-based consumption of the JSON
report is unaffected either way.

**Reader guidance, not a rule the renderer enforces:** a `related`
verdict paired with a high `same_subject` probability is often worth a
second look — a genuine refinement or state change that falls just short
of a clean `updates` verdict tends to land there.

### `spine-review archive` / `spine-review restore`

`engram spine-review archive --id <id>` [`--id <id> ...`] explicitly retires
one or more records: it stamps `archived_at` (see
[Archiving](/reference/memory-record/#archiving) for the field's full
contract), an entirely new key orthogonal to both expiry (`not_after`) and
supersession (`superseded_by`). `engram spine-review restore --id <id>`
reverses it. Both accept `--id` **more than once**, processing every id and
reporting a per-id outcome — there is no filter form; a mutating verb's
blast radius is always the explicit id set an operator supplied, never an
implicit scope sweep.

Each id resolves to exactly one of three outcomes, reported honestly rather
than inferred from an error string:

| Outcome | Meaning |
|---------|---------|
| `changed` | The record's archived state was mutated (stamped or cleared). |
| `already` | The record was already in the target state — no write issued. |
| `not_found` | The id does not resolve to an existing record. |

An unknown id reports `not_found` on **both** verbs identically — there is
no asymmetry where one verb errors and the other silently reports success
for the same typo'd id. Archiving an already-archived record, or restoring
a never-archived one, is idempotent: it reports `already` and mutates
nothing.

**Retained, not deleted.** Neither verb removes a point, erases content, or
drops a vector. An archived record stays fully fetchable via `get_memory`
(and the MCP tools) — only recall (`search_memory`/`list_memory`/
`search_discovery`/`list_scheduled`) hides it, exactly like a superseded or
expired record. `engram spine-review scan` reports an `archived` count as
its own bucket, separate from `expired` — an archived record's `not_after`
may or may not also be lapsed, but the two states are independently
observable and never folded together.

### `spine-review purge`

`engram spine-review purge` deletes purge-eligible records — the phase's
sharpest edge, and the only irreversible command in the `spine-review`
group. It previews by default (see [Destructive commands](#destructive-commands)
above) and mutates only under `--apply`.

**Eligibility classes** (`--class`, repeatable): `superseded` (a
`superseded_by` link to an existing record, past a grace window),
`expired` (`not_after` lapsed, past a grace window), and `archived`
(`archived_at` past a retention window — **90 days by default**,
overridable with `--older-than`). A class-only run needs no `--scope`: a
class is a derivation, not an operator judgment.

**The free-form filter path** is gated harder:
<!-- engram:rule:start purge-filter-requires-scope -->the free-form filter path requires an explicit --scope or --all-scopes: category or tags always engage it, and older-than engages it when no class is selected<!-- engram:rule:end purge-filter-requires-scope -->.

Note the asymmetry, which is deliberate. `--older-than` alongside a
`--class` is read as *parameterizing* that class's window (overriding
`archived`'s 90-day retention, say), so it does not engage the harder gate.
`--category` and `--tags` engage it **even when a class is also selected**,
because narrowing by category or tag is a semantic judgment no structural
class expresses — and D-10 gates on how much judgment the operator supplied,
not on how many records the run would touch. A `--class superseded
--category decision` run is therefore narrower than `--class superseded`
alone yet still requires an explicit scope.

**The extract-before-delete gate** (rule `7smp8vy9hr`). Every candidate
must satisfy one of two paths before it can be deleted: a server-set
`superseded_by` link naming a later record (unforgeable — this is the
identical field `Store.Supersede` writes and `Update` preserves, never a
caller-supplied tag), or an authoritative milestone-summary record
covering the batch. The per-record path and the batch floor carry
different strength: the per-record link cannot be satisfied by anything a
caller writes, while the batch floor's marker is a caller-mintable tag
convention over a real, server-timestamped record — strictly stronger than
no artifact at all, strictly weaker than the per-record link. Neither is
ever presented as proof.

**Preview and apply happen within ONE invocation.** `purge` carries its
preview into `--apply` through an in-process manifest — there is no
`--manifest`/`--token` flag, and a preview cannot be carried into a later
invocation. `--apply` re-derives eligibility within its own run and
deletes only the **intersection** of what it just showed and what is
still eligible at that moment; a record that became ineligible since
preview is spared, and a record that became newly eligible is reported
`appeared` (never deleted — re-run to include it). Because the two
derivations run milliseconds apart in one process, the intersection
guards against a **concurrent writer**, not against operator delay — the
extract gate, the mandatory `--scope` on the filter path, and the
`discovery`/`rule` category exclusions carry the real safety weight.
Cross-invocation preview is a possible future ADDITIVE change requiring a
signature; it is not shipped here.

## Exit codes

The CLI uses the following exit-code meanings. Codes `8` and `9` belong to
setup, available from v0.16.0. See
[Agent Setup](/guides/agent-setup/) for availability and result handling.
Code `10` comes from a client verb whose server response overflowed what one
response can carry — in practice `list` and `search` — or from an operator
command whose own Qdrant read overflows (Phase 5 of this milestone bounds
those sweeps).

| Code | Meaning |
|------|---------|
| 0 | Success (including an empty result set) |
| 1 | Unclassified internal error — a backstop, not a general-purpose failure code. Reached by exactly two paths (see caution below). |
| 2 | Usage or validation error — a bad flag value, a violated mutually-exclusive flag group, or engram's own semantic validation (a missing `--server`/`ENGRAM_SERVER_URL`, an invalid `--output` value, an empty required flag) |
| 3 | Authentication or authorization failure |
| 4 | Not found |
| 5 | Transport or server unavailable |
| 6 | Request deadline exceeded — the server accepted the request but did not answer within `--timeout` |
| 7 | Findings reported under an explicit opt-in flag (e.g. `spine-review verify --fail-on`) — the command itself succeeded; the data just didn't pass the check |
| 8 | Setup partially failed: some attempted runtimes succeeded and some failed |
| 9 | All attempted setup runtimes failed |
| 10 | Response too large — the server's result exceeded what one response can carry (Connect `resource_exhausted`, hint `response_too_large`); retry with a smaller `--limit` or `--k`, or without `--full`; retrying the same request fails the same way. |

Absent runtimes are skipped and do not count as failed setup attempts.

:::caution[Only two paths still exit 1]
Framework flag errors — an unknown flag, an unparseable flag value — and a
violated mutually-exclusive flag group all exit `2` now, the same code as
engram's own semantic validation. Previously published guidance said a flag
typo exits `1`, "not `2`"; that guidance is **retracted**. Only two paths
still exit `1`:

- A **mistyped verb** (`engram serach`), rejected during cobra's own command
  resolution before any engram hook runs.
- A **genuinely unclassified internal error**, including an HTTP
  listener of `serve` failing to bind (for example, "address already in
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

Every client verb (`search`, `list`, `store`, `get` and the curation verbs
listed under [The client verbs](#the-client-verbs)) bounds its RPC call with
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
| `search`, `list`, `store`, `get`, `migration-status`, `tui`, and every curation verb (`related`, `tags`, `rules`, `scheduled`, `search-discovery`, `archive`, `restore`, `update`, `set-visibility`, `schedule`, `supersede`, `store-discovery`, `store-rule`, `delete`) | Per-RPC-call deadline | **Rejected** (usage error) |
| `prime` | One deadline for the whole command: the git probes, the credential and every RPC call together | **Rejected** (usage error) |
| `reindex`, `prune-expired`, `summarize-missing`, `backfill-short-ids`, `spine-review scan`, `spine-review verify`, `spine-review consolidate`, `spine-review archive`, `spine-review restore`, `spine-review purge`, `migrate`, `migrate status`, `migrate revert` | Whole-sweep wall-clock budget | Disables the deadline (unbounded), unchanged |
| `migrate-remap-owner`, `migrate-set-owner` | Whole-sweep wall-clock budget | **Rejected** (usage error) — changed this release, see the [upgrade guide](/guides/upgrade/#6-migrate-remap-owner---timeout-0--migrate-set-owner---timeout-0-no-longer-means-unbounded) |

A reader comparing `engram search --help` against `engram reindex --help`
and `engram migrate-remap-owner --help` side by side should not have to
infer this table from the flag's one-line usage text — the three groups
genuinely disagree on what `--timeout 0` does.

**`migrate --apply` and `backfill-short-ids --apply` budget TWO full-backlog
passes under one `--timeout`:** a fresh `DryRun` preview (which additionally
performs a `MintShortID` collision-probe `Count` per eligible record), then
the manifest-limited apply pass — with no way to allocate time between the
two. Size `--timeout` for both passes together, not just the write you
expect.

**`migrate revert --apply` budgets an even larger share under one
`--timeout`:** the CLI's own whole-range preflight, `Store.Revert`'s
independent SECOND whole-range preflight (repeating the identical exhaustive
scan before it is allowed to write anything), and then the write-convergence
loop itself, which re-derives and re-scrolls on every pass until the backlog
reaches zero. An operator sizing `--timeout` for "one reverse walk of my
backlog" is actually budgeting for at least two full read-only passes over
that same range before any inverse is even applied.

## Terminal console

In a terminal, a bare `engram` opens the terminal console: a full-screen,
read-only view of the memories you can read. `engram tui` opens the same
console. It accepts four of the shared flags (`--server`, `--token-file`,
`--insecure` and `--timeout`) and two of its own (`--theme` and `--mouse`). A
bare `engram` accepts no flags. It reads its settings from the environment
(`ENGRAM_SERVER_URL`, `ENGRAM_TOKEN`, `ENGRAM_TIMEOUT`, `ENGRAM_TUI_THEME`,
`ENGRAM_TUI_MOUSE`), the saved login and the console's saved choices. Use
`engram tui` when your credential is in a file or when you need `--insecure`,
because neither has an environment form.

The console sends only read requests. It cannot store, update, supersede,
archive, restore, delete or share a record.

The console needs a terminal on both stdin and stdout. Without one (an agent,
a pipe or a script), `engram` and `engram tui` open nothing and do not wait
for input. They write one line to stderr that names `engram catalog`, and
exit `2`. Agents read [the self-describe catalog](#the-self-describe-catalog)
instead.

Before the console takes over the screen, it checks its settings and reads
your scopes from the server. If that fails, the terminal stays as it was, and
the command exits with the code a client verb gives for the same failure:

| Code | Cause |
|------|-------|
| 2 | No `--server`, `ENGRAM_SERVER_URL` or saved login, an unknown `--theme`, a `--mouse` value that is not a boolean, or another bad flag value |
| 3 | The server rejected the credential |
| 5 | The server is unreachable |
| 6 | The server did not answer within `--timeout` |

Quitting the console exits `0`.

### Layout

The results list sits on the left and a preview of the selected record sits
on the right. The preview follows the selection. It shows the summary, the
category, scope, visibility and owner, the age, the tags and the state words
at once. The record's full content, rendered as markdown, follows when the
selection stops moving.

The layout follows the terminal's size:

| Terminal | Layout |
|---|---|
| 100 columns or wider | List and preview side by side |
| Narrower than 100 columns and at least 32 rows | Preview below the list |
| Smaller | List only. Enter opens the selected record full screen, and Esc returns to the list |

`p` hides or shows the preview. `z` makes the focused pane fill the screen,
and `z` again restores it. `tab` moves focus between the list and the
preview; with a pane filling the screen, `tab` shows the other pane in its
place. While the preview has focus, `j` and `k` scroll it. From the list,
`J` and `K` scroll the preview one line, and Ctrl-D and Ctrl-U scroll it half
a page.

Each row shows the category, the summary, the state words (`archived`,
`superseded`, `expired`, `scheduled`), the tags with a `+N` for those that do
not fit, and the age (`7h`, `2d`, `1mo`). When the rows come from more than
one scope and there is room, a row also shows its scope. Rows for archived,
superseded and expired records are drawn muted.

The line above the rows states what the view searched: how many records it
shows, across how many scopes, the query, how the rows are ordered, and how
many records the recall gate hid. The line below the rows shows the
selection's position (`12/50+` while more pages exist), the hidden counts by
state, and short notes such as `copied short_id …`. When a request fails, it
shows the server's message unchanged, including any `field=<name>
hint=<code>` envelope, and the console keeps running.

### Views

The console opens on your latest memories across every scope you can read.

- **1 Memories** lists your memories, or shows search results. Press `/`,
  type a query and press Enter. The search box says what Enter will do:
  `enter: search · 1 embedding` for a query, `enter: open` for a UUID or a
  `short_id` (which opens that record instead of searching), and
  `enter: list` for an empty query. A search runs only on Enter, because the
  server sends every search query to its embedding provider. The next page of
  a list loads on its own when the selection nears the end, and `n` loads it
  at once.
- **2 Tags** shows the tags in the scope, with a bar for each count. `space`
  chooses a tag, and Enter lists the memories that carry every chosen tag. The
  filter line shows them as `all of #helm #oci`.
- **3 Rules** shows the rules in every rule scope you can read. The preview
  shows a rule's full text.
- **4 Scheduled** shows your own records that are not active yet, with each
  window in words (`reveals in 3d`). `w` switches to your expired records
  (`expired 2d ago`).

`r` shows the selected record's related memories. They are grouped by edge
type, in the order supersession (`S`), shared citation (`C`), shared tag
(`T`) and vector neighbour (`V`). Each group heading shows its count, and each
row says why it is related. Enter walks to the selected record's own related
memories, and `[` or Esc walks back.

`=` narrows the rows already loaded as you type, without asking the server.
The line above the rows then reads, for example, `7 of 50 loaded match "oci"`.
Esc clears the narrowing.

List and search show only the records the recall gate returns. `f` opens the
scope and filters panel. There you choose one scope or every readable scope
(type to filter the scopes), include archived, superseded, or scheduled and
expired records, and remove chosen tags. When nothing matches, the console
names what it searched and the filters that would widen it.

### Command palette, themes and mouse

Ctrl-K opens the command palette. It lists every view, filter and action,
each with its key, and narrows the list as you type. Enter runs the
highlighted entry, and Esc closes the palette.

`T` opens the theme picker. The themes are `engram` (the web console's
colours), `terminal` (only your terminal's own colours), `catppuccin` and
`tokyo-night`. Each picks its light or dark colours from the terminal's
background. Moving through the picker shows each theme at once. Enter keeps
the theme and saves it, and Esc goes back to the theme you started with. With
`NO_COLOR` set to any value, the console draws no colour. Every state and edge
type also has a word or letter, so colour is never the only signal.

Mouse support is off by default, so your terminal's own text selection keeps
working. Turn it on with `--mouse`, `ENGRAM_TUI_MOUSE=true`, or the palette's
"Mouse" entry. With it on, a click selects a row and the wheel scrolls the
pane under the pointer.

The theme and mouse setting are chosen in this order: the flag, then the
environment variable, then the choice saved from inside the console, then the
default (`engram`, mouse off). A choice saved in the console goes to
`console.json` in the engram configuration directory, next to the login
cache. It holds no credential and is never sent to the server. A missing or
invalid file leaves the console on its defaults. When `--theme` or
`ENGRAM_TUI_THEME` is set, a theme you save from the picker applies only once
neither is set, and the console says so.

### Keys

The console uses the web console's key where the web console has one for the
same action. Every key also shows in the `?` overlay and the command palette.

| Key | Action |
|-----|--------|
| `j` / `k`, Down / Up | Move the selection, or scroll the preview when it has focus |
| PgDn / PgUp, Home / End | Move by one page, or to the first or last row |
| Enter | Open the selected record full screen; in the related view, walk to it; in the tags view, list the memories with the chosen tags |
| Esc | Go back: close an overlay, clear the narrowing, return focus to the list, or leave a record or related view |
| `tab`, Shift-Tab | Move focus between the list and the preview |
| `J` / `K` | Scroll the preview one line from the list |
| Ctrl-D / Ctrl-U | Scroll the preview half a page from the list |
| `p` | Hide or show the preview |
| `z` | Make the focused pane fill the screen, or restore it |
| `/` | Search: type a query, then press Enter |
| `=` | Narrow the loaded rows |
| `f` | Open the scope and filters panel; `space` toggles a row and Enter chooses it |
| `n` | Load the next page now |
| `1` `2` `3` `4` | Open the memories, tags, rules or scheduled view |
| `r` | Show the related memories of the selected record |
| `[` | Walk back along the related trail |
| `space` | In the tags view, choose or drop a tag |
| `w` | In the scheduled view, switch between scheduled and expired records |
| `c` / `C` | Copy the selected record's `short_id` / full id to the clipboard |
| `T` | Open the theme picker |
| Ctrl-K | Open the command palette |
| `?` | Show all keys |
| `q`, Ctrl-C | Quit |

The copy keys send the text through the terminal's clipboard sequence (OSC
52), which also works over SSH. Some terminals turn that sequence off.

## Session primer: `engram prime`

`engram prime` writes a session primer for an agent. The primer holds the
memory scope, the rules index for that scope, and a short digest of recent
memories. It is one read, so an agent can start a session with its memory already in
context. The Claude Code plugin's session-start hook runs it for you (see
[the plugin guide](/guides/plugin/#sessionstart-hook--memory-recall)).

```sh
engram prime
```

The primer has four parts, in this order:

1. **The scope header.** `Memory spine scope: <scope>` and a line that names
   where the scope came from. In a linked git worktree it adds
   `Memory workspace scope: <scope>` and its source. A last line names the
   rule scopes that prime read.
2. **The rules index.** One line per rule: its full `short_id`, its summary and
   its tags. There is no rule content. Fetch a rule's full text with
   `engram get <short_id>`. When there are no rules, this part is not shown.
3. **The recall digest.** It lists up to 10 recent records in each scope that
   you can recall. Each record has one line: its full `short_id` and its
   summary. The
   groups are "Repo-wide memories" and "This workspace's memories". A record in
   both groups shows only in the first group. A group of 10 ends with a note
   that older records exist. An empty group is not shown. Archived,
   superseded, expired and not-yet-active records are never shown, and another
   user's record shows only when it is `shared`.
4. **The catalog line.** It names `engram catalog`, the command that prints
   [the machine-readable catalog](#the-self-describe-catalog).

prime only reads. It never writes to the store, starts a login, prompts, or
reads stdin.

### Where the scope comes from

prime uses the first of these that is set:

| Source | What prime uses | The header names |
|---|---|---|
| `--scope <scope>` | That scope, exactly as given | `--scope` |
| `ENGRAM_SCOPE` | That scope, exactly as given | `ENGRAM_SCOPE` |
| The git `origin` remote | `repo:<host>/<org>/<repo>`, with the scheme, the user name, a port number and `.git` removed | `git origin remote` |
| No `origin` remote | `repo:<name>`, where `<name>` is the name of the main repository's directory | The repository directory name. The header also says that this is probably not a scope on the server. |

prime reads git on your machine only. It makes no network call to find the
scope. In a linked git worktree it also reads the workspace overlay,
`<spine>:ws:<worktree directory name>`. A scope from `--scope` or
`ENGRAM_SCOPE` has no overlay.

The rule scopes come from the memory scope. When the memory scope starts with
`repo:<repo-id>`, prime reads `rule:repo:<repo-id>`. When `ENGRAM_PROJECT` is
set, prime also reads `rule:project:<ENGRAM_PROJECT>`. This is the same
variable that the session-start hook reads.

prime rejects these values with exit `2` before it calls the server, and the
error names the flag or the variable:

- An empty `--scope ""`.
- A `--scope` or `ENGRAM_SCOPE` value that starts with `rule:` or
  `discovery:`. Give the memory scope instead. prime finds the rule scope from
  it.

### Outside a git repository

Outside a git repository, with no `--scope` and no `ENGRAM_SCOPE`, prime writes
a short primer. The primer says that prime found no scope and that `--scope`
sets one. It
calls no server, needs no credential, and exits `0`. This means a session start
never fails because of prime.

```sh
engram prime --scope repo:github.com/org/repo
```

### Output

prime writes the text primer by default, also when stdout is not a terminal.
This is different from the other client commands, which write JSON to a pipe.
The reason is that prime's output is usually put straight into an agent's
context.

`--output json` writes one JSON document with the same facts:

```sh
engram prime --output json | jq '.spine, .rule_scopes'
```

| Key | Value |
|---|---|
| `spine`, `overlay` | `{"scope": ..., "source": ...}`, or `null`. `source` is one of `flag`, `env`, `git_origin_remote`, `repo_directory_name` and `linked_worktree`. |
| `rule_scopes` | The rule scopes that prime read |
| `rules` | The rules, in the record shape that `engram list` writes |
| `memories.spine`, `memories.overlay` | The digest records, in the same shape |
| `catalog_command` | `engram catalog` |

### Server, credential and exit codes

prime finds the server and the credential in the same way as the other client
commands (see [Shared flags](#shared-flags)). Its `--timeout` is one deadline
for the whole command. When a call fails, prime writes nothing to stdout and
exits with the client exit code:

| Code | Cause |
|------|-------|
| 0 | The primer was written. This includes a primer outside a git repository. |
| 2 | No `--server`, `ENGRAM_SERVER_URL` or saved login; a rejected scope; an `--output` other than `text` or `json`; or a scope that the server rejected (its `field=` / `hint=` message is passed through) |
| 3 | The server rejected the credential |
| 5 | The server is unreachable |
| 6 | The server did not answer within `--timeout` |

## The self-describe catalog

Run `engram catalog`. It writes one JSON document to stdout and exits `0`:
every command in the live binary, every flag with its type, default, and
usage, and this same exit-code table — derived from the running binary's
actual command tree and exit-code constants, not maintained by hand. It needs
no server and no credential. `engram --help` is unaffected and still prints
ordinary human help.

A bare `engram` printed this document in earlier releases. It now opens the
[terminal console](#terminal-console). Without a terminal, a bare `engram`
exits `2` and writes one line to stderr that names `engram catalog`.

This catalog is the **machine-readable, authoritative** form of everything on
this page. When the two disagree, trust the binary — this page is prose
describing it, not the contract itself.

```sh
engram catalog | jq '.commands[] | select(.name == "search")'
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
