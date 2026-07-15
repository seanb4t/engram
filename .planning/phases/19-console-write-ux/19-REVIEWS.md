---
phase: 19
reviewers: [codex, opencode, antigravity]
reviewed_at: 2026-07-15T13:02:51Z
review_round: 5
reviewed_commit: 7bc5ea81
opencode_model: openrouter/x-ai/grok-4.5
codex_cli: 0.144.1
agy_cli: 1.1.2
plans_reviewed: [19-01-PLAN.md, 19-02-PLAN.md, 19-03-PLAN.md, 19-04-PLAN.md, 19-05-PLAN.md, 19-06-PLAN.md]
---

# Cross-AI Plan Review — Phase 19 (Round 5 — Final Convergence Confirmation)

Round 5 review of the round-4 plans (commit `6734d4b4`) + finished CONTEXT reconciliation (`7bc5ea81`) against live source. grok-4.5 and antigravity both rate the phase EXECUTE-READY with zero blockers (LOW-only). Codex rates it HIGH / not-execute-ready on ONE new, localized finding: the round-4 delete/share SC3 fix introduced a cross-plan contradiction — 19-03's DeleteConfirmDialog closes on confirm, while 19-06 requires it to stay open to show the retain-target + re-auth CTA on terminal auth failure. All other round-4 fixes verified holding by all three. Codex blocker trend: HIGH 7 → 6 → 3 → 1 → (incorporated) → 1 (new self-introduced seam).

---

## Codex Review

# Summary

The plans are close, but the phase is **not yet execute-ready**. The round-4 fixes for edit visibility, resume typing/ownership, and upstream auth-race wording hold. However, the inline-delete SC3 fix contains a direct cross-plan contradiction: `DeleteConfirmDialog` closes immediately after invoking `onconfirm`, while `WriteSurfaces` requires it to remain open after terminal auth failure to display the re-auth CTA.

## Plan assessments

| Plan | Assessment | Verdict |
|---|---|---|
| 19-01 | Structure-preserving generated-client vendoring, compile gate, drift guard, and destructive token/variant are well scoped. The canonical client really imports the missing `buf/validate` dependency ([engram_pb.ts:12](/Volumes/Code/github.com/seanb4t/engram/gen/ts/engram/v1/engram_pb.ts:12)), while current generation and CI only cover `gen/` ([Taskfile.yaml:145](/Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml:145), [ci.yaml:127](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/ci.yaml:127)). Plugin-scoped `include_imports` is supported by the installed Buf v1.71 parser. | Execute-ready |
| 19-02 | Retry semantics match the backend: errored responses skip resealing ([connectreseal.go:39](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectreseal.go:39)), expired sessions fail before the handler ([resolver.go:49](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/resolver.go:49)), and CSRF tokens remain owner-bound across reseals ([csrf.go:38](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/csrf.go:38)). The composed-interceptor test closes the important ordering seam. | Execute-ready |
| 19-03 | Non-button row restructuring, rule suppression, per-callback gating, visibility-aware sharing, and accurate sharing copy are sound. Its delete-dialog close contract conflicts with Plan 06, however. | **Not execute-ready** |
| 19-04 | RPC shapes follow the actual proto: Store/Schedule have no visibility field ([engram.proto:104](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:104), [engram.proto:203](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:203)); Update uses a FieldMask ([engram.proto:164](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:164)); generated enum members are `PRIVATE`/`SHARED` ([engram_pb.ts:927](/Volumes/Code/github.com/seanb4t/engram/gen/ts/engram/v1/engram_pb.ts:927)). The discriminated partial-success state machine prevents duplicate creates. | Execute-ready |
| 19-05 | The round-4 visibility fix holds: shared edit records are explicitly read-only and cannot emit `shared:false` ([19-05-PLAN.md:101](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-05-PLAN.md:101), [19-05-PLAN.md:121](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-05-PLAN.md:121)). `ResumeDraft = Omit<ResumeEnvelope,'v'|'ts'>` and `persistResume(draft)` are now type-consistent ([19-05-PLAN.md:89](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-05-PLAN.md:89)). | Execute-ready |
| 19-06 | Full-record edit fetching is necessary because list/search shaping clears content ([connectapi.go:70](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:70)). Resume acknowledgment is now exact: `onresumeapplied` is passed through and valid envelopes are consumed only after application ([19-06-PLAN.md:107](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-06-PLAN.md:107)). Embedded-SPA rebuild/drift gates are correct ([Taskfile.yaml:21](/Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml:21), [ci.yaml:155](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/ci.yaml:155)). Blocked by the dialog contract below. | **Not execute-ready** |

