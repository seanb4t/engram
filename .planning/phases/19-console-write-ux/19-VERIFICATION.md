---
phase: 19-console-write-ux
verified: 2026-07-15T15:32:13Z
status: human_needed
score: 14/14 must-haves verified
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "Against a live `engram serve` + Qdrant + real OIDC session: create a memory, edit it, delete it, change its visibility (private→shared), and schedule a memory; create a discovery, delete it, and change its visibility."
    expected: "Each operation lands the corresponding write RPC (StoreMemory/UpdateMemory/DeleteMemory/SetVisibility/ScheduleMemory/StoreDiscovery), the list/detail panes reflect the change, and the console never silently drops a write."
    why_human: "Requires a running Qdrant + engram server + real OIDC IdP; no automated harness exercises the real Connect wire in this repo's test suite (unit/browser tests use createRouterTransport fakes and mocked hooks)."
  - test: "Confirm the CSRF header is actually present and accepted server-side on a real write request (not just asserted in the interceptor's own unit test)."
    expected: "A real write request carries `X-CSRF-Token` matching the `engram_csrf` cookie and the server's double-submit check (`connectcsrf.go`) accepts it."
    why_human: "Requires a live server round-trip; the interceptor unit tests only prove the client-side header-setting behavior against a fake transport."
  - test: "Trigger create-as-shared (or schedule-as-shared) against a live server and confirm the record actually lands with `visibility=shared` (two real RPCs: Store*/Schedule then SetVisibility)."
    expected: "The two-call composite is observed on the wire and the record is queryable as shared afterward."
    why_human: "The composite's call-count/sequencing is unit-tested against a fake client; the actual server-side visibility persistence needs a live store."
  - test: "Force a real session rotation/expiry (e.g. wait out or invalidate the session cookie) mid-write and observe: (a) the transparent single auth-race retry, (b) on terminal failure, the inline re-auth prompt for a form (create/edit) preserving typed input across the `/auth/login` → `/ui/` OIDC round-trip, and (c) on terminal failure of an inline delete/share, the console lands on `/ui/` home (not the originating filtered route) per the documented in-SPA-only scope (WR-01 in 19-REVIEW.md)."
    expected: "Retry is invisible on a recoverable race; a hard-expired session surfaces the re-auth CTA; form input survives the real redirect via the resume envelope; delete/share do not auto-replay and the operator re-initiates them after landing back on `/ui/`."
    why_human: "Requires manipulating a real session's expiry/rotation against the live auth stack; unit/browser tests substitute fake ConnectError codes and never exercise the real OIDC callback."
---

# Phase 19: Console Write UX Verification Report

