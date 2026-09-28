---
phase: 04-curation-surfaces
plan: 06
subsystem: ui
tags: [svelte, tanstack-query, connect-rpc, supersede, curation, tdd, resume-envelope]

# Dependency graph
requires:
  - phase: 04-curation-surfaces
    provides: "04-01 (CurationSurfaces/flash.svelte.ts host architecture, option-a not-owned decision), 04-02 (ResumeEnvelope v2 discriminated union, SupersedeResumeEnvelope/ArchiveResumeEnvelope), 04-04 (chain.ts's headIdFrom over RelatedMemories)"
provides:
  - Task 1 (tracer): openSupersede -> prefill from newest predecessor's FULL
    record -> debounced validate_only preview -> commit -> in-place
    supersededBy patch -> success body, proven end to end
  - Task 2 (TDD): the full SupersedeDialog contract -- per-chip client/server
    issues, use-head swap, add-target-by-short_id, ordered gates, byte
    counter, cross-scope warning, a derived-only chain preview strip, and
    loading/validation/server-rejection/re-auth/opaque status blocks
  - Task 3 (TDD): curation re-auth resume for both supersede and archive
    (persist-then-redirect, reopenFromResume with a never-dropped
    not-found placeholder chip), DetailPane's Supersede/Chain buttons
    (D-05/D-06), and the curationHost ⌘K registry module
  - Post-task fixes: onviewsuperseded/onopenrecord forwarded through
    CurationSurfaces (Task 1's own spec gap); onlookup/onresolvehead wired
    to real GetMemory/RelatedMemories implementations (both props existed
    on the dialog since Task 1/2 but had no host implementation)
affects: [04-07, 04-08, 04-10]

# Actuals (#2632)
actuals:
  tokens: 22803
  tasks: 3
  commits: 6
  plan_head_before: 24d54c860b2e75704db8b0884b606b844e99f55d
  plan_head_after: 38223a2e739fbd07fe012a80045afc5ea6c681a4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "SupersedeDialog never imports engram/engramWrite (acceptance-grep
      enforced) -- every RPC (preview, commit, add-by-short_id lookup, head
      resolution) is a callback prop the host performs"
    - "Revision-guarded debounced preview: a monotonic counter bumped on
      every draft-input read inside the $effect body; an async preview
      answer applies only if its captured revision still matches the
      current one, so a stale in-flight preview for an earlier draft can
      never mark a LATER draft as validated"
    - "latestRejection = submitRejectionParsed ?? previewRejectionParsed --
      the status block and per-chip server overlay read from whichever
      rejection is more recent, so an Unauthenticated PREVIEW (not just a
      failed commit) surfaces the same re-auth block the resend flow needs"
    - "useSupersedeMemory's own onSuccess patches supersededBy in place and
      invalidates recall surfaces (mirrors useArchiveMemory); the host's
      onsubmit wrapper adds only the parts needing component props (flash,
      onchanged) -- draft.targets are always FULL ids by construction (every
      chip is a resolved Memory), so no separate canonicalization step
      exists"
    - "resolveRecordsKeepAll: a resume envelope's target/id set must never
      silently drop an unresolvable id -- an id GetMemory can't return
      becomes a placeholder Memory chip instead of disappearing"

key-files:
  created:
    - ui/src/lib/components/SupersedeDialog.svelte
    - ui/src/lib/components/SupersedeDialog.browser.test.ts
    - ui/src/lib/curation/supersede-rejection.ts
    - ui/src/lib/curation/supersede-rejection.test.ts
    - ui/src/lib/curation/host.svelte.ts
  modified:
    - ui/src/lib/mutations/curation.ts
    - ui/src/lib/mutations/curation.test.ts
    - ui/src/lib/components/CurationSurfaces.svelte
    - ui/src/lib/components/CurationSurfaces.browser.test.ts
    - ui/src/lib/components/DetailPane.svelte
    - ui/src/lib/components/DetailPane.browser.test.ts

