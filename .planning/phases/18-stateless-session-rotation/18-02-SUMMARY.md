---
phase: 18-stateless-session-rotation
plan: 02
subsystem: docs
tags: [adr, session-rotation, security-posture]
dependency-graph:
  requires: []
  provides: [engram-slr8-adr]
  affects: [gsd-secure-phase]
tech-stack:
  added: []
  patterns: [hand-authored ADR (post-beads-retirement)]
key-files:
  created:
    - docs/adr/engram-slr8-stateless-sliding-session-reseal.md
  modified:
    - docs/adr/README.md
decisions:
  - "engram-slr8 ADR (Accepted) authored hand-written, omitting the bd-render provenance comment (beads retired 2026-07-08)"
  - "ADR names ENGRAM_UI_COOKIE_KEY (registry.go:56) as the sole kill-switch, never the phantom ENGRAM_SESSION_KEY"
metrics:
  duration: 5min
  completed: 2026-07-13
status: complete
---

# Phase 18 Plan 02: Stateless Sliding-Expiry Session Re-seal ADR Summary

Hand-authored the SC2 Architecture Decision Record (engram-slr8, Accepted) documenting
what "rotation" means under statelessness, the explicit no-revocation limitation with
its single ENGRAM_UI_COOKIE_KEY kill-switch, and the hard-expiry-strict vs
threshold-skew-tolerant split — then indexed it in docs/adr/README.md and corrected
the now-stale bd-render pipeline prose.

## What Was Built

### Task 1: engram-slr8 ADR

Created `docs/adr/engram-slr8-stateless-sliding-session-reseal.md` matching the
existing rendered ADR shape (Date/Status/Decision/Deciders header block; Context /
Decision / Rationale / Alternatives Considered / Consequences / References sections)
but deliberately omitting the `<!-- adr-render: source=bd:... -->` provenance comment,
since the bd→render pipeline died with beads retirement (2026-07-08).

Content covers the three D-10-mandated points:
1. "Rotation" under statelessness = sliding-expiry re-seal of `{owner, expiry}` with
   zero server-side state — explicitly not a token store, not server-side revocation.
   Amends engram-u9v's original per-request-refresh clause.
2. The explicit no-revocation limitation, stated prominently in Consequences: a
   stolen sealed cookie is valid up to a full `sessionTTL`, and because sliding
   re-seal extends the window while actively used, an actively-abused stolen cookie
   never expires on its own. The ONLY kill-switch is rotating `ENGRAM_UI_COOKIE_KEY`
   (`ui.cookie_key`, `internal/config/registry.go:56`) — verified as the real key;
   the phantom `ENGRAM_SESSION_KEY` from the ROADMAP SC2 prose does not appear.
3. The hard-expiry-strict vs threshold-skew-tolerant split: `Resolver.Resolve`'s
   hard-expiry check stays untouched/fail-closed; the `resealSkew` budget applies
   only to the soft re-seal-threshold comparison.

References section: Amends engram-u9v; Governed by (unchanged) engram-8q3; Revisits
engram-1xv.

### Task 2: README index + prose correction

Edited `docs/adr/README.md`:
- Inserted the engram-slr8 row at the top of the newest-first index table
  (2026-07-13, Accepted).
- Added a sentence to the intro noting beads retirement (2026-07-08) killed the
  bd-render pipeline, so ADRs dated after that point are hand-authored Markdown with
  no backing bead and no render step. The original "edit the bead, then re-render"
  instruction was left in place for the 60 pre-existing bd-backed ADRs (still
  accurate for those), with the new sentence clarifying the split.

## Verification

- Task 1 verify command: `test -f ... && ! grep ENGRAM_SESSION_KEY && ! grep
  'adr-render: source=bd' && grep ENGRAM_UI_COOKIE_KEY` → printed `OK`.
- `task lint:markdown` (rumdl): zero issues on either touched file (pre-existing
  issues in unrelated `.planning/*.md` files are out of scope, tracked separately
  for Phase 21 per STATE.md).
- `task license:check`: 0 invalid files (docs/adr/*.md is exempt per
  `.licenserc.yaml`; confirmed no SPDX header was added to the new ADR).

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check

- FOUND: docs/adr/engram-slr8-stateless-sliding-session-reseal.md
- FOUND: docs/adr/README.md (modified)
- FOUND: commit 706a0ca7 (Task 1 — author ADR)
- FOUND: commit 5a282be1 (Task 2 — index + prose correction)

## Self-Check: PASSED
