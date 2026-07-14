---
phase: 19
reviewers: [codex, opencode, antigravity]
reviewed_at: 2026-07-14T17:49:22Z
review_round: 2
reviewed_commit: cf07a388
opencode_model: openrouter/x-ai/grok-4.5
codex_cli: 0.144.1
agy_cli: 1.1.2
plans_reviewed: [19-01-PLAN.md, 19-02-PLAN.md, 19-03-PLAN.md, 19-04-PLAN.md, 19-05-PLAN.md, 19-06-PLAN.md]
---

# Cross-AI Plan Review — Phase 19 (Round 2)

Round 2 review of the revised plans (commit `cf07a388`, which incorporated the round-1 findings) against live source. Codex and OpenCode/grok-4.5 are the grounded reviewers; antigravity was genuinely source-grounded this round (a real improvement over round 1) but returned a clean APPROVED while missing the two convergent blockers the grounded pair found.

---

## Codex Review

# Round 2 verdict

The revisions materially improve the plans, especially generated-client vendoring, discovery validation, valid row DOM, and embedded-SPA shipping. However, the phase has not converged: six blocking issues remain. The most serious are incorrect generated enum names, incomplete scheduled-and-shared creation, loss of edit state across re-auth, editing summary-shaped records, an impossible visibility-gating API, and a retry requirement the current server cannot actually satisfy.

## 19-01 — Foundation

### Summary

The generated-client and destructive-token fixes are mostly well designed. Structure-preserving vendoring and the real compile gate directly address the earlier failure.

### Strengths

- The plan correctly identifies the unresolved generated import at [gen/ts/engram/v1/engram_pb.ts:12](/Volumes/Code/github.com/seanb4t/engram/gen/ts/engram/v1/engram_pb.ts:12). Copying the whole TS tree plus a stable barrel is the right shape.
- Extending the existing drift job is appropriate: current CI only checks `gen/` at [.github/workflows/ci.yaml:127](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/ci.yaml:127).
- Adding a real Button variant is necessary because the current component exposes only four variants at [button.svelte:8](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/components/ui/button/button.svelte:8).
- `pnpm check` is a meaningful acceptance gate, unlike a byte comparison alone.

### Concerns

- **MEDIUM — destructive foreground contrast.** The plan assigns white foreground in both modes, but dark mode uses the light orange `#ffa657` at [app.css:28](/Volumes/Code/github.com/seanb4t/engram/ui/src/app.css:28). White text on that background will have weak contrast.
- **LOW — import generation is unnecessarily discretionary.** The action first suggests global `--include-imports`, which would affect all three plugins. The existing ES plugin is isolated at [buf.gen.yaml:21](/Volumes/Code/github.com/seanb4t/engram/buf.gen.yaml:21), so plugin-scoped `include_imports: true` is the deterministic solution.

### Suggestions

- Set `--destructive-foreground: var(--background)` per theme, or use an explicit dark foreground in `.dark`.
- Specify `include_imports: true` only on the ES plugin; avoid “try global, constrain if needed.”

### Risk Assessment

**MEDIUM.** The build approach is sound, but the destructive color choice should be fixed before it becomes a shared UI primitive.

---

## 19-02 — Write transport

### Summary

The interceptor construction is safe and well tested, but the round-one “retry through reseal” problem has been renamed rather than resolved. The implementation cannot fulfill SC3 as currently worded.

### Strengths

- `[retryOnce, attachCsrf]` is the correct order: Connect applies interceptors in reverse wrapping order at [interceptor.js:19](/Volumes/Code/github.com/seanb4t/engram/ui/node_modules/@connectrpc/connect/dist/esm/interceptor.js:19).
- CSRF names match the server contract at [connectcsrf.go:22](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectcsrf.go:22).
- Retrying these codes is currently safe from duplicate writes: authentication rejects before the handler at [connectauth.go:18](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectauth.go:18), CSRF rejects before `next()` at [connectcsrf.go:58](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectcsrf.go:58), and business errors do not map to either retry code at [connecterror.go:49](/Volumes/Code/github.com/seanb4t/engram/internal/server/connecterror.go:49).

### Concerns

- **HIGH — SC3 remains unachievable with the existing backend.** Failed requests are explicitly not resealed at [connectreseal.go:39](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectreseal.go:39). An expired session is rejected before the handler at [resolver.go:49](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/resolver.go:49), while a merely aging but valid session succeeds and is resealed normally. There is no “needs rotation” error state.
- **MEDIUM — the proposed CSRF freshness race is largely theoretical.** The CSRF token is owner-bound and intentionally stable across reseals at [csrf.go:38](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/csrf.go:38) and [reseal.go:41](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/reseal.go:41). A concurrent reseal refreshes expiry, not token value.

### Suggestions

- Amend SC3 and D-08 to promise one opportunistic auth-class retry, not “retry through a re-seal.”
- If literal reseal recovery is mandatory, add an explicit server recovery contract; the frontend interceptor alone cannot provide it.
- Keep the retry tests, but name them “auth-race retry,” not rotation recovery.

### Risk Assessment

**HIGH.** The code can be correct while the phase success criterion remains false.

---

## 19-03 — Action affordances

### Summary

This plan successfully fixes the invalid nested-button design and mechanically protects rules. Its remaining weakness is that the callback model only expresses “share,” not general visibility changes.

### Strengths

- Restructuring `MemoryRow` is necessary: its current root is a button at [MemoryRow.svelte:19](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/components/MemoryRow.svelte:19).
- Explicit rule guards in both row and detail components are stronger than relying on route behavior.
- The revised sharing copy accurately distinguishes unsharing from irreversible prior disclosure.
- The 360px detail-pane constraint is grounded at [MemoryDetail.svelte:32](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/components/MemoryDetail.svelte:32).

