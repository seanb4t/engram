---
phase: 17
round: 8
reviewers: [codex, opencode]
reviewed_at: 2026-07-12T21:32:17Z
plans_reviewed: [17-01-PLAN.md, 17-02-PLAN.md, 17-03-PLAN.md, 17-04-PLAN.md, 17-05-PLAN.md, 17-06-PLAN.md]
models:
  codex: gpt-5.x (codex-cli 0.144.1)
  opencode: openrouter/x-ai/grok-4.5
reviewed_commit: 33ad9efb   # round-7-incorporated plans
result: HIGH_REOPENED   # new HIGH in 17-04 (Task-3 file manifest cannot deliver its acceptance tests) + net-new 17-03 bool/enum MEDIUM; the round-7 17-02 revert is CONFIRMED sound by both reviewers
antigravity: excluded   # -agy (single dash); advisory-only across rounds 5-7, round-7 clean bill missed the HIGH
verified_against_source: true
caveats:
  opencode:
    - "grok-4.5 flushed a full source-grounded review this round (13KB) — the round-7 stdout-flush failure did NOT recur. Its cites are folded in as complementary coverage; the decisive net-new findings are Codex, all orchestrator-verified against the live tree."
  antigravity:
    - "Excluded this round by the -agy flag. Rationale: advisory-only across rounds 5-7 (weighted zero), and its round-7 clean bill MISSED the content/vector desync HIGH. No decisive signal on a 6-plan/360KB prompt."
---

# Cross-AI Plan Review — Phase 17 (Round 8)

