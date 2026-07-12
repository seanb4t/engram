---
phase: 17
round: 6
reviewers: [codex, opencode, antigravity]
reviewed_at: 2026-07-12T18:44:08Z
plans_reviewed: [17-01-PLAN.md, 17-02-PLAN.md, 17-03-PLAN.md, 17-04-PLAN.md, 17-05-PLAN.md, 17-06-PLAN.md]
models:
  codex: gpt-5.x (codex-cli 0.144.1)
  opencode: openrouter/x-ai/grok-4.5
  antigravity: agy 1.1.1 (model selected internally)
reviewed_commit: e47c59b6   # round-5-incorporated plans
caveats:
  antigravity:
    - "Prompt was 333KB; agy truncated the tail again — it reviewed only 17-01 through 17-05 (17-06 missing)."
    - "agy split output: a stdout summary + interactive questions, with the substance written to an artifact under its brain dir. It DID read some real repo files this round (3 real-repo cites), but its findings re-raise the round-5 test-file omissions (handlers_test.go/oidc_exchange_test.go/embed_wiring_test.go) AS IF STILL OPEN — those were fixed in commit e47c59b6 and now appear in files_modified. It reviewed a stale/pre-round-5 mental model."
    - "Every agy finding duplicates an already-incorporated round 1-5 fix. Weighted ~zero for consensus."
verified_against_source: true   # every codex + opencode/grok-4.5 net-new cite spot-checked against the live tree by the orchestrator; all confirmed
---

# Cross-AI Plan Review — Phase 17 (Round 6)

Round 6 reviews the ROUND-5-INCORPORATED plans (commit e47c59b6). Both source-grounded reviewers
(Codex, OpenCode/grok-4.5) independently VERIFIED that all six round-5 fixes are sound and that no
round 1-4 disposition was regressed. No HIGH-severity issue remains. The residual findings are
execution-mechanics refinements (task sequencing, payload-write atomicity, CI test-enforcement,
test-intent preservation) — not design flaws.

## Codex Review
*(codex-cli 0.144.1, source-grounded — all net-new cites orchestrator-verified against the live tree)*

# Round 6 Plan Review

## Summary

The plans are technically strong and now cover the phase’s main correctness and security risks well. The round-5 changes are largely sound: the anonymous-caller embed tests still stop before store access, the added CSRF and webauth call sites are genuine compile/runtime dependencies, key deletion avoids zero-time provenance corruption, and the RFC3339Nano claim is now correctly limited to validation rather than persisted scheduling. I found three substantive unresolved issues: Plan 17-01’s first task cannot pass its own repository-wide lint gate before Task 2 updates external call sites; Plan 17-02’s two-operation payload mutation can partially succeed; and Plan 17-05 does not actually guarantee that real-Qdrant tests run in CI. There is also one generated-file scope omission.

Overall risk: **MEDIUM** until those issues are corrected.

---

## 17-01 — Ordered owner claims and migration

### Summary

The authorization-key design is careful and substantially complete. Injective namespacing, malformed-claim rejection, email verification, legacy-cookie invalidation, and the migration runbook address the important privilege-boundary risks. The only material issue is task sequencing.

### Strengths

- The injective namespace is justified by the actual authorization mechanism: owner values are compared directly in read and write filters at [internal/store/store.go:493](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:493) and [internal/store/store.go:514](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:514). Treating encoding collisions as authorization collisions is correct.

- Session version injection belongs in `Seal`, because the actual callback mint site passes only owner and expiry at [internal/webauth/handlers.go:172](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/handlers.go:172). The round-5 placement prevents future mint sites from omitting the version.

- Rejecting legacy cookies closes a real rollout split: the current resolver forwards the sealed owner verbatim at [internal/webauth/resolver.go:44](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/resolver.go:44) and [internal/webauth/resolver.go:54](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/resolver.go:54).

- The migration runbook is backed by a real dry-run-capable command. `RemapOwner` counts without writing when `dryRun` is true and documents its collection-wide, non-transactional behavior at [internal/store/store.go:1890](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1890).

### Concerns

- **MEDIUM — Task 1 cannot pass its own lint/acceptance gate before Task 2.** Task 1 changes both `auth.New` and `ClaimIdentity` to take `[]string`, but limits its files to `internal/auth` and says only “in-package” call sites are updated ([17-01-PLAN.md:173](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-01-PLAN.md:173), [17-01-PLAN.md:197](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-01-PLAN.md:197)). External scalar call sites remain in [cmd/engram/serve.go:152](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/serve.go:152), [cmd/engram/serve.go:284](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/serve.go:284), and [internal/webauth/oidc.go:78](/Volumes/Code/github.com/seanb4t/engram/internal/webauth/oidc.go:78). Yet Task 1 requires `task lint:go`, which runs `golangci-lint run ./...` across the entire module at [Taskfile.yaml:64](/Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml:64). It also requires a repository-wide grep showing every call already uses a slice at [17-01-PLAN.md:207](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-01-PLAN.md:207). That state is only achievable after Task 2.

