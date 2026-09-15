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

**2026-08-23.01 — Distribution & Agent Bootstrap (Phases 1–6), roadmapped 2026-08-23, shipped
2026-09-12 as v0.16.0.** engram
becomes installable in one command and self-configuring across every agent runtime it targets:
`brew install engram`, then `engram setup` detects what's on the machine, shows what it would
write, and wires it up. Research was bimodal — HIGH confidence on distribution mechanics, MEDIUM on
the runtime config surfaces that make up most of the new code — but a live post-synthesis
verification retired the round's two named highest-risk items before roadmapping: `codex mcp add`
(codex-cli 0.148.0) and `opencode mcp add` (opencode 1.18.15) both exist on the machine's installed
CLIs, so every v1 runtime is a shell-out writer and engram parses no third-party config format at
all — the "surgical marker-bounded text editing" design the research body proposed for Codex's TOML
and opencode's JSONC is unneeded, and opencode's live V1/V2 schema self-contradiction is moot
because engram never reads that file. Cursor was deferred to v2 at scoping precisely because it
would have been the one config-file writer among four shell-out writers, and its own CLI surface
was unverifiable on the researching machine. The build order follows the confidence gradient:
`engram version --output json` and the Homebrew cask (Phase 1) are near-execution-ready and independent of
the setup-command track, so they run first and in parallel with it; `internal/setup`'s core
abstraction, the `Runtime` interface, and `cmd/engram/setup.go` land together in Phase 2 because
`cmd/engram/catalog.go` panics on any cobra command missing a row in
`internal/surfaces/toolclass.go` — adding `setup` to the tree and its own classification row cannot
be sequenced across two phases. Runtime registration (Phase 3), skills distribution (Phase 4), and
the `/engram-setup` delegation gate (Phase 5) each depend on the phase before it, since delegation's
generated-equivalence proof needs the full runtime registry and skill content settled to generate
complete prose against. Install documentation (Phase 6) is sequenced last so it documents final
shipped behavior rather than a moving target.

## Milestones

