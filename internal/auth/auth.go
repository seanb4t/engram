// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package auth validates OIDC bearer tokens issued by a configured IdP and
// forwarded by an MCP gateway, then extracts the caller's identity so memory
// writes can be attributed to a verified user (canonical deployment: Authentik
// IdP + LiteLLM gateway).
//
// The token is the only trustworthy source of identity: clients never assert
// who they are, the IdP-signed JWT proves it. Validation covers signature
// (JWKS), issuer, and expiry; the audience claim is checked only when an
// expected value is configured.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

// idVerifier is the subset of *oidc.IDTokenVerifier that TokenVerifier needs.
// Extracting it as an interface lets tests inject a stub — the concrete oidc
// verifier is hard to fake.
type idVerifier interface {
	Verify(ctx context.Context, rawIDToken string) (*oidc.IDToken, error)
}

// Verifier wraps an OIDC token verifier discovered from an issuer's well-known
// configuration (which yields the JWKS used to check signatures).
type Verifier struct {
	idv idVerifier
}

// New performs OIDC discovery against issuer and returns a Verifier. If audience
// is non-empty it becomes the required `aud` claim; empty disables the audience
// check (signature + issuer + expiry are always enforced). The supplied ctx
// bounds only the one-time discovery fetch — per-request JWKS refresh uses the
// request context at verification time.
func New(ctx context.Context, issuer, audience string) (*Verifier, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery %q: %w", issuer, err)
	}
	return &Verifier{idv: provider.Verifier(&oidc.Config{
		ClientID:          audience,
		SkipClientIDCheck: audience == "",
	})}, nil
}

// identityClaims are the optional human-readable identity fields Authentik emits
// under the profile/email scopes. Subject is always present and is the stable
// fallback when neither is available.
type identityClaims struct {
	Email             string `json:"email"`
	PreferredUsername string `json:"preferred_username"`
}

// TokenVerifier adapts this Verifier to the go-sdk's auth.TokenVerifier contract.
// On success it returns a TokenInfo whose UserID is the verified caller identity,
// which downstream tool handlers read via auth.TokenInfoFromContext to attribute
// writes. A verification failure is wrapped in auth.ErrInvalidToken so the
// RequireBearerToken middleware responds 401.
func (v *Verifier) TokenVerifier() mcpauth.TokenVerifier {
	return func(ctx context.Context, token string, _ *http.Request) (*mcpauth.TokenInfo, error) {
		idt, err := v.idv.Verify(ctx, token)
		if err != nil {
			slog.WarnContext(ctx, "token rejected", "err", err)
			// Join keeps ErrInvalidToken in the chain (so RequireBearerToken maps
			// to 401) while preserving the underlying verification error.
			return nil, errors.Join(mcpauth.ErrInvalidToken, err)
		}
		var claims identityClaims
		// Identity is best-effort: a token may carry only `sub` (no profile/email
		// scope). Subject still uniquely identifies the user, so a Claims decode
		// error must not fail an otherwise-valid token.
		_ = idt.Claims(&claims)
		return &mcpauth.TokenInfo{
			UserID:     identity(idt.Subject, claims.Email, claims.PreferredUsername),
			Expiration: idt.Expiry,
			Extra:      map[string]any{"sub": idt.Subject, "email": claims.Email},
		}, nil
	}
}

// identity prefers a human-readable handle (email, then username) over the opaque
// subject, so attributed memories are legible.
func identity(subject, email, username string) string {
	switch {
	case email != "":
		return email
	case username != "":
		return username
	default:
		return subject
	}
}
