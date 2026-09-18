---
phase: 02-custom-auth-headers
plan: 01
subsystem: setup
tags: [go, mcp-registration, claude-code, codex, custom-headers, cli]

# Dependency graph
requires:
  - phase: 01-executor-correctness-man-pages
    provides: internal/keylinks gate, red-evidence registration discipline, --force-isolation none ordering
provides:
  - "HeaderSpec{Name, EnvVar} type and Options.Headers []HeaderSpec field in internal/setup/runtime.go"
  - "ErrHeaderUnsupported sentinel, distinct from ErrAuthModeUnsupported (D-10)"
  - "sortedHeaders(hs) — clone-and-sort helper, case-insensitive by Name (D-08), never formats"
  - "claude-code renders one sorted --header 'NAME: ${ENVVAR}' pair per header, appended to the single claude mcp add action in every auth arm (D-01, D-04, D-08)"
  - "codex declines any header before its auth switch with an ErrHeaderUnsupported-wrapped failed-row reason naming the header(s), the capability gap, and the remedy (D-09, D-10)"
  - "TestNoSecretInArgs extended to 32 subtests (4 runtimes x 4 auth modes x {no header, one header}) proving no header VALUE ever reaches Args or Config"
affects: [02-02-opencode-and-generic-headers, 02-03-header-flag-wiring, 02-04-docs]

# Actuals (#2632)
actuals:
  tokens: 6519
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Per-runtime header rendering: each runtime file authors its own header dialect string (colon-space for claude-code); no shared cross-runtime formatter (avoids the opencode colon-space regression)"
    - "sortedHeaders returns a clone, never mutates the caller's slice — the ordering helper is pure"
    - "Unsupported-capability decline: a runtime's Plan() returns the zero Plan plus a wrapped sentinel error at the top of the method, before any other work — apply.go's existing Plan()-error-to-failed-row conversion is the free mechanism, no changes needed there"

key-files:
  created: []
  modified:
    - internal/setup/runtime.go
    - internal/setup/claudecode.go
    - internal/setup/codex.go
    - internal/setup/plan_test.go
    - internal/setup/claudecode_test.go
    - internal/setup/codex_test.go

key-decisions:
  - "D-01/D-04/D-08 implemented exactly as locked: extra headers are additional, orthogonal to --auth, rendered as bare ${ENVVAR} references in claude-code's own colon-space syntax, sorted case-insensitively by Name, auth-mode header first"
  - "D-09/D-10 implemented exactly as locked: codex's header guard is the FIRST statement of Plan() — before env.HomeDir() and before the opts.Auth switch — returning the zero Plan and an ErrHeaderUnsupported-wrapped reason naming every header, the capability gap, and the remedy"
  - "No shared cross-runtime header formatter was added (D-04's explicit prohibition) — claudeCodeHeaderArgs lives only in claudecode.go"

requirements-completed: [REQ-header-name-parameter, REQ-header-value-env-ref-only, REQ-header-codex-declined, REQ-header-bearer-unchanged]

