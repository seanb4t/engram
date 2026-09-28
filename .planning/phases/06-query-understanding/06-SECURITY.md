---
phase: "06"
slug: "query-understanding"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-28"
---

# Phase 06 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Browser → server (Connect `UnderstandQuery`) | Authenticated console read RPC; caller identity from the Connect context, never the request | Query text, currently-applied filters (scope, categories, tags, window) |
| Server → decisions provider (Jev) | Outbound decision call, default-on when `ENGRAM_DECISIONS_PROVIDER=jev`; dedicated no-retry client with a bounded timeout | Query text only (State has exactly one key); never record content, scope names, or tag vocabulary |
| Server → store (`ListScopes` / `ListTags`) | Caller-scoped reads that build the scope options and local tag vocabulary | Caller's own readable scope names and tag counts |
| Server → logs / traces | Always-on bounded `engram.understand.*` span attributes; opt-in audit line | Outcome words and counts by default; query text + suggestion labels only under `ENGRAM_SEARCH_UNDERSTANDING_AUDIT` |
| Operator config (env / Helm) → server | `ENGRAM_SEARCH_UNDERSTANDING*` keys validated at startup | Enablement, timeout, audit flag |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-06-01 | Information Disclosure | understanding off path | high | mitigate | `understandDecider` returns nil when off; off short-circuit precedes validation and store calls (`internal/server/understand.go:77-79`); `TestUnderstandQueryOffNeverDecides` | closed |
| T-06-02 | Spoofing | Connect handler | high | mitigate | `callerFromConnectContext` first, `CodeUnauthenticated` on error (`connectapi.go:684-687`); tracer `unauthenticated` subtest | closed |
| T-06-03 | Information Disclosure | decision State | high | mitigate | State holds only `StateQuery` (`internal/understand/understand.go:215-218`) | closed |
| T-06-04 | Tampering | proto wire contract | medium | mitigate | Additive proto change; CI `buf lint` + `buf breaking` against main | closed |
| T-06-05 | Denial of Service | decision call | medium | mitigate | `jev.WithTimeout` + `WithNoRetry`; `Suggest` never returns an error | closed |
| T-06-06 | Tampering (CSRF) | read RPC | low | accept | Read-only RPC, excluded from `csrfWriteProcedures`; `TestReadRPCsCSRFExempt/UnderstandQuery` | closed |
| T-06-07 | Information Disclosure | scope options | high | mitigate | Caller-scoped `ListScopes`/`listTags` (`understand.go:94,103`); `TestUnderstandQueryScopeSuggestion` proves a foreign scope is never offered | closed |
| T-06-08 | Information Disclosure | tag vocabulary | high | mitigate | Tags matched locally, never sent; `TestUnderstandQueryAllKinds` asserts no vocabulary in questions and a single-key State | closed |
| T-06-09 | Tampering | malformed answers | medium | mitigate | `validateChoiceAnswer` / `FromResponse` drop any malformed answer entirely | closed |
| T-06-10 | Denial of Service | hung / failing provider | medium | mitigate | `TestUnderstandDeciderBoundedNoRetry` (hung and 503 both one request, bounded) | closed |
| T-06-11 | Information Disclosure | fallback logging | medium | mitigate | `logFallback` logs the class word only; fixed Warn lines without query/scope/tag/err | closed |
| T-06-12 | Tampering (search intent) | Suggested row | high | mitigate | Accept/dismiss on click only; `search.browser.test.ts` "suggested filters (NLQ-03)" proves URL and SearchMemories unchanged until accept | closed |
| T-06-13 | Spoofing (UI state) | chip styling | medium | mitigate | Unapplied chips never filled; computed-background test in `SuggestedRow.browser.test.ts` | closed |
| T-06-14 | Tampering (injection) | chip rendering | medium | mitigate | No `@html` in `SuggestedRow.svelte` / `understand.ts` | closed |
| T-06-15 | Spoofing (stale answer) | query cache | low | mitigate | Bare-`q` query key, no `placeholderData`, `AbortSignal` passed; stale-response discard test | closed |
| T-06-16 | Information Disclosure | dismissals | low | mitigate | Dismissals held in `$state` only, reset on `q` change, never URL or storage | closed |
| T-06-17 | Information Disclosure | default-on egress | high | mitigate | `logUnderstandingEnabled` startup Warn (default vs explicit); `TestBuildDepsFromEnvUnderstandingDefaultFollowsProvider` | closed |
| T-06-18 | Information Disclosure | disclosure line | medium | mitigate | Discloses host only (`url.Parse().Host`); `TestUnderstandingEnabledLogLine` | closed |
| T-06-19 | Tampering (misconfig) | config validation | medium | mitigate | Exact-literal enum, jev-without-provider rejected, timeout gated on effective enablement, audit boolean (`validate.go:396-453`); `TestUnderstandingConfigValidate` | closed |
| T-06-20 | Repudiation | audit flag | low | mitigate | Audit flag disclosed at startup and warned when it does nothing; `TestUnderstandingAuditEnabledLogLine` | closed |
| T-06-21 | Information Disclosure | query-text logging | high | mitigate | Audit line only behind the flag (`report.go:78-97`); non-ASCII sentinel sweep through the real Connect chain (`TestUnderstandQueryNoQueryTextWithoutAudit`, `TestUnderstandQuerySpanTelemetry`) | closed |
| T-06-22 | Information Disclosure | audit line shape | medium | mitigate | Exact key set (query, outcome, questions_asked, fallback_class, suggestions) — never content, owner, or credentials | closed |
| T-06-23 | Repudiation | audit off by default | low | accept | Tier-1 span attributes exist independent of the audit opt-in (`report.go:41-54`) | closed |
| T-06-24 | Information Disclosure | operator docs | high | mitigate | Configure guide documents default-follows-provider, egress, and the off switch; `TestUnderstandingVarsDocumented`; upgrade guide §22 | closed |
| T-06-25 | Tampering (config drift) | Helm chart | medium | mitigate | `with`-guarded env rows (`_helpers.tpl:105-112`); `task chart:validate` both directions + checksum | closed |
| T-06-26 | Tampering (stale bundle) | vendored SPA | low | mitigate | CI `ui-drift` job rebuilds and diffs `internal/webauth/static` | closed |
| T-06-27 | Information Disclosure | e2e fixture | low | accept | Loopback `httptest` server and synthetic query only | closed |
| T-06-SC (×6) | Tampering (supply chain) | dependencies | low | accept | No dependency added across 06-01..06-06 (`go.mod`, `go.sum`, `ui/package.json`, `ui/pnpm-lock.yaml` unchanged) | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-06-01 | T-06-06 | `UnderstandQuery` is a side-effect-free read; CSRF protection applies to writes only | plan 06-01 threat model | 2026-09-28 |
| AR-06-02 | T-06-23 | Audit capture stays opt-in by design (NLQ-04); always-on span attributes provide the non-repudiation floor | plan 06-05 threat model | 2026-09-28 |
| AR-06-03 | T-06-27 | e2e test traffic stays on loopback with a synthetic query | plan 06-06 threat model | 2026-09-28 |
| AR-06-04 | T-06-SC | No new dependencies were introduced by any plan | plans 06-01..06-06 threat models | 2026-09-28 |

The default-on egress of console query text (D-01) is a user decision (Sean, 2026-09-28), mitigated by the startup disclosure (T-06-17) and the explicit off switch rather than accepted silently.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-28 | 33 | 33 | 0 | gsd-security-auditor (live test execution) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-28
