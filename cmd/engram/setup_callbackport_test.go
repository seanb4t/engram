// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanb4t/engram/internal/setup"
)

// setupPreviewDoc runs a setup preview with args against env and decodes
// its JSON report.
func setupPreviewDoc(t *testing.T, env setup.Environment, args ...string) setupReportDoc {
	t.Helper()
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, env)
	stdout, stderr, err := runClient(t, append([]string{"setup"}, args...)...)
	if err != nil {
		t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
	}
	var doc setupReportDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	return doc
}

// TestSetupCallbackPort covers `engram setup --callback-port` (change
// cli-parity): the Claude Code registration carries the port on the oauth
// arm only when given and on the oauth-client arm always (8765 by
// default), every present row names the port it was given, a non-OAuth
// mode or an out-of-range value is a usage error before any runtime is
// touched, and a runtime without the option reports a failed row beside a
// Claude Code success, with the partial exit code under --apply.
func TestSetupCallbackPort(t *testing.T) {
	const url = "https://engram.example.com/mcp"

	t.Run("oauth with a fixed port", func(t *testing.T) {
		doc := setupPreviewDoc(t, fakeSetupEnv("claude"), "--url", url, "--auth", "oauth", "--callback-port", "8765", "--runtime", "claude-code", "--output", "json")
		row := doc.Runtimes[0]
		if !strings.Contains(row.Command, "--callback-port 8765") {
			t.Errorf("command = %q, want it to carry --callback-port 8765", row.Command)
		}
		if row.CallbackPort != "8765" {
			t.Errorf("callback_port = %q, want 8765", row.CallbackPort)
		}
	})

	t.Run("oauth without a port is unchanged and names none", func(t *testing.T) {
		doc := setupPreviewDoc(t, fakeSetupEnv("claude"), "--url", url, "--auth", "oauth", "--runtime", "claude-code", "--output", "json")
		row := doc.Runtimes[0]
		if strings.Contains(row.Command, "--callback-port") || row.CallbackPort != "" {
			t.Errorf("row = %+v, want no callback port on a bare oauth registration", row)
		}
	})

	t.Run("oauth-client keeps 8765 by default and takes an override", func(t *testing.T) {
		doc := setupPreviewDoc(t, fakeSetupEnv("claude"), "--url", url, "--auth", "oauth-client", "--client-id", "c", "--runtime", "claude-code", "--output", "json")
		if row := doc.Runtimes[0]; !strings.Contains(row.Command, "--callback-port 8765") || row.CallbackPort != "8765" {
			t.Errorf("default row = %+v, want --callback-port 8765 and callback_port 8765", row)
		}
		doc = setupPreviewDoc(t, fakeSetupEnv("claude"), "--url", url, "--auth", "oauth-client", "--client-id", "c", "--callback-port", "9000", "--runtime", "claude-code", "--output", "json")
		if row := doc.Runtimes[0]; !strings.Contains(row.Command, "--callback-port 9000") || strings.Contains(row.Command, "8765") || row.CallbackPort != "9000" {
			t.Errorf("override row = %+v, want --callback-port 9000 only", row)
		}
	})

	t.Run("codex oauth-client carries no implied port", func(t *testing.T) {
		doc := setupPreviewDoc(t, fakeSetupEnv("codex"), "--url", url, "--auth", "oauth-client", "--client-id", "c", "--runtime", "codex", "--output", "json")
		row := doc.Runtimes[0]
		if row.Outcome == "failed" {
			t.Fatalf("codex oauth-client unexpectedly failed: %+v", row)
		}
		if strings.Contains(row.Command, "--callback-port") || row.CallbackPort != "" {
			t.Errorf("row = %+v, want no callback port on a runtime whose argv carries none", row)
		}
	})

	t.Run("not-present rows carry no port", func(t *testing.T) {
		doc := setupPreviewDoc(t, fakeSetupEnv(), "--url", url, "--auth", "oauth", "--callback-port", "8765", "--runtime", "claude-code", "--output", "json")
		if row := doc.Runtimes[0]; row.Present || row.CallbackPort != "" {
			t.Errorf("row = %+v, want an absent runtime with no callback-port facet", row)
		}
	})

	t.Run("non-OAuth modes and out-of-range values are usage errors before detection", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			args []string
			want string
		}{
			{"bearer", []string{"--auth", "bearer", "--callback-port", "8765"}, "--auth oauth or --auth oauth-client"},
			{"none", []string{"--auth", "none", "--callback-port", "8765"}, "--auth oauth or --auth oauth-client"},
			{"too high", []string{"--auth", "oauth", "--callback-port", "70000"}, "1 to 65535"},
			{"zero", []string{"--auth", "oauth", "--callback-port", "0"}, "1 to 65535"},
			{"env", nil, "1 to 65535"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				resetClientFlags(t)
				resetCommandFlagState(t, setupCmd)
				if tc.name == "env" {
					t.Setenv("ENGRAM_CALLBACK_PORT", "99999")
				}
				env := fakeSetupEnv("claude")
				env.LookPath = func(string) (string, error) {
					t.Error("runtime detection ran despite the usage error")
					return "", nil
				}
				withFakeSetupEnv(t, env)
				_, _, err := runClient(t, append([]string{"setup", "--url", url, "--output", "json"}, tc.args...)...)
				if got := exitCodeFromError(err); got != exitUsage {
					t.Fatalf("exit = %d, want %d: %v", got, exitUsage, err)
				}
				if !strings.Contains(err.Error(), tc.want) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), tc.want)
				}
			})
		}
	})

	t.Run("codex reports failed beside a Claude Code success", func(t *testing.T) {
		doc := setupPreviewDoc(t, fakeSetupEnv("claude", "codex"), "--url", url, "--auth", "oauth", "--callback-port", "8765", "--runtime", "claude-code,codex", "--output", "json")
		byName := map[string]setupRuntimeRow{}
		for _, r := range doc.Runtimes {
			byName[r.Name] = r
		}
		if row := byName["claude-code"]; row.Outcome == "failed" || !strings.Contains(row.Command, "--callback-port 8765") || row.CallbackPort != "8765" {
			t.Errorf("claude-code row = %+v, want the port carried", row)
		}
		if row := byName["codex"]; row.Outcome != "failed" || row.Command != "" || !strings.Contains(row.Reason, "--callback-port") || row.CallbackPort != "8765" {
			t.Errorf("codex row = %+v, want failed naming --callback-port and the requested port", row)
		}

		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		var calls [][]string
		withFakeSetupEnv(t, fakeSetupEnvWithRun(func(_ context.Context, path string, args []string) (setup.RunResult, error) {
			calls = append(calls, append([]string{filepath.Base(path)}, args...))
			return setup.RunResult{}, nil
		}, "claude", "codex"))
		_, _, err := runClient(t, "setup", "--url", url, "--auth", "oauth", "--callback-port", "8765", "--runtime", "claude-code,codex", "--apply", "--output", "json")
		if got := exitCodeFromError(err); got != exitPartial {
			t.Fatalf("exit = %d, want exitPartial (%d): %v", got, exitPartial, err)
		}
		for _, call := range calls {
			if call[0] == "codex" && len(call) > 2 && call[2] == "add" {
				t.Errorf("codex decline still ran an add invocation: %q", call)
			}
		}
	})

	t.Run("help names the flag", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		stdout, _, err := runClient(t, "setup", "--help")
		if err != nil {
			t.Fatalf("help: %v", err)
		}
		for _, want := range []string{"--callback-port", "ENGRAM_CALLBACK_PORT", "ssh -L", "never compared"} {
			if !strings.Contains(stdout, want) {
				t.Errorf("help missing %q", want)
			}
		}
	})
}
