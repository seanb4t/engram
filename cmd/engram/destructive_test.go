// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"io"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/seanb4t/engram/internal/surfaces"
)

// destructiveCommandNames derives the set of CLI command names the
// internal/surfaces blast-radius table classifies destructive — the SAME
// derivation every test in this file uses, never a hand-typed literal set,
// so a future table edit is what changes membership, not an edit to this
// file.
func destructiveCommandNames() map[string]bool {
	out := map[string]bool{}
	for _, op := range surfaces.Operations() {
		if op.Class.Destructive && op.CLICommand != "" {
			out[op.CLICommand] = true
		}
	}
	return out
}

// applyRoutedAdditions is the NAMED set of commands that are ADDITIVE
// (Destructive:false) yet route through registerDestructive because
// previewing before mutating is the right operator contract for any write,
// destructive or not (D-16). Each entry's justification is why a
// table-derived predicate cannot express it (REVIEWS.md C4-H1): the
// blast-radius table has no "routed through registerDestructive" column,
// so membership here is a routing fact this file names explicitly,
// mirroring the operatorCommandExclusions precedent (cmdwalk.go:63-97 —
// a small named set, per-entry justification, pinned by a test).
//
//   - "migrate": the v0->v1 sweep only ever ADDS keys (migrate.CheckAdditive
//     enforces this), yet a bare invocation previews and --apply performs
//     the write — the same preview/apply contract every destructive command
//     gets, extended here to an additive one (04-03-PLAN.md Task 2).
//   - "backfill-short-ids": 04-04 Task 1 converts this alias onto the SAME
//     sweep (migrateSweepPreviewRun/migrateSweepApplyRun) and gives it its
//     own --apply flag in that same task, in the same commit that deletes
//     04-03's temporary pendingApplyConversion exclusion.
//
// migrate revert is deliberately NOT here: its toolclass row is
// Destructive:true (04-03-PLAN.md Task 3), so destructiveCommandNames()
// already returns it, and listing it again would be a stale duplicate —
// TestApplyRoutedAdditionsArePinned's Destructive:false check would catch
// that mistake.
var applyRoutedAdditions = map[string]bool{
	"migrate":            true,
	"backfill-short-ids": true,
}

// mutatingCommandNames is the `--apply`-REQUIRED set (REVIEWS.md M12 as
// corrected by C4-H1): destructiveCommandNames() (the table-derived
// Destructive:true set) UNIONED with the small named applyRoutedAdditions
// set. 04-04 Task 1 DELETED the one-wave pendingApplyConversion exclusion
// this function subtracted through the end of wave 3 — backfill-short-ids
// gained its own --apply flag in that same task, closing the window the
// exclusion existed to cover.
//
// This is DELIBERATELY NOT `!op.Class.ReadOnly && op.CLICommand != ""`. A
// prior revision defined it that way; executed against the live
// surfaces.Operations() table that predicate selects ELEVEN commands
// (backfill-short-ids, migrate-remap-owner, migrate-set-owner,
// prune-expired, reindex, serve, spine-review archive, spine-review purge,
// spine-review restore, store, summarize-missing), of which only THREE are
// Destructive:true, and --apply exists on exactly the commands routed
// through registerDestructive (prune.go:159, spine_review_purge.go:425,
// migrate.go:257, plus 04-03's migrate and 04-04's backfill-short-ids). So
// SEVEN commands (store, reindex, summarize-missing, serve,
// migrate-set-owner, spine-review archive, spine-review restore) would be
// demanded to carry --apply and have none — the --apply-routed tier is a
// ROUTING fact, and the blast-radius table has no routing column; !ReadOnly
// is a different question ("does this command write?") that happens to
// select a strictly larger set.
//
// destructiveCommandNames() STAYS as the table-derived half — this function
// is built ON it, never a replacement for it.
func mutatingCommandNames() map[string]bool {
	out := destructiveCommandNames()
	for name := range applyRoutedAdditions {
		out[name] = true
	}
	return out
}

// firstDestructiveTopLevelCommandName returns the CLICommand of the first
// declared surfaces.Operations() row that is classified destructive and
// whose CLICommand is a top-level name (no space) — i.e. resolvable by
// commandKey from a parentless *cobra.Command's bare Name(). Derived from
// the registry rather than a hardcoded command name, so
// TestDestructiveGatePreventsMutation tracks the table if a future edit
// changes which command is first.
func firstDestructiveTopLevelCommandName(t *testing.T) string {
	t.Helper()
	for _, op := range surfaces.Operations() {
		if op.Class.Destructive && op.CLICommand != "" && !strings.Contains(op.CLICommand, " ") {
			return op.CLICommand
		}
	}
	t.Fatal("surfaces.Operations(): no top-level destructive CLICommand found")
	return ""
}

