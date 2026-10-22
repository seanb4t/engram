// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
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

// validAuthority reports whether v is a bare host[:port] authority and
// nothing more. The host is a dot-separated DNS name whose labels hold only
// letters, digits, hyphens and underscores, or a bracketed IPv6 address. The
// port, when present, is 1 to 65535 in digits with no leading zero.
func validAuthority(v string) bool {
	host := v
	if i := strings.LastIndex(v, ":"); i >= 0 && !strings.HasSuffix(v, "]") {
		host = v[:i]
		port := v[i+1:]
		if port == "" || port[0] == '0' || strings.ContainsFunc(port, func(c rune) bool { return c < '0' || c > '9' }) {
			return false
		}
		if n, err := strconv.Atoi(port); err != nil || n > 65535 {
			return false
		}
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		addr, err := netip.ParseAddr(host[1 : len(host)-1])
		return err == nil && addr.Is6()
	}
	for label := range strings.SplitSeq(host, ".") {
		if label == "" || strings.ContainsFunc(label, notHostNameByte) {
			return false
		}
	}
	return true
}

func notHostNameByte(c rune) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		return false
	}
	return true
}

// validForwardedHost reports whether v is safe to trust as an
// X-Forwarded-Host value: a bare authority (validAuthority) of at most 255
// bytes. A comma-separated multi-proxy list is rejected whole, because its
// leading element is the most client-controlled. Callers MUST fall back to
// r.Host when this returns false -- see derivedOrigin.
func validForwardedHost(v string) bool {
	return v != "" && len(v) <= 255 && validAuthority(v)
}

// normalizeOrigin returns scheme://authority with the scheme and host
// lowercased and the default port of the scheme (443 for https, 80 for
// http) removed, so the same origin written two ways compares equal.
func normalizeOrigin(scheme, authority string) string {
	scheme, authority = strings.ToLower(scheme), strings.ToLower(authority)
	switch {
	case scheme == "https" && strings.HasSuffix(authority, ":443"):
		authority = strings.TrimSuffix(authority, ":443")
	case scheme == "http" && strings.HasSuffix(authority, ":80"):
		authority = strings.TrimSuffix(authority, ":80")
	}
	return scheme + "://" + authority
}

// derivedOrigin returns the normalized origin (normalizeOrigin) a request was
// made to. The scheme is the X-Forwarded-Proto value only when it equals
// exactly "http" or "https" (any other value falls back to the TLS-derived
// scheme and is never echoed); the host is the X-Forwarded-Host value only
// when validForwardedHost accepts it, else r.Host.
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
	return normalizeOrigin(scheme, host)
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
// and returns it verbatim with its scheme://host origin. Empty is a valid
// no-op. The message names the env var, quotes the offending value and
// states the constraint, like ENGRAM_OPENAI_EMBEDDINGS_URL in
// internal/config/validate.go.
func resolveMCPResourceURL(raw string) (resource, origin string, err error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", "", nil
	}
	u, err := url.Parse(v)
	if err != nil {
		return "", "", fmt.Errorf("ENGRAM_MCP_RESOURCE_URL %q: must be a valid URL: %w", v, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", "", fmt.Errorf("ENGRAM_MCP_RESOURCE_URL %q: scheme must be http or https", v)
	}
	if u.Host == "" {
		return "", "", fmt.Errorf("ENGRAM_MCP_RESOURCE_URL %q: missing host", v)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", "", fmt.Errorf("ENGRAM_MCP_RESOURCE_URL %q: must not contain a query or fragment", v)
	}
	return v, u.Scheme + "://" + u.Host, nil
}

// originSet is the ENGRAM_MCP_RESOURCE_ORIGINS list, each entry normalized
// with normalizeOrigin.
type originSet map[string]struct{}

// resolveMCPResourceOrigins parses ENGRAM_MCP_RESOURCE_ORIGINS, a
// comma-separated list of http or https origins. An entry carries a scheme
// and a host (validAuthority, with an optional port) and nothing else. Empty
// is a valid no-op that returns a nil set. Like resolveMCPResourceURL, each
// message names the variable and quotes the offending value.
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
		if !validAuthority(u.Host) {
			return nil, fmt.Errorf("ENGRAM_MCP_RESOURCE_ORIGINS entry %q: the host must be a DNS name or an IP address, with no wildcard, and any port must be 1 to 65535", v)
		}
		origin := normalizeOrigin(u.Scheme, u.Host)
		if _, dup := set[origin]; dup {
			return nil, fmt.Errorf("ENGRAM_MCP_RESOURCE_ORIGINS entry %q: listed more than once", v)
		}
		set[origin] = struct{}{}
	}
	return set, nil
}

// maxWarnedOrigins caps how many distinct refused origins a protectedResource
// remembers, and maxWarnedOriginLen caps the length of each, so a flood of
// spoofed hosts cannot grow memory without bound.
const (
	maxWarnedOrigins   = 256
	maxWarnedOriginLen = 512
)

