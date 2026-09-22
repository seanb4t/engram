---
phase: 02-error-classification-resourceexhausted-mapping
plan: 01
subsystem: store
tags: [grpc, qdrant, error-classification, resourceexhausted, interceptor]

requires:
  - phase: 01-test-harness-fixture-helper
    provides: "store.NewQdrantClient(host, port, opts...); storetest.Dial/RecvLimit/SeedOversized (both fixture shapes)"
provides:
  - "store.ErrResponseTooLarge sentinel and store.ResponseTooLargeError (Method/Limit/Observed), errors.Is/As-matchable, GRPCStatus()-preserving"
  - "classifyResponseTooLarge gRPC unary client interceptor installed once in NewQdrantClient's base dial options"
  - "the classification boundary (four grpc-go v1.83.2 receive shapes in, everything else including server-side ResourceExhausted unchanged out) pinned by synthetic-status tests"
  - "a real-Qdrant end-to-end overflow regression proving the classifier fires on live traffic, not just synthetic input"
affects: ["02-02", "02-03", "02-04"]

actuals:
  tokens: 6520
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "gRPC unary client interceptor installed once in a shared constructor's base dial options, matching on status code AND message-shape substring (never code alone) so it stays transparent to an interceptor sitting outside it in the chain"
    - "stdlib errors.New sentinel + typed error carrying Unwrap()/GRPCStatus(), following store.go's existing ErrNotFound/ensureIndexes idioms"
    - "best-effort regex-parsed byte counts on an error type (Observed left at 0 when the upstream message shape does not report it, never a required field)"

key-files:
  created:
    - internal/store/responsetoolarge.go
    - internal/store/responsetoolarge_oversized_test.go
    - internal/store/responsetoolarge_test.go
  modified:
    - internal/store/store.go

key-decisions:
  - "Classifier lives inside NewQdrantClient's base dial options (D-01), appended after the otelgrpc stats handler and before the caller's own opts — the same base-then-caller order Phase 1 established, so every production and test client gets it automatically."
  - "isRecvLimitMessage matches strings.HasPrefix(msg, \"grpc: \") AND strings.Contains(msg, \"larger than max\") AND (strings.Contains(msg, \"received message\") OR strings.Contains(msg, \"message after decompression\")) — covers all four grpc-go v1.83.2 receive shapes and excludes both send shapes and any server-sent ResourceExhausted text, matching RESEARCH.md Pitfalls 1 and 2 exactly."
  - "ResponseTooLargeError.Observed is best-effort (0 when the single-number receive shape fires, per Pitfall 6) — never a required field, never silently defaulted to something misleading."
  - "Idempotency falls out of the design rather than needing a special case: a classified error's own Error() text never starts with \"grpc: \", so re-classifying it never matches isRecvLimitMessage and returns unchanged."

patterns-established:
  - "A non-primary file's doc comment in package store is separated from `package store` by a blank line (never directly attached), so revive's package-comments rule does not mistake it for the package doc — see responsetoolarge.go."

requirements-completed: [REQ-exhausted-sentinel]

