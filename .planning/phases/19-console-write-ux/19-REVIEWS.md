---
phase: 19
reviewers: [codex, opencode, antigravity]
reviewed_at: 2026-07-14T00:42:11Z
plans_reviewed: [19-01-PLAN.md, 19-02-PLAN.md, 19-03-PLAN.md, 19-04-PLAN.md, 19-05-PLAN.md, 19-06-PLAN.md]
---

# Cross-AI Plan Review — Phase 19

Reviewers invoked with repo (working-tree) access and the source-grounding mandate (cite `file:line`, verify claims against live code). Models: **codex** = codex-cli 0.144.1 (default model); **opencode** = openrouter/x-ai/grok-4.5; **antigravity** = agy 1.1.1.

---

## Codex Review

# Cross-AI Plan Review

## Overall assessment

The plans are well decomposed and source-aware, but several implementation assumptions do not match the live contracts. The largest blockers are: the generated-client copy will not compile as designed; the retry cannot itself cause a session re-seal; full-page reauthentication loses form state; several forms do not map cleanly to the RPC schema; and the final vendored SPA is not included. These should be corrected before execution.

Overall risk: **HIGH**.

---

## Plan 19-01 — Generated client and design tokens

### Summary

The plan correctly identifies both prerequisites, but copying only `engram_pb.ts` into the existing flattened location will introduce a broken generated import.

### Strengths

- The console client is demonstrably stale: its service ends after the five read RPCs at [ui/src/lib/gen/engram_pb.ts:430](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/gen/engram_pb.ts:430), while the canonical client includes all writes starting at [gen/ts/engram/v1/engram_pb.ts:995](/Volumes/Code/github.com/seanb4t/engram/gen/ts/engram/v1/engram_pb.ts:995).
- No current task or CI step synchronizes it. `proto:gen` only runs Buf at [Taskfile.yaml:145](/Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml:145), and CI only checks `gen/` at [.github/workflows/ci.yaml:127](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/ci.yaml:127).
- The destructive-token gap is real: [ui/src/app.css:5](/Volumes/Code/github.com/seanb4t/engram/ui/src/app.css:5)–47 define no destructive bridge, while the dropdown item consumes `text-destructive` and `bg-destructive` at [dropdown-menu-item.svelte:23](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/components/ui/dropdown-menu/dropdown-menu-item.svelte:23).

### Concerns

- **HIGH — The proposed whole-file copy will not compile.** The canonical generated file imports `../../buf/validate/validate_pb` at [gen/ts/engram/v1/engram_pb.ts:12](/Volumes/Code/github.com/seanb4t/engram/gen/ts/engram/v1/engram_pb.ts:12), but neither `gen/ts/` nor `ui/src/` contains that generated dependency. Flattening the file into `ui/src/lib/gen/engram_pb.ts` also changes what `../../buf` resolves to.
- **LOW — An acceptance criterion names `StoreMemorySchema`, which does not exist.** The generated symbol is `StoreMemoryRequestSchema` at [gen/ts/engram/v1/engram_pb.ts:530](/Volumes/Code/github.com/seanb4t/engram/gen/ts/engram/v1/engram_pb.ts:530).

### Suggestions

- Generate imported protobuf dependencies as well, preserve the generated directory structure under `ui/src/lib/gen/`, and update `client.ts` to import `gen/engram/v1/engram_pb`.
- Make CI invoke the same vendoring helper as `proto:gen`, avoiding duplicated copy logic.
- Add a compile check immediately after re-vendoring, not only a byte comparison.

### Risk assessment

**HIGH** — the first wave can leave every downstream plan uncompilable.

---

## Plan 19-02 — CSRF and retry transport

### Summary

The interceptor composition is correct and double-creation is not presently a concern, but the stated “retry through re-seal” mechanism does not exist server-side.

### Strengths

- `[retryOnce, attachCsrf]` is the correct order. Connect-ES reverses the array while wrapping at [interceptor.js:23](/Volumes/Code/github.com/seanb4t/engram/ui/node_modules/.pnpm/@connectrpc+connect@2.1.2_@bufbuild+protobuf@2.12.1/node_modules/@connectrpc/connect/dist/esm/interceptor.js:23), making the first element outermost. A retry therefore re-enters `attachCsrf`.
- The wire names match [internal/webauth/csrf.go:18](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/csrf.go:18).
- Retrying these codes does not currently risk double-creating. Authentication and CSRF reject before `next()` at [connectauth.go:21](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectauth.go:21) and [connectcsrf.go:61](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectcsrf.go:61). Business-handler errors do not map to `Unauthenticated` or `PermissionDenied` in [connecterror.go:49](/Volumes/Code/github.com/seanb4t/engram/internal/server/connecterror.go:49).

### Concerns

- **HIGH — A failed request cannot trigger re-sealing.** The reseal interceptor explicitly skips any error at [connectreseal.go:39](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectreseal.go:39). Retrying the same expired session therefore fails again unless some concurrent successful request happens to replace the cookie.
- **MEDIUM — Session rotation is not an error state.** A valid near-expiry session is accepted and re-sealed after success; a hard-expired session is rejected at [resolver.go:49](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/resolver.go:49). There is no ordinary “needs rotation” failure for the retry to repair.
- **LOW — The safety claim is an undocumented coupling.** Future handler code returning either retry code after mutation could make create retries unsafe.

### Suggestions

