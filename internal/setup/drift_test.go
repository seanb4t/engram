// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// codexGetEngramBearer is the package-level fixture base every codex
// registration test in this plan derives from via strings.Replace — the
// 04-RESEARCH.md verbatim `codex mcp get engram --json` shape (codex-cli
// 0.153.4), never a second hand-typed document.
const codexGetEngramBearer = `{"name":"engram","enabled":true,"disabled_reason":null,"transport":{"type":"streamable_http","url":"https://engram.example.com/mcp","bearer_token_env_var":"ENGRAM_TOKEN","http_headers":null,"env_http_headers":null,"http_headers_helper":null},"enabled_tools":null,"disabled_tools":null,"startup_timeout_sec":null,"tool_timeout_sec":null}`

// codexObservedLiteralHeader is .planning/phases/04-drift-detection-read-only/
// 04-OBSERVATIONS.md §"Codex — literal header (hand-edited)"'s VERBATIM
// `codex mcp get probe-literal-04 --json` capture (codex-cli 0.154.0,
// 2026-09-15), with ONLY "name":"probe-literal-04" rewritten to
// "name":"engram" so Observe's framing check passes — reused by
// codex_test.go's TestObserveCodexRegistration subtest for this shape and
// this file's TestRedactionUnconditional subtest for the same shape, so
// both quote the SAME fixture (D-08).
const codexObservedLiteralHeader = `{
  "name": "engram",
  "enabled": true,
  "disabled_reason": null,
  "transport": {
    "type": "streamable_http",
    "url": "http://127.0.0.1:1/mcp",
    "bearer_token_env_var": "DUMMY_04",
    "http_headers": {
      "x-litellm-api-key": "sk-DO-NOT-COMMIT-literal-test-abc123"
    },
    "env_http_headers": null,
    "http_headers_helper": null
  },
  "enabled_tools": null,
  "disabled_tools": null,
  "startup_timeout_sec": null,
  "tool_timeout_sec": null
}`

