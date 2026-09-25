// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// pinHitSet is one of the three hit-set shapes TestRankCandidatesIsTheD05Winner
// (rerank_test.go) pins the lexical rank step to. Reused here so RankWithHook's
// composition edges are proven against the SAME shapes the lexical step itself
// is pinned against, never a fresh hand-picked set that could drift.
type pinHitSet struct {
	name  string
	query string
	hits  []Memory
}

func pinHitSets() []pinHitSet {
	return []pinHitSet{
		{
			name:  "mixed scores",
			query: "task lint golangci-lint config",
			hits: []Memory{
				{ID: "topical-neighbor", Content: "The bare task target runs lint then test; CI invokes it directly.", Score: 0.91},
				{ID: "high-overlap", Content: "Run task lint before every commit; golangci-lint config lives in .golangci.yaml.", Tags: []string{"lint", "task"}, Score: 0.80},
				{ID: "unrelated", Content: "Qdrant collection payload schema.", Score: 0.40},
			},
		},
		{
			name:  "tied scores",
			query: "same content",
			hits: []Memory{
				{ID: "b", Content: "same content same content", Score: 0.5},
				{ID: "a", Content: "same content same content", Score: 0.5},
			},
		},
		{
			name:  "gh261-shaped",
			query: "correctable memory MCP server for coding agents",
			hits: []Memory{
				{ID: "verbatim-restatement", Content: "correctable memory MCP server for coding agents", Score: 0.55},
				{ID: "topically-similar", Content: "a memory store for AI assistants with correction support", Score: 0.93},
			},
		},
	}
}

// allSameHook returns a RankHook that maps every hit it is handed to v.
func allSameHook(v float64) RankHook {
	return func(_ context.Context, _ string, hits []Memory) (map[string]float64, error) {
		m := make(map[string]float64, len(hits))
		for _, h := range hits {
			m[h.ID] = v
		}
		return m, nil
	}
}

// TestRankWithHookNilIsRankCandidates pins the nil-hook path to be the
// IDENTICAL rankCandidates call SearchReranked made before RankHook existed
// (byte-identical default), across every D-05 pin hit-set shape and a range
// of k including below/at/above len(hits).
func TestRankWithHookNilIsRankCandidates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, hs := range pinHitSets() {
		t.Run(hs.name, func(t *testing.T) {
			t.Parallel()
			for _, k := range []int{1, len(hs.hits) - 1, len(hs.hits), len(hs.hits) + 5} {
				got := RankWithHook(ctx, hs.query, hs.hits, k, nil)
				want := rankCandidates(hs.query, hs.hits, k)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("RankWithHook(nil, k=%d) = %v, want rankCandidates output %v", k, idsOf(got), idsOf(want))
				}
				for _, m := range got {
					if m.Relevance != nil {
						t.Errorf("k=%d: hit %s carries Relevance with a nil hook", k, m.ID)
					}
				}
			}
		})
	}
}

// TestRankWithHookSortsByRelevance proves distinct probabilities reorder the
// pool descending, every returned hit's Relevance equals its map value
// exactly, and the hook receives the WHOLE pool (D-04) in exactly
// rankCandidates(query, hits, len(hits)) order even when k is below len(hits).
func TestRankWithHookSortsByRelevance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hs := pinHitSets()[0]
	rel := map[string]float64{
		"topical-neighbor": 0.10,
		"high-overlap":     0.95,
		"unrelated":        0.50,
	}
	var received []Memory
	hook := func(_ context.Context, _ string, hits []Memory) (map[string]float64, error) {
		received = hits
		return rel, nil
	}

	got := RankWithHook(ctx, hs.query, hs.hits, 2, hook)

	wantPool := rankCandidates(hs.query, hs.hits, len(hs.hits))
	if !reflect.DeepEqual(received, wantPool) {
		t.Fatalf("hook received %v, want the whole lexical-order pool %v", idsOf(received), idsOf(wantPool))
	}

	wantIDs := []string{"high-overlap", "unrelated"}
	if gotIDs := idsOf(got); !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("RankWithHook order = %v, want %v", gotIDs, wantIDs)
	}
	for _, m := range got {
		if m.Relevance == nil || *m.Relevance != rel[m.ID] {
			t.Errorf("hit %s Relevance = %v, want %v", m.ID, m.Relevance, rel[m.ID])
		}
	}
}

