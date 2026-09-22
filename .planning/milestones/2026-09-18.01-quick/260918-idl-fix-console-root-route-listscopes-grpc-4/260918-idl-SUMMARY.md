---
phase: quick-260918-idl
plan: 01
subsystem: ui
tags: [go, qdrant, grpc, connect-rpc, svelte, sveltekit, vitest, chromedp, e2e]

requires: []
provides:
  - "Store.ListScopes requests only the scope payload key, staying under grpc-go's 4 MiB default client receive limit on a production-sized store"
  - "Console root route's Recent memories panel sends cross_spine: true through the shared listMemoriesKey helper (GH #500)"
  - "e2e browser regression proving the root route renders the cross-spine Recent memories feed with no failed ListScopes/ListMemories RPC"
affects: [console, store, e2e]

actuals:
  tokens: 4948
  tasks: 3
  commits: 3
  plan_head_before: c640c3a8799030a3714c71d22f31b905f43ba74c

tech-stack:
  added: []
  patterns:
    - "ListScopes/spine.go precedent: partial Qdrant payload selector (NewWithPayloadInclude) plus direct payload-key read, instead of decoding a full Memory for one field"
    - "listMemoriesKey's trailing, explicit boolean slot (crossSpine at index 9) as the pattern for widening a shared query-key helper without disturbing existing index positions consumers rely on (mutations/memory.ts reads key[3])"

