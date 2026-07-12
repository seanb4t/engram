---
phase: 17
round: 5
reviewers: [codex, opencode, antigravity]
reviewed_at: 2026-07-12T18:02:18Z
plans_reviewed: [17-01-PLAN.md, 17-02-PLAN.md, 17-03-PLAN.md, 17-04-PLAN.md, 17-05-PLAN.md, 17-06-PLAN.md]
models:
  codex: gpt-5.x (codex-cli 0.144.1)
  opencode: openrouter/x-ai/grok-4.5
  antigravity: agy 1.1.1 (model selected internally)
caveats:
  antigravity:
    - "Prompt was 300KB/2688 lines; agy truncated the tail — it reports 17-06 as \"missing\" and 17-05 as \"truncated\" though both were fully present. Same artifact as round 2."
    - "Not source-grounded: all 7 file citations resolve to /Users/sean/.gemini/antigravity-cli/internal/... (agy's own cache dir), not the real repo. Findings re-derived from plan text, not verified against source."
    - "Every agy finding duplicates an already-incorporated round 1-4 fix. Weighted ~zero for plan-level consensus."
verified_against_source: true   # codex + opencode/grok-4.5 cites spot-checked against the live tree by the orchestrator; all confirmed
---

# Cross-AI Plan Review — Phase 17 (Round 5)

Round 5 of adversarial cross-AI review. Rounds 1-4 fixes are incorporated (commit 23ba8077,
plan-checker PASSED). Both source-grounded reviewers (Codex, OpenCode/grok-4.5) independently
confirm the architecture is sound and prior-round fixes are well integrated; the phase's
remaining risk is **entirely a set of unlisted test call sites that break the build/CI when the
planned signatures and handlers land** — no design flaws found.

## Codex Review
*(codex-cli 0.144.1, source-grounded — all cites orchestrator-verified against the live tree)*

# Round 5 Plan Review

## Overall assessment

The architecture is strong and the previous review fixes are well integrated: explicit caller threading, typed read convergence, vector-preserving partial updates, a production error mapper, and structural parity tests collectively address the phase’s main risk.

However, three unlisted test call sites will break once the planned signatures and handlers land, and the sub-second scheduling fix stops at validation rather than persistence. Overall risk remains **HIGH until the compile/test blockers are added to the plans**.

---

## 17-01 — Ordered owner claims and rollout migration

### Summary

The authorization-key migration is thoughtfully hardened. Ordered claims, malformed-value rejection, injective owner encoding, legacy-cookie invalidation, and an operator migration runbook form a coherent security story.

### Strengths

- The plan correctly treats owner strings as authorization keys. Store filters compare the owner directly at `internal/store/store.go:497` and `internal/store/store.go:517`, so injectivity and namespace disjointness are security requirements.
- Session-version invalidation is well placed. Existing sessions contain only the old bare owner (`internal/webauth/session.go:26-29`), and the resolver forwards it unchanged (`internal/webauth/resolver.go:44-54`).
- Auto-stamping the version inside `SessionCodec.Seal` covers the real mint site at `internal/webauth/handlers.go:172-175` and avoids per-call-site drift.

### Concerns

- **HIGH — `NewAuthenticator` signature changes omit two compiling test files.** The plan changes the final argument from `string` to `[]string`, but:

  - `internal/webauth/handlers_test.go:159` passes `"email"`.
  - `internal/webauth/oidc_exchange_test.go:148` passes `tc.ownerClaim`.
  - The latter’s cases are themselves modeled as scalar `ownerClaim` values at `internal/webauth/oidc_exchange_test.go:107-138`.

  Neither file appears in `files_modified` or the task file lists. `go test ./internal/webauth/...` therefore cannot compile as written.

### Suggestions

- Add `internal/webauth/handlers_test.go` and `internal/webauth/oidc_exchange_test.go` to Plan 17-01.
- Update the exchange test table to carry `ownerClaims []string`, including at least one ordered fallback case such as `[]string{"email", "sub"}`.

### Risk Assessment

**HIGH** until the omitted call sites are included; otherwise **MEDIUM** because this changes the persisted authorization namespace and session compatibility.

---

## 17-02 — Store seam, caller threading, and partial updates

### Summary

This is a sound foundation plan. It resolves the concrete-store interface blocker, preserves vectors for payload-only updates, retains usage signals, and gives downstream handlers typed results and errors.

### Strengths

- The payload-only method is necessary: `Store.Update` always overwrites content and upserts a supplied vector (`internal/store/store.go:1367-1406`), while `SetVisibility` demonstrates the vector-preserving `SetPayload` pattern (`internal/store/store.go:1417-1442`).
- Explicitly clearing summary provenance is correct. Full payload encoding omits empty provenance keys (`internal/store/store.go:332-336`), but the decoder consumes them whenever present (`internal/store/store.go:433-439`), so a partial payload write must overwrite or delete stale keys.
- Including `DeleteAll` and `ListScopes` in `memStore` is source-grounded: both remain direct interface calls at `internal/server/tools.go:1143` and `internal/server/connectapi.go:93`.

### Concerns

- **HIGH — caller-signature changes omit `embed_wiring_test.go`.** This file directly calls both methods being refactored:

  - `d.searchMemory(...)` at `internal/server/embed_wiring_test.go:38`
  - `d.storeMemory(...)` at `internal/server/embed_wiring_test.go:52`

  Plan 17-02 changes `storeMemory` to require a caller, while Plan 17-06 changes `searchMemory`. Neither plan lists this file. The server test package will fail to compile.

### Suggestions

