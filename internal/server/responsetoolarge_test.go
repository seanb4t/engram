// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
	"github.com/seanb4t/engram/internal/auth"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)

// slogRecord is one captured log entry: level, message, and every attribute
// (including WithAttrs-carried ones) rendered as key=value strings.
type slogRecord struct {
	level slog.Level
	msg   string
	attrs map[string]string
}

// slogRecorder is a mutex-guarded sink for slogRecorderHandler. Safe for
// concurrent Handle calls (the go-sdk and Connect stack may log from more
// than one goroutine).
type slogRecorder struct {
	mu      sync.Mutex
	records []slogRecord
}

func (r *slogRecorder) add(rec slogRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, rec)
}

// containing returns every captured record whose message OR any attribute
// value contains substr.
func (r *slogRecorder) containing(substr string) []slogRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []slogRecord
	for _, rec := range r.records {
		if strings.Contains(rec.msg, substr) {
			out = append(out, rec)
			continue
		}
		for _, v := range rec.attrs {
			if strings.Contains(v, substr) {
				out = append(out, rec)
				break
			}
		}
	}
	return out
}

// slogRecorderHandler is a slog.Handler that records every entry into a
// shared *slogRecorder rather than writing anywhere — WithAttrs returns a
// handler carrying the accumulated attrs forward (matching slog's own
// contract), WithGroup is a no-op since this test package never needs
// grouped attribute keys.
type slogRecorderHandler struct {
	rec   *slogRecorder
	attrs []slog.Attr
}

func (h *slogRecorderHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *slogRecorderHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := make(map[string]string, len(h.attrs))
	for _, a := range h.attrs {
		attrs[a.Key] = a.Value.String()
	}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.String()
		return true
	})
	h.rec.add(slogRecord{level: r.Level, msg: r.Message, attrs: attrs})
	return nil
}

func (h *slogRecorderHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	merged = append(merged, h.attrs...)
	merged = append(merged, attrs...)
	return &slogRecorderHandler{rec: h.rec, attrs: merged}
}

func (h *slogRecorderHandler) WithGroup(string) slog.Handler { return h }

// captureSlog installs a slogRecorder as slog.Default() for the duration of
// t, restoring the previous default in t.Cleanup — this is the Connect
// lane's end-to-end proof of D-06's "logged raw, exactly once" requirement
// (rule m45p2b4bp7: we assert OUR log line, never grpc-go's own default).
// Tests using this helper must not call t.Parallel() (mutates the process
// global slog default).
func captureSlog(t *testing.T) *slogRecorder {
	t.Helper()
	rec := &slogRecorder{}
	prev := slog.Default()
	slog.SetDefault(slog.New(&slogRecorderHandler{rec: rec}))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return rec
}

// noASCIIDigit reports whether s contains no ASCII digit — used to assert
// the wire envelope never carries a byte ceiling.
func noASCIIDigit(s string) bool {
	return !regexp.MustCompile(`[0-9]`).MatchString(s)
}

