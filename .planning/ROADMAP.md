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

**2026-08-23.01 — Distribution & Agent Bootstrap (Phases 1–6), roadmapped 2026-08-23.** engram
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
`engram version --json` and the Homebrew cask (Phase 1) are near-execution-ready and independent of
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

- [ ] **Phase 1: Version & Homebrew Distribution** - `engram version --json` plus a published, credential-verified, recoverable Homebrew cask
- [ ] **Phase 2: Setup Command Core** - `engram setup` detects runtimes, previews by default, converges idempotently, and is fully scriptable without a TTY
- [ ] **Phase 3: Runtime Registration** - `engram setup --apply` registers engram with Claude Code, Codex, and opencode via their own CLIs, plus a generic-MCP fallback, across every auth mode
- [ ] **Phase 4: Skills Distribution** - The five curation skills reach every runtime, native format where one exists, AGENTS.md fallback otherwise
- [ ] **Phase 5: Slash Command Delegation** - `/engram-setup` delegates to the binary when present, keeps its prose fallback first-class otherwise, with a generated (not hand-checked) equivalence gate
- [ ] **Phase 6: Install Documentation** - docs-site documents how to get the binary and how to run `engram setup`

## Phase Details

### Phase 1: Version & Homebrew Distribution

**Goal:** A user can install engram with `brew install` on macOS or Linux (amd64 and arm64), and
the pipeline that publishes it is proven to work end to end rather than merely configured.
`engram version --json` lands in this same phase because it is the cask's install-time correctness
gate (`version.go:16` prints a bare string today) and cannot be delegated to Homebrew's
`generate_completions_from_executable`, which rescues a broken binary's failure to a warning
(`rescue => e; opoo e`). The cask's `postflight` strips `com.apple.quarantine` as its literal first
action, before it ever invokes the binary — reversing that order gets the gate itself SIGKILLed by
Gatekeeper, since engram ships unsigned by design (no GoReleaser Pro / Apple Developer Program
membership).

**Requirements:** REQ-version-json, REQ-homebrew-cask-published, REQ-cask-install-gate, REQ-cask-credential-verified, REQ-cask-reship-recovery

**Depends on:** Nothing (first phase).

**Success criteria:**

1. `engram version --json` prints a machine-readable payload carrying the version, while the
   existing human-readable `engram version` output is unchanged for existing callers.
2. A tagged release publishes a working Homebrew cask to `seanb4t/homebrew-tap` (via GoReleaser's
   `homebrew_casks:`), installable on both amd64 and arm64, on both macOS and Linux.
3. Installing a binary that fails the version-json assertion makes `brew install` fail loudly
   rather than install silently broken, even though the binary is unsigned — because the quarantine
   strip runs before the gate, not after.
4. The release workflow's cross-repo credential to `seanb4t/homebrew-tap` is proven to work by an
   explicit check performed before any real release depends on it — never assumed from the default,
   repo-scoped `GITHUB_TOKEN`.
5. A rehearsed failure between tag creation and cask publication is recovered using this repo's
   existing `workflow_dispatch` re-ship path, with no hand-edit to the tap.

**Plans:** TBD

---

### Phase 2: Setup Command Core

**Goal:** `engram setup` exists as a real, preview-by-default, fully scriptable CLI command that
can detect which supported agent runtimes are present on the machine and report what it would
write to each — before it writes anything. `cmd/engram/catalog.go` panics if any cobra command
lacks a row in `internal/surfaces/toolclass.go`, so registering the `setup` command and adding its
classification row land together in this one phase, not sequenced across two. If `setup` needs a
server-URL flag, it must not be spelled `--server`: `cmdwalk.go:118`'s `operatorCommands()`
predicate excludes any command carrying a flag literally named `server`, which would silently drop
`setup` out of operator-tier classification — the opposite of this milestone's intent.

**Requirements:** REQ-setup-detects-runtimes, REQ-setup-previews-by-default, REQ-setup-idempotent, REQ-setup-non-interactive, REQ-setup-partial-failure-legible, REQ-setup-correct-by-reading

**Depends on:** Nothing (parallelizable with Phase 1 — touches unrelated files).

**Success criteria:**

1. `engram setup` (no `--apply`) reports which supported runtimes are present, using each
   runtime's own binary as the primary signal — a leftover config directory from an uninstalled
   runtime does not read as installed — and shows the exact command or content it would issue per
   runtime, changing nothing on disk.
