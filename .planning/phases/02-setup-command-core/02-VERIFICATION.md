---
phase: 02-setup-command-core
verified: 2026-08-30T14:05:49Z
status: gaps_found
score: 14/16 must-haves verified
behavior_unverified: 0
overrides_applied: 0
gaps:
  - truth: "`engram setup --help`'s advertised `(default: ENGRAM_URL)` / `(default: ENGRAM_AUTH)` behavior is accurate — setting the env var with no matching flag actually changes the previewed command (REQ-setup-correct-by-reading)."
    status: failed
    reason: >
      Confirmed empirically (CR-01, independently reproduced during this verification):
      `ENGRAM_URL="https://env-url.example.com/mcp" ENGRAM_AUTH="bearer" go run ./cmd/engram setup --output json`
      emits `"command":"codex mcp add engram --url "` (URL absent, bearer form never triggered) and
      `"command":"claude mcp add --transport http engram  --scope user"` (double space where the URL
      belongs). `cmd/engram/setup.go` never calls `config.Load`; `--url`/`--auth` are bound only via
      `config.FlagDefault(...)`, a static registry-default lookup that never consults the environment.
      `internal/config.Config.Setup` (`SetupConfig{URL,Auth}`) and the `setup.url`/`setup.auth` registry
      rows are dead code — `rg "\.Setup\b|SetupConfig"` finds only the declaration site. This directly
      contradicts the flag help text's own claims and the registry's stated design intent, and no test
      anywhere sets `ENGRAM_URL`/`ENGRAM_AUTH` and asserts on the resulting command.
    artifacts:
      - path: "cmd/engram/setup.go"
        issue: "init() binds --url/--auth via config.FlagDefault only; setupPlanDoc never calls config.Load, so ENGRAM_URL/ENGRAM_AUTH are never consulted despite being advertised in --help"
      - path: "internal/config/registry.go"
        issue: "setup.url (Env: ENGRAM_URL) and setup.auth (Env: ENGRAM_AUTH) rows exist but nothing reads Config.Setup, making the Env field's promise false"
      - path: "internal/config/config.go"
        issue: "SetupConfig{URL,Auth} and Config.Setup are declared but never referenced outside their own definition"
    missing:
      - "Route --url/--auth through config.Load(cmd.Flags()) in setupPlanDoc, mirroring client_common.go/serve.go, so Config.Setup.URL/Auth actually reach Plan() when only the env var is set"
      - "Add regression coverage: t.Setenv(\"ENGRAM_URL\", ...) / t.Setenv(\"ENGRAM_AUTH\", ...) with no matching flag, asserting the emitted command reflects the env value"
      - "Alternatively, if env support for --url/--auth is genuinely out of scope this phase, delete SetupConfig and the two registry rows and drop the false '(default: ENGRAM_*)' help suffixes, rather than leaving a documented guarantee the code does not keep"
---

# Phase 2: Setup Command Core Verification Report

**Phase Goal:** `engram setup` exists as a real, preview-by-default, fully scriptable CLI command
that can detect which supported agent runtimes are present on the machine and report what it
would write to each — before it writes anything.

**Verified:** 2026-08-30T14:05:49Z
**Status:** gaps_found
**Re-verification:** No — initial verification

## Goal Achievement

