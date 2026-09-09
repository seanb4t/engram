---
phase: 03-runtime-registration
verified: 2026-09-09T17:03:30Z
status: human_needed
score: 5/5 must-haves verified
covered_files:
  - .planning/REQUIREMENTS.md
  - .planning/phases/03-runtime-registration/03-01-PLAN.md
  - .planning/phases/03-runtime-registration/03-01-SUMMARY.md
  - .planning/phases/03-runtime-registration/03-02-PLAN.md
  - .planning/phases/03-runtime-registration/03-02-SUMMARY.md
  - .planning/phases/03-runtime-registration/03-03-PLAN.md
  - .planning/phases/03-runtime-registration/03-03-SUMMARY.md
  - .planning/phases/03-runtime-registration/03-04-PLAN.md
  - .planning/phases/03-runtime-registration/03-04-SUMMARY.md
  - .planning/phases/03-runtime-registration/03-05-PLAN.md
  - .planning/phases/03-runtime-registration/03-05-SUMMARY.md
  - .planning/phases/03-runtime-registration/03-REVIEW.md
  - cmd/engram/setup.go
  - internal/keylinks/keylinks.go
  - internal/setup/apply.go
  - internal/setup/claudecode.go
  - internal/setup/codex.go
  - internal/setup/environment.go
  - internal/setup/generic.go
  - internal/setup/opencode.go
  - internal/setup/plan.go
  - internal/setup/quote.go
  - internal/setup/runtime.go
  - internal/surfaces/toolclass.go
covered_digest: "v1:sha256:56b4bf6da36b0accb0e27260d52f4bfda943f0957e56147f87c7151431d0a71f"
behavior_unverified: 0
overrides_applied: 0
behavior_unverified_items: []
human_verification:
  - test: "Run `engram setup --apply --runtime claude-code` twice in a row against a real Claude Code install with no prior engram registration, then a third time after manually deleting the entry."
    expected: "Run 1: outcome=wrote. Run 2 (state unchanged): outcome=already-correct. Between runs 1 and 2 there is a real (tolerant-remove-then-fatal-add) window where the registration briefly does not exist — this is by design, not a bug."
    why_human: "Repo rule m45p2b4bp7 forbids any test in this repo from invoking a real third-party CLI; the mechanism is proven with a scripted fake (TestApplyConvergesClaudeCode) and I independently confirmed the live read-side (`claude mcp get`) and command construction against my own real claude/codex/opencode installs during this verification, but a real two-invocation --apply round trip against a live claude-code install was intentionally not run here because --apply is destructive against the verifier's own machine state."
  - test: "Run `engram setup --apply --runtime codex` twice, then `engram setup --apply --runtime opencode` twice, against real installs."
    expected: "codex: run 1 wrote, run 2 already-correct (its `mcp add` overwrites silently and `mcp get --json` is a pure local read, so already-correct should be the common case). opencode: run 2 is expected to report wrote far more often than already-correct, because `opencode mcp list` dials every registered server live and any one flip differs the two probe captures — this is documented as the safe-direction-only degradation, not a defect."
    why_human: "Same repo rule as above — no test may invoke a real third-party binary. Verified via scripted fakes (TestApplyConvergesCodex, TestApplyOpenCodeConvergence) and via live read-only preview probes against my real installs (see Behavioral Spot-Checks); the live --apply round trip itself was not run to avoid mutating the verifier's own MCP registrations."
  - test: "Point --runtime at a claude/codex/opencode binary that has since removed or renamed a flag this package depends on (e.g. an intentionally broken PATH entry pointing at a wrapper script that rejects `--transport` or `--bearer-token-env-var`), then run `engram setup --apply`."
    expected: "outcome=failed with Reason naming the runtime, the exact argv issued, the nonzero exit code, and the runtime's stderr verbatim — never a silent no-op."
    why_human: "The mechanism is unit-tested end-to-end (TestDriftReportedLegibly, both with and without stderr) against a scripted fake; reproducing an actual drifted third-party flag surface requires a real modified binary, which the repo rule above precludes fabricating as an automated test."
---

# Phase 03: Runtime Registration Verification Report

