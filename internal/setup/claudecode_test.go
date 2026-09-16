// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestClaudeCodePlan is the table over all four auth modes proving
// claude-code's Plan() authors the checkpoint-approved two-action write
// sequence (03-02-PLAN.md Task 1's decision, correcting 03-CONTEXT.md
// D-08's falsified "mcp add overwrites" premise, 03-RESEARCH.md Pitfall
// 1): a tolerant clear-the-slot `claude mcp remove` first, then a fatal
// `claude mcp add` — for EVERY auth mode, not only bearer, because `claude
// mcp add` refuses on an existing name independent of auth mode.
func TestClaudeCodePlan(t *testing.T) {
	const url = "https://engram.example.com/mcp"
	wantRemove := []string{"claude", "mcp", "remove", "engram", "--scope", "user"}
	wantProbe := []string{"claude", "mcp", "get", "engram"}
	// Phase 4 widened Plan() to consult env.HomeDir() for the SkillTarget
	// it now authors (claudecode.go:114); a fake keeps this test isolated
	// from the real $HOME rather than reaching os.UserHomeDir() as a side
	// effect of asserting the (unrelated) registration Args (WR-01).
	env := Environment{
		LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		Getenv:   func(string) string { return "" },
		HomeDir:  func() (string, error) { return "/home/fake", nil },
	}

	tests := []struct {
		auth    string
		wantAdd []string
	}{
		{
			auth:    "oauth",
			wantAdd: []string{"claude", "mcp", "add", "--transport", "http", "engram", url, "--scope", "user"},
		},
		{
			auth:    "none",
			wantAdd: []string{"claude", "mcp", "add", "--transport", "http", "engram", url, "--scope", "user"},
		},
		{
			auth: "oauth-client",
			wantAdd: []string{"claude", "mcp", "add", "--transport", "http", "engram", url,
				"--scope", "user", "--client-id", "test-client", "--client-secret", "--callback-port", "8765"},
		},
		{
			auth: "bearer",
			wantAdd: []string{"claude", "mcp", "add", "--transport", "http", "engram", url,
				"--scope", "user", "--header", "Authorization: Bearer ${ENGRAM_TOKEN}"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.auth, func(t *testing.T) {
			plan, err := ClaudeCode.Plan(env, Options{URL: url, Auth: tc.auth, ClientID: "test-client"})
			if err != nil {
				t.Fatalf("Plan(auth=%q): %v", tc.auth, err)
			}
			if len(plan.Actions) != 2 {
				t.Fatalf("Plan(auth=%q): len(Actions) = %d, want 2 (tolerant remove, fatal add)", tc.auth, len(plan.Actions))
			}
			if !plan.Actions[0].Tolerant {
				t.Errorf("Plan(auth=%q): Actions[0].Tolerant = false, want true (the clear-the-slot action)", tc.auth)
			}
			if !reflect.DeepEqual(plan.Actions[0].Args, wantRemove) {
				t.Errorf("Plan(auth=%q): Actions[0].Args = %v, want %v", tc.auth, plan.Actions[0].Args, wantRemove)
			}
			if plan.Actions[1].Tolerant {
				t.Errorf("Plan(auth=%q): Actions[1].Tolerant = true, want false (the registration action is fatal)", tc.auth)
			}
			if !reflect.DeepEqual(plan.Actions[1].Args, tc.wantAdd) {
				t.Errorf("Plan(auth=%q): Actions[1].Args = %v, want %v", tc.auth, plan.Actions[1].Args, tc.wantAdd)
			}
			if !reflect.DeepEqual(plan.Probe, wantProbe) {
				t.Errorf("Plan(auth=%q): Probe = %v, want %v (D-09: authored in the same Plan() call)", tc.auth, plan.Probe, wantProbe)
			}
		})
	}
}

// TestClaudeCodeBearerHeaderIsAnEnvVarReference proves claude-code's
// bearer mode (D-05, D-06) authors the --header value as a shell-style
// ${ENGRAM_TOKEN} variable REFERENCE, verified end-to-end against claude
// 2.1.265 (03-RESEARCH.md Pattern 2) to resolve at connect time while
// `claude mcp get` and the on-disk config both echo back only the literal
// unexpanded text — never a credential value and never Options.TokenFile's
// path.
func TestClaudeCodeBearerHeaderIsAnEnvVarReference(t *testing.T) {
	const sentinelCredential = "SUPER-SECRET-VALUE-MUST-NEVER-APPEAR-9f3e2a"
	const sentinelTokenFile = "/home/u/.engram/token-sentinel"

	// Fake home, for the same WR-01 isolation reason as TestClaudeCodePlan
	// above: Plan() now calls env.HomeDir() to author the SkillTarget this
	// test never asserts against.
	env := Environment{
		LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		Getenv:   func(string) string { return "" },
		HomeDir:  func() (string, error) { return "/home/fake", nil },
	}
	plan, err := ClaudeCode.Plan(env, Options{URL: "https://x", Auth: "bearer", TokenFile: sentinelTokenFile})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Actions) != 2 {
		t.Fatalf("len(Actions) = %d, want 2", len(plan.Actions))
	}
	headerArg := findHeaderArg(t, plan.Actions[1].Args)

	if !strings.Contains(headerArg, "${ENGRAM_TOKEN}") {
		t.Errorf("header arg = %q, want it to contain the ${ENGRAM_TOKEN} variable reference form", headerArg)
	}
	if strings.Contains(headerArg, sentinelCredential) {
		t.Errorf("header arg = %q, contains a sentinel credential value — must never carry a resolved secret", headerArg)
	}
	if strings.Contains(headerArg, sentinelTokenFile) {
		t.Errorf("header arg = %q, contains the sentinel token-file path — D-06 requires an env-var reference, never a path", headerArg)
	}
}