Round 8 reviews the ROUND-7-INCORPORATED plans (commit 33ad9efb) against the live tree. **The round-7 fix
held:** both reviewers independently confirm the 17-02 revert (whole-payload `OverwritePayload` -> targeted
`SetPayload`+`DeletePayload`, matching the codebase's own `SetVisibility`) is correct. **But it reopens a HIGH
in a different plan:** 17-04's Task 3 declares only the production file in its `<files>`, so it cannot author
the read-lane regression tests its own acceptance criteria require — the SAME task-fidelity class the round-7
fix closed in 17-05, now recurring in 17-04. Codex also caught a real 17-03 bool-vs-enum mapping bug that grok
missed. Every net-new Codex cite below was orchestrator-verified against source.

## Codex Review
*(codex-cli 0.144.1, source-grounded — all net-new cites orchestrator-verified against the live tree)*

# Overall assessment

The plans are unusually thorough and mostly match the live code. The architecture—explicit caller seam, shared `deps.*` core, narrow store interface, typed conversion/error layers, and real-store plus spy testing—does achieve the phase goal.

I found one high-severity execution gap in 17-04 and several medium correctness issues. I would revise 17-03, 17-04, and 17-05 before execution. Overall risk is **MEDIUM-HIGH until those changes land**, then **MEDIUM** due to the size and security sensitivity of the refactor.

## 17-01 — Owner claims and session migration

### Summary

This plan is source-grounded and handles the owner-key migration seriously. Ordered claims, strict type checks, injective namespace encoding, session invalidation, and the migration runbook cover the main authorization risks.

### Strengths

- The plan correctly treats owner strings as authorization keys. Store filters compare the resolved owner directly at `internal/store/store.go:493-503` and `internal/store/store.go:514-519`; injectivity and email namespace disjointness are therefore security requirements, not formatting choices.
- D-05 strengthens the existing gate correctly. Current `ClaimIdentity` only checks `email_verified` when `email` is configured, at `internal/auth/auth.go:83-95`; the proposed presence/type distinction prevents malformed claims from silently selecting a later bucket.
- Session migration is necessary and well targeted. Current cookies contain only bare `Owner` and `Expiry` (`internal/webauth/session.go:21-29`), and the resolver forwards `sess.Owner` verbatim (`internal/webauth/resolver.go:44-54`). Version rejection prevents old non-email cookies from surviving the namespace change.
- Auto-stamping inside `SessionCodec.Seal` is the right central seam: the actual session mint occurs at `internal/webauth/handlers.go:172-181`.
- Parse/default separation matches current configuration behavior: empty environment values preserve defaults at `internal/config/config.go:176-184`, while explicit CLI empty values can reach the startup guard at `cmd/engram/serve.go:250-270`.

### Concerns

- **LOW — Deployment blast radius:** versioning invalidates every existing UI session, including unaffected email-owner sessions. This is safe and intentional, but operationally broader than the non-email migration itself.
- **LOW — Irreversible provenance loss in legacy migration:** the runbook cannot distinguish two historical claim sources that shared the same bare owner. This follows from owner being stored as one string and compared directly. The plan documents the global rewrite risk, which is the best available mitigation.

### Suggestions

- Add a release-note/checklist item requiring operators to identify all historical non-email claim configurations before migration.
- Test Unicode claim/value inputs against the length-prefixed encoder, since Go `len` counts bytes. The encoding remains injective, but the behavior should be pinned.
- Make unsupported session versions return one generic invalid-session error, avoiding version-specific information on the browser-visible surface.

### Risk assessment

**MEDIUM.** The design is strong, but changing the authorization key and invalidating sessions remains intrinsically sensitive.

---

## 17-02 — Store seam, caller threading, and partial update

### Summary

This is the largest foundational plan and is largely correct. It accurately identifies the concrete-store test break, the inability to preserve vectors with the current `Store.Update`, and the need for canonical mutation results.

### Strengths

- The `memStore` extraction is justified by the live concrete field at `internal/server/tools.go:34-35`.
- Including `DeleteAll` and `ListScopes` is necessary: direct calls exist at `internal/server/tools.go:1139-1143` and `internal/server/connectapi.go:88-93`.
- The concrete-store test break is real. `storeFill` requires `*store.Store`, while tests currently pass `d.st` at `internal/server/summaryqueue_test.go:402` and `:422`; `buildUsageQueue` has the same issue at `internal/server/tools_test.go:1914-1915`.
- The `Content *string` correction is essential. Current `updateArgs.Content` is unconditional (`internal/server/tools.go:507-513`), and `Store.Update` always overwrites content and upserts a vector (`internal/store/store.go:1367-1406`).
- The targeted-payload approach correctly follows the established `SetVisibility` pattern at `internal/store/store.go:1417-1442`. It avoids rewriting stale `content` and `tags`.
- Deleting provenance keys is necessary because decoding treats key presence as meaningful at `internal/store/store.go:433-438`.
- Actor fallback is justified: the cookie resolver does not set `TokenInfo.UserID` (`internal/webauth/resolver.go:54`), whereas bearer authentication does (`internal/auth/auth.go:139-142`).

### Concerns

- **MEDIUM — Partial-success semantics are understated:** the proposed `SetPayload` followed by `DeletePayload` can return an error after the primary visibility/summary/access mutation has already committed. The plan discusses stale provenance metadata, but not the caller-visible ambiguous outcome. A retry can apply the mutation again and increment `AccessCount` again. This is especially relevant because the RPCs are deliberately not marked side-effect-free.
- **MEDIUM — Concurrency still affects usage counters:** the method derives `AccessCount+1` from the earlier `FetchForUpdate` snapshot. Concurrent updates can regress or lose increments. Existing `IncrementAccess` explicitly accepts last-writer-wins behavior at `internal/store/store.go:1445-1453`, so this may be acceptable, but the new method should document the same limitation.
- **LOW — Very wide atomic signature change:** source enumeration shows dozens of direct test calls to the write methods. The plan lists the principal files and compile gates, but this remains a high-churn mechanical step.

### Suggestions

- Add an injected-failure test where `SetPayload` succeeds and `DeletePayload` fails. Assert and document the exact returned error and resulting state.
- Explicitly state that clients may observe “mutation committed but RPC failed” for provenance cleanup failure. If that is unacceptable, return success after logging/telemetry for the cleanup failure, or introduce a later repair sweep.
- Document the usage-counter RMW as soft, last-writer-wins, matching `IncrementAccess`.
- Keep the raw-vector and raw-payload assertions; `Store.Get` cannot prove vector preservation.

### Risk assessment

**MEDIUM-HIGH.** The design is sound, but the change touches most server tests and introduces a two-operation mutation with ambiguous failure semantics.

---

## 17-03 — Proto conversion

### Summary

A dedicated conversion layer is the right design, but the current plan contains one source mismatch and leaves a real timestamp persistence edge unresolved.

### Strengths

- Mask-driven `Content=nil` is the correct second half of the partial-update fix.
- `RFC3339Nano` is appropriate at the adapter/validation boundary because `parseWindow` accepts RFC3339 fractional seconds at `internal/server/tools.go:452-470`.
- Mapping canonical mutation results avoids a second fetch.
- Exact-mapping tests are a better description than symmetric round trips.

### Concerns

- **MEDIUM — The plan incorrectly reuses the Visibility enum conversion for `UpdateMemory.shared`:** `UpdateMemoryRequest.shared` is already a protobuf `bool` at `proto/engram/v1/engram.proto:166-171`. Only `SetVisibilityRequest.visibility` is the enum, at `proto/engram/v1/engram.proto:183-189`. The update adapter must take `&req.Shared` when `"shared"` is in the mask; it should not pass through the enum mapper.
- **MEDIUM — A valid sub-second expiry can succeed and immediately expire:** adapter validation preserves nanoseconds, but storage floors both bounds with `.Unix()` at `internal/store/store.go:319-323` and reconstructs them at whole-second precision at `internal/store/store.go:405-411`. A `not_after` 500 ms in the future can pass `parseWindow`, persist at the beginning of the current second, and be immediately inactive. Calling this merely a documentation limitation permits a silent semantic failure.
- **LOW — Offset preservation is impossible:** protobuf `Timestamp` represents an instant and `.AsTime()` normalizes it. A “non-UTC offset maps correctly” test should assert the equivalent UTC instant, not retention of the original zone offset.

### Suggestions

- Add an exact mapping case for `update_mask=["shared"]` with `shared=false`. Assert a non-nil pointer whose value is false; this is the presence-sensitive case most likely to regress.
- Restrict `visibilityToShared` to `SetVisibilityRequest`.
- Resolve sub-second windows explicitly:

  - reject bounds that cannot survive second-granular persistence;
  - round `not_after` upward and `not_before` according to documented semantics; or
  - persist nanoseconds.

  Silent success followed by immediate expiry should not remain.
- Rephrase timezone tests around instant equivalence.

### Risk assessment

**MEDIUM.** The layer is straightforward once the bool/enum mismatch and timestamp behavior are corrected.

---

## 17-06 — Typed read core

### Summary

This is a strong response to the risk of forcing Connect reads through the existing MCP-shaped helpers. The typed superset preserves transport-specific behavior while converging business logic.

### Strengths

- The need for a superset is confirmed by the current implementations: MCP drops totals and hard-codes cursor mode at `internal/server/tools.go:793-820`, while Connect carries offset, filters, total, and next token at `internal/server/connectapi.go:104-150`.
- Moving defaults to transport boundaries is correct. MCP currently defaults list limit to 20 at `internal/server/tools.go:793-796` and search `k` to 8 at `:854-857`; Connect defaults search `k` to 20 at `internal/server/connectapi.go:162-165`.
- Preserving MCP `CursorMode:true` is required because the store emits a first-page cursor only in cursor mode at `internal/store/store.go:844-867`.
- Separating offset and cursor assertions matches store behavior: offset mode deliberately leaves `nextCursor` empty at `internal/store/store.go:817-823` and `:870-880`.
- Moving date parsing to transport boundaries prevents raw parse errors from reaching the Connect internal-error mapper.
- The plan correctly preserves `subjectFromContext` for display-only instrumentation, which still uses it at `internal/server/instrument.go:78-84`.

### Concerns

- **LOW — Large test migration surface:** there are many direct calls to all five read methods in `internal/server/tools_test.go` and `connectapi_test.go`. The compile gate will catch signature omissions, but not tests whose intent becomes vacuous after type conversion.
- **LOW — `searchDiscovery` remains asymmetrical:** it retains the MCP default internally while `searchMemory` becomes fully default-free. The plan documents this, but it is a future maintenance trap.

### Suggestions

- Add a comment directly on `searchDiscovery` explaining why its default remains internal and identifying the Connect adapter as responsible for supplying 20.
- Prefer named adapter helpers for MCP and Connect request construction so defaults are testable without exercising anonymous tool closures.
- Ensure the relocated malformed-window test invokes the real MCP closure rather than merely calling `parseRFC3339` directly.

### Risk assessment

**LOW-MEDIUM.** Broad but well specified, with clear source-backed regression tests.

---

## 17-04 — Handler wiring and error mapping

### Summary

The production design is correct, but this plan is not executable as currently scoped because Task 3 requires substantial new regression tests while allowing modification only to `connectapi.go`.

### Strengths

- The six handlers are correctly shaped as caller resolution → protoconv → shared `deps.*` → protoconv response → one error mapper.
- Removing handler-local embeddings is mandatory. Current duplicate candidates are at `internal/server/connectapi.go:158` and `:220`; the shared read methods already embed internally at `internal/server/tools.go:870` and `:911`.
- Removing the handler-level usage enqueue is correct. Connect currently enqueues at `internal/server/connectapi.go:209-211`, while `deps.getMemory` already enqueues at `internal/server/tools.go:996-1004`.
- Empty discovery scope must become `CrossSpine=true`: `effectiveDiscoveryScope` otherwise rejects it at `internal/server/tools.go:881-891`, while the current Connect path passes empty scope to the store.
- Preserving Connect `limit=0` as “all” matches `internal/store/store.go:870-880`.
- The CSRF matrix break is real: it currently uses `&deps{}` and expects `CodeUnimplemented` at `internal/server/connectcsrf_test.go:219-250`.

### Concerns

- **HIGH — Task/file manifest cannot deliver its own acceptance tests:** Task 3 lists only `internal/server/connectapi.go`, but requires new or changed tests for:

  - SearchMemories default 20;
  - SearchDiscoveries default 20;
  - empty-scope discovery;
  - exactly-once usage enqueue;
  - `limit=0` returning more than 20;
  - malformed created windows.

  Existing relevant tests live in `internal/server/connectapi_test.go`—for example malformed `created_after` at `:522-530`—and cookie-lane isolation lives in `internal/server/connectapi_cookie_test.go:21-92`. Neither file is in Plan 17-04’s task or top-level file list. A task-scoped executor cannot satisfy the acceptance criteria.
- **MEDIUM — Cancellation/deadline mapping is missing:** the proposed `connectError` maps unknown errors to `CodeInternal`, but handlers pass request context into embeddings and store operations. `context.Canceled` and `context.DeadlineExceeded` should map to `CodeCanceled` and `CodeDeadlineExceeded`, not Internal.
- **LOW — ListScopes remains an architectural exception:** the current direct store call is at `internal/server/connectapi.go:88-93`. The exception is defensible because there is no MCP counterpart, but the final architecture documentation should make it explicit.

### Suggestions

- Add at least these files to Task 3 and the plan header:

  - `internal/server/connectapi_test.go`
  - `internal/server/connectapi_cookie_test.go`
  - potentially a focused usage test file if the exactly-once assertion does not fit cleanly.

- Add `context.Canceled` and `context.DeadlineExceeded` cases to the production mapper and its table tests.
- Add an HTTP-level test for at least one successful write through the real interceptor chain, not only direct method calls.
- Keep the AST/static delegation assertion in 17-05, but also make the production handlers short enough that direct inspection remains obvious.

### Risk assessment

**HIGH until the file manifest is corrected; MEDIUM afterward.** This is the security-sensitive centerpiece, and its current task declaration cannot produce its promised regression coverage.

---

## 17-05 — Parity, leak tests, and CI gate

### Summary

The layered proof strategy is good: store spy for behavioral trace, AST assertion for named delegation, real Qdrant for authorization behavior, and explicit cross-owner leak tables. The current wording overstates what the spy itself can prove.

### Strengths

- Independent per-lane fixtures are essential for destructive operations and are correctly required.
- Mapping direct domain errors through the production `connectError` is the correct apples-to-apples comparison.
- The AST assertion closes a real ambiguity: `storeMemory` and `scheduleMemory` both ultimately call `MintShortID` and `Upsert` in `internal/server/tools.go:634-694`; store-level traces alone cannot identify which wrapper ran.
- The split short-ID/UUID assertions reflect the real leak invariant. Existing code already re-wraps original inputs at `internal/server/tools.go:930-936`, `:1019-1022`, and `:1050-1060`.
- Real-store coverage already exercises all three cross-owner mutations at `internal/server/tools_test.go:1700-1769`; making Qdrant mandatory in CI prevents those tests from silently skipping.
- The current skip hole is real: `TestMain` continues after container startup failure at `internal/server/tools_test.go:127-140`, and `testDeps` skips at `:189-196`.

### Concerns

- **MEDIUM — The spy cannot record the `deps.*` method invoked:** `memStore` sits below `deps`; it can record store calls and subjects/owners, not which wrapper called it. The plan sometimes says the spy proves both lanes invoked the same `deps` method. Only the AST assertion proves that. If implemented literally, the test may add a tautological wrapper label supplied by the test itself.
- **LOW — Invalid `ENGRAM_REQUIRE_QDRANT` values can silently disable the gate:** if `requireQdrant()` uses `strconv.ParseBool` and treats parse errors as false, a CI typo such as `"treu"` restores silent skipping.
- **LOW — CI becomes dependent on Docker Hub/testcontainers availability:** this is intentional fail-closed behavior, but it may add infrastructure flakiness and startup cost.

### Suggestions

- Reword the proof precisely:

  - spy proves identical store trace, subject, arguments, and effects;
  - AST proves the named Connect handler calls the named `deps.*` method;
  - real Qdrant proves actual authorization behavior.

- Make `requireQdrant` return `(bool, error)` and fail startup on any non-empty invalid value.
- Consider pinning the Qdrant image by digest in CI.
- Ensure the real Qdrant gate explicitly runs `TestShortIDCrossOwnerVisibility`, not only the read-only `TestConnectCookieLaneIsolation`.

### Risk assessment

**MEDIUM.** The test strategy is strong after clarifying the spy’s evidentiary boundary and hardening environment parsing.

# Final recommendation

Revise before execution:

1. Fix 17-03’s `UpdateMemory.shared` bool/Visibility mismatch and add a false-presence test.
2. Decide how sub-second scheduled expiry is represented or rejected.
3. Add the missing test files to 17-04’s task and plan manifests.
4. Map canceled/deadline errors in `connectError`.
5. Document and test the partial-success behavior of the payload/provenance two-operation update.
6. Reword 17-05 so the spy, AST check, and real store each claim only what they actually prove.
7. Fail on malformed `ENGRAM_REQUIRE_QDRANT`, rather than treating it as false.

With those changes, the six-plan wave structure is credible and should satisfy `REQ-connect-write-authz-parity` without duplicating authorization logic.

---

## OpenCode Review
*(openrouter/x-ai/grok-4.5 — full source-grounded review this round; the round-7 stdout-flush failure did not recur)*

I'll verify the plan claims against the live codebase, starting with the core identity, store, and Connect surfaces the plans depend on.
Verifying critical plan claims against line numbers and live behavior in the repo.
Checking a few high-risk spots the earlier rounds still may mis-state.
# Phase 17 Plan Review (source-verified)

Verified against live tree: `internal/auth/auth.go`, `internal/server/{tools,connectapi,identity,summary,...}.go`, `internal/store/store.go`, `internal/webauth/{session,resolver,handlers,oidc}.go`, `internal/config/*`, `cmd/engram/serve.go`, `proto/engram/v1/engram.proto`, `.github/workflows/ci.yaml`, and the six plan files.

---

## Overall Summary

These plans are unusually grounded: almost every cited line still matches, prior-round landmines (content mask blanking, empty Connect `Actor`, nil-store CSRF/negative matrices, whole-payload race, memStore `DeleteAll`/`ListScopes`, `errStaleSummary` reuse, fail-closed Qdrant CI) are closed with evidence, and wave order has a coherent compile story. Residual risk is **LOW–MEDIUM** and mostly execution discipline (wide test-signature rewiring; ordered-claim security tests must stay exhaustive), not missing design.

---

## 17-01 — Ordered owner-claim + injective namespace + session versioning

### Summary
Security-critical authz-key plan that matches live seams (`ClaimIdentity` at auth.go:83–96 is single-claim; cookie mints via `handlers.go:172–175`; resolver forwards bare owner at resolver.go:54). Round-6 Task1 isolation gate is correct for Go package compile.

### Strengths
- Correct choke point: both lanes share `ClaimIdentity` (`auth.go:134`, `webauth/oidc.go:78`).
- Length-prefixed encoding fixes a real authz collision (store compares owner strings at store.go:497/:517).
- Session versioning is necessary: live cookies hold bare `Session.Owner` only (`session.go:26–28`).
- Test call sites called out (`handlers_test.go:159`, `oidc_exchange_test.go:99–148`) match source.
- Empty-ENV vs CLI empty correctly distinguished (`config.go:178–182`, `registry.go:52`).

### Concerns
- **MEDIUM** — Current `ClaimIdentity` uses `raw[ownerClaim].(string)` and, for `email` with `email_verified=true` and a **non-string** email value, returns `owner=""` with **nil error** (auth.go:86–96). Plan’s reject-malformed-type behavior is a **behavior change**, not just list support — tests must pin this or deployments with weird IdP claim types break differently.
- **LOW** — Empty-string email under `[email,sub]` deliberately falls through (T-17-15 accept). Correct and documented; secure-phase must keep it distinct from D-05.
- **LOW** — Plan forces global UI re-login (version≠1 → reject). Intentional; operators only need the migrate runbook for **non-email** `ENGRAM_OWNER_CLAIM` history.

### Suggestions
- Add one regression: single-claim `["email"]` + non-string email + `email_verified:true` → reject (not empty owner).
- Keep fail-fast parser errors at startup only (already specified).

### Risk: **LOW–MEDIUM**

---

## 17-02 — memStore + payload-only update + caller + write threading

### Summary
Foundations plan correctly diagnoses live risks: `Content string` on `updateArgs` (tools.go:509) vs mask promise (`engram.proto:156`); unconditional re-embed in `updateMemory` (tools.go:972–980); `store.Update` always takes content/vector (store.go:1367–1406); `SetVisibility` is targeted SetPayload (store.go:1417–1441). Round-7 targeted two-op design matches established store pattern.

### Strengths
- Payload-only method + usage bump (`AccessCount++` at store.go:1382–1384) closes real Connect shared/summary-only gap.
- Provenance clear via **DeletePayload**, not zero-time write — decoder reads key presence (store.go:433–438).
- `errStaleSummary` already at summary.go:16/:34 — plan correctly forbids redeclaration.
- Concrete-store test break is real: `storeFill(d.st,…)` at summaryqueue_test.go:402/:422 and `buildUsageQueue(..., d.st, …)` at tools_test.go:1915.
- `embed_wiring_test.go:52` listed — must get caller for compile.

### Concerns
- **MEDIUM** — Blast radius of rewrite: dozens of direct `d.storeMemory/updateMemory/…` test call sites (tools_test.go, rules_test.go, etc.) still use ctx identity. Plan relies on compile + `rg`; miss any and CI is red mid-wave. Expected, but large.
- **MEDIUM** — Plan text says MCP passes `&a.Content` after `Content *string`; once the field is already `*string`, registration just forwards `a`. Wording nit — risk is MCP jsonschema: keep content **required** on MCP tool schema so MCP never omits content.
- **LOW** — `mutationResult` ShortID empty on legacy shorts until backfill; proto already returns empty short_id for those — OK if documented.
- **LOW** — Accepted non-atomicity if DeletePayload fails after SetPayload (stale provenance only). Acceptable vs whole-payload race; document in method godoc as specified.

### Suggestions
- In Task 3 AC: assert MCP `update_memory` jsonschema still requires content (no `omitempty` path for MCP clients).
- Ensure `memStore` method set matches every production `d.st.*` call (includes moral `Get`/`OwnedOrAbsent` used by storeDiscovery/storeRule).

### Risk: **MEDIUM** (mostly volume/execution)

---

## 17-03 — protoconv

### Summary
Tight, well-scoped TDD plan. Live `parseWindow` uses `time.Parse(time.RFC3339, …)` (tools.go:453–460), which accepts fractional seconds; store floors windows to Unix seconds (store.go:320+) — round-5 scoped guarantee is accurate.

### Strengths
- Nil Content on missing mask path correctly closes second half of landmine 2.
- Single Visibility map reused for both write paths.
- Explicit non–end-to-end claim for sub-second scheduling prevents false UAT.

### Concerns
- **LOW** — Ensure result mappers cover all five non-delete responses; Delete remains empty (proto is empty).
- **LOW** — Do not invent reverse Timestamp fields for write responses not in contract.

### Suggestions
- One table cell: mask `["shared"]` only → Shared non-nil, Content/Tags/Summary nil.

### Risk: **LOW**

---

## 17-04 — Handlers + connectError + spy + read rewire

### Summary
Live code confirms: **zero** write methods on `engramAPI` (only List/Search/Get/SearchDiscoveries, connectapi.go:88–233); embeds Unimplemented; CSRF happy path still expects `CodeUnimplemented` (connectcsrf_test.go:225/:250); negative matrix uses `d := &deps{}` (:64); GetMemory double-enqueue hazard is real (connectapi.go:211 + tools.go:1003); SearchDiscoveries empty scope vs `effectiveDiscoveryScope` (tools.go:884–889) is a real rewire regression risk.

### Strengths
- Same-plan fix for CSRF + negative matrix + handsinator wiring — prevents deterministic CI red.
- `connectError` maps typed sentinels including `ErrAmbiguousShortID` (store.go:56, ResolvePointID :1217).
- Search k=20 defaults preserved at adapters (connectapi.go:162–164, :224–226 vs MCP 8 at tools.go:855–856/:904–905).
- Explicit deletion of handler-local `EmbedQuery` (connectapi.go:158/:220).
- ListScopes exception is the right OQ2 resolution.

### Concerns
- **MEDIUM** — After wiring, authenticated-valid cells of `TestWriteRPCNegativeMatrix` are no longer “stub shape” tests: UpdateMemory with id `"some-id"` (negative_test.go:120–124) will hit real domain logic. Spy scripting must distinguish “valid wire shape” success vs NotFound vs validation — carefully per RPC.
- **MEDIUM** — Connect time fields are **strings** (proto:59–60, :78–79), not Timestamps. Plan’s “parse to time.Time at boundary” is correct, but must keep using `parseRFC3339` (or equivalent), not `timestamppb`.
- **LOW** — schedule window strings still go through RFC3339Nano → `parseWindow` string parse. Fine if 17-03 GREEN + 17-02 ErrInvalidArgument wrap land first (depends_on 17-03/17-06).

### Suggestions
- For negative-matrix authenticated-valid: prefer spy-scripted **success** for Store*; for Update/Delete/SetVisibility with dummy IDs prefer explicit NotFound → CodeNotFound so cells stay stable.

### Risk: **MEDIUM** (wiring + matrix expectations)

---

## 17-05 — Parity, leak tables, fail-closed Qdrant

### Summary
Acceptance gate for REQ-connect-write-authz-parity. Live `TestMain` действительно continues on container failure (tools_test.go:136–139); CI runs bare `go test ./...` (ci.yaml:33) with no Qdrant service — silent skip risk is real. Actor asymmetry MCP UserID (auth.go:139–140) vs cookie no UserID (resolver.go:54) is correctly non-equal for non-email.

### Strengths
- Spy + AST delegation closes shared MintShortID+Upsert ambiguity between store/schedule.
- Per-lane fixtures prevent order-dependent false fails.
- Split short_id vs UUID leak tables match the real DEC-xa6 surface (GetMemory analog → write RPCs).
- `ENGRAM_REQUIRE_QDRANT` + unit-tested `requireQdrant()` is a clean local/CI split.
- Maps domain errors through production `connectError` before CodeOf — correct (raw CodeOf on plain error is Unknown).

### Concerns
- **MEDIUM** — CI will start failing hard if Docker/testcontainers is flaky on runners. Prefer ensuring image pull cache or document on-call response; optional service container remains fallback.
- **LOW** — AST assertion on method name strings is brittle under refactors/formatting; acceptable if consistently named.
- **LOW** — Parity fixtures on spy are not a substitute for store authz (plan correctly keeps real isolation as gate).

### Suggestions
- In CI, pin Qdrant image already used by TestMain (`qdrant/qdrant:v1.18.2`, tools_test.go:135).
- Keep at least one real-Qdrant write isolation case (ownership) beyond cookie read isolation if capacity allows.

### Risk: **LOW–MEDIUM**

---

## 17-06 — Typed core read contract

### Summary
Correctly rejects naive rewire onto today’s `listMemory`/`searchMemory`: list drops total (tools.go:809 discards middle return), hard-codes `CursorMode: true` (:815), defaults limit 20 (:794–796), returns `[]any` via `shapeRecall`; search defaults k=8 (:855–856). Connect List carries offset/categories/visibility/total/cursor (connectapi.go:124–149). Round-3/4/6 fixes about CursorMode, split paging modes, limit=0, and `TestListMemoryRejectsBadWindow` are source-accurate.

### Strengths
- Superset core is the only honest D-07 for reads.
- Per-lane k defaults (MCP 8 / Connect 20) and **retaining** searchDiscovery’s internal k=8 default is necessary for 17-04 finding 7.
- Moving parseRFC3339 out of core prevents CodeInternal misclassification.
- `subjectFromContext` retained for `identityForLog` (instrument.go:81–82) — correct.

### Concerns
- **MEDIUM** — Wave-2 parallel with 17-03 is fine (disjoint files), but tools.go is also touched by 17-02 (Wave 1). If Wave 1 is incomplete, 17-06 cannot safely start — depends_on on 17-02 is load-,bearing; treat 17-02 merge as a hard gate.
- **LOW** — `listScheduled` still has limit-default 20 (tools.go:824–826). Out of Connect rewire; leave alone or note explicitly to avoid “uniform defaults” drive-bys.

### Suggestions
- Explicit AC: `grep` shows no limit default inside `listMemory` core while list_memory MCP closure still sets 20.
- When updating `TestRerankParityMCPAndConnect`, do not reintroduce `shapeRecall` on the deps path.

### Risk: **LOW–MEDIUM**

---

## Cross-plan / dependency assessment

| Topic | Verdict |
|--------|---------|
| Wave 1 parallel (01 ∥ 02) | Safe — disjoint packages |
| Wave 2 (03 ∥ 06) after 02 | Safe — protoconv vs tools; 03 only needs mutationResult from 02 |
| Wave 3 (04) after 03+06 | Correct |
| Wave 4 (05) after 04 | Correct |
| Phase goals SC1–SC5 | Covered when all plans land |
| Zero new deps | Held |
| Security /gsl-secure-phase | Still required (D-04/05/06, CSRF already Phase 16) |

### Residual risks (phase-wide)
1. **Execution volume** — huge mechanical signature/(test) rewire; compile is the safety net, but expect long red cycles if tasks are split poorly.
2. **Owner-encoding migration** — correct design; ops risk only for non-email historical owners (runbook must ship).
3. **CI Qdrant fail-closed** — improves parity integrity; may surface previously green-but-skipped CI flake.

### What is **not** a gap (verified)
- Content mask blanking is real and fixed by 02+03.
- Connect Actor empty is real (`resolver.go:54`) and fixed by caller fallback.
- Polluting whole-payload overwrite race is real and correctly reverted.
- CSRF/negative Unimplemented landmines are real and co-located with wiring.
- Proto tags-only doc is wrong today (`engram.proto:156`) and gets corrected in 02.
- Idempotency ban already exists (Taskfile:141–142 / ci.yaml:124–127).

---

## Verdict

**Plans are ready to execute** after round 1–7 hardening. No remaining BLOCKER against source.

**Overall risk: LOW–MEDIUM** — architecture and security design are sound; risk is implementation width and CI/harness tightness, not missing parity strategy.

Optional pre-execute nits only:
1. Clarify MCP `Content *string` vs `&a.Content` wording in 17-02.
2. Pin negative-matrix expected codes per RPC once spy-backed.
3. Document CI behavior under `ENGRAM_REQUIRE_QDRANT=1` for maintainers.

---

## Antigravity Review

**Excluded this round** (`-agy`). Antigravity was advisory-only across rounds 5-7 (weighted zero), and its
round-7 clean bill missed the content/vector desync HIGH that Codex caught. On a 6-plan / ~360KB prompt it
adds no decisive signal, so it was dropped from round 8 by the single-dash `-agy` flag.

---

## Consensus Summary

Codex reviewed the round-7-incorporated plans (commit `33ad9efb`) against the live tree; every net-new
`file:line` cite below was independently re-verified by the orchestrator and **confirmed**. grok-4.5
(openrouter/x-ai/grok-4.5) **succeeded this round** — it flushed a full source-grounded review (the round-7
stdout-flush failure did not recur), and its findings are folded in below as complementary coverage.
Antigravity was **excluded** this round (`-agy`): across rounds 5–7 it was advisory-only, and its round-7
clean bill missed the HIGH — it adds no decisive signal on a 6-plan/360KB prompt.

**Verdict: NOT execute-ready — 1 new HIGH + net-new MEDIUMs.** The round-7 fix HELD: both reviewers confirm
the 17-02 revert (whole-payload `OverwritePayload` → targeted `SetPayload`+`DeletePayload`) is correct and
matches the codebase's own `SetVisibility` (grok: *"whole-payload overwrite race is real and correctly
reverted"*; codex: *"targeted-payload approach correctly follows the established `SetVisibility` pattern"*).
But round 8 surfaces a **new HIGH in 17-04** — Task 3's file manifest cannot deliver its own acceptance
tests — **the same task-fidelity class the round-7 fix closed in 17-05**, now recurring in 17-04. Plus a
real 17-03 correctness bug (bool-vs-enum) that grok missed and codex caught.

### THE headline finding

**[HIGH — 17-04, Codex, orchestrator-CONFIRMED] Task 3's `<files>` cannot produce its own acceptance tests.**
Task 3 (read-lane rewire) declares `<files>internal/server/connectapi.go</files>` (17-04-PLAN.md:227) — the
production file only. But its `<acceptance_criteria>` require **five+ new/changed regression tests**:
SearchMemories default-20, SearchDiscoveries default-20 (finding 7), empty-scope→`CrossSpine=true`,
GetMemory exactly-once usage bump, `limit=0`-returns-all, and malformed-window→`CodeInvalidArgument`. These
belong in `internal/server/connectapi_test.go`, which **exists** and holds the analog `TestSearchMemories`/
`TestListMemories` tests today — but it is absent from Task 3's `<files>` **and** from 17-04's plan-level
`files_modified` (which lists only `connectapi.go`, `fakestore_test.go`, `connectapi_negative_test.go`,
`connectcsrf_test.go`; 17-04-PLAN.md:9-12). A task-scoped executor / commit tooling editing only
`connectapi.go` **cannot author these tests**, so Task 3 cannot satisfy its own gate. `connectapi_cookie_test.go`
appears only as a `read_first` (17-04-PLAN.md:234), never in a `<files>`.
→ *Fix:* add `internal/server/connectapi_test.go` (and, if the isolation assertion is touched,
`internal/server/connectapi_cookie_test.go`) to Task 3's `<files>` **and** to 17-04's `files_modified`.

**Meta-pattern (worth fixing systemically):** rounds 5, 7, and 8 each found the SAME class of defect —
a task's scoped `<files>` (or a plan's `files_modified`) omitting a test file the task must modify. Round 5:
unlisted test call sites. Round 7: 17-05 Task 2 (`tools_test.go` + `ci.yaml`). Round 8: 17-04 Task 3
(`connectapi_test.go`). The planner reliably lists production-file edits but under-declares the test files its
acceptance criteria imply.

