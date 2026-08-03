---
phase: 02-headless-cli-client
verified: 2026-07-31T00:00:00Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 2: Headless CLI Client Verification Report

**Phase Goal:** An agent with only a shell — a subagent with a closed tool list, a CI step, a cron
loop — can search, store, and list memories against a remote engram server.
**Verified:** 2026-07-31
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `engram search`/`list`/`store` complete against a running server given a server URL and a token, emitting structured JSON when stdout is not a TTY | ✓ VERIFIED | Built `/tmp/engram-verify` and ran real invocations; `TestClientSearchEndToEndJSON`, `TestClientListEndToEndJSON`, `TestClientStoreEndToEndJSON` run a real generated Connect handler (`startStubServer`, real wire protocol) and pass (`go test -v`, RUN/PASS pairs captured). `renderJSON` uses `protojson` with `UseProtoNames`/`EmitDefaultValues`, so field names mirror the proto (`memories`, `total`, `next_page_token`, `id`, `short_id`). |
| 2 | Data to stdout, diagnostics to stderr; exit codes distinguish auth/not-found/validation/transport failure; no command prompts on any path | ✓ VERIFIED | Live binary: bare invocation exits 0, unknown verb exits 1, missing `--server` exits 2, closed-port dial exits 5 (`Error: unavailable: dial tcp 127.0.0.1:1: connect: connection refused`). `exitCodeForConnectErr` maps `Unauthenticated`/`PermissionDenied`→3, `NotFound`→4, `InvalidArgument`/`FailedPrecondition`/`OutOfRange`→2, `Unavailable`/`DeadlineExceeded`/`Canceled`→5, else 1 — proven by `TestClientSearchExitCode{Auth,NotFound,InvalidArgument,Transport}`, `TestClientListExitCodes`, `TestClientStoreExitCodes` (all RUN/PASS). No-prompt: `TestNoClientPathReadsStandardInput` is an AST walk over every `client_*.go` production file asserting no `os.Stdin` reference exists (structural, not just behavioral) — PASS. |
| 3 | A token supplied by env var or file never appears in `argv`, and TLS verification cannot be disabled silently | ✓ VERIFIED | Structural: `--token` flag does not exist anywhere in the binary (`--help` output enumerated; `TestNoTokenFlagAnywhere` walks the whole cobra tree with a positive control on `search --token-file`, PASS). Only `--token-file` (a path) and `ENGRAM_TOKEN` (env) resolve a credential (`resolveToken` in `client_common.go`). TLS: `newHTTPClient(false)` leaves `InsecureSkipVerify=false` (`TestTLSVerificationOnByDefault`, PASS); `--insecure` unconditionally writes to stderr and never stdout, verified by full-buffer JSON-unmarshal + substring checks (`TestInsecureWarnsOnStderrAndStdoutStaysJSON`, PASS); no env var can flip it (`TestInsecureIsNotSetByEnvironment` sets three plausible env names, asserts `clientInsecure` stays false and stderr stays empty, PASS). |
| 4 | A bare invocation returns the full command/flag/exit-code catalog as structured output | ✓ VERIFIED | Live binary: bare `engram` invocation emits one JSON document (verified by hand — full catalog with 11 commands including search/list/store, all flags, all 6 exit codes, and D-17 notes) and exits 0. `TestCatalogEnumeratesEveryCommand`/`TestCatalogEnumeratesEveryFlag` assert **set equality** (not membership) against the live cobra tree, with positive controls (search/list/store name membership) so the test cannot pass on an empty set — PASS. `TestCatalogExitCodesMatchMapper` asserts set equality between the catalog's advertised codes and every code `exitCodeForConnectErr` can actually produce across all 16 connect codes plus the non-connect-error case — PASS. `--help` remains distinct human cobra output (`TestHelpFlagStillPrintsHumanHelp`, `TestHelpAndCatalogAreDifferentOutputs`, PASS). |
| 5 | No client subcommand imports `internal/store`, `internal/authz`, or `internal/embed` | ✓ VERIFIED | `TestClientFilesImportBoundary` AST-parses every `client_*.go` production file's imports (not `go list -deps`, which would be package-wide and produce a false RED since `reindex.go`/`migrate.go` legitimately import `internal/store`), asserts an exhaustive allowlist (6 entries, none repo-internal) and an explicit denylist including `internal/store`, `internal/authz`, `internal/embed`, `internal/server`, `internal/config` — PASS. Gate has two non-vacuous guards: `len(files)==0` and `scanned==0` both `t.Fatal`, so it cannot pass by finding nothing. |

