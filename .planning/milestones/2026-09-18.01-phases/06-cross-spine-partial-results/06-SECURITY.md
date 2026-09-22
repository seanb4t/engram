---
phase: "6"
slug: "cross-spine-partial-results"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-20"
---

# Phase 6 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

The register was authored at plan time across all three plans' `<threat_model>` blocks
(`register_authored_at_plan_time: true`), so this audit verified that each declared mitigation is
present in the shipped implementation rather than scanning for new threats. ASVS L1 is configured;
the authz-boundary placement, absence-vs-empty ordering, flag mutual-exclusivity and wire-leak
threats were verified at L2 depth because they are this phase's real surface.

The audit was not grep-only. It diffed `5b75fa01..HEAD` to prove the authz-producing recall calls
are byte-identical to pre-phase, ran the phase's test set fresh, and **hand-executed two
red-evidence attacks end-to-end** — applying `06-01-empty-scopes-substituted-for-absence.patch` and
`06-01-helper-swallows-listscopes-error.patch`, confirming a genuine test FAILURE (not a build
break) in each case, then reverting and confirming a byte-clean tree. The tree was porcelain-clean
at the end of the audit (only the pre-existing untracked `.planning/milestone.lock` remains).

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Client → recall handlers (MCP + Connect) | `search_memory` / `list_memory` with `cross_spine=true`. **The authz boundary is the recall itself** — `d.searchMemory` / `d.listMemory` decide what the caller may see. This phase's change lives strictly *after* that call | Caller identity, `cross_spine` flag; hits already filtered by owner/visibility |
| `deps.searchedScopes` → `Store.ListScopes` | The failure path this phase exists for. The helper now absorbs a `ListScopes` error into `scopeCoverage{Unknown: true}` **without** returning the error value | Scope names the caller may read; on failure, nothing |
| Server → wire (proto response) | New `scopes_unknown` field on both responses. Must carry a *flag*, never the underlying error text or partial scope names | `searched_scopes`, `scopes_truncated`, `scopes_unknown` |
| Server → operator logs | The error that does **not** cross the wire must still be recorded, or the failure becomes unattributable | `slog.ErrorContext` with the raw `ListScopes` error |
| CLI → operator stdout | `engram search` / `engram list` footer gains a third mutually-exclusive form | `scopes_unknown: true`, or the truncated/count forms — never scope names |

---

## Threat Register

24 threats, all closed. Verified 2026-09-20 by `gsd-security-auditor` against the shipped tree.