- ✅ **v0.8.x Baseline** — Phases 1–7 (shipped)
- ✅ **Connect Auth Hardening** — Phase 8 (shipped; R1–R4 verified 2026-07-08)
- ✅ **v0.9.x — Recall Quality** — Phases 9–12 (shipped 2026-07-10, PR #336): retrieval eval + ranking precision (#261), embedder query/document asymmetry (#305), async-on-write summaries (#320), per-memory usage signals (#317). Full detail archived at `milestones/v0.9.x-ROADMAP.md`.
- ✅ **v0.10.x — Hardening & Write Lane** — Phases 13–21 (shipped 2026-07-16): embedder reliability & options (#333/#332/#331/#334/#337, closes #261), Connect write lane + CSRF + stateless session rotation (#322/#323), correctness & polish tail, CI/maintenance hygiene. 19/20 requirements (REQ-ci-renovate-spa-drift's live self-heal observation deferred, post-merge only → #369). Full detail archived at `milestones/v0.10.x-ROADMAP.md`.
- ✅ **v0.11.x — Capture & Service Identity** — Phases 22–26 (shipped 2026-07-26): Cedar authz foundation (#362/#373 trust anchor), service auth chain + tenancy isolation (#362/#373), idempotent capture (#340), supersession with history (#342), structured citations + category filter + chat base URL (#341/#374/#350). 11/11 requirements, audit PASSED. Full detail archived at `milestones/v0.11.x-ROADMAP.md`.
- ✅ **v0.12.x — Headless Reach & Diagnosability** — Phases 1–7 (shipped 2026-08-02): Connect bearer identity + headless mount + CSRF provenance (#343), headless CLI client (#343), cross-spine memory recall (#344), diagnosability trio (#394/#360/#347), operator config & reindex correctness (#350/#345), rule-capture investigation & fix (#351), CLI cross-spine wiring. 21/21 requirements, audit `tech_debt` (0 blockers). Full detail archived at `milestones/v0.12.x-ROADMAP.md`.
- ✅ **v0.13.x — Curation & Self-Evidence** — Phases 1–5 plus inserted 03.1 (shipped 2026-08-12): CLI interface enforceability (#453/#467 unified + #452 timeout), interface discoverability (conditional-rule conformance, MCP tool annotations, pinned `--help`), `engram spine-review` structural spine curation, multi-target merge supersession, a companion semantic curation skill, and Nyquist `VALIDATION.md` reconciliation (incl. #355). 23/24 requirements, audit `tech_debt` (0 blockers). Full detail archived at `milestones/v0.13.x-ROADMAP.md`.
- ✅ **2026-08-12.01 — Record State & Schema Evolution** — Phases 1–9 (shipped 2026-08-22): gate & CI integrity first (#479/#497), a `schema_version` payload discriminator (absent-safe, wire-visible, never recall-gated), a versioned `internal/migrate` step registry + `Store.Migrate` sweep with mandatory additive-only/reversibility declarations, `engram migrate` via `registerDestructive` folding in `backfill-short-ids` as its first step, Connect record-state parity (#482) proven by an exhaustive round-trip test, a typed operator renderer (#481), console + CLI state surfacing, and the `RuleSweepScopeOrAllScopesRequired` registry/docs tail (#480). 27/27 requirements, audit `tech_debt` (0 blockers). Full detail archived at `milestones/2026-08-12.01-ROADMAP.md`.
- ✅ **2026-08-23.01 — Distribution & Agent Bootstrap** — Phases 1–6 (shipped 2026-09-12 as v0.16.0): `engram version --output json` + a credential-verified, backfill-safe Homebrew cask (#514/#516), `engram setup` (detect → preview → `--apply`, three-way exit taxonomy), runtime registration through `claude`/`codex`/`opencode mcp add` plus a `generic` portable-config fallback across all four auth modes, the five curation skills embedded in the binary and installed natively (AGENTS.md index for Codex; preservation gap #559 closed by 04-05), `/engram-setup` delegation with a generated equivalence gate, and canonical Install / Agent Setup guides. 25/25 requirements, audit `tech_debt` (0 blockers, Nyquist 6/6). Full detail archived at `milestones/2026-08-23.01-ROADMAP.md`.
- 🚧 **2026-09-13.01 — Setup v2** — Phases 1–5 (in progress, roadmapped 2026-09-13): plugin-first delivery for Claude Code/Codex, a custom-header auth shape for gateway registrations (LiteLLM's `x-litellm-api-key`), three-way drift detection with an apply-time preserve gate, man pages, and the `osRun` deadline fix (#560).

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

<details>
<summary>✅ 2026-08-12.01 — Record State & Schema Evolution (Phases 1–9) — SHIPPED 2026-08-22</summary>

Full detail archived at [`milestones/2026-08-12.01-ROADMAP.md`](milestones/2026-08-12.01-ROADMAP.md).
The `### Phase N:` detail sections were moved out of this file at milestone close so a bare
heading resolves to the **active** milestone, not this shipped one — the same reason the v0.8.x
block records.

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
- [x] **Phase 9: Report pending in migrate status** - `engram migrate status` reports `pending` from the single `MigrateStatusResult.Pending()` definition, and `guides/migrate.md` stops claiming a CLI derivation that never existed (closes audit W2/W3) (completed 2026-08-22)

</details>

<details>
<summary>✅ 2026-08-23.01 — Distribution & Agent Bootstrap (Phases 1–6) — SHIPPED 2026-09-12</summary>

Full detail archived at [`milestones/2026-08-23.01-ROADMAP.md`](milestones/2026-08-23.01-ROADMAP.md).
The `### Phase N:` detail sections were moved out of this file at milestone close so a bare
heading resolves to the **active** milestone, not this shipped one — the same reason the v0.8.x
block records.

**Milestone Goal (2026-08-23.01):** engram is installable in one command and self-configuring
across every agent runtime it targets — `brew install engram`, then `engram setup` detects what's
on the machine, shows what it would write, and wires it up.

- [x] **Phase 1: Version & Homebrew Distribution** - `engram version --output json` plus a published, credential-verified, recoverable Homebrew cask (completed 2026-08-25)
- [x] **Phase 2: Setup Command Core** - `engram setup` detects runtimes, previews by default, declares its full outcome vocabulary, and is fully scriptable without a TTY (completed 2026-08-30)
- [x] **Phase 3: Runtime Registration** - `engram setup --apply` registers engram with Claude Code, Codex, and opencode via their own CLIs, converging idempotently, plus a generic-MCP fallback, across every auth mode (completed 2026-09-09)
- [x] **Phase 4: Skills Distribution** - The five curation skills reach every runtime, native format where one exists, AGENTS.md fallback otherwise (audit reopened preservation gap #559 on 2026-09-12) (completed 2026-09-12)
- [x] **Phase 5: Slash Command Delegation** - `/engram-setup` delegates to the binary when present, keeps its prose fallback first-class otherwise, with a generated (not hand-checked) equivalence gate (completed 2026-09-12)
- [x] **Phase 6: Install Documentation** - docs-site documents how to get the binary and how to run `engram setup` (completed 2026-09-12)

</details>

- [x] **Phase 1: Executor Correctness & Man Pages** - A deadline-killed runtime subprocess reports a timeout instead of a clean failure, and the binary generates and ships its own man pages (completed 2026-09-13)
- [x] **Phase 2: Custom Auth Headers** - A gateway registration (e.g. LiteLLM's `x-litellm-api-key`) is expressible on every runtime that can render it, with existing auth modes unchanged (completed 2026-09-14)
- [ ] **Phase 3: Plugin-First Delivery** - Claude Code and Codex receive skills, hooks, and `/engram-setup` through their own plugin system instead of a plain skills copy
- [ ] **Phase 4: Drift Detection (Read-Only)** - Preview classifies an existing registration as identical, reproducible, or preserved by comparing against what setup would actually write
- [ ] **Phase 5: Apply-Time Preserve Gate & Documentation** - `--apply` never rewrites a registration it cannot reproduce, and shipped docs are brought current with a post-release observation

## Phase Details

### Phase 1: Executor Correctness & Man Pages

**Goal:** `engram`'s subprocess executor correctly classifies a deadline-killed runtime CLI as a timeout instead of a clean failure, and the released binary can generate and ship its own man pages alongside the completions it already ships. Both fixes are small, independently shippable, and touch no other feature in this milestone — the `osRun` fix is a three-line diff checking `ctx.Err()` before the `errors.As(*exec.ExitError)` branch (`internal/setup/environment.go`), and `engram man` mirrors the already-shipped hidden `completion` command via `cobra/doc`'s `GenManTree` (`cobra/doc` promoted from an indirect to a direct dependency; zero new Go dependencies). Both are standard, fully-specified patterns needing no additional research at plan time.

**Requirements:** REQ-osrun-deadline-error, REQ-manpages-generated, REQ-manpages-cask-installed

**Depends on:** Nothing (first phase).

**Success criteria:**

1. When a runtime subprocess (`claude`, `codex`, `opencode`) is killed because its command context deadline expired, `engram setup`'s result row for that runtime reports a timeout — the executor's timeout path engages — rather than a clean nonzero exit with no error.
2. An operator can run the hidden `engram man <dir>` command to generate one man page per command from the live cobra tree, and re-running it produces byte-identical output (no auto-generated timestamp).
3. The Homebrew cask installs the generated man pages on `post_install` and removes exactly those paths on `post_uninstall`, symmetric with the shipped completions hooks, with ordering and absence pinned by `releaseconfig_test.go`.

**Plans:** 2/2 plans complete

Plans:
**Wave 1**

- [x] 01-01-PLAN.md — `osRun` reports `ctx.Err()` before unwrapping `*exec.ExitError`; `runSeam` names the 20s timeout in the result row (#560, REQ-osrun-deadline-error)
- [x] 01-02-PLAN.md — Hidden `engram man <dir>` generating byte-stable man pages via `cobra/doc`, plus the symmetric cask install/uninstall hook step pinned by `releaseconfig_test.go` (REQ-manpages-generated, REQ-manpages-cask-installed)

---

### Phase 2: Custom Auth Headers

**Goal:** A user can name the auth header a registration uses (for example LiteLLM's `x-litellm-api-key`) alongside its env-var-reference value, for every runtime that can express it — Claude Code, opencode, and `generic` — each rendered in that runtime's own CLI syntax and authored per-runtime file, never through a shared cross-runtime formatter (reusing one previously reintroduced opencode's colon-space regression). Codex declines a non-`Authorization` header name explicitly, the same way `oauth-client` is already declined for opencode, and never gains a hand-written `[mcp_servers.engram.http_headers]` TOML edit. Every existing `--auth oauth|oauth-client|bearer|none` mode keeps its shipped argv, help text, and generated prose unchanged when no header name is given. Live-verify opencode's `--header KEY=VALUE` repeatability at implementation time — its CLI could not be exercised this research session (Gatekeeper/AMFI kill), so its facts are carried forward from the prior milestone's live verification, not re-confirmed.

**Requirements:** REQ-header-name-parameter, REQ-header-value-env-ref-only, REQ-header-bearer-unchanged, REQ-header-codex-declined, REQ-header-documented

**Depends on:** Nothing (independent of Phase 1 — touches unrelated files; sequenced after it purely as build order).

**Success criteria:**

1. A user can register engram with Claude Code, opencode, or `generic`, naming a custom header alongside an env-var reference for its value, rendered in each runtime's own CLI syntax (`"Name: value"` for Claude Code, `Name=value` for opencode).
2. No literal secret value ever appears on argv, in written config, in preview text, `--output json`, or in logs — only an env-var reference in the runtime's own reference syntax.
3. Running `engram setup` with any of `--auth oauth|oauth-client|bearer|none` and no header name produces argv, help text, and generated `/engram-setup` prose identical to what shipped in `2026-08-23.01`.
4. Naming a header other than `Authorization` for Codex produces a `failed` row whose reason names the capability gap, and setup never writes `[mcp_servers.engram.http_headers]` by hand.
5. `engram setup --help`, `guides/agent-setup.md`, and the regenerated `/engram-setup` prose show the gateway header shape with its env-reference form, including the Codex limitation.

**Plans:** 4/4 plans complete

Plans:
**Wave 1**

- [x] 02-01-PLAN.md — `Options.Headers []HeaderSpec` + `ErrHeaderUnsupported` + `sortedHeaders` in `internal/setup`; claude-code renders sorted `--header 'NAME: ${ENVVAR}'` pairs on its single add action in every mode; codex declines any header before its auth switch with a reason naming header(s), gap, and remedy (D-01, D-04, D-06, D-08, D-09, D-10; REQ-header-name-parameter, REQ-header-value-env-ref-only, REQ-header-codex-declined)

**Wave 2**

- [x] 02-02-PLAN.md — opencode renders `--header 'NAME={env:ENVVAR}'` pairs in its own file; generic carries `${ENVVAR}` extras in its existing `headers` object marshaled Authorization-first then sorted, with zero-header Config byte-identical; `TestNoSecretInArgs` positive control (D-04, D-05, D-08; REQ-header-name-parameter, REQ-header-value-env-ref-only, REQ-header-bearer-unchanged)

**Wave 3**

- [x] 02-03-PLAN.md — `--header NAME=ENVVAR` flag + `ENGRAM_HEADERS` default (no registry row), four exact usage errors at the CLI boundary with no right-hand-side echo, flat-scalar `headers` row facet, help paragraph + fifth example, regenerated help/catalog goldens (D-01, D-02, D-03, D-06, D-07, D-08, D-09; all five REQ-header-* IDs)

**Wave 4**

- [x] 02-04-PLAN.md — fifth `bearer+header` setupgen case regenerating the `/engram-setup` tables, header-aware `TestSetupGeneratedInvocations`, `/engram-setup` prose and `guides/agent-setup.md` gateway-header section, `ENGRAM_HEADERS`, and the Codex limitation (D-01, D-02, D-04, D-07, D-08, D-09; REQ-header-documented, REQ-header-bearer-unchanged, REQ-header-codex-declined, REQ-header-name-parameter)

---

### Phase 3: Plugin-First Delivery

**Goal:** Under `--apply`, a plugin-capable Claude Code or Codex receives engram's skills, hooks, and `/engram-setup` command through its own plugin system — installing or updating only engram's own marketplace plugin, never a foreign one — while a runtime without a working `plugin` CLI, and opencode/`generic` regardless, keep the native skills-copy path, mutually exclusive per runtime per run so `curating-memory` never appears twice. Live-verify at implementation time, before locking any plugin `Action.Args`: whether `codex plugin` exists as a stable, scriptable surface (no confirmed `--json` on any subcommand; the best current source is an in-flight, unmerged upstream PR, not a released feature); whether `claude plugin install`/`codex plugin` refuse-on-already-installed or silently overwrite; and whether `skill/engram/.codex-plugin/plugin.json` needs the richer `interface` block for a CLI-only install. Also surface explicitly, before building: whether a first-run plugin/marketplace install needs consent beyond the existing `--apply` gate (PROJECT.md is silent on this).

**Requirements:** REQ-plugin-capability-detection, REQ-plugin-install-or-update, REQ-plugin-three-way-state, REQ-plugin-skips-skills-copy, REQ-plugin-facet-reported, REQ-codex-plugin-manifest, REQ-plugin-setupgen-regenerated

**Depends on:** Phase 2 (sequenced after only to reduce merge risk in the shared runtime files `claudecode.go`/`codex.go` — no functional dependency on headers).

**Success criteria:**

1. Setup detects whether a present Claude Code or Codex binary exposes a working `plugin` CLI, distinct from binary-on-PATH detection, and a runtime without that capability falls back to the native skills copy with a reported reason rather than a failed row.
2. Under `--apply`, a plugin-capable runtime gets engram's own marketplace added when absent, the plugin installed when absent, and updated when outdated — doing nothing when already current — while preview shows the exact plugin-CLI argv beforehand and no additional consent flag exists beyond `--apply`.
3. Plugin state is reported as one of absent / installed-but-outdated / installed-and-current, and a plugin-delivered runtime never also receives a native skills-copy install or an `AGENTS.md` index block.
4. A result row shows plugin delivery as its own facet in both text and JSON output, alongside registration and skills, so a `wrote` registration next to a `failed` plugin install stays visible.
5. `skill/engram/.codex-plugin/plugin.json` exists, is release-please-synced like `.claude-plugin/plugin.json` with a drift gate keeping their identity fields equal, and `/engram-setup`'s generated prose reflects the new plugin actions and outcomes in the same change that introduces them, keeping the existing `setupgen` CI drift gate green.

**Plans:** 3/4 plans executed

Plans:
**Wave 1**

- [x] 03-01-PLAN.md — `internal/setup` plugin lane: `PluginState` (absent/outdated/current/unavailable), the exported `PluginRuntime` optional interface implemented by claude-code and codex in their own files (probes, JSON parse, coarse marketplace parse, authored argv incl. `-y --scope user` for Claude Code, HTTPS Git URL marketplace add + remove-then-add for Codex), a stdlib SemVer-core comparator, and `PluginPreview`/`PluginApply` running one `plugin list --json` probe for capability+state, the `marketplace list` probe, and the decided actions through `runSeam` — probe failure is `unavailable`, never failed (D-01, D-02, D-03, D-04, D-05, D-06, D-10, D-11, D-12; REQ-plugin-capability-detection, REQ-plugin-install-or-update, REQ-plugin-three-way-state)
- [x] 03-02-PLAN.md — `internal/skills`: `Environment.Lstat` seam + read-only `DetectPresence` (count of present skills, symlink vs copy, `AGENTS.md` index block via `scanBlock`) — never a write or delete (D-08, D-09; REQ-plugin-skips-skills-copy)

**Wave 2**

- [x] 03-03-PLAN.md — `cmd/engram/setup.go`: `setupVersion` seam (`resolvedVersion`), seven flat-scalar `plugin*` row fields + `skills_native`, `setupApplyPluginFacet` folding a failed plugin beside a wrote registration to `exitPartial`, `Delivered()` routing that never calls `skills.Install` for a plugin-delivered runtime and reports leftovers instead, D-12 native fallback, plugin-first `--help` paragraph, view fixtures, regenerated help/catalog goldens, `TestSetupGeneratedInvocations` pinning the new probe (D-01, D-05, D-06, D-07, D-08, D-09, D-11, D-12; REQ-plugin-facet-reported, REQ-plugin-skips-skills-copy, REQ-plugin-capability-detection, REQ-plugin-install-or-update, REQ-plugin-three-way-state)
- [ ] 03-04-PLAN.md — `skill/engram/.codex-plugin/plugin.json` (minimal `$schema`/`name`/`version`/`description`), `release-please-config.json` `$.version` sync entry, `TestPluginManifestIdentityMatches` drift gate, pytest privacy guard; `setupgen.Render(planFn, pluginFn)` appending the `### Claude Code plugin delivery (--apply)` table from claude-code's real `PluginActions`, regenerated `/engram-setup` region in the same commit with plugin-first prose, `--check-setup` green (D-01, D-02, D-04, D-06; REQ-codex-plugin-manifest, REQ-plugin-setupgen-regenerated)

---

### Phase 4: Drift Detection (Read-Only)

**Goal:** Preview compares a runtime's actual existing engram registration — URL, auth mode, header set — against what setup would write, classifying it as exactly one of three states (identical / reproducible-difference / non-reproducible-so-`preserved`) rather than a read-probe heuristic, and names which facet differs when it does. Before any comparison or rendering code is trusted, live-verify what each runtime's read verb actually prints for a header whose value is NOT a bare `${VAR}`/`{env:VAR}` reference: both `claude mcp get` and `codex mcp get --json` were confirmed this research session to echo literal header values in cleartext for registrations engram did not write, but the non-reference-value echo case itself could not be tested under this session's no-mutation constraint — this is a blocking prerequisite for trusting the redaction path, not an assumption to carry forward. opencode's registration is explicitly NOT parsed (its `mcp list` output is a box-drawing table) and stays a documented coarse comparison; re-verify opencode's live CLI behavior at implementation time (unexecutable this research session).

**Requirements:** REQ-drift-observed-registration, REQ-drift-three-way, REQ-drift-preserved-outcome, REQ-drift-facet-naming, REQ-drift-redaction

**Depends on:** Phase 2 (hard dependency — the comparison surface needs the header vocabulary Phase 2 introduces before anything can be compared against it).

**Success criteria:**

1. Preview reads a runtime's existing engram registration through the runtime's own read verb (Codex's `mcp get --json` as structured input, a bounded text scan for Claude Code) and normalizes it to the same shape the Plan authors, without reading any third-party config file directly.
2. A registration is classified as exactly one of `already-correct`, `would-write`, or `preserved` — the latter two are never collapsed into one bucket — with unit coverage of all three states per runtime.
3. `preserved` is a first-class outcome in both text and JSON output, with a reason naming what setup cannot reproduce, reflected consistently in aggregation and exit codes, and documented in `guides/agent-setup.md`'s results table, including Codex's whole-entry (preserve-or-overwrite, no partial merge) semantics.
4. A `would-write` row for an existing registration names which facet differs — URL, auth mode, header name, or value reference — rather than a bare "differs".
5. Header values obtained from any runtime read-probe are redacted unconditionally before comparison storage, rendering, JSON output, or logging, proven against a fixture whose probe output carries a literal value.

**Plans:** TBD

---

### Phase 5: Apply-Time Preserve Gate & Documentation

**Goal:** `--apply` consults the same drift classification before writing and performs zero write actions against a `preserved` registration — including never running Claude Code's destructive `mcp remove` step — closing the root cause of the 2026-09-10 overwrite incident (gotcha `ryr82bf2s2`) that this milestone exists to prevent; a reproducible rewrite on Claude Code states upfront that an OAuth-authenticated registration will need to log in again. This is genuinely new executor surface (the shipped executor never branches on probe content today), so it stays its own phase rather than folding into Phase 4's read-only comparison. `guides/install.md`, `guides/agent-setup.md`, and `guides/plugin.md` are brought current with everything this milestone shipped — plugin-first delivery, the header shape, the `preserved` outcome and apply gate, and man pages — closed out with a post-release live-observation note (the `2026-08-23.01` D-10 pattern) rather than checked off from code alone.

**Requirements:** REQ-apply-preserve-gate, REQ-apply-rewrite-consequence, REQ-docs-setup-v2

**Depends on:** Phase 4 (hard dependency — the apply-time gate consults the same classification Phase 4 introduces).

**Success criteria:**

1. Running `--apply` (not only preview) against a pre-seeded, unreproducible registration performs zero registration-write actions for that runtime, while still applying its skills/plugin actions — proven by a fixture test that runs `--apply` itself, not only preview.
2. Claude Code's `mcp remove` step never runs against a `preserved` registration.
3. When a reproducible difference on Claude Code requires remove-then-add of an existing OAuth-authenticated registration, both preview and apply state that the rewrite will require logging in again before it runs.
4. `guides/install.md`, `guides/agent-setup.md`, and `guides/plugin.md` describe the shipped plugin-first delivery, header shape, `preserved` outcome and apply gate, and man pages — checked off only after a post-release live observation is recorded, not from code alone.

**Plans:** TBD

---

## Progress

**Execution Order:** 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 (v0.8.x, shipped) · 9 → 10 → 11 → 12 (v0.9.x, shipped 2026-07-10) · 13 → 14 (embedder track) · 15 → 16 → 17 → 18 → 19 (write-lane track, strict order) · 20 → 21 (independent) — v0.10.x shipped 2026-07-16 · 22 → 23 (Cedar foundation → service auth/tenancy, strict order) · 24 → 25 → 26 (capture trio + recall/config tail, strict order; 24 can start in parallel with 22–23) — v0.11.x shipped 2026-07-26 · v0.12.x: 1 → 2 (spine → CLI, strict order) · 3 · 4 · 5 · 6 (independent of the spine and of each other; ran in parallel once 1 was underway) · 7 (CLI cross-spine wiring, closed the audit seam between 2 and 3) — v0.12.x shipped 2026-08-02 · v0.13.x: 1 · 2 (parallelizable with each other) → 3 (needs 1 and 2 settled first) → 4 (authored in parallel with 3, full acceptance trails it) → 5 (last; needs 3's `verify` for the #355 fixture, reconciles each phase's own validation as it closes) — v0.13.x planned 2026-08-03 · 2026-08-12.01: 1 → 2 → 3 → 4 → 5 (needs 4) · 6 (independent, parallelizable with 3–5; must finish before 7) → 7 (needs 5 and 6) → 8 (needs 4 and 7) — 2026-08-12.01 roadmapped 2026-08-12 · 2026-08-23.01: 1 · 2 (independent, parallelizable with 1) → 3 → 4 → 5 (strict order) → 6 (needs 1 and 5) — 2026-08-23.01 roadmapped 2026-08-23 · 2026-09-13.01: 1 → 2 → 3 → 4 → 5 (1 is an independent quick win, run first; 2 → 4 → 5 is a hard-dependency chain — headers before drift, drift-read before the apply-time preserve gate; 3 is sequenced after 2 only to reduce shared-file merge risk, not a functional dependency) — 2026-09-13.01 roadmapped 2026-09-13

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
| 1. Shared Auth Chain & Connect Bearer Identity | v0.12.x | 4/4 | In Progress|  |
| 2. Headless CLI Client | v0.12.x | 4/4 | In Progress|  |
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
| 6. Typed Operator Renderer | 2026-08-12.01 | 1/1 | Complete | 2026-08-17 |
| 7. Console & CLI State Surfacing | 2026-08-12.01 | 3/3 | Complete | 2026-08-20 |
| 8. Registry & Docs Tail | 2026-08-12.01 | 3/3 | Complete | 2026-08-22 |
| 9. Report pending in migrate status | 2026-08-12.01 | 2/2 | Complete | 2026-08-22 |
| 1. Version & Homebrew Distribution | 2026-08-23.01 | 3/3 | Complete | 2026-08-25 |
| 2. Setup Command Core | 2026-08-23.01 | 3/3 | Complete | 2026-08-30 |
| 3. Runtime Registration | 2026-08-23.01 | 5/5 | Complete | 2026-09-09 |
| 4. Skills Distribution | 2026-08-23.01 | 3/3 | Complete | 2026-09-12 |
| 5. Slash Command Delegation | 2026-08-23.01 | 3/3 | Complete | 2026-09-12 |
| 6. Install Documentation | 2026-08-23.01 | 2/2 | Complete; released docs live | 2026-09-12 |
| 1. Executor Correctness & Man Pages | 2026-09-13.01 | 3/3 | Complete | 2026-09-13 |
| 2. Custom Auth Headers | 2026-09-13.01 | 5/5 | Complete | 2026-09-14 |
| 3. Plugin-First Delivery | 2026-09-13.01 | 0/7 | Pending |  |
| 4. Drift Detection (Read-Only) | 2026-09-13.01 | 0/5 | Pending |  |
| 5. Apply-Time Preserve Gate & Documentation | 2026-09-13.01 | 0/3 | Pending |  |

**v0.9.x — Recall Quality: ✅ shipped 2026-07-10 (PR #336) · 6/6 requirements · audit PASSED.**
**v0.10.x — Hardening & Write Lane: ✅ shipped 2026-07-16 · 9 phases (13–21) · 19/20 requirements (REQ-ci-renovate-spa-drift's live self-heal observation deferred, post-merge → #369) · audit tech_debt (9/9 Nyquist, 0 blockers).** Full detail: `milestones/v0.10.x-ROADMAP.md`.
**v0.11.x — Capture & Service Identity: ✅ shipped 2026-07-26 · 5 phases (22–26), 19 plans, 46 tasks · 11/11 requirements · audit PASSED (6/6 integration seams, 2/2 E2E flows, 0 blockers; Nyquist 5/5 validated — phases 24 and 26 reconciled 2026-07-26, 0 gaps).** Full detail: `milestones/v0.11.x-ROADMAP.md`.
**v0.12.x — Headless Reach & Diagnosability: ✅ shipped 2026-08-02 · 7 phases (1–7, first milestone on restarted numbering), 28 plans, 68 tasks · 21/21 requirements · audit `tech_debt` (5/5 integration seams, 2/2 E2E flows, 0 blockers; Nyquist not validated — 6 phases at `status: draft`, phase 2 has none, tracked as debt not gaps).** Full detail: `milestones/v0.12.x-ROADMAP.md`.
**v0.13.x — Curation & Self-Evidence: ✅ shipped 2026-08-12 · 6 phases (1–5 plus inserted 03.1), 33 plans, 99 tasks · 23/24 requirements (REQ-consent-adversarial-proof left unproven — cold-read run cap exhausted at 3, terminal verdict NOT-OBTAINED, non-result accepted by the user; WINDOWS.md id 3 open) · audit `tech_debt` (6/6 integration seams, 4/4 E2E flows, 0 blockers; Nyquist 5/6 COMPLIANT — phase 4 PARTIAL by design, its one pending row *is* the unproven requirement) · cleared the inherited v0.12.x Nyquist debt: all 6 phases now `status: validated`.** Full detail: `milestones/v0.13.x-ROADMAP.md`.
**2026-08-12.01 — Record State & Schema Evolution: ✅ shipped 2026-08-22 · 9 phases (1–9), 46 plans, 121 tasks · 27/27 requirements · audit `tech_debt` (5/5 integration seams, 3/5 E2E flows, 0 blockers; Nyquist 9/9 COMPLIANT) · closeout `override_closeout` — 8 open artifacts acknowledged at close, see STATE.md Deferred Items.** First CalVer-labeled milestone. Full detail: `milestones/2026-08-12.01-ROADMAP.md`.

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

### Phase 999.5: `engram setup` should install via plugins for harnesses that support them (claude, codex), not plain installs (BACKLOG)

**Goal:** [Captured for future planning]
**Requirements:** TBD
**Plans:** 0 plans

Plans:

- [ ] TBD (promote with /gsd-review-backlog when ready)

**Context (captured 2026-09-13, from a live `engram setup` review on the maintainer's machine):**

v0.16.x `engram setup --apply` treats every native runtime as a "plain install":
it copies the embedded skills into the runtime's user-scope skills directory
(`~/.claude/skills/`, `~/.agents/skills/` + a managed `~/.codex/AGENTS.md`
block, `~/.config/opencode/skills/`) and registers the MCP server with the
runtime's `mcp add`. Claude Code and Codex both have a first-class plugin
mechanism that already carries these skills, the session hooks, and the
`/engram-setup` command — and on a machine where the plugin is installed,
`setup` has no awareness of it:

- Claude Code: skills arrive via the `engram@engram` marketplace plugin
  (`~/.claude/plugins/marketplaces/engram/skill/engram/skills/…`) and
  `~/.claude/skills/` is empty. `--apply` would write a second copy there,
  so `curating-memory` and `engram:curating-memory` both surface. The binary
  path also never installs the plugin's session hooks
  (`guides/agent-setup.md` calls this out as a known gap).
- Codex: `~/.agents/skills/*` are symlinks into that same plugin checkout, so
  they track plugin updates. `setup` replaces a symlink target with a static
  copy pinned to the binary's embedded version
  (`internal/skills/install.go` — `os.Rename` over the target replaces the
  link).
- `internal/skills/` and `internal/setup/` contain no marketplace / plugin
  detection at all.

**Open questions for planning:**

- Which runtimes count as "supports plugins" for v1 (Claude Code marketplace,
  Codex — confirm Codex's plugin surface), and what does opencode get?
- Plugin-first, or plugin-when-detected? I.e. should `setup` *install* the
  plugin (`claude plugin marketplace add` + `claude plugin install`), or only
  detect an installed plugin and skip the plain skill install?
- Where does MCP registration live once the plugin is the delivery vehicle —
  still `mcp add`, or the plugin's own MCP declaration?
- Interaction with `/engram-setup` delegation: the plugin's command delegates to
  the binary, which would then install the plugin — define the fixed point.
- Related: `setup` has no custom-header auth mode, so a gateway registration
  (e.g. LiteLLM `x-litellm-api-key`) cannot be expressed and `--apply` replaces
  it. Same milestone or separate backlog item?

### Phase 999.6: `engram setup` needs to support custom headers / auth keys (allow the maintainer's current gateway setup) (BACKLOG)

**Goal:** [Captured for future planning]
**Requirements:** TBD
**Plans:** 0 plans

Plans:

- [ ] TBD (promote with /gsd-review-backlog when ready)

**Context (captured 2026-09-13, from a live `engram setup` review on the maintainer's machine):**

`--auth` accepts exactly `oauth | oauth-client | bearer | none`
(`cmd/engram/setup.go:628`). `bearer` is hard-wired to the `Authorization`
header with an `ENGRAM_TOKEN` reference, expressed per runtime as:

- claude-code: `--header "Authorization: Bearer ${ENGRAM_TOKEN}"` (`internal/setup/claudecode.go:162`)
- codex: `--bearer-token-env-var ENGRAM_TOKEN` (`internal/setup/codex.go:110`)
- opencode: `--header "Authorization=Bearer {env:ENGRAM_TOKEN}"` (`internal/setup/opencode.go:131`)
- generic: `"Authorization": "Bearer ${ENGRAM_TOKEN}"` (`internal/setup/generic.go:141-145`)

The maintainer's real registrations (all three runtimes) reach engram through a
LiteLLM gateway and authenticate with a custom header
(`x-litellm-api-key: Bearer <key>`), not `Authorization`. No `--auth` mode can
express that, so `setup --apply` cannot *reproduce* the working config — it
replaces it (claude-code is `mcp remove` + `mcp add`) and breaks the
connection. This is the same class of incident as the 2026-09-10 overwrite
(engram gotcha `ryr82bf2s2`), now with a root cause: the auth model is too
narrow, not just the verification discipline.

Preview is also affected: with the wrong `--auth`, the read probe reports the
existing registration as drift to be replaced rather than as already-correct.

**Open questions for planning:**

- Shape of the option: a repeatable `--header NAME=VALUE-REF` (value names an
  env var, never a literal), or `--auth header --header-name X --token-env Y`?
  Must keep the "no secret in argv/config" property that `bearer` already has.
- Per-runtime feasibility: claude-code and opencode take arbitrary `--header`;
  codex's `mcp add` exposes `--bearer-token-env-var` only — does
  `[mcp_servers.<name>.http_headers]` in `config.toml` need a direct write path,
  and does that violate "register via the runtime's own CLI"?
- Should `already-correct` detection compare the full header set so a preview
  against an existing custom-header registration is a no-op instead of a
  replacement?
- Overlap with Phase 999.5 (plugin-based install): if the plugin carries the
  MCP declaration, the header option must exist there too.
