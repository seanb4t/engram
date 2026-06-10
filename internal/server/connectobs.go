// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
)

// serverErrorCode returns true for Connect codes that indicate a server-side
// fault (as opposed to a client or auth error).
func serverErrorCode(c connect.Code) bool {
	switch c {
	case connect.CodeInternal, connect.CodeUnknown, connect.CodeDataLoss, connect.CodeUnimplemented:
		return true
	}
	return false
}

// newConnectAccessLogInterceptor logs one line per unary RPC with the procedure
// and the resulting connect code, giving the Connect lane access-log parity with
// the MCP path (cmd/engram/httplog.go). Severity is slog.LevelError for
// server-class codes (internal, unknown, data_loss, unimplemented) and
// slog.LevelInfo for all others (ok, client errors, unauthenticated). The log
// call uses Log(ctx, …) so trace_id/span_id from the otelconnect span are
// stamped when the slog/otel bridge is active (see internal/telemetry/logger.go).
func newConnectAccessLogInterceptor(logger *slog.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			resp, err := next(ctx, req)
			code := "ok"
			level := slog.LevelInfo
			if err != nil {
				c := connect.CodeOf(err)
				code = c.String()
				if serverErrorCode(c) {
					level = slog.LevelError
				}
			}
			logger.Log(ctx, level, "connect rpc",
				"procedure", req.Spec().Procedure,
				"code", code)
			return resp, err
		}
	}
}
