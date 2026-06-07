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