// TestRankWithHookTiesKeepLexicalOrder pins D-03's tie rule: an accepted map
// whose every value is identical (the no-answer 0.02 shape) leaves the
// output in the plain lexical order, and every hit still carries the tied
// Relevance value.
func TestRankWithHookTiesKeepLexicalOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hs := pinHitSets()[0]

	got := RankWithHook(ctx, hs.query, hs.hits, len(hs.hits), allSameHook(0.02))
	want := rankCandidates(hs.query, hs.hits, len(hs.hits))
	if !reflect.DeepEqual(idsOf(got), idsOf(want)) {
		t.Fatalf("tie order = %v, want lexical order %v", idsOf(got), idsOf(want))
	}
	for _, m := range got {
		if m.Relevance == nil || *m.Relevance != 0.02 {
			t.Errorf("hit %s Relevance = %v, want 0.02", m.ID, m.Relevance)
		}
	}
}

// TestRankWithHookPromotesBeyondK proves relevance can promote a hit the
// lexical step ranked last (10th of 10) all the way to first, surviving
// truncation to k=3.
func TestRankWithHookPromotesBeyondK(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hits := make([]Memory, 10)
	for i := range hits {
		hits[i] = Memory{ID: fmt.Sprintf("h%d", i), Content: "filler content, no query overlap", Score: float32(1.0 - float64(i)*0.1)}
	}
	lexical := rankCandidates("", hits, len(hits))
	last := lexical[len(lexical)-1]

	rel := make(map[string]float64, len(hits))
	for _, h := range lexical {
		if h.ID == last.ID {
			rel[h.ID] = 0.99
		} else {
			rel[h.ID] = 0.01
		}
	}

	got := RankWithHook(ctx, "", hits, 3, func(_ context.Context, _ string, _ []Memory) (map[string]float64, error) {
		return rel, nil
	})
	if len(got) != 3 {
		t.Fatalf("got %d hits, want 3", len(got))
	}
	if got[0].ID != last.ID {
		t.Fatalf("RankWithHook[0] = %s, want the lexically-last hit %s promoted to first", got[0].ID, last.ID)
	}
}

// TestRankWithHookFallback tables every D-03 fallback shape: a hook error, a
// nil map with a nil error, a map missing one hit, and every out-of-domain
// probability value (NaN, +Inf, below 0, above 1). Every case must deep-equal
// the plain rankCandidates output with every Relevance left nil — a rejected
// map NEVER partially applies.
func TestRankWithHookFallback(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hs := pinHitSets()[0]
	const k = 2

	cases := []struct {
		name string
		hook RankHook
	}{
		{"hook error", func(context.Context, string, []Memory) (map[string]float64, error) {
			return nil, errors.New("boom")
		}},
		{"nil map with nil error", func(context.Context, string, []Memory) (map[string]float64, error) {
			return nil, nil
		}},
		{"map missing one hit", func(_ context.Context, _ string, hits []Memory) (map[string]float64, error) {
			m := make(map[string]float64, len(hits))
			for i, h := range hits {
				if i == 0 {
					continue // deliberately omit the first hit
				}
				m[h.ID] = 0.5
			}
			return m, nil
		}},
		{"NaN", allSameHook(math.NaN())},
		{"+Inf", allSameHook(math.Inf(1))},
		{"-0.1", allSameHook(-0.1)},
		{"1.0000001", allSameHook(1.0000001)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RankWithHook(ctx, hs.query, hs.hits, k, tc.hook)
			want := rankCandidates(hs.query, hs.hits, k)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v, want lexical order %v", idsOf(got), idsOf(want))
			}
			for _, m := range got {
				if m.Relevance != nil {
					t.Errorf("hit %s carries Relevance %v, want nil", m.ID, *m.Relevance)
				}
			}
		})
	}
}

