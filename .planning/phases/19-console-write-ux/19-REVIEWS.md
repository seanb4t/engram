---
phase: 19
reviewers: [codex, opencode, antigravity]
reviewed_at: 2026-07-14T22:00:44Z
review_round: 4
reviewed_commit: fc686723
opencode_model: openrouter/x-ai/grok-4.5
codex_cli: 0.144.1
agy_cli: 1.1.2
plans_reviewed: [19-01-PLAN.md, 19-02-PLAN.md, 19-03-PLAN.md, 19-04-PLAN.md, 19-05-PLAN.md, 19-06-PLAN.md]
---

# Cross-AI Plan Review — Phase 19 (Round 4 — Convergence Check)

Round 4 review of the round-3 plans (commit `1ae5d1f2`) + reconciled upstream docs (`fc686723`) against live source. All three reviewers confirm every round-3 second-order fix HOLDS in source. grok-4.5 and antigravity rate the phase EXECUTE-READY (LOW residuals only). Codex rates it MEDIUM — the round-3 architecture converged, but it found one new HIGH (SC3 re-auth is not wired for the inline delete/share actions, only for the forms) plus two MEDIUMs and an incomplete-doc-reconciliation LOW. The Codex blocker trend across rounds: HIGH 7 → 6 → 3 → MEDIUM(1 HIGH).

---

## Codex Review

# Summary

The round-3 architecture largely converged: the composite partial-success state machine, callback-level action gating, non-empty scope validation, full-record edit fetch, and base-relative return-path design are all source-grounded and correctly specified.

The plans are close, but not fully execute-ready. Three remaining gaps should be resolved first:

1. Terminal auth failures on delete/share do not produce the SC3 re-auth prompt.
2. Edit-mode visibility can accidentally expose an unshare path despite the locked one-way decision.
3. The resume lifecycle’s acknowledgement callback remains underspecified, and `persistResume`’s proposed type signature is internally inconsistent.

Overall risk: **MEDIUM**.

## Round-3 convergence check

| Fix | Verdict | Evidence |
|---|---|---|
| Composite partial-success state machine | Holds | Store and schedule requests have no ID/idempotency field, while responses return the created ID ([engram.proto:104](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:104), [engram.proto:203](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:203)). Plan 04 catches secondary `SetVisibility` failures into `created_private` ([19-04-PLAN.md:41](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-04-PLAN.md:41)), and Plan 05 treats that as success rather than replaying create ([19-05-PLAN.md:95](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-05-PLAN.md:95)). |
| Single-owner resume lifecycle | Mostly holds | Forms persist and restore through props without deleting storage ([19-05-PLAN.md:99](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-05-PLAN.md:99)); root peeks without consuming, and the destination consumes after acknowledgement ([19-06-PLAN.md:143](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-06-PLAN.md:143)). The exact acknowledgement prop remains vague. |
| Per-item callback gating | Holds | Plan 03 explicitly requires independent `onedit`, `ondelete`, and `onshare` guards ([19-03-PLAN.md:25](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-03-PLAN.md:25)). This correctly handles the discovery route, which omits `onedit` ([19-06-PLAN.md:142](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-06-PLAN.md:142)). |
| Non-empty memory scope | Holds | Proto validation requires `scope.min_len=1` for store and schedule ([engram.proto:106](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:106), [engram.proto:210](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:210)); Plan 05 blocks blank/whitespace scope ([19-05-PLAN.md:93](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-05-PLAN.md:93)). |
| Base-relative return path | Holds in design | SvelteKit’s base is `/ui` ([svelte.config.js:9](/Volumes/Code/github.com/seanb4t/engram/ui/svelte.config.js:9)); Plan 05 normalizes `/ui/observe` to `/observe` ([19-05-PLAN.md:88](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-05-PLAN.md:88)), and Plan 06 tests the actual root landing/goto path ([19-06-PLAN.md:146](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-06-PLAN.md:146)). |
| SC3/D-08 wording reconciliation | Partial | ROADMAP and REQUIREMENTS now correctly say “opportunistic auth-race retry” ([ROADMAP.md:408](/Volumes/Code/github.com/seanb4t/engram/.planning/ROADMAP.md:408), [REQUIREMENTS.md:53](/Volumes/Code/github.com/seanb4t/engram/.planning/REQUIREMENTS.md:53)), and D-08 itself is corrected ([19-CONTEXT.md:69](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-CONTEXT.md:69)). However, CONTEXT still says “mid-write session re-seal,” “resolves rotation failures,” and “get the re-seal semantics right” elsewhere ([19-CONTEXT.md:13](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-CONTEXT.md:13), [19-CONTEXT.md:127](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-CONTEXT.md:127), [19-CONTEXT.md:165](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-CONTEXT.md:165)). |

