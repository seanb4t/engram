---
gsd_state_version: 1.0
milestone: v0.10.x
milestone_name: — Hardening & Write Lane
current_phase: 17
current_phase_name: wired-write-handlers-full-crud-schedule
status: executing
stopped_at: Completed 17-04-PLAN.md
last_updated: "2026-07-12T23:58:21.734Z"
last_activity: 2026-07-12
last_activity_desc: Phase 17 execution started
progress:
  total_phases: 5
  completed_phases: 4
  total_plans: 19
  completed_plans: 18
  percent: 80
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-07-11 after Phase 14)

**Core value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its context, and wrong/stale memories can be corrected or superseded.
**Current focus:** Phase 17 — wired-write-handlers-full-crud-schedule

## Current Position

Phase: 17 (wired-write-handlers-full-crud-schedule) — EXECUTING
Plan: 6 of 6
Status: Ready to execute
Last activity: 2026-07-12 — Phase 17 execution started

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
- [Phase 15]: buf.gen.yaml gained a managed-mode disable rule scoped to buf.build/bufbuild/protovalidate so go_package_prefix override doesn't break the BSR dependency's generated import path (Plan 01, Rule 3)
- [Phase 15]: Duplicated the idempotency-ban grep regex verbatim between Taskfile.yaml and .github/workflows/ci.yaml (no shared script) — matches this repo's bare-runner CI convention of mirroring Taskfile commands inline; Plan 04's descriptor test is the defense-in-depth backstop against regex drift
- [Phase 15]: go mod tidy in Plan 03 (not Plan 01) promotes buf.build/go/protovalidate indirect->direct, since connectvalidate.go is the first code to import the runtime package (review finding #4)
- [Phase 15]: validate interceptor's CodeInternal branch (non-ValidationError) is covered via a fake protovalidate.Validator since a real validator over generated constraints only returns nil or *ValidationError (review finding #5)
- [Phase 15]: Descriptor test pins per-field wire-shape tables (number/name/kind/cardinality/message-type) on Memory/ScopeCount/read messages, not just message names, per cross-AI review finding #6 (SC4)
- [Phase 15]: Negative matrix uses a generic callWrite[Req, Resp] helper to keep the six write-RPC table uniform, and asserts GET-405 via generated engramv1connect Procedure constants rather than hardcoded paths (finding #6)
- [Phase 16]: NewCSRFSigner returns (*CSRFSigner, error) rather than a bare *CSRFSigner, mirroring NewSessionCodec's fail-fast convention (D-08)
- [Phase 16 P02]: TestWriteRPCNegativeMatrix's callWrite auto-attaches a matching CSRF cookie/header pair for authenticated calls (stub csrfVerify always true) so the pre-existing four-cell matrix stays green alongside the new CSRF interceptor
- [Phase 16 P02]: TestReadRPCsCSRFExempt uses testDeps(t) (real Qdrant) instead of a bare deps{} -- read handlers dereference deps.st/em directly, unlike the still-Unimplemented write stubs
- [Phase 16 P02]: connectcsrf_test.go's token-matrix verify is a local inline HMAC replica, not an internal/webauth import -- preserves the internal/server -> internal/webauth layering boundary
- [Phase 16]: Minted the engram_csrf cookie in webauth.Handler.Callback this phase (not deferred to Phase 19) — SC2 is live end-to-end via a real fake-OIDC Callback test, per RESEARCH.md's resolved Open Question 1
- [Phase 17]: [Phase 17 P01] Non-email owner encoding uses fmt.Sprintf("%d:%s:%d:%s", len(claim), claim, len(value), value) instead of the ambiguous claim:value form, closing the (sub,x:y)/(sub:x,y) collision (D-06 hardened)
- [Phase 17]: [Phase 17 P01] Session cookie payload versioned (SessionCodec.Seal auto-injects); Resolve rejects any legacy/mismatched-version cookie with the same generic invalid-session error, forcing automatic re-login on the owner-encoding rollout without a manual --ui-cookie-key rotation
- [Phase 17]: [Phase 17 P01] ENGRAM_OWNER_CLAIM parsing (config.ParseOwnerClaims) kept strictly separate from defaulting; registry still supplies default email when unset; malformed comma-lists (duplicate/interior-empty/bad claim name) fail fast rather than silently normalizing
- [Phase 17]: [Phase 17 P02] UpdatePayload uses a targeted two-op SetPayload+DeletePayload (not a whole-payload OverwritePayload) to avoid content/vector desync from a stale FetchForUpdate snapshot with no CAS
- [Phase 17]: [Phase 17 P02] callerFromTokenInfo is the single choke point for both auth lanes; Actor falls back to the resolved owner when TokenInfo.UserID is empty (Connect cookie lane never sets it)
- [Phase 17]: [Phase 17 P02] errRuleImmutable is a NEW sentinel; errStaleSummary is REUSED unchanged (not redeclared)
- [Phase 17]: [Phase 17 P03] Visibility enum<->bool mapping is used ONLY by SetVisibility; UpdateMemory shared maps &req.Shared (proto bool) directly to *bool, never the enum mapper (round-8 MED)
- [Phase 17]: [Phase 17 P03] windowBoundFloor/windowBoundCeil round scheduling bounds OUTWARD to whole seconds before RFC3339Nano formatting so a sub-second not_after cannot persist as immediately-expired after the store's .Unix() flooring (round-8 MED)
- [Phase 17]: [Phase 17 P06] coreListRequest/coreListResult/coreSearchRequest defined as a superset of both lanes (offset, categories, visibility, exact total, cursor/cursor_mode, tags, created window); no internal Limit/K default lives in deps.listMemory/searchMemory — each lane applies its own (MCP 20/8 in the tool closures, Connect leaves limit=0 as 'all' and applies k=20 in 17-04)
- [Phase 17]: [Phase 17 P06] deps.searchDiscovery intentionally retains its internal k=8 default (MCP lane only, with a retention comment) since the Connect SearchDiscoveries adapter (17-04) pre-applies k=20
- [Phase 17]: [Phase 17 P06] MCP recall shaping (shapeRecall -> []any) moved out of deps.listMemory/searchMemory into the MCP list_memory/search_memory tool closures; the list closure explicitly sets CursorMode: true to preserve today's unconditional MCP cursor-mode pagination
- [Phase 17]: [Phase 17 P06] TestListMemoryRejectsBadWindow relocated to assert parseRFC3339 directly (the exact call the MCP closures make) since the typed core's CreatedAfter/CreatedBefore are time.Time and cannot carry an invalid string
- [Phase 17]: [Phase 17 P04] connectError(ctx, err) is the single production error mapper matching typed sentinels (ErrNotFound/ErrInvalidArgument/errRuleImmutable/errStaleSummary/ErrAmbiguousShortID/context.Canceled/context.DeadlineExceeded); no CodeAborted arm since no distinct conflict sentinel exists
- [Phase 17]: [Phase 17 P04] spyStore is a scripted-spy memStore (records method+owner+args, non-nil embedder) rather than a full store-authz reimplementation; the real Qdrant isolation suite remains the authz gate
- [Phase 17]: [Phase 17 P04] SearchDiscoveries maps an empty Connect scope to CrossSpine=true so it still spans all discovery scopes after the read-lane rewire; GetMemory's handler-level usage-enqueue call is removed since deps.getMemory is now the sole enqueue point

### Blockers/Concerns

- **Env restore (non-blocking):** repo-local `commit.gpgsign=false` is still set (1Password SSH-signing was flaky during the v0.9.x milestone; those commits were unsigned and `main` had no required-signatures). Restore when 1Password is stable: `git config --local --unset commit.gpgsign`. Also sync local `main` past the squash merge (`658795e9`).
- **Research Pitfall 1 (highest risk this milestone):** a Connect write RPC that bypasses `deps.*` and calls `store.*` directly would silently reintroduce the handler-vs-store authz split DEC-cgb rejected, one layer up (business logic, e.g. rule immutability DEC-iedk). Phase 17's success criteria make MCP/Connect parity tests a hard gate, not optional.
- **Research Pitfall 7 (session rotation):** stateless rotation has no revocation mechanism; Phase 18 requires a new ADR explicitly documenting this trade-off before implementation, not a silent drift past DEC-u9v.
- Tracked tech debt now scoped into v0.10.x phases: #334→Phase 14, #335→Phase 21, #333/#332/#331→Phase 13/14, #337→Phase 14. Systemic `.rumdl.toml` `.planning` exclude → Phase 21.

## Session Continuity

Last session: 2026-07-12T23:58:21.727Z
Stopped at: Completed 17-04-PLAN.md
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
| Phase 15 P01 | 7min | 3 tasks | 8 files |
| Phase 15 P02 | 6min | 2 tasks | 2 files |
| Phase 15 P03 | 12min | 2 tasks | 4 files |
| Phase 15 P04 | 12min | 2 tasks | 2 files |
| Phase 16 P01 | 10min | 2 tasks | 2 files |
| Phase 16 P02 | 25min | 3 tasks | 9 files |
| Phase 16 P03 | 20min | 3 tasks | 5 files |
| Phase 17 P01 | 35min | 3 tasks | 13 files |
| Phase 17 P02 | 25min | 3 tasks | 14 files |
| Phase 17 P03 | 10min | 2 tasks | 2 files |
| Phase 17 P06 | 27min | 2 tasks | 4 files |
| Phase 17 P04 | 17min | 3 tasks | 7 files |
