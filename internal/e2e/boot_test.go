// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package e2e

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestStartupRejectsMalformedChatBaseURL pins the fail-fast contract at the
// BINARY level: a malformed data-plane value aborts startup with an error naming
// the offending variable, before any client is constructed.
//
// internal/config's TestValidateChatBaseURLOverride already proves Validate
// rejects these values, and internal/server's
// TestStoreAndEmbedderFromEnvNoEnsureValidatesConfig proves loadAndValidate is
// on the store-building path. What neither covers is that `engram serve`
// actually reaches that path and exits non-zero — the thing an operator sees.
//
// Deliberately Qdrant-free: ENGRAM_QDRANT_ADDR is well-formed but unroutable, so
// reaching a *connection* error instead of a validation error would itself be
// the regression (validation must run first). RFC5737 TEST-NET-1 is used so a
// wrong ordering fails on the assertion rather than hanging on a live host.
func TestStartupRejectsMalformedChatBaseURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		value string
	}{
		{"unparseable", "http://[::1"},
		{"non-http scheme", "ftp://x/v1"},
		{"missing host", "http://"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, engramBin, "serve")
			cmd.Env = childEnv(map[string]string{
				"ENGRAM_QDRANT_ADDR":          "192.0.2.1:6334",
				"ENGRAM_LISTEN_ADDR":          "127.0.0.1:0",
				"ENGRAM_OPENAI_BASE_URL":      "http://127.0.0.1:9",
				"ENGRAM_OPENAI_CHAT_BASE_URL": tc.value,
			})
			out, err := cmd.CombinedOutput()

			if err == nil {
				t.Fatalf("engram serve with ENGRAM_OPENAI_CHAT_BASE_URL=%q exited 0, want non-zero\n%s", tc.value, out)
			}
			if ctx.Err() != nil {
				t.Fatalf("engram serve hung with ENGRAM_OPENAI_CHAT_BASE_URL=%q — validation must run before any client is built\n%s", tc.value, out)
			}
			if !strings.Contains(string(out), "ENGRAM_OPENAI_CHAT_BASE_URL") {
				t.Errorf("startup error does not name ENGRAM_OPENAI_CHAT_BASE_URL (an operator cannot tell what to fix):\n%s", out)
			}
			if !strings.Contains(string(out), "invalid configuration") {
				t.Errorf("startup error is not the aggregated validation error — did it fail on a Qdrant connection instead?\n%s", out)
			}
		})
	}
}

// TestStartupAcceptsHostedChatBaseURL is the positive companion: a well-formed
// hosted URL (the REQ-chat-base-url headline shape, which ends in /v1) must not
// be rejected. Without this, a validator that rejected everything would still
// pass the negative cases above.
func TestStartupAcceptsHostedChatBaseURL(t *testing.T) {
	srv := startServer(t, map[string]string{
		"ENGRAM_OPENAI_CHAT_BASE_URL": "https://api.openai.com/v1",
	})
	if srv.addr == "" {
		t.Fatal("server did not start with a well-formed hosted chat base URL")
	}
}

// documentedTools is the tool surface promised by CLAUDE.md's memory contract.
// Asserting the full set (not a sample) is the point: a tool that silently fails
// to register is invisible to every handler-level test, since those call the
// handler directly rather than going through registration. Adding a tool should
// require updating this list deliberately.
var documentedTools = []string{
	"delete_all", "delete_memory", "get_memory", "list_memory", "list_rules",
	"list_scheduled", "schedule_memory", "search_discovery", "search_memory",
	"set_visibility", "store_discovery", "store_memory", "store_rule",
	"supersede_memory", "update_memory",
}

