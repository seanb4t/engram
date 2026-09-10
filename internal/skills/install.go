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
	// (D-13/D-15/D-16): the skill files are written beneath Dir exactly
	// as FormatNative writes them, and the index block is additionally
	// spliced into IndexFile.
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
// write and returns an empty, error-free Report. FormatNative writes each
// skill's files beneath target.Dir, one subdirectory per skill.
// FormatAgentsMD does the same, using the SAME shared file-install
// helper (installFiles) so the two formats' byte-compare-then-overwrite
// logic can never drift, and additionally splices the skills index block
// into target.IndexFile (installAgentsMDIndex) — independently: a
// malformed index file accumulates its own error without skipping or
// undoing the skill-file writes (D-07).
func Install(env Environment, target Target, skills []Skill) Report {
	switch target.Format {
	case FormatNone:
		return Report{}
	case FormatNative:
		wrote, alreadyCorrect, errs := installFiles(env, target.Dir, skills)
		return Report{Wrote: wrote, AlreadyCorrect: alreadyCorrect, Err: errors.Join(errs...)}
	case FormatAgentsMD:
		wrote, alreadyCorrect, errs := installFiles(env, target.Dir, skills)
		wrote, alreadyCorrect, errs = installAgentsMDIndex(env, target, skills, wrote, alreadyCorrect, errs)
		return Report{Wrote: wrote, AlreadyCorrect: alreadyCorrect, Err: errors.Join(errs...)}
	default:
		return Report{Err: fmt.Errorf("skills: install: unrecognized format %q", target.Format)}
	}
}

// installFiles is the ONE byte-compare-then-overwrite implementation both
// FormatNative and FormatAgentsMD call: for each skill and each of its
// files, join dir, the skill's own Name, and the file's own relative
// Path; create the parent directory; read the existing destination; when
// the read succeeds and its bytes equal the embedded content, report it
// AlreadyCorrect and write nothing; otherwise write the embedded content
// UNCONDITIONALLY and report it Wrote (D-08). Every error — a MkdirAll
// failure or a WriteFile failure — is accumulated into errs rather than
// stopping the loop (D-07): one skill's failure never prevents a later
// skill, or a later file within the SAME skill, from being attempted.
func installFiles(env Environment, dir string, list []Skill) (wrote []string, alreadyCorrect []string, errs []error) {
	for _, s := range list {
		for _, f := range s.Files {
			dest := filepath.Join(dir, s.Name, f.Path)
			destDir := filepath.Dir(dest)

			if err := env.MkdirAll(destDir, dirPerm); err != nil {
				errs = append(errs, fmt.Errorf("skills: mkdir %s: %w", destDir, err))
				continue
			}

			existing, readErr := env.ReadFile(dest)
			if readErr == nil && bytes.Equal(existing, f.Content) {
				alreadyCorrect = append(alreadyCorrect, dest)
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
			wrote = append(wrote, dest)
		}
	}
	return wrote, alreadyCorrect, errs
}

// installAgentsMDIndex performs FormatAgentsMD's second, independent
// step: splicing the skills index block into target.IndexFile. It reads
// the index file through the seam — ANY read error, not just "does not
// exist", is treated as empty content (the create case), matching
// installFiles' own ambiguity-resolves-to-wrote posture one layer up.
// Rendering and splicing is pure (agentsmd.go); this function's only job
// is deciding whether that result needs writing, and doing so.
//
// On a malformed result, Splice itself refuses to produce content
// (D-15): that error is accumulated onto errs and NOTHING is written to
// the index file — but wrote and alreadyCorrect, already populated by the
// skill-file pass, are returned UNCHANGED. This split is the deliberate
// reading of D-15 and D-07 together: not one byte reaches an ambiguous
// AGENTS.md, and nothing else is skipped because that one thing failed.
//
// On a well-formed or absent result: comparing the spliced document
// against the content just read decides the outcome exactly as D-08
// decides it for a skill file — equal means AlreadyCorrect and no write;
// different means create the index file's parent directory if needed and
// write the WHOLE spliced document with a single call to the seam's
// whole-file write function (never staged-and-renamed — D-16), then
// report it Wrote.
func installAgentsMDIndex(
	env Environment, target Target, skills []Skill,
	wrote []string, alreadyCorrect []string, errs []error,
) ([]string, []string, []error) {
	existing, readErr := env.ReadFile(target.IndexFile)
	if readErr != nil {
		existing = nil
	}

	block := RenderBlock(skills, target.Dir)
	spliced, spliceErr := Splice(existing, block)
	if spliceErr != nil {
		errs = append(errs, fmt.Errorf("skills: index %s: %w", target.IndexFile, spliceErr))
		return wrote, alreadyCorrect, errs
	}

	if bytes.Equal(spliced, existing) {
		alreadyCorrect = append(alreadyCorrect, target.IndexFile)
		return wrote, alreadyCorrect, errs
	}

	// The write is deliberately in place and deliberately not atomic
	// (D-16): env.WriteFile is a single os.WriteFile-shaped call against
	// target.IndexFile's own path. os.WriteFile on an existing symlink
	// writes THROUGH the link — opening, truncating, and rewriting the
	// link's TARGET file — while the alternative (stage into a sibling
	// temporary path, then os.Rename over the target) REPLACES the link
	// itself with a regular file, silently breaking a stow/chezmoi/yadm
	// managed dotfile. That is a common way a runtime's own AGENTS.md
	// exists at all — this repository's own AGENTS.md is one such
	// symlink. The accepted cost, stated plainly: this write is not
	// atomic, and a crash mid-write truncates the file; the window is one
	// write call of a few kilobytes, and the recovery is re-running
	// `--apply`, which converges from any partial state because every
	// write here is an unconditional overwrite decided by byte-compare.
	indexDir := filepath.Dir(target.IndexFile)
	if err := env.MkdirAll(indexDir, dirPerm); err != nil {
		errs = append(errs, fmt.Errorf("skills: mkdir %s: %w", indexDir, err))
		return wrote, alreadyCorrect, errs
	}
	if err := env.WriteFile(target.IndexFile, spliced, filePerm); err != nil {
		errs = append(errs, fmt.Errorf("skills: write index %s: %w", target.IndexFile, err))
		return wrote, alreadyCorrect, errs
	}
	wrote = append(wrote, target.IndexFile)
	return wrote, alreadyCorrect, errs
}
