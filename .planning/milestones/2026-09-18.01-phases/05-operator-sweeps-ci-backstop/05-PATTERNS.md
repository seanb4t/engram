# Phase 5: Operator Sweeps & CI Backstop - Pattern Map

**Mapped:** 2026-09-20
**Files analyzed:** 10 (8 modify sites in the 03-INVENTORY Phase-5 rows, plus 2 new-test targets)
**Analogs found:** 10 / 10 (every analog lives in-repo; several sites' best analog is a
sibling function in the SAME file already migrated, or Phase 3/4's own primitive)

## File Classification

| New/Modified File (site) | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/store/spine.go` — `Store.ScanSpine` (:278) | service (sweep) | batch (whole-spine scan) | `internal/store/revert.go` — `previewRevertWithSteps` (:278, already `scrollAllPoints`+`unbudgetedView`) | role-match — same call shape, only the view constructor swaps |
| `internal/store/spine.go` — `Store.EnumerateCitations` (:385) | service (sweep) | batch | same as above | role-match |
| `internal/store/spine.go` — `Store.NearDuplicates` id enum (:591) | service (sweep) | batch | `internal/store/boundedread.go` — `keysView()` (:250-259, Phase 4's own minimal-projection precedent) | exact — NearDuplicates already projects `short_id`+`scope`; this IS the keysView idiom, just uninstantiated |
| `internal/store/spine.go` — `Store.derivePurgeEligible` (:1052) | service (sweep) | batch | `internal/store/boundedread.go` — `summaryView()` (:216-222) | role-match — needs tags/category/timestamps but not content/citations |
| `internal/store/revert.go` — `previewRevertWithSteps` (:278) | service (sweep) | batch | itself — already correct shape; only its `unbudgetedView` argument changes | exact |
| `internal/store/revert.go` — `revertWithSteps` (:431, own pass-loop) | service (sweep, mutating) | batch (multi-pass write) | `internal/store/migrate.go` — the default sweep-mode pass loop (:483-594), byte-identical shape (Count → PA-3 guard → nil-offset ScrollAndOffset → per-point write) | exact structural twin — same author, same shape, migrate first |
| `internal/store/migrate.go` — `DryRun` full-backlog walk (:308-345) | service (sweep, read-only) | batch (exhaustive projection) | `internal/store/revert.go` — `previewRevertWithSteps` (:278, exhaustive `scrollAllPoints` walk already) | exact — same "exhaustive read-only pass, closure accumulates a map" shape |
| `internal/store/migrate.go` — `Manifest`-limited walk (:353-420) | service (sweep, mutating, single-pass) | batch | same DryRun walk in this file (sibling closure) plus `scrollAllPoints`'s contract | exact |
| `internal/store/migrate.go` — default sweep-mode pass loop (:483-594) | service (sweep, mutating, multi-pass) | batch | `internal/store/revert.go` — `revertWithSteps`'s own pass loop (structural twin, mutual analog) | exact (co-migrate together) |
| `internal/store/summarize.go` — `SummarizeMissing` (:145-182) | service (sweep) | batch (exhaustive, early-exit on `opts.Limit`) | `internal/store/spine.go` — `ScanSpine`'s post-migration shape (exhaustive `scrollAllPoints`, closure counters) | exact once ScanSpine migrates; today's nearest already-migrated exhaustive-with-early-exit shape is `Store.List`'s D-05 loop (Phase 4 PATTERNS.md) | 
| `internal/store/store.go` — `Reindex` (:3449, own loop) | service (sweep) | batch (exhaustive, resume lookup per page) | `internal/store/spine.go` — `ScanSpine` post-migration shape for the walk; `reindexTargetContents` itself is UNCHANGED (Get is outside the migration's scope, see below) | role-match |
| `internal/store/store.go` — `NewQdrantClient` (:529-536) | config/utility (client construction) | — | itself — the file's OWN existing base-dial-option append (`otelgrpc` stats handler + `classifyResponseTooLarge` interceptor, Phase 2) | exact — add a third base option in the same append block |
| `internal/store/boundedread.go` — new per-sweep `readView` constructors (D-04) | utility (transform, per-view sizing) | transform | same file's `fullView`/`summaryView`/`keysView` (:216-259) | exact — sibling constructors, same file |
| `internal/store/boundedread.go` — `unbudgetedView` deletion | utility | — | itself, doc comment already names its deletion condition (:260-266) | exact |
| `internal/store/schemaversion_recallgate_test.go` — `operatorMigrationEmitters` | test (AST/reflection gate) | event-driven | itself — existing entries for `Store.Migrate`, `Store.SummarizeMissing`, `Store.revertWithSteps`, `Store.Reindex`, `Store.scrollAllPoints` (:519-568) | exact |
| `internal/store/*_test.go` (new, per-sweep regressions) | test | integration (real Qdrant) | `internal/store/listscheduled_oversized_test.go`, `internal/store/searchtwophase_oversized_test.go`, `internal/store/orderedpage_oversized_test.go` | exact |
| `internal/store/*_test.go` (new, D-06 pass-through-only backstop test) | test | request-response (dial-option assertion) | `internal/store/storetest/storetest.go:265-286` (`dialOptions`, last-wins append) + `internal/store/qdrant_client_convergence_test.go` (construction-site gate, unmodified but the model for "assert shape, not third-party behavior") | exact |

## Pattern Assignments

### `internal/store/spine.go` — `Store.ScanSpine` (:278), `Store.EnumerateCitations` (:385), `Store.derivePurgeEligible` (:1052)

**Analog:** each is ALREADY built on `s.scrollAllPoints(ctx, filter, view, fn)` — the only change is the `view` argument. This is the smallest possible migration: swap `unbudgetedView(qdrant.NewWithPayload(true))` for a per-sweep projected view (D-04).

**Today's call site, unchanged in shape** (`spine.go:278`, `ScanSpine`):
```go
scanErr := s.scrollAllPoints(ctx, filter, unbudgetedView(qdrant.NewWithPayload(true)), func(p *qdrant.RetrievedPoint) error {
	m := fromPayload(p.Id.GetUuid(), p.Payload)
	res.Total++
	counts[bucketKey{scope: m.Scope, category: m.Category}]++
	if m.Summary == "" { res.WithoutSummary++ } else { res.WithSummary++ }
	if m.SupersededBy != nil { res.Superseded++ }
	if m.NotAfter != nil && !now.Before(*m.NotAfter) { res.Expired++ }
	if m.NotBefore != nil && m.NotBefore.After(now) { res.Scheduled++ }
	if m.ArchivedAt != nil { res.Archived++ }
	if n := len(m.Citations); n > 0 { res.WithCitations++; res.Citations += uint64(n) }
	if m.Owner != "" { owners[m.Owner] = true }
	return nil
})
```
**Fields ScanSpine's callback actually reads:** `Scope`, `Category`, `Summary` (presence only), `SupersededBy`, `NotAfter`, `NotBefore`, `ArchivedAt`, `Citations` (count only, via `len`), `Owner`. It never reads `Content` or `Tags`. This is a NEW projection shape, distinct from both `fullView` and `summaryView` (which excludes only content+citations but ScanSpine needs neither content, tags, nor citation bodies — only the citations COUNT, which still requires the field present since Qdrant cannot count array length server-side). Candidate per-sweep view: `qdrant.NewWithPayloadExclude("content", "tags")` — keeps citations (needed for `len`) while dropping the two largest uncapped-adjacent fields ScanSpine never touches.

**`EnumerateCitations`'s callback** (`spine.go:385`) reads `ID`, `ShortID`, `Scope`, `Category`, `Citations` — never `Content`, `Tags`, `Summary`, `SupersededBy`, etc. Candidate view: `qdrant.NewWithPayloadInclude("scope", "category", "citations", "short_id")` (id itself always returns unconditionally — never part of a selector, per `keysView`'s own doc comment). This is citations-bearing, so its ceiling must still budget `Citations*(CitationExcerptBytes+citationEntryAllowance)` — do not reuse `keysRecordCeiling` (256 bytes) for this one.

**`derivePurgeEligible`'s callback** (`spine.go:1052-1090`) reads `Tags` (`slices.Contains`), `Category`, `SupersededBy`, `NotAfter`, `ArchivedAt`, `CreatedAt`, `ID`, `ShortID`, `Scope` — never `Content`, `Citations`, `Summary`. Candidate view: `qdrant.NewWithPayloadExclude("content", "citations")` — this IS `summaryView()`'s exact selector; `derivePurgeEligible` can reuse `s.summaryView()` directly rather than a new constructor, since its ceiling (`summaryRecordCeiling`) already accounts for the tags term this function reads.

**Error handling:** unchanged in all three — `scrollAllPoints` returns the raw (possibly `ErrResponseTooLarge`) error; each function's `if scanErr != nil { return ..., scanErr }` is untouched.

---

### `internal/store/spine.go` — `Store.NearDuplicates` id enumeration (:591)

**Analog:** `internal/store/boundedread.go` `keysView()` (:250-259) — the exact idiom already in production for a keys-plus-two-tiny-fields projection; `NearDuplicates` is already hand-rolling the identical shape one level down (`qdrant.NewWithPayloadInclude("short_id", "scope")`), just wrapped in `unbudgetedView` instead of a budgeted sibling.

**Today's call site** (`spine.go:591`):
```go
enumErr := s.scrollAllPoints(ctx, enumFilter, unbudgetedView(qdrant.NewWithPayloadInclude("short_id", "scope")), func(p *qdrant.RetrievedPoint) error {
	id := p.Id.GetUuid()
	ids = append(ids, id)
	identities[id] = nearDuplicateIdentity{
		shortID: p.Payload["short_id"].GetStringValue(),
		scope:   p.Payload["scope"].GetStringValue(),
	}
	return nil
})
```
**New sibling to author, following `keysView`'s exact doc-comment convention** (`boundedread.go:242-259`):
```go
// nearDuplicateIdentityView is the two-field (short_id, scope) readView
// NearDuplicates' id enumeration uses (D-04) — sized like keysView but for
// two small strings instead of one timestamp. Not derived from RecordCaps:
// both fields are uncapped strings today (GitHub #589), so this is a
// documented fixed allowance, not a proven bound, exactly like
// keysRecordCeiling.
func nearDuplicateIdentityView() readView {
	return readView{selector: qdrant.NewWithPayloadInclude("short_id", "scope"), maxRecordBytes: nearDuplicateIdentityRecordCeiling}
}
```
Per D-04's own text: "confirm or restate with a written justification" whether `QueryBatch` stays exempt — it requests no payload at all (`spine.go` around :625, `qp[i] = &qdrant.QueryPoints{..., Limit: qdrant.PtrOf(topK)}` with no `WithPayload`), so it stays Exempt; only the id-enumeration `scrollAllPoints` call needs a budgeted view.

---

### `internal/store/revert.go` — `previewRevertWithSteps` (:278)

**Analog:** itself — this function is ALREADY the cleanest `scrollAllPoints` caller in the whole inventory (a single exhaustive read-only pass building a version-keyed cache, `revert.go:275-330`). Only the view argument changes.

**Today's call site:**
```go
err := s.scrollAllPoints(ctx, aboveTargetFilter(to), unbudgetedView(qdrant.NewWithPayload(true)), func(p *qdrant.RetrievedPoint) error {
	plan.Candidates++
	v := versionOf(p.Payload)
	...
})
```
**Fields read:** only `versionOf(p.Payload)` — i.e., `schema_version` alone (see `versionOf`, likely in `migrate.go`/`store.go`). This is the NARROWEST projection in the whole phase: `qdrant.NewWithPayloadInclude("schema_version")`. Candidate view, same doc-comment shape as `keysView`:
```go
// schemaVersionOnlyView is the one-field readView previewRevertWithSteps
// uses (D-04): the preflight only ever reads schema_version off each
// point, never any other payload key.
func schemaVersionOnlyView() readView {
	return readView{selector: qdrant.NewWithPayloadInclude("schema_version"), maxRecordBytes: schemaVersionOnlyRecordCeiling}
}
```
`schemaVersionOnlyRecordCeiling` should be smaller than or equal to `keysRecordCeiling` (256) — one small int, not an RFC3339 string.

---

### `internal/store/revert.go` — `revertWithSteps` (:431, own pass-loop) — the D-01 own-loop migration, hardest alongside `Migrate`'s sweep mode

**Analog:** `internal/store/migrate.go`'s default sweep-mode loop (:483-594) — a byte-for-byte structural twin (same author-stated mirroring: "PA-3-analog termination guard ... mirroring Store.Migrate's identical guard", `revert.go` doc comment directly above the loop). Migrate these TWO loops together with the SAME closure shape; whichever is written first becomes the concrete analog for the second.

**Today's shape to replace** (`revert.go:390-431`+):
```go
for {
	cnt, cerr := s.client.Count(ctx, &qdrant.CountPoints{CollectionName: s.collection, Filter: filter, Exact: qdrant.PtrOf(true)})
	...
	res.Backlog = cnt
	res.Passes++
	if cnt == 0 { return res, nil }
	if !first && cnt >= prevBacklog { /* PA-3 guard, named error */ return res, err }
	prevBacklog = cnt
	first = false

	// Offset is nil on EVERY pass, by design — each pass drains its own
	// batch out of the filter, so a resume is just calling Revert again.
	pts, _, serr := s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection, Filter: filter,
		Limit: qdrant.PtrOf(uint32(migrateBatch)), Offset: nil,
		WithPayload: qdrant.NewWithPayload(true),
	})
	...
	for _, p := range pts {
		// per-point write (chain apply + DeletePayload/SetPayload)
	}
}
```
**The migration shape (Claude's Discretion per CONTEXT — exact identifiers are the executor's call):** each PASS becomes one call to `s.scrollAllPoints(ctx, filter, view, fn)`, where `fn` writes each point exactly as today and increments a closure-scoped counter; once the counter reaches `migrateBatch` (today's per-pass page size), `fn` returns a SENTINEL error (e.g. `errPassBatchComplete`) that is not a real failure — `scrollAllPoints`'s contract (`spine.go:88-94`, "a callback error propagates out of scrollAllPoints") carries it straight out, and the pass loop's `errors.Is(perr, errPassBatchComplete)` check treats it as "this pass's batch is done, re-Count and start the next pass" rather than returning it to the caller. A genuine per-point write error is a DIFFERENT, non-sentinel error and propagates as a real failure exactly as today.

```go
// Illustrative shape only — identifiers are the executor's discretion.
var errPassBatchComplete = errors.New("revert: pass batch complete")