The backend confirms the corrected interpretation: errored requests skip resealing ([connectreseal.go:39](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectreseal.go:39)), expired sessions fail before the handler ([resolver.go:49](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/resolver.go:49)), and the CSRF token remains stable across reseals because it is owner-bound ([csrf.go:38](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/csrf.go:38), [reseal.go:66](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/reseal.go:66)).

# Plan reviews

## 19-01 — Foundation

### Summary

The generated-client strategy is source-grounded and appropriately fixes the actual unresolved `buf/validate` import. Risk is **LOW**.

### Strengths

- The canonical generated file really imports `../../buf/validate/validate_pb`, while the current tree contains only `gen/ts/engram/v1/engram_pb.ts`; therefore a flat copy would fail ([engram_pb.ts:12](/Volumes/Code/github.com/seanb4t/engram/gen/ts/engram/v1/engram_pb.ts:12)).
- Existing `proto:gen` currently only runs Buf ([Taskfile.yaml:145](/Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml:145)), and CI only checks `gen/` drift ([ci.yaml:127](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/ci.yaml:127)). The planned structure-preserving copy and UI drift assertion close a real gap.
- `pnpm check` provides the correct compile-level guard.
- The destructive-token and button-variant additions match the current missing source: no destructive token exists in [app.css:5](/Volumes/Code/github.com/seanb4t/engram/ui/src/app.css:5), and the button currently exposes only four variants ([button.svelte:8](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/components/ui/button/button.svelte:8)).

### Concerns

- **LOW:** The plan’s `files_modified` manifest omits canonical files generated under `gen/ts/buf/**`. `include_imports` may emit more than the one named validation file, while the action explicitly permits extra imported TS output.

### Suggestions

- Add `gen/ts/buf/**` or the exact generated dependency set to the plan manifest.
- Retain the full-tree diff and compile checks; they are stronger than enumerating only one imported file.

## 19-02 — Write transport

### Summary

The retry semantics now match the shipped backend. Risk is **LOW**, with one testing and one documentation cleanup remaining.

### Strengths

- The two-code retry set matches pre-handler auth and CSRF rejection paths: subject resolution produces `Unauthenticated` ([connectauth.go:21](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectauth.go:21)); CSRF rejection produces `PermissionDenied` before `next()` ([connectcsrf.go:65](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectcsrf.go:65)).
- `[retryOnce, attachCsrf]` correctly makes retry outermost so token attachment reruns.
- Non-auth failures are not retried.
- The plan correctly avoids describing retry as rotation recovery in the implementation tasks ([19-02-PLAN.md:38](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-02-PLAN.md:38)).

### Concerns

- **LOW:** Tests cover each interceptor and structurally assert order, but do not explicitly exercise a first failure, cookie change, and second request with the refreshed header.
- **LOW:** The success-criteria/review text still says ROADMAP/REQUIREMENTS/D-08 are stale ([19-02-PLAN.md:186](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-02-PLAN.md:186)), although ROADMAP, REQUIREMENTS, and D-08 have now been corrected.

### Suggestions

- Add one composed-interceptor test where the first handler invocation changes the cookie then returns `PermissionDenied`; assert the second invocation sees the new header.
- Update the stale reconciliation notes and the three residual CONTEXT passages.

## 19-03 — Action affordances

### Summary

The per-item callback fix holds and is sufficiently testable. Risk is **LOW**.

### Strengths

- The current row root really is a button ([MemoryRow.svelte:19](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/components/MemoryRow.svelte:19)); the planned sibling selection/dropdown controls resolve the nested-interactive problem.
- Independent callback guards are explicit for both row and detail.
- The discovery route’s missing `onedit` is now safe rather than relying on an undefined callback.
- Rule suppression is mechanical at both component layers.
- Share receives the complete `Memory`, allowing visibility-aware suppression.

### Concerns

- **LOW:** The objective still describes sharing as impossible to narrow back, while the task copy correctly says sharing can later stop. The backend demonstrably supports unsharing by writing private visibility ([store.go:1571](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1571)).

### Suggestions

- Make all prose use the accurate distinction: sharing can stop, but prior disclosure cannot be retracted.

## 19-04 — Mutation hooks

### Summary

The composite state machine is the strongest part of the revision and directly fixes the duplicate-create risk. Risk is **LOW**.

### Strengths

- Primary failure, complete success, and partial success are explicitly separated.
- Secondary failures, including both auth codes, become `created_private`; they never escape into whole-create replay.
- Tests require exactly one Store/Schedule/StoreDiscovery call on partial failure.
- Generated enum names are correct: `Visibility.PRIVATE` and `Visibility.SHARED` ([engram_pb.ts:927](/Volumes/Code/github.com/seanb4t/engram/gen/ts/engram/v1/engram_pb.ts:927)).
- Multi-key rollback matches the actual parameterized list key ([queries.ts:34](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/queries.ts:34)).
- Dirty masks are justified: content/tags trigger re-embedding ([tools.go:1003](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:1003)), and summary updates change provenance ([store.go:1404](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1404)).

