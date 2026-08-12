---
phase: 01-interface-enforceability
plan: 07
subsystem: cli
tags: [go, cobra, koanf, exit-codes, connect-rpc, config]

# Dependency graph
requires:
  - phase: 01-interface-enforceability plan 03
    provides: "the five client.* koanf registry rows (server_url, token_file, output, insecure, timeout), ClientConfig, and ValidateClient — deliberately separate from Config.Validate"
  - phase: 01-interface-enforceability plan 06
    provides: "the search/malformed-client-timeout-env deferred baseline row this plan closes, and the D-05 --timeout=0-rejected precedent already applied to migrate's own --timeout"
provides:
  - "exitTimeout = 6, a new dedicated exit code for a client-side request deadline, distinct from exitUnavailable (5) by explicit test assertion"
  - "engram catalog advertising seven exit codes (0-6), built from the constants, TestCatalogExitCodesMatchMapper unedited"
  - "clientFromFlags as the single config.Load(cmd.Flags()) + config.ValidateClient call site for every client verb (search/list/store), returning the resolved output format and parsed request deadline alongside the client"
  - "--timeout registered on every client verb (client.timeout, default 30s, 0/negative/malformed/empty rejected before any dial)"
  - "a named, per-file exception in TestClientFilesImportBoundary: only client_common.go may import internal/config"
affects: [01-08 (wires the returned time.Duration into the RPC calls via context.WithTimeout), 01-09 (guides/upgrade.md: new exit code 6, new --timeout flag, retired resolvers)]

# Actuals (#2632)
actuals:
  tokens: 12468
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "clientFromFlags as the one config-loading call site per client verb, returning (client, outputFormat, time.Duration, error) — every setting a client command needs comes from one config.Load, not a resolver per setting"
    - "outputFormatFromConfig replaces resolveOutputFormat: value validation moved entirely into config.ValidateClient, so the mapping function's default branch is unreachable-by-construction rather than a second rejection site"
    - "per-file import-boundary exception (TestClientFilesImportBoundary): a test asserting a package-wide invariant restructured to track imports per-file so ONE named file can carry a documented, positive-controlled exception without weakening the check for every other file"

key-files:
  created: []
  modified:
    - cmd/engram/client_common.go
    - cmd/engram/client_common_test.go
    - cmd/engram/catalog.go
    - cmd/engram/catalog_test.go
    - cmd/engram/exitcode_baseline_test.go
    - cmd/engram/client_search.go
    - cmd/engram/client_list.go
    - cmd/engram/client_store.go
    - cmd/engram/client_store_test.go
    - cmd/engram/clienttest_test.go

key-decisions:
  - "D-01 (pre-approved checkpoint): exitAuth=3 survives the unification unchanged — already distinct, already pinned, already advertised."
  - "D-06 (pre-approved checkpoint): a client-side timeout gets a NEW dedicated exit code 6 rather than reusing 5, so a caller can distinguish 'raise --timeout' from 'check the server is up'."
  - "clientFromFlags's signature widened to (client, outputFormat, time.Duration, error) — not just the plan-mandated duration — so output-format resolution also comes from the single config.Load call instead of a second, duplicate Load happening at each of the three call sites."
  - "TestClientFilesImportBoundary (a locked D-04-labeled gate from plan 01-01, predating this phase's own D-04 client-config-unification decision) denylisted internal/config in every client_*.go file, which directly conflicted with this plan's mandate that clientFromFlags call config.Load/config.ValidateClient. Resolved by restructuring the test to track imports per-file and grant client_common.go ONE named, positive-controlled exception — every other client_*.go file (client_search.go, client_list.go, client_store.go) remains denied, since they only ever reach configuration indirectly through clientFromFlags's return values."
  - "TestExitCodeBaselineClaims's own doc comment already promised introduced rows are 'exempt from the distinct/identical rules below,' but the implementation never actually skipped the after==before check for them — dormant because no row had ever set introduced:true before this plan's three new rows. Fixed by returning early on c.introduced, matching the test's own documented contract (Rule 1 — pre-existing bug, exposed by this plan's own new coverage, not introduced by it)."
  - "resetClientFlags (clienttest_test.go) now also calls resetEveryCommandFlagState(t, rootCmd), both immediately and via t.Cleanup. Deleting the four package-level client flag vars (clientServerURL/clientTokenFile/clientInsecure/clientOutput) removed an accidental safety net several existing tests were relying on without realizing it: those vars were zeroed via t.Cleanup independent of whether pflag's own Changed latch was ever cleared by resetCommandFlagState. TestInsecureIsNotSetByEnvironment failed full-suite-only (individually green) once that net was gone; folding the broader reset into the dozens of existing resetClientFlags(t) call sites fixes it without auditing every runClient call site individually."