### Concerns

- **MEDIUM — the `onshare(id)` API cannot enforce private-only rendering downstream.** `MemoryRow` already owns the complete `Memory`, including visibility, but the planned callback discards that state.
- **MEDIUM — visibility is reduced to one direction.** The server supports both shared and private through `SetVisibility`; private maps back to the empty canonical value at [store.go:1571](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1571). Shared discoveries otherwise have no way to become private because discovery has no edit form.

### Suggestions

- Replace `onshare(id)` with `onvisibility(memory, target)` or separate `onshare(memory)`/`onmakeprivate(memory)` callbacks.
- Hide Share directly in `MemoryRow`/`MemoryDetail` when `memory.visibility === 'shared'`; do not defer that to the host.

### Risk Assessment

**MEDIUM.** The presentational fixes are strong, but the callback contract causes Plan 06’s visibility logic to fail.

---

## 19-04 — Mutation hooks

### Summary

The mutation architecture is thoughtful, particularly partial-failure handling and multi-key rollback. Two concrete schema mismatches still block execution.

### Strengths

- The create-then-SetVisibility composite correctly reflects that store requests contain no visibility field at [engram.proto:104](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:104).
- Typed discovery citations match both the schema at [engram.proto:121](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:121) and server validation at [tools.go:575](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:575).
- Invalidate-only create avoids fabricating records from responses that contain only IDs.
- Extracting cache transforms avoids calling `useQueryClient()` outside Svelte context; the installed implementation requires that context at [createMutation.svelte.ts:26](/Volumes/Code/github.com/seanb4t/engram/ui/node_modules/@tanstack/svelte-query/src/createMutation.svelte.ts:26).

### Concerns

- **HIGH — the planned enum member names do not exist.** The generated ES enum exposes `Visibility.PRIVATE` and `Visibility.SHARED` at [engram_pb.ts:927](/Volumes/Code/github.com/seanb4t/engram/gen/ts/engram/v1/engram_pb.ts:927), not `VISIBILITY_PRIVATE` or `VISIBILITY_SHARED` as specified repeatedly in the plan.
- **HIGH — scheduled-and-shared creation is not implemented.** `ScheduleMemoryRequest` has no visibility field at [engram.proto:203](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:203), but the task only gives the composite to `useCreateMemory`; `useScheduleMemory` neither accepts `shared` nor performs the second call.
- **MEDIUM — filtered cache membership is under-specified.** Visibility is embedded in `listMemoriesKey` at [queries.ts:34](/Volumes/Code/github.com/seanb4t/engram/ui/src/lib/queries.ts:34). Merely changing `memory.visibility` leaves shared records in private-only cached lists until refetch. Delete should also update `total`.
- **MEDIUM — dirty-field behavior needs an explicit test.** Passing unchanged content/tags still forces a re-embed because pointer presence controls the branch at [tools.go:1003](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:1003). Passing an unchanged auto summary also converts its provenance to client-authored at [store.go:1404](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1404).

### Suggestions

- Replace every generated enum reference with `Visibility.SHARED` or `Visibility.PRIVATE`.
- Give `useScheduleMemory` the same `shared` composite and partial-failure behavior as ordinary create.
- Add tests for scheduled+shared partial failure.
- Require true dirty-field diffing and disable Save when the resulting mask is empty.
- Remove records from incompatible filtered caches and update totals; invalidate `['listScopes']` after create/delete/schedule.

### Risk Assessment

**HIGH.** The enum references fail compilation, and scheduled shared records would silently remain private.

---

## 19-05 — Form sheets

### Summary

The form/schema reconciliation is much improved, especially immutable edit fields and typed discovery citations. The claimed sessionStorage fix still does not preserve a resumable edit across the actual OIDC round trip.

### Strengths

- Edit-mode scope/category locking matches the update schema, which permits only content/shared/tags/summary at [engram.proto:164](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:164).
- Schedule is correctly restricted to create mode because the RPC has no ID.
- Discovery requires the correct content, citation, kind, and scope fields.
- The relational schedule validation mirrors the schema CEL at [engram.proto:203](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:203).

### Concerns

- **HIGH — the sessionStorage restoration is not integrated with routing or host state.** Authentication always returns to `/ui/` at [handlers.go:187](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/handlers.go:187). That root page currently has no write host at [+page.svelte:1](/Volumes/Code/github.com/seanb4t/engram/ui/src/routes/+page.svelte:1), and Plan 06 only adds hosts to observe/search/discovery. Therefore no matching sheet necessarily remounts after authentication.
- **HIGH — edit drafts cannot reconstruct edit mode.** The proposed key contains an ID, but `WriteSurfaces.editMemory` is lost on navigation. After callback there is no record prop telling `MemoryFormSheet` which edit key to restore or which record to prefill.
- **HIGH — scheduled+shared submission inherits Plan 04’s hole.** The form sends scheduled creation through `useScheduleMemory`, while only ordinary create receives the shared intent.
- **MEDIUM — edit submission lacks explicit dirty-mask/no-op behavior.** A form that submits all bound fields can unnecessarily re-embed and rewrite auto-summary provenance.

### Suggestions

- Persist a resume envelope containing return URL, kind, mode, record ID, and values.
- On `/ui/` startup, consume that envelope, navigate back to the originating route, fetch the full record for edit mode, and reopen the correct sheet.
- Test the route-level callback landing, not just component unmount/remount in isolation.
- Pass `shared` through scheduled creation.
- Diff edit values against the original record before constructing `update_mask`.

### Risk Assessment

**HIGH.** The component-level draft test can pass while real re-auth still loses the active edit workflow.

---

## 19-06 — Route integration and shipping

### Summary

The shipping and route-fence portions are solid, but two host contracts are internally impossible, and row editing risks destructive data loss.

### Strengths