### Concerns

- **LOW:** Toast handling is ambiguous. Partial failure is told to emit `created (private) — sharing failed`, while generic `onSuccess` is also told to emit the normal success toast ([19-04-PLAN.md:98](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-04-PLAN.md:98)). A naïve implementation could show both.
- **LOW:** “Exports exactly” the three discovery hooks conflicts with separately testable pure factories unless those helpers are exported through a test-only object or moved elsewhere.

### Suggestions

- Define `onSuccess(result)` explicitly: normal statuses toast `created`; `created_private` emits only the partial-success warning.
- Specify how pure factories are exposed for tests without contradicting the public API claim.

## 19-05 — Forms and resume envelope

### Summary

Scope validation, partial-success consumption, and prop-driven restoration hold, but two concrete contract issues remain. Risk is **MEDIUM**.

### Strengths

- The form blocks blank memory scope before store/schedule.
- Edit mode cannot schedule and cannot edit immutable scope/category fields.
- `created_private` closes as success and never enters resume/replay.
- The real OIDC flow is correctly understood: login leaves the SPA, and callback returns to `/ui/` ([handlers.go:60](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/handlers.go:60), [handlers.go:187](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/handlers.go:187)).
- Forms persist only; restored values arrive through props and the form acknowledges application.

### Concerns

- **MEDIUM:** The proposed type contract is inconsistent. `ResumeEnvelope` requires `v` and `ts`, but `persistResume` is described as adding them, and every call site omits them ([19-05-PLAN.md:88](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-05-PLAN.md:88), [19-05-PLAN.md:99](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-05-PLAN.md:99)). If `persistResume` accepts `ResumeEnvelope`, this fails TypeScript compilation.
- **MEDIUM:** Edit visibility is underspecified. The form renders a private/shared control and allows `shared` in the dirty mask ([19-05-PLAN.md:89](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-05-PLAN.md:89), [19-05-PLAN.md:96](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-05-PLAN.md:96)), while Plans 03/06 insist the UI must not offer shared→private. A shared record opened in Edit can therefore accidentally gain an unshare path.
- **LOW:** `peekResume` checks JSON/version/TTL but does not explicitly validate the remaining runtime shape before returning it as `ResumeEnvelope`.

### Suggestions

- Define:
  ```ts
  type ResumeDraft = Omit<ResumeEnvelope, 'v' | 'ts'>;
  function persistResume(draft: ResumeDraft): void;
  ```
- Decide edit visibility explicitly:
  - If D-07 remains one-way, shared records must render visibility read-only and must never produce `shared:false`.
  - If “change visibility” is bidirectional, revise D-07 and add an acknowledged Make private flow.
- Runtime-validate `kind`, `mode`, `recordId`, `values`, and `dirtyPaths` in `peekResume`.

## 19-06 — Route integration and shipping

### Summary

The root landing, full-record edit fetch, route-specific callback wiring, and embedded-SPA ship gate are strong. Terminal auth handling and the acknowledgement API still block execute-readiness. Risk is **MEDIUM**.

### Strengths

- `openEdit(id)` refetches full content, which is necessary because list/search responses clear content when `full=false` ([connectapi.go:70](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:70)); `GetMemory` returns the full record ([connectapi.go:202](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:202)).
- Root `/ui/` peeks and redirects without deleting; destination routes restore and consume after acknowledgement.
- Search and discovery recovery tests supplement observe rather than assuming one route proves all.
- Discovery receives no edit callback.
- The SPA rebuild and drift gate match the current build pipeline ([Taskfile.yaml:21](/Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml:21), [ci.yaml:138](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/ci.yaml:138)).

### Concerns

- **HIGH:** SC3 is incomplete for delete and row/detail share. ROADMAP says any write whose retry fails prompts re-authentication ([ROADMAP.md:408](/Volumes/Code/github.com/seanb4t/engram/.planning/ROADMAP.md:408)). Forms implement that, but `WriteSurfaces` clears delete/share targets after invoking the mutation and specifies no terminal-auth prompt ([19-06-PLAN.md:98](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-06-PLAN.md:98)). The operator gets only `write failed`, with no re-auth CTA or retained target.
- **MEDIUM:** The supposedly exact host contract becomes vague for the critical resume acknowledgement: it says “e.g. via an `onresumeapplied` prop” ([19-06-PLAN.md:103](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-06-PLAN.md:103)). Yet that callback is the only event authorizing envelope deletion.
- **LOW:** Plan 06’s reconciliation note still claims upstream docs are stale ([19-06-PLAN.md:254](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-06-PLAN.md:254)), which is no longer true for ROADMAP/REQUIREMENTS/D-08.

### Suggestions

- For delete/share, await mutation settlement:
  - Clear the target only on success.
  - On terminal `Unauthenticated`/`PermissionDenied`, retain the target and show a re-auth CTA.
  - No resume values are necessary, but the pending action should remain visible for deliberate retry after login.
