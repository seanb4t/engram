---
phase: 17
reviewers: [codex, antigravity]
reviewed_at: 2026-07-12T13:10:54Z
plans_reviewed: [17-01-PLAN.md, 17-02-PLAN.md, 17-03-PLAN.md, 17-04-PLAN.md, 17-05-PLAN.md]
---

# Cross-AI Plan Review — Phase 17

Reviewers: **Codex** (codex-cli 0.144.1, default model) and **Antigravity** (agy 1.1.1, gemini). Both ran inside the git working tree with source-grounding instructions (verify plan claims against actual code, cite file:line).

---

## Codex Review

# Cross-AI Plan Review — Phase 17

## Overall assessment

The plans correctly identify the phase’s central architectural requirement: both transports must resolve identity once and delegate to the same `deps.*` business methods, with authorization remaining in the store layer. The sequencing of identity work, conversion work, handler wiring, and parity tests is directionally sound.

However, the plans are not execution-ready. Several source-verified problems would either prevent compilation or regress the existing Connect contract:

- The proposed `memStore` interface omits methods still called through `deps.st`.
- The read-lane rewire loses Connect-only pagination, filtering, totals, and default behavior.
- Partial metadata updates have no vector-preserving store path.
- `UpdateMemory` and `SetVisibility` cannot populate their declared response fields from their current `deps.*` signatures.
- Error mapping would expose expected business rejections as `CodeInternal`.
- The proposed owner namespace format is not provably collision-free.
- The parity plan does not clearly exercise all six RPCs.

Overall phase-plan risk: **HIGH** until these issues are resolved in the plans.

---

# 17-01 — Ordered owner claims and namespacing

## Summary

The plan puts ordered claim resolution in the right shared choke point and gives the `email_verified` fallback boundary appropriate attention. Its principal weakness is that the proposed namespace encoding is not actually proven disjoint, even though the owner string is the authorization key.

## Strengths

- It correctly evolves the function already shared by bearer and cookie authentication. The bearer verifier calls `ClaimIdentity` at [auth.go:134](/Volumes/Code/github.com/seanb4t/engram/internal/auth/auth.go:134), while the cookie login path calls it at [oidc.go:78](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/oidc.go:78).

- The D-05 test cases are well targeted. Today the email gate is applied only for the selected `"email"` claim at [auth.go:83](/Volumes/Code/github.com/seanb4t/engram/internal/auth/auth.go:83), with strict Boolean verification at [auth.go:87](/Volumes/Code/github.com/seanb4t/engram/internal/auth/auth.go:87).

- Keeping email owners bare preserves existing store ownership keys. `store.Authenticated` merely wraps the supplied string; it does not provide domain separation itself.

- Updating the verifier and web authenticator atomically is necessary because both currently store a single `ownerClaim` string.

## Concerns

- **HIGH — `<claim>:<value>` is not provably injective or disjoint.** For example, claim/value pairs `("sub", "x:y")` and `("sub:x", "y")` produce the same string. More importantly, because selected email values remain arbitrary bare strings, an IdP-issued email equal to a reserved service-owner string can collide unless the email domain is validated or reserved prefixes are rejected. The store compares owner strings directly in its authorization filters at [store.go:487](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:487) and [store.go:509](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:509), so any collision is an authorization collision.

- **MEDIUM — empty configuration handling is internally contradictory.** The plan says the parser turns an empty result into `["email"]`, while also saying `ownerClaimGuard` rejects an empty list. Once normalized, the guard can never observe the empty configuration. The current guard deliberately rejects explicit `--owner-claim=""` at [serve.go:250](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/serve.go:250), while the registry already supplies the default `"email"` at [registry.go:52](/Volumes/Code/github.com/seanb4t/engram/internal/config/registry.go:52).

- **MEDIUM — invalid list entries are not addressed.** Whitespace and empty elements are dropped, but duplicate claims and malformed claim identifiers are not rejected. Duplicates are harmless operationally but can obscure operator intent; arbitrary delimiters make namespace encoding unsafe.

- **LOW — test/file scope is understated.** Existing owner-claim behavior is pinned in [config_test.go:136](/Volumes/Code/github.com/seanb4t/engram/internal/config/config_test.go:136) and [serve_test.go:8](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/serve_test.go:8), but those files are not listed under `files_modified`.

## Suggestions

