// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

const hiddenUUID = "018f6c2e-6b2b-7c9e-9c2e-6b2b7c9e9c2e"

func TestArchiveRestore(t *testing.T) {
	t.Run("mixed outcomes in input order", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, archiveCmd)
		var gotIDs []string
		svc := &stubEngramService{
			archiveFn: func(_ context.Context, req *engramv1.ArchiveMemoryRequest) (*engramv1.ArchiveMemoryResponse, error) {
				gotIDs = req.GetIds()
				return &engramv1.ArchiveMemoryResponse{Results: []*engramv1.ArchiveResult{
					{Requested: "a1", Id: "11111111-1111-1111-1111-111111111111", Outcome: engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_ARCHIVED},
					{Requested: "b2", Id: "22222222-2222-2222-2222-222222222222", Outcome: engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_ALREADY_ARCHIVED},
					{Requested: "c3", Outcome: engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_NOT_FOUND},
				}}, nil
			},
		}
		url := startStubServer(t, svc)
		stdout, _, err := runClient(t, "archive", "a1", "b2", "c3", "--server", url, "--output", "text")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Join(gotIDs, ",") != "a1,b2,c3" {
			t.Errorf("request ids = %v, want caller order", gotIDs)
		}
		i1, i2, i3 := strings.Index(stdout, "ARCHIVE_OUTCOME_ARCHIVED"), strings.Index(stdout, "ARCHIVE_OUTCOME_ALREADY_ARCHIVED"), strings.Index(stdout, "ARCHIVE_OUTCOME_NOT_FOUND")
		if i1 < 0 || i2 < 0 || i3 < 0 || i1 >= i2 || i2 >= i3 {
			t.Errorf("stdout = %q, want the three outcomes in input order", stdout)
		}
		if !strings.HasPrefix(stdout, "archive: 3 id(s)") {
			t.Errorf("stdout = %q, want the counting headline", stdout)
		}
		if strings.Contains(stdout, hiddenUUID) {
			t.Errorf("stdout = %q, echoes a UUID the server withheld", stdout)
		}
		if svc.writeCalls["ArchiveMemory"] != 1 {
			t.Errorf("ArchiveMemory calls = %d, want 1", svc.writeCalls["ArchiveMemory"])
		}
	})

	t.Run("restore json lane carries results", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, restoreCmd)
		svc := &stubEngramService{
			restoreFn: func(context.Context, *engramv1.RestoreMemoryRequest) (*engramv1.RestoreMemoryResponse, error) {
				return &engramv1.RestoreMemoryResponse{Results: []*engramv1.ArchiveResult{
					{Requested: "a1", Outcome: engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_RESTORED},
					{Requested: "b2", Outcome: engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_NOT_ARCHIVED},
				}}, nil
			},
		}
		url := startStubServer(t, svc)
		stdout, _, err := runClient(t, "restore", "a1", "b2", "--server", url, "--output", "json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		doc := decodeJSONDoc(t, stdout)
		if doc["results"] == nil || !strings.Contains(string(doc["results"]), "ARCHIVE_OUTCOME_RESTORED") {
			t.Errorf("json results = %s, want the restored outcome", doc["results"])
		}
	})

	t.Run("too many ids passes the envelope through", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, archiveCmd)
		const envelope = "field=ids hint=too_many: at most 1000 ids per call"
		svc := &stubEngramService{
			archiveFn: func(context.Context, *engramv1.ArchiveMemoryRequest) (*engramv1.ArchiveMemoryResponse, error) {
				return nil, invalidArg(envelope)
			},
		}
		url := startStubServer(t, svc)
		args := append([]string{"archive"}, make([]string, 1001)...)
		for i := 1; i <= 1001; i++ {
			args[i] = "id"
		}
		_, _, err := runClient(t, append(args, "--server", url)...)
		assertEnvelopePassthrough(t, err, envelope)
	})

	t.Run("no ids is a usage error before any request", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, restoreCmd)
		svc := &stubEngramService{
			restoreFn: func(context.Context, *engramv1.RestoreMemoryRequest) (*engramv1.RestoreMemoryResponse, error) {
				return &engramv1.RestoreMemoryResponse{}, nil
			},
		}
		url := startStubServer(t, svc)
		_, _, err := runClient(t, "restore", "--server", url)
		assertExitCode(t, err, exitUsage)
		if svc.writeCalls["RestoreMemory"] != 0 {
			t.Error("a request was sent despite no ids")
		}
	})
}

