// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// invalidArg returns the Connect error the server emits for a rejected
// input: CodeInvalidArgument carrying the field=/hint= envelope verbatim.
func invalidArg(envelope string) error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New(envelope))
}

// assertEnvelopePassthrough asserts the error Execute() prints to stderr
// carries the server's envelope unchanged and the command exited with
// exitUsage. runClient returns the error rather than the printed line
// (root.go's Execute is what writes "Error: <err>" to stderr), so the
// assertion reads err.Error(), the exact text that line carries.
func assertEnvelopePassthrough(t *testing.T, err error, envelope string) {
	t.Helper()
	assertExitCode(t, err, exitUsage)
	if !strings.Contains(err.Error(), envelope) {
		t.Errorf("error = %q, want it to carry %q unchanged", err.Error(), envelope)
	}
}

// decodeJSONDoc unmarshals stdout as the single JSON object a client verb
// emits under --output json.
func decodeJSONDoc(t *testing.T, stdout string) map[string]json.RawMessage {
	t.Helper()
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc); err != nil {
		t.Fatalf("stdout did not unmarshal as a single JSON object: %v\nstdout=%q", err, stdout)
	}
	return doc
}

func TestRelated(t *testing.T) {
	const hiddenUUID = "018f6c2e-6b2b-7c9e-9c2e-6b2b7c9e9c2e"

	t.Run("renders edges and truncated on both lanes", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, relatedCmd)
		var gotReq *engramv1.RelatedMemoriesRequest
		svc := &stubEngramService{
			relatedFn: func(_ context.Context, req *engramv1.RelatedMemoriesRequest) (*engramv1.RelatedMemoriesResponse, error) {
				gotReq = req
				return &engramv1.RelatedMemoriesResponse{
					Anchor: &engramv1.Memory{ShortId: "ANCHOR0001", Scope: "repo:x"},
					Related: []*engramv1.RelatedMemory{
						{Memory: &engramv1.Memory{ShortId: "SUPER00001"}, Edges: []*engramv1.RelatedEdge{{
							Type: engramv1.EdgeType_EDGE_TYPE_SUPERSESSION,
						}}},
						{Memory: &engramv1.Memory{ShortId: "TAGGED0001"}, Edges: []*engramv1.RelatedEdge{{
							Type:     engramv1.EdgeType_EDGE_TYPE_TAG,
							Evidence: &engramv1.RelatedEdge_Tag{Tag: &engramv1.TagEvidence{SharedTags: []*engramv1.WeightedTag{{Tag: "auth", Weight: 1}}}},
						}}},
					},
					Truncated: true,
				}, nil
			},
		}
		url := startStubServer(t, svc)

		stdout, _, err := runClient(t, "related", "ANCHOR0001", "--server", url, "--k", "3", "--full", "--output", "text")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetId() != "ANCHOR0001" || gotReq.GetK() != 3 || !gotReq.GetFull() {
			t.Errorf("request = %+v, want id/k/full passed through", gotReq)
		}
		for _, want := range []string{"related to ANCHOR0001: 2 record(s), truncated: true", "EDGE_TYPE_SUPERSESSION", "EDGE_TYPE_TAG", "auth", "SUPER00001", "TAGGED0001"} {
			if !strings.Contains(stdout, want) {
				t.Errorf("text stdout = %q, want it to contain %q", stdout, want)
			}
		}

		resetCommandFlagState(t, relatedCmd)
		jsonOut, _, err := runClient(t, "related", "ANCHOR0001", "--server", url, "--output", "json")
		if err != nil {
			t.Fatalf("json lane: %v", err)
		}
		doc := decodeJSONDoc(t, jsonOut)
		for _, key := range []string{"anchor", "related", "truncated"} {
			if doc[key] == nil {
				t.Errorf("json document missing %q: %q", key, jsonOut)
			}
		}
	})

	t.Run("not found exits 4 with no UUID echo", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, relatedCmd)
		svc := &stubEngramService{
			relatedFn: func(context.Context, *engramv1.RelatedMemoriesRequest) (*engramv1.RelatedMemoriesResponse, error) {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("memory NOPE123456: not found"))
			},
		}
		url := startStubServer(t, svc)
		stdout, _, err := runClient(t, "related", "NOPE123456", "--server", url)
		assertExitCode(t, err, exitNotFound)
		if stdout != "" {
			t.Errorf("stdout = %q, want empty on a rejection", stdout)
		}
		if strings.Contains(err.Error(), hiddenUUID) {
			t.Errorf("error = %q, echoes a UUID the caller never supplied", err.Error())
		}
	})

	t.Run("missing id is a usage error before any request", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, relatedCmd)
		called := false
		svc := &stubEngramService{
			relatedFn: func(context.Context, *engramv1.RelatedMemoriesRequest) (*engramv1.RelatedMemoriesResponse, error) {
				called = true
				return &engramv1.RelatedMemoriesResponse{}, nil
			},
		}
		url := startStubServer(t, svc)
		_, _, err := runClient(t, "related", "--server", url)
		assertExitCode(t, err, exitUsage)
		if called {
			t.Error("a request was sent despite the missing id")
		}
		if !strings.Contains(err.Error(), "id argument") {
			t.Errorf("error = %q, want it to name the id argument", err.Error())
		}
	})
}

