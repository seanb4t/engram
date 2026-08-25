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
