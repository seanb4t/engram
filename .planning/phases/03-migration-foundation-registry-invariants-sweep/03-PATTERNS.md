# Phase 3: Migration Foundation (Registry, Invariants & Sweep) - Pattern Map

**Mapped:** 2026-08-13
**Files analyzed:** 4 (2 new/grown in `internal/migrate`, 2 new in `internal/store`)
**Analogs found:** 4 / 4

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/migrate/migrate.go` (grows: `Step`, `NewStep`, `Reversibility`, `Reversible`, `Irreversible`, `Validate`, `Registry`) | model / leaf-package type | transform (pure, in-process) | `internal/openaiurl/openaiurl.go` (leaf shape) + `internal/store/store.go:2919-2924` `RemapFrom` (panic-on-invalid-arg ctor) | exact (leaf shape) / role-match (ctor idiom, cross-package) |
| `internal/migrate/migrate_test.go` (grows: invariant + fixture-step tests) | test | transform | `internal/store/store_test.go:2791-2798` `TestRemapFromPanicsOnEmptyValue`; `internal/store/schemaversion_recallgate_test.go` (set-equality/coverage-guard style) | role-match |
| `internal/store/store.go` → new `Store.Migrate` sweep method | service / sweep method | batch (event-driven convergence loop) | `internal/store/store.go:2741-2797` `BackfillShortIDs` (closest: single-loop scroll+SetPayload+cursor) with deliberate divergence per `Store.Supersede`'s re-derivation doctrine (`store.go:129-136`, `:2218-2293`) | role-match, explicit divergence noted |
| new `internal/store/migrate_*_test.go` (sweep tests: partial-failure resume, no-lock convergence) | test (integration, real Qdrant) | event-driven / concurrency | `internal/store/schemaversion_recallgate_test.go:885-936` (interceptor/dial seam) + `internal/store/store_test.go:3484-3547` `TestSupersedeConcurrent` (goroutine-pair concurrency shape) | exact (interceptor seam) / role-match (concurrency shape) |

## Pattern Assignments

### `internal/migrate/migrate.go` (grows into `step.go`-shaped additions — leaf package, no new file required by CONTEXT, but RESEARCH.md's `Recommended Project Structure` splits into `step.go`/`registry.go`; either is fine, this map treats them as one logical unit)

**Analog 1 — leaf-package shape:** `internal/openaiurl/openaiurl.go:1-33`

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package openaiurl builds OpenAI-compatible endpoint URLs ...
// It is ... a stdlib-only leaf package deliberately so either lane can
// import it with zero cycle risk.
package openaiurl

import "strings"

// Join resolves ... (full doc comment explaining behavior + edge cases)
func Join(baseURL, suffix string) string { ... }
```

Copy: package-doc comment stating *why* it's a leaf (stdlib-only, one-way import direction), one blank import line only if actually needed (Phase 3 additions likely still need zero imports — `fmt`/`errors` only inside `_test.go` or, if `Validate` needs `fmt.Errorf`, that becomes the package's first import; do not add it speculatively), and doc comments on every exported symbol that state the invariant being enforced, not just what the code does.

**Already established in `internal/migrate/migrate.go:1-46` (Phase 2, preserve verbatim):**
```go
package migrate

// Version is a record's schema-version discriminator ...
type Version int

// CurrentVersion is the schema version produced by applying every
// registered migration step. It is 0 in this phase, for three reasons: ...
// Raising this constant is a Phase 3/4 action taken together with
// registering the step that defines the new version — never a standalone
// bump.
const CurrentVersion Version = 0
```
Phase 3 MUST NOT touch `CurrentVersion`'s value (stays `0`) and MUST NOT remove/alter this doc comment's reasoning — only append new types below it.

**Analog 2 — panic-on-invalid-argument constructor:** `internal/store/store.go:2919-2924` (`RemapFrom`), proven by `internal/store/store_test.go:2791-2798` (`TestRemapFromPanicsOnEmptyValue`)

```go
// store.go:2919-2924
func RemapFrom(from string) OwnerRemapSource {
	if from == "" {
		panic("store.RemapFrom: from value must be non-empty")
	}
	return remapFrom{from: from}
}
```
```go
// store_test.go:2791-2798
func TestRemapFromPanicsOnEmptyValue(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("RemapFrom(\"\") did not panic; an empty From must fail at construction, not silently collapse into a different source")
		}
	}()
	RemapFrom("")
}
```

Copy this exact shape twice:
1. `Irreversible(reason string) Reversibility` — panics with a `"migrate: Irreversible requires a non-empty reason"`-style message (mirror `"store.RemapFrom: ... must be non-empty"` wording convention: `<pkg>.<Func>: <field> ... non-empty`).
2. `NewStep(...)` — nil-checks `rev` (and `apply`) the same way, per RESEARCH.md Pitfall 1 (sealing does not block a literal `nil` interface value — this is a REQUIRED addition, not optional).

