---
phase: 02-custom-auth-headers
fixed_at: 2026-09-14T10:30:00Z
review_path: .planning/phases/02-custom-auth-headers/02-REVIEW.md
iteration: 1
findings_in_scope: 2
fixed: 2
skipped: 1
status: partial
---

# Phase 02: Code Review Fix Report

**Fixed at:** 2026-09-14T10:30:00Z
**Source review:** .planning/phases/02-custom-auth-headers/02-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 2 (WR-01, in-scope by default; IN-02, scope-extended by the orchestrator)
- Fixed: 2 (WR-01, IN-02)
- Skipped: 1 (IN-01, explicitly out of scope this pass per orchestrator decision)

Note: IN-01 is normally outside `fix_scope: critical_warning` (it is an Info finding). It is
listed here under Skipped Issues, per the orchestrator's explicit instruction to record it as
"skipped: out of scope this pass" with a forward-looking note, rather than silently omitting it.

## Fixed Issues

### WR-01: `internal/setup` has no Authorization-collision or duplicate-name guard of its own for `Options.Headers`

**Files modified:** `internal/setup/runtime.go`, `internal/setup/claudecode_test.go`
**Commit:** 2578e42
**Applied fix:** Per the orchestrator's explicit decision, did NOT duplicate CLI-boundary
validation into `internal/setup` (02-RESEARCH.md Pitfall 5 locks validation at the CLI boundary
only). Instead fixed the genuine correctness defect:
- `sortedHeaders` (`internal/setup/runtime.go`) now breaks a case-insensitive tie with an exact
  byte-wise `strings.Compare(a.Name, b.Name)`, making the order **total** rather than dependent
  on `slices.SortFunc`'s (unguaranteed) stability.
- `Options.Headers`' doc comment now states the package contract explicitly: headers are expected
  to arrive already validated by the CLI boundary (`setupParseHeaders`); `internal/setup` orders
  and renders them deterministically but does not re-validate, and a direct package caller that
  skips that validation owns the consequences.
- Added `TestSortedHeadersTotalOrder` in `internal/setup/claudecode_test.go`, proving two names
  equal under case folding but different byte-wise (`a-key` / `A-key`) sort deterministically in
  both input orders.

**RED/GREEN evidence:** Temporarily reverted the tiebreak line in `sortedHeaders` and re-ran
`go test ./internal/setup/ -run TestSortedHeadersTotalOrder -v -count=1`:
```
claudecode_test.go:372: sortedHeaders([{a-key LOWER} {A-key UPPER}]) = [{a-key LOWER} {A-key UPPER}], want [{A-key UPPER} {a-key LOWER}]
--- FAIL: TestSortedHeadersTotalOrder (0.00s)
    --- FAIL: TestSortedHeadersTotalOrder/lower-then-upper (0.00s)
    --- PASS: TestSortedHeadersTotalOrder/upper-then-lower (0.00s)
FAIL
```
Restored the tiebreak and re-ran the same command — GREEN:
```
--- PASS: TestSortedHeadersTotalOrder (0.00s)
    --- PASS: TestSortedHeadersTotalOrder/upper-then-lower (0.00s)
    --- PASS: TestSortedHeadersTotalOrder/lower-then-upper (0.00s)
ok  	github.com/seanb4t/engram/internal/setup	0.148s
```

### IN-02: Help text's description of the ENVVAR grammar undersells what is actually rejected

**Files modified:** `cmd/engram/setup.go`, `cmd/engram/testdata/help.golden`,
`docs-site/src/content/docs/guides/agent-setup.md`
**Commit:** 68aad2c
**Applied fix:** Restated the `--header` ENVVAR restriction in both the CLI long-help and the
docs-site guide as "a POSIX-shell identifier (ASCII letters, digits, and underscore, not starting
with a digit)" instead of the incomplete "rejects `$`, `{`, whitespace, or `:`" list. Left the
`Accepted --auth modes` block untouched (D-01). Regenerated `cmd/engram/testdata/help.golden` via
`go test ./cmd/engram -update` (the tests' own `-update` idiom); `catalog.golden` did not move
(the `Short` summary line is unchanged, as required).

## Skipped Issues

### IN-01: `setupHeadersSummary` duplicates `sortedHeaders`'s comparator instead of sharing it

**File:** `cmd/engram/setup.go:91-101`, `internal/setup/runtime.go:197-...`
**Reason:** skipped: out of scope this pass, per explicit orchestrator decision.
**Original issue:** `cmd/engram/setup.go`'s `setupHeadersSummary` re-implements the same
case-insensitive sort comparator `sortedHeaders` already has, because `sortedHeaders` is
unexported. A future change to one comparator without the other would silently desynchronize the
`headers` row facet's order from the rendered command's own header order.

With WR-01's tiebreak now making both `sortedHeaders`'s and `setupHeadersSummary`'s orderings
total (though `setupHeadersSummary`'s own comparator was NOT changed and remains a simple
case-fold sort without a tiebreak — see below), the cheapest durable guard for a follow-up is a
test asserting the two orderings agree on a representative input set, including a case-colliding
pair. Left for a follow-up rather than exporting a shared comparator this pass.

## Verification

All fixes verified in the main checkout at `/Volumes/Code/github.com/seanb4t/engram`, branch
`feat/2026-09-13.01` (no isolated worktree — `workflow.use_worktrees` was not read/enforced by
this ad hoc invocation; edits were made directly on the branch per the orchestrator's explicit
working-tree instruction).

**Red-evidence harness:** `go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1 -v`
— PASS. All five phase-02 patches (`02-01-claudecode-header-args.patch`,
`02-01-codex-header-decline.patch`, `02-02-generic-header-order.patch`,
`02-02-opencode-header-dialect.patch`, `02-03-header-validation.patch`) still apply cleanly, drive
their mapped target test RED, and revert cleanly — no regeneration needed.

**Final gate:**
- `task` — lint + test, all clean (`golangci-lint`, `rumdl`, `actionlint`, `yamlfmt`, `ruff`, Python + Go test suites) — PASS
- `task license:check` — 405 valid, 0 invalid — PASS
- `git diff --exit-code -- go.mod go.sum` — clean, no diff — PASS
- `go test ./internal/keylinks/ -count=1` — PASS
- `go run ./internal/surfacesgen --check-setup` — exit 0, no drift — PASS
- `go test ./internal/setup/ -count=1 -shuffle=on` — PASS
- `go test ./cmd/engram -count=1` — PASS

---

_Fixed: 2026-09-14T10:30:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
