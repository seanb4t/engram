// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestInstrumentToolsExtractsToolNameAndOutcome(t *testing.T) {
	var sawTool, sawOutcome string
	record := func(_ context.Context, tool, outcome string, _ float64) {
		sawTool, sawOutcome = tool, outcome
	}
	mw := instrumentTools(record)

	inner := func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		return &mcp.CallToolResult{}, errors.New("boom")
	}
	h := mw(inner)
	req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "store_memory"}}

	_, _ = h(context.Background(), "tools/call", req)

	if sawTool != "store_memory" {
		t.Errorf("tool name: got %q", sawTool)
	}
	if sawOutcome != "error" {
		t.Errorf("outcome: got %q want error", sawOutcome)
	}
}

func TestInstrumentToolsIgnoresNonToolMethods(t *testing.T) {
	called := false
	record := func(context.Context, string, string, float64) { called = true }
	mw := instrumentTools(record)
	h := mw(func(context.Context, string, mcp.Request) (mcp.Result, error) {
		return &mcp.ListToolsResult{}, nil
	})
	_, _ = h(context.Background(), "tools/list", &mcp.ListToolsRequest{})
	if called {
		t.Error("non-tool method should not record tool metrics")
	}
}