// TestRankWithHookBoundaryValues proves 0 and 1 (the domain's own closed
// boundary) are ACCEPTED, not rejected as out-of-range — a 0 relevance is a
// legitimate "confidently not relevant" value, distinct from "not scored"
// (nil).
func TestRankWithHookBoundaryValues(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hs := pinHitSets()[1] // tied scores: ids "a" and "b"
	rel := map[string]float64{"a": 0.0, "b": 1.0}

	got := RankWithHook(ctx, hs.query, hs.hits, len(hs.hits), func(context.Context, string, []Memory) (map[string]float64, error) {
		return rel, nil
	})
	if len(got) != 2 {
		t.Fatalf("got %d hits, want 2", len(got))
	}
	if got[0].ID != "b" || got[0].Relevance == nil || *got[0].Relevance != 1.0 {
		t.Fatalf("got[0] = %+v, want id b with Relevance 1.0", got[0])
	}
	if got[1].ID != "a" || got[1].Relevance == nil || *got[1].Relevance != 0.0 {
		t.Fatalf("got[1] = %+v, want id a with a PRESENT Relevance 0.0 (0 is accepted, not rejected)", got[1])
	}
}

// TestRankWithHookIgnoresForeignIDs proves a map naming an id outside the
// candidate pool never adds a record to the result — ranking can only
// reorder the hits it was handed, never widen membership.
func TestRankWithHookIgnoresForeignIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hs := pinHitSets()[0]
	rel := map[string]float64{
		"topical-neighbor":       0.30,
		"high-overlap":           0.90,
		"unrelated":              0.60,
		"foreign-id-not-in-pool": 1.0,
	}

	got := RankWithHook(ctx, hs.query, hs.hits, len(hs.hits), func(context.Context, string, []Memory) (map[string]float64, error) {
		return rel, nil
	})
	wantIDs := []string{"high-overlap", "unrelated", "topical-neighbor"}
	if gotIDs := idsOf(got); !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("got %v, want %v (foreign id must never appear)", gotIDs, wantIDs)
	}
}

