# Phase 3: Bounded-Read Call-Site Inventory (D-08)

Scope: every non-test function in `internal/store` that emits a Qdrant
`Scroll`/`ScrollAndOffset`/`Query`/`QueryBatch`/`Get`/`Count` or calls
`scrollAllPoints`. The identity key is the enclosing function's display name
(`Store.<name>`) — the same key the recall-gate AST test
(`schemaversion_recallgate_test.go`) uses. Recorded at commit `ddf6355f`
(`git rev-parse --short HEAD` when this file was written).

## Derivation

```bash
for f in internal/store/*.go; do case "$f" in *_test.go) continue;; esac; awk '/^func /{fn=$0} /s[.]client[.](Scroll|ScrollAndOffset|Query|QueryBatch|Get|Count)[(]|s[.]scrollAllPoints[(]/{print fn}' "$f"; done | sed -E 's/^func [(][a-z]+ [*]?([A-Za-z]+)[)] ([A-Za-z]+).*/\1.\2/' | sort -u
```

This printed 27 names at the recorded SHA.

## Inventory

| Function | File | Qdrant call(s) | Payload | Current bound | Assigned | Justification |
|---|---|---|---|---|---|---|
| `Store.Search` | `internal/store/store.go` | Query | full payload | caller `k`, no maximum | Phase 4 | Recall path; REQ-search-k-bounded caps `k`. |
| `Store.SearchDiscovery` | `internal/store/store.go` | Query | full payload | caller `k`, no maximum | Phase 4 | Recall path; REQ-search-k-bounded caps `k`. |
| `Store.List` | `internal/store/store.go` | Count + offset-mode Scroll | full payload | `limit 0` = all, deep offset | Phase 4 | Recall path; REQ-list-bounded. The Count is a number, not a payload — exempt in spirit but the function's Scroll dominates its classification. |
| `Store.listByCursor` | `internal/store/store.go` | Scroll | full payload | `maxListLimit` (1000) records | Phase 4 | Recall path (List's cursor mode); migrates onto `Store.scrollOrderedPage`. |
| `Store.ListScheduled` | `internal/store/store.go` | Scroll | full payload | caller `opts.Limit`, no ceiling | Phase 4 | Recall path; REQ-list-scheduled-bounded. |
| `Store.scrollAllPoints` | `internal/store/spine.go` | ScrollAndOffset | per-view (`unbudgetedView` count-only for unmigrated callers; budgeted `fullView`/`summaryView` for byte-derived callers) | per-view byte-derived count and batch-of-1 (D-02/D-06/D-07) | Phase 3 primitive | Built by plan 03-02: the byte-budget sweep primitive every Phase 5 sweep migrates onto. |
| `Store.scrollOrderedPage` | `internal/store/orderedpage.go` | Scroll | per-view | byte-derived count + measured page budget + batch-of-1 (D-02/D-03/D-06/D-07) | Phase 3 primitive | Built by this plan (03-04): the ordered-page primitive every Phase 4 `List`-shaped read migrates onto. |
| `Store.ScanSpine` | `internal/store/spine.go` | via `scrollAllPoints` | `unbudgetedView(qdrant.NewWithPayload(true))` | inherits `scrollAllPoints`'s bound | Phase 5 | spine-review scan; rides along with `scrollAllPoints`'s extension when migrated off `unbudgetedView`. |
| `Store.EnumerateCitations` | `internal/store/spine.go` | via `scrollAllPoints` | `unbudgetedView(qdrant.NewWithPayload(true))` | inherits `scrollAllPoints`'s bound | Phase 5 | spine-review verify. |
| `Store.NearDuplicates` | `internal/store/spine.go` | id enumeration via `scrollAllPoints`; `QueryBatch` | id enumeration: `unbudgetedView(qdrant.NewWithPayloadInclude("short_id", "scope"))`; `QueryBatch` requests no payload | inherits `scrollAllPoints`'s bound; `QueryBatch` payload-frugal | Phase 5 | Near-duplicate detection; its `QueryBatch` is exempt in effect (no payload requested) but the function's `scrollAllPoints` id enumeration dominates its classification. |
| `Store.derivePurgeEligible` | `internal/store/spine.go` | via `scrollAllPoints` | `unbudgetedView(qdrant.NewWithPayload(true))` | inherits `scrollAllPoints`'s bound | Phase 5 | spine-review purge. |
| `Store.previewRevertWithSteps` | `internal/store/revert.go` | via `scrollAllPoints` | `unbudgetedView(qdrant.NewWithPayload(true))` | inherits `scrollAllPoints`'s bound | Phase 5 | migrate revert preview. |
| `Store.revertWithSteps` | `internal/store/revert.go` | Count + ScrollAndOffset (own re-derivation, mirrors `Store.Migrate`) | full payload | `migrateBatch`-shaped batch | Phase 5 | migrate revert apply. Its Count is exempt (a number, not a payload). |
| `Store.Migrate` | `internal/store/migrate.go` | Count (×1) + ScrollAndOffset (×3 loops) | full payload | `migrateBatch` (256) | Phase 5 | `engram migrate` schema-version sweep. Its Count is exempt. |
| `Store.SummarizeMissing` | `internal/store/summarize.go` | ScrollAndOffset | full payload | literal `256` | Phase 5 | `engram summarize-missing`. |
| `Store.Reindex` | `internal/store/store.go` | ScrollAndOffset | full payload, `WithVectors(false)` | `reindexBatch` (256) | Phase 5 | `engram reindex` embedder migration. |
| `Store.reindexTargetContents` | `internal/store/store.go` | Get (batch) | `WithVectors(false)` | bounded by the calling page's `batch` (256) | Phase 5 | Rides along with `Store.Reindex`: one `Get` per reindex page's ids. |
| `Store.ListScopes` | `internal/store/store.go` | Scroll | `WithPayloadInclude("scope")` only | `scanCap` (1000) | Exempt | Already payload-scoped (the #583 fix); kept as a plain `Scroll` deliberately per its own doc comment, for the recall-gate AST test. |
| `Store.Get` | `internal/store/store.go` | Scroll | full payload, one point | id-addressed, no page | Exempt | Single-point fetch is a fundamentally different risk shape (one record, not a page) — there is no page to bound. D-01's content cap plus Phase 5's production `MaxCallRecvMsgSize` backstop cover it. |
| `Store.ResolvePointID` | `internal/store/store.go` | Scroll | `WithPayload(false)` | `Limit: 2` | Exempt | No payload requested at all; id-addressed lookup, not a recall surface. |
| `Store.CountOwnerless` | `internal/store/store.go` | Count | none | N/A | Exempt | `Count` returns a number, never payloads. |
| `Store.CountAnonymousBucket` | `internal/store/store.go` | Count | none | N/A | Exempt | `Count` returns a number, never payloads. |
| `Store.MigrateSetOwner` | `internal/store/store.go` | Count | none | N/A | Exempt | `Count` returns a number, never payloads. |
| `Store.RemapOwner` | `internal/store/store.go` | Count | none | N/A | Exempt | `Count` returns a number, never payloads. |
| `Store.MintShortID` | `internal/store/store.go` | Count | none | N/A | Exempt | Collision probe; `Count` returns a number, never payloads. |
| `Store.CountExpired` | `internal/store/spine.go` | Count | none | N/A | Exempt | `Count` returns a number, never payloads. |
| `Store.MigrateStatus` | `internal/store/migrate_status.go` | Count (×2) | none | N/A | Exempt | Both Counts return numbers, never payloads. |

## Phase 5 closing check

Phase 5 re-runs these commands verbatim before closing REQ-bounded-read-mechanism:

(a) The derivation command (above). Its output must equal this table's Function set.

```bash
for f in internal/store/*.go; do case "$f" in *_test.go) continue;; esac; awk '/^func /{fn=$0} /s[.]client[.](Scroll|ScrollAndOffset|Query|QueryBatch|Get|Count)[(]|s[.]scrollAllPoints[(]/{print fn}' "$f"; done | sed -E 's/^func [(][a-z]+ [*]?([A-Za-z]+)[)] ([A-Za-z]+).*/\1.\2/' | sort -u
```

(b) No unbudgeted `scrollAllPoints` caller remains — the constructor is deleted once this is 0:

```bash
rg -o 's[.]scrollAllPoints[(].*unbudgetedView[(]' internal/store --glob '!*_test.go' | wc -l
```

Must print `0`.

(c) Every Phase 4 and Phase 5 row above must, by Phase 5's close, route through `Store.scrollOrderedPage` or a budgeted `Store.scrollAllPoints` view (`fullView`/`summaryView`), or be moved to Exempt with a new written justification.

(d) The recall gate's classifications must match: `Store.scrollOrderedPage` moves from `otherNonRecallEmitters` to `recallTransmitters` in Phase 4's own change that wires `Store.List`/`Store.listByCursor`/`Store.ListScheduled` onto it.

## Not adopted (D-05)

The two-phase ids→payload design (stored `payload_bytes`, a schema-version step, a
`GetPoints` re-fetch) was considered and not adopted, so the
TOCTOU-on-delete/supersede/archive and `GetPoints`-order acceptance criteria ROADMAP
Phase 3 names for that design do not apply.
