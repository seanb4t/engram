// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// TestClaudeCodeCallbackPort pins the claude-code arms around
// Options.CallbackPort (change cli-parity): without it the oauth arm is
// byte-identical to the shipped argv and the oauth-client arm keeps its
// shipped 8765; with it, both arms carry the caller's port.
func TestClaudeCodeCallbackPort(t *testing.T) {
	const url = "https://engram.example.com/mcp"
	env := fakeEnv()
	base := []string{"claude", "mcp", "add", "--transport", "http", "engram", url, "--scope", "user"}
	for _, tc := range []struct {
		name string
		opts Options
		want []string
	}{
		{"oauth without port is byte-identical", Options{URL: url, Auth: "oauth"}, base},
		{"oauth with port", Options{URL: url, Auth: "oauth", CallbackPort: 9000},
			append(append([]string{}, base...), "--callback-port", "9000")},
		{"oauth with port keeps headers after it", Options{URL: url, Auth: "oauth", CallbackPort: 9000, Headers: []HeaderSpec{{Name: "x-k", EnvVar: "K"}}},
			append(append([]string{}, base...), "--callback-port", "9000", "--header", "x-k: ${K}")},
		{"oauth-client without port keeps 8765", Options{URL: url, Auth: "oauth-client", ClientID: "c"},
			append(append([]string{}, base...), "--client-id", "c", "--client-secret", "--callback-port", "8765")},
		{"oauth-client with port", Options{URL: url, Auth: "oauth-client", ClientID: "c", CallbackPort: 9000},
			append(append([]string{}, base...), "--client-id", "c", "--client-secret", "--callback-port", "9000")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := ClaudeCode.Plan(env, tc.opts)
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			got := plan.Actions[1].Args
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("add argv = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCodexAndOpenCodeDeclineCallbackPort proves both runtimes decline a
// fixed callback port before anything else runs — HomeDir fails the test
// if called — with an ErrCallbackPortUnsupported-wrapped error naming the
// flag and never the other two sentinels, so the gaps stay distinguishable.
func TestCodexAndOpenCodeDeclineCallbackPort(t *testing.T) {
	const url = "https://engram.example.com/mcp"
	for _, rt := range []Runtime{Codex, OpenCode} {
		for _, auth := range []string{"oauth", "oauth-client"} {
			t.Run(rt.Name()+"/"+auth, func(t *testing.T) {
				env := fakeEnv()
				env.HomeDir = func() (string, error) {
					t.Errorf("HomeDir called; the callback-port guard must run first")
					return "/home/fake", nil
				}
				plan, err := rt.Plan(env, Options{URL: url, Auth: auth, ClientID: "c", CallbackPort: 8765})
				if !errors.Is(err, ErrCallbackPortUnsupported) {
					t.Fatalf("err = %v, want errors.Is(err, ErrCallbackPortUnsupported)", err)
				}
				if errors.Is(err, ErrHeaderUnsupported) || errors.Is(err, ErrAuthModeUnsupported) {
					t.Errorf("err = %v, must not satisfy the header or auth-mode sentinels", err)
				}
				if !strings.Contains(err.Error(), "--callback-port") || !strings.Contains(err.Error(), "--runtime") {
					t.Errorf("err = %q, want it to name --callback-port and the --runtime remedy", err.Error())
				}
				if !reflect.DeepEqual(plan, Plan{}) {
					t.Errorf("plan = %#v, want the zero Plan", plan)
				}
			})
		}
	}
}