requirements-completed: [REQ-cli-request-timeout, REQ-client-config-unified, REQ-exit-code-unified]

coverage:
  - id: D1
    description: "exitTimeout=6 exists, is produced only by connect.CodeDeadlineExceeded, and is asserted DISTINCT from exitUnavailable (5) by explicit inequality — not merely set membership"
    requirement: "REQ-exit-code-unified"
    verification:
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestExitCodeTimeoutDistinctFromUnavailable"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestExitCodeForConnectErrTable"
        status: pass
    human_judgment: false
  - id: D2
    description: "engram catalog advertises exactly seven exit codes (0-6), built from the constants; TestCatalogExitCodesMatchMapper (the anti-drift gate) required no edit"
    requirement: "REQ-exit-code-unified"
    verification:
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestCatalogListsEveryExitCode"
        status: pass
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestCatalogExitCodesMatchMapper"
        status: pass
    human_judgment: false
  - id: D3
    description: "clientFromFlags is the single shared constructor resolving every client setting through the client.* koanf registry via one config.Load(cmd.Flags()) call, validated by config.ValidateClient before any dial; resolveServerURL and resolveOutputFormat no longer exist"
    requirement: "REQ-client-config-unified"
    verification:
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestClientConfigResolution"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestOutputFormatFromConfig"
        status: pass
    human_judgment: false
  - id: D4
    description: "--timeout exists on every client verb, defaults to 30s, and rejects 0/negative/malformed/empty before any dial — including alongside an unreachable --server (validation precedes the dial)"
    requirement: "REQ-cli-request-timeout"
    verification:
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestClientConfigResolution (the --timeout subtests and the 'bad --timeout together with an unreachable --server' subtest)"
        status: pass
      - kind: unit
        ref: "cmd/engram/exitcode_baseline_test.go#TestExitCodeBaseline (search/timeout-zero, search/timeout-malformed, search/timeout-zero-beats-unreachable)"
        status: pass
    human_judgment: false
  - id: D5
    description: "the resolveToken credential path survives unchanged (file-read-and-trim body), sourced from cfg.Client.TokenFile; no client.token registry key exists and the credential never reaches argv"
    requirement: "REQ-client-config-unified"
    verification:
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestTokenFileTrailingNewlineTrimmed"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestTokenNeverAppearsInOutput"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestClientConfigResolution (token-file subtests)"
        status: pass
    human_judgment: false

duration: ~25min
completed: 2026-08-04
status: complete
---

# Phase 1 Plan 7: Close the Exit-Code Taxonomy and Retire Hand-Rolled Client Resolvers Summary

**`exitTimeout=6` distinguishes a client-side deadline from an unreachable server, and every client setting (`--server`, `--token-file`, `--output`, `--insecure`, the new `--timeout`) now resolves through one `config.Load` + `config.ValidateClient` call in `clientFromFlags` instead of four hand-rolled resolvers.**

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-08-04
- **Tasks:** 2 (plus two pre-approved `checkpoint:decision` tasks, D-01 and D-06, recorded as accepted with no interactive stop)
- **Files modified:** 10

## Accomplishments

- Added `exitTimeout = 6` and split `exitCodeForConnectErr`'s mapper arm: `connect.CodeDeadlineExceeded` now returns `exitTimeout`, while `CodeCanceled` stays with `CodeUnavailable` on 5 (a caller-initiated cancellation is not a server that failed to answer). `TestExitCodeTimeoutDistinctFromUnavailable` asserts 5 and 6 by explicit inequality, per memory `667p88n2be`'s precedent that set-membership-only assertions can pass on a silent collapse.
- `engram catalog`'s `doc.ExitCodes` grew a seventh entry built from the constant, never a bare `6`. `TestCatalogListsEveryExitCode` now derives its bounds from a `wantExitCodes` slice instead of hard-coded literals, so the next code addition won't repeat this plan's own drift risk. `TestCatalogExitCodesMatchMapper` — the anti-drift gate — needed zero edits, exactly as designed.
- Deleted `resolveServerURL` and `resolveOutputFormat`, and the four package-level `clientServerURL`/`clientTokenFile`/`clientInsecure`/`clientOutput` vars. `addClientFlags` now registers five flags (`--server`, `--token-file`, `--output`, `--insecure`, `--timeout`) directly against the `client.*` koanf registry via `config.FlagDefault`.
- `clientFromFlags` is now the single `config.Load(cmd.Flags())` + `config.ValidateClient` call site for every client verb, widened to return `(client, outputFormat, time.Duration, error)` — the output format and the parsed request deadline both come from this one load, so plan 01-08 (and any future output-format consumer) has exactly one place to read from.
- `--timeout` (default `30s`) is validated before any dial: `0`, `0s`, `-1s`, `abc`, and an explicit empty string are all rejected as `exitUsage`, including when combined with an unreachable `--server` (`config.ValidateClient` runs before `newHTTPClient`/`NewEngramServiceClient` ever get to dial).
- Closed the deferred `search/malformed-client-timeout-env` baseline row from plan 01-06 (flipped `landed: true`) and emptied `exitCodeBaselineFullyMigratedAllowlist` — the phase's closing proof for `TestExitCodeBaselineFullyMigrated`. Added three `introduced: true` rows (`search/timeout-zero`, `search/timeout-malformed`, `search/timeout-zero-beats-unreachable`); row count 29 → 32.
- `resolveToken` keeps its exact file-read-and-trim body and doc comment, only its argument source changed (`cfg.Client.TokenFile` instead of the deleted package var).

