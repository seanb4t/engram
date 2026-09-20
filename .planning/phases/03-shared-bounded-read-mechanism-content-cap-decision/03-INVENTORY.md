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
| `Store.fetchPayloadBatch` | `internal/store/searchfetch.go` | Scroll | per-view, byte-derived (D-09's two-phase fetch) | `perRPCLimit(view.maxRecordBytes)` per RPC, batch-of-1 fallback on overflow (D-07) | Phase 4 | Added at Phase 5 close: this row is Phase 4's own two-phase bounded search (04-04), the per-batch `Scroll` a 2026-08 code-review fix (WR-01) extracted out of `Store.fetchPayloadsByID` so a batch could retry its ids individually on `ErrResponseTooLarge`. The derivation (check (a)) sees it directly (`s.client.Scroll(`); the recall gate (`schemaversion_recallgate_test.go`) already classifies it as a `recallTransmitters` entry — this row was simply never added to this table when 04-04 shipped it. |
| `Store.List` | `internal/store/store.go` | Count + offset-mode Scroll | full payload | `limit 0` = all, deep offset | Phase 4 | Recall path; REQ-list-bounded. The Count is a number, not a payload — exempt in spirit but the function's Scroll dominates its classification. |
| `Store.listByCursor` | `internal/store/store.go` | Scroll | full payload | `maxListLimit` (1000) records | Phase 4 | Recall path (List's cursor mode); migrated onto `Store.scrollOrderedPage` (Phase 4). Phase 5 closing check (a) note: this row no longer emits `s.client.Scroll(` directly, so the single-level textual derivation cannot see it any more — it composes `scrollOrderedPage` one level down instead. Invisible to the derivation, not absent from the mechanism. |
| `Store.ListScheduled` | `internal/store/store.go` | Scroll | full payload | caller `opts.Limit`, no ceiling | Phase 4 | Recall path; REQ-list-scheduled-bounded. Phase 5 closing check (a) note: same as `Store.listByCursor` above — this row composes `Store.collectOrderedPages`/`Store.scrollOrderedPage` (Phase 4) rather than emitting `s.client.Scroll(` itself, so the derivation cannot see it either. |
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
| `Store.reindexTargetContents` | `internal/store/store.go` | Get (batch) | `WithVectors(false)`, full payload otherwise | bounded by the size of the page its caller (`Store.Reindex`) accumulates before flushing | Exempt | Reclassified at Phase 5 close: it routes through no shared primitive (not `scrollAllPoints`, not `scrollOrderedPage`) — it is a direct `s.client.Get` call. Its batch is bounded by the size of the page `Store.Reindex`'s own accumulator flushes (`flushReindexPage`), which is itself derived from `ReindexOptions.Batch`, never by a byte-budget view. This is a known, accepted limitation (05-04-SUMMARY.md's own deviation record): the default `Batch` (256) can overflow the receive limit for a source of unusually large records, so **CORRECTED at the phase-5 security audit:** an earlier draft of this justification attributed the bound to "an operator sizing `--batch` for the record size in play". That lever does not exist — `engram reindex` exposes `--target`, `--source`, `--dry-run`, `--resume`, `--timeout` and `--output`, and never sets `ReindexOptions.Batch`, so the CLI always runs at the fixed `reindexBatch = 256`. The honest statement is: this `Get` is unbounded by bytes, and at `DefaultRecordCaps()` (~970 KB/record ceiling) a 256-record page can reach ~237 MiB — past both the 4 MiB test limit and the 64 MiB production backstop — reachable through the documented `--resume` workflow on a collection of large-but-cap-compliant records, with no operator lever to reduce it. Tracked as GitHub **#596** rather than silently expanded into this phase. |
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

## Phase 5 closing-check results

Re-run 2026-09-20 against a clean, committed tree at the end of this phase's plans (05-01 through
this plan's own red-evidence registration commits).

### (a) Derivation vs. table

Command (verbatim, from `## Derivation` above):

```bash
for f in internal/store/*.go; do case "$f" in *_test.go) continue;; esac; awk '/^func /{fn=$0} /s[.]client[.](Scroll|ScrollAndOffset|Query|QueryBatch|Get|Count)[(]|s[.]scrollAllPoints[(]/{print fn}' "$f"; done | sed -E 's/^func [(][a-z]+ [*]?([A-Za-z]+)[)] ([A-Za-z]+).*/\1.\2/' | sort -u
```

Output: 27 lines total. `wc -l` on the raw output reports 27, but the FIRST line is blank —
not a function name.

**Finding 1 — the blank line is a command artifact, not a 27th function.** `awk`'s trigger regex
matches prose, not just code: `internal/store/spine.go:76`, a doc-comment line preceding that
file's first `^func` declaration, contains the literal substring `s.client.ScrollAndOffset(` in
running prose describing the mechanism `scrollAllPoints` replaces. Because this per-file `awk`
pass hits the trigger before `fn` is ever assigned, it prints an empty string. This is not a new
finding — 05-04-SUMMARY.md's own "Issues Encountered" section independently hit the identical
line inflating a different, unrelated grep's count by one. **The derivation's real function-name
set is 26 distinct names, not 27.** Recorded here in writing rather than "fixed" by editing the
derivation command: excluding comment lines would require the command to parse Go syntax rather
than match text, which is a different (and heavier) tool than this inventory's own stated scope,
and the workaround of skipping this one line by content would itself need re-verification against
every future edit to that comment. The 26-name derivation is accepted as this check's true output.

**Finding 2 — the table's Function set is 28 rows** (27 recorded at Phase 3 + this plan's new
`Store.fetchPayloadBatch` row, below). Diffing the table's 28 names against the derivation's 26
real (non-blank) names:

