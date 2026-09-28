---
phase: 06-query-understanding
verified: 2026-09-28T20:15:00Z
status: gaps_found
score: 3/4 must-haves verified
covered_files: [".planning/phases/06-query-understanding/06-01-PLAN.md", ".planning/phases/06-query-understanding/06-01-SUMMARY.md", ".planning/phases/06-query-understanding/06-02-PLAN.md", ".planning/phases/06-query-understanding/06-02-SUMMARY.md", ".planning/phases/06-query-understanding/06-03-PLAN.md", ".planning/phases/06-query-understanding/06-03-SUMMARY.md", ".planning/phases/06-query-understanding/06-04-PLAN.md", ".planning/phases/06-query-understanding/06-04-SUMMARY.md", ".planning/phases/06-query-understanding/06-05-PLAN.md", ".planning/phases/06-query-understanding/06-05-SUMMARY.md", ".planning/phases/06-query-understanding/06-06-PLAN.md", ".planning/phases/06-query-understanding/06-06-SUMMARY.md", "charts/engram/templates/_helpers.tpl", "charts/engram/values.yaml", "docs-site/src/content/docs/guides/configure.md", "docs-site/src/content/docs/guides/deploy.md", "docs-site/src/content/docs/guides/upgrade.md", "internal/config/config.go", "internal/config/registry.go", "internal/config/validate.go", "internal/e2e/console_browser_test.go", "internal/server/connectapi.go", "internal/server/decider.go", "internal/server/tools.go", "internal/server/understand.go", "internal/understand/report.go", "internal/understand/tags.go", "internal/understand/understand.go", "internal/understand/window.go", "proto/engram/v1/engram.proto", "ui/src/lib/components/SuggestedRow.svelte", "ui/src/lib/search/understand.ts", "ui/src/routes/search/+page.svelte"]
covered_digest: "v2:sha256:ac415e4b8d9368d969925bb3738c2d010456eae3ec5d7b0076861c5bb410df5a"
behavior_unverified: 0
overrides_applied: 0
gaps:
  - truth: "SC2 (D-07): the time-window suggestion converts the chosen bucket to created_after using the request day's UTC midnight minus one calendar month / one calendar year for past_month/past_year"
    status: partial
    reason: "internal/understand/window.go computes past_month/past_year via time.Time.AddDate(0,-1,0)/AddDate(-1,0,0), which does not clamp day-of-month overflow. On any 'now' whose day-of-month exceeds the target month's length (29th-31st, ~15-16 calendar dates/year, plus leap-day Feb 29), the suggested window silently shrinks to ~1-3 days instead of ~30/365 days while the chip label still reads 'past month'/'past year'. Confirmed by direct code read (window.go:58-61) and by the code-review report (06-REVIEW.md WR-01, disposition 'open' as of verification time). window_test.go's TestWindow fixes now at 2026-09-28 and never exercises the overflow path, so no test guards this."
    artifacts:
      - path: "internal/understand/window.go"
        issue: "AddDate(0,-1,0)/AddDate(-1,0,0) overshoots forward on month/year-length overflow dates instead of landing near the start of the previous month/year"
    missing:
      - "Compute past_month/past_year via day-count subtraction or an explicit overflow clamp (see 06-REVIEW.md WR-01 for a suggested fix), and add a window_test.go case with `now` set to a date whose previous month is shorter (e.g. 2026-05-31) or a Feb-29 leap-year `now`, to pin the fix."
---

# Phase 6: Query Understanding Verification Report

