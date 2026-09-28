---
phase: 06
review: 06-REVIEW.md
titles: json
findings:
  - id: WR-01
    severity: warning
    disposition: open
    title: "`past_month`/`past_year` time-window suggestion can silently shrink to a few days"
  - id: WR-02
    severity: warning
    disposition: open
    title: "`UnderstandQueryRequest.cross_spine` is threaded through but never consulted"
  - id: IN-01
    severity: info
    disposition: open
    title: "`Result.Audit` logs the trimmed query, not \"as received\" as documented"
  - id: IN-02
    severity: info
    disposition: open
    title: "`MatchTags` and `NewRequest` run against different query lengths"
open: 4
total: 4
recorded: 2026-09-28T16:34:59.667Z
---

# Phase 06: Code Review Disposition

| Finding | Severity | Disposition | Source |
|---------|----------|-------------|--------|
| WR-01 | warning | open | - |
| WR-02 | warning | open | - |
| IN-01 | info | open | - |
| IN-02 | info | open | - |

Dispositions: `open` (recorded, not yet triaged), `fixed`, `skipped`, `deferred`.
Set `deferred` by hand and put the reason in the Source cell; both are preserved. A `|` in the reason is kept as prose and escaped on the next run.
Re-running the gate keeps every row it can. A row the current review no longer reports is kept and its Source cell flagged, so a finding does not leave this record silently. ONE exception: when a finding id is REUSED by a different finding, the earlier decision cannot keep a row — the id is taken — and it is dropped. A RECORDED decision (anything but `open`) is named on the console when that happens; a row still at `open` is replaced silently, because `open` records no decision to lose.
