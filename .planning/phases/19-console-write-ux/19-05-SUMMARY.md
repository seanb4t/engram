---
phase: 19-console-write-ux
plan: 05
subsystem: ui
tags: [svelte, sheet, resume-envelope, session-storage, tanstack-query]

# Dependency graph
requires:
  - phase: 19-04-console-write-ux
    provides: "useCreateMemory/useUpdateMemory/useDeleteMemory/useSetMemoryVisibility/useScheduleMemory + useCreateDiscovery/useDeleteDiscovery/useSetDiscoveryVisibility mutation hooks, and the discriminated WriteResult ({status:'created'|'created_shared'|'created_private', id}) create/schedule-as-shared composite this form consumes"
  - phase: 19-03-console-write-ux
    provides: "ShareWarningInline.svelte's host-authoritative awaitable-onconfirm/pending/authFailure contract, embedded directly in both forms"
provides:
  - "ui/src/lib/resume.ts: single typed resume-envelope module (ResumeEnvelope/ResumeDraft types, RESUME_KEY, persistResume/peekResume/consumeResume, normalizeReturnPath, isAllowedDestination, redirectToLogin) -- versioned+TTL, runtime-shape validated on peek, persist-only from the forms (single-owner lifecycle; the route/host in Plan 06 owns peek/consume/delete)"
  - "ui/src/lib/components/MemoryFormSheet.svelte: create+edit slide-over Sheet (content/scope/category/tags/visibility/summary + a create-only schedule window); edit mode disables scope/category and computes a content/tags/summary/shared dirty-mask (Save disabled when empty); an already-shared edit record renders visibility READ-ONLY (shared never enters the dirty mask, no accidental unshare); the ShareWarningInline gate defers the `shared` intent to the Plan-04 create/schedule composite or the update mask; consumes created/created_shared/created_private as SUCCESS (no D-09 resubmit on a secondary SetVisibility failure); D-09 inline re-auth on a post-retry Unauthenticated/PermissionDenied keeps the sheet open + persists a resume envelope before redirecting, and restores via resumeValues/resumeDirtyPaths PROPS + fires onresumeapplied (never self-reads/deletes sessionStorage)"
  - "ui/src/lib/components/DiscoveryFormSheet.svelte: create-only Sheet (D-04, no edit prop) covering content/kind(map|fact)/a citation editor (>=1 row, kind file|commit|url|repo + required ref)/summary/tags/a discovery:-prefixed scope (client fail-fast)/visibility; shares the same ShareWarningInline gate, composite-result consumption, and resume.ts persist-only D-09 lifecycle as MemoryFormSheet"
affects: [19-06-route-wiring]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Single-owner resume-envelope lifecycle: the write forms call ONLY persistResume() before navigating to /auth/login; they never call peekResume/consumeResume themselves. Restoration is entirely prop-driven (resumeValues/resumeDirtyPaths props + an onresumeapplied callback), so the route/host (Plan 06) is the sole reader/deleter -- eliminates the two-owner deletion race a mount-restore approach would have, and survives the host's async edit-target refetch because it's driven by prop presence, not component mount timing"
    - "Visibility select + a deferred boolean gate: the Select's bound `visibility` value flips immediately on selection (so the UI shows the choice), but a separate `shareAcknowledged` flag (set only by ShareWarningInline's onconfirm) gates the actual `shared` intent passed to the mutation -- Cancel resets both back to private in one assignment"
    - "Edit dirty-mask via a single $derived.by comparing each field against the original `memory` prop -- shared enters the mask ONLY as `true` on a currently-private record (never `false`), and Save is disabled when the resulting object has zero keys"
    - "A thin mockable seam (resume.ts's redirectToLogin()) around window.location.assign('/auth/login'), added specifically because window.location.assign is a non-configurable own property on real browser Location instances and cannot be vi.spyOn'd directly -- browser-mode component tests mock this one function via vi.mock('$lib/resume', ...) instead of touching global navigation"

key-files:
  created:
    - ui/src/lib/resume.ts
    - ui/src/lib/resume.test.ts
    - ui/src/lib/components/MemoryFormSheet.svelte
    - ui/src/lib/components/MemoryFormSheet.browser.test.ts
    - ui/src/lib/components/DiscoveryFormSheet.svelte
    - ui/src/lib/components/DiscoveryFormSheet.browser.test.ts

