// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// stripComment returns the portion of a line before the first '#'. That is
// a YAML comment everywhere except shell parameter expansions such as
// ${TAG_NAME#v} inside release.yaml's run blocks, which it also cuts; no
// assertion reads those lines.
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
// install-time token, {{ "{{NAME}}" }}, so GoReleaser's templating emits the
// bare Homebrew token.
var quotedHomebrewToken = regexp.MustCompile(`{{ "({{[A-Za-z_.]+}})" }}`)

var (
	quotedString       = regexp.MustCompile(`"([^"]*)"`)
	stdoutPathArg      = regexp.MustCompile(`stdout_path:\s*"([^"]*)"`)
	completionArg      = regexp.MustCompile(`\["completion", "([a-z]+)"\]`)
	homebrewPrefixBase = regexp.MustCompile(`\bbase:\s*:homebrew_prefix\b`)
)

// macosSigningVars are the variables .goreleaser.yaml's notarize: block reads
// and release.yaml fills from the release environment's secrets, in the order
// the enabled condition lists them.
var macosSigningVars = []string{
	"MACOS_SIGN_P12", "MACOS_SIGN_PASSWORD", "MACOS_NOTARY_ISSUER_ID", "MACOS_NOTARY_KEY_ID", "MACOS_NOTARY_KEY",
}

// Install paths as the rendered install_steps spell them.
const (
	homebrewPrefix = "{{HOMEBREW_PREFIX}}"
	caskManDir     = homebrewPrefix + "/share/man/man1"
)

var caskCompletionPaths = map[string]string{
	"bash": homebrewPrefix + "/etc/bash_completion.d/engram",
	"zsh":  homebrewPrefix + "/share/zsh/site-functions/_engram",
	"fish": homebrewPrefix + "/share/fish/vendor_completions.d/engram.fish",
}

// stepBlocks renders each quoted Homebrew token the way GoReleaser does and
// splits a steps body into one string per step: a step starts on a line with
// no leading space, and its indented option lines and the closing bracket of
// a multi-line list follow it.
func stepBlocks(body string) []string {
	var blocks []string
	for l := range strings.SplitSeq(quotedHomebrewToken.ReplaceAllString(body, "$1"), "\n") {
		if l == "" {
			continue
		}
		if (strings.HasPrefix(l, " ") || strings.HasPrefix(l, "]")) && len(blocks) > 0 {
			blocks[len(blocks)-1] += "\n" + l
			continue
		}
		blocks = append(blocks, l)
	}
	return blocks
}

// loadYAML parses a file, relative to this package's directory, into v.
func loadYAML(t *testing.T, relPath string, v any) {
	t.Helper()
	data, err := os.ReadFile(relPath)
	if err != nil {
		t.Fatalf("reading %s: %v", relPath, err)
	}
	if err := yaml.Unmarshal(data, v); err != nil {
		t.Fatalf("parsing %s: %v", relPath, err)
	}
}

