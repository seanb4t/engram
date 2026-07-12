---
phase: 17
reviewers: [codex, antigravity]
reviewed_at: 2026-07-12T14:38:05Z
plans_reviewed: [17-01-PLAN.md, 17-02-PLAN.md, 17-03-PLAN.md, 17-04-PLAN.md, 17-05-PLAN.md, 17-06-PLAN.md]
review_round: 2
supersedes_round: 1
---

# Cross-AI Plan Review — Phase 17 (Round 2)

Reviewers: **Codex** (codex-cli 0.144.1, model `gpt-5.6-sol`) and **Antigravity** (agy 1.1.1, gemini). This is the **second** review round — it re-reviews the six plans (17-01…17-06) after they were revised to incorporate round-1 findings. Both ran inside the git working tree with source-grounding instructions (verify plan claims against actual code, cite file:line). The plans are not yet executed, so the cited code reflects the pre-change state the plans intend to modify.

> **Antigravity truncation caveat (important):** the combined prompt (225 KB / ~55 K tokens, six full plans) exceeded Antigravity's model context, so agy truncated the tail of the prompt. As a result its "**17-06 is missing**" and "**17-05 is truncated**" flags are **artifacts of agy's own truncation, not real gaps** — both plans were present in the prompt (17-06 at prompt line 2203, 17-05 immediately before it) and are committed on disk. Antigravity's read-lane "ownership conflict" concern likewise stems from not having seen 17-06. Treat Antigravity's verdict on the write-lane foundations (17-01…17-04) as grounded, but discount its 17-05/17-06 completeness claims. Codex read the source files directly (agentic file access) and did not have this blind spot.

---

## Codex Review

## Summary

The revised plans are substantially stronger: they address the previous interface, vector-preservation, response-shaping, timestamp, and read-convergence findings with source-aligned designs. However, they are not yet safe to execute as written. There are two compile/test blockers, an unresolved owner-namespace migration/session hazard, incomplete Connect error taxonomy, and a parity-test design that can produce order-dependent false failures or false confidence. Overall, the architecture is sound, but another focused revision is warranted before implementation.

## Strengths

- The `memStore` extraction correctly includes the previously missed production calls. `delete_all` invokes `d.st.DeleteAll` directly at `internal/server/tools.go:1143`, while `ListScopes` calls `a.d.st.ListScopes` at `internal/server/connectapi.go:93`. Including both avoids the earlier interface-carve compile failure.

- A separate vector-preserving payload update is the right mechanism. The existing `Store.Update` always changes content, increments usage, and performs an `Upsert` with a supplied vector at `internal/store/store.go:1379-1406`; `SetVisibility` demonstrates the correct `SetPayload` pattern that preserves vectors at `internal/store/store.go:1437-1441`.

- Making `updateArgs.Content` optional is necessary. It is currently an unconditional string at `internal/server/tools.go:507-513`, and `updateMemory` embeds and persists it unconditionally at `internal/server/tools.go:957-980`. The proposed `*string` closes the tags/shared-only content-blanking bug.

- Returning canonical mutation results from the fetched record is well grounded. `UpdateMemoryResponse` and `SetVisibilityResponse` require both `id` and `short_id` at `proto/engram/v1/engram.proto:173-190`, while the existing by-id methods return only errors. Using the already-fetched `Memory` avoids another lookup.

- RFC3339Nano is the correct precision fix. `parseWindow` compares exact instants for future and strict ordering at `internal/server/tools.go:459-470`; formatting with second-only RFC3339 could collapse distinct sub-second bounds.

- The typed superset read core is preferable to adapting Connect onto the current MCP shape. Connect currently carries offset, categories, visibility, total, and page tokens at `internal/server/connectapi.go:124-150`, while `deps.listMemory` discards total and forces cursor mode at `internal/server/tools.go:809-820`. Plan 17-06 explicitly preserves those fields and removes `[]any` from the shared layer.

- The hardened owner encoding is genuinely injective over `(claim,value)`. A byte-length-prefixed representation can distinguish delimiter-bearing pairs, while keeping verified email owners bare preserves the dominant existing bucket format. The need is security-critical because store filters compare owner strings directly, for example at `internal/store/store.go:788-812`.

- The Connect-cookie actor fallback addresses a real gap: the resolver returns a `TokenInfo` with only `Extra[owner_claim]` at `internal/webauth/resolver.go:44-54`, so `UserID` cannot be assumed present.

