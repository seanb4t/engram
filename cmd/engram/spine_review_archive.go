// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/store"
)

// archiveSummary is a headline producer (06-CONTEXT.md D-04): it returns
// ONE hand-written prose line naming the verb and how many ids were
// processed, shared by both `spine-review archive` and `spine-review
// restore` -- pure (value types only, no *store.Store, no context.Context)
// so it is unit-testable without a live Qdrant, mirroring
// spineScanSummary's/consolidateSummary's discipline. verb names which of
// the two ran ("archive" or "restore"); the per-id outcome table (id
// present or omitted, per row) is rendered by the operator view from
// archiveDoc, not by this function -- this line is additive prose over that
// complete table and its wording may change freely (D-03: --output json is
// the contract).
func archiveSummary(results []store.ArchiveResult, verb string) string {
	return fmt.Sprintf("spine %s: %d id(s) processed", verb, len(results))
}

// archiveResultDoc is one id's JSON-mode outcome: the SAME three explicit
// states (changed/already/not_found) the text report names, as a plain
// string -- never left for a JSON consumer to infer from prose (T-03-29's
// mitigation). The conditional "id=" segment a rendered row shows (or
// omits) is produced entirely by encoding/json dropping the "id" key under
// ID's omitempty tag below and the operator view rendering only the keys
// the json lane actually emitted -- there is no conditional-rendering code
// anywhere (06-CONTEXT.md D-01/D-02).
type archiveResultDoc struct {
	// Requested is the caller's own token, present on every row so a
	// consumer can correlate results back to its input list; ID is the
	// canonical id and is omitted when the token resolved to nothing.
	Requested string `json:"requested"`
	ID        string `json:"id,omitempty"`
	Outcome   string `json:"outcome"`
}

// archiveReportDoc is the JSON-mode report's exported-field shape: ids and
// outcomes only -- NEVER record content, mirroring every other spine-review
// leaf's report shape (T-03-05's sibling discipline). A hand-declared
// struct, not an embedded store.ArchiveResult slice, so this exclusion is
// enforced by the type itself.
type archiveReportDoc struct {
	Verb    string             `json:"verb"`
	Results []archiveResultDoc `json:"results"`
}

// archiveDoc converts results into archiveReportDoc, keeping Results
// non-nil so it marshals as "[]", never "null", mirroring every other
// spine-review leaf's *Doc converter.
func archiveDoc(results []store.ArchiveResult, verb string) archiveReportDoc {
	doc := archiveReportDoc{Verb: verb, Results: make([]archiveResultDoc, 0, len(results))}
	for _, r := range results {
		doc.Results = append(doc.Results, archiveResultDoc{Requested: r.Requested, ID: r.ID, Outcome: string(r.Outcome)})
	}
	return doc
}

// spineArchiveOrRestore resolves each id in ids -- accepting a full UUID OR
// a short_id, exactly like every other id-accepting surface in this system
// (CLAUDE.md's memory contract: a short_id is "accepted anywhere an id is
// accepted") -- via st.ResolvePointID, then calls fn (store.Archive or
// store.Restore) with the resolved canonical id. store.Archive/Restore
// themselves take a resolved id only and never resolve short ids (mirroring
// Store.Supersede's own documented contract: "callers thread resolved ids
// in, store methods do not resolve short ids"), so resolution happens here,
// once, before either verb is called.
//
// A per-id NOT-FOUND outcome -- either ResolvePointID finding no record for
// a short_id, or fn's own existence check on an already-resolved UUID -- is
// recoverable: appended to results, and the loop CONTINUES to the remaining
// ids, never silently abandoning them (the behavior this command's plan
// requires). The FIRST such error is returned once every id has been
// processed, so the caller can still render the complete per-id report
// before applying that error's exit code.
//
// Any OTHER error -- an ambiguous short_id, an empty/malformed id, or a
// lock/transport failure -- aborts immediately: results is returned exactly
// as far as it got, shorter than ids, the signal the caller uses to
// distinguish "completed with a not-found among the results" from "aborted
// partway through".
//
// This function itself still returns those already-collected partial
// results on abort (never discards them) -- the discard happens one layer
// up, in renderArchiveResults, which reads len(results) < len(ids) as "no
// complete report to render" and skips rendering entirely (IN-01,
// 06-REVIEW.md). That is a deliberate asymmetry with every other
// partial-progress path this phase added (migrate revert's CR-06, purge's
// spared/appeared accounting), which all render whatever partial progress
// they have rather than discard it: a multi-id archive/restore batch's
// completeness bar for rendering ANYTHING was judged higher than a single
// scalar operation's, because a partial archive/restore report has no
// natural "in-progress" framing the way a manifest-driven purge/migrate
// does. If that judgment turns out wrong in practice, the fix is in
// renderArchiveResults: render the partial results slice (with an
// Applied-equivalent field honestly reflecting "processing was
// interrupted") before returning the classified error, mirroring
// revertApplyRun's render-then-return-error pattern.
func spineArchiveOrRestore(ctx context.Context, st *store.Store, ids []string, fn func(context.Context, string) (store.ArchiveResult, error)) (results []store.ArchiveResult, err error) {
	results = make([]store.ArchiveResult, 0, len(ids))
	for _, raw := range ids {
		resolved, rerr := st.ResolvePointID(ctx, raw)
		if rerr != nil {
			if !errors.Is(rerr, store.ErrNotFound) {
				return results, rerr
			}
			// ID stays empty: the token resolved to nothing, so there is no
			// canonical id to report. Requested carries the caller's token
			// so the row is still correlatable to its input.
			results = append(results, store.ArchiveResult{Requested: raw, Outcome: store.ArchiveOutcomeNotFound})
			if err == nil {
				err = rerr
			}
			continue
		}
		res, rerr := fn(ctx, resolved)
		if rerr != nil && res.Outcome == "" {
			return results, rerr
		}
		// The store sees only the resolved id, so the raw token is stamped
		// here -- the one layer that knows both.
		res.Requested = raw
		results = append(results, res)
		if rerr != nil && err == nil {
			err = rerr
		}
	}
	return results, err
}

