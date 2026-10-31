// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/seanb4t/engram/internal/auth"
	"github.com/seanb4t/engram/internal/telemetry"
)

func TestNewMCPServerCapabilities(t *testing.T) {
	st, ct := mcp.NewInMemoryTransports()
	ss, err := newMCPServer("test").Connect(t.Context(), st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer func() { _ = ss.Close() }()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil).Connect(t.Context(), ct, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() { _ = cs.Close() }()
	caps, err := json.Marshal(cs.InitializeResult().Capabilities)
	if err != nil {
		t.Fatalf("marshal capabilities: %v", err)
	}
	if got, want := string(caps), `{"tools":{}}`; got != want {
		t.Fatalf("capabilities = %s, want %s", got, want)
	}
}

func TestMCPProtocolRouter(t *testing.T) {
	cases := []struct {
		name, version, session string
		wantSessionless        bool
		wantWarning            bool
	}{
		{name: "no header"},
		{name: "2025-11-25", version: "2025-11-25"},
		{name: "2025-06-18", version: "2025-06-18"},
		{name: "2025-03-26", version: "2025-03-26"},
		{name: "2024-11-05", version: "2024-11-05"},
		{name: "2026-07-28", version: "2026-07-28", wantSessionless: true},
		{name: "2026-07-28 with a session id", version: "2026-07-28", session: "stale", wantSessionless: true},
		{name: "unserved later version", version: "2099-01-01", wantSessionless: true, wantWarning: true},
		{name: "unserved earlier version", version: "2024-01-01", wantWarning: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureLogs(t)
			var ran string
			router := newProtocolRouter(
				http.HandlerFunc(func(http.ResponseWriter, *http.Request) { ran = "sessions" }),
				http.HandlerFunc(func(http.ResponseWriter, *http.Request) { ran = "sessionless" }))
			spans := tracetest.NewSpanRecorder()
			ctx, span := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)).Tracer("test").Start(t.Context(), "mcp")
			req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/mcp", nil)
			req.Header.Set("User-Agent", "router-test/1.0")
			if tc.version != "" {
				req.Header.Set("Mcp-Protocol-Version", tc.version)
			}
			if tc.session != "" {
				req.Header.Set("Mcp-Session-Id", tc.session)
			}
			router.ServeHTTP(httptest.NewRecorder(), req)
			span.End()

			want := "sessions"
			if tc.wantSessionless {
				want = "sessionless"
			}
			if ran != want {
				t.Fatalf("handler = %q, want %q", ran, want)
			}
			var attr string
			for _, kv := range spans.Ended()[0].Attributes() {
				if kv.Key == "mcp.protocol.version" {
					attr = kv.Value.AsString()
				}
			}
			if attr != tc.version {
				t.Fatalf("span mcp.protocol.version = %q, want %q", attr, tc.version)
			}
			warned := strings.Contains(logs.String(), "does not serve")
			if warned != tc.wantWarning {
				t.Fatalf("warning logged = %v, want %v: %s", warned, tc.wantWarning, logs.String())
			}
			if tc.wantWarning {
				generation := generationLegacy
				if tc.wantSessionless {
					generation = generation20260728
				}
				for _, want := range []string{`"mcp_protocol_version":"` + tc.version + `"`, `"generation":"` + generation + `"`, `"ua":"router-test/1.0"`} {
					if !strings.Contains(logs.String(), want) {
						t.Fatalf("warning lacks %s: %s", want, logs.String())
					}
				}
			}
		})
	}

	t.Run("a repeated unserved version warns once", func(t *testing.T) {
		logs := captureLogs(t)
		router := newProtocolRouter(http.NotFoundHandler(), http.NotFoundHandler())
		for range 10 {
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			req.Header.Set("Mcp-Protocol-Version", "2099-01-01")
			router.ServeHTTP(httptest.NewRecorder(), req)
		}
		if n := strings.Count(logs.String(), "does not serve"); n != 1 {
			t.Fatalf("warnings = %d, want 1: %s", n, logs.String())
		}
	})
}

// generationsToken is the static service token of newStaticTokenChain.
const generationsToken = "secret-token-value"

// whoamiOutput is the result of the whoami stub tool: the owner of the
// verified token that called it.
type whoamiOutput struct {
	Owner string `json:"owner"`
}

