// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
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