### Suggestions

- Make Tasks 1 and 2 one atomic compiling task, or move the public signature changes and all external call-site updates into Task 2.

- If the split is retained, remove repository-wide lint and grep from Task 1 and run only `go test ./internal/auth/...`; make Task 2 the first full-module compile/lint gate.

### Risk Assessment

**MEDIUM.** The security design is strong, but the current task boundary creates a deterministic execution/commit failure.

---

## 17-02 — Store seam, caller threading, partial updates

### Summary

This plan correctly solves the hardest architectural prerequisites: the interface seam, explicit caller threading, partial content semantics, typed errors, and by-ID results. The round-5 key-deletion correction is semantically right, but implementing it as two separate Qdrant mutations introduces a partial-write failure mode.

### Strengths

- `updateArgs.Content -> *string` is necessary: the existing store update always assigns content and then upserts at [internal/store/store.go:1379](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1379) and [internal/store/store.go:1406](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1406).

- The usage-signal preservation mirrors the existing update semantics exactly: `AccessCount` and `LastAccessedAt` are updated at [internal/store/store.go:1382](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:1382).

- The round-5 stop-before-store fix is sound. `storeMemory` calls the embedder before `MintShortID` or `Upsert` at [internal/server/tools.go:641](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:641), matching the test double’s intentional error at [internal/server/embed_wiring_test.go:23](/Volumes/Code/github.com/seanb4t/engram/internal/server/embed_wiring_test.go:23). An explicit anonymous caller does not reach storage.

- Key deletion is required if `SetPayload` remains the update primitive: the decoder considers provenance present whenever the keys exist at [internal/store/store.go:433](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:433), while the full encoder omits zero/empty provenance at [internal/store/store.go:332](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:332).

### Concerns

- **MEDIUM — `SetPayload` plus `DeletePayload` is a non-atomic partial-write path.** The plan explicitly mandates distinct operations at [17-02-PLAN.md:179](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-02-PLAN.md:179). They are separate RPCs in the pinned Qdrant client: `SetPayload` at [points.go:140](/Users/sean/go/pkg/mod/github.com/qdrant/go-client@v1.18.3/qdrant/points.go:140) and `DeletePayload` at [points.go:174](/Users/sean/go/pkg/mod/github.com/qdrant/go-client@v1.18.3/qdrant/points.go:174). If the first succeeds and the second fails, the method returns an error after visibility, summary, access count, and timestamp have already changed while stale provenance remains. Retrying may repair it, but the caller has observed a failed mutation that partially committed.

- **LOW — regenerated TypeScript is omitted from `files_modified`.** The plan changes a proto source comment and runs full `buf generate`, but lists only the Go protobuf output at [17-02-PLAN.md:7](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-02-PLAN.md:7). Buf also generates TypeScript at [buf.gen.yaml:21](/Volumes/Code/github.com/seanb4t/engram/buf.gen.yaml:21), and the exact comment being changed exists in [gen/ts/engram/v1/engram_pb.ts:665](/Volumes/Code/github.com/seanb4t/engram/gen/ts/engram/v1/engram_pb.ts:665). CI checks all of `gen/` for drift at [.github/workflows/ci.yaml:120](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/ci.yaml:120).

### Suggestions

- Prefer one `OverwritePayload` request using the fully updated `cur` payload. The pinned client supports whole-payload overwrite at [points.go:148](/Users/sean/go/pkg/mod/github.com/qdrant/go-client@v1.18.3/qdrant/points.go:148), and `toPayload(cur)` already omits cleared provenance keys while retaining all other fields at [internal/store/store.go:304](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:304). This preserves the vector and avoids a two-request partial commit.

- If two operations are retained, explicitly document non-atomic semantics and add a fault-injection test for failure between `SetPayload` and `DeletePayload`.

- Add `gen/ts/engram/v1/engram_pb.ts` to `files_modified`.

### Risk Assessment

**MEDIUM.** The architecture is correct, but payload mutation durability should be fixed before execution.

---

## 17-03 — Proto conversion layer

### Summary

This plan is well scoped and source-consistent. I found no net-new concern.

### Strengths

- Mask-driven `Content == nil` cleanly closes the content-blanking risk.