**Phase Goal:** An operator can create, edit, delete, re-share, and schedule memories/discoveries directly from the console, with write failures handled gracefully rather than losing their input.
**Verified:** 2026-07-15T15:32:13Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | (SC1) Operator can create/edit/delete/change visibility/schedule a memory or discovery from the console UI, backed by the Connect write lane | ✓ VERIFIED | `ui/src/lib/mutations/memory.ts`/`discovery.ts` (8 `createMutation` hooks) wired into `MemoryFormSheet.svelte`/`DiscoveryFormSheet.svelte` → `WriteSurfaces.svelte` → `observe`/`search`/`discovery` routes (`onedit`/`ondelete`/`onshare` threaded to bound `WriteSurfaces` methods). `cd ui && pnpm test` → 201/201 tests pass; `pnpm check` → 0 errors. |
| 2 | (SC2) Console attaches the CSRF token to every write request automatically | ✓ VERIFIED | `ui/src/lib/interceptors/csrf.ts` (`attachCsrf`) composed into `engramWrite`'s transport in `client.ts:23` (`interceptors: [retryOnce, attachCsrf]`); cookie/header names (`engram_csrf`/`X-CSRF-Token`) match `internal/webauth/csrf.go`'s exported constants verbatim. `csrf.test.ts` + composed test in `client.test.ts` pass. |
| 3 | (SC3a) A write failing with an auth-class error is silently retried once (opportunistic auth-race retry, re-reads session/CSRF cookie) | ✓ VERIFIED | `ui/src/lib/interceptors/retryOnce.ts`: retries exactly once on `{Unauthenticated, PermissionDenied}`, rethrows unchanged on second failure; `[retryOnce, attachCsrf]` order (retryOnce outer) proven by a composed test that mutates `document.cookie` mid-flight and asserts the retry re-reads it. `retryOnce.test.ts` (4 tests) + composed test pass. |
| 4 | (SC3b) On terminal auth failure the operator is prompted to re-authenticate without losing in-flight **form** input, preserved across the `/auth/login` OIDC redirect via a `sessionStorage` resume envelope | ✓ VERIFIED | `ui/src/lib/resume.ts` (versioned+TTL+shape-validated envelope; `persistResume`/`peekResume`/`consumeResume`/`normalizeReturnPath`/`isAllowedDestination`); `MemoryFormSheet.svelte`/`DiscoveryFormSheet.svelte` call `persistResume(...)` before `redirectToLogin()` on a post-retry hard failure and restore only via `resumeValues`/`resumeDirtyPaths` props + `onresumeapplied` (never self-read/delete). `/ui/` root `+page.svelte` peeks+`goto()`s without deleting; each route's `reopenFromResume` + `consumeResume()`-after-ack completes the single-owner lifecycle. `resume.test.ts` (14 tests) + form D-09 describe blocks + route-level `page.browser.test.ts`/`observe.browser.test.ts`/`search.browser.test.ts`/`discovery.browser.test.ts` all pass. |
| 5 | Create-as-shared / schedule-as-shared is a two-call composite (Store*/Schedule then SetVisibility) that never risks a duplicate create on a secondary auth failure | ✓ VERIFIED | `shareIfRequested`/`createMemoryComposite`/`scheduleMemoryComposite`/`createDiscoveryComposite` in `memory.ts`/`discovery.ts`: secondary `setVisibility` failure (incl. both auth codes) is caught and returned as `{status:'created_private', id}`, never rethrown. `memory.test.ts`/`discovery.test.ts` assert exactly-one primary call on a parameterized Unauthenticated/PermissionDenied secondary failure. Forms treat `created_private` as success (no resubmit). |
| 6 | Delete requires an explicit confirm (D-06); the confirm dialog is host-authoritative and does not self-close on Delete, enabling terminal-auth retention | ✓ VERIFIED | `DeleteConfirmDialog.svelte`: Cancel funnels through bits-ui `Dialog.Close`→`onOpenChange`→`oncancel`; Delete is a bare button that never touches that path — structurally cannot self-close. `WriteSurfaces.svelte` owns `deleteTarget`, clears it only on success, retains it + shows the re-auth CTA on terminal `Unauthenticated`/`PermissionDenied`. `DeleteConfirmDialog.browser.test.ts` (8 tests) + `WriteSurfaces.browser.test.ts` SC3 describe block (6 tests, parameterized over both auth codes for delete and share) pass. |
| 7 | private→shared requires an explicit acknowledged warning (D-07); already-shared records offer no re-share/unshare affordance (one-way) | ✓ VERIFIED | `ShareWarningInline.svelte` gates the `shared` intent behind `Share anyway`; `MemoryRow`/`MemoryDetail` suppress the Share item when `visibility === 'shared'`; `WriteSurfaces.requestShare(memory, kind)` reads visibility off the passed `Memory` and no-ops when already shared; edit-mode already-shared visibility is read-only and `shared` never enters the dirty mask (`useUpdateMemory` cannot emit `shared:false`). Verified in `ShareWarningInline.browser.test.ts`, `MemoryRow.browser.test.ts`, `MemoryDetail.browser.test.ts`, `MemoryFormSheet.browser.test.ts`, `WriteSurfaces.browser.test.ts`, and independently confirmed by code review (finding "Edit-visibility read-only for shared — Correct"). |
| 8 | Rule records never expose any write affordance anywhere in the console (D-05) | ✓ VERIFIED | `MemoryRow.svelte:42`/`MemoryDetail.svelte:35`: mechanical `category === 'rule'` guard suppresses the entire kebab/action surface regardless of which callbacks are passed. Tested in both components' browser test suites. |
| 9 | Discovery has no edit surface anywhere in the console (D-04) | ✓ VERIFIED | `DiscoveryFormSheet.svelte` has no edit prop/mode at all; `discovery/+page.svelte` passes no `onedit` to `MemoryList`/`MemoryDetail`/`WriteSurfaces`; `discovery.ts` exports no update/edit hook (asserted `undefined` in `discovery.test.ts`). |
| 10 | List/detail reflect writes optimistically and revert on failure (D-10) | ✓ VERIFIED | `applyUpdateOptimistic`/`applyDeleteOptimistic`/`applySetVisibilityOptimistic` snapshot via `getQueriesData` and roll back via `setQueryData` across parametrized `listMemories*`/`searchMemories*`/`getMemory` keys; create is invalidate-only. `memory.test.ts`/`discovery.test.ts` assert rollback restores exact prior cache state. |
| 11 | Row/detail edit fetches the FULL record via `GetMemory` before opening the prefilled sheet (never prefills from a summary-shaped row) | ✓ VERIFIED | `WriteSurfaces.openEdit(id)`: `queryClient.fetchQuery({queryKey:['getMemory', id], queryFn: () => engram.getMemory({id})})` resolves before `editMemory` is set / sheet opens; routes pass `onedit={(id) => writeSurfaces?.openEdit(id)}` (id-only, no summary-shaped record). Verified in `WriteSurfaces.browser.test.ts` and `observe.browser.test.ts`. |
| 12 | Re-vendored console gen client compiles and exposes all 6 write RPCs + Citation + Visibility; re-vendoring is reproducible and CI-drift-guarded | ✓ VERIFIED | `diff gen/ts/engram/v1/engram_pb.ts ui/src/lib/gen/engram/v1/engram_pb.ts` → empty; `ui/src/lib/gen/buf/validate/validate_pb.ts` exists; `Taskfile.yaml`/`ci.yaml` both run the identical `rm -rf`+`cp -R` re-vendor + `git diff --exit-code -- ui/src/lib/gen/`; `pnpm check` → 0 errors. |
| 13 | `--destructive` design tokens resolve to a visible, per-theme-contrast-correct color and a real `destructive` Button variant exists | ✓ VERIFIED | `ui/src/app.css`: `--destructive`/`--destructive-foreground` in both `:root`/`.dark` (foreground aliased to `var(--background)`, not hardcoded white) + `@theme` mapping; `button.svelte:13` defines `destructive: 'bg-destructive text-destructive-foreground hover:opacity-90'`. `app.css.test.ts`/`button.test.ts` pass. |
| 14 | The shipped binary actually contains the write UX (embedded SPA rebuilt, committed, byte-reproducible) | ✓ VERIFIED | `task ui:build` run fresh during this verification; `git diff --exit-code -- internal/webauth/static/` exits 0 (clean) immediately after — matches what CI's `ui-drift` job asserts. `task lint:go` (0 issues) and `task test:go` (all packages ok) also independently green. |

