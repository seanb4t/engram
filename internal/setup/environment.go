// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"os"
	"os/exec"
)

// Environment is the injectable seam every Runtime's Detect/Plan consults
// for external-boundary reads: which binaries are on PATH, environment
// variables, and the caller's home directory. Modeled on cliNow
// (cmd/engram/destructive.go) and citationFileReader
// (cmd/engram/spine_review_verify.go): a struct of func fields, so a test
// can build a fake value and inject it directly (cmd/engram/setup.go
// overrides its own package-level seam via t.Cleanup) instead of mutating
// the real PATH — repo rule m45p2b4bp7 forbids testing or red-gating
// third-party CLI behavior.
type Environment struct {
	// LookPath resolves file the same way exec.LookPath does: a resolved
	// path and a nil error when file is found on PATH, or a non-nil error
	// (exec.ErrNotFound in the real implementation) when it is not. This is
	// the ONLY signal Detect consults (D-12) — never a config-directory
	// stat.
	LookPath func(file string) (string, error)
	// Getenv mirrors os.Getenv: the empty string for an unset variable,
	// never an error.
	Getenv func(key string) string
	// HomeDir mirrors os.UserHomeDir.
	HomeDir func() (string, error)
}

// OSEnvironment is the real, production Environment: exec.LookPath,
// os.Getenv, and os.UserHomeDir. This is the only Environment value any
// non-test code path constructs.
var OSEnvironment = Environment{
	LookPath: exec.LookPath,
	Getenv:   os.Getenv,
	HomeDir:  os.UserHomeDir,
}
