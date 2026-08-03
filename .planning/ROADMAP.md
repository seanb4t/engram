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

**Active milestone — v0.12.x — Headless Reach & Diagnosability (Phases 1–7), opened 2026-07-29.**
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
- ✅ **v0.12.x — Headless Reach & Diagnosability** — Phases 1–7 (shipped 2026-08-02): Connect bearer identity + headless mount + CSRF provenance (#343), headless CLI client (#343), cross-spine memory recall (#344), diagnosability trio (#394/#360/#347), operator config & reindex correctness (#350/#345), rule-capture investigation & fix (#351), CLI cross-spine wiring. 21/21 requirements, audit `tech_debt` (0 blockers). Full detail archived at `milestones/v0.12.x-ROADMAP.md`.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): milestone work
- Decimal phases (2.1, 2.2): urgent insertions (marked INSERTED)

<details>
<summary>✅ v0.8.x Baseline (Phases 1–7) — SHIPPED</summary>

Full detail archived at [`milestones/v0.8.x-ROADMAP.md`](milestones/v0.8.x-ROADMAP.md). The detail
sections were moved out of this file on 2026-07-31 so a bare `### Phase N:` heading resolves to the
**active** milestone — v0.12.x restarted phase numbering at 1, and the historical headings were
shadowing it for every GSD phase-resolution verb.

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

Full detail archived at [`milestones/v0.9.x-ROADMAP.md`](milestones/v0.9.x-ROADMAP.md).
Requirements outcomes at [`milestones/v0.9.x-REQUIREMENTS.md`](milestones/v0.9.x-REQUIREMENTS.md).
Audit (PASSED) at [`milestones/v0.9.x-MILESTONE-AUDIT.md`](milestones/v0.9.x-MILESTONE-AUDIT.md).

- [x] **Phase 9: Retrieval Eval Harness & Ranking Precision** - Labeled retrieval eval (recall@k/MRR), always-on similarity scores in `search_memory`, dependency-free reranker to kill phrasing-sensitivity — chosen by the eval numbers (completed 2026-07-10)
- [x] **Phase 10: Asymmetric Query/Document Embeddings** - Native API-param passthrough (cloud) + document-side prefix (E5/nomic) for query≠document embeds — found ALREADY SHIPPED under Phase 4 (verified 2026-07-10; #305 closed; no plans built)
- [x] **Phase 11: Async-on-Write Summaries** - In-process worker drains `FillSummary` after upsert, off the synchronous write path; eval-gated (completed 2026-07-10)
- [x] **Phase 12: Per-Memory Usage Signals** - Strong-signal counters (get/update) via hybrid OTLP + payload `access_count`; never affects ranking (completed 2026-07-10)

</details>

<details>
<summary>✅ v0.10.x — Hardening & Write Lane (Phases 13–21) — SHIPPED 2026-07-16</summary>

Full detail archived at [`milestones/v0.10.x-ROADMAP.md`](milestones/v0.10.x-ROADMAP.md).
Requirements outcomes at [`milestones/v0.10.x-REQUIREMENTS.md`](milestones/v0.10.x-REQUIREMENTS.md).
Audit (tech_debt — 19/20 requirements, 1 deferred) at
[`milestones/v0.10.x-MILESTONE-AUDIT.md`](milestones/v0.10.x-MILESTONE-AUDIT.md).

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

Full detail archived at [`milestones/v0.11.x-ROADMAP.md`](milestones/v0.11.x-ROADMAP.md).
Requirements outcomes at [`milestones/v0.11.x-REQUIREMENTS.md`](milestones/v0.11.x-REQUIREMENTS.md).
Audit (PASSED — 11/11 requirements, 6/6 integration seams, 2/2 E2E flows) at
[`milestones/v0.11.x-MILESTONE-AUDIT.md`](milestones/v0.11.x-MILESTONE-AUDIT.md).

- [x] **Phase 22: Cedar Authz Foundation & Store Enforcement** - Cedar (cedar-go v1.8.0) PDP decides authorization over enumerable buckets; `internal/store` compiles decisions into the Qdrant filter — behavior-preserving refinement of DEC-cgb (completed 2026-07-17)
- [x] **Phase 23: Service Auth Chain & Tenancy Isolation** - Pluggable verifier chain (OIDC user → OIDC client-credentials → static token); a service principal never resolves to the anonymous bucket (completed 2026-07-17)
- [x] **Phase 24: Idempotent Capture** - `store_memory` accepts an idempotency key with strict, owner-scoped, race-safe replay-safety (completed 2026-07-18)
- [x] **Phase 25: Supersession with History** - A memory can supersede another via additive links; superseded records are soft-hidden from recall but stay fetchable by id (completed 2026-07-19)
- [x] **Phase 26: Structured Citations, Category Filter & Chat Base URL** - Optional provenance on curated memories, MCP↔Connect category-filter parity, and a distinct chat/summarize base URL (completed 2026-07-25)

</details>

<details>
<summary>✅ v0.12.x — Headless Reach & Diagnosability (Phases 1–7) — SHIPPED 2026-08-02</summary>

- [x] **Phase 1: Shared Auth Chain & Connect Bearer Identity** - One composed verifier for both lanes, enforced token expiry, server-set lane provenance driving the CSRF exemption, opt-in headless mount
- [x] **Phase 2: Headless CLI Client** - `engram search|store|list` over the generated Connect stubs, agent-shaped output, credential safety (completed 2026-07-31)
- [x] **Phase 3: Cross-Spine Memory Recall** - `cross_spine` on `search_memory` with the store-layer authz composition verified, not assumed (completed 2026-08-01)
- [x] **Phase 4: Diagnosability** - Authz decisions reach a reader; rejections name the true field and carry a remediation hint; provider error bodies survive (completed 2026-08-01)
- [x] **Phase 5: Operator Config & Reindex Correctness** - Per-lane chat credential; tag-aware resume plus a repair path for already-skipped records (completed 2026-08-01)
- [x] **Phase 6: Rule Capture — Investigation & Fix** - Find why `store_rule` never fires, then fix the documented cause without touching who decides (completed 2026-08-01)
- [x] **Phase 7: CLI Cross-Spine Wiring** - `--cross-spine` on `engram search|list` through one shared guard, with the coverage footer and bidirectional help text that make it learnable by reading (completed 2026-08-02)

</details>

## Phase Details

No active milestone. v0.12.x detail sections are archived at
[`milestones/v0.12.x-ROADMAP.md`](milestones/v0.12.x-ROADMAP.md); the next milestone's phases land
here when `/gsd-new-milestone` runs.

---

## Progress

**Execution Order:** 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 (v0.8.x, shipped) · 9 → 10 → 11 → 12 (v0.9.x, shipped 2026-07-10) · 13 → 14 (embedder track) · 15 → 16 → 17 → 18 → 19 (write-lane track, strict order) · 20 → 21 (independent) — v0.10.x shipped 2026-07-16 · 22 → 23 (Cedar foundation → service auth/tenancy, strict order) · 24 → 25 → 26 (capture trio + recall/config tail, strict order; 24 can start in parallel with 22–23) — v0.11.x shipped 2026-07-26 · v0.12.x: 1 → 2 (spine → CLI, strict order) · 3 · 4 · 5 · 6 (independent of the spine and of each other; ran in parallel once 1 was underway) · 7 (CLI cross-spine wiring, closed the audit seam between 2 and 3) — v0.12.x shipped 2026-08-02

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
**v0.12.x — Headless Reach & Diagnosability: ✅ shipped 2026-08-02 · 7 phases (1–7, first milestone on restarted numbering), 28 plans, 68 tasks · 21/21 requirements · audit `tech_debt` (5/5 integration seams, 2/2 E2E flows, 0 blockers; Nyquist not validated — 6 phases at `status: draft`, phase 2 has none, tracked as debt not gaps).** Full detail: `milestones/v0.12.x-ROADMAP.md`.


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
