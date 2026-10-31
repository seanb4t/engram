// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccessLogCapturesStatusAndCountsAuthFailures(t *testing.T) {
	var gotStatus int
	var authReason string
	mw := accessLog(
		func(_ context.Context, reason string) { authReason = reason },
		func(status int) { gotStatus = status },
	)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))

	if gotStatus != http.StatusUnauthorized {
		t.Errorf("captured status: got %d", gotStatus)
	}
	if authReason != "unauthorized" {
		t.Errorf("auth reason: got %q", authReason)
	}
}

func TestAccessLogForbiddenFiresAuthFailure(t *testing.T) {
	var gotStatus int
	var authReason string
	mw := accessLog(
		func(_ context.Context, reason string) { authReason = reason },
		func(status int) { gotStatus = status },
	)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if gotStatus != http.StatusForbidden {
		t.Errorf("captured status: got %d, want %d", gotStatus, http.StatusForbidden)
	}
	if authReason != "forbidden" {
		t.Errorf("auth reason: got %q, want %q", authReason, "forbidden")
	}
}

func TestStatusWriterUnwrap(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, status: http.StatusOK}

	if sw.Unwrap() != rec {
		t.Error("Unwrap did not return the wrapped ResponseWriter")
	}

	// http.NewResponseController must be able to reach the underlying writer.
	rc := http.NewResponseController(sw)
	if err := rc.Flush(); err != nil {
		t.Errorf("Flush via ResponseController: %v", err)
	}
}

func TestAccessLogNamesLocalAddress(t *testing.T) {
	logs := captureLogs(t)
	h := accessLog(nil, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	local := net.Addr(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081})
	req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, local))
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !strings.Contains(logs.String(), `"local":"127.0.0.1:8081"`) {
		t.Fatalf("access log does not name the local address: %s", logs.String())
	}
}

func TestAccessLogNamesProtocolVersion(t *testing.T) {
	h := accessLog(nil, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	logs := captureLogs(t)
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Mcp-Protocol-Version", "2026-07-28")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !strings.Contains(logs.String(), `"mcp_protocol_version":"2026-07-28"`) {
		t.Fatalf("access log does not name the protocol version: %s", logs.String())
	}

	logs = captureLogs(t)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if strings.Contains(logs.String(), "mcp_protocol_version") {
		t.Fatalf("access log names a protocol version the request did not carry: %s", logs.String())
	}

	logs = captureLogs(t)
	long := strings.Repeat("9", 2*maxProtocolVersionLen)
	req = httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Mcp-Protocol-Version", long)
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !strings.Contains(logs.String(), `"mcp_protocol_version":"`+long[:maxProtocolVersionLen]+`"`) || strings.Contains(logs.String(), long) {
		t.Fatalf("access log does not cut a long protocol version to %d bytes: %s", maxProtocolVersionLen, logs.String())
	}
}

func TestMinLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(minLevel{Handler: slog.NewJSONHandler(&buf, nil), min: slog.LevelWarn}).With("k", "v").WithGroup("g")
	logger.Info("info record")
	logger.Warn("warn record")
	logger.Error("error record")
	out := buf.String()
	if strings.Contains(out, "info record") {
		t.Fatalf("an info record passed the warn floor: %s", out)
	}
	for _, want := range []string{"warn record", "error record", `"k":"v"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("output lacks %s: %s", want, out)
		}
	}
}