// renderArchiveResults renders results (whatever spineArchiveOrRestore
// collected) through the operator output path, then applies procErr's exit
// code -- mirroring spine-review verify's D-14 shape: render first, THEN
// return the classifying error, so a caller piping `--output json | jq`
// always sees the full report even on a nonzero exit.
func renderArchiveResults(cmd *cobra.Command, format outputFormat, verb string, ids []string, results []store.ArchiveResult, procErr error) error {
	if procErr != nil && len(results) < len(ids) {
		// Aborted partway through on a genuine (non-not-found) failure --
		// no complete report to render. This discards spineArchiveOrRestore's
		// already-collected results for the ids that DID succeed before the
		// abort; see that function's doc comment (IN-01, 06-REVIEW.md) for
		// why this differs from every other partial-progress path in this
		// phase, and what changing it would look like.
		return classifyOperatorErr(procErr)
	}
	if err := renderOperator(cmd, format, archiveSummary(results, verb), archiveDoc(results, verb)); err != nil {
		return err
	}
	if procErr != nil {
		return classifyOperatorErr(procErr)
	}
	return nil
}

var (
	spineArchiveIDs     []string
	spineArchiveTimeout time.Duration
	spineArchiveOutput  string

	spineRestoreIDs     []string
	spineRestoreTimeout time.Duration
	spineRestoreOutput  string
)

// spineReviewArchiveCmd stamps archived_at on one or more records by id,
// excluding each from Search/List/SearchDiscovery/ListScheduled recall
// while leaving it fetchable via get_memory -- reversible via
// spine-review restore, never a delete, content erasure, or vector removal
// (REQ-archive-tier's safety prohibition). Ids only, never a filter form
// (Task 1's checkpoint: option-a) -- a mutating verb's blast radius is
// always the explicit id set an operator supplied, never an implicit scope
// sweep. See the CLI guide's "spine-review archive / spine-review restore"
// section for the full contract.
var spineReviewArchiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Archive one or more records by id (reversible via restore)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if len(spineArchiveIDs) == 0 {
			return usageErrorf("--id <id> is required (may be repeated)")
		}
		format, err := operatorOutputFormat(cmd, spineArchiveOutput)
		if err != nil {
			return err
		}
		st, err := server.StoreFromEnv()
		if err != nil {
			return classifyOperatorErrConstruction(err)
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if spineArchiveTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, spineArchiveTimeout)
			defer cancel()
		}
		results, procErr := spineArchiveOrRestore(ctx, st, spineArchiveIDs, st.Archive)
		return renderArchiveResults(cmd, format, "archive", spineArchiveIDs, results, procErr)
	},
}

// spineReviewRestoreCmd reverses spine-review archive: it deletes the
// archived_at key outright, returning the record to normal recall. See
// spineReviewArchiveCmd's doc comment for the shared contract (ids only,
// per-id outcomes, the CLI guide section).
var spineReviewRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore one or more previously-archived records by id",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if len(spineRestoreIDs) == 0 {
			return usageErrorf("--id <id> is required (may be repeated)")
		}
		format, err := operatorOutputFormat(cmd, spineRestoreOutput)
		if err != nil {
			return err
		}
		st, err := server.StoreFromEnv()
		if err != nil {
			return classifyOperatorErrConstruction(err)
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if spineRestoreTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, spineRestoreTimeout)
			defer cancel()
		}
		results, procErr := spineArchiveOrRestore(ctx, st, spineRestoreIDs, st.Restore)
		return renderArchiveResults(cmd, format, "restore", spineRestoreIDs, results, procErr)
	},
}

func init() {
	addOperatorOutputFlag(spineReviewArchiveCmd, &spineArchiveOutput)
	spineReviewArchiveCmd.Flags().StringSliceVar(&spineArchiveIDs, "id", nil,
		"id (or short_id) of a record to archive; may be repeated")
	spineReviewArchiveCmd.Flags().DurationVar(&spineArchiveTimeout, "timeout", 5*time.Minute,
		"max wall-clock for the run (0 disables); also cancellable via Ctrl-C")
	spineReviewCmd.AddCommand(spineReviewArchiveCmd)

	addOperatorOutputFlag(spineReviewRestoreCmd, &spineRestoreOutput)
	spineReviewRestoreCmd.Flags().StringSliceVar(&spineRestoreIDs, "id", nil,
		"id (or short_id) of a previously-archived record to restore; may be repeated")
	spineReviewRestoreCmd.Flags().DurationVar(&spineRestoreTimeout, "timeout", 5*time.Minute,
		"max wall-clock for the run (0 disables); also cancellable via Ctrl-C")
	spineReviewCmd.AddCommand(spineReviewRestoreCmd)
}
