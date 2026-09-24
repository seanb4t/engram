---
phase: "2"
slug: "decision-interface-jev-backend"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-23"
---

# Phase 2 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| engram → Decisions provider (OpenRouter / LiteLLM pass-through) | Outbound calls only when `ENGRAM_DECISIONS_PROVIDER=jev` | Decision state (memory text supplied by future callers) and API key (high) |
| Provider response → engram | Untrusted JSON bodies and error bodies | Bounded, status-classified, strictly decoded |
| Third-party SDK → build | Candidate dependency evaluated in an isolated nested module, then rejected | Supply chain (high) |
| Operator config / Helm → server | `ENGRAM_DECISIONS_*`, key via `secretKeyRef` | Credentials (high) |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-02-SC | Tampering (supply chain) | OpenRouter Go SDK | high | mitigate | Legitimacy checkpoint; isolated `sdk-eval` module; rejected — no SDK in `go.mod` | closed |
| T-02-01 | Repudiation | SDK facts | medium | mitigate | Facts quoted from `go list -m -json` in `02-SDK-EVALUATION.md` | closed |
| T-02-02 | Info disclosure | SDK fact gathering | low | accept | See Accepted Risks | closed |
| T-02-03 | Info disclosure | sdk-eval fixtures | medium | mitigate | Synthetic hosts (`gateway.example`) | closed |
| T-02-04 | Repudiation | D-06 verdict | medium | mitigate | Fixed R1–R4 rule table | closed |
| T-02-05 | Info disclosure (API key) | client/wiring/telemetry | high | mitigate | Key never logged (`jev.go`, `decider.go`); `TestJevTelemetryCarriesNoContent` | closed |
| T-02-06 | Spoofing/Tampering | base URL | high | mitigate | No base-URL fallback; http(s)+host required when provider=jev (`validate.go`) | closed |
| T-02-07 | DoS | response read | medium | mitigate | `context.WithTimeout`, `io.LimitReader`, `httpdrain.Drain` on every exit | closed |
| T-02-08 | Info disclosure | key fallback | medium | mitigate | `cmp.Or` fallback with logged source; documented | closed |
| T-02-09 | DoS (cascade) | error handling | medium | mitigate | Named sentinels; `decideOneSafe` panic recovery | closed |
| T-02-10 | DoS/cost | DecideMany fan-out | medium | mitigate | Bounded concurrency; peak-in-flight tests | closed |
| T-02-11 | Tampering | malformed input | low | mitigate | Validate before I/O; `MaxChoices=255` | closed |
| T-02-12 | Elevation/integrity | Decider surface | low | mitigate | Interface exposes only `Decide`/`DecideMany` | closed |
| T-02-13 | Info disclosure (egress while off) | wiring/chart | high | mitigate | `provider=""` constructs nothing; chart block gated; `TestDeciderFromConfigProviderUnset` | closed |
| T-02-14 | Info disclosure (record content) | docs | medium | mitigate | Data-disclosure section; `TestDecisionsVarsDocumented` | closed |
| T-02-15 | Info disclosure | docs hostnames | low | mitigate | Example hosts only | closed |
| T-02-16 | DoS (body reads) | client | high | mitigate | 4096-byte error bound; bounded success read; `TestJevRetryAndBounds` | closed |
| T-02-17 | Info disclosure | telemetry | medium | mitigate | `TestJevTelemetryCarriesNoContent` | closed |
| T-02-18 | Info disclosure | provider error detail | low | accept | See Accepted Risks | closed |
| T-02-19 | Tampering | classification | low | mitigate | Status-only classification; `code` never read (`classify.go`) | closed |
| T-02-20 | Tampering/DoS | response decode | medium | mitigate | Strict per-question decode incl. type match (WR-01, `e3a60dbd`) | closed |
| T-02-21 | Info disclosure | live test payload | low | mitigate | Synthetic text, gated off by default | closed |
| T-02-22 | Repudiation | live gate | low | mitigate | Unset → skip; invalid value → fail loudly | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-02-01 | T-02-02 | Public repository metadata only; no credentials sent beyond the operator's own `gh` session | plan 02-01 threat model | 2026-09-23 |
| AR-02-02 | T-02-18 | Provider error detail bounded at 4096 bytes and returned to the same caller that supplied the content (same posture as `internal/embed` T-04-05); classification never depends on it | plan 02-07 threat model | 2026-09-23 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-23 | 23 | 23 | 0 | gsd-security-auditor (L1 + test/lint/chart runs); accepted risks logged by orchestrator |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-23
