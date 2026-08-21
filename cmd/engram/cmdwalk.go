// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// commandWalkSkip is the single skip predicate every recursive walk in this
// package applies, at EVERY depth: a command explicitly marked Hidden, or
// cobra's own "help"/"completion" scaffolding (auto-registered at
// Execute() time via InitDefaultHelpCmd/InitDefaultCompletionCmd). A
// command for which this returns true is excluded from walkCommands'
// result AND its whole subtree is never visited — applying the skip only
// at the top level (the pre-D-01 behavior) would let a hidden or
// scaffolding command smuggle a nested leaf past every conformance gate
// that reads this predicate.
func commandWalkSkip(cmd *cobra.Command) bool {
	return cmd.Hidden || cmd.Name() == "help" || cmd.Name() == "completion"
}

// commandKey returns cmd's qualified path relative to the binary: cmd's own
// cobra.CommandPath() (e.g. "engram spine-review scan") with the root
// binary's own name and the following space trimmed, so a top-level
// command becomes its bare name ("reindex") and a nested leaf becomes its
// space-joined path ("spine-review scan"). This is the key every
// blast-radius lookup (internal/surfaces.ClassForCommand) and every
// catalog entry name uses post-D-01 — never cmd.Name() alone, which
// collides across a group and its own leaves sharing a name.
func commandKey(cmd *cobra.Command) string {
	path := cmd.CommandPath()
	prefix := cmd.Root().Name() + " "
	if strings.HasPrefix(path, prefix) {
		return strings.TrimPrefix(path, prefix)
	}
	return path
}

// walkCommands performs a depth-first, pre-order traversal of from's
// descendant tree (from itself is never included), returning every
// descendant for which skip is false — not descending into a skipped
// command's subtree at all. Each level's children are visited in
// lexicographic order by Name() before descending, so the result is
// independent of registration order (memory jb33frww29): a two-level tree
// yields parent, then that parent's own children, in that order, at every
// level.
//
// The parameter is deliberately named "from", NOT "root": this package's
// walker-conversion acceptance gate is a grep for the literal call form
// `(rootCmd|root)\.Commands()`, run over the whole package to prove every
// single-level walk site has been converted to this shared helper. Naming
// this parameter "root" would make the walker's own recursive call below
// match that same regex, corrupting the gate's own empty-post-state
// invariant. This walker is the ONLY caller of Commands() left in the
// package once every other site (buildCatalog, goldenCommands,
// withGoldenDeterminism, nonHiddenCommands, wantCommandNames,
// TestEveryDeclaredExclusivityHasAFlagGroup, resetEveryCommandFlagState)
// has been converted to call this function instead.
// operatorCommandExclusions is the small NAMED set operatorCommands drops
// after its two structural filters (non-nil RunE, no "server" flag) have
// already run. Each exclusion exists for a reason a structural filter
// cannot express:
//
//   - "serve": the daemon, not a report — it has a RunE and registers no
//     "server" client flag, so it would otherwise pass both structural
//     filters, but it never produces an operator-facing summary to render
//     through --output.
//   - "version": prints a constant — currently registered with a plain
//     Run (nil RunE), so today's structural filter alone already excludes
//     it; named here anyway so the exclusion survives a future change to
//     Run/RunE without silently widening operatorCommands' membership.
//
// TestOperatorCommands asserts this set is EXACTLY {"serve", "version"}
// and that every name in it resolves to a live command in the tree, so a
// future addition to this set cannot go unnoticed and a stale entry
// (naming a command that no longer exists) cannot silently persist.
var operatorCommandExclusions = map[string]bool{
	"serve":   true,
	"version": true,
}

// operatorCommands returns every operator-tier command in the live cobra
// tree: the concrete structural predicate the phase's --output parity and
// timeout-group gates derive from, replacing the previous informal
// "whatever walkCommands classifies as an operator command" language —
// walkCommands is a pure traversal with a hidden/help/completion skip
// predicate and performs no tier classification at all.
//
// The predicate, in order: walk the whole tree (walkCommands with
// commandWalkSkip), keep only commands with a non-nil RunE (drops
// non-runnable GROUP commands such as "spine-review", whose own bare
// invocation only ever prints help), drop any command whose own flag set
// declares the client-tier's "server" flag, then drop the small named
// exclusion set above.
//
// The "server"-flag check reads the LIVE flag set
// (cmd.Flags().Lookup("server") != nil), so every command that calls
// addClientFlags is excluded by construction and no enumeration needs
// maintaining here. That structural grounding is why this doc comment can
// state, rather than merely assert, which commands it excludes today
// without going stale the next time one is added: as of phase
// 07-console-cli-state-surfacing those commands are search, list, store,
// and get. A prior version of this comment named a fixed count of client
// verbs as the reasoning itself, which engram get (this phase) invalidated;
// the structural predicate above was already true underneath that count and
// needed no change — only the comment's own grounds did.
func operatorCommands() []*cobra.Command {
	var out []*cobra.Command
	for _, cmd := range walkCommands(rootCmd, commandWalkSkip) {
		if cmd.RunE == nil {
			continue
		}
		if cmd.Flags().Lookup("server") != nil {
			continue
		}
		if operatorCommandExclusions[commandKey(cmd)] {
			continue
		}
		out = append(out, cmd)
	}
	return out
}

func walkCommands(from *cobra.Command, skip func(*cobra.Command) bool) []*cobra.Command {
	children := from.Commands()
	sorted := make([]*cobra.Command, len(children))
	copy(sorted, children)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name() < sorted[j].Name() })

	var out []*cobra.Command
	for _, c := range sorted {
		if skip(c) {
			continue
		}
		out = append(out, c)
		out = append(out, walkCommands(c, skip)...)
	}
	return out
}