- Define a canonical owner encoding with an explicit security contract. Either:

  - restrict non-email claim names to a fixed allowlist and use a reserved format that valid email owners cannot occupy; or
  - length-prefix/base64url-encode both the claim name and value, while rejecting bare email values in the reserved namespace.

- Separate parsing from defaulting:

  - registry default handles “unset” as `email`;
  - parser returns an empty list for an explicitly empty/empty-after-trim value;
  - startup guard rejects that empty list when authentication is active.

- Add tests for duplicate claims, whitespace-only configuration, malformed claim names, delimiter-containing values, and a bare email attempting to occupy the reserved namespace.

## Risk assessment

**HIGH.** The email verification behavior is well planned, but namespace collision affects the primary authorization key and therefore needs a stronger encoding design before implementation.

---

# 17-02 — Store interface, caller threading, partial updates

## Summary

This plan contains the correct identity and actor-attribution refactor, but it has immediate compilation failures and an incomplete partial-update design. It needs another design pass before execution.

## Strengths

- The actor fallback addresses a real source discrepancy. Bearer tokens populate `TokenInfo.UserID` at [auth.go:139](/Volumes/Code/github.com/seanb4t/engram/internal/auth/auth.go:139), while the cookie resolver populates only `Extra[owner_claim]` at [resolver.go:54](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/resolver.go:54).

- Explicit `caller` threading removes identity as a hidden context dependency while preserving `context.Context` for cancellation and tracing.

- The existing not-found rewraps are correctly identified and should be preserved:

  - update: [tools.go:932](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:932)
  - delete: [tools.go:1019](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:1019)
  - visibility: [tools.go:1044](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:1044)

- Making content presence-signaled is necessary. Today `updateArgs.Content` is an unconditional string at [tools.go:507](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:507), and `store.Update` always replaces content at [store.go:1379](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1379).

## Concerns

- **HIGH — the proposed `memStore` interface cannot compile.** The plan explicitly excludes `ListScopes` and `DeleteAll`, but both are invoked through `deps.st`:

  - Connect `ListScopes`: [connectapi.go:93](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:93)
  - MCP `delete_all`: [tools.go:1143](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:1143)

  Once `deps.st` changes from `*store.Store` at [tools.go:34](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:34) to the narrower interface, those calls fail to compile. Plan 02 Task 1 therefore cannot satisfy its own `go build ./...` acceptance gate.

- **HIGH — the vector-preserving update path is missing.** The plan says shared-only or summary-only updates should not re-embed. But `store.Update` always requires a vector and always calls `Upsert` at [store.go:1367](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1367) and [store.go:1406](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1406). A fetched `store.Memory` does not contain its vector. Simply skipping `Embed` leaves no valid vector to pass.

- **HIGH — the plan conflicts with the proto’s documented tags-only semantics.** The proto says a tags-only update does not re-embed at [engram.proto:150](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:150), while current business logic deliberately embeds tags and re-embeds on tag changes at [tools.go:965](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:965). The plan adopts the latter behavior without reconciling the public contract.

- **HIGH — widespread test call sites are omitted.** Changing every `deps.*` signature and changing `updateArgs.Content` to `*string` breaks many direct calls, including [tools_test.go:556](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools_test.go:556), [tools_test.go:1144](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools_test.go:1144), [rules_test.go:367](/Volumes/Code/github.com/seanb4t/engram/internal/server/rules_test.go:367), and [connectapi_test.go:406](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi_test.go:406). These test files are absent from the plan’s modified-file list and tasks.

- **MEDIUM — anonymous actor semantics need an explicit invariant.** `callerFromTokenInfo(nil)` necessarily produces anonymous owner `""` and actor `""`. The plan should state that its “non-empty actor” guarantee applies only to authenticated callers, preserving the existing anonymous bucket behavior.

## Suggestions

- Add `ListScopes` and `DeleteAll` to the interface, or introduce `deps.listScopes`/`deps.deleteAll` before retyping the field. Do not leave either call on a method unavailable through the interface.

- Decide partial-update storage semantics explicitly:

  - add a store payload-update method that preserves the existing vector for shared/summary-only updates;
  - decide whether tags affect embeddings;
  - align the proto comment, MCP behavior, and implementation.

- Consider an internal update command such as:

  - `Content *string`
  - `Tags *[]string`
  - `Shared *bool`
  - `Summary *string`

  with the shared `deps` method choosing either vector-upsert or payload-only update.

