// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import (
	"os"
	"path/filepath"
)

// Presence is a REPORT-ONLY view of what already exists at a runtime's
// native skills destination (and, for FormatAgentsMD, its index file):
// the operator is TOLD what is there so they can remove it by hand if
// they wish; DetectPresence itself never removes anything (a future
// explicit prune verb is the only sanctioned place for removal —
// REQ-plugin-skips-skills-copy's "never removes a skills path without
// first checking for a managed symlink" clause is satisfied here by
// never removing at all, D-08/D-09).
type Presence struct {
	// Skills is the number of entries in the list passed to
	// DetectPresence whose own directory (filepath.Join(target.Dir,
	// skill.Name)) can be statted successfully. Any stat error — the
	// entry does not exist, or any other error such as a permission
	// denial — counts as absent: this is a report, and ambiguity should
	// under-report rather than invent presence.
	Skills int
	// Symlink is true when target.Dir ITSELF, or ANY counted skill
	// entry beneath it, carries os.ModeSymlink. Both the maintainer's
	// own hand-made per-skill symlinks and a single symlinked
	// destination directory are "the symlink case" — either shape means
	// a copy was never actually written there.
	Symlink bool
	// IndexBlock is true, for FormatAgentsMD targets only, when
	// target.IndexFile can be read and its content carries an engram
	// skills index block in ANY state other than absent — a malformed
	// block still counts as present, since it is something an operator
	// needs to see and address, not nothing. FormatNative and
	// FormatNone never set this field.
	IndexBlock bool
}

// DetectPresence reports what already exists at target's native
// destination without writing, creating, renaming, or removing anything:
// it consults env ONLY through Lstat (to distinguish a symlink from a
// regular entry) and ReadFile (to classify an AGENTS.md-shaped index
// file's existing skills block, reusing this package's own scanBlock
// classifier rather than a second marker scan). FormatNone has no
// filesystem destination and returns the zero Presence without any env
// call at all.
func DetectPresence(env Environment, target Target, list []Skill) Presence {
	if target.Format == FormatNone {
		return Presence{}
	}

	var p Presence

	if fi, err := env.Lstat(target.Dir); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		p.Symlink = true
	}

	for _, s := range list {
		fi, err := env.Lstat(filepath.Join(target.Dir, s.Name))
		if err != nil {
			continue
		}
		p.Skills++
		if fi.Mode()&os.ModeSymlink != 0 {
			p.Symlink = true
		}
	}

	if target.Format == FormatAgentsMD && target.IndexFile != "" {
		if content, err := env.ReadFile(target.IndexFile); err == nil {
			state, _, _, _, _, _ := scanBlock(content)
			p.IndexBlock = state != blockAbsent
		}
	}

	return p
}
