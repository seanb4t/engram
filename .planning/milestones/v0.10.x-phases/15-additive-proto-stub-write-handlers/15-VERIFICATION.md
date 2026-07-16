---
phase: 15-additive-proto-stub-write-handlers
verified: 2026-07-11T18:55:00Z
status: passed
score: 4/4 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 15: Additive Proto + Stub Write Handlers Verification Report

**Phase Goal:** The Connect wire contract for all six write RPCs exists, is additive-only, and is provably impossible to reach over an unauthenticated GET — before any business logic is wired behind it.
**Verified:** 2026-07-11T18:55:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (Roadmap Success Criteria, Phase 15)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `EngramService` exposes `StoreMemory`, `StoreDiscovery`, `UpdateMemory`, `DeleteMemory`, `SetVisibility`, `ScheduleMemory` as additive proto RPCs (no field renumbering), `gen/go`/`gen/ts` regenerated, CI drift check green | ✓ VERIFIED | `proto/engram/v1/engram.proto` lines 223-236 declares all 11 RPCs (5 existing read + 6 new write, unchanged field numbers on existing messages). `go tool buf generate` produces zero diff against the committed `gen/` tree (ran live: no output from `git status --short gen/` / `git diff --stat gen/` after regenerating). `go tool buf breaking --against 'https://github.com/seanb4t/engram.git#branch=main'` passes clean (additive-only vs main). |
| 2 | A CI lint/grep gate fails the build if any RPC in `engram.proto` carries `idempotency_level = NO_SIDE_EFFECTS`, asserted for all six new write RPCs | ✓ VERIFIED | `Taskfile.yaml` `proto:lint` (lines 136-144) runs `go tool buf lint` then greps `proto/` for the banned annotation with the anchored regex; ran `task proto:lint` live — exit 0, clean. `.github/workflows/ci.yaml` lines 124-129 mirror the identical grep inline in the `buf` job (no `setup-task`/`task` binary on the runner, confirmed via grep — no matches). `rg idempotency_level proto/` returns nothing. `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` (ran live, PASS) additionally asserts `IDEMPOTENCY_UNKNOWN` on all 11 methods via protoreflect, independent of the grep gate. |
| 3 | Calling any of the six write RPCs today returns `CodeUnimplemented`, not a panic/500; a raw HTTP GET against any write RPC's path returns non-2xx | ✓ VERIFIED | `engramAPI` (connectapi.go:29) embeds `engramv1connect.UnimplementedEngramServiceHandler` with no override method defined anywhere in `internal/server/*.go` for any of the six write RPCs — confirmed via symbol search (only test helpers/MCP tool functions named similarly exist, not Connect handler methods). `TestWriteRPCNegativeMatrix` (ran live, PASS, all 6 RPCs) proves: authenticated+valid → `CodeUnimplemented`; unauthenticated → `CodeUnauthenticated` (401-before-400 ordering, even with invalid payload); raw `http.Get` against the generated `...Procedure` constant → HTTP 405 for every write RPC; authenticated+invalid → `CodeInvalidArgument`. The interceptor chain (`connectapi.go:259-263`) confirms `newConnectValidateInterceptor` is wired as the last (innermost) argument to `WithInterceptors`, after `newConnectSubjectInterceptor` — auth strictly precedes validation. |
| 4 | The five existing read RPCs are unaffected — identical wire format and behavior, verified by a regression test | ✓ VERIFIED | `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` (ran live, PASS) pins a per-field table (number/name/kind/cardinality/message-type) for `Memory`, `ScopeCount`, and all five read request/response messages — not just message names, per the SC4 "identical wire format" requirement. `go tool buf breaking --against main` independently confirms no read-lane field was renumbered or retyped. |

**Score:** 4/4 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `proto/engram/v1/engram.proto` | 11 RPCs, 13 new messages/enum, buf.validate annotations | ✓ VERIFIED | Read in full; all six write RPCs + Visibility enum + Citation + per-message buf.validate rules present exactly as planned (D-01 through D-08 honored, including the UpdateMemory mask CEL and category `string.in` allowlist) |
| `buf.yaml` / `buf.lock` | protovalidate BSR dep declared + pinned | ✓ VERIFIED | `buf.yaml` has `deps: [buf.build/bufbuild/protovalidate]`; `buf.lock` exists and references the resolved module |
| `gen/go/engram/v1/`, `gen/go/engram/v1/engramv1connect/` | Regenerated message types + client/handler methods | ✓ VERIFIED | `UnimplementedEngramServiceHandler` methods for all 6 write RPCs present (engram.connect.go:381-404); `go build ./...` compiles |
| `gen/ts/engram/v1/` | Regenerated TS types | ✓ VERIFIED (not independently re-inspected beyond drift check) | `buf generate` produces no diff |
| `Taskfile.yaml`, `.github/workflows/ci.yaml` | Idempotency-ban gate, local + CI | ✓ VERIFIED | Both contain the identical anchored grep; live run of `task proto:lint` and `task lint:go` both green |
| `internal/server/connectvalidate.go` + test | Hand-rolled protovalidate interceptor | ✓ VERIFIED | Read in full; maps `*protovalidate.ValidationError`→`CodeInvalidArgument`, other errors→`CodeInternal`, non-proto passthrough; `TestConnectValidateInterceptor` (4 subtests) passes live |
| `internal/server/connectapi.go` (`mountConnect`) | Validator wired last in interceptor chain | ✓ VERIFIED | `WithInterceptors(otelIc, accessLog, newConnectSubjectInterceptor(resolve), newConnectValidateInterceptor(validator))` — validate is last argument |
| `internal/server/connectdescriptor_test.go` | Descriptor-walk regression test | ✓ VERIFIED | `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` passes live |
| `internal/server/connectapi_negative_test.go` | Full negative-path matrix | ✓ VERIFIED | `TestWriteRPCNegativeMatrix` (incl. mask cells + category cells) passes live |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `connectapi.go` `mountConnect` | `connectvalidate.go` `newConnectValidateInterceptor` | `connect.WithInterceptors(..., newConnectValidateInterceptor(validator))` as final arg | ✓ WIRED | Confirmed via source read; order is otel → access-log → subject (401) → validate (400) |
| `engram.proto` buf.validate annotations | runtime enforcement | `protovalidate.New()` interceptor validates every unary request message | ✓ WIRED | `TestConnectValidateInterceptor/invalid_message_returns_invalid_argument` uses the REAL validator, proving generated annotations load and enforce |
| `engramAPI` | `UnimplementedEngramServiceHandler` | Go struct embedding, no override methods for the 6 write RPCs | ✓ WIRED (intentionally stub) | Confirmed no manual implementation exists — CodeUnimplemented is the actual runtime behavior, not a documentation claim |
| `Taskfile.yaml` `proto:lint` grep | `.github/workflows/ci.yaml` `buf` job grep | Identical anchored regex, no shared script (by design) | ✓ WIRED | Both greps verified byte-identical in pattern; CI step confirmed present at ci.yaml:124-129 |