- Recast this as an opportunistic single retry for cookie/CSRF races, not “retry through re-seal.”
- Add a server-contract test proving both retry codes remain pre-handler for all six write procedures.
- If deterministic rotation recovery is required, the protocol needs an explicit recoverable signal or refresh/reseal operation; frontend-only retry cannot supply it.

### Risk assessment

**HIGH** — the code is straightforward, but the mechanism does not achieve the success criterion as described.

---

## Plan 19-03 — Action affordances

### Summary

The component split is good, but the plan has a compile-time button-variant problem, invalid nested-interactive markup, and only implements one direction of visibility changes.

### Strengths

- Confirm-before-delete and acknowledge-before-share cleanly encode D-06/D-07.
- Callback-only presentational components keep transport concerns out of rows and details.
- The rule exclusion is recognized, and rules are identifiable as `category: "rule"` in [internal/server/rules.go:144](/Volumes/Code/github.com/seanb4t/engram/internal/server/rules.go:144).

### Concerns

- **HIGH — `Button variant="destructive"` does not exist.** The local Button exposes only `default`, `outline`, `ghost`, and `surface` at [button.svelte:8](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/components/ui/button/button.svelte:8). CSS variables alone do not add a component variant.
- **HIGH — A dropdown trigger cannot be inserted inside the current row root.** `MemoryRow` is itself a `<button>` at [MemoryRow.svelte:19](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/components/MemoryRow.svelte:19). Adding another button-like dropdown trigger inside creates invalid nested interactive content.
- **MEDIUM — Only “Share” is exposed.** The backend supports both `SHARED` and `PRIVATE` at [proto/engram/v1/engram.proto:98](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:98), and `SetVisibility(false)` actively unshares at [internal/store/store.go:1571](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1571). The plan lacks a “Make private” path.
- **MEDIUM — The warning copy is technically false.** A record can be made private again; what cannot be undone is prior disclosure to callers.
- **MEDIUM — The rule fence is not mechanically specified.** Plan 06 passes callbacks to whole lists, so Plan 03 must explicitly suppress every item when `memory.category === "rule"`.
- **LOW — Four header buttons may overflow the fixed 360px detail pane** defined at [MemoryDetail.svelte:32](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/components/MemoryDetail.svelte:32).

### Suggestions

- Add a real destructive Button variant or use explicit destructive classes.
- Refactor `MemoryRow` to a non-interactive wrapper containing sibling selection and action buttons.
- Render menu items individually based on supplied callbacks, including `Make private` for shared records.
- Change the warning to say that prior exposure cannot be revoked, while still permitting unshare.
- Add an explicit `category !== "rule"` guard and browser test.

### Risk assessment

**HIGH** — current instructions can fail compilation and produce invalid DOM.

---

## Plan 19-04 — Mutation hooks

### Summary

The operation inventory and update-mask focus are sound, but optimistic caching and testing are underspecified, and some form capabilities cannot be represented by these hooks.

### Strengths

- The planned update mask matches the server’s sole-presence contract at [proto/engram/v1/engram.proto:164](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:164).
- The operation split honors D-04: no discovery-update hook.
- The route query prefixes are correctly identified: `listMemories` at [queries.ts:34](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/queries.ts:34), `searchMemories` at [search/+page.svelte:14](/Volumes/Code/github.com/seanb4t/engram/ui/src/routes/search/+page.svelte:14), and `searchDiscoveries` at [discovery/+page.svelte:14](/Volumes/Code/github.com/seanb4t/engram/ui/src/routes/discovery/+page.svelte:14).

### Concerns

- **HIGH — Create/schedule requests have no visibility field.** `StoreMemoryRequest` at [engram.proto:104](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:104), `StoreDiscoveryRequest` at [engram.proto:130](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:130), and `ScheduleMemoryRequest` at [engram.proto:203](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:203) cannot honor the forms’ visibility choice without a second RPC.
- **MEDIUM — Multi-cache rollback needs `getQueriesData`/`setQueriesData`.** One exact `getQueryData` snapshot cannot cover every scope/filter/page key or the different `{memories}` versus `{discoveries}` response shapes.
- **MEDIUM — The proposed node hook tests are not viable as written.** `createMutation` calls `useQueryClient` at [createMutation.svelte.js:8](/Volumes/Code/github.com/seanb4t/engram/ui/node_modules/.pnpm/@tanstack+svelte-query@6.1.36_svelte@5.56.4/node_modules/@tanstack/svelte-query/dist/createMutation.svelte.js:8), which requires Svelte context and throws without a provider at [context.js:5](/Volumes/Code/github.com/seanb4t/engram/ui/node_modules/.pnpm/@tanstack+svelte-query@6.1.36_svelte@5.56.4/node_modules/@tanstack/svelte-query/dist/context.js:5).
- **LOW — “Do not swallow/rethrow” is confused.** `mutate()` intentionally catches its promise at [createMutation.svelte.js:20](/Volumes/Code/github.com/seanb4t/engram/ui/node_modules/.pnpm/@tanstack+svelte-query@6.1.36_svelte@5.56.4/node_modules/@tanstack/svelte-query/dist/createMutation.svelte.js:20); caller-level `onError` does not require throwing from the hook callback.

### Suggestions

