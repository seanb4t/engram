// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"pgregory.net/rapid"
)

// modelRecord is what the memory contract says about one record. The store
// never repairs a link to a deleted record, so a deleted record keeps its
// entry with deleted set, and the model makes no claim about its contents.
type modelRecord struct {
	owner        string
	shared       bool
	supersededBy string
	archived     bool
	notBefore    *time.Time
	notAfter     *time.Time
	deleted      bool
}

func (r modelRecord) expired(now time.Time) bool {
	return r.notAfter != nil && !r.notAfter.After(now)
}

func (r modelRecord) scheduled(now time.Time) bool {
	return !r.expired(now) && r.notBefore != nil && r.notBefore.After(now)
}

func (r modelRecord) recallVisible(principal string, now time.Time) bool {
	return !r.deleted && (r.owner == principal || r.shared) &&
		!r.archived && r.supersededBy == "" && !r.expired(now) && !r.scheduled(now)
}

// storeModel holds the model and the store it is compared with. Each rapid
// check uses its own scope on one collection, so checks never see each
// other's records.
type storeModel struct {
	ctx        context.Context
	s          *Store
	scope      string
	idPrefix   uint32
	now        time.Time
	records    map[string]*modelRecord
	order      []string
	principals []string
	// touched is the record the last action named, if any. Its
	// readability is checked by id after the action.
	touched string
}

func (m *storeModel) nextID() string {
	return fmt.Sprintf("%08x-0000-4000-8000-%012x", m.idPrefix, len(m.order)+1)
}

func (m *storeModel) anyID(t *rapid.T, label string) string {
	if len(m.order) == 0 || rapid.IntRange(0, 9).Draw(t, label+"_unknown") == 0 {
		return fmt.Sprintf("%08x-ffff-4000-8000-000000000000", m.idPrefix)
	}
	return rapid.SampledFrom(m.order).Draw(t, label)
}

func (m *storeModel) writable(id, principal string) bool {
	r, ok := m.records[id]
	return ok && !r.deleted && r.owner == principal
}

