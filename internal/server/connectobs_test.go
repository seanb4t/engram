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

func TestConnectAccessLogInterceptor_SuccessPath(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ic := newConnectAccessLogInterceptor(logger)
	next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, nil // success
	})
	wrapped := ic.WrapUnary(next)
	req := connect.NewRequest(&struct{}{})
	_, _ = wrapped(context.Background(), req)

	out := buf.String()
	if !strings.Contains(out, "code=ok") {
		t.Fatalf("access log missing code=ok: %q", out)
	}
	// Info-level log must not surface as ERROR.
	if strings.Contains(out, "level=ERROR") {
		t.Fatalf("success path must log at INFO, got ERROR: %q", out)
	}
}

func TestConnectAccessLogInterceptor_ServerErrorLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ic := newConnectAccessLogInterceptor(logger)
	next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, connect.NewError(connect.CodeInternal, errors.New("db exploded"))
	})
	wrapped := ic.WrapUnary(next)
	req := connect.NewRequest(&struct{}{})
	_, _ = wrapped(context.Background(), req)

	out := buf.String()
	if !strings.Contains(out, "internal") {
		t.Fatalf("access log missing internal code: %q", out)
	}
	if !strings.Contains(out, "level=ERROR") {
		t.Fatalf("server-error path must log at ERROR level: %q", out)
	}
}
