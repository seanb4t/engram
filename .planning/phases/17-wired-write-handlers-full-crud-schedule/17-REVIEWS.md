---
phase: 17
reviewers: [codex, opencode]
reviewed_at: 2026-07-12T15:33:41Z
plans_reviewed: [17-01-PLAN.md, 17-02-PLAN.md, 17-03-PLAN.md, 17-04-PLAN.md, 17-05-PLAN.md, 17-06-PLAN.md]
review_round: 3
supersedes_round: 2
opencode_model: google/gemini-3.5-flash
---

# Cross-AI Plan Review — Phase 17 (Round 3)

Reviewers: **Codex** (codex-cli 0.144.1, model `gpt-5.6-sol`) and **OpenCode** (opencode 1.17.15, model `google/gemini-3.5-flash`). This is the **third** review round — it re-reviews the six plans after two prior revision rounds (round 1 fixed the design; round 2 fixed the compile blockers + the D-06 authz-key migration hazard). Both ran source-grounded inside the git working tree; the plans are not yet executed, so cited code reflects the pre-change state.

> **Verdict split:** Codex (source-grounded, adversarial) rates **HIGH** — it found two genuinely-new regressions the refactor introduced (a malformed-email-type fallthrough on the authz key, and a missing MCP cursor-mode mapping) plus several medium test/payload gaps. OpenCode/gemini rates **LOW** ("exceptionally solid") and endorses every fix with only minor polish, but did not surface Codex's two HIGH items. Weight Codex's HIGH findings — they are on the authorization key and the public MCP pagination contract. The trend across rounds is convergence: round 1 (~8 HIGH) → round 2 (2 blockers + HIGH) → round 3 (2 narrow HIGH).

---

## Codex Review

## Summary

The third-round plans are substantially stronger and mostly source-grounded. They correctly address the prior compile blockers, partial-update data-loss risk, session-cookie migration hazard, read-contract regression, timestamp precision, and by-id response/error behavior. However, two high-impact gaps remain: malformed-but-present email claims can still fall through to a later owner claim, violating the stated fail-closed rule; and Plan 17-06 does not explicitly preserve MCP's always-on cursor mode when moving list construction into the transport adapter. Several medium-risk test and payload details also need tightening. Overall risk remains **HIGH** until those are corrected because one affects the authorization key and the other can silently break MCP pagination.

## Strengths

- The session-version migration addresses a real authorization-key rollout hazard. The current cookie contains only the already-resolved bare `Owner` (`internal/webauth/session.go:21`), and the resolver forwards it verbatim as `owner_claim` (`internal/webauth/resolver.go:44`, `:54`). Rejecting version-zero legacy cookies before forwarding them correctly prevents old bare non-email identities from coexisting with newly encoded identities.

- The hardened owner encoding is appropriately motivated by the store's direct string-based authorization key. Owner values are stored directly in payload (`internal/store/store.go:304`, `:315`) and indexed as a tenant keyword (`internal/store/store.go:263`, `:275`). A length-prefixed `(claim,value)` encoding plus reserved-prefix rejection is therefore materially safer than delimiter concatenation.

- Plan 17-02 correctly identifies the partial-update failure in the current code. `updateArgs.Content` is presently unconditional (`internal/server/tools.go:507`), `updateMemory` compares and embeds that unconditional value (`internal/server/tools.go:957`, `:972`), and `Store.Update` always overwrites `cur.Content` (`internal/store/store.go:1367`, `:1379`). Making content presence-signaled and adding a vector-preserving payload path is necessary to honor the proto mask.

- The payload-only path correctly preserves the shipped usage semantics in principle. Existing full updates increment `AccessCount` and stamp `LastAccessedAt` (`internal/store/store.go:1379`), while the established vector-preserving primitive uses `SetPayload` (`internal/store/store.go:1454`, `:1470`). The plan explicitly carries that behavior into shared/summary-only updates.

- The `memStore` compile blockers are real and now accounted for. `deps.st` is currently concrete (`internal/server/tools.go:34`), while production summary and usage builders intentionally accept `*store.Store` (`internal/server/summaryqueue.go:119`, `internal/server/tools.go:265`). Existing tests pass `d.st` into those concrete seams (`internal/server/summaryqueue_test.go:402`, `:422`, `internal/server/tools_test.go:1915`). `testDepsWithStore` is a clean fix that avoids bloating the interface.

