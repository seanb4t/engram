# Phase 26: Structured Citations, Category Filter & Chat Base URL - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-25
**Phase:** 26-structured-citations-category-filter-chat-base-url
**Mode:** `--auto --chain --research` — no interactive prompts; every area auto-resolved to the
recommended option, grounded in a direct codebase scout rather than in generic defaults.
**Areas discussed:** Citations payload-gate shape · Citations write-path routing · Citations tool
surface · Citations validation · Citations in recall shaping · Category arg shape · Search
filter-param plumbing · Connect SearchMemories parity · Category value allowlist · Chat base-URL
fallback point · Chat `/v1` join shape-awareness · Chat config validation · SC4 verification ·
Plan decomposition

---

## Citations payload-gate shape (D-01)

| Option | Description | Selected |
|--------|-------------|----------|
| Split the gate; emit `citations` only when non-empty | `kind` stays discovery-gated; `citations` written for any category when `len > 0`. Byte-identical payload for citation-less records. | ✓ |
| Relax the whole block to all categories | Every memory record gains `citations: []` and a `kind: ""` key — payload bloat + a meaningless `kind` on non-discoveries. | |
| Keep `citations` discovery-only, add a parallel `memory_citations` key | A second key for the same concept; violates "reuse the existing Citation shape verbatim". | |

**Selection:** Split the gate, emit when non-empty (recommended default).
**Notes:** Matches the repo's existing optional-key discipline (`summary_model`, `short_id` are
written only when set). Behavior-identical for discoveries, which `validateStoreDiscovery` already
forces to carry ≥1 citation.

---

## Citations write-path routing (D-02) — highest-risk area

| Option | Description | Selected |
|--------|-------------|----------|
| Relax the gate inside `payload()` | The shared whole-payload marshaller. `Update`/`Reindex` round-trip citations for free because `fromPayload` already decodes them ungated. | ✓ |
| Write citations via a targeted `SetPayload` outside `payload()` | Would be silently erased by `Store.Update`'s read-modify-whole-payload-Upsert. | |

**Selection:** Relax inside `payload()`.
**Notes:** Engram memory `86q25vq6jf` (Phase 25 CR-04) names Phase 26 by number as the place this
cross-path lost-write hazard could recur. The `SetPayload` alternative is not merely worse — it is
the documented bug. Flagged in CONTEXT.md as the single most important planner constraint, with a
mandatory store→update→refetch regression test.

---

## Citations tool surface (D-04)

| Option | Description | Selected |
|--------|-------------|----------|
| `Citations` on shared `storeArgs` | One declaration; `store_memory`, `schedule_memory`, `supersede_memory` all inherit via Go embedding. Phase 24 D-13 precedent. | ✓ |
| `store_memory` only, hand-rolled field | Reintroduces exactly the three-tool drift that `persistAndEnqueue` / shared `storeArgs` exist to prevent. | |
| Also extend `update_memory` | Requires deciding replace-vs-merge semantics; beyond the REQ. | |

**Selection:** Shared `storeArgs` (recommended default). `update_memory` deferred.

---

## Citations validation (D-05)

| Option | Description | Selected |
|--------|-------------|----------|
| Shared `validateCitations(cites, minCount)`; discovery passes 1, memory passes 0 | Same resource-exhaustion guards (≤50 citations, ≤16 KiB excerpt, kind allowlist), one implementation. | ✓ |
| Separate memory-side validator | Duplicates the caps; the two drift the first time one is tuned. | |
| No caps on memory citations | A memory citation is not a cheaper object than a discovery citation. | |

**Selection:** Shared validator with a min-count parameter.

---

## Citations in recall shaping (D-07)

| Option | Description | Selected |
|--------|-------------|----------|
| Citations on `full=true` + `get_memory`; omitted from the compact view | Preserves the compact summary shape that keeps session-start spine bootstrap small. | ✓ |
| Citations always returned | Up to 50 citations × 16 KiB excerpts would blow the compact-recall budget — the exact thing the summary view exists to protect. | |
| Citations never returned over recall, `get_memory` only | Unnecessarily strict; `full=true` already means "give me everything". | |

**Selection:** `full=true` / `get_memory` only. `search_discovery` unchanged (citations always).

---

## Category arg shape (D-08)

