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

// TestDestructiveCommandsRequireApply is D-03's derivation gate: the set of
// live commands carrying an --apply flag must equal the set
// surfaces.Operations() classifies destructive, in BOTH directions, so
// neither a missing flag nor a stray one passes.
func TestDestructiveCommandsRequireApply(t *testing.T) {
	want := destructiveCommandNames()
	got := map[string]bool{}
	for _, cmd := range walkCommands(rootCmd, commandWalkSkip) {
		if cmd.Flags().Lookup("apply") != nil {
			got[commandKey(cmd)] = true
		}
	}
	for key := range want {
		if !got[key] {
			t.Errorf("destructive command %q has no --apply flag", key)
		}
	}
	for key := range got {
		if !want[key] {
			t.Errorf("command %q carries --apply but is not classified destructive", key)
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
// gate prevents MUTATION, rather than merely proving flag presence.
//
// Its subject is a throwaway *cobra.Command carrying an ALREADY-CLASSIFIED
// destructive Use name (derived via firstDestructiveTopLevelCommandName,
// never hardcoded), constructed fresh and never attached to rootCmd or any
// parent. commandKey is CommandPath() with the root binary name trimmed, and
// a parentless command's CommandPath() is its bare Name(), so the throwaway
// resolves to a real registry key that surfaces.Operations() already
// classifies Destructive: true — registerDestructive's panic branch is never
// reached, and because the command is never attached to the tree, this test
// perturbs neither the live tree, the walkers, nor either golden.
func TestDestructiveGatePreventsMutation(t *testing.T) {
	name := firstDestructiveTopLevelCommandName(t)

	run := func(args ...string) (previewCount, applyCount int) {
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

	if _, applyCount := run(); applyCount != 0 {
		t.Errorf("bare run: apply closure called %d times, want 0", applyCount)
	}
	if _, applyCount := run("--apply=false"); applyCount != 0 {
		t.Errorf("--apply=false: apply closure called %d times, want 0 (the gate reads the flag's VALUE, never Changed)", applyCount)
	}
	if _, applyCount := run("--apply"); applyCount != 1 {
		t.Errorf("--apply: apply closure called %d times, want 1", applyCount)
	}
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
