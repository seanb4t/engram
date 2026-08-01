<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright 2026 Sean Brandt -->

# Phase 5: Operator Config & Reindex Correctness - Context

**Gathered:** 2026-08-01
**Status:** Ready for planning
**Mode:** Smart discuss (autonomous) — three grey areas proposed in batch, all accepted by Sean

<domain>
## Phase Boundary

Two independent operator-correctness defects, plus the repair path for the second.

1. **#350 — the chat/summarize lane cannot carry its own provider credential.** The base-URL
   split already shipped (`ENGRAM_OPENAI_CHAT_BASE_URL`, resolved by `cmp.Or` at
   `summarizerFromConfig`), but both lanes still share one `ENGRAM_OPENAI_API_KEY`. An operator
   pointing the summarizer at a different gateway has no way to give it that gateway's key.

2. **#345 — `engram reindex --resume` skips a record whose tags changed while its content did
   not.** The skip predicate at `internal/store/store.go:2716-2717` compares the source point's
   content against the *target's* content and never looks at tags, while the vector is computed
   from `EmbedText(content, tags)` — which appends `"\n\ntags: a, b"`. So a tags-only edit leaves
   a stale vector behind and resume reports it `Unchanged`.

3. **SC3 — repairing records an earlier unpatched `--resume` already skipped.**

Out of scope: normalizing tag order at write time; any change to how `EmbedText` composes its
string; any change to the embedder lane's own credential.

</domain>

<decisions>
## Implementation Decisions

### Per-lane chat credential (#350)

- **D-01 (the config surface is an exact mirror of ChatBaseURL):** add
  `openai.chat_api_key` / `ENGRAM_OPENAI_CHAT_API_KEY` to `internal/config/registry.go` with **no**
  `Default` and **no** `Legacy` alias, and a `ChatAPIKey string` field on `OpenAIConfig`. Registry
  and struct shape copied from the `chat_base_url` entry directly above it.

- **D-02 (the inherit fallback is resolved at the construction site, never at config load):**
  `summarizerFromConfig` (`internal/server/tools.go:419-428`) gains
  `cmp.Or(cfg.OpenAI.ChatAPIKey, cfg.OpenAI.APIKey)` alongside the existing `chatBaseURL` line.
  This is D-12 from the base-URL work restated — the config field always faithfully reflects what
  the operator set, and exactly one site knows about the fallback.

- **D-03 (behavior with the key unset is byte-identical to today):** `cmp.Or` on an empty
  `ChatAPIKey` yields `cfg.OpenAI.APIKey`, which is the literal argument passed today. This is
  the requirement's "byte-identical when unset" clause and is provable by argument equality, not
  by asserting equal outbound behavior.

- **D-04 (the Helm value ships in this phase, not as a follow-up):** add
  `memory.openai.chatApiKeySecret` with its own `secretKeyRef` in `charts/engram/values.yaml` +
  `_helpers.tpl`, omitted meaning the env var is absent meaning inherit. Same reasoning that
  pulled `connect.headless` into v0.12.x Phase 1 — `charts/engram` has no generic extra-env
  escape hatch, so an env-only feature is unreachable for chart users.

- **D-05 (no startup validation of the key):** unlike a base URL, an API key has no verifiable
  shape, and empty is meaningful (inherit). A wrong key must fail at the provider, not at boot.
  No `loadAndValidate` change.

- **D-06 (configure.md carries a statement this phase makes false, and fixing it is in scope):**
  `docs-site/src/content/docs/guides/configure.md` currently asserts there is no separate key for
  the chat base URL. That sentence must be corrected, not merely supplemented, alongside the new
  table row and a `guides/upgrade.md` v0.12.0 entry.

### Reindex resume tag-awareness (#345)