- Add `internal/server/embed_wiring_test.go` to Plan 17-02 or 17-06, with ownership clearly assigned.
- Preserve these tests’ stop-before-store intent by supplying an explicit anonymous caller; they deliberately construct `deps` without a store and return from the embedder first (`internal/server/embed_wiring_test.go:13-30`).

### Risk Assessment

**HIGH** until the omitted test is assigned. The production design itself is **MEDIUM** risk due to the new payload-only mutation path.

---

## 17-03 — Proto conversion layer

### Summary

The dedicated conversion layer is well scoped and keeps handlers thin. Exact-mapping tests are a better fit than artificial round-trip assertions.

### Strengths

- Mask-aware pointer mapping directly prevents the current content-blanking hazard: `updateArgs.Content` is presently unconditional (`internal/server/tools.go:507-512`), and `Store.Update` always applies it (`internal/store/store.go:1379`).
- `RFC3339Nano` is correct for preserving protobuf timestamp precision through `parseWindow`, which accepts RFC3339 fractional seconds (`internal/server/tools.go:452-470`).

### Concerns

- **MEDIUM — sub-second precision is preserved only through validation, not persistence.** The plan specifically tests a `not_after` less than one second in the future. That value may pass `parseWindow`, but storage floors both bounds to Unix seconds:

  - Encoding: `internal/store/store.go:319-323`
  - Decoding: `internal/store/store.go:405-411`

  A bound 500ms in the future can therefore be persisted as the start of the current second and become immediately expired. The RFC3339Nano adapter test would pass while end-to-end behavior remains wrong.

### Suggestions

- Either add a backward-compatible nanosecond-precision store representation and an end-to-end scheduling test, or explicitly state that scheduling persistence remains second-granular and avoid the `<1s future` correctness claim.
- Prefer a real `ScheduleMemory → persisted Memory → recall gate` test if precision is intended as part of the Connect contract.

### Risk Assessment

**MEDIUM**, due to a mismatch between the adapter’s precision guarantee and the actual persisted behavior.

---

## 17-04 — Connect handlers, reads, and error mapping

### Summary

The handler and read-lane wiring is structurally correct, but an existing CSRF test still assumes the write handlers are unimplemented and will break immediately.

### Strengths

- Removing the duplicate Connect usage enqueue is necessary: the current handler enqueues at `internal/server/connectapi.go:209-211`, while `deps.getMemory` already does so at `internal/server/tools.go:1000-1003`.
- Preserving Connect-specific defaults at the adapter is correct. Existing Connect search defaults to 20 (`internal/server/connectapi.go:162-165` and `:224-227`), while MCP defaults to 8 (`internal/server/tools.go:854-857` and `:904-906`).
- Empty-scope discovery handling is correctly identified: the current store treats empty scope as cross-scope, whereas `effectiveDiscoveryScope` rejects it unless `CrossSpine` is set (`internal/server/tools.go:881-891`).

### Concerns

- **HIGH — `connectcsrf_test.go` still expects an unimplemented handler and uses nil dependencies.** `TestConnectCSRFTokenMatrix` constructs `d := &deps{}` at `internal/server/connectcsrf_test.go:225` and expects a valid CSRF request to return `CodeUnimplemented` at `:247-250`. Once `StoreMemory` is wired, that request reaches `d.em.Embed`/`d.st` and will panic or return a different result. Plan 17-04 updates only the negative matrix, not this CSRF matrix.
- **LOW — the `CodeAborted` must-have has no current typed sentinel.** The current relevant sentinel set is `ErrNotFound`, `ErrInvalidArgument`, and `ErrAmbiguousShortID` (`internal/store/store.go:42-56`), plus `errStaleSummary` (`internal/server/summary.go:14-18`). The action qualifies Aborted with “if a distinct sentinel exists,” while the must-have states it unconditionally.

### Suggestions

- Add `internal/server/connectcsrf_test.go` to Plan 17-04.
- Back `TestConnectCSRFTokenMatrix` with the same spy/non-nil embedder fixture and change the matching-token cell to a real success or deterministic domain result.
- Remove the unconditional Aborted claim or introduce a specifically defined conflict sentinel and test.

### Risk Assessment

**HIGH** because the existing CSRF suite will fail as soon as real handlers replace the stubs.

---

## 17-05 — Parity and existence-leak gates

### Summary

This is a strong acceptance plan. Independent fixtures, production error mapping, spy traces, AST delegation checks, and split leak assertions cover both behavioral and structural parity.

### Strengths

- Independent per-lane fixtures correctly prevent mutation-order contamination.
- Mapping direct domain errors through the production mapper avoids comparing `connect.CodeOf(domainError) == Unknown` against a mapped handler error.
- Split short-ID and UUID assertions accurately encode the leak boundary. Existing GetMemory coverage establishes the intended rule at `internal/server/connectapi_test.go:621-645`.
- The AST assertion closes the real ambiguity that `storeMemory` and `scheduleMemory` share similar store traces.

### Concerns

- No net-new blocking issue found in round 5.

### Suggestions

- Keep AST checks narrowly limited to named delegation calls so harmless handler refactors do not make the test unnecessarily brittle.
- Ensure the actor assertion checks exact expected values per lane, as the plan now specifies, rather than merely non-empty values.

### Risk Assessment

**LOW to MEDIUM**. The plan is test-heavy, but appropriately so for the milestone’s primary authorization-parity requirement.

---

## 17-06 — Typed read core

### Summary

The typed superset read core is the correct response to the earlier field-loss risk. Per-lane defaults, typed timestamps, and separate cursor/offset tests preserve existing transport contracts.

### Strengths

