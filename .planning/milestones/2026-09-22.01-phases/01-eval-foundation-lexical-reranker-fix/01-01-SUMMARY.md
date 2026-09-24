---
phase: 01-eval-foundation-lexical-reranker-fix
plan: 01
subsystem: eval
tags: [koanf, cosine-similarity, retrieval-eval, config, tdd]

# Dependency graph
requires: []
provides:
  - "StoreAndEmbedderFromEnvNoEnsure returns the resolved *config.Config alongside the store/embedder/identity, from the single existing config load (D-14)"
  - "internal/retrievaleval/gate.go: resolveEvalGate/retrievalEvalEnabled — a package-local koanf gate for ENGRAM_RETRIEVAL_EVAL, deliberately unregistered in internal/config (D-15)"
  - "internal/retrievaleval/vector.go: cosineDistance + differMinCosineDistance — the cosine-epsilon differ gate replacing bit-identity (D-13)"
affects: ["01-04", "01-05", "01-06 (Phase 1's later plans build the paraphrase eval and reranker decision on these now-trustworthy gates)"]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
actuals:
  tokens: 7128
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Package-local koanf gate for a test-only env var, mirroring production's prefix/precedence without registering in internal/config's field registry"
    - "Hard-fail-on-degenerate-input numeric gates (cosineDistance) rather than coercing NaN/Inf to a value that could silently satisfy a threshold comparison"

key-files:
  created:
    - internal/retrievaleval/gate.go
    - internal/retrievaleval/gate_test.go
    - internal/retrievaleval/vector.go
    - internal/retrievaleval/vector_test.go
  modified:
    - internal/server/tools.go
    - internal/server/tools_test.go
    - cmd/engram/reindex.go
    - internal/retrievaleval/retrieval_eval_test.go
    - internal/retrievaleval/doc.go

key-decisions:
  - "D-14 implemented exactly as planned: StoreAndEmbedderFromEnvNoEnsure widened to 6 return values (added *config.Config), single-load invariant preserved, reindex.go discards the new value"
  - "D-15 (revised) implemented exactly as planned: ENGRAM_RETRIEVAL_EVAL resolved by a package-local koanf load (gate.go), never added to internal/config's registry, never calls Config.Validate()"
  - "D-13 implemented exactly as planned: differ gate now compares by cosine distance > 1e-3 epsilon, with NaN/Inf/zero-norm/length-mismatch as hard errors, and a causally-neutral PASS log"
  - "TestSymmetricEmbedConfig's table-row local variable renamed from cfg to loadedCfg so the file's only literal match for symmetricEmbedConfig(cfg.Embed) is the production differ-test call site the plan's acceptance criterion targets"

patterns-established:
  - "Test-local koanf gate for a var deliberately excluded from the production registry (precedent for any future test-only ENGRAM_* flag)"

requirements-completed: [EVAL-01, EVAL-02]

coverage:
  - id: D1
    description: "StoreAndEmbedderFromEnvNoEnsure returns the resolved *config.Config the store/embedder were built from, with the single-load invariant intact end to end (D-14, #354)"
    requirement: EVAL-02
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce"
        status: pass
      - kind: unit
        ref: "internal/server/tools_test.go#TestStoreAndEmbedderFromEnvNoEnsureValidatesConfig"
        status: pass
      - kind: unit
        ref: "internal/retrievaleval/retrieval_eval_test.go#TestSymmetricEmbedConfig"
        status: pass
    human_judgment: false
  - id: D2
    description: "ENGRAM_RETRIEVAL_EVAL resolved by a package-local koanf load with production prefix/precedence, unregistered, never validated — an unrelated malformed ENGRAM_* var cannot fail the package, a malformed gate value errors loudly (D-15, #354)"
    requirement: EVAL-02
    verification:
      - kind: unit
        ref: "internal/retrievaleval/gate_test.go#TestEvalGate"
        status: pass
      - kind: integration
        ref: "go test ./internal/retrievaleval/ -run ^TestSharedQdrantAddressHonored$ with ENGRAM_EMBED_DIM=abc (SKIP, not FAIL)"
        status: pass
      - kind: integration
        ref: "go test ./internal/retrievaleval/ -run ^TestEvalGate$ with ENGRAM_RETRIEVAL_EVAL=yes (non-zero exit naming the var and value)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Differ gate compares by cosine distance above a 1e-3 epsilon instead of bit identity; NaN/Inf/zero-norm/length-mismatch are hard errors; the PASS log makes no causal claim (D-13, #353)"
    requirement: EVAL-01
    verification:
      - kind: unit
        ref: "internal/retrievaleval/vector_test.go#TestCosineDistance"
        status: pass
      - kind: unit
        ref: "internal/retrievaleval/retrieval_eval_test.go#TestRetrievalEval_AsymmetryDiffer (SKIP with gate off, confirming wiring)"
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-09-23
status: complete
---

