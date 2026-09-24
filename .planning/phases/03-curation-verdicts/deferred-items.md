# Deferred Items — Phase 03 (Curation Verdicts)

## Pre-existing, out-of-scope: `go vet ./...` fails on `cmd/engram/operator_view_test.go`

- **File:** `cmd/engram/operator_view_test.go:441` (`TestOperatorViewDuplicateKeyAdjacency`)
- **Finding:** `struct field B repeats json tag "dup" also at operator_view_test.go:440`
- **Status:** Pre-existing, not touched by plan 03-01. The offending line already
  carries `//nolint:govet` with a comment explaining the duplicate tag is a
  deliberate adjacency-edge probe. `golangci-lint run` (which honors `nolint`
  directives) reports 0 issues across `./internal/verdict/... ./internal/store/...
  ./internal/server/... ./cmd/engram/...`; only the raw `go vet` invocation (which
  does not understand `nolint`) flags it. Reproduces identically on `go vet ./...`
  from a clean checkout, independent of this plan's changes.
- **Verification:** `go vet ./internal/verdict/ ./internal/store/ ./internal/server/`
  (the packages plan 03-01 actually modifies, excluding cmd/engram) is clean.
- **Action:** Not fixed — out of scope per the executor's scope-boundary rule
  (file not in plan 03-01's `files_modified`). Left for a future phase/plan that
  actually owns `operator_view_test.go`, or for the project maintainer to decide
  whether the project's real gate (`golangci-lint`, which already passes) should
  supersede raw `go vet` in any documentation that implies otherwise.