for {
	cnt, cerr := s.client.Count(...)          // UNCHANGED
	...                                        // UNCHANGED: Backlog, Passes, PA-3 guard
	processed := 0
	perr := s.scrollAllPoints(ctx, filter, view, func(p *qdrant.RetrievedPoint) error {
		// UNCHANGED per-point body (chain apply + Delete/SetPayload)
		processed++
		if processed >= migrateBatch {
			return errPassBatchComplete
		}
		return nil
	})
	if perr != nil && !errors.Is(perr, errPassBatchComplete) {
		err = perr
		return res, err
	}
}
```
**View to pass:** full payload is genuinely needed here (`payloadToMap(p.Payload)` decodes the WHOLE record to compute the reverse chain's before/after diff) — use `s.fullView()`, NOT a new per-sweep projection; D-04's "materially less" carve-out is for scan/verify/purge, not for a sweep that must round-trip the entire payload through `payloadToMap`.

**Which existing tests pin this migration (D-02):** `TestMigrateRevertPartialFailureReconciliation` (:371), `TestMigrateRevertMidLoopRefusalIsTypedAndCatchable` (:564), `TestMigrateRevertMidLoopUnsupportedRefusalIsTypedAndCatchable` (:638), `TestMigrateRevertFixtureInjectionConverges` (:155) — these four specifically exercise multi-pass behavior, mid-loop refusal typing, and the PA-3 guard; all must stay green with identical `RevertResult.Passes`/`Failed`/`Reverted` semantics.

---

### `internal/store/migrate.go` — `DryRun` full-backlog walk (:308-345) and `Manifest`-limited walk (:353-420)

**Analog:** BOTH are already single exhaustive `ScrollAndOffset` cursor loops with no PA-3 guard and no re-Count mid-walk — i.e., already `scrollAllPoints`-shaped, matching `previewRevertWithSteps`'s exact pattern (exhaustive walk, closure builds a map, error propagates raw).

**DryRun's loop to replace verbatim in structure** (`migrate.go:308-329`):
```go
var pageOffset *qdrant.PointId
for {
	pts, next, serr := s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection, Filter: filter,
		Limit: qdrant.PtrOf(batch), Offset: pageOffset,
		WithPayload: qdrant.NewWithPayload(true),
	})
	if serr != nil { err = serr; return res, err }
	for _, p := range pts {
		// applyChain(...) projection, no write; previewManifest[id] = fromV
	}
	if next == nil { break }
	pageOffset = next
}
```
becomes:
```go
scanErr := s.scrollAllPoints(ctx, filter, s.fullView(), func(p *qdrant.RetrievedPoint) error {
	id := p.Id.GetUuid()
	fromV := versionOf(p.Payload)
	original, derr := payloadToMap(p.Payload)
	if derr != nil { return fmt.Errorf("migrate: point %s: %w", id, derr) }
	if _, aerr := applyChain(id, fromV, original); aerr != nil { return aerr }
	previewManifest[id] = fromV
	return nil
})
if scanErr != nil { err = scanErr; return res, err }
```
No sentinel/early-termination needed here — this walk is genuinely exhaustive by design (REVIEWS.md H2: "the preview covers the WHOLE backlog, never one batch").

**Manifest walk** (`migrate.go:353-420`) is structurally identical — swap the same `for { ScrollAndOffset ...; if next == nil { break } }` shape for one `scrollAllPoints` call; its per-point body (observed/appeared bookkeeping, conditional write via `SetPayload`) becomes the callback body unchanged. **View:** full payload — the manifest walk decodes the whole payload (`payloadToMap`) exactly like DryRun and the sweep-mode pass below.

**Which existing tests pin these two (D-02):** `TestMigrateFullBacklogProjection` (:802), `TestMigrateDryRunAndManifestMutuallyExclusive` (:858), `TestMigrateManifestIntersection` (:921), `TestMigrateManifestSparedDeletedRecord` (:995), `TestMigrateManifestBacklogAppeared` (:1038), `TestMigrateDryRunWritesNothing` (:759).

---

### `internal/store/migrate.go` — default sweep-mode pass loop (:483-594)

**Analog:** `internal/store/revert.go`'s `revertWithSteps` pass loop (above) — mutual twins; migrate them together using the identical sentinel-error-for-early-pass-termination shape. `migrateBatch` (:22) stays the per-pass record cap; only the inner `ScrollAndOffset(..., Offset: nil, Limit: batch)` call is replaced by one bounded `s.scrollAllPoints` invocation per pass, same sentinel-error technique.

**View:** `s.fullView()` — the write path decodes the whole payload (`payloadToMap`) and computes `migrate.AddedKeys(original, current)`, so nothing less than full payload is legitimate here (D-04's projection carve-out does not apply to Migrate's own write path — only to spine.go's read-only scan/verify/purge and to the id-enumeration/preflight sites).

**Which existing tests pin this (D-02):** `TestMigrateHonorsCancel` (:721), `TestMigrateWritesOnlyAddedKeys` (:471), `TestMigrateRefusesNonAdditiveStep` (:365), `TestMigrateV0ToV1MintEndToEnd` (:523), `TestMigrateExistingShortIDPreserves` (:590), `TestMigrateOwnerlessRecordInvariant` (:676), `TestMigrateTracerLegacyRecordEndToEnd` (:119) — these are the ones most likely to notice a changed pass/page boundary; `TestBacklogFilterMatchesAbsentAndBelowTarget` (:208) pins the filter itself, unaffected by this migration.

---

### `internal/store/summarize.go` — `SummarizeMissing` (:145-182)

**Analog:** `internal/store/spine.go`'s `ScanSpine`, once migrated, is the closest same-shape sibling (exhaustive scroll, closure-scoped counters, no writes-per-page issue). The one wrinkle `ScanSpine` doesn't have: a mid-page early exit on `opts.Limit`.

**Today's loop** (`summarize.go:139-182`):
```go
var offset *qdrant.PointId
for {
	pts, next, serr := s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection, Filter: filter,
		Limit: qdrant.PtrOf(uint32(256)), Offset: offset,
		WithPayload: qdrant.NewWithPayload(true),
	})
	if serr != nil { return res, serr }
	for _, p := range pts {
		if opts.Limit > 0 && res.Scanned >= opts.Limit {
			return res, nil   // <-- mid-page early exit, becomes the sentinel case
		}
		res.Scanned++
		m := fromPayload(p.Id.GetUuid(), p.Payload)
		// age filter, shouldSummarize, DryRun, FillSummary, egress logging
	}
	if next == nil { return res, nil }
	offset = next
}
```
**Migration shape:** the `if opts.Limit > 0 && res.Scanned >= opts.Limit { return res, nil }` line is EXACTLY the early-termination case D-01 anticipates: inside the `scrollAllPoints` callback, return a sentinel `errSummarizeLimitReached`; the outer call does `if err := s.scrollAllPoints(...); err != nil && !errors.Is(err, errSummarizeLimitReached) { return res, err }` then falls through to `return res, nil` either way. Every other branch (age filter, `shouldSummarize`, `DryRun`, `FillSummary`, egress logging) moves into the callback body verbatim.

**View:** `SummarizeMissing` calls `fromPayload` (needs `Content` for `FillSummary`'s `summarize(ctx, m.Content)`) and `shouldSummarize(m, maxChars)` (reads `Summary`, likely `Content` length) — full payload is required; use `s.fullView()`, not a projection.

**Which existing tests pin this (D-02):** `TestSummarizeMissingFillsEmptyOnly` (:192), `TestSummarizeMissingDryRun` (:124), `TestSummarizeMissingStampsEgressAt` (:231), `TestSummarizeMissingEmitsEgressAuditLog` (:281), `TestFillSummarySkipsWhenNotEligible` (:180) — none of these currently seed enough records to force a second page, so the `Limit` mid-page early exit's sentinel path may be UNEXERCISED by the existing suite; flag as a D-03 real-Qdrant regression's job to prove specifically (seed a scope where `opts.Limit` cuts inside a byte-budget-limited page, not just inside the old fixed-256 page).

---

### `internal/store/store.go` — `Reindex` (:3449, own loop)

**Analog:** `internal/store/spine.go`'s `ScanSpine` (post-migration) for the walk shape; `reindexTargetContents` (its per-page `Get`, :3657-3683) is OUT OF SCOPE for the `scrollAllPoints` migration — `Get` is not in `recallEmissionMethods`' vocabulary (`Query`/`QueryBatch`/`Scroll`/`ScrollAndOffset`/`Count` only) and 03-INVENTORY classifies it as "rides along with Store.Reindex ... bounded by the calling page's batch (256)" — its own page size is already bounded by whatever `Reindex`'s own migrated loop requests per RPC, so no separate migration action is needed there; only the enclosing `Reindex` loop moves onto `scrollAllPoints`.

**Today's loop** (`store.go:3446-3465`):
```go
var offset *qdrant.PointId
for {
	var pts []*qdrant.RetrievedPoint
	var next *qdrant.PointId
	pts, next, err = s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
		CollectionName: source, Limit: qdrant.PtrOf(batch), Offset: offset,
		WithPayload: qdrant.NewWithPayload(true), WithVectors: qdrant.NewWithVectors(false),
	})
	if err != nil { return res, fmt.Errorf("reindex: scroll source: %w", err) }
	// Resume lookup: s.reindexTargetContents(ctx, opts.Target, pts) — one Get per page
	for _, p := range pts {
		// per-point embed/skip/upsert logic
	}
	if next == nil { break }  // (implicit; loop continues on next != nil)
	offset = next
}
```
**Migration wrinkle — `WithVectors(false)` is NOT part of `readView`:** `readView` (`boundedread.go:245-249`) only carries a payload `selector` and `maxRecordBytes`; it has no vectors field. `scrollAllPoints`'s own `ScrollPoints` construction (`spine.go:88-94`) does not set `WithVectors` at all, meaning it defaults to Qdrant's "no vectors" behavior only if that is the client's own zero-value default — confirm this against `qdrant-go-client`'s `ScrollPoints` zero value before assuming `scrollAllPoints` is a drop-in replacement; if vectors are returned by default, `scrollAllPoints`'s signature may need an explicit accommodation (Claude's Discretion — the CONTEXT does not resolve this, and it is the one structural difference between `Reindex`'s call and every other migrated site in this phase).

**Resume lookup composition:** `reindexTargetContents(ctx, opts.Target, pts)` needs the FULL page's `pts` slice at once (one `Get` per PAGE, not per record) — this is incompatible with `scrollAllPoints`'s per-RECORD callback shape unless the callback accumulates a page-sized batch itself and flushes it. Two migration shapes are viable: (a) accumulate points into a local slice inside the callback, flushing (calling `reindexTargetContents` + per-point logic) every `batch` records via the same sentinel-early-termination technique as `SummarizeMissing`/`revertWithSteps`'s passes, or (b) call `reindexTargetContents` per-record with a length-1 slice (loses the "one Get per page" batching REQUEST-COUNT property `store.go`'s own comment relies on: "One Get per page keeps the lookup O(pages), not O(points)"). Preserve O(pages): shape (a) is the one consistent with every other migrated sweep's closure-accumulator pattern in this phase.

**View:** full payload (`fromPayload` reads `Content`, `Tags`) — use `s.fullView()`.

**Which existing tests pin this (D-02):** `TestReindexMultiPageCursor` (:647) is the ONE test that already forces a second page — it is the closest thing to a pre-existing characterization of the cursor-boundary behavior this migration must preserve exactly. `TestReindexResumeSkipsUnchanged` (:362), `TestReindexResumeTags` (:425), `TestReindexResumeRestampsStaleIdentity` (:813) exercise the per-page `reindexTargetContents` batching this migration must not silently turn into a per-record `Get` (see wrinkle above — a regression here would be invisible to `TestReindexMultiPageCursor` since it doesn't count `Get` RPCs, only records/pages of results).

---

### `internal/store/store.go` — `NewQdrantClient` (:529-536) — the 64 MiB backstop (D-05)

**Analog:** itself — the function's OWN existing base-dial-option composition, which already appends two base options before the caller's:
```go
func NewQdrantClient(host string, port int, opts ...grpc.DialOption) (*qdrant.Client, error) {
	dialOpts := make([]grpc.DialOption, 0, 2+len(opts))
	dialOpts = append(dialOpts, grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	dialOpts = append(dialOpts, grpc.WithChainUnaryInterceptor(classifyResponseTooLarge))
	dialOpts = append(dialOpts, opts...)
	return qdrant.NewClient(&qdrant.Config{Host: host, Port: port, GrpcOptions: dialOpts})
}
```
**D-05's addition — one more base option, same position (before `opts...`):**
```go
dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(64<<20)))
```
Placing it BEFORE `opts...` (not after) is what makes `storetest.dialOptions`' own last-wins append (`storetest.go:265-279`, appends its `recvLimit` LAST) still win for every test dial — D-06 explicitly requires this ordering stay intact and documented, not given a behavioral test against gRPC (rule `m45p2b4bp7`). The doc comment reserving this exact addition already exists at `store.go:529-531`: "This adds no production read-bounding behavior on its own — the MaxCallRecvMsgSize backstop is a later requirement (REQ-recv-limit-backstop), set once the regression tests already pass without it" — that sentence itself needs updating once this phase lands it (it will otherwise read as still-future after the change).

**Constant naming:** no existing named constant for 64 MiB exists anywhere in the module — introduce one (e.g. `const productionRecvLimit = 64 << 20`) rather than an inline literal, mirroring `storetest.RecvLimit`'s own naming (`storetest.go:37-40ish`, a named constant with a doc comment, not a bare literal at the call site).

**Only ONE call site needs nothing else changed:** `internal/server/tools.go:226` (`storeFromConfig`, `qc, err := store.NewQdrantClient(host, port)`, no extra opts) — the backstop applies to it automatically since it lives inside `NewQdrantClient` itself, not at this call site. Do not add a second, redundant `grpc.MaxCallRecvMsgSize` option here — D-05 mandates "exactly ONE place."

---

### `internal/store/boundedread.go` — per-sweep projected views (D-04) and `unbudgetedView` deletion

**Analog:** the file's own `fullView`/`summaryView`/`keysView` constructor trio (:216-259) — every new constructor this phase adds follows their EXACT shape: a doc comment naming which fields the view is safe for and which it is NOT, a `readView{selector: ..., maxRecordBytes: ...}` literal, and (for a genuinely new ceiling) a small documented fixed constant sitting beside `keysRecordCeiling` (:161-171) rather than derived from `RecordCaps` — only `fullRecordCeiling`/`summaryRecordCeiling` derive from caps; a narrower-than-summary view with no capped field left to derive from gets a fixed allowance exactly like `keysRecordCeiling`.

**Deletion, once every non-test caller (`spine.go` x4, `revert.go` x1) is migrated** (`boundedread.go:260-266`):
```go
// unbudgetedView wraps sel with no byte-derived ceiling — today's
// count-only scrollAllPoints loop, kept byte-for-byte for callers this
// phase has not migrated (ScanSpine, EnumerateCitations, NearDuplicates' id
// enumeration, derivePurgeEligible, previewRevertWithSteps, and
// spine_test.go's snapshotCollection). Phase 5 replaces every call and
// deletes this constructor; its closing check is that no unbudgetedView(
// call remains.
func unbudgetedView(sel *qdrant.WithPayloadSelector) readView {
	return readView{selector: sel, maxRecordBytes: 0}
}
```
**Test-only callers that ALSO block deletion (found by direct read, not covered by inventory closing check (b) since that check excludes `*_test.go`):**
- `internal/store/spine_test.go:81` — `snapshotCollection`'s own `scrollAllPoints(ctx, nil, unbudgetedView(qdrant.NewWithPayload(true)), ...)`. Migrate to `s.fullView()` — it wants the whole payload for a digest, same as before.
- `internal/store/boundedread_test.go:86-91` — `sweepLimit(unbudgetedView(nil))` directly tests `sweepLimit`'s unbudgeted branch. Either construct the zero-value `readView{}` directly (its own `budgeted()` is already `false` for a zero `maxRecordBytes`) or delete/retarget the test if the unbudgeted branch of `sweepLimit` itself is being retired — Claude's Discretion, but the constructor's deletion cannot proceed while this call site still names `unbudgetedView` by that identifier.
- `internal/store/export_test.go:81-82` — the `UnbudgetedView` shim (`func UnbudgetedView(sel *qdrant.WithPayloadSelector) ReadView { return unbudgetedView(sel) }`) must be deleted in the SAME change, and its own consumer:
- `internal/store/orderedpage_oversized_test.go:654` — `store.UnbudgetedView(qdrant.NewWithPayload(true))` used as one `scrollOrderedPage` test-table row (`name: "unbudgeted view"`). This is Phase 4's OWN test, exercising `scrollOrderedPage` (not `scrollAllPoints`) with an unbudgeted view as one of its boundary cases — retarget this row to `store.FullView()` or remove the row if "an unbudgeted view reaching scrollOrderedPage" is no longer a reachable production shape after this phase (Claude's Discretion, but note it belongs to Phase 4's test file, so touching it here is a Phase-5-caused ripple, not scope creep).

---

### `internal/store/schemaversion_recallgate_test.go` — `operatorMigrationEmitters` reclassification

**Analog:** itself — the existing entries for `Store.Migrate`, `Store.SummarizeMissing`, `Store.revertWithSteps`, `Store.Reindex`, `Store.scrollAllPoints` (:519-568). The governing mechanism (`recallEmissionMethods`, :334-341) scans ONLY for direct `s.client.{Query,QueryBatch,Scroll,ScrollAndOffset,Count}(...)` calls textually inside each function — `s.scrollAllPoints(...)` calls do NOT themselves register the enclosing function as an emitter (confirmed: `ScanSpine`/`EnumerateCitations`/`derivePurgeEligible`/`previewRevertWithSteps` already call `scrollAllPoints` exclusively today and carry NO entry of their own in any of the three lists — only `Store.scrollAllPoints` itself is classified). This has a direct, mechanical consequence for two entries:

**`Store.SummarizeMissing`'s entry MUST BE DELETED** (not merely edited) once its own body no longer contains a direct `s.client.ScrollAndOffset(...)` call — after migration it calls only `s.scrollAllPoints(...)`, so `deriveRecallEmissionSites` (:637-648) will no longer produce `Store.SummarizeMissing` as a name at all, and `TestRecallEmissionSetIsCompleteAndClassified`'s "three-way classification of the remainder" subtest (:685-716, `assertNameSetEqual` against the union of all three lists vs. `derived`) will fail with `SummarizeMissing` as an "extra" entry if it is left in `operatorMigrationEmitters`. Delete this entry:
```go
{
	enclosingFunc: "Store.SummarizeMissing",
	justification: "Emits ScrollAndOffset (summarize.go:145). D-16 operator command (`engram summarize-missing`); same Phase 3 rationale.",
},
```

**`Store.Reindex`'s entry MUST ALSO BE DELETED**, for the identical reason: `Get` is not in `recallEmissionMethods`' vocabulary, so `reindexTargetContents`' unchanged `Get` call never registers anything, and once `Reindex`'s own body no longer directly calls `s.client.ScrollAndOffset`, `Store.Reindex` has NO remaining direct emission and drops out of the derived set entirely. Delete:
```go
{
	enclosingFunc: "Store.Reindex",
	justification: "Emits ScrollAndOffset (store.go:3199). D-16 operator command (`engram reindex`). ...",
},
```

**`Store.Migrate`'s entry SURVIVES but its justification must be edited** — `Migrate` keeps a DIRECT `s.client.Count(...)` call (the `opts.DryRun`/sweep-mode backlog counts), so it remains in the derived set via `Count` alone, never via `ScrollAndOffset` after migration. Edit the justification to drop the now-false "and ScrollAndOffset" claim:
```go
{
	enclosingFunc: "Store.Migrate",
	justification: "Emits Count (internal/store/migrate.go), against backlogFilter. As of Phase 5, its three former ScrollAndOffset loops (DryRun projection, Manifest-limited apply, default sweep-mode pass) all route through Store.scrollAllPoints instead (already classified below) — Migrate itself no longer emits ScrollAndOffset directly. D-16 operator command — Phase 3's migration sweep must be able to filter/count by schema_version to find its own backlog, so a blanket ban across every Qdrant query in this package would make Phase 3 unimplementable. backlogFilter is never reachable from any recallEntryPointSeeds member.",
},
```

**`Store.revertWithSteps`'s entry SURVIVES, same edit shape** (keeps its own `Count`, its `ScrollAndOffset` moves onto `scrollAllPoints`):
```go
{
	enclosingFunc: "Store.revertWithSteps",
	justification: "Emits Count (internal/store/revert.go), against aboveTargetFilter. As of Phase 5 its own pass-mode ScrollAndOffset call routes through Store.scrollAllPoints instead (already classified below). D-16 operator command (`engram migrate revert`); same Phase 3 rationale — the revert sweep must be able to filter/count by schema_version to find its own above-target backlog. Store.previewRevertWithSteps enumerates the SAME range but through Store.scrollAllPoints, so it needs no row of its own. Never reachable from any recallEntryPointSeeds member.",
},
```

**`Store.scrollAllPoints`'s entry SURVIVES and its justification should be widened** to name the new callers (it already anticipates this — "This entry buys something specific: if a future recall path ever routes through it..." is untouched by the widening):
```go
{
	enclosingFunc: "Store.scrollAllPoints",
	justification: "Emits ScrollAndOffset (spine.go:49). The package's ONE shared paginated whole-spine iterator, behind ScanSpine/EnumerateCitations/NearDuplicates/derivePurgeEligible/previewRevertWithSteps AND, as of Phase 5, Store.Migrate's three loops, Store.SummarizeMissing, Store.revertWithSteps's pass loop, and Store.Reindex — all operator-tier today. This entry buys something specific: if a future recall path ever routes through it, reachability pulls it into the reachable set, it stops matching this entry, and the suite goes RED.",
},
```

**Action checklist for this file, in ONE change per CONTEXT's "same change" rule:**
1. Delete the `Store.SummarizeMissing` and `Store.Reindex` entries from `operatorMigrationEmitters`.
2. Edit the `Store.Migrate`, `Store.revertWithSteps`, `Store.scrollAllPoints` justifications as above.
3. Re-run `TestRecallEmissionSetIsCompleteAndClassified` — both subtests (`reachable emission completeness` and `three-way classification of the remainder`) must pass; a leftover entry or a missing edit surfaces as a named "extra"/"missing" diff via `assertNameSetEqual` (:609-628), not a silent pass.
4. Double-check `foundScrollAllPointsRationale` (:733-737, requires the `Store.scrollAllPoints` entry's justification to still contain the substring `"reachable set"`) — the widened text above preserves that substring; do not drop it.

---

### New per-sweep real-Qdrant regressions (D-03)

**Analog:** `internal/store/listscheduled_oversized_test.go` and `internal/store/searchtwophase_oversized_test.go` (package `store_test`, dial via `storetest.Dial(t, storetest.RecvLimit)`, seed via `storetest.SeedOversized`), and Phase 4 PATTERNS.md's own minimal-shape template (reproduced here for direct reuse):
```go
package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)

