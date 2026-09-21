---
phase: 07-bounded-provider-responses
verified: 2026-09-21T20:35:00Z
status: passed
score: 13/13 must-haves verified
covered_files:
  - ".planning/REQUIREMENTS.md"
  - ".planning/ROADMAP.md"
  - ".planning/phases/07-bounded-provider-responses/07-01-PLAN.md"
  - ".planning/phases/07-bounded-provider-responses/07-01-SUMMARY.md"
  - ".planning/phases/07-bounded-provider-responses/07-02-PLAN.md"
  - ".planning/phases/07-bounded-provider-responses/07-02-SUMMARY.md"
  - ".planning/phases/07-bounded-provider-responses/07-03-PLAN.md"
  - ".planning/phases/07-bounded-provider-responses/07-03-SUMMARY.md"
  - ".planning/phases/07-bounded-provider-responses/07-04-PLAN.md"
  - ".planning/phases/07-bounded-provider-responses/07-04-SUMMARY.md"
  - ".planning/phases/07-bounded-provider-responses/07-05-PLAN.md"
  - ".planning/phases/07-bounded-provider-responses/07-05-SUMMARY.md"
  - ".planning/phases/07-bounded-provider-responses/07-CONTEXT.md"
  - ".planning/phases/07-bounded-provider-responses/07-REVIEW.md"
  - "docs-site/src/content/docs/guides/configure.md"
  - "docs-site/src/content/docs/guides/upgrade.md"
  - "internal/config/config.go"
  - "internal/config/config_test.go"
  - "internal/config/providerbounds_test.go"
  - "internal/config/registry.go"
  - "internal/config/service_auth_test.go"
  - "internal/config/validate.go"
  - "internal/config/validate_test.go"
  - "internal/embed/embed.go"
  - "internal/embed/embed_test.go"
  - "internal/httpdrain/httpdrain.go"
  - "internal/httpdrain/httpdrain_test.go"
  - "internal/server/providerbounds_test.go"
  - "internal/server/tools.go"
  - "internal/store/redevidence_harness_test.go"
  - "internal/summarize/summarize.go"
  - "internal/summarize/summarize_test.go"
  - "internal/testhttp/reuse.go"
  - "internal/testhttp/trickle.go"
covered_digest: "v1:sha256:406b5bacaf2b6903bbe97e901c142d23c170311f94d301b0010e720b4b8bf5f4"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 12/13
  gaps_closed:
    - "The phase's recorded state is internally consistent at close: the roadmap's plan list, both requirement checkboxes and both traceability rows say the same thing about Phase 7, and no other milestone's rows moved"
  gaps_remaining: []
  regressions: []
---

# Phase 7: Bounded Provider Responses Verification Report