- The plan correctly models the full Connect list contract currently passed to `store.List` at `internal/server/connectapi.go:124-138`.
- Removing shared defaults is source-consistent: `Store.List` deliberately treats `Limit==0` as all records (`internal/store/store.go:870-880`), while `SearchReranked` rejects `k==0` (`internal/store/store.go:677-679`).
- Separate offset and cursor assertions match the actual mutual-exclusion check and branch behavior at `internal/store/store.go:844-867`.

### Concerns

- **HIGH — shared with Plan 17-02: `embed_wiring_test.go` is absent.** Its direct `searchMemory` call at `internal/server/embed_wiring_test.go:38` must change to the new caller/core-request signature. Plan 17-06’s `files_modified` lists only `tools.go`, `tools_test.go`, and `connectapi_test.go`.

### Suggestions

- Assign `internal/server/embed_wiring_test.go` to Plan 17-06 for the search-path update, while Plan 17-02 handles its store-path update, or place both edits in 17-06 after 17-02.
- Add a repository-wide `rg` acceptance check for direct calls to every changed `deps.*` method rather than enumerating only two test files.

### Risk Assessment

**HIGH** until all signature call sites are covered; otherwise **MEDIUM** because read-contract regressions would be externally visible.

---

## Required round-5 changes

1. Add `internal/webauth/handlers_test.go` and `internal/webauth/oidc_exchange_test.go` to 17-01.
2. Add `internal/server/embed_wiring_test.go` to 17-02/17-06.
3. Add and update `internal/server/connectcsrf_test.go` in 17-04.
4. Resolve or explicitly document the sub-second scheduling persistence mismatch.
5. Reconcile the unconditional `CodeAborted` must-have with the absence of a conflict sentinel.

With those corrections, the six-plan wave structure should achieve the phase goal without reopening the authz or existence-leak risks.

---

## OpenCode Review
*(openrouter/x-ai/grok-4.5, source-grounded — real repo reads observed; cites orchestrator-verified)*

I'll verify the plans against current source — checking identity, store update paths, Connect handlers, and the test landmines called out in research.
# Phase 17 Plan Review — Round 5

Verified against live source under `/Volumes/Code/github.com/seanb4t/engram` (no CodeGraph index; file:line cites from direct reads). Round 1–4 items that the plans still implement and still match code are **not** re-litigated.

---

## Overall

The six-plan wave structure still hits REQ-connect-write-authz-parity: shared `caller` + `deps.*`, injectable `memStore`, mask-safe update, injective owner encoding with session versioning, typed read core, thin Connect adapters, and spy/AST parity. Round-4 fixes (malformed-claim generalization, CrossSpine empty-scope, usage double-count, pagination split, lane-appropriate Actor) remain grounded.

**One net-new HIGH test landmine** is still open: `connectcsrf_test.go` (not only `connectapi_negative_test.go`). A few MEDIUM/LOW implementation notes remain around SetPayload provenance clearing and executor precision.

**Risk: MEDIUM** (was trending LOW after round-4) solely because the CSRF happy-path CI break will fire when 17-04 wires handlers unless fixed in the same plan.

---

## 17-01 — Ordered owner claims + injective encoding + session versioning

### Summary
Authz-key change is correctly treated as migration+encoder work, not a rename. Still sound against `ClaimIdentity` (`internal/auth/auth.go:83-97`), `resolver.go:54`, and `handlers.go:172-175`.

### Strengths
- D-05 fail-closed + round-4 present-but-non-string reject for **every** ordered claim still matches today’s coercion hazard: `owner, _ = raw[ownerClaim].(string)` (`auth.go:86`) that turns non-strings into `""` and can fall through later claims.
- Length-prefixed encoding is necessary: store authz is exact string match at `ownerOrSharedCondition` / `ownerOnlyCondition` (`store.go:497`, `:517`).
- Session version in `Seal` is right: only mint is `handlers.go:172-175`; version field on `Session` (`session.go:26-29`) is zero today, so unversioned cookies naturally unseal as `V==0`.
- Empty-ENV vs CLI flag is correctly grounded: `config.go:179-184` empty ENV preserves default.

### Concerns
- **[LOW]** Empty-string email + `email_verified=false` + list `[email,sub]`: plan falls through; today with claim=`email` always hits the verified gate (`auth.go:87-94`). Fail-closed still holds for list=`[email]`; for `[email,sub]` this is an intentional widening. Document in secure-phase.
- **[LOW]** No residual open item from prior rounds.

### Suggestions
- Keep one unit case for empty-string email under `[email,sub]` so the fall-through semantics are deliberate, not accidental.

### Risk
**LOW** for this plan alone.

---

## 17-02 — memStore, payload-only update, caller, update *string

### Summary
Still the correct foundation wave. Ceiling claims (DeleteAll/ListScopes signatures, `errStaleSummary` at `summary.go:16`, `storeFill`/`buildUsageQueue` concrete `*store.Store`, tags re-embed in `tools.go:965-972`) match the tree.

### Strengths
- Acknowledge real `Update` always takes `content string` + vec + Upsert (`store.go:1367-1406`) and always re-embeds from MCP path (`tools.go:972-980`).
- Payload-only path correctly modeled on `SetVisibility` SetPayload (`store.go:1417-1442`) while adding AccessCount bump from `Update` (`:1379-1384`).
- `ListScopes` return `([]ScopeCount, bool, error)` at `store.go:1110` and `DeleteAll` at `:1508` still force interface inclusion.
- Reusing `errStaleSummary` is mandatory — redeclaring would not compile.

### Concerns
- **[MEDIUM]** Provenance clear via SetPayload is underspecified for `summary_egress_at`. Full upsert omits zero egress (`store.go:335-336`); SetPayload only overwrites supplied keys. Writing `time.Time{}.Format(RFC3339)` would store a `0001-01-01…` string that `time.Parse` accepts (`store.go:436-438`) → non-zero egress again. Spec: **delete** `summary_egress_at` (or set a value the decoder treats as zero), not blank-format zero time. Same for relying on encoder map semantics.
- **[LOW]** `listScheduled` invalid `state` stays bare `fmt.Errorf` (`tools.go:844`); no Connect RPC today — OK, but if ever exposed it maps Internal unless wrapped.