- RFC3339Nano is correctly scoped to adapter validation. The proto accepts timestamp bounds at [proto/engram/v1/engram.proto:199](/Volumes/Code/github.com/seanb4t/engram/proto/engram/v1/engram.proto:199), while persisted window values are deliberately reduced to Unix seconds at [internal/store/store.go:319](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:319) and reconstructed at [internal/store/store.go:405](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:405). The round-5 wording no longer overclaims persistence precision.

- Exact-mapping tests are a better contract than pretending lossy proto-to-args conversion round-trips symmetrically.

### Concerns

- None net-new.

### Suggestions

- Retain the explicit test that two nanosecond-distinct bounds produce distinct adapter strings, while avoiding any persisted sub-second assertion.

### Risk Assessment

**LOW.**

---

## 17-04 — Connect handlers and read convergence

### Summary

The handler plan now has the right layering and regression coverage. The round-5 CSRF and double-embedding fixes are valid. I found no net-new blocker.

### Strengths

- The CSRF test assignment is necessary: the current happy path uses a bare `deps` and expects `CodeUnimplemented` at [internal/server/connectcsrf_test.go:219](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectcsrf_test.go:219). Once `StoreMemory` is real, that path reaches storage.

- Removing handler-local query embedding is correct because current Connect searches embed directly at [internal/server/connectapi.go:158](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:158) and [internal/server/connectapi.go:220](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:220), while the shared dependency methods already embed at [internal/server/tools.go:870](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:870) and [internal/server/tools.go:911](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:911).

- Removing the Connect-side usage enqueue is necessary: it currently enqueues at [internal/server/connectapi.go:211](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:211), while `deps.getMemory` already enqueues on success at [internal/server/tools.go:1003](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:1003).

- Empty-scope discovery mapping is correctly identified: the current Connect handler passes empty scope directly to the store at [internal/server/connectapi.go:228](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:228), whereas the shared helper rejects it unless `CrossSpine` is set at [internal/server/tools.go:884](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:884).

### Concerns

- None net-new.

### Suggestions

- In the CSRF happy-path cell, assert `err == nil` for scripted success rather than describing `connect.CodeOf(nil)` as a “success code”; Connect’s `CodeOf` returns `CodeUnknown` for non-Connect errors, including nil.

### Risk Assessment

**LOW**, with normal implementation complexity from touching all transport handlers.

---

## 17-05 — Parity and existence-leak gates

### Summary

The parity design is strong: independent lane fixtures, production error mapping, spy traces, AST delegation checks, and split short-ID/UUID leak cases complement each other. The remaining issue is that the claimed real-store CI gate is not guaranteed to run.

### Strengths

- The original-input error invariant is grounded in the existing Connect implementation, which re-wraps a cross-owner result using the supplied ID at [internal/server/connectapi.go:200](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:200). Splitting short-ID and direct-UUID assertions is correct.

- Independent fixtures are essential for mutating operations and prevent order-dependent parity failures.

- The AST check closes a real blind spot in store-level spy traces where different dependency methods may call identical store primitives.

- Lane-appropriate actor checks correctly avoid asserting false equality between bearer `UserID` attribution and cookie-owner fallback.

### Concerns

- **MEDIUM — “not skipped in CI” is not enforced.** The plan requires the real Qdrant suite to be “not skipped in CI” at [17-05-PLAN.md:188](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/17-wired-write-handlers-full-crud-schedule/17-05-PLAN.md:188), but CI merely runs `go test ./...` at [.github/workflows/ci.yaml:23](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/ci.yaml:23). `TestMain` explicitly continues with integration tests skipped whenever the container fails to start at [internal/server/tools_test.go:123](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools_test.go:123) and [internal/server/tools_test.go:135](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools_test.go:135). A Docker or image-pull failure therefore produces a green CI run without the real-store authorization gate.

### Suggestions

- In CI, either provision Qdrant as an explicit service and set `ENGRAM_QDRANT_TEST_ADDR`, or make `TestMain` fail rather than skip when a dedicated CI-required environment flag is set.

- Add the relevant CI/test-harness file to this plan’s `files_modified`, since the present file list cannot establish the stated acceptance criterion.

### Risk Assessment

**MEDIUM.** The parity tests are excellent, but the real authorization implementation can still go untested in a green CI run.

---

## 17-06 — Typed read core

### Summary

The typed read-core plan correctly preserves both transport contracts and addresses the previously identified pagination, limit, time parsing, and query-default differences. No net-new concern found.

### Strengths

- The offset/cursor split matches store behavior: the store rejects mixed modes at [internal/store/store.go:844](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:844), emits cursors only in cursor mode at [internal/store/store.go:865](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:865), and treats `limit=0` as all records at [internal/store/store.go:873](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:873).

