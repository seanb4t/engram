// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// --- Visibility enum <-> shared bool (SetVisibility path ONLY, round-8 MED) ---

func TestProtoconvVisibilityToShared(t *testing.T) {
	cases := []struct {
		name string
		in   engramv1.Visibility
		want bool
	}{
		{"shared", engramv1.Visibility_VISIBILITY_SHARED, true},
		{"private", engramv1.Visibility_VISIBILITY_PRIVATE, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := visibilityToShared(tc.in); got != tc.want {
				t.Errorf("visibilityToShared(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestProtoconvSetVisibilityRequestToArgs(t *testing.T) {
	req := &engramv1.SetVisibilityRequest{Id: "mem-1", Visibility: engramv1.Visibility_VISIBILITY_SHARED}
	got := setVisibilityRequestToArgs(req)
	want := setVisibilityArgs{ID: "mem-1", Shared: true}
	if got != want {
		t.Errorf("setVisibilityRequestToArgs = %+v, want %+v", got, want)
	}
}

// --- UpdateMemory mask-driven mapping (landmine 2 nil-Content + round-8 bool/enum fix) ---

func TestProtoconvUpdateMemoryRequestToArgs(t *testing.T) {
	t.Run("tags-only mask leaves content/shared/summary nil", func(t *testing.T) {
		req := &engramv1.UpdateMemoryRequest{
			Id:         "mem-1",
			Content:    "should not leak through",
			Tags:       []string{"a", "b"},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"tags"}},
		}
		got := updateMemoryRequestToArgs(req)
		if got.ID != "mem-1" {
			t.Errorf("ID = %q, want mem-1", got.ID)
		}
		if got.Content != nil {
			t.Errorf("Content = %v, want nil (absent from mask)", *got.Content)
		}
		if got.Shared != nil {
			t.Errorf("Shared = %v, want nil (absent from mask)", *got.Shared)
		}
		if got.Summary != nil {
			t.Errorf("Summary = %v, want nil (absent from mask)", *got.Summary)
		}
		if got.Tags == nil || len(*got.Tags) != 2 || (*got.Tags)[0] != "a" || (*got.Tags)[1] != "b" {
			t.Errorf("Tags = %v, want non-nil [a b]", got.Tags)
		}
	})

	t.Run("content+summary mask leaves tags/shared nil", func(t *testing.T) {
		req := &engramv1.UpdateMemoryRequest{
			Id:         "mem-2",
			Content:    "new content",
			Summary:    "new summary",
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"content", "summary"}},
		}
		got := updateMemoryRequestToArgs(req)
		if got.Content == nil || *got.Content != "new content" {
			t.Errorf("Content = %v, want non-nil %q", got.Content, "new content")
		}
		if got.Summary == nil || *got.Summary != "new summary" {
			t.Errorf("Summary = %v, want non-nil %q", got.Summary, "new summary")
		}
		if got.Tags != nil {
			t.Errorf("Tags = %v, want nil (absent from mask)", *got.Tags)
		}
		if got.Shared != nil {
			t.Errorf("Shared = %v, want nil (absent from mask)", *got.Shared)
		}
	})

	// round-8 MED (Codex): UpdateMemoryRequest.shared is a proto bool
	// (engram.proto:168), not the Visibility enum. The presence-sensitive
	// false case is the one most likely to regress into a dropped update.
	t.Run("shared mask with false yields non-nil *bool(false), not the enum mapper", func(t *testing.T) {
		req := &engramv1.UpdateMemoryRequest{
			Id:         "mem-3",
			Shared:     false,
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"shared"}},
		}
		got := updateMemoryRequestToArgs(req)
		if got.Shared == nil {
			t.Fatalf("Shared = nil, want non-nil *bool(false) (presence-sensitive false case)")
		}
		if *got.Shared {
			t.Errorf("*Shared = true, want false")
		}
		if got.Content != nil || got.Tags != nil || got.Summary != nil {
			t.Errorf("other fields leaked: Content=%v Tags=%v Summary=%v", got.Content, got.Tags, got.Summary)
		}
	})

	t.Run("shared mask with true yields non-nil *bool(true)", func(t *testing.T) {
		req := &engramv1.UpdateMemoryRequest{
			Id:         "mem-4",
			Shared:     true,
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"shared"}},
		}
		got := updateMemoryRequestToArgs(req)
		if got.Shared == nil || !*got.Shared {
			t.Errorf("Shared = %v, want non-nil *bool(true)", got.Shared)
		}
	})
}