### Suggestions
- In Task 1 store test (c), assert decoder sees zero `SummaryEgressAt` **and** that the payload key is absent (raw Get), not just struct field.
- Explicit note in action: use Qdrant overwrite/delete for provenance keys; do not marshal zero-time.

### Risk
**LOW–MEDIUM** (payload provenance edge if executor is sloppy).

---

## 17-03 — protoconv

### Summary
Still a clean, complete D-09 layer. RFC3339Nano + nil Content residual are correct relative to `parseWindow` (`tools.go:445-473`) and `*string` Content from 17-02.

### Strengths
- Sub-second future/`not_before < not_after` risk is real for RFC3339 truncation.
- Exact-mapping (not false round-trip) fits optional FieldMask → pointer fields.

### Concerns
None net-new. Round-2 “no 17-03 findings” still holds.

### Suggestions
None required.

### Risk
**LOW**

---

## 17-04 — Handlers, connectError, spy, read rewire

### Summary
Wiring design still closes Pitfall 1. CrossSpine empty-scope (`tools.go:884-892` + `SearchDiscovery` empty=all `store.go:688-716`), dual-enqueue (`connectapi.go:211` + `tools.go:1000-1003`), and k=20 vs MCP k=8 (`connectapi.go:162-165`, `:224-227` vs `tools.go:855-856`, `:904-905`) are correctly handled. **Missed comprehensive “nil deps / Unimplemented flip” across the full CSRF suite.**

### Strengths
- `engramAPI` still has only five read methods; writes still use `UnimplementedEngramServiceHandler` (`connectapi.go:88-233`) — “author six methods” remains true.
- `ErrAmbiguousShortID` at `store.go:56` / `:1217`; GetMemory today falls through to Internal (`connectapi.go:191-198`) — mapping to FailedPrecondition is a justified read+write improvement.
- `usageQueue.tryEnqueue` is nil-safe (`usagequeue.go:72-73`), so spy deps without a queue is fine.
- ListScopes-as-exception still correct (no MCP counterpart).

### Concerns
- **[HIGH] `TestConnectCSRFTokenMatrix` happy path will break when writes are real.**  
  - `connectcsrf_test.go:225` — `d := &deps{}`  
  - `:248-250` — matching cookie+header expects **`CodeUnimplemented`**  
  - Once `StoreMemory` is wired, request hits nil `d.st` → panic, or a real error/success — **not** Unimplemented.  
  - Plan 17-04 only lists `connectapi_negative_test.go` (`:64`, `:169`) in landmine-1 and `files_modified`. CSRF suite is a permanent Phase-16 gate and will redden CI in the same commit sequence.
- **[MEDIUM]** SearchMemories / SearchDiscoveries rewrite must **replace** handler-local `EmbedQuery` (`connectapi.go:158`, `:220`) with deps-only embed. Easy to leave a double-embed or dead vec if body is patched rather than replaced. Action implies thin adapter but doesn’t say “delete EmbedQuery from handlers.”
- **[LOW]** Happy-path CSRF cell fix options: spy+non-nil embedder + expect success/InvalidArgument, or “any code ≠ PermissionDenied” credential for CSRF-pass-through.

### Suggestions
1. Add `internal/server/connectcsrf_test.go` to 17-04 `files_modified` and must_haves.
2. Happy path: spy-backed deps + real success (or assert Code ≠ PermissionDenied ∧ ≠ Unimplemented if you only care about CSRF pass-through).
3. Explicit action line: remove handler EmbedQuery on Search* rewires.
4. Keep asserted k=0→20 **and** CrossSpine for empty scope (already present — keep as-is).

### Risk
**MEDIUM–HIGH** until CSRF happy path is in scope of the same wave-3 commit set as handler wiring.

---

## 17-05 — Parity + leak tables + idempotency re-assert

### Summary
Acceptance-gate plan still matches SC2/SC4/SC5. Round-4 lane-appropriate Actor (`resolver.go:54` vs MCP UserID) and connectError code normalization for direct deps errors are correct.

### Strengths
- Per-lane fixtures for mutating rows (Delete/update) remain necessary.
- AST assertion via `go/parser`/`runtime.Caller` (not relative ReadFile under `go test` cwd) remains right.
- Split short_id vs UUID leak tables match GetMemory template (`connectapi.go:203-205`) / deps re-wrap (`tools.go:934-935`, `:1020-1021`).

### Concerns
- **[LOW]** Spy authz drift already mitigated by keeping Qdrant isolation as gate — no reopen.
- Depends on 17-04 spy + connectError existing; sequencing is correct.

### Suggestions
- Optional: one parity row that stresses `ErrAmbiguousShortID` if spy can script it — nice but not required for #322.

### Risk
**LOW** (assuming 17-04 CSRF fix lands first).

---

## 17-06 — Typed read core (D-07 hardened)

### Summary
Still the right answer to the naive rewire regression. Live code confirms: MCP list always `CursorMode: true` and discards total (`tools.go:809-820`); list default Limit 20 (`:794-796`); search k=8 (`:855-856`); Connect list carries offset/categories/visibility/total/token (`connectapi.go:124-149`). Offset vs cursor mutual exclusion (`store.go:844-845`, token only in cursor mode).

