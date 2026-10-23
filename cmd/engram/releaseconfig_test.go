// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// stripComment returns the portion of a line before the first '#', which
// starts a YAML comment in the files these tests read.
func stripComment(line string) string {
	before, _, _ := strings.Cut(line, "#")
	return before
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

// quotedHomebrewToken matches how .goreleaser.yaml writes a Homebrew
// install-time token, {{ "{{NAME}}" }}, so GoReleaser's own templating
// leaves it intact.
var quotedHomebrewToken = regexp.MustCompile(`{{ "({{[A-Za-z_.]+}})" }}`)

var quotedString = regexp.MustCompile(`"([^"]*)"`)

// stepLines splits a steps body into lines with each quoted Homebrew token
// rendered the way GoReleaser renders it, so string literals parse plainly.
func stepLines(body string) []string {
	return strings.Split(quotedHomebrewToken.ReplaceAllString(body, "$1"), "\n")
}

// lineIndexes returns the indexes of the lines containing substr.
func lineIndexes(lines []string, substr string) []int {
	var idx []int
	for i, l := range lines {
		if strings.Contains(l, substr) {
			idx = append(idx, i)
		}
	}
	return idx
}

// TestReleaseConfigCaskInstallGate pins the cask's install and uninstall
// steps in .goreleaser.yaml (REQ-cask-install-gate): the version gate runs
// before the completions, which run before the man pages; no step may
// fail softly or strip quarantine; the uninstall removes exactly the files
// the install writes and never a directory; and the soft-failing Homebrew
// mechanisms stay out.
func TestReleaseConfigCaskInstallGate(t *testing.T) {
	const file = "../../.goreleaser.yaml"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("reading %s: %v", file, err)
	}
	var cfg struct {
		Brews         any              `yaml:"brews"`
		HomebrewCasks []map[string]any `yaml:"homebrew_casks"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parsing %s: %v", file, err)
	}
	if cfg.Brews != nil {
		t.Errorf("%s: top-level `brews` is set; engram ships a cask only", file)
	}
	if len(cfg.HomebrewCasks) != 1 {
		t.Fatalf("%s: want exactly 1 homebrew_casks entry, got %d", file, len(cfg.HomebrewCasks))
	}
	cask := cfg.HomebrewCasks[0]

	for _, key := range []string{"generate_completions_from_executable", "completions", "manpages"} {
		if _, ok := cask[key]; ok {
			t.Errorf("%s: homebrew_casks[0].%s is set; it turns a failing binary into a warning", file, key)
		}
	}
	if desc, _ := cask["description"].(string); desc != rootCmd.Short {
		t.Errorf("%s: cask description %q does not byte-match rootCmd.Short %q", file, desc, rootCmd.Short)
	}

	hooks, _ := cask["hooks"].(map[string]any)
	post, _ := hooks["post"].(map[string]any)
	for _, legacy := range []string{"install", "uninstall"} {
		if _, ok := post[legacy]; ok {
			t.Errorf("%s: hooks.post.%s is set; it renders a Ruby flight block, use %s_steps", file, legacy, legacy)
		}
	}
	installBody, _ := post["install_steps"].(string)
	uninstallBody, _ := post["uninstall_steps"].(string)
	if installBody == "" || uninstallBody == "" {
		t.Fatalf("%s: hooks.post.install_steps and uninstall_steps must both be set", file)
	}

	install := stepLines(installBody)
	for _, banned := range []string{"must_succeed: false", "xattr"} {
		if idx := lineIndexes(install, banned); len(idx) != 0 {
			t.Errorf("install_steps line %d contains %q", idx[0]+1, banned)
		}
	}
	// Homebrew's {{version}} token tracks the cask's version stanza at
	// install time; a GoReleaser-rendered literal would not.
	expect := lineIndexes(install, `"--expect", "{{version}}"`)
	completion := lineIndexes(install, `["completion", `)
	man := lineIndexes(install, `["man", `)
	if len(expect) != 1 || len(completion) != 3 || len(man) != 1 {
		t.Fatalf("install_steps: want 1 --expect, 3 completion and 1 man step, got %d, %d and %d", len(expect), len(completion), len(man))
	}
	if expect[0] > completion[0] {
		t.Errorf("install_steps: the --expect step (line %d) must come before the completion steps (line %d)", expect[0]+1, completion[0]+1)
	}
	if completion[2] > man[0] {
		t.Errorf("install_steps: the completion steps (last at line %d) must come before the man step (line %d)", completion[2]+1, man[0]+1)
	}

	var written []string
	for _, l := range install {
		if _, after, ok := strings.Cut(l, "stdout_path: "); ok {
			written = append(written, strings.Trim(strings.TrimSpace(after), `",`))
		}
	}
	manArgs := quotedString.FindAllStringSubmatch(install[man[0]], -1)
	if len(manArgs) != 2 {
		t.Fatalf("install_steps man step %q: want [\"man\", <dir>]", install[man[0]])
	}
	manDir := manArgs[1][1]
	written = append(written, manDir+"/engram.1", manDir+"/engram-*.1")

	uninstall := stepLines(uninstallBody)
	if idx := lineIndexes(uninstall, "recursive"); len(idx) != 0 {
		t.Errorf("uninstall_steps line %d sets recursive; the uninstall removes files only", idx[0]+1)
	}
	quoted := quotedString.FindAllStringSubmatch(strings.Join(uninstall, "\n"), -1)
	removed := make([]string, 0, len(quoted))
	for _, m := range quoted {
		if strings.HasSuffix(m[1], "/") {
			t.Errorf("uninstall_steps removes %q, a directory", m[1])
		}
		removed = append(removed, m[1])
	}
	slices.Sort(written)
	slices.Sort(removed)
	if !slices.Equal(written, removed) {
		t.Errorf("uninstall_steps removes %q, want exactly what install_steps writes: %q", removed, written)
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
