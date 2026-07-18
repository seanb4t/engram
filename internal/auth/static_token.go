// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

// staticTokenExpirationHorizon is a practically-permanent sentinel expiration
// for static tokens. Static tokens are long-lived by design (revoked by
// removing them from ENGRAM_SERVICE_AUTH_STATIC_TOKENS, never by natural
// expiry), but mcpauth.RequireBearerToken hard-rejects any TokenInfo with a
// zero Expiration ("token missing expiration") — so a genuinely unset
// Expiration is not an option; this horizon is the closest equivalent.
const staticTokenExpirationHorizon = 100 * 365 * 24 * time.Hour

// errStaticTokenNotRecognized is a constant rejection message: it never
// embeds the raw token or a configured candidate value (D-12/DEC-wot no-leak
// discipline) — the returned error is always this same literal, regardless
// of why the match failed.
var errStaticTokenNotRecognized = errors.New("static token not recognized")

// StaticTokenVerifier holds an operator-provisioned map of opaque bearer
// tokens to owner IDs (D-11): each token resolves to exactly one distinct
// owner via namespacedOwner("static_token", ownerID) — never a single shared
// "static service" owner for all tokens, which would defeat tenancy
// isolation (#373). Multiple tokens may map to the same owner, supporting
// rotation with no flag-day cutover: both remain valid at once. An empty map
// disables the mechanism entirely (every verify rejects).
type StaticTokenVerifier struct {
	tokens map[string]string // token -> ownerID
}

// NewStaticTokenVerifier builds a StaticTokenVerifier from a token->ownerID
// map (the orientation config.ParseServiceStaticTokens already produces —
// the presented bearer token is the map key, since verification looks up by
// the presented token).
func NewStaticTokenVerifier(tokens map[string]string) *StaticTokenVerifier {
	return &StaticTokenVerifier{tokens: tokens}
}

// TokenVerifier adapts StaticTokenVerifier to the go-sdk's auth.TokenVerifier
// contract. Every configured candidate is compared with
// crypto/subtle.ConstantTimeCompare over the FULL token value (D-12) — never
// `==`, a prefix check, or strings.Compare — and the loop never returns early
// on a match, so timing cannot reveal which candidate (if any) shares a
// prefix with token. An empty candidate token is never eligible to match; an
// empty input token is rejected outright.
func (v *StaticTokenVerifier) TokenVerifier() mcpauth.TokenVerifier {
	return func(_ context.Context, token string, _ *http.Request) (*mcpauth.TokenInfo, error) {
		if token == "" {
			return nil, errors.Join(mcpauth.ErrInvalidToken, errStaticTokenNotRecognized)
		}
		var matchedOwner string
		matched := false
		for candidateToken, ownerID := range v.tokens {
			if candidateToken == "" {
				continue
			}
			if subtle.ConstantTimeCompare([]byte(token), []byte(candidateToken)) == 1 {
				matchedOwner = ownerID
				matched = true
			}
		}
		if !matched {
			// D-12: this error never interpolates the input token or any
			// configured candidate value.
			return nil, errors.Join(mcpauth.ErrInvalidToken, errStaticTokenNotRecognized)
		}
		return &mcpauth.TokenInfo{
			UserID:     matchedOwner,
			Extra:      map[string]any{OwnerClaimExtraKey: namespacedOwner("static_token", matchedOwner)},
			Expiration: time.Now().Add(staticTokenExpirationHorizon),
		}, nil
	}
}