// firstAdditiveRoutedTopLevelCommandName returns the lexicographically-first
// space-free member of applyRoutedAdditions whose surfaces.ClassForCommand
// row resolves and reports Destructive == false — i.e. it derives from the
// SAME named set the generalized admission gate derives from, so the
// helper and the gate cannot drift apart. A member with no row yet (e.g.
// "migrate" before its Task 2 toolclass row lands) is silently skipped
// rather than failing, which is what lets this helper resolve to
// "backfill-short-ids" at the end of Task 1 and to either name once both
// rows exist.
func firstAdditiveRoutedTopLevelCommandName(t *testing.T) string {
	t.Helper()
	var names []string
	for name := range applyRoutedAdditions {
		if strings.Contains(name, " ") {
			continue
		}
		class, ok := surfaces.ClassForCommand(name)
		if !ok || class.Destructive {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		t.Fatal("applyRoutedAdditions: no top-level, Destructive:false, classified member found")
	}
	sort.Strings(names)
	return names[0]
}

// ownFlagNames returns cmd's OWN registered flag names — cmd.Flags() minus
// anything inherited from an ancestor's PersistentFlags(), and minus "help"
// — sorted. cobra only merges an ancestor's persistent flags into a
// command's own FlagSet at Execute()-time, so cmd.Flags() alone is already
// just the command's own declarations for an un-executed tree; the explicit
// subtraction below keeps this correct even if a future ancestor hoists a
// persistent flag before Execute() runs. "help" is excluded because cobra
// registers it lazily (InitDefaultHelpFlag, called from execute()) onto a
// command's OWN FlagSet the first time ANY test in this shared-rootCmd test
// binary executes or help(s) that command — its presence here would
// therefore depend on test run order, exactly the hazard
// buildCatalogGoldenContent's stripLazyHelpFlag (catalog.go) already
// documents and works around the same way.
func ownFlagNames(cmd *cobra.Command) []string {
	inherited := map[string]bool{}
	for anc := cmd.Parent(); anc != nil; anc = anc.Parent() {
		anc.PersistentFlags().VisitAll(func(f *pflag.Flag) { inherited[f.Name] = true })
	}
	var out []string
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Name != "help" && !inherited[f.Name] {
			out = append(out, f.Name)
		}
	})
	sort.Strings(out)
	return out
}

// TestDestructiveCommandsRequireApply is D-03's derivation gate, widened by
// REVIEWS.md M12 (as corrected by C4-H1, Task 1): the set of live commands
// carrying an --apply flag must equal mutatingCommandNames() — the NAMED
// union destructiveCommandNames() ∪ applyRoutedAdditions — in BOTH
// directions, so neither a missing flag nor a stray one passes. This is
// deliberately NOT destructiveCommandNames() alone: migrate
// (Destructive:false) carries --apply too, via applyRoutedAdditions. At the
// end of 04-04 Task 1 this resolves to exactly six names — migrate, migrate
// revert, migrate-remap-owner, prune-expired, spine-review purge,
// backfill-short-ids — matching the six live registerDestructive callers
// (prune.go:159, spine_review_purge.go:425, migrate.go:257,
// migrate_family.go x2, backfill.go).
func TestDestructiveCommandsRequireApply(t *testing.T) {
	want := mutatingCommandNames()
	got := map[string]bool{}
	for _, cmd := range walkCommands(rootCmd, commandWalkSkip) {
		if cmd.Flags().Lookup("apply") != nil {
			got[commandKey(cmd)] = true
		}
	}
	for key := range want {
		if !got[key] {
			t.Errorf("mutating command %q has no --apply flag", key)
		}
	}
	for key := range got {
		if !want[key] {
			t.Errorf("command %q carries --apply but is not classified mutating (destructiveCommandNames() ∪ applyRoutedAdditions)", key)
		}
	}
}