// TestClaudeCodeSkillTarget asserts claude-code's own authored SkillTarget
// (claudecode.go:118-121) against a fake environment with a deterministic
// home, across every supported auth mode, mirroring
// TestCodexSkillTarget/TestOpenCodeSkillTarget (WR-02): claude-code's
// destination had no equivalent independent assertion, so a typo or
// segment reordering in the authored Dir would have passed every existing
// test undetected.
func TestClaudeCodeSkillTarget(t *testing.T) {
	const url = "https://engram.example.com/mcp"
	env := fakeEnv()
	wantDir := filepath.Join("/home/fake", ".claude", "skills")
	wantTarget := SkillTarget{Format: SkillFormatNative, Dir: wantDir}

	modes := []string{"oauth", "none", "oauth-client", "bearer"}
	for _, mode := range modes {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			plan, err := ClaudeCode.Plan(env, Options{URL: url, Auth: mode, ClientID: "test-client"})
			if err != nil {
				t.Fatalf("Plan(%q): %v", mode, err)
			}
			if !reflect.DeepEqual(plan.Skills, wantTarget) {
				t.Errorf("Plan(%q).Skills = %+v, want %+v", mode, plan.Skills, wantTarget)
			}
		})
	}
}

// TestClaudeCodePlanFailsWhenHomeUnresolvable asserts a fake whose
// home-directory function errors makes Plan() return an error naming the
// runtime, rather than silently producing an empty-destination
// SkillTarget — mirroring TestCodexPlanFailsWhenHomeUnresolvable /
// TestOpenCodePlanFailsWhenHomeUnresolvable (WR-02).
func TestClaudeCodePlanFailsWhenHomeUnresolvable(t *testing.T) {
	env := Environment{
		LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		Getenv:   func(string) string { return "" },
		HomeDir:  func() (string, error) { return "", errors.New("boom") },
	}
	_, err := ClaudeCode.Plan(env, Options{URL: "https://engram.example.com/mcp", Auth: "oauth"})
	if err == nil {
		t.Fatal("Plan: want an error when the home directory is unresolvable, got nil")
	}
	if !strings.Contains(err.Error(), "claude-code") {
		t.Errorf("Plan err = %q, want it to name the runtime", err.Error())
	}
}

// findHeaderArg locates the argv element immediately following --header
// in args, failing the test if --header is not present.
func findHeaderArg(t *testing.T, args []string) string {
	t.Helper()
	for i, a := range args {
		if a == "--header" && i+1 < len(args) {
			return args[i+1]
		}
	}
	t.Fatalf("no --header flag found in Args %v", args)
	return ""
}