func TestUpdate(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantPaths string
		wantTags  []string
		wantShare bool
	}{
		{"content", []string{"--content", "new text"}, "content", nil, false},
		{"summary", []string{"--summary", "s"}, "summary", nil, false},
		{"tags", []string{"--tags", "a", "--tags", "b"}, "tags", []string{"a", "b"}, false},
		{"tags cleared", []string{"--tags", ""}, "tags", []string{}, false},
		{"shared", []string{"--shared"}, "shared", nil, true},
		{"private", []string{"--private"}, "shared", nil, false},
		{"shared negated unshares", []string{"--shared=false"}, "shared", nil, false},
		{"private negated shares", []string{"--private=false"}, "shared", nil, true},
		{"content and tags", []string{"--content", "new text", "--tags", "a", "--tags", "b"}, "content,tags", []string{"a", "b"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetClientFlags(t)
			resetCommandFlagState(t, updateCmd)
			var gotReq *engramv1.UpdateMemoryRequest
			svc := &stubEngramService{
				updateFn: func(_ context.Context, req *engramv1.UpdateMemoryRequest) (*engramv1.UpdateMemoryResponse, error) {
					gotReq = req
					return &engramv1.UpdateMemoryResponse{Id: "11111111-1111-1111-1111-111111111111", ShortId: "UPDATED001"}, nil
				},
			}
			url := startStubServer(t, svc)
			args := append([]string{"update", "UPDATED001", "--server", url, "--apply", "--output", "text"}, tc.args...)
			stdout, _, err := runClient(t, args...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := strings.Join(gotReq.GetUpdateMask().GetPaths(), ","); got != tc.wantPaths {
				t.Errorf("update_mask paths = %q, want %q", got, tc.wantPaths)
			}
			if tc.wantTags != nil && strings.Join(gotReq.GetTags(), ",") != strings.Join(tc.wantTags, ",") {
				t.Errorf("tags = %v, want %v", gotReq.GetTags(), tc.wantTags)
			}
			if gotReq.GetShared() != tc.wantShare {
				t.Errorf("shared = %v, want %v", gotReq.GetShared(), tc.wantShare)
			}
			if gotReq.GetId() != "UPDATED001" {
				t.Errorf("id = %q, want the positional id", gotReq.GetId())
			}
			if !strings.HasPrefix(stdout, "updated: UPDATED001 (11111111-1111-1111-1111-111111111111)") {
				t.Errorf("stdout = %q, want the updated headline", stdout)
			}
		})
	}

	t.Run("preview fetches the record and writes nothing", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, updateCmd)
		svc := &stubEngramService{
			getFn: func(context.Context, *engramv1.GetMemoryRequest) (*engramv1.GetMemoryResponse, error) {
				return &engramv1.GetMemoryResponse{Memory: &engramv1.Memory{ShortId: "UPDATED001", Scope: "repo:x", Summary: "before"}}, nil
			},
			updateFn: func(context.Context, *engramv1.UpdateMemoryRequest) (*engramv1.UpdateMemoryResponse, error) {
				return &engramv1.UpdateMemoryResponse{}, nil
			},
		}
		url := startStubServer(t, svc)
		stdout, _, err := runClient(t, "update", "UPDATED001", "--server", url, "--content", "after", "--tags", "a", "--output", "text")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(stdout, "would update record UPDATED001 (content, tags)") || !strings.Contains(stdout, "before") {
			t.Errorf("stdout = %q, want the would-update headline naming the mask and the current record", stdout)
		}
		if svc.writeCalls["UpdateMemory"] != 0 || svc.getCalls != 1 {
			t.Errorf("UpdateMemory calls = %d, GetMemory calls = %d; want 0 and 1", svc.writeCalls["UpdateMemory"], svc.getCalls)
		}
		resetCommandFlagState(t, updateCmd)
		jsonOut, _, err := runClient(t, "update", "UPDATED001", "--server", url, "--content", "after", "--output", "json")
		if err != nil {
			t.Fatalf("json lane: %v", err)
		}
		doc := decodeJSONDoc(t, jsonOut)
		if string(doc["applied"]) != "false" || string(doc["update_mask"]) != `["content"]` || doc["memory"] == nil {
			t.Errorf("json = %q, want applied:false, the mask and the memory", jsonOut)
		}
	})

	t.Run("no field flag is a usage error before any request", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, updateCmd)
		svc := &stubEngramService{
			updateFn: func(context.Context, *engramv1.UpdateMemoryRequest) (*engramv1.UpdateMemoryResponse, error) {
				return &engramv1.UpdateMemoryResponse{}, nil
			},
		}
		url := startStubServer(t, svc)
		_, _, err := runClient(t, "update", "X", "--server", url)
		assertExitCode(t, err, exitUsage)
		if !strings.Contains(err.Error(), "--content") || !strings.Contains(err.Error(), "--private") {
			t.Errorf("error = %q, want it to name the field flags", err.Error())
		}
		if svc.writeCalls["UpdateMemory"] != 0 {
			t.Error("a request was sent despite no field flag")
		}
	})

	t.Run("shared and private together exit 2", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, updateCmd)
		url := startStubServer(t, &stubEngramService{})
		_, _, err := runClient(t, "update", "X", "--server", url, "--shared", "--private")
		assertExitCode(t, err, exitUsage)
	})

	t.Run("un-sharing a rule passes the rejection through", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, updateCmd)
		const envelope = "field=shared hint=not_applicable: a rule is always shared; delete the rule instead"
		svc := &stubEngramService{
			updateFn: func(context.Context, *engramv1.UpdateMemoryRequest) (*engramv1.UpdateMemoryResponse, error) {
				return nil, invalidArg(envelope)
			},
		}
		url := startStubServer(t, svc)
		_, _, err := runClient(t, "update", "RULE000001", "--server", url, "--private", "--apply")
		assertEnvelopePassthrough(t, err, "delete the rule instead")
	})

	t.Run("not owned exits 4 with no UUID echo", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, updateCmd)
		svc := &stubEngramService{
			updateFn: func(context.Context, *engramv1.UpdateMemoryRequest) (*engramv1.UpdateMemoryResponse, error) {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("not found: THEIRS00001"))
			},
		}
		url := startStubServer(t, svc)
		_, _, err := runClient(t, "update", "THEIRS00001", "--server", url, "--content", "x", "--apply")
		assertExitCode(t, err, exitNotFound)
		if strings.Contains(err.Error(), hiddenUUID) {
			t.Errorf("error = %q, echoes a UUID", err.Error())
		}
	})
}

