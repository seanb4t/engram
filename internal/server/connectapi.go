// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"connectrpc.com/otelconnect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
	"github.com/seanb4t/engram/internal/auth"
	"github.com/seanb4t/engram/internal/store"
)

// engramAPI implements the generated EngramServiceHandler. It reuses the same
// *deps (store + embedder) as the MCP handlers; the caller's Subject is resolved
// from the Connect request context by the interceptor (see connectauth.go).
type engramAPI struct {
	engramv1connect.UnimplementedEngramServiceHandler
	d *deps
}

// citationsToProto maps store-layer discovery citations to their proto
// counterpart. Returns nil (not an empty slice) for empty input so
// non-discovery memories round-trip with Citations == nil.
func citationsToProto(cs []store.Citation) []*engramv1.Citation {
	if len(cs) == 0 {
		return nil
	}
	out := make([]*engramv1.Citation, len(cs))
	for i, c := range cs {
		out[i] = &engramv1.Citation{Kind: c.Kind, Ref: c.Ref, Locator: c.Locator, Pin: c.Pin, Excerpt: c.Excerpt}
	}
	return out
}

func memoryToProto(m store.Memory) *engramv1.Memory {
	// LastAccessedAt is nil for never-accessed records; leave the proto field
	// unset rather than emitting a year-1 (0001-01-01) Timestamp.
	var lastAccessed *timestamppb.Timestamp
	if m.LastAccessedAt != nil {
		lastAccessed = timestamppb.New(*m.LastAccessedAt)
	}
	return &engramv1.Memory{
		Id: m.ID, Content: m.Content, Scope: m.Scope,
		Repo: m.Repo, Workspace: m.Workspace, Worktree: m.Worktree, BaseDir: m.BaseDir,
		Source: m.Source, Category: m.Category, Tags: m.Tags,
		Actor: m.Actor, Owner: m.Owner, Visibility: m.Visibility,
		CreatedAt:      timestamppb.New(m.CreatedAt),
		Summary:        m.Summary,
		SummarySource:  string(m.SummarySource),
		Score:          m.Score,
		ShortId:        m.ShortID,
		AccessCount:    m.AccessCount,
		LastAccessedAt: lastAccessed,
		Kind:           m.Kind,
		Citations:      citationsToProto(m.Citations),
	}
}

func memoriesToProto(ms []store.Memory) []*engramv1.Memory {
	out := make([]*engramv1.Memory, len(ms))
	for i, m := range ms {
		out[i] = memoryToProto(m)
	}
	return out
}

// parseRFC3339 maps an optional RFC3339 string to a time.Time; empty → zero.
func parseRFC3339(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, s)
}

// shapeProtoMemories mirrors the MCP recall contract over the Connect wire: when
// not full, clear Content and surface a summary-or-truncation so default callers
// pay summary-sized payloads. Callers opt into full content with full=true.
//
// Citations and Kind are ALSO cleared in the non-full branch (D-07): MCP's
// recallView (summary.go) already omits them from the compact view for free
// by being a hand-written allow-list struct with no citations field, but this
// shaper starts from the complete memoryToProto message and must clear them
// explicitly, or a curated memory carrying up to 50 citations with 16 KiB
// excerpts each would ride the default (compact) Connect response and defeat
// the token budget this view exists to protect.
func shapeProtoMemories(ms []store.Memory, full bool, maxChars int) []*engramv1.Memory {
	out := make([]*engramv1.Memory, len(ms))
	for i, m := range ms {
		pb := memoryToProto(m)
		if !full {
			summary, _ := summaryOrTruncation(m, maxChars)
			pb.Content = ""
			pb.Summary = summary
			pb.Citations = nil
			pb.Kind = ""
		}
		out[i] = pb
	}
	return out
}

