---
phase: "05"
slug: "related-memories-graph-tag-cloud"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-28"
---

# Phase 05 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| URL path segment `[id]` → `RelatedMemories` request | Attacker-controllable route param sent to the server | Anchor id string |
| Connect read lane → SVG/DOM rendering | Summaries, short ids, tags, citation refs and scores render into the page | Server response text |
| URL `from` / `trail` params → `goto` navigation | Query text read back and navigated to | Route string |
| Typed picker / header text → host filter | Free text becomes a `#tag` read filter, never sent to the server | Tag substring (client-only) |
| Selection → navigation hrefs | Re-centre / Open-record hrefs built from response ids | Id + query params |
| Pointer / wheel input → gesture handlers | Untrusted input drives zoom and drag math | Scale / translate numbers |
| localStorage → rail tab state | Client-editable stored value read back into UI state | `'graph' \| 'tags'` string |
| npm registry → `ui/node_modules` | New runtime packages d3-force/drag/zoom/selection + `@types` | Package code |
| `ui/` build → embedded binary | The vendored SPA tree is what users are served | Build artifacts |
| e2e fixture → live Qdrant | Test records written into a per-test server | Synthetic records |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-05-01 | Information Disclosure | `/related/[id]` not-found | medium | mitigate | `internal/server/related.go:49-50` maps unreadable and nonexistent anchors to the same `ErrNotFound`; `internal/store/relatedmemories_test.go:98-107`; route renders one `err-not-found` block echoing only `{id}` | closed |
| T-05-02 | Tampering (markup injection) | anchor card, node labels, call line | medium | mitigate | No `@html` in the related route or its components — text-node interpolation only | closed |
| T-05-03 | Denial of Service | client layout | low | accept | `RELATED_K=64`, `SETTLE_TICKS=300` (`ui/src/lib/related/graph.ts:35,38`); server ceiling `relatedTotalCeiling = 64` (`internal/store/relatedmemories.go:170`) | closed |
| T-05-SC | Tampering (supply chain) | npm installs | high | mitigate | Exact pins for d3-drag/force/selection/zoom@3.0.0 + `@types/*` (`ui/package.json`); committed `pnpm-lock.yaml`; `pnpm install --frozen-lockfile` in `task ui:build`; plans 05-02..05-09 add no package (accept) | closed |
| T-05-04 | Spoofing (open redirect) | `parseRelatedParams` `from` | medium | mitigate | `from` gated by `isAllowedDestination` (`ui/src/lib/search/related-params.ts:26-28`, `ui/src/lib/resume.ts:111,123-129`); `related-params.test.ts` | closed |
| T-05-05 | Tampering | `trail` entries | low | mitigate | Filtered on `UUID_RE`/`SHORT_ID_RE`, capped at `RELATED_TRAIL_MAX` (`related-params.ts:30`) | closed |
| T-05-06 | Tampering (markup injection) | TagBars / TagMatchRow highlight | medium | mitigate | `TagMatchRow.svelte:11` renders `{#each match.parts}` text nodes with `<b>` for the hit; no `@html` | closed |
| T-05-07 | Information Disclosure | tag counts | low | accept | `listTagsQuery` sends `{scope, limit}` only (`ui/src/lib/tags/query.ts:20-27`); server counts the caller's recall-visible set | closed |
| T-05-08 | Tampering | "Add #{tag}" free entry | low | accept | `toggleTag` only mutates the URL `tags` filter (`ui/src/routes/search/+page.svelte:99-101`); no write path | closed |
| T-05-09 | Tampering (markup injection) | EdgeLane / SupersessionLane / EvidenceSection | medium | mitigate | No `@html` in these components | closed |
| T-05-10 | Tampering (URL injection) | re-centre / Open-record hrefs | low | mitigate | `relatedPath` uses `encodeURIComponent(id)` (`related-params.ts:44-47`); `encodeSearchParams` (URLSearchParams) throughout | closed |
| T-05-11 | Denial of Service | wheel / drag handlers | low | accept | `SCALE_MIN=0.5`, `SCALE_MAX=4` (`ui/src/lib/related/zoom.ts:7-8`) applied via the d3-zoom extent | closed |
| T-05-12 | Tampering (markup injection) | focus card | low | mitigate | No `@html` in `RelatedGraph.svelte` | closed |
| T-05-13 | Tampering (markup injection) | tag rows / unknown-tag row | low | mitigate | `HeaderSearch.svelte` renders through the text-node `TagMatchRow` | closed |
| T-05-14 | Information Disclosure | typed tag text | low | accept | `queryFn` sends `{scope, limit}` only (`query.ts:22-24`); matching runs client-side | closed |
| T-05-15 | Tampering (URL injection) | `toggleTag` / picker `onadd` | low | mitigate | `navigate()` → `encodeSearchParams(...)` (`ui/src/routes/search/+page.svelte:85-86,99-101`) | closed |
| T-05-16 | Spoofing (open redirect) | `exitToOrigin` | medium | mitigate | Navigates only to the already-gated `params.from`, else `/search?sel=...` (`ui/src/routes/related/[id]/+page.svelte:191-198`) | closed |
| T-05-17 | Tampering | `engram.console.relatedRailTab` localStorage | low | mitigate | Read/write in try/catch; value validated ∈ `{'graph','tags'}` (`+page.svelte:71-89`) | closed |
| T-05-18 | Tampering (integrity) | `internal/webauth/static` | medium | mitigate | `task ui:build` = frozen-lockfile install + build + vendor; CI `ui-drift` job rebuilds and diffs | closed |
| T-05-19 | Information Disclosure | e2e fixtures | low | accept | Dedicated scope `repo:e2e-console-related`, synthetic content only (`internal/e2e/console_browser_test.go:1189`) | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-05-01 | T-05-03 | Layout cost bounded by matching client/server caps (64 nodes, 300 ticks) | plan 05-01 threat model; verified by gsd-security-auditor | 2026-09-28 |
| AR-05-02 | T-05-07 | Tag counts computed server-side over the caller's recall-visible set; client cannot widen | plan threat model; verified by gsd-security-auditor | 2026-09-28 |
| AR-05-03 | T-05-08 | Free tag entry only changes a read filter; no write path | plan threat model; verified by gsd-security-auditor | 2026-09-28 |
| AR-05-04 | T-05-11 | Zoom scale clamped to [0.5, 4] | plan threat model; verified by gsd-security-auditor | 2026-09-28 |
| AR-05-05 | T-05-14 | Typed tag substring never leaves the browser | plan threat model; verified by gsd-security-auditor | 2026-09-28 |
| AR-05-06 | T-05-19 | e2e fixtures are synthetic records in an isolated scope | plan threat model; verified by gsd-security-auditor | 2026-09-28 |
| AR-05-07 | T-05-SC (05-02..05-09) | Those plans install no package; lockfile unchanged beyond the 05-01 baseline | plan threat models; verified by gsd-security-auditor | 2026-09-28 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-28 | 25 | 25 | 0 | gsd-security-auditor (retroactive, post-archive, at ship) |

No `## Threat Flags` in any 05-*-SUMMARY.md. Open code-review items in 05-REVIEW-DISPOSITION.md (WR-01..04, IN-01..02) are UX/code-quality findings and bear on no threat mitigation.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-28
