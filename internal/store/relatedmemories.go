// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file implements the related-memories neighbourhood of one anchor
// record (STORE-02): four typed edges — supersession, citation, tag, and
// vector — merged into one entry per candidate (D-06). The caller's read
// predicate (recallVisibleFilter) is composed into every Qdrant sub-query's
// filter and into the final payload fetch, never applied as a post-filter
// (D-10): no layer above internal/store may need to filter this method's
// output. RelatedMemories is read-only — it issues no write RPC and never
// stamps, links, supersedes, or archives anything.
//
// Plan 01-03 lands the contract, anchor resolution, the vector edge, and the
// supersession chain (D-06, D-09, D-10, D-11 chain-side, D-12). Plan 01-04
// adds the tag and citation edges (D-07, D-08) into this same contract.

package store

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// RelatedEdgeType names the kind of relationship a candidate record was
// reached by. This is the canonical order — used both for RelatedMemory's
// per-entry admission order (assembleRelated) and for the order edges are
// sorted within one entry.
type RelatedEdgeType string

const (
	// RelatedEdgeSupersession marks a chain member reached via superseded_by
	// (forward, to the live head) or supersedes (backward, to predecessors).
	RelatedEdgeSupersession RelatedEdgeType = "supersession"
	// RelatedEdgeCitation marks a candidate sharing a citation kind+ref with
	// the anchor (D-08; edge fields filled by plan 01-04).
	RelatedEdgeCitation RelatedEdgeType = "citation"
	// RelatedEdgeTag marks a candidate sharing one or more rarity-weighted
	// tags with the anchor (D-07; edge fields filled by plan 01-04).
	RelatedEdgeTag RelatedEdgeType = "tag"
	// RelatedEdgeVector marks a candidate reached by the query-by-id vector
	// neighbourhood search.
	RelatedEdgeVector RelatedEdgeType = "vector"
)

// SupersessionDirection names which pointer a supersession edge was reached
// through, relative to the anchor.
type SupersessionDirection string

const (
	// SupersessionSuccessor is reached by following superseded_by forward,
	// toward the live head.
	SupersessionSuccessor SupersessionDirection = "successor"
	// SupersessionPredecessor is reached by following supersedes backward,
	// toward the records the anchor's chain corrected.
	SupersessionPredecessor SupersessionDirection = "predecessor"
)

// WeightedTag is one tag the anchor and a candidate share, plus its rarity
// weight (D-07, filled by plan 01-04).
type WeightedTag struct {
	Tag    string  `json:"tag"`
	Weight float64 `json:"weight"`
}

// CitationRef is one citation kind+ref the anchor and a candidate share
// (D-08, filled by plan 01-04) — locator, pin, and excerpt are deliberately
// excluded from the identity comparison.
type CitationRef struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

// RelatedEdge is one piece of per-type evidence for why a candidate appeared
// in a RelatedMemory's Edges. Evidence is never comparable across types
// (D-12): a vector Score and a supersession Depth measure different things,
// so admission and ranking within one entry go by relatedEdgeRank (fixed
// type order), never a blended score.
type RelatedEdge struct {
	Type RelatedEdgeType `json:"type"`
	// Score is the vector edge's raw Qdrant cosine similarity. Empty for
	// every other edge type.
	Score float32 `json:"score,omitempty"`
	// SharedTags and TagWeight are the tag edge's evidence (plan 01-04):
	// the shared tags with their rarity weight, and the sum of those
	// weights. Empty for every other edge type.
	SharedTags []WeightedTag `json:"shared_tags,omitempty"`
	TagWeight  float64       `json:"tag_weight,omitempty"`
	// SharedCitations is the citation edge's evidence (plan 01-04): the
	// shared kind+ref pairs. Empty for every other edge type.
	SharedCitations []CitationRef `json:"shared_citations,omitempty"`
	// Direction and Depth are the supersession edge's evidence: which
	// pointer was followed, and how many hops from the anchor (1 = direct
	// link). Empty/zero for every other edge type.
	Direction SupersessionDirection `json:"direction,omitempty"`
	Depth     int                   `json:"depth,omitempty"`
}

// RelatedMemory is one candidate record plus every edge type that reached
// it, in canonical order (D-06's documented multi-edge rule): a candidate
// reachable by more than one edge type is ONE entry, never returned twice.
type RelatedMemory struct {
	Memory Memory        `json:"memory"`
	Edges  []RelatedEdge `json:"edges"`
}

