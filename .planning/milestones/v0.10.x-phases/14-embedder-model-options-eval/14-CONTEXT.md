# Phase 14: Embedder Model Options & Eval - Context

**Gathered:** 2026-07-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Make Gemini a first-class, **eval-verified** embedding option and close the last
v0.9.x eval follow-up (#261/#334), then document the supported models as
copy-paste operator recipes. This is a **docs + eval-correctness** phase — no
proto/Connect wire changes, no new embed connection wiring (Phase 13's D-10/D-11
base-URL join already handles Gemini's `/v1beta/openai` shape).

Deliverables (three requirements, GitHub #331 / #334 / #337):

1. **Gemini direct + task_type correctness gate** (REQ-embed-gemini-direct, #331) —
   engram embeds queries and documents against the current-GA Gemini embeddings
   model via its OpenAI-compat endpoint, with the wire shape verified against
   live docs and asymmetric `task_type` behavior **proven** (query vector ≠
   document vector) by a `task eval:retrieval` run. A silent `task_type` no-op is
   a recall regression with no error to catch it (PITFALLS.md Pitfall 12) — this
   is a correctness gate, not a docs note.
2. **#261 prod-parity re-confirm** (REQ-embed-prod-parity-eval, #334, closes #261) —
   re-run the `gh261Case` recall@8 hard gate against the real qwen3-embedding-8b
   @4096 config (with query instruction), now that the Phase-13 embed-timeout knob
   makes long/cloud eval runs reliable.
3. **Model-recipe docs** (REQ-embed-model-docs, #337) — a docs-site guide +
   commented Helm `values.yaml` recipes documenting the supported models
   (OpenRouter / Gemini / OpenAI / local TEI-Ollama-vLLM), each pairing base URL +
   model + vector dim + query instruction, with every model/dim change called out
   as requiring `engram reindex` (cross-linking `guides/reindex`).

**Framing (load-bearing):** engram documents recipes as **equal operator
choices** — there is NO blessed/standardized production model or hosting
provider. Model and provider are the operator's call. engram picks *concrete*
configs (OpenRouter-hosted qwen3, a specific Gemini config) **only as the
eval/CI reference**, never as a mandated recommendation.

</domain>

<decisions>
## Implementation Decisions

### Gemini recipe & wire (REQ-embed-gemini-direct / #331)

- **D-01:** Canonical Gemini model = the **current-GA Gemini embedding model**.
  The user named it "Gemini Embedder 2"; the exact model id (e.g.
  `gemini-embedding-001` vs a newer gen) and its OpenAI-compat wire shape (the
  `task_type` values `RETRIEVAL_QUERY` / `RETRIEVAL_DOCUMENT`, param location, and
  native dimension) MUST be **verified against live Gemini docs by the
  researcher** — success criterion #1 mandates "verified against live docs," so
  do not assume from training data.
- **D-02:** Ship the Gemini recipe at the model's **native full output
  dimension** (no MRL/`output_dimensionality` truncation). The concrete native
  dim value comes from D-01's doc verification.
- **D-03:** Gemini connectivity itself needs **no new wiring** — reuse the
  existing `embed.Client` OpenAI-compat path and Phase-13 base-URL join. The work
  is (a) the correct query/document param maps for Gemini's `task_type`, and
  (b) the eval assertion that they actually take effect.

### task_type correctness gate (REQ-embed-gemini-direct / #331)

- **D-04:** The gate is a **new live eval fixture case** in
  `internal/retrievaleval/` — embed a single text both query-side and
  document-side through the production embed path and **assert the two vectors
  differ**. Proving the provider *honors* `task_type` (not merely that engram
  *sends* it) is the whole point, so a request-body-only unit test is
  insufficient on its own.
- **D-05:** The Gemini differ-case is a **permanent, skip-gated** fixture
  alongside `gh261Case` — skipped unless `ENGRAM_RETRIEVAL_EVAL=1` and the Gemini
  env are set. It stays in the suite as a re-runnable regression guard, not a
  one-time check.

### Eval execution & evidence (REQ-embed-gemini-direct + REQ-embed-prod-parity-eval)

- **D-06:** Execution is **local/manual**. Keep the existing
  `ENGRAM_RETRIEVAL_EVAL=1` skip gate; **document** the exact env + the
  `task eval:retrieval` command (in the eval/reindex/embedding-models guide) so a
  developer runs it before merge. **No secrets in CI**, no new CI job this phase.
- **D-07:** Evidence that closes success criteria #1 and #2 is a **committed run
  artifact** — capture the eval output (recall@8 numbers + the Gemini
  differ-assertion pass) into a committed results/verification file so the pass
  is auditable later. (Planner: choose a stable location — e.g. under the phase
  dir or an `eval/` results file.)
- **D-08:** The #261 parity **reference endpoint = OpenRouter-hosted
  qwen3-embedding-8b @4096** (reproducible with an API key). This is the *eval
  reference config*, chosen for reproducibility — NOT a standard operators must
  adopt. Document the exact env to reproduce.

### #261 parity re-confirm (REQ-embed-prod-parity-eval / #334, closes #261)

- **D-09:** **Reuse the existing hard gate** — `gh261Case` already asserts "Record
  T within default k" as a hard assertion (Phase 9 Plan 03). Phase 14 re-points it
  at the real qwen3@4096 config; **no new threshold is invented**. (The observed
  MRR MAY be recorded in the artifact as an informational baseline.)
- **D-10:** qwen3-embedding-8b@4096 is **both** a documented recipe in the
  embedding-models guide **and** the #261 eval reference — but per the
  operator-choice framing (D-00 framing), it is presented as one option among
  several, not a recommended default.

### Model-recipe docs (REQ-embed-model-docs / #337)

- **D-11:** Recipes live in a **new dedicated page `guides/embedding-models.md`**.
  The existing `guides/embedding-instructions.md` stays focused on
  instruction/param *tuning mechanics* and **cross-links** to the new page (clean
  split: "which model + how to connect" vs "how to tune instructions").
- **D-12:** Recipe format = **at-a-glance comparison table** (model | base URL |
  dim | query instruction | reindex note) **PLUS per-provider copy-paste env
  blocks** (OpenRouter / Gemini / OpenAI / local TEI-Ollama-vLLM).
- **D-13:** Helm `charts/engram/values.yaml` carries **commented recipes inline** —
  keep a neutral uncommented default (current `ollama/bge-m3`) and add
  commented-out OpenRouter / Gemini / OpenAI blocks with dim + instruction, each
  noting reindex. The values file stays self-documenting.
- **D-14:** Every recipe (docs table + Helm comments) **explicitly calls out that
  a model or dim change requires `engram reindex`**, cross-linking
  `guides/reindex` (ties to the Phase-13 embedder-config-identity reindex
  boundary).

### Claude's Discretion

- Exact committed-artifact file location and format (D-07).
- Whether the Gemini differ-case reuses the `gh261` dataset or a minimal
  2-record differ probe (D-04).
- Exact query/document param-map keys for Gemini `task_type` — pending D-01 live-doc
  verification.
- Keeping `ollama/bge-m3` vs another neutral symmetric model as the Helm default
  (D-13) — a concrete default is required; the choice is neutral.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Scope anchors
- `.planning/ROADMAP.md` § Phase 14 — goal + 3 success criteria (the fixed boundary).
- `.planning/REQUIREMENTS.md` — `REQ-embed-gemini-direct` (#331),
  `REQ-embed-prod-parity-eval` (#334, closes #261), `REQ-embed-model-docs` (#337).
- `.planning/research/PITFALLS.md` — Pitfall 12 (silent `task_type` no-op is a
  recall regression with no error; the correctness-gate rationale).
- `.planning/phases/13-embedder-reliability-foundation/13-CONTEXT.md` — Phase-13
  embedder decisions: D-10/D-11 base-URL join (already handles Gemini
  `/v1beta/openai`), D-01 embedder-config-identity + reindex boundary, D-07
  embed-timeout (this phase's dependency).

### Eval harness (source of truth)
- `internal/retrievaleval/retrieval_eval_test.go` — `TestRetrievalEval`,
  `ENGRAM_RETRIEVAL_EVAL=1` skip gate, `StoreAndEmbedderFromEnvNoEnsure()`
  prod-parity builder, testcontainer store, `recallAtK`.
- `internal/retrievaleval/fixtures.go` — `gh261Case` (#261 regression fixture,
  Record T + sticky distractors), `retrievalCases` slice (add the Gemini
  differ-case here).
- `internal/retrievaleval/doc.go` — package intent.
- `Taskfile.yaml` § `eval:retrieval` / `eval:summary` — the run targets.

### Embedder subsystem
- `internal/embed/embed.go` — `Client`, `embed()`, functional `Option`s
  (`WithQueryParams`, `WithDocumentInstruction`, base-URL join from Phase 13).
- `internal/config/embedparams.go` — `ParseEmbedParams` (query vs document param
  maps that carry `task_type`).
- `internal/config/config.go` / `registry.go` / `validate.go` — `EmbedConfig` /
  `OpenAIConfig`, koanf field registry, embed validation.
- `internal/server/tools.go` — `embedderFromConfig`, `StoreAndEmbedderFromEnvNoEnsure`.

### Docs & chart targets
- `docs-site/src/content/docs/guides/embedding-instructions.md` — existing
  per-model instruction guide (cross-link target; already covers Qwen3 / BGE /
  Google / OpenAI / Cohere / Voyage / Jina instruction values).
- `docs-site/src/content/docs/guides/reindex.md` — reindex boundary (cross-link
  from every recipe per D-14).
- `docs-site/src/content/docs/guides/configure.md` — config landing page.
- `charts/engram/values.yaml` — `memory.embed.*` (model/dim/queryInstruction/
  params/documentInstruction) + `memory.openai.*` (baseURL/apiKeySecret); the D-13
  commented-recipe target.
- **NEW:** `docs-site/src/content/docs/guides/embedding-models.md` — the recipes
  page to create (D-11).

### Codebase maps
- `.planning/codebase/CONVENTIONS.md`, `.planning/codebase/STACK.md`,
  `.planning/codebase/INTEGRATIONS.md` — Go conventions, stack, embedder integration.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`internal/retrievaleval` harness** — already a hard-gated recall@k eval
  (`gh261Case`), skip-gated on `ENGRAM_RETRIEVAL_EVAL=1`, seeds through the
  production doc-embed path and searches the production query path. Add the Gemini
  differ-case to `retrievalCases` and re-point the qwen3 config; do NOT build a
  new eval framework.
- **`StoreAndEmbedderFromEnvNoEnsure()`** (`internal/server`) — the exported
  prod-parity embedder builder the eval already reuses; the Gemini/qwen3 configs
  flow in via env, not new code.
- **`embed.Option` query/document param maps** (`WithQueryParams` /
  `ParseEmbedParams`) — Gemini `task_type` rides these; no new option needed.
- **Phase-13 base-URL join** — already resolves Gemini `/v1beta/openai` →
  `/embeddings`; Gemini connectivity is config, not code.

### Established Patterns
- Eval gates are **skip-by-default, env-opted-in** (mirror `gh261Case` and
  `internal/summarize/fidelity_test.go`'s `TestSummaryFidelity`). The new Gemini
  case follows the same skip pattern (D-05).
- Recall@k assertion via `recallAtK` (membership in top-k). "Record T within
  default k" is the reused hard bar (D-09).
- Every model/dim change is a **reindex boundary** (Phase-13
  embedder-config-identity) — recipes MUST surface this (D-14).
- Docs-site is Astro Starlight Markdown under
  `docs-site/src/content/docs/guides/`; new guide = new `.md` with frontmatter.
- Every Go/Markdown file carries the Apache-2.0 SPDX header (`task license:check`);
  `task` (lint + test) must be clean; `.planning/` rumdl noise is pre-existing.

### Integration Points
- `retrievalCases` (fixtures.go) ← new Gemini differ-case.
- `Taskfile.yaml eval:retrieval` ← documented run command in the new guide (D-06).
- `charts/engram/values.yaml memory.embed/openai` ← commented recipes (D-13),
  paralleling the docs table (D-12).
- `guides/embedding-instructions.md` ↔ new `guides/embedding-models.md`
  cross-links (D-11).

</code_context>

<specifics>
## Specific Ideas

- The Gemini differ-assertion proves the *provider honors* `task_type`, not just
  that engram sends it — a request-body unit test alone would pass even against a
  provider that silently ignores the param (the exact PITFALLS #12 failure).
- Providers to cover in the recipe table (D-12): **OpenRouter**
  (`https://openrouter.ai/api/v1`), **Gemini**
  (`https://generativelanguage.googleapis.com/v1beta/openai`), **OpenAI**
  (`https://api.openai.com/v1`), **local TEI / Ollama / vLLM**. Pair each with
  base URL + model + dim + query instruction + reindex note.
- qwen3-embedding-8b@4096 uses `ENGRAM_EMBED_QUERY_INSTRUCTION` (asymmetric) — the
  #261 parity config; document the OpenRouter env to reproduce (D-08).

</specifics>

<deferred>
## Deferred Ideas

- **Opt-in CI eval job** (workflow_dispatch wired to Gemini + qwen3 secrets) —
  considered and NOT taken this phase (D-06 chose local/manual). Revisit if the
  evals need to gate `main` automatically.
- **Runtime enforcement of the reindex boundary** (reject/quarantine reads of
  records whose embedder-identity hash mismatches the live config) — Phase 13
  only stamps the identity; enforcement is a separate later decision (already in
  REQUIREMENTS.md "Future Requirements").
- **`google.golang.org/genai` native SDK** — explicitly out of scope (ROADMAP
  Out-of-Scope): Gemini rides the existing OpenAI-compat `embed.Client`; no second
  SDK.
- **Per-provider embedder config profiles** — out of scope (DEC-zyhq keeps a
  generic param-map passthrough, not per-vendor profiles).
- **Standardizing on a blessed production model/provider** — deliberately NOT
  done; recipes are equal operator choices (domain framing).

</deferred>

---

*Phase: 14-embedder-model-options-eval*
*Context gathered: 2026-07-11*