// mcpConnect dials the running server over the real streamable-HTTP transport,
// completing the full initialize handshake.
func mcpConnect(t *testing.T, srv *serverProc) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	c := mcp.NewClient(&mcp.Implementation{Name: "engram-e2e", Version: "test"}, nil)
	cs, err := c.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.endpoint()}, nil)
	if err != nil {
		t.Fatalf("MCP connect to %s: %v", srv.endpoint(), err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// TestBootServesDocumentedToolSurface is the core wiring smoke test: build ->
// boot -> MCP handshake -> tools/list. It is the only test that would catch a
// break in cobra/env -> runServe -> mux -> transport -> registration.
func TestBootServesDocumentedToolSurface(t *testing.T) {
	srv := startServer(t, nil)
	cs := mcpConnect(t, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	got := make(map[string]bool, len(res.Tools))
	for _, tool := range res.Tools {
		got[tool.Name] = true
	}
	for _, want := range documentedTools {
		if !got[want] {
			t.Errorf("tool %q not registered on the served surface", want)
		}
	}
	if len(res.Tools) != len(documentedTools) {
		names := make([]string, 0, len(res.Tools))
		for _, tool := range res.Tools {
			names = append(names, tool.Name)
		}
		t.Errorf("served %d tools, documentedTools lists %d — update the list deliberately.\nserved: %v",
			len(res.Tools), len(documentedTools), names)
	}
}

// callTool invokes a tool and fails on transport error or an isError result.
func callTool(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: transport error: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s: tool returned an error: %s", name, contentText(res))
	}
	return res
}

func contentText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// structuredJSON renders a result's structured content for substring assertions
// about which KEYS crossed the wire (e.g. whether "citations" is present).
func structuredJSON(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	return string(b)
}

// TestBootCitationsAndCategoryFilterRoundTrip drives Phase 26's two data-plane
// features through the deployed shape — real binary, real MCP transport, real
// Qdrant — rather than through in-process handlers.
//
// Semantics are covered more precisely in internal/server (see
// TestSearchListMemoryCompactViewOmitsCitations, TestCategoriesArgEdges); this
// asserts only that they survive the full stack, which is what an operator gets.
func TestBootCitationsAndCategoryFilterRoundTrip(t *testing.T) {
	srv := startServer(t, nil)
	cs := mcpConnect(t, srv)
	const scope = "repo:e2e-roundtrip"

	// A decision record carrying two citations in a deliberate order (url before
	// file): citation order is caller-authored and must be preserved verbatim.
	stored := callTool(t, cs, "store_memory", map[string]any{
		"content":  "We chose OR semantics for the categories filter.",
		"scope":    scope,
		"category": "decision",
		"source":   "agent-inferred",
		"summary":  "categories filter uses OR",
		"citations": []map[string]any{
			{"kind": "url", "ref": "https://example.com/adr-9", "excerpt": "OR across categories"},
			{"kind": "file", "ref": "internal/store/store.go", "locator": "789-801", "excerpt": "categoryMatchCondition"},
		},
	})
	shortID := extractShortID(t, stored)

	callTool(t, cs, "store_memory", map[string]any{
		"content":  "Naive base URL concat causes a doubled /v1 and a 404.",
		"scope":    scope,
		"category": "gotcha",
		"source":   "agent-inferred",
		"summary":  "double /v1 404",
	})

	t.Run("category filter narrows with OR semantics", func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			categories []string
			want       int
		}{
			{"no filter", nil, 2},
			{"single category", []string{"decision"}, 1},
			{"two categories (OR)", []string{"decision", "gotcha"}, 2},
			{"empty slice is a passthrough, not a contradiction", []string{}, 2},
			{"unknown value matches nothing", []string{"no-such-category"}, 0},
		} {
			t.Run(tc.name, func(t *testing.T) {
				args := map[string]any{"scope": scope}
				if tc.categories != nil {
					args["categories"] = tc.categories
				}
				// An unknown category must return an empty list, never an error:
				// an error would be an existence oracle for categories (D-11).
				// callTool fails the test on IsError, which enforces that here.
				res := callTool(t, cs, "list_memory", args)
				if n := countMemories(t, res); n != tc.want {
					t.Errorf("list_memory categories=%v returned %d memories, want %d", tc.categories, n, tc.want)
				}
			})
		}
	})

	t.Run("compact recall omits citations", func(t *testing.T) {
		res := callTool(t, cs, "list_memory", map[string]any{"scope": scope, "categories": []string{"decision"}})
		if strings.Contains(structuredJSON(t, res), `"citations"`) {
			t.Error("compact list_memory leaked citations — the default view is a token budget boundary")
		}
	})

	t.Run("full recall includes citations", func(t *testing.T) {
		res := callTool(t, cs, "list_memory", map[string]any{"scope": scope, "categories": []string{"decision"}, "full": true})
		if !strings.Contains(structuredJSON(t, res), "https://example.com/adr-9") {
			t.Errorf("full=true did not return citations:\n%s", structuredJSON(t, res))
		}
	})

	t.Run("get_memory returns citations in submitted order", func(t *testing.T) {
		res := callTool(t, cs, "get_memory", map[string]any{"id": shortID})
		body := structuredJSON(t, res)
		urlAt := strings.Index(body, "https://example.com/adr-9")
		fileAt := strings.Index(body, "internal/store/store.go")
		if urlAt < 0 || fileAt < 0 {
			t.Fatalf("get_memory did not return both citations:\n%s", body)
		}
		if urlAt > fileAt {
			t.Error("citation order not preserved: the url citation was submitted first but came back after the file citation")
		}
		if !strings.Contains(body, "789-801") {
			t.Error("citation locator was not round-tripped verbatim")
		}
	})
}

// extractShortID pulls the server-minted short_id out of a store_memory result.
func extractShortID(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	var payload struct {
		ShortID string `json:"short_id"`
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal store result: %v", err)
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		t.Fatalf("decode store result %s: %v", b, err)
	}
	if payload.ShortID == "" {
		t.Fatalf("store_memory returned no short_id: %s", b)
	}
	return payload.ShortID
}

// countMemories reads the memories array length out of a recall result.
func countMemories(t *testing.T, res *mcp.CallToolResult) int {
	t.Helper()
	var payload struct {
		Memories []json.RawMessage `json:"memories"`
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal recall result: %v", err)
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		t.Fatalf("decode recall result %s: %v", b, err)
	}
	return len(payload.Memories)
}
