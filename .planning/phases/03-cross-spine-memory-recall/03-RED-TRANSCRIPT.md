# v0.12.x Phase 3, Plan 01, Task 2 — RED-by-mutation transcript

**Recorded:** 2026-08-01
**Test:** `TestCrossSpineAuthzIsolation` (`internal/store/store_test.go`)
**Purpose:** D-15 requires that a green on the isolation test not be accepted as evidence until it
has been observed to fail for the right reason. This document is that observation: real `go test -v`
output, pasted verbatim, for PASS → FAIL → PASS.

## Mutation chosen

Per `03-AUTHZ-GATE.md`:115-119 and RESEARCH.md Assumption A2 (first variant, which the gate's own
wording favors): drop the authz clause from the `Must` slice, leaving it empty. An empty `Must`
matches every point in the collection, so owner B's private record must appear in the scroll result
and the exclusion assertion must fire.

This targets the AUTHZ CLAUSE directly, per D-15 — not a feature-flag toggle (there is no
`cross_spine` flag to toggle yet; production code is untouched by this plan).

### Exact diff applied (and reverted)

```diff
--- a/internal/store/store_test.go
+++ b/internal/store/store_test.go
@@ -4453,10 +4453,11 @@ func TestCrossSpineAuthzIsolation(t *testing.T) {
 	mk(bSharedID, "sub-xspine-B", "shared")  // B shared

 	// The cross-spine-shaped filter: authz clause only, no scope element.
-	f := &qdrant.Filter{Must: []*qdrant.Condition{
-		s.ownerOrSharedCondition(Authenticated("sub-xspine-A")),
-	}}
+	// MUTATION (D-15 RED observation, 03-01-PLAN.md Task 2): the authz clause
+	// is temporarily dropped, leaving an empty Must — this must fail the
+	// leaked-owner-B assertion. Restored immediately after capturing output.
+	f := &qdrant.Filter{Must: []*qdrant.Condition{}}

 	const limit = 10000
 	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
```

## Run 1 — GREEN, pre-mutation

Command: `go test ./internal/store/... -run TestCrossSpineAuthzIsolation -v -count=1`

```
=== RUN   TestCrossSpineAuthzIsolation
2026/08/01 10:42:41 WARN Unable to compare versions err="unable to parse version, expected format: x.y[.z], found: "
2026/08/01 10:42:41 WARN Client version is not compatible with server version. Major versions should match and minor version difference must not exceed 1. Set SkipCompatibilityCheck=true to skip version check. clientVersion=Unknown serverVersion=1.18.2
--- PASS: TestCrossSpineAuthzIsolation (0.14s)
PASS
ok  	github.com/seanb4t/engram/internal/store	1.240s
```

## Run 2 — FAIL, mutation applied

Command: `go test ./internal/store/... -run TestCrossSpineAuthzIsolation -v -count=1`

```
=== RUN   TestCrossSpineAuthzIsolation
2026/08/01 10:42:53 WARN Unable to compare versions err="unable to parse version, expected format: x.y[.z], found: "
2026/08/01 10:42:53 WARN Client version is not compatible with server version. Major versions should match and minor version difference must not exceed 1. Set SkipCompatibilityCheck=true to skip version check. clientVersion=Unknown serverVersion=1.18.2
    store_test.go:4488: leaked owner B's private record across the cross-spine-shaped filter: c5c50000-0000-0000-0000-000000000002
--- FAIL: TestCrossSpineAuthzIsolation (0.14s)
FAIL
FAIL	github.com/seanb4t/engram/internal/store	1.191s
FAIL
```

**The failure is the leaked-owner-B assertion, exactly as required** — the log line names
`c5c50000-0000-0000-0000-000000000002` (owner B's private record) as leaked, and the failure is
`--- FAIL: TestCrossSpineAuthzIsolation`, not a compile error, not the truncation guard, and not a
missing-record assertion.

## Run 3 — GREEN again, after restore

The file was restored with `git checkout -- internal/store/store_test.go` (reverting to the Task 1
commit's content) before this run.

Command: `go test ./internal/store/... -run TestCrossSpineAuthzIsolation -v -count=1`

```
=== RUN   TestCrossSpineAuthzIsolation
2026/08/01 10:43:01 WARN Unable to compare versions err="unable to parse version, expected format: x.y[.z], found: "
2026/08/01 10:43:01 WARN Client version is not compatible with server version. Major versions should match and minor version difference must not exceed 1. Set SkipCompatibilityCheck=true to skip version check. clientVersion=Unknown serverVersion=1.18.2
--- PASS: TestCrossSpineAuthzIsolation (0.13s)
PASS
ok  	github.com/seanb4t/engram/internal/store	1.183s
```

## Restore verification

Command: `git diff --exit-code -- internal/store/store_test.go`

Exit code 0 — no output, no diff. `internal/store/store_test.go` is byte-identical to its Task 1
commit (`737178e2`) state after the RED-observation sequence.

## Conclusion

`TestCrossSpineAuthzIsolation` is demonstrated to be sensitive to the authz clause being dropped: it
passes when `ownerOrSharedCondition` is present in the filter's `Must` slice, and it fails —
specifically on the leaked-owner-B assertion — when that clause is removed. The pre-mutation green
recorded in Run 1 (and in the Task 1 commit) is therefore non-vacuous evidence for ROADMAP
criterion 2, satisfying D-15.