// TestPreviewClassifiesRegistration drives Preview end-to-end through the
// scripted Run fake, proving the full observe -> compare -> classify ->
// redact -> render pipeline (D-01, D-02, D-03) for codex.
func TestPreviewClassifiesRegistration(t *testing.T) {
	t.Run("codex/preserved-unrecognized-field", func(t *testing.T) {
		opts := Options{URL: "https://engram.example.com/mcp", Auth: "bearer"}
		stdout := strings.Replace(codexGetEngramBearer,
			`"enabled_tools"`,
			`"oauth_client_id":"SENTINEL-LITERAL-9f3e2a-DO-NOT-LEAK","enabled_tools"`, 1)

		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: stdout}},
		), "codex")

		res := Preview(context.Background(), env, Codex, opts)

		if res.Outcome != OutcomePreserved {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomePreserved)
		}
		if res.Facets != "unrecognized-content" {
			t.Errorf("Facets = %q, want %q", res.Facets, "unrecognized-content")
		}
		wantReason := "codex: preserved: unrecognized-content: oauth_client_id; " + codexWholeEntryNote + "; " + codexManualRemediation
		if res.Reason != wantReason {
			t.Errorf("Reason = %q, want %q", res.Reason, wantReason)
		}
		if res.Drift != "unrecognized-content: oauth_client_id" {
			t.Errorf("Drift = %q, want %q", res.Drift, "unrecognized-content: oauth_client_id")
		}
		wantRegistered := "url=https://engram.example.com/mcp auth=bearer headers=none unrecognized=oauth_client_id"
		if res.Registered != wantRegistered {
			t.Errorf("Registered = %q, want %q", res.Registered, wantRegistered)
		}
		wantCommand := "codex mcp add engram --url https://engram.example.com/mcp --bearer-token-env-var ENGRAM_TOKEN"
		if res.Command != wantCommand {
			t.Errorf("Command = %q, want %q (a preserved row still shows what setup would have written)", res.Command, wantCommand)
		}
		if len(calls) != 1 {
			t.Fatalf("Run called %d times, want exactly 1: %+v", len(calls), calls)
		}
		wantArgs := []string{"mcp", "get", "engram", "--json"}
		if !reflect.DeepEqual(calls[0].Args, wantArgs) {
			t.Errorf("calls[0].Args = %q, want %q", calls[0].Args, wantArgs)
		}
		b, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if strings.Contains(string(b), "SENTINEL-LITERAL") {
			t.Errorf("json.Marshal(res) = %s, must not contain the sentinel literal", b)
		}
	})

	t.Run("codex/already-correct", func(t *testing.T) {
		opts := Options{URL: "https://engram.example.com/mcp", Auth: "bearer"}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: codexGetEngramBearer}},
		), "codex")

		res := Preview(context.Background(), env, Codex, opts)

		if res.Outcome != OutcomeAlreadyCorrect {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeAlreadyCorrect)
		}
		if res.Facets != "" {
			t.Errorf("Facets = %q, want empty", res.Facets)
		}
		if res.Drift != "" {
			t.Errorf("Drift = %q, want empty", res.Drift)
		}
		if res.Reason != "" {
			t.Errorf("Reason = %q, want empty", res.Reason)
		}
		wantRegistered := "url=https://engram.example.com/mcp auth=bearer headers=none"
		if res.Registered != wantRegistered {
			t.Errorf("Registered = %q, want %q", res.Registered, wantRegistered)
		}
		if len(calls) != 1 {
			t.Fatalf("Run called %d times, want exactly 1", len(calls))
		}
	})

	t.Run("codex/would-write-url", func(t *testing.T) {
		opts := Options{URL: "https://engram.example.com/mcp", Auth: "bearer"}
		stdout := strings.Replace(codexGetEngramBearer, `"url":"https://engram.example.com/mcp"`, `"url":"https://old.example/mcp"`, 1)
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: stdout}},
		), "codex")

		res := Preview(context.Background(), env, Codex, opts)

		if res.Outcome != OutcomeWouldWrite {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeWouldWrite)
		}
		if res.Facets != "url" {
			t.Errorf("Facets = %q, want %q", res.Facets, "url")
		}
		wantDrift := "url: observed https://old.example/mcp, would write https://engram.example.com/mcp"
		if res.Drift != wantDrift {
			t.Errorf("Drift = %q, want %q", res.Drift, wantDrift)
		}
		if res.Reason != "" {
			t.Errorf("Reason = %q, want empty", res.Reason)
		}
		if len(calls) != 1 {
			t.Fatalf("Run called %d times, want exactly 1", len(calls))
		}
	})

	t.Run("codex/would-write-auth-mode", func(t *testing.T) {
		opts := Options{URL: "https://engram.example.com/mcp", Auth: "oauth"}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: codexGetEngramBearer}},
		), "codex")

		res := Preview(context.Background(), env, Codex, opts)

		if res.Outcome != OutcomeWouldWrite {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeWouldWrite)
		}
		if res.Facets != "auth-mode" {
			t.Errorf("Facets = %q, want %q", res.Facets, "auth-mode")
		}
		if len(calls) != 1 {
			t.Fatalf("Run called %d times, want exactly 1", len(calls))
		}
	})

	t.Run("codex/preserved-enabled-false", func(t *testing.T) {
		opts := Options{URL: "https://engram.example.com/mcp", Auth: "bearer"}
		stdout := strings.Replace(codexGetEngramBearer, `"enabled":true`, `"enabled":false`, 1)
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: stdout}},
		), "codex")

		res := Preview(context.Background(), env, Codex, opts)

		if res.Outcome != OutcomePreserved {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomePreserved)
		}
		if !strings.Contains(res.Reason, "unrecognized-content: enabled") {
			t.Errorf("Reason = %q, want it to contain %q", res.Reason, "unrecognized-content: enabled")
		}
		if !strings.Contains(res.Reason, codexWholeEntryNote) {
			t.Errorf("Reason = %q, want it to contain %q", res.Reason, codexWholeEntryNote)
		}
		if len(calls) != 1 {
			t.Fatalf("Run called %d times, want exactly 1", len(calls))
		}
	})

	// claude-code: the second parsed runtime (04-05-PLAN.md Task 1),
	// exercising the SAME observe -> compare -> classify -> redact ->
	// render pipeline through claudeCodeRuntime.Observe (claudecode.go),
	// with fixtures built from
	// .planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md
	// (D-08) via claudecode_test.go's claudeGetFixture. Each subtest
	// scripts exactly ONE Run result — Preview issues a single probe —
	// with ExitCode 0, the RECORD's own observed failed-dial exit code
	// (claude mcp get exits 0 even when the dial itself fails), proving
	// parsing survives it (Pitfall 4).
	t.Run("claude-code", func(t *testing.T) {
		opts := Options{
			URL:     "https://engram.example.com/mcp",
			Auth:    "bearer",
			Headers: []HeaderSpec{{Name: "x-gateway-api-key", EnvVar: "GATEWAY_KEY"}},
		}
		noExtraOpts := Options{URL: "https://engram.example.com/mcp", Auth: "bearer"}

		runClaudeCode := func(t *testing.T, opts Options, stdout string, exitCode int) (Result, []runCall) {
			t.Helper()
			var calls []runCall
			env := fakeEnvWithRun(scriptedRun(&calls,
				scriptedResult{Result: RunResult{Stdout: stdout, ExitCode: exitCode}},
			), "claude")
			res := Preview(context.Background(), env, ClaudeCode, opts)
			return res, calls
		}

		assertOneProbeCall := func(t *testing.T, calls []runCall) {
			t.Helper()
			if len(calls) != 1 {
				t.Fatalf("Run called %d times, want exactly 1", len(calls))
			}
			wantArgs := []string{"mcp", "get", "engram"}
			if !reflect.DeepEqual(calls[0].Args, wantArgs) {
				t.Errorf("calls[0].Args = %q, want %q", calls[0].Args, wantArgs)
			}
		}

		t.Run("already-correct", func(t *testing.T) {
			stdout := claudeGetFixture([]string{
				"Authorization: " + claudeCodeBearerForm,
				"x-gateway-api-key: ${GATEWAY_KEY}",
			}, claudeStatusFailedDial)
			res, calls := runClaudeCode(t, opts, stdout, 0)
			if res.Outcome != OutcomeAlreadyCorrect {
				t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeAlreadyCorrect)
			}
			if res.Facets != "" {
				t.Errorf("Facets = %q, want empty", res.Facets)
			}
			wantRegistered := "url=https://engram.example.com/mcp auth=bearer headers=x-gateway-api-key=<redacted>"
			if res.Registered != wantRegistered {
				t.Errorf("Registered = %q, want %q", res.Registered, wantRegistered)
			}
			assertOneProbeCall(t, calls)
		})

		t.Run("would-write-url", func(t *testing.T) {
			stdout := strings.Replace(claudeGetFixture([]string{
				"Authorization: " + claudeCodeBearerForm,
				"x-gateway-api-key: ${GATEWAY_KEY}",
			}, claudeStatusFailedDial), "URL: https://engram.example.com/mcp", "URL: https://old.example/mcp", 1)
			res, calls := runClaudeCode(t, opts, stdout, 0)
			if res.Outcome != OutcomeWouldWrite {
				t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeWouldWrite)
			}
			if res.Facets != "url" {
				t.Errorf("Facets = %q, want %q", res.Facets, "url")
			}
			assertOneProbeCall(t, calls)
		})

		t.Run("would-write-auth-mode", func(t *testing.T) {
			stdout := claudeGetFixture([]string{"Authorization: " + claudeCodeBearerForm}, claudeStatusFailedDial)
			res, calls := runClaudeCode(t, Options{URL: "https://engram.example.com/mcp", Auth: "oauth"}, stdout, 0)
			if res.Outcome != OutcomeWouldWrite {
				t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeWouldWrite)
			}
			if res.Facets != "auth-mode" {
				t.Errorf("Facets = %q, want %q", res.Facets, "auth-mode")
			}
			wantDrift := "auth-mode: observed bearer, would write oauth"
			if res.Drift != wantDrift {
				t.Errorf("Drift = %q, want %q", res.Drift, wantDrift)
			}
			assertOneProbeCall(t, calls)
		})

		t.Run("would-write-header-value-ref", func(t *testing.T) {
			stdout := claudeGetFixture([]string{
				"Authorization: " + claudeCodeBearerForm,
				"x-gateway-api-key: ${OLD_KEY}",
			}, claudeStatusFailedDial)
			res, calls := runClaudeCode(t, opts, stdout, 0)
			if res.Outcome != OutcomeWouldWrite {
				t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeWouldWrite)
			}
			wantDrift := "x-gateway-api-key: observed <redacted>, would write ${GATEWAY_KEY}"
			if res.Drift != wantDrift {
				t.Errorf("Drift = %q, want %q", res.Drift, wantDrift)
			}
			assertOneProbeCall(t, calls)
		})

		t.Run("preserved-unplanned-literal", func(t *testing.T) {
			// Shape: 04-OBSERVATIONS.md §"Claude Code — literal value"
			// (claude 2.1.273, 2026-09-15) — the literal header line
			// (claudeLiteralHeaderLine) quoted verbatim.
			stdout := claudeGetFixture([]string{
				"Authorization: " + claudeCodeBearerForm,
				claudeLiteralHeaderLine,
			}, claudeStatusFailedDial)
			res, calls := runClaudeCode(t, noExtraOpts, stdout, 0)
			if res.Outcome != OutcomePreserved {
				t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomePreserved)
			}
			if res.Facets != "header-name" {
				t.Errorf("Facets = %q, want %q", res.Facets, "header-name")
			}
			wantReason := "claude-code: preserved: x-litellm-api-key: observed <redacted>, not authored by setup; " + claudeCodeWholeEntryNote + "; " + claudeCodeManualRemediation
			if res.Reason != wantReason {
				t.Errorf("Reason = %q, want %q", res.Reason, wantReason)
			}
			b, err := json.Marshal(res)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			if strings.Contains(string(b), "sk-DO-NOT-COMMIT-literal-test-abc123") {
				t.Errorf("json.Marshal(res) = %s, must not contain the observed literal", b)
			}
			assertOneProbeCall(t, calls)
		})

		t.Run("preserved-url-and-unplanned", func(t *testing.T) {
			stdout := strings.Replace(claudeGetFixture([]string{
				"Authorization: " + claudeCodeBearerForm,
				claudeReferenceHeaderLine,
			}, claudeStatusFailedDial), "URL: https://engram.example.com/mcp", "URL: https://old.example/mcp", 1)
			res, calls := runClaudeCode(t, noExtraOpts, stdout, 0)
			if res.Outcome != OutcomePreserved {
				t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomePreserved)
			}
			if res.Facets != "url,header-name" {
				t.Errorf("Facets = %q, want %q", res.Facets, "url,header-name")
			}
			assertOneProbeCall(t, calls)
		})

		t.Run("not-found-would-write", func(t *testing.T) {
			res, calls := runClaudeCode(t, opts, claudeNotFoundAfterRemove, 1)
			if res.Outcome != OutcomeWouldWrite {
				t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeWouldWrite)
			}
			if !strings.HasPrefix(res.Drift, "claude-code: not compared:") {
				t.Errorf("Drift = %q, want prefix %q", res.Drift, "claude-code: not compared:")
			}
			assertOneProbeCall(t, calls)
		})
	})
}

