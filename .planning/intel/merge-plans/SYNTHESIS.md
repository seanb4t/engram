<!--
SPDX-License-Identifier: Apache-2.0
-->
# Synthesis — engram implementation plans (Pass B, DOC set)

Entry point for `gsd-roadmapper`. This pass folds in 24 engram
implementation/execution plans, all classified **DOC (precedence 3 — lowest)**.
Their sole role is **traceability context**: the HOW behind features whose
WHAT/decisions are already owned by the 25 baseline ADRs + 25 baseline specs +
31 Pass-A companion ADRs. No normative content (decisions/requirements/constraints)
is extracted from them — a DOC cannot override higher-precedence sources, so
precedence conflicts are impossible by construction.

## Doc counts by type

- DOC: 24 (all high-confidence, precedence 3, not locked, manifest_override=true)
- ADR / SPEC / PRD: 0
- UNKNOWN / low-confidence: 0

## Extracted intel

- Decisions: 0 — none (DOC context set); see decisions.md
- Requirements: 0 — none (DOC context set); see requirements.md
- Constraints: 0 — none (DOC context set); see constraints.md
- Context topics: 24 — one compact entry per plan, in context.md

## Cross-ref cycle detection

- Method: 3-color DFS, depth cap 50 — PASSED, no cycles.
- All cross_refs are dangling out-of-set traceability references (spec filenames,
  ADR/bead ids `engram-*`, PRs, versions).
- One intra-set edge: rule-memory-kind plan -> short-id-handle plan (single
  direction, no back-edge, acyclic).

## Context grouped by baseline phase

- Phase 1 — Authorization & Isolation: 3 plans
  (per-actor isolation, typed Subject authz-core, configurable-claim owner)
- Phase 2 — Recall Semantics: 3 plans
  (scheduled memories, windowed+cursor recall, auto-summary)
- Phase 3 — Memory Kinds & Tools: 3 plans
  (discovery memory type, rule memory kind, short_id handle)
- Phase 4 — Embedder: 1 plan
  (asymmetric/cloud embedder param passthrough)
- Phase 5 — Config & Transport: 2 plans
  (ENGRAM_ prefix + koanf config, Config.Validate)
- Phase 6 — Telemetry & Observability: 2 plans
  (slog + OTel/OTLP, telemetry at every seam)
- Phase 7 — Web UI, Docs Site & Distribution: 9 plans
  (web-ui v1 backend, operator-console SPA, console redesign, memory-display UX,
  vitest browser-mode tests, docs site, docs landing redesign, brand system,
  relocate memory-curator)
- Phase 8 — Connect Observe-Lane Auth Hardening (DEFERRED): 1 plan
  (web UI cookie/OIDC auth lane — REQ-connect-auth-posture, R1–R4)

Total: 24 plans mapped across 8 phases.

## Conflicts

- Blockers: 0
- Competing variants: 0
- Auto-resolved: 0

(As expected for a pure DOC set — precedence conflicts impossible by construction.)

## Pointers

- Conflict report: .planning/intel/merge-plans/INGEST-CONFLICTS.md
- Context intel (per-plan, phase-grouped): .planning/intel/merge-plans/context.md
- Decisions / Requirements / Constraints: .planning/intel/merge-plans/{decisions,requirements,constraints}.md (all empty — DOC context set)
- Baseline this pass traces against:
  .planning/PROJECT.md (25 baseline LOCKED ADRs),
  .planning/intel/decisions.md,
  .planning/intel/merge-adrs/decisions.md (31 Pass-A companion ADRs),
  .planning/REQUIREMENTS.md, .planning/ROADMAP.md
