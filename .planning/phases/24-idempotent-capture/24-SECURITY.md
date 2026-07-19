---
phase: 24
slug: idempotent-capture
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-07-18
---

# Phase 24 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| MCP client → store write path | `idempotency_key` crosses here on `store_memory`/`schedule_memory`. Owner is the server-verified `Subject` (never client-supplied); the point ID and content fingerprint are computed server-side from `(owner, scope, key)`. | idempotency key (client-supplied, untrusted); owner (server-resolved, trusted) |
| Concurrent retries → single Qdrant point | Multiple simultaneous identical keyed calls converge on one deterministic point ID; Qdrant's atomic per-point Upsert is the isolation primitive (no application-level lock). | deterministic point ID, record payload |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-24-01 | Information Disclosure / Tampering | cross-owner idempotency-key collision/leak (`idempotencyPointID` / `checkIdempotentReplay` Get) | high | mitigate | Owner is baked into the injective UUIDv5 hash input via length-prefixed `(owner, scope, key)` encoding (`internal/server/idempotency.go:39-42`) — cross-owner collision is structurally impossible between authenticated owners (D-09), not filter-enforced; a Get resolves only to the caller's own point. Tests: `TestIdempotencyPointIDOwnerScoped`, `TestIdempotencyPointIDBoundaryShiftInjective`, SC3 `TestStoreMemoryIdempotentKeyScopedPerOwner`. | closed |
| T-24-02 | Tampering / Repudiation | silent content overwrite on same-key/different-content (`IdempotencyFingerprint`, `ErrIdempotencyConflict`) | high | mitigate | On replay the stored fingerprint is compared and a mismatch returns the distinct `store.ErrIdempotencyConflict` sentinel BEFORE `Embed`/Upsert (`internal/server/tools.go:706-709`), never a silent overwrite. Sentinel is not `ErrNotFound` (`internal/store/store.go:90`) and maps to Connect `AlreadyExists` (`internal/server/connecterror.go:60`). Test: SC2 `TestStoreMemoryIdempotentReplayRejectsMismatch`. Honest boundary (D-12): under a truly simultaneous mismatch race the reject may not fire but converges to a single record — documented, not a defect. | closed |
| T-24-03 | Tampering (data integrity) | duplicate-record race on concurrent identical retries | medium | mitigate | Deterministic point ID feeds Qdrant's atomic per-point Upsert (no check-then-insert, no TOCTOU), resolving concurrent identical keyed calls to exactly one point. Test: SC4 `TestStoreMemoryIdempotentConcurrentIdenticalOnePoint` (`go test -race`). | closed |
| T-24-SC | Tampering | dependency installs (supply chain) | low | accept | No new packages introduced this phase (`google/uuid` v1.6.0 already vendored; `crypto/sha256`, `encoding/hex` stdlib) — package-legitimacy gate not applicable. | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| R-24-01 | T-24-SC | No new third-party dependencies added this phase; only stdlib + already-vendored `google/uuid`. Supply-chain surface unchanged. | Sean (phase owner) | 2026-07-18 |
| R-24-02 | T-24-02 | Truly-simultaneous same-key/different-content race may not fire the conflict reject on every arm, but always converges to one record (deterministic-ID Upsert). Accepted per D-12 as an honest concurrency boundary; strict serialization would require per-point locking/singleflight, out of scope. | Sean (phase owner) | 2026-07-18 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-18 | 4 | 4 | 0 | gsd-secure-phase (L1, register authored at plan time) |

Notes: ASVS L1 short-circuit (`threats_open: 0` + `register_authored_at_plan_time: true` + `asvs_level == 1`) — L1 grep-depth mitigation-existence verification. Mitigations independently corroborated this session by the Phase 24 deep code review (24-REVIEW.md / 24-REVIEW-FIX.md), which scrutinized cross-tenant point-ID poisoning (sound), the no-upsert-on-mismatch guarantee, and owner-in-hash isolation, and by the passing `-race` SC test suite.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-18
