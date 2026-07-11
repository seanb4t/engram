# Project Research Summary

**Project:** engram — v0.10.x "Hardening & Write Lane"
**Domain:** Self-hosted, OAuth-secured MCP memory server (Go + Qdrant) adding a browser-facing Connect write lane
**Researched:** 2026-07-10
**Confidence:** HIGH

## Executive Summary

This milestone bolts a cookie-authenticated write surface onto a Connect API that has so far
only served reads, plus two independent reliability fixes to the embedder client and a Helm
scheduling gap. The four research passes converge on the same core thesis: almost nothing new
needs to be built from scratch — the security-sensitive and business-logic machinery this
milestone needs already exists in internal/store and internal/server/tools.go's deps methods;
the job is wiring, not invention. Zero new Go dependencies are required anywhere. The single
biggest risk is architectural, not technological: it is trivially easy to wire the new Connect
write RPCs the way the existing read RPCs are wired (straight to internal/store) and thereby
skip the authz/business-logic guards (rule immutability, existence-leak re-wrap, summary
reconciliation) that actually live one layer up, in the MCP tool handlers. All four research
files flag this as the #1 pitfall, independently.

The recommended approach is a dependency-ordered build sequence, not a feature-parallel one: fix
the embedder (isolated, unblocks nothing else, unblocked by nothing) first as low-risk
throughput; then do the additive proto + stub write RPCs; then the mechanical deps.* subject/
actor refactor that lets Connect and MCP share one code path; then the CSRF interceptor; then
wire the write handlers through that shared path; then stateless session rotation; then console
UI wiring. This order lets the highest-risk work (CSRF, rotation) land on top of an already-
refactored, already-tested shared write path rather than being invented and tested for the first
time at the same moment new security surface is exposed.

The key risks are: (1) silent authz/business-logic drift between MCP and Connect write paths if
handlers bypass deps.* (Pitfall 1, critical); (2) a proto footgun (idempotency_level =
NO_SIDE_EFFECTS) that would make a mutating RPC reachable via unauthenticated-looking GET,
defeating CSRF entirely (Pitfall 2); (3) session rotation reintroducing server-side state or a
client-held live credential, reversing the locked stateless-cookie ADR (DEC-u9v) without an
explicit new ADR (Pitfall 7); and (4) a provider-specific embedder footgun where Gemini's
OpenAI-compat shim silently no-ops task_type asymmetric-embedding params, producing a valid
response with wrong semantics and no error (Pitfall 12). Mitigation for all four is procedural
(parity tests, CI lint gates, an explicit ADR, an eval re-run) more than architectural — the
architecture research already prescribes the shape that avoids them.

## Key Findings

### Recommended Stack

Zero new Go modules. Every capability in scope — CSRF (net/http.CrossOriginProtection, stdlib
since Go 1.25, project is on 1.26.3), session rotation (extend the existing sealed AES-GCM
cookie + already-vendored golang.org/x/oauth2/coreos/go-oidc), embedder timeout/base-URL fix
(stdlib http.Client.Timeout + strings normalization), and Gemini embeddings (reuse the existing
OpenAI-compat embed.Client, no google.golang.org/genai SDK) — is achievable with the toolchain
and dependencies already pinned. The Helm CronJob for summarize-missing is a sibling batch/v1
template reusing the existing image/env-var plumbing, no new tooling.

**Core technologies:**
- `net/http.CrossOriginProtection` (Go 1.25+ stdlib): primary CSRF defense via Origin/Sec-Fetch-Site — zero new deps, matches project's stdlib-first convention.
- `internal/webauth.SessionCodec` (existing AES-GCM codec, extended, not replaced): carries rotation state without a server-side store — honors DEC-u9v.
- Existing `internal/embed.Client` (OpenAI-compat): reused for Gemini via its documented `/v1beta/openai` compat endpoint — no second SDK.
- Kubernetes `batch/v1` `CronJob`: standard sibling Helm template, no in-process scheduler library.

**Do NOT add:** gorilla/csrf or filippo.io/csrf (stdlib supersedes them), any server-side
session/token store (Redis/Postgres — reverses DEC-u9v), google.golang.org/genai (unneeded SDK
weight), a bespoke CSRF-token-issuing RPC (unnecessary round trip).

### Expected Features

**Must have (table stakes, P1):**
- StoreMemory/StoreDiscovery Connect write RPCs delegating to the existing deps methods (not reimplementing them).
- CSRF protection (Origin/Sec-Fetch-Site primary defense) on every state-changing RPC.
- Session refresh/rotation that doesn't drop an in-flight write on access-token expiry.
- ENGRAM_EMBED_TIMEOUT configurable knob, replacing the hardcoded 30s.
- Base-URL join fix for the embedder endpoint (the OpenRouter /v1/v1/embeddings bug).
- Embedding-model documentation + Helm recipes, explicitly stating the reindex-on-model-change requirement.

