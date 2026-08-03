---
phase: 02-headless-cli-client
plan: 01
subsystem: api
tags: [cobra, connect-go, cli, exit-codes, tls, bearer-auth]

# Dependency graph
requires:
  - phase: v0.12.x Phase 1 (shared auth chain / connect bearer identity)
    provides: the bearer-authenticated Connect lane (composed verifier, CSRF exemption for
      auth.LaneBearer) this plan's client calls into
provides:
  - "cmd/engram/client_common.go: the shared client foundation (clientFromFlags,
    server-URL/token resolution, TLS policy, bearer interceptor, TTY-aware output-format
    resolution, the D-10 connect.Code -> exit-code mapper, JSON/table renderers)"
  - "cmd/engram/client_search.go: `engram search`, the first of the three D-01 subcommands"
  - "cmd/engram/clienttest_test.go: the reusable in-process real-Connect-handler test harness
    (stubEngramService/startStubServer/resetClientFlags/runClient) for Plans 02 and 03"
  - "cmd/engram/root.go: Execute() now dispatches on an ExitCode() int accessor via errors.As"
affects: [02-02 (list, store), 02-03 (self-describe catalog, rootCmd.RunE)]

# Actuals (#2632)
actuals:
  tokens: 12630
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "client_*.go per-file import allowlist (AST-parsed, not go list -deps) — the only way to
      gate a client command's dependencies when operator commands in the same package
      legitimately import internal/store"
    - "cliError{code, err} with ExitCode() int, consulted via errors.As in Execute() — additive,
      byte-compatible exit-code taxonomy over every pre-existing plain-error command"
    - "connect.UnaryInterceptorFunc client-side bearer attachment, mirroring the server-side
      interceptor idiom already in internal/server"

key-files:
  created:
    - cmd/engram/client_common.go
    - cmd/engram/client_common_test.go
    - cmd/engram/client_search.go
    - cmd/engram/client_search_test.go
    - cmd/engram/clienttest_test.go
  modified:
    - cmd/engram/root.go
    - cmd/engram/root_test.go

key-decisions:
  - "resolveServerURL resolves at RUN time (not baked into the flag default via
    os.Getenv-in-default like reindex.go), so D-02 precedence stays testable with t.Setenv and
    the env value never leaks into --help output — a deliberate divergence from the repo's
    existing idiom, documented in the code comment."
  - "renderJSON uses protojson with UseProtoNames+EmitDefaultValues so the wire vocabulary
    (short_id, created_at, summary_source, empty arrays not null) is derived from the message,
    not a hand-written struct that could drift from D-08/D-12."
  - "The import-boundary gate (TestClientFilesImportBoundary) is per-file via go/parser
    ImportsOnly, not go list -json ./cmd/engram/... — a package-level gate is a permanent false
    RED because reindex.go/prune.go legitimately import internal/store in the same package."

patterns-established:
  - "clientFromFlags(cmd) is the single seam every future client subcommand (list, store) hangs
    server URL / token / TLS / bearer-header construction off."
  - "resolveOutputFormat(flagVal string, isTTY bool) takes the TTY boolean as a parameter, not a
    global call to isTerminal — the seam that lets tests force both branches without a pty."

requirements-completed: [REQ-cli-client-commands, REQ-cli-agent-output, REQ-cli-credential-safety]

