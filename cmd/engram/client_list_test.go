// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

func TestClientListEndToEndJSON(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, listCmd)
	svc := &stubEngramService{
		listFn: func(_ context.Context, req *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			return &engramv1.ListMemoriesResponse{
				Memories: []*engramv1.Memory{
					{ShortId: "AAAA111111", Scope: req.GetScope()},
					{ShortId: "BBBB222222", Scope: req.GetScope()},
					{ShortId: "CCCC333333", Scope: req.GetScope()},
				},
				Total:         42,
				NextPageToken: "cursor-xyz",
			}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, stderr, err := runClient(t, "list",
		"--server", url, "--scope", "repo:x", "--output", "json")
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
		Total         string `json:"total"` // protojson renders uint64 as a JSON string
		NextPageToken string `json:"next_page_token"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout did not unmarshal as a single JSON object: %v\nstdout=%q", err, stdout)
	}
	if len(out.Memories) != 3 {
		t.Fatalf("len(memories) = %d, want 3", len(out.Memories))
	}
	if out.Total != "42" {
		t.Errorf("total = %q, want %q", out.Total, "42")
	}
	if out.NextPageToken != "cursor-xyz" {
		t.Errorf("next_page_token = %q, want %q", out.NextPageToken, "cursor-xyz")
	}
}

func TestClientListPassesFiltersToRequest(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, listCmd)
	var captured *engramv1.ListMemoriesRequest
	svc := &stubEngramService{
		listFn: func(_ context.Context, req *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			captured = req
			return &engramv1.ListMemoriesResponse{}, nil
		},
	}
	url := startStubServer(t, svc)

	// --offset and --page-token are mutually exclusive (D-07/D-08), so this
	// test exercises --offset; TestClientListPageTokenReachesRequest below
	// exercises --page-token on its own.
	_, _, err := runClient(t, "list",
		"--server", url,
		"--scope", "repo:x",
		"--limit", "10",
		"--offset", "5",
		"--categories", "decision,gotcha",
		"--visibility", "shared",
		"--tags", "foo,bar",
		"--full",
		"--created-after", "2026-01-01T00:00:00Z",
		"--created-before", "2026-12-31T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("ListMemories was never called")
	}
	if captured.GetScope() != "repo:x" {
		t.Errorf("Scope = %q, want %q", captured.GetScope(), "repo:x")
	}
	if captured.GetLimit() != 10 {
		t.Errorf("Limit = %d, want 10", captured.GetLimit())
	}
	if captured.GetOffset() != 5 {
		t.Errorf("Offset = %d, want 5", captured.GetOffset())
	}
	if got, want := captured.GetCategories(), []string{"decision", "gotcha"}; !equalStrSlices(got, want) {
		t.Errorf("Categories = %v, want %v", got, want)
	}
	if captured.GetVisibility() != "shared" {
		t.Errorf("Visibility = %q, want %q", captured.GetVisibility(), "shared")
	}
	if got, want := captured.GetTags(), []string{"foo", "bar"}; !equalStrSlices(got, want) {
		t.Errorf("Tags = %v, want %v", got, want)
	}
	if !captured.GetFull() {
		t.Error("Full = false, want true")
	}
	if captured.GetCreatedAfter() != "2026-01-01T00:00:00Z" {
		t.Errorf("CreatedAfter = %q, want %q", captured.GetCreatedAfter(), "2026-01-01T00:00:00Z")
	}
	if captured.GetCreatedBefore() != "2026-12-31T00:00:00Z" {
		t.Errorf("CreatedBefore = %q, want %q", captured.GetCreatedBefore(), "2026-12-31T00:00:00Z")
	}
}

// TestClientListPageTokenReachesRequest pins --page-token reaching the wire
// request on its own, now that D-08 makes it mutually exclusive with
// --offset and --cursor-mode (split out of TestClientListPassesFiltersToRequest).
func TestClientListPageTokenReachesRequest(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, listCmd)
	var captured *engramv1.ListMemoriesRequest
	svc := &stubEngramService{
		listFn: func(_ context.Context, req *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			captured = req
			return &engramv1.ListMemoriesResponse{}, nil
		},
	}
	url := startStubServer(t, svc)

	_, _, err := runClient(t, "list",
		"--server", url,
		"--scope", "repo:x",
		"--page-token", "opaque-token",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("ListMemories was never called")
	}
	if captured.GetPageToken() != "opaque-token" {
		t.Errorf("PageToken = %q, want %q", captured.GetPageToken(), "opaque-token")
	}
}

func equalStrSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestClientListCursorModeReachesRequest(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, listCmd)
	var captured *engramv1.ListMemoriesRequest
	svc := &stubEngramService{
		listFn: func(_ context.Context, req *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			captured = req
			return &engramv1.ListMemoriesResponse{}, nil
		},
	}
	url := startStubServer(t, svc)

	_, _, err := runClient(t, "list", "--server", url, "--scope", "repo:x", "--cursor-mode")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("ListMemories was never called")
	}
	if !captured.GetCursorMode() {
		t.Error("CursorMode = false, want true")
	}
}

func TestClientListEmptyResultIsEmptyArray(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, listCmd)
	svc := &stubEngramService{
		listFn: func(context.Context, *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			return &engramv1.ListMemoriesResponse{}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, _, err := runClient(t, "list", "--server", url, "--scope", "repo:x", "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, `"memories":[]`) {
		t.Errorf("stdout = %q, want it to contain %q (a nil Go slice marshals to null, not [])", stdout, `"memories":[]`)
	}
}

func TestClientListExitCodes(t *testing.T) {
	cases := []struct {
		name string
		code connect.Code
		want int
	}{
		{"unauthenticated", connect.CodeUnauthenticated, exitAuth},
		{"notfound", connect.CodeNotFound, exitNotFound},
		{"invalidargument", connect.CodeInvalidArgument, exitUsage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetClientFlags(t)
	resetCommandFlagState(t, listCmd)
			svc := &stubEngramService{
				listFn: func(context.Context, *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
					return nil, connect.NewError(tc.code, errUnimplementedStub)
				},
			}
			url := startStubServer(t, svc)

			_, _, err := runClient(t, "list", "--server", url, "--scope", "repo:x")
			assertExitCode(t, err, tc.want)
		})
	}
}

func TestClientListMissingServerURLIsUsageError(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, listCmd)
	t.Setenv("ENGRAM_SERVER_URL", "")
	svc := &stubEngramService{
		listFn: func(context.Context, *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			return &engramv1.ListMemoriesResponse{}, nil
		},
	}
	// The stub server is started but --server is never passed to it, so a
	// non-zero call count would prove a network call was attempted despite
	// the missing URL.
	startStubServer(t, svc)

	_, _, err := runClient(t, "list", "--scope", "repo:x")
	assertExitCode(t, err, exitUsage)
	if svc.listCalls != 0 {
		t.Errorf("listCalls = %d, want 0 (no call should be attempted)", svc.listCalls)
	}
}

func TestClientListNoDeprecatedApproximateFlag(t *testing.T) {
	if listCmd.Flags().Lookup("approximate") != nil {
		t.Error(`listCmd declares an "approximate" flag; the proto marks the response field deprecated and always false`)
	}

	resetClientFlags(t)
	resetCommandFlagState(t, listCmd)
	svc := &stubEngramService{
		listFn: func(context.Context, *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			return &engramv1.ListMemoriesResponse{
				Memories: []*engramv1.Memory{{ShortId: "AAAA111111", Scope: "repo:x"}},
			}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, _, err := runClient(t, "list", "--server", url, "--scope", "repo:x", "--output", "text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(strings.ToUpper(stdout), "APPROXIMATE") {
		t.Errorf("text output header contains APPROXIMATE: %q", stdout)
	}
}

func TestClientListTextOutput(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, listCmd)
	svc := &stubEngramService{
		listFn: func(context.Context, *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			return &engramv1.ListMemoriesResponse{
				Memories: []*engramv1.Memory{{ShortId: "AAAA111111", Scope: "repo:x"}},
				Total:    1,
			}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, _, err := runClient(t, "list", "--server", url, "--scope", "repo:x", "--output", "text")
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

// TestClientListCrossSpineEndToEnd mirrors
// TestClientSearchCrossSpineEndToEnd: --cross-spine reaches the wire request
// untouched (D-01), and the text-mode coverage footer reports a count only,
// never the scope names (D-05), printed after the existing total line.
func TestClientListCrossSpineEndToEnd(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, listCmd)
	var gotReq *engramv1.ListMemoriesRequest
	svc := &stubEngramService{
		listFn: func(_ context.Context, req *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			gotReq = req
			return &engramv1.ListMemoriesResponse{
				Memories: []*engramv1.Memory{
					{ShortId: "AAAA111111"},
					{ShortId: "BBBB222222"},
				},
				Total:           2,
				SearchedScopes:  []string{"repo:a", "repo:b", "repo:c"},
				ScopesTruncated: false,
			}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, _, err := runClient(t, "list",
		"--server", url, "--cross-spine", "--output", "text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.listCalls != 1 {
		t.Fatalf("listCalls = %d, want 1", svc.listCalls)
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
	totalIdx := strings.Index(stdout, "total: 2")
	footerIdx := strings.Index(stdout, "searched_scopes: 3")
	if totalIdx < 0 {
		t.Errorf("stdout = %q, want the existing total line", stdout)
	}
	if footerIdx < 0 {
		t.Errorf("stdout = %q, want a coverage footer reporting count 3", stdout)
	}
	if totalIdx >= 0 && footerIdx >= 0 && footerIdx < totalIdx {
		t.Errorf("stdout = %q, want the coverage footer to appear after the total line", stdout)
	}
	for _, name := range []string{"repo:a", "repo:b", "repo:c"} {
		if strings.Contains(stdout, name) {
			t.Errorf("stdout = %q, must not name scope %q (D-05: count only)", stdout, name)
		}
	}
}

// TestClientListMissingScopeIsUsageErrorBeforeDialing pins D-01: with
// neither --scope nor --cross-spine, the guard fires before any network
// call.
func TestClientListMissingScopeIsUsageErrorBeforeDialing(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, listCmd)
	svc := &stubEngramService{
		listFn: func(context.Context, *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			return &engramv1.ListMemoriesResponse{}, nil
		},
	}
	url := startStubServer(t, svc)

	_, _, err := runClient(t, "list", "--server", url)
	assertExitCode(t, err, exitUsage)
	if svc.listCalls != 0 {
		t.Errorf("listCalls = %d, want 0 (guard must fire before dialing)", svc.listCalls)
	}
}

// TestClientListScopeWithCrossSpineIsUsageErrorBeforeDialing pins D-04:
// --scope together with --cross-spine is rejected client-side before
// dialing, never silently discarding the scope the way the server does.
func TestClientListScopeWithCrossSpineIsUsageErrorBeforeDialing(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, listCmd)
	svc := &stubEngramService{
		listFn: func(context.Context, *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			return &engramv1.ListMemoriesResponse{}, nil
		},
	}
	url := startStubServer(t, svc)

	_, _, err := runClient(t, "list", "--server", url, "--scope", "repo:x", "--cross-spine")
	assertExitCode(t, err, exitUsage)
	if svc.listCalls != 0 {
		t.Errorf("listCalls = %d, want 0 (guard must fire before dialing)", svc.listCalls)
	}
}

// TestClientListFooterUnchangedWithoutCrossSpine is the D-06 measured
// baseline for list: a scope-confined text-mode call is byte-identical to
// the pre-phase output — table, then the single total line, no third
// line — even when the stub populates the provenance fields, proving the
// footer is gated on the caller's own flag, not on what the server happened
// to return.
func TestClientListFooterUnchangedWithoutCrossSpine(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, listCmd)
	mems := []*engramv1.Memory{
		{ShortId: "AAAA111111", Scope: "repo:x"},
		{ShortId: "BBBB222222", Scope: "repo:x"},
	}
	svc := &stubEngramService{
		listFn: func(context.Context, *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			return &engramv1.ListMemoriesResponse{
				Memories:        mems,
				Total:           2,
				SearchedScopes:  []string{"repo:x"},
				ScopesTruncated: true,
			}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, _, err := runClient(t, "list", "--server", url, "--scope", "repo:x", "--output", "text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var want strings.Builder
	if err := renderMemoryTable(&want, mems, false); err != nil {
		t.Fatalf("renderMemoryTable: %v", err)
	}
	fmt.Fprintf(&want, "total: %d\n", 2)
	if stdout != want.String() {
		t.Errorf("stdout = %q, want exactly %q (no coverage footer line without --cross-spine)", stdout, want.String())
	}
}
