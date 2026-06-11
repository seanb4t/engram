// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// spanAttr returns the string value of the named attribute on a span, or "".
func spanAttr(sp sdktrace.ReadOnlySpan, key string) string {
	for _, kv := range sp.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsString()
		}
	}
	return ""
}

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

func TestInstrumentToolsOkOutcome(t *testing.T) {
	var sawOutcome string
	record := func(_ context.Context, _, outcome string, _ float64) { sawOutcome = outcome }
	mw := instrumentTools(record)

	inner := func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		return &mcp.CallToolResult{}, nil
	}
	h := mw(inner)
	req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "get_memory"}}

	_, _ = h(context.Background(), "tools/call", req)

	if sawOutcome != "ok" {
		t.Errorf("outcome: got %q want ok", sawOutcome)
	}
}

func TestInstrumentToolsIsErrorOutcome(t *testing.T) {
	var sawOutcome string
	record := func(_ context.Context, _, outcome string, _ float64) { sawOutcome = outcome }
	mw := instrumentTools(record)

	inner := func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		return &mcp.CallToolResult{IsError: true}, nil
	}
	h := mw(inner)
	req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "delete_memory"}}

	_, _ = h(context.Background(), "tools/call", req)

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

func TestInstrumentToolsSetsOwnerSpanAttribute(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	prev := otel.GetTracerProvider()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	mw := instrumentTools(func(context.Context, string, string, float64) {})
	h := mw(func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		return &mcp.CallToolResult{}, nil
	})
	ctx := authedContext(t, "owner-xyz")
	_, _ = h(ctx, "tools/call", &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "search_memory"}})

	sp := sr.Ended()
	if len(sp) != 1 {
		t.Fatalf("want 1 span, got %d", len(sp))
	}
	if got := spanAttr(sp[0], "engram.owner"); got != "owner-xyz" {
		t.Errorf("engram.owner = %q, want owner-xyz", got)
	}
}
