// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/seanb4t/engram/internal/surfaces"
)

// catalogDoc is the self-describe document a bare `engram` invocation emits
// on stdout (D-15). This is the CLI's own contract, not a wire message, so
// it is hand-declared rather than protobuf-derived. Field names use
// snake_case JSON tags, matching the convention the client commands' own
// output (client_common.go's renderJSON) already uses.
type catalogDoc struct {
	Binary    string            `json:"binary"`
	Version   string            `json:"version"`
	Commands  []catalogCommand  `json:"commands"`
	ExitCodes []catalogExitCode `json:"exit_codes"`
	Notes     []string          `json:"notes"`
}

// catalogCommand is one entry in catalogDoc.Commands.
type catalogCommand struct {
	Name    string        `json:"name"`
	Summary string        `json:"summary"`
	Flags   []catalogFlag `json:"flags"`
	// BlastRadius is derived from the SAME internal/surfaces table the MCP
	// tool annotations compose from (D-11) — never a second literal, so the
	// two lanes cannot silently disagree. buildCatalog PANICS rather than
	// emitting a zero-valued BlastRadius when a command has no
	// surfaces.ClassForCommand entry: a command advertised as safe because
	// nobody classified it is exactly the failure this field exists to
	// prevent, and TestCatalogBlastRadiusMatchesToolClasses's
	// both-directions gate is meant to catch any such gap long before it
	// reaches a release — this panic is the defense-in-depth backstop for
	// the case where it somehow didn't.
	BlastRadius catalogBlastRadius `json:"blast_radius"`
}

// catalogBlastRadius mirrors surfaces.Class's four MCP ToolAnnotations
// hints in the shape chosen at 02-05's checkpoint (option-a): a nested
// object, lossless with the MCP lane, so an agent's blast-radius logic
// transfers unchanged across both. Keys are snake_case, matching this
// document's own convention — they rhyme with, but do not match, the MCP
// wire's camelCase readOnlyHint/destructiveHint/idempotentHint/
// openWorldHint.
type catalogBlastRadius struct {
	ReadOnly    bool `json:"read_only"`
	Destructive bool `json:"destructive"`
	Idempotent  bool `json:"idempotent"`
	OpenWorld   bool `json:"open_world"`
}

// catalogFlag is one entry in catalogCommand.Flags.
type catalogFlag struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Default string `json:"default"`
	Usage   string `json:"usage"`
}

// catalogExitCode is one entry in catalogDoc.ExitCodes.
type catalogExitCode struct {
	Code    int    `json:"code"`
	Meaning string `json:"meaning"`
}