All verification below was run against the actual built binary (`go run ./cmd/engram ...`) at
HEAD (`2d64363c`), not inferred from source reading alone, per the task's instruction.

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Bare `engram setup` reports one row per runtime with presence + exact command, creates/modifies/deletes zero files | ✓ VERIFIED | `go run ./cmd/engram setup --output json --url https://x.example.com/mcp` emitted a 3-row `runtimes` array; `find` diff before/after in a scratch dir showed zero filesystem changes |
| 2 | Two consecutive preview runs produce byte-identical stdout on both text and json lanes | ✓ VERIFIED | `diff` of two consecutive JSON-lane runs and two consecutive text-lane (`--output text`) runs both empty |
| 3 | Two preview processes run concurrently without interference; no lock, no partial artifact | ✓ VERIFIED | Two backgrounded `setup --url a...`/`setup --url b...` invocations each returned their own URL with no cross-contamination; `rg` for `flock\|Lock()\|sync.Mutex\|os.Create\|os.WriteFile` in `cmd/engram/setup.go` and `internal/setup/*.go` found none |
| 4 | `internal/setup.Outcome` carries five distinct first-class values; `not-present` is never a zero value | ✓ VERIFIED | `internal/setup/plan.go` declares `OutcomeNotPresent`/`OutcomeWouldWrite`/`OutcomeAlreadyCorrect`/`OutcomeWrote`/`OutcomeFailed` as explicit non-empty string constants; `exit.go`'s `Classify` routes the zero-value `""` to the failure branch, confirmed by `TestClassifyRejectsZeroValueOutcome` |
| 5 | `engram setup --help` is a pure read: byte-identical across runs, writes nothing | ✓ VERIFIED | `diff` of two consecutive `--help` invocations empty |
| 6 | `engram setup --help` acquires no lock, touches no shared state | ✓ VERIFIED | Same lock/write grep as truth 3 — help path shares the same command tree, no separate I/O |
| 7 | Detection consults only `exec.LookPath` via the injectable `Environment`; no config-directory read | ✓ VERIFIED | Source inspection: `Detect()` in `claudecode.go`/`codex.go`/`opencode.go` each call only `env.LookPath(...)`; `TestDetectIgnoresConfigDirectory` (in `internal/setup/detect_test.go`) passes and asserts a fake with an existing `~/.claude/`-shaped HomeDir but failing LookPath still reports absent |
| 8 | `engram setup --output json --runtime <name>` completes with no TTY, emits parseable JSON | ✓ VERIFIED | Ran non-interactively (no TTY attached via `go run` redirected to file); JSON parsed cleanly with `node` |
| 9 | `engram setup --auth <bad-value>` exits 2, names all four accepted modes | ✓ VERIFIED | `engram setup --auth basic` → `Error: --auth "basic": must be "oauth", "oauth-client", "bearer", "none", or empty`, `exit status 2` |
| 10 | Two concurrent `engram setup --apply` runs cannot produce a duplicated registration | ⚠️ (backstop, insufficient_spec) | Marked `verification: backstop` in PLAN frontmatter; `Apply()` is intentionally stubbed this phase (`ErrApplyNotImplemented`, D-09) so this property has no live code path to exercise yet — abstained per the backstop-tag rule rather than inferred from presence |
| 11 | `engram setup` publishes exit codes 8 (partial) and 9 (total failure), both advertised in the catalog with non-empty meaning | ✓ VERIFIED | `go run ./cmd/engram` (bare self-describe) `exit_codes` array contains code 8 ("at least one runtime was registered successfully and at least one failed...") and code 9 ("every runtime engram setup attempted failed...") |
| 12 | A runtime that is not installed contributes to neither exit code; all-`not-present` exits 0 | ✓ VERIFIED | `internal/setup/exit_test.go`'s `TestClassifyBoundaryCases` pins `[not-present, not-present, not-present] -> ExitTotalSuccess` and `[not-present, wrote] -> ExitTotalSuccess`; `cmd/engram/setup_test.go`'s `TestSetupApplyAllAbsentExitsZero` exercises it through a fake `Environment` |
| 13 | Exit classification is a pure function, exhaustively tested, no I/O | ✓ VERIFIED | `Classify`'s body contains no `os`/`net`/`context` import (leaf-purity gate passes); `TestClassifyExhaustiveOutcomeCombinations` enumerates every non-empty multiset of the five outcomes up to length 3, generated not transcribed — ran and passed |
| 14 | Preview exits nonzero only for a usage/config error; a missing-runtime report still exits 0 | ✓ VERIFIED | `setupPreview` contains no call to `setupExitCode` (confirmed by reading); `engram setup --auth basic` (config error) exits 2, while a real-machine preview with all runtimes present/absent exits 0 in every fake-driven test |
| 15 | Mixed outcomes are reported per-runtime, never aggregated or discarded | ✓ VERIFIED | `engram setup --apply --output json` on this machine (all three present) emitted three distinct `failed` rows, each with its own `reason`; `renderOperator` runs unconditionally before the nonzero exit is returned |
| 16 | `engram setup --help`'s advertised `(default: ENGRAM_URL)` / `(default: ENGRAM_AUTH)` claims are accurate (REQ-setup-correct-by-reading) | ✗ FAILED | See gap below (CR-01) — confirmed empirically that these environment variables have zero effect |