## Concerns

- **HIGH — Retyping `deps.st` to `memStore` still breaks existing tests not listed in Plan 17-02.**
  `storeFill` requires a concrete `*store.Store` at `internal/server/summaryqueue.go:119`, but tests pass `d.st` at `internal/server/summaryqueue_test.go:402` and `:422`. After `d.st` becomes `memStore`, those calls no longer compile. Similarly, `buildUsageQueue` accepts `*store.Store` at `internal/server/tools.go:265`, and `internal/server/tools_test.go:1915` passes `d.st`. Plan 17-02 includes `tools_test.go` but omits `summaryqueue_test.go`, so its stated full server test gate cannot pass as written.

- **HIGH — Plan 17-02 attempts to redeclare an existing `errStaleSummary`.**
  The plan's Task 2 says to declare `errStaleSummary` in `identity.go`, but it already exists as an `errors.New` sentinel in `internal/server/summary.go:14-18`, and `resolveSummaryUpdate` already returns that exact sentinel at `internal/server/summary.go:23-35`. Following the plan literally creates a duplicate package-level identifier and fails compilation. It should reuse the existing sentinel and only add the missing rule sentinel.

- **HIGH — The owner encoding rollout lacks migration and existing-session handling.**
  Existing deployments using a non-email owner claim currently store the bare claim value. The new encoding changes that authz key, making existing records invisible unless remapped. More seriously, an already-issued cookie stores only the old bare owner in `webauth.Session` at `internal/webauth/session.go:21-29`; `Resolver.Resolve` trusts and forwards it unchanged at `internal/webauth/resolver.go:44-54`. New logins would receive encoded owners while old live sessions continue using bare owners. This splits one principal across buckets and temporarily preserves the very cross-namespace collision that D-06 is intended to eliminate. No plan addresses cookie invalidation/versioning or `migrate-remap-owner`.

- **HIGH — The proposed parity test runs both lanes against the same mutable fixture.**
  Plan 17-05 says to seed/reset once per table row, then invoke direct `deps.*` and Connect sequentially. That is insufficient. A successful direct `DeleteMemory` removes the record before Connect runs; an update can change the stale-summary precondition; a replace can change the second call's result. Fresh fixtures between rows do not solve mutation between the two lane executions within a row. Each lane needs an independent identically seeded fixture, or the spy/state must be restored between calls.

- **HIGH — `connectError` does not cover several caller-caused semantic errors.**
  The proposed fallback maps all unrecognized errors to `CodeInternal`, but `parseWindow` returns plain errors for a past `not_after` and invalid ordering at `internal/server/tools.go:445-470`. The proto explicitly leaves the wall-clock future check to the handler at `proto/engram/v1/engram.proto:192-198`, so a validly encoded but expired request would incorrectly become Internal. Rule-summary validation also returns plain errors at `internal/server/rules.go:74-85`. These need an `ErrInvalidArgument`-compatible sentinel or explicit typed domain errors before the common mapper is safe.

- **MEDIUM — The payload-only update omits the established update usage signal.**
  `Store.Update` increments `AccessCount` and stamps `LastAccessedAt` on every update at `internal/store/store.go:1379-1384`; this is explicitly pinned by `internal/store/usage_test.go:112-145` and documented as the update signal at `internal/config/config.go:144-150`. Plan 17-02's payload method only mirrors visibility/summary setting. Consequently, Connect shared-only or summary-only updates would no longer count as updates, diverging from the shipped usage-signal contract.

- **MEDIUM — SearchDiscoveries can regress from Connect's default `k=20` to MCP's `k=8`.**
  The current Connect handler defaults discovery search to 20 at `internal/server/connectapi.go:224-228`. The shared `deps.searchDiscovery` defaults to 8 at `internal/server/tools.go:894-915`. Plan 17-06 leaves that internal default in place, while Plan 17-04's read-rewire action explicitly applies `k=20` only to `SearchMemories`, not `SearchDiscoveries`. The high-level must-have says the default is preserved, but the executable task instructions do not.

- **MEDIUM — The spy does not, by itself, prove invocation of the same `deps` method.**
  `memStore` is below `deps`; it records store calls, not which `deps.*` method was entered. This matters because `storeMemory` and `scheduleMemory` both call `MintShortID` and `Upsert` at `internal/server/tools.go:634-694`. A Connect handler duplicating logic or calling the wrong method could produce the same spy trace. The planned source grep against `a.d.st.*` helps, but the claim that the spy structurally proves same-`deps` delegation is overstated.

