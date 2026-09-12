---
phase: "02"
slug: "setup-command-core"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-08"
---

# Phase 02 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

Register origin: **authored at plan time** — all three PLAN.md files carried a
`<threat_model>` block, so this audit verifies declared mitigations rather than
retroactively deriving a register. Blocking threshold: `high`. ASVS level 1
(grep-depth verification).

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| caller argv → engram CLI | `--url`, `--auth`, `--runtime`, `--token-file`; argv is world-readable via the process table on a shared host | endpoint URL, auth mode, a credential **path** (never the credential) |
| process environment → engram CLI | `ENGRAM_URL` / `ENGRAM_AUTH` / `ENGRAM_RUNTIME` became live inputs in plan 02-03; an exported shell-profile variable now influences what `setup` would write | endpoint URL, auth mode |
| local filesystem `PATH` → engram | `exec.LookPath` resolves a runtime name against whatever `PATH` names | resolved binary path (**never executed this phase**) |
| engram → operator terminal | the rendered report, including the previewed `mcp add` command string, crosses into a terminal that interprets control sequences | report rows, authored command strings |
| engram → CI / calling script | the process exit status is the machine-readable contract a caller branches on | exit code (0 / 2 / 8 / 9) |
| engram → third-party runtime CLI | **NOT crossed this phase** — `Apply()` is stubbed (D-09). Recorded so Phase 3 inherits the boundary rather than rediscovering it | (none yet) |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-02-01 | Information Disclosure | `internal/setup.Plan()` bearer-mode invocation string | high | mitigate | `--token-file` carries a PATH, never the secret (D-05). Verified: **zero** `os.ReadFile`/`os.Open`/`os.Stat` calls in `internal/setup` production code — the only hits are in `leafpurity_test.go`, the test that enforces the constraint. `bearerProvenance` (`plan.go:88-103`) emits `Bearer <from PATH>`; pinned by `TestSetupBearerTokenFileRedactedInOutput` (`setup_test.go:266`) | closed |
| T-02-02 | Tampering | text-lane rendering of any `Plan()`-authored command string | medium | mitigate | Verified: `cmd/engram/setup.go` contains no `fmt.Print`, `json.Marshal`, or `os.Stdout` write — every value renders through `renderOperator` → `renderOperatorView` → `sanitizeViewValue` (`operator_view.go:223`) | closed |
| T-02-03 | Elevation of Privilege | the `--apply` preview/mutate gate | high | mitigate | Verified: `internal/config/registry.go` declares **no** row whose `Flag` is `apply`, so no `ENGRAM_` variable can flip the gate (D-04). Pinned by `TestDestructiveModeIgnoresEnvironment` (`destructive_test.go:496`), which sets every registry-declared variable and asserts the preview branch still runs | closed |
| T-02-04 | Spoofing | `exec.LookPath` runtime detection | low | accept | A hostile binary earlier on `PATH` named `claude`/`codex`/`opencode` reads as present. `exec.LookPath` is the locked detection mechanism (D-12); engram executes nothing this phase; `PATH` integrity is the operator's own boundary. **Re-evaluate in Phase 3**, where the resolved binary is actually invoked | closed — accepted |
| T-02-05 | Tampering | a `--url` / token-file path carrying shell metacharacters | low | accept | **Rationale corrected — see Accepted Risks R-02-01 and issue #523.** The original accept reasoning ("nothing in Phase 2 passes the URL to `os/exec` or a shell") is true for *programmatic* execution and was re-verified: the only `os/exec` use is `exec.LookPath`. It did not account for the *previewed* command string, which this phase ships and whose stated purpose is to be copy-pasted into a shell | closed — accepted, tracked to #523 |
| T-02-06 | Repudiation | `setupApplyRun`'s failure path | high | mitigate | Verified: `renderOperator` is called at `setup.go:316`, unconditionally, before any error return — a nonzero exit cannot erase the per-runtime record of what happened | closed |
| T-02-07 | Tampering | the exit-code taxonomy's five declaration sites | medium | mitigate | Verified: `TestCatalogExitCodesMatchMapper` set-equality in both directions plus `nonConnectProducedCodes`' named allowlist (`catalog_test.go:335-347`) make a code advertised-without-provenance, or produced-without-advertisement, a test failure rather than a silent divergence | closed |
| T-02-08 | Elevation of Privilege | `setupPreview` gaining a nonzero-exit path | medium | mitigate | Verified structurally: `setupPreview` (`setup.go`) has exactly one terminal statement, `return renderOperator(...)`, and `setupExitCode`'s **only** caller is `setup.go:331` inside `setupApplyRun`. A preview cannot produce a nonzero code (D-08) | closed |
| T-02-09 | Denial of Service | a not-installed runtime classified as a failure | medium | mitigate | Verified: `internal/setup/exit.go` treats `OutcomeNotPresent` as neither an attempt nor a failure. Without this, `engram setup` would fail every CI build on a machine that simply lacks a runtime | closed |
| T-02-10 | Information Disclosure | the bearer credential vs. the new `ENGRAM_*` environment lane | high | mitigate | Verified: `ENGRAM_AUTH` selects an auth **mode** only. `internal/config/registry.go:91` declares `client.token_file` with `Flag: "token-file"` and **no `Env:` row** (D-05, on the `client.token_file` D-13 precedent), so no secret reaches argv or the process table. Redaction-by-provenance survives the new lane | closed |
| T-02-11 | Spoofing | an exported `ENGRAM_URL` redirecting registration to an attacker endpoint | medium | mitigate | Preview-by-default (D-14) renders the exact resolved URL for every runtime before anything is written, and `--apply` is stubbed (D-09). `--url` overrides the environment (`TestSetupFlagBeatsEnvForURL`), and `--help` names the variable so its influence is discoverable rather than hidden | closed |
| T-02-12 | Elevation of Privilege | an environment variable flipping preview into mutation | high | mitigate | Same control as T-02-03, verified independently: no registry row for `apply`, asserted by a Task 1 acceptance criterion and a `must_haves` prohibition so a later "consistency" edit that enrolls it fails the gate | closed |
| T-02-13 | Tampering | a URL carrying **control characters** forging report structure | low | mitigate | Verified: `sanitizeViewValue` (`operator_view.go:223`) maps `r < 0x20 \|\| r == 0x7f` to a space on every field and on the headline. **Scope note:** this covers C0 controls and DEL only — it does **not** strip shell metacharacters, which are printable. It does not close T-02-05; the two are adjacent but orthogonal | closed |
| T-02-14 | Denial of Service | the new required-URL guard failing a previously-succeeding scripted invocation | low | accept | A bare `engram setup` with no URL previously emitted a malformed command and exited 0; it now exits 2. This is the intended correction (WR-01) — the prior "success" was a silently broken command — and the phase has not shipped, so no released caller depends on the old behavior | closed — accepted |
| T-02-SC | Tampering | dependency supply chain | low | accept | This phase adds zero packages to `go.mod` and runs no `npm`/`pip`/`cargo` install (`02-RESEARCH.md` § Package Legitimacy Audit: no external-package surface). A task that would add a `require` line must halt and re-open this row | closed — accepted |

