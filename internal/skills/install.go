// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
)

// Format classifies a Target's write shape. It is a three-value enum,
// string-backed for readable rendering, following internal/setup's
// Outcome idiom: every value is a REAL, explicit value, never modeled as
// an absence or the zero value — a Target built without an explicit
// Format is a programming error, not a "do nothing" instruction.
type Format string

const (
	// FormatNone means this runtime has no filesystem destination for
	// skills at all (D-11's "no skills" case, e.g. the generic
	// pseudo-runtime) — Install has nothing to write and returns
	// immediately.
	FormatNone Format = "none"
	// FormatNative means Dir is a runtime's own native skill directory —
	// one subdirectory per skill, each file written at its own relative
	// path beneath it.
	FormatNative Format = "native"
	// FormatAgentsMD means the runtime has no native skill format and
	// falls back to a delimited block in an AGENTS.md-shaped file
	// (D-13/D-15/D-16). Not implemented by this task — Install returns
	// an error naming that.
	FormatAgentsMD Format = "agents-md"
)

// Target is one runtime's authored skills-install destination: Format
// selects the write shape, Dir is the absolute destination directory (for
// FormatNative and FormatAgentsMD), and IndexFile is the absolute path of
// the file the AGENTS.md splice targets (meaningful only for
// FormatAgentsMD).
type Target struct {
	Format    Format
	Dir       string
	IndexFile string
}

// Report is one Install call's outcome: Wrote and AlreadyCorrect each
// name every destination PATH that landed in that bucket (D-08's
// byte-compare decision, per file), and Err accumulates every failure
// encountered across the whole call via errors.Join (D-07) — a failure on
// one skill never skips a later skill, and never discards an earlier
// skill's success.
type Report struct {
	Wrote          []string
	AlreadyCorrect []string
	Err            error
}

// Install writes skills to target through env. FormatNone has nothing to
// write and returns an empty, error-free Report. FormatAgentsMD is not
// implemented by this task and returns a Report whose Err states that.
// FormatNative writes each skill's files beneath target.Dir, one
// subdirectory per skill.
func Install(env Environment, target Target, skills []Skill) Report {
	switch target.Format {
	case FormatNone:
		return Report{}
	case FormatNative:
		return installNative(env, target, skills)
	case FormatAgentsMD:
		return Report{Err: errors.New("skills: agents-md install is not wired yet")}
	default:
		return Report{Err: fmt.Errorf("skills: install: unrecognized format %q", target.Format)}
	}
}

// installNative implements FormatNative: for each skill and each of its
// files, join target.Dir, the skill's own Name, and the file's own
// relative Path; create the parent directory; read the existing
// destination; when the read succeeds and its bytes equal the embedded
// content, report AlreadyCorrect and write nothing; otherwise write the
// embedded content UNCONDITIONALLY and report Wrote (D-08). Every error —
// a MkdirAll failure or a WriteFile failure — is accumulated via
// errors.Join rather than stopping the loop (D-07): one skill's failure
// never prevents a later skill, or a later file within the SAME skill,
// from being attempted.
func installNative(env Environment, target Target, list []Skill) Report {
	var report Report
	var errs []error

	for _, s := range list {
		for _, f := range s.Files {
			dest := filepath.Join(target.Dir, s.Name, f.Path)
			dir := filepath.Dir(dest)

			if err := env.MkdirAll(dir, dirPerm); err != nil {
				errs = append(errs, fmt.Errorf("skills: mkdir %s: %w", dir, err))
				continue
			}

			existing, readErr := env.ReadFile(dest)
			if readErr == nil && bytes.Equal(existing, f.Content) {
				report.AlreadyCorrect = append(report.AlreadyCorrect, dest)
				continue
			}
			// readErr != nil (file does not exist, or any other read
			// failure — Environment carries no way to distinguish those,
			// and D-08's ambiguity-resolves-to-wrote invariant means we
			// do not need to) or the bytes differ: write unconditionally.
			if err := env.WriteFile(dest, f.Content, filePerm); err != nil {
				errs = append(errs, fmt.Errorf("skills: write %s: %w", dest, err))
				continue
			}
			report.Wrote = append(report.Wrote, dest)
		}
	}

	report.Err = errors.Join(errs...)
	return report
}
