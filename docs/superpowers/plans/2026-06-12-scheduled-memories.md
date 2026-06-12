<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Scheduled / Future Memories Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a temporal validity window (`not_before` deferred-reveal + `not_after` expiry) to engram memories so agents can capture future/scheduled memories, gated purely at recall time with no scheduler.

**Architecture:** Two nullable `*time.Time` fields on `Memory`, stored as epoch-second integers in the Qdrant payload only when set. The recall paths (`Search`, `List`) gain a server-side active-window filter (`NewRange` + `NewIsEmpty`) sourced from an injectable `Store.now` clock; by-id reads stay ungated. Two new MCP tools (`schedule_memory`, `list_scheduled`) and one operator CLI command (`prune-expired`) round it out.

**Tech Stack:** Go, Qdrant (`github.com/qdrant/go-client` v1.18.2), `modelcontextprotocol/go-sdk`, cobra CLI, testcontainers-go for store/server tests.

**Spec:** `docs/superpowers/specs/2026-06-12-scheduled-memories-design.md`
**Design bead:** engram-rb2

---

## Conventions for every task

- **VCS:** jj-colocated. Commit with `jj commit -m "<conventional-commit>"` (see `references/vcs-preamble.md`). One logical change per commit.
- **License header:** every new `.go` file MUST start with the two-line SPDX header (copy from any existing file):
  ```go
  // SPDX-License-Identifier: Apache-2.0
  // Copyright 2026 Sean Brandt
  ```
- **Run store/server tests** against a live Qdrant: `export MEM_QDRANT_TEST_ADDR=localhost:6334` (or have Docker running for testcontainers). Tests `t.Skip` without one.
- **Format/lint gate before each commit:** `task fmt && task lint` must be clean.

---

## Task 1: Add `NotBefore` / `NotAfter` fields and payload round-trip

