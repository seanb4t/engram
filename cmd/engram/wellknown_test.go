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
		suffix := protectedResourcePaths("/mcp")

		recBase := httptest.NewRecorder()
		h.ServeHTTP(recBase, httptest.NewRequest(http.MethodGet, wellKnownPRMBase, nil))
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

func TestMountWellKnownRoutes(t *testing.T) {
	const mcpHdr = "X-Mcp-Stub"
	mcpStub := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(mcpHdr, "hit")
		w.WriteHeader(http.StatusOK)
	})
	const metaHdr = "X-Meta-Stub"
	metaStub := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(metaHdr, "hit")
		w.WriteHeader(http.StatusOK)
	})

	t.Run("non-escape-hatch mode", func(t *testing.T) {
		const mcpPath = "/mcp"
		mux := http.NewServeMux()
		mountMCPRoutes(mux, mcpStub, false, mcpPath)
		mountWellKnownRoutes(mux, metaStub, mcpPath)
		suffix := protectedResourcePaths(mcpPath)

		cases := []struct {
			name     string
			method   string
			path     string
			wantCode int
			wantMeta bool
			wantMCP  bool
		}{
			{"GET host-only well-known reaches metadata", http.MethodGet, wellKnownPRMBase, http.StatusOK, true, false},
			{"GET suffix well-known reaches metadata", http.MethodGet, suffix, http.StatusOK, true, false},
			{"POST host-only well-known is 404", http.MethodPost, wellKnownPRMBase, http.StatusNotFound, false, false},
			{"GET unrelated well-known path is 404", http.MethodGet, "/.well-known/nope", http.StatusNotFound, false, false},
			{"GET /mcp reaches MCP", http.MethodGet, mcpPath, http.StatusOK, false, true},
			{"POST /mcp reaches MCP", http.MethodPost, mcpPath, http.StatusOK, false, true},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
				if rec.Code != tc.wantCode {
					t.Fatalf("%s %s: status=%d want %d", tc.method, tc.path, rec.Code, tc.wantCode)
				}
				if got := rec.Header().Get(metaHdr) == "hit"; got != tc.wantMeta {
					t.Fatalf("%s %s: reachedMeta=%v want %v", tc.method, tc.path, got, tc.wantMeta)
				}
				if got := rec.Header().Get(mcpHdr) == "hit"; got != tc.wantMCP {
					t.Fatalf("%s %s: reachedMCP=%v want %v", tc.method, tc.path, got, tc.wantMCP)
				}
			})
		}
	})

	t.Run("escape-hatch mode", func(t *testing.T) {
		const mcpPath = "/"
		mux := http.NewServeMux()
		mountMCPRoutes(mux, mcpStub, false, mcpPath)
		mountWellKnownRoutes(mux, metaStub, mcpPath)
		suffix := protectedResourcePaths(mcpPath)

		if suffix != "" {
			t.Fatalf("protectedResourcePaths(%q)=%q want empty (root MCP path)", mcpPath, suffix)
		}

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, wellKnownPRMBase, nil))
		if rec.Code != http.StatusOK || rec.Header().Get(metaHdr) != "hit" {
			t.Fatalf("GET %s: status=%d metaHit=%v want 200/true", wellKnownPRMBase, rec.Code, rec.Header().Get(metaHdr) == "hit")
		}

		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, wellKnownPRMBase, nil))
		if rec.Code != http.StatusOK || rec.Header().Get(mcpHdr) != "hit" {
			t.Fatalf("POST %s: status=%d mcpHit=%v want 200/true (legacy root catch-all must not be narrowed)", wellKnownPRMBase, rec.Code, rec.Header().Get(mcpHdr) == "hit")
		}
	})
}

func TestResolveResourceMetadataURL(t *testing.T) {
	cases := []struct {
		name           string
		configured     string
		mcpResourceURL string
		mcpPath        string
		want           string
	}{
		{
			name:           "configured wins even when mcpResourceURL also set",
			configured:     "https://configured.example.test/metadata",
			mcpResourceURL: "https://gateway.example.test/mcp",
			mcpPath:        "/mcp",
			want:           "https://configured.example.test/metadata",
		},
		{
			name: "both empty: empty result (today's shipped behaviour)",
			want: "",
		},
		{
			name:           "derived from mcpResourceURL, non-root mcpPath: suffix form",
			mcpResourceURL: "https://gateway.example.test/mcp",
			mcpPath:        "/mcp",
			want:           "https://gateway.example.test/.well-known/oauth-protected-resource/mcp",
		},
		{
			name:           "derived from mcpResourceURL, root mcpPath: host-only form, no trailing slash",
			mcpResourceURL: "https://gateway.example.test/mcp",
			mcpPath:        "/",
			want:           "https://gateway.example.test/.well-known/oauth-protected-resource",
		},
		{
			name:           "mcpResourceURL's own path/query/trailing-slash discarded",
			mcpResourceURL: "https://gateway.example.test/ignored/path?x=1#frag/",
			mcpPath:        "/mcp",
			want:           "https://gateway.example.test/.well-known/oauth-protected-resource/mcp",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveResourceMetadataURL(tc.configured, tc.mcpResourceURL, tc.mcpPath)
			if got != tc.want {
				t.Fatalf("resolveResourceMetadataURL(%q, %q, %q)=%q want %q", tc.configured, tc.mcpResourceURL, tc.mcpPath, got, tc.want)
			}
		})
	}
}
