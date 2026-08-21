// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMemoryStateWords(t *testing.T) {
	now := time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)
	past := now.Add(-24 * time.Hour)
	future := now.Add(24 * time.Hour)

	tests := []struct {
		name string
		mem  *engramv1.Memory
		want []string
	}{
		{
			name: "empty record",
			mem:  &engramv1.Memory{},
			want: nil,
		},
		{
			name: "archived only",
			mem:  &engramv1.Memory{ArchivedAt: timestamppb.New(now)},
			want: []string{"archived"},
		},
		{
			name: "superseded only",
			mem:  &engramv1.Memory{SupersededBy: strPtr("successor-id")},
			want: []string{"superseded"},
		},
		{
			name: "expired only (not_after strictly in the past)",
			mem:  &engramv1.Memory{NotAfter: timestamppb.New(past)},
			want: []string{"expired"},
		},
		{
			name: "scheduled only (not_before strictly in the future)",
			mem:  &engramv1.Memory{NotBefore: timestamppb.New(future)},
			want: []string{"scheduled"},
		},
		{
			name: "archived + superseded + expired compound, canonical order",
			mem: &engramv1.Memory{
				ArchivedAt:   timestamppb.New(now),
				SupersededBy: strPtr("successor-id"),
				NotAfter:     timestamppb.New(past),
			},
			want: []string{"archived", "superseded", "expired"},
		},
		{
			name: "order independence: same compound, fields set in reverse sequence",
			mem: &engramv1.Memory{
				NotAfter:     timestamppb.New(past),
				SupersededBy: strPtr("successor-id"),
				ArchivedAt:   timestamppb.New(now),
			},
			want: []string{"archived", "superseded", "expired"},
		},
		{
			name: "not_before == now yields NO word (inclusive Lte bound, already active)",
			mem:  &engramv1.Memory{NotBefore: timestamppb.New(now)},
			want: nil,
		},
		{
			name: "not_after == now yields expired (exclusive Gt bound, already expired)",
			mem:  &engramv1.Memory{NotAfter: timestamppb.New(now)},
			want: []string{"expired"},
		},
		{
			name: "expired and scheduled never co-occur: inverted window (not_after in the past AND not_before in the future) yields exactly [expired]",
			mem: &engramv1.Memory{
				NotAfter:  timestamppb.New(past),
				NotBefore: timestamppb.New(future),
			},
			want: []string{"expired"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := memoryStateWords(tt.mem, now)
			if !slices.Equal(got, tt.want) {
				t.Errorf("memoryStateWords() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMemoryStateCell(t *testing.T) {
	now := time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)

	if got := memoryStateCell(&engramv1.Memory{}, now); got != "" {
		t.Errorf("memoryStateCell(empty) = %q, want empty string", got)
	}

	compound := &engramv1.Memory{
		ArchivedAt:   timestamppb.New(now),
		SupersededBy: strPtr("successor-id"),
	}
	if got, want := memoryStateCell(compound, now), "archived,superseded"; got != want {
		t.Errorf("memoryStateCell(compound) = %q, want %q", got, want)
	}
}

func strPtr(s string) *string { return &s }

// TestRenderMemoryTableStateColumn pins the exact list/search header shape
// (D-12) and the STATE cell content per row: a live record's cell is empty,
// an archived+superseded record's cell is "archived,superseded", and a
// zero-row result still renders the header including STATE.
func TestRenderMemoryTableStateColumn(t *testing.T) {
	liveID := "AAAA111111"
	archivedID := "BBBB222222"
	mems := []*engramv1.Memory{
		{ShortId: liveID, Scope: "s", Category: "c"},
		{
			ShortId:      archivedID,
			Scope:        "s",
			Category:     "c",
			ArchivedAt:   timestamppb.Now(),
			SupersededBy: strPtr("successor-id"),
		},
	}

	t.Run("list header (no score)", func(t *testing.T) {
		var buf strings.Builder
		if err := renderMemoryTable(&buf, mems, false); err != nil {
			t.Fatalf("renderMemoryTable: %v", err)
		}
		lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
		if got, want := lines[0], "SHORT_ID    SCOPE  CATEGORY  STATE                SUMMARY"; got != want {
			t.Errorf("header = %q, want %q", got, want)
		}
		liveLine := lineContaining(t, buf.String(), liveID)
		if strings.Contains(liveLine, "archived") || strings.Contains(liveLine, "superseded") {
			t.Errorf("live row = %q, want an empty STATE cell", liveLine)
		}
		archivedLine := lineContaining(t, buf.String(), archivedID)
		if !strings.Contains(archivedLine, "archived,superseded") {
			t.Errorf("archived+superseded row = %q, want STATE cell %q", archivedLine, "archived,superseded")
		}
	})

	t.Run("search header (with score)", func(t *testing.T) {
		var buf strings.Builder
		if err := renderMemoryTable(&buf, mems, true); err != nil {
			t.Fatalf("renderMemoryTable: %v", err)
		}
		lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
		if got, want := lines[0], "SHORT_ID    SCOPE  CATEGORY  STATE                SCORE   SUMMARY"; got != want {
			t.Errorf("header = %q, want %q", got, want)
		}
	})

	t.Run("zero rows still render the header", func(t *testing.T) {
		var buf strings.Builder
		if err := renderMemoryTable(&buf, nil, false); err != nil {
			t.Fatalf("renderMemoryTable: %v", err)
		}
		got := strings.TrimRight(buf.String(), "\n")
		if got != "SHORT_ID  SCOPE  CATEGORY  STATE  SUMMARY" {
			t.Errorf("zero-row output = %q, want just the header", got)
		}
	})
}
