// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"slices"
	"testing"
)

func TestCursorRoundTrip(t *testing.T) {
	in := listCursor{C: "2026-06-27T12:00:00Z", Seen: []string{"id-1", "id-2"}}
	tok := encodeCursor(in)
	if tok == "" {
		t.Fatal("encodeCursor returned empty")
	}
	out, err := decodeCursor(tok)
	if err != nil {
		t.Fatalf("decodeCursor: %v", err)
	}
	if out.C != in.C || !slices.Equal(out.Seen, in.Seen) {
		t.Errorf("round-trip mismatch: got %+v want %+v", out, in)
	}
}

func TestDecodeCursorRejectsGarbage(t *testing.T) {
	if _, err := decodeCursor("!!!not-base64!!!"); err == nil {
		t.Error("decodeCursor accepted non-base64 garbage; want error")
	}
}