**Phase Goal:** An operator gets NL-query-understanding filter chips (on by default when a decisions provider is configured, explicitly disableable) that never change results until confirmed, with zero behavioral or logging change when the capability is off.
**Verified:** 2026-09-28T20:15:00Z
**Status:** gaps_found
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth (ROADMAP Success Criterion) | Status | Evidence |
|---|---|---|---|
| 1 | Query understanding defaults on when `ENGRAM_DECISIONS_PROVIDER=jev` (explicit `off` disables it, startup discloses when on); off = no decision call, byte-identical behavior/config | ✓ VERIFIED | `understandingEnabled` (internal/server/decider.go:303-314) resolves `""`→follows provider, `"off"`/`"jev"`→explicit, unknown→off with Warn. `understandQuery` (internal/server/understand.go:69-71) short-circuits on `d.understandDec == nil` before any validation/store/decide call. `Config.Validate` (internal/config/validate.go:401-452) rejects `jev` without a provider and any non-`off`/`jev`/empty value. `logUnderstandingEnabled`/`SearchRerankInfo`-style disclosure exists (decider.go, D-15). Tests run live and passed: `TestUnderstandQueryOffNeverDecides`, `TestBuildDepsFromEnvUnderstandingDefaultFollowsProvider`, `TestUnderstandingEnabledLogLine`, `TestUnderstandingConfigValidate`. |
| 2 | With it on, a prose query yields suggested filter chips (categories, time window, tags, scope) from a server-side `UnderstandQuery` RPC within a bounded no-retry timeout; a decision failure/timeout yields zero suggestions and never fails/delays the search | ⚠️ PARTIAL — see gap | All four suggestion kinds are implemented and independently tested: categories (`internal/understand/understand.go`, `TestSuggestCategoryPaths`), scope (D-08 gate, `TestUnderstandQueryScopeSuggestion`, `TestScopeQuestionGate`), tags (`internal/understand/tags.go`, `TestMatchTags`), time window (`internal/understand/window.go`, `TestWindow`). Dedicated no-retry, bounded client (`understandDeciderFromConfig`, `jev.WithNoRetry()`, `jev.WithTimeout`) proven bounded by `TestUnderstandDeciderBoundedNoRetry` (hung server → zero suggestions <1s, exactly one request). Failure classes proven zero-suggestion/no-RPC-error via `TestUnderstandQueryDecisionFailureZeroSuggestions`. Live e2e round trip proven by `TestConsoleQueryUnderstanding` (real Chrome + real binary + fake decisions server). **Gap:** the time-window bucket→date conversion has a proven, untested date-overflow bug (see `gaps` above) that silently shrinks "past month"/"past year" on ~15-16 dates per year — the chip is still produced (never delays or fails the search) but its stated date range is wrong on those dates. |
| 3 | A test proves search results are unchanged until a suggested chip is clicked, and an accepted chip is indistinguishable from a manually added one | ✓ VERIFIED | `ui/src/routes/search/+page.svelte` wires `SuggestedRow`'s `onchange` to the identical `navigate({ ...partial, sel: '' })` callback used by `FacetStrip` (line 622 vs 631) — same URL/request codec, no separate path. `acceptPartial` (ui/src/lib/search/understand.ts) produces the exact partial shape FacetStrip's own controls would (category/time_window/scope/tag). Ran named tests live (not full suite): `search.browser.test.ts > accepts a suggested category chip to the same URL and SearchMemories calls the manual FacetStrip chip produces` — PASS; `> hovering or focusing a suggested chip changes neither the URL nor the SearchMemories calls` — PASS. `TestConsoleQueryUnderstanding` also proves the click → `cat=decision` URL transition against a live server. |
| 4 | A test proves no query text appears in logs unless the explicit opt-in audit flag (mirroring `ENGRAM_SEARCH_RERANK_AUDIT`) is set | ✓ VERIFIED | `ENGRAM_SEARCH_UNDERSTANDING_AUDIT` gate implemented (`understandingAudit`, decider.go:405-432; `Result.Audit`, internal/understand/report.go:68-96, emits query text only when `d.understandAudit` is true). Ran live: `TestUnderstandQueryNoQueryTextWithoutAudit` (sentinel sweep through the real Connect interceptor chain across every path) — PASS; `TestUnderstandQueryAuditLogsQueryVerbatim` — PASS; `TestUnderstandingAuditIndependentOfRerankAudit` — PASS; `TestUnderstandQueryAuditEmptyQueryLogsNothing` — PASS (not re-run individually but covered in the same `-run` pass). `TestUnderstandQuerySpanTelemetry` confirms the always-on `engram.understand.*` span attributes never carry query/scope/tag. |

