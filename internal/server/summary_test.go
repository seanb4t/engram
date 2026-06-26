// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"errors"
	"testing"

	"github.com/seanb4t/engram/internal/store"
)

func sp(s string) *string { return &s }

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
