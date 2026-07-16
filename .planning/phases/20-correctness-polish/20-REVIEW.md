---
phase: 20-correctness-polish
reviewed: 2026-07-15T00:00:00Z
depth: standard
files_reviewed: 16
files_reviewed_list:
  - proto/engram/v1/engram.proto
  - internal/server/connectapi.go
  - internal/server/connectapi_test.go
  - internal/server/connectdescriptor_test.go
  - internal/server/tools_test.go
  - internal/embed/embed.go
  - internal/config/embedparams.go
  - internal/config/embedparams_test.go
  - internal/store/store.go
  - internal/store/store_test.go
  - charts/engram/templates/_helpers.tpl
  - charts/engram/templates/memory-mcp.yaml
  - charts/engram/templates/summarize-cronjob.yaml
  - charts/engram/values.yaml
  - Taskfile.yaml
  - .github/workflows/ci.yaml
findings:
  critical: 0
  warning: 0
  info: 0
  total: 0
status: clean
---

# Phase 20: Code Review Report

**Reviewed:** 2026-07-15
**Depth:** standard
**Status:** clean

## Narrative Findings (AI reviewer)

### Summary

Re-review (auto-fix iteration 2) of Phase 20's correctness/polish changes against
base `72a80119..HEAD`. All four prior findings are confirmed resolved, no
regressions were introduced by the fixes, and `go build` across the four touched
Go packages (`internal/embed`, `internal/config`, `internal/server`,
`internal/store`) is clean — confirming the #304 config↔embed dependency edge does
not cycle. All reviewed files meet quality standards. No issues found.

**Prior findings — resolution confirmed:**

- **WR-01 (chart:validate orphaned) — RESOLVED.** The guardrail now runs in an
  automated gate. `.github/workflows/ci.yaml` chart job installs Task via
  `go install github.com/go-task/task/v3/cmd/task@latest` (mirroring the existing
  actionlint job convention) and runs `task chart:validate` on every PR;
  `Taskfile.yaml:114` also wires `chart:validate` as a `chart:lint` dep so local
  `task chart:lint` exercises it. The target enforces the real invariants: default
  render must NOT emit a CronJob (D-07), `--set ...cronjob.enabled=true` must emit
  one with `concurrencyPolicy: Forbid` and `schedule: "0 3 * * *"` (D-08), and the
  `engram.containerEnv` checksum pin (D-09). The `\*` BRE escapes in the schedule
  grep correctly match the rendered `0 3 * * *`, and the `values.yaml` defaults
  (`schedule: "0 3 * * *"`, `concurrencyPolicy: Forbid`) match the asserted render.

- **WR-02 (MintShortID termination hole) — RESOLVED.** The loop is now
  `for attempts, spins := 0, 0; attempts < maxMintAttempts; spins++` with an
  absolute `maxMintSpins = maxMintAttempts * 100` (1600) cap checked at the top of
  each iteration (store.go:1801-1810). Traced: seen-map hits `continue` without
  incrementing `attempts` (D-05 preserved — the 16 real-collision-check budget is
  untouched by skips), while the loop post-statement `spins++` fires on every
  iteration including skips, so a generator returning only already-seen candidates
  halts at exactly `maxMintSpins` and returns an `errors.Is`-checkable
  `ErrShortIDExhausted`. `TestMintShortIDDegenerateGeneratorTerminates`
  (store_test.go:2829) asserts both `errors.Is(err, ErrShortIDExhausted)` and
  `calls == maxMintSpins`; the count arithmetic is exact (gen() called on spins
  0..1599, not on the terminating spins==1600 check). The 1600 cap is
  astronomically clear of any legitimate seen-map churn in the 32^10 space, so it
  cannot fire on the production `shortid.New` path.

- **IN-01 (discarded json.Marshal error) — RESOLVED.** embed.go:243-245 now does
  `body, err := json.Marshal(m); if err != nil { return nil, fmt.Errorf("embeddings: marshal request body: %w", err) }`.
  The single map-based body build sets `model`/`input` last (authoritative over
  operator params), matching the documented wire contract.

- **IN-02 (no citations wire round-trip test) — RESOLVED.**
  `TestConnectSearchDiscoveriesCitationsRoundTrip` (connectapi_test.go:980) seeds a
  discovery carrying two `store.Citation` records, drives the real
  `SearchDiscoveries` handler → `memoriesToProto` → wire path, and asserts both
  `Kind` and every citation field (`Kind`/`Ref`/`Locator`/`Pin`/`Excerpt`) survive
  field-for-field. `citationsToProto` (connectapi.go:33) correctly returns `nil`
  (not an empty slice) for non-discovery memories, covered by
  `TestMemoryToProtoZeroValueKindAndCitations`.

---

_Reviewed: 2026-07-15_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
