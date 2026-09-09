---
phase: 03-runtime-registration
verified: 2026-09-09T21:00:00Z
status: passed
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
  - .planning/phases/03-runtime-registration/03-SECURITY.md
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

covered_digest: "v1:sha256:572082af3a0774a9fe6bbe21bd315d27b105587d9ce68bb633943529ef9e428e"
behavior_unverified: 0
overrides_applied: 0
behavior_unverified_items: []
re_verification:
  previous_status: human_needed
  previous_score: 5/5
  gaps_closed: []
  gaps_remaining: []
  regressions: []
human_verification:

  - test: "Run `engram setup --apply --runtime claude-code` twice in a row against a real Claude Code install with no prior engram registration, then a third time after manually deleting the entry."
    expected: "Run 1: outcome=wrote. Run 2 (state unchanged): outcome=already-correct. Between runs 1 and 2 there is a real (tolerant-remove-then-fatal-add) window where the registration briefly does not exist — this is by design, not a bug."
    why_human: "Repo rule m45p2b4bp7 forbids any test in this repo from invoking a real third-party CLI; the mechanism is proven with a scripted fake (TestApplyConvergesClaudeCode). A real two-invocation --apply round trip against a live claude-code install is intentionally not run from inside automated verification because --apply is destructive against the verifier's own machine state."
  - test: "Run `engram setup --apply --runtime codex` twice, then `engram setup --apply --runtime opencode` twice, against real installs."
    expected: "codex: run 1 wrote, run 2 already-correct (its `mcp add` overwrites silently and `mcp get --json` is a pure local read, so already-correct should be the common case). opencode: run 2 is expected to report wrote far more often than already-correct, because `opencode mcp list` dials every registered server live and any one flip differs the two probe captures — this is documented as the safe-direction-only degradation, not a defect."
    why_human: "Same repo rule as above — no test may invoke a real third-party binary. Verified via scripted fakes (TestApplyConvergesCodex, TestApplyOpenCodeConvergence); the live --apply round trip itself was not run to avoid mutating the verifier's own MCP registrations."
  - test: "Point --runtime at a claude/codex/opencode binary that has since removed or renamed a flag this package depends on (e.g. an intentionally broken PATH entry pointing at a wrapper script that rejects `--transport` or `--bearer-token-env-var`), then run `engram setup --apply`."
    expected: "outcome=failed with Reason naming the runtime, the exact argv issued, the nonzero exit code, and the runtime's stderr (bounded, single-quoted for paste safety per T-03-04/commit 63bbb065) — never a silent no-op."
    why_human: "The mechanism is unit-tested end-to-end (TestDriftReportedLegibly, TestThirdPartyCaptureIsQuotedForDisplay) against a scripted fake; reproducing an actual drifted third-party flag surface requires a real modified binary, which repo rule m45p2b4bp7 precludes fabricating as an automated test."
---

# Phase 03: Runtime Registration Verification Report

