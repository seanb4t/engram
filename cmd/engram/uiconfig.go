// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// UIConfig is the resolved web-UI activation state. When Enabled is false every
// field except Enabled is zero and the caller mounts nothing (headless).
type UIConfig struct {
	Enabled      bool
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	CookieKey    string
}

// uiRaw is the unresolved web-UI input, built from *config.Config by the caller.
// ClientID/ClientSecret come from the OIDC family (ENGRAM_OIDC_CLIENT_ID/SECRET);
// OIDCIssuer is the MCP-lane issuer used as the UI-issuer fallback.
type uiRaw struct {
	Enabled      string // ENGRAM_UI_ENABLED
	UIIssuer     string // ENGRAM_UI_ISSUER
	OIDCIssuer   string // ENGRAM_OIDC_ISSUER
	ClientID     string // ENGRAM_OIDC_CLIENT_ID
	ClientSecret string // ENGRAM_OIDC_CLIENT_SECRET
	RedirectURL  string // ENGRAM_UI_REDIRECT_URL
	CookieKey    string // ENGRAM_UI_COOKIE_KEY
}

// resolveUIConfig implements the spec's activation tiebreaker (unchanged logic):
//   - Enabled=="false" (any case) is a hard off-switch.
//   - Otherwise the UI is on iff all four required creds are present.
//   - Enabled=="true" with missing creds is a fail-fast startup error.
//   - UIIssuer falls back to OIDCIssuer; enabled with neither is an error.
func resolveUIConfig(r uiRaw) (UIConfig, error) {
	if strings.EqualFold(r.Enabled, "false") {
		return UIConfig{Enabled: false}, nil
	}
	type cred struct {
		env, val string
	}
	creds := []cred{
		{"ENGRAM_OIDC_CLIENT_ID", r.ClientID},
		{"ENGRAM_OIDC_CLIENT_SECRET", r.ClientSecret},
		{"ENGRAM_UI_REDIRECT_URL", r.RedirectURL},
		{"ENGRAM_UI_COOKIE_KEY", r.CookieKey},
	}
	var missing []string
	for _, c := range creds {
		if c.val == "" {
			missing = append(missing, c.env)
		}
	}
	allCreds := len(missing) == 0

	if strings.EqualFold(r.Enabled, "true") && !allCreds {
		return UIConfig{}, fmt.Errorf("ENGRAM_UI_ENABLED=true but missing required creds: %v", missing)
	}
	if !allCreds {
		return UIConfig{Enabled: false}, nil
	}
	issuer := r.UIIssuer
	if issuer == "" {
		issuer = r.OIDCIssuer
	}
	if issuer == "" {
		return UIConfig{}, fmt.Errorf("web UI enabled but no OIDC issuer: set ENGRAM_UI_ISSUER or ENGRAM_OIDC_ISSUER")
	}
	return UIConfig{
		Enabled:      true,
		Issuer:       issuer,
		ClientID:     r.ClientID,
		ClientSecret: r.ClientSecret,
		RedirectURL:  r.RedirectURL,
		CookieKey:    r.CookieKey,
	}, nil
}

// decodeCookieKey turns the ENGRAM_UI_COOKIE_KEY value into the 32-byte AES-256 key
// the session codec requires. Operators provisioning secrets typically supply
// encoded material, so we accept (in precedence order) hex, standard base64, or
// raw-URL base64 that decodes to exactly 32 bytes, and finally a literal 32-byte
// string. Anything else is a fail-fast startup error rather than a confusing
// "key must be 32 bytes" deep in the codec.
func decodeCookieKey(s string) ([]byte, error) {
	if b, err := hex.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	if len(s) == 32 {
		return []byte(s), nil
	}
	return nil, fmt.Errorf("ENGRAM_UI_COOKIE_KEY must decode to 32 bytes (hex, base64, or a literal 32-byte string); got %d chars", len(s))
}