// TestClaudeCodeHeaders proves claude-code appends one sorted "--header"
// pair per entry of opts.Headers to the SAME claude mcp add action, in
// every auth mode, after every shipped argument (and, for bearer, after
// the auth-mode header) — D-01, D-04, D-08. Two headers are supplied in
// REVERSE sorted order to prove sortedHeaders (not caller order) governs
// the rendered order, and a mixed-case discriminating pair proves the
// sort key is strings.ToLower(Name), not raw byte order.
func TestClaudeCodeHeaders(t *testing.T) {
	const url = "https://engram.example.com/mcp"
	env := Environment{
		LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		Getenv:   func(string) string { return "" },
		HomeDir:  func() (string, error) { return "/home/fake", nil },
	}

	headers := []HeaderSpec{
		{Name: "x-gateway-api-key", EnvVar: "GATEWAY_KEY"},
		{Name: "CF-Access-Client-Id", EnvVar: "CF_ID"},
	}
	wantExtra := []string{
		"--header", "CF-Access-Client-Id: ${CF_ID}",
		"--header", "x-gateway-api-key: ${GATEWAY_KEY}",
	}

	tests := []struct {
		auth    string
		wantAdd []string
	}{
		{
			auth:    "oauth",
			wantAdd: append([]string{"claude", "mcp", "add", "--transport", "http", "engram", url, "--scope", "user"}, wantExtra...),
		},
		{
			auth:    "none",
			wantAdd: append([]string{"claude", "mcp", "add", "--transport", "http", "engram", url, "--scope", "user"}, wantExtra...),
		},
		{
			auth: "oauth-client",
			wantAdd: append([]string{"claude", "mcp", "add", "--transport", "http", "engram", url,
				"--scope", "user", "--client-id", "test-client", "--client-secret", "--callback-port", "8765"}, wantExtra...),
		},
		{
			auth: "bearer",
			wantAdd: append([]string{"claude", "mcp", "add", "--transport", "http", "engram", url,
				"--scope", "user", "--header", "Authorization: Bearer ${ENGRAM_TOKEN}"}, wantExtra...),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.auth, func(t *testing.T) {
			plan, err := ClaudeCode.Plan(env, Options{URL: url, Auth: tc.auth, ClientID: "test-client", Headers: headers})
			if err != nil {
				t.Fatalf("Plan(auth=%q): %v", tc.auth, err)
			}
			if len(plan.Actions) != 2 {
				t.Fatalf("Plan(auth=%q): len(Actions) = %d, want 2 (headers never author a third action)", tc.auth, len(plan.Actions))
			}
			if !reflect.DeepEqual(plan.Actions[0], claudeCodeRemoveAction) {
				t.Errorf("Plan(auth=%q): Actions[0] = %#v, want claudeCodeRemoveAction unchanged", tc.auth, plan.Actions[0])
			}
			if !reflect.DeepEqual(plan.Actions[1].Args, tc.wantAdd) {
				t.Errorf("Plan(auth=%q): Actions[1].Args = %v, want %v", tc.auth, plan.Actions[1].Args, tc.wantAdd)
			}
		})
	}

	// The caller's slice must not be re-ordered in place (D-08:
	// sortedHeaders returns a clone).
	if headers[0].Name != "x-gateway-api-key" {
		t.Errorf("caller's Headers slice was reordered in place: headers[0].Name = %q, want %q", headers[0].Name, "x-gateway-api-key")
	}

	// Discriminating case-insensitive sort pair: byte order would put
	// "B-Key" before "a-key", but strings.ToLower order puts "a-key"
	// first.
	t.Run("case-insensitive-sort", func(t *testing.T) {
		mixed := []HeaderSpec{
			{Name: "B-Key", EnvVar: "B2"},
			{Name: "a-key", EnvVar: "A2"},
		}
		plan, err := ClaudeCode.Plan(env, Options{URL: url, Auth: "bearer", Headers: mixed})
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		args := plan.Actions[1].Args
		idxA, idxB := -1, -1
		for i, a := range args {
			switch a {
			case "a-key: ${A2}":
				idxA = i
			case "B-Key: ${B2}":
				idxB = i
			}
		}
		if idxA == -1 || idxB == -1 {
			t.Fatalf("Args %v missing expected header elements", args)
		}
		if idxA > idxB {
			t.Errorf("Args %v: want a-key before B-Key (case-insensitive sort), got a-key at %d, B-Key at %d", args, idxA, idxB)
		}
	})

	// Second discriminating pair: "A-Key" and "b-key" — byte order agrees
	// with case-insensitive order here, so this pins the simple case
	// alongside the discriminating one above.
	t.Run("simple-sort", func(t *testing.T) {
		mixed := []HeaderSpec{
			{Name: "b-key", EnvVar: "B"},
			{Name: "A-Key", EnvVar: "A"},
		}
		plan, err := ClaudeCode.Plan(env, Options{URL: url, Auth: "bearer", Headers: mixed})
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		want := []string{
			"claude", "mcp", "add", "--transport", "http", "engram", url,
			"--scope", "user", "--header", "Authorization: Bearer ${ENGRAM_TOKEN}",
			"--header", "A-Key: ${A}", "--header", "b-key: ${B}",
		}
		if !reflect.DeepEqual(plan.Actions[1].Args, want) {
			t.Errorf("Args = %v, want %v", plan.Actions[1].Args, want)
		}
	})

	t.Run("display-single-quoted", func(t *testing.T) {
		plan, err := ClaudeCode.Plan(env, Options{URL: url, Auth: "bearer", Headers: headers})
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		cmd := plan.Actions[1].Command()
		want := "--header 'Authorization: Bearer ${ENGRAM_TOKEN}' --header 'CF-Access-Client-Id: ${CF_ID}' --header 'x-gateway-api-key: ${GATEWAY_KEY}'"
		if !strings.Contains(cmd, want) {
			t.Errorf("Command() = %q, want it to contain %q", cmd, want)
		}
	})

	// Zero-header control: nil and empty both yield Args deep-equal to
	// TestClaudeCodePlan's bearer vector (REQ-header-bearer-unchanged's
	// package half).
	wantBearerNoHeader := []string{"claude", "mcp", "add", "--transport", "http", "engram", url,
		"--scope", "user", "--header", "Authorization: Bearer ${ENGRAM_TOKEN}"}
	for name, hs := range map[string][]HeaderSpec{"nil": nil, "empty": {}} {
		hs := hs
		t.Run("zero-header-"+name, func(t *testing.T) {
			plan, err := ClaudeCode.Plan(env, Options{URL: url, Auth: "bearer", Headers: hs})
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if !reflect.DeepEqual(plan.Actions[1].Args, wantBearerNoHeader) {
				t.Errorf("Args = %v, want %v", plan.Actions[1].Args, wantBearerNoHeader)
			}
		})
	}
}

// TestSortedHeadersTotalOrder proves sortedHeaders' comparator is a TOTAL
// order rather than depending on slices.SortFunc's stability (WR-01,
// 02-REVIEW.md): "a-key" and "A-key" compare equal under the primary
// strings.ToLower(Name) key, so the result is only deterministic if the
// byte-wise strings.Compare(a.Name, b.Name) tiebreak fires. Feeding both
// input orders and asserting the SAME output order each time is what
// distinguishes "genuinely total" from "happens to be stable today" — a
// caller that skips the CLI-boundary uniqueness/case-collision guard
// (Options.Headers' own doc comment) would otherwise get a rendered
// header order that flaps between runs depending on internal sort
// implementation details, silently violating D-08's ordering guarantee.
func TestSortedHeadersTotalOrder(t *testing.T) {
	lower := HeaderSpec{Name: "a-key", EnvVar: "LOWER"}
	upper := HeaderSpec{Name: "A-key", EnvVar: "UPPER"}
	// strings.Compare("A-key", "a-key") < 0 ('A' = 0x41 < 'a' = 0x61), so
	// the total order always places upper before lower, regardless of
	// input order.
	want := []HeaderSpec{upper, lower}

	for name, in := range map[string][]HeaderSpec{
		"lower-then-upper": {lower, upper},
		"upper-then-lower": {upper, lower},
	} {
		in := in
		t.Run(name, func(t *testing.T) {
			got := sortedHeaders(in)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("sortedHeaders(%v) = %v, want %v", in, got, want)
			}
		})
	}
}