**Score:** 14/14 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `ui/src/lib/gen/engram/v1/engram_pb.ts` + `ui/src/lib/gen/buf/validate/validate_pb.ts` | Re-vendored write-RPC client | ✓ VERIFIED | Byte-identical to canonical `gen/ts/`; compiles clean |
| `ui/src/lib/interceptors/csrf.ts`, `retryOnce.ts` | CSRF attach + auth-race retry interceptors | ✓ VERIFIED | Composed into `engramWrite` transport in correct order |
| `ui/src/lib/client.ts` (`engramWrite` export) | Write-only client, read client untouched | ✓ VERIFIED | `engram` unchanged; `engramWrite` on `[retryOnce, attachCsrf]` |
| `ui/src/lib/components/DeleteConfirmDialog.svelte`, `ShareWarningInline.svelte` | Host-authoritative destructive/warning components | ✓ VERIFIED | Never self-close on confirm; awaitable + pending state |
| `ui/src/lib/components/MemoryRow.svelte`, `MemoryList.svelte`, `MemoryDetail.svelte` | Row/detail write affordances | ✓ VERIFIED | Non-button root; per-item gating; rule/shared suppression |
| `ui/src/lib/mutations/memory.ts`, `discovery.ts` | 8 mutation hooks + composite state machine | ✓ VERIFIED | Correctly-shaped requests; optimistic rollback; no-duplicate-create composite |
| `ui/src/lib/resume.ts` | Single-owner resume-envelope module | ✓ VERIFIED | Versioned+TTL+shape-validated; persist-only from forms |
| `ui/src/lib/components/MemoryFormSheet.svelte`, `DiscoveryFormSheet.svelte` | Slide-over create/edit forms | ✓ VERIFIED | D-01 sheet, D-07 gate, D-09 two-tier retain-and-reauth |
| `ui/src/lib/components/WriteSurfaces.svelte` | Route-level write host | ✓ VERIFIED | Exact `bind:this` API; SC3 for inline delete/share |
| `ui/src/routes/{observe,search,discovery}/+page.svelte`, `ui/src/routes/+page.svelte` | Route wiring + `/ui/` landing resume consumption | ✓ VERIFIED | Callbacks threaded; peek/goto/reopen/consume lifecycle |
| `internal/webauth/static/` | Rebuilt embedded production SPA | ✓ VERIFIED | `task ui:build` re-run during verification; diff clean |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `attachCsrf` | `engramWrite` transport | `client.ts:23` interceptor array | ✓ WIRED | `[retryOnce, attachCsrf]` |
| `retryOnce` | `attachCsrf` (cookie re-read on retry) | Composed router-transport test | ✓ WIRED | Retry carries refreshed cookie value (test-proven) |
| Row/detail `onedit`/`ondelete`/`onshare` | `WriteSurfaces` bound methods | Route `bind:this` + callback props | ✓ WIRED | `observe`/`search`/`discovery` all thread correctly (discovery omits `onedit`) |
| `MemoryFormSheet`/`DiscoveryFormSheet` submit | `useCreateMemory`/`useUpdateMemory`/`useScheduleMemory`/`useCreateDiscovery` | Direct hook call with `shared` intent | ✓ WIRED | Verified via mocked-hook browser tests |
| Mutation hooks | `engramWrite` | `mutationFn` calls matching RPC method | ✓ WIRED | Request builders + call-count tests |
| Form `persistResume` | `/ui/` root `peekResume`+`goto` | `resume.ts` sessionStorage envelope | ✓ WIRED | Real root-landing test (`page.browser.test.ts`) |
| Route `reopenFromResume` | `WriteSurfaces` → form `resumeValues` props | `onresumeapplied` → `consumeResume()` | ✓ WIRED | Exactly-once-after-ack, tested per route |
| `task ui:build` | `internal/webauth/static/` (go:embed) | `cp -R build/.` | ✓ WIRED | Re-run + diff-clean confirmed during this verification |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full UI test suite (node+browser) | `cd ui && pnpm test` | 30 files, 201 tests passed | ✓ PASS |
| TypeScript/Svelte compile gate | `cd ui && pnpm check` | 1746 files, 0 errors, 16 pre-existing warnings | ✓ PASS |
| Re-vendored gen client matches canonical | `diff gen/ts/... ui/src/lib/gen/...` | empty diff | ✓ PASS |
| Embedded SPA reproducibility | `task ui:build && git diff --exit-code -- internal/webauth/static/` | exit 0 (clean) | ✓ PASS |
| Go lint | `task lint:go` | 0 issues | ✓ PASS |
| Go test | `task test:go` | all packages ok | ✓ PASS |
| CSRF/retry drift guard present | `rg "ui/src/lib/gen" .github/workflows/ci.yaml Taskfile.yaml` | matches in both | ✓ PASS |
| No debt markers in phase-touched write-path files | `rg "TBD\|FIXME\|XXX"` across mutations/interceptors/resume/WriteSurfaces/forms/dialogs | no matches | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|--------------|--------|----------|
| REQ-console-write-ux | 19-01..19-06 | Console can create/edit/delete/re-share/schedule memories/discoveries over the Connect write lane with CSRF + auth-race retry + resume-envelope re-auth | ✓ SATISFIED | All 6 plans' `requirements: [REQ-console-write-ux]` frontmatter; roadmap SC1-3 verified above; no orphaned requirement IDs found in `REQUIREMENTS.md` for Phase 19 beyond this one |

