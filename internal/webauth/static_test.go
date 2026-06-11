// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStaticHandlerSPAFallback(t *testing.T) {
	h := StaticHandler()
	// A real asset (index.html exists in the embed) serves 200.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("index: status=%d want 200", rec.Code)
	}
	// A client route with no matching asset falls back to index.html (200), not 404.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/observe", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "operator console") {
		t.Fatalf("fallback: status=%d body=%q", rec.Code, rec.Body.String())
	}
	// A genuinely missing asset (has a file extension) still 404s.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_app/missing.js", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing asset: status=%d want 404", rec.Code)
	}
}

func TestStaticHandlerServesIndex(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	StaticHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "operator console") {
		t.Fatalf("index not served: %q", rec.Body.String())
	}
}
