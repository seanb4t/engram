// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// codexGetEngramBearer is the package-level fixture base every codex
// registration test in this plan derives from via strings.Replace — the
// 04-RESEARCH.md verbatim `codex mcp get engram --json` shape (codex-cli
// 0.153.4), never a second hand-typed document.
const codexGetEngramBearer = `{"name":"engram","enabled":true,"disabled_reason":null,"transport":{"type":"streamable_http","url":"https://engram.example.com/mcp","bearer_token_env_var":"ENGRAM_TOKEN","http_headers":null,"env_http_headers":null,"http_headers_helper":null},"enabled_tools":null,"disabled_tools":null,"startup_timeout_sec":null,"tool_timeout_sec":null}`

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
		wantReason := "codex: preserved: unrecognized-content: oauth_client_id; " + codexWholeEntryNote
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
		if !reflectEqualStrings(calls[0].Args, wantArgs) {
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
		wantReason := "codex: preserved: " + wantDrift + "; " + codexWholeEntryNote
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
}

// reflectEqualStrings compares two string slices element-wise — a small
// local helper so this file needs no reflect import for one simple
// comparison.
func reflectEqualStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
