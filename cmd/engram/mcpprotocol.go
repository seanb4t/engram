// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

// protocolVersion20260728 is the first MCP protocol version that go-sdk
// serves only on a stateless handler. go-sdk keeps its own constant
// unexported and compares versions as strings, as protocolRouter does.
const protocolVersion20260728 = "2026-07-28"

// servedProtocolVersions returns the MCP protocol versions that engram serve
// serves, newest first. newMCPServer serves exactly these, and
// protocolRouter warns about every other version. go-sdk panics at
// mcp.NewServer if it does not support one of them.
func servedProtocolVersions() []string {
	return []string{protocolVersion20260728, "2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}
}

// maxWarnedVersions caps how many distinct protocol versions a
// protocolRouter warns about in one warnWindow. maxProtocolVersionLen caps
// the length of a client-supplied protocol version in a log line or a span,
// and maxWarnedUserAgentLen the length of the user agent in a warning.
const (
	maxWarnedVersions     = 64
	maxProtocolVersionLen = 64
	maxWarnedUserAgentLen = 256
)

// Generation names in the protocol warnings.
const (
	generationLegacy   = "legacy"
	generation20260728 = protocolVersion20260728
)

// protocolRouter sends each MCP request to one protocol generation. A
// request whose Mcp-Protocol-Version header is 2026-07-28 or later goes to
// sessionless. A session-less POST with no header whose JSON-RPC _meta names
// a protocol version also goes to sessionless, which answers that the header
// is required. Every other request goes to sessions. The router sets the
// header version on the request's span, and warns about a version outside
// served and about a missing header.
type protocolRouter struct {
	sessions, sessionless http.Handler
	served                []string
	warner                *distinctWarner
}

// newProtocolRouter returns the router over the two handlers of one MCP
// server that serves the protocol versions in served.
func newProtocolRouter(sessions, sessionless http.Handler, served []string) *protocolRouter {
	return &protocolRouter{
		sessions:    sessions,
		sessionless: sessionless,
		served:      served,
		warner:      newDistinctWarner(maxWarnedVersions, maxProtocolVersionLen),
	}
}

func (p *protocolRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	version := truncate(r.Header.Get("Mcp-Protocol-Version"), maxProtocolVersionLen)
	next, generation := p.sessions, generationLegacy
	switch {
	case version >= protocolVersion20260728:
		next, generation = p.sessionless, generation20260728
	case version == "":
		if meta := metaProtocolVersion(r); meta != "" {
			next = p.sessionless
			p.warn(r, "_meta "+meta, "MCP request names a protocol version in its _meta but sends no Mcp-Protocol-Version header",
				"meta_protocol_version", truncate(meta, maxProtocolVersionLen))
		}
	}
	if version != "" {
		trace.SpanFromContext(r.Context()).SetAttributes(semconv.McpProtocolVersion(version))
		if !slices.Contains(p.served, version) {
			p.warn(r, version, "MCP request names a protocol version this server does not serve",
				"mcp_protocol_version", version, "generation", generation, "supported", p.served)
		}
	}
	next.ServeHTTP(w, r)
}

// warn logs msg with attrs and the request's user agent once per distinct
// key in each warnWindow, for at most maxWarnedVersions keys, and then one
// limit warning.
func (p *protocolRouter) warn(r *http.Request, key, msg string, attrs ...any) {
	_, warn, atLimit := p.warner.admit(key)
	if !warn {
		return
	}
	ctx := r.Context()
	slog.WarnContext(ctx, msg, append(attrs, "ua", truncate(r.UserAgent(), maxWarnedUserAgentLen))...)
	if atLimit {
		slog.WarnContext(ctx, "MCP protocol warning limit reached; further protocol warnings are not logged until the window ends", "limit", p.warner.limit, "window", warnWindow)
	}
}

// metaProtocolVersion returns the protocol version that the JSON-RPC _meta of
// a POST with no Mcp-Session-Id names, or "". It reads at most
// mcp.DefaultMaxRequestBodyBytes of the body, and the next handler reads the
// same bytes.
func metaProtocolVersion(r *http.Request) string {
	if r.Method != http.MethodPost || r.Body == nil || r.Header.Get("Mcp-Session-Id") != "" {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, mcp.DefaultMaxRequestBodyBytes))
	r.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(body), r.Body), r.Body}
	if err != nil {
		return ""
	}
	var msg struct {
		Params struct {
			Meta map[string]any `json:"_meta"`
		} `json:"params"`
	}
	if json.Unmarshal(body, &msg) != nil {
		return ""
	}
	version, _ := msg.Params.Meta[mcp.MetaKeyProtocolVersion].(string)
	return version
}