- Plan 17-06 correctly avoids a least-common-denominator read rewire. Connect currently carries offset, categories, visibility, exact total, cursor mode, and next-page token (`internal/server/connectapi.go:124`, `:145`), while `deps.listMemory` currently drops total and returns MCP-shaped `[]any` (`internal/server/tools.go:793`, `:809`, `:820`). A typed superset core is the right convergence point.

- RFC3339Nano is the correct scheduling conversion. `parseWindow` accepts RFC3339 fractional timestamps (`internal/server/tools.go:453`, `:460`) and applies strict future/order checks (`internal/server/tools.go:464`, `:469`); formatting as whole-second RFC3339 could collapse valid close bounds.

- The original-input not-found behavior is already centralized in `deps.*`, so the handler plans correctly avoid duplicating it. Update, delete, and visibility all re-wrap with `a.ID` after resolution (`internal/server/tools.go:930`, `:1019`, `:1050`).

## Concerns

- **HIGH — Malformed present email claims can still fall through to `sub`.** Plan 17-01 only invokes the email gate when `raw["email"]` is a non-empty string. Under `[email,sub]`, a present email value of the wrong JSON type would be treated as absent and resolve through `sub`. That violates D-05's stronger rule that fallback occurs only when the earlier claim is "entirely absent/empty." The current implementation already uses strict type assertions (`internal/auth/auth.go:83`, `:92`), but ordered fallback introduces a new consequence: malformed presence can now select another authz bucket. Presence must be checked separately from string conversion.

- **HIGH — Plan 17-06 does not explicitly set `CursorMode: true` for MCP `list_memory`.** Today MCP listing always invokes `store.List` with cursor mode enabled, even on the tokenless first page (`internal/server/tools.go:809`, `:815`). The proposed core request defaults `CursorMode` to false, and the plan says the MCP adapter builds the request but never explicitly requires `CursorMode: true`. In `store.List`, first-page cursor behavior occurs only when `CursorMode` is true (`internal/store/store.go:865`); otherwise `nextCursor` stays empty in offset mode (`internal/store/store.go:817`). This can silently break MCP pagination despite the "byte-for-byte unchanged" must-have.

- **MEDIUM — Payload-only summary updates need explicit stale-key handling and provenance tests.** A client summary update must clear `SummaryModel` and `SummaryEgressAt`; full `Update` does so in memory before a complete Upsert (`internal/store/store.go:1395`, `:1404`). `SetPayload` only overwrites supplied keys. Existing decoding reads `summary_model` and `summary_egress_at` whenever those keys remain (`internal/store/store.go:430`, `:436`). Plan 17-02 says to mirror the semantics but should explicitly require writing empty values or deleting those keys and test an auto-summary-to-client-summary transition and summary clear.

- **MEDIUM — The proposed vector-preservation assertion is underspecified.** `Store.Get` requests payload but not vectors (`internal/store/store.go:1160`), so "assert via Get/round-trip the vector is unchanged" cannot actually prove vector preservation. The test must retrieve the point with vectors enabled through the raw Qdrant client before and after, as existing reindex tests do, or otherwise inspect the stored vector explicitly.

- **MEDIUM — `connectError` omits `store.ErrAmbiguousShortID`.** `ResolvePointID` can return this typed error when legacy duplicate handles exist (`internal/store/store.go:1201`, `:1217`). All by-id write handlers will propagate it to `connectError`, whose planned default is generic `CodeInternal`. This is an operator/data precondition or caller-addressing failure, not an infrastructure fault. It should map deliberately, likely to `FailedPrecondition` or `InvalidArgument`, and have a table case.

- **MEDIUM — The parity-test mechanics remain incomplete.** A direct `deps.*` call returns a domain error, not a Connect error, so `connect.CodeOf(mcpErr)` will be `Unknown`; the plan must explicitly map the direct-lane error through the production `connectError` before comparing codes. Separately, the store spy cannot observe which `deps.*` method was called — only downstream store operations. The source assertion compensates for that, but its suggested `os.ReadFile("internal/server/connectapi.go")` path will fail when `go test` runs with `internal/server` as the package working directory. Use `go/parser` on `connectapi.go` or locate the source via `runtime.Caller`.

