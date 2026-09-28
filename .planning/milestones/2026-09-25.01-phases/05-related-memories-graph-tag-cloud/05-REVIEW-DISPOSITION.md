---
phase: 05
review: 05-REVIEW.md
titles: json
findings:
  - id: WR-01
    severity: warning
    disposition: open
    title: "`RelatedGraph.svelte`'s floating-card glyph map hand-copies `LANE_GLYPH` instead of importing it"
  - id: WR-02
    severity: warning
    disposition: open
    title: "`RelatedGraph.svelte`'s auto-pan effect can be re-triggered by an unrelated node's drag, not just the focused node's own movement"
  - id: WR-03
    severity: warning
    disposition: open
    title: "`TagBars.svelte`'s roving-tabindex listbox gives no visible indication of the currently-focused row"
  - id: WR-04
    severity: warning
    disposition: open
    title: "`/related/[id]`'s trail-driven \"back\" can strand the user outside the app on a direct/bookmarked URL"
  - id: IN-01
    severity: info
    disposition: open
    title: "`RelatedGraph.svelte`'s `cardPlacement` hardcodes the viewBox half-extents instead of deriving them from `VIEW`"
  - id: IN-02
    severity: info
    disposition: open
    title: "Skeleton call-line text hardcodes `k=64` instead of interpolating `RELATED_K`"
open: 6
total: 6
recorded: 2026-09-28T05:38:41.959Z
---

# Phase 05: Code Review Disposition

| Finding | Severity | Disposition | Source |
|---------|----------|-------------|--------|
| WR-01 | warning | open | - |
| WR-02 | warning | open | - |
| WR-03 | warning | open | - |
| WR-04 | warning | open | - |
| IN-01 | info | open | - |
| IN-02 | info | open | - |

Dispositions: `open` (recorded, not yet triaged), `fixed`, `skipped`, `deferred`.
Set `deferred` by hand and put the reason in the Source cell; both are preserved. A `|` in the reason is kept as prose and escaped on the next run.
Re-running the gate keeps every row it can. A row the current review no longer reports is kept and its Source cell flagged, so a finding does not leave this record silently. ONE exception: when a finding id is REUSED by a different finding, the earlier decision cannot keep a row — the id is taken — and it is dropped. A RECORDED decision (anything but `open`) is named on the console when that happens; a row still at `open` is replaced silently, because `open` records no decision to lose.
