// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/config"
)

// addOperatorOutputFlag registers a `--output` flag on cmd, writing into
// target, matching the client tier's own vocabulary and default sentence
// (client_common.go's addClientFlags) so a reader comparing `engram search
// --help` against an operator command's `--help` sees the same contract.
// This is the operator tier's ONE `--output` registration site: every
// spine-review leaf and (plan 03-02) every existing operator command calls
// this rather than declaring the flag by hand.
func addOperatorOutputFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVar(target, "output", config.FlagDefault("output"),
		`output format: "json" or "text" (default: detect from stdout)`)
}

// operatorOutputFormat validates v through config.ValidateOutputFormat —
// the SAME exported validator the client tier's own ValidateClient calls —
// and resolves it against cmd's own stdout writer, never the process's real
// os.Stdout directly. Resolving TTY state from a stream the command is not
// actually writing to is how a caller with a custom writer (a test's
// *bytes.Buffer, or a future embedding of this binary) would get a format
// chosen for the wrong destination. An illegal value is rejected here with
// usageErrorf, so it exits exitUsage through Phase 1's taxonomy — there is
// no second, unvalidated path (outputFormatFromConfig is never called
// directly by any spine-review leaf).
func operatorOutputFormat(cmd *cobra.Command, v string) (outputFormat, error) {
	if err := config.ValidateOutputFormat(v); err != nil {
		return formatJSON, usageErrorf("%w", err)
	}
	return outputFormatFromConfig(v, isTTYWriter(cmd.OutOrStdout())), nil
}

// isTTYWriter reports whether w is an interactive character device,
// delegating to isTerminal (client_common.go) when w is a real *os.File and
// failing toward false (non-TTY, so JSON) for any other writer — the same
// "unknown environment fails toward JSON" default isTerminal itself
// documents.
func isTTYWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isTerminal(f)
}

// renderOperator writes cmd's final result through the ONE rendering path
// every operator command and spine-review leaf shares: text mode writes
// text plus a trailing newline; json mode marshals doc as exactly one JSON
// document plus a trailing newline (json.Encoder.Encode's own contract),
// mirroring runSelfDescribe's (catalog.go) encoding discipline.
func renderOperator(cmd *cobra.Command, format outputFormat, text string, doc any) error {
	if format == formatText {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), text)
		return err
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	return enc.Encode(doc)
}
