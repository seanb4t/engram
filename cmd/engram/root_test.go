// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestRootRejectsLegacyEnv(t *testing.T) {
	t.Setenv("MEM_LITELLM_URL", "http://old:4000")
	t.Cleanup(func() { rootCmd.SetArgs(nil) }) // don't leak args into sibling tests
	rootCmd.SetArgs([]string{"version"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected PersistentPreRunE to reject MEM_LITELLM_URL")
	}
	if !strings.Contains(err.Error(), "ENGRAM_OPENAI_BASE_URL") {
		t.Errorf("error %q should map the retired var to its replacement", err.Error())
	}
}

// TestExitCodeFromError is the backward-compatibility proof for every
// existing operator command: a plain error (what every pre-existing
// command returns) must still map to 1.
func TestExitCodeFromError(t *testing.T) {
	if got := exitCodeFromError(nil); got != 0 {
		t.Errorf("exitCodeFromError(nil) = %d, want 0", got)
	}
	if got := exitCodeFromError(errors.New("plain")); got != 1 {
		t.Errorf("exitCodeFromError(plain error) = %d, want 1", got)
	}
	if got := exitCodeFromError(&cliError{code: 4, err: errors.New("not found")}); got != 4 {
		t.Errorf("exitCodeFromError(*cliError code=4) = %d, want 4", got)
	}
	wrapped := fmt.Errorf("wrapped: %w", &cliError{code: 4, err: errors.New("not found")})
	if got := exitCodeFromError(wrapped); got != 4 {
		t.Errorf("exitCodeFromError(fmt.Errorf-wrapped *cliError) = %d, want 4 (errors.As walk)", got)
	}
}
