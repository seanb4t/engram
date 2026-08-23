---
phase: 09
slug: report-pending-in-migrate-status
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-22
---

# Phase 09 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| operator shell → local CLI process | An operator runs `engram migrate status` with their own configured credentials. Offline, operator-tier; no untrusted network input crosses in. | operator-supplied flags only |
| Qdrant collection payload → operator report doc | Payloads aggregated server-side by `Store.MigrateStatus`; only counts and version numbers cross. | aggregate counts, version numbers — no record content |
| report doc → operator terminal | Rendered through the shared `renderOperator` / `viewFields` path into a possibly-TTY writer. | `uint64` scalars, bucket labels |
| repo → `docs-site` build | `guides/migrate.md` is hand-authored markdown outside any generated or anchored region; no build-time codegen consumes the row. | static documentation text |
| published documentation → operator | The row tells an operator what a reported number means; a wrong row is an integrity defect in operator decision support. | documentation semantics |
| test process → repo filesystem | The new docs gate reads one file by a package-relative path and never writes. | read-only file access |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-09-01 | Information Disclosure | `migrateStatusReportDoc` (`cmd/engram/migrate_family.go`) | medium | mitigate | `Pending uint64` scalar only (migrate_family.go:316); `store.MigrateStatusResult` never embedded (verified: 0 embedded-struct occurrences). Enforced by construction via tier-wide `TestOperatorDocsAreHandDeclared` (`cmd/engram/operator_output_test.go`) — verified PASS. | closed |
| T-09-02 | Tampering | rendered text field table (`viewFields` / `sanitizeViewValue`) | low | accept | `pending` is `uint64`, never free-form string; `sanitizeViewValue` runs only on JSON string values (`cmd/engram/operator_view.go:206-222`). No control-character or terminal-escape path added. | closed |
| T-09-03 | Denial of Service | `statusReportDoc` / `statusSummary` | low | accept | Pure in-process functions over an already-fetched result; `Pending()` is one pass over an already-materialised bucket slice. No new I/O, no unbounded input. | closed |
| T-09-04 | Information Disclosure | `guides/migrate.md` `pending` row | low | accept | Publishes only the arithmetic of an already-published field (`MigrateStatusResponse` field 7, already in the same table). No new field, value, or surface disclosed. | closed |
| T-09-05 | Tampering | `migrateGuidePendingRowViolations` (the gate itself) | medium | mitigate | Three structural controls, all verified: inflection-free anchor; **zero-occurrence** assertion (`strings.Count(doc, anchor) != 0`, migrate_docs_test.go:60) rather than a conversion count; 7-case positive control including a discriminating `clean` case (`expectViolation: false`). `TestMigrateGuidePendingRowGateFiresOnInjectedViolation` verified PASS. | closed |
| T-09-06 | Repudiation | operator acting on a wrong `pending` reading | low | mitigate | Closed by the correction itself: `guides/migrate.md:279` now names both exclusions explicitly (buckets *at* `current_version`, and every `future` bucket) and names `MigrateStatusResult.Pending()` as the single shared definition across all three surfaces. | closed |
| T-09-SC | Tampering | npm/pip/cargo installs | low | accept | Zero package-manager installs in this phase — no `go.mod`, `go.sum`, `package.json`, or lockfile change. `09-RESEARCH.md` §Package Legitimacy Audit records "not applicable". | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

No threat in this register is rated high or critical; nothing here blocks the phase.

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| R-09-01 | T-09-02 | `uint64` scalar cannot carry control characters or terminal escapes; `sanitizeViewValue` covers only JSON strings. No new injection path. | seanb4t | 2026-08-22 |
| R-09-02 | T-09-03 | Pure in-process arithmetic over already-materialised data. No new I/O or unbounded input. | seanb4t | 2026-08-22 |
| R-09-03 | T-09-04 | Documents the arithmetic of an already-published field; discloses no new field, value, or surface. | seanb4t | 2026-08-22 |
| R-09-04 | T-09-SC | Zero package-manager installs in the phase — no supply-chain surface to audit. | seanb4t | 2026-08-22 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-22 | 7 | 7 | 0 | /gsd-secure-phase (orchestrator, L1 short-circuit) |

Verification depth: ASVS L1 grep-depth classification. Register was authored at plan time
(both `09-01-PLAN.md` and `09-02-PLAN.md` carry a parseable `<threat_model>`), so the
short-circuit rule applied and no auditor subagent was spawned. The three `mitigate`
dispositions were each verified against the implementation, and their pinning tests
(`TestOperatorDocsAreHandDeclared`, `TestMigrateGuidePendingRowGateFiresOnInjectedViolation`)
were observed PASS with a `[no tests to run]` canary check.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-22
