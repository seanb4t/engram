// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"time"
)

// RecordState is the per-record state Store.RecordStates fetches to feed
// the curation-verdict pass's truncated state (D-09): ID, Summary, Content
// and CreatedAt only. Deliberately excludes tags, citations, owner and
// scope — this fetch exists solely to feed the verdict pass's truncated
// state, not to become a second Get.
type RecordState struct {
	ID        string
	Summary   string
	Content   string
	CreatedAt time.Time
}

// RecordStates fetches the verdict-state view (content, summary,
// created_at) for ids, deduplicated preserving first-seen order, via
// fetchPayloadsByID. Subject-less operator-tier read exactly like
// NearDuplicates: no scope, owner or recall-gate filter, because a
// candidate pair can include a superseded or archived record. An id absent
// from the store is simply absent from the result map — never an error.
// Issues no write RPC on any path.
func (s *Store) RecordStates(ctx context.Context, ids []string) (map[string]RecordState, error) {
	fetched, err := s.fetchPayloadsByID(ctx, nil, s.verdictStateView(), dedupeIDsPreserveOrder(ids))
	if err != nil {
		return nil, err
	}
	out := make(map[string]RecordState, len(fetched))
	for id, m := range fetched {
		out[id] = RecordState{ID: id, Summary: m.Summary, Content: m.Content, CreatedAt: m.CreatedAt}
	}
	return out, nil
}

// dedupeIDsPreserveOrder returns ids with duplicates removed, preserving
// first-seen order. Returns a non-nil empty slice for a nil or empty
// input.
func dedupeIDsPreserveOrder(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
