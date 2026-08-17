// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
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
// every operator command and spine-review leaf shares: text mode renders
// headline followed by a view of doc — renderOperatorView (operator_view.go)
// walks the SAME bytes the json lane marshals doc to, so text is a rendered
// view of the json document rather than a second, independently-maintained
// format; json mode marshals doc as exactly one JSON document plus a
// trailing newline (json.Encoder.Encode's own contract), mirroring
// runSelfDescribe's (catalog.go) encoding discipline.
//
// There is no second serialization and no second call site from a doc
// value to rendered text (06-CONTEXT.md D-01), so text/json identity holds
// by construction rather than by an enforced coverage rule (D-02) — which
// is also why doc any stays safe as this function's argument. Text output
// carries no stability contract beyond "it shows every field" (D-03).
func renderOperator(cmd *cobra.Command, format outputFormat, headline string, doc any) error {
	if format == formatText {
		return renderOperatorView(cmd.OutOrStdout(), headline, doc)
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	return enc.Encode(doc)
}
