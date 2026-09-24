// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package curationeval

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/seanb4t/engram/internal/decide/jev"
	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/verdict"
)

// stubRelationResponse is a canned Decisions API reply: relation "duplicate"
// at 0.95 (all five relation probabilities populated) and same_subject at
// 0.9. Matches wireResponse's shape in internal/decide/jev/wire.go.
const stubRelationResponse = `{
	"model": "typesafe/jev-1.13-stub",
	"answers": {
		"relation": {
			"type": "choice",
			"choice": "duplicate",
			"probabilities": {"duplicate": 0.95, "contradicts": 0.01, "updates": 0.01, "related": 0.02, "unrelated": 0.01}
		},
		"same_subject": {"type": "noul", "noul": 0.9}
	}
}`

// TestEvaluateAgainstStubProvider is hermetic: an httptest Jev endpoint
// stands in for a real Decisions provider, recording every request body it
// receives. It proves evaluate sends exactly one request per pair, each
// carrying record_a/record_b equal to verdict.State(...) and the shared
// five-option relation question (D-06), and that the returned predictions
// are gold-labeled in pair order.
func TestEvaluateAgainstStubProvider(t *testing.T) {
	t.Parallel()

	var (
		mu       sync.Mutex
		requests []map[string]any
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		requests = append(requests, body)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(stubRelationResponse))
	}))
	defer srv.Close()

	// Concurrency 1 keeps the request order deterministic (matching
	// syntheticPairs' order) so the per-request assertions below can index
	// requests[i] against syntheticPairs[i] directly.
	dec := jev.New(srv.URL, "k", "m", jev.WithConcurrency(1))

	const stateChars = verdict.DefaultStateChars
	const threshold = verdict.DefaultThreshold

	preds := evaluate(context.Background(), dec, syntheticPairs, threshold, stateChars)

	if len(preds) != len(syntheticPairs) {
		t.Fatalf("len(preds) = %d, want %d", len(preds), len(syntheticPairs))
	}
	for i, p := range preds {
		if p.gold != syntheticPairs[i].label {
			t.Errorf("preds[%d].gold = %q, want %q", i, p.gold, syntheticPairs[i].label)
		}
		if p.v.Failed() {
			t.Errorf("preds[%d].v.ErrorClass = %q, want a decided verdict", i, p.v.ErrorClass)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) != len(syntheticPairs) {
		t.Fatalf("recorded %d HTTP requests, want %d (exactly one per pair)", len(requests), len(syntheticPairs))
	}
	for i, req := range requests {
		state, ok := req["state"].(map[string]any)
		if !ok {
			t.Fatalf("request %d: state missing or wrong type: %v", i, req["state"])
		}
		wantA := verdict.State("", syntheticPairs[i].recordA, stateChars)
		wantB := verdict.State("", syntheticPairs[i].recordB, stateChars)
		if got, _ := state["record_a"].(string); got != wantA {
			t.Errorf("request %d record_a = %q, want %q", i, got, wantA)
		}
		if got, _ := state["record_b"].(string); got != wantB {
			t.Errorf("request %d record_b = %q, want %q", i, got, wantB)
		}

		questions, ok := req["questions"].(map[string]any)
		if !ok {
			t.Fatalf("request %d: questions missing or wrong type: %v", i, req["questions"])
		}
		relationQ, ok := questions["relation"].(map[string]any)
		if !ok {
			t.Fatalf("request %d: relation question missing: %v", i, questions)
		}
		if relationQ["type"] != "choice" {
			t.Errorf("request %d: relation question type = %v, want %q", i, relationQ["type"], "choice")
		}
		criteria, ok := relationQ["criteria"].(map[string]any)
		if !ok || len(criteria) != 5 {
			t.Errorf("request %d: relation criteria = %v, want the 5 verdict.Relations() options", i, relationQ["criteria"])
		}
	}
}

// TestCurationEval is the gated live test (task eval:curation): it sends
// syntheticPairs through a real Decisions provider and requires at least
// one scored verdict. Off by default (D-15) — see resolveEvalGate.
func TestCurationEval(t *testing.T) {
	enabled, pairsPath, gerr := curationEvalEnabled()
	if gerr != nil {
		t.Fatalf("%v", gerr)
	}
	if !enabled {
		t.Skip("set ENGRAM_CURATION_EVAL=1 plus ENGRAM_DECISIONS_PROVIDER, ENGRAM_DECISIONS_BASE_URL and a key to run the curation eval (task eval:curation)")
	}

	dec, settings, err := server.DeciderFromEnv()
	if err != nil {
		t.Fatalf("server.DeciderFromEnv: %v", err)
	}
	if dec == nil {
		t.Fatal("ENGRAM_DECISIONS_PROVIDER is empty: required to run the curation eval")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	t.Run("committed", func(t *testing.T) {
		preds := evaluate(ctx, dec, syntheticPairs, settings.Threshold, settings.StateChars)

		scored := 0
		for _, p := range preds {
			if !p.v.Failed() {
				scored++
			}
		}
		if scored == 0 {
			t.Fatal("no verdict was scored")
		}

		for _, line := range formatReport("committed", preds, settings.Threshold) {
			t.Log(line)
		}

		// D-03's single hard gate: the threshold must be validated on the
		// committed corpus, or `task eval:curation` must exit non-zero.
		_, _, result := thresholdGate(preds, settings.Threshold)
		if result != "PASS" {
			t.Errorf("gate result = %s, want PASS (threshold %.3f not validated on the committed corpus)", result, settings.Threshold)
		}
	})

	// D-01's private real-spine mode: only runs when
	// ENGRAM_CURATION_EVAL_PAIRS names a local file. Aggregates only — no
	// D-03 gate on this corpus (D-03 gates the committed set only).
	if pairsPath != "" {
		t.Run("local", func(t *testing.T) {
			pairs, lerr := loadLocalPairs(pairsPath)
			if lerr != nil {
				t.Fatalf("loadLocalPairs: %v", lerr)
			}

			preds := evaluate(ctx, dec, pairs, settings.Threshold, settings.StateChars)

			scored := 0
			for _, p := range preds {
				if !p.v.Failed() {
					scored++
				}
			}
			if scored == 0 {
				t.Fatal("no verdict was scored")
			}

			for _, line := range formatReport("local", preds, settings.Threshold) {
				t.Log(line)
			}
		})
	}
}
