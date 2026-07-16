---
phase: 14-embedder-model-options-eval
reviewed: 2026-07-11T17:59:30Z
depth: standard
files_reviewed: 5
files_reviewed_list:
  - charts/engram/values.yaml
  - docs-site/src/content/docs/guides/embedding-instructions.md
  - docs-site/src/content/docs/guides/embedding-models.md
  - internal/retrievaleval/fixtures.go
  - internal/retrievaleval/retrieval_eval_test.go
findings:
  critical: 0
  warning: 3
  info: 2
  total: 5
status: issues_found
---

# Phase 14: Code Review Report

**Reviewed:** 2026-07-11T17:59:30Z
**Depth:** standard
**Files Reviewed:** 5
**Status:** issues_found

## Summary

Phase 14 adds a skip-gated `TestRetrievalEval_AsymmetryDiffer` correctness gate
plus a `differProbe` synthetic fixture to `internal/retrievaleval`, and
operator-facing embedding-model recipes across two Starlight docs pages and
commented blocks in the Helm `values.yaml`.

I verified the Go additions against the real production surfaces they claim to
mirror: `server.StoreAndEmbedderFromEnvNoEnsure()` (5-return signature matches),
`embed.Client.Embed` / `EmbedQuery` (instruction/params behavior matches the
docs' three-mode description exactly), `store.SearchReranked` / `store.Search`
(signatures match), and the `ENGRAM_EMBED_*` env-var names against
`internal/config/registry.go` (all four match). No secrets, injection, or
data-loss risks — fixtures are synthetic public-tooling text and the test only
dials an ephemeral testcontainer.

No BLOCKERs. The findings are: a soundness gap in the differ gate's exact-float
equality check (it can give a false PASS and its PASS does not actually prove
what its message claims), a config-source mismatch in the symmetric-config skip
guard, an operator-facing model-id that should be verified against the
provider's current catalog, and two stale/broken references. The
adversarial-review focus (the correctness gate) is the differ test; the docs are
internally consistent with the verified `embed.go` behavior.

## Warnings

### WR-01: Differ gate's exact-equality check can give a false PASS and does not prove "instruction-prefix took effect"

**File:** `internal/retrievaleval/retrieval_eval_test.go:263-277`
**Issue:** The Pitfall-12 gate treats `reflect.DeepEqual(queryVec, documentVec)`
as a proxy for "the asymmetric instruction-prefix took effect." Two problems:

1. **False PASS under embedding-API nondeterminism.** The misconfiguration this
   gate exists to catch (no-op `*_PARAMS`/`task_type` on Gemini) sends the *same
   raw string* to both `EmbedQuery` and `Embed`. The gate only fails if the two
   returned vectors are bit-identical. Many hosted gateways are **not**
   bit-deterministic across calls (batching, load-balanced heterogeneous
   backends, non-associative float reduction on GPU). If any component differs
   by one ULP, `reflect.DeepEqual` returns `false` and the test **PASSES**,
   silently masking exactly the misconfiguration it was built to catch.
2. **The PASS message overclaims.** `query != document` only proves the two API
   calls differed *for some reason* — not that the instruction-prefix mechanism
   is what caused it. The `t.Logf("...instruction-prefix took effect")` asserts
   causation the check cannot establish.

(Minor, same root: `reflect.DeepEqual` also reports two identical vectors
containing `NaN` as unequal, another false-PASS path.)

**Fix:** Assert a *meaningful* separation instead of exact inequality — e.g.
require cosine distance above a small epsilon so pure numerical jitter cannot
satisfy the gate, and soften the PASS log to "vectors differ materially" rather
than asserting the mechanism:
```go
d := cosineDistance(queryVec, documentVec) // 1 - cos_sim
const minAsymmetryDistance = 1e-3
if d < minAsymmetryDistance {
    t.Fatalf("asymmetry differ FAIL: query≈document (cos-dist=%g < %g, dim=%d) — asymmetric instruction-prefix had no material effect; operator likely wired the no-op *_PARAMS/task_type mechanism", d, minAsymmetryDistance, dim)
}
```

### WR-02: Symmetric-config skip guard reads raw env vars but the embedder is built from full koanf config

**File:** `internal/retrievaleval/retrieval_eval_test.go:233-238`
**Issue:** The skip decision inspects the four `ENGRAM_EMBED_*` values via
`os.Getenv`, while the embedder under test is built by
`server.StoreAndEmbedderFromEnvNoEnsure()` → `loadAndValidate()`, which resolves
config through koanf (env-first, but with `--flag`/file overrides — per
CLAUDE.md's config contract). The two paths can disagree:

- Instruction set via config file (not env) → guard sees all-empty → **skips a
  genuinely asymmetric config**, losing coverage.
- Env instruction set but cleared by a higher-precedence override → embedder is
  symmetric (`query == document`) yet the guard runs → **spurious `t.Fatal`** on
  a legitimately symmetric setup.

Likelihood is low in the env-driven CI/eval path, but the guard should gate on
the *same* config the embedder is built from, not a parallel env read.

**Fix:** Derive the skip from the resolved config (or from the built `*Client`'s
effective instruction/params) rather than re-reading `os.Getenv`, so the skip
predicate and the object under test agree by construction. If the builder does
not expose the effective config, load it once and branch on that struct.

### WR-03: `gemini-embedding-2` model id in the copy-paste recipes should be verified against the provider catalog

**File:** `docs-site/src/content/docs/guides/embedding-models.md:24,57,64-67`; also `charts/engram/values.yaml:57`, `docs-site/src/content/docs/guides/embedding-instructions.md:115`
**Issue:** The recipes are copy-paste `export ENGRAM_EMBED_MODEL='gemini-embedding-2'`
blocks — their entire value is that they work verbatim. Google's GA embedding
model on the Generative Language API is `gemini-embedding-001` (dim 3072); I
could not confirm `gemini-embedding-2` is a served id on the
`/v1beta/openai/embeddings` endpoint. A wrong model id fails the recipe with a
model-not-found error at the gateway, and the same id is wired as the
"differ-case reference config" (`embedding-models.md:72`), so a bad id would
also break `TestRetrievalEval_AsymmetryDiffer` live runs.

**Fix:** Verify the id against Google's current embeddings catalog for the
OpenAI-compat endpoint and pin the exact served name (e.g. `gemini-embedding-001`)
consistently across `embedding-models.md`, `embedding-instructions.md`, and
`values.yaml`. If `gemini-embedding-2` is intentionally a project-internal
alias/gateway mapping, say so inline so operators do not paste a non-existent id.

## Info

### IN-01: Stale line-number citations in comments

**File:** `internal/retrievaleval/retrieval_eval_test.go:23-24` and `:94-95`
**Issue:** `defaultK`'s comment cites "deps.searchMemory's production default
(tools.go:706)", but that line is now inside `storeDiscovery`; the actual
`searchMemory` default `a.K = 8` lives at `tools.go:856`. Likewise the seed-loop
comment cites "tools.go:508-515" for the doc-embed sequence, where lines 508-515
are now the `updateArgs` struct. The *values* (k=8, the EmbedText→Embed→Upsert
sequence) are still correct — only the line anchors drifted.
**Fix:** Cite the symbol names (`deps.searchMemory`'s `a.K = 8` default;
`store.EmbedText`) without line numbers, since anchors rot on every edit.

### IN-02: Broken cross-reference "see its row above" for the OpenRouter params row

**File:** `docs-site/src/content/docs/guides/embedding-instructions.md:106`
**Issue:** The cloud-models params table's OpenRouter row says "forwards
whichever field name/value the backend model expects — see its row above," but
there is no OpenRouter row above it on this page (the OpenRouter row with the
query instruction lives in `embedding-models.md`). The reader has nothing to
follow.
**Fix:** Point explicitly at the model-recipes page, e.g. "see the OpenRouter
row in [Embedding model recipes](/guides/embedding-models/)."

---

_Reviewed: 2026-07-11T17:59:30Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
