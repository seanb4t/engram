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
