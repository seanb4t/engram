// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/seanb4t/engram/internal/authz"
)

// captureDebugLog installs a JSON slog handler at debug level — the idiom
// this repo already ships at internal/auth/auth_test.go:50-62 — and restores
// the previous default logger on cleanup. The explicit debug level matters:
// the default handler level is Info and would swallow every decision-log
// line under test, producing a green run that proves nothing.
func captureDebugLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// decodeLogLines parses each newline-delimited JSON log record in buf.
func decodeLogLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatalf("decode log line %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

// TestDecideBucketLogsAllowAndDeny proves decideBucket — one of the two
// chokepoints every production Decision consumption funnels through — emits
// exactly one debug-level line per call, unconditionally on both the allow
// and the deny arm (D-04), carrying the decision, action, bucket token, and
// the satisfied policy ids. Both arms live in one table so a regression that
// drops one arm's emission is a table failure, not a silently uncovered
// path.
//
// decideBucket is exercised directly (not through Search/List) against a
// Store built with New(nil, ...): decideBucket never touches s.client, and
// s.authz defaults to authz.MustDefault() — the real embedded policy corpus
// — so "alice" reading her own BucketOwn is a genuine allow (own-records)
// and an anonymous caller probing BucketShared is a genuine deny
// (defense-empty-owner fires), no live Qdrant required.
func TestDecideBucketLogsAllowAndDeny(t *testing.T) {
	cases := []struct {
		name             string
		owner            string
		bucket           authz.Bucket
		wantAllow        bool
		wantBucket       string
		wantNonEmptyPIDs bool
	}{
		{name: "allow", owner: "alice", bucket: authz.BucketOwn, wantAllow: true, wantBucket: "own", wantNonEmptyPIDs: true},
		{name: "deny", owner: "", bucket: authz.BucketShared, wantAllow: false, wantBucket: "shared", wantNonEmptyPIDs: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := New(nil, "decisionlog-bucket-test")
			buf := captureDebugLog(t)

			_ = s.decideBucket(context.Background(), tc.owner, "human", authz.ActionRead, tc.bucket)

			lines := decodeLogLines(t, buf)
			if len(lines) != 1 {
				t.Fatalf("got %d log lines, want exactly 1: %v", len(lines), lines)
			}
			line := lines[0]

			if got, ok := line["allow"].(bool); !ok || got != tc.wantAllow {
				t.Errorf("allow = %v, want %v", line["allow"], tc.wantAllow)
			}
			if got, ok := line["action"].(string); !ok || got != string(authz.ActionRead) {
				t.Errorf("action = %v, want %q", line["action"], authz.ActionRead)
			}
			if got, ok := line["bucket"].(string); !ok || got != tc.wantBucket {
				t.Errorf("bucket = %v, want %q (a readable token, not a raw int)", line["bucket"], tc.wantBucket)
			}
			pids, ok := line["policy_ids"].([]any)
			if !ok {
				t.Fatalf("policy_ids field missing or wrong shape: %v", line["policy_ids"])
			}
			if tc.wantNonEmptyPIDs && len(pids) == 0 {
				t.Error("want a non-empty policy_ids for the allow arm")
			}
			if _, ok := line["policy_error_count"]; !ok {
				t.Error("missing policy_error_count field")
			}
		})
	}
}