func TestSetVisibility(t *testing.T) {
	for _, tc := range []struct {
		flag string
		want engramv1.Visibility
	}{
		{"--shared", engramv1.Visibility_VISIBILITY_SHARED},
		{"--private", engramv1.Visibility_VISIBILITY_PRIVATE},
		// A negated form reads the way it is written, on both verbs.
		{"--shared=false", engramv1.Visibility_VISIBILITY_PRIVATE},
		{"--private=false", engramv1.Visibility_VISIBILITY_SHARED},
	} {
		t.Run(tc.flag, func(t *testing.T) {
			resetClientFlags(t)
			resetCommandFlagState(t, setVisibilityCmd)
			var gotReq *engramv1.SetVisibilityRequest
			svc := &stubEngramService{
				setVisibilityFn: func(_ context.Context, req *engramv1.SetVisibilityRequest) (*engramv1.SetVisibilityResponse, error) {
					gotReq = req
					return &engramv1.SetVisibilityResponse{Id: "11111111-1111-1111-1111-111111111111", ShortId: "VIS0000001"}, nil
				},
			}
			url := startStubServer(t, svc)
			stdout, _, err := runClient(t, "set-visibility", "VIS0000001", "--server", url, tc.flag, "--output", "json")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotReq.GetVisibility() != tc.want || gotReq.GetId() != "VIS0000001" {
				t.Errorf("request = %+v, want visibility %v on VIS0000001", gotReq, tc.want)
			}
			doc := decodeJSONDoc(t, stdout)
			if string(doc["short_id"]) != `"VIS0000001"` {
				t.Errorf("json short_id = %s", doc["short_id"])
			}
		})
	}

	t.Run("neither flag is a usage error before any request", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setVisibilityCmd)
		svc := &stubEngramService{
			setVisibilityFn: func(context.Context, *engramv1.SetVisibilityRequest) (*engramv1.SetVisibilityResponse, error) {
				return &engramv1.SetVisibilityResponse{}, nil
			},
		}
		url := startStubServer(t, svc)
		_, _, err := runClient(t, "set-visibility", "X", "--server", url)
		assertExitCode(t, err, exitUsage)
		if !strings.Contains(err.Error(), "--shared") || !strings.Contains(err.Error(), "--private") {
			t.Errorf("error = %q, want it to name both flags", err.Error())
		}
		if svc.writeCalls["SetVisibility"] != 0 {
			t.Error("a request was sent despite neither flag")
		}
	})

	t.Run("rule target passes the rejection through", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setVisibilityCmd)
		svc := &stubEngramService{
			setVisibilityFn: func(context.Context, *engramv1.SetVisibilityRequest) (*engramv1.SetVisibilityResponse, error) {
				return nil, invalidArg("field=id hint=not_applicable: a rule is always shared; delete the rule instead")
			},
		}
		url := startStubServer(t, svc)
		_, _, err := runClient(t, "set-visibility", "RULE000001", "--server", url, "--private")
		assertEnvelopePassthrough(t, err, "delete the rule instead")
	})
}