### Other net-new (Codex, orchestrator-CONFIRMED)

**[MEDIUM — 17-03] `UpdateMemory.shared` is a proto `bool`, not the `Visibility` enum — the plan mis-maps it.**
`UpdateMemoryRequest.shared` is `bool shared = 3` (proto/engram/v1/engram.proto:168); only
`SetVisibilityRequest.visibility` is `Visibility visibility = 2` (engram.proto:185). But 17-03 instructs "a
single Visibility↔shared mapping reused by **both** the SetVisibility and **UpdateMemory-shared** paths"
(17-03-PLAN.md:153, and the grep-1 acceptance at :158). The update request has no `Visibility` field to map
from, so an executor following the plan literally either won't compile or mis-maps. → *Fix:* map the update
path as `mask has "shared" → updateArgs.Shared = &req.Shared` (bool→`*bool`), and reserve the `Visibility`
enum↔bool mapping to the `SetVisibility` path only. Add an exact-mapping test for `update_mask=["shared"]`
with `shared=false` asserting a non-nil `*bool` whose value is false (the presence-sensitive case).

**[MEDIUM — 17-04] `connectError` omits `context.Canceled` / `context.DeadlineExceeded` mapping.**
The 17-04 `connectError` design (17-04-PLAN.md:175) maps nil→nil, `ErrNotFound`→NotFound,
`ErrInvalidArgument`→InvalidArgument, `errRuleImmutable`/`errStaleSummary`/`ErrAmbiguousShortID`→
FailedPrecondition, **else→CodeInternal**. Handlers pass request `ctx` into embeddings/store ops, so a
cancelled or timed-out request currently maps to `CodeInternal` instead of `CodeCanceled` /
`CodeDeadlineExceeded`. → *Fix:* add `errors.Is(err, context.Canceled)`→`CodeCanceled` and
`context.DeadlineExceeded`→`CodeDeadlineExceeded` arms (before the else) + table cells.

