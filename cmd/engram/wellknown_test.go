// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestProtectedResourceDocument(t *testing.T) {
	t.Run("issuer and resource URL configured", func(t *testing.T) {
		h := protectedResourceHandler(mustResource(t, protectedResourceConfig{ResourceURL: "https://engram.example.test/mcp", MCPPath: "/mcp"}), "https://issuer.example.test")
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
		h := protectedResourceHandler(mustResource(t, protectedResourceConfig{ResourceURL: "https://engram.example.test/mcp", MCPPath: "/mcp"}), "")
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
		h := protectedResourceHandler(mustResource(t, protectedResourceConfig{ResourceURL: "https://engram.example.test/mcp", MCPPath: "/mcp"}), "https://issuer.example.test")
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
		name    string
		mcpPath string
		host    string
		xfProto string
		xfHost  string
		tls     bool
		want    string
	}{
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
			name:    "default https port dropped",
			mcpPath: "/mcp",
			host:    "engram.example.test",
			xfProto: "https",
			xfHost:  "gateway.example.test:443",
			want:    "https://gateway.example.test/mcp",
		},
		{
			name:    "default http port dropped",
			mcpPath: "/mcp",
			host:    "engram.example.test:80",
			want:    "http://engram.example.test/mcp",
		},
		{
			name:    "host lowercased",
			mcpPath: "/mcp",
			host:    "Engram.Example.TEST",
			want:    "http://engram.example.test/mcp",
		},
		{
			name:    "X-Forwarded-Host bracketed IPv6 with port honored",
			mcpPath: "/mcp",
			host:    "engram.example.test",
			xfHost:  "[2001:db8::1]:8443",
			want:    "http://[2001:db8::1]:8443/mcp",
		},
		{
			name:    "X-Forwarded-Host wildcard rejected",
			mcpPath: "/mcp",
			host:    "engram.example.test",
			xfHost:  "*.example.test",
			want:    "http://engram.example.test/mcp",
		},
		{
			name:    "X-Forwarded-Host with a quote rejected",
			mcpPath: "/mcp",
			host:    "engram.example.test",
			xfHost:  `gateway.example.test"`,
			want:    "http://engram.example.test/mcp",
		},
		{
			name:    "X-Forwarded-Host with an empty port rejected",
			mcpPath: "/mcp",
			host:    "engram.example.test",
			xfHost:  "gateway.example.test:",
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
			got := resourceForOrigin(derivedOrigin(r), tc.mcpPath)
			if got != tc.want {
				t.Fatalf("resourceForOrigin(derivedOrigin())=%q want %q", got, tc.want)
			}
		})
	}
}

func TestDerivedOriginEdges(t *testing.T) {
	cases := []struct {
		name, host string
		xfh        []string
		want       string
	}{
		{"two X-Forwarded-Host lines", "engram.example.test", []string{"a.example", "b.example"}, "http://engram.example.test"},
		{"X-Forwarded-Host over 255 bytes", "engram.example.test", []string{strings.Repeat("a", 250) + ".example"}, "http://engram.example.test"},
		{"X-Forwarded-Host with an IPv6 zone", "engram.example.test", []string{"[fe80::1%eth0]"}, "http://engram.example.test"},
		{"trailing dot removed", "Engram.Example.Test.", nil, "http://engram.example.test"},
		{"IPv6 Host in its canonical form", "[0:0:0:0:0:0:0:1]:8080", nil, "http://[::1]:8080"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, wellKnownPRMBase, nil)
			r.Host = tc.host
			for _, v := range tc.xfh {
				r.Header.Add("X-Forwarded-Host", v)
			}
			if got := derivedOrigin(r); got != tc.want {
				t.Fatalf("derivedOrigin=%q want %q", got, tc.want)
			}
		})
	}
}

