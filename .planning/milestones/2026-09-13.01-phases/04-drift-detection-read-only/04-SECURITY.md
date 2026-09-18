---
phase: "04"
slug: "drift-detection-read-only"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-17"
---

# Phase 04 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| vendor CLI stdout/stderr → `Observe` | `codex mcp get --json` / `claude mcp get` output is untrusted third-party data: decoded strictly (codex) or line-classified by prefix (claude), compared for equality in locals, never interpolated into argv, never executed, never rendered verbatim. | Registration probe text incl. possible literal header values (untrusted, secret-bearing) |
| `Observation` → `Result` (rendered fields) | Only engram-composed text built from typed fields (names, labels, states, a URL) crosses into `Registered`/`Drift`/`Reason`/`Facets`; header values do not cross at all. | Names, states, labels, redacted URL |
| `setup.Result` → `setupRuntimeRow` → `renderOperator` (text / `--output json`) | Already-redacted scalars cross into the operator's terminal and into scripts consuming JSON; nothing re-derives probe content. | Flat scalar strings |
| maintainer's shell → `04-OBSERVATIONS.md` (git) | Real CLI output pasted into a committed planning file — permanent history. | Observed CLI output (throwaway registration only) |
| `04-OBSERVATIONS.md` → test fixtures | Fixtures' honesty depends on the record's provenance (D-08 citation). | Quoted capture text |
| docs-site / `--help` → operator | Prose an operator acts on before `--apply`; a wrong claim (e.g. "opencode is compared") is a false sense of safety. | Drift-outcome vocabulary |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-04-01 | Information Disclosure | literal header value / bearer variable echoed by `codex mcp get --json` reaching `Registered`, `Reason`, `Drift`, `--output json`, or a log | high | mitigate | Redaction by construction: values compared in `rawHeader` locals (`drift.go`, `codex.go`, `claudecode.go`) and never assigned; `Observation`/`ObservedHeader` (`drift.go:114,244`) carry no value field; `Registered` is `renderObservation(obs)`; `TestRedactionUnconditional`, `TestPreviewClassifiesRegistration` assert sentinel absence from `json.Marshal(Result)`; `TestPreviewAmbiguityResolvesToWouldWrite` proves a not-compared read renders zero probe bytes | closed |
| T-04-02 | Tampering | codex release adding a field to `mcp get --json`, silently redefining "whole registration matches" (false `already-correct` → Phase 5 overwrite) | high | mitigate | D-11 totality: `DisallowUnknownFields` gate + key-set diff in `codex.go` → any unknown key / non-null never-set field is `unrecognized-content` → `preserved`; pinned by `unknown-transport-key`/`two-unknown-keys-sorted`/`enabled-false` subtests | closed |
| T-04-03 | Information Disclosure | `preserved` Reason/facet composed via `%v` over the observation carrying a raw fragment | medium | mitigate | `Compare` composes every line from `Name`/`State`/`Planned`/label strings and fixed literals; `Observation` has no value field; `displayCapture(probe1` count is 0 in code | closed |
| T-04-04 | Information Disclosure | `claude mcp get` dialing the registered URL during read-only preview | low | accept | Pre-existing probe behaviour (Phase 3 lane); this phase adds no dial — it parses the existing probe's output; codex's probe is a local read — see AR-04-02 | closed |
| T-04-05 | Information Disclosure | observed URL with userinfo (`https://user:pass@host`) rendered in `Registered`/`Drift` | medium | mitigate | `displayURL` renders through stdlib `url.URL.Redacted()` (`drift.go`) when userinfo is present; raw URL still compared byte-for-byte; pinned by `url-userinfo-redacted` (`codex_test.go`) | closed |
| T-04-06 | Tampering | hostile observed header name / unknown key / URL pasted from the report into a shell | low | mitigate | Every observed name, label, URL on `Registered`/`Drift`/`Reason` passes through `quoteWord` (18 code uses); labels bounded (`claudeCodeUnrecognizedLabelBound = 40`) | closed |
| T-04-07 | Denial of Service | flooding `codex mcp get` stdout | low | mitigate | Nothing renders probe bytes; unknown-key labels bounded; `runSeam` `execTimeout` bounds the probe | closed |
| T-04-08 | Information Disclosure | real credential pasted into `04-OBSERVATIONS.md` and committed | high | mitigate | Protocol registers a throwaway entry with fixed dummy `sk-DO-NOT-COMMIT-literal-test-abc123`; the committed record's only `sk-` tokens are that dummy (8 occurrences, verified); test fixtures carry only the dummy plus non-secret `sk-` identifiers | closed |
| T-04-09 | Tampering | protocol overwriting/removing the maintainer's real `engram` registration or real `~/.codex/config.toml` | high | mitigate | Throwaway name `probe-literal-04` on both runtimes; Codex under isolated `CODEX_HOME=/tmp/engram-04-observe`, deleted afterwards; checklist requires post-remove not-found confirmation (recorded in `04-OBSERVATIONS.md`) | closed |
| T-04-10 | Information Disclosure | dial to the throwaway URL during `claude mcp get` | low | accept | URL is `127.0.0.1:1` — loopback, never answers; nothing leaves the machine — see AR-04-03 | closed |
| T-04-11 | Repudiation | fixture claiming an observed shape the record does not support | medium | mitigate | D-08: every 04-05 fixture cites `04-OBSERVATIONS.md` (25 citations across `setup_test.go`, `codex_test.go`, `drift_test.go`, `claudecode_test.go`); `## What this pins` bullets quote the capture | closed |
| T-04-12 | Repudiation | guide silently losing the `preserved` row or the opencode statement in a later edit | medium | mitigate | `agentSetupGuideDriftViolations` (`cmd/engram/agent_setup_docs_test.go`) fails the build on a missing preserved row, missing whole-entry/never-merge/never-shown clauses, `would-write` without `facets`, or no opencode-not-compared line; positive control proves each leg | closed |
| T-04-13 | Information Disclosure | docs example showing a header VALUE | low | mitigate | Only example is `x-gateway-api-key: observed <redacted>, would write ${GATEWAY_KEY}`; preserved row states values are never shown (gate leg 4) | closed |
| T-04-14 | Information Disclosure | probe-read literal reaching stdout/stderr through the row renderer in either lane | high | mitigate | Row copies only redacted scalars (`Registered`/`Facets`/`Drift` are `string`, `setup.go:509,527,528`); `TestSetupJSONNeverLeaksProbeLiteral` asserts sentinel absence on both streams in both lanes; ambiguous-read subtests assert `Registered == ""` | closed |
| T-04-15 | Tampering | nested facet field bypassing `sanitizeViewValue`'s scalar-only branch | medium | mitigate | `Facets`/`Drift` are `string`; new fixture rows walked by `TestOperatorViewFixturesHaveNoUnsanitizedNesting` | closed |
| T-04-16 | Repudiation | `preserved` registration laundered into `already-correct` at the row headline | medium | mitigate | Fold is `setup.AggregateOutcome` (`aggregate.go:55`) over `precedenceOrder` (preserved > already-correct); `setupOutcomeFoldTable` pins `{preserved, would-write} → preserved`; `registration=preserved` rides beside the headline in both lanes | closed |
| T-04-17 | Spoofing | `--help`/headline claiming opencode is compared | low | mitigate | Both texts state opencode is not compared; `TestSetupHelpNamesDriftOutcomes` and `TestSetupPreviewSummaryNamesComparison` pin the phrases; docs gate (T-04-12) pins the guide | closed |
| T-04-18 | Information Disclosure | literal header value in `claude mcp get`'s `Headers:` block reaching any rendered field (gotcha ryr82bf2s2's exact shape) | high | mitigate | Values split at first `": "` compared in a `rawHeader` local (`claudecode.go`), never assigned to `Observation`; `TestObserveClaudeCodeRegistration/unplanned-header-literal-observed`, `TestRedactionUnconditional/claude-code-observed-literal`, `TestSetupJSONNeverLeaksProbeLiteral/claude-code-observed-shape` assert absence at every layer | closed |
| T-04-19 | Tampering | Claude Code release adding a label/line, silently changing what "matches" means | high | mitigate | D-11 total parse: any unclassified line → unrecognized content → `preserved` naming its label; pinned by `unrecognized-label`/`unrecognized-unlabeled-line`/`type-not-http` (`claudecode_test.go`) | closed |
| T-04-20 | Denial of Service (of the comparison) | live connection-status line varying with reachability producing spurious `preserved` or blocking `already-correct` | medium | mitigate | `Status:` is chrome by prefix; `execute()` parses regardless of exit code; pinned by `status-failed-dial-is-chrome` and failed-dial-exit `Preview` subtests | closed |
| T-04-21 | Information Disclosure | unrecognized line's remainder (potentially a value) copied into the reason | medium | mitigate | `claudeCodeLineLabel` copies only text before the first colon, bounded to 40 bytes, `quoteWord`ed; colon-less line contributes fixed token `line` | closed |
| T-04-22 | Repudiation | fixture claiming an echo shape the record does not support | medium | mitigate | Every literal-echo fixture cites `04-OBSERVATIONS.md`; fixture builder's header comment names record section, CLI version, date | closed |
| T-04-23 | Tampering | hand-edit-only codex key added to the known-key list, laundering a foreign field into `already-correct` | medium | mitigate | Rule: a key joins the mirror/known lists only if the record shows engram's own `mcp add` producing it; hand-edit-only keys stay unknown → `preserved` (`enabled-false` subtest) | closed |
| T-04-SC | Tampering | npm/pip/cargo installs | low | accept | Not applicable — zero new packages across all five plans; stdlib-only leaf preserved; `go.mod`/`go.sum` byte-unchanged — see AR-04-01 | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-04-01 | T-04-SC | No package-manager install task in any of the five plans; `internal/setup` remains stdlib-only; `go.mod`/`go.sum` byte-unchanged. Supply-chain control is not applicable. | Plan author (04-01..05 threat models) | 2026-09-17 |
| AR-04-02 | T-04-04 | `claude mcp get` dials the registered URL as part of its own output — vendor behaviour engram does not own (rule m45p2b4bp7). The preview adds no dial; it only parses output the Phase 3 probe already produced. (04-01-PLAN cites this as "T-03-10 in 03-SECURITY.md"; no such entry existed — the acceptance is recorded here.) | Plan author (04-01 threat model) | 2026-09-17 |
| AR-04-03 | T-04-10 | The observation protocol's throwaway URL is loopback `127.0.0.1:1`, which never answers; nothing leaves the maintainer's machine. | Plan author (04-02 threat model) | 2026-09-17 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-17 | 24 | 24 | 0 | /gsd-secure-phase (orchestrator, L1 grep-depth short-circuit — register authored at plan time, ASVS L1) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-17
