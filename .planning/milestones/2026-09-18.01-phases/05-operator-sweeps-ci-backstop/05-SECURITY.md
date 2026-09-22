---
phase: "5"
slug: "operator-sweeps-ci-backstop"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-20"
---

# Phase 5 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

The register was authored at plan time across all six plans' `<threat_model>` blocks
(`register_authored_at_plan_time: true`), so this audit verified that each declared mitigation is
present in the shipped implementation rather than scanning for new threats. ASVS L1 is configured;
the filter-fidelity, payload-projection, collection-parameter, sentinel-unwrapping and
backstop-ordering threats were verified at L2/L3 depth because they are this phase's real surface.

The audit was not grep-only: it ran the live red-evidence harness (53/53 confirmed RED, tree clean
afterwards), ran the in-place characterization suites unedited, read every migrated callback body
against its paired view's selector, traced each sentinel's `errors.Is` unwrap site, and confirmed
`gh issue view 497` reports CLOSED.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Operator CLI → `internal/store` | `engram migrate`, `migrate revert`, `summarize-missing`, `spine-review`, `reindex`. Operator-tier and **Subject-less by design** — the existing contract, not a regression introduced here | Scope/collection names, batch and timeout options |
| `internal/store` → Qdrant (gRPC) | The capacity boundary. Every sweep now iterates through the single `Store.scrollAllPoints`, which sizes each request from a per-view byte ceiling | Filtered points, payloads projected per sweep |
| `Store.Reindex` → a SECOND collection | New this phase: `scrollAllPoints` gained an explicit `collection` parameter so reindex can walk `opts.Source` rather than `s.collection` | Source-collection points |

---

## Threat Register

41 threats, all closed. Verified 2026-09-20 by `gsd-security-auditor` against the shipped tree.

| Threat ID | Category | Severity | Disposition | Mitigation | Status |
|-----------|----------|----------|-------------|------------|--------|
| T-05-01-01 | Tampering | high | mitigate | Filters byte-identical to pre-migration at `spine.go:301,408,614,1083`; only the `readView` argument changed | closed |
| T-05-01-02 | Information Disclosure | high | mitigate | Every callback body read against its view's selector (`boundedread.go:293-330`) — no field read outside the projection | closed |
| T-05-01-03 | Denial of Service | medium | accept | ~2 records/RPC for `scanView` recorded in `boundedread.go:237-246` and 05-01-SUMMARY — a documented cost, not a gate | closed |
| T-05-01-04 | Tampering | high | mitigate | All ten non-Reindex `scrollAllPoints` calls pass `s.collection`; only Reindex passes `source` (`store.go:3643`) | closed |
| T-05-01-05 | Repudiation | high | mitigate | Spine suites green, unedited (D-02) | closed |
| T-05-01-SC | Tampering | low | accept | `go.mod`/`go.sum` unchanged | closed |
| T-05-02-01 | Tampering | high | mitigate | `schemaVersionOnlyView` (`boundedread.go:339`) and `versionOf` both key on `schemaVersionKey` | closed |
| T-05-02-02 | Repudiation | high | mitigate | `unbudgetedView` references = 0, whole-package (not production-scoped) | closed |
| T-05-02-03 | Denial of Service | medium | mitigate | Constructor and all five callers moved in one commit (`e9afaee6`); build and vet clean | closed |
| T-05-02-04 | Tampering | medium | mitigate | `orderedpage_oversized_test.go:660` retargeted to `store.ReadView{}` with recorded reasoning | closed |
| T-05-02-05 | Information Disclosure | high | mitigate | `revert.go:290` — `aboveTargetFilter(to)` unchanged, only the view swapped | closed |
| T-05-02-SC | Tampering | low | accept | `go.mod`/`go.sum` unchanged | closed |
| T-05-03-01 | Tampering | critical | mitigate | The DryRun callback (`migrate.go:315-366`) contains no write call — only `previewManifest[id] = fromV` | closed |
| T-05-03-02 | Tampering | high | mitigate | Forward-cursor design note at `migrate.go:532-542`; observed `res.Migrated` == seeded, `res.Failed` == 0 | closed |
| T-05-03-03 | Repudiation | high | mitigate | Four distinct sentinels, each unwrapped via `errors.Is` at its own call site only — no cross-confusion | closed |
| T-05-03-04 | Repudiation | high | mitigate | PA-3 non-shrinking-backlog guard intact (`migrate.go:517-529`); many-small observed `res.Passes = 5` | closed |
| T-05-03-05 | Tampering | critical | mitigate | Filter arguments unchanged at every migrated call site | closed |
| T-05-03-06 | Repudiation | medium | mitigate | `Store.Migrate`/`Store.revertWithSteps` rows retained with corrected justifications (both keep a direct `Count`) | closed |
| T-05-03-SC | Tampering | low | accept | `go.mod`/`go.sum` unchanged | closed |
| T-05-04-01 | Spoofing | high | mitigate | `store.go:3643` passes `source`, never `s.collection` | closed |
| T-05-04-02 | Information Disclosure | high | mitigate | `summarize.go:162-193` branch order preserved verbatim | closed |
| T-05-04-03 | Denial of Service | high | mitigate | `reindexTargetContents` has exactly one call site, one `Get` per accumulated page — **see the unregistered flag below, which this ID does NOT cover** | closed |
| T-05-04-04 | Tampering | high | mitigate | Trailing `flushReindexPage(false)` after clean iterator exit (`store.go:3657-3661`) | closed |
| T-05-04-05 | Repudiation | medium | mitigate | `Store.SummarizeMissing`/`Store.Reindex` rows absent from `operatorMigrationEmitters`; gate green | closed |
| T-05-04-06 | Tampering | high | mitigate | Reindex dry-run tests green (writes nothing, resumes correctly) | closed |
| T-05-04-SC | Tampering | low | accept | `go.mod`/`go.sum` unchanged | closed |
| T-05-05-01 | Tampering | critical | mitigate | Backstop appended at `store.go:578`, `opts...` at `:579`; `TestQdrantRecvLimitBackstopPrecedesCallerOptions` passes and a red-evidence patch reverses it to confirm RED | closed |
| T-05-05-02 | Repudiation | high | mitigate | 36 test-dial sites still name `storetest.RecvLimit` explicitly; oversized regressions pass | closed |
| T-05-05-03 | Repudiation | high | mitigate | `qdrantbackstop_test.go` contains no live-dial or `ResourceExhausted` vocabulary — pass-through and order only (D-06) | closed |
| T-05-05-04 | Elevation of Privilege | medium | mitigate | Exactly one non-test `MaxCallRecvMsgSize` call site and one literal | closed |
| T-05-05-05 | Denial of Service | medium | accept | The 64 MiB masking trade-off is recorded at `store.go:540-549` (D-05) | closed |
| T-05-05-SC | Tampering | low | accept | `go.mod`/`go.sum` unchanged | closed |
| T-05-06-01 | Tampering | medium | mitigate | Tree porcelain-clean after all 53 apply/revert cycles | closed |
| T-05-06-02 | Repudiation | high | mitigate | The harness dynamically re-proves all 53 patches RED, not merely registered | closed |
| T-05-06-03 | Repudiation | high | mitigate | All 53 `confirmed RED:` lines present; no unresolved target | closed |
| T-05-06-04 | Repudiation | high | mitigate | Inventory drift named and reconciled in writing (the 26-vs-27 blank-line artifact explained, not rounded away) | closed |
| T-05-06-05 | Repudiation | high | mitigate | REQUIREMENTS.md: exactly 17 ticked / 3 unticked, four "Phase 5 \| Complete" rows | closed |
| T-05-06-06 | Tampering | high | mitigate | Git history clean for phases 01/02/04 directories across this milestone | closed |
| T-05-06-07 | Repudiation | medium | mitigate | `gh issue view 497` → CLOSED with the full D-07 reasoning comment | closed |
| T-05-06-08 | Denial of Service | medium | mitigate | Harness completed without narrowing; no CI or Taskfile timeout raised | closed |
| T-05-06-SC | Tampering | low | accept | No installs | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Unregistered Flags

