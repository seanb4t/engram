---
phase: 06-query-understanding
fixed_at: 2026-09-28T16:48:52Z
review_path: .planning/phases/06-query-understanding/06-REVIEW.md
iteration: 1
findings_in_scope: 2
fixed: 2
skipped: 0
status: all_fixed
---

# Phase 6: Code Review Fix Report

**Fixed at:** 2026-09-28T16:48:52Z
**Source review:** .planning/phases/06-query-understanding/06-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 2 (WR-01, WR-02 — `fix_scope: critical_warning`; the two Info findings were out of scope)
- Fixed: 2
- Skipped: 0

Verification ran in the main checkout on branch `feat/2026-09-25.01` (no isolated worktree — the
orchestrator assigned this working directory directly and stated single-writer safety for this
run), so the gate results below are reproducible from the tree as committed.

## Fixed Issues

### WR-01: `past_month`/`past_year` time-window suggestion can silently shrink to a few days

**Files modified:** `internal/understand/window.go`, `internal/understand/window_test.go`
**Commit:** `dc939fa7`
**Applied fix:** Replaced `midnight.AddDate(0, -1, 0)` / `midnight.AddDate(-1, 0, 0)` with a new
`subtractCalendarClamped` helper (plus `lastDayOfMonth`) that subtracts calendar months/years by
computing the target year/month directly and clamping the day-of-month to the target month's last
day when it overflows — instead of letting `time.Time.AddDate` normalize the overflow forward into
the following month. This keeps the UTC-midnight RFC3339 encoding byte-identical to FacetStrip's
own date-input encoding (D-07's wire contract, `06-01-SUMMARY.md` key-decisions item 2) while fixing
the date math itself, exactly as `06-VERIFICATION.md`'s gap description required. Pinned with two
new `window_test.go` cases per the required_reading instructions: `now = 2026-05-31` for `past_month`
(April has 30 days; expected `2026-04-30T00:00:00Z`, not the `AddDate`-rollover `2026-05-01`) and
`now = 2028-02-29` (a leap day) for `past_year` (2027 is not a leap year; expected
`2027-02-28T00:00:00Z`, not the rollover `2027-03-01`). All existing `TestWindow` sub-tests
(today/past_week/past_month/past_year at the original fixed `now`, the non-UTC-day case, and the
unrecognized-bucket case) still pass unchanged.

### WR-02: `UnderstandQueryRequest.cross_spine` is threaded through but never consulted

**Files modified:** `internal/server/understand.go`
**Commit:** `7118877b`
**Applied fix:** Documented the intentional non-consumption at the `understandArgs.CrossSpine`
field declaration: it is carried from the wire request per D-03/Task-1-option-a item 5, but the
D-08 scope-suggestion gate deliberately checks only whether `Scope` is empty, since suggesting a
single scope would narrow rather than clarify a query the caller already broadened to cross-spine.
The task instructions specifically called out documenting this at the decode/declare site rather
than removing the field. I evaluated also adding the equivalent comment to
`proto/engram/v1/engram.proto`'s `UnderstandQueryRequest.cross_spine` field (regenerating via
`task proto:gen` as instructed if the proto comment changed) but reverted that half of the change:
`internal/surfaces` runs a proto-comment conformance gate
(`RuleScopeRequiredUnlessCrossSpine`, `internal/surfaces/rules.go:230`) that treats **any** commented
`cross_spine`/`scope` proto field as asserting the canonical sentence "scope is required unless
cross_spine is true" — which is not true for this RPC (`scope` is optional context here, not a
required-unless-cross-spine field). The original field had no leading comment at all, so it was
correctly excluded from that gate's field-matching; adding one would have made
`TestSurfaceConformanceProseFiles` fail on a true statement being asserted falsely by a shared,
field-name-keyed rule. Confirmed by running the gate before and after: failed with the proto
comment added (`rule=scope-required-unless-cross-spine surface=proto_comment expected="scope is
required unless cross_spine is true" found="... cross_spine carries the caller's cross-spine search
state ..."`), passed once the proto comment was reverted and only the Go-side `understandArgs`
struct comment carries the documentation. `task proto:gen` was run to confirm the reverted proto
produces a byte-identical `gen/`/`ui/src/lib/gen` tree (empty diff), so no generated-code commit was
needed.

## Skipped Issues

None — both in-scope findings (WR-01, WR-02) were fixed.

## Verification

Ran in the main checkout (`/Volumes/Code/github.com/seanb4t/engram`, branch `feat/2026-09-25.01`),
after both fix commits:

- `go build ./...` — success
- `go vet ./internal/server/... ./internal/understand/...` — clean
- `go test ./internal/understand/... ./internal/server/... -run 'Window|Understand' -v` — all PASS,
  including the two new `TestWindow` overflow sub-tests
  (`past_month_clamps_instead_of_rolling_forward_on_a_month-length_overflow_date`,
  `past_year_clamps_a_leap-day_now_instead_of_rolling_forward`)
- `go test ./internal/surfaces/... -run '^TestSurfaceConformanceProseFiles$' -v` — PASS (run before
  and after the proto-comment revert to confirm the WR-02 fix does not regress this gate)
- `go tool buf breaking --against '.git#branch=main'` — clean (run while the proto comment change
  was still present, before it was reverted; confirmed no field-number/name break)
- `task lint` (golangci-lint, rumdl, yamlfmt, actionlint, ruff) — all checks passed

Both findings from the IN-01/IN-02 (Info tier) were out of `fix_scope: critical_warning` and were
not touched.

---

_Fixed: 2026-09-28T16:48:52Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