// TestApplyRoutedAdditionsArePinned is the direct analogue of
// TestOperatorCommands' exclusion-set pinning (cmdwalk_test.go:117-130):
// every name in applyRoutedAdditions must resolve to a live
// surfaces.ClassForCommand row that is ReadOnly:false (an additive-but-
// mutating command CAN route through registerDestructive) and
// Destructive:false (a Destructive:true entry would be redundant with
// destructiveCommandNames() and therefore stale by construction — a future
// reclassification of migrate/backfill-short-ids to Destructive:true fails
// this pin loudly instead of leaving a silently duplicated name).
func TestApplyRoutedAdditionsArePinned(t *testing.T) {
	for name := range applyRoutedAdditions {
		class, ok := surfaces.ClassForCommand(name)
		if !ok {
			t.Errorf("applyRoutedAdditions names %q, which has no surfaces.ClassForCommand row", name)
			continue
		}
		if class.ReadOnly {
			t.Errorf("applyRoutedAdditions names %q, which is ReadOnly:true — it cannot be an additive-but-mutating command", name)
		}
		if class.Destructive {
			t.Errorf("applyRoutedAdditions names %q, which is Destructive:true — it is redundant with destructiveCommandNames() and stale by construction", name)
		}
	}
}

// TestMutatingCommandNamesMembership pins mutatingCommandNames()'s result
// to the exact SIX names live at the end of wave 4 (REVIEWS.md M12, INV-1):
// migrate, migrate revert, migrate-remap-owner, prune-expired,
// spine-review purge, backfill-short-ids — updated HERE, in 04-04 Task 1,
// which deletes pendingApplyConversion in the SAME task that gives
// backfill-short-ids its own --apply flag, closing the one-wave window the
// exclusion existed to cover.
//
// If this fails naming the seven UNRELATED commands the rejected !ReadOnly
// predicate would select (store, reindex, summarize-missing, serve,
// migrate-set-owner, spine-review archive, spine-review restore), the
// ENUMERATED SET here is what needs correcting, never the pin — see
// mutatingCommandNames()'s own doc comment.
func TestMutatingCommandNamesMembership(t *testing.T) {
	want := map[string]bool{
		"migrate":             true,
		"migrate revert":      true,
		"migrate-remap-owner": true,
		"prune-expired":       true,
		"spine-review purge":  true,
		"backfill-short-ids":  true,
	}
	if got := mutatingCommandNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("mutatingCommandNames() = %v, want %v", got, want)
	}
}

// TestDestructiveCommandsRouteThroughGate is the structural gate that makes
// the guard unbypassable rather than merely conventional: for every
// MUTATING command (REVIEWS.md N1 — widened from destructiveCommandNames()
// to mutatingCommandNames() in 04-04 Task 2, so the safety net covers every
// command that routes through registerDestructive, not only the
// Destructive:true subset), cmd.RunE must be the closure registerDestructive
// installs, resolved via runtime.FuncForPC.
//
// This is a SUBSTRING match against "registerDestructive", never an equality
// against the fully-qualified closure symbol (e.g. a ".funcN" suffix). Two
// independent reasons, both recorded here so neither is "tidied" into an
// equality later: (1) Go's closure-naming scheme — a numbered suffix
// appended to the enclosing function's symbol — is a compiler implementation
// detail that can change across toolchain versions for reasons unrelated to
// this safety property, and pinning it would turn a legitimate Go upgrade
// into a red gate; (2) registerDestructive carries a //go:noinline pragma
// (destructive.go) specifically to keep the installed closure's symbol name
// attributable at runtime — the substring match plus that pragma together
// are what keep this assertion durable, not an exact suffix.
//
// This test goes RED the moment a mutating command's RunE is assigned by
// hand instead of through registerDestructive — exactly the gap the prior
// revision's flag-presence-only test could not see. See the plan SUMMARY for
// the natural (not injected) RED observation this test produced before
// prune-expired and migrate-remap-owner were converted, and for the
// deliberate RED experiment 04-04 Task 2 ran against backfill-short-ids to
// prove the widened gate is non-vacuous.
func TestDestructiveCommandsRouteThroughGate(t *testing.T) {
	for key := range mutatingCommandNames() {
		key := key
		t.Run(key, func(t *testing.T) {
			var target *cobra.Command
			for _, cmd := range walkCommands(rootCmd, commandWalkSkip) {
				if commandKey(cmd) == key {
					target = cmd
					break
				}
			}
			if target == nil {
				t.Fatalf("mutating command %q not found in the live tree", key)
			}
			if target.RunE == nil {
				t.Fatalf("%s: RunE is nil, want a closure installed by registerDestructive", key)
			}
			name := runtime.FuncForPC(reflect.ValueOf(target.RunE).Pointer()).Name()
			if !strings.Contains(name, "registerDestructive") {
				t.Errorf("%s: RunE = %s, want a closure installed by registerDestructive (substring match)", key, name)
			}
		})
	}
}

