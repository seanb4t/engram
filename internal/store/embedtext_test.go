// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"testing"

	"github.com/qdrant/go-client/qdrant"
)

func TestEmbedText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		tags    []string
		want    string
	}{
		{"no tags", "hello world", nil, "hello world"},
		{"empty tags", "hello world", []string{}, "hello world"},
		{"one tag", "dns bug", []string{"dns"}, "dns bug\n\ntags: dns"},
		{"many tags preserve order", "dns bug", []string{"musl", "ndots", "getaddrinfo"},
			"dns bug\n\ntags: musl, ndots, getaddrinfo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EmbedText(tc.content, tc.tags); got != tc.want {
				t.Errorf("EmbedText(%q, %v) = %q, want %q", tc.content, tc.tags, got, tc.want)
			}
		})
	}
}

// memoriesFromPoints must carry the Qdrant similarity score onto each Memory so
// callers can see how close a near-miss was (GH#261 ask #3).
func TestMemoriesFromPointsCarriesScore(t *testing.T) {
	t.Parallel()
	pts := []*qdrant.ScoredPoint{{
		Id:      &qdrant.PointId{PointIdOptions: &qdrant.PointId_Uuid{Uuid: "11111111-1111-1111-1111-111111111111"}},
		Score:   0.4242,
		Payload: map[string]*qdrant.Value{"content": qdrant.NewValueString("x")},
	}}
	ms := memoriesFromPoints(pts)
	if len(ms) != 1 {
		t.Fatalf("got %d memories, want 1", len(ms))
	}
	if ms[0].Score != 0.4242 {
		t.Errorf("Score = %v, want 0.4242", ms[0].Score)
	}
}