func TestSchedule(t *testing.T) {
	base := []string{"schedule", "--content", "c", "--scope", "repo:x", "--source", "user-said", "--category", "gotcha"}

	t.Run("window and idempotency key reach the request", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, scheduleCmd)
		var gotReq *engramv1.ScheduleMemoryRequest
		svc := &stubEngramService{
			scheduleFn: func(_ context.Context, req *engramv1.ScheduleMemoryRequest) (*engramv1.ScheduleMemoryResponse, error) {
				gotReq = req
				return &engramv1.ScheduleMemoryResponse{Id: "11111111-1111-1111-1111-111111111111", ShortId: "SCHED00001"}, nil
			},
		}
		url := startStubServer(t, svc)
		args := append(append([]string{}, base...), "--server", url, "--not-before", "2027-01-01T00:00:00Z", "--not-after", "2027-02-01T00:00:00Z", "--idempotency-key", "k1", "--output", "text")
		stdout, _, err := runClient(t, args...)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetNotBefore().AsTime().Format("2006-01-02") != "2027-01-01" || gotReq.GetNotAfter().AsTime().Format("2006-01-02") != "2027-02-01" {
			t.Errorf("window = %v..%v, want the parsed bounds", gotReq.GetNotBefore().AsTime(), gotReq.GetNotAfter().AsTime())
		}
		if gotReq.GetIdempotencyKey() != "k1" || gotReq.GetCategory() != "gotcha" || gotReq.GetSource() != "user-said" {
			t.Errorf("request = %+v, want the store fields and key passed through", gotReq)
		}
		if !strings.Contains(stdout, "scheduled: SCHED00001") {
			t.Errorf("stdout = %q, want the id line", stdout)
		}
	})

	t.Run("no window is a usage error before any request", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, scheduleCmd)
		svc := &stubEngramService{scheduleFn: func(context.Context, *engramv1.ScheduleMemoryRequest) (*engramv1.ScheduleMemoryResponse, error) {
			return &engramv1.ScheduleMemoryResponse{}, nil
		}}
		url := startStubServer(t, svc)
		_, _, err := runClient(t, append(append([]string{}, base...), "--server", url)...)
		assertExitCode(t, err, exitUsage)
		if !strings.Contains(err.Error(), "--not-before") || !strings.Contains(err.Error(), "--not-after") {
			t.Errorf("error = %q, want it to name both bounds", err.Error())
		}
		if svc.writeCalls["ScheduleMemory"] != 0 {
			t.Error("a request was sent despite no window")
		}
	})

	t.Run("malformed timestamp names the field and hint before any request", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, scheduleCmd)
		svc := &stubEngramService{scheduleFn: func(context.Context, *engramv1.ScheduleMemoryRequest) (*engramv1.ScheduleMemoryResponse, error) {
			return &engramv1.ScheduleMemoryResponse{}, nil
		}}
		url := startStubServer(t, svc)
		_, _, err := runClient(t, append(append([]string{}, base...), "--server", url, "--not-after", "tomorrow")...)
		assertEnvelopePassthrough(t, err, "field=not_after hint=format")
		if svc.writeCalls["ScheduleMemory"] != 0 {
			t.Error("a request was sent despite the malformed bound")
		}
	})

	t.Run("discovery category passes the envelope through", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, scheduleCmd)
		const envelope = "field=category hint=enum: discovery is not schedulable; use store_discovery"
		svc := &stubEngramService{scheduleFn: func(context.Context, *engramv1.ScheduleMemoryRequest) (*engramv1.ScheduleMemoryResponse, error) {
			return nil, invalidArg(envelope)
		}}
		url := startStubServer(t, svc)
		_, _, err := runClient(t, "schedule", "--content", "c", "--scope", "repo:x", "--category", "discovery", "--not-after", "2027-01-01T00:00:00Z", "--server", url)
		assertEnvelopePassthrough(t, err, envelope)
	})
}

