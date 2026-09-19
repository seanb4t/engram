// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

// This file proves the production link between the memory write caps
// (D-01/D-09/D-10, plan 03-01) and the store's read-side record ceiling
// (D-02, plan 03-02): recordCapsFromConfig builds a store.RecordCaps from
// EXACTLY the same parsers buildDepsFromEnv uses for the write caps, and
// storeFromConfig — the one production store construction site serve,
// reindex, migrate, and prune all funnel through — passes it to
// store.WithRecordCaps. TestDefaultRecordCapsMatchRegistryDefaults is the
// drift gate binding the store's defaults to the registry defaults and the
// server's citation constants, so the two can never silently diverge.

import (
	"testing"

	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)

// TestRecordCapsFromConfig proves recordCapsFromConfig builds a
// store.RecordCaps from the configured memory write caps plus
// maxMemorySummaryBytes/maxDiscoveryCitations/maxCitationExcerptBytes.
func TestRecordCapsFromConfig(t *testing.T) {
	t.Setenv("ENGRAM_MEMORY_MAX_CONTENT_BYTES", "1000")
	t.Setenv("ENGRAM_MEMORY_MAX_SUMMARY_BYTES", "0")
	t.Setenv("ENGRAM_MEMORY_MAX_TAGS", "3")
	t.Setenv("ENGRAM_MEMORY_MAX_TAG_BYTES", "9")

	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	got := recordCapsFromConfig(cfg)
	want := store.RecordCaps{
		ContentBytes: 1000, SummaryBytes: 0, Tags: 3, TagBytes: 9,
		Citations: maxDiscoveryCitations, CitationExcerptBytes: maxCitationExcerptBytes,
	}
	if got != want {
		t.Errorf("recordCapsFromConfig = %+v, want %+v", got, want)
	}
}

// TestDefaultRecordCapsMatchRegistryDefaults is the drift gate binding the
// store's DefaultRecordCaps() to the registry's own defaults and the
// server's citation constants — with the four env vars cleared,
// recordCapsFromConfig(cfg) must equal store.DefaultRecordCaps() field for
// field.
func TestDefaultRecordCapsMatchRegistryDefaults(t *testing.T) {
	t.Setenv("ENGRAM_MEMORY_MAX_CONTENT_BYTES", "")
	t.Setenv("ENGRAM_MEMORY_MAX_SUMMARY_BYTES", "")
	t.Setenv("ENGRAM_MEMORY_MAX_TAGS", "")
	t.Setenv("ENGRAM_MEMORY_MAX_TAG_BYTES", "")

	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	got := recordCapsFromConfig(cfg)
	want := store.DefaultRecordCaps()
	if got != want {
		t.Errorf("recordCapsFromConfig(defaults) = %+v, want store.DefaultRecordCaps() %+v", got, want)
	}
}

// TestStoreFromConfigCarriesRecordCaps proves the ONE production store
// construction site actually wires WithRecordCaps: a Store built by
// storeFromConfig from a config carrying custom caps reports those exact
// caps from RecordCaps(), not store.DefaultRecordCaps().
func TestStoreFromConfigCarriesRecordCaps(t *testing.T) {
	addr := storetest.Addr()
	if addr == "" {
		storetest.SkipOrFailNoQdrant(t)
	}
	t.Setenv("ENGRAM_QDRANT_ADDR", addr)
	t.Setenv("ENGRAM_QDRANT_COLLECTION", testCollection("mem_recordcaps_test"))
	t.Setenv("ENGRAM_EMBED_DIM", "3")
	t.Setenv("ENGRAM_MEMORY_MAX_CONTENT_BYTES", "1000")
	t.Setenv("ENGRAM_MEMORY_MAX_SUMMARY_BYTES", "0")
	t.Setenv("ENGRAM_MEMORY_MAX_TAGS", "3")
	t.Setenv("ENGRAM_MEMORY_MAX_TAG_BYTES", "9")

	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	st, _, err := storeFromConfig(cfg)
	if err != nil {
		t.Fatalf("storeFromConfig: %v", err)
	}

	want := store.RecordCaps{
		ContentBytes: 1000, SummaryBytes: 0, Tags: 3, TagBytes: 9,
		Citations: maxDiscoveryCitations, CitationExcerptBytes: maxCitationExcerptBytes,
	}
	if got := st.RecordCaps(); got != want {
		t.Errorf("storeFromConfig(cfg).RecordCaps() = %+v, want %+v", got, want)
	}
}
