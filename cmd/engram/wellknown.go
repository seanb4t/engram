// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// wellKnownPRMBase is the RFC 9728 host-only path for the OAuth protected-
// resource metadata document.
const wellKnownPRMBase = "/.well-known/oauth-protected-resource"

// bearerMethodHeader and offlineAccessScope are the fixed document values
// decided in D-01 (260909-ofg-PLAN.md): they reproduce, byte-for-byte, what
// the retired agentgateway-synthesised document advertised
// (argocd/app-configs/agentgateway/mcp-engram.yaml). NOT configurable --
// there is no second known consumer wanting a different value, and a
// constant can be widened into a config key later without breaking anyone,
// whereas a config key is a permanent ENGRAM_ surface that can never be
// withdrawn.
const (
	bearerMethodHeader = "header"
	offlineAccessScope = "offline_access"
)

// protectedResourceMetadata is the RFC 9728 protected-resource metadata
// document body. AuthorizationServers carries omitempty so an unconfigured
// issuer (auth disabled) omits the key entirely rather than emitting an
// empty array -- an auth-disabled deployment must not advertise a
// nonexistent authorization server (T-526-02).
type protectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers,omitempty"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
	ScopesSupported        []string `json:"scopes_supported"`
}

// protectedResourcePaths returns the RFC 9728 §3.1 path-suffix form (the
// host-only path -- wellKnownPRMBase -- concatenated with mcpPath) when
// mcpPath names a non-root MCP mount. It returns empty when mcpPath is the
// bare root: concatenating it would produce a trailing-slash ServeMux
// subtree pattern, and the host-only form already IS the correct document
// location for a root-mounted resource. Callers that also need the
// host-only path use the wellKnownPRMBase constant directly.
func protectedResourcePaths(mcpPath string) (suffix string) {
	if mcpPath == "" || mcpPath == "/" {
		return ""
	}
	return wellKnownPRMBase + mcpPath
}

// validForwardedHost reports whether v is safe to trust as an
// X-Forwarded-Host value: a bare authority (host, or host:port), nothing
// more. A comma means a multi-proxy append list whose leading element is
// the most client-controlled -- the whole value is rejected rather than
// trusting any element of it. Any of a slash, an at sign, or whitespace
// signals something other than a bare authority (a path, userinfo, or a
// smuggled value). Length is capped at 255 (the DNS name limit) to reject
// pathological input cheaply before parsing. The final round-trip check
// (parsing a protocol-relative form and confirming url.Parse reflects the
// same Host back unchanged) catches anything the cheap checks above missed.
// Callers MUST fall back to the trusted r.Host when this returns false --
// documented at the resourceURLFor call site.
func validForwardedHost(v string) bool {
	if v == "" || len(v) > 255 {
		return false
	}
	if strings.ContainsAny(v, ", \t\n\r/@") {
		return false
	}
	u, err := url.Parse("//" + v)
	if err != nil {
		return false
	}
	return u.Host == v
}

// derivedOrigin returns the scheme://host origin a request was made to. The
// scheme is the X-Forwarded-Proto value only when it equals exactly "http" or
// "https" (any other value falls back to the TLS-derived scheme and is never
// echoed); the host is the X-Forwarded-Host value only when validForwardedHost
// accepts it, else the trusted r.Host.
func derivedOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if xfp := r.Header.Get("X-Forwarded-Proto"); xfp == "http" || xfp == "https" {
		scheme = xfp
	}
	host := r.Host
	if xfh := r.Header.Get("X-Forwarded-Host"); validForwardedHost(xfh) {
		host = xfh
	}
	return scheme + "://" + host
}

// resourceForOrigin returns the RFC 9728 `resource` value on origin: origin
// plus mcpPath, or origin alone when mcpPath is the bare root.
func resourceForOrigin(origin, mcpPath string) string {
	if mcpPath == "" || mcpPath == "/" {
		return origin
	}
	return origin + mcpPath
}

// resourceMetadataURLForOrigin returns the URL of the protected-resource
// metadata document on origin: the path-suffix form for a non-root mcpPath,
// else the host-only form.
func resourceMetadataURLForOrigin(origin, mcpPath string) string {
	if suffix := protectedResourcePaths(mcpPath); suffix != "" {
		return origin + suffix
	}
	return origin + wellKnownPRMBase
}

// resourceURLFor computes the RFC 9728 `resource` field for one request.
// When configured is non-empty it is returned verbatim and NO request header
// is consulted, so a spoofed X-Forwarded-Host or X-Forwarded-Proto can never
// override a configured ENGRAM_MCP_RESOURCE_URL. Otherwise it is the
// resource on the request's derivedOrigin.
func resourceURLFor(configured, mcpPath string, r *http.Request) string {
	if configured != "" {
		return configured
	}
	return resourceForOrigin(derivedOrigin(r), mcpPath)
}

