// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/surfaces"
)

// storeFields is the `engram store` flag set every store-shaped verb
// (store, schedule, supersede) shares, bound once per verb through
// addStoreFlags so the three cannot drift apart.
type storeFields struct {
	content   string
	scope     string
	source    string
	category  string
	tags      []string
	repo      string
	workspace string
	worktree  string
	baseDir   string
	summary   string
}

// addStoreFlags binds the store field set onto cmd, writing into f.
// categoryUsage lets a verb compose a rule onto --category (schedule's
// discovery-not-schedulable rule) without the other verbs stating a
// rejection they never raise.
func addStoreFlags(cmd *cobra.Command, f *storeFields, categoryUsage string) {
	cmd.Flags().StringVar(&f.content, "content", "", "memory content (required)")
	cmd.Flags().StringVar(&f.scope, "scope", "", "memory scope (required)")
	cmd.Flags().StringVar(&f.source, "source", "", "source of the memory")
	cmd.Flags().StringVar(&f.category, "category", "", categoryUsage)
	cmd.Flags().StringSliceVar(&f.tags, "tags", nil, "tags to attach to the memory")
	cmd.Flags().StringVar(&f.repo, "repo", "", "repo identifier")
	cmd.Flags().StringVar(&f.workspace, "workspace", "", "workspace identifier")
	cmd.Flags().StringVar(&f.worktree, "worktree", "", "worktree identifier")
	cmd.Flags().StringVar(&f.baseDir, "base-dir", "", "base directory")
	cmd.Flags().StringVar(&f.summary, "summary", "", "client-authored summary")
}

// requireStoreFields is the no-round-trip pre-check engram store runs: an
// empty --content or --scope is the caller's own mistake (exit 2).
func requireStoreFields(f *storeFields) error {
	if f.content == "" {
		return usageErrorf("--content is required")
	}
	if f.scope == "" {
		return usageErrorf("--scope is required")
	}
	return nil
}

var (
	scheduleFields         storeFields
	scheduleNotBefore      string
	scheduleNotAfter       string
	scheduleIdempotencyKey string
)

// parseWindowBound parses an RFC3339 flag value into a Timestamp, naming
// the proto field in the same field=/hint= envelope the server would
// return for a malformed bound. An empty value is an absent bound.
func parseWindowBound(field, value string) (*timestamppb.Timestamp, error) {
	if value == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, usageErrorf("field=%s hint=format: %s must be an RFC3339 timestamp, got %q", field, field, value)
	}
	return timestamppb.New(t), nil
}

// scheduleCmd is `engram schedule`, the client-tier verb over the
// ScheduleMemory RPC: the store field set plus a validity window, the same
// write the schedule_memory MCP tool makes. At least one bound is required
// and both are checked locally before any request.
var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Store a memory with a validity window (deferred reveal and/or expiry)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if err := requireStoreFields(&scheduleFields); err != nil {
			return err
		}
		if scheduleNotBefore == "" && scheduleNotAfter == "" {
			return usageErrorf("field=not_before hint=required: at least one of --not-before or --not-after is required")
		}
		notBefore, err := parseWindowBound("not_before", scheduleNotBefore)
		if err != nil {
			return err
		}
		notAfter, err := parseWindowBound("not_after", scheduleNotAfter)
		if err != nil {
			return err
		}
		client, format, timeout, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		f := &scheduleFields
		resp, err := client.ScheduleMemory(ctx, connect.NewRequest(&engramv1.ScheduleMemoryRequest{
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
			NotBefore:      notBefore,
			NotAfter:       notAfter,
			IdempotencyKey: scheduleIdempotencyKey,
		}))
		if err != nil {
			return wrapRPCError(err)
		}
		if format == formatText {
			return renderIDLine(cmd.OutOrStdout(), "scheduled", resp.Msg.GetShortId(), resp.Msg.GetId())
		}
		return renderJSON(cmd.OutOrStdout(), resp.Msg)
	},
}

func init() {
	addClientFlags(scheduleCmd)
	// Each window rule's Usage composes the registry's declared Sentence
	// verbatim (D-03), the same composition search/list use for --scope.
	atLeastOne, _ := surfaces.RuleByID(surfaces.RuleScheduleWindowAtLeastOne)
	ordering, _ := surfaces.RuleByID(surfaces.RuleWindowOrdering)
	notSchedulable, _ := surfaces.RuleByID(surfaces.RuleDiscoveryNotSchedulable)
	addStoreFlags(scheduleCmd, &scheduleFields,
		"category: one of decision, preference, convention, gotcha; "+notSchedulable.Sentence)
	scheduleCmd.Flags().StringVar(&scheduleNotBefore, "not-before", "",
		"RFC3339 instant before which the record is hidden from recall (deferred reveal); "+atLeastOne.Sentence)
	scheduleCmd.Flags().StringVar(&scheduleNotAfter, "not-after", "",
		"RFC3339 instant at which the record drops out of recall (expiry); "+ordering.Sentence)
	scheduleCmd.Flags().StringVar(&scheduleIdempotencyKey, "idempotency-key", "",
		"owner-scoped replay-safety key: a repeat with the same key and identical content returns the original record; omit for a fresh record every time")
	rootCmd.AddCommand(scheduleCmd)
}
