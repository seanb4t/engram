---
title: Upgrade Guide
description: Behavioral and breaking changes by version — including the v0.7.10 recall-by-default change and how to restore full content.
---

engram follows [semantic versioning](https://semver.org/). Most releases are
additive, but some minor versions change **default behavior** without changing
the wire schema. This page lists those changes and the opt-in to restore prior
behavior.

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
