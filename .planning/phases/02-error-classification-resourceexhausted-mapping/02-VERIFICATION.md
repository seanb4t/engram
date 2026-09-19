---
phase: 02-error-classification-resourceexhausted-mapping
verified: 2026-09-19T13:10:00Z
status: passed
score: 9/9 must-haves verified
covered_files:
  - .planning/phases/02-error-classification-resourceexhausted-mapping/02-01-PLAN.md
  - .planning/phases/02-error-classification-resourceexhausted-mapping/02-01-SUMMARY.md
  - .planning/phases/02-error-classification-resourceexhausted-mapping/02-02-PLAN.md
  - .planning/phases/02-error-classification-resourceexhausted-mapping/02-02-SUMMARY.md
  - .planning/phases/02-error-classification-resourceexhausted-mapping/02-03-PLAN.md
  - .planning/phases/02-error-classification-resourceexhausted-mapping/02-03-SUMMARY.md
  - .planning/phases/02-error-classification-resourceexhausted-mapping/02-04-PLAN.md
  - .planning/phases/02-error-classification-resourceexhausted-mapping/02-04-SUMMARY.md
  - cmd/engram/catalog.go
  - cmd/engram/catalog_test.go
  - cmd/engram/client_common.go
  - cmd/engram/client_common_test.go
  - cmd/engram/exitcode_baseline_test.go
  - cmd/engram/operror.go
  - cmd/engram/operror_test.go
  - cmd/engram/testdata/catalog.golden
  - docs-site/src/content/docs/guides/cli.md
  - docs-site/src/content/docs/guides/upgrade.md
  - docs-site/src/content/docs/reference/errors.md
  - internal/server/argerror.go
  - internal/server/connecterror.go
  - internal/server/connecterror_test.go
  - internal/server/hintcodedocs_test.go
  - internal/server/instrument.go
  - internal/server/responsetoolarge.go
  - internal/server/responsetoolarge_test.go
  - internal/server/tools.go
  - internal/store/redevidence_harness_test.go
  - internal/store/responsetoolarge.go
  - internal/store/responsetoolarge_oversized_test.go
  - internal/store/responsetoolarge_test.go
  - internal/store/store.go
covered_digest: "v1:sha256:4134a2d98b6377f15c27e01085d43bc0c23dbcb276c5d53af07e5b99c2384c5f"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 2: Error Classification & ResourceExhausted Mapping Verification Report

**Phase Goal:** Classify a Qdrant response that exceeded the client's receive limit into
one typed sentinel — matching both the gRPC `ResourceExhausted` code and the receive-limit
message shape, so an unrelated server-capacity `ResourceExhausted` is never relabeled —
then map that sentinel once per lane at each lane's existing chokepoint: a new
`resource_exhausted` arm in `connectError` (never `internal`), a new single MCP-side
mapper, and a documented CLI exit code plus a docs-site `reference/errors.md` entry.