- Define composite create-then-set-visibility behavior, including partial-failure handling, or remove visibility from create mode.
- Extract pure mutation option/cache-transform factories that can be node-tested; test the actual hooks through a Svelte provider harness.
- Snapshot all matching cache entries and restore each exact key/value pair.
- Normalize private visibility because the server returns `""`, not `"private"` ([connectapi.go:43](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:43)).

### Risk assessment

**HIGH** — the mutation surface currently cannot support all promised form behavior safely.

---

## Plan 19-05 — Form sheets

### Summary

This is the highest-risk plan. Several required fields or mappings are absent, edit and schedule semantics conflict with the RPCs, and redirecting to reauthenticate necessarily destroys the in-memory draft.

### Strengths

- Sheet, inline warning, and local `$state` are appropriate for the common non-redirect failure case.
- Discovery edit remains excluded.
- Client-side validation is correctly treated as UX rather than the security boundary.

### Concerns

- **HIGH — Reauthentication loses input.** `/auth/login` redirects to the IdP at [handlers.go:60](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/handlers.go:60), and the callback redirects to `/ui/` at [handlers.go:187](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/handlers.go:187). Navigating there destroys component `$state`; the plan contains no draft persistence or popup return channel.
- **HIGH — Discovery content is missing from the planned field set.** The server requires non-empty content at [engram.proto:130](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:130) and again at [tools.go:575](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:575).
- **HIGH — Citation URLs need a concrete proto mapping.** Every citation requires `kind ∈ file|commit|url|repo` and non-empty `ref` at [engram.proto:121](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:121). A string list must explicitly become `{kind: "url", ref: value}`.
- **HIGH — Scheduling during edit creates a new record.** `ScheduleMemoryRequest` contains no ID at [engram.proto:203](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:203). Routing an edit submission to `ScheduleMemory` duplicates rather than schedules the existing memory.
- **HIGH — Create-time visibility is not sent.** None of the store/schedule request messages carries visibility.
- **MEDIUM — Scope and category look editable but cannot be updated.** `UpdateMemoryRequest` permits only content/shared/tags/summary at [engram.proto:164](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:164).
- **MEDIUM — Schedule validation is incomplete.** At least one bound is mandatory, and `not_after` must exceed `not_before` at [engram.proto:203](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:203).
- **MEDIUM — Private edit prefill needs normalization.** Stored private records arrive with `visibility === ""`, not `"private"`.

### Suggestions

- Add sessionStorage-backed draft restoration keyed by form kind/mode/record ID, clearing it only after successful resubmission.
- Add discovery content and map citation URLs to typed Citation messages.
- Make schedule create-only unless a backend “schedule existing” RPC is introduced.
- Disable scope/category in edit mode.
- Decide and test composite create-then-share semantics.
- Validate discovery scope prefixes and schedule bounds before submission.

### Risk assessment

**HIGH** — as planned, discovery creation can be rejected, edit-schedule can duplicate data, and SC3’s input-preservation promise is not met.

---

## Plan 19-06 — Route integration and shipment

### Summary

A central host is sensible, but the route callback contract is ambiguous, rule/discovery fences can be violated, and the final embedded SPA is omitted.

### Strengths

- Current scope sources are correctly available: observe uses parsed `scope` at [observe/+page.svelte:17](/Volumes/Code/github.com/seanb4t/engram/ui/src/routes/observe/+page.svelte:17), while search/discovery read `?scope` at [search/+page.svelte:13](/Volumes/Code/github.com/seanb4t/engram/ui/src/routes/search/+page.svelte:13) and [discovery/+page.svelte:13](/Volumes/Code/github.com/seanb4t/engram/ui/src/routes/discovery/+page.svelte:13).
- Hosting sheets once per route avoids duplicated dialog state.
- Existing reads and selection remain separable from mutation orchestration.

### Concerns

- **HIGH — The built SPA is not listed as an artifact.** Production serves `internal/webauth/static/`; `task ui:build` rebuilds and copies it at [Taskfile.yaml:21](/Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml:21), and CI rejects drift at [.github/workflows/ci.yaml:155](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/ci.yaml:155). No plan modifies or commits the final static bundle.
- **MEDIUM — Discovery callback wiring conflicts with D-04.** The task says to thread `onedit` through all routes, but discovery must not expose edit. Callback props must be route- and item-specific.
- **MEDIUM — Rule exclusion is not guaranteed.** Passing callbacks to every `MemoryList` row exposes actions on `category === "rule"` unless the row/detail components enforce the guard.
- **MEDIUM — Shared foreign records will receive write affordances.** Reads include owned records or any shared record at [store.go:496](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:496), while writes require ownership at [store.go:1568](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1568). The UI has no specified current-owner comparison.
- **MEDIUM — The imperative handler contract is too vague.** Svelte parents need an explicit `bind:this` exported-method API or callback registration; “or equivalent” leaves the route integration unplannable.
- **LOW — The host test is mocked orchestration, not end-to-end coverage.** It does not prove CSRF, retry, cache invalidation, or the embedded build.

### Suggestions

- Add a final task running `task ui:build`, commit `internal/webauth/static/`, then run the full `task` gate.
- Specify an exact host API, preferably avoiding imperative component methods where possible.
- Pass no `onedit` callback on discovery routes and conditionally render each row item.
- Enforce `category !== "rule"` in both row and detail components.
- Document foreign-shared-record behavior: either suppress actions with a reliable ownership signal or explicitly handle the server’s not-found rejection.

### Risk assessment

**HIGH** — even if the source UI works, the shipped binary can remain unchanged, and scope fences are not mechanically enforced.

