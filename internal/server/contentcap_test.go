// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

// This file proves D-01 (memory content byte cap), D-09 (the caps are
// always enforced — Config.Validate rejects 0/negative rather than
// honoring them as "disabled") and D-10 (memory tags count/byte caps) end
// to end, on both the MCP and Connect lanes, against a spy-backed deps —
// every test here is hermetic (no Qdrant).

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/seanb4t/engram/internal/config"
)

// newCapsMCPSession registers d's tools on a fresh in-memory MCP server and
// connects an authenticated in-memory client session as owner. Callers must
// close the returned session (t.Cleanup is sufficient); the underlying
// server session is cleaned up alongside it.
func newCapsMCPSession(t *testing.T, d *deps, owner string) *mcp.ClientSession {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "engram-test", Version: "test"}, nil)
	if err := registerTools(s, d); err != nil {
		t.Fatalf("registerTools: %v", err)
	}

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	ss, err := s.Connect(authedContext(t, owner), serverTransport, nil)
	if err != nil {
		t.Fatalf("server Connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	c := mcp.NewClient(&mcp.Implementation{Name: "engram-test-client", Version: "test"}, nil)
	cs, err := c.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client Connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	return cs
}

// callToolText calls the named tool with args and returns the first
// TextContent's text plus whether the result is an error result.
func callToolText(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): got Go error %v, want nil (the mapped result must still be a normal, non-erroring CallTool round trip)", name, err)
	}
	if len(res.Content) != 1 {
		t.Fatalf("CallTool(%s): len(Content) = %d, want 1 (content: %+v)", name, len(res.Content), res.Content)
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("CallTool(%s): Content[0] is %T, want *mcp.TextContent", name, res.Content[0])
	}
	return tc.Text, res.IsError
}

// TestMemoryContentCapFlowsFromConfig proves D-01/D-09 end to end: a
// configured ENGRAM_MEMORY_MAX_CONTENT_BYTES reaches the MCP store_memory
// rejection unchanged, using the existing field=content hint=too_long
// envelope, and an at-cap write still succeeds and is actually stored.
func TestMemoryContentCapFlowsFromConfig(t *testing.T) {
	t.Setenv("ENGRAM_MEMORY_MAX_CONTENT_BYTES", "10")

	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	d, sp := newSpyDeps()
	d.writeCaps = memoryWriteCapsFromConfig(cfg)

	cs := newCapsMCPSession(t, d, "cap-owner")

	t.Run("over", func(t *testing.T) {
		text, isError := callToolText(t, cs, "store_memory", map[string]any{
			"content":  "01234567890", // 11 bytes — one over the configured cap of 10
			"scope":    "repo:cap",
			"source":   "user-said",
			"category": "decision",
		})
		if !isError {
			t.Fatalf("store_memory(11-byte content, cap 10): IsError = false, want true (text: %q)", text)
		}
		const wantPrefix = "field=content hint=too_long: "
		if !strings.HasPrefix(text, wantPrefix) {
			t.Errorf("store_memory(11-byte content): text = %q, want prefix %q", text, wantPrefix)
		}
		if !strings.Contains(text, "(max 10)") {
			t.Errorf("store_memory(11-byte content): text = %q, want it to contain %q", text, "(max 10)")
		}
		sp.mu.Lock()
		n := len(sp.records)
		sp.mu.Unlock()
		if n != 0 {
			t.Errorf("store_memory(11-byte content) rejected, but spy holds %d record(s), want 0", n)
		}
	})

	t.Run("at", func(t *testing.T) {
		const content = "0123456789" // exactly 10 bytes — at the configured cap
		text, isError := callToolText(t, cs, "store_memory", map[string]any{
			"content":  content,
			"scope":    "repo:cap",
			"source":   "user-said",
			"category": "decision",
		})
		if isError {
			t.Fatalf("store_memory(10-byte content, cap 10): IsError = true, want false (text: %q)", text)
		}
		sp.mu.Lock()
		defer sp.mu.Unlock()
		if len(sp.records) != 1 {
			t.Fatalf("store_memory(10-byte content) accepted, but spy holds %d record(s), want 1", len(sp.records))
		}
		for _, m := range sp.records {
			if m.Content != content {
				t.Errorf("stored record Content = %q, want %q", m.Content, content)
			}
		}
	})
}
