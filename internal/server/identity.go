// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/seanb4t/engram/internal/auth"
	"github.com/seanb4t/engram/internal/store"
)

// SubjectFromTokenInfo maps a verified TokenInfo to a store.Subject. It is the
// single owner-claim->Subject mapping shared by every auth lane (the MCP bearer
// lane via subjectFromContext, and the Connect cookie lane via
// subjectFromConnectContext). nil TokenInfo (auth disabled) yields the anonymous
// bucket; a present token with a missing/empty owner-claim value fails closed
// rather than collapsing to anonymous.
func SubjectFromTokenInfo(ti *mcpauth.TokenInfo) (store.Subject, error) {
	if ti == nil {
		return store.Anonymous(), nil
	}
	if v, ok := ti.Extra[auth.OwnerClaimExtraKey].(string); ok && v != "" {
		return store.Authenticated(v), nil
	}
	return nil, fmt.Errorf("validated token missing owner claim")
}

// connectSubjectKey is engram-owned (NOT the go-sdk's unexported key); the
// Connect interceptor writes the resolved TokenInfo under it and
// subjectFromConnectContext reads it. Tests use withConnectTokenInfo to inject.
type connectSubjectKey struct{}

func withConnectTokenInfo(ctx context.Context, ti *mcpauth.TokenInfo) context.Context {
	return context.WithValue(ctx, connectSubjectKey{}, ti)
}

// subjectFromConnectContext resolves the Subject for a Connect request.
//
// Two distinct cases:
//   - Key present, ti==nil  (interceptor ran, auth disabled / anonymous caller):
//     resolves to the anonymous bucket via SubjectFromTokenInfo(nil). This is the
//     legitimate no-issuer path.
//   - Key absent (interceptor was never installed — a programming error):
//     fails closed with an error rather than silently granting anonymous-bucket access.
func subjectFromConnectContext(ctx context.Context) (store.Subject, error) {
	ti, ok := ctx.Value(connectSubjectKey{}).(*mcpauth.TokenInfo)
	if !ok {
		// Interceptor not installed — a programming error. Fail closed rather
		// than silently granting anonymous-bucket access.
		return nil, fmt.Errorf("connect subject key absent: interceptor not installed")
	}
	return SubjectFromTokenInfo(ti)
}

// connectLaneKey is engram-owned, mirroring connectSubjectKey: the subject
// interceptor stamps which credential family authenticated a Connect
// request (D-07) under this key, and laneFromConnectContext reads it back.
type connectLaneKey struct{}

// withConnectLane stamps the resolved auth.Lane onto ctx.
func withConnectLane(ctx context.Context, lane auth.Lane) context.Context {
	return context.WithValue(ctx, connectLaneKey{}, lane)
}

// laneFromConnectContext returns the auth.Lane stamped by the subject
// interceptor, or auth.LaneUnknown when the key is absent or holds an
// unexpected type. auth.LaneUnknown being the zero value is itself the
// fail-closed signal downstream (D-08): callers that gate on the lane
// (e.g. the CSRF exemption) reject on it rather than treat an absent key as
// a permissive default, so this accessor returns no error.
func laneFromConnectContext(ctx context.Context) auth.Lane {
	lane, ok := ctx.Value(connectLaneKey{}).(auth.Lane)
	if !ok {
		return auth.LaneUnknown
	}
	return lane
}

// caller is the verified caller identity threaded explicitly through every
// write deps.* method (and storeRule/listRules), replacing the internal
// subjectFromContext/actorFromContext ctx reads (SC1/D-01). Subj is the
// authorization key (store.Subject); Actor is the attribution value stamped
// onto Memory.Actor.
type caller struct {
	Subj  store.Subject
	Actor string
}

// callerFromTokenInfo is the single choke point that builds a caller for both
// auth lanes (MCP via callerFromContext, Connect via callerFromConnectContext)
// from one verified TokenInfo. Subj resolves exactly as SubjectFromTokenInfo
// does. Actor resolves to ti.UserID, falling back to the resolved owner
// (Subj.Owner()) when UserID is empty — the Connect cookie lane's TokenInfo
// carries only Extra[owner_claim], never UserID (landmine 3), so without this
// fallback every Connect-attributed Memory.Actor would be permanently empty.
//
// callerFromTokenInfo(nil) yields the anonymous caller (owner "", actor ""),
// no error — the "non-empty actor" guarantee below applies only to
// authenticated callers, preserving the existing anonymous bucket. A present
// token with a missing/empty owner claim fails closed (mirrors
// SubjectFromTokenInfo).
func callerFromTokenInfo(ti *mcpauth.TokenInfo) (caller, error) {
	subj, err := SubjectFromTokenInfo(ti)
	if err != nil {
		return caller{}, err
	}
	actor := ""
	if ti != nil {
		actor = ti.UserID
	}
	if actor == "" {
		actor = subj.Owner()
	}
	return caller{Subj: subj, Actor: actor}, nil
}

// callerFromContext resolves the caller for an MCP request (the bearer-token
// lane), mirroring subjectFromContext's TokenInfo source.
func callerFromContext(ctx context.Context) (caller, error) {
	return callerFromTokenInfo(mcpauth.TokenInfoFromContext(ctx))
}

// callerFromConnectContext resolves the caller for a Connect request (the
// cookie lane), mirroring subjectFromConnectContext's fail-closed behavior
// when the interceptor was never installed.
func callerFromConnectContext(ctx context.Context) (caller, error) {
	ti, ok := ctx.Value(connectSubjectKey{}).(*mcpauth.TokenInfo)
	if !ok {
		return caller{}, fmt.Errorf("connect subject key absent: interceptor not installed")
	}
	return callerFromTokenInfo(ti)
}

// mutationResult is the typed by-id mutation outcome deps.updateMemory and
// deps.setVisibility return alongside error, so their Connect responses can
// populate id/short_id from the already-fetched record without a re-fetch.
// deps.deleteMemory stays error-only: DeleteMemoryResponse carries no fields.
type mutationResult struct {
	ID      string
	ShortID string
}

// errRuleImmutable is the typed sentinel for the rule-immutability rejections
// (a rule cannot be made private, un-shared, or content-summary-broken via
// update_memory/set_visibility — delete it instead). errors.Is-matchable so
// the Connect error mapper (17-04) maps it without string-matching. Distinct
// from the pre-existing errStaleSummary (summary.go:16), which is REUSED
// unchanged, not redeclared here (review round-2 BLOCKER 2).
var errRuleImmutable = errors.New("rules are always shared")