- Make the host prop exact:
  ```ts
  onresumeapplied?: () => void
  ```
  Test that it fires once, after form application, and that the route then calls `consumeResume()`.
- Update stale reconciliation prose.

# Risk assessment

**Overall: MEDIUM.**

The central data-integrity risks from round 3 are addressed: secondary visibility failures cannot replay non-idempotent creates, edit restoration refetches full data, and action gating is callback-specific. The remaining issues are narrower but still affect explicit phase guarantees—especially SC3 for delete/share and the unresolved one-way visibility behavior. After those targeted plan edits, the phase should be execute-ready without another architectural redesign.

---

## OpenCode Review (grok-4.5 — openrouter/x-ai/grok-4.5)

Verifying round-3 claims against live source before judging converge-ready.
Checking critical wire contracts and UI seams the plans rely on.
Checking one remaining product/authz coupling and search default scope.
# Phase 19 Plan Review — Round 4 (Convergence Check)

## Summary

**Execute-ready.** Round-3 second-order fixes are present in plan text and match live server contracts I re-checked in this tree. The five gate items hold: (a) create/schedule→SetVisibility is a **discriminated composite** where secondary auth failures become `created_private` success (no create replay); (b) resume envelope is **route-owned** (form only `persistResume` + prop restore); (c) row/detail actions **gate per callback**; (d) non-empty scope fail-fast; (e) base-relative return paths; (f) SC3/D-08 wording matches **auth-race retry + resume envelope** in plans **and** reconciled `ROADMAP`/`REQUIREMENTS`/`CONTEXT`. Residual notes are LOW documentation/meta noise only.

---

## Round-3 Fix Verification (against source)

### (a) Composite partial-success state machine — HOLDS

| Claim | Evidence |
|--------|-----------|
| Store*/Schedule* have **no** `visibility` | `StoreMemoryRequest` ends at `summary` (`gen/ts/.../engram_pb.ts:474-523`); same for `ScheduleMemoryRequest` (`:829-888`); `proto/engram/v1/engram.proto:104-115,203-210` |
| Response is `{id, short_id}` only — no optimistic list body | `StoreMemoryResponse` (`engram_pb.ts:536-545`) |
| No idempotency key/id on create/schedule | proto Store/Schedule create messages (`engram.proto:104,203`) |
| Secondary SetVisibility auth failure must **not** rethrow into D-09 whole-create resubmit | Plan 04 state machine + T-19-45; Plan 05 treats `created_private` as SUCCESS |
| TS enum members are `Visibility.SHARED`/`PRIVATE` | `engram_pb.ts:927-941` (`UNSPECIFIED=0, PRIVATE=1, SHARED=2` — not `VISIBILITY_*`) |

Business failures map to `NotFound`/`InvalidArgument`/`FailedPrecondition` — **not** `PermissionDenied` (`connecterror.go:24-55`), so post-mutation `PermissionDenied` from handlers is not a normal path. `Unauthenticated`/`PermissionDenied` stay pre-handler (`connectauth.go:23`, `connectcsrf.go:70-85`). Retry-on-those-codes remains safe for double-create (T-19-11).

### (b) Single-owner resume lifecycle — HOLDS

| Claim | Evidence |
|--------|-----------|
| OIDC callback lands on `/ui/`, not origin route | `handlers.go:135-136,187` → `http.Redirect(..., "/ui/")` |
| `/ui/` root is **not** a write host | `ui/src/routes/+page.svelte` = scopes + recent list; no WriteSurfaces |
| SvelteKit `base` is `/ui` | `svelte.config.js:9` → double-prefix risk is real without `normalizeReturnPath` |
| Plans 05/06 sole-owner contract | form: `persistResume` only + `resumeValues` props; route: peek → `goto(base + normalize…)` without delete → dest `reopenFromResume` → `consumeResume` after `onresumeapplied` |

### (c) Per-item callback gating — HOLDS

Plan 03 Tasks 2–3 and tests require `{#if onedit}` / `{#if ondelete}` / `{#if onshare && visibility !== 'shared'}` on **both** MemoryRow and MemoryDetail; discovery wiring (Plan 06) passes delete/share only. Matches D-04 and the `undefined` Edit hazard.

### (d) Scope validation — HOLDS

`scope.min_len = 1` on Store/Schedule (`engram.proto:106,210`). Search defaults `scope` to `''` (`search/+page.svelte:13`). Plan 05 client fail-fast is required, not polish.

### (e) Return-path normalization — HOLDS

Plan 05 `normalizeReturnPath` + `isAllowedDestination`; Plan 06 real root landing test asserts `goto('/ui/observe?…')` not `/ui/ui/…`. Aligned with `base: '/ui'` and root `goto(\`${base}/observe?…\`)` pattern (`+page.svelte:17`).

