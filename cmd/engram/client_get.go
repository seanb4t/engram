// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// getHeadline builds engram get's headline: "record {short_id}", plus the
// canonical-order state words (memoryStateWords, D-13) in parentheses,
// comma-SPACE-joined, when the record carries non-default state. This
// deliberately diverges from the STATE table cell's comma-NO-space join
// (memoryStateCell): the table cell is optimised for cut/awk-style piping,
// the headline is prose. Both draw from the same memoryStateWords, which is
// what keeps engram get's headline and engram list/search's STATE column
// agreeing about vocabulary.
func getHeadline(m *engramv1.Memory, now time.Time) string {
	headline := "record " + m.GetShortId()
	words := memoryStateWords(m, now)
	if len(words) == 0 {
		return headline
	}
	return headline + " (" + strings.Join(words, ", ") + ")"
}

// getCmd is `engram get <id>`, the client-tier verb over the ungated
// GetMemory RPC (D-09): GetMemory is not recall-gated, so this always
// returns a record's full content by id or short id — archived, superseded,
// or windowed-out — with no `--full` flag, matching the get_memory MCP
// tool. It deliberately accepts exactly ONE id: the operator view is a
// field-per-line table for one record, and N ids would produce N stacked
// field tables, which is exactly the shape rejected for search/list.
var getCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Fetch one memory by id or short id and render its full state",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, format, timeout, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		// The deadline comes from clientFromFlags' resolved timeout — the
		// single resolution point (D-05) — never a second resolution here.
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		resp, err := client.GetMemory(ctx, connect.NewRequest(&engramv1.GetMemoryRequest{Id: args[0]}))
		if err != nil {
			// Do not retry. Return wrapRPCError(err) and let Execute() map
			// it to a process exit code (D-09/D-10).
			return wrapRPCError(err)
		}
		mem := resp.Msg.GetMemory()
		if format == formatText {
			// Marshal with the SAME protojson options renderJSON uses below,
			// so protojson's Timestamp rendering and optional presence
			// semantics (superseded_by, schema_version, summary_model)
			// survive into the text lane, and the text field set cannot
			// widen past the json document. Marshalling the Go struct with
			// encoding/json instead would lose both.
			b, err := protojson.MarshalOptions{
				UseProtoNames:     true,
				EmitDefaultValues: true,
			}.Marshal(mem)
			if err != nil {
				return err
			}
			return renderOperatorView(cmd.OutOrStdout(), getHeadline(mem, time.Now()), json.RawMessage(b))
		}
		// The MEMORY, not the response wrapper — both lanes must serialize
		// the same value with the same options or the text/json identity
		// property does not hold.
		return renderJSON(cmd.OutOrStdout(), mem)
	},
}

func init() {
	// addClientFlags is mandatory, not stylistic: operatorCommands()
	// classifies tiers by whether a command's own flag set declares
	// "server" (cmd/engram/cmdwalk.go), so using addOperatorOutputFlag
	// instead would silently pull engram get into every operator-tier gate
	// — including the operator view-fixture coverage gate, which would
	// then demand a fixture entry this command should never have.
	addClientFlags(getCmd)
	rootCmd.AddCommand(getCmd)
}