**[MEDIUM — 17-03] Sub-second scheduled expiry can succeed then immediately expire (silent semantic failure).**
Adapter validation preserves nanoseconds, but storage floors both bounds via `.Unix()` (store.go:320/:323)
and reconstructs at whole-second precision (store.go:405-411). A `not_after` ~500 ms in the future passes
`parseWindow`'s future check, persists at the start of the current second, and is immediately inactive. 17-03
scopes its test to `parseWindow` acceptance only and frames persistence as merely "second-granular"
(17-03-PLAN.md:129) — codex argues that permits a silent success-then-expiry. → *Fix:* reject bounds that
cannot survive second-granular persistence, OR round `not_after` up / `not_before` down per documented
semantics, OR persist nanoseconds. (Re-raise of the round-5 MED with sharper framing; grok rates 17-03 LOW.)

**[MEDIUM — 17-02] Partial-success + usage-counter concurrency semantics understated.**
The targeted two-op update (`SetPayload` then `DeletePayload`) can return an error **after** the primary
visibility/summary/access mutation commits; a retry re-applies and re-increments `AccessCount`. The plan
documents "stale provenance metadata" but not the caller-visible "mutation committed but RPC failed"
ambiguity — relevant because the write RPCs are deliberately NOT side-effect-free. Also the `AccessCount+1`
derives from the `FetchForUpdate` snapshot, so it is last-writer-wins under concurrency, matching existing
`IncrementAccess` (store.go:1445-1453) — but the new method should say so. → *Fix:* document partial-success
+ soft last-writer-wins RMW in the method godoc; add an injected-failure test (SetPayload OK, DeletePayload
fails) asserting the exact returned error and state.