key-decisions:
  - "Scope is a free-text Input (not a Select) in both forms -- scope is a hierarchical string (e.g. repo:x, discovery:repo:x), not a fixed enum; defaults from the host-supplied current-scope prop and is validated client-side (non-empty for memory create/schedule; discovery:-prefixed for discovery)"
  - "redirectToLogin() lives in resume.ts (not a new standalone module) since D-09 already centralizes the whole redirect-tier lifecycle there -- kept the new-file surface at exactly what the plan named (resume.ts/resume.test.ts + the two form+test pairs), no extra files"
  - "Visibility select uses the same one-way value+onValueChange pattern already established in ScopesSidebar.svelte (not bind:value), and citation-kind selects in DiscoveryFormSheet follow the identical convention for consistency"
  - "The tag chip input and citation-ref inputs use functional array reassignment (tags = tags.filter(...)/citations = citations.map(...)) rather than bind: into an array item's property, matching the immutable-update style already used by the mutation hooks' cache-transform functions (Plan 04)"

patterns-established:
  - "Mocked-hook browser testing for Sheet forms: vi.mock the createMutation hook module (useCreateMemory etc.) to return {mutate: vi.fn()} directly, rather than standing up a QueryClientProvider + fake engramWrite transport -- keeps form-level tests focused on field/validation/D-09 logic already-covered mutation internals stay in Plan 04's node-tier suite"

requirements-completed: [REQ-console-write-ux]

coverage:
  - id: D1
    description: "resume.ts exports a versioned+TTL resume-envelope module (persistResume/peekResume/consumeResume/normalizeReturnPath/isAllowedDestination/redirectToLogin) that runtime-validates envelope shape on peek and rejects malformed/expired/wrong-version envelopes"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/resume.test.ts (14 tests: draft round-trip with v/ts stamped, wrong-version rejection, TTL expiry rejection, malformed JSON rejection, 7 structurally-invalid-shape cases, consumeResume, normalizeReturnPath /ui stripping, isAllowedDestination allow/reject)"
        status: pass
    human_judgment: false
  - id: D2
    description: "MemoryFormSheet renders the full field set (content/scope/category/tags/visibility/summary + create-only schedule window), disables scope/category in edit mode, computes a dirty-mask update in edit mode, and blocks submit on a blank scope or an invalid schedule window"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/components/MemoryFormSheet.browser.test.ts (18 tests across create field set, share-warning gate, create+schedule, edit mode field disabling/dirty-mask/read-only-shared, created_private consumption, D-09 re-auth+resume)"
        status: pass
    human_judgment: false
  - id: D3
    description: "MemoryFormSheet's ShareWarningInline gate defers the shared intent until acknowledged, routes create+schedule+shared through useScheduleMemory (never useCreateMemory) with the shared intent preserved, and an already-shared edit record's visibility is read-only with shared never entering the dirty mask (no accidental unshare, D-07 one-way)"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/components/MemoryFormSheet.browser.test.ts (share-warning gate: 3 tests; create+schedule window: 2 tests; edit-mode already-shared read-only + private-to-shared one-way: 2 tests)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Both forms consume the Plan-04 composite's discriminated WriteResult -- created_private (secondary SetVisibility auth failure) closes the sheet as SUCCESS without persisting a resume envelope or entering the D-09 resubmit tier, so a partial failure can never duplicate the record"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/components/MemoryFormSheet.browser.test.ts + DiscoveryFormSheet.browser.test.ts, 'created_private composite result' describe blocks (1 test each)"
        status: pass
    human_judgment: false
  - id: D5
    description: "D-09 two-tier retain-and-reauth: a post-retry Unauthenticated/PermissionDenied on the PRIMARY write keeps the sheet open with entered values intact and renders the inline re-auth alert; clicking Re-authenticate persists a versioned+TTL resume envelope (base-relative returnPath) via persistResume before redirecting; the form restores ONLY via resumeValues/resumeDirtyPaths props + onresumeapplied, never self-reading/deleting sessionStorage"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/components/MemoryFormSheet.browser.test.ts + DiscoveryFormSheet.browser.test.ts, 'D-09 inline re-auth + resume' describe blocks (3 tests each: keep-open-with-values, persist-before-redirect, prop-driven-restore-without-self-read)"
        status: pass
    human_judgment: false
  - id: D6
    description: "DiscoveryFormSheet is create-only (no edit prop) and enforces the server-required discovery contract client-side: non-empty content, a discovery:-prefixed scope, and >=1 typed citation (kind+ref) before submit -- sends typed Citation[] (URL as {kind:'url',ref}, never a bare string) to useCreateDiscovery"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/components/DiscoveryFormSheet.browser.test.ts (11 tests: field set + no-edit-mode, 3 fail-fast tests, 2 submit-shape tests, created_private, 3 D-09 tests)"
        status: pass
    human_judgment: false

# Metrics
duration: 35min
completed: 2026-07-15
status: complete
---

# Phase 19 Plan 05: Write Forms (MemoryFormSheet + DiscoveryFormSheet + resume.ts) Summary

