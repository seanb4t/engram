// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/surfaces"
)

// cliNow is the only clock a destructive command's preview cutoff reads.
// Before this plan, the cutoff was computed from a bare time.Now().UTC()
// called directly inline, with no injection point — two preview runs
// straddling a second boundary could then legitimately differ, which made a
// byte-identical-re-run assertion unsatisfiable. Overriding this var in a
// test (t.Cleanup-restored) is what makes that assertion possible.
var cliNow = func() time.Time { return time.Now().UTC() }

// destructiveByClassification reports whether cmd is classified destructive
// in the internal/surfaces blast-radius table, looked up by cmd's qualified
// command path (commandKey) — never a second, hand-maintained list.
// Membership is DERIVED from the table (D-03), not declared here.
//
// A command with no classification row at all is a programming error —
// every command must carry one, the same invariant buildCatalog's own panic
// backstop enforces (catalog.go) — so this panics rather than silently
// returning false: a destructive command silently treated as non-destructive
// is exactly the failure this function exists to prevent.
func destructiveByClassification(cmd *cobra.Command) bool {
	key := commandKey(cmd)
	class, ok := surfaces.ClassForCommand(key)
	if !ok {
		panic(fmt.Sprintf(
			"destructive: command %q has no internal/surfaces blast-radius classification — "+
				"add a row to internal/surfaces/toolclass.go's operations table",
			key,
		))
	}
	return class.Destructive
}

// addApplyFlag registers the shared --apply bool flag on cmd, writing into
// target and defaulting to false (preview). Its Usage string is composed
// from the declared surfaces.RuleDestructiveRequiresApply rule's Sentence —
// referenced from the registry, never re-typed — so
// TestSurfaceConformanceCobraUsage's cobra_usage gate is satisfied by
// construction rather than by a second hand-copied string staying in sync
// with the registry by convention.
func addApplyFlag(cmd *cobra.Command, target *bool) {
	rule, ok := surfaces.RuleByID(surfaces.RuleDestructiveRequiresApply)
	if !ok {
		panic("destructive: surfaces.RuleDestructiveRequiresApply is not registered in internal/surfaces/rules.go")
	}
	cmd.Flags().BoolVar(target, "apply", false, rule.Sentence)
}

// applyRequested is a one-line pure predicate over the flag's VALUE, never
// pflag.Flag.Changed: cobra flag groups trip on a flag being SUPPLIED rather
// than on its value, and an explicit --apply=false must behave exactly like
// an omitted flag — a latch on Changed would treat the two differently.
func applyRequested(applied bool) bool {
	return applied
}

// registerDestructive is the structural choke point every destructive
// command's RunE is installed by (the fix for the prior revision's gap: a
// derived --apply flag proved PRESENCE, not runtime PREVENTION — nothing
// stopped a hand-written RunE from ignoring it). It takes the RunE away from
// the leaf entirely: a destructive command supplies a preview closure and an
// apply closure and never assigns cmd.RunE itself, so it cannot skip the
// gate by forgetting to consult the flag. There is no code path from the
// preview branch to the apply closure — the dispatch below calls exactly one
// of the two, never falls through, and never composes them.
//
// registerDestructive panics if cmd is not classified destructive: this
// helper exists ONLY for the destructive tier (surfaces.ClassForCommand's
// Destructive: true rows), and routing a non-destructive command through it
// by mistake is a programming error to catch at registration time (init()),
// not silently at runtime.
//
// The rejected alternative — an injectable classification seam. A
// package-level function variable (e.g. classForCommand, initialised to
// surfaces.ClassForCommand) that a same-package test could swap via
// t.Cleanup, letting a genuinely synthetic command name register here, was
// evaluated and REJECTED. First, it is unnecessary: an already-classified
// destructive command name proves the identical property (preview routes to
// preview, --apply routes to apply, the apply closure is unreachable
// otherwise) with no production change at all — see destructive_test.go's
// TestDestructiveGatePreventsMutation. Second, it is actively worse: a
// mutable hook deciding whether a command is destructive is precisely the
// reclassification bypass this plan's threat model forbids, and shipping it
// in production code so a test can reach it puts the escape hatch in the
// binary. A test seam must not be the one thing that can turn the safety
// gate off. Do not add such a seam here or in any later plan.
//
// preview and apply take ctx before cmd (revive's context-as-argument
// convention), unlike cobra's own RunE(cmd, args) shape — a deliberate
// divergence from cobra's convention in favor of this repo's enabled lint
// rule, which has no per-signature exception mechanism.
//
//go:noinline
func registerDestructive(
	cmd *cobra.Command,
	target *bool,
	preview func(context.Context, *cobra.Command) error,
	apply func(context.Context, *cobra.Command) error,
) {
	if !destructiveByClassification(cmd) {
		panic(fmt.Sprintf(
			"registerDestructive: command %q is not classified destructive in internal/surfaces/toolclass.go — "+
				"only a command whose blast-radius row has Destructive: true may route through this gate",
			commandKey(cmd),
		))
	}
	addApplyFlag(cmd, target)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if applyRequested(*target) {
			return apply(ctx, cmd)
		}
		return preview(ctx, cmd)
	}
}