// TestRankWithHookEmptyPoolSkipsHook proves the hook is NEVER called for an
// empty candidate pool.
func TestRankWithHookEmptyPoolSkipsHook(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	called := false
	hook := func(context.Context, string, []Memory) (map[string]float64, error) {
		called = true
		return nil, nil
	}
	got := RankWithHook(ctx, "q", nil, 5, hook)
	if called {
		t.Fatal("hook was called for an empty pool")
	}
	want := rankCandidates("q", nil, 5)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestRankWithHookDoesNotMutateInput proves a successful rerank never
// mutates the caller's input slice: its order is unchanged and none of its
// elements gains a Relevance pointer.
func TestRankWithHookDoesNotMutateInput(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hs := pinHitSets()[0]
	inputOrderBefore := idsOf(hs.hits)
	rel := map[string]float64{
		"topical-neighbor": 0.30,
		"high-overlap":     0.90,
		"unrelated":        0.60,
	}

	_ = RankWithHook(ctx, hs.query, hs.hits, len(hs.hits), func(context.Context, string, []Memory) (map[string]float64, error) {
		return rel, nil
	})

	if got := idsOf(hs.hits); !reflect.DeepEqual(got, inputOrderBefore) {
		t.Fatalf("RankWithHook mutated input order: got %v, want %v", got, inputOrderBefore)
	}
	for i, h := range hs.hits {
		if h.Relevance != nil {
			t.Errorf("input hit %d (%s) carries Relevance %v after RankWithHook, want nil (untouched)", i, h.ID, *h.Relevance)
		}
	}
}

// TestSearchRerankedRankHookSeesOnlyReadable proves the hook receives
// EXACTLY the authz-filtered candidate pool (T-04-02): owner A's three
// private records plus owner B's one SHARED record — never B's two private
// records, even when the hook's own map tries to smuggle relevance in for
// them.
func TestSearchRerankedRankHookSeesOnlyReadable(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "rank-hook-visibility:project:test"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	vec := []float32{0.1, 0.2, 0.3}
	now := time.Now().UTC()

	aPriv1 := Memory{ID: "e5000000-0000-0000-0000-000000000001", Content: "owner A private record one", Scope: scope, Owner: "owner-A", CreatedAt: now}
	aPriv2 := Memory{ID: "e5000000-0000-0000-0000-000000000002", Content: "owner A private record two", Scope: scope, Owner: "owner-A", CreatedAt: now}
	aPriv3 := Memory{ID: "e5000000-0000-0000-0000-000000000003", Content: "owner A private record three", Scope: scope, Owner: "owner-A", CreatedAt: now}
	bPriv1 := Memory{ID: "e5000000-0000-0000-0000-000000000004", Content: "owner B private record one", Scope: scope, Owner: "owner-B", CreatedAt: now}
	bPriv2 := Memory{ID: "e5000000-0000-0000-0000-000000000005", Content: "owner B private record two", Scope: scope, Owner: "owner-B", CreatedAt: now}
	bShared := Memory{ID: "e5000000-0000-0000-0000-000000000006", Content: "owner B shared record", Scope: scope, Owner: "owner-B", Visibility: "shared", CreatedAt: now}

	for _, m := range []Memory{aPriv1, aPriv2, aPriv3, bPriv1, bPriv2, bShared} {
		if err := s.Upsert(ctx, m, vec); err != nil {
			t.Fatalf("upsert %s: %v", m.ID, err)
		}
	}

	var received []Memory
	hook := func(_ context.Context, _ string, hits []Memory) (map[string]float64, error) {
		received = hits
		rel := make(map[string]float64, len(hits)+2)
		for _, h := range hits {
			rel[h.ID] = 0.5
		}
		// Attempt to smuggle in relevance for owner B's private ids too —
		// must have zero effect on the returned membership: ranking can
		// only reorder the hits it was handed.
		rel[bPriv1.ID] = 1.0
		rel[bPriv2.ID] = 1.0
		return rel, nil
	}

	got, err := s.SearchReranked(ctx, scope, Authenticated("owner-A"), "record", vec, 10, SearchOptions{RankHook: hook})
	if err != nil {
		t.Fatalf("SearchReranked: %v", err)
	}

	wantIDs := []string{aPriv1.ID, aPriv2.ID, aPriv3.ID, bShared.ID}
	gotIDs := recordIDs(got)
	slices.Sort(gotIDs)
	slices.Sort(wantIDs)
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("SearchReranked ids = %v, want exactly %v", gotIDs, wantIDs)
	}
	receivedIDs := recordIDs(received)
	slices.Sort(receivedIDs)
	if !slices.Equal(receivedIDs, wantIDs) {
		t.Fatalf("hook received %v, want exactly the authz-filtered pool %v", receivedIDs, wantIDs)
	}
	for _, m := range got {
		if m.Relevance == nil {
			t.Errorf("hit %s has nil Relevance", m.ID)
		}
	}
}