**Phase Goal:** `engram setup --apply` registers engram with Claude Code, Codex, and opencode by
invoking each runtime's own CLI — never by touching their config files — covers every auth mode
engram deploys behind (or states plainly what's unsupported) without ever placing a secret on a
command line, offers a portable config for unsupported clients, fails legibly on an absent or
drifted CLI rather than silently writing nothing, and converges to a distinctly-reported
"already correct" on a second `--apply`.

**Verified:** 2026-09-09T17:03:30Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth (ROADMAP success criterion) | Status | Evidence |
|---|---|---|---|
| 1 | `--apply` registers with Claude Code/Codex/opencode by invoking their own CLI, never by touching config files | ✓ VERIFIED | `internal/setup/{claudecode,codex,opencode}.go` author only `Args` argv for `claude`/`codex`/`opencode` binaries; `internal/setup/apply.go` execs via `Environment.Run`/`env.LookPath` only. `rg -n "os\.(ReadFile\|Open\|Stat)"` across `internal/setup/` matches only `leafpurity_test.go` (a go.mod locator, unrelated to any runtime config file). Live-confirmed: ran the built binary's bare preview against my real `claude`/`codex`/`opencode` installs — each row's `command`/`registered` field reflects the real CLI's own state (see Behavioral Spot-Checks), and `go list -deps ./internal/setup` shows zero non-stdlib, non-same-module imports (leaf-purity holds). |
| 2 | For an MCP client engram doesn't natively support, `setup` prints a portable, pasteable server config | ✓ VERIFIED | `internal/setup/generic.go`'s `Plan()` returns zero `Actions`/`Probe`, only `Plan.Config` (minified JSON `{"mcpServers":{"engram":{...}}}`). Live-ran `engram setup --url ... --runtime generic --output json`: emitted `{"runtimes":[{"name":"generic","present":true,"outcome":"would-write","config":"{\"mcpServers\":{\"engram\":{\"type\":\"http\",\"url\":\"https://engram.example.com/mcp\"}}}"}]}` — valid, pasteable, single-line JSON. Bare `engram setup` (no `--runtime`) omits the generic row entirely (confirmed live and by `TestSetupBareInvocationExcludesGeneric`), matching `Select`'s `optInOnlyRuntime` exclusion. |
| 3a | Every registration path covers OAuth / pre-registered OAuth client / bearer / none, or states plainly which are unsupported | ✓ VERIFIED | `claudecode.go`/`codex.go`/`opencode.go`/`generic.go` each switch on all four modes; opencode's `oauth-client` and generic's `oauth-client` return `fmt.Errorf(..., ErrAuthModeUnsupported)` naming the runtime and mode. Live-confirmed: `setup --runtime opencode --auth oauth-client` (preview and `--apply`) both produced `outcome=failed reason="opencode: auth mode \"oauth-client\": setup: auth mode is not supported by this runtime"` — preview exited 0, `--apply` exited 9 (`ExitTotalFailure`). |
| 3b | No secret is ever placed on a command line | ✓ VERIFIED | `rg -n "TokenFile"` across `internal/setup`/`cmd/engram` shows the path is only ever rendered as provenance (`bearerProvenance`) or ignored (`tokenFileIgnoredMarker`) — never opened/read (no `os.ReadFile`/`os.Open`/`os.Stat` call site touches it outside the go.mod-locating test helper). Live-ran `setup --auth bearer` (no `--apply`): claude-code's argv carries `--header 'Authorization: Bearer ${ENGRAM_TOKEN}'`, codex's carries `--bearer-token-env-var ENGRAM_TOKEN`, opencode's carries `--header 'Authorization=Bearer {env:ENGRAM_TOKEN}'` — every one a variable NAME/reference, never a literal credential value or file path. `TestPlanBearerNeverReadsTokenFile` pins the negative assertion with a nonexistent path. |
| 4 | Absent CLI or unexpected flag surface fails legibly, naming the runtime and what was expected — never a silent no-op | ✓ VERIFIED | Absent: `Detect()` false → `Result{Present:false, Outcome:"not-present"}`, an explicit per-runtime row, never omitted from the report. Live-confirmed with `PATH=/nonexistent`: JSON report explicitly listed all three runtimes as `"present":false,"outcome":"not-present"` (exit 0 — by design, D-07: absence of an optional runtime is not a whole-command failure). Flag-surface drift: any nonzero exit from a non-tolerant action/probe seam produces `OutcomeFailed` with `Reason` built from the runtime name + rendered argv + exit code + bounded stderr (`describeFailure`/`describeSeamError`), pinned by `TestDriftReportedLegibly`'s two subtests (with and without stderr) — never a silent no-op. |
| 5 | Running `--apply` twice converges, and reports "already correct" distinctly from "wrote it" | ✓ VERIFIED | `internal/setup/apply.go`'s `execute()` byte-compares raw probe reads #1/#2 (step 9); `TestApplyConvergesCodex`, `TestApplyConvergesClaudeCode`, and `TestApplyOpenCodeConvergence` each drive first-run (`wrote`) then second-run (`already-correct`) through a scripted `Environment` fake and pass. Claude-code's premise correction (tolerant `mcp remove` then fatal `mcp add`, since `claude mcp add` refuses on an existing name at every scope with no force flag) is the mechanism that makes `already-correct` reachable at all for claude-code — confirmed present in `claudecode.go` and exercised by the named test. opencode's asymmetric degradation (an unrelated server's live-dialed status flip differs the two probe captures, so `already-correct` is expected to be rare in practice, never `wrote`-in-the-wrong-direction) is a documented, tested, safe-direction-only property (`TestApplyOpenCodeConvergence/unrelated-server-status-flip-still-wrote`), not an inconsistency. |

