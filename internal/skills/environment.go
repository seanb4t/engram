// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import "os"

// Environment is the injectable filesystem seam Install consults for
// every write-path external-boundary call. Modeled on
// internal/setup.Environment (a struct of func fields, never an
// interface — the same family as cliNow in cmd/engram/destructive.go and
// citationFileReader in cmd/engram/spine_review_verify.go), so a test can
// build a fake value backed by an in-memory map and inject it directly,
// never touching the real home directory (repo rule m45p2b4bp7).
//
// Deliberately NO home-directory field: every destination this package
// ever writes to arrives already ABSOLUTE, authored inside a runtime's
// own Plan() from setup.Environment.HomeDir() (D-10). A later contributor
// must not re-add path derivation here — that would let this package
// construct a destination itself, which is exactly the authority D-10
// keeps out of it.
type Environment struct {
	// ReadFile mirrors os.ReadFile: the whole file's content, or a
	// non-nil error. Install treats ANY read error identically — a
	// missing file and a permission error both mean "cannot prove this
	// destination already matches," which resolves to the write case
	// (D-08's ambiguity-resolves-to-wrote invariant, applied one layer
	// down).
	ReadFile func(name string) ([]byte, error)
	// WriteFile mirrors os.WriteFile, including its file-mode parameter.
	WriteFile func(name string, data []byte, perm os.FileMode) error
	// MkdirAll mirrors os.MkdirAll: create name and every missing parent
	// directory.
	MkdirAll func(name string, perm os.FileMode) error
}

// OSEnvironment is the real, production Environment: os.ReadFile,
// os.WriteFile, and os.MkdirAll directly. This is the only Environment
// value any non-test code path constructs.
var OSEnvironment = Environment{
	ReadFile:  os.ReadFile,
	WriteFile: os.WriteFile,
	MkdirAll:  os.MkdirAll,
}

const (
	// dirPerm is the mode Install creates a skill's destination
	// directory with — umask-respecting, like every other directory this
	// binary creates.
	dirPerm os.FileMode = 0o755
	// filePerm is the mode Install writes a skill's destination file
	// with — umask-respecting. No skill file carries secret material —
	// the shipped skills are public plugin content.
	filePerm os.FileMode = 0o644
)
