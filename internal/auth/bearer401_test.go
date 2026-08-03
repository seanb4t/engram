// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

// This file exists solely as the D-08 scope fence. Phase 4 plan 04-04
// reformats argument-validation message TEXT throughout internal/server
// (D-08). The MCP 401 auth rejection body is NOT argument-validation text —
// it is produced entirely inside the go-sdk's RequireBearerToken/verify
// path (auth/auth.go, this module's github.com/modelcontextprotocol/go-sdk
// dependency) before any internal/server/tools.go code runs, and engram
// e9yv53pmnv records it as a wire contract. TestMCP401BodyByteIdentical
// pins that body and its WWW-Authenticate header byte-for-byte so a future
// reformat cannot silently reach it. Do not delete this as "redundant with"
// TestBearerLaneParityRejectionBodiesMatch in
// connectapi_bearer_parity_test.go — that test pins the EnforceExpiry
// denial bodies ("token expired" / "token missing expiration"); this test
// pins the go-sdk's OWN pre-verifier rejection ("no bearer token") for a
// missing/malformed Authorization header, a case the parity test does not
// drive.
//
// Verified go-sdk source location for the body construction, read this
// session: github.com/modelcontextprotocol/go-sdk@v1.6.1/auth/auth.go —
// verify() returns the literal "no bearer token" at line 104 when the
// Authorization header does not parse as exactly two fields with a
// case-insensitive "bearer" scheme; RequireBearerToken's returned handler
// (lines 72-96) writes that string via http.Error(w, errmsg, code) at line
// 90. Nothing in internal/server or internal/auth contributes to this
// specific body — internal/auth's EnforceExpiry only ever sees a token
// AFTER the header has already parsed as a well-formed bearer credential,
// so it cannot intercept or rewrite the "no bearer token" rejection.

// TestMCP401BodyByteIdentical drives the go-sdk's RequireBearerToken
// middleware — unwrapped by any engram code — with a missing and with a
// malformed Authorization header, and asserts the response status, body,
// and WWW-Authenticate header are byte-identical to what verify() actually
// produces. Comparison is full-string ==, never strings.Contains: a
// containment check would not notice a prefix or suffix change, which is
// exactly the drift e9yv53pmnv exists to prevent.
func TestMCP401BodyByteIdentical(t *testing.T) {
	const resourceMetadataURL = "https://example.com/.well-known/oauth-protected-resource"

	// A verifier that must never be invoked: both cases below are rejected
	// by verify() before the verifier is ever called, because the header
	// does not parse as a well-formed bearer credential.
	unreachable := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		t.Fatal("verifier must not be reached: rejection happens at header-parse time")
		return nil, errors.New("unreachable")
	}

	drive := func(t *testing.T, authHeader string, setHeader bool) (status int, body, wwwAuth string) {
		t.Helper()
		h := mcpauth.RequireBearerToken(unreachable, &mcpauth.RequireBearerTokenOptions{
			ResourceMetadataURL: resourceMetadataURL,
		})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("handler must not be reached for a rejected header")
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if setHeader {
			req.Header.Set("Authorization", authHeader)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code, strings.TrimSpace(rec.Body.String()), rec.Header().Get("WWW-Authenticate")
	}

	t.Run("missing Authorization header", func(t *testing.T) {
		status, body, wwwAuth := drive(t, "", false)
		if status != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", status, http.StatusUnauthorized)
		}
		if body != "no bearer token" {
			t.Errorf("body = %q, want exactly %q", body, "no bearer token")
		}
		wantWWWAuth := `Bearer resource_metadata="` + resourceMetadataURL + `"`
		if wwwAuth != wantWWWAuth {
			t.Errorf("WWW-Authenticate = %q, want exactly %q", wwwAuth, wantWWWAuth)
		}
	})

	t.Run("malformed Authorization header (non-bearer scheme)", func(t *testing.T) {
		status, body, wwwAuth := drive(t, "Basic dXNlcjpwYXNz", true)
		if status != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", status, http.StatusUnauthorized)
		}
		if body != "no bearer token" {
			t.Errorf("body = %q, want exactly %q", body, "no bearer token")
		}
		wantWWWAuth := `Bearer resource_metadata="` + resourceMetadataURL + `"`
		if wwwAuth != wantWWWAuth {
			t.Errorf("WWW-Authenticate = %q, want exactly %q", wwwAuth, wantWWWAuth)
		}
	})

	t.Run("bare scheme, no credential", func(t *testing.T) {
		status, body, wwwAuth := drive(t, "Bearer", true)
		if status != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", status, http.StatusUnauthorized)
		}
		if body != "no bearer token" {
			t.Errorf("body = %q, want exactly %q", body, "no bearer token")
		}
		wantWWWAuth := `Bearer resource_metadata="` + resourceMetadataURL + `"`
		if wwwAuth != wantWWWAuth {
			t.Errorf("WWW-Authenticate = %q, want exactly %q", wwwAuth, wantWWWAuth)
		}
	})
}