// claudeGetFixture builds a "claude mcp get engram" fixture reproducing
// the EXACT framing .planning/phases/04-drift-detection-read-only/
// 04-OBSERVATIONS.md recorded for the `engram` entry: the entry-name
// line, "Scope:", the caller-supplied status block (claudeStatusConnected
// or claudeStatusFailedDial), "Type: http", "URL:", a "Headers:" block
// with one 4-space-indented "Name: value" line per element of headers, a
// blank line, and the trailing hint line — never a shape the record does
// not show.
// Shape: .planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md
// §"Claude Code — literal value" (claude 2.1.273, 2026-09-15).
func claudeGetFixture(headers []string, status string) string {
	var b strings.Builder
	b.WriteString("engram:\n")
	b.WriteString("  Scope: User config (available in all your projects)\n")
	b.WriteString(status)
	b.WriteString("  Type: http\n")
	b.WriteString("  URL: https://engram.example.com/mcp\n")
	b.WriteString("  Headers:\n")
	for _, h := range headers {
		b.WriteString("    " + h + "\n")
	}
	b.WriteString("\n")
	b.WriteString("To remove this server, run: claude mcp remove engram -s user\n")
	return b.String()
}

// claudeStatusConnected is a plausible connected-status block — NOT
// directly observed (04-OBSERVATIONS.md's capture was a failed dial), but
// the record's own "What this pins" section notes "a successful dial
// would presumably omit Issue:", which this reproduces: no Issue: line at
// all, Status is chrome regardless (Pitfall 4).
const claudeStatusConnected = "  Status: ✓ Connected\n"

// claudeStatusFailedDial is 04-OBSERVATIONS.md's VERBATIM failed-dial
// Status:/Issue: block (claude 2.1.273, 2026-09-15) — the exact two
// chrome lines the record captured for a dial to an unreachable URL.
// Shape: .planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md
// §"Claude Code — literal value".
const claudeStatusFailedDial = "  Status: ✘ Failed to connect\n  Issue: ConnectionRefused: Unable to connect. Is the computer able to access the url?\n"

// claudeNotFoundAfterRemove is 04-OBSERVATIONS.md's VERBATIM post-remove
// "mcp get" output (claude 2.1.273, 2026-09-15) — no "URL:" line, so
// Observe's framing rule (D-09) reports it unreadable. Quoted byte-for-
// byte, including the record's own "tosee" spacing.
// Shape: .planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md
// §"Claude Code — literal value".
const claudeNotFoundAfterRemove = `No MCP server named "engram". Configured servers: claude.ai Gmail, claude.ai Google Calendar, claude.ai Google Drive, clickhouse_ro, clickhouse_rw, clickstack, codegraph, context7 (and 16 more — run 'claude mcp list' tosee all)`

// claudeLiteralHeaderLine is the record's literal Headers: line — quoted
// verbatim into every literal-echo fixture that needs it (D-08).
// Shape: .planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md
// §"Claude Code — literal value".
const claudeLiteralHeaderLine = "x-litellm-api-key: sk-DO-NOT-COMMIT-literal-test-abc123"

// claudeReferenceHeaderLine is the record's bare-reference control for
// the same header — byte-identical framing, only the value differs
// (D-02).
// Shape: .planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md
// §"Claude Code — bare reference control".
const claudeReferenceHeaderLine = "x-litellm-api-key: ${LITELLM_KEY}"

