<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Design: Auto-Summary for Curated Memories (Recall-Time Token Reduction)

- **Bead:** engram-cly5
- **Date:** 2026-06-25
- **Status:** Design (pending design-reviewer)

## Problem

engram records are dense, caveat-heavy mini-documents (2–4 KB each). Recall
(`search_memory` / `list_memory`, and the Connect `SearchMemories` /
`ListMemories`) returns the full `content` of every hit. At session bootstrap an
agent lists the spine scope and pays for every record in full — for this repo a
single `list_memory` already overflows the MCP tool token limit (~70 KB). That
cost is pure waste when the agent only needs to know *which* memories exist and
roughly what each says, then fetch the few it actually needs in full.

The lever is **recall shaping**, not enrichment: a `summary` returned *in place
of* full content saves tokens; a summary returned *in addition to* full content
is net-negative. The full record stays one `get_memory` away.

The open question this design answers: should the summary be **explicit**
(submitter-authored) or **auto-generated** (server-side, a cheaper model than the
submitting agent) — and how do the two coexist without violating engram's design
intent (*explicit, zero-junk, correctable, no auto-extraction*)?

## Decision summary

- **Explicit-first, auto as fallback.** The submitter — which has full
  conversational context and intent — may author a `summary`. When omitted, an
  **operator-invoked** cheap-model pass may fill it. Provenance is tracked and
  every summary is correctable. Auto-extraction stays banned at *write* time;
  auto-*summary* of already-explicit content is an explicit operator action, not
  silent mining.
- **Recall returns summary by default, full content opt-in.** This is a
  deliberate, versioned evolution of the "stable" memory contract.
- **Execution = offline operator CLI now; async-on-write queue is a documented
  future seam, not built in v1.**

## Non-goals (YAGNI)

- Synchronous (inline) summarization on the hot write path.
- Async-on-write queue + worker (v1 ships the reusable seam only).
- Lazy-at-first-recall generation (mutates on read; concurrency/cold-start cost).
- Auto-tagging, auto-labelling (category), auto-linking — separately considered
  and rejected: tags/category are filter-only (the embedder sees only `content`)
  and belong to explicit curation; links duplicate what the vector index already
  does on demand.
- Re-summarization beyond the stale-clear rule (§ Stale-on-edit).

## Architecture

Five units, each independently testable, mirroring existing engram patterns.

### 1. Data model (zero migration)

`store.Memory` already carries `Summary string` (`store.go:77`), round-tripped by
`fromPayload`; today `payload()` only writes it inside the discovery branch
(`if m.Category == "discovery"`, `store.go:175`). Un-gate it so curated records persist `summary` too. Add two
additive payload keys (no Qdrant migration — same pattern discoveries shipped
with):

| Field | Payload key | Values | Purpose |
|-------|-------------|--------|---------|
| `Summary` | `summary` | string | the compact recall text (existing field, now curated too) |
| `SummarySource` | `summary_source` | `client` \| `auto` \| `""` | trust signal; surfaced at recall |
| `SummaryModel` | `summary_model` | string (set only when `auto`) | diagnostics / trust / which model wrote it |

`summary_source` is the trust signal: an `auto` summary is lossy, so recall
surfaces the source and curating-memory tells agents to fetch full before acting
on specifics.

### 2. Summarizer client (`internal/summarize`)

A new package mirroring `internal/embed`:

- `Summarize(ctx, content) (string, error)` → `POST /v1/chat/completions` on the
  **same** OpenAI-compatible gateway engram already uses for embeddings
  (`openai.base_url` + `openai.api_key`). No new dependency, no new auth.
- Model from new `ENGRAM_SUMMARY_MODEL` (registry field). The "cheaper model" is
  an operator routing concern — a gateway model alias — engram only sends a name.
- **Presence-enables the feature:** empty `ENGRAM_SUMMARY_MODEL` ⇒ auto-summary
  disabled (same convention as `oidc.issuer` enabling auth).
- System prompt: preserve **negations, imperatives, identifiers, numbers
  verbatim**; never invent; one line, ≤ `ENGRAM_SUMMARY_MAX_CHARS` (default
  ~280). Content already shorter than the cap is returned/kept as-is (nothing to
  compress).
- OTel span (`engram.summarize.model`), 30 s timeout — same shape as `embed`.

New config (registry + validate, env-first per the no-viper convention):

| Key | Env | Default | Notes |
|-----|-----|---------|-------|
| `summarize.model` | `ENGRAM_SUMMARY_MODEL` | `""` | empty disables auto-summary |
| `summarize.max_chars` | `ENGRAM_SUMMARY_MAX_CHARS` | `280` | summary target + truncation cap |