### (f) SC3 / D-08 wording — RECONCILED (source + docs)

| Doc | Status |
|-----|--------|
| `ROADMAP.md` Phase 19 SC3 | auth-race retry; server does not re-seal failed requests; `sessionStorage` resume |
| `REQUIREMENTS.md` REQ-console-write-ux | same |
| `19-CONTEXT.md` D-08/D-09 | auth-race correction; resume envelope **in scope** (no longer deferred) |

Backend matches the framing:

- Reseal **skips** errors: `connectreseal.go:39-41`
- Expired session pre-handler: `resolver.go:50` `"session expired"`
- CSRF token Owner-only / stable across re-seal: `csrf.go:38-40`; reseal refreshes Max-Age, not HMAC value

Plan 02 still contains a **stale meta** note in `<review_incorporation>`/`success_criteria` that upstream text was “user-owned / not reconciled.” That meta is outdated; the live docs **are** reconciled. Operational acceptance criteria already use auth-race language — not a blocking defect.

---

## Per-Plan Notes

### 19-01 — Foundation (re-vendor + destructive)
- Flat vendor is broken today: `gen/ts/engram/v1/engram_pb.ts:12` imports `../../buf/validate/validate_pb`; `gen/ts/buf/` does **not** exist; console `ui/src/lib/gen/engram_pb.ts` is a stale read-only embed with no write RPCs.
- Button has no `destructive` (`button.svelte:8-12`: default|outline|ghost|surface only). Token + variant task is still required.
- Structure-preserving tree + barrel + `pnpm check` gate is correct.

### 19-02 — Transport
- `[retryOnce, attachCsrf]` order and dual-code retry set are correct.
- Cookie names match wire contract (`csrf.go:22-23`).
- Auth-race (not “rotation recovery”) framing verified against reseal/CSRF.

### 19-03 — Affordances
- MemoryRow **is** a root `<button>` today (`MemoryRow.svelte:19-24`) — restructure is mandatory.
- Accurate share copy + mechanical rule fence + visibility-aware Share match source and D-05/D-07.

### 19-04 — Mutations
- Two-call shared composite is mandatory (proto).
- Discriminated state machine is the right fix for duplicate-create under D-09.
- Invalidate-only create matches `{id, short_id}` responses.
- Multi-key rollback vs `listMemoriesKey(..., visibility, ...)` (`queries.ts:34`) is correctly specified.
- `Visibility.SHARED`/`PRIVATE` naming is correct (would break tsc if `VISIBILITY_*`).

### 19-05 — Sheets + resume module
- D-09 without envelope cannot survive `/ui/` landing (`handlers.go:187`).
- Consume composite statuses; primary-failure-only re-auth path is correctly wired to 04.
- Edit dirty-mask / create-only schedule / discovery typed citations match proto.

### 19-06 — Route wiring + ship
- `openEdit(id)` → GetMemory full content is mandatory: `shapeProtoMemories` clears `Content` when `!full` (`connectapi.go:70-79`); GetMemory returns full (`:211`).
- Real root landing test is the right place to pin base-normalization.
- `task ui:build` + `internal/webauth/static/` is still the ship gate for go:embed.

---

## Strengths

1. **Contract-first** — RPC shapes, Visibility enum TS names, CSRF constants, reseal skip-on-error, summary-cleared lists all grounded in this tree.
2. **Duplicate-create closed** — composite + form both treat secondary SetVisibility failure as terminal **success** with by-id re-share.
3. **D-09 is honest** — single-owner resume across the real OIDC `/ui/` landing, not keep-sheet-open fantasy.
4. **Wave shape** — 01 blocks compile; 02∥03; 04→05→06; ship rebuild last.
5. **Authz not reinvented** — foreign shared writes stay server-rejected (`NotFound` path via `connectError`); no whoami fiction.
6. **Test surface** — bridge claims (composite both auth codes → count===1 Store; root goto not delete; discovery no Edit) are acceptance-coded.

---

## Concerns

### HIGH
**None** that block execute.

### MEDIUM
**None** material after doc reconciliation. Product lock already decided:

| Topic | Note |
|--------|------|
| D-07 one-way vs accurate warning copy | Warning says unshare is possible; UI still omits Make private. Server **can** set private (`store.go:1571+`). Intentional under locked D-07; not a plan defect. |

### LOW

1. **Plan 02 meta lag** — review_incorporation still says SC3 docs are user-owned/stale; they are updated. Optional SUMMARY note when executing 02.
2. **CONTEXT residual phrasing** — `19-CONTEXT.md` canonical_refs still say retry “resolves rotation failures” (~line 127) while D-08 body is correct. Cosmetic.
3. **T-19-11 follow-up** — recommended server contract test (auth-class codes stay pre-handler) remains a good Phase 20/21 hygiene item, not Phase 19 scope.
4. **ShareWarningInline vs D-07 operator expectation** — accurate copy may lead operators to ask for Make private; UX ticket only if product reopens D-07.
5. **Setup steps unrun** — target tree still has stale gen, no `buf/validate` TS, no `destructive` variant, MemoryRow still a button. Plans know this; execute Wave 1 first.