- **LOW — Owner-claim parser behavior is internally contradictory.** Plan 17-01's must-haves say duplicate and whitespace-only entries are rejected, while Task 2 says duplicates are silently deduplicated, empty entries are dropped, and malformed whitespace may be "rejected/dropped." This affects operator intent and startup behavior. Pick one deterministic contract and test it.

- **LOW — The migration runbook needs the exact encoded target, not only a placeholder.** Because existing non-email records require `migrate-remap-owner`, documentation should show how to derive the exact length-prefixed target and include examples for `sub` and `client_id`. Otherwise the migration mechanism exists but is error-prone.

## Suggestions

1. Amend `ClaimIdentity` so each configured claim distinguishes: key absent or string `""` → eligible to fall through; key present with a non-string value → reject; selected email present as a non-empty string but `email_verified != true` → reject. Add `[email,sub]` tests for non-string email, `null`, empty string, and unverified string.
2. In Plan 17-06, explicitly require the MCP list adapter to build `coreListRequest{ ..., CursorMode: true }`. Add a tokenless first-page MCP test asserting a full page returns a non-empty `next_cursor`.
3. Require the payload-only store test to seed `SummarySourceAuto`, `SummaryModel`, and `SummaryEgressAt`, then verify client replacement and clearing remove all auto-summary provenance. Retrieve vectors with `WithVectors(true)` before and after.
4. Add `store.ErrAmbiguousShortID` to `connectError` and its table tests.
5. Clarify the parity test: Direct lane `connect.CodeOf(connectError(ctx, depsErr))`; Connect lane `connect.CodeOf(handlerErr)`; store spy verifies equivalent downstream store calls and subjects; AST inspection verifies each handler calls its named `deps.*` method; use `go/parser` rather than fragile textual path scanning.
6. Resolve parser policy explicitly. Rejecting duplicates and empty interior elements is safer for auth configuration because it exposes mistakes instead of silently changing operator intent.

## Risk Assessment

**HIGH until revised.** The overall architecture is sound and the previous two review rounds fixed most foundational problems. The remaining malformed-email fallback is directly on the owner/authz-key boundary, and the missing explicit MCP cursor-mode mapping can silently regress a stable public memory contract. Once those two issues and the medium-risk payload/error/test details are addressed, the implementation plan should drop to **MEDIUM**, appropriate for a large security-sensitive cross-transport refactor.

---

## OpenCode Review

> OpenCode (`google/gemini-3.5-flash`) streamed ~36 lines of source-tracing narration (glob/grep/read steps across `internal/auth`, `internal/webauth`, `internal/server`, `internal/store`) before emitting the structured review reproduced below. The narration is elided; it confirms the review was source-grounded (it opened `auth.go`, `session.go`, `resolver.go`, `store.go`, `summaryqueue_test.go`, `tools_test.go`, `connectapi_negative_test.go`, etc.).

### Structured Plan Review

#### 1. Summary
The implementation plans for **Phase 17** represent an exceptionally thorough, high-fidelity, and secure design. They successfully unify the MCP and Connect auth/write lanes into a single, uniform `deps.*` code path, completely eliminating the risk of handler-level authz drift (the primary milestone hazard in `PITFALLS.md § Pitfall 1`). By introducing a mathematically proven, length-prefixed injective namespace encoding for non-email claims, coupled with strict session versioning and a reserved-namespace email guard, the plans resolve critical multi-tenant authorization collision risks during rollout. Additionally, the plans successfully pre-empt multiple subtle compile-time and run-time landmines, such as test packages breaking on the `deps.st` retype, partial content field-mask blanking, and `time.RFC3339` sub-second precision loss.