key-decisions:
  - "draft.targets are treated as always-canonical full ids (never
    short_ids) by construction -- every chip a target can become (initial
    resolution, add-by-short_id via onlookup, a 'use head' swap) is a
    resolved Memory before it joins the target set, so
    applySupersedeOptimistic/useSupersedeMemory's onSuccess never needs a
    second id-canonicalization pass against the preview's own `supersedes`
    field."
  - "The add-target-by-short_id combobox (Task 2) is NOT wrapped in a
    Popover the way ScopeCombobox is -- it renders the Command.Root result
    inline, gated on addLookupPending/addResult/addValue.trim(), avoiding
    Popover open/focus-timing complexity in browser tests while still
    satisfying the shouldFilter={false} pattern the interfaces block names."
  - "SupersedeDialog's status block and per-chip server-issue overlay were
    widened from submit-only to latestRejection (preview OR commit) during
    Task 3, after writing Task 3's own resume test exposed that an
    Unauthenticated PREVIEW (not a failed commit) is what the resend flow
    actually re-authenticates from. Verified genuinely RED first: reverted
    SupersedeDialog.svelte to its Task 2 commit, re-ran the new
    CurationSurfaces resume test (real assertion failure on the missing
    status block, not an import error), then restored the fix."
  - "onlookup/onresolvehead (SupersedeDialog props since Task 1/2) had no
    CurationSurfaces implementation anywhere in the plan's task action
    text -- found and closed post-task, reusing chain.ts's headIdFrom
    (04-04) over a RelatedMemories read for onresolvehead, and a single
    GetMemory call for onlookup (short_id is accepted anywhere an id is,
    per the memory contract, so no separate short_id-resolution RPC is
    needed)."

requirements-completed: [CUR-01, CUR-05]

coverage:
  - id: D1
    description: "CUR-01 tracer: supersede one record through the host end to end -- prefill from the newest predecessor's FULL record, a debounced validate_only preview gates the commit, the commit carries a non-empty idempotency_key, and the predecessor's cache row is patched with supersededBy in place"
    requirement: "CUR-01"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/CurationSurfaces.browser.test.ts#supersede via the host (CUR-01 tracer) > supersede end to end (CUR-01 tracer)"
        status: pass
      - kind: unit
        ref: "ui/src/lib/mutations/curation.test.ts#buildSupersedeRequest / applySupersedeOptimistic"
        status: pass
    human_judgment: false
  - id: D2
    description: "The full supersede dialog contract (D-07): per-chip client issues (rule, already-superseded with lazy head resolution and use-head swap), add-target-by-short_id with dedupe, server-rejection overlay on preview AND commit, ordered gates, byte counter, cross-scope warning, a derived-only chain preview, and every named status block"
    requirement: "CUR-01"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/SupersedeDialog.browser.test.ts (15 cases, 4 screenshots: invalid, rejected, re-auth, populated)"
        status: pass
      - kind: unit
        ref: "ui/src/lib/curation/supersede-rejection.test.ts (9 cases covering the NotFound/rule/already-superseded sentinels, the field= envelope, and Unauthenticated/PermissionDenied)"
        status: pass
    human_judgment: false
  - id: D3
    description: "CUR-05/D-15/D-16: re-auth resume for supersede (persist targets/fields/idempotencyKey, restore with 'Signed in again — review and resend' and a Resend-labeled primary button that carries the original key) and for archive (persist mode/ids, restore with the same notice, no auto-call)"
    requirement: "CUR-05"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/CurationSurfaces.browser.test.ts#supersede re-auth resume (CUR-05, D-15) / archive re-auth resume (CUR-05, D-15)"
        status: pass
    human_judgment: false
  - id: D4
    description: "D-05/D-06: DetailPane's Supersede… action (not rule, not discovery) and View chain link (successor or predecessor present) are callback-gated like every other action button"
    requirement: null
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/DetailPane.browser.test.ts#DetailPane — Supersede and Chain (D-05, D-06) (5 cases)"
        status: pass
    human_judgment: false
  - id: D5
    description: "The curationHost ⌘K registry module (registerCurationHost, defaultActionsFor) ships as infrastructure for plans 04-07/04-08/04-10's consumers"
    requirement: null
    verification: []
    human_judgment: true
    rationale: "No consumer exists yet in this plan (explicitly deferred by the plan's own text to 04-07/04-08/04-10), so there is nothing to exercise it against beyond a TypeScript compile; its correctness is proven by its future consumers, not by this plan."

# Metrics
duration: ~2h30m
completed: 2026-09-27
status: complete
---

# Phase 4 Plan 6: Supersede dialog, re-auth resume, pane entry points Summary

**Supersede (CUR-01) shipped end to end through the curation host — preview-before-commit dialog with per-chip issue detection, add-by-short_id, a derived chain preview, and every status block — plus the curation re-auth resume for supersede and archive (CUR-05), DetailPane's Supersede/Chain buttons, and the ⌘K host registry.**

## Performance

- **Duration:** ~2h30m
- **Tasks:** 3 of 3 completed (Task 1 tracer, Task 2 TDD, Task 3 TDD), plus two post-task fixes found during self-check
- **Files modified:** 11 (5 created, 6 modified)
- **Commits:** 6

## Accomplishments