- **MEDIUM — The "explicit empty ENGRAM_OWNER_CLAIM is rejected" acceptance criterion contradicts the loader.**
  The env transform intentionally drops all empty environment values and preserves defaults at `internal/config/config.go:176-185`; this invariant is tested at `internal/config/config_test.go:34-50`. Therefore `ENGRAM_OWNER_CLAIM=""` resolves to the registry default `email`, not an empty parsed list. Only an explicitly changed empty CLI flag currently reaches the guard. Plan 17-01 cannot satisfy its environment-variable acceptance criterion without deliberately special-casing or changing this established loader behavior.

- **LOW — `connectError(err)` lacks request context for structured logging.**
  The plan requires logging unexpected internal errors, but a context-free mapper cannot use request-scoped trace/log fields. Prefer `connectError(ctx, err)`.

## Suggestions

1. Amend Plan 17-02 to:

   - reuse the existing `errStaleSummary`;
   - add `summaryqueue_test.go` to `files_modified`;
   - either retain a concrete test-store reference alongside `deps.st`, or introduce narrow `summaryStore`/`usageStore` interfaces for `storeFill` and `buildUsageQueue`;
   - include usage-counter fields in the payload-only `SetPayload` operation and test the increment.

2. Treat owner encoding as a migration, not merely a resolver refactor:

   - define the exact old-owner → encoded-owner mapping for existing non-email deployments;
   - document/run `migrate-remap-owner`;
   - invalidate existing UI cookies during rollout, such as by requiring cookie-key rotation, or version the session payload and reject legacy non-email sessions;
   - add a test proving an old bare `sub` cookie cannot coexist with the new namespaced owner space.

3. Introduce typed request/domain errors before wiring `connectError`:

   - wrap `parseWindow` semantic failures with `store.ErrInvalidArgument` or a server `errInvalidArgument`;
   - do likewise for rule-summary validation and other adapter-reachable semantic validation;
   - table-test every mapped write failure, including past `not_after`.

4. Give each parity lane its own independently seeded fixture. Compare:

   - returned code/result;
   - recorded store trace and subject;
   - final state.

   A direct source/AST assertion that each Connect method calls its named `deps` method would be a stronger structural check than claiming the store spy proves this alone.

5. Explicitly apply `k=20` in the Connect `SearchDiscoveries` adapter before calling `deps.searchDiscovery`, and add a regression test analogous to the SearchMemories default test.

6. Revise the owner-claim empty-value test to match the intended contract:

   - either test explicit `--owner-claim=""`, which is observable today;
   - or explicitly approve and test a narrowly scoped exception to the global empty-env-default rule.

## Risk Assessment

**HIGH.** The core architecture now points in the right direction, but the plans remain non-executable as written because of the `memStore` test compile break and duplicate sentinel declaration. More importantly, changing the authorization key without a record/session migration strategy can split principals across buckets or preserve namespace collisions during rollout. The parity and error-mapping gaps could also allow the phase to appear green without actually proving consistent behavior across both lanes. Addressing these items should reduce the implementation risk to medium.

---

## Antigravity Review

> Antigravity wrote its detailed review to an artifact file (`plan_review_feedback.md`) and emitted only a pointer + action-item summary to stdout; the full artifact content is reproduced below. **See the truncation caveat at the top of this document** — agy's "17-06 missing" and "17-05 truncated" flags are artifacts of its own prompt truncation, not real gaps.

# implementation Plan Review: Phase 17 (Wired Write Handlers)

This document provides a structured review of the proposed implementation plans for **Phase 17: Wired Write Handlers (Full CRUD + Schedule)** in the `engram` project. It evaluates the plan's quality, completeness, alignment with project requirements, security postures, and potential execution risks.

## Executive Summary

The overall design is **highly robust** and directly addresses the primary milestone risk: ensuring absolute business-logic and authz parity between the MCP and Connect transports (**REQ-connect-write-authz-parity**). By forcing both transports to route through a single, caller-parameterized `deps.*` layer (rather than letting Connect write directly to the database), the plans prevent code duplication and authz drift.