- Add all direct method-call tests to the task’s file and compilation scope. A mechanical helper can turn existing required MCP content strings into pointers.

## Risk assessment

**HIGH.** As written, Plan 02 cannot compile and does not provide an implementable no-reembed path for partial metadata updates.

---

# 17-03 — Proto conversion layer

## Summary

A dedicated conversion layer is appropriate and will help keep Connect handlers thin. The main correctness problem is timestamp precision: formatting protobuf timestamps with `time.RFC3339` discards fractional seconds.

## Strengths

- Field-mask-to-pointer mapping is the correct way to distinguish an omitted field from a proto zero value. The proto explicitly makes the mask the sole presence mechanism at [engram.proto:150](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:150).

- Centralizing `Visibility` conversion prevents `UpdateMemory` and `SetVisibility` from drifting.

- Citation fields are structurally aligned with `citationArg` at [tools.go:519](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:519).

- Assuming validation has already run is consistent with the interceptor order at [connectapi.go:262](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:262).

## Concerns

- **HIGH — `time.RFC3339` truncates protobuf timestamp precision.** `Timestamp.AsTime().Format(time.RFC3339)` drops fractional seconds. This can turn two valid bounds within one second into equal strings, after which `parseWindow` rejects them at [tools.go:469](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:469). It can also turn a slightly future `not_after` into the current second and fail the future check at [tools.go:464](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:464).

- **MEDIUM — the planned “sub-second round trip” test cannot honestly pass as written.** It says to preserve the boundary “within the RFC3339 second-level contract,” but that weakens the typed protobuf contract without a stated product decision.

- **MEDIUM — response conversion is underspecified for update and visibility.** Their proto responses contain `id` and `short_id` at [engram.proto:173](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:173) and [engram.proto:187](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:187), but the corresponding `deps.updateMemory` and `deps.setVisibility` methods currently return only `error`.

- **LOW — “round trip” does not apply naturally to FieldMask conversion.** Proto request → internal optional fields is lossy unless a reverse mapping is deliberately defined. The test should be described as exact mapping rather than round-trip.

## Suggestions

- Use `time.RFC3339Nano`; `time.Parse(time.RFC3339, ...)` accepts fractional seconds, so `parseWindow` need not change.

- Add tests with:

  - bounds differing only in nanoseconds;
  - `not_after` less than one second in the future;
  - UTC and non-UTC offsets;
  - nil timestamp fields.

- Resolve the update/visibility result model in Plan 02 before building response helpers. A shared result struct carrying canonical UUID and short ID would work.

## Risk assessment

**MEDIUM-HIGH.** The layer itself is sound, but timestamp truncation can reject valid scheduling requests, and the response conversion depends on unresolved business-method signatures.

---

# 17-04 — Fake store, handlers, and read-lane rewire

## Summary

This plan tackles the right integration point, but it currently combines several incompatible assumptions. The write handlers lack a complete result/error contract, and the proposed read rewire would regress established Connect behavior.

## Strengths

- Sequencing the fake and negative-matrix repair with handler wiring correctly recognizes the nil-store hazard at [connectapi_negative_test.go:64](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi_negative_test.go:64).

- The handler shape—resolve caller, convert, call `deps.*`, map result—is correct.

- Preserving the original-input rewrap inside `deps.*`, rather than duplicating it in Connect handlers, is the correct single-path design.

- Keeping authorization checks out of `connectapi.go` follows the store’s owner/read gates at [store.go:487](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:487).

## Concerns

- **HIGH — rewiring `ListMemories` to the current `deps.listMemory` regresses the Connect API.** Connect currently supports:

  - offset pagination;
  - categories;
  - visibility;
  - exact total;
  - cursor opt-in and next token.

  These are present in the proto at [engram.proto:51](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:51) and are passed into `store.List` at [connectapi.go:124](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:124). By contrast, `listArgs` has none of offset/categories/visibility at [tools.go:485](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:485), and `deps.listMemory` discards `total` at [tools.go:809](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:809).

- **HIGH — search defaults would change.** Connect defaults `k` to 20 at [connectapi.go:162](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:162), while `deps.searchMemory` defaults it to 8 at [tools.go:854](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:854). A direct rewire silently changes API behavior.

