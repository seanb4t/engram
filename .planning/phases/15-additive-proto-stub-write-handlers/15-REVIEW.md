---
phase: 15-additive-proto-stub-write-handlers
reviewed: 2026-07-11T18:50:00Z
depth: standard
files_reviewed: 11
files_reviewed_list:
  - proto/engram/v1/engram.proto
  - buf.yaml
  - buf.gen.yaml
  - Taskfile.yaml
  - .github/workflows/ci.yaml
  - internal/server/connectvalidate.go
  - internal/server/connectvalidate_test.go
  - internal/server/connectapi.go
  - internal/server/connectdescriptor_test.go
  - internal/server/connectapi_negative_test.go
  - go.mod
findings:
  critical: 0
  warning: 3
  info: 2
  total: 5
status: issues_found
---

# Phase 15: Code Review Report

**Reviewed:** 2026-07-11T18:50:00Z
**Depth:** standard
**Files Reviewed:** 11
**Status:** issues_found

## Summary

Reviewed the additive proto extension (six write RPCs + `Visibility` enum + buf.validate
annotations), the hand-rolled `protovalidate` interceptor, the idempotency-ban build gate
(Taskfile + CI), and the descriptor/negative-matrix regression tests. Confirmed against the
phase's own planning artifacts (`15-01-PLAN.md`, `15-RESEARCH.md`) that the six write RPCs are
deliberately `StoreMemory / StoreDiscovery / UpdateMemory / DeleteMemory / SetVisibility /
ScheduleMemory` — `delete_all` is out of scope for this phase, so its absence from the proto is
not a defect.

Ran `go build ./...` and `go test ./internal/server/... -run
'TestConnectValidateInterceptor|TestEngramServiceDescriptor|TestWriteRPCNegativeMatrix'` — all
green. The interceptor ordering (auth 401 before validate 400, per D-10), the `UpdateMemory`
FieldMask allowlist CEL (`content`/`shared`/`tags`/`summary`, rejecting absent/empty/unknown
paths), the `category` `string.in` enum, and the `SetVisibility` enum zero-value rejection all
behave correctly and are exercised by real end-to-end tests (httptest server + the actual
interceptor chain, not mocks). No stub incorrectly returns anything other than
`CodeUnimplemented`, and the CI/Taskfile idempotency-ban check plus the descriptor test's
`IDEMPOTENCY_UNKNOWN`-on-every-method assertion together correctly block the GET-reachable-CSRF
pitfall this phase is designed to prevent.

No blocking defects found. The issues below are test-coverage gaps in the negative matrix and
one build-gate duplication/robustness concern — all worth fixing before this contract is
considered fully proven, but none represent incorrect runtime behavior in the shipped code.

## Warnings

### WR-01: ScheduleMemory's window-ordering CEL rule is never independently exercised

**File:** `internal/server/connectapi_negative_test.go:148-150`, `proto/engram/v1/engram.proto:199-204`
**Issue:** The message-level CEL on `ScheduleMemoryRequest` (`schedule_memory.window`) encodes
two distinct rules: (1) at least one of `not_before`/`not_after` must be set, and (2) when both
are set, `not_after` must be strictly after `not_before`. `TestWriteRPCNegativeMatrix`'s
`invalidCall` for ScheduleMemory is an entirely empty request — it fails validation for several
reasons at once (empty `content`/`scope`/`category` *and* no window bound), so the test proves
nothing specific about rule (2), and only incidentally exercises rule (1) alongside unrelated
field violations. If the CEL expression were subtly wrong (e.g. `>=` vs `>`, or the
`!has(...)` short-circuit ordering were flipped), no test in this file would catch it — a
regression here is a permanent wire-contract defect (silently accepting `not_after <=
not_before`, or rejecting valid reversed-only-one-bound requests).
**Fix:** Add a dedicated case with an otherwise-valid `content`/`scope`/`category` and only
`not_before`/`not_after` misuse, e.g.:
```go
t.Run("ScheduleMemory_window_cells", func(t *testing.T) {
	base := time.Now()
	cases := []struct {
		name string
		nb, na *timestamppb.Timestamp
	}{
		{"neither_bound_set", nil, nil},
		{"not_after_before_not_before", timestamppb.New(base), timestamppb.New(base.Add(-time.Hour))},
		{"not_after_equal_not_before", timestamppb.New(base), timestamppb.New(base)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &engramv1.ScheduleMemoryRequest{Content: "valid", Scope: "test:scope", Category: "decision", NotBefore: tc.nb, NotAfter: tc.na}
			if err := callWrite(ctx, client.ScheduleMemory, req, "actor-A"); connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Errorf("got code %v (%v), want InvalidArgument", connect.CodeOf(err), err)
			}
		})
	}
})
```