// TestObserveClaudeCodeRegistration drives claudeCodeRuntime.Observe
// directly on scripted probe-output strings built from
// 04-OBSERVATIONS.md — no subprocess, no Environment (rule m45p2b4bp7).
func TestObserveClaudeCodeRegistration(t *testing.T) {
	dr, ok := ClaudeCode.(DriftRuntime)
	if !ok {
		t.Fatal("ClaudeCode does not implement DriftRuntime")
	}

	opts := Options{
		URL:     "https://engram.example.com/mcp",
		Auth:    "bearer",
		Headers: []HeaderSpec{{Name: "x-gateway-api-key", EnvVar: "GATEWAY_KEY"}},
	}
	noExtraOpts := Options{URL: "https://engram.example.com/mcp", Auth: "bearer"}

	bearerHeaders := []string{
		"Authorization: " + claudeCodeBearerForm,
		"x-gateway-api-key: ${GATEWAY_KEY}",
	}

	var alreadyCorrect Observation

	t.Run("already-correct-bearer", func(t *testing.T) {
		stdout := claudeGetFixture(bearerHeaders, claudeStatusConnected)
		obs, ok := dr.Observe(stdout, opts)
		if !ok {
			t.Fatal("Observe: ok = false, want true")
		}
		if obs.URL != "https://engram.example.com/mcp" {
			t.Errorf("URL = %q, want %q", obs.URL, "https://engram.example.com/mcp")
		}
		if obs.Auth != AuthBearer {
			t.Errorf("Auth = %q, want %q", obs.Auth, AuthBearer)
		}
		want := []ObservedHeader{{Name: "x-gateway-api-key", State: HeaderMatches, Planned: "${GATEWAY_KEY}"}}
		if !reflect.DeepEqual(obs.Headers, want) {
			t.Errorf("Headers = %+v, want %+v", obs.Headers, want)
		}
		if len(obs.Unrecognized) != 0 {
			t.Errorf("Unrecognized = %q, want none", obs.Unrecognized)
		}
		if obs.BearerForm != claudeCodeBearerForm {
			t.Errorf("BearerForm = %q, want %q", obs.BearerForm, claudeCodeBearerForm)
		}
		if obs.WholeEntryNote != claudeCodeWholeEntryNote {
			t.Errorf("WholeEntryNote = %q, want %q", obs.WholeEntryNote, claudeCodeWholeEntryNote)
		}
		if obs.ManualRemediation != claudeCodeManualRemediation {
			t.Errorf("ManualRemediation = %q, want %q", obs.ManualRemediation, claudeCodeManualRemediation)
		}
		if obs.RewriteConsequence != "" {
			t.Errorf("RewriteConsequence = %q, want empty (Pitfall 4: a bearer-shaped registration has no OAuth session to lose)", obs.RewriteConsequence)
		}
		alreadyCorrect = obs
	})

	t.Run("status-failed-dial-is-chrome", func(t *testing.T) {
		stdout := claudeGetFixture(bearerHeaders, claudeStatusFailedDial)
		obs, ok := dr.Observe(stdout, opts)
		if !ok {
			t.Fatal("Observe: ok = false, want true")
		}
		if !reflect.DeepEqual(obs, alreadyCorrect) {
			t.Errorf("Observation with the RECORD's failed-dial status = %+v, want identical to the connected-status Observation %+v (Pitfall 4: Status is chrome)", obs, alreadyCorrect)
		}
	})

	t.Run("oauth-shape", func(t *testing.T) {
		stdout := claudeGetFixture(nil, claudeStatusConnected)
		obs, ok := dr.Observe(stdout, Options{URL: "https://engram.example.com/mcp", Auth: "oauth"})
		if !ok {
			t.Fatal("Observe: ok = false, want true")
		}
		if obs.Auth != AuthNone {
			t.Errorf("Auth = %q, want %q", obs.Auth, AuthNone)
		}
		if len(obs.Headers) != 0 {
			t.Errorf("Headers = %+v, want none", obs.Headers)
		}
		if obs.RewriteConsequence != claudeCodeOAuthReLoginNote {
			t.Errorf("RewriteConsequence = %q, want %q", obs.RewriteConsequence, claudeCodeOAuthReLoginNote)
		}
	})

	t.Run("header-value-ref-differs", func(t *testing.T) {
		stdout := claudeGetFixture([]string{
			"Authorization: " + claudeCodeBearerForm,
			"x-gateway-api-key: ${OLD_KEY}",
		}, claudeStatusConnected)
		obs, ok := dr.Observe(stdout, opts)
		if !ok {
			t.Fatal("Observe: ok = false, want true")
		}
		want := []ObservedHeader{{Name: "x-gateway-api-key", State: HeaderDiffers, Planned: "${GATEWAY_KEY}"}}
		if !reflect.DeepEqual(obs.Headers, want) {
			t.Errorf("Headers = %+v, want %+v", obs.Headers, want)
		}
	})

	t.Run("header-missing", func(t *testing.T) {
		stdout := claudeGetFixture([]string{"Authorization: " + claudeCodeBearerForm}, claudeStatusConnected)
		obs, ok := dr.Observe(stdout, opts)
		if !ok {
			t.Fatal("Observe: ok = false, want true")
		}
		want := []ObservedHeader{{Name: "x-gateway-api-key", State: HeaderMissing, Planned: "${GATEWAY_KEY}"}}
		if !reflect.DeepEqual(obs.Headers, want) {
			t.Errorf("Headers = %+v, want %+v", obs.Headers, want)
		}
	})

	t.Run("unplanned-header-literal-observed", func(t *testing.T) {
		stdout := claudeGetFixture([]string{
			"Authorization: " + claudeCodeBearerForm,
			claudeLiteralHeaderLine,
		}, claudeStatusFailedDial)
		obs, ok := dr.Observe(stdout, noExtraOpts)
		if !ok {
			t.Fatal("Observe: ok = false, want true")
		}
		want := []ObservedHeader{{Name: "x-litellm-api-key", State: HeaderUnplanned}}
		if !reflect.DeepEqual(obs.Headers, want) {
			t.Errorf("Headers = %+v, want %+v", obs.Headers, want)
		}
		b, err := json.Marshal(obs)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if strings.Contains(string(b), "sk-DO-NOT-COMMIT-literal-test-abc123") {
			t.Errorf("Observation carries the observed literal: %s", b)
		}
	})

	t.Run("unplanned-header-reference-observed", func(t *testing.T) {
		stdout := claudeGetFixture([]string{
			"Authorization: " + claudeCodeBearerForm,
			claudeReferenceHeaderLine,
		}, claudeStatusFailedDial)
		obs, ok := dr.Observe(stdout, noExtraOpts)
		if !ok {
			t.Fatal("Observe: ok = false, want true")
		}
		literalStdout := claudeGetFixture([]string{
			"Authorization: " + claudeCodeBearerForm,
			claudeLiteralHeaderLine,
		}, claudeStatusFailedDial)
		literalObs, ok := dr.Observe(literalStdout, noExtraOpts)
		if !ok {
			t.Fatal("Observe: ok = false, want true")
		}
		if !reflect.DeepEqual(obs, literalObs) {
			t.Errorf("reference-shaped Observation %+v != literal-shaped Observation %+v (D-02: no shape branching)", obs, literalObs)
		}
	})

	t.Run("foreign-authorization", func(t *testing.T) {
		stdout := claudeGetFixture([]string{"Authorization: Bearer sk-DO-NOT-COMMIT-literal-test-abc123"}, claudeStatusConnected)
		obs, ok := dr.Observe(stdout, Options{URL: "https://engram.example.com/mcp", Auth: "oauth"})
		if !ok {
			t.Fatal("Observe: ok = false, want true")
		}
		if obs.Auth != AuthForeign {
			t.Errorf("Auth = %q, want %q", obs.Auth, AuthForeign)
		}
		b, err := json.Marshal(obs)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if strings.Contains(string(b), "sk-DO-NOT-COMMIT-literal-test-abc123") {
			t.Errorf("Observation carries the sentinel value: %s", b)
		}
	})

	t.Run("duplicate-header-names", func(t *testing.T) {
		stdout := claudeGetFixture([]string{
			"Authorization: " + claudeCodeBearerForm,
			"x-gateway-api-key: ${GATEWAY_KEY}",
			"x-gateway-api-key: ${OTHER}",
		}, claudeStatusConnected)
		obs, ok := dr.Observe(stdout, opts)
		if !ok {
			t.Fatal("Observe: ok = false, want true")
		}
		want := []ObservedHeader{
			{Name: "x-gateway-api-key", State: HeaderMatches, Planned: "${GATEWAY_KEY}"},
			{Name: "x-gateway-api-key", State: HeaderUnplanned},
		}
		if !reflect.DeepEqual(obs.Headers, want) {
			t.Errorf("Headers = %+v, want %+v", obs.Headers, want)
		}
	})

	t.Run("case-insensitive-name", func(t *testing.T) {
		stdout := claudeGetFixture([]string{
			"Authorization: " + claudeCodeBearerForm,
			"X-Gateway-Api-Key: ${GATEWAY_KEY}",
		}, claudeStatusConnected)
		obs, ok := dr.Observe(stdout, opts)
		if !ok {
			t.Fatal("Observe: ok = false, want true")
		}
		want := []ObservedHeader{{Name: "x-gateway-api-key", State: HeaderMatches, Planned: "${GATEWAY_KEY}"}}
		if !reflect.DeepEqual(obs.Headers, want) {
			t.Errorf("Headers = %+v, want %+v", obs.Headers, want)
		}
	})

	t.Run("type-not-http", func(t *testing.T) {
		stdout := strings.Replace(claudeGetFixture(bearerHeaders, claudeStatusConnected), "Type: http", "Type: sse", 1)
		obs, ok := dr.Observe(stdout, opts)
		if !ok {
			t.Fatal("Observe: ok = false, want true")
		}
		if !reflect.DeepEqual(obs.Unrecognized, []string{"Type"}) {
			t.Errorf("Unrecognized = %q, want %q", obs.Unrecognized, []string{"Type"})
		}
	})

	t.Run("unrecognized-label", func(t *testing.T) {
		stdout := strings.Replace(claudeGetFixture(bearerHeaders, claudeStatusConnected), "  Type: http\n", "  Type: http\n  Proxy: http://p\n", 1)
		obs, ok := dr.Observe(stdout, opts)
		if !ok {
			t.Fatal("Observe: ok = false, want true")
		}
		if !reflect.DeepEqual(obs.Unrecognized, []string{"Proxy"}) {
			t.Errorf("Unrecognized = %q, want %q", obs.Unrecognized, []string{"Proxy"})
		}
	})

	t.Run("unrecognized-unlabeled-line", func(t *testing.T) {
		stdout := strings.Replace(claudeGetFixture(bearerHeaders, claudeStatusConnected), "  Type: http\n", "  Type: http\n  something odd\n", 1)
		obs, ok := dr.Observe(stdout, opts)
		if !ok {
			t.Fatal("Observe: ok = false, want true")
		}
		if !reflect.DeepEqual(obs.Unrecognized, []string{"line"}) {
			t.Errorf("Unrecognized = %q, want %q", obs.Unrecognized, []string{"line"})
		}
	})

	t.Run("not-found-after-remove", func(t *testing.T) {
		_, ok := dr.Observe(claudeNotFoundAfterRemove, opts)
		if ok {
			t.Error("Observe: ok = true, want false (no URL: line, D-09)")
		}
	})

	t.Run("empty", func(t *testing.T) {
		_, ok := dr.Observe("", opts)
		if ok {
			t.Error("Observe: ok = true, want false")
		}
	})

	t.Run("no-url-line", func(t *testing.T) {
		stdout := strings.Replace(claudeGetFixture(bearerHeaders, claudeStatusConnected), "  URL: https://engram.example.com/mcp\n", "", 1)
		_, ok := dr.Observe(stdout, opts)
		if ok {
			t.Error("Observe: ok = true, want false")
		}
	})
}

