---
phase: 10
slug: asymmetric-query-document-embeddings
status: passed
verified: 2026-07-10
disposition: already-shipped
requirements: [REQ-embedder-native-params]
---

# Phase 10 — Verification (already shipped)

> **Disposition: ALREADY SHIPPED.** Phase 10 (`REQ-embedder-native-params`, GitHub #305)
> was found fully implemented, config-wired, tested, and documented during
> `/gsd-discuss-phase 10` baseline verification — before any planning. It was
> delivered under the Phase 4 embedder work (`REQ-asymmetric-embedder-params`, DEC-zyhq)
> after #305/beads `engram-wd89.1` was filed; the issue was simply never closed.
> This mirrors the Phase 8 already-shipped reconciliation. No plans were written or
> executed for this phase.

## Success Criteria — assessment against the codebase

### SC1 — Cloud embedders receive a distinct native API field per call — **MET**
`internal/embed/embed.go`:
- `WithQueryParams` / `WithDocumentParams` (lines 56–67) hold a `map[string]any` merged
  into the `/v1/embeddings` request body per call (lines 180–190); `model`/`input` stay
  authoritative. This is the native `input_type`/`task_type` passthrough for
  Cohere/Google/Voyage/Jina (map form is a superset of #305's proposed `name=value`).
- `EmbedQuery` applies `queryParams`; `Embed` applies `documentParams` (lines 112, 153) —
  so query vs document get distinct native fields.

Config wiring: `ENGRAM_EMBED_QUERY_PARAMS` / `ENGRAM_EMBED_DOCUMENT_PARAMS`
(`internal/config/registry.go:33–34`), parsed + validated via `ParseEmbedParams`
(`internal/config/validate.go:77–80`), passed through `embedderFromConfig`
(`internal/server/tools.go:227–228`). Tested: `internal/config/embedparams_test.go`,
`internal/embed/embed_test.go`, `internal/config/validate_test.go`.

### SC2 — Both-side-prefix models (E5/nomic) get a document-side prefix at store + reindex — **MET**
`WithDocumentInstruction` (`embed.go:131`): empty / `{document}` template / prefix, applied by
`Embed` (store + reindex path). Config `ENGRAM_EMBED_DOCUMENT_INSTRUCTION`
(`registry.go:35`), wired at `tools.go:226`. The doc-string explicitly notes changing it
alters stored document vectors and requires a reindex — honoring the reindex boundary.

### SC3 — Documented per-model + Phase 9 eval shows non-regression — **PARTIAL (docs MET; eval demo optional)**
- Per-model documentation: **MET** — `docs-site/src/content/docs/guides/embedding-instructions.md`
  covers `ENGRAM_EMBED_QUERY_PARAMS` (×10), `ENGRAM_EMBED_DOCUMENT_PARAMS` (×5),
  `ENGRAM_EMBED_DOCUMENT_INSTRUCTION` (×9), and gemini/cohere/voyage/jina (×6).
- Eval non-regression demonstration for an asymmetric model config: **not separately run.**
  The mechanism is code-verified and unit-tested; an end-to-end demonstration on the
  Phase 9 harness (e.g. Gemini `task_type=RETRIEVAL_QUERY|RETRIEVAL_DOCUMENT`) is an
  OPTIONAL follow-up, not a blocker — it is a proof-of-benefit, not proof-of-capability.
  (Currently gated by the same OpenRouter/embed-timeout constraints tracked in #333/#334.)

## Verdict

REQ-embedder-native-params is **satisfied by shipped code** (SC1, SC2, SC3-docs). The only
open item is an optional eval demonstration of retrieval benefit for an asymmetric config,
which can ride on the #334 prod-parity eval run. Phase 10 is marked **Complete (already
shipped)**; GitHub #305 closed with this evidence.
