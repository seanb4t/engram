---
phase: 19
reviewers: [codex, opencode, antigravity]
reviewed_at: 2026-07-14T18:55:31Z
review_round: 3
reviewed_commit: 4bd98fc8
opencode_model: openrouter/x-ai/grok-4.5
codex_cli: 0.144.1
agy_cli: 1.1.2
plans_reviewed: [19-01-PLAN.md, 19-02-PLAN.md, 19-03-PLAN.md, 19-04-PLAN.md, 19-05-PLAN.md, 19-06-PLAN.md]
---

# Cross-AI Plan Review — Phase 19 (Round 3)

Round 3 review of the round-2 revised plans (commit `4bd98fc8`) against live source. All three reviewers confirm the round-2 fixes HOLD in source. The remaining findings are second-order: Codex (highest-signal) found the newly-added composite-mutation and resume-envelope mechanisms have incomplete edge-case/lifecycle specs (3 new HIGH); grok-4.5 rates the plans essentially ready (LOW/MEDIUM residuals); antigravity approved. One MEDIUM is convergent (Codex+grok): the authoritative SC3/D-08 requirement text still promises the old reseal mechanism.

---

## Codex Review

# Round 3 Review — Phase 19 Console Write UX

## Overall assessment

The plans are substantially stronger after two review rounds, and most round-2 corrections are grounded in source. However, two cross-plan defects remain blocking:

1. A successful create/schedule followed by a failed `SetVisibility` can be treated as a failed whole mutation; re-authenticated resubmission would create a duplicate.
2. The resume-envelope lifecycle is split between the route and form, so edit drafts may never restore or may be cleared before restoration.

Overall risk is **HIGH** until Plans 04–06 resolve these issues.

## Round-2 fix verification

| Fix | Verdict | Evidence |
|---|---|---|
| TS visibility members | Confirmed | Generated members are `UNSPECIFIED`, `PRIVATE`, and `SHARED`. [engram_pb.ts](/Volumes/Code/github.com/seanb4t/engram/gen/ts/engram/v1/engram_pb.ts:927) |
| Scheduled+shared composite | Correctly planned | `ScheduleMemoryRequest` has no visibility field, so Schedule→SetVisibility is necessary. [engram.proto](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:203), [19-04-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-04-PLAN.md:87) |
| Retry semantics | Source-correct, specification still stale | Errored responses skip resealing, and expired sessions fail before handlers. [connectreseal.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectreseal.go:39), [resolver.go](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/resolver.go:49) |
| Resume through `/ui/` | Mechanism added, not yet sound | Callback does land at `/ui/`, but restore/clear ownership is racy. [handlers.go](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/handlers.go:187) |
| Full-record edit fetch | Confirmed correct | List/search clear content; `GetMemory` returns the full record. [connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:70), [connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:202) |
| Visibility-aware Share | Correctly planned | Row/detail pass `Memory`; host rejects already-shared input. [19-03-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-03-PLAN.md:112), [19-06-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-06-PLAN.md:97) |
| No Make-private path | Consistent with locked D-07 | D-07 explicitly describes the console exposure as one-way, even though the store technically supports unsharing. [19-CONTEXT.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-CONTEXT.md:62), [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1571) |

---

## 19-01 — Foundation

### Summary

This plan is technically sound and resolves the stale-client prerequisite properly. The structure-preserving generated tree, barrel export, compile gate, CI drift check, and destructive button variant address the actual source gaps.

### Strengths

- The canonical generated client imports `../../buf/validate/validate_pb`, while the current UI copy has only the five read RPCs. Preserving the generated directory tree is therefore necessary. [engram_pb.ts](/Volumes/Code/github.com/seanb4t/engram/gen/ts/engram/v1/engram_pb.ts:12), [current UI client](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/gen/engram_pb.ts:396)
- Plugin-scoped `include_imports` is appropriately limited to the ES plugin; the current Buf configuration cleanly separates Go and TS plugins. [buf.gen.yaml](/Volumes/Code/github.com/seanb4t/engram/buf.gen.yaml:14)
- The destructive token and component variant are both required: neither currently exists. [app.css](/Volumes/Code/github.com/seanb4t/engram/ui/src/app.css:5), [button.svelte](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/components/ui/button/button.svelte:8)
- `pnpm check` is the right acceptance gate because a byte-equal copy can still have unresolved imports.

### Concerns

- **LOW — Stale generated files may survive.** The proposed command removes only `engram/` and `buf/`, then copies all of `gen/ts`. If future imports generate another top-level directory and that dependency is later removed, the stale UI directory will remain and the CI copy will not detect it. [19-01-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-01-PLAN.md:83)
- **LOW — Task 1’s direct verification is only an `rg` presence check.** The stronger compile/idempotency verification appears in Task 2, so the plan-level gate is adequate, but Task 1 alone does not prove the generated tree is self-consistent.

### Suggestions

- Make generated-output cleanup exact: use Buf `clean: true`, or reserve a dedicated generated subdirectory and replace it wholesale while leaving the hand-authored barrel outside it.
- Add an idempotency assertion: run `task proto:gen` twice and confirm the second run produces no diff in both `gen/` and `ui/src/lib/gen/`.

### Risk assessment

**LOW.** The core mechanism is correct; remaining concerns are future-proofing rather than phase blockers.

---

## 19-02 — Write transport

### Summary

The interceptor design matches the live backend. The revised “opportunistic auth-race retry” language is accurate, and retrying only `Unauthenticated`/`PermissionDenied` is currently safe because both originate before the business handler.

### Strengths

- Auth rejection occurs before `next`, producing `Unauthenticated`. [connectauth.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectauth.go:18)
- CSRF rejection similarly occurs before the write handler and produces `PermissionDenied`. [connectcsrf.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectcsrf.go:58)
- Business errors do not map to those two codes; they map to NotFound, InvalidArgument, FailedPrecondition, cancellation, or Internal. [connecterror.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connecterror.go:45)
- The CSRF token is owner-bound and stable across reseals, confirming that “CSRF rotation recovery” would be inaccurate. [csrf.go](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/csrf.go:57), [reseal.go](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/reseal.go:37)
- `[retryOnce, attachCsrf]` is the correct composition for rereading the cookie during the second attempt.

### Concerns

