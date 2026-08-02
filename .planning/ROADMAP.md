# Roadmap: engram

## Overview

This is a **retrospective / as-built roadmap** for v0.8.x/v0.9.x, extended by GSD-tracked
milestones going forward. Phases 1–7 group the already-completed v0.8.x work by synthesis area —
Authorization & Isolation, Recall Semantics, Memory Kinds & Tools, Embedder, Config & Transport,
Telemetry & Observability, and Web UI / Docs Site / Distribution. All 56 ADR-locked decisions (25
core + 31 companion refinements, folded 2026-07-08) and all 24 routed v0.8.x requirements are
implemented and merged to main. Per-phase implementation plans are cross-referenced in
`.planning/intel/merge-plans/context.md`. Phase 8 (Connect observe-lane auth hardening, R1–R4) was
**found already shipped** during a 2026-07-08 reconciliation — the cookie/OIDC lane landed
opportunistically inside PR #248 and was hardened in PR #266, before this retrospective baseline
was authored; the earlier "deferred stub" framing (ingested from the 2026-06-09 plan/spec, which
described the interim anonymous state as current) was stale. Phases 9–12 (v0.9.x — Recall
Quality) shipped 2026-07-10 (PR #336); full detail archived at `milestones/v0.9.x-ROADMAP.md`.
Phases 13–21 (v0.10.x — Hardening & Write Lane) shipped 2026-07-16; full detail archived at
`milestones/v0.10.x-ROADMAP.md`. Success criteria are stated as observable truths that hold when
each phase completes.

Phases 22–26 (v0.11.x — Capture & Service Identity) shipped 2026-07-26; full detail archived at
`milestones/v0.11.x-ROADMAP.md`. The research-derived build order held end to end: the Cedar authz
foundation landed first as the trust anchor (a behavior-preserving refactor of `internal/store`'s
filter/gate functions, refining LOCKED `DEC-cgb` via new ADR `engram-cdr1` rather than overriding
it), then service-auth-chain + tenancy isolation — where the milestone's #1 risk, a service
principal silently resolving to `owner==""`, was proven fail-closed as the phase's first test —
then the capture trio in strict internal order (idempotency → supersession → citations, since
supersession reuses idempotency's re-Upsert mechanism), with the category filter and the
chat/summarize base-URL split as a low-risk independent tail. The milestone held its two standing
constraints: zero new store-layer authz **primitive**, and (except `cedar-go`) zero new
dependencies — every feature extended an existing seam.

**Active milestone — v0.12.x — Headless Reach & Diagnosability (Phases 1–6), opened 2026-07-29.**
Two halves: make engram reachable by agents that are **not** a top-level MCP client, and make what
the server decides and rejects legible. The structural root is a bearer-token identity on the
ConnectRPC lane — today that lane has exactly one credential type (a sealed cookie session) and one
reason to be mounted (the UI is enabled), so a headless deployment has no Connect surface at all.
Research (HIGH confidence, 4-dimension fan-out at `.planning/research/`) confirmed **zero new Go
dependencies** are required and found two security-critical, silently-passing defect classes
concentrated in that first phase: a CSRF exemption keyed on request-controlled input would be a full
bypass on all six write RPCs, and Connect never routes through `mcpauth.RequireBearerToken`, whose
private `verify()` is the only place `TokenInfo.Expiration` is enforced — so reusing
`auth.ChainVerifier` alone makes token expiry decorative on that lane. Both compile, vet, lint, and
pass a happy-path suite. Per the v0.11.x precedent, the fail-closed negative tests are v0.12.x Phase 1's
first tests, not follow-up work. Research also raised and deliberately **did not resolve** a
disagreement about `cross_spine` (v0.12.x Phase 3): whether the store-layer authz `Must` clause composes
independently of the scope clause is to be settled by reading `Store.Search` end to end, not by
analogy to `search_discovery`.

## Milestones

- ✅ **v0.8.x Baseline** — Phases 1–7 (shipped)
- ✅ **Connect Auth Hardening** — Phase 8 (shipped; R1–R4 verified 2026-07-08)
- ✅ **v0.9.x — Recall Quality** — Phases 9–12 (shipped 2026-07-10, PR #336): retrieval eval + ranking precision (#261), embedder query/document asymmetry (#305), async-on-write summaries (#320), per-memory usage signals (#317). Full detail archived at `milestones/v0.9.x-ROADMAP.md`.
- ✅ **v0.10.x — Hardening & Write Lane** — Phases 13–21 (shipped 2026-07-16): embedder reliability & options (#333/#332/#331/#334/#337, closes #261), Connect write lane + CSRF + stateless session rotation (#322/#323), correctness & polish tail, CI/maintenance hygiene. 19/20 requirements (REQ-ci-renovate-spa-drift's live self-heal observation deferred, post-merge only → #369). Full detail archived at `milestones/v0.10.x-ROADMAP.md`.
- ✅ **v0.11.x — Capture & Service Identity** — Phases 22–26 (shipped 2026-07-26): Cedar authz foundation (#362/#373 trust anchor), service auth chain + tenancy isolation (#362/#373), idempotent capture (#340), supersession with history (#342), structured citations + category filter + chat base URL (#341/#374/#350). 11/11 requirements, audit PASSED. Full detail archived at `milestones/v0.11.x-ROADMAP.md`.
- 🔨 **v0.12.x — Headless Reach & Diagnosability** — Phases 1–6 (opened 2026-07-29): Connect bearer identity + headless mount + CSRF provenance (#343), headless CLI client (#343), cross-spine memory recall (#344), diagnosability trio (#394/#360/#347), operator config & reindex correctness (#350/#345), rule-capture investigation & fix (#351). 20 requirements. `REQUIREMENTS.md` + `research/SUMMARY.md`.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): milestone work
- Decimal phases (2.1, 2.2): urgent insertions (marked INSERTED)

<details>
<summary>✅ v0.8.x Baseline (Phases 1–7) — SHIPPED</summary>

- [x] **v0.8.x Phase 1: Authorization & Isolation** - Per-actor read isolation, write gating, opt-in sharing, configurable owner key
- [x] **v0.8.x Phase 2: Recall Semantics** - Summary-by-default, tag/temporal gating, windowed cursor paging, payload indexes
- [x] **v0.8.x Phase 3: Memory Kinds & Tools** - Discovery + rule kinds, schedule tools, short_id handle
- [x] **v0.8.x Phase 4: Embedder** - Protocol-named connection vars + asymmetric query/document param passthrough
- [x] **v0.8.x Phase 5: Config & Transport** - ENGRAM_ koanf config, Config.Validate, fatal legacy guard, explicit MCP path
- [x] **v0.8.x Phase 6: Telemetry & Observability** - slog + OTel over OTLP at every seam, never blocking startup
- [x] **v0.8.x Phase 7: Web UI, Docs Site & Distribution** - Operator console SPA, docs site, brand system, bundled client plugin

</details>

<details>
<summary>✅ Connect Auth Hardening (Phase 8) — SHIPPED (PR #248/#266)</summary>

- [x] **v0.8.x Phase 8: Connect Observe-Lane Auth Hardening** - Cookie/OIDC observe lane replaces the interim anonymous mount (R1–R4); shipped in PR #248/#266

</details>

<details>
<summary>✅ v0.9.x — Recall Quality (Phases 9–12) — SHIPPED 2026-07-10 (PR #336)</summary>

Full detail archived at `milestones/v0.9.x-ROADMAP.md`.

- [x] **Phase 9: Retrieval Eval Harness & Ranking Precision** - Labeled retrieval eval (recall@k/MRR), always-on similarity scores in `search_memory`, dependency-free reranker to kill phrasing-sensitivity — chosen by the eval numbers (completed 2026-07-10)
- [x] **Phase 10: Asymmetric Query/Document Embeddings** - Native API-param passthrough (cloud) + document-side prefix (E5/nomic) for query≠document embeds — found ALREADY SHIPPED under Phase 4 (verified 2026-07-10; #305 closed; no plans built)
- [x] **Phase 11: Async-on-Write Summaries** - In-process worker drains `FillSummary` after upsert, off the synchronous write path; eval-gated (completed 2026-07-10)
- [x] **Phase 12: Per-Memory Usage Signals** - Strong-signal counters (get/update) via hybrid OTLP + payload `access_count`; never affects ranking (completed 2026-07-10)

</details>

<details>
<summary>✅ v0.10.x — Hardening & Write Lane (Phases 13–21) — SHIPPED 2026-07-16</summary>

Full detail archived at `milestones/v0.10.x-ROADMAP.md`.

- [x] **Phase 13: Embedder Reliability Foundation** - Configurable HTTP timeout (re-derived backoff budget) + base-URL `/v1` join fix across every provider shape + embedder-config-identity payload stamp (completed 2026-07-11)
- [x] **Phase 14: Embedder Model Options & Eval** - Direct Gemini embeddings (eval-verified task_type behavior) + #261 prod-parity re-confirm on qwen3 + docs-site/Helm model recipes (completed 2026-07-11)
- [x] **Phase 15: Additive Proto + Stub Write Handlers** - Six new write RPCs (additive-only, buf-generated), CI lint gate against `idempotency_level`, safe `CodeUnimplemented` stubs (completed 2026-07-11)
- [x] **Phase 16: CSRF Interceptor** - Origin/Sec-Fetch-Site primary defense + session-bound double-submit token on every write RPC; read lane untouched (completed 2026-07-12)
- [x] **Phase 17: Wired Write Handlers (Full CRUD + Schedule)** - deps.* subject/actor refactor + all six write RPCs delegating to the shared MCP business-logic layer, MCP/Connect parity-tested (completed 2026-07-13)
- [x] **Phase 18: Stateless Session Rotation** - Sliding-expiry cookie re-seal on every authenticated request, new ADR for the no-revocation trade-off, no server-side state (completed 2026-07-13)
- [x] **Phase 19: Console Write UX** - Create/edit/delete/re-share/schedule from the operator console over the write lane, with CSRF + a silent opportunistic auth-race retry (completed 2026-07-15; live browser E2E UAT deferred → #366)
- [x] **Phase 20: Correctness & Polish** - Discovery proto fidelity, MintShortID collision cap, embed param-key/body-build cleanup, discovery short_id schema, summarize-missing CronJob (completed 2026-07-16)
- [x] **Phase 21: CI / Maintenance Hygiene** - Renovate vendored-SPA self-heal, Phase-11 review residuals, `.rumdl.toml` `.planning` exclude (completed 2026-07-16; #301 live self-heal observation deferred, post-merge only → #369)

</details>

<details>
<summary>✅ v0.11.x — Capture & Service Identity (Phases 22–26) — SHIPPED 2026-07-26</summary>

**Milestone Goal:** Make programmatic capture correct and re-runnable, and give headless service
principals a first-class, isolated identity — so agents can write memory mechanically and safely
into shared stores.

- [x] **Phase 22: Cedar Authz Foundation & Store Enforcement** - Cedar (cedar-go v1.8.0) PDP decides authorization over enumerable buckets; `internal/store` compiles decisions into the Qdrant filter — behavior-preserving refinement of DEC-cgb (completed 2026-07-17)
- [x] **Phase 23: Service Auth Chain & Tenancy Isolation** - Pluggable verifier chain (OIDC user → OIDC client-credentials → static token); a service principal never resolves to the anonymous bucket (completed 2026-07-17)
- [x] **Phase 24: Idempotent Capture** - `store_memory` accepts an idempotency key with strict, owner-scoped, race-safe replay-safety (completed 2026-07-18)
- [x] **Phase 25: Supersession with History** - A memory can supersede another via additive links; superseded records are soft-hidden from recall but stay fetchable by id (completed 2026-07-19)
- [x] **Phase 26: Structured Citations, Category Filter & Chat Base URL** - Optional provenance on curated memories, MCP↔Connect category-filter parity, and a distinct chat/summarize base URL (completed 2026-07-25)

</details>

### 🔨 v0.12.x — Headless Reach & Diagnosability (Phases 1–6) — ACTIVE

- [x] **Phase 1: Shared Auth Chain & Connect Bearer Identity** - One composed verifier for both lanes, enforced token expiry, server-set lane provenance driving the CSRF exemption, opt-in headless mount
- [x] **Phase 2: Headless CLI Client** - `engram search|store|list` over the generated Connect stubs, agent-shaped output, credential safety (completed 2026-07-31)
- [x] **Phase 3: Cross-Spine Memory Recall** - `cross_spine` on `search_memory` with the store-layer authz composition verified, not assumed (completed 2026-08-01)
- [x] **Phase 4: Diagnosability** - Authz decisions reach a reader; rejections name the true field and carry a remediation hint; provider error bodies survive (completed 2026-08-01)
- [x] **Phase 5: Operator Config & Reindex Correctness** - Per-lane chat credential; tag-aware resume plus a repair path for already-skipped records (completed 2026-08-01)
- [x] **Phase 6: Rule Capture — Investigation & Fix** - Find why `store_rule` never fires, then fix the documented cause without touching who decides (completed 2026-08-01)

## Phase Details

### Phases 1–8 (v0.8.x — Baseline + Connect Auth Hardening) — ✅ SHIPPED

Full phase details (goals, success criteria, status) are archived at
[`milestones/v0.8.x-ROADMAP.md`](milestones/v0.8.x-ROADMAP.md). Moved out of this file on
2026-07-31 so that a bare `### Phase N:` heading resolves unambiguously to the **active**
milestone — v0.12.x restarted phase numbering at 1, and these historical headings were shadowing
it for every GSD phase-resolution verb.

- Phase 1 — Authorization & Isolation: per-actor read isolation, write gating, opt-in sharing, configurable owner key
- Phase 2 — Recall Semantics: summary-by-default, tag/temporal gating, windowed cursor paging, payload indexes
- Phase 3 — Memory Kinds & Tools: discovery + rule kinds, schedule tools, short_id handle
- Phase 4 — Embedder: protocol-named connection vars + asymmetric query/document param passthrough
- Phase 5 — Config & Transport: ENGRAM_ koanf config, Config.Validate, fatal legacy guard, explicit MCP path
- Phase 6 — Telemetry & Observability: slog + OTel over OTLP at every seam, never blocking startup
- Phase 7 — Web UI, Docs Site & Distribution: operator console SPA, docs site, brand system, bundled client plugin
- Phase 8 — Connect Observe-Lane Auth Hardening: cookie/OIDC observe lane replaces the interim anonymous mount (R1–R4); shipped in PR #248/#266

### Phases 9–12 (v0.9.x — Recall Quality) — ✅ SHIPPED 2026-07-10

Full phase details (goals, success criteria, plans, decisions, tech debt) are archived at
[`milestones/v0.9.x-ROADMAP.md`](milestones/v0.9.x-ROADMAP.md). Requirements outcomes at
[`milestones/v0.9.x-REQUIREMENTS.md`](milestones/v0.9.x-REQUIREMENTS.md). Audit (PASSED) at
[`milestones/v0.9.x-MILESTONE-AUDIT.md`](milestones/v0.9.x-MILESTONE-AUDIT.md).

- Phase 9 — Retrieval Eval Harness & Ranking Precision (3 plans): eval harness + always-on similarity score + dependency-free reranker (#261; recall@8=1.00)
- Phase 10 — Asymmetric Query/Document Embeddings: already shipped under Phase 4 (#305 closed; no plans)
- Phase 11 — Async-on-Write Summaries (3 plans): bounded worker pool off the write path (#320)
- Phase 12 — Per-Memory Usage Signals (6 plans): get/update counters, hybrid OTLP+payload, never affects ranking (#317)

### Phases 13–21 (v0.10.x — Hardening & Write Lane) — ✅ SHIPPED 2026-07-16

Full phase details (goals, success criteria, plans, decisions, tech debt) are archived at
[`milestones/v0.10.x-ROADMAP.md`](milestones/v0.10.x-ROADMAP.md). Requirements outcomes at
[`milestones/v0.10.x-REQUIREMENTS.md`](milestones/v0.10.x-REQUIREMENTS.md). Audit (tech_debt — 19/20 requirements, 1 deferred) at
[`milestones/v0.10.x-MILESTONE-AUDIT.md`](milestones/v0.10.x-MILESTONE-AUDIT.md).

- Phase 13 — Embedder Reliability Foundation (3 plans): configurable timeout + base-URL join fix + embedder-config-identity stamp (#333/#332)
- Phase 14 — Embedder Model Options & Eval (3 plans): direct Gemini + prod-parity re-confirm + model recipes (#331/#334/#337, closes #261)
- Phase 15 — Additive Proto + Stub Write Handlers (4 plans): 6 additive write RPCs, idempotency-annotation CI gate (#322)
- Phase 16 — CSRF Interceptor (3 plans): Origin/Sec-Fetch-Site + session-bound double-submit token (#322)
- Phase 17 — Wired Write Handlers (6 plans): deps.* refactor, MCP↔Connect authz parity (#322)
- Phase 18 — Stateless Session Rotation (3 plans): sliding-expiry cookie re-seal, no server state (#323)
- Phase 19 — Console Write UX (6 plans): create/edit/delete/re-share/schedule over the write lane, CSRF + auth-race retry (live browser E2E UAT deferred → #366)
- Phase 20 — Correctness & Polish (4 plans): discovery proto fidelity, MintShortID cap, embed cleanups, summarize CronJob (#307/#308/#304/#302/#303/#269)
- Phase 21 — CI / Maintenance Hygiene (3 plans): rumdl `.planning` exclude, phase-11 residuals (#335), Renovate self-heal (#301 — live observation deferred, post-merge only → #369)

### Phases 22–26 (v0.11.x — Capture & Service Identity) — ✅ SHIPPED 2026-07-26

Full phase details (goals, success criteria, plans, decisions, tech debt) are archived at
[`milestones/v0.11.x-ROADMAP.md`](milestones/v0.11.x-ROADMAP.md). Requirements outcomes at
[`milestones/v0.11.x-REQUIREMENTS.md`](milestones/v0.11.x-REQUIREMENTS.md). Audit (PASSED — 11/11
requirements, 6/6 integration seams, 2/2 E2E flows) at
[`milestones/v0.11.x-MILESTONE-AUDIT.md`](milestones/v0.11.x-MILESTONE-AUDIT.md).

- Phase 22 — Cedar Authz Foundation & Store Enforcement (3 plans): `internal/authz` cedar-go v1.8.0 PDP decides over enumerable buckets; the store compiles decisions into the Qdrant filter, byte-for-byte behavior-preserving (#362/#373, ADR engram-cdr1)
- Phase 23 — Service Auth Chain & Tenancy Isolation (6 plans): pluggable verifier chain (OIDC user → client-credentials → static token), fail-closed proof that a service principal never resolves to `owner==""` (#362/#373)
- Phase 24 — Idempotent Capture (2 plans): optional `idempotency_key`, deterministic UUIDv5 point ID, payload-only fingerprint checked before embedding, reject-not-overwrite on mismatch (#340)
- Phase 25 — Supersession with History (2 plans): `supersede_memory` back-stamps `superseded_by` via single-key SetPayload; superseded records soft-hidden from recall, still fetchable by id (#342)
- Phase 26 — Structured Citations, Category Filter & Chat Base URL (6 plans): optional citations on any category, `categories` OR-filter at MCP↔Connect parity, `ENGRAM_OPENAI_CHAT_BASE_URL` + shared shape-aware URL join (#341/#374/#350)

### Phase 1: Shared Auth Chain & Connect Bearer Identity

**Goal:** A headless caller can authenticate to the ConnectRPC lane with a bearer token — safely.
One composed verifier serves both lanes, token expiry is actually enforced, the authenticating lane
is recorded by the server, the CSRF exemption is decided from that record alone, and the lane is
mounted only when explicitly enabled.

**Requirements:** REQ-connect-bearer-identity, REQ-connect-token-expiry, REQ-connect-lane-provenance, REQ-connect-headless-mount

**Success criteria:**

1. A bearer token accepted on the MCP lane is accepted on the Connect lane, and one rejected there
   is rejected here — both resolve through a single composed verifier constructed **once**, proven
   structurally (not two independently-built chains that can drift).

2. A token whose `Expiration` has passed is rejected on the Connect lane. *(Written as the phase's
   first test, per the v0.11.x fail-closed precedent — this closes a live gap, not a hypothetical.)*

3. A cookie-authenticated caller is still rejected on all six write RPCs when it omits
   `X-CSRF-Token`, and cannot obtain the bearer exemption by attaching a garbage `Authorization`
   header to its session.

4. A bearer verification failure never authenticates via the cookie lane.
5. With the UI disabled and the headless flag unset, no Connect handler is registered —
   byte-for-byte today's behavior, so no deployment gains a surface on upgrade.

**Why these four ship together:** the provenance stamp and the exemption that reads it are one
atomic unit. Shipping the stamp alone would land a value nothing reads — precisely the defect
`REQ-authz-decision-diagnostics` exists to fix. (Contrast v0.10.x's Phase 15, where deliberately
unreachable stubs *were* a genuinely separable increment.)

**Research flag:** needs research at plan time — the extraction shape for a transport-agnostic
expiry check out of the go-sdk's `RequireBearerToken`/`verify()` internals. Also warrants a
security-focused plan review given the CSRF-bypass and confused-deputy risk classes.
*Resolved 2026-07-31 (`01-RESEARCH.md`): there is no extraction — `verify()` is unexported and its
only exported caller wraps a whole `http.Handler`, so the two checks are reimplemented in
`internal/auth`.*

**Plans:** 4 plans in 3 waves

Plans:

- [x] 01-01-PLAN.md — Tracer: bearer identity on Connect, lane-stamped, CSRF exemption reads the stamp (wave 1)
- [x] 01-02-PLAN.md — Reseal gates on the cookie lane; MCP↔Connect bearer and actor parity (wave 2)
- [x] 01-03-PLAN.md — `connect.headless` config key, build-once verifier injection, mount/bearer decoupling + startup refusal (wave 2)
- [x] 01-04-PLAN.md — Operator docs and Helm value for the headless lane; deferred follow-up filed (wave 3)

---

### Phase 2: Headless CLI Client

**Goal:** An agent with only a shell — a subagent with a closed tool list, a CI step, a cron loop —
can search, store, and list memories against a remote engram server.

**Requirements:** REQ-cli-client-commands, REQ-cli-agent-output, REQ-cli-credential-safety, REQ-cli-self-describing

**Depends on:** v0.12.x Phase 1 (strict).

**Success criteria:**

1. `engram search`, `engram list`, and `engram store` complete against a running server given a
   server URL and a token, emitting structured JSON when stdout is not a TTY.

2. Data goes to stdout and diagnostics to stderr; exit codes distinguish auth failure from
   not-found from validation failure from transport failure; no command prompts on any path.

3. A token supplied by env var or file never appears in `argv`, and TLS verification cannot be
   disabled silently.

4. A bare invocation returns the full command / flag / exit-code catalog as structured output.
5. No client subcommand imports `internal/store`, `internal/authz`, or `internal/embed`.

**Why one phase, not two:** the dependency boundary research identified — `search`/`list` need only
the bearer mount while `store` additionally needs the CSRF exemption green — collapses because
v0.12.x Phase 1 delivers both. Splitting would create a phase whose only distinction is which half of an
already-landed dependency it uses.

**Plans:** 3 plans in 3 waves

Plans:

- [x] 02-01-PLAN.md — Tracer: `engram search` end-to-end over a real Connect server, plus the shared client foundation and the `Execute` exit-code taxonomy (wave 1)
- [x] 02-02-PLAN.md — `engram list` and `engram store` expansion over the proven client; the write is attempted exactly once (wave 2)
- [x] 02-03-PLAN.md — Bare-invocation self-describe catalog derived from the live command tree, with an anti-drift gate on the exit codes (wave 3)

---

### Phase 3: Cross-Spine Memory Recall

**Goal:** An agent can recall curated memories across every scope it is permitted to see, with the
authorization filter proven un-widened rather than assumed safe.

**Requirements:** REQ-cross-spine-search, REQ-cross-spine-authz-verified, REQ-cross-spine-result-provenance

**Success criteria:**

1. `Store.Search`'s filter construction has been read end to end and it is recorded **in writing**
   that the owner/authz `Must` clause is composed as a separate, unconditional entry from the scope
   clause — never a combined condition where omitting scope could drop part of the authz gate.

2. A two-owner isolation test against **real Qdrant** (testcontainers, not a mock) proves owner A's
   `cross_spine=true` search over overlapping scope names never returns owner B's private records —
   and it exists and passes **before** the feature is implemented.

3. `cross_spine=true` returns hits from multiple scopes; omitting it returns only the named scope.
4. Available on MCP and Connect at parity via an additive proto field.
5. Every result is attributable to its originating scope, and the response reports which scopes were
   searched — so "found nothing here" is distinguishable from "searched everywhere and found nothing."

**Research flag:** needs research at plan time. Criterion 1 is the unresolved
architecture-vs-pitfalls disagreement and is a **gate**, not a task — it must close before
implementation begins. **Resolved:** criterion 1 CLOSED 2026-08-01 (`03-AUTHZ-GATE.md`, commit
`a7f827b6`), amended in plan 03-01 to cover `listFilter`.

**Plans:** 5/5 plans executed in 5 waves

Plans:

- [x] 03-01-PLAN.md — The non-vacuous two-owner authz isolation proof against real Qdrant, plus the recorded RED-by-mutation transcript, landing before any filter edit (wave 1)
- [x] 03-02-PLAN.md — Tracer: `cross_spine=true` on `search_memory` end-to-end through the guard, the typed core, and the `ownerScopeFilter` conditional (wave 2)
- [x] 03-03-PLAN.md — Expansion: cross-spine `list_memory`, plus `searched_scopes`/`scopes_truncated` and per-result scope attribution on the MCP lane (wave 3)
- [x] 03-04-PLAN.md — Connect + proto parity via six additive fields, read explicitly and never inferred from an empty scope (wave 4)
- [x] 03-05-PLAN.md — Agent-facing guidance across the tool reference, `curating-memory`, and the repo memory contract, including when not to widen (wave 5)

---

### Phase 4: Diagnosability

**Goal:** What the server decided, and why it rejected something, reaches whoever needs it — the
operator debugging a denial, the agent retrying a rejected call.

**Requirements:** REQ-authz-decision-diagnostics, REQ-validation-error-attribution, REQ-error-hint-envelope, REQ-embed-provider-error-body

**Success criteria:**

1. At debug level, **both** an allowed and a denied authorization decision emit a log line carrying
   field-allowlisted Cedar diagnostics; no full expression trace is ever emitted.

2. An argument-validation rejection names the field that actually failed, proven by a matrix with
   one case per single-field-invalid input rather than by matching exact wording.

3. A rejection carries a structured remediation hint alongside the field attribution.
4. A non-2xx embeddings response surfaces a bounded prefix of the provider's error body alongside
   the status code and drains the body for connection reuse; the chat/summarize lane has been
   audited for the same gap and fixed if it shares it.

**Why grouped:** four independent fixes across different subsystems sharing one design discipline —
bounded, structured, redaction-conscious disclosure. Grouping lets a single internal convention
emerge instead of three ad hoc mechanisms.

**Scope note (D-06a, decided 2026-08-01):** research proved issue #360's misleading
`missing properties: ["content"]` comes from the go-sdk's schema layer BEFORE any `tools.go` code
runs, so criterion 2's original scope could not have fixed its own motivating bug. Scope was extended
to move required-ness out of the advertised tool schema and into engram's validation. That, D-08's
message reformat, and D-11's Connect code widening ship together as one `feat!`.

**Plans:** 7/7 plans executed in 5 waves

Plans:

- [x] 04-01-PLAN.md — Tracer: the field+hint argument-error envelope proven end-to-end on one rejection across both wire lanes, plus the `connectError` class dispatch and the one-way-door decision gate (wave 1)
- [x] 04-02-PLAN.md — Cedar decision diagnostics: a narrow allowlisted accessor over `authz.Decision`, a debug line on both arms at the two `internal/store` chokepoints, and a negative gate proving no expression trace is reachable (wave 1)
- [x] 04-03-PLAN.md — Provider lanes: the bounded embeddings error body, the drain on BOTH lanes, the dimension-sizeable success bound, and the phase's only new test helper (wave 1)
- [x] 04-04-PLAN.md — The `tools.go` sweep and the criterion-2 matrix, gated behind a read-first proof that the MCP 401 auth body is out of D-08's scope (wave 2)
- [x] 04-05-PLAN.md — `rules.go` and `connectapi.go`: the last unwrapped validator, and removing the seven hand-wraps that would otherwise make D-11 a no-op on the Connect lane (wave 3)
- [x] 04-06-PLAN.md — D-06a: required-ness moves from the inferred schema into engram, with issue #360's own repro as a named regression and its positive control (wave 4)
- [x] 04-07-PLAN.md — Agent- and operator-facing guidance for the three new contracts, plus the upgrade note (wave 5)

---

### Phase 5: Operator Config & Reindex Correctness

**Goal:** The chat lane can carry its own provider credential, and an interrupted reindex resumes
without silently leaving stale vectors behind.

**Requirements:** REQ-per-lane-api-key, REQ-reindex-resume-tags, REQ-reindex-stale-repair

**Success criteria:**

1. The chat/summarize client uses its own API key when set and inherits the shared key when unset;
   behavior with it unset is byte-identical to today. Closes #350.

2. `reindex --resume` re-embeds a record whose tags changed while content did not, **and** skips one
   where both match (the paired positive control — without it, a resume that silently stops skipping
   anything looks green while quietly re-embedding everything). Tag comparison is order-independent.

3. An operator can identify and heal records an earlier unpatched `--resume` run skipped
   incorrectly, via a documented path following the existing one-time-reconciliation command
   precedent.

**Note:** the per-lane-key edit and v0.12.x Phase 3's cross-spine edit both touch `tools.go` in different
functions (`summarizerFromConfig` vs. `searchMemory`). Low but non-zero merge risk if landed
concurrently — sequence them or keep the diffs small and independently reviewed. Phase 3 has since
shipped, so this risk is resolved.

**Plans:** 3 plans

Plans:

- [x] 05-01-PLAN.md — per-lane chat credential: config key, construction-site fallback, end-to-end test, Helm value, checksum re-pin (wave 1)
- [x] 05-02-PLAN.md — reindex resume tag-awareness: shared tag decoder, order-independent comparison, three-conjunct predicate, `--dry-run --resume` sizing (wave 1)
- [x] 05-03-PLAN.md — operator docs: corrected shared-key prose, the pre-patch-resume repair path and its limit, v0.12.0 upgrade entries, phase-close gates (wave 2)

**Note on execution:** waves 1 and 2. `git.branching_strategy` is `none`, so 05-01 and 05-02 run
concurrently in one working directory sharing one git index — both plans mandate explicit-pathspec
commits. 05-02 deliberately avoids bare `task` because 05-01 is transiently mid-edit on the chart
drift checksum; the full gate runs in 05-03.

---

### Phase 6: Rule Capture — Investigation & Fix

**Goal:** An agent with the skill and MCP installed *proposes* rules instead of waiting to be
asked, and the rule set that results stays free of duplicates, contradictions, and rot.

**Requirements:** REQ-rule-capture-investigation, REQ-rule-capture-intervention, REQ-rule-curation-hygiene

**Amended 2026-08-01 (discuss).** The original goal asked why `store_rule` "effectively never
fires — one rule exists repo-wide against dozens of ordinary memories." Both halves of that
premise failed under examination: three rules exist, two created after this roadmap was written;
and a rule-count-to-memory-count ratio was never evidence of anything, since rules are per-scope,
normative and user-blessed while memories are per-fact and continuous. Sean restated the real
problem — the agent never *suggests* a rule, it has to be pushed — and added rule-hygiene to the
scope. Root cause was established during discuss (see `06-CONTEXT.md` D-01/D-02) rather than
deferred to research: the instruction to propose exists at `curating-memory/SKILL.md:51-53` but is
buried inside its own prohibition and gated on a belief nothing in the skill produces.

**Success criteria:**

1. A written root cause exists distinguishing a mechanical cause from a friction cause, grounded in
   evidence rather than reconstruction. (Satisfied in discuss: friction — no trigger exists.)

2. The intervention addresses that documented cause, not a presumed one.
3. Rule capture demonstrably fires in a scenario where it previously did not.
4. No path promotes a rule without explicit user instruction — the user-blessed gate is intact and
   proven so. Deletion is gated symmetrically: an agent may propose removing a rotted or
   contradictory rule, never remove one itself.

5. The rule set has a hygiene discipline covering duplicates, contradictions, and rot — accounting
   for the fact that rules cannot be superseded (correction is delete-then-re-store) and that
   session start loads only the one-line index, so full-text checks cost a fetch.

**Internal gate:** criterion 1 must be satisfied and reviewed before any intervention is
implemented. This roadmap deliberately does **not** commit to a specific fix.
Separated from v0.12.x Phase 5 because the shape differs — investigation-gated, and the surfaces are skill
markdown and tool descriptions rather than Go correctness.

**Plans:** 3 plans

Plans:

- [x] 06-01-PLAN.md — de-bury the permission: `### Proposing a rule` with two observable triggers, the inline protocol, the `rule-declined` record, the skill `description` cue, and the tool-reference/`CLAUDE.md` mirrors; closes REQ-rule-capture-investigation by citation (wave 1)
- [x] 06-02-PLAN.md — `### Rule hygiene` (duplicates, contradictions, rot, the code-verified correction table, user-blessed deletion, the `list_rules(full=true)` price) and `### One-time rule backfill sweep` (wave 2)
- [x] 06-03-PLAN.md — closed `REQ-rule-capture-intervention` by citation to `06-COLD-READ.md`'s PASS, restored the milestone-completion cadence clause (rule `7smp8vy9hr`), and ran the phase-close gates; the live backfill sweep against the three named gotchas is reassigned to the orchestrator (no MCP access in this environment) and recorded as a scaffold in `06-DEMONSTRATION.md` pending its result (wave 3)

**Note on execution:** waves 1, 2, and 3, strictly sequential — all three plans edit
`skill/engram/skills/curating-memory/SKILL.md` and two of them edit
`docs-site/.../reference/tools.md`, so there is no parallelism to take. `git.branching_strategy` is
`none`, so every commit needs an explicit pathspec (`git commit -m "..." -- <files>`); `git add -A`
and `git commit -am` are banned. 06-01 and 06-03 each carry one blocking human checkpoint: a cold
read of the corrected section in wave 1 (deliberately early — it is the only detector for this
phase's core risk) and the live sweep in wave 3. The phase should change no Go; the plans gate on
`git diff --exit-code <base> -- '*.go' go.mod go.sum` and treat any Go diff as scope drift.

---

## Progress

**Execution Order:** 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 (v0.8.x, shipped) · 9 → 10 → 11 → 12 (v0.9.x, shipped 2026-07-10) · 13 → 14 (embedder track) · 15 → 16 → 17 → 18 → 19 (write-lane track, strict order) · 20 → 21 (independent) — v0.10.x shipped 2026-07-16 · 22 → 23 (Cedar foundation → service auth/tenancy, strict order) · 24 → 25 → 26 (capture trio + recall/config tail, strict order; 24 can start in parallel with 22–23) — v0.11.x shipped 2026-07-26 · **v0.12.x: 1 → 2 (spine → CLI, strict order)** · **3 · 4 · 5 · 6 (independent of the spine and of each other; may run in parallel once 1 is underway — sequence 3 and 5 or keep their `tools.go` diffs small)** — v0.12.x planned 2026-07-29

> **Phase numbering restarts per milestone as of v0.12.x.** Phases 1–26 above are the pre-v0.12.x
> monotonic sequence and keep their historical numbers. In **prose**, a phase number is only
> meaningful with its milestone — always write `v0.12.x Phase 1`, never bare `Phase 1`. The
> `Milestone` column below is part of the row key. Note that `gsd-tools query find-phase <N>` takes a
> bare number and globs every archived `milestones/vX.Y.x-phases/` directory, so it may report a hit
> from another milestone; qualify by milestone at the call site.
>
> **Structural invariant (added 2026-07-31): a bare `Phase N` anchor always means the ACTIVE
> milestone.** GSD's phase resolvers match a bare `Phase N` in the `## Phases` checklist and the
> `### Phase N:` detail headings. When v0.12.x restarted numbering, the still-inline v0.8.x
> sections shadowed it — every resolver silently answered from v0.8.x, which is how
> `roadmap.update-plan-progress` came to overwrite shipped v0.8.x history (repaired in `e5e9ce4c`).
> Fixed by archiving the v0.8.x detail sections to
> [`milestones/v0.8.x-ROADMAP.md`](milestones/v0.8.x-ROADMAP.md) (matching v0.9.x–v0.11.x) and
> qualifying their checklist entries as `v0.8.x Phase N`. So: **structural anchors for the active
> milestone stay bare; archived milestones whose numbers collide get milestone-qualified.** Keep
> `**Requirements:**` on ONE line per phase — a wrapped line truncates `phase_req_ids` to whatever
> fits before the break.

| Phase | Milestone | Requirements | Status | Completed |
|-------|-----------|--------------|--------|-----------|
| v0.8.x Phase 1: Authorization & Isolation | v0.8.x | 3/3 | Complete | shipped (v0.8.x) |
| v0.8.x Phase 2: Recall Semantics | v0.8.x | 3/3 | Complete | shipped (v0.8.x) |
| v0.8.x Phase 3: Memory Kinds & Tools | v0.8.x | 3/3 | Complete | shipped (v0.8.x) |
| v0.8.x Phase 4: Embedder | v0.8.x | 1/1 | Complete | shipped (v0.8.x) |
| v0.8.x Phase 5: Config & Transport | v0.8.x | 2/2 | Complete | shipped (v0.8.x) |
| v0.8.x Phase 6: Telemetry & Observability | v0.8.x | 2/2 | Complete | shipped (v0.8.x) |
| 7. Web UI, Docs Site & Distribution | v0.8.x | 9/9 | Complete | shipped (v0.8.x) |
| 8. Connect Auth Hardening | v0.8.x | 1/1 | Complete | shipped (PR #248/#266) |
| 9. Retrieval Eval & Ranking Precision | v0.9.x | 3/3 | Complete | 2026-07-10 (PR #336) |
| 10. Asymmetric Query/Document Embeddings | v0.9.x | 1/1 | Complete (already shipped) | 2026-07-10 (#305) |
| 11. Async-on-Write Summaries | v0.9.x | 1/1 | Complete | 2026-07-10 (PR #336) |
| 12. Per-Memory Usage Signals | v0.9.x | 1/1 | Complete | 2026-07-10 (PR #336) |
| 13. Embedder Reliability Foundation | v0.10.x | 3/3 | Complete    | 2026-07-11 |
| 14. Embedder Model Options & Eval | v0.10.x | 3/3 | Complete    | 2026-07-11 |
| 15. Additive Proto + Stub Write Handlers | v0.10.x | 4/4 | Complete    | 2026-07-11 |
| 16. CSRF Interceptor | v0.10.x | 3/3 | Complete    | 2026-07-12 |
| 17. Wired Write Handlers (Full CRUD + Schedule) | v0.10.x | 6/6 | Complete    | 2026-07-13 |
| 18. Stateless Session Rotation | v0.10.x | 3/3 | Complete    | 2026-07-13 |
| 19. Console Write UX | v0.10.x | 6/6 | Complete   | 2026-07-15 |
| 20. Correctness & Polish | v0.10.x | 4/4 | Complete    | 2026-07-16 |
| 21. CI / Maintenance Hygiene | v0.10.x | 3/3 | Complete   | 2026-07-16 |
| 22. Cedar Authz Foundation & Store Enforcement | v0.11.x | 3/3 | Complete | 2026-07-17 |
| 23. Service Auth Chain & Tenancy Isolation | v0.11.x | 6/6 | Complete | 2026-07-17 |
| 24. Idempotent Capture | v0.11.x | 2/2 | Complete | 2026-07-18 |
| 25. Supersession with History | v0.11.x | 2/2 | Complete   | 2026-07-19 |
| 26. Structured Citations, Category Filter & Chat Base URL | v0.11.x | 6/6 | Complete | 2026-07-25 |
| 1. Shared Auth Chain & Connect Bearer Identity | v0.12.x | 4/4 | Complete | 2026-07-31 |
| 2. Headless CLI Client | v0.12.x | 4/4 | Complete | 2026-07-31 |
| 3. Cross-Spine Memory Recall | v0.12.x | 3/3 | Complete | 2026-08-01 |
| 4. Diagnosability | v0.12.x | 4/4 | Complete | 2026-08-01 |
| 5. Operator Config & Reindex Correctness | v0.12.x | 3/3 | Complete | 2026-08-01 |
| 6. Rule Capture — Investigation & Fix | v0.12.x | 3/3 | Complete | 2026-08-01 |

**v0.9.x — Recall Quality: ✅ shipped 2026-07-10 (PR #336) · 6/6 requirements · audit PASSED.**
**v0.10.x — Hardening & Write Lane: ✅ shipped 2026-07-16 · 9 phases (13–21) · 19/20 requirements (REQ-ci-renovate-spa-drift's live self-heal observation deferred, post-merge → #369) · audit tech_debt (9/9 Nyquist, 0 blockers).** Full detail: `milestones/v0.10.x-ROADMAP.md`.
**v0.11.x — Capture & Service Identity: ✅ shipped 2026-07-26 · 5 phases (22–26), 19 plans, 46 tasks · 11/11 requirements · audit PASSED (6/6 integration seams, 2/2 E2E flows, 0 blockers; Nyquist 5/5 validated — phases 24 and 26 reconciled 2026-07-26, 0 gaps).** Full detail: `milestones/v0.11.x-ROADMAP.md`.

### Phase 7: CLI Cross-Spine Wiring

**Goal:** [To be planned]
**Requirements**: TBD
**Depends on:** Phase 6
**Plans:** 0 plans

Plans:

- [ ] TBD (run /gsd-plan-phase 7 to break down)

---

## Backlog

Unsequenced ideas parked outside the active phase sequence. Promote with `/gsd-review-backlog`.

### Phase 999.1: Spine review / consolidate / verify / purge / archive command (BACKLOG)

**Goal:** [Captured for future planning] A single operation that audits a memory spine end to end —
reviews what is there, consolidates duplicates and near-duplicates, verifies records still hold
against the tree they describe, purges what has rotted, and archives what is finished — instead of
the ad-hoc, per-session, human-driven curation this repo does today.

**Requirements:** TBD
**Plans:** 0 plans

**Why this came up (captured 2026-08-01, during the v0.12.x milestone close):**

Rule `7smp8vy9hr` already mandates a curation pass at milestone completion, and specifies a real
procedure — extract embedded reusable gotchas into standalone records FIRST, then write one
authoritative milestone summary, then delete the collapsed per-phase process records, never touching
reusable codebase facts. But it is a **playbook a human or agent executes by hand**, with an explicit
safety clause requiring user confirmation before large delete batches. Nothing enforces the order,
nothing detects when it is overdue, and nothing verifies the extract actually happened before the
delete.

The v0.12.x session surfaced every failure mode this command would address:

- **Consolidate** — `2ak73h8bta` had to be superseded by `478rhhmhb0` because one of its three
  claims (`ENGRAM_REQUIRE_QDRANT` fails closed) was false for `internal/store`. Found by accident
  during phase planning, not by any review pass.

- **Review** — three records filed `category: gotcha` were phrased as normative MUSTs and were
  really rule candidates. Surfaced only because v0.12.x Phase 6 went looking. One (`r3bjakymtz`)
  became rule `n6m4as49mr`; two were declined (`hxwad6qr58`).

- **Verify** — records cite `file:line` anchors (`store.go:752-757`, `rules.go:103-146`). Those drift
  every time the file is edited. Nothing checks whether a cited anchor still points at what the
  record claims.

- **Purge / archive** — per-phase lifecycle records for shipped milestones accumulate; `7smp8vy9hr`
  says to delete them but only after extraction, and deletes are irreversible.

**Shape questions to resolve when this is planned:**

- Is this an engram CLI command (`engram spine-review`, joining `migrate-remap-owner` /
  `backfill-short-ids` / `prune-expired` / `summarize-missing`), a GSD skill, or both? The existing
  one-time-reconciliation commands all resolve **structural** predicates; "is this record still
  true" and "are these two records the same fact" are **semantic** judgments — the same split that
  made v0.12.x Phase 6 route its backfill sweep to an agent procedure rather than a CLI.

- What is proposed vs. what is performed? Deletes are irreversible and rules are user-blessed, so
  the consent model from v0.12.x Phase 6 (`### Proposing a rule`, `### Rule hygiene`) is the
  precedent — propose, never promote; the same should hold for purge.

- Cadence: on demand, at milestone close (hooking `7smp8vy9hr`'s existing moment), or on a volume
  signal like `rules.go`'s existing `ruleThreshold = 50`.

- Scope: spine only, or overlays too? Interaction with the `promoting-memory` skill, which already
  graduates overlay memories into the spine.

Plans:

- [ ] TBD (promote with /gsd-review-backlog when ready)

---

### Phase 999.2: Review the CLI and MCP surface under the self-evident-interface principle (BACKLOG)

**Goal:** [Captured for future planning] Audit every command, operation, flag, and tool argument on
both the `engram` CLI and the MCP tool surface against one standard: it must be discoverable and
usable **correctly by reading** — from help text, from the naming of operations and parameters, and
from the self-describe / tool-schema output alone. No teaching by example, no error-and-find-out, no
surprises.

**Requirements:** TBD
**Plans:** 0 plans

**Why this came up (captured 2026-08-02, during v0.12.x Phase 7 discussion):**

Stated by Sean as a general principle while deciding how `--cross-spine` should behave:

> "the flags, cli help, all of this should _read_ well and be discoverable for an agent or human. no
> surprises, no error and wait to see how it works. The goal is to NOT need to teach by example how
> to use the cli, it should be evident from its help and the naming of its operations and
> parameters."

Phase 7 applies it to two commands. This item applies it to the whole surface.

Two concrete failures motivated it, both found by the v0.12.x milestone audit and its follow-on
discussion:

- **A capability with no way to reach it.** `cross_spine` shipped on the Connect API in v0.12.x
  Phase 3; the CLI shipped in Phase 2 and never wired it. Nothing in `engram search --help`
  suggested the capability existed, so the only way to discover the gap was to read the proto.

- **A default that fails without saying why.** `engram search --query x` with no `--scope` is
  rejected by `effectiveSearchScope` (`internal/server/tools.go:1374-1382`) with *"scope is required
  unless cross_spine is true"* — a rule the CLI's help text never states. The most natural
  invocation teaches by failure, which is exactly the pattern this principle forbids.

**Scope to audit:**

- `cmd/engram/*` — every flag's help string, every command's `Short`/`Use`, and whether related
  flags name each other (mutually-exclusive pairs, conditionally-required pairs).
- The v0.12.x Phase 2 D-15 self-describe JSON catalog — an agent's primary discovery path; it must
  carry the same guidance as `--help`, not a thinner version.
- MCP tool descriptions and argument docs in `internal/server` — same standard, different surface.
  Server-side conditional-requirement rules (`effectiveSearchScope` and its siblings) should be
  stated wherever the argument is advertised.

**Related:** convention `yaj7dqz9qq` — *"a new tool argument with no guidance is an incomplete
feature."* This item is that convention applied retroactively and surface-wide, rather than only at
the moment an argument is added.

Plans:

- [ ] TBD (promote with /gsd-review-backlog when ready)
