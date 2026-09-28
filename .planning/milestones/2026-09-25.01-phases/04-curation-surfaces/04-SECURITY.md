---
phase: "04"
slug: "curation-surfaces"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-28"
---

# Phase 04 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Browser → Connect write lane | `ArchiveMemory` / `RestoreMemory` / `SupersedeMemory` / `DeleteMemory` mutate state on the caller's behalf | Id sets, draft content, idempotency keys |
| Browser → Connect read lane | `RelatedMemories` / `GetMemory` / `ListRules` / `ListScheduled` answers render into the UI | Record content, ids, owner, scope |
| sessionStorage → route hosts / landing | The re-auth resume envelope repopulates a write dialog or picks a post-OIDC destination | returnPath, draft fields, target ids, idempotency key |
| Pointer / keyboard input → write intent | Row anchoring, bulk selection and ⌘K dispatch turn UI state into an id set sent to a write RPC | Selected / anchored record ids |
| npm registry / GitHub skill repos → `ui/` devDependencies & agent skills | Third-party test code and design-review skill instructions load into the session | Package code, skill instructions |
| `ui/` source → vendored static bundle → engram binary | The committed SPA is what operators run | Built JS/CSS artifact |
| Repo → public GitHub issues | Audit findings published outside the repo | Component / criterion text only |
| e2e harness → live server | The test mints a session + CSRF token for a fixture identity | Test-only cookie key, stub OIDC doc |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-04-01 | Spoofing (CSRF) | `curation.ts` archive / restore | high | mitigate | `engramWrite` client (`curation.ts:76,90`); both RPCs in `csrfWriteProcedures` (`connectcsrf.go`) | closed |
| T-04-02 | Information Disclosure | ArchiveConfirmDialog | medium | mitigate | Renders `r.requested` only, never `r.id`, on `not_found` (`ArchiveConfirmDialog.svelte:167`) | closed |
| T-04-03 | Repudiation | archive entry points | medium | mitigate | Undo restores exactly the `resultChangedIds` shown (`ArchiveConfirmDialog.svelte:78-82,116-118`) | closed |
| T-04-04 | Tampering (open redirect) | `resume.ts` | high | mitigate | Four-entry `ALLOWED_DESTINATIONS` (`resume.ts:111,123-129`); `resume.test.ts:222-232` | closed |
| T-04-05 | Tampering (forged envelope) | `resume.ts` `isValidShape` | medium | mitigate | Per-kind structure + version + TTL checks (`resume.ts:172-202`) | closed |
| T-04-06 | Information Disclosure | deleted `/observe` route | low | accept | No `ui/src/routes/observe/+page.svelte`; SPA not-found only | closed |
| T-04-07 | Tampering (supply chain) | third-party design skills | high | mitigate | Three candidates reviewed via fable-security-review; only the unqualified pass installed (04-03-SUMMARY) | closed |
| T-04-08 | Tampering (supply chain) | axe-core devDependency | medium | mitigate | Exact pin `4.13.0` (`ui/package.json`); no bundled import from `.svelte` sources | closed |
| T-04-09 | Information Disclosure | ChainDialog placeholders | low | accept | Owner-scoped read predicate enforced in `internal/store` | closed |
| T-04-10 | Tampering (markup injection) | ChainDialog rendering | low | mitigate | No `@html` in `ChainDialog.svelte` | closed |
| T-04-11 | Tampering (wrong target) | ResultsList selection | medium | mitigate | `selectedIds` cleared on `selectionKey` change (`ResultsList.svelte:448-455`) | closed |
| T-04-12 | Denial of Service | bulk archive batch | low | accept | Server 1000-id cap (`archive.go:79` `rejectOverMaximumCount`) | closed |
| T-04-13 | Spoofing (CSRF) | supersede preview / commit | high | mitigate | `SupersedeDialog` imports no client (callback props only); `SupersedeMemory` in `csrfWriteProcedures` | closed |
| T-04-14 | Tampering (stale commit) | SupersedeDialog gate | medium | mitigate | Revision-guarded `validate_only` preview (`SupersedeDialog.svelte:267-306`) | closed |
| T-04-15 | Repudiation (duplicate write) | commit after re-auth | medium | mitigate | `idempotencyKey` persisted in the envelope and reused (`resume.ts:57,189`, `curation.ts:135`) | closed |
| T-04-16 | Information Disclosure | draft in sessionStorage | low | accept | sessionStorage only, `RESUME_TTL_MS` 10 min, consumed once (`resume.ts:29`) | closed |
| T-04-17 | Tampering (wrong row) | RowActions anchoring | low | mitigate | Id captured at render (`RowActions.svelte:14-19`); toolbar hidden on scroll / text-size (`ResultsList.svelte:372-387`) | closed |
| T-04-18 | Repudiation (auto-resend) | `/search` resume reopen | high | mitigate | Reopen sets a notice, never auto-submits; `onresumeapplied` fires once (`CurationSurfaces.svelte:408-429`) | closed |
| T-04-19 | Elevation of Privilege (stale host) | curation host registry | low | mitigate | Unregister clears only the current registration (`host.svelte.ts:23-27`) | closed |
| T-04-20 | Tampering (rule mutated) | `/rules` affordances | medium | mitigate | Only `ondelete` passed; no edit / visibility / archive / supersede (`rules/+page.svelte:231,248`) | closed |
| T-04-21 | Repudiation (auto delete) | delete resume | high | mitigate | `requestDelete` only opens the dialog; no mutate on resume (`rules/+page.svelte:130-134,187-196`) | closed |
| T-04-22 | Spoofing (CSRF) | rule delete | medium | mitigate | `useDeleteMemory` uses `engramWrite.deleteMemory` (`memory.ts:381`); `DeleteMemory` allowlisted | closed |
| T-04-23 | Information Disclosure | cross-spine `ListScheduled` | low | transfer | Server-side `ownerOnlyCondition` (`store.go:2019`) | closed |
| T-04-24 | Tampering (early archive) | expired-only gate | low | accept | Client UX rule; server archive is owner-only and reversible | closed |
| T-04-25 | Repudiation (auto archive) | `/scheduled` resume | medium | mitigate | `reopenFromResume` is notice-only, no call until the operator clicks (`scheduled/+page.svelte:142-144`) | closed |
| T-04-26 | Information Disclosure | filed GitHub issues | medium | mitigate | Issues cite component, criterion and `04-A11Y-AUDIT.md` only — no record content, ids, owners or tokens | closed |
| T-04-27 | Tampering (untrusted skill) | installed third-party skills | low | mitigate | Only the fable-security-review-passed skill installed; its output treated as claims to verify | closed |
| T-04-28 | Tampering (stale bundle) | `internal/webauth/static` | medium | mitigate | CI `ui-drift` job rebuilds via frozen lockfile and diffs | closed |
| T-04-29 | Spoofing | e2e session / CSRF minting | low | accept | `crypto/rand` per-run cookie key + stub OIDC provider (`console_browser_test.go:208-220`) | closed |
| T-04-30 | Tampering (stale target) | SupersedeDialog add-target lookup | medium | mitigate | Code review WR-01: the add-target-by-id lookup (`SupersedeDialog.svelte:205-228`) lacks the revision guard the preview has, so a slow stale lookup can place the wrong predecessor in the (guarded) preview. Tracked in GitHub #642 | open — below high threshold (non-blocking) |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-04-01 | T-04-06 | Deleted route has no server handler; SPA not-found renders | plan 04-01 threat model; verified by gsd-security-auditor | 2026-09-28 |
| AR-04-02 | T-04-09 | Read predicate lives server-side; client renders only what it receives | plan threat model; verified by gsd-security-auditor | 2026-09-28 |
| AR-04-03 | T-04-12 | Server enforces the 1000-id cap; client omission affects UX only | plan threat model; verified by gsd-security-auditor | 2026-09-28 |
| AR-04-04 | T-04-16 | Tab-scoped, 10-minute TTL, single-use; content is the caller's own draft | plan threat model; verified by gsd-security-auditor | 2026-09-28 |
| AR-04-05 | T-04-24 | Client-only UX guard; archive is owner-only and reversible server-side | plan 04-10 threat model; verified by gsd-security-auditor | 2026-09-28 |
| AR-04-06 | T-04-29 | Test-only harness; never ships, no real IdP or credential | plan 04-12 threat model; verified by gsd-security-auditor | 2026-09-28 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-28 | 30 | 29 | 1 (below threshold) | gsd-security-auditor (retroactive, post-archive, at ship) |

T-04-30 was registered from code-review finding WR-01 (04-REVIEW-DISPOSITION.md), not from a SUMMARY threat flag. SUMMARY `## Threat Flags` (04-05..04-08, 04-10..04-12) all report none beyond the plan threat models.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-28
