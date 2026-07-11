---
gsd_state_version: 1.0
milestone: v0.10.x
milestone_name: — Hardening & Write Lane
current_phase: 15
current_phase_name: Additive Proto + Stub Write Handlers
status: ready-to-plan
stopped_at: Phase 14 shipped — PR #352 (secured + reviewed at ship time); ready to plan Phase 15
last_updated: "2026-07-11T18:05:29.000Z"
last_activity: 2026-07-11
last_activity_desc: Phase 14 shipped via PR #352; Phase 15 ready to plan
progress:
  total_phases: 2
  completed_phases: 2
  total_plans: 6
  completed_plans: 6
  percent: 100
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-07-11 after Phase 14)

**Core value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its context, and wrong/stale memories can be corrected or superseded.
**Current focus:** Phase 15 — Additive Proto + Stub Write Handlers

## Current Position

Phase: 15 — Additive Proto + Stub Write Handlers
Plan: Not started
Status: Ready to plan Phase 15 (Phase 14 shipped via PR #352 — secured + code-reviewed at ship time; awaiting merge)
Last activity: 2026-07-11 — Phase 14 shipped (PR #352); follow-ups #353/#354/#355 filed

## Deferred Items

Items acknowledged and deferred at milestone close on 2026-07-10:

| Category | Item | Status |
|----------|------|--------|
| pending_todo | document-embedding-model-options (docs-site + Helm embedding-model guide) | Picked up in v0.10.x Phase 14 (REQ-embed-model-docs, #337) |

## Accumulated Context

### Decisions

Full decision record (56 ADR-locked baseline decisions + v0.9.x milestone decisions) in
PROJECT.md. v0.9.x headline decisions: D-04 always-on `search_memory` score; D-06 stdlib
lexical reranker (`store.SearchReranked`); D-01/D-08 async summaries off the write path
drained after shutdown under the CR-01 kernel; D-08 usage signals never affect ranking;
D-09 `ENGRAM_USAGE_SIGNALS` defaults on (non-egressing). Reusable Go conventions: CR-01
shutdown-safety (RWMutex+closed guard); `*time.Time` for optional timestamps (never
`time.Time`+`omitempty`).

**v0.10.x milestone decisions (resolved at scoping, 2026-07-10 — full text in REQUIREMENTS.md):**

- DECISION 1 — Write-lane CRUD scope: full CRUD + Schedule (all six write RPCs ship this milestone).
- DECISION 2 — Session rotation: stateless sliding-expiry re-seal, no server-side state (honors DEC-u9v); a new ADR is required in Phase 18 documenting the no-revocation trade-off.
- DECISION 3 — Reindex boundary: document AND payload-stamp embedder-config identity (Phase 13).

**Roadmap build-order rationale (research-derived, locked at roadmap creation):** embedder track
(13–14) is fully isolated and ships independently. Write-lane track (15–19) is strict-order:
proto+stubs (15) → CSRF interceptor (16) → deps.* refactor + wired handlers (17) → session
rotation (18) → console UX (19). Correctness/polish (20) and CI hygiene (21) are independent of
both tracks. Phases 16–18 are flagged for `/gsd-secure-phase` (18 mandatory — changes the
cookie-auth security posture).

- [Phase 13]: Task 1+2 committed together (shared embed.New Option seam + koanf config trio); Task 3 (D-09 regression) committed separately.
- [Phase 13]: Query/fragment base-URL join left non-canonicalizing (operator-error scope, T-13-01 trust boundary parity).
- [Phase 13]: Memory.EmbedderIdentity tagged json:"-" (not a normal json tag) — store.Memory serializes verbatim on 3 full-response MCP wire paths, so a normal tag would leak the audit field (round-1 review HIGH blocker).
- [Phase 13]: document_params empty-form canonicalization (len(params)==0 -> map[string]any{}) so "" and "{}" hash identically — prevents false null vs {} provenance drift (round-2 review MEDIUM).
- [Phase 13]: Reindex identity-aware resume — reindexTargetContents returns map[string]reindexTarget{content, identity}; a content match with an absent/stale embedder_identity falls through to re-embed+restamp instead of being skipped as Unchanged.
- [Phase 13]: StoreAndEmbedderFromEnvNoEnsure returns the computed embedder identity as a 5th value from its single config load; all 3 callers updated to the new arity.
- [Phase 14]: Named the differ test TestRetrievalEval_AsymmetryDiffer so task eval:retrieval's -run TestRetrievalEval regex substring-matches it without any Taskfile change
- [Phase 14]: Gemini asymmetry rides ENGRAM_EMBED_QUERY_INSTRUCTION/ENGRAM_EMBED_DOCUMENT_INSTRUCTION (text-prefix), never the *_PARAMS/task_type mechanism
- [Phase 14]: Local TEI/Ollama/vLLM recipes in guides/embedding-models.md documented as concrete complete rows (exact model id/dim/base URL/empty instruction) rather than operator-chosen placeholders (review B7)
- [Phase 14]: Gemini compat model-id confirmed unchanged (gemini-embedding-2, 3072-dim); embedding-models.md and values.yaml left untouched

### Blockers/Concerns

- **Env restore (non-blocking):** repo-local `commit.gpgsign=false` is still set (1Password SSH-signing was flaky during the v0.9.x milestone; those commits were unsigned and `main` had no required-signatures). Restore when 1Password is stable: `git config --local --unset commit.gpgsign`. Also sync local `main` past the squash merge (`658795e9`).
- **Research Pitfall 1 (highest risk this milestone):** a Connect write RPC that bypasses `deps.*` and calls `store.*` directly would silently reintroduce the handler-vs-store authz split DEC-cgb rejected, one layer up (business logic, e.g. rule immutability DEC-iedk). Phase 17's success criteria make MCP/Connect parity tests a hard gate, not optional.
- **Research Pitfall 7 (session rotation):** stateless rotation has no revocation mechanism; Phase 18 requires a new ADR explicitly documenting this trade-off before implementation, not a silent drift past DEC-u9v.
- Tracked tech debt now scoped into v0.10.x phases: #334→Phase 14, #335→Phase 21, #333/#332/#331→Phase 13/14, #337→Phase 14. Systemic `.rumdl.toml` `.planning` exclude → Phase 21.

## Session Continuity

Last session: 2026-07-11
Stopped at: Phase 14 shipped via PR #352 (secured + code-reviewed at ship time; awaiting merge); ready to plan Phase 15
Resume file: None

## Performance Metrics

| Phase | Plan | Duration | Notes |
|-------|------|----------|-------|
| Phase 13 P01 | 21min | 3 tasks | 9 files |
| Phase 13 P02 | 15min | 4 tasks | 9 files |
| Phase 13 P03 | 20min | 3 tasks | 6 files |
| Phase 14 P01 | 11min | 2 tasks | 2 files |
| Phase 14 P02 | 12min | 3 tasks | 3 files |
| Phase 14-embedder-model-options-eval P03 | 8min | 2 tasks | 1 files |
