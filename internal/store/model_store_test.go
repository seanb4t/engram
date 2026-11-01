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
			if !ok {
				return
			}
			m.records[id] = &modelRecord{owner: principal}
			m.order = append(m.order, id)
			for _, target := range targets {
				m.records[target].supersededBy = id
			}
		},
		"archive": func(t *rapid.T) {
			principal := rapid.SampledFrom(m.principals).Draw(t, "principal")
			id := m.anyID(t, "id")
			res, err := m.s.ArchiveAs(m.ctx, id, Authenticated(principal))
			m.checkArchiveOutcome(t, "ArchiveAs", id, principal, res, err, true)
		},
		"restore": func(t *rapid.T) {
			principal := rapid.SampledFrom(m.principals).Draw(t, "principal")
			id := m.anyID(t, "id")
			res, err := m.s.RestoreAs(m.ctx, id, Authenticated(principal))
			m.checkArchiveOutcome(t, "RestoreAs", id, principal, res, err, false)
		},
		"delete": func(t *rapid.T) {
			principal := rapid.SampledFrom(m.principals).Draw(t, "principal")
			id := m.anyID(t, "id")
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

func (m *storeModel) checkInvariants(t *rapid.T) {
	for _, principal := range m.principals {
		m.checkPagination(t, principal)
	}
	for _, id := range m.order {
		r := m.records[id]
		if r.deleted {
			continue
		}
		got, err := m.s.Get(m.ctx, id)
		if err != nil {
			t.Fatalf("Get(%s): %v", id, err)
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
		m.checkNoCycle(t, id)
		for _, principal := range m.principals {
			_, err := m.s.GetReadable(m.ctx, id, Authenticated(principal))
			if readable := r.owner == principal || r.shared; (err == nil) != readable {
				t.Fatalf("GetReadable(%s) by %s: err = %v, model readable = %v", id, principal, err, readable)
			}
		}
	}
}

// checkPagination walks every cursor page and compares the union with the
// model's recall-visible set, with no duplicate and no gap.
func (m *storeModel) checkPagination(t *rapid.T, principal string) {
	var got []string
	cursor := ""
	for page := 0; ; page++ {
		items, _, next, err := m.s.List(m.ctx, m.scope, Authenticated(principal), ListOptions{Limit: 2, CursorMode: true, Cursor: cursor})
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

func (m *storeModel) checkNoCycle(t *rapid.T, id string) {
	seen := map[string]bool{}
	for cur := id; cur != ""; {
		if seen[cur] {
			t.Fatalf("supersession chain from %s has a cycle at %s", id, cur)
		}
		seen[cur] = true
		r, ok := m.records[cur]
		if !ok || r.deleted {
			return
		}
		got, err := m.s.Get(m.ctx, cur)
		if err != nil {
			t.Fatalf("Get(%s) in chain from %s: %v", cur, id, err)
		}
		cur = deref(got.SupersededBy)
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
	})
}
