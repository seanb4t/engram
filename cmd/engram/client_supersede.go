// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

var (
	supersedeFields         storeFields
	supersedeTargets        []string
	supersedeCitations      []string
	supersedeIdempotencyKey string
	supersedeValidateOnly   bool
)

// parseCitationFlags converts repeated --citation values of the form
// <kind>:<ref> (kind one of file, commit, url, repo; the ref keeps every
// later colon, so file:a.go:12 is kind file, ref a.go:12) into Citation
// messages. A value with no colon is the caller's own mistake, rejected
// before any request; the kind and ref themselves are validated by the
// server, which names the field and hint.
func parseCitationFlags(values []string) ([]*engramv1.Citation, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make([]*engramv1.Citation, 0, len(values))
	for _, v := range values {
		kind, ref, ok := strings.Cut(v, ":")
		if !ok {
			return nil, usageErrorf("--citation %q: expected <kind>:<ref>, e.g. file:internal/auth/token.go", v)
		}
		out = append(out, &engramv1.Citation{Kind: kind, Ref: ref})
	}
	return out, nil
}

// supersedeCmd is `engram supersede`, the client-tier verb over the
// SupersedeMemory RPC: the store field set for the new, correcting record
// plus the set of predecessors it replaces, the same write the
// supersede_memory MCP tool makes. --validate-only runs the server's full
// preflight and writes nothing; --idempotency-key is passed through and
// never generated here, so two identical invocations without one are two
// records, as the caller asked.
var supersedeCmd = &cobra.Command{
	Use:   "supersede",
	Short: "Store a correcting memory that supersedes one or more predecessors",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if err := requireStoreFields(&supersedeFields); err != nil {
			return err
		}
		if len(supersedeTargets) == 0 {
			return usageErrorf("field=supersedes hint=required: at least one --supersedes <id> is required")
		}
		citations, err := parseCitationFlags(supersedeCitations)
		if err != nil {
			return err
		}
		client, format, timeout, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		f := &supersedeFields
		resp, err := client.SupersedeMemory(ctx, connect.NewRequest(&engramv1.SupersedeMemoryRequest{
			Content:        f.content,
			Scope:          f.scope,
			Source:         f.source,
			Category:       f.category,
			Tags:           f.tags,
			Repo:           f.repo,
			Workspace:      f.workspace,
			Worktree:       f.worktree,
			BaseDir:        f.baseDir,
			Summary:        f.summary,
			Citations:      citations,
			Supersedes:     supersedeTargets,
			IdempotencyKey: supersedeIdempotencyKey,
			ValidateOnly:   supersedeValidateOnly,
		}))
		if err != nil {
			return wrapRPCError(err)
		}
		if format == formatText {
			headline := fmt.Sprintf("superseded: %s (%s)", resp.Msg.GetShortId(), resp.Msg.GetId())
			if resp.Msg.GetValidated() && resp.Msg.GetId() == "" {
				headline = fmt.Sprintf("validated: %d target(s), nothing written", len(resp.Msg.GetSupersedes()))
			}
			return renderProtoView(cmd.OutOrStdout(), headline, resp.Msg)
		}
		return renderJSON(cmd.OutOrStdout(), resp.Msg)
	},
}

func init() {
	addClientFlags(supersedeCmd)
	addStoreFlags(supersedeCmd, &supersedeFields, "category: one of decision, preference, convention, gotcha")
	supersedeCmd.Flags().StringSliceVar(&supersedeTargets, "supersedes", nil,
		"id (full UUID or short id) of a record the new one corrects; repeatable; an invalid set rejects the whole call (required)")
	supersedeCmd.Flags().StringSliceVar(&supersedeCitations, "citation", nil,
		"citation as <kind>:<ref> (kind: file, commit, url or repo); repeatable")
	supersedeCmd.Flags().StringVar(&supersedeIdempotencyKey, "idempotency-key", "",
		"owner-scoped replay-safety key over content and the target set: a repeat with the same key replays instead of duplicating; never generated for you")
	supersedeCmd.Flags().BoolVar(&supersedeValidateOnly, "validate-only", false,
		"run the full preflight (ownership, single live head, rule and ambiguity checks) and name the resolved targets without writing anything")
	rootCmd.AddCommand(supersedeCmd)
}