| Option | Description | Selected |
|--------|-------------|----------|
| Plural `categories []string`, OR semantics | Matches `ListOptions.Categories`, `coreListRequest.Categories`, and `ListMemoriesRequest.categories` (all already plural). Zero impedance. | ✓ |
| Singular `category string` | Literally matches the REQ's noun, but mismatches every existing layer and Connect parity — the load-bearing SC2 clause. | |
| Both (plural primary + singular alias) | Two args for one concept on a published schema. | |

**Selection:** Plural, OR semantics, with the AND-vs-OR asymmetry spelled out in the jsonschema.
**Notes:** Deviation from the REQ/SC singular noun explicitly flagged in CONTEXT.md, with a stated
escape hatch if a verifier reads SC2 strictly.

---

## Search filter-param plumbing (D-09)

| Option | Description | Selected |
|--------|-------------|----------|
| Extract `store.SearchOptions{Tags, Categories, CreatedAfter, CreatedBefore}` | Mirrors the already-shipped `ListOptions` convention — the repo's own answer to this problem. Mechanical, compiler-verified. | ✓ |
| Add a 9th positional param | Places two adjacent `[]string` params (`tags, categories`) in a 9-arg signature across 3+ call sites — a silent transposition bug the compiler cannot catch. | |
| Variadic functional options | Heavier than the problem; no precedent for filters in this codebase. | |

**Selection:** `SearchOptions` struct, with a documented escape hatch to positional (params
separated) if the refactor's diff threatens the phase.

---

## Connect `SearchMemories` parity (D-10)

| Option | Description | Selected |
|--------|-------------|----------|
| Add `repeated string categories = 8`, regenerate `gen/` | Phase goal says "**MCP↔Connect** parity". Additive field numbers are buf-lint-safe; buf plugins are now pinned so regen is byte-identical. | ✓ |
| MCP-only; leave Connect search without categories | Closes the list-side gap while opening a mirror-image search-side one. | |

**Selection:** Add the proto field.
**Notes:** Rated **one-way** — a published proto field number can never be reused. Also flagged:
do NOT copy the write RPCs' `buf.validate` category allowlist onto this filter field (see D-11).

---

## Category value allowlist (D-11)

| Option | Description | Selected |
|--------|-------------|----------|
| No allowlist — opaque pass-through; unknown value matches nothing | The legitimate *filter* domain (incl. `discovery`, `rule`) is strictly larger than the *write* domain (the 4 memory categories). | ✓ |
| Enforce the 4 write categories | Would reject `category=rule` inside a `rule:*` scope — a valid query. | |
| Enforce all 6 known categories | An allowlist maintained in three places (jsonschema, proto validate, store) with no failure it prevents. | |

**Selection:** No allowlist.

---

## Chat base-URL fallback point (D-12)

| Option | Description | Selected |
|--------|-------------|----------|
| `cmp.Or(ChatBaseURL, BaseURL)` at the `summarize.New` call site | Config field stays a faithful `"" = unset`; embedder path untouched. Exactly what SC3 asks. | ✓ |
| Materialize the fallback at config load | Loses the distinction between "unset" and "explicitly equal", and makes the config struct lie about what the operator set. | |

**Selection:** Resolve at the construction site.
**Notes:** Registry row placed adjacent to `openai.embeddings_url`, whose no-default,
validate-only-when-set shape it mirrors exactly.

---

## Chat `/v1` join shape-awareness (D-13) — the "feature ships broken without it" area

| Option | Description | Selected |
|--------|-------------|----------|
| Port the three-way shape-aware join to the chat lane | `…/v1beta/openai` and `…/v1` get `+ /chat/completions`; bare host gets `+ /v1/chat/completions`. | ✓ |
| Leave the naive `baseURL + "/v1/chat/completions"` concat | `ENGRAM_OPENAI_CHAT_BASE_URL=https://api.openai.com/v1` → `…/v1/v1/chat/completions` → 404 on first use, which is the REQ's own headline scenario. | |
| Require operators to supply a full endpoint URL | Inconsistent with the `*_BASE_URL` naming and with the embedder lane's behavior. | |

**Selection:** Port the join.
**Notes:** SC3's "zero configuration-change behavior impact" verified to hold — the LiteLLM default
`http://localhost:4000` produces a byte-identical URL, and the only URLs whose behavior changes are
ones currently producing a double `/v1` (i.e. already broken).

---