// TestRedactionUnconditional proves a header/credential value observed
// from a probe never reaches any rendered field or the marshaled Result,
// regardless of whether it "looks like" a reference or a literal secret
// (D-02): both a literal-shaped and a reference-shaped foreign bearer
// value redact identically.
func TestRedactionUnconditional(t *testing.T) {
	if _, ok := Codex.(DriftRuntime); !ok {
		t.Fatal("Codex does not implement DriftRuntime")
	}

	opts := Options{URL: "https://engram.example.com/mcp", Auth: "oauth"}

	runForeign := func(t *testing.T, sentinel string) Result {
		t.Helper()
		stdout := strings.Replace(codexGetEngramBearer,
			`"bearer_token_env_var":"ENGRAM_TOKEN"`,
			`"bearer_token_env_var":"`+sentinel+`"`, 1)
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: stdout}},
		), "codex")
		return Preview(context.Background(), env, Codex, opts)
	}

	t.Run("codex/foreign-bearer-var-sentinel", func(t *testing.T) {
		const sentinel = "SENTINEL-VALUE-7c1d4b-DO-NOT-LEAK"
		res := runForeign(t, sentinel)

		if res.Outcome != OutcomePreserved {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomePreserved)
		}
		if res.Facets != "header-name" {
			t.Errorf("Facets = %q, want %q", res.Facets, "header-name")
		}
		wantDrift := "Authorization: observed <redacted>, not authored by setup"
		if res.Drift != wantDrift {
			t.Errorf("Drift = %q, want %q", res.Drift, wantDrift)
		}
		wantReason := "codex: preserved: " + wantDrift + "; " + codexWholeEntryNote + "; " + codexManualRemediation
		if res.Reason != wantReason {
			t.Errorf("Reason = %q, want %q", res.Reason, wantReason)
		}
		wantRegistered := "url=https://engram.example.com/mcp auth=foreign headers=none"
		if res.Registered != wantRegistered {
			t.Errorf("Registered = %q, want %q", res.Registered, wantRegistered)
		}

		b, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if strings.Contains(string(b), sentinel) {
			t.Errorf("json.Marshal(res) = %s, must not contain the sentinel", b)
		}
		for field, value := range map[string]string{
			"Registered": res.Registered,
			"Reason":     res.Reason,
			"Drift":      res.Drift,
			"Notes":      res.Notes,
			"Facets":     res.Facets,
		} {
			if strings.Contains(value, sentinel) {
				t.Errorf("%s = %q, must not contain the sentinel", field, value)
			}
		}
	})

	t.Run("codex/foreign-bearer-var-reference-shaped", func(t *testing.T) {
		sentinelRes := runForeign(t, "SENTINEL-VALUE-7c1d4b-DO-NOT-LEAK")
		referenceRes := runForeign(t, "OTHER_TOKEN")

		sentinelJSON, err := json.Marshal(sentinelRes)
		if err != nil {
			t.Fatalf("json.Marshal(sentinelRes): %v", err)
		}
		referenceJSON, err := json.Marshal(referenceRes)
		if err != nil {
			t.Fatalf("json.Marshal(referenceRes): %v", err)
		}
		if string(sentinelJSON) != string(referenceJSON) {
			t.Errorf("literal-shaped and reference-shaped foreign bearer values produced different Results (D-02: no shape branching):\nliteral:   %s\nreference: %s", sentinelJSON, referenceJSON)
		}
	})

	// assertNoObservedLiteral asserts sk-DO-NOT-COMMIT-literal-test-abc123
	// (the SC5 fixture proof on the OBSERVED shape, REQ-drift-redaction) is
	// absent from json.Marshal(res) and from each of
	// Registered/Reason/Drift/Facets/Notes.
	assertNoObservedLiteral := func(t *testing.T, res Result) {
		t.Helper()
		const sentinel = "sk-DO-NOT-COMMIT-literal-test-abc123"
		b, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if strings.Contains(string(b), sentinel) {
			t.Errorf("json.Marshal(res) = %s, must not contain the observed literal", b)
		}
		for field, value := range map[string]string{
			"Registered": res.Registered,
			"Reason":     res.Reason,
			"Drift":      res.Drift,
			"Notes":      res.Notes,
			"Facets":     res.Facets,
		} {
			if strings.Contains(value, sentinel) {
				t.Errorf("%s = %q, must not contain the observed literal", field, value)
			}
		}
	}

	// The two subtests below are the SC5 fixture proof against the
	// OBSERVED shape (04-05-PLAN.md Task 2, REQ-drift-redaction): both
	// runtimes' literal-echo fixtures, quoted from 04-OBSERVATIONS.md,
	// drive Preview end-to-end and prove the dummy literal never reaches
	// a rendered field or the marshaled Result.
	t.Run("claude-code-observed-literal", func(t *testing.T) {
		// Shape: .planning/phases/04-drift-detection-read-only/
		// 04-OBSERVATIONS.md §"Claude Code — literal value" (claude
		// 2.1.273, 2026-09-15).
		opts := Options{URL: "https://engram.example.com/mcp", Auth: "oauth"}
		stdout := claudeGetFixture([]string{claudeLiteralHeaderLine}, claudeStatusFailedDial)
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: stdout, ExitCode: 0}},
		), "claude")
		res := Preview(context.Background(), env, ClaudeCode, opts)
		if res.Outcome != OutcomePreserved {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomePreserved)
		}
		assertNoObservedLiteral(t, res)
	})

	t.Run("codex-observed-literal", func(t *testing.T) {
		// Shape: .planning/phases/04-drift-detection-read-only/
		// 04-OBSERVATIONS.md §"Codex — literal header (hand-edited)"
		// (codex-cli 0.154.0, 2026-09-15) — codexObservedLiteralHeader.
		opts := Options{URL: "https://engram.example.com/mcp", Auth: "oauth"}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: codexObservedLiteralHeader}},
		), "codex")
		res := Preview(context.Background(), env, Codex, opts)
		if res.Outcome != OutcomePreserved {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomePreserved)
		}
		assertNoObservedLiteral(t, res)
	})
}

