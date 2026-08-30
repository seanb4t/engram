---
phase: 02-setup-command-core
reviewed: 2026-08-30T13:58:15Z
depth: standard
files_reviewed: 28
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
  warning: 2
  info: 2
  total: 5
status: issues_found
---

# Phase 02: Code Review Report

**Reviewed:** 2026-08-30T13:58:15Z
**Depth:** standard
**Files Reviewed:** 28
**Status:** issues_found

## Summary

Reviewed the `internal/setup` package (Runtime/Detect/Plan, the injectable Environment
seam, the three runtimes) and its `engram setup` CLI wiring in `cmd/engram/setup.go`,
plus the exit-status taxonomy (`internal/setup.Classify`, the five exit-code constants
in `client_common.go`, and their catalog/test coverage).

The preview/apply boundary (invariant 3) holds: `setupPreview` performs only
`exec.LookPath`-style detection through the injected `Environment` seam and pure string
authoring in `Plan()`; no file write, no runtime-CLI execution, and no lock acquisition
happens on any code path this phase ships, confirmed by tracing every call site and by
`TestSetupPreviewExecutesNoRuntimeCLI`. Secret handling (invariant 6) is also sound:
`bearerProvenance` never opens, stats, or reads `--token-file`, and every bearer-mode
`Action.Command` embeds only the file's *path* or a fixed `ENGRAM_TOKEN` name, never
content — confirmed by `TestPlanBearerNeverReadsTokenFile` and by direct inspection.
`internal/setup.Classify`'s outcome table is genuinely exhaustive over all five `Outcome`
values, and `OutcomeNotPresent` is a real first-class value, never a zero-value stand-in
(the zero value `""` is explicitly routed to the failure branch — see `exit.go`'s
`default` case). The three published exit codes (`exitOK`/`exitPartial`/
`exitSetupFailed`) are wired consistently across `client_common.go`, `catalog.go`,
`setup.go`'s `setupExitCode`, and their respective tests.

One BLOCKER was found and confirmed empirically (not just by static reading): the
`setup.url`/`setup.auth` registry rows declare `ENGRAM_URL`/`ENGRAM_AUTH` as env-first
overrides, but `cmd/engram/setup.go` never routes its flags through `config.Load`, so
those two environment variables are completely inert despite being advertised in
`--help` text and in `internal/config`'s own design commentary. Two WARNINGs and two INFO
items round out the report.

## Critical Issues

### CR-01: `ENGRAM_URL` and `ENGRAM_AUTH` are declared env-first but have zero effect on `engram setup`

**File:** `cmd/engram/setup.go:361-376` (flag registration), `cmd/engram/setup.go:152-174`
(`setupPlanDoc`), `internal/config/registry.go:100-114` (`setup.url`/`setup.auth` rows),
`internal/config/config.go:264-279` (`SetupConfig`)

**Issue:** `internal/config/registry.go` declares `setup.url` (`Env: "ENGRAM_URL"`) and
`setup.auth` (`Env: "ENGRAM_AUTH", Default: "oauth"`) as ordinary env-first-with-flag-
override entries — the registry's own comment states they are "enrolled env-first with
flag override like the 45/48 majority of this registry." `internal/config.Config.Setup`
(`SetupConfig{URL, Auth}`) exists to carry the resolved values, and its doc comment
repeats the `ENGRAM_URL / --url` / `ENGRAM_AUTH / --auth` precedence claim.

However, `cmd/engram/setup.go` never calls `config.Load(cmd.Flags())` anywhere (unlike
`client_common.go:134` and `serve.go:69`, the two commands that actually do this). Its
`init()` binds `--url`/`--auth` directly to package vars with `pflag.StringVar`, defaulted
only from `config.FlagDefault(...)` — the registry's *static* `Default` field, which does
not consult the environment at all:

```go
setupCmd.Flags().StringVar(&setupURL, "url", config.FlagDefault("url"), ...)
setupCmd.Flags().StringVar(&setupAuth, "auth", config.FlagDefault("auth"), ...)
```

