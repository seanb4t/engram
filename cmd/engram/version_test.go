// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestVersionJSONLane pins D-06: the json lane is exactly one document whose
// only key is "version", with a non-empty string value. resetClientFlags(t)
// runs first — see this file's package-level reset-discipline note in
// clienttest_test.go:131-155: versionCmd is a package-level var shared by
// the whole test binary, and runClient resets nothing, so a leaked
// --output json latch from this test would silently contaminate
// TestVersionTextLane if it ran afterward in the same binary.
func TestVersionJSONLane(t *testing.T) {
	resetClientFlags(t)
	stdout, _, err := runClient(t, "version", "--output", "json")
	if err != nil {
		t.Fatalf("runClient(version --output json) returned error: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	if len(doc) != 1 {
		t.Fatalf("len(doc) = %d, want 1 (doc: %#v) — D-06 pins the payload to exactly one key", len(doc), doc)
	}
	v, ok := doc["version"].(string)
	if !ok || v == "" {
		t.Fatalf(`doc["version"] = %#v, want a non-empty string`, doc["version"])
	}
}

// TestVersionTextLane pins D-01/D-07: with no flags, stdout is the bare
// version string plus exactly one trailing newline — no headline, no blank
// line, no padded label/value row. This is the behavioral proof that the
// shared operator render path (renderOperator) is not in use: that path
// always emits a headline first, so a bare-scalar assertion cannot pass
// through it. runClient's stdout is always a *bytes.Buffer (never a TTY),
// which is also half of the D-01 default-detection gate.
func TestVersionTextLane(t *testing.T) {
	resetClientFlags(t)
	stdout, _, err := runClient(t, "version")
	if err != nil {
		t.Fatalf("runClient(version) returned error: %v", err)
	}

	if !strings.HasSuffix(stdout, "\n") {
		t.Fatalf("stdout %q does not end in a newline", stdout)
	}
	body := strings.TrimSuffix(stdout, "\n")
	if body == "" {
		t.Fatalf("stdout %q is empty after trimming the trailing newline", stdout)
	}
	if strings.Contains(body, "\n") {
		t.Fatalf("stdout %q contains more than one line — want a bare scalar", stdout)
	}
}

// TestVersionExplicitTextLane pins the explicit --output text branch
// separately from the no-flags default (M-2 from cycle-1 review): the
// default branch and the explicit branch are separately reachable in RunE,
// so pinning only the default would let an implementation satisfy
// --output text through some other path (or drop it) with every other test
// still green.
func TestVersionExplicitTextLane(t *testing.T) {
	resetClientFlags(t)
	explicit, _, err := runClient(t, "version", "--output", "text")
	if err != nil {
		t.Fatalf("runClient(version --output text) returned error: %v", err)
	}

	resetClientFlags(t)
	implicit, _, err := runClient(t, "version")
	if err != nil {
		t.Fatalf("runClient(version) returned error: %v", err)
	}

	if explicit != implicit {
		t.Fatalf("explicit --output text = %q, default (no flags) = %q, want byte-identical", explicit, implicit)
	}
}

// TestVersionTextEqualsJSON pins D-07's required invariant: the text lane
// and the json lane are produced by two separate render paths and must not
// drift. The string TestVersionTextLane observes, trimmed of its trailing
// newline, must be byte-equal to the "version" field TestVersionJSONLane
// decodes.
func TestVersionTextEqualsJSON(t *testing.T) {
	resetClientFlags(t)
	textOut, _, err := runClient(t, "version")
	if err != nil {
		t.Fatalf("runClient(version) returned error: %v", err)
	}

	resetClientFlags(t)
	jsonOut, _, err := runClient(t, "version", "--output", "json")
	if err != nil {
		t.Fatalf("runClient(version --output json) returned error: %v", err)
	}

	var doc struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &doc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", jsonOut, err)
	}

	textVersion := strings.TrimSuffix(textOut, "\n")
	if textVersion != doc.Version {
		t.Fatalf("text lane version %q != json lane version %q", textVersion, doc.Version)
	}
}

// TestVersionOutputFlagDefault pins D-01: the flag's registered default is
// exactly "text", the hardcoded literal — never config.FlagDefault("output")
// (which resolves to "", i.e. auto-detect from stdout). Combined with
// TestVersionTextLane (whose stdout is always non-TTY), this is the gate
// that catches TTY detection being reintroduced: a default that
// auto-detected from stdout would make the text-lane test emit JSON, and
// this test's assertion on DefValue would independently go red.
func TestVersionOutputFlagDefault(t *testing.T) {
	resetClientFlags(t)
	f := versionCmd.Flags().Lookup("output")
	if f == nil {
		t.Fatal(`versionCmd.Flags().Lookup("output") = nil, want the registered flag`)
	}
	if f.DefValue != "text" {
		t.Fatalf(`versionCmd.Flags().Lookup("output").DefValue = %q, want "text"`, f.DefValue)
	}
}

// TestVersionExpect pins --expect, the cask install gate: the normal output
// is printed whether or not the versions match, a mismatch exits 7 with one
// error line naming both versions, an empty value is a usage error before
// any output, and the comparison is an exact string match.
func TestVersionExpect(t *testing.T) {
	// A release-shaped version: an implementation that strips a leading v
	// from --expect would then match in the v-prefix case, which wants exit 7.
	saved := version
	version = "0.30.0"
	t.Cleanup(func() { version = saved })

	cases := []struct {
		name       string
		args       []string
		wantStdout string
		wantCode   int
		wantErrHas []string
	}{
		{name: "omitted", args: []string{"version"}, wantStdout: "0.30.0\n", wantCode: exitOK},
		{name: "match", args: []string{"version", "--expect", "0.30.0"}, wantStdout: "0.30.0\n", wantCode: exitOK},
		{name: "text mismatch", args: []string{"version", "--expect", "0.31.0"}, wantStdout: "0.30.0\n", wantCode: exitFindings, wantErrHas: []string{"0.30.0", "0.31.0"}},
		{name: "json mismatch", args: []string{"version", "--expect", "0.31.0", "--output", "json"}, wantStdout: `{"version":"0.30.0"}` + "\n", wantCode: exitFindings, wantErrHas: []string{"0.30.0", "0.31.0"}},
		{name: "v prefix", args: []string{"version", "--expect", "v0.30.0"}, wantStdout: "0.30.0\n", wantCode: exitFindings, wantErrHas: []string{"0.30.0", "v0.30.0"}},
		{name: "empty", args: []string{"version", "--expect", ""}, wantStdout: "", wantCode: exitUsage, wantErrHas: []string{"--expect"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetClientFlags(t)
			stdout, stderr, err := runClient(t, tc.args...)
			if stdout != tc.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, tc.wantStdout)
			}
			// rootCmd silences cobra's own error printing, so the captured
			// stderr is empty; the package-level Execute (root.go) prints
			// err as the single "Error: ..." line the user sees.
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
			if got := exitCodeFromError(err); got != tc.wantCode {
				t.Fatalf("exit code = %d, want %d (err: %v)", got, tc.wantCode, err)
			}
			if err == nil {
				return
			}
			msg := err.Error()
			if strings.Contains(msg, "\n") {
				t.Errorf("error %q spans more than one line", msg)
			}
			for _, want := range tc.wantErrHas {
				if !strings.Contains(msg, want) {
					t.Errorf("error %q does not name %q", msg, want)
				}
			}
		})
	}
}