However, there is **one critical structural gap** and a few coordination risks that must be resolved before proceeding to execution:
1. **Critical Gap**: `17-06-PLAN.md` (Read-lane transport-neutral typed core convergence) is missing from the plan files, though it is referenced in the roadmap as a Wave 2 blocker. *(REVIEWER CAVEAT: false positive — 17-06 was present in the prompt and on disk; this is an agy prompt-truncation artifact.)*
2. **Overlap Risk**: There is a duplicate/unaligned ownership of the read-lane rewiring between the missing `17-06-PLAN.md` and Task 3 of `17-04-PLAN.md`. *(Stems from not seeing 17-06.)*
3. **Truncation**: `17-05-PLAN.md` is truncated, leaving its verification and threat model incomplete in the source document. *(REVIEWER CAVEAT: agy-side truncation, not a real plan gap.)*

### Detailed Plan-by-Plan Analysis

**17-01 (Ordered Owner-Claim List & Hardened Namespacing)** — **Excellent**. Successfully hardens D-06: a naive `claim:value` scheme is vulnerable to collisions (e.g. `("sub","x:y")` and `("sub:x","y")` both serialize to `sub:x:y`); the proposed length-prefixed encoding (`len:claim:len:val`) is mathematically injective. Startup guard correctly rejects an empty list when auth is active while the registry default (`email`) handles the unset case. **Recommendation (IMPORTANT): reserved-namespace email guard** — reject email values matching `^[0-9]+:` so a user with a verified email cannot register an email like `3:sub:3:foo` to hijack another user's `sub` namespace; this guard must run before any string comparison. Update all OIDC-login unit assertions (e.g. `TestTokenVerifierStampsOwnerClaimKey`) in the same commit to avoid CI build failures.

**17-02 (Store Payload-Only Update & Interface Foundations)** — **Excellent**. Decoupling `deps` from concrete `*store.Store` via `memStore` is the load-bearing seam for hermetic unit testing. Payload-only update via Qdrant `SetPayload` is a good optimization (vector reads are expensive; `store.Update` requires a vector). Includes `DeleteAll` and `ListScopes` to keep `tools.go`/`connectapi.go` compiling. **WARNING (Landmine 2):** converting `updateArgs.Content` to `*string` is critical — a plain string would send `""` on a tags-only update and silently delete the memory text.

**17-03 (protoconv)** — **High**. Covers all field-mask, citation, and Visibility mappings. **TIP:** `time.RFC3339Nano` (not `RFC3339`) is critical — RFC3339 truncates fractional seconds and would collapse sub-second scheduling windows, failing `parseWindow` ordering. Verify tests confirm Go's `time.Parse` accepts the nano-formatted strings.

**17-04 (connectError Mapper & Write Handler Wiring)** — **Good**. The scripted-spy fake asserts *delegation* (recording method + subject/args) rather than replicating DB constraints. Single `connectError` mapper returns mapped codes + generic internal messages. **Read-lane ownership conflict:** Task 3 rewires read handlers through `deps.*`, which overlaps the roadmap's Wave-2 17-06 "read-lane typed core convergence" — align whether 17-06 does signature updates and 17-04 wires them, or merge. *(Caveat: overlap concern is a consequence of not seeing 17-06, which scopes exactly this split.)*

**17-05 (Parity & Security Testing)** — **Very High**. The parity test compares MCP and Connect closures directly at the Go-call level to ensure they hit the same `deps.*` method, preventing coincidental-mapping parity. Splitting `short_id` vs UUID inputs is a good existence-leak defense. Ensure the real Qdrant-backed suite (`TestConnectCookieLaneIsolation`) stays active in CI alongside the fake-store unit tests.

### Actionable Recommendations (Antigravity)

1. **Reconstruct or Merge `17-06-PLAN.md`** before Wave 2 — *(void: 17-06 exists; agy truncation)*.
2. **Verify OIDC test coverage**: explicit cases for T-17-03 (unverified email reject), T-17-04 (injective namespace validation), T-17-08 (reserved-namespace email reject).
3. **Document stateless session sliding re-seal limitations** in Phase 18.

### Security & Invariants (Antigravity)