`Config.Setup` is never referenced anywhere outside its own declaration (`rg
"\.Setup\b|SetupConfig"` across the whole repo returns only the declaration site and the
`koanf:"setup"` struct tag) — the entire `SetupConfig` type and its two registry rows are
dead code from the moment they were written.

Confirmed empirically (not just by reading), by running the built binary with the env
vars set and no matching flags:

```
$ ENGRAM_URL="https://env-url.example.com/mcp" ENGRAM_AUTH="bearer" \
    go run ./cmd/engram setup --output json
{"runtimes":[
  {"name":"claude-code","outcome":"would-write",
   "command":"claude mcp add --transport http engram  --scope user"},
  {"name":"codex","outcome":"would-write",
   "command":"codex mcp add engram --url "},
  {"name":"opencode","outcome":"would-write",
   "command":"opencode mcp add engram --url "}]}
```

Both `ENGRAM_URL` (the endpoint never appears at all — see the blank `--url ` and the
double space in claude-code's command) and `ENGRAM_AUTH=bearer` (none of the three
commands take the bearer form — no `--header`, no `--bearer-token-env-var`) are silently
ignored. This directly contradicts:
- the flag help text itself: `"...used verbatim — never appended to or stripped (default:
  ENGRAM_URL)"` and `"...(default: ENGRAM_AUTH)"` (both false claims as shipped);
- `internal/config/registry.go`'s own stated design intent for these two rows;
- the general env-first contract every other configured value in this registry honors
  (project invariant 2: 45 of 48 registry rows have an `Env` fallback that actually
  works — these two are silently broken, not documented exceptions).

No test in `cmd/engram/setup_test.go`, `internal/config/client_validate_test.go`, or
anywhere else sets `ENGRAM_URL`/`ENGRAM_AUTH` and asserts on the resulting command, so
this gap has no regression coverage at all — the only test that exercises
`setupRuntimeEnvDefault` (`ENGRAM_RUNTIME`) covers the ONE flag among the three that
actually does read its env var directly (via `os.Getenv` in `setupRuntimeEnvDefault`,
not through the registry).

**Fix:** Route `--url`/`--auth` through the registry the way `client_common.go` and
`serve.go` already do, e.g.:

```go
func setupPlanDoc(cmd *cobra.Command) (setupReportDoc, error) {
	cfg, err := config.Load(cmd.Flags())
	if err != nil {
		return setupReportDoc{}, usageErrorf("load setup configuration: %w", err)
	}
	if err := config.ValidateSetupAuth(cfg.Setup.Auth); err != nil {
		return setupReportDoc{}, usageErrorf("%w", err)
	}
	auth := cfg.Setup.Auth
	if auth == "" {
		auth = "oauth"
	}
	runtimes, err := setup.Select(setupRuntime)
	if err != nil {
		return setupReportDoc{}, usageErrorf("%w", err)
	}
	opts := setup.Options{URL: cfg.Setup.URL, Auth: auth, TokenFile: setupTokenFile}
	rows := setupBuildRows(setupEnv, runtimes, opts)
	return setupReportDoc{Runtimes: rows}, nil
}
```

and add regression coverage (`t.Setenv("ENGRAM_URL", ...)` / `t.Setenv("ENGRAM_AUTH",
...)` with no matching flag, asserting the emitted `Command` reflects the env value) —
either delete `SetupConfig`/the two registry rows if env support for `--url`/`--auth` is
genuinely out of scope, or wire it up; leaving the registry rows in place while silently
not consulting them is the worst of both options because it actively documents a
guarantee the code does not keep.

## Warnings

### WR-01: No validation that `--url`/`ENGRAM_URL` is non-empty before authoring a preview command

**File:** `cmd/engram/setup.go:148-174` (`setupPlanDoc`), `internal/setup/claudecode.go:40-73`,
`internal/setup/codex.go:37-66`, `internal/setup/opencode.go:38-61`