**Slide-over create/edit forms driving the Plan-04 mutation hooks, backed by a single typed resume-envelope module that survives a real OIDC re-auth redirect without any form ever reading or deleting its own sessionStorage.**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-07-15T14:32:00Z (approx.)
- **Completed:** 2026-07-15T14:51:00Z
- **Tasks:** 2
- **Files modified:** 6 (all new)

## Accomplishments

- `ui/src/lib/resume.ts` centralizes the entire D-09 resume-envelope lifecycle in one module: a versioned (`v`) + TTL-stamped (`ts`, 10min) `ResumeEnvelope`, a `ResumeDraft` type (`Omit<ResumeEnvelope,'v'|'ts'>`) so call sites never pass the stamped fields, `peekResume` that rejects bad JSON/wrong-version/expired/structurally-invalid envelopes before ever returning one, `normalizeReturnPath`/`isAllowedDestination` closing the `/ui/ui/` double-prefix and open-redirect risks, and `redirectToLogin()` as a mockable navigation seam.
- `MemoryFormSheet.svelte` is a single component covering both create and edit: content/scope/category/tags/visibility/summary always render, a "schedule this memory" window appears create-only, and edit mode disables scope/category (read-only display, prefilled from the record) and never shows the schedule toggle. Edit submission computes a `$derived.by` dirty-mask against the original record (content/tags/summary + `shared` only as a private-to-shared one-way transition) and disables Save when nothing changed.
- The `ShareWarningInline` share gate is identical in both forms: selecting "shared" flips the visible select immediately but the `shared` intent used for submit stays `false` until `Share anyway` is clicked (a `shareAcknowledged` flag), and Cancel reverts both to private in one step. An already-shared record in edit mode renders visibility as a fixed read-only "shared" label with no select at all, so `shared` can never enter that record's dirty mask (`useUpdateMemory` literally cannot emit `shared:false`).
- Both forms consume the Plan-04 composite's discriminated `WriteResult`: `created`/`created_shared`/`created_private` all close the sheet as success; only a rejected PRIMARY create/schedule/update promise reaches the D-09 hard-auth branch, so a secondary `SetVisibility` failure (`created_private`) can never trigger a resubmit or a resume-envelope persist (no duplicate-create risk).
- D-09's redirect tier is honest end-to-end: on a post-retry `Unauthenticated`/`PermissionDenied`, the sheet stays open with all entered `$state` intact and renders the inline `write failed — session expired. re-authenticate to continue.` alert; clicking `Re-authenticate` calls `persistResume(...)` with a base-relative `returnPath` (create mode stores all entered values, edit mode stores only the dirty fields + `dirtyPaths`) and then `redirectToLogin()`. The form never reads or deletes `sessionStorage` itself — restoration is entirely prop-driven (`resumeValues`/`resumeDirtyPaths` + `onresumeapplied`), ready for Plan 06's route/host to own peek/consume/delete.
- `DiscoveryFormSheet.svelte` is create-only (D-04 — no `memory`/edit prop exists at all) with a minimal citation editor (add/remove rows, each needing a typed `kind` + non-empty `ref`) and requires a `discovery:`-prefixed scope before submit, matching the server's `StoreDiscoveryRequest` contract exactly (typed `Citation[]`, never a bare URL string).
- 29 new browser tests (18 MemoryFormSheet + 11 DiscoveryFormSheet) plus 14 new node tests (resume.ts) — full suite (`pnpm test:node` + `pnpm test:browser`) is 171 tests, all green; `pnpm check` is 0 errors.

## Task Commits

Each task was committed atomically:

1. **Task 1: Resume module (resume.ts) + MemoryFormSheet (create/edit, schedule, share-warning, inline re-auth)** - `cf1ec4a4` (feat)
2. **Task 2: DiscoveryFormSheet (create-only)** - `b80d52c5` (feat)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `ui/src/lib/resume.ts` - single typed resume-envelope module (persist/peek/consume/validate + redirectToLogin seam)
- `ui/src/lib/resume.test.ts` - 14 node-tier tests
- `ui/src/lib/components/MemoryFormSheet.svelte` - create+edit slide-over form
- `ui/src/lib/components/MemoryFormSheet.browser.test.ts` - 18 browser tests
- `ui/src/lib/components/DiscoveryFormSheet.svelte` - create-only slide-over form with citation editor
- `ui/src/lib/components/DiscoveryFormSheet.browser.test.ts` - 11 browser tests

## Decisions Made