func (m *storeModel) put(t *rapid.T, principal string, nb, na *time.Time) {
	id := m.nextID()
	shared := rapid.Bool().Draw(t, "shared")
	mem := Memory{
		ID: id, Content: "c " + id, Scope: m.scope, Category: "decision",
		Owner: principal, Actor: principal, CreatedAt: m.now, NotBefore: nb, NotAfter: na,
	}
	if shared {
		mem.Visibility = visibilityShared
	}
	if err := m.s.Upsert(m.ctx, mem, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	m.records[id] = &modelRecord{owner: principal, shared: shared, notBefore: nb, notAfter: na}
	m.order = append(m.order, id)
	m.touched = id
}

func (m *storeModel) offset(t *rapid.T, label string) time.Time {
	return m.now.Add(time.Duration(rapid.IntRange(-5, 30).Draw(t, label)) * time.Second)
}

func (m *storeModel) actions() map[string]func(*rapid.T) {
	return map[string]func(*rapid.T){
		"store": func(t *rapid.T) {
			m.put(t, rapid.SampledFrom(m.principals).Draw(t, "principal"), nil, nil)
		},
		"schedule": func(t *rapid.T) {
			var nb, na *time.Time
			if rapid.Bool().Draw(t, "has_not_before") {
				v := m.offset(t, "not_before")
				nb = &v
			}
			if nb == nil || rapid.Bool().Draw(t, "has_not_after") {
				v := m.now.Add(time.Duration(rapid.IntRange(1, 30).Draw(t, "not_after")) * time.Second)
				if nb != nil && !v.After(*nb) {
					v = nb.Add(time.Second)
				}
				na = &v
			}
			m.put(t, rapid.SampledFrom(m.principals).Draw(t, "principal"), nb, na)
		},
		"supersede": func(t *rapid.T) {
			principal := rapid.SampledFrom(m.principals).Draw(t, "principal")
			targets := []string{m.anyID(t, "target")}
			if rapid.Bool().Draw(t, "second_target") {
				targets = append(targets, m.anyID(t, "target2"))
			}
			ok := true
			for _, id := range targets {
				ok = ok && m.writable(id, principal) && m.records[id].supersededBy == ""
			}
			id := m.nextID()
			mem := Memory{ID: id, Content: "c " + id, Scope: m.scope, Category: "decision",
				Owner: principal, Actor: principal, CreatedAt: m.now, Supersedes: slices.Compact(slices.Sorted(slices.Values(targets)))}
			err := m.s.Supersede(m.ctx, mem, []float32{0.1, 0.2, 0.3}, targets, Authenticated(principal))
			if (err == nil) != ok {
				t.Fatalf("Supersede(%v) by %s: err = %v, model says ok = %v", targets, principal, err, ok)
			}
			m.touched = targets[0]
			if !ok {
				return
			}
			m.touched = id
			m.records[id] = &modelRecord{owner: principal}
			m.order = append(m.order, id)
			for _, target := range targets {
				m.records[target].supersededBy = id
			}
		},
		"archive": func(t *rapid.T) {
			principal := rapid.SampledFrom(m.principals).Draw(t, "principal")
			id := m.anyID(t, "id")
			m.touched = id
			res, err := m.s.ArchiveAs(m.ctx, id, Authenticated(principal))
			m.checkArchiveOutcome(t, "ArchiveAs", id, principal, res, err, true)
		},
		"restore": func(t *rapid.T) {
			principal := rapid.SampledFrom(m.principals).Draw(t, "principal")
			id := m.anyID(t, "id")
			m.touched = id
			res, err := m.s.RestoreAs(m.ctx, id, Authenticated(principal))
			m.checkArchiveOutcome(t, "RestoreAs", id, principal, res, err, false)
		},
		"delete": func(t *rapid.T) {
			principal := rapid.SampledFrom(m.principals).Draw(t, "principal")
			id := m.anyID(t, "id")
			m.touched = id
			err := m.s.Delete(m.ctx, id, Authenticated(principal))
			if ok := m.writable(id, principal); (err == nil) != ok || (!ok && !errors.Is(err, ErrNotFound)) {
				t.Fatalf("Delete(%s) by %s: err = %v, model says ok = %v", id, principal, err, ok)
			}
			if err == nil {
				m.records[id].deleted = true
			}
		},
		"set_visibility": func(t *rapid.T) {
			principal := rapid.SampledFrom(m.principals).Draw(t, "principal")
			id := m.anyID(t, "id")
			shared := rapid.Bool().Draw(t, "shared")
			m.touched = id
			err := m.s.SetVisibility(m.ctx, id, Authenticated(principal), shared)
			if ok := m.writable(id, principal); (err == nil) != ok {
				t.Fatalf("SetVisibility(%s) by %s: err = %v, model says ok = %v", id, principal, err, ok)
			}
			if err == nil {
				m.records[id].shared = shared
			}
		},
		"advance_clock": func(t *rapid.T) {
			m.now = m.now.Add(time.Duration(rapid.IntRange(1, 20).Draw(t, "seconds")) * time.Second)
		},
		"": m.checkInvariants,
	}
}

func (m *storeModel) checkArchiveOutcome(t *rapid.T, verb, id, principal string, res ArchiveResult, err error, archive bool) {
	want := ArchiveOutcomeNotFound
	if m.writable(id, principal) {
		want = ArchiveOutcomeAlready
		if m.records[id].archived != archive {
			want = ArchiveOutcomeChanged
		}
	}
	if res.Outcome != want {
		t.Fatalf("%s(%s) by %s: outcome %q (err %v), want %q", verb, id, principal, res.Outcome, err, want)
	}
	if want == ArchiveOutcomeNotFound && !errors.Is(err, ErrNotFound) {
		t.Fatalf("%s(%s) by %s: err = %v, want ErrNotFound", verb, id, principal, err)
	}
	if want != ArchiveOutcomeNotFound {
		m.records[id].archived = archive
	}
}

// snapshot reads every record of the check's scope in one scroll, so the
// invariant check costs a few calls per step instead of several per record.
func (m *storeModel) snapshot(t *rapid.T) map[string]Memory {
	pts, err := m.s.client.Scroll(m.ctx, &qdrant.ScrollPoints{
		CollectionName: m.s.collection,
		Filter:         &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("scope", m.scope)}},
		Limit:          new(uint32(len(m.order) + 1)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		t.Fatalf("Scroll(%s): %v", m.scope, err)
	}
	out := make(map[string]Memory, len(pts))
	for _, p := range pts {
		id := p.GetId().GetUuid()
		out[id] = fromPayload(id, p.GetPayload())
	}
	return out
}

func (m *storeModel) checkInvariants(t *rapid.T) {
	for _, principal := range m.principals {
		m.checkVisible(t, principal, uint64(len(m.order)+1))
	}
	stored := m.snapshot(t)
	for _, id := range m.order {
		r := m.records[id]
		got, ok := stored[id]
		if r.deleted {
			if ok {
				t.Fatalf("%s: deleted in the model but still stored", id)
			}
			continue
		}
		if !ok {
			t.Fatalf("%s: missing from the store", id)
		}
		if succ := deref(got.SupersededBy); succ != r.supersededBy {
			t.Fatalf("%s: superseded_by = %q, model %q", id, succ, r.supersededBy)
		}
		if (got.ArchivedAt != nil) != r.archived {
			t.Fatalf("%s: archived = %v, model %v", id, got.ArchivedAt != nil, r.archived)
		}
		if !timesEqual(got.NotAfter, r.notAfter) || !timesEqual(got.NotBefore, r.notBefore) {
			t.Fatalf("%s: window (%v, %v), model (%v, %v)", id, got.NotBefore, got.NotAfter, r.notBefore, r.notAfter)
		}
		if r.expired(m.now) && r.scheduled(m.now) {
			t.Fatalf("%s: expired did not suppress scheduled", id)
		}
		checkNoCycle(t, stored, id)
	}
	if len(stored) != len(m.order)-m.deletedCount() {
		t.Fatalf("store holds %d records, model %d", len(stored), len(m.order)-m.deletedCount())
	}
	m.checkTouchedReadable(t)
}

func (m *storeModel) deletedCount() int {
	n := 0
	for _, r := range m.records {
		if r.deleted {
			n++
		}
	}
	return n
}

// checkTouchedReadable reads the last action's record by id for each
// principal. The page walk covers every record that recall shows, and this
// covers the hidden and deleted cases one record at a time.
func (m *storeModel) checkTouchedReadable(t *rapid.T) {
	r, ok := m.records[m.touched]
	if !ok {
		return
	}
	for _, principal := range m.principals {
		_, err := m.s.GetReadable(m.ctx, m.touched, Authenticated(principal))
		readable := !r.deleted && (r.owner == principal || r.shared)
		if (err == nil) != readable {
			t.Fatalf("GetReadable(%s) by %s: err = %v, model readable = %v", m.touched, principal, err, readable)
		}
	}
}

// checkVisible walks every cursor page of pageSize records and compares the
// union with the model's recall-visible set, with no duplicate and no gap.
// After each step it runs with one page that holds every record. The walk
// with small pages runs once, at the end of the sequence, because a walk
// after every step costs a call per page per step.
func (m *storeModel) checkVisible(t *rapid.T, principal string, pageSize uint64) {
	var got []string
	cursor := ""
	for page := 0; ; page++ {
		items, _, next, err := m.s.List(m.ctx, m.scope, Authenticated(principal), ListOptions{Limit: pageSize, CursorMode: true, Cursor: cursor})
		if err != nil {
			t.Fatalf("List page %d for %s: %v", page, principal, err)
		}
		got = append(got, recordIDs(items)...)
		if next == "" {
			break
		}
		if page > len(m.order) {
			t.Fatalf("List for %s did not end after %d pages", principal, page)
		}
		cursor = next
	}
	var want []string
	for _, id := range m.order {
		if m.records[id].recallVisible(principal, m.now) {
			want = append(want, id)
		}
	}
	sorted := slices.Sorted(slices.Values(got))
	if len(slices.Compact(slices.Clone(sorted))) != len(got) {
		t.Fatalf("List for %s returned a duplicate: %v", principal, got)
	}
	sort.Strings(want)
	if !slices.Equal(sorted, want) {
		t.Fatalf("List for %s at %s = %v, model %v", principal, m.now.Format(time.RFC3339), sorted, want)
	}
}

func checkNoCycle(t *rapid.T, stored map[string]Memory, id string) {
	seen := map[string]bool{}
	for cur := id; cur != ""; {
		if seen[cur] {
			t.Fatalf("supersession chain from %s has a cycle at %s", id, cur)
		}
		seen[cur] = true
		next, ok := stored[cur]
		if !ok {
			return
		}
		cur = deref(next.SupersededBy)
	}
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func timesEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

func TestModelStore(t *testing.T) {
	var clock atomic.Pointer[time.Time]
	s := newTestStore(t, dialTestClient(t), testCollection("model_store"), WithClock(func() time.Time { return *clock.Load() }))
	ctx := context.Background()
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	// Point ids are global to the collection, so they carry the run.
	run := time.Now().UnixNano()
	var checks atomic.Uint32
	rapid.Check(t, func(t *rapid.T) {
		n := checks.Add(1)
		m := &storeModel{
			ctx: ctx, s: s, scope: fmt.Sprintf("model-store:%d:%d", run, n), idPrefix: uint32(run) + n<<20,
			now:     time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC),
			records: map[string]*modelRecord{}, principals: []string{"alice", "bob"},
		}
		defer func() { _ = s.DeleteAllRaw(ctx, m.scope) }()
		// The store reads m.now through this pointer, so advance_clock moves
		// both clocks at once.
		clock.Store(&m.now)
		t.Repeat(m.actions())
		pageSize := uint64(rapid.IntRange(1, 5).Draw(t, "page_size"))
		for _, principal := range m.principals {
			m.checkVisible(t, principal, pageSize)
		}
	})
}