func TestScanSpineBoundedOverGRPCLimit(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			c := storetest.Dial(t, storetest.RecvLimit)
			name := store.PrefixedTestCollection("oversized_scanspine_" + uuid.NewString())
			st := store.NewTestStore(t, c, name)
			ctx := context.Background()
			if err := st.EnsureCollection(ctx, 3); err != nil {
				t.Fatalf("EnsureCollection: %v", err)
			}
			t.Cleanup(func() {
				if err := c.DeleteCollection(ctx, name); err != nil {
					t.Errorf("DeleteCollection(%q): %v", name, err)
				}
			})
			fx := storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3}})
			res, err := st.ScanSpine(ctx, store.SpineScanOptions{Scope: fx.Scope})
			if err != nil {
				t.Fatalf("ScanSpine: %v (request shape exceeded the %d-byte named receive limit)", err, storetest.RecvLimit)
			}
			if res.Total != uint64(len(fx.IDs)) {
				t.Errorf("Total = %d, want %d", res.Total, len(fx.IDs))
			}
		})
	}
}
```
One test file per migrated command is the natural granularity given `storetest.Dial`/`SeedOversized`'s per-test container/collection lifecycle cost — `TestScanSpineBoundedOverGRPCLimit`, `TestEnumerateCitationsBoundedOverGRPCLimit`, `TestNearDuplicatesBoundedOverGRPCLimit`, `TestDerivePurgeEligibleBoundedOverGRPCLimit` (or one file covering all four `spine.go` sweeps), plus `TestMigrateBoundedOverGRPCLimit`, `TestRevertBoundedOverGRPCLimit`, `TestSummarizeMissingBoundedOverGRPCLimit`, `TestReindexBoundedOverGRPCLimit`. REQ-sweeps-bounded itself names five CLI commands (`engram migrate`, `migrate revert`, `summarize-missing`, `spine-review` scan/verify/purge as one command with three subcommands, `reindex`) — Claude's Discretion whether `spine-review`'s three subcommands share one test file or get three, but each of the SEVEN underlying Store methods this phase migrates needs its own assertion that a 256-record-per-page-exceeding scope still completes.

**Package requirement (memory `y02a9ft3gy`):** every new file here MUST be `package store_test` — `internal/store` (package `store`) cannot import `storetest` (import cycle). If a test needs an internal-only symbol not yet exposed, add a one-line shim to `export_test.go` following its existing doc-comment convention (e.g. `// ScanSpineOptions... ` — though `ScanSpine`/`SpineScanOptions` are already exported, so no new shim is needed for this specific test; `NearDuplicates`/`NearDuplicateOptions`, `PreviewPurge`/`ApplyPurge`/`PurgeOptions`, `Migrate`/`MigrateOptions`, `Revert`/`RevertOptions`, `SummarizeMissing`/`SummarizeOptions`, `Reindex`/`ReindexOptions` are likewise already exported — this phase's new regressions should not need ANY new export_test.go shim for the sweep entry points themselves).