**Score:** 14/16 truths verified (1 backstop-abstained, 1 failed)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/setup/plan.go` | `Outcome`/`Action`/`Plan`/`Result` types | ✓ VERIFIED | Five-value `Outcome` enum present, wired |
| `internal/setup/environment.go` | Injectable `Environment` seam | ✓ VERIFIED | Struct-of-func-fields, `OSEnvironment` package value; `Getenv`/`HomeDir` unused by any `Runtime` this phase (IN-02, info-level, not blocking) |
| `internal/setup/runtime.go` | `Runtime` interface, `Runtimes` registry, `Names`, `Select` | ✓ VERIFIED | Three-element literal, no `init()`; `Select` does not dedupe repeated names (WR-02, warning-level, not blocking — see below) |
| `internal/setup/claudecode.go`, `codex.go`, `opencode.go` | Three structurally identical Runtime impls | ✓ VERIFIED | All twelve (runtime, auth-mode) cells authored; `TestPlanAuthModes` (12 subtests) all pass |
| `cmd/engram/setup.go` | `setup` command, preview/apply closures | ✓ VERIFIED | Routes through `registerDestructive`, no own `RunE`; flag named `url` (not `server`) confirmed via `--help` |
| `cmd/engram/setup_test.go`, `operator_view_setup_test.go` | Test coverage | ✓ VERIFIED | All pass (`go test ./cmd/engram/... -count=1` green) |
| `internal/setup/detect_test.go`, `plan_test.go` | Fake-environment-only tests | ✓ VERIFIED | No `t.Setenv("PATH", ...)`; all fakes injected via `Environment` struct |
| `internal/setup/exit.go`, `exit_test.go` | Pure `Classify` + exhaustive table | ✓ VERIFIED | Present, exhaustive cross-product test passes |
| `internal/surfaces/toolclass.go` | `setup` row, `Destructive:true`, `Idempotent:true` | ✓ VERIFIED | Row present with justifying comment; `buildCatalog`/`destructiveByClassification` do not panic |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `internal/surfaces/toolclass.go` | `cmd/engram/catalog.go` / `destructive.go` | `setup` row backstops both panics | ✓ WIRED | `go build`/`go test` pass; no panic |
| `cmd/engram/setup.go` | `cmd/engram/destructive.go` | `registerDestructive` owns `RunE` | ✓ WIRED | Confirmed no `setupCmd.RunE =` assignment; `TestMutatingCommandNamesMembership` includes `"setup": true` |
| `internal/setup/runtime.go` | `cmd/engram/setup.go` | `Runtimes` literal feeds preview closure | ✓ WIRED | Three-runtime output confirmed live |
| `cmd/engram/setup.go` | `cmd/engram/operator_view.go` | Report renders via `renderOperator`/`viewRow` | ✓ WIRED | Text and JSON lanes both confirmed byte-identical and structurally consistent |
| `internal/config/registry.go` | `cmd/engram/setup.go` | `setup.url`/`setup.auth` rows reach `config.FlagDefault` and the `(default: ENGRAM_*)` help suffixes | ⚠️ HOLLOW | The help-suffix TEXT is produced (link technically "wired" per the plan's literal `key_links` wording), but the underlying `Env` field the suffix advertises is never consulted — `config.Load` is never called in `cmd/engram/setup.go`. The chain terminates in a static default (`config.FlagDefault`), not a real environment read. This is CR-01 — see gap above and Data-Flow Trace below. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|---------------------|--------|
| `cmd/engram/setup.go` `--url` flag | `setupURL` | `config.FlagDefault("url")` (static registry default) | Flag value: yes (verbatim, confirmed byte-for-byte). `ENGRAM_URL` env value: **no** | ⚠️ STATIC (env lane only) |
| `cmd/engram/setup.go` `--auth` flag | `setupAuth` | `config.FlagDefault("auth")` (static registry default) | Flag value: yes. `ENGRAM_AUTH` env value: **no** | ⚠️ STATIC (env lane only) |
| `internal/setup.Plan()` command string | `Options.URL`/`Options.Auth` | Flags above | Real (from flags); disconnected from env, per above | ⚠️ STATIC (env lane only) |

The flag lane is fully live end to end (confirmed by passing `--url`/`--auth` directly and observing
the exact invocation change). Only the environment-variable lane is disconnected — this is CR-01,
scoped narrowly to those two config keys; it does not affect `--runtime`'s `ENGRAM_RUNTIME` handling,
which reads `os.Getenv` directly and was confirmed working via `setupRuntimeEnvDefault`'s own test.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Preview writes zero files | `go run ./cmd/engram setup --output json --url ...` in a scratch dir, `find` diff before/after | Empty diff | ✓ PASS |
| Preview byte-identical (json) | Two consecutive runs, `diff` | Empty diff | ✓ PASS |
| Preview byte-identical (text) | Two consecutive `--output text` runs, `diff` | Empty diff | ✓ PASS |
| Help byte-identical | Two consecutive `--help` runs, `diff` | Empty diff | ✓ PASS |
| Bad `--auth` exits 2 | `setup --auth basic` | exit 2, names all four modes | ✓ PASS |
| Bad `--runtime` exits 2 | `setup --runtime nope` | exit 2, names all three runtimes | ✓ PASS |
| `--apply` with runtimes present exits 9 | `setup --apply --output json` (real machine, 3 present) | exit 9, full 3-row report on stdout before error | ✓ PASS |
| Concurrent previews independent | Two backgrounded runs with different `--url` | Each returned its own URL, no interference | ✓ PASS |
| `--url` passed verbatim | `setup --url https://direct.example.com/mcp` | URL appears byte-for-byte in all three commands | ✓ PASS |
| `ENGRAM_URL`/`ENGRAM_AUTH` env vars have no effect | `ENGRAM_URL=... ENGRAM_AUTH=bearer setup --output json` | URL blank/malformed, bearer form never triggered | ✗ FAIL (CR-01, confirmed) |
| Exit codes 8/9 advertised | `go run ./cmd/engram` (bare self-describe) | Both present with non-empty meaning | ✓ PASS |
| `task lint` / `task license:check` / `task surfaces:gen` (no diff) | as named | All clean | ✓ PASS |
| Full test suite for touched packages | `go test ./internal/setup/... ./internal/config/... ./internal/surfaces/... ./cmd/engram/... -count=1` | All `ok` | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|--------------|--------|----------|
| REQ-setup-detects-runtimes | 02-01 | Reports runtime presence via own binary; config-dir false positive rejected | ✓ SATISFIED | `TestDetectIgnoresConfigDirectory` passes; `Detect()` calls only `LookPath` |
| REQ-setup-previews-by-default | 02-01 | Preview shows exact command, mutates nothing until `--apply` | ✓ SATISFIED | Confirmed zero file writes; exact `mcp add` invocations shown (when `--url` supplied) |
| REQ-setup-non-interactive | 02-01 | Fully usable without a TTY: explicit runtime selection, no confirmation prompt, machine-readable output | ✓ SATISFIED | All three named capabilities (`--runtime`, no prompt anywhere in the code, `--output json`) work via flags with no TTY. This requirement's text does not name environment-variable configuration as part of its scope, so CR-01 (env vars inert) does **not** block this requirement — verdict stated plainly per the task's instruction not to soften or over-extend it. |
| REQ-setup-partial-failure-legible | 02-02 | Per-runtime outcome + 3-way exit status distinguishing total/partial/total-failure | ✓ SATISFIED | Exit codes 8/9 published and tested; `Classify` exhaustive; apply-lane confirmed live |
| REQ-setup-correct-by-reading | 02-01 | `--help` teaches the correct invocation without running it | ✗ BLOCKED | `--help` names every targetable runtime, all four auth modes, and what `--apply` does (the three items the requirement text explicitly enumerates) — those parts are satisfied. But the SAME help text also asserts `--url`/`--auth` default from `ENGRAM_URL`/`ENGRAM_AUTH`, and that claim is false (CR-01): a caller who follows the documented env-var lane gets a silently malformed command with `outcome: would-write` and no error. This is a direct violation of "teaches the correct invocation... without the caller having to run it and interpret a failure" — here running it doesn't even surface a failure, which is worse. Verdict: this requirement is not fully met as shipped. |

