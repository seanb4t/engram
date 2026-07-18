---
phase: 24-idempotent-capture
reviewed: 2026-07-18T21:04:44Z
depth: deep
files_reviewed: 8
files_reviewed_list:
  - internal/server/connecterror.go
  - internal/server/connecterror_test.go
  - internal/server/idempotency.go
  - internal/server/idempotency_test.go
  - internal/server/tools.go
  - internal/server/tools_test.go
  - internal/store/store.go
  - internal/store/store_test.go
findings:
  critical: 0
  warning: 0
  info: 0
  total: 0
status: clean
---

# Phase 24: Code Review Report

**Reviewed:** 2026-07-18T21:04:44Z
**Depth:** deep
**Files Reviewed:** 8
**Status:** clean

## Summary

Iteration 3 (final) re-review of the idempotent-capture phase, independently re-verified against the current state of all 8 files — no prior-pass findings were assumed correct without re-checking.

Deep-focus verification performed:

- **Point-ID determinism & injectivity** (`idempotencyPointID`, `internal/server/idempotency.go:39-43`): the `(owner, scope, key)` triple is combined via a per-component length-prefixed encoding (`%d:%s:%d:%s:%d:%s`), which is injective against boundary-shift collisions (e.g. `owner="a",scope="bc"` vs `owner="ab",scope="c"`). Confirmed by `TestIdempotencyPointIDBoundaryShiftInjective`, `TestIdempotencyPointIDOwnerScoped`, `TestIdempotencyPointIDKeySensitive`, `TestIdempotencyPointIDDeterministic`, `TestIdempotencyPointIDValidUUID` — all pass.
- **Fingerprint injectivity** (`contentFingerprint`, `idempotency.go:56-80`): tags are individually length-prefixed (`%d:%s:` per tag) before being folded into the outer per-field length-prefixed encoding, closing the separator-collision gap a raw `strings.Join` would have (verified live: `tags=["a\x1fb"]` vs `tags=["a","b"]` produce different fingerprints). `TestContentFingerprintTagsBoundaryShiftInjective` and `TestContentFingerprintFieldSensitivity` (per-field sensitivity, all 9 identity fields) pass. `idempotency_key` itself is correctly excluded from the fingerprint input (confirmed by inspection — `contentFingerprint`'s signature never reads `a.IdempotencyKey`).
- **Check-before-embed ordering on every path**: `storeMemory` (`tools.go:712-732`) and `scheduleMemory` (`tools.go:787-827`) both call `checkIdempotentReplay` before `d.em.Embed(...)` on every branch, including the mismatch-error branch (returns before Embed) and the schedule path, where `checkIdempotentReplay` deliberately runs before `parseWindow` so a delayed retry of an already-successful call isn't wrongly rejected once its `not_after` lapses (`TestScheduleMemoryIdempotentRetryAfterWindowLapses` — pass).
- **Conflict-vs-upsert (reject on mismatch, never overwrite)**: `checkIdempotentReplay` returns `store.ErrIdempotencyConflict` on fingerprint mismatch before any Upsert; `TestStoreMemoryIdempotentReplayRejectsMismatch` confirms the original record's content is untouched after a rejected mismatched replay.
- **Owner-scoping of the key space**: owner is baked into the point-ID hash input (not a post-hoc filter), so two owners using the identical key+content structurally cannot collide (`TestStoreMemoryIdempotentKeyScopedPerOwner` — pass, verified distinct ids, correct owner stamps, no cross-owner leak in `List`).
- **Concurrency safety of the gated read-back** (`persistAndEnqueue`, `tools.go:744-775`): the post-Upsert re-`Get` is correctly gated on `m.IdempotencyFingerprint != ""`, so keyless writes (the overwhelming common case) skip the extra round trip, and keyed writes correctly resolve the actually-persisted `short_id` when a concurrent racer's Upsert won the last-write-wins race. Ran `TestStoreMemoryIdempotentConcurrentIdenticalOnePoint` three times under `go test -race`: clean, no data race, single point persisted, ids consistent across 20 concurrent identical-key racers each time. A failed re-Get is non-fatal (falls back to the locally-minted `short_id` rather than failing an otherwise-successful write) — correct, and doesn't block enqueue (`m.ID` is used for `tryEnqueue`, unaffected by the short_id fallback).
- **`maxIdempotencyKeyBytes` cap**: enforced in `checkIdempotentReplay` immediately after the `IdempotencyKey == ""` no-op check and before the point-ID Get, the embed call, or any store round trip — confirmed by `TestStoreMemoryIdempotencyKeyTooLarge` (rejects with `store.ErrInvalidArgument`, zero store/embed interaction since `d := &deps{}` has no store/embedder wired).
- **Error mapping**: `connectError` maps `store.ErrIdempotencyConflict` → `CodeAlreadyExists` via `errors.Is` (never string matching); `connecterror_test.go`'s acceptance table exercises this case explicitly and passes. Cross-checked against the proto/generated Connect types (`gen/go`, `proto/engram/v1/*.proto`) — `idempotency_key` is correctly absent from `StoreMemoryRequest`/`ScheduleMemoryRequest`, confirming the code comment's claim that this error path is "structurally unreachable" from the Connect write lane this phase is accurate, not stale.
- **Cross-tool namespace sharing and anonymous single-bucket collision** — confirmed present and documented as locked design decisions (D-07/D-08/IN-02), pinned by `TestCheckIdempotentReplayCrossToolNamespaceShared`; not re-reported as defects per review instructions.
- **Payload round-trip**: `IdempotencyFingerprint` correctly carries the `json:"-"` tag (never crosses the JSON wire) and round-trips exclusively through the manual `payload()`/`fromPayload()` codec under `idempotencyFingerprintKey`; `TestPayloadRoundTripsIdempotencyFingerprint` covers both the present and legacy-missing-key cases.

Verification commands run (all clean):
- `go build ./...`, `go vet ./internal/server/... ./internal/store/...`
- `go test ./internal/server/... ./internal/store/...` — full package suites pass
- `go test ./internal/server/... -run 'Idempot|TestConnectError' -v` — all idempotency-specific tests pass
- `go test ./internal/server/... -run TestStoreMemoryIdempotentConcurrentIdenticalOnePoint -race -count=3 -v` — race-clean across 3 runs
- `gofmt -l` on all 8 files — no output (clean)
- `golangci-lint run ./internal/server/... ./internal/store/...` — 0 issues
- `rg` scan for TODO/FIXME/XXX/HACK/debug artifacts across all 8 files — none found

No Critical, Warning, or Info findings remain. The prior two fixer passes' changes (gated short_id read-back, per-tag length-prefixed fingerprint encoding, check-before-parseWindow ordering, the 512-byte key cap, and the `connecterror_test.go` acceptance-table case) are all correctly implemented and test-verified, not merely present. This phase is clean and ready to ship.

---

_Reviewed: 2026-07-18T21:04:44Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