coverage:
  - id: D1
    description: "engram search completes against a real in-process Connect server over the
      Connect wire protocol, writing exactly one JSON document to stdout, exit 0"
    requirement: "REQ-cli-client-commands"
    verification:
      - kind: integration
        ref: "cmd/engram/client_search_test.go#TestClientSearchEndToEndJSON"
        status: pass
    human_judgment: false
  - id: D2
    description: "Server URL resolves --server then ENGRAM_SERVER_URL; missing both fails with
      exit 2 and makes no network call"
    requirement: "REQ-cli-client-commands"
    verification:
      - kind: integration
        ref: "cmd/engram/client_search_test.go#TestClientSearchMissingServerURLIsUsageError"
        status: pass
    human_judgment: false
  - id: D3
    description: "Bearer credential resolves ENGRAM_TOKEN then --token-file, rides the wire as
      exactly `Bearer <token>`, one space"
    requirement: "REQ-cli-credential-safety"
    verification:
      - kind: integration
        ref: "cmd/engram/client_search_test.go#TestClientSearchSendsBearerHeader"
        status: pass
      - kind: integration
        ref: "cmd/engram/client_search_test.go#TestClientSearchTokenFromFile"
        status: pass
      - kind: integration
        ref: "cmd/engram/client_search_test.go#TestClientSearchEnvBeatsTokenFile"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestTokenFileTrailingNewlineTrimmed"
        status: pass
    human_judgment: false
  - id: D4
    description: "No command declares a --token flag anywhere in the tree; --token-file is the
      only token-bearing flag"
    requirement: "REQ-cli-credential-safety"
    verification:
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestNoTokenFlagAnywhere"
        status: pass
    human_judgment: false
  - id: D5
    description: "TLS verification on by default; --insecure always warns unconditionally on
      stderr, never gated by --output, never suppressible; no env fallback"
    requirement: "REQ-cli-credential-safety"
    verification:
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestTLSVerificationOnByDefault"
        status: pass
      - kind: integration
        ref: "cmd/engram/client_common_test.go#TestInsecureWarnsOnStderrAndStdoutStaysJSON"
        status: pass
      - kind: integration
        ref: "cmd/engram/client_common_test.go#TestInsecureIsNotSetByEnvironment"
        status: pass
    human_judgment: false
  - id: D6
    description: "Output format is JSON when stdout is not a character device, table when it is;
      --output=json|text overrides in both directions; invalid value is a usage error"
    requirement: "REQ-cli-agent-output"
    verification:
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestResolveOutputFormat"
        status: pass
      - kind: integration
        ref: "cmd/engram/client_search_test.go#TestClientSearchTextOutputIsNotJSON"
        status: pass
    human_judgment: false
  - id: D7
    description: "An empty search result exits 0 and emits memories:[] — never null"
    requirement: "REQ-cli-agent-output"
    verification:
      - kind: integration
        ref: "cmd/engram/client_search_test.go#TestClientSearchEmptyResultIsEmptyArray"
        status: pass
    human_judgment: false
  - id: D8
    description: "One shared connect.CodeOf mapper drives the D-09 exit-code taxonomy
      (auth/permission->3, not-found->4, invalid-argument family->2, unavailable family->5,
      everything else->1) for all 16 connect.Code values plus a non-connect error"
    requirement: "REQ-cli-agent-output"
    verification:
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestExitCodeForConnectErrTable"
        status: pass
      - kind: integration
        ref: "cmd/engram/client_search_test.go#TestClientSearchExitCodeAuth"
        status: pass
      - kind: integration
        ref: "cmd/engram/client_search_test.go#TestClientSearchExitCodeNotFound"
        status: pass
      - kind: integration
        ref: "cmd/engram/client_search_test.go#TestClientSearchExitCodeInvalidArgument"
        status: pass
      - kind: integration
        ref: "cmd/engram/client_search_test.go#TestClientSearchExitCodeTransport"
        status: pass
    human_judgment: false
  - id: D9
    description: "Execute() consults an error's ExitCode() via errors.As and exits with that
      code; a plain error (every pre-existing operator command) still exits 1 byte-for-byte"
    requirement: "REQ-cli-agent-output"
    verification:
      - kind: unit
        ref: "cmd/engram/root_test.go#TestExitCodeFromError"
        status: pass
      - kind: e2e
        ref: "built binary: `no-server-url-exit=2`, `closed-port-exit=5` (real os.Exit, not an in-process assertion)"
        status: pass
    human_judgment: false
  - id: D10
    description: "No client_*.go implementation file imports internal/store, internal/authz,
      internal/embed, internal/server, or internal/config; the allowlist itself contains no
      internal/ path"
    requirement: "REQ-cli-client-commands"
    verification:
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestClientFilesImportBoundary"
        status: pass
    human_judgment: false
  - id: D11
    description: "No client code path reads standard input; no invocation can block on a prompt"
    requirement: "REQ-cli-agent-output"
    verification:
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestNoClientPathReadsStandardInput"
        status: pass
    human_judgment: false
  - id: D12
    description: "The resolved bearer credential never appears in stdout, stderr, or a returned
      error's Error() string, on the success, auth-failure, and transport-failure paths"
    requirement: "REQ-cli-credential-safety"
    verification:
      - kind: integration
        ref: "cmd/engram/client_common_test.go#TestTokenNeverAppearsInOutput"
        status: pass
    human_judgment: false
  - id: D13
    description: "No command accepts positional data (structural second guard against a
      credential riding in as a bare word)"
    requirement: "REQ-cli-credential-safety"
    verification:
      - kind: integration
        ref: "cmd/engram/client_common_test.go#TestClientCommandsAcceptNoPositionalArgs"
        status: pass
    human_judgment: false

