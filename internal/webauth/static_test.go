// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"io/fs"
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

// TestStaticHandlerServesAppAssets guards the SvelteKit build output under
// _app/, which lives in a directory whose name begins with "_". A bare
// `//go:embed static` directive silently excludes "_"-prefixed subtrees, so the
// SPA's JS/CSS would 404 and the console would never mount (GH #106). This test
// fails loudly if the embed loses the _app subtree again.
func TestStaticHandlerServesAppAssets(t *testing.T) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		t.Fatalf("fs.Sub(static): %v", err)
	}

	// Discover a real hashed JS asset rather than hardcoding a content hash
	// (which changes on every UI rebuild).
	var asset string
	_ = fs.WalkDir(sub, "_app/immutable", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".js") {
			asset = p
			return fs.SkipAll
		}
		return nil
	})
	if asset == "" {
		t.Fatal("no _app/immutable/*.js in embed — //go:embed in static.go is " +
			"likely missing the `all:` prefix (a bare directory pattern excludes " +
			"\"_\"-prefixed subtrees, dropping the entire SvelteKit build)")
	}

	h := StaticHandler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/"+asset, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /%s: status=%d want 200", asset, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("GET /%s: content-type=%q want javascript", asset, ct)
	}
	if rec.Body.Len() == 0 {
		t.Fatalf("GET /%s: empty body", asset)
	}

	// _app/version.json has a stable (unhashed) name — a fixed regression anchor.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_app/version.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /_app/version.json: status=%d want 200", rec.Code)
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
