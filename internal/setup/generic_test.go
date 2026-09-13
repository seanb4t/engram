// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// genericMCPServerView is the test-side decode shape for one entry of
// genericConfigDoc.MCPServers — a local mirror rather than importing the
// unexported production type, so this test proves the CONTRACT (what
// json.Unmarshal sees) rather than reusing the exact struct the
// production code marshals from.
type genericMCPServerView struct {
	Type    string            `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

// TestGenericConfig proves, for every auth mode generic accepts, that the
// config text unmarshals cleanly, contains no newline, carries opts.URL
// byte-for-byte, always names "type":"http", and carries a
// headers.Authorization entry for bearer and no such entry otherwise.
func TestGenericConfig(t *testing.T) {
	const url = "https://engram.example.com/mcp"

	for _, auth := range []string{"oauth", "none", "bearer"} {
		auth := auth
		t.Run(auth, func(t *testing.T) {
			plan, err := Generic.Plan(OSEnvironment, Options{URL: url, Auth: auth})
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if len(plan.Actions) != 0 {
				t.Errorf("Plan.Actions = %v, want none (generic authors no Action)", plan.Actions)
			}
			if plan.Probe != nil {
				t.Errorf("Plan.Probe = %v, want nil (generic authors no Probe)", plan.Probe)
			}
			if strings.ContainsRune(plan.Config, '\n') {
				t.Errorf("Config contains a newline: %q", plan.Config)
			}

			var top map[string]json.RawMessage
			if err := json.Unmarshal([]byte(plan.Config), &top); err != nil {
				t.Fatalf("Config does not unmarshal: %v (%q)", err, plan.Config)
			}
			if len(top) != 1 {
				t.Fatalf("Config top-level key set has %d keys, want exactly {mcpServers}: %q", len(top), plan.Config)
			}
			serversRaw, ok := top["mcpServers"]
			if !ok {
				t.Fatalf("Config top-level key set = %v, want exactly {mcpServers}", top)
			}

			var servers map[string]genericMCPServerView
			if err := json.Unmarshal(serversRaw, &servers); err != nil {
				t.Fatalf("mcpServers does not unmarshal: %v", err)
			}
			entry, ok := servers["engram"]
			if !ok {
				t.Fatalf("mcpServers has no engram entry: %v", servers)
			}
			if entry.Type != "http" {
				t.Errorf("engram.type = %q, want %q", entry.Type, "http")
			}
			if entry.URL != url {
				t.Errorf("engram.url = %q, want %q byte-for-byte", entry.URL, url)
			}
			_, hasAuthHeader := entry.Headers["Authorization"]
			wantHeader := auth == "bearer"
			if hasAuthHeader != wantHeader {
				t.Errorf("engram.headers has an Authorization key = %v, want %v (auth=%s)", hasAuthHeader, wantHeader, auth)
			}
			if wantHeader {
				const want = "Bearer ${ENGRAM_TOKEN}"
				if entry.Headers["Authorization"] != want {
					t.Errorf("engram.headers.Authorization = %q, want %q (no --token-file supplied)", entry.Headers["Authorization"], want)
				}
			}
		})
	}
}

// TestGenericHeaders proves generic carries opts.Headers extras in the
// EXISTING headers map beside any bearer entry (D-05), each valued as a
// bare "${ENVVAR}" reference, with the marshaled JSON key order fixed to
// "Authorization" first then extras sorted case-insensitively (D-08) —
// and that the zero-header documents for oauth/none, bearer, and
// bearer+token-file stay byte-identical to the three literals captured
// live at HEAD before this task's edit (REQ-header-bearer-unchanged).
func TestGenericHeaders(t *testing.T) {
	const url = "https://engram.example.com/mcp"

	headers := []HeaderSpec{
		{Name: "x-litellm-api-key", EnvVar: "LITELLM_KEY"},
		{Name: "CF-Access-Client-Id", EnvVar: "CF_ID"},
	}

	for _, auth := range []string{"oauth", "none", "bearer"} {
		auth := auth
		t.Run(auth, func(t *testing.T) {
			plan, err := Generic.Plan(OSEnvironment, Options{URL: url, Auth: auth, Headers: headers})
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if len(plan.Actions) != 0 {
				t.Errorf("Plan.Actions = %v, want none", plan.Actions)
			}
			if plan.Probe != nil {
				t.Errorf("Plan.Probe = %v, want nil", plan.Probe)
			}

			var top map[string]json.RawMessage
			if err := json.Unmarshal([]byte(plan.Config), &top); err != nil {
				t.Fatalf("Config does not unmarshal: %v (%q)", err, plan.Config)
			}
			var servers map[string]genericMCPServerView
			if err := json.Unmarshal(top["mcpServers"], &servers); err != nil {
				t.Fatalf("mcpServers does not unmarshal: %v", err)
			}
			entry, ok := servers["engram"]
			if !ok {
				t.Fatalf("mcpServers has no engram entry: %v", servers)
			}

			if entry.Headers["x-litellm-api-key"] != "${LITELLM_KEY}" {
				t.Errorf("headers[x-litellm-api-key] = %q, want %q", entry.Headers["x-litellm-api-key"], "${LITELLM_KEY}")
			}
			if entry.Headers["CF-Access-Client-Id"] != "${CF_ID}" {
				t.Errorf("headers[CF-Access-Client-Id] = %q, want %q", entry.Headers["CF-Access-Client-Id"], "${CF_ID}")
			}

			if auth == "bearer" {
				if entry.Headers["Authorization"] != "Bearer ${ENGRAM_TOKEN}" {
					t.Errorf("headers[Authorization] = %q, want %q", entry.Headers["Authorization"], "Bearer ${ENGRAM_TOKEN}")
				}
				if len(entry.Headers) != 3 {
					t.Errorf("len(headers) = %d, want 3 (Authorization + 2 extras)", len(entry.Headers))
				}
			} else {
				if _, has := entry.Headers["Authorization"]; has {
					t.Errorf("headers has an Authorization key for auth=%s, want none", auth)
				}
				if len(entry.Headers) != 2 {
					t.Errorf("len(headers) = %d, want 2 (2 extras only)", len(entry.Headers))
				}
			}

			// RAW ORDER: the reason MarshalJSON exists.
			raw := plan.Config
			idxAuth := strings.Index(raw, "\"Authorization\"")
			idxCF := strings.Index(raw, "\"CF-Access-Client-Id\"")
			idxLite := strings.Index(raw, "\"x-litellm-api-key\"")
			if idxCF < 0 || idxLite < 0 {
				t.Fatalf("raw Config missing an expected header key: %q", raw)
			}
			if idxCF >= idxLite {
				t.Errorf("raw order: CF-Access-Client-Id at %d, x-litellm-api-key at %d, want CF-Access-Client-Id first (case-insensitive sort)", idxCF, idxLite)
			}
			if auth == "bearer" {
				if idxAuth < 0 {
					t.Fatalf("raw Config missing Authorization key: %q", raw)
				}
				if idxAuth >= idxCF {
					t.Errorf("raw order: Authorization at %d, CF-Access-Client-Id at %d, want Authorization first", idxAuth, idxCF)
				}
			}
		})
	}

	t.Run("case-insensitive-discriminator", func(t *testing.T) {
		hs := []HeaderSpec{
			{Name: "a-key", EnvVar: "A2"},
			{Name: "B-Key", EnvVar: "B2"},
		}
		plan, err := Generic.Plan(OSEnvironment, Options{URL: url, Auth: "oauth", Headers: hs})
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		idxA := strings.Index(plan.Config, "\"a-key\"")
		idxB := strings.Index(plan.Config, "\"B-Key\"")
		if idxA < 0 || idxB < 0 {
			t.Fatalf("raw Config missing an expected key: %q", plan.Config)
		}
		if idxA >= idxB {
			t.Errorf("raw order: a-key at %d, B-Key at %d, want a-key first", idxA, idxB)
		}
	})

	t.Run("token-file-provenance-unaffected", func(t *testing.T) {
		plan, err := Generic.Plan(OSEnvironment, Options{
			URL: url, Auth: "bearer", TokenFile: "/home/u/.engram/token", Headers: headers,
		})
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		var top map[string]json.RawMessage
		if err := json.Unmarshal([]byte(plan.Config), &top); err != nil {
			t.Fatalf("Config does not unmarshal: %v", err)
		}
		var servers map[string]genericMCPServerView
		if err := json.Unmarshal(top["mcpServers"], &servers); err != nil {
			t.Fatalf("mcpServers does not unmarshal: %v", err)
		}
		entry := servers["engram"]
		const want = "Bearer <from /home/u/.engram/token>"
		if entry.Headers["Authorization"] != want {
			t.Errorf("headers[Authorization] = %q, want %q (provenance stays bearer-only)", entry.Headers["Authorization"], want)
		}
		if entry.Headers["x-litellm-api-key"] != "${LITELLM_KEY}" {
			t.Errorf("headers[x-litellm-api-key] = %q, want %q (extras still references)", entry.Headers["x-litellm-api-key"], "${LITELLM_KEY}")
		}
	})

	t.Run("zero-header-byte-identity", func(t *testing.T) {
		oauthPlan, err := Generic.Plan(OSEnvironment, Options{URL: url, Auth: "oauth"})
		if err != nil {
			t.Fatalf("Plan(oauth): %v", err)
		}
		const wantOauth = `{"mcpServers":{"engram":{"type":"http","url":"https://engram.example.com/mcp"}}}`
		if oauthPlan.Config != wantOauth {
			t.Errorf("oauth zero-header Config = %q, want %q byte-identical to HEAD", oauthPlan.Config, wantOauth)
		}

		bearerPlan, err := Generic.Plan(OSEnvironment, Options{URL: url, Auth: "bearer"})
		if err != nil {
			t.Fatalf("Plan(bearer): %v", err)
		}
		const wantBearer = `{"mcpServers":{"engram":{"type":"http","url":"https://engram.example.com/mcp","headers":{"Authorization":"Bearer ${ENGRAM_TOKEN}"}}}}`
		if bearerPlan.Config != wantBearer {
			t.Errorf("bearer zero-header Config = %q, want %q byte-identical to HEAD", bearerPlan.Config, wantBearer)
		}

		bearerFilePlan, err := Generic.Plan(OSEnvironment, Options{URL: url, Auth: "bearer", TokenFile: "/home/u/.engram/token"})
		if err != nil {
			t.Fatalf("Plan(bearer, TokenFile): %v", err)
		}
		const wantBearerFile = "{\"mcpServers\":{\"engram\":{\"type\":\"http\",\"url\":\"https://engram.example.com/mcp\",\"headers\":{\"Authorization\":\"Bearer \\u003cfrom /home/u/.engram/token\\u003e\"}}}}"
		if bearerFilePlan.Config != wantBearerFile {
			t.Errorf("bearer+token-file zero-header Config = %q, want %q byte-identical to HEAD", bearerFilePlan.Config, wantBearerFile)
		}
	})

	t.Run("input-order-unchanged", func(t *testing.T) {
		hs := []HeaderSpec{
			{Name: "x-litellm-api-key", EnvVar: "LITELLM_KEY"},
			{Name: "CF-Access-Client-Id", EnvVar: "CF_ID"},
		}
		if _, err := Generic.Plan(OSEnvironment, Options{URL: url, Auth: "oauth", Headers: hs}); err != nil {
			t.Fatalf("Plan: %v", err)
		}
		if hs[0].Name != "x-litellm-api-key" {
			t.Errorf("input slice reordered: hs[0].Name = %q, want %q", hs[0].Name, "x-litellm-api-key")
		}
	})
}

// TestGenericStartsNoProcess drives Apply for Generic through an
// Environment whose LookPath and Run both call t.Fatal if invoked at
// all, proving generic reaches neither seam: its zero-Action Plan is
// classified OutcomeWouldWrite without ever resolving a binary or
// starting a subprocess (D-16).
func TestGenericStartsNoProcess(t *testing.T) {
	env := Environment{
		LookPath: func(file string) (string, error) {
			t.Fatalf("LookPath(%q) called; generic must never resolve a binary", file)
			return "", exec.ErrNotFound
		},
		Getenv:  func(string) string { return "" },
		HomeDir: func() (string, error) { return "/home/fake", nil },
		Run: func(context.Context, string, []string) (RunResult, error) {
			t.Fatal("Run called; generic must never start a subprocess")
			return RunResult{}, nil
		},
	}

	res := Apply(context.Background(), env, Generic, Options{URL: "https://engram.example.com/mcp", Auth: "oauth"})
	if res.Outcome != OutcomeWouldWrite {
		t.Errorf("Apply outcome = %q, want %q", res.Outcome, OutcomeWouldWrite)
	}
	if res.Config == "" {
		t.Error("Apply result Config is empty, want the portable config text")
	}
}

// TestGenericSkillTargetHasNoDestination proves generic's Plan() authors
// the explicit no-destination skill format (D-11) for every auth mode it
// supports, with both path fields left empty — generic has no machine of
// its own and therefore no destination to derive — and that the returned
// Plan still carries zero Actions and a nil Probe, exactly as it did
// before Phase 4 wired its skills payload.
func TestGenericSkillTargetHasNoDestination(t *testing.T) {
	const url = "https://engram.example.com/mcp"

	for _, auth := range []string{"oauth", "none", "bearer"} {
		auth := auth
		t.Run(auth, func(t *testing.T) {
			plan, err := Generic.Plan(OSEnvironment, Options{URL: url, Auth: auth})
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if plan.Skills.Format != SkillFormatNone {
				t.Errorf("Plan.Skills.Format = %q, want %q", plan.Skills.Format, SkillFormatNone)
			}
			if plan.Skills.Dir != "" {
				t.Errorf("Plan.Skills.Dir = %q, want empty (generic has no destination)", plan.Skills.Dir)
			}
			if plan.Skills.IndexFile != "" {
				t.Errorf("Plan.Skills.IndexFile = %q, want empty (generic has no destination)", plan.Skills.IndexFile)
			}
			if len(plan.Actions) != 0 {
				t.Errorf("Plan.Actions = %v, want none (generic authors no Action)", plan.Actions)
			}
			if plan.Probe != nil {
				t.Errorf("Plan.Probe = %v, want nil (generic authors no Probe)", plan.Probe)
			}
		})
	}
}

// TestGenericSkillsOutcomeIsWouldWriteInBothLanes is the unit-level pin
// for D-11's central claim: SkillsOutcome for the no-destination format
// always yields would-write, in both the preview lane (mutate=false) and
// the apply lane (mutate=true) — generic never writes and never
// converges, so the already-correct value can never legitimately be
// produced for it.
func TestGenericSkillsOutcomeIsWouldWriteInBothLanes(t *testing.T) {
	for _, mutate := range []bool{false, true} {
		got := SkillsOutcome(SkillFormatNone, mutate, 0, 0, false)
		if got != OutcomeWouldWrite {
			t.Errorf("SkillsOutcome(SkillFormatNone, mutate=%v, 0, 0, false) = %q, want %q", mutate, got, OutcomeWouldWrite)
		}
	}
}

// TestGenericConfigCarriesNoSecret proves that a resolved ENGRAM_TOKEN
// credential value never reaches generic's Config text, mirroring
// TestNoSecretInArgs' sentinel discipline (plan_test.go) for the
// Config-only deliverable this runtime authors.
func TestGenericConfigCarriesNoSecret(t *testing.T) {
	const secretValue = "SUPER-SECRET-VALUE-MUST-NEVER-APPEAR-9f3e2a"
	env := Environment{
		LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		Getenv: func(key string) string {
			if key == "ENGRAM_TOKEN" {
				return secretValue
			}
			return ""
		},
		HomeDir: func() (string, error) { return "/home/fake", nil },
	}

	for _, tokenFile := range []string{"", "/home/u/.engram/token"} {
		plan, err := Generic.Plan(env, Options{URL: "https://x", Auth: "bearer", TokenFile: tokenFile})
		if err != nil {
			t.Fatalf("Plan(TokenFile=%q): %v", tokenFile, err)
		}
		if strings.Contains(plan.Config, secretValue) {
			t.Errorf("Plan(TokenFile=%q).Config contains the resolved credential value: %q", tokenFile, plan.Config)
		}
	}
}