func TestTags(t *testing.T) {
	t.Run("count table and more footer", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, tagsCmd)
		var gotReq *engramv1.ListTagsRequest
		svc := &stubEngramService{
			listTagsFn: func(_ context.Context, req *engramv1.ListTagsRequest) (*engramv1.ListTagsResponse, error) {
				gotReq = req
				return &engramv1.ListTagsResponse{
					Tags: []*engramv1.TagCount{{Tag: "auth", Count: 7}, {Tag: "ci", Count: 2}},
					More: true,
				}, nil
			},
		}
		url := startStubServer(t, svc)
		stdout, _, err := runClient(t, "tags", "--server", url, "--scope", "repo:x", "--limit", "2", "--output", "text")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetScope() != "repo:x" || gotReq.GetLimit() != 2 {
			t.Errorf("request = %+v, want scope/limit passed through", gotReq)
		}
		if !strings.HasPrefix(stdout, "TAG") || !strings.Contains(stdout, "auth") || !strings.Contains(stdout, "7") {
			t.Errorf("stdout = %q, want a TAG/COUNT table with auth 7", stdout)
		}
		if !strings.HasSuffix(stdout, "more: true\n") {
			t.Errorf("stdout = %q, want it to end with the more footer", stdout)
		}

		resetCommandFlagState(t, tagsCmd)
		jsonOut, _, err := runClient(t, "tags", "--server", url, "--output", "json")
		if err != nil {
			t.Fatalf("json lane: %v", err)
		}
		doc := decodeJSONDoc(t, jsonOut)
		if string(doc["more"]) != "true" {
			t.Errorf(`json "more" = %s, want true`, doc["more"])
		}
		if gotReq.GetScope() != "" {
			t.Errorf("scope omitted should send an empty scope, got %q", gotReq.GetScope())
		}
	})

	t.Run("limit out of range passes the envelope through", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, tagsCmd)
		const envelope = "field=limit hint=out_of_range: limit must be at most 1000"
		svc := &stubEngramService{
			listTagsFn: func(context.Context, *engramv1.ListTagsRequest) (*engramv1.ListTagsResponse, error) {
				return nil, connect.NewError(connect.CodeOutOfRange, errors.New(envelope))
			},
		}
		url := startStubServer(t, svc)
		_, _, err := runClient(t, "tags", "--server", url, "--limit", "1001")
		assertEnvelopePassthrough(t, err, envelope)
	})
}

