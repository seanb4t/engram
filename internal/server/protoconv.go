// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"slices"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// protoconv is the D-09 conversion layer: every write RPC proto request ->
// internal *Args mapping and every write-result -> proto response mapping
// lives here, so the six write handlers (17-04) stay thin adapters (identity
// resolve -> protoconv -> one deps.* call -> protoconv -> response). protoconv
// does not re-validate anything the Phase-15 protovalidate interceptor already
// enforces (mask presence/allowlist, enum zero value) — V5.

// visibilityToShared maps the Visibility enum to the internal `shared` bool.
// Used ONLY by the SetVisibility path (SetVisibilityRequest.visibility is the
// Visibility enum, engram.proto:185); the UpdateMemory `shared` path is a
// plain proto bool (engram.proto:168) and never goes through this mapper —
// see updateMemoryRequestToArgs.
func visibilityToShared(v engramv1.Visibility) bool {
	return v == engramv1.Visibility_VISIBILITY_SHARED
}

func setVisibilityRequestToArgs(req *engramv1.SetVisibilityRequest) setVisibilityArgs {
	return setVisibilityArgs{
		ID:     req.GetId(),
		Shared: visibilityToShared(req.GetVisibility()),
	}
}

// updateMemoryRequestToArgs converts an UpdateMemoryRequest into updateArgs,
// populating a pointer field ONLY when its path is present in
// req.update_mask.paths (already CEL-validated upstream: non-empty,
// allowlisted to {content,shared,tags,summary} — not re-validated here, V5).
// An absent `content` path yields a nil Content (landmine 2: preserves
// cur.Content in deps.updateMemory, never silently blanks it). A present
// `shared` path maps updateArgs.Shared = &req.Shared (proto bool -> *bool,
// round-8 MED, Codex) — NOT the Visibility enum mapper, which is reserved to
// the SetVisibility path: UpdateMemoryRequest has no Visibility field
// (engram.proto:168 declares `shared` a plain bool).
func updateMemoryRequestToArgs(req *engramv1.UpdateMemoryRequest) updateArgs {
	paths := req.GetUpdateMask().GetPaths()
	a := updateArgs{ID: req.GetId()}
	if slices.Contains(paths, "content") {
		c := req.GetContent()
		a.Content = &c
	}
	if slices.Contains(paths, "shared") {
		s := req.GetShared()
		a.Shared = &s
	}
	if slices.Contains(paths, "tags") {
		tags := req.GetTags()
		a.Tags = &tags
	}
	if slices.Contains(paths, "summary") {
		s := req.GetSummary()
		a.Summary = &s
	}
	return a
}

// citationToArg / citationsToArgs convert the proto Citation into the
// internal citationArg shape used by storeDiscoveryArgs.
func citationToArg(c *engramv1.Citation) citationArg {
	return citationArg{
		Kind:    c.GetKind(),
		Ref:     c.GetRef(),
		Locator: c.GetLocator(),
		Pin:     c.GetPin(),
		Excerpt: c.GetExcerpt(),
	}
}

func citationsToArgs(cs []*engramv1.Citation) []citationArg {
	out := make([]citationArg, len(cs))
	for i, c := range cs {
		out[i] = citationToArg(c)
	}
	return out
}

func storeMemoryRequestToArgs(req *engramv1.StoreMemoryRequest) storeArgs {
	return storeArgs{
		Content:   req.GetContent(),
		Scope:     req.GetScope(),
		Source:    req.GetSource(),
		Category:  req.GetCategory(),
		Tags:      req.GetTags(),
		Repo:      req.GetRepo(),
		Workspace: req.GetWorkspace(),
		Worktree:  req.GetWorktree(),
		BaseDir:   req.GetBaseDir(),
		Summary:   req.GetSummary(),
	}
}

func storeDiscoveryRequestToArgs(req *engramv1.StoreDiscoveryRequest) storeDiscoveryArgs {
	return storeDiscoveryArgs{
		Content:   req.GetContent(),
		Kind:      req.GetKind(),
		Citations: citationsToArgs(req.GetCitations()),
		Scope:     req.GetScope(),
		Tags:      req.GetTags(),
		Summary:   req.GetSummary(),
		ID:        req.GetId(),
	}
}

