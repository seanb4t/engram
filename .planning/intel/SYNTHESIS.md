# Synthesis Summary

Single entry point for `gsd-roadmapper`. Synthesized from 50 classified planning docs for
**engram** — a self-hosted, correctable, OAuth-secured memory MCP server for coding agents
(Go + Qdrant). Mode: `new`. Precedence: ADR > SPEC > PRD > DOC.

## Doc counts by type

- ADR: 25 (all LOCKED, precedence 0)
- SPEC: 25 (precedence 1)
- PRD: 0
- DOC: 0
- UNKNOWN/low-confidence: 0
- Total: 50

## Decisions locked (25)

All 25 ADRs are LOCKED and cannot be auto-overridden. Grouped by area (full text +
source paths in `decisions.md`):

- Authorization & Isolation (5): engram-cgb, engram-g37x, engram-kyz, engram-xa6, engram-12c
- Recall Semantics (5): engram-ambu, engram-4xt7, engram-y1g, engram-1frj, engram-ef28
- Memory Kinds & Tools (5): engram-2bv, engram-90w, engram-iedk, engram-zzq0, engram-02ta
- Embedder (2): engram-378, engram-zyhq
- Config (3): engram-jgq, engram-irq, engram-bj6
- Telemetry (2): engram-dwi, engram-uxh
- Web UI & Docs Site (3): engram-8xe, engram-0lu, engram-ttb

## Requirements extracted (25)

One functional requirement per SPEC (`requirements.md`). IDs:
REQ-per-actor-isolation, REQ-typed-subject-authz, REQ-configurable-claim-owner,
REQ-connect-auth-posture, REQ-scheduled-memories, REQ-windowed-cursor-recall, REQ-auto-summary,
REQ-discovery-memory-type, REQ-rule-memory-kind, REQ-short-id-handle,
REQ-asymmetric-embedder-params, REQ-config-prefix-koanf, REQ-config-validation,
REQ-observability-telemetry, REQ-telemetry-seams, REQ-web-ui-console, REQ-operator-console-spa,
REQ-operator-console-redesign, REQ-memory-display-ux, REQ-ui-test-unification, REQ-docs-site,
REQ-docs-landing-redesign, REQ-brand-identity, REQ-relocate-memory-curator.
(REQ-connect-auth-posture carries DEFERRED requirements R1–R4.)

The historical client-config-generalization SPEC is recorded as context only (superseded by
out-of-set ADR engram-50b), not routed as an active requirement.

## Constraints (21)

Technical contracts and bounds in `constraints.md`, by type:

- api-contract (5): embedder body params, EngramService read RPCs, summary-by-default recall,
  cursor+date-window recall, AND-default tag filter
- schema (5): 10-char Crockford short_id, temporal window payloads, owner authz key,
  Qdrant payload indexes, discovery 5th category
- protocol (6): store-layer default-deny authz, read/write asymmetry, 404 uniformity,
  recall-gate asymmetry, MCP transport path, deferred Connect auth R1–R4
- nfr (5): OTLP-only telemetry, telemetry non-blocking, fatal legacy-env config guard,
  real-Chromium UI tests, static-only docs site

## Context topics (3 groupings)

`context.md`: historical/superseded notes (3), expected ADR↔SPEC pairing map, dangling
cross-refs to 5 out-of-set decisions, and the acyclic in-set cross-ref graph.

## Conflicts

- BLOCKERS: 0
- WARNINGS (competing variants): 0
- INFO (auto-resolved / historical / deferred): 6

Detail: `.planning/INGEST-CONFLICTS.md`

## Pointers

- Decisions: `.planning/intel/decisions.md`
- Requirements: `.planning/intel/requirements.md`
- Constraints: `.planning/intel/constraints.md`
- Context: `.planning/intel/context.md`
- Conflicts: `.planning/INGEST-CONFLICTS.md`

## Status

READY — safe to route. No blockers, no competing variants requiring user resolution.