- Rebuilding the embedded SPA is essential and correctly specified: `task ui:build` vendors into `internal/webauth/static` at [Taskfile.yaml:21](/Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml:21), and CI rejects drift at [.github/workflows/ci.yaml:155](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/ci.yaml:155).
- Discovery correctly omits edit wiring.
- Transitive dependency ordering is valid: Plan 04 brings in Plan 02 and Plan 01.
- Rule affordance suppression is assigned to the components closest to the record.

### Concerns

- **HIGH — row editing passes summary-shaped data into the edit form.** List and search requests omit `full`, as seen at [observe/+page.svelte:27](/Volumes/Code/github.com/seanb4t/engram/ui/src/routes/observe/+page.svelte:27) and [search/+page.svelte:14](/Volumes/Code/github.com/seanb4t/engram/ui/src/routes/search/+page.svelte:14). The server explicitly clears content when `full=false` at [connectapi.go:70](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:70). Calling `openEdit(recordFor(id))` can therefore prefill empty content and overwrite the real body.
- **HIGH — `requestShare(id, kind)` cannot know visibility.** The exact host API accepts only ID and kind, yet acceptance requires it to distinguish private from shared. Plan 03 also passes only ID. The “already shared is a no-op” test has no defined data source.
- **MEDIUM — discovery visibility remains one-way.** Discovery has no edit form and the route offers only Share, so a shared discovery cannot be made private despite the phase’s “change visibility” scope.
- **MEDIUM — re-auth resume remains unwired.** Nothing in the listed route changes consumes the Plan 05 resume envelope after the fixed `/ui/` callback.

### Suggestions

- Change `openEdit(record)` to `openEdit(id)` and fetch `['getMemory', id]` before opening the sheet. `GetMemory` returns full content at [connectapi.go:202](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:202).
- Change the visibility API to accept the record/current visibility and target state, e.g. `requestVisibility(record, 'shared' | 'private')`.
- Add “Make private” for already-shared records, especially discoveries.
- Integrate the re-auth resume envelope in layout or root-route startup.
- Add route-level browser tests proving row edit loads full content and callback recovery reopens the correct sheet.

### Risk Assessment

**HIGH.** The current route wiring can overwrite full content with an empty summary-shaped value and cannot implement its own visibility acceptance criteria.

---

# Overall Risk Assessment

**HIGH — revision required before execution.**

The phase should not execute until these blockers are incorporated:

1. Use the actual ES enum members: `Visibility.PRIVATE` / `Visibility.SHARED`.
2. Implement scheduled-create → SetVisibility for shared scheduled records.
3. Reconcile SC3 with the server’s actual reseal semantics.
4. Add route-aware, mode-aware re-auth restoration.
5. Fetch full records before opening edit from a row.
6. Replace the ID-only sharing API with a visibility-aware, bidirectional contract.

---

## OpenCode Review (grok-4.5 — openrouter/x-ai/grok-4.5)

Verifying plan claims against the live source for Round 2 review.
# Phase 19 Plan Review — Round 2

Verified claims against live tree under `/Volumes/Code/github.com/seanb4t/engram` (not just plan text / `<review_incorporation>`).

---

## Overall phase verdict

Wave structure, proto-aligned create→SetVisibility composite, non-button row shell, sessStorage re-auth draft, structure-preserving gen vendor + `ui:build` embed are real improvements over round 1 and map to SC1–SC3.

**One HIGH defect remains** that will ship wrong UX if unfixed: **create-with-schedule + shared loses the share**. Plus a **compile-breaking Visibility enum naming** error in Plan 04 task text.

**Phase residual risk: MEDIUM** (HIGH items are localized and fixable before Wave 3).

---

## 19-01 — Foundation (re-vendor + destructive)

### Summary
Correctly treats gen re-vendor as a **structure + compile** problem, not a flat `cp`. Matches `gen/ts/engram/v1/engram_pb.ts:12` importing `../../buf/validate/validate_pb` while `gen/ts/buf/` **does not exist today**, and the UI copy (`ui/src/lib/gen/engram_pb.ts`) exposing **only 5 read RPCs**. Destructive Button gap is real (`button.svelte:8–12` has only `default|outline|ghost|surface`; `app.css` has no `--destructive`).

### Strengths
- Structure-preserving vendor + barrel keeps `client.ts:3` `./gen/engram_pb` stable.
- Real gate: `pnpm check` (not byte-diff only).
- Button variant required, not token-only (`dropdown-menu-item.svelte` already uses `text-destructive` / `data-[variant=destructive]`).
- CI drift on `ui/src/lib/gen/` mirrors existing `gen/` drift (`ci.yaml` ~129–130).

### Concerns
| Severity | Finding |
|----------|---------|
| **MEDIUM** | `--include-imports` path is **unproven** on this repo. `buf.gen.yaml` only plugs `buf.build/bufbuild/es` → `gen/ts`. Plan correctly counts on managed `go_package` for protovalidate; still must verify `git diff -- gen/go/` stays empty and that `validate_pb.ts` is enough for tsc (fileDesc blob links `file_buf_validate_validate`). |
| **LOW** | Task 1 verify is only `rg "ui/src/lib/gen"` — weak vs acceptance. Compile check is in Task 2; OK if Task 2 is not skipped. |
| **LOW** | `cp -R gen/ts/. ui/src/lib/gen/` after `rm -rf engram buf` leaves the hand barrel; document “never copy root `engram_pb.ts` from gen/ts” (there is none today — OK). |

### Suggestions
- After Task 1 config, run `go tool buf generate` once and paste the resulting tree into SUMMARY (`gen/ts/buf/validate/...` present).
- Point Task 1 verify at `test -f gen/ts/buf/validate/validate_pb.ts` after generate.

### Risk
**LOW–MEDIUM** — foundation is sound; operational risk is only get-buf-to-emit validate.

