// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// stripComment returns the portion of a line before the first '#' that
// starts a comment. This is used both for YAML comments and for the Ruby
// block scalar embedded in .goreleaser.yaml's cask post-install hook, where
// '#' also starts a Ruby comment — EXCEPT that hook's paths are built with
// Ruby string interpolation ("#{HOMEBREW_PREFIX}/..."), where '#{' is not a
// comment marker. A bare "strip from first #" would truncate every
// interpolated line to nothing and blind every assertion that greps for a
// completion path, so '#' immediately followed by '{' is skipped over.
func stripComment(line string) string {
	for i := 0; i < len(line); i++ {
		if line[i] == '#' {
			if i+1 < len(line) && line[i+1] == '{' {
				continue
			}
			return line[:i]
		}
	}
	return line
}

// nonCommentLines reads a repo-root file and returns its lines with any
// trailing comment stripped, alongside the original 1-based line numbers.
func nonCommentLines(t *testing.T, relPath string) (lines []string, lineNos []int) {
	t.Helper()
	data, err := os.ReadFile(relPath)
	if err != nil {
		t.Fatalf("reading %s: %v", relPath, err)
	}
	raw := strings.Split(string(data), "\n")
	lines = make([]string, len(raw))
	lineNos = make([]int, len(raw))
	for i, l := range raw {
		lines[i] = stripComment(l)
		lineNos[i] = i + 1
	}
	return lines, lineNos
}

// firstMatch returns the line number (1-based) of the first non-comment line
// containing substr, or -1 if none match.
func firstMatch(lines []string, lineNos []int, substr string) int {
	for i, l := range lines {
		if strings.Contains(l, substr) {
			return lineNos[i]
		}
	}
	return -1
}

// countMatches returns the number of non-comment lines containing substr.
func countMatches(lines []string, substr string) int {
	n := 0
	for _, l := range lines {
		if strings.Contains(l, substr) {
			n++
		}
	}
	return n
}

// checkOrdering is the reusable ordering assertion used by
// TestReleaseConfigCaskInstallGate. It fails with the file name, the
// expected ordering, and the line numbers actually found for `before`/
// `after`, so both real-file and fixture-string callers get an actionable
// message.
func checkOrdering(t testing.TB, file string, lines []string, lineNos []int, before, after string) {
	t.Helper()
	beforeLine := firstMatch(lines, lineNos, before)
	afterLine := firstMatch(lines, lineNos, after)
	if beforeLine == -1 {
		t.Errorf("%s: expected a non-comment line containing %q, found none", file, before)
		return
	}
	if afterLine == -1 {
		t.Errorf("%s: expected a non-comment line containing %q, found none", file, after)
		return
	}
	if beforeLine >= afterLine {
		t.Errorf("%s: expected line containing %q (found at line %d) strictly BEFORE line containing %q (found at line %d)",
			file, before, beforeLine, after, afterLine)
	}
}

// TestCheckOrderingCatchesTransposedLines proves checkOrdering actually goes
// RED on a wrong ordering, using an inline fixture with the two markers
// transposed relative to what the cask hook requires.
func TestCheckOrderingCatchesTransposedLines(t *testing.T) {
	fixture := "version --output json gate\nxattr strip happens second, wrongly\n"
	raw := strings.Split(fixture, "\n")
	lines := make([]string, len(raw))
	lineNos := make([]int, len(raw))
	for i, l := range raw {
		lines[i] = stripComment(l)
		lineNos[i] = i + 1
	}
	rt := &recordingT{T: t}
	checkOrdering(rt, "fixture", lines, lineNos, "xattr strip", "version --output json")
	if !rt.failed {
		t.Fatal("checkOrdering did not fail on a fixture with transposed ordering — the checker is not sensitive to order")
	}
}

// recordingT wraps *testing.T to capture Errorf calls without failing the
// outer test, so we can assert the *checker* goes red without failing this
// meta-test itself.
type recordingT struct {
	*testing.T
	failed bool
}

func (r *recordingT) Errorf(_ string, _ ...any) {
	r.failed = true
}

