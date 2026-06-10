// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Authenticator is engram acting as an OIDC confidential client for the web
// login: it discovers the issuer, exchanges auth codes for tokens, and verifies
// ID tokens. It reuses the same issuer the MCP bearer lane already trusts.
type Authenticator struct {
	clientID     string
	clientSecret string
	redirectURL  string
	endpoint     oauth2.Endpoint
	verifier     *oidc.IDTokenVerifier
}

// NewAuthenticator performs OIDC discovery against issuer and returns an
// Authenticator. The ID-token verifier checks signature, issuer, and audience
// (== clientID for the auth-code flow).
func NewAuthenticator(ctx context.Context, issuer, clientID, clientSecret, redirectURL string) (*Authenticator, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	return &Authenticator{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		endpoint:     provider.Endpoint(),
		verifier:     provider.Verifier(&oidc.Config{ClientID: clientID}),
	}, nil
}

// oauthConfig builds the per-flow oauth2.Config. offline_access requests a
// refresh token for the future write phase.
func (a *Authenticator) oauthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     a.clientID,
		ClientSecret: a.clientSecret,
		RedirectURL:  a.redirectURL,
		Endpoint:     a.endpoint,
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email", oidc.ScopeOfflineAccess},
	}
}

// exchange trades an auth code (with its PKCE verifier) for tokens and verifies
// the returned ID token, returning the verified subject.
func (a *Authenticator) exchange(ctx context.Context, code, verifier string) (*oauth2.Token, string, error) {
	tok, err := a.oauthConfig().Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, "", fmt.Errorf("code exchange: %w", err)
	}
	rawID, ok := tok.Extra("id_token").(string)
	if !ok || rawID == "" {
		return nil, "", fmt.Errorf("token response missing id_token")
	}
	idTok, err := a.verifier.Verify(ctx, rawID)
	if err != nil {
		return nil, "", fmt.Errorf("verify id_token: %w", err)
	}
	if idTok.Subject == "" {
		return nil, "", fmt.Errorf("verified id_token has empty subject")
	}
	return tok, idTok.Subject, nil
}