- Parsing time strings at each transport boundary avoids Connect error-code misclassification and keeps the core transport-neutral.

- The round-5 search test assignment is correct: `searchMemory` invokes `EmbedQuery` before touching the store at [internal/server/tools.go:870](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:870), so an anonymous caller preserves the existing stop-before-store test intent.

- Retaining the discovery-search MCP default while applying Connect’s default at its adapter is the right per-lane-default pattern.

### Concerns

- None net-new.

### Suggestions

- Keep separate regression tests for MCP’s tokenless cursor first page and Connect’s `limit=0` all-record behavior; they validate different adapter contracts over the same core.

### Risk Assessment

**LOW.**

---

## Final Risk Assessment

**Overall: MEDIUM.**

No high-severity design flaw remains, and the phase goals are achievable. Before execution:

1. Make Plan 17-01’s signature change and external call-site updates atomic.
2. Replace or explicitly harden Plan 17-02’s two-operation payload mutation.
3. Include the generated TypeScript output.
4. Make the real-Qdrant CI gate fail closed instead of silently skipping.

The other round-5 revisions are source-consistent and do not regress the earlier review fixes.

---

## OpenCode Review
*(openrouter/x-ai/grok-4.5, source-grounded — real repo reads observed; cites orchestrator-verified)*

# Phase 17 Plan Review — Round 6

Verified against live sources (`internal/auth/auth.go`, `internal/webauth/{session,resolver,oidc,handlers_test,oidc_exchange_test}.go`, `internal/server/{tools,connectapi,connectcsrf_test,embed_wiring_test,summary}.go`, `internal/store/store.go`, proto). Round-5 call-site and semantics fixes check out; no regression of earlier security/parity dispositions.

---

## Overall

**Synopsis:** Six-plan, four-wave design still meets REQ-connect-write-authz-parity: ordered injective owner claims + session migration (17-01), `memStore` + `caller` + content-mask/payload path (17-02), protoconv (17-03), typed-read core (17-06), thin Connect writers + `connectError` + spy (17-04), spy parity + leak tables (17-05). Round-5 mechanical compile gates (csrf/webauth/embed tests), `DeletePayload` provenance clear, second-granular scheduling scope, and dropped `CodeAborted` are consistent with the code.

**Risk: LOW–MEDIUM** — residual risk is mostly executor enumeration of signature call sites and post-deploy non-email remapping, not plan design holes.

---

## 17-01 — Owner-claim list + injective encoding + session versioning

### Strengths
- Landing in `ClaimIdentity` (`auth.go:83-97`) correctly fans out to MCP + cookie (`TokenVerifier` + `webauth` exchange).
- Injectivity + reserved-namespace email guard are correct given store match on owner (`store.go:497/517`).
- Session version in `Seal` + legacy cookie reject matches `Session`/`Resolve` shape (`session.go:26-29`, `resolver.go:54`).
- `handlers_test.go:159` / `oidc_exchange_test.go:148` call-site coverage is real and required for compile.
- `RemapOwner(..., dryRun bool)` (`store.go:1895`) + dry-run docs match implementation.

### Concerns
- **LOW** — Empty-string email fallthrough under `[email,sub]` (T-17-15) widens fail-closed `[email]` empty+unverified from hard reject to empty→next claim. Documented and safe; keep the unit pin.
- **LOW** — `ownerClaimGuard` must take `[]string` and warn when `email` is *absent* (today: `ownerClaim != "email"`, `serve.go:260-270`). Fine, but update `serve_test.go` with the list form so the warning/empty semantics stay green.

### Suggestions
- Explicit acceptance: `[email]`-only deployments still end fail-closed (empty owner → login/`SubjectFromTokenInfo` reject), even if `ClaimIdentity` no longer errors on empty+unverified email.

### Risk
**MEDIUM** (authz-key migration); mitigations in-plan.

---

## 17-02 — memStore, payload update, caller seam, write-path rewire

### Strengths
- `deps.st *store.Store` (`tools.go:35`) is still concrete → interface extraction is real.
- `storeFill` / `buildUsageQueue` stay `*store.Store` (`summaryqueue.go:119`, `tools.go:265`); test break at `summaryqueue_test.go:402/422`, `tools_test.go:1915` is correctly fixed via `testDepsWithStore`.
- `errStaleSummary` already at `summary.go:16` — reuse, no redeclare.
- Content landmine: plain `Content string` + unconditional embed (`tools.go:507-513,972-980`) vs proto tags-only “does not re-embed” (`engram.proto:150-156`) — `*string` + vector vs payload split is the right fix.
- Provenance clear must be `DeletePayload` (decoder re-parses zero-time / present keys, `store.go:433-439`); round-5 wording is correct.
- `embed_wiring_test.go:52` store-path caller update is needed; anonymous + stop-before-store matches the recording embedder.