**Orphaned requirements check:** REQUIREMENTS.md maps exactly these five IDs to Phase 2
(`REQ-setup-detects-runtimes`, `REQ-setup-previews-by-default`, `REQ-setup-non-interactive`,
`REQ-setup-partial-failure-legible`, `REQ-setup-correct-by-reading`); all five are declared across
the two plans' `requirements` frontmatter (four in 02-01, one in 02-02). No orphans.
`REQ-setup-idempotent` is correctly scoped to Phase 3 and not claimed here.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `cmd/engram/setup.go` / `internal/config/registry.go` / `config.go` | multiple | Dead code: `SetupConfig`, `Config.Setup`, and two registry `Env` rows are declared but never consulted at runtime | 🛑 Blocker (via CR-01, tied to REQ-setup-correct-by-reading) | See gap above |
| `internal/setup/runtime.go:87-104` | `Select` | No dedup of repeated `--runtime` names | ⚠️ Warning (WR-02, not blocking) | `--runtime claude-code,claude-code` produces two identical report rows and inflates apply-summary counts; independently reproduced during this verification. Pre-existing, review-flagged; does not defeat any must-have truth. |
| `cmd/engram/setup.go` (no `--url` validation) | `setupPlanDoc` | Empty `--url` silently produces a malformed `would-write` command instead of a usage error | ⚠️ Warning (WR-01, not blocking) | Reproduced: `setup --output json` with no `--url` emits `"command":"codex mcp add engram --url "` (trailing space, no error). Pre-existing, review-flagged; does not defeat any must-have truth since no truth requires `--url` to be mandatory. |
| `internal/setup/plan.go` / `claudecode.go` / `codex.go` / `opencode.go` | `Action.Description` field | Populated at all 12 call sites, read nowhere (`setupBuildRows` only reads `.Command`) | ℹ️ Info (IN-01) | Dead field, not user-visible defect |
| `internal/setup/environment.go` | `Getenv`/`HomeDir` | Declared, unused by any current `Runtime` | ℹ️ Info (IN-02) | Plausibly forward-provisioned; harmless |

