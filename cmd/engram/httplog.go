// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"log/slog"
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
// responses as auth failures. observe is a test seam (nil in production).
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

			slog.Info("http request",
				"method", r.Method, "path", r.URL.Path, "status", sw.status,
				"dur_ms", ms, "remote", r.RemoteAddr, "ua", r.UserAgent())
		})
	}
}