---

## Suggestions

1. When executing 19-02, one-line SUMMARY: “SC3 docs reconciled pre-execute; abort on any remaining ‘re-seal recovery’ wording in code comments/tests.”
2. Prefer exact acceptance strings already in 04/05 (`created_private`, auth-code call counters, root `goto` spy) — do not invent alternate status enums mid-implementation.
3. Optional follow-up issue for pre-handler retry-code contract test (out of UI plans’ file scope — already noted in 02 threat model).

---

## Risk Assessment

**Overall: LOW**

| Area | Risk | Why |
|------|------|-----|
| Compile/gen vendor | LOW if 01 runs as written | Failure mode known; gate is `pnpm check` |
| Duplicate creates | LOW | State machine + form consumption + call-count tests |
| Resume race / `/ui/ui/` | LOW | Single owner + normalize + root test |
| CSRF/retry semantics | LOW | Matches backend; framing accurate |
| Ship binary drift | LOW | 06 Task 3 + CI ui-drift |
| D-07 one-way product | Accepted | Locked decision, not execution risk |

**Verdict: plans are execute-ready for Phase 19.** Round-3 second-order fixes hold in source and plan text. No further planning revision required before `/gsd-execute-phase`.

---

## Antigravity Review

# Phase 19 (Console Write UX) Plan Review — Round 4 (Convergence Check)

### Executive Assessment: **EXECUTE-READY**

All six plans (**19-01-PLAN.md** through **19-06-PLAN.md**) have successfully converged. The round-3 second-order fixes have been thoroughly verified against the git working tree source code (`proto/engram/v1/engram.proto`, `internal/server/`, `ui/src/app.css`, `ui/src/lib/components/ui/button/button.svelte`, `ui/src/lib/client.ts`, `ui/svelte.config.js`), and no unaddressed landmines remain.

---

### Verification of Round-3 Second-Order Fixes

#### 1. Composite Partial-Success State Machine (`19-04-PLAN.md` & `19-05-PLAN.md`)
* **Source Grounding:** In `proto/engram/v1/engram.proto`, `StoreMemoryRequest` (lines 104–115), `StoreDiscoveryRequest` (lines 130–144), and `ScheduleMemoryRequest` (lines 203–221) carry **no** `visibility` or `shared` fields. Responses carry only `{ id, short_id }` with **no** idempotency key or caller-supplied client ID.
* **Verification:** Creating a record as shared requires a two-step sequence (`Store*`/`Schedule` followed by `SetVisibility({ id, visibility: Visibility.SHARED })`). If `SetVisibility` fails (e.g., due to an auth race or transport error), the mutation hook catches the secondary error, swallows it, and returns a discriminated `{ status: 'created_private', id }` result while displaying a `created (private) — sharing failed` toast. `MemoryFormSheet` and `DiscoveryFormSheet` consume `created_private` as a **success** state (closing the sheet and clearing the envelope). Crucially, the form's whole-create D-09 resubmit path never fires for a secondary `SetVisibility` auth failure, completely preventing duplicate record creation.

#### 2. Single-Owner Resume-Envelope Lifecycle (`19-05-PLAN.md` & `19-06-PLAN.md`)
* **Source Grounding:** Tested against `webauth/handlers.go:60,187` where the OIDC callback lands at root `/ui/` (destroying in-memory SPA state), and `svelte.config.js:9` where `base = '/ui'`.
* **Verification:** The single typed module `ui/src/lib/resume.ts` (`persistResume`, `peekResume`, `consumeResume`, `normalizeReturnPath`, `isAllowedDestination`) acts as the sole envelope authority:
  * The form components (`MemoryFormSheet`, `DiscoveryFormSheet`) **only persist** envelopes before `/auth/login` navigation. They never read or delete from `sessionStorage`.
  * The root `/ui/` landing page (`routes/+page.svelte`) peeks the envelope and executes `goto(base + normalizeReturnPath(env.returnPath))` **without deleting** it.
  * The target host route (`observe`, `search`, `discovery`) calls `reopenFromResume(env)` on `WriteSurfaces`, which fetches the full record for edit targets via `GetMemory` and passes `resumeValues`/`resumeDirtyPaths` as **props** to the form sheet.
  * The host route calls `consumeResume()` **only after** the form fires its `onresumeapplied` callback. This eliminates the two-owner deletion race and handles async `GetMemory` fetches cleanly.

