# Testing Patterns

**Analysis Date:** 2026-07-08

## Test Framework

**Runner:**

- Go standard library `testing` — no testify, no assertion DSL
- Python hooks tested with `pytest` (via `uv`) under `skill/engram/hooks/tests`

**Assertion Library:**

- None. Plain `if got != want { t.Errorf(...) }` / `t.Fatalf(...)` checks throughout (`internal/server/rules_test.go`)

**Run Commands:**

```bash
task test              # go + python unit tests (task test:go + test:python)
task test:go           # go test ./...
task test:short        # go test -short ./...  (skips heavy integration)
task test:coverage     # go test -coverprofile=cover.out ./...
task bench             # go test -run '^$' -bench=. -benchmem ./...
task test:python       # uv run --with pytest pytest skill/engram/hooks/tests -q
```

`task` (default) runs `lint` then `test`. CI runs each as a separate job in `.github/workflows/ci.yaml` (`test`, `golangci-lint`, `license headers`, `helm chart`, `actionlint`, `python`, `buf`, ui drift/tests, commit-lint).

## Test File Organization

**Location:** Co-located with source (`_test.go` in the same package/directory).

**Naming:** `<source>_test.go`; test funcs `TestXxx`, benchmarks `BenchmarkXxx` (`internal/store/bench_test.go`), package `TestMain` for integration setup.

**Structure:**

```text
internal/server/rules.go          # implementation
internal/server/rules_test.go     # tests for that file
internal/store/store_test.go      # TestMain provisions Qdrant
internal/store/bench_test.go      # microbenchmarks
```

No `testdata/` directories or golden files in the Go tree (license config references `**/*.golden` but none are checked in).

## Test Structure

**Suite Organization (table-driven):**

```go
func TestValidateStoreRule(t *testing.T) {
	// happy-path assertions first
	if err := validateStoreRule(good); err != nil {
		t.Errorf("valid args rejected: %v", err)
	}

	// negative cases as an anonymous-struct slice
	bad := []struct {
		name string
		a    storeRuleArgs
	}{
		{"empty content", storeRuleArgs{Content: "", Scope: "rule:repo:x", Summary: "s"}},
		{"non-rule scope", storeRuleArgs{Content: "x", Scope: "repo:x", Summary: "s"}},
		// ...
	}
	for _, tc := range bad {
		if err := validateStoreRule(tc.a); err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}
```

**Patterns:**

- Table-driven with a `name` field on each case; loop variable `tc`
- `t.Fatalf` for setup/precondition failures, `t.Errorf` for assertion failures (test continues)
- `t.Cleanup(...)` for teardown (preferred over defer) — e.g. `t.Cleanup(func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) })`
- `context.Background()` for handler calls; `authedContext(t, sub)` to inject a verified OIDC identity

## Mocking

**Framework:** None — no gomock/mockgen. Real dependencies via testcontainers; hand-written stub functions for narrow seams.

**Patterns:**

```go
// Stub verifier injected through the real go-sdk middleware to produce an
// authenticated context (the only way to set the unexported TokenInfo key).
verifier := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
	// ...
}
```

**What to Mock:**

- Auth token verification (function-value stub through real middleware)

**What NOT to Mock:**

- Qdrant — use a real instance via testcontainers (integration-first). The store's fail-closed contracts are only meaningful against real Qdrant.

## Fixtures and Factories

**Test Deps:** `testDeps(t *testing.T) *deps` (`internal/server/tools_test.go:190`) is the shared factory — builds a `*deps` wired to the test Qdrant, `t.Helper()`-marked, and `t.Skip`s when no Qdrant is available.

**Cleanup helper:** `cleanupErr(t, what, err)` swallows `store.ErrNotFound` and reports other cleanup errors.

**Location:** Helpers live in the package's primary `_test.go` (e.g. `tools_test.go`), not a separate fixtures file.

## Integration Test Provisioning

Packages needing Qdrant define `TestMain(m *testing.M)` (`internal/server/tools_test.go`, `internal/store/store_test.go`):

1. If `ENGRAM_QDRANT_TEST_ADDR` is set, use that instance (fast path / CI override)
2. Otherwise boot an ephemeral Qdrant via `testcontainers-go/modules/qdrant` and tear it down after
3. If neither Docker nor the env addr is available, integration tests `t.Skip` (with an actionable message) rather than fail

This makes `go test ./...` pass on a machine without Docker (integration tests skip) while still exercising real Qdrant in CI.

## Coverage

**Requirements:** No enforced threshold. Recent commits explicitly backfill coverage (e.g. `list_rules`/`store.List`).

**View Coverage:**

```bash
task test:coverage     # writes cover.out
go tool cover -html=cover.out
```

## Test Types

**Unit Tests:** Pure validation/logic (`TestValidateStoreRule`) — no external deps, always run.

**Integration Tests:** Handler + store paths against real Qdrant via testcontainers; skip without Docker. The bulk of `internal/server` and `internal/store` tests.

**Benchmarks:** `internal/store/bench_test.go`, run via `task bench` (`-benchmem`, no live Qdrant required for the benched hot paths).

**Eval Tests:** `task eval:summary` runs `TestSummaryFidelity` in `internal/summarize/` gated behind `ENGRAM_SUMMARY_EVAL=1` (needs a live gateway+model); skipped by default.

**Python Hook Tests:** `pytest` under `skill/engram/hooks/tests`, linted with `ruff`.

## Common Patterns

**Skip-when-unavailable:**

```go
if testQdrantAddr == "" {
	t.Skip("no Qdrant available: set ENGRAM_QDRANT_TEST_ADDR or start Docker (testcontainers)")
}
```

**Error Testing:**

```go
if err := validateStoreRule(tc.a); err == nil {
	t.Errorf("%s: expected error, got nil", tc.name)
}
// sentinel checks use errors.Is, never ==
if err != nil && !errors.Is(err, store.ErrNotFound) { /* ... */ }
```

**Server-set field assertions:** After a store, `Get` the record and assert server-authored fields (`Category`, `Source`, `Visibility`, `ShortID`, `SummarySource`) match the contract rather than trusting the write path.

---

*Testing analysis: 2026-07-08*
