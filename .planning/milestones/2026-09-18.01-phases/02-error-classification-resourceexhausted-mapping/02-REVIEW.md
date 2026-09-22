---
phase: 02-error-classification-resourceexhausted-mapping
reviewed: 2026-09-19T16:59:28Z
depth: standard
files_reviewed: 24
files_reviewed_list:
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
findings:
  critical: 0
  warning: 0
  info: 1
  total: 1
status: clean
---

# Phase 02: Code Review Report

**Reviewed:** 2026-09-19T16:59:28Z
**Depth:** standard
**Files Reviewed:** 24
**Status:** clean

## Summary

This phase adds `store.ErrResponseTooLarge` classification (a gRPC unary client
interceptor installed in `NewQdrantClient`'s base dial options), the shared
`renderHintEnvelope`/`field=response hint=too_large` envelope, a `connectError`
arm mapping to `resource_exhausted`, an MCP receiving middleware
(`mapResponseTooLarge`), and CLI `exitTooLarge = 10` on both the Connect client
tier and the operator tier. I read every file in scope in full, traced the
diff against the pre-phase commit (`0f096b7b`) to isolate the actual changed
regions in the two large files (`store.go`, `tools.go`), and cross-checked the
implementation against the pinned third-party sources it depends on:

- **grpc-go v1.83.2** (`/Users/sean/go/pkg/mod/google.golang.org/grpc@v1.83.2`):
  confirmed the four receive-limit message literals (`rpc_util.go:795,798,1008,1039`)
  and the two send-side shapes (`server.go:1215`, `stream.go:991/1496/1779`)
  match `isRecvLimitMessage`'s doc comment verbatim, and that
  `WithChainUnaryInterceptor`/`chainUnaryClientInterceptors` accumulate and
  execute in append order (index 0 outermost) as the code comments claim.
- **qdrant-go-client v1.19.2**: confirmed `NewGrpcClient` installs
  `getRateLimitInterceptor()` into its own base options *before* appending
  `config.GrpcOptions` (`grpc_client.go:38-53`), so `classifyResponseTooLarge`
  genuinely sits inside (closer to the wire than) qdrant-go-client's own
  rate-limit interceptor, exactly as the `NewQdrantClient` doc comment states.
  Traced `QdrantResourceExhaustedError.Unwrap()` to confirm `errors.Is(err,
  store.ErrResponseTooLarge)` would still survive even in the theoretical edge
  case where that interceptor wraps a classified error further.
- **go-sdk v1.8.0**: confirmed `addMiddleware`'s `slices.Backward` semantics
  produce the ordering `addToolMiddleware`'s doc comment describes
  (`instrumentTools` outermost, `mapResponseTooLarge` innermost), and that
  `CallToolResult.SetError`/`GetError` and the typed-handler error path
  (`server.go:425-433`, allocating a *fresh* `CallToolResult`) mean
  `mapResponseTooLarge`'s mutation of `Content`/`IsError` can never leak a
  stale `StructuredContent` value.

I also verified the doc-gate (`hintcodedocs_test.go`) against the live
`errors.md` (11 `HintCode` constants, "eleven" heading word, no stale
"ten"-word phrases anywhere under `docs-site/`), the exit-code catalog against
`client_common.go`'s constants (golden fixture, `wantExitCodes`,
`nonConnectProducedCodes` non-membership), the `exitcode_baseline_test.go` row
count (41, hand-counted against the literal cases), and that every named
red-evidence patch in `redEvidenceDirs` exists on disk and reverts exactly the
line(s) its comment claims (spot-checked `02-01-classifier-relabels-any-resource-exhausted.patch`
against `classifyResponseTooLarge`'s current guard).

I could not find a defect that rises to Critical or Warning. The
false-positive/false-negative risk explicitly flagged in this phase's context
(text-matching the receive-limit shape) is a deliberate, documented,
test-pinned tradeoff (`isRecvLimitMessage`'s own doc comment, rule
`m45p2b4bp7`, `TestClassifyResponseTooLarge`'s exhaustive passthrough table),
not an oversight. Middleware ordering, log-once behavior, `errors.Is`/
`status.FromError` preservation through both the interceptor and
qdrant-go-client's own wrapping, `CallToolResult` mutation safety, exit-code
table completeness (both directions, via `TestCatalogExitCodesMatchMapper`),
and the doc-gate's anti-false-green measures (heading-boundary cutoff,
whole-page stale-word scan, cross-file anchor scan) all check out under
independent tracing, not just by trusting the tests' own claims.

## Info

### IN-01: Stale line-number reference in argError's ConnectCode doc comment (pre-existing, not introduced by this phase)

**File:** `internal/server/argerror.go:118`
**Issue:** The doc comment on `(*argError).ConnectCode` says "All three arms
sit inside the trio `exitCodeForConnectErr` (`cmd/engram/client_common.go:237-249`)
already groups under `exitUsage`" — but `exitCodeForConnectErr` is now at
`cmd/engram/client_common.go:443-463` (and was already at line 432 before this
phase's edits, per `git show 0f096b7b:cmd/engram/client_common.go`). This
comment block was not touched by this phase's diff, so the drift predates
Phase 2 and this phase's own additions (the `exitTooLarge` const and case) are
unaffected and accurately commented. Flagging only because the file is in
this review's scope and a future reader following the citation will land in
the wrong place.
**Fix:** Drop the specific line-number citation (or replace it with a
function-name-only reference, e.g. "`cmd/engram/client_common.go`'s
`exitCodeForConnectErr`") the next time this comment is touched, so it can't
drift again when unrelated lines are inserted above it.

---

_Reviewed: 2026-09-19T16:59:28Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