---

## Required replanning priorities

1. Fix generated TS dependency vendoring and preserve generated import paths.
2. Resolve SC3: either persist drafts across OIDC redirects or revise the requirement.
3. Correct the retry/reseal claim to match server behavior.
4. Reconcile forms with RPC capabilities: discovery content/citations, create visibility, immutable edit fields, and create-only scheduling.
5. Refactor row markup and add a real destructive Button variant.
6. Add bidirectional visibility actions and explicit rule guards.
7. Finish with `task ui:build` and commit the embedded SPA.

---

## OpenCode Review (grok-4.5)

I'll verify the plans against live source—gen client drift, destructive tokens, CSRF/retry safety, and scope fences—then write a structured review.
## Phase 19 Plan Review — Console Write UX

### 1. Summary

The six-wave plan structure matches the phase: fix the known blocker first (stale TS client + missing destructive tokens), then transport interceptors, presentational actions (parallel), mutation hooks, sheets, then route wiring. Core claims check out against the repo: vendored gen is stale (441 vs 1046 lines; only 5 read RPCs), no Task/CI no-re-vendor path, and `--destructive` is absent from `ui/src/app.css` while shadcn classes reference `text-destructive`. Retry-once on `Unauthenticated`/`PermissionDenied` is safe against double-create because the server maps those codes **only** in pre-handler interceptors (`connectauth.go:22–23`, `connectcsrf.go:70–86`), and business errors never become those codes (`connecterror.go:45–67`). Several **implementation gaps** will break SC1 if not fixed: create-path shared visibility cannot go through `StoreMemory` alone; discovery form omits required `content` and under-specifies citations/scopes; schedule belongs create-only; `MemoryRow` is a single `<button>` so a nested kebab is invalid HTML; `Button` has no `destructive` variant.

---

### 2. Strengths

- **Prerequisite correctness is grounded in code.**  
  - `ui/src/lib/gen/engram_pb.ts:398–440` only exposes list/search/get RPCs.  
  - `gen/ts/engram/v1/engram_pb.ts` has write RPCs + `Citation` + `Visibility`.  
  - `Taskfile.yaml:145–148` is only `go tool buf generate`; CI `buf` job only diffs `gen/` (`.github/workflows/ci.yaml:127–130`); `ui-drift` only diffs `internal/webauth/static/` (lines 138–160). Plan 01’s re-vendor + CI guard is the right fix class.
- **CSRF wire constants match.** Plan 02 names match `internal/webauth/csrf.go:22–23` and `connectcsrf.go:23–24` (`engram_csrf` / `X-CSRF-Token`).
- **Interceptor order claim is correct for Connect-ES.** `applyInterceptors` reverse-wraps the array (`ui/node_modules/@connectrpc/connect/dist/esm/interceptor.js:19–26`), so `[retryOnce, attachCsrf]` makes retry outer and re-enters CSRF attachment on retry (Pitfall 3).
- **Double-create risk is actually mitigated by server design.** Subject runs before CSRF before validate before handler (`connectapi.go:361–368`); reseal only runs after success (`connectreseal.go:39–41`). Handler codes are NotFound/InvalidArgument/FailedPrecondition/Internal/Canceled — never PermissionDenied/Unauthenticated (`connecterror.go`).
- **Wave graph is sound.** 01 blocks everything; 02∥03 after 01; 04 after 02; 05 after 03+04; 06 last — matches real compile/UI dependencies.
- **Scope fences D-04/D-05 are explicit** across 03–06 (no discovery edit export; no rule UI). Aligns with `tools.go:1082–1099` rule immutability and missing `update_discovery` RPC.
- **Test plan is concrete** (router-transport interceptors, thunk `createMutation`, browser sheet re-auth retention) and maps to existing Vitest node/browser setup.

---

### 3. Concerns

#### HIGH

- **Create + `shared` is not written by Store\* RPCs.**  
  `StoreMemoryRequest` / `ScheduleMemoryRequest` / `StoreDiscoveryRequest` have **no** `shared`/`visibility` field (`gen/ts/engram/v1/engram_pb.ts` StoreMemory fields end at `summary`). `toMemory` never sets `Visibility` (`tools.go:631–647`); store treats `""` as canonical private (`store.go:793+`).  
  Plans 04/05 collect a visibility field on create but never specify the required **second call** `SetVisibility` after create (and only after ShareWarning). Without that, SC1 “create … re-share” succeeds in UI but leaves private records.

- **Discovery form under-spec'd vs server validation (SC1 will fail).**  
  `validateStoreDiscovery` requires (`tools.go:575–602`):
  - non-empty **content**
  - `kind` ∈ `map|fact`
  - `scope` **prefix `discovery:`**
  - ≥1 citation with `kind` ∈ `file|commit|url|repo` and non-empty `ref`  
  Plan 05 Task 2 lists kind/citations-as-URLs/summary/tags/scope/visibility and **omits content**, and treats citations as bare URLs. That will bounce as InvalidArgument. UI-SPEC deferred complexity ≠ optional server fields.

- **Schedule is create-SFW RPC only, not an edit path.**  
  `ScheduleMemory` is a full flattened create (`connectapi.go:318–327`, `ScheduleMemoryRequest` has content+window, no `id`). Plan 05 “create/edit sheet” with schedule toggle and “if window set → `useScheduleMemory`” risks turning an **edit into a second create**, or showing schedule on edit at all. Restrict schedule UI + RPC to **create mode only**.

