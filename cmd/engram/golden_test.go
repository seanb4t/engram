// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// updateGolden regenerates cmd/engram/testdata/help.golden and
// cmd/engram/testdata/catalog.golden from the live cobra tree when passed,
// following the standard Go golden-file idiom (`go test ./cmd/engram
// -update`). D-14/T-02-19: regeneration must never be a side effect of an
// ordinary test run — only `task surfaces:gen` (and the CI `surfaces` job
// that re-runs it in a throwaway checkout) ever passes -update; neither
// `task default`, `task test`, nor `task lint` does.
var updateGolden = flag.Bool("update", false, "regenerate golden files (cmd/engram/testdata/*.golden)")

// goldenTestVersion replaces the ldflags-injected main.version for the
// duration of golden generation. Without this, catalogDoc.Version (set from
// root.Version, itself set from main.version at root.go:19,71) would bake
// whatever version the local build happened to carry into a committed
// fixture, breaking the catalog golden on every release retag for a reason
// unrelated to any actual interface drift (RESEARCH.md Pitfall 5).
const goldenTestVersion = "test-version"

// envDerivedFlagDefaults names the exact (command, flag) pairs whose pflag
// default is computed from os.Getenv(...) at package init() time, rather
// than a fixed literal or a value routed through internal/config's static
// registry (config.FlagDefault, confirmed a pure map lookup — CONTEXT.md
// D-12's determinism basis). Found by this plan's required sweep
// (`rg -n 'os\.Getenv' cmd/engram/*.go`, recorded in the SUMMARY): three
// hits total, of which only these two feed a flag's DEFAULT VALUE (the
// third, client_common.go's resolveToken, reads ENGRAM_TOKEN at RUNTIME to
// resolve a credential and never touches a flag registration or Usage
// string at all).
//
//   - reindexCmd's --target: reindex.go's
//     `StringVar(&reindexTarget, "target", os.Getenv("ENGRAM_REINDEX_TARGET"), ...)`
//   - migrateSetOwnerCmd's --owner: migrate.go's
//     `StringVar(&migrateOwner, "owner", os.Getenv("ENGRAM_MIGRATE_OWNER"), ...)`
//
// init() runs once for the whole test binary, before any test's t.Setenv
// could take effect — so a contributor who happens to have either variable
// set in their shell when running `go test ./cmd/engram -update` would
// otherwise silently bake their own local value into the committed golden.
// Confirmed live this session: `ENGRAM_REINDEX_TARGET=my_target go run
// ./cmd/engram reindex --help` prints `--target string ... (default
// "my_target")`, proving the hazard is real, not theoretical. The fix
// normalizes each named flag's pflag.Flag.DefValue (the field both
// cmd.Help()'s "(default ...)" text and catalogFlag.Default read) to a
// fixed, env-independent placeholder for the duration of golden generation
// — never the environment the test happens to run in.
var envDerivedFlagDefaults = map[string]map[string]bool{
	"reindex":           {"target": true},
	"migrate-set-owner": {"owner": true},
}

// withGoldenDeterminism pins rootCmd.Version to goldenTestVersion and blanks
// every flag named in envDerivedFlagDefaults back to its committed,
// env-independent default, restoring both via t.Cleanup. Every golden must
// be generated under this, so neither a release version nor a
// contributor's local environment can perturb it.
func withGoldenDeterminism(t *testing.T) {
	t.Helper()

	origVersion := rootCmd.Version
	rootCmd.Version = goldenTestVersion
	t.Cleanup(func() { rootCmd.Version = origVersion })

	for _, cmd := range walkCommands(rootCmd, commandWalkSkip) {
		flagNames, ok := envDerivedFlagDefaults[commandKey(cmd)]
		if !ok {
			continue
		}
		for flagName := range flagNames {
			f := cmd.Flags().Lookup(flagName)
			if f == nil {
				continue
			}
			orig := f.DefValue
			f.DefValue = ""
			t.Cleanup(func(f *pflag.Flag, orig string) func() {
				return func() { f.DefValue = orig }
			}(f, orig))
		}
	}
}

