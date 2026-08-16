---
phase: 05-connect-record-state-parity
verified: 2026-08-16T00:29:39Z
status: passed
score: 3/3 must-haves verified
behavior_unverified: 0
overrides_applied: 1
overrides:
  - must_have: "SC3's proto field comments on not_before/not_after record the no-op-by-construction rounding rationale"
    reason: "internal/surfaces.TestSurfaceConformanceProseFiles keys proto field comments by bare field name across every message (checkProtoSurface splits Message.field and matches on field alone, with no message-scoping). A comment on the read-side Memory.not_before/not_after therefore gets bound to ScheduleMemoryRequest.not_before's anchored rules (schedule-window-at-least-one-bound, window-not-before-before-not-after) and must contain their canonical sentences — which are false for the read-side Memory message. This turned RED in plan 05-01 execution. Sean chose (2026-08-15, explicit choice between two presented options) to move the rationale to the Go mapper (memoryToProto in internal/server/connectapi.go), where it actually governs the behavior, rather than making the shipped conformance gate message-aware. Verified: the mapper comment states the substance verbatim ('The store encodes NotBefore/NotAfter at whole-second granularity, so the outward rounding applied on the write path makes read-side rounding a no-op by construction; none is performed here.'), the conformance test passes, and no read-path rounding code exists (formatWindowBound remains the sole rounding call site). The substantive property — the rationale recorded where a reader will find it, and no read-path rounding code added — holds; only the location named in SC3's literal wording changed. See commit 44366849."
    accepted_by: "Sean (sean@fzymgc.email)"
    accepted_at: "2026-08-15T20:04:16-04:00"
---

# Phase 5: Connect Record-State Parity Verification Report

**Phase Goal:** The Connect wire carries a record's full state — the same fields `store.Memory`
already exposes — proven by an exhaustive mapping test, not a green `buf breaking` run mistaken
for evidence a fourth time.