## Strengths

- Dependency ordering is coherent: generated client → transport/components → mutations → forms → route integration.
- Create-plus-share partial success cannot accidentally replay Store/Schedule.
- Full-content edit fetching avoids a serious empty-content overwrite.
- Resume-envelope ownership is singular and tested across the real `/ui/` callback landing.
- Upstream wording is reconciled: ROADMAP SC3 ([ROADMAP.md:408](/Volumes/Code/github.com/seanb4t/engram/.planning/ROADMAP.md:408)), the requirement ([REQUIREMENTS.md:53](/Volumes/Code/github.com/seanb4t/engram/.planning/REQUIREMENTS.md:53)), and D-08/D-09 ([19-CONTEXT.md:68](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-CONTEXT.md:68)) consistently describe an opportunistic auth-race retry, not reseal-on-retry.

## Concerns

### HIGH — Delete confirmation closes before terminal-auth state can be displayed

Plan 19-03 explicitly says clicking Delete calls `onconfirm()` **then closes the dialog** ([19-03-PLAN.md:83](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-03-PLAN.md:83)). Plan 19-06 simultaneously requires `onconfirm` to await the mutation, retain `deleteTarget` on terminal auth failure, keep the dialog open, and display `authFailure` ([19-06-PLAN.md:101](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-06-PLAN.md:101), [19-06-PLAN.md:111](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/19-console-write-ux/19-06-PLAN.md:111)).

As written, the dialog closes before the asynchronous failure arrives, so the retained target and re-auth CTA are hidden. This means round-4 fix (a) does not fully hold for delete.

There is also a related wording risk: clicking Re-authenticate performs a full redirect whose callback always returns to `/ui/` ([handlers.go:187](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/handlers.go:187)). Without preserving the inline action, the claim that it remains available for deliberate retry “after re-auth” is not true across navigation.

## Suggestions

1. Make the host authoritative for dialog closure:

   - `DeleteConfirmDialog` must not close on Delete.
   - `onconfirm` should be awaitable, or simply notify the host.
   - Close only on Cancel or successful mutation.
   - Add a component/host test proving terminal auth leaves the dialog visibly open with the CTA.

2. For the OIDC round trip, either:

   - persist a minimal non-auto-replayed inline-action envelope (`returnPath`, action, kind, id) and reopen the confirmation after re-auth; or
   - narrow the acceptance wording so retention is promised only until navigation, not after re-auth.

## Risk assessment

**Overall risk: HIGH — not execute-ready.**

The remaining defect is localized to Plans 19-03/19-06, and the other round-4 fixes hold. Once the delete-dialog ownership contradiction—and preferably the post-redirect inline-action continuity—is resolved, the phase should be ready without another broad redesign.

---

## OpenCode Review (grok-4.5 — openrouter/x-ai/grok-4.5)

Verifying round-4 fixes and key plan claims against the live tree.
# Phase 19 Plan Review — Round 5 (Final Convergence)

## Summary

All six plans hold up against live source. Round-4 fixes are real, not papered over: SC3 for inline delete/share is fully specified (retain target + re-auth CTA, no auto-replay); edit mode blocks accidental unshare; resume ownership/`ResumeDraft` are exact; ROADMAP SC3, REQ-console-write-ux, and D-08/D-09 all agree on **opportunistic auth-race retry** + **sessionStorage resume**. Backend facts the plans depend on match code. **Verdict: execute-ready.**

## Strengths