coverage:
  - id: D1
    description: "A real 4 MiB-named overflow through Store.List (both fixture shapes) is classified as store.ErrResponseTooLarge with GRPCStatus() intact and a Method/Limit payload"
    requirement: "REQ-exhausted-sentinel"
    verification:
      - kind: integration
        ref: "internal/store#TestStoreListOverflowIsResponseTooLarge/few-large"
        status: pass
      - kind: integration
        ref: "internal/store#TestStoreListOverflowIsResponseTooLarge/many-small"
        status: pass
    human_judgment: false
  - id: D2
    description: "The classifier is installed exactly once, in NewQdrantClient's base dial options; no new Qdrant client construction site exists"
    requirement: "REQ-exhausted-sentinel"
    verification:
      - kind: unit
        ref: "rg -o 'grpc[.]WithChainUnaryInterceptor[(]classifyResponseTooLarge[)]' internal/store/store.go | wc -l -> 1"
        status: pass
      - kind: integration
        ref: "internal/store#TestQdrantClientConstructedOnlyByNewQdrantClient"
        status: pass
    human_judgment: false
  - id: D3
    description: "Exactly the four grpc-go v1.83.2 receive shapes classify; both send shapes, a server-sent status, an unprefixed receive-worded message, and a non-ResourceExhausted code carrying the receive shape all pass through unchanged"
    requirement: "REQ-exhausted-sentinel"
    verification:
      - kind: unit
        ref: "internal/store#TestClassifyResponseTooLarge"
        status: pass
    human_judgment: false
  - id: D4
    description: "A nil invoker result returns nil; an empty-message ResourceExhausted status and a plain non-status error both pass through unchanged"
    requirement: "REQ-exhausted-sentinel"
    verification:
      - kind: unit
        ref: "internal/store#TestClassifyResponseTooLarge/empty_message"
        status: pass
      - kind: unit
        ref: "internal/store#TestClassifyResponseTooLarge/nil_invoker_result"
        status: pass
    human_judgment: false
  - id: D5
    description: "The classified error keeps its gRPC ResourceExhausted status through status.FromError on the bare error, through qdrant-go-client's *QdrantError wrap, and through Store.List's return path; a caller interceptor added via grpc.WithChainUnaryInterceptor runs inside the classifier"
    requirement: "REQ-exhausted-sentinel"
    verification:
      - kind: integration
        ref: "internal/store#TestResponseTooLargeClassifierSitsInsideCallerChain"
        status: pass
      - kind: integration
        ref: "internal/store#TestStoreListOverflowIsResponseTooLarge"
        status: pass
    human_judgment: false
  - id: D6
    description: "Observed/Limit are parsed as exact integers from the two-number shapes (including a count above 2^31); the single-number shape sets Limit and leaves Observed at 0"
    requirement: "REQ-exhausted-sentinel"
    verification:
      - kind: unit
        ref: "internal/store#TestClassifyResponseTooLarge (four classify subtests)"
        status: pass
    human_judgment: false
  - id: D7
    description: "Classifying an already-classified error returns it unchanged — same fields, same Error() text"
    requirement: "REQ-exhausted-sentinel"
    verification:
      - kind: unit
        ref: "internal/store#TestClassifyResponseTooLargeIsIdempotent"
        status: pass
    human_judgment: false
  - id: D8
    description: "The classifier holds no mutable state: 64 concurrent classifications under -race each return their own method with no data race"
    requirement: "REQ-exhausted-sentinel"
    verification:
      - kind: unit
        ref: "internal/store#TestClassifyResponseTooLargeIsConcurrencySafe"
        status: pass
    human_judgment: false
  - id: D9
    description: "go.mod and go.sum are byte-unchanged — zero new Go dependencies"
    requirement: "REQ-exhausted-sentinel"
    verification:
      - kind: other
        ref: "git diff --exit-code HEAD -- go.mod go.sum"
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-09-19
status: complete
---

# Phase 2 Plan 1: Classify Receive-Limit Overflows as store.ErrResponseTooLarge Summary

**A gRPC unary client interceptor installed once in `NewQdrantClient`'s base dial options classifies a Qdrant response that exceeded the client's receive limit into `store.ErrResponseTooLarge`/`ResponseTooLargeError`, matching on gRPC code AND message shape so a genuine server-side `ResourceExhausted` is never relabeled — proven on real overflow traffic and pinned against 13 synthetic-status subtests.**

## Performance

- **Duration:** ~45 min
- **Completed:** 2026-09-19
- **Tasks:** 2 completed
- **Files:** 4 changed (3 created, 1 modified)

## Accomplishments

- `internal/store/responsetoolarge.go` — `ErrResponseTooLarge` sentinel, `ResponseTooLargeError` (`Error`/`Unwrap`/`GRPCStatus`), `classifyResponseTooLarge` gRPC unary client interceptor, and `isRecvLimitMessage` matching all four grpc-go v1.83.2 client-side receive-limit message shapes (and none of the two send-side shapes).
- `NewQdrantClient` (`internal/store/store.go`) installs the classifier exactly once, in its base dial options, right after the otelgrpc stats handler and before the caller's own opts — inside qdrant-go-client's own rate-limit interceptor, so a genuine server-side `ResourceExhausted` still reaches that interceptor's retry-after handling untouched.
- `TestStoreListOverflowIsResponseTooLarge` (`responsetoolarge_oversized_test.go`) — a real 4 MiB-named overflow through `Store.List`, both fixture shapes (`few-large`, `many-small`), observed RED (both subtests failing on `errors.Is`) before the base option landed and GREEN after.
- `TestClassifyResponseTooLarge`, `TestClassifyResponseTooLargeIsIdempotent`, `TestClassifyResponseTooLargeIsConcurrencySafe`, `TestResponseTooLargeClassifierSitsInsideCallerChain` (`responsetoolarge_test.go`) — 13 synthetic-status subtests pinning the boundary, idempotency under a double installation, race-freedom under `-race`, and the chain position against real Qdrant with an injected caller interceptor.

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end classification: a real overflow through NewQdrantClient's base interceptor surfaces as store.ErrResponseTooLarge** — `96ed0599` (feat)
2. **Task 2: The matcher's boundary: every receive shape classifies, nothing else does, and a caller interceptor sits inside it** — `23591c35` (test)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP)

