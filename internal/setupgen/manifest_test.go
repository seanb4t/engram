// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setupgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// repoRoot walks parent directories from the test's working directory
// until one contains go.mod, failing loudly rather than silently passing
// on a moved tree.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repoRoot: no go.mod found walking up from %s", dir)
		}
		dir = parent
	}
}

// pluginVersionCorePattern mirrors internal/setup's own release-core
// grammar (bare X.Y.Z, no "v" prefix, no prerelease/build metadata) — the
// exact shape release-please writes and classifyPluginVersion requires.
var manifestVersionCorePattern = regexp.MustCompile(`^(0|[1-9][0-9]*)[.](0|[1-9][0-9]*)[.](0|[1-9][0-9]*)$`)

func readManifest(t *testing.T, root, rel string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", rel, err)
	}
	return m
}

func TestPluginManifestIdentityMatches(t *testing.T) {
	root := repoRoot(t)
	claude := readManifest(t, root, filepath.Join("skill", "engram", ".claude-plugin", "plugin.json"))
	codex := readManifest(t, root, filepath.Join("skill", "engram", ".codex-plugin", "plugin.json"))

	keys := make([]string, 0, len(codex))
	for k := range codex {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	want := []string{"$schema", "description", "name", "version"}
	if !slices.Equal(keys, want) {
		t.Fatalf("codex manifest keys = %v, want exactly %v", keys, want)
	}
	if _, ok := codex["hooks"]; ok {
		t.Fatal("codex manifest must not carry a hooks key")
	}
	if _, ok := codex["mcpServers"]; ok {
		t.Fatal("codex manifest must not carry an mcpServers key")
	}

	if got, want := codex["$schema"], "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"; got != want {
		t.Fatalf("codex $schema = %v, want %v", got, want)
	}

	for _, field := range []string{"name", "version", "description"} {
		cv, cok := claude[field].(string)
		xv, xok := codex[field].(string)
		if !cok || !xok {
			t.Fatalf("field %q is not a string on both manifests: claude=%v codex=%v", field, claude[field], codex[field])
		}
		if cv != xv {
			t.Fatalf("field %q diverges: claude=%q codex=%q", field, cv, xv)
		}
	}
	if name, _ := codex["name"].(string); name != "engram" {
		t.Fatalf("codex name = %q, want %q", name, "engram")
	}

	version, _ := codex["version"].(string)
	if !manifestVersionCorePattern.MatchString(version) {
		t.Fatalf("codex version %q is not a bare X.Y.Z core", version)
	}

	// release-please-config.json's extra-files must sync the codex
	// manifest's $.version exactly once, beside the unchanged
	// .claude-plugin entry.
	rpData, err := os.ReadFile(filepath.Join(root, "release-please-config.json"))
	if err != nil {
		t.Fatalf("read release-please-config.json: %v", err)
	}
	var rp struct {
		Packages map[string]struct {
			ExtraFiles []map[string]any `json:"extra-files"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(rpData, &rp); err != nil {
		t.Fatalf("unmarshal release-please-config.json: %v", err)
	}
	pkg, ok := rp.Packages["."]
	if !ok {
		t.Fatal(`release-please-config.json: missing packages["."]`)
	}

	countEntry := func(path string) int {
		n := 0
		for _, e := range pkg.ExtraFiles {
			if p, _ := e["path"].(string); p == path {
				n++
			}
		}
		return n
	}
	if n := countEntry("skill/engram/.codex-plugin/plugin.json"); n != 1 {
		t.Fatalf("release-please-config.json extra-files entries for codex manifest = %d, want exactly 1", n)
	}
	if n := countEntry("skill/engram/.claude-plugin/plugin.json"); n != 1 {
		t.Fatalf("release-please-config.json extra-files entries for claude manifest = %d, want exactly 1 (twin must not replace it)", n)
	}
	for _, e := range pkg.ExtraFiles {
		if p, _ := e["path"].(string); p != "skill/engram/.codex-plugin/plugin.json" {
			continue
		}
		if typ, _ := e["type"].(string); typ != "json" {
			t.Fatalf("codex extra-files entry type = %q, want %q", typ, "json")
		}
		if jp, _ := e["jsonpath"].(string); jp != "$.version" {
			t.Fatalf("codex extra-files entry jsonpath = %q, want %q", jp, "$.version")
		}
	}

	t.Run("description-is-vendor-neutral", func(t *testing.T) {
		description, _ := codex["description"].(string)
		lower := strings.ToLower(description)
		banned := []string{
			"fzy" + "mgc",
			"lite" + "llm",
			"memory" + "_oauth",
		}
		for _, needle := range banned {
			if strings.Contains(lower, needle) {
				t.Fatalf("codex manifest description contains banned substring %q", needle)
			}
		}
	})
}
