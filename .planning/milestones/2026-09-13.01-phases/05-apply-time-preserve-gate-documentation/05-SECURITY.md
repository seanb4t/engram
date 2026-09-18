---
phase: "05"
slug: "apply-time-preserve-gate-documentation"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-17"
---

# Phase 05 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| vendor CLI stdout/stderr (probe1, probe2) → `Observe` | Untrusted third-party text; parsed by the runtime's own scanner, compared, never executed, never rendered raw on a compared row. | Pre- and post-write registration probe text (untrusted, may carry literal header values) |
| classification → write-action loop | The single place `plan.Actions` is ever run; the classification must return BEFORE the loop's first iteration. | `preserved` / `already-correct` verdict |
| `Observation` (runtime-authored sentences) → `Result.Reason` / `Notes` | Engram-authored fixed constants cross; the executor appends them without reading their content. | Remediation / consequence sentences |
| `Result` → operator terminal / `--output json` (incl. `--apply`) | Flat scalar strings only; observation-derived text bounded by `boundCapture`. | Rendered rows |
| `setupCmd.Long` / `help.golden` / guide text → operator | Engram-authored prose; must describe only behaviour that exists (no phantom flag, no false overwrite claim, no false availability claim). | Help and guide text |
| docs-gate tests → repo filesystem | Read Markdown files relative to `cmd/engram`; never a CLI, never `$HOME`. | Guide content |
| planning artifact → milestone audit / verifier | `05-POST-RELEASE.md` frontmatter is parsed by tool-owned readers; an invented key/heading silently changes what the audit sees. | Handoff frontmatter |
| executor → GitHub | One issue is created; nothing is closed, assigned, labelled by guess, or tagged. | Issue #567 |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-05-01 | Tampering / self-inflicted DoS | `preserved` classification failing to short-circuit BEFORE `plan.Actions[0]` (claude-code's tolerant `mcp remove`) — the ryr82bf2s2 incident class | high | mitigate | `internal/setup/apply.go`: `case OutcomeAlreadyCorrect, OutcomePreserved: return res` (line 493–494) sits above the write loop at line 506; `TestApplyPreservedNeverRunsClaudeCodeRemove` (`apply_test.go`, argv negative) and `TestSetupApplyPreservedRuntimeSkipsRegistrationWrite` (`cmd/engram/setup_test.go`, process boundary) pin it | closed |
| T-05-02 | Repudiation / Tampering | silent OAuth logout as a side effect of a legitimate `would-write` rewrite on claude-code | high | mitigate | `RewriteConsequence` set by `Observe` on `AuthNone` (`plan.go`, `claudecode.go`), rendered on `Notes` in both lanes ahead of the remove record; `TestOAuthReLoginConsequence` (`claudecode_test.go`) positive + negatives | closed |
| T-05-03 | Information Disclosure | literal header value echoed by the POST-write probe reaching `Registered` / `--output json` under `--apply` | high | mitigate | Compared rows rebuild `Registered` via `Observe → renderObservation(obs2)` (empty when unframeable); `TestApplyWroteRegisteredIsRedacted` asserts sentinel absence from `json.Marshal(Result)`; `TestSetupJSONNeverLeaksProbeLiteral/claude-code-observed-shape` extended with an `apply` mode in both lanes | closed |
| T-05-04 | Tampering | ambiguous read (format drift, seam error) treated as licence to skip the write — silent non-registration | high | mitigate | Ambiguity falls through to the unchanged write-then-byte-compare path; `TestApplyConvergesClaudeCode/first-run-wrote`, `TestApplyConvergesCodex/first-run-wrote`, `TestThirdPartyCaptureIsQuotedForDisplay` green unchanged; `TestOAuthReLoginConsequence/ambiguous-read-no-note` | closed |
| T-05-05 | Elevation of Privilege | a second consent surface (overwrite/replace/force flag) in code, help, or guide, re-opening the incident class | medium | mitigate | Not built (D-05): zero flag definitions named `replace`/`force`/`overwrite` in `cmd/engram`; the only `--replace`/`--force`/`--overwrite` hits are comments in `claudecode.go:36` and `toolclass.go:195` stating no such flag exists; guide states there is no flag to suppress the rewrite; remediation is a manual step in the runtime's own tool | closed |
| T-05-06 | Information Disclosure | remediation/consequence sentence composed beside observation-derived facet text flooding a rendered field | low | mitigate | `boundCapture` applied to `Reason`/`Drift`/`Registered` after composition inside `renderClassification` (`apply.go`); the sentences are fixed engram constants | closed |
| T-05-07 | Tampering | `opts.Auth`/`opts.Headers` consulted inside the gate to let argv intent override `preserved` | medium | mitigate | `apply.go` has zero `opts.Auth`/`opts.Headers` references; only `Compare` reads opts; Phase 4 D-01 carried forward | closed |
| T-05-08 | Tampering (operator misled) | guide's stale `already-correct` sentence / "may perform writes again" clause contradicting D-01, or preserved row claiming `--apply` may overwrite | medium | mitigate | `agentSetupGuideStaleClaimAnchors` zero-occurrence anchors + reworded rows gated by legs 9–11 in `cmd/engram/agent_setup_docs_test.go`, each with a positive control | closed |
| T-05-09 | Repudiation | OAuth re-login consequence undocumented before `--apply` | medium | mitigate | Help sentence pinned by `TestSetupHelpStatesApplyGate` (`setup_test.go`); guide leg 12 (`OAuth` + `log in again` + `notes` on one line) | closed |
| T-05-10 | Tampering | `help.golden`/`catalog.golden` regenerated as a side effect or drifting from the live cobra tree | low | mitigate | Explicit `-update -count=1` only; `TestHelpGolden`/`TestCatalogGolden` green; `git status --porcelain` empty over generated paths | closed |
| T-05-11 | Repudiation (false availability) | `install.md`/`plugin.md` claiming man pages or plugin-first delivery exist in v0.16.1 | medium | mitigate | `Unreleased as of v` aside present in each (`docs-site/src/content/docs/guides/{install,plugin}.md`, 1 each), gated by `install_docs_test.go` / `plugin_docs_test.go`; `05-POST-RELEASE.md` step replaces them after observation | closed |
| T-05-12 | Tampering (operator misled) | documenting the hidden `engram man <dir>` verb | low | mitigate | `install.md` has zero `engram man ` occurrences; the cask hook is the only producer | closed |
| T-05-13 | Information Disclosure | `preserved` operator copying the fallback `claude mcp remove` blindly, destroying the unreproducible entry | medium | mitigate | Cross-link sentence beside the fallback block (guide gate leg 3) routes to the results section's remediation, which names what is being preserved first | closed |
| T-05-14 | Denial of Service (CI) | docs gate skipping silently on a trimmed checkout or passing on an empty file | low | mitigate | Both live-file tests `t.Skipf` explicitly on absence and `t.Fatalf` on empty content (`install_docs_test.go`, `plugin_docs_test.go`) | closed |
| T-05-15 | Repudiation | REQ-docs-setup-v2 checked off, or `post_release_status: complete`, from code alone | high | mitigate | `REQUIREMENTS.md:61` still `- [ ] **REQ-docs-setup-v2**` and `:113` reads `Pending`; no `05-RELEASE-*.md` exists; `05-POST-RELEASE.md` carries `post_release_status: pending`; the SUMMARY states the handoff explicitly for the verifier | closed |
| T-05-16 | Tampering | invented frontmatter key/heading in `05-POST-RELEASE.md` or a stub milestone heading in ROADMAP.md truncating tool-owned parsers (rule 8dfdhfs5nn) | medium | mitigate | `05-POST-RELEASE.md` has exactly 3 frontmatter keys and exactly 3 `## ` headings (verified); ROADMAP/STATE never written by a task | closed |
| T-05-17 | Elevation of Privilege | executor publishing more than the sanctioned issue (closing, tagging, assigning) | medium | mitigate | Issue #567 verified via `gh`: state `OPEN`, 0 assignees, 0 labels; handoff states "Do not mint a tag" | closed |
| T-05-18 | Denial of Service (CI) | a gate silently green because it examined nothing | low | mitigate | Whole-package `-count=1` tests and the repo's own `task`; every `rg` gate is a positive-presence check or a counted `-o \| wc -l` | closed |
| T-05-SC | Tampering | npm/pip/cargo installs | low | accept | No new packages; docs build uses the committed lockfile with `--frozen-lockfile`; gate diffs `go.mod`/`go.sum` and the docs-site lockfile — see AR-05-01 | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-05-01 | T-05-SC | No package-manager install task in any of the four plans; `internal/setup` remains stdlib-only; `go.mod`/`go.sum` and `docs-site/pnpm-lock.yaml` byte-unchanged (`--frozen-lockfile`). Supply-chain control is not applicable. | Plan author (05-01..04 threat models) | 2026-09-17 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-17 | 19 | 19 | 0 | /gsd-secure-phase (orchestrator, L1 grep-depth short-circuit — register authored at plan time, ASVS L1) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-17
