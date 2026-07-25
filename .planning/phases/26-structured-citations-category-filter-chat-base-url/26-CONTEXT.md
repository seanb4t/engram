# Phase 26: Structured Citations, Category Filter & Chat Base URL - Context

**Gathered:** 2026-07-25
**Status:** Ready for planning
**Mode:** `--auto --chain --research` (all gray areas auto-selected and auto-resolved to the
recommended option; each selection logged in `26-DISCUSSION-LOG.md`). Grounded in the locked
ROADMAP/REQUIREMENTS text, the shipped Phase 13 (`joinEmbeddingsURL`) / Phase 20 (discovery
proto fidelity) / Phase 24 (`storeArgs` embedding) / Phase 25 (`payload()` + `TargetLocker`)
precedents, and a direct scout of `internal/store/store.go`, `internal/server/tools.go`,
`internal/summarize/summarize.go`, `internal/config/`, and `proto/engram/v1/engram.proto`.

<domain>
## Phase Boundary

Three **small, independent** extensions of existing seams close out the v0.11.x milestone. They
share no code and no ordering constraint — they are bundled for pacing only.

**Track A — Structured citations on curated memories (`REQ-memory-citations`)**
A `memory`-category record may optionally carry structured provenance using the **existing
`store.Citation` shape verbatim** (no new struct, no new collection). The `payload()` write gate
is relaxed from discovery-only to any category; `Kind` (`map`|`fact`) stays discovery-specific.
Citations are **optional and never auto-populated** — the zero-junk / no-auto-extraction invariant
is untouched.

**Track B — MCP category filter (`REQ-category-filter`)**
`search_memory` and `list_memory` accept an optional category filter over the MCP surface,
composing as a **hard Qdrant pre-filter** alongside the existing owner/scope/tags filters (applied
*before* vector ranking on search), at parity with Connect's `ListMemories`. Connect's
`SearchMemories` gains the same field so the phase closes the gap in **both** directions rather
than opening a new mirror-image one (the goal line says "MCP↔Connect category-filter parity").

**Track C — Distinct chat/summarize base URL (`REQ-chat-base-url`)**
A new `ENGRAM_OPENAI_CHAT_BASE_URL` (koanf `openai.chat_base_url`) lets the summarizer target a
different gateway than the embedder, falling back to the shared `ENGRAM_OPENAI_BASE_URL` when
empty. Resolved **only** in the summarizer construction path; the embedder path is untouched.

**Cross-cutting (SC4):** none of the three introduces new store-layer authz surface. Citations are
a payload field; the category filter composes onto the existing authz-outer-`Must` invariant
(`ownerOrSharedCondition` stays the outer constraint). No new Cedar action, no new PDP call site,
no new Qdrant index.

### Explicitly NOT this phase

- **Citations on `update_memory`** — the update field set stays content/shared/tags/summary. Editing
  citations in place is a separate capability (see Deferred).
- **Citations on `store_rule`** — rules use their own args path, not `storeArgs`; rules are
  normative ground truth, not evidence-backed claims.
- **Citations / idempotency on the Connect write lane** — REQUIREMENTS.md Deferred is explicit:
  MCP-first, Connect parity follows. (Track B's *read*-lane proto field is a different thing and
  IS in scope — see D-08.)
- **Auto-populating or inferring citations from content** — violates the no-auto-extraction
  invariant in CLAUDE.md.
- **Citation aging / staleness checks on memory citations** — discovery `pin`s already carry the
  aging signal; acting on it is not this phase.
- **A chat-specific API key** (`ENGRAM_OPENAI_CHAT_API_KEY`) — REQ is base-URL only, and the shared
  key already works for the target scenario (see Deferred for the analysis).
- **A `category` filter on `list_scheduled` / `search_discovery`** — `ListScheduled` already
  ignores `ListOptions.Tags`; discovery search filters by `Kind` instead. Out of the REQ text.

</domain>

<decisions>
## Implementation Decisions

### Track A — Citations payload shape

