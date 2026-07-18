// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

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
	tokens map[string]string // ownerID -> token
}

// NewStaticTokenVerifier builds a StaticTokenVerifier from an ownerID->token
// map.
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
		for ownerID, candidate := range v.tokens {
			if candidate == "" {
				continue
			}
			if subtle.ConstantTimeCompare([]byte(token), []byte(candidate)) == 1 {
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
			UserID: matchedOwner,
			Extra:  map[string]any{OwnerClaimExtraKey: namespacedOwner("static_token", matchedOwner)},
		}, nil
	}
}