// TestDestructiveGatePreventsMutation is the ONLY behavioural proof that the
// gate prevents MUTATION, rather than merely proving flag presence. It has
// two sub-tests: the original destructive-command case, and (Phase 4 D-16)
// an additive (Destructive:false) mutating command, proving the generalized
// `!ReadOnly` admission gate (destructive.go) admits BOTH shapes, not only
// the destructive one.
//
// Each sub-test's subject is a throwaway *cobra.Command carrying an
// ALREADY-CLASSIFIED Use name (derived, never hardcoded), constructed fresh
// and never attached to rootCmd or any parent. commandKey is CommandPath()
// with the root binary name trimmed, and a parentless command's
// CommandPath() is its bare Name(), so each throwaway resolves to a real
// registry key surfaces.Operations() already classifies — registerDestructive's
// panic branch is never reached, and because neither command is attached to
// the tree, this test perturbs neither the live tree, the walkers, nor
// either golden.
func TestDestructiveGatePreventsMutation(t *testing.T) {
	run := func(name string, args ...string) (previewCount, applyCount int) {
		cmd := &cobra.Command{Use: name}
		var apply bool
		registerDestructive(cmd, &apply,
			func(context.Context, *cobra.Command) error { previewCount++; return nil },
			func(context.Context, *cobra.Command) error { applyCount++; return nil },
		)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute %v: %v", args, err)
		}
		return previewCount, applyCount
	}

	t.Run("destructive command is gated", func(t *testing.T) {
		name := firstDestructiveTopLevelCommandName(t)
		if _, applyCount := run(name); applyCount != 0 {
			t.Errorf("bare run: apply closure called %d times, want 0", applyCount)
		}
		if _, applyCount := run(name, "--apply=false"); applyCount != 0 {
			t.Errorf("--apply=false: apply closure called %d times, want 0 (the gate reads the flag's VALUE, never Changed)", applyCount)
		}
		if _, applyCount := run(name, "--apply"); applyCount != 1 {
			t.Errorf("--apply: apply closure called %d times, want 1", applyCount)
		}
	})

	t.Run("additive Destructive:false command is admitted", func(t *testing.T) {
		name := firstAdditiveRoutedTopLevelCommandName(t)
		if _, applyCount := run(name); applyCount != 0 {
			t.Errorf("bare run: apply closure called %d times, want 0", applyCount)
		}
		if _, applyCount := run(name, "--apply=false"); applyCount != 0 {
			t.Errorf("--apply=false: apply closure called %d times, want 0", applyCount)
		}
		if _, applyCount := run(name, "--apply"); applyCount != 1 {
			t.Errorf("--apply: apply closure called %d times, want 1 — the generalized !ReadOnly gate must ADMIT an additive command, not only a destructive one", applyCount)
		}
	})
}

// TestApplyFlagUsageComposesRuleSentence proves every MUTATING command's
// --apply Usage string is composed from the registry's declared Sentence
// (04-04 Task 2 widens this from destructiveCommandNames() to
// mutatingCommandNames() — the sentence itself already reads "a mutating
// operator command …", so this widening is what makes the assertion cover
// every command the sentence describes), never a second hand-typed copy.
func TestApplyFlagUsageComposesRuleSentence(t *testing.T) {
	rule, ok := surfaces.RuleByID(surfaces.RuleDestructiveRequiresApply)
	if !ok {
		t.Fatal("surfaces.RuleDestructiveRequiresApply is not registered")
	}
	for key := range mutatingCommandNames() {
		for _, cmd := range walkCommands(rootCmd, commandWalkSkip) {
			if commandKey(cmd) != key {
				continue
			}
			f := cmd.Flags().Lookup("apply")
			if f == nil {
				t.Errorf("%s: no --apply flag", key)
				continue
			}
			if !strings.Contains(f.Usage, rule.Sentence) {
				t.Errorf("%s: --apply Usage = %q, want it to contain %q", key, f.Usage, rule.Sentence)
			}
		}
	}
}

