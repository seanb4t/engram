---
phase: 20-correctness-polish
fixed_at: 2026-07-15T00:00:00Z
review_path: .planning/phases/20-correctness-polish/20-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 20: Code Review Fix Report

**Fixed at:** 2026-07-15
**Source review:** .planning/phases/20-correctness-polish/20-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4 (2 warning + 2 info; fix_scope=all)
- Fixed: 4
- Skipped: 0

## Fixed Issues

### WR-02: `MintShortID` termination guarantee has a hole for a degenerate candidate generator

**Files modified:** `internal/store/store.go`, `internal/store/store_test.go`
**Commit:** 2b93b250
**Applied fix:** Added `maxMintSpins = maxMintAttempts * 100`, an absolute total-iteration
cap. Restructured the loop to `for attempts, spins := 0, 0; attempts < maxMintAttempts; spins++`
so seen-map skips `continue` (bumping only `spins`, via the loop post-statement) while real
Qdrant `Count()`-checked candidates do `attempts++` before the check. A degenerate generator
returning only already-seen candidates now trips the spin cap and returns a wrapped
`ErrShortIDExhausted` instead of looping forever. D-05 is preserved: real-collision-check
attempts stay capped at `maxMintAttempts=16` and seen-map skips still do not consume that
budget (verified — `TestMintShortIDSeenMapDoesNotConsumeBudget` still asserts exactly 16 real
calls). New test `TestMintShortIDDegenerateGeneratorTerminates` proves the seen-only loop
terminates after exactly `maxMintSpins` generator calls. All `TestMintShortID*` tests pass.

### IN-01: `embed()` discards the `json.Marshal` error

**Files modified:** `internal/embed/embed.go`
**Commit:** 5727573d
**Applied fix:** Replaced `body, _ := json.Marshal(m)` with error capture and a wrapped
return (`return nil, fmt.Errorf("embeddings: marshal request body: %w", err)`). The following
`req, err := http.NewRequestWithContext(...)` remains valid (`req` is a new binding). `internal/embed`
tests pass.

### IN-02: No end-to-end test that discovery citations round-trip on the Connect wire

**Files modified:** `internal/server/connectapi_test.go`
**Commit:** d5067972
**Applied fix:** Added `TestConnectSearchDiscoveriesCitationsRoundTrip`, which seeds a
discovery carrying `Kind: "map"` and two `Citations` (the shared `seedDiscoveries` helper
leaves `Citations` empty, so a dedicated inline seed was used to avoid perturbing existing
tests), invokes the real `SearchDiscoveries` handler, and asserts `kind` plus every citation
field survives the `SearchDiscoveries -> memoriesToProto -> Connect response` path
field-for-field. Test passes against the Qdrant testcontainer.

### WR-01: `chart:validate` guardrail is orphaned — never runs in CI or the default task chain

**Files modified:** `.github/workflows/ci.yaml`, `Taskfile.yaml`
**Commit:** ccd4d5ce
**Applied fix:** Wired the guardrail into an automated gate two ways. (1) Added a `chart validate`
step to the CI `chart` job that installs Task via `go install github.com/go-task/task/v3/cmd/task@latest`
(mirroring the existing actionlint job pattern, since the runner is bare) plus a `setup-go` step,
then runs `task chart:validate` — so the D-07/D-08 CronJob invariants and the D-09
`engram.containerEnv` drift-pin execute on every PR. (2) Added `deps: [chart:validate]` to the
`chart:lint` target so a direct `task chart:lint` also runs the guardrail locally. The `task`
default chain was intentionally not touched (pre-existing rumdl/.planning-exclude gap, deferred
to Phase 21). Verified: `task chart:validate` -> "chart:validate: OK", `task chart:lint` runs the
dep and exits 0, `actionlint .github/workflows/ci.yaml` exits 0, `helm lint charts/engram` passes.

---

_Fixed: 2026-07-15_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