coverage:
  - id: D1
    description: "Options.Headers []HeaderSpec, ErrHeaderUnsupported sentinel, and sortedHeaders clone-and-sort helper exist in internal/setup/runtime.go; internal/setup never dereferences a header's EnvVar"
    requirement: "REQ-header-value-env-ref-only"
    verification:
      - kind: unit
        ref: "internal/setup/claudecode_test.go#TestClaudeCodeHeaders/case-insensitive-sort"
        status: pass
      - kind: unit
        ref: "internal/setup/claudecode_test.go#TestClaudeCodeHeaders/zero-header-nil"
        status: pass
      - kind: other
        ref: "rg -n -e 'Getenv' internal/setup/runtime.go internal/setup/claudecode.go internal/setup/codex.go (prints nothing)"
        status: pass
    human_judgment: false
  - id: D2
    description: "claude-code appends one sorted --header 'NAME: ${ENVVAR}' pair per header to the single claude mcp add action, in every auth mode, with no-header output byte-identical to HEAD"
    requirement: "REQ-header-bearer-unchanged"
    verification:
      - kind: unit
        ref: "internal/setup/claudecode_test.go#TestClaudeCodeHeaders"
        status: pass
      - kind: unit
        ref: "internal/setup/claudecode_test.go#TestClaudeCodePlan"
        status: pass
    human_judgment: false
  - id: D3
    description: "Codex declines any header set, in every auth mode, before resolving home or dispatching on auth mode, with the exact reason naming the header(s), the gap, and the remedy, distinct from ErrAuthModeUnsupported"
    requirement: "REQ-header-codex-declined"
    verification:
      - kind: unit
        ref: "internal/setup/codex_test.go#TestCodexDeclinesHeaders"
        status: pass
    human_judgment: false
  - id: D4
    description: "No header VALUE reaches Args or Config for any runtime x auth mode x header-presence combination (negative-space secret proof, second sentinel)"
    requirement: "REQ-header-value-env-ref-only"
    verification:
      - kind: unit
        ref: "internal/setup/plan_test.go#TestNoSecretInArgs (32 subtests)"
        status: pass
    human_judgment: false
  - id: D5
    description: "Repo-wide gate green: task (lint+test across every package), task license:check, zero go.mod/go.sum drift, internal/keylinks gate, shuffled internal/setup run"
    verification:
      - kind: other
        ref: "task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1 && go test ./internal/setup/ -count=1 -shuffle=on"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-09-13
status: complete
---

# Phase 2 Plan 1: End-to-end header vocabulary — Options.Headers, claude-code rendering, codex decline Summary

**`setup.Options.Headers []HeaderSpec` lands with a `sortedHeaders` ordering helper, claude-code renders each as a sorted `--header 'NAME: ${ENVVAR}'` pair on its single `mcp add` action, and codex declines any header up front via the new `ErrHeaderUnsupported` sentinel — never dereferencing a header's environment variable anywhere in the package.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-09-13T22:40:00Z (approx.)
- **Completed:** 2026-09-13T23:00:00Z (approx.)
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments
- `HeaderSpec{Name, EnvVar string}`, `Options.Headers []HeaderSpec`, `ErrHeaderUnsupported`, and `sortedHeaders(hs []HeaderSpec) []HeaderSpec` added to `internal/setup/runtime.go` — additive, no environment read of any header variable anywhere in the package.
- claude-code (`internal/setup/claudecode.go`) appends one `--header "NAME: ${ENVVAR}"` pair per sorted header to the SAME `claude mcp add` action, in every auth arm (`oauth`, `none`, `oauth-client`, `bearer`), after every shipped argument and after the bearer arm's own `Authorization: Bearer ${ENGRAM_TOKEN}` pair. Zero headers leave every mode's Args byte-identical to HEAD.
- codex (`internal/setup/codex.go`) declines any `opts.Headers` entry as the FIRST statement of `Plan()` — before `env.HomeDir()` and before the auth-mode switch — returning the zero `Plan` and an error satisfying `errors.Is(err, ErrHeaderUnsupported)` (never `ErrAuthModeUnsupported`), with the exact reason `codex: custom header(s) <names>: codex mcp add exposes only --bearer-token-env-var (no custom header flag); drop --header or exclude codex via --runtime: setup: custom header is not supported by this runtime`.
- `TestNoSecretInArgs` extended to 32 subtests (4 runtimes x 4 auth modes x {no header, one header}) with a second sentinel (`HEADER-SECRET-VALUE-MUST-NEVER-APPEAR-7c1d4b`, exported through the fake `Environment.Getenv` for `LITELLM_KEY`), proving the header vocabulary carries a NAME, never a VALUE, for every runtime.

## Task Commits

Each task was committed atomically (TDD RED->GREEN observed for both tasks 1 and 2 before the corresponding commit):