### Concerns
- **LOW** — Payload-only = two Qdrant ops (`SetPayload` + `DeletePayload`); last-writer triage with async summarizer is acceptable if documented in the store method comment.
- No net-new HIGH issues.

### Suggestions
- Acceptance grep for production write callers is already implied; keep the repo-wide `rg` gate from the plan as the compile proof.

### Risk
**MEDIUM** (subtle store semantics); well-specified.

---

## 17-03 — protoconv (RFC3339Nano, mask → nil Content)

### Strengths
- `parseWindow` uses `time.Parse(time.RFC3339, ...)` (`tools.go:452-470`); Go accepts fractional input under that layout — RFC3339Nano at the adapter is correct for validation-boundary precision only.
- Store still floors windows via `.Unix()` (`store.go:320-323,406-410`) — second-granular scheduling scope is accurate; round-5 framing avoids overclaiming.
- Exact-mapping (not naive round-trip) fits optional-field lossiness.

### Concerns
None net-new.

### Risk
**LOW**

---

## 17-06 — Typed read core (superset; per-lane defaults)

### Strengths
- Today’s list core drops total/offset/categories/visibility and hardcodes `CursorMode: true` + `Limit==0→20` (`tools.go:793-820`); naïve rewire would regress Connect. Superset + per-lane defaults is right.
- Search k: Connect 20 vs deps 8 (`connectapi.go:162-164` vs `tools.go:855-856`); discovery k retention with 17-04 adapter default matches code.
- `identityForLog` still needs `subjectFromContext` (`instrument.go:80-82`) — keep helpers.

### Concerns
- **MEDIUM (net-new residual)** — Direct test call sites still use `listArgs`/`searchArgs` and rely on *internal* string parse + hard-coded cursor mode:
  - `TestListMemoryRejectsBadWindow` (`tools_test.go:1619-1623`) expects `parseRFC3339` inside `listMemory` — will not compile / lose intent after core takes `time.Time`.
  - Cursor page tests (`tools_test.go:1584+`, tag/list tests) assume internal `CursorMode: true`; without `CursorMode: true` on every direct `coreListRequest` that asserts `next`, they go red or silently empty `next`.
  - Plan says “update call sites” and MCP-closure `CursorMode: true`, but does **not** enumerate relocating the bad-window test to the MCP/parse boundary or forcing `CursorMode: true` on existing deps-level list tests.

### Suggestions
- In 17-06 Task 2, name: (1) rewrite/move `TestListMemoryRejectsBadWindow` to the transport/parse boundary; (2) any deps-level list test that expects `next` must set `CursorMode: true` (mirror MCP).

### Risk
**MEDIUM** until Task 2 explicitly covers those tests; then LOW.

---

## 17-04 — Handlers, connectError, spy, CSRF matrix

### Strengths
- No write methods on `engramAPI` today; stub → real body is correct.
- Landmines verified: `TestWriteRPCNegativeMatrix` `d := &deps{}` (`connectapi_negative_test.go:64`); CSRF happy path expects `CodeUnimplemented` (`connectcsrf_test.go:225,250`).
- Read rewires match bugs: double usage enqueue (`connectapi.go:211` + `tools.go:1000-1003`); double `EmbedQuery` (`connectapi.go:158/220` vs deps embed); empty discovery scope vs `effectiveDiscoveryScope` (`tools.go:884-892` vs `connectapi.go:228`); Connect list `limit=0` = all.
- `ErrAmbiguousShortID` (`store.go:56,1217`) → FailedPrecondition is good; no CodeAborted is correct (no conflict sentinel today).
- Leaving `TestNoAnonymousWrite` / empty-owner CSRF on bare `&deps{}` is safe (interceptor rejects pre-handler).

### Concerns
- None beyond sequencing (depends on 17-03 + 17-06 already stated).

### Risk
**MEDIUM** (large surface); gates are sound.

---

## 17-05 — Parity + cross-owner tables + idempotency re-assert

### Strengths
- Spy delegation + AST/source assertion closes storeMemory/scheduleMemory trace aliasing.
- Per-lane fixtures prevent order-dependent false parity.
- Actor assertion is lane-appropriate (MCP `UserID` aka `auth.go:139` vsConnect owner fallback `resolver.go:54`) — not false cross-lane equality.
- Split short_id vs UUID leak tables match D-11.
- Idempotency ban still live (`Taskfile.yaml:141-142`, `ci.yaml:126-127`).

### Concerns
None net-new.

