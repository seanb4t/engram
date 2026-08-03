---
gsd_state_version: 1.0
milestone: v0.13.x
milestone_name: Curation & Self-Evidence
current_phase: 01
current_phase_name: interface-enforceability
status: executing
stopped_at: Completed 01-03-PLAN.md
last_updated: "2026-08-03T19:31:45.015Z"
last_activity: 2026-08-03
last_activity_desc: Phase 01 execution started
progress:
  total_phases: 1
  completed_phases: 0
  total_plans: 9
  completed_plans: 3
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-08-02 — after closing milestone v0.12.x)

**Core value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its context, and wrong/stale memories can be corrected or superseded.
**Current focus:** Phase 01 — interface-enforceability

## Current Position

Phase: 01 (interface-enforceability) — EXECUTING
Plan: 4 of 9
Status: Ready to execute
Last activity: 2026-08-03 — Phase 01 execution started

## Deferred Items

Items acknowledged and deferred at milestone close on 2026-07-10:

| Category | Item | Status |
|----------|------|--------|
| pending_todo | document-embedding-model-options (docs-site + Helm embedding-model guide) | Picked up in v0.10.x Phase 14 (REQ-embed-model-docs, #337) |

## Accumulated Context

### Decisions

Full decision record lives in `.planning/PROJECT.md` (56 ADR-locked baseline decisions plus the
per-milestone Key Decisions table). Per-milestone detail is archived alongside each milestone in
`.planning/milestones/v*-{ROADMAP,REQUIREMENTS}.md`. This section carries only what the *next*
milestone needs in working memory.

**Standing invariants (do not relitigate without an ADR):**

- Authorization is enforced in `internal/store` (Qdrant read filters + owner gates), never in
  handlers. As of v0.11.x the predicate comes from the `internal/authz` Cedar PDP — bucket-level
  decisions only, compiled into the Qdrant filter; no per-record Cedar eval on bulk paths
  (ADR `engram-cdr1`, refines LOCKED `DEC-cgb`).

- Capture is explicit and zero-junk. No auto-extraction, no similarity-triggered supersession, no
  auto-populated citations.

- Unauthorized id-addressed operations are 404-indistinguishable from a missing id (`DEC-xa6`).
- One Qdrant collection for every memory kind; new features add payload keys, never collections
  (`DEC-2bv`).

**Carry-forward gotchas for the next milestone:**

- New payload keys must survive every sibling write path. Whole-payload `Upsert` either round-trips
  all out-of-band keys (`idempotency_fingerprint`, `superseded_by`, `citations`) or takes
  `store.TargetLocker`; targeted `SetPayload` is the merge-safe alternative.

- `contentFingerprint` (`internal/server/idempotency.go`) hashes an **explicit** field list, not
  reflection — any new client-authored `storeArgs` field must be added to it in the same change, or
  a keyed replay silently discards the caller's value.

- Provider-endpoint URLs must go through the shape-aware `internal/openaiurl.Join`, never a bare
  concat. Fixing one lane and not its sibling is how the doubled-`/v1` bug survived from Phase 13
  to Phase 26.

- `connectError`'s `*argError` case must stay FIRST in the type switch, ahead of the
  `store.ErrInvalidArgument` sentinel arm — `argError.Unwrap()` returns that sentinel, so any
  reordering silently collapses every error class back to `CodeInvalidArgument`. A test asserting
  only "not `CodeInternal`" still passes on the collapse; assert the codes are DISTINCT. Durable
  record: `667p88n2be`.

- Proto field numbers: a `deprecated = true` field still OCCUPIES its number. Reusing one is the
  single way an otherwise-additive change trips `buf breaking` in FILE mode. Any new required
  `internal/config` registry field must also be added to every full `Config{}` literal in that
  package's tests. Durable record: `s780vae1vr`.

- The CLI is **correct-by-reading** (`4aksmneehh`): help text and the self-describe catalog are
  deliverables with acceptance criteria, related flags name each other, and a validation error is
  a backstop for someone who did not read — never the teaching mechanism.

**v0.13.x roadmap decisions (2026-08-03, carried from research + user scoping):**

- `spine-review` is the sixth Subject-less operator-tier command (`reindex`,
  `migrate-remap-owner`, `prune-expired`, `summarize-missing`, `backfill-short-ids`) — never a new
  authz path, never composed from `Search`/`List`.

- #467 resolves via **unification**, not a documented boundary (user override of the research
  default recommendation) — ships with a pinned-current-behavior regression test authored before
  the change, a consumer audit, and a `guides/upgrade.md` entry.

- #453 and #467 are phased together (v0.13.x Phase 1): cobra's `MarkFlagsMutuallyExclusive` raises
  a plain `fmt.Errorf` that bypasses `cliError`/`ExitCode()`, so adopting #453 without resolving
  #467 first would reintroduce the exact undocumented exit-code split #467 exists to close.

- `REQ-archive-tier` (Phase 3) and the semantic-skill cold-read test design (Phase 4) are both
  flagged as needing a research pass at plan-phase — no single existing precedent to copy verbatim.

