---
phase: 14
slug: embedder-model-options-eval
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-07-11
---

# Phase 14 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

Phase 14 (embedder-model-options-eval) is a docs + eval-correctness phase: a new
retrieval-eval differ fixture/test, a new `guides/embedding-models.md` operator
guide plus Helm `values.yaml` recipe comments, and a committed live-eval
evidence file. No production Go request path, proto, or wire surface changed;
no dependencies were added. The one recurring threat across all three plans is
**information disclosure** (a secret leaking into a public repo/docs artifact or
an eval probe carrying sensitive content) — mitigated at every site and verified
below at ASVS L1 grep-depth.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| eval fixture → third-party embedding API | The differ probe + gh261 fixture strings are sent over the network to a third-party embedder (Gemini/OpenRouter/OpenAI) during a live eval run. | Synthetic public-tooling text (no private/customer content). |
| docs / Helm values → operators & public repo | Recipe examples are committed to a public repo and rendered on a public docs site. | Configuration examples — API-key **placeholders** only, never real credentials. |
| operator keys / eval output → committed repo artifact | Eval runs use real Gemini/OpenRouter keys; their output is transcribed into a committed, version-controlled evidence file. | Redacted eval results — model-id/dim/exit-status/metric lines; no keys, no `Authorization` values. |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-14-01a | Information Disclosure | `differProbe` content in `internal/retrievaleval/fixtures.go` | low | mitigate | Probe is synthetic public-tooling text (`Run \`task lint\`…`), mirroring the gh261Distractors "no secrets" convention; no private/customer content is embedded, so nothing sensitive leaves for the provider. Verified: no secret patterns in `fixtures.go`. | closed |
| T-14-01b | Information Disclosure | `guides/embedding-models.md` env blocks + `charts/engram/values.yaml` recipe comments | medium | mitigate | All API keys are shell-safe quoted placeholders (`export ENGRAM_OPENAI_API_KEY='replace-with-your-key'`, 3 sites) — never a real key and never an unquoted angle-bracket token (review B6). Helm comments reference the existing `memory.openai.apiKeySecret` `secretKeyRef` indirection — never an inline secret. Verified: no secret patterns, no `<your-key>` anti-pattern. | closed |
| T-14-01c | Information Disclosure | `.planning/phases/14-…/14-EVAL-EVIDENCE.md` contents | medium | mitigate | Redaction takes precedence over completeness (review B8): keys shown as the `$ENGRAM_OPENAI_API_KEY` placeholder, (public) model-id + dim, exit status, and only the specific success/metric lines — no raw terminal paste, no `Authorization: Bearer` value, no env dump. Carries an explicit T-14-01 redaction note. Verified: no `Bearer`/key values present. | closed |
| T-14-SC | Tampering (supply-chain) | npm/pip/cargo installs | low | accept | N/A this phase — no package installs (RESEARCH § Package Legitimacy Audit: N/A). Verified: zero `go.mod`/`go.sum`/`package.json`/`Cargo`/lockfile changes in the phase diff (`6e3dc9d3..HEAD`). | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-14-SC | T-14-SC | No package installs this phase — docs/config/test-fixture/eval-evidence only. Zero dependency-manifest changes verified in the phase diff, so there is no supply-chain surface to gate. | Sean (operator) | 2026-07-11 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-11 | 4 | 4 | 0 | gsd-secure-phase (ASVS L1 grep-depth; register authored at plan time → short-circuit, no auditor spawn) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-11
