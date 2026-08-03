// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	"github.com/seanb4t/engram/internal/auth"
)

// wantNamespacedOwner mirrors auth.go's DOCUMENTED namespacedOwner encoding
// (`<len(claim)>:<claim>:<len(value)>:<value>`; see docs-site
// reference/auth.md "Upgrading to namespaced service owners") as a test
// oracle. The format is a stable public contract, not a private
// implementation detail reached into across the package boundary — the
// static-token sub-test below drives it through the REAL
// auth.NewStaticTokenVerifier and asserts against this independently
// computed expectation.
func wantNamespacedOwner(claim, value string) string {
	return fmt.Sprintf("%d:%s:%d:%s", len(claim), claim, len(value), value)
}

// chainParityJWTToken is a structurally JWT-shaped bearer (D-04 discriminator:
// exactly two dots) so it routes to the OIDC branch (human then service, D-02
// order) regardless of which stub answers it.
const chainParityJWTToken = "header.payload.signature"

// stubOIDCVerifier is a mcpauth.TokenVerifier stub standing in for the human
// or client-credentials OIDC lane (per the plan's action: "stub/fake
// verifiers for the human and client-credentials lanes"). It always succeeds
// with the given owner (mirroring auth.go's TokenInfo shape), or — when owner
// is "" — always fails closed with mcpauth.ErrInvalidToken, modeling the D-08
// service-lane empty-owner reject at the verifier boundary. The chain
// composition wired in Task 1 must never swallow that reject into a
// fall-through success (T-23-01).
func stubOIDCVerifier(userID, owner string) mcpauth.TokenVerifier {
	return func(_ context.Context, _ string, _ *http.Request) (*mcpauth.TokenInfo, error) {
		if owner == "" {
			return nil, errors.Join(mcpauth.ErrInvalidToken, errors.New("service principal: no resolvable owner claim"))
		}
		return &mcpauth.TokenInfo{
			UserID: userID,
			Extra:  map[string]any{auth.OwnerClaimExtraKey: owner},
			// Expiration is load-bearing (REVIEWS.md MED-9): auth.EnforceExpiry
			// (D-05) hard-rejects a zero Expiration, and 01-02's parity tests
			// reuse this fixture wrapped in that decorator. Without an explicit
			// future value here, every happy-path parity test would fail for
			// the wrong reason. This file's pre-existing tests never wrap with
			// EnforceExpiry, so the field is inert for them.
			Expiration: time.Now().Add(time.Hour),
		}, nil
	}
}

// alwaysFailOIDCVerifier models a lane whose token does not verify at all
// (wrong lane, bad signature, etc.) — distinct from stubOIDCVerifier's
// owner=="" fail-closed reject, this is the D-02 "try the next lane" failure
// shape that lets a JWT-shaped bearer fall through from the human lane to the
// service lane.
func alwaysFailOIDCVerifier(_ context.Context, _ string, _ *http.Request) (*mcpauth.TokenInfo, error) {
	return nil, errors.Join(mcpauth.ErrInvalidToken, errors.New("stub: this lane does not own this token"))
}