duration: ~40min
completed: 2026-07-31
status: complete
---

# Phase 2 Plan 1: Headless CLI Client Tracer Summary

**`engram search` end-to-end over a real in-process Connect server: bearer credential resolved
from env/file (never argv), TLS-verifying transport, a D-10 `connect.CodeOf` exit-code mapper
wired through `Execute()`'s new `ExitCode()` dispatch, and JSON/table rendering split cleanly
across stdout/stderr.**

## Performance

- **Duration:** ~40 min
- **Tasks:** 2/2 completed
- **Files modified:** 7 (5 created, 2 modified)
- **Commits:** 2 task commits + this metadata commit

## Accomplishments
- `engram search` is a real, working CLI command: resolves `--server`/`ENGRAM_SERVER_URL`,
  resolves `ENGRAM_TOKEN`/`--token-file`, builds a TLS-verifying Connect client with a bearer
  interceptor, calls `SearchMemories`, and renders JSON (default off-TTY) or a table
  (`--output=text` or on a TTY).
- `Execute()` in `root.go` now exits with the code carried by a `cliError`'s `ExitCode()`
  accessor (via `errors.As`) instead of hard-coding 1 for every failure — proven at the binary
  level: a missing server URL exits 2, a closed-port dial exits 5, with every pre-existing
  operator command (`serve`, `reindex`, `prune-expired`, …) still exiting 1 on error
  (`TestExitCodeFromError`'s plain-error case is the compatibility proof).
- The reusable `clienttest_test.go` harness (`stubEngramService`, `startStubServer`,
  `resetClientFlags`, `runClient`) mounts the *real* generated `engramv1connect` handler over
  `httptest.NewServer` — not a hand-written stub — so the client's codec, header handling, and
  error wrapping are all genuinely exercised. Plans 02 and 03 reuse this harness unchanged.
- The client's four structural boundaries (no forbidden imports, no `--token` flag, TLS-off only
  behind an explicit warned flag, no stdin reads) are machine-enforced by negative/structural
  tests, each proven capable of firing red before being trusted (see RED Observations below).

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end `engram search` against a real Connect server** - `7f3ee9cc` (feat, tdd)
2. **Task 2: The client's boundaries are asserted, not documented** - `d16e2733` (test, tdd)

**Plan metadata:** committed alongside this SUMMARY (see final commit below)

_Note: both tasks were TDD — Task 1's tracer test (`TestClientSearchEndToEndJSON`) was observed
RED against a stub `RunE` before the real implementation was written; Task 2 added only
assertions (no new production behavior) and required four separate RED observations, one per
gate, each performed as a temporary local edit, run, and revert before this commit._

## Files Created/Modified
- `cmd/engram/client_common.go` - shared client foundation: flag binding, server-URL/token
  resolution, bearer interceptor, TLS transport, TTY-aware output-format resolution, the D-10
  `exitCodeForConnectErr` mapper, `cliError`/`usageErrorf`/`wrapRPCError`, `renderJSON`,
  `renderMemoryTable`
- `cmd/engram/client_search.go` - `engram search`: flags mirror `SearchMemoriesRequest` 1:1,
  `RunE` validates `--query`, resolves output format, builds the client, issues one RPC call,
  renders, never retries
- `cmd/engram/clienttest_test.go` - `stubEngramService` (real generated Connect handler wrapper),
  `startStubServer`, `resetClientFlags`, `runClient` — the shared harness