### Round-1 incorporation
buf/validate + Button `destructive` + `StoreMemoryRequestSchema` naming: **verified needed and corrected in plan**. SOUND items kept.

---

## 19-02 — Write transport

### Summary
Csrf names match wire contract (`internal/webauth/csrf.go:22–23`). Reseal-on-error skip is real (`connectreseal.go` path via `if err != nil || resp == nil`). Interceptor story and separate `engramWrite` are right. Retry framing is **honest** post-round-1.

### Strengths
- `[retryOnce, attachCsrf]` outer→inner for cookie re-read.
- Retry only `Unauthenticated` | `PermissionDenied` maps to real pre-handler codes: auth (`connectauth.go`) and CSRF (`connectcsrf.go` → `CodePermissionDenied`).
- Foreign ownership writes are **`CodeNotFound`**, not those codes (`connecterror.go:50–51`, `connectapi_crossowner_test.go`) — so retry/D-09 won’t masquerade as “re-login still owns this.” Good separation.
- Double-create mitigation holds: those codes fire before mutate (`connectapi.go` ~351–365 interceptor stack).

### Concerns
| Severity | Finding |
|----------|---------|
| **LOW** | Persistent CSRF/`PermissionDenied` after retry still routes to D-09 re-auth in forms — Round-trip often remints `engram_csrf`; slightly broad UX, acceptable. |
| **LOW** | `mapAuthError` only redirects on `Unauthenticated` (`client.ts:15–19`); write forms must not rely on it for permission-class hard-fail (Plan 05 does its own branch — OK). |

### Suggestions
- Keep relapse-forbidden wording: “opportunistic cookie/CSRF race,” not “reseal on retry” (plan already says this).
- Recommend the file-as-follow-up server contract test for “those codes stay pre-handler” (already noted in T-19-11).

### Risk
**LOW**

### Round-1 incorporation
Reseal-on-retry: **correctly recast** vs `connectreseal.go`. SOUND order kept.

---

## 19-03 — Destructive affordances

### Summary
Grounded: `MemoryRow.svelte:19–24` is a `<button>` shell — nest kebab is unsafe. Rule fence must be mechanical. Share copy corrected for unshare-possible-but-disclosure-irreversible matches store semantics (`Visibility` `""` ↔ private, `"shared"` shareable again).

### Strengths
- Presentational-only (no client yet) allows parallel Wave 2 with 19-02.
- Callback threading mirrors `MemoryList.svelte:6–8` `onselect` pattern.
- Drop-down already supports `variant="destructive"`; Button does not until 01 — correct dep.
- Detail pane `w-[360px]` (`MemoryDetail.svelte:32`) overflow note is real.

### Concerns
| Severity | Finding |
|----------|---------|
| **MEDIUM** | Callbacks are `onedit?(id)` only, while edit needs full `Memory` for prefill. Forces weak `recordFor(id)` later (see 19-06). Better: pass `Memory` (row/detail already have it). |
| **LOW** | Share item shown by row even when already `shared` is deferred to Plan 06 exclude — OK if 03 renders item and 06/host no-ops; cleaner if 03 also hides Share when `visibility === 'shared'` (normalize `''` as private). |

### Suggestions
- Change props to `(memory: Memory) => void` or pass `(id, memory)` so Plan 06 doesn’t invent lookup.
- Browser-test nested-button: assert root tag not `button` and trigger not descendant of interactive selection control.

### Risk
**LOW**

### Round-1 incorporation
Non-button shell, rule guard, share copy, overflow: **verified real + addressed**.

---

## 19-04 — Mutation hooks

### Summary
Create-as-shared composite is **required** by proto: `StoreMemoryRequest` fields end at `summary` with **no** visibility (`proto/engram/v1/engram.proto:104–115`); create path leaves `Visibility` unset → `""` private (`tools.go` `toMemory` / `storeMemory`; `store.go:108`). Partial-failure keep-private toast is good. Multi-key cache + invalidate-only create match response shape `{id, short_id}`.

### Strengths
- FieldMask as object (`UpdateMemoryRequest.update_mask` / CEL allowlist content|shared|tags|summary) — use `FieldMaskSchema` from `@bufbuild/protobuf/wkt` (available; gen imports FieldMask there).
- Discovery shape matches `validateStoreDiscovery` (`tools.go:575–602`) + proto citations/min_items/prefix.
- No discovery edit export = D-04.
- Cache factories for node tests — necessary for v6 + `useQueryClient`.

### Concerns
| Severity | Finding |
|----------|---------|
| **HIGH** | **Wrong Visibility enum member names in task action copy:** plan uses `Visibility.VISIBILITY_SHARED` / `VISIBILITY_PRIVATE`. Generated TS is `Visibility.SHARED` / `Visibility.PRIVATE` / `Visibility.UNSPECIFIED` (`gen/ts/engram/v1/engram_pb.ts` ~927–946). Go uses `Visibility_VISIBILITY_*`; **TS does not**. As written, implementers copy-paste → **tsc failure or UNSPECIFIED=0** which server rejects (`not_in: [0]`). must_haves once say `Visibility.SHARED` correctly — **normalize the plan to TS names**. |
| **HIGH** | **Schedule path ignores `shared`.** `useScheduleMemory` has no shared composite; Plan 05 routes create+window → schedule only. Operator selects shared + schedule → **private scheduled memory**, silent violation of T-19-43. Fix: `useScheduleMemory({..., shared})` → ScheduleMemory then SetVisibility(SHARED), same partial-failure toast. |
| **MEDIUM** | `updateMask` paths must be snake proto names (`content`, `shared`, `tags`, `summary`) not camelCase field names — state that explicitly (CEL allowlist). |
| **MEDIUM** | Response field is `shortId` in TS; keep SetVisibility on `resp.id` (plan OK). |
| **LOW** | Toasts more than one word for partial share (`created (private) — sharing failed`) violate “one-word” voice slightly — intentional; fine. |