#### 2. Strengths
- **Mathematically Proven Injective Encoding (`internal/auth/auth.go:83-97`):** the length-prefixed scheme `fmt.Sprintf("%d:%s:%d:%s", len(claim), claim, len(value), value)` for non-email claims guarantees prefix-free uniqueness — prevents collisions between adversarial pairs like `("sub","x:y")` and `("sub:x","y")`.
- **Reserved-Namespace Email Guard (`internal/auth/auth.go:83-97`):** rejects email values matching the namespaced pattern `^[0-9]+:`, blocking spoofing where an email mimics an encoded claim.
- **Stateless Session Versioning (`internal/webauth/session.go:26-29`):** adding `V int` to the session payload and validating on unseal elegantly invalidates legacy (unversioned bare-owner) cookies on rollout, preventing identity-splitting without cookie-key rotation.
- **Pre-empting Concrete-Store Test Breaks (`internal/server/tools_test.go`):** retyping `deps.st` to `memStore` normally breaks `storeFill(d.st,...)` (`summaryqueue_test.go:402`) and `buildUsageQueue(..., d.st, ...)` (`tools_test.go:1915`); the `testDepsWithStore` helper preserves compile-safety.
- **Vector-Preserving Payload Updates (`internal/store/store.go:1367`):** a separate `UpdatePayload` via Qdrant `SetPayload` ensures metadata updates don't erase vectors or force re-embedding.
- **Source/AST Delegation Assertion (`connectapi_write_parity_test.go`):** because `storeMemory` and `scheduleMemory` produce identical store traces (`MintShortID`+`Upsert`), textually asserting the named method call via Go's parser ensures fidelity.
- **Split Existence-Leak Tables (`connectapi_crossowner_test.go`):** distinguishing `short_id` inputs (hide resolved UUID) from direct-UUID inputs (echo user ID) corrects the prior contradictory assertion.

#### 3. Concerns
- **LOW — Handling of malformed claim names (`internal/config/config.go` & `cmd/engram/serve.go:260-272`):** if an operator configures a claim list containing reserved delimiters (e.g. `ENGRAM_OWNER_CLAIM="sub:x,email"`), the claim name could interfere with the length-prefixed formatter. Mitigation: parser + startup guard must strictly reject any claim name containing `:` or `,` or starting with digits (fast failure).
- **LOW — Context/tracer propagation on CodeInternal errors (`internal/server/connecterror.go`):** logging unexpected `CodeInternal` errors without binding to the active OTel span hides troubleshooting context. Mitigation: use `slog.ErrorContext(ctx, "internal error", "err", err)`.

#### 4. Suggestions
- **Automate session versioning inside `Seal`:** rather than requiring `V: sessionPayloadVersion` at every mint site, have `SessionCodec.Seal(s Session)` inject `s.V = sessionPayloadVersion` automatically — guarantees version-consistent cookies while letting tests bypass `Seal` to verify legacy rejection.
- **Sync double-submit CSRF token expiry during slide:** during Phase 18's sliding re-seal, update the companion CSRF cookie (`CSRFCookieName`) MaxAge to match the slid session TTL, avoiding unexpected CSRF expiry on active sessions.

#### 5. Risk Assessment
- **Risk Level: LOW.** The plans are exceptionally solid; every identified risk has been thoroughly mitigated, compile-safety boundaries preemptively mapped, and the validation architecture (TDD parity tests, AST assertions, real Qdrant integration checks) guarantees correctness before merge.

---

## Consensus Summary

Two independent source-grounded reviewers re-reviewed the round-2-hardened six plans. Both open the referenced Go files and cite `file:line`. **Both confirm the round-1/round-2 fixes are correct** and the architecture is sound. They diverge on residual risk: **Codex rates HIGH** (two new regressions on the authz key + MCP contract, plus medium test/payload gaps); **OpenCode/gemini rates LOW** ("exceptionally solid", only polish). OpenCode did not surface Codex's two HIGH items, so — as with Antigravity in round 2 — the adversarial source-grounded reviewer (Codex) carries the higher weight on residual defects; the positive reviewer contributes endorsement + ergonomic polish.

### Agreed Strengths (both reviewers, verified against source)

All round-2 fixes confirmed correct: the length-prefixed injective owner encoding (prevents the `sub:x:y` collision class); the stateless session-version migration (`session.go:26-29`, rejects legacy bare-owner cookies → no identity split during rollout); the reserved-namespace email guard; `testDepsWithStore` for the `memStore` retype compile-safety; the vector-preserving `UpdatePayload` via `SetPayload`; RFC3339Nano; the source/AST delegation assertion; the split cross-owner existence-leak tables; and the typed superset read core (17-06) preserving offset/categories/visibility/total/cursor.

### Agreed Concerns (both reviewers)

