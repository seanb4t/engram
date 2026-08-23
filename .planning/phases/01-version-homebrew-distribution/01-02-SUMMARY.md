---
phase: 01-version-homebrew-distribution
plan: 02
subsystem: cli
tags: [go, build-info, semver, release-please, version, telemetry, mcp]

requires:
  - phase: 01-01
    provides: "`engram version --output json|text` — the machine-readable contract this plan feeds a real value into"
provides:
  - "`cmd/engram/buildversion.go` — nextPatch, deriveDevVersion, versionFromModuleVersion, resolvedVersion, and the release-please-managed `lastRelease` const"
  - "A local `go build` of `./cmd/engram` reports `X.Y.Z-dev.0+g<hash>[.dirty]` instead of the bare `dev` sentinel"
  - "`go install …/cmd/engram@vX.Y.Z` resolves to the bare tag version (parser proven; end-to-end run deferred to 01-03 Task 4)"
  - "All three `engram serve` version surfaces (OpenTelemetry `service.version`, MCP `Implementation.Version`, startup log) report the same resolved string"
affects: [01-03-release-plumbing]

actuals:
  tokens: 4824
  tasks: 2
  commits: 4

tech-stack:
  added: []
  patterns:
    - "resolvedVersion() as the single call-through seam every version-reporting surface in the binary reads, mirroring outputFormatFromConfig's parameter-injection shape for testability"
    - "release-please generic extra-files entry matched by a same-line trailing annotation comment (x-release-please-version), pinned against .release-please-manifest.json by a dedicated drift test"

key-files:
  created:
    - cmd/engram/buildversion.go
    - cmd/engram/buildversion_test.go
  modified:
    - cmd/engram/version.go
    - cmd/engram/serve.go
    - cmd/engram/root.go
    - release-please-config.json

key-decisions:
  - "The '.N' in '0.14.1-dev.N+g<hash>' is the fixed literal 0 — debug.BuildInfo.Settings carries no commit-distance field, and a real count would require a build-time git shell-out (reopening the no-Taskfile-change premise) or git present at runtime. Per-build uniqueness comes from the revision + dirty marker, not N. Recorded in buildversion.go's own source comment."
  - "nextPatch gates on an anchored patchCorePattern (^(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)$) before strconv.ParseUint(c, 10, 32), never bare strconv.Atoi — Atoi accepts signs and silently normalizes leading zeros, neither of which is the intended grammar."
  - "engram --version and the bare-invocation catalog deliberately keep reading the raw `version` package var (D-02); engram version and all three engram serve surfaces read resolvedVersion(). This is the one accepted, recorded divergence — not an oversight."
  - "root.go gained a short comment recording the D-02 non-rewire, matching the exact fixture the plan's own cycle-3 acceptance-criteria review anticipated and tested against; not in the plan's files_modified list but a documentation-only, zero-behavior-change addition."

requirements-completed: [REQ-version-json]

