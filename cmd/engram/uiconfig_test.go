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

func TestResolveUIConfig(t *testing.T) {
	full := map[string]string{
		"MEM_OIDC_CLIENT_ID":     "id",
		"MEM_OIDC_CLIENT_SECRET": "secret",
		"MEM_UI_REDIRECT_URL":    "https://x/auth/callback",
		"MEM_UI_COOKIE_KEY":      "0123456789abcdef0123456789abcdef", // 32 bytes
	}
	cases := []struct {
		name    string
		env     map[string]string
		wantOn  bool
		wantErr bool
	}{
		{"unset and no creds -> headless", map[string]string{}, false, false},
		{"creds present, flag unset -> on", full, true, false},
		{"explicit false wins over creds", merge(full, "MEM_UI_ENABLED", "false"), false, false},
		{"explicit true with full creds -> on", merge(full, "MEM_UI_ENABLED", "true"), true, false},
		{"enabled with partial creds -> error", map[string]string{"MEM_UI_ENABLED": "true", "MEM_OIDC_CLIENT_ID": "id"}, false, true},
		{"creds present but one missing, flag unset -> headless (not an error)", drop(full, "MEM_UI_COOKIE_KEY"), false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := resolveUIConfig(func(k string) string { return tc.env[k] })
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if err == nil && cfg.Enabled != tc.wantOn {
				t.Fatalf("Enabled=%v want %v", cfg.Enabled, tc.wantOn)
			}
		})
	}
}

func merge(m map[string]string, k, v string) map[string]string {
	out := map[string]string{}
	for kk, vv := range m {
		out[kk] = vv
	}
	out[k] = v
	return out
}

func drop(m map[string]string, k string) map[string]string {
	out := map[string]string{}
	for kk, vv := range m {
		if kk != k {
			out[kk] = vv
		}
	}
	return out
}
