---
phase: 06-query-understanding
verified: 2026-09-28T16:53:51Z
status: passed
score: 4/4 must-haves verified
covered_files: [".planning/phases/06-query-understanding/06-01-PLAN.md", ".planning/phases/06-query-understanding/06-01-SUMMARY.md", ".planning/phases/06-query-understanding/06-02-PLAN.md", ".planning/phases/06-query-understanding/06-02-SUMMARY.md", ".planning/phases/06-query-understanding/06-03-PLAN.md", ".planning/phases/06-query-understanding/06-03-SUMMARY.md", ".planning/phases/06-query-understanding/06-04-PLAN.md", ".planning/phases/06-query-understanding/06-04-SUMMARY.md", ".planning/phases/06-query-understanding/06-05-PLAN.md", ".planning/phases/06-query-understanding/06-05-SUMMARY.md", ".planning/phases/06-query-understanding/06-06-PLAN.md", ".planning/phases/06-query-understanding/06-06-SUMMARY.md", ".planning/phases/06-query-understanding/06-REVIEW-DISPOSITION.md", ".planning/phases/06-query-understanding/06-REVIEW-FIX.iter2.md", ".planning/phases/06-query-understanding/06-REVIEW-FIX.md", ".planning/phases/06-query-understanding/06-REVIEW.iter2.md", ".planning/phases/06-query-understanding/06-REVIEW.md", "charts/engram/templates/_helpers.tpl", "charts/engram/values.yaml", "docs-site/src/content/docs/guides/configure.md", "docs-site/src/content/docs/guides/deploy.md", "docs-site/src/content/docs/guides/upgrade.md", "internal/config/config.go", "internal/config/registry.go", "internal/config/validate.go", "internal/e2e/console_browser_test.go", "internal/server/connectapi.go", "internal/server/decider.go", "internal/server/tools.go", "internal/server/understand.go", "internal/understand/report.go", "internal/understand/tags.go", "internal/understand/understand.go", "internal/understand/window.go", "proto/engram/v1/engram.proto", "ui/src/lib/components/SuggestedRow.svelte", "ui/src/lib/search/understand.ts", "ui/src/routes/search/+page.svelte"]
covered_digest: "v2:sha256:b7ea9b87bab56fa944282bb684ebc3e04e81c280a7f75567995a93d5f742f9a9"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 3/4
  gaps_closed:
    - "SC2 (D-07): the time-window suggestion converts the chosen bucket to created_after using the request day's UTC midnight minus one calendar month / one calendar year for past_month/past_year — the AddDate month/year-overflow defect (WR-01) is fixed and now tested against the overflow cases."
  gaps_remaining: []
  regressions: []
---

# Phase 6: Query Understanding Verification Report

**Phase Goal:** An operator gets NL-query-understanding filter chips (on by default when a decisions provider is configured, explicitly disableable) that never change results until confirmed, with zero behavioral or logging change when the capability is off.
**Verified:** 2026-09-28T16:53:51Z
**Status:** passed
**Re-verification:** Yes — after gap closure

## Goal Achievement

### Gap Closure Verification (WR-01)

The previous verification's single gap was a code-review-confirmed defect in
`internal/understand/window.go`: `time.Time.AddDate(0,-1,0)`/`AddDate(-1,0,0)`
overshoots forward on day-of-month overflow dates (the 29th-31st where the
prior month is shorter, plus Feb 29 leap-day boundaries), silently shrinking a
"past month"/"past year" suggested window to ~1-3 days while the chip label
stayed unchanged.

**Fix commit:** `dc939fa7` — replaced the two `AddDate` calls with a new
`subtractCalendarClamped` helper (`internal/understand/window.go:82-99`) that
computes the target year/month directly and clamps the day-of-month to the
target month's last day (`lastDayOfMonth`, a standard "day 0 of next month"
idiom) instead of letting `AddDate` roll the overflow forward into the
following month.

**Independent test execution (not taken from SUMMARY/REVIEW-FIX claims):**

