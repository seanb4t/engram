// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/decide"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/understand"
)

// understandAuditSentinel is a non-ASCII sentinel query with no quotes,
// angle brackets or ampersands, so it survives slog's JSON handler
// byte-for-byte (NLQ-04 proof, this file's tests).
const understandAuditSentinel = "Ωmega-sentinel ünderstand zeta"

// newUnderstandAuditScriptedDecider answers gotcha 0.95, decision 0.93, the
// other two categories at 0.1, and time_window "none" — this file's fixed
// fixture for the audit-line and no-text-without-audit tests.
func newUnderstandAuditScriptedDecider() *scriptedDecider {
	return &scriptedDecider{resp: decide.Response{Answers: map[string]decide.Answer{
		understand.CategoryQuestion("convention"): {Type: decide.QuestionNoul, Probability: 0.1},
		understand.CategoryQuestion("gotcha"):     {Type: decide.QuestionNoul, Probability: 0.95},
		understand.CategoryQuestion("decision"):   {Type: decide.QuestionNoul, Probability: 0.93},
		understand.CategoryQuestion("preference"): {Type: decide.QuestionNoul, Probability: 0.1},
		understand.QuestionTimeWindow: {
			Type:   decide.QuestionChoice,
			Choice: "none",
			Probabilities: map[string]float64{
				"none":       0.97,
				"today":      0.01,
				"past_week":  0.01,
				"past_month": 0.005,
				"past_year":  0.005,
			},
		},
	}}}
}

// captureSlogJSON installs a slog.NewJSONHandler-backed slog.Default writing
// to buf, restored via t.Cleanup. Unlike responsetoolarge_test.go's
// captureSlog (a structured slogRecorder), this file needs the raw JSON
// handler itself — the sentinel's byte-for-byte survival and the record's
// exact top-level key set (including "time"/"level"/"msg") are the thing
// under test, mirroring rankReport.audit's own capture harness. Must be
// called BEFORE mountConnect (understandTestClient), since
// newConnectAccessLogInterceptor captures slog.Default() at mount time.
func captureSlogJSON(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// logRecordsWithMsg parses out's newline-delimited JSON log lines and
// returns every record whose "msg" field equals msg.
func logRecordsWithMsg(t *testing.T, out, msg string) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("unmarshal log line %q: %v", line, err)
		}
		if m["msg"] == msg {
			records = append(records, m)
		}
	}
	return records
}

// TestUnderstandQueryAuditLogsQueryVerbatim proves D-16's runtime half
// (Task 1): with ENGRAM_SEARCH_UNDERSTANDING_AUDIT on, one understood query
// emits exactly one "query understanding audit" Info record carrying the
// sentinel query verbatim, the response's suggestion labels in order, and
// nothing else; with the flag off, no such record is emitted and the
// sentinel appears nowhere in the captured output.
func TestUnderstandQueryAuditLogsQueryVerbatim(t *testing.T) {
	t.Run("flag on", func(t *testing.T) {
		buf := captureSlogJSON(t)

		d, sp := newSpyDeps()
		sp.tags = []store.TagCount{{Tag: "zeta", Count: 3}}
		d.understandDec = newUnderstandAuditScriptedDecider()
		d.understandAudit = true
		client := understandTestClient(t, d)

		req := connect.NewRequest(&engramv1.UnderstandQueryRequest{Query: understandAuditSentinel})
		req.Header().Set("X-Test-Actor", "actor-A")
		resp, err := client.UnderstandQuery(context.Background(), req)
		if err != nil {
			t.Fatalf("UnderstandQuery: %v", err)
		}

		suggestions := resp.Msg.GetSuggestions()
		if len(suggestions) != 3 {
			t.Fatalf("len(Suggestions) = %d, want 3: %v", len(suggestions), suggestions)
		}
		if got := suggestions[0]; got.GetCategory() != "gotcha" {
			t.Errorf("suggestions[0] = %+v, want category=gotcha", got)
		}
		if got := suggestions[1]; got.GetCategory() != "decision" {
			t.Errorf("suggestions[1] = %+v, want category=decision", got)
		}
		if got := suggestions[2]; got.GetTag() != "zeta" {
			t.Errorf("suggestions[2] = %+v, want tag=zeta", got)
		}

		records := logRecordsWithMsg(t, buf.String(), "query understanding audit")
		if len(records) != 1 {
			t.Fatalf("len(audit records) = %d, want 1: %v", len(records), records)
		}
		rec := records[0]

		if got, ok := rec["query"].(string); !ok || got != understandAuditSentinel {
			t.Errorf("query = %v, want %q byte-for-byte", rec["query"], understandAuditSentinel)
		}

		labelsJSON, ok := rec["suggestions"].(string)
		if !ok {
			t.Fatalf("suggestions field is not a string: %v (%T)", rec["suggestions"], rec["suggestions"])
		}
		var labels []string
		if err := json.Unmarshal([]byte(labelsJSON), &labels); err != nil {
			t.Fatalf("unmarshal suggestions %q: %v", labelsJSON, err)
		}
		wantLabels := []string{"category:gotcha", "category:decision", "tag:zeta"}
		if !slices.Equal(labels, wantLabels) {
			t.Errorf("suggestion labels = %v, want %v (response order)", labels, wantLabels)
		}

		wantKeys := []string{"time", "level", "msg", "query", "outcome", "questions_asked", "suggestions"}
		if len(rec) != len(wantKeys) {
			t.Errorf("record key set = %v, want exactly %v", keysOf(rec), wantKeys)
		}
		for _, k := range wantKeys {
			if _, ok := rec[k]; !ok {
				t.Errorf("record missing key %q: %v", k, rec)
			}
		}

		if got := rec["level"]; got != "INFO" {
			t.Errorf("level = %v, want INFO", got)
		}
		if got := rec["outcome"]; got != string(understand.OutcomeDecided) {
			t.Errorf("outcome = %v, want %q", got, understand.OutcomeDecided)
		}
	})

	t.Run("flag off", func(t *testing.T) {
		buf := captureSlogJSON(t)

		d, sp := newSpyDeps()
		sp.tags = []store.TagCount{{Tag: "zeta", Count: 3}}
		d.understandDec = newUnderstandAuditScriptedDecider()
		d.understandAudit = false
		client := understandTestClient(t, d)

		req := connect.NewRequest(&engramv1.UnderstandQueryRequest{Query: understandAuditSentinel})
		req.Header().Set("X-Test-Actor", "actor-A")
		if _, err := client.UnderstandQuery(context.Background(), req); err != nil {
			t.Fatalf("UnderstandQuery: %v", err)
		}

		out := buf.String()
		if records := logRecordsWithMsg(t, out, "query understanding audit"); len(records) != 0 {
			t.Errorf("audit records = %v, want none (flag off)", records)
		}
		if strings.Contains(out, understandAuditSentinel) {
			t.Errorf("log output contains the sentinel query, want none: %q", out)
		}
	})
}

// keysOf returns m's keys, for a readable test-failure message.
func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
