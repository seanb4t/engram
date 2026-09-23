// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package jev

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/v2"
	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/decide"
)

// liveGateEnv is the env var gating TestJevLive. It is test-only: never
// added to internal/config's registry (mirrors
// internal/retrievaleval.resolveEvalGate's rationale for
// ENGRAM_RETRIEVAL_EVAL).
const liveGateEnv = "ENGRAM_DECISIONS_LIVE"

// liveGateKey is the koanf key liveGate reads after the env layer.
const liveGateKey = "decisions_live"

// liveGate resolves ENGRAM_DECISIONS_LIVE through a package-local koanf
// load: a default-false layer, then production's ENGRAM_ prefix and env
// layer with the same empty-value-preserves-default rule config.Load uses
// (mirrors internal/retrievaleval.resolveEvalGate exactly). A malformed
// value errors rather than reading as off, so a mistyped enable is never a
// silent skip.
func liveGate(environ func() []string) (bool, error) {
	k := koanf.New(".")

	if err := k.Load(confmap.Provider(map[string]any{liveGateKey: "false"}, "."), nil); err != nil {
		return false, fmt.Errorf("decisions live gate defaults: %w", err)
	}

	if err := k.Load(env.Provider(".", env.Opt{
		Prefix:      config.Prefix,
		EnvironFunc: environ,
		TransformFunc: func(key, val string) (string, any) {
			if key != liveGateEnv || val == "" {
				// Empty preserves the default (mirrors config.Load), and
				// every other ENGRAM_* var is ignored: this gate maps
				// exactly one variable, never the whole registry.
				return "", nil
			}
			return liveGateKey, val
		},
	}), nil); err != nil {
		return false, fmt.Errorf("decisions live gate env: %w", err)
	}

	raw := k.String(liveGateKey)
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s: invalid value %q: %w", liveGateEnv, raw, err)
	}
	return enabled, nil
}

// TestJevLive is an opt-in live smoke test against a real Decisions
// endpoint (DEC-03's manual-only verification, run via `task
// eval:decisions`). It sends one synthetic noul+choice+score request and
// asserts typed answers, usage and a typesafe/jev-1.13 snapshot. Never logs
// the API key.
func TestJevLive(t *testing.T) {
	enabled, gerr := liveGate(os.Environ)
	if gerr != nil {
		t.Fatalf("%v", gerr)
	}
	if !enabled {
		t.Skip("set ENGRAM_DECISIONS_LIVE=1 plus ENGRAM_DECISIONS_BASE_URL and a key to run the live Decisions smoke test (task eval:decisions)")
	}

	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.Decisions.BaseURL == "" {
		t.Fatal("ENGRAM_DECISIONS_BASE_URL is empty: required to run the live Decisions smoke test")
	}

	apiKey := cmp.Or(cfg.Decisions.APIKey, cfg.OpenAI.APIKey)
	c := New(cfg.Decisions.BaseURL, apiKey, cfg.Decisions.Model, WithTimeout(30*time.Second))

	req := decide.Request{
		State: decide.State{
			"ticket": "My checkout page shows a blank screen after I click Pay. I have tried two browsers.",
		},
		Questions: map[string]decide.Question{
			"is_bug": decide.Noul(
				"Does this ticket describe a software bug?",
				"the ticket describes broken or unexpected software behavior",
				"the ticket does not describe a software bug",
			),
			"team": decide.Choice("Which team should triage this ticket?", map[string]string{
				"account":  "account management, login, or billing issues",
				"frontend": "UI rendering, layout, or client-side behavior issues",
				"payments": "checkout, payment processing, or transaction issues",
			}),
			"urgency": decide.Score("How urgent is this ticket?", []string{
				"can wait",
				"this week",
				"blocking",
			}),
		},
	}

	start := time.Now()
	resp, err := c.Decide(context.Background(), req)
	latency := time.Since(start)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}

	if len(resp.Answers) != 3 {
		t.Fatalf("len(Answers) = %d, want 3 (got %v)", len(resp.Answers), resp.Answers)
	}

	isBug, ok := resp.Answers["is_bug"]
	if !ok {
		t.Fatal("is_bug answer missing")
	}
	if isBug.Type != decide.QuestionNoul {
		t.Errorf("is_bug.Type = %v, want noul", isBug.Type)
	}
	if isBug.Probability < 0 || isBug.Probability > 1 {
		t.Errorf("is_bug.Probability = %v, want in [0, 1]", isBug.Probability)
	}

	team, ok := resp.Answers["team"]
	if !ok {
		t.Fatal("team answer missing")
	}
	if team.Type != decide.QuestionChoice {
		t.Errorf("team.Type = %v, want choice", team.Type)
	}
	switch team.Choice {
	case "account", "frontend", "payments":
	default:
		t.Errorf("team.Choice = %q, want one of account/frontend/payments", team.Choice)
	}
	for opt, p := range team.Probabilities {
		if p < 0 || p > 1 {
			t.Errorf("team.Probabilities[%q] = %v, want in [0, 1]", opt, p)
		}
	}

	urgency, ok := resp.Answers["urgency"]
	if !ok {
		t.Fatal("urgency answer missing")
	}
	if urgency.Type != decide.QuestionScore {
		t.Errorf("urgency.Type = %v, want score", urgency.Type)
	}
	if urgency.Score < 0 || urgency.Score > 2 {
		t.Errorf("urgency.Score = %v, want in [0, 2]", urgency.Score)
	}

	if resp.Usage == nil {
		t.Fatal("Usage is nil, want non-nil")
	}
	if resp.Usage.InputTokens <= 0 {
		t.Errorf("Usage.InputTokens = %d, want > 0", resp.Usage.InputTokens)
	}

	const wantModelPrefix = "typesafe/jev-1.13"
	if len(resp.Model) < len(wantModelPrefix) || resp.Model[:len(wantModelPrefix)] != wantModelPrefix {
		t.Errorf("Model = %q, want prefix %q", resp.Model, wantModelPrefix)
	}

	host := cfg.Decisions.BaseURL
	if u, perr := url.Parse(cfg.Decisions.BaseURL); perr == nil {
		host = u.Host // never u.User: userinfo must not reach a log line
	}
	var costUSD float64
	if resp.Usage.CostUSD != nil {
		costUSD = *resp.Usage.CostUSD
	}
	t.Logf("host=%s model_snapshot=%s input_tokens=%d output_tokens=%d cost_usd=%v latency=%s",
		host, resp.Model, resp.Usage.InputTokens, resp.Usage.OutputTokens, costUSD, latency)
}
