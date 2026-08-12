// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/seanb4t/engram/internal/shortid"
	"github.com/seanb4t/engram/internal/store"
)

// spyCall records one invocation on spyStore: the method name, the caller's
// resolved owner (empty for anonymous/unset), and a minimal args value (the
// subject/id/scope the call concerned) — enough for the 17-05 delegation
// parity test to prove the MCP and Connect lanes hit the SAME deps.* method
// for the same subject, without reimplementing full store authz. The real
// Qdrant isolation suite (testDeps-backed tests) remains the authz gate; this
// spy is a scripted recorder, not a second authorization implementation.
type spyCall struct {
	Method string
	Owner  string
	Args   any
}

// spyStore is a scripted-spy memStore: a map-backed fake that records every
// invoked method + owner + key args and returns the minimal behavior the
// test scenarios need (create/fetch/update/delete/list/search over an
// in-memory map), never reimplementing the real store's full Qdrant filter
// semantics. It exists so Connect/MCP handler tests can run without a live
// Qdrant while still proving delegation (17-05) and exercising the six write
// handlers (17-04) end-to-end.
type spyStore struct {
	mu         sync.Mutex
	records    map[string]store.Memory
	shortIndex map[string]string // shortid.Canonical(short_id) -> id
	calls      []spyCall
}

var _ memStore = (*spyStore)(nil)

func newSpyStore() *spyStore {
	return &spyStore{
		records:    map[string]store.Memory{},
		shortIndex: map[string]string{},
	}
}

// newSpyDeps builds a deps backed by a spyStore and the non-nil fake embedder
// (tools_test.go's fakeEmbedder) — so a valid StoreMemory/StoreDiscovery/
// ScheduleMemory call reaches d.em.Embed without a nil-pointer panic (review
// HIGH). Returns the spy too, for tests that assert on recorded calls.
func newSpyDeps() (*deps, *spyStore) {
	sp := newSpyStore()
	return &deps{st: sp, em: fakeEmbedder{}}, sp
}

// callLog returns a snapshot copy of the recorded calls (safe to range over
// without racing further store activity).
func (s *spyStore) callLog() []spyCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]spyCall, len(s.calls))
	copy(out, s.calls)
	return out
}

// ownerOfSubject mirrors store.go's unexported ownerOf: a nil Subject (a
// discarded-extraction-error edge case) reports "" rather than panicking on
// Owner() (which panics only on a genuinely nil Subject, by design).
func ownerOfSubject(subj store.Subject) string {
	if subj == nil {
		return ""
	}
	return subj.Owner()
}

// readableBy mirrors store.go's GetReadable authz shape: the owner, or any
// authenticated (non-empty-owner) caller when the record is shared.
func readableBy(m store.Memory, owner string) bool {
	if m.Owner == owner {
		return true
	}
	return m.Visibility == "shared" && owner != ""
}

func hasAllTags(have, want []string) bool {
	for _, w := range want {
		if !slices.Contains(have, w) {
			return false
		}
	}
	return true
}

func (s *spyStore) record(method, owner string, args any) {
	s.calls = append(s.calls, spyCall{Method: method, Owner: owner, Args: args})
}

func (s *spyStore) Upsert(_ context.Context, m store.Memory, _ []float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("Upsert", m.Owner, m.ID)
	s.records[m.ID] = m
	if m.ShortID != "" {
		s.shortIndex[shortid.Canonical(m.ShortID)] = m.ID
	}
	return nil
}

func (s *spyStore) Get(_ context.Context, id string) (store.Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("Get", "", id)
	m, ok := s.records[id]
	if !ok {
		return store.Memory{}, fmt.Errorf("%w: %s", store.ErrNotFound, id)
	}
	return m, nil
}

func (s *spyStore) GetReadable(_ context.Context, id string, subj store.Subject) (store.Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner := ownerOfSubject(subj)
	s.record("GetReadable", owner, id)
	m, ok := s.records[id]
	if !ok || !readableBy(m, owner) {
		return store.Memory{}, fmt.Errorf("%w: %s", store.ErrNotFound, id)
	}
	return m, nil
}

func (s *spyStore) FetchForUpdate(_ context.Context, id string, subj store.Subject) (store.Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner := ownerOfSubject(subj)
	s.record("FetchForUpdate", owner, id)
	m, ok := s.records[id]
	if !ok || m.Owner != owner {
		return store.Memory{}, fmt.Errorf("%w: %s", store.ErrNotFound, id)
	}
	return m, nil
}