| Feature / Decision | Approach | Quality / Risk | Comment |
|:---|:---|:---|:---|
| D-06 Namespacing | Hardened injective length-prefix | High | Prevents claim collisions (`sub:x:y`) |
| D-07 Read Rewire | Split 17-04 / 17-06 | Coordination Risk | Resolve overlap *(void — 17-06 exists)* |
| D-10 Store Seam | Narrow interface + spy fake | High | Fast tests, asserts exact delegation |
| D-11 Error Leak | Original-input re-wrap | High | Uniform errors hide internal UUIDs |
| Timestamp Wire | `time.RFC3339Nano` | High | Avoids sub-second window truncation |
| Partial Updates | `UpdatePayload` / `*string` | High | Prevents content blanking |

---

## Consensus Summary

Two independent source-grounded reviewers re-reviewed the **revised** six plans (round 2). Both open the referenced Go files and cite `file:line`. **Both agree the round-1 fixes were correct and the architecture is sound.** They diverge on readiness: Antigravity is broadly positive (its only "blockers" were its own prompt-truncation artifacts — 17-06/17-05); **Codex read the source directly and found several concrete NEW issues that survived the round-1 revision, and rates overall risk HIGH ("another focused revision warranted")**. Because Codex did not have Antigravity's truncation blind spot on 17-05/17-06, its findings carry the higher source-grounding weight this round.

### Agreed Strengths (both reviewers, verified against source)

All round-1 fixes are confirmed correct and well-grounded:
- `memStore` now includes `DeleteAll` (`tools.go:1143`) + `ListScopes` (`connectapi.go:93`) — the prior compile blocker is closed.
- Vector-preserving payload-only update is the right mechanism (mirrors the existing `SetVisibility`/`SetPayload` pattern at `store.go:1437-1441`).
- `updateArgs.Content` → `*string` closes the tags/shared-only content-blanking bug (`tools.go:507-513`).
- By-id `mutationResult{id, short_id}` from the already-fetched record satisfies proto responses (`engram.proto:173-190`) without a re-fetch.
- `time.RFC3339Nano` is the correct precision fix (`parseWindow` at `tools.go:459-470`).
- The transport-neutral strongly-typed superset read core (17-06) preserves offset/categories/visibility/total/page-tokens and removes `[]any` from the shared layer.
- The hardened length-prefixed owner encoding is genuinely injective over `(claim, value)` — mathematically prevents the `sub:x:y` collision class.
- Connect-cookie actor fallback addresses the real `resolver.go:44-54` gap.

### Agreed Concerns (both reviewers — highest priority)

1. **D-06 owner-encoding needs guardrails beyond the encoder itself.** Antigravity: add a **reserved-namespace email guard** rejecting email values matching `^[0-9]+:` (before any string comparison) so a verified-email user cannot register `3:sub:3:foo` to occupy a namespaced owner. Codex (deeper): the encoding change is an **authz-key migration** — existing non-email records + already-issued cookies still carry bare owners, so it needs `migrate-remap-owner` + cookie invalidation/versioning, not just a resolver refactor (see BLOCKER below). Both are facets of the same theme: the hardened encoding is correct but its **rollout** is not yet safe.

### Codex-only findings (source-grounded, NEW this round — Antigravity truncated and could not corroborate)

These are the round-2 payoff. Codex traced them into the live source:

- **[BLOCKER] `memStore` retype breaks tests not listed in 17-02.** `storeFill` needs concrete `*store.Store` (`summaryqueue.go:119`; called with `d.st` at `summaryqueue_test.go:402/422`) and `buildUsageQueue` too (`tools.go:265`; `tools_test.go:1915`). 17-02 lists `tools_test.go` but omits `summaryqueue_test.go`, so its full-server test gate cannot pass. **Fix:** add `summaryqueue_test.go`; keep a concrete test-store reference alongside `deps.st`, or introduce narrow `summaryStore`/`usageStore` interfaces for those two consumers.
- **[BLOCKER] Duplicate `errStaleSummary` declaration.** It already exists at `summary.go:14-18` and is returned by `resolveSummaryUpdate` (`summary.go:23-35`); 17-02 Task 2 re-declares it in `identity.go` → compile failure. **Fix:** reuse the existing sentinel; only add the missing rule sentinel.
- **[HIGH] Owner-encoding rollout has no migration/session story.** Old non-email records + live cookies (`session.go:21-29`, forwarded unchanged at `resolver.go:44-54`) keep bare owners while new logins get encoded owners → one principal split across buckets, and the D-06 collision is *preserved* during rollout. **Fix:** old→encoded mapping via `migrate-remap-owner`; cookie invalidation (cookie-key rotation) or session-payload versioning that rejects legacy non-email sessions; a test proving an old bare `sub` cookie can't coexist with the new namespace.
- **[HIGH] Parity test shares one mutable fixture across both lanes within a row.** A direct `DeleteMemory`/update/replace mutates state before the Connect call runs → order-dependent false failures; per-row reset does not fix per-lane mutation. **Fix:** each lane gets its own independently seeded fixture (or restore spy/state between the two calls).
- **[HIGH] `connectError` taxonomy still incomplete.** `parseWindow` past-`not_after`/ordering errors (`tools.go:445-470`; proto leaves the wall-clock check to the handler at `engram.proto:192-198`) and rule-summary validation (`rules.go:74-85`) are plain errors → wrongly mapped to `CodeInternal`. **Fix:** typed `errInvalidArgument`-compatible sentinels for those before the common mapper; table-test every mapped write failure incl. past `not_after`.
- **[MEDIUM] Payload-only update drops the usage signal.** `Store.Update` increments `AccessCount` + stamps `LastAccessedAt` (`store.go:1379-1384`, pinned by `usage_test.go:112-145`, documented at `config.go:144-150`); the payload-only method mirrors only visibility/summary → shared/summary-only Connect updates stop counting as updates. **Fix:** include usage-counter fields in the `SetPayload` op + test.
- **[MEDIUM] `SearchDiscoveries` k regresses 20→8.** Connect defaults discovery search to 20 (`connectapi.go:224-228`); `deps.searchDiscovery` defaults to 8 (`tools.go:894-915`); 17-04's read-rewire applies `k=20` only to `SearchMemories`. **Fix:** apply `k=20` in the Connect `SearchDiscoveries` adapter + regression test (the same class as the round-1 SearchMemories fix, one path missed).
- **[MEDIUM] Spy alone does not prove same-`deps` delegation.** `memStore` sits below `deps`; the spy records store calls, and `storeMemory`/`scheduleMemory` share `MintShortID`+`Upsert` (`tools.go:634-694`), so a wrong-method handler could produce the same trace. **Fix:** add a source/AST assertion that each Connect method calls its named `deps.*` method.
- **[MEDIUM] Empty-`ENGRAM_OWNER_CLAIM` acceptance criterion contradicts the loader.** The env transform drops empty env values and keeps defaults (`config.go:176-185`, tested `config_test.go:34-50`), so `ENGRAM_OWNER_CLAIM=""` resolves to `email`, never an empty list. **Fix:** test explicit `--owner-claim=""` (CLI, observable) instead, or explicitly carve an exception.
- **[LOW] `connectError(err)` lacks request context.** Prefer `connectError(ctx, err)` for request-scoped trace/log fields.

### Divergent Views

- **Overall readiness.** Antigravity: ready once 17-06 is reconstructed/merged — **but 17-06 already exists** (agy truncation), so this "blocker" is void; on the plans it *did* see, agy is positive. Codex: **HIGH risk, not executable as written** due to the two compile blockers, the authz-key migration hazard, the parity-fixture flaw, and the incomplete error taxonomy. Weight Codex's source-grounded findings as the true state this round.

### Recommended disposition for `/gsd-plan-phase 17 --reviews` (round 2)

Incorporate (must): (1) add `summaryqueue_test.go` to 17-02 and resolve the `storeFill`/`buildUsageQueue` concrete-store break (narrow `summaryStore`/`usageStore` interfaces, or a retained concrete test-store handle); (2) reuse the existing `errStaleSummary` sentinel, add only the rule sentinel; (3) make D-06 a migration: `migrate-remap-owner` mapping + cookie invalidation/versioning + the reserved-namespace email guard (`^[0-9]+:`) + a legacy-session-rejection test; (4) give each parity lane its own independently seeded fixture; (5) add typed `errInvalidArgument` sentinels for `parseWindow` + rule-summary validation before wiring `connectError`, and table-test past-`not_after`; (6) mirror the usage signal (`AccessCount`/`LastAccessedAt`) in the payload-only update; (7) apply `k=20` in the Connect `SearchDiscoveries` adapter + regression test; (8) add a source/AST delegation assertion alongside the spy; (9) retarget the empty-owner-claim acceptance criterion to `--owner-claim=""` (CLI). Consider: `connectError(ctx, err)` signature. Everything the round-1 revision changed is endorsed by both reviewers — these are refinements on top of a now-sound architecture.
