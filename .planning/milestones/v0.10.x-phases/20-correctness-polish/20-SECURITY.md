---
phase: 20
slug: correctness-polish
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-07-16
register_authored_at_plan_time: true
---

# Phase 20 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Verified from PLAN.md `<threat_model>` blocks (State B). ASVS L1, block-on-high.
> `threats_open: 0` — short-circuit: no threat at or above the `high` block threshold; all six
> threats are low-severity and CLOSED (3 mitigations verified in-code, 3 accepted risks documented).
> Phase adds **no new attack surface** (RESEARCH §Security Domain).

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Connect client → `SearchDiscoveries` RPC | Authenticated caller reads discovery records; authz (per-actor + shared-read rules) is upstream of `memoryToProto` and unchanged | Discovery record fields (`kind`/`citations`) — already-authorized read |
| engram server → embedder HTTP endpoint | Outbound embeddings request; `model`/`input` set authoritatively, operator params merged but cannot override reserved keys | Embedding request body |
| operator config (`ENGRAM_EMBED_*_PARAMS`) → `config.ParseEmbedParams` | Operator-supplied JSON validated against the reserved-key list before use | Operator-trusted config |
| `store_memory`/`store_discovery`/`store_rule` → `MintShortID` → Qdrant `Count` | Server-internal short_id minting; exhaustion surfaces as a normal write error, not an unbounded hang | Server-internal |
| Kubernetes scheduler → summarize CronJob pod | Scheduled batch pod runs the same engram binary/image/env as the Deployment; no exposed network port (no Service/Ingress) | Scheduled batch execution |
| operator `values.yaml` → CronJob spec | Operator-trusted schedule/policy knobs; not end-user input | Operator-trusted config |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-20-01 | Information Disclosure | new `Memory.kind`/`citations` proto fields on `SearchDiscoveries` | low | accept | Not a new risk — `kind`/`citations` ride the same per-record authz-filtered `Memory` conversion every existing field uses; the `SearchDiscoveries` authz gate is entirely upstream of `memoryToProto` and untouched | closed |
| T-20-02 | Tampering | operator params overriding reserved `model`/`input` keys | low | mitigate | Single-source reserved-key list — `config.ReservedEmbedParamKeys` (`internal/config/embedparams.go:25`) with `embed.ReservedParamKeys` alias (`internal/embed/embed.go:144`); rejection strictness unchanged, desync eliminated | closed |
| T-20-03 | Denial of Service | unbounded `MintShortID` retry loop hanging a write request | low | mitigate | Bounded at `maxMintAttempts=16` real checks + absolute `maxMintSpins` cap (`internal/store/store.go:67/78`); returns wrapped `ErrShortIDExhausted` (store.go:1808/1839) so a stuck request fails fast | closed |
| T-20-04 | Elevation of Privilege (blast radius) | CronJob pod reuses full Deployment env (incl. secrets `summarize-missing` never reads) | low | accept | Accepted per D-09: full-env reuse via `engram.containerEnv` guarantees zero drift; mirrors the Deployment's existing secret-mount posture; no new secret introduced; no Service/Ingress (no new network surface). Tightening is a separate future hardening phase | closed |
| T-20-05 | Denial of Service | overlapping summarize sweeps | low | mitigate | `concurrencyPolicy: Forbid` (D-08) prevents overlap; `restartPolicy: OnFailure` + `failedJobsHistoryLimit: 1` bound retry/history growth (`summarize-cronjob.yaml`) | closed |
| T-20-SC | Tampering (supply chain) | dependency / subchart / plugin installs | low | accept | No new package installs this phase — `proto:gen`/`ui:build` use already-vendored toolchain; the `config→embed` edge is an in-repo import; chart validation is bash+grep (no `helm-unittest`); `batch/v1` CronJob is a core Kubernetes kind. Package Legitimacy Gate N/A | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `high` count toward `threats_open`*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-20-01 | T-20-01 | New `kind`/`citations` proto fields are read-only additions on an already-authz-gated RPC; no new read surface, just more fields on an already-authorized response | Phase 20 discuss (D-02, wire-only) | 2026-07-16 |
| AR-20-02 | T-20-04 | CronJob reuses the full Deployment env (incl. unused OIDC/UI secrets) to guarantee byte-identical zero-drift (D-09); no new secret, no exposed port; blast-radius tightening deferred to a future hardening phase | Phase 20 discuss (D-09) | 2026-07-16 |
| AR-20-03 | T-20-SC | Phase introduces no new external dependency, subchart, or plugin; Package Legitimacy Gate is not applicable | Phase 20 research (§Package Legitimacy Audit) | 2026-07-16 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-16 | 6 | 6 | 0 | /gsd-secure-phase (State B, ASVS L1 short-circuit — register authored at plan time, threats_open: 0) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-16