1. **Task 1: End-to-end header vocabulary** - `3f70041b` (feat)
2. **Task 2: Codex declines any header before its auth switch** - `5d0d89dc` (feat)
3. **Task 3: Plan gate (full `task`, license, zero go.mod drift, key-links, shuffle)** - `b71c6df6` (chore — two golangci-lint/staticcheck style fixups in test files; no behavior change)

_No separate plan-metadata commit is issued beyond the three above; this SUMMARY's own commit follows immediately._

## Files Created/Modified
- `internal/setup/runtime.go` - `HeaderSpec`, `Options.Headers`, `ErrHeaderUnsupported`, `sortedHeaders`
- `internal/setup/claudecode.go` - `claudeCodeHeaderArgs`, wired into all three case arms
- `internal/setup/codex.go` - header-decline guard at the top of `Plan()`
- `internal/setup/plan_test.go` - `TestNoSecretInArgs` extended with the second sentinel and header-presence dimension
- `internal/setup/claudecode_test.go` - new `TestClaudeCodeHeaders` (ordering, case-insensitive sort, zero-header control, display quoting)
- `internal/setup/codex_test.go` - new `TestCodexDeclinesHeaders` (every mode, two-header sort order, HomeDir-not-called proof, zero-header control)

## Decisions Made
- Followed 02-CONTEXT.md's locked decisions D-01, D-04, D-06, D-08, D-09, D-10 exactly as written — no deviations from the decision set.
- `sortedHeaders` lives in `runtime.go` (shared ordering, no formatting) while `claudeCodeHeaderArgs` (the dialect string) lives only in `claudecode.go`, per D-04's explicit no-shared-formatter prohibition.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed two golangci-lint (staticcheck) style findings in test code**
- **Found during:** Task 3 (repo-wide gate)
- **Issue:** `var idxA, idxB int = -1, -1` in `claudecode_test.go` (redundant type, QF1011) and `if !(withHeader && rt.Name() == "codex")` in `plan_test.go` (De Morgan simplification, QF1001) — both introduced by this plan's own test code in Tasks 1/2, caught only when the full `task` lint gate ran in Task 3.
- **Fix:** `idxA, idxB := -1, -1`; `if !withHeader || rt.Name() != "codex"`. No behavior change — both are pure style/readability transforms staticcheck can prove equivalent.
- **Files modified:** `internal/setup/claudecode_test.go`, `internal/setup/plan_test.go`
- **Verification:** `task` (lint+test, all packages) green after the fix; `go test ./internal/setup/ -count=1` and `-shuffle=on` both green.
- **Committed in:** `b71c6df6` (Task 3's own `chore(setup)` commit, exactly as the plan anticipated for a gate-fixup)

---

**Total deviations:** 1 auto-fixed (1 bug/lint fix, Rule 1)
**Impact on plan:** Purely cosmetic; caught and fixed within Task 3 as the plan's own action anticipated. No scope creep.

## Issues Encountered
None. RED was observed before GREEN for both Task 1 (`Headers`/`HeaderSpec` undefined, confirmed via `go test ./internal/setup/ -count=1` failing to compile) and Task 2 (`TestCodexDeclinesHeaders` failing with `err = <nil>` for every mode before the guard existed).

`TestRedEvidencePatchesAreLive` (`internal/store`) was GREEN throughout this plan (phase 01's patches already registered), consistent with the plan's own note that this plan's tests are registered by the orchestrator after the phase's last plan — not chased.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The shared header vocabulary (`HeaderSpec`, `Options.Headers`, `sortedHeaders`, `ErrHeaderUnsupported`) is in place for plan 02-02 to extend to opencode and generic.
- The `ErrHeaderUnsupported` sentinel is ready for plan 02-03's CLI-boundary wiring and Phase 4's drift comparison to distinguish header gaps from auth-mode gaps.
- No blockers. `go.mod`/`go.sum` unchanged; `internal/setup` remains a stdlib-only leaf.

---
*Phase: 02-custom-auth-headers*
*Completed: 2026-09-13*