**Score:** 5/5 truths verified (0 present, behavior-unverified)

All five truths are behavior-dependent (state-transition/convergence claims) per the verification
process's definition. Each is backed by a named, passing test that exercises the actual transition
through a scripted `Environment` fake — never a real third-party binary, per repo rule m45p2b4bp7 —
plus, where safe (read-only), a live run of the built binary against my own real `claude`/`codex`/
`opencode` installs. A full mutating `--apply`-twice round trip against a real installed CLI was
deliberately NOT run during this verification (it would alter the verifier's own machine state) and
is listed under Human Verification instead, per this phase's explicit instruction that live
end-to-end convergence against real third-party CLIs is legitimately unverifiable from inside an
automated check.

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/setup/apply.go` | Shared executor: LookPath, Run, probe/byte-compare, D-11 failure legibility | ✓ VERIFIED | Present, substantive, wired from `cmd/engram/setup.go`'s `setupBuildRows`/`setupApplyRun`. CR-01 (panic on non-first empty-`Args` action) is fixed at lines 242-249, with regression test `TestEveryActionArgsValidated` passing. |
| `internal/setup/claudecode.go` | Two-action tolerant-remove-then-fatal-add per auth mode | ✓ VERIFIED | All four modes return the two-action shape; `claudeCodeRemoveAction.Tolerant=true` authored explicitly, never positional. |
| `internal/setup/codex.go` | Single non-tolerant `mcp add`, bearer via `--bearer-token-env-var` | ✓ VERIFIED | Matches; probe is `codex mcp get engram --json`. |
| `internal/setup/opencode.go` | Single non-tolerant `mcp add` with `KEY=VALUE` header form fix, no remove verb | ✓ VERIFIED | Header form is `Authorization=Bearer {env:ENGRAM_TOKEN}` (fixes the shipped colon-space bug); `oauth-client` returns `ErrAuthModeUnsupported`. |
| `internal/setup/generic.go` | Zero-Action, zero-Probe opt-in pseudo-runtime emitting portable JSON | ✓ VERIFIED | `OptInOnly()==true`; `Detect` unconditionally true but excluded from `Select(nil)`'s default set. |
| `internal/setup/quote.go` | D-02 minimal POSIX display quoting | ✓ VERIFIED | `quoteWord`/`quoteArgs`; safe-rune set matches must-have text exactly. |
| `internal/setup/runtime.go` | `Runtimes` registry, `Select`, `optInOnlyRuntime` predicate | ✓ VERIFIED | `Runtimes = []Runtime{ClaudeCode, Codex, OpenCode, Generic}`; default-set exclusion is structural (`optInOnlyRuntime`), not name-based. |
| `internal/surfaces/toolclass.go` | `setup` row comment states the real per-runtime asymmetry | ✓ VERIFIED | Comment names codex/opencode as silently overwriting and claude-code as refusing, matching `03-RESEARCH.md`; `Class` unchanged (`Destructive:true, Idempotent:true`). |
| `cmd/engram/setup.go` | Preview runs the probe (D-10); `--apply` calls the real executor; `token_file=ignored`/`registered=` rows | ✓ VERIFIED | `setupBuildRows`→`setup.Preview`, `setupApplyRun`→`setup.Apply`; no stub loop remains. Live-confirmed `token_file=ignored` marker and `registered=` field both render for real. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `cmd/engram/setup.go: setupApplyRun` | `internal/setup.Apply` | direct call, per selected runtime | ✓ WIRED | `go test ./...` green; live-ran `--apply` for a real unsupported-mode failure (opencode+oauth-client) and observed the exact reported error and exit code 9. |
| `cmd/engram/setup.go: setupPreview` | `internal/setup.Preview` | via `setupBuildRows` | ✓ WIRED | Same function underlies both preview and apply row-building (D-15's one-path invariant); live-confirmed identical `Registered`/`Command` shape in both lanes. |
| `internal/setup/apply.go: execute()` | `Environment.Run`/`env.LookPath` | `runSeam` | ✓ WIRED | Every exec goes through the injectable seam; no direct `exec.Command` call in `apply.go`. |
| `internal/setup/{claudecode,codex,opencode}.go: Plan()` | `internal/setup/apply.go: execute()` | shared executor, no per-runtime exec code | ✓ WIRED | Confirmed by reading `apply.go` end-to-end: no `if name == "claude-code"` (or similar) branch exists anywhere in the executor. |
| `internal/setup/generic.go: Plan()` | `internal/setup/apply.go: execute()` step 2a | zero-Actions early return | ✓ WIRED | `len(plan.Actions)==0` branch returns `OutcomeWouldWrite` immediately without touching `LookPath`/`Run`; live-confirmed generic never starts a process (no `binary=`/`registered=` field on its row). |

### Requirements Coverage

| Requirement | Source Plan(s) | Status | Evidence |
|---|---|---|---|
| REQ-setup-idempotent | 03-01, 03-02, 03-03, 03-05 | ✓ SATISFIED | Byte-compare convergence in `apply.go`; three named convergence tests (codex/claude-code/opencode) all pass. |
| REQ-register-claude-code | 03-02 | ✓ SATISFIED | `claudecode.go`; live-confirmed real `claude mcp add`/`remove`/`get` invocation shape. |
| REQ-register-codex | 03-01 | ✓ SATISFIED | `codex.go`; live-confirmed real `codex mcp add`/`get --json` invocation shape. |
| REQ-register-opencode | 03-03 | ✓ SATISFIED | `opencode.go`; live-confirmed real `opencode mcp add`/`list` invocation shape, including the `KEY=VALUE` header fix. |
| REQ-register-generic-mcp | 03-04 | ✓ SATISFIED | `generic.go`; live-confirmed pasteable JSON output and default-set exclusion. |
| REQ-register-auth-modes | 03-01 – 03-05 | ✓ SATISFIED | All four modes handled or explicitly unsupported per runtime; no literal credential ever in `Args` (checked source-wide). |
| REQ-register-cli-surface-drift-legible | 03-01, 03-05 | ✓ SATISFIED | `describeFailure`/`describeSeamError`; `TestDriftReportedLegibly`; live-confirmed absent-CLI reporting via `PATH=/nonexistent`. |

No orphaned requirements: every ID `grep`'d against `Phase 3` in `.planning/REQUIREMENTS.md` (7 IDs)
appears in at least one of the five plans' `requirements:` frontmatter, and every plan's declared
requirement IDs appear in `.planning/REQUIREMENTS.md`.

### Anti-Patterns Found

None. `rg -n "TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER|not yet implemented|coming soon"` across every
file this phase modified (`internal/setup/*.go`, `cmd/engram/setup.go`,
`internal/surfaces/toolclass.go`) returns zero matches. `setup.ErrApplyNotImplemented` and
`setupApplyStubReason` no longer exist anywhere in the tree (confirmed by `rg`).

The one Critical finding from `03-REVIEW.md` (CR-01: panic on a non-first empty-`Args` action) is
already fixed on `main` (commit `ae1ebc39`, preceded by RED commit `9e05c5cc`), with a passing
regression test (`TestEveryActionArgsValidated`). The four Warnings and one Info in `03-REVIEW.md`
are lower-severity robustness/defense-in-depth notes, not goal-blocking — not duplicated here per
this task's instructions.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| `go build ./...` | `go build ./...` | clean, no output | ✓ PASS |
| `go test ./...` (run once, full suite) | `go test ./...` | all packages `ok` | ✓ PASS |
| `task lint` | `task lint` | "All checks passed!" | ✓ PASS |
| Convergence mechanism, all four runtimes | `go test ./internal/setup/... -run 'TestApplyConverges(Codex\|ClaudeCode)\|TestApplyOpenCodeConvergence'` | all subtests PASS | ✓ PASS |
| CR-01 regression | `go test ./internal/setup/... -run TestEveryActionArgsValidated` | PASS | ✓ PASS |
| Live: bare preview against real claude-code/codex/opencode installs | built binary, `setup --url ... --output json` | real `command`/`registered` fields reflecting actual machine state (e.g. claude-code showed a real existing "engram" registration; codex showed "No MCP server named 'engram' found"; opencode live-dialed 3 unrelated registered servers) | ✓ PASS (read-only; no `--apply` run against real installs) |
| Live: generic portable config | built binary, `setup --url ... --runtime generic --output json` | single-line pasteable `{"mcpServers":{"engram":{"type":"http","url":"..."}}}` | ✓ PASS |
| Live: bearer mode argv, all three native runtimes | built binary, `setup --url ... --auth bearer --output json` | env-var-reference forms only, no credential value | ✓ PASS |
| Live: unsupported mode (opencode + oauth-client) | built binary, `setup --runtime opencode --auth oauth-client [--apply]` | preview: `outcome=failed`, exit 0; apply: same reason, exit 9 | ✓ PASS |
| Live: absent CLI | built binary, `PATH=/nonexistent setup --url ...` | all three runtimes reported `outcome=not-present`, exit 0 | ✓ PASS |
| `internal/setup` leaf purity | `go list -deps ./internal/setup` | only stdlib + itself | ✓ PASS |

### Probe Execution

Not applicable — no `scripts/*/tests/probe-*.sh` files exist in this repository and neither the
plans nor the review reference probe scripts.

## Human Verification Required

1 through 3 above (frontmatter `human_verification`): full mutating `--apply`-twice round trips
against real installed `claude`/`codex`/`opencode` CLIs, and a real drifted-flag-surface CLI. All
three are legitimately unrunnable from inside this repo's own test suite (repo rule m45p2b4bp7) and
were deliberately not simulated by mutating the verifier's own machine state. The convergence
*mechanism* is proven with scripted fakes that mirror the exact exit codes/stdout/stderr
`03-RESEARCH.md` recorded from live probing, and the read-only halves (probe output, argv
construction, unsupported-mode/absent-CLI reporting) were independently live-confirmed against my
own real installs during this verification.

## Gaps Summary

None. Every ROADMAP success criterion has direct, positive evidence in the current codebase — not
merely a SUMMARY.md claim — backed by passing named tests and, where safe, live runs of the built
binary against real installed runtime CLIs. The phase's own tail work (the `internal/keylinks`
malformed-shape fix and the CR-01 panic-guard hoist) is present, scoped as claimed, and covered by
passing regression tests. Status is `human_needed` rather than `passed` solely because a full
mutating `--apply`-twice round trip against real third-party CLIs cannot be run inside this repo's
own automated verification — not because any check failed.

---

_Verified: 2026-09-09T17:03:30Z_
_Verifier: Claude (gsd-verifier)_