func TestRules(t *testing.T) {
	t.Run("all-scopes read renders the coverage footer", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, rulesCmd)
		var gotReq *engramv1.ListRulesRequest
		svc := &stubEngramService{
			listRulesFn: func(_ context.Context, req *engramv1.ListRulesRequest) (*engramv1.ListRulesResponse, error) {
				gotReq = req
				return &engramv1.ListRulesResponse{
					Rules:          []*engramv1.Memory{{ShortId: "RULE000001", Scope: "rule:repo:x", Category: "rule", Summary: "MUST do x"}},
					SearchedScopes: []string{"rule:repo:x", "rule:project:p"},
				}, nil
			},
		}
		url := startStubServer(t, svc)
		stdout, _, err := runClient(t, "rules", "--server", url, "--output", "text")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(gotReq.GetScopes()) != 0 {
			t.Errorf("scopes = %v, want none for the all-scopes read", gotReq.GetScopes())
		}
		if !strings.Contains(stdout, "RULE000001") || !strings.Contains(stdout, "MUST do x") {
			t.Errorf("stdout = %q, want the rule row", stdout)
		}
		if !strings.HasSuffix(stdout, "searched_scopes: 2\n") {
			t.Errorf("stdout = %q, want the searched_scopes footer last", stdout)
		}
	})

	t.Run("explicit scopes and tags pass through with no footer", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, rulesCmd)
		var gotReq *engramv1.ListRulesRequest
		svc := &stubEngramService{
			listRulesFn: func(_ context.Context, req *engramv1.ListRulesRequest) (*engramv1.ListRulesResponse, error) {
				gotReq = req
				return &engramv1.ListRulesResponse{SearchedScopes: []string{"rule:repo:x"}}, nil
			},
		}
		url := startStubServer(t, svc)
		stdout, _, err := runClient(t, "rules", "--server", url, "--scope", "rule:repo:x", "--scope", "rule:project:p", "--tag", "vcs", "--full", "--output", "text")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(gotReq.GetScopes()) != 2 || gotReq.GetTags()[0] != "vcs" || !gotReq.GetFull() {
			t.Errorf("request = %+v, want two scopes, one tag, full", gotReq)
		}
		if strings.Contains(stdout, "searched_scopes") {
			t.Errorf("stdout = %q, want no coverage footer on a scoped read", stdout)
		}
	})

	t.Run("wrong prefix passes the envelope through", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, rulesCmd)
		const envelope = "field=scopes hint=prefix: scope must be rule:repo:<repo> or rule:project:<project>"
		svc := &stubEngramService{
			listRulesFn: func(context.Context, *engramv1.ListRulesRequest) (*engramv1.ListRulesResponse, error) {
				return nil, invalidArg(envelope)
			},
		}
		url := startStubServer(t, svc)
		_, _, err := runClient(t, "rules", "--server", url, "--scope", "repo:example")
		assertEnvelopePassthrough(t, err, envelope)
	})
}

func TestScheduled(t *testing.T) {
	t.Run("page with cursor and coverage footer", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, scheduledCmd)
		var gotReq *engramv1.ListScheduledRequest
		svc := &stubEngramService{
			listScheduledFn: func(_ context.Context, req *engramv1.ListScheduledRequest) (*engramv1.ListScheduledResponse, error) {
				gotReq = req
				return &engramv1.ListScheduledResponse{
					Memories:       []*engramv1.Memory{{ShortId: "SCHED00001", Scope: "repo:x"}},
					NextPageToken:  "cursor-2",
					SearchedScopes: []string{"repo:x", "repo:y"},
				}, nil
			},
		}
		url := startStubServer(t, svc)
		stdout, _, err := runClient(t, "scheduled", "--server", url, "--cross-spine", "--state", "all", "--limit", "5",
			"--created-after", "2026-01-01T00:00:00Z", "--created-before", "2026-02-01T00:00:00Z", "--cursor", "cursor-1", "--output", "text")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !gotReq.GetCrossSpine() || gotReq.GetState() != "all" || gotReq.GetLimit() != 5 || gotReq.GetPageToken() != "cursor-1" ||
			gotReq.GetCreatedAfter() != "2026-01-01T00:00:00Z" || gotReq.GetCreatedBefore() != "2026-02-01T00:00:00Z" {
			t.Errorf("request = %+v, want every flag passed through", gotReq)
		}
		if !strings.Contains(stdout, "SCHED00001") || !strings.Contains(stdout, "next_page_token: cursor-2\n") {
			t.Errorf("stdout = %q, want the row and the next_page_token line", stdout)
		}
		if !strings.HasSuffix(stdout, "searched_scopes: 2\n") {
			t.Errorf("stdout = %q, want the coverage footer last", stdout)
		}

		resetCommandFlagState(t, scheduledCmd)
		jsonOut, _, err := runClient(t, "scheduled", "--server", url, "--scope", "repo:x", "--output", "json")
		if err != nil {
			t.Fatalf("json lane: %v", err)
		}
		doc := decodeJSONDoc(t, jsonOut)
		if string(doc["next_page_token"]) != `"cursor-2"` {
			t.Errorf("json next_page_token = %s, want cursor-2", doc["next_page_token"])
		}
	})

	t.Run("scope missing without cross-spine is rejected before any request", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, scheduledCmd)
		called := false
		svc := &stubEngramService{
			listScheduledFn: func(context.Context, *engramv1.ListScheduledRequest) (*engramv1.ListScheduledResponse, error) {
				called = true
				return &engramv1.ListScheduledResponse{}, nil
			},
		}
		url := startStubServer(t, svc)
		_, _, err := runClient(t, "scheduled", "--server", url)
		assertExitCode(t, err, exitUsage)
		if called {
			t.Error("a request was sent despite the missing scope")
		}
	})

	t.Run("conditional_required and enum envelopes pass through", func(t *testing.T) {
		for _, envelope := range []string{
			"field=scope hint=conditional_required: scope is required unless cross_spine is true",
			"field=state hint=enum: state must be one of scheduled, expired, all",
		} {
			resetClientFlags(t)
			resetCommandFlagState(t, scheduledCmd)
			svc := &stubEngramService{
				listScheduledFn: func(context.Context, *engramv1.ListScheduledRequest) (*engramv1.ListScheduledResponse, error) {
					return nil, invalidArg(envelope)
				},
			}
			url := startStubServer(t, svc)
			_, _, err := runClient(t, "scheduled", "--server", url, "--scope", "repo:x", "--state", "later")
			assertEnvelopePassthrough(t, err, envelope)
		}
	})
}