// TestReleaseConfigCaskInstallGate pins the cask post-install hook's
// ordering and shape in .goreleaser.yaml (REQ-cask-install-gate): the
// Gatekeeper quarantine strip, guarded to macOS, must run before the
// binary-version gate, which must run before completions are generated —
// and the declarative/deprecated GoReleaser mechanisms this design
// deliberately avoids must not reappear.
func TestReleaseConfigCaskInstallGate(t *testing.T) {
	const file = "../../.goreleaser.yaml"
	lines, lineNos := nonCommentLines(t, file)

	xattrCount := countMatches(lines, `system_command "/usr/bin/xattr"`)
	if xattrCount != 1 {
		t.Errorf("%s: expected exactly 1 non-comment occurrence of `system_command \"/usr/bin/xattr\"`, found %d", file, xattrCount)
	}

	checkOrdering(t, file, lines, lineNos, "if OS.mac?", `system_command "/usr/bin/xattr"`)
	checkOrdering(t, file, lines, lineNos, `system_command "/usr/bin/xattr"`, `"version", "--output", "json"`)
	checkOrdering(t, file, lines, lineNos, `"version", "--output", "json"`, `args: ["completion"`)

	forbidden := []string{"generate_completions_from_executable", "brews:", "rm_rf"}
	for _, f := range forbidden {
		if n := countMatches(lines, f); n != 0 {
			t.Errorf("%s: expected 0 non-comment occurrences of %q, found %d", file, f, n)
		}
	}

	completionPaths := []string{
		"etc/bash_completion.d/engram",
		"share/zsh/site-functions/_engram",
		"share/fish/vendor_completions.d/engram.fish",
	}
	for _, p := range completionPaths {
		if n := countMatches(lines, p); n != 2 {
			t.Errorf("%s: expected exactly 2 non-comment occurrences of %q (install + uninstall), found %d", file, p, n)
		}
	}

	// The cask description must be byte-identical to rootCmd.Short.
	descLine := ""
	descLineNo := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "description:") {
			descLine = l
			descLineNo = lineNos[i]
			break
		}
	}
	if descLineNo == -1 {
		t.Fatalf("%s: no non-comment `description:` line found", file)
	}
	trimmed := strings.TrimSpace(descLine)
	trimmed = strings.TrimPrefix(trimmed, "description:")
	trimmed = strings.TrimSpace(trimmed)
	trimmed = strings.Trim(trimmed, `"`)
	if trimmed != rootCmd.Short {
		t.Errorf("%s:%d: cask description %q does not byte-match rootCmd.Short %q", file, descLineNo, trimmed, rootCmd.Short)
	}
}

// TestReleaseConfigCaskReshipRecovery pins the newest-tag upload guard
// (REQ-cask-reship-recovery): release.yaml must compute
// SKIP_HOMEBREW_UPLOAD before invoking goreleaser, and .goreleaser.yaml's
// skip_upload must read that guarded env var rather than using `auto`.
func TestReleaseConfigCaskReshipRecovery(t *testing.T) {
	const releaseFile = "../../.github/workflows/release.yaml"
	lines, lineNos := nonCommentLines(t, releaseFile)

	guardLine := firstMatch(lines, lineNos, "name: Resolve Homebrew upload guard")
	if guardLine == -1 {
		t.Fatalf("%s: expected a non-comment `name: Resolve Homebrew upload guard` step", releaseFile)
	}
	checkoutLine := firstMatch(lines, lineNos, "uses: actions/checkout")
	goreleaserLine := firstMatch(lines, lineNos, "uses: goreleaser/goreleaser-action")
	if checkoutLine == -1 {
		t.Fatalf("%s: expected a non-comment `uses: actions/checkout` step", releaseFile)
	}
	if goreleaserLine == -1 {
		t.Fatalf("%s: expected a non-comment `uses: goreleaser/goreleaser-action` step", releaseFile)
	}
	if checkoutLine >= guardLine {
		t.Errorf("%s: expected checkout (line %d) strictly before the upload-guard step (line %d)", releaseFile, checkoutLine, guardLine)
	}
	if guardLine >= goreleaserLine {
		t.Errorf("%s: expected the upload-guard step (line %d) strictly before goreleaser-action (line %d)", releaseFile, guardLine, goreleaserLine)
	}

	for _, want := range []string{"SKIP_HOMEBREW_UPLOAD=false", "SKIP_HOMEBREW_UPLOAD=true"} {
		if n := countMatches(lines, want); n != 1 {
			t.Errorf("%s: expected exactly 1 non-comment occurrence of %q, found %d", releaseFile, want, n)
		}
	}

	const goreleaserFile = "../../.goreleaser.yaml"
	glLines, _ := nonCommentLines(t, goreleaserFile)
	skipUploadLine := ""
	for _, l := range glLines {
		if strings.Contains(l, "skip_upload:") {
			skipUploadLine = l
			break
		}
	}
	if skipUploadLine == "" {
		t.Fatalf("%s: expected a non-comment `skip_upload:` line", goreleaserFile)
	}
	if !strings.Contains(skipUploadLine, `index .Env "SKIP_HOMEBREW_UPLOAD"`) {
		t.Errorf("%s: skip_upload value %q does not contain the guarded optional-env form `index .Env \"SKIP_HOMEBREW_UPLOAD\"`", goreleaserFile, skipUploadLine)
	}
	if strings.Contains(skipUploadLine, "auto") {
		t.Errorf("%s: skip_upload value %q must not be `auto`", goreleaserFile, skipUploadLine)
	}
}