// TestConnectListMemoriesResponseTooLarge is the Connect lane's end-to-end
// proof at the named limit (rule m45p2b4bp7): a real 4 MiB-named overflow
// through Store.List, reached over real HTTP via the production
// mountConnect interceptor chain, surfaces to a Connect client as
// resource_exhausted carrying the ONE shared envelope — never internal, and
// never a byte count or upstream grpc/Qdrant text. This is why no
// internal/e2e binary test is added for this scenario: engram serve dials
// Qdrant with no named receive limit until Phase 5's
// REQ-recv-limit-backstop, so a binary-level overflow would rest on
// grpc-go's own default, which this project's own tests must never assert
// against (rule m45p2b4bp7).
func TestConnectListMemoriesResponseTooLarge(t *testing.T) {
	d, st := testDepsWithStore(t)
	fx := storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: storetest.FewLarge, Vector: []float32{0.1, 0.2, 0.3}})

	resolve := func(_ context.Context, _ connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error) {
		return &mcpauth.TokenInfo{Extra: map[string]any{auth.OwnerClaimExtraKey: fx.Owner}}, auth.LaneBearer, nil
	}
	csrfVerify := func(_, _ string) bool { return true }

	mux := http.NewServeMux()
	if err := d.mountConnect(mux, resolve, csrfVerify, nil); err != nil {
		t.Fatalf("mountConnect: %v", err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)

	rec := captureSlog(t)

	resp, err := client.ListMemories(context.Background(), connect.NewRequest(&engramv1.ListMemoriesRequest{
		Scope: fx.Scope,
		Limit: 0, // all — one full-payload Scroll, exactly the shape that overflows
	}))
	if resp != nil {
		t.Fatalf("ListMemories: got a non-nil response, want nil (the overflow must never look like a success)")
	}
	if err == nil {
		t.Fatal("ListMemories: got nil error, want a resource_exhausted error")
	}
	if code := connect.CodeOf(err); code != connect.CodeResourceExhausted {
		t.Fatalf("ListMemories: code = %v, want CodeResourceExhausted", code)
	}
	var cerr *connect.Error
	if !errors.As(err, &cerr) {
		t.Fatalf("ListMemories: err %v does not unwrap to *connect.Error", err)
	}
	msg := cerr.Message()
	if want := responseTooLargeEnvelope(); msg != want {
		t.Fatalf("ListMemories: message = %q, want %q", msg, want)
	}
	if !strings.HasPrefix(msg, "field=response hint=response_too_large: ") {
		t.Fatalf("ListMemories: message %q does not start with the field/hint envelope prefix", msg)
	}
	for _, banned := range []string{"grpc", "larger than max", "qdrant", "Scroll"} {
		if strings.Contains(msg, banned) {
			t.Errorf("ListMemories: message %q leaks banned substring %q", msg, banned)
		}
	}
	if !noASCIIDigit(msg) {
		t.Errorf("ListMemories: message %q contains an ASCII digit (a byte ceiling must never reach the wire)", msg)
	}

	larger := rec.containing("larger than max")
	if len(larger) != 1 {
		t.Fatalf("captured log records containing %q: got %d, want exactly 1 (log records: %+v)", "larger than max", len(larger), rec.records)
	}
	if larger[0].level != slog.LevelError {
		t.Errorf("the one raw-error log record has level %v, want ERROR", larger[0].level)
	}
	found := false
	for _, v := range larger[0].attrs {
		if strings.Contains(v, "/qdrant.Points/Scroll") {
			found = true
			break
		}
	}
	if !found && !strings.Contains(larger[0].msg, "/qdrant.Points/Scroll") {
		t.Errorf("the one raw-error log record does not contain the RPC method /qdrant.Points/Scroll: %+v", larger[0])
	}
}

// toolCallRecord is one (tool, outcome) pair recorded by toolCallRecorder.
type toolCallRecord struct {
	tool, outcome string
}

// toolCallRecorder is a mutex-guarded recordFunc sink, used to assert D-08's
// "instrumentTools still records outcome=error for the mapped result" edge
// (the mapper must not run so early that instrumentTools never sees the
// tool call at all).
type toolCallRecorder struct {
	mu    sync.Mutex
	calls []toolCallRecord
}

func (r *toolCallRecorder) record(_ context.Context, tool, outcome string, _ float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, toolCallRecord{tool, outcome})
}

func (r *toolCallRecorder) has(tool, outcome string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.calls {
		if c.tool == tool && c.outcome == outcome {
			return true
		}
	}
	return false
}