**Score:** 3/4 truths verified, 1 partial (Success Criterion 2, D-07 time-window date math — see Gaps)

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `proto/engram/v1/engram.proto` | `SuggestionSource`, `TimeWindowSuggestion`, `FilterSuggestion`, `UnderstandQueryRequest/Response`, `rpc UnderstandQuery` | ✓ VERIFIED | All present; `rpc UnderstandQuery(...)` appended after `ListTags` per D-02. |
| `internal/understand/understand.go` (348 lines) | 3rd `internal/decide` consumer: `NewRequest`, `FromResponse`, `Suggest`, `Suggestion`, `Applied`, `Input`, `Result` | ✓ VERIFIED | Substantive; exercised by 775-line `understand_test.go`. |
| `internal/understand/window.go` | D-07 bucket vocabulary + conversion | ⚠️ VERIFIED-WITH-DEFECT | Present, wired, tested for the common case; date-overflow defect noted above. |
| `internal/understand/tags.go` | D-09 local tag matching (`func MatchTags(`) | ✓ VERIFIED | Present, tested (`TestMatchTags`). |
| `internal/server/understand.go` (156 lines) | Shared `UnderstandQuery` core, wire shaping | ✓ VERIFIED | `understandQuery`, `understandResultToProto` present and wired from `connectapi.go`. |
| `internal/server/decider.go` (589 lines) | `understandDeciderFromConfig`, `understandingEnabled`, `understandingTimeout`, `logUnderstandingEnabled`, `understandingAudit` | ✓ VERIFIED | All present; each has a dedicated passing test. |
| `internal/config/understanding_config_test.go`, `understanding_docs_test.go` | Registry/validate/docs gates | ✓ VERIFIED | `TestUnderstandingConfigValidate`, `TestUnderstandingVarsDocumented` pass live. |
| `ui/src/lib/search/understand.ts` (132 lines) | Eligibility, query key, request/label/accept-path helpers | ✓ VERIFIED | Substantive, pure, unit-tested (`understand.test.ts`), consumed by `+page.svelte`. |
| `ui/src/lib/components/SuggestedRow.svelte` (261 lines) | Roving-tabindex Suggested row | ✓ VERIFIED | Wired into `+page.svelte` line 628-631; keyboard/visual/AA-audit browser tests pass. |
| `internal/e2e/console_browser_test.go` | `TestConsoleQueryUnderstanding` — live round trip | ✓ VERIFIED | Present, matches the plan's exact tracer scenario (fake decisions server, one request, `cat=decision` after click). |
| `internal/webauth/static/index.html` + `_app/` bundle | Vendored SPA carries the Suggested row | ✓ VERIFIED | "Suggested filter" string found in the built JS chunk (`_app/immutable/nodes/7.BpBw_wrd.js`); `git status` shows no drift against the committed vendored tree. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `internal/understand/tags.go` | `internal/server/understand.go` | shared caller-scoped `d.listTags(ctx, c, ...)` | ✓ WIRED | `understand.go:96` calls `d.listTags`, not a second store path. |
| `internal/understand/report.go` (`Stamp`) | `internal/server/understand.go` | ambient RPC span stamping | ✓ WIRED | `rep.Stamp(trace.SpanFromContext(ctx))` called unconditionally on the enabled path (understand.go:114). |
| `internal/understand/report.go` (`Audit`) | `internal/server/understand.go` | opt-in audit gate | ✓ WIRED | Called only `if d.understandAudit` (understand.go:112-114). |
| `ui/src/lib/components/SuggestedRow.svelte` (`onchange`) | `ui/src/routes/search/+page.svelte` (`navigate`) | identical partial → `navigate({...partial, sel:''})` as `FacetStrip` | ✓ WIRED | Confirmed byte-identical callback wiring at lines 622 and 631. |
| `internal/config/understanding_config_test.go` | `internal/config/validate.go` | D-01/D-01a/D-16 validation table | ✓ WIRED | `TestUnderstandingConfigValidate` passes against live `Validate`. |
| CSRF write-list | `UnderstandQuery` Procedure | absence from `csrfWriteProcedures` | ✓ VERIFIED | `csrfWriteProcedures` map (connectcsrf.go:36-46) does not include `EngramServiceUnderstandQueryProcedure` — confirmed read RPC, no CSRF gate. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| Go build compiles | `go build ./...` | success | ✓ PASS |
| Go vet on new packages | `go vet ./internal/understand/... ./internal/server/...` | clean | ✓ PASS |
| Off-path never decides | `go test ./internal/server/... -run TestUnderstandQueryOffNeverDecides -v` | PASS | ✓ PASS |
| Bounded no-retry timeout | `go test ./internal/server/... -run TestUnderstandDeciderBoundedNoRetry -v` | PASS | ✓ PASS |
| No query text without audit flag | `go test ./internal/server/... -run TestUnderstandQueryNoQueryTextWithoutAudit -v` | PASS | ✓ PASS |
| Span telemetry never carries query/scope/tag | `go test ./internal/server/... -run TestUnderstandQuerySpanTelemetry -v` | PASS | ✓ PASS |
| Default-follows-provider disclosure | `go test ./internal/server/... -run TestBuildDepsFromEnvUnderstandingDefaultFollowsProvider -v` | PASS | ✓ PASS |
| Registry/docs gates | `go test ./internal/config/... -run 'TestUnderstandingVarsDocumented\|TestSearchVarsDocumented\|TestUnderstandingConfigValidate' -v` | 3/3 PASS | ✓ PASS |
| Time-window bucket conversion | `go test ./internal/understand/... -run TestWindow -v` | PASS (all sub-tests), but fixed `now`=2026-09-28 never exercises the overflow date range | ⚠️ PASS-BUT-GAP |
| Accepted chip == manual chip (browser) | `pnpm vitest run -t "accepts a suggested category chip to the same URL and SearchMemories calls the manual FacetStrip chip produces"` | 1 passed | ✓ PASS |
| No behavior change on hover/focus (browser) | `pnpm vitest run -t "hovering or focusing a suggested chip changes neither the URL nor the SearchMemories calls"` | 1 passed | ✓ PASS |
| Roving-tabindex toolbar role (browser) | `pnpm vitest run -t "the row is role=\"toolbar\" with aria-label \"Suggested filters\""` | 1 passed | ✓ PASS |
| AA audit of the 4-kind Suggested row (browser) | `pnpm vitest run -t "a four-kind Suggested row passes the AA audit in both themes"` | 1 passed | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|---|---|---|---|---|
| NLQ-01 | 06-01, 06-04, 06-06 | Default-on/off resolution, startup disclosure, byte-identical off behavior | ✓ SATISFIED | See Truth #1. |
| NLQ-02 | 06-01, 06-02, 06-05 | `UnderstandQuery` RPC yields chips under a bounded no-retry timeout; failure/timeout → zero suggestions, never fails/delays search | ⚠️ PARTIALLY SATISFIED | See Truth #2 / gap (D-07 date math). |
| NLQ-03 | 06-03, 06-06 | Suggested/unapplied chips; results unchanged until accepted; accepted == manual | ✓ SATISFIED | See Truth #3. |
| NLQ-04 | 06-04, 06-05, 06-06 | Query text never logged/exported unless opt-in audit flag set | ✓ SATISFIED | See Truth #4. |

