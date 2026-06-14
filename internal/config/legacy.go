// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"fmt"
	"sort"
	"strings"
)

// legacyMap is every retired MEM_* var → its ENGRAM_* replacement. It is the
// registry's Legacy column plus the command-local vars that are read directly
// by their command (and so are not in the Config registry). Delete this whole
// file at 1.0.
var legacyMap = func() map[string]string {
	m := map[string]string{
		// command-local vars read by a real command (reindex.go / migrate.go).
		// Test-only vars (e.g. MEM_QDRANT_TEST_ADDR, read solely by integration
		// tests) are deliberately NOT guarded here: the runtime binary never
		// reads them, so flagging them would false-trip a CI/dev env that
		// exports the legacy name to point tests at Qdrant.
		"MEM_REINDEX_TARGET": "ENGRAM_REINDEX_TARGET",
		"MEM_MIGRATE_OWNER":  "ENGRAM_MIGRATE_OWNER",
	}
	for _, f := range registry {
		if f.Legacy != "" {
			m[f.Legacy] = f.Env
		}
	}
	return m
}()

// CheckLegacy returns an error naming every retired MEM_* variable present in
// environ (the os.Environ() slice form, "KEY=VALUE"), mapped to its ENGRAM_*
// replacement. Returns nil when none are set. Called from root PersistentPreRunE
// so a half-migrated deployment fails fast instead of silently falling back to a
// default.
func CheckLegacy(environ []string) error {
	var hits []string
	for _, kv := range environ {
		name := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			name = kv[:i]
		}
		if repl, ok := legacyMap[name]; ok {
			hits = append(hits, fmt.Sprintf("  %s → %s", name, repl))
		}
	}
	if len(hits) == 0 {
		return nil
	}
	sort.Strings(hits)
	return fmt.Errorf("retired environment variables are set and no longer read:\n%s\nRename them (see the v0.x migration notes) and retry",
		strings.Join(hits, "\n"))
}