func (s *spyStore) ResolvePointID(_ context.Context, idOrShort string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("ResolvePointID", "", idOrShort)
	t := strings.TrimSpace(idOrShort)
	if t == "" {
		return "", fmt.Errorf("%w: empty id", store.ErrInvalidArgument)
	}
	// Fast path mirrors store.go:1207 — any well-formed UUID canonicalizes and
	// returns WITHOUT an existence check.
	if u, err := uuid.Parse(t); err == nil {
		return u.String(), nil
	}
	id, ok := s.shortIndex[shortid.Canonical(t)]
	if !ok {
		return "", fmt.Errorf("%w: %s", store.ErrNotFound, idOrShort)
	}
	return id, nil
}

func (s *spyStore) OwnedOrAbsent(_ context.Context, id string, subj store.Subject) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner := ownerOfSubject(subj)
	s.record("OwnedOrAbsent", owner, id)
	m, ok := s.records[id]
	if !ok {
		return nil // absent: a fresh id, permitted
	}
	if m.Owner != owner {
		return fmt.Errorf("%w: %s", store.ErrNotFound, id)
	}
	return nil
}

func (s *spyStore) SetVisibility(_ context.Context, id string, subj store.Subject, shared bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner := ownerOfSubject(subj)
	s.record("SetVisibility", owner, id)
	m, ok := s.records[id]
	if !ok || m.Owner != owner {
		return fmt.Errorf("%w: %s", store.ErrNotFound, id)
	}
	if shared {
		m.Visibility = "shared"
	} else {
		m.Visibility = ""
	}
	s.records[id] = m
	return nil
}

// Supersede mirrors Store.Supersede's shape, generalized to a target SET
// (phase 03.1): owner-only write gate PER TARGET, single-hop already-
// superseded rejection PER TARGET, create the new record, then back-stamp
// every target's SupersededBy — same ordering (new record first, back-stamp
// second) and same ErrNotFound ownership-gate semantics the real store uses
// (store.go). Every gate loops over the FULL targets slice — this is
// deliberate, not incidental: a version that only inspected targets[0]
// would let every multi-target authz test in later plans pass while proving
// nothing (TestSpyStoreSupersedeGatesEverySetMember pins this).
func (s *spyStore) Supersede(_ context.Context, newMem store.Memory, _ []float32, targets []string, subj store.Subject) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner := ownerOfSubject(subj)
	for _, target := range targets {
		s.record("Supersede", owner, target)
	}
	for _, target := range targets {
		targetRec, ok := s.records[target]
		if !ok || targetRec.Owner != owner {
			return fmt.Errorf("%w: %s", store.ErrNotFound, target)
		}
	}
	for _, target := range targets {
		targetRec := s.records[target]
		if targetRec.SupersededBy != nil && *targetRec.SupersededBy != "" {
			return fmt.Errorf("%w: %s", store.ErrAlreadySuperseded, target)
		}
	}
	s.records[newMem.ID] = newMem
	if newMem.ShortID != "" {
		s.shortIndex[shortid.Canonical(newMem.ShortID)] = newMem.ID
	}
	for _, target := range targets {
		targetRec := s.records[target]
		targetRec.SupersededBy = &newMem.ID
		s.records[target] = targetRec
	}
	return nil
}

func (s *spyStore) Update(_ context.Context, cur store.Memory, content string, shared *bool, tags *[]string, summary *string, _ []float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("Update", cur.Owner, cur.ID)
	m, ok := s.records[cur.ID]
	if !ok {
		return fmt.Errorf("%w: %s", store.ErrNotFound, cur.ID)
	}
	m.Content = content
	if shared != nil {
		if *shared {
			m.Visibility = "shared"
		} else {
			m.Visibility = ""
		}
	}
	if tags != nil {
		m.Tags = *tags
	}
	if summary != nil {
		m.Summary = *summary
	}
	s.records[cur.ID] = m
	return nil
}

func (s *spyStore) UpdatePayload(_ context.Context, cur store.Memory, shared *bool, summary *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("UpdatePayload", cur.Owner, cur.ID)
	m, ok := s.records[cur.ID]
	if !ok {
		return fmt.Errorf("%w: %s", store.ErrNotFound, cur.ID)
	}
	if shared != nil {
		if *shared {
			m.Visibility = "shared"
		} else {
			m.Visibility = ""
		}
	}
	if summary != nil {
		m.Summary = *summary
	}
	s.records[cur.ID] = m
	return nil
}

func (s *spyStore) Delete(_ context.Context, id string, subj store.Subject) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner := ownerOfSubject(subj)
	s.record("Delete", owner, id)
	m, ok := s.records[id]
	if !ok || m.Owner != owner {
		return fmt.Errorf("%w: %s", store.ErrNotFound, id)
	}
	delete(s.records, id)
	if m.ShortID != "" {
		delete(s.shortIndex, shortid.Canonical(m.ShortID))
	}
	return nil
}