// RelatedResult is RelatedMemories' return shape: the resolved anchor, its
// neighbourhood, and whether the total ceiling (relatedTotalCeiling) left
// out at least one vector neighbour.
type RelatedResult struct {
	Anchor    Memory          `json:"anchor"`
	Related   []RelatedMemory `json:"related"`
	Truncated bool            `json:"truncated,omitempty"`
}

// relatedCandidate is an unresolved edge hit — an id plus the single edge
// that reached it — produced by a gated sub-query (relatedVectorEdges today;
// tag and citation sub-queries from plan 01-04) before assembleRelated
// resolves its payload.
type relatedCandidate struct {
	id   string
	edge RelatedEdge
}

// relatedEdgeRank fixes the canonical admission/sort order (D-06, D-12):
// supersession, citation, tag, vector. An unrecognized type sorts last.
func relatedEdgeRank(t RelatedEdgeType) int {
	switch t {
	case RelatedEdgeSupersession:
		return 0
	case RelatedEdgeCitation:
		return 1
	case RelatedEdgeTag:
		return 2
	case RelatedEdgeVector:
		return 3
	default:
		return 4
	}
}

// relatedVectorDefaultK is the number of vector neighbours RelatedMemories
// returns when the caller passes k == 0 (D-12's "roughly 8"). The caller's k
// adjusts only this cap, subject to the shared MaxRecallLimit ceiling
// (rejectOverMaximum).
const relatedVectorDefaultK = 8

// relatedTotalCeiling is the hard cap on RelatedMemories' merged entry
// count (D-12). Documented invariant: the supersession cap
// (relatedSupersessionCap, plan 01-03 task 2) plus plan 01-04's tag and
// citation caps together stay below this ceiling, so only the vector edge —
// the one edge type the caller's k widens — can ever reach it, and no edge
// type crowds another out of the result.
const relatedTotalCeiling = 64

// relatedSupersessionDepth is the per-direction hop cap on the supersession
// walk (D-09): at most this many hops following superseded_by forward
// (toward the live head), and, independently, at most this many hops
// following supersedes backward (toward predecessors).
const relatedSupersessionDepth = 8

// relatedSupersessionCap is the total member cap across BOTH directions of
// the supersession walk (D-09) — two directions of the depth-8 walk. Kept,
// together with plan 01-04's tag and citation caps, below relatedTotalCeiling
// (64) so only the vector edge — the one edge type the caller's k widens —
// can ever reach that ceiling.
const relatedSupersessionCap = 16

// dedupSortedExcluding returns the distinct ids in ids that are not already
// in visited, sorted ascending — the shared "next level" builder for
// relatedSupersessionChain's backward breadth-first walk (D-09's
// sorted-ascending-per-level order, a Claude's-discretion flagged
// assumption).
func dedupSortedExcluding(ids []string, visited map[string]bool) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if visited[id] || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// relatedSupersessionChain walks anchor's supersession chain in both
// directions (D-09) — superseded_by forward to the live head, supersedes
// backward breadth-first through predecessors, never a full component
// traversal. Each hop is one GetReadable, which is NOT recall-gated, so a
// soft-hidden member (archived, superseded, expired, or not-yet-active
// scheduled) still appears (D-11's chain-side rule). A member the caller
// cannot read, or that no longer exists, ends that branch silently: it is
// never returned and its own supersession pointers are never followed, so
// an unreadable member's chain structure never leaks. visited guards
// against corrupt cycles across both directions and against the anchor
// itself. Bounded to at most relatedSupersessionDepth hops per direction and
// relatedSupersessionCap members total.
func (s *Store) relatedSupersessionChain(ctx context.Context, anchor Memory, subj Subject) ([]RelatedMemory, error) {
	visited := map[string]bool{anchor.ID: true}
	var out []RelatedMemory

	cur := anchor
	for depth := 1; depth <= relatedSupersessionDepth && cur.SupersededBy != nil && len(out) < relatedSupersessionCap; depth++ {
		next := *cur.SupersededBy
		if visited[next] {
			break
		}
		visited[next] = true
		m, err := s.GetReadable(ctx, next, subj)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				break
			}
			return nil, err
		}
		out = append(out, RelatedMemory{
			Memory: summaryShape(m),
			Edges:  []RelatedEdge{{Type: RelatedEdgeSupersession, Direction: SupersessionSuccessor, Depth: depth}},
		})
		cur = m
	}

	level := dedupSortedExcluding(anchor.Supersedes, visited)
	for depth := 1; depth <= relatedSupersessionDepth && len(level) > 0; depth++ {
		var next []string
		for _, id := range level {
			if len(out) >= relatedSupersessionCap {
				return out, nil
			}
			if visited[id] {
				continue
			}
			visited[id] = true
			m, err := s.GetReadable(ctx, id, subj)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					continue
				}
				return nil, err
			}
			out = append(out, RelatedMemory{
				Memory: summaryShape(m),
				Edges:  []RelatedEdge{{Type: RelatedEdgeSupersession, Direction: SupersessionPredecessor, Depth: depth}},
			})
			next = append(next, m.Supersedes...)
		}
		level = dedupSortedExcluding(next, visited)
	}
	return out, nil
}

