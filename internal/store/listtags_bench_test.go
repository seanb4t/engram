// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

// Shaped after production on 2026-09-28: 4,101 records, ~26 tags each,
// 72,235 distinct tags of which 64,439 appear once, and a stored payload of
// median 4.1 KB (content median 2.5 KB, p90 5.1 KB).
const (
	benchTagsRecords = 4000
	benchTagsCommon  = 8    // per record, Zipf-drawn from benchTagsVocab
	benchTagsUnique  = 18   // per record, carried by no other record
	benchTagsVocab   = 2000 // shared vocabulary size
	benchTagsOwner   = "bench-owner-a"
)

// BenchmarkFacetTags compares the exact filtered Facet facetTags used to
// issue (#675) with its paged Scroll replacement, over a collection shaped
// like production. Set ENGRAM_BENCH_TAGS=1 to run it (task bench:tags).
func BenchmarkFacetTags(b *testing.B) {
	if os.Getenv("ENGRAM_BENCH_TAGS") == "" {
		b.Skip("set ENGRAM_BENCH_TAGS=1 to run (needs Qdrant)")
	}
	ctx := context.Background()
	collection := testCollection("bench_facet_tags")
	c := dialTestClient(b)
	_ = c.DeleteCollection(ctx, collection)
	b.Cleanup(func() { _ = c.DeleteCollection(ctx, collection) })
	s := newTestStore(b, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		b.Fatal(err)
	}
	seedBenchTags(b, s)

	f := s.recallVisibleFilter(ctx, "", Authenticated(benchTagsOwner))
	const limit = MaxRecallLimit

	// Compare untruncated: Facet breaks ties at the limit boundary its own way.
	all := uint64(benchTagsVocab + benchTagsRecords*benchTagsUnique)
	want, _, err := exactFacetTags(ctx, s, f, all)
	if err != nil {
		b.Fatal(err)
	}
	got, _, err := s.facetTags(ctx, f, all)
	if err != nil {
		b.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		b.Fatalf("facetTags disagrees with exact Facet: %d tags vs %d", len(got), len(want))
	}
	b.Logf("%d distinct recall-visible tags", len(got))

	b.Run("exact-facet", func(b *testing.B) {
		for b.Loop() {
			if _, _, err := exactFacetTags(ctx, s, f, limit); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("scroll-count", func(b *testing.B) {
		for b.Loop() {
			if _, _, err := s.facetTags(ctx, f, limit); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// exactFacetTags is the pre-#675 facetTags, kept as the benchmark baseline.
func exactFacetTags(ctx context.Context, s *Store, f *qdrant.Filter, limit uint64) ([]TagCount, bool, error) {
	hits, err := s.client.Facet(ctx, &qdrant.FacetCounts{
		CollectionName: s.collection,
		Key:            "tags",
		Filter:         f,
		Exact:          qdrant.PtrOf(true),
		Limit:          qdrant.PtrOf(limit + 1),
	})
	if err != nil {
		return nil, false, err
	}
	out := make([]TagCount, 0, len(hits))
	for _, h := range hits {
		out = append(out, TagCount{Tag: h.GetValue().GetStringValue(), Count: h.GetCount()})
	}
	sortTagCounts(out)
	more := uint64(len(out)) > limit
	if more {
		out = out[:limit]
	}
	return out, more, nil
}

// seedBenchTags writes benchTagsRecords records with a long-tail tag
// vocabulary across three owners, twelve scopes, shared and private visibility, and a
// mix of archived, superseded, expired and scheduled records.
func seedBenchTags(b *testing.B, s *Store) {
	b.Helper()
	ctx := context.Background()
	r := rand.New(rand.NewPCG(675, 675))
	zipf := rand.NewZipf(r, 1.1, 2, benchTagsVocab-1)
	owners := []string{benchTagsOwner, "bench-owner-b", "bench-owner-c"}
	now := time.Now().UTC()
	past, future := now.Add(-time.Hour), now.Add(time.Hour)
	superseder := "00000000-0000-0000-0000-000000000675"

	points := make([]*qdrant.PointStruct, 0, benchTagsRecords)
	for i := range benchTagsRecords {
		seen := map[string]bool{}
		var tags []string
		for len(tags) < benchTagsCommon {
			tag := fmt.Sprintf("tag-%04d", zipf.Uint64())
			if !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
		for j := range benchTagsUnique {
			tags = append(tags, fmt.Sprintf("r%04d-tag-%02d", i, j))
		}
		m := Memory{
			ID:        fmt.Sprintf("00000000-0000-4000-8000-%012d", i),
			Content:   strings.Repeat("x", 800+r.IntN(3400)),
			Summary:   strings.Repeat("s", 200),
			Scope:     fmt.Sprintf("repo:bench/%02d", i%12),
			Owner:     owners[i%len(owners)],
			Category:  "gotcha",
			Tags:      tags,
			CreatedAt: now.Add(-time.Duration(i) * time.Minute),
		}
		if i%2 == 0 {
			m.Visibility = visibilityShared
		}
		switch i % 20 {
		case 1:
			m.ArchivedAt = &past
		case 2:
			m.SupersededBy = &superseder
		case 3:
			m.NotAfter = &past
		case 4:
			m.NotBefore = &future
		}
		points = append(points, &qdrant.PointStruct{
			Id:      qdrant.NewID(m.ID),
			Vectors: qdrant.NewVectors(r.Float32(), r.Float32(), r.Float32()),
			Payload: qdrant.NewValueMap(payload(m)),
		})
	}
	for start := 0; start < len(points); start += 500 {
		end := min(start+500, len(points))
		if _, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: s.collection, Wait: qdrant.PtrOf(true), Points: points[start:end],
		}); err != nil {
			b.Fatal(err)
		}
	}
}
