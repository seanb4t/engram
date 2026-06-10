// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import "fmt"

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
	if eqFold(flag, "false") {
		return UIConfig{Enabled: false}, nil
	}
	present := 0
	for _, k := range requiredUICreds {
		if getenv(k) != "" {
			present++
		}
	}
	allCreds := present == len(requiredUICreds)

	if eqFold(flag, "true") && !allCreds {
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

// eqFold is a tiny ASCII case-insensitive compare (avoids importing strings for
// one call site; matches the repo's lean-import style in cmd/engram).
func eqFold(s, want string) bool {
	if len(s) != len(want) {
		return false
	}
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != want[i] {
			return false
		}
	}
	return true
}