- **D-01 (split the gate inside `payload()`, emit citations when non-empty):** At
  `internal/store/store.go:502` the single `if m.Category == "discovery"` block writes **both**
  `p["kind"]` and `p["citations"]`. Split it: `kind` stays discovery-gated (unchanged); `citations`
  becomes gated on `len(m.Citations) > 0` for **any** category. A record with no citations writes
  no `citations` key at all — byte-identical payload to today for every existing memory record, and
  behavior-identical for discoveries (which `validateStoreDiscovery` already requires ≥1 citation
  for, so the "always write, possibly empty" branch was never observably empty).
  — **Reversibility:** costly — once memory-category records carry a `citations` payload key,
  re-gating it to discovery-only would strand that data invisible on read (or require a migration
  sweep to strip it). The relax direction is safe; the un-relax direction is not.

- **D-02 (relax it in `payload()`, NEVER via a bespoke write path — the load-bearing hazard):** The
  citations write MUST go through `payload()` (the shared whole-payload marshaller), not a targeted
  `SetPayload` bolted onto the store path. Rationale: `fromPayload` already decodes `citations`
  **ungated** (`store.go:613`), and `Store.Update` is a read-modify-**whole-payload-Upsert**
  (`store.go:1641`). Relaxing `payload()` therefore makes `Update`, `Reindex`, and every other
  round-trip preserve citations *for free*. Conversely, writing citations outside `payload()` would
  mean the next `update_memory` silently erases them — the exact cross-path lost-write class
  documented in engram memory `86q25vq6jf` (Phase 25 CR-04) and `m43h2yt97m` (Phase 24 CR-01).
  **This is the single most important thing for the planner to get right.**
  — **Reversibility:** reversible — a code-shape constraint, not a data contract.

- **D-03 (no new struct, no new field on `Memory`):** `store.Memory` already carries
  `Citations []Citation` (`store.go:198`) and `Citation` already carries
  `Kind/Ref/Locator/Pin/Excerpt` (`store.go:279`). Reuse verbatim per the REQ text. Zero struct
  churn; the change is entirely in the gate and the arg surface.

### Track A — Citations tool surface & validation

- **D-04 (`Citations []citationArg` goes on the shared `storeArgs`):** Declare it once on
  `storeArgs` (`tools.go:424`) so `store_memory`, `schedule_memory`, and `supersede_memory` all
  inherit it via Go field embedding — the exact D-13 precedent Phase 24 set for `idempotency_key`,
  which exists specifically to stop the write tools from drifting. `citationArg` (`tools.go:575`)
  is reused verbatim, so the wire shape is identical to `store_discovery`'s.
  — **Reversibility:** costly — removing a field from three advertised MCP tool schemas is a
  published-contract break for any agent that started sending it.

- **D-05 (shared `validateCitations` helper; min-count is the only difference):** Extract the
  citation loop out of `validateStoreDiscovery` (`tools.go:638-656`) into a shared
  `validateCitations(cites []citationArg, minCount int) error` enforcing: `kind ∈
  {file,commit,url,repo}` (`validCitationKind`), `ref` non-empty, `len(citations) <=
  maxDiscoveryCitations` (50), `len(excerpt) <= maxCitationExcerptBytes` (16 KiB). Discovery calls
  it with `minCount=1` (unchanged behavior); the memory path calls it with `minCount=0` (optional).
  Constant names may be de-`Discovery`-ified by the planner. Rationale: the resource-exhaustion
  guards are the *same* guards — a memory citation is not a cheaper object than a discovery
  citation, and duplicating the validator is how the two drift.

- **D-06 (memory citations are stored, never interpreted):** Citations on a memory record are inert
  provenance — they are not embedded (`EmbedText` still folds only content+tags,
  `store.go:271`), never affect ranking, never gate recall, and are not aged or verified. Same
  posture as discovery citations, minus the `>= 1` requirement.

### Track A — Citations in recall shaping

