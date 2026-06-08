// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// recordFunc records one tool call's metrics; matches telemetry.ToolMetrics.Record.
type recordFunc func(ctx context.Context, tool, outcome string, ms float64)

// instrumentTools returns an mcp.Middleware that wraps tool calls with a span,
// metrics, and a structured log line. Non-tool methods pass through with only a
// debug-level trace. The tool name comes from the *mcp.CallToolRequest params;
// outcome is "error" when the handler errors or the result IsError.
func instrumentTools(record recordFunc) mcp.Middleware {
	tracer := otel.Tracer("github.com/seanb4t/engram")
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}
			ctr, ok := req.(*mcp.CallToolRequest)
			if !ok || ctr.Params == nil {
				return next(ctx, method, req)
			}
			tool := ctr.Params.Name

			ctx, span := tracer.Start(ctx, "tool/"+tool, oteltrace.WithSpanKind(oteltrace.SpanKindServer))
			defer span.End() // panic-safe: the span always closes even if next() panics
			span.SetAttributes(attribute.String("engram.tool", tool))
			start := time.Now()

			res, err := next(ctx, method, req)

			outcome := classifyOutcome(res, err)
			ms := float64(time.Since(start).Microseconds()) / 1000.0
			record(ctx, tool, outcome, ms)

			actor, owner := identityForLog(ctx)
			lg := slog.With("tool", tool, "outcome", outcome, "dur_ms", ms, "actor", actor, "owner", owner)
			if err != nil {
				span.SetStatus(codes.Error, err.Error())
				span.RecordError(err)
				lg.ErrorContext(ctx, "tool call failed", "err", err)
			} else {
				lg.InfoContext(ctx, "tool call")
			}
			return res, err
		}
	}
}

func classifyOutcome(res mcp.Result, err error) string {
	if err != nil {
		return "error"
	}
	if ctr, ok := res.(*mcp.CallToolResult); ok && ctr.IsError {
		return "error"
	}
	return "ok"
}

// identityForLog extracts the verified actor (human-readable) and owner (sub)
// from context for log attribution. Both are "" when auth is disabled. The owner
// is read via subjectFromContext for DISPLAY ONLY — a nil/error subject degrades
// to "" rather than failing the log; this is never an enforcement decision.
func identityForLog(ctx context.Context) (actor, owner string) {
	actor = actorFromContext(ctx)
	if subj, err := subjectFromContext(ctx); err == nil && subj != nil {
		owner = subj.Owner()
	}
	return actor, owner
}