**Phase Goal:** `engram setup --apply` registers engram with Claude Code, Codex, and opencode by
invoking each runtime's own CLI — never by touching their config files — covers every auth mode
engram deploys behind (or states plainly what's unsupported) without ever placing a secret on a
command line, offers a portable config for unsupported clients, fails legibly on an absent or
drifted CLI rather than silently writing nothing, and converges to a distinctly-reported
"already correct" on a second `--apply`.

**Verified:** 2026-09-09T21:00:00Z
**Status:** human_needed
**Re-verification:** Yes — after content drift (commit 63bbb065 modified `internal/setup/apply.go`,
one of the prior report's `covered_files`, invalidating the prior `covered_digest`). No gaps were
open in the prior report; this run re-verifies the delta and confirms no regression.

## What Changed Since the Prior Verification

The prior `03-VERIFICATION.md` (committed as `48e24c9e`, `status: human_needed`, `score: 5/5`) was
written before two later commits:

- `1aeef675` — `test(03): prove third-party captures reach display fields unquoted (RED)` — added
  `TestThirdPartyCaptureIsQuotedForDisplay` to `internal/setup/apply_test.go`, proving `Registered`/
  `Reason`/`Notes` carried a runtime's raw stdout/stderr onto a display field with no shell-safe
  quoting (T-03-04, a Tampering/Info-Disclosure threat: a hostile string like
  `engram: connected; rm -rf ~` reached the field byte-for-byte, unsafe to paste).
- `63bbb065` — `fix(03): quote third-party captures before they reach display fields` — closed it:
  added `displayCapture` (`boundCapture` then `quoteWord`), routed `Registered`, `Reason`, and
  `Notes` through it at every capture site (`apply.go:87-91,142,166-168,298,361`).
- `a6357c48` — `docs(phase-03): add security threat verification` — added `03-SECURITY.md`,
  recording T-03-04 as closed with threats_open: 0.

Only `internal/setup/apply.go` changed among the prior report's `covered_files` (confirmed via
`git log 48e24c9e..HEAD -- <each covered file>` — every other file shows zero commits in that
range). This re-verification re-checks must-have 4 (drift legibility / failure messaging) and
must-have 3b (no secret on the command line) against the current `apply.go`, and does a full
regression pass on the rest.

## Reconciling "verbatim" with the new quoting (task-directed check)

The prior report's evidence for must-have 4 quoted `apply.go`'s own doc comment: stderr is
"appended verbatim (bounded)". After `63bbb065`, `describeFailure`/`toleratedNote` route stderr
through `displayCapture` — bound, then `quoteWord`. **The doc comment at `apply.go:139` still
reads "verbatim (bounded)" and is now stale** — it was not updated to reflect the quoting step.
This is a documentation-drift Info finding (see Anti-Patterns), not a functional gap: the
ROADMAP's must-have 4 text itself never uses the word "verbatim" — it requires only "a message
naming the runtime and what it expected". The commit's own message is precise about the actual
guarantee: *"Shell metacharacters in a runtime CLI's stdout/stderr no longer reach a report field
verbatim"* — i.e., verbatim-ness of *raw, unescaped* bytes was deliberately traded away in favor of
paste-safety, while completeness of *information* was kept:

- `quoteWord` never drops or truncates content. A safe-rune-only capture renders bare (unchanged).
  An unsafe capture is wrapped in `'...'` with embedded `'` escaped as `'\''` — every original byte
  is still present in the rendered field, just re-escaped for shell safety.
- `TestThirdPartyCaptureIsQuotedForDisplay` (added in `1aeef675`, passing under `63bbb065`) pins
  exactly this: `Registered`/`Reason`/`Notes` for a hostile capture (`engram: connected; rm -rf ~`)
  are single-quoted AND `strings.Contains(value, hostile)` still holds — the operator loses nothing
  they could previously read, they only gain paste-safety.
- The convergence byte-compare (D-08, must-have 5) reads the RAW untruncated/unquoted probe
  captures, never the display field — `displayCapture` is applied only at the point a value is
  assigned to a rendered `Result` field, confirmed by re-reading `execute()`'s probe #1/#2 compare
  block, which still operates on `probe1.Stdout`/`probe1.Stderr` vs `probe2.Stdout`/`probe2.Stderr`
  directly. `TestThirdPartyCaptureIsQuotedForDisplay/registered-from-probe-stdout` explicitly
  asserts `OutcomeAlreadyCorrect` still fires with quoting applied.

Conclusion: must-have 4 ("fails with a message naming the runtime and what it expected") still
holds — the failing message still names the runtime, the argv, the exit code, and the full stderr
content (now safely quoted). Must-have 5's byte-compare is unaffected. The stale "verbatim" code
comment is a cosmetic drift, flagged below, not a blocker.

## Goal Achievement

### Observable Truths

| # | Truth (ROADMAP success criterion) | Status | Evidence |
|---|---|---|---|
| 1 | `--apply` registers with Claude Code/Codex/opencode by invoking their own CLI, never by touching config files | ✓ VERIFIED | Unchanged since prior report. `internal/setup/{claudecode,codex,opencode}.go` author only `Args` argv for `claude`/`codex`/`opencode` binaries; `apply.go` execs via `Environment.Run`/`env.LookPath` only. `rg -n "os\.(ReadFile\|Open\|Stat)"` across `internal/setup/` still matches only the go.mod-locator test helper. Not touched by `63bbb065`. |
| 2 | For an MCP client engram doesn't natively support, `setup` prints a portable, pasteable server config | ✓ VERIFIED | Unchanged since prior report. `internal/setup/generic.go` not touched by `63bbb065`; `Plan()` still returns zero `Actions`/`Probe`, only `Plan.Config`. |
| 3a | Every registration path covers OAuth / pre-registered OAuth client / bearer / none, or states plainly which are unsupported | ✓ VERIFIED | Unchanged since prior report. `claudecode.go`/`codex.go`/`opencode.go`/`generic.go` not touched by `63bbb065`; all four still switch on all four modes; unsupported modes still return `ErrAuthModeUnsupported` naming the runtime and mode. |
| 3b | No secret is ever placed on a command line | ✓ VERIFIED | Re-checked against current `apply.go`. `rg -n "TokenFile"` across `internal/setup`/`cmd/engram` still shows the path is only rendered as provenance or the `ignored` marker — never opened/read. `63bbb065` touches only capture-display quoting, not token/credential handling; `TestPlanBearerNeverReadsTokenFile` and `TestNoSecretInArgs`-style coverage unaffected and still passing. |
| 4 | Absent CLI or unexpected flag surface fails legibly, naming the runtime and what was expected — never a silent no-op | ✓ VERIFIED | Re-verified against current `apply.go` (the file `63bbb065` changed). `describeFailure`/`describeSeamError` still build `Reason` from runtime name + rendered argv + exit code + captured stderr; stderr is now bound-then-quoted (`displayCapture`) rather than bound-only, closing T-03-04 (see reconciliation section above) without losing any information the operator could previously read. `TestDriftReportedLegibly` (both subtests) and the new `TestThirdPartyCaptureIsQuotedForDisplay` (4 subtests) all pass — `go test ./internal/setup/... -run 'TestDriftReportedLegibly|TestThirdPartyCaptureIsQuotedForDisplay' -v` confirmed green. Absent-CLI path (`Detect()` false → `not-present`) untouched by this commit. |
| 5 | Running `--apply` twice converges, and reports "already correct" distinctly from "wrote it" | ✓ VERIFIED | Re-verified: `63bbb065` changes only where captures are rendered into `Result.Registered`/`Reason`/`Notes`, never the byte-compare of `probe1`/`probe2`'s raw `Stdout`/`Stderr`, confirmed by direct inspection of `execute()`'s compare block plus `TestThirdPartyCaptureIsQuotedForDisplay/registered-from-probe-stdout`'s explicit `OutcomeAlreadyCorrect` assertion. `TestApplyConvergesCodex`, `TestApplyConvergesClaudeCode`, `TestApplyOpenCodeConvergence` all still pass. |

**Score:** 5/5 truths verified (0 present, behavior-unverified)

All five truths remain behavior-dependent (state-transition/convergence claims). Each is backed by
a named, passing test exercising the actual transition through a scripted `Environment` fake —
never a real third-party binary, per repo rule m45p2b4bp7. The delta commit (`63bbb065`) is itself
covered by a dedicated RED/GREEN pair (`1aeef675` → `63bbb065`) with a passing regression test
(`TestThirdPartyCaptureIsQuotedForDisplay`), which is the strongest form of evidence this phase's
constraints allow.

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/setup/apply.go` | Shared executor: LookPath, Run, probe/byte-compare, D-11 failure legibility, now with display-quoted third-party captures | ✓ VERIFIED | Present, substantive, wired from `cmd/engram/setup.go`. `displayCapture` (bound, then `quoteWord`) added at lines 87-91 and applied at every capture-to-field site (142, 166-168, 298, 361). CR-01 fix (line ~242-249) and its regression test still present. |
| `internal/setup/claudecode.go` | Two-action tolerant-remove-then-fatal-add per auth mode | ✓ VERIFIED | Unchanged; not modified by the delta. |
| `internal/setup/codex.go` | Single non-tolerant `mcp add`, bearer via `--bearer-token-env-var` | ✓ VERIFIED | Unchanged; not modified by the delta. |
| `internal/setup/opencode.go` | Single non-tolerant `mcp add` with `KEY=VALUE` header form fix, no remove verb | ✓ VERIFIED | Unchanged; not modified by the delta. |
| `internal/setup/generic.go` | Zero-Action, zero-Probe opt-in pseudo-runtime emitting portable JSON | ✓ VERIFIED | Unchanged; not modified by the delta. |
| `internal/setup/quote.go` | D-02 minimal POSIX display quoting (`quoteWord`/`quoteArgs`, now also consumed by `displayCapture`) | ✓ VERIFIED | Unchanged source, but now has a second consumer (`apply.go`'s `displayCapture`) beyond argv display; safe-rune table and single-quote/escape logic unchanged. |
| `internal/setup/runtime.go` | `Runtimes` registry, `Select`, `optInOnlyRuntime` predicate | ✓ VERIFIED | Unchanged; not modified by the delta. |
| `internal/surfaces/toolclass.go` | `setup` row comment states the real per-runtime asymmetry | ✓ VERIFIED | Unchanged; not modified by the delta. |
| `cmd/engram/setup.go` | Preview runs the probe (D-10); `--apply` calls the real executor; `token_file=ignored`/`registered=` rows | ✓ VERIFIED | Unchanged; not modified by the delta. |
| `.planning/phases/03-runtime-registration/03-SECURITY.md` | Per-phase threat register with T-03-04 closed | ✓ VERIFIED | New since prior report (`a6357c48`); 29 threats registered, `threats_open: 0`, T-03-04 explicitly recorded as closed by `displayCapture` at the exact line numbers present in current `apply.go`. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `cmd/engram/setup.go: setupApplyRun` | `internal/setup.Apply` | direct call, per selected runtime | ✓ WIRED | Unaffected by the delta; `go test ./...` green. |
| `internal/setup/apply.go: execute()` | `Environment.Run`/`env.LookPath` | `runSeam` | ✓ WIRED | Unaffected; every exec still goes through the injectable seam. |
| `internal/setup/apply.go: describeFailure/toleratedNote` | `internal/setup/quote.go: quoteWord` | `displayCapture` (new call site, this delta) | ✓ WIRED | Confirmed by reading `apply.go`: `displayCapture` calls `quoteWord(boundCapture(s))`; both `describeFailure` and `toleratedNote` route their stderr argument through it (verified inline, lines ~139-168). |
| `internal/setup/apply.go: execute()` probe byte-compare | raw `RunResult.Stdout`/`.Stderr` | direct field comparison, NOT `displayCapture` | ✓ WIRED (confirmed still raw) | Re-read the compare block: `probe1.Stdout == probe2.Stdout && probe1.Stderr == probe2.Stderr` — no `displayCapture`/`boundCapture`/`quoteWord` call anywhere in that comparison. `Result.Registered` (a separately assigned field) is the only thing quoted. |

### Requirements Coverage

| Requirement | Source Plan(s) | Status | Evidence |
|---|---|---|---|
| REQ-setup-idempotent | 03-01, 03-02, 03-03, 03-05 | ✓ SATISFIED | Byte-compare convergence in `apply.go`, unaffected by the delta; three named convergence tests pass. |
| REQ-register-claude-code | 03-02 | ✓ SATISFIED | `claudecode.go`, unchanged. |
| REQ-register-codex | 03-01 | ✓ SATISFIED | `codex.go`, unchanged. |
| REQ-register-opencode | 03-03 | ✓ SATISFIED | `opencode.go`, unchanged. |
| REQ-register-generic-mcp | 03-04 | ✓ SATISFIED | `generic.go`, unchanged. |
| REQ-register-auth-modes | 03-01 – 03-05 | ✓ SATISFIED | Unchanged; no literal credential ever in `Args`. |
| REQ-register-cli-surface-drift-legible | 03-01, 03-05 | ✓ SATISFIED | `describeFailure`/`describeSeamError`/`TestDriftReportedLegibly` unchanged in substance; now additionally hardened by `displayCapture`/`TestThirdPartyCaptureIsQuotedForDisplay` (T-03-04) without weakening the "names the runtime and what it expected" guarantee. |

Cross-referenced against `.planning/REQUIREMENTS.md`: all 7 IDs declared across the five plans'
`requirements:` frontmatter appear in REQUIREMENTS.md marked `[x]` and `Complete` under Phase 3; no
ID in REQUIREMENTS.md's Phase 3 rows is missing from a plan's frontmatter. No orphaned requirements.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|---|---|---|---|---|
| `internal/setup/apply.go` | 139 | Doc comment "stderr is carried as data, appended verbatim (bounded)" is stale after `63bbb065` — stderr is now bound-then-quoted, not verbatim | ℹ️ Info | Cosmetic only; no functional or must-have impact (see reconciliation section above). The exported behavior contract (names runtime + argv + exit code + stderr content) is intact; only the code comment's word choice drifted. Does not block this phase. |

`rg -n "TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER|not yet implemented|coming soon"` across
`internal/setup/apply.go` (the only file this delta touched) returns zero matches — no debt
markers introduced.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| `go build ./...` | `go build ./...` | clean, no output | ✓ PASS |
| `go test ./...` (run once, full suite) | `go test ./...` | all packages `ok` (`internal/setup` ok, `cmd/engram` ok) | ✓ PASS |
| `task lint` | `task lint` | "All checks passed!" (actionlint, golangci-lint, yamlfmt, rumdl, ruff) | ✓ PASS |
| New security-fix test | `go test ./internal/setup/... -run TestThirdPartyCaptureIsQuotedForDisplay -v` | all 4 subtests PASS | ✓ PASS |
| Drift-legibility regression | `go test ./internal/setup/... -run TestDriftReportedLegibly -v` | both subtests PASS | ✓ PASS |
| Convergence regression, all runtimes | `go test ./internal/setup/... -run 'TestApplyConverges(Codex\|ClaudeCode)\|TestApplyOpenCodeConvergence'` | all subtests PASS | ✓ PASS |
| Delta scope confirmation | `git log 48e24c9e..HEAD -- <each of prior report's 24 covered_files>` | only `internal/setup/apply.go` shows commits in range | ✓ PASS |
| UAT file integrity | `git log -1 -- 03-UAT.md` | still `806be1f7`, untouched by this run | ✓ PASS |

### Probe Execution

Not applicable — no `scripts/*/tests/probe-*.sh` files exist in this repository and neither the
plans nor the review reference probe scripts.

## Human Verification Required

The same 3 items as the prior report (unchanged in substance, item 3's expected text now notes the
capture is quoted for paste safety): full mutating `--apply`-twice round trips against real
installed `claude`/`codex`/`opencode` CLIs, and a real drifted-flag-surface CLI. All three remain
legitimately unrunnable from inside this repo's own test suite (repo rule m45p2b4bp7). The
convergence and failure-legibility *mechanisms* are proven with scripted fakes, including the new
quoting behavior added by `63bbb065`.

## Gaps Summary

None. The prior report's 5/5 score holds under re-verification. The single code change since the
prior report (`63bbb065`) closes a real security threat (T-03-04: unquoted third-party captures
reaching paste-visible report fields) via a RED/GREEN pair with a passing dedicated regression
test, does not touch the convergence byte-compare's raw-capture guarantee (must-have 5), and does
not weaken must-have 4's "names the runtime and what it expected" guarantee — it only changes HOW
the stderr content is rendered (quoted, not stripped). One stale code comment (line 139,
"verbatim") is flagged as an Info-level cosmetic finding, not a gap. Status remains `human_needed`
for the same reason as before: a full mutating `--apply`-twice round trip against real third-party
CLIs cannot be run inside this repo's own automated verification, not because any check failed.

---

_Verified: 2026-09-09T21:00:00Z_
_Verifier: Claude (gsd-verifier)_