**Phase Goal:** Bound the embed and summarize clients' post-response body drain by both bytes and
time, independent of `http.Client.Timeout` — an `io.LimitReader` alone would close only the byte
axis, leaving #457's slow-trickle-body-under-`WithTimeout(0)` scenario still able to hang. #347's
bounded error-body read already shipped (v0.12.x, #464); this phase's real work is #457's drain
only, confirmed by a regression test that exercises both a large-but-fast body and a slow-trickle
body under `WithTimeout(0)` separately. Fully independent of the Qdrant read-path work.

**Verified:** 2026-09-21T20:35:00Z
**Status:** passed
**Re-verification:** Yes — after gap closure (previous run: gaps_found, 12/13, single ROADMAP.md bookkeeping gap)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | One shared `internal/httpdrain` helper is imported by both provider clients; neither has a private copy of the drain mechanism (D-02) | ✓ VERIFIED | `internal/httpdrain/httpdrain.go` exports `Drain`; `internal/embed/embed.go:403,417` and `internal/summarize/summarize.go:302,311` both call `httpdrain.Drain(resp.Body, c.drainBytes, c.drainTimeout)` |
| 2 | The drain is bounded on BOTH the byte axis (`io.LimitReader`) and the time axis (a `time.AfterFunc` timer that closes the body, unblocking the in-flight `Read`), with no goroutine outliving the call | ✓ VERIFIED | `httpdrain.go:55-63`: `io.Copy(io.Discard, io.LimitReader(body, maxBytes))` guarded by `t := time.AfterFunc(maxTime, ...)` / `defer t.Stop()`. Mechanism matches D-01's pinned `net/http` analysis. Unit-proven by `TestDrainStopsAtByteBound` and `TestDrainClosesBodyAtTimeBound` (`internal/httpdrain/httpdrain_test.go`), both passing (`go test ./internal/httpdrain/...` → ok) |
| 3 | A zero on either drain bound skips the drain entirely (body closed immediately, connection not reused) — no value means "unbounded" | ✓ VERIFIED | `httpdrain.go:56-59`: `if maxBytes <= 0 \|\| maxTime <= 0 { _ = body.Close(); return }`; proven by `TestDrainZeroSkipsEntirely` |
| 4 | Both embed drain sites (non-2xx and success) and both summarize drain sites route through the shared helper | ✓ VERIFIED | `embed.go:403` (error path), `embed.go:417` (success path); `summarize.go:302` (error path), `summarize.go:311` (success path) — all four call `httpdrain.Drain` |
| 5 | `WithDrainBytes(0)`/`WithDrainTimeout(0)` are honored as zero (defaults set in the struct literal before options run), unlike `WithMaxResponseBytes`'s swallow-zero behavior (D-06) | ✓ VERIFIED | Both `New` functions set `drainBytes`/`drainTimeout` in the struct literal before `for _, opt := range opts { opt(c) }`; proven by `TestEmbedDrainOptionsHonorZero` and `TestSummarizeDrainOptionsHonorZero` |
| 6 | D-11 shape 1 (byte axis): a body over the byte bound leaves the connection unreused, a body inside the bound leaves it reused — proven together per client, both lanes | ✓ VERIFIED | `TestEmbedDrainBoundedByBytes` and `TestSummarizeDrainBoundedByBytes` each run an "over the bound: not reused" and an "inside the bound: reused (control)" subtest using `testhttp.ReuseTracker`, working around `net/http`'s own 256 KiB post-close safety-net drain via an explicit oversized `Content-Length` |
| 7 | D-11 shape 2 (time axis): a slow-trickle body under `WithTimeout(0)` returns in far less than the trickle would cost, driven by a shared handler that always finishes on its own | ✓ VERIFIED | `TestEmbedDrainBoundedByTimeUnderZeroRequestTimeout` / `TestSummarizeDrainBoundedByTimeUnderZeroRequestTimeout` use `WithTimeout(0)` + `testhttp.TrickleHandler` (bounded ~3s trickle, always exits via `count*pause` or context-done) and assert `elapsed < 1s` |
| 8 | An explicit positive `WithTimeout(d)` is honored UNCAPPED, however large; only a non-positive `d` resolves to the configurable ceiling, applied once in `New` after every option has run (D-07, D-09) | ✓ VERIFIED | `embed.go:253-254` / `summarize.go:208-209`: `if c.http.Timeout <= 0 { c.http.Timeout = c.maxTimeout }` — the over-ceiling clamp clause found by 07-REVIEW.md's CR-01 is gone. `TestEmbedTimeoutCeiling`/`TestSummarizeTimeoutCeiling`'s "positive duration above the default ceiling is honored uncapped" subtest (`want: 20*time.Minute`) passes |
| 9 | An oversized non-2xx/non-200 provider error body is surfaced TRUNCATED at the named bound, closing REQ-provider-error-body-closed's remaining gap (D-10) | ✓ VERIFIED | `TestEmbedNon2xxErrorBodyTruncated` / `TestSummarizeNon200ErrorBodyTruncated` assert the provider's marker text survives AND the surfaced error length is bounded near `maxErrorBodyBytes`, not near the served body length |
| 10 | Six new registry keys (per-client drain bytes, drain timeout, max-timeout ceiling) exist with defaults, reach their `Config` fields, and reject invalid values naming their own env var (D-04, D-05, D-08) | ✓ VERIFIED | `internal/config/registry.go` carries all six `ENGRAM_*` keys; `internal/config/validate.go` rejects negative drain bounds and non-positive ceilings by name; `TestValidateProviderBounds`/`TestProviderBoundRegistryEntries` pass (`go test ./internal/config/...` → ok) |
| 11 | Each of the six configured bounds is actually wired into its client's constructor (not just present in config), proven by a source-level gate | ✓ VERIFIED | `internal/server/tools.go:621-623` (embed) and `:666-668` (summarize) pass `WithDrainBytes`/`WithDrainTimeout`/`WithMaxTimeout`; `TestProviderBoundOptionsWiredIntoBothClients` passes (`go test ./internal/server/... -run TestProviderBound` → ok) |
| 12 | Both requirements are ticked complete in REQUIREMENTS.md (checklist and traceability table), and GitHub #347 is closed citing the pinning tests; #457 stays open (closes through the normal ship flow) | ✓ VERIFIED | `.planning/REQUIREMENTS.md:59-60,104-105` — both `[x]` and both traceability rows read "Complete". `gh issue view 347` → `state: CLOSED, stateReason: COMPLETED`. `gh issue view 457` → `state: OPEN` (expected — untouched by design per 07-05-SUMMARY.md) |
| 13 | The phase's recorded state (ROADMAP.md plan list + REQUIREMENTS.md) is internally consistent at close | ✓ VERIFIED | Re-verified 2026-09-21T20:35:00Z after commit `f661807a`. `git show f661807a -- .planning/ROADMAP.md` confirms an exact values-only diff: `**Plans:** 2/5 plans executed` → `5/5`, and the 07-03/07-04/07-05 checkboxes flipped `[ ]` → `[x]` (07-01/07-02 already `[x]`). Current tree: `grep -n "07-0[1-5]-PLAN.md\|Plans:.*executed" .planning/ROADMAP.md` shows all five checked and "5/5 plans executed". No other lines in ROADMAP.md changed — `git diff --stat HEAD~1 HEAD` (4 insertions, 4 deletions, one file) and `git status --porcelain` confirm nothing else moved; the two archived same-numbered Phase 7 rows (v0.8.x line 625, 2026-08-12.01 line 663) are byte-identical to my prior run |

**Score:** 13/13 truths verified

### Required Artifacts

Verified via `gsd_run query verify.artifacts` against each plan's `must_haves.artifacts` frontmatter — all 19 artifacts across the five plans pass (exists, substantive, contains required pattern):

| Plan | Artifacts checked | Result |
|------|-------------------|--------|
| 07-01 | 5 (`httpdrain.go`, `httpdrain_test.go`, `trickle.go`, `embed.go`, `embed_test.go`) | 5/5 passed |
| 07-02 | 4 (`registry.go`, `config.go`, `validate.go`, `providerbounds_test.go`) | 4/4 passed |
| 07-03 | 2 (`summarize.go`, `summarize_test.go`) | 2/2 passed |
| 07-04 | 4 (`tools.go`, `providerbounds_test.go`, `configure.md`, `upgrade.md`) | 4/4 passed |
| 07-05 | 4 (`redevidence_harness_test.go`, two red-evidence patches, `REQUIREMENTS.md`) | 4/4 passed |

### Key Link Verification

Verified via `gsd_run query verify.key-links` against each plan's `must_haves.key_links` frontmatter — all 12 links across the five plans verified:

| Plan | Links checked | Result |
|------|----------------|--------|
| 07-01 | 3 | 3/3 verified |
| 07-02 | 2 | 2/2 verified |
| 07-03 | 2 | 2/2 verified |
| 07-04 | 3 | 3/3 verified |
| 07-05 | 2 | 2/2 verified |

Manually spot-checked in addition to the query tool: `internal/server/tools.go:621-623` and `:666-668` carry all six `embed.With*`/`summarize.With*` option calls sourced from `cfg`, never a direct environment read.

### Behavioral Spot-Checks / Test Execution

| Check | Command | Result | Status |
|-------|---------|--------|--------|
| httpdrain unit tests | `go test ./internal/httpdrain/... -count=1` | ok, 0.20s | ✓ PASS |
| embed package tests | `go test ./internal/embed/... -count=1` | ok, 0.50s | ✓ PASS |
| summarize package tests | `go test ./internal/summarize/... -count=1` | ok, 0.44s | ✓ PASS |
| config package tests | `go test ./internal/config/... -count=1` | ok, 0.19s | ✓ PASS |
| provider-bound wiring/source gate | `go test ./internal/server/... -run TestProviderBound -v -count=1` | all subtests PASS (parse/default/warn coverage for all six knobs + the source gate) | ✓ PASS |
| `TestEmbedTimeoutCeiling` "above-ceiling honored uncapped" subtest | included in embed package run above | PASS — confirms CR-01 fix in the shipped code, not just the doc comment | ✓ PASS |
| Full-repo red-evidence harness (`TestRedEvidencePatchesAreLive`) | Not re-run in this verification pass (requires `-timeout 180m`; explicitly flagged by the orchestrator as having stranded mutated source on four prior kills). Relying on orchestrator-gathered evidence: three full runs post-CR-01-fix (458.969s / 295.705s / 781.927s), all exit 0, all 63 RED directions confirmed | Relied on documented evidence, not independently re-executed | ℹ️ see note |
| Registry mapping for phase 7's five patches | `grep -n "07-bounded-provider-responses" internal/store/redevidence_harness_test.go` | entry present, maps all 5 patches to 5 distinct target tests | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|-----------------|--------------|--------|----------|
| REQ-provider-drain-bounded | 07-01, 07-02, 07-03, 07-04, 07-05 | Both clients bound the drain by bytes and time, independent of `http.Client.Timeout`, proven under `WithTimeout(0)` | ✓ SATISFIED | Truths 1-8, 10-11 above; `.planning/REQUIREMENTS.md:59,104` marked Complete |
| REQ-provider-error-body-closed | 07-01, 07-03, 07-05 | Already-shipped bounded error read confirmed pinned by tests, #347 closed | ✓ SATISFIED | Truth 9, 12 above; GitHub #347 confirmed CLOSED/COMPLETED via `gh` |

No orphaned requirements found — REQUIREMENTS.md's Phase 7 mapping (lines 59-60, 104-105) exactly matches the two requirement IDs declared across all five plans' frontmatter.

### Code Review Findings — Fix Verification

07-REVIEW.md found 1 critical + 2 warnings + 1 info. Per orchestrator notes, three post-review commits addressed the critical and one warning:

| Finding | Severity | Fixed? | Evidence |
|---------|----------|--------|----------|
| CR-01 (clamp overrode D-07's "honor uncapped" for positive over-ceiling values) | Critical | ✓ Fixed | `embed.go:253`/`summarize.go:208` now read `if c.http.Timeout <= 0` only (commit `13635b13`); both `...TimeoutCeiling` tests updated to assert `want: 20*time.Minute` for the over-ceiling case, and pass |
| WR-01 (`Drain`'s doc comment didn't state the caller-Close precondition) | Warning | ✓ Fixed | `httpdrain.go:38-42` now states the CALLER PRECONDITION explicitly (commit `2e60ce2e`) |
| IN-01 ("no goroutine outlives this call" overclaimed) | Info | ✓ Fixed | `httpdrain.go:33-36` now describes the narrow `time.AfterFunc` callback race honestly (commit `2e60ce2e`) |
| WR-02 (`Config.Validate` never cross-checks `Timeout` vs `MaxTimeout`) | Warning | Not fixed (by design) | Confirmed absent in `internal/config/validate.go` — review itself called this "not a blocker," and it was dissolved in severity by the CR-01 fix (no longer silently downgrades, just leaves an unreachable ceiling on that lane, which the review already characterized as "confusing but harmless") |

### Anti-Patterns Found

`rg` scan across all changed production files (`internal/httpdrain/`, `internal/embed/embed.go`, `internal/summarize/summarize.go`, `internal/config/{registry,config,validate}.go`, `internal/server/tools.go`, `internal/testhttp/{trickle,reuse}.go`) for `TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER`: **zero matches.** No debt markers, no stub returns, no empty handlers found in any of the four drain call sites or the six config-wiring helpers.

### Human Verification Required

None. Every truth in this phase is either directly proven by a passing, substantive test (both the byte and time axes are independently exercised with control cases, not just presence-checked) or a static/documentation fact directly observed in the repository (registry entries, GitHub issue state, requirements checkboxes).

### Gaps Summary

None. The single gap from the prior run — ROADMAP.md's Phase 7 plan checklist and plan-count line not reflecting all five plans as complete — is closed.

**Re-verification (2026-09-21T20:35:00Z):** Commit `f661807a` ("docs(07): mark all five phase 7 plans executed in the roadmap section") made the exact values-only edit needed: `**Plans:** 2/5 plans executed` → `5/5 plans executed`, and the 07-03/07-04/07-05 checkboxes flipped `[ ]` → `[x]`. Confirmed via `git show f661807a -- .planning/ROADMAP.md` (a 2-line/6-line values-only diff — plan count plus three checkbox flips, no structure invented) and by reading the current file directly. Two things confirmed deliberately unchanged, per the coordinator's note, and correctly so — neither is a gap:

1. The Phase 7 row in the `## Progress` table still reads `| 7. Bounded Provider Responses | 2026-09-18.01 | 5/5 | In Progress | - |`. Marking it "Complete" is `phase.complete`'s job at the `update_roadmap` step, which runs only after verification passes — setting it now would claim an outcome (this verification) that had not yet happened at commit time. This does not block `status: passed`; it is the correct next action for whatever consumes this VERIFICATION.md.
2. The two archived same-numbered Phase 7 rows (`| 7. Web UI, Docs Site & Distribution | v0.8.x | 9/9 | Complete | 2026-08-20 |` at line 625, `| 7. Console & CLI State Surfacing | 2026-08-12.01 | 3/3 | Complete | 2026-08-20 |` at line 663) are byte-identical to my prior run. `git diff --stat HEAD~1 HEAD` confirms exactly one file changed (`.planning/ROADMAP.md`, 4 insertions / 4 deletions) and `git status --porcelain` shows nothing else outstanding besides this VERIFICATION.md itself and an unrelated untracked `.planning/milestone.lock`.

All 13 must-haves now verify. No other part of the tree regressed since the initial run — the twelve previously-verified truths, all 19 artifacts, and all 12 key links were re-confirmed present and unchanged during this pass.

---

_Verified: 2026-09-21T20:35:00Z (re-verification; initial run 2026-09-21T20:10:00Z)_
_Verifier: Claude (gsd-verifier)_

## Re-fingerprint

**2026-09-21 — `covered_digest` recomputed at HEAD `6a504ca2`. Verdict unchanged: `passed`, 13/13.**

Not a re-verification. `covered_files` includes the shared planning ledgers
`.planning/ROADMAP.md` and `.planning/REQUIREMENTS.md`, so the ordinary
post-verification step `phase.complete` — which ticks the Phases checklist and
advances the Progress table — mutated a covered file and flipped the status to
`stale` without any verified claim changing. This is the recurring false-stale
documented in engram memory `rgcp7yb5fh`; the repair is a re-fingerprint by the
orchestrator, never `/gsd-verify-work` (`6mhdtdkyn4`).

Re-proved before recomputing: the only commit between the verification artifact
(`dfc7a7c4`) and HEAD is `6a504ca2`, and `git diff --name-only dfc7a7c4..HEAD`
returns exactly `.planning/ROADMAP.md`, `.planning/STATE.md` and
`.planning/state.json` — no source file, plan, summary, requirement or review.
Every one of the 13 must-haves is therefore untouched.

- Previous digest: `v1:sha256:63a4959d…`
- Current digest: `v1:sha256:406b5bac…`
- Covered files: 34, all present and readable

Note for the next person hitting this: `gsd-tools query verification.fingerprint
<files...>` returned a digest that did NOT satisfy the status check
(`a474ee8e…` vs the required `406b5bac…`) — the CLI verb and the staleness
comparison canonicalize the file list differently. The authoritative value is
whatever `computeCoveredDigest(findProjectRoot(phaseDir), <the frontmatter's own
covered_files array>)` returns, from `gsd-core/bin/lib/verification.cjs`. Compute
it that way and confirm with `verification.status --pick status` before
committing; do not trust the CLI verb's output for this repair.

---

## SUPERSEDED 2026-09-21 — red-evidence coverage removed by decision

Every claim in this document about red-evidence patches, `redEvidenceDirs`,
`TestRedEvidencePatchesAreLive`, or "confirmed RED directions" **no longer
describes the repository**. On 2026-09-21 Sean established a repo rule
(engram `3p0zsqrhmb`):

> NEVER write tests for tests, tests for integration tests, gates of gates, or
> similar. Tests verify behaviour.

The red-evidence harness was a mutation-testing rig whose subject was the test
suite rather than the product, so it violated that rule. `internal/store/
redevidence_harness_test.go` (437 lines) and all 122 `.patch` files across every
phase and archived milestone were deleted.

**What this does and does not change:**

- The behavioural tests those patches pointed at are UNTOUCHED and still pass —
  `TestEmbedDrainBoundedByBytes`, `TestEmbedDrainBoundedByTimeUnderZeroRequestTimeout`,
  `TestEmbedDrainOptionsHonorZero`, `TestEmbedTimeoutCeiling`,
  `TestEmbedNon2xxErrorBodyTruncated`, `TestSummarizeTimeoutCeiling`,
  `TestSummarizeNon200ErrorBodyTruncated`, and the `internal/httpdrain` unit tests.
  The phase's actual behaviour is still verified.
- What is gone is the second-order proof that each of those tests fails when its
  bug is reintroduced. That proof is not being replaced.

Left in place rather than rewritten: this document records what was true when it
was written. Read it with this note.
