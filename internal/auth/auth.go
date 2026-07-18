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
//
// Two constructors build two independently-configured lanes: New for the
// human/no-issuer lane (fail-open — an unresolved owner claim still yields a
// TokenInfo, matching today's behavior) and NewService for the
// client-credentials service lane (fail-closed — an authenticated service
// principal that resolves to an empty owner is rejected at the verifier
// boundary, never silently mapped to the anonymous bucket). Each lane's
// audience check is baked into its own *Verifier at construction time
// (go-oidc has no per-call audience override), so tightening or loosening one
// lane's ENGRAM_*_AUDIENCE never affects the other.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/seanb4t/engram/internal/telemetry"
)

// reservedOwnerNamespace matches the length-prefix grammar (`%d:...`) that
// namespacedOwner produces for non-email claims. A winning email value that
// matches this pattern is rejected rather than written bare, so a crafted
// email cannot impersonate a namespaced service owner.
var reservedOwnerNamespace = regexp.MustCompile(`^[0-9]+:`)

var tracer = otel.Tracer("github.com/seanb4t/engram/internal/auth")

// OwnerClaimExtraKey is the mcpauth.TokenInfo.Extra key carrying the resolved
// owner-claim value. Both auth lanes stamp it (the bearer lane here, the
// Connect cookie lane in internal/webauth) so internal/server's
// SubjectFromTokenInfo has one contract to read instead of three ad hoc
// string literals.
const OwnerClaimExtraKey = "owner_claim"

// idVerifier is the subset of *oidc.IDTokenVerifier that TokenVerifier needs.
// Extracting it as an interface lets tests inject a stub — the concrete oidc
// verifier is hard to fake.
type idVerifier interface {
	Verify(ctx context.Context, rawIDToken string) (*oidc.IDToken, error)
}

// Verifier wraps an OIDC token verifier discovered from an issuer's well-known
// configuration (which yields the JWKS used to check signatures). failClosed
// distinguishes the two lanes New and NewService construct: false (human/
// no-issuer lane, the default zero value) preserves today's fail-open-to-
// anonymous behavior when the owner claim resolves empty; true (service lane,
// set only by NewService) hard-rejects that same empty-owner resolution at
// the TokenVerifier boundary instead (D-08/D-09/D-10).
type Verifier struct {
	idv         idVerifier
	ownerClaims []string
	failClosed  bool
}

// New performs OIDC discovery against issuer and returns a Verifier for the
// human/no-issuer lane (failClosed=false — fail-open on an unresolved owner
// claim, unchanged behavior). If audience is non-empty it becomes the required
// `aud` claim; empty disables the audience check (signature + issuer + expiry
// are always enforced). ownerClaims is the ordered list of OIDC claims tried,
// in order, to resolve a record's owner (default ["email"]); see
// ClaimIdentity. The supplied ctx bounds only the one-time discovery fetch —
// per-request JWKS refresh uses the request context at verification time.
func New(ctx context.Context, issuer, audience string, ownerClaims []string) (*Verifier, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery %q: %w", issuer, err)
	}
	return &Verifier{
		idv: provider.Verifier(&oidc.Config{
			ClientID:          audience,
			SkipClientIDCheck: audience == "",
		}),
		ownerClaims: ownerClaims,
	}, nil
}

// NewService performs OIDC discovery against issuer and returns a Verifier
// for the client-credentials service lane (failClosed=true). It mirrors New
// exactly, except a validated token that resolves to an empty owner is
// hard-rejected by TokenVerifier instead of being returned as a fail-open
// TokenInfo (D-08/D-09/D-10) — an authenticated service principal must never
// land in the anonymous owner=="" bucket. audience is this lane's OWN
// audience check, configured entirely independently of the human lane's (via
// New) — go-oidc bakes the ClientID/SkipClientIDCheck check into the
// *oidc.IDTokenVerifier at construction time, so the service lane always
// needs its own Verifier, never a shared one (D-14). issuer may be the same
// IdP as the human lane (a second discovery round-trip against it) or a
// distinct service issuer. ownerClaims is this lane's OWN owner-claim order
// (default ["client_id","azp"], never the human lane's "email" default,
// D-05).
func NewService(ctx context.Context, issuer, audience string, ownerClaims []string) (*Verifier, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery %q: %w", issuer, err)
	}
	return &Verifier{
		idv: provider.Verifier(&oidc.Config{
			ClientID:          audience,
			SkipClientIDCheck: audience == "",
		}),
		ownerClaims: ownerClaims,
		failClosed:  true,
	}, nil
}

