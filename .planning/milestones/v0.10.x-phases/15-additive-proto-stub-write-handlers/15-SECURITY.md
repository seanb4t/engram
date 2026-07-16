---
phase: 15
slug: additive-proto-stub-write-handlers
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-07-11
---

# Phase 15 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| client → Connect mux | Untrusted request payloads cross into the generated proto handlers; the negative matrix drives real untrusted request shapes through the full interceptor chain | Write-RPC request bodies (memory content, tags, field masks, visibility, schedule windows) |
| client → Connect interceptor chain | Untrusted payloads are authenticated (subject) then validated (protovalidate) before any handler runs | Bearer/OIDC identity + raw proto message payload |
| build/CI → runtime | The proto annotation set controls whether a mutating RPC is GET-reachable; the grep gate enforces the invariant before merge | `idempotency_level` method options in `proto/` |
| repo → BSR / Go module graph | External dependency resolution for protovalidate (buf.lock BSR pin, go.mod module promotion) | Third-party module identity + content digest |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-15-01 | Tampering/Spoofing | write RPC idempotency_level / GET-reachability | high | mitigate | No RPC carries an `idempotency_level` option (all default `IDEMPOTENCY_UNKNOWN`, so no write RPC is GET-reachable / CSRF-exposed). Defense-in-depth: grep-ban gate `Taskfile.yaml:141` (`task proto:lint`) + CI mirror `ci.yaml:124-127`; descriptor test `connectdescriptor_test.go:125` asserts `IDEMPOTENCY_UNKNOWN` on every method; negative matrix `connectapi_negative_test.go` asserts raw-GET → HTTP 405 for every write RPC. | closed |
| T-15-02 | Information Disclosure | interceptor ordering | medium | mitigate | Validate interceptor is placed AFTER the subject interceptor (D-10). Order at `connectapi.go:259-263` = otel → access-log → subject (401) → validate (400), so an unauthenticated caller gets `CodeUnauthenticated` and never learns field-level request-shape detail. Proven by the negative matrix's `unauth + invalid payload → CodeUnauthenticated` cell. | closed |
| T-15-03 | Denial of Service (partial) | malformed/oversized write payloads | medium | mitigate | `protovalidate.New()` (`connectapi.go:249`) is wired as the validate interceptor (`connectapi.go:263`); buf.validate annotations (min_len/max_bytes/enum/CEL, Plan 01) reject malformed/over-limit messages at the interceptor layer before any handler code runs. | closed |
| T-15-SC | Tampering | protovalidate BSR dep + Go module promotion; validation dependency choice | medium | mitigate | `buf.lock` (commit c09a054c) pins BSR commit `435963d1631043e694e56e6bcc3c79c3` + b5 digest. No new external Go module: `buf.build/gen/go/.../protovalidate` (option types) and `buf.build/go/protovalidate v1.2.0` (runtime) were both pre-resolved indirect and only reclassified to direct (`go.mod:6-7`). The self-described-unstable `connectrpc.com/validate` is deliberately NOT added (absent from go.mod/go.sum). RESEARCH.md package-legitimacy audit found no [SLOP]/[SUS]/[ASSUMED] packages (Buf first-party). | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|

No accepted risks.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-11 | 4 | 4 | 0 | /gsd-secure-phase (L1 grep-depth, short-circuit: register_authored_at_plan_time + asvs_level 1) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-11