- Phase 5 (Nyquist reconciliation + #355) is ordered last: #355 is the live acceptance fixture for
  Phase 3's `verify`, not a prerequisite to building it.

- [Phase ?]: D-09 before-table: every hand-derived 'before' exit code matched actual observed behavior on the first run; fixed a flag-state leak (resetCommandFlagState only covers the handed command, not the whole tree) discovered under full-package test run order
- [Phase ?]: D-08 checkpoint pre-approved: reject-both (any two of the paging trio, regardless of value, including offset=0/page-token='') — recorded as accepted, execution proceeded without stopping
- [Phase ?]: resetCommandFlagState (plan 01-01) had a latent bug: f.Value.Set(f.DefValue) corrupts stringSlice-typed flags via pflag's append-once-changed semantics; fixed to skip Set for stringSlice flags
- [Phase ?]: client.timeout=0 rejected as usage error (D-05), diverging from Embed/Summarize.Timeout's zero-means-unbounded convention
- [Phase ?]: ValidateClient kept structurally separate from Config.Validate to avoid forcing ~33 hand-built Config{} test literals to carry new required fields

### Blockers/Concerns

**Open:**

- **Deployed server lags `main`:** the running engram instance predates the v0.11.x and v0.12.x merges, so `supersede_memory`, memory `citations`, the `categories` filter, and every v0.12.x capability (Connect bearer identity, the headless CLI, `cross_spine`, the field+hint error envelope) are not callable until the next release.
- **Not deployed → not exercised:** every v0.11.x and v0.12.x feature is verified against tests and a real Qdrant via testcontainers, but none has run in the deployed instance. Watch the first release for integration surprises.
- **Validation commands can false-green:** `go test -run X ./pkg/...` matching nothing exits 0 with `ok … [no tests to run]`. This bit v0.12.x too: VALIDATION.md `-run` commands are written at PLAN time and routinely never match what shipped (wrong package in Phase 4, wrong test name in Phase 7), so the row reports a false green forever. Re-resolve every `-run` against `go test -list` when auditing, and prove execution with `-v` RUN/PASS pairs, not a package-level `ok`. Durable record: `bsbsvn4hbc`. **This is v0.13.x Phase 5's own deliverable — do not let Phase 5 reproduce the bug it exists to close.**
- Tracked tech debt: #369 (Renovate self-heal live observation, post-merge only), #366 (console e2e harness), #370 (Taskfile yamlfmt/CI reconciliation), plus 2 high Dependabot alerts open on `main`.
- **CI gates outside the phase lifecycle:** `task chart:validate` (containerEnv checksum pin) and `task ui:build` (vendored SPA) are required checks that no phase gate runs. Run both locally before shipping any phase touching `charts/` or generated TS.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260717-g1r | Triage + fix #301 — Renovate ui/ postUpgradeTasks via `bash -c` (shell-free); branch unmerged, gated on a cluster-first allowlist update | 2026-07-17 | 1462da20 | [260717-g1r-renovate-ui-vendor-shell](./quick/260717-g1r-renovate-ui-vendor-shell/) |

### Roadmap Evolution

- Phase 1 edited: edited fields: requirements, success_criteria (D-04 client-config scope expansion)

## Session Continuity

Last session: 2026-08-03T19:31:45.007Z
Stopped at: Completed 01-03-PLAN.md
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
| Phase 26 P02 | 9min | 2 tasks | 2 files |
| Phase 26 P04 | 25min | 3 tasks | 13 files |
| Phase 26 P03 | 12min | 2 tasks | 7 files |
| Phase 26 P05 | 6min | 3 tasks | 6 files |
| Phase 26 P06 | 18min | 3 tasks | 4 files |
| Phase 01 P01 | 40min | 2 tasks | 16 files |
| Phase 01 P02 | 13min | 2 tasks | 4 files |
| Phase 01 P03 | 35min | 3 tasks | 11 files |
| Phase 01 P04 | ~20min | 3 tasks | 4 files |
| Phase 02 P01 | 40min | 2 tasks | 7 files |
| Phase 02 P02 | ~15min | 2 tasks | 4 files |
| Phase 02 P03 | ~35min | 2 tasks | 4 files |
| Phase 03 P01 | 9min | 3 tasks | 3 files |
| Phase 03 P02 | 12min | 2 tasks | 4 files |
| Phase 03 P03 | 20min | 3 tasks | 5 files |
| Phase 03 P04 | ~10min | 4 tasks | 7 files |
| Phase 03 P05 | ~15min | 2 tasks | 3 files |
| Phase 04 P01 | 5min | 3 tasks | 4 files |
| Phase 04 P03 | 6min | 3 tasks | 5 files |
| Phase 04 P02 | 35min | 3 tasks | 6 files |
| Phase 04 P04 | ~25min | 3 tasks | 3 files |
| Phase 04 P05 | 18min | 3 tasks | 5 files |
| Phase 04 P06 | 35min | 2 tasks | 15 files |
| Phase 01 P01 | 15min | 3 tasks | 2 files |
| Phase 01 P02 | ~13min | 4 tasks | 8 files |
| Phase 01 P03 | 4min | 3 tasks | 5 files |

## Operator Next Steps

- Review `.planning/ROADMAP.md`'s v0.13.x phase details (5 phases, 18/18 requirements mapped).
- Start Phase 1 with `/gsd-plan-phase 1`.