// namespacedOwner encodes a non-email (claim, value) pair as a provably
// injective owner string, using a length-prefixed scheme so that no two
// distinct (claim, value) pairs can ever collide: `("sub","x:y")` and
// `("sub:x","y")` — which collide under the ambiguous "claim:value" form —
// produce distinct strings under this encoding because the byte-length
// prefixes disambiguate any embedded delimiter. Go's len() counts bytes, so
// the prefix is a byte count; this stays injective for multi-byte (Unicode)
// claim names/values too, since the byte-length prefix and the byte payload
// stay consistent.
func namespacedOwner(claim, value string) string {
	return fmt.Sprintf("%d:%s:%d:%s", len(claim), claim, len(value), value)
}

// ClaimIdentity extracts identity fields from a decoded ID-token payload and
// resolves the record owner by walking ownerClaims in order, returning the
// first claim that resolves to a non-empty value. Pure (no I/O) so both auth
// lanes share and unit-test it from a map.
//
// For "email": a present, non-empty, email_verified=true value wins and is
// returned bare (unless it collides with the reserved owner namespace grammar
// `^[0-9]+:`, which is rejected). A present, non-empty, but NOT verified email
// rejects outright — it never falls through to a later claim (D-05). A present
// email whose JSON value is not a string (number/object/array/null) also
// rejects outright, without being coerced to "" and falling through — an
// absent key or a present empty string "" is what is eligible to fall
// through. This presence-vs-type distinction is a deliberate behavior change:
// previously a non-string email under a single-claim ["email"] list silently
// resolved to owner "" with a nil error.
//
// For any other claim: a present, non-empty string value wins and is returned
// as the injective namespaced encoding via namespacedOwner. The same
// presence-vs-type discipline applies — a present but non-string value
// rejects outright rather than falling through to a different claim (which
// would select a different authz bucket).
//
// If every claim in ownerClaims is absent/empty, owner is "" with a nil error
// (fail-closed is preserved; the caller treats an empty owner as fatal). A
// non-nil error always means reject.
func ClaimIdentity(raw map[string]any, ownerClaims []string) (owner, email, username string, err error) {
	email, _ = raw["email"].(string)
	username, _ = raw["preferred_username"].(string)

	for _, claim := range ownerClaims {
		if claim == "email" {
			rawEmail, present := raw["email"]
			if !present {
				continue
			}
			strEmail, ok := rawEmail.(string)
			if !ok {
				return "", "", "", fmt.Errorf("email claim present but not a string")
			}
			if strEmail == "" {
				continue
			}
			// email_verified is read strictly as a JSON bool (the OIDC spec type,
			// and what Authentik emits). A provider sending the string "true"
			// fails this assertion -> false -> reject: fail-closed.
			if verified, _ := raw["email_verified"].(bool); !verified {
				return "", "", "", fmt.Errorf("email not verified")
			}
			if reservedOwnerNamespace.MatchString(strEmail) {
				return "", "", "", fmt.Errorf("email value occupies reserved owner namespace")
			}
			return strEmail, email, username, nil
		}

		rawVal, present := raw[claim]
		if !present {
			continue
		}
		strVal, ok := rawVal.(string)
		if !ok {
			return "", "", "", fmt.Errorf("claim %q present but not a string", claim)
		}
		if strVal == "" {
			continue
		}
		return namespacedOwner(claim, strVal), email, username, nil
	}
	return "", email, username, nil
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
		// best-effort. owner-claim value may be empty here: on the failClosed
		// (service) lane that is rejected immediately below; on the human/
		// no-issuer lane it flows through, and SubjectFromTokenInfo fails
		// closed on an empty owner_claim downstream instead.
		var raw map[string]any
		_ = idt.Claims(&raw)
		ownerVal, email, username, cerr := ClaimIdentity(raw, v.ownerClaims)
		if cerr != nil {
			err = errors.Join(mcpauth.ErrInvalidToken, cerr)
			return nil, err
		}
		if v.failClosed && ownerVal == "" {
			// D-08/D-09: the service lane never falls through to the
			// anonymous owner=="" bucket — reject here, at the verifier
			// boundary, for a clean 401. Never interpolate a token/claim
			// value into this error (no leak of untrusted claim content).
			err = errors.Join(mcpauth.ErrInvalidToken, errors.New("service principal: no resolvable owner claim"))
			return nil, err
		}
		return &mcpauth.TokenInfo{
			UserID:     identity(idt.Subject, email, username),
			Expiration: idt.Expiry,
			Extra:      map[string]any{"sub": idt.Subject, "email": email, OwnerClaimExtraKey: ownerVal},
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