// generationsServer serves mcpTransport over httptest with newMCPServer, the
// static token chain and a whoami stub tool, and returns its base URL.
func generationsServer(t *testing.T) string {
	t.Helper()
	srv := newMCPServer("test")
	mcp.AddTool(srv, &mcp.Tool{Name: "whoami"}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, whoamiOutput, error) {
		var owner string
		if ti := mcpauth.TokenInfoFromContext(ctx); ti != nil {
			owner, _ = ti.Extra[auth.OwnerClaimExtraKey].(string)
		}
		return nil, whoamiOutput{Owner: owner}, nil
	})
	resource := mustResource(t, protectedResourceConfig{MCPPath: "/mcp"})
	mux := http.NewServeMux()
	mountMCPRoutes(mux, mcpTransport(t.Context(), srv, newStaticTokenChain(t), resource, telemetry.NewToolMetrics(otel.Meter("test"))), false, "/mcp")
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts.URL
}

// bearerTransport adds a bearer token to every request.
type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

// connectGeneration connects a go-sdk client to base at version and checks
// that the server negotiated that version. The client falls back from
// 2026-07-28 to initialize without an error, so the check is what proves the
// generation.
func connectGeneration(t *testing.T, base, version string) *mcp.ClientSession {
	t.Helper()
	c := mcp.NewClient(&mcp.Implementation{Name: "generations-test", Version: "test"}, nil)
	cs, err := c.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint:   base + "/mcp",
		HTTPClient: &http.Client{Transport: bearerTransport{generationsToken}},
	}, &mcp.ClientSessionOptions{ProtocolVersion: version})
	if err != nil {
		t.Fatalf("connect at %s: %v", version, err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	if got := cs.InitializeResult().ProtocolVersion; got != version {
		t.Fatalf("negotiated %s, want %s", got, version)
	}
	return cs
}

// checkToolsAsCaller lists the tools of cs and calls whoami, which must
// report the owner that the token chain verifies for the static token.
func checkToolsAsCaller(t *testing.T, cs *mcp.ClientSession) {
	t.Helper()
	ti, err := newStaticTokenChain(t)(t.Context(), generationsToken, nil)
	if err != nil {
		t.Fatalf("verify the static token: %v", err)
	}
	want, _ := ti.Extra[auth.OwnerClaimExtraKey].(string)
	tools, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	if !slices.ContainsFunc(tools.Tools, func(tool *mcp.Tool) bool { return tool.Name == "whoami" }) {
		t.Fatalf("tools/list = %v, want whoami", tools.Tools)
	}
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "whoami"})
	if err != nil {
		t.Fatalf("tools/call whoami: %v", err)
	}
	var out whoamiOutput
	b, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(b, &out); err != nil || want == "" || out.Owner != want {
		t.Fatalf("whoami = %s (%v), want owner %q", b, err, want)
	}
}

// meta20260728 is the SEP-2575 _meta envelope of a raw 2026-07-28 call.
var meta20260728 = map[string]any{
	"io.modelcontextprotocol/protocolVersion":    protocolVersion20260728,
	"io.modelcontextprotocol/clientInfo":         map[string]string{"name": "raw", "version": "test"},
	"io.modelcontextprotocol/clientCapabilities": map[string]any{},
}

// call20260728 POSTs one raw 2026-07-28 call of method with params to base
// and returns the response and the call's JSON-RPC result.
func call20260728(t *testing.T, base, method string, params map[string]any, header map[string]string) (*http.Response, json.RawMessage) {
	t.Helper()
	if params == nil {
		params = map[string]any{}
	}
	params["_meta"] = meta20260728
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		t.Fatalf("marshal %s: %v", method, err)
	}
	h := map[string]string{"Mcp-Protocol-Version": protocolVersion20260728, "Mcp-Method": method}
	for k, v := range header {
		h[k] = v
	}
	resp, text := postMCPBody(t, base, generationsToken, string(body), h)
	for line := range strings.Lines(text) {
		data, ok := strings.CutPrefix(strings.TrimSpace(line), "data: ")
		if !ok {
			data = strings.TrimSpace(line)
		}
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Result json.RawMessage `json:"result"`
		}
		if json.Unmarshal([]byte(data), &msg) == nil && string(msg.ID) == "1" && msg.Result != nil {
			return resp, msg.Result
		}
	}
	t.Fatalf("%s: status=%d, no result in %q", method, resp.StatusCode, text)
	return nil, nil
}