2. Running `engram setup --apply` twice converges to the same state on the second run, which
   reports "already correct" distinctly from the first run's "wrote it".
3. `engram setup` runs to completion without a TTY: a caller can select runtimes explicitly, skip
   confirmation, and receive machine-readable output — scriptable from CI or another agent.
4. When one runtime succeeds and another fails in the same invocation, the report names each
   runtime's own outcome individually, and the process exit status distinguishes total success,
   partial success, and total failure.
5. `engram setup --help` alone teaches which runtimes are targetable, what `--apply` does, and
   which auth modes are accepted, without the caller needing to run it and interpret a failure.

**Plans:** TBD

---

### Phase 3: Runtime Registration

**Goal:** `engram setup --apply` actually registers the engram MCP server with Claude Code, Codex,
and opencode by shelling out to each runtime's own CLI, and emits a portable configuration for any
MCP client engram doesn't natively support — covering every auth mode engram deploys behind. Live
post-synthesis verification confirmed `codex mcp add` (codex-cli 0.148.0) and `opencode mcp add`
(opencode 1.18.15) both exist, so all three native writers are structurally the same shape as the
already-shipped `/engram-setup` prose's `claude mcp add` path — no TOML or JSONC is ever parsed or
written, and the zero-new-Go-dependencies constraint is under no pressure.

**Requirements:** REQ-register-claude-code, REQ-register-codex, REQ-register-opencode, REQ-register-generic-mcp, REQ-register-auth-modes, REQ-register-cli-surface-drift-legible

**Depends on:** Phase 2 (the `Runtime` interface and the `setup` command must exist before any
runtime writer plugs into it).

**Success criteria:**

