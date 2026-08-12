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

func TestClientStoreEndToEndJSON(t *testing.T) {
	resetClientFlags(t)
	svc := &stubEngramService{
		storeFn: func(context.Context, *engramv1.StoreMemoryRequest) (*engramv1.StoreMemoryResponse, error) {
			return &engramv1.StoreMemoryResponse{Id: "full-id-value", ShortId: "AAAA111111"}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, stderr, err := runClient(t, "store",
		"--server", url, "--content", "c", "--scope", "repo:x", "--category", "decision", "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}

	var out struct {
		ID      string `json:"id"`
		ShortID string `json:"short_id"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout did not unmarshal as a single JSON object: %v\nstdout=%q", err, stdout)
	}
	if out.ID != "full-id-value" {
		t.Errorf("id = %q, want %q", out.ID, "full-id-value")
	}
	if out.ShortID != "AAAA111111" {
		t.Errorf("short_id = %q, want %q", out.ShortID, "AAAA111111")
	}
}

func TestClientStorePassesFieldsToRequest(t *testing.T) {
	resetClientFlags(t)
	var captured *engramv1.StoreMemoryRequest
	svc := &stubEngramService{
		storeFn: func(_ context.Context, req *engramv1.StoreMemoryRequest) (*engramv1.StoreMemoryResponse, error) {
			captured = req
			return &engramv1.StoreMemoryResponse{Id: "id", ShortId: "short"}, nil
		},
	}
	url := startStubServer(t, svc)

	_, _, err := runClient(t, "store",
		"--server", url,
		"--content", "the content",
		"--scope", "repo:x",
		"--source", "cli",
		"--category", "gotcha",
		"--tags", "foo,bar",
		"--repo", "engram",
		"--workspace", "ws1",
		"--worktree", "wt1",
		"--base-dir", "/tmp/x",
		"--summary", "a summary",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("StoreMemory was never called")
	}
	if captured.GetContent() != "the content" {
		t.Errorf("Content = %q, want %q", captured.GetContent(), "the content")
	}
	if captured.GetScope() != "repo:x" {
		t.Errorf("Scope = %q, want %q", captured.GetScope(), "repo:x")
	}
	if captured.GetSource() != "cli" {
		t.Errorf("Source = %q, want %q", captured.GetSource(), "cli")
	}
	if captured.GetCategory() != "gotcha" {
		t.Errorf("Category = %q, want %q", captured.GetCategory(), "gotcha")
	}
	if got, want := captured.GetTags(), []string{"foo", "bar"}; !equalStrSlices(got, want) {
		t.Errorf("Tags = %v, want %v", got, want)
	}
	if captured.GetRepo() != "engram" {
		t.Errorf("Repo = %q, want %q", captured.GetRepo(), "engram")
	}
	if captured.GetWorkspace() != "ws1" {
		t.Errorf("Workspace = %q, want %q", captured.GetWorkspace(), "ws1")
	}
	if captured.GetWorktree() != "wt1" {
		t.Errorf("Worktree = %q, want %q", captured.GetWorktree(), "wt1")
	}
	if captured.GetBaseDir() != "/tmp/x" {
		t.Errorf("BaseDir = %q, want %q", captured.GetBaseDir(), "/tmp/x")
	}
	if captured.GetSummary() != "a summary" {
		t.Errorf("Summary = %q, want %q", captured.GetSummary(), "a summary")
	}
}

func TestClientStoreRequiresContentAndScope(t *testing.T) {
	t.Run("empty content", func(t *testing.T) {
		resetClientFlags(t)
		svc := &stubEngramService{
			storeFn: func(context.Context, *engramv1.StoreMemoryRequest) (*engramv1.StoreMemoryResponse, error) {
				return &engramv1.StoreMemoryResponse{}, nil
			},
		}
		url := startStubServer(t, svc)

		_, _, err := runClient(t, "store", "--server", url, "--content", "", "--scope", "repo:x")
		assertExitCode(t, err, exitUsage)
		if svc.storeCalls != 0 {
			t.Errorf("storeCalls = %d, want 0", svc.storeCalls)
		}
	})

	t.Run("empty scope", func(t *testing.T) {
		resetClientFlags(t)
		svc := &stubEngramService{
			storeFn: func(context.Context, *engramv1.StoreMemoryRequest) (*engramv1.StoreMemoryResponse, error) {
				return &engramv1.StoreMemoryResponse{}, nil
			},
		}
		url := startStubServer(t, svc)

		_, _, err := runClient(t, "store", "--server", url, "--content", "c", "--scope", "")
		assertExitCode(t, err, exitUsage)
		if svc.storeCalls != 0 {
			t.Errorf("storeCalls = %d, want 0", svc.storeCalls)
		}
	})
}

// TestClientStoreNeverRetries proves a failed write is attempted exactly
// once, for three distinct error classes — including the ambiguous ones
// (Unavailable, DeadlineExceeded) a well-meaning retry would target.
//
// The stub's storeCalls counter increments in stubEngramService.StoreMemory
// itself (clienttest_test.go), NOT inside storeFn — so it observes every
// attempt, not just a successful one. A counter that only the success path
// reached could not observe a retry no matter how many times the client
// retried.
func TestClientStoreNeverRetries(t *testing.T) {
	cases := []struct {
		name string
		code connect.Code
		want int
	}{
		{"unavailable", connect.CodeUnavailable, exitUnavailable},
		{"internal", connect.CodeInternal, exitGeneric},
		// D-06: a client-side deadline is now its own exitTimeout, distinct
		// from exitUnavailable — see TestExitCodeTimeoutDistinctFromUnavailable.
		{"deadlineexceeded", connect.CodeDeadlineExceeded, exitTimeout},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetClientFlags(t)
			svc := &stubEngramService{
				storeFn: func(context.Context, *engramv1.StoreMemoryRequest) (*engramv1.StoreMemoryResponse, error) {
					return nil, connect.NewError(tc.code, errUnimplementedStub)
				},
			}
			url := startStubServer(t, svc)

			_, _, err := runClient(t, "store", "--server", url, "--content", "c", "--scope", "repo:x")
			assertExitCode(t, err, tc.want)
			if svc.storeCalls != 1 {
				t.Errorf("storeCalls = %d, want exactly 1 (no retry)", svc.storeCalls)
			}
		})
	}
}

func TestClientStoreNoActorOrOwnerFlag(t *testing.T) {
	forbidden := []string{"actor", "owner", "id", "short_id", "score", "created_at", "access_count", "last_accessed_at"}
	for _, name := range forbidden {
		if storeCmd.Flags().Lookup(name) != nil {
			t.Errorf("storeCmd declares a %q flag; that field is response-only/server-set and must never be client-supplied", name)
		}
	}
}

func TestClientStoreCategoryHelpNamesLegalValues(t *testing.T) {
	f := storeCmd.Flags().Lookup("category")
	if f == nil {
		t.Fatal("storeCmd has no --category flag")
	}
	for _, v := range []string{"decision", "preference", "convention", "gotcha"} {
		if !strings.Contains(f.Usage, v) {
			t.Errorf("--category usage %q does not name legal value %q", f.Usage, v)
		}
	}
}

func TestClientStoreExitCodes(t *testing.T) {
	cases := []struct {
		name string
		code connect.Code
		want int
	}{
		{"invalidargument", connect.CodeInvalidArgument, exitUsage},
		{"unauthenticated", connect.CodeUnauthenticated, exitAuth},
		{"permissiondenied", connect.CodePermissionDenied, exitAuth},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetClientFlags(t)
			svc := &stubEngramService{
				storeFn: func(context.Context, *engramv1.StoreMemoryRequest) (*engramv1.StoreMemoryResponse, error) {
					return nil, connect.NewError(tc.code, errUnimplementedStub)
				},
			}
			url := startStubServer(t, svc)

			_, _, err := runClient(t, "store", "--server", url, "--content", "c", "--scope", "repo:x")
			assertExitCode(t, err, tc.want)
		})
	}
}

func TestClientStoreTextOutput(t *testing.T) {
	resetClientFlags(t)
	svc := &stubEngramService{
		storeFn: func(context.Context, *engramv1.StoreMemoryRequest) (*engramv1.StoreMemoryResponse, error) {
			return &engramv1.StoreMemoryResponse{Id: "full-id-value", ShortId: "AAAA111111"}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, _, err := runClient(t, "store", "--server", url, "--content", "c", "--scope", "repo:x", "--output", "text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var js json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &js); err == nil {
		t.Errorf("stdout unmarshalled as JSON, want plain text: %q", stdout)
	}
	if !strings.Contains(stdout, "AAAA111111") {
		t.Errorf("stdout = %q, want it to contain the returned short id", stdout)
	}
}