#### 3. Per-Item Callback Gating (`19-03-PLAN.md` & `19-06-PLAN.md`)
* **Source Grounding:** Verified against `MemoryRow.svelte`, `MemoryDetail.svelte`, and `discovery/+page.svelte`.
* **Verification:** Row and detail components gate each action item on its own individual callback prop (`{#if onedit}`, `{#if ondelete}`, `{#if onshare && visibility !== 'shared'}`). On the `discovery` route, `ondelete` and `onshare` are passed, but `onedit` is omitted (enforcing the D-04 non-editable discovery constraint). `MemoryRow` on the discovery route cleanly renders Delete and Share without rendering Edit or invoking an `undefined` callback handler.

#### 4. Scope Validation (`19-04-PLAN.md` & `19-05-PLAN.md`)
* **Source Grounding:** Verified against `proto/engram/v1/engram.proto:106,210` (`scope.min_len = 1`) and `ui/src/routes/search/+page.svelte:13` (where search defaults scope to `''`).
* **Verification:** Client-side fail-fast validation in `MemoryFormSheet` ensures that scope is non-empty after trimming before submitting a create or schedule operation. This prevents silent server-side `InvalidArgument` rejections when launching a create form from the search panel. `DiscoveryFormSheet` additionally validates that scope starts with the `discovery:` prefix.

#### 5. Return-Path Normalization (`19-05-PLAN.md` & `19-06-PLAN.md`)
* **Source Grounding:** Verified against `ui/svelte.config.js:9` (`base = '/ui'`).
* **Verification:** `normalizeReturnPath(returnPath)` in `resume.ts` strips any leading `/ui` base prefix so that `goto(base + normalizeReturnPath(returnPath))` produces clean paths (e.g. `/ui/observe?sel=...`) and avoids double-prefixing (e.g. `/ui/ui/observe`). `isAllowedDestination` enforces that destination routes match allowed internal patterns (`/observe`, `/search`, `/discovery`), preventing open-redirect abuse.

---

### Wave & Plan Alignment

| Wave | Plan | Key Responsibilities | Prerequisite Dependencies |
|---|---|---|---|
| **Wave 1** | `19-01-PLAN.md` | Re-vendor Connect TS client (`buf/validate` dep, structure-preserving `cp -R`, barrel export, `pnpm check` gate) + `--destructive` CSS & Button variants | None (Wave 1 Blocker) |
| **Wave 2** | `19-02-PLAN.md` | `attachCsrf` & `retryOnce` interceptors (opportunistic auth-race retry) + `engramWrite` client | 19-01 |
| **Wave 2** | `19-03-PLAN.md` | `DeleteConfirmDialog`, `ShareWarningInline`, non-button `MemoryRow` shell with sibling kebab, `MemoryDetail` actions | 19-01 |
| **Wave 3** | `19-04-PLAN.md` | Memory & Discovery mutation hooks (`useCreateMemory`, `useScheduleMemory`, etc.), composite partial-success state machine, dirty-mask update | 19-02 |
| **Wave 4** | `19-05-PLAN.md` | `MemoryFormSheet`, `DiscoveryFormSheet`, single-owner `resume.ts` module, scope & schedule validation | 19-03, 19-04 |
| **Wave 5** | `19-06-PLAN.md` | `WriteSurfaces` host, full record `GetMemory` prefill on edit, `/ui/` landing resume consumption, `task ui:build` & committed SPA build | 19-03, 19-04, 19-05 |

---

### Conclusion & Recommendation

The plan suite is comprehensive, self-consistent, accurately aligned with the Go/Connect backend implementation, and fully ready for execution. You may proceed directly with executing **Wave 1 (`19-01-PLAN.md`)**.

---

## Consensus Summary

**Round-4 verdict: near-converged; 2 of 3 reviewers say execute-ready, Codex says MEDIUM.** All three confirm every round-3 second-order fix HOLDS in source. grok-4.5 and antigravity find only LOW documentation/meta noise and rate the phase execute-ready. Codex agrees the round-3 architecture converged but surfaced one new HIGH and two MEDIUMs by tracing paths the prior rounds hadn't focused on (the inline delete/share actions and the edit-mode visibility control). Codex blocker trend: HIGH 7 → 6 → 3 → MEDIUM(1 HIGH) — narrowing steadily.

### Verified HOLDING in source (all 3 reviewers — round-3 fixes confirmed)
- **Composite partial-success state machine** — secondary `SetVisibility` auth failure → `created_private` success, never replays create; test asserts exactly one Store/Schedule (`19-04`, `engram.proto:104,203`).
- **Single-owner resume lifecycle** — form persists + restores via props (no self-delete); route peeks→consumes after ack (`19-05`, `19-06`, `handlers.go:187`, `svelte.config.js:9`).
- **Per-item callback gating** — independent `{#if onedit/ondelete/onshare}` guards; discovery row has no Edit (`19-03`, `19-06`).
- **Non-empty scope validation** (`scope.min_len=1`, `engram.proto:106,210`), **base-relative return-path** (no `/ui/ui/`), **full-record `GetMemory` edit fetch** (`connectapi.go:70,202`), **enum names** `Visibility.PRIVATE/SHARED`.
- **SC3/D-08 reconciliation** — ROADMAP, REQUIREMENTS, and D-08 now say "opportunistic auth-race retry"; backend confirms (errored requests skip reseal `connectreseal.go:39`, expired sessions fail pre-handler `resolver.go:49`, CSRF owner-bound stable `csrf.go:38`).