- **Wave graph** is clean: 01 → (02 ‖ 03) → 04 → 05 → 06, with 02/03 correctly decoupling transport from presentational affordances.
- **Hard prerequisites (19-01)** match reality: flat gen is still read-only (`ui/src/lib/gen/engram_pb.ts`); canonical `gen/ts/engram/v1/engram_pb.ts:12` imports `../../buf/validate/validate_pb` and `gen/ts/buf/` is absent — structure-preserving re-vendor + `pnpm check` is required.
- **Retry framing (19-02)** matches backend: reseal skips errors (`connectreseal.go:40`), expired session is reject-not-rotate (`resolver.go:50`), CSRF value is owner-HMAC and stable across reseal (`csrf.go:39`, `reseal.go:43/78`), Cookie/header names match (`csrf.go:22-23`).
- **RPC/form contracts (19-04/05)** match proto: Store/Schedule/Discovery have no visibility field; responses are `{id,short_id}` only; schedule is create-only (no id); discovery needs content/kind/citations/`discovery:` scope (`engram.proto:104–222`, `tools.go:575–608`).
- **List→edit data loss (19-06)** correctly requires GetMemory full fetch: list/search clear content when `full=false` (`connectapi.go:70–79`); GetMemory returns full (`connectapi.go:202–211`).
- **D-07 product choice** is explicit and consistent (one-way private→shared in UI even though `SetVisibility(..., false)` exists at `store.go:1555–1581`).
- **Ship gate** (`task ui:build` → `internal/webauth/static/` + CI ui-drift) closes a real binary-delivery hole.

## Concerns

None blocking.

| Severity | Item | Notes |
|----------|------|--------|
| LOW | Inline delete/share after full OIDC | Target is retained only until `/auth/login`; there is no resume envelope, so intent is lost after callback → `/ui/`. Acceptable (no typed input; id-idempotent re-fire). Do not invent auto-replay. |
| LOW | `include_imports` may emit more than `validate_pb.ts` | 19-01 already lists `gen/ts/buf/**` / full-tree copy + `pnpm check`. |
| LOW | Optional server-contract test “retry codes stay pre-handler” | Filed as follow-up under T-19-11; correctly out of this UI phase. |

## Round-4 verification (source + docs)

| Claim | Evidence | Status |
|-------|----------|--------|
| (a) Inline delete/share honor SC3 (retain + re-auth CTA) | 19-06 Task 1 + 19-03 `authFailure`/`onreauth`; handlers skip reseal on error (`connectreseal.go:40`) | **Holds** |
| (b) Shared-record edit: visibility read-only; never `shared:false` | 19-05 Task 1; UpdateMemory allows `shared` in mask (`engram.proto:164–172`) so the lock is necessary | **Holds** |
| (c) `onresumeapplied` sole `consumeResume` trigger; `persistResume(ResumeDraft)` | 19-05/06 exact peeks at root, consume after form ack; Callback → `/ui/` (`handlers.go:187`); base `/ui` (`svelte.config.js:9`) | **Holds** |
| (d) Upstream SC3/D-08/D-09 = auth-race, not reseal-on-retry | ROADMAP Phase 19 SC3; REQUIREMENTS REQ-console-write-ux; CONTEXT D-08/D-09 + deferred→resume note | **Holds** |
| Vendored gen missing writes | Vendored service methods stop at SearchDiscoveries; gen has store/delete/setVisibility/schedule | **Holds** |
| MemoryRow is a `<button>` (button-in-button risk) | `MemoryRow.svelte:19` | **Holds** (19-03 restructure) |
| No `destructive` Button variant | `button.svelte` variants: default/outline/ghost/surface only; no `--destructive` in `app.css` | **Holds** (19-01 adds both) |
| Visibility enum in TS | `Visibility.PRIVATE`/`SHARED` at `engram_pb.ts:927–941` (not `VISIBILITY_*`) | **Holds** |
| CSRF wire names | `engram_csrf` / `X-CSRF-Token` (`csrf.go:22–23`) | **Holds** |

## Suggestions

1. During 19-02, keep test names / comments strictly **“auth-race retry”** — code and docs already forbid “rotation recovery.”
2. At 19-06 UAT, manually hit: create-as-shared (two RPCs), schedule+shared, form re-auth with resume on all three routes, and terminal-auth on delete/share (**re-open action after OIDC is expected**).
3. Optional post-phase issue: server test that Unauthenticated/PermissionDenied never escape write handlers after mutation (T-19-11 follow-up). Not a phase gate.