# Phase 1 Plan 1: Eval Foundation Correctness Gates Summary

**Widened `StoreAndEmbedderFromEnvNoEnsure` to return its resolved config (D-14), moved `ENGRAM_RETRIEVAL_EVAL` off raw `os.Getenv` onto a package-local koanf gate that never touches the production registry (D-15), and replaced the differ gate's bit-identity comparison with a cosine-distance epsilon that tolerates embedder float jitter but rejects true near-identity (D-13).**

## Performance

- **Duration:** ~45 min
- **Tasks:** 3/3 completed
- **Files created:** 4
- **Files modified:** 5

## Accomplishments

- `internal/server.StoreAndEmbedderFromEnvNoEnsure` now returns `(*store.Store, uint64, *embed.Client, string, *config.Config, error)` — the exact `*config.Config` `loadAndValidate` produced, still loaded exactly once. `reindex.go` discards it; the retrieval eval's symmetric-config skip reads it directly instead of four independent `os.Getenv` calls.
- New `internal/retrievaleval/gate.go` resolves `ENGRAM_RETRIEVAL_EVAL` through a fresh `koanf.New(".")` load (default-false layer, then an `env.Provider` layer using `config.Prefix` and a `TransformFunc` that maps only the one var, empty-value-preserves-default like production). It is deliberately absent from `internal/config`'s registry and never calls `Config.Validate()`, so an unrelated malformed `ENGRAM_*` var can never fail the package while the eval is off, and a malformed gate value itself errors rather than reading as off.
- New `internal/retrievaleval/vector.go` adds `cosineDistance`, erroring on empty/mismatched-length vectors, any NaN/±Inf component, or a zero-norm vector — never returning a number for degenerate input, so a NaN distance can never silently satisfy the pass threshold. The differ gate in `retrieval_eval_test.go` now gates on `distance > differMinCosineDistance` (1e-3) with a causally-neutral PASS log.

## Task Commits

Each task was committed atomically:

1. **Task 1: Resolved config end to end (D-14)** — `546bbdaf` (refactor)
2. **Task 2: ENGRAM_RETRIEVAL_EVAL resolved by koanf, not raw env (D-15)** — `076c319e` (test)
3. **Task 3: Differ gate by cosine distance (D-13, #353)** — `e03368cc` (test)

**Plan metadata:** (this commit)

## RED Observations (TDD)

- **Task 1:** Before widening the return arity, `go vet ./internal/server/ ./internal/retrievaleval/` failed at compile time: `assignment mismatch: 5 variables but server.StoreAndEmbedderFromEnvNoEnsure returns 6 values` at both `tools_test.go:5154` and `retrieval_eval_test.go:80` (the test files already asserted the new 6-value arity before `tools.go` was changed). Confirmed RED, then widened the function to GREEN (`go test -run '^TestStoreAndEmbedderFromEnvNoEnsure(ValidatesConfig|LoadsConfigOnce)$'` and `-run '^TestSymmetricEmbedConfig$'` both PASS).
- **Task 2:** `gate_test.go` was run against a stub `resolveEvalGate` that always returned `(false, nil)`. `TestEvalGate` failed (`--- FAIL: TestEvalGate`) on the `1`/`true`/`TRUE`/malformed/gate-set-with-noise rows. Restored the real koanf-backed implementation; `TestEvalGate` PASSed (12/12 subtests).
- **Task 3:** `vector_test.go` was run against a stub `cosineDistance` that always returned `(0, nil)`. `TestCosineDistance` failed (`--- FAIL: TestCosineDistance`) on the orthogonal/opposite/near-identical-jitter and every degenerate-input error row. Restored the real cosine-distance implementation; `TestCosineDistance` PASSed.

## Files Created/Modified

- `internal/server/tools.go` — `StoreAndEmbedderFromEnvNoEnsure` widened to 6 return values
- `internal/server/tools_test.go` — both `TestStoreAndEmbedderFromEnvNoEnsure*` tests updated to the new arity; `LoadsConfigOnce` gained pointer-identity + `Embed.Dim` assertions
- `cmd/engram/reindex.go` — call site updated to discard the new config return value
- `internal/retrievaleval/retrieval_eval_test.go` — `symmetricEmbedConfig` + `TestSymmetricEmbedConfig` added; the differ test's skip now reads the resolved config; `requireEvalEnabled(t)` helper added and used by every gated test; `TestMain` resolves the koanf gate and exits non-zero on a malformed value; the differ block now uses `cosineDistance`
- `internal/retrievaleval/gate.go` (new) — `resolveEvalGate`, `retrievalEvalEnabled`, `evalGateEnv`, `evalGateKey`
- `internal/retrievaleval/gate_test.go` (new) — `TestEvalGate`
- `internal/retrievaleval/vector.go` (new) — `cosineDistance`, `differMinCosineDistance`
- `internal/retrievaleval/vector_test.go` (new) — `TestCosineDistance`
- `internal/retrievaleval/doc.go` — package doc updated to describe the koanf-resolved gate

## Decisions Made

- Renamed `TestSymmetricEmbedConfig`'s table-row local variable from `cfg` to `loadedCfg`. The plan's own acceptance criterion for Task 1 requires exactly one literal match of `symmetricEmbedConfig(cfg.Embed)` in the file, but the plan's own `<behavior>` spec for the new test also calls for `symmetricEmbedConfig(cfg.Embed)` verbatim — the two plan sections collide on a shared literal. Renaming the test's local variable (no behavior change) resolves the collision in favor of the acceptance criterion's intent: pinning the production differ-test call site specifically. [Rule 3 — blocking, resolved without user input: a naming choice, not an architectural change]
- One `go vet` pass across `./internal/server/ ./cmd/engram/ ./internal/retrievaleval/` surfaces a pre-existing, unrelated finding: `cmd/engram/operator_view_test.go:441` duplicate `json:"dup"` struct tag. This is `TestOperatorViewDuplicateKeyAdjacency`'s deliberate adjacency probe, carrying a `//nolint:govet` comment that suppresses it for `golangci-lint` but not for bare `go vet` (confirmed present at `HEAD` before this plan's first commit, via `git show HEAD:cmd/engram/operator_view_test.go`). Out of scope for this plan (scope boundary: pre-existing, unrelated file) — not fixed. `golangci-lint run ./internal/retrievaleval/...` and the full `task lint` (which runs `golangci-lint`, not bare `go vet`) are both clean.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — blocking, scope-internal] `symmetricEmbedConfig(cfg.Embed)` literal collision in the acceptance grep**
- **Found during:** Task 1, acceptance-criteria verification
- **Issue:** The plan's Task 1 acceptance criteria assert `rg -o 'symmetricEmbedConfig[(]cfg[.]Embed[)]' internal/retrievaleval/retrieval_eval_test.go | wc -l` prints `1`, but the plan's own `<behavior>` spec instructs `TestSymmetricEmbedConfig` to call `config.Load(nil)` into a variable and assert `symmetricEmbedConfig(cfg.Embed)` — the same literal as the differ-test call site, producing 2 matches
- **Fix:** Renamed the new test's local variable to `loadedCfg` (`symmetricEmbedConfig(loadedCfg.Embed)`), preserving identical behavior while leaving the differ test as the sole literal match
- **Files modified:** `internal/retrievaleval/retrieval_eval_test.go`
- **Verification:** `rg -o 'symmetricEmbedConfig[(]cfg[.]Embed[)]' internal/retrievaleval/retrieval_eval_test.go | wc -l` → `1`
- **Commit:** `546bbdaf`

**Total deviations:** 1 auto-fixed (naming collision, no behavior change). **Impact:** none — cosmetic rename only, all behavior and test coverage identical to the plan's intent.

## Authentication Gates

None.

## Known Stubs

None.

## Threat Flags

None — this plan implements exactly the two mitigations recorded in the plan's own `<threat_model>` (T-01-01, T-01-02), both now proven by `TestEvalGate`'s unrelated-malformed rows and `TestCosineDistance`'s degenerate-input rows respectively. No new trust boundary or surface was introduced.

## Self-Check: PASSED

- `[ -f internal/retrievaleval/gate.go ]` → FOUND
- `[ -f internal/retrievaleval/gate_test.go ]` → FOUND
- `[ -f internal/retrievaleval/vector.go ]` → FOUND
- `[ -f internal/retrievaleval/vector_test.go ]` → FOUND
- `git log --oneline --all --grep="01-01"` → 0 matches (plan uses D-13/D-14/D-15/#353/#354 in subjects, not the plan-id string); commits verified directly by hash below instead
- `git log --oneline --all | grep -q 546bbdaf` → FOUND
- `git log --oneline --all | grep -q 076c319e` → FOUND
- `git log --oneline --all | grep -q e03368cc` → FOUND
- Re-ran every task's `<acceptance_criteria>` command: all PASS (see per-task verify output above)
- Re-ran the plan-level `<verification>`: `env -u ENGRAM_RETRIEVAL_EVAL go test ./internal/retrievaleval/ ./internal/server/ ./cmd/engram/ -count=1` → exit 0; `task lint` → exit 0

Ready for `01-02`.
