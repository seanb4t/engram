// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

func TestDecodeCookieKey(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i)
	}
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"hex 32 bytes", hex.EncodeToString(raw), false},
		{"base64 std 32 bytes", base64.StdEncoding.EncodeToString(raw), false},
		{"base64 rawurl 32 bytes", base64.RawURLEncoding.EncodeToString(raw), false},
		{"literal 32-byte string", strings.Repeat("k", 32), false},
		{"hex of 20 bytes (wrong length, not 32 chars)", hex.EncodeToString(raw[:20]), true},
		{"too short", "deadbeef", true},
		{"empty", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := decodeCookieKey(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if err == nil && len(b) != 32 {
				t.Fatalf("decoded key is %d bytes, want 32", len(b))
			}
		})
	}
}

func TestResolveUIConfigDefaultsIssuerToOIDC(t *testing.T) {
	got, err := resolveUIConfig(uiRaw{
		Enabled: "true", OIDCIssuer: "https://oidc", ClientID: "c", ClientSecret: "s",
		RedirectURL: "https://cb", CookieKey: "0123456789abcdef0123456789abcdef",
	})
	if err != nil {
		t.Fatalf("resolveUIConfig: %v", err)
	}
	if !got.Enabled || got.Issuer != "https://oidc" {
		t.Errorf("got %+v, want enabled with issuer defaulted to OIDC issuer", got)
	}
}

func TestResolveUIConfigUIIssuerOverrides(t *testing.T) {
	got, err := resolveUIConfig(uiRaw{
		UIIssuer: "https://ui", OIDCIssuer: "https://oidc", ClientID: "c", ClientSecret: "s",
		RedirectURL: "https://cb", CookieKey: "0123456789abcdef0123456789abcdef",
	})
	if err != nil {
		t.Fatalf("resolveUIConfig: %v", err)
	}
	if got.Issuer != "https://ui" {
		t.Errorf("Issuer = %q, want ui issuer to win", got.Issuer)
	}
}

func TestResolveUIConfigEnabledTrueMissingCreds(t *testing.T) {
	_, err := resolveUIConfig(uiRaw{Enabled: "true"})
	if err == nil {
		t.Fatal("want error for ENGRAM_UI_ENABLED=true with missing creds")
	}
}

func TestResolveUIConfigNoIssuerError(t *testing.T) {
	_, err := resolveUIConfig(uiRaw{
		ClientID: "c", ClientSecret: "s", RedirectURL: "https://cb",
		CookieKey: "0123456789abcdef0123456789abcdef",
	})
	if err == nil {
		t.Fatal("want error: enabled-by-creds but no issuer")
	}
}

func TestResolveUIConfigFalseIsHardOff(t *testing.T) {
	got, err := resolveUIConfig(uiRaw{
		Enabled: "false", OIDCIssuer: "https://oidc", ClientID: "c", ClientSecret: "s",
		RedirectURL: "https://cb", CookieKey: "0123456789abcdef0123456789abcdef",
	})
	if err != nil || got.Enabled {
		t.Errorf("got %+v err %v, want disabled", got, err)
	}
}

func TestResolveUIConfigUnsetNoCredsIsHeadless(t *testing.T) {
	got, err := resolveUIConfig(uiRaw{})
	if err != nil || got.Enabled {
		t.Errorf("got %+v err %v, want headless (flag unset, no creds → implied off, not an error)", got, err)
	}
}

func TestResolveUIConfigUnsetPartialCredsIsHeadless(t *testing.T) {
	got, err := resolveUIConfig(uiRaw{OIDCIssuer: "https://oidc", ClientID: "c"})
	if err != nil || got.Enabled {
		t.Errorf("got %+v err %v, want headless (flag unset, partial creds → implied off, not an error)", got, err)
	}
}
