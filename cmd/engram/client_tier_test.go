// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// clientCommandUsePattern matches the Use string of a cobra.Command literal
// declared in a client_*.go file and captures its first word, the verb a
// caller types. Every client verb in this package is declared as a
// package-level `var <x>Cmd = &cobra.Command{ Use: "<verb> ...", ...}`
// literal, so the first word of Use is also the key commandKey resolves.
var clientCommandUsePattern = regexp.MustCompile(`(?s)&cobra\.Command\{\s*Use:\s*"([a-z][a-z-]*)`)

// TestClientFilesDeclareServerFlag is the structural half of the client
// tier's classification (cmdwalk.go's operatorCommands): a command is
// client tier because its OWN flag set declares "server", which only
// addClientFlags/addConnectionFlags register. A verb file that forgets
// that call silently joins every operator-tier gate (the --output
// fixture gate, the zero-disables timeout group) and fails confusingly
// far from the cause. This test names the cause: every cobra.Command
// literal in a cmd/engram/client_*.go production file must resolve to a
// live top-level command that declares the server flag. It goes red the
// moment one verb's addClientFlags call is removed.
func TestClientFilesDeclareServerFlag(t *testing.T) {
	files, err := filepath.Glob("client_*.go")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	live := map[string]bool{}
	for _, cmd := range walkCommands(rootCmd, commandWalkSkip) {
		live[commandKey(cmd)] = cmd.Flags().Lookup("server") != nil
	}
	checked := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, m := range clientCommandUsePattern.FindAllStringSubmatch(string(src), -1) {
			verb := m[1]
			checked++
			declares, ok := live[verb]
			if !ok {
				t.Errorf("%s declares a cobra.Command with Use %q, which is not a live top-level command", f, verb)
				continue
			}
			if !declares {
				t.Errorf("%s: command %q declares no \"server\" flag — it is missing addClientFlags and would be classified operator tier", f, verb)
			}
		}
	}
	// Positive control: search, list, store, get and migration-status
	// already live in client_*.go files, so a walk that found fewer than
	// five literals did not read the files it claims to have read.
	if checked < 5 {
		t.Fatalf("checked %d client command literals, want at least 5 — the client_*.go scan did not reach the verbs", checked)
	}
}