### WR-02: StoreDiscovery's nested `Citation` buf.validate constraints are never proven through the Connect wire path

**File:** `internal/server/connectapi_negative_test.go:89-101`, `proto/engram/v1/engram.proto:122-128`
**Issue:** `Citation.kind` (`string.in` allowlist `["file","commit","url","repo"]`), `Citation.ref`
(`min_len=1`), and `Citation.excerpt` (`max_bytes=16384`) are all buf.validate constraints on a
nested message that protovalidate recursively validates. The only negative case for
`StoreDiscovery` is a fully empty `StoreDiscoveryRequest{}`, which never even populates a
`Citation`, so the recursive-validation path for nested messages is never actually exercised at
the Connect layer — only inferred. The equivalent MCP-side validation (`storeDiscoveryArgs`,
`citationArg`) *is* thoroughly tested in `tools_test.go:738-750` (bad/empty kind, empty ref, too
many citations, excerpt-too-large), which makes the absence of the analogous cases for the new
wire contract more conspicuous: it's the one part of this phase's negative matrix that doesn't
mirror the coverage the phase claims to be porting.
**Fix:** Add cases with a non-empty top-level request but a violating `Citation`, e.g. `Kind:
"video"` (not in allowlist) or `Ref: ""` with everything else valid, asserting
`CodeInvalidArgument`.

### WR-03: Idempotency-ban grep gate is duplicated verbatim in `Taskfile.yaml` and `ci.yaml`, and can be bypassed by a numeric enum literal

**File:** `Taskfile.yaml:140-144`, `.github/workflows/ci.yaml:124-129`
**Issue:** Both files independently hardcode the identical regex
`idempotency_level[[:space:]]*=[[:space:]]*NO_SIDE_EFFECTS` (with only the echo/annotation text
differing). This is a copy-paste duplication of a security-relevant gate: a future edit to
tighten or fix the pattern in one file (e.g., adding an idempotency check for `IDEMPOTENT` too,
or fixing a regex bug) can easily be applied to only one of the two copies, silently weakening
CI enforcement while local `task proto:lint` still looks "safe" (or vice versa). Separately, the
regex only matches the *symbolic* enum literal `NO_SIDE_EFFECTS`; proto text format also accepts
the numeric literal (`option idempotency_level = 1;`), which would slip past both grep gates
undetected. In this specific case the gap is closed in practice by
`TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs`'s semantic check
(`opts.GetIdempotencyLevel() != IDEMPOTENCY_UNKNOWN`), which inspects the compiled descriptor
and is immune to symbolic-vs-numeric spelling — but that safety net lives in a different file
from the one being reviewed for "the idempotency-ban gate's robustness," and isn't documented as
the reason the numeric-literal gap is acceptable.
**Fix:** Extract the grep into a single shared script (e.g. `scripts/check-idempotency-ban.sh`)
invoked identically by both `Taskfile.yaml` and `ci.yaml`, and add a comment at the grep site
noting that the descriptor test is the semantic backstop for non-symbolic option spellings.

## Info

### IN-01: Redundant `required` + CEL `has()` check on `UpdateMemoryRequest.update_mask` produces duplicate violations for the same input

**File:** `proto/engram/v1/engram.proto:161-171`
**Issue:** `update_mask` carries both `(buf.validate.field).required = true` (line 171) and the
message-level CEL's `has(this.update_mask)` clause (line 164). For an absent mask, protovalidate
(which does not fail-fast by default) reports two violations for one root cause, which is
harmless but slightly noisy for API consumers parsing `ValidationError.Violations()`.
**Fix:** Either drop the field-level `required` (the CEL already covers absence via `has()`) or
drop the redundant `has()` clause from the CEL and rely solely on `required`; not urgent.

### IN-02: `Citation` message fields are undocumented relative to the rest of the phase's meticulous field comments

**File:** `proto/engram/v1/engram.proto:121-128`
**Issue:** Every other new message in this phase (`UpdateMemoryRequest`, `ScheduleMemoryRequest`,
`StoreDiscoveryRequest`) carries detailed per-field or message-level design-rationale comments.
`Citation` has only a one-line message comment and no per-field documentation — in particular
`pin` ("aging pin" per `CLAUDE.md`'s discovery contract) has no explanation of its format or
aging semantics on the wire type itself.
**Fix:** Add a short comment on `pin` describing its expected format/semantics, consistent with
the rest of the file's documentation density.

---

_Reviewed: 2026-07-11T18:50:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