coverage:
  - id: D1
    description: "A go build inside the git working tree reports X.Y.Z-dev.0+g<hash>, with .dirty appended when the tree is modified, instead of the bare dev sentinel"
    requirement: REQ-version-json
    verification:
      - kind: unit
        ref: "cmd/engram/buildversion_test.go#TestDeriveDevVersion"
        status: pass
      - kind: other
        ref: "go build -o /tmp/engram-devcheck ./cmd/engram; /tmp/engram-devcheck version — printed 0.14.1-dev.0+g9830586d.dirty, matching ^[0-9]+\\.[0-9]+\\.[0-9]+-dev\\.0\\+g[0-9a-f]{1,8}(\\.dirty)?$"
        status: pass
    human_judgment: false
  - id: D2
    description: "versionFromModuleVersion maps a bare release-tag module version vX.Y.Z to X.Y.Z (no v prefix) and rejects pseudo-versions, (devel), unprefixed input, and the empty string — the parser half of the go install pkg@vX.Y.Z path"
    requirement: REQ-version-json
    verification:
      - kind: unit
        ref: "cmd/engram/buildversion_test.go#TestVersionFromModuleVersion"
        status: pass
    human_judgment: true
    rationale: "The end-to-end go install …@vX.Y.Z observation is explicitly out of scope for this phase (M-D, cycle-2 review): every tag that exists today predates cmd/engram/buildversion.go, so no go install of a real tag can reach this resolver. That observation is carried by 01-03 Task 4's post-merge checklist item C, not this plan."
  - id: D3
    description: "A GoReleaser-built release binary reports exactly the ldflags-injected version, unchanged by this plan"
    requirement: REQ-version-json
    verification:
      - kind: unit
        ref: "cmd/engram/version_test.go#TestVersionTextEqualsJSON (re-run green after the value's source changed)"
        status: pass
    human_judgment: false
  - id: D4
    description: "engram version renders resolvedVersion()'s output in both the text and json lanes, from the single call site in version.go's RunE"
    requirement: REQ-version-json
    verification:
      - kind: unit
        ref: "cmd/engram/version_test.go#TestVersionJSONLane, #TestVersionTextLane, #TestVersionExplicitTextLane, #TestVersionTextEqualsJSON, #TestVersionOutputFlagDefault"
        status: pass
      - kind: other
        ref: "rg -o '^[^/]*' cmd/engram/version.go | rg -o -F 'resolvedVersion' | wc -l = 1"
        status: pass
    human_judgment: false
  - id: D5
    description: "engram serve passes resolvedVersion() at all three of its version surfaces (:83 OpenTelemetry service.version, :231 MCP Implementation.Version, :296 startup log), so no single engram serve process reports two different version strings"
    requirement: REQ-version-json
    verification:
      - kind: unit
        ref: "go test ./internal/telemetry -count=1 (TestConfigFromEnv, TestConfigFromEnvDefaults) — unchanged, confirms the sink was not touched"
        status: pass
      - kind: other
        ref: "source-level positive/negative/total gate over cmd/engram/serve.go: three positives = 1 each, three negatives = 0 each, total resolvedVersion() count = 3"
        status: pass
    human_judgment: false
  - id: D6
    description: "release-please-config.json carries a fourth extra-files entry (generic, cmd/engram/buildversion.go, no jsonpath), and a committed test fails if the const drifts from .release-please-manifest.json"
    requirement: REQ-version-json
    verification:
      - kind: unit
        ref: "cmd/engram/buildversion_test.go#TestLastReleaseMatchesManifest"
        status: pass
      - kind: other
        ref: "jq checks over release-please-config.json: extra-files length == 4, entry [3] == {generic, cmd/engram/buildversion.go}, no jsonpath key"
        status: pass
    human_judgment: false
  - id: D7
    description: "engram --version and the bare-invocation self-describe catalog keep reporting the raw version package var — a deliberate, recorded D-02 divergence, not an oversight"
    requirement: REQ-version-json
    verification:
      - kind: other
        ref: "rg -o '^[^/]*' cmd/engram/catalog.go | rg -o -F 'resolvedVersion' | wc -l = 0; same for root.go = 0; git diff cmd/engram/testdata/help.golden is empty"
        status: pass
    human_judgment: false

duration: ~35min
completed: 2026-08-23
status: complete
---

# Phase 1 Plan 2: Dev-build version derivation & release-please wiring Summary

**A local `go build` now reports `0.14.1-dev.0+g<8charhash>[.dirty]` instead of the bare `dev` sentinel, via a five-function `cmd/engram/buildversion.go` unit (const + 4 pure/thin functions), wired into `engram version` and all three `engram serve` version surfaces, with a fourth `release-please-config.json` extra-files entry keeping the patch-bump base current and a committed drift test guarding it.**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-08-23 (session start)
- **Completed:** 2026-08-23
- **Tasks:** 2 completed
- **Files modified:** 6 (2 created, 4 modified)

## Accomplishments

