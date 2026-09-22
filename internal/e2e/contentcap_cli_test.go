// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package e2e

import (
	"fmt"
	"strings"
	"testing"
)

// TestCLIStoreRejectsOversizedContentAndTags proves D-01/D-10's memory
// content/tags caps end to end through the REAL engram binary's `store`
// verb against a REAL `engram serve` (headless Connect, static-token auth).
//
// The CLI is a Connect client with no cap of its own — it cannot know the
// server's configured value — so this is not a second copy of the caps
// asserted anywhere else; it proves the server-side cap actually reaches an
// operator through the real binary's process exit status and stderr text,
// the one seam internal/server's in-process tests cannot reach.
func TestCLIStoreRejectsOversizedContentAndTags(t *testing.T) {
	srv := startServer(t, map[string]string{
		"ENGRAM_CONNECT_HEADLESS":           "true",
		"ENGRAM_SERVICE_AUTH_STATIC_TOKENS": "e2e-cap=tok-e2e-cap",
	})
	env := map[string]string{"ENGRAM_TOKEN": "tok-e2e-cap"}
	commonArgs := []string{
		"store", "--server", srv.baseURL(), "--output", "text",
		"--scope", "repo:e2e-cap", "--source", "user-said", "--category", "decision",
	}

	t.Run("at-cap", func(t *testing.T) {
		content := strings.Repeat("x", 65536)
		args := append(append([]string{}, commonArgs...), "--content", content)
		stdout, stderr, code := runCLIEnv(t, env, args...)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
		if !strings.HasPrefix(stdout, "stored: ") {
			t.Fatalf("stdout = %q, want prefix %q\nstderr:\n%s", stdout, "stored: ", stderr)
		}
	})

	t.Run("content-over-cap", func(t *testing.T) {
		content := strings.Repeat("x", 65537)
		args := append(append([]string{}, commonArgs...), "--content", content)
		stdout, stderr, code := runCLIEnv(t, env, args...)
		if code != 2 {
			t.Fatalf("exit code = %d, want 2\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
		if !strings.Contains(stderr, "field=content hint=too_long") {
			t.Fatalf("stderr = %q, want to contain %q", stderr, "field=content hint=too_long")
		}
	})

	t.Run("too-many-tags", func(t *testing.T) {
		tags := make([]string, 129)
		for i := range tags {
			tags[i] = fmt.Sprintf("t%03d", i)
		}
		args := append(append([]string{}, commonArgs...),
			"--content", "small content", "--tags", strings.Join(tags, ","))
		stdout, stderr, code := runCLIEnv(t, env, args...)
		if code != 2 {
			t.Fatalf("exit code = %d, want 2\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
		if !strings.Contains(stderr, "field=tags hint=too_many") {
			t.Fatalf("stderr = %q, want to contain %q", stderr, "field=tags hint=too_many")
		}
	})

	t.Run("tag-too-long", func(t *testing.T) {
		tag := strings.Repeat("t", 129)
		args := append(append([]string{}, commonArgs...),
			"--content", "small content", "--tags", tag)
		stdout, stderr, code := runCLIEnv(t, env, args...)
		if code != 2 {
			t.Fatalf("exit code = %d, want 2\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
		if !strings.Contains(stderr, "field=tags hint=too_long") {
			t.Fatalf("stderr = %q, want to contain %q", stderr, "field=tags hint=too_long")
		}
	})
}
