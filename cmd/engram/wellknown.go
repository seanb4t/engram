// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
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
// see derivedOrigin.
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

// originSet is the normalized ENGRAM_MCP_RESOURCE_ORIGINS list of
// scheme://host[:port] origins, with scheme and host lowercased.
type originSet map[string]struct{}

// resolveMCPResourceOrigins parses ENGRAM_MCP_RESOURCE_ORIGINS, a
// comma-separated list of http or https origins. An entry carries a scheme
// and a host (with an optional port) and nothing else. Empty is a valid
// no-op that returns a nil set. Like resolveMCPResourceURL it runs at its use
// site in runServe and names the variable and quotes the offending value.
func resolveMCPResourceOrigins(raw string) (originSet, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	set := originSet{}
	for entry := range strings.SplitSeq(raw, ",") {
		v := strings.TrimSpace(entry)
		if v == "" {
			return nil, fmt.Errorf("ENGRAM_MCP_RESOURCE_ORIGINS %q: the list has an empty entry", raw)
		}
		u, err := url.Parse(v)
		if err != nil {
			return nil, fmt.Errorf("ENGRAM_MCP_RESOURCE_ORIGINS entry %q: must be a valid URL: %w", v, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, fmt.Errorf("ENGRAM_MCP_RESOURCE_ORIGINS entry %q: scheme must be http or https", v)
		}
		if u.User != nil {
			return nil, fmt.Errorf("ENGRAM_MCP_RESOURCE_ORIGINS entry %q: must not contain userinfo", v)
		}
		if u.Host == "" {
			return nil, fmt.Errorf("ENGRAM_MCP_RESOURCE_ORIGINS entry %q: missing host", v)
		}
		if u.Path != "" || u.RawQuery != "" || u.ForceQuery || strings.Contains(v, "#") {
			return nil, fmt.Errorf("ENGRAM_MCP_RESOURCE_ORIGINS entry %q: an entry is a scheme and host only, with no path, query or fragment", v)
		}
		origin := u.Scheme + "://" + strings.ToLower(u.Host)
		if _, dup := set[origin]; dup {
			return nil, fmt.Errorf("ENGRAM_MCP_RESOURCE_ORIGINS entry %q: listed more than once", v)
		}
		set[origin] = struct{}{}
	}
	return set, nil
}

// maxWarnedOrigins caps how many distinct rejected origins an originGate
// remembers, and maxWarnedOriginLen caps the length of each, so a flood of
// spoofed hosts cannot grow memory without bound.
const (
	maxWarnedOrigins   = 256
	maxWarnedOriginLen = 512
)

// originGate decides whether the derived mode may name a request's origin.
// A nil gate, or one with an empty allowed set, allows every origin. It logs
// one warning per distinct rejected origin, for at most maxWarnedOrigins
// origins per process.
type originGate struct {
	allowed originSet

	mu     sync.Mutex
	warned map[string]struct{}
}

func newOriginGate(allowed originSet) *originGate {
	return &originGate{allowed: allowed, warned: map[string]struct{}{}}
}

// originFor returns the origin the derived mode names for r, and false when
// the gate rejects it. A listed origin comes back normalized.
func (g *originGate) originFor(r *http.Request) (string, bool) {
	origin := derivedOrigin(r)
	if g == nil || len(g.allowed) == 0 {
		return origin, true
	}
	normalized := strings.ToLower(origin)
	if _, ok := g.allowed[normalized]; ok {
		return normalized, true
	}
	g.warnRejected(normalized)
	return "", false
}

func (g *originGate) warnRejected(origin string) {
	if len(origin) > maxWarnedOriginLen {
		origin = origin[:maxWarnedOriginLen]
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, seen := g.warned[origin]; seen || len(g.warned) >= maxWarnedOrigins {
		return
	}
	g.warned[origin] = struct{}{}
	slog.Warn("request origin is not in ENGRAM_MCP_RESOURCE_ORIGINS; serving no protected-resource metadata for it", "origin", origin)
	if len(g.warned) == maxWarnedOrigins {
		slog.Warn("rejected-origin warning limit reached; further rejected origins are not logged", "limit", maxWarnedOrigins)
	}
}

// protectedResourceHandler serves the RFC 9728 protected-resource metadata
// document. It is deliberately unauthenticated (RFC 9728 requires public
// reachability): no bearer gate, no session lookup, and no store or embedder
// call anywhere in this handler (T-526-04). A configured resource URL is
// served verbatim and no request header is consulted, so a spoofed
// X-Forwarded-Host or X-Forwarded-Proto cannot override it; the document is
// then static and carries no Cache-Control header. Otherwise the resource is
// derived per request on the origin gate allows, and a rejected origin gets a
// 404. Every derived response, the 404 included, carries Cache-Control:
// no-store, so a shared cache can never retain a response derived from a
// spoofable header (T-526-01).
func protectedResourceHandler(configuredResourceURL, issuer, mcpPath string, gate *originGate) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resource := configuredResourceURL
		if resource == "" {
			w.Header().Set("Cache-Control", "no-store")
			origin, ok := gate.originFor(r)
			if !ok {
				http.NotFound(w, r)
				return
			}
			resource = resourceForOrigin(origin, mcpPath)
		}
		w.Header().Set("Content-Type", "application/json")
		doc := protectedResourceMetadata{
			Resource:               resource,
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
// document handler derives for that request, or "" when gate rejects it.
func derivedChallengeURL(mcpPath string, gate *originGate) func(*http.Request) string {
	return func(r *http.Request) string {
		origin, ok := gate.originFor(r)
		if !ok {
			return ""
		}
		return resourceMetadataURLForOrigin(origin, mcpPath)
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
