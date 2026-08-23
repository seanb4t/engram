---
phase: 08
slug: registry-docs-tail
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-21
---

# Phase 08 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

Register origin: **authored at plan time**. Four of six plans (`08-01` … `08-04`)
carry a parseable `<threat_model>` block; `08-05` and `08-06` declare no threat
model applies, each with a stated change-surface justification and a
`git diff --name-only` gate pinning that surface. Retroactive-STRIDE mode was
therefore not required.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| operator argv → sweep `RunE` | Operator-supplied `--scope` / `--all-scopes` values cross into a command that can sweep every scope in the collection | scope selector (untrusted operator input); blast radius = every record in scope |
| rule registry → operator terminal | The declared `Sentence` becomes text rendered into an operator's terminal on rejection and in `--help` | normative constraint text (trusted, but load-bearing for operator decisions) |
| rule registry → generated prose | The same `Sentence` is written verbatim into a committed documentation file by the generator | generated documentation region (integrity-critical) |
| documentation → operator action | A reader acts on these pages when deciding whether a record is recoverable, whether a window has lapsed, and whether a version stamp is safe to ignore | behavioral claims that gate destructive operator decisions |
| store implementation → documented claim | Every behavioral sentence added asserts something about code the page does not compile against | unverifiable-by-build assertions |
| documentation → operator action on a live collection | A reader follows the migrate guide while holding credentials that can rewrite every record's payload | full-collection payload write authority |
| CLAUDE.md → every agent acting in this repo | Loaded ahead of the task and treated as normative; a wrong statement propagates into agent behavior with no further review step | routing/normative instructions consumed without review |
| shipped command surface → documented inventory | The Layout row asserts what commands exist and which tier they act on, and nothing recompiles it | command inventory + client/operator tier classification |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-08-01 | Elevation of Privilege | the three sweep `RunE` guards | medium | mitigate | Emptiness semantics (not cobra `Changed`) pinned at all three leaves by `TestSweepLeavesRejectPresentButEmptyScope`; `TestRequireSweepScope` positive rows prove the guard is not rejecting unconditionally. Both PASS. | closed |
| T-08-02 | Spoofing | `--all-scopes` `Usage` on `spine-review consolidate` / `purge` | high | mitigate | `SurfaceFields` narrowing + the NEGATIVE half of `TestSweepLeavesUsageStatesRegisteredRule` (`sweep_scope_test.go:228-229`), which errors if the registered `Sentence` appears on a non-enforcing leaf. Sentence verified absent from both commands; test PASSES. | closed |
| T-08-03 | Denial of Service | `sweepScopeRule()`'s panic-on-unregistered-rule | low | accept | Unreachable in a built binary — rule const and registry literal are in the same package; `ValidateRules` covers the registry and `TestValidateRules` runs in CI (PASSES). Mirrors the shipped `requirePurgeFilterScope` precedent. | closed |
| T-08-04a | Repudiation | the anchored region in `reference/tools.md` | low | mitigate | `TestSurfaceConformanceProseFiles` compares the region body for exact equality with the registered `Sentence`; CI's `surfaces` job re-runs the generator and diffs the tree. Test PASSES. | closed |
| T-08-04b | Repudiation | the validity-window boundary prose, on both pages | medium | mitigate | Bounds derived from `activeWindowConditions`: inclusive lower / exclusive upper, half-open `[not_before, not_after)` documented at `memory-record.md:100-109`; the previously-shipped `tools.md` `expired` bullet corrected at `tools.md:238` to "`not_after` is at or before now" — consistent with the exclusive upper bound. | closed |
| T-08-05 | Repudiation | the forward-version / rollback-safety claim | high | mitigate | All three contract parts plus the boundary present at `memory-record.md:133-155`, including the lower-level by-id replace path that **can** lower a stored version from a stale caller — stated explicitly as "a real, narrower boundary on the guarantee, not an edge case to wave away". Cross-linked to `guides/upgrade.md`'s rollback hazard with a must-not-disagree clause. | closed |
| T-08-06 | Information Disclosure | `schema_version` and usage-signal field documentation | low | mitigate | `schema_version` "never appears in a recall or authorization filter" (`memory-record.md:131`); `access_count` "Never read by the reranker or any recall filter" (`memory-record.md:38`). The `schema_version` claim is additionally an enforced property with a negative test in `internal/store`. | closed |
| T-08-07 | Tampering | the migrate guide's `--apply` examples | high | mitigate | Preview-and-apply contract established at `guides/migrate.md:39-42`, above the first `--apply` occurrence (line 42, inside the contract sentence itself). No copyable `--apply` invocation precedes it. | closed |
| T-08-08 | Repudiation | the revert refusal contract | high | mitigate | Both refusal shapes documented — preflight refusal (`migrate.md:196`) and race-discovered mid-loop refusal (`migrate.md:202-208`, explicitly stating earlier records may already have been reverted) — plus the snapshot recovery path (`migrate.md:188`) and the `reverted`/`applied`/`reversible` JSON semantics that carry the same warning (`migrate.md:291-294`). | closed |
| T-08-09 | Repudiation | the convergence and re-run claims | medium | mitigate | "Strictly shrinking" and "resumable" both absent as claims; the page states the negation directly — "**It is not resumable.** There is no persisted cursor… no checkpoint file and no `--resume` flag" (`migrate.md:79-81`). | closed |
| T-08-10 | Information Disclosure | the documented json output | low | accept | The three report structs are hand-declared scalar-and-count shapes that deliberately exclude record ids; documenting their keys discloses nothing a caller running the command does not already receive. Exclusion enforced by the struct types, not by the guide. | closed |
| T-08-11 | Repudiation | the corrected release note | low | mitigate | The hazard paragraph survives the correction — "**The rollback hazard.**" heading present at `guides/upgrade.md:326` with its additive-only sentence at `:330` and the deliberate-rollback qualifier at `:335`. Corrected by updating the remedy, not by deleting the warning. | closed |
| T-08-12 | Tampering | anchored region inside `reference/tools.md` | low | mitigate | Same control as T-08-04a: `TestSurfaceConformanceProseFiles` PASSES, and no diff line falls inside an anchor pair. | closed |
| T-08-13 | Spoofing | the revised migrations convention | medium | mitigate | Registry boundary stated explicitly (`CLAUDE.md:77-82`): `migrate-remap-owner`, `summarize-missing`, and `reindex` key off an IdP claim change, async summary fill, and embedder config identity respectively — "none of which is version-driven, so none is in the registry or the status histogram". | closed |
| T-08-14 | Repudiation | the `cmd/engram/` inventory row | medium | mitigate | Derived cross-check: every command leaf token in `cmd/engram/testdata/catalog.golden` (23 command paths) appears in the row at `CLAUDE.md:15`. Zero missing. Gate proven non-vacuous by a constructed-defect self-test. | closed |
| T-08-15 | Spoofing | the client-tier / operator-tier distinction in the row | medium | mitigate | The row separates "client-tier commands reaching a running server over Connect (`get`, `search`, `list`, `store`, `migration-status`)" from "operator-tier commands acting on Qdrant directly", filing `migration-status` and `migrate status` on opposite sides. | closed |
| T-08-16 | Repudiation | the automation half of the migrations bullet | medium | mitigate | Both halves stated (`CLAUDE.md:73-77`): no migration applies automatically, AND "What IS automatic: server startup runs a read-only `MigrateStatus` probe that may log a pending-migrations warning… it never invokes the sweep and never gates startup". | closed |
| T-08-17 | Information Disclosure | the Memory contract additions | low | accept | Everything added is already public in `docs-site` and in the tool schemas; CLAUDE.md compresses it for routing. No credential, endpoint, or internal-only detail introduced. | closed |
| T-08-SC | Tampering | npm/pip/cargo installs | low | accept | Zero dependency-manifest changes across the entire phase: `git diff --name-only 7db3ed49~1 HEAD -- go.mod go.sum docs-site/package.json docs-site/pnpm-lock.yaml` returns 0 files. The `go.mod`/`go.sum` delta against `main` originates in `4b67b658 feat(05-04)`, a prior phase. | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