These land as a new `SummarizeConfig{ Model string; MaxChars int }` sub-struct on
`config.Config` (koanf-tagged, mirroring `EmbedConfig` / `OpenAIConfig`),
registered in `internal/config/registry.go`. `Validate()` does **not** error on
an empty `summarize.model` (presence-disables, exactly like `oidc.issuer`);
`max_chars` validates as a positive integer.

### 3. Core fill operation (the reusable seam)

`FillSummary(ctx, rec) error` = `Summarize(content)` → `Store.SetSummary(id,
summary, model)`. Properties:

- **Idempotent:** no-op if `rec.Summary != ""` or content ≤ cap.
- **Vector-preserving:** `SetSummary` uses Qdrant `SetPayload` (already used for
  the owner backfill, `store.go:953`) — content is unchanged so the embedding is
  still valid; never re-embeds.
- **Store-layer only:** `FillSummary` takes a `store.Memory`, where `Summary` is
  a plain `string`. The `*string` presence-signal (§Stale-on-edit) lives solely
  in the MCP handler layer and never reaches the store.
- **Best-effort:** a per-record failure is logged + counted, never fatal to a
  batch.

This single function is called by the CLI sweep now **and** a future queue worker
later — that is the async-on-write seam, with zero v1 cost.

### 4. Explicit write path

- Add optional `summary` to `storeArgs` / `store_memory` → `summary_source =
  client`. (Curated parity with `storeDiscoveryArgs`, which already has it.)
- The write path gains **zero** new external calls — explicit summaries are just
  a stored string.

### 5. Auto fallback — execution (Approach A: offline operator CLI)

`engram summarize-missing [--scope S] [--older-than DUR] [--limit N] [--dry-run]
[--all-scopes]`, modeled on `reindex` / `prune-expired` / `migrate-set-owner`:

1. Error if `ENGRAM_SUMMARY_MODEL` is unset (auto disabled).
2. Scroll Qdrant for empty-`summary` records matching the filters. `--older-than
   DUR` matches records whose `created_at` is older than `DUR` (summaries carry
   no timestamp of their own); `--scope` / `--all-scopes` bound the scope set;
   `--limit` caps records processed.
3. `FillSummary` each (best-effort; per-record errors logged + counted).
4. Report `filled` / `skipped` (already had one, or too short) / `failed`.

Covers the existing backlog and any summary-less new records. Records without a
summary surface truncated content at recall until the next sweep.

**Future (documented, not built):** async-on-write — the store handler enqueues
the record id *after* a successful upsert; an in-process worker drains the queue
via `FillSummary`. The write path never blocks on or fails because of
summarization. v1 ships `FillSummary` standalone and idempotent so the worker is
a thin addition.

## Data flow

**Store (explicit):** client → `store_memory{summary?}` → embed(content) →
upsert(payload incl. `summary`, `summary_source=client`).

**Auto fill (operator):** `engram summarize-missing` → scroll empty-summary →
`Summarize(content)` → `SetPayload{summary, summary_source=auto, summary_model}`.

**Recall (the token win):** `search_memory` / `list_memory` default
(`full=false`) → per hit return `summary` (or deterministic head-truncation of
`content` to the cap with `truncated=true` when no summary) + `id`, `scope`,
`category`, `tags`, `created_at`, `summary_source`. Full `content` is omitted.
`full=true` returns the current full shape. `get_memory` always returns full
content (unchanged; it is the escape hatch and is not recall-gated).

## Error handling

### Stale-on-edit (`update_memory`) — fail loud, never drift

`summary` becomes a presence-signal `*string`: `nil` = unaddressed, `"x"` = set,
`""` = clear. Evaluated against whether `content` changed in the same call:

| Existing summary | `content` changed? | Caller passed `summary`? | Result |
|------------------|--------------------|--------------------------|--------|
| none | — | — | proceed |
| any | no | `nil` | preserve (no-op on summary) |
| any | — | non-`nil` | apply: set (`source=client`) or clear |
| `auto` | yes | `nil` | **auto-clear** (server-derived, regenerable; re-filled next sweep) |
| `client` | yes | `nil` | **REJECT the write (atomic, nothing persisted)** |

The rejection is an actionable error: *"content changed but a caller-authored
summary would go stale: re-send the same summary to keep it, pass an updated one,
or pass `summary:\"\"` to clear it."* Rationale: a human/agent-authored summary
silently surviving a content edit is exactly the unowned drift engram rejects;
forcing an explicit decision keeps memories correctable and honest. Auto
summaries never block — there is no human intent to lose.