**[MEDIUM — 17-05] The store spy's evidentiary boundary is overstated.**
`memStore` sits **below** `deps`, so the spy records store calls + subject/owner/args, but NOT which
`deps.*` wrapper invoked it. The plan sometimes claims the spy proves both lanes invoked the same `deps`
method — only the AST assertion proves that; a spy "wrapper label" would be tautological (test-supplied).
→ *Fix:* reword the proof precisely — spy proves identical store trace/subject/args/effects; AST proves the
named Connect handler calls the named `deps.*` method; real Qdrant proves actual authorization behavior.

**[LOW — 17-05] Malformed `ENGRAM_REQUIRE_QDRANT` silently disables the fail-closed gate.**
If `requireQdrant()` uses `strconv.ParseBool` and treats a parse error as false, a CI typo like `"treu"`
restores silent skipping — defeating the round-6/7 hardening. → *Fix:* have `requireQdrant()` return
`(bool, error)` and fail startup on any non-empty invalid value (do not coerce to false). (Refines the
round-7 LOW that added the helper.)

**[LOW — 17-01] Encoder byte-length + session-error hygiene (Codex).** Pin Unicode claim/value behavior
against the length-prefixed encoder (Go `len` counts bytes; encoding stays injective but pin it); return one
GENERIC invalid-session error for unsupported versions (no version-specific info on the browser surface).