**Verified:** 2026-09-19T13:10:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A Qdrant response overflowing the client's receive limit classifies into one typed sentinel (`store.ErrResponseTooLarge` / `*store.ResponseTooLargeError`), matching BOTH the gRPC code and the receive-limit message shape | ✓ VERIFIED | `internal/store/responsetoolarge.go` (`classifyResponseTooLarge`, `isRecvLimitMessage` requiring `grpc: ` prefix + `larger than max` + `received message`/`message after decompression`); real end-to-end test `TestStoreListOverflowIsResponseTooLarge` PASS (both `few-large` and `many-small` fixtures) |
| 2 | An unrelated server-capacity `ResourceExhausted` is never relabeled | ✓ VERIFIED | `isRecvLimitMessage` guard excludes send-side shapes, unprefixed text, and server `Too many requests`; `TestClassifyResponseTooLarge` table covers all these cases and passes; `TestResponseTooLargeClassifierSitsInsideCallerChain/server-sent_too_many_requests_passes_through_unrelabeled` PASS over a real chained interceptor |
| 3 | The classifier is installed exactly once, in `NewQdrantClient`'s base dial options | ✓ VERIFIED | `internal/store/store.go:530` `grpc.WithChainUnaryInterceptor(classifyResponseTooLarge)`; no second construction site (Phase 1's `TestQdrantClientConstructedOnlyByNewQdrantClient` unaffected) |
| 4 | Connect maps the sentinel to `resource_exhausted`, never `internal` | ✓ VERIFIED | `internal/server/connecterror.go`: dedicated `errors.Is(err, store.ErrResponseTooLarge)` arm → `connect.CodeResourceExhausted`, placed before the `default` arm that would otherwise return `CodeInternal`; real HTTP round-trip test `TestConnectListMemoriesResponseTooLarge` PASS |
| 5 | A single MCP-side mapper maps the sentinel to the same envelope (no tool closure surfaces the raw error) | ✓ VERIFIED | `internal/server/responsetoolarge.go` (`mapResponseTooLarge`, an `mcp.Middleware`), wired exactly once via `instrument.go:75` `addToolMiddleware` → `tools.go:2262`; `TestMCPListMemoryResponseTooLarge`, `TestRegisterInstallsToolMiddleware`, `TestMapResponseTooLargePassesOtherResultsThrough` all PASS |
| 6 | Both lanes render the identical shared envelope (`field=response hint=too_large: ...`), never a second hand-built string | ✓ VERIFIED | `renderHintEnvelope` in `argerror.go` is the one renderer; `responseTooLargeEnvelope()` in `internal/server/responsetoolarge.go` calls it and both `connectError` and `mapResponseTooLarge` call `responseTooLargeEnvelope()`; `TestResponseTooLargeEnvelopeShape` PASS |
| 7 | CLI maps the failure to a documented, dedicated exit code (both the Connect client tier and the operator tier) | ✓ VERIFIED | `cmd/engram/client_common.go`: `exitTooLarge = 10` after `exitSetupFailed = 9`; `exitCodeForConnectErr(CodeResourceExhausted) → exitTooLarge`; `cmd/engram/operror.go:118-125` `classifyOperatorErr` maps `store.ErrResponseTooLarge` → `exitTooLarge`; `TestExitCodeForConnectErrTable`, `TestExitCodeTimeoutDistinctFromUnavailable`, `TestClassifyOperatorErrCodesAreDistinct`, `TestCatalogExitCodesMatchMapper` all PASS |
| 8 | The hint code and exit code are documented in docs-site `reference/errors.md` and `guides/cli.md`, mechanically kept in sync with source | ✓ VERIFIED | `errors.md` "eleven hint codes" table has a `too_large` row and a `## Response too large: resource_exhausted and exit 10` section; `cli.md`'s exit-code table has row `10`; `guides/upgrade.md` entry 14 documents the before/after; `internal/server/hintcodedocs_test.go`'s `TestErrorsDocHintCodesMatchArgErrorConstants`/`TestParseHintCodeTable` PASS (derives the vocabulary from `argerror.go`'s AST, not a hand-typed list) |
| 9 | Each lane's mapping is regression-proven RED-before-GREEN (so later phases assert the right failure mode) | ✓ VERIFIED | `internal/store/redevidence_harness_test.go`'s `redEvidenceDirs` carries 8 new Phase 2 patches; `TestRedEvidencePatchesAreLive` confirms all 12 (4 Phase 1 + 8 Phase 2) RED and PASS overall, tree left clean after each apply/revert cycle |

**Score:** 9/9 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/store/responsetoolarge.go` | `ErrResponseTooLarge`, `ResponseTooLargeError`, `classifyResponseTooLarge`, `isRecvLimitMessage` | ✓ VERIFIED | Present, substantive (154 lines), matches all documented shapes |
| `internal/store/store.go` | Classifier installed in `NewQdrantClient` base dial options | ✓ VERIFIED | `grpc.WithChainUnaryInterceptor(classifyResponseTooLarge)` at line 530, ordered after otelgrpc, before caller opts |
| `internal/server/responsetoolarge.go` | `responseTooLargeEnvelope`, `mapResponseTooLarge` | ✓ VERIFIED | Present, wired via `addToolMiddleware` |
| `internal/server/connecterror.go` | `resource_exhausted` arm | ✓ VERIFIED | Dedicated arm present, before default `CodeInternal` fallthrough |
| `internal/server/argerror.go` | `HintTooLarge` (11th `HintCode`), `renderHintEnvelope` | ✓ VERIFIED | Both present; single-renderer convention upheld |
| `internal/server/instrument.go` | `addToolMiddleware` single registration | ✓ VERIFIED | `instrumentTools` outermost, `mapResponseTooLarge` innermost, one `AddReceivingMiddleware` call |
| `cmd/engram/client_common.go` | `exitTooLarge = 10` + mapping | ✓ VERIFIED | Present and mapped |
| `cmd/engram/operror.go` | operator-tier exit-10 arm | ✓ VERIFIED | Present |
| `docs-site/.../reference/errors.md` | eleven-code table, `too_large` row, exit-10 section | ✓ VERIFIED | Present, gated by `hintcodedocs_test.go` |
| `docs-site/.../guides/cli.md` | exit-code-10 row | ✓ VERIFIED | Present |
| `internal/store/redevidence_harness_test.go` | 8 registered Phase 2 red-evidence patches | ✓ VERIFIED | Present, all confirmed RED |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `store.go` | `responsetoolarge.go` | `NewQdrantClient` installs classifier once | ✓ WIRED | Confirmed by source + `TestResponseTooLargeClassifierSitsInsideCallerChain` |
| `connecterror.go` | `store.ErrResponseTooLarge` | `errors.Is` match, never string match | ✓ WIRED | Confirmed by source; real HTTP round-trip test passes |
| `instrument.go`/`tools.go` | `responsetoolarge.go` | `addToolMiddleware` registers `mapResponseTooLarge` | ✓ WIRED | Single registration site confirmed; `TestRegisterInstallsToolMiddleware` PASS |
| `argerror.go` (`renderHintEnvelope`) | `connecterror.go` + `responsetoolarge.go` | shared renderer, both lanes call `responseTooLargeEnvelope()` | ✓ WIRED | Confirmed by source read of both call sites |
| `client_common.go`/`operror.go` | `store.ErrResponseTooLarge` | exit-10 mapping on both tiers | ✓ WIRED | Confirmed by source + passing table tests |
| `hintcodedocs_test.go` | `argerror.go` + `errors.md` | AST-derived doc gate | ✓ WIRED | `TestErrorsDocHintCodesMatchArgErrorConstants` PASS |
| `redevidence_harness_test.go` | 8 Phase 2 `.patch` files | harness applies each, requires target test RED | ✓ WIRED | `TestRedEvidencePatchesAreLive` PASS, 12/12 confirmed RED |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go build ./...` | build entire repo | exit 0, no output | ✓ PASS |
| Classifier unit/table tests | `go test ./internal/store/... -run 'TestClassifyResponseTooLarge$\|...IsIdempotent\|...ConcurrencySafe\|...SitsInsideCallerChain' -race` | all subtests PASS | ✓ PASS |
| Real-Qdrant end-to-end overflow | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -run TestStoreListOverflowIsResponseTooLarge` | both fixture shapes PASS | ✓ PASS |
| Connect + MCP lane real round-trip | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... -run 'TestConnectListMemoriesResponseTooLarge\|TestMCPListMemoryResponseTooLarge\|TestRegisterInstallsToolMiddleware\|TestMapResponseTooLargePassesOtherResultsThrough\|TestResponseTooLargeEnvelopeShape'` | all PASS | ✓ PASS |
| CLI exit-code + catalog table tests | `go test ./cmd/engram/... -run 'TestExitCodeForConnectErrTable\|TestCatalogExitCodesMatchMapper\|TestExitCodeTimeoutDistinctFromUnavailable\|TestClassifyOperatorErrCodesAreDistinct'` | all PASS | ✓ PASS |
| Docs gate | `go test ./internal/server/... -run 'TestErrorsDocHintCodesMatchArgErrorConstants\|TestParseHintCodeTable'` | all PASS | ✓ PASS |
| Red-evidence harness (12 patches) | `go test ./internal/store/... -run TestRedEvidencePatchesAreLive` | PASS, 12/12 confirmed RED, tree clean | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|--------------|--------|----------|
| REQ-exhausted-sentinel | 02-01 | Classify into one typed sentinel matching code AND message shape | ✓ SATISFIED | Truths #1-3; `TestStoreListOverflowIsResponseTooLarge`, `TestClassifyResponseTooLarge` |
| REQ-exhausted-connect | 02-02 | Connect RPCs return `resource_exhausted`, never `internal`, scrubbed envelope | ✓ SATISFIED | Truth #4; `TestConnectListMemoriesResponseTooLarge` |
| REQ-exhausted-mcp | 02-02 | MCP tools return same envelope via a single mapper | ✓ SATISFIED | Truth #5; `TestMCPListMemoryResponseTooLarge` |
| REQ-exhausted-cli-docs | 02-03 | CLI maps to documented exit code; errors.md documents hint code | ✓ SATISFIED | Truths #7-8; `TestExitCodeForConnectErrTable`, `TestErrorsDocHintCodesMatchArgErrorConstants` |

No orphaned requirements: `.planning/REQUIREMENTS.md` maps exactly these four IDs to Phase 2, and all four are claimed across plans 02-01/02-02/02-03 and confirmed satisfied above.

### Anti-Patterns Found

None. Scanned all files listed in `covered_files` (source, tests, docs) for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER`/stub-language patterns — no matches. `02-REVIEW.md` (code review, depth standard, 24 files) found 0 critical, 0 warning, 1 info (a pre-existing stale line-number citation in an unrelated doc comment, not introduced by this phase) — consistent with independent findings here.

### Human Verification Required

None. All truths are either symbol-presence-and-wiring facts or state-transition/behavior facts directly exercised by a real Qdrant + real HTTP/in-memory-transport regression test that this verifier ran independently (not merely re-reading SUMMARY.md claims).

### Gaps Summary

No gaps. All four requirement IDs are satisfied, all plan-declared must-haves for all four plans (02-01 through 02-04) are verified against the live codebase — not just SUMMARY.md narrative — and the phase's own regression-proof mechanism (the red-evidence harness) independently confirms every lane fails without its mapping. The `internal/surfaces` verification the phase goal explicitly called out was addressed and recorded (surfaces covers conditional-rule sentences only; `hintcodedocs_test.go` is the actual doc-code gate). The deliberately dropped `internal/e2e` exit-10 binary test (no named receive limit exists on `engram serve` until Phase 5) is judged acceptable per the orchestrator's note: the chain is proven in two independently-verified halves (real-Qdrant classifier → Connect/MCP wire) that compose correctly, since both halves consume the identical `store.ErrResponseTooLarge` sentinel through the identical mapping code exercised by the CLI's own `exitCodeForConnectErr`/`classifyOperatorErr` unit tests.

---

*Verified: 2026-09-19T13:10:00Z*
*Verifier: Claude (gsd-verifier)*