- **D-07 (citations ride the `full=true` / `get_memory` path, not the compact summary view):** The
  compact summary shape is what keeps the session-start spine bootstrap small — that is its entire
  reason to exist. Citations (with up-to-16 KiB excerpts, up to 50 of them) would blow that budget.
  So: omit citations from the default compact `search_memory`/`list_memory` result, include them on
  `full=true` and on `get_memory`. `search_discovery` is **unchanged** — it already returns
  citations always, and that is correct for a surface whose whole value is citation-backed recall.
  — **Reversibility:** reversible — a presentation choice in the MCP result shaper, no stored data
  involved.

### Track B — Category filter arg shape

- **D-08 (plural `categories []string` with OR semantics, on both MCP tools):** Expose
  `categories []string` (not a singular `category` string) on `searchArgs` and `listArgs`. Rationale:
  the store layer (`ListOptions.Categories`), the transport-neutral core (`coreListRequest.Categories`),
  and the proto (`ListMemoriesRequest.categories`, field 4) are **already plural**, so plural is the
  zero-impedance choice and is literally what SC2's "at parity with Connect's `ListMemories`" asks
  for. A single-element array covers the singular case at no cost.
  **Semantics note for the planner:** categories compose as **OR** (`listFilter` builds a
  `Should` sub-filter, `store.go:1007-1013`), unlike `tags`, which is **AND**. A record has exactly
  one category, so AND across two categories is always empty — OR is the only sane reading. The
  jsonschema description MUST say so explicitly, because the adjacent `tags` field says the
  opposite and an agent will assume symmetry.
  **Deviation flagged:** REQUIREMENTS.md and SC2 use the singular noun "`category` filter". This is
  satisfied by a plural arg accepting one element; if the verifier reads SC2 strictly as "an arg
  literally named `category`", the planner may add a singular alias — but plural is the primary.
  — **Reversibility:** costly — renaming an advertised MCP arg after agents adopt it breaks them.

- **D-09 (extract a `SearchOptions` struct rather than add a 9th positional param):**
  `Store.Search` is already `(ctx, scope, subj, vec, k, tags, after, before)` and `SearchReranked`
  mirrors it. Adding `categories []string` would place **two adjacent `[]string` parameters**
  (`tags, categories`) in a 9-arg signature — a silent transposition bug at every one of the three+
  call sites (MCP `deps.searchMemory`, Connect `SearchMemories`, the retrieval eval harness), which
  the compiler cannot catch. Instead introduce `store.SearchOptions{Tags, Categories, CreatedAfter,
  CreatedBefore}` and reshape to `Search(ctx, scope, subj, vec, k, opts)` /
  `SearchReranked(ctx, scope, subj, query, vec, k, opts)`. This **mirrors the already-shipped
  `ListOptions` convention exactly** — it is the repo's own established answer to this problem, not
  a new invention. The refactor is mechanical and fully compiler-verified.
  — **Reversibility:** costly — undoing it means touching every recall call site again; but it is
  internal-only (no wire/API contract), so the blast radius is bounded by the package.
  **Planner escape hatch:** if the refactor's diff genuinely threatens the phase, a positional
  param is acceptable — but then the two `[]string` params MUST be separated in the signature or
  the transposition hazard is real.