No orphaned requirements: `REQUIREMENTS.md` maps only `REQ-console-write-ux` to Phase 19, and it is claimed by all 6 plans.

### Anti-Patterns Found

None blocking. `deferred-items.md` (created by Plan 06) documents one pre-existing, out-of-scope `lint:markdown`/`.rumdl.toml` gap spanning 139 `.planning/*.md` files across multiple phases — already tracked as a Phase 21 backlog item and independently confirmed not to touch any file this phase modified. `task lint:go`, `task test:go`, and `task ui:build` reproducibility were verified green in isolation during this verification, which is the actually-relevant gate for Phase 19's own deliverables (CI's dedicated `ui-drift` job does not run `lint:markdown`).

### Human Verification Required

1. **End-to-end write flow against a live server**
   **Test:** Against a live `engram serve` + Qdrant + real OIDC session, create/edit/delete/change-visibility/schedule a memory and create/delete/change-visibility a discovery through the console UI.
   **Expected:** Each action lands the corresponding write RPC and list/detail reflect it; no write is silently dropped.
   **Why human:** No automated harness in this repo exercises the real Connect wire (unit/browser tests use `createRouterTransport` fakes and mocked hooks).

2. **CSRF header acceptance on a real request**
   **Test:** Inspect a real write request's `X-CSRF-Token` header against a live server and confirm the server's double-submit check accepts it.
   **Expected:** Header matches the `engram_csrf` cookie; server accepts.
   **Why human:** Interceptor unit tests only prove client-side header-setting against a fake transport.

