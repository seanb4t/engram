---
phase: "3"
slug: "shared-bounded-read-mechanism-content-cap-decision"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-19"
---

# Phase 3 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Caller ↔ memory write paths | Three always-enforced caps reject oversized content/tags on MCP, Connect and CLI before embed and store | Memory content, tags |
| Operator config ↔ runtime | One shared parser now backs both `Config.Validate()` and runtime enforcement (WR-01 fix) | Cap values |
| Store ↔ Qdrant | Per-view record ceilings size every RPC; batch-of-1 fallback; named error instead of a skip | Payload bytes |
| Caller filter ↔ keyset resume | `excludeSeen` nests the caller's filter under `Must` and only adds `MustNot` | Authz filter |
| Recall gate | New emitter classified `otherNonRecallEmitters` until Phase 4 wires it | AST classification |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-03-01-01 | Denial of Service | unbounded content/tags | high | mitigate | `checkContentBytes`/`checkTags` in `validateStoreArgs` before embed+store; `TestMemoryWriteCapsRejectOnEveryCreateLane` PASS | closed |
| T-03-01-02 | Information Disclosure | rejection text echoing input | low | mitigate | byte counts/indices only; `TestHintNeverEchoesValue/tag_value_no_echo` PASS | closed |
| T-03-01-03 | Tampering | cap configured 0/negative | medium | mitigate | startup validation names the variable; `TestMemoryCapsRejectZeroAndNonPositive` PASS; overflow closed by WR-01 fix (`TestOverflowValueRejectedByValidate` PASS) | closed |
| T-03-01-04 | Elevation of Privilege | a create lane reaching the store uncapped | medium | mitigate | one enforcement point shared by MCP + Connect; both lanes tested | closed |
| T-03-01-SC | Tampering | package-manager installs | low | accept | none; `go.mod`/`go.sum` unchanged | closed |
| T-03-02-01 | Denial of Service | sweep aborting on an oversized response | high | mitigate | byte-derived per-RPC count per view; `TestScrollAllPointsByteBudget` PASS; red-evidence patch confirmed RED | closed |
| T-03-02-02 | Repudiation | sweep skipping an unreadable record | high | mitigate | batch-of-1 then `ErrResponseTooLarge`, never a skip; `TestScrollAllPointsBatchOfOneFallback`, `…SingleOversizedRecordFailsNamed` PASS | closed |
| T-03-02-03 | Denial of Service | ceiling undercount | medium | mitigate | ceiling includes citations + tags; `TestRecordCeilingDerivesFromCaps`, `TestRecordCeilingHoldsForMaxCapRecord` (empirical `proto.Size`) PASS; uncapped fields carry the D-11 allowance (#589) | closed |
| T-03-02-04 | Tampering | existing sweep behavior changing early | medium | mitigate | every current caller passes `unbudgetedView`; spine/revert/purge tests green | closed |
| T-03-02-SC | Tampering | package-manager installs | low | accept | none | closed |
| T-03-03-01 | Elevation of Privilege | Connect `UpdateMemory` bypassing the cap | high | mitigate | checks inside `deps.updateMemory`; `TestUpdateMemoryContentCap` PASS; red-evidence patch confirmed RED | closed |
| T-03-03-02 | Denial of Service | legacy over-cap record becoming uneditable | medium | mitigate | gated on `contentChanged` / changed tag set; `TestUpdateMemoryLegacyOversizedRecord` (8 subtests) PASS | closed |
| T-03-03-03 | Information Disclosure | size rejection revealing another owner's record | low | mitigate | checks run after `FetchForUpdate`'s owner gate (uniform not-found, DEC-xa6) | closed |
| T-03-03-04 | Tampering | read ceilings drifting from write caps | medium | mitigate | one parse path (`recordCapsFromConfig`); `TestStoreFromConfigCarriesRecordCaps`, `TestDefaultRecordCapsMatchRegistryDefaults` PASS | closed |
| T-03-03-SC | Tampering | package-manager installs | low | accept | none | closed |
| T-03-04-01 | Denial of Service | ordered page of a few huge records overflowing | high | mitigate | per-RPC count clamped to remaining page budget; `TestScrollOrderedPageByteBudget` PASS (pre-clamp RED measured 5.2 MB) | closed |
| T-03-04-02 | Elevation of Privilege | tie-exclusion relaxing the caller's authz filter | high | mitigate | `excludeSeen` nests the caller filter under `Must`, adds only `MustNot`; `TestScrollOrderedPageTiesAcrossRPCBoundaries` PASS | closed |
| T-03-04-03 | Repudiation | budget-cut page read as the last page, or a skipped record | high | mitigate | explicit `CutByBudget`/`Exhausted`/`Next` (structurally exclusive); red-evidence patches confirmed RED | closed |
| T-03-04-04 | Tampering | unclassified transmitting helper bypassing the recall gate | medium | mitigate | classified `otherNonRecallEmitters` in the same change; `TestRecallEmissionSetIsCompleteAndClassified` PASS | closed |
| T-03-04-SC | Tampering | package-manager installs | low | accept | none | closed |
| T-03-05-01 | Elevation of Privilege | a client lane reaching the store uncapped | medium | mitigate | `TestCLIStoreRejectsOversizedContentAndTags` drives the real binary against a real server; CLI holds no second copy of the cap | closed |
| T-03-05-02 | Spoofing | the e2e static token | low | accept | test-only token on loopback, per-test server and collection; never shipped | closed |
| T-03-05-03 | Tampering | operator misconfiguring a cap from docs | low | mitigate | configure.md states 0 is rejected and why; startup validation is the backstop | closed |
| T-03-05-SC | Tampering | package-manager installs | low | accept | `pnpm install --frozen-lockfile` replays the committed lockfile | closed |
| T-03-06-01 | Tampering | a patch left applied after a failed run | medium | mitigate | harness reverts via `defer`/`t.Cleanup` and asserts a clean tree; verified clean after every run | closed |
| T-03-06-02 | Repudiation | a stale patch no longer proving its gate RED | medium | mitigate | each patch hand-verified against its target's `--- FAIL:`; all 25 re-proved on every full run | closed |
| T-03-06-03 | Denial of Service | harness outgrowing the default test timeout | low | mitigate | measured 111s under the default; per-package narrowing contingency did not fire; timeouts never raised | closed |
| T-03-06-SC | Tampering | package-manager installs | low | accept | none | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-01 | T-03-0N-SC (all) | No new packages or modules; docs-site install replays the committed lockfile | plan-time register | 2026-09-19 |
| AR-02 | T-03-05-02 | e2e static token is test-only, loopback, per-test server/collection; never a shipped credential | plan-time register | 2026-09-19 |
| AR-03 | T-03-02-03 (residual) | Fields still without a write cap (scope/repo/workspace/worktree_path/base_dir/source, citation ref/locator/pin, supersedes count, discovery/rule tags) carry a documented 16 KiB allowance rather than a proven bound; an over-allowance record takes the batch-of-1 fallback and then the named error, never a silent skip. Capping them is tracked as GitHub #589 | user (discuss-phase D-11) | 2026-09-19 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-19 | 26 | 26 | 0 | orchestrator (secure-phase, ASVS L1 grep-depth short-circuit — register authored at plan time; evidence from the live validation run at HEAD, 23/23 named tests PASS, 25 confirmed RED) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-19