### Codex-unique HIGH — the one genuine remaining coverage gap
- **SC3 is not wired for inline delete/share (19-06).** ROADMAP SC3 promises any write whose retry fails prompts re-authentication without losing input. The create/edit FORMS implement this (sheet-open + resume envelope), but `WriteSurfaces` clears the delete/share target right after invoking the mutation and specifies no terminal-auth handling (`19-06-PLAN.md:98`) — on a terminal `Unauthenticated`/`PermissionDenied` the operator gets only a generic `write failed`, with no re-auth CTA and no retained target. Fix: for delete/share, await settlement, clear the target only on success; on terminal auth failure retain the target and show a re-auth CTA (no resume values needed — the pending action just stays visible for deliberate retry).

### Codex MEDIUM — cheap correctness/consistency fixes
- **Edit-mode can expose an unshare path vs locked D-07 (19-05).** The edit form renders a private/shared control and allows `shared` in the dirty mask (`19-05:89,96`), while Plans 03/06 (and D-07) forbid a shared→private UI. A shared record opened in Edit could produce `shared:false`. Fix: if D-07 stays one-way, render visibility read-only for already-shared records and never emit `shared:false` (or, if bidirectional is wanted, that's a D-07 revision + an acknowledged Make-private flow — a scope decision, not a silent plan change).
- **Resume acknowledgement + `persistResume` type are underspecified (19-05/06).** The host prop that authorizes envelope deletion is "e.g. via an `onresumeapplied` prop" (`19-06:103`) — but it's the *only* deletion trigger, so it must be exact (`onresumeapplied?: () => void`, fires once after apply, route then calls `consumeResume()`). And `ResumeEnvelope` requires `v`/`ts` while every `persistResume` call site omits them (`19-05:88,99`) → TS compile failure; fix with `type ResumeDraft = Omit<ResumeEnvelope,'v'|'ts'>; persistResume(draft: ResumeDraft)`.

### Codex LOW (incl. an incomplete-reconciliation cleanup)
- **The doc reconciliation was incomplete.** `fc686723` fixed D-08/D-09 + the deferred idea, but CONTEXT.md still has stale reseal/rotation language at `19-CONTEXT.md:13` ("tolerates a mid-write session re-seal"), `:127` ("resolves rotation failures"), and `:165`. Also, the plans' own `<review_incorporation>` notes still say "upstream docs are stale" (`19-02-PLAN.md:186`, `19-06-PLAN.md:254`) — now outdated for ROADMAP/REQUIREMENTS/D-08.
- Other LOWs: `files_modified` should list `gen/ts/buf/**`; add one composed-interceptor test (first-fail → cookie change → retry sees new header); disambiguate the partial-success toast so `created_private` emits only the warning (not also the success toast); reconcile "exports exactly three hooks" with the separately-testable pure factories; 19-03 objective prose still implies sharing can't be narrowed (task copy correctly says it can stop, though prior disclosure can't be retracted).

### Divergent Views
- **grok-4.5 + antigravity: EXECUTE-READY** (LOW noise only). **Codex: MEDIUM** (1 HIGH + 2 MEDIUM). The disagreement is real coverage, not model pessimism: Codex traced the inline delete/share path and the edit-visibility control, which the other two did not drill into. The HIGH (delete/share SC3) is a genuine stated-criterion gap; the MEDIUMs are cheap.
- **Antigravity:** clean EXECUTE-READY again, source-grounded on the landed fixes — consistent optimistic pattern; use as a soundness cross-check, not a clearing verdict.

### Convergence assessment
Very close. The fundamentals and all prior-round fixes are verified sound; the remaining set is (1) a real SC3 coverage gap for two inline actions, (2) two cheap correctness fixes localized to 19-05/06, and (3) doc/note cleanup (some of it from an incomplete round-3 reconciliation). None require architectural redesign.

### Recommended next step
1. **One more targeted `/gsd-plan-phase 19 --reviews` pass** (recommended) — close the delete/share SC3 re-auth CTA, make edit-mode visibility read-only for shared records (honoring D-07), pin the `onresumeapplied` contract + `ResumeDraft` type, and finish the doc reconciliation (CONTEXT.md:13/127/165 + the two stale plan notes). All narrow and cheap.
2. **Execute now** — 2 of 3 reviewers say ready; the remaining items are localized to 19-05/06 and the delete/share re-auth CTA can be handled as a disclosed executor deviation. Accepts a partial SC3 gap on the inline destructive actions.
