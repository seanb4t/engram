// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
		doc.Commands = append(doc.Commands, catalogCommand{
			Name:    cmd.Name(),
			Summary: cmd.Short,
			Flags:   collectFlags(root, cmd),
		})
	}

	// Built from the exitOK/exitGeneric/exitUsage/exitAuth/exitNotFound/
	// exitUnavailable constants declared in client_common.go — never from a
	// second literal list of integers. D-11's whole value is that the
	// advertised taxonomy and the mapper's real output are the same thing;
	// TestCatalogExitCodesMatchMapper gates that they stay so.
	doc.ExitCodes = []catalogExitCode{
		{Code: exitOK, Meaning: "success"},
		{Code: exitGeneric, Meaning: "generic or unclassified failure"},
		{Code: exitUsage, Meaning: "usage or validation error"},
		{Code: exitAuth, Meaning: "authentication or authorization failure"},
		{Code: exitNotFound, Meaning: "not found"},
		{Code: exitUnavailable, Meaning: "transport or server unavailable"},
	}

	doc.Notes = []string{
		"A flag-parsing error raised by the command framework itself (an unknown flag, " +
			"an unparseable flag value, or a mistyped verb) exits 1, not 2 (D-17). Exit 2 is " +
			"reserved for engram's own semantic validation — for example a missing " +
			"--server/ENGRAM_SERVER_URL, an invalid --output value, or a missing required " +
			"flag one of engram's own commands checks. Do not assume every usage-shaped " +
			"failure exits 2.",
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
	var flags []catalogFlag
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