- `cmd/engram/client_common_test.go` - `TestExitCodeForConnectErrTable` (all 16 `connect.Code`
  values + non-connect error), `TestResolveOutputFormat`, `TestIsTerminalOnNonTTY`, plus Task 2's
  ten negative/structural gates
- `cmd/engram/client_search_test.go` - 13 integration tests covering the full JSON/text/auth/
  bearer/exit-code/empty-result surface
- `cmd/engram/root.go` - `Execute()` now calls `os.Exit(exitCodeFromError(err))`; added
  `exitCodeFromError`
- `cmd/engram/root_test.go` - added `TestExitCodeFromError`

## Decisions Made
- `resolveServerURL` resolves at run time rather than baking `os.Getenv("ENGRAM_SERVER_URL")`
  into the flag's default (the idiom `reindex.go` uses for `--target`). This is a deliberate,
  commented divergence from the established repo pattern: baking the env read into the flag
  default happens at `init()` time, which is untestable with `t.Setenv` and leaks the resolved
  env value into `--help` output. Run-time resolution in `resolveServerURL`/`resolveToken` keeps
  D-02/D-13 precedence testable and `--help` output stable.
- The import-boundary gate (`TestClientFilesImportBoundary`) walks `client_*.go` files with
  `go/parser`'s `ImportsOnly` mode rather than `go list -json ./cmd/engram/...`, because the
  package-level dependency set legitimately includes `internal/store` (via `reindex.go`,
  `prune.go`, etc. in the same package) — a package-wide gate would be a permanent false RED.
- `renderJSON` uses `protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}` so
  field names and the empty-array-not-null behavior are derived structurally from the `.proto`
  message rather than risking drift in a hand-written struct.

## Deviations from Plan

None — plan executed exactly as written. Two minor build-fix iterations were needed and are
recorded here for completeness, both squarely Rule 1 (bug in freshly-written code, fixed before
the task's own commit — not a deviation from the plan's design):

**1. [Rule 1 - Lint] `errcheck` failures on unchecked `fmt.Fprintln`/`fmt.Fprintf` writes**
- **Found during:** Task 1, running `task lint` as part of the plan's own verification gate
- **Issue:** the `--insecure` warning write and the four `tabwriter` writes in
  `renderMemoryTable` discarded their `(int, error)` return, which `golangci-lint`'s `errcheck`
  linter (already configured in this repo) flags
- **Fix:** the warning write uses `_, _ = fmt.Fprintln(...)`; `renderMemoryTable` was restructured
  around a small `writeLine` closure that captures the first write error and short-circuits
  further writes, returned before the `tw.Flush()` call
