// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
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
	"github.com/seanb4t/engram/internal/store"
)

// engramAPI implements the generated EngramServiceHandler. It reuses the same
// *deps (store + embedder) as the MCP handlers; the caller's Subject is resolved
// from the Connect request context by the interceptor (see connectauth.go).
type engramAPI struct {
	engramv1connect.UnimplementedEngramServiceHandler
	d *deps
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
func shapeProtoMemories(ms []store.Memory, full bool, maxChars int) []*engramv1.Memory {
	out := make([]*engramv1.Memory, len(ms))
	for i, m := range ms {
		pb := memoryToProto(m)
		if !full {
			summary, _ := summaryOrTruncation(m, maxChars)
			pb.Content = ""
			pb.Summary = summary
		}
		out[i] = pb
	}
	return out
}

func (a *engramAPI) ListScopes(ctx context.Context, _ *connect.Request[engramv1.ListScopesRequest]) (*connect.Response[engramv1.ListScopesResponse], error) {
	subj, err := subjectFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	scopes, approx, err := a.d.st.ListScopes(ctx, subj)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	resp := &engramv1.ListScopesResponse{Approximate: approx}
	for _, sc := range scopes {
		resp.Scopes = append(resp.Scopes, &engramv1.ScopeCount{Scope: sc.Scope, Count: sc.Count})
	}
	return connect.NewResponse(resp), nil
}

func (a *engramAPI) ListMemories(ctx context.Context, req *connect.Request[engramv1.ListMemoriesRequest]) (*connect.Response[engramv1.ListMemoriesResponse], error) {
	subj, err := subjectFromConnectContext(ctx)
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
	ms, total, nextToken, err := a.d.st.List(ctx, req.Msg.Scope, subj, store.ListOptions{
		Limit:         req.Msg.Limit,
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
		if errors.Is(err, store.ErrInvalidArgument) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&engramv1.ListMemoriesResponse{
		Memories:      shapeProtoMemories(ms, req.Msg.Full, a.d.summaryMaxChars),
		Total:         total,
		Approximate:   false,
		NextPageToken: nextToken,
	}), nil
}

func (a *engramAPI) SearchMemories(ctx context.Context, req *connect.Request[engramv1.SearchMemoriesRequest]) (*connect.Response[engramv1.SearchMemoriesResponse], error) {
	subj, err := subjectFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	vec, err := a.d.em.EmbedQuery(ctx, req.Msg.Query)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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
	ms, err := a.d.st.SearchReranked(ctx, req.Msg.Scope, subj, req.Msg.Query, vec, k, req.Msg.Tags, after, before)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&engramv1.SearchMemoriesResponse{
		Memories: shapeProtoMemories(ms, req.Msg.Full, a.d.summaryMaxChars),
	}), nil
}

func (a *engramAPI) GetMemory(ctx context.Context, req *connect.Request[engramv1.GetMemoryRequest]) (*connect.Response[engramv1.GetMemoryResponse], error) {
	subj, err := subjectFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	// Resolve id or short id to the point UUID (owner-agnostic; the read gate
	// below governs visibility), mirroring the MCP by-id tools' getMemory.
	pid, err := a.d.st.ResolvePointID(ctx, req.Msg.Id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if errors.Is(err, store.ErrInvalidArgument) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	m, err := a.d.st.GetReadable(ctx, pid, subj)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Re-wrap with the caller's ORIGINAL input so a resolved short id
			// never leaks another owner's real UUID into the error message.
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%w: %s", store.ErrNotFound, req.Msg.Id))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	// D-01: count only on a successful fetch-by-id; call-and-ignore — mirrors
	// the MCP getMemory handler.
	a.d.usageQueue.tryEnqueue(pid)
	return connect.NewResponse(&engramv1.GetMemoryResponse{Memory: memoryToProto(m)}), nil
}

func (a *engramAPI) SearchDiscoveries(ctx context.Context, req *connect.Request[engramv1.SearchDiscoveriesRequest]) (*connect.Response[engramv1.SearchDiscoveriesResponse], error) {
	subj, err := subjectFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	vec, err := a.d.em.EmbedQuery(ctx, req.Msg.Query)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	k := req.Msg.K
	if k == 0 {
		k = 20
	}
	ms, err := a.d.st.SearchDiscovery(ctx, req.Msg.Scope, "", subj, vec, k)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&engramv1.SearchDiscoveriesResponse{Discoveries: memoriesToProto(ms)}), nil
}

// The six Connect write RPCs below are thin adapters (SC2): resolve the caller
// via callerFromConnectContext, convert the proto request to args via the
// 17-03 protoconv layer, call the SAME deps.* method the MCP tool calls, map
// the result via protoconv, and map any error via the single connectError
// mapper (D-11) — never a.d.st.* directly, never a hand-rolled per-handler
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

// connectResolver supplies the per-request identity TokenInfo for the Connect
// lane. The webauth cookie/OIDC resolver is passed in by the caller (serve.go)
// when the web UI is enabled. A nil resolver means Connect is NOT mounted at
// all (R1): mountConnect returns immediately without registering any handler.
type connectResolver func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error)

func (d *deps) mountConnect(mux *http.ServeMux, resolve connectResolver, csrfVerify func(owner, token string) bool) error {
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
		// request detail.
		connect.WithInterceptors(
			otelIc,
			newConnectAccessLogInterceptor(slog.Default()),
			newConnectSubjectInterceptor(resolve),
			newConnectCSRFInterceptor(csrfVerify),
			newConnectValidateInterceptor(validator),
		),
	)
	mux.Handle(path, h)
	return nil
}