// --- Citation <-> citationArg ---

func TestProtoconvCitationToArg(t *testing.T) {
	c := &engramv1.Citation{
		Kind: "file", Ref: "internal/server/tools.go", Locator: "200-240",
		Pin: "abc123", Excerpt: "func parseWindow...",
	}
	got := citationToArg(c)
	want := citationArg{
		Kind: "file", Ref: "internal/server/tools.go", Locator: "200-240",
		Pin: "abc123", Excerpt: "func parseWindow...",
	}
	if got != want {
		t.Errorf("citationToArg = %+v, want %+v", got, want)
	}
}

func TestProtoconvCitationsToArgs(t *testing.T) {
	cs := []*engramv1.Citation{
		{Kind: "file", Ref: "a.go"},
		{Kind: "url", Ref: "https://example.com"},
	}
	got := citationsToArgs(cs)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Kind != "file" || got[0].Ref != "a.go" {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].Kind != "url" || got[1].Ref != "https://example.com" {
		t.Errorf("got[1] = %+v", got[1])
	}
}

// --- StoreMemory/StoreDiscovery request -> args (exact field mapping) ---

func TestProtoconvStoreMemoryRequestToArgs(t *testing.T) {
	req := &engramv1.StoreMemoryRequest{
		Content: "c", Scope: "s", Source: "src", Category: "decision",
		Tags: []string{"t1"}, Repo: "r", Workspace: "w", Worktree: "wt",
		BaseDir: "bd", Summary: "sum",
	}
	got := storeMemoryRequestToArgs(req)
	if got.Content != "c" || got.Scope != "s" || got.Source != "src" ||
		got.Category != "decision" || got.Repo != "r" || got.Workspace != "w" ||
		got.Worktree != "wt" || got.BaseDir != "bd" || got.Summary != "sum" {
		t.Errorf("storeMemoryRequestToArgs = %+v", got)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "t1" {
		t.Errorf("Tags = %v, want [t1]", got.Tags)
	}
}

func TestProtoconvStoreDiscoveryRequestToArgs(t *testing.T) {
	req := &engramv1.StoreDiscoveryRequest{
		Content: "understanding", Kind: "fact",
		Citations: []*engramv1.Citation{{Kind: "file", Ref: "x.go"}},
		Scope:     "discovery:repo:engram",
		Tags:      []string{"t"},
		Summary:   "sum",
		Id:        "existing-id",
	}
	got := storeDiscoveryRequestToArgs(req)
	if got.Content != "understanding" || got.Kind != "fact" || got.Scope != "discovery:repo:engram" ||
		got.Summary != "sum" || got.ID != "existing-id" {
		t.Errorf("storeDiscoveryRequestToArgs = %+v", got)
	}
	if len(got.Citations) != 1 || got.Citations[0].Kind != "file" || got.Citations[0].Ref != "x.go" {
		t.Errorf("Citations = %+v", got.Citations)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "t" {
		t.Errorf("Tags = %v", got.Tags)
	}
}

// --- Timestamp -> scheduling-window string (outward rounding, round-8 MED) ---

func TestProtoconvWindowBoundNil(t *testing.T) {
	if got := windowBoundFloor(nil); got != "" {
		t.Errorf("windowBoundFloor(nil) = %q, want empty", got)
	}
	if got := windowBoundCeil(nil); got != "" {
		t.Errorf("windowBoundCeil(nil) = %q, want empty", got)
	}
}

func TestProtoconvWindowBoundFloorsAndCeils(t *testing.T) {
	// A sub-second instant: not_before rounds DOWN, not_after rounds UP.
	sub := time.Date(2026, 7, 12, 10, 0, 0, 500_000_000, time.UTC) // :00.5
	floor := windowBoundFloor(timestamppb.New(sub))
	ceil := windowBoundCeil(timestamppb.New(sub))

	wantFloor := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	wantCeil := time.Date(2026, 7, 12, 10, 0, 1, 0, time.UTC).Format(time.RFC3339Nano)

	if floor != wantFloor {
		t.Errorf("windowBoundFloor = %q, want %q", floor, wantFloor)
	}
	if ceil != wantCeil {
		t.Errorf("windowBoundCeil = %q, want %q", ceil, wantCeil)
	}
	if floor == ceil {
		t.Errorf("floor and ceil collapsed to the same string: %q (ordering not preserved)", floor)
	}
}

func TestProtoconvWindowBoundOnBoundaryIsUnchanged(t *testing.T) {
	// Already on a whole-second boundary: neither floor nor ceil should shift it.
	whole := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	want := whole.Format(time.RFC3339Nano)

	if got := windowBoundFloor(timestamppb.New(whole)); got != want {
		t.Errorf("windowBoundFloor(on-boundary) = %q, want %q (no spurious shift)", got, want)
	}
	if got := windowBoundCeil(timestamppb.New(whole)); got != want {
		t.Errorf("windowBoundCeil(on-boundary) = %q, want %q (no spurious +1s)", got, want)
	}
}

func TestProtoconvWindowBoundUTCAndOffsetAgree(t *testing.T) {
	// google.protobuf.Timestamp is an absolute instant (no wall-clock offset
	// survives proto encoding); a Time constructed in a non-UTC location and
	// one constructed directly in UTC for the same instant must format to the
	// same rounded bound.
	loc := time.FixedZone("UTC-5", -5*3600)
	inOffset := time.Date(2026, 7, 12, 5, 0, 0, 250_000_000, loc) // 10:00:00.25 UTC
	inUTC := time.Date(2026, 7, 12, 10, 0, 0, 250_000_000, time.UTC)

	got := windowBoundFloor(timestamppb.New(inOffset))
	want := windowBoundFloor(timestamppb.New(inUTC))
	if got != want {
		t.Errorf("offset instant floored to %q, want %q (same absolute instant)", got, want)
	}
}

// round-8 MED (Codex): the store floors both window bounds to integer Unix
// seconds on encode/decode (store.go:320/:323/:406/:410). Without outward
// rounding, a not_after ~500ms in the future would pass parseWindow's
// future-check (tools.go:464) but persist at the START of the current second
// and be immediately expired. This test asserts the FLOORED-back-to-Unix-
// second value of the produced bound is strictly after "now" — the
// resolution, not merely that parseWindow accepts the formatted string.
func TestProtoconvNotAfterNearFutureSurvivesStoreFlooring(t *testing.T) {
	now := time.Now().UTC()
	nearFuture := now.Add(500 * time.Millisecond)

	bound := windowBoundCeil(timestamppb.New(nearFuture))
	parsed, err := time.Parse(time.RFC3339Nano, bound)
	if err != nil {
		t.Fatalf("time.Parse(%q): %v", bound, err)
	}

	// Simulate the store's .Unix() flooring (store.go:406/:410).
	storeFloored := time.Unix(parsed.Unix(), 0).UTC()

	if !storeFloored.After(now) {
		t.Errorf("store-floored bound %v is not strictly after now %v (silent immediate-expiry)", storeFloored, now)
	}
}

func TestProtoconvWindowBoundOrderingPreserved(t *testing.T) {
	notBefore := time.Date(2026, 7, 12, 10, 0, 0, 100_000_000, time.UTC)
	notAfter := time.Date(2026, 7, 12, 10, 0, 0, 900_000_000, time.UTC)

	nb := windowBoundFloor(timestamppb.New(notBefore))
	na := windowBoundCeil(timestamppb.New(notAfter))

	nbT, err := time.Parse(time.RFC3339Nano, nb)
	if err != nil {
		t.Fatalf("parse not_before: %v", err)
	}
	naT, err := time.Parse(time.RFC3339Nano, na)
	if err != nil {
		t.Fatalf("parse not_after: %v", err)
	}
	if !nbT.Before(naT) {
		t.Errorf("floor(not_before)=%v is not before ceil(not_after)=%v", nbT, naT)
	}
}

func TestProtoconvScheduleMemoryRequestToArgs(t *testing.T) {
	nb := time.Date(2026, 7, 12, 10, 0, 0, 500_000_000, time.UTC)
	na := time.Date(2026, 7, 12, 11, 0, 0, 500_000_000, time.UTC)
	req := &engramv1.ScheduleMemoryRequest{
		Content: "c", Scope: "s", Source: "src", Category: "decision",
		Tags: []string{"t1"}, Repo: "r", Workspace: "w", Worktree: "wt",
		BaseDir: "bd", Summary: "sum",
		NotBefore: timestamppb.New(nb),
		NotAfter:  timestamppb.New(na),
	}
	got := scheduleMemoryRequestToArgs(req)

	if got.Content != "c" || got.Scope != "s" || got.Category != "decision" {
		t.Errorf("base storeArgs fields not mapped: %+v", got.storeArgs)
	}
	wantNB := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	wantNA := time.Date(2026, 7, 12, 11, 0, 1, 0, time.UTC).Format(time.RFC3339Nano)
	if got.NotBefore != wantNB {
		t.Errorf("NotBefore = %q, want %q (floored)", got.NotBefore, wantNB)
	}
	if got.NotAfter != wantNA {
		t.Errorf("NotAfter = %q, want %q (ceil'd)", got.NotAfter, wantNA)
	}
}

func TestProtoconvScheduleMemoryRequestToArgsNilBounds(t *testing.T) {
	req := &engramv1.ScheduleMemoryRequest{Content: "c", Scope: "s"}
	got := scheduleMemoryRequestToArgs(req)
	if got.NotBefore != "" || got.NotAfter != "" {
		t.Errorf("nil bounds mapped to non-empty strings: NotBefore=%q NotAfter=%q", got.NotBefore, got.NotAfter)
	}
}

// --- mutationResult / ids -> proto response mapping ---

func TestProtoconvResultToResponse(t *testing.T) {
	r := mutationResult{ID: "uuid-1", ShortID: "SHORT01"}

	upd := mutationResultToUpdateMemoryResponse(r)
	if upd.GetId() != "uuid-1" || upd.GetShortId() != "SHORT01" {
		t.Errorf("mutationResultToUpdateMemoryResponse = %+v", upd)
	}

	vis := mutationResultToSetVisibilityResponse(r)
	if vis.GetId() != "uuid-1" || vis.GetShortId() != "SHORT01" {
		t.Errorf("mutationResultToSetVisibilityResponse = %+v", vis)
	}
}

func TestProtoconvIDsToResponses(t *testing.T) {
	storeResp := idsToStoreMemoryResponse("id-1", "short-1")
	if storeResp.GetId() != "id-1" || storeResp.GetShortId() != "short-1" {
		t.Errorf("idsToStoreMemoryResponse = %+v", storeResp)
	}

	schedResp := idsToScheduleMemoryResponse("id-2", "short-2")
	if schedResp.GetId() != "id-2" || schedResp.GetShortId() != "short-2" {
		t.Errorf("idsToScheduleMemoryResponse = %+v", schedResp)
	}

	discResp := idsToStoreDiscoveryResponse("id-3", "short-3")
	if discResp.GetId() != "id-3" || discResp.GetShortId() != "short-3" {
		t.Errorf("idsToStoreDiscoveryResponse = %+v", discResp)
	}
}
