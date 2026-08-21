// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// migrationStatusCmd is `engram migration-status`, the client-tier sibling
// of the operator-tier `engram migrate status` (see "Naming decision" in
// 07-06-PLAN.md for why this name and not `engram status`): it reaches the
// SAME schema-version histogram over the Connect MigrateStatus RPC, so an
// operator with only a server URL and a token — no direct Qdrant access —
// can ask whether `engram migrate` would do work. Takes no arguments: the
// histogram is a whole-collection aggregate.
var migrationStatusCmd = &cobra.Command{
	Use:   "migration-status",
	Short: "Report the server's schema-version migration status",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, format, timeout, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		// The deadline comes from clientFromFlags' resolved timeout — the
		// single resolution point (D-05) — never a second resolution here.
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		resp, err := client.MigrateStatus(ctx, connect.NewRequest(&engramv1.MigrateStatusRequest{}))
		if err != nil {
			// Do not retry. Return wrapRPCError(err) and let Execute() map
			// it to a process exit code.
			return wrapRPCError(err)
		}
		if format == formatText {
			// Marshal with the SAME protojson options renderJSON uses below,
			// so the text field set cannot widen past the json document
			// (D-10 identity, mirroring engram get's client_get.go). No
			// bespoke table is needed: the existing field-table machinery
			// already renders the repeated SchemaVersionBucket messages as
			// indented rows, and EmitDefaultValues is what makes an empty
			// repeated field render as [] rather than being omitted.
			b, err := protojson.MarshalOptions{
				UseProtoNames:     true,
				EmitDefaultValues: true,
			}.Marshal(resp.Msg)
			if err != nil {
				return err
			}
			return renderOperatorView(cmd.OutOrStdout(), "migration status", json.RawMessage(b))
		}
		return renderJSON(cmd.OutOrStdout(), resp.Msg)
	},
}

func init() {
	// addClientFlags is mandatory, not stylistic: operatorCommands()
	// classifies tiers by whether a command's own flag set declares
	// "server" (cmd/engram/cmdwalk.go), so using addOperatorOutputFlag
	// instead would silently pull this verb into every operator-tier gate
	// — including the operator view-fixture coverage gate, which would
	// then demand a fixture entry this command should never have.
	addClientFlags(migrationStatusCmd)
	rootCmd.AddCommand(migrationStatusCmd)
}