func TestSupersede(t *testing.T) {
	base := []string{"supersede", "--content", "merged", "--scope", "repo:x", "--source", "user-said"}

	t.Run("validate-only renders targets and writes nothing", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, supersedeCmd)
		var gotReq *engramv1.SupersedeMemoryRequest
		svc := &stubEngramService{
			supersedeFn: func(_ context.Context, req *engramv1.SupersedeMemoryRequest) (*engramv1.SupersedeMemoryResponse, error) {
				gotReq = req
				return &engramv1.SupersedeMemoryResponse{
					Validated:  true,
					Supersedes: []string{"11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"},
					Targets:    []*engramv1.Memory{{ShortId: "TARGET0001"}, {ShortId: "TARGET0002"}},
				}, nil
			},
		}
		url := startStubServer(t, svc)
		args := append(append([]string{}, base...), "--server", url, "--supersedes", "TARGET0001", "--supersedes", "TARGET0002",
			"--citation", "file:a.go:12", "--validate-only", "--output", "text")
		stdout, _, err := runClient(t, args...)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !gotReq.GetValidateOnly() || strings.Join(gotReq.GetSupersedes(), ",") != "TARGET0001,TARGET0002" {
			t.Errorf("request = %+v, want validate_only and both targets", gotReq)
		}
		if c := gotReq.GetCitations(); len(c) != 1 || c[0].GetKind() != "file" || c[0].GetRef() != "a.go:12" {
			t.Errorf("citations = %v, want file:a.go:12 split at the first colon", c)
		}
		if !strings.HasPrefix(stdout, "validated: 2 target(s), nothing written") || !strings.Contains(stdout, "TARGET0002") {
			t.Errorf("stdout = %q, want the validated headline and the resolved targets", stdout)
		}

		resetCommandFlagState(t, supersedeCmd)
		jsonOut, _, err := runClient(t, append(append([]string{}, base...), "--server", url, "--supersedes", "TARGET0001", "--validate-only", "--output", "json")...)
		if err != nil {
			t.Fatalf("json lane: %v", err)
		}
		doc := decodeJSONDoc(t, jsonOut)
		if string(doc["validated"]) != "true" {
			t.Errorf(`json validated = %s, want true`, doc["validated"])
		}
	})

	t.Run("no targets is rejected with the envelope before any request", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, supersedeCmd)
		svc := &stubEngramService{supersedeFn: func(context.Context, *engramv1.SupersedeMemoryRequest) (*engramv1.SupersedeMemoryResponse, error) {
			return &engramv1.SupersedeMemoryResponse{}, nil
		}}
		url := startStubServer(t, svc)
		_, _, err := runClient(t, append(append([]string{}, base...), "--server", url)...)
		assertEnvelopePassthrough(t, err, "field=supersedes hint=required")
		if svc.writeCalls["SupersedeMemory"] != 0 {
			t.Error("a request was sent despite no targets")
		}
	})

	t.Run("rule target exits 2 and non-owned target exits 4", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			err  error
			want int
		}{
			{"rule", invalidArg("field=supersedes hint=not_applicable: rules cannot be superseded (delete the rule instead): RULE000001"), exitUsage},
			{"non-owned", connect.NewError(connect.CodeNotFound, errors.New("supersedes: not found: THEIRS00001")), exitNotFound},
		} {
			t.Run(tc.name, func(t *testing.T) {
				resetClientFlags(t)
				resetCommandFlagState(t, supersedeCmd)
				svc := &stubEngramService{supersedeFn: func(context.Context, *engramv1.SupersedeMemoryRequest) (*engramv1.SupersedeMemoryResponse, error) {
					return nil, tc.err
				}}
				url := startStubServer(t, svc)
				_, _, err := runClient(t, append(append([]string{}, base...), "--server", url, "--supersedes", "X")...)
				assertExitCode(t, err, tc.want)
				if svc.writeCalls["SupersedeMemory"] != 1 {
					t.Errorf("SupersedeMemory calls = %d, want exactly 1 (no retry)", svc.writeCalls["SupersedeMemory"])
				}
			})
		}
	})

	t.Run("same key replays the first record", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, supersedeCmd)
		ledger := map[string]string{}
		next := 0
		svc := &stubEngramService{supersedeFn: func(_ context.Context, req *engramv1.SupersedeMemoryRequest) (*engramv1.SupersedeMemoryResponse, error) {
			if id, ok := ledger[req.GetIdempotencyKey()]; ok {
				return &engramv1.SupersedeMemoryResponse{Id: id, ShortId: "SAME000001"}, nil
			}
			next++
			id := "1111111" + strings.Repeat("1", 29-7+1)[:1] + "-0000-0000-0000-00000000000" + string(rune('0'+next))
			ledger[req.GetIdempotencyKey()] = id
			return &engramv1.SupersedeMemoryResponse{Id: id, ShortId: "SAME000001"}, nil
		}}
		url := startStubServer(t, svc)
		args := append(append([]string{}, base...), "--server", url, "--supersedes", "X", "--idempotency-key", "k", "--output", "json")
		first, _, err := runClient(t, args...)
		if err != nil {
			t.Fatalf("first call: %v", err)
		}
		resetCommandFlagState(t, supersedeCmd)
		second, _, err := runClient(t, args...)
		if err != nil {
			t.Fatalf("second call: %v", err)
		}
		if first != second {
			t.Errorf("replay returned a different document:\n%s\n%s", first, second)
		}
		if next != 1 {
			t.Errorf("records created = %d, want 1", next)
		}
	})
}

