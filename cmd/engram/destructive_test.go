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
//   - "backfill-short-ids": 04-04 converts this alias onto the SAME sweep
//     (migrateSweepPreviewRun/migrateSweepApplyRun); it belongs in this set
//     from the moment the derivation exists, even though its own --apply
//     flag does not land until 04-04 (see pendingApplyConversion below).
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

// pendingApplyConversion is a NAMED, TEMPORARY exclusion (REVIEWS.md
// C4-H1/M12): backfill-short-ids is an applyRoutedAdditions member and
// therefore already in the mutating set, but its conversion to
// registerDestructive — and therefore its own --apply flag — lands in plan
// 04-04 Task 1, one wave later. This entry keeps the wave-3 tree green:
// switching TestDestructiveCommandsRequireApply onto mutatingCommandNames()
// without this exclusion would demand --apply from a command that does not
// carry it yet, by design-order, not by defect. Plan 04-04 Task 1 DELETES
// both this var and every reference to it, in the SAME task that gives
// backfill-short-ids its --apply flag.
//
// It holds exactly one name, for exactly one wave. If a second entry is
// ever added here to make a gate pass, the DERIVATION is wrong, not the
// exclusion list — that is precisely how the rejected !ReadOnly predicate
// (C4-H1) stayed invisible for three review cycles.
var pendingApplyConversion = map[string]bool{
	"backfill-short-ids": true,
}

// mutatingCommandNames is the `--apply`-REQUIRED set (REVIEWS.md M12 as
// corrected by C4-H1): destructiveCommandNames() (the table-derived
// Destructive:true set) UNIONED with the small named applyRoutedAdditions
// set, MINUS the one-wave pendingApplyConversion exclusion.
//
// This is DELIBERATELY NOT `!op.Class.ReadOnly && op.CLICommand != ""`. A
// prior revision defined it that way; executed against the live
// surfaces.Operations() table that predicate selects ELEVEN commands
// (backfill-short-ids, migrate-remap-owner, migrate-set-owner,
// prune-expired, reindex, serve, spine-review archive, spine-review purge,
// spine-review restore, store, summarize-missing), of which only THREE are
// Destructive:true, and --apply exists on exactly the commands routed
// through registerDestructive (prune.go:159, spine_review_purge.go:425,
// migrate.go:257, plus 04-03's migrate). So SEVEN commands (store, reindex,
// summarize-missing, serve, migrate-set-owner, spine-review archive,
// spine-review restore) would be demanded to carry --apply and have none —
// the --apply-routed tier is a ROUTING fact, and the blast-radius table has
// no routing column; !ReadOnly is a different question ("does this command
// write?") that happens to select a strictly larger set.
//
// destructiveCommandNames() STAYS as the table-derived half — this function
// is built ON it, never a replacement for it.
func mutatingCommandNames() map[string]bool {
	out := destructiveCommandNames()
	for name := range applyRoutedAdditions {
		out[name] = true
	}
	for name := range pendingApplyConversion {
		delete(out, name)
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
// union destructiveCommandNames() ∪ applyRoutedAdditions −
// pendingApplyConversion — in BOTH directions, so neither a missing flag
// nor a stray one passes. This is deliberately NOT destructiveCommandNames()
// alone: migrate (Destructive:false) carries --apply too, via
// applyRoutedAdditions. At the end of Task 2 this resolves to exactly four
// names — migrate, migrate-remap-owner, prune-expired, spine-review purge
// — matching the four live registerDestructive callers (prune.go:159,
// spine_review_purge.go:425, migrate.go:257, migrate_family.go); Task 3
// adds the fifth (migrate revert) on both sides in one edit.
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
			t.Errorf("command %q carries --apply but is not classified mutating (destructiveCommandNames() ∪ applyRoutedAdditions − pendingApplyConversion)", key)
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

// TestDestructiveCommandsRouteThroughGate is the structural gate that makes
// the guard unbypassable rather than merely conventional: for every
// table-derived destructive command, cmd.RunE must be the closure
// registerDestructive installs, resolved via runtime.FuncForPC.
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
// This test goes RED the moment a destructive command's RunE is assigned by
// hand instead of through registerDestructive — exactly the gap the prior
// revision's flag-presence-only test could not see. See the plan SUMMARY for
// the natural (not injected) RED observation this test produced before
// prune-expired and migrate-remap-owner were converted.
func TestDestructiveCommandsRouteThroughGate(t *testing.T) {
	for key := range destructiveCommandNames() {
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
				t.Fatalf("destructive command %q not found in the live tree", key)
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

// TestApplyFlagUsageComposesRuleSentence proves every destructive command's
// --apply Usage string is composed from the registry's declared Sentence,
// never a second hand-typed copy.
func TestApplyFlagUsageComposesRuleSentence(t *testing.T) {
	rule, ok := surfaces.RuleByID(surfaces.RuleDestructiveRequiresApply)
	if !ok {
		t.Fatal("surfaces.RuleDestructiveRequiresApply is not registered")
	}
	for key := range destructiveCommandNames() {
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

// destructiveFlagCase pairs a destructive command with its expected OWN
// flag-name set. This table is the SANCTIONED place to add a row when a
// later plan registers a new destructive command (03-07's spine-review
// purge; 03-06 may add archive/restore rows) — adding a row is the correct
// edit; loosening the equality this test asserts is not. Keyed by Go
// identifier (the command's own package-level *cobra.Command var), never by
// a string literal of the command's name, so this table cannot be mistaken
// for a second declared destructive-membership list (that membership is
// destructiveCommandNames(), derived from surfaces.Operations() alone).
var destructiveFlagCases = []struct {
	cmd  *cobra.Command
	want []string
}{
	{pruneExpiredCmd, []string{"apply", "older-than", "output", "timeout"}},
	{migrateRemapOwnerCmd, []string{"apply", "from", "from-anon", "from-missing", "output", "timeout", "to"}},
	{spineReviewPurgeCmd, []string{"all-scopes", "apply", "category", "class", "older-than", "output", "scope", "tags", "timeout"}},
}

// TestDestructiveCommandsExactFlagSet is the "no escape hatch exists"
// gate, asserted by flag-set EQUALITY rather than by grepping for names
// nobody has invented yet: a bypass flag added later — whatever it is
// called — fails this equality, which a grep for a guessed vocabulary would
// not. It also asserts destructiveFlagCases covers every command
// destructiveCommandNames() names, in both directions, so a newly
// table-classified destructive command with no row here fails loudly rather
// than silently escaping this gate.
func TestDestructiveCommandsExactFlagSet(t *testing.T) {
	covered := make(map[string]bool, len(destructiveFlagCases))
	for _, c := range destructiveFlagCases {
		key := commandKey(c.cmd)
		covered[key] = true
		want := append([]string(nil), c.want...)
		sort.Strings(want)
		got := ownFlagNames(c.cmd)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: own flags = %v, want %v", key, got, want)
		}
	}
	for key := range destructiveCommandNames() {
		if !covered[key] {
			t.Errorf("destructive command %q has no row in destructiveFlagCases — add one, do not loosen the equality", key)
		}
	}
	for key := range covered {
		if !destructiveCommandNames()[key] {
			t.Errorf("destructiveFlagCases names %q which is not currently classified destructive", key)
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
