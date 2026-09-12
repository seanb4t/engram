// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/seanb4t/engram/internal/setup"
	"github.com/seanb4t/engram/internal/setupgen"
)

func TestSetupGeneratedInvocations(t *testing.T) {
	for _, c := range setupgen.Cases() {
		t.Run(c.Options.Auth, func(t *testing.T) {
			assertGeneratedPreviewArgs(t, c)
			for _, lane := range []string{"preview", "apply"} {
				t.Run(lane, func(t *testing.T) {
					resetClientFlags(t)
					resetCommandFlagState(t, setupCmd)
					var calls [][]string
					env := fakeSetupEnvWithRun(func(_ context.Context, path string, args []string) (setup.RunResult, error) {
						calls = append(calls, append([]string{filepath.Base(path)}, args...))
						return setup.RunResult{}, nil
					}, "claude", "codex", "opencode")
					env.Getenv = func(key string) string {
						if key != "XDG_CONFIG_HOME" {
							t.Errorf("unexpected environment read %q", key)
						}
						return ""
					}
					withFakeSetupEnv(t, env)
					mutations := 0
					write, mkdir := skillsEnv.WriteFile, skillsEnv.MkdirAll
					skillsEnv.WriteFile = func(path string, data []byte, mode os.FileMode) error {
						mutations++
						return write(path, data, mode)
					}
					skillsEnv.MkdirAll = func(path string, mode os.FileMode) error {
						mutations++
						return mkdir(path, mode)
					}
					args := append(slices.Clone(c.DelegationArgs[1:]), "--output", "json")
					if lane == "apply" {
						args = append(args, "--apply")
					}
					stdout, stderr, err := runClient(t, args...)
					wantExit := 0
					if lane == "apply" && c.Options.Auth == "oauth-client" {
						wantExit = exitPartial // OpenCode remains unsupported.
					}
					if got := exitCodeFromError(err); got != wantExit {
						t.Fatalf("exit=%d, want %d: %v (stderr=%q)", got, wantExit, err, stderr)
					}
					var doc setupReportDoc
					if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
						t.Fatal(err)
					}
					runtimes, err := setup.Select(nil)
					if err != nil {
						t.Fatal(err)
					}
					if len(doc.Runtimes) != len(runtimes) || len(doc.Runtimes) != 3 {
						t.Fatalf("default detected native runtimes were scoped away: %+v", doc.Runtimes)
					}
					var wantCalls [][]string
					for i, rt := range runtimes {
						row := doc.Runtimes[i]
						if row.Name != rt.Name() || !row.Present || row.Name == "generic" {
							t.Fatalf("unexpected detected runtime row: %+v", row)
						}
						plan, planErr := rt.Plan(env, c.Options)
						if errors.Is(planErr, setup.ErrAuthModeUnsupported) {
							if row.Name != "opencode" || c.Options.Auth != "oauth-client" || row.Outcome != "failed" || row.Command != "" || !strings.Contains(row.Reason, "oauth-client") {
								t.Fatalf("unsupported auth row lost: %+v", row)
							}
							continue
						}
						if planErr != nil {
							t.Fatal(planErr)
						}
						if row.Command != plan.Display() {
							t.Errorf("%s command=%q, want real Plan %q", row.Name, row.Command, plan.Display())
						}
						assertGeneratedAuthWiring(t, c, row)
						wantOutcome := "would-write"
						if lane == "apply" {
							wantOutcome = "wrote"
						}
						if row.Outcome != wantOutcome {
							t.Errorf("%s outcome=%q, want %q", row.Name, row.Outcome, wantOutcome)
						}
						if len(plan.Probe) > 0 {
							wantCalls = append(wantCalls, plan.Probe)
						}
						if lane == "apply" {
							for _, action := range plan.Actions {
								wantCalls = append(wantCalls, action.Args)
							}
							if len(plan.Probe) > 0 {
								wantCalls = append(wantCalls, plan.Probe)
							}
						}
					}
					if !reflect.DeepEqual(calls, wantCalls) {
						t.Errorf("captured argv=%q, want Plan-authored sequence=%q", calls, wantCalls)
					}
					if lane == "preview" && mutations != 0 || lane == "apply" && mutations == 0 {
						t.Errorf("%s skills mutations=%d", lane, mutations)
					}
				})
			}
		})
	}
	t.Run("unknown-flag", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		env := fakeSetupEnv()
		env.LookPath = func(string) (string, error) {
			t.Fatal("invalid flag reached runtime detection")
			return "", nil
		}
		withFakeSetupEnv(t, env)
		args := slices.Clone(setupgen.Cases()[0].DelegationArgs[1:])
		args[1] = "--setupgen-unknown-url"
		_, stderr, err := runClient(t, args...)
		if err == nil || !strings.Contains(err.Error(), "unknown flag: --setupgen-unknown-url") {
			t.Fatalf("Cobra accepted mutated generated flag: err=%v stderr=%q", err, stderr)
		}
	})
}

func assertGeneratedPreviewArgs(t *testing.T, c setupgen.Case) {
	t.Helper()
	args := c.DelegationArgs
	if len(args) < 2 || args[0] != "engram" || args[1] != "setup" {
		t.Fatalf("not a setup invocation: %q", args)
	}
	for i := 2; i < len(args); i += 2 {
		if args[i] == "--apply" || args[i] == "--runtime" || args[i] == "--token-file" {
			t.Fatalf("preview grants mutation, scopes detection, or uses native token-file: %q", args)
		}
		if !strings.HasPrefix(args[i], "--") || setupCmd.Flags().Lookup(strings.TrimPrefix(args[i], "--")) == nil || i+1 >= len(args) {
			t.Fatalf("generated option does not conform to Cobra: %q", args[i:])
		}
		want, ok := map[string]string{"--url": c.Options.URL, "--auth": c.Options.Auth, "--client-id": c.Options.ClientID}[args[i]]
		if !ok || args[i+1] != want {
			t.Fatalf("generated option %q=%q differs from shared auth inputs", args[i], args[i+1])
		}
	}
	if c.Options.Auth == "oauth-client" {
		if c.Options.ClientID == "" || c.Options.ClientID == "<id>" || !slices.Contains(args, "--client-id") {
			t.Fatal("OAuth-client must carry a real synthetic non-secret client ID")
		}
	} else if c.Options.ClientID != "" || slices.Contains(args, "--client-id") {
		t.Fatal("only OAuth-client may carry client ID")
	}
}

func assertGeneratedAuthWiring(t *testing.T, c setupgen.Case, row setupRuntimeRow) {
	t.Helper()
	if c.Options.Auth == "oauth-client" {
		flag := "--client-id "
		if row.Name == "codex" {
			flag = "--oauth-client-id "
		}
		if !strings.Contains(row.Command, flag+c.Options.ClientID) {
			t.Errorf("non-secret client ID missing from %s: %q", row.Name, row.Command)
		}
	}
	if c.Options.Auth == "bearer" && row.Name == "claude-code" && !strings.Contains(row.Command, "'Authorization: Bearer ${ENGRAM_TOKEN}'") {
		t.Errorf("bearer literal environment reference lost: %q", row.Command)
	}
}
