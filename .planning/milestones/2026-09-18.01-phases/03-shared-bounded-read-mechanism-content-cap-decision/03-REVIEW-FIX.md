---
phase: 03-shared-bounded-read-mechanism-content-cap-decision
fixed_at: 2026-09-19T23:23:26Z
review_path: .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-REVIEW.md
iteration: 1
findings_in_scope: 1
fixed: 1
skipped: 0
status: all_fixed
---

# Phase 03: Code Review Fix Report

**Fixed at:** 2026-09-19T23:23:26Z
**Source review:** .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 1 (fix_scope: critical_warning — WR-01 only; IN-01/IN-02 out of scope per repo constraints)
- Fixed: 1
- Skipped: 0

**Verification environment:** main checkout (no worktree — `workflow.use_worktrees` bypassed per this run's explicit `repo_constraints`: branch `feat/2026-09-18.01`, single commit, no `git stash`). Docker/Qdrant was available locally (`testcontainers` auto-provisioned `qdrant/qdrant:v1.19.1`), so the final gate ran against a live Qdrant, not skipped.

## Fixed Issues

### WR-01: Config validation and cap-resolution use different integer parsers, so a value that passes `Validate()` can silently fall back to the default at runtime

**Files modified:** `internal/config/validate.go`, `internal/config/validate_test.go`, `internal/server/tools.go`, `.planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence/03-01-config-accepts-zero.patch`
**Commit:** `7e6113cc` — `fix(03): WR-01 make validated cap range equal enforced range`
**Applied fix:**

- Added two exported functions to `internal/config` — `ParsePositiveIntCap` (for the three always-enforced caps) and `ParseNonNegativeIntCap` (for `ENGRAM_MEMORY_MAX_SUMMARY_BYTES`) — both parsing with `strconv.Atoi` (the same platform-`int` width `internal/server`'s enforcement side already used), rather than the wider `strconv.ParseUint(value, 10, 64)` `Config.Validate` used before. This is the one shared parser both sides now call (D-00: idiom/long-term maintenance over two parsers kept in sync by convention), closing the review's named divergence exactly: a value between `math.MaxInt64` and `math.MaxUint64` could previously pass `Validate()` (via `ParseUint`) and then silently fall back to the documented default at runtime (via `Atoi`, with only a `slog.Warn`).
  - `validatePositiveCap` (config side, for `MAX_CONTENT_BYTES`/`MAX_TAGS`/`MAX_TAG_BYTES`) now calls `ParsePositiveIntCap`.
  - `Config.Validate`'s `MAX_SUMMARY_BYTES` check now calls `ParseNonNegativeIntCap`.
  - `internal/server/tools.go`'s `positiveIntOrDefault` (the three always-enforced caps' runtime builder) now calls `config.ParsePositiveIntCap`.
  - `internal/server/tools.go`'s `maxMemorySummaryBytes` now calls `config.ParseNonNegativeIntCap`.
- Per this run's `repo_constraints`, checked whether `ENGRAM_MEMORY_MAX_SUMMARY_BYTES` shared the same `ParseUint`-validates/`Atoi`-enforces split: it did (same overflow-silent-fallback class), so its overflow-range bound was tightened to match. Its `0 disables` semantics (D-18) are **unchanged** — `ParseNonNegativeIntCap` only bounds the parse range; the "0 is valid, honored as disabled" decision still lives entirely in `maxMemorySummaryBytes`/`Config.Validate`, unaffected by this fix.
- **Red-evidence patch regenerated in the same commit:** `.planning/phases/.../red-evidence/03-01-config-accepts-zero.patch` targeted `validatePositiveCap`'s exact old body (`switch n, err := strconv.ParseUint(...)`), which no longer exists after this fix, so `git apply --check` on it failed. Regenerated the patch to remove the equivalent zero-rejection arm from the new `ParsePositiveIntCap` (the `if n <= 0 { return 0, errors.New(...) }` block). Reproved RED: applied the regenerated patch, confirmed `TestMemoryCapsRejectZeroAndNonPositive` failed (`--- FAIL:` for the `MAX_CONTENT_BYTES`/`MAX_TAGS`/`MAX_TAG_BYTES` `0`/`-1` subtests), then reverted with `git apply -R` and confirmed `git diff --exit-code` clean before committing.
- **New test added** (`internal/config/validate_test.go`, `TestOverflowValueRejectedByValidate`): sets each of the four fields to `"9223372036854775808"` (`math.MaxInt64 + 1` — valid `uint64`, invalid platform `int`) and asserts `Config.Validate()` now rejects it, naming the env var. Proved RED first (observed `--- FAIL:` for all four subtests against the pre-fix `strconv.ParseUint`-based validation, both for the three always-enforced caps and for `MAX_SUMMARY_BYTES`), then GREEN after the fix.
- Confirmed `TestRedEvidencePatchesAreLive` (all 25 patches across phases 01/02/03) is green post-fix, including the 5 other patches that also touch `internal/server/tools.go` (`03-01-content-cap-removed`, `03-01-tags-cap-removed`, `03-03-record-caps-not-wired`, `03-03-update-content-cap-removed`, `03-03-update-gates-on-presence-not-change`) — `git apply`'s context matching tolerated the line-number shift from the added doc comments/functions.

**Final gate results:**
- `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/config/ ./internal/server/ ./internal/store/ -count=1` → **PASS** (`ok internal/config 0.093s`, `ok internal/server 8.298s`, `ok internal/store 138.041s` — the 138s store run includes the full, non-`-short` red-evidence harness against a live testcontainers-provisioned Qdrant).
- `task lint` → **PASS** (actionlint, setup-check, golangci-lint, yamlfmt, rumdl, ruff check + format all clean).

## Out of Scope (not attempted — per this run's explicit repo_constraints)

- **IN-01** (`internal/store/orderedpage.go` — `scrollOrderedPage` has no production caller yet): Phase 4 wiring, explicitly excluded.
- **IN-02** (`internal/server/tools.go:1902-1906` — tag-set trim rejected if still over cap): documented all-or-nothing design choice, explicitly excluded.

---

_Fixed: 2026-09-19T23:23:26Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