- **MEDIUM — The authoritative requirement remains inconsistent.** The plan correctly says retry does not cause resealing, but ROADMAP, REQUIREMENTS, and locked D-08 still promise “retry through a re-seal.” [ROADMAP.md](/Volumes/Code/github.com/seanb4t/engram/.planning/ROADMAP.md:408), [REQUIREMENTS.md](/Volumes/Code/github.com/seanb4t/engram/.planning/REQUIREMENTS.md:53), [19-CONTEXT.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-CONTEXT.md:69)
- **LOW — Interceptor order is only structurally tested.** A literal-array assertion proves configuration, but not that retry actually rereads a changed cookie.

### Suggestions

- Amend the authoritative success criterion and requirement to “single opportunistic auth-race retry,” or explicitly mark the original reseal promise as infeasible under the shipped backend.
- Add one composed-interceptor test: first attempt fails, the cookie changes, retry observes the new token, and exactly two calls occur.

### Risk assessment

**MEDIUM.** Implementation is sound, but the phase would close against semantics different from its authoritative requirement.

---

## 19-03 — Action affordances

### Summary

The non-button row shell, mechanical rule fence, corrected sharing copy, and visibility-aware callback are all good revisions. One callback-rendering ambiguity can still expose a discovery Edit action.

### Strengths

- Restructuring is necessary because the current entire row is a button. [MemoryRow.svelte](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/components/MemoryRow.svelte:19)
- Mechanical rule suppression is appropriate because generated `Memory.category` is a general string, not the narrower UI `Category` type.
- Hiding Share for already-shared records and passing the complete `Memory` gives both the component and host enough information to enforce the no-op.
- The corrected warning accurately separates reversible unsharing from irreversible prior disclosure.

### Concerns

- **HIGH — Individual actions are not explicitly conditional on their own callbacks.** The plan says render the kebab when “the callbacks” are provided, then describes all three menu items calling optional callbacks. [19-03-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-03-PLAN.md:112) Plan 06 deliberately passes delete/share but no edit on the discovery route. [19-06-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-06-PLAN.md:134) Unless each item is separately gated, discovery rows can display Edit and attempt to invoke `undefined`.
- **MEDIUM — The same ambiguity exists in `MemoryDetail`.** “Render these only when the callbacks are provided” should mean each button checks its matching callback, not that any callback enables the whole action group. [19-03-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-03-PLAN.md:143)

### Suggestions

- Specify and test individual gates:

  - `{#if onedit}` → Edit
  - `{#if ondelete}` → Delete
  - `{#if onshare && memory.visibility !== 'shared'}` → Share

- Add row/detail tests with only delete/share callbacks and assert Edit is absent. That directly covers discovery-route behavior.

### Risk assessment

**MEDIUM.** The defect is localized, but it violates the explicit no-discovery-edit boundary.

---

## 19-04 — Mutation hooks

### Summary

RPC shaping, enum names, typed citations, dirty masks, multi-query rollback, and the scheduled+shared composite are well designed. The unresolved partial-success semantics can nevertheless cause duplicate records after re-authentication.

### Strengths

- `Visibility.SHARED`/`PRIVATE` matches the generated TypeScript enum exactly. [engram_pb.ts](/Volumes/Code/github.com/seanb4t/engram/gen/ts/engram/v1/engram_pb.ts:927)
- Both StoreMemory and ScheduleMemory lack a visibility field, validating the explicit second RPC. [engram.proto](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:104), [engram.proto](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:203)
- `FieldMask` is the sole update-presence mechanism, so sending only dirty paths is important. [engram.proto](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:150)
- Multi-key snapshots are necessary because visibility is embedded in parametrized list keys. [queries.ts](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/queries.ts:34)

### Concerns

- **HIGH — Secondary visibility failure and D-09 can duplicate creates.** The plan says Store/Schedule success followed by `SetVisibility` failure keeps the private record and shows a partial-success toast. [19-04-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-04-PLAN.md:83) It also says every surviving auth-class error reaches the form’s hard-auth path. [19-04-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-04-PLAN.md:89) If the secondary call returns `Unauthenticated`, restoring and resubmitting the original form will issue Store/Schedule again. Neither request has a client idempotency key or record id. [engram.proto](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:104), [engram.proto](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:203)
- **MEDIUM — Partial-success toast behavior is underspecified.** If the secondary error is thrown, generic `write failed` and inline re-auth can appear alongside `created (private) — sharing failed`. If it is swallowed, the hook must return a discriminated partial-success result so the form closes without treating the whole create as failed.
- **LOW — Optimistic edit coverage is less explicit than delete/visibility coverage.** The behavior section only expressly patches `getMemory`, although edited summary/tags are visible in list/search rows.

### Suggestions

- Treat the composite as a state machine:

  - Primary Store/Schedule failure: propagate normally.
  - Primary success + visibility success: full success.
  - Primary success + visibility failure: return `{status: "created_private", id}`; do not enter whole-create re-auth/resubmit.

- Alternatively, persist a visibility-only continuation containing the returned ID, never the original create command.
- Add tests where the second RPC fails with both auth codes and prove no second Store/Schedule occurs.
- Test optimistic edit across `{memory}`, `{memories}`, and `{discoveries}` cache shapes, including bigint `total`.

### Risk assessment

**HIGH.** Without an explicit partial-success contract, D-09 can turn a recoverable sharing failure into duplicate stored records.

---

## 19-05 — Form sheets

### Summary

The form contracts now match the RPCs well, including create-only scheduling, immutable edit fields, typed discovery citations, and dirty masks. The resume design still has lifecycle, data-integrity, and validation gaps.

### Strengths

- Disabling scope/category during edit matches `UpdateMemoryRequest`, which permits only content/shared/tags/summary. [engram.proto](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:164)
- Schedule is correctly create-only because its request has no ID.
- Discovery validation matches the server’s content, kind, scope, and citation checks. [tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:575)
- Dirty-path computation protects against unnecessary re-embedding and summary-provenance changes.

### Concerns