// buildCatalog walks the live cobra tree rooted at root and derives a
// catalogDoc from it — never from a hand-maintained literal — so a command
// or flag added later appears here with no edit, and cannot silently go
// missing (D-15).
func buildCatalog(root *cobra.Command) catalogDoc {
	doc := catalogDoc{
		Binary:  root.Name(),
		Version: root.Version,
	}

	for _, cmd := range root.Commands() {
		// Cobra auto-registers "help" and "completion" at Execute() time
		// (InitDefaultHelpCmd / InitDefaultCompletionCmd, called before any
		// RunE runs). Neither is part of the surface this phase's three
		// client verbs — or any operator command — advertise, so both are
		// skipped by name here, alongside any command explicitly marked
		// Hidden. This omission is deliberate, not a bug: an agent parsing
		// this catalog should see the same commands a human sees listed in
		// `engram --help`'s "Available Commands" section, minus cobra's own
		// scaffolding.
		if cmd.Hidden || cmd.Name() == "help" || cmd.Name() == "completion" {
			continue
		}
		class, ok := surfaces.ClassForCommand(cmd.Name())
		if !ok {
			panic(fmt.Sprintf(
				"catalog: command %q has no internal/surfaces blast-radius classification — "+
					"add a row to internal/surfaces/toolclass.go's operations table",
				cmd.Name(),
			))
		}
		doc.Commands = append(doc.Commands, catalogCommand{
			Name:    cmd.Name(),
			Summary: cmd.Short,
			Flags:   collectFlags(root, cmd),
			BlastRadius: catalogBlastRadius{
				ReadOnly:    class.ReadOnly,
				Destructive: class.Destructive,
				Idempotent:  class.Idempotent,
				OpenWorld:   class.OpenWorld,
			},
		})
	}

	// Built from the exitOK/exitGeneric/exitUsage/exitAuth/exitNotFound/
	// exitUnavailable constants declared in client_common.go — never from a
	// second literal list of integers. D-11's whole value is that the
	// advertised taxonomy and the mapper's real output are the same thing;
	// TestCatalogExitCodesMatchMapper gates that they stay so.
	doc.ExitCodes = []catalogExitCode{
		{Code: exitOK, Meaning: "success"},
		// D-02: exitGeneric is redefined as an unreachable-by-design
		// internal-error backstop, not a general-purpose failure code —
		// every classified path is now typed. It survives only for a
		// mistyped verb (see notes) and a genuinely unclassified Go error.
		{Code: exitGeneric, Meaning: "unclassified internal error (backstop only — not a general-purpose failure code; see notes)"},
		{Code: exitUsage, Meaning: "usage or validation error"},
		{Code: exitAuth, Meaning: "authentication or authorization failure"},
		{Code: exitNotFound, Meaning: "not found"},
		{Code: exitUnavailable, Meaning: "transport or server unavailable"},
		// D-06: a client-side request deadline is distinguished from a
		// transport/server-unavailable failure — "raise --timeout" and
		// "check the server is up" are different remedies.
		{Code: exitTimeout, Meaning: "request deadline exceeded"},
	}

	doc.Notes = []string{
		// D-02/D-03 retract the published D-17 promise that a framework
		// flag error exits 1: it now exits exitUsage, alongside a violated
		// mutually-exclusive flag group and engram's own semantic
		// validation. The two remaining exitGeneric paths are named
		// explicitly so neither reads as an oversight.
		fmt.Sprintf(
			"A flag-parsing error raised by the command framework itself (an unknown flag "+
				"or an unparseable flag value) and a violated mutually-exclusive flag group "+
				"both exit %d, alongside engram's own semantic validation — for example a "+
				"missing --server/ENGRAM_SERVER_URL or an invalid --output value. Two paths "+
				"still exit %d: a mistyped verb, rejected during cobra's own command "+
				"resolution before any engram hook runs, and a genuinely unclassified "+
				"internal error.",
			exitUsage, exitGeneric,
		),
		"An empty result set is a success and exits 0 (D-12); absence of results is a " +
			"legitimate answer, not a failure.",
	}

	return doc
}

// collectFlags returns the sorted, de-duplicated set of flags a caller may
// legitimately pass to cmd: cmd's own flags, plus root's persistent flags
// (since a caller may pass those on any subcommand too).
func collectFlags(root, cmd *cobra.Command) []catalogFlag {
	seen := make(map[string]bool)
	// Non-nil by construction so a flagless command always serializes as
	// "flags": [] and never "flags": null. Two reasons this matters:
	// a caller parsing the catalog should not have to handle both shapes
	// for the same fact, and cobra registers a command's own -h/--help
	// lazily inside execute() — so within the shared cmd/engram test
	// binary, whether a flagless command had been executed earlier decided
	// nil-vs-empty and made TestCatalogGolden depend on test run order.
	flags := []catalogFlag{}
	add := func(f *pflag.Flag) {
		if f.Hidden || seen[f.Name] {
			return
		}
		seen[f.Name] = true
		flags = append(flags, catalogFlag{
			Name:    f.Name,
			Type:    f.Value.Type(),
			Default: f.DefValue,
			Usage:   f.Usage,
		})
	}
	cmd.Flags().VisitAll(add)
	root.PersistentFlags().VisitAll(add)
	sort.Slice(flags, func(i, j int) bool { return flags[i].Name < flags[j].Name })
	return flags
}

// runSelfDescribe is rootCmd's RunE (D-15): it builds the catalog from the
// live tree and encodes it as one JSON document to cmd.OutOrStdout().
// Nothing is written to stderr on this path (D-07). Writing through the
// command's writer rather than the package-level print family is what lets
// tests capture it via rootCmd.SetOut.
func runSelfDescribe(cmd *cobra.Command, _ []string) error {
	doc := buildCatalog(cmd.Root())
	enc := json.NewEncoder(cmd.OutOrStdout())
	return enc.Encode(doc)
}
