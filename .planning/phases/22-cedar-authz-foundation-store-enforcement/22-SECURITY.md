---
phase: 22
slug: cedar-authz-foundation-store-enforcement
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-07-17
---

# Phase 22 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| caller identity (owner string) → PDP decision | Untrusted owner-claim value crosses into policy evaluation; an empty/unresolved owner must not escalate | owner-claim string (authz key) |
| embedded policy corpus → built binary | A policy text regression (typo, over-broad grant) would ship a wrong authz decision if not CI-gated | Cedar policy text |
| go module supply chain → build | A malicious/typosquatted cedar-go would compromise the entire authz layer | third-party dependency |
| Subject → read-filter builder | Converted owner/kind must fail closed for nil/unknown Subject; never a broader bucket set than today | caller identity |
| PDP bucket decision → Qdrant filter | Decision must compile with the authz clause as the outer Must; mis-composition could leak another owner's records | authz predicate |
| store ↔ handlers | PDP stays owned by the store; a handler consulting it would reintroduce a bypassable gate (DEC-cgb) | authz decisions |
| id-addressed request → record fetch → PDP decision | Unauthorized caller must be indistinguishable from a missing-id caller (DEC-xa6) | record ids / errors |
| PDP Diagnostic → caller / logs / spans | Policy ids and reasons are operator-audit data; never reach a caller, no PII in logs/spans (DEC-wot) | policy diagnostics |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-22-01 | Elevation of Privilege | defense_empty_owner.cedar | high | mitigate | Scoped forbid (`policies/defense_empty_owner.cedar:9-20`); `TestPolicyCorpus_EmptyOwnerDenyAll` + `TestPolicyCorpus_AnonOwnBucketReachable` pass live | closed |
| T-22-02 | Tampering / EoP | shared_read.cedar | high | mitigate | `action == Action::"read"` + `principal.owner != ""` (`policies/shared_read.cedar:6-14`); `TestPolicyCorpus_SharedReadOnly` asserts write/delete/share/schedule Deny (DEC-kyz) | closed |
| T-22-SC | Tampering | cedar-go dependency (go.mod/go.sum) | high | mitigate | Package Legitimacy Audit (RESEARCH.md, Go-module-proxy-verified); pinned exactly `v1.8.0` (`go.mod:10`), checksummed (`go.sum:57-58`) | closed |
| T-22-03 | Info Disclosure / DoS | MustDefault corpus load | medium | mitigate | Panic-on-parse-failure (`policies.go:61-67`), never a silent nil PolicySet; corpus test parses the same embedded bytes every CI run | closed |
| T-22-05 | Elevation of Privilege | ownerOrSharedCondition / ownerOnlyCondition | high | mitigate | `!ok → matchNothing()` before any PDP call (`store.go:584-620`); authz condition outer Must at every call site; `TestBulkFilterZeroBucketFailsClosed` / `TestBulkFilterOrderIndependent` pass live | closed |
| T-22-06 | Info Disclosure / DoS | bulk recall path | high | mitigate | `decideBucket` called exactly 2× per Search regardless of record count; `TestSearchAuthzCallCount` (12 records → 2 calls) passes live | closed |
| T-22-07 | Elevation of Privilege | principalParams converter | high | mitigate | `ok=false` for nil/unknown Subject (no PDP call); anonymous owner=="" never matches shared_read (`principal.owner != ""`); Subject sum stays 2-variant (DEC-12c) | closed |
| T-22-08 | Info Disclosure | id-addressed gate error mapping | high | mitigate | Deny → identical `fmt.Errorf("%w: %s", ErrNotFound, id)` as absent id (`store.go:1359-1429`); `TestGetReadableDenyMapsToNotFound` asserts exact string equality, no Diagnostic | closed |
| T-22-09 | Info Disclosure | Diagnostic exposure (DEC-wot) | medium | mitigate | `Decision.diag` unexported with zero accessors (`authz.go:48-51`) — structurally unreachable outside package authz; no consumer reads it (grep-verified) | closed |
| T-22-10 | Elevation of Privilege | id-addressed absent-record path | medium | mitigate | `s.Get` → ErrNotFound short-circuit precedes `decideRecord` in all 3 gates; `TestIdAddressedAbsentShortCircuit` proves absent id under all-deny PDP → ErrNotFound/nil | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|

No accepted risks.

---

## Observations (non-blocking, outside the register)

- `Store.DeleteAll` (`internal/store/store.go:1737-1770`) still uses a pre-existing hand-rolled
  `switch subj.(type)` ownership check and never consults the PDP. Confirmed pre-existing
  (untouched by all phase-22 commits) and behaviorally equivalent to what the policy corpus
  would decide for `delete` today — but a future Cedar policy change would silently not apply
  to bulk delete. Tracked as follow-up (see code review WR-01).
- T-22-09's optional debug-level Diagnostic logging enhancement was not built; the security
  invariant holds by construction instead (unexported field, no accessor).

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-17 | 10 | 10 | 0 | gsd-security-auditor (ASVS L1, verify-mitigations mode) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-17