---

### New D-06 test — backstop pass-through only, never gRPC enforcement

**Analog:** `internal/store/storetest/storetest.go:239-279` (`Dial`/`dialOptions`) for the "assert the dial option was PASSED, never that gRPC enforces it" idiom, and `internal/store/qdrant_client_convergence_test.go` for the "gate the CONSTRUCTION SITE, not third-party behavior" idiom (rule `m45p2b4bp7` governs both).

**What to assert (per D-06, explicitly NOT a behavioral gRPC test):**
1. `store.NewQdrantClient(host, port)` (no caller opts) puts a `grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(64<<20))`-equivalent option into its dial options — inspect the constructed `*qdrant.Client`'s config or dial-option count/shape, NOT a real oversized RPC's outcome. If the client exposes no introspectable dial-option list, assert this at the SOURCE level instead: a small `go/ast`-free unit test constructing `NewQdrantClient`'s `dialOpts` slice length and its last-appended-before-caller-opts position, or (simplest, matching D-06's own text) a test that calls `NewQdrantClient(host, port, someProbeOption)` and confirms `someProbeOption` still lands AFTER the backstop in the slice order (whitebox, in-package `store` test — this one is legitimately `package store`, not `store_test`, since it inspects `NewQdrantClient`'s own internals/order rather than dialing real Qdrant).
2. `storetest.Dial(t, storetest.RecvLimit)` still ends up with the 4 MiB `storetest.RecvLimit` in effect, not 64 MiB — i.e., prove the "append LAST, last-wins" ordering documented in `dialOptions`' comment (`storetest.go:265-269`) survives `NewQdrantClient`'s own new base option. This is most directly proven by the EXISTING oversized regressions continuing to correctly reject/bound at 4 MiB after the backstop lands (D-02's "the regression tests already pass without it" framing implies they must ALSO still correctly exercise the 4 MiB ceiling WITH it) — a dedicated new test is only needed if no existing test would catch a 64 MiB leak into `storetest.Dial`'s effective limit. Never write a test asserting a response of N bytes fails at the boundary — that is the exact third-party-behavior assertion D-06 prohibits.

---

## Shared Patterns

### License header (every new Go file)
**Source:** every file read in this map, e.g. `internal/store/spine.go:1-2`, `internal/store/boundedread.go:1-2`.
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
```
**Apply to:** every new `_test.go` file this phase adds. Never above `.planning/**` frontmatter (not applicable here — no `.planning/*.md` file is being created besides this PATTERNS.md itself, which per the orchestrator's own instruction gets NO SPDX header).

### `package store_test` for every storetest-importing test
**Source:** `internal/store/listscheduled_oversized_test.go:1-20`, memory `y02a9ft3gy`.
**Apply to:** every new D-03 real-Qdrant regression. The D-06 whitebox dial-order test is the ONE exception in this phase — it inspects `NewQdrantClient`'s own internals and does not dial real Qdrant, so it may legitimately live in `package store`.

### `scrollAllPoints`'s callback-error-as-early-termination contract
**Source:** `internal/store/spine.go:88-94` (the loop body: `for _, p := range pts { if ferr := fn(p); ferr != nil { return ferr } }`).
**Apply to:** `revertWithSteps`'s pass loop, `Migrate`'s sweep-mode pass loop, `SummarizeMissing`'s `opts.Limit` early exit, and (if shape (a) is chosen) `Reindex`'s per-page `reindexTargetContents` batching. In every case: define a package-level sentinel `error` value, return it from the callback to stop `scrollAllPoints` early, and unwrap it with `errors.Is` at the call site to distinguish "intentional early stop" from "genuine failure." Never reuse `ErrResponseTooLarge` or any Phase 3 sentinel for this — each sweep's early-termination reason is its own, distinct sentinel.

### Per-sweep `readView` projection, never one shared "sweepView"
**Source:** `internal/store/boundedread.go:216-259` (`fullView`/`summaryView`/`keysView`), extended by D-04.
**Apply to:** every `spine.go` scan/verify/purge site and `previewRevertWithSteps`. Each new constructor: (1) a doc comment naming exactly which fields the caller reads and which it deliberately omits, (2) a `readView{selector: qdrant.NewWithPayloadInclude(...)/NewWithPayloadExclude(...), maxRecordBytes: <ceiling>}` literal, (3) either reuse of `fullRecordCeiling`/`summaryRecordCeiling` when the projection matches `fullView`/`summaryView` exactly (as `derivePurgeEligible` can, via `s.summaryView()` directly — no new constructor needed there) or a small fixed constant beside `keysRecordCeiling` when it does not. A sweep that omits a field it actually reads is a bug its own existing suite must catch (D-04's own discipline) — do not add a NEW test for this; rely on the existing named suites listed per-site above.

### `unbudgetedView` deletion is a whole-repo grep, not a single-file edit
**Source:** `internal/store/boundedread.go:260-266` (self-documented deletion condition), cross-checked against direct reads of `spine_test.go:81`, `boundedread_test.go:86-91`, `export_test.go:81-82`, `orderedpage_oversized_test.go:654`.
**Apply to:** whichever plan performs the deletion — it must touch ALL FIVE production call sites (`spine.go` x4, `revert.go` x1) AND the four test-file callers above in the SAME change, or the build breaks. Closing check (b) (`rg -o 's[.]scrollAllPoints[(].*unbudgetedView[(]' internal/store --glob '!*_test.go' | wc -l`) only proves the FIVE production sites are clear — it is silent about the four test callers, which is why they are called out here explicitly rather than left to be discovered by a failed `go build`.

### Name-keyed AST gate updates ride in the SAME change as the call-site move
**Source:** `internal/store/schemaversion_recallgate_test.go` (`operatorMigrationEmitters`), CONTEXT.md's own "Established Patterns" note.
**Apply to:** every migrated sweep. A plan that migrates `Store.SummarizeMissing`'s loop WITHOUT deleting its `operatorMigrationEmitters` entry in the same commit leaves `TestRecallEmissionSetIsCompleteAndClassified` RED between the two — per CLAUDE.md's quality-gate discipline (`task` = lint + test, run before considering work done), this must never be split across two plans/commits.

### `NewQdrantClient`'s base-option append order — one place, before caller opts
**Source:** `internal/store/store.go:534-538` (existing `otelgrpc`/`classifyResponseTooLarge` append), `internal/store/storetest/storetest.go:265-269` (`dialOptions`' documented last-wins append).
**Apply to:** the D-05 backstop's own append — it MUST land in `dialOpts` BEFORE `opts...` in `NewQdrantClient`, exactly like the two existing base options, so every caller (test or production) that appends its own `MaxCallRecvMsgSize` afterward still wins. Getting this ordering backward silently raises every existing 4 MiB-bounded regression test's effective ceiling to 64 MiB, which would make those tests stop proving what their names claim without any of them going red (D-06's own concern is exactly this failure mode).

## No Analog Found

None. Every touched site in this phase either has a strong same-file/same-shape analog (`ScanSpine`/`EnumerateCitations`/`derivePurgeEligible` are already one-line-away from correct; `Migrate` and `revertWithSteps`'s pass loops are mutual analogs of each other) or reuses a Phase 3/4 primitive (`scrollAllPoints`, `fullView`/`summaryView`/`keysView`, `storetest.Dial`/`SeedOversized`) directly. The one open engineering question flagged above (`Reindex`'s `WithVectors(false)` interacting with `scrollAllPoints`'s fixed `ScrollPoints` construction, and its per-page `Get` batching) is a design decision for the plan/executor, not a missing analog — `scrollAllPoints`'s own source is the analog, and its gap for this one caller is called out explicitly under `Reindex` above.

## Metadata

**Analog search scope:** `internal/store/{spine,revert,migrate,summarize,store,boundedread}.go` (targeted `sed -n` reads by line range, no full-file loads where >400 lines), `internal/store/{schemaversion_recallgate_test,qdrant_client_convergence_test,export_test,boundedread_test,spine_test,orderedpage_oversized_test}.go` (targeted sections), `internal/store/storetest/storetest.go` (full read), `internal/server/tools.go` (targeted section around `storeFromConfig`), Phase 4's `04-PATTERNS.md` (full read, reused directly for the D-05/List-loop precedent and the oversized-test template), function-name inventories (`rg -n "^func Test"`) across `migrate_test.go`, `revert_test.go`, `summarize_test.go`, `reindex_test.go`, `spine_test.go`.
**Files scanned:** 16 non-test + test files read in targeted line ranges or in full; zero full reads exceeded 400 lines per call.
**Pattern extraction date:** 2026-09-20
