// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"testing"

	"golang.org/x/oauth2"
)

func TestOAuthConfigShape(t *testing.T) {
	a := &Authenticator{
		clientID:     "id",
		clientSecret: "secret",
		redirectURL:  "https://x/auth/callback",
		endpoint:     oauth2.Endpoint{AuthURL: "https://issuer/auth", TokenURL: "https://issuer/token"},
	}
	cfg := a.oauthConfig()
	if cfg.ClientID != "id" || cfg.ClientSecret != "secret" {
		t.Fatalf("client creds not wired: %+v", cfg)
	}
	if cfg.RedirectURL != "https://x/auth/callback" {
		t.Fatalf("redirect not wired: %q", cfg.RedirectURL)
	}
	wantScopes := map[string]bool{"openid": true, "profile": true, "email": true, "offline_access": true}
	for _, s := range cfg.Scopes {
		delete(wantScopes, s)
	}
	if len(wantScopes) != 0 {
		t.Fatalf("missing scopes: %v (got %v)", wantScopes, cfg.Scopes)
	}
}
