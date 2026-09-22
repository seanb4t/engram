// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storetest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// moduleRoot walks up from the working directory to the directory holding
// go.mod, failing loudly if none is found. Mirrors internal/store's own
// findModuleRoot (schemaversion_stamp_gate_test.go), duplicated here rather
// than exported cross-package since this file is the only caller in this
// package.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found walking up from %s", dir)
		}
		dir = parent
	}
}

// qdrantImageRefPattern matches a qdrant/qdrant:vX.Y.Z-style image reference
// anywhere in CI's YAML text.
var qdrantImageRefPattern = regexp.MustCompile(`qdrant/qdrant:v[0-9][0-9A-Za-z.+-]*`)

// TestQdrantImageMatchesCIService is D-10's gate: CI's shared services.qdrant
// image, and every testcontainer fallback across this module (QdrantImage,
// above), must stay byte-identical — internal/store's
// qdrantTOCTOUVerifiedVersion guard exists to force a conscious
// re-verification whenever the version bumps, and that guard is only
// meaningful if the pin it re-verifies is actually the one CI runs. This
// test turns that pin into a gate instead of a comment.
func TestQdrantImageMatchesCIService(t *testing.T) {
	root := moduleRoot(t)
	path := filepath.Join(root, ".github", "workflows", "ci.yaml")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(src)

	matches := qdrantImageRefPattern.FindAllString(text, -1)
	if len(matches) == 0 {
		t.Fatalf("%s: zero qdrant/qdrant:vX.Y.Z references found — a scan that sees nothing must not report clean", path)
	}
	for _, m := range matches {
		if m != QdrantImage {
			t.Errorf("%s: found image reference %q, want %q (byte-identical to storetest.QdrantImage)", path, m, QdrantImage)
		}
	}

	want := "image: " + QdrantImage
	if !strings.Contains(text, want) {
		t.Errorf("%s: does not contain %q — services.qdrant.image must name storetest.QdrantImage directly", path, want)
	}
}