// resolveMCPResourceURL shape-checks the configured ENGRAM_MCP_RESOURCE_URL
// (D-04): validated here, at its single use site in runServe, rather than in
// Config.Validate, whose own doc comment scopes it to fields every command's
// store/embedder path consumes -- this field is serve-only. Empty is a
// valid no-op. Mirrors the ENGRAM_OPENAI_EMBEDDINGS_URL message idiom in
// internal/config/validate.go: name the env var, quote the offending value,
// state the constraint.
func resolveMCPResourceURL(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", nil
	}
	u, err := url.Parse(v)
	if err != nil {
		return "", fmt.Errorf("ENGRAM_MCP_RESOURCE_URL %q: must be a valid URL: %w", v, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("ENGRAM_MCP_RESOURCE_URL %q: scheme must be http or https", v)
	}
	if u.Host == "" {
		return "", fmt.Errorf("ENGRAM_MCP_RESOURCE_URL %q: missing host", v)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("ENGRAM_MCP_RESOURCE_URL %q: must not contain a query or fragment", v)
	}
	return v, nil
}

// protectedResourceHandler serves the RFC 9728 protected-resource metadata
// document. It is deliberately unauthenticated (RFC 9728 requires public
// reachability): no bearer gate, no session lookup, and no store or embedder
// call anywhere in this handler (T-526-04). When configuredResourceURL is
// empty (the resource is derived per request) the response carries
// Cache-Control: no-store, so a shared cache in front of engram can never
// retain a document whose resource was derived from a spoofable header
// (T-526-01); when configuredResourceURL is set, no Cache-Control header is
// emitted at all -- the document is static for the life of the process.
func protectedResourceHandler(configuredResourceURL, issuer, mcpPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if configuredResourceURL == "" {
			w.Header().Set("Cache-Control", "no-store")
		}
		doc := protectedResourceMetadata{
			Resource:               resourceURLFor(configuredResourceURL, mcpPath, r),
			BearerMethodsSupported: []string{bearerMethodHeader},
			ScopesSupported:        []string{offlineAccessScope},
		}
		if issuer != "" {
			doc.AuthorizationServers = []string{issuer}
		}
		_ = json.NewEncoder(w).Encode(doc)
	})
}

// resolveResourceMetadataURL returns the fixed resource_metadata value of the
// MCP 401 challenge, or empty when the challenge is derived per request. An
// explicitly configured value (ENGRAM_OIDC_RESOURCE_METADATA) always wins and
// is returned untouched, including when mcpResourceURL is also set. Otherwise
// a set mcpResourceURL yields the document on its origin (scheme and host
// only: any path, query or trailing slash it carries is discarded, so the
// result never doubles a path segment). Both empty yields empty: withAuth
// then names the document on each request's own origin (derivedChallengeURL).
//
// A parse failure of mcpResourceURL yields empty rather than a malformed
// challenge URL; resolveMCPResourceURL has already rejected malformed input
// at startup, so this is not a live path.
func resolveResourceMetadataURL(configured, mcpResourceURL, mcpPath string) string {
	if configured != "" {
		return configured
	}
	if mcpResourceURL == "" {
		return ""
	}
	u, err := url.Parse(mcpResourceURL)
	if err != nil {
		return ""
	}
	return resourceMetadataURLForOrigin(u.Scheme+"://"+u.Host, mcpPath)
}

// derivedChallengeURL returns the per-request resource_metadata value of the
// MCP 401 challenge in the derived mode: the document on the same origin the
// document handler derives for that request.
func derivedChallengeURL(mcpPath string) func(*http.Request) string {
	return func(r *http.Request) string {
		return resourceMetadataURLForOrigin(derivedOrigin(r), mcpPath)
	}
}

// mountWellKnownRoutes registers h under the RFC 9728 host-only path and,
// when mcpPath is not the bare root, the §3.1 path-suffix path too. Both
// patterns are method-scoped ("GET <path>") rather than a bare path
// pattern: a bare pattern would steal POST from the ENGRAM_MCP_PATH=/
// legacy root catch-all, whereas the GET-scoped form lets a POST fall
// through to it untouched (verified behaviour -- see the probe note in
// 260909-ofg-PLAN.md's verified_orientation).
func mountWellKnownRoutes(mux *http.ServeMux, h http.Handler, mcpPath string) {
	mux.Handle("GET "+wellKnownPRMBase, h)
	if suffix := protectedResourcePaths(mcpPath); suffix != "" {
		mux.Handle("GET "+suffix, h)
	}
}
