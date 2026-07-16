---
phase: 19-console-write-ux
plan: 06
subsystem: ui
tags: [svelte, tanstack-query, resume-envelope, connect-rpc, vitest]

# Dependency graph
requires:
  - phase: 19-05-console-write-ux
    provides: "MemoryFormSheet/DiscoveryFormSheet (create+edit slide-over forms, resumeValues/resumeDirtyPaths props + onresumeapplied) and ui/src/lib/resume.ts (persistResume/peekResume/consumeResume/normalizeReturnPath/isAllowedDestination, single-owner envelope lifecycle)"
  - phase: 19-04-console-write-ux
    provides: "useDeleteMemory/useSetMemoryVisibility/useDeleteDiscovery/useSetDiscoveryVisibility mutation hooks (optimistic cache + rollback)"
  - phase: 19-03-console-write-ux
    provides: "DeleteConfirmDialog/ShareWarningInline host-authoritative awaitable-onconfirm/pending/authFailure contract; MemoryRow/MemoryList/MemoryDetail onedit(id)/ondelete(id)/onshare(memory) callback threading"
provides:
  - "ui/src/lib/components/WriteSurfaces.svelte: route-level write host exposing the exact bind:this contract openCreate()/openEdit(id)/requestDelete(id,kind)/requestShare(memory,kind)/reopenFromResume(env); openEdit refetches FULL content via GetMemory before opening; requestShare reads visibility from the passed Memory and no-ops when already shared; delete/share await mutation settlement with host-authoritative closure (target cleared only on success, retained + re-auth CTA on terminal Unauthenticated/PermissionDenied)"
  - "observe/search/discovery routes wired to WriteSurfaces + row/detail write callbacks; discovery route omits onedit (D-04) and never seeds a discovery create scope from a raw memory scope"
  - "/ui/ root landing consumes the D-09 resume envelope (peek + goto(base+normalizeReturnPath), no delete); each write route reopens via reopenFromResume(env) and calls consumeResume() only after the form's onresumeapplied acknowledgement"
  - "internal/webauth/static/ rebuilt via task ui:build and committed (go:embed'd production console now carries the write UX)"
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Route-level write host with an exact export-function bind:this API (openCreate/openEdit/requestDelete/requestShare/reopenFromResume) — one instance per route, keyed remount via {#key mode-recordId-instanceKey} so the form's mount-once $state initializers always start clean for a new create/edit target"
    - "Host-authoritative surface closure extended to the inline delete/share actions: the host owns deleteDialogOpen/shareTarget and clears them only on mutation success; a terminal Unauthenticated/PermissionDenied retains the target so the dialog/banner stays visibly open with the same re-auth CTA the forms use, with no auto-replay (delete/set-visibility are id-idempotent)"
    - "Single-owner resume-envelope lifecycle completed end-to-end: the /ui/ root peeks+goto()s without deleting; the destination route peeks+reopens via WriteSurfaces.reopenFromResume(env); WriteSurfaces forwards resumeValues/resumeDirtyPaths as props and relays the form's onresumeapplied to the route's consumeResume() — deletion happens exactly once, only after a successful apply"
    - "Browser-test QueryClientProvider wrapper: vitest-browser-svelte's render(Component, props, {wrapper, wrapperProps}) mounts a real QueryClient/QueryClientProvider around the tested component, giving useQueryClient()-calling components real Svelte context in tests without a component-specific test harness"

key-files:
  created:
    - ui/src/lib/components/WriteSurfaces.svelte
    - ui/src/lib/components/WriteSurfaces.browser.test.ts
    - ui/src/routes/page.browser.test.ts
    - ui/src/routes/observe/observe.browser.test.ts
    - ui/src/routes/search/search.browser.test.ts
    - ui/src/routes/discovery/discovery.browser.test.ts
    - .planning/phases/19-console-write-ux/deferred-items.md
  modified:
    - ui/src/routes/+page.svelte
    - ui/src/routes/observe/+page.svelte
    - ui/src/routes/search/+page.svelte
    - ui/src/routes/discovery/+page.svelte
    - internal/webauth/static/** (rebuilt via task ui:build)

key-decisions:
  - "WriteSurfaces mounts ONE MemoryFormSheet instance covering both create and edit (mode + editMemory), remounted via a {#key} block keyed on mode+recordId+an incrementing instanceKey on every open — matches the plan's 'keyed by mode + recordId' requirement while also giving each open a clean $state slate"
  - "requestDelete/requestShare/openEdit/reopenFromResume never auto-retry the underlying mutation on a terminal auth failure — the host only retains the target and surfaces the re-auth CTA; a re-fire is always operator-initiated, preserving the delete/set-visibility no-duplicate-create posture"
  - "Browser tests for WriteSurfaces and all three routes use vitest-browser-svelte's wrapper/wrapperProps render option with a real QueryClientProvider + fresh QueryClient per test, mocking only engram.getMemory/the delete/set-visibility hooks — avoids a bespoke test-harness .svelte file while still exercising WriteSurfaces' real useQueryClient()-backed fetchQuery path"

patterns-established:
  - "Deferred-items.md scope-boundary logging: a pre-existing, already-tracked repo-wide issue (task lint:markdown / .rumdl.toml's missing .planning exclude, Phase 21 backlog) blocking the full task gate is documented rather than fixed, with each independently-relevant sub-gate (lint:go, lint:yaml, lint:actions, lint:python, test:go, test:python, task ui:build reproducibility) verified green in isolation"

requirements-completed: [REQ-console-write-ux]

coverage:
  - id: D1
    description: "WriteSurfaces exposes the exact bind:this contract (openCreate/openEdit/requestDelete/requestShare/reopenFromResume); openCreate opens the correct New sheet defaulting to the current scope; openEdit(id) fetches the FULL record via GetMemory before opening and prefills from it (not a summary-shaped row); on fetch error it toasts and does not open a blank edit"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/components/WriteSurfaces.browser.test.ts (memory-kind create + openEdit describe blocks, 3 tests)"
        status: pass
    human_judgment: false
  - id: D2
    description: "requestShare(memory, kind) reads visibility from the passed Memory: shows the warning and calls set-visibility(SHARED) only for a private/empty-string record; is a genuine no-op (no warning, no call) for an already-shared record"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/components/WriteSurfaces.browser.test.ts (requestShare describe block, 4 tests incl. empty-string normalization)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Delete/share await mutation settlement with host-authoritative closure: the target is cleared (dialog/banner closes) ONLY on success; a terminal Unauthenticated OR PermissionDenied RETAINS the target so the surface stays visibly open with the same inline re-auth CTA the forms use, invoking the mutation exactly once (no auto-replay); Cancel and success both close the surface"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/components/WriteSurfaces.browser.test.ts (requestDelete + SC3 terminal-auth-retention describe blocks, 6 tests parameterized over Unauthenticated/PermissionDenied for both delete and share)"
        status: pass
    human_judgment: false
  - id: D4
    description: "observe/search/discovery routes render WriteSurfaces (kind=memory on observe/search, kind=discovery on discovery) and thread onedit/ondelete/onshare into MemoryList/MemoryDetail; the discovery route passes no onedit (D-04) and never seeds a discovery create scope from a raw memory scope"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/routes/observe/observe.browser.test.ts (onedit full-content fetch test); ui/src/routes/discovery/discovery.browser.test.ts (New discovery + no edit surface describe block, 2 tests)"
        status: pass
    human_judgment: false
  - id: D5
    description: "The /ui/ root landing peeks the resume envelope on mount and goto()s base + normalizeReturnPath(returnPath) WITHOUT deleting it (peek-not-consume), never double-prefixing to /ui/ui/...; rejects and discards an envelope whose returnPath fails isAllowedDestination"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/routes/page.browser.test.ts (5 tests: peek+goto, /ui-prefixed normalization, discovery-kind route, no-envelope no-op, rejected-destination discard)"
        status: pass
    human_judgment: false
  - id: D6
    description: "Each write route reopens the correct sheet from a matching-kind resume envelope on mount (edit refetches full content via GetMemory, create restores values) and calls consumeResume() ONLY after the form's onresumeapplied acknowledgement fires exactly once — a kind-mismatched envelope is left untouched for the correct route to consume"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/routes/observe/observe.browser.test.ts (edit + create + kind-mismatch recovery, 3 tests); ui/src/routes/search/search.browser.test.ts (create recovery + kind-mismatch, 2 tests); ui/src/routes/discovery/discovery.browser.test.ts (create recovery + kind-mismatch, 2 tests)"
        status: pass
    human_judgment: false
  - id: D7
    description: "The embedded production SPA is rebuilt via task ui:build and committed under internal/webauth/static/, byte-reproducible (git diff --exit-code clean immediately after a fresh rebuild) — the shipped binary now carries the write UX"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: other
        ref: "task ui:build && git diff --exit-code -- internal/webauth/static/ (clean, confirmed twice)"
        status: pass
    human_judgment: false
  - id: D8
    description: "Manual end-to-end UAT against a live engram serve + Qdrant: create/edit/delete/share/schedule a memory and create/delete/share a discovery through the console, confirming the CSRF header, the create-as-shared two-call flow lands SHARED, and the retry/re-auth + sessionStorage-draft flow on a rotated/expired session"
    verification: []
    human_judgment: true
    rationale: "Requires a live engram server + Qdrant + real OIDC session and is explicitly deferred to /gsd-verify-work per the plan's <verification> section — cannot be proven by unit/browser tests alone."

# Metrics
duration: 62min
completed: 2026-07-15
status: complete
---

# Phase 19 Plan 06: Route Wiring (WriteSurfaces host + observe/search/discovery + resume envelope + embedded SPA rebuild) Summary

**WriteSurfaces host component orchestrating create/edit/delete/share across all three console routes, closing the D-09 re-auth resume loop end-to-end and shipping the rebuilt write-UX SPA in the embedded binary.**

## Performance

- **Duration:** ~62 min
- **Started:** 2026-07-15T14:14:00Z (approx.)
- **Completed:** 2026-07-15T15:16:00Z
- **Tasks:** 3
- **Files modified:** 12 source/test files + the `internal/webauth/static/` embedded SPA tree (34 files) + 1 deferred-items note

## Accomplishments

- `WriteSurfaces.svelte` is the single route-level write host: it owns which sheet is open, the edit/delete/share targets, and exposes the plan's EXACT `bind:this` contract — `openCreate()`, `openEdit(id)`, `requestDelete(id, kind)`, `requestShare(memory, kind)`, `reopenFromResume(env)` — as `export function` instance methods, no "or equivalent" ambiguity.
- `openEdit(id)` always fetches the FULL record via `queryClient.fetchQuery(['getMemory', id], () => engram.getMemory({id}))` BEFORE opening the edit sheet — list/search rows are summary-shaped (server clears content when `full=false`), so this closes the data-loss risk of prefilling Save from an empty body. A fetch error toasts and never opens a blank edit.
- `requestShare(memory, kind)` reads visibility straight off the passed `Memory` and is a genuine no-op for an already-shared record (normalizing a stored `''` as private) — the second no-op layer behind row/detail's own already-hidden Share item.
- SC3 now covers the INLINE delete/share actions, not just the forms: delete/share await the mutation's settlement and the host is the SOLE closer — the target (and thus the dialog/banner) clears ONLY on success. A terminal `Unauthenticated`/`PermissionDenied` RETAINS the target so the surface stays visibly open with the same inline re-auth CTA the forms use, and the mutation fires exactly once (no host-driven auto-replay; delete/set-visibility are id-idempotent so a manual retry can't duplicate anything).
- `reopenFromResume(env)` reopens the correct sheet and passes `resumeValues`/`resumeDirtyPaths` in as PROPS — WriteSurfaces never peeks or deletes the envelope itself; it only relays the form's `onresumeapplied` acknowledgement up through its own `onresumeapplied` prop.
- All three routes (`observe`, `search`, `discovery`) mount `WriteSurfaces` via `bind:this`, thread `onedit`/`ondelete`/`onshare` into `MemoryList`/`MemoryDetail`, and default the create scope appropriately per route; the discovery route passes NO `onedit` (D-04) and only seeds its create scope from an already-`discovery:`-prefixed `?scope` param, never a raw memory scope.
- The `/ui/` root `+page.svelte` — the actual OIDC callback landing target — now `peekResume()`s on mount and `goto(base + normalizeReturnPath(env.returnPath))` WITHOUT deleting the envelope, rejecting and discarding anything that fails `isAllowedDestination`. Each write route separately `peekResume()`s on mount and, on a kind match, calls `writeSurfaces.reopenFromResume(env)`; `consumeResume()` fires only via `WriteSurfaces`' `onresumeapplied` passthrough, so deletion happens exactly once and only after the form has actually applied the restored values.
- `task ui:build` rebuilt the SvelteKit adapter-static bundle and vendored it into `internal/webauth/static/`; a second `task ui:build` immediately after confirmed `git diff --exit-code -- internal/webauth/static/` is clean (byte-reproducible), matching what CI's `ui-drift` job asserts.
- 30 new browser tests across `WriteSurfaces.browser.test.ts` (15), `page.browser.test.ts` (5, the REAL root landing), `observe.browser.test.ts` (4), `search.browser.test.ts` (2), and `discovery.browser.test.ts` (4) — `pnpm test` (full node+browser suite, 201 tests) and `pnpm check` (0 errors) both green.

## Task Commits

Each task was committed atomically:

1. **Task 1: WriteSurfaces host — orchestrate sheets, delete dialog, share warning, New entry** - `48234caa` (feat)
2. **Task 2: Wire observe/search/discovery routes to WriteSurfaces + row/detail callbacks + consume the re-auth resume envelope** - `60d26ccf` (feat)
3. **Task 3: Rebuild + commit the embedded SPA (task ui:build)** - `2143d4a7` (chore)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `ui/src/lib/components/WriteSurfaces.svelte` - route-level write host (openCreate/openEdit/requestDelete/requestShare/reopenFromResume)
- `ui/src/lib/components/WriteSurfaces.browser.test.ts` - 15 browser tests
- `ui/src/routes/+page.svelte` - `/ui/` root landing: peekResume + goto(base+normalizeReturnPath), no delete
- `ui/src/routes/page.browser.test.ts` - 5 browser tests (real root landing)
- `ui/src/routes/observe/+page.svelte` - WriteSurfaces mount + onedit/ondelete/onshare threading + resume recovery
- `ui/src/routes/observe/observe.browser.test.ts` - 4 browser tests
- `ui/src/routes/search/+page.svelte` - same wiring for the search route
- `ui/src/routes/search/search.browser.test.ts` - 2 browser tests
- `ui/src/routes/discovery/+page.svelte` - kind=discovery wiring, no onedit, discovery:-prefixed scope guard
- `ui/src/routes/discovery/discovery.browser.test.ts` - 4 browser tests
- `internal/webauth/static/**` - rebuilt embedded SPA (34 files changed via `task ui:build`)
- `.planning/phases/19-console-write-ux/deferred-items.md` - pre-existing `lint:markdown` gap logged, not fixed (Phase 21 backlog item)

## Decisions Made

- WriteSurfaces mounts ONE `MemoryFormSheet` instance for both create and edit, remounted via `{#key mode-recordId-instanceKey}` on every open — satisfies the plan's "keyed by mode + recordId" wording while guaranteeing each open starts from a clean `$state` slate (the forms' `$state` initializers capture props once at mount, per Plan 05).
- Delete/share confirm handlers use the `.mutate(vars, {onSuccess, onError})` callback-pair pattern (wrapped in a `Promise`) rather than `mutateAsync`, matching the mocked-hook testing convention already established by `MemoryFormSheet`/`DiscoveryFormSheet` in Plan 05.
- Browser tests for `WriteSurfaces` and all three routes use `vitest-browser-svelte`'s `wrapper`/`wrapperProps` render option with a real `QueryClientProvider` + fresh `QueryClient` per test (rather than a bespoke `.svelte` test-harness file) — this is what lets `WriteSurfaces`' real `useQueryClient()`-backed `openEdit` fetch path be exercised directly while still mocking `engram.getMemory` and the delete/set-visibility hooks.

## Deviations from Plan

**1. [Rule 3 - Blocking] `task` (full lint+test gate) blocked by a pre-existing, already-tracked `lint:markdown` issue unrelated to this plan's files.**
- **Found during:** Task 3 (full `task` gate run)
- **Issue:** `task lint:markdown` (`rumdl check .`) reports 1342 issues across 139 `.planning/*.md` files — none touched by this plan (all in `ui/`/`internal/webauth/static/`). This is `.rumdl.toml`'s known missing `.planning` exclude, already documented in `PROJECT.md` as a Phase 21 CI-hygiene backlog item.
- **Fix:** Not fixed (out of scope per the scope-boundary rule — `.rumdl.toml` is explicitly Phase 21's responsibility). Verified every OTHER sub-gate independently: `task lint:go` (0 issues), `task lint:yaml`, `task lint:actions`, `task lint:python`, and `task test` (all Go + Python suites green) all pass in isolation. `task ui:build` is confirmed byte-reproducible via a second run + `git diff --exit-code`, satisfying the actual intent of T-19-65 (CI's dedicated `ui-drift` job does not run `lint:markdown`).
- **Files modified:** `.planning/phases/19-console-write-ux/deferred-items.md` (new — documents the finding)
- **Verification:** `task lint:go`, `task lint:yaml`, `task lint:actions`, `task lint:python`, `task test` all exit 0 independently; `task ui:build && git diff --exit-code -- internal/webauth/static/` clean.
- **Committed in:** part of the final metadata commit (deferred-items.md is a phase artifact, not a task commit)

---

**Total deviations:** 1 documented-and-deferred (pre-existing, out-of-scope repo-wide lint gap)
**Impact on plan:** No impact on this plan's own deliverables — every gate this plan's changes can actually affect (Go lint/test, UI check/test, SPA build reproducibility) is green.

## Issues Encountered

- `vitest-browser-svelte`'s `render()` third argument (`SetupOptions`) supports a `wrapper`/`wrapperProps` pair that mounts `<Wrapper {...wrapperProps}><Component {...componentProps} /></Wrapper>` and still returns the INNER component's exports via `screen.component` — this made `WriteSurfaces`' real `useQueryClient()` call testable with a genuine `QueryClientProvider` without writing a bespoke test-harness `.svelte` file. Discovered by reading `@testing-library/svelte-core`'s `wrapper-scaffold.svelte`/`render.d.ts` source directly (not documented in `vitest-browser-svelte`'s own README).
- Several ambiguous-locator test failures (`getByText('New memory')`/`getByText('New discovery')` matching both the toolbar button AND the sheet's own heading) were fixed by scoping to `getByRole('heading', {name: ...})` instead — a direct consequence of WriteSurfaces now rendering both the trigger button and the opened sheet's title with identical copy in the same DOM tree, which the isolated Plan-05 form tests never had to disambiguate.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Phase 19 (console-write-ux) is now feature-complete: SC1 is a working console flow across observe/search/discovery (create/edit/delete/set-visibility/schedule for memory, create/delete/set-visibility for discovery) over the CSRF-guarded, retry-once write lane, with confirm/warning gates and optimistic rollback, and the rebuilt SPA is committed so the shipped binary carries it.
- Manual UAT against a live `engram serve` + Qdrant is explicitly deferred to `/gsd-verify-work` (coverage item D8 above): create/edit/delete/share/schedule a memory and create/delete/share a discovery; confirm the CSRF header, the create-as-shared two-call flow lands SHARED, and the retry/re-auth + sessionStorage-draft flow on a rotated/expired session.
- The pre-existing `lint:markdown`/`.rumdl.toml` `.planning`-exclude gap (deferred-items.md) remains open for the already-scoped Phase 21 CI-hygiene work — not a Phase 19 blocker.
- No other blockers.

---
*Phase: 19-console-write-ux*
*Completed: 2026-07-15*

## Self-Check: PASSED

All 12 created/modified source files plus `deferred-items.md` and this SUMMARY.md verified present on disk. All 3 task commits (`48234caa`, `60d26ccf`, `2143d4a7`) verified present in `git log --oneline --all`.