// TestSearchRerankedRankHookPoolAtRecallMaximum proves the hook never sees
// more than CandidateK's own bound (RANK-05: "up to the recall maximum"
// never means "unbounded") — 105 readable records seeded, k=2 hands the hook
// CandidateK(2) (32) hits, and k=MaxRecallLimit hands it exactly the
// CandidateK cap of 100.
func TestSearchRerankedRankHookPoolAtRecallMaximum(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "rank-hook-pool-max:project:test"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	vec := []float32{0.1, 0.2, 0.3}
	now := time.Now().UTC()

	const total = 105
	for i := 0; i < total; i++ {
		id := fmt.Sprintf("e6000000-0000-0000-%04d-000000000000", i)
		m := Memory{ID: id, Content: fmt.Sprintf("pool max record %d", i), Scope: scope, Owner: "owner-pool", CreatedAt: now}
		if err := s.Upsert(ctx, m, vec); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}

	var poolLen int
	hook := func(_ context.Context, _ string, hits []Memory) (map[string]float64, error) {
		poolLen = len(hits)
		rel := make(map[string]float64, len(hits))
		for _, h := range hits {
			rel[h.ID] = 0.5
		}
		return rel, nil
	}

	got, err := s.SearchReranked(ctx, scope, Authenticated("owner-pool"), "pool max", vec, 2, SearchOptions{RankHook: hook})
	if err != nil {
		t.Fatalf("SearchReranked k=2: %v", err)
	}
	if want := int(CandidateK(2)); poolLen != want {
		t.Fatalf("hook pool size at k=2 = %d, want CandidateK(2) = %d", poolLen, want)
	}
	if len(got) != 2 {
		t.Fatalf("got %d hits at k=2, want 2", len(got))
	}

	got, err = s.SearchReranked(ctx, scope, Authenticated("owner-pool"), "pool max", vec, MaxRecallLimit, SearchOptions{RankHook: hook})
	if err != nil {
		t.Fatalf("SearchReranked k=MaxRecallLimit: %v", err)
	}
	if poolLen != 100 {
		t.Fatalf("hook pool size at k=MaxRecallLimit = %d, want 100 (CandidateK's own cap)", poolLen)
	}
	if len(got) != 100 {
		t.Fatalf("got %d hits at k=MaxRecallLimit, want 100", len(got))
	}
}

// TestSearchRerankedAuditGate proves SearchOptions.RankAudit is the ONLY
// thing that makes SearchReranked emit the "search rerank audit" line
// (#618): off → no line even with a hook that applied; on → exactly one
// line, carrying the caller's owner and query and never record content.
func TestSearchRerankedAuditGate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "rank-audit-gate:project:test"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	vec := []float32{0.1, 0.2, 0.3}
	const sentinel = "SENTINEL-AUDIT-GATE-4b2d"
	for i, id := range []string{"e6000000-0000-0000-0000-000000000001", "e6000000-0000-0000-0000-000000000002"} {
		m := Memory{ID: id, Content: sentinel, Summary: sentinel, Scope: scope, Owner: "owner-A", CreatedAt: time.Now().UTC().Add(time.Duration(i) * time.Second)}
		if err := s.Upsert(ctx, m, vec); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	hook := allSameHook(0.5)

	for _, audit := range []bool{false, true} {
		t.Run(fmt.Sprintf("audit=%v", audit), func(t *testing.T) {
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
			t.Cleanup(func() { slog.SetDefault(prev) })

			if _, err := s.SearchReranked(ctx, scope, Authenticated("owner-A"), "audit query", vec, 10, SearchOptions{RankHook: hook, RankAudit: audit}); err != nil {
				t.Fatalf("SearchReranked: %v", err)
			}
			var lines []string
			for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
				if strings.Contains(l, `"search rerank audit"`) {
					lines = append(lines, l)
				}
			}
			if !audit {
				if len(lines) != 0 {
					t.Fatalf("audit off emitted %d audit lines: %v", len(lines), lines)
				}
				return
			}
			if len(lines) != 1 {
				t.Fatalf("audit on emitted %d audit lines, want 1: %q", len(lines), buf.String())
			}
			if strings.Contains(lines[0], sentinel) {
				t.Fatalf("audit line leaked record content: %s", lines[0])
			}
			for _, want := range []string{`"owner":"owner-A"`, `"query":"audit query"`, `"outcome":"applied"`, `"surface":"search_memory"`} {
				if !strings.Contains(lines[0], want) {
					t.Errorf("audit line missing %s: %s", want, lines[0])
				}
			}
		})
	}
}