// TestOAuthReLoginConsequence is REQ-apply-rewrite-consequence's proof
// (D-03, D-04): a claude-code registration observed with NO Authorization
// header, that classifies would-write, carries claudeCodeOAuthReLoginNote
// on Notes — in BOTH preview and apply, ahead of any tolerant-remove
// record — while a bearer-shaped, foreign-shaped, already-correct,
// preserved, ambiguous, or codex fixture never does (Pitfall 4). The
// trigger is the observed SHAPE alone (obs.Auth == AuthNone), never
// opts.Auth and never "claude-code + would-write" alone.
func TestOAuthReLoginConsequence(t *testing.T) {
	opts := Options{URL: "https://engram.example.com/mcp", Auth: "oauth"}

	// wouldWriteURL mutates a fixture's URL so it no longer matches opts
	// — a reproducible would-write facet (FacetURL) that says nothing by
	// itself about Auth.
	wouldWriteURL := func(fixture string) string {
		return strings.Replace(fixture, "URL: https://engram.example.com/mcp", "URL: https://old.example/mcp", 1)
	}

	t.Run("preview-oauth-shape-would-write-carries-note", func(t *testing.T) {
		stdout := wouldWriteURL(claudeGetFixture(nil, claudeStatusFailedDial))
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: stdout, ExitCode: 0}},
		), "claude")

		res := Preview(context.Background(), env, ClaudeCode, opts)
		if res.Outcome != OutcomeWouldWrite {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeWouldWrite)
		}
		if res.Facets != "url" {
			t.Errorf("Facets = %q, want %q", res.Facets, "url")
		}
		if res.Notes != claudeCodeOAuthReLoginNote {
			t.Errorf("Notes = %q, want %q", res.Notes, claudeCodeOAuthReLoginNote)
		}
		if len(calls) != 1 {
			t.Fatalf("Run called %d times, want exactly 1: %+v", len(calls), calls)
		}
	})

	t.Run("preview-oauth-shape-no-headers-line", func(t *testing.T) {
		fixture := strings.Replace(claudeGetFixture(nil, claudeStatusFailedDial), "  Headers:\n", "", 1)
		stdout := wouldWriteURL(fixture)
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: stdout, ExitCode: 0}},
		), "claude")

		res := Preview(context.Background(), env, ClaudeCode, opts)
		if res.Outcome != OutcomeWouldWrite {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeWouldWrite)
		}
		if res.Facets != "url" {
			t.Errorf("Facets = %q, want %q", res.Facets, "url")
		}
		if res.Notes != claudeCodeOAuthReLoginNote {
			t.Errorf("Notes = %q, want %q", res.Notes, claudeCodeOAuthReLoginNote)
		}
		if len(calls) != 1 {
			t.Fatalf("Run called %d times, want exactly 1: %+v", len(calls), calls)
		}
	})

	t.Run("apply-oauth-shape-would-write-carries-note-before-remove", func(t *testing.T) {
		probe1 := wouldWriteURL(claudeGetFixture(nil, claudeStatusFailedDial))
		probe2 := claudeGetFixture(nil, claudeStatusConnected)
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: probe1, ExitCode: 0}}, // probe #1: would-write on url
			scriptedResult{Result: RunResult{ExitCode: 0}},                 // tolerant remove
			scriptedResult{Result: RunResult{ExitCode: 0}},                 // fatal add
			scriptedResult{Result: RunResult{Stdout: probe2, ExitCode: 0}}, // probe #2
		), "claude")

		res := Apply(context.Background(), env, ClaudeCode, opts)
		if res.Outcome != OutcomeWrote {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeWrote)
		}
		wantNotes := claudeCodeOAuthReLoginNote + "; " + claudeCodeRemoveAction.Description
		if res.Notes != wantNotes {
			t.Errorf("Notes = %q, want %q", res.Notes, wantNotes)
		}
		if len(calls) != 4 {
			t.Fatalf("Run called %d times, want exactly 4: %+v", len(calls), calls)
		}
		if calls[1].Args[0] != "mcp" || calls[1].Args[1] != "remove" {
			t.Errorf("calls[1].Args = %q, want a %q call", calls[1].Args, "mcp remove")
		}
	})

	t.Run("apply-oauth-shape-remove-tolerated-note-order", func(t *testing.T) {
		probe1 := wouldWriteURL(claudeGetFixture(nil, claudeStatusFailedDial))
		probe2 := claudeGetFixture(nil, claudeStatusConnected)
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: probe1, ExitCode: 0}},
			scriptedResult{Result: RunResult{ExitCode: 1, Stderr: "No MCP server named 'engram' in user scope"}}, // remove: tolerated failure
			scriptedResult{Result: RunResult{ExitCode: 0}},
			scriptedResult{Result: RunResult{Stdout: probe2, ExitCode: 0}},
		), "claude")

		res := Apply(context.Background(), env, ClaudeCode, opts)
		if res.Outcome != OutcomeWrote {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeWrote)
		}
		wantPrefix := claudeCodeOAuthReLoginNote + "; "
		if !strings.HasPrefix(res.Notes, wantPrefix) {
			t.Errorf("Notes = %q, want prefix %q", res.Notes, wantPrefix)
		}
		if !strings.Contains(res.Notes, "exited 1") {
			t.Errorf("Notes = %q, want it to contain %q", res.Notes, "exited 1")
		}
	})

	t.Run("oauth-client-opts-same-shape-rule", func(t *testing.T) {
		clientOpts := Options{URL: "https://engram.example.com/mcp", Auth: "oauth-client", ClientID: "example-client-id"}
		stdout := wouldWriteURL(claudeGetFixture(nil, claudeStatusFailedDial))
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: stdout, ExitCode: 0}},
		), "claude")

		res := Preview(context.Background(), env, ClaudeCode, clientOpts)
		if res.Notes != claudeCodeOAuthReLoginNote {
			t.Errorf("Notes = %q, want %q (the rule keys on the observed shape, not the requested mode)", res.Notes, claudeCodeOAuthReLoginNote)
		}
	})

	t.Run("bearer-shape-no-note", func(t *testing.T) {
		stdout := claudeGetFixture([]string{"Authorization: " + claudeCodeBearerForm}, claudeStatusConnected)
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: stdout, ExitCode: 0}},
		), "claude")

		res := Preview(context.Background(), env, ClaudeCode, opts)
		if res.Outcome != OutcomeWouldWrite {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeWouldWrite)
		}
		if res.Facets != "auth-mode" {
			t.Errorf("Facets = %q, want %q", res.Facets, "auth-mode")
		}
		if res.Notes != "" {
			t.Errorf("Notes = %q, want empty (Pitfall 4: a bearer-shaped registration has no OAuth session to lose)", res.Notes)
		}
	})

	t.Run("foreign-authorization-no-note", func(t *testing.T) {
		const sentinel = "SENTINEL-FOREIGN-DO-NOT-LEAK"
		stdout := claudeGetFixture([]string{"Authorization: Basic " + sentinel}, claudeStatusConnected)
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: stdout, ExitCode: 0}},
		), "claude")

		res := Preview(context.Background(), env, ClaudeCode, opts)
		if res.Outcome != OutcomePreserved {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomePreserved)
		}
		if res.Notes != "" {
			t.Errorf("Notes = %q, want empty", res.Notes)
		}
		if !strings.HasSuffix(res.Reason, claudeCodeManualRemediation) {
			t.Errorf("Reason = %q, want it to end with %q", res.Reason, claudeCodeManualRemediation)
		}
		b, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if strings.Contains(string(b), sentinel) {
			t.Errorf("json.Marshal(res) = %s, must not contain the sentinel", b)
		}
	})

	t.Run("already-correct-no-note", func(t *testing.T) {
		stdout := claudeGetFixture(nil, claudeStatusConnected)

		var previewCalls []runCall
		previewEnv := fakeEnvWithRun(scriptedRun(&previewCalls,
			scriptedResult{Result: RunResult{Stdout: stdout, ExitCode: 0}},
		), "claude")
		previewRes := Preview(context.Background(), previewEnv, ClaudeCode, opts)
		if previewRes.Outcome != OutcomeAlreadyCorrect {
			t.Fatalf("Preview Outcome = %q, want %q", previewRes.Outcome, OutcomeAlreadyCorrect)
		}
		if previewRes.Notes != "" {
			t.Errorf("Preview Notes = %q, want empty", previewRes.Notes)
		}

		var applyCalls []runCall
		applyEnv := fakeEnvWithRun(scriptedRun(&applyCalls,
			scriptedResult{Result: RunResult{Stdout: stdout, ExitCode: 0}},
		), "claude")
		applyRes := Apply(context.Background(), applyEnv, ClaudeCode, opts)
		if applyRes.Outcome != OutcomeAlreadyCorrect {
			t.Fatalf("Apply Outcome = %q, want %q", applyRes.Outcome, OutcomeAlreadyCorrect)
		}
		if applyRes.Notes != "" {
			t.Errorf("Apply Notes = %q, want empty", applyRes.Notes)
		}
	})

	t.Run("preserved-oauth-shape-no-note", func(t *testing.T) {
		stdout := claudeGetFixture([]string{"x-litellm-api-key: sk-x"}, claudeStatusFailedDial)
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: stdout, ExitCode: 0}},
		), "claude")

		res := Preview(context.Background(), env, ClaudeCode, opts)
		if res.Outcome != OutcomePreserved {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomePreserved)
		}
		if res.Notes != "" {
			t.Errorf("Notes = %q, want empty (no write runs, so no consequence)", res.Notes)
		}
	})

	t.Run("ambiguous-read-no-note", func(t *testing.T) {
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stderr: "No MCP server named 'engram' in user scope", ExitCode: 1}},
		), "claude")

		res := Preview(context.Background(), env, ClaudeCode, opts)
		if res.Outcome != OutcomeWouldWrite {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeWouldWrite)
		}
		if res.Notes != "" {
			t.Errorf("Notes = %q, want empty", res.Notes)
		}
		if !strings.HasPrefix(res.Drift, "claude-code: not compared:") {
			t.Errorf("Drift = %q, want prefix %q", res.Drift, "claude-code: not compared:")
		}
	})

	t.Run("codex-never-carries-note", func(t *testing.T) {
		noBearer := strings.Replace(codexGetEngramBearer,
			`"bearer_token_env_var":"ENGRAM_TOKEN"`, `"bearer_token_env_var":null`, 1)
		stdout := strings.Replace(noBearer, `"url":"https://engram.example.com/mcp"`, `"url":"https://old.example/mcp"`, 1)

		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: stdout, ExitCode: 0}},
		), "codex")

		res := Preview(context.Background(), env, Codex, opts)
		if res.Outcome != OutcomeWouldWrite {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeWouldWrite)
		}
		if res.Notes != "" {
			t.Errorf("Notes = %q, want empty (codex never authors a rewrite consequence)", res.Notes)
		}

		dr, ok := Codex.(DriftRuntime)
		if !ok {
			t.Fatal("Codex does not implement DriftRuntime")
		}
		obs, ok := dr.Observe(stdout, opts)
		if !ok {
			t.Fatal("Observe: ok = false, want true")
		}
		if obs.RewriteConsequence != "" {
			t.Errorf("RewriteConsequence = %q, want empty", obs.RewriteConsequence)
		}
	})
}

