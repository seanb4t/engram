# Synthesis Summary — merge-adrs

Entry point for gsd-roadmapper. Follow-up merge of the fine-grained companion ADRs
omitted from the original 50-doc bootstrap.

## Mode
merge — checked against 25 already-LOCKED baseline decisions in `.planning/PROJECT.md`
plus existing `.planning/intel/*` and `.planning/{REQUIREMENTS,ROADMAP}.md`.

## Doc counts by type
- ADR: 31 (all LOCKED, precedence 0, high confidence)
- SPEC / PRD / DOC: 0
- UNKNOWN / low-confidence: 0

## Decisions
- Locked decisions extracted: 31 → `decisions.md`
- Sources: docs/adr/engram-{0gy, 1h3k, 1w7, 2xl, 3l0, 3nas, 4ag, 4y7p, 50b, 6gb, 7qd,
  8q3, 9tj, bgj, c0m, c4y, d24, d386, ddiw, f7p, lzz, m4s8, no3, om5b, tdk, u5h, u9v,
  ufz, vxk, wot, wtw}-*.md
- Character: all are narrower-grain companions that REFINE/IMPLEMENT a scope already
  governed by a baseline lock. No new product scope introduced.

## Requirements
- Extracted: 0 (no PRDs in set) → `requirements.md`

## Constraints
- Implied normative constraints captured: 15 → `constraints.md`
  (breakdown: nfr/PII 3, api/handler contract 5, config 2, ui/security 3, ops/packaging 2)
- All derived from ADRs; no dedicated SPEC documents ingested.

## Context
- Cross-ref graph + refinement map + 50b adjudication → `context.md`
- Cycle detection: 3-color DFS, depth cap 50 → NO cycles (in-set edges 8q3→u9v,
  m4s8→ddiw are terminal; max depth 2).
- Dangling cross-refs (out-of-set / non-id): recorded, non-blocking.

## Conflicts
- Blockers: 0
- Competing variants: 0
- Auto-resolved / INFO: 11
- Detail: `INGEST-CONFLICTS.md`

## Key adjudications
- engram-50b vs bundled skill/engram plugin: CONSISTENT (INFO). Plugin stays bundled;
  50b only removes the auto-registering .mcp.json so /engram-setup is the sole MCP
  registration path. Different axes.
- engram-8q3 vs engram-u9v: staged/complementary (INFO), not contradictory — v1 read
  lane (sub+expiry) vs eventual write-phase custody (access+refresh+sub).

## Downstream note
These 31 companions do not add, remove, or alter product requirements or acceptance
criteria. They are safe to fold into the decision record as refinements of the existing
25 baseline locks. No BLOCKER gate is raised.

## Pointers
- Decisions:   .planning/intel/merge-adrs/decisions.md
- Constraints: .planning/intel/merge-adrs/constraints.md
- Requirements:.planning/intel/merge-adrs/requirements.md
- Context:     .planning/intel/merge-adrs/context.md
- Conflicts:   .planning/intel/merge-adrs/INGEST-CONFLICTS.md
