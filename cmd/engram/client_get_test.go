// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// TestGetHeadline pins getHeadline directly, independent of the CLI
// plumbing: a record carrying only default state has no parenthetical, and
// an archived+superseded record carries both words, comma-space-joined, in
// canonical order — the same vocabulary memoryStateCell draws from.
func TestGetHeadline(t *testing.T) {
	now := time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("default state record has no parenthesis", func(t *testing.T) {
		m := &engramv1.Memory{ShortId: "LIVE111111"}
		got := getHeadline(m, now)
		if strings.ContainsAny(got, "()") {
			t.Errorf("getHeadline(default state) = %q, want no parenthesis character", got)
		}
		if !strings.Contains(got, "LIVE111111") {
			t.Errorf("getHeadline(default state) = %q, want it to contain the short id", got)
		}
	})

	t.Run("archived plus superseded record carries both words in canonical order", func(t *testing.T) {
		m := &engramv1.Memory{
			ShortId:      "ARCH222222",
			ArchivedAt:   timestamppb.New(now.Add(-time.Hour)),
			SupersededBy: proto.String("successor-id"),
		}
		got := getHeadline(m, now)
		want := "record ARCH222222 (archived, superseded)"
		if got != want {
			t.Errorf("getHeadline(archived+superseded) = %q, want %q", got, want)
		}
	})
}

// TestGetAcceptsUUIDOrShortID asserts GetMemoryRequest.Id carries args[0]
// UNCHANGED — the CLI adds no id parsing of its own, for either shape.
func TestGetAcceptsUUIDOrShortID(t *testing.T) {
	cases := []string{
		"018f6c2e-6b2b-7c9e-9c2e-6b2b7c9e9c2e", // uuid-shaped
		"ABCDEFGH12",                           // short_id-shaped
	}
	for _, id := range cases {
		t.Run(id, func(t *testing.T) {
			resetClientFlags(t)
			resetCommandFlagState(t, getCmd)
			var gotReq *engramv1.GetMemoryRequest
			svc := &stubEngramService{
				getFn: func(_ context.Context, req *engramv1.GetMemoryRequest) (*engramv1.GetMemoryResponse, error) {
					gotReq = req
					return &engramv1.GetMemoryResponse{Memory: &engramv1.Memory{ShortId: "RESOLVEDID"}}, nil
				},
			}
			url := startStubServer(t, svc)

			_, _, err := runClient(t, "get", id, "--server", url, "--output", "json")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotReq == nil {
				t.Fatal("stub never received a request")
			}
			if gotReq.GetId() != id {
				t.Errorf("request Id = %q, want %q (no CLI-side id parsing)", gotReq.GetId(), id)
			}
		})
	}
}

