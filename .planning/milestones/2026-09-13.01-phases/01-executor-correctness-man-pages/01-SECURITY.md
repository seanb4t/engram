---
phase: "01"
slug: "executor-correctness-man-pages"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-17"
---

# Phase 01 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| engram → runtime CLI subprocess | `osRun` spawns a third-party binary (`claude`, `codex`, `opencode`) resolved from PATH and captures stdout/stderr/exit; the child's behaviour (hang, partial output, exit code) is untrusted input to the executor's classification. | Child process stdout/stderr/exit code (untrusted) |
| executor → operator terminal / `--output json` | `Result.Reason` renders into the operator's terminal and the JSON lane; what the seam reports is what the operator acts on. | Result rows, reason strings |
| cask hook (Ruby, end user's machine) → filesystem under `HOMEBREW_PREFIX` | `post_install` creates a directory and invokes the installed binary to write man pages; `post_uninstall` deletes files by glob in a directory shared with every other formula and cask. | Filesystem writes/deletes under `share/man/man1` |
| argv → `engram man <dir>` | A local filesystem path supplied by the operator (or the hook's fixed `#{HOMEBREW_PREFIX}` path) selects where the binary writes files. | Operator-supplied local path |
| cobra command tree → generated documentation | Every available command's `Short`/`Long`/flag usage is rendered into pages installed system-wide; hidden and deprecated commands must not leak. | Command help text |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-01-01 | Tampering | `osRun` classification — hung/timed-out runtime CLI misread as clean `exited -1` or a real "not registered" answer | high | mitigate | `internal/setup/environment.go` `osRun`: `case ctx.Err() != nil` precedes the `exec.ExitError` arm and returns the seam's own error, routing `execute` to `describeSeamError`; `TestOsRunReportsContextDeadlineExceeded` / `TestOsRunReportsContextCanceled` (`internal/setup/environment_test.go`) prove it against a real killed child | closed |
| T-01-02 | Information Disclosure | partial third-party stdout/stderr captured before the kill reaching a rendered field | low | mitigate | `osRun` returns `RunResult{}` (zero value) on the context-error path; `environment_test.go:79,105` assert `res != (RunResult{})` fails; `describeSeamError` takes only `err` | closed |
| T-01-03 | Denial of Service | a runtime CLI that never exits | medium | mitigate | `execTimeout` (`internal/setup/apply.go`) via `exec.CommandContext` kills the child; rendered as `timed out after 20s: context deadline exceeded`; `environment_test.go:82` asserts return within 3 s of the deadline | closed |
| T-01-04 | Repudiation | operator cannot tell "our timeout" from "their tool failed" | medium | mitigate | Exact-row assertions in `internal/setup/apply_test.go` / `cmd/engram/setup_test.go`: DeadlineExceeded → `timed out after 20s: context deadline exceeded`; Canceled → `context canceled`, never `exited N` | closed |
| T-01-05 | Tampering | test suite invoking a real third-party CLI or the operator's `$HOME` | medium | mitigate | Only real child is the test binary itself (`os.Args[0]` re-exec → `TestOsRunHelperProcess`); `environment_test.go` has zero `exec.Command(` and zero `UserHomeDir` hits | closed |
| T-01-SC | Tampering | npm/pip/cargo installs | low | accept | Not applicable — plan 01-01 installs nothing and adds no import outside the Go stdlib; `go.mod`/`go.sum` asserted byte-unchanged | closed |
| T-02-01 | Tampering | `.goreleaser.yaml` `hooks.post.install` man step writing outside `HOMEBREW_PREFIX`, or silently degrading to a warning | high | mitigate | Only path literal is `"#{HOMEBREW_PREFIX}/share/man/man1"` (exactly 2 occurrences in `.goreleaser.yaml`); `system_command` default `must_succeed: true`; `TestReleaseConfigCaskInstallGate` (`cmd/engram/releaseconfig_test.go`) counts forbidden literals at 0 | closed |
| T-02-02 | Tampering | `hooks.post.uninstall` glob deleting another package's pages or the shared `man1` directory | medium | mitigate | Glob prefix-scoped to `engram{,-*}.1` (exactly 1 occurrence), applied with file-scoped `FileUtils.rm_f`; `rm_rf` count is 0 in `.goreleaser.yaml`; residual (third-party page literally named `engram-<x>.1`) accepted — see AR-01-02 | closed |
| T-02-03 | Information Disclosure | hidden (`man`) or deprecated (`backfill-short-ids`, `migrate-set-owner`) command help published as man pages | low | mitigate | `GenManTree` walks with cobra's own `IsAvailableCommand()`; `TestManPagesMatchAvailableCommands` (`cmd/engram/man_test.go`) asserts the absence of `engram-man.1`, `engram-help.1`, `engram-backfill-short-ids.1`, `engram-migrate-set-owner.1` | closed |
| T-02-04 | Tampering | `engram man <dir>` argv path (ASVS V5, marginal) | low | accept | Operator's own local path; `cobra.ExactArgs(1)` bounds arity; `os.MkdirAll` + `os.Create` run with the invoking user's privileges and cross no trust boundary — see AR-01-03 | closed |
| T-02-05 | Repudiation | non-deterministic pages (wall-clock date, local TZ, `SOURCE_DATE_EPOCH`) making "same binary, different pages" undiagnosable | medium | mitigate | `cmd/engram/man.go`: `Date` pinned to `time.Unix(0, 0).UTC()`, `Source` from compiled-in `version`, `DisableAutoGenTag` on root; `TestManPagesByteStable` asserts byte-identity across two runs and the exact root `.TH` line | closed |
| T-02-06 | Denial of Service | the install hook's binary exercise hanging | low | accept | Same profile as the already-shipped completions step: local, bounded, run under Homebrew's installer; a hang is a failed install the user sees immediately (rule m45p2b4bp7 — Homebrew's process, not ours) — see AR-01-04 | closed |
| T-02-07 | Tampering | man-page generation mutating the shared cobra tree (grafted `help` children) and silently changing `--help` output or a golden | medium | mitigate | `snapshotCommandTree` / `pruneToSnapshot` restore inside `writeManPages` (`cmd/engram/man.go`); `TestManGenerationLeavesCommandTreeUnchanged` pins it; `TestHelpGolden` stays green | closed |
| T-02-SC | Tampering | npm/pip/cargo installs | low | accept | Not applicable — `github.com/spf13/cobra/doc` is a subpackage of the already-direct `cobra` requirement; `go.mod`/`go.sum` asserted byte-unchanged | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-01-01 | T-01-SC, T-02-SC | No package-manager install task in either plan; zero new Go modules; `go.mod`/`go.sum` byte-unchanged. Supply-chain control is not applicable. | Plan author (01-01, 01-02 threat model) | 2026-09-17 |
| AR-01-02 | T-02-02 (residual) | A third-party man page literally named `engram-<x>.1` would be removed by the uninstall glob. No such package exists; the prefix is engram's own name. | Plan author (01-02 D-08) | 2026-09-17 |
| AR-01-03 | T-02-04 | `engram man <dir>` takes the operator's own local path with the operator's own privileges; no trust boundary is crossed. Path traversal is canonical ASVS V5 and does not apply to a self-directed local write. | Plan author (01-02 threat model) | 2026-09-17 |
| AR-01-04 | T-02-06 | Install-hook hang is Homebrew's process and surfaces as a visible failed install; adding an engram-side timeout would gate third-party behaviour we do not own (rule m45p2b4bp7). | Plan author (01-02 threat model) | 2026-09-17 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-17 | 14 | 14 | 0 | /gsd-secure-phase (orchestrator, L1 grep-depth short-circuit — register authored at plan time, ASVS L1) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-17