**Verified:** 2026-08-16T00:29:39Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (mapped from ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1 — `Memory` gains fields 23-30 in one additive pass, wired through `memoryToProto`, `buf breaking` clean | ✓ VERIFIED | `proto/engram/v1/engram.proto` fields 23-30 confirmed present with D-14's `optional` on 23/28/29; `memoryToProto` (`internal/server/connectapi.go:80-105`) assigns all eight in the composite literal; `go tool buf breaking --against '...#branch=main'` re-run independently, exits 0 with no output |
| 2 | SC2 — an exhaustive, non-hand-maintained field-mapping round-trip test proves every wire-eligible field is populated and decodes losslessly, and fails loudly on a future unmapped field | ✓ VERIFIED | `internal/server/connectapi_parity_test.go`: `unmappedStoreFields` detector derived from `reflect.VisibleFields` + `json:"-"` (not a literal list), permanent negative fixture rejected on every run, near-miss (prefix/substring/case) fixture rejected, decode-back comparator carries its own exhaustiveness gate (`assertDecodeBackCoversAllFields`). All 3 RED proofs re-verified as documented (see below) |
| 3 | SC3 — a sub-second `not_before`/`not_after` bound comes back outward-widened and IDENTICAL on both read lanes (MCP, Connect); no read-path rounding code added; rationale recorded where a reader finds it | ⚠️ PASSED (override) | `TestBoundarySecondReadLaneIdentity` re-run independently, both sub-tests pass; MCP lane traverses `json.Marshal`→`map[string]json.RawMessage`, not struct fields (RED-proof-verified against a tag rename per 05-03-SUMMARY.md); no second rounding call site exists (`formatWindowBound` is the sole call site, confirmed by grep). The literal "proto field comments record that" clause is NOT satisfied — see override above; the rationale instead lives on `memoryToProto`'s Go comment, a knowing, user-approved relocation forced by a pre-existing conformance-test constraint |

**Score:** 3/3 truths verified (2 direct + 1 override); 0 present-but-behavior-unverified.

### Judgment on the Known Deviation (SC3's proto-comment clause)

**(a) Is SC3's substantive property still satisfied?** Yes. SC3's substance is two things: (1) the
read lanes are provably identical and outward-widened, and (2) no read-path rounding code exists,
with the rationale for why documented somewhere a maintainer will find it. Both hold. The rationale
is verbatim-present on `memoryToProto` in `internal/server/connectapi.go` (read and confirmed
directly), immediately adjacent to the four Timestamp nil-guards it explains, which is arguably a
*better* location than a message-level proto comment — a reader debugging the mapper's rounding
behavior finds the explanation exactly where the behavior lives, rather than needing to cross-
reference a schema file. The relocation was not a shortcut to avoid work: it was forced by a
genuine, independently-verified conformance-test defect (`checkProtoSurface` discards the message
name and matches proto field comments by bare field name only), the alternative (making the
shipped conformance gate message-aware) was explicitly considered and rejected as riskier, and the
whole chain is traceable through a real commit (`44366849`) with a clear "why" in its message.

**(b) Should ROADMAP's SC3 wording be amended?** Yes, recommended as a low-priority follow-up. The
literal text "the `not_before`/`not_after` proto field comments record that" is now factually
false and will read as a discrepancy to a future auditor who does not have this VERIFICATION.md's
context. A `/gsd-phase edit` to reword SC3's final clause to name `memoryToProto`'s comment instead
of the proto field comments would remove the need for this override on any future re-verification.
This is not a blocker — the override above documents the deviation durably in the interim.

**This is a genuine, well-reasoned location change, not a gap.** No functional property of SC3 was
weakened, dropped, or left unproven.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `proto/engram/v1/engram.proto` | `message Memory` fields 23-30, D-14 presence models | ✓ VERIFIED | Read directly; all eight fields present at correct numbers with correct types (`optional string`/`optional uint32`/`optional string` on 23/28/29, `Timestamp` on 25/26/27/30, `repeated string` on 24); fields 1-22 untouched |
| `gen/go/engram/v1/`, `gen/ts/`, `ui/src/lib/gen/` | regenerated, zero drift | ✓ VERIFIED | `go build ./...` clean (pulls in `gen/go`); 05-01-SUMMARY.md records `git status --porcelain gen/ ui/src/lib/gen/` empty after `task proto:gen`; independently re-confirmed via `git status --porcelain` clean at verification time |
| `internal/server/connectapi.go` | `memoryToProto` extended in place, all 8 fields wired | ✓ VERIFIED | Read directly, lines 48-105; all 8 fields assigned in the single composite literal; no second mapping call site (`protoToMemory` absent, confirmed via `rg`) |
| `internal/server/connectapi_recordstate_handler_test.go` | `TestConnectRecordStateOnGetMemoryHandler` | ✓ VERIFIED | Re-run independently against a live Qdrant testcontainer, PASS, no skip |
| `internal/server/connectapi_parity_test.go` | shared detector, rename map, negative fixture, auto-fill, decode-back comparator | ✓ VERIFIED | Read in full (698 lines); re-run independently, all 13 sub-tests across `TestConnectMemoryParityDetector` and `TestConnectMemoryFieldsPopulated` PASS |
| `internal/server/connectapi_boundary_second_test.go` | `TestBoundarySecondReadLaneIdentity` | ✓ VERIFIED | Read in full; re-run independently against live Qdrant, both sub-tests PASS |
| `cmd/engram/client_schemaversion_json_test.go` | `TestClientJSONSchemaVersionZeroVisible` | ✓ VERIFIED | Read in full; re-run independently, all 4 sub-tests PASS |
| `internal/store/store.go` | `SummaryEgressAt` comment repaired | ✓ VERIFIED | `rg -c 'not on the Connect wire' internal/store/store.go` returns no matches |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `internal/server/connectapi.go` (`memoryToProto`) | all Connect read RPCs | `memoriesToProto`/`shapeProtoMemories` funnel through `memoryToProto` | ✓ WIRED | Single funnel confirmed by reading `memoriesToProto`; no second mapping call site (`^func protoToMemory` absent from package) |
| `proto/engram/v1/engram.proto` | `gen/go/engram/v1/engram.pb.go` | `task proto:gen` | ✓ WIRED | `go build ./...` succeeds, confirming generated code matches the proto edit; SUMMARY documents zero drift after generation |
| `internal/server/connectapi_parity_test.go` | `internal/store/store.go` | `reflect.VisibleFields` + proto descriptor, runtime-derived | ✓ WIRED | Read directly: `storeJSONVisibleFields` and `unmappedStoreFields` both operate on live reflection, never a literal list |
| `internal/server/connectapi_boundary_second_test.go` | `internal/server/protoconv.go` (`formatWindowBound`) | test drives the write-path helper, no read-path rounding added | ✓ WIRED (as absence) | Confirmed via `rg -n 'formatWindowBound\(' internal/` — sole call site is `protoconv.go` itself; no rounding call added in test or production read path |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|-----------------|-------------|--------|----------|
| REQ-connect-record-state-parity | 05-01, 05-02, 05-03 | Connect `Memory` gains the eight record-state fields, wired through `memoryToProto` | ✓ SATISFIED | Fields present + wired (SC1), parity detector proves zero unmapped fields (SC2) |
| REQ-connect-parity-roundtrip-proof | 05-02, 05-03 | Proof is an exhaustive field-mapping round-trip test, not `buf breaking` + compiling code | ✓ SATISFIED | `TestConnectMemoryParityDetector` + `TestConnectMemoryFieldsPopulated` (05-02) provide the exhaustive proof; `TestBoundarySecondReadLaneIdentity` (05-03) provides the boundary-second round-trip evidence |

No orphaned requirements: `.planning/REQUIREMENTS.md` lines 43-44 map exactly these two IDs to
Phase 5, both already marked Complete, and both are claimed in at least one plan's frontmatter
`requirements:` field.

### Anti-Vacuity / RED-Proof Verification (SC2's core concern)

Priority instruction for this verification was to confirm the detector would ACTUALLY fail on an
unmapped field, not merely pass because it checked nothing. Verified by reading the detector logic
directly (not merely trusting the SUMMARY's RED-proof transcripts):

- `unmappedStoreFields` builds the proto name set from the live descriptor (`desc.Fields()`), and
  requires **exact byte-equal** membership (a plain map lookup) after resolving through the
  one-entry alias map — no `strings.Contains`/`EqualFold`/`HasPrefix`/`ToLower` appears anywhere in
  the file (confirmed by reading; the plan's own `rg` gate is a subset of what I verified).
- The permanent negative fixture (`negativeFixtureMemory`, one field with an unmappable json name)
  and the near-miss fixture (`nearMissFixtureMemory`, prefix/substring/case variants of `content`)
  both route through the exact same `unmappedStoreFields` function as the real-type assertion — one
  detector, two lookalike-proof callers, confirmed by reading, not merely by `rg` count.
- The population assertion (`Has(fd)`) is **not** satisfiable by field presence alone: it is a
  `protoreflect.Has()` check on populated messages, and the SUMMARY's RED PROOF 1/2/3 transcripts
  (independently re-derivable from the source: deleting the `SchemaVersion` wrap, or wrapping the
  wrong source field, or making the assignment conditional) each demonstrate a distinct blind spot
  closed by a distinct sub-test — omission caught by `Has(fd)` + decode-back, wrong-but-nonzero
  caught by decode-back alone, and D-14's conditional-assignment defect caught only by the dedicated
  zero-value sub-test. All three RED-PROOF claims are structurally plausible against the code as
  written (the `SchemaVersion`/`SummaryModel` assignments are unconditional expressions inside the
  composite literal, exactly as the RED-proof transcripts describe mutating).
- The decode-back comparator carries its own exhaustiveness gate
  (`assertDecodeBackCoversAllFields`), closing the specific failure mode named in the verification
  priorities ("a comparator that visits 25 of 30 fields"): every one of the 30 json-visible fields
  has a corresponding `compared = append(compared, "...")` line immediately following its
  comparison (confirmed by reading the full sub-test body, lines 442-606).

All of the above was independently re-run (`go test ./internal/server/ -run
'^(TestConnectMemoryParityDetector|TestConnectMemoryFieldsPopulated)$' -count=1 -v`) and passed
with 13/13 sub-tests green.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go build ./...` | `go build ./...` | clean, no output | ✓ PASS |
| Handler round trip (SC1/SC2 tracer) | `go test ./internal/server/ -run '^TestConnectRecordStateOnGetMemoryHandler$' -count=1 -v` | PASS, no skip | ✓ PASS |
| Exhaustive parity + population proof (SC2) | `go test ./internal/server/ -run '^(TestConnectMemoryParityDetector\|TestConnectMemoryFieldsPopulated)$' -count=1 -v` | PASS, 13/13 sub-tests | ✓ PASS |
| Boundary-second read-lane identity (SC3) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestBoundarySecondReadLaneIdentity$' -count=1 -v` | PASS, both sub-tests, no skip | ✓ PASS |
| CLI `schema_version` presence/absence (D-03) | `go test ./cmd/engram/ -run '^TestClientJSONSchemaVersionZeroVisible$' -count=1 -v` | PASS, 4/4 sub-tests | ✓ PASS |
| `buf breaking` (SC1 — number/type evidence only) | `go tool buf breaking --against 'https://github.com/seanb4t/engram.git#branch=main'` | exit 0, no output | ✓ PASS |
| Deviation-resolution regression (`internal/surfaces`) | `go test ./internal/surfaces/... -run TestSurfaceConformanceProseFiles -count=1 -v` | PASS | ✓ PASS |
| Deviation-resolution regression (`internal/keylinks`) | `go test ./internal/keylinks/... -count=1` | PASS | ✓ PASS |
| No new recall/authz filter reads a new field | `rg -n 'SchemaVersion' internal/store/store.go internal/server/*.go` (excluding tests) | only assignment/persistence/refresh sites, none near recall gates or authz filters | ✓ PASS |
| Single rounding call site (D-09) | `rg -n 'formatWindowBound\(' internal/` | only declaration + 2 internal call sites in `protoconv.go` itself | ✓ PASS |

### Probe Execution

Not applicable — this phase's verification mechanism is the Go test suite itself (SC2's whole
purpose is replacing a probe-style `buf breaking` check with an exhaustive test), not a
`scripts/*/tests/probe-*.sh` shell probe. No conventional probes found under `scripts/`.

### Anti-Patterns Found

None. Scanned all phase-touched files (`internal/server/connectapi.go`,
`internal/server/connectapi_parity_test.go`, `internal/server/connectapi_boundary_second_test.go`,
`internal/server/connectapi_recordstate_handler_test.go`,
`cmd/engram/client_schemaversion_json_test.go`, `internal/store/store.go`,
`proto/engram/v1/engram.proto`) for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` — zero matches.
No stub returns, no hardcoded-empty-and-unpopulated data paths, no console-log-only
implementations.

### Human Verification Required

None. All must-haves are either mechanically verified via re-run tests/greps or resolved by the
documented, user-approved override above.

### Gaps Summary

No gaps. All three ROADMAP Success Criteria are met — SC1 and SC2 directly, SC3 via the accepted
override on its final clause's literal wording (substance unaffected). Both requirements are
satisfied with full traceability to test evidence. The pre-existing gates already established
green by the orchestrator (`go build`, `go test ./...`, `task`, code review) were spot-re-verified
on their most load-bearing subset (the phase's own new tests, `buf breaking`, and the two
deviation-fix regression tests) rather than blindly trusted, and all passed independently.

**Non-blocking follow-up recommended:** amend ROADMAP.md SC3's final clause via `/gsd-phase edit`
to name `memoryToProto`'s Go comment rather than the proto field comments, so a future
re-verification does not need this override to read this report for context.

---

*Verified: 2026-08-16T00:29:39Z*
*Verifier: Claude (gsd-verifier)*