- **HIGH — Mount-only restoration is incompatible with the planned host lifecycle.** Plan 05 restores an edit envelope only “on this component’s mount” when mode/record ID already match. [19-05-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-05-PLAN.md:89) Plan 06 says `WriteSurfaces` renders the memory form whenever `kind=memory`, then asynchronously fetches the edit target. [19-06-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-06-PLAN.md:90) The form can therefore mount initially in create mode, reject the edit envelope, and never rerun mount restoration after `editMemory` arrives.
- **HIGH — Envelope deletion has two owners.** Plan 05 says the form restores and deletes it; Plan 06 says the route calls reopen and then clears it. [19-06-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-06-PLAN.md:135) The route can clear storage before the newly opened form reads it.
- **MEDIUM — Whole-form restoration can undermine the dirty mask.** After re-auth, the host refetches the latest full record and overlays every saved value. A concurrently updated summary/content can then appear dirty even if the operator never touched it. There is no server edit-conflict mechanism. [connecterror.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connecterror.go:42)
- **MEDIUM — Memory scope validation is missing.** StoreMemory and ScheduleMemory require a non-empty scope. [engram.proto](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:106), [engram.proto](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:210) Search can supply an empty default scope. [search/+page.svelte](/Volumes/Code/github.com/seanb4t/engram/ui/src/routes/search/+page.svelte:13)
- **MEDIUM — The threat-model claim that edit envelopes contain no server data is inaccurate.** Edit values are prefilled from full `GetMemory`, so sessionStorage can contain existing memory content, not only newly typed input. `GetMemory` returns the complete record. [connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:211)
- **MEDIUM — This reverses a locked deferred decision without updating context.** Session-storage restoration is explicitly deferred in CONTEXT. [19-CONTEXT.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-CONTEXT.md:162)

### Suggestions

- Give the route/host sole ownership of storage parsing and deletion. Pass `resumeValues` and `dirtyPaths` directly into the form; delete only after the form acknowledges applying them.
- Store only dirty edit fields plus their paths, then refetch the full record and overlay those fields. Create drafts may still require all entered values.
- If restoration remains mount-based, key/conditionally mount the form by `mode + recordId`; direct props are safer.
- Require non-empty trimmed memory scope before create or schedule.
- Version and safely parse the envelope; include a timestamp/TTL and allowed-route validation.
- Update CONTEXT.md to record that the previously deferred persistence decision is now required by the actual OIDC redirect.

### Risk assessment

**HIGH.** The advertised re-auth recovery is still unreliable for edit mode and can reintroduce stale-field writes.

---

## 19-06 — Route integration and shipping

### Summary

The full-record edit fetch, visibility-aware share host, route-specific discovery fence, and embedded-SPA rebuild are strong. The plan does not yet prove or reliably implement the complete `/ui/` landing-to-restored-sheet sequence.

### Strengths

- Refetching by ID before edit directly addresses the content-clearing behavior of list/search responses.
- Passing the whole `Memory` into `requestShare` makes the already-shared no-op enforceable.
- Omitting discovery `onedit` is correct, assuming Plan 03 individually gates its action items.
- Rebuilding `internal/webauth/static/` is mandatory because CI independently rebuilds and diffs the vendored SPA. [ci.yaml](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/ci.yaml:138), [Taskfile.yaml](/Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml:21)

### Concerns

- **HIGH — Resume sequencing remains ambiguous and races with form restoration.** `reopenFromResume()` is asynchronous for edit, but the route is also told to clear the envelope after calling it. [19-06-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-06-PLAN.md:98), [19-06-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-06-PLAN.md:135)
- **MEDIUM — Return-path normalization is not specified.** SvelteKit’s base is `/ui`. [svelte.config.js](/Volumes/Code/github.com/seanb4t/engram/ui/svelte.config.js:9) `goto(base + env.returnPath)` works only if `returnPath` is guaranteed base-relative such as `/observe?...`; storing `window.location.pathname` would produce `/ui/ui/observe`.
- **MEDIUM — The claimed landing test does not actually mount the `/ui/` landing.** It seeds storage and mounts the observe route directly. [19-06-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-06-PLAN.md:138) That tests destination reopening, not root-page parsing, `goto`, base normalization, or envelope preservation during navigation.
- **MEDIUM — Search and discovery recovery are untested.** Only an observe-route test is planned, despite different scope/default and no-edit behavior.
- **LOW — Host API and discovery acceptance conflict.** The host must export `openEdit()` unconditionally, while the test says a discovery host “exposes no openEdit path.” [19-06-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-06-PLAN.md:95), [19-06-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-06-PLAN.md:101)

### Suggestions

- Introduce a shared typed `resume.ts` module responsible for:

  - safe parsing/versioning;
  - base-relative return-path normalization;
  - allowed destination validation;
  - single-consumer semantics;
  - explicit `peek` → reopen → `consume` lifecycle.

- Add a real root-page test asserting `/ui/` reads the envelope and calls `goto('/ui/observe?...')` without deleting it.
- Add destination tests for observe-edit, search-create, and discovery-create.
- Make `openEdit()` reject/no-op when `kind !== 'memory'`, and test that behavior rather than claiming the method is absent.

### Risk assessment

**HIGH.** Shipping and full-record editing are well covered, but SC3’s end-to-end re-auth recovery is not yet reliable or fully tested.

---

## Required revisions before execution

1. Define composite-mutation partial success so secondary `SetVisibility` failure can never resubmit Store/Schedule.
2. Centralize resume-envelope ownership; pass restored values directly to forms and delete only after application.
3. Store edit dirty fields/paths rather than an undifferentiated full form snapshot.
4. Add non-empty memory-scope validation.
5. Gate each row/detail action on its corresponding callback.
6. Update ROADMAP/REQUIREMENTS/CONTEXT to reflect actual opportunistic retry semantics and the newly required sessionStorage decision.
7. Add a true `/ui/` landing test plus search/discovery recovery coverage.

---

## OpenCode Review (grok-4.5 — openrouter/x-ai/grok-4.5)

Verifying round-2 claims against live source before scoring the plans.
Digging into Visibility members, form field contracts, and UI component shapes.
# Phase 19 Cross-AI Plan Review — Round 3

Verified against the working tree (not plan prose alone). Round-2 claims checked against live source first.

---

## Round-2 fix verification (source-backed)

