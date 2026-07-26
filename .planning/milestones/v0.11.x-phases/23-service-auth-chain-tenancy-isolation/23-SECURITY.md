---
phase: 23
slug: service-auth-chain-tenancy-isolation
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-07-17
---

# Phase 23 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Verified by gsd-security-auditor (ASVS L1, block-on: high) — mitigations confirmed present in
> the implementation with file:line evidence and passing tests. **SECURED — threats_open: 0.**

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| client → MCP bearer gate | An untrusted bearer (client-credentials JWT or opaque static secret) crosses into the verifier chain; identity is derived only from the IdP-signed token or a constant-time static-token match, never client assertion | bearer token, resolved owner |
| verifier → store subject | The resolved owner string crosses into the authz key; an empty owner here would select the anonymous bucket (fail-closed on the service lane) | owner claim (namespaced) |
| verifier → logs/telemetry | The raw static secret must never cross into a log/error/span (DEC-wot) | static token secret |
| operator config → live request path | `ENGRAM_SERVICE_AUTH_*` config crosses into `withAuth`; malformed config fails fast at startup, and a mis-enabled lane must not alter the human-lane behavior | issuer/audience/owner-claims/static-token map |
| service principal → store recall filter | The resolved owner is the authz key the Phase-22 Cedar-backed Qdrant filter enforces — isolation for private records, global grant for `shared` | owner-scoped recall predicate |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-23-01 | Elevation of Privilege | service-lane `TokenVerifier` empty-owner path | high | mitigate | D-08 fail-closed reject `internal/auth/auth.go:254-261` (`failClosed && ownerVal==""` → `errors.Join(mcpauth.ErrInvalidToken,…)` → 401); survives chain composition — `TestServiceAuthChainParity_EmptyOwnerFailsClosedPostComposition` (`connectapi_service_auth_parity_test.go:160-169`) | closed |
| T-23-02 | Spoofing / EoP | auth-chain mechanism selection | high | mitigate | D-04 structural discriminator `chain.go:41-57` (`looksLikeJWT`/`discriminate`) routes by shape before verify, deny-by-default `chain.go:84-86`; a JWT can never be accepted as a static token or vice versa — `TestChainVerifier_Routes*`/`UnrecognizedShapeDeniesByDefault` | closed |
| T-23-04 | Information Disclosure | rejection-path logging/errors | high | mitigate | D-12/DEC-wot no-leak — constant literal error, no token interpolation `static_token.go:72-76`; `TestStaticTokenNoLeak` asserts no leak in error string or log buffer (both paths) | closed |
| T-23-11 | Information Disclosure | cross-owner private-record recall | high | mitigate | Phase-22 store filter (owner==X OR shared), zero new store code; `TestServicePrincipalIsolation` (`service_principal_isolation_test.go:20-149`, real Qdrant via testcontainers) incl. insertion-order independence | closed |
| T-23-05 | Information Disclosure | per-lane OIDC audience | medium | mitigate | D-14 `NewService` own audience param `auth.go:100-127`; separate `ENGRAM_OIDC_AUDIENCE`/`ENGRAM_SERVICE_AUTH_OIDC_AUDIENCE` keys `registry.go:48,59`; `TestNewServiceIndependentAudienceFromHumanLane` | closed |
| T-23-03 | Information Disclosure | static-token comparison | medium | mitigate | D-12 `subtle.ConstantTimeCompare` over the full value, no early return `static_token.go:67`; `TestStaticTokenPrefixNotMatched` | closed |
| T-23-07 | Elevation of Privilege | token→owner config shape | medium | mitigate | D-11 post-CR-01-fix `token → ownerID` per-owner map `static_token.go:37-39` (never a shared-default owner); `TestStaticTokenDistinctOwnersResolveDistinctly` | closed |
| T-23-09 | Elevation of Privilege | service owner-claims default | medium | mitigate | D-05 default `"client_id,azp"` (never `email`) `registry.go:60`, fail-fast `ParseOwnerClaims`; `TestServiceAuthRegistryDefaults` | closed |
| T-23-10 | Tampering | malformed static-tokens config | medium | mitigate | Fatal-when-malformed validation `validate.go:126-143` (ASVS V5); `TestServiceAuthValidate_StaticTokensMalformedIsFatal` | closed |
| T-23-12 | Elevation of Privilege | human-lane behavior drift | medium | mitigate | D-03 additive per-lane construction `serve.go:297-338`, human-only + no-issuer paths unchanged; `TestWithAuth_HumanOnlyConfig_RejectsUnauthenticated`, `TestWithAuth_NoLaneConfigured_ReturnsHandlerUnchanged` | closed |
| T-23-08 | Denial of Service | nil routed verifier | low | mitigate | D-03 nil-mechanism guards `chain.go:80-82,95,100` before dereference; `TestChainVerifier_Nil*Denies` | closed |
| T-23-06 | Information Disclosure | cross-tenant `shared` read | low | accept | D-15 explicit decision — global shared-read is intended v0.11.x behavior; ADR `docs/adr/engram-svct-service-tenant-global-shared-read.md` (Accepted, negative consequences stated), pinned by `TestSharedCrossTenantReadIntended` (`service_principal_isolation_test.go:162-194`), cross-referenced in docs-site auth reference + configure guide | closed (accepted) |
| T-23-SC | Tampering | dependency installs (×6, one per plan) | low | accept | Zero new external Go packages introduced by Phase 23 (`go.mod` diff attributable to the prior Phase-22 `cedar-go` addition); only new import is stdlib `crypto/subtle` — package-legitimacy gate not applicable | closed (accepted) |