// TestReleaseConfigCaskInstallGate pins the cask's install and uninstall
// steps in .goreleaser.yaml: the only hooks are post install_steps and
// uninstall_steps, with no custom_block; every Homebrew token is written
// quoted; every run step runs the linked bin/engram; the version gate runs
// against the cask's {{version}}, then the bash, zsh and fish completions
// write their install paths, then the man pages go into the man1 directory;
// no step sets must_succeed or runs xattr; the uninstall is one remove of
// exactly the files the install writes and never a directory. It also keeps
// out top-level brews and the cask keys that install prebuilt files or
// soften a failure, and pins the description to rootCmd.Short.
func TestReleaseConfigCaskInstallGate(t *testing.T) {
	var cfg struct {
		Brews         any              `yaml:"brews"`
		HomebrewCasks []map[string]any `yaml:"homebrew_casks"`
	}
	loadYAML(t, "../../.goreleaser.yaml", &cfg)
	if cfg.Brews != nil {
		t.Error("top-level `brews` is set; engram ships a cask only")
	}
	if len(cfg.HomebrewCasks) != 1 {
		t.Fatalf("want exactly 1 homebrew_casks entry, got %d", len(cfg.HomebrewCasks))
	}
	cask := cfg.HomebrewCasks[0]

	for _, key := range []string{"generate_completions_from_executable", "completions", "manpages"} {
		if _, ok := cask[key]; ok {
			t.Errorf("homebrew_casks[0].%s is set; the installed binary must write the completions and man pages, failing the install if it cannot", key)
		}
	}
	if desc, _ := cask["description"].(string); desc != rootCmd.Short {
		t.Errorf("cask description %q does not byte-match rootCmd.Short %q", desc, rootCmd.Short)
	}

	if _, ok := cask["custom_block"]; ok {
		t.Error("homebrew_casks[0].custom_block is set; it adds Ruby to the cask that the steps checks below never see")
	}
	hooks, _ := cask["hooks"].(map[string]any)
	if keys := slices.Sorted(maps.Keys(hooks)); !slices.Equal(keys, []string{"post"}) {
		t.Errorf("homebrew_casks[0].hooks keys = %q, want only post", keys)
	}
	post, _ := hooks["post"].(map[string]any)
	if keys, want := slices.Sorted(maps.Keys(post)), []string{"install_steps", "uninstall_steps"}; !slices.Equal(keys, want) {
		t.Errorf("hooks.post keys = %q, want %q; install and uninstall render Ruby flight blocks", keys, want)
	}
	installBody, _ := post["install_steps"].(string)
	uninstallBody, _ := post["uninstall_steps"].(string)
	if installBody == "" || uninstallBody == "" {
		t.Fatal("hooks.post.install_steps and uninstall_steps must both be set")
	}
	for name, body := range map[string]string{"install_steps": installBody, "uninstall_steps": uninstallBody} {
		if rest := quotedHomebrewToken.ReplaceAllString(body, ""); strings.Contains(rest, "{{") {
			t.Errorf("%s has a {{ that is not a quoted Homebrew token; GoReleaser would template it", name)
		}
		for _, banned := range []string{"must_succeed", "xattr"} {
			if strings.Contains(body, banned) {
				t.Errorf("%s contains %q", name, banned)
			}
		}
	}

	var order, written []string
	for _, step := range stepBlocks(installBody) {
		if strings.HasPrefix(step, "mkdir_p ") {
			continue
		}
		if !strings.HasPrefix(step, `run "bin/engram",`) || !homebrewPrefixBase.MatchString(step) {
			t.Errorf("every install run step must be run \"bin/engram\" with base: :homebrew_prefix, got step:\n%s", step)
		}
		switch {
		case strings.Contains(step, `["version", "--expect", "{{version}}"]`):
			order = append(order, "expect")
		case completionArg.MatchString(step):
			shell := completionArg.FindStringSubmatch(step)[1]
			order = append(order, "completion "+shell)
			path := stdoutPathArg.FindStringSubmatch(step)
			if path == nil || path[1] != caskCompletionPaths[shell] {
				t.Errorf("the %s completion step must write %q, got step:\n%s", shell, caskCompletionPaths[shell], step)
				continue
			}
			written = append(written, path[1])
		case strings.Contains(step, `["man", `):
			order = append(order, "man")
			if !strings.Contains(step, `["man", "`+caskManDir+`"]`) {
				t.Errorf("the man step must write into %q, got step:\n%s", caskManDir, step)
			}
			written = append(written, caskManDir+"/engram.1", caskManDir+"/engram-*.1")
		default:
			t.Errorf("install_steps has an unexpected step:\n%s", step)
		}
	}
	wantOrder := []string{"expect", "completion bash", "completion zsh", "completion fish", "man"}
	if !slices.Equal(order, wantOrder) {
		t.Errorf("install_steps order = %q, want %q", order, wantOrder)
	}

	if strings.Contains(uninstallBody, "recursive") {
		t.Error("uninstall_steps sets recursive; the uninstall removes files only")
	}
	uninstall := stepBlocks(uninstallBody)
	if len(uninstall) != 1 || !strings.HasPrefix(uninstall[0], "remove [") {
		t.Fatalf("uninstall_steps must be one `remove [...]` step, got:\n%s", strings.Join(uninstall, "\n"))
	}
	quoted := quotedString.FindAllStringSubmatch(uninstall[0], -1)
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

// TestReleaseConfigNotarizeMacOS pins the notarize block: one entry whose
// ids are exactly the darwin builds, whose sign and notarize fields read the
// five MACOS_* variables and wait for Apple's verdict, and that is enabled
// exactly when all five are set.
func TestReleaseConfigNotarizeMacOS(t *testing.T) {
	var cfg struct {
		Builds []struct {
			ID   string   `yaml:"id"`
			Goos []string `yaml:"goos"`
		} `yaml:"builds"`
		Notarize struct {
			MacOS []struct {
				Enabled string   `yaml:"enabled"`
				IDs     []string `yaml:"ids"`
				Sign    struct {
					Certificate string `yaml:"certificate"`
					Password    string `yaml:"password"`
				} `yaml:"sign"`
				Notarize struct {
					IssuerID string `yaml:"issuer_id"`
					KeyID    string `yaml:"key_id"`
					Key      string `yaml:"key"`
					Wait     bool   `yaml:"wait"`
				} `yaml:"notarize"`
			} `yaml:"macos"`
		} `yaml:"notarize"`
	}
	loadYAML(t, "../../.goreleaser.yaml", &cfg)
	if len(cfg.Notarize.MacOS) != 1 {
		t.Fatalf("want exactly 1 notarize.macos entry, got %d", len(cfg.Notarize.MacOS))
	}
	entry := cfg.Notarize.MacOS[0]

	var darwinBuilds []string
	for _, b := range cfg.Builds {
		if slices.Contains(b.Goos, "darwin") {
			darwinBuilds = append(darwinBuilds, b.ID)
		}
	}
	ids := slices.Clone(entry.IDs)
	slices.Sort(ids)
	slices.Sort(darwinBuilds)
	if len(darwinBuilds) == 0 || !slices.Equal(ids, darwinBuilds) {
		t.Errorf("notarize.macos[0].ids = %q, want exactly the darwin build ids %q", entry.IDs, darwinBuilds)
	}

	for _, f := range []struct{ field, got, env string }{
		{"sign.certificate", entry.Sign.Certificate, "MACOS_SIGN_P12"},
		{"sign.password", entry.Sign.Password, "MACOS_SIGN_PASSWORD"},
		{"notarize.issuer_id", entry.Notarize.IssuerID, "MACOS_NOTARY_ISSUER_ID"},
		{"notarize.key_id", entry.Notarize.KeyID, "MACOS_NOTARY_KEY_ID"},
		{"notarize.key", entry.Notarize.Key, "MACOS_NOTARY_KEY"},
	} {
		if want := "{{ .Env." + f.env + " }}"; f.got != want {
			t.Errorf("notarize.macos[0].%s = %q, want %q", f.field, f.got, want)
		}
	}
	if !entry.Notarize.Wait {
		t.Error("notarize.macos[0].notarize.wait is not true; without it Apple rejecting a binary cannot fail the release")
	}

	terms := make([]string, len(macosSigningVars))
	for i, v := range macosSigningVars {
		terms[i] = `(isEnvSet "` + v + `")`
	}
	if got, want := strings.TrimSpace(entry.Enabled), "{{ and "+strings.Join(terms, " ")+" }}"; got != want {
		t.Errorf("notarize.macos[0].enabled = %q, want %q", got, want)
	}
}

// TestReleaseConfigSigningSecretsEnv pins which release.yaml steps get the
// signing secrets: only the release job's "Require the macOS signing secrets"
// step and its goreleaser-action step, each with exactly the five MACOS_*
// secrets under their own names, so the check reads what GoReleaser reads.
func TestReleaseConfigSigningSecretsEnv(t *testing.T) {
	var wf struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string            `yaml:"name"`
				Uses string            `yaml:"uses"`
				Env  map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	loadYAML(t, "../../.github/workflows/release.yaml", &wf)

	want := make(map[string]string, len(macosSigningVars))
	for _, v := range macosSigningVars {
		want[v] = "${{ secrets." + v + " }}"
	}
	var readers []string
	for job, j := range wf.Jobs {
		for _, s := range j.Steps {
			got := map[string]string{}
			for k, v := range s.Env {
				if strings.HasPrefix(k, "MACOS_") || strings.Contains(v, "secrets.MACOS_") {
					got[k] = v
				}
			}
			if len(got) == 0 {
				continue
			}
			step := job + ": " + s.Name + s.Uses
			readers = append(readers, step)
			allowed := s.Name == "Require the macOS signing secrets" || strings.HasPrefix(s.Uses, "goreleaser/goreleaser-action@")
			if job != "release" || !allowed {
				t.Errorf("%s reads the signing secrets; only the release job's check and goreleaser steps may", step)
				continue
			}
			if !maps.Equal(got, want) {
				t.Errorf("%s env MACOS_* = %v, want %v", step, got, want)
			}
		}
	}
	if len(readers) != 2 {
		t.Errorf("signing secrets reach %q, want exactly the check step and the goreleaser step", readers)
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
