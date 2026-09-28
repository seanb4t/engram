---
phase: 04
review: 04-REVIEW.md
titles: json
findings:
  - id: WR-01
    severity: warning
    disposition: open
    title: "Add-target-by-id/short_id lookup has no stale-response guard"
  - id: WR-02
    severity: warning
    disposition: open
    title: "`onresolvehead` never retries a failed head resolution"
  - id: WR-03
    severity: warning
    disposition: open
    title: "State-chip overflow collapse never un-collapses"
  - id: WR-04
    severity: warning
    disposition: open
    title: "`ChainDialog`'s `peekId` is not reset when the dialog reopens for a different anchor"
  - id: WR-05
    severity: warning
    disposition: open
    title: "Shift+click range-select does not move keyboard focus (`activeId`)"
  - id: WR-06
    severity: warning
    disposition: open
    title: "Toast-triggered restore-undo (D-09 surface 2) skips the row flash and `onchanged` notification that the in-dialog undo (surface 1) performs"
  - id: IN-01
    severity: info
    disposition: open
    title: "Client-side \"not owned\" guess for a rejected supersede/archive target can be wrong for a since-deleted record"
  - id: IN-02
    severity: info
    disposition: open
    title: "`resolveRecords`/`resolveRecordsKeepAll` re-fetch is not deduplicated against concurrent opens"
open: 8
total: 8
recorded: 2026-09-27T20:30:50.302Z
---

# Phase 04: Code Review Disposition

| Finding | Severity | Disposition | Source |
|---------|----------|-------------|--------|
| WR-01 | warning | open | - |
| WR-02 | warning | open | - |
| WR-03 | warning | open | - |
| WR-04 | warning | open | - |
| WR-05 | warning | open | - |
| WR-06 | warning | open | - |
| IN-01 | info | open | - |
| IN-02 | info | open | - |

Dispositions: `open` (recorded, not yet triaged), `fixed`, `skipped`, `deferred`.
Set `deferred` by hand and put the reason in the Source cell; both are preserved. A `|` in the reason is kept as prose and escaped on the next run.
Re-running the gate keeps every row it can. A row the current review no longer reports is kept and its Source cell flagged, so a finding does not leave this record silently. ONE exception: when a finding id is REUSED by a different finding, the earlier decision cannot keep a row — the id is taken — and it is dropped. A RECORDED decision (anything but `open`) is named on the console when that happens; a row still at `open` is replaced silently, because `open` records no decision to lose.