key-files:
  created: []
  modified:
    - internal/store/store.go
    - internal/store/store_test.go
    - ui/src/lib/queries.ts
    - ui/src/lib/queries.test.ts
    - ui/src/routes/+page.svelte
    - ui/src/routes/observe/+page.svelte
    - ui/src/routes/page.browser.test.ts
    - internal/webauth/static/** (regenerated vendored SPA)
    - internal/e2e/console_browser_test.go

key-decisions:
  - "ListScopes.Scroll requests qdrant.NewWithPayloadInclude(\"scope\") instead of NewWithPayload(true), reading p.GetPayload()[\"scope\"].GetStringValue() directly (spine.go:558 precedent) rather than decoding a full Memory via fromPayload"
  - "crossSpine is a required (not defaulted) trailing parameter on listMemoriesKey, appended at index 9 so every existing index position (including mutations/memory.ts's key[3] visibility read) is untouched"
  - "Root route's recentQ now explicitly sends crossSpine: true rather than the server inferring it from an empty scope, preserving D-04's explicit-cross_spine contract (no internal/server files touched)"

requirements-completed:
  - GH-500
  - QUICK-260918-idl

coverage:
  - id: D1
    description: "Store.ListScopes stays under grpc-go's 4 MiB default client receive limit on a production-sized store by requesting only the scope payload key"
    requirement: "QUICK-260918-idl"
    verification:
      - kind: integration
        ref: "internal/store/store_test.go#TestListScopesFullPayloadsOverGRPCLimit"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestListScopes"
        status: pass
    human_judgment: false
  - id: D2
    description: "Console root route's Recent memories panel sends cross_spine: true through the shared listMemoriesKey helper instead of an empty scope with no cross_spine (GH #500)"
    requirement: "GH-500"
    verification:
      - kind: unit
        ref: "ui/src/lib/queries.test.ts#appends crossSpine as a trailing, explicit index-9 slot without disturbing indexes 0-8"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/page.browser.test.ts#/ui/ root — Recent memories cross-spine feed (#500)"
        status: pass
    human_judgment: false
  - id: D3
    description: "A real headless Chrome against the real engram binary renders the seeded fixture marker on the root route (/ui/) via the cross-spine Recent memories feed, and no ListScopes/ListMemories RPC returns HTTP >= 400 in either navigation"
    requirement: "GH-500"
    verification:
      - kind: e2e
        ref: "internal/e2e/console_browser_test.go#TestConsoleBundleRendersRecordInBrowser"
        status: pass
    human_judgment: false

duration: 13min
completed: 2026-09-18
status: complete
---

# Quick Task 260918-idl: Console root route — ListScopes gRPC limit + Recent memories cross_spine Summary

**Fixed two independent defects breaking `/ui/` in production: `Store.ListScopes` requesting full payloads overflowed grpc-go's 4 MiB receive limit (`ResourceExhausted` → Connect `internal`), and the root route's Recent memories panel sent an empty scope with no `crossSpine`, rejected `invalid_argument` by D-04 (GH #500).**

## Performance

- **Duration:** 13 min
- **Tasks:** 3/3 completed
- **Files modified:** 9 source files + the regenerated `internal/webauth/static/` vendored SPA tree
- **Commits:** 3 (measured: `git rev-list --count c640c3a8..HEAD` = 3)

## Accomplishments

- `Store.ListScopes` now scrolls a scope-only payload (`qdrant.NewWithPayloadInclude("scope")`), staying under grpc-go's 4 MiB default client receive limit regardless of stored content/summary/citation volume — proven against real Qdrant with a 5 MiB full-payload regression test that demonstrably failed (`ResourceExhausted`) on the pre-fix selector.
- The console root route's Recent memories panel now requests a cross-spine recent-activity feed (`crossSpine: true`), rendering across every readable scope instead of erroring `failed to load`. `listMemoriesKey` gained an explicit, required, trailing `crossSpine` parameter at array index 9 — indexes 0–8 (including the visibility filter `mutations/memory.ts` reads at `key[3]`) are untouched.
- The vendored SPA (`internal/webauth/static/`) was regenerated via `task ui:build` and is byte-deterministic (a second build produces an empty diff).
- The e2e browser test now proves the root route's cross-spine round trip with a real headless Chrome against the real binary, and fails on any `ListScopes`/`ListMemories` RPC returning HTTP ≥ 400. Stale workaround prose from 05-04 (claiming the root route was a known-broken, out-of-scope bug) was retired.

## Task Commits

Each task was committed atomically:

1. **Task 1: ListScopes requests only the scope payload key, with a real-Qdrant regression test proven RED** — `9b61f5ad` (fix)
2. **Task 2: Root-route Recent memories sends cross_spine (#500) via the shared key helper, plus the regenerated vendored SPA** — `aa0e1008` (fix)
3. **Task 3: e2e browser test asserts the root route renders the fixture via Recent memories, with no failed ListScopes/ListMemories RPC** — `339ab181` (test)

_No plan-metadata commit — the orchestrator handles the docs commit per the constraints for this quick task._

## Files Created/Modified

- `internal/store/store.go` — `ListScopes` requests `NewWithPayloadInclude("scope")` and reads the scope key directly; doc comment extended
- `internal/store/store_test.go` — new `TestListScopesFullPayloadsOverGRPCLimit` (40 records × 128 KiB, real Qdrant, proven RED then GREEN)
- `ui/src/lib/queries.ts` — `listMemoriesKey` gains a required trailing `crossSpine: boolean` parameter (index 9)
- `ui/src/lib/queries.test.ts` — new/updated key-shape assertions for the 10-element key
- `ui/src/routes/+page.svelte` — `recentQ` sends `crossSpine: true` via `listMemoriesKey`, replacing the inline 6-element key array
- `ui/src/routes/observe/+page.svelte` — passes `false` as the new trailing `listMemoriesKey` argument (unchanged behavior)
- `ui/src/routes/page.browser.test.ts` — new describe block pinning the cross-spine request args and cache-key shape
- `internal/webauth/static/**` — regenerated vendored SPA (hashed chunk renames/adds/deletes per `task ui:build`)
- `internal/e2e/console_browser_test.go` — `rootRoutePollExpr` helper, `failedRPCs` tracking on `browserObserver`, root-route render assertion + failed-RPC gate in `TestConsoleBundleRendersRecordInBrowser`, retired stale 05-04 workaround prose

## Decisions Made

- Read the scope payload key directly (`p.GetPayload()["scope"].GetStringValue()`) rather than decoding a full `Memory` via `fromPayload` for one field — follows the existing `spine.go:558` precedent and avoids wasted decode work now that the payload selector is narrowed.
- Kept `crossSpine` a required (non-defaulted) parameter on `listMemoriesKey` so every caller states its cross-spine intent explicitly, mirroring the server's D-04 "never infer cross_spine" rule on the client side too.
- No `internal/server` changes — the server's rejection of an empty scope without `cross_spine` is correct by design (D-04, `TestConnectCrossSpineNotInferred`); the fix is entirely client-side.

## Deviations from Plan

None — plan executed exactly as written. All three tasks matched their planned behavior, action, and verify/acceptance criteria.

## RED/GREEN Evidence

### Task 1 — `TestListScopesFullPayloadsOverGRPCLimit` (real Qdrant via testcontainers)

**RED** (`ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestListScopesFullPayloadsOverGRPCLimit$' -count=1 -v`, pre-fix full-payload selector):
```
=== RUN   TestListScopesFullPayloadsOverGRPCLimit
    store_test.go:1830: ListScopes: Scroll() failed: store_mem_eval_test: rpc error: code = ResourceExhausted desc = grpc: received message after decompression larger than max 4194304 (full-payload scroll exceeded the gRPC receive limit)
--- FAIL: TestListScopesFullPayloadsOverGRPCLimit (0.45s)
FAIL
```

**GREEN** (same command, post-fix scope-only selector):
```
--- PASS: TestListScopes (0.26s)
--- PASS: TestListScopesFullPayloadsOverGRPCLimit (0.08s)
ok  	github.com/seanb4t/engram/internal/store	1.155s
```

A second deliberate RED/GREEN cycle was also run per the plan's instruction (restore the old full-payload selector, observe `ResourceExhausted` again, restore the fix, re-confirm green) — both reproduced identically to the above.

Recall-gate tests also confirmed green and untouched: `TestRecallEmissionSetIsCompleteAndClassified`, `TestSchemaVersionNeverGatesRecall`, `TestFilterWalkerSeesEveryPosition` all PASS in the same run; `git diff --quiet c640c3a8 -- internal/store/schemaversion_recallgate_test.go` exits 0.

### Task 2 — UI cross-spine assertions

**RED** (`pnpm -C ui exec vitest run --project node src/lib/queries.test.ts`, pre-fix 9-arg `listMemoriesKey`):
```
FAIL  |node| src/lib/queries.test.ts > observe params + keys > builds a stable list query key
AssertionError: expected [ 'listMemories', 'repo:x', …(7) ] to deeply equal [ 'listMemories', 'repo:x', …(8) ]
FAIL  |node| src/lib/queries.test.ts > observe params + keys > appends crossSpine as a trailing, explicit index-9 slot without disturbing indexes 0-8
AssertionError: expected [ 'listMemories', 'repo:x', …(7) ] to not deeply equal [ 'listMemories', 'repo:x', …(7) ]
```

**RED** (`pnpm -C ui exec vitest run --project browser src/routes/page.browser.test.ts`, pre-fix root route):
```
FAIL  |browser (chromium)| ... > requests listMemories with an empty scope and crossSpine: true
AssertionError: expected { scope: '', limit: 50n, …(3) } to match object { scope: '', crossSpine: true }
- "crossSpine": true,
FAIL  |browser (chromium)| ... > caches the recent-activity feed under one listMemories entry with crossSpine at key index 9 and visibility at key index 3
AssertionError: expected undefined to be true // Object.is equality
```

**GREEN** (both commands, post-fix):
```
node:    Test Files  1 passed (1) — Tests 13 passed (13)
browser: Test Files  1 passed (1) — Tests  7 passed (7)
```

Full `pnpm -C ui test`: **32 test files, 266 tests, all passed** (at the time of Task 2's commit).

Vendored SPA determinism: `task ui:build && test -z "$(git status --porcelain -- internal/webauth/static ui/)"` exits 0 (confirmed twice, once mid-task and once post-commit).

### Task 3 — e2e root-route render assertion (real headless Chrome + real binary + real Qdrant)

**RED** (pre-fix vendored bundle, restored via `git restore --source=c640c3a8 --worktree -- internal/webauth/static`, then `ENGRAM_REQUIRE_QDRANT=1 ENGRAM_REQUIRE_BROWSER=1 go test ./internal/e2e/ -run '^TestConsoleBundleRendersRecordInBrowser$' -count=1 -v`):
```
=== RUN   TestConsoleBundleRendersRecordInBrowser
    console_browser_test.go:425: diagnostics: location="http://127.0.0.1:52784/ui/" body="error: [invalid_argument] field=scope,cross_spine hint=conditional_required: scope is required unless cross_spine is true\n...\nRECENT MEMORIES\nfailed to load — retry from the toolbar\n..."
    console_browser_test.go:426: root-route Recent memories render wait failed (cross_spine ListMemories, #500): waiting for function failed: timeout
--- FAIL: TestConsoleBundleRendersRecordInBrowser (48.16s)
FAIL
```
After the RED run, the bundle was restored (`git restore --source=HEAD --worktree -- internal/webauth/static`, plus manual removal of stale untracked renamed-chunk leftovers left behind by the git restore) and confirmed clean: `git status --porcelain -- internal/webauth/static` printed nothing.

**GREEN** (same command, post-fix bundle):
```
=== RUN   TestConsoleBundleRendersRecordInBrowser
--- PASS: TestConsoleBundleRendersRecordInBrowser (2.58s)
PASS
ok  	github.com/seanb4t/engram/internal/e2e	5.475s
```

The browser test **actually ran** (not skipped): `ENGRAM_REQUIRE_QDRANT=1` and `ENGRAM_REQUIRE_BROWSER=1` were set on every invocation, Docker/Qdrant testcontainers booted successfully, and Chrome was resolved via the macOS fallback path (`/Applications/Google Chrome.app`). No `--- SKIP` appeared in any run.

## Final Verification Gate

Run from the repo root on branch `fix/console-root-route`, after all three commits:

1. `task lint` — clean (golangci-lint, yamlfmt, actionlint, rumdl, python ruff). One `prealloc` finding was caught and fixed inline during Task 3 before the commit.
2. `ENGRAM_REQUIRE_QDRANT=1 ENGRAM_REQUIRE_BROWSER=1 task test` — **green**: every Go package (`go test ./...`) passed including `internal/store`, `internal/server` (with `TestConnectCrossSpineNotInferred`/`TestListCrossSpine`), and `internal/e2e` (`TestConsoleBundleRendersRecordInBrowser`); `internal/webauth` passed; python hook tests 33/33 passed.
3. `task license:check` — clean, 415 valid / 0 invalid / 1540 ignored.
4. `pnpm -C ui test` — **265/266 passed, 1 flaky failure** in `ScopesSidebar.browser.test.ts` ("toggles a category on via onfilter when its checkbox is clicked"), a file this plan never touched (`git diff --quiet c640c3a8 -- ui/src/lib/components/ScopesSidebar.browser.test.ts ui/src/lib/components/ScopesSidebar.svelte` exits 0 — byte-identical to base). Reproduced twice in the full-suite parallel run; passed cleanly every time run standalone (`pnpm -C ui exec vitest run --project browser src/lib/components/ScopesSidebar.browser.test.ts` → 13/13 passed). This is a **pre-existing, unrelated flake** (likely a parallel-worker timing race), reported separately per plan instructions — never folded into this change's pass/fail.
5. `task ui:build && test -z "$(git status --porcelain -- internal/webauth/static ui/)"` — exits 0 (confirmed after the final commit).
6. `git log --oneline c640c3a8..HEAD` shows exactly three commits in order `fix(store)`, `fix(ui)`, `test(e2e)`, each with `Quick task: 260918-idl` in its body. `git branch --show-current` prints `fix/console-root-route` (verified before the first edit and after every commit).
7. `git diff --quiet c640c3a8 -- .planning/WINDOWS.md .planning/ROADMAP.md` exits 0 (no edits). `git diff --quiet c640c3a8 -- .planning/STATE.md` exits 0 (no executor edits — orchestrator owns it).

## Issues Encountered

- **`ScopesSidebar.browser.test.ts` flake (pre-existing, out of scope):** see item 4 above. Not fixed here — the file is untouched by this plan and the test passes standalone; recorded for the orchestrator/user rather than silently absorbed into this task's pass/fail.
- **Stale untracked leftovers after `git restore --source=c640c3a8 --worktree`:** restoring the base-commit bundle for Task 3's RED proof left old hashed-chunk filenames as untracked files after restoring back to `HEAD` (git restore doesn't delete now-untracked paths that were renamed away in the index). Resolved by explicitly `rm -f`-ing the seven stale leftover files by exact name (not a blanket `git clean`), then confirmed `git status --porcelain -- internal/webauth/static` was empty before re-running the GREEN verification.

## WINDOWS.md / GH #500 — ready to close

`.planning/WINDOWS.md` entry id 4 (`deviation`, phase 05, `ui/src/routes/+page.svelte`, "Root route Recent-memories query (recentQ) calls listMemories with empty scope + no cross_spine... always errors live... fix deferred") describes exactly the defect Task 2 of this quick task fixes. GH #500 is the same live defect. **Not edited here** per this quick task's constraints — the orchestrator/PR should mark WINDOWS.md id 4 `fixed` and close GH #500 when this branch merges.

## Next Steps

- Branch `fix/console-root-route` is ready for PR review — three atomic commits, `task` green (modulo the one pre-existing unrelated UI flake documented above), vendored SPA in sync.
- Orchestrator: close WINDOWS.md id 4 and GH #500 on merge.

---
*Quick task: 260918-idl*
*Completed: 2026-09-18*

## Self-Check: PASSED

All claimed files exist (`internal/store/store.go`, `internal/store/store_test.go`, `ui/src/lib/queries.ts`, `ui/src/routes/+page.svelte`, `internal/e2e/console_browser_test.go`, this SUMMARY.md) and all three claimed commit hashes (`9b61f5ad`, `aa0e1008`, `339ab181`) are found in `git log --oneline --all`.