// TestMCPListMemoryResponseTooLarge is the MCP lane's end-to-end proof at
// the named limit (rule m45p2b4bp7): a real 4 MiB-named overflow through the
// real list_memory tool, reached over an in-memory transport against a
// server built with addToolMiddleware and registerTools, comes back as an
// IsError result carrying the ONE shared envelope, with instrumentTools
// still recording outcome=error and the raw error logged exactly once
// server-side (D-08).
func TestMCPListMemoryResponseTooLarge(t *testing.T) {
	d, st := testDepsWithStore(t)
	fx := storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: storetest.FewLarge, Vector: []float32{0.1, 0.2, 0.3}})

	rec := captureSlog(t)
	tcRec := &toolCallRecorder{}

	s := mcp.NewServer(&mcp.Implementation{Name: "engram-test", Version: "test"}, nil)
	addToolMiddleware(s, tcRec.record)
	if err := registerTools(s, d); err != nil {
		t.Fatalf("registerTools: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	ss, err := s.Connect(authedContext(t, fx.Owner), serverTransport, nil)
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

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_memory",
		Arguments: map[string]any{"scope": fx.Scope, "limit": len(fx.IDs)},
	})
	if err != nil {
		t.Fatalf("CallTool: got Go error %v, want nil (the mapped result must still be a normal, non-erroring CallTool round trip)", err)
	}
	if !res.IsError {
		t.Fatalf("CallTool: IsError = false, want true")
	}
	if len(res.Content) != 1 {
		t.Fatalf("CallTool: len(Content) = %d, want 1 (content: %+v)", len(res.Content), res.Content)
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("CallTool: Content[0] is %T, want *mcp.TextContent", res.Content[0])
	}
	if want := responseTooLargeEnvelope(); tc.Text != want {
		t.Fatalf("CallTool: Text = %q, want %q", tc.Text, want)
	}
	for _, banned := range []string{"grpc", "larger than max", "qdrant", "Scroll"} {
		if strings.Contains(tc.Text, banned) {
			t.Errorf("CallTool: text %q leaks banned substring %q", tc.Text, banned)
		}
	}
	if !noASCIIDigit(tc.Text) {
		t.Errorf("CallTool: text %q contains an ASCII digit (a byte ceiling must never reach the wire)", tc.Text)
	}

	larger := rec.containing("larger than max")
	if len(larger) != 1 {
		t.Fatalf("captured log records containing %q: got %d, want exactly 1 (log records: %+v)", "larger than max", len(larger), rec.records)
	}
	if larger[0].level != slog.LevelError {
		t.Errorf("the one raw-error log record has level %v, want ERROR", larger[0].level)
	}

	if !tcRec.has("list_memory", "error") {
		t.Errorf("recorded (tool,outcome) pairs do not include (list_memory, error): %+v", tcRec.calls)
	}
}

// findFuncDecl returns the top-level *ast.FuncDecl named name in af, or nil.
func findFuncDecl(af *ast.File, name string) *ast.FuncDecl {
	for _, decl := range af.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

// countCallsByIdent counts CallExpr nodes within fn whose Fun is a bare
// *ast.Ident matching name (an unqualified function call, e.g.
// addToolMiddleware(...)).
func countCallsByIdent(fn *ast.FuncDecl, name string) int {
	n := 0
	ast.Inspect(fn, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == name {
			n++
		}
		return true
	})
	return n
}

// findSelectorCalls returns every CallExpr within fn whose Fun is a
// *ast.SelectorExpr with the given selector name (e.g. s.AddReceivingMiddleware(...)).
func findSelectorCalls(fn *ast.FuncDecl, name string) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(fn, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == name {
			calls = append(calls, call)
		}
		return true
	})
	return calls
}

// isCallTo reports whether expr is a CallExpr invoking the bare, unqualified
// function name.
func isCallTo(expr ast.Expr, name string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	ident, ok := call.Fun.(*ast.Ident)
	return ok && ident.Name == name
}

