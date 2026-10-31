// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"log/slog"
	"net/http"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

// protocolVersion20260728 is the first MCP protocol version that go-sdk
// serves only on a stateless handler (SEP-2575). go-sdk keeps its own
// constant unexported and compares versions as strings, as protocolRouter
// does.
const protocolVersion20260728 = "2026-07-28"

// maxWarnedVersions caps how many distinct unserved protocol versions a
// protocolRouter remembers in one warnWindow, and maxWarnedVersionLen caps
// the length of each.
const (
	maxWarnedVersions   = 64
	maxWarnedVersionLen = 64
)

// Generation names in the unserved-version warning.
const (
	generationLegacy   = "legacy"
	generation20260728 = protocolVersion20260728
)

// protocolRouter sends each MCP request to one protocol generation by its
// Mcp-Protocol-Version header. A version of 2026-07-28 or later goes to
// sessionless, and every other request, a request with no header included,
// goes to sessions. It sets the version on the request's span, and warns
// once per window about a version that go-sdk does not serve.
type protocolRouter struct {
	sessions, sessionless http.Handler
	served                []string
	warner                distinctWarner
}

// newProtocolRouter returns the router over the two handlers of one MCP
// server.
func newProtocolRouter(sessions, sessionless http.Handler) *protocolRouter {
	return &protocolRouter{
		sessions:    sessions,
		sessionless: sessionless,
		served:      mcp.SupportedProtocolVersions(),
		warner:      distinctWarner{limit: maxWarnedVersions},
	}
}

func (p *protocolRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	version := r.Header.Get("Mcp-Protocol-Version")
	next, generation := p.sessions, generationLegacy
	if version >= protocolVersion20260728 {
		next, generation = p.sessionless, generation20260728
	}
	if version != "" {
		trace.SpanFromContext(r.Context()).SetAttributes(semconv.McpProtocolVersion(version))
		if !slices.Contains(p.served, version) {
			p.unserved(r, version, generation)
		}
	}
	next.ServeHTTP(w, r)
}

// unserved logs a warning for a request that names version, which go-sdk
// does not serve, once per distinct version in each warnWindow.
func (p *protocolRouter) unserved(r *http.Request, version, generation string) {
	version = truncate(version, maxWarnedVersionLen)
	warn, atLimit := p.warner.admit(version)
	if !warn {
		return
	}
	ctx := r.Context()
	slog.WarnContext(ctx, "MCP request names a protocol version this server does not serve",
		"mcp_protocol_version", version, "generation", generation, "supported", p.served, "ua", r.UserAgent())
	if atLimit {
		slog.WarnContext(ctx, "unserved protocol version warning limit reached; further versions are not logged until the window ends", "limit", maxWarnedVersions, "window", warnWindow)
	}
}