## Risk Assessment

**Overall risk: LOW.** Phase is frontend-only on a shipped write lane; plans encode the hard proto/cache/auth edges that prior reviews found; upstream docs are reconciled.

**Execute-ready: YES.** Proceed with Wave 1 (`19-01`) when ready.

---

## Antigravity Review

# Cross-AI Plan Review — Round 5 (Final Convergence Confirmation)

## Status: EXECUTE-READY 🚀

All plans (19-01 through 19-06) for **Phase 19: Console Write UX** have successfully converged. The round-4 fixes have been verified against the actual repository source code and planning artifacts. There are no remaining blockers, contradictions, or unhandled edge cases. Phase 19 is ready for execution.

---

## Round-4 Verification Summary

| Item / Claim | Verified Location | Status |
| :--- | :--- | :--- |
| **Inline Delete/Share SC3 Re-Auth CTA** | `19-03-PLAN.md` (Task 1) & `19-06-PLAN.md` (Task 1, lines 103, 119) | **CONFIRMED** — `DeleteConfirmDialog` & `ShareWarningInline` expose optional `authFailure`/`onreauth` props. `WriteSurfaces` retains the target and surfaces the inline re-auth prompt on terminal `Unauthenticated`/`PermissionDenied` errors without auto-replay. |
| **Plan 01 Manifest `buf/**` Scope** | `19-01-PLAN.md` (`files_modified`) | **CONFIRMED** — `gen/ts/buf/**` and `ui/src/lib/gen/buf/**` are explicitly listed in `files_modified`. |
| **Plan 02 Composed Interceptor Test** | `19-02-PLAN.md` (Task 3) | **CONFIRMED** — Added a composed `[retryOnce, attachCsrf]` test asserting that cookie mutations during a failed first attempt are re-read by the header generator on retry. |
| **Plan 04 Named Cache Factories & Single-Toast** | `19-04-PLAN.md` (Tasks 1 & 2) | **CONFIRMED** — Pure cache factories are exported by name for Svelte-context-free node testing, and `onSuccess` status switching strictly guarantees a single toast per write operation. |
| **Upstream Docs Reconciliation** | `ROADMAP.md`, `REQUIREMENTS.md`, `19-CONTEXT.md` | **CONFIRMED** — All upstream references explicitly match the "opportunistic auth-race retry" framing. |

---

## Summary of Work Done

- Verified git repository state (`phase-19-console-write-ux` branch).
- Inspected `.planning/ROADMAP.md`, `.planning/REQUIREMENTS.md`, and `.planning/phases/19-console-write-ux/` artifacts.
- Confirmed that all round-4 feedback items have been correctly incorporated across all 6 plans and upstream documents.
- Evaluated total phase completeness: Phase 19 is fully scoped, technically sound, and execute-ready.

---

## Consensus Summary

**Round-5 verdict: 2 of 3 execute-ready; Codex found one localized HIGH.** grok-4.5 ("None blocking") and antigravity ("EXECUTE-READY 🚀") both confirm all six plans hold against live source with zero blockers. Codex confirms all the round-4 fixes hold too — except it caught a genuine cross-plan contradiction that the round-4 delete/share SC3 fix introduced. This is the same tail pattern as round 4: closing one gap opened a small new seam, and only the highest-signal reviewer traced it.

### Verified holding in source (all 3 reviewers — round-4 fixes confirmed)
- **Inline share honors SC3** (retain target + re-auth CTA via `ShareWarningInline` `authFailure`/`onreauth`); **edit-mode visibility read-only for shared records** (never emits `shared:false`, honoring one-way D-07); **`onresumeapplied` sole `consumeResume` trigger** + `persistResume(draft: ResumeDraft)` type-consistent; **upstream SC3/D-08/D-09 reconciled** to opportunistic auth-race retry (ROADMAP:408, REQUIREMENTS:53, CONTEXT D-08/D-09 + the previously-stale passages). All prior-round fixes (composite state machine, gen re-vendor, destructive variant, non-button MemoryRow, GetMemory edit fetch, ship gate, interceptor order, etc.) confirmed intact.