### Suggestions
- Single helper: `async function createThenMaybeShare(id, shared, setVis)`.
- Test: schedule+shared → two RPCs; schedule-only → one.

### Risk
**MEDIUM–HIGH** until Visibility names + schedule+shared fixed.

### Round-1 incorporation
Create composite, multi-key rollback, invalidate-only create, Discovery shape: **verified RESOLVED**. Visibility symbol names: **new HIGH introduced/unclean**.

---

## 19-05 — Form sheets

### Summary
Strongest post-reopen plan. Create-only schedule (no id on `ScheduleMemoryRequest` 203–227), edit scope/category disabled (`UpdateMemoryRequest` content/shared/tags/summary only), discovery server-required fields, and **sessionStorage draft** are all grounded. OIDC leaves SPA (`handlers.go` Login redirect ~60–81; Callback → `/ui/` ~187) so live `$state` alone **cannot** meet D-09 — reversing deferred draft is correct, not polish.

### Strengths
- Prefill `visibility === ''` → private matches UI `Visibility` type (`queries.ts:4`) and store (`""` private).
- Share gate as intent, not create RPC field.
- Client schedule CEL mirror (at least one bound; not_after > not_before).
- Discovery citations typed `{kind,ref}`; bare URL invalid.

### Concerns
| Severity | Finding |
|----------|---------|
| **HIGH** | Same **shared + schedule** routing bug: “Create → schedule when window set, else create(shared)” drops share when window set. Coordinate with 04. |
| **MEDIUM** | Draft key `engram:draft:memory:{mode}:{id}` risks leaking partial secrets into sessionStorage — plan T-19-53 mitigates clear-on-success; also clear on Cancel and successful re-auth submit; never store auth cookies. |
| **MEDIUM** | After OIDC, remount may open with restore but sheet not automatically open — D-09 “resubmit after re-auth” needs “reopen sheet if draft present” (or explicit wizard). Specify open-from-draft on mount. |
| **LOW** | Default scope on Search when no `?scope` is empty — correct per research; caveat UAT. |

### Suggestions
- On mount: if draft restored → `open = true` + surface “recovered draft after sign-in.”
- Unified mutation entry: form always posts one payload `{fields, shared, window?}` and one hook chooses schedule vs store then maybe share.

### Risk
**MEDIUM** (D-09 story good; schedule+share + post-login open gaps)

### Round-1 incorporation
OIDC draft, create-shared, schedule create-only, discovery fields, edit immutables: **verified against source and mostly RESOLVED**. Schedule+share remains open.

---

## 19-06 — Route integration + embed ship

### Summary
Payoff plan closes the phase if dependencies hold. Exact `bind:this` methods, discovery no `onedit`, rebuild `task ui:build` → `internal/webauth/static/` (matches ADR + `ci.yaml` ui-drift ~155–159) address real ship-risk. Foreign shared writes: **NotFound + rollback**, not invent whoami — matches store isolation (`store.go:108–109`, cross-owner tests).

### Strengths
- Ship gate forces product path through go:embed, not only `ui/src`.
- Discovery scope default rules avoid raw observe scope (prefix check real in proto + tools).
- `requestShare` no-op when already shared — aligns with dead-share concern.

### Concerns
| Severity | Finding |
|----------|---------|
| **MEDIUM** | `openEdit(record)` vs `onedit(id)` + undefined `recordFor(id)` — easy miss when detail selected memory is current, list page incomplete under filters. Prefer callbacks carry `Memory`. |
| **MEDIUM** | `requestShare(id)` cannot know private vs shared without cache/record; row should pass visibility or full Memory; also host no-op logic needs that input. |
| **MEDIUM** | Discoveries use `category: "discovery"` in store (`tools.go` storeDiscovery); rule fence is `category === 'rule'` — OK. Ensure no edit ever mounted on discovery (plan states). |
| **LOW** | New button placement vs AppShell header search — placement discretion fine; avoid header collision documented in SUMMARY. |
| **LOW** | Wave depends on 05 only for forms; depends_on lists 03/04/05 — OK; exceeds-loading if 19-01 gen broken. |

### Suggestions
- Host API: `requestShare(memory: Memory)`, `openEdit(memory: Memory)`, `requestDelete(id, kind)`.
- Explicit smoke UAT matrix in SUMMARY for CSRF header, create-as-shared, schedule+shared, re-auth draft restore, foreign share toast.

### Risk
**MEDIUM** (integration glue / ship path sound; callback shapes underspecified)

### Round-1 incorporation
`ui:build`, exact host API, D-04 route fence, dead share, foreign docs: **verified needed / largely RESOLVED**.

---

## Cross-cutting (all plans)

| Item | Status after R2 check |
|------|------------------------|
| Stale UI gen client (5 RPC) | True |
| `buf/validate` missing under gen/ts | True; vendor strategy necessary |
| No Button `destructive` | True |
| MemoryRow is `<button>` | True (`MemoryRow.svelte:19`) |
| Create has no visibility field | True (`engram.proto:104–115`) |
| Schedule has no id | True |
| Reseal skips errors | True |
| CSRF cookie/header names | True |
| Full OIDC login redirects off SPA | True |
| Foreign write → NotFound | True (not PermissionDenied) |
| TS Visibility enum members | **Plan text often wrong** (`SHARED` not `VISIBILITY_SHARED`) |
| shared + schedule composite | **Missing / HIGH** |
| openEdit data plumbing | **Incomplete / MEDIUM** |

Phase goal (REQ-console-write-ux): **achievable** after fixing Visibility symbols + schedule+shared composite + tightening edit/share host data.

