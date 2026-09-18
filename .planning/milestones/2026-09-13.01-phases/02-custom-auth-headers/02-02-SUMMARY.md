---
phase: 02-custom-auth-headers
plan: 02
subsystem: setup
tags: [go, mcp-registration, opencode, generic, custom-headers, json-marshaling]

# Dependency graph
requires:
  - phase: 02-custom-auth-headers
    provides: "HeaderSpec{Name, EnvVar}, Options.Headers, ErrHeaderUnsupported, sortedHeaders (02-01)"
provides:
  - "openCodeHeaderArgs(hs []HeaderSpec) []string in internal/setup/opencode.go — one sorted --header NAME={env:ENVVAR} pair per header, appended to the single opencode mcp add action's Args in every mode opencode supports"
  - "genericHeaders (named map[string]string) with an ordered MarshalJSON in internal/setup/generic.go — Authorization first, then extras sorted case-insensitively, every key/value passed through json.Marshal"
  - "generic's Plan() carries opts.Headers extras in the existing Headers map as bare \"${ENVVAR}\" references beside any bearer entry"
  - "TestOpenCodeHeaders, TestGenericHeaders, and TestNoSecretInArgs' positive control (want the env var NAME)"
affects: [02-03-header-flag-wiring, 02-04-docs]

# Actuals (#2632)
actuals:
  tokens: 5944
  tasks: 3
  commits: 2
  plan_head_before: fdb52f2b895f2aca379b55102b09a7888f7ca81e

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Per-runtime header rendering extended to opencode/generic: each authors its own dialect string in its own file (opencode: NAME={env:ENVVAR}; generic: \"NAME\": \"${ENVVAR}\") — no shared cross-runtime formatter, per D-04's explicit prohibition"
    - "Named map type + MarshalJSON to fix key order without hand-rolling escaping: genericHeaders wraps map[string]string and orders keys (Authorization first, then case-insensitive) while delegating every key/value to json.Marshal so HTML-safe escaping stays byte-for-byte identical to an anonymous map"

key-files:
  created: []
  modified:
    - internal/setup/opencode.go
    - internal/setup/opencode_test.go
    - internal/setup/generic.go
    - internal/setup/generic_test.go
    - internal/setup/plan_test.go

key-decisions:
  - "D-01/D-04/D-08 implemented exactly as locked for opencode: extra headers rendered as bare {env:ENVVAR} references in opencode's own KEY=VALUE dialect, sorted case-insensitively, appended after every shipped argument (and after the bearer arm's own auth-mode header) on the SAME single add action — never a second Action"
  - "D-04/D-05/D-08 implemented exactly as locked for generic: extras land in the EXISTING headers map beside any bearer Authorization entry, valued as bare ${ENVVAR} references; --token-file provenance stays bearer-only and unaffected by --header"
  - "genericHeaders' MarshalJSON is a thin key-ordering wrapper, not a hand-rolled encoder: it collects keys, orders them (Authorization first via strings.EqualFold, then sortedHeaders' case-insensitive order for the rest), then writes each key/value through json.Marshal into a bytes.Buffer — this is what keeps the --token-file document's \\u003c/\\u003e escaping byte-identical to the anonymous-map version"
  - "No shared cross-runtime header formatter was added (D-04's explicit prohibition) — openCodeHeaderArgs lives only in opencode.go; genericHeaders.MarshalJSON lives only in generic.go"

requirements-completed: [REQ-header-name-parameter, REQ-header-value-env-ref-only, REQ-header-bearer-unchanged]

coverage:
  - id: D1
    description: "opencode appends one sorted --header 'NAME={env:ENVVAR}' pair per header to its single add action in oauth/none/bearer, after every shipped argument (and the bearer arm's own auth header); oauth-client still declines via ErrAuthModeUnsupported regardless of headers; zero headers leave Args byte-identical to HEAD"
    requirement: "REQ-header-name-parameter"
    verification:
      - kind: unit
        ref: "internal/setup/opencode_test.go#TestOpenCodeHeaders (oauth, none, bearer, oauth-client-still-declines, case-insensitive-sort, input-order-unchanged, zero-header-control, command-rendering)"
        status: pass
      - kind: unit
        ref: "internal/setup/opencode_test.go#TestOpenCodePlan"
        status: pass
      - kind: unit
        ref: "internal/setup/opencode_test.go#TestOpenCodeBearerHeaderSyntax"
        status: pass
    human_judgment: false
  - id: D2
    description: "generic carries opts.Headers extras in the EXISTING headers JSON object beside any bearer entry, each valued as a bare \"${ENVVAR}\" reference, with the marshaled key order fixed to Authorization-first then case-insensitive extras — identical to the order argv shows for native runtimes"
    requirement: "REQ-header-name-parameter"
    verification:
      - kind: unit
        ref: "internal/setup/generic_test.go#TestGenericHeaders (oauth, none, bearer, case-insensitive-discriminator, token-file-provenance-unaffected, input-order-unchanged)"
        status: pass
    human_judgment: false
  - id: D3
    description: "generic's zero-header documents for oauth/none, bearer, and bearer+token-file stay byte-identical to the three literals captured live at HEAD before this task's edit (json.Marshal escaping preserved by genericHeaders.MarshalJSON)"
    requirement: "REQ-header-bearer-unchanged"
    verification:
      - kind: unit
        ref: "internal/setup/generic_test.go#TestGenericHeaders/zero-header-byte-identity"
        status: pass
      - kind: unit
        ref: "internal/setup/generic_test.go#TestGenericConfig"
        status: pass
      - kind: unit
        ref: "internal/setup/generic_test.go#TestGenericConfigCarriesNoSecret"
        status: pass
    human_judgment: false
  - id: D4
    description: "No header VALUE reaches Args or Config for any runtime x auth mode x header-presence combination, and every successful header-carrying Plan actually names the env var somewhere in Args or Config (positive control closing the vacuous-pass gap)"
    requirement: "REQ-header-value-env-ref-only"
    verification:
      - kind: unit
        ref: "internal/setup/plan_test.go#TestNoSecretInArgs (32 subtests, including the +header positive control)"
        status: pass
      - kind: other
        ref: "rg -n -e 'Getenv' internal/setup/generic.go (prints nothing); rg -c -F 'env.Getenv' internal/setup/opencode.go (prints 1, the pre-existing XDG_CONFIG_HOME read only)"
        status: pass
    human_judgment: false
  - id: D5
    description: "Repo-wide gate green: task (lint+test across every package), task license:check, zero go.mod/go.sum drift, internal/keylinks gate, shuffled internal/setup run, CLI consumer of generic's Config unaffected"
    verification:
      - kind: other
        ref: "task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1 && go test ./internal/setup/ -count=1 -shuffle=on && go test ./cmd/engram -run 'TestSetupGenericRowCarriesPortableConfig' -count=1"
        status: pass
    human_judgment: false

duration: ~35min
completed: 2026-09-13
status: complete
---

# Phase 2 Plan 2: opencode and generic header rendering Summary

**opencode renders `--header 'NAME={env:ENVVAR}'` pairs on its single `mcp add` action in every supported mode, and generic carries extras in its existing `headers` JSON object via a new `genericHeaders` named map type whose `MarshalJSON` orders `Authorization` first then extras case-insensitively — both extending 02-01's header vocabulary with zero-header outputs proven byte-identical to HEAD.**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-09-13T23:10:00Z (approx.)
- **Completed:** 2026-09-13T23:45:00Z (approx.)
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments
- `openCodeHeaderArgs(hs []HeaderSpec) []string` added to `internal/setup/opencode.go`, appended in both case arms (`oauth`/`none` and `bearer`) via `append(...existing Args..., openCodeHeaderArgs(opts.Headers)...)` — extras render after every shipped argument (and after the bearer arm's own auth-mode header), in opencode's own `NAME={env:ENVVAR}` dialect, sorted case-insensitively; `oauth-client` still declines via `ErrAuthModeUnsupported` regardless of headers.
- `genericHeaders` (a named `map[string]string`) added to `internal/setup/generic.go` with a `MarshalJSON` that orders `Authorization` first (via `strings.EqualFold`) then every other key via `sortedHeaders`' case-insensitive order, delegating every key and value to `json.Marshal` so the encoder's HTML-safe escaping (`<` -> `<`) stays byte-for-byte identical to what the prior anonymous map produced.
- `generic.Plan()` extended to carry `sortedHeaders(opts.Headers)` extras into the same `server.Headers` map beside any bearer `Authorization` entry, each valued `"${" + h.EnvVar + "}"`; `--token-file` provenance stays bearer-only, unaffected by `--header`.
- `TestOpenCodeHeaders` (opencode_test.go) and `TestGenericHeaders` (generic_test.go) added, each covering: exact rendering per mode, case-insensitive sort, input-slice-unchanged, zero-header byte-identity, and (opencode) `Command()` single-quoting; generic additionally covers raw-text key ordering and `--token-file` provenance non-interference.
- `TestNoSecretInArgs` (plan_test.go) gained its positive control: for every `+header` subtest, the joined Args+Config text must contain `LITELLM_KEY` — closing the vacuous-pass gap a header-dropping bug would otherwise slip through.

## Task Commits

Each task was committed atomically (TDD RED->GREEN observed for both tasks 1 and 2 before the corresponding commit):

1. **Task 1: opencode renders `--header NAME={env:ENVVAR}` pairs** - `40c20104` (feat)
2. **Task 2: generic carries extras in its existing `headers` object, Authorization-first** - `16e1582e` (feat)
3. **Task 3: Plan gate (full `task`, license, zero go.mod drift, key-links, shuffle)** - no commit needed; every gate link passed with zero fixups required (no formatter or license-fixer rewrite occurred).

_No separate plan-metadata commit is issued beyond the two above; this SUMMARY's own commit follows immediately._

## Files Created/Modified
- `internal/setup/opencode.go` - `openCodeHeaderArgs`, wired into both case arms
- `internal/setup/opencode_test.go` - new `TestOpenCodeHeaders`
- `internal/setup/generic.go` - `genericHeaders` named map type + `MarshalJSON`, `genericMCPServer.Headers` retyped, `Plan()` extended
- `internal/setup/generic_test.go` - new `TestGenericHeaders`
- `internal/setup/plan_test.go` - `TestNoSecretInArgs` positive control (`want the env var NAME`)

## Decisions Made
- Followed 02-CONTEXT.md's locked decisions D-01, D-04, D-05, D-08 exactly as written for both opencode and generic — no deviations from the decision set.
- `genericHeaders.MarshalJSON` reuses `sortedHeaders` (runtime.go) for the case-insensitive ordering of non-`Authorization` keys rather than a second hand-rolled comparator, keeping the ORDER rule in the one place it already lives.
- Confirmed the three zero-header generic Config literals live at HEAD (commit `fdb52f2b`) before editing `generic.go`, per the plan's explicit instruction — they matched the plan's stated literals exactly (including the `<`/`>` escaping of the `--token-file` form) and are now pinned as `TestGenericHeaders/zero-header-byte-identity`.

## Deviations from Plan

None — plan executed exactly as written. One acceptance-criterion observation is worth recording, not as a deviation but for the verifier's benefit:

### Observations (no fix required)

**1. `runtime.go`'s own doc comment collides with a literal acceptance grep**
- **Found during:** Task 1's acceptance-criteria pass
- **Observation:** The acceptance criterion `rg -n -e '=[{]env:|: [$][{]' internal/setup/runtime.go` is specified to print nothing, but it prints one line: `sortedHeaders`'s doc comment (added by plan 02-01, commit `3f70041b`, predates this plan) illustrates both dialects in prose — `"NAME={env:ENVVAR}" for opencode`. This is a documentation string inside a comment, not executable dialect-formatting code; `runtime.go` is not in this plan's `files_modified` and was not touched. Confirmed via `git blame` that the line originates from 02-01, not from this plan's work. No fix applied — flagging this as a pre-existing false-positive against the literal grep pattern, not a real cross-file dialect leak (the substantive guarantee — no dialect-FORMATTING code in the shared file — still holds; `sortedHeaders` only orders, never formats, exactly as its own doc states).
- **Files affected:** None (no file modified for this).
- **Verification:** `git blame -L 185,195 internal/setup/runtime.go` shows commit `3f70041b` (02-01) for every affected line.

---

**Total deviations:** 0 (one non-fix observation recorded above for verifier visibility).
**Impact on plan:** None. All substantive acceptance criteria and the full plan-level `<verification>` block pass.

## Issues Encountered
- One transient environment issue: `task` (the full lint+test gate) intermittently failed with `Error: parallel golangci-lint is running` due to concurrent `golangci-lint` invocations from unrelated sessions on the same machine contending for a shared cache lock. Resolved by retrying after a short backoff (no code change involved); the gate passed cleanly once contention cleared.

RED was observed before GREEN for both Task 1 (`TestOpenCodeHeaders` failing on the Args deep-equal against plan-02-01 HEAD's `opencode.go`) and Task 2 (`TestGenericHeaders` failing on the extras assertions and the `generic:*+header` positive control against plan-02-01 HEAD's `generic.go`, while the three zero-header literals already passed at HEAD, exactly as the plan predicted).

`TestRedEvidencePatchesAreLive` (`internal/store`) is registered by the orchestrator after the phase's last plan — not chased in this plan, consistent with 02-01's own note.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- opencode and generic now render the full header vocabulary (`Options.Headers`) alongside claude-code (02-01) and codex's explicit decline (02-01) — every runtime in `Runtimes` has a defined, tested header behavior.
- Plan 02-03 can now wire the CLI-boundary `--header` flag and its validation (D-02/D-03) against a fully-implemented `internal/setup` package.
- No blockers. `go.mod`/`go.sum` unchanged; `internal/setup` remains a stdlib-only leaf (`bytes`/`strings` are stdlib, already used elsewhere in the package).

## Self-Check: PASSED

All 5 modified files plus this SUMMARY.md verified present on disk (`[ -f ]`). Both task commits (`40c20104`, `16e1582e`) verified present via `git log --oneline --all`. All task-level `<acceptance_criteria>` re-verified passing (see coverage block above). Plan-level `<verification>` block re-run: `go test ./internal/setup/ -run '^(TestOpenCodeHeaders|TestGenericHeaders|TestNoSecretInArgs)$' -count=1 -v` all PASS; three zero-header generic literals confirmed `==` at HEAD before and after the edit; `task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1` all exit 0; `git diff --stat` touches exactly the 5 `files_modified` entries.

---
*Phase: 02-custom-auth-headers*
*Completed: 2026-09-13*