```
$ go test ./internal/understand/... -run TestWindow -v
--- PASS: TestWindow (0.00s)
    --- PASS: TestWindow/today (0.00s)
    --- PASS: TestWindow/past_week (0.00s)
    --- PASS: TestWindow/past_month (0.00s)
    --- PASS: TestWindow/past_year (0.00s)
    --- PASS: TestWindow/non-UTC_now_resolves_to_the_UTC_calendar_day (0.00s)
    --- PASS: TestWindow/past_month_clamps_instead_of_rolling_forward_on_a_month-length_overflow_date (0.00s)
    --- PASS: TestWindow/past_year_clamps_a_leap-day_now_instead_of_rolling_forward (0.00s)
    --- PASS: TestWindow/unrecognized_buckets_report_ok_false (0.00s)
        --- PASS: TestWindow/unrecognized_buckets_report_ok_false/none (0.00s)
        --- PASS: TestWindow/unrecognized_buckets_report_ok_false/yesterday (0.00s)
ok  	github.com/seanb4t/engram/internal/understand	(cached)
```

The two new sub-tests directly pin the previously-open overflow cases:
- `past_month_clamps_instead_of_rolling_forward_on_a_month-length_overflow_date`:
  `now = 2026-05-31` (April has 30 days) asserts `after = 2026-04-30T00:00:00Z`
  — not the `AddDate`-rollover `2026-05-01T00:00:00Z` the old code produced.
- `past_year_clamps_a_leap-day_now_instead_of_rolling_forward`:
  `now = 2028-02-29` (leap day; 2027 is not a leap year) asserts
  `after = 2027-02-28T00:00:00Z` — not the `AddDate`-rollover
  `2027-03-01T00:00:00Z`.

Both RUN/PASS pairs are present in the output above (0 fails, 0 skips). I
read `window.go` directly (not just the test) and confirmed
`subtractCalendarClamped` computes `y, m, d := midnight.Date()`, subtracts
years/months with underflow borrowing, and clamps `d` to
`lastDayOfMonth(y, targetMonth)` before constructing the result — the fix is
a real algorithmic change, not a test-only patch. `git status --short` shows
no uncommitted changes to `window.go`/`window_test.go`, confirming the fix as
committed (`dc939fa7`) is what is on disk.

**Verdict:** Gap closed. SC2/NLQ-02 is now fully satisfied.

### Regression Check (previously-passed truths, targeted -v runs)

```
$ go test ./internal/server/... -run \
  'TestUnderstandQueryOffNeverDecides|TestUnderstandDeciderBoundedNoRetry|TestUnderstandQueryNoQueryTextWithoutAudit|TestUnderstandQuerySpanTelemetry|TestBuildDepsFromEnvUnderstandingDefaultFollowsProvider|TestUnderstandingEnabledLogLine|TestUnderstandQueryDecisionFailureZeroSuggestions|TestUnderstandQueryAuditLogsQueryVerbatim|TestUnderstandingAuditIndependentOfRerankAudit|TestUnderstandQueryAuditEmptyQueryLogsNothing' -v
```
All named sub-tests PASS (10 top-level tests, all sub-cases), 0 fails. `ok  	github.com/seanb4t/engram/internal/server	2.939s`.

```
$ go test ./internal/config/... -run 'TestUnderstandingVarsDocumented|TestSearchVarsDocumented|TestUnderstandingConfigValidate' -v
```
All PASS, including the full `TestUnderstandingConfigValidate` table (valid combos, bad-value rejection, jev-without-provider, timeout gating on/off, audit rejection) and `TestUnderstandingVarsDocumented`'s registry/disclosure-anchor checks.

```
$ pnpm vitest run -t "accepts a suggested category chip to the same URL and SearchMemories calls the manual FacetStrip chip produces"
Test Files  1 passed | 69 skipped (70)
     Tests  1 passed | 1051 skipped (1052)
```
Confirms Truth 3 (accepted chip indistinguishable from a manual one) still holds; no regression from the WR-01/WR-02 fix commits, which did not touch any UI file.