| Claim | Verdict | Evidence |
|-------|---------|----------|
| (a) TS Visibility members are `PRIVATE`/`SHARED` | **HOLDS** | `gen/ts/engram/v1/engram_pb.ts:927-941` (`UNSPECIFIED=0`, `PRIVATE=1`, `SHARED=2`). Go uses `Visibility_VISIBILITY_*`. |
| (b) Scheduled+shared needs Schedule→SetVisibility | **HOLDS (plan)** | `ScheduleMemoryRequest` fields end at `not_after` — no visibility (`engram_pb.ts:829-889`, `engram.proto:203+`). |
| (c) SC3 = auth-race retry, not reseal-on-fail | **HOLDS** | `connectreseal.go:39-41` skips reseal on error; CSRF bound only to Owner and stable across reseals (`csrf.go:38-40`, `reseal.go:43,78`); expired session → pre-handler fail (`resolver.go:50`). |
| (d) OIDC lands on `/ui/` with no write host | **HOLDS** | `handlers.go:187` → `/ui/`; `ui/src/routes/+page.svelte` is home list only. |
| (e) List/search content cleared when `full=false` | **HOLDS** | `connectapi.go:70-79` clears `Content`; `GetMemory` returns full at `:202`. |
| (f) Share visibility-aware + hide when shared | **HOLDS (plan)** | Matches `Visibility` string field + `queries.ts:4` (`'' \| 'private' \| 'shared'`). |
| (g) Make-private rejected under D-07 | **Consistent with CONTEXT D-07; residual product tension** | Server supports unshare (`set_visibility` MCP, `store.go` / `SetVisibility`). Plans ban unshare UI while Share copy says “you can stop sharing later.” |

Stale gen client confirmed: `ui/src/lib/gen/engram_pb.ts` has only 5 read RPCs; no `StoreMemory`; no `buf/validate` import (canonical `gen/ts/.../engram_pb.ts:12` imports `../../buf/validate/validate_pb`; `gen/ts/buf/` is absent).

---

## 19-01 — Foundation: re-vendor gen + destructive tokens

### Summary
Correct and now source-aligned. Structure-preserving re-vendor + real `pnpm check` + plugin-scoped `include_imports` + real `destructive` Button variant + per-theme foreground are the right blockers. Ready to execute.

### Strengths
- Flat-copy compile break is real: `gen/ts/engram/v1/engram_pb.ts:12` → missing `gen/ts/buf/`.
- Barrel keeps `client.ts:3` `./gen/engram_pb` stable.
- Button has only `default|outline|ghost|surface` (`button.svelte:8-12`); CSS token alone would not fix Plan 03.
- Dark `--cat-gotcha` is light orange `#ffa657` (`app.css:28`); `var(--background)` foreground fix is correct.
- CI extension of existing `buf` job (`ci.yaml:127-130`) matches how `gen/` is drift-checked.

### Concerns
- **LOW** — `include_imports: true` may emit extra TS files; plan accepts them if `gen/go/` stays clean — confirm first `task proto:gen` commit includes **`gen/ts/buf/`** (not only `ui/src/lib/gen/`).
- **LOW** — Taskfile copy must not wipe the hand-authored barrel; the advertised `rm -rf engram buf` before `cp -R` is the safe pattern — keep acceptance criteria wording that path in SUMMARY.

### Suggestions
- After first `proto:gen`, assert both: `git diff -- gen/go/` empty **and** `test -f gen/ts/buf/validate/validate_pb.ts`.
- Add a one-line note in Taskfile that `ui/src/lib/gen/engram_pb.ts` is hand-authored and not overwritten.

### Risk Assessment
**LOW** — prerequisites are fully specified and grounded.

---

## 19-02 — Write transport: CSRF + retry-once

### Summary
Round-2 reframe is accurate and should stick. Interceptor order, separate `engramWrite`, and pre-handler-only retry codes all match the backend. Residual risk is operational rarity of recoverable races, not plan error.

### Strengths
- Reseal-skip on failure verified (`connectreseal.go:39-41`).
- CSRF names match wire contract (`webauth/csrf.go:22-23` / `connectcsrf.go:23-24`).
- Rejections are pre-handler: subject → CSRF → validate → handle → reseal (`connectapi.go:361-367`); CSRF → `PermissionDenied` (`connectcsrf.go:70-85`).
- Retry-only on Unauth/PermissionDenied avoids double-create for `Store*` (business errors are NotFound/InvalidArgument via `connecterror.go`).
- `[retryOnce, attachCsrf]` order matches Connect outer→inner composition (Pitfall 3).

### Concerns
- **MEDIUM** — Roadmap SC3 still says “retried once through a re-seal.” Plan correctly implements an **auth-race** retry. Product wording and implementation diverge; rare session races may nearly always terminal → D-09. Acceptable if SC3 is re-read as “one silent transport retry.”
- **LOW** — Node suite is pure `environment: 'node'` (`vite.config.ts:29`); no `document`. CSRF tests must `Object.defineProperty(globalThis, 'document', …)` — `localStorage` is stubbed in `vitest-setup.ts`, `document` is not.
- **LOW** — CSRF cookie parse via `split('=')[1]` is fine for raw-url base64 tokens (`csrf.go:62`), but worth decoding if ever cookie-encoded.

### Suggestions
- Comment in `retryOnce.ts` with file:line cites to `connectreseal.go:40` and `csrf.go:39` so future edits don’t restore “rotation recovery” language.
- In `csrf.test.ts`, install a minimal `document` global in that file’s `beforeEach` (don’t assume setup).

### Risk Assessment
**LOW–MEDIUM** — mechanism correct; value of retry is intentionally small.

---

## 19-03 — Destructive/action affordances

### Summary
Solid presentational plan. Button-in-button restructure is mandatory (root is a `<button>` today). Share visibility gate + mechanical rule fence are right. Residual: D-07 one-way UI vs corrected “can stop sharing later” copy.

### Strengths
- MemoryRow root is `<button>` at `MemoryRow.svelte:19-24` — nested kebab would be invalid DOM; sibling restructure required.
- DropdownMenu already has `variant: "default" | "destructive"` and `text-destructive` classes (`dropdown-menu-item.svelte:13,23`) — needs Plan 01 CSS tokens.
- Rule fence matches contract (`set_visibility` rejects rules — `tools.go:978+`, `1082`).
- Accurate share copy matches server truth (unshare exists) more than prior “can’t narrow” absolute.

### Concerns
- **MEDIUM** — Plans reject “Make private” citing locked D-07, yet ShareWarningInline copy says **“you can stop sharing later.”** In-console, they cannot. Either restore a make-private affordance (server already supports it) or soften copy to “you’ll need another API/MCP path to unshare” / drop the stop-sharing claim.
- **LOW** — `onshare(memory)` vs id-only is right for visibility; hosts must not strip to id before burning the share gate.
- **LOW** — 360px detail header + 4 buttons (`MemoryDetail.svelte:32,42-46`); compact/kebab contingency is adequate.

### Suggestions
- Prefer aligning copy with UI capability: if personal D-07 lock stays one-way-UI, drop “stop sharing later.”
- Browser-test assert kebab + selection siblings: `row.querySelector('button')` count / no nested `button button`.