### Strengths
- Removing shared `Limit==0→20` preserves Connect `limit=0` = all (`store.go` Limit semantics at list path).
- Keeping `searchDiscovery` internal k=8 with comment + Connect pre-default in 17-04 avoids 20→8 regression.
- Keeping `subjectFromContext` for `identityForLog` (`instrument.go:81-82`) avoids a false “delete helpers” cleanup.

### Concerns
None net-new vs round-4.

### Suggestions
None required.

### Risk
**LOW**

---

## Cross-cutting

| Check | Verdict |
|--------|---------|
| Wave order 01+02 → 03∥06 → 04 → 05 | Sound; 04 depends on 03+06 |
| Phase goals SC1–SC5 | Covered if “thin adapter” + parity + leak tables land |
| SECURE-phase flag | Still warranted (owner encoding + claim list) |
| Scope creep | Acceptable; identity+read rewire bundling was user-locked |
| Zero new deps | Still true |
| Double-count usage on Connect Get | Addressed (remove `connectapi.go:211`) |
| CSRF/negative nil-store landmines | **Partial** — negative matrix yes, CSRF matrix no |

---

## Must-fix before execute (rank ordered)

1. **HIGH — 17-04:** Include `connectcsrf_test.go` happy path well beyond “not PermissionDenied while Unimplemented.”  
   Evidence: `connectcsrf_test.go:225`, `:248-250`. Same wave as wiring or CI fails mid-phase.

2. **MEDIUM — 17-02:** Specify SetPayload provenance clear: delete/absent `summary_egress_at`, do not write zero-time RFC3339.  
   Evidence: `store.go:335-336` vs `:436-438`.

3. **MEDIUM — 17-04:** Explicit “delete handler EmbedQuery; only deps embeds” on Search rewires.  
   Evidence: `connectapi.go:158`, `:220` vs `tools.go:870`, `:911`.

---

## Risk Assessment

| Plan | Risk |
|------|------|
| 17-01 | LOW |
| 17-02 | LOW–MEDIUM (payload clear detail) |
| 17-03 | LOW |
| 17-04 | **MEDIUM–HIGH** (CSRF happy path omitted) |
| 17-05 | LOW |
| 17-06 | LOW |
| **Phase overall** | **MEDIUM** — architecture solid; unfixed CSRF test will block green merge at wave 3 |

After the CSRF landmine is written into 17-04 (and provenance wording tightened in 17-02), this set is ready to execute without another full replan.

---