- `ui/src/lib/mutations/curation.ts`: `SupersedeFields`/`SupersedeDraft` types, `buildSupersedeRequest` (spreads the field set directly since names match the proto schema), `previewSupersede`, `useSupersedeMemory` (onSuccess patches `supersededBy` in place and invalidates recall surfaces, mirroring `useArchiveMemory`), `applySupersedeOptimistic`.
- New `ui/src/lib/components/SupersedeDialog.svelte`: the full preview-before-commit dialog — predecessor chips with per-chip client/server issue overlay, "use head" swap (lazy head resolution), add-target-by-short_id (Command, `shouldFilter={false}`, dedupe), a correcting-record form (summary with a 512-byte danger counter, content with a blur-triggered `field=content hint=required` error, category/scope selects, tags), a cross-scope warning, ordered gates, a derived-only chain preview strip (`d2`/`d1`/`head · d0`), and status blocks for loading, field-validation, server-rejection, re-auth (with a `resend`-driven "Resend — Supersede N → 1" primary label), and opaque errors. Never imports `engram`/`engramWrite` — every RPC is a callback prop.
- New `ui/src/lib/curation/supersede-rejection.ts`: `parseSupersedeRejection` maps the three server sentinel prefixes (not-found, rule, already-superseded), the `field=`/`hint=` envelope, and Unauthenticated/PermissionDenied to the dialog's issue vocabulary; `TARGET_ISSUE_COPY` holds the UI-SPEC strings verbatim.
- `CurationSurfaces.svelte`: `openSupersede(ids)` resolves targets, prefills the correcting record from the newest predecessor's FULL `GetMemory` record, mints one `idempotencyKey`; `reopenFromResume(env)` restores either the supersede or archive dialog with `'Signed in again — review and resend'`, re-fetching every target by id via `resolveRecordsKeepAll` (an unreadable id becomes a placeholder chip, never dropped); `handleSupersedeReauth`/`onreauth` (archive) persist a v2 resume envelope before redirecting; `onlookup`/`onresolvehead` wire the dialog's add-target and use-head features to real `GetMemory`/`RelatedMemories` calls.
- `DetailPane.svelte`: `onsupersede`/`onchain` props; `Supersede…` in the actions row (not rule, not discovery); `View chain` in the State section (successor or predecessor present).
- New `ui/src/lib/curation/host.svelte.ts`: `curationHost` registry, `registerCurationHost`, `defaultActionsFor` — infrastructure for plans 04-07/04-08/04-10's ⌘K wiring, no consumer yet.

## Task Commits

