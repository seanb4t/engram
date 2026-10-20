// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// memoryStateStub is a stateful in-process Connect server for the client
// verbs: one principal's records in a map, with the per-verb semantics the
// real core has at the level the CLI can observe (ids and short ids,
// archived state, visibility, windows, supersession stamps, deletion). It
// exists so the whole verb sequence can run against ONE server in ONE
// test; it is not a model of the server's validation, which the stub
// tests above cover envelope by envelope.
type memoryStateStub struct {
	stubEngramService
	mu      sync.Mutex
	next    int
	records map[string]*engramv1.Memory
}

func newMemoryStateStub() *memoryStateStub {
	s := &memoryStateStub{records: map[string]*engramv1.Memory{}}
	s.storeFn = func(_ context.Context, req *engramv1.StoreMemoryRequest) (*engramv1.StoreMemoryResponse, error) {
		m := s.add(req.GetContent(), req.GetScope(), req.GetTags())
		return &engramv1.StoreMemoryResponse{Id: m.GetId(), ShortId: m.GetShortId()}, nil
	}
	s.getFn = func(_ context.Context, req *engramv1.GetMemoryRequest) (*engramv1.GetMemoryResponse, error) {
		m, err := s.find(req.GetId())
		if err != nil {
			return nil, err
		}
		return &engramv1.GetMemoryResponse{Memory: m}, nil
	}
	s.updateFn = func(_ context.Context, req *engramv1.UpdateMemoryRequest) (*engramv1.UpdateMemoryResponse, error) {
		m, err := s.find(req.GetId())
		if err != nil {
			return nil, err
		}
		for _, p := range req.GetUpdateMask().GetPaths() {
			switch p {
			case "content":
				m.Content = req.GetContent()
			case "tags":
				m.Tags = req.GetTags()
			case "shared":
				m.Visibility = visibilityWord(req.GetShared())
			case "summary":
				m.Summary = req.GetSummary()
			}
		}
		return &engramv1.UpdateMemoryResponse{Id: m.GetId(), ShortId: m.GetShortId()}, nil
	}
	s.setVisibilityFn = func(_ context.Context, req *engramv1.SetVisibilityRequest) (*engramv1.SetVisibilityResponse, error) {
		m, err := s.find(req.GetId())
		if err != nil {
			return nil, err
		}
		m.Visibility = visibilityWord(req.GetVisibility() == engramv1.Visibility_VISIBILITY_SHARED)
		return &engramv1.SetVisibilityResponse{Id: m.GetId(), ShortId: m.GetShortId()}, nil
	}
	s.scheduleFn = func(_ context.Context, req *engramv1.ScheduleMemoryRequest) (*engramv1.ScheduleMemoryResponse, error) {
		m := s.add(req.GetContent(), req.GetScope(), req.GetTags())
		m.NotBefore = req.GetNotBefore()
		return &engramv1.ScheduleMemoryResponse{Id: m.GetId(), ShortId: m.GetShortId()}, nil
	}
	s.supersedeFn = func(_ context.Context, req *engramv1.SupersedeMemoryRequest) (*engramv1.SupersedeMemoryResponse, error) {
		var ids []string
		var targets []*engramv1.Memory
		for _, t := range req.GetSupersedes() {
			m, err := s.find(t)
			if err != nil {
				return nil, err
			}
			ids = append(ids, m.GetId())
			targets = append(targets, m)
		}
		if req.GetValidateOnly() {
			return &engramv1.SupersedeMemoryResponse{Validated: true, Supersedes: ids, Targets: targets}, nil
		}
		m := s.add(req.GetContent(), req.GetScope(), req.GetTags())
		for _, t := range targets {
			id := m.GetId()
			t.SupersededBy = &id
		}
		return &engramv1.SupersedeMemoryResponse{Id: m.GetId(), ShortId: m.GetShortId(), Supersedes: ids}, nil
	}
	s.archiveFn = func(_ context.Context, req *engramv1.ArchiveMemoryRequest) (*engramv1.ArchiveMemoryResponse, error) {
		return &engramv1.ArchiveMemoryResponse{Results: s.flip(req.GetIds(), true)}, nil
	}
	s.restoreFn = func(_ context.Context, req *engramv1.RestoreMemoryRequest) (*engramv1.RestoreMemoryResponse, error) {
		return &engramv1.RestoreMemoryResponse{Results: s.flip(req.GetIds(), false)}, nil
	}
	s.relatedFn = func(_ context.Context, req *engramv1.RelatedMemoriesRequest) (*engramv1.RelatedMemoriesResponse, error) {
		anchor, err := s.find(req.GetId())
		if err != nil {
			return nil, err
		}
		resp := &engramv1.RelatedMemoriesResponse{Anchor: anchor}
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, m := range s.records {
			if m.GetSupersededBy() == anchor.GetId() {
				resp.Related = append(resp.Related, &engramv1.RelatedMemory{Memory: m, Edges: []*engramv1.RelatedEdge{{Type: engramv1.EdgeType_EDGE_TYPE_SUPERSESSION}}})
			}
		}
		return resp, nil
	}
	s.listTagsFn = func(context.Context, *engramv1.ListTagsRequest) (*engramv1.ListTagsResponse, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		counts := map[string]uint64{}
		for _, m := range s.records {
			if m.ArchivedAt != nil || m.SupersededBy != nil || m.NotBefore != nil {
				continue
			}
			for _, t := range m.GetTags() {
				counts[t]++
			}
		}
		resp := &engramv1.ListTagsResponse{}
		for _, tag := range []string{"e2e", "renamed"} {
			if c, ok := counts[tag]; ok {
				resp.Tags = append(resp.Tags, &engramv1.TagCount{Tag: tag, Count: c})
			}
		}
		return resp, nil
	}
	s.listScheduledFn = func(context.Context, *engramv1.ListScheduledRequest) (*engramv1.ListScheduledResponse, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		resp := &engramv1.ListScheduledResponse{}
		for _, m := range s.records {
			if m.NotBefore != nil {
				resp.Memories = append(resp.Memories, m)
			}
		}
		return resp, nil
	}
	s.deleteFn = func(_ context.Context, req *engramv1.DeleteMemoryRequest) (*engramv1.DeleteMemoryResponse, error) {
		m, err := s.find(req.GetId())
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		delete(s.records, m.GetId())
		s.mu.Unlock()
		return &engramv1.DeleteMemoryResponse{}, nil
	}
	return s
}