*Status: open · closed · open — below `high` threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `workflow.security_block_on` count toward `threats_open`*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

All five `high` threats (T-02-01, T-02-03, T-02-06, T-02-10, T-02-12) are dispositioned
`mitigate` and each was verified against the implementation, not against its SUMMARY claim.

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| R-02-01 | T-02-05 | **Shell metacharacters in a previewed command string.** All seven `Command:` constructions in `internal/setup/{claudecode,codex,opencode}.go` interpolate `opts.URL` — and, in bearer mode, the token-file path via `bearerProvenance` — with no shell quoting. Accepted **for Phase 2 only**, on two grounds that were re-verified this audit: (1) nothing in Phase 2 executes the string — the sole `os/exec` use is `exec.LookPath`, and `Apply()` is stubbed per D-09; (2) severity is `low`, below the `high` blocking threshold. **The original rationale was too narrow** and is corrected here: it reasoned only about programmatic execution and did not account for the previewed string itself, whose documented purpose (`setup.go`'s own doc comment: "reports the exact command it would issue") is to be copy-pasted into a shell — and whose likely consumers are the coding agents named in `--runtime`, which act on such text programmatically. Tracked as issue #523 and **a precondition of Phase 3**, which is the change that turns this from a copy-paste risk into an unattended sink. Note that `plan.go:11-16` commits Phase 3's `Apply()` to consuming these strings verbatim, never re-deriving them | sean | 2026-09-08 |
| R-02-02 | T-02-04 | **`PATH`-based runtime spoofing.** A hostile binary earlier on `PATH` named `claude`/`codex`/`opencode` would be reported present. Accepted because `exec.LookPath` is the locked detection mechanism (D-12), engram executes nothing this phase, and `PATH` integrity is the operator's own trust boundary. **Must be re-evaluated in Phase 3**, where the resolved binary is actually invoked — at which point the disposition may no longer hold | sean | 2026-09-08 |
| R-02-03 | T-02-14 | **Exit-status change for a bare `engram setup`.** Previously exited 0 with a malformed command; now exits 2. Accepted as the intended correction of WR-01 — the prior success was a silently broken command — and the phase has not shipped, so no released caller depends on the old behavior | sean | 2026-09-08 |
| R-02-04 | T-02-SC | **Supply chain.** No accepted exposure: the phase adds zero `go.mod` requirements and runs no package-manager install, so there is no package-legitimacy surface to gate. Recorded so that a future task adding a `require` line must halt and re-open the row | sean | 2026-09-08 |

---

## Notes for Phase 3

Two rows above are explicitly deferred **to** Phase 3 and should be re-opened when it is planned:

- **T-02-05 / R-02-01 → issue #523.** The threat model's own prescribed remedy is *"use `exec.Command`'s argument-list form rather than shell-string interpolation."* That is a **stronger** control than the `shellQuote` helper proposed in `02-REVIEW.md`: with an argv-form exec there is no shell to inject into, so quoting becomes unnecessary on the execution path. Both are needed, and they cover different paths — argv form for what `Apply()` *runs*, quoting for what the preview *displays* and a human may paste. Fixing only the execution path leaves the copy-paste vector open.
- **T-02-04 / R-02-02.** `exec.LookPath` spoofing stops being theoretical the moment Phase 3 invokes the resolved binary.

Also inherited: the `engram → third-party runtime CLI` trust boundary, recorded above but not crossed this phase.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-08 | 15 | 15 | 0 | gsd-secure-phase (orchestrator, ASVS L1 grep-depth) |

Verification method: each `mitigate` row was checked against the implementation
directly (`rg` over the named files and tests), not against the corresponding
SUMMARY.md claim. Two register rows were found to carry reasoning narrower than
the risk they name (T-02-05's execution-only scope; T-02-13's control-character-only
scope) and both are corrected in place above.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-08