// rawMCP sends method to base/mcp with the static token and headers.
func rawMCP(t *testing.T, method, base, token string, header map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, base+"/mcp", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s/mcp: %v", method, base, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// rawLegacySession initializes a raw legacy session at version on base and
// returns its Mcp-Session-Id.
func rawLegacySession(t *testing.T, base, version string) string {
	t.Helper()
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"` + version + `","capabilities":{},"clientInfo":{"name":"raw","version":"test"}}}`
	resp, body := postMCPBody(t, base, generationsToken, initialize, nil)
	session := resp.Header.Get("Mcp-Session-Id")
	if resp.StatusCode != http.StatusOK || session == "" || !strings.Contains(body, `"protocolVersion":"`+version+`"`) {
		t.Fatalf("initialize at %s: status=%d Mcp-Session-Id=%q body=%q", version, resp.StatusCode, session, body)
	}
	postMCPBody(t, base, generationsToken, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, map[string]string{"Mcp-Session-Id": session, "Mcp-Protocol-Version": version})
	return session
}

func TestMCPTransportGenerations(t *testing.T) {
	base := generationsServer(t)

	t.Run("2026-07-28", func(t *testing.T) {
		cs := connectGeneration(t, base, protocolVersion20260728)
		checkToolsAsCaller(t, cs)

		resp, result := call20260728(t, base, "server/discover", nil, nil)
		var discover struct {
			SupportedVersions []string        `json:"supportedVersions"`
			Capabilities      json.RawMessage `json:"capabilities"`
		}
		if err := json.Unmarshal(result, &discover); err != nil {
			t.Fatalf("decode discover: %v", err)
		}
		if want := []string{"2026-07-28", "2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}; !slices.Equal(discover.SupportedVersions, want) {
			t.Fatalf("discover supportedVersions = %v, want %v", discover.SupportedVersions, want)
		}
		if got := string(discover.Capabilities); got != `{"tools":{}}` {
			t.Fatalf("discover capabilities = %s, want {\"tools\":{}}", got)
		}
		if id := resp.Header.Get("Mcp-Session-Id"); id != "" {
			t.Fatalf("discover set Mcp-Session-Id %q", id)
		}

		resp, _ = call20260728(t, base, "tools/list", nil, map[string]string{"Mcp-Session-Id": "stale-session"})
		if resp.StatusCode != http.StatusOK || resp.Header.Get("Mcp-Session-Id") != "" {
			t.Fatalf("tools/list with a stale session: status=%d Mcp-Session-Id=%q, want 200 and none", resp.StatusCode, resp.Header.Get("Mcp-Session-Id"))
		}

		header := map[string]string{"Mcp-Protocol-Version": protocolVersion20260728}
		for _, method := range []string{http.MethodGet, http.MethodDelete} {
			if resp := rawMCP(t, method, base, generationsToken, header); resp.StatusCode != http.StatusMethodNotAllowed || resp.Header.Get("Allow") != "POST" {
				t.Fatalf("%s: status=%d Allow=%q, want 405 POST", method, resp.StatusCode, resp.Header.Get("Allow"))
			}
		}
		if resp := rawMCP(t, http.MethodGet, base, "", header); resp.StatusCode != http.StatusUnauthorized || resp.Header.Get("WWW-Authenticate") == "" {
			t.Fatalf("GET without a token: status=%d WWW-Authenticate=%q, want a 401 challenge", resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
		}

		done := make(chan struct{})
		go func() {
			defer close(done)
			call20260728(t, base, "subscriptions/listen", map[string]any{"notifications": map[string]bool{"toolsListChanged": true}}, nil)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("subscriptions/listen for tool-list changes did not complete")
		}
	})

	for _, version := range []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"} {
		t.Run(version, func(t *testing.T) {
			cs := connectGeneration(t, base, version)
			if cs.ID() == "" {
				t.Fatal("legacy session has no Mcp-Session-Id")
			}
			checkToolsAsCaller(t, cs)

			session := rawLegacySession(t, base, version)
			header := map[string]string{"Mcp-Session-Id": session, "Mcp-Protocol-Version": version}
			stream := rawMCP(t, http.MethodGet, base, generationsToken, map[string]string{"Mcp-Session-Id": session, "Mcp-Protocol-Version": version, "Accept": "text/event-stream"})
			if stream.StatusCode != http.StatusOK || !strings.HasPrefix(stream.Header.Get("Content-Type"), "text/event-stream") {
				t.Fatalf("GET: status=%d Content-Type=%q, want an event stream", stream.StatusCode, stream.Header.Get("Content-Type"))
			}
			_ = stream.Body.Close()

			if resp := rawMCP(t, http.MethodDelete, base, generationsToken, header); resp.StatusCode/100 != 2 {
				t.Fatalf("DELETE: status=%d, want 2xx", resp.StatusCode)
			}
			resp, _ := postMCPBody(t, base, generationsToken, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, header)
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("tools/list after DELETE: status=%d, want 404 for the ended session", resp.StatusCode)
			}
		})
	}
}
