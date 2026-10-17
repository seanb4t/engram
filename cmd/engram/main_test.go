// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"os"
	"testing"
)

// TestMain points XDG_CONFIG_HOME at an empty directory for the whole test
// binary, so no test reads or writes the developer's real login cache
// (credentialsPath). A test that needs its own cache sets XDG_CONFIG_HOME
// again with t.Setenv.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "engram-cmd-test-config-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain:", err)
		os.Exit(1)
	}
	if err := os.Setenv("XDG_CONFIG_HOME", dir); err != nil {
		fmt.Fprintln(os.Stderr, "TestMain:", err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