**Issue:** Nothing rejects an empty `--url` (no `cmd.MarkFlagRequired("url")`, no explicit
empty-string check in `setupPlanDoc`). Every `Runtime.Plan` happily formats the empty
string into its invocation, producing a malformed, misleading "would-write" preview, e.g.
`claude mcp add --transport http engram  --scope user` (note the double space where the
URL argument should be) or `codex mcp add engram --url ` (trailing space, no value).
Reproduced directly: `go run ./cmd/engram setup --output json` (no `--url` set) prints
exactly this shape for all three runtimes. A caller relying on the preview text to decide
what to run — or worse, copy-pasting it — gets a broken command with no indication
anything is wrong; the command still reports `outcome: would-write`, not `failed`.

**Fix:** Treat a missing URL as a usage error in `setupPlanDoc`, the same way
`clientFromFlags` rejects a missing `--server`:

```go
if setupURL == "" {
    return setupReportDoc{}, usageErrorf("--url or ENGRAM_URL is required")
}
```

(This also needs test coverage: `TestSetupPreviewExitsZeroRegardlessOfPresence` currently
omits `--url` entirely and only checks the exit code, never the command content, so this
gap has no regression coverage today.)

### WR-02: `setup.Select` does not deduplicate repeated `--runtime` values

**File:** `internal/setup/runtime.go:87-104`

**Issue:** `Select` appends one entry to `out` per name in the input slice with no
dedup check. Because `--runtime` is a `StringSliceVar` (comma-separated or repeatable),
`engram setup --runtime claude-code,claude-code` (or `--runtime claude-code --runtime
claude-code`) resolves to `[]Runtime{ClaudeCode, ClaudeCode}`, and `setupBuildRows`
faithfully emits two identical rows for "claude-code" in the JSON `runtimes` array. Under
`--apply` this also inflates `setupApplySummary`'s and the returned `cliError`'s "%d of %d
selected runtime(s) failed" counts (e.g. reporting "2 of 2 failed" for what is really one
physical runtime attempted twice), which is misleading to a caller scripting against that
count.

**Fix:** Dedupe in `Select`, e.g. skip a name already present in `out` (or reject a
duplicate as a usage error, mirroring the existing "unknown runtime" rejection) —
whichever behavior is chosen, add a test pinning it, since none exists today.

## Info

### IN-01: `Action.Description` is populated everywhere but read nowhere

**File:** `internal/setup/plan.go:59-62`, and every `Plan()` implementation in
`claudecode.go`, `codex.go`, `opencode.go` (12 call sites total)

**Issue:** Every `Plan()` branch across all three runtimes populates
`Action.Description` with a human-readable label (e.g. "register engram as a user-scope
MCP server (bearer token)"). `cmd/engram/setup.go`'s `setupBuildRows` only ever reads
`plan.Actions[0].Command`; `Description` is never surfaced in the JSON document, the text
view, or anywhere else (`rg "\.Description\b"` across `internal/setup` and `cmd/engram`
turns up only the write sites). This is either dead effort that should be trimmed, or a
missed opportunity to explain the previewed command to an operator — worth a decision
either way rather than silently carrying an unused field through every code path.

**Fix:** Either surface `Description` in `setupRuntimeRow` (e.g. a `description` JSON
field, consistent with the D-15 "text and json both render one document" design already
used for `Command`/`Reason`), or drop the field until a consumer exists.

### IN-02: `Environment.Getenv` and `Environment.HomeDir` are unused by every current `Runtime`

**File:** `internal/setup/environment.go:20-32`, `internal/setup/claudecode.go`,
`internal/setup/codex.go`, `internal/setup/opencode.go`

**Issue:** `Environment`'s doc comment describes it as "the injectable seam every
Runtime's Detect/Plan consults for external-boundary reads: which binaries are on PATH,
environment variables, and the caller's home directory," but as shipped this phase, only
`LookPath` is ever called (`rg "env\.Getenv|env\.HomeDir"` across non-test `internal/setup`
files returns no hits). This is plausibly deliberate forward-provisioning for a later
phase, but as it stands the two fields are unreachable dead surface with no current
producer or consumer — worth a one-line note if intentional, since a future reviewer
cannot otherwise tell "unused so far, reserved for later" from "wired up somewhere I
missed."

**Fix:** No action required if this is confirmed intentional; otherwise trim the unused
fields until a runtime actually needs them.

---

_Reviewed: 2026-08-30T13:58:15Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