// TestDecideRecordLogsBothArms is TestDecideBucketLogsAllowAndDeny's twin at
// the id-addressed chokepoint (decideRecord), with no `bucket` field expected
// — a per-record decision has no bucket (D-02's field list names bucket for
// the bucket arm only).
func TestDecideRecordLogsBothArms(t *testing.T) {
	cases := []struct {
		name             string
		owner            string
		memoryOwner      string
		visibility       string
		action           authz.Action
		wantAllow        bool
		wantNonEmptyPIDs bool
	}{
		// alice reading her own record: own-records fires, allow.
		{name: "allow", owner: "alice", memoryOwner: "alice", visibility: "", action: authz.ActionRead, wantAllow: true, wantNonEmptyPIDs: true},
		// alice writing bob's shared record: shared-read is read-only, so no
		// permit policy fires — implicit deny.
		{name: "deny", owner: "alice", memoryOwner: "bob", visibility: "shared", action: authz.ActionWrite, wantAllow: false, wantNonEmptyPIDs: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := New(nil, "decisionlog-record-test")
			buf := captureDebugLog(t)

			_ = s.decideRecord(context.Background(), tc.owner, "human", tc.action, tc.memoryOwner, "gotcha", tc.visibility, "scope1")

			lines := decodeLogLines(t, buf)
			if len(lines) != 1 {
				t.Fatalf("got %d log lines, want exactly 1: %v", len(lines), lines)
			}
			line := lines[0]

			if got, ok := line["allow"].(bool); !ok || got != tc.wantAllow {
				t.Errorf("allow = %v, want %v", line["allow"], tc.wantAllow)
			}
			if got, ok := line["action"].(string); !ok || got != string(tc.action) {
				t.Errorf("action = %v, want %q", line["action"], tc.action)
			}
			if _, ok := line["bucket"]; ok {
				t.Errorf("record-arm log line must not carry a bucket field, got %v", line["bucket"])
			}
			pids, ok := line["policy_ids"].([]any)
			if !ok {
				t.Fatalf("policy_ids field missing or wrong shape: %v", line["policy_ids"])
			}
			if tc.wantNonEmptyPIDs && len(pids) == 0 {
				t.Error("want a non-empty policy_ids for the allow arm")
			}
			if _, ok := line["policy_error_count"]; !ok {
				t.Error("missing policy_error_count field")
			}
		})
	}
}

// TestDecisionLogFieldSetIsExact double-checks decideBucket/decideRecord's
// emitted key set from the store side (complementing
// authz.TestDecisionLogNeverLeaksExpressionTrace, which pins the Log()
// boundary itself): the log line for each chokepoint carries exactly its
// documented fields, so a future call site cannot smuggle an extra
// caller-adjacent value (owner, memory owner, scope, category, visibility)
// into the emission without this test failing.
func TestDecisionLogFieldSetIsExact(t *testing.T) {
	bucketWant := map[string]bool{
		"time": true, "level": true, "msg": true,
		"allow": true, "action": true, "bucket": true,
		"policy_ids": true, "policy_error_count": true,
	}
	recordWant := map[string]bool{
		"time": true, "level": true, "msg": true,
		"allow": true, "action": true,
		"policy_ids": true, "policy_error_count": true,
	}

	t.Run("bucket", func(t *testing.T) {
		s := New(nil, "decisionlog-fieldset-bucket-test")
		buf := captureDebugLog(t)
		_ = s.decideBucket(context.Background(), "alice", "human", authz.ActionRead, authz.BucketOwn)
		lines := decodeLogLines(t, buf)
		if len(lines) != 1 {
			t.Fatalf("got %d log lines, want exactly 1", len(lines))
		}
		got := map[string]bool{}
		for k := range lines[0] {
			got[k] = true
		}
		if len(got) != len(bucketWant) {
			t.Fatalf("bucket-arm field set = %v, want %v", got, bucketWant)
		}
		for k := range bucketWant {
			if !got[k] {
				t.Errorf("bucket-arm log line missing field %q: %v", k, got)
			}
		}
	})

	t.Run("record", func(t *testing.T) {
		s := New(nil, "decisionlog-fieldset-record-test")
		buf := captureDebugLog(t)
		_ = s.decideRecord(context.Background(), "alice", "human", authz.ActionRead, "alice", "gotcha", "", "scope1")
		lines := decodeLogLines(t, buf)
		if len(lines) != 1 {
			t.Fatalf("got %d log lines, want exactly 1", len(lines))
		}
		got := map[string]bool{}
		for k := range lines[0] {
			got[k] = true
		}
		if len(got) != len(recordWant) {
			t.Fatalf("record-arm field set = %v, want %v", got, recordWant)
		}
		for k := range recordWant {
			if !got[k] {
				t.Errorf("record-arm log line missing field %q: %v", k, got)
			}
		}
	})
}
