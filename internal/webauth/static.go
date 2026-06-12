// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// all: is required — a bare `//go:embed static` excludes files and directories
// whose names begin with "_" or ".", which would drop the SvelteKit build output
// under static/_app/ and leave the SPA unable to mount (GH #106).
//
//go:embed all:static
var staticFS embed.FS

// StaticHandler serves the committed SPA assets and falls back to index.html for
// client-side routes (SPA deep links / refresh). A request whose path has a file
// extension but no matching asset still 404s, so a mistyped asset URL is visible
// rather than masked by index.html.
func StaticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err) // staticFS is compiled-in; a Sub failure is build-time impossible.
	}
	files := http.FileServer(http.FS(sub))
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		panic(err) // index.html is always vendored.
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := strings.TrimPrefix(r.URL.Path, "/")
		if clean == "" {
			clean = "index.html"
		}
		if _, statErr := fs.Stat(sub, clean); statErr == nil {
			files.ServeHTTP(w, r) // real asset
			return
		}
		if path.Ext(clean) != "" {
			http.NotFound(w, r) // looks like an asset but missing → 404
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index) // client route → SPA shell
	})
}