- **HIGH — read result shapes are incompatible.** `deps.listMemory` and `deps.searchMemory` return MCP-shaped `[]any`, not `[]store.Memory`; Connect currently builds full proto records from `store.Memory` using [connectapi.go:33](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:33). The plan does not define a lossless shared result type.

- **HIGH — update and visibility responses cannot be populated.** `deps.updateMemory` and `deps.setVisibility` return only errors, yet their proto responses expose canonical ID and short ID. Returning the request ID would be wrong when the caller used a short ID; calling the store again in the handler would violate the thin-adapter rule.

- **HIGH — expected business failures would become `CodeInternal`.** Rule immutability is currently a plain error at [tools.go:1055](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:1055), as are validation and stale-summary failures. The proposed mapping handles only `ErrNotFound` and `ErrInvalidArgument`, sending everything else to Internal. These are client-visible precondition/invalid-argument failures, not infrastructure faults.

- **MEDIUM — a fake store alone does not make the negative matrix runnable.** Valid StoreMemory, StoreDiscovery, and ScheduleMemory calls reach `d.em.Embed` at [tools.go:641](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:641), [tools.go:739](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:739), and [tools.go:681](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:681). A fake-backed `deps` also needs a non-nil embedder.

- **MEDIUM — the fake is being asked to duplicate too much real-store policy.** Reimplementing ownership and rule behavior in a map fake can drift from Qdrant behavior. Rule rejection and summary reconciliation already happen in `deps`; a scripted/spying store is better for structural delegation tests.

## Suggestions

- Introduce transport-neutral core request/result types that are a superset of both lanes. For list operations, preserve all Connect fields and let MCP pass zero values for unsupported options.

- Preserve each transport’s default at the adapter boundary, or encode defaults explicitly in the shared request.

- Define typed domain errors or a single production `connectError(err)` mapper. Expected examples:

  - not found → `CodeNotFound`;
  - malformed ID/input → `CodeInvalidArgument`;
  - stale summary/rule mutation → `CodeFailedPrecondition`;
  - unexpected store/embed failures → `CodeInternal`.

- Refactor by-ID mutations to return a result containing canonical ID and short ID from the already-fetched record.

- Use a minimal scripted fake/spying store for parity, plus existing real-store integration tests for authorization semantics.

- Add end-to-end effect tests for all six RPCs, not only StoreMemory.

## Risk assessment

**HIGH.** This plan would otherwise regress the read API and does not yet have sufficient result or error semantics to implement the new write handlers correctly.

---

# 17-05 — Parity and existence-leak tests

## Summary

The final verification wave targets the right security invariants, but its coverage and oracle design need tightening. In particular, it does not clearly test all six RPCs, and comparing Connect codes through a test-only error mapper can merely duplicate production logic.

## Strengths

- The plan correctly uses the existing rerank parity test as a structural model. That test directly invokes both paths at [connectapi_test.go:404](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi_test.go:404) and [connectapi_test.go:420](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi_test.go:420).

- The short-ID leak test has a good existing precedent at [connectapi_test.go:621](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi_test.go:621).

- Actor attribution and tags/content preservation deserve explicit regression rows rather than being inferred from successful writes.

- Re-running the idempotency gate after real handlers land is appropriate.

## Concerns

- **HIGH — “per-RPC parity” is not concretely covered.** The named scenarios primarily exercise `UpdateMemory` and `SetVisibility`, with a generic cross-owner mutation. There are no explicit parity rows for StoreMemory, StoreDiscovery, ScheduleMemory, or DeleteMemory. This falls short of the requirement that every write handler delegates through the same shared path.

- **HIGH — the test oracle may duplicate production error mapping.** A helper that converts raw `deps.*` errors into Connect codes using the same hand-written rules as the handler only proves those two mappings currently agree. It does not prove that the handler delegated to the intended method.

- **MEDIUM — direct-UUID assertions are contradictory.** When owner B supplies owner A’s UUID directly, the required original-input rewrap must contain that same UUID. The acceptance criterion cannot simultaneously require the message to echo the supplied UUID and exclude the resolved UUID. The leak invariant applies specifically when resolution transforms a short ID into a UUID the caller did not provide.

- **MEDIUM — fake-store parity does not prove production-store authorization.** It proves both adapters reach the same business method, which is valuable, but the final gate should also rely on real store isolation tests because the fake’s authz implementation can drift.