## Antigravity Review
*(agy 1.1.1 — SEE CAVEATS: prompt-tail truncated + citations point at agy's cache dir, not the repo. Findings duplicate already-incorporated round 1-4 fixes; retained for completeness, not weighted in consensus.)*

# Cross-AI Plan Review: Phase 17 — Wired Write Handlers (Full CRUD & Schedule)

This document provides structured, critical feedback on the implementation plans for **Phase 17** of the `engram` project. It evaluates the plan quality, completeness, and risks, highlighting gaps between the proposed tasks and the target requirements.

---

## 1. Architectural & Sequence Overview

The phase is structured in a four-wave dependency graph designed to transition `engram`'s Connect write lane from unimplemented stubs into production-ready endpoints sharing a unified code path with the MCP tool handlers. 

### Core Plan Gaps
* **Missing Plan File**: The roadmap specifies `17-06-PLAN.md — Read-lane transport-neutral typed core convergence (D-07 hardened)` as a Wave 2 task. However, the plan content for `17-06-PLAN.md` is omitted from the plans provided for review. This represents a significant gap in planning completeness.
* **Wave 1 Circular & Compilation Dependencies**: Retyping `deps.st` in `17-02-PLAN.md` to `memStore` without including [DeleteAll](file:///Users/sean/.gemini/antigravity-cli/internal/store/store.go#L1508) and `ListScopes` will immediately break compilation in `tools.go` and `connectapi.go`. 

---

## 2. Plan-by-Plan Analysis

### 17-01-PLAN.md: Ordered Owner-Claim List & Namespacing
This plan implements the multi-claim identity resolution ([ClaimIdentity](file:///Users/sean/.gemini/antigravity-cli/internal/auth/auth.go#L83)) and namespacing (D-04, D-05, D-06).

#### Critical Deficiencies & Risks
1. **Non-Injective Encoding Collision (STRIDE: Elevation of Privilege)**:
   * **The Issue**: The plan implements naive namespacing in the format `<claim>:<value>` (e.g. `sub:x`). This is not injection-safe. For example, `(claim: "sub", value: "x:y")` and `(claim: "sub:x", value: "y")` serialize to the identical owner string `sub:x:y`. Since the owner string is compared directly in store authorization filters, this allows cross-user access/hijacking.
   * **Mitigation**: Implement a provably injective length-prefixed encoding format: `fmt.Sprintf("%d:%s:%d:%s", len(claim), claim, len(value), value)`.
2. **Missing Reserved-Namespace Email Guard**:
   * **The Issue**: An email value containing length prefixes and colons (e.g., `3:sub:5:svc-1`) could be registered to impersonate a namespaced service token.
   * **Mitigation**: Add a validation guard rejecting email values matching `^[0-9]+:`.
3. **No Rollout/Session Cookie Invalidation Strategy (Migration Blocker)**:
   * **The Issue**: Existing active UI session cookies store raw, un-namespaced owner strings. If unresolved on rollout, legacy sessions will write bare records, splitting user identity and leaving collisions active.
   * **Mitigation**: Add a version field (e.g., `V int json:"v"`) to [Session](file:///Users/sean/.gemini/antigravity-cli/internal/webauth/session.go#L20). Have [SessionCodec.Seal](file:///Users/sean/.gemini/antigravity-cli/internal/webauth/session.go) auto-inject the current version, and reject legacy (version 0 / unversioned) cookies in [Resolver.Resolve](file:///Users/sean/.gemini/antigravity-cli/internal/webauth/resolver.go#L36) to force re-login.
4. **Permissive Claim-Parsing Rules**:
   * **The Issue**: Comma-splitting `ENGRAM_OWNER_CLAIM` without validation could allow duplicates, empty interior entries, or invalid characters (like `:` or `,`) to silently resolve.
   * **Mitigation**: Enforce a strict parser contract rejecting duplicates, interior-empty values, and claim names starting with digits or containing delimiters.
5. **Type Coercion Vulnerability (T-17-03 / HIGH-1)**:
   * **The Issue**: Checking claim presence without checking the type (e.g., if a JSON payload includes a non-string `email` like `null` or a number) can result in silent fall-through to a later claim (like `sub`), routing the user to a different authz bucket.
   * **Mitigation**: Explicitly assert that the claim value is a string; if present but non-string, reject immediately rather than falling through.

---

### 17-02-PLAN.md: Interface Extraction & Caller Seam
This plan refactors `deps` to take an explicit `caller` parameter and extracts a database interface.

#### Critical Deficiencies & Risks
1. **Compilation Blockers (DeleteAll / ListScopes)**:
   * **The Issue**: Retyping `deps.st` to `memStore` without including `DeleteAll` and `ListScopes` causes build errors since `tools.go` and `connectapi.go` call them. Additionally, server tests (`storeFill` and `buildUsageQueue`) expect `*store.Store` but receive the interface.
   * **Mitigation**: Add `DeleteAll` and `ListScopes` to the [memStore](file:///Users/sean/.gemini/antigravity-cli/internal/server/store_iface.go) interface definition. Introduce a `testDepsWithStore` helper in tests to return a concrete store pointer.
2. **Missing Vector-Preserving Payload Update (`SetPayload`)**:
   * **The Issue**: `store.Update` requires document vectors to upsert. The fetched memory in `updateMemory` contains no vector. Without a payload-only update pathway, tags-only or summary-only edits will silently overwrite vectors with null values.
   * **Mitigation**: Add a database method leveraging Qdrant's `SetPayload` to write metadata updates without modifying vectors. Preserve the update usage signal by incrementing `AccessCount` and updating `LastAccessedAt`.
3. **Stale Auto-Summary Provenance Leak (MED-3)**:
   * **The Issue**: `SetPayload` only overwrites provided keys. If a user provides a client-written summary, the database's `summary_model` and `summary_egress_at` fields will persist, leading the decoder to misclassify the source as an auto-summary.
   * **Mitigation**: Explicitly delete or write-empty the `summary_model` and `summary_egress_at` keys when writing client-sourced summaries.
4. **Lack of Typed Sentinels**:
   * **The Issue**: The plan lacks an `errRuleImmutable` sentinel and fails to declare the reuse of the existing [errStaleSummary](file:///Users/sean/.gemini/antigravity-cli/internal/server/summary.go#L16).
   * **Mitigation**: Declare a new typed error sentinel for rule immutability and reuse the existing summary error to prevent string-matching in the handler mapper.

---

### 17-03-PLAN.md: protoconv Adapter
This plan implements request and response conversion helpers.

#### Critical Deficiencies & Risks
1. **Timestamp Precision Loss**:
   * **The Issue**: Converting `google.protobuf.Timestamp` into `RFC3339` strings drops sub-second/nanosecond precision, creating window boundary drift for scheduled memories.
   * **Mitigation**: Utilize `time.RFC3339Nano` or map directly to native Go `time.Time` structures.
2. **Undocumented mutationResult Mapping**:
   * **The Issue**: The response conversion details are sparse on mapping the [mutationResult](file:///Users/sean/.gemini/antigravity-cli/internal/server/identity.go) (containing ID and ShortID) to response stubs.
   * **Mitigation**: Include explicit helper functions in `protoconv` to populate response messages from `mutationResult`.

---

### 17-04-PLAN.md: Handler Wiring & Fake Store
This plan implements the thin-adapter Connect handlers and a mock store.

#### Critical Deficiencies & Risks
1. **Duplicate Inline Error Mapping**:
   * **The Issue**: Error mapping to Connect codes (`CodeNotFound`, `CodeInvalidArgument`) is mapped inline across all six write RPCs, increasing duplication and drift risk.
   * **Mitigation**: Implement a central [connectError](file:///Users/sean/.gemini/antigravity-cli/internal/server/connectapi.go) mapper helper to streamline handler bodies and ensure error taxonomy consistency.

---

### 17-05-PLAN.md: Parity Tests (Truncated)
This plan tests lane parity and prevents existence leaks.

#### Critical Deficiencies & Risks
1. **Truncated File Contents**:
   * **The Issue**: The file is cut off at the end (from Task 2 onwards), hiding validation constraints, threat modeling, and verification steps.
2. **Connect Actor Verification Gaps**:
   * **The Issue**: Parity tests must verify that `Actor` is set correctly for cookies (falling back to owner when `UserID` is missing).
   * **Mitigation**: Add a test row in the parity table verifying non-empty, equal `Actor` metadata across MCP and Connect paths.

---

## 3. Summary Risk Register

| Risk ID | Category | Component | Description | Mitigation Plan |
|---|---|---|---|---|
| **R-17-01** | **Compile Blocker** | `17-02` (Interface Retype) | Omitting `DeleteAll` and `ListScopes` from `memStore` breaks `tools.go` and `connectapi.go`. Retype breaks tests passing `d.st` to functions expecting `*store.Store`. | Include `DeleteAll` and `ListScopes` in `memStore`. Implement `testDepsWithStore` to retrieve concrete stores for tests. |
| **R-17-02** | **Security Risk (Access Bypass)** | `17-01` (Namespacing) | Naive `<claim>:<value>` formatting allows collisions (e.g. `sub:x:y`), bypassing authorization. | Implement a provably injective length-prefixed format (`%d:%s:%d:%s`). |
| **R-17-03** | **Security Risk (Data Split)** | `17-01` (Session cookies) | Rollout changes owner key formats but leaves old active session cookies unchanged, splitting identities. | Version the session cookie payload; reject unversioned/legacy cookies to force re-login. |
| **R-17-04** | **Data Loss** | `17-02` (Update Path) | Calling `store.Update` with an empty vector (missing from fetched records) overwrites the Qdrant document vector. | Implement a payload-only update pathway (`SetPayload`) preserving existing vectors. |
| **R-17-05** | **Logic Bug** | `17-02` (Auto-Summary) | Updating a document with a client summary leaves stale auto-summary metadata in Qdrant. | Programmatically clean out `summary_model` and `summary_egress_at` fields on client update. |
| **R-17-06** | **Security Risk (Type Spoofing)** | `17-01` (Type coercion) | Non-string claim values (like `null` or numbers) may bypass checks and fall through to subsequent claims. | Assert claim value types explicitly; reject if present but not a string. |

---

## 4. Recommendations & Next Steps

1. **Reject Plan 17-01**: Reject until the non-injective namespacing is replaced by the length-prefixed scheme, the session versioning is introduced, and type coercion rejections are added.
2. **Reject Plan 17-02**: Reject until `DeleteAll`/`ListScopes` are included in `memStore`, the test concrete store references are handled via `testDepsWithStore`, and the vector-preserving `UpdatePayload` is implemented.
3. **Request 17-06-PLAN.md**: Request that the missing read-lane core convergence plan is written and reviewed as part of Wave 2 before proceeding.
4. **Standardize Error Handling**: Mandate a central `connectError` helper in `17-04` instead of inline handlers.

---

## Consensus Summary

Two source-grounded reviewers (Codex, OpenCode/grok-4.5) reviewed against the live tree; every
`file:line` cite below was independently re-verified by the orchestrator and **confirmed**.
Antigravity is excluded from consensus (truncated + non-grounded — see caveats).

**Verdict: HIGH risk — but narrowly and mechanically.** The design is done; the blockers are
all *unlisted test call sites* that will fail `go build`/CI the moment the planned signatures and
handlers land. No new design flaws. Both reviewers agree: add the omitted test files + tighten
two MEDIUM implementation details and the six-plan set is execute-ready **without another full replan**.

### Agreed Strengths (2+ reviewers)
- Owner-key change is correctly treated as migration + injective-encoding + session-versioning
  work, not a rename (Codex, grok). Grounded on `store.go:497/517` (exact-match authz),
  `session.go:26-29`, `handlers.go:172-175`, `resolver.go:44-54`.
- Payload-only update (`SetPayload`, vector-preserving) is the right answer to `Store.Update`
  always upserting a vector (`store.go:1367-1406` vs `:1417-1442`) (Codex, grok).
- `memStore` must include `DeleteAll` (`tools.go:1143`) + `ListScopes` (`connectapi.go:93`);
  reuse `errStaleSummary` (`summary.go:16`) — redeclaring won't compile (Codex, grok).
- Typed read core (17-06) correctly preserves per-lane defaults: Connect list keeps offset/
  categories/visibility/total/cursor; MCP search k=8 vs Connect k=20; `Limit==0`=all vs
  `k==0` rejected (`connectapi.go:124-149`, `tools.go:855-856`, `store.go:870-880/677-679`) (Codex, grok).
- Removing the duplicate Connect usage enqueue is necessary (`connectapi.go:209-211` double-counts
  vs `tools.go:1000-1003`) (Codex, grok).
- 17-05 parity/leak-table plan is strong: per-lane fixtures, production error mapper, AST
  delegation assertion, split short_id/UUID leak tables (Codex, grok — no net-new issue).

### Agreed Concerns (highest priority)

**[HIGH — CONVERGENT, both reviewers, orchestrator-CONFIRMED] 17-04: `connectcsrf_test.go` breaks when writes are wired.**
`TestConnectCSRFTokenMatrix` constructs `d := &deps{}` (`connectcsrf_test.go:225`, also `:194`, `:372`)
and the happy-path cell expects `want: connect.CodeUnimplemented` (`:250`, guard at `:212-213`). Once
`StoreMemory` is wired, a CSRF-passing request reaches nil `d.em`/`d.st` → panic or a non-Unimplemented
result → red CI **in the same commit that wires the handlers**. This is a permanent Phase-16 gate.
Plan 17-04's landmine-1 + `files_modified` name only `connectapi_negative_test.go`; **`connectcsrf_test.go`
is absent from all six plans** (verified). Both reviewers found this independently via real source reads.
→ *Fix:* add `connectcsrf_test.go` to 17-04 `files_modified`/must-haves; back the happy-path cell with a
spy + non-nil embedder and expect real success (or assert Code ∉ {PermissionDenied, Unimplemented} if only
CSRF pass-through matters).

### Codex-only HIGH (net-new, orchestrator-CONFIRMED — grok & agy missed)

**[HIGH] 17-01: `NewAuthenticator` `string → []string` omits two webauth test files.**
`handlers_test.go:159` passes scalar `"email"`; `oidc_exchange_test.go:148` passes `tc.ownerClaim`,
whose cases are scalar `string` (`:99`, `:107-138`). Neither file is in any plan (verified).
`go test ./internal/webauth/...` will not compile.
→ *Fix:* add both files to 17-01; convert the exchange test table to `ownerClaims []string`, incl. one
ordered-fallback case (`[]string{"email","sub"}`).

**[HIGH] 17-02 / 17-06: caller-signature change omits `embed_wiring_test.go`.**
`embed_wiring_test.go:38` calls `d.searchMemory(...)`, `:52` calls `d.storeMemory(...)`, both via
`d := &deps{em: rec}` with no store (`:37/:51`, intentional stop-before-store at `:13-30`). 17-02 adds a
caller to `storeMemory`; 17-06 adds one to `searchMemory`. The file is in no plan (verified). Server test
package won't compile.
→ *Fix:* assign `embed_wiring_test.go` — store-path edit to 17-02, search-path edit to 17-06 (or both in
17-06 after 17-02); preserve the stop-before-store intent with an explicit anonymous caller. Codex also
recommends a repo-wide `rg` acceptance check for direct calls to every changed `deps.*` method rather than
enumerating files by hand.

### Codex-only MEDIUM (net-new, orchestrator-CONFIRMED)

**[MEDIUM] 17-03: sub-second scheduling precision is preserved only through validation, not persistence.**
`parseWindow` accepts RFC3339Nano (`tools.go:452-470`), but storage floors both bounds to integer Unix
seconds: encode `store.go:320` (`m.NotBefore.Unix()`) / `:323` (`m.NotAfter.Unix()`), decode `:406`/`:410`
(`time.Unix(sec,0)`). A `not_after` 500ms in the future persists as the start of the current second → the
record is immediately expired, yet the RFC3339Nano *adapter* test (17-03) passes. The adapter's precision
guarantee is contradicted by end-to-end behavior.
→ *Fix:* either add a backward-compatible nanosecond store representation + a real
`ScheduleMemory → persisted → recall-gate` E2E test, **or** drop the `<1s future` correctness claim and
state persistence is second-granular.

### grok-4.5-only MEDIUM (net-new, orchestrator-CONFIRMED)

**[MEDIUM] 17-02: SetPayload provenance clear must DELETE the key, not blank-format zero-time.**
Full upsert omits `summary_egress_at` when zero (`store.go:335-336`, guarded `!IsZero()`), but the decoder
consumes the key whenever *present* (`:436-438`). A partial `SetPayload` that writes
`time.Time{}.Format(RFC3339)` stores `"0001-01-01T00:00:00Z"`, which `time.Parse` accepts → a non-zero
`SummaryEgressAt` reappears, misclassifying a client summary as auto-generated. (Full path handles this by
setting the field to `time.Time{}` at `:1404`, which then omits on re-encode — a partial write cannot omit.)
→ *Fix:* spec Qdrant key **deletion** (or a decoder-treated-as-zero sentinel) for `summary_egress_at` /
`summary_model` on client-sourced summaries; assert in the store test that a raw `Get` shows the key
*absent*, not merely that the struct field is zero.

**[MEDIUM] 17-04: on the Search rewire, DELETE handler-local `EmbedQuery`, don't just patch.**
`connectapi.go:158`/`:220` embed in the handler today; a thin-adapter rewire that patches rather than
replaces risks a double-embed or dead vec.
→ *Fix:* add an explicit "remove handler `EmbedQuery`; only `deps.*` embeds" action line to 17-04.

### Lower-priority (single reviewer, confirmed)
- **[LOW — Codex] 17-04:** the `CodeAborted` must-have is stated unconditionally, but no conflict sentinel
  exists today (sentinels are `ErrNotFound`/`ErrInvalidArgument`/`ErrAmbiguousShortID` at `store.go:42-56`
  + `errStaleSummary` at `summary.go:14-18`); the action itself qualifies "if a distinct sentinel exists."
  → Reconcile: drop the unconditional claim or define + test a real conflict sentinel.
- **[LOW — grok] 17-01:** empty-string `email` + `email_verified=false` under list `[email, sub]` falls
  through to `sub` — an intentional widening vs the fail-closed `[email]` case. Document the deliberate
  semantics in secure-phase and keep one unit case pinning it.

### Divergent Views
None contradictory. Codex and grok-4.5 are **complementary, not conflicting** — each surfaced net-new
compile/CI blockers the other missed, and they converged exactly on the CSRF landmine. Both rate overall
risk HIGH *solely* on the unlisted test call sites, and both agree the design and round-1-4 fixes are sound.
Antigravity diverges only by re-raising already-fixed items (a truncation/grounding artifact, not a real
disagreement).

### Required round-5 changes (rank-ordered, all orchestrator-CONFIRMED)
1. **HIGH** — Add + fix `internal/server/connectcsrf_test.go` in 17-04 (nil-deps happy-path cell). *[Codex + grok]*
2. **HIGH** — Add `internal/webauth/handlers_test.go` + `oidc_exchange_test.go` to 17-01; convert table to `[]string`. *[Codex]*
3. **HIGH** — Assign `internal/server/embed_wiring_test.go` to 17-02 (store) / 17-06 (search); anonymous caller. *[Codex]*
4. **MEDIUM** — 17-03: resolve or explicitly document the sub-second scheduling persistence mismatch (drop `<1s` claim or add nanosecond repr + E2E). *[Codex]*
5. **MEDIUM** — 17-02: spec `summary_egress_at`/`summary_model` **key deletion** on client summaries + raw-Get test. *[grok]*
6. **MEDIUM** — 17-04: explicit "delete handler `EmbedQuery` on Search rewire" action. *[grok]*
7. **LOW** — 17-04: reconcile unconditional `CodeAborted` must-have with absent conflict sentinel. *[Codex]*
8. **LOW** — 17-01: document intentional empty-email `[email,sub]` widening in secure-phase. *[grok]*

With items 1-6 landed, both grounded reviewers judge the six-plan set execute-ready without another full replan.
