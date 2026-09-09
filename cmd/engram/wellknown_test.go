// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProtectedResourceDocument(t *testing.T) {
	t.Run("issuer and resource URL configured", func(t *testing.T) {
		h := protectedResourceHandler("https://engram.example.test/mcp", "https://issuer.example.test", "/mcp")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, wellKnownPRMBase, nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d want 200", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("Content-Type=%q want %q", ct, "application/json")
		}
		var doc protectedResourceMetadata
		if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if doc.Resource != "https://engram.example.test/mcp" {
			t.Fatalf("resource=%q want configured URL", doc.Resource)
		}
		if want := []string{"https://issuer.example.test"}; !prmEqualStrSlices(doc.AuthorizationServers, want) {
			t.Fatalf("authorization_servers=%v want %v", doc.AuthorizationServers, want)
		}
		if want := []string{"header"}; !prmEqualStrSlices(doc.BearerMethodsSupported, want) {
			t.Fatalf("bearer_methods_supported=%v want %v", doc.BearerMethodsSupported, want)
		}
		if want := []string{"offline_access"}; !prmEqualStrSlices(doc.ScopesSupported, want) {
			t.Fatalf("scopes_supported=%v want %v", doc.ScopesSupported, want)
		}
	})

	t.Run("issuer empty: authorization_servers key absent", func(t *testing.T) {
		h := protectedResourceHandler("https://engram.example.test/mcp", "", "/mcp")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, wellKnownPRMBase, nil))

		var raw map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if _, ok := raw["authorization_servers"]; ok {
			t.Fatalf("authorization_servers key present, want absent: %v", raw)
		}
	})

	t.Run("both well-known paths produce byte-identical bodies", func(t *testing.T) {
		h := protectedResourceHandler("https://engram.example.test/mcp", "https://issuer.example.test", "/mcp")
		base, suffix := protectedResourcePaths("/mcp")

		recBase := httptest.NewRecorder()
		h.ServeHTTP(recBase, httptest.NewRequest(http.MethodGet, base, nil))
		recSuffix := httptest.NewRecorder()
		h.ServeHTTP(recSuffix, httptest.NewRequest(http.MethodGet, suffix, nil))

		if recBase.Body.String() != recSuffix.Body.String() {
			t.Fatalf("bodies differ:\nbase=%s\nsuffix=%s", recBase.Body.String(), recSuffix.Body.String())
		}
	})
}

func prmEqualStrSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestProtectedResourceResourceURLDerivation(t *testing.T) {
	cases := []struct {
		name       string
		configured string
		mcpPath    string
		host       string
		xfProto    string
		xfHost     string
		tls        bool
		want       string
	}{
		{
			name:       "configured wins over spoofed X-Forwarded-Host",
			configured: "https://trusted.example.test/mcp",
			mcpPath:    "/mcp",
			host:       "trusted.example.test",
			xfHost:     "attacker.example.test",
			want:       "https://trusted.example.test/mcp",
		},
		{
			name:    "derived: plain host, no forwarded headers",
			mcpPath: "/mcp",
			host:    "engram.example.test",
			want:    "http://engram.example.test/mcp",
		},
		{
			name:    "derived: request over TLS",
			mcpPath: "/mcp",
			host:    "engram.example.test",
			tls:     true,
			want:    "https://engram.example.test/mcp",
		},
		{
			name:    "X-Forwarded-Proto https honored",
			mcpPath: "/mcp",
			host:    "engram.example.test",
			xfProto: "https",
			want:    "https://engram.example.test/mcp",
		},
		{
			name:    "X-Forwarded-Proto bogus falls back to TLS-derived scheme",
			mcpPath: "/mcp",
			host:    "engram.example.test",
			xfProto: "ftp",
			want:    "http://engram.example.test/mcp",
		},
		{
			name:    "X-Forwarded-Host bare host honored",
			mcpPath: "/mcp",
			host:    "engram.example.test",
			xfHost:  "gateway.example.test",
			want:    "http://gateway.example.test/mcp",
		},
		{
			name:    "X-Forwarded-Host bare host:port honored",
			mcpPath: "/mcp",
			host:    "engram.example.test",
			xfHost:  "gateway.example.test:8443",
			want:    "http://gateway.example.test:8443/mcp",
		},
		{
			name:    "X-Forwarded-Host comma list rejected",
			mcpPath: "/mcp",
			host:    "engram.example.test",
			xfHost:  "attacker.example.test, gateway.example.test",
			want:    "http://engram.example.test/mcp",
		},
		{
			name:    "X-Forwarded-Host with slash rejected",
			mcpPath: "/mcp",
			host:    "engram.example.test",
			xfHost:  "gateway.example.test/evil",
			want:    "http://engram.example.test/mcp",
		},
		{
			name:    "X-Forwarded-Host with at sign rejected",
			mcpPath: "/mcp",
			host:    "engram.example.test",
			xfHost:  "user@gateway.example.test",
			want:    "http://engram.example.test/mcp",
		},
		{
			name:    "X-Forwarded-Host with whitespace rejected",
			mcpPath: "/mcp",
			host:    "engram.example.test",
			xfHost:  "gateway.example.test ",
			want:    "http://engram.example.test/mcp",
		},
		{
			name:    "X-Forwarded-Host empty rejected",
			mcpPath: "/mcp",
			host:    "engram.example.test",
			xfHost:  "",
			want:    "http://engram.example.test/mcp",
		},
		{
			name:    "mcpPath bare root: no path component appended",
			mcpPath: "/",
			host:    "engram.example.test",
			want:    "http://engram.example.test",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, wellKnownPRMBase, nil)
			r.Host = tc.host
			if tc.xfProto != "" {
				r.Header.Set("X-Forwarded-Proto", tc.xfProto)
			}
			if tc.xfHost != "" {
				r.Header.Set("X-Forwarded-Host", tc.xfHost)
			} else if tc.name == "X-Forwarded-Host empty rejected" {
				r.Header.Set("X-Forwarded-Host", "")
			}
			if tc.tls {
				r.TLS = &tls.ConnectionState{}
			}
			got := resourceURLFor(tc.configured, tc.mcpPath, r)
			if got != tc.want {
				t.Fatalf("resourceURLFor()=%q want %q", got, tc.want)
			}
		})
	}
}

func TestProtectedResourceCacheControl(t *testing.T) {
	t.Run("derived response carries no-store", func(t *testing.T) {
		h := protectedResourceHandler("", "", "/mcp")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, wellKnownPRMBase, nil))
		if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
			t.Fatalf("Cache-Control=%q want %q", cc, "no-store")
		}
	})

	t.Run("configured response carries no Cache-Control header", func(t *testing.T) {
		h := protectedResourceHandler("https://engram.example.test/mcp", "", "/mcp")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, wellKnownPRMBase, nil))
		if cc := rec.Header().Get("Cache-Control"); cc != "" {
			t.Fatalf("Cache-Control=%q want absent", cc)
		}
	})
}