- **MEDIUM — stateful table rows need isolation.** Delete and update scenarios mutate records. Reusing one fake/deps instance across rows can make later results order-dependent unless each row seeds an isolated fixture.

- **LOW — the proposed idempotency grep is broader/less precise than stated.** A plain grep confirms annotations exist or do not exist, but a descriptor-level test is more robust for checking only the six write procedures.

## Suggestions

- Define at least one row for every RPC:

  - StoreMemory: success plus actor/owner stamping;
  - StoreDiscovery: success and replacement/cross-owner rejection;
  - UpdateMemory: stale summary, rule mutation, partial masks;
  - DeleteMemory: success and cross-owner not found;
  - SetVisibility: rule rejection and cross-owner not found;
  - ScheduleMemory: success and clock-dependent invalid window.

- Make the fake a spy that records which method was invoked and with which subject/arguments. Assert the Connect call and direct `deps.*` call hit the same method and produce the same stored effect or sentinel error.

- Split existence-leak tests:

  - short-ID input: error contains the short ID and excludes the resolved UUID;
  - direct UUID input: error contains exactly the supplied UUID, because the caller already knows it.

- Give each table row a fresh fixture or explicit reset.

- Run both hermetic parity tests and the existing Qdrant-backed isolation suite as the phase gate.

## Risk assessment

**HIGH.** The planned tests cover important cases, but they do not yet prove delegation for every RPC and contain a logically inconsistent direct-UUID assertion.

---

# Recommended plan restructuring

Before execution, I would revise the waves as follows:

1. **Owner resolution/security**
   - finalize collision-safe owner encoding;
   - preserve explicit-empty startup rejection.

2. **Shared core contracts**
   - complete `memStore`, including `ListScopes` and `DeleteAll`;
   - introduce `caller`;
   - define typed mutation results and typed domain errors;
   - define vector-preserving partial-update behavior.

3. **Transport-neutral read/write models**
   - preserve the full Connect list/search contract;
   - preserve transport-specific defaults;
   - update all direct tests for new signatures.

4. **Proto conversion**
   - use `RFC3339Nano`;
   - map masks and mutation results.

5. **Handler wiring**
   - centralized Connect error mapper;
   - all six write handlers;
   - read rewire without losing fields or totals.

6. **Verification**
   - every RPC exercised through both lanes;
   - spy-based delegation assertions;
   - real-store isolation regression;
   - short-ID leak tests;
   - full `task` and secure-phase review.

With those corrections, the overall architecture is strong and should achieve true MCP/Connect authorization parity without weakening the existing Connect read contract.

---

## Antigravity Review

> Antigravity wrote its detailed review to an artifact file (`plan_review.md`) and emitted only a pointer to stdout; the full artifact content is reproduced below.

# Phase 17: Wired Write Handlers (Full CRUD & Schedule) — Plan Review

This document provides a structured, technical review of the five implementation plans (`17-01-PLAN.md` through `17-05-PLAN.md`) proposed for **Phase 17** of the **engram** project.

---

## Executive Summary

The proposed plans are **high-quality, thorough, and highly aligned** with the architectural decisions and constraints of the project. They successfully anticipate complex edge cases (e.g., partial update masks, actor attribution fallbacks, and negative test failures) and address them proactively.

However, there is **one critical compilation risk** in `17-02-PLAN.md` (Task 1) regarding the extracted `memStore` interface. If left unaddressed, the Go compiler will reject the code during compilation of `tools.go` and `connectapi.go`. 

Beyond this blocker, the plans are sound and ready to execute once the suggested adjustments are applied.

---

## Detailed Plan-by-Plan Analysis

### 17-01-PLAN.md: Ordered Owner-Claim List & Namespacing
* **Focus**: Implements the ordered claims fallback (`D-04`), the `email_verified` gate (`D-05`), and claim namespacing (`D-06`).
* **Strengths**:
  * Correctly identifies the wide blast radius of changing `ClaimIdentity`'s signature and schedules atomic updates across all packages in a single commit.
  * Properly preserves the strictness of the `email_verified` gate: if `email` is present but unverified, it fails immediately instead of falling through to subsequent claims.
  * Ensures claim spaces are disjoint by prefixing non-email claims (e.g., `sub:<value>`), preventing collision attacks where a `sub` claim could impersonate a bare email address.
* **Gaps / Suggestions**:
  * **None**. The plan's test assertions are complete and address both security invariants directly.

