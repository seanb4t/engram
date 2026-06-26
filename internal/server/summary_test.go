// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"errors"
	"testing"

	"github.com/seanb4t/engram/internal/store"
)

func sp(s string) *string { return &s }

func TestToRecallViewPrefersSummaryElseTruncates(t *testing.T) {
	withSum := store.Memory{ID: "1", Content: "a very long body well past the cap", Summary: "kept", SummarySource: "client", Scope: "s", Category: "decision"}
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

func TestResolveSummaryUpdate(t *testing.T) {
	clientSum := store.Memory{Summary: "hand-written", SummarySource: "client"}
	autoSum := store.Memory{Summary: "machine", SummarySource: "auto"}
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
