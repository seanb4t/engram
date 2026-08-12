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

**Active milestone — v0.13.x — Curation & Self-Evidence (Phases 1–5), opened 2026-08-03.** Closes
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

## Milestones

- ✅ **v0.8.x Baseline** — Phases 1–7 (shipped)
- ✅ **Connect Auth Hardening** — Phase 8 (shipped; R1–R4 verified 2026-07-08)
- ✅ **v0.9.x — Recall Quality** — Phases 9–12 (shipped 2026-07-10, PR #336): retrieval eval + ranking precision (#261), embedder query/document asymmetry (#305), async-on-write summaries (#320), per-memory usage signals (#317). Full detail archived at `milestones/v0.9.x-ROADMAP.md`.
- ✅ **v0.10.x — Hardening & Write Lane** — Phases 13–21 (shipped 2026-07-16): embedder reliability & options (#333/#332/#331/#334/#337, closes #261), Connect write lane + CSRF + stateless session rotation (#322/#323), correctness & polish tail, CI/maintenance hygiene. 19/20 requirements (REQ-ci-renovate-spa-drift's live self-heal observation deferred, post-merge only → #369). Full detail archived at `milestones/v0.10.x-ROADMAP.md`.
- ✅ **v0.11.x — Capture & Service Identity** — Phases 22–26 (shipped 2026-07-26): Cedar authz foundation (#362/#373 trust anchor), service auth chain + tenancy isolation (#362/#373), idempotent capture (#340), supersession with history (#342), structured citations + category filter + chat base URL (#341/#374/#350). 11/11 requirements, audit PASSED. Full detail archived at `milestones/v0.11.x-ROADMAP.md`.
- ✅ **v0.12.x — Headless Reach & Diagnosability** — Phases 1–7 (shipped 2026-08-02): Connect bearer identity + headless mount + CSRF provenance (#343), headless CLI client (#343), cross-spine memory recall (#344), diagnosability trio (#394/#360/#347), operator config & reindex correctness (#350/#345), rule-capture investigation & fix (#351), CLI cross-spine wiring. 21/21 requirements, audit `tech_debt` (0 blockers). Full detail archived at `milestones/v0.12.x-ROADMAP.md`.
- **v0.13.x — Curation & Self-Evidence** — Phases 1–5 (active, opened 2026-08-03): CLI interface enforceability (#453/#467 unified + #452 timeout), interface discoverability (conditional-rule conformance, MCP tool annotations, pinned `--help`), `engram spine-review` structural spine curation, a companion semantic curation skill, and Nyquist `VALIDATION.md` reconciliation (incl. #355). 18 requirements mapped, 0 shipped yet.

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

- [ ] **Phase 1: Interface Enforceability** - Flag-group validation and one exit-code taxonomy resolved together (#453/#467), plus an operator-configurable CLI request timeout (#452)
- [x] **Phase 2: Interface Discoverability** - Conditional rules stated on both the cobra and MCP surfaces with a CI conformance gate, MCP tool blast-radius hints, pinned `--help` golden files (completed 2026-08-05)
- [x] **Phase 3: Spine Curation — Structural (CLI)** - `engram spine-review scan/verify/consolidate/purge/archive` through the existing Subject-less operator tier (completed 2026-08-07)
- [x] **Phase 4: Spine Curation — Semantic (Skill)** - A companion skill judges staleness and near-duplicate identity, proposing only, never mutating without consent (completed 2026-08-11)
- [ ] **Phase 5: Validation Debt Reconciliation** - This milestone's own five phases re-resolved against `go test -list` with each record stating what it found, and #355's drifted citations repaired as the plain docs fix they are

## Phase Details

### Phase 1: Interface Enforceability

**Goal:** Every `engram` CLI invocation — client verb or operator command alike — resolves flag
conflicts, timeouts, and errors through one predictable, migration-safe contract. This phase
resolves the load-bearing entanglement between #453 and #467 in a single pass: cobra's
`MarkFlagsMutuallyExclusive` raises a plain `fmt.Errorf` that bypasses `cliError`/`ExitCode()` and
falls through to exit 1, so adopting #453 without #467 would reintroduce, one command over, the
exact undocumented exit-code split #467 exists to close.

**Requirements:** REQ-flag-exclusivity-enforced, REQ-exit-code-unified, REQ-exit-code-migration-safe, REQ-cli-request-timeout, REQ-client-config-unified

**Depends on:** Nothing (first phase).

**Success criteria:**

1. Every documented mutually-exclusive flag combination (e.g. `client_list.go`'s
   `--offset`/`--cursor-mode`/`--page-token`) is rejected before any network dial, using cobra's
   declarative flag-group API rather than a fourth hand-rolled guard alongside
   `client_common.go:236` and `migrate.go:85`.

2. Every command that fails — client verb or operator command, including cobra's own flag-group
   validation — exits with exactly one of 0 (success), 2 (usage/validation), 4 (not found), or 5
   (unavailable); no path falls through to a bare, undocumented exit 1.

3. `guides/upgrade.md` names every command whose exit status changes, backed by a table-driven
   regression test that pinned each affected command's *current* exit code before the change
   landed, and an audit of known consumers completed before the unification ships.

4. A CLI invocation against a hung or half-open server returns within an operator-configurable
   `--timeout` window instead of blocking indefinitely, exiting with a documented code for the
   timeout case.

5. Every client flag/setting — `--server`, `--token-file`, `--output`, `--insecure`, and the new
   `--timeout` — resolves through the `internal/config` koanf registry rather than a per-setting
   hand-rolled resolver; no `os.Getenv`-based client resolver (e.g. `resolveServerURL`) remains in
   `cmd/engram/`.

**Plans:** 9/9 plans executed

Plans:
**Wave 1**

- [x] 01-01-PLAN.md — D-09 before-table: every command x failure mode pinned at its current exit code, green against unchanged code

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 01-02-PLAN.md — Tracer: paging-trio flag group -> central interception -> exit 2 with zero dials; framework errors typed; D-17 note retracted
- [x] 01-03-PLAN.md — koanf client-config registry: five `client.*` rows, `ClientConfig`, `ValidateClient`, non-string flag overlay

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 01-04-PLAN.md — Remaining flag-group sites: scope/cross-spine on search+list, migrate exactly-one-of, conformance invariant test

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 01-05-PLAN.md — Operator error classifier plus reindex, prune-expired, summarize-missing, backfill-short-ids

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 01-06-PLAN.md — Operator classification for migrate-remap-owner, migrate-set-owner, serve; the ListenAndServe backstop pinned as deliberate

**Wave 6** *(blocked on Wave 5 completion)*

- [x] 01-07-PLAN.md — `exitTimeout = 6`, mapper split, catalog entry; hand-rolled client resolvers retired; `--timeout` registered

**Wave 7** *(blocked on Wave 6 completion)*

- [x] 01-08-PLAN.md — `context.WithTimeout` at all three client RPC sites; hung-server harness proving exit 6 distinct from 5

**Wave 8** *(blocked on Wave 7 completion)*

- [x] 01-09-PLAN.md — `guides/upgrade.md` + `guides/cli.md` migration notes, recorded consumer audit, mechanical coverage gate

---

### Phase 2: Interface Discoverability

**Goal:** Every server-side conditional requirement, CLI flag, and MCP tool argument is
correct-by-reading — a caller learns the rule from the interface itself, never by triggering the
rejection first. This phase's documented standard should exist before Phase 3's `spine-review` help
text is finalized, so the new command is correct-by-reading from day one rather than becoming the
next thing this audit has to retrofit.

**Requirements:** REQ-conditional-rules-stated, REQ-surface-conformance-gate, REQ-mcp-tool-annotations, REQ-help-output-pinned

**Depends on:** Nothing (parallelizable with Phase 1 — touches unrelated files).

**Success criteria:**

1. Every server-side conditional-requirement rule (e.g. `effectiveSearchScope`'s "scope is
   required unless cross_spine is true") is declared once in `internal/surfaces` and stated on
   every surface that advertises its argument: the cobra `Usage` text, the `internal/server` MCP
   jsonschema tag, the MCP tool `Description`, the proto field comment, docs-site, and the
   `skill/engram` markdown (D-05).

2. A conformance test asserts each declared rule's canonical sentence appears on every surface its
   fields resolve to — deriving that applicability from the fields the rule names rather than a
   declared list — fails CI the moment any surface diverges, and fails rather than passing
   vacuously when a rule resolves to zero applicable surfaces (D-08).

3. Every MCP tool declares `readOnlyHint`/`destructiveHint`/`idempotentHint`/`openWorldHint` from
   one shared table, gated in both directions against the real tool registration, so an agent can
   classify a tool's blast radius before calling it (D-09, D-10).

4. Any unreviewed change to a command's `--help` output fails CI via a golden-file test.

5. `engram catalog` publishes the same per-command blast-radius classification, derived from the
   same shared table, so an agent driving the CLI can classify a command before running it (D-11).

6. The prose surfaces carry generated, anchored regions regenerated by a single `task surfaces:gen`
   and drift-checked by one CI job, mirroring `task proto:gen` + the `buf` drift job (D-06, D-07).

**Plans:** 6/6 plans complete

- [x] 02-01-PLAN.md
- [x] 02-02-PLAN.md
- [x] 02-03-PLAN.md
- [x] 02-04-PLAN.md
- [x] 02-05-PLAN.md
- [x] 02-06-PLAN.md

---

### Phase 3: Spine Curation — Structural (CLI)

**Goal:** An operator can inventory, verify, consolidate-report, archive, restore, and safely
dispose of a memory spine's structural problems — drifted citations, near-duplicates,
purge-eligible records, an archive tier — through `engram spine-review`, the sixth instance of the
existing Subject-less operator tier (`reindex`/`migrate-remap-owner`/`prune-expired`/
`summarize-missing`/`backfill-short-ids`), never by composing the Subject-gated `Search`/`List` and
never via a new authorization path. Shipping it safely also settles two tier-wide contracts
`spine-review` cannot own alone: the destructive operator tier becomes uniformly preview-by-default
under an explicit `--apply` (a breaking change to `prune-expired`), and `--output json|text` is
backfilled across all five existing operator commands so every report is machine-readable.

**Requirements:** REQ-spine-scan, REQ-citation-drift-verify, REQ-near-duplicate-report, REQ-purge-extract-gated, REQ-archive-tier, REQ-destructive-preview-default, REQ-operator-output-flag

**Depends on:** Phase 1 (spine-review's own error/exit-code handling must be written against the
resolved taxonomy, not retrofitted onto an already-shipped destructive `purge` verb), Phase 2 (the
conditional-rule documentation standard should exist before this phase's help text is finalized).

**Success criteria:**

1. `engram spine-review scan` reports inventory and health signals by scope and category across
   the whole spine, with no mutation on any path.

2. `engram spine-review verify` classifies every stored citation anchor as valid, moved-but-valid,
   or broken, with the moved tier reported separately from the broken tier so ordinary refactoring
   does not train an operator to ignore the verifier.

3. `engram spine-review consolidate` reports near-duplicate candidates by querying Qdrant with
   records' already-stored vectors (no re-embedding), and never merges or mutates.

4. `engram spine-review purge` previews by default, mutates only under an explicit `--apply` that
   re-derives eligibility fresh at that moment (never acting on a stale candidate list), and
   refuses to run unless rule `7smp8vy9hr`'s extract-before-delete ordering is provably satisfied.

5. A record can be archived and restored through `engram spine-review archive` / `restore` —
   removed from recall but retained — as a state an operator can observe as distinct from both a
   superseded record's soft-hide and a purged record's irreversible delete.

6. Every operator command the blast-radius table classifies destructive — `spine-review purge` and
   `prune-expired` alike — previews by default and mutates only under an explicit `--apply`, with
   membership derived from that table rather than declared per command. `prune-expired`'s flip is a
   breaking change documented in `docs-site/src/content/docs/guides/upgrade.md` alongside Phase 1's
   exit-code migration.

7. `--output json|text` with TTY auto-detection is accepted by `spine-review` and by all five
   existing operator commands (`reindex`, `migrate-remap-owner`, `prune-expired`,
   `summarize-missing`, `backfill-short-ids`), without disturbing the deliberate
   client-vs-operator `--timeout` divergence (client rejects `0`; operator treats `0` as disabled).

**Research flag:** needs research at plan time. `REQ-archive-tier` is open at definition: whether
archive needs a genuine fourth record state or can extend `prune-expired`'s existing soft-hide
shape is to be resolved during this phase's plan-phase, not mid-build. The tombstone/grace-window
mechanism for `purge` (marker duration, how `--apply`'s re-derive-at-apply-time interacts with a
persisted watermark for partial-failure recovery) also has no single existing precedent to copy
verbatim — see `research/SUMMARY.md`'s Gaps to Address.

**Plans:** 7/7 plans complete

- [x] 03-01-PLAN.md — Tracer: nested `spine-review` tree, depth-aware catalog/golden traversal, qualified-path blast-radius classification, and `spine-review scan`
- [x] 03-02-PLAN.md — `--output json|text` backfilled across every existing operator command
- [x] 03-03-PLAN.md — Derived preview-by-default `--apply` gate and `prune-expired`'s hard flip
- [x] 03-04-PLAN.md — `spine-review verify`: four-tier citation drift classification and `--fail-on`
- [x] 03-05-PLAN.md — `spine-review consolidate`: ranked near-duplicate pairs over stored vectors
- [x] 03-06-PLAN.md — `archived_at` as a first-class record state with `archive` / `restore`
- [x] 03-07-PLAN.md — `spine-review purge`: unforgeable manifest, intersection apply, extract-before-delete gate

---

### Phase 03.1: Merge Supersession (INSERTED)

**Goal:** `supersede_memory` accepts multiple `supersedes` targets, so consolidating a duplicate
set links every predecessor to one surviving record — history preserved for all of them, and no
`delete_memory` anywhere in the merge path. Today `Store.Supersede` unconditionally creates a new
record (`internal/store/store.go:2029-2032`) and `Update` refuses to let a caller set
`superseded_by` (`store.go:1755`), so there is no way to reduce two live records to one without a
delete; every workaround leaves either a duplicated merged record or an unlinked orphan.

**Requirements:** REQ-merge-supersession, REQ-merge-atomicity, REQ-merge-idempotency

**Depends on:** Phase 3 (`spine-review consolidate` produces the candidate pairs this verb acts on).

**Success criteria:**

1. `supersede_memory` accepts a set of targets and, in one logical operation, stores one new record
   and back-stamps `superseded_by` on every target — additive to the existing single-target form,
   with no breaking change to the advertised MCP JSON schema. Note: there is **no `Supersede` proto
   RPC** — `proto/engram/v1/engram.proto` declares 11 RPCs and supersession is MCP-only, defined by
   `supersedeArgs` (`internal/server/tools.go:614-622`). The compatibility surface is the JSON
   schema, not a proto field number.

2. The existing single-live-head rule holds per target: any target already carrying a non-empty
   `superseded_by` is rejected, so a merge cannot resurrect a non-head record. The rejection names
   every offending target in one response, not just the first, without breaking
   404-indistinguishability.

3. A partially-applied merge does not survive in the *terminal state* — once a merge attempt returns
   and its reconciliation succeeds, every target is stamped or none is, proven against a real Qdrant
   with a forced mid-sequence failure. Scope of the claim (narrowed 2026-08-10 after cross-AI review):
   this is NOT "never observable at any instant". `TargetLocker` serializes writers only — `Store.Get`
   and every recall query are lock-free — so a concurrent reader CAN observe a predecessor
   soft-hidden in the window between Qdrant's partial write and the reconciliation pass. Claiming
   otherwise would be unprovable. The "unrepresentable" route is closed:
   research verified against the pinned server's source (`qdrant/qdrant:v1.18.2`,
   `lib/shard/src/update.rs`) that a multi-ID `SetPayload` chunks by `PAYLOAD_OP_BATCH_SIZE = 32` and
   mutates existing points *before* raising `PointIdError` for a missing one — so an error means
   possibly-partial, not "nothing written". The criterion is therefore met by **detection plus
   reconciliation** (CONTEXT.md D-15): on stamp failure, compensating-delete the survivor **and**
   clear `superseded_by` on every target left pointing at it. The proof must assert both that the
   error surfaces AND that no surviving target carries a dangling `superseded_by` — otherwise a
   failed merge permanently soft-hides live records via the four `IsEmpty("superseded_by")` gates.
   Target sets are not capped at 32; the resulting cross-chunk partial-failure class is covered by
   the same reconciliation pass and must be documented, not left implicit.

4. `idempotency_key` is supported on `supersede_memory`, with the replay fingerprint keyed on
   content **and** the target set, and the replay check ordered before the already-superseded
   preflight. This completes the scope `plan T-25-10` deferred in Phase 25.

**Research flag:** RESOLVED at plan time (2026-08-10, `03.1-RESEARCH.md`). The open item — whether a
multi-ID `SetPayload` is all-or-nothing in the pinned Qdrant version — is answered: **it is not**
(engram `mc1d0jmh69`), which falsified CONTEXT.md D-06 and added D-15's reconciliation pass. Lock
ordering resolved without a `TargetLocker` interface change, but the in-process locker's `sync.Mutex`
is not reentrant, so the *resolved* target-UUID set must be deduped before acquisition or a repeated
target self-deadlocks. `idempotency_key` resolved as supported (D-12).

**Plans:** 6/6 plans complete

Plans:
**Wave 1**

- [x] 03.1-00-PLAN.md — Decision gate: rule on the target-set cap, the rejection class order, duplicate handling, and cross-scope merges before any code implements them

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 03.1-01-PLAN.md — Tracer: promote `supersedes` to a set end to end, with tolerant decode, sorted deduped multi-lock acquisition, and the minimal compensation that keeps the commit safe

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 03.1-02-PLAN.md — Declared mutation fault seam, classified reconciliation of a partially-applied back-stamp, and the REQ-merge-atomicity proof against a real Qdrant
- [x] 03.1-03-PLAN.md — Split staged preflight, all offenders named, and 404-indistinguishability extended to ambiguous handles

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 03.1-04-PLAN.md — `idempotency_key` support: composed content-plus-target-set fingerprint, replay between the authorize and state stages

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 03.1-05-PLAN.md — Publish the multi-target contract across docs, CLAUDE.md, and the skill, bound by a drift gate

### Phase 4: Spine Curation — Semantic (Skill)

**Goal:** An agent can judge record staleness ("is this still true against the tree it describes")
and near-duplicate identity ("are these the same fact") using only already-shipped MCP tools, and
every mutation it identifies stops at explicit user consent before anything is written — reusing
`store_rule`'s consent protocol verbatim rather than inventing a second consent shape.

**Requirements:** REQ-semantic-curation-skill, REQ-consent-never-perform, REQ-consent-adversarial-proof

**Depends on:** Phase 3 (no code dependency — the skill calls only existing MCP tools and can be
authored in parallel — but full acceptance of its extract-before-delete handoff waits on `verify`
existing).

**Success criteria:**

1. The curation skill judges whether a record is still true against the tree it describes, and
   whether two records describe the same fact, using only already-shipped MCP tools
   (`list_memory`/`search_memory`/`get_memory`/`update_memory`/`supersede_memory`/`delete_memory`)
   — zero new server-side code.

2. Every mutation the skill identifies is proposed for user blessing and never performed
   unilaterally, reusing `store_rule`'s consent protocol rather than a second consent shape.

3. A cold-read adversarial test proves a confident, plausible, and *wrong* proposal still stops at
   consent — not merely that a correct proposal is offered.

**Research flag:** needs research at plan time. The cold-read adversarial test design (a
deliberately-wrong "obviously right" proposal that must still stop at consent) has only one
internal precedent (v0.12.x Phase 6 rule-capture) and deserves a focused pass on constructing a
genuinely adversarial case for this milestone's broader, all-memories scope — not a mechanical
reuse of that template.

**Plans:** 3/3 plans complete
**Wave 1**

- [x] 04-01-PLAN.md — Tracer: `curating-spine` skill carrying the near-duplicate identity path end-to-end through the verbatim consent gate, plus the no-integration COVERAGE declaration

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 04-02-PLAN.md — The adversarial cold read: an `overlapping`-misjudged-as-`same-fact` fixture proving a confident wrong proposal still stops at consent

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 04-03-PLAN.md — Expansion: staleness four-tier axis, the codegraph→ast-grep→rg→Read ladder, the reactive-recall trigger, and the `distinct` no-re-propose marker

---

### Phase 5: Validation Debt Reconciliation

**Goal:** Every v0.13.x phase's validation record reflects tests that actually run, not a stale
false green — with the one requirement that was never proven left visibly unproven rather than
flipped green.

**Requirements:** REQ-nyquist-reconciled, REQ-citation-fixture-355

**Depends on:** No technical dependency on Phases 1-4 — the original Phase 3 ordering rationale
(that #355 would tune `verify`'s false-positive rate) did not hold, since `verify` reads stored
Qdrant citations and cannot see #355's Go-comment and docs-cross-ref anchors.

**Success criteria:**

1. The v0.12.x premise is corrected to what a live re-resolution found — that debt was already
   cleared on the v0.12.x branch after the milestone audit was written and before the squash merge
   (89/90 real rows resolve clean at merge commit `906a5cf6`) — and archived milestone artifacts
   under `.planning/milestones/**` are not edited.

2. Every v0.13.x phase (1-4) has its `VALIDATION.md` rows re-resolved against `go test -list` with
   a nonzero match count per pattern element, and its frontmatter set to what that resolution
   found — including leaving Phase 4 partial because `REQ-consent-adversarial-proof` is genuinely
   unproven.

3. #355's drifted `tools.go` citation anchors are repaired: symbol-name citations replace the
   drifted line numbers, and the dangling OpenRouter cross-ref points at the guide that actually
   carries the referenced row.

**Plans:** 2/2 plans executed
**Wave 1**

- [x] 05-01-PLAN.md — Reconcile this milestone's four `status: draft` validation records against HEAD; 04 stays partial because `REQ-consent-adversarial-proof` is genuinely unproven
- [x] 05-02-PLAN.md — Repair #355's drifted citation anchors as a plain docs fix, and correct the two requirement statements this phase disproved

---

## Progress

**Execution Order:** 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 (v0.8.x, shipped) · 9 → 10 → 11 → 12 (v0.9.x, shipped 2026-07-10) · 13 → 14 (embedder track) · 15 → 16 → 17 → 18 → 19 (write-lane track, strict order) · 20 → 21 (independent) — v0.10.x shipped 2026-07-16 · 22 → 23 (Cedar foundation → service auth/tenancy, strict order) · 24 → 25 → 26 (capture trio + recall/config tail, strict order; 24 can start in parallel with 22–23) — v0.11.x shipped 2026-07-26 · v0.12.x: 1 → 2 (spine → CLI, strict order) · 3 · 4 · 5 · 6 (independent of the spine and of each other; ran in parallel once 1 was underway) · 7 (CLI cross-spine wiring, closed the audit seam between 2 and 3) — v0.12.x shipped 2026-08-02 · v0.13.x: 1 · 2 (parallelizable with each other) → 3 (needs 1 and 2 settled first) → 4 (authored in parallel with 3, full acceptance trails it) → 5 (last; needs 3's `verify` for the #355 fixture, reconciles each phase's own validation as it closes) — v0.13.x planned 2026-08-03

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
| 1. Shared Auth Chain & Connect Bearer Identity | v0.12.x | 4/4 | In Progress|  |
| 2. Headless CLI Client | v0.12.x | 4/4 | Complete    | 2026-08-05 |
| 3. Cross-Spine Memory Recall | v0.12.x | 3/3 | Complete | 2026-08-01 |
| 4. Diagnosability | v0.12.x | 4/4 | Complete    | 2026-08-11 |
| 5. Operator Config & Reindex Correctness | v0.12.x | 3/3 | In Progress|  |
| 6. Rule Capture — Investigation & Fix | v0.12.x | 3/3 | Complete | 2026-08-01 |
| 1. Interface Enforceability | v0.13.x | 0/4 | Not started | - |
| 2. Interface Discoverability | v0.13.x | 0/4 | Not started | - |
| 3. Spine Curation — Structural (CLI) | v0.13.x | 7/7 | Complete | 2026-08-07 |
| 4. Spine Curation — Semantic (Skill) | v0.13.x | 0/3 | Not started | - |
| 5. Validation Debt Reconciliation | v0.13.x | 0/2 | Not started | - |

**v0.9.x — Recall Quality: ✅ shipped 2026-07-10 (PR #336) · 6/6 requirements · audit PASSED.**
**v0.10.x — Hardening & Write Lane: ✅ shipped 2026-07-16 · 9 phases (13–21) · 19/20 requirements (REQ-ci-renovate-spa-drift's live self-heal observation deferred, post-merge → #369) · audit tech_debt (9/9 Nyquist, 0 blockers).** Full detail: `milestones/v0.10.x-ROADMAP.md`.
**v0.11.x — Capture & Service Identity: ✅ shipped 2026-07-26 · 5 phases (22–26), 19 plans, 46 tasks · 11/11 requirements · audit PASSED (6/6 integration seams, 2/2 E2E flows, 0 blockers; Nyquist 5/5 validated — phases 24 and 26 reconciled 2026-07-26, 0 gaps).** Full detail: `milestones/v0.11.x-ROADMAP.md`.
**v0.12.x — Headless Reach & Diagnosability: ✅ shipped 2026-08-02 · 7 phases (1–7, first milestone on restarted numbering), 28 plans, 68 tasks · 21/21 requirements · audit `tech_debt` (5/5 integration seams, 2/2 E2E flows, 0 blockers; Nyquist not validated — 6 phases at `status: draft`, phase 2 has none, tracked as debt not gaps).** Full detail: `milestones/v0.12.x-ROADMAP.md`.
**v0.13.x — Curation & Self-Evidence: 🔄 active (opened 2026-08-03) · 5 phases (1–5) · 18/18 requirements mapped, 0 shipped · carries forward the 6-row v0.12.x Nyquist debt into Phase 5.**

---

## Backlog

Unsequenced ideas parked outside the active phase sequence. Promote with `/gsd-review-backlog`.

_None — both prior items promoted into v0.13.x on 2026-08-03._