### 17-02-PLAN.md: Store-Interface Extraction & Caller-Threaded Refactor
* **Focus**: Extracts a narrow `memStore` interface, threads `caller` through `deps.*` methods, and resolves partial updates via `*string` content pointers.
* **Critical Risk (Go Compilation Failure)**:
  * **The Problem**: The plan retypes `deps.st` from `*store.Store` to the new `memStore` interface and states: *"Do NOT fold in `DeleteAll` or `ListScopes` unless a later task needs them (research OQ1/OQ2 scope them out for now)."*
  * **Why it fails**: 
    1. In `tools.go:1143`, the MCP `delete_all` tool registration closure directly calls `d.st.DeleteAll(ctx, a.Scope, subj)`.
    2. In `connectapi.go:93`, `ListScopes` directly calls `a.d.st.ListScopes(ctx, subj)`.
    3. If `st` is retyped to `memStore` and these methods are omitted from the interface, the Go compiler will throw an undefined method error (e.g., `d.st.DeleteAll undefined (type memStore has no method DeleteAll)`), preventing the package from compiling.
  * **The Fix**: Add both `DeleteAll` and `ListScopes` to the `memStore` interface definition from the start (see [Recommendations](#recommendations)).
* **Other Observations**:
  * **Landmine 2 (`updateArgs.Content` pointer)**: Changing `updateArgs.Content` to `*string` is a great catch. It avoids blanking the content during a partial update (like a tags-only update) over the Connect lane, keeping `deps.updateMemory` as the single code path for both lanes.
  * **Landmine 3 (Actor fallback)**: Correctly identifies that the Connect cookie lane `TokenInfo` does not contain `UserID` and fallback-attributes `Actor` to the resolved owner string.

### 17-03-PLAN.md: protoconv Conversion Layer & Round-Trip Tests
* **Focus**: Implements the `protoconv` adapter for request/response serialization and its round-trip unit tests (`D-09`).
* **Strengths**:
  * Adheres to the TDD pattern (RED first, then GREEN).
  * Avoids redundant validation of `FieldMask` paths and enums by relying on the upstream `buf.validate` CEL rules already executed in the interceptor layer.
  * Uses `.AsTime().Format(time.RFC3339)` to map `google.protobuf.Timestamp` to RFC3339 strings, preserving `parseWindow`'s signature and minimizing regression risks.

### 17-04-PLAN.md: Fake Store, Connect API Write Handlers, and Read-Lane Rewire
* **Focus**: Implements the `fakeStore` test double (`D-10`), authoring the six write RPC handlers, and rewires Connect read API handlers through `deps.*` (`D-07`).
* **Strengths**:
  * **Landmine 1 (Nil Store Fix)**: Proactively recognizes that `TestWriteRPCNegativeMatrix` constructs `d := &deps{}` with a nil store, which would panic once the handlers call real business logic.
  * **Read-Lane Rewire**: Correctly isolates `ListScopes` as a documented exception to `D-07` since there is no equivalent MCP tool or `deps` wrapper.
  * Preserves `DEC-cgb` (authorization lives only in the store layer; handlers remain thin adapters and do not compare owners).

### 17-05-PLAN.md: Parity Testing & Security Re-assertion
* **Focus**: Implements the shared-scenario parity test (`D-10`), cross-owner not-found re-wrapping table (`D-11`), and enforces the idempotency-level ban (`D-12`).
* **Strengths**:
  * The parity test clone structure is solid: it tests actual behavior (errors, results, data mutation) through both lanes to ensure structural correctness.
  * The cross-owner re-wrap test ensures that error messages return the user's original input (`short_id` or supplied UUID) rather than the resolved UUID, preventing token-guessing information leaks.

---

## Security & Invariants Review

| Invariant / Decision | Assessment | Verification Status in Plans |
|:---|:---|:---|
| **D-05 (`email_verified` invariant)** | **Secure**. Prevents unverified email claims from bypassing verification by falling back to other claims. | Covered by a dedicated unit test in `17-01-PLAN.md`. |
| **D-06 (Namespace disjointness)** | **Secure**. Namespaces non-email claims (e.g. `sub:<val>`) to prevent collisions with bare email owner strings. | Covered by a dedicated disjointness test in `17-01-PLAN.md`. |
| **D-11 (Existence Leak / DEC-xa6)** | **Secure**. Re-wraps `store.ErrNotFound` with the original input supplied by the user (never the resolved UUID). | Covered by cross-owner table tests in `17-05-PLAN.md`. |
| **D-12 (Idempotency Ban / SC5)** | **Secure**. Enforces that write RPCs do not carry `NO_SIDE_EFFECTS` to prevent caching/GET-like retry semantics. | Verified via static lint check re-assertion in `17-05-PLAN.md`. |

---

## Recommendations

### 1. [CRITICAL] Update the `memStore` Interface definition to include `DeleteAll` and `ListScopes`
In `17-02-PLAN.md` (Task 1), define the `memStore` interface to include the following signatures so that `tools.go` and `connectapi.go` compile cleanly:

```go
type memStore interface {
	// ... existing methods ...
	DeleteAll(ctx context.Context, scope string, subj store.Subject) error
	ListScopes(ctx context.Context, subj store.Subject) (map[string]uint64, error)
}
```

### 2. Map Concurrency Conflicts (`WORLD_CONCURRENT_EDIT` equivalent) to Connect Error Codes
In `connectapi.go` (Plan 17-04 Task 2), ensure that stale-summary conflicts or edit concurrency conflicts are mapped to a suitable Connect error code instead of falling through to `connect.CodeInternal`. 
* **Recommendation**: Map concurrency/stale-state errors to `connect.CodeAborted` or `connect.CodeFailedPrecondition` to allow clients to retry gracefully.

---

## Consensus Summary

Two independent source-grounded reviewers (Codex `gpt-5.x`, Antigravity `gemini`) reviewed the five plans against the live tree. Both open the referenced Go files and cite `file:line`. **Antigravity is broadly positive** ("high-quality, ready to execute once one blocker is fixed"); **Codex is far more critical** ("not execution-ready", overall risk HIGH), surfacing compile-breakers and Connect read-API regressions Antigravity did not. Where a finding is cited by only one reviewer but is concrete and source-grounded, it is still worth acting on — the value of cross-AI review is exactly this non-overlap.

### Agreed Concerns (both reviewers — highest priority)

1. **[BLOCKER] `memStore` interface omits `DeleteAll` and `ListScopes` → 17-02 will not compile.** Both reviewers independently flag this as the top issue. `deps.st` is retyped from `*store.Store` to the new `memStore` interface, but `d.st.DeleteAll(...)` is still called at `tools.go:1143` (MCP `delete_all`) and `a.d.st.ListScopes(...)` at `connectapi.go:93` (Connect `ListScopes`). 17-02 Task 1's explicit "do NOT fold in DeleteAll/ListScopes" instruction directly causes an `undefined method` failure of its own `go build ./...` acceptance gate. **Fix:** include both methods in the `memStore` interface from the start (OQ1/OQ2 in RESEARCH must be resolved as "include", not "defer").

2. **[HIGH] Connect error mapping sends expected business failures to `CodeInternal`.** Both reviewers: rule-immutability (`tools.go:1055`), stale-summary/concurrency conflicts, and validation failures are client-visible precondition/invalid-argument errors, not infrastructure faults. The plan's `ErrNotFound`+`ErrInvalidArgument`-only mapping buckets everything else into `CodeInternal`. **Fix:** a single production `connectError(err)` mapper — not-found→`CodeNotFound`, malformed→`CodeInvalidArgument`, stale-summary/rule-mutation→`CodeFailedPrecondition` (or `CodeAborted` for concurrency), unexpected→`CodeInternal`.

### Agreed Strengths (both reviewers)

- All three verified landmines are correctly pre-empted: nil-store negative-matrix fix (`connectapi_negative_test.go:64`), `updateArgs.Content *string` to avoid tags-only blanking, and the `Actor`-from-owner fallback for the Connect cookie lane (`resolver.go:54`).
- Single-path delegation design: thin Connect adapters call `deps.*`, authorization stays in the store layer (DEC-cgb), the not-found re-wrap lives once in `deps.*`.
- `ClaimIdentity` is the correct shared choke point; the `email_verified` hard-fail (D-05) is well targeted.
- The parity test correctly clones the existing `TestRerankParityMCPAndConnect` structure and tests real behavior through both lanes.

### High-Value Single-Reviewer Concerns (Codex — source-grounded, not raised by Antigravity)

These are the reviews' biggest payoff. Antigravity cleared these areas; Codex traced them into the code and found problems:

- **[HIGH] Read-lane rewire (D-07/17-04) regresses the Connect read API.** `deps.listMemory` has no offset pagination / categories / visibility filter and discards `total` (`tools.go:485`, `tools.go:809`), whereas Connect currently passes all of these into `store.List` (`connectapi.go:124`, proto `engram.proto:51`). Search `k` default silently changes 20→8 (`connectapi.go:162` vs `tools.go:854`). Result shapes are incompatible: `deps.listMemory`/`searchMemory` return MCP-shaped `[]any`, not `[]store.Memory`. **A naive read rewire is a functional regression, not a refactor** — D-07 needs transport-neutral core request/result types (superset of both lanes) before the reads can share a path.
- **[HIGH] No vector-preserving partial-update path.** `store.Update` always requires a vector and Upserts (`store.go:1367`, `store.go:1406`); a fetched `store.Memory` does not carry its vector, so "skip re-embed on shared/summary-only update" is unimplementable as written — there is no valid vector to pass. Needs a store payload-only update method.
- **[HIGH] `UpdateMemory`/`SetVisibility` responses cannot be populated.** Their proto responses expose `id`+`short_id` (`engram.proto:173/187`) but `deps.updateMemory`/`deps.setVisibility` return only `error`. Echoing the request id is wrong when the caller supplied a short_id; re-fetching in the handler violates the thin-adapter rule. **Fix:** have the by-id mutation `deps.*` methods return a small result carrying canonical UUID + short_id from the already-fetched record.
- **[HIGH] Widespread test call sites omitted from `files_modified`.** Changing every `deps.*` signature and `updateArgs.Content`→`*string` breaks direct callers in `tools_test.go:556/1144`, `rules_test.go:367`, `connectapi_test.go:406` — none listed in 17-02.
- **[MEDIUM] 17-05 per-RPC parity is under-covered.** Named scenarios mostly exercise `UpdateMemory`/`SetVisibility`; there are no explicit parity rows for StoreMemory/StoreDiscovery/ScheduleMemory/DeleteMemory. A test-only error mapper that reuses the handler's mapping rules only proves the two mappings agree, not that the handler delegated. **Fix:** make the fake a spy that records which method was invoked; add a row per RPC.

### Divergent Views (reviewers disagree — investigate before executing)

- **Namespace encoding (D-06).** Codex: **HIGH** — `<claim>:<value>` is not provably injective (`("sub","x:y")` and `("sub:x","y")` collide), and a bare email could equal a reserved service-owner string; owner strings are compared directly in store authz filters (`store.go:487/509`), so a collision is an authz collision. Antigravity: **secure** (prefixing prevents `sub`→email impersonation). *Codex's collision case is concrete; worth a canonical/length-prefixed encoding or a reserved-namespace guard even if the common case is safe.*
- **RFC3339 timestamps (17-03).** Codex: **HIGH** — `time.RFC3339` truncates fractional seconds and can reject valid scheduling windows (`parseWindow` `tools.go:469`, future-check `tools.go:464`); use `RFC3339Nano`. Antigravity: the RFC3339 approach "minimizes regression risks." *Codex is correct that `RFC3339Nano` is strictly safer with no downside — `time.Parse(time.RFC3339,...)` still accepts fractional input.*
- **Overall readiness.** Antigravity: ready after the one interface fix. Codex: needs another design pass (read-model, result/error contracts, partial-update storage). *Given Codex's concrete compile/regression findings, treat overall risk as HIGH, not LOW.*

### Recommended disposition for `/gsd-plan-phase 17 --reviews`

Incorporate (must): (1) `memStore` includes `DeleteAll`+`ListScopes`; (2) production `connectError` mapper with FailedPrecondition/Aborted for stale/rule/concurrency; (3) by-id mutation `deps.*` return a UUID+short_id result; (4) list all impacted `*_test.go` files in 17-02 `files_modified`; (5) resolve the read-lane rewire with a transport-neutral list/search request+result (or scope D-07 down to a spy-verified delegation without dropping Connect list/search fields); (6) vector-preserving payload-only store update; (7) `RFC3339Nano`; (8) per-RPC parity rows + spy fake. Investigate/decide: namespace encoding hardening (D-06). Everything else in the plans is endorsed by both reviewers.
