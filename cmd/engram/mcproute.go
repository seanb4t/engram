// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"net/http"
	"strings"
)

// defaultMCPPath is where the MCP StreamableHTTP transport is mounted unless the
// operator overrides it via MEM_MCP_PATH / --mcp-path.
const defaultMCPPath = "/mcp"

// reservedMCPRoots are the namespaces already owned by other mux mounts (the
// console, the cookie/OIDC auth routes, and the Connect API). Mounting MCP under
// any of them would panic http.ServeMux on a conflicting registration at
// startup, so resolveMCPPath rejects them up front with a clear error. Keep in
// sync with the explicit mounts in runServe.
var reservedMCPRoots = []string{"/ui", "/auth", "/engram.v1.EngramService"}

// resolveMCPPath normalizes the configured MCP transport path. An empty value
// defaults to /mcp; the path MUST be absolute (leading "/") and MUST NOT fall
// under a reserved root (see reservedMCPRoots). The special value "/" is the
// escape hatch that restores the legacy root catch-all (MCP answers every path),
// which is how an operator opts a pre-0.7 client back in.
func resolveMCPPath(raw string) (string, error) {
	p := strings.TrimSpace(raw)
	if p == "" {
		return defaultMCPPath, nil
	}
	if !strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("MEM_MCP_PATH must be an absolute path starting with %q, got %q", "/", p)
	}
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		// A trailing slash makes http.ServeMux register a subtree pattern that
		// 301-redirects POST <path> -> <path>/, which MCP clients do not follow.
		// The bare root "/" is exempt — it is the catch-all escape hatch.
		return "", fmt.Errorf("MEM_MCP_PATH %q must not end with a trailing slash (it would 301-redirect and break MCP POST)", p)
	}
	for _, root := range reservedMCPRoots {
		if p == root || strings.HasPrefix(p, root+"/") {
			return "", fmt.Errorf("MEM_MCP_PATH %q collides with the reserved %q routes; choose another path", p, root)
		}
	}
	return p, nil
}

// mountMCPRoutes wires the MCP transport and the root "/" handler onto mux. When
// mcpPath is "/" the transport is the root catch-all in either UI mode (the
// MEM_MCP_PATH=/ escape hatch for legacy clients); otherwise it is mounted at the
// exact mcpPath and "/" is given to rootHandler, which lands a browser on the
// console when the UI is enabled and 404s everything else rather than the bearer
// gate. The MCP go-sdk handler dispatches on method + session header and ignores
// the request path, so an exact (non-subtree) mount is correct and avoids the
// ServeMux trailing-slash 301 that would break MCP POSTs.
func mountMCPRoutes(mux *http.ServeMux, mcpHandler http.Handler, uiEnabled bool, mcpPath string) {
	if mcpPath == "/" {
		mux.Handle("/", mcpHandler)
		return
	}
	mux.Handle(mcpPath, mcpHandler)
	mux.Handle("/", rootHandler(uiEnabled))
}

// rootHandler owns "/" when MCP lives at an explicit path. A browser GET of the
// bare root redirects to the console when the UI is enabled; every other request
// (non-GET, sub-paths, or any request when headless) is a 404 — deliberately not
// the MCP bearer gate, so stray paths and mis-targeted MCP clients get a clear
// signal instead of a noisy "no bearer token" 401.
func rootHandler(uiEnabled bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if uiEnabled && r.Method == http.MethodGet && r.URL.Path == "/" {
			http.Redirect(w, r, "/ui/", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})
}
