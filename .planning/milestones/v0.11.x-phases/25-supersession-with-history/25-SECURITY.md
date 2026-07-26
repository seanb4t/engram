---
phase: 25
slug: supersession-with-history
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-07-19
---

# Phase 25 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> **Verified SECURED** — 12/12 threats closed, all 4 high-severity mitigations confirmed
> present in code via live tests against real Qdrant (testcontainers).

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| MCP caller → `Store.Supersede` (target) | Caller-supplied target id crosses into an owner-gated write mutation of an EXISTING record (not the caller's fresh create) | Target record id (UUID/short_id), correcting content |
| `Store.Supersede` → Cedar PDP (`getWritable`) | Authorization decision on the target record's owner/visibility | Owner-claim value, action verb (`ActionWrite`) |
| recall gate → Qdrant filter | Which records the `superseded_by IS EMPTY` soft-hide filter admits into `search_memory`/`list_memory`/`search_discovery`/`list_scheduled` | Recall result set |
| MCP client → `supersede_memory` handler | Untrusted tool args (target id, correcting content) cross into resolve + owner-gated back-stamp | Tool arguments |
| `supersede_memory` handler → `Store.Supersede` | Handler delegates the target write gate entirely to `Store.Supersede` — no parallel authz | Resolved target id, new record |
| `connectError` → Connect client | Business-error sentinel becomes a Connect status code (info-disclosure surface) | Error code + message |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-25-01 | Elevation of Privilege | `Store.Supersede` target gate | high | mitigate | `getWritable(target, subj, authz.ActionWrite)` before any mutation (store.go:1862); `own_records.cedar` grants write only to owner. `TestSupersedeOwnerGate` PASS (live). | closed |
| T-25-02 | Tampering (cycle/integrity) | single-hop / cycle | high | mitigate | Reject already-superseded targets with `ErrAlreadySuperseded` (store.go:1868); a fresh record can't already be superseded → no cycle. `TestSupersedeAlreadySuperseded` PASS. **Hardened (CR-01):** `TargetLocker` (locker.go) held across the check→back-stamp closes the concurrent-writer race; `TestSupersedeConcurrent` PASS. | closed |
| T-25-03 | Information Disclosure | target-existence leak | medium | mitigate | `getWritable` returns identical `ErrNotFound` for not-owner and not-found (store.go:1490); propagated unchanged. | closed |
| T-25-04 | Elevation of Privilege | cross-tenant supersede | high | mitigate | `getWritable → decideRecord` is the unchanged Phase 22/23 Cedar seam (store.go:725); namespaced owner ≠ principal → cross-tenant denied. No Phase-25 change to `internal/authz/`. | closed |
| T-25-05 | DoS / integrity (TOCTOU) | delete-during-supersede | low | accept | Back-stamp `SetPayload` propagates `NotFound` unchanged on mid-op delete (store.go:1879, fail-closed like `SetVisibility`). `TestSupersedeTOCTOU` PASS. Bounded, reversible orphan documented (store.go:1817). **Hardened (CR-04):** `Store.Update` now takes the same target lock + re-reads `superseded_by`, so a racing Update can no longer erase the back-stamp. | closed |
| T-25-06 | Repudiation / censorship | supersede as hide-vector | low | accept | Owner-only, additive (never deletes); superseded record stays fetchable via ungated `Store.Get` (store.go:1370). An owner can only soft-hide their OWN records; nothing is lost. | closed |
| T-25-07 | Elevation of Privilege | `supersede_memory` handler | high | mitigate | Handler never calls `GetReadable`; delegates target access solely to `FetchForUpdate`/`Store.Supersede` (getWritable/ActionWrite) (tools.go:1284). `TestSupersedeMemory` cross-owner PASS + no-UUID-leak assertion. | closed |
| T-25-08 | Information Disclosure | target-not-found error | medium | mitigate | Re-wraps `ErrNotFound` with caller's ORIGINAL input `a.Supersedes`, never the resolved UUID (tools.go:1305/1330). 404-indistinguishable. Pinned by the cross-owner UUID-leak check. | closed |
| T-25-09 | Information Disclosure | `connectError` generic fallthrough | low | mitigate | `ErrAlreadySuperseded → CodeFailedPrecondition` (connecterror.go:70); default arm returns a generic `"internal error"` message. `TestConnectError/already_superseded` PASS. | closed |
| T-25-10 | Tampering (retry integrity) | two-step create+back-stamp | low | accept | `supersede_memory` is not idempotency-keyed this phase; a step-4-failed retry could create a duplicate correcting record — bounded and reversible by delete; no SC requires it. **Hardened (WR-03/WR-04):** `idempotency_key` now schema-excluded (`json:"-"`, tools.go:517) AND defensively cleared (tools.go:1292) so the field can't silently ride the wire. `TestSupersedeMemorySchemaExcludesIdempotencyKey` + `TestSupersedeMemoryIgnoresIdempotencyKey` PASS. | closed |
| T-25-SC | Tampering (supply chain) | dependencies | n/a | accept | Zero new production deps (locker.go uses only `sync`/`context` stdlib; WR-03's `jsonschema-go` was an indirect→direct promotion only, no new package). No install task, no legitimacy checkpoint. | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Only open threats at or above `workflow.security_block_on` (high) count toward `threats_open`.*

**Non-register mitigation confirmed:** CR-03 cost-amplification hardening (post-dates register) — `FetchForUpdate` ownership gate (tools.go:1303) runs BEFORE `d.em.Embed` (tools.go:1322) and `MintShortID` (tools.go:1326); `TestSupersedeMemoryEmbedNotCalledForNonOwner` confirms zero embed calls on a non-owner attempt. A non-owner can no longer force billable embed calls.

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-25-01 | T-25-05 | Orphan forward-link on back-stamp failure is bounded and reversible by delete; TOCTOU delete is fail-closed. CR-04 further closes the Update-erases-back-stamp race. | Phase plan (D-05) + review disposition | 2026-07-19 |
| AR-25-02 | T-25-06 | Owner-only, additive, fetchable-by-id — supersede cannot destroy or hide another actor's data. | Phase plan (D-06) | 2026-07-19 |
| AR-25-03 | T-25-10 | Idempotency deliberately deferred for supersede (T-25-10); made honest via WR-03/WR-04 (schema-excluded + cleared + tested-ignored). Duplicate-on-retry is bounded and reversible by delete. | Phase plan + review disposition | 2026-07-19 |
| AR-25-04 | T-25-SC | Zero new production dependencies this phase. | RESEARCH Package Legitimacy Audit (N/A) | 2026-07-19 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-19 | 12 | 12 | 0 | gsd-security-auditor (ASVS L1, live tests vs real Qdrant) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-19
