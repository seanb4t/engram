// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveMCPPath(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"empty defaults to /mcp", "", "/mcp", false},
		{"explicit /mcp", "/mcp", "/mcp", false},
		{"custom path", "/transport", "/transport", false},
		{"root escape hatch", "/", "/", false},
		{"whitespace trimmed then defaulted", "   ", "/mcp", false},
		{"whitespace trimmed around value", "  /mcp  ", "/mcp", false},
		{"missing leading slash is an error", "mcp", "", true},
		// Reserved paths collide with the console/auth/Connect mounts and would
		// panic http.ServeMux at startup — reject them with a clean error.
		{"reserved /ui is an error", "/ui", "", true},
		{"reserved /ui/ subtree is an error", "/ui/", "", true},
		{"reserved under /auth is an error", "/auth/login", "", true},
		{"reserved Connect path is an error", "/engram.v1.EngramService/", "", true},
		{"non-reserved lookalike is allowed", "/uimcp", "/uimcp", false},
		// A trailing slash makes ServeMux register a subtree pattern that
		// 301-redirects POST /mcp -> /mcp/, breaking MCP clients — reject it.
		// (The bare root "/" is the exception: it's the catch-all escape hatch.)
		{"trailing slash is an error", "/mcp/", "", true},
		{"nested trailing slash is an error", "/a/b/", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveMCPPath(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Fatalf("resolveMCPPath(%q)=%q want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestMountMCPRoutes(t *testing.T) {
	const stubHdr = "X-Mcp-Stub"
	mcpStub := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(stubHdr, "hit")
		w.WriteHeader(http.StatusOK)
	})

	cases := []struct {
		name         string
		uiEnabled    bool
		mcpPath      string
		method       string
		path         string
		wantCode     int
		wantStub     bool
		wantLocation string
	}{
		// UI enabled, default /mcp: console at /, MCP at /mcp.
		{"ui: GET / redirects to console", true, "/mcp", http.MethodGet, "/", http.StatusFound, false, "/ui/"},
		{"ui: GET /mcp reaches MCP", true, "/mcp", http.MethodGet, "/mcp", http.StatusOK, true, ""},
		{"ui: POST /mcp reaches MCP", true, "/mcp", http.MethodPost, "/mcp", http.StatusOK, true, ""},
		{"ui: GET /favicon.ico is 404", true, "/mcp", http.MethodGet, "/favicon.ico", http.StatusNotFound, false, ""},
		{"ui: POST / (mis-targeted MCP) is 404", true, "/mcp", http.MethodPost, "/", http.StatusNotFound, false, ""},

		// Headless, default /mcp: MCP at /mcp, no console so / is 404.
		{"headless: GET /mcp reaches MCP", false, "/mcp", http.MethodGet, "/mcp", http.StatusOK, true, ""},
		{"headless: GET / is 404 (no console)", false, "/mcp", http.MethodGet, "/", http.StatusNotFound, false, ""},
		{"headless: POST / is 404", false, "/mcp", http.MethodPost, "/", http.StatusNotFound, false, ""},

		// Escape hatch ENGRAM_MCP_PATH=/: legacy root catch-all in either mode.
		{"escape hatch ui: POST / reaches MCP", true, "/", http.MethodPost, "/", http.StatusOK, true, ""},
		{"escape hatch headless: POST /anything reaches MCP", false, "/", http.MethodPost, "/anything", http.StatusOK, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mountMCPRoutes(mux, mcpStub, tc.uiEnabled, tc.mcpPath)

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))

			if rec.Code != tc.wantCode {
				t.Fatalf("%s %s: status=%d want %d", tc.method, tc.path, rec.Code, tc.wantCode)
			}
			if gotStub := rec.Header().Get(stubHdr) == "hit"; gotStub != tc.wantStub {
				t.Fatalf("%s %s: reachedMCP=%v want %v", tc.method, tc.path, gotStub, tc.wantStub)
			}
			if tc.wantLocation != "" {
				if loc := rec.Header().Get("Location"); loc != tc.wantLocation {
					t.Fatalf("%s %s: Location=%q want %q", tc.method, tc.path, loc, tc.wantLocation)
				}
			}
		})
	}
}
