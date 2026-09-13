// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// manDate is pinned to the Unix epoch in UTC (D-01) so every generated
// page's .TH date is byte-stable across machines and releases. cobra/doc
// renders the .TH date with Format("Jan 2006"): a nil Date falls back to
// SOURCE_DATE_EPOCH then time.Now(), which is byte-stable only within a
// calendar month and depends on an env var the cask hook's end-user
// machine never exports. .UTC() is load-bearing, not decorative: the
// same epoch instant with no zone conversion renders in the process's
// local zone, which prints "Dec 1969" on any negative-UTC-offset machine
// (most of the Americas) — only .UTC() makes "Jan 1970" true everywhere,
// including on the CI runner and on an arbitrary end user's machine at
// cask-install time.
var manDate = time.Unix(0, 0).UTC()

// manManual is D-02's fixed Manual field for every generated page.
const manManual = "Engram Manual"

// manHeader builds a fresh *doc.GenManHeader per call (doc.GenManTree
// takes ownership of the pointer it's given via a shallow copy per file
// — see cobra/doc man_docs.go's GenManTreeFromOpts — so returning a new
// struct each time avoids two callers' GenManTree runs mutating the same
// shared header). Date still points at the single package-level manDate
// (never a per-call copy): that aliasing is intentional, not a surprise
// to avoid, because manDate is set once at init and never mutated
// afterward (IN-01, 01-REVIEW.md). Source is "engram " + version: the RAW
// ldflags-injected version package var
// (root.go:19), the SAME value rootCmd.Version carries (root.go:78),
// deliberately not the fully resolved version string used elsewhere in
// this binary — D-02 makes the same choice root.go's own init()
// explains: a dev build's pages read "engram dev".
// Title and Section are left unset so cobra fills them in (the
// upper-cased dashed command path, and "1", respectively).
func manHeader() *doc.GenManHeader {
	return &doc.GenManHeader{
		Date:   &manDate,
		Source: "engram " + version,
		Manual: manManual,
	}
}

// writeManPages generates one roff man page per available command in
// root's tree into dir (created if absent) using cobra/doc.GenManTree.
//
// The three InitDefault* calls mirror golden_test.go's own precedent
// (buildHelpGoldenContent): rootCmd.Execute() only lazily registers
// --version/help/completion from inside execute() when SOME command in
// the shared, process-wide rootCmd has already run one, so calling this
// function directly (as the man-page tests do, bypassing Execute) would
// otherwise generate a smaller page set than a real `engram man <dir>`
// invocation. Forcing them here makes the generated set and the root
// page's own OPTIONS section identical regardless of caller.
//
// The tree is snapshotted before generation and pruned back to that
// snapshot afterward (even on error) because cobra/doc's own genMan
// calls InitDefaultHelpCmd() on every command it renders, which grafts
// a "help" child onto every subgroup (migrate, spine-review, completion)
// — a child a real `engram <group> --help` never lists. Because rootCmd
// is a process-wide singleton shared with every other test in this
// binary, leaving it mutated makes TestHelpGolden drift whenever a
// man-page generation happens to run first in the same test process
// (reproduced during planning). The generated pages themselves are
// unaffected: GenManTree's own walk and its SEE ALSO section both
// already exclude the help command by predicate, before this restore
// ever runs.
func writeManPages(root *cobra.Command, dir string) error {
	root.InitDefaultVersionFlag()
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	snap := snapshotCommandTree(root)
	err := doc.GenManTree(root, manHeader(), dir)
	pruneToSnapshot(snap)
	return err
}

// snapshotCommandTree records, for c and every descendant, a COPY of its
// current children — the state pruneToSnapshot restores after generation
// mutates the live tree (see writeManPages' doc comment).
func snapshotCommandTree(c *cobra.Command) map[*cobra.Command][]*cobra.Command {
	snap := make(map[*cobra.Command][]*cobra.Command)
	var walk func(*cobra.Command)
	walk = func(cur *cobra.Command) {
		children := cur.Commands()
		cp := make([]*cobra.Command, len(children))
		copy(cp, children)
		snap[cur] = cp
		for _, ch := range children {
			walk(ch)
		}
	}
	walk(c)
	return snap
}

// pruneToSnapshot removes, from every recorded parent, any current child
// not present in that parent's snapshotted child list — undoing the
// "help" children cobra/doc's genMan grafts on during generation.
func pruneToSnapshot(snap map[*cobra.Command][]*cobra.Command) {
	for parent, wantChildren := range snap {
		want := make(map[*cobra.Command]bool, len(wantChildren))
		for _, ch := range wantChildren {
			want[ch] = true
		}
		current := parent.Commands()
		for _, ch := range current {
			if !want[ch] {
				parent.RemoveCommand(ch)
			}
		}
	}
}

// manCmd is a hidden command wrapping cobra/doc.GenManTree over the live
// rootCmd. It is used by the Homebrew cask's post_install hook to write
// this binary's own man pages straight into the installed man1
// directory (.goreleaser.yaml). Hidden: true excludes it from every
// consumer of this package's shared hidden/help/completion skip
// predicate (the catalog, help.golden, the flag-group and exit-code
// gates) automatically — no surfaces classification row is needed and
// task surfaces:gen must not be run for it.
var manCmd = &cobra.Command{
	Use:    "man <dir>",
	Short:  "Write one roff man page per command into <dir> (used by the Homebrew cask post-install hook)",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return writeManPages(rootCmd, args[0])
	},
}

func init() {
	// D-03: set once on the root; cobra/doc's own VisitParents walk
	// copies this down onto every command it renders (see man_docs.go's
	// hasSeeAlso/genMan), which is what drops the date-bearing
	// "# HISTORY … Auto generated by spf13/cobra" footer tree-wide,
	// including on pages generated from a subcommand with no
	// DisableAutoGenTag of its own.
	rootCmd.DisableAutoGenTag = true
	rootCmd.AddCommand(manCmd)
}