3. **Create-as-shared / schedule-as-shared lands SHARED on a live server**
   **Test:** Create (or schedule) a memory with the "share" option acknowledged; confirm the record is queryable as `visibility=shared` afterward.
   **Expected:** Two real RPCs observed (Store*/Schedule then SetVisibility); persisted visibility is shared.
   **Why human:** Composite call-sequencing is unit-tested against a fake client; actual server persistence needs a live store.

4. **Real session rotation/expiry mid-write**
   **Test:** Force a real session cookie rotation or expiry mid-write and observe the transparent retry, the terminal re-auth CTA, and the resume-envelope round-trip for a form; separately observe that a terminal inline delete/share auth failure lands the operator on `/ui/` home (documented in-SPA-only scope — see 19-REVIEW.md WR-01) rather than losing typed input.
   **Expected:** Retry is invisible on a recoverable race; hard-expired session surfaces re-auth; form input survives the real OIDC redirect; delete/share are never auto-replayed.
   **Why human:** Requires manipulating a real session against the live auth stack; unit tests substitute fake `ConnectError` codes and never exercise the real `/auth/login → /auth/callback → /ui/` round-trip.

### Gaps Summary

No blocking gaps. All 14 must-have truths derived from ROADMAP.md's 3 Success Criteria plus each plan's `must_haves` are verified in the actual codebase: the write-RPC mutation hooks, CSRF/retry transport, host-authoritative destructive/share components, resume-envelope re-auth lifecycle, route wiring, D-04/D-05/D-07 fences, and the embedded-SPA ship gate all exist, are wired end-to-end, and pass 201/201 automated tests plus a clean `pnpm check`/`task lint:go`/`task test:go`/`task ui:build` run independently re-executed during this verification (not merely trusted from SUMMARY.md).

One WARNING-level, already-documented and code-review-confirmed UX asymmetry exists (WR-01 in `19-REVIEW.md`, independently confirmed in `WriteSurfaces.svelte:188-190,222-224` — `handleDeleteReauth`/`handleShareReauth` call `redirectToLogin()` without `persistResume(...)`): an inline delete/share that terminally fails on auth and proceeds through the full `/auth/login` OIDC redirect lands the operator on `/ui/` home rather than back on their originating filtered route, because delete/share carry no typed input and were deliberately scoped (round-5 cross-AI review, `19-06-PLAN.md`) to retain-and-CTA only for the in-SPA case. This does not violate any must-have truth as written (delete/share have no "in-flight input" to lose) and was an explicit, reviewed scope decision rather than an oversight — flagged here for visibility, not as a blocking gap.

The remaining work is exclusively the live-server manual UAT the phase's own plans (19-06 coverage item D8) explicitly deferred to `/gsd-verify-work` — hence `status: human_needed` rather than `passed`.

---

*Verified: 2026-07-15T15:32:13Z*
*Verifier: Claude (gsd-verifier)*
