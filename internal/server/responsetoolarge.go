// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file hosts phase 2's response-too-large mapping (02-CONTEXT.md D-04,
// D-06, D-08, D-09): the one shared wire envelope for
// store.ErrResponseTooLarge on both server lanes. Connect's connectError arm
// (connecterror.go) and the MCP receiving middleware in this file
// (mapResponseTooLarge, wired via instrument.go's addToolMiddleware) both
// render the SAME envelope through argerror.go's renderHintEnvelope — never
// a second hand-built string (D-04). The raw upstream error (RPC method,
// byte counts, grpc-go text) is logged server-side exactly once per lane
// (D-03, D-06, D-08) and never reaches either wire. Every OTHER server or
// tool error is untouched by this file (D-09) — that lane inconsistency
// (Connect scrubs unclassified errors to "internal error"; MCP still
// returns raw text for everything else) is a recorded deferred item, not
// fixed here.

package server

import (
	"context"
	"errors"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/seanb4t/engram/internal/store"
)

// responseTooLargeDetail is the published detail text for
// HintResponseTooLarge. It names the remedies generically (a smaller
// limit/k, or omitting full) and
// contains no byte ceiling and no upstream grpc/Qdrant text (D-04) — a
// caller can act on it with no additional context, and it never steers the
// caller toward a retry that cannot work (no "later"/"again" wording, since
// the ceiling this response hit does not change between requests).
const responseTooLargeDetail = "the result is too large to return in one response; retry with a smaller limit or k, or omit full"

// responseTooLargeEnvelope renders the one shared wire envelope for
// store.ErrResponseTooLarge, via argerror.go's renderHintEnvelope (D-04).
// "response" is a fixed pseudo-field — the RESPONSE overflowed, not a
// caller-supplied input — which keeps both mappers independent of every
// tool's own argument shape (precedent: internal/store/revert.go's
// field=steps).
func responseTooLargeEnvelope() string {
	return renderHintEnvelope([]string{"response"}, HintResponseTooLarge, responseTooLargeDetail)
}

// mapResponseTooLarge returns an mcp.Middleware that rewrites a "tools/call"
// result's content to the shared envelope when the handler's original error
// (recovered via go-sdk v1.8.0's CallToolResult.GetError(), since a typed
// AddTool handler's error never surfaces as the MethodHandler-level Go error
// — RESEARCH.md Pitfall 3) satisfies errors.Is(err, store.ErrResponseTooLarge).
// IsError stays true; every other method, non-*CallToolResult result, Go
// error from next, or unmapped tool error is returned completely unchanged
// (D-09). The raw error is logged here exactly once, at ERROR level — this
// is the FIRST server-side log line this error class has ever gotten on the
// MCP lane, since instrumentTools' own error-log branch never fires for an
// ordinary typed-handler business error (RESEARCH.md Pitfall 3).
func mapResponseTooLarge() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			res, err := next(ctx, method, req)
			if method != "tools/call" {
				return res, err
			}
			ctr, ok := res.(*mcp.CallToolResult)
			if !ok || err != nil {
				return res, err
			}
			toolErr := ctr.GetError()
			if !errors.Is(toolErr, store.ErrResponseTooLarge) {
				return res, err
			}
			tool := ""
			if ctr2, ok := req.(*mcp.CallToolRequest); ok && ctr2.Params != nil {
				tool = ctr2.Params.Name
			}
			slog.ErrorContext(ctx, "mcp tool call: response too large", "tool", tool, "error", toolErr)
			ctr.Content = []mcp.Content{&mcp.TextContent{Text: responseTooLargeEnvelope()}}
			ctr.IsError = true
			return ctr, err
		}
	}
}
