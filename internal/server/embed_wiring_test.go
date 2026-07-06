// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// recordingEmbedder captures which embed method the handler called and with what
// input, then returns an error so the handler returns before touching the store
// (letting these assertions run without a live Qdrant).
type recordingEmbedder struct {
	method string
	input  string
}

var errStopBeforeStore = errors.New("stop before store")

func (r *recordingEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	r.method, r.input = "Embed", text
	return nil, errStopBeforeStore
}

func (r *recordingEmbedder) EmbedQuery(_ context.Context, text string) ([]float32, error) {
	r.method, r.input = "EmbedQuery", text
	return nil, errStopBeforeStore
}

// search_memory must embed the query via EmbedQuery (the instruction-aware,
// asymmetric path), not the document Embed path.
func TestSearchMemoryUsesEmbedQuery(t *testing.T) {
	rec := &recordingEmbedder{}
	d := &deps{em: rec}
	_, err := d.searchMemory(context.Background(), searchArgs{Query: "why did pushover fail", Scope: "s"})
	if !errors.Is(err, errStopBeforeStore) {
		t.Fatalf("expected stop-before-store error, got %v", err)
	}
	if rec.method != "EmbedQuery" {
		t.Errorf("search embedded via %q, want EmbedQuery", rec.method)
	}
}

// store_memory must fold the record's tags into the embedded document text so
// curated tags contribute to vector recall.
func TestStoreMemoryEmbedsContentPlusTags(t *testing.T) {
	rec := &recordingEmbedder{}
	d := &deps{em: rec}
	_, _, err := d.storeMemory(context.Background(), storeArgs{
		Content:  "musl getaddrinfo fails on NODATA",
		Scope:    "s",
		Source:   "user-said",
		Category: "gotcha",
		Tags:     []string{"dns", "musl"},
	})
	if !errors.Is(err, errStopBeforeStore) {
		t.Fatalf("expected stop-before-store error, got %v", err)
	}
	if rec.method != "Embed" {
		t.Errorf("store embedded via %q, want Embed", rec.method)
	}
	if !strings.Contains(rec.input, "musl getaddrinfo fails on NODATA") ||
		!strings.Contains(rec.input, "tags: dns, musl") {
		t.Errorf("embed input missing content or tags line: %q", rec.input)
	}
}