**Files:**
- Modify: `internal/store/store.go` (`Memory` struct ~line 66; `payload` ~line 122; `fromPayload` ~line 199)
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go`:

```go
func TestPayloadRoundTripWindow(t *testing.T) {
	nb := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	na := time.Date(2031, 6, 7, 8, 9, 10, 0, time.UTC)
	m := Memory{
		ID: "22222222-2222-2222-2222-222222222222", Content: "windowed",
		Scope: "win-test:project:x", Owner: "sub-A", CreatedAt: time.Now().UTC(),
		NotBefore: &nb, NotAfter: &na,
	}
	got := fromPayload(m.ID, qdrant.NewValueMap(payload(m)))
	if got.NotBefore == nil || !got.NotBefore.Equal(nb) {
		t.Errorf("NotBefore round-trip: got %v want %v", got.NotBefore, nb)
	}
	if got.NotAfter == nil || !got.NotAfter.Equal(na) {
		t.Errorf("NotAfter round-trip: got %v want %v", got.NotAfter, na)
	}
	// Unwindowed record: keys absent, pointers stay nil.
	plain := fromPayload("id", qdrant.NewValueMap(payload(Memory{ID: "id", Content: "x"})))
	if plain.NotBefore != nil || plain.NotAfter != nil {
		t.Errorf("unwindowed: want nil pointers, got nb=%v na=%v", plain.NotBefore, plain.NotAfter)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestPayloadRoundTripWindow -v`
Expected: COMPILE FAIL — `m.NotBefore undefined (type Memory has no field NotBefore)`.

- [ ] **Step 3: Add the struct fields**

In `internal/store/store.go`, in the `Memory` struct immediately after the `CreatedAt time.Time` field (line ~67):

```go
	CreatedAt time.Time `json:"created_at"`
	// NotBefore gates deferred reveal: the record is hidden from recall until
	// now >= NotBefore. nil = always active (no lower gate).
	NotBefore *time.Time `json:"not_before,omitempty"`
	// NotAfter gates expiry: the record drops out of recall once now >= NotAfter.
	// nil = never expires.
	NotAfter *time.Time `json:"not_after,omitempty"`
```

- [ ] **Step 4: Write the window keys in `payload`**

In `payload` (line ~122), after the `p := map[string]any{...}` literal closes (after line ~141, before the `if m.Category == "discovery"` block), add:

```go
	if m.NotBefore != nil {
		p["not_before"] = m.NotBefore.Unix()
	}
	if m.NotAfter != nil {
		p["not_after"] = m.NotAfter.Unix()
	}
```

- [ ] **Step 5: Read the window keys in `fromPayload`**

In `fromPayload` (line ~199), after the `created_at` block (after line ~203), add:

```go
	if v, ok := p["not_before"]; ok {
		t := time.Unix(v.GetIntegerValue(), 0).UTC()
		m.NotBefore = &t
	}
	if v, ok := p["not_after"]; ok {
		t := time.Unix(v.GetIntegerValue(), 0).UTC()
		m.NotAfter = &t
	}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestPayloadRoundTripWindow -v`
Expected: PASS.

- [ ] **Step 7: Commit**

`task fmt && task lint && jj commit -m "feat(store): add not_before/not_after window fields to Memory"`

---

## Task 2: Injectable `Store.now` clock via functional option

**Files:**
- Modify: `internal/store/store.go` (`Store` struct ~line 84; `New` ~line 90)
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go`:

```go
func TestWithClockOverridesNow(t *testing.T) {
	fixed := time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)
	s := New(nil, "c", WithClock(func() time.Time { return fixed }))
	if got := s.now(); !got.Equal(fixed) {
		t.Errorf("WithClock: got %v want %v", got, fixed)
	}
	// Default clock is time.Now (non-zero, recent).
	d := New(nil, "c")
	if d.now().IsZero() {
		t.Error("default clock returned zero time")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestWithClockOverridesNow -v`
Expected: COMPILE FAIL — `undefined: WithClock` and `s.now undefined`.

- [ ] **Step 3: Add the `now` field, `Option` type, and `WithClock`**

In `internal/store/store.go`, change the `Store` struct (line ~84) to add the clock field:

```go
// Store persists and queries memories in a Qdrant collection.
type Store struct {
	client     *qdrant.Client
	collection string
	now        func() time.Time
}

// Option configures a Store at construction.
type Option func(*Store)

// WithClock overrides the time source the recall window gate reads. Defaults to
// time.Now. Tests inject a fixed clock to exercise active/scheduled/expired
// boundaries deterministically.
func WithClock(fn func() time.Time) Option {
	return func(s *Store) { s.now = fn }
}
```

Then change `New` (line ~90) to accept options and default the clock:

```go
// New returns a Store backed by the given Qdrant client and collection.
func New(c *qdrant.Client, collection string, opts ...Option) *Store {
	s := &Store{client: c, collection: collection, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestWithClockOverridesNow -v`
Expected: PASS. (No existing caller breaks — `New(c, coll)` still compiles via the variadic.)

- [ ] **Step 5: Commit**

`task fmt && task lint && jj commit -m "feat(store): injectable clock via WithClock option"`

---

## Task 3: Active-window recall gate in `Search` and `List`

**Files:**
- Modify: `internal/store/store.go` (`Search` ~line 319; `List` ~line 454; add helper near `ownerScopeFilter` ~line 296)
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go`:

```go
func TestRecallWindowGate(t *testing.T) {
	s := testStore(t)
	fixed := time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixed } // white-box override
	ctx := context.Background()
	scope := "gate-test:project:x"
	subj := Authenticated("sub-A")
	past := fixed.Add(-24 * time.Hour)
	future := fixed.Add(24 * time.Hour)

	mk := func(id string, nb, na *time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A",
			CreatedAt: fixed, NotBefore: nb, NotAfter: na}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, subj)) })
	}
	mkVis := func(id, owner, vis string, nb, na *time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: owner, Visibility: vis,
			CreatedAt: fixed, NotBefore: nb, NotAfter: na}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, Authenticated(owner))) })
	}
	mk("a0000000-0000-0000-0000-000000000001", nil, nil)        // unwindowed -> visible
	mk("a0000000-0000-0000-0000-000000000002", &past, &future)  // active -> visible
	mk("a0000000-0000-0000-0000-000000000003", &future, nil)    // scheduled -> hidden
	mk("a0000000-0000-0000-0000-000000000004", nil, &past)      // expired -> hidden
	// sub-B's SHARED but scheduled record: must stay hidden from sub-A until active
	// (the window gate composes with the owner/shared authz envelope).
	mkVis("a0000000-0000-0000-0000-000000000005", "sub-B", "shared", &future, nil)

	hits, err := s.Search(ctx, scope, subj, []float32{0.1, 0.2, 0.3}, 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 2 {
		t.Errorf("Search: got %d want 2 (unwindowed+active; shared-but-scheduled stays hidden)", len(hits))
	}
	for _, h := range hits {
		if h.ID == "a0000000-0000-0000-0000-000000000005" {
			t.Error("Search leaked sub-B's scheduled shared record to sub-A before it is active")
		}
	}
	lst, _, _, err := s.List(ctx, scope, subj, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lst) != 2 {
		t.Errorf("List: got %d want 2 (unwindowed+active)", len(lst))
	}
	// By-id is ungated: the scheduled record is still fetchable directly.
	if _, err := s.GetReadable(ctx, "a0000000-0000-0000-0000-000000000003", subj); err != nil {
		t.Errorf("GetReadable on scheduled record should be ungated, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestRecallWindowGate -v`
Expected: FAIL — `Search: got 4 want 2` (no gate yet; all four returned).

- [ ] **Step 3: Add the `activeWindowConditions` helper**

In `internal/store/store.go`, after `ownerScopeFilter` (line ~296), add:

```go
// activeWindowConditions gates recall to records whose validity window is open
// at now: (not_before absent OR <= now) AND (not_after absent OR > now). Stored
// window keys are epoch-second integers; the Range bound is *float64 (Qdrant's
// Range field type). Records with no window match via NewIsEmpty — unchanged
// behavior for every pre-feature record. not_after is exclusive (expires AT it).
func activeWindowConditions(now time.Time) []*qdrant.Condition {
	sec := qdrant.PtrOf(float64(now.Unix()))
	return []*qdrant.Condition{
		qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
			qdrant.NewRange("not_before", &qdrant.Range{Lte: sec}),
			qdrant.NewIsEmpty("not_before"),
		}}),
		qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
			qdrant.NewRange("not_after", &qdrant.Range{Gt: sec}),
			qdrant.NewIsEmpty("not_after"),
		}}),
	}
}
```

- [ ] **Step 4: Apply the gate in `Search`**

In `Search` (line ~319), replace the `res, err := s.client.Query(...)` call so the filter gains the window conditions:

```go
	f := s.ownerScopeFilter(scope, subj)
	f.Must = append(f.Must, activeWindowConditions(s.now())...)
	res, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collection, Query: qdrant.NewQuery(vec...),
		Filter: f, Limit: qdrant.PtrOf(k), WithPayload: qdrant.NewWithPayload(true),
	})
```

- [ ] **Step 5: Apply the gate in `List`**

In `List` (line ~454), replace the `Filter: listFilter(scope, subj, opts),` line in the `Scroll` call with a pre-built filter that appends the window conditions:

```go
	f := listFilter(scope, subj, opts)
	f.Must = append(f.Must, activeWindowConditions(s.now())...)
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         f,
		Limit:          qdrant.PtrOf(uint32(scanCap)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
```

(Appending to `f.Must` is correct for both `listFilter` return shapes — the default `&qdrant.Filter{Must: must}` and the early-return `private` case `&qdrant.Filter{Must: must, MustNot: ...}` both populate `Must`.)

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestRecallWindowGate -v`
Expected: PASS.

- [ ] **Step 7: Run the full store suite to confirm no regression**

Run: `go test ./internal/store/ -v`
Expected: PASS (existing isolation tests use unwindowed records → all match via `NewIsEmpty`).

- [ ] **Step 8: Commit**

`task fmt && task lint && jj commit -m "feat(store): gate Search/List recall to the active validity window"`

---

## Task 4: `Store.ListScheduled` — the inverse-window management view

**Files:**
- Modify: `internal/store/store.go` (add method + state constants after `List` ~line 479)
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go`:

```go
func TestListScheduledStates(t *testing.T) {
	s := testStore(t)
	fixed := time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixed }
	ctx := context.Background()
	scope := "sched-test:project:x"
	subj := Authenticated("sub-A")
	past := fixed.Add(-24 * time.Hour)
	future := fixed.Add(24 * time.Hour)

	mk := func(id string, nb, na *time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A",
			CreatedAt: fixed, NotBefore: nb, NotAfter: na}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, subj)) })
	}
	mk("b0000000-0000-0000-0000-000000000001", &future, nil)   // scheduled
	mk("b0000000-0000-0000-0000-000000000002", nil, &past)     // expired
	mk("b0000000-0000-0000-0000-000000000003", &past, &future) // active -> never listed

	sched, err := s.ListScheduled(ctx, scope, subj, ScheduledPending, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("scheduled: %v", err)
	}
	if len(sched) != 1 || sched[0].ID != "b0000000-0000-0000-0000-000000000001" {
		t.Errorf("ScheduledPending: got %d want 1 (the future record)", len(sched))
	}
	exp, _ := s.ListScheduled(ctx, scope, subj, ScheduledExpired, ListOptions{Limit: 10})
	if len(exp) != 1 || exp[0].ID != "b0000000-0000-0000-0000-000000000002" {
		t.Errorf("ScheduledExpired: got %d want 1 (the past record)", len(exp))
	}
	all, _ := s.ListScheduled(ctx, scope, subj, ScheduledAll, ListOptions{Limit: 10})
	if len(all) != 2 {
		t.Errorf("ScheduledAll: got %d want 2 (scheduled+expired, never active)", len(all))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestListScheduledStates -v`
Expected: COMPILE FAIL — `undefined: ScheduledPending` / `s.ListScheduled undefined`.

- [ ] **Step 3: Implement `ListScheduled` and the state constants**

In `internal/store/store.go`, after `List` (line ~479), add:

```go
// ScheduledState selects which hidden-by-the-recall-gate records ListScheduled
// returns. Active (currently-valid) windowed records are never returned here —
// they surface through normal Search/List.
type ScheduledState string

const (
	ScheduledPending ScheduledState = "scheduled" // now < not_before (not yet active)
	ScheduledExpired ScheduledState = "expired"   // now >= not_after (already lapsed)
	ScheduledAll     ScheduledState = "all"       // union of pending and expired
)

// scheduledStateCondition returns the inverse-window clause for a state. now is
// epoch seconds as *float64 (Qdrant Range field type).
func scheduledStateCondition(state ScheduledState, now time.Time) *qdrant.Condition {
	sec := qdrant.PtrOf(float64(now.Unix()))
	pending := qdrant.NewRange("not_before", &qdrant.Range{Gt: sec})
	expired := qdrant.NewRange("not_after", &qdrant.Range{Lte: sec})
	switch state {
	case ScheduledExpired:
		return expired
	case ScheduledAll:
		return qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{pending, expired}})
	default: // ScheduledPending
		return pending
	}
}

// ListScheduled returns the caller's windowed records that the recall gate is
// hiding, for management (review/reschedule/delete). It mirrors List's scope +
// owner authz envelope but applies the INVERSE temporal clause; it does not
// reuse List (whose gate would exclude exactly these records). CreatedAt-desc,
// bounded by the same scanCap as List.
func (s *Store) ListScheduled(ctx context.Context, scope string, subj Subject, state ScheduledState, opts ListOptions) (items []Memory, err error) {
	ctx, span := tracer.Start(ctx, "store.ListScheduled", trace.WithAttributes(
		attribute.String("engram.scope", scope),
		attribute.String("engram.owner", ownerOf(subj)),
		attribute.String("engram.scheduled_state", string(state)),
	))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "ListScheduled", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int("engram.result_count", len(items)))
		}
	}()

	const scanCap = 1000
	f := &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		ownerOrSharedCondition(subj),
		scheduledStateCondition(state, s.now()),
	}}
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection, Filter: f,
		Limit: qdrant.PtrOf(uint32(scanCap)), WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	all := make([]Memory, 0, len(pts))
	for _, p := range pts {
		all = append(all, fromPayload(p.Id.GetUuid(), p.Payload))
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	if opts.Limit > 0 && uint64(len(all)) > opts.Limit {
		all = all[:opts.Limit]
	}
	return all, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestListScheduledStates -v`
Expected: PASS.

- [ ] **Step 5: Commit**

`task fmt && task lint && jj commit -m "feat(store): ListScheduled inverse-window management view"`

---

## Task 5: `Store.PruneExpired` — operator filter-delete

**Files:**
- Modify: `internal/store/store.go` (add method near `DeleteAll` ~line 820)
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go`:

```go
func TestPruneExpired(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "prune-test:project:x"
	subj := Authenticated("sub-A")
	now := time.Now().UTC()
	old := now.Add(-48 * time.Hour)
	future := now.Add(48 * time.Hour)

	mk := func(id string, na *time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A", CreatedAt: now, NotAfter: na}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, subj)) })
	}
	mk("c0000000-0000-0000-0000-000000000001", &old)    // expired -> pruned
	mk("c0000000-0000-0000-0000-000000000002", &future) // not expired -> kept
	mk("c0000000-0000-0000-0000-000000000003", nil)     // no window -> kept

	n, err := s.PruneExpired(ctx, now)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 1 {
		t.Errorf("PruneExpired: deleted %d want 1", n)
	}
	if _, err := s.Get(ctx, "c0000000-0000-0000-0000-000000000001"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired record should be gone, got %v", err)
	}
	if _, err := s.Get(ctx, "c0000000-0000-0000-0000-000000000002"); err != nil {
		t.Errorf("future record should survive, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestPruneExpired -v`
Expected: COMPILE FAIL — `s.PruneExpired undefined`.

- [ ] **Step 3: Implement `PruneExpired`**

In `internal/store/store.go`, after `DeleteAll` (line ~820), add:

```go
// PruneExpired deletes every record whose not_after is strictly before the given
// instant — an operator/admin sweep run from the CLI across the WHOLE collection
// (no subject authz; it is not on behalf of a caller). Records without a
// not_after key are never matched. Returns the number deleted (counted before
// the delete, since Qdrant's delete response carries no count).
func (s *Store) PruneExpired(ctx context.Context, before time.Time) (deleted uint64, err error) {
	ctx, span := tracer.Start(ctx, "store.PruneExpired",
		trace.WithAttributes(attribute.Int64("engram.before", before.Unix())))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "PruneExpired", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	f := &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewRange("not_after", &qdrant.Range{Lt: qdrant.PtrOf(float64(before.Unix()))}),
	}}
	n, err := s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: s.collection, Filter: f, Exact: qdrant.PtrOf(true),
	})
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}
	if _, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelectorFilter(f),
	}); err != nil {
		return 0, err
	}
	return n, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestPruneExpired -v`
Expected: PASS.

- [ ] **Step 5: Commit**

`task fmt && task lint && jj commit -m "feat(store): PruneExpired operator filter-delete"`

---

## Task 6: `schedule_memory` MCP tool

**Files:**
- Modify: `internal/server/tools.go` (add `scheduleArgs` + validation near `storeArgs` ~line 141; handler near `storeMemory` ~line 279; registration near line 444)
- Test: `internal/server/tools_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/server/tools_test.go`:

```go
func TestScheduleMemoryValidation(t *testing.T) {
	d := testDeps(t) // skips if no Qdrant
	ctx := authedContext(t, "sub-A")
	base := scheduleArgs{Content: "do X next week", Scope: "sched:project:x",
		Source: "user-said", Category: "decision"}

	// No window at all -> rejected.
	if _, err := d.scheduleMemory(ctx, base); err == nil {
		t.Error("missing window: want error, got nil")
	}
	// not_after already in the past -> rejected.
	past := base
	past.NotAfter = "2000-01-01T00:00:00Z"
	if _, err := d.scheduleMemory(ctx, past); err == nil {
		t.Error("past not_after: want error, got nil")
	}
	// Inverted window (not_before >= not_after) -> rejected.
	inv := base
	inv.NotBefore = "2031-01-01T00:00:00Z"
	inv.NotAfter = "2030-01-01T00:00:00Z"
	if _, err := d.scheduleMemory(ctx, inv); err == nil {
		t.Error("inverted window: want error, got nil")
	}
	// discovery category -> rejected.
	disc := base
	disc.Category = "discovery"
	disc.NotBefore = "2030-01-01T00:00:00Z"
	if _, err := d.scheduleMemory(ctx, disc); err == nil {
		t.Error("discovery category: want error, got nil")
	}
	// Valid future-scheduled memory -> stored, hidden from normal recall.
	ok := base
	ok.NotBefore = "2030-01-01T00:00:00Z"
	id, err := d.scheduleMemory(ctx, ok)
	if err != nil {
		t.Fatalf("valid schedule: %v", err)
	}
	t.Cleanup(func() { _ = d.st.Delete(context.Background(), id, store.Authenticated("sub-A")) })
	hits, _ := d.listMemory(ctx, listArgs{Scope: "sched:project:x"})
	for _, h := range hits {
		if h.ID == id {
			t.Error("future-scheduled memory leaked into normal list_memory")
		}
	}
}
```

> Helpers used (already defined in `tools_test.go`): `testDeps(t)` returns
> `*deps` (skips if no Qdrant); `authedContext(t, sub)` returns a context
> carrying a validated TokenInfo with the given sub.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestScheduleMemoryValidation -v`
Expected: COMPILE FAIL — `undefined: scheduleArgs` / `d.scheduleMemory undefined`.

- [ ] **Step 3: Add `scheduleArgs` and validation**

In `internal/server/tools.go`, after the `storeArgs` struct (line ~141), add:

```go
type scheduleArgs struct {
	Content   string   `json:"content" jsonschema:"the memory text to persist"`
	Scope     string   `json:"scope" jsonschema:"run:tier:repo, e.g. eval-2026-05:project:selfhosted-cluster"`
	Source    string   `json:"source" jsonschema:"user-said or agent-inferred"`
	Category  string   `json:"category" jsonschema:"decision|preference|convention|gotcha"`
	Tags      []string `json:"tags,omitempty"`
	Repo      string   `json:"repo,omitempty"`
	Workspace string   `json:"workspace,omitempty"`
	Worktree  string   `json:"worktree_path,omitempty"`
	BaseDir   string   `json:"base_dir,omitempty"`
	NotBefore string   `json:"not_before,omitempty" jsonschema:"RFC3339; hide from recall until this time"`
	NotAfter  string   `json:"not_after,omitempty" jsonschema:"RFC3339; drop from recall at this time"`
}

// parseWindow validates and parses the schedule_memory temporal window. At least
// one bound is required; not_after must be in the future and after not_before.
func parseWindow(a scheduleArgs, now time.Time) (nb, na *time.Time, err error) {
	if a.NotBefore == "" && a.NotAfter == "" {
		return nil, nil, fmt.Errorf("schedule_memory requires not_before and/or not_after (use store_memory for unscheduled records)")
	}
	if a.Category == "discovery" {
		return nil, nil, fmt.Errorf("discovery is not schedulable; use store_discovery")
	}
	if a.NotBefore != "" {
		t, perr := time.Parse(time.RFC3339, a.NotBefore)
		if perr != nil {
			return nil, nil, fmt.Errorf("not_before: %w", perr)
		}
		nb = &t
	}
	if a.NotAfter != "" {
		t, perr := time.Parse(time.RFC3339, a.NotAfter)
		if perr != nil {
			return nil, nil, fmt.Errorf("not_after: %w", perr)
		}
		if !t.After(now) {
			return nil, nil, fmt.Errorf("not_after %s is not in the future", a.NotAfter)
		}
		na = &t
	}
	if nb != nil && na != nil && !nb.Before(*na) {
		return nil, nil, fmt.Errorf("not_before must be strictly before not_after")
	}
	return nb, na, nil
}
```

- [ ] **Step 4: Add the `scheduleMemory` handler**

In `internal/server/tools.go`, after `storeMemory` (line ~279), add:

```go
func (d *deps) scheduleMemory(ctx context.Context, a scheduleArgs) (string, error) {
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return "", err
	}
	nb, na, err := parseWindow(a, time.Now().UTC())
	if err != nil {
		return "", err
	}
	vec, err := d.em.Embed(ctx, a.Content)
	if err != nil {
		return "", err
	}
	m := store.Memory{
		ID:        uuid.NewString(),
		Content:   a.Content,
		Scope:     a.Scope,
		Repo:      a.Repo,
		Workspace: a.Workspace,
		Worktree:  a.Worktree,
		BaseDir:   a.BaseDir,
		Source:    a.Source,
		Category:  a.Category,
		Tags:      a.Tags,
		Actor:     actorFromContext(ctx),
		Owner:     subj.Owner(),
		CreatedAt: time.Now().UTC(),
		NotBefore: nb,
		NotAfter:  na,
	}
	return m.ID, d.st.Upsert(ctx, m, vec)
}
```

- [ ] **Step 5: Register the tool**

In `Register` (line ~444), after the `store_memory` `mcp.AddTool(...)` block, add:

```go
	mcp.AddTool(s, &mcp.Tool{Name: "schedule_memory", Description: "Persist a memory with a validity window (not_before defers recall; not_after expires it). At least one bound (RFC3339) is required; use store_memory for unscheduled records."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a scheduleArgs) (*mcp.CallToolResult, any, error) {
			id, err := d.scheduleMemory(ctx, a)
			return textResult(fmt.Sprintf("scheduled %s", id)), map[string]string{"id": id}, err
		})
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestScheduleMemoryValidation -v`
Expected: PASS.

- [ ] **Step 7: Extend the schema-panic guard**

In `internal/server/tools_test.go`, in `TestToolArgSchemasDoNotPanic` (the `check(...)` series ending ~line 87), add a `check` for the new tool so schema generation is guarded (this test exists because `AddTool` panicked at startup in a past version):

```go
	check("schedule_memory", func(s *mcp.Server) {
		mcp.AddTool(s, &mcp.Tool{Name: "schedule_memory", Description: "x"}, func(context.Context, *mcp.CallToolRequest, scheduleArgs) (*mcp.CallToolResult, any, error) {
			return noop()
		})
	})
```

Run: `go test ./internal/server/ -run TestToolArgSchemasDoNotPanic -v`
Expected: PASS.

- [ ] **Step 8: Commit**

`task fmt && task lint && jj commit -m "feat(server): schedule_memory tool with validity-window validation"`

---

## Task 7: `list_scheduled` MCP tool

**Files:**
- Modify: `internal/server/tools.go` (add `listScheduledArgs` near `listArgs` ~line 152; handler near `listMemory` ~line 349; registration near line 456)
- Test: `internal/server/tools_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/server/tools_test.go`:

```go
func TestListScheduledTool(t *testing.T) {
	d := testDeps(t)
	ctx := authedContext(t, "sub-A")
	// A far-future scheduled memory is hidden from normal recall but shows in list_scheduled.
	id, err := d.scheduleMemory(ctx, scheduleArgs{Content: "future", Scope: "ls:project:x",
		Source: "user-said", Category: "decision", NotBefore: "2099-01-01T00:00:00Z"})
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}
	t.Cleanup(func() { _ = d.st.Delete(context.Background(), id, store.Authenticated("sub-A")) })

	got, err := d.listScheduled(ctx, listScheduledArgs{Scope: "ls:project:x"}) // default state=scheduled
	if err != nil {
		t.Fatalf("list_scheduled: %v", err)
	}
	found := false
	for _, m := range got {
		if m.ID == id {
			found = true
		}
	}
	if !found {
		t.Error("list_scheduled (default scheduled) did not return the future memory")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestListScheduledTool -v`
Expected: COMPILE FAIL — `undefined: listScheduledArgs` / `d.listScheduled undefined`.

- [ ] **Step 3: Add `listScheduledArgs` and the handler**

In `internal/server/tools.go`, after `listArgs` (line ~152), add:

```go
type listScheduledArgs struct {
	Scope string `json:"scope" jsonschema:"the scope to list scheduled/expired memories from"`
	State string `json:"state,omitempty" jsonschema:"scheduled (default, not yet active) | expired | all"`
	Limit uint64 `json:"limit,omitempty" jsonschema:"max memories to return (default 20)"`
}
```

After `listMemory` (line ~349), add:

```go
func (d *deps) listScheduled(ctx context.Context, a listScheduledArgs) ([]store.Memory, error) {
	if a.Limit == 0 {
		a.Limit = 20
	}
	state := store.ScheduledPending
	switch a.State {
	case "", "scheduled":
		state = store.ScheduledPending
	case "expired":
		state = store.ScheduledExpired
	case "all":
		state = store.ScheduledAll
	default:
		return nil, fmt.Errorf("state must be one of scheduled|expired|all, got %q", a.State)
	}
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return d.st.ListScheduled(ctx, a.Scope, subj, state, store.ListOptions{Limit: a.Limit})
}
```

- [ ] **Step 4: Register the tool**

In `Register` (line ~456), after the `list_memory` `mcp.AddTool(...)` block, add:

```go
	mcp.AddTool(s, &mcp.Tool{Name: "list_scheduled", Description: "List your windowed memories the recall gate is hiding: state=scheduled (not yet active, default) | expired | all. Active memories surface via list_memory/search_memory."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a listScheduledArgs) (*mcp.CallToolResult, any, error) {
			mems, err := d.listScheduled(ctx, a)
			return textResult(fmt.Sprintf("%d scheduled", len(mems))), map[string]any{"memories": mems}, err
		})
```

- [ ] **Step 5: Extend the schema-panic guard**

In `internal/server/tools_test.go`, in `TestToolArgSchemasDoNotPanic`, add:

```go
	check("list_scheduled", func(s *mcp.Server) {
		mcp.AddTool(s, &mcp.Tool{Name: "list_scheduled", Description: "x"}, func(context.Context, *mcp.CallToolRequest, listScheduledArgs) (*mcp.CallToolResult, any, error) {
			return noop()
		})
	})
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/server/ -run 'TestListScheduledTool|TestToolArgSchemasDoNotPanic' -v`
Expected: PASS.

- [ ] **Step 7: Commit**

`task fmt && task lint && jj commit -m "feat(server): list_scheduled tool for hidden windowed memories"`

---

## Task 8: `engram prune-expired` CLI command

**Files:**
- Create: `cmd/engram/prune.go`
- Test: covered by Task 5's `TestPruneExpired` (store layer). CLI wiring is validated by build + `engram prune-expired --help`.

- [ ] **Step 1: Create the command file**

Create `cmd/engram/prune.go` (note the SPDX header):

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/server"
)

var (
	pruneOlderThan time.Duration
	pruneTimeout   time.Duration
)

// pruneExpiredCmd deletes memories whose not_after has lapsed by at least
// --older-than. Operator-run reclamation; recall already hides expired records
// at read time, so this only reclaims storage. Collection-wide, no per-caller authz.
var pruneExpiredCmd = &cobra.Command{
	Use:   "prune-expired",
	Short: "Delete memories whose validity window (not_after) has lapsed",
	RunE: func(cmd *cobra.Command, _ []string) error {
		st, err := server.StoreFromEnv()
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if pruneTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, pruneTimeout)
			defer cancel()
		}
		before := time.Now().UTC().Add(-pruneOlderThan)
		n, err := st.PruneExpired(ctx, before)
		if err != nil {
			return err
		}
		cmd.Printf("pruned %d expired record(s) (not_after < %s)\n", n, before.Format(time.RFC3339))
		return nil
	},
}

func init() {
	pruneExpiredCmd.Flags().DurationVar(&pruneOlderThan, "older-than", 0,
		"grace period: only prune records whose not_after lapsed at least this long ago (0 = any past not_after)")
	pruneExpiredCmd.Flags().DurationVar(&pruneTimeout, "timeout", 5*time.Minute,
		"max wall-clock for the sweep (0 disables); also cancellable via Ctrl-C")
	rootCmd.AddCommand(pruneExpiredCmd)
}
```

- [ ] **Step 2: Verify it builds and registers**

Run: `go build ./... && go run ./cmd/engram prune-expired --help`
Expected: build succeeds; help shows `--older-than` and `--timeout` flags.

- [ ] **Step 3: Run the store prune test (the behavioral gate)**

Run: `go test ./internal/store/ -run TestPruneExpired -v`
Expected: PASS (already green from Task 5; confirms the method the CLI calls).

- [ ] **Step 4: Commit**

`task fmt && task lint && jj commit -m "feat(cli): engram prune-expired sweep command"`

---

## Task 9: Documentation — memory contract + docs-site

**Files:**
- Modify: `CLAUDE.md` ("Memory contract (stable)" section)
- Modify: `docs-site/src/content/docs/reference/tools.md` (tool summary table ~line 13 + per-tool `##` sections)

- [ ] **Step 1: Update the CLAUDE.md memory contract**

In `CLAUDE.md`, in the "Memory contract (stable)" section, add `schedule_memory` and `list_scheduled` to the tool list and append a sentence documenting the window:

```markdown
Scheduled tools: `schedule_memory` stores a memory with a temporal validity
window — `not_before` (RFC3339; deferred reveal: hidden from recall until then)
and/or `not_after` (RFC3339; expiry: dropped from recall at then). `list_scheduled`
surfaces windowed records the recall gate is hiding (`state` = `scheduled` default
| `expired` | `all`); active windowed records surface normally via
`search_memory`/`list_memory`. Recall is gated; fetch-by-id (`get_memory`) is not.
Operators reclaim lapsed records with `engram prune-expired [--older-than DUR]`.
```

- [ ] **Step 2: Update the docs-site tool reference**

In `docs-site/src/content/docs/reference/tools.md`:

1. Add two rows to the "Tool summary" table (after the `delete_all` row, line ~21):

```markdown
| `schedule_memory` | Persist a memory with a validity window (deferred reveal / expiry) |
| `list_scheduled` | List windowed memories the recall gate is hiding |
```

2. Add per-tool `##` sections following the `## store_memory` format (argument table + "Returns" line). For `schedule_memory`, list the same args as `store_memory` plus `not_before` / `not_after` (string, no, RFC3339). For `list_scheduled`, args `scope` (yes), `state` (no — `scheduled` default | `expired` | `all`), `limit` (no). Note in prose that `schedule_memory` requires at least one bound, that active windowed records surface via `list_memory`/`search_memory` (not `list_scheduled`), and that operators reclaim lapsed records with the `engram prune-expired` CLI command.

- [ ] **Step 3: License/lint check on Markdown**

Run: `task license:check && task lint`
Expected: PASS (Markdown files carry the SPDX header; rumdl clean).

- [ ] **Step 4: Commit**

`jj commit -m "docs: document schedule_memory, list_scheduled, and prune-expired"`

---

## Final verification

- [ ] **Full suite:** `task` (lint + test). Expected: all green with `MEM_QDRANT_TEST_ADDR` set (or Docker for testcontainers).
- [ ] **Buf drift (no proto touched, but CI checks it):** `task proto:lint` — expected clean.
- [ ] **Tool surface sanity:** start the server locally and confirm `schedule_memory` and `list_scheduled` appear in the MCP tool list.
<!-- adr-capture: sha256=ca41fe19317d460e; session=cli; ts=2026-06-12T13:19:12Z; adrs=engram-y1g,engram-90w,engram-ufz,engram-c0m -->