## Task Commits

Both `checkpoint:decision` tasks (D-01: `keep-3`, D-06: `new-code-6`) were pre-approved per the orchestrator's LOCKED CONTEXT.md decisions — recorded here as accepted, no interactive stop.

1. **Task 1: Add exitTimeout = 6, split the mapper arm, and advertise it** — `222ad00c` (feat)
2. **Task 2: Retire the hand-rolled client resolvers and register --timeout** — `bad13e0d` (feat)

_Per this repo's Phase 1 convention (see 01-01/01-02/01-03), tests and the implementation they exercise were committed together per task rather than as separate RED/GREEN commits._

## Files Created/Modified

- `cmd/engram/client_common.go` — `exitTimeout` constant, `exitCodeForConnectErr`'s split mapper arm, `addClientFlags` (5 registry-backed flags), `clientFromFlags` (single config-load constructor), `outputFormatFromConfig` (replaces `resolveOutputFormat`); `resolveServerURL` and the four package vars deleted
- `cmd/engram/client_common_test.go` — `TestExitCodeTimeoutDistinctFromUnavailable`, `TestOutputFormatFromConfig` (replaces `TestResolveOutputFormat`), `TestClientConfigResolution` + `newClientTestCmd`/`mustSetFlag`/`assertUsageError` helpers, `TestClientFilesImportBoundary` restructured with a per-file `internal/config` exception, `TestInsecureIsNotSetByEnvironment` fixed to not reference the deleted `clientInsecure` var
- `cmd/engram/catalog.go` — seventh `doc.ExitCodes` entry
- `cmd/engram/catalog_test.go` — `TestCatalogListsEveryExitCode` derives bounds from a `wantExitCodes` slice instead of hard-coded `6`/`0-5` literals
- `cmd/engram/exitcode_baseline_test.go` — `search/malformed-client-timeout-env` flipped `landed: true`; three new `introduced: true` rows; `TestExitCodeBaselineClaims` fixed to actually exempt `introduced` rows from the before/after check (see Deviations); row count 29 → 32; allowlist emptied
- `cmd/engram/client_search.go`, `cmd/engram/client_list.go`, `cmd/engram/client_store.go` — call sites updated for `clientFromFlags`'s widened 4-value return; `os` import dropped (no longer calling `isTerminal(os.Stdout)` directly)
- `cmd/engram/client_store_test.go` — `TestClientStoreNeverRetries`'s `deadlineexceeded` case updated to expect `exitTimeout`, not `exitUnavailable`
- `cmd/engram/clienttest_test.go` — `resetClientFlags` no longer zeroes four deleted vars; now also folds `resetEveryCommandFlagState(t, rootCmd)` in (see Deviations)

## Decisions Made