### Agreed: no blocker EXCEPT the delete-dialog contract
- grok-4.5 and antigravity: zero blockers.
- Codex: one HIGH (below). It is localized to Plans 19-03/19-06 — "the other round-4 fixes hold ... without another broad redesign."

### Codex HIGH — the one genuine remaining defect (a round-4-introduced self-contradiction)
- **`DeleteConfirmDialog` closes before the terminal-auth state can be displayed.** Plan 19-03 says clicking Delete calls `onconfirm()` then **closes the dialog** (`19-03-PLAN.md:83`). Plan 19-06 simultaneously requires `onconfirm` to await the mutation, **retain `deleteTarget` on terminal auth failure, keep the dialog open, and show `authFailure`** (`19-06-PLAN.md:101,111`). As written the dialog closes before the async failure arrives, so the retain-target + re-auth CTA are hidden → round-4 fix (a) does NOT hold for **delete** (it does hold for share, which uses the inline `ShareWarningInline`, not a self-closing dialog). Fix: make the host authoritative for dialog closure — `DeleteConfirmDialog` must not close on Delete; `onconfirm` is awaitable / notifies the host; close only on Cancel or successful mutation; add a test proving terminal auth leaves the dialog open with the CTA.
- **Related continuity nuance (Codex + grok both touch it):** the re-auth CTA triggers a full OIDC redirect whose callback returns to `/ui/` (`handlers.go:187`), and the inline delete/share action is NOT persisted in a resume envelope — so the pending action is lost after the round trip. Codex frames this as a wording risk ("available for deliberate retry *after re-auth*" isn't true across navigation); grok rates it LOW-acceptable (no typed input, id-idempotent, "do not invent auto-replay"). Resolution is a scope decision: either narrow the acceptance wording to "retained until navigation" OR persist a minimal non-auto-replay inline-action envelope (`returnPath`, action, kind, id) and reopen the confirmation after re-auth.

### grok-4.5 LOW (all non-blocking, correctly scoped)
- Inline delete/share intent lost after a full OIDC round trip (acceptable — no typed input, id-idempotent re-fire; do not add auto-replay). `include_imports` may emit more than `validate_pb.ts` (19-01 already lists `gen/ts/buf/**` + full-tree copy + `pnpm check`). Optional server-contract "retry codes stay pre-handler" test (filed as follow-up under T-19-11, correctly out of this UI phase).

### Divergent Views
- **grok-4.5 + antigravity: EXECUTE-READY, zero blockers. Codex: HIGH, not execute-ready.** As in round 4, the split is real coverage, not model pessimism: Codex traced the `DeleteConfirmDialog` close-timing against the 19-06 keep-open requirement, a cross-plan contradiction the other two did not drill into. It is a true plan-level inconsistency (two plans specify incompatible dialog behavior) — the kind of thing that belongs resolved in the plan, not left for the executor to guess.
- **Antigravity:** shortest, cleanest EXECUTE-READY yet; source-grounded on the landed fixes; consistent optimistic pattern — soundness cross-check, not a clearing verdict.

### Convergence assessment
Extremely close. Every fundamental and every prior-round fix is verified sound; the sole remaining defect is a localized dialog-ownership contradiction in 19-03/19-06 plus a one-line scope decision on post-redirect continuity. No architectural work remains.

### Recommended next step
1. **One final targeted `/gsd-plan-phase 19 --reviews` pass** (recommended) — make the host authoritative for `DeleteConfirmDialog` closure (don't close on confirm; close only on Cancel/success; keep open + CTA on terminal auth; add the test), and pick the OIDC-continuity resolution (narrow wording, or a minimal inline-action envelope). Surgical, confined to 19-03/19-06.
2. **Execute now** — 2 of 3 reviewers say ready; the dialog-close contradiction becomes an executor reconciliation (the executor would have to pick the keep-open behavior). Accepts one known cross-plan inconsistency going into execution.