// TestServiceAuthChainParity proves D-07: owner-claim resolution and
// isolation are identical regardless of WHICH chain verifier answered. Three
// lanes (human OIDC, client-credentials OIDC, static-token) each produce a
// TokenInfo through a composed auth.ChainVerifier (Task 1's withAuth
// combinator); each TokenInfo is mapped through the SAME
// SubjectFromTokenInfo/callerFromTokenInfo choke point (identity.go) every
// downstream tool handler uses — proving the resolution is lane-independent,
// not merely "each lane individually looks right." The static-token lane
// uses the REAL auth.NewStaticTokenVerifier (per the plan's action), not a
// stub, so its owner encoding is proven against production code.
func TestServiceAuthChainParity(t *testing.T) {
	const (
		humanUserID     = "human@example.com"
		clientCredsUser = "svc-42"
		staticOwnerID   = "svc-static-owner"
		staticToken     = "static-token-value-parity"
	)
	humanOwner := humanUserID
	clientCredsOwner := wantNamespacedOwner("client_id", clientCredsUser)
	staticOwner := wantNamespacedOwner("static_token", staticOwnerID)

	static := auth.NewStaticTokenVerifier(map[string]string{staticToken: staticOwnerID}).TokenVerifier()

	tests := []struct {
		name      string
		chain     mcpauth.TokenVerifier
		token     string
		wantOwner string
	}{
		{
			name:      "human OIDC lane",
			chain:     auth.ChainVerifier(stubOIDCVerifier(humanUserID, humanOwner), alwaysFailOIDCVerifier, static),
			token:     chainParityJWTToken,
			wantOwner: humanOwner,
		},
		{
			// The human stub fails first (D-02 order: human tried before
			// service), so the chain falls through to the service stub.
			name:      "client-credentials OIDC lane",
			chain:     auth.ChainVerifier(alwaysFailOIDCVerifier, stubOIDCVerifier(clientCredsUser, clientCredsOwner), static),
			token:     chainParityJWTToken,
			wantOwner: clientCredsOwner,
		},
		{
			name:      "static-token lane (real verifier, not a stub)",
			chain:     auth.ChainVerifier(alwaysFailOIDCVerifier, alwaysFailOIDCVerifier, static),
			token:     staticToken,
			wantOwner: staticOwner,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ti, err := tt.chain(ctx, tt.token, nil)
			if err != nil {
				t.Fatalf("chain verify: %v", err)
			}

			subj, err := SubjectFromTokenInfo(ti)
			if err != nil {
				t.Fatalf("SubjectFromTokenInfo: %v", err)
			}
			if subj.Owner() == "" {
				t.Fatal("resolved owner is empty")
			}
			if subj.Owner() != tt.wantOwner {
				t.Errorf("SubjectFromTokenInfo owner = %q, want %q", subj.Owner(), tt.wantOwner)
			}

			// D-07: the SAME choke point (callerFromTokenInfo, which itself
			// calls SubjectFromTokenInfo) must resolve to the identical
			// owner, proving the resolution doesn't depend on which lane
			// produced the TokenInfo — only on the TokenInfo shape itself.
			caller, err := callerFromTokenInfo(ti)
			if err != nil {
				t.Fatalf("callerFromTokenInfo: %v", err)
			}
			if caller.Subj.Owner() != subj.Owner() {
				t.Errorf("callerFromTokenInfo owner = %q, want the same resolution as SubjectFromTokenInfo %q (D-07: identical regardless of which verifier answered)",
					caller.Subj.Owner(), subj.Owner())
			}
			if caller.Actor == "" {
				t.Error("expected a non-empty caller.Actor")
			}
		})
	}
}

// TestServiceAuthChainParity_EmptyOwnerFailsClosedPostComposition proves
// T-23-01: the D-08 service-lane fail-closed empty-owner reject survives the
// chain composition Task 1 wires into withAuth — a service principal that
// authenticates but resolves to owner=="" is rejected AT THE VERIFIER
// BOUNDARY (errors.Is(err, mcpauth.ErrInvalidToken), nil TokenInfo), never
// returned as a success that would later collapse to the anonymous bucket
// downstream in SubjectFromTokenInfo.
func TestServiceAuthChainParity_EmptyOwnerFailsClosedPostComposition(t *testing.T) {
	chain := auth.ChainVerifier(alwaysFailOIDCVerifier, stubOIDCVerifier("svc-empty", ""), nil)
	ti, err := chain(context.Background(), chainParityJWTToken, nil)
	if !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Fatalf("expected errors.Is(err, mcpauth.ErrInvalidToken), got %v", err)
	}
	if ti != nil {
		t.Fatalf("expected nil TokenInfo on fail-closed reject, got %+v", ti)
	}
}