- **Owner-claim parser policy must be deterministic and strict.** Codex (LOW): 17-01's must-haves (reject duplicates/whitespace) contradict Task 2 (silently dedupe/drop). OpenCode (LOW): the parser + startup guard must reject any claim name containing `:` / `,` / leading digits. **Converge on one contract: reject malformed/duplicate/empty claim names and fail fast at startup** (safer for an auth key than silent normalization).
- **`connectError` internal-error logging needs request context.** Codex (LOW) + OpenCode (LOW): use `connectError(ctx, err)` + `slog.ErrorContext(ctx, ...)` so `CodeInternal` logs bind to the active OTel span.

### Codex-only findings (source-grounded, NEW this round — OpenCode rated LOW and missed these)

- **[HIGH] Malformed present email of the wrong JSON type falls through to `sub`.** Under `[email,sub]`, 17-01 gates email only on a non-empty *string*; a present email of a non-string type is treated as absent and resolves via `sub`, violating D-05 fail-closed. **Fix:** check presence separately from string conversion — key absent / `""` → fall through; present-but-non-string → reject; present non-empty string but `email_verified != true` → reject. Add `[email,sub]` tests for non-string / null / empty / unverified email (`auth.go:83`, `:92`).
- **[HIGH] 17-06 never sets `CursorMode: true` for MCP `list_memory`.** MCP always cursors today, even on the tokenless first page (`tools.go:809/815`); the core request defaults `CursorMode` false and first-page cursor behavior only fires when true (`store.go:865`, else offset mode leaves `nextCursor` empty at `store.go:817`) → silent MCP pagination break vs the "byte-for-byte unchanged" must-have. **Fix:** MCP adapter sets `CursorMode: true`; add a tokenless-first-page test asserting a non-empty `next_cursor`.
- **[MEDIUM] Payload-only summary update must clear stale provenance.** `SetPayload` only overwrites supplied keys, but a client summary update must clear `SummaryModel`/`SummaryEgressAt` (full `Update` does, `store.go:1395/1404`; decoder reads them if present, `store.go:430/436`) → test an auto→client summary transition and a summary clear.
- **[MEDIUM] Vector-preservation test can't use `Store.Get`** (it requests payload, not vectors, `store.go:1160`) — retrieve with `WithVectors(true)` via the raw Qdrant client before/after.
- **[MEDIUM] `connectError` omits `store.ErrAmbiguousShortID`** (`ResolvePointID` returns it for legacy dup handles, `store.go:1201/1217`) → maps to `CodeInternal`; should be `FailedPrecondition`/`InvalidArgument` + a table case.
- **[MEDIUM] Parity-test mechanics.** The direct `deps.*` call returns a *domain* error, so `connect.CodeOf(depsErr)` = `Unknown` — map it through `connectError` before comparing; and the source-assertion `os.ReadFile("internal/server/connectapi.go")` fails under `go test`'s package working dir → use `go/parser` or `runtime.Caller`.
- **[LOW] Migration runbook needs the exact encoded target** (worked examples for `sub`/`client_id`), not a placeholder.

### OpenCode-only useful additions

- **Automate the session version in `Seal()`** — inject `s.V = sessionPayloadVersion` in `SessionCodec.Seal` so every mint site is version-consistent by default (tests still bypass `Seal` to forge legacy cookies). Good ergonomics that closes a "forgot to set V" footgun.
- **Phase-18 note:** when the sliding re-seal lands, sync the companion CSRF cookie MaxAge to the slid TTL.

### Recommended disposition for `/gsd-plan-phase 17 --reviews` (round 3)

Incorporate (must): (1) split email *presence* from string conversion in `ClaimIdentity` so a malformed present email rejects instead of falling through (+ `[email,sub]` type/null/empty/unverified tests); (2) MCP list adapter sets `CursorMode: true` + a tokenless-first-page `next_cursor` test. Incorporate (should): (3) payload-only summary update clears `SummaryModel`/`SummaryEgressAt` + provenance test; (4) vector-preservation test uses `WithVectors(true)`; (5) `connectError` maps `store.ErrAmbiguousShortID`; (6) parity test maps the direct-lane error through `connectError` before `CodeOf`, and uses `go/parser`/`runtime.Caller` for the source assertion; (7) one deterministic strict parser contract (reject malformed/dup/empty claim names, fail fast); (8) `connectError(ctx, err)` + `slog.ErrorContext`; (9) auto-inject the session version in `Seal()`; document the exact migration target. Everything the prior two rounds changed is endorsed by both reviewers — round 3 is refinement, and both HIGH items are narrow, well-localized fixes. Risk is converging toward MEDIUM.
