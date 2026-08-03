// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
)

// errUnimplementedStub is returned by stubEngramService's RPC methods when
// the corresponding func field is unset.
var errUnimplementedStub = errors.New("stub RPC not configured")

// stubEngramService is an in-process, real Connect handler standing in for
// the engram server. It embeds UnimplementedEngramServiceHandler so it only
// needs to override the three RPCs this phase surfaces, and it records the
// Authorization header value and a per-RPC call counter for tests to assert
// on.
type stubEngramService struct {
	engramv1connect.UnimplementedEngramServiceHandler

	searchFn func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error)
	listFn   func(context.Context, *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error)
	storeFn  func(context.Context, *engramv1.StoreMemoryRequest) (*engramv1.StoreMemoryResponse, error)

	searchCalls int
	listCalls   int
	storeCalls  int

	// lastAuthHeader is the Authorization header value observed on the most
	// recent request to any of the three overridden RPCs.
	lastAuthHeader string
}

func (s *stubEngramService) SearchMemories(ctx context.Context, req *connect.Request[engramv1.SearchMemoriesRequest]) (*connect.Response[engramv1.SearchMemoriesResponse], error) {
	s.searchCalls++
	s.lastAuthHeader = req.Header().Get("Authorization")
	if s.searchFn == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errUnimplementedStub)
	}
	resp, err := s.searchFn(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (s *stubEngramService) ListMemories(ctx context.Context, req *connect.Request[engramv1.ListMemoriesRequest]) (*connect.Response[engramv1.ListMemoriesResponse], error) {
	s.listCalls++
	s.lastAuthHeader = req.Header().Get("Authorization")
	if s.listFn == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errUnimplementedStub)
	}
	resp, err := s.listFn(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (s *stubEngramService) StoreMemory(ctx context.Context, req *connect.Request[engramv1.StoreMemoryRequest]) (*connect.Response[engramv1.StoreMemoryResponse], error) {
	s.storeCalls++
	s.lastAuthHeader = req.Header().Get("Authorization")
	if s.storeFn == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errUnimplementedStub)
	}
	resp, err := s.storeFn(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// startStubServer mounts svc's real generated Connect handler on an
// httptest.Server and returns its base URL. This is the real generated
// Connect handler over the real Connect wire protocol and codec — not a
// hand-written HTTP stub — so the client's own error wrapping, codec, and
// header handling are all exercised.
func startStubServer(t *testing.T, svc engramv1connect.EngramServiceHandler) string {
	t.Helper()
	mux := http.NewServeMux()
	path, handler := engramv1connect.NewEngramServiceHandler(svc)
	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

// resetClientFlags restores every package-level client flag var to its zero
// value via t.Cleanup. The whole package shares one rootCmd and one set of
// flag vars across the test binary, and a leaked --insecure or --output
// would silently contaminate a later test.
func resetClientFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		clientServerURL = ""
		clientTokenFile = ""
		clientInsecure = false
		clientOutput = ""

		// The StringSliceVar-backed vars MUST be reset too (REVIEW.md CR-01).
		// pflag's stringSliceValue.Set APPENDS once its internal `changed`
		// bool has latched true, and cobra's commands are package-level vars
		// shared by every test in this binary — so without this, a second
		// invocation of the same command accumulates: --tags foo,bar twice
		// yields [foo bar foo bar]. That is dormant under `task test`
		// (-count=1, no shuffle) but turns real under -count=2 or
		// -shuffle=on, which is exactly when a suite is being trusted most.
		//
		// Resetting the Go var is sufficient: pflag holds a pointer to it,
		// so the latched append targets the freshly-nil slice. `changed`
		// itself is unexported and cannot be cleared from here.
		storeTags = nil
		searchTags = nil
		searchCategories = nil
		searchScope = ""
		searchCrossSpine = false
		listTags = nil
		listCategories = nil
		listScope = ""
		listCrossSpine = false
	})
}

// resetCommandFlagState clears pflag's Changed latch for every flag on cmd
// AND on the root command's persistent flags, via t.Cleanup. It is the
// complement of resetClientFlags, not a replacement: resetClientFlags resets
// the Go variables a flag writes into; this resets pflag's own bookkeeping
// about whether a flag was supplied on this invocation.
//
// Why it is required: every cobra command in this binary is a package-level
// var shared across the whole test binary, and pflag.Flag.Changed latches
// true the first time a flag is supplied and never clears itself. Every
// mechanism this phase introduces (ValidateFlagGroups,
// MarkFlagsMutuallyExclusive, MarkFlagsOneRequired, and koanf's
// changed-flag overlay in config.Load) branches on Changed, not on the
// flag's value. Without this reset, row N+1 in a table-driven test sees row
// N's supplied flags as still supplied and silently reports the wrong exit
// code.
//
// pflag.Flag.Changed is an exported struct field, so it can be cleared
// directly from this package. f.Value.Set(f.DefValue)'s error is ignored:
// for a StringSliceVar-backed flag, pflag's Set APPENDS rather than
// replacing once Changed has latched, so calling it here would not restore
// the zero value anyway — resetClientFlags already handles those by nilling
// the Go var directly, and this helper's job is limited to the Changed bit
// itself.
func resetCommandFlagState(t *testing.T, cmd *cobra.Command) {
	t.Helper()
	doReset := func() {
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			f.Changed = false
			_ = f.Value.Set(f.DefValue)
		})
		cmd.Root().PersistentFlags().VisitAll(func(f *pflag.Flag) {
			f.Changed = false
			_ = f.Value.Set(f.DefValue)
		})
	}
	doReset()
	t.Cleanup(doReset)
}

// runClient sets rootCmd's output/error writers to two independent buffers,
// runs it against args, and returns the captured stdout, stderr, and the
// error rootCmd.Execute() returned.
//
// args must be a non-nil slice: passing nil here is a trap, because cobra
// then falls back to os.Args[1:], which under `go test` is the test
// binary's own flags (e.g. -test.run=...), not the intended command line.
func runClient(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs(args)
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})
	err = rootCmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

// TestResetCommandFlagState proves the latch is actually cleared: run list
// once with --offset 1 (against a server that refuses the connection, so the
// command still returns quickly), call the helper, then assert
// listCmd.Flags().Changed("offset") is false.
func TestResetCommandFlagState(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, listCmd)

	_, _, _ = runClient(t, "list", "--server", "http://127.0.0.1:1", "--scope", "s", "--offset", "1")

	if !listCmd.Flags().Changed("offset") {
		t.Fatal("precondition failed: --offset should be Changed immediately after being supplied")
	}

	resetCommandFlagState(t, listCmd)

	if listCmd.Flags().Changed("offset") {
		t.Error(`listCmd.Flags().Changed("offset") = true after resetCommandFlagState, want false`)
	}
}
