// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/store"
)

var (
	spineScanScope     string
	spineScanAllScopes bool
	spineScanTimeout   time.Duration
	spineScanOutput    string
)

// spineReviewScanCmd reports an inventory of the memory spine: total
// records and a scope-by-category breakdown. Read-only by construction —
// it never issues a mutating Qdrant RPC (T-03-07/T-03-24's mitigation) —
// and Subject-less like every other operator-tier command on this binary,
// so the report spans every actor's records, never one caller's bucket.
var spineReviewScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Report an inventory of the memory spine",
	RunE: func(cmd *cobra.Command, _ []string) error {
		// Reuses summarize.go's exact wording (summarize.go:38) so the
		// operator tier stays internally consistent — NOT because this
		// check satisfies an already-registered surfaces.ConditionalRule:
		// summarize.go's own check is a bare usageErrorf, and the registry
		// (internal/surfaces/rules.go) holds no rule for it. See this
		// plan's SUMMARY (§ Deferrals) for the follow-up that would
		// register one.
		if spineScanScope == "" && !spineScanAllScopes {
			return usageErrorf("--scope <scope> or --all-scopes is required")
		}
		format, err := operatorOutputFormat(cmd, spineScanOutput)
		if err != nil {
			return err
		}
		st, err := server.StoreFromEnv()
		if err != nil {
			return classifyOperatorErrConstruction(err)
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if spineScanTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, spineScanTimeout)
			defer cancel()
		}
		res, err := st.ScanSpine(ctx, store.SpineScanOptions{Scope: spineScanScope})
		if err != nil {
			return classifyOperatorErr(err)
		}
		return renderOperator(cmd, format, spineScanSummary(res, spineScanScope), spineScanDoc(res, spineScanScope))
	},
}

// spineScanBreakdownDoc is one (scope, category) bucket in the JSON report.
type spineScanBreakdownDoc struct {
	Scope    string `json:"scope"`
	Category string `json:"category"`
	Count    uint64 `json:"count"`
}

// spineScanReportDoc is the JSON-mode report's exported-field shape: ids,
// counts, scopes and categories only — NEVER record content or summary
// text, on any spine-review output path (T-03-05's mitigation). A hand
// declared struct, not an embedded store.SpineScanResult, so this
// exclusion is enforced by the type itself rather than by discipline.
//
// Scope is the raw --scope flag value, with the empty string meaning all
// scopes (spineReviewScanCmd requires one of --scope or --all-scopes, so
// no separate all_scopes key is needed to disambiguate). Added per
// 06-01-PLAN.md §Conversion Rules R2 (gap closure): spineScanSummary has
// always rendered the scan target on its headline, but no key carried it
// until now — 06-CONTEXT.md <specifics> names this gap explicitly and
// blesses closing it. Placed first so the rendered field table leads with
// the scan target exactly as the headline always has.
type spineScanReportDoc struct {
	Scope          string    `json:"scope"`
	Total          uint64    `json:"total"`
	ScannedAt      time.Time `json:"scanned_at"`
	Owners         uint64    `json:"owners"`
	WithoutSummary uint64    `json:"without_summary"`
	WithSummary    uint64    `json:"with_summary"`
	Superseded     uint64    `json:"superseded"`
	Expired        uint64    `json:"expired"`
	Scheduled      uint64    `json:"scheduled"`
	// Archived is its own bucket, separate from Expired (D-12, plan 03-06):
	// an archived record's not_after may or may not also be lapsed, but the
	// two states are independently observable, never folded together.
	Archived        uint64                  `json:"archived"`
	WithCitations   uint64                  `json:"with_citations"`
	Citations       uint64                  `json:"citations"`
	ByScopeCategory []spineScanBreakdownDoc `json:"by_scope_category"`
}

// spineScanDoc converts res into spineScanReportDoc, keeping the breakdown
// slice non-nil so an empty spine marshals it as `[]`, never `null`. scope
// is the raw --scope flag value (empty means all scopes), threaded through
// verbatim into Scope.
func spineScanDoc(res store.SpineScanResult, scope string) spineScanReportDoc {
	doc := spineScanReportDoc{
		Scope:           scope,
		Total:           res.Total,
		ScannedAt:       res.ScannedAt,
		Owners:          res.Owners,
		WithoutSummary:  res.WithoutSummary,
		WithSummary:     res.WithSummary,
		Superseded:      res.Superseded,
		Expired:         res.Expired,
		Scheduled:       res.Scheduled,
		Archived:        res.Archived,
		WithCitations:   res.WithCitations,
		Citations:       res.Citations,
		ByScopeCategory: make([]spineScanBreakdownDoc, 0, len(res.ByScopeCategory)),
	}
	for _, c := range res.ByScopeCategory {
		doc.ByScopeCategory = append(doc.ByScopeCategory, spineScanBreakdownDoc{
			Scope: c.Scope, Category: c.Category, Count: c.Count,
		})
	}
	return doc
}

// spineScanSummary renders the operator-facing headline. Pure (value types
// only — no *store.Store, no context.Context) so it is unit-testable
// without a live Qdrant, mirroring reindexSummary's discipline
// (reindex.go). Per D-04 (06-CONTEXT.md) this is a headline producer only:
// it names the scan target, the scan instant, the total and the owner
// count, and returns a single line with no trailing newline. The health
// signals (without_summary/with_summary/superseded/expired/scheduled/
// archived/with_citations/citations) and the (scope, category) breakdown
// this function used to print as extra lines now render only as
// renderOperatorView's field-table rows, from spineScanDoc.
func spineScanSummary(res store.SpineScanResult, scope string) string {
	target := scope
	if target == "" {
		target = "all scopes"
	}
	return fmt.Sprintf("spine scan (%s) at %s: total=%d owners=%d",
		target, res.ScannedAt.Format(time.RFC3339), res.Total, res.Owners)
}

func init() {
	addOperatorOutputFlag(spineReviewScanCmd, &spineScanOutput)
	spineReviewScanCmd.Flags().StringVar(&spineScanScope, "scope", "", "only scan records in this scope")
	spineReviewScanCmd.Flags().BoolVar(&spineScanAllScopes, "all-scopes", false, "sweep every scope (required if --scope is omitted)")
	spineReviewScanCmd.Flags().DurationVar(&spineScanTimeout, "timeout", 5*time.Minute,
		"max wall-clock for the sweep (0 disables); also cancellable via Ctrl-C")
	spineReviewScanCmd.MarkFlagsMutuallyExclusive("scope", "all-scopes")
	spineReviewCmd.AddCommand(spineReviewScanCmd)
}