// bareEnvTokenPattern is GoReleaser's own regex (internal/tmpl
// ApplySingleEnvOnly) for the bare `{{ .Env.NAME }}` token form. A
// conditional such as `{{ if .Env.X }}…{{ end }}` never matches it, which is
// exactly the failure this test guards against re-introducing.
var bareEnvTokenPattern = regexp.MustCompile(`^{{\s*\.Env\.[^.\s}]+\s*}}$`)

// TestReleaseConfigCaskCredentialVerified pins the CURRENT
// tap-publisher-App credential shape (REQ-cask-credential-verified), post
// #516: a bare-env-token cask credential distinct from the release App, a
// dispatch-only verification workflow with no write verbs, and no
// contents:write permission on that workflow.
func TestReleaseConfigCaskCredentialVerified(t *testing.T) {
	const goreleaserFile = "../../.goreleaser.yaml"
	glLines, _ := nonCommentLines(t, goreleaserFile)
	tokenLine := ""
	for _, l := range glLines {
		if strings.Contains(l, "token:") && strings.Contains(l, ".Env.HOMEBREW_TAP_TOKEN") {
			tokenLine = l
			break
		}
	}
	if tokenLine == "" {
		t.Fatalf("%s: expected a non-comment `token:` line referencing HOMEBREW_TAP_TOKEN", goreleaserFile)
	}
	val := strings.TrimSpace(tokenLine)
	if i := strings.Index(val, "token:"); i != -1 {
		val = strings.TrimSpace(val[i+len("token:"):])
	}
	val = strings.Trim(val, `"`)
	literal := "{{ .Env.HOMEBREW_TAP_TOKEN }}"
	if val != literal {
		t.Errorf("%s: homebrew_casks[].repository.token = %q, want exact bare form %q", goreleaserFile, val, literal)
	}
	if !bareEnvTokenPattern.MatchString(val) {
		t.Errorf("%s: homebrew_casks[].repository.token = %q does not match GoReleaser's bare-env-token pattern %s", goreleaserFile, val, bareEnvTokenPattern.String())
	}

	const releaseFile = "../../.github/workflows/release.yaml"
	relLines, _ := nonCommentLines(t, releaseFile)
	if n := countMatches(relLines, "HOMEBREW_TAP_TOKEN: ${{ steps.tap-token.outputs.token }}"); n != 1 {
		t.Errorf("%s: expected exactly 1 non-comment occurrence of `HOMEBREW_TAP_TOKEN: ${{ steps.tap-token.outputs.token }}`, found %d", releaseFile, n)
	}
	if n := countMatches(relLines, "repositories: homebrew-tap"); n < 1 {
		t.Errorf("%s: expected at least 1 non-comment occurrence of `repositories: homebrew-tap` on the tap-token mint, found %d", releaseFile, n)
	}

	const verifyFile = "../../.github/workflows/verify-tap-credential.yaml"
	verLines, _ := nonCommentLines(t, verifyFile)
	full := strings.Join(verLines, "\n")

	if !strings.Contains(full, "workflow_dispatch") {
		t.Errorf("%s: expected `on: workflow_dispatch` trigger", verifyFile)
	}
	for _, forbiddenTrigger := range []string{"push:", "pull_request:", "schedule:"} {
		if n := countMatches(verLines, forbiddenTrigger); n != 0 {
			t.Errorf("%s: expected 0 non-comment occurrences of trigger %q, found %d", verifyFile, forbiddenTrigger, n)
		}
	}
	if n := countMatches(verLines, "repositories: homebrew-tap"); n != 1 {
		t.Errorf("%s: expected exactly 1 non-comment occurrence of `repositories: homebrew-tap`, found %d", verifyFile, n)
	}
	// `permission-contents: write` on the App-token mint is the intended,
	// narrowly-scoped grant this workflow relies on; a workflow-level (or
	// job-level) `contents: write` permission would be the unwanted,
	// broader grant this assertion guards against. Match only the
	// permission-block key, not the (expected) mint parameter.
	if n := countMatches(verLines, "  contents: write"); n != 0 {
		t.Errorf("%s: expected 0 non-comment occurrences of the `contents: write` permission key, found %d", verifyFile, n)
	}
	writeVerbs := []string{"--method PUT", "--method POST", "--method PATCH", "--method DELETE", "git push", "git commit"}
	for _, verb := range writeVerbs {
		if n := countMatches(verLines, verb); n != 0 {
			t.Errorf("%s: expected 0 non-comment occurrences of write verb %q, found %d", verifyFile, verb, n)
		}
	}
}