> **ID note.** `T-08-04` was assigned independently in `08-01` and `08-02` to two
> different components. Both are carried here, disambiguated as `T-08-04a`
> (08-01, the anchored region) and `T-08-04b` (08-02, the validity-window prose).
> Neither was dropped by the collision.

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| R-08-01 | T-08-03 | Panic-on-unregistered-rule is unreachable in a built binary — the const and the registry literal are in the same package and `TestValidateRules` gates the registry in CI. Mirrors a shipped precedent. | seanb4t | 2026-08-21 |
| R-08-02 | T-08-10 | Report structs are scalar-and-count shapes excluding record ids by type; documenting their keys discloses nothing a caller does not already receive. | seanb4t | 2026-08-21 |
| R-08-03 | T-08-17 | Memory-contract additions restate content already public in `docs-site` and the tool schemas; no credential, endpoint, or internal-only detail introduced. | seanb4t | 2026-08-21 |
| R-08-04 | T-08-SC | Phase changed no dependency manifest in any ecosystem; Package Legitimacy Audit recorded as not applicable in `08-RESEARCH.md`. | seanb4t | 2026-08-21 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-21 | 19 | 19 | 0 | /gsd-secure-phase (orchestrator, ASVS L1 short-circuit) |

**Verification method.** ASVS L1 grep-depth, per the short-circuit rule
(`threats_open: 0` + register authored at plan time + `asvs_level == 1`); no
`gsd-security-auditor` subagent was spawned. Mitigation evidence was gathered by
reading the implementation and by executing the five pinning tests named in the
registers:

```
go test ./cmd/engram/... ./internal/surfaces/... -run \
  'TestSweepLeavesRejectPresentButEmptyScope|TestRequireSweepScope|TestSweepLeavesUsageStatesRegisteredRule|TestSurfaceConformanceProseFiles|TestValidateRules' -v
→ exit 0; 8 --- PASS, 0 FAIL, 0 SKIP
```

**Gate hygiene note.** The T-08-14 inventory cross-check was authored during this
audit and its first two formulations were defective — the first produced an empty
expected set (wrong extractor against a JSON golden, would have reported green on
any input), the second produced eight false positives because the CLAUDE.md row
writes `migrate` (`status`, `revert`) while the golden emits `"migrate status"`.
Only the third formulation was accepted, and only after a constructed-defect
self-test confirmed it fires. Recorded here because a gate written during an audit
carries the same vacuity risk as one written during planning.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-21