1. `engram setup --apply` registers engram with each of Claude Code, Codex, and opencode by
   invoking that runtime's own CLI (`claude mcp add`, `codex mcp add`, `opencode mcp add`
   respectively) — never by reading, parsing, or hand-writing any of their config files
   (`~/.claude.json`, `.mcp.json`, `~/.codex/config.toml`, opencode's config).
2. For an MCP client engram does not natively support, `engram setup` prints a portable server
   configuration the user can paste or redirect into that client.
3. Every registration path covers OAuth, pre-registered OAuth client, static bearer token, and
   no-auth deployments — or states plainly which are unsupported for that runtime — and no secret
   is ever placed on a command line where the shell or process table would capture it.
4. When a runtime's CLI is absent, or present with an unexpected flag surface, `engram setup`
   fails with a message naming the runtime and what it expected, rather than silently writing
   nothing.

**Plans:** TBD

---

### Phase 4: Skills Distribution

**Goal:** The five curation skills reach every configured runtime — installed in that runtime's
native skill or rules format where one exists, and appended into a delimited, re-detectable
AGENTS.md block where none does — from a brew-installed binary that carries the skill content
itself, so a Claude-plugin-free install still teaches an agent how to curate. The embedded content
is sourced from the same files the plugin ships, so the two cannot drift apart.

**Requirements:** REQ-skills-embedded-in-binary, REQ-skills-native-format, REQ-skills-agents-md-fallback

**Depends on:** Phase 3 (skill installation is scoped per runtime, reusing the same runtime list
and detection Phase 3 established).

**Success criteria:**

1. A brew-installed engram binary with no Claude plugin present can still produce the full content
   of the five curation skills, sourced from the same files the plugin ships.
2. For a runtime with a native skill or rules format, `engram setup --apply` installs the skills
   in that native format.
3. For a runtime with no native skill format, `engram setup --apply` writes the guidance into
   AGENTS.md inside a delimited, re-detectable block; re-running replaces that block rather than
   appending a second copy, and content outside the block is left byte-for-byte untouched.

**Plans:** TBD

---

### Phase 5: Slash Command Delegation

**Goal:** `/engram-setup` and `engram setup` are two entry points to the same outcome. The slash
command hands off to the binary when it's on PATH, and keeps its own prose bootstrap first-class
when it's absent — the plugin installs standalone via `claude plugin install`, so the binary is
never guaranteed present. The two paths are proven equivalent by construction: the mechanical parts
of the prose are generated from the same source of truth the CLI reads, with a CI gate that fails
on any difference after regeneration — not a keyword-presence or liveness check that could pass
while proving nothing, the failure shape this repo has hit before.

**Requirements:** REQ-engram-setup-delegates, REQ-engram-setup-prose-fallback, REQ-delegation-equivalence-derived

**Depends on:** Phase 4 (delegation hands off to a `engram setup` that already registers every
runtime and installs skills — otherwise "equivalent to the binary" has nothing complete to be
equivalent to).

**Success criteria:**

1. `/engram-setup` detects the `engram` binary on PATH and delegates to `engram setup` when it is
   present.
2. When the binary is absent, `/engram-setup` completes setup for the current agent using its own
   instructions, unchanged from today's first-class prose path.
3. The mechanical parts of `/engram-setup`'s prose are generated from the same source of truth
   `engram setup` reads, and CI fails on any difference between the generated content and what's
   committed — so the two paths cannot silently diverge.

**Plans:** TBD

---

### Phase 6: Install Documentation

**Goal:** docs-site tells a new user how to actually obtain engram and how to run `engram setup`,
reflecting the final, shipped behavior of every earlier phase rather than the Docker-only,
binary-optional story it tells today.

**Requirements:** REQ-docs-install-path, REQ-docs-setup-documented

**Depends on:** Phase 1 (needs the working cask) and Phase 5 (needs `engram setup`'s final behavior
and the delegation story settled).

**Success criteria:**

1. docs-site documents how to obtain the binary, including the exact working Homebrew invocation —
   closing the gap where `guides/quickstart.md` covers Docker only and `guides/cli.md` never says
   how to get the binary.
2. docs-site documents `engram setup` — which runtimes it configures, its preview/`--apply`
   behavior, and how to configure a runtime it does not support.

**Plans:** TBD

---

## Progress

**Execution Order:** 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 (v0.8.x, shipped) · 9 → 10 → 11 → 12 (v0.9.x, shipped 2026-07-10) · 13 → 14 (embedder track) · 15 → 16 → 17 → 18 → 19 (write-lane track, strict order) · 20 → 21 (independent) — v0.10.x shipped 2026-07-16 · 22 → 23 (Cedar foundation → service auth/tenancy, strict order) · 24 → 25 → 26 (capture trio + recall/config tail, strict order; 24 can start in parallel with 22–23) — v0.11.x shipped 2026-07-26 · v0.12.x: 1 → 2 (spine → CLI, strict order) · 3 · 4 · 5 · 6 (independent of the spine and of each other; ran in parallel once 1 was underway) · 7 (CLI cross-spine wiring, closed the audit seam between 2 and 3) — v0.12.x shipped 2026-08-02 · v0.13.x: 1 · 2 (parallelizable with each other) → 3 (needs 1 and 2 settled first) → 4 (authored in parallel with 3, full acceptance trails it) → 5 (last; needs 3's `verify` for the #355 fixture, reconciles each phase's own validation as it closes) — v0.13.x planned 2026-08-03 · 2026-08-12.01: 1 → 2 → 3 → 4 → 5 (needs 4) · 6 (independent, parallelizable with 3–5; must finish before 7) → 7 (needs 5 and 6) → 8 (needs 4 and 7) — 2026-08-12.01 roadmapped 2026-08-12 · 2026-08-23.01: 1 · 2 (independent, parallelizable with 1) → 3 → 4 → 5 (strict order) → 6 (needs 1 and 5) — 2026-08-23.01 roadmapped 2026-08-23

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
| 6. Typed Operator Renderer | 2026-08-12.01 | 1/1 | Complete | 2026-08-17 |
| 7. Console & CLI State Surfacing | 2026-08-12.01 | 3/3 | Complete | 2026-08-20 |
| 8. Registry & Docs Tail | 2026-08-12.01 | 3/3 | Complete | 2026-08-22 |
| 9. Report pending in migrate status | 2026-08-12.01 | 2/2 | Complete | 2026-08-22 |
| 1. Version & Homebrew Distribution | 2026-08-23.01 | 0/5 | Not started | - |
| 2. Setup Command Core | 2026-08-23.01 | 0/6 | Not started | - |
| 3. Runtime Registration | 2026-08-23.01 | 0/6 | Not started | - |
| 4. Skills Distribution | 2026-08-23.01 | 0/3 | Not started | - |
| 5. Slash Command Delegation | 2026-08-23.01 | 0/3 | Not started | - |
| 6. Install Documentation | 2026-08-23.01 | 0/2 | Not started | - |

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
