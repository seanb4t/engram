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
- [ ] **Phase 2: Headless CLI Client** - `engram search|store|list` over the generated Connect stubs, agent-shaped output, credential safety
- [ ] **Phase 3: Cross-Spine Memory Recall** - `cross_spine` on `search_memory` with the store-layer authz composition verified, not assumed
- [ ] **Phase 4: Diagnosability** - Authz decisions reach a reader; rejections name the true field and carry a remediation hint; provider error bodies survive
- [ ] **Phase 5: Operator Config & Reindex Correctness** - Per-lane chat credential; tag-aware resume plus a repair path for already-skipped records
- [ ] **Phase 6: Rule Capture — Investigation & Fix** - Find why `store_rule` never fires, then fix the documented cause without touching who decides

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
- [ ] 02-03-PLAN.md — Bare-invocation self-describe catalog derived from the live command tree, with an anti-drift gate on the exit codes (wave 3)

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
implementation begins.

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
concurrently — sequence them or keep the diffs small and independently reviewed.

---

### Phase 6: Rule Capture — Investigation & Fix

**Goal:** Find out why `store_rule` effectively never fires — one rule exists repo-wide against
dozens of ordinary memories — and fix the cause that is actually there.

**Requirements:** REQ-rule-capture-investigation, REQ-rule-capture-intervention

**Success criteria:**

1. A written root cause exists, derived from tracing actual invocation **attempts including
   failures** across the chain (`curating-memory` skill routing → session-start rules index →
   `store_rule` tool description → user-blessing gate), and it distinguishes a mechanical/bug cause
   from a friction cause.

2. The intervention addresses that documented cause, not a presumed one.
3. Rule capture demonstrably fires in a scenario where it previously did not.
4. No path promotes a rule without explicit user instruction — the user-blessed gate is intact and
   proven so.

**Internal gate:** criterion 1 must be satisfied and reviewed before any intervention is
implemented. The roadmap deliberately does **not** commit to a specific fix; research supplied
consent-preserving candidates but explicitly warned against choosing one before the trace exists.
Separated from v0.12.x Phase 5 because the shape differs — investigation-gated, and the surfaces are skill
markdown and tool descriptions rather than Go correctness.

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
| 1. Authorization & Isolation | v0.8.x | 3/3 | Complete | shipped (v0.8.x) |
| 2. Recall Semantics | v0.8.x | 3/3 | Complete | shipped (v0.8.x) |
| 3. Memory Kinds & Tools | v0.8.x | 3/3 | Complete | shipped (v0.8.x) |
| 4. Embedder | v0.8.x | 1/1 | Complete | shipped (v0.8.x) |
| 5. Config & Transport | v0.8.x | 2/2 | Complete | shipped (v0.8.x) |
| 6. Telemetry & Observability | v0.8.x | 2/2 | Complete | shipped (v0.8.x) |
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
| 2. Headless CLI Client | v0.12.x | 3/4 | In Progress (2/3 plans, wave 2 of 3) | — |
| 3. Cross-Spine Memory Recall | v0.12.x | 0/3 | Pending | — |
| 4. Diagnosability | v0.12.x | 0/4 | Pending | — |
| 5. Operator Config & Reindex Correctness | v0.12.x | 0/3 | Pending | — |
| 6. Rule Capture — Investigation & Fix | v0.12.x | 0/2 | Pending | — |

**v0.9.x — Recall Quality: ✅ shipped 2026-07-10 (PR #336) · 6/6 requirements · audit PASSED.**
**v0.10.x — Hardening & Write Lane: ✅ shipped 2026-07-16 · 9 phases (13–21) · 19/20 requirements (REQ-ci-renovate-spa-drift's live self-heal observation deferred, post-merge → #369) · audit tech_debt (9/9 Nyquist, 0 blockers).** Full detail: `milestones/v0.10.x-ROADMAP.md`.
**v0.11.x — Capture & Service Identity: ✅ shipped 2026-07-26 · 5 phases (22–26), 19 plans, 46 tasks · 11/11 requirements · audit PASSED (6/6 integration seams, 2/2 E2E flows, 0 blockers; Nyquist 5/5 validated — phases 24 and 26 reconciled 2026-07-26, 0 gaps).** Full detail: `milestones/v0.11.x-ROADMAP.md`.