**Should have (P2, explicit scoping decision needed — not silently deferred):**
- UpdateMemory/DeleteMemory/SetVisibility Connect RPCs — full CRUD parity for console completeness; #322 names only Store*, so this is DECISION 1 below.
- Direct Google Gemini embeddings — verify current API shape against docs before implementation.

**Defer (v2+, explicit anti-features):**
- Batch/bulk write RPCs (no MCP-side precedent, would need co-design).
- Auto-extraction/auto-capture from console activity (permanently excluded — violates zero-junk invariant).
- Usage-signal feedback from write-lane traffic into ranking (violates D-08 invariant).
- Handler-level authz duplicating store-layer gates (anti-pattern, not a feature).

### Architecture Approach

This is an integration, not greenfield work. The existing Connect mux already has an interceptor
chain (otelIc → access-log → subject interceptor); the write lane adds one new interceptor
(CSRF, innermost, gated to a static set of write-procedure names so reads are untouched) and one
mechanical refactor: deps.* write methods (storeMemory, updateMemory, deleteMemory,
setVisibility, scheduleMemory, storeDiscovery) currently resolve identity internally via
ctx-scoped lookups tied to the MCP go-sdk's context key; they must be refactored to take an
explicit subj store.Subject, actor string parameter so both the MCP tool-registration call site
and the new Connect write handlers can call the identical method body. internal/store itself
does not change at all — it remains the single default-deny authz chokepoint.

**Major components:**
1. `internal/server/tools.go` deps.* write methods — refactored to explicit subject/actor params; the shared delegation target for both lanes (MODIFIED, not new).
2. `internal/server/connectcsrf.go` (new) — CSRF interceptor, innermost in the chain, gated to write-procedure names only.
3. `internal/webauth/{session,handlers,resolver}.go` — extended for sliding-expiry re-seal on every authenticated request; stays stateless.
4. `proto/engram/v1/engram.proto` + `gen/` — additive-only new write RPCs/messages, buf-generated, committed.
5. `internal/embed/embed.go` — fully isolated blast radius for timeout/base-URL/Gemini work; no import-graph overlap with the write-lane files above.

### Critical Pitfalls

1. **Connect write RPC bypasses deps.* and calls store.* directly** — reintroduces exactly the DEC-cgb handler-vs-store authz split the project already rejected, one layer up (business logic, not authz) this time; a rule record could become mutable/un-shareable over Connect even though DEC-iedk locks it immutable over MCP. Avoid by treating every Connect write handler as a thin proto/args adapter over the identical deps method the MCP tool calls, verified by MCP/Connect parity tests.
2. **idempotency_level = NO_SIDE_EFFECTS proto misannotation** makes a mutating RPC reachable via plain HTTP GET — no preflight, no custom header needed, blind CSRF via a bare `<img src>`. Avoid with a CI/lint gate asserting this option never appears on any write RPC, plus a raw-net/http GET regression test per write RPC.
3. **Stateless session rotation reintroduces server-side state or a client-held live credential** without redeciding what "rotation" means under DEC-u9v's no-store constraint — a stolen sealed cookie could self-renew indefinitely with no revocation mechanism. Avoid with an explicit new ADR before implementation, and keep the Session struct free of any live upstream OIDC token.
4. **Permissive CORS reintroduced on the Connect mux** to support a future cross-origin client silently restores cross-origin credentialed POST for every write RPC, since same-origin-only (not SameSite=Lax) is the actual load-bearing CSRF mitigation today. Keep TestConnectNoCORSHeaders as a permanent CI gate.
5. **Existence-leak re-wrap omitted on a new by-id write RPC** reopens the DEC-xa6 cross-actor existence leak, now via a browser-visible network tab. Avoid with a table test per new write RPC asserting the caller's original input (never the resolved UUID) appears in not-found errors.

## Implications for Roadmap

Based on research, the four research files converge on the same dependency-ordered build
sequence. This is the backbone the roadmap should follow — not a feature checklist to be
parallelized freely.