---

## Priority fix list (before execute)

1. **HIGH** — Plan 04/05: `Visibility.SHARED` / `.PRIVATE`; never Go-style `VISIBILITY_*` in TS.
2. **HIGH** — Plan 04/05: schedule create with `shared` → Schedule then SetVisibility (same partial failure as store).
3. **MEDIUM** — Plan 03/06: pass `Memory` into edit/share (drop invent `recordFor`).
4. **MEDIUM** — Plan 05: reopen sheet when sessionStorage draft restores after `/auth/login`.
5. **MEDIUM** — Plan 01: prove `buf generate` emits `gen/ts/buf/validate/validate_pb.ts` before locking CI.

---

## Risk Assessment (phase)

| Dimension | Level | Why |
|-----------|-------|-----|
| Correctness vs proto/store | **MEDIUM** | Composite + enums mostly right; schedule+shared hole |
| Security (CSRF / retry / double write) | **LOW** | Grounded in real interceptors; NotFound for foreign writes |
| Ship / embed | **LOW** | 19-06 Task 3 + ui-drift align with repo |
| Dependency order | **LOW** | Waves match compile reality (01 → 02/03 → 04 → 05 → 06) |
| Over-engineering | **LOW** | sessionStorage is required; host component appropriate |
| Residual HIGH | 2 localized exec bugs | Fixable without re-architecture |

**Overall: MEDIUM** — plans are execution-ready after the HIGH sheet fixes; do not treat `<review_incorporation>` as fully closed until Visibility names and schedule+shared are corrected in PLAN text and acceptance tests.

---

## Antigravity Review

# Cross-AI Plan Review: Phase 19 — Console Write UX (Round 2)

**Status:** **PASSED / APPROVED FOR EXECUTION**
**Confidence:** **HIGH** (100% of claimed fixes in `<review_incorporation>` verified against actual source code, proto schemas, and CI workflows in this git working tree).

---

## 1. Executive Summary

Phase 19 plans (`19-01-PLAN.md` through `19-06-PLAN.md`) have undergone a thorough Round 2 revision. The claimed fixes and architecture choices were verified directly against the `engram` repository source:

1. **Codegen & Directory Integrity (19-01):** Verified that `gen/ts/engram/v1/engram_pb.ts` imports `../../buf/validate/validate_pb`. The structure-preserving vendor task and compilation gate (`pnpm check`) eliminate the flat-copy compilation bug.
2. **Accurate Server Re-seal Framing (19-02):** Verified in `internal/server/connectreseal.go:39` that `newConnectResealInterceptor` skips re-sealing on error (`if err != nil || resp == nil ...`). The transport retry is correctly framed as an opportunistic race retry for cookie freshness rather than a server-triggered re-seal.
3. **UI Component Integrity (19-03):** Verified in `ui/src/lib/components/MemoryRow.svelte` that the root was `<button>`, which made nested dropdown triggers invalid interactive DOM. The restructuring to a non-button container solves event bubbling and accessibility. Added the missing `destructive` Button variant in `button.svelte`.
4. **Proto Protocol Fidelity (19-04):** Verified in `proto/engram/v1/engram.proto` that `StoreMemoryRequest` and `StoreDiscoveryRequest` carry **no `visibility` field**, and `ScheduleMemoryRequest` has **no `id` field**. The create-as-shared two-step composite (`Store*` $\to$ `SetVisibility`) and create-only schedule constraints accurately model the backend wire contracts.
5. **OIDC Navigation & Input Preservation (19-05):** Recognized that a full `/auth/login` redirect destroys in-memory Svelte `$state`. Adding a lightweight `sessionStorage` draft persistence before the re-auth navigation guarantees `D-09` ("resubmit without losing input") across OIDC round-trips.
6. **Embedded Bundle Delivery (19-06):** Verified that `.github/workflows/ci.yaml:138` checks `ui-drift` against `internal/webauth/static/`. Running `task ui:build` and committing the output ensures the shipped Go binary carries the frontend write UX.

---

## 2. Source-Verified Audit of `<review_incorporation>` Claims