See `key-decisions` in frontmatter for full rationale on: the two pre-approved checkpoints (D-01, D-06), the widened `clientFromFlags` return signature (format bundled with duration), the `TestClientFilesImportBoundary` per-file exception, the `TestExitCodeBaselineClaims` introduced-row fix, and the `resetClientFlags` broadening.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] `TestClientFilesImportBoundary`'s blanket `internal/config` denylist blocked the plan's own mandated architecture**
- **Found during:** Task 2, before any code change
- **Issue:** The plan's `key_links` explicitly requires `clientFromFlags` to call `config.Load(cmd.Flags())` and `config.ValidateClient`, both requiring `client_common.go` to import `internal/config`. `TestClientFilesImportBoundary` (authored by plan 01-01, before this phase's own D-04 client-config-unification decision existed) denylisted `internal/config` in every `client_*.go` production file with no exception, and its Clause 2 additionally forbade ever widening the general allowlist with any `internal/` path. Implementing the plan as written would fail this pre-existing gate.
- **Fix:** Restructured the test from an aggregated cross-file import set to a per-file one, and added a single named constant (`clientConfigException = "client_common.go"`) permitted to import `internal/config`, with a positive control proving the exception is actually exercised (not silently inert). Every other `client_*.go` file (`client_search.go`, `client_list.go`, `client_store.go`) remains fully denied, since they only ever reach client configuration indirectly through `clientFromFlags`'s return values — the general allowlist (Clause 2) is untouched, preserving its "no repo-internal path" guarantee for everything except this one named, documented exception.
- **Files modified:** `cmd/engram/client_common_test.go`
- **Verification:** `TestClientFilesImportBoundary` passes; manually confirmed via `rg` that `internal/config` appears in no client_*.go production file except `client_common.go`.
- **Committed in:** `bad13e0d` (Task 2)

**2. [Rule 1 — Bug] `TestExitCodeBaselineClaims` never actually exempted `introduced` rows, despite its own doc comment promising it**
- **Found during:** Task 2, after adding the three `introduced: true` baseline rows
- **Issue:** The test's doc comment says introduced rows "assert only `after` and are exempt from the distinct/identical rules below," but the implementation only skipped the `changes==true` distinctness check for them — never the `changes==false` (default for an introduced row) implies `after==before` check. Since no row before this plan had ever set `introduced: true`, this gap was latent and untested. Adding the first three introduced rows made all three fail: their `before` (unset, zero value `0`/`exitOK`) legitimately differed from their `after` (`exitUsage`).
- **Fix:** Added an early `return` on `c.introduced` before the distinct/identical checks, matching the test's own documented contract exactly.
- **Files modified:** `cmd/engram/exitcode_baseline_test.go`
- **Verification:** `TestExitCodeBaselineClaims` passes for all three new rows plus every pre-existing row; `go test ./cmd/engram/... -run TestExitCodeBaseline` green.
- **Committed in:** `bad13e0d` (Task 2)