func TestClaudeCodeClientID(t *testing.T) {
	const url = "https://engram.example.com/mcp"
	for _, id := range []string{"test-client", "  client 'quoted'; $(echo nope) &  "} {
		t.Run(id, func(t *testing.T) {
			plan, err := ClaudeCode.Plan(fakeEnv(), Options{URL: url, Auth: "oauth-client", ClientID: id})
			if err != nil {
				t.Fatal(err)
			}
			want := []Action{
				claudeCodeRemoveAction,
				{
					Args: []string{"claude", "mcp", "add", "--transport", "http", "engram", url,
						"--scope", "user", "--client-id", id, "--client-secret", "--callback-port", "8765"},
					Description: "register engram as a user-scope MCP server (pre-registered OAuth client)",
				},
			}
			if !reflect.DeepEqual(plan.Actions, want) {
				t.Errorf("Actions = %#v, want %#v", plan.Actions, want)
			}
			if !reflect.DeepEqual(plan.Probe, []string{"claude", "mcp", "get", "engram"}) {
				t.Errorf("Probe = %q", plan.Probe)
			}
			if want := (SkillTarget{Format: SkillFormatNative, Dir: filepath.Join("/home/fake", ".claude", "skills")}); plan.Skills != want {
				t.Errorf("Skills = %+v, want %+v", plan.Skills, want)
			}
		})
	}
}