- **D-10 (Connect `SearchMemories` gains `repeated string categories = 8`):** Additive proto field
  on `SearchMemoriesRequest` (`proto/engram/v1/engram.proto:76-84`), plus `task proto:gen` and the
  committed `gen/` tree. Rationale: the phase goal says "**MCP↔Connect** category-filter parity".
  Closing only the list-side gap while leaving search-side asymmetric (MCP has it, Connect doesn't)
  swaps one parity bug for another. Additive field numbers are buf-lint-safe, and per engram memory
  `3tejqw6q3j` the buf remote plugins are now **version-pinned**, so regeneration is byte-identical
  and the CI `buf` drift job stays green.
  **Note:** `buf.validate`'s `in: ["decision","preference","convention","gotcha"]` constraint on
  the *write* RPCs' `category` field must NOT be copied onto this *filter* field — see D-11.
  — **Reversibility:** one-way — a published proto field number can never be reused or removed
  without breaking wire compatibility for any client that adopted it. Reserve-and-deprecate is the
  only retreat.

- **D-11 (no server-side allowlist on filter values — unknown value yields an empty result, not an
  error):** The filter passes the value through verbatim as an opaque Qdrant `Match` on the
  `category` payload key. Rationale: the legitimate *filter* domain is strictly larger than the
  *write* domain — `discovery` and `rule` are real stored categories (DEC-2bv makes discovery the
  5th category; rules are the 6th), so filtering `list_memory` by `category=rule` inside a `rule:*`
  scope is a valid query. An allowlist would have to be maintained in three places (MCP jsonschema,
  proto `buf.validate`, store) and would reject valid queries. An unknown value simply matches
  nothing — no new failure mode, no new error surface.

### Track C — Chat base URL

- **D-12 (fallback resolved at the summarizer construction site, not at config load):** Add
  `ChatBaseURL string \`koanf:"chat_base_url"\`` to `OpenAIConfig` (`internal/config/config.go:107`)
  and `{Key: "openai.chat_base_url", Env: "ENGRAM_OPENAI_CHAT_BASE_URL"}` to the registry
  (`registry.go:46`, adjacent to the `openai.embeddings_url` row it mirrors) with **no default**.
  Resolve with `cmp.Or(cfg.OpenAI.ChatBaseURL, cfg.OpenAI.BaseURL)` at the single
  `summarize.New(...)` call site (`internal/server/tools.go:369`). The config field stays a
  faithful `"" = unset`; the embedder call site (`tools.go:357`) is not touched. This is exactly
  what SC3 and the REQ text ask for ("resolved only in the summarizer path").

- **D-13 (port the shape-aware `/v1` join to the chat lane — REQ-chat-base-url is broken without
  it):** `internal/summarize/summarize.go:155` builds its endpoint with a **naive concat**:
  `c.baseURL + "/v1/chat/completions"`. The embedder got Phase 13's shape-aware
  `joinEmbeddingsURL` (`internal/embed/embed.go:125`) precisely because provider base URLs come in
  three shapes; the chat lane never did. This has been latent because every current deployment
  points at LiteLLM (`http://localhost:4000`, no `/v1` suffix) — but REQ-chat-base-url's stated
  purpose is "a local embedder **+ a hosted chat model**", and every hosted chat base URL ends in
  `/v1` (OpenAI: `https://api.openai.com/v1`) or `/v1beta/openai` (Gemini). Setting the new var to
  a real provider URL would produce `https://api.openai.com/v1/v1/chat/completions` — a 404 on
  first use. Port the same three-way switch: `…/v1beta/openai` → `+ "/chat/completions"`; `…/v1`
  → `+ "/chat/completions"`; otherwise `+ "/v1/chat/completions"`.
  **SC3 behavior-preservation holds:** for the existing default `http://localhost:4000` the output
  is byte-identical, and the only URLs whose behavior changes are ones that are *currently broken*
  (double `/v1`), so no working deployment regresses.
  — **Reversibility:** reversible — a pure URL-construction change with no stored state.

- **D-14 (hoist the join heuristic to one shared helper; do not duplicate the switch):** Rather than
  copy-pasting the ten-line switch into `internal/summarize`, extract it once as a suffix-parameterised
  helper (e.g. `Join(baseURL, "embeddings")` / `Join(baseURL, "chat/completions")`) and refactor
  `internal/embed` to call it. Rationale: a subtle provider-shape heuristic duplicated across two
  packages is exactly what silently drifts when a fourth provider shape appears. **Package placement
  is planner's discretion** — a tiny new `internal/openaiurl`, or a spot in `internal/config`
  (already imported by `internal/embed` via `ReservedParamKeys`, `embed.go:144`). Do **not** make
  `internal/summarize` import `internal/embed` — that dependency edge is backwards.
  — **Reversibility:** reversible — internal package layout only.

- **D-15 (validate only when set — exact `ENGRAM_OPENAI_EMBEDDINGS_URL` mirror):** In
  `internal/config/validate.go`, add a chat-base-URL block mirroring the `EmbeddingsURL` idiom at
  `validate.go:87-101`: skip entirely when empty; when set, require a parseable URL, an `http`/`https`
  scheme, and a non-empty host. Unlike `ENGRAM_OPENAI_BASE_URL`, empty is **valid** (it means
  "inherit"), so the empty-string error branch (`validate.go:73`) must NOT be copied.

### Track D — Cross-cutting

- **D-16 (SC4 asserted by test, not by inspection):** Add a test proving the category filter cannot
  widen visibility — a `categories` filter that would match another owner's private record still
  returns nothing, because `ownerOrSharedCondition` remains the outer `Must` (`store.go:1003-1006`).
  Same for citations: a citation-carrying `shared` record readable by a second actor is still not
  writable by them. Neither addition touches `decideRecord` / `getWritable`.

- **D-17 (three independent plans, parallelizable):** Tracks A, B, and C share no files beyond
  `tools.go` (arg structs, different lines) and have no ordering dependency. The planner should
  emit three plans and may wave them in parallel. If a single serialization point is wanted,
  Track B (the largest, because of D-09's refactor + D-10's regen) should go first.

### Claude's Discretion

- Exact Go names: `SearchOptions` field names, the shared citation-validator signature, the URL-join
  helper's package and function name, the constant renames if `maxDiscoveryCitations` is generalized.
- Whether `SearchOptions` also absorbs `k` (recommend **no** — `ListOptions` carries `Limit`, but
  `SearchReranked` deliberately rejects `k == 0` as a caller-default-discipline guard, `store.go:889`,
  and burying it in a struct weakens that).
- Whether the compact-view citation omission (D-07) is implemented by clearing the field in the MCP
  result shaper or by a dedicated summary-shape helper.
- Test-file organization; whether the Connect `SearchMemories` categories wiring gets its own
  MCP↔Connect parity test or folds into the existing parity suite.
- Whether `citationArg`/`maxDiscoveryCitations` are renamed now that they serve two categories, or
  left as-is with a doc comment.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope & requirements
- `.planning/ROADMAP.md` § "Phase 26: Structured Citations, Category Filter & Chat Base URL" — goal,
  the 4 success criteria, and the locked decision line ("relaxes the discovery-only `payload()`
  citations write-gate to any category (`Kind` stays discovery-specific) without violating DEC-2bv;
  mirrors DEC-4xt7's tag-filter hard-AND-pre-filter pattern for category filtering").
- `.planning/REQUIREMENTS.md` — `REQ-memory-citations` (§74-78), `REQ-category-filter` (§82-86),
  `REQ-chat-base-url` (§90-94); the **Deferred** entry pinning Connect *write*-lane citations/
  idempotency to a later milestone (§107-108); the **Out of Scope** table (§110-115).

### Locked ADRs governing this phase
- `docs/adr/engram-2bv-discovery-is-5th-category-single-memory-collection.md` (DEC-2bv) — one
  Memory collection; citations on a memory record are a payload field on it, never a new
  collection, and discovery-as-5th-category is why D-11's filter domain is larger than the write
  domain.
- `docs/adr/engram-4xt7-tag-filtered-recall-hard-qdrant-filter-and-default.md` (DEC-4xt7) — the
  hard-pre-filter-before-vector-ranking pattern the category filter mirrors. **Read the semantics
  carefully:** tags are AND, categories are OR (D-08).
- `docs/adr/engram-ef28-index-owner-scope-created-at-as-qdrant-payload-indexes.md` (DEC-ef28) —
  owner/scope/created_at are the only payload indexes. The category filter adds **no** index (it is
  an unindexed payload match, same as `tags` and `visibility` today).
- `docs/adr/engram-cgb-enforce-per-actor-authorization-store-layer-not-handlers.md` (DEC-cgb) +
  `docs/adr/engram-cdr1-cedar-pdp-decides-predicate-store-enforces-qdrant-filter.md` (DEC-cdr1) —
  the store is the sole authz chokepoint and the authz condition is the outer `Must`. SC4 is the
  statement that neither addition perturbs this.

### Prior phase context (patterns this phase reuses)
- `.planning/phases/24-idempotent-capture/24-CONTEXT.md` § D-13 — the shared-`storeArgs`-embedding
  rationale D-04 reuses verbatim (one declaration, three write tools, no drift).
- `.planning/phases/25-supersession-with-history/25-CONTEXT.md` § D-01/D-09 — the
  `SetPayload`-vs-whole-`Upsert` distinction and the recall-gate composition shape.

### Milestone research
- `.planning/research/PITFALLS.md`, `.planning/research/STACK.md`, `.planning/research/SUMMARY.md` —
  v0.11.x research; primarily idempotency/authz-focused, but the payload-stamp and
  filter-composition discussions apply.
- `.planning/codebase/ARCHITECTURE.md`, `.planning/codebase/CONVENTIONS.md`,
  `.planning/codebase/TESTING.md` — repo-wide conventions the three tracks must match.

### Prior-art hazards (engram memory — fetch full text with `get_memory`)
- `86q25vq6jf` — **the critical one.** Cross-path lost-write: any whole-payload `Upsert` on an
  existing record erases payload keys a concurrent path set via `SetPayload`. Its closing line
  names this phase directly: *"Phase 26 adds more payload keys (citations) — any new whole-payload
  writer MUST preserve `superseded_by` (and every other out-of-band key) or take the lock."*
  D-02 is the resolution.
- `m43h2yt97m` — Phase 24 CR-01: deterministic `Upsert` fully replaces the payload (no merge).
- `3tejqw6q3j` — unpinned CI toolchain broke `main` twice; buf remote plugins are **now pinned**, so
  D-10's `task proto:gen` regeneration is byte-identical. Also: `golangci` passes while CI's
  `gofmt -l .` fails — **always run `gofmt` on changed `.go` files before pushing.**
- `a6gw97hfwg` / `q26sf43wzk` — `protect-main` ruleset requires review-thread resolution; ship-note
  `[ci skip]` strands PRs at BLOCKED. Relevant at `/gsd-ship` time, not during implementation.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

**Track A**
- `internal/store/store.go:279` `Citation{Kind,Ref,Locator,Pin,Excerpt}` and `store.go:198`
  `Memory.Citations []Citation` — **already exist**. D-03 reuses them verbatim; no struct change.
- `internal/store/store.go:502-512` — the single `if m.Category == "discovery"` block writing both
  `p["kind"]` and `p["citations"]`. This is the exact gate D-01 splits.
- `internal/store/store.go:613-627` `fromPayload` — **already decodes `citations` ungated**, no
  category check. This is why relaxing only the write gate is sufficient (D-02).
- `internal/server/tools.go:575` `citationArg` + `tools.go:660` `validCitationKind` +
  `tools.go:638-656` the validation loop — lifted into the shared validator (D-05).
- `internal/server/tools.go:907-909` — the `citationArg → store.Citation` mapping loop in
  `storeDiscovery`; the memory path needs the identical mapping.

**Track B**
- `internal/store/store.go:1002-1029` `listFilter` — **already builds the category OR-filter**
  (`Should` sub-filter appended to `must`, `:1007-1013`). List-side store work is **zero**.
- `internal/server/tools.go:970-981` `coreListRequest` — **already carries `Categories []string`**
  and `deps.listMemory` already threads it into `ListOptions` (`tools.go:1021`). List-side core
  work is **zero**; only the MCP `listArgs` field + closure wiring is missing.
- `internal/store/store.go:760` `tagMatchConditions` — the shape a `categoryMatchCondition` helper
  mirrors, except OR (`Should`) rather than AND (`Must`).
- `internal/store/store.go:950-971` `ListOptions` — **the precedent D-09's `SearchOptions` copies.**
  The repo already solved "too many filter params" this way once.

**Track C**
- `internal/embed/embed.go:125-135` `joinEmbeddingsURL` — the three-way provider-shape switch
  (`/v1beta/openai`, `/v1`, bare) D-13/D-14 generalize.
- `internal/config/registry.go:44-46` — the `openai.*` rows; `openai.embeddings_url`
  (`ENGRAM_OPENAI_EMBEDDINGS_URL`, no default) is the **exact structural precedent** for
  `openai.chat_base_url`.
- `internal/config/validate.go:87-101` — the "validate only when set" block D-15 mirrors.
- `internal/server/tools.go:357` (`embed.New`) and `tools.go:369` (`summarize.New`) — the two
  adjacent construction sites; only the second changes.

### Established Patterns
- **Optional payload keys are written only when present** (`p["summary_model"]` at `store.go:493`,
  `p["short_id"]` at `:499`) — D-01's non-empty gate matches this exactly.
- **One declaration on `storeArgs`, three write tools inherit** (Phase 24 D-13, `tools.go:435-438`)
  — the drift-prevention seam D-04 reuses.
- **Transport-neutral `core*Request` structs** decouple MCP from Connect (`tools.go:970`/`:1000`);
  every new filter belongs there, not in a lane-specific handler.
- **Filter-params-as-struct** (`ListOptions`) once a signature exceeds a handful of args — D-09.
- **Full-URL override + shape-aware fallback join** for provider endpoints (Phase 13, embed lane) —
  D-13 ports the fallback half to the chat lane.
- Authz stays the **outer `Must`**; request filters are always appended inside it. Never reorder.

### Integration Points
- `internal/store/store.go` — split the `payload()` gate (D-01); add `SearchOptions` + thread
  categories through `Search`/`SearchReranked` (D-09); a `categoryMatchCondition` helper shared by
  `listFilter` and the search filter.
- `internal/server/tools.go` — `Citations` on `storeArgs` (D-04); `Categories` on `searchArgs` and
  `listArgs` (D-08); `Categories` on `coreSearchRequest`; shared `validateCitations` (D-05);
  `cmp.Or` chat-base-URL resolution at the `summarize.New` call site (D-12); compact-view citation
  omission (D-07).
- `internal/summarize/summarize.go` — endpoint construction via the shared join helper (D-13/D-14).
- `internal/embed/embed.go` — refactored to call the hoisted helper (D-14), behavior unchanged.
- `internal/config/{config.go,registry.go,validate.go}` — the new `chat_base_url` field, registry
  row, and set-only validation (D-12/D-15).
- `proto/engram/v1/engram.proto` + `gen/` — `repeated string categories = 8` on
  `SearchMemoriesRequest`, plus `task proto:gen` with the regenerated tree committed in the **same
  commit** (D-10; CI's `buf` job checks for drift).
- `internal/server/connectapi.go` — wire `req.Msg.Categories` into `coreSearchRequest` (mirrors the
  existing `ListMemories` wiring at `connectapi.go:155`).
- **Docs surface (do not skip):** `docs-site` tool/config pages, `charts/engram/` values +
  README for the new env var, and the `curating-memory` skill if citations change agent guidance.
  Phase 25's lesson (engram `hkb8bwknpb`) was explicit: *a tool with no skill/doc guidance is an
  incomplete feature.*

</code_context>

<specifics>
## Specific Ideas

- **D-02 is the phase's highest-risk decision.** Engram memory `86q25vq6jf` names Phase 26 by
  number as the place this hazard could recur. If the planner routes citations through anything
  other than `payload()`, `update_memory` will silently drop them and the bug will surface as
  "citations randomly disappear after an edit" — expensive to diagnose. A regression test that
  stores a memory *with* citations, calls `update_memory`, and re-fetches asserting the citations
  survived is the cheapest possible insurance and should be non-negotiable.

- **D-13 is the difference between shipping the feature and shipping a 404.** The naive
  `baseURL + "/v1/chat/completions"` concat means the very first thing an operator will try
  (`ENGRAM_OPENAI_CHAT_BASE_URL=https://api.openai.com/v1`) fails. Add a table test covering all
  three shapes plus the existing LiteLLM default.

- **The OR-vs-AND asymmetry (D-08) will confuse agents** unless the jsonschema says so. `tags`
  literally reads "records carrying ALL listed tags"; `categories` sitting next to it must read
  something like "records in ANY of the listed categories". Copy the wording discipline the
  existing filter args already use.

- **SC-verification oracle:**
  SC1 → store a `memory`-category record with citations, `get_memory` returns them; store one
  without, payload has no `citations` key; nothing auto-populates them.
  SC2 → `search_memory`/`list_memory` with `categories` return only matching records; on search the
  filter is applied pre-ranking (a filtered-out record cannot appear even at rank 1); MCP and
  Connect return the same set for the same filter.
  SC3 → `ENGRAM_OPENAI_CHAT_BASE_URL` set → summarizer hits that host, embedder still hits
  `ENGRAM_OPENAI_BASE_URL`; unset → both hit the shared URL, byte-identical to today.
  SC4 → the D-16 visibility tests.

- **Definition of done includes docs + Helm + skill**, per the Phase 25 lesson. Three small code
  tracks, but the new env var needs a chart value and a docs-site row, and memory citations need a
  line in `curating-memory` / `docs-site` `tools/memory-record` or agents will never use them.

</specifics>

<deferred>
## Deferred Ideas

- **Editing citations via `update_memory`** — the update field set stays content/shared/tags/summary
  this phase. Adding a `citations` field there means deciding replace-vs-merge semantics (tags chose
  "replace, omit to preserve") and is its own small design. File as a follow-up if agents ask for it.

- **`ENGRAM_OPENAI_CHAT_API_KEY`** — a per-lane API key alongside the per-lane base URL. Deliberately
  out of scope (the REQ is base-URL only), and it is **not blocking**: the target scenario is a local
  embedder + a hosted chat model, and local embedders (Ollama/vLLM/LiteLLM) ignore the
  `Authorization` header, so setting the single shared `ENGRAM_OPENAI_API_KEY` to the hosted chat key
  already works. It becomes necessary only when *both* lanes are hosted with different providers.

- **Per-lane chat timeout / model options** (`ENGRAM_OPENAI_CHAT_TIMEOUT`, chat-side params to
  mirror `ENGRAM_EMBED_*_PARAMS`) — same "per-lane provider" family as this REQ, not in it.

- **Citations on `store_rule`** — rules are user-blessed normative ground truth, not evidence-backed
  claims; the citation model does not obviously apply. Revisit only if a real need appears.

- **Citation aging / staleness verification for memory citations** — discovery `pin`s already encode
  the aging signal; a tool that re-checks pins and flags drifted citations is a genuine future
  feature, and would apply to both categories at once.

- **Idempotency + citations on the Connect *write* lane** — already tracked in REQUIREMENTS.md
  Deferred ("v0.11.x lands these on the MCP `store_memory` path first; Connect parity follows").
  Note this is unrelated to D-10, which is a *read*-lane filter field.

- **Category filter on `list_scheduled` / `search_discovery`** — `ListScheduled` already ignores
  `ListOptions.Tags` (documented at `store.go:955`) and discovery search filters by `Kind`. Adding
  category there is a consistency nicety, not a requirement.

- **Pre-existing unpinned CI tools** — `go install actionlint@latest` and `task@latest` in
  `ci.yaml` remain unpinned (engram `3tejqw6q3j`, latent same-class risk). Not this phase; worth a
  standalone issue.

</deferred>

---

*Phase: 26-structured-citations-category-filter-chat-base-url*
*Context gathered: 2026-07-25 (auto mode)*
