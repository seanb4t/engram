// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import "testing"

// TestRecordCeilingDerivesFromCaps pins the exact-arithmetic derivation
// (D-02's provability requirement): at DefaultRecordCaps(), the full-view
// ceiling is 970240 bytes and the summary-view ceiling 34304; with
// SummaryBytes: 0 the summary term becomes the content cap (1035264 /
// 99328); with ContentBytes: 4 << 20 the full ceiling exceeds rpcByteBudget
// and perRPCLimit returns 1; doubling ContentBytes raises the full ceiling
// by exactly the added bytes and leaves the summary ceiling unchanged. No
// Qdrant: exercises the pure derivation functions directly.
func TestRecordCeilingDerivesFromCaps(t *testing.T) {
	d := DefaultRecordCaps()
	if got := fullRecordCeiling(d); got != 970240 {
		t.Errorf("fullRecordCeiling(default) = %d, want 970240", got)
	}
	if got := summaryRecordCeiling(d); got != 34304 {
		t.Errorf("summaryRecordCeiling(default) = %d, want 34304", got)
	}
	if got := perRPCLimit(fullRecordCeiling(d)); got != 2 {
		t.Errorf("perRPCLimit(fullRecordCeiling(default)) = %d, want 2", got)
	}
	if got := perRPCLimit(summaryRecordCeiling(d)); got != 61 {
		t.Errorf("perRPCLimit(summaryRecordCeiling(default)) = %d, want 61", got)
	}

	noSummary := d
	noSummary.SummaryBytes = 0
	if got := fullRecordCeiling(noSummary); got != 1035264 {
		t.Errorf("fullRecordCeiling(SummaryBytes:0) = %d, want 1035264", got)
	}
	if got := summaryRecordCeiling(noSummary); got != 99328 {
		t.Errorf("summaryRecordCeiling(SummaryBytes:0) = %d, want 99328", got)
	}

	bigContent := d
	bigContent.ContentBytes = 4 << 20
	if got := fullRecordCeiling(bigContent); got <= rpcByteBudget {
		t.Fatalf("fullRecordCeiling(ContentBytes:4<<20) = %d, want > rpcByteBudget %d", got, rpcByteBudget)
	}
	if got := perRPCLimit(fullRecordCeiling(bigContent)); got != 1 {
		t.Errorf("perRPCLimit(fullRecordCeiling(ContentBytes:4<<20)) = %d, want 1", got)
	}

	doubled := d
	doubled.ContentBytes = d.ContentBytes * 2
	if got, want := fullRecordCeiling(doubled)-fullRecordCeiling(d), d.ContentBytes; got != want {
		t.Errorf("fullRecordCeiling delta on doubling ContentBytes = %d, want %d", got, want)
	}
	if got, want := summaryRecordCeiling(doubled), summaryRecordCeiling(d); got != want {
		t.Errorf("summaryRecordCeiling changed when only ContentBytes doubled: got %d, want unchanged %d", got, want)
	}
}

// TestWithRecordCapsNormalizesNonPositive proves normalizeRecordCaps (via
// WithRecordCaps and New's zero-value default): New(nil, ...).RecordCaps()
// reports DefaultRecordCaps(); every field 0 except a valid ContentBytes
// keeps that content value, keeps SummaryBytes == 0 (never defaulted), and
// defaults the rest; a negative SummaryBytes becomes the default 512. No
// Qdrant: New(nil, ...) never dials.
func TestWithRecordCapsNormalizesNonPositive(t *testing.T) {
	if got, want := New(nil, "x").RecordCaps(), DefaultRecordCaps(); got != want {
		t.Errorf("New(nil, x).RecordCaps() = %+v, want %+v", got, want)
	}

	partial := New(nil, "x", WithRecordCaps(RecordCaps{ContentBytes: 1234}))
	got := partial.RecordCaps()
	want := DefaultRecordCaps()
	want.ContentBytes = 1234
	want.SummaryBytes = 0
	if got != want {
		t.Errorf("RecordCaps() after WithRecordCaps({ContentBytes:1234}) = %+v, want %+v", got, want)
	}

	neg := New(nil, "x", WithRecordCaps(RecordCaps{SummaryBytes: -5}))
	if got, want := neg.RecordCaps().SummaryBytes, DefaultRecordCaps().SummaryBytes; got != want {
		t.Errorf("negative SummaryBytes normalized to %d, want default %d", got, want)
	}
}

// TestSweepLimit proves sweepLimit's own branching directly: a zero-value
// readView (no byte-derived ceiling — budgeted() reports false) always
// returns spineScrollBatch; a budgeted view's limit is capped by
// spineScrollBatch (forced to 1 here, restored via t.Cleanup — never
// t.Parallel with a mutated package var).
func TestSweepLimit(t *testing.T) {
	if got, want := sweepLimit(readView{}), spineScrollBatch; got != want {
		t.Errorf("sweepLimit(readView{}) = %d, want spineScrollBatch %d", got, want)
	}

	orig := spineScrollBatch
	t.Cleanup(func() { spineScrollBatch = orig })
	spineScrollBatch = 1

	full := readView{maxRecordBytes: fullRecordCeiling(DefaultRecordCaps())}
	if got := sweepLimit(full); got != 1 {
		t.Errorf("sweepLimit(fullView) with spineScrollBatch=1 = %d, want 1", got)
	}
	summary := readView{maxRecordBytes: summaryRecordCeiling(DefaultRecordCaps())}
	if got := sweepLimit(summary); got != 1 {
		t.Errorf("sweepLimit(summaryView) with spineScrollBatch=1 = %d, want 1", got)
	}
}