#### MEDIUM

- **`MemoryRow` is a full-row `<button>` (`MemoryRow.svelte:19–37`).** Nesting dropdown trigger/menu items gut the accessibility tree and makes `stopPropagation` fragile. Plan 03 should change the row shell to a non-button container (or move kebab outside the select target) before adding the menu.

- **No `Button` `destructive` variant.** `buttonVariants` only has `default|outline|ghost|surface` (`ui/src/lib/components/ui/button/button.svelte`). Dropdown-menu item can use `variant="destructive"` (uses `text-destructive` CSS — Plan 01 tokens help). Delete dialog “destructive variant button” will not compile/`px` unless Plan 01/03 adds a button variant or uses explicit destructive classes.

- **D-09 / SC3 incomplete vs full login redirect.** Sheet-kept `$state` survives the failed mutation (good), but Plan 05/Research Pattern 3 still do `window.location.href = '/auth/login'`, which wipes form state. CONTEXT deferred sessionStorage drafts. Plans should state explicitly that **SC3 is satisfied only until the operator clicks re-auth**, not through OIDC return — or add minimal draft persist now. Verifiers will otherwise flag SC3 as unmet.

- **Share on create still needs ShareWarning** paths that actually call `SetVisibility`. Row/detail share paths (Plan 06) are fine; create form is not specified as a sequenced mutation.

- **Optimistic create under-specified.** Create responses are `{ id, short_id }` only (`StoreMemoryResponse`). Optimistic list inserts need a temp client identity or must wait for invalidate-only (no optimistic list insert). Plan 04 should say which keys are optimistically mutated for create vs after-success invalidate only — create-optimistic-without-id is easy to get wrong.

- **Discovery scope default “current scope” is usually wrong on observe/search.** Observe scopes are memory scopes (`observe/+page.svelte:27`); discovery requires `discovery:` prefix. Plan 05/06 must separate memory vs discovery default scope (e.g. derive `discovery:repo:…` or force empty + manual entry on wrong routes).

#### LOW

- Plan 01 acceptance string says `StoreMemorySchema` once; wire name is `StoreMemoryRequestSchema` (verify command is correct — cosmetic).
- Plan 04 `useUpdateMemory` with `shared` via `update_mask` **and** separate `useSetMemoryVisibility` — two places to change visibility. Prefer SetVisibility for share affordances; keep `shared` on update optional or document single path to avoid bypassing ShareWarning.
- Plan 02 retrying `PermissionDenied` will also retry real “CSRF missing forever” with no cookie — one extra 403; harm less; document.
- No automated E2E against live CSRF cookies (manual UAT only) — acceptable if noted for verify-work, but plan 06 already does.
- Wave 2 plan 03 uses Plan 01 tokens but does not list `depends_on: ["19-01"]` wait — it does. Good.

---

### 4. Suggestions

1. **Plan 04 + 05: create shared as two steps**  
   `store* → (if shared acknowledged) setVisibility(SHARED)`. Failures mid-sequence: keep id and surface “created private; share failed” vs rollback delete (pick one and test). Mirror for discovery.

2. **Plan 05 DiscoveryFormSheet**  
   - Required full `content` field.  
   - Citation mini-editor: at least `kind` + `ref` (+ optional locator/pin/excerpt).  
   - Scope validation: must start with `discovery:`; default only from discovery route, not raw observe scope.

3. **Plan 05 schedule**  
   only when `mode === 'create'` and window set → `ScheduleMemory`; edit never offers schedule (no schedule-on-existing API).

4. **Plan 03 MemoryRow**  
   refactor to `div`/`role="option"`/clickable region + action menu as sibling so interactive controls are not nested buttons.

5. **Plan 01 or 03**  
   add `destructive: 'bg-destructive text-destructive-foreground …'` to `buttonVariants`, or mandate `class="text-destructive …"` for DeleteConfirmDialog primary.

6. **Plan 05 D-09**  
   document partial SC3 + optional tiny `sessionStorage` draft keyed by form id before `/auth/login` if verifier will hold the bar strictly; otherwise write success criterion as “values retained until explicit re-auth navigation.”

7. **Plan 04 tests**  
   assert interceptor order via composed behavior: first fail PermissionDenied after cookie flip, second succeeds (covers Pitfall 3 better than structural string match).

8. **Plan 06**  
   hide Share for already-shared records (or make it unshare with different copy — CONTEXT emphasizes private→shared irreversibility, not unshare UX). Avoid dead Share via warning then SHARED no-op.

9. **Plan 01**  
   ensure `proto:gen` re-vendor is in the same cmd path CI runs (today CI only runs `go tool buf generate` — plan must add both the copy **and** the `git diff -- ui/src/lib/gen/` to that job).

---

### 5. Risk Assessment

**Overall: MEDIUM**

| Area | Level | Why |
|------|-------|-----|
| Transport CSRF + retry | **LOW** | Matches server pre-handler codes; order + double-create story holds in source |
| Gen re-vendor + tokens | **LOW** | Clearly stated blocker; soft spot only if CI copy (not just Taskfile) is missed |
| Mutation + form RPC mapping | **HIGH** | Shared-on-create, discovery content/citations/scope, schedule-on-edit mismatches will fail SC1 in UAT |
| Row action HTML/a11y | **MEDIUM** | Nesting menus in `<button>` is a building myeloma, not cosmetic |
| Re-auth input retention | **MEDIUM** | Partial fulfillment vs SC3 wording as written |
| Over-engineering | **LOW** | Separate write client + WriteSurfaces host look look right sized; no new packages |

