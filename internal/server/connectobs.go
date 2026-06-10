// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
)

// newConnectAccessLogInterceptor logs one line per unary RPC with the procedure
// and the resulting connect code, giving the Connect lane access-log parity with
// the MCP path (cmd/engram/httplog.go). It uses InfoContext so trace_id/span_id
// from the otelconnect span are stamped when the slog/otel bridge is active
// (see internal/telemetry/logger.go).
func newConnectAccessLogInterceptor(logger *slog.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			resp, err := next(ctx, req)
			code := "ok"
			if err != nil {
				code = connect.CodeOf(err).String()
			}
			logger.InfoContext(ctx, "connect rpc",
				"procedure", req.Spec().Procedure,
				"code", code)
			return resp, err
		}
	}
}