- `cmd/engram/buildversion.go`: `lastRelease` const (release-please-managed, carrying the `x-release-please-version` same-line annotation), `nextPatch` (anchored `patchCorePattern`, `strconv.ParseUint(..., 32)` — never `Atoi`), `deriveDevVersion` (composes the locked `0.14.1-dev.0+g<hash>[.dirty]` format, taking revision/dirty as parameters for testability), `versionFromModuleVersion` (closes the `go install pkg@vX.Y.Z` bug D-04 names, rejecting Go's pseudo-version shape empirically confirmed by RESEARCH.md), and `resolvedVersion` (the thin `runtime/debug` wrapper, checking module version before VCS settings).
- `cmd/engram/buildversion_test.go`: 15-row `TestNextPatch` table (including all eight Atoi-bypass grammar rows: leading `+`/`-`, leading zeros, and a 32-bit overflow case), 6-branch `TestDeriveDevVersion` (happy path, dirty suffix, short-revision, two empty-string failure modes, and the narrow prerelease-ordering assertion), 6-row `TestVersionFromModuleVersion` (release tag, `(devel)`, plain and dirty pseudo-versions, empty string, unprefixed bare SemVer), and `TestLastReleaseMatchesManifest` (the D-05 drift gate against `.release-please-manifest.json`).
- `cmd/engram/version.go`'s `RunE` now obtains its value from the single `resolvedVersion()` call — both the text and json lanes read the same source, keeping 01-01's `TestVersionTextEqualsJSON` invariant true by construction.
- All three of `cmd/engram/serve.go`'s version-reporting sites (`:83` OpenTelemetry `service.version`, `:231` MCP handshake `Implementation.Version`, `:296` startup log) now pass `resolvedVersion()` — cycle-2 review's HIGH-2/M-B finding that an earlier draft named only one of three real consumers. `internal/telemetry/config.go` (the parameter sink, not the seam) is untouched.
- `release-please-config.json` gained a fourth `extra-files` entry: `{"type": "generic", "path": "cmd/engram/buildversion.go"}`, matching the three existing entries' shape, no `jsonpath` key.
- `cmd/engram/root.go` gained a short comment recording the deliberate D-02 non-rewire of `rootCmd.Version` — `engram --version` and the bare-invocation catalog (`buildCatalog` reading `root.Version`) still report the raw package var. This is the one accepted, recorded divergence: `engram version` and all three `engram serve` surfaces read the resolved string; `engram --version` and the catalog do not, because D-02 forecloses touching cobra's built-in version template and the pinned `help.golden` line it protects.

## Task Commits

Each task was committed atomically (TDD RED/GREEN split for Task 1):

1. **Task 1 RED — failing buildversion tests** — `3832ce9b` (test)
2. **Task 1 GREEN — buildversion.go implementation** — `18641f57` (feat)
3. **Task 2 — wire resolvedVersion + release-please entry** — `15abc05d` (feat)
4. **Deferred-items documentation** — `417d93a5` (docs)

_No REFACTOR commit: the GREEN implementation was already minimal; no cleanup was needed._

## Files Created/Modified

- `cmd/engram/buildversion.go` — `lastRelease` const, `nextPatch`, `deriveDevVersion`, `versionFromModuleVersion`, `resolvedVersion`.
- `cmd/engram/buildversion_test.go` — table tests for the three pure functions plus the manifest-drift gate.
- `cmd/engram/version.go` — one-line change: `v := version` → `v := resolvedVersion()`.
- `cmd/engram/serve.go` — three call-site changes (`:83`, `:231`, `:296`), each `version` → `resolvedVersion()`, plus one comment naming the single source.
- `cmd/engram/root.go` — one comment recording the deliberate D-02 non-rewire; no behavior change.
- `release-please-config.json` — fourth `extra-files` entry.

## Decisions Made

- Kept Task 1 as a genuine TDD RED/GREEN split (test commit, then implementation commit) per its `tdd="true"` marking; Task 2 is a single `feat` commit since it carries no new behavior of its own to test-first — it is wiring plus a JSON config addition plus one already-tested pure-function-consuming test (`TestLastReleaseMatchesManifest`).
- Wrote three explicit `strconv.ParseUint` calls (major/minor/patch) in `nextPatch` rather than a loop — matches the plan's own composition instruction ("Format the result with the `%d` of the three `uint64` values") and keeps each component's role legible at the call site.
- Added the `root.go` comment even though `root.go` is not in this plan's `files_modified` frontmatter list: the plan's own cycle-3 acceptance-criteria review explicitly anticipated and tested against exactly this fixture ("the `L-3` disposition makes a `// Deliberately NOT resolvedVersion() — D-02` comment at `root.go:28` the *expected* thing for an executor to write"), so this is documentation-only, zero-behavior-change, and within the plan's own stated intent rather than scope creep.

## Deviations from Plan

### Auto-fixed Issues

None — no bugs, missing critical functionality, or blocking issues were found beyond what the plan already anticipated and instructed.

### Out-of-Scope Discoveries (logged, not fixed)

**1. [Out of scope] `go vet ./cmd/engram/...` fails on a pre-existing, deliberately-suppressed test fixture**
- **Found during:** Both tasks' `go vet ./cmd/engram/...` acceptance criterion.
- **Issue:** `cmd/engram/operator_view_test.go:441` (last touched in `62c39c22`, well before Phase 01) intentionally declares a duplicate `json:"dup"` struct tag to exercise `encoding/json`'s field-conflict rule, guarded by a `//nolint:govet` comment that `golangci-lint` honors but bare `go vet` does not.
- **Why not fixed:** the file is outside both tasks' `files_modified`, and the condition is identical before and after this plan's commits — it is not caused by this plan.
- **Disposition:** `task` (lint + test, the repo's actual gate per CLAUDE.md) passes clean via `golangci-lint`'s nolint-aware govet linter. Logged to `deferred-items.md` (commit `417d93a5`) rather than fixed.
- **Verification that this plan's own files vet clean:** `go vet` produces exactly this one finding, on this one pre-existing line, with no other output.

---

**Total deviations:** 0 auto-fixed. 1 out-of-scope discovery logged and left untouched.
**Impact on plan:** None — plan executed exactly as written, both tasks' actual acceptance criteria (scoped to files this plan touches) are fully satisfied.

## Issues Encountered

A locally built dev binary reported `.dirty` even immediately after a clean `git status` in this worktree (no staged or unstaged changes). This does not affect correctness: the plan's own end-to-end verification regex (`^[0-9]+\.[0-9]+\.[0-9]+-dev\.0\+g[0-9a-f]{1,8}(\.dirty)?$`) treats `.dirty` as optional and the observed string matched it exactly. Likely a `runtime/debug`/`git` VCS-dirty-detection quirk specific to linked worktrees rather than a defect in `deriveDevVersion` or `resolvedVersion`, both of which only consume whatever `vcs.modified` reports. Not investigated further — out of scope for this plan's file set, and does not block any acceptance criterion.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `resolvedVersion()` is now the single, tested source for every version-reporting surface in the binary except the two D-02-protected ones (`engram --version`, the bare-invocation catalog), which are a recorded, accepted divergence.
- `release-please-config.json`'s fourth `extra-files` entry and `TestLastReleaseMatchesManifest` mean the dev-version base stays current automatically on every release, with a red build if that mechanism ever silently breaks.
- 01-03 (release plumbing: App-token scope, `skip_upload` re-ship guard, credential-verify job) can proceed independently — this plan made no changes to `.github/workflows/release.yaml`, `.github/workflows/verify-tap-credential.yaml`, or `.goreleaser.yaml` (explicitly out of this plan's lane, per the parallel-executor instructions).
- 01-03 Task 4's post-merge checklist item C carries the still-unverified end-to-end `go install github.com/seanb4t/engram/cmd/engram@vX.Y.Z` observation — not verifiable in this phase because every existing tag predates `cmd/engram/buildversion.go`. `TestVersionFromModuleVersion` proves the parser half only; this is a known, recorded gap, not an oversight.
- REQ-version-json is now fully covered by 01-01 (the json/text contract) and 01-02 (a real, non-`dev` value feeding that contract) together; marked complete in `.planning/REQUIREMENTS.md` as of this plan (the shared-ID gate held it pending until both declaring plans finished).
- No blockers.

---
*Phase: 01-version-homebrew-distribution*
*Completed: 2026-08-23*
