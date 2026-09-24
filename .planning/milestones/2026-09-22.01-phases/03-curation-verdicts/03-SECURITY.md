---
phase: "3"
slug: "curation-verdicts"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-24"
---

# Phase 3 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| consolidate → Decisions provider | Per-pair record state sent only when a provider is configured and `--no-verdicts` is absent; disclosed on stderr first | Memory summary+content, bounded per side (high) |
| Provider response → consolidate | Untrusted verdict answers mapped via `verdict.FromResult` | Choices/probabilities |
| Record content → operator terminal | Text view renders nested verdict objects | Untrusted strings (terminal control) |
| curationeval → provider / repo | Gated eval over synthetic corpus; optional private local pair file | Synthetic text; private pairs never committed or echoed |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-03-01 | Info disclosure | verdict pass egress | high | mitigate | Gated on provider + `--no-verdicts`; disclosure line before any send; WR-01 wording fix (`0d15d0b5`) | closed |
| T-03-02 | Tampering/Elevation | consolidate store surface | high | mitigate | Read-only interface; `TestConsolidateStoreSurfaceIsReadOnly`, `TestRecordStatesDoesNotMutate` | closed |
| T-03-03 | Tampering (terminal injection) | operator view | medium | mitigate | All renderer/flatten output through `sanitizeViewValue` | closed |
| T-03-04 | Info disclosure | endpoint host in disclosure | medium | mitigate | `url.Parse(...).Host` only | closed |
| T-03-05 | DoS | uncapped pairs | low | accept | See Accepted Risks | closed |
| T-03-06 | Info disclosure | eval corpus / local file | high | mitigate | Denylist test; errors name line numbers only; gitignored local dir; report is aggregate-only | closed |
| T-03-07 | Tampering (evidence) | blind labels | medium | mitigate | Fresh tool-less labeler; provenance with prompt SHA | closed |
| T-03-08 | Repudiation | eval results | low | mitigate | Provenance section in `03-EVAL-RESULTS.md` | closed |
| T-03-09 | Integrity (#508) | scope guard | medium | mitigate | `requireSweepScope` first in RunE; enforcing-leaves table | closed |
| T-03-10 | DoS | state fetch | medium | mitigate | `verdictStateRecordCeiling`; fetch error degrades to `state_unavailable` | closed |
| T-03-11 | Tampering | provider answers | medium | mitigate | `FromResult` rejects malformed answers | closed |
| T-03-12 | Tampering | threshold parse | medium | mitigate | `ParseProbability` rejects NaN/Inf/out-of-range | closed |
| T-03-13 | DoS | state chars knob | low | accept | See Accepted Risks | closed |
| T-03-14 | Tampering (docs drift) | rules comment | low | mitigate | Comment names four enforcers | closed |
| T-03-15 | Repudiation | corpus provenance | low | mitigate | Labels header + pairs.go comment | closed |
| T-03-16 | Tampering | threshold flag | medium | mitigate | Same parser as `Config.Validate` | closed |
| T-03-17 | Repudiation | failure policy | low | mitigate | Per-pair error class, stderr summary, all-auth warning | closed |
| T-03-18 | Tampering (docs drift) | CLI guide | low | mitigate | `TestConsolidateGuideStatesVerdictContract` + red control | closed |
| T-03-19 | Tampering (measurement) | D-03 gate | medium | mitigate | Integer arithmetic; `VACUOUS` on n=0 | closed |
| T-03-20 | Info disclosure | live eval | low | accept | See Accepted Risks | closed |
| T-03-21 | Info disclosure | eval artifact | medium | mitigate | Aggregate lines + provenance only | closed |
| T-03-22 | Tampering (evidence) | eval run | medium | mitigate | First complete run recorded; no tuning | closed |
| T-03-SC | Tampering (deps) | go.mod | low | accept | See Accepted Risks | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-03-01 | T-03-05 | Uncapped pair count is bounded in practice by `ENGRAM_DECISIONS_CONCURRENCY` and `--timeout`; ~$0.00005 per pair (user declined a cap, D-11) | plan threat model / user D-11 | 2026-09-24 |
| AR-03-02 | T-03-13 | Oversized `ENGRAM_DECISIONS_VERDICT_STATE_CHARS` only yields per-pair `context_too_large`; sweep exits 0 (D-10) | plan threat model | 2026-09-24 |
| AR-03-03 | T-03-20 | Live eval corpus is synthetic and public (D-01); operator opts in with own credentials | plan threat model | 2026-09-24 |
| AR-03-04 | T-03-SC | No new module dependency introduced (no go.mod changes in phase range) | plan threat model | 2026-09-24 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-24 | 23 | 23 | 0 | gsd-security-auditor (L1); accepted risks logged by orchestrator |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-24
