# Deferred Items — Phase 02 (out-of-scope discoveries)

## 02-05: `TestExitCodeBaseline` rows assume `ENGRAM_REINDEX_TARGET`/`ENGRAM_MIGRATE_OWNER` are unset

**Tracked:** [#476](https://github.com/seanb4t/engram/issues/476)

**Found during:** 02-05 Task 2's determinism stress-test (`go test ./cmd/engram/... -shuffle=on`
with `ENGRAM_REINDEX_TARGET`/`ENGRAM_MIGRATE_OWNER` deliberately set, to prove golden_test.go's own
normalization holds under the hazard it names).

**Issue:** `exitcode_baseline_test.go`'s `reindex/missing-target` and `migrate-set-owner/missing-owner`
rows expect a usage error (`exitUsage`) when `--target`/`--owner` is omitted. Both flags' pflag default
is `os.Getenv("ENGRAM_REINDEX_TARGET")`/`os.Getenv("ENGRAM_MIGRATE_OWNER")` (`reindex.go:111`,
`migrate.go:147`) — pre-existing, unrelated to this plan. If either env var happens to be set in the
process running `go test`, the flag silently carries a non-empty value, the row's "missing" case never
triggers, and the command instead attempts to dial Qdrant, reporting `exitUnavailable` instead.

**Scope:** out of scope for 02-05 — `exitcode_baseline_test.go` is not a file this plan touches, and the
hazard is orthogonal to the `--help`/catalog goldens' own determinism (which 02-05 fixed for the SAME
two flags' DEFAULT VALUE inside the goldens, via `golden_test.go`'s `envDerivedFlagDefaults`
normalization — that fix does not and cannot reach `exitcode_baseline_test.go`'s independent flag-reset
path).

**Suggested fix (future):** `exitcode_baseline_test.go`'s `resetCommandFlagState`/table-row harness could
`t.Setenv("ENGRAM_REINDEX_TARGET", "")`/`t.Setenv("ENGRAM_MIGRATE_OWNER", "")` before each affected row —
except, per the same `init()`-runs-once constraint 02-05 documented, `t.Setenv` cannot retroactively
change a flag's already-computed `DefValue`. A real fix needs either resetting `f.DefValue` directly
(the same technique `golden_test.go`'s `withGoldenDeterminism` uses) or moving these two flags off a
direct `os.Getenv` default entirely (e.g., reading the env var inside `RunE` instead of at flag
registration time).

**Verification of scope boundary:** without the artificially-injected env vars, `go clean -testcache &&
go test ./cmd/engram/... -count=1 -shuffle=on` passes cleanly across multiple runs — the fragility only
manifests when a contributor's shell happens to carry either variable, which is not the case in this
repo's CI or documented local dev flow.
