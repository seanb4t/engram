---
phase: "03"
slug: "curation-rpcs-mcp-tools"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-28"
---

# Phase 03 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Browser session (cookie) → Connect write Procedures | A cross-origin page can send the session cookie; only the double-submit token proves intent | CSRF token, session cookie |
| MCP / Connect caller → `deps.archiveBatch` / `supersede` / `listScheduled` / `relatedMemories` / `listTags` | Caller-chosen ids, scopes, cursors, `k` and `limit` reach the resolver and core | Ids (UUID / short_id), scope strings, cursor tokens |
| `internal/server` → `internal/store` | The Subject crosses here; authz is decided only in the store (DEC-cgb) | Subject, resolved filter conditions |
| `internal/store` → Qdrant | Owner-only / owner-or-shared filter composed into Facet / Scroll / Search | Filter conditions, payload projections |
| Agent guidance → agent behaviour | Skill / CLAUDE.md text steers which mutations an agent attempts | Consent framing text |
| Vendored SPA → browser | The committed static bundle is what the server serves | Built JS/CSS bundle |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-03-01 | Tampering (forged CSRF) | `ArchiveMemory` / `RestoreMemory` | high | mitigate | `csrfWriteProcedures` allowlist (`connectcsrf.go:43-44`); `TestCSRFCurationWritesRequireDoubleSubmit` | closed |
| T-03-02 | Elevation of Privilege | `archiveBatch` → store | high | mitigate | `ArchiveAs` / `RestoreAs` owner gate (`store/spine.go:837,948`); `TestArchiveMemoryOwnerGate` | closed |
| T-03-03 | Information Disclosure | per-id `not_found` rows | high | mitigate | Single `outcomeNotFound` class, no resolved UUID (`archive.go:60`); `TestArchiveMemoryBatchOutcomes` | closed |
| T-03-04 | Denial of Service | batch size / entry length | medium | mitigate | ≤1000 ids, `maxArchiveIDBytes=256` before any store call (`archive.go:29,67`); `TestArchiveMemoryRejectsMalformedBatch` | closed |
| T-03-05 | Tampering (wire contract) | `engram.proto` | medium | mitigate | CI `buf breaking` against main | closed |
| T-03-06 | Repudiation | archive / restore | low | accept | `store.ArchiveAs` / `store.RestoreAs` spans record owner + outcome (`spine.go:838-848`) | closed |
| T-03-07 | Tampering (forged write, incl. dry run) | `SupersedeMemory` | high | mitigate | Allowlisted (`connectcsrf.go:45`); `connectcsrf_lane_test.go` `SupersedeMemory_validate_only` | closed |
| T-03-08 | Information Disclosure | `validate_only` preview | high | mitigate | Same stage functions as a real call (`supersedepreview.go`); `TestSupersedeMemoryConnectNamesEveryOffender` | closed |
| T-03-09 | Tampering (ledger poisoning) | idempotency ledger | medium | mitigate | `TestSupersedeValidateOnlyLeavesIdempotencyLedger` (Connect + MCP) | closed |
| T-03-10 | Elevation of Privilege | supersede targets | high | mitigate | `FetchForUpdate` preflight (`tools.go:2525`); per-target locks re-gate in `Store.Supersede` | closed |
| T-03-11 | Denial of Service | `supersedes` entries | low | mitigate | `maxSupersedeTargetBytes=256` (`tools.go:1122`) on both modes | closed |
| T-03-12 | Information Disclosure (deferred reveal) | `ListScheduled` all-scopes span | high | mitigate | Owner-only condition unconditional; `TestListScheduledCrossSpineDeferredReveal` | closed |
| T-03-13 | Tampering (cursor) | `page_token` / `cursor` | medium | mitigate | `decodeCursor` → `ErrInvalidArgument`; seen set > 1000 rejected (`store.go:2043-2049`); `TestDecodeCursorRejectsGarbage` | closed |
| T-03-14 | Denial of Service | `limit` / cross-spine span | low | mitigate | `rejectOverMaximumCount` before any store call (`tools.go:1947`) | closed |
| T-03-15 | Information Disclosure (coverage) | `searched_scopes` | low | accept | Reuses the existing readable-scopes enumeration; no new information class | closed |
| T-03-16 | Tampering (wire contract) | `engram.proto` | medium | mitigate | CI `buf breaking` | closed |
| T-03-17 | Information Disclosure | all-scopes rules read | medium | mitigate | `Store.List` with caller Subject (`rules.go:224,241`); `ownerOrSharedCondition` (`store.go:929`) | closed |
| T-03-18 | Information Disclosure (coverage) | `searched_scopes` on `ListRules` | low | mitigate | `TestListRulesCoverageThreeStates` (both lanes) | closed |
| T-03-19 | Denial of Service | all-scopes read size | low | mitigate | Capped at `store.MaxRecallLimit` (1000) (`rules.go:224,241`) | closed |
| T-03-20 | Tampering (wire contract) | `engram.proto` | medium | mitigate | CI `buf breaking` | closed |
| T-03-21 | Information Disclosure | `RelatedMemories` neighbourhood | high | mitigate | `recallVisibleFilter` (`relatedmemories.go:756`); `TestRelatedMemoriesNeverShowsPrivate` | closed |
| T-03-22 | Information Disclosure (anchor existence) | anchor resolution | medium | mitigate | `ErrNotFound` re-wrapped with caller input only (`related.go:49-50`) | closed |
| T-03-23 | Information Disclosure (full view) | `full=true` | medium | mitigate | `relatedShape` changes projection only, never the filter (`relatedmemories.go:565-569`) | closed |
| T-03-24 | Denial of Service | `k` | low | mitigate | `rejectOverMaximumCount` (`related.go:41`); `relatedTotalCeiling=64` | closed |
| T-03-25 | Tampering (wire contract) | `engram.proto` | medium | mitigate | CI `buf breaking`; oneof evidence | closed |
| T-03-26 | Information Disclosure (tag side channel) | `Store.ListTags` | high | mitigate | `recallVisibleFilter` → `ownerScopeFilter` (`listtags.go:41-42`); `TestListTagsNeverShowsPrivate` | closed |
| T-03-27 | Denial of Service | `limit` | low | mitigate | `rejectOverMaximumCount` (`tags.go:40`) | closed |
| T-03-28 | Tampering (wire contract) | `engram.proto` | low | mitigate | CI `buf breaking` | closed |
| T-03-29 | Elevation of Privilege (consent bypass) | curating-memory / curating-spine skills, CLAUDE.md | medium | mitigate | Explicit per-call consent language in both skills and CLAUDE.md | closed |
| T-03-30 | Tampering (stale vendored bundle) | `internal/webauth/static` | medium | mitigate | CI `ui-drift` job rebuilds and diffs | closed |
| T-03-31 | Information Disclosure (docs) | `tools.md` / `errors.md` | low | accept | Docs describe the indistinguishable-by-design rejection only; no internals | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-03-01 | T-03-06 | Existing `ArchiveAs` / `RestoreAs` spans record owner + outcome; no new audit surface needed | plan 03-01 threat model; verified by gsd-security-auditor | 2026-09-28 |
| AR-03-02 | T-03-15 | `searched_scopes` reuses the readable-scopes enumeration every cross-spine surface already publishes | plan 03-03 threat model; verified by gsd-security-auditor | 2026-09-28 |
| AR-03-03 | T-03-31 | Docs name the rejection contract without revealing internals | plan threat model; verified by gsd-security-auditor | 2026-09-28 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-28 | 31 | 31 | 0 | gsd-security-auditor (retroactive, post-archive, at ship) |

The mitigation tests were re-run against the current tree and pass. No `## Threat Flags` in any 03-*-SUMMARY.md.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-28
