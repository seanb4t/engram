---
phase: 05-connect-record-state-parity
plan: 04
subsystem: testing
tags: [e2e, chromedp, headless-chrome, sveltekit, embed, ci, connect-rpc]

# Dependency graph
requires:
  - phase: 05-connect-record-state-parity (plans 01-03)
    provides: the Connect API record-state parity this plan re-proves at the console end
provides:
  - "internal/e2e/console_browser_test.go: a real-headless-Chrome render test proving the embedded SvelteKit console bundle hydrates and renders a Connect-written record"
  - "browser-observed CDP failure gate (chromedp.ListenTarget) catching failed/aborted _app/immutable requests and uncaught JS exceptions, with a non-vacuity assertion"
  - "asset-completeness sweep over the served index.html (closes the UAT's literal G-05-9 missing item)"
  - "ENGRAM_REQUIRE_BROWSER fail-closed gate mirroring ENGRAM_REQUIRE_QDRANT, wired into CI's existing test job and task test:strict"
affects: [06-typed-operator-renderer, any-future-ui-regeneration]

# Actuals (#2632)
actuals:
  tokens: 8831
  tasks: 3
  commits: 3

tech-stack:
  added: ["github.com/chromedp/chromedp v0.16.0 (pinned, test-only import in internal/e2e)"]
  patterns:
    - "Fail-closed test-tier gate: ENGRAM_REQUIRE_BROWSER mirrors ENGRAM_REQUIRE_QDRANT's requireX/skipOrFailNoX shape byte-for-byte."
    - "Browser-observed CDP failure gate: chromedp.ListenTarget registered BEFORE navigation, guarded by sync.Mutex, paired with a non-emptiness assertion so zero observed requests cannot trivially pass."
    - "Load-bearing per-run-random DOM marker (crypto/rand) as a strictly-stronger render proof than any data-testid."

key-files:
  created:
    - internal/e2e/console_browser_test.go
    - .planning/phases/05-connect-record-state-parity/deferred-items.md
  modified:
    - internal/e2e/harness_test.go
    - go.mod
    - go.sum
    - .github/workflows/ci.yaml
    - Taskfile.yaml
    - .planning/WINDOWS.md

key-decisions:
  - "Two-stage browser navigation (/ui/ then /ui/observe?scope=) instead of the plan's single /ui/ navigation, to route around a real pre-existing console bug this test discovered (see Deviations)."
  - "chromedp v0.16.0 pinned (not @latest), per G9-D1 — go.sum records checksums for the exact API this plan verified against."

patterns-established:
  - "requireBrowser/skipOrFailNoBrowser/findChrome: the canonical shape for adding a new fail-closed, ENGRAM_REQUIRE_*-gated optional test tier to internal/e2e."

requirements-completed: [REQ-connect-parity-roundtrip-proof]

coverage:
  - id: D1
    description: "A real headless Chrome renders the real engram binary's real embedded console bundle: SvelteKit hydrates, a Connect-written record (same session identity) appears in the live DOM, every declared _app/immutable asset resolves, zero browser-observed failures/exceptions, and the tier is fail-closed under ENGRAM_REQUIRE_BROWSER (wired into CI)."
    requirement: "REQ-connect-parity-roundtrip-proof"
    verification:
      - kind: e2e
        ref: "internal/e2e/console_browser_test.go#TestConsoleBundleRendersRecordInBrowser"
        status: pass
    human_judgment: false

duration: 32min
completed: 2026-08-16
status: complete
---

# Phase 05 Plan 04: Real-Browser Console Render Gate Summary

**A real headless Chrome (chromedp v0.16.0) drives the real `engram` binary's real embedded SvelteKit bundle, proves it hydrates, writes a record over Connect with the browser's own session identity, and proves that record appears in the live DOM — closing G-05-9 (nothing previously rendered the embedded console bundle in a browser).**

## Performance

- **Duration:** ~32 min
- **Started:** 2026-08-16T13:33:00Z (approx.)
- **Completed:** 2026-08-16T14:05:13Z
- **Tasks:** 3
- **Files modified:** 6 (+1 new test file, +1 new deferred-items note)