### Risk
**LOW**

---

## Cross-plan / round-5 integrity

| Round-5 claim | Source check | Status |
|---|---|---|
| CSRF matrix files_modified + flip Unimplemented | `connectcsrf_test.go:225,250` | Sound |
| webauth test NewAuthenticator lists | `handlers_test.go:159`, `oidc_exchange_test.go:148` | Sound |
| embed_wiring store (02) + search (06) | `:52` / `:38` | Sound; wave order correct |
| DeletePayload vs zero-time | `store.go:436-438` re-parses present keys | Sound |
| Second-granular schedule | `store.go:320-323` | Sound |
| No CodeAborted | sentinels `store.go:42-56`, `summary.go:16` | Sound |

No round-1–4 security disposition is broken by these edits.

---

## Summary verdict

**Executable as written for execution**, with **one medium residual**: tighten **17-06 Task 2** so existing list/search unit tests (`CursorMode`, bad `created_after`) are named and migrated, not only MCP registration + embed_wiring.

Phase goals are achievable: one shared `deps.*` path, authz parity under test, SC4/SC5 covered, secure-phase hooks for owner-encoding intact.

**Overall risk: LOW–MEDIUM** — plan quality is high after five revision rounds; remaining work is disciplined execution + 17-06 test enumeration.

---

## Antigravity Review
*(agy 1.1.1 — SEE CAVEATS: tail-truncated to 17-01..17-05, substance in an artifact file, and its findings re-raise round-5 fixes as if still open. The stdout capture below is a summary + clarifying questions, not a grounded plan verdict. Retained for completeness; not weighted in consensus.)*

I have reviewed the Phase 17 implementation plans (`17-01-PLAN.md` through `17-05-PLAN.md`) in detail. 