The owner gate still runs once (via `FetchForUpdate`) **before** the
content-change/summary evaluation, so an unauthorized caller is rejected first
and never learns whether a summary exists.

### Summarizer / gateway failures

- Summarizer errors are fail-closed *for the auto pass only*: `FillSummary`
  returns an error, the record keeps an empty summary (recall falls back to
  truncation), and the CLI counts it as `failed`. No partial/garbage summary is
  ever written.
- Auto-summary is never on the write path, so a gateway outage cannot fail a
  `store_memory`.

## MCP tool + Connect/proto parity

Memory-contract changes ship to **both** surfaces (as `tags` did):

- **MCP tools** (`internal/server/tools.go` schemas + descriptions):
  `store_memory` (+`summary`), `update_memory` (+`summary` presence-signal and
  the stale-summary contract), `search_memory` / `list_memory` (+`full` arg;
  summary-by-default output), `get_memory` (unchanged — full).
- **Connect `EngramService`** (`proto/engram/v1`): `Memory` gains `summary`
  (field 15) + `summary_source` (16); `SearchMemoriesRequest` /
  `ListMemoriesRequest` gain `full` (bool). Regenerate the committed `gen/` tree
  (`task proto:gen`); the `buf` drift check must stay green.

**Backward-compat (behavioral, not wire).** The proto schema stays additive —
new fields/args, nothing removed or renumbered — so `buf breaking` remains green.
But the *default response semantics* change: a caller that omits `full` now
receives summary-shaped memories (no `content`). This is a **minor-semver
behavioral change**, called out in release notes. The one in-tree Connect
consumer — the web UI (`ui/`, generated client `gen/ts/`) — must be updated in
this work to either pass `full=true` on its recall calls or render the
`summary` / `summary_source` shape; that UI update is in scope here, not deferred.

## Skill / plugin / docs scope (ships with the feature)

- `curating-memory` skill: when/how to author a `summary` (caveat-faithful), the
  lossy-`auto` warning ("fetch full before acting on specifics"), and the
  `update_memory` stale-summary contract.
- engram plugin skill docs + the session bootstrap recall guidance (the "call
  `list_memory` per spine scope" instruction): recall now returns summaries by
  default — this is what resolves the ~70 KB bootstrap overflow.
- docs-site `reference/tools.md` + `reference/memory-record.md`: `summary` /
  `summary_source` / `full` arg; `summarize-missing` command.
- `CLAUDE.md` memory-contract section: summary field + recall-default evolution.

## Validation — the "test" of the auto path

A fidelity harness (a `task` target / Go test over a corpus sample, reusing the
existing `eval-*` scope convention — e.g. `eval-2026-05:project:…`, per the
`store.Memory.Scope` comment in `store.go`) runs the candidate cheap model over **real**
memories and scores the dangerous failure mode: **negation + identifier +
imperative preservation** (does the summary keep "DECLINE…", `--type related`,
exact ids/numbers?). This is the empirical gate for trusting/enabling auto
broadly — answering "is the cheap model good enough?" with data, not argument.

## Testing

- **Unit:** `summarize` client against an `httptest` mock (mirrors
  `embed_test.go`); curated `summary` payload round-trip; `SetSummary` partial
  write preserves the vector; truncation logic + `truncated` flag; recall shaping
  under `full` true/false; `update_memory` matrix above (auto-clear vs
  client-reject vs explicit set/clear).
- **Connect parity:** `summary`/`summary_source` mapped on `Memory`; `full`
  honored on both `SearchMemories` and `ListMemories` (extends the existing
  parity test).
- **CLI:** `summarize-missing` dry-run vs apply; filter/limit honored; errors
  when `ENGRAM_SUMMARY_MODEL` unset; failure counted not fatal.
- **Eval:** the fidelity harness above.

## ADR-worthy decisions (for capture-adrs)

1. **Explicit-first + auto-fallback** (vs auto-always / explicit-only).
2. **Offline operator CLI execution** (vs synchronous / lazy), async-queue later.
3. **Recall returns summary by default, full opt-in** — versioned evolution of
   the stable memory contract.
4. **Reuse the existing `Summary` payload field** for curated records (zero
   migration).
5. **`update_memory` with a content change rejects an unaddressed `client`
   summary** (fail-closed vs silent-preserve vs silent-clear).

## Open questions

- Exact `ENGRAM_SUMMARY_MAX_CHARS` default (280 is a starting point; the eval may
  retune it).
- Whether the fidelity eval becomes a CI gate or a manual operator step before
  enabling auto in a given deployment.