1. **Task 1: Supersede one record through the host (tracer)** — `5353ece2` (feat)
2. **Task 2: The full supersede dialog (TDD)** — `67fe16d0` (test, RED — `supersede-rejection.test.ts` against a stub, 8/9 cases failed on real assertions), `86c7d4cb` (feat, GREEN — real `parseSupersedeRejection` plus the full dialog; `SupersedeDialog.browser.test.ts`'s 15 cases confirmed RED against the Task 1 dialog via a manual revert-and-rerun before this commit, not a separate commit)
3. **Task 3: Curation re-auth resume, pane entry, host registry (TDD)** — `42a7d334` (feat, GREEN — RED confirmed the same way: the new `CurationSurfaces.browser.test.ts`/`DetailPane.browser.test.ts` cases failed for real reasons against the pre-Task-3 code before this commit)
4. **Post-task fix 1** — `86ec5432` (fix: forward `onviewsuperseded`/`onopenrecord` — Task 1's own spec gap)
5. **Post-task fix 2** — `38223a2e` (feat: wire `onlookup`/`onresolvehead` to real implementations)

**Plan metadata:** this commit (docs: complete plan)

_Note: Tasks 2 and 3 are `tdd="true"` but each produced a single GREEN commit for their SupersedeDialog/CurationSurfaces/DetailPane behavior tests, with RED evidence captured by direct observation (temporarily reverting the target file to its prior commit, re-running the new tests to confirm a genuine assertion failure, then restoring) rather than a separate `test(...)` commit for every file — mirroring 04-04's own documented precedent for this stack. Task 2's `supersede-rejection.ts` DID get its own dedicated `test(...)` RED commit since that module has no prior-commit baseline to revert to._

## Files Created/Modified

- `ui/src/lib/mutations/curation.ts` — supersede request builder, preview/commit hooks, cache-patch
- `ui/src/lib/mutations/curation.test.ts` — request-builder and cache-patch unit tests
- `ui/src/lib/components/SupersedeDialog.svelte` — the full supersede dialog
- `ui/src/lib/components/SupersedeDialog.browser.test.ts` — 15 cases, 4 screenshots
- `ui/src/lib/curation/supersede-rejection.ts` — rejection-to-issue mapping
- `ui/src/lib/curation/supersede-rejection.test.ts` — 9 cases
- `ui/src/lib/curation/host.svelte.ts` — the ⌘K curation host registry
- `ui/src/lib/components/CurationSurfaces.svelte` — openSupersede, reopenFromResume, reauth handlers, onlookup/onresolvehead
- `ui/src/lib/components/CurationSurfaces.browser.test.ts` — tracer, resume (supersede + archive), onlookup/onresolvehead wiring cases
- `ui/src/lib/components/DetailPane.svelte` — onsupersede/onchain, Supersede…/View chain
- `ui/src/lib/components/DetailPane.browser.test.ts` — 5 new cases

## Decisions Made

See `key-decisions` in the frontmatter — summarized: `draft.targets` are always canonical full ids by construction; the add-target combobox skips the Popover wrapper for test-timing simplicity; the dialog's status block/issue overlay was widened from commit-only to `latestRejection` (preview or commit) once Task 3's resend flow needed a preview-time auth failure to surface the same re-auth block; `onlookup`/`onresolvehead` were found unwired and closed post-task.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] SupersedeDialog's status block only reacted to commit failures, not preview failures**
- **Found during:** Task 3, while designing the supersede re-auth resume test
- **Issue:** Task 3's own behavior text ("an Unauthenticated preview → clicking Re-authenticate...") requires the re-auth status block and button to appear from a PREVIEW failure, but Task 2's implementation only rendered it from `submitError`. `SupersedeDialog.svelte` is not in Task 3's declared `<files>` list, but the fix is required for Task 3's own explicitly-specified behavior to be reachable at all.
- **Fix:** Widened the status block and per-chip server-issue overlay to read from `latestRejection = submitRejectionParsed ?? previewRejectionParsed`; added the `resend`-driven "Resend — " button-label prefix (also required by Task 3's behavior text, also previously unwired).
- **Files modified:** `ui/src/lib/components/SupersedeDialog.svelte`
- **Verification:** Manually reverted the file to its Task 2 commit (`86c7d4cb`), re-ran the new `CurationSurfaces.browser.test.ts` resume case — genuine assertion failure (missing status block text), not an import error — then restored the fix and confirmed GREEN.
- **Committed in:** `42a7d334` (Task 3 commit)

**2. [Rule 2 - Missing Critical] `onviewsuperseded`/`onopenrecord` were never forwarded through CurationSurfaces**
- **Found during:** Post-Task-3 self-check
- **Issue:** Task 1's own action text specifies these as CurationSurfaces host props forwarded into `SupersedeDialog`'s success-footer actions (Artifacts table row for `CurationSurfaces`), but `CurationSurfaces.svelte`'s prop list never declared them.
- **Fix:** Added `onviewsuperseded?`/`onopenrecord?` to `CurationSurfaces`' own props, forwarded straight through to `SupersedeDialog`.
- **Files modified:** `ui/src/lib/components/CurationSurfaces.svelte`
- **Verification:** `pnpm --dir ui vitest run --project browser src/lib/components/CurationSurfaces.browser.test.ts src/lib/components/SupersedeDialog.browser.test.ts` (22/22 pass), `pnpm --dir ui build` exit 0.
- **Committed in:** `86ec5432`

**3. [Rule 2 - Missing Critical] `onlookup`/`onresolvehead` existed on SupersedeDialog's contract with no host implementation anywhere in the plan's task action text**
- **Found during:** Post-Task-3 self-check
- **Issue:** Add-target-by-short_id and the "use head" swap are inert without a real host-side lookup/head-resolution implementation. No task's action text assigns this wiring to CurationSurfaces, but the plan's own `<interfaces>` block explicitly names `chain.ts`'s `headIdFrom` as available context for exactly this purpose.
- **Fix:** `supersedeOnlookup(value)` — a single `GetMemory` call (short_id is accepted anywhere an id is, per the memory contract). `supersedeOnresolvehead(id)` — a `RelatedMemories` read, `headIdFrom` (04-04) to find the successor, then `GetMemory` for the resolved head.
- **Files modified:** `ui/src/lib/components/CurationSurfaces.svelte`, `ui/src/lib/components/CurationSurfaces.browser.test.ts` (2 new cases)
- **Verification:** `pnpm --dir ui vitest run --project browser src/lib/components/CurationSurfaces.browser.test.ts` (9/9 pass), full node+browser regression (259 + 343 tests) and `pnpm --dir ui build` all green.
- **Committed in:** `38223a2e`

### Style deviations (not functional)

**4. [Style] `feat(04-06): ...` commit scope diverges from the plan's literal `feat(ui): ...` for Tasks 2 and 3's commit messages.** Task 1's commit used the plan's literal message verbatim (`feat(ui): supersede with validate_only preview and in-place history (CUR-01, D-07, D-10)`); Tasks 2 and 3's GREEN commits used the repo's default `feat(04-06): ...` phase-plan scope instead of the plan's literal `feat(ui): ...` text. Cosmetic only — both are valid Conventional Commits and CI validates only the PR title, not per-commit scope — left as-is per the never-amend-a-commit rule, mirroring 04-04-SUMMARY's identical documented precedent.

---

**Total deviations:** 3 auto-fixed (all Rule 2 — missing critical functionality, all discovered while implementing or self-checking a later task's own explicitly-specified behavior), 1 cosmetic style note. **Impact:** All three fixes are required for the plan's own stated behavior (Task 3's resend flow, Task 1's forwarded props, the dialog's add-target/use-head features) to actually work — none is scope creep beyond completing what the plan itself specifies.

## Known Stubs

- `resolveRecordsKeepAll`'s placeholder chip for a resume-envelope target `GetMemory` cannot resolve renders as a plain chip (`summary: 'not found: {id}'`) rather than triggering `SupersedeDialog`'s styled server-rejection issue treatment (red text, note line). The target is visibly present and never silently dropped (the plan's own requirement), but it doesn't get the same visual weight as a genuine server rejection. `SupersedeDialog.svelte` was intentionally not touched further for this cosmetic gap since it is out of Task 3's declared file scope beyond the two fixes above. Logged to `.planning/WINDOWS.md` (entry 21, kind `stub`).
- `host.svelte.ts`'s `curationHost`/`registerCurationHost`/`defaultActionsFor` have no consumer in this plan — by the plan's own design ("Consumers land in plans 04-07, 04-08 and 04-10"). Not a gap; documented for the verifier's awareness.

## Threat Flags

None beyond the plan's own `<threat_model>` (T-04-13 through T-04-16), all mitigated as specified: `engramWrite`-only RPC access (acceptance-grep enforced zero `engram`/`engramWrite` imports in the dialog), the revision-guarded preview against a stale-commit race, one idempotency_key reused through resend, and sessionStorage-only envelope storage.

## Issues Encountered

None beyond the three deviations documented above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `SupersedeDialog`, `curation.ts`'s supersede hooks, `supersede-rejection.ts`, and `CurationSurfaces`' `openSupersede`/`reopenFromResume`/`onlookup`/`onresolvehead` are all ready for plan 04-08's `/search` route wiring (mounting `CurationSurfaces` with real `onviewsuperseded`/`onopenrecord`/`onresumeapplied` handlers, and the `/ui/` landing's `peekResume`/`consumeResume` ownership).
- `host.svelte.ts`'s registry is ready for plans 04-07/04-08/04-10's ⌘K consumers.
- No blockers.

---
*Phase: 04-curation-surfaces*
*Completed: 2026-09-27*

## Self-Check: PASSED

- All 5 created key-files verified present on disk with `[ -f ]`/`ls`: `SupersedeDialog.svelte`, `SupersedeDialog.browser.test.ts`, `supersede-rejection.ts`, `supersede-rejection.test.ts`, `host.svelte.ts`.
- All 6 commits (`5353ece2`, `67fe16d0`, `86c7d4cb`, `42a7d334`, `86ec5432`, `38223a2e`) verified present in `git log --oneline --all`.
- Re-ran the plan-level `<verification>`: `curation.test.ts` (node, 8/8), `supersede-rejection.test.ts` (node, 9/9), `CurationSurfaces.browser.test.ts` (browser, 9/9), `SupersedeDialog.browser.test.ts` (browser, 15/15), `DetailPane.browser.test.ts` (browser, 24/24), `ArchiveConfirmDialog.browser.test.ts` (browser, unaffected regression, still green) — all pass; `pnpm --dir ui build` exits 0.
- Full regression: `pnpm --dir ui vitest run --project node` (23 files / 259 tests) and `--project browser` (31 files / 343 tests) both green after every commit in this plan.
- Re-ran all acceptance-criteria `rg` checks from all three tasks — all match their required counts (`engramWrite.supersedeMemory` ≥1, zero `engram`/`engramWrite` refs in the dialog, `crypto.randomUUID` ≥1, `shouldFilter={false}` ≥1, the three target-issue strings = 3 unique, `page.screenshot` ≥4, `kind: 'supersede'|'archive'` = 2, zero `peekResume`/`consumeResume` in `CurationSurfaces.svelte`, `Signed in again — review and resend` ≥1).