func TestProtectedResourceCacheControl(t *testing.T) {
	t.Run("derived response carries no-store", func(t *testing.T) {
		h := protectedResourceHandler(mustResource(t, protectedResourceConfig{MCPPath: "/mcp"}), "")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, wellKnownPRMBase, nil))
		if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
			t.Fatalf("Cache-Control=%q want %q", cc, "no-store")
		}
	})

	t.Run("configured response carries no Cache-Control header", func(t *testing.T) {
		h := protectedResourceHandler(mustResource(t, protectedResourceConfig{ResourceURL: "https://engram.example.test/mcp", MCPPath: "/mcp"}), "")
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

func TestValidAuthority(t *testing.T) {
	cases := []struct {
		v    string
		want bool
	}{
		{"a.example", true},
		{"a.example:8443", true},
		{"engram-memory_mcp.svc:8081", true},
		{"192.0.2.1:65535", true},
		{"[2001:db8::1]", true},
		{"[2001:db8::1]:443", true},
		{"a.example.", true},
		{"a.example..", false},
		{"[fe80::1%eth0]", false},
		{"[fe80::1%/evil.example/x]", false},
		{"[::1%a], [::2]", false},
		{"", false},
		{"a.example:", false},
		{"a.example:0443", false},
		{"a.example:0", false},
		{"a.example:65536", false},
		{"a.example:+80", false},
		{".a.example", false},
		{"*.a.example", false},
		{"a.example, b.example", false},
		{"2001:db8::1", false},
		{"[192.0.2.1]", false},
		{"[2001:db8::1]:", false},
		{"user@a.example", false},
		{"a.example/path", false},
	}
	for _, tc := range cases {
		if got := validAuthority(tc.v); got != tc.want {
			t.Errorf("validAuthority(%q)=%v want %v", tc.v, got, tc.want)
		}
	}
}

func TestProtectedResourceChallenge(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Host = "b.example"
	req.Header.Set("X-Forwarded-Proto", "https")

	cases := []struct {
		name                        string
		resourceURL, metadata, path string
		wantChallenge               string
		wantFixed                   bool
	}{
		{
			name:          "configured metadata URL wins even with a pinned resource URL",
			resourceURL:   "https://gateway.example.test/mcp",
			metadata:      "https://configured.example.test/metadata",
			path:          "/mcp",
			wantChallenge: "https://configured.example.test/metadata",
		},
		{
			name:          "pinned, non-root path: suffix form on the pinned origin",
			resourceURL:   "https://gateway.example.test/mcp",
			path:          "/mcp",
			wantChallenge: "https://gateway.example.test/.well-known/oauth-protected-resource/mcp",
		},
		{
			name:          "pinned, root path: host-only form, no trailing slash",
			resourceURL:   "https://gateway.example.test/mcp",
			path:          "/",
			wantChallenge: "https://gateway.example.test/.well-known/oauth-protected-resource",
		},
		{
			name:          "pinned URL's own path and trailing slash discarded",
			resourceURL:   "https://gateway.example.test/ignored/path/",
			path:          "/mcp",
			wantChallenge: "https://gateway.example.test/.well-known/oauth-protected-resource/mcp",
		},
		{
			name:          "derived: the requesting origin",
			path:          "/mcp",
			wantChallenge: "https://b.example/.well-known/oauth-protected-resource/mcp",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := mustResource(t, protectedResourceConfig{ResourceURL: tc.resourceURL, MetadataURL: tc.metadata, MCPPath: tc.path})
			if varies := p.challengeVaries(); varies != (tc.resourceURL == "" && tc.metadata == "") {
				t.Fatalf("challengeVaries()=%v", varies)
			}
			if got := p.challengeURL(req); got != tc.wantChallenge {
				t.Fatalf("challengeURL=%q want %q", got, tc.wantChallenge)
			}
		})
	}

	t.Run("ENGRAM_OIDC_RESOURCE_METADATA is trimmed", func(t *testing.T) {
		p := mustResource(t, protectedResourceConfig{MetadataURL: " https://meta.example/prm ", MCPPath: "/mcp"})
		if got := p.challengeURL(req); got != "https://meta.example/prm" {
			t.Fatalf("challengeURL=%q want the trimmed value", got)
		}
	})
}

func TestNewProtectedResourceRejects(t *testing.T) {
	cases := []struct {
		name, resourceURL, origins string
		wantText                   []string
	}{
		{"both set", "https://a.example/mcp", "https://a.example", []string{"ENGRAM_MCP_RESOURCE_ORIGINS", "ENGRAM_MCP_RESOURCE_URL", "set one of them"}},
		{"malformed resource URL", "ftp://a.example/mcp", "", []string{"ENGRAM_MCP_RESOURCE_URL", `"ftp://a.example/mcp"`}},
		{"malformed origin", "", "https://a.example/mcp", []string{"ENGRAM_MCP_RESOURCE_ORIGINS", `"https://a.example/mcp"`}},
		{"resource URL with userinfo", "https://user:secret@a.example/mcp", "", []string{"ENGRAM_MCP_RESOURCE_URL", "userinfo"}},
		{"resource URL with a wildcard host", "https://*.a.example/mcp", "", []string{"ENGRAM_MCP_RESOURCE_URL", "DNS name or an IP address"}},
		{"resource URL with an empty port", "https://a.example:/mcp", "", []string{"ENGRAM_MCP_RESOURCE_URL", "DNS name or an IP address"}},
		{"resource URL with an empty query", "https://a.example/mcp?", "", []string{"ENGRAM_MCP_RESOURCE_URL", "query or fragment"}},
		{"resource URL with an empty fragment", "https://a.example/mcp#", "", []string{"ENGRAM_MCP_RESOURCE_URL", "query or fragment"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newProtectedResource(protectedResourceConfig{ResourceURL: tc.resourceURL, Origins: tc.origins, MCPPath: "/mcp"})
			if err == nil {
				t.Fatal("want an error")
			}
			for _, want := range tc.wantText {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}

func TestResolveMCPResourceOrigins(t *testing.T) {
	t.Run("valid lists", func(t *testing.T) {
		cases := []struct {
			name string
			raw  string
			want []string
		}{
			{"empty", "", nil},
			{"blank", "  ", nil},
			{"one origin", "https://a.example", []string{"https://a.example"}},
			{"two origins, spaces trimmed", " https://a.example , http://localhost:8080 ", []string{"https://a.example", "http://localhost:8080"}},
			{"scheme and host lowercased", "HTTPS://A.Example:8443", []string{"https://a.example:8443"}},
			{"default https port dropped", "https://a.example:443", []string{"https://a.example"}},
			{"default http port dropped", "http://a.example:80", []string{"http://a.example"}},
			{"other port kept", "https://a.example:80", []string{"https://a.example:80"}},
			{"bracketed IPv6", "http://[::1]:8080", []string{"http://[::1]:8080"}},
			{"IPv6 in its canonical form", "http://[0:0:0:0:0:0:0:1]:8080", []string{"http://[::1]:8080"}},
			{"trailing dot removed", "https://a.example.", []string{"https://a.example"}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := resolveMCPResourceOrigins(tc.raw)
				if err != nil {
					t.Fatalf("resolveMCPResourceOrigins(%q): %v", tc.raw, err)
				}
				if len(got) != len(tc.want) {
					t.Fatalf("resolveMCPResourceOrigins(%q)=%v want %v", tc.raw, got, tc.want)
				}
				for _, o := range tc.want {
					if _, ok := got[o]; !ok {
						t.Fatalf("resolveMCPResourceOrigins(%q)=%v, missing %q", tc.raw, got, o)
					}
				}
			})
		}
	})

	t.Run("rejections", func(t *testing.T) {
		cases := []struct {
			name, raw, quoted, wantText string
		}{
			{"empty entry", "https://a.example,,https://b.example", "https://a.example,,https://b.example", "empty entry"},
			{"trailing comma", "https://a.example,", "https://a.example,", "empty entry"},
			{"duplicate after normalizing", "https://a.example,https://A.EXAMPLE", "https://A.EXAMPLE", "more than once"},
			{"unsupported scheme", "ftp://a.example", "ftp://a.example", "scheme must be http or https"},
			{"no scheme", "a.example", "a.example", "scheme must be http or https"},
			{"path", "https://a.example/mcp", "https://a.example/mcp", "scheme and host only"},
			{"trailing slash", "https://a.example/", "https://a.example/", "scheme and host only"},
			{"query", "https://a.example?x=1", "https://a.example?x=1", "scheme and host only"},
			{"empty query", "https://a.example?", "https://a.example?", "scheme and host only"},
			{"fragment", "https://a.example#frag", "https://a.example#frag", "scheme and host only"},
			{"empty fragment", "https://a.example#", "https://a.example#", "scheme and host only"},
			{"userinfo", "https://user@a.example", "https://user@a.example", "userinfo"},
			{"missing host", "https://", "https://", "missing host"},
			{"duplicate through the default port", "https://a.example,https://a.example:443", "https://a.example:443", "more than once"},
			{"wildcard host", "https://*.a.example", "https://*.a.example", "DNS name or an IP address"},
			{"empty port", "https://a.example:", "https://a.example:", "DNS name or an IP address"},
			{"leading-zero port", "https://a.example:0443", "https://a.example:0443", "DNS name or an IP address"},
			{"duplicate through a trailing dot", "https://a.example,https://a.example.", "https://a.example.", "more than once"},
			{"IPv6 zone", "http://[fe80::1%25eth0]:8080", "http://[fe80::1%25eth0]:8080", "DNS name or an IP address"},
			{"port that is not a number", "https://a.example:abc", "https://a.example:abc", "must be a valid URL"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := resolveMCPResourceOrigins(tc.raw)
				if err == nil {
					t.Fatalf("resolveMCPResourceOrigins(%q): want an error", tc.raw)
				}
				msg := err.Error()
				for _, want := range []string{"ENGRAM_MCP_RESOURCE_ORIGINS", strconv.Quote(tc.quoted), tc.wantText} {
					if !strings.Contains(msg, want) {
						t.Fatalf("error %q does not contain %q", msg, want)
					}
				}
			})
		}
	})
}

// prmThrough requests the path-suffix document from h with the given Host
// and forwarded headers.
func prmThrough(h http.Handler, host, xfProto, xfHost string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, protectedResourcePaths("/mcp"), nil)
	r.Host = host
	if xfProto != "" {
		r.Header.Set("X-Forwarded-Proto", xfProto)
	}
	if xfHost != "" {
		r.Header.Set("X-Forwarded-Host", xfHost)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// captureLogs routes slog to a buffer for the rest of the test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// mustResource returns newProtectedResource(cfg), failing the test on an
// error.
func mustResource(t *testing.T, cfg protectedResourceConfig) *protectedResource {
	t.Helper()
	p, err := newProtectedResource(cfg)
	if err != nil {
		t.Fatalf("newProtectedResource(%+v): %v", cfg, err)
	}
	return p
}

func TestProtectedResourceOriginAllowlist(t *testing.T) {
	served := []struct {
		name, origins, host, xfProto, xfHost, want string
	}{
		{"listed origin", "https://a.example,https://b.example", "engram.internal", "https", "b.example", "https://b.example/mcp"},
		{"listed origin with port", "http://localhost:8080", "localhost:8080", "", "", "http://localhost:8080/mcp"},
		{"host case folded", "https://a.example", "engram.internal", "https", "A.EXAMPLE", "https://a.example/mcp"},
		{"default port in the forwarded host", "https://a.example", "engram.internal", "https", "a.example:443", "https://a.example/mcp"},
		{"empty list keeps the unbounded mode", "", "c.example", "https", "", "https://c.example/mcp"},
	}
	for _, tc := range served {
		t.Run(tc.name, func(t *testing.T) {
			h := protectedResourceHandler(mustResource(t, protectedResourceConfig{Origins: tc.origins, MCPPath: "/mcp"}), "")
			rec := prmThrough(h, tc.host, tc.xfProto, tc.xfHost)
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d want 200", rec.Code)
			}
			var doc protectedResourceMetadata
			if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if doc.Resource != tc.want {
				t.Fatalf("resource=%q want %q", doc.Resource, tc.want)
			}
		})
	}

	refused := []struct {
		name, host, xfProto, xfHost string
	}{
		{"scheme differs from the entry", "a.example", "", ""},
		{"port differs from the entry", "engram.internal", "https", "a.example:8443"},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			h := protectedResourceHandler(mustResource(t, protectedResourceConfig{Origins: "https://a.example", MCPPath: "/mcp"}), "")
			if rec := prmThrough(h, tc.host, tc.xfProto, tc.xfHost); rec.Code != http.StatusNotFound {
				t.Fatalf("status=%d want 404", rec.Code)
			}
		})
	}

	t.Run("spoofed forwarded host outside the list", func(t *testing.T) {
		logs := captureLogs(t)
		h := protectedResourceHandler(mustResource(t, protectedResourceConfig{Origins: "https://a.example", MCPPath: "/mcp"}), "")
		rec := prmThrough(h, "a.example", "https", "c.example")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d want 404", rec.Code)
		}
		if strings.Contains(rec.Body.String(), "c.example") {
			t.Fatalf("404 body names the spoofed host: %q", rec.Body.String())
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
			t.Fatalf("Cache-Control=%q want no-store", cc)
		}
		if v := rec.Header().Get("Vary"); !strings.Contains(v, "X-Forwarded-Host") {
			t.Fatalf("Vary=%q want X-Forwarded-Host in it", v)
		}
		if !strings.Contains(logs.String(), `"origin":"https://c.example"`) {
			t.Fatalf("no warning names https://c.example: %s", logs.String())
		}
	})

	t.Run("repeated unlisted origin warns once and counts every refusal", func(t *testing.T) {
		logs := captureLogs(t)
		var counted int
		p := mustResource(t, protectedResourceConfig{Origins: "https://a.example", MCPPath: "/mcp", OnRefused: func(context.Context) { counted++ }})
		h := protectedResourceHandler(p, "")
		for range 10 {
			prmThrough(h, "c.example", "https", "")
		}
		if n := strings.Count(logs.String(), `"origin":"https://c.example"`); n != 1 {
			t.Fatalf("warnings for https://c.example=%d want 1: %s", n, logs.String())
		}
		if counted != 10 {
			t.Fatalf("onRefused calls=%d want 10", counted)
		}
	})

	t.Run("warnings stop at the limit", func(t *testing.T) {
		logs := captureLogs(t)
		h := protectedResourceHandler(mustResource(t, protectedResourceConfig{Origins: "https://a.example", MCPPath: "/mcp"}), "")
		for i := range maxWarnedOrigins + 10 {
			prmThrough(h, "c"+strconv.Itoa(i)+".example", "https", "")
		}
		if n := strings.Count(logs.String(), `"origin":`); n != maxWarnedOrigins {
			t.Fatalf("origin warnings=%d want %d", n, maxWarnedOrigins)
		}
		if n := strings.Count(logs.String(), "warning limit reached"); n != 1 {
			t.Fatalf("limit warnings=%d want 1", n)
		}
	})

	t.Run("the host-only path answers an unlisted origin", func(t *testing.T) {
		logs := captureLogs(t)
		h := protectedResourceHandler(mustResource(t, protectedResourceConfig{Origins: "https://a.example", MCPPath: "/mcp"}), "https://issuer.example.test")
		r := httptest.NewRequest(http.MethodGet, wellKnownPRMBase, nil)
		r.Host = "c.example"
		r.Header.Set("X-Forwarded-Proto", "https")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d want 200", rec.Code)
		}
		var doc protectedResourceMetadata
		if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if want := []string{"https://issuer.example.test"}; !prmEqualStrSlices(doc.AuthorizationServers, want) {
			t.Fatalf("authorization_servers=%v want %v", doc.AuthorizationServers, want)
		}
		if doc.Resource != "https://c.example/mcp" {
			t.Fatalf("resource=%q want %q", doc.Resource, "https://c.example/mcp")
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
			t.Fatalf("Cache-Control=%q want no-store", cc)
		}
		if strings.Contains(logs.String(), "ENGRAM_MCP_RESOURCE_ORIGINS") {
			t.Fatalf("the host-only document logged a refusal: %s", logs.String())
		}
	})

	t.Run("a long origin is cut in the warning", func(t *testing.T) {
		logs := captureLogs(t)
		h := protectedResourceHandler(mustResource(t, protectedResourceConfig{Origins: "https://a.example", MCPPath: "/mcp"}), "")
		host := "c" + strings.Repeat("a", 2*maxWarnedOriginLen) + ".example"
		prmThrough(h, host, "https", "")
		want := ("https://" + host)[:maxWarnedOriginLen]
		if !strings.Contains(logs.String(), `"origin":"`+want+`"`) {
			t.Fatalf("no warning names the origin cut to %d bytes: %s", maxWarnedOriginLen, logs.String())
		}
	})

	t.Run("warnings start again after the window", func(t *testing.T) {
		logs := captureLogs(t)
		p := mustResource(t, protectedResourceConfig{Origins: "https://a.example", MCPPath: "/mcp"})
		now := time.Unix(0, 0)
		p.warner.now = func() time.Time { return now }
		h := protectedResourceHandler(p, "")
		for i := range maxWarnedOrigins + 1 {
			prmThrough(h, "c"+strconv.Itoa(i)+".example", "https", "")
		}
		now = now.Add(warnWindow)
		prmThrough(h, "late.example", "https", "")
		if n := strings.Count(logs.String(), `"origin":`); n != maxWarnedOrigins+1 {
			t.Fatalf("origin warnings=%d want %d", n, maxWarnedOrigins+1)
		}
		if !strings.Contains(logs.String(), `"origin":"https://late.example"`) {
			t.Fatalf("no warning for an origin refused after the window: %s", logs.String())
		}
	})

	t.Run("the warning names an ignored forwarded host", func(t *testing.T) {
		logs := captureLogs(t)
		h := protectedResourceHandler(mustResource(t, protectedResourceConfig{Origins: "https://a.example", MCPPath: "/mcp"}), "")
		prmThrough(h, "c.example", "https", "a.example, evil.example")
		if !strings.Contains(logs.String(), `"origin":"https://c.example"`) || !strings.Contains(logs.String(), `"ignored_x_forwarded_host":"a.example, evil.example"`) {
			t.Fatalf("warning does not name the origin and the ignored forwarded host: %s", logs.String())
		}
	})

	t.Run("pinned resource ignores forwarded headers", func(t *testing.T) {
		h := protectedResourceHandler(mustResource(t, protectedResourceConfig{ResourceURL: "https://a.example/mcp", MCPPath: "/mcp"}), "")
		rec := prmThrough(h, "a.example", "https", "b.example")
		var doc protectedResourceMetadata
		if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if doc.Resource != "https://a.example/mcp" {
			t.Fatalf("resource=%q want the configured URL", doc.Resource)
		}
	})
}
