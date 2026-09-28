---
phase: 06-query-understanding
reviewed: 2026-09-28T16:33:46Z
depth: standard
files_reviewed: 34
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
  warning: 2
  info: 2
  total: 4
status: issues_found
---

# Phase 6: Code Review Report

**Reviewed:** 2026-09-28T16:33:46Z
**Depth:** standard
**Files Reviewed:** 34 (of 43 listed; see Notes)
**Status:** issues_found

## Summary

This phase adds the advisory query-understanding path (`UnderstandQuery` Connect RPC,
`internal/understand`, and the `/search` Suggested row) fairly cleanly against the locked
D-01..D-17 decisions in `06-CONTEXT.md`. The implementation and its very extensive test suite
(server-side outcome/telemetry/audit tests, config registry/validate/docs gate tests, and
browser-level a11y/keyboard tests) agree with each other on almost every documented contract:
the off-path never calls the store or the decider, the audit flag never leaks query text without
being explicitly on, span telemetry never carries the query/scope/tag, the CSRF write-list
correctly excludes this RPC as a read, and the Helm chart / docs-site pages are kept in lockstep
with the registry via dedicated docs-gate tests.

I found one real correctness bug worth fixing before this ships (the `past_month`/`past_year`
time-window calculation can silently collapse to a 1-3 day window on certain calendar dates due
to Go's `time.AddDate` end-of-month rollover, and it is untested against that case), one design
smell around an unused request field, and two minor documentation/logging nits.

## Warnings

### WR-01: `past_month`/`past_year` time-window suggestion can silently shrink to a few days

**File:** `internal/understand/window.go:58-61`
**Issue:** `Window` computes the "past month" bound as `midnight.AddDate(0, -1, 0)` and "past
year" as `midnight.AddDate(-1, 0, 0)`. Go's `time.AddDate` does not clamp an overflowing day —
it normalizes by rolling into the *following* month. For any "now" whose day-of-month is larger
than the previous month's length (i.e. any `now` on the 29th-31st where the prior month is
shorter, e.g. March 29/30/31, May 29/30/31, July 29/30/31, October 29/30/31, December 29/30/31),
`AddDate(0, -1, 0)` overshoots forward instead of landing near the start of the previous month.
For example, `time.Date(2026, 5, 31, ...).AddDate(0, -1, 0)` yields `2026-05-01` (April has only
30 days, so day 31 rolls over into May), not late April as a user would expect. A decided
"past month" suggestion accepted on such a day would silently apply a filter covering only
~30 hours instead of ~30 days, contradicting its own chip label and hiding almost everything a
user actually wants. `past_year` has the same failure mode around leap-day boundaries
(`AddDate(-1,0,0)` from Feb 29 in a leap year lands on Mar 1 of the prior non-leap year). This
is untested: `window_test.go`'s fixed `now` is 2026-09-28, which never exercises the overflow
path for any bucket.
**Fix:** Compute the boundary via day-count subtraction (or explicitly clamp) instead of
`AddDate` month/year arithmetic, e.g.:
```go
case "past_month":
    t = midnight.AddDate(0, -1, 0)
    if t.After(midnight.AddDate(0, 0, -28)) { // overflowed forward past the sane range
        t = time.Date(nowUTC.Year(), nowUTC.Month()-1, 1, 0, 0, 0, 0, time.UTC)
    }
```
or simplest: pick a fixed day count for "past month" (e.g. 30 days) the way "past week" already
uses a fixed 7-day count, sidestepping calendar-month arithmetic entirely. Add a test case with
`now` set to a date whose previous month is shorter (e.g. `2026-05-31`) to pin the fix.

### WR-02: `UnderstandQueryRequest.cross_spine` is threaded through but never consulted

**File:** `internal/server/understand.go:31-118`, `proto/engram/v1/engram.proto:762`
**Issue:** `understandArgs.CrossSpine` is populated from the wire request
(`connectapi.go:692`) but is never read anywhere in `understandQuery`, `understand.NewRequest`,
or `understand.Applied` (which has no `CrossSpine` field at all). The D-08 scope-suggestion gate
only checks `a.Scope == ""`, regardless of whether the caller already broadened to cross-spine.
This may be an intentional simplification (a scope suggestion is still useful even when the user
is already searching cross-spine), but as written the field is dead code on the server — nothing
distinguishes "no scope, single-spine" from "no scope, cross-spine" for suggestion purposes, and
a future reader of `understandArgs` may reasonably assume it does something because it exists
and is plumbed all the way from the proto request.
**Fix:** Either wire `CrossSpine` into the D-08 gate if that was the intent (e.g. skip the scope
question when the caller has already explicitly gone cross-spine, since suggesting one scope
would narrow rather than clarify), or drop the field from `understandArgs`/document explicitly
in the doc comment above `understandArgs` *why* it is accepted but intentionally ignored, so the
next reader doesn't have to rediscover this by tracing the whole call chain.

## Info

### IN-01: `Result.Audit` logs the trimmed query, not "as received" as documented

**File:** `internal/understand/report.go:68-77`, `internal/server/understand.go:76,113-115`
**Issue:** `Audit`'s doc comment states it logs "the query text (verbatim, as received — not the
MaxQueryChars-truncated text the decision call sees)". The caller (`understandQuery`) actually
passes `q := strings.TrimSpace(a.Query)`, so any leading/trailing whitespace present in the raw
wire request is stripped before it reaches the audit log — it is verbatim only up to trimming,
not truly "as received". Low impact (whitespace trimming is unlikely to matter to an offline
grader), but the doc comment overstates the guarantee.
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

Nine of the 43 listed `required_reading` files were not present as distinct readable artifacts
relative to what was reviewed above; their content overlaps entirely with files already read in
full (e.g. `internal/config/config.go`/`validate.go`/`registry.go` were read for the Search/
Understanding fields; `config_test.go`/`validate_test.go`/`search_config_test.go`/
`search_docs_test.go`/`service_auth_test.go` were grepped and spot-read for
understanding-specific assertions rather than read end-to-end, since they are large
pre-existing files where only a small, already-verified slice is new to this phase). No
additional findings emerged from the grepped portions beyond what is reported above.

No BLOCKER-tier findings: the off-switch, CSRF exclusion, audit-flag gating, span telemetry
scrubbing, and Helm/docs consistency are all correct and are each backed by a dedicated,
specific test that would fail if the guarantee regressed.

---

_Reviewed: 2026-09-28T16:33:46Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