func visibilityWord(shared bool) string {
	if shared {
		return "shared"
	}
	return "private"
}

func (s *memoryStateStub) add(content, scope string, tags []string) *engramv1.Memory {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	id := "e2e00000-0000-0000-0000-00000000000" + string(rune('0'+s.next))
	m := &engramv1.Memory{Id: id, ShortId: "E2E000000" + string(rune('0'+s.next)), Content: content, Scope: scope, Tags: tags, Visibility: "private", Category: "decision"}
	s.records[id] = m
	return m
}

func (s *memoryStateStub) find(idOrShort string) (*engramv1.Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.records {
		if m.GetId() == idOrShort || m.GetShortId() == idOrShort {
			return m, nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, errors.New("not found: "+idOrShort))
}

func (s *memoryStateStub) flip(ids []string, archive bool) []*engramv1.ArchiveResult {
	out := make([]*engramv1.ArchiveResult, 0, len(ids))
	for _, id := range ids {
		m, err := s.find(id)
		if err != nil {
			out = append(out, &engramv1.ArchiveResult{Requested: id, Outcome: engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_NOT_FOUND})
			continue
		}
		switch {
		case archive && m.ArchivedAt == nil:
			m.ArchivedAt = timestamppb.Now()
			out = append(out, &engramv1.ArchiveResult{Requested: id, Id: m.GetId(), Outcome: engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_ARCHIVED})
		case archive:
			out = append(out, &engramv1.ArchiveResult{Requested: id, Id: m.GetId(), Outcome: engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_ALREADY_ARCHIVED})
		case m.ArchivedAt != nil:
			m.ArchivedAt = nil
			out = append(out, &engramv1.ArchiveResult{Requested: id, Id: m.GetId(), Outcome: engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_RESTORED})
		default:
			out = append(out, &engramv1.ArchiveResult{Requested: id, Id: m.GetId(), Outcome: engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_NOT_ARCHIVED})
		}
	}
	return out
}

// TestClientVerbsEndToEnd runs the parity verbs in sequence against one
// stateful in-process Connect server under one principal — store, update,
// set-visibility, schedule, supersede --validate-only, supersede, archive,
// restore, related, tags, scheduled, delete --apply — asserting each
// verb's exit code and the JSON document it emits, and that each write's
// effect is visible to the next verb.
func TestClientVerbsEndToEnd(t *testing.T) {
	svc := newMemoryStateStub()
	url := startStubServer(t, svc)

	run := func(args ...string) map[string]json.RawMessage {
		t.Helper()
		resetClientFlags(t)
		resetEveryCommandFlagState(t, rootCmd)
		stdout, _, err := runClient(t, append(args, "--server", url, "--output", "json")...)
		if err != nil {
			t.Fatalf("%v: exit %d: %v", args, exitCodeFromError(err), err)
		}
		return decodeJSONDoc(t, stdout)
	}
	str := func(doc map[string]json.RawMessage, key string) string {
		var v string
		_ = json.Unmarshal(doc[key], &v)
		return v
	}

	stored := run("store", "--content", "first", "--scope", "repo:e2e", "--source", "user-said", "--category", "decision", "--tags", "e2e")
	id := str(stored, "short_id")
	if id == "" {
		t.Fatal("store returned no short_id")
	}

	updated := run("update", id, "--content", "second", "--tags", "renamed", "--apply")
	if !strings.Contains(string(updated["update_mask"]), "content") || str(updated, "short_id") != id {
		t.Errorf("update doc = %v, want the mask and the same short_id", updated)
	}
	if got := run("get", id); str(got, "content") != "second" || !strings.Contains(string(got["tags"]), "renamed") {
		t.Errorf("get after update = %v, want content second and tag renamed", got)
	}

	run("set-visibility", id, "--shared")
	if got := run("get", id); str(got, "visibility") != "shared" {
		t.Errorf("visibility after set-visibility = %q, want shared", str(got, "visibility"))
	}

	scheduled := run("schedule", "--content", "later", "--scope", "repo:e2e", "--source", "user-said", "--category", "gotcha", "--not-before", "2030-01-01T00:00:00Z")
	if str(scheduled, "short_id") == "" {
		t.Error("schedule returned no short_id")
	}
	if listed := run("scheduled", "--scope", "repo:e2e"); !strings.Contains(string(listed["memories"]), str(scheduled, "short_id")) {
		t.Errorf("scheduled listing = %v, want the windowed record", listed)
	}

	validated := run("supersede", "--content", "third", "--scope", "repo:e2e", "--source", "user-said", "--category", "decision", "--supersedes", id, "--validate-only")
	if string(validated["validated"]) != "true" || str(validated, "id") != "" {
		t.Errorf("validate-only doc = %v, want validated:true and no id", validated)
	}
	superseded := run("supersede", "--content", "third", "--scope", "repo:e2e", "--source", "user-said", "--category", "decision", "--tags", "renamed", "--supersedes", id)
	successor := str(superseded, "short_id")
	if successor == "" || successor == id {
		t.Fatalf("supersede returned short_id %q", successor)
	}
	if got := run("get", id); str(got, "superseded_by") == "" {
		t.Errorf("predecessor after supersede = %v, want superseded_by set", got)
	}

	archived := run("archive", successor, "nope")
	if !strings.Contains(string(archived["results"]), "ARCHIVE_OUTCOME_ARCHIVED") || !strings.Contains(string(archived["results"]), "ARCHIVE_OUTCOME_NOT_FOUND") {
		t.Errorf("archive doc = %v, want archived then not_found", archived)
	}
	restored := run("restore", successor)
	if !strings.Contains(string(restored["results"]), "ARCHIVE_OUTCOME_RESTORED") {
		t.Errorf("restore doc = %v, want restored", restored)
	}

	related := run("related", successor)
	if !strings.Contains(string(related["related"]), id) || !strings.Contains(string(related["related"]), "EDGE_TYPE_SUPERSESSION") {
		t.Errorf("related doc = %v, want the predecessor on a supersession edge", related)
	}

	tags := run("tags", "--scope", "repo:e2e")
	if !strings.Contains(string(tags["tags"]), "renamed") || strings.Contains(string(tags["tags"]), `"e2e"`) {
		t.Errorf("tags doc = %v, want renamed counted on the live successor and the superseded record's tag absent", tags)
	}

	preview := run("delete", successor)
	if string(preview["deleted"]) != "false" {
		t.Errorf("delete preview = %v, want deleted:false", preview)
	}
	deleted := run("delete", successor, "--apply")
	if string(deleted["deleted"]) != "true" {
		t.Errorf("delete apply = %v, want deleted:true", deleted)
	}
	resetClientFlags(t)
	resetEveryCommandFlagState(t, rootCmd)
	_, _, err := runClient(t, "get", successor, "--server", url)
	assertExitCode(t, err, exitNotFound)
}