**Will these plans achieve Phase 19 SC1–SC3 if executed verbatim?**  
Transport / CSRF / confirm-dialog half of SC2–SC3: **yes**. Full SC1 end-to-end create–with–shared + discovery create: **not without the HIGH fixes above**. Recommend patching Plans 04/05 (and MemoryRow shell in 03) before execute; rest is ready.

---

## Antigravity Review

# Cross-AI Plan Review: Phase 19 (Console Write UX)

**Reviewed Plans:** `19-01-PLAN.md` through `19-06-PLAN.md`  
**Target Requirement:** `REQ-console-write-ux`  
**Repository State:** Verified against `main` at `/Volumes/Code/github.com/seanb4t/engram`

---

## 1. Executive Summary

Phase 19 is a meticulously planned, highly structured frontend integration phase that enables full operational write capabilities (create, edit, delete, re-share, and schedule) for memories and discoveries in the SvelteKit operator console. The plan set establishes a clean separation of concerns: unblocking prerequisites and design tokens (Plan 01), building transport-level CSRF token attachment and silent session-reseal retries (Plan 02), authoring presentational dialogs and row/detail action affordances (Plan 03), creating optimistic TanStack Query v6 mutation hooks (Plan 04), constructing slide-over form sheets that preserve in-flight user input across re-authentication events (Plan 05), and orchestrating write entry points across all application routes (Plan 06). Strict alignment with locked domain choices—such as D-04 (omitting discovery editing) and D-05 (preventing rule creation from the console)—is preserved throughout.

---

## 2. Strengths

