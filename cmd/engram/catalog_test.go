// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/seanb4t/engram/internal/surfaces"
)

// noArgs is a non-nil empty argument slice. runClient's own doc comment
// warns that passing no variadic arguments at all yields a nil slice, which
// cobra then falls back to os.Args[1:] for — under `go test` that is the
// test binary's own flags, not a bare invocation. Spreading this explicit
// empty slice keeps args non-nil.
var noArgs = []string{}

// isJSON reports whether s unmarshals as a single JSON value.
func isJSON(s string) bool {
	var v any
	return json.Unmarshal([]byte(s), &v) == nil
}

// resetHelpFlag clears cmd's own "help" flag back to false after the test.
// Cobra's --help flag is bound directly to pflag FlagSet storage, not
// re-parsed on every Execute() call — so leaving it "true" after one
// runClient(t, "--help") invocation would silently make a LATER bare
// invocation, or a later unrelated test's invocation, on the same
// process-lifetime shared command render help text instead of its real
// output (this file is the first in the package to exercise --help at all,
// so this leak is new surface this file must close itself).
func resetHelpFlag(t *testing.T, cmd *cobra.Command) {
	t.Helper()
	t.Cleanup(func() {
		if f := cmd.Flags().Lookup("help"); f != nil {
			_ = f.Value.Set("false")
			f.Changed = false
		}
	})
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedIntKeys(m map[int]bool) string {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = strconv.Itoa(k)
	}
	return strings.Join(parts, ",")
}

// decodedCatalog is the test-side decode shape mirroring catalogDoc's JSON
// tags exactly.
type decodedCatalog struct {
	Binary   string `json:"binary"`
	Version  string `json:"version"`
	Commands []struct {
		Name    string `json:"name"`
		Summary string `json:"summary"`
		Flags   []struct {
			Name    string `json:"name"`
			Type    string `json:"type"`
			Default string `json:"default"`
			Usage   string `json:"usage"`
		} `json:"flags"`
		BlastRadius struct {
			ReadOnly    bool `json:"read_only"`
			Destructive bool `json:"destructive"`
			Idempotent  bool `json:"idempotent"`
			OpenWorld   bool `json:"open_world"`
		} `json:"blast_radius"`
	} `json:"commands"`
	ExitCodes []struct {
		Code    int    `json:"code"`
		Meaning string `json:"meaning"`
	} `json:"exit_codes"`
	Notes []string `json:"notes"`
}

// decodeCatalog runs a bare invocation and decodes the whole of stdout as a
// single JSON object, failing the test on any parse error rather than a
// substring check — a substring check would pass even if help text were
// prepended ahead of the JSON payload.
func decodeCatalog(t *testing.T) decodedCatalog {
	t.Helper()
	resetClientFlags(t)
	stdout, _, err := runClient(t, noArgs...)
	if err != nil {
		t.Fatalf("bare invocation returned error: %v", err)
	}
	var doc decodedCatalog
	if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
		t.Fatalf("stdout did not unmarshal in its entirety as one JSON object: %v\nstdout=%q", uErr, stdout)
	}
	return doc
}

// wantCommandNames returns the set of names buildCatalog is expected to
// enumerate: rootCmd's non-hidden subcommands, excluding cobra's own
// auto-generated help and completion commands. This mirrors buildCatalog's
// own filter exactly, so the two can only diverge if buildCatalog's actual
// derivation itself drifts from the live tree.
func wantCommandNames(t *testing.T) map[string]bool {
	t.Helper()
	want := make(map[string]bool)
	for _, cmd := range rootCmd.Commands() {
		if cmd.Hidden || cmd.Name() == "help" || cmd.Name() == "completion" {
			continue
		}
		want[cmd.Name()] = true
	}
	return want
}