func TestSearchDiscovery(t *testing.T) {
	t.Run("rank order with score, kind and citations", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, searchDiscoveryCmd)
		var gotReq *engramv1.SearchDiscoveriesRequest
		svc := &stubEngramService{
			searchDiscFn: func(_ context.Context, req *engramv1.SearchDiscoveriesRequest) (*engramv1.SearchDiscoveriesResponse, error) {
				gotReq = req
				return &engramv1.SearchDiscoveriesResponse{Discoveries: []*engramv1.Memory{
					{ShortId: "DISC000001", Kind: "fact", Score: 0.91, Citations: []*engramv1.Citation{{Kind: "file", Ref: "a.go:12"}}},
					{ShortId: "DISC000002", Kind: "map", Score: 0.42},
				}}, nil
			},
		}
		url := startStubServer(t, svc)
		stdout, _, err := runClient(t, "search-discovery", "--server", url, "--query", "how auth works", "--scope", "discovery:repo:x", "--k", "2", "--output", "text")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetQuery() != "how auth works" || gotReq.GetScope() != "discovery:repo:x" || gotReq.GetK() != 2 {
			t.Errorf("request = %+v, want query/scope/k passed through", gotReq)
		}
		if strings.Index(stdout, "DISC000001") > strings.Index(stdout, "DISC000002") {
			t.Errorf("stdout = %q, want rank order preserved", stdout)
		}
		for _, want := range []string{"discoveries: 2", "kind=fact", "score=0.91", "a.go:12"} {
			if !strings.Contains(stdout, want) {
				t.Errorf("stdout = %q, want it to contain %q", stdout, want)
			}
		}
	})

	t.Run("non-discovery scope passes the envelope through", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, searchDiscoveryCmd)
		const envelope = "field=scope hint=prefix: scope must start with discovery:"
		svc := &stubEngramService{
			searchDiscFn: func(context.Context, *engramv1.SearchDiscoveriesRequest) (*engramv1.SearchDiscoveriesResponse, error) {
				return nil, invalidArg(envelope)
			},
		}
		url := startStubServer(t, svc)
		_, _, err := runClient(t, "search-discovery", "--server", url, "--query", "q", "--scope", "repo:x")
		assertEnvelopePassthrough(t, err, envelope)
	})

	t.Run("empty query is a usage error before any request", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, searchDiscoveryCmd)
		called := false
		svc := &stubEngramService{
			searchDiscFn: func(context.Context, *engramv1.SearchDiscoveriesRequest) (*engramv1.SearchDiscoveriesResponse, error) {
				called = true
				return &engramv1.SearchDiscoveriesResponse{}, nil
			},
		}
		url := startStubServer(t, svc)
		_, _, err := runClient(t, "search-discovery", "--server", url, "--scope", "discovery:repo:x")
		assertExitCode(t, err, exitUsage)
		if called {
			t.Error("a request was sent despite the empty query")
		}
		if !strings.Contains(err.Error(), "--query") {
			t.Errorf("error = %q, want it to name --query", err.Error())
		}
	})
}
