// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
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
		return renderOperator(cmd, format, spineScanSummary(res, spineScanScope), spineScanDoc(res))
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
type spineScanReportDoc struct {
	Total           uint64                  `json:"total"`
	ScannedAt       time.Time               `json:"scanned_at"`
	Owners          uint64                  `json:"owners"`
	WithoutSummary  uint64                  `json:"without_summary"`
	WithSummary     uint64                  `json:"with_summary"`
	Superseded      uint64                  `json:"superseded"`
	Expired         uint64                  `json:"expired"`
	Scheduled       uint64                  `json:"scheduled"`
	WithCitations   uint64                  `json:"with_citations"`
	Citations       uint64                  `json:"citations"`
	ByScopeCategory []spineScanBreakdownDoc `json:"by_scope_category"`
}

// spineScanDoc converts res into spineScanReportDoc, keeping the breakdown
// slice non-nil so an empty spine marshals it as `[]`, never `null`.
func spineScanDoc(res store.SpineScanResult) spineScanReportDoc {
	doc := spineScanReportDoc{
		Total:           res.Total,
		ScannedAt:       res.ScannedAt,
		Owners:          res.Owners,
		WithoutSummary:  res.WithoutSummary,
		WithSummary:     res.WithSummary,
		Superseded:      res.Superseded,
		Expired:         res.Expired,
		Scheduled:       res.Scheduled,
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

// spineScanSummary renders the operator-facing text report. Pure (value
// types only — no *store.Store, no context.Context) so it is
// unit-testable without a live Qdrant, mirroring reindexSummary's
// discipline (reindex.go). Two parts: a header line with Total, Owners and
// the scan instant, then one line per health signal, then the (scope,
// category) breakdown sorted as store.ScanSpine already returns it.
func spineScanSummary(res store.SpineScanResult, scope string) string {
	target := scope
	if target == "" {
		target = "all scopes"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "spine scan (%s) at %s: total=%d owners=%d\n",
		target, res.ScannedAt.Format(time.RFC3339), res.Total, res.Owners)
	fmt.Fprintf(&b, "  without_summary=%d with_summary=%d\n", res.WithoutSummary, res.WithSummary)
	fmt.Fprintf(&b, "  superseded=%d expired=%d scheduled=%d\n", res.Superseded, res.Expired, res.Scheduled)
	fmt.Fprintf(&b, "  with_citations=%d citations=%d\n", res.WithCitations, res.Citations)
	for _, c := range res.ByScopeCategory {
		fmt.Fprintf(&b, "  scope=%q category=%q count=%d\n", c.Scope, c.Category, c.Count)
	}
	return strings.TrimRight(b.String(), "\n")
}

func init() {
	addOperatorOutputFlag(spineReviewScanCmd, &spineScanOutput)
	spineReviewScanCmd.Flags().StringVar(&spineScanScope, "scope", "", "only scan records in this scope")
	spineReviewScanCmd.Flags().BoolVar(&spineScanAllScopes, "all-scopes", false, "sweep every scope (required if --scope is omitted)")
	spineReviewScanCmd.Flags().DurationVar(&spineScanTimeout, "timeout", 5*time.Minute,
		"max wall-clock for the sweep (0 disables); also cancellable via Ctrl-C")
	spineReviewCmd.AddCommand(spineReviewScanCmd)
}