// TestRegisterInstallsToolMiddleware is D-08's source gate (go/parser over
// this package's own tools.go and instrument.go, mirroring
// conditionalsweep_test.go's house style): Register contains exactly one
// call to addToolMiddleware and no direct AddReceivingMiddleware call;
// addToolMiddleware contains exactly one AddReceivingMiddleware call whose
// arguments are, in order, a call to instrumentTools and a call to
// mapResponseTooLarge, and no other arguments. This is what pins the
// ordering (instrumentTools outermost, mapper innermost) structurally,
// rather than by convention.
func TestRegisterInstallsToolMiddleware(t *testing.T) {
	fset := token.NewFileSet()

	toolsAst, err := parser.ParseFile(fset, "tools.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("ParseFile(tools.go): %v", err)
	}
	registerFn := findFuncDecl(toolsAst, "Register")
	if registerFn == nil {
		t.Fatal("Register FuncDecl not found in tools.go")
	}
	if n := countCallsByIdent(registerFn, "addToolMiddleware"); n != 1 {
		t.Errorf("Register calls addToolMiddleware %d times, want 1", n)
	}
	if calls := findSelectorCalls(registerFn, "AddReceivingMiddleware"); len(calls) != 0 {
		t.Errorf("Register calls AddReceivingMiddleware directly %d times, want 0 (must go through addToolMiddleware)", len(calls))
	}

	instrumentAst, err := parser.ParseFile(fset, "instrument.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("ParseFile(instrument.go): %v", err)
	}
	addToolFn := findFuncDecl(instrumentAst, "addToolMiddleware")
	if addToolFn == nil {
		t.Fatal("addToolMiddleware FuncDecl not found in instrument.go")
	}
	calls := findSelectorCalls(addToolFn, "AddReceivingMiddleware")
	if len(calls) != 1 {
		t.Fatalf("addToolMiddleware contains %d AddReceivingMiddleware calls, want 1", len(calls))
	}
	call := calls[0]
	if len(call.Args) != 2 {
		t.Fatalf("AddReceivingMiddleware call has %d args, want 2 (got %d)", len(call.Args), len(call.Args))
	}
	if !isCallTo(call.Args[0], "instrumentTools") {
		t.Errorf("AddReceivingMiddleware arg 0 is not a call to instrumentTools: %#v", call.Args[0])
	}
	if !isCallTo(call.Args[1], "mapResponseTooLarge") {
		t.Errorf("AddReceivingMiddleware arg 1 is not a call to mapResponseTooLarge: %#v", call.Args[1])
	}
}

