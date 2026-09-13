---
phase: "03"
slug: "runtime-registration"
status: secured
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-09"
---

# Phase 03 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| caller argv → engram CLI | `--url`, `--auth`, `--runtime`, `--token-file`; world-readable via the process table | operator-supplied config; a token FILE PATH, never a token |
| local filesystem `PATH` → engram | `exec.LookPath` resolves a runtime name against whatever `PATH` names, and the resolved binary is then executed | a binary path engram did not author |
| engram → third-party runtime CLI | engram starts `claude` / `codex` / `opencode` as a child process with an argument list it authored | argv only; no shell, no config file |
| third-party runtime CLI → engram | the child's stdout and stderr become report content in the terminal and in `--output json` | untrusted bytes engram did not author |
| engram → the operator's existing MCP configuration | claude-code's clear-the-slot action destroys state engram never read | registration state |
| engram → the configured MCP endpoint | claude-code's and opencode's probes dial the registered URL; a bare `setup` therefore makes a network request | endpoint reachability |
| engram → operator terminal | the rendered report, including strings a human may paste into a shell | display text |
| engram → an arbitrary unsupported MCP client | the `generic` runtime's emitted config is consumed by software engram has no knowledge of | a pasteable config document |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-03-01 | Tampering | argv reaching `Environment.Run` | high | mitigate | `exec.CommandContext(ctx, path, args...)` argv form; no shell anywhere in the path (`environment.go:85`) | closed |
| T-03-02 | Tampering | derived display string a human may paste | medium | mitigate | `quoteWord`/`quoteArgs` safe-set quoter with a pinned table test (`quote.go:17-51`) | closed |
| T-03-03 | Spoofing | `LookPath` resolving a hostile binary | high | mitigate + accept residual | exec the resolved ABSOLUTE path, reported as `binary=` (`apply.go:251-259`); residual accepted (AR-01) | closed |
| T-03-04 | Tampering / Info Disclosure | third-party stdout+stderr rendered into `reason=` / `notes=` / `registered=` | high | mitigate | `displayCapture` (bound, then `quoteWord`) applied at all capture sites (`apply.go:87-91`, `:142`, `:166-168`, `:298`, `:361`) | closed |
| T-03-05 | Info Disclosure | bearer credential reaching argv | high | mitigate | argv carries a variable NAME only; `TestNoSecretInArgs` over every runtime × auth-mode pair | closed |
| T-03-06 | DoS | runtime CLI prompting or hanging | medium | mitigate | `execTimeout` per exec via `context.WithTimeout`; `cmd.Stdin = nil` | closed |
| T-03-07 | Repudiation | failed `--apply` erasing the per-runtime record | high | mitigate | `renderOperator` called unconditionally before any error return (`setup.go:350-354`) | closed |
| T-03-08 | Tampering | nondeterministic probe falsely classifying `already-correct` | medium | mitigate | byte-compare reads RAW untruncated captures; every ambiguous case resolves to `wrote` (`apply.go:357-361`) | closed |
| T-03-09 | DoS | the claude-code remove/add destructive window | high | mitigate | user-accepted (AR-02); failure `reason` names the argv, `notes=` records the slot was cleared plus recovery | closed |
| T-03-10 | Info Disclosure | `claude mcp get` dialing the registered URL | low | accept | AR-03 | closed |
| T-03-11 | Tampering | tolerating a nonzero exit by matching message text | medium | mitigate | tolerance branches only on the authored `Action.Tolerant` bool, never stderr content (`apply.go:314,323`) | closed |
| T-03-12 | Spoofing | stale `toolclass.go` comment misrepresenting a destructive command | medium | mitigate | comment corrected to live-verified per-runtime behavior, source named (`toolclass.go:187-211`) | closed |
| T-03-13 | Info Disclosure | `mcp list` naming every OTHER registered server | medium | mitigate + accept residual | bounded + scalar-typed field; residual accepted (AR-04) | closed |
| T-03-14 | Tampering | box-drawing glyphs forging report structure | low | mitigate | `sanitizeViewValue` strips the escape-sequence class; classification never parses captured text | closed |
| T-03-15 | Spoofing | opencode `{env:...}` silently failing to resolve | medium | accept | AR-05 | closed |
| T-03-16 | Tampering | polluted opencode probe falsely classifying `already-correct` | medium | mitigate | `TestApplyOpenCodeConvergence` differing-captures subtest asserts `wrote` | closed |
| T-03-17 | Tampering | `--url` control chars via an unsanitized nested row value | high | mitigate | `config` row field typed as a Go `string` so `viewScalar` reaches it; nesting guard green with no exception | closed |
| T-03-18 | Info Disclosure | credential reaching the pasteable portable config | high | mitigate | `bearerProvenance` never opens the file; zero `os.Open`/`ReadFile`/`Stat` in the package; `TestGenericConfigCarriesNoSecret` | closed |
| T-03-19 | Spoofing | a silently normalized `url` in the pasted config | medium | mitigate | `opts.URL` placed byte-for-byte; `TestGenericConfig` asserts byte equality | closed |
| T-03-20 | EoP | pseudo-runtime inflating what a bare `setup` claims | low | mitigate | opt-in-only predicate excludes `Generic` from `Select(nil)`; `TestSetupBareInvocationOmitsGeneric` | closed |
| T-03-21 | Repudiation | a `Classify` change altering the documented exit code | medium | mitigate | `internal/setup/exit.go` untouched; `TestSetupGenericAndFailingRuntimeExitsPartial` | closed |
| T-03-22 | EoP | `setupPreview` gaining a nonzero-exit path | high | mitigate | non-mutating branch cannot set any Outcome but `would-write`; `TestSetupPreviewExitsZeroWhenProbeFails` | closed |
| T-03-23 | Info Disclosure | a bare `setup` making a network request | medium | accept | AR-06 | closed |
| T-03-24 | DoS | preview hanging on an unreachable endpoint | medium | mitigate | `execTimeout`/`runSeam` apply to the preview lane identically; stdin closed | closed |
| T-03-25 | Repudiation | a flag honored or inert depending on `--runtime` | medium | mitigate | `token_file` marker set by a structural rule keyed on `len(plan.Actions)`; both directions pinned | closed |
| T-03-26 | Info Disclosure | duplicating the token-file PATH into a second field | low | mitigate | fixed engram-authored marker containing no path; `TestSetupTokenFilePathNotDuplicatedIntoMarker` | closed |
| T-03-27 | EoP | an escape-hatch flag added to `setupCmd` | high | mitigate | `TestDestructiveCommandsExactFlagSet` set-equality gate; `setupCmd` row unedited | closed |
| T-03-SC | Tampering | dependency supply chain | low | accept | AR-07 | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-01 | T-03-03 | `PATH` integrity remains the operator's own trust boundary; engram cannot distinguish a legitimate from a hostile binary at a resolved path. Executing the absolute path closes the TOCTOU window and `binary=` leaves a trace in the report and in `--output json`. | Sean | 2026-09-08 |
| AR-02 | T-03-09 | `claude mcp add` refuses an existing name at every scope and has no `--force`; reading the config file is forbidden. A failed or interrupted `add` after a successful `remove` leaves no registration, which engram cannot restore because it never read the prior entry. Accepted as a regression in failure safety, traded for reachable `already-correct` and a satisfiable REQ-setup-idempotent. Alternatives were presented and rejected at a decision checkpoint. | Sean | 2026-09-09 |
| AR-03 | T-03-10 | The dial reveals the operator's reachability to an endpoint they configured. Parsing the read verb's output instead would buy a third-party output-format dependency, rejected by D-08. Documented in `claudecode.go`. | Sean | 2026-09-08 |
| AR-04 | T-03-13 | `mcp list` names the operator's other registered servers, but this is disclosure to the operator, on their own terminal, of their own machine's output. Bounded and scalar-typed so the value cannot flood or bypass sanitization. | Sean | 2026-09-08 |
| AR-05 | T-03-15 | Third-party behavior engram cannot detect (it never observes the resolved header, by design) and cannot gate on without asserting third-party behavior. Mitigated by documentation: `opencode.go` records the failure signature and names the upstream issue. | Sean | 2026-09-08 |
| AR-06 | T-03-23 | A bare `setup` dials the configured URL for claude-code and opencode. Accepted as the price of showing present state next to intended state, and made discoverable by reading: `setupPreviewSummary` and `--help` both state it. codex's probe is a pure local read. | Sean | 2026-09-08 |
| AR-07 | T-03-SC | Zero external packages added across the phase; every mechanism is Go stdlib. `leafpurity_test.go` mechanically fails any change that adds a non-stdlib import reachable from `internal/setup`. | Sean | 2026-09-08 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-09 | 28 | 27 | 1 | gsd-security-auditor |
| 2026-09-09 | 28 | 28 | 0 | orchestrator (T-03-04 remediated) |