**Score:** 5/5 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/engram/client_common.go` | shared constructor, TLS/token/output resolution, exit-code mapper | ✓ VERIFIED | Present, substantive (315 lines), wired into all three commands via `clientFromFlags`/`wrapRPCError` |
| `cmd/engram/client_search.go` | `engram search` | ✓ VERIFIED | Present, registers via `init()` → `rootCmd.AddCommand(searchCmd)`, exercised live |
| `cmd/engram/client_list.go` | `engram list` | ✓ VERIFIED | Present, registers via `init()`, exercised live |
| `cmd/engram/client_store.go` | `engram store`, single-attempt write | ✓ VERIFIED | Present, `TestClientStoreNeverRetries` proves single-attempt via a call counter placed in the stub's `StoreMemory` method itself (not inside the injected error closure), so it observes every attempt — PASS |
| `cmd/engram/clienttest_test.go` | shared real-Connect-handler test harness | ✓ VERIFIED | `startStubServer` mounts the actual generated `engramv1connect` handler on `httptest.Server` — real wire protocol, not a hand-rolled stub |
| `cmd/engram/catalog.go` | self-describe catalog builder | ✓ VERIFIED | Builds from live cobra tree + exit-code constants, not a parallel literal list |
| `cmd/engram/root.go` | `Execute` exit-code dispatch, runnable root | ✓ VERIFIED | `exitCodeFromError` via `errors.As`, `RunE: runSelfDescribe` + `Args: cobra.NoArgs` |
| `docs-site/src/content/docs/guides/cli.md` | CLI guide | ✓ VERIFIED | Present, documents all 3 verbs, credential precedence, exit-code table incl. D-17 caveat, self-describe catalog |

### Key Link Verification

| From | To | Via | Status |
|------|----|----|--------|
| `client_search.go` | `client_common.go` | `clientFromFlags(cmd)` | ✓ WIRED |
| `client_list.go` | `client_common.go` | `clientFromFlags(cmd)` | ✓ WIRED |
| `client_store.go` | `client_common.go` | `clientFromFlags(cmd)` / `wrapRPCError(err)` | ✓ WIRED |
| `root.go` | `client_common.go` | `errors.As(err, &ec)` exit-code dispatch | ✓ WIRED |
| `root.go` | `catalog.go` | `RunE: runSelfDescribe` | ✓ WIRED |
| `catalog.go` | `client_common.go` | exit-code section built from `exitOK`/`exitGeneric`/... constants | ✓ WIRED |
| `client_common.go` | `gen/go/engram/v1/engramv1connect` | `engramv1connect.NewEngramServiceClient(...)` | ✓ WIRED |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Bare invocation | `/tmp/engram-verify` | JSON catalog, exit 0 | ✓ PASS |
| Unknown verb | `/tmp/engram-verify definitely-not-a-verb` | `Error: unknown command "definitely-not-a-verb" for "engram"`, exit 1 | ✓ PASS |
| Missing `--server` | `/tmp/engram-verify search --query q` | `Error: --server or ENGRAM_SERVER_URL is required`, exit 2 | ✓ PASS |
| Transport failure (closed port) | `/tmp/engram-verify search --server http://127.0.0.1:1 --query q` | `Error: unavailable: dial tcp 127.0.0.1:1: connect: connection refused`, exit 5 | ✓ PASS |
| `--help` | `/tmp/engram-verify --help` / `search --help` | Ordinary cobra help, not JSON | ✓ PASS |
| No `--token` flag | `/tmp/engram-verify search --help` | Only `--token-file` present | ✓ PASS |
| Named unit tests (37 total across boundary/exit-code/TLS/token/catalog/no-retry/no-stdin gates) | `go test ./cmd/engram/... -run '<names>' -v` | Every test produced a RUN/PASS pair (no vacuous `ok` credited) | ✓ PASS |
| `go vet ./cmd/engram/...` | | clean | ✓ PASS |
| `task license:check` | | 232 valid, 0 invalid | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Status | Evidence |
|-------------|-----------------|--------|----------|
| REQ-cli-client-commands | 02-01, 02-02 | ✓ SATISFIED | search/list/store all complete against a live Connect stub; import boundary gate enforced |
| REQ-cli-agent-output | 02-01, 02-02, 02-03 | ✓ SATISFIED | TTY-aware JSON/text, stdout/stderr split, exit-code taxonomy, no-stdin gate, catalog documents the taxonomy |
| REQ-cli-credential-safety | 02-01 | ✓ SATISFIED | No `--token` flag, TLS on by default with unconditional warning, no env-based `--insecure` bypass |
| REQ-cli-self-describing | 02-03 | ✓ SATISFIED | Bare invocation catalog with set-equality anti-drift gates on commands, flags, and exit codes |

No orphaned requirements found — `REQ-cli-*` mapping in REQUIREMENTS.md lists exactly these four against v0.12.x Phase 2, all four appear in at least one plan's `requirements:` frontmatter.

### Anti-Patterns Found

None. Scanned `cmd/engram/client_*.go`, `catalog.go`, `root.go` for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER`/stub phrasing — zero matches. `os.Exit` does not appear in any `RunE` (prohibition from 02-01-PLAN.md honored). No new dependencies (`allowedClientImports` is closed over already-vendored packages; `go.mod`/`go.sum` reported zero diff by the orchestrator's gate run).

### Already-Settled Items (not re-litigated)

- `cobra.NoArgs` / `legacyArgs` interaction on the runnable root — confirmed correct per task brief, `TestRootUnknownSubcommandStillErrors` and the live "unknown verb" run both confirm the behavior holds regardless of mechanism.
- D-17 exit-1-vs-2 asymmetry — confirmed present and documented in the catalog's `Notes` and the docs-site guide's caution callout; not treated as a gap.
- CSRF plumbing absence — confirmed absent in `client_common.go` (no cookie-jar or anti-forgery code); correct per D-14/D-01 research finding.

### Human Verification Required

None. Every success criterion resolved to a structural or behavioral check runnable from the shell; no visual, real-time, or external-service judgment call remained.

### Gaps Summary

No gaps. All 5 ROADMAP success criteria verified with concrete, non-vacuous evidence: real binary invocations against a genuine failure mode (closed port), and 37+ named unit tests confirmed to actually execute (RUN/PASS pairs) rather than trusted via package-level `ok`. Fixture soundness was checked for the two claims called out in the verification brief — `TestClientFilesImportBoundary` cannot pass on an empty file set (explicit `t.Fatal` guards), and `TestClientStoreNeverRetries`'s call counter lives in the stub's method body (observes every attempt) rather than inside the injected error closure (which would only observe a successful attempt).

---

_Verified: 2026-07-31_
_Verifier: Claude (gsd-verifier)_
