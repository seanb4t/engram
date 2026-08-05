// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// wantRegisteredToolNames is the full-set inventory this test pins: the 15
// tool registrations already live on main (memory contract in CLAUDE.md).
// This is a deliberate, single, reviewable inventory assertion on tool
// IDENTIFIERS — it does NOT retype any tool's Description or Annotations
// (the values plan 02-02/02-04's conformance gates read from THIS test's
// registeredTools helper, from the live registration, never from a second
// hand-typed copy here).
var wantRegisteredToolNames = []string{
	"store_memory", "schedule_memory", "search_memory", "list_memory",
	"list_scheduled", "get_memory", "update_memory", "delete_memory",
	"delete_all", "store_discovery", "search_discovery", "set_visibility",
	"supersede_memory", "store_rule", "list_rules",
}

// registeredTools connects an in-memory MCP client to a server built by
// calling registerTools(s, &deps{}) — no live Qdrant, no live embedder, no
// ENGRAM_* environment configuration — and returns the real registered
// tool set via a tools/list round trip. This is the reusable seam Task 3's
// MCP-Description conformance gate and plan 02-04's both-directions
// annotation gate both consume, so every caller reads the SAME real
// registration rather than each building its own.
func registeredTools(t *testing.T) []*mcp.Tool {
	t.Helper()

	s := mcp.NewServer(&mcp.Implementation{Name: "engram-test", Version: "test"}, nil)
	d := &deps{}
	if err := registerTools(s, d); err != nil {
		t.Fatalf("registerTools: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	ss, err := s.Connect(ctx, serverTransport, nil)
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

	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	return res.Tools
}

// TestRegisterToolsEnumerable proves registerTools registers the real,
// full 15-tool set — with real, non-empty Descriptions — against a bare
// &deps{} literal with no live Qdrant, no live embedder, and no ENGRAM_*
// environment configuration. This is the seam every later conformance gate
// in this phase needs: without it, no test can read the real MCP tool
// Descriptions without standing up a live backend.
func TestRegisterToolsEnumerable(t *testing.T) {
	tools := registeredTools(t)

	got := make(map[string]bool, len(tools))
	for _, tool := range tools {
		got[tool.Name] = true
		if tool.Description == "" {
			t.Errorf("tool %q: Description is empty", tool.Name)
		}
	}

	want := make(map[string]bool, len(wantRegisteredToolNames))
	for _, n := range wantRegisteredToolNames {
		want[n] = true
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("registered tool names = %v, want %v", sortedNames(got), sortedNames(want))
	}
}

func sortedNames(m map[string]bool) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