No regressions found. All four previously-VERIFIED truths remain verified.

### Observable Truths

| # | Truth (ROADMAP Success Criterion) | Status | Evidence |
|---|---|---|---|
| 1 | Query understanding defaults on when `ENGRAM_DECISIONS_PROVIDER=jev` (explicit `off` disables it, startup discloses when on); off = no decision call, byte-identical behavior/config | ✓ VERIFIED | `understandingEnabled` (internal/server/decider.go:303-314) resolves `""`→follows provider, `"off"`/`"jev"`→explicit, unknown→off with Warn. `understandQuery` (internal/server/understand.go:69-71) short-circuits on `d.understandDec == nil` before any validation/store/decide call. `Config.Validate` rejects `jev` without a provider and any non-`off`/`jev`/empty value. Re-run live: `TestUnderstandQueryOffNeverDecides`, `TestBuildDepsFromEnvUnderstandingDefaultFollowsProvider`, `TestUnderstandingEnabledLogLine`, `TestUnderstandingConfigValidate` — all PASS. |
| 2 | With it on, a prose query yields suggested filter chips (categories, time window, tags, scope) from a server-side `UnderstandQuery` RPC within a bounded no-retry timeout; a decision failure/timeout yields zero suggestions and never fails/delays the search | ✓ VERIFIED | All four suggestion kinds implemented and tested (categories, scope via D-08 gate, tags via `MatchTags`, time window via `Window`). Bounded no-retry client proven by `TestUnderstandDeciderBoundedNoRetry` (re-run: PASS, hung-server sub-case 0.15s). Failure classes proven zero-suggestion via `TestUnderstandQueryDecisionFailureZeroSuggestions` (re-run: PASS). Live e2e round trip via `TestConsoleQueryUnderstanding`. **Previously-open gap (WR-01, time-window date overflow) is now fixed and independently confirmed above** — the chip's stated date range is now correct on every calendar date, not just the common case. |
| 3 | A test proves search results are unchanged until a suggested chip is clicked, and an accepted chip is indistinguishable from a manually added one | ✓ VERIFIED | `ui/src/routes/search/+page.svelte` wires `SuggestedRow`'s `onchange` to the identical `navigate({ ...partial, sel: '' })` callback `FacetStrip` uses. Re-run live: `search.browser.test.ts > accepts a suggested category chip to the same URL and SearchMemories calls the manual FacetStrip chip produces` — 1 passed, 0 failed. |
| 4 | A test proves no query text appears in logs unless the explicit opt-in audit flag (mirroring `ENGRAM_SEARCH_RERANK_AUDIT`) is set | ✓ VERIFIED | `ENGRAM_SEARCH_UNDERSTANDING_AUDIT` gate implemented (decider.go, report.go). Re-run live: `TestUnderstandQueryNoQueryTextWithoutAudit`, `TestUnderstandQueryAuditLogsQueryVerbatim`, `TestUnderstandingAuditIndependentOfRerankAudit`, `TestUnderstandQueryAuditEmptyQueryLogsNothing` — all PASS. `TestUnderstandQuerySpanTelemetry` re-run PASS, confirming span attributes never carry query/scope/tag. |