// TestCompareRegistrationThreeWay is a literal-expectation table over
// hand-built Observation values, proving Compare's D-01 predicate in
// isolation from any runtime's parsing (REQ-drift-three-way,
// REQ-drift-facet-naming).
func TestCompareRegistrationThreeWay(t *testing.T) {
	defaultOpts := Options{
		URL:     "https://engram.example.com/mcp",
		Auth:    "bearer",
		Headers: []HeaderSpec{{Name: "x-gateway-api-key", EnvVar: "GATEWAY_KEY"}},
	}
	matchingHeader := ObservedHeader{Name: "x-gateway-api-key", State: HeaderMatches, Planned: "${GATEWAY_KEY}"}

	cases := []struct {
		name          string
		obs           Observation
		opts          Options
		wantOutcome   Outcome
		wantFacets    []Facet
		wantDetails   []string
		wantPreserved []string
	}{
		{
			name:        "already-correct",
			obs:         Observation{URL: defaultOpts.URL, Auth: AuthBearer, Headers: []ObservedHeader{matchingHeader}},
			opts:        defaultOpts,
			wantOutcome: OutcomeAlreadyCorrect,
		},
		{
			name:        "url-only",
			obs:         Observation{URL: "https://old.example/mcp", Auth: AuthBearer, Headers: []ObservedHeader{matchingHeader}},
			opts:        defaultOpts,
			wantOutcome: OutcomeWouldWrite,
			wantFacets:  []Facet{FacetURL},
			wantDetails: []string{"url: observed https://old.example/mcp, would write https://engram.example.com/mcp"},
		},
		{
			name:        "auth-none-planned-bearer",
			obs:         Observation{URL: defaultOpts.URL, Auth: AuthNone, Headers: []ObservedHeader{matchingHeader}},
			opts:        defaultOpts,
			wantOutcome: OutcomeWouldWrite,
			wantFacets:  []Facet{FacetAuthMode},
			wantDetails: []string{"auth-mode: observed none, would write bearer"},
		},
		{
			name:        "auth-bearer-planned-oauth",
			obs:         Observation{URL: defaultOpts.URL, Auth: AuthBearer},
			opts:        Options{URL: defaultOpts.URL, Auth: "oauth"},
			wantOutcome: OutcomeWouldWrite,
			wantFacets:  []Facet{FacetAuthMode},
			wantDetails: []string{"auth-mode: observed bearer, would write oauth"},
		},
		{
			name:        "foreign-planned-bearer",
			obs:         Observation{URL: defaultOpts.URL, Auth: AuthForeign, BearerForm: "ENGRAM_TOKEN"},
			opts:        Options{URL: defaultOpts.URL, Auth: "bearer"},
			wantOutcome: OutcomeWouldWrite,
			wantFacets:  []Facet{FacetHeaderValueRef},
			wantDetails: []string{"Authorization: observed <redacted>, would write ENGRAM_TOKEN"},
		},
		{
			name:          "foreign-planned-oauth",
			obs:           Observation{URL: defaultOpts.URL, Auth: AuthForeign, BearerForm: "ENGRAM_TOKEN"},
			opts:          Options{URL: defaultOpts.URL, Auth: "oauth"},
			wantOutcome:   OutcomePreserved,
			wantFacets:    []Facet{FacetHeaderName},
			wantDetails:   []string{"Authorization: observed <redacted>, not authored by setup"},
			wantPreserved: []string{"Authorization: observed <redacted>, not authored by setup"},
		},
		{
			name:        "header-value-differs",
			obs:         Observation{URL: defaultOpts.URL, Auth: AuthBearer, Headers: []ObservedHeader{{Name: "x-gateway-api-key", State: HeaderDiffers, Planned: "${GATEWAY_KEY}"}}},
			opts:        defaultOpts,
			wantOutcome: OutcomeWouldWrite,
			wantFacets:  []Facet{FacetHeaderValueRef},
			wantDetails: []string{"x-gateway-api-key: observed <redacted>, would write ${GATEWAY_KEY}"},
		},
		{
			name:        "header-missing",
			obs:         Observation{URL: defaultOpts.URL, Auth: AuthBearer, Headers: []ObservedHeader{{Name: "x-gateway-api-key", State: HeaderMissing, Planned: "${GATEWAY_KEY}"}}},
			opts:        defaultOpts,
			wantOutcome: OutcomeWouldWrite,
			wantFacets:  []Facet{FacetHeaderName},
			wantDetails: []string{"x-gateway-api-key: not registered, would write ${GATEWAY_KEY}"},
		},
		{
			name:          "header-unplanned",
			obs:           Observation{URL: defaultOpts.URL, Auth: AuthBearer, Headers: []ObservedHeader{{Name: "x-litellm-api-key", State: HeaderUnplanned}}},
			opts:          defaultOpts,
			wantOutcome:   OutcomePreserved,
			wantFacets:    []Facet{FacetHeaderName},
			wantDetails:   []string{"x-litellm-api-key: observed <redacted>, not authored by setup"},
			wantPreserved: []string{"x-litellm-api-key: observed <redacted>, not authored by setup"},
		},
		{
			name: "url-and-unplanned-header",
			obs: Observation{
				URL:     "https://old.example/mcp",
				Auth:    AuthBearer,
				Headers: []ObservedHeader{{Name: "x-litellm-api-key", State: HeaderUnplanned}},
			},
			opts:        defaultOpts,
			wantOutcome: OutcomePreserved,
			wantFacets:  []Facet{FacetURL, FacetHeaderName},
			wantDetails: []string{
				"url: observed https://old.example/mcp, would write https://engram.example.com/mcp",
				"x-litellm-api-key: observed <redacted>, not authored by setup",
			},
			wantPreserved: []string{"x-litellm-api-key: observed <redacted>, not authored by setup"},
		},
		{
			name:          "unrecognized-only",
			obs:           Observation{URL: defaultOpts.URL, Auth: AuthBearer, Unrecognized: []string{"oauth_client_id"}},
			opts:          defaultOpts,
			wantOutcome:   OutcomePreserved,
			wantFacets:    []Facet{FacetUnrecognizedContent},
			wantDetails:   []string{"unrecognized-content: oauth_client_id"},
			wantPreserved: []string{"unrecognized-content: oauth_client_id"},
		},
		{
			name: "everything-differs",
			obs: Observation{
				URL:  "https://old.example/mcp",
				Auth: AuthNone,
				Headers: []ObservedHeader{
					{Name: "x-gateway-api-key", State: HeaderDiffers, Planned: "${GATEWAY_KEY}"},
					{Name: "x-missing-header", State: HeaderMissing, Planned: "${MISSING_KEY}"},
					{Name: "x-litellm-api-key", State: HeaderUnplanned},
				},
				Unrecognized: []string{"oauth_client_id"},
			},
			opts:        defaultOpts,
			wantOutcome: OutcomePreserved,
			wantFacets:  []Facet{FacetURL, FacetAuthMode, FacetHeaderName, FacetHeaderValueRef, FacetUnrecognizedContent},
		},
		{
			name:        "empty-both-sides",
			obs:         Observation{URL: defaultOpts.URL, Auth: AuthBearer},
			opts:        defaultOpts,
			wantOutcome: OutcomeAlreadyCorrect,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			d := Compare(tc.obs, tc.opts)
			if d.Outcome != tc.wantOutcome {
				t.Errorf("Outcome = %q, want %q", d.Outcome, tc.wantOutcome)
			}
			if !reflect.DeepEqual(d.Facets, tc.wantFacets) {
				t.Errorf("Facets = %v, want %v", d.Facets, tc.wantFacets)
			}
			if tc.wantDetails != nil && !reflect.DeepEqual(d.Details, tc.wantDetails) {
				t.Errorf("Details = %v, want %v", d.Details, tc.wantDetails)
			}
			if tc.wantPreserved != nil && !reflect.DeepEqual(d.Preserved, tc.wantPreserved) {
				t.Errorf("Preserved = %v, want %v", d.Preserved, tc.wantPreserved)
			}
		})
	}

	t.Run("preserved-never-empty-causes", func(t *testing.T) {
		for _, tc := range cases {
			if tc.wantOutcome != OutcomePreserved {
				continue
			}
			d := Compare(tc.obs, tc.opts)
			if len(d.Preserved) == 0 {
				t.Errorf("%s: Preserved is empty, want at least one cause for a preserved row", tc.name)
			}
		}
	})
}

