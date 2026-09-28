---
phase: 06-query-understanding
reviewed: 2026-09-28T17:15:00Z
depth: standard
files_reviewed: 41
files_reviewed_list:
  - .claude/skills/engram-connect-client/SKILL.md
  - .claude/skills/engram-console-conventions/SKILL.md
  - Taskfile.yaml
  - charts/engram/templates/_helpers.tpl
  - charts/engram/values.yaml
  - docs-site/src/content/docs/guides/configure.md
  - docs-site/src/content/docs/guides/deploy.md
  - docs-site/src/content/docs/guides/upgrade.md
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/config/registry.go
  - internal/config/search_config_test.go
  - internal/config/search_docs_test.go
  - internal/config/service_auth_test.go
  - internal/config/understanding_config_test.go
  - internal/config/understanding_docs_test.go
  - internal/config/validate.go
  - internal/config/validate_test.go
  - internal/e2e/console_browser_test.go
  - internal/server/connectapi.go
  - internal/server/connectcsrf_test.go
  - internal/server/decider.go
  - internal/server/decider_test.go
  - internal/server/tools.go
  - internal/server/tools_test.go
  - internal/server/understand.go
  - internal/server/understand_audit_test.go
  - internal/server/understand_test.go
  - internal/understand/report.go
  - internal/understand/tags.go
  - internal/understand/tags_test.go
  - internal/understand/understand.go
  - internal/understand/understand_test.go
  - internal/understand/window.go
  - internal/understand/window_test.go
  - proto/engram/v1/engram.proto
  - ui/src/lib/a11y/surfaces.browser.test.ts
  - ui/src/lib/components/SuggestedRow.browser.test.ts
  - ui/src/lib/components/SuggestedRow.svelte
  - ui/src/lib/search/understand.test.ts
  - ui/src/lib/search/understand.ts
  - ui/src/routes/search/+page.svelte
  - ui/src/routes/search/search.browser.test.ts
findings:
  critical: 0
  warning: 0
  info: 2
  total: 2
status: issues_found
---

# Phase 6: Code Review Report

**Reviewed:** 2026-09-28T17:15:00Z
**Depth:** standard
**Files Reviewed:** 41 (of 43 listed; see Notes)
**Status:** issues_found

## Summary

This is a re-review (--auto iteration 2) verifying the two fixes applied against the prior
review's findings (WR-01, WR-02; `06-REVIEW-FIX.md`, commits `dc939fa7` and `7118877b`) and
re-checking the full phase scope for regressions or newly-introduced defects.

**WR-01 (`past_month`/`past_year` `AddDate` overflow) — confirmed fixed, no regression.**
`internal/understand/window.go`'s new `subtractCalendarClamped`/`lastDayOfMonth` helpers were
traced by hand against the two pinned overflow cases (`2026-05-31` past_month → `2026-04-30`,
`2028-02-29` past_year → `2027-02-28`) and three additional cases not in the test suite:
a January-crossing borrow (`2026-01-31` past_month → `2025-12-31`), a non-leap-year February
clamp (`2026-03-31` past_month → `2026-02-28`), and a leap-year February no-clamp
(`2028-03-31` past_month → `2028-02-29`) — all correct. `go build ./...`, `go vet
./internal/server/... ./internal/understand/...`, and `go test ./internal/understand/...
./internal/server/... -run 'Window|Understand'` were independently re-run and are clean/green,
including the two new `TestWindow` overflow sub-tests. The fix does not touch the RFC3339/UTC
encoding contract (D-07/D-11) — `t.Format(time.RFC3339)` is unchanged — so FacetStrip's
byte-for-byte match still holds.

**WR-02 (`CrossSpine` unconsulted) — confirmed fixed, no regression.** The doc comment added to
`understandArgs.CrossSpine` in `internal/server/understand.go` accurately states the intentional
non-consumption and the reasoning. Verified independently: the proto file
(`proto/engram/v1/engram.proto:759-767`, `UnderstandQueryRequest`) carries no comment on
`cross_spine = 3` (confirming the fixer's claim that the proto-comment half of the change was
reverted to avoid tripping `RuleScopeRequiredUnlessCrossSpine`), and
`go test ./internal/surfaces/... -run '^TestSurfaceConformanceProseFiles$'` passes. The UI's
`understandQueryRequest` (`ui/src/lib/search/understand.ts:22-34`) still sends `crossSpine`
per the wire contract; the field being server-side-documented-but-unread is consistent end to
end and not a functional bug.

Neither fix touched files outside its stated scope (`git show --stat` confirms `dc939fa7` only
touched `window.go`/`window_test.go`, `7118877b` only touched `understand.go`), and no new
BLOCKER/WARNING-tier issues were found across the rest of the phase's scope on this pass. The
two prior Info-tier findings (IN-01, IN-02) were explicitly out of the fixer's `fix_scope:
critical_warning` and remain present, unchanged, below.

## Info

### IN-01: `Result.Audit` logs the trimmed query, not "as received" as documented

**File:** `internal/understand/report.go:68-77`, `internal/server/understand.go:86,124`
**Issue:** `Audit`'s doc comment states it logs "the query text (verbatim, as received — not the
MaxQueryChars-truncated text the decision call sees)". The caller (`understandQuery`) actually
passes `q := strings.TrimSpace(a.Query)` to `rep.Audit(ctx, q)`, so any leading/trailing
whitespace present in the raw wire request is stripped before it reaches the audit log — it is
verbatim only up to trimming, not truly "as received". Low impact (whitespace trimming is
unlikely to matter to an offline grader), but the doc comment overstates the guarantee.
**Fix:** Either pass `a.Query` (untrimmed) to `rep.Audit(ctx, ...)`, or soften the doc comment to
say "as received, up to leading/trailing whitespace trimming."

### IN-02: `MatchTags` and `NewRequest` run against different query lengths

**File:** `internal/understand/understand.go:321-327`
**Issue:** `Suggest` calls `MatchTags(in.Query, in.Tags, in.Applied.Tags)` with the full,
untruncated query, while `NewRequest` truncates the text sent to the decision provider to
`MaxQueryChars` (2000 code points) via `verdict.State`. For a query longer than 2000 code points,
tag matching considers tokens beyond what the decider ever saw, so a tag suggestion could fire
based on a token that was never part of the "state" the model reasoned over. This is a very
unlikely edge case in a console search box, and it's not a functional bug (tag matching is
local, not decided), but it is a minor inconsistency between the two suggestion paths worth a
one-line comment noting it's intentional (tag matching not being decision-provider-bound), since
D-09 already established tags are exempt from decide's contract — this is more a
documentation/consistency nit than a behavioral issue.
**Fix:** Optional — add a short comment at the `MatchTags` call site noting it deliberately
operates on the untruncated query (tags aren't sent to the provider, so `MaxQueryChars` doesn't
apply to them), to head off future confusion.

## Notes

Two of the 43 listed files were not distinct readable artifacts from what was already covered
(overlap with files read in full for the Search/Understanding config surface), consistent with
the prior iteration's Notes.

No BLOCKER-tier findings and no WARNING-tier findings remain: both `06-REVIEW.iter2.md` warnings
(WR-01, WR-02) were independently re-verified as correctly and completely fixed, with no
regressions introduced in `window.go`, `window_test.go`, or `understand.go`, and no new
correctness, security, or robustness defects were found elsewhere in the reviewed scope on this
pass.

---

_Reviewed: 2026-09-28T17:15:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