// protectedResource decides what the RFC 9728 document and the MCP 401
// challenge name for a request. A pinned resource URL names one origin for
// every request. Otherwise each request's derived origin is named, and a
// non-empty allowed set limits which origins may be named.
type protectedResource struct {
	mcpPath string
	// pinned is ENGRAM_MCP_RESOURCE_URL verbatim. Empty selects the derived
	// mode.
	pinned string
	// challenge is the resource_metadata value of every 401 when it does not
	// depend on the request: ENGRAM_OIDC_RESOURCE_METADATA, else the document
	// on the pinned origin. Empty in the derived mode.
	challenge string
	// allowed bounds the derived mode. Empty allows every origin.
	allowed originSet
	// onRefused, when set, is called for every refused document request.
	onRefused func(context.Context)

	mu     sync.Mutex
	warned map[string]struct{}
}

// newProtectedResource validates ENGRAM_MCP_RESOURCE_URL (resourceURL) and
// ENGRAM_MCP_RESOURCE_ORIGINS (origins), refuses both set, and returns the
// protected resource for mcpPath. metadataURL is ENGRAM_OIDC_RESOURCE_METADATA,
// which replaces the challenge value in every mode.
func newProtectedResource(resourceURL, origins, metadataURL, mcpPath string) (*protectedResource, error) {
	pinned, pinnedOrigin, err := resolveMCPResourceURL(resourceURL)
	if err != nil {
		return nil, err
	}
	allowed, err := resolveMCPResourceOrigins(origins)
	if err != nil {
		return nil, err
	}
	if len(allowed) > 0 && pinned != "" {
		return nil, errors.New("ENGRAM_MCP_RESOURCE_ORIGINS and ENGRAM_MCP_RESOURCE_URL are both set: the origin list bounds the resource derived for each request, which a set ENGRAM_MCP_RESOURCE_URL replaces; set one of them")
	}
	p := &protectedResource{mcpPath: mcpPath, pinned: pinned, challenge: metadataURL, allowed: allowed}
	if p.challenge == "" && pinned != "" {
		p.challenge = resourceMetadataURLForOrigin(pinnedOrigin, mcpPath)
	}
	return p, nil
}

// origin returns the derived origin of r, and false when the allowed set
// refuses it. It has no side effects.
func (p *protectedResource) origin(r *http.Request) (string, bool) {
	origin := derivedOrigin(r)
	if len(p.allowed) == 0 {
		return origin, true
	}
	_, ok := p.allowed[origin]
	return origin, ok
}

// fixedChallenge returns the challenge's resource_metadata value and true
// when it is the same for every request. A nil p has none ("", true).
func (p *protectedResource) fixedChallenge() (string, bool) {
	if p == nil {
		return "", true
	}
	return p.challenge, p.challenge != ""
}

// challengeURL returns the challenge's resource_metadata value for r, or ""
// for none: the fixed value when there is one, else the document on r's
// origin when the allowed set accepts it.
func (p *protectedResource) challengeURL(r *http.Request) string {
	if v, fixed := p.fixedChallenge(); fixed {
		return v
	}
	origin, ok := p.origin(r)
	if !ok {
		return ""
	}
	return resourceMetadataURLForOrigin(origin, p.mcpPath)
}

// refused records a refused document request: onRefused for every one, and
// one warning per distinct origin, for at most maxWarnedOrigins origins and
// then one limit warning.
func (p *protectedResource) refused(ctx context.Context, origin string) {
	if p.onRefused != nil {
		p.onRefused(ctx)
	}
	if len(origin) > maxWarnedOriginLen {
		origin = origin[:maxWarnedOriginLen]
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.warned == nil {
		p.warned = map[string]struct{}{}
	}
	if _, seen := p.warned[origin]; seen || len(p.warned) >= maxWarnedOrigins {
		return
	}
	p.warned[origin] = struct{}{}
	slog.WarnContext(ctx, "request origin is not in ENGRAM_MCP_RESOURCE_ORIGINS; serving no protected-resource metadata for it", "origin", origin)
	if len(p.warned) == maxWarnedOrigins {
		slog.WarnContext(ctx, "refused-origin warning limit reached; further refused origins are not logged", "limit", maxWarnedOrigins)
	}
}

// protectedResourceHandler serves the RFC 9728 protected-resource metadata
// document for p. It is deliberately unauthenticated (RFC 9728 requires
// public reachability): no bearer gate, no session lookup, and no store or
// embedder call anywhere in this handler. A pinned resource URL is served
// verbatim and no request header is consulted, so a spoofed X-Forwarded-Host
// or X-Forwarded-Proto cannot override it; the document is then static and
// carries no Cache-Control header. Otherwise the resource is derived from
// each request's origin, and an origin the allowed set refuses gets a 404.
// Every derived response, the 404 included, carries Cache-Control: no-store,
// so a shared cache can never keep a response derived from a spoofable
// header.
func protectedResourceHandler(p *protectedResource, issuer string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resource := p.pinned
		if resource == "" {
			w.Header().Set("Cache-Control", "no-store")
			origin, ok := p.origin(r)
			if !ok {
				p.refused(r.Context(), origin)
				http.NotFound(w, r)
				return
			}
			resource = resourceForOrigin(origin, p.mcpPath)
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
