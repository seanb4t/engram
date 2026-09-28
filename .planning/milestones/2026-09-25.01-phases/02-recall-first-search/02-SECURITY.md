---
phase: "02"
slug: "recall-first-search"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-26"
---

# Phase 02 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| caller (browser session / MCP bearer / CLI) → internal/server | caller-chosen scope, k, limit, filters and include flags reach the shared recall core | verified identity, scope, k/limit, filters |
| internal/server → internal/store → Qdrant | the extra recall-gate comparison query runs under the caller's own Subject | verified identity, scope, per-state counts (uint64 only) |
| server response → CLI stdout | the CLI renders server-reported hidden counts | counts |
| localStorage → inline script / display store | a stored text-size value drives the `--ui-font` CSS variable | untrusted string |
| user input / URL → Connect read client | typed text and shareable URL params become SearchMemories/GetMemory/ListMemories/ListScopes requests | query text, ids, filters, k |
| server-provided memory fields → DOM | summaries, content, tags, scopes, error text render in rows, cards, pane and dropdown | caller-readable record data |
| keyboard / pane buttons → CSRF write client | row keys and pane buttons delete, share and make-private records | record id, visibility |
| npm registry → ui/node_modules → vendored SPA → engram binary | a new third-party package ships inside the committed console bundle | build artifacts |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-02-01 | Information Disclosure | searchRecallHidden / listRecallHidden | high | mitigate | Comparison calls plain `d.st.List`/`d.st.Search` with the caller's own `c.Subj` and resolved scope (hiddencount.go:131-150, 170-195), so ownerScopeFilter/ownerOrSharedCondition apply unchanged; only the three Include flags are relaxed; never SearchReranked; only counts leave. TestRecallHiddenListParity / TestRecallHiddenSearchParity seed another owner's private archived record and assert it is never counted (run live, PASS). Reviewed clean in 02-REVIEW.md focus area 1 | closed |
| T-02-02 | Tampering (wire contract) | engram.proto | medium | mitigate | Additive `RecallGateHidden` message (engram.proto:160) at next free field numbers (List 8, Search 5; engram.proto:150, 221); `buf breaking` against main in CI (ci.yaml:256-257) | closed |
| T-02-03 | Denial of Service | extra query per recall call | medium | mitigate | `rejectOverMaximumCount("limit"/"k")` runs before any work (tools.go:1856, 1896, 1947, 2119); `allIncluded()` skips the comparison (hiddencount.go:57, 137); plain Search, no Jev call; TestRecallHiddenSkipsComparisonWhenAllIncluded | closed |
| T-02-04 | Information Disclosure | comparison error text | low | mitigate | Failure logged once at ERROR server-side and degrades to nil/absent field (hiddencount.go:146, 190); TestRecallHiddenDegradesOnComparisonFailure asserts the sentinel is logged once and the field is absent on every lane | closed |
| T-02-05 | Elevation of Privilege | MCP recall gate | medium | mitigate | MCP `searchArgs`/`listArgs` (tools.go:950, 964) carry no `include_*` fields; MCP recall stays gated | closed |
| T-02-06 | Repudiation / integrity | renderRecallHiddenFooter | low | mitigate | Footer prints only server-reported numbers and nothing for nil/zero (client_common.go:379-386); TestClientSearchNoRecallHiddenFooterWhenAbsentOrZero | closed |
| T-02-07 | Information Disclosure | docs / tool descriptions | low | accept | See Accepted Risks Log AR-02-01 | closed |
| T-02-08 | Tampering | engram.console.textSize → --ui-font | low | mitigate | `parseInt` + `Number.isFinite` + round + clamp 12..16, else 15, before `setProperty` (app.html:25-30; display.svelte.ts); app.html.test.ts, display.test.ts | closed |
| T-02-09 | Denial of Service | text-size keyboard shortcuts | low | accept | See Accepted Risks Log AR-02-02 | closed |
| T-02-SC (02-03) | Tampering | shadcn-svelte popover install | low | mitigate | Popover vendored under ui/src/lib/components/ui/popover via pinned `shadcn-svelte` 1.7.0; `git diff` of ui/package.json across the phase shows the only dependency change is the 02-06 virtual-list add; bits-ui unchanged | closed |
| T-02-SC (02-06) | Tampering | `@humanspeak/svelte-virtual-list` install | high | mitigate | Exact pin `"0.5.14"` (package.json:39, no range), sha512 integrity in ui/pnpm-lock.yaml; single commit 93c284a2; research legitimacy audit approved; no fallback package installed | closed |
| T-02-10 | Tampering / XSS | HeaderSearch rendering | medium | mitigate | No `{@html}` in HeaderSearch.svelte (text interpolation only) | closed |
| T-02-11 | Information Disclosure | GetMemory by pasted id | medium | mitigate | Authz in internal/store (DEC-cgb); UI copy "not-found and not-yours look the same by design" (HeaderSearch.svelte:471) never asserts existence | closed |
| T-02-12 | Spoofing / CSRF | header search reads | low | accept | See Accepted Risks Log AR-02-03 | closed |
| T-02-13 | Denial of Service | keystroke-driven queries | low | mitigate | 70ms debounce (HeaderSearch.svelte:42-50); `{ signal }` on every queryFn (:107-146); k 50n, full false (:115-116) | closed |
| T-02-14 | Tampering / XSS | hand-off label rendering typed text | low | mitigate | Text interpolation only (no `{@html}` in the header-search components); id path URL-encoded via `encodeSearchParams` (search/params.ts) | closed |
| T-02-15 | Information Disclosure | copy-id commands | low | accept | See Accepted Risks Log AR-02-04 | closed |
| T-02-16 | Tampering / XSS | ResultRow summary / hover card | medium | mitigate | No `{@html}` in ResultRow.svelte | closed |
| T-02-17 | Elevation / accidental destruction | '#' row key | medium | mitigate | '#' only calls the host's `ondelete` (ResultsList.svelte:302-305), routed through WriteSurfaces → DeleteConfirmDialog (WriteSurfaces.svelte:277) | closed |
| T-02-18 | Information Disclosure | clipboard copy | low | accept | See Accepted Risks Log AR-02-05 | closed |
| T-02-19 | Tampering / XSS | DetailPane content | medium | mitigate | Only `{@html}` is `bodyHtml = renderMarkdown(memory.content)` (DetailPane.svelte:81, 213); renderMarkdown is DOMPurify-sanitized with a tight allowlist (markdown.ts:44-48) | closed |
| T-02-20 | Spoofing / CSRF | make-private, share, delete | medium | mitigate | Mutations use `engramWrite` (mutations/memory.ts:5, 332, 368) with DeleteConfirmDialog and ShareWarningInline in WriteSurfaces.svelte; owner-only writes enforced in internal/store | closed |
| T-02-21 | Elevation of Privilege | make-private on a non-owned record | low | mitigate | Server owner gate rejects; optimistic patch rolls back on error via `restoreMemoryQueries` (mutations/memory.ts:311-315, 341-383) | closed |
| T-02-22 | Tampering | URL parameters | medium | mitigate | `parseSearchParams` reads known keys only, k restricted to K_STEPS 50/100/250/1000, non-empty scope forces cross_spine false (search/params.ts:24, 46-66); server validates every field and enforces authz | closed |
| T-02-23 | Tampering / XSS | header query, chips, scope names | medium | mitigate | Text interpolation only; no `{@html}` in search route components | closed |
| T-02-24 | Denial of Service | full=true search payloads | low | mitigate | k bounded by K_STEPS / server max 1000; `response_too_large` rendered, not retried (errors/connect-error.ts:62; search/+page.svelte:101); signal cancels superseded requests | closed |
| T-02-25 | Spoofing / CSRF | row keys → writes | low | mitigate | /search writes only via WriteSurfaces (search/+page.svelte:46, 505) → engramWrite + CSRF, existing dialogs | closed |
| T-02-26 | Tampering / XSS | RecallState raw error text | medium | mitigate | No `{@html}` in RecallState.svelte | closed |
| T-02-27 | Information Disclosure | error details | low | accept | See Accepted Risks Log AR-02-06 | closed |
| T-02-28 | Denial of Service | k 1000 / infinite listing | low | mitigate | k limited to K_STEPS; listing pages `limit: 50n` (search/params.ts:167); signal cancels abandoned requests | closed |
| T-02-29 | Tampering | internal/webauth/static | medium | mitigate | CI `ui-drift` job rebuilds from `pnpm install --frozen-lockfile` and `git diff --exit-code internal/webauth/static/` (ci.yaml:302-325); Taskfile ui:build uses frozen lockfile | closed |
| T-02-30 | Spoofing / CSRF | /observe writes | low | mitigate | /observe routes writes through WriteSurfaces (observe/+page.svelte:18, 65) → engramWrite (double-submit CSRF) | closed |
| T-02-31 | Information Disclosure | skill documents | low | accept | See Accepted Risks Log AR-02-07 | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-02-01 | T-02-07 | Documentation and tool descriptions describe hidden counts only; no identifiers or record data are documented or exposed | plan 02-02 threat model (plan-time disposition) | 2026-09-26 |
| AR-02-02 | T-02-09 | ⌘+/⌘-/⌘0 are captured only inside the console tab; the worst case is a text size between 12 and 16 | plan 02-03 threat model (plan-time disposition) | 2026-09-26 |
| AR-02-03 | T-02-12 | Header-search reads go through the `engram` read client only; no write is reachable from the dropdown | plan 02-04 threat model (plan-time disposition) | 2026-09-26 |
| AR-02-04 | T-02-15 | Copy-id commands copy an id already present in the page URL or the local query cache; nothing new is fetched | plan 02-05 threat model (plan-time disposition) | 2026-09-26 |
| AR-02-05 | T-02-18 | Clipboard copy exposes only ids the user can already see on screen | plan 02-06 threat model (plan-time disposition) | 2026-09-26 |
| AR-02-06 | T-02-27 | The server already shapes errors for the caller (argError envelope; internal causes logged server-side); the UI shows exactly what the caller received | plan 02-09 threat model (plan-time disposition) | 2026-09-26 |
| AR-02-07 | T-02-31 | Skills describe code structure and public contracts only; no secrets, tokens or record data | plan 02-10 threat model (plan-time disposition) | 2026-09-26 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-26 | 33 | 33 | 0 | /gsd-secure-phase orchestrator (L1 grep-depth; auditor skipped per ASVS L1 short-circuit; TestRecallHidden* and TestCountRecallHidden run live, PASS) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-26