**WARNING — `Store.reindexTargetContents`' Exempt justification cited a control that does not exist.**

The recorded justification attributed its bound to "an operator sizing `--batch` for the record size
in play". **`engram reindex` has no `--batch` flag** — it exposes `--target`, `--source`,
`--dry-run`, `--resume`, `--timeout` and `--output`, and never sets `ReindexOptions.Batch`, so the
CLI always runs at the fixed `reindexBatch = 256`. Independently confirmed by the orchestrator
against `cmd/engram/reindex.go`.

The underlying limitation is real: that `Get` is unbounded by bytes, and at `DefaultRecordCaps()`
(~970 KB/record ceiling) a 256-record page can reach ~237 MiB — past both the 4 MiB test limit and
the new 64 MiB production backstop — reachable through the documented `--resume` workflow on
large-but-cap-compliant records, with no operator lever to reduce it. It maps to none of the 41
declared threat IDs (T-05-04-03 covers request-count collapse, not this).

**Actions taken:** the false justification was corrected in `03-INVENTORY.md` and `05-VALIDATION.md`
to state the limitation plainly, and the behavior is tracked as **GitHub #596** (candidate fixes: a
byte-budget view on the `Get`, preferred; or exposing `--batch`). Not fixed in this phase — doing so
would reopen a phase already verified, and the gap is narrow and now tracked.

Every other `## Threat Flags` section across the six SUMMARYs either declares no new surface or maps
onto that plan's own registered IDs.

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| R-05-01 | T-05-05-05 | The 64 MiB backstop masks a genuinely unbounded read longer than a tighter ceiling would. Raised at decision time and chosen deliberately (D-05); the fix is the byte-budget mechanism, which every regression proves WITHOUT the backstop in place. | user (discuss-phase, 2026-09-20) | 2026-09-20 |
| R-05-02 | T-05-01-03 | Per-sweep projections yield as few as ~2 records/RPC for the widest view. Accepted: correctness of the projection outranks RPC count, and the alternative (one shared full-payload view) is worse on every sweep. | orchestrator | 2026-09-20 |
| R-05-03 | (unregistered) | `reindexTargetContents`' unbounded per-page `Get` — accepted for THIS phase only, tracked as #596 with the justification corrected. Not a silent acceptance: the limitation is stated plainly wherever it was previously misdescribed. | user (2026-09-20, chose "file a GitHub issue") | 2026-09-20 |
| R-05-04 | all `T-05-NN-SC` | No supply-chain change this phase: zero new `go.mod`/`go.sum` entries, no installs. | orchestrator | 2026-09-20 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-20 | 41 | 41 | 0 | gsd-security-auditor (verify-mitigations mode, ASVS L1 / L2–L3 depth on the flagged high-risk items) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] The one unregistered flag corrected in-repo and tracked as GitHub #596
