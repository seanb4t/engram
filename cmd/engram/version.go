// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file is the local tier's own, deliberately fourth `--output`
// registration site in this binary (after the client tier's addClientFlags,
// the operator tier's addOperatorOutputFlag, and cobra's built-in
// -v/--version). `version` cannot reuse either existing --output site: both
// detect from stdout and default to JSON when piped (D-01 forbids that for
// `version` — REQ-version-json requires the existing bare-string output stay
// unchanged for every piped caller), and neither can produce a bare scalar
// through renderOperator's headline-plus-padded-rows rendering (D-07). This
// is a bounded, documented divergence, not an oversight — see
// 01-CONTEXT.md's "Emergent pattern" note.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/config"
)

// versionDoc is the version subcommand's json lane payload (D-06): exactly
// one field. The version string already folds commit and dirty-state in as
// SemVer build metadata, so a separate "commit" field would be a second
// spelling of the same fact.
type versionDoc struct {
	Version string `json:"version"`
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the engram version",
	RunE:  runVersion,
}

func init() {
	// D-01: the default is the hardcoded string literal "text", never
	// config.FlagDefault("output") — that resolves to "" (the client.output
	// registry row carries no Default), which means "auto-detect from
	// stdout" and would silently make `engram version | cat` emit JSON.
	// Unlike every other --output flag in this binary, this one never
	// auto-detects from stdout — the usage string says so.
	versionCmd.Flags().String("output", "text",
		`output format: "json" or "text" (default: text, always — unlike every `+
			`other --output flag in this binary, this one never auto-detects from stdout)`)
	versionCmd.Flags().String("expect", "",
		"after printing, exit 7 unless this binary's version is exactly this value (no v-prefix normalisation)")
}

// runVersion validates --output through the single shared validator
// (D-03) and renders through its own minimal text/json pair (D-07) rather
// than renderOperator, which derives text FROM the json document via
// renderOperatorView and always emits a headline, a blank line, then padded
// label/value rows — it cannot produce a bare scalar.
func runVersion(cmd *cobra.Command, _ []string) error {
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		return err
	}
	// config.ValidateOutputFormat accepts three values: "json", "text", and
	// "" (empty). Branch on "json" alone and let everything else — both
	// "text" and the empty string — fall through to the text lane below.
	// The empty case is not reachable through normal argv (pflag rejects a
	// bare --output with no value) and the hardcoded default above is
	// non-empty, so this is a deliberate, stated treatment of the
	// validator's third legal value rather than an accident of the default.
	if err := config.ValidateOutputFormat(output); err != nil {
		return usageErrorf("%w", err)
	}
	expect, err := cmd.Flags().GetString("expect")
	if err != nil {
		return err
	}
	checkExpect := cmd.Flags().Changed("expect")
	if checkExpect && expect == "" {
		return usageErrorf("--expect needs a version to compare against")
	}

	// resolvedVersion (buildversion.go) is the ONLY place this value is
	// obtained: both the json and text lanes below read it, so D-07's
	// text-equals-json invariant keeps holding by construction across the
	// two separate render paths.
	v := resolvedVersion()
	if output == "json" {
		err = json.NewEncoder(cmd.OutOrStdout()).Encode(versionDoc{Version: v})
	} else {
		_, err = fmt.Fprintln(cmd.OutOrStdout(), v)
	}
	if err != nil {
		return err
	}
	// --expect is the Homebrew cask's install gate: the steps DSL cannot
	// compare values, so the binary does. A mismatch is a failed opt-in
	// check, not a broken command, hence exitFindings rather than 1 or 2.
	if checkExpect && v != expect {
		return &cliError{code: exitFindings, err: fmt.Errorf("version mismatch: this binary is %s, --expect %s", v, expect)}
	}
	return nil
}
