---
phase: "5"
slug: "operator-correctness"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-24"
---

# Phase 5 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Contributor shell env → CLI test process | Ambient `ENGRAM_*` values feed cobra flag defaults read by the exit-code baseline | Local config values (non-secret) |
| Record payload → operator text view | Nested JSON values rendered to a terminal | Operator-facing record fields |
| PLAN.md key_links → keylinks CI gate | YAML frontmatter parsed by the satisfiability gate | Planning metadata |
| Concurrent writer → migrate sweep | Below-cursor insert during a paged sweep | Record payloads (test-only coverage) |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-05-01 | Tampering | `TestExitCodeBaseline` | low | mitigate | Test-only change; `neutralizeEnvDerivedFlagDefaults` (`cmd/engram/golden_test.go`) blanks env-derived defaults with cleanup restore; no baseline row removed | closed |
| T-05-02 | Info disclosure | env-derived defaults in test output | low | accept | See Accepted Risks | closed |
| T-05-03 | Tampering | `viewFields` bare nested object | medium | mitigate | Path ends in `sanitizeViewValue`; hostile-leaf subtest asserts no control rune survives | closed |
| T-05-04 | Repudiation | operator view structure | low | mitigate | Empty nested object renders zero rows; `TestViewFieldsEmptyNestedObjectRendersNoRows` | closed |
| T-05-05 | Tampering | `ScanPlansWithStats` gate integrity | medium | mitigate | Scanner uses unfiltered `parsePlanKeyLinkItems`; only exported `ParsePlanKeyLinks` skips fieldless items; malformed-shape offenders still reported | closed |
| T-05-06 | DoS | `ScanStats` zero-applicability guard | low | mitigate | `stats.KeyLinks` counts every item; `TestZeroApplicabilityGuardFires` green | closed |
| T-05-07 | Tampering | migrate sweep convergence | low | mitigate | `TestMigrateBelowCursorInsertConverges` with non-vacuity checks; hand mutation observed red | closed |
| T-05-08 | DoS | shared `spineScrollBatch` var | low | mitigate | Saved/restored in `t.Cleanup`; no `t.Parallel` | closed |
| T-05-09 | Repudiation | `guides/cli.md` operator list | low | mitigate | Docs test derives required names from live `operatorCommands()` | closed |
| T-05-10 | Tampering | `cli.md` anchored rule regions | low | mitigate | Only the prose list changed; `internal/surfaces` green | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-05-01 | T-05-02 | Ambient `ENGRAM_*` flag defaults can surface in local test output only; the affected vars carry a collection name, an OIDC sub, runtime names, or header→env-var name pairs — never secret values; the fix reduces exposure | plan 05-01 threat model | 2026-09-24 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-24 | 10 | 10 | 0 | gsd-security-auditor (L1, source-traced); accepted risk logged by orchestrator |

Informational (non-blocking, pre-existing, no live input path): operator-view JSON object keys are not sanitized (all keys come from struct tags / proto field names today; no `map<>`/`Struct` fields); empty objects/arrays as array elements render one blank row per element (preserves element count).

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-24