Test shape to copy per constructor: `TestIrreversiblePanicsOnEmptyReason`, `TestNewStepPanicsOnNilReversibility` — same `defer func() { if recover() == nil { t.Fatal(...) } }()` wrapper, one direct call, no table needed (single-invariant tests, matching `TestRemapFromPanicsOnEmptyValue`'s minimalism).

**Sealed-interface + registry patterns:** no existing analog in this codebase (first sealed interface with unexported-marker-method pattern in the repo) — RESEARCH.md's Code Examples (`Reversibility`, `NewStep`, `Validate`, additive-only diff test) are the load-bearing reference here since no in-repo precedent exists; those excerpts are already concrete and should be followed near-verbatim.

---

### `internal/migrate/migrate_test.go` (grows with invariant tests)

**Analog — non-zero-coverage guard + set-equality style:** `internal/store/schemaversion_recallgate_test.go` establishes this repo's idiom for "a scan/diff test must assert it actually exercised something and must use set equality, not subset/count." Concretely:

- `schemaversion_recallgate_test.go:174` — the walker's default case `t.Fatalf(...)` on an unrecognized shape (fail loudly on drift, don't silently skip) — copy this "fail loud on an unrecognized/uncovered case" reflex for `Validate`'s tests and the additive-only diff test's fixture loop.
- The `recallCapture`/`walkCondition` machinery in that file demonstrates exhaustive type-switch-with-fatal-default over Qdrant condition variants — same defensive idiom to apply if `Validate` or the additive-only test iterates over `Step` internals via any interface type-switch.
- RESEARCH.md's own `TestAdditiveOnlyKeySetDiff` code example (lines 641-701 of 03-RESEARCH.md) already implements D-05's `if len(fixtures) == 0 { t.Fatal(...) }` guard — copy that literally; it is this repo's `x6v6qxqd6f`-lesson idiom (memory `x6v6qxqd6f`: `len(findings) > 0` is not a real assertion) applied here.