- **D-07 (the target lookup is extended, not just the equality check):** `reindexTarget`
  (`store.go:2770`) gains a `tags []string`, and `reindexTargetContents` (`store.go:2782`) reads
  the target payload's `tags`. Comparing the source's tags against an always-nil field would
  either preserve the bug or defeat resume entirely — the requirement names this explicitly.

- **D-08 (both sides decode tags through the same path):** the target-side read must produce the
  same Go shape the source side already produces for `m.Tags`. An encoding asymmetry between the
  two reads surfaces as permanent false re-embeds — resume never converges — which is strictly
  worse than the bug being fixed.

- **D-09 (tag comparison is order-independent, and the residual is documented at the predicate):**
  the requirement mandates order-independence. `EmbedText` (`store.go:277-282`) joins tags in slice
  order, so a tags-permuted-only record embeds to different text yet is deliberately skipped. That
  residual is stated in a comment at the predicate rather than chased. Normalizing tag order at
  write time is the root fix and is explicitly out of scope — it touches every write path and
  every stored record.

- **D-10 (nil and empty tag slices compare equal):** a record with no tags stores no `tags` payload
  key, so the target read yields `nil` while the source may decode to an empty slice. Strict
  equality there would re-embed every untagged record on every resume, forever.

- **D-11 (the stale comment at the skip predicate is corrected, not left in place):** the comment
  at `store.go:2718-2720` asserts equal content implies equal tags "from the same source payload".
  That premise is false — it compares source content against target content, and the target's tags
  are a snapshot from whenever it was last written. It must be rewritten to describe what the code
  actually does.

- **D-12 (the paired positive control is mandatory and RED comes from mutating the predicate):**
  one test, three cases, against real Qdrant via testcontainers — content-same and
  tags-differ re-embeds; content-and-tags-same skips; same-elements-different-order skips. Without
  the skip case a fix that silently stops skipping anything goes green while re-embedding the whole
  collection on every resume. RED is observed by mutating the predicate, never by toggling a flag.

### Stale-record repair (SC3)

- **D-13 (no new command — the patched resume is itself the repair path):** vector and payload are
  written by the same upsert from the same source read, so they never disagree. A record the buggy
  predicate skipped therefore holds both the stale vector and the stale tag snapshot, and the
  patched predicate detects it on a plain re-run of `engram reindex --resume`. A dedicated
  one-time command would be a wrapper around `Reindex` taking the same arguments. This is a
  deliberate, reasoned departure from REQ-reindex-stale-repair's literal wording, which points at
  the `backfill-short-ids` / `migrate-remap-owner` precedent — those commands exist because no
  shipped path healed those records; here one does.

- **D-14 (--dry-run must honor --resume so the repair can be sized before it is run):** today
  `DryRun` short-circuits before the resume lookup (`store.go:2691-2700`), so `--dry-run --resume`
  reports every scanned record as "would be re-embedded" and tells the operator nothing about the
  blast radius. Wiring the resume lookup into the dry-run arm is what makes "just re-run it" a real
  operator tool. The dry-run guarantee that nothing is written and no target is created is
  unchanged.

- **D-15 (the source-collection-gone case is a stated limit, never a best-effort guess):** the
  correct tags live only in the source collection. If it was deleted after cutover they are
  unrecoverable. Re-embedding from the live payload would produce a vector consistent with the
  *stale* tags — silently wrong while looking healed. Document the limit; implement nothing.

- **D-16 (the repair is documented in the existing reindex guide, not a new page):**
  `docs-site/src/content/docs/guides/reindex.md` gains a "Repairing a pre-patch resume" section;
  `guides/upgrade.md`'s v0.12.0 entry names the defect and the re-run.

### Claude's Discretion

- Exact naming of any new comparison helper, and whether it lives in `internal/store` beside
  `EmbedText` or unexported next to the predicate.
- Plan/wave decomposition — #350 and #345 touch disjoint files and could be one plan or two.
- Whether the `--dry-run --resume` counts are surfaced through the existing `ReindexResult` fields
  or a new one, provided `reindexSummary`'s dry-run wording stays honest about what was counted.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets

