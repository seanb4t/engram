// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"connectrpc.com/connect"
)

func TestConnectAccessLogInterceptor(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	ic := newConnectAccessLogInterceptor(logger)
	next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("nope"))
	})
	wrapped := ic.WrapUnary(next)
	req := connect.NewRequest(&struct{}{})
	_, _ = wrapped(context.Background(), req)

	out := buf.String()
	if !strings.Contains(out, "unauthenticated") {
		t.Fatalf("access log missing code: %q", out)
	}
}
