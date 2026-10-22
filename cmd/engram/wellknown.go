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
	"time"
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

// splitAuthority splits a host[:port] authority into its host and port. A
// bracketed IPv6 host keeps its brackets. ok is false when a colon has no
// port after it, or when an unbracketed host holds a colon.
func splitAuthority(v string) (host, port string, ok bool) {
	if strings.HasPrefix(v, "[") {
		end := strings.Index(v, "]")
		if end < 0 {
			return "", "", false
		}
		host, rest := v[:end+1], v[end+1:]
		if rest == "" {
			return host, "", true
		}
		if len(rest) == 1 || rest[0] != ':' {
			return "", "", false
		}
		return host, rest[1:], true
	}
	host, port, found := strings.Cut(v, ":")
	if !found {
		return v, "", true
	}
	if port == "" || strings.Contains(port, ":") {
		return "", "", false
	}
	return host, port, true
}

// validAuthority reports whether v is a bare host[:port] authority and
// nothing more. The host is a DNS name whose dot-separated labels hold only
// letters, digits, hyphens and underscores, with at most one trailing dot
// (an IPv4 literal passes as a name of digit labels), or a bracketed IPv6
// address with no zone. The port, when present, is 1 to 65535 in digits
// with no leading zero.
func validAuthority(v string) bool {
	host, port, ok := splitAuthority(v)
	if !ok {
		return false
	}
	if port != "" {
		if port[0] == '0' || strings.ContainsFunc(port, func(c rune) bool { return c < '0' || c > '9' }) {
			return false
		}
		if n, err := strconv.Atoi(port); err != nil || n > 65535 {
			return false
		}
	}
	if strings.HasPrefix(host, "[") {
		addr, err := netip.ParseAddr(host[1 : len(host)-1])
		return err == nil && addr.Is6() && addr.Zone() == ""
	}
	for label := range strings.SplitSeq(strings.TrimSuffix(host, "."), ".") {
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
// lowercased, a trailing dot removed from a host name, a bracketed IPv6
// address in its canonical form with no zone, and the default port of the
// scheme (443 for https, 80 for http) removed, so the same origin written
// two ways compares equal. An authority splitAuthority cannot split is
// only lowercased.
func normalizeOrigin(scheme, authority string) string {
	scheme, authority = strings.ToLower(scheme), strings.ToLower(authority)
	host, port, ok := splitAuthority(authority)
	if !ok {
		return scheme + "://" + authority
	}
	if strings.HasPrefix(host, "[") {
		if addr, err := netip.ParseAddr(host[1 : len(host)-1]); err == nil {
			host = "[" + addr.WithZone("").String() + "]"
		}
	} else {
		host = strings.TrimSuffix(host, ".")
	}
	if scheme == "https" && port == "443" || scheme == "http" && port == "80" {
		port = ""
	}
	if port != "" {
		host += ":" + port
	}
	return scheme + "://" + host
}

// derivedOrigin returns the normalized origin (normalizeOrigin) a request was
// made to. The scheme is the X-Forwarded-Proto value only when it equals
// exactly "http" or "https" (any other value falls back to the TLS-derived
// scheme and is never echoed); the host is the X-Forwarded-Host value only
// when the request carries exactly one such header and validForwardedHost
// accepts it, else r.Host.
func derivedOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if xfp := r.Header.Get("X-Forwarded-Proto"); xfp == "http" || xfp == "https" {
		scheme = xfp
	}
	host := r.Host
	if xfh := r.Header.Values("X-Forwarded-Host"); len(xfh) == 1 && validForwardedHost(xfh[0]) {
		host = xfh[0]
	}
	return normalizeOrigin(scheme, host)
}

// ignoredForwardedHost returns the X-Forwarded-Host values of r that
// derivedOrigin did not use, joined and cut to maxWarnedOriginLen bytes, or
// "" when there are none.
func ignoredForwardedHost(r *http.Request) string {
	xfh := r.Header.Values("X-Forwarded-Host")
	if len(xfh) == 0 || len(xfh) == 1 && validForwardedHost(xfh[0]) {
		return ""
	}
	return truncate(strings.Join(xfh, ", "), maxWarnedOriginLen)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
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
// and returns it, trimmed of surrounding space, with its scheme://host
// origin. The host follows validAuthority, as an ENGRAM_MCP_RESOURCE_ORIGINS
// entry does. Empty is a valid no-op. The message names the env var, quotes
// the offending value and states the constraint, like
// ENGRAM_OPENAI_EMBEDDINGS_URL in internal/config/validate.go.
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
	if u.User != nil {
		return "", "", fmt.Errorf("ENGRAM_MCP_RESOURCE_URL %q: must not contain userinfo", v)
	}
	if u.Host == "" {
		return "", "", fmt.Errorf("ENGRAM_MCP_RESOURCE_URL %q: missing host", v)
	}
	if !validAuthority(u.Host) {
		return "", "", fmt.Errorf("ENGRAM_MCP_RESOURCE_URL %q: the host must be a DNS name or an IP address, with no wildcard, and any port must be 1 to 65535", v)
	}
	if u.RawQuery != "" || u.ForceQuery || strings.Contains(v, "#") {
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
// remembers in one warnWindow, and maxWarnedOriginLen caps the length of
// each, so a flood of spoofed hosts cannot grow memory without bound. The
// remembered set starts again empty after each warnWindow, so a flood cannot
// silence a later refusal for longer than that.
const (
	maxWarnedOrigins   = 256
	maxWarnedOriginLen = 512
	warnWindow         = time.Hour
)

// protectedResourceConfig is the configuration newProtectedResource builds a
// protectedResource from.
type protectedResourceConfig struct {
	ResourceURL string // ENGRAM_MCP_RESOURCE_URL
	Origins     string // ENGRAM_MCP_RESOURCE_ORIGINS
	MetadataURL string // ENGRAM_OIDC_RESOURCE_METADATA
	MCPPath     string // the resolved MCP path
	// OnRefused is called for every refused document request; nil calls
	// nothing.
	OnRefused func(context.Context)
}

// protectedResource decides what the RFC 9728 documents and the MCP 401
// challenge name for a request. A pinned resource URL names one origin for
// every request. Otherwise each request's derived origin is named, and a
// non-empty allowed set limits which origins the challenge and the
// path-suffix document may name. The host-only document, which `engram
// login` reads, answers every origin.
type protectedResource struct {
	mcpPath string
	// pinned is ENGRAM_MCP_RESOURCE_URL, trimmed of surrounding space. Empty
	// selects the derived mode.
	pinned string
	// challenge is the resource_metadata value of every 401 when it does not
	// depend on the request: ENGRAM_OIDC_RESOURCE_METADATA, else the document
	// on the pinned origin. Empty only when the value depends on the request:
	// the derived mode without ENGRAM_OIDC_RESOURCE_METADATA.
	challenge string
	// allowed bounds the derived mode. Empty allows every origin.
	allowed   originSet
	onRefused func(context.Context)
	// now is the clock for warnWindow; nil means time.Now.
	now func() time.Time

	mu          sync.Mutex
	warned      map[string]struct{}
	warnedSince time.Time
}

// newProtectedResource validates ENGRAM_MCP_RESOURCE_URL and
// ENGRAM_MCP_RESOURCE_ORIGINS, refuses both set, and returns the protected
// resource for cfg.MCPPath. ENGRAM_OIDC_RESOURCE_METADATA, trimmed of
// surrounding space, replaces the challenge value in every mode.
func newProtectedResource(cfg protectedResourceConfig) (*protectedResource, error) {
	pinned, pinnedOrigin, err := resolveMCPResourceURL(cfg.ResourceURL)
	if err != nil {
		return nil, err
	}
	allowed, err := resolveMCPResourceOrigins(cfg.Origins)
	if err != nil {
		return nil, err
	}
	if len(allowed) > 0 && pinned != "" {
		return nil, errors.New("ENGRAM_MCP_RESOURCE_ORIGINS and ENGRAM_MCP_RESOURCE_URL are both set: the origin list bounds the resource derived for each request, which a set ENGRAM_MCP_RESOURCE_URL replaces; set one of them")
	}
	p := &protectedResource{
		mcpPath:   cfg.MCPPath,
		pinned:    pinned,
		challenge: strings.TrimSpace(cfg.MetadataURL),
		allowed:   allowed,
		onRefused: cfg.OnRefused,
	}
	if p.challenge == "" && pinned != "" {
		p.challenge = resourceMetadataURLForOrigin(pinnedOrigin, cfg.MCPPath)
	}
	return p, nil
}

// mode returns the mode of p and, in the bounded derived mode, the number
// of allowed origins, as slog attributes.
func (p *protectedResource) mode() []any {
	switch {
	case p.pinned != "":
		return []any{"resource", "configured"}
	case len(p.allowed) > 0:
		return []any{"resource", "derived-per-request", "origins", len(p.allowed)}
	default:
		return []any{"resource", "derived-per-request"}
	}
}

// unbounded reports whether p names any origin a request gives: the
// derived mode with no allowed set.
func (p *protectedResource) unbounded() bool {
	return p.pinned == "" && len(p.allowed) == 0
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

// challengeURL returns the challenge's resource_metadata value for r, or ""
// for none: the fixed value when there is one, else the path-suffix
// document on r's origin when the allowed set accepts it.
func (p *protectedResource) challengeURL(r *http.Request) string {
	if p.challenge != "" {
		return p.challenge
	}
	origin, ok := p.origin(r)
	if !ok {
		return ""
	}
	return resourceMetadataURLForOrigin(origin, p.mcpPath)
}

// challengeVaries reports whether the challenge value depends on the
// request's forwarded headers.
func (p *protectedResource) challengeVaries() bool {
	return p.challenge == ""
}

// refused records a refused document request for r's origin: onRefused for
// every one, and one warning per distinct origin in each warnWindow, for at
// most maxWarnedOrigins origins and then one limit warning. A warning also
// names the X-Forwarded-Host values derivedOrigin ignored.
func (p *protectedResource) refused(r *http.Request, origin string) {
	ctx := r.Context()
	if p.onRefused != nil {
		p.onRefused(ctx)
	}
	origin = truncate(origin, maxWarnedOriginLen)
	now := time.Now
	if p.now != nil {
		now = p.now
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if t := now(); p.warned == nil || t.Sub(p.warnedSince) >= warnWindow {
		p.warned = map[string]struct{}{}
		p.warnedSince = t
	}
	if _, seen := p.warned[origin]; seen || len(p.warned) >= maxWarnedOrigins {
		return
	}
	p.warned[origin] = struct{}{}
	attrs := []any{"origin", origin}
	if ignored := ignoredForwardedHost(r); ignored != "" {
		attrs = append(attrs, "ignored_x_forwarded_host", ignored)
	}
	slog.WarnContext(ctx, "request origin is not in ENGRAM_MCP_RESOURCE_ORIGINS; serving no path-suffix protected-resource metadata for it", attrs...)
	if len(p.warned) == maxWarnedOrigins {
		slog.WarnContext(ctx, "refused-origin warning limit reached; further refused origins are not logged until the window ends", "limit", maxWarnedOrigins, "window", warnWindow)
	}
}

// protectedResourceHandler serves the RFC 9728 protected-resource metadata
// document for p. It is deliberately unauthenticated (RFC 9728 requires
// public reachability): no bearer gate, no session lookup, and no store or
// embedder call anywhere in this handler. A pinned resource URL is served
// as configured and no request header is consulted, so a spoofed
// X-Forwarded-Host or X-Forwarded-Proto cannot override it; the document is
// then static and carries no Cache-Control header. Otherwise the resource
// is derived from each request's origin. On the path-suffix path an origin
// the allowed set refuses gets a 404; the host-only path answers every
// origin, because `engram login` reads its authorization_servers through
// whatever name the CLI uses. Every derived response, the 404 included,
// carries Cache-Control: no-store and a Vary on the forwarded headers, so a
// shared cache can never keep a response derived from a spoofable header.
func protectedResourceHandler(p *protectedResource, issuer string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resource := p.pinned
		if resource == "" {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Vary", "X-Forwarded-Host, X-Forwarded-Proto")
			origin, ok := p.origin(r)
			if !ok && r.URL.Path != wellKnownPRMBase {
				p.refused(r, origin)
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
