// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

func TestClientSearchEndToEndJSON(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	svc := &stubEngramService{
		searchFn: func(_ context.Context, req *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{
				Memories: []*engramv1.Memory{
					{ShortId: "AAAA111111", Scope: req.GetScope()},
					{ShortId: "BBBB222222", Scope: req.GetScope()},
				},
			}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, stderr, err := runClient(t, "search",
		"--server", url, "--query", "q", "--scope", "repo:x", "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}

	var out struct {
		Memories []struct {
			ShortID string `json:"short_id"`
			Scope   string `json:"scope"`
		} `json:"memories"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout did not unmarshal as a single JSON object: %v\nstdout=%q", err, stdout)
	}
	if len(out.Memories) != 2 {
		t.Fatalf("len(memories) = %d, want 2", len(out.Memories))
	}
	if out.Memories[0].ShortID != "AAAA111111" || out.Memories[0].Scope != "repo:x" {
		t.Errorf("memories[0] = %+v, want short_id=AAAA111111 scope=repo:x", out.Memories[0])
	}
}

func TestClientSearchSendsBearerHeader(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	svc := &stubEngramService{
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{}, nil
		},
	}
	url := startStubServer(t, svc)
	t.Setenv("ENGRAM_TOKEN", "sentinel-token-value")

	_, _, err := runClient(t, "search", "--server", url, "--query", "q", "--scope", "repo:x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.lastAuthHeader != "Bearer sentinel-token-value" {
		t.Errorf("Authorization header = %q, want %q", svc.lastAuthHeader, "Bearer sentinel-token-value")
	}
}

func TestClientSearchNoTokenSendsNoAuthHeader(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	svc := &stubEngramService{
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{}, nil
		},
	}
	url := startStubServer(t, svc)
	t.Setenv("ENGRAM_TOKEN", "")

	_, _, err := runClient(t, "search", "--server", url, "--query", "q", "--scope", "repo:x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.lastAuthHeader != "" {
		t.Errorf("Authorization header = %q, want empty (anonymous call)", svc.lastAuthHeader)
	}
}

func TestClientSearchTokenFromFile(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	t.Setenv("ENGRAM_TOKEN", "")
	svc := &stubEngramService{
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{}, nil
		},
	}
	url := startStubServer(t, svc)

	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("file-token-value\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, _, err := runClient(t, "search", "--server", url, "--query", "q", "--scope", "repo:x", "--token-file", path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.lastAuthHeader != "Bearer file-token-value" {
		t.Errorf("Authorization header = %q, want %q", svc.lastAuthHeader, "Bearer file-token-value")
	}
}

func TestClientSearchEnvBeatsTokenFile(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	svc := &stubEngramService{
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{}, nil
		},
	}
	url := startStubServer(t, svc)
	t.Setenv("ENGRAM_TOKEN", "env-wins")

	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("file-loses\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, _, err := runClient(t, "search", "--server", url, "--query", "q", "--scope", "repo:x", "--token-file", path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.lastAuthHeader != "Bearer env-wins" {
		t.Errorf("Authorization header = %q, want %q", svc.lastAuthHeader, "Bearer env-wins")
	}
}

func TestClientSearchEmptyResultIsEmptyArray(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	svc := &stubEngramService{
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, _, err := runClient(t, "search", "--server", url, "--query", "q", "--scope", "repo:x", "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, `"memories":[]`) {
		t.Errorf("stdout = %q, want it to contain %q (a nil Go slice marshals to null, not [])", stdout, `"memories":[]`)
	}
}

func TestClientSearchMissingServerURLIsUsageError(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	t.Setenv("ENGRAM_SERVER_URL", "")
	svc := &stubEngramService{
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{}, nil
		},
	}
	// The stub server is started but --server is never passed to it, so a
	// non-zero call count would prove a network call was attempted despite
	// the missing URL. --scope is load-bearing here too: once the D-01/D-04
	// scope guard lands, an omitted --scope would make this test keep
	// asserting exit 2 while actually exercising the scope guard instead of
	// the missing-server check it was written for.
	startStubServer(t, svc)

	_, _, err := runClient(t, "search", "--query", "q", "--scope", "repo:x")
	if err == nil {
		t.Fatal("expected an error for a missing --server/ENGRAM_SERVER_URL")
	}
	var ec interface{ ExitCode() int }
	if !errors.As(err, &ec) {
		t.Fatalf("error %v does not carry ExitCode()", err)
	}
	if ec.ExitCode() != exitUsage {
		t.Errorf("ExitCode() = %d, want %d", ec.ExitCode(), exitUsage)
	}
	if svc.searchCalls != 0 {
		t.Errorf("searchCalls = %d, want 0 (no call should be attempted)", svc.searchCalls)
	}
}

func TestClientSearchMissingQueryIsUsageError(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	svc := &stubEngramService{
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{}, nil
		},
	}
	url := startStubServer(t, svc)

	_, _, err := runClient(t, "search", "--server", url, "--query", "", "--scope", "repo:x")
	if err == nil {
		t.Fatal("expected an error for an empty --query")
	}
	var ec interface{ ExitCode() int }
	if !errors.As(err, &ec) {
		t.Fatalf("error %v does not carry ExitCode()", err)
	}
	if ec.ExitCode() != exitUsage {
		t.Errorf("ExitCode() = %d, want %d", ec.ExitCode(), exitUsage)
	}
	if svc.searchCalls != 0 {
		t.Errorf("searchCalls = %d, want 0", svc.searchCalls)
	}
}

func TestClientSearchExitCodeAuth(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	svc := &stubEngramService{
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("nope"))
		},
	}
	url := startStubServer(t, svc)

	_, _, err := runClient(t, "search", "--server", url, "--query", "q", "--scope", "repo:x")
	assertExitCode(t, err, exitAuth)
}

func TestClientSearchExitCodeNotFound(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	svc := &stubEngramService{
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("nope"))
		},
	}
	url := startStubServer(t, svc)

	_, _, err := runClient(t, "search", "--server", url, "--query", "q", "--scope", "repo:x")
	assertExitCode(t, err, exitNotFound)
}

func TestClientSearchExitCodeInvalidArgument(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	svc := &stubEngramService{
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("nope"))
		},
	}
	url := startStubServer(t, svc)

	// --scope is load-bearing here too: once the D-01/D-04 scope guard
	// lands, an omitted --scope would make this test keep asserting exit 2
	// while actually exercising the scope guard instead of the
	// CodeInvalidArgument mapping it was written for.
	_, _, err := runClient(t, "search", "--server", url, "--query", "q", "--scope", "repo:x")
	assertExitCode(t, err, exitUsage)
}

// TestClientSearchExitCodeTransport points --server at a closed port and
// asserts the error the real dial failure produces maps to exit 5. This is
// driven through a genuine transport failure, not a synthesized error
// handed to the mapper — the real path never hands the mapper a raw
// non-*connect.Error, so a synthesized test would pass against a mapper
// that is wrong for production (02-RESEARCH.md Pitfall 5).
func TestClientSearchExitCodeTransport(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	t.Setenv("ENGRAM_TOKEN", "")

	_, _, err := runClient(t, "search", "--server", "http://127.0.0.1:1", "--query", "q", "--scope", "repo:x")
	assertExitCode(t, err, exitUnavailable)
}

func TestClientSearchTextOutputIsNotJSON(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	svc := &stubEngramService{
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{
				Memories: []*engramv1.Memory{{ShortId: "AAAA111111", Scope: "repo:x"}},
			}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, _, err := runClient(t, "search", "--server", url, "--query", "q", "--scope", "repo:x", "--output", "text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var js json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &js); err == nil {
		t.Errorf("stdout unmarshalled as JSON, want plain text: %q", stdout)
	}
	if !strings.Contains(stdout, "AAAA111111") {
		t.Errorf("stdout = %q, want it to contain the short_id", stdout)
	}
}

// TestClientSearchCrossSpineEndToEnd is the phase's tracer slice: the
// --cross-spine flag reaches the wire request untouched (D-01), and the
// text-mode coverage footer reports a count only, never the scope names
// (D-05, T-07-02).
func TestClientSearchCrossSpineEndToEnd(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	var gotReq *engramv1.SearchMemoriesRequest
	svc := &stubEngramService{
		searchFn: func(_ context.Context, req *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			gotReq = req
			return &engramv1.SearchMemoriesResponse{
				Memories: []*engramv1.Memory{
					{ShortId: "AAAA111111"},
					{ShortId: "BBBB222222"},
				},
				SearchedScopes:  []string{"repo:a", "repo:b", "repo:c"},
				ScopesTruncated: false,
			}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, _, err := runClient(t, "search",
		"--server", url, "--query", "q", "--cross-spine", "--output", "text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.searchCalls != 1 {
		t.Fatalf("searchCalls = %d, want 1", svc.searchCalls)
	}
	if gotReq == nil {
		t.Fatal("stub never received a request")
	}
	if !gotReq.GetCrossSpine() {
		t.Error("request CrossSpine = false, want true")
	}
	if gotReq.GetScope() != "" {
		t.Errorf("request Scope = %q, want empty", gotReq.GetScope())
	}
	if !strings.Contains(stdout, "searched_scopes: 3") {
		t.Errorf("stdout = %q, want a coverage footer reporting count 3", stdout)
	}
	for _, name := range []string{"repo:a", "repo:b", "repo:c"} {
		if strings.Contains(stdout, name) {
			t.Errorf("stdout = %q, must not name scope %q (D-05: count only)", stdout, name)
		}
	}
}

// TestClientSearchMissingScopeIsUsageErrorBeforeDialing pins D-01: with
// neither --scope nor --cross-spine, the guard fires before any network
// call.
func TestClientSearchMissingScopeIsUsageErrorBeforeDialing(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	svc := &stubEngramService{
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{}, nil
		},
	}
	url := startStubServer(t, svc)

	_, _, err := runClient(t, "search", "--server", url, "--query", "q")
	assertExitCode(t, err, exitUsage)
	if svc.searchCalls != 0 {
		t.Errorf("searchCalls = %d, want 0 (guard must fire before dialing)", svc.searchCalls)
	}
}

// TestClientSearchScopeWithCrossSpineIsUsageErrorBeforeDialing pins D-04:
// --scope together with --cross-spine is rejected client-side before
// dialing, never silently discarding the scope the way the server does.
func TestClientSearchScopeWithCrossSpineIsUsageErrorBeforeDialing(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	svc := &stubEngramService{
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{}, nil
		},
	}
	url := startStubServer(t, svc)

	_, _, err := runClient(t, "search", "--server", url, "--query", "q", "--scope", "repo:x", "--cross-spine")
	assertExitCode(t, err, exitUsage)
	if svc.searchCalls != 0 {
		t.Errorf("searchCalls = %d, want 0 (guard must fire before dialing)", svc.searchCalls)
	}
}

// TestClientSearchNoFooterWithoutCrossSpine is the D-06 measured baseline:
// a scope-confined text-mode call is byte-identical to what
// renderMemoryTable alone produces — no trailing footer line — even when
// the stub populates the provenance fields, proving the footer is gated on
// the caller's own flag, not on what the server happened to return.
func TestClientSearchNoFooterWithoutCrossSpine(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	mems := []*engramv1.Memory{
		{ShortId: "AAAA111111", Scope: "repo:x"},
		{ShortId: "BBBB222222", Scope: "repo:x"},
	}
	svc := &stubEngramService{
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{
				Memories:        mems,
				SearchedScopes:  []string{"repo:x"},
				ScopesTruncated: true,
			}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, _, err := runClient(t, "search", "--server", url, "--query", "q", "--scope", "repo:x", "--output", "text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var want strings.Builder
	if err := renderMemoryTable(&want, mems, true); err != nil {
		t.Fatalf("renderMemoryTable: %v", err)
	}
	if stdout != want.String() {
		t.Errorf("stdout = %q, want exactly %q (no footer line without --cross-spine)", stdout, want.String())
	}
}

// assertExitCode is a shared helper for the exit-code-mapping tests above.
func assertExitCode(t *testing.T, err error, want int) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a non-nil error")
	}
	var ec interface{ ExitCode() int }
	if !errors.As(err, &ec) {
		t.Fatalf("error %v does not carry ExitCode()", err)
	}
	if got := ec.ExitCode(); got != want {
		t.Errorf("ExitCode() = %d, want %d", got, want)
	}
}
