// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
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