### Phase 1: Embedder Reliability Foundation
**Rationale:** Fully isolated (confirmed by direct file-read: zero import-graph overlap with internal/webauth/connectapi.go/proto/) — no dependency on or from the write-lane work in either direction. Ship it first as low-risk, immediately-valuable throughput (fixes a real production brownout root cause from v0.9.x evals) while the write-lane design settles.
**Delivers:** ENGRAM_EMBED_TIMEOUT config knob; base-URL /v1 join fix (table-tested across Ollama/vLLM/OpenRouter/Gemini shapes); optionally direct Gemini embeddings support.
**Addresses:** FEATURES.md P1 items ENGRAM_EMBED_TIMEOUT, base-URL join fix; P2 Gemini support.
**Avoids:** Pitfall 10 (timeout raised without re-deriving summaryqueue.go's backoff budget), Pitfall 11 (join fix over/under-corrects — needs a full provider-shape test matrix), Pitfall 12 (Gemini task_type silent no-op — needs an eval-harness re-run, not just a docs note), Pitfall 13 (reindex-boundary — needs an explicit documented decision, DECISION 3 below).

### Phase 2: Additive Proto + Stub Write Handlers
**Rationale:** Establishes the wire contract before any business logic touches it, so the buf-gen drift discipline (additive-only, no renumbering) is validated early and the SPA/console team can start against a stable (if CodeUnimplemented) contract.
**Delivers:** New StoreMemoryRequest/Response etc. proto messages + RPCs appended to EngramService; regenerated gen/go/gen/ts; write RPCs registered on engramAPI but returning CodeUnimplemented via the embedded Unimplemented...Handler (safe default).
**Uses:** connect-go v1.20.0 (already pinned, no bump needed).
**Implements:** ARCHITECTURE.md Pattern 5 (additive-only proto changes with buf drift discipline).
**Must avoid at this stage:** Pitfall 2 — no idempotency_level option on any new RPC; add the CI lint/grep gate now, before any handler logic exists to hide behind it.

### Phase 3: deps.* Subject/Actor Refactor
**Rationale:** This is the mechanical prerequisite that makes every subsequent write-handler phase safe by construction rather than by discipline — it must land before real write logic is implemented, or the temptation (and the git history) of a second, store-direct code path forms.
**Delivers:** storeMemory/updateMemory/deleteMemory/setVisibility/scheduleMemory/storeDiscovery refactored to accept explicit subj store.Subject, actor string params; MCP call sites updated (unchanged behavior); actor-resolution parity fix for the Connect lane (TokenInfo.UserID currently unset on the web/cookie lane — DECISION 2 below).
**Implements:** ARCHITECTURE.md Pattern 1 (lane-agnostic write methods) and Pattern 2 (actor resolution parity).
**Avoids:** Pitfall 1 (the single highest-risk item in the milestone) — by construction, not by review.

### Phase 4: CSRF Interceptor
**Rationale:** Must land before real write handlers are wired through, so the write lane never exists in a CSRF-unprotected state even transiently in the phase sequence.
**Delivers:** internal/server/connectcsrf.go — new interceptor, innermost in the chain (after the subject interceptor, before the handler), gated to the static set of write-procedure names; session-bound synchronizer/double-submit CSRF token (non-HttpOnly, HMAC over session identity, not a bare random value); TestConnectNoCORSHeaders-style regression gate kept in CI.
**Addresses:** FEATURES.md CSRF UX section; ARCHITECTURE.md Pattern 3.
**Avoids:** Pitfall 3 (permissive CORS), Pitfall 4 (SameSite treated as sufficient), Pitfall 5 (double-submit implemented backwards — HttpOnly-on-the-wrong-cookie or not session-bound).
**Flag for /gsd-secure-phase.**

### Phase 5: Wired Write Handlers (StoreMemory/StoreDiscovery + CRUD-parity decision)
**Rationale:** Now that the shared deps.* path (Phase 3) and CSRF gate (Phase 4) both exist, the actual write handlers are thin, low-risk adapter code.
**Delivers:** StoreMemory/StoreDiscovery RPCs calling the shared deps methods with Connect-resolved subject/actor; error-code mapping identical to existing read RPCs; existence-leak re-wrap preserved on every by-id path; MCP/Connect parity tests per RPC. **DECISION 1** (CRUD scope) must be made explicitly here, not silently — see below.
**Avoids:** Pitfall 1, Pitfall 6 (existence-leak re-wrap omitted).
**Flag for /gsd-secure-phase.**

### Phase 6: Stateless Session Rotation
**Rationale:** Ordered after the write lane exists (rotation's value is keeping long write-capable sessions alive) but is architecturally independent enough to be its own phase with its own ADR gate — sequencing it after Phase 5 avoids conflating "does the write lane work" review with "is the security posture of the whole cookie model still sound" review.
**Delivers:** Sliding-expiry re-seal on every authenticated Connect request (read or write); no new server-side state; **DECISION 2** (rotation approach) resolved and recorded as a new ADR before implementation, not silently drifted past DEC-u9v.
**Avoids:** Pitfall 7 (no revocation — mandatory ADR), Pitfall 8 (re-seal race — monotonic absolute-forward-jump expiry, not delta-from-old-value), Pitfall 9 (clock skew — explicit skew budget for the rotation threshold, hard expiry stays strict).
**Flag for /gsd-secure-phase — mandatory** (changes the security posture of the whole cookie-auth model).

### Phase 7: Console Wiring
**Rationale:** Last — depends on every prior phase's wire contract (proto, CSRF token issuance/echo pattern, rotation UX) being stable.
**Delivers:** SPA "add memory"/discovery forms; client-side CSRF-token interceptor mirroring the server-side pattern; silent-refresh retry-once-then-reauth UX preserving in-flight write input.
**Addresses:** FEATURES.md "differentiator" CSRF-token-silently-refreshed-alongside-rotation item.

### Phase Ordering Rationale

- **Embedder work is genuinely independent** (confirmed by direct file-read across all four research files) — it can and should ship first or in parallel, never gated behind or gating the write-lane work (Anti-Pattern 4 in ARCHITECTURE.md explicitly warns against coupling them).
- **The deps.* refactor must precede both CSRF and the wired write handlers** — it is the structural guarantee against Pitfall 1, not an optional cleanup step.
- **CSRF must land before write handlers are wired** so there is no window where the write lane exists without its primary defense.
- **Session rotation is deliberately its own phase, after the write lane**, because it requires an explicit ADR decision (DECISION 2) that should not be made implicitly as a side effect of wiring write handlers.

### Three Decisions to Surface Explicitly (not silently resolve)

**DECISION 1 — CRUD scope of the Connect write lane.** GitHub #322 names only StoreMemory/
StoreDiscovery. All four research files independently flag that a console which can create but
not edit/delete/reschedule/re-share memories is a confusing half-write-lane, and recommend
scoping UpdateMemory/DeleteMemory/SetVisibility (and lower-priority ScheduleMemory) as an
explicit in-scope-if-time decision for this milestone rather than a silent v0.11.x deferral. The
roadmap must record this as a named decision point, not assume either answer.

**DECISION 2 — Session rotation approach.** Two options, not one:
(a) Stateless sliding-expiry re-seal — re-seal Session{Owner, Expiry: now+sessionTTL} on every
successful authenticated request, no new server-side state, honors DEC-u9v as-is. This is the
ARCHITECTURE.md-recommended default.
(b) A true refresh-token model (server holds the real OIDC refresh token, standard
rotation-with-reuse-detection) — requires a new ADR superseding or extending DEC-u9v/DEC-8q3,
adds server-side custody of a live credential, and per PITFALLS.md Pitfall 7 is unsafe to
implement naively (no store means no reuse-detection, meaning a stolen sealed cookie could
self-renew indefinitely). The roadmap must force this choice into the open at Phase 6 planning
time, with (a) as the default unless the team explicitly commits to writing the new ADR for (b).

**DECISION 3 — Reindex-boundary enforcement.** Changing ENGRAM_EMBED_MODEL/base-URL-to-a-
different-provider/query-document-params in production without running engram reindex silently
corrupts recall (Qdrant's dimension check passes but the semantic space has shifted) — this is
PITFALLS.md's Pitfall 13, explicitly called out as "must document/decide, not necessarily must
code." The roadmap must record an explicit choice: (a) v0.10.x only documents the gap (docs +
Helm recipes pairing every ENGRAM_EMBED_* change with a reindex callout, per DECISION-adjacent
#337 work), or (b) v0.10.x also stamps each record with an embedder-config-identity hash in
payload metadata to make a future audit/enforcement possible. Do not let this fall through
unaddressed while three separate levers (base-URL fix, Gemini, docs work) each make the mistake
easier to trigger.

### Research Flags

**Needs deeper research at plan time (/gsd-plan-phase --research-phase N):**
- **Phase 5/6 (CSRF + rotation):** security-sensitive centerpiece per PROJECT.md; STACK.md and ARCHITECTURE.md both flag the CSRF Origin/Sec-Fetch-Site + double-submit-token combination and the rotation ADR decision as needing explicit threat-modeling before/during planning, not just at research time.
- **Phase 1 (Gemini embeddings, if in scope):** STACK.md/FEATURES.md/PITFALLS.md all flag the exact Gemini OpenAI-compat wire shape and task_type/dimension-truncation behavior as verified via search-result synthesis only, not a direct fetch — do one direct curl/find-docs lookup against the live endpoint before locking this phase's plan, and extend the Phase 9 eval harness to the shipped Gemini config before documenting it as a recommended recipe (this is a correctness gate, not optional polish — a silent recall regression has no error to catch it).

**Standard patterns, skip research-phase:**
- **Phase 2 (additive proto + stub handlers):** mechanical, field-for-field mirror of existing MCP tool arg/return shapes; buf-gen discipline is already established project convention.
- **Phase 3 (deps.* refactor):** small, mechanical signature change with a fully specified before/after shape already in ARCHITECTURE.md.
- **Phase 7 (console wiring):** standard connect-es client interceptor pattern, mirrors the existing server-side interceptor shape.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Verified via direct pkg.go.dev/GitHub-releases reads for stdlib CSRF, connect-go, go-oidc, oauth2 pins; Gemini OpenAI-compat endpoint confirmed via official docs but not a live fetch (flagged). |
| Features | HIGH (mechanics) / MEDIUM (Gemini specifics) | Write-lane/CSRF/rotation UX grounded directly in the current codebase's locked invariants (DEC-cgb/xa6/iedk/ddiw); Gemini embeddings API shape not independently fetched this session. |
| Architecture | HIGH | Every pattern anchored to a specific file/function read directly this session (connectapi.go, connectauth.go, tools.go, webauth/*, store.go, embed.go, serve.go); the TokenInfo.UserID gap was confirmed by reading the go-sdk source itself. |
| Pitfalls | HIGH (codebase mechanics) / MEDIUM (a few provider-specific claims) | Store/webauth/connectapi/embed pitfalls are codebase-grounded with exact file:line citations; the Gemini task_type and clock-skew-in-practice claims are sourced from vendor docs/forum + general distributed-systems reasoning, not engram-specific incidents. |

**Overall confidence:** HIGH

### Gaps to Address

- **Gemini OpenAI-compat exact wire shape** (response JSON field names, task_type behavior per model version, output_dimensionality renormalization requirement) — verify with a direct fetch/find-docs lookup before locking Phase 1's plan if Gemini support ships this milestone.
- **go-oidc v3.19.0 → v3.20.0 bump** — optional, thematically adjacent (back-channel-logout support) but not required for basic rotation; worth a quick spike during Phase 6 planning, not a blocker.
- **CronJob env-var duplication** — whether to factor memory-mcp.yaml's ~40-line env block into a shared _helpers.tpl template consumed by both the Deployment and the new CronJob; flag as a refactor decision during the Helm CronJob phase's planning, not a research gap per se.
- **DECISION 1/2/3 above** are not gaps in the research — they are explicit forks the research deliberately did not resolve on the roadmap's behalf. The roadmap must surface all three as named decision points during phase planning, not default silently to either branch.

## Sources

### Primary (HIGH confidence)
- Direct reads of internal/server/{connectapi,connectauth,connectobs,identity,tools}.go,
  internal/webauth/{session,handlers,resolver}.go, internal/store/store.go,
  internal/embed/embed.go, internal/config/{registry,validate}.go, cmd/engram/serve.go,
  proto/engram/v1/engram.proto, charts/engram/templates/memory-mcp.yaml, .planning/PROJECT.md
  (DEC-cgb/12c/xa6/kyz/iedk/ddiw/u9v/8q3/378/zyhq/bgj/0lu/g37x/8xe/wot), go.mod — read this
  session, 2026-07-10.
- /Users/sean/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.6.1/auth/auth.go — confirmed
  no exported setter for TokenInfoFromContext, explaining engram's dual context-key design.
- Go stdlib net/http.CrossOriginProtection (pkg.go.dev), connectrpc/connect-go releases/source
  (protocol_connect.go), golang/oauth2 tags, Buf breaking-change docs.

### Secondary (MEDIUM-HIGH confidence)
- OWASP CSRF Prevention Cheat Sheet; Auth0/Okta/Duende refresh-token-rotation documentation
  (cross-checked across three vendors, consistent on rotation + reuse-detection).
- Gemini API docs (embeddings, OpenAI-compatibility) — official Google docs, not independently re-fetched this session for exact wire shape.

### Tertiary (LOW-MEDIUM confidence, flagged for validation)
- Google AI Developers Forum thread on task_type/OpenAI-compat-shim limitation — community
  forum, Google-account-answered, consistent with official docs but not primary-sourced.
- Refresh-token "collision causes spurious logout" UX failure mode — synthesized across vendor
  guidance, treated as a design-review flag rather than a settled implementation detail.

---
*Research completed: 2026-07-10*
*Ready for roadmap: yes*
