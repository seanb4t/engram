// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestProtectedResourceDocument(t *testing.T) {
	t.Run("issuer and resource URL configured", func(t *testing.T) {
		h := protectedResourceHandler("https://engram.example.test/mcp", "https://issuer.example.test", "/mcp", nil)
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
		h := protectedResourceHandler("https://engram.example.test/mcp", "", "/mcp", nil)
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
		h := protectedResourceHandler("https://engram.example.test/mcp", "https://issuer.example.test", "/mcp", nil)
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

func TestProtectedResourceCacheControl(t *testing.T) {
	t.Run("derived response carries no-store", func(t *testing.T) {
		h := protectedResourceHandler("", "", "/mcp", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, wellKnownPRMBase, nil))
		if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
			t.Fatalf("Cache-Control=%q want %q", cc, "no-store")
		}
	})

	t.Run("configured response carries no Cache-Control header", func(t *testing.T) {
		h := protectedResourceHandler("https://engram.example.test/mcp", "", "/mcp", nil)
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
			{"default port kept as written", "https://a.example:443", []string{"https://a.example:443"}},
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

// captureWarnings routes slog to a buffer for the rest of the test.
func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func mustOrigins(t *testing.T, raw string) *originGate {
	t.Helper()
	set, err := resolveMCPResourceOrigins(raw)
	if err != nil {
		t.Fatalf("resolveMCPResourceOrigins(%q): %v", raw, err)
	}
	return newOriginGate(set)
}

func TestProtectedResourceOriginAllowlist(t *testing.T) {
	served := []struct {
		name, origins, host, xfProto, xfHost, want string
	}{
		{"listed origin", "https://a.example,https://b.example", "engram.internal", "https", "b.example", "https://b.example/mcp"},
		{"listed origin with port", "http://localhost:8080", "localhost:8080", "", "", "http://localhost:8080/mcp"},
		{"host case folded", "https://a.example", "engram.internal", "https", "A.EXAMPLE", "https://a.example/mcp"},
		{"empty list keeps the unbounded mode", "", "c.example", "https", "", "https://c.example/mcp"},
	}
	for _, tc := range served {
		t.Run(tc.name, func(t *testing.T) {
			h := protectedResourceHandler("", "", "/mcp", mustOrigins(t, tc.origins))
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

	t.Run("spoofed forwarded host outside the list", func(t *testing.T) {
		logs := captureWarnings(t)
		h := protectedResourceHandler("", "", "/mcp", mustOrigins(t, "https://a.example"))
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
		if !strings.Contains(logs.String(), `"origin":"https://c.example"`) {
			t.Fatalf("no warning names https://c.example: %s", logs.String())
		}
	})

	t.Run("repeated unlisted origin warns once", func(t *testing.T) {
		logs := captureWarnings(t)
		h := protectedResourceHandler("", "", "/mcp", mustOrigins(t, "https://a.example"))
		for range 10 {
			prmThrough(h, "c.example", "https", "")
		}
		if n := strings.Count(logs.String(), `"origin":"https://c.example"`); n != 1 {
			t.Fatalf("warnings for https://c.example=%d want 1: %s", n, logs.String())
		}
	})

	t.Run("warnings stop at the limit", func(t *testing.T) {
		logs := captureWarnings(t)
		h := protectedResourceHandler("", "", "/mcp", mustOrigins(t, "https://a.example"))
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

	t.Run("pinned resource ignores forwarded headers", func(t *testing.T) {
		h := protectedResourceHandler("https://a.example/mcp", "", "/mcp", nil)
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