// TestFacetOrderIsAuthored pins D-12's fixed stable rendering order.
func TestFacetOrderIsAuthored(t *testing.T) {
	want := []Facet{FacetURL, FacetAuthMode, FacetHeaderName, FacetHeaderValueRef, FacetUnrecognizedContent}
	if !reflect.DeepEqual(facetOrder, want) {
		t.Fatalf("facetOrder = %v, want %v", facetOrder, want)
	}

	if got := joinFacets([]Facet{FacetUnrecognizedContent, FacetURL, FacetHeaderName}); got != "url,header-name,unrecognized-content" {
		t.Errorf("joinFacets(...) = %q, want %q", got, "url,header-name,unrecognized-content")
	}
	if got := joinFacets(nil); got != "" {
		t.Errorf("joinFacets(nil) = %q, want empty", got)
	}

	// A Compare whose emission order is header-first still lists url
	// first (already covered by url-and-unplanned-header above; asserted
	// again here on joinFacets(d.Facets)).
	obs := Observation{
		URL:     "https://old.example/mcp",
		Auth:    AuthBearer,
		Headers: []ObservedHeader{{Name: "x-litellm-api-key", State: HeaderUnplanned}},
	}
	d := Compare(obs, Options{URL: "https://engram.example.com/mcp", Auth: "bearer"})
	if got := joinFacets(d.Facets); got != "url,header-name" {
		t.Errorf("joinFacets(d.Facets) = %q, want %q", got, "url,header-name")
	}
}