### Behavioral Spot-Checks / Live Gate Runs

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Proto lints clean | `go tool buf lint` | no output, exit 0 | ✓ PASS |
| Additive-only vs main | `go tool buf breaking --against 'https://github.com/seanb4t/engram.git#branch=main'` | no output, exit 0 | ✓ PASS |
| gen/ drift-free | `go tool buf generate` + `git status --short gen/` | no diff | ✓ PASS |
| No idempotency_level anywhere | `rg idempotency_level proto/` | no matches | ✓ PASS |
| Taskfile gate | `task proto:lint` | exit 0 | ✓ PASS |
| golangci-lint | `task lint:go` | "0 issues." | ✓ PASS |
| SPDX headers | `task license:check` | 172 valid, 0 invalid | ✓ PASS |
| Descriptor + negative-matrix + interceptor unit tests | `go test ./internal/server/ -run 'TestEngramServiceDescriptor\|TestWriteRPCNegativeMatrix\|TestConnectValidateInterceptor' -v` | all subtests PASS | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| REQ-connect-write-rpcs | 15-01, 15-02, 15-03, 15-04 | Six additive write RPCs, additive-only, no `idempotency_level = NO_SIDE_EFFECTS`, CI lint gate | ✓ SATISFIED | Marked `[x]` in REQUIREMENTS.md; proto, gates, interceptor, and tests all independently verified above. No orphaned requirements — REQUIREMENTS.md maps exactly this one ID to Phase 15. |

No orphaned requirements found for Phase 15.

### Anti-Patterns Found

None. Grep for `TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER` across all phase-modified files (`engram.proto`, `connectvalidate.go`, `connectvalidate_test.go`, `connectapi.go`, `connectdescriptor_test.go`, `connectapi_negative_test.go`, `Taskfile.yaml`, `ci.yaml`) returned zero matches. All four new/modified test and source files carry the required Apache-2.0 SPDX header.

### Human Verification Required

None. All truths were verifiable programmatically via live gate execution (buf lint/breaking/generate, grep gates, and the three named Go test suites), not merely SUMMARY narrative.

### Design Notes (Not Gaps)

- The six write RPC handler **bodies** intentionally return `CodeUnimplemented` via the embedded `UnimplementedEngramServiceHandler` — no business logic exists yet. This is the phase's explicit design (Phase 17 wires the handlers to `deps.*`). Confirmed this is not an oversight: no override methods exist for any of the six RPCs in `internal/server/`.
- The semantic application of `UpdateMemoryRequest.update_mask` (actually applying only the named fields) and the wall-clock "not_after in the future" check for `ScheduleMemoryRequest` are explicitly deferred to Phase 17 per proto comments — this phase enforces wire-shape only (mask presence/non-empty/allowlist, window shape), which is the correct scope boundary for a stub-only phase.
- `ui/src/lib/gen/engram_pb.ts` staleness (pre-existing drift predating Phase 15) was explicitly deferred by the plan's review notes as out of scope (frontend TS types, not the root `gen/` tree this phase's CI drift check covers) — correctly not a Phase 15 gap.

### Gaps Summary

No gaps found. All four roadmap Success Criteria for Phase 15 are independently verified against the live codebase (not SUMMARY claims): the wire contract exists and is additive-only (buf breaking clean, gen/ drift-free), the idempotency-ban gate is live in both Taskfile and CI, the six write RPCs are provably `CodeUnimplemented`/`CodeUnauthenticated`/405/`CodeInvalidArgument` per the exact matrix required, and the read lane is provably unaffected via a per-field descriptor pin test. Auth-before-validate ordering (401 before 400) is confirmed both by source inspection of the interceptor chain and by a live-passing negative-matrix cell (unauthenticated + invalid payload → `CodeUnauthenticated`, never leaking validation detail).

---

*Verified: 2026-07-11T18:55:00Z*
*Verifier: Claude (gsd-verifier)*