// ListScopes is the ONE documented D-07 exception: no MCP-side
// deps.listScopes counterpart exists (read-only scope-count listing —
// research OQ2), so this handler still calls a.d.st.ListScopes directly.
func (a *engramAPI) ListScopes(ctx context.Context, _ *connect.Request[engramv1.ListScopesRequest]) (*connect.Response[engramv1.ListScopesResponse], error) {
	subj, err := subjectFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	scopes, approx, err := a.d.st.ListScopes(ctx, subj)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	resp := &engramv1.ListScopesResponse{Approximate: approx}
	for _, sc := range scopes {
		resp.Scopes = append(resp.Scopes, &engramv1.ScopeCount{Scope: sc.Scope, Count: sc.Count})
	}
	return connect.NewResponse(resp), nil
}

// ListMemories is rewired onto the 17-06 typed core deps.listMemory (D-07):
// every Connect field (offset/categories/visibility/exact total/cursor/
// cursor_mode/tags/created window) survives, and Limit is passed through
// UNCHANGED — limit=0 means "all" (store.go:873-874), NOT silently capped to
// 20 (round-4 finding-7; 17-06 removed the shared Limit==0->20 default from
// the core, so no lane may re-introduce it here). created_after/before are
// parsed to time.Time AT THIS BOUNDARY so a malformed value returns
// CodeInvalidArgument directly, never reaching the typed core to be
// misclassified as CodeInternal by connectError (round-4 MED-6).
func (a *engramAPI) ListMemories(ctx context.Context, req *connect.Request[engramv1.ListMemoriesRequest]) (*connect.Response[engramv1.ListMemoriesResponse], error) {
	c, err := callerFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	after, err := parseRFC3339(req.Msg.CreatedAfter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("created_after: %w", err))
	}
	before, err := parseRFC3339(req.Msg.CreatedBefore)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("created_before: %w", err))
	}
	// Enforce the cursor_mode/offset mutual exclusion (documented on the proto
	// field) at the handler, alongside the created_after/before validation above.
	// store.List also rejects this, but a fail-fast guard keeps the wire contract
	// self-evident at its boundary.
	if req.Msg.CursorMode && req.Msg.Offset > 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cursor_mode is mutually exclusive with offset"))
	}
	res, err := a.d.listMemory(ctx, c, coreListRequest{
		Scope:         req.Msg.Scope,
		Limit:         req.Msg.Limit, // 0 = "all" — no default re-introduced here (round-4 finding-7)
		Offset:        req.Msg.Offset,
		Categories:    req.Msg.Categories,
		Visibility:    req.Msg.Visibility,
		Tags:          req.Msg.Tags,
		CreatedAfter:  after,
		CreatedBefore: before,
		Cursor:        req.Msg.PageToken,
		// CursorMode is the explicit opt-in (engram-3hp9): a pure-Connect client
		// sets cursor_mode=true to bootstrap cursor paging on the tokenless first
		// page. PageToken != "" keeps resume working whether or not the flag is
		// re-sent. Default (both false) stays offset-for-UI (ADR engram-1frj).
		CursorMode: req.Msg.CursorMode || req.Msg.PageToken != "",
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return connect.NewResponse(&engramv1.ListMemoriesResponse{
		Memories:      shapeProtoMemories(res.Memories, req.Msg.Full, a.d.summaryMaxChars),
		Total:         res.Total,
		Approximate:   false,
		NextPageToken: res.NextToken,
	}), nil
}

// SearchMemories is rewired onto the 17-06 typed core deps.searchMemory
// (D-07). The Connect k=20 default is applied HERE, before the call —
// deps.searchMemory carries no internal default. The handler no longer
// embeds the query itself (round-5 MED, grok): deps.searchMemory embeds the
// query internally, so a handler-local embed step would double-embed.
func (a *engramAPI) SearchMemories(ctx context.Context, req *connect.Request[engramv1.SearchMemoriesRequest]) (*connect.Response[engramv1.SearchMemoriesResponse], error) {
	c, err := callerFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	k := req.Msg.K
	if k == 0 {
		k = 20
	}
	after, err := parseRFC3339(req.Msg.CreatedAfter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("created_after: %w", err))
	}
	before, err := parseRFC3339(req.Msg.CreatedBefore)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("created_before: %w", err))
	}
	ms, err := a.d.searchMemory(ctx, c, coreSearchRequest{
		Scope: req.Msg.Scope, Query: req.Msg.Query, K: k, Tags: req.Msg.Tags,
		CreatedAfter: after, CreatedBefore: before, Categories: req.Msg.Categories,
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return connect.NewResponse(&engramv1.SearchMemoriesResponse{
		Memories: shapeProtoMemories(ms, req.Msg.Full, a.d.summaryMaxChars),
	}), nil
}

// GetMemory is rewired onto deps.getMemory (D-07); the ErrNotFound
// original-input re-wrap already lives inside deps.getMemory, so the handler
// does not duplicate it (D-11). deps.getMemory is now the SOLE
// usage-enqueue point (tools.go:1000-1003) — the handler-level enqueue call
// this method used to make is REMOVED so AccessCount increments exactly
// once per Connect get, not twice (round-4 MED, consensus Codex+grok).
func (a *engramAPI) GetMemory(ctx context.Context, req *connect.Request[engramv1.GetMemoryRequest]) (*connect.Response[engramv1.GetMemoryResponse], error) {
	c, err := callerFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	m, err := a.d.getMemory(ctx, c, idArgs{ID: req.Msg.GetId()})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return connect.NewResponse(&engramv1.GetMemoryResponse{Memory: memoryToProto(m)}), nil
}

// SearchDiscoveries is rewired onto deps.searchDiscovery (D-07). Two
// adaptations preserve today's Connect contract across the rewire (round-4
// HIGH-3, finding 7): an EMPTY request scope maps to CrossSpine=true (while
// preserving any non-empty caller Scope) so an empty Connect scope still
// spans ALL discovery scopes — deps.searchDiscovery's effectiveDiscoveryScope
// rejects an empty scope unless CrossSpine=true, whereas the old direct
// Store.SearchDiscovery call treated empty as "all" implicitly; and the
// Connect k=20 default is applied HERE, before the call — deps.
// searchDiscovery's internal default (8) is MCP-lane only. The handler no
// longer embeds the query itself (round-5 MED, grok): deps.searchDiscovery
// embeds the query internally, so a handler-local embed step would
// double-embed.
func (a *engramAPI) SearchDiscoveries(ctx context.Context, req *connect.Request[engramv1.SearchDiscoveriesRequest]) (*connect.Response[engramv1.SearchDiscoveriesResponse], error) {
	c, err := callerFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	k := req.Msg.K
	if k == 0 {
		k = 20
	}
	ms, err := a.d.searchDiscovery(ctx, c, searchDiscoveryArgs{
		Query:      req.Msg.Query,
		Scope:      req.Msg.Scope,
		K:          k,
		CrossSpine: req.Msg.Scope == "",
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return connect.NewResponse(&engramv1.SearchDiscoveriesResponse{Discoveries: memoriesToProto(ms)}), nil
}

// The six Connect write RPCs below are thin adapters (SC2): resolve the caller
// via callerFromConnectContext, convert the proto request to args via the
// 17-03 protoconv layer, call the SAME deps.* method the MCP tool calls, map
// the result via protoconv, and map any error via the single connectError
// mapper (D-11) — never the store directly, never a hand-rolled per-handler
// error mapping, never an ownership comparison (DEC-cgb).

func (a *engramAPI) StoreMemory(ctx context.Context, req *connect.Request[engramv1.StoreMemoryRequest]) (*connect.Response[engramv1.StoreMemoryResponse], error) {
	c, err := callerFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	id, shortID, err := a.d.storeMemory(ctx, c, storeMemoryRequestToArgs(req.Msg))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return connect.NewResponse(idsToStoreMemoryResponse(id, shortID)), nil
}

func (a *engramAPI) StoreDiscovery(ctx context.Context, req *connect.Request[engramv1.StoreDiscoveryRequest]) (*connect.Response[engramv1.StoreDiscoveryResponse], error) {
	c, err := callerFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	id, shortID, err := a.d.storeDiscovery(ctx, c, storeDiscoveryRequestToArgs(req.Msg))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return connect.NewResponse(idsToStoreDiscoveryResponse(id, shortID)), nil
}

// UpdateMemory maps the returned mutationResult{ID, ShortID} into the response
// WITHOUT a re-fetch — deps.updateMemory already resolved the canonical id/
// short_id from the fetched record (17-REVIEWS.md response-conversion finding).
func (a *engramAPI) UpdateMemory(ctx context.Context, req *connect.Request[engramv1.UpdateMemoryRequest]) (*connect.Response[engramv1.UpdateMemoryResponse], error) {
	c, err := callerFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	res, err := a.d.updateMemory(ctx, c, updateMemoryRequestToArgs(req.Msg))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return connect.NewResponse(mutationResultToUpdateMemoryResponse(res)), nil
}

func (a *engramAPI) DeleteMemory(ctx context.Context, req *connect.Request[engramv1.DeleteMemoryRequest]) (*connect.Response[engramv1.DeleteMemoryResponse], error) {
	c, err := callerFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	if err := a.d.deleteMemory(ctx, c, idArgs{ID: req.Msg.GetId()}); err != nil {
		return nil, connectError(ctx, err)
	}
	return connect.NewResponse(&engramv1.DeleteMemoryResponse{}), nil
}

// SetVisibility maps the returned mutationResult{ID, ShortID} into the
// response WITHOUT a re-fetch, mirroring UpdateMemory.
func (a *engramAPI) SetVisibility(ctx context.Context, req *connect.Request[engramv1.SetVisibilityRequest]) (*connect.Response[engramv1.SetVisibilityResponse], error) {
	c, err := callerFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	res, err := a.d.setVisibility(ctx, c, setVisibilityRequestToArgs(req.Msg))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return connect.NewResponse(mutationResultToSetVisibilityResponse(res)), nil
}

func (a *engramAPI) ScheduleMemory(ctx context.Context, req *connect.Request[engramv1.ScheduleMemoryRequest]) (*connect.Response[engramv1.ScheduleMemoryResponse], error) {
	c, err := callerFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	id, shortID, err := a.d.scheduleMemory(ctx, c, scheduleMemoryRequestToArgs(req.Msg))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return connect.NewResponse(idsToScheduleMemoryResponse(id, shortID)), nil
}

// connectResolver supplies the per-request identity TokenInfo for the
// Connect lane, plus WHICH credential family authenticated it (auth.Lane,
// D-07). NewConnectResolver composes the bearer half and the webauth
// cookie/OIDC half (passed in by the caller, serve.go); the lane it returns
// is decided exclusively by which half actually succeeded, never by a
// second read of the request (D-02). A nil resolver means Connect is NOT
// mounted at all (R1): mountConnect returns immediately without registering
// any handler.
type connectResolver func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error)

func (d *deps) mountConnect(mux *http.ServeMux, resolve connectResolver, csrfVerify func(owner, token string) bool, reseal resealFunc) error {
	if resolve == nil {
		return nil // R1: no resolver => UI disabled => Connect not mounted at all.
	}
	otelIc, err := otelconnect.NewInterceptor()
	if err != nil {
		return fmt.Errorf("otelconnect interceptor: %w", err)
	}
	validator, err := protovalidate.New()
	if err != nil {
		return fmt.Errorf("protovalidate.New: %w", err)
	}
	path, h := engramv1connect.NewEngramServiceHandler(
		&engramAPI{d: d},
		// Order: otel outermost (spans cover auth + logging), then access-log,
		// then the subject interceptor that resolves identity (401), then the
		// CSRF double-submit token interceptor (PermissionDenied, write-only,
		// D-02), then the validate interceptor (400) — auth must run before
		// CSRF and CSRF before validation (D-10/D-02) so neither an
		// unauthenticated nor a CSRF-forged caller ever sees field-level
		// request detail. The reseal interceptor runs LAST/innermost, after
		// validate: it wraps the handler itself, so it only ever touches a
		// fully-authorized, valid, successful response (D-04) and — unlike
		// every interceptor above it — is NOT procedure-gated: it re-seals
		// both read and write responses (D-03).
		connect.WithInterceptors(
			otelIc,
			newConnectAccessLogInterceptor(slog.Default()),
			newConnectSubjectInterceptor(resolve),
			newConnectCSRFInterceptor(csrfVerify),
			newConnectValidateInterceptor(validator),
			newConnectResealInterceptor(reseal),
		),
	)
	mux.Handle(path, h)
	return nil
}
