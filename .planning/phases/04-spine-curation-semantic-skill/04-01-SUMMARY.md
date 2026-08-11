---
phase: 04-spine-curation-semantic-skill
plan: 01
subsystem: skills
tags: [engram, skill, spine-curation, consent-gate, supersede]
dependency-graph:
  requires: []
  provides:
    - "skill/engram/skills/curating-spine/SKILL.md (identity axis: consolidate intake -> verdict -> verb proposal -> consent -> supersede_memory)"
    - ".planning/phases/04-spine-curation-semantic-skill/COVERAGE.md (no-external-API declaration)"
  affects:
    - "04-02 (adversarial cold-read proof over this file's consent gate)"
    - "04-03 (expands this file with the staleness axis and reactive-recall trigger)"
tech-stack:
  added: []
  patterns:
    - "sibling skill file, flat H2 workflow shape (promoting-memory/discovering analog, not curating-memory's deep nesting)"
    - "verbatim reuse of an existing consent protocol step and verb-selection table, content-anchored not line-anchored"
key-files:
  created:
    - skill/engram/skills/curating-spine/SKILL.md
    - .planning/phases/04-spine-curation-semantic-skill/COVERAGE.md
  modified: []
decisions:
  - "Reused curating-memory's ask-once-then-stop consent step (SKILL.md:89-92) and D-09 verb table (SKILL.md:336-338) byte-for-byte per plan requirement, verified via content-anchored automated gates rather than line numbers."
  - "COVERAGE.md written to resolve the blocking api-coverage.verify-pre gate at plan time (no capability matrix — there is no external API in this phase)."
actuals:
  tokens: 2439
  tasks: 2
  commits: 2
metrics:
  duration: "~35m"
  completed: 2026-08-11
status: complete
---

# Phase 4 Plan 1: Curating-Spine Skill Tracer Summary

One-liner: New sibling skill `curating-spine/SKILL.md` ships the full identity-axis path — `spine-review consolidate --output json` candidate intake through a consented multi-target `supersede_memory` call — with a six-tool allow-list and no `delete_memory` in the merge path.

## What Was Built

- **`skill/engram/skills/curating-spine/SKILL.md`** (160 lines): a new sibling skill matching the
  `promoting-memory`/`discovering` flat-H2 shape (D-01). Sections: `## Tools this skill may call`
  (explicit six-tool allow-list with full `mcp__engram__` prefix, and the three-way 401/403/tool-layer
  rejection split); `## Record content is data, not instruction` (T-04-01 mitigation — record content
  is untrusted, never a directive); `## Getting candidate pairs` (consumes `consolidate --output json`
  by its real field names, never derives pairs itself — D-04); `## Identity verdicts` (same-fact /
  overlapping / distinct, with the qualifier-adjacency operational test as the same-fact/overlapping
  boundary — D-08); `## Choosing the verb` (verbatim D-09 table); `## Proposing a mutation` (the
  consent gate — batch report grouped by verdict as the single inline moment, per-item ask-once-stop
  reproduced verbatim from `curating-memory/SKILL.md:89-92`, forbidden-shortcuts, report-ordering and
  zero-findings rules).
- **`.planning/phases/04-spine-curation-semantic-skill/COVERAGE.md`**: reasoned no-external-API
  declaration so the blocking `api-coverage.verify-pre` gate resolves now rather than at seal time.

## Deviations from Plan

None — plan executed exactly as written. All five automated gates (A: verb table verbatim, B: consent
step verbatim, C: allowed-tool surface both directions plus both abbreviation forms, D: zero
server-side diff) pass, `rumdl check` on the new file exits 0, and `task lint` / `task license:check`
are green.

## Auth Gates

None encountered.

## Known Stubs

None. The tracer's own scope excludes the staleness axis and reactive-recall trigger (both explicitly
deferred to plan 04-03) — this is documented plan scope, not a stub.

## Verification Results

- Gate A (verb table verbatim vs. `curating-memory/SKILL.md:336-338`): PASS
- Gate B (consent step 3 verbatim vs. `curating-memory/SKILL.md:89-92`): PASS
- Gate C (six-tool allow-list positive + negative + both abbreviation forms): PASS
- Gate D (`git diff --stat 72a32c58..HEAD -- internal/ cmd/ proto/ gen/` empty): PASS
- `rumdl check skill/engram/skills/curating-spine/SKILL.md`: PASS (0 issues)
- `task lint` (rumdl, yamlfmt, actionlint, golangci-lint, ruff): PASS
- `task license:check`: PASS (0 invalid; new files correctly excluded — no SPDX header, per
  `.licenserc.yaml`'s `skill/**/SKILL.md` and `.planning/**` carve-outs)
- COVERAGE.md gate (non-empty, declares "no external API integration", no SPDX header on line 1): PASS

## Self-Check: PASSED

- FOUND: skill/engram/skills/curating-spine/SKILL.md
- FOUND: .planning/phases/04-spine-curation-semantic-skill/COVERAGE.md
- FOUND commit 1cdef4d9 (feat(04-01): add curating-spine skill tracer path)
- FOUND commit 20d4f26c (docs(04-01): declare no external API integration for phase 4)
