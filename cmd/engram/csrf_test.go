// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/webauth"
)

// TestCrossOriginProtectionRejectsCrossOrigin proves SC1: a cross-origin
// unsafe-method request is rejected with 403 before the inner handler runs.
func TestCrossOriginProtectionRejectsCrossOrigin(t *testing.T) {
	var innerHit bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		innerHit = true
		w.WriteHeader(http.StatusOK)
	})
	h := newCrossOriginProtection().Handler(inner)

	req := httptest.NewRequest(http.MethodPost, "http://engram.example/engram.v1.EngramService/StoreMemory", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if innerHit {
		t.Fatal("inner handler was invoked on a cross-origin request; SC1 requires rejection before it runs")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusForbidden)
	}
}

// TestCrossOriginProtectionAllowsSafeAndNoOrigin proves D-07: a safe-method
// GET and a request with neither Origin nor Sec-Fetch-Site (the MCP
// transport's shape) both reach the inner handler untouched.
func TestCrossOriginProtectionAllowsSafeAndNoOrigin(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := newCrossOriginProtection().Handler(inner)

	t.Run("safe method GET", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://engram.example/auth/callback", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("no Origin or Sec-Fetch-Site (MCP transport)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://engram.example/mcp", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d want %d", rec.Code, http.StatusOK)
		}
	})
}

// TestCrossOriginDenyHandlerEnvelope proves D-04: the deny handler's 403
// response is a Connect-shaped JSON envelope, not the stdlib default
// plain-text body.
func TestCrossOriginDenyHandlerEnvelope(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := newCrossOriginProtection().Handler(inner)

	req := httptest.NewRequest(http.MethodPost, "http://engram.example/engram.v1.EngramService/StoreMemory", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusForbidden)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type=%q want %q", ct, "application/json")
	}
	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != "permission_denied" {
		t.Fatalf("code=%q want %q", body.Code, "permission_denied")
	}
	if body.Message == "" {
		t.Fatal("message is empty; want a fixed generic message")
	}
}

// TestCSRFWireNamesMatch proves the server and webauth packages agree on the
// CSRF cookie/header wire-contract names byte-for-byte — no drift between
// the issuance path (webauth.Handler.Callback) and the verification path
// (server's Connect CSRF interceptor).
func TestCSRFWireNamesMatch(t *testing.T) {
	if server.CSRFCookieName != webauth.CSRFCookieName {
		t.Fatalf("cookie name drift: server=%q webauth=%q", server.CSRFCookieName, webauth.CSRFCookieName)
	}
	if server.CSRFHeaderName != webauth.CSRFHeaderName {
		t.Fatalf("header name drift: server=%q webauth=%q", server.CSRFHeaderName, webauth.CSRFHeaderName)
	}
}