The complete structured feedback, highlighting compile blockers, security vulnerabilities, and logic regressions, has been compiled into the following artifact:
*   [plan_review_feedback.md](file:///Users/sean/.gemini/antigravity-cli/brain/612d2365-3f64-4de2-a2e6-ccdf8db60456/plan_review_feedback.md)

### Key Decisions / Actions Requiring Your Input:

1.  **Strict Parser Config Contract & Plural Key:** Do you prefer using the singular `ENGRAM_OWNER_CLAIM` as a comma-separated list or defining a new plural `ENGRAM_OWNER_CLAIMS` key? (We recommend reusing the singular key to maintain compatibility with single-value deployments without changing the config surface).
2.  **`memStore` Complete RETYPE (Blockers):** Will you approve adding [DeleteAll](file:///Volumes/Code/github.com/seanb4t/engram/internal/store/store.go#L1508) and [ListScopes](file:///Volumes/Code/github.com/seanb4t/engram/internal/store/store.go#L1110) to the [memStore](file:///Users/sean/.gemini/antigravity-cli/scratch/untruncated_user_prompt.md#L1656) interface in Plan 02? This is required to prevent compile errors.
3.  **Unified Error Mapping:** Should we adopt a centralized [connectError](file:///Users/sean/.gemini/antigravity-cli/scratch/untruncated_user_prompt.md#L1969) mapper in Plan 04 rather than inline error mapping inside the handlers?
4.  **Active Workspace Recommendation:** You do not have an active workspace set. I recommend setting `/Volumes/Code/github.com/seanb4t/engram` as your active workspace for implementing these changes.

***

### Summary of Work
*   **Inspected codebase structure** at `/Volumes/Code/github.com/seanb4t/engram` to check `ClaimIdentity` and Connect read/write handlers.
*   **Identified and verified** multiple compile blockers, security vulnerabilities (unverified email bypass, non-injective namespace collisions, unversioned cookie migration risk), and logic regressions in `K` defaulting.
*   **Created the review feedback report** at [plan_review_feedback.md](file:///Users/sean/.gemini/antigravity-cli/brain/612d2365-3f64-4de2-a2e6-ccdf8db60456/plan_review_feedback.md).

---

## Consensus Summary

Two source-grounded reviewers (Codex, OpenCode/grok-4.5) reviewed the round-5-incorporated plans
(commit e47c59b6) against the live tree; every net-new `file:line` cite below was independently
re-verified by the orchestrator and **confirmed**. Antigravity is excluded (truncated + stale — see caveats).

**Verdict: MEDIUM — execution-mechanics refinements only, no design flaws, round-5 fixes verified sound.**
Both reviewers confirm the six-plan design still meets REQ-connect-write-authz-parity and that the round-5
edits (new test-file assignments, anonymous-caller stop-before-store, SetPayload key-deletion,
second-granular scheduling doc, dropped CodeAborted, T-17-15) are all source-consistent and regress nothing
from rounds 1-4. The 4 MEDIUM + 2 LOW residuals are about *how* the work executes (task boundaries, atomicity,
CI enforcement, test intent), not *what* it builds.

**Notable: zero convergence this round.** Codex and grok-4.5 found entirely DIFFERENT net-new issues —
complementary coverage, not conflicting verdicts. The round-4→5→6 trend is HIGH→MEDIUM: round 5 had 3 HIGH
compile blockers; round 6 has 0 HIGH. This is convergence toward execute-ready.

### Agreed Strengths (both reviewers, verified)
- Round-5 integrity confirmed end-to-end: CSRF matrix flip (`connectcsrf_test.go:225,250`), webauth `NewAuthenticator`
  `[]string` call sites (`handlers_test.go:159`, `oidc_exchange_test.go:148`), `embed_wiring_test.go` store(02)/search(06)
  split with anonymous stop-before-store caller (`:52`/`:38`, embedder errors first at `:23`), `DeletePayload`
  vs zero-time provenance (`store.go:433-439`), second-granular scheduling (`store.go:319-323/405-411`), no
  `CodeAborted` (sentinels `store.go:42-56` + `summary.go:16`). All "Sound."
- Injective owner encoding + session-version-in-`Seal` + legacy-cookie reject are grounded on the real authz
  mechanism (`store.go:493/514`, `handlers.go:172`, `resolver.go:44/54`) and the dry-run `RemapOwner` runbook (`store.go:1890`).
- 17-06 typed superset read-core correctly preserves per-lane defaults (offset/cursor split `store.go:844/865`,
  `limit=0`=all `store.go:873`, MCP k=8 vs Connect k=20, transport-boundary time parsing).
- 17-04 read rewires match real bugs (double usage enqueue `connectapi.go:211`, double `EmbedQuery` `:158/:220`,
  empty-scope discovery `tools.go:884`), and 17-05 parity/leak/AST design is strong.

### Agreed Concerns
None convergent — each reviewer's net-new findings are disjoint (see below). Both agree overall risk is MEDIUM
and no finding blocks the phase goal.

### Codex net-new (grounded, all orchestrator-CONFIRMED)

**[MEDIUM] 17-01: Task 1 cannot pass its own lint/acceptance gate before Task 2 (sequencing bug).**
Task 1 `<files>` is only `internal/auth/{auth,auth_test}.go` and it updates "in-package" call sites, but it
changes `auth.New`/`ClaimIdentity` to `[]string` while external scalar call sites remain in
`cmd/engram/serve.go:152`, `:284`, and `internal/webauth/oidc.go:78`. Task 1's own acceptance requires
module-wide `task lint:go` (`golangci-lint run ./...`, Taskfile.yaml:64) AND `grep -rn 'ClaimIdentity(' internal/ cmd/`
showing every call uses a slice (17-01-PLAN.md:38-39) — a state only reachable after Task 2. → deterministic
Task-1 commit failure.
→ *Fix:* make the public signature change + all external call-site updates atomic (one task/commit), OR scope
Task 1's gate to `go test ./internal/auth/...` and make Task 2 the first full-module lint/grep gate.

**[MEDIUM] 17-02: `SetPayload` + `DeletePayload` is a non-atomic partial-write path.**
The action mandates the provenance clear as "a distinct op alongside the SetPayload" (17-02-PLAN.md:179) — two
separate Qdrant RPCs. If `SetPayload` succeeds and `DeletePayload` fails, visibility/summary/access-count/timestamp
have already changed while stale provenance remains; the caller observes a failed mutation that partially committed.
→ *Fix:* prefer a single `OverwritePayload(toPayload(cur))` request — `toPayload` (`store.go:304`) already omits
cleared provenance keys while retaining all other fields, and whole-payload overwrite preserves the vector
(one RPC, atomic). If two ops are retained, document non-atomic semantics + add a fault-injection test for
failure between the two.

**[MEDIUM] 17-05: "not skipped in CI" is not enforced — the real-Qdrant authz gate can silently no-op.**
17-05 asserts the isolation suite is "not skipped in CI" (17-05-PLAN.md:188), but CI merely runs `go test ./...`
(ci.yaml:33), and the integration harness calls `t.Skip("no Qdrant available…")` (`tools_test.go:195`, `:1449`)
whenever the container fails to start. A Docker/image-pull failure therefore produces a GREEN CI run without the
real-store authorization gate — the phase's primary parity guarantee.
→ *Fix:* provision Qdrant as an explicit CI service + set `ENGRAM_QDRANT_TEST_ADDR`, OR make the harness FAIL
(not skip) when a CI-required env flag is set; add the CI/harness file to 17-05 `files_modified` (the current file
list cannot establish the stated acceptance criterion).

**[LOW] 17-02: regenerated TypeScript omitted from `files_modified`.**
The plan changes a proto-source comment and runs full `buf generate`, but lists only `gen/go/engram/v1/engram.pb.go`
(17-02-PLAN.md:11). buf also emits TypeScript (buf.gen.yaml:21-23, `out: gen/ts`); the exact comment exists at
`gen/ts/engram/v1/engram_pb.ts:665`; and CI drift-checks ALL of `gen/` (`git diff --exit-code -- gen/`, ci.yaml:123).
→ *Fix:* add `gen/ts/engram/v1/engram_pb.ts` to `files_modified`.

**[LOW — suggestion] 17-04:** in the CSRF happy-path cell, assert `err == nil` for scripted success rather than
treating `connect.CodeOf(nil)` as a "success code" — `CodeOf` returns `CodeUnknown` for nil / non-Connect errors.

### grok-4.5 net-new (grounded, orchestrator-CONFIRMED)

**[MEDIUM] 17-06 Task 2: existing list-test intent is lost, not just recompiled.**
`TestListMemoryRejectsBadWindow` (`tools_test.go:1619-1623`) passes `CreatedAfter: "nope"` (a string) expecting the
core's internal `parseRFC3339` to reject it — but after the core takes `time.Time` (round-4 MED-6 moves parsing to
the transport boundary), that test can't compile *and its intent evaporates* (an invalid window can't be handed to a
`time.Time` field). Likewise, existing deps-level cursor-page tests assume the core's hard-coded `CursorMode: true`;
once the core carries the flag from the request, any direct `coreListRequest` asserting `next` must set
`CursorMode: true` or it goes red / silently empties `next`. 17-06 says "update any direct call sites" generically
but never *names* relocating the bad-window rejection to the MCP/parse boundary nor forcing `CursorMode: true` on the
existing deps-level list tests — a mechanical signature fix would make them compile-but-meaningless.
→ *Fix:* in 17-06 Task 2, name (1) rewrite/move `TestListMemoryRejectsBadWindow` to the transport/parse boundary
where `parseRFC3339` now lives; (2) any deps-level list test that asserts `next` must set `CursorMode: true` (mirror MCP).

**[LOW] 17-01:** T-17-15 empty-string-email `[email,sub]` widening is documented and safe — keep the unit pin
(no change needed). grok also flagged `serve_test.go` for the `ownerClaimGuard` `[]string` change — **already covered**:
`cmd/engram/serve_test.go` is in 17-01's `files_modified` (line 13) and its Task 2 `<files>`/action (lines 218/224).
No action (grok false-negative).

### Divergent Views
No contradictions. The two grounded reviewers found disjoint net-new issues (Codex: 17-01 sequencing / 17-02
atomicity / 17-05 CI enforcement / gen-ts; grok: 17-06 test-intent), which is complementary coverage. Both rate
overall risk MEDIUM/LOW-MEDIUM and agree the design is sound and round-5 fixes hold. Antigravity's apparent
"divergence" (re-raising fixed test-file omissions) is a truncation/staleness artifact, not a real disagreement.

### Required round-6 changes (rank-ordered, all orchestrator-CONFIRMED)
1. **MEDIUM — 17-01:** make the `[]string` signature change + all external call-site updates atomic (or rescope Task 1's lint/grep gate to `internal/auth`). *[Codex]*
2. **MEDIUM — 17-02:** replace two-op `SetPayload`+`DeletePayload` with a single atomic `OverwritePayload(toPayload(cur))` (or document non-atomicity + fault-injection test). *[Codex]*
3. **MEDIUM — 17-05:** make the real-Qdrant CI gate fail-closed (Qdrant service + `ENGRAM_QDRANT_TEST_ADDR`, or fail-not-skip under a CI flag); add the CI/harness file to `files_modified`. *[Codex]*
4. **MEDIUM — 17-06:** in Task 2, explicitly relocate `TestListMemoryRejectsBadWindow` to the parse boundary + force `CursorMode: true` on existing deps-level list tests asserting `next`. *[grok]*
5. **LOW — 17-02:** add `gen/ts/engram/v1/engram_pb.ts` to `files_modified`. *[Codex]*
6. **LOW — 17-04:** CSRF happy-path cell asserts `err == nil`, not `CodeOf(nil)` as a success code. *[Codex]*

None of these block the phase goal or indicate a design flaw. They can be landed in one more targeted
`/gsd-plan-phase 17 --reviews` pass, or accepted as executor-time refinements — the six-plan design is
otherwise execute-ready.