**Score:** 4/4 truths verified (0 present-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `proto/engram/v1/engram.proto` | `SuggestionSource`, `TimeWindowSuggestion`, `FilterSuggestion`, `UnderstandQueryRequest/Response`, `rpc UnderstandQuery` | ✓ VERIFIED | Unchanged since initial verification (WR-02's proto-comment addition was reverted per 06-REVIEW-FIX.md; `task proto:gen` confirmed byte-identical `gen/` tree). |
| `internal/understand/window.go` | D-07 bucket vocabulary + conversion | ✓ VERIFIED | Now correct on every calendar date via `subtractCalendarClamped`/`lastDayOfMonth`; previously ⚠️ VERIFIED-WITH-DEFECT, gap closed this round. |
| `internal/understand/understand.go`, `tags.go`, `internal/server/understand.go`, `decider.go` | (see initial verification) | ✓ VERIFIED | No changes since initial verification other than the WR-02 doc-comment addition to `understand.go` (documents intentional non-consumption of `CrossSpine`); re-confirmed present and wired. |
| `internal/config/understanding_config_test.go`, `understanding_docs_test.go` | Registry/validate/docs gates | ✓ VERIFIED | Re-run live: `TestUnderstandingConfigValidate`, `TestUnderstandingVarsDocumented` — all PASS. |
| `ui/src/lib/search/understand.ts`, `SuggestedRow.svelte`, `+page.svelte` | Suggested-row UI | ✓ VERIFIED | Unchanged since initial verification; no UI files touched by the WR-01/WR-02 fix commits (`git log` on these paths shows no new commits post-initial-verification). |
| `internal/e2e/console_browser_test.go` | `TestConsoleQueryUnderstanding` | ✓ VERIFIED | Unchanged; present and matching the plan's tracer scenario. |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|---|---|---|---|---|
| — | — | `rg -n "TBD\|FIXME\|XXX"` across `window.go`, `window_test.go`, `understand.go`, `engram.proto` | — | No matches — clean. |
| `internal/understand/report.go` / `internal/server/understand.go` | IN-01 | `Audit` logs the trimmed query, doc comment says "as received" | ℹ️ Info | Disposition: `open` (Info-tier, out of `fix_scope: critical_warning`, not a must-have). Does not affect goal achievement. |
| `internal/understand/understand.go` | IN-02 | `MatchTags` runs against untruncated query vs. `NewRequest`'s truncated one | ℹ️ Info | Disposition: `open` (Info-tier). Does not affect goal achievement. |

Per `06-REVIEW-DISPOSITION.md`: WR-01 and WR-02 (the two Warning-tier findings, the only ones capable of blocking a must-have) are both `fixed`. The two remaining `open` items are Info-tier documentation nits that do not bear on any of the four observable truths.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|---|---|---|---|---|
| NLQ-01 | 06-01, 06-04, 06-06 | Default-on/off resolution, startup disclosure, byte-identical off behavior | ✓ SATISFIED | Truth #1, re-confirmed. |
| NLQ-02 | 06-01, 06-02, 06-05 | `UnderstandQuery` RPC yields chips under a bounded no-retry timeout; failure/timeout → zero suggestions | ✓ SATISFIED | Truth #2, gap closed this round (WR-01). |
| NLQ-03 | 06-03, 06-06 | Suggested/unapplied chips; results unchanged until accepted; accepted == manual | ✓ SATISFIED | Truth #3, re-confirmed. |
| NLQ-04 | 06-04, 06-05, 06-06 | Query text never logged/exported unless opt-in audit flag set | ✓ SATISFIED | Truth #4, re-confirmed. |

No orphaned requirements: REQUIREMENTS.md maps exactly NLQ-01..04 to Phase 6, all four appear in plan frontmatter `requirements:` fields and are marked `Mapped`.

### Gaps Summary

None. The one gap from the initial verification (WR-01, `internal/understand/window.go` date-overflow defect) is closed: the fix (`subtractCalendarClamped`/`lastDayOfMonth`, commit `dc939fa7`) was independently re-derived by reading the code, and the two new pinning test cases were independently re-run with visible RUN/PASS pairs (not taken on the SUMMARY's or REVIEW-FIX's word). All previously-verified truths were spot-checked with targeted `-v`/`-t` runs and show no regressions. The phase goal is achieved: query understanding is on-by-default/explicitly-disableable with byte-identical off behavior, produces correctly-dated suggested chips under a bounded no-retry timeout that never fail or delay search, chips never change results until accepted and are then indistinguishable from manual ones, and query text is never logged without the explicit opt-in audit flag.

No human verification items required.

---

_Verified: 2026-09-28T16:53:51Z_
_Verifier: Claude (gsd-verifier)_