No `TBD`/`FIXME`/`XXX` markers found in any phase-modified file. The one `ErrApplyNotImplemented`
"not implemented" surface is explicitly named and scoped to "(Phase 3)" in both the error string and
its doc comment — satisfies the debt-marker gate's follow-up-reference exception (D-09, a
plan-documented, intentional stub, not an unmarked debt item).

### Human Verification Required

None required to resolve status. The one item routed to abstention (truth 10, the backstop-tagged
concurrent-`--apply`-dedup property) is a forward-looking guard with no live code path this phase
(`Apply()` is stubbed) — it is not a human-verification item, it is correctly unexercisable yet and
is recorded as `insufficient_spec` for the record, per the backstop-tag rule. It does not affect the
`gaps_found` status determination and should be re-evaluated once Phase 3 implements `Apply()`.

### Gaps Summary

One gap blocks a clean pass: **CR-01** — `ENGRAM_URL`/`ENGRAM_AUTH` are declared as env-first
overrides in the registry and advertised as working defaults in `--help`, but `cmd/engram/setup.go`
never calls `config.Load`, so both variables are completely inert. Verified independently in this
session (not merely re-asserted from the code review): setting both env vars with no matching flags
produces a malformed `claude mcp add --transport http engram  --scope user` (missing URL) and a
`codex mcp add engram --url ` with no bearer form despite `ENGRAM_AUTH=bearer`.

This does **not** defeat the phase's core scriptability goal — the `--url`/`--auth`/`--runtime`
**flag** lane is fully live, verbatim, and TTY-free, so `REQ-setup-non-interactive` (which names only
explicit runtime selection, no-prompt, and machine-readable output — not environment-variable
configuration) is satisfied as written. It does, however, directly falsify `REQ-setup-correct-by-
reading`'s core claim for two of the six documented flags: the help text teaches an invocation path
(`ENGRAM_URL`/`ENGRAM_AUTH`) that silently does not work, with no error surfaced to the caller. That
requirement is therefore BLOCKED as shipped, and this phase should not be considered fully done
until either the env lane is wired (`config.Load` call + regression tests) or the false claims and
dead registry rows are removed.

Two additional review-flagged, non-blocking warnings (WR-01: no `--url` required-ness check; WR-02:
`Select` does not dedupe repeated `--runtime` names) were independently reproduced during this
verification and are recorded for completeness but do not affect the status determination — no
must-have truth requires either behavior.

Every other must-have truth across both plans (14 of 16, plus one correctly-abstained backstop item)
is verified against the live binary: runtime detection via `exec.LookPath` only, preview-by-default
with zero file writes, byte-identical determinism on both output lanes, concurrent-preview
independence, the five-value `Outcome` enum, all twelve (runtime, auth-mode) authored/typed-error
cells, credential redaction by provenance, the full exit-status taxonomy (0/8/9) live and
pure-function-tested, and per-runtime outcome legibility under `--apply`.

---

_Verified: 2026-08-30T14:05:49Z_
_Verifier: Claude (gsd-verifier)_