- **Reproducible Vendored Client Generation & CI Drift Guard (Plan 19-01)**  
  *Evidence:* In [ui/src/lib/gen/engram_pb.ts](file:///Volumes/Code/github.com/seanb4t/engram/ui/src/lib/gen/engram_pb.ts#L90-L95), `EngramService` currently exposes only 5 read RPCs and lacks the 6 write schemas (`StoreMemoryRequestSchema`, `UpdateMemoryRequestSchema`, `SetVisibilityRequestSchema`, etc.) as well as the `Visibility` enum and `CitationSchema`. Plan 19-01 fixes this by creating a reproducible `task proto:gen` whole-file copy step from [gen/ts/engram/v1/engram_pb.ts](file:///Volumes/Code/github.com/seanb4t/engram/gen/ts/engram/v1/engram_pb.ts#L679-L720) and locking it with a `git diff --exit-code -- ui/src/lib/gen/` assertion in [.github/workflows/ci.yaml:L127](file:///Volumes/Code/github.com/seanb4t/engram/.github/workflows/ci.yaml#L127).

- **Design Token Integrity for Destructive Controls (Plan 19-01)**  
  *Evidence:* Inspecting [ui/src/app.css:L5-47](file:///Volumes/Code/github.com/seanb4t/engram/ui/src/app.css#L5-L47) confirms that `--destructive` and `--color-destructive` are absent. Without Plan 19-01 Task 3, shadcn components (`DropdownMenu.Item variant="destructive"`, `Dialog` delete triggers) would silently fall back to transparent/inherited text colors. Aliasing `--destructive: var(--cat-gotcha)` ensures error/destructive UI elements maintain high-contrast visual hierarchy.

- **Non-Mutilating Transport Interceptor Composition (Plan 19-02)**  
  *Evidence:* Plan 19-02 configures `interceptors: [retryOnce, attachCsrf]` on `engramWrite`. Because Connect-ES evaluates interceptors outer-to-inner, placing `retryOnce` outermost ensures that when a transient auth failure triggers a retry, execution re-enters `attachCsrf` inner-wise, dynamically reading the freshly rotated `engram_csrf` cookie from `document.cookie` before dispatching the second request.

- **Pre-Handler Idempotency on Write Retries (Plan 19-02)**  
  *Evidence:* Examining [internal/server/connectcsrf.go:L70-87](file:///Volumes/Code/github.com/seanb4t/engram/internal/server/connectcsrf.go#L70-L87) verifies that CSRF token mismatches and unauthenticated sessions return `connect.CodePermissionDenied` or `CodeUnauthenticated` before invoking any service handler logic. Retrying state-mutating requests (`StoreMemory`, `StoreDiscovery`) on these two status codes carries zero risk of double-store or duplicate creation because the server aborted processing on the first attempt prior to database write execution.

- **Input Preservation on Hard Auth Expiration (Plan 19-05, D-09)**  
  *Evidence:* Plan 19-05 Task 1 implements a resilient failure recovery pattern: if a mutation fails post-retry with `Unauthenticated` or `PermissionDenied`, `MemoryFormSheet` sets a `hardAuthFailure` flag while keeping `open = true` and preserving all bound `$state` field values. The operator is presented with an inline re-auth affordance without discarding their typed content.

- **Strict Adherence to Boundary Fences (Plans 19-03, 19-04, D-04/D-05)**  
  *Evidence:* Plan 19-04 Task 2 explicitly omits discovery updates (`useUpdateDiscovery` does not exist), and Plan 19-03 Task 2 excludes rules from row/detail action affordances, preserving zero-junk immutability invariants.

---

## 3. Concerns

### LOW: FieldMask Construction Syntax in `useUpdateMemory` (Plan 19-04)
- **File / Location:** `ui/src/lib/mutations/memory.ts` (Plan 19-04 Task 1)
- **Mechanism:** In [gen/ts/engram/v1/engram_pb.ts:L712](file:///Volumes/Code/github.com/seanb4t/engram/gen/ts/engram/v1/engram_pb.ts#L712), `UpdateMemoryRequest.updateMask` is specified as `google.protobuf.FieldMask | undefined`. Under `@bufbuild/protobuf` v2, `FieldMask` expects an object of shape `{ paths: string[] }` (or `create(FieldMaskSchema, { paths: [...] })`), not a plain array of field names (`string[]`).
- **Impact:** If an implementer attempts to set `update_mask: ['content', 'tags']` directly, TypeScript compilation will fail.
- **Mitigation:** Explicitly document in Plan 19-04 implementation notes that `updateMask` must be constructed as `{ paths: ['content', 'tags', ...] }` (or using `@bufbuild/protobuf/wkt` helpers).

---

## 4. Suggestions

1. **Verify Query Key Parity Across Optimistic Invalidation (Plan 19-04 & 19-06)**  
   Ensure that `useCreateMemory`, `useUpdateMemory`, `useDeleteMemory`, and `useSetMemoryVisibility` invalidate both standard query keys (`['listMemories']`, `['getMemory', id]`) and active scope-filtered keys (`listMemoriesKey(...)` from `ui/src/lib/queries.ts`) so that background lists re-sync immediately across page switches.

2. **Form Reset on Success vs. Hard-Fail Retain (Plan 19-05)**  
   Confirm that form fields reset to default initial values *only* inside `onSuccess` callbacks when `open` transitions to `false`, preventing stale input state from polluting subsequent "New Memory" sheet invocations.

---

## 5. Risk Assessment

- **Overall Risk:** **LOW**
- **Justification:** The 6 plans are remarkably detailed, well-grounded in existing codebase mechanics, and verified against actual proto schemas, server interceptor logic, and Svelte 5 rune conventions. The architectural decisions around CSRF attachment, retry-once interceptors, optimistic cache rollbacks, and input retention on re-auth strictly protect both system security and operator productivity.

---

### Recommended Next Steps
Proceed with executing **Wave 1 (Plan 19-01)** to establish the re-vendored Connect stubs and `--destructive` CSS design tokens.

---

## Consensus Summary

**Grounded reviewers (source-verified, `file:line` evidence):** Codex, OpenCode (grok-4.5). Both independently opened the referenced files and traced claims against live code. **Antigravity** was prompt-fed but low-signal this round — it largely restated the plans' own claims, surfaced one LOW issue, rated risk LOW, and listed "input preservation on hard auth" as a *strength* that the two grounded reviewers independently traced and **refuted** (the OIDC redirect destroys `$state`). Plan-level consensus below is therefore weighted on Codex + OpenCode; Antigravity is treated as a corroborating source only where it agrees on verifiable facts.

**Verdict: NOT execute-ready as written.** The plans are well-decomposed and the security/transport reasoning holds, but Codex and OpenCode converge on a cluster of RPC-schema-mismatch HIGH issues in Plans 04/05 that will fail SC1 in UAT, plus DOM/variant defects in 03 and an unmet SC3 input-preservation promise. Codex additionally surfaced two blockers the others missed.

### Agreed Strengths (2+ grounded reviewers, source-verified)

- **The stale gen-client blocker is real and correctly diagnosed.** All three confirm `ui/src/lib/gen/engram_pb.ts` exposes only the 5 read RPCs; the 6 write schemas + `Citation`/`Visibility` live only in canonical `gen/ts/engram/v1/engram_pb.ts`, and **no Task/CI target syncs them** (`Taskfile.yaml:145` runs only buf; CI `buf` job diffs `gen/`; `ui-drift` diffs `internal/webauth/static/`). Plan 19-01's re-vendor + drift-guard is the right *class* of fix.
- **`--destructive` / `--color-destructive` genuinely absent** from `ui/src/app.css` while vendored shadcn primitives reference `text-destructive`/`bg-destructive` — 19-01's token addition is required.
- **Interceptor order `[retryOnce, attachCsrf]` is correct.** Connect-ES reverse-wraps the array (`@connectrpc/connect/.../interceptor.js`), making `retryOnce` outermost so a retry re-enters `attachCsrf` and re-reads the rotated cookie.
- **The double-create threat is genuinely mitigated by server design** (validates the planner's threat model): auth + CSRF reject *before* the handler (`connectauth.go`, `connectcsrf.go:70-87`), and business-handler errors never map to `Unauthenticated`/`PermissionDenied` (`connecterror.go`). Retrying those two codes cannot double-create.
- **Wave graph is sound and scope fences D-04 (no discovery edit) / D-05 (no rule authoring) are honored** at the plan level.

### Agreed Concerns (Codex + OpenCode converge — highest priority)

- **HIGH — Create + shared/visibility cannot be written by the Store*/Schedule RPCs.** `StoreMemoryRequest`, `StoreDiscoveryRequest`, and `ScheduleMemoryRequest` have **no** `shared`/`visibility` field; the server treats `""` as private. Plans 04/05 collect a create-time visibility choice but never specify the required **second `SetVisibility` call** after create. As written, SC1's "create … re-share" succeeds in the UI but leaves the record private.
- **HIGH — Discovery form under-specified vs server validation (`tools.go:575-602`).** It omits the **required non-empty `content`** field and treats citations as bare URL strings, but the server requires ≥1 citation with `kind ∈ file|commit|url|repo` + non-empty `ref`, and `scope` must carry the `discovery:` prefix. A discovery create will bounce as `InvalidArgument` → SC1 fails.
- **HIGH — Schedule is a create-only RPC, not an edit path.** `ScheduleMemoryRequest` carries no `id`; routing an *edit* submission with a window through `ScheduleMemory` **duplicates** the record rather than scheduling the existing one. Restrict schedule UI + RPC to create mode.
- **HIGH — SC3 input-preservation is unmet on hard re-auth.** `/auth/login` redirects to the IdP and the callback returns to `/ui/` (`internal/webauth/handlers.go:60,187`), destroying component `$state`. The sheet-kept state survives the *failed mutation* (good) but NOT the OIDC round-trip. Either persist a draft (sessionStorage, currently deferred) or reword SC3 as "retained until explicit re-auth navigation." (Antigravity mis-scored this as a strength.)
- **HIGH/MEDIUM — `MemoryRow` is itself a `<button>` (`MemoryRow.svelte:19`).** Nesting a dropdown trigger inside creates invalid nested-interactive DOM and fragile `stopPropagation`. The row shell must become a non-button container before adding the kebab menu.
- **HIGH/MEDIUM — No `Button variant="destructive"` exists** (`button.svelte` has only `default|outline|ghost|surface`). CSS tokens alone don't add a component variant; the DeleteConfirmDialog's destructive primary won't compile/style as specified unless 19-01/03 adds the variant or uses explicit destructive classes.

### Codex-unique HIGH findings (grok + agy missed — verify first)

- **HIGH — The whole-file gen copy will not compile.** Canonical `gen/ts/engram/v1/engram_pb.ts:12` imports `../../buf/validate/validate_pb`, which is **not** vendored under `gen/ts/` or `ui/src/`, and flattening the file into `ui/src/lib/gen/engram_pb.ts` changes what `../../buf` resolves to. 19-01's "whole-file cp" approach as written leaves every downstream plan uncompilable. The re-vendor must generate/preserve the imported protobuf deps and directory structure, and add a compile check (not just a byte-diff).
- **HIGH — The built SPA is never rebuilt/committed.** Production serves `internal/webauth/static/`; `task ui:build` rebuilds+copies it and CI **rejects drift** (`ci.yaml:155`). No plan runs `task ui:build` or commits the updated bundle, so the shipped binary would remain unchanged even if the source UI works. Add a final task: `task ui:build` → commit `internal/webauth/static/` → full `task` gate.

### Notable MEDIUM/LOW (single grounded reviewer)

- MEDIUM — Optimistic create is under-specified: `StoreMemoryResponse` is `{id, short_id}` only, so an optimistic list-insert needs a temp client id or must fall back to invalidate-only (grok).
- MEDIUM — Multi-cache rollback needs `getQueriesData`/`setQueriesData`, not a single `getQueryData` snapshot, to cover every scope/filter/page key and the `{memories}` vs `{discoveries}` shapes (codex).
- MEDIUM — `createMutation` hook tests aren't viable as bare node tests (it calls `useQueryClient`, which throws without a Svelte provider); extract pure option/cache factories or use a provider harness (codex + grok).
- MEDIUM — No "Make private"/unshare path though `SetVisibility(false)` works server-side; and the share-warning copy is technically false (a record *can* be unshared; only prior disclosure is irreversible) (codex + grok).
- MEDIUM — Private-record prefill needs normalization: the server returns `visibility === ""`, not `"private"` (codex + grok).
- MEDIUM — Foreign *shared* records surface in reads but writes require ownership; the UI needs a current-owner comparison to avoid dead write affordances (codex).
- LOW — Acceptance string names `StoreMemorySchema`; the generated symbol is `StoreMemoryRequestSchema` (all three — cosmetic, but fix the verify command).
- LOW — `UpdateMemoryRequest.updateMask` must be built as `{ paths: [...] }` (or `create(FieldMaskSchema, …)`), not a plain `string[]` (antigravity — legitimate).

### Divergent Views

- **Overall risk:** Codex **HIGH / not ready** (7 replanning priorities); OpenCode **MEDIUM** ("patch Plans 04/05 + the MemoryRow shell before execute; the rest is ready"); Antigravity **LOW / proceed**. The Antigravity verdict is discounted — it missed every HIGH finding and inverted the SC3 assessment.
- **Gen re-vendor severity:** Codex rates it **HIGH** (won't compile — the `buf/validate` import). OpenCode rates it **LOW** ("soft spot only if the CI copy is missed") and did not catch the import blocker. Resolve by attempting the re-vendor and confirming `ui/` typechecks before trusting either framing.
- **Retry/reseal mechanism:** Codex flags **HIGH** that a failed request *cannot* trigger a re-seal (`connectreseal.go:39` skips errored responses), so "retry through re-seal" is not achievable frontend-only — recast it as an opportunistic cookie/CSRF-race retry. OpenCode treats the retry as safe/LOW and did not flag the reseal-on-retry impossibility as strongly. This matches RESEARCH.md Open Question 1 and should be reconciled: the retry helps races, not deterministic rotation recovery.