| Claimed Fix / Finding | Verification Source File | Actual Code Finding | Verdict |
| :--- | :--- | :--- | :--- |
| **Buf validate TS dep missing** | `gen/ts/engram/v1/engram_pb.ts#L12` | `import { file_buf_validate_validate } from "../../buf/validate/validate_pb";` exists. Flat copy breaks; structure-preserving vendor + `buf/validate` stub required. | **VERIFIED CORRECT** |
| **Missing `destructive` Button variant** | [button.svelte](file:///Volumes/Code/github.com/seanb4t/engram/ui/src/lib/components/ui/button/button.svelte#L8-L13) | `buttonVariants` contained only `default`, `outline`, `ghost`, `surface`. CSS token alone was insufficient. | **VERIFIED CORRECT** |
| **Server re-seal on error impossible** | `internal/server/connectreseal.go#L39-L42` | `if err != nil \|\| resp == nil \|\| reseal == nil { return resp, err }`. Errored requests never re-seal. | **VERIFIED CORRECT** |
| **Invalid nested interactive DOM** | [MemoryRow.svelte](file:///Volumes/Code/github.com/seanb4t/engram/ui/src/lib/components/MemoryRow.svelte#L19) | Root element was `<button type="button">`. Kebab dropdown trigger inside it created invalid nested buttons. | **VERIFIED CORRECT** |
| **`StoreMemoryRequest` lacks `visibility`** | `proto/engram/v1/engram.proto#L104-L115` | Request fields end at `summary`. No `visibility` field exists on creation; two-step composite required. | **VERIFIED CORRECT** |
| **`ScheduleMemoryRequest` lacks `id`** | `proto/engram/v1/engram.proto#L203-L215` | Scheduled memories are create-only store requests. Editing cannot route to `ScheduleMemory`. | **VERIFIED CORRECT** |
| **CI SPA drift gate failure risk** | `.github/workflows/ci.yaml#L138-L160` | `ui-drift` job rebuilds SPA and executes `git diff --exit-code -- internal/webauth/static/`. | **VERIFIED CORRECT** |

---

## 3. Comprehensive Phase Plan Review

### Wave 1 — Foundation (Plan `19-01`)
* **Scope:** Re-vendor TS Connect client, wire CI drift guards, add CSS tokens and `destructive` button variant.
* **Assessment:** Outstanding. By forcing a directory structure copy (`cp -R gen/ts/. ui/src/lib/gen/`) and including `--include-imports` in `buf generate`, the missing `buf/validate/validate_pb` dependency is vendored alongside `engram_pb.ts`. The compilation gate (`cd ui && pnpm check`) guarantees all imports resolve before subsequent plans run.

### Wave 2 — Transport & Destructive Affordances (Plans `19-02` & `19-03`)
* **Scope:** `attachCsrf` & `retryOnce` transport interceptors, `DeleteConfirmDialog`, `ShareWarningInline`, restructured `MemoryRow`, `MemoryList`, `MemoryDetail`.
* **Assessment:** Highly sound design.
  * `[retryOnce, attachCsrf]` interceptor ordering correctly leverages Connect-ES outer-to-inner execution so retried requests re-execute `attachCsrf` and pick up fresh `document.cookie` values.
  * Restructuring `MemoryRow.svelte` to a `<div>` root while keeping selection controls and the kebab dropdown as siblings fixes browser accessibility violations.
  * `ShareWarningInline` copy accurately notes that while visibility can be changed later, prior disclosures to authenticated callers cannot be un-read.

### Wave 3 — Mutation Hooks (Plan `19-04`)
* **Scope:** Svelte-query v6 mutation hooks in `memory.ts` and `discovery.ts`.
* **Assessment:** Full protocol adherence.
  * `createMutation(() => ({...}))` thunk syntax strictly satisfies Svelte 5 rune reactivity rules.
  * Field masks for updates (`UpdateMemoryRequest`) are correctly constructed via `create(FieldMaskSchema, { paths: [...] })`.
  * Optimistic UI changes snapshot multi-key caches (`getQueriesData`) and roll back gracefully (`setQueriesData`) without losing previous state.
  * Extracted pure cache transform factories enable unit testing against TanStack Query without requiring a Svelte component context.

### Wave 4 — Form Surfaces & Resilience (Plan `19-05`)
* **Scope:** `MemoryFormSheet.svelte` and `DiscoveryFormSheet.svelte`.
* **Assessment:** Exceptional UX resilience.
  * Disabling immutable fields (`scope` and `category`) during edit mode matches `UpdateMemoryRequest` wire rules.
  * The two-tier input preservation mechanism (in-SPA form holding state for transient retries, combined with `sessionStorage` draft persistence for OIDC full-page redirects) fully satisfies `SC3` / `D-09` without data loss.
  * Discovery creation enforces content, typed `Citation[]` messages (`kind` $\in$ `file|commit|url|repo` + `ref`), and `discovery:` scope prefixes.

### Wave 5 — Route Integration & Binary Distribution (Plan `19-06`)
* **Scope:** `WriteSurfaces.svelte` host component, route integration across `observe`, `search`, and `discovery`, and `task ui:build` SPA compilation.
* **Assessment:** Clean orchestration.
  * Explicit `bind:this` instance methods (`openCreate`, `openEdit`, `requestDelete`, `requestShare`) provide a typed, unambiguous API for routes.
  * Omitting `onedit` on the discovery route strictly respects the `D-04` boundary (discovery editing deferred).
  * Executing `task ui:build` to commit `internal/webauth/static/` ensures the embedded binary shipped to operators contains the production write UX.

---

## 4. Final Recommendation & Readiness

Plans `19-01-PLAN.md` through `19-06-PLAN.md` are **complete, robust, and fully verified against the codebase**. 

**Action:** PROCEED WITH EXECUTION.

---

## Consensus Summary

**Round-2 verdict: NOT converged — one more revision pass warranted.** The round-1 replan successfully fixed all 7 round-1 HIGHs (verified landed by all three reviewers), but round 2 surfaces new/residual blockers introduced or left open by that replan. Codex (highest-signal, 6 blockers) and grok-4.5 (MEDIUM residual) converge on two clear compile/logic defects; antigravity approved but missed both. Base the plan-level verdict on the grounded pair.

### Agreed Strengths (2+ reviewers — round-1 fixes confirmed landed)
- **Gen re-vendor is structure+compile, not flat copy** — all 3 verified `gen/ts/engram/v1/engram_pb.ts:12` imports `../../buf/validate/validate_pb`; structure-preserving vendor + real `pnpm check` gate is correct.
- **Real `destructive` Button variant** added to `button.svelte:8` (only 4 variants shipped), not a token alone — all 3.
- **Non-button MemoryRow shell** fixes invalid nested interactive DOM (`MemoryRow.svelte:19` was `<button>`) — all 3.
- **create→SetVisibility composite** correctly models that `StoreMemoryRequest` carries no visibility field (`engram.proto:104`) — all 3.
- **Typed discovery citations** match schema (`engram.proto:121`) + server validation (`tools.go:575`) — codex + agy.
- **Schedule is create-only** (ScheduleMemoryRequest has no id) — all 3.
- **`task ui:build` → commit `internal/webauth/static/` → CI drift gate** (`ci.yaml:155`) closes the stale-binary gap — all 3.
- **Interceptor order `[retryOnce, attachCsrf]`** correct; double-create still mitigated pre-handler — codex + grok.

### Agreed Concerns (2+ grounded reviewers — HIGHEST PRIORITY, must fix before execute)
- **CONVERGENT HIGH #1 — Visibility enum member names are wrong (compile break).** Plan 04/05 task text repeatedly uses `VISIBILITY_PRIVATE` / `VISIBILITY_SHARED`, but the generated ES enum exposes `Visibility.PRIVATE` / `Visibility.SHARED` (`gen/ts/engram/v1/engram_pb.ts:927`). Codex + grok. Replace every reference.
- **CONVERGENT HIGH #2 — scheduled-and-shared creation silently loses the share.** The create→SetVisibility composite was given to `useCreateMemory` only; `useScheduleMemory` neither accepts `shared` nor performs the second `SetVisibility` call (`engram.proto:203` — ScheduleMemoryRequest has no visibility field). A memory created as "scheduled + shared" lands PRIVATE. Codex + grok. Give `useScheduleMemory` the same composite + partial-failure behavior + a scheduled+shared test.
- **Visibility is one-directional** — no "Make private" / unshare path for already-shared records, especially discoveries (no discovery edit form). Codex (MED, both 19-03 and 19-06); grok notes the same asymmetry. Note this sits against locked D-07's one-way framing — resolve as a scope clarification, not necessarily a new feature.

### Divergent Views (worth investigating before deciding severity)
- **Overall risk: Codex HIGH / revision-required (6 blockers) vs grok MEDIUM (HIGHs localized, fixable before Wave 3) vs antigravity PASSED/APPROVED.** Antigravity missed both convergent HIGHs → its verdict is optimistic; do not treat APPROVED as clearing them.
- **SC3 / D-08 retry — Codex HIGH, grok/agy accept.** Codex: the "opportunistic retry" recast is a rename, not a fix — SC3 is still unachievable because failed requests are never resealed (`connectreseal.go:39`), an expired session is rejected pre-handler (`resolver.go:49`), and there is no "needs rotation" error state; wants SC3/D-08 amended to promise a single opportunistic auth-race retry (never "retry through reseal"), with tests renamed "auth-race retry." Codex also flags the CSRF-freshness race as largely theoretical (token is owner-bound and stable across reseals — `csrf.go:38`, `reseal.go:41`). grok rated the recast "honest post-round-1" and lower severity. **This is the key call for the next replan: adjust SC3/D-08 wording to match backend reality rather than re-litigate the mechanism.**
- **sessionStorage re-auth draft — Codex HIGH, grok/agy accept as landed.** Codex: the draft is not wired to the actual OIDC landing — auth always returns to `/ui/` (`handlers.go:187`), which has no write host (`+page.svelte`), and Plan 06 only adds hosts to observe/search/discovery, so no sheet necessarily remounts; edit drafts also can't reconstruct edit mode (no record prop after callback). Wants a resume envelope (return URL + kind + mode + record id + values) consumed on `/ui/` startup, with a route-level landing test, not just component unmount/remount. grok/agy accepted the draft as a real improvement without flagging the routing gap.
- **Codex-unique HIGHs (grok + agy missed):**
  - **Row edit prefills summary-shaped data → data loss.** List/search omit `full`; the server clears content when `full=false` (`connectapi.go:70`), so `openEdit(recordFor(id))` can prefill EMPTY content and overwrite the real body on save. Fix: `openEdit(id)` → fetch `GetMemory` (full content, `connectapi.go:202`) before opening the sheet.
  - **`requestShare(id, kind)` / `onshare(id)` ID-only API can't know or enforce visibility** — the "already shared is a no-op" acceptance has no data source. Replace with a visibility-aware, bidirectional contract (`onvisibility(record, target)` / `requestVisibility(record, 'shared'|'private')`).
- **Lower-severity, grounded (mostly Codex):** destructive foreground contrast — white text on dark-mode `#ffa657` (`app.css:28`) is weak, set `--destructive-foreground` per theme; prefer plugin-scoped `include_imports: true` on the ES plugin (`buf.gen.yaml:21`) over global `--include-imports` (grok flags the same `--include-imports` path as unproven — verify `git diff gen/go/` stays empty); filtered-cache membership — visibility is in `listMemoriesKey` (`queries.ts:34`) so a visibility change leaves records in incompatible cached lists until refetch, and delete should update `total`; dirty-field diffing — unchanged content still forces re-embed (`tools.go:1003`) and an unchanged auto-summary flips provenance to client-authored (`store.go:1404`) → require true dirty-mask + disable Save when mask empty.

### Reviewer signal assessment (this round)
- **Codex (codex-cli 0.144.1): highest signal.** 6 grounded blockers, several unique (row-edit data loss, sessionStorage routing gap, SC3 backend reality), all with `file:line` evidence. Consistent with its round-1 dominance.
- **OpenCode/grok-4.5: grounded, complementary.** Converged with Codex on the 2 clearest blockers (enum naming, scheduled+shared); rated overall MEDIUM. Confirmed round-1 fixes with source citations.
- **Antigravity (agy 1.1.2): improved but optimistic.** Genuinely source-grounded this round (verified round-1 fixes with `file:line`, no round-1-style inversion) — a real step up — but returned APPROVED/HIGH-confidence while missing both convergent HIGHs. Treat as a soundness cross-check on the landed fixes, not as a clearing verdict.

### Recommended next step
`/gsd-plan-phase 19 --reviews` to incorporate round 2. Minimum blocking set for the next pass: (1) enum member names `Visibility.PRIVATE`/`Visibility.SHARED`; (2) `useScheduleMemory` shared composite + test; (3) amend SC3/D-08 to the honest opportunistic auth-race retry; (4) route-/mode-aware re-auth resume envelope wired to the `/ui/` landing; (5) `openEdit(id)` → GetMemory full-content fetch before edit; (6) visibility-aware bidirectional share/make-private API.