// TestMapResponseTooLargePassesOtherResultsThrough drives mapResponseTooLarge
// directly against a fake next (instrument_test.go's house style), pinning
// D-09's pass-through guarantee and the mapper's empty/adjacency edges: every
// method other than "tools/call", every non-error result, every OTHER tool
// error (an *argError, store.ErrNotFound, an unclassified server-side
// ResourceExhausted), and a Go-level error from next are all returned
// completely unchanged; only store.ErrResponseTooLarge is rewritten.
func TestMapResponseTooLargePassesOtherResultsThrough(t *testing.T) {
	mw := mapResponseTooLarge()

	t.Run("non_tools_call_method", func(t *testing.T) {
		calls := 0
		want := &mcp.ListToolsResult{}
		next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
			calls++
			return want, nil
		}
		res, err := mw(next)(context.Background(), "tools/list", &mcp.ListToolsRequest{})
		if err != nil {
			t.Errorf("err = %v, want nil", err)
		}
		if res != want {
			t.Errorf("result pointer changed: got %#v, want %#v", res, want)
		}
		if calls != 1 {
			t.Errorf("next called %d times, want 1", calls)
		}
	})

	t.Run("no_error_result", func(t *testing.T) {
		want := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}
		next := func(context.Context, string, mcp.Request) (mcp.Result, error) { return want, nil }
		res, err := mw(next)(context.Background(), "tools/call", &mcp.CallToolRequest{})
		if err != nil {
			t.Errorf("err = %v, want nil", err)
		}
		ctr, ok := res.(*mcp.CallToolResult)
		if !ok || ctr != want {
			t.Fatalf("result changed: got %#v, want the same pointer as %#v", res, want)
		}
		if ctr.IsError {
			t.Errorf("IsError = true, want false")
		}
		tc, ok := ctr.Content[0].(*mcp.TextContent)
		if !ok || tc.Text != "ok" {
			t.Errorf("content changed: %+v", ctr.Content)
		}
	})

	t.Run("arg_error_passes_through", func(t *testing.T) {
		errVal := argErrf(classOutOfRange, HintTooLong, "summary", "summary too large: %d bytes (max %d)", 700, 512)
		ctr := &mcp.CallToolResult{}
		ctr.SetError(errVal)
		next := func(context.Context, string, mcp.Request) (mcp.Result, error) { return ctr, nil }
		res, err := mw(next)(context.Background(), "tools/call", &mcp.CallToolRequest{})
		if err != nil {
			t.Errorf("err = %v, want nil", err)
		}
		got := res.(*mcp.CallToolResult)
		if len(got.Content) != 1 {
			t.Fatalf("content = %+v, want 1 item", got.Content)
		}
		tc, ok := got.Content[0].(*mcp.TextContent)
		if !ok || tc.Text != errVal.Error() {
			t.Errorf("content text = %+v, want byte-identical to errVal.Error() = %q", got.Content, errVal.Error())
		}
	})

	t.Run("not_found_passes_through", func(t *testing.T) {
		ctr := &mcp.CallToolResult{}
		ctr.SetError(fmt.Errorf("%w: some-id", store.ErrNotFound))
		wantText := ctr.Content[0].(*mcp.TextContent).Text
		next := func(context.Context, string, mcp.Request) (mcp.Result, error) { return ctr, nil }
		res, err := mw(next)(context.Background(), "tools/call", &mcp.CallToolRequest{})
		if err != nil {
			t.Errorf("err = %v, want nil", err)
		}
		got := res.(*mcp.CallToolResult)
		if gotText := got.Content[0].(*mcp.TextContent).Text; gotText != wantText {
			t.Errorf("content text changed: got %q, want %q", gotText, wantText)
		}
	})

	t.Run("unclassified_resource_exhausted_passes_through", func(t *testing.T) {
		ctr := &mcp.CallToolResult{}
		ctr.SetError(status.Error(grpccodes.ResourceExhausted, "Too many requests"))
		wantText := ctr.Content[0].(*mcp.TextContent).Text
		next := func(context.Context, string, mcp.Request) (mcp.Result, error) { return ctr, nil }
		res, err := mw(next)(context.Background(), "tools/call", &mcp.CallToolRequest{})
		if err != nil {
			t.Errorf("err = %v, want nil", err)
		}
		got := res.(*mcp.CallToolResult)
		if gotText := got.Content[0].(*mcp.TextContent).Text; gotText != wantText {
			t.Errorf("content text changed: got %q, want %q (an unclassified server-side ResourceExhausted must not be relabeled)", gotText, wantText)
		}
	})

	t.Run("go_error_from_next_passes_through", func(t *testing.T) {
		someErr := errors.New("boom")
		next := func(context.Context, string, mcp.Request) (mcp.Result, error) { return nil, someErr }
		res, err := mw(next)(context.Background(), "tools/call", &mcp.CallToolRequest{})
		if res != nil {
			t.Errorf("res = %v, want nil", res)
		}
		if !errors.Is(err, someErr) {
			t.Errorf("err = %v, want %v", err, someErr)
		}
	})

	t.Run("response_too_large_maps", func(t *testing.T) {
		ctr := &mcp.CallToolResult{}
		ctr.SetError(fmt.Errorf("list: %w", &store.ResponseTooLargeError{Method: "/m"}))
		next := func(context.Context, string, mcp.Request) (mcp.Result, error) { return ctr, nil }
		res, err := mw(next)(context.Background(), "tools/call", &mcp.CallToolRequest{})
		if err != nil {
			t.Errorf("err = %v, want nil", err)
		}
		got := res.(*mcp.CallToolResult)
		if !got.IsError {
			t.Errorf("IsError = false, want true")
		}
		if len(got.Content) != 1 {
			t.Fatalf("content = %+v, want 1 item", got.Content)
		}
		tc, ok := got.Content[0].(*mcp.TextContent)
		if !ok || tc.Text != responseTooLargeEnvelope() {
			t.Errorf("content text = %+v, want %q", got.Content, responseTooLargeEnvelope())
		}
	})
}

// TestResponseTooLargeEnvelopeShape pins the envelope's wording contract
// (D-04): it starts with the field/hint prefix, carries no ASCII digit and
// no "later"/"again" wording (never steers the caller to retry the
// identical request — the ceiling it hit does not change between
// requests), and names every remedy (limit, k, full).
func TestResponseTooLargeEnvelopeShape(t *testing.T) {
	env := responseTooLargeEnvelope()
	if !strings.HasPrefix(env, "field=response hint=response_too_large: ") {
		t.Errorf("envelope %q does not start with the field/hint prefix", env)
	}
	if !noASCIIDigit(env) {
		t.Errorf("envelope %q contains an ASCII digit (a byte ceiling must never reach the wire)", env)
	}
	for _, banned := range []string{"later", "again"} {
		if strings.Contains(env, banned) {
			t.Errorf("envelope %q contains banned wording %q — never steer the caller toward a retry that cannot work", env, banned)
		}
	}
	for _, want := range []string{"limit", "k", "full"} {
		if !strings.Contains(env, want) {
			t.Errorf("envelope %q does not name remedy %q", env, want)
		}
	}
}