// TestRootBareInvocationEmitsCatalog is D-15's core gate: a bare
// invocation writes one JSON document to stdout and exits 0, with stderr
// empty.
func TestRootBareInvocationEmitsCatalog(t *testing.T) {
	resetClientFlags(t)
	stdout, stderr, err := runClient(t, noArgs...)
	if err != nil {
		t.Fatalf("bare invocation returned error: %v", err)
	}
	var doc map[string]any
	if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
		t.Fatalf("stdout did not unmarshal in its entirety as one JSON object: %v\nstdout=%q", uErr, stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

// TestCatalogEnumeratesEveryCommand asserts set equality (not membership)
// between the catalog's command names and the live tree's non-hidden
// subcommands, plus an explicit three-verb membership control so the test
// cannot pass by comparing two empty sets.
func TestCatalogEnumeratesEveryCommand(t *testing.T) {
	doc := decodeCatalog(t)

	got := make(map[string]bool)
	for _, c := range doc.Commands {
		got[c.Name] = true
	}
	want := wantCommandNames(t)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("catalog commands = %v, want %v", sortedKeys(got), sortedKeys(want))
	}

	for _, verb := range []string{"search", "list", "store"} {
		if !got[verb] {
			t.Errorf("catalog missing command %q", verb)
		}
	}
}

// TestCatalogEnumeratesEveryFlag asserts, per command, set equality between
// the catalog's flag names and the flags actually collected by walking that
// command's own flags plus root's persistent flags — the same derivation
// buildCatalog itself performs, so this test can only diverge from
// buildCatalog's true output if the live tree changes underneath it.
func TestCatalogEnumeratesEveryFlag(t *testing.T) {
	doc := decodeCatalog(t)

	byName := make(map[string][]struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Default string `json:"default"`
		Usage   string `json:"usage"`
	})
	for _, c := range doc.Commands {
		byName[c.Name] = c.Flags
	}

	for _, verb := range []string{"search", "list", "store"} {
		cmd, _, err := rootCmd.Find([]string{verb})
		if err != nil {
			t.Fatalf("rootCmd.Find(%q): %v", verb, err)
		}
		want := make(map[string]bool)
		cmd.Flags().VisitAll(func(f *pflag.Flag) { want[f.Name] = true })
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { want[f.Name] = true })

		got := make(map[string]bool)
		for _, f := range byName[verb] {
			got[f.Name] = true
			if f.Type == "" {
				t.Errorf("%s flag %q has empty type", verb, f.Name)
			}
			if f.Usage == "" {
				t.Errorf("%s flag %q has empty usage", verb, f.Name)
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s catalog flags = %v, want %v", verb, sortedKeys(got), sortedKeys(want))
		}
	}
}

// wantExitCodes is every exit code this binary's constants declare —
// deriving TestCatalogListsEveryExitCode's bounds and membership check from
// this slice, rather than a second set of hand-written integer literals, is
// what makes the next code addition (after D-06's exitTimeout) not repeat
// the drift TestCatalogListsEveryExitCode's own hard-coded "6"/"0-5"
// literals caused when exitTimeout was added.
var wantExitCodes = []int{exitOK, exitGeneric, exitUsage, exitAuth, exitNotFound, exitUnavailable, exitTimeout}

// TestCatalogListsEveryExitCode asserts the catalog carries exactly one
// entry per constant in wantExitCodes, with no duplicates and no code
// outside that set's min/max range, each with a non-empty meaning.
func TestCatalogListsEveryExitCode(t *testing.T) {
	doc := decodeCatalog(t)

	minCode, maxCode := wantExitCodes[0], wantExitCodes[0]
	for _, c := range wantExitCodes {
		if c < minCode {
			minCode = c
		}
		if c > maxCode {
			maxCode = c
		}
	}

	if len(doc.ExitCodes) != len(wantExitCodes) {
		t.Fatalf("len(exit_codes) = %d, want %d", len(doc.ExitCodes), len(wantExitCodes))
	}
	seen := make(map[int]bool)
	for _, ec := range doc.ExitCodes {
		if ec.Code < minCode || ec.Code > maxCode {
			t.Errorf("exit code %d is outside the %d-%d range", ec.Code, minCode, maxCode)
		}
		if seen[ec.Code] {
			t.Errorf("duplicate exit code %d", ec.Code)
		}
		seen[ec.Code] = true
		if ec.Meaning == "" {
			t.Errorf("exit code %d has an empty meaning", ec.Code)
		}
	}
	for _, c := range wantExitCodes {
		if !seen[c] {
			t.Errorf("exit code %d missing from the catalog", c)
		}
	}
}

// TestRootUnknownSubcommandStillErrors is the landmine guard: making the
// root runnable must not turn a typo'd verb into a silent exit-0 success.
// Before trusting this test, cobra.NoArgs was temporarily removed from
// rootCmd's literal and this test was observed to fail — see the SUMMARY
// for the quoted FAIL line.
//
// The accepted, deliberate gap this test also pins: a mistyped verb stays
// on exitGeneric (never reclassified to exitUsage), because cobra's Find()
// fails inside ExecuteC() before execute() runs, so PersistentPreRunE never
// sees it and no sentinel exists to classify it without matching the
// message text — which this plan's prohibitions forbid. This mirrors the
// D-09 baseline's root/unknown-subcommand row (before == after == exitGeneric,
// changes: false).
func TestRootUnknownSubcommandStillErrors(t *testing.T) {
	resetClientFlags(t)
	stdout, _, err := runClient(t, "definitely-not-a-verb")
	if err == nil {
		t.Fatal("expected an error for an unknown subcommand, got nil")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("error = %q, want it to name an unknown command", err.Error())
	}
	if isJSON(stdout) {
		t.Errorf("stdout unexpectedly parsed as JSON: %q", stdout)
	}
	// exitCodeFromError, not assertExitCode: a mistyped verb's error is a
	// bare cobra error with no ExitCode() method, which is exactly the
	// untyped-fallback case exitCodeFromError exists to cover — assertExitCode
	// would abort the test the moment it found no ExitCode() method.
	if got := exitCodeFromError(err); got != exitGeneric {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitGeneric)", got, exitGeneric)
	}
}

