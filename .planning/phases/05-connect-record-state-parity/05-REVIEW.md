---
phase: 05-connect-record-state-parity
reviewed: 2026-08-16T14:14:47Z
depth: standard
files_reviewed: 16
files_reviewed_list:
  - .github/workflows/ci.yaml
  - Taskfile.yaml
  - cmd/engram/client_schemaversion_json_test.go
  - gen/go/engram/v1/engram.pb.go
  - gen/ts/engram/v1/engram_pb.ts
  - go.mod
  - go.sum
  - internal/e2e/console_browser_test.go
  - internal/e2e/harness_test.go
  - internal/server/connectapi.go
  - internal/server/connectapi_boundary_second_test.go
  - internal/server/connectapi_parity_test.go
  - internal/server/connectapi_recordstate_handler_test.go
  - internal/server/connectdescriptor_test.go
  - internal/store/store.go
  - proto/engram/v1/engram.proto
  - ui/src/lib/gen/engram/v1/engram_pb.ts
findings:
  critical: 0
  warning: 1
  info: 1
  total: 2
status: issues_found
---

# Phase 5: Code Review Report

**Reviewed:** 2026-08-16T14:14:47Z
**Depth:** standard
**Files Reviewed:** 16
**Status:** issues_found

## Summary

This phase adds eight record-state fields (23-30) to the Connect `Memory` wire message
and builds several exhaustiveness proofs (field-name parity detector, reflection
auto-fill population/decode-back comparator, descriptor field-shape pins, a real
Qdrant-backed handler round trip, and a real headless-Chrome console render test).
The added/changed production code is small (`proto/engram/v1/engram.proto`,
`internal/server/connectapi.go`'s `memoryToProto`, plus the new
`internal/e2e/console_browser_test.go` and the `ENGRAM_REQUIRE_BROWSER` CI/Taskfile
wiring) and is unusually well defended: every anti-vacuity pattern called out in the
review brief (working-tree diff gates, comment-filtered negative greps,
empty-observation-set assertions, tautological parity detectors, skip-instead-of-fail
gates) was checked against this phase's own additions and each one held up —
`unmappedStoreFields` is proven capable of rejecting via a permanent negative fixture
and a near-miss fixture, `browserObserver.assertClean`/`sweepConsoleAssets` both carry
explicit non-emptiness assertions on their observed sets, and `ENGRAM_REQUIRE_BROWSER`
is byte-for-byte shaped like the existing `ENGRAM_REQUIRE_QDRANT`/`skipOrFailNoQdrant`
pair and is genuinely wired into CI (`ci.yaml`) and `Taskfile.yaml`'s `test:strict`.

The `EmbedderIdentity`/`IdempotencyFingerprint` `json:"-"` audit stamps stay correctly
off the Connect wire (confirmed against `TestConnectMemoryParityDetector`), and the
generated Go/TS artifacts (`gen/go`, `gen/ts`, `ui/src/lib/gen`) are consistent with
the proto and with each other. `go.mod`/`go.sum`'s new `chromedp`/`cdproto`/`sysutil`
dependency triplet is clean (no duplicate entries, pseudo-versioned `cdproto` is
expected for that module, which has no proper release tags).

One real, if low-severity, type-safety gap was found in the new `memoryToProto` code
(WARNING below), plus one documentation/consistency note (INFO).

## Warnings

### WR-01: Unchecked `int` → `uint32` narrowing conversion for `schema_version`

**File:** `internal/server/connectapi.go:102`
**Issue:** `memoryToProto` populates the new `schema_version` field with
`proto.Uint32(uint32(m.SchemaVersion))`. `store.Memory.SchemaVersion` is a
`migrate.Version`, which is defined as a plain signed `int`
(`internal/migrate/migrate.go:20`) and is read back off the Qdrant payload via
`migrate.Version(v.GetIntegerValue())` (`internal/store/store.go:742`), with no
range validation anywhere on that read path. A negative or `> math.MaxUint32`
value — from a corrupted/tampered Qdrant payload, a future migration-numbering bug,
or a manually edited point — silently wraps around instead of failing loudly, and the
wrapped value is then asserted onto the wire as if it were a normal schema version.
Every other numeric-adjacent field this phase added guards against exactly this kind
of silent misrepresentation (e.g. the `nil`-vs-zero-value guards on the four new
`Timestamp` fields), so this conversion is the one place in the same function that
has no equivalent guard.
**Fix:**
```go
// memoryToProto (or a small helper) should refuse to silently wrap a
// SchemaVersion outside uint32's range, e.g.:
sv := m.SchemaVersion
if sv < 0 || sv > migrate.Version(math.MaxUint32) {
    // surface as a server-side invariant violation (log + clamp, or panic in a
    // debug build) rather than letting `uint32(sv)` wrap silently.
}
SchemaVersion: proto.Uint32(uint32(sv)),
```

## Info

### IN-01: Connect summary-shaped recall exposes the eight new record-state fields; MCP's `recallView` does not

**File:** `internal/server/connectapi.go:151-165` (`shapeProtoMemories`) vs.
`internal/server/summary.go:40-60` (`recallView`)
**Issue:** `shapeProtoMemories` (the Connect `full=false` compact shaper) only clears
`Content`, `Summary`, `Citations`, and `Kind` — it leaves `SupersededBy`, `Supersedes`,
`NotBefore`, `NotAfter`, `ArchivedAt`, `SchemaVersion`, `SummaryModel`, and
`SummaryEgressAt` populated. The MCP lane's hand-written `recallView` (the equivalent
compact shape) omits all eight of these fields entirely. This is confirmed as a
deliberate, documented choice for this phase (`05-01-PLAN.md`: "the eight fields must
appear on all [Connect read RPCs] ... via memoriesToProto/shapeProtoMemories"; and
`05-CONTEXT.md`'s Deferred Ideas explicitly defers "`schema_version` on the compact
`recallView`" to a later phase per Phase 2's D-11), so this is not a defect to fix in
this phase — it is flagged here only so the cross-lane (MCP vs. Connect) compact-view
asymmetry stays visible to the next reader of this file, since nothing in
`connectapi.go` itself notes that the omission is intentional and MCP-side, only that
`Content`/`Citations`/`Kind` are cleared.
**Fix:** No action required for this phase. Consider a one-line comment on
`shapeProtoMemories` cross-referencing the deferred `recallView` parity item so a
future reader does not mistake this for an oversight.

---

_Reviewed: 2026-08-16T14:14:47Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