### Risk Assessment
**LOW** for build correctness; **MEDIUM** for UX honesty unless copy/product is reconciled.

---

## 19-04 — Mutation hooks

### Summary
Best-grounded backend/frontend contract plan in the set. Create/schedule shared composites, Visibility member names, multi-key optimistic cache, dirty-mask, and pure factory test approach all hold. Implement with strict attention to enum names and cache membership.

### Strengths
- Store/Schedule lack visibility: `StoreMemoryRequest` ends at `summary` (`engram_pb.ts:474-524`); schedule same + window only (`:829-889`).
- `Visibility.SHARED`/`PRIVATE` required for tsc (`:927-941`); Go-style `VISIBILITY_*` is wrong in TS.
- Create response is `{id, shortId}` only (`:536-546`) → invalidate-only create correct.
- Dirty update mask matters: content/tags re-embed path at `tools.go:991-1024`; auto-summary provenance rules at `:995-1004`.
- `update_mask` is FieldMask object (`engram.proto:168`, `UpdateMemoryRequest` + `protoconv.go:48-67`).
- Discovery required shape matches `validateStoreDiscovery` (`tools.go:575-606`) and proto CEL.
- Partial share-failure keep-private toast avoids data-loss rollback.

### Concerns
- **MEDIUM** — Filtered-list eviction on set-visibility is easy to botch: `listMemoriesKey(..., visibility, ...)` (`queries.ts:34`). Must remove from `visibility:'private'` pages when flipping to shared (and inverse), not only patch the field.
- **LOW** — Discoveries: `SearchDiscoveries` uses `memoriesToProto` with **full content** (`connectapi.go:54-62, 215-245`), unlike list’s `shapeProtoMemories`. Optimistic patches must still key `['searchDiscoveries']` and `['getMemory', id]` correctly.
- **LOW** — Hook `onError` must rethrow/not swallow so form-level D-09 still fires (called out — keep it explicit in tests).

### Suggestions
- Gate CI-style: `rg 'Visibility\.VISIBILITY_' ui/src/lib/mutations` empty.
- Unit-test: private-filtered cache drops the row after share; shared-filtered drops after (if ever) private.
- Fake `engramWrite`: assert `setVisibility` payload numeric `2` (`SHARED`), never `0`.

### Risk Assessment
**MEDIUM** — many cache edge cases; heatmaps cleared well enough to implement safely.

---

## 19-05 — Form sheets + re-auth envelope

### Summary
Material upgrade of round 1–2. Resume envelope laced to real OIDC `/ui/` landing is necessary for D-09. Schedule create-only, edit immutability, discovery Citations shape, scheduled+shared intent, dirty-mask are source-true. Largest remaining complexity is the envelope lifecycle across 05+06.

### Strengths
- OIDC always returns to `/ui/` (`handlers.go:187`); keeping sheet open alone cannot survive re-auth.
- Schedule create-only: no `id` on `ScheduleMemoryRequest`.
- Edit only content/shared/tags/summary (`UpdateMemoryRequest` + CEL allowlist `engram.proto:164-168`).
- Discovery form requirements match server validators (content, kind, ≥1 citation kind+ref, `discovery:` scope).
- `visibility === ''` prefill → private matches wire memory field / `queries.ts:4`.
- Explicit reversal of CONTEXT “deferred sessionStorage” is justified by grounded handlers.

### Concerns
- **MEDIUM** — Envelope contract spans two plans with several handoffs (`write → login → /ui/ → goto(returnPath) → reopenFromResume → restore values → clear`). Any mismatch of `returnPath` base (`/observe?...` + `base='/ui'` from `svelte.config.js:9`) or premature envelope clear fails silently.
- **MEDIUM** — Same D-07 / “stop sharing later” copy tension as 03, inside the form warning.
- **LOW** — sessionStorage on Cancel/success must clear only `engram:resume` for this form’s envelope, not unrelated tab state (single fixed key → whole-blob clear is fine).
- **LOW** — Draft holds only typed input (good); ensure `recordId` for edit doesn’t re-open against a deleted record without a friendly error (06 openEdit toast covers).

### Suggestions
- Freeze envelope schema as a small typed helper (`lib/resume.ts`) shared by 05 and 06 — single key, zod-light shape, base-path helpers using `$app/paths` `base`.
- Browser test only component-level restore in 05; do **not** claim full OIDC chain green until 06 route test.

### Risk Assessment
**MEDIUM** — correctness depends on 06 landing consumption; design itself is sound.

---

## 19-06 — Route integration + ship SPA

### Summary
Closes the phase. Critical data-loss path (summary-shaped edit) and resume landing are correctly specified. Host API, discovery fences, and `task ui:build` ship gate are non-optional. Main residual is timing of `bind:this` + resume, and D-07 bidirectional visibility vs SC1.

### Strengths
- **HIGH real risk fixed:** prefilling edit from list/search would save empty content (`connectapi.go:70-79` vs GetMemory `:202`). `openEdit(id)` + `fetchQuery(['getMemory', id])` is the right mitigation.
- `/ui/` landing has no write host (`+page.svelte`) — envelope consume + `goto` is required.
- Exact `bind:this` API removes prior “or equivalent” fog.
- `requestShare(memory)` can no-op on already-shared (must_haves previously unsatisfiable with id-only).
- Discovery route omits `onedit` (D-04); `StoreDiscoveryRequest.id` exists for replace-in-place but is correctly unused for console edit.
- Task 3 + CI `ui-drift` (`ci.yaml:138-155`) prevent shipping a stale embedded SPA.
- Foreign shared:NotFound/PermissionDenied + rollback (no whoami) matches store as sole authz chokepoint.

### Concerns
- **MEDIUM** — Resume: root must `goto` without clearing; destination clears after `reopenFromResume`. Use `$effect` after `bind:this` — if reopen runs before host mount, envelope is lost. Plan mentions order; implement with a single debounce/`tick()`.
- **MEDIUM** — SC1 “change visibility” / REQ “re-share (visibility)” can be read as bidirectional; implemented as private→shared only. Document as intentional D-07 narrowing so UAT doesn’t fail against a different reading.
- **LOW** — Home `/ui/` Recent list (`+page.svelte:40-48`) stays read-only (no write callbacks). Acceptable; optional later New-on-home.
- **LOW** — Plan text still once describes `shareTarget` as id-only while API is memory-aware — internal state can be id after gate; keep API as specified.

