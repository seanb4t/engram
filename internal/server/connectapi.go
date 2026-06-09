// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"
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
	return &engramv1.Memory{
		Id: m.ID, Content: m.Content, Scope: m.Scope,
		Repo: m.Repo, Workspace: m.Workspace, Worktree: m.Worktree, BaseDir: m.BaseDir,
		Source: m.Source, Category: m.Category, Tags: m.Tags,
		Actor: m.Actor, Owner: m.Owner, Visibility: m.Visibility,
		CreatedAt: timestamppb.New(m.CreatedAt),
	}
}

func memoriesToProto(ms []store.Memory) []*engramv1.Memory {
	out := make([]*engramv1.Memory, len(ms))
	for i, m := range ms {
		out[i] = memoryToProto(m)
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
	ms, err := a.d.st.List(ctx, req.Msg.Scope, subj, req.Msg.Limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&engramv1.ListMemoriesResponse{Memories: memoriesToProto(ms)}), nil
}

func (a *engramAPI) SearchMemories(ctx context.Context, req *connect.Request[engramv1.SearchMemoriesRequest]) (*connect.Response[engramv1.SearchMemoriesResponse], error) {
	subj, err := subjectFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	vec, err := a.d.em.Embed(ctx, req.Msg.Query)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	k := req.Msg.K
	if k == 0 {
		k = 20
	}
	ms, err := a.d.st.Search(ctx, req.Msg.Scope, subj, vec, k)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&engramv1.SearchMemoriesResponse{Memories: memoriesToProto(ms)}), nil
}

func (a *engramAPI) GetMemory(ctx context.Context, req *connect.Request[engramv1.GetMemoryRequest]) (*connect.Response[engramv1.GetMemoryResponse], error) {
	subj, err := subjectFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	m, err := a.d.st.GetReadable(ctx, req.Msg.Id, subj)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&engramv1.GetMemoryResponse{Memory: memoryToProto(m)}), nil
}

func (a *engramAPI) SearchDiscoveries(ctx context.Context, req *connect.Request[engramv1.SearchDiscoveriesRequest]) (*connect.Response[engramv1.SearchDiscoveriesResponse], error) {
	subj, err := subjectFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	vec, err := a.d.em.Embed(ctx, req.Msg.Query)
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

// connectResolver supplies the per-request identity TokenInfo for the Connect
// lane. The cookie/OIDC lane (later plan) provides a real one; a nil resolver
// defaults to anonymous (the no-issuer case).
type connectResolver func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error)

func (d *deps) mountConnect(mux *http.ServeMux, resolve connectResolver) {
	if resolve == nil {
		resolve = func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error) { return nil, nil }
	}
	path, h := engramv1connect.NewEngramServiceHandler(
		&engramAPI{d: d},
		connect.WithInterceptors(newConnectSubjectInterceptor(resolve)),
	)
	mux.Handle(path, h)
}
