# Phase 14: Embedder Model Options & Eval - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-11
**Phase:** 14-embedder-model-options-eval
**Areas discussed:** Gemini recipe + task_type gate, Eval execution & evidence, Model-recipe docs home + shape, #261 parity config + bar

---

## Gemini recipe + task_type gate

### Which Gemini embedding model as the canonical recipe?

| Option | Description | Selected |
|--------|-------------|----------|
| gemini-embedding-001 | Current GA, MRL/output_dimensionality, RETRIEVAL_QUERY/DOCUMENT task types | |
| text-embedding-004 | Older 768-dim, being superseded | |
| Research picks current GA | Defer exact model to researcher | |

**User's choice:** "gemini embedder 2 is current ga"
**Notes:** User asserts a gen-2 Gemini embedding model is now GA. Captured as "current-GA Gemini embedding model"; exact model id + OpenAI-compat wire shape MUST be researcher-verified against live docs (success criterion #1 mandates live-doc verification).

### Output dimension for the Gemini recipe?

| Option | Description | Selected |
|--------|-------------|----------|
| 1536, documented as a knob | MRL-truncated default + reindex callout | |
| 3072 native full | Native dimension, larger vectors | ✓ (native full) |
| You decide | Planner/research chooses | |

**User's choice:** "use gemini embedder 2 native full"
**Notes:** Ship at the model's native full output dimension, no MRL truncation. Concrete dim value pending D-01 doc verification.

### How to wire the "query ≠ document vectors" correctness gate?

| Option | Description | Selected |
|--------|-------------|----------|
| New live eval fixture case | Embed one text both ways, assert vectors differ; runs under task eval:retrieval | ✓ |
| Standalone embed unit test | Assert query vs document request bodies differ (no live call) | |
| Both (unit + live eval) | Belt-and-suspenders | |

**User's choice:** New live eval fixture case
**Notes:** Proves the provider *honors* task_type, not just that engram sends it.

---

## Eval execution & evidence

### How should the live-endpoint evals be executed?

| Option | Description | Selected |
|--------|-------------|----------|
| Local/manual, documented | Keep ENGRAM_RETRIEVAL_EVAL=1 skip gate; document env + command; no CI secrets | ✓ |
| Opt-in CI job w/ secrets | workflow_dispatch job wired to Gemini + qwen3 secrets | |
| Both | Local procedure + opt-in CI job | |

**User's choice:** Local/manual, documented

### What evidence closes criteria #1 and #2?

| Option | Description | Selected |
|--------|-------------|----------|
| Committed run artifact | Capture recall@8 numbers + differ-pass into a committed file | ✓ |
| One-time confirmation in notes | Record numbers in VERIFICATION, no artifact | |
| Green CI run link | Requires CI job | |

**User's choice:** Committed run artifact

### Source for the live qwen3-embedding-8b@4096 endpoint?

| Option | Description | Selected |
|--------|-------------|----------|
| Operator's own gateway | Run against reachable homelab/OpenRouter endpoint | |
| OpenRouter-hosted qwen3 | Cloud-hosted, reproducible with a key | ✓ |
| Researcher confirms availability | Leave to planning | |

**User's choice:** OpenRouter-hosted qwen3
**Notes:** Chosen as the reproducible *eval reference* config — not a mandated operator standard (see framing clarification below).

---

## Model-recipe docs home + shape

### Where should the recipes live?

| Option | Description | Selected |
|--------|-------------|----------|
| New guides/embedding-models.md | Dedicated recipes page; embedding-instructions.md cross-links | ✓ |
| Extend guides/configure.md | Fold into config landing page | |
| Extend embedding-instructions.md | Add recipes to existing instructions guide | |

**User's choice:** New guides/embedding-models.md

### Recipe format?

| Option | Description | Selected |
|--------|-------------|----------|
| Table + per-provider env blocks | Comparison table + copy-paste snippets | ✓ |
| Comparison table only | Terser, no snippets | |
| Per-provider prose only | Narrative, no table | |

**User's choice:** Table + per-provider env blocks

### How should values.yaml carry the recipes?

| Option | Description | Selected |
|--------|-------------|----------|
| Commented recipes inline | Default + commented OpenRouter/Gemini/OpenAI blocks | ✓ |
| Pointer to docs page | Minimal values, link only | |
| Both | Inline + link | |

**User's choice:** Commented recipes inline

---

## #261 parity config + bar

### Is qwen3-8b@4096 a shipped recipe or eval-only reference?

| Option | Description | Selected |
|--------|-------------|----------|
| Shipped recipe + eval reference | In docs table AND the #261 reference | ✓ (with framing caveat) |
| Eval-only reference | Only in the fixture, not documented as a recipe | |
| You decide | Planner decides | |

**User's choice:** "1 is fine - but I don't know that we've decided to _standardize_ on any particular model - it's an operator choice, as is the hosting provider. We make a choice for CI, etc"
**Notes:** KEY FRAMING CLARIFICATION — recipes are equal operator choices; engram does NOT standardize on a blessed production model/provider. The OpenRouter-hosted qwen3 (and the Gemini config) are concrete configs picked ONLY for the eval/CI reference, not recommendations operators must adopt.

### How is the #261 recall@8 pass bar set?

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse existing hard gate | gh261 already asserts "T within default k" | ✓ |
| Re-baseline the number | Re-measure and set a fresh threshold | |
| Hard gate + MRR baseline note | Keep hard gate, record MRR informationally | |

**User's choice:** Reuse existing hard gate

### Permanent or one-time Gemini differ-assertion?

| Option | Description | Selected |
|--------|-------------|----------|
| Permanent, skip-gated | Lives in retrievaleval alongside gh261, skip-gated | ✓ |
| One-time verification | Prove once, don't keep | |
| Permanent unit + one-time live | Permanent wiring unit test, one-time live check | |

**User's choice:** Permanent, skip-gated

---

## Claude's Discretion

- Committed-artifact file location/format.
- Gemini differ-case dataset (reuse gh261 vs minimal 2-record probe).
- Exact Gemini task_type param-map keys (pending live-doc verification).
- Neutral Helm default model choice (ollama/bge-m3 vs alternative).

## Deferred Ideas

- Opt-in CI eval job (considered, not taken — chose local/manual).
- Runtime enforcement of the reindex boundary (future; Phase 13 only stamps identity).
- `google.golang.org/genai` native SDK (out of scope — OpenAI-compat only).
- Per-provider embedder config profiles (out of scope — DEC-zyhq generic passthrough).
- Standardizing on a blessed production model/provider (deliberately not done).