// TestPreviewAmbiguityResolvesToWouldWrite pins D-09: every unparseable,
// unreadable, or unrecognized-registration probe result resolves to
// would-write with no facets and a Drift note that quotes no probe
// bytes — never already-correct, never preserved.
func TestPreviewAmbiguityResolvesToWouldWrite(t *testing.T) {
	opts := Options{URL: "https://engram.example.com/mcp", Auth: "bearer"}

	assertAmbiguous := func(t *testing.T, res Result, calls []runCall) {
		t.Helper()
		if res.Outcome != OutcomeWouldWrite {
			t.Errorf("Outcome = %q, want %q", res.Outcome, OutcomeWouldWrite)
		}
		if res.Facets != "" {
			t.Errorf("Facets = %q, want empty", res.Facets)
		}
		if res.Registered != "" {
			t.Errorf("Registered = %q, want empty", res.Registered)
		}
		if res.Reason != "" {
			t.Errorf("Reason = %q, want empty", res.Reason)
		}
		if !strings.HasPrefix(res.Drift, "codex: not compared: ") {
			t.Errorf("Drift = %q, want prefix %q", res.Drift, "codex: not compared: ")
		}
		if len(calls) != 1 {
			t.Errorf("Run called %d times, want exactly 1", len(calls))
		}
		if strings.Contains(res.Drift, "PROBE-BYTES-MUST-NOT-RENDER") {
			t.Errorf("Drift = %q, must not contain probe bytes", res.Drift)
		}
	}

	t.Run("empty-stdout", func(t *testing.T) {
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls, scriptedResult{Result: RunResult{Stdout: ""}}), "codex")
		res := Preview(context.Background(), env, Codex, opts)
		assertAmbiguous(t, res, calls)
	})

	t.Run("not-json", func(t *testing.T) {
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls, scriptedResult{Result: RunResult{Stdout: "PROBE-BYTES-MUST-NOT-RENDER not json"}}), "codex")
		res := Preview(context.Background(), env, Codex, opts)
		assertAmbiguous(t, res, calls)
	})

	t.Run("json-null", func(t *testing.T) {
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls, scriptedResult{Result: RunResult{Stdout: "null"}}), "codex")
		res := Preview(context.Background(), env, Codex, opts)
		assertAmbiguous(t, res, calls)
	})

	t.Run("wrong-name", func(t *testing.T) {
		stdout := strings.Replace(codexGetEngramBearer, `"name":"engram"`, `"name":"other"`, 1)
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls, scriptedResult{Result: RunResult{Stdout: stdout}}), "codex")
		res := Preview(context.Background(), env, Codex, opts)
		assertAmbiguous(t, res, calls)
	})

	t.Run("nonzero-exit-not-found", func(t *testing.T) {
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls, scriptedResult{Result: RunResult{ExitCode: 1, Stderr: "No MCP server named 'engram' PROBE-BYTES-MUST-NOT-RENDER"}}), "codex")
		res := Preview(context.Background(), env, Codex, opts)
		assertAmbiguous(t, res, calls)
		if !strings.Contains(res.Drift, "exited 1") {
			t.Errorf("Drift = %q, want it to contain %q", res.Drift, "exited 1")
		}
	})

	t.Run("seam-error", func(t *testing.T) {
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls, scriptedResult{Err: errors.New("exec: start failure")}), "codex")
		res := Preview(context.Background(), env, Codex, opts)
		assertAmbiguous(t, res, calls)
		if !strings.Contains(res.Drift, "exec: start failure") {
			t.Errorf("Drift = %q, want it to contain %q", res.Drift, "exec: start failure")
		}
	})
}