func TestStoreDiscovery(t *testing.T) {
	t.Run("stored id", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, storeDiscoveryCmd)
		var gotReq *engramv1.StoreDiscoveryRequest
		svc := &stubEngramService{storeDiscoveryFn: func(_ context.Context, req *engramv1.StoreDiscoveryRequest) (*engramv1.StoreDiscoveryResponse, error) {
			gotReq = req
			return &engramv1.StoreDiscoveryResponse{Id: "11111111-1111-1111-1111-111111111111", ShortId: "DISC000001"}, nil
		}}
		url := startStubServer(t, svc)
		stdout, _, err := runClient(t, "store-discovery", "--server", url, "--content", "auth flows through X", "--kind", "fact",
			"--citation", "file:internal/auth/token.go", "--scope", "discovery:repo:x", "--tags", "auth", "--summary", "s", "--id", "OLD0000001", "--output", "json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetKind() != "fact" || gotReq.GetScope() != "discovery:repo:x" || gotReq.GetId() != "OLD0000001" || len(gotReq.GetCitations()) != 1 || gotReq.GetCitations()[0].GetRef() != "internal/auth/token.go" {
			t.Errorf("request = %+v, want every flag passed through", gotReq)
		}
		doc := decodeJSONDoc(t, stdout)
		if string(doc["short_id"]) != `"DISC000001"` || doc["id"] == nil {
			t.Errorf("json = %q, want id and short_id", stdout)
		}
	})

	for _, tc := range []struct {
		name     string
		args     []string
		envelope string
	}{
		{"no citation", []string{"--content", "c", "--kind", "fact", "--scope", "discovery:repo:x"}, "field=citations hint=required: at least one citation is required"},
		{"wrong kind", []string{"--content", "c", "--kind", "guess", "--citation", "file:a.go", "--scope", "discovery:repo:x"}, "field=kind hint=enum: kind must be map or fact"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetClientFlags(t)
			resetCommandFlagState(t, storeDiscoveryCmd)
			svc := &stubEngramService{storeDiscoveryFn: func(context.Context, *engramv1.StoreDiscoveryRequest) (*engramv1.StoreDiscoveryResponse, error) {
				return nil, invalidArg(tc.envelope)
			}}
			url := startStubServer(t, svc)
			_, _, err := runClient(t, append([]string{"store-discovery", "--server", url}, tc.args...)...)
			assertEnvelopePassthrough(t, err, tc.envelope)
		})
	}
}

