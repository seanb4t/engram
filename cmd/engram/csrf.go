// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"net/http"
)

// newCrossOriginProtection builds the primary CSRF defense (D-01 layer 1):
// Go 1.26 stdlib net/http.CrossOriginProtection, wrapped around the whole
// top-level mux (D-07 — verified safe against the Go 1.26.5 stdlib source:
// safe-method GET/HEAD/OPTIONS always pass, same-origin Sec-Fetch-Site or a
// matching Origin/Host pass, and requests with neither header — the MCP
// transport — fall through and pass too). No trusted-origin or bypass-pattern
// exemption is registered: strict same-origin is the default posture (SC4 —
// no permissive CORS anywhere).
//
// The deny handler is normalized (D-04) to the same Connect-shaped
// permission_denied/403 envelope newConnectCSRFInterceptor emits (D-03), so
// every client parses all write-lane CSRF failures uniformly instead of
// special-casing the stdlib default's raw-text 403. The message is a fixed,
// generic string — never the underlying cross-origin error's text — so the
// response never leaks internal detail (T-16-05).
func newCrossOriginProtection() *http.CrossOriginProtection {
	ccp := http.NewCrossOriginProtection()
	ccp.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden) // connectCodeToHTTP(CodePermissionDenied) == 403
		_ = json.NewEncoder(w).Encode(struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{
			Code:    "permission_denied",
			Message: "cross-origin request rejected",
		})
	}))
	return ccp
}
