# Phase 1: Eval Foundation & Lexical Reranker Fix - Context

**Gathered:** 2026-09-22
**Status:** Ready for planning

<domain>
## Phase Boundary

Make `task eval:retrieval` trustworthy and use it to settle the lexical reranker's fate (#605):

- Fix the asymmetry differ gate to compare with a cosine epsilon, not `reflect.DeepEqual` (#353, EVAL-01).
- Make the eval's skip guards read the resolved koanf config, not raw `os.Getenv` (#354, EVAL-02).
- Add an independently written paraphrase case beside the #261 fixture; report recall@k and MRR per
  ranking variant (RANK-01).
- Ship the ranking the pre-committed decision rule picks, so the paraphrase case does not regress
  against vector-only order and #261's target stays at rank 1 (RANK-02).

Not in this phase: any Jev client, decision interface, or config (Phase 2); the Jev reranker and
per-hit relevance probability (Phase 4).

</domain>

<decisions>
## Implementation Decisions

### Paraphrase case: authorship and corpus
- **D-01:** Queries are written by a **blind subagent** that sees only short topic descriptions of
  each target (e.g. "the record about pre-commit linting"), never the record text. A separate pass
  maps each query to its target key. The fixture comment records this procedure so the case is
  reproducible and its independence is auditable.
- **D-02:** The corpus is a **new multi-domain synthetic set of 80–120 spine-shaped records**
  (several domains, e.g. tooling, auth, deploy, data model, config, testing), with deliberate
  sticky topical neighbours for each target. Content is synthetic public-style text — no verbatim
  spine content, no secrets (public repo). A corpus this size exceeds `candidateK`'s floor of 32,
  so the reranker sees a real candidate subset, unlike spike 004 where every candidate set was the
  whole corpus.
- **D-03:** About **20 single-answer queries** (one intended record each, fits the existing
  `wantKey` shape) **plus a few no-answer queries** (no correct record in the corpus).
- **D-04:** The #261 case (`gh261Case`) stays unchanged as the permanent near-verbatim regression
  guard.

### Lexical reranker: keep / demote / replace
- **D-05:** **Pre-committed decision rule** (decide before seeing numbers): among variants that
  (a) keep #261's target at rank 1 for both #261 queries and (b) have paraphrase MRR ≥ vector-only
  MRR, ship the one with the best paraphrase MRR. A tuned variant (blend or gate) wins over
  vector-only only if it beats vector-only paraphrase MRR by a clear margin (≥ 0.05); otherwise
  the simpler variant wins. Ties go to the simplest.
- **D-06:** Variants measured: **vector-only (no lexical)**, **cosine blend**
  (score = cosine + α·normalized overlap, rather than reordering purely on overlap),
  **overlap-threshold gate** (promote only on very high, near-verbatim overlap; otherwise keep
  vector order), and **shipped lexical** (baseline).
- **D-07:** α and the gate threshold are tuned over a **small fixed grid (3–4 values each)**, with
  the ≥ 0.05 MRR margin in D-05 as the guard against overfitting to the same eval set.
- **D-08:** If vector-only wins: **keep `store.SearchReranked` as the single shared seam** (MCP
  `deps.searchMemory` and Connect `SearchMemories` both call it) with a no-op rank step, and
  delete the lexical code. Phase 4's Jev reranker plugs into this seam. — **Reversibility:**
  reversible — lexical code is recoverable from git; the seam is unchanged.
- **D-09:** Whichever variant wins, the chosen ranking and the measured table are recorded (in
  the eval log output and the phase summary) so #605 can be closed with evidence.

### Eval gates and Jev column
- **D-10:** Hard `t.Errorf` bars: #261 target **at rank 1** (tightened from "within default k")
  for both #261 queries, and **shipped paraphrase MRR ≥ vector-only paraphrase MRR**. Per-variant
  recall@k and MRR are logged as a table (`t.Logf`), not gated. No absolute thresholds (brittle
  across embedders).
- **D-11:** The eval iterates a **pluggable, named list of rankers** (vector-only, lexical
  variants, …) so Phase 4 appends Jev without refactoring. Phase 1 also adds a **Jev stub entry**
  that reports "Jev: disabled" in the table — no Jev client, config, or network call in Phase 1.
- **D-12:** No-answer queries are **excluded from recall@k/MRR** and only logged (top hit and its
  score) — kept in the fixture for Phase 4's per-hit relevance signal (RANK-04). No gate.

### Differ gate and skip guards
- **D-13:** Differ gate (#353): pass only when **cosine distance > 1e-3** between the query- and
  document-side vectors; a NaN or Inf component in either vector is a hard `t.Fatal`. Soften the
  PASS log to "vectors differ materially" (no claim about the cause). Keep the existing dimension
  contract check.
- **D-14:** Symmetric-config skip (#354): decide from the **resolved koanf config** — the same
  config the embedder is built from — checking its four embed instruction/params fields. This
  likely needs a loader that returns the resolved config alongside the embedder (or reuses
  `loadAndValidate`).
- **D-15:** (Revised after research, user-confirmed 2026-09-22.) **Do NOT register
  `ENGRAM_RETRIEVAL_EVAL` in `internal/config`'s field registry.** The registry deliberately
  excludes test-only vars (`ENGRAM_QDRANT_TEST_ADDR`, `ENGRAM_REQUIRE_QDRANT`), and
  `TestCheckLegacyIgnoresTestOnlyVar` pins that convention. Instead, resolve the gate with a
  **test-local koanf load** using the same `ENGRAM_` prefix and precedence rules as production,
  so any source koanf reads enables it (success criterion 2). Do not call `Validate()`, so a
  malformed, unrelated ambient `ENGRAM_*` var cannot fail the package when the eval is off.
  `TestMain` and each test's gate read the resolved value. `TestMain` still short-circuits before
  any Docker/testcontainer startup when the gate is off.

### Claude's Discretion
- Exact domains and record count within 80–120; exact α / threshold grid values; the number of
  no-answer queries ("a few").
- How the blind-subagent procedure is run during execution (prompt shape, how topic descriptions
  are produced), as long as D-01's independence holds and is documented.
- Whether per-variant ranking lives in the eval package or as small exported helpers in
  `internal/store`, provided the shipped path is measured through `SearchReranked` itself.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope
- `.planning/ROADMAP.md` §"Phase 1: Eval Foundation & Lexical Reranker Fix" — goal and success criteria
- `.planning/REQUIREMENTS.md` — EVAL-01, EVAL-02, RANK-01, RANK-02

### Spike evidence and blueprint
- `.claude/skills/spike-findings-engram/SKILL.md` — findings index
- `.claude/skills/spike-findings-engram/references/recall-rerank.md` — lexical vs vector vs Jev numbers, the "write a paraphrase case blind to targets" instruction, what to avoid
- `.planning/spikes/004-jev-rerank-eval/README.md` — spike method and caveats (target-aware queries, 16 records)

### Issues
- GitHub #605 — lexical reranker degrades paraphrased queries (proposed work + acceptance)
- GitHub #353 — differ gate `reflect.DeepEqual` false-PASS; cosine-epsilon fix
- GitHub #354 — skip guard reads `os.Getenv`, not resolved koanf config
- GitHub #261 — original near-verbatim crowding regression (must stay at rank 1)

### Code
- `internal/retrievaleval/fixtures.go` — `gh261Case`, `recallAtK`, `reciprocalRank`, `differProbe`
- `internal/retrievaleval/retrieval_eval_test.go` — `TestRetrievalEval`, `TestRetrievalEval_AsymmetryDiffer`, `TestMain` gate
- `internal/retrievaleval/doc.go` — package contract (gated, zero CI Docker cost when off)
- `internal/store/rerank.go` — `candidateK`, `tokenize`, `lexicalOverlap`, `RerankHits`
- `internal/store/store.go` `SearchReranked` — the shared ranking seam
- `internal/server/tools.go` `loadAndValidate`, `StoreAndEmbedderFromEnvNoEnsure`, search handler
- `internal/config/registry.go` — field registry for `ENGRAM_` vars
- `Taskfile.yaml` `eval:retrieval` — the eval entrypoint

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `recallAtK` / `reciprocalRank` (`fixtures.go`): metric helpers, reuse as-is.
- `seedRecord` / `retrievalCase` / `retrievalQuery`: fixture shapes. The no-answer queries need a
  way to express "no wantKey" (e.g. an empty key or a separate list).
- `newTestcontainerStore` + a fresh collection per case: the seeding pattern for the new corpus.
- `server.StoreAndEmbedderFromEnvNoEnsure`: the prod-parity embedder builder.

### Established Patterns
- Seeding must use the exact production doc-embed sequence (`store.EmbedText` → `em.Embed` →
  `st.Upsert`); querying must use `em.EmbedQuery` → `SearchReranked` for the shipped path.
- The eval is gated and skipped in CI; `TestMain` short-circuits before Docker. The
  `TestRetrievalEval` name prefix is load-bearing (`-run TestRetrievalEval`).
- Fixtures carry synthetic public-style content only.
- Config: koanf, env-first with the `ENGRAM_` prefix; the registry is the single source of truth.

### Integration Points
- `store.SearchReranked` is called by the MCP handler (`internal/server/tools.go`) and declared on
  the server's store interface (`store_iface.go`); Connect `SearchMemories` reaches the same seam.
  Any ranking change lands there and ships to both surfaces.
- Adding `ENGRAM_RETRIEVAL_EVAL` to the registry may trip config-doc gates (docs-site config
  reference); keep them green.

</code_context>

<specifics>
## Specific Ideas

- Spike 004's worst cases (rank 1 → 10 for "Can badly indented YAML break the build?") show the
  failure shape to reproduce: shared tool words pulling topical neighbours above the answer.
- The Jev column appears in the table as "disabled" in Phase 1, so Phase 4's diff is "enable the
  stub", not "add a column".

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. (No-answer queries are captured now but used for
the relevance signal only in Phase 4.)

</deferred>

---

*Phase: 01-eval-foundation-lexical-reranker-fix*
*Context gathered: 2026-09-22*
