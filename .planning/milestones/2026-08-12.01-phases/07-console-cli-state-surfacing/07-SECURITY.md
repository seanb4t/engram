---
phase: 07
slug: console-cli-state-surfacing
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-21
---

# Phase 07 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

This phase relaxes the store's recall gate behind three orthogonal opt-in booleans
(`include_archived` / `include_superseded` / `include_scheduled`) on the Connect and CLI lanes,
adds a `MigrateStatus` Connect read RPC exposing a whole-collection schema histogram, adds the
`engram get` and `engram migration-status` verbs, and surfaces record state in the web console.
Two properties carry the security weight: **state relaxation must never move the authorization
boundary** (D-04), and the new headline path must not reopen terminal injection (issue #505).

## Trust Boundaries

| Boundary | Description | Data Crossing |
|---|---|---|
| Connect client → `ListMemories` / `SearchMemories` handlers | An authenticated caller supplies three new request bools that relax recall conditions | Filter-condition decisions; never authz decisions |
| `internal/server` typed core → `internal/store` | The bools become Qdrant filter-condition appends | `ListOptions` / `SearchOptions` state flags |
| any authenticated Connect caller → `MigrateStatus` | A whole-collection aggregate crosses to a caller with no owner scoping | Version-bucket counts only — no ids, scopes, owners, or content |
| stored record content → operator terminal | `engram get` is the first surface routing record-derived state into a `renderOperatorView` headline | Record summary/owner/tags, sanitized |
| URL query string → console filter state | An operator-supplied or shared URL controls which include flags are set | Filter state only — no credential, no scope grant |
| server response → browser DOM | Record-derived values render into list rows and the detail pane | Full UUIDs, timestamps, fixed-vocabulary state words |
| CLI primary command → best-effort footer lookup | A secondary RPC must not affect the primary command's result or exit code | Aggregate counts, bounded and discardable |

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|---|---|---|---|---|---|---|
| T-07-01 | Tampering | `renderOperatorView` headline, `cmd/engram/operator_view.go` | high | mitigate | Headline routed through `sanitizeViewValue` at `operator_view.go:267` — the single write site. Regression test `TestRenderOperatorViewSanitizesHeadline` (`operator_view_test.go:572`) proves C0/DEL neutralisation. Closes issue #505; lineage T-06-03. | closed |
| T-07-02 | Elevation of Privilege | `Store.Search` / `Store.List` gate blocks, `internal/store/store.go` | high | mitigate | Authz filter built BEFORE the conditional state appends at both sites: `store.go:1105` (`ownerScopeFilter`) and `:1376` (`listFilter` → `ownerOrSharedCondition` at `:1321`); state appends follow at `:1112-1130` / `:1383-1400` and are the only conditional parts. `TestSearchAndListAuthorizationOrthogonalToState` (`store_test.go:7285`) passes live: a second subject with all three flags set sees zero of another owner's private records on both lanes. | closed |
| T-07-03 | Information Disclosure | `MigrateStatus` Connect handler | medium | accept | `MigrateStatusResponse` (`proto/engram/v1/engram.proto:194-206`) has no id, scope, or owner field — aggregate buckets and counts only. Whole-collection scope is deliberate: an owner-scoped histogram can read zero while a large legacy backlog exists. Grounded in locked D-06. | closed |
| T-07-05 | Information Disclosure | `MigrationBanner` on every route | low | accept | Renders only `pending` / `futureTotal` (`MigrationBanner.svelte:27-40`) — the same aggregate T-07-03 already discloses, rendered rather than newly obtained. | closed |
| T-07-06 | Tampering | `renderMemoryTable` STATE cell, `cmd/engram/client_common.go` | low | accept | `memoryStateWords` (`cmd/engram/memory_state.go:39-67`) appends only the four literal vocabulary strings; no record-controlled bytes enter the cell. | closed |
| T-07-07 | Information Disclosure | `MemoryDetail` successor / predecessor links | medium | mitigate | Links render unconditionally from `memory.supersededBy` / `memory.supersedes` — fields the viewer already holds (`MemoryDetail.svelte:150-163`). No per-link RPC at render time and no short-handle resolution, so link presence is not a readability oracle. An unreadable target fails on activation through the 404-indistinguishable envelope (`DEC-xa6`). | closed |
| T-07-08 | Information Disclosure | include bools, Connect + console lanes | medium | accept | State relaxation widens what an already-authorized caller sees within their own readable set; the authz boundary does not move. Same evidence as T-07-02 — `connectapi.go:289-291,350-352` wire the flags only into store opts. Grounded in locked D-04. | closed |
| T-07-09 | Tampering | `MemoryRow` / `MemoryDetail` state rendering | low | mitigate | `RecordStateWord` is a closed four-member union (`ui/src/lib/memorystate.ts:9`); `MemoryRow.svelte:75` interpolates plain text. No new `{@html}` — the single existing use at `MemoryDetail.svelte:129` predates this phase (blame: `49309b2e`). | closed |
| T-07-10 | Denial of Service | `MemoryRow` meta-line wrapping | low | accept | No virtualization and no fixed row height in `MemoryRow.svelte` or `MemoryList.svelte`, so variable height from wrapping degrades layout at worst, never throughput. | closed |
| T-07-11 | Elevation of Privilege | `Store.ListScheduled` / `Store.SearchDiscovery` | high | mitigate | The two out-of-scope gate sites do not inherit the relaxation. `ListScheduled` builds its filter inline with hardcoded `IsEmpty` gates and never reads `opts.Include*` (`store.go:1598-1636`, documented at `:1281-1283`); `SearchDiscovery` takes no options parameter at all (`store.go:1195-1238`, gates at `:1224,1228`). `TestListScheduledSupersededHidden` (`store_test.go:2450-2501`) passes all three flags to `ListScheduled` and still asserts the superseded record is hidden. | closed |
| T-07-12 | Repudiation | `backlogFilter` reachability claim, `internal/store/migratebacklog.go` | medium | mitigate | Doc comment re-derived at `migratebacklog.go:44-61` on grounds that survive conditional gating, naming phase 07 plan 03 as the forcing change. `TestSchemaVersionNeverGatesRecall` passes with 18 filters walked. | closed |
| T-07-13 | Tampering | `parseObserveParams` `inc` parameter | low | mitigate | `queries.ts:20` filters `sp.getAll('inc')` against the exported `INCLUDE_STATES` list, so a crafted URL cannot inject an unrecognised value. Asserted by the drop test at `queries.test.ts:82-87`. | closed |
| T-07-14 | Spoofing | shared / bookmarked console URLs | low | accept | The URL carries only `scope`/`cat`/`vis`/`inc`/`offset`/`sel` (`queries.ts:18-45`) — no credential and no scope grant. A recipient sees only what their own session authorizes. | closed |
| T-07-15 | Information Disclosure | `engram get` over the ungated `GetMemory` path | medium | accept | `deps.getMemory` (`internal/server/tools.go:1798-1816`) uses `GetReadable(ctx, pid, c.Subj)` and re-wraps every failure as the same `ErrNotFound`, keeping unauthorized indistinguishable from missing (`DEC-xa6`). Reached identically by Connect `GetMemory` (`connectapi.go:382`) and `engram get` (`client_get.go:56`) — the verb adds a transport, not an authorization path. | closed |
| T-07-16 | Tampering | `engram get`'s json lane | low | mitigate | `client_get.go:68-71` and `client_common.go:444-448` construct field-identical `protojson.MarshalOptions{UseProtoNames:true, EmitDefaultValues:true}`, so neither lane can render a field the other omits. `TestGetOutputIdentity` (`client_get_test.go:149`) covers both a set and an unset optional field. See Note 1. | closed |
| T-07-17 | Spoofing | `MigrateStatus` Connect handler | high | mitigate | `connectapi.go:200-203` calls `subjectFromConnectContext(ctx)` and returns `CodeUnauthenticated` BEFORE `a.d.st.MigrateStatus(ctx)` is reached, mirroring `ListScopes`. `TestConnectMigrateStatusUnauthenticatedRejected` (`connectapi_test.go:1479`) additionally asserts the store was never called. | closed |
| T-07-18 | Repudiation | `Store.MigrateStatus` error path | medium | mitigate | `migrate_status.go:214,226` raise distinct errors for facet truncation vs concurrent-writer reconciliation failure; the handler maps them through `connectError` and returns a nil response, so a zeroed histogram is never reported as healthy. `TestConnectMigrateStatusStoreErrorSurfacesAsConnectError` asserts `resp == nil` alongside the error. | closed |
| T-07-19 | Denial of Service | best-effort footer lookup on `search` / `list` | low | accept | Bounded by `min(resolvedTimeout, footerLookupBudget)` with a 2s budget (`client_common.go:329,359-361`), run sequentially after the primary RPC (`client_list.go:102`, `client_search.go:90`) with the `ok` return discarded so failure never propagates to output or exit code. A degraded server costs at most one timed-out request, never a retry storm. | closed |
| T-07-20 | Denial of Service | `MigrationBanner` fetch scheduling | low | mitigate | `MigrationBanner.svelte:17-25` sets `staleTime: Infinity`, `refetchOnMount: false`, `refetchOnWindowFocus: false`, `retry: false`; `AppShell.svelte:30` mounts it once from the root layout, not per route. | closed |
| T-07-21 | Tampering | banner copy interpolation | low | mitigate | `MigrationBanner.svelte:34,40` interpolate only two BigInt counts; no record-derived string reaches the DOM and no `{@html}` is introduced. | closed |
| T-07-22 | Repudiation | silent failure of the migration lookup | medium | mitigate | `errors.ts:40-51` — `handleQueryError` keeps the auth-redirect branch first (`:41-45`), then `meta.silent` → `logError` only (`:46-49`). Wired as the sole `QueryCache.onError` at `+layout.svelte:20-21` with no component-level `$effect` duplicating it, so the diagnostic is neither lost nor doubled while the user-facing banner stays silent. | closed |
| T-07-SC | Tampering | npm / pip / cargo installs | low | accept | `git diff 2a13d0aa18^..HEAD --stat -- go.mod go.sum ui/package.json ui/pnpm-lock.yaml` is empty across the phase's 68 commits — no dependency added in any ecosystem, preserving the zero-new-dependency invariant. | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---|---|---|---|---|
| R-07-01 | T-07-03, T-07-05 | The migration histogram is whole-collection and readable by any authenticated caller. An owner-scoped histogram would answer the wrong question — it can read zero while a large legacy backlog exists. Acceptable for a self-hosted operator console; `internal/authz/entities.go`'s deliberately-omitted `roles` attribute is the forward-compat seam if a later milestone narrows it. | Sean (locked D-06) | 2026-08-20 |
| R-07-02 | T-07-08, T-07-14 | Relaxing state gates makes archived/superseded/windowed records readable by exactly the callers their live predecessors were readable by. State visibility widens within an already-authorized set; the authorization boundary does not move, and enforcement stays in `internal/store` where the console cannot widen its own authorization. | Sean (locked D-04) | 2026-08-20 |
| R-07-03 | T-07-15 | `GetMemory` is deliberately not recall-gated and was already reachable via the `get_memory` MCP tool and the Connect lane with identical authorization. `engram get` adds a transport, not an authorization path. | Sean (locked D-01) | 2026-08-20 |
| R-07-04 | T-07-06, T-07-09, T-07-10, T-07-19 | Low-severity residuals accepted on structural grounds: fixed-vocabulary rendering (no record bytes in state cells), no virtualization to destabilise, and a bounded discardable footer lookup. | gsd-security-auditor | 2026-08-21 |

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|---|---|---|---|---|
| 2026-08-21 | 22 | 22 | 0 | gsd-security-auditor (ASVS L1, block_on high) |

Register origin: authored at plan time — all seven PLAN.md files carry a parseable `<threat_model>`
block, so this was a verification run, not retroactive STRIDE. No `## Threat Flags` section exists in
any SUMMARY.md, so there were no unregistered flags to reconcile.

Every `mitigate` threat was verified against source with a file:line citation rather than against
plan prose, and the four `high`-severity threats that gate ship under `block_on: high` (T-07-01,
T-07-02, T-07-11, T-07-17) were independently re-derived from source rather than deferred to the
phase's earlier code review. Live test execution backed the verification where a runnable test
existed, including `internal/store` against a real Qdrant container.

### Note 1 — documentation-precision deviation (not a gap)

T-07-16's plan-time mitigation text says both lanes serialize through "ONE" `protojson.MarshalOptions`
value. The implementation uses two independently-constructed, field-identical literals
(`client_get.go:68-71`, `client_common.go:444-448`). The behavioural identity property the threat
cares about still holds and is tested by `TestGetOutputIdentity`; only the register's wording is
imprecise. Recorded rather than silently normalised, since a future reader diffing the register
against the code would otherwise see a discrepancy and have to re-derive this conclusion.

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-21