| Threat ID | Category | Severity | Disposition | Mitigation | Status |
|-----------|----------|----------|-------------|------------|--------|
| T-06-01-01 | Elevation of Privilege | high | mitigate | `git diff 5b75fa01..HEAD -- internal/server/{connectapi.go,tools.go}` removes only the abort branch; `a.d.listMemory`/`a.d.searchMemory` (Connect) and `d.searchMemory`/`d.listMemory` (MCP) are byte-identical to pre-phase. `cov` is computed strictly *after* hits, so no authz decision moved | closed |
| T-06-01-02 | Information Disclosure | high | mitigate | `tools.go:1849-1852` logs then `return scopeCoverage{Unknown: true}` — `scopeCoverage` carries no error field, so no error value can reach a caller. `crossspinecoverage_test.go` asserts the injected sentinel text is absent from the response | closed |
| T-06-01-03 | Spoofing | high | mitigate | `recallResultMap` (`tools.go:1889-1899`) returns immediately on `cov.Unknown`; `cov.Scopes` stays `nil`, never an allocated empty slice — an empty list is never substituted for absence. **Independently reproduced**: patch applied → genuine FAIL → reverted clean | closed |
| T-06-01-04 | Repudiation | high | mitigate | The same `slog.ErrorContext(ctx, "searchedScopes: ListScopes failed", "error", err)` fires exactly once per failure; observed live in a fresh test run | closed |
| T-06-01-05 | Tampering | high | mitigate | `scopes_unknown = 7` / `= 4` are next-free and additive (no renumber, no reuse of the deprecated `approximate=3`); `connectdescriptor_test.go` pins both field counts and each field | closed |
| T-06-01-06 | Tampering | medium | mitigate | `task proto:gen && git diff --exit-code` clean; `gen/go`, `gen/ts` and `ui/src/lib/gen` all carry `ScopesUnknown`/`scopesUnknown` | closed |
| T-06-01-07 | Denial of Service | low | accept | No new backend call: `ListScopes` was already invoked post-recall pre-phase (confirmed by the minimal diff). Returning completed hits is strictly cheaper than discarding them | closed |
| T-06-01-SC | Tampering | low | accept | No `go.mod`/`go.sum` or package-manifest change in the phase diff | closed |
| T-06-02-01 | Spoofing | high | mitigate | `client_common.go:341-350` — `renderCoverageFooter` tests `scopesUnknown` FIRST and returns inside that branch, so unknown can never render as a count or as truncation | closed |
| T-06-02-02 | Tampering | medium | mitigate | Exactly one `renderCoverageFooter` definition; `client_list.go:94` and `client_search.go:82` each call it once with `GetScopesUnknown()` — no divergent second renderer | closed |
| T-06-02-03 | Repudiation | medium | mitigate | `guides/upgrade.md` Unreleased section gained one numbered subsection (17 total); `scopes_unknown` documented twice | closed |
| T-06-02-04 | Denial of Service | medium | mitigate | New tests assert `err == nil` from `runClient` — real success, not absence-of-panic. No `os.Exit` or stderr write added (D-05: coverage-unknown is not an error exit) | closed |
| T-06-02-05 | Information Disclosure | medium | mitigate | The unknown branch prints only the literal `scopes_unknown: true`; the known branch prints `len(searchedScopes)`, never names | closed |
| T-06-02-06 | Tampering | low | mitigate | No SPDX header added to any of the three docs pages (frontmatter rule); exactly one `## Unreleased` heading — not renamed or duplicated | closed |
| T-06-02-SC | Tampering | low | accept | No manifest change in the phase diff | closed |
| T-06-03-01 | Tampering | medium | mitigate | `redevidence_harness_test.go:349-364` — `defer revert()` + `t.Cleanup(revert)`, both idempotent, with `t.Fatalf` (never a silent pass) if the revert fails or leaves the tree dirty; refuses to run at all on an already-dirty target | closed |
| T-06-03-02 | Repudiation | high | mitigate | Harness step 2 self-detects staleness (`runErr == nil` → `t.Fatalf`). **Independently reproduced** for 2 of 5 patches, including the forbidden-fix patch: apply → genuine FAIL → clean revert | closed |
| T-06-03-03 | Repudiation | high | mitigate | All 5 phase-6 target test names confirmed to exist and actually run — fresh `go test -run` invocations show real PASS/FAIL, never "no tests to run" | closed |
| T-06-03-04 | Tampering | high | mitigate | `git diff --exit-code HEAD` clean over the five earlier phase directories; 3 of the 8 at-risk earlier patches spot-checked with `git apply --check` at current HEAD — all apply | closed |
| T-06-03-05 | Repudiation | high | mitigate | `REQUIREMENTS.md`: 18 ticked / 2 unticked (17+1 done, Phase 7's two outstanding); `REQ-cross-spine-partial \| Phase 6 \| Complete` | closed |
| T-06-03-06 | Tampering | high | mitigate | Same clean diff over the five earlier phase directories as T-06-03-04 | closed |
| T-06-03-07 | Tampering | medium | mitigate | `ROADMAP.md:233` (archived v0.12.x Phase 6) still reads `completed 2026-08-01`; the active-milestone Phase 6 rows read complete/2026-09-20. The documented revert-and-hand-fill recovery for the `phase.complete` archived-row rewrite is confirmed **in the file**, not merely claimed in a SUMMARY | closed |
| T-06-03-08 | Denial of Service | medium | mitigate | Harness ran to completion under explicit `-timeout 180m` (54/54, then 58/58); no `Taskfile.yaml` or CI timeout raised to accommodate it | closed |
| T-06-03-SC | Tampering | low | accept | No manifest change | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Unregistered Flags

**None.** No `## Threat Flags` section exists in any of the three SUMMARY.md files, and the auditor
observed no new attack surface while reading the diffs. `connecterror.go` — the single Connect error
mapper whose `*argError` arm must stay first — is untouched by this phase (empty `git diff`).

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| R-06-01 | T-06-01-07 | A `ListScopes` failure now costs a completed recall's full latency instead of failing fast. Accepted deliberately at decision time (D-01: succeed with hits + a coverage flag): the recall has *already* run when coverage is queried, so discarding it buys nothing and loses the caller's result. No new backend call was introduced. | user (discuss-phase, 2026-09-20) | 2026-09-20 |
| R-06-02 | (D-03, by design) | A caller that ignores `scopes_unknown` sees a normal successful response with no `searched_scopes`. Accepted: the three states are distinguishable on the wire (`nil` scopes + `unknown=true` ≠ known-empty ≠ truncated), and an empty list is **never** substituted for absence — the property is pinned by a red-evidence patch. Silent degradation for a non-reading client is the cost of not breaking the RPC. | user (discuss-phase, 2026-09-20) | 2026-09-20 |
| R-06-03 | all `T-06-NN-SC` | No supply-chain change this phase: zero `go.mod`/`go.sum` or package-manifest edits, no installs. | orchestrator | 2026-09-20 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-20 | 24 | 24 | 0 | gsd-security-auditor (verify-mitigations mode, ASVS L1 / L2 depth on authz placement, absence-vs-empty ordering, mutual exclusivity and wire leaks) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] No unregistered flags; `connecterror.go` confirmed untouched
- [x] Two mitigations verified by hand-executing the actual attack, with a byte-clean tree afterwards