// TestDriftRuntimeIsOptional pins D-10: only claude-code and codex
// implement DriftRuntime; opencode and generic do not, and a scanner-less
// probe-wired runtime never yields already-correct or preserved — a
// convincing-looking table never fools the executor.
func TestDriftRuntimeIsOptional(t *testing.T) {
	if _, ok := Codex.(DriftRuntime); !ok {
		t.Error("Codex does not implement DriftRuntime")
	}
	if _, ok := ClaudeCode.(DriftRuntime); !ok {
		t.Error("ClaudeCode does not implement DriftRuntime")
	}
	if _, ok := OpenCode.(DriftRuntime); ok {
		t.Error("OpenCode implements DriftRuntime, want it not to (D-10)")
	}
	if _, ok := Generic.(DriftRuntime); ok {
		t.Error("Generic implements DriftRuntime, want it not to (D-10)")
	}

	opts := Options{URL: "https://engram.example.com/mcp", Auth: "bearer"}

	t.Run("opencode-not-compared", func(t *testing.T) {
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: "┌ engram ─ https://engram.example.com/mcp ─ connected"}},
		), "opencode")
		res := Preview(context.Background(), env, OpenCode, opts)
		if res.Outcome != OutcomeWouldWrite {
			t.Errorf("Outcome = %q, want %q", res.Outcome, OutcomeWouldWrite)
		}
		if res.Facets != "" {
			t.Errorf("Facets = %q, want empty", res.Facets)
		}
		if res.Registered != "" {
			t.Errorf("Registered = %q, want empty", res.Registered)
		}
		wantDrift := "opencode: not compared: runtime authors no registration scanner"
		if res.Drift != wantDrift {
			t.Errorf("Drift = %q, want %q", res.Drift, wantDrift)
		}
		if len(calls) != 1 {
			t.Errorf("Run called %d times, want exactly 1", len(calls))
		}
	})

	t.Run("scanner-less-fake-runtime-not-compared", func(t *testing.T) {
		rt := fakeRuntime{name: "faketool", plan: Plan{
			Runtime: "faketool",
			Actions: []Action{{Args: []string{"faketool", "mcp", "add"}}},
			Probe:   []string{"faketool", "mcp", "get"},
		}}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: "existing state"}},
		), "faketool")
		res := Preview(context.Background(), env, rt, opts)
		wantDrift := "faketool: not compared: runtime authors no registration scanner"
		if res.Drift != wantDrift {
			t.Errorf("Drift = %q, want %q", res.Drift, wantDrift)
		}
		if res.Outcome != OutcomeWouldWrite {
			t.Errorf("Outcome = %q, want %q", res.Outcome, OutcomeWouldWrite)
		}
	})
}