// mutatingFlagCase pairs a mutating command with its expected OWN
// flag-name set. This table is the SANCTIONED place to add a row when a
// later plan registers a new mutating command — adding a row is the correct
// edit; loosening the equality this test asserts is not. Keyed by Go
// identifier (the command's own package-level *cobra.Command var), never by
// a string literal of the command's name, so this table cannot be mistaken
// for a second declared mutating-membership list (that membership is
// mutatingCommandNames(), derived from surfaces.Operations() plus the
// applyRoutedAdditions union). RENAMED from destructiveFlagCases in 04-04
// Task 2 (REVIEWS.md N1): the widening is what makes this the "no escape
// hatch exists" gate for every command routed through registerDestructive,
// not only the Destructive:true subset.
var mutatingFlagCases = []struct {
	cmd  *cobra.Command
	want []string
}{
	{pruneExpiredCmd, []string{"apply", "older-than", "output", "timeout"}},
	{migrateRemapOwnerCmd, []string{"apply", "from", "from-anon", "from-missing", "output", "timeout", "to"}},
	{spineReviewPurgeCmd, []string{"all-scopes", "apply", "category", "class", "older-than", "output", "scope", "tags", "timeout"}},
	// 04-03-PLAN.md Task 3: migrateRevertCmd's row.
	{migrateRevertCmd, []string{"apply", "output", "timeout", "to"}},
	// 04-04-PLAN.md Task 2: migrateCmd and backfillShortIDsCmd's rows —
	// both additive-but-mutating (Destructive:false) commands routed
	// through registerDestructive via applyRoutedAdditions, each carrying
	// exactly {apply, output, timeout} once registerDestructive's
	// addApplyFlag call adds "apply" to their own {output, timeout}.
	{migrateCmd, []string{"apply", "output", "timeout"}},
	{backfillShortIDsCmd, []string{"apply", "output", "timeout"}},
}

// TestDestructiveCommandsExactFlagSet is the "no escape hatch exists"
// gate, asserted by flag-set EQUALITY rather than by grepping for names
// nobody has invented yet: a bypass flag added later — whatever it is
// called — fails this equality, which a grep for a guessed vocabulary would
// not. It also asserts mutatingFlagCases covers every command
// mutatingCommandNames() names, in both directions (04-04 Task 2 widens
// this from destructiveCommandNames() — REVIEWS.md N1), so a newly
// table-classified mutating command with no row here fails loudly rather
// than silently escaping this gate.
func TestDestructiveCommandsExactFlagSet(t *testing.T) {
	covered := make(map[string]bool, len(mutatingFlagCases))
	for _, c := range mutatingFlagCases {
		key := commandKey(c.cmd)
		covered[key] = true
		want := append([]string(nil), c.want...)
		sort.Strings(want)
		got := ownFlagNames(c.cmd)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: own flags = %v, want %v", key, got, want)
		}
	}
	for key := range mutatingCommandNames() {
		if !covered[key] {
			t.Errorf("mutating command %q has no row in mutatingFlagCases — add one, do not loosen the equality", key)
		}
	}
	for key := range covered {
		if !mutatingCommandNames()[key] {
			t.Errorf("mutatingFlagCases names %q which is not currently classified mutating", key)
		}
	}
}

// engramEnvVarsFromRegistry reads internal/config/registry.go's raw source
// and extracts every ENGRAM_-prefixed Env value declared there — a real read
// of the live registry file, never a hand-duplicated list, mirroring
// internal/surfaces/conformance_test.go's exposedFileFields discipline of
// scanning the real file rather than a declared per-test list. This is what
// lets TestDestructiveModeIgnoresEnvironment track the registry as it grows.
func engramEnvVarsFromRegistry(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile("../../internal/config/registry.go")
	if err != nil {
		t.Fatalf("read internal/config/registry.go: %v", err)
	}
	matches := regexp.MustCompile(`Env:\s*"(ENGRAM_[A-Z0-9_]+)"`).FindAllStringSubmatch(string(data), -1)
	seen := make(map[string]bool, len(matches))
	var out []string
	for _, m := range matches {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	if len(out) == 0 {
		t.Fatal("engramEnvVarsFromRegistry: found zero ENGRAM_ vars — has internal/config/registry.go's shape changed?")
	}
	return out
}

// TestDestructiveModeIgnoresEnvironment proves no environment variable
// internal/config's registry defines can set the apply mode: the mode is
// resolved SOLELY from the --apply flag's value. Every ENGRAM_-prefixed key
// the live registry declares is set in the process environment before a bare
// (no --apply) run, and the preview branch must still be the one that runs.
func TestDestructiveModeIgnoresEnvironment(t *testing.T) {
	for _, v := range engramEnvVarsFromRegistry(t) {
		t.Setenv(v, "true")
	}

	name := firstDestructiveTopLevelCommandName(t)
	var previewCalled, applyCalled bool
	cmd := &cobra.Command{Use: name}
	var apply bool
	registerDestructive(cmd, &apply,
		func(context.Context, *cobra.Command) error { previewCalled = true; return nil },
		func(context.Context, *cobra.Command) error { applyCalled = true; return nil },
	)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !previewCalled || applyCalled {
		t.Errorf("previewCalled=%v applyCalled=%v, want preview only — an ENGRAM_ environment variable must never flip the destructive gate", previewCalled, applyCalled)
	}
}