// edgeFilter returns a NEW filter (f is never mutated, mirroring
// searchfetch.go's includeIDs) whose Must wraps f as a single nested
// condition followed by extra, and whose MustNot excludes anchorID — every
// gated sub-query composes the caller's read + recall-gate filter this way
// so the anchor itself is never returned as its own neighbour.
func edgeFilter(f *qdrant.Filter, anchorID string, extra ...*qdrant.Condition) *qdrant.Filter {
	must := make([]*qdrant.Condition, 0, 1+len(extra))
	if f != nil {
		must = append(must, qdrant.NewFilterAsCondition(f))
	}
	must = append(must, extra...)
	return &qdrant.Filter{
		Must:    must,
		MustNot: []*qdrant.Condition{qdrant.NewHasID(qdrant.NewID(anchorID))},
	}
}

// summaryShape returns m in the summary-view shape Search returns:
// Citations nil, and Content cleared when m carries a stored Summary (an
// empty-Summary record keeps its Content, matching backfillNoSummaryContent's
// no-summary fallback).
func summaryShape(m Memory) Memory {
	m.Citations = nil
	if m.Summary != "" {
		m.Content = ""
	}
	return m
}

// relatedVectorEdges finds the anchor's vector neighbourhood: exactly one
// query-by-id Query (mirroring NearDuplicates' shape, spine.go) under
// edgeFilter(f, anchorID) — f already carries the caller's read predicate
// and the recall gate (D-10, D-11). Results are sorted by score descending,
// then id ascending, so equal-score neighbours come back in a deterministic
// order (D-12 precision).
func (s *Store) relatedVectorEdges(ctx context.Context, f *qdrant.Filter, anchorID string, k uint64) ([]relatedCandidate, error) {
	res, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collection,
		Query:          qdrant.NewQueryID(qdrant.NewID(anchorID)),
		Filter:         edgeFilter(f, anchorID),
		Limit:          qdrant.PtrOf(k),
		WithPayload:    qdrant.NewWithPayload(false),
	})
	if err != nil {
		return nil, err
	}
	out := make([]relatedCandidate, len(res))
	for i, p := range res {
		out[i] = relatedCandidate{
			id:   p.Id.GetUuid(),
			edge: RelatedEdge{Type: RelatedEdgeVector, Score: p.Score},
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].edge.Score != out[j].edge.Score {
			return out[i].edge.Score > out[j].edge.Score
		}
		return out[i].id < out[j].id
	})
	return out, nil
}