- **Files modified:** `cmd/engram/client_common.go`
- **Verification:** `task lint` clean; `go test ./cmd/engram/...` unaffected
- **Committed in:** `7f3ee9cc` (part of Task 1's commit — the file had not been committed yet)

**2. `gofmt` reformatted `client_common_test.go`'s `allowedClientImports` map**
- **Found during:** Task 2, running `task fmt`/`gofmt -l` as part of verification
- **Issue:** the map literal's `:` alignment didn't match gofmt's canonical column width
- **Fix:** ran `gofmt -w cmd/engram/client_common_test.go`
- **Files modified:** `cmd/engram/client_common_test.go`
- **Committed in:** `d16e2733` (part of Task 2's commit — pre-commit formatting pass, not a
  separate commit)

**Out-of-scope drift declined:** `task fmt`'s `dprint` step also reformatted
`.claude/settings.json`, `docs-site/package.json`, `internal/webauth/static/_app/version.json`,
and `ui/tsconfig.json` — pre-existing formatting drift unrelated to this plan's files. Reverted
with `git checkout -- <those four paths>` per the scope-boundary rule (only fix issues directly
caused by this plan's changes) before either task commit.

---

**Total deviations:** 0 (two same-task lint/format fixes folded into their originating commits;
not tracked as separate deviations since neither changed the plan's design)

## RED Observations (required, quoted verbatim)

**Task 1 tracer RED** — `TestClientSearchEndToEndJSON` against a `searchCmd` whose `RunE`
returned `nil` without calling the RPC:
```
=== RUN   TestClientSearchEndToEndJSON
    client_search_test.go:50: stdout did not unmarshal as a single JSON object: unexpected end of JSON input
        stdout=""
--- FAIL: TestClientSearchEndToEndJSON (0.00s)
FAIL
FAIL	github.com/seanb4t/engram/cmd/engram	0.543s
FAIL
```

**Task 2, gate 1/4** — blank `_ "github.com/seanb4t/engram/internal/store"` import added to
`client_common.go`, confirming `TestClientFilesImportBoundary` fails on both clause 1 (allowlist
membership) and clause 3 (denylist):
```
=== RUN   TestClientFilesImportBoundary
    client_common_test.go:201: REQ-cli-client-commands/D-04: import "github.com/seanb4t/engram/internal/store" in a client_*.go production file is not in allowedClientImports
    client_common_test.go:223: REQ-cli-client-commands/D-04: client_*.go imports "github.com/seanb4t/engram/internal/store", which no client implementation file may import
--- FAIL: TestClientFilesImportBoundary (0.00s)
FAIL
```

**Task 2, gate 2/4** — a temporary `--token` string flag registered on `searchCmd`, confirming
`TestNoTokenFlagAnywhere` fails:
```
=== RUN   TestNoTokenFlagAnywhere
    client_common_test.go:239: command "engram search" declares a --token flag
--- FAIL: TestNoTokenFlagAnywhere (0.00s)
FAIL
```

**Task 2, gate 3/4** — the `--insecure` warning temporarily also written to `cmd.OutOrStdout()`
(prepended ahead of the JSON payload, alongside the existing stderr write), confirming
`TestInsecureWarnsOnStderrAndStdoutStaysJSON` fails on the whole-stdout JSON-parse assertion (not
merely a `strings.Contains` check, which would have missed this):
```
=== RUN   TestInsecureWarnsOnStderrAndStdoutStaysJSON
    client_common_test.go:297: stdout did not unmarshal in its entirety as one JSON object: invalid character 'W' looking for beginning of value
        stdout="WARNING: TLS certificate verification is disabled (--insecure); do not use against an untrusted network\n{\"memories\":[]}\n"
--- FAIL: TestInsecureWarnsOnStderrAndStdoutStaysJSON (0.00s)
FAIL
```

**Task 2, gate 4/4** — `strings.TrimSpace` temporarily dropped from `resolveToken`'s file-read
path, confirming `TestTokenFileTrailingNewlineTrimmed` fails:
```
=== RUN   TestTokenFileTrailingNewlineTrimmed
    client_common_test.go:425: resolveToken(file with trailing newline) = "abc123\n", want "abc123"
--- FAIL: TestTokenFileTrailingNewlineTrimmed (0.00s)
FAIL
```

All five temporary edits above were reverted immediately after their RED observation; `git diff`
against each file was empty before the corresponding task's real commit.

## Issues Encountered

None beyond the lint/format fixes documented under Deviations above. `ssh-add -T` confirmed the
1Password SSH-signing agent was live before either commit; both commits carry a real SSH
signature (`git cat-file commit` shows a `gpgsig` block on each — `git log --show-signature`
reports "No signature" locally only because this checkout lacks
`gpg.ssh.allowedSignersFile`, a verification-side config gap, not a signing failure).

## User Setup Required

None — no external service configuration required. (A reachable engram server with the Connect
lane mounted is required for real-world manual use of `engram search`, per `02-RESEARCH.md`'s
Environment Availability table, but all automated verification in this plan runs against an
in-process `httptest` server wrapping the real generated Connect handler.)

## Next Phase Readiness

Plan 02-02 (`list`, `store`) and 02-03 (self-describe catalog, `rootCmd.RunE`) can build directly
on `clientFromFlags`, `resolveOutputFormat`, `exitCodeForConnectErr`, `renderJSON`,
`renderMemoryTable`, and the `clienttest_test.go` harness — none of that surface needs to change
shape for the remaining two subcommands or the D-15 bare-invocation catalog. No blockers.

---
*Phase: 02-headless-cli-client*
*Completed: 2026-07-31*

## Self-Check: PASSED

All 7 claimed files verified present on disk; both task commits (`7f3ee9cc`, `d16e2733`)
verified present in `git log --oneline --all`.