### Audit 2026-09-09 — T-03-04

Opened by the audit as a blocker (`high` ≥ `block_on: high`).

The threat's stated sub-controls were fully implemented: every capture byte-bounded via
`boundCapture`, and every new field a plain Go `string` so `viewScalar` routes it through
`sanitizeViewValue`. Those close the flooding and container-bypass facets only.
`sanitizeViewValue` maps just `r < 0x20 || r == 0x7f`, so every shell metacharacter passed
through untouched — phase 02's T-02-13 (control chars, mitigated) mistaken for T-02-05 (shell
metacharacters, not covered).

`registered=`, `reason=` and `notes=` are the first report fields carrying bytes engram did not
author, and they render adjacent to `Command`, which is quoted precisely because a human may
paste it. Reached the same conclusion independently in `03-REVIEW.md` (Warning 2).

Accepting the residual was rejected: AR-04's "your own machine, your own terminal" argument
answers an Information Disclosure concern, and does not transfer to a code-execution risk
triggered by a paste.

Remediated by `displayCapture` (bound, then `quoteWord`) applied at all five capture sites in
`internal/setup/apply.go`. Placed in this package rather than in `sanitizeViewValue`, which is
shared by every operator view in the CLI and whose other consumers legitimately carry arbitrary
printable characters. RED commit `1aeef675`, GREEN commit `63bbb065`, pinned by
`TestThirdPartyCaptureIsQuotedForDisplay` including a positive control that a safe capture stays
unquoted and a regression guard that convergence classification is unaffected.

### Non-register observations

- `03-REVIEW.md` CR-01 (panic on a non-first empty `Args`) was already remediated before this
  audit — `apply.go` validates every action's `Args`.
- `03-REVIEW.md` Warning 4 (`describeSeamError` not bounded via `boundCapture`) is confirmed
  present and correctly out of scope for T-03-04: it formats a Go error value, never third-party
  captured output. Tracked as code quality, not a threat-register gap.
- No `## Threat Flags` section exists in any phase SUMMARY; nothing to reconcile as an
  unregistered flag.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
