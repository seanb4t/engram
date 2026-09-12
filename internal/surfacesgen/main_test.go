// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanb4t/engram/internal/setup"
	"github.com/seanb4t/engram/internal/setupgen"
)

func checkFixture(t *testing.T, drift bool) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	body, err := setupgen.Render(setup.ClaudeCode.Plan)
	if err != nil {
		t.Fatal(err)
	}
	if drift {
		body = "stale\n"
	}
	content := "Authored prefix.\n<!-- engram:rule:start setup-commands -->\n" + body + "\n<!-- engram:rule:end setup-commands -->\nAuthored suffix.\n"
	path := filepath.Join(root, setupgen.Path)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	// This is the first regular writer target. If check dispatch falls through
	// to run(), it overwrites this region before failing on other absent files.
	sentinel := filepath.Join(root, toolBlastRadiusPath)
	if err := os.MkdirAll(filepath.Dir(sentinel), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sentinel, []byte(sentinelContent), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, path, content
}

const sentinelContent = "Before.\n<!-- engram:rule:start scope-required-unless-cross-spine -->\nSENTINEL\n<!-- engram:rule:end scope-required-unless-cross-spine -->\nAfter.\n"

func assertFixtureUnchanged(t *testing.T, root, path, before string) {
	t.Helper()
	for name, want := range map[string]string{path: before, filepath.Join(root, toolBlastRadiusPath): sentinelContent} {
		got, err := os.ReadFile(name)
		if err != nil || string(got) != want {
			t.Fatalf("check changed %s: %v", name, err)
		}
	}
}

func TestCheckDispatch(t *testing.T) {
	for _, drift := range []bool{false, true} {
		name := "exact"
		if drift {
			name = "drift"
		}
		t.Run(name, func(t *testing.T) {
			root, path, before := checkFixture(t, drift)
			t.Chdir(root)
			err := dispatch([]string{"--check-setup"})
			if (err != nil) != drift {
				t.Fatalf("dispatch error = %v, drift = %v", err, drift)
			}
			assertFixtureUnchanged(t, root, path, before)
		})
	}
}

// Execute the actual main in a subprocess so os.Exit and stderr are tested.
func TestCheckMainHelper(t *testing.T) {
	if os.Getenv("ENGRAM_SURFACESGEN_TEST_MAIN") != "1" {
		return
	}
	os.Args = []string{"surfacesgen", "--check-setup"}
	main()
	os.Exit(0)
}

func TestCheckSubprocessExit(t *testing.T) {
	for _, drift := range []bool{false, true} {
		name := "exact"
		if drift {
			name = "drift"
		}
		t.Run(name, func(t *testing.T) {
			root, path, before := checkFixture(t, drift)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(executable, "-test.run=^TestCheckMainHelper$")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "ENGRAM_SURFACESGEN_TEST_MAIN=1")
			out, err := cmd.CombinedOutput()
			if drift {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
					t.Fatalf("exit = %v, output = %s", err, out)
				}
				if !strings.Contains(string(out), setupgen.Path) || !strings.Contains(string(out), "task surfaces:gen") {
					t.Fatalf("missing diagnostic: %s", out)
				}
			} else if err != nil {
				t.Fatalf("exact check: %v, %s", err, out)
			}
			assertFixtureUnchanged(t, root, path, before)
		})
	}
}

func TestDispatchRejectsUnknownArgs(t *testing.T) {
	root, path, before := checkFixture(t, false)
	t.Chdir(root)
	for _, args := range [][]string{{"--unknown"}, {"--check-setup", "extra"}} {
		if err := dispatch(args); err == nil {
			t.Fatalf("accepted %q", args)
		}
		assertFixtureUnchanged(t, root, path, before)
	}
}