### Suggestions
- Route test is the one that matters for D-09: seed envelope, mount observe, assert GetMemory full content + restored values (already in plan — guard it as phase must-have).
- After `task ui:build`, smoke that embedded assets include a string exclusive to write UX (e.g. `New memory`) so ship isn’t an empty rebuild.
- If stakeholders want unshare: one follow-up plan flips D-07 and adds Make private → planning only; don’t half-add in 19.

### Risk Assessment
**MEDIUM** — data-loss and ship gates are well covered; residual is orchestration timing + product scope of visibility.

---

## Phase-level assessment

### Does this achieve Phase 19 goals?

| SC | Plan coverage | Residual |
|----|---------------|----------|
| SC1 create/edit/delete/visibility/schedule | Plans 03–06 full recreate surface | Visibility is effectively private→shared only |
| SC2 CSRF auto-attach | Plan 02 | — |
| SC3 silent once-retry then re-auth w/ input kept | 02 race-retry + 05/06 envelope | Retry rarely helps; envelope is the real SC3 salvage path for hard expiry |

Core requirement **REQ-console-write-ux** is achievable if D-07 is accepted as one-way share UI. Nest: share copy must not promise stop-sharing in-console.

### Dependency / wave ordering
Wave 1→2(02∥03)→3(04)→4(05)→5(06) is correct. Mutations depend only on transport; forms on mutations+affordances; host last. No cycle.

### Security
- CSRF client-echo only; server authoritative (`connectcsrf.go`).
- Retry limited to pre-handler codes — double-create mitigated.
- Resume store is operator-typed data only (good against token leakage).
- No new packages (“accept” SC installs).

### What’s newly broken or still wrong after round 2?
1. **MEDIUM product tension:** Share copy promises unshare; UI refuses unshare (03/05/06 rejection of bidirectional). Not a source bug — a consistency bug introduced by fixing copy without reopening D-07.
2. **No source-false HIGH regressions** from round-2 technical fixes: Visibility names, schedule composite, auth-race wording, resume landing, GetMemory prefill, visibility-aware share all **land correctly in plan text** and **match source**.

### Overall phase risk
**MEDIUM** — ready to execute with two explicit product locks before build:

1. D-07 one-way share UI is intentional; fix warning copy so it does not say “stop sharing later” if unshare is not offered.  
2. Treat transport retry as best-effort auth-race helper; D-09 integrity is the resume envelope path.

No further replan required for compile/runtime correctness of the six plans; only the share copy / unshare decision needs a conscious YES/NO before Front-of-frontend work ships to operators.

---

## Antigravity Review

# Cross-AI Plan Review Report: Phase 19 — Console Write UX (Round 3)

**Status:** ✅ **VERIFIED & APPROVED FOR EXECUTION**  
**Review Round:** 3 (Final Verification)  
**Target Phase:** Phase 19 — Console Write UX  
**Plans Reviewed:** `19-01-PLAN.md` through `19-06-PLAN.md`  

---

## 1. Executive Summary

All six plans in Phase 19 (`19-01-PLAN.md` to `19-06-PLAN.md`) have been subjected to direct verification against the git working tree. The claims made in the `<review_incorporation>` blocks for Rounds 1 and 2 were audited against the actual source code (`engram.proto`, `connectapi.go`, `connectcsrf.go`, `connectreseal.go`, `csrf.go`, `app.css`, `button.svelte`, `handlers.go`, and `gen/ts/engram/v1/engram_pb.ts`).

Every Round 2 fix has been verified as holding in source. No new regressions or unaddressed gaps were introduced. The wave dependencies, component contracts, wire representations, and test suites across all plans are fully consistent and complete.

---

## 2. Verification of Round 2 Fixes Against Working Tree Source

| Claim / Fix Area | Claimed Behavior | Verified Source Evidence | Verification Status |
| :--- | :--- | :--- | :--- |
| **Proto Wire Asymmetry & Shared Composites** (*Plans 04, 05*) | `StoreMemoryRequest`, `StoreDiscoveryRequest`, and `ScheduleMemoryRequest` carry no `visibility` field. Create-shared must execute a 2-call composite (Store/Schedule $\rightarrow$ `SetVisibility(SHARED)`). | `proto/engram/v1/engram.proto` lines 104–115, 130–144, 203–221 confirm no `visibility` fields exist on store/schedule requests. The two-call composite with partial failure handling (`created (private) — sharing failed`) is accurately specified. | ✅ **VERIFIED** |
| **TS Enum Member Resolution** (*Plan 04*) | Connect-ES TS enum drops `VISIBILITY_` prefix (`Visibility.SHARED`/`Visibility.PRIVATE`). | `gen/ts/engram/v1/engram_pb.ts` line 927 exports `export enum Visibility { UNSPECIFIED = 0, PRIVATE = 1, SHARED = 2 }`. Using `Visibility.VISIBILITY_SHARED` would fail compilation; `Visibility.SHARED` is exact. | ✅ **VERIFIED** |
| **Summary-Shaped List Row Edit Defect** (*Plan 06*) | List/search rows clear content (`full=false`). Prefilling an edit sheet from a row object would overwrite stored content with `""` on save. `openEdit(id)` must fetch full record via `GetMemory`. | `internal/server/connectapi.go` line 70 (`shapeProtoMemories`) explicitly sets `m.Content = ""` when `full=false`. Line 202 (`GetMemory`) returns full content. `openEdit(id)` fetching `GetMemory` before sheet mounting prevents data corruption. | ✅ **VERIFIED** |
| **OIDC Redirect Input Preservation** (*Plans 05, 06*) | `/auth/callback` redirects to `/ui/` (root SPA), not origin sub-route. Pure component state is lost on redirect. Draft requires a `sessionStorage` `engram:resume` envelope consumed on `/ui/` landing. | `internal/webauth/handlers.go` line 187 confirms callback redirects to `/ui/`. The `engram:resume` envelope persisted prior to `/auth/login` navigation and consumed on `/ui/` startup accurately resolves input recovery across full re-auth. | ✅ **VERIFIED** |
| **Invalid Nested Button DOM** (*Plan 03*) | `MemoryRow` root is `<button>`, nesting a kebab trigger button inside it produces invalid nested interactive HTML. Root must be non-button container. | `ui/src/lib/components/MemoryRow.svelte` line 19 confirms the root element is `<button>`. Restructuring root to a `<div class="relative ...">` container with sibling dropdown trigger fixes DOM validation. | ✅ **VERIFIED** |
| **Buf/Validate Import Dependency in Gen Client** (*Plan 01*) | `gen/ts/engram/v1/engram_pb.ts` imports `../../buf/validate/validate_pb`. Flat file copy leaves `validate_pb` unresolved. | `gen/ts/engram/v1/engram_pb.ts` line 12 imports `../../buf/validate/validate_pb`. Plan 01 scopes `include_imports: true` on the ES plugin in `buf.gen.yaml` and executes structure-preserving `cp -R gen/ts/. ui/src/lib/gen/`. | ✅ **VERIFIED** |
| **Embedded SPA Build Gate** (*Plan 06*) | Production console is embedded via `go:embed`. Built SPA must be rebuilt and committed under `internal/webauth/static/`. | `.github/workflows/ci.yaml` lines 138–160 (`ui-drift` job) asserts `git diff --exit-code internal/webauth/static/`. Plan 06 Task 3 includes `task ui:build` and commits static assets. | ✅ **VERIFIED** |