## Files Created/Modified

- `internal/store/responsetoolarge.go` — sentinel, error type, interceptor, matcher, parser.
- `internal/store/store.go` — `NewQdrantClient` gains the base interceptor option (doc comment rewritten to name both base options and the chain-position rationale).
- `internal/store/responsetoolarge_oversized_test.go` — `TestStoreListOverflowIsResponseTooLarge` (package `store_test`, real Qdrant, both fixture shapes).
- `internal/store/responsetoolarge_test.go` — the four synthetic-status/chain-position tests (package `store`).

## Decisions Made

- Interceptor ordering, method-name identifiers, and file placement all followed the plan's own discretion notes without deviation — see `key-decisions` in frontmatter for the substantive ones (chain position, matcher shape, `Observed`'s best-effort semantics, and idempotency falling out of the design rather than a special case).
- The doc comment preceding `package store` in `responsetoolarge.go` was deliberately separated by a blank line from the `package store` line, so `golangci-lint`'s `revive` `package-comments` check does not treat it as (and reject) a second package doc comment — this is now the pattern for any future non-primary file in this package (see `patterns-established`).

## Deviations from Plan

None - plan executed exactly as written. The RED observation required by Task 1's `<behavior>` is documented below as normal flow, not a deviation.

**RED run (Task 1, before the base option landed):**

```
--- FAIL: TestStoreListOverflowIsResponseTooLarge (2.77s)
    --- FAIL: TestStoreListOverflowIsResponseTooLarge/few-large (0.48s)
    --- FAIL: TestStoreListOverflowIsResponseTooLarge/many-small (2.29s)
```

Raw error text both subtests printed (`errors.Is` check failing):

```
List: errors.Is(err, store.ErrResponseTooLarge) = false; err = Scroll() failed: store_oversized_toolarge_82e8194b-2395-41a6-92ed-33666b5e225f: rpc error: code = ResourceExhausted desc = grpc: received message after decompression larger than max 4194304
List: errors.Is(err, store.ErrResponseTooLarge) = false; err = Scroll() failed: store_oversized_toolarge_aa7a364b-bb84-4b4d-bca8-3145ed78458b: rpc error: code = ResourceExhausted desc = grpc: received message after decompression larger than max 4194304
```

After the base option landed, both subtests passed (GREEN), confirmed by re-running the same command.

**Total deviations:** 0. **Impact:** None — plan executed as written; the RED above was a required observation, not a correction.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `store.ErrResponseTooLarge` and `store.ResponseTooLargeError` are ready for plan 02-02 (Connect `connectError` arm, MCP receiving-middleware mapper) and plan 02-03 (CLI exit code) to map at their own chokepoints — the classifier is the sentinel every later lane and every later regression test asserts.
- `internal/store` stays green: the full suite (`ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -count=1`) passes, including Phase 1's D-11/D-13 gates and `TestRedEvidencePatchesAreLive` (unaffected — this phase's own red-evidence registration is plan 02-04's job, per RESEARCH.md Pitfall 7: the gate is green today and stays green).
- `task lint` exits 0 repo-wide; `git diff --exit-code HEAD -- go.mod go.sum` exits 0 — zero new Go dependencies.
- No blockers or concerns for plan 02-02.

---
*Phase: 02-error-classification-resourceexhausted-mapping*
*Completed: 2026-09-19*

## Self-Check: PASSED

All 4 created/modified files verified present on disk; both task commits (`96ed0599`, `23591c35`) verified present in `git log --oneline --all`. Every acceptance criterion for both tasks re-run and confirmed passing; `go vet ./internal/store/...` and `golangci-lint run ./internal/store/...` both clean; `task license:check` and `task lint` (repo-wide) both clean; `go.mod`/`go.sum` byte-unchanged; the full plan-level `<verification>` block (RED/GREEN both tasks, `-race` clean, full `internal/store` suite including Phase 1 gates and `TestRedEvidencePatchesAreLive`) all pass.