func (s *spyStore) DeleteAll(_ context.Context, scope string, subj store.Subject) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner := ownerOfSubject(subj)
	s.record("DeleteAll", owner, scope)
	for id, m := range s.records {
		if m.Scope == scope && m.Owner == owner {
			delete(s.records, id)
			if m.ShortID != "" {
				delete(s.shortIndex, shortid.Canonical(m.ShortID))
			}
		}
	}
	return nil
}

func (s *spyStore) List(_ context.Context, scope string, subj store.Subject, opts store.ListOptions) ([]store.Memory, uint64, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner := ownerOfSubject(subj)
	s.record("List", owner, scope)
	var matched []store.Memory
	for _, m := range s.records {
		if m.Scope != scope || !readableBy(m, owner) {
			continue
		}
		if len(opts.Categories) > 0 && !slices.Contains(opts.Categories, m.Category) {
			continue
		}
		switch opts.Visibility {
		case "shared":
			if m.Visibility != "shared" {
				continue
			}
		case "private":
			if m.Visibility == "shared" {
				continue
			}
		}
		if !hasAllTags(m.Tags, opts.Tags) {
			continue
		}
		if !opts.CreatedAfter.IsZero() && m.CreatedAt.Before(opts.CreatedAfter) {
			continue
		}
		if !opts.CreatedBefore.IsZero() && !m.CreatedAt.Before(opts.CreatedBefore) {
			continue
		}
		matched = append(matched, m)
	}
	sort.Slice(matched, func(i, j int) bool {
		if opts.Ascending {
			return matched[i].CreatedAt.Before(matched[j].CreatedAt)
		}
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})
	total := uint64(len(matched))
	start := opts.Offset
	if start > total {
		start = total
	}
	end := total // Limit==0 means "all" (store.go:873-874, round-4 finding-7)
	if opts.Limit > 0 && start+opts.Limit < end {
		end = start + opts.Limit
	}
	page := append([]store.Memory{}, matched[start:end]...)
	return page, total, "", nil
}

func (s *spyStore) ListScheduled(_ context.Context, scope string, subj store.Subject, state store.ScheduledState, opts store.ListOptions) ([]store.Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner := ownerOfSubject(subj)
	s.record("ListScheduled", owner, scope)
	now := time.Now().UTC()
	var out []store.Memory
	for _, m := range s.records {
		if m.Scope != scope || m.Owner != owner {
			continue
		}
		pending := m.NotBefore != nil && now.Before(*m.NotBefore)
		expired := m.NotAfter != nil && !now.Before(*m.NotAfter)
		switch state {
		case store.ScheduledPending:
			if !pending {
				continue
			}
		case store.ScheduledExpired:
			if !expired {
				continue
			}
		case store.ScheduledAll:
			if !pending && !expired {
				continue
			}
		}
		out = append(out, m)
		if opts.Limit > 0 && uint64(len(out)) >= opts.Limit {
			break
		}
	}
	return out, nil
}

func (s *spyStore) ListScopes(_ context.Context, subj store.Subject) ([]store.ScopeCount, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner := ownerOfSubject(subj)
	s.record("ListScopes", owner, nil)
	counts := map[string]uint64{}
	for _, m := range s.records {
		if !readableBy(m, owner) {
			continue
		}
		counts[m.Scope]++
	}
	out := make([]store.ScopeCount, 0, len(counts))
	for scope, c := range counts {
		out = append(out, store.ScopeCount{Scope: scope, Count: c})
	}
	return out, false, nil
}

func (s *spyStore) MintShortID(_ context.Context, seen map[string]struct{}) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("MintShortID", "", nil)
	for range 100 {
		id, err := shortid.New()
		if err != nil {
			return "", err
		}
		canon := shortid.Canonical(id)
		if _, exists := s.shortIndex[canon]; exists {
			continue
		}
		if _, inSeen := seen[canon]; inSeen {
			continue
		}
		return id, nil
	}
	return "", errors.New("spyStore: could not mint a unique short id")
}

func (s *spyStore) SearchReranked(_ context.Context, scope string, subj store.Subject, _ string, _ []float32, k uint64, opts store.SearchOptions) ([]store.Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner := ownerOfSubject(subj)
	s.record("SearchReranked", owner, scope)
	var matched []store.Memory
	for _, m := range s.records {
		if m.Scope != scope || !readableBy(m, owner) {
			continue
		}
		if len(opts.Categories) > 0 && !slices.Contains(opts.Categories, m.Category) {
			continue
		}
		if !hasAllTags(m.Tags, opts.Tags) {
			continue
		}
		if !opts.CreatedAfter.IsZero() && m.CreatedAt.Before(opts.CreatedAfter) {
			continue
		}
		if !opts.CreatedBefore.IsZero() && !m.CreatedAt.Before(opts.CreatedBefore) {
			continue
		}
		matched = append(matched, m)
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].CreatedAt.After(matched[j].CreatedAt) })
	if uint64(len(matched)) > k {
		matched = matched[:k]
	}
	return matched, nil
}

