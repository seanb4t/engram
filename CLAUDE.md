<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# CLAUDE.md — engram

AI assistant routing for the `engram` repo: a self-hosted, correctable,
OAuth-secured memory MCP server for coding agents (Go + Qdrant).

## Conventions

- **VCS:** git. Branch + PR; never push to `main` directly (protect-main ruleset).
- **Planning:** OpenSpec (`openspec/`, `/opsx:*`). GSD is retired and `.planning/` is a read-only
  archive — see **Planning** below.
- **Milestone labels (legacy):** the archived GSD milestones in `.planning/` are labelled CalVer
  `YYYY-MM-DD.NN` (the milestone's local **start** date plus a two-digit same-day counter) from
  2026-08-12 onward; milestones through `v0.13.x` keep their SemVer-style labels. They label
  milestones **only** — they are not release versions (see **Releases** below), so
  milestone → release is a lookup, never an inference. No new milestone label is ever stamped.
- **Task runner:** `task` (see `Taskfile.yaml`). `task` = lint + test.
- **Protobuf/buf:** the `EngramService` Connect API is defined in `proto/` and
  generated via `go tool buf` (`task proto:lint` / `task proto:gen`); the
  generated `gen/` tree is committed and CI-checked for drift (`buf` job).
- **CLI:** cobra; config is loaded by `internal/config` (koanf): env-first via the `ENGRAM_` prefix with `--flag` overrides; no viper.
- **Commits:** Conventional Commits; PR titles validated in CI
  (action-semantic-pull-request).
- **License:** every **in-scope** Go/Markdown file carries the Apache-2.0 SPDX
  header (`task license:check`). `task license:add` applies it. Scope is owned
  by `.licenserc.yaml` — never by hand. **Do not add an SPDX header to any file
  whose first line must be `---` YAML frontmatter**: `.planning/**` (the
  archived GSD artifacts), `skill/**/SKILL.md`, slash-command
  markdown, and `docs-site/**`. All are excluded in `.licenserc.yaml`; if
  `license:check` is green, the file does not need one.
- **Lint/format:** `task lint` (golangci-lint, yamlfmt, actionlint, rumdl) and
  `task fmt` (gofmt, dprint, yamlfmt) must be clean.
- **Releases:** release-please-driven (see `RELEASING.md`), **SemVer, always** —
  never CalVer, and decoupled from the milestone label above. Helm requires
  `Chart.yaml: version` to be SemVer 2 and Go module tags require `vX.Y.Z`, so a
  date-shaped release version is not representable. release-please derives the
  bump from Conventional Commit types; nothing about a milestone label feeds it.
  Merging the release PR cuts the `vX.Y.Z` tag + GitHub Release; the release
  workflow then ships the binary + image (goreleaser) and the OCI Helm chart
  (`task chart:push`). release-please syncs `charts/engram/Chart.yaml`
  (`version`/`appVersion`) and `skill/engram/.claude-plugin/plugin.json`
  (`$.version`); the binary version is ldflags-injected into `main.version`.
- **Migrations:** payload migrations ARE schema-version-driven — an ordered
  registry of additive-only steps in `internal/migrate`, each declaring its
  reversibility, swept by `engram migrate` (see docs-site `guides/migrate`).
  No migration ever applies automatically — not on startup, not on failure —
  the mutating verbs preview by default and mutate only under `--apply`.
  What IS automatic: server startup runs a read-only `MigrateStatus` probe
  that may log a pending-migrations warning (and a separate future-version
  warning); it never invokes the sweep and never gates startup. The registry
  covers version-driven payload evolution only — `migrate-remap-owner`,
  `summarize-missing`, and `reindex` key off an IdP claim change, ongoing
  async summary fill, and embedder config identity respectively, none of
  which is version-driven, so none is in the registry or the status
  histogram.
- **Not used here:** viper, cocogitto.
- **Spike findings for engram** (implementation patterns, constraints, gotchas) → `Skill("spike-findings-engram")`
- **Sketch findings for engram** (console design decisions, CSS patterns, visual direction) → `Skill("sketch-findings-engram")`
- **Console conventions for engram** (tokens, state words, classifier, honest feedback, keyboard) → `Skill("engram-console-conventions")`
- **SPA ↔ Connect client for engram** (clients, CSRF, query keys, resume, per-RPC contract) → `Skill("engram-connect-client")`

## Planning

OpenSpec is the planning workflow: `openspec/` holds the specs and changes, driven by `/opsx:*`.
GSD was retired on 2026-10-03.

- **Never use GSD here.** Do not invoke or offer a `/gsd-*` skill or command, a `gsd-*` agent, or a
  GSD MCP tool. GSD is installed globally, so this repo switches it off in `.claude/settings.json`
  (`skillOverrides` and `permissions.deny`). If a GSD surface still appears, ignore it and say so.
- **`.planning/` is a read-only archive.** Never create, edit, move, or regenerate a file under it.
  Read it as legacy context when exploring a change: the requirements, decisions, and phase contexts
  for everything shipped through milestone `2026-09-25.01` live there.
- **Specs grow from changes — do not backfill.** `openspec/specs/` starts nearly empty and fills in
  as each archived change merges its delta. Do not generate baseline specs from existing code or
  from `.planning/` history, and do not convert old requirements wholesale. Bring the relevant
  slice into the change being proposed instead.
- **Backlog is GitHub Issues.** Work with no exploration or proposal yet lives as an issue (see
  **Issue Tracking**) and leaves that stage when an OpenSpec change is created for it.

## Memory contract (stable)

Tools: `store_memory` / `schedule_memory` / `search_memory` / `list_memory` /
`list_scheduled` / `get_memory` / `supersede_memory` / `update_memory` /
`delete_memory` / `delete_all` / `archive_memory` / `restore_memory` /
`related_memories` / `list_tags`. A record carries `content`,
`scope`, repo/workspace/worktree/base_dir, `source`, `category`, `tags`,
`summary`/`summary_source` (client-authored or auto-generated digest; omit for none),
`actor` (verified caller — server-set, never client-supplied), `owner` (caller's
configured owner-claim value — `ENGRAM_OWNER_CLAIM`, default `email`, the authz
key — server-set), `visibility` (`private` default |
`shared`), `created_at`, a server-minted `short_id` (10-char Crockford
base32 handle, accepted anywhere an id is accepted; legacy records gain one via
`engram backfill-short-ids`), and `schema_version` (server-set on every write; a
record predating the key reads as version 0 by absence; never gates recall).
Recall returns summaries by default with `full=true` opt-in;
full content via `get_memory`. `search_memory` results carry an always-on per-result
`score` (raw Qdrant cosine similarity, higher = closer; zero/omitted on unranked
`list_memory`/`get_memory` results). With `ENGRAM_SEARCH_RANKER=jev` (opt-in; requires
`ENGRAM_DECISIONS_PROVIDER`), `search_memory` and `search_discovery` results are reordered
by the decision provider's per-hit probability that the record answers the query and carry
`relevance` (0 to 1, omitted when reranking is off or fell back); a decision error or timeout
never fails the search. Design intent: explicit, zero-junk, correctable. Do not
add auto-extraction. A rejected call names the failing field and a machine-stable hint
code in one envelope (`field=<name> hint=<code>: <text>`; see docs-site
`reference/errors.md`), with a memory `summary` bounded at `ENGRAM_MEMORY_MAX_SUMMARY_BYTES`
(default 512 bytes), `content` bounded at `ENGRAM_MEMORY_MAX_CONTENT_BYTES` (default 65536
bytes), and `tags` at `ENGRAM_MEMORY_MAX_TAGS` entries of `ENGRAM_MEMORY_MAX_TAG_BYTES` bytes
each (default 128 / 128), always enforced.

With `ENGRAM_SUMMARY_ON_WRITE=true` (and `ENGRAM_SUMMARY_MODEL` set), auto-generated
summaries are filled **asynchronously** shortly after `store_memory`/`schedule_memory`
returns, by a bounded background worker pool — never on the synchronous write path. A
write always succeeds once persisted, even if the summarizer is down, slow, or the
queue is full; an unfilled record just has "no summary yet" until a later async fill or
the next `engram summarize-missing` sweep reclaims it. See `guides/configure.md`
(Auto-summary → Async-on-write summaries) for the opt-in gate and knobs.

**Isolation (authz):** each actor sees/mutates only their own records; `shared`
records are readable (never writable) by any **authenticated** caller — the
shared read grant requires a non-empty owner-claim value. No issuer → single anonymous
bucket (`owner==""`); anonymous callers (auth disabled) see only that bucket and
cannot read other actors' `shared` records. The `set_visibility` tool and
`update_memory`'s `shared` field toggle sharing; `update_memory`'s `tags` field
replaces the tag set (omit to preserve, empty array to clear). `search_memory`
and `list_memory` accept an optional `tags` filter — records must carry **all**
listed tags (AND); on `search_memory` it is a hard pre-filter applied before
vector ranking. `search_memory` / `list_memory` / `list_scheduled` also accept
optional `created_after` / `created_before` (RFC3339, half-open `[after, before)`)
to window recall by creation time; `list_memory` paginates via an opaque `cursor`
arg and returns `{memories, next_cursor}` (empty `next_cursor` = last page);
its `limit` defaults to 20 and rejects any value above the shared maximum, 1000.
`search_memory` and `list_memory` also accept `cross_spine` (bool) to span every
scope the caller can read, with the response reporting `searched_scopes` and
`scopes_truncated`, or, if the coverage enumeration itself failed after hits
were already found, `scopes_unknown` (the call still succeeds; `searched_scopes`
is then absent rather than an empty list); the `engram search`/`engram list`
CLI verbs reach the same capability and report the same three fields.
`search_memory` and `list_memory` results (and Connect `SearchMemories`/`ListMemories`,
`engram search`/`engram list`) also carry `recall_gate_hidden` — `{total, archived,
superseded, expired, scheduled}` counts of records the recall gate hid from the
returned window (the same top-k for search, the same page for list), counting only
states the request did not include; a record with several states counts once in
`total` and once per state; absent when the count could not be computed; counts
only, never ids — the recall gate itself and the MCP input schemas are unchanged.
Pre-isolation records (missing
`owner` key) are invisible to every read until you backfill them with `engram
migrate-remap-owner --from-missing --to <owner>` (the `migrate-set-owner` command
is a deprecated alias). To re-stamp records after an IdP `sub`/claim change, use
`engram migrate-remap-owner --from <old> --to <new>`.

Scheduled tools: `schedule_memory` stores a memory with a temporal validity
window — `not_before` (RFC3339; deferred reveal: hidden from recall until then)
and/or `not_after` (RFC3339; expiry: dropped from recall at then). `list_scheduled`
surfaces windowed records the recall gate is hiding (`state` = `scheduled` default
| `expired` | `all`); active windowed records surface normally via
`search_memory`/`list_memory`. Recall is gated; fetch-by-id (`get_memory`) is not.
`list_scheduled` also accepts `cross_spine` (bool; still only the caller's own
records — deferred reveal holds across every scope) and paginates via an opaque
`cursor`/`next_cursor`, the same shape `list_memory` uses. Operators reclaim
lapsed records with `engram prune-expired --apply` (preview
by default without `--apply`; add `--older-than DUR` for a grace period).

Supersession: `supersede_memory` corrects a record without losing history.
`supersedes` is a set of one or more target ids (full UUID or `short_id`) — a
call names a set of duplicate RECORDS to merge into one surviving record with
history preserved for every predecessor. It takes the `store_memory` field set
for the new/correcting record plus `supersedes`, stores the new record, and
stamps `superseded_by` onto every target — additive links, never a delete or an
overwrite; the forward link is a list (one correcting record, several
predecessors), the backward link stays a single id, and each predecessor has
exactly one successor. A superseded record is soft-hidden from recall
(`search_memory`/`list_memory`/`search_discovery`/`list_scheduled`) but stays
fetchable by id via `get_memory`. Owner-only (the write gate — a `shared` record
you can read is not one you can supersede); the single-live-head rule is
enforced per target, an invalid set rejects the whole call, and the rejection
names every offending target of one failure class — a non-owned target, a
nonexistent target, and a target whose short id is ambiguous all stay
indistinguishable from each other across the whole set; never automatic (no
similarity or write-through path); rules cannot be superseded (delete instead);
`idempotency_key` is accepted on this verb — the fingerprint covers content and
the target set, and a retry after an ambiguous failure replays instead of
duplicating. Use it for *reversals* — prefer `update_memory` for in-place
refinement and `delete_memory` for junk. `validate_only` (bool) runs the same
preflight (ownership, single-live-head, rule rejection, ambiguous short_id)
without writing anything, naming the resolved targets or the exact rejection a
real call would produce — optional, useful before a multi-target merge, never
consulting `idempotency_key`. Agent-facing guidance lives in the
`curating-memory` skill.

Archived state: `engram spine-review archive` stamps `archived_at` on one or
more records by id; `engram spine-review restore` deletes it, returning the
record to normal recall — always reversible, and never a delete, content
erasure, or vector removal. The MCP `archive_memory` / `restore_memory` tools
and the Connect `ArchiveMemory` / `RestoreMemory` RPCs reach the identical
effect: `ids` (1 to 1000, each a full UUID or `short_id`), one outcome per id
in caller-supplied order (`archived` / `already_archived` / `restored` /
`not_archived` / `not_found`), owner-only — a record you do not own reads
`not_found`, indistinguishable from a nonexistent id, and never echoes its
UUID. Agent use is gated on the user agreeing to it in the conversation, never
on the agent's own judgment; `engram spine-review archive` / `restore` remain
the operator-tier path, unchanged. `archived_at` shares supersession's soft-hidden-
but-still-fetchable-by-id contract: an archived record drops out of
`search_memory`/`list_memory`/`search_discovery`/`list_scheduled` but stays reachable by id via `get_memory`.
Archiving is an orthogonal key — it never writes an expiry and never writes a
supersession link, and each of a record's derived states clears independently
of the others. Every surface renders a record's derived state as up to four
words, in canonical order: `archived`, `superseded`, `expired`, `scheduled` —
descending by finality. `expired` is evaluated first and, when present,
suppresses `scheduled`; the window-boundary rule that decides each state
lives on `reference/memory-record.md`, not restated here.

Discovery tools: `store_discovery` / `search_discovery`. A discovery is a 5th
`category` carrying `kind` (`map`|`fact`), `citations` (with aging `pin`s), and
`summary`; it lives in a separate `discovery:repo:*` scope, is recalled on
demand (never at session start), and is captured via the `discovering` skill.
Design intent unchanged: explicit, citation-backed, no auto-extraction.

Rule tools: `store_rule` / `list_rules`. A rule is a 6th `category`: normative,
user-blessed, always-shared ground truth in a dedicated `rule:repo:*` /
`rule:project:*` scope. An agent proposes a rule candidate when it notices
one; `store_rule` is invoked only after the user blesses it (never promoted
unilaterally); its `summary` must be a single line (the index entry). `list_rules` returns the complete set — up to 1000
rules per scope, the same shared recall maximum — for one or more `rule:*`
scopes, oldest-first, compact index shape by default (`full` for
content); omitting `scopes` lists every readable rule scope's rules in one
cross-scope read (up to 1000 rules in total rather than per scope), reporting
the covered rule scopes the same way a cross-spine recall reports
`searched_scopes`. Rules surface at session start as a progressive-disclosure index (one
line per rule; full text fetched on demand via `get_memory`). `set_visibility`
is rejected for rules — delete the rule instead. Design intent unchanged:
explicit, user-blessed, no auto-extraction.

Related-memories and tags: `related_memories` returns one record's
neighbourhood — supersession chain, shared tags, shared citations, and vector
neighbours, each edge typed with its evidence — on demand only: curating
(dedup before a store, finding what a correction should supersede) or an
explicit user ask, never at session start and never as an automatic search
follow-up. `list_tags` returns exact, recall-visible tag counts for a scope or
(scope omitted) every readable scope — default 100, maximum 1000, most-used
first, `more` when truncated, no server-side prefix filter — used to reuse an
existing tag before `store_memory` rather than inventing a near-duplicate, and
to choose a `tags` filter. Both are reads; agent-facing guidance for all seven
curation capabilities lives in the `curating-memory` skill.

All seven curation capabilities above (`archive_memory`/`restore_memory`,
`supersede_memory`'s `validate_only`, the `list_rules`/`list_scheduled`
widenings, `related_memories`, `list_tags`) also exist on Connect
(`ArchiveMemory`/`RestoreMemory`/`SupersedeMemory` are CSRF-gated writes;
`ListRules`/`ListScheduled`/`RelatedMemories`/`ListTags` are reads), delegating
to the same core as their MCP tool.

## Auth

`--oidc-issuer`/`ENGRAM_OIDC_ISSUER` enables bearer-token enforcement (JWKS
signature + issuer + expiry; optional audience). The verified identity becomes
the memory `actor`. No issuer → validation disabled (logged loudly).

## Issue Tracking

**GitHub Issues** is the tracker (`gh issue list`, `gh issue create`). Beads was retired
2026-07-08: the full export is archived at `.planning/archive/` and active work was migrated to
GitHub Issues (label `from-beads`). The GSD backlog was migrated the same way on 2026-10-03
(label `from-gsd`); `.planning/BACKLOG.md` is a stale index, so list issues with `gh` instead.
Do not use markdown TODO lists for durable tracking.

Durable project memory (decisions, conventions, gotchas) → the **engram** MCP store (see
"Memory contract" above) — not `MEMORY.md` files.

## Session Completion

Subordinate to explicit user, repository, and orchestrator instructions.

1. **File follow-ups** — open GitHub issues for remaining work.
2. **Run quality gates** (if code changed) — `task` (lint + test).
3. **Commit as you go** — commit in coherent groups of related work; don't wait to be asked.
   Conventional Commits are **required** (`type(scope): description`; PR titles are CI-validated).
   `main` is protected — branch + PR, never push to `main` directly.
4. **Hand off** — summarize changes, validation, and any blocked step with its exact command and error.
