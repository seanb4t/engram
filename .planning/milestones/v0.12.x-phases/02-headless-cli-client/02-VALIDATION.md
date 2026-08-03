---
phase: 2
slug: headless-cli-client
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-02
---

# v0.12.x Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Reconstructed retroactively by `/gsd-validate-phase 2` from `02-01`–`02-03-SUMMARY.md`
> (State B — the phase executed before a VALIDATION.md existed).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing`, table-driven — existing convention across `cmd/engram`, `internal/e2e` |
| **Config file** | none — no test-framework config beyond `go.mod` / `go vet` / `golangci-lint` |
| **Quick run command** | `go test ./cmd/engram/...` |
| **Binary tier command** | `task test:e2e` (builds the binary, drives the real process) |
| **Full suite command** | `task` (lint + test, per `Taskfile.yaml`) |
| **Estimated runtime** | ~1s quick / ~3s binary tier / ~2–4 min full |

---

## Sampling Rate

- **After every task commit:** Run the quick run command above
- **After every plan wave:** Run `task` (full lint + test)
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~1 second (quick), ~3 seconds (binary tier)

> **Repo-specific false-green guard:** `go test -run X ./pkg/...` matching nothing exits `0`
> with `ok … [no tests to run]`. Every targeted `-run` invocation MUST be proven with `-v` and a
> visible `=== RUN` / `--- PASS` pair.

---

## Per-Task Verification Map

One row per coverage decision recorded in the plan SUMMARYs. All commands are rooted at
`./cmd/engram/...` unless stated otherwise.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 02-01 T1 | 02-01 | 1 | REQ-cli-client-commands | — | `engram search` completes over the real Connect wire; exactly one JSON doc on stdout, exit 0 | integration | `go test ./cmd/engram/... -run TestClientSearchEndToEndJSON -v` | ✅ `client_search_test.go` | ✅ green |
| 02-01 T1 | 02-01 | 1 | REQ-cli-client-commands | — | Server URL resolves `--server` then `ENGRAM_SERVER_URL`; missing both → exit 2, no network call | integration | `go test ./cmd/engram/... -run TestClientSearchMissingServerURLIsUsageError -v` | ✅ `client_search_test.go` | ✅ green |
| 02-01 T1 | 02-01 | 1 | REQ-cli-credential-safety | T-02-01 / T-02-06 | Credential resolves `ENGRAM_TOKEN` then `--token-file`; rides the wire as exactly `Bearer <token>`, one space | integration | `go test ./cmd/engram/... -run 'TestClientSearchSendsBearerHeader\|TestClientSearchTokenFromFile\|TestClientSearchEnvBeatsTokenFile\|TestTokenFileTrailingNewlineTrimmed' -v` | ✅ `client_search_test.go`, `client_common_test.go` | ✅ green |
| 02-01 T2 | 02-01 | 1 | REQ-cli-credential-safety | T-02-01 | No command declares a `--token` flag anywhere in the tree | unit | `go test ./cmd/engram/... -run TestNoTokenFlagAnywhere -v` | ✅ `client_common_test.go` | ✅ green |
| 02-01 T1/T2 | 02-01 | 1 | REQ-cli-credential-safety | T-02-02 | TLS verification on by default; `--insecure` always warns on stderr, never suppressible, no env fallback | unit/integration | `go test ./cmd/engram/... -run 'TestTLSVerificationOnByDefault\|TestInsecureWarnsOnStderrAndStdoutStaysJSON\|TestInsecureIsNotSetByEnvironment' -v` | ✅ `client_common_test.go` | ✅ green |
| 02-01 T1 | 02-01 | 1 | REQ-cli-agent-output | — | JSON when stdout is not a character device, table when it is; `--output` overrides both directions | unit/integration | `go test ./cmd/engram/... -run 'TestResolveOutputFormat\|TestClientSearchTextOutputIsNotJSON' -v` | ✅ `client_common_test.go`, `client_search_test.go` | ✅ green |
| 02-01 T1 | 02-01 | 1 | REQ-cli-agent-output | — | An empty search result exits 0 and emits `memories:[]` — never `null` | integration | `go test ./cmd/engram/... -run TestClientSearchEmptyResultIsEmptyArray -v` | ✅ `client_search_test.go` | ✅ green |
| 02-01 T1 | 02-01 | 1 | REQ-cli-agent-output | — | One shared mapper drives the exit-code taxonomy across all 16 `connect.Code` values plus a non-connect error | unit/integration | `go test ./cmd/engram/... -run 'TestExitCodeForConnectErrTable\|TestClientSearchExitCode' -v` | ✅ `client_common_test.go`, `client_search_test.go` | ✅ green |
| 02-01 T1 | 02-01 | 1 | REQ-cli-agent-output | — | `Execute()` consults `ExitCode()` via `errors.As`; a plain error still exits 1 byte-for-byte | unit | `go test ./cmd/engram/... -run TestExitCodeFromError -v` | ✅ `root_test.go` | ✅ green |
| 02-01 T2 | 02-01 | 1 | REQ-cli-client-commands | T-02-04 | No `client_*.go` imports `internal/{store,authz,embed,server,config}`; allowlist holds no `internal/` path | unit | `go test ./cmd/engram/... -run TestClientFilesImportBoundary -v` | ✅ `client_common_test.go` | ✅ green |
| 02-01 T2 | 02-01 | 1 | REQ-cli-agent-output | T-02-05 | No client code path reads stdin; no invocation can block on a prompt | unit | `go test ./cmd/engram/... -run TestNoClientPathReadsStandardInput -v` | ✅ `client_common_test.go` | ✅ green |
| 02-01 T2 | 02-01 | 1 | REQ-cli-credential-safety | T-02-03 | The credential never appears in stdout, stderr, or a returned error's `Error()` — success, auth-failure, transport-failure | integration | `go test ./cmd/engram/... -run TestTokenNeverAppearsInOutput -v` | ✅ `client_common_test.go` | ✅ green |
| 02-01 T2 | 02-01 | 1 | REQ-cli-credential-safety | T-02-01 | No command accepts positional data (second structural guard against a credential riding in as a bare word) | integration | `go test ./cmd/engram/... -run TestClientCommandsAcceptNoPositionalArgs -v` | ✅ `client_common_test.go` | ✅ green |
| 02-02 T1 | 02-02 | 2 | REQ-cli-client-commands | — | `engram list` returns memories + exact total + `next_page_token` as one JSON object, exit 0 | integration | `go test ./cmd/engram/... -run TestClientListEndToEndJSON -v` | ✅ `client_list_test.go` | ✅ green |
| 02-02 T1 | 02-02 | 2 | REQ-cli-client-commands | — | Every `ListMemoriesRequest` filter flag reaches its wire field | integration | `go test ./cmd/engram/... -run 'TestClientListPassesFiltersToRequest\|TestClientListCursorModeReachesRequest' -v` | ✅ `client_list_test.go` | ✅ green |
| 02-02 T1 | 02-02 | 2 | REQ-cli-agent-output | — | An empty list result exits 0 and emits `memories:[]` — never `null` | integration | `go test ./cmd/engram/... -run TestClientListEmptyResultIsEmptyArray -v` | ✅ `client_list_test.go` | ✅ green |
| 02-02 T1 | 02-02 | 2 | REQ-cli-agent-output | — | `list` exit codes: Unauthenticated→3, NotFound→4, InvalidArgument→2; missing server URL→2 with zero calls | integration | `go test ./cmd/engram/... -run 'TestClientListExitCodes\|TestClientListMissingServerURLIsUsageError' -v` | ✅ `client_list_test.go` | ✅ green |
| 02-02 T1 | 02-02 | 2 | REQ-cli-client-commands | — | `list` exposes no flag or column for the deprecated `approximate` response field | unit | `go test ./cmd/engram/... -run TestClientListNoDeprecatedApproximateFlag -v` | ✅ `client_list_test.go` | ✅ green |
| 02-02 T1 | 02-02 | 2 | REQ-cli-agent-output | — | `list --output=text` renders a human table containing the returned `short_id`, not JSON | integration | `go test ./cmd/engram/... -run TestClientListTextOutput -v` | ✅ `client_list_test.go` | ✅ green |
| 02-02 T2 | 02-02 | 2 | REQ-cli-client-commands | — | `engram store` writes through a real Connect server, returning `id` + `short_id` as one JSON object, exit 0 | integration | `go test ./cmd/engram/... -run TestClientStoreEndToEndJSON -v` | ✅ `client_store_test.go` | ✅ green |
| 02-02 T2 | 02-02 | 2 | REQ-cli-client-commands | — | Every `StoreMemoryRequest` field reaches its wire field | integration | `go test ./cmd/engram/... -run TestClientStorePassesFieldsToRequest -v` | ✅ `client_store_test.go` | ✅ green |
| 02-02 T2 | 02-02 | 2 | REQ-cli-agent-output | — | Empty `--content`/`--scope` rejected locally with exit 2 and zero network calls, before any RPC | integration | `go test ./cmd/engram/... -run TestClientStoreRequiresContentAndScope -v` | ✅ `client_store_test.go` | ✅ green |
| 02-02 T2 | 02-02 | 2 | REQ-cli-agent-output | T-02-09 | A failed `StoreMemory` is attempted exactly once, never retried — Unavailable, Internal, DeadlineExceeded | integration | `go test ./cmd/engram/... -run TestClientStoreNeverRetries -v` | ✅ `client_store_test.go` | ✅ green |
| 02-02 T2 | 02-02 | 2 | REQ-cli-client-commands | T-02-10 | `store` declares no flag for `actor`, `owner`, or any response-only field | unit | `go test ./cmd/engram/... -run TestClientStoreNoActorOrOwnerFlag -v` | ✅ `client_store_test.go` | ✅ green |
| 02-02 T2 | 02-02 | 2 | REQ-cli-agent-output | — | `store --category` help text names all four accepted wire values | unit | `go test ./cmd/engram/... -run TestClientStoreCategoryHelpNamesLegalValues -v` | ✅ `client_store_test.go` | ✅ green |
| 02-02 T2 | 02-02 | 2 | REQ-cli-agent-output | — | `store` exit codes: InvalidArgument→2, Unauthenticated→3, PermissionDenied→3 | integration | `go test ./cmd/engram/... -run TestClientStoreExitCodes -v` | ✅ `client_store_test.go` | ✅ green |
| 02-02 T2 | 02-02 | 2 | REQ-cli-agent-output | — | `store --output=text` renders plain text containing the returned short id | integration | `go test ./cmd/engram/... -run TestClientStoreTextOutput -v` | ✅ `client_store_test.go` | ✅ green |
| 02-02 T1/T2 | 02-02 | 2 | REQ-cli-client-commands | — | Both commands build through the single shared `clientFromFlags` and classify through the single shared mapper — no second constructor or mapper | source gate | `rg -c 'clientFromFlags\(cmd\)' cmd/engram/client_list.go` = 1; `rg -c 'StoreMemory\(' cmd/engram/client_store.go` = 1 | ✅ `client_list.go`, `client_store.go` | ✅ green |
| 02-03 T1 | 02-03 | 3 | REQ-cli-self-describing | — | A bare `engram` writes one JSON document to stdout and exits 0; stderr is empty | integration | `go test ./cmd/engram/... -run 'TestRootBareInvocationEmitsCatalog\|TestCatalogGoesToStdoutNotStderr' -v` | ✅ `catalog_test.go` | ✅ green |
| 02-03 T2 | 02-03 | 3 | REQ-cli-self-describing | T-02-15 | Catalog command-name set **equals** the live tree's non-hidden subcommands (set equality, not subset) | unit | `go test ./cmd/engram/... -run TestCatalogEnumeratesEveryCommand -v` | ✅ `catalog_test.go` | ✅ green |
| 02-03 T2 | 02-03 | 3 | REQ-cli-self-describing | T-02-15 | Per command, catalog flag-name set equals own flags ∪ root persistent flags; each entry carries type + usage | unit | `go test ./cmd/engram/... -run TestCatalogEnumeratesEveryFlag -v` | ✅ `catalog_test.go` | ✅ green |
| 02-03 T2 | 02-03 | 3 | REQ-cli-self-describing | — | Catalog lists exactly six exit-code entries covering 0–5, no duplicates, each with a non-empty meaning | unit | `go test ./cmd/engram/... -run TestCatalogListsEveryExitCode -v` | ✅ `catalog_test.go` | ✅ green |
| 02-03 T2 | 02-03 | 3 | REQ-cli-agent-output | T-02-14 | Catalog exit-code set equals, **bidirectionally**, what the mapper can actually produce ∪ `exitOK` | unit | `go test ./cmd/engram/... -run TestCatalogExitCodesMatchMapper -v` | ✅ `catalog_test.go` | ✅ green |
| 02-03 T2 | 02-03 | 3 | REQ-cli-self-describing | — | Catalog documents D-17: both exit 1 (framework flag-parse) and exit 2 (engram's own validation) | unit | `go test ./cmd/engram/... -run TestCatalogDocumentsFlagParseExitCode -v` | ✅ `catalog_test.go` | ✅ green |
| 02-03 T1 | 02-03 | 3 | REQ-cli-self-describing | T-02-13 | A mistyped verb still fails with non-JSON stdout; every pre-existing subcommand still resolves | unit | `go test ./cmd/engram/... -run 'TestRootUnknownSubcommandStillErrors\|TestRootExistingSubcommandsStillResolve' -v` | ✅ `catalog_test.go` | ✅ green |
| 02-03 T1 | 02-03 | 3 | REQ-cli-self-describing | — | `--help` stays ordinary human cobra output and is provably a different artifact from the catalog | unit | `go test ./cmd/engram/... -run 'TestHelpFlagStillPrintsHumanHelp\|TestHelpAndCatalogAreDifferentOutputs' -v` | ✅ `catalog_test.go` | ✅ green |
| 02-03 T2 | 02-03 | 3 | REQ-cli-self-describing | — | The CLI guide documents the three verbs, credential precedence, output contract, exit-code table, D-17 caveat | docs gate | `task lint` (rumdl) + exit-code table rows 0–5 present in `docs-site/src/content/docs/guides/cli.md` | ✅ `guides/cli.md` | ✅ green |
| AUDIT-01 | — | audit | REQ-cli-agent-output / REQ-cli-self-describing | T-02-13 / T-02-14 | The **real built binary's** `os.Exit` path yields the advertised taxonomy: no-server-url→2, closed-port→5, bare→0 (one JSON doc, empty stderr), unknown-verb→1, unknown-flag→1, `--help`→0 | e2e | `go test ./internal/e2e/ -run TestCLIExitCodes -count=1 -v` | ✅ `internal/e2e/cli_exitcode_test.go` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements. The three plans landed their own test files
alongside the implementation, and the binary tier (`internal/e2e`) already had a `TestMain` build
harness this audit reused rather than duplicated.

- [x] `cmd/engram/clienttest_test.go` — the in-process real-Connect-handler harness (`stubEngramService`, `startStubServer`, `resetClientFlags`, `runClient`), shared by Plans 01–03
- [x] `cmd/engram/client_common_test.go`, `client_search_test.go` — Plan 01
- [x] `cmd/engram/client_list_test.go`, `client_store_test.go` — Plan 02
- [x] `cmd/engram/catalog_test.go` — Plan 03
- [x] `internal/e2e/cli_exitcode_test.go` — **added by this audit** (AUDIT-01), reusing `harness_test.go`'s `TestMain` build and `childEnv` hermetic environment

---

## Manual-Only Verifications

*All phase behaviors have automated verification.* The binary-level exit-code observations that
were manual at execution time (recorded as `e2e: built binary` refs in `02-01-SUMMARY.md` D9 and
`02-03-SUMMARY.md` D1/D7/D8) are automated as of this audit — see row AUDIT-01.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Every targeted `-run` command proven with `-v` RUN/PASS pairs (repo false-green guard)
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-08-02 — retroactive reconstruction, 1 gap found and resolved

---

## Validation Audit 2026-08-02

| Metric | Count |
|--------|-------|
| Gaps found | 1 |
| Resolved | 1 |
| Escalated | 0 |

Reconstructed from artifacts (State B — no VALIDATION.md existed for this phase). Evidence:

- The 50 Go tests named across the three SUMMARY `coverage:` blocks were checked as a set against
  a full `go test -count=1 -v ./cmd/engram/...` run: **50 expected, 50 `--- PASS:`, 0 fail, 0 skip,
  0 missing.** Matching by name against `--- PASS:` lines (rather than trusting the package-level
  `ok`) is required here — a `-run` filter matching nothing still exits 0.
- **GAP (resolved):** the CLI's exit-code taxonomy was proven only *in-process*. `TestExitCodeFromError`
  proves the `errors.As` mapping and `TestRootBareInvocationEmitsCatalog` proves the catalog shape,
  but nothing re-ran the real binary's `os.Exit` path — those six observations lived only as
  `e2e: built binary` prose in the SUMMARYs. A regression in the `Execute()` → `os.Exit` wiring, or
  a cobra behavior change at the true entry point, would have passed the entire suite.
  Closed by `internal/e2e/cli_exitcode_test.go` (`TestCLIExitCodes`, 6 subtests), which reuses the
  existing `TestMain` binary build rather than adding a second one.
- The six expected exit codes were confirmed **independently of the generated test**, by invoking a
  freshly built binary directly: 2, 5, 0, 1, 1, 0 — matching the assertions. This guards the failure
  mode this repo has hit before (v0.11.x shipped a *passing* test that asserted a bug): the test
  encodes correct behavior, not merely self-consistent behavior.
- Known cosmetic imprecision, left as-is: the first subtest's name claims "no network call", but the
  subtest asserts only the exit code. Exit 2 before any dial is a sound proxy; the stronger
  zero-calls assertion already exists in-process at
  `cmd/engram/client_search_test.go#TestClientSearchMissingServerURLIsUsageError`.