func TestStoreRule(t *testing.T) {
	t.Run("stored id", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, storeRuleCmd)
		var gotReq *engramv1.StoreRuleRequest
		svc := &stubEngramService{storeRuleFn: func(_ context.Context, req *engramv1.StoreRuleRequest) (*engramv1.StoreRuleResponse, error) {
			gotReq = req
			return &engramv1.StoreRuleResponse{Id: "11111111-1111-1111-1111-111111111111", ShortId: "RULE000001"}, nil
		}}
		url := startStubServer(t, svc)
		stdout, _, err := runClient(t, "store-rule", "--server", url, "--content", "MUST pass a pathspec", "--scope", "rule:repo:x",
			"--summary", "MUST pass a pathspec to git commit", "--tags", "git", "--output", "text")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetScope() != "rule:repo:x" || gotReq.GetSummary() != "MUST pass a pathspec to git commit" || gotReq.GetTags()[0] != "git" {
			t.Errorf("request = %+v, want every flag passed through", gotReq)
		}
		if !strings.Contains(stdout, "stored rule: RULE000001 (11111111-1111-1111-1111-111111111111)") {
			t.Errorf("stdout = %q, want the id line", stdout)
		}
	})

	for _, tc := range []struct {
		name     string
		args     []string
		envelope string
	}{
		{"multi-line summary", []string{"--content", "c", "--scope", "rule:repo:x", "--summary", "a\nb"}, "field=summary hint=format: rule summary must be a single line"},
		{"wrong scope prefix", []string{"--content", "c", "--scope", "repo:x", "--summary", "s"}, "field=scope hint=prefix: scope must be rule:repo:<repo> or rule:project:<project>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetClientFlags(t)
			resetCommandFlagState(t, storeRuleCmd)
			svc := &stubEngramService{storeRuleFn: func(context.Context, *engramv1.StoreRuleRequest) (*engramv1.StoreRuleResponse, error) {
				return nil, invalidArg(tc.envelope)
			}}
			url := startStubServer(t, svc)
			_, _, err := runClient(t, append([]string{"store-rule", "--server", url}, tc.args...)...)
			assertEnvelopePassthrough(t, err, tc.envelope)
		})
	}
}