// TestRootExistingSubcommandsStillResolve confirms making the root runnable
// does not disturb a pre-existing operator command's resolution.
func TestRootExistingSubcommandsStillResolve(t *testing.T) {
	resetClientFlags(t)
	stdout, _, err := runClient(t, "version")
	if err != nil {
		t.Fatalf("engram version returned error: %v", err)
	}
	if isJSON(stdout) {
		t.Errorf("stdout unexpectedly parsed as JSON: %q", stdout)
	}
}

// TestCatalogGoesToStdoutNotStderr is D-07's gate on the self-describe
// path specifically: stderr is empty and stdout is the whole document.
func TestCatalogGoesToStdoutNotStderr(t *testing.T) {
	resetClientFlags(t)
	stdout, stderr, err := runClient(t, noArgs...)
	if err != nil {
		t.Fatalf("bare invocation returned error: %v", err)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if !isJSON(stdout) {
		t.Errorf("stdout did not parse as JSON: %q", stdout)
	}
}

// TestCatalogExitCodesMatchMapper is the D-11 anti-drift gate: the set of
// exit codes the catalog advertises must equal, as a set, the set of exit
// codes exitCodeForConnectErr can actually produce across every
// connect.Code plus the not-a-connect-error case, unioned with exitOK for
// the success path. Set equality carries both directions — a code the
// mapper can return but the catalog omits fails, and a code the catalog
// invents but the mapper never returns fails too.
//
// Both directions of this gate were observed failing before being trusted:
// see the SUMMARY for the quoted FAIL lines from (1) a temporary seventh
// catalog entry the mapper never produces, and (2) a temporary mapper arm
// returning a code absent from the catalog.
func TestCatalogExitCodesMatchMapper(t *testing.T) {
	doc := decodeCatalog(t)

	catalogCodes := make(map[int]bool)
	for _, ec := range doc.ExitCodes {
		catalogCodes[ec.Code] = true
	}

	mapperCodes := map[int]bool{exitOK: true}
	for i := 1; i <= 16; i++ {
		mapperCodes[exitCodeForConnectErr(connect.NewError(connect.Code(i), errors.New("boom")))] = true
	}
	mapperCodes[exitCodeForConnectErr(errors.New("not a connect error"))] = true

	if !reflect.DeepEqual(catalogCodes, mapperCodes) {
		t.Errorf("catalog exit codes = {%s}, mapper-producible exit codes = {%s}",
			sortedIntKeys(catalogCodes), sortedIntKeys(mapperCodes))
	}
}

// TestCatalogBlastRadiusMatchesToolClasses is D-11's both-directions gate,
// copying TestCatalogExitCodesMatchMapper's shape: the catalog's emitted
// command names and internal/surfaces.Operations()'s non-empty CLICommand
// values must be the EXACT same set — a command the catalog emits but the
// table never classified fails here, and a table row naming a command the
// catalog never emits fails here too. A second loop then asserts every
// catalog command's rendered classification equals
// surfaces.ClassForCommand(name)'s value for that name, so the two lanes
// cannot silently disagree about a command they both classify.
func TestCatalogBlastRadiusMatchesToolClasses(t *testing.T) {
	doc := decodeCatalog(t)

	catalogNames := make(map[string]bool, len(doc.Commands))
	for _, c := range doc.Commands {
		catalogNames[c.Name] = true
	}

	classifiedNames := make(map[string]bool)
	for _, op := range surfaces.Operations() {
		// "engram" (rootCmd.Name()) is the bare self-describe invocation's
		// own sentinel CLICommand value (toolclass.go's comment: added
		// because ValidateOperations rejects a row with both key columns
		// empty) — it names the root command itself, which buildCatalog
		// never emits as one of doc.Commands, so it is deliberately excluded
		// from this comparison rather than a genuine mismatch.
		if op.CLICommand == "" || op.CLICommand == rootCmd.Name() {
			continue
		}
		classifiedNames[op.CLICommand] = true
	}

	if !reflect.DeepEqual(catalogNames, classifiedNames) {
		t.Errorf("catalog command names = %v, surfaces.Operations() non-empty CLICommand names = %v",
			sortedKeys(catalogNames), sortedKeys(classifiedNames))
	}

	for _, c := range doc.Commands {
		want, ok := surfaces.ClassForCommand(c.Name)
		if !ok {
			t.Errorf("catalog command %q has no surfaces.ClassForCommand entry", c.Name)
			continue
		}
		if c.BlastRadius.ReadOnly != want.ReadOnly ||
			c.BlastRadius.Destructive != want.Destructive ||
			c.BlastRadius.Idempotent != want.Idempotent ||
			c.BlastRadius.OpenWorld != want.OpenWorld {
			t.Errorf("catalog command %q blast_radius = %+v, want %+v", c.Name, c.BlastRadius, want)
		}
	}
}

// TestCatalogClaimsNoFlagErrorExitsGeneric replaces the retracted D-17 gate
// (the prior test pinning the flag-error note's exact wording).
// D-02/D-03 falsified the old promise ("a framework flag error exits 1, not
// 2"): after this plan, a
// framework flag error exits exitUsage, and exitGeneric is reserved for a
// mistyped verb and a genuinely unclassified internal error — never for a
// flag-shaped failure.
//
// The old test matched on any note containing the digits "1" and "2" near
// the word "flag" or "usage" — a reworded note can satisfy that *by
// accident* while asserting the opposite of what the test intended (e.g. a
// note saying "exits 1, not 2" contains both digits just as readily as one
// saying "exits 2, not 1"). This test asserts the intention directly, as a
// positive-set assertion over the numbers appearing in the SENTENCE that
// names a flag error, not a substring hunt over the whole note: for that
// sentence, the set of exit codes it names must contain exitUsage and must
// NOT contain exitGeneric. A sentence elsewhere in the same note that
// separately, correctly, names exitGeneric's other two causes is not a
// false positive, because it is not the sentence that mentions "flag".
func TestCatalogClaimsNoFlagErrorExitsGeneric(t *testing.T) {
	doc := decodeCatalog(t)

	var flagNotes []string
	for _, n := range doc.Notes {
		if strings.Contains(n, "flag") {
			flagNotes = append(flagNotes, n)
		}
	}
	if len(flagNotes) != 1 {
		t.Fatalf("notes mentioning flag errors = %d, want exactly 1; notes=%v", len(flagNotes), doc.Notes)
	}
	note := flagNotes[0]

	numRe := regexp.MustCompile(`\b\d+\b`)
	found := false
	for _, sentence := range strings.Split(note, ". ") {
		if !strings.Contains(sentence, "flag") {
			continue
		}
		found = true
		codes := make(map[int]bool)
		for _, m := range numRe.FindAllString(sentence, -1) {
			n, convErr := strconv.Atoi(m)
			if convErr == nil {
				codes[n] = true
			}
		}
		if codes[exitGeneric] {
			t.Errorf("the sentence naming a flag error also names exitGeneric (%d): %q", exitGeneric, sentence)
		}
		if !codes[exitUsage] {
			t.Errorf("the sentence naming a flag error does not name exitUsage (%d): %q", exitUsage, sentence)
		}
	}
	if !found {
		t.Fatalf("no sentence in the flag note mentions %q: %q", "flag", note)
	}
}

// TestHelpFlagStillPrintsHumanHelp is D-16's gate: --help stays ordinary
// human cobra output for both the root and a subcommand, never the JSON
// catalog.
func TestHelpFlagStillPrintsHumanHelp(t *testing.T) {
	resetClientFlags(t)
	resetHelpFlag(t, rootCmd)
	resetHelpFlag(t, searchCmd)

	stdout, _, err := runClient(t, "--help")
	if err != nil {
		t.Fatalf("--help returned error: %v", err)
	}
	if !strings.Contains(stdout, "Usage:") {
		t.Errorf("stdout does not contain cobra's usage heading: %q", stdout)
	}
	if isJSON(stdout) {
		t.Errorf("--help stdout unexpectedly parsed as JSON: %q", stdout)
	}

	stdout2, _, err2 := runClient(t, "search", "--help")
	if err2 != nil {
		t.Fatalf("search --help returned error: %v", err2)
	}
	if !strings.Contains(stdout2, "Search memories on a remote engram server") {
		t.Errorf("search --help did not show the subcommand's own help: %q", stdout2)
	}
	if isJSON(stdout2) {
		t.Errorf("search --help stdout unexpectedly parsed as JSON: %q", stdout2)
	}
}

// TestCatalogCarriesCrossSpineGuidance is D-07's guidance-content gate. The
// pre-existing TestCatalogEnumeratesEveryFlag already proves --cross-spine is
// *present* in the catalog with a *non-empty* Usage on both search and list;
// it does not check what that Usage says. D-00 requires the agent-facing
// discovery path (this catalog) to carry the same guidance as the
// human-facing one (--help), so this test pins the content, and asserts it
// is literally the same string collectFlags emits verbatim from
// pflag.Flag.Usage — not a paraphrase that could quietly drift from --help.
func TestCatalogCarriesCrossSpineGuidance(t *testing.T) {
	doc := decodeCatalog(t)

	byName := make(map[string][]struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Default string `json:"default"`
		Usage   string `json:"usage"`
	})
	for _, c := range doc.Commands {
		byName[c.Name] = c.Flags
	}

	commands := []string{"search", "list"}
	if len(commands) != 2 {
		t.Fatalf("test table has %d entries, want 2 (search and list)", len(commands))
	}

	for _, verb := range commands {
		t.Run(verb, func(t *testing.T) {
			flags := byName[verb]

			var crossSpine, scope *struct {
				Name    string `json:"name"`
				Type    string `json:"type"`
				Default string `json:"default"`
				Usage   string `json:"usage"`
			}
			for i := range flags {
				switch flags[i].Name {
				case "cross-spine":
					crossSpine = &flags[i]
				case "scope":
					scope = &flags[i]
				}
			}
			if crossSpine == nil {
				t.Fatalf("%s catalog entry has no cross-spine flag", verb)
			}
			if scope == nil {
				t.Fatalf("%s catalog entry has no scope flag", verb)
			}

			if crossSpine.Type != "bool" {
				t.Errorf("%s cross-spine type = %q, want %q", verb, crossSpine.Type, "bool")
			}
			if crossSpine.Default != "false" {
				t.Errorf("%s cross-spine default = %q, want %q", verb, crossSpine.Default, "false")
			}

			if !strings.Contains(crossSpine.Usage, "--scope") {
				t.Errorf("%s cross-spine usage does not name --scope: %q", verb, crossSpine.Usage)
			}
			if !strings.Contains(scope.Usage, "--cross-spine") {
				t.Errorf("%s scope usage does not name --cross-spine: %q", verb, scope.Usage)
			}

			cmd, _, err := rootCmd.Find([]string{verb})
			if err != nil {
				t.Fatalf("rootCmd.Find(%q): %v", verb, err)
			}
			liveCrossSpine := cmd.Flags().Lookup("cross-spine")
			if liveCrossSpine == nil {
				t.Fatalf("%s live command has no cross-spine flag", verb)
			}
			liveScope := cmd.Flags().Lookup("scope")
			if liveScope == nil {
				t.Fatalf("%s live command has no scope flag", verb)
			}

			// The catalog and --help must be the same string by
			// construction, not merely similar prose — this is what makes
			// "an agent's discovery path is not strictly worse than a
			// human's" a checked property rather than a hope (D-00).
			if crossSpine.Usage != liveCrossSpine.Usage {
				t.Errorf("%s catalog cross-spine usage != live --help usage:\ncatalog: %q\nlive:    %q",
					verb, crossSpine.Usage, liveCrossSpine.Usage)
			}
			if scope.Usage != liveScope.Usage {
				t.Errorf("%s catalog scope usage != live --help usage:\ncatalog: %q\nlive:    %q",
					verb, scope.Usage, liveScope.Usage)
			}
		})
	}
}

// TestHelpAndCatalogAreDifferentOutputs pins the bare-invocation catalog
// and --help's human output as a pair: a change that made help emit JSON,
// or the catalog emit help text, breaks this even if each half's own test
// were somehow satisfied.
func TestHelpAndCatalogAreDifferentOutputs(t *testing.T) {
	resetClientFlags(t)
	resetHelpFlag(t, rootCmd)

	catalogOut, _, err := runClient(t, noArgs...)
	if err != nil {
		t.Fatalf("bare invocation returned error: %v", err)
	}
	helpOut, _, err2 := runClient(t, "--help")
	if err2 != nil {
		t.Fatalf("--help returned error: %v", err2)
	}

	if catalogOut == helpOut {
		t.Fatal("bare invocation and --help produced identical stdout")
	}
	catalogIsJSON := isJSON(catalogOut)
	helpIsJSON := isJSON(helpOut)
	if catalogIsJSON == helpIsJSON {
		t.Errorf("expected exactly one of the two outputs to parse as JSON: catalog=%v help=%v",
			catalogIsJSON, helpIsJSON)
	}
}