// TestGetTextOutputRendersHeadlineAndFieldTable covers the --output text
// behavior: a labelled field table beneath the headline.
func TestGetTextOutputRendersHeadlineAndFieldTable(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, getCmd)
	svc := &stubEngramService{
		getFn: func(context.Context, *engramv1.GetMemoryRequest) (*engramv1.GetMemoryResponse, error) {
			return &engramv1.GetMemoryResponse{Memory: &engramv1.Memory{
				ShortId: "LIVE111111",
				Scope:   "repo:x",
				Content: "hello world",
			}}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, _, err := runClient(t, "get", "LIVE111111", "--server", url, "--output", "text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(stdout, "record LIVE111111\n") {
		t.Errorf("stdout = %q, want it to start with the headline", stdout)
	}
	if !strings.Contains(stdout, "\n  Content") || !strings.Contains(stdout, "hello world") {
		t.Errorf("stdout = %q, want a Content field line carrying the record's content", stdout)
	}
}

// TestGetJSONOutputIsSingleLineDocument covers the --output json behavior
// (D-08): one JSON object per invocation, not NDJSON.
func TestGetJSONOutputIsSingleLineDocument(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, getCmd)
	svc := &stubEngramService{
		getFn: func(context.Context, *engramv1.GetMemoryRequest) (*engramv1.GetMemoryResponse, error) {
			return &engramv1.GetMemoryResponse{Memory: &engramv1.Memory{ShortId: "LIVE111111", Scope: "repo:x"}}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, _, err := runClient(t, "get", "LIVE111111", "--server", url, "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc); err != nil {
		t.Fatalf("stdout did not unmarshal as a single JSON object: %v\nstdout=%q", err, stdout)
	}
	if doc["short_id"] == nil {
		t.Errorf("json document missing short_id key: %q", stdout)
	}
	if got, want := strings.Count(stdout, "\n"), 1; got != want {
		t.Errorf("stdout has %d newlines, want exactly %d (single-line document plus trailing newline)", got, want)
	}
}

// TestGetOutputIdentity asserts the text lane's rendered label set and the
// json lane's key set correspond for the same record — including a record
// with superseded_by unset (the field is absent from BOTH lanes, since
// SupersededBy is `optional`) and a record with superseded_by set (present
// in both). Both lanes must serialize the same value with the same
// protojson options or this correspondence does not hold (T-07-16).
func TestGetOutputIdentity(t *testing.T) {
	now := timestamppb.Now()
	cases := []struct {
		name string
		mem  *engramv1.Memory
	}{
		{
			name: "superseded_by unset",
			mem:  &engramv1.Memory{ShortId: "AAAA111111", Scope: "repo:x", Content: "c", CreatedAt: now},
		},
		{
			name: "superseded_by set",
			mem: &engramv1.Memory{
				ShortId: "BBBB222222", Scope: "repo:x", Content: "c", CreatedAt: now,
				SupersededBy: proto.String("successor-id"),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetClientFlags(t)
			resetCommandFlagState(t, getCmd)
			svc := &stubEngramService{
				getFn: func(context.Context, *engramv1.GetMemoryRequest) (*engramv1.GetMemoryResponse, error) {
					return &engramv1.GetMemoryResponse{Memory: tc.mem}, nil
				},
			}
			url := startStubServer(t, svc)

			jsonOut, _, err := runClient(t, "get", tc.mem.GetShortId(), "--server", url, "--output", "json")
			if err != nil {
				t.Fatalf("json lane: unexpected error: %v", err)
			}
			var doc map[string]json.RawMessage
			if err := json.Unmarshal([]byte(strings.TrimSpace(jsonOut)), &doc); err != nil {
				t.Fatalf("json lane: stdout did not unmarshal as a single JSON object: %v\nstdout=%q", err, jsonOut)
			}

			resetCommandFlagState(t, getCmd)
			textOut, _, err := runClient(t, "get", tc.mem.GetShortId(), "--server", url, "--output", "text")
			if err != nil {
				t.Fatalf("text lane: unexpected error: %v", err)
			}

			for key := range doc {
				label := humanizeKey(key)
				if !strings.Contains(textOut, "\n  "+label) {
					t.Errorf("%s: text lane missing field line for json key %q (label %q); textOut=%q", tc.name, key, label, textOut)
				}
			}

			_, hasSupersededInJSON := doc["superseded_by"]
			hasSupersededInText := strings.Contains(textOut, "\n  "+humanizeKey("superseded_by"))
			if hasSupersededInJSON != hasSupersededInText {
				t.Errorf("%s: superseded_by presence mismatch: json=%v text=%v", tc.name, hasSupersededInJSON, hasSupersededInText)
			}
		})
	}
}

// TestGetUnknownIDExitCodeNotFoundNoRetry pins the existing error envelope
// and exit-code taxonomy (D-09/D-10), unchanged by this plan: an unknown id
// maps to exitNotFound and the client makes exactly one attempt.
func TestGetUnknownIDExitCodeNotFoundNoRetry(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, getCmd)
	svc := &stubEngramService{
		getFn: func(context.Context, *engramv1.GetMemoryRequest) (*engramv1.GetMemoryResponse, error) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("nope"))
		},
	}
	url := startStubServer(t, svc)

	_, _, err := runClient(t, "get", "MISSING1111", "--server", url)
	assertExitCode(t, err, exitNotFound)
	if svc.getCalls != 1 {
		t.Errorf("getCalls = %d, want 1 (no retry)", svc.getCalls)
	}
}