// goldenCommands returns EVERY non-hidden, non-cobra-scaffolding command in
// root's whole tree, at any depth, in walkCommands' deterministic pre-order
// (parent immediately followed by its own children) — the SAME shared
// walker and skip predicate buildCatalog uses (cmdwalk.go), so the help
// golden and the catalog golden can only ever cover the identical command
// set, now including nested leaves such as "spine-review scan".
func goldenCommands(root *cobra.Command) []*cobra.Command {
	return walkCommands(root, commandWalkSkip)
}

// buildHelpGoldenContent captures --help output for every command
// goldenCommands returns, keyed under a `## <full command path>` section
// header (cobra's own CommandPath(), e.g. "engram search") rather than by
// help text — so two commands with byte-identical help text still occupy
// distinct, individually-diffable sections, and a command with zero
// non-hidden flags of its own (every command inherits -h/--help, so this
// never renders empty) still gets a section. Flags print in cobra/pflag's
// own default sorted order (pflag.FlagSet.SortFlags defaults to true and is
// never overridden in this binary — confirmed by `rg -n SortFlags
// cmd/engram/*.go` returning no match).
func buildHelpGoldenContent(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	// The ROOT command's own --help is pinned too, ahead of the subcommands.
	// `engram --help` is the first teaching surface any caller meets, and it
	// is the one place rootCmd.Short/Long render — neither of which appears
	// anywhere in catalog.golden, whose command list covers subcommands only.
	// goldenCommands is deliberately left alone: it mirrors buildCatalog's
	// skip predicate so the two goldens cover an identical SUBCOMMAND set,
	// and widening it would break that parity. Prepending the root here adds
	// the missing surface without disturbing it.
	//
	// The root needs THREE more deterministic registrations than a subcommand
	// does, for the same lazy-init reason InitDefaultHelpFlag is called below:
	// cobra adds -v/--version (InitDefaultVersionFlag) and the `help` and
	// `completion` subcommands (InitDefaultHelpCmd / InitDefaultCompletionCmd)
	// from inside execute(). Because the root's help LISTS its children,
	// whether some earlier test in this shared-rootCmd binary happened to
	// Execute() anything decided whether `help`/`completion`/`--version`
	// appeared — which made this golden flap across test orderings (observed
	// live: 3 of 4 -shuffle seeds failed before these calls were added).
	// Forcing them reproduces what a real `engram --help` always prints.
	rootCmd.InitDefaultVersionFlag()
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()

	for _, cmd := range append([]*cobra.Command{rootCmd}, goldenCommands(rootCmd)...) {
		// cobra only lazily registers -h/--help on a command's OWN FlagSet
		// from inside execute() (command.go InitDefaultHelpFlag, called
		// right before ParseFlags) — cmd.Help() called directly, as below,
		// bypasses execute() entirely and does NOT register it. A real
		// interactive `engram <cmd> --help` invocation always goes through
		// execute() first, so it always shows "-h, --help"; calling
		// InitDefaultHelpFlag() explicitly here reproduces that same,
		// real, user-facing guarantee deterministically, regardless of
		// whether some EARLIER test in this shared-rootCmd test binary
		// happened to Execute() this specific command first (confirmed
		// live this session: in isolation this flag is silently absent
		// without this call, flapping the golden by test run/subset).
		cmd.InitDefaultHelpFlag()

		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		if err := cmd.Help(); err != nil {
			t.Fatalf("cmd.Help() for %q: %v", cmd.Name(), err)
		}
		cmd.SetOut(nil)
		cmd.SetErr(nil)

		b.WriteString("## " + cmd.CommandPath() + "\n\n")
		b.WriteString("```\n")
		text := buf.String()
		b.WriteString(text)
		if !strings.HasSuffix(text, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("```\n\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// buildCatalogGoldenContent builds the catalog document through the
// existing buildCatalog derivation (never a second traversal or encoder)
// and marshals it with a stable, human-diffable indent — decodeCatalog's
// own json.NewEncoder path stays compact-single-line for runtime
// consumption, so this is a rendering choice, not a second derivation.
//
// stripLazyHelpFlag removes any catalogFlag literally named "help" from
// every command before marshaling. This is NOT a normal flag that should
// ever appear in the bare-invocation catalog: cobra only lazily registers
// -h/--help on a SUBCOMMAND's own FlagSet from inside that subcommand's
// OWN execute() (see buildHelpGoldenContent's comment) — a bare `engram`
// invocation (runSelfDescribe) only ever executes root, never any
// subcommand, so a genuinely fresh `engram` process's catalog JSON NEVER
// carries a "help" flag on any subcommand (confirmed live this session: a
// freshly built binary's bare invocation omits it for every command).
// rootCmd and every subcommand are package-level singletons SHARED across
// this whole test binary, though, so an EARLIER test that ran e.g.
// `runClient(t, "search", ...)` permanently leaves "-h, --help" registered
// on searchCmd's FlagSet for the rest of the process — without this strip,
// the golden would flap depending on test run/subset, showing a flag that
// a real fresh bare invocation never emits.
func buildCatalogGoldenContent(t *testing.T) string {
	t.Helper()
	doc := buildCatalog(rootCmd)
	for i := range doc.Commands {
		kept := doc.Commands[i].Flags[:0]
		for _, f := range doc.Commands[i].Flags {
			if f.Name == "help" {
				continue
			}
			kept = append(kept, f)
		}
		doc.Commands[i].Flags = kept
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent(catalogDoc): %v", err)
	}
	return string(b) + "\n"
}

// writeGoldenFileAtomic writes content to path via a same-directory temp
// file and os.Rename — the same atomic contract
// internal/surfaces.WriteRegion's own writeFileAtomic uses (create temp in
// the target directory, write, close, rename) — so an interrupted or
// concurrently-run `-update` invocation leaves either the previous or the
// new content, never a truncated golden (T-02-21).
func writeGoldenFileAtomic(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".golden-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanupOnErr := true
	defer func() {
		if cleanupOnErr {
			_ = os.Remove(tmpName)
		}
	}()
	if _, werr := tmp.WriteString(content); werr != nil {
		_ = tmp.Close()
		return werr
	}
	if cerr := tmp.Close(); cerr != nil {
		return cerr
	}
	if rerr := os.Rename(tmpName, path); rerr != nil {
		return rerr
	}
	cleanupOnErr = false
	return nil
}

// checkGolden is the shared -update/compare body for both golden tests: it
// proves in-process determinism (generating twice must produce identical
// bytes, so map-iteration order or an unsorted walk cannot produce a
// flapping fixture), then either regenerates path (under -update) or
// compares against the committed content.
func checkGolden(t *testing.T, path string, build func(t *testing.T) string) {
	t.Helper()

	got1 := build(t)
	got2 := build(t)
	if got1 != got2 {
		t.Fatalf("golden generation for %s is not deterministic across two in-process runs", path)
	}

	if *updateGolden {
		if err := writeGoldenFileAtomic(path, got1); err != nil {
			t.Fatalf("writing golden %s: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden %s: %v (run `task surfaces:gen` to generate it)", path, err)
	}
	if got1 != string(want) {
		t.Errorf("%s has drifted from the live tree — run `task surfaces:gen` and review the diff\n--- got ---\n%s\n--- want (committed) ---\n%s", path, got1, string(want))
	}
}

// TestHelpGolden is D-12's gate: every command's --help output, walked from
// the live cobra tree, is pinned by cmd/engram/testdata/help.golden.
func TestHelpGolden(t *testing.T) {
	// cmd.Help() (called by buildHelpGoldenContent below) renders straight
	// through the help template — it never parses flags or touches the
	// "help" flag's own Changed latch, so unlike TestHelpFlagStillPrintsHumanHelp
	// (which drives --help through actual flag parsing) no resetHelpFlag
	// call is needed here. resetClientFlags still resets every OTHER flag's
	// value/Changed state left over from an earlier test in this package.
	resetClientFlags(t)
	withGoldenDeterminism(t)

	checkGolden(t, filepath.Join("testdata", "help.golden"), buildHelpGoldenContent)
}

// TestCatalogGolden is D-13's gate: the bare `engram` catalog JSON,
// including D-11's blast-radius classification, is pinned by
// cmd/engram/testdata/catalog.golden.
func TestCatalogGolden(t *testing.T) {
	resetClientFlags(t)
	withGoldenDeterminism(t)

	checkGolden(t, filepath.Join("testdata", "catalog.golden"), buildCatalogGoldenContent)
}
