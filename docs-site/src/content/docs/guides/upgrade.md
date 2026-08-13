---
title: Upgrade Guide
description: Behavioral and breaking changes by version — including the v0.7.10 recall-by-default change and how to restore full content.
---

engram follows [semantic versioning](https://semver.org/). Most releases are
additive, but some minor versions change **default behavior** without changing
the wire schema. This page lists those changes and the opt-in to restore prior
behavior.

## Unreleased — One CLI exit-code taxonomy, enforced flag groups, and a client request timeout

<!--
  Release-time step: rename "Unreleased" above to the version release-please
  actually cuts (e.g. "## vX.Y.Z — ..."). Do not hand-author a version number
  before that point — release-please computes the next version from
  Conventional Commits at merge time and syncs charts/engram/Chart.yaml and
  the skill plugin manifest, but not this file. See RELEASING.md.
-->

This release closes GitHub issues #453 (flag exclusivity) and #467
(exit-code unification) in one pass, and ships #452 (a client request
timeout). Every `engram` invocation — client verb or operator command alike
— now resolves flag conflicts, configuration, timeouts, and errors through
one predictable, migration-safe contract.

**Do you need to act?**

| If you… | Read |
|---|---|
| branch on `engram`'s exit status in any script or CI job | §1, §2, §4 |
| pass `--offset` with `--page-token`, or `--scope` with `--cross-spine` | §3 |
| script `migrate-remap-owner` / `migrate-set-owner` with `--timeout 0` | §6 |
| rely on a CLI call blocking until the server answers | §5 |
| set client configuration through environment variables | §7 |
| pattern-match the exact `field=` value of an argument rejection (not just check a field name's presence) | §8 |
| only run `engram` interactively | nothing — no action |

### 1. Framework flag errors now exit 2, not 1

Previously published guidance told callers that a flag-parsing error raised
by the command framework itself — an unknown flag or an unparseable flag
value — exits `1`, "not `2`," and warned against assuming every usage-shaped
failure exits `2`. **That guidance is retracted.** An unknown flag, an
unparseable flag value (`engram search --k notanumber`), and a violated
mutually-exclusive flag group all exit `2` now, alongside engram's own
semantic validation.

A retired `MEM_*` legacy environment variable being set (superseded by its
`ENGRAM_*` equivalent) also now exits `2` instead of `1` — reachable from
any command, for example `engram version` with a stale `MEM_QDRANT_ADDR`
still set. See [Configuration](/guides/configure/) for the full
`ENGRAM_*`/`MEM_*` legacy mapping.

Two paths still exit `1`, and only two:

- A **mistyped verb** (`engram serach`), rejected during cobra's own command
  resolution before any engram hook runs.
- A **genuinely unclassified internal error** — an untyped Go error that
  reached the top of `Execute()` without being wrapped by any of this
  release's new classifiers.

**Who should act:** any script branching on exit `1` to detect a flag
mistake. It now reports `2`, the same code as every other usage error.

### 2. Operator commands now return classified exit statuses

`reindex`, `prune-expired`, `summarize-missing`, `backfill-short-ids`,
`migrate-remap-owner`, `migrate-set-owner` (the deprecated alias for
`migrate-remap-owner --from-missing`), and `serve` previously returned exit
`1` for every failure — a bad flag value, an unreachable Qdrant, and a
config error were indistinguishable to a caller's exit-status check. All
seven now use the same vocabulary the client verbs already used:

| Code | Meaning |
|---|---|
| `2` | Bad flag or configuration value |
| `4` | Missing target |
| `5` | Backend (Qdrant) unreachable |

**The one documented exception:** `serve`'s own `ListenAndServe()` call —
the underlying HTTP listener failing to bind (for example, "address already
in use") — still exits `1`. Every other `serve` startup failure (bad
config, missing auth credentials, OIDC discovery failure) exits `2` or `5`
like everything else in this list. This is deliberate, not an oversight:
exit `5` has meant "the remote server or Qdrant is unreachable" at every
other site in this taxonomy, and a local OS bind failure is a different
condition — force-mapping it onto `5` would make `5` ambiguous for any
caller scripting both `serve` and a client verb. See the caution box in the
[CLI guide](/guides/cli/#exit-codes) for the same statement in context.

**Who should act:** any operator script that branches on a specific exit
code from any of these seven commands. A script that only checks
zero-vs-nonzero needs no change.

### 3. `--page-token` together with `--offset` is now an error

Previously, `engram list --offset N --page-token X` silently ignored
`--offset` and paged by token. The help text made two different claims for
the same trio of flags — `--offset` was "mutually exclusive with
`--cursor-mode`" but `--page-token` merely "ignored `--offset`." Silently
ignoring a flag the caller explicitly passed is the same class of defect as
an unenforced exclusivity claim, so `list`'s
`--offset`/`--cursor-mode`/`--page-token` trio is now a declared, enforced
mutually-exclusive group: supplying any two exits `2` before any network
call.

The same enforcement now covers `search --scope` / `search --cross-spine`
and `list --scope` / `list --cross-spine`, which previously had a
hand-rolled guard covering only the symmetric case.

**The blast radius is wider than "don't pass both."** cobra's flag groups
count a flag being *supplied*, not its value. `engram list --offset 0
--page-token ''` — both at their zero values — is rejected too, because
both flags were explicitly set on the command line, even though `0` and
`""` look like "nothing." The same applies to `engram search --scope s
--cross-spine=false`: `--cross-spine=false` is still a supplied flag and
still conflicts with `--scope`. If your script always passes every flag it
knows about, even at a default-looking value, this is the change most
likely to affect it.

**Who should act:** any script or shell alias that passes
`--offset`/`--page-token` together, or `--scope`/`--cross-spine` together
(at any value), on `search` or `list`.

### 4. New exit code 6 for a request timeout

A client-side request deadline being exceeded now reports **exit code
`6`**, distinct from exit `5` (transport or server unavailable). Code `5`
means "I could not reach the server or it refused the connection"; code `6`
means "the server accepted the request but did not answer before the
deadline." A caller currently treating `5` as "retry later" should decide
whether a timeout warrants raising `--timeout` instead of a bare retry.

### 5. New client `--timeout`, default 30s, and `0` is rejected

Every client verb (`search`, `list`, `store`) now accepts `--timeout` (also
settable via `ENGRAM_TIMEOUT`), bounding the RPC call with a real deadline.
Default: `30s`. A value of `0` is rejected as a usage error (exit `2`) — it
does **not** mean "unbounded."

**This is the opposite convention from four of the six operator commands'
own `--timeout`** (`reindex`, `prune-expired`, `summarize-missing`,
`backfill-short-ids`, where `0` still disables the deadline, unchanged).
It matches `migrate-remap-owner`/`migrate-set-owner`'s own `--timeout` as
of this release (see #6 below). Same flag name, different zero-semantics
across three groups of commands — a reader must not have to infer it. See
[Request timeout](/guides/cli/#request-timeout) for the full comparison
table.

**Who should act:** any client-side script invoking `search`/`list`/`store`
with no explicit `--timeout` gets a new 30-second ceiling it did not have
before — there was previously no client-side deadline at all, so a hung
server blocked the invocation forever. If your workload legitimately needs
longer than 30 seconds, pass `--timeout` explicitly.

### 6. `migrate-remap-owner --timeout 0` / `migrate-set-owner --timeout 0` no longer means unbounded

Before this release, these two commands' pre-existing `--timeout` flag
treated `0` as "disable the deadline." That is now a rejected usage error
(exit `2`), reconciled onto the same rule the new client `--timeout` uses
(#5 above) — the binary no longer ships a `--timeout` whose zero-semantics
depend on which command you happen to be scripting, for these two commands.

**Who should act:** any operator who scripts `migrate-remap-owner --timeout
0` or `migrate-set-owner --timeout 0` expecting an unbounded run. Remove
the flag (both commands' own default, `5m`, still applies) or supply a
large explicit duration (`--timeout 24h`) instead.

### 7. Client configuration now resolves through the `ENGRAM_` registry

`--server`, `--token-file`, `--output`, `--insecure`, and `--timeout` all
now resolve flag-then-environment-then-default through the same
`internal/config` registry the server side already used, instead of four
separate hand-rolled resolvers. `ENGRAM_TIMEOUT` is new. `ENGRAM_SERVER_URL`
is unchanged. `--insecure` deliberately has **no** environment fallback —
it cannot be set any way other than the flag itself, so it can never be
silently enabled by an inherited environment variable. There is still no
`--token`/env-value flag: a credential reaches the client only via
`--token-file` or `ENGRAM_TOKEN`, never argv.

**Who should act:** nobody, for ordinary use — this is an internal refactor
of how the same five settings resolve, not a change to which settings exist
or their precedence order. Flagged here only because the internal resolver
functions this replaced (`resolveServerURL`, `resolveOutputFormat`) are
gone; relevant only if external tooling somehow referenced them directly,
which nothing in this repository did.

### 8. Interface discoverability: no published hint code changed, two field lists widened

Phase 2 (interface discoverability) reclassified six existing argument
rejections onto a declared rule registry (see the
[error envelope reference](/reference/errors/)), so every conditional
requirement is now stated on the CLI Usage text, the MCP tool schema and
Description, and the proto field comment — not just discoverable by
triggering the rejection. **No rejection's `hint=` code changed** as part of
this: every converted site kept the hint code it already carried
(`conditional_required`, `required`, `not_applicable`, `ordering`,
`mutually_exclusive`).

**Two `field=` lists widened** — both intentional, neither a narrowing:

- `search_discovery`'s empty-scope rejection now names `field=scope,cross_spine`
  (previously `field=scope` alone), matching `search_memory`/`list_memory`'s
  existing attribution for the same scope-required-unless-cross_spine rule.
- `list_memory`'s Connect-lane `cursor_mode`/`offset` mutual-exclusion
  rejection now names `field=cursor_mode,offset,page_token` (previously
  `field=cursor_mode,offset`) — the declared rule states all three mutually
  exclusive paging controls, matching the CLI's existing three-way
  `MarkFlagsMutuallyExclusive` group from the prior release.

**Who should act:** a caller that pattern-matches the exact `field=` value
(rather than checking whether a specific field name is present in the
comma-joined list) for either of these two rejections. Checking for a
specific field's presence in the list — the documented, correct way to read
`field=` — is unaffected by either widening.

**The `mutually_exclusive` hint's documented shape widened from "always two
fields" to "two or more fields"** to match the paging-trio case above — see
the [error envelope reference](/reference/errors/#the-ten-hint-codes) for the
updated wording. No code that already reads `field=` as a list is affected.

### 9. `prune-expired` now previews by default; `--apply` performs the deletion

Before this release, `engram prune-expired` deleted matching records
immediately, with no confirmation step. It now previews: a bare invocation
reports the number of eligible records and exits `0` without deleting
anything. Pass `--apply` to perform the deletion — the only flag that does.

No deprecation window was offered for this one: it is a safety default, and
shipping a release that still deletes without `--apply` would leave the
dangerous behavior live for one more cycle, which is exactly what this
change exists to close.

**Audited:** no `.github/` workflow, chart template, or CI caller invokes
`prune-expired` — the only Helm CronJob runs `summarize-missing
--all-scopes` (`charts/engram/templates/summarize-cronjob.yaml:23`), which
this change does not touch. This flip has no in-repo consumer; it is
documentation-only outside the binary itself.

**Who should act:** any operator who scripts `engram prune-expired` and
relies on it deleting. Add `--apply` to restore the previous behavior.

### 10. `migrate-remap-owner --dry-run` is removed; `--apply` performs the mutation

`migrate-remap-owner` is classified `destructive` in the blast-radius table
(a `--from`/`--from-anon` call can overwrite an existing, non-empty owner
value with no history retained), so it now follows the SAME preview-by-default
contract as `prune-expired` (#9 above) rather than keeping its own
`--dry-run` flag. A bare invocation previews the count of records it would
remap and exits `0` without writing; `--apply` performs the remap.
`--dry-run` no longer exists on this command — passing it now fails with an
unknown-flag usage error (exit `2`), a loud failure rather than a silent
behavior change.

No deprecation shim (a `--dry-run` no-op alias) was offered either: shipping
two ways to request a preview — `--dry-run` on one destructive command and
`--apply`'s absence on the other — is exactly the "two ways to say one
thing" ambiguity a single `--apply` contract across the destructive tier
exists to remove.

**Who should act:** any operator who scripts `migrate-remap-owner
--dry-run`. Remove the flag — a bare invocation now previews by default.
Add `--apply` to perform the remap.

### 11. New `engram spine-review verify` and exit code 7 for opt-in findings

`engram spine-review verify` is new: it classifies every stored citation
into `valid`, `moved`, `broken`, or `unverifiable`, bounded to the file a
citation names and to the repo the command runs in. It exits `0` by
default even when it reports broken citations — the command itself
succeeded; the data just didn't pass a check nobody asked it to run yet.

Pass `--fail-on <broken|moved|unverifiable|any>` to ask that question from
a CI step: it exits **code `7`**, a new addition to the taxonomy, when the
named tier has at least one entry. Code `7` is additive — no previously
published exit code changes meaning — but the taxonomy is a published
contract, and this release's exit-code migration (#1–#4 above) set the
precedent that changes to it are announced rather than left to a diff.

**Who should act:** nobody, unless you adopt `spine-review verify
--fail-on` in a CI pipeline — in which case treat exit `7` as "findings
reported," distinct from every other nonzero code in the taxonomy, which
all mean "the command itself failed."

### In-repo consumer audit

REQ-exit-code-migration-safe requires this migration to be checked against
every consumer this repository can actually observe. engram is self-hosted
with no telemetry — the maintainer cannot enumerate, and this guide cannot
notify, any consumer outside this repository. What follows is what *was*
checked, re-run at the commit shipping this guide, not carried forward from
earlier research: it names what was checked rather than implying a survey
of users who cannot be enumerated. This guide itself is the notification
channel for everyone else.

| Location | Finding |
|---|---|
| `Taskfile.yaml` | Only builds the binary (`go build ... -o bin/engram`). No invocation of the CLI, no exit-status branch. |
| `.github/workflows/` (`ci.yaml`, `docs-site.yaml`, `release.yaml`) | No workflow invokes the built `engram` binary or branches on its exit status. |
| `charts/engram/templates/summarize-cronjob.yaml` | Runs `summarize-missing --all-scopes` as a CronJob container. Kubernetes CronJob semantics distinguish only zero from nonzero exit — confirmed no numeric status branch in the template. |
| `skill/engram/hooks/` | Both hooks (`session-start-memory-recall`, `posttooluse-memory-capture-nudge`) are Python scripts with no `subprocess`/HTTP-client import — they never shell out to the CLI or call the Connect API directly. |
| `docs-site/` | `reference/errors.md` links to this guide's sibling [exit-code table](/guides/cli/#exit-codes) rather than duplicating it; no example in the docs site branches on a specific exit code. |
| `internal/e2e/cli_exitcode_test.go` | **One consumer found.** `TestCLIExitCodes` asserts specific numeric exit codes end to end. Its `unknown flag` case expected `1` and now expects `2`; its `unknown verb` case still expects `1` and is unchanged. Updated in this release. |

One in-repo consumer branches on a specific numeric exit code — the end-to-end
CLI test above — and it was updated alongside the change. It is worth naming how
it was nearly missed: the first pass of this audit swept build, CI, chart, skill,
and docs surfaces but not Go test files under `internal/`, which is exactly where
a numeric exit-code assertion is most likely to live. If you maintain a fork,
grep your own test suites, not just your scripts.

### 12. Records now carry a `schema_version` stamp

Every record written from this release carries a `schema_version` payload
key, an integer stamped by the server on every write. A record written
before this release has no such key — it reads as version `0`, exactly the
same as a record explicitly stamped at version `0`. **No backfill is
required and none is offered**: recall behaviour is unchanged for those
records, and nothing you do today is affected by this field's existence.

`schema_version` is visible on `full=true` recall and on `get_memory`
responses; it is never used to filter or gate what recall returns.

**The rollback hazard.** If you run a binary *older* than a record's stored
`schema_version` and that binary edits the record, the record keeps its
higher version stamp, but the payload was rebuilt from the older binary's
struct — so any key only a newer version knows about is dropped from that
rewrite. This is safe: schema evolution in this project is additive-only,
so anything lost this way is always re-derivable, and the recovery is to
re-run the migration sweep against the affected record. **That sweep does
not exist in this release** — there is no `engram migrate` command to run
yet, so do not look for one. The hazard is only reachable if you
deliberately roll a binary backward across a schema change; until the
sweep ships, the mitigation is simply not to do that against records you
cannot re-derive.

`engram reindex` copies payloads verbatim from source to target and
therefore does **not** advance a record's `schema_version` — reindexing is
not a migration, and it is not the missing sweep either.

**Who should act:** nobody. This is purely additive, forward-looking
groundwork; no existing behavior changes.

---

## v0.7.10 — Recall returns summaries by default

**Affected tools:** `search_memory`, `list_memory` (MCP), and `SearchMemories`,
`ListMemories` (Connect `EngramService` v1).

**What changed.** Recall now returns **summary-shaped** records by default: the
`content` field is cleared and a compact `summary` (or a deterministic
head-truncation when no summary exists) is returned in its place. This cuts the
token cost of session bootstrap and broad recall for the common case where the
caller only needs to know *what* a memory is, not its full text.

Full content is still available — it is one opt-in away, never removed:

- **MCP:** pass `full=true` to `search_memory` / `list_memory`.
- **Connect:** set `full=true` on `SearchMemoriesRequest` /
  `ListMemoriesRequest`.
- **Any tool:** `get_memory` (and `GetMemory`) always returns the full record;
  fetch-by-id is deliberately **not** recall-gated.

### Is this a breaking change for my client?

It is a **behavioral** change, not a wire-schema change:

- The protobuf schema stayed **additive** — `summary`, `summary_source`, and
  `full` were added; nothing was removed or renumbered. `buf breaking` stays
  green, and the Connect service remains **`engram.v1`** (no version bump).
- Existing clients that read `content` from recall responses will now see an
  **empty string** where they previously saw full text. If your client relies on
  `content` from `search_memory` / `list_memory` (or their Connect
  counterparts), set `full=true` to restore the prior shape.

`get_memory` is unchanged and always returns full content, so clients that fetch
records by id after recall are unaffected.

### Why this was made the default

Returning full content for every recalled record caused session-bootstrap token
overflows (~70 KB) for typical memory sets. Returning summaries by default
resolves this at the source without forcing callers onto a different tool.
Recorded as ADR
[`engram-ambu`](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-ambu-recall-returns-summary-by-default-full-content-opt.md).

### Opting in to full content

```jsonc
// MCP — search_memory
{ "query": "auth middleware", "full": true }

// MCP — list_memory
{ "scope": "repo:github.com/seanb4t/engram", "full": true }
```

For the Connect API, set `full: true` on the corresponding request message.

### Back-filling summaries on existing records

Summaries are generated on write when `ENGRAM_SUMMARY_MODEL` is set. Records
created before summaries were enabled (or migrated from an older deployment)
return a head-truncation in place of a generated summary until they are
back-filled. Run the offline sweep:

```sh
engram summarize-missing            # all scopes
engram summarize-missing --scope repo:github.com/seanb4t/engram
engram summarize-missing --dry-run   # preview without writing
```

See the [MCP Tools reference](/reference/tools/) for full argument docs and the
[Memory Record reference](/reference/memory-record/) for the `summary` /
`summary_source` fields.

---

## v0.8.4 — Memories now carry a `short_id` handle

Every memory now carries an additive `short_id` field — a 10-character lowercase
Crockford base32 handle (case-insensitive; confusable glyphs are folded on input).
It is minted on creation alongside the UUID and can be used anywhere an `id` is
accepted: `get_memory`, `update_memory`, `delete_memory`, `set_visibility`,
`store_discovery` (replace-in-place), and the Connect `GetMemory` RPC.

Recall output (`search_memory`, `list_memory`) includes both `id` and `short_id`.

### Backfilling existing records

Memories created before this feature was enabled do not have a `short_id`. Backfill
them with the operator command. Preview first, then apply:

```sh
engram backfill-short-ids --dry-run          # run this first: preview without writing
engram backfill-short-ids                    # apply to all memories
engram backfill-short-ids --timeout 5m       # custom wall-clock limit
```

No re-embedding or data migration — the UUID is unchanged and still valid everywhere.
The backfill is payload-only and can safely run alongside read traffic.

---

## v0.12.0 — Field-attributed, hint-carrying argument rejections

This release ships six changes: three affecting how engram rejects a
malformed or invalid argument (full grammar and vocabulary: the
[error envelope reference](/reference/errors/)), plus a per-lane chat/summarize
credential, a reindex resume correctness fix, and the CLI reaching cross-spine
recall.

### 1. Argument-validation message text changed

A rejected argument now returns a message in a stable `field=<name> hint=<code>:
<text>` grammar, leading with the field that failed, instead of free-form prose. If
your client matched on the old message wording, match on `field=`/`hint=` instead —
the wording after the colon is not a contract and has already changed once in this
release.

**Scope fence:** this covers `internal/server/tools.go`'s argument validation only.
**The MCP 401 bearer-auth rejection body is unchanged and byte-identical** — it is
pinned by a dedicated test (`TestMCP401BodyByteIdentical`) precisely so this note
cannot be read as broader than it is. If you match on the 401 body text, nothing to
change.

### 2. The published tool schema loosened

Fields that were previously `required` in the advertised MCP JSON schema (e.g.
`store_memory`'s `content`, `scope`, `source`, `category`) are no longer marked
required at the schema level — `tools/list` now shows a shorter or empty `required`
array for the affected tools. Required-ness moved into engram's own validation
instead, so the **same calls are still rejected**; the difference is that the
rejection now names the correct field (this closes issue #360, where an oversized
`summary` produced a schema-level error naming `content`).

One genuinely **new** rejection: a memory `summary` is now bounded at
`ENGRAM_MEMORY_MAX_SUMMARY_BYTES` (default 512 bytes) — see
[Configuration → Memory](/guides/configure/#memory). A summary that used to be
accepted at any length is now rejected past that bound.

### 3. Connect error codes widened

A validation failure on the Connect API previously always mapped to
`CodeInvalidArgument`. It now maps to one of `CodeInvalidArgument`,
`CodeOutOfRange`, or `CodeFailedPrecondition`, selected by the failure class (see
the [class-to-code table](/reference/errors/#the-class-to-connect-code-mapping)).

**The `engram` CLI needs no change.** All three codes already shared the CLI's
`exitUsage` exit code (`2`) before this release, and still do — verified against
`exitCodeForConnectErr`'s own unmodified test table
(`TestExitCodeForConnectErrTable`). A Connect client that branches on the error
code directly (not through the CLI) and only handles `CodeInvalidArgument` must
widen to handle all three.

### 4. The chat/summarize lane can carry its own API key

`ENGRAM_OPENAI_CHAT_API_KEY` (or the Helm value
`memory.summarize.chatApiKeySecret`) lets the chat/summarize lane use a
different credential than the embedder's `ENGRAM_OPENAI_API_KEY`. **No action
is required** — leaving it unset is byte-identical to previous behavior: the
embedder's key inherits to the chat lane exactly as before. **Who should
act:** an operator who has pointed `ENGRAM_OPENAI_CHAT_BASE_URL` at a
different gateway and would rather that gateway not receive the embedder's
credential. See
[Configuration → Auto-summary](/guides/configure/#auto-summary) for the full
per-lane credential behavior.

### 5. `reindex --resume` now compares tags, not just content

Before this release, `reindex --resume` compared only content, so a record
whose tags changed while its content did not was reported unchanged and kept
a stale vector. **Who should act:** operators who have run `--resume` on an
earlier version should re-run it — size the blast radius first with
`engram reindex --target <target> --resume --dry-run`, then re-run without
`--dry-run`. See
[Reindex → Repairing a pre-patch resume](/guides/reindex/#repairing-a-pre-patch-resume)
for the full procedure; its one limit is that a source collection deleted
after cutover makes the correct tags unrecoverable.

### 6. `engram search` and `engram list` can now request cross-spine recall

Cross-spine recall (`cross_spine`, plus the `searched_scopes` /
`scopes_truncated` provenance fields) shipped on the Connect API in an
earlier release in this line, but the CLI never wired it — the flag did not
exist and neither request ever set the field. `engram search` and
`engram list` now accept `--cross-spine`, mutually exclusive with `--scope`;
see [Recall scope selection](/guides/cli/#recall-scope-selection) for the
full rule and the coverage footer it adds to text-mode output.

**Who should act:** existing CLI users who could not reach cross-spine recall
from a shell before. **What action is needed:** none to keep current
behavior — a scope-bearing invocation is unchanged. A recall invocation that
omitted `--scope` still fails, but now fails client-side with no round trip
to the server, exiting `2` instead of waiting on a network call to find out.
Anyone who wants the wider recall opts in explicitly with `--cross-spine`.
This is purely additive: no protobuf field was added, no wire schema
changed, and no existing invocation's behavior changed beyond where the
rejection now happens.