// assembleRelated merges chain (the supersession chain, already resolved
// full RelatedMemory entries) with every candidate in gated (unresolved
// vector/tag/citation hits sharing one payload fetch under f — the SAME
// filter value each sub-query used, the enforcement point, never a
// post-filter) into RelatedMemories' final Related slice.
//
// Admission is by type, never a blended score (D-12): each gated list is
// walked in the order given, and within one entry Edges are sorted by
// relatedEdgeRank. A candidate already present (in chain, or added by an
// earlier gated list) gets its new edge appended to the existing entry — the
// one-entry-per-candidate rule (D-06). A candidate dropped between the
// sub-query and the fetch (fetchPayloadsByID's re-applied filter) is skipped
// silently. Once len(related) reaches relatedTotalCeiling, further NEW
// entries are skipped and truncated is set — an already-present candidate's
// extra edge is still recorded, since that costs no new entry.
func (s *Store) assembleRelated(ctx context.Context, f *qdrant.Filter, anchorID string, chain []RelatedMemory, gated ...[]relatedCandidate) (related []RelatedMemory, truncated bool, err error) {
	chainIDs := make(map[string]bool, len(chain))
	for _, c := range chain {
		chainIDs[c.Memory.ID] = true
	}

	seen := make(map[string]bool)
	var ids []string
	for _, list := range gated {
		for _, c := range list {
			if c.id == anchorID || chainIDs[c.id] || seen[c.id] {
				continue
			}
			seen[c.id] = true
			ids = append(ids, c.id)
		}
	}

	fetched, err := s.fetchPayloadsByID(ctx, f, s.summaryView(), ids)
	if err != nil {
		return nil, false, err
	}
	slice := make([]Memory, 0, len(ids))
	sliceIndex := make(map[string]int, len(ids))
	for _, id := range ids {
		m, ok := fetched[id]
		if !ok {
			continue
		}
		sliceIndex[id] = len(slice)
		slice = append(slice, m)
	}
	if err := s.backfillNoSummaryContent(ctx, f, slice); err != nil {
		return nil, false, err
	}

	related = append([]RelatedMemory{}, chain...)
	index := make(map[string]int, len(related))
	for i, r := range related {
		index[r.Memory.ID] = i
	}

	for _, list := range gated {
		for _, c := range list {
			if idx, ok := index[c.id]; ok {
				related[idx].Edges = append(related[idx].Edges, c.edge)
				continue
			}
			si, ok := sliceIndex[c.id]
			if !ok {
				// Dropped between the sub-query and the fetch — never
				// surfaced, never counted against the ceiling.
				continue
			}
			if len(related) >= relatedTotalCeiling {
				truncated = true
				continue
			}
			related = append(related, RelatedMemory{
				Memory: summaryShape(slice[si]),
				Edges:  []RelatedEdge{c.edge},
			})
			index[c.id] = len(related) - 1
		}
	}

	for i := range related {
		edges := related[i].Edges
		sort.SliceStable(edges, func(a, b int) bool {
			return relatedEdgeRank(edges[a].Type) < relatedEdgeRank(edges[b].Type)
		})
	}
	return related, truncated, nil
}

// RelatedMemories returns id's neighbourhood (STORE-02, SC3): the
// supersession chain in both directions (D-09, appearing even when
// soft-hidden, D-11) plus the vector, tag, and citation edges gated by the
// caller's read predicate and the recall gate (D-10, D-11) — cross-spine,
// every scope the caller can read. k adjusts only the vector cap (0 becomes
// relatedVectorDefaultK) and is rejected above MaxRecallLimit before any
// RPC. The merged result never exceeds relatedTotalCeiling; Truncated
// reports whether a fetched vector candidate was left out by it.
//
// An unreadable, nonexistent, or nil-Subject anchor returns GetReadable's
// ErrNotFound — indistinguishable, matching every other per-id gate in this
// package. A readable anchor that is itself soft-hidden (archived or
// superseded) still returns its neighbourhood: Get is not recall-gated.
//
// id must be a canonical id — callers resolve short ids first
// (ResolvePointID). RelatedMemories issues no write RPC. Phase 3 wraps this
// as the RelatedMemories RPC and the related_memories MCP tool (RPC-04).
func (s *Store) RelatedMemories(ctx context.Context, id string, subj Subject, k uint64) (res RelatedResult, err error) {
	ctx, span := tracer.Start(ctx, "store.RelatedMemories", trace.WithAttributes(
		attribute.String("engram.id", id),
		attribute.String("engram.owner", ownerOf(subj)),
		attribute.Int64("engram.k", int64(k)),
	))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "RelatedMemories", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(
				attribute.Int("engram.result_count", len(res.Related)),
				attribute.Bool("engram.related.truncated", res.Truncated),
			)
		}
	}()

	if err = rejectOverMaximum("k", k); err != nil {
		return RelatedResult{}, err
	}
	if k == 0 {
		k = relatedVectorDefaultK
	}
	anchor, err := s.GetReadable(ctx, id, subj)
	if err != nil {
		return RelatedResult{}, err
	}
	f := s.recallVisibleFilter(ctx, "", subj)
	chain, err := s.relatedSupersessionChain(ctx, anchor, subj)
	if err != nil {
		return RelatedResult{}, err
	}
	vector, err := s.relatedVectorEdges(ctx, f, anchor.ID, k)
	if err != nil {
		return RelatedResult{}, err
	}
	related, truncated, err := s.assembleRelated(ctx, f, anchor.ID, chain, vector)
	if err != nil {
		return RelatedResult{}, err
	}
	return RelatedResult{Anchor: summaryShape(anchor), Related: related, Truncated: truncated}, nil
}
