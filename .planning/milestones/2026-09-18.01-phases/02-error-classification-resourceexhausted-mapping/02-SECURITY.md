---
phase: "2"
slug: "error-classification-resourceexhausted-mapping"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-19"
---

# Phase 2 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Qdrant gRPC ↔ store client | The classifier interceptor sits innermost in the client chain, inside qdrant-go-client's own rate-limit interceptor | gRPC status code + message |
| Server ↔ Connect client | `connectError` maps the sentinel to `resource_exhausted` with a scrubbed envelope | Error code + envelope text |
| Server ↔ MCP client | The receiving middleware rewrites the tool result's text to the envelope | Tool-result text content |
| CLI ↔ scripts | Exit code `10` is a scripting contract (client and operator tiers) | Process exit status |
| Docs ↔ readers | errors.md / cli.md / upgrade.md publish the contract, never the byte ceiling | Published reference text |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-02-01-01 | Denial of Service | classifier relabeling a server-side `ResourceExhausted` | medium | mitigate | Code AND `grpc: ` prefix AND receive marker; `TestClassifyResponseTooLarge` pass-through rows + chain test PASS; red-evidence patches confirmed RED | closed |
| T-02-01-02 | Tampering | qdrant-go-client retry-after handling outside the classifier | medium | mitigate | Non-matches return the invoker's error unchanged (`TestClassifyResponseTooLarge`, `TestResponseTooLargeClassifierSitsInsideCallerChain` PASS) | closed |
| T-02-01-03 | Information Disclosure | `ResponseTooLargeError.Error()` text (byte counts, method, grpc text) | low | transfer | Server-log-only on the wire lanes: Connect arm emits `errors.New(responseTooLargeEnvelope())` (`connecterror.go:105`), MCP mapper replaces content; both lane tests assert no digit/`grpc`/`qdrant`/`Scroll`/`larger than max` | closed |
| T-02-01-04 | Repudiation | classifier absent from production dialing | medium | mitigate | Installed once in `NewQdrantClient` base options (`store.go:530`), the only construction site (Phase 1 D-11 gate PASS); real-overflow test dials through it | closed |
| T-02-01-SC | Tampering | package-manager installs | low | accept | No installs; `go.mod`/`go.sum` unchanged since phase start | closed |
| T-02-02-01 | Information Disclosure | Connect overflow message | medium | mitigate | `TestConnectListMemoriesResponseTooLarge` PASS (exclusion assertions) | closed |
| T-02-02-02 | Information Disclosure | MCP overflow tool-result text | medium | mitigate | `TestMCPListMemoryResponseTooLarge` PASS; unregistered-mapper red-evidence patch confirmed RED | closed |
| T-02-02-03 | Tampering | `connectError` switch reorder | medium | mitigate | `errors.As` arm first; `TestConnectError` distinctness rows PASS | closed |
| T-02-02-04 | Repudiation | raw error never logged or logged twice | low | mitigate | One `slog` ERROR in the mapper and one in the arm; lane tests count captured records | closed |
| T-02-02-05 | Information Disclosure | every other MCP tool error still returns raw text | low | accept | Out of scope by D-09; deferred item in 02-CONTEXT.md | closed |
| T-02-02-SC | Tampering | package-manager installs | low | accept | No installs | closed |
| T-02-03-01 | Repudiation | scripts mis-handling the 1 → 10 change | medium | mitigate | Upgrade guide §14; `TestUpgradeGuideNamesEveryChangedCommand` PASS; baseline rows | closed |
| T-02-03-02 | Tampering | exit-code reuse or renumbering | medium | mitigate | `TestExitCodeTooLargeDistinct`, `TestCatalogExitCodesMatchMapper`, `TestCatalogListsEveryExitCode` PASS | closed |
| T-02-03-03 | Repudiation | errors.md hint table drifting from `argerror.go` | medium | mitigate | `TestErrorsDocHintCodesMatchArgErrorConstants` + `TestParseHintCodeTable` PASS; doc-drift red-evidence patch confirmed RED | closed |
| T-02-03-04 | Information Disclosure | docs publishing the byte ceiling | low | mitigate | Envelope example carries no number; doc gate checks it verbatim | closed |
| T-02-03-SC | Tampering | package-manager installs | low | accept | `pnpm install --frozen-lockfile` replays the committed lockfile only | closed |
| T-02-04-01 | Repudiation | patch counted RED by breaking the build | medium | mitigate | Each patch hand-verified by its target's own `--- FAIL:` line (02-04-SUMMARY) | closed |
| T-02-04-02 | Tampering | harness leaving files mutated | medium | mitigate | Working tree clean after every harness run (executor, verifier, orchestrator runs) | closed |
| T-02-04-03 | Repudiation | a lane test losing its teeth later in the milestone | medium | mitigate | Each lane has a registered patch; `TestRedEvidencePatchesAreLive` (12 confirmed RED) runs in `task test` | closed |
| T-02-04-SC | Tampering | package-manager installs | low | accept | No installs | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-01 | T-02-01-SC, T-02-02-SC, T-02-03-SC, T-02-04-SC | No new packages or modules; docs-site install replays the committed lockfile | plan-time register | 2026-09-19 |
| AR-02 | T-02-02-05 | MCP returns raw text for every OTHER unclassified error, unchanged from before this phase; lane consistency deferred by user decision D-09 | user (discuss-phase D-09) | 2026-09-19 |
| AR-03 | T-02-01-03 (operator tier) | Operator commands (`engram migrate`, `reindex`, …) print the full `ResponseTooLargeError` text, including byte counts, to their own stderr. The operator already holds direct Qdrant access, so this discloses nothing beyond their trust boundary; the scrubbing requirement (REQ-exhausted-connect) covers the Connect/MCP wires | orchestrator (secure-phase) | 2026-09-19 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-19 | 20 | 20 | 0 | orchestrator (secure-phase, ASVS L1 grep-depth short-circuit — register authored at plan time; evidence from the live test run at HEAD) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-19
