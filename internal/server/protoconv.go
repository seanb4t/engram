// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"slices"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
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
	// D-06a: setVisibilityArgs.Shared is now *bool (never nil on this lane —
	// the Visibility enum is a required, always-populated proto field, so
	// visibilityToShared always resolves a definite bool). A nil Shared can
	// only arise from the MCP JSON decode path when the field is genuinely
	// absent on the wire.
	shared := visibilityToShared(req.GetVisibility())
	return setVisibilityArgs{
		ID:     req.GetId(),
		Shared: &shared,
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

// supersedeMemoryRequestToArgs converts a SupersedeMemoryRequest into
// supersedeArgs (milestone 2026-09-25.01 Phase 3 plan 03-02, D-08/D-09):
// embeds the store_memory field set plus Supersedes, IdempotencyKey, and
// ValidateOnly. Citations is assigned only when the request carries at
// least one — citationsToArgs always allocates, so this keeps a
// Citation-less Connect call matching the MCP lane's nil.
func supersedeMemoryRequestToArgs(req *engramv1.SupersedeMemoryRequest) supersedeArgs {
	a := supersedeArgs{
		storeArgs: storeArgs{
			Content:        req.GetContent(),
			Scope:          req.GetScope(),
			Source:         req.GetSource(),
			Category:       req.GetCategory(),
			Tags:           req.GetTags(),
			Repo:           req.GetRepo(),
			Workspace:      req.GetWorkspace(),
			Worktree:       req.GetWorktree(),
			BaseDir:        req.GetBaseDir(),
			Summary:        req.GetSummary(),
			IdempotencyKey: req.GetIdempotencyKey(),
		},
		Supersedes:   req.GetSupersedes(),
		ValidateOnly: req.GetValidateOnly(),
	}
	if len(req.GetCitations()) > 0 {
		a.Citations = citationsToArgs(req.GetCitations())
	}
	return a
}

// supersedeOutcomeToResponse maps a supersedeOutcome into the wire response
// (D-08/D-09, option-a): Id/ShortId are set on a real call; Validated,
// Supersedes, and Targets (compact view) are set only when Validated is
// true.
func supersedeOutcomeToResponse(o supersedeOutcome, maxChars int) *engramv1.SupersedeMemoryResponse {
	resp := &engramv1.SupersedeMemoryResponse{
		Id:        o.ID,
		ShortId:   o.ShortID,
		Validated: o.Validated,
	}
	if o.Validated {
		resp.Supersedes = o.Supersedes
		resp.Targets = shapeProtoMemories(o.Targets, false, maxChars)
	}
	return resp
}

// archiveOutcomeToProto maps one archive.go outcome word to its
// ArchiveOutcome enum value (milestone 2026-09-25.01 Phase 3). An unknown
// word (should not occur — archive.go's outcome constants are the only
// producers) maps to ARCHIVE_OUTCOME_UNSPECIFIED rather than panicking, so a
// future outcome word added to archive.go without a matching arm here fails
// visibly on the wire (an UNSPECIFIED outcome) instead of crashing the
// handler.
func archiveOutcomeToProto(outcome string) engramv1.ArchiveOutcome {
	switch outcome {
	case outcomeArchived:
		return engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_ARCHIVED
	case outcomeAlreadyArchived:
		return engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_ALREADY_ARCHIVED
	case outcomeRestored:
		return engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_RESTORED
	case outcomeNotArchived:
		return engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_NOT_ARCHIVED
	case outcomeNotFound:
		return engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_NOT_FOUND
	default:
		return engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_UNSPECIFIED
	}
}

// archiveResultsToProto is a plain field-by-field copy (no validation, no
// re-fetch — protoconv's standing discipline) from archive.go's
// []archiveResult into the wire []*engramv1.ArchiveResult shape shared by
// ArchiveMemoryResponse and RestoreMemoryResponse.
func archiveResultsToProto(rs []archiveResult) []*engramv1.ArchiveResult {
	out := make([]*engramv1.ArchiveResult, len(rs))
	for i, r := range rs {
		out[i] = &engramv1.ArchiveResult{
			Requested: r.Requested,
			Id:        r.ID,
			Outcome:   archiveOutcomeToProto(r.Outcome),
		}
	}
	return out
}

// listScheduledRequestToArgs maps ListScheduledRequest onto listScheduledArgs
// (plan 03-03, D-11) — Scope/State/Limit/CreatedAfter/CreatedBefore mirror
// the pre-existing MCP fields; CrossSpine and Cursor (from PageToken) are the
// widened fields shared with the Connect ListMemories precedent.
func listScheduledRequestToArgs(req *engramv1.ListScheduledRequest) listScheduledArgs {
	return listScheduledArgs{
		Scope:         req.GetScope(),
		State:         req.GetState(),
		Limit:         req.GetLimit(),
		CreatedAfter:  req.GetCreatedAfter(),
		CreatedBefore: req.GetCreatedBefore(),
		CrossSpine:    req.GetCrossSpine(),
		Cursor:        req.GetPageToken(),
	}
}

// listRulesRequestToArgs maps ListRulesRequest onto listRulesArgs (plan
// 03-04, D-10): Scopes/Tags/Full mirror the pre-existing MCP fields
// verbatim — an empty Scopes list is the all-scopes read on this lane too.
func listRulesRequestToArgs(req *engramv1.ListRulesRequest) listRulesArgs {
	return listRulesArgs{
		Scopes: req.GetScopes(),
		Tags:   req.GetTags(),
		Full:   req.GetFull(),
	}
}

// edgeTypeToProto maps store.RelatedEdgeType onto the wire EdgeType enum
// (plan 03-05, D-12). An unrecognized type maps to UNSPECIFIED — this
// mirrors archiveOutcomeToProto's default-arm discipline, never a panic.
func edgeTypeToProto(t store.RelatedEdgeType) engramv1.EdgeType {
	switch t {
	case store.RelatedEdgeSupersession:
		return engramv1.EdgeType_EDGE_TYPE_SUPERSESSION
	case store.RelatedEdgeCitation:
		return engramv1.EdgeType_EDGE_TYPE_CITATION
	case store.RelatedEdgeTag:
		return engramv1.EdgeType_EDGE_TYPE_TAG
	case store.RelatedEdgeVector:
		return engramv1.EdgeType_EDGE_TYPE_VECTOR
	default:
		return engramv1.EdgeType_EDGE_TYPE_UNSPECIFIED
	}
}

// supersessionDirectionToProto maps store.SupersessionDirection onto the
// wire SupersessionDirection enum.
func supersessionDirectionToProto(d store.SupersessionDirection) engramv1.SupersessionDirection {
	switch d {
	case store.SupersessionSuccessor:
		return engramv1.SupersessionDirection_SUPERSESSION_DIRECTION_SUCCESSOR
	case store.SupersessionPredecessor:
		return engramv1.SupersessionDirection_SUPERSESSION_DIRECTION_PREDECESSOR
	default:
		return engramv1.SupersessionDirection_SUPERSESSION_DIRECTION_UNSPECIFIED
	}
}

// relatedEdgeToProto maps one store.RelatedEdge onto the wire RelatedEdge —
// a switch on e.Type setting exactly one oneof wrapper, copying every
// evidence value exactly (D-12): the type and the oneof case always match.
// An unrecognized type maps to UNSPECIFIED with no evidence case set.
func relatedEdgeToProto(e store.RelatedEdge) *engramv1.RelatedEdge {
	switch e.Type {
	case store.RelatedEdgeVector:
		return &engramv1.RelatedEdge{
			Type:     edgeTypeToProto(e.Type),
			Evidence: &engramv1.RelatedEdge_Vector{Vector: &engramv1.VectorEvidence{Score: e.Score}},
		}
	case store.RelatedEdgeTag:
		tags := make([]*engramv1.WeightedTag, len(e.SharedTags))
		for i, t := range e.SharedTags {
			tags[i] = &engramv1.WeightedTag{Tag: t.Tag, Weight: t.Weight}
		}
		return &engramv1.RelatedEdge{
			Type:     edgeTypeToProto(e.Type),
			Evidence: &engramv1.RelatedEdge_Tag{Tag: &engramv1.TagEvidence{SharedTags: tags, TagWeight: e.TagWeight}},
		}
	case store.RelatedEdgeCitation:
		refs := make([]*engramv1.CitationRef, len(e.SharedCitations))
		for i, c := range e.SharedCitations {
			refs[i] = &engramv1.CitationRef{Kind: c.Kind, Ref: c.Ref}
		}
		return &engramv1.RelatedEdge{
			Type:     edgeTypeToProto(e.Type),
			Evidence: &engramv1.RelatedEdge_Citation{Citation: &engramv1.CitationEvidence{SharedCitations: refs}},
		}
	case store.RelatedEdgeSupersession:
		return &engramv1.RelatedEdge{
			Type: edgeTypeToProto(e.Type),
			Evidence: &engramv1.RelatedEdge_Supersession{Supersession: &engramv1.SupersessionEvidence{
				Direction: supersessionDirectionToProto(e.Direction),
				Depth:     uint32(e.Depth),
			}},
		}
	default:
		return &engramv1.RelatedEdge{Type: edgeTypeToProto(e.Type)}
	}
}

// relatedResultToProto shapes a store.RelatedResult for the Connect
// RelatedMemories response: the anchor and every entry's memory go through
// shapeProtoMemories exactly like every other read RPC, so full governs the
// compact-vs-full projection on this lane too (D-13).
func relatedResultToProto(res store.RelatedResult, full bool, maxChars int) *engramv1.RelatedMemoriesResponse {
	anchor := shapeProtoMemories([]store.Memory{res.Anchor}, full, maxChars)[0]
	related := make([]*engramv1.RelatedMemory, len(res.Related))
	for i, r := range res.Related {
		edges := make([]*engramv1.RelatedEdge, len(r.Edges))
		for j, e := range r.Edges {
			edges[j] = relatedEdgeToProto(e)
		}
		related[i] = &engramv1.RelatedMemory{
			Memory: shapeProtoMemories([]store.Memory{r.Memory}, full, maxChars)[0],
			Edges:  edges,
		}
	}
	return &engramv1.RelatedMemoriesResponse{Anchor: anchor, Related: related, Truncated: res.Truncated}
}
