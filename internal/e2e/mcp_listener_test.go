// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"syscall"
	"testing"
	"time"
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
// static service token, ENGRAM_MCP_RESOURCE_ORIGINS and the headless Connect
// API, and checks end to end: the second port serves the gated MCP
// transport and the discovery documents and nothing else, the primary port
// still serves Connect, both ports refuse the path-suffix document for an
// unlisted origin and serve it the host-only document, and a SIGTERM exits 0
// before the shutdown deadline with legacy event streams open on both ports
// and a 2026-07-28 client that called subscriptions/listen.
func TestMCPOnlyListener(t *testing.T) {
	mcpAddr := fmt.Sprintf("127.0.0.1:%d", freePort(t))
	srv := startServer(t, map[string]string{
		"ENGRAM_MCP_LISTEN_ADDR":            mcpAddr,
		"ENGRAM_SERVICE_AUTH_STATIC_TOKENS": "e2e=" + mcpListenerToken,
		"ENGRAM_MCP_RESOURCE_ORIGINS":       "http://" + mcpAddr,
		"ENGRAM_CONNECT_HEADLESS":           "true",
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
		t.Run("an unlisted origin gets no path-suffix document on the "+name+" listener", func(t *testing.T) {
			resp, _ := mcpRequest(t, http.MethodGet, url+"/.well-known/oauth-protected-resource/mcp", "c.example", "", "", nil)
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("status=%d want 404", resp.StatusCode)
			}
		})
		t.Run("an unlisted origin gets the host-only document on the "+name+" listener", func(t *testing.T) {
			resp, body := mcpRequest(t, http.MethodGet, url+"/.well-known/oauth-protected-resource", "c.example", "", "", nil)
			if resp.StatusCode != http.StatusOK || !strings.Contains(body, `"resource":"http://c.example/mcp"`) {
				t.Fatalf("status=%d body=%q, want 200 naming http://c.example/mcp", resp.StatusCode, body)
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

	t.Run("a Connect procedure is refused on the MCP-only listener only", func(t *testing.T) {
		const procedure = "/engram.v1.EngramService/SearchMemories"
		if resp, _ := mcpRequest(t, http.MethodPost, srv.baseURL()+procedure, "", mcpListenerToken, "{}", nil); resp.StatusCode == http.StatusNotFound {
			t.Fatalf("primary listener: status=%d, want the Connect API to answer", resp.StatusCode)
		}
		if resp, _ := mcpRequest(t, http.MethodPost, base+procedure, "", mcpListenerToken, "{}", nil); resp.StatusCode != http.StatusNotFound {
			t.Fatalf("MCP-only listener: status=%d want 404", resp.StatusCode)
		}
	})

	t.Run("SIGTERM with both protocol generations connected exits 0", func(t *testing.T) {
		streams := []*http.Response{openLegacyEventStream(t, base), openLegacyEventStream(t, srv.baseURL())}
		defer func() {
			for _, s := range streams {
				_ = s.Body.Close()
			}
		}()

		cs := mcpConnectAt(t, base+"/mcp", "2026-07-28", mcpListenerToken)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := cs.ListTools(ctx, nil); err != nil {
			t.Fatalf("2026-07-28 tools/list: %v", err)
		}
		listen := `{"jsonrpc":"2.0","id":7,"method":"subscriptions/listen","params":{"notifications":{"toolsListChanged":true},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"engram-e2e","version":"test"},"io.modelcontextprotocol/clientCapabilities":{}}}}`
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/mcp", strings.NewReader(listen))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		for k, v := range map[string]string{
			"Content-Type": "application/json", "Accept": "application/json, text/event-stream",
			"Authorization": "Bearer " + mcpListenerToken, "Mcp-Protocol-Version": "2026-07-28", "Mcp-Method": "subscriptions/listen",
		} {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("subscriptions/listen: %v", err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"id":7`) || !strings.Contains(string(body), `"result"`) {
			t.Fatalf("subscriptions/listen did not complete: status=%d err=%v body=%q", resp.StatusCode, err, body)
		}

		signalled := time.Now()
		if err := srv.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatalf("SIGTERM: %v", err)
		}
		if err := srv.cmd.Wait(); err != nil {
			t.Fatalf("engram serve after SIGTERM: %v, want exit 0", err)
		}
		if elapsed := time.Since(signalled); elapsed >= 15*time.Second {
			t.Fatalf("shutdown took %s, want less than the 15s deadline", elapsed)
		}
	})
}

// openLegacyEventStream initializes a legacy session on base and opens its
// GET event stream, returning the open response.
func openLegacyEventStream(t *testing.T, base string) *http.Response {
	t.Helper()
	resp, _ := mcpRequest(t, http.MethodPost, base+"/mcp", "", mcpListenerToken, mcpInitialize, nil)
	session := resp.Header.Get("Mcp-Session-Id")
	if session == "" {
		t.Fatalf("initialize on %s returned no Mcp-Session-Id", base)
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
		t.Fatalf("open the event stream on %s: %v", base, err)
	}
	if stream.StatusCode != http.StatusOK {
		_ = stream.Body.Close()
		t.Fatalf("event stream on %s: status=%d want 200", base, stream.StatusCode)
	}
	return stream
}

// TestMCPOnlyListenerWithConsole runs `engram serve` with the console and
// ENGRAM_MCP_LISTEN_ADDR, and checks that the primary port still serves the
// console and the login route while the MCP-only port refuses both.
func TestMCPOnlyListenerWithConsole(t *testing.T) {
	mcpAddr := fmt.Sprintf("127.0.0.1:%d", freePort(t))
	fx := startConsoleServerWithEnv(t, map[string]string{
		"ENGRAM_MCP_LISTEN_ADDR":            mcpAddr,
		"ENGRAM_SERVICE_AUTH_STATIC_TOKENS": "e2e=" + mcpListenerToken,
	})
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	get := func(t *testing.T, url string) *http.Response {
		t.Helper()
		resp, err := noRedirect.Get(url)
		if err != nil {
			t.Fatalf("GET %s: %v", url, err)
		}
		_ = resp.Body.Close()
		return resp
	}
	for _, path := range []string{"/ui/", "/auth/login"} {
		t.Run(path+" answers on the primary listener", func(t *testing.T) {
			if resp := get(t, fx.srv.baseURL()+path); resp.StatusCode == http.StatusNotFound {
				t.Fatalf("status=%d, want the primary listener to serve it", resp.StatusCode)
			}
		})
		t.Run(path+" is refused on the MCP-only listener", func(t *testing.T) {
			if resp := get(t, "http://"+mcpAddr+path); resp.StatusCode != http.StatusNotFound || resp.Header.Get("Location") != "" {
				t.Fatalf("status=%d Location=%q, want 404 with no redirect", resp.StatusCode, resp.Header.Get("Location"))
			}
		})
	}
	t.Run("the bare root redirects to the console on the primary listener only", func(t *testing.T) {
		if resp := get(t, fx.srv.baseURL()+"/"); resp.StatusCode != http.StatusFound {
			t.Fatalf("primary: status=%d want 302", resp.StatusCode)
		}
		if resp := get(t, "http://"+mcpAddr+"/"); resp.StatusCode != http.StatusNotFound {
			t.Fatalf("MCP-only: status=%d want 404", resp.StatusCode)
		}
	})
}