---

## 3. Plan-by-Plan Quality & Completeness Assessment

### Wave 1: Foundation
* **`19-01-PLAN.md` — Client Re-vendoring & Design Tokens**
  * **Scope:** Scopes `include_imports: true` to the ES plugin in `buf.gen.yaml`, updates `Taskfile.yaml` for structure-preserving vendoring, adds barrel re-export `ui/src/lib/gen/engram_pb.ts`, configures `--destructive` / `--destructive-foreground: var(--background)` CSS variables per theme, adds `destructive` variant to `button.svelte`, and gates on `pnpm check`.
  * **Completeness:** 100%. Addresses compiler, build, CI drift, and component styling dependencies.

### Wave 2: Transport & Presentational Affordances
* **`19-02-PLAN.md` — Transport Interceptors & Write Client**
  * **Scope:** Implements `attachCsrf` (X-CSRF-Token double-submit) and `retryOnce` (single opportunistic auth-race retry for `Code.Unauthenticated` / `Code.PermissionDenied`), composed in `[retryOnce, attachCsrf]` order on the `engramWrite` client.
  * **Completeness:** 100%. Accurately reframed to session-cookie freshness race per `connectreseal.go` logic.
* **`19-03-PLAN.md` — Action Affordances & Confirm Dialogs**
  * **Scope:** Implements `DeleteConfirmDialog.svelte` and `ShareWarningInline.svelte` (with accurate disclosure copy), restructures `MemoryRow.svelte` shell to a non-button container, attaches visibility-aware `onshare(memory)`, and enforces mechanical suppression of write affordances when `category === 'rule'`.
  * **Completeness:** 100%. Fully respects D-05 (rule immutability) and D-07 (one-way private-to-shared visibility).

### Wave 3: Mutation Hooks
* **`19-04-PLAN.md` — Svelte-Query Mutation Hooks**
  * **Scope:** Implements `useCreateMemory`, `useUpdateMemory`, `useDeleteMemory`, `useSetMemoryVisibility`, `useScheduleMemory`, `useCreateDiscovery`, `useDeleteDiscovery`, `useSetDiscoveryVisibility`.
  * **Completeness:** 100%. Handles dirty-field mask construction (`update_mask`), invalidation-only creates, multi-key optimistic cache updates with rollback, two-call shared composites, and exports pure cache factories for node testing.

### Wave 4: Form Surfaces
* **`19-05-PLAN.md` — Slide-Over Form Sheets**
  * **Scope:** Implements `MemoryFormSheet.svelte` (create/edit, create-only schedule toggle, edit immutability for scope/category, dirty-field diffing) and `DiscoveryFormSheet.svelte` (create-only, required content, typed `Citation[]` editor, `discovery:` scope validation).
  * **Completeness:** 100%. Incorporates D-09 input preservation via inline re-auth prompt and the `engram:resume` envelope stored in `sessionStorage`.

### Wave 5: Route Integration & Distribution
* **`19-06-PLAN.md` — Route Integration & SPA Embedding**
  * **Scope:** Implements `WriteSurfaces.svelte` host component, connects `observe`, `search`, and `discovery` routes, handles full-content prefill via `GetMemory`, consumes the `engram:resume` envelope on `/ui/` startup, and executes `task ui:build` to commit `internal/webauth/static/`.
  * **Completeness:** 100%. Complete end-to-end integration and release packaging.

---

## 4. Risk & Invariant Audit

1. **Rule Immutability (DEC-iedk / D-05):** Guaranteed. `MemoryRow` and `MemoryDetail` suppress edit/delete/share controls when `category === 'rule'`. `SetVisibility` is rejected server-side for rules.
2. **CSRF & Session Hardening:** Guaranteed. Interceptor order `[retryOnce, attachCsrf]` ensures retried requests re-read `engram_csrf` cookie. Read client `engram` remains unchanged.
3. **Data Loss Prevention on Edit (Summary vs Full):** Guaranteed. `openEdit(id)` in `WriteSurfaces` fetches `GetMemory` before rendering `MemoryFormSheet`, preventing content wiping caused by summary-shaped list rows (`full=false`).
4. **Data Integrity on Update (Dirty Masking):** Guaranteed. Form diffing passes only altered paths into `UpdateMemoryRequest.update_mask`, avoiding accidental summary provenance overwrites or unnecessary vector re-embeddings.
5. **Distribution Hygiene:** Guaranteed. SPA asset generation via `task ui:build` is explicitly committed to satisfy CI `ui-drift` checks.

---

## 5. Recommendation

The Phase 19 planning package is **exceptionally thorough, fully verified against the codebase, and architecturally complete**. 

**Proceed with execution according to the wave ordering (Wave 1 through Wave 5).**

---

## Consensus Summary

