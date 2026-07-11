// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/seanb4t/engram/internal/store"
)

func sp(s string) *string { return &s }

func TestToRecallViewPrefersSummaryElseTruncates(t *testing.T) {
	withSum := store.Memory{ID: "1", Content: "a very long body well past the cap", Summary: "kept", SummarySource: store.SummarySourceClient, Scope: "s", Category: "decision"}
	v := toRecallView(withSum, 8)
	if v.Summary != "kept" || v.Truncated {
		t.Fatalf("summary should win untruncated: %+v", v)
	}
	noSum := store.Memory{ID: "2", Content: "abcdefghijklmnop", Scope: "s", Category: "gotcha"}
	v2 := toRecallView(noSum, 8)
	if !v2.Truncated || len([]rune(v2.Summary)) > 9 { // 8 + ellipsis
		t.Fatalf("long no-summary should truncate: %+v", v2)
	}
	short := store.Memory{ID: "3", Content: "tiny", Scope: "s", Category: "gotcha"}
	if v3 := toRecallView(short, 8); v3.Truncated || v3.Summary != "tiny" {
		t.Fatalf("short content returned as-is: %+v", v3)
	}
}

func TestShapeRecallFullVsSummary(t *testing.T) {
	ms := []store.Memory{{ID: "1", Content: "loooooong content over cap", Scope: "s", Category: "decision"}}
	full := shapeRecall(ms, true, 4)
	if _, ok := full[0].(store.Memory); !ok {
		t.Fatalf("full=true must yield store.Memory, got %T", full[0])
	}
	compact := shapeRecall(ms, false, 4)
	if _, ok := compact[0].(recallView); !ok {
		t.Fatalf("full=false must yield recallView, got %T", compact[0])
	}
}

func TestRecallViewCarriesShortID(t *testing.T) {
	v := toRecallView(store.Memory{ID: "u", ShortID: "j7k2m9p4x0", Content: "hello", Scope: "s", Category: "gotcha"}, 8)
	if v.ShortID != "j7k2m9p4x0" {
		t.Fatalf("recallView.ShortID = %q", v.ShortID)
	}
}

// TestToRecallViewSurfacesUsageSignals is a Phase 12 D-07 regression guard:
// recallView is a hand-written allow-list, so a store.Memory carrying
// AccessCount/LastAccessedAt is not enough on its own — toRecallView must
// explicitly copy both fields onto the compact list/search shape.
func TestToRecallViewSurfacesUsageSignals(t *testing.T) {
	last := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	m := store.Memory{
		ID: "u", Content: "hello", Scope: "s", Category: "gotcha",
		AccessCount: 5, LastAccessedAt: &last,
	}
	v := toRecallView(m, 8)
	if v.AccessCount != 5 {
		t.Fatalf("recallView.AccessCount = %d, want 5", v.AccessCount)
	}
	if v.LastAccessedAt == nil || !v.LastAccessedAt.Equal(last) {
		t.Fatalf("recallView.LastAccessedAt = %v, want %v", v.LastAccessedAt, last)
	}

	// A never-accessed record must leave LastAccessedAt nil so the
	// json:",omitempty" tag actually omits it (no bogus 0001-01-01 stamp).
	nv := toRecallView(store.Memory{ID: "n", Content: "x", Scope: "s", Category: "gotcha"}, 8)
	if nv.LastAccessedAt != nil {
		t.Fatalf("never-accessed recallView.LastAccessedAt = %v, want nil (omitted)", nv.LastAccessedAt)
	}
}

// TestEmbedderIdentityNeverOnRecallWire is a D-06 regression guard (review
// round-1 HIGH blocker + round-2 confirmation): store.Memory.EmbedderIdentity
// is payload-only (json:"-") and must NEVER surface on the recall wire — at
// shapeRecall's full=true path (which returns store.Memory verbatim) AND at
// the compact toRecallView shape (whose recallView allow-list must not gain
// the field). A toRecallView-only assertion would be insufficient on its
// own — this test also covers the verbatim full path.
func TestEmbedderIdentityNeverOnRecallWire(t *testing.T) {
	sentinel := "v1:deadbeefdeadbeef"
	m := store.Memory{ID: "u", Content: "hello", Scope: "s", Category: "gotcha", EmbedderIdentity: sentinel}

	full := shapeRecall([]store.Memory{m}, true, 8)
	fullJSON, err := json.Marshal(full[0])
	if err != nil {
		t.Fatalf("marshal shapeRecall(full=true) result: %v", err)
	}
	if strings.Contains(string(fullJSON), "embedder_identity") || strings.Contains(string(fullJSON), sentinel) {
		t.Fatalf("shapeRecall(full=true) leaked embedder identity onto the wire: %s", fullJSON)
	}

	compact := toRecallView(m, 8)
	compactJSON, err := json.Marshal(compact)
	if err != nil {
		t.Fatalf("marshal toRecallView result: %v", err)
	}
	if strings.Contains(string(compactJSON), "embedder_identity") || strings.Contains(string(compactJSON), sentinel) {
		t.Fatalf("toRecallView leaked embedder identity onto the wire: %s", compactJSON)
	}
}

func TestResolveSummaryUpdate(t *testing.T) {
	clientSum := store.Memory{Summary: "hand-written", SummarySource: store.SummarySourceClient}
	autoSum := store.Memory{Summary: "machine", SummarySource: store.SummarySourceAuto}
	none := store.Memory{}

	cases := []struct {
		name           string
		cur            store.Memory
		contentChanged bool
		arg            *string
		wantValue      string
		wantApply      bool
		wantErr        bool
	}{
		{"explicit set", clientSum, true, sp("new"), "new", true, false},
		{"explicit clear", clientSum, true, sp(""), "", true, false},
		{"unchanged preserves", clientSum, false, nil, "", false, false},
		{"none + change = noop", none, true, nil, "", false, false},
		{"auto + change = autoclear", autoSum, true, nil, "", true, false},
		{"client + change + unaddressed = reject", clientSum, true, nil, "", false, true},
		{"legacy (empty source) + change = preserve", store.Memory{Summary: "old", SummarySource: store.SummarySourceNone}, true, nil, "", false, false},
	}
	for _, tc := range cases {
		v, apply, err := resolveSummaryUpdate(tc.cur, tc.contentChanged, tc.arg)
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: err=%v wantErr=%v", tc.name, err, tc.wantErr)
			continue
		}
		if tc.wantErr {
			if !errors.Is(err, errStaleSummary) {
				t.Errorf("%s: want errStaleSummary, got %v", tc.name, err)
			}
			continue
		}
		if v != tc.wantValue || apply != tc.wantApply {
			t.Errorf("%s: got (%q,%v) want (%q,%v)", tc.name, v, apply, tc.wantValue, tc.wantApply)
		}
	}
}