### grok (openrouter/x-ai/grok-4.5) — complementary, all LOW–MEDIUM, no blocker

grok's verdict: **"Plans are ready to execute after round 1–7 hardening. No remaining BLOCKER against source.
Overall LOW–MEDIUM."** Its net-new / convergent points:
- **[MED — 17-01]** Current `ClaimIdentity` returns `owner=""`+**nil error** for a non-string `email` value
  (`email_verified=true`) at auth.go:86-96; the plan's reject-malformed-type behavior is a *behavior change*,
  not just list support — add a regression: single-claim `["email"]` + non-string email + verified → reject
  (not empty owner). (Echoes the round-3 HIGH; converges with codex's 17-01 D-05 area.)
- **[MED — 17-04]** Post-wiring, authenticated-valid `TestWriteRPCNegativeMatrix` cells hit real domain logic;
  spy scripting must distinguish valid-shape success vs NotFound vs validation **per RPC**. Connect time
  fields are **strings** (proto:59-60/:78-79), not Timestamps — keep using `parseRFC3339`, not `timestamppb`.
  (Converges with codex's 17-04 scrutiny.)
- **[MED — 17-05]** CI fails hard if Docker/testcontainers is flaky on runners — pin the Qdrant image
  (`qdrant/qdrant:v1.18.2`, already used by TestMain at tools_test.go:135) / cache pulls / document on-call.
  (Converges with codex's 17-05 Docker-dependency LOW.)
- **[MED — 17-06]** 17-02 touches `tools.go` (Wave 1); 17-06 also touches `tools.go` (Wave 2) — treat the
  17-02 merge as a hard gate for 17-06 (the `depends_on: 17-02` is load-bearing).
- Confirms the phase design end-to-end: content-mask blanking (fixed by 02+03), empty Connect Actor
  (resolver.go:54, fixed by caller fallback), CSRF/negative Unimplemented landmines (co-located with wiring),
  and the whole-payload race (**correctly reverted**) are all real-and-handled.

### Verified sound (round-7 state that holds — both reviewers, no net-new design flaw)
- **17-02 revert (round-7 HIGH):** targeted `SetPayload` + `DeletePayload` matching `SetVisibility`
  (store.go:1417-1442); provenance cleared by `DeletePayload` (decoder reads key presence, store.go:433-438);
  vector preserved (payload-only op supplies no vector). **The round-7 fix is CONFIRMED correct by both.**
- **17-01** ordered-claim authz-key design, length-prefixed injective encoding, session versioning, legacy-cookie
  reject — sound; residual items are refinements, not flaws.
- **17-06** typed-superset read core (MCP drops total + hard-codes CursorMode; Connect carries offset/total/cursor)
  — the only honest D-07 for reads; source-accurate.
- Idempotency-ban gate already exists (Taskfile:141-142 / ci.yaml:124-127); zero new dependencies held.

### Divergent Views
- **17-03 bool/enum:** codex MEDIUM (orchestrator-CONFIRMED against proto) vs grok silent (missed it). Codex correct.
- **17-03 sub-second expiry:** codex MEDIUM (silent success→immediate-expiry) vs grok LOW ("explicit non-end-to-end
  claim prevents false UAT"). Codex's edge case is real; severity is arguable.
- **Overall severity:** codex MEDIUM-HIGH with one HIGH (17-04 manifest) vs grok LOW-MEDIUM / no BLOCKER. As in
  prior rounds, codex is the more adversarial, higher-signal reviewer; weight it accordingly.

### Required round-8 changes (rank-ordered, all orchestrator-CONFIRMED unless noted)
1. **HIGH — 17-04:** add `internal/server/connectapi_test.go` (+ `connectapi_cookie_test.go` if touched) to
   Task 3's `<files>` and to 17-04's `files_modified` so the read-lane acceptance tests can actually be
   authored. *[Codex]*
2. **MEDIUM — 17-03:** map `UpdateMemory` shared via `mask has "shared" → updateArgs.Shared = &req.Shared`
   (bool→`*bool`); reserve the `Visibility` enum mapping to `SetVisibility` only; add the `shared=false`
   presence test. *[Codex]*
3. **MEDIUM — 17-04:** add `context.Canceled`→`CodeCanceled` and `context.DeadlineExceeded`→
   `CodeDeadlineExceeded` arms to `connectError` + table cells. *[Codex]*
4. **MEDIUM — 17-03:** resolve sub-second scheduled expiry explicitly (reject / round / persist-nanos), not a
   silent success-then-expire. *[Codex; grok rates 17-03 LOW]*
5. **MEDIUM — 17-02:** document partial-success ("mutation committed but RPC failed") + soft last-writer-wins
   `AccessCount` RMW; add the SetPayload-ok/DeletePayload-fail injected test. *[Codex]*
6. **MEDIUM — 17-05:** reword the proof so spy (store trace), AST (named delegation), and real-Qdrant (authz)
   each claim only what they prove. *[Codex]*
7. **MEDIUM — 17-01:** add the non-string-`email` reject regression test (behavior-change pin). *[grok; converges
   with codex D-05]*
8. **LOW — 17-05:** `requireQdrant()` returns `(bool, error)`, fail on malformed `ENGRAM_REQUIRE_QDRANT`
   (don't coerce to false). *[Codex]*
9. **LOW — 17-01:** Unicode-encoder test + generic session-version error. *[Codex]*
10. **LOW — 17-06:** treat 17-02 merge as a hard gate (depends_on already correct — belt-and-suspenders). *[grok]*
11. **LOW — 17-05:** pin `qdrant/qdrant:v1.18.2` in CI. *[grok+Codex]*

Item 1 (HIGH) is a genuine execution-fidelity blocker — Task 3 cannot deliver its acceptance as scoped — and
reprises the exact class the round-7 fix closed in 17-05. Item 2 (17-03 bool/enum) is a real plan-correctness
bug that forces executor deviation. The 17-02 round-7 revert is CONFIRMED sound and needs no change. Land
items 1–3 (and ideally 4–6) via `/gsd-plan-phase 17 --reviews`, then execute.