**Round-3 verdict: converging, not fully converged.** All three reviewers confirm every round-2 fix HOLDS in source. The residual set has shifted in *character*: rounds 1–2 were fundamental defects (plans specced against schemas/DOM that don't exist); round 3's blockers are **second-order** — the newly-added round-2 mechanisms (the create→SetVisibility composite and the sessionStorage resume envelope) have incomplete edge-case/lifecycle specs. Codex (highest-signal) rates this HIGH with 7 required revisions; grok-4.5 rates it LOW/MEDIUM and essentially ready; antigravity approved. Base the verdict on the grounded pair.

**Grounded trend across rounds:** Codex HIGH(7 blockers) → HIGH(6) → HIGH(3, all second-order). grok MEDIUM → MEDIUM(2 convergent HIGH) → LOW/MEDIUM(0 HIGH). The direction is clearly toward convergence; the disagreement is whether the remaining second-order gaps are plan-blockers (Codex) or executor-level detail (grok).

### Verified holding in source (all 3 reviewers — round-2 fixes confirmed landed)
- **Visibility enum names** `Visibility.PRIVATE`/`Visibility.SHARED` exact (`gen/ts/engram/v1/engram_pb.ts:927`).
- **Scheduled+shared composite** correctly planned (ScheduleMemoryRequest has no visibility field, `engram.proto:203`).
- **Retry semantics source-correct** — auth-race retry, not reseal-on-fail (`connectreseal.go:39`, `resolver.go:49`, CSRF owner-bound/stable `csrf.go`/`reseal.go`).
- **Full-record edit fetch** — `GetMemory` before prefill avoids `full=false` content clearing (`connectapi.go:70`, `:202`).
- **Visibility-aware Share** — row/detail pass `Memory`, host no-ops when already shared.
- **Make-private rejection consistent with locked D-07** (`19-CONTEXT.md:62`; server technically supports `SetVisibility(private)` at `store.go:1571` but the product decision is one-way).

### Agreed Concerns (2+ grounded reviewers)
- **CONVERGENT MEDIUM — authoritative SC3/D-08 requirement text is stale.** The plan (19-02) correctly implements a single opportunistic auth-race retry, but `ROADMAP.md:408`, `REQUIREMENTS.md:53`, and locked `D-08` (`19-CONTEXT.md:69`) still promise "retried once through a re-seal." Codex + grok both flag it. Cheap fix: amend the authoritative success criterion / requirement / D-08 wording to "single opportunistic auth-race retry," or explicitly mark the original reseal promise infeasible under the shipped backend. (Also update CONTEXT.md's `D-09` deferred-sessionStorage note, which the plans now reverse.)

### Codex-unique HIGH (grok/agy did not raise — new second-order defects from the round-2 mechanisms)
- **Composite partial-success can DUPLICATE creates (19-04).** Store/Schedule success followed by a `SetVisibility` failure that returns `Unauthenticated` can route into the form's whole-create hard-auth resubmit path (D-09), re-issuing Store/Schedule — neither request has an idempotency key or id (`engram.proto:104,203`). Fix: model the composite as a state machine — primary success + visibility failure → return `{status:"created_private", id}` and a **visibility-only** continuation (carrying the returned id), never the original create command; test that the second RPC failing with both auth codes issues no second Store/Schedule.
- **Resume-envelope lifecycle is split and racy (19-05/19-06).** (a) Mount-only restoration is incompatible with the host lifecycle — `WriteSurfaces` mounts the form in create mode, rejects the edit envelope, then asynchronously fetches `editMemory`, and mount-restoration never reruns (`19-05:89`, `19-06:90`). (b) Envelope deletion has two owners — Plan 05 says the form deletes it, Plan 06 says the route clears it after `reopenFromResume()`, which can wipe storage before the form reads it (`19-06:135`). Fix: give the route/host sole ownership — parse once, pass `resumeValues` + `dirtyPaths` as props, delete only after the form acknowledges applying them; key/conditionally mount the form by `mode + recordId`.

### Codex MEDIUM (grounded, single-reviewer)
- **Per-item callback gating (19-03).** "Render the kebab when the callbacks are provided" must mean each item gates on its OWN callback (`{#if onedit}` / `{#if ondelete}` / `{#if onshare && visibility!=='shared'}`), else the discovery route (which passes delete/share but no edit, `19-06:134`) can show Edit and invoke `undefined`. Add row/detail tests with only delete/share callbacks asserting Edit is absent.
- **Memory scope validation missing (19-05).** StoreMemory/ScheduleMemory require non-empty scope (`engram.proto:106,210`); search can supply an empty default scope (`search/+page.svelte:13`). Require a non-empty trimmed scope before create/schedule.
- **Threat-model claim inaccurate (19-05).** Edit values are prefilled from full `GetMemory`, so the sessionStorage envelope can contain existing server-side memory content, not only newly-typed input (`connectapi.go:211`). Correct the "no server data in envelope" claim; version + TTL + allowed-route validation on the envelope.
- **Return-path normalization + landing test (19-06).** SvelteKit base is `/ui` (`svelte.config.js:9`); `goto(base + returnPath)` double-prefixes to `/ui/ui/observe` unless `returnPath` is guaranteed base-relative. The claimed landing test seeds storage and mounts the observe route directly (`19-06:138`) — it does not mount the `/ui/` root, so it never exercises root-page parsing / `goto` / base normalization. Add a real root-page test; add search/discovery recovery coverage.

### Divergent Views
- **Overall risk: Codex HIGH / 7 required revisions vs grok LOW–MEDIUM / ready vs antigravity APPROVED.** The disagreement is not about facts (all agree the round-2 fixes hold) but about severity of the second-order gaps. Codex treats the composite-duplicate and resume-lifecycle races as plan-blockers with concrete failure scenarios; grok treats them as acceptable executor-level detail and rates the phase ready; antigravity did not surface them at all.
- **Antigravity (agy 1.1.2):** clean VERIFIED & APPROVED again. Source-grounded on the landed fixes (a genuine improvement sustained from round 2), but missed every second-order defect Codex found → optimistic; use as a soundness cross-check on the landed fixes, not a clearing verdict.

### Reviewer signal assessment
- **Codex: highest signal, 3 grounded second-order HIGHs + 5 MEDIUMs**, all with file:line + concrete failure scenarios. Consistent pattern across all three rounds.
- **OpenCode/grok-4.5: grounded, convergence-leaning.** Verified all round-2 fixes hold; flagged the SC3-wording MEDIUM (convergent with Codex) and only LOW test-harness nits; rated the phase ready.
- **Antigravity: optimistic soundness cross-check.** Confirmed landed fixes; no new findings.

### Recommended next step
Two defensible paths:
1. **One more `/gsd-plan-phase 19 --reviews` pass** to tighten the second-order specs — composite partial-success state machine (no duplicate create), single-owner resume-envelope lifecycle (props not mount-restore), per-item callback gating, memory-scope validation, and the cheap convergent doc fix (ROADMAP/REQUIREMENTS/D-08/D-09 wording). This is the Codex-aligned path.
2. **Execute now** (grok-aligned) — the fundamentals are verified sound and the residuals are localized to 19-04/05/06 lifecycle detail the executor can resolve as disclosed deviations. Higher risk on the create-duplicate edge case and re-auth edit recovery.
