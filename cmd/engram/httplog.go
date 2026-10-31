// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.NewResponseController reach the underlying writer's
// Flush/deadline methods — required for the MCP streamable-HTTP SSE path.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// authFailureFunc counts a rejected request by reason; matches
// telemetry.ToolMetrics.RecordAuthFailure.
type authFailureFunc func(ctx context.Context, reason string)

// accessLog emits one structured log line per request and counts 401/403
// responses as auth failures. The line's "local" field is the local address
// of the connection; its port tells the primary listener from the MCP-only
// one. The line carries "mcp_protocol_version", cut to maxProtocolVersionLen
// bytes, when the request names one.
// observe is a test seam (nil in production).
func accessLog(authFail authFailureFunc, observe func(int)) func(http.Handler) http.Handler {
	if authFail == nil {
		authFail = func(context.Context, string) {}
	}
	if observe == nil {
		observe = func(int) {}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			next.ServeHTTP(sw, r)
			ms := float64(time.Since(start).Microseconds()) / 1000.0

			switch sw.status {
			case http.StatusUnauthorized:
				authFail(r.Context(), "unauthorized")
			case http.StatusForbidden:
				authFail(r.Context(), "forbidden")
			}
			observe(sw.status)

			var local string
			if addr, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
				local = addr.String()
			}
			attrs := []any{"method", r.Method, "path", r.URL.Path, "status", sw.status,
				"dur_ms", ms, "remote", r.RemoteAddr, "local", local, "ua", r.UserAgent()}
			if v := r.Header.Get("Mcp-Protocol-Version"); v != "" {
				attrs = append(attrs, "mcp_protocol_version", truncate(v, maxProtocolVersionLen))
			}
			slog.InfoContext(r.Context(), "http request", attrs...)
		})
	}
}

// minLevel passes the records of Handler at level min and above, and drops
// the rest.
type minLevel struct {
	slog.Handler
	min slog.Level
}

func (h minLevel) Enabled(ctx context.Context, l slog.Level) bool {
	return l >= h.min && h.Handler.Enabled(ctx, l)
}

func (h minLevel) Handle(ctx context.Context, rec slog.Record) error {
	if rec.Level < h.min {
		return nil
	}
	return h.Handler.Handle(ctx, rec)
}

func (h minLevel) WithAttrs(attrs []slog.Attr) slog.Handler {
	return minLevel{Handler: h.Handler.WithAttrs(attrs), min: h.min}
}

func (h minLevel) WithGroup(name string) slog.Handler {
	return minLevel{Handler: h.Handler.WithGroup(name), min: h.min}
}