Set-equality idiom to reuse (no existing named helper in the codebase for this — check for `slices.Equal`/sorted-comparison usage before hand-rolling): prefer converting both sides to `map[string]bool` (as RESEARCH.md's example already does with `keysOf`/`setDiff`/`setEqual`) over `reflect.DeepEqual` on unsorted slices.

---

### `internal/store/store.go` — new `Store.Migrate` sweep method

**Primary analog (closest structural match):** `BackfillShortIDs`, `internal/store/store.go:2741-2797`

```go
func (s *Store) BackfillShortIDs(ctx context.Context, dryRun bool) (n uint64, err error) {
	ctx, span := tracer.Start(ctx, "store.BackfillShortIDs")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "BackfillShortIDs", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(n)))
		}
	}()

	if dryRun { ... }

	seen := map[string]struct{}{}
	var offset *qdrant.PointId
	for {
		pts, next, serr := s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
			CollectionName: s.collection,
			Filter:         missingShortIDFilter(),
			Limit:          qdrant.PtrOf(uint32(reindexBatch)),
			Offset:         offset,
			WithPayload:    qdrant.NewWithPayload(false),
		})
		if serr != nil { err = serr; return n, err }
		for _, p := range pts {
			// per-point SetPayload, err -> immediate return
		}
		if next == nil { return n, nil }
		offset = next
	}
}
```

**Copy:** the telemetry/span wrapper (`tracer.Start` + deferred `telemetry.RecordStoreOp` + span error/attribute recording) — every sweep method in this file uses this exact boilerplate; `Store.Migrate` must match it (`"store.Migrate"` span name, `"Migrate"` telemetry op name, `attribute.Int64("engram.result_count", int64(migrated))` on success).

**Deliberate divergence (per D-07/D-08, RESEARCH.md Pattern 4):** do NOT thread `offset` across the OUTER loop like `BackfillShortIDs` does across its whole run. `Store.Migrate`'s outer loop re-derives the backlog filter fresh on every pass (`for { backlog := scroll(fresh filter); if empty break }`) — `offset` may only be used for paging WITHIN one pass if the backlog exceeds one page, never persisted between passes. This is the one genuinely new sweep shape in the file; state this divergence explicitly in the plan/code comment so a future reader doesn't "fix" it to look like `BackfillShortIDs`.

**Reconciliation doctrine to cite in the doc comment (copy the reasoning, not code):** `store.go:127-140` (`qdrantPayloadOpBatchSize` doc comment) and `:2218-2223` (`Supersede`'s doc comment on batch non-atomicity) — both already document qdrant/qdrant#9371 for this exact codebase; `Store.Migrate`'s doc comment should reference these line ranges rather than re-deriving the citation.

**Secondary analogs for filter/dry-run/cursor shape:**
- `PruneExpired` (`store.go:2594-2627`) — Count-then-mutate dry-run split; simpler (single Count, not scroll), useful for `Store.Migrate`'s per-pass "how many still need it" if a Count-based backlog-size telemetry attribute is added.
- `RemapOwner` (`store.go:2950-2997`) — `ValidateOwnerRemap`-before-any-Qdrant-call pattern (validate args before the network hop) and the `dryRun` early-return-after-Count shape.
- `Reindex` (`store.go:3133+`, `ReindexOptions` at `:3019-3053`) — the `Batch uint32 // scroll page size (0 → a sane default)` idiom (`store.go:3019`) is the exact shape to copy for `Store.Migrate`'s `chunkSize` parameter per RESEARCH.md Open Question 1's recommendation (parameter, not hardcoded constant, 0 → sane default).

**Backlog filter — copy the `Should`+`IsEmpty` OR-shape, not a `Must`-only Range:**
Existing `IsEmpty` usage to mirror exactly: `store.go:1091,1097,1191,1195,1328,1333,1563,1568` (`superseded_by`/`archived_at` recall gates, all `f.Must = append(f.Must, qdrant.NewIsEmpty(...))` — but those are single-condition Must additions, NOT the OR-with-Range shape needed here). Existing `Range` usage: `store.go:1012,1016,1510,1511` and `spine.go:82` — e.g.:
```go
// spine.go:82 (expiredFilter)
qdrant.NewRange("not_after", &qdrant.Range{Lt: qdrant.PtrOf(float64(before.Unix()))}),
```
and the active-window OR-shape already in this file for a *different* absent-vs-below-target problem:
```go
// store.go:1012-1013 (activeWindowConditions, not-before half)
qdrant.NewRange("not_before", &qdrant.Range{Lte: qdrant.PtrOf(sec)}),
qdrant.NewIsEmpty("not_before"),
```
This `Range(...), IsEmpty(...)` pairing inside one `Should`/OR group is the exact precedent for `backlogFilter`'s `Should: [Range(schema_version, Lt: target), IsEmpty(schema_version)]` shape RESEARCH.md specifies — `store.go:1004-1017`'s `activeWindowConditions` is the closest in-repo analog for "absent key must count as matching, a Range condition alone won't see it," predating this phase's own discovery of the same trap for `schema_version`. Read `store.go:1004-1020` in full before writing `backlogFilter` — it is the same shape already solved once in this file.

---

### new `internal/store/migrate_*_test.go` (sweep tests)

**Analog 1 — gRPC interceptor test seam (D-10), copy verbatim structure:** `internal/store/schemaversion_recallgate_test.go:885-936`

```go
// recallCaptureInterceptor, :885-901
func recallCaptureInterceptor(t *testing.T, capture *recallCapture) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		switch r := req.(type) {
		case *qdrant.QueryPoints:
			capture.record("Query", r.GetFilter())
		// ...
		default:
			if fc, ok := req.(filterCarryingRequest); ok && fc.GetFilter() != nil {
				t.Fatalf(...)
			}
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// dialCapturingTestClient, :908-936
func dialCapturingTestClient(t *testing.T, capture *recallCapture) *qdrant.Client {
	t.Helper()
	if testQdrantAddr == "" { /* requireQdrant / skip logic, unchanged */ }
	host, portStr, err := net.SplitHostPort(testQdrantAddr)
	// ...
	c, err := qdrant.NewClient(&qdrant.Config{
		Host: host, Port: port,
		GrpcOptions: []grpc.DialOption{grpc.WithUnaryInterceptor(recallCaptureInterceptor(t, capture))},
	})
	// ...
	return c
}
```

**Write the fault-injecting sibling exactly per RESEARCH.md's Code Examples § "Fault-injection interceptor"** (already a full, ready-to-use skeleton: `dialFaultInjectingTestClient`, type-switch on `*qdrant.SetPayloadPoints`, count-and-fail-Nth via `sync.Mutex`, `status.Error(codes.Unavailable, "injected: forced Nth SetPayload failure")`). Reuse `dialCapturingTestClient`'s exact skip/`testQdrantAddr`/`net.SplitHostPort` boilerplate (lines 909-927) unchanged — only the `GrpcOptions` interceptor value differs.

**Analog 2 — real-Qdrant concurrency test shape:** `TestSupersedeConcurrent`, `internal/store/store_test.go:3484-3547`

```go
var wg sync.WaitGroup
errs := make([]error, 2)
wg.Add(2)
go func() { defer wg.Done(); errs[0] = s.Supersede(...) }()
go func() { defer wg.Done(); errs[1] = s.Supersede(...) }()
wg.Wait()

successes, conflicts := 0, 0
for _, err := range errs {
	switch {
	case err == nil: successes++
	case errors.Is(err, ErrAlreadySuperseded): conflicts++
	default: t.Fatalf("unexpected ... error: %v", err)
	}
}
if successes != 1 || conflicts != 1 { t.Fatalf(...) }
```

For SC5's mid-sweep-write test (D-08): copy the `sync.WaitGroup` two-goroutine interleave shape, but the two goroutines here are asymmetric ("run `Store.Migrate`" vs. "write a new record mid-sweep") rather than `TestSupersedeConcurrent`'s symmetric pair — `TestSupersedeVsUpdateConcurrent` (`store_test.go:3559+`) is the closer analog for an *asymmetric* two-actor race (one `Supersede`, one `Update`, racing over the same target with a pre-race snapshot fetched via `FetchForUpdate` before the race starts) — mirror its comment style documenting *why* the race is deterministic enough to assert on (e.g., a synchronization point via a channel or a small sleep-free gate, not a raw goroutine race hoping for a particular interleave). Follow its top-of-function doc comment convention explaining the race's precondition and postcondition in prose before the code.

**Do NOT copy:** the `s.setPayloadKeys` test-only hook field pattern (`store_test.go:~4640-4720`, referenced in RESEARCH.md's "Alternatives Considered" table) — explicitly rejected by D-10 for `Migrate`; that hook exists for `Supersede`'s tests only and must not be extended to `Migrate`.

## Shared Patterns

### Telemetry/span wrapper (every sweep method)
**Source:** `internal/store/store.go:2741-2757` (`BackfillShortIDs`), identical shape at `PruneExpired` (:2594-2607), `RemapOwner` (:2950-2963)
**Apply to:** `Store.Migrate`
```go
ctx, span := tracer.Start(ctx, "store.Migrate")
defer span.End()
start := time.Now()
defer func() {
	telemetry.RecordStoreOp(ctx, "Migrate", start, err)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetAttributes(attribute.Int64("engram.result_count", int64(migrated)))
	}
}()
```

### Batch-non-atomicity doc-comment citation
**Source:** `internal/store/store.go:127-140` (`qdrantPayloadOpBatchSize` const doc) and `:2218-2223` (`Supersede` doc comment)
**Apply to:** `Store.Migrate`'s doc comment and `backlogFilter`'s doc comment — cite these line ranges rather than re-deriving the qdrant/qdrant#9371 explanation from scratch.

### Absent-key-must-match OR-filter shape
**Source:** `internal/store/store.go:1004-1020` (`activeWindowConditions`, `not_before`/`not_after` Range+IsEmpty pairing)
**Apply to:** `backlogFilter(target)` in `internal/store/store.go` (new)
```go
qdrant.NewRange("not_before", &qdrant.Range{Lte: qdrant.PtrOf(sec)}),
qdrant.NewIsEmpty("not_before"),
```
This is the in-repo precedent for RESEARCH.md's mandated `Should: [Range(schema_version, Lt: target), IsEmpty(schema_version)]` shape — same absent-key trap, already solved once in this file for a different field.

### Panic-on-invalid-construction-argument
**Source:** `internal/store/store.go:2919-2924` (`RemapFrom`), tested at `internal/store/store_test.go:2791-2798`
**Apply to:** `migrate.Irreversible(reason string)` (panic on `reason == ""`) and `migrate.NewStep(...)` (panic on `rev == nil` / `apply == nil`)

## No Analog Found

| File/Pattern | Role | Data Flow | Reason |
|---|---|---|---|
| Sealed interface (`interface{ isReversibility() }`) with exactly-two-constructors | model (leaf) | transform | First use of this pattern in the codebase — no existing sealed interface to copy from. RESEARCH.md's Pattern 1/2 code examples are the reference instead (already concrete, cite external source: rodusek.com Go closed-interfaces post). |
| Outer re-derive-every-pass sweep loop with no persisted cursor | service (sweep) | batch | All four existing sweep methods (`BackfillShortIDs`, `RemapOwner`, `PruneExpired`, `Reindex`) use single-pass-with-cursor or Count-then-mutate; none re-derive their backlog across an outer loop. `Store.Migrate` is the first of this shape — RESEARCH.md's Pattern 4 code example is the reference. |

## Metadata

**Analog search scope:** `internal/migrate/`, `internal/store/store.go`, `internal/store/store_test.go`, `internal/store/schemaversion_recallgate_test.go`, `internal/store/spine.go`, `internal/store/reindex_test.go`, `internal/openaiurl/openaiurl.go`
**Files scanned:** 8 read directly (2 fully, 6 via targeted grep + offset/limit reads)
**Pattern extraction date:** 2026-08-13
