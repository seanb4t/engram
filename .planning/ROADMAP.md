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

**v0.12.x — Headless Reach & Diagnosability (Phases 1–7), opened 2026-07-29, shipped 2026-08-02.**
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

**v0.13.x — Curation & Self-Evidence (Phases 1–5 plus inserted 03.1), opened 2026-08-03, shipped
2026-08-12.** Closed
two classes v0.12.x left to human diligence. First: an `engram spine-review` CLI resolving
**structural** spine predicates (drifted `file:line` citations, near-duplicate candidates,
purge-eligible records, an archive tier) through the existing Subject-less operator tier
(`reindex`/`migrate-remap-owner`/`prune-expired`/`summarize-missing`/`backfill-short-ids`) — never
a new authorization path — paired with a companion curation skill for the **semantic** judgments a
CLI cannot make ("is this still true," "are these the same fact"), propose-never-perform, reusing
`store_rule`'s consent gate verbatim. Second: a correct-by-reading interface audit that states
every server-side conditional requirement on both the cobra `Usage` text and the MCP jsonschema
tags with a CI conformance gate, adds MCP tool blast-radius annotations
(`readOnlyHint`/`destructiveHint`/`idempotentHint`), pins `--help` output, and unifies the CLI's
flag-exclusivity enforcement (#453) with its exit-code taxonomy (#467) **in the same phase** —
because cobra's `MarkFlagsMutuallyExclusive` raises a plain `fmt.Errorf` that bypasses
`cliError`/`ExitCode()`, adopting #453 without resolving #467 first would reintroduce, one command
over, the exact undocumented exit-code split #467 exists to close. The exit-code change ships as a
unification (not a documented boundary), with a pinned-current-behavior regression test authored
before the change, a consumer audit, and a `guides/upgrade.md` entry. Also folds in the still-open
Nyquist `VALIDATION.md` reconciliation debt inherited from v0.12.x (six `status: draft` rows plus
one phase with none). Research (HIGH confidence) confirmed **zero new Go dependencies**: citation
drift detection is a byte compare against `Citation.Excerpt` already cached at write time,
near-duplicate scoring reuses a stored vector via `qdrant.NewQueryID` (no re-embedding), and the
flag/timeout gaps are one-line `cobra`/stdlib fixes.

**2026-08-12.01 — Record State & Schema Evolution (Phases 1–8), roadmapped 2026-08-12.** First
milestone under the CalVer label convention (rule `e325awbf7x`) — v0.13.0 is released but not yet
deployed, so three milestones of code have still never run outside tests and testcontainers. Makes
a record's full state — supersession, scheduling, archival, and its own schema version — reachable
and legible on every lane, and gives payload evolution a real mechanism instead of another one-shot
operator command. Research (HIGH confidence, zero new Go dependencies) converged unanimously across
all three tracks — stack, architecture, and pitfalls — on a seven-step dependency order, widened to
eight phases here by splitting the single heaviest requirement cluster (11 of 27 requirements, the
migration mechanism, 41% of the milestone) into a foundation phase (registry, additive-only /
reversibility invariants, the `Store.Migrate` sweep, partial-failure resume, lock-free convergence)
and a CLI phase (`engram migrate` via `registerDestructive`, status histogram, preview/apply
parity, revert, folding in `backfill-short-ids` as the registered v0→v1 step). Gate & CI integrity
(#479/#497) lands first, because this milestone authors new `internal/surfaces` key-links and past
v0.13.x Phase 1–2 key-link gates were silent no-ops. Schema versioning and the full migration
mechanism land before the Connect proto pass (#482), because proto field numbers are a permanent
one-way commitment and freezing `schema_version` on the wire before its semantics settle would be
unfixable. The typed operator renderer (#481) is an independent prerequisite that must land before
console/CLI state surfacing, not a retrofit after six new fields already flow through the untyped
renderer. The single highest-risk finding, confirmed independently by both the architecture and
pitfalls research: the codebase's own idiom for new orthogonal record state is a sibling `IsEmpty`
recall-gate condition (as already done for `superseded_by`/`archived_at`) — applying that idiom to
`schema_version`, whose cardinality is inverted (absence is the majority state at adoption, not a
minority one), would silently exclude every pre-migration record from recall. `schema_version`
therefore never appears in any recall or authz filter, proven by a negative test landed in the same
phase that introduces the field, not a later hardening pass.

## Milestones

- ✅ **v0.8.x Baseline** — Phases 1–7 (shipped)
- ✅ **Connect Auth Hardening** — Phase 8 (shipped; R1–R4 verified 2026-07-08)
- ✅ **v0.9.x — Recall Quality** — Phases 9–12 (shipped 2026-07-10, PR #336): retrieval eval + ranking precision (#261), embedder query/document asymmetry (#305), async-on-write summaries (#320), per-memory usage signals (#317). Full detail archived at `milestones/v0.9.x-ROADMAP.md`.
- ✅ **v0.10.x — Hardening & Write Lane** — Phases 13–21 (shipped 2026-07-16): embedder reliability & options (#333/#332/#331/#334/#337, closes #261), Connect write lane + CSRF + stateless session rotation (#322/#323), correctness & polish tail, CI/maintenance hygiene. 19/20 requirements (REQ-ci-renovate-spa-drift's live self-heal observation deferred, post-merge only → #369). Full detail archived at `milestones/v0.10.x-ROADMAP.md`.
- ✅ **v0.11.x — Capture & Service Identity** — Phases 22–26 (shipped 2026-07-26): Cedar authz foundation (#362/#373 trust anchor), service auth chain + tenancy isolation (#362/#373), idempotent capture (#340), supersession with history (#342), structured citations + category filter + chat base URL (#341/#374/#350). 11/11 requirements, audit PASSED. Full detail archived at `milestones/v0.11.x-ROADMAP.md`.
- ✅ **v0.12.x — Headless Reach & Diagnosability** — Phases 1–7 (shipped 2026-08-02): Connect bearer identity + headless mount + CSRF provenance (#343), headless CLI client (#343), cross-spine memory recall (#344), diagnosability trio (#394/#360/#347), operator config & reindex correctness (#350/#345), rule-capture investigation & fix (#351), CLI cross-spine wiring. 21/21 requirements, audit `tech_debt` (0 blockers). Full detail archived at `milestones/v0.12.x-ROADMAP.md`.
- ✅ **v0.13.x — Curation & Self-Evidence** — Phases 1–5 plus inserted 03.1 (shipped 2026-08-12): CLI interface enforceability (#453/#467 unified + #452 timeout), interface discoverability (conditional-rule conformance, MCP tool annotations, pinned `--help`), `engram spine-review` structural spine curation, multi-target merge supersession, a companion semantic curation skill, and Nyquist `VALIDATION.md` reconciliation (incl. #355). 23/24 requirements, audit `tech_debt` (0 blockers). Full detail archived at `milestones/v0.13.x-ROADMAP.md`.
- 🚧 **2026-08-12.01 — Record State & Schema Evolution** — Phases 1–8 (roadmapped 2026-08-12): gate & CI integrity first (#479/#497), a `schema_version` payload discriminator (absent-safe, wire-visible, never recall-gated), a versioned `internal/migrate` step registry + `Store.Migrate` sweep with mandatory additive-only/reversibility declarations, `engram migrate` via `registerDestructive` folding in `backfill-short-ids` as its first step, Connect record-state parity (#482) proven by an exhaustive round-trip test, a typed operator renderer (#481), console + CLI state surfacing, and the `RuleSweepScopeOrAllScopesRequired` registry/docs tail (#480). 27/27 v1 requirements mapped.

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

<details>
<summary>✅ v0.13.x — Curation & Self-Evidence (Phases 1–5 plus inserted 03.1) — SHIPPED 2026-08-12</summary>

Full detail archived at [`milestones/v0.13.x-ROADMAP.md`](milestones/v0.13.x-ROADMAP.md). The
`### Phase N:` detail sections were moved out of this file at milestone close so a bare heading
resolves to the **active** milestone, not this shipped one — the same reason the v0.8.x block
records.

- [x] **Phase 1: Interface Enforceability** - Flag-group validation and one exit-code taxonomy resolved together (#453/#467), plus an operator-configurable CLI request timeout (#452) (completed 2026-08-04)
- [x] **Phase 2: Interface Discoverability** - Conditional rules stated on both the cobra and MCP surfaces with a CI conformance gate, MCP tool blast-radius hints, pinned `--help` golden files (completed 2026-08-05)
- [x] **Phase 3: Spine Curation — Structural (CLI)** - `engram spine-review scan/verify/consolidate/purge/archive` through the existing Subject-less operator tier (completed 2026-08-07)
- [x] **Phase 03.1: Merge Supersession (INSERTED)** - `supersede_memory` accepts multiple `supersedes` targets, so a duplicate set collapses to one survivor with history preserved for every predecessor and no `delete_memory` in the merge path (completed 2026-08-11)
- [x] **Phase 4: Spine Curation — Semantic (Skill)** - A companion skill judges staleness and near-duplicate identity, proposing only, never mutating without consent (completed 2026-08-11)
- [x] **Phase 5: Validation Debt Reconciliation** - This milestone's own phases re-resolved against `go test -list` with each record stating what it found, and #355's drifted citations repaired as the plain docs fix they are (completed 2026-08-12)

</details>

**Milestone Goal (2026-08-12.01):** a record's full state — supersession, scheduling, archival, and
its own schema version — is reachable and legible on every lane, and payload evolution has a real
mechanism instead of another one-shot operator command.

- [x] **Phase 1: Gate & CI Integrity** - Key-link `pattern:` matching and the Qdrant testcontainer's mid-run stability fixed so this milestone's own gates can be trusted (#479/#497) (completed 2026-08-13)
- [x] **Phase 2: Record Schema Versioning Foundation** - `schema_version` discriminator: absent-safe, wire-visible, forward-compatible, and structurally incapable of narrowing recall (completed 2026-08-13)
- [x] **Phase 3: Migration Foundation (Registry, Invariants & Sweep)** - `internal/migrate`'s ordered step registry enforces additive-only + mandatory reversibility declarations; `Store.Migrate` sweeps to convergence without a collection lock (completed 2026-08-14)
- [x] **Phase 4: Migration CLI & First Customer** - `engram migrate` (status/preview/apply/revert) via `registerDestructive`, with `backfill-short-ids` folded in as the registered v0→v1 step (completed 2026-08-15)
- [x] **Phase 5: Connect Record-State Parity** - `proto`'s `Memory` gains supersession/scheduling/archival/schema-version fields in one additive pass, proven by an exhaustive field-mapping round-trip test, not `buf breaking` alone (#482) (completed 2026-08-15)
- [x] **Phase 6: Typed Operator Renderer** - `renderOperator` refactored so a json document cannot structurally widen past what its text sentence states (#481) (completed 2026-08-17)
- [x] **Phase 7: Console & CLI State Surfacing** - The operator console UI and the CLI both surface archived/superseded/scheduled/schema-version and pending-migration state (completed 2026-08-20)
- [x] **Phase 8: Registry & Docs Tail** - The shared scope-or-all-scopes guard becomes a registered conditional rule (#480); docs and CLAUDE.md brought current with what this milestone actually ships (completed 2026-08-21)

### Phase 1: Gate & CI Integrity

**Goal**: The build can actually go red for schema/migration work — key-link pattern gates are provably matchable again, and the Qdrant testcontainer no longer masks real failures with unrelated infra flakiness.
**Depends on**: Nothing (first phase)
**Requirements**: REQ-keylink-pattern-matchable, REQ-keylink-past-gates-reassessed, REQ-ci-qdrant-container-stability
**Success Criteria** (what must be TRUE):

  1. A key-link `pattern:` field containing `\\` escaping compiles into an actually-matchable `RegExp`, and a guard test proves a reintroduced corrupted-pattern instance fails the build (fail-first, not silently passing).
  2. Every v0.13.x Phase 1–2 key-link is re-resolved against the tool: each is either genuinely pinned (a test exists that would fail on regression) or explicitly recorded as unpinned — a past "key-links passed" claim is never accepted as evidence on its own.
  3. A full `go test ./...` run no longer fails from `internal/store`'s Qdrant testcontainer dying mid-run; when the container does die, its exit reason is captured in the failure output so a recurrence is diagnosable from evidence.

**Plans:** 6/6 plans complete

Plans:
**Wave 1**

- [x] 01-01-PLAN.md — Tracer: `internal/keylinks` guard core with the committed good/bad fixture pair (wave 1)
- [x] 01-04-PLAN.md — Tracer: one shared CI Qdrant service, shared-address proof per package, one-container assertion and on-failure diagnostics (wave 1)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 01-02-PLAN.md — Normalize every escaped `pattern:` repo-wide, then land the two recurring gates with D-04's asymmetric scopes (wave 2)
- [x] 01-05-PLAN.md — Per-package collection namespaces enforced by a prefix-asserting construction seam across all four Qdrant-backed packages (wave 2)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 01-03-PLAN.md — One-time v0.13.x Phase 1–2 key-link reassessment; verdict table plus the D-01 upstream-reporting decision (wave 3)
- [x] 01-06-PLAN.md — Source-level conformance gate plus prefix-disjointness proof; real CI run confirms the three mechanism claims (wave 3)

### Phase 2: Record Schema Versioning Foundation

**Goal**: Every record carries a `schema_version` discriminator that is wire-visible, absent-safe (no backfill needed), forward-compatible in both directions, and structurally incapable of narrowing recall.
**Depends on**: 2026-08-12.01 Phase 1
**Requirements**: REQ-schema-version-stamped, REQ-schema-version-never-gates-recall, REQ-schema-version-wire-visible, REQ-schema-version-forward-compatible
**Success Criteria** (what must be TRUE):

  1. Every write path (store/schedule/supersede/update) stamps the current `schema_version`, proven by a test asserting 100% of write paths stamp — not a sample.
  2. A record written before this milestone (no `schema_version` key present) reads as v0 by absence with no backfill required, and remains fully recallable through every existing search/list path, unchanged from today's behavior.
  3. `schema_version` is a plain wire-visible field on `store.Memory` (never `json:"-"`), observable on `full=true` recall and `get_memory` — the deliberate divergence from the `EmbedderIdentity`/`IdempotencyFingerprint` payload-only precedent.
  4. A negative "recall gate blast radius" test proves `schema_version` never appears in any Qdrant recall or authz filter condition built by `Search`/`SearchReranked`/`SearchDiscovery`/`List`/`ListScheduled` — the adjacent `superseded_by`/`archived_at` `IsEmpty` idiom has inverted cardinality here and copying it would silently exclude every pre-migration record from recall.
  5. A binary reads a record whose `schema_version` is NEWER than its own constant without rejecting, hiding, or downgrading it — tested in both the older-than and newer-than direction, which is what makes rolling the binary back across a schema change safe.

**Plans:** 5/5 plans complete

Plans:
**Wave 1**

- [x] 02-01-PLAN.md — Tracer: `internal/migrate` leaf package, the `SchemaVersion` field, the monotonic `payload()` stamp, absent-safe decode, payload index and operator upgrade note (wave 1)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 02-02-PLAN.md — Criterion 1: AST call-site-identity gate proving `payload()` is the only door, plus the behavioral per-write-method stamping proof (wave 2)
- [x] 02-04-PLAN.md — Criterion 5: raw-payload-injection proof of forward and backward version compatibility against real Qdrant (wave 2)
- [x] 02-05-PLAN.md — Criterion 3: wire visibility on `full=true` recall and `get_memory`, with the compact `recallView` proven untouched (wave 2)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 02-03-PLAN.md — Criterion 4: recall-gate blast-radius negative test over the filters actually transmitted to Qdrant, with a derived builder enumeration (wave 3)

### Phase 3: Migration Foundation (Registry, Invariants & Sweep)

**Goal**: A pure, dependency-free migration-step registry exists, and no step can be registered without declaring both additive-only compliance and its reversibility — enforced structurally, not caught in review. The sweep that drives it survives Qdrant's real batch non-atomicity and converges without a collection lock.
**Depends on**: 2026-08-12.01 Phase 2
**Requirements**: REQ-migration-step-registry, REQ-migration-additive-only-gated, REQ-migration-step-reversibility, REQ-migrate-partial-failure-resume, REQ-migrate-converges-without-lock
**Success Criteria** (what must be TRUE):

  1. `internal/migrate` is a stdlib-only leaf package with zero Qdrant or authz dependency, holding the ordered migration-step registry, imported by `internal/store` (never the reverse); a single `Validate` invariant checks step ordering and idempotency over the whole registry.
  2. Registering a step that removes or renames a payload key fails to build or fails a test — not a review catch — and the step interface is shaped so a per-version decoder can attach later without breaking existing steps.
  3. Registering a step that is silent about reversibility fails the same way: a reversible step must supply its inverse, an irreversible one must name why — "nobody thought about it" is not a representable state.
  4. `Store.Migrate`'s sweep survives a forced mid-sequence partial `SetPayload` failure against a real pinned Qdrant, then a subsequent resume converges the backlog to zero — reconciling by re-derivation (a fresh scroll/count), never by trusting the write call's own success/failure signal.
  5. The sweep runs with no collection lock: because the write path (Phase 2) stamps the current version before the sweep runs, new writes arrive already-current and never create new backlog, proven by a test that writes new records mid-sweep and confirms they are never re-processed.

**Plans:** 5/5 plans complete

Plans:
**Wave 1**

- [x] 03-01-PLAN.md — Tracer: one absent-key legacy record end-to-end through `NewStep`/`Validate`/`StepsFrom`, `Store.Migrate`'s re-derived backlog and `backlogFilter`'s Range+IsEmpty OR-shape, plus the additive-only check wired fail-closed into the sweep (wave 1)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 03-02-PLAN.md — Criterion 1/3: the four construction-time panics, the seal proven by an observed build failure, `Validate`'s three rules, the stdlib-only leaf gate, and the decoder door (wave 2)
- [x] 03-03-PLAN.md — Criterion 2: the seven-row additive-only key-set diff over fixture steps, with both anti-vacuity guards and three RED cycles (wave 2)
- [x] 03-04-PLAN.md — Criterion 4: forced mid-sequence `SetPayload` failure against real pinned Qdrant, including a committing-but-erroring write, then a resume that converges (wave 2)
- [x] 03-05-PLAN.md — Criterion 5: mid-sweep writes proving already-current records are never re-processed and the sweep converges with no collection lock (wave 2)

Research flag: yes — the exact `internal/migrate` step-registry API shape (step struct, `Validate` invariants, a `StepsFrom(v)` helper) and the partial-failure-resume test design need explicit attention at plan time; this is the highest-complexity test in the milestone. Resolved at plan time: see `03-RESEARCH.md` and `03-01-PLAN.md`'s `<planner_assumptions>`.

### Phase 4: Migration CLI & First Customer

**Goal**: An operator can preview, apply, and revert schema migrations through the standard destructive-tier CLI, with `backfill-short-ids` folded in as the registry's first real step — never running automatically.
**Depends on**: 2026-08-12.01 Phase 3
**Requirements**: REQ-migrate-command, REQ-migrate-status-histogram, REQ-migrate-preview-apply-parity, REQ-backfill-shortids-first-step, REQ-migrate-revert, REQ-migrate-never-automatic
**Success Criteria** (what must be TRUE):

  1. `engram migrate` is registered via `registerDestructive`: a bare invocation previews only (no writes), `--apply` is the explicit runtime choke point, and `--output json|text` matches the rest of the operator tier.
  2. `engram migrate status` reports a version-distribution histogram across the collection, not a single scalar version — a mixed-version collection mid-rollout is correctly represented, not misreported.
  3. `--apply` acts only on the intersection of the previewed, gate-passing set and a fresh re-derivation (reusing the shipped `spine-review purge` pattern) — a preview that does not match what apply does is treated as a defect, provable by test.
  4. `backfill-short-ids` is registered as the v0→v1 step, giving the mechanism a real first customer; the standalone command becomes a thin delegating alias (soft deprecation, per the `migrate-set-owner` precedent — never hard removal), and its prior apply-by-default is reconciled with `registerDestructive`'s preview-by-default via a `guides/upgrade.md` entry gated by a test.
  5. `engram migrate revert` previews by default like `--apply`, runs declared inverses in reverse order, and refuses the whole operation at the first irreversible step in the requested range rather than reverting partially — the refusal message names a collection snapshot as the recovery path.
  6. No migration ever runs automatically on server startup; at most, startup emits a non-blocking warning that pending migrations exist.

**Plans:** 4/4 plans complete

Plans:
**Wave 1**

- [x] 04-01-PLAN.md — Tracer: v0→v1 step + CheckAdditive pre-existing-key carve-out (H1) + CurrentVersion 0→1 with its full blast radius repaired and PA-10a item 3 discharged (C5-H2) + full-backlog DryRun + manifest-based intersection w/ single-pass PA-3-safe path (H7); carries the phase-wide Conformance Registry Impact ledger (wave 1)

**Wave 2**

- [x] 04-02-PLAN.md — Store MigrateStatus histogram w/ PER-VERSION future buckets (M4) + exported Store.PreviewRevert/RevertPlan (cycle-3 HIGH #1) as a whole-range zero-write preflight over the entire above-target set (cycle-3 HIGH #2) + Store.Revert w/ per-record chain selection (H5), pinned StepsFrom(to,from) arg order (H6), fixture injection (H4), DeletePayload-then-stamping-SetPayload partial-failure reconciliation (M3), corrected startup-warning predicate (H3) (wave 2)

**Wave 3**

- [x] 04-03-PLAN.md — CLI surface: generalize registerDestructive's ADMISSION gate to !ReadOnly + engram migrate/status/revert w/ in-apply-closure re-preview (H5/purge pattern, NOT package var) + shared migrateSweep{Preview,Apply}Run funcs the alias reuses (cycle-3 #7) + duration-taking migrateWithTimeout w/ per-leaf vars incl. read-only migrate status (H8/N3/C5-M6) + consumes 04-02's exported PreviewRevert and RevertRefusalError (M8/C5-M4) + store interface seam (M7) + leaf-only Use strings (M6) + TestDestructiveCommandsRequireApply re-derived from the NAMED union mutatingCommandNames() (M12/C4-H1) + pinned "mutating operator command" rule sentence via surfaces:gen (N5) + toolclass rows; **every operatorCommands()-keyed registry and a golden regeneration land in the SAME task as their command** — T2 for migrate/migrate status, T3 for migrate revert (wave 3)

**Wave 4**

- [x] 04-04-PLAN.md — backfill-short-ids as thin delegating alias over the SHARED sweep run funcs so its apply path has full manifest parity (cycle-3 #7) w/ PRESERVED-and-passed-through --timeout (H8/N3, NOT removed) + M10 discharged by composition of 04-01's real-Qdrant carve-out test and the alias call-sequence-equality test (C6-H7) + dead store code and both classification-table rows deleted + sibling conformance gates widened to the mutating set w/ RED-first proof (cycle-3 #6/N1) + upgrade.md entry documenting --dry-run removal only (NOT --timeout) + §5's stale --timeout group enumeration and cli.md's stale two-idiom paragraph repaired (C6-M11/C6-M5) + D-12 bidirectional gate (wave 4)

### Phase 5: Connect Record-State Parity

**Goal**: The Connect wire carries a record's full state — the same fields `store.Memory` already exposes — proven by an exhaustive mapping test, not a green `buf breaking` run mistaken for evidence a fourth time.
**Depends on**: 2026-08-12.01 Phase 4
**Requirements**: REQ-connect-record-state-parity, REQ-connect-parity-roundtrip-proof
**Success Criteria** (what must be TRUE):

  1. `proto`'s `Memory` message gains `superseded_by`, `supersedes`, `not_before`, `not_after`, `archived_at`, `schema_version`, `summary_model`, and `summary_egress_at` in ONE additive pass (field numbers 23–30), wired through `memoryToProto`; `buf breaking` stays clean.
  2. An exhaustive field-mapping round-trip test — not a hand-maintained field list — proves every wire-eligible `store.Memory` field is populated by `memoryToProto` and decodes losslessly; the test fails loudly (not silently) if a future field is added to `store.Memory` without a corresponding proto mapping, closing the gap that recurred across v0.8.x, v0.11.x, and v0.13.x.
  3. A sub-second `not_before`/`not_after` bound submitted through the write lane comes back outward-widened and IDENTICAL on both read lanes — MCP (`get_memory` / `full=true` recall) and Connect — proven by a boundary-second test. No read-path rounding code is added: the store's whole-second encoding makes read-side rounding a no-op by construction, and `memoryToProto` (`internal/server/connectapi.go`) records that. The rationale deliberately does NOT live on the `not_before`/`not_after` proto field comments: `internal/surfaces`' `checkProtoSurface` matches proto comments by bare field name across every message, so any comment on `Memory.not_before` binds the `schedule-window-at-least-one-bound` and `window-not-before-before-not-after` rules anchored on `ScheduleMemoryRequest.not_before` and turns `TestSurfaceConformanceProseFiles` red. Do not move it back (amended 2026-08-16 post-Phase-5; commit `44366849`, gotcha `3x25etde4f`).

**Plans:** 4/4 plans complete

Plans:
**Wave 1**

- [x] 05-01-PLAN.md — Tracer: the eight record-state fields at proto numbers 23–30, regenerated `gen/` trees, `memoryToProto` wiring, and one real Connect `GetMemory` round trip proving all eight reach the wire; plus the `SummaryEgressAt` comment repair (D-04)
- [x] 05-04-PLAN.md — Gap closure for G-05-9: a chromedp browser test in `internal/e2e` that boots the real binary, writes a record over Connect, and proves the embedded console bundle hydrates and renders that record — plus the fail-closed `ENGRAM_REQUIRE_BROWSER` gate on the existing CI test job (independent of 05-01…05-03; no shared files)

**Wave 2**

- [x] 05-02-PLAN.md — The exhaustive parity proof: one shared `json:"-"`-derived detector with a permanent negative fixture proving it can reject, plus a reflection auto-filled population fixture and inline decode-back comparison (D-05/D-06/D-07/D-08)
- [x] 05-03-PLAN.md — Boundary-second read-lane identity across MCP and Connect with no read-path rounding added (D-09), and the CLI `renderJSON` `schema_version: 0` anchor for Phase 2's D-10 promise (D-03)

### Phase 6: Typed Operator Renderer

**Goal**: Operator command output cannot let a json document silently carry more state than its text sentence states — enforced by construction, not merely detected by test.
**Depends on**: Nothing within this milestone (independent of the other phases; must complete before 2026-08-12.01 Phase 7)
**Requirements**: REQ-operator-renderer-typed
**Success Criteria** (what must be TRUE):

  1. `renderOperator`'s text and json output both derive from one shared ordered field set, so a json document cannot widen past what its text sentence states — field-set identity holds by construction, not by a test over hand-built rows.
  2. Every existing operator command's `--output json|text` behavior is unchanged (regression-free) after the refactor.
  3. Adding a new field to an operator report requires touching exactly one field-set declaration to appear correctly in both json and text output — there is no second call site to remember, which is what makes the six new record-state fields (Phase 5/7) safe to add afterward.

**Plans:** 7/7 plans complete

Plans:
**Wave 1**

- [x] 06-01-PLAN.md — Tracer: the view renderer (`operator_view.go`) walking the bytes `json.Marshal` produced, `renderOperator` rewired, `prune-expired` converted end to end, and the three-part identity gate with a committed negative case per part (wave 1)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 06-02-PLAN.md — D-03 decision checkpoint, then the `--output` flag help and docs-site CLI guide declaring `text` a view and `json` the contract (wave 2)
- [x] 06-03-PLAN.md — Flat group: `reindex`, `summarize-missing`, `migrate-set-owner`, `migrate-remap-owner` converted; byte-stability claims removed from their doc comments (wave 2)
- [x] 06-04-PLAN.md — Migrate family: `migrate`, `migrate revert`, the `backfill-short-ids` alias, and `migrate status` given a hand-declared CLI document replacing the `store` passthrough (wave 2)
- [x] 06-05-PLAN.md — Two-level group: `spine-review archive`/`restore` and `spine-review purge`; the re-run command becomes a document key (wave 2)
- [x] 06-06-PLAN.md — Remaining spine-review leaves: `scan` (gains the `scope` key its sentence always stated), `consolidate`, `verify` (wave 2)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 06-07-PLAN.md — Retire the hand-built parity gate, merge the five fixture groups under a cobra-tree-derived bidirectional coverage gate, and demonstrate Success Criterion 3 live (wave 3)

### Phase 7: Console & CLI State Surfacing

**Goal**: An operator can see a record's full state — archived, superseded, scheduled, schema version, and pending migrations — from the operator console UI and the CLI, not only by running `engram migrate status`.
**Depends on**: 2026-08-12.01 Phase 5, 2026-08-12.01 Phase 6
**Requirements**: REQ-console-record-state, REQ-cli-record-state, REQ-migration-state-visible
**Success Criteria** (what must be TRUE):

  1. The operator console UI renders a record's archived, superseded, and scheduled state — today it cannot render the v0.13.x archive tier at all.
  2. `engram search`/`list`/`get` surface the same state fields through the typed renderer, so the CLI and the console agree on what a record is.
  3. An operator can see pending-migration state (e.g., how many records sit behind the current schema version) through the console and CLI surfaces, not only by running `engram migrate status` directly.

**UI hint**: yes

Research flag: yes — the operator-UI soft-hidden-state conventions (archived/superseded/scheduled badges, precedence rules for compound state) are synthesized from general product convention, not a single citable spec; validate against real console usage rather than pre-guessing precedence for compound states.

**Plans:** 7/7 plans complete

Plans:
**Wave 1**

- [x] 07-01-PLAN.md — Tracer: the three recall-gate opt-in bools end to end on the List lane, plus the Go state-word vocabulary and D-12's always-present STATE column (wave 1)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 07-03-PLAN.md — Search-lane mirror, the 2-of-4 gate-scope proof, authorization orthogonality, and re-derivation of the two claims conditional gating invalidates (wave 2)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 07-02-PLAN.md — Console record-state rendering: MemoryRow badges with the dim-iff-past rule, MemoryDetail's schema chip and State section, and the TS state-word vocabulary (wave 3)
- [x] 07-05-PLAN.md — `engram get` over the ungated GetMemory RPC, plus D-11's structural headline sanitization fix (#505) (wave 3)

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 07-06-PLAN.md — MigrateStatus Connect RPC, one shared definition of "pending", the `engram migration-status` verb, and the advisory footer on search/list (wave 4)

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 07-04-PLAN.md — Console include toggles in ScopesSidebar, round-tripped through the URL and the query cache (wave 5)

**Wave 6** *(blocked on Wave 5 completion)*

- [x] 07-07-PLAN.md — The silent-at-zero migration banner in AppShell, on every console route (wave 6)

### Phase 8: Registry & Docs Tail

**Goal**: The shared scope-or-all-scopes guard is a registered, conformance-gated rule instead of a hand-rolled check, and the docs plus CLAUDE.md describe what this milestone actually shipped instead of what it superseded.
**Depends on**: 2026-08-12.01 Phase 4, 2026-08-12.01 Phase 7
**Requirements**: REQ-sweep-scope-rule-registered, REQ-docs-record-state, REQ-claude-md-migrations-convention
**Success Criteria** (what must be TRUE):

  1. `RuleSweepScopeOrAllScopesRequired` is a registered `surfaces.ConditionalRule` (not a hand-rolled `usageErrorf`), reused by both `summarize-missing` and `spine-review scan`, with its canonical sentence anchored and conformance-gated on every surface its fields resolve to.
  2. `reference/memory-record.md` and `reference/tools.md` document the full record state including `schema_version`, and a new operator-facing guide documents the migration mechanism end to end.
  3. CLAUDE.md's "Not used here: database migrations" line is revised to accurately describe what this milestone ships and its scope — schema-version-driven migrations only, deliberately not `migrate-remap-owner`/`summarize-missing`/`reindex` — so the normative doc no longer contradicts the code.

**Plans:** 6/6 plans complete

Plans:
**Wave 1**

- [x] 08-01-PLAN.md — Tracer: `RuleSweepScopeOrAllScopesRequired` registered with a `SurfaceFields` narrowing, anchored on the one derived surface, and composed by every sweep leaf in both its rejection and its `--all-scopes` help text, closing the gate at zero hand-rolled copies (wave 1)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 08-03-PLAN.md — The new `guides/migrate.md` operator procedure, and the schema-version release note corrected to link to it (wave 2)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 08-02-PLAN.md — `reference/memory-record.md` and `reference/tools.md` document every wire-visible field, the half-open window boundary, and `schema_version` (wave 3)

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 08-04-PLAN.md — CLAUDE.md audit: the migrations convention, the row-scoped catalog-derived command inventory with its tier split, and the record-state vocabulary (wave 4)

**Wave 5** *(gap closure, after 08-VERIFICATION.md; 08-05 and 08-06 share no files and are both wave 1 of the `--gaps-only` run, so they execute in parallel)*

- [x] 08-05-PLAN.md — CLAUDE.md accuracy repairs: every deprecated command marked in the Layout row (derived from the goldens, not a named pair), and the Archived-state paragraph's soft-hide surface set brought into gated agreement with its own Supersession paragraph and the store's `archived_at` gate sites (wave 1)
- [x] 08-06-PLAN.md — The sweep rule's doc comment claims only the enforcement that exists, and `TestNoHandRolledSweepScopeGuards` turns the phase's one-time "zero hand-rolled guards" check into a tree-walking both-directions gate, proven RED against a deliberately constructed defect (wave 1)

*Fully serialized on purpose. 08-01 owns every generated artifact and rewrites `docs-site/`,
`skill/`, `gen/` and the goldens; a concurrent docs plan makes its drift checks see the sibling's
uncommitted edits. 08-02 and 08-04 both state the record-state vocabulary and must agree, which a
shared wave cannot force. The plans are small; the serialization costs wall-clock, not scope.*

## Progress

**Execution Order:** 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 (v0.8.x, shipped) · 9 → 10 → 11 → 12 (v0.9.x, shipped 2026-07-10) · 13 → 14 (embedder track) · 15 → 16 → 17 → 18 → 19 (write-lane track, strict order) · 20 → 21 (independent) — v0.10.x shipped 2026-07-16 · 22 → 23 (Cedar foundation → service auth/tenancy, strict order) · 24 → 25 → 26 (capture trio + recall/config tail, strict order; 24 can start in parallel with 22–23) — v0.11.x shipped 2026-07-26 · v0.12.x: 1 → 2 (spine → CLI, strict order) · 3 · 4 · 5 · 6 (independent of the spine and of each other; ran in parallel once 1 was underway) · 7 (CLI cross-spine wiring, closed the audit seam between 2 and 3) — v0.12.x shipped 2026-08-02 · v0.13.x: 1 · 2 (parallelizable with each other) → 3 (needs 1 and 2 settled first) → 4 (authored in parallel with 3, full acceptance trails it) → 5 (last; needs 3's `verify` for the #355 fixture, reconciles each phase's own validation as it closes) — v0.13.x planned 2026-08-03 · 2026-08-12.01: 1 → 2 → 3 → 4 → 5 (needs 4) · 6 (independent, parallelizable with 3–5; must finish before 7) → 7 (needs 5 and 6) → 8 (needs 4 and 7) — 2026-08-12.01 roadmapped 2026-08-12

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
>
> **2026-08-12.01 is the first CalVer-labeled milestone; the same convention applies unchanged.**
> Phase numbering restarts at 1 again (this is now the active milestone), the structural checklist
> and `### Phase N:` headers above stay bare, and every **prose** reference elsewhere qualifies as
> `2026-08-12.01 Phase N` — the milestone label is a CalVer string (`YYYY-MM-DD.NN`), not a SemVer
> version, and is never reformatted to a `vX.Y` shape.

| Phase | Milestone | Requirements | Status | Completed |
|-------|-----------|--------------|--------|-----------|
| v0.8.x Phase 1: Authorization & Isolation | v0.8.x | 3/3 | Complete | shipped (v0.8.x) |
| v0.8.x Phase 2: Recall Semantics | v0.8.x | 3/3 | Complete | shipped (v0.8.x) |
| v0.8.x Phase 3: Memory Kinds & Tools | v0.8.x | 3/3 | Complete | shipped (v0.8.x) |
| v0.8.x Phase 4: Embedder | v0.8.x | 1/1 | Complete | shipped (v0.8.x) |
| v0.8.x Phase 5: Config & Transport | v0.8.x | 2/2 | Complete | shipped (v0.8.x) |
| v0.8.x Phase 6: Telemetry & Observability | v0.8.x | 2/2 | Complete | shipped (v0.8.x) |
| 7. Web UI, Docs Site & Distribution | v0.8.x | 9/9 | Complete   | 2026-08-20 |
| 8. Connect Auth Hardening | v0.8.x | 1/1 | Complete    | 2026-08-21 |
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
| 1. Shared Auth Chain & Connect Bearer Identity | v0.12.x | 4/4 | Complete    | 2026-08-13 |
| 2. Headless CLI Client | v0.12.x | 4/4 | Complete    | 2026-08-13 |
| 3. Cross-Spine Memory Recall | v0.12.x | 3/3 | Complete    | 2026-08-14 |
| 4. Diagnosability | v0.12.x | 4/4 | Complete   | 2026-08-15 |
| 5. Operator Config & Reindex Correctness | v0.12.x | 3/3 | Complete    | 2026-08-16 |
| 6. Rule Capture — Investigation & Fix | v0.12.x | 3/3 | Complete    | 2026-08-17 |
| 1. Interface Enforceability | v0.13.x | 9/9 | Complete | 2026-08-04 |
| 2. Interface Discoverability | v0.13.x | 6/6 | Complete | 2026-08-05 |
| 3. Spine Curation — Structural (CLI) | v0.13.x | 7/7 | Complete | 2026-08-07 |
| 03.1. Merge Supersession (INSERTED) | v0.13.x | 6/6 | Complete | 2026-08-11 |
| 4. Spine Curation — Semantic (Skill) | v0.13.x | 3/3 | Complete | 2026-08-11 |
| 5. Validation Debt Reconciliation | v0.13.x | 2/2 | Complete | 2026-08-12 |
| 1. Gate & CI Integrity | 2026-08-12.01 | 3/3 | Complete | 2026-08-13 |
| 2. Record Schema Versioning Foundation | 2026-08-12.01 | 4/4 | Complete | 2026-08-13 |
| 3. Migration Foundation (Registry, Invariants & Sweep) | 2026-08-12.01 | 5/5 | Complete | 2026-08-14 |
| 4. Migration CLI & First Customer | 2026-08-12.01 | 6/6 | Complete | 2026-08-15 |
| 5. Connect Record-State Parity | 2026-08-12.01 | 2/2 | Complete | 2026-08-15 |
| 6. Typed Operator Renderer | 2026-08-12.01 | 0/1 | Not started | - |
| 7. Console & CLI State Surfacing | 2026-08-12.01 | 0/3 | Not started | - |
| 8. Registry & Docs Tail | 2026-08-12.01 | 0/3 | Not started | - |

**v0.9.x — Recall Quality: ✅ shipped 2026-07-10 (PR #336) · 6/6 requirements · audit PASSED.**
**v0.10.x — Hardening & Write Lane: ✅ shipped 2026-07-16 · 9 phases (13–21) · 19/20 requirements (REQ-ci-renovate-spa-drift's live self-heal observation deferred, post-merge → #369) · audit tech_debt (9/9 Nyquist, 0 blockers).** Full detail: `milestones/v0.10.x-ROADMAP.md`.
**v0.11.x — Capture & Service Identity: ✅ shipped 2026-07-26 · 5 phases (22–26), 19 plans, 46 tasks · 11/11 requirements · audit PASSED (6/6 integration seams, 2/2 E2E flows, 0 blockers; Nyquist 5/5 validated — phases 24 and 26 reconciled 2026-07-26, 0 gaps).** Full detail: `milestones/v0.11.x-ROADMAP.md`.
**v0.12.x — Headless Reach & Diagnosability: ✅ shipped 2026-08-02 · 7 phases (1–7, first milestone on restarted numbering), 28 plans, 68 tasks · 21/21 requirements · audit `tech_debt` (5/5 integration seams, 2/2 E2E flows, 0 blockers; Nyquist not validated — 6 phases at `status: draft`, phase 2 has none, tracked as debt not gaps).** Full detail: `milestones/v0.12.x-ROADMAP.md`.
**v0.13.x — Curation & Self-Evidence: ✅ shipped 2026-08-12 · 6 phases (1–5 plus inserted 03.1), 33 plans, 99 tasks · 23/24 requirements (REQ-consent-adversarial-proof left unproven — cold-read run cap exhausted at 3, terminal verdict NOT-OBTAINED, non-result accepted by the user; WINDOWS.md id 3 open) · audit `tech_debt` (6/6 integration seams, 4/4 E2E flows, 0 blockers; Nyquist 5/6 COMPLIANT — phase 4 PARTIAL by design, its one pending row *is* the unproven requirement) · cleared the inherited v0.12.x Nyquist debt: all 6 phases now `status: validated`.** Full detail: `milestones/v0.13.x-ROADMAP.md`.
**2026-08-12.01 — Record State & Schema Evolution: 🚧 roadmapped 2026-08-12 · 8 phases (1–8) · 0/27 requirements.** First CalVer-labeled milestone. Phase 3 of the original 7-step research build order (11/27 requirements, 41% of the milestone) split into Phase 3 (migration foundation: registry, invariants, sweep) and Phase 4 (migration CLI: `engram migrate`, `backfill-short-ids` fold-in) to avoid one oversized phase. Full detail: this file (active milestone, not yet archived).

### Phase 9: Report pending in migrate status

**Goal:** Close audit items W2 and W3 (`.planning/2026-08-12.01-MILESTONE-AUDIT.md`). W2: `engram migrate status` omits `pending`, the value the milestone declared canonical — `migrateStatusReportDoc` (`cmd/engram/migrate_family.go:306-313`) carries no such field and `statusSummary` (`:348-358`) never computes it, so the offline operator verb is the one surface that does not report it. W3: `docs-site/src/content/docs/guides/migrate.md:279` already claims "the CLI's own text and json output derive the equivalent number from `absent` and `buckets` directly" — behavior the code does not have. One code fix closes both: add `pending` to the CLI report struct and text summary via the single existing `store.MigrateStatusResult.Pending()` definition (`internal/store/migrate_status.go:76`), never a re-derivation, then correct the doc sentence to match.
**Requirements**: REQ-migrate-status-histogram, REQ-docs-record-state (both already satisfied; this is debt closure, not new scope)
**Depends on:** Phase 8
**Plans:** 2/2 plans executed

Plans:
**Wave 1**

- [x] 09-01-PLAN.md — `pending` end-to-end: report struct field, converter call, unconditional text-headline clause, and the discriminating gate that proves it came from `Pending()` (W2)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 09-02-PLAN.md — rewrite `guides/migrate.md`'s `pending` row and land a self-tested zero-occurrence docs gate behind it (W3)

---

## Backlog

Unsequenced ideas parked outside the active phase sequence. Promote with `/gsd-review-backlog`.

### Phase 999.1: Vendored-SPA staleness gate that runs on feature branches (BACKLOG)

**Goal:** [Captured for future planning]
**Requirements:** TBD
**Plans:** 0 plans

Plans:

- [ ] TBD (promote with /gsd-review-backlog when ready)

**Context (captured 2026-08-21, during phase 07 UAT):**

Phase 07 changed `ui/` across three plans and never ran `task ui:build`, so the
go:embed'd console in `internal/webauth/static` was still the phase-05 build.
Every phase-07 console deliverable was green in vitest and absent from the
shipped binary. Nothing caught it for the whole phase:

- CI's `ui vendored-asset drift` job (`.github/workflows/ci.yaml:301`) is correct
  and required, but triggers on `pull_request` and `push: main` only — it cannot
  fire on an unmerged feature branch.

- `task` (= `lint` + `test`) has no drift check at all. `ui:build` is a generate
  verb with no verify counterpart.

- `internal/e2e/console_browser_test.go` does drive the real vendored bundle in
  headless Chrome (CI forces it via `ENGRAM_REQUIRE_BROWSER: "1"`), but asserts
  only hydration (an `<h1>` containing "operator console") and one seeded
  record's marker text — both true of a phase-05 bundle. Per its own package
  header it deliberately covers the cobra→mux→transport wiring seam, so it is
  structurally incapable of noticing the bundle predates the phase.

**Proposed shape:** a Go test in `internal/webauth` comparing
`git log -1 --format=%ct` over `ui/src ui/package.json ui/pnpm-lock.yaml` against
`internal/webauth/static`. No pnpm, no build, milliseconds. Living in
`go test ./...` gates both `task test` locally and the CI test job on every
branch push, while the existing `ui-drift` job stays the authoritative content
check at PR time. Verified against the phase-07 history: it reports STALE.

**Open questions for planning:**

- Rebase/cherry-pick can reorder commit timestamps — decide the tolerance and
  whether a same-commit vendor (equal timestamps) must pass.

- Whether a working-tree-dirty case should also fail, or only committed state.
- Whether the same shape should cover the `surfaces:gen` drift check, which has
  the identical local-verb/CI-verify split.

### Phase 999.2: Full-stack E2E for `engram migrate` (BACKLOG)

**Goal:** [Captured for future planning]
**Requirements:** TBD
**Plans:** 0 plans

Plans:

- [ ] TBD (promote with /gsd-review-backlog when ready)

**Context (captured 2026-08-22, milestone-audit item W1):**

`internal/e2e/` has zero migrate coverage — `rg -ln 'migrate|Migrate' internal/e2e/`
returns nothing; the package holds `boot_test.go`, `cli_exitcode_test.go`,
`console_browser_test.go`, `harness_test.go`, `spine_review_test.go` only. The
flagship operator verb of milestone 2026-08-12.01 is proven by two disjoint
suites that never compose:

- CLI layer: `cmd/engram/migrate_family_test.go` runs against
  `fakeMigrateFamilyStore` (`:29`, explicit "no live Qdrant dial" comment). Proves
  CLI→store-*interface* wiring.

- Store layer: `internal/store/migrate_test.go` (e.g. `TestMigrateV0ToV1MintEndToEnd`)
  exercises `store.Migrate` directly against a real Qdrant, bypassing cobra entirely.

Both halves are strong; the join between them is inferred, never witnessed. The
integration checker independently re-derived this as the milestone's one PARTIAL
seam (P4→P2), and found no additional gap.

**Proposed shape:** one `internal/e2e` test driving the built binary's
`engram migrate status` → `--apply` → `revert` against the existing live-Qdrant
harness, asserting preview/apply parity on a seeded v0 collection.

**Open questions for planning:**

- Whether `migrate-remap-owner` (also untested full-stack, but not
  schema-version-driven) belongs in the same harness or its own item.

- Runtime cost against the existing `internal/e2e` Qdrant fixture.

**Affected requirements (already satisfied; this is depth, not coverage):**
REQ-migrate-command, REQ-migrate-preview-apply-parity, REQ-migrate-revert.

### Phase 999.3: Narrow CLAUDE.md's "every surface" record-state claim (BACKLOG)

**Goal:** [Captured for future planning]
**Requirements:** TBD
**Plans:** 0 plans

Plans:

- [ ] TBD (promote with /gsd-review-backlog when ready)

**Context (captured 2026-08-22, milestone-audit item W4):**

CLAUDE.md:180 states "Every surface renders a record's derived state as up to four
words, in canonical order". That overstates by one lane. Two surfaces derive the
words — `cmd/engram/memory_state.go` (CLI) and `ui/src/lib/memorystate.ts` (console).
The MCP lane emits raw fields and leaves derivation to the caller: `internal/server`
renders zero state words, its only `archived|superseded|expired|scheduled` hits being
comments and jsonschema argument descriptions.

Small, documentation-only, and non-blocking — but the sentence is the kind of
convention claim a future agent will act on. Either narrow it to name the two
rendering surfaces, or state the MCP lane's raw-field contract explicitly.

**Affected requirement (already satisfied):** REQ-claude-md-migrations-convention.

### Phase 999.4: Unify `schema_version` proto typing (BACKLOG)

**Goal:** [Captured for future planning]
**Requirements:** TBD
**Plans:** 0 plans

Plans:

- [ ] TBD (promote with /gsd-review-backlog when ready)

**Context (captured 2026-08-22, milestone-audit cross-cutting item):**

Schema version is typed three ways across one proto file and the Go side:

- `Memory.schema_version` — `optional uint32` (`proto/engram/v1/engram.proto:52`)
- `SchemaVersionBucket.version` — `int32` (`:186`)
- `MigrateStatusResponse.current_version` — `int32` (`:203`)
- Go-side `migrate.Version` — `int`

No live break today: every value in play is small and non-negative, so the
signed/unsigned split never manifests. It is a trap for a future signed sentinel
(a `-1` meaning "unset" or "unknown" would round-trip through `uint32` as
4294967295), and a wire-compat decision once chosen — changing a field's type
after release is not free.

**Open questions for planning:**

- Whether unification is worth a proto change at all, or whether the right
  outcome is a comment pinning "non-negative, never a sentinel" as the contract.