## Accomplishments

- `TestConsoleBundleRendersRecordInBrowser`: an end-to-end real-browser test in `internal/e2e/` that boots the real `engram` binary with the web UI enabled behind a stub OIDC discovery document, mints a sealed session + CSRF token directly (no login flow), writes a per-run-random-marker fixture record over the Connect API with that identity, drives headless Chrome to render it, and asserts zero browser-observed asset failures / JS exceptions over a non-empty observation set.
- A supplementary HTTP sweep of every `_app/immutable/**` reference the served `index.html` declares, asserting 200 + non-empty body for each (closes the UAT's literal `missing:` item).
- `ENGRAM_REQUIRE_BROWSER` fail-closed gate (mirrors `ENGRAM_REQUIRE_QDRANT`) wired into CI's existing `test` job and `task test:strict` — no new job, no matrix, no browser-install step (the `ubuntu-latest` runner already ships Chrome/Chromium).
- Filed [seanb4t/engram#500](https://github.com/seanb4t/engram/issues/500) for a real, previously-undetected console bug this test discovered (see Deviations).

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end hydration proof** — `4b67b658` (feat) — chromedp dependency, `baseURL()` accessor, `requireBrowser`/`skipOrFailNoBrowser`/`findChrome`, `stubOIDCProvider`, `startConsoleServer`, and the initial hydration-only render test.
2. **Task 2: Connect-write round trip** — `0a7e413c` (feat) — `mintFixtureMarker`, `seedFixtureRecord`, and the strengthened render assertion; includes the deviation to two-stage navigation.
3. **Task 3: Browser-observed failures, asset sweep, fail-closed CI** — `2dc26ca6` (feat) — `browserObserver`, `sweepConsoleAssets`, CI/Taskfile wiring, and a revive lint fix.

**Plan metadata:** (this commit, docs)

## TASK_BASE SHAs (per `<commit_range_protocol>`)

- Task 1 TASK_BASE: `6f6abc8b091e7e35a2a8d3fc6b3cdbceacdd46d4`
- Task 2 TASK_BASE: `4b67b65888005b36ad7d1b556dbb0272fb87219b`
- Task 3 TASK_BASE: `0a7e413c4ac8797101ad6430bf9d4fcd08dbd2cb`
- Plan-level PLAN_BASE: `6f6abc8b091e7e35a2a8d3fc6b3cdbceacdd46d4` (same as task 1's, since nothing committed before task 1 started)

`git diff 6f6abc8b091e7e35a2a8d3fc6b3cdbceacdd46d4 -- internal/e2e/ go.mod go.sum .github/workflows/ci.yaml Taskfile.yaml` is non-empty (757 lines).

## Files Created/Modified

- `internal/e2e/console_browser_test.go` (new) — the full render test, ~600 lines.
- `internal/e2e/harness_test.go` — added `(*serverProc).baseURL()`.
- `go.mod` / `go.sum` — `github.com/chromedp/chromedp` pinned at `v0.16.0`.
- `.github/workflows/ci.yaml` — `ENGRAM_REQUIRE_BROWSER: "1"` added to the existing `test` job's `Test` step env block.
- `Taskfile.yaml` — `test:strict` env block mirrors the same variable.
- `.planning/WINDOWS.md` — entry id 4 (deviation kind) recording the discovered UI regression.
- `.planning/phases/05-connect-record-state-parity/deferred-items.md` (new) — pre-existing, out-of-scope `dprint`/`task fmt:check` drift discovered during verification.

## Test Evidence

**`go test -list` (proves the `-run` pattern matches something):**
```
$ go test -list '^TestConsoleBundleRendersRecordInBrowser$' ./internal/e2e/
TestConsoleBundleRendersRecordInBrowser
ok  	github.com/seanb4t/engram/internal/e2e	4.253s
```

**Final green run:**
```
=== RUN   TestConsoleBundleRendersRecordInBrowser
--- PASS: TestConsoleBundleRendersRecordInBrowser (1.71s)
PASS
ok  	github.com/seanb4t/engram/internal/e2e	4.229s
```

**Three-consecutive-run stability (`-count=1`, `ENGRAM_REQUIRE_QDRANT=1`):** PASS every time — 4.013s, 3.386s, 3.391s.

**Browser discovery in this run:** resolved via `findChrome`'s macOS dev fallback (`/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`), since `google-chrome`/`chromium`/etc. are not on `PATH` on this darwin/arm64 dev machine. In CI (`ubuntu-latest`), `exec.LookPath` will resolve `google-chrome`/`chromium` directly — the manifest-verified preinstalled binaries (verified_facts item 11) — so the dev fallback path is never reached there.

**Platform:** darwin/arm64 (Docker Desktop for the Qdrant testcontainer; native macOS Chrome for chromedp).

**chromedp version pinned:** `github.com/chromedp/chromedp v0.16.0` (not `@latest`). `05-RESEARCH.md`'s "no new external dependencies" statement is amended by this plan — see `T-05-SC` in the plan's threat model; this is a test-only Go module import, never linked into the shipped `engram` binary.

## GATE-RED Proofs (all five, verbatim)

### 1. GH #106 reproduction (Task 1) — dropped the `all:` embed prefix

Mutation: `internal/webauth/static.go` `//go:embed all:static` → `//go:embed static`.

RED:
```
=== RUN   TestConsoleBundleRendersRecordInBrowser
    console_browser_test.go:281: diagnostics: location="http://127.0.0.1:53899/ui/" body=""
    console_browser_test.go:284: hydration wait failed: waiting for function failed: timeout
--- FAIL: TestConsoleBundleRendersRecordInBrowser (46.32s)
FAIL
```

Revert proof: `rg -c 'embed all:static' internal/webauth/static.go` → `1`.

GREEN:
```
=== RUN   TestConsoleBundleRendersRecordInBrowser
--- PASS: TestConsoleBundleRendersRecordInBrowser (1.39s)
PASS
```

### 2. Wrong marker (Task 2) — searched for `marker+"-RED-PROOF-NEVER-WRITTEN"`

RED:
```
=== RUN   TestConsoleBundleRendersRecordInBrowser
    console_browser_test.go:405: diagnostics: location="http://127.0.0.1:61867/ui/observe?scope=repo%3Ae2e-console-roundtrip" body="...engram-e2e-marker-dc60cae25b3e87d5c048fa24750c4f0a\nconvention\n·\nnow\n1 of 1..."
    console_browser_test.go:406: render wait failed: waiting for function failed: timeout
--- FAIL: TestConsoleBundleRendersRecordInBrowser (46.59s)
FAIL
```
(The diagnostic dump confirms the REAL marker was present in the DOM the whole time — the mutation, not the mechanism, caused the failure.)

Revert proof: `rg -c 'chromedp.Poll(markerPollExpr(marker), &rendered,' internal/e2e/console_browser_test.go` → `1`.

### 3. Dropped `X-CSRF-Token` header on the seed write (Task 2)

RED (fails at the WRITE, not later at render):
```
=== RUN   TestConsoleBundleRendersRecordInBrowser
    console_browser_test.go:358: seed fixture record: permission_denied: csrf: token mismatch
--- FAIL: TestConsoleBundleRendersRecordInBrowser (0.65s)
FAIL
```

Revert proof: `rg -c 'req.Header().Set(webauth.CSRFHeaderName, fixture.csrfToken)' internal/e2e/console_browser_test.go` → `1`.

**Combined GREEN (both marker + CSRF reverts, one re-run):**
```
=== RUN   TestConsoleBundleRendersRecordInBrowser
--- PASS: TestConsoleBundleRendersRecordInBrowser (3.10s)
PASS
```

### 4. Stale chunk reference (Task 3) — the exact incident class G-05-9 is about

Mutation: `internal/webauth/static/index.html` line 20 `href="/ui/_app/immutable/chunks/C36dIUA8.js"` → `href="/ui/_app/immutable/chunks/STALE-HASH-DOES-NOT-EXIST.js"`.

RED (names the exact URL):
```
=== RUN   TestConsoleBundleRendersRecordInBrowser
    console_browser_test.go:427: browser observed failed requests under /ui/_app/immutable/: [http://127.0.0.1:49326/ui/_app/immutable/chunks/STALE-HASH-DOES-NOT-EXIST.js: net::ERR_ABORTED]
--- FAIL: TestConsoleBundleRendersRecordInBrowser (1.78s)
FAIL
```

Revert: `git checkout -- internal/webauth/static/index.html`; confirmed via `rg -c 'C36dIUA8' internal/webauth/static/index.html` → `1` (and the stale literal produced zero matches).

GREEN:
```
=== RUN   TestConsoleBundleRendersRecordInBrowser
--- PASS: TestConsoleBundleRendersRecordInBrowser (1.62s)
PASS
```

### 5. Browser-absent fail-closed / skip pair (Task 3)

**Fail-closed half** — `ENGRAM_REQUIRE_BROWSER=1 ENGRAM_CHROME_PATH=/nonexistent/chrome`:
```
=== RUN   TestConsoleBundleRendersRecordInBrowser
    console_browser_test.go:366: no usable Chrome/Chromium found and ENGRAM_REQUIRE_BROWSER is set: failing instead of skipping (checked ENGRAM_CHROME_PATH, then PATH for google-chrome/chromium)
--- FAIL: TestConsoleBundleRendersRecordInBrowser (0.00s)
FAIL
```
`--- FAIL`, no `--- SKIP` — as required.

**Permissive half** — `ENGRAM_CHROME_PATH=/nonexistent/chrome` (no `ENGRAM_REQUIRE_BROWSER`):
```
=== RUN   TestConsoleBundleRendersRecordInBrowser
    console_browser_test.go:366: no usable Chrome/Chromium found: set ENGRAM_CHROME_PATH or install google-chrome/chromium, or set ENGRAM_REQUIRE_BROWSER=1 to fail instead of skip
--- SKIP: TestConsoleBundleRendersRecordInBrowser (0.00s)
PASS
```
`--- SKIP`, no `--- FAIL` — as required.

## Decisions Made

- **chromedp v0.16.0, pinned exactly** — per G9-D1 (already decided in the plan; re-confirmed here as executed).
- **Two-stage browser navigation** (`/ui/` for hydration, then `/ui/observe?scope=<fixtureScope>` for the round-trip render) instead of the plan's original single-`/ui/`-navigation design — see Deviations below for why.

## Deviations from Plan

### Rule 3 — Auto-fixed blocking issue: root-route navigation reworked to avoid a real, pre-existing console bug

**Found during:** Task 2, first live run.

**Issue:** The plan's original design (verified_facts item 8, written at plan authoring time) assumed `ui/src/routes/+page.svelte`'s "Recent memories" query (`recentQ`, calling `listMemories({scope: '', ...})` with no `cross_spine`) would successfully render the seeded record on the root `/ui/` route. Driving the real browser against the real server proved this assumption false: the live diagnostic dump showed `error: [invalid_argument] field=scope,cross_spine hint=conditional_required: scope is required unless cross_spine is true`, and the "Recent memories" panel permanently shows "failed to load — retry from the toolbar".

Root cause (confirmed via `git log`): `+page.svelte`'s `recentQ` was last touched in commit `72a80119` (2026-07-15), which **predates** the "scope required unless cross_spine" server-side constraint added in `9ba6449b` (2026-08-12, `proto/engram/v1/engram.proto`). `internal/server/connectapi.go`'s D-04 note confirms this is intentional server-side behavior: `SearchMemories`/`ListMemories` deliberately do NOT infer `cross_spine` from an empty scope (unlike `SearchDiscoveries`), and `TestConnectCrossSpineNotInferred` pins that asymmetry. The root route's UI code was simply never updated after the constraint landed — this is a real, currently-shipped regression that has been silently broken since 2026-08-12, undetected precisely because (per G-05-9's own premise) nothing previously rendered the embedded bundle in a browser.

**Why this is Rule 3 (blocking issue), not Rule 4 (architectural):** The plan's `<prohibitions>` explicitly forbid touching `ui/` source, `internal/webauth` production code, or the vendored bundle in this plan. Fixing the actual bug (adding `cross_spine: true` to `recentQ`) requires a `ui/` source change, `task ui:build`, and re-vendoring — all out of this plan's file scope. The fix applied here is entirely confined to the already-owned test file (`internal/e2e/console_browser_test.go`): the browser still loads `/ui/` FIRST and proves hydration there via the `<h1>` hook (satisfying the plan's literal "loads /ui/" language), then navigates to `/ui/observe?scope=<fixtureScope>` — the EXACT link the root route's own scope-tile button already navigates to on click (a real, working, user-reachable path, not a workaround route) — to prove the round trip. No production code, `ui/` source, or vendored bundle byte was touched (confirmed: `git status --porcelain internal/webauth/static/ ui/` is empty).

**Fix:** Two-stage `chromedp.Run` sequence (see `TestConsoleBundleRendersRecordInBrowser`'s doc comment for the full rationale, inlined at the call site for future readers).

**Files modified:** `internal/e2e/console_browser_test.go` only.

**Follow-up filed:** [seanb4t/engram#500](https://github.com/seanb4t/engram/issues/500) — recommends adding `cross_spine: true` to `recentQ`. Also recorded in `.planning/WINDOWS.md` (entry id 4, kind `deviation`, phase 05).

---

**Total deviations:** 1 auto-fixed (Rule 3 — blocking issue, test-file-only fix)
**Impact on plan:** No production code, `ui/` source, or vendored bundle changed. The render assertion is, if anything, a MORE realistic proof of the round trip (an actual user-reachable navigation) than the plan's original single-page design would have been. The underlying UI bug this test discovered is real, filed, and tracked separately — exactly the kind of gap G-05-9 exists to surface.

## Issues Encountered

- `task fmt:check` (dprint) surfaced 4 pre-existing unformatted files (`.claude/settings.json`, `docs-site/package.json`, `internal/webauth/static/_app/version.json`, `ui/tsconfig.json`) — confirmed via `git status --short` that none were touched by this plan's commits. Logged to `.planning/phases/05-connect-record-state-parity/deferred-items.md`, not fixed (out of scope).
- `golangci-lint`'s `revive` flagged `context-as-argument` on `logConsoleDiagnostics(t *testing.T, browserCtx context.Context)` (Task 3). Fixed by reordering to `(browserCtx context.Context, t *testing.T)`, matching this file's other `ctx`-first helpers. Folded into the Task 3 commit.

## User Setup Required

None — no external service configuration required. `ENGRAM_CHROME_PATH` is an optional test-only override, not a deployment requirement.

## Next Phase Readiness

- G-05-9 is closed: the embedded console bundle now has a real-browser render gate, fail-closed in CI.
- `coverage:` entry `D1` above links this work to `G-05-9` for `/gsd-verify-work` reconciliation.
- The discovered root-route `recentQ` regression (issue #500) is a pre-existing bug, independent of this plan's own correctness, and does not block phase 05 completion — it predates this plan and phase 05's own scope never touched `ui/`'s query shapes.

## Self-Check: PASSED

- `internal/e2e/console_browser_test.go` — FOUND
- `.planning/phases/05-connect-record-state-parity/05-04-SUMMARY.md` — FOUND
- `.planning/phases/05-connect-record-state-parity/deferred-items.md` — FOUND
- Commit `4b67b658` — FOUND
- Commit `0a7e413c` — FOUND
- Commit `2dc26ca6` — FOUND
- Commit `28729783` — FOUND

---
*Phase: 05-connect-record-state-parity*
*Plan: 04*
*Completed: 2026-08-16*
