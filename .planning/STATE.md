---
gsd_state_version: 1.0
milestone: v0.11.x
milestone_name: Capture & Service Identity
current_phase: 26
current_phase_name: Structured Citations, Category Filter & Chat Base URL
status: executing
stopped_at: Completed 26-01-PLAN.md
last_updated: "2026-07-25T23:10:46.536Z"
last_activity: 2026-07-25
last_activity_desc: Phase 26 execution started
progress:
  total_phases: 8
  completed_phases: 4
  total_plans: 19
  completed_plans: 14
  percent: 50
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-07-18 — after Phase 24)

**Core value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its context, and wrong/stale memories can be corrected or superseded.
**Current focus:** Phase 26 — Structured Citations, Category Filter & Chat Base URL

## Current Position

Phase: 26 (Structured Citations, Category Filter & Chat Base URL) — EXECUTING
Plan: 2 of 6
Status: Ready to execute
Last activity: 2026-07-25 — Phase 26 execution started

Progress: [████████████████████] 11/11 plans ([███████░░░] 74%) · 3/8 phases (38%)

## Deferred Items

Items acknowledged and deferred at milestone close on 2026-07-10:

| Category | Item | Status |
|----------|------|--------|
| pending_todo | document-embedding-model-options (docs-site + Helm embedding-model guide) | Picked up in v0.10.x Phase 14 (REQ-embed-model-docs, #337) |

## Accumulated Context

### Decisions

Full decision record (56 ADR-locked baseline decisions + v0.9.x/v0.10.x milestone decisions) in
PROJECT.md. v0.9.x headline decisions: D-04 always-on `search_memory` score; D-06 stdlib
lexical reranker (`store.SearchReranked`); D-01/D-08 async summaries off the write path
drained after shutdown under the CR-01 kernel; D-08 usage signals never affect ranking; D-09
`ENGRAM_USAGE_SIGNALS` defaults on (non-egressing). Reusable Go conventions: CR-01
shutdown-safety (RWMutex+closed guard); `*time.Time` for optional timestamps (never
`time.Time`+`omitempty`).

**v0.10.x milestone decisions (resolved at scoping, 2026-07-10 — full text in
`milestones/v0.10.x-REQUIREMENTS.md`):**

- DECISION 1 — Write-lane CRUD scope: full CRUD + Schedule (all six write RPCs shipped).
- DECISION 2 — Session rotation: stateless sliding-expiry re-seal, no server-side state (honors DEC-u9v); ADR `engram-slr8` documents the no-revocation trade-off.
- DECISION 3 — Reindex boundary: document AND payload-stamp embedder-config identity (Phase 13).

**v0.11.x roadmap build-order rationale (research-derived, locked at roadmap creation
2026-07-16 — full detail in `.planning/research/{SUMMARY,ARCHITECTURE,CEDAR}.md`):** Phase 22
(Cedar authz foundation, `internal/authz` + cedar-go v1.8.0) must land first — it's a
behavior-preserving refinement of `DEC-cgb` (new ADR, working id DEC-cdr1: "PDP decides the
predicate; the store enforces it as the Qdrant filter" — bucket-level decisions only, no
per-record Cedar eval, no partial evaluation since cedar-go doesn't have it). Phase 23 (service
auth chain + tenancy isolation) builds on Phase 22 and must prove, as its FIRST test, that an
authenticated service principal never resolves to `owner==""` (the milestone's #1 risk). Phases
24→25→26 are the capture trio + recall/config tail: idempotency (24) lands before supersession
(25) because supersession reuses idempotency's payload-only re-Upsert mechanism to stamp the
superseded record; citations/category-filter/chat-base-url (26) are the least-coupled, bundled
last for pacing. Phase 24 is orthogonal to auth and can run in parallel with 22–23. Zero new
store-layer authz **primitive** and (except cedar-go) zero new dependencies this milestone.

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
- [Phase 17]: Non-email owner encoding uses fmt.Sprintf("%d:%s:%d:%s", len(claim), claim, len(value), value) instead of the ambiguous claim:value form, closing the (sub,x:y)/(sub:x,y) collision (D-06 hardened)
- [Phase 17]: ENGRAM_OWNER_CLAIM parsing (config.ParseOwnerClaims) kept strictly separate from defaulting; registry still supplies default email when unset; malformed comma-lists fail fast rather than silently normalizing
- [Phase 18]: engram-slr8 ADR (Accepted) authored hand-written; names ENGRAM_UI_COOKIE_KEY (registry.go:56) as the sole kill-switch, never the phantom ENGRAM_SESSION_KEY
- [Phase 19]: retryOnce's retry set is exactly {Unauthenticated, PermissionDenied} — client-side interpretation; a SINGLE OPPORTUNISTIC AUTH-RACE RETRY, never 're-seal on retry'/'rotation'
- [Phase 20]: Reversed config<->embed import direction (config owns ReservedEmbedParamKeys, embed aliases it) to avoid a real import cycle through internal/telemetry
- [Phase 21]: D-09: plain .planning rumdl exclude entry (not .planning/** glob), matching convention of .beads/.agents/docs-site neighbors
- [Phase 22 P01]: A1/A2 confirmed (kind hardcoded 'human'; MustDefault panics on parse failure, not New() error return)
- [Phase 22 P01]: TestPolicyCorpus_ForbidOverridesPermit proven via a synthetic inline permit-all/forbid-all PolicySet (white-box &PDP{policies:ps}) since the shipped 4-policy corpus has no naturally-overlapping permit+forbid request
- [Phase 22 P02]: decideBucketHook function-var test seam (mirrors mintCandidate/deletePayloadKeys) added to store.go since *authz.PDP is a sealed concrete type with no test constructor; zero new exported API
- [Phase 22 P03]: decideRecordHook function-var test seam (mirrors decideBucketHook) added to store.go since *authz.PDP is a sealed concrete type with no test constructor; zero new exported API
- [Phase 23 P01]: oidctest.Server+SignIDToken (already used in internal/webauth) reused as the fail-closed test fixture — fakeIDV's stub cannot carry claims (oidc.IDToken.claims is unexported), so real signed tokens exercise the actual TokenVerifier/ClaimIdentity path instead.
- [Phase 23 P01]: NewFromProvider same-issuer optimization skipped (planner discretion) — NewService does a plain second oidc.NewProvider discovery, zero new exported surface beyond NewService itself.
- [Phase 23 P02]: TokenInfo.UserID is the non-namespaced ownerID; Extra[OwnerClaimExtraKey] carries namespacedOwner("static_token", ownerID) — matches PATTERNS.md shape
- [Phase 23 P02]: empty configured static-token candidates are structurally excluded from matching (never eligible), independent of the empty-input-token guard
- [Phase 23 P03]: chainVerifier's D-02 order and D-03 nil-mechanism guards are intrinsic to a correct routing closure; Task 2's tests landed as test-only against Task 1's already-complete implementation rather than driving new production code
- [Phase ?]: service_auth.owner_claims defaults to client_id,azp (D-05), never email; static_tokens has no cobra flag (ENGRAM_-only secret map)
- [Phase ?]: SC4/SC5 (tenancy isolation + cross-tenant shared-read) proven with two permanent store-package tests, zero new production code
- [Phase ?]: Global cross-tenant shared-read (D-15) recorded as ADR engram-svct; per-tenant scoping deferred to a future full-ABAC milestone
- [Phase ?]: Exported internal/auth's chainVerifier/newStaticTokenVerifier as ChainVerifier/NewStaticTokenVerifier so cmd/engram's withAuth can build the composed verifier chain
- [Phase ?]: [Phase 24 P01]: engramIdempotencyNS fixed at 69fbe3e4-a53b-4d6e-971a-cad2f107e23c (uuidgen); idempotencyPointID returns string not uuid.UUID per plan artifact signature
- [Phase ?]: [Phase 24 P01]: Added TestIdempotencyPointIDKeySensitive to resolve golangci-lint unparam finding (key literal was constant "k" across all pre-wiring tests) — genuine coverage gain, no nolint directive
- [Phase ?]: [Phase 24 P02]: checkIdempotentReplay wired before Embed in both handlers; scheduleMemory calls it after parseWindow (cheap validation stays first) but still before Embed; SC5 test uses testDeps(t) matching the adjacent sibling test convention
- [Phase ?]: D-01 confirmed: target back-stamp uses SetPayload (single-key merge), not a full re-Upsert
- [Phase ?]: Recall-gate condition added independently at both Search and List call sites, not folded into activeWindowConditions
- [Phase ?]: Store.Get left deliberately ungated so superseded records stay fetchable by id
- [Phase ?]: supersede_memory does not call checkIdempotentReplay despite storeArgs.IdempotencyKey riding along via embedding (plan's explicit handler steps omit it; accepted per RESEARCH Pitfall 2)
- [Phase ?]: connectError maps store.ErrAlreadySuperseded to CodeFailedPrecondition, pre-positioning only (no Connect RPC exposes supersede_memory this phase)
- [Phase ?]: [Phase 26 P01]: D-09 confirmed: SearchOptions{Tags,Categories,CreatedAfter,CreatedBefore} replaces Search/SearchReranked's positional tail; k stays positional so SearchReranked's k==0 ErrInvalidArgument guard is unweakened
- [Phase ?]: [Phase 26 P01]: categoryMatchCondition extracted from listFilter's inline category loop, shared by list and search lanes; flagged tightening — empty-string category elements now skipped (mirrors tagMatchConditions), so categories:[""] becomes a passthrough

### Blockers/Concerns

- **Env restore (non-blocking):** repo-local `commit.gpgsign=false` is still set (1Password SSH-signing was flaky during the v0.9.x milestone; those commits were unsigned and `main` had no required-signatures). Restore when 1Password is stable: `git config --local --unset commit.gpgsign`. Also sync local `main` past the squash merge (`658795e9`).
- **v0.11.x #1 risk (Phase 23):** a service principal (OIDC client-credentials or static token) whose owner claim resolves empty must be REJECTED, never silently mapped to the anonymous bucket — this must be the first test proven in Phase 23, not discovered later. Cedar's Phase-22 defense-in-depth policy (`forbid ... unless principal.owner != ""`) is a second, independent backstop but does not substitute for the upstream fix.
- **v0.11.x open product question (Phase 23):** whether `shared` visibility (DEC-kyz, "readable by any authenticated caller") crosses service-tenant boundaries once multiple isolated service-principal owners exist — must be an explicit, tested policy decision, not an assumption.
- **v0.11.x requirements-clarification (Phase 24):** the same-key/different-content idempotency contract (reject vs. explicit upsert) must be locked before Phase 24 planning proceeds past its first test.
- Tracked tech debt from v0.10.x: #369 (Renovate self-heal live observation, post-merge only), #366 (console e2e harness), #370 (Taskfile yamlfmt/CI reconciliation) — carried forward, not in v0.11.x scope.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260717-g1r | Triage + fix #301 — Renovate ui/ postUpgradeTasks via `bash -c` (shell-free); branch unmerged, gated on a cluster-first allowlist update | 2026-07-17 | 1462da20 | [260717-g1r-renovate-ui-vendor-shell](./quick/260717-g1r-renovate-ui-vendor-shell/) |

## Session Continuity

Last session: 2026-07-25T23:10:46.528Z
Stopped at: Completed 26-01-PLAN.md
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
| Phase 17 P05 | 20min | 2 tasks | 4 files |
| Phase 18-stateless-session-rotation P01 | 20min | 2 tasks | 3 files |
| Phase 18 P02 | 5min | 2 tasks | 2 files |
| Phase 18-stateless-session-rotation P03 | 20min | 2 tasks | 10 files |
| Phase 19 P01 | 25min | 3 tasks | 11 files |
| Phase 19 P02 | 15min | 3 tasks | 6 files |
| Phase 19 P03 | 20min | 3 tasks | 9 files |
| Phase 19 P04 | 25min | 2 tasks | 4 files |
| Phase 19 P05 | 35min | 2 tasks | 6 files |
| Phase 19 P06 | 62min | 3 tasks | 12 files |
| Phase 20-correctness-polish P01 | 12min | 3 tasks | 9 files |
| Phase 20-correctness-polish P02 | 20 | 2 tasks | 3 files |
| Phase 20-correctness-polish P03 | 25min | 1 tasks | 2 files |
| Phase 20 P04 | 3min | 3 tasks | 5 files |
| Phase 21 P01 | 6min | 2 tasks | 3 files |
| Phase 21 P02 | 15min | 3 tasks | 5 files |
**Per-Plan Metrics:**

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 22 P01 | 8min | 3 tasks | 13 files |
| Phase 22 P02 | 5min | 3 tasks | 2 files |
| Phase 22 P03 | 3min | 3 tasks | 3 files |
| Phase 23 P01 | 14min | 2 tasks | 2 files |
| Phase 23 P02 | 12min | 2 tasks | 2 files |
| Phase 23 P03 | 12min | 2 tasks | 2 files |
| Phase 23 P04 | 25min | 2 tasks | 4 files |
| Phase 23 P05 | 20min | 2 tasks | 1 files |
| Phase 23-service-auth-chain-tenancy-isolation P06 | 20min | 3 tasks | 9 files |
| Phase 24 P01 | 12min | 2 tasks | 5 files |
| Phase 24 P02 | 9min | 3 tasks | 2 files |
| Phase 25 P01 | 4min | 2 tasks | 2 files |
| Phase 25 P02 | 3min | 2 tasks | 6 files |
| Phase 26 P01 | 10min | 2 tasks | 9 files |

## Operator Next Steps

- Run `/gsd-plan-phase 22` to plan the Cedar Authz Foundation & Store Enforcement phase.