**3. [Rule 1 — Bug] Deleting the four package-level client flag vars silently broke test isolation `resetClientFlags` used to provide for free**
- **Found during:** Task 2, full-suite test run (`go test ./cmd/engram/...`) — individually green, `TestInsecureIsNotSetByEnvironment` only failed under the full package run
- **Issue:** Before this plan, `resetClientFlags`'s `t.Cleanup` zeroed `clientInsecure` (and its three siblings) unconditionally, independent of whether a given test also called `resetCommandFlagState` to clear pflag's own `Changed` latch. Several existing tests (e.g. `TestInsecureWarnsOnStderrAndStdoutStaysJSON`) set `--insecure` via `runClient` without pairing it with `resetCommandFlagState(t, searchCmd)`, relying on the deleted Go var's automatic reset instead. Once the var was deleted (D-04: no package-level var backs any of the five shared flags anymore), pflag's own `Value` on `searchCmd`'s FlagSet became the only state, and it leaked across tests that never explicitly reset it — reproduced with `go test ./cmd/engram/... -count=2 -shuffle=on` before the fix, and confirmed by `TestInsecureIsNotSetByEnvironment` failing only in a full-suite run.
- **Fix:** Folded `resetEveryCommandFlagState(t, rootCmd)` (the existing helper from `exitcode_baseline_test.go` that resets pflag state for rootCmd and every direct subcommand) into `resetClientFlags` itself, called immediately (not just via `t.Cleanup`). Every one of the dozens of pre-existing `resetClientFlags(t)` call sites in this package now gets the broader reset for free, without auditing each `runClient` call site individually for a missing `resetCommandFlagState` pairing.
- **Files modified:** `cmd/engram/clienttest_test.go`
- **Verification:** `go test ./cmd/engram/... -count=3 -shuffle=on` green (previously failed on `TestInsecureIsNotSetByEnvironment` and `TestClientStoreNeverRetries/deadlineexceeded` under `-count=2 -shuffle=on`, the latter fixed separately in Task 1's commit).
- **Committed in:** `bad13e0d` (Task 2)

**4. [Rule 1 — Bug] `TestClientStoreNeverRetries`'s `deadlineexceeded` case still expected the pre-D-06 exit code**
- **Found during:** Task 1, full-suite test run
- **Issue:** This pre-existing table test hard-coded `exitUnavailable` as the expected exit code for a `connect.CodeDeadlineExceeded` store failure — correct before this plan's mapper split, stale after.
- **Fix:** Updated the expected value to `exitTimeout`, with a one-line comment pointing to the new distinctness test.
- **Files modified:** `cmd/engram/client_store_test.go`
- **Verification:** `TestClientStoreNeverRetries` passes, including its `deadlineexceeded` subtest.
- **Committed in:** `222ad00c` (Task 1)

**5. [Rule 2 — call-site fallout named in the plan's own "Flagged assumptions"] `client_search.go`/`client_list.go`/`client_store.go` updated for `clientFromFlags`'s widened signature**
- **Found during:** Task 2, immediately after changing `clientFromFlags`'s return type
- **Issue:** The plan's `<files>` list for Task 2 named only `client_common.go`, `client_common_test.go`, and `exitcode_baseline_test.go`, but deleting `clientOutput`/`resolveOutputFormat` and widening `clientFromFlags`'s return signature necessarily breaks all three client verb call sites. The plan's own "Flagged assumptions" section anticipated exactly this: "If any caller outside the three client verbs uses it, update those call sites in this plan rather than adding a second resolution path; record any such caller in the SUMMARY."
- **Fix:** Updated all three call sites to the new `client, format, _, err := clientFromFlags(cmd)` shape (the duration is intentionally discarded here — plan 01-08 wires it into the RPC context), dropping the now-unused `os` import from each.
- **Files modified:** `cmd/engram/client_search.go`, `cmd/engram/client_list.go`, `cmd/engram/client_store.go`
- **Verification:** `go test ./cmd/engram/... -run 'TestClientSearch|TestClientList|TestClientStore'` green — no pre-existing client test regressed.
- **Committed in:** `bad13e0d` (Task 2)

---

**Total deviations:** 5 auto-fixed (1 Rule 3 — blocking pre-existing gate; 3 Rule 1 — pre-existing bugs surfaced by this plan's own new coverage; 1 Rule 2 — necessary call-site fallout explicitly anticipated by the plan itself)
**Impact on plan:** All five were necessary to land the plan's own mandated design (D-04's `clientFromFlags` architecture) without leaving pre-existing gates or newly-exposed test flakiness behind. No scope creep beyond `cmd/engram/*.go` — no file outside this plan's package was touched, and `internal/config`'s ~33 `Config{}` test literals were confirmed untouched (`git status --short internal/config/` empty throughout).

## Issues Encountered

None beyond the five deviations above, all resolved without blocking progress. `go test ./... -count=2 -shuffle=on` also surfaced two failures in `internal/e2e` (`TestCLIExitCodes/unknown_flag_exits_1`) and `internal/embed` (`TestEmbedEmitsSpan`) — confirmed via `git stash` to be pre-existing on the base commit (`b7b9f051`), unrelated to this plan, and reproducible with zero files from this plan applied. Not fixed (out of scope — neither file is touched by this plan), left for a future plan/issue to address.

## Known Stubs

None.

## Threat Flags

None — no new network endpoints, auth paths, or schema changes at a trust boundary. `--timeout`'s DoS mitigation (T-1-01, a finite client deadline is now enforceable) is validated in this plan and will be load-bearing once plan 01-08 wires the duration into the RPC context; the credential-handling mitigation (T-1-04) is unchanged, confirmed by `TestTokenNeverAppearsInOutput` and the `client.token_file`-only (never `client.token`) registry shape.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Plan 01-08 (deferred): `clientFromFlags`'s returned `time.Duration` is resolved and validated but NOT yet wired into any RPC call's context — every client verb still dials with no deadline. Plan 01-08 should wrap `cmd.Context()` with `context.WithTimeout(ctx, timeout)` at each of the three call sites (`client_search.go`, `client_list.go`, `client_store.go`), replacing the `_` discard added here.
- Plan 01-09 (docs): must document (1) the new `exitTimeout = 6` exit code and its meaning, (2) the new `--timeout` flag on every client verb (default 30s, 0 rejected — opposite convention from the operator commands' own `--timeout`, already flagged by 01-06), and (3) that `resolveServerURL`/`resolveOutputFormat` and the four package-level client flag vars no longer exist (internal API surface, not user-facing, but relevant if any external tooling referenced them).
- `exitCodeBaselineFullyMigratedAllowlist` is now empty — the phase's D-09 closing proof holds for every row this table tracks, client and operator alike.
- `TestClientFilesImportBoundary`'s new per-file exception mechanism (`clientConfigException`) is reusable if a future plan needs the same "one named file, one named internal import" pattern elsewhere in this package.
- No blockers.

---
*Phase: 01-interface-enforceability*
*Completed: 2026-08-04*

## Self-Check: PASSED

All ten created/modified files found on disk; both task commit hashes (`222ad00c`, `bad13e0d`) found in git log.
