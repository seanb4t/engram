// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package auth validates OIDC bearer tokens issued by a configured IdP and
// forwarded by an MCP gateway, then extracts the caller's identity so memory
// writes can be attributed to a verified user (canonical deployment: Authentik
// IdP + OIDC-aware embedding gateway).
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
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/seanb4t/engram/internal/telemetry"
)

var tracer = otel.Tracer("github.com/seanb4t/engram/internal/auth")

// idVerifier is the subset of *oidc.IDTokenVerifier that TokenVerifier needs.
// Extracting it as an interface lets tests inject a stub — the concrete oidc
// verifier is hard to fake.
type idVerifier interface {
	Verify(ctx context.Context, rawIDToken string) (*oidc.IDToken, error)
}

// Verifier wraps an OIDC token verifier discovered from an issuer's well-known
// configuration (which yields the JWKS used to check signatures).
type Verifier struct {
	idv        idVerifier
	ownerClaim string
}

// New performs OIDC discovery against issuer and returns a Verifier. If audience
// is non-empty it becomes the required `aud` claim; empty disables the audience
// check (signature + issuer + expiry are always enforced). ownerClaim names the
// OIDC claim whose value becomes a record's owner (default "email"). The supplied
// ctx bounds only the one-time discovery fetch — per-request JWKS refresh uses
// the request context at verification time.
func New(ctx context.Context, issuer, audience, ownerClaim string) (*Verifier, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery %q: %w", issuer, err)
	}
	return &Verifier{
		idv: provider.Verifier(&oidc.Config{
			ClientID:          audience,
			SkipClientIDCheck: audience == "",
		}),
		ownerClaim: ownerClaim,
	}, nil
}

// ClaimIdentity extracts identity fields from a decoded ID-token payload and
// enforces email_verified when ownerClaim=="email". Pure (no I/O) so both auth
// lanes share and unit-test it from a map. owner MAY be "" (caller decides if
// fatal; the read seam SubjectFromTokenInfo defers rejection of an empty owner to
// Task 3). A non-nil error means reject (currently only the email_verified gate).
// Absent email_verified => false => reject.
func ClaimIdentity(raw map[string]any, ownerClaim string) (owner, email, username string, err error) {
	email, _ = raw["email"].(string)
	username, _ = raw["preferred_username"].(string)
	owner, _ = raw[ownerClaim].(string)
	if ownerClaim == "email" {
		if verified, _ := raw["email_verified"].(bool); !verified {
			return "", "", "", fmt.Errorf("email not verified")
		}
	}
	return owner, email, username, nil
}

// TokenVerifier adapts this Verifier to the go-sdk's auth.TokenVerifier contract.
// On success it returns a TokenInfo whose UserID is the verified caller identity,
// which downstream tool handlers read via auth.TokenInfoFromContext to attribute
// writes. A verification failure is wrapped in auth.ErrInvalidToken so the
// RequireBearerToken middleware responds 401.
func (v *Verifier) TokenVerifier() mcpauth.TokenVerifier {
	return func(ctx context.Context, token string, _ *http.Request) (info *mcpauth.TokenInfo, err error) {
		ctx, span := tracer.Start(ctx, "auth.VerifyToken")
		defer span.End()
		start := time.Now()
		defer func() {
			telemetry.RecordAuthVerify(ctx, start, err)
			if err != nil {
				span.SetAttributes(attribute.String("engram.auth.outcome", "error"))
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			} else {
				span.SetAttributes(attribute.String("engram.auth.outcome", "ok"))
			}
		}()

		idt, verr := v.idv.Verify(ctx, token)
		if verr != nil {
			slog.WarnContext(ctx, "token rejected", "err", verr)
			// Join keeps ErrInvalidToken in the chain (so RequireBearerToken maps
			// to 401) while preserving the underlying verification error.
			err = errors.Join(mcpauth.ErrInvalidToken, verr)
			return nil, err
		}
		// Decode the full payload so the configured owner-claim can be read by
		// name. ClaimIdentity enforces email_verified; identity (UserID) stays
		// best-effort. owner-claim value may be empty here — SubjectFromTokenInfo
		// fails closed on an empty owner_claim downstream.
		var raw map[string]any
		_ = idt.Claims(&raw)
		ownerVal, email, username, cerr := ClaimIdentity(raw, v.ownerClaim)
		if cerr != nil {
			err = errors.Join(mcpauth.ErrInvalidToken, cerr)
			return nil, err
		}
		return &mcpauth.TokenInfo{
			UserID:     identity(idt.Subject, email, username),
			Expiration: idt.Expiry,
			Extra:      map[string]any{"sub": idt.Subject, "email": email, "owner_claim": ownerVal},
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