- `cmp.Or(cfg.OpenAI.ChatBaseURL, cfg.OpenAI.BaseURL)` at `internal/server/tools.go:423` — the
  exact shape D-02 mirrors, with its D-12 rationale already written in the comment above it.
- `OpenAIConfig.ChatBaseURL` (`internal/config/config.go:132-137`) — struct field + doc comment
  to copy for `ChatAPIKey`.
- Registry entries `openai.base_url` / `openai.api_key` / `openai.chat_base_url`
  (`internal/config/registry.go:48-51`) — the new entry goes here.
- `charts/engram` already carries an `apiKeySecret` `secretKeyRef` pattern in `values.yaml` +
  `_helpers.tpl` for the shared key; D-04's value copies it.
- `EmbedText(content string, tags []string)` at `internal/store/store.go:277-282` — the function
  whose output the resume predicate is implicitly claiming to have compared.
- `reindexTargetContents` (`store.go:2782`) and `reindexTarget` (`store.go:2770`) — the two
  symbols D-07 extends.
- `TestReindexResumeSkipsUnchanged` (`internal/store/reindex_test.go:362`) and
  `TestReindexResumeRestampsStaleIdentity` (`:682`) — existing resume tests establishing the
  seed/mutate/re-run shape D-12's test follows, including the `Batch: 1` trick that forces the
  target lookup to run per page.

### Established Patterns

- Reindex resume already has a second dimension of staleness beyond content — `opts.Identity`
  (Phase 13 SC3). The predicate is already a conjunction of content plus an identity guard, so
  tags is a third conjunct in an established shape, not a new concept.
- Payload is copied into the target **verbatim**; the reindex path never synthesizes keys. The
  target's tags are therefore a faithful snapshot of the source at write time.
- One-time reconciliation commands (`migrate-remap-owner`, `backfill-short-ids`, `prune-expired`,
  `summarize-missing`) share a `--dry-run` + `--timeout` + Ctrl-C-cancellable shape. D-13 declines
  to add a fifth; D-14 borrows the dry-run-as-preflight idea instead.
- Store tests requiring a live Qdrant dial through `dialTestClient(t)` and clean up collections in
  `t.Cleanup`.

### Integration Points

- `internal/config/registry.go` + `internal/config/config.go` — new key and field.
- `internal/server/tools.go:419-428` — the single `cmp.Or` site.
- `charts/engram/values.yaml` + `charts/engram/templates/_helpers.tpl` — Helm value.
- `internal/store/store.go` — `reindexTarget`, `reindexTargetContents`, the skip predicate, and
  the `DryRun` branch.
- `cmd/engram/reindex.go` — `reindexSummary` wording if dry-run counts change meaning.
- `docs-site/src/content/docs/guides/{configure,reindex,upgrade}.md`.

</code_context>

<specifics>
## Specific Ideas

- Sean accepted all three grey areas in batch with no overrides.
- The roadmap's own note flags merge risk between this phase's `tools.go` edit
  (`summarizerFromConfig`) and v0.12.x Phase 3's (`searchMemory`). Phase 3 has since shipped, so
  the risk is resolved — but the two edits remain in different functions and the diff should stay
  small regardless.
- The whole milestone rides one branch (`git.branching_strategy: none`). No phase branch.

</specifics>

<deferred>
## Deferred Ideas

- **Normalizing tag order at write time.** The root fix for D-09's residual. Touches every write
  path and every stored record; belongs in its own phase with a migration story.
- **A dedicated `repair-*` reconciliation command.** Reconsider only if planning research
  falsifies D-13's premise that a patched `--resume` re-run heals the full affected set.
- **A separate credential for any third provider lane.** Only the chat/summarize split is in
  scope; the embedder keeps `ENGRAM_OPENAI_API_KEY`.

</deferred>
