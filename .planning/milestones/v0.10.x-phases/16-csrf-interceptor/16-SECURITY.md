---
phase: 16
slug: csrf-interceptor
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-07-12
---

# Phase 16 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> State B run: register authored at plan time (all 3 PLAN.md carried `<threat_model>`),
> ASVS L1 → L1 grep-depth verification, no auditor spawn (short-circuit rule).

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| cross-origin browser → top-level `http.Handler` | An attacker page issues a state-changing request using the victim's ambient cookies; rejected at the HTTP edge before Connect parses (`CrossOriginProtection`) | unsafe-method HTTP request |
| browser console (untrusted origin) → Connect write RPC | A cross-site page could attempt a state-changing write with the victim's ambient session cookie | write RPC invocation |
| double-submit cookie vs echoed header | Attacker can force the cookie to be sent (ambient) but cannot read it to echo the header (same-origin secrecy) | CSRF token |
| resolved Subject (upstream interceptor) → CSRF interceptor | The CSRF layer must not blindly trust that an upstream interceptor already guaranteed a non-empty Owner | caller identity |
| `ui.cookie_key` (operator secret) → `k_csrf` (derived) | The single session secret is stretched into a second, domain-separated HMAC key via HKDF | key material |
| token bytes → comparison | An attacker-supplied token is compared against the server-computed HMAC; non-constant-time compare leaks the valid token | secret token |
| `SetDenyHandler` / interceptor response → client | The rejection body must be a parseable Connect envelope without leaking internal error detail | error body |
| `Set-Cookie(engram_csrf)` → browser JS | Deliberately readable by same-origin JS (SC2) but `Secure` so it never crosses plaintext | CSRF token |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-16-01 | Spoofing / Tampering | write-RPC interceptor + `CrossOriginProtection` wrap | high | mitigate | Double-submit HMAC token required on all 6 write Procedures + whole-server same-origin wrap (`cmd/engram/csrf.go:26`, `serve.go:214`; `internal/server/connectcsrf.go`). `TestNoAnonymousWrite`, `TestCrossOriginProtectionRejectsCrossOrigin` | closed |
| T-16-02 | Information Disclosure | `CSRFSigner.Verify` | medium | mitigate | `hmac.Equal` constant-time compare (`internal/webauth/csrf.go:69`), never `==`. `TestCSRFSigner_TamperRejected` | closed |
| T-16-03 | Spoofing | token / Owner binding | medium | mitigate | HMAC bound to `Owner` only (D-08); a token minted for one Owner never validates for another. `TestConnectCSRFTokenMatrix` cross-owner cell | closed |
| T-16-04 | Tampering | Connect mux CORS posture | high | mitigate | No `AddTrustedOrigin`/permissive CORS in production; strict same-origin. `TestConnectNoCORSHeaders` permanent gate (SC4) stays green | closed |
| T-16-05 | Information Disclosure | rejection message / `SetDenyHandler` body | low | mitigate | Fixed generic message `"cross-origin request rejected"` (`cmd/engram/csrf.go:36`), never `err.Error()`. `TestCrossOriginDenyHandlerEnvelope` | closed |
| T-16-06 | Elevation of Privilege | interceptor fail-open on upstream reorder | high | mitigate | D-05 independent re-read of `Subject.Owner`, fail-closed even if the subject interceptor is later reordered/weakened. `TestConnectCSRFInterceptor_EmptyOwner` | closed |
| T-16-07 | Tampering | `DeriveCSRFKey` key material | medium | mitigate | RFC 5869 HKDF with distinct `engram-csrf-v1` info label → cryptographic domain separation from the AES-GCM session-seal key (`internal/webauth/csrf.go:16,31`). `TestDeriveCSRFKey_DeterministicAndDistinct` | closed |
| T-16-08 | Tampering | write-only allowlist drift | high | mitigate | `csrfWriteProcedures` keyed on generated `engramv1connect.*Procedure` constants, never hand-listed (`internal/server/connectcsrf.go:32-38`). `TestCSRFWriteProcedureAllowlist` (exact-6) + `TestReadRPCsCSRFExempt` guard both drift directions | closed |
| T-16-09 | Information Disclosure | `engram_csrf` cookie transport | medium | mitigate | Cookie is `Secure` + `SameSite=Lax` (`internal/webauth/handlers.go:91-92,105`); non-HttpOnly is a deliberate SC2 requirement (JS must echo it), with same-origin secrecy as the load-bearing property | closed |
| T-16-10 | Tampering | wire-name drift (issuance vs verification) | medium | mitigate | Dual exported `CSRFCookieName`/`CSRFHeaderName` consts in both `internal/server` and `internal/webauth` (import-cycle-safe) + `TestCSRFWireNamesMatch` equality assertion | closed |
| T-16-SC | Tampering | `go.mod` dependencies (supply chain) | high | accept | Zero new external packages — stdlib `crypto/hkdf`, `crypto/hmac`, `crypto/sha256`, `net/http` only. `go.mod`/`go.sum` unchanged vs phase base `7d17c534` (`git diff` empty) | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `high` count toward `threats_open`*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-16-01 | T-16-SC | No third-party supply-chain surface added: the phase uses stdlib `crypto/hkdf` (Go 1.24+), `crypto/hmac`, `crypto/sha256`, and `net/http.CrossOriginProtection` (Go 1.25+) exclusively. `go.mod`/`go.sum` are byte-identical to the phase base commit `7d17c534`. | Sean (via `/gsd-secure-phase 16`) | 2026-07-12 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-12 | 11 | 11 | 0 | gsd-secure-phase (L1 grep-depth; register authored at plan time, ASVS L1 short-circuit) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-12