func TestDelete(t *testing.T) {
	// deleteStub is a one-record store: get answers while the record
	// exists, delete removes it, and a non-owned id reads not found on
	// both RPCs.
	newDeleteStub := func() *stubEngramService {
		exists := true
		svc := &stubEngramService{}
		svc.getFn = func(_ context.Context, req *engramv1.GetMemoryRequest) (*engramv1.GetMemoryResponse, error) {
			if req.GetId() != "MINE000001" || !exists {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("not found: "+req.GetId()))
			}
			return &engramv1.GetMemoryResponse{Memory: &engramv1.Memory{ShortId: "MINE000001", Scope: "repo:x", Summary: "mine"}}, nil
		}
		svc.deleteFn = func(_ context.Context, req *engramv1.DeleteMemoryRequest) (*engramv1.DeleteMemoryResponse, error) {
			if req.GetId() != "MINE000001" || !exists {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("not found: "+req.GetId()))
			}
			exists = false
			return &engramv1.DeleteMemoryResponse{}, nil
		}
		return svc
	}

	t.Run("preview leaves the record in place", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, deleteCmd)
		svc := newDeleteStub()
		url := startStubServer(t, svc)
		stdout, _, err := runClient(t, "delete", "MINE000001", "--server", url, "--output", "text")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(stdout, "would delete record MINE000001") || !strings.Contains(stdout, "mine") {
			t.Errorf("stdout = %q, want the would-delete headline and the record's summary", stdout)
		}
		if svc.writeCalls["DeleteMemory"] != 0 || svc.getCalls != 1 {
			t.Errorf("DeleteMemory calls = %d, GetMemory calls = %d; want 0 and 1", svc.writeCalls["DeleteMemory"], svc.getCalls)
		}
		resetCommandFlagState(t, deleteCmd)
		jsonOut, _, err := runClient(t, "delete", "MINE000001", "--server", url, "--output", "json")
		if err != nil {
			t.Fatalf("json lane: %v", err)
		}
		doc := decodeJSONDoc(t, jsonOut)
		if string(doc["deleted"]) != "false" || doc["memory"] == nil {
			t.Errorf("json = %q, want deleted:false with the memory", jsonOut)
		}
		resetCommandFlagState(t, getCmd)
		if _, _, err := runClient(t, "get", "MINE000001", "--server", url); err != nil {
			t.Errorf("get after preview: %v, want the record still present", err)
		}
	})

	t.Run("apply deletes once and a later get exits 4", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, deleteCmd)
		svc := newDeleteStub()
		url := startStubServer(t, svc)
		stdout, _, err := runClient(t, "delete", "MINE000001", "--server", url, "--apply", "--output", "json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		doc := decodeJSONDoc(t, stdout)
		if string(doc["deleted"]) != "true" {
			t.Errorf("json = %q, want deleted:true", stdout)
		}
		if svc.writeCalls["DeleteMemory"] != 1 {
			t.Errorf("DeleteMemory calls = %d, want exactly 1", svc.writeCalls["DeleteMemory"])
		}
		resetCommandFlagState(t, getCmd)
		_, _, err = runClient(t, "get", "MINE000001", "--server", url)
		assertExitCode(t, err, exitNotFound)
	})

	t.Run("not owned exits 4 on both paths with no UUID echo", func(t *testing.T) {
		for _, extra := range [][]string{nil, {"--apply"}} {
			resetClientFlags(t)
			resetCommandFlagState(t, deleteCmd)
			svc := newDeleteStub()
			url := startStubServer(t, svc)
			stdout, _, err := runClient(t, append([]string{"delete", "THEIRS00001", "--server", url}, extra...)...)
			assertExitCode(t, err, exitNotFound)
			if stdout != "" || strings.Contains(err.Error(), hiddenUUID) {
				t.Errorf("args=%v stdout=%q err=%q, want no stdout and no UUID echo", extra, stdout, err.Error())
			}
		}
	})
}