// scheduleMemoryRequestToArgs converts a (D-05 flattened) ScheduleMemoryRequest
// into scheduleArgs. NotBefore/NotAfter are formatted via windowBoundFloor/
// windowBoundCeil so the existing parseWindow (tools.go:452) is fed a
// whole-second RFC3339Nano string unchanged (round-8 MED).
func scheduleMemoryRequestToArgs(req *engramv1.ScheduleMemoryRequest) scheduleArgs {
	return scheduleArgs{
		storeArgs: storeArgs{
			Content:   req.GetContent(),
			Scope:     req.GetScope(),
			Source:    req.GetSource(),
			Category:  req.GetCategory(),
			Tags:      req.GetTags(),
			Repo:      req.GetRepo(),
			Workspace: req.GetWorkspace(),
			Worktree:  req.GetWorktree(),
			BaseDir:   req.GetBaseDir(),
			Summary:   req.GetSummary(),
		},
		NotBefore: windowBoundFloor(req.GetNotBefore()),
		NotAfter:  windowBoundCeil(req.GetNotAfter()),
	}
}

// windowBoundFloor / windowBoundCeil format a *timestamppb.Timestamp as a
// scheduling-window bound string for parseWindow (tools.go:452), rounding the
// bound OUTWARD to a whole second BEFORE formatting (round-8 MED, Codex):
// not_before rounds DOWN (never advances the reveal time), not_after rounds
// UP (never truncates an expiry into the past). This keeps the store's
// second-granular `.Unix()` flooring on encode/decode (store.go:320/:323/
// :406/:410) a no-op on the value protoconv hands it — a sub-second
// `not_after` is WIDENED to the containing whole-second window instead of
// silently collapsing to immediate-expiry. time.RFC3339Nano is used (the
// plain-second RFC3339 layout truncates fractional seconds) so the rounded
// whole-second value round-trips exactly. A nil timestamp maps to "" (no
// window bound).
func windowBoundFloor(ts *timestamppb.Timestamp) string {
	return formatWindowBound(ts, false)
}

func windowBoundCeil(ts *timestamppb.Timestamp) string {
	return formatWindowBound(ts, true)
}

func formatWindowBound(ts *timestamppb.Timestamp, roundUp bool) string {
	if ts == nil {
		return ""
	}
	t := ts.AsTime()
	bound := t.Truncate(time.Second)
	if roundUp && bound.Before(t) {
		bound = bound.Add(time.Second)
	}
	return bound.Format(time.RFC3339Nano)
}

// mutationResultToUpdateMemoryResponse / mutationResultToSetVisibilityResponse
// map 17-02's mutationResult{ID, ShortID} into the by-id write responses —
// the handler does NOT re-fetch (17-REVIEWS.md response-conversion MEDIUM).
func mutationResultToUpdateMemoryResponse(r mutationResult) *engramv1.UpdateMemoryResponse {
	return &engramv1.UpdateMemoryResponse{Id: r.ID, ShortId: r.ShortID}
}

func mutationResultToSetVisibilityResponse(r mutationResult) *engramv1.SetVisibilityResponse {
	return &engramv1.SetVisibilityResponse{Id: r.ID, ShortId: r.ShortID}
}

// idsToStoreMemoryResponse / idsToScheduleMemoryResponse /
// idsToStoreDiscoveryResponse map the (id, short_id) tuples deps.storeMemory,
// deps.scheduleMemory, and deps.storeDiscovery return into their responses.
func idsToStoreMemoryResponse(id, shortID string) *engramv1.StoreMemoryResponse {
	return &engramv1.StoreMemoryResponse{Id: id, ShortId: shortID}
}

func idsToScheduleMemoryResponse(id, shortID string) *engramv1.ScheduleMemoryResponse {
	return &engramv1.ScheduleMemoryResponse{Id: id, ShortId: shortID}
}

func idsToStoreDiscoveryResponse(id, shortID string) *engramv1.StoreDiscoveryResponse {
	return &engramv1.StoreDiscoveryResponse{Id: id, ShortId: shortID}
}
