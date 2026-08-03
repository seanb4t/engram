// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

// benchMemory is a representative curated-category record — the dominant recall
// payload. All curated fields are populated with realistic values and a few
// tags; discovery citations are absent (discovery is recalled on demand, not the
// hot path). Shared across the payload-mapping benchmarks so a regression is
// measured against a fixed, realistic shape rather than a degenerate empty one.
func benchMemory() Memory {
	return Memory{
		ID:         "11111111-1111-1111-1111-111111111111",
		Content:    "Qdrant NewClient fires a construction-time HealthCheck RPC; it fast-fails on a refused loopback port but hangs against a SYN-dropping host.",
		Scope:      "repo:github.com/seanb4t/engram",
		Repo:       "github.com/seanb4t/engram",
		Workspace:  "repo:github.com/seanb4t/engram:ws:worktree-engram-mbnw",
		Worktree:   "/Volumes/Code/github.com/seanb4t/engram_worktrees/engram-mbnw",
		BaseDir:    "/Volumes/Code/github.com/seanb4t/engram",
		Source:     "agent-inferred",
		Category:   "gotcha",
		Tags:       []string{"qdrant", "store", "test-affecting"},
		Actor:      "sean@fzymgc.email",
		Owner:      "oidc-sub-1234567890",
		Visibility: visibilityShared,
		CreatedAt:  time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC),
	}
}

// BenchmarkPayload measures the engram-owned write-side mapping Memory ->
// map[string]any that runs on every Upsert/Update. Isolated from
// qdrant.NewValueMap (see BenchmarkPayloadValueMap) so a regression can be
// attributed to this package's code rather than the qdrant client library.
func BenchmarkPayload(b *testing.B) {
	m := benchMemory()
	if len(payload(m)) == 0 {
		b.Fatal("fixture produced empty payload")
	}
	b.ReportAllocs()
	for b.Loop() {
		payload(m)
	}
}

// BenchmarkPayloadValueMap measures the full write-side transform handed to
// client.Upsert: payload(m) plus qdrant.NewValueMap. This is the end-to-end CPU
// cost of preparing a record for persistence, library conversion included.
func BenchmarkPayloadValueMap(b *testing.B) {
	m := benchMemory()
	if len(qdrant.NewValueMap(payload(m))) == 0 {
		b.Fatal("fixture produced empty payload")
	}
	b.ReportAllocs()
	for b.Loop() {
		qdrant.NewValueMap(payload(m))
	}
}

// BenchmarkFromPayload measures the read-side Qdrant payload -> Memory mapping.
// This is the hottest pure path in the store: fromPayload runs once per result
// point on every Search/List/ListScopes/Get, so a regression here taxes all
// recall. The fixture is built via the real write path so the input matches what
// Qdrant returns in production.
func BenchmarkFromPayload(b *testing.B) {
	const id = "11111111-1111-1111-1111-111111111111"
	p := qdrant.NewValueMap(payload(benchMemory()))
	if fromPayload(id, p).Content == "" {
		b.Fatal("fixture round-trip produced empty content")
	}
	b.ReportAllocs()
	for b.Loop() {
		fromPayload(id, p)
	}
}

// BenchmarkSearchFilter measures the per-query filter construction performed at
// the head of every Search: the authorization scope/owner filter composed with
// the active recall-window conditions and the tag pre-filter. It mirrors the
// exact append sequence in Store.Search.
func BenchmarkSearchFilter(b *testing.B) {
	s := New(nil, "bench")
	subj := Authenticated("oidc-sub-1234567890")
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	tags := []string{"qdrant", "store"}
	const scope = "repo:github.com/seanb4t/engram"
	if len(s.ownerScopeFilter(context.Background(), scope, subj).Must) == 0 {
		b.Fatal("fixture produced empty filter")
	}
	b.ReportAllocs()
	for b.Loop() {
		f := s.ownerScopeFilter(context.Background(), scope, subj)
		f.Must = append(f.Must, activeWindowConditions(now)...)
		f.Must = append(f.Must, tagMatchConditions(tags)...)
	}
}
