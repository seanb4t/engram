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

// destructiveByClassification reports whether cmd is classified as a
// MUTATING operator command in the internal/surfaces blast-radius table
// (Class.ReadOnly == false), looked up by cmd's qualified command path
// (commandKey) — never a second, hand-maintained list. Membership is
// DERIVED from the table (D-03), not declared here.
//
// The predicate is `!class.ReadOnly`, not `class.Destructive` (Phase 4's
// D-16 generalization): the gate this function feeds,
// registerDestructive's ADMISSION check, answers "may this command route
// through the preview/apply choke point?" — and an additive-but-mutating
// command (e.g. `migrate`, Destructive:false) deserves that same
// preview-by-default contract as a genuinely destructive one. This is a
// DIFFERENT question from "which commands MUST carry --apply?" — that
// narrower, --apply-REQUIRED set lives in destructive_test.go's
// mutatingCommandNames(), a NAMED union, never this predicate (REVIEWS.md
// C4-H1 — see that function's doc comment for the rejected alternative and
// why it is wrong).
//
// M9 (accepted name debt): this function's name, and registerDestructive's
// panic message below, predate the gate's generalization from
// Destructive:true to !ReadOnly. "destructive" now means "any mutating
// operation" throughout this file's identifiers — not exclusively
// key-removing ones — kept for backward compatibility with every existing
// caller site rather than renamed in this phase.
//
// A command with no classification row at all is a programming error —
// every command must carry one, the same invariant buildCatalog's own panic
// backstop enforces (catalog.go) — so this panics rather than silently
// returning false: a mutating command silently treated as read-only is
// exactly the failure this function exists to prevent.
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
	return !class.ReadOnly
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

// registerDestructive is the structural choke point every mutating operator
// command's RunE is installed by (the fix for the prior revision's gap: a
// derived --apply flag proved PRESENCE, not runtime PREVENTION — nothing
// stopped a hand-written RunE from ignoring it). It takes the RunE away from
// the leaf entirely: a command supplies a preview closure and an apply
// closure and never assigns cmd.RunE itself, so it cannot skip the gate by
// forgetting to consult the flag. There is no code path from the preview
// branch to the apply closure — the dispatch below calls exactly one of the
// two, never falls through, and never composes them.
//
// registerDestructive panics if cmd is not classified as mutating
// (destructiveByClassification, `!class.ReadOnly` — Phase 4's D-16
// generalization from the original Destructive:true-only gate): this helper
// exists for every write-capable operator command, additive or destructive
// alike, and routing a ReadOnly command through it by mistake is a
// programming error to catch at registration time (init()), not silently at
// runtime. The name retains its original "destructive" wording as an
// accepted debt (M9) — see destructiveByClassification's doc comment.
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
			"registerDestructive: command %q is not classified as a mutating operation in internal/surfaces/toolclass.go — "+
				"only a command whose blast-radius row has ReadOnly: false may route through this gate "+
				"(additive mutations included, not only Destructive: true ones — REVIEWS.md M9)",
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
