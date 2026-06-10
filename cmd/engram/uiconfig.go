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
	ClientID     string
	ClientSecret string
	RedirectURL  string
	CookieKey    string
}

// requiredUICreds are all-or-nothing: enabling the UI without all of them is a
// fail-fast startup error, never a silent half-on state.
var requiredUICreds = []string{
	"MEM_OIDC_CLIENT_ID",
	"MEM_OIDC_CLIENT_SECRET",
	"MEM_UI_REDIRECT_URL",
	"MEM_UI_COOKIE_KEY",
}

// resolveUIConfig implements the spec's activation tiebreaker:
//   - MEM_UI_ENABLED=="false" (any case) is a hard off-switch — headless even
//     when creds are present.
//   - Otherwise the UI is enabled iff all required creds are present.
//   - MEM_UI_ENABLED=="true" with missing creds is a startup error (fail fast),
//     NOT a silent half-on state.
//   - MEM_UI_ENABLED unset with partial creds is headless (not an error):
//     presence-of-all-creds implies enabled; partial implies the operator has
//     not finished wiring it.
func resolveUIConfig(getenv func(string) string) (UIConfig, error) {
	flag := getenv("MEM_UI_ENABLED")
	if strings.EqualFold(flag, "false") {
		return UIConfig{Enabled: false}, nil
	}
	present := 0
	for _, k := range requiredUICreds {
		if getenv(k) != "" {
			present++
		}
	}
	allCreds := present == len(requiredUICreds)

	if strings.EqualFold(flag, "true") && !allCreds {
		var missing []string
		for _, k := range requiredUICreds {
			if getenv(k) == "" {
				missing = append(missing, k)
			}
		}
		return UIConfig{}, fmt.Errorf("MEM_UI_ENABLED=true but missing required creds: %v", missing)
	}
	if !allCreds {
		return UIConfig{Enabled: false}, nil
	}
	return UIConfig{
		Enabled:      true,
		ClientID:     getenv("MEM_OIDC_CLIENT_ID"),
		ClientSecret: getenv("MEM_OIDC_CLIENT_SECRET"),
		RedirectURL:  getenv("MEM_UI_REDIRECT_URL"),
		CookieKey:    getenv("MEM_UI_COOKIE_KEY"),
	}, nil
}

// decodeCookieKey turns the MEM_UI_COOKIE_KEY value into the 32-byte AES-256 key
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
	return nil, fmt.Errorf("MEM_UI_COOKIE_KEY must decode to 32 bytes (hex, base64, or a literal 32-byte string); got %d chars", len(s))
}