No orphaned requirements: REQUIREMENTS.md maps exactly NLQ-01..04 to Phase 6, and all four appear in plan frontmatter `requirements:` fields.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|---|---|---|---|---|
| `internal/understand/window.go` | 58-61 | `time.AddDate` month/year-overflow date-math bug (06-REVIEW.md WR-01) | ⚠️ Warning | Silently shrinks the "past month"/"past year" suggested window to ~1-3 days on ~15-16 dates/year and around Feb 29 leap years; untested against this case. Disposition in `06-REVIEW-DISPOSITION.md` is still `open`. |
| `internal/server/understand.go` / `proto/engram/v1/engram.proto` | understand.go:31-118, proto:762 | `UnderstandQueryRequest.cross_spine` threaded through but never consulted (06-REVIEW.md WR-02) | ℹ️ Info | Dead field on the server; does not affect any must-have (D-08's gate is documented only against `Scope == ""`), but worth a follow-up per the review's own recommendation. |
| No `TBD`/`FIXME`/`XXX`/`HACK` markers found | — | grep swept all phase-modified core files | — | Clean. |

### Gaps Summary

One narrow, code-review-confirmed correctness defect remains open in the time-window suggestion's date computation (`internal/understand/window.go`): `AddDate(0, -1, 0)`/`AddDate(-1, 0, 0)` overshoot forward instead of landing near the start of the previous month/year whenever "now"'s day-of-month exceeds the target month's length. This affects `past_month`/`past_year` suggestions on roughly 15-16 calendar dates per year (any 29th-31st where the prior month is shorter) plus Feb 29 leap-year boundaries — on those dates, an accepted "past month" chip would silently cover only ~30 hours instead of ~30 days while its label still reads "past month." The bug does not violate the confirmation-gating, off-path, or audit-logging guarantees (Truths 1, 3, 4 are all solidly verified with passing tests), and it does not cause the search to fail or delay — the chip is simply mis-dated on those specific days. It is a real, unresolved defect (code review WR-01, disposition `open`), untested by the shipped `window_test.go` (its fixed `now` of 2026-09-28 never exercises the overflow path), and there is no later phase in this milestone that would pick it up. Recommend either fixing before shipping (the code review report includes a concrete day-count-subtraction fix) or explicitly accepting it via a VERIFICATION.md override with a documented rationale and a follow-up issue.

No human verification items are required beyond the above — WR-01 is a deterministic, already-diagnosed code defect, not an ambiguous or UX-judgment item.

---

_Verified: 2026-09-28T20:15:00Z_
_Verifier: Claude (gsd-verifier)_
