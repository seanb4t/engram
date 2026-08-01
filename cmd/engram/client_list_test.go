// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"connectrpc.com/connect"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

func TestClientListEndToEndJSON(t *testing.T) {
	resetClientFlags(t)
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
		"--limit", "10",
		"--offset", "5",
		"--categories", "decision,gotcha",
		"--visibility", "shared",
		"--tags", "foo,bar",
		"--full",
		"--created-after", "2026-01-01T00:00:00Z",
		"--created-before", "2026-12-31T00:00:00Z",
		"--page-token", "opaque-token",
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
