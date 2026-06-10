// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var staticFS embed.FS

// StaticHandler serves the committed SPA assets. v1 ships a placeholder
// index.html; the SvelteKit build replaces the static/ contents in the SPA plan
// (the go:embed + handler are unchanged). ADRs engram-0lu / engram-bgj.
func StaticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		// staticFS is compiled-in; a Sub failure is a build-time impossibility.
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
