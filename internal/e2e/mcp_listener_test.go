// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"syscall"
	"testing"
)

const mcpListenerToken = "e2e-listener-secret"

// mcpRequest sends one request to url with host as its Host header and the
// given bearer token (none when empty), and returns the response with its
// body read.
func mcpRequest(t *testing.T, method, url, host, token, body string, header map[string]string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if host != "" {
		req.Host = host
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s %s: %v", method, url, err)
	}
	return resp, string(b)
}

const mcpInitialize = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"engram-e2e","version":"test"}}}`

// TestMCPOnlyListener runs `engram serve` with ENGRAM_MCP_LISTEN_ADDR, a
// static service token and ENGRAM_MCP_RESOURCE_ORIGINS, and checks the
// second port end to end: the gated MCP transport and the bounded discovery
// document, nothing else, and a SIGTERM that exits 0 with an event stream
// open on it.
func TestMCPOnlyListener(t *testing.T) {
	mcpAddr := fmt.Sprintf("127.0.0.1:%d", freePort(t))
	srv := startServer(t, map[string]string{
		"ENGRAM_MCP_LISTEN_ADDR":            mcpAddr,
		"ENGRAM_SERVICE_AUTH_STATIC_TOKENS": "e2e=" + mcpListenerToken,
		"ENGRAM_MCP_RESOURCE_ORIGINS":       "http://" + mcpAddr,
	})
	base := "http://" + mcpAddr

	t.Run("no token gets the challenge for the listed origin", func(t *testing.T) {
		resp, _ := mcpRequest(t, http.MethodPost, base+"/mcp", "", "", mcpInitialize, nil)
		want := `Bearer resource_metadata="` + base + `/.well-known/oauth-protected-resource/mcp"`
		if resp.StatusCode != http.StatusUnauthorized || resp.Header.Get("WWW-Authenticate") != want {
			t.Fatalf("status=%d WWW-Authenticate=%q, want 401 %q", resp.StatusCode, resp.Header.Get("WWW-Authenticate"), want)
		}
	})

	t.Run("no token through an unlisted origin gets a bare challenge", func(t *testing.T) {
		resp, _ := mcpRequest(t, http.MethodPost, base+"/mcp", "c.example", "", mcpInitialize, nil)
		if resp.StatusCode != http.StatusUnauthorized || resp.Header.Get("WWW-Authenticate") != "Bearer" {
			t.Fatalf("status=%d WWW-Authenticate=%q, want 401 %q", resp.StatusCode, resp.Header.Get("WWW-Authenticate"), "Bearer")
		}
	})

	t.Run("a valid token completes the handshake", func(t *testing.T) {
		resp, body := mcpRequest(t, http.MethodPost, base+"/mcp", "", mcpListenerToken, mcpInitialize, nil)
		if resp.StatusCode != http.StatusOK || !strings.Contains(body, `"serverInfo"`) {
			t.Fatalf("status=%d body=%q, want a 200 handshake", resp.StatusCode, body)
		}
	})

	t.Run("the document names the listed origin", func(t *testing.T) {
		resp, body := mcpRequest(t, http.MethodGet, base+"/.well-known/oauth-protected-resource/mcp", "", "", "", nil)
		var doc struct {
			Resource string `json:"resource"`
		}
		if err := json.Unmarshal([]byte(body), &doc); err != nil {
			t.Fatalf("status=%d body=%q: %v", resp.StatusCode, body, err)
		}
		if resp.StatusCode != http.StatusOK || doc.Resource != base+"/mcp" {
			t.Fatalf("status=%d resource=%q, want 200 %q", resp.StatusCode, doc.Resource, base+"/mcp")
		}
	})

	for name, url := range map[string]string{"MCP-only": base, "primary": srv.baseURL()} {
		t.Run("an unlisted origin gets no document on the "+name+" listener", func(t *testing.T) {
			resp, _ := mcpRequest(t, http.MethodGet, url+"/.well-known/oauth-protected-resource/mcp", "c.example", "", "", nil)
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("status=%d want 404", resp.StatusCode)
			}
		})
	}

	for _, path := range []string{"/", "/ui/", "/auth/login"} {
		t.Run("GET "+path+" is refused", func(t *testing.T) {
			resp, _ := mcpRequest(t, http.MethodGet, base+path, "", "", "", nil)
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("status=%d want 404", resp.StatusCode)
			}
		})
	}

	t.Run("a Connect procedure is refused", func(t *testing.T) {
		resp, _ := mcpRequest(t, http.MethodPost, base+"/engram.v1.EngramService/SearchMemories", "", mcpListenerToken, "{}", nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status=%d want 404", resp.StatusCode)
		}
	})

	t.Run("SIGTERM with an open event stream exits 0", func(t *testing.T) {
		resp, _ := mcpRequest(t, http.MethodPost, base+"/mcp", "", mcpListenerToken, mcpInitialize, nil)
		session := resp.Header.Get("Mcp-Session-Id")
		if session == "" {
			t.Fatal("initialize returned no Mcp-Session-Id")
		}
		headers := map[string]string{"Mcp-Session-Id": session, "Mcp-Protocol-Version": "2025-06-18"}
		mcpRequest(t, http.MethodPost, base+"/mcp", "", mcpListenerToken, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, headers)
		req, err := http.NewRequest(http.MethodGet, base+"/mcp", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Authorization", "Bearer "+mcpListenerToken)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		stream, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("open the event stream: %v", err)
		}
		defer stream.Body.Close()
		if stream.StatusCode != http.StatusOK {
			t.Fatalf("event stream status=%d want 200", stream.StatusCode)
		}

		if err := srv.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatalf("SIGTERM: %v", err)
		}
		if err := srv.cmd.Wait(); err != nil {
			t.Fatalf("engram serve after SIGTERM: %v, want exit 0", err)
		}
	})
}