func (s *spyStore) SearchDiscovery(_ context.Context, scope, kind string, subj store.Subject, _ []float32, k uint64) ([]store.Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner := ownerOfSubject(subj)
	s.record("SearchDiscovery", owner, scope)
	var matched []store.Memory
	for _, m := range s.records {
		if m.Category != "discovery" || !readableBy(m, owner) {
			continue
		}
		if scope != "" && m.Scope != scope {
			continue
		}
		if kind != "" && m.Kind != kind {
			continue
		}
		matched = append(matched, m)
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].CreatedAt.After(matched[j].CreatedAt) })
	if uint64(len(matched)) > k {
		matched = matched[:k]
	}
	return matched, nil
}

// TestSpyStoreRecordsMethodAndSubject pins the spy's core contract (Task 1
// acceptance): a call records the method name and the caller's owner.
func TestSpyStoreRecordsMethodAndSubject(t *testing.T) {
	sp := newSpyStore()
	ctx := context.Background()
	if err := sp.Upsert(ctx, store.Memory{ID: "x1", Owner: "actor-A"}, nil); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	calls := sp.callLog()
	if len(calls) != 1 || calls[0].Method != "Upsert" || calls[0].Owner != "actor-A" {
		t.Fatalf("calls = %+v, want single Upsert(actor-A, ...)", calls)
	}
}

// TestSpyDepsStoreMemoryReachesEmbedder pins the non-nil-embedder acceptance:
// a valid StoreMemory-shaped deps.storeMemory call over the spy-backed deps
// reaches d.em.Embed without a nil-pointer panic and records the Upsert call.
func TestSpyDepsStoreMemoryReachesEmbedder(t *testing.T) {
	d, sp := newSpyDeps()
	ctx := context.Background()
	c := caller{Subj: store.Authenticated("actor-A"), Actor: "actor-A"}
	id, sid, err := d.storeMemory(ctx, c, storeArgs{Content: "hi", Scope: "s", Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatalf("storeMemory: %v", err)
	}
	if id == "" || sid == "" {
		t.Fatalf("expected non-empty id/short_id, got %q/%q", id, sid)
	}
	found := false
	for _, call := range sp.callLog() {
		if call.Method == "Upsert" {
			found = true
		}
	}
	if !found {
		t.Fatal("spy did not record an Upsert call")
	}
}

// TestSpyStoreSupersedeGatesEverySetMember (Task 1, cycle-1 review HIGH)
// proves spyStore.Supersede gates and stamps EVERY member of the target set,
// not just the first — a signature-only pluralization would let every
// multi-target authz test in plans 03.1-03/04 pass while the fake silently
// only ever inspected targets[0]. Drives the spy directly with a two-element
// set whose SECOND element fails each gate in turn.
func TestSpyStoreSupersedeGatesEverySetMember(t *testing.T) {
	ctx := context.Background()
	owner := "sub-gate-owner"
	other := "sub-gate-other"
	subj := store.Authenticated(owner)

	t.Run("second target owned by another actor", func(t *testing.T) {
		sp := newSpyStore()
		firstID := "e0000000-0000-0000-0000-000000000001"
		secondID := "e0000000-0000-0000-0000-000000000002"
		sp.records[firstID] = store.Memory{ID: firstID, Owner: owner}
		sp.records[secondID] = store.Memory{ID: secondID, Owner: other}

		newMem := store.Memory{ID: "e0000000-0000-0000-0000-000000000003", Owner: owner}
		err := sp.Supersede(ctx, newMem, nil, []string{firstID, secondID}, subj)
		if !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("Supersede err = %v, want ErrNotFound", err)
		}
		if _, ok := sp.records[newMem.ID]; ok {
			t.Error("new record inserted despite a not-owned second target — gate is vacuous")
		}
	})

	t.Run("second target already superseded", func(t *testing.T) {
		sp := newSpyStore()
		firstID := "e0000000-0000-0000-0000-000000000004"
		secondID := "e0000000-0000-0000-0000-000000000005"
		supersededBy := "e0000000-0000-0000-0000-000000000006"
		sp.records[firstID] = store.Memory{ID: firstID, Owner: owner}
		sp.records[secondID] = store.Memory{ID: secondID, Owner: owner, SupersededBy: &supersededBy}

		newMem := store.Memory{ID: "e0000000-0000-0000-0000-000000000007", Owner: owner}
		err := sp.Supersede(ctx, newMem, nil, []string{firstID, secondID}, subj)
		if !errors.Is(err, store.ErrAlreadySuperseded) {
			t.Fatalf("Supersede err = %v, want ErrAlreadySuperseded", err)
		}
		if _, ok := sp.records[newMem.ID]; ok {
			t.Error("new record inserted despite an already-superseded second target — gate is vacuous")
		}
	})
}