## Join-helper placement (D-14)

| Option | Description | Selected |
|--------|-------------|----------|
| Hoist to one shared suffix-parameterised helper; refactor `internal/embed` to call it | Single source of truth for a subtle provider-shape heuristic. | ✓ |
| Duplicate the 10-line switch in `internal/summarize` | Exactly the duplication that drifts when a fourth provider shape appears. | |
| `internal/summarize` imports `internal/embed` | Backwards dependency edge. | |

**Selection:** Shared helper; package placement left to the planner.

---

## Chat config validation (D-15)

| Option | Description | Selected |
|--------|-------------|----------|
| Mirror the `ENGRAM_OPENAI_EMBEDDINGS_URL` block — validate only when set | Empty is valid (means "inherit"), so the empty-string error branch is deliberately not copied. | ✓ |
| No validation | An unparseable chat URL would fail at first summarize instead of at startup. | |

**Selection:** Mirror the embeddings-URL idiom.

---

## SC4 verification (D-16)

| Option | Description | Selected |
|--------|-------------|----------|
| Assert by test | A `categories` filter that would match another owner's private record still returns nothing; a citation-carrying `shared` record readable by a second actor is still not writable by them. | ✓ |
| Assert by code inspection / comment | "No new authz surface" is a claim worth a test, not a comment. | |

**Selection:** Test-asserted.

---

## Plan decomposition (D-17)

| Option | Description | Selected |
|--------|-------------|----------|
| Three independent plans, parallelizable | Tracks A/B/C share no files beyond different line ranges of `tools.go` and have no ordering dependency. | ✓ |
| One combined plan | Bundles three unrelated changes into one reviewable unit. | |
| Serialize A → B → C | No dependency justifies the wall-clock cost. | |

**Selection:** Three plans, wave-parallel. If a serialization point is wanted, Track B first
(largest — `SearchOptions` refactor + proto regen).

---

## Claude's Discretion

Left explicitly open for the planner in CONTEXT.md:

- Exact Go names: `SearchOptions` fields, the shared citation-validator signature, the URL-join
  helper's package and function name, and whether `maxDiscoveryCitations` / `citationArg` are
  renamed now that they serve two categories.
- Whether `SearchOptions` absorbs `k` (recommendation recorded: **no** — `SearchReranked`'s
  `k == 0` rejection is a deliberate caller-default-discipline guard that a struct would weaken).
- Whether the compact-view citation omission is a result-shaper clear or a dedicated summary-shape
  helper.
- Test-file organization; whether the Connect search-categories wiring gets a dedicated parity test.

## Deferred Ideas

Captured in CONTEXT.md `<deferred>` rather than acted on:

- Editing citations via `update_memory` (needs replace-vs-merge semantics).
- `ENGRAM_OPENAI_CHAT_API_KEY` — analyzed and found **non-blocking**: local embedders ignore the
  `Authorization` header, so the single shared key already covers the local-embedder + hosted-chat
  scenario. Only needed when both lanes are hosted with different providers.
- Per-lane chat timeout / chat-side model params.
- Citations on `store_rule` (rules are normative ground truth, not evidence-backed claims).
- Citation aging / staleness verification (would apply to both categories at once).
- Idempotency + citations on the Connect *write* lane (already in REQUIREMENTS.md Deferred;
  unrelated to D-10's *read*-lane filter field).
- Category filter on `list_scheduled` / `search_discovery`.
- Pre-existing unpinned CI tools (`actionlint@latest`, `task@latest`) — same latent class as engram
  `3tejqw6q3j`; worth a standalone issue, not this phase.

## Scope-creep redirections

- **Connect `SearchMemories` categories (D-10)** was evaluated against the scope guardrail and
  admitted, not deferred: the phase goal line reads "MCP↔Connect category-filter parity", which is
  bidirectional. Recorded as one-way (proto field number) so the planner treats it as a
  checkpoint-worthy decision.
- **The chat `/v1` join fix (D-13)** was evaluated as possible creep and admitted: without it
  `ENGRAM_OPENAI_CHAT_BASE_URL` cannot reach any hosted provider, so it is required for SC3 to be
  meaningfully true rather than an adjacent improvement.
- **`ENGRAM_OPENAI_CHAT_API_KEY`** was the one candidate genuinely redirected to Deferred, after
  confirming it does not block the REQ's stated use case.