*Status: open · closed · open — below high threshold (non-blocking)*
*Only open threats at or above `high` (block_on) count toward threats_open. All HIGH threats are closed.*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-23-01 | T-23-06 | Cross-tenant `shared`-read stays global for v0.11.x — `shared` (DEC-kyz) means "readable by any authenticated caller"; per-tenant `shared` scoping is genuinely-new authz surface deferred to the full-ABAC milestone (the Phase-22 schema reserves the `tenant` attribute). Explicit, written (ADR `engram-svct`), and tested (`TestSharedCrossTenantReadIntended`) — not a silent grant. Tenancy isolation is guaranteed for **private/owner-scoped** records. | Sean (D-15, roadmap SC5) | 2026-07-17 |
| AR-23-02 | T-23-SC | No new external dependency introduced this phase (stdlib `crypto/subtle` only); the package-legitimacy install gate is not applicable. | gsd-security-auditor | 2026-07-17 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-17 | 12 (+6 T-23-SC accept) | 12 | 0 | gsd-security-auditor (ASVS L1, block-on: high) |

**CR-01 residual-risk check (special focus):** the post-fix static-token orientation is `map // token -> ownerID` (`static_token.go:37-39`); the verify loop binds `ownerID` in the value position and sets `matchedOwner = ownerID` — never the map key/raw token — so `TokenInfo.UserID` and the `namespacedOwner("static_token", matchedOwner)` claim resolve to the owner, not the presented secret. `serve.go:326-333` passes the token-keyed parser output straight through, matching the corrected constructor contract. The WR-01 follow-on defect (zero `Expiration` hard-rejected by `mcpauth.RequireBearerToken`) is fixed via `staticTokenExpirationHorizon` (`static_token.go:22,80`) and covered end-to-end by `TestWithAuth_StaticTokenLane_AuthenticatesConfiguredToken`. No raw-token-into-owner leak path remains.

**Process note (non-blocking):** only `23-03-SUMMARY.md` carries an explicit `## Threat Flags` section (states "None"); the other five plan summaries omit it. Not a security gap — direct reading of all six SUMMARY files and the full diff found no attack surface outside the six PLAN threat registers.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-17