- In the table, NOT in the derivation (2): `Store.listByCursor`, `Store.ListScheduled` — both
  ROW-ANNOTATED above. Confirmed by reading `internal/store/store.go`: neither function body
  contains `s.client.Scroll(`, `s.client.ScrollAndOffset(`, or `s.scrollAllPoints(` today — each
  now composes `Store.scrollOrderedPage`/`Store.collectOrderedPages` (Phase 4) one level down,
  invisible to a single-level textual match.
- In the derivation, NOT in the table before this plan (1): `Store.fetchPayloadBatch` — the one
  real gap, closed by the new row above (Phase 4, D-09's two-phase search; a 2026-08 code-review
  fix, WR-01, that this table simply never recorded when it shipped).
- The remaining 25 names appear in both, unchanged.

**Verdict: reconciled.** 28 (table) = 25 common + 2 derivation-blind (annotated) + 1
table-exclusive addition (`Store.fetchPayloadBatch`, now present); 26 (derivation) = 25 common +
1 (`Store.fetchPayloadBatch`, table-exclusive before this plan, now matched). No further table
edit closes the `listByCursor`/`ListScheduled` gap — they will never reappear in the derivation's
output while they compose `scrollOrderedPage`, and that absence is correct: bounded, not missing.

### (b) No unbudgeted `scrollAllPoints` caller

Command:

```bash
rg -o 's[.]scrollAllPoints[(].*unbudgetedView[(]' internal/store --glob '!*_test.go' | wc -l
```

Output: `0`. Confirmed independently with a whole-package (not production-scoped) grep too —
`rg -o 'unbudgetedView' internal/store | wc -l` also prints `0` — the constructor itself is
deleted (plan 05-02), not merely its production call sites. **Verdict: pass.**

### (c) Every Phase 4/5 row routes through a shared primitive or Exempt

Verified by reading each row's current call site — a property of the code, not a re-run grep:

| Row | Current binding | Verdict |
|---|---|---|
| `Store.ScanSpine` | `scrollAllPoints(..., s.scanView(), ...)` — budgeted (`scanRecordCeiling`) | pass |
| `Store.EnumerateCitations` | `scrollAllPoints(..., s.citationsView(), ...)` — budgeted (`citationsRecordCeiling`) | pass |
| `Store.NearDuplicates` | id enumeration: `scrollAllPoints(..., nearDuplicateIdentityView(), ...)` — budgeted; `QueryBatch` — see Finding 3 below | pass |
| `Store.derivePurgeEligible` | `scrollAllPoints(..., s.summaryView(), ...)` — budgeted (`summaryRecordCeiling`) | pass |
| `Store.previewRevertWithSteps` | `scrollAllPoints(..., schemaVersionOnlyView(), ...)` — budgeted (`schemaVersionOnlyRecordCeiling`) | pass |
| `Store.Migrate` | all three loops (DryRun, Manifest, sweep) `scrollAllPoints(..., s.fullView(), ...)` — budgeted; sweep pass additionally bounded by `errMigratePassBatchComplete`'s per-pass count | pass |
| `Store.revertWithSteps` | own pass loop `scrollAllPoints(..., s.fullView(), ...)` — budgeted; additionally bounded by `errRevertPassBatchComplete` | pass |
| `Store.SummarizeMissing` | `scrollAllPoints(..., s.fullView(), ...)` — budgeted; caller-limit additionally bounded by `errSummarizeLimitReached` | pass |
| `Store.Reindex` | `scrollAllPoints(ctx, source, nil, s.fullView(), ...)` over the EFFECTIVE source — budgeted; per-page accumulator (`flushReindexPage`) | pass |
| `Store.reindexTargetContents` | direct `s.client.Get`, no shared primitive | **moved to Exempt** (row above) with a written justification |
| `Store.List` / `Store.listByCursor` / `Store.ListScheduled` | `Store.scrollOrderedPage` (Phase 4) | pass (unchanged since Phase 4's close) |

**Verdict: pass** — every one of the ten remaining Phase 5 rows routes through a budgeted
`scrollAllPoints` view; the eleventh (`Store.reindexTargetContents`) is correctly Exempt with a
written justification, never silently omitted.

**Finding 3 — restating `Store.NearDuplicates`' `QueryBatch` exemption in writing, as D-04 asked
this phase to confirm rather than inherit.** Read directly: `internal/store/spine.go`'s
`QueryBatch` call constructs `&qdrant.QueryBatchPoints{CollectionName: s.collection, QueryPoints:
qp}` with no `WithPayload` field set — Qdrant's zero-value default for an unset payload selector
returns no payload. Confirmed again here by direct reading, not inherited from the table's own
prior claim: `Store.NearDuplicates`' `QueryBatch` stays Exempt because it requests no payload at
all, full stop.

### (d) Recall-gate classifications match

`internal/store/schemaversion_recallgate_test.go`'s `recallTransmitters` list already carries
`Store.scrollOrderedPage` (not `otherNonRecallEmitters`), justified by `Store.List`'s offset AND
cursor modes and `Store.ListScheduled` all routing through it (Phase 4's own change, plans
04-02/04-03). `TestRecallEmissionSetIsCompleteAndClassified` passes as part of the full `task` run
this plan's own verification records. **Verdict: pass — unchanged since Phase 4's close,
re-confirmed here.**

## Not adopted (D-05)

The two-phase ids→payload design (stored `payload_bytes`, a schema-version step, a
`GetPoints` re-fetch) was considered and not adopted, so the
TOCTOU-on-delete/supersede/archive and `GetPoints`-order acceptance criteria ROADMAP
Phase 3 names for that design do not apply.