- Scope is rendered as a free-text `Input` in both forms (not a `Select`) — it's a hierarchical string (`repo:x`, `discovery:repo:x`), not a bounded enum; the UI-SPEC's "select" mention was read as non-binding given the actual field shape (Claude's discretion, documented here).
- `redirectToLogin()` was added to `resume.ts` rather than a new standalone module — D-09's whole redirect-tier lifecycle already lives there, and this kept the new-file surface exactly at what the plan named. It exists purely as a mockable seam: `window.location.assign` is a non-configurable own property on real browser `Location` instances (confirmed empirically — `vi.spyOn(window.location, 'assign')` throws `Cannot redefine property`, and `vi.spyOn(Location.prototype, 'assign')` throws `property not defined on object`), so a real Chromium browser test cannot intercept it without an indirection layer. No production behavior changed (`redirectToLogin()` still calls `window.location.assign('/auth/login')`).
- Removed SPDX headers I had initially added to `resume.ts`/`resume.test.ts` — `.licenserc.yaml`'s `paths` allowlist does not include `ui/**` at all (only `cmd/**`/`internal/**`/`proto/**`/`docs/**`/`*.md`/`skill/**`), and no other `ui/src/lib/*.ts` file in the repo carries one; matched existing convention instead of introducing an inconsistent new one.
- Both forms use the same one-way `Select` `value`+`onValueChange` pattern already established in `ScopesSidebar.svelte` (not `bind:value`), for both the category/kind/visibility selects and (in `DiscoveryFormSheet`) each citation row's kind select.

## Deviations from Plan

**1. [Claude's discretion, documented above] `redirectToLogin()` seam added to `resume.ts`.** The plan's `<action>` text described "a Re-authenticate button that triggers `/auth/login`" without mandating a literal inline `window.location.href = ...` call at each form's call site. Extracting the navigation call behind one exported function in the same file the plan already tasked with owning D-09's persist lifecycle (`resume.ts`) made the redirect step testable in real-browser Playwright-driven component tests without touching global `window.location`, which is not spy-friendly in Chromium. No new file was added beyond what the plan's `files_modified` list specified; no user-facing or production behavior changed.

No other deviations — plan executed as written, including the round-3/round-4 review-hardened contracts (single-owner resume lifecycle, `created_private` = success with no resubmit, create-only schedule, edit scope/category immutability, private-`''` prefill normalization, edit-mode read-only-shared with no `shared:false` path, non-empty/`discovery:`-prefixed scope fail-fast, typed discovery citations).

## Issues Encountered

- Two initial test-authoring missteps, both fixed before commit (no source changes needed):
  - `Locator.press(...)` is not a method on this vitest-browser-svelte version's `Locator`; switched to `userEvent.keyboard(...)` (imported from `vitest/browser`) after an explicit `.click()` to focus the target input, for the Enter-to-add-tag interaction.
  - Directly spying on `window.location.assign`/`Location.prototype.assign` fails in a real Chromium browser test (see the `redirectToLogin()` decision above) — resolved via the `resume.ts` seam rather than a DOM-API workaround.
- Svelte 5's `state_referenced_locally` warning fires on both new components' `$state` initializers that read `memory`/`defaultScope` props once at mount (e.g. `let content = $state(isEdit && memory ? memory.content : '')`). This is intentional: the host (Plan 06) keys each Sheet instance by `mode + recordId`/route so a fresh component instance mounts per edit target, making one-time initial-value capture correct rather than a reactivity bug. `svelte-check` reports these as warnings only (exit code 0, 0 errors) — documented in-source at the capture site.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Plan 06 (route wiring) can mount `MemoryFormSheet`/`DiscoveryFormSheet` directly: pass `open` (bindable), `mode`/`memory` (memory form only), `scope` (the route's current default scope), and — on the `/ui/` landing after a `peekResume()` hit — `resumeValues`/`resumeDirtyPaths` props plus an `onresumeapplied` callback that calls `consumeResume()`. Both forms are already keyed-remount-safe (`$state` captures initial props once), so Plan 06 should key each Sheet instance by `mode + (memory?.id ?? 'create')` (memory) or just mount-once (discovery, always create).
- `resume.ts`'s `isAllowedDestination`/`normalizeReturnPath` are ready for Plan 06's `/ui/` landing guard before it `goto()`s back to a persisted `returnPath`.
- Both forms' `onresumeapplied` callback is the ONLY signal the host needs to know it's safe to `consumeResume()` — the forms never call it themselves.
- No blockers.

---
*Phase: 19-console-write-ux*
*Completed: 2026-07-15*

## Self-Check: PASSED

All 6 created files (`resume.ts`, `resume.test.ts`, `MemoryFormSheet.svelte`, `MemoryFormSheet.browser.test.ts`, `DiscoveryFormSheet.svelte`, `DiscoveryFormSheet.browser.test.ts`) verified present on disk. Both task commits (`cf1ec4a4`, `b80d52c5`) verified present in `git log --oneline --all`.
