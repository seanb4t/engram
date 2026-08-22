// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/store"
)

// spineConsolidateDefaultTopK is consolidate's --top-k default, mirroring
// internal/store/spine.go's own defaultNearDuplicateTopK — kept as a
// separate CLI-side constant (that package's constant is unexported)
// rather than plumbing an exported default through store, since the two
// only need to agree on VALUE, not share an identifier.
const spineConsolidateDefaultTopK uint64 = 5

// spineConsolidateStore is the minimal store surface consolidate's RunE
// depends on — satisfied by *store.Store, and by a recording fake in
// spine_review_consolidate_test.go, so the --scope/--all-scopes/--top-k/
// --min-score-to-NearDuplicateOptions mapping is provable without dialing
// a live Qdrant.
type spineConsolidateStore interface {
	NearDuplicates(ctx context.Context, opts store.NearDuplicateOptions) ([]store.DuplicatePair, error)
}

// spineConsolidateStoreFromEnv constructs the store consolidate's RunE
// calls NearDuplicates on. A package-level var — mirroring
// citationFileReader's injection pattern (spine_review_verify.go) — so
// spine_review_consolidate_test.go can substitute a recording fake without
// dialing a live Qdrant. Returns a nil interface (not a non-nil interface
// wrapping a nil *store.Store) on error.
var spineConsolidateStoreFromEnv = func() (spineConsolidateStore, error) {
	st, err := server.StoreFromEnv()
	if err != nil {
		return nil, err
	}
	return st, nil
}

// parseMinScore parses --min-score's string value into a *float32: empty
// means nil (no filter). Registered as a STRING flag rather than a float
// flag specifically so this empty-means-unset state is representable —
// a float flag's zero value IS a real cosine threshold, and its DefValue
// would advertise "0" as though that were the contract, exactly the
// design D-15 and this plan's <planner_assumptions> forbid.
func parseMinScore(v string) (*float32, error) {
	if v == "" {
		return nil, nil
	}
	f, err := strconv.ParseFloat(v, 32)
	if err != nil {
		return nil, usageErrorf("--min-score %q is not a valid number: %v", v, err)
	}
	f32 := float32(f)
	return &f32, nil
}

var (
	spineConsolidateScope     string
	spineConsolidateAllScopes bool
	spineConsolidateTimeout   time.Duration
	spineConsolidateOutput    string
	spineConsolidateTopK      uint64
	spineConsolidateMinScore  string
)

// spineReviewConsolidateCmd reports ranked near-duplicate candidate pairs
// across the memory spine, using each record's already-stored vector.
// Read-only by construction — it never issues a mutating Qdrant RPC
// (T-03-16's mitigation) — and Subject-less like every other operator-tier
// command on this binary. Never clusters, never applies a default
// threshold, never labels a pair a "duplicate": the report ranks
// candidates and stops, per REQ-near-duplicate-report's transparency
// requirement — deciding whether two records are the same fact is a
// judgment the operator or a future semantic skill makes, not this
// command.
var spineReviewConsolidateCmd = &cobra.Command{
	Use:   "consolidate",
	Short: "Report ranked near-duplicate candidate pairs across the memory spine",
	RunE: func(cmd *cobra.Command, _ []string) error {
		format, err := operatorOutputFormat(cmd, spineConsolidateOutput)
		if err != nil {
			return err
		}
		minScore, err := parseMinScore(spineConsolidateMinScore)
		if err != nil {
			return err
		}
		st, err := spineConsolidateStoreFromEnv()
		if err != nil {
			return classifyOperatorErrConstruction(err)
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if spineConsolidateTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, spineConsolidateTimeout)
			defer cancel()
		}
		// Per-batch progress goes to stderr so it never pollutes the single
		// parseable result document on stdout (mirrors reindex.go's stream
		// discipline). The final scanned/queried counts are also carried
		// into both output forms below, so an operator can see the sweep's
		// size without having to have watched stderr live.
		var lastScanned, lastQueried uint64
		pairs, err := st.NearDuplicates(ctx, store.NearDuplicateOptions{
			Scope: spineConsolidateScope, AllScopes: spineConsolidateAllScopes,
			TopK: spineConsolidateTopK, MinScore: minScore,
			Progress: func(scanned, queried uint64) {
				lastScanned, lastQueried = scanned, queried
				cmd.PrintErrf("consolidate progress: scanned %d, queried %d\n", scanned, queried)
			},
		})
		if err != nil {
			return classifyOperatorErr(err)
		}
		text := consolidateSummary(pairs, spineConsolidateScope, spineConsolidateAllScopes, minScore, spineConsolidateTopK, lastScanned, lastQueried)
		return renderOperator(cmd, format,
			text, consolidateDoc(pairs, spineConsolidateScope, spineConsolidateAllScopes, minScore, spineConsolidateTopK, lastScanned, lastQueried))
	},
}

// consolidatePairDoc is one ranked candidate's JSON-mode shape: both
// records' identities and Score, printed as reported — never normalised,
// bucketed, or labelled a verdict. Never a record's content or summary
// text (T-03-05's sibling mitigation).
type consolidatePairDoc struct {
	A        string  `json:"a"`
	B        string  `json:"b"`
	AShortID string  `json:"a_short_id"`
	BShortID string  `json:"b_short_id"`
	AScope   string  `json:"a_scope"`
	BScope   string  `json:"b_scope"`
	Score    float32 `json:"score"`
}

// consolidateReportDoc is the JSON-mode report's exported-field shape.
// MinScore is a pointer so JSON mode can distinguish "no filter was
// applied" (field absent, via omitempty on a nil pointer) from "filtered
// at exactly 0" (field present with value 0) — the same distinction
// NearDuplicateOptions.MinScore exists to make representable at the store
// layer. Under D-01 (06-CONTEXT.md) this struct is the sole
// serialization, so that absent key IS the "no filter applied" signal in
// BOTH lanes together; the human-facing explanation of what the absence
// means lives in consolidateSummary's headline, never as a second
// rendering rule here.
type consolidateReportDoc struct {
	Scope      string               `json:"scope"`
	AllScopes  bool                 `json:"all_scopes"`
	TopK       uint64               `json:"top_k"`
	MinScore   *float32             `json:"min_score,omitempty"`
	Scanned    uint64               `json:"scanned"`
	Queried    uint64               `json:"queried"`
	Candidates []consolidatePairDoc `json:"candidates"`
}

// consolidateDoc converts pairs into consolidateReportDoc, keeping
// Candidates non-nil so a zero-candidate report marshals it as "[]",
// never "null".
func consolidateDoc(pairs []store.DuplicatePair, scope string, allScopes bool, minScore *float32, topK, scanned, queried uint64) consolidateReportDoc {
	doc := consolidateReportDoc{
		Scope: scope, AllScopes: allScopes, TopK: topK, MinScore: minScore,
		Scanned: scanned, Queried: queried,
		Candidates: make([]consolidatePairDoc, 0, len(pairs)),
	}
	for _, p := range pairs {
		doc.Candidates = append(doc.Candidates, consolidatePairDoc{
			A: p.A, B: p.B, AShortID: p.AShortID, BShortID: p.BShortID,
			AScope: p.AScope, BScope: p.BScope, Score: p.Score,
		})
	}
	return doc
}

// consolidateSummary renders the operator-facing headline. Pure (value
// types only — no *store.Store, no context.Context) so it is unit-testable
// without a live Qdrant, mirroring reindexSummary's/spineScanSummary's
// discipline. Per D-04 (06-CONTEXT.md) this is a headline producer only:
// states plainly that these are ranked candidates, never duplicates, and
// that consolidate never merges or mutates — the report's meaning is
// stated in the output itself, not just in the CLI guide. Returns a
// single line with no trailing newline; the per-pair rows this function
// used to also print now render only as renderOperatorView's field-table
// rows, from consolidateDoc.
//
// When minScore is nil, the headline also states that no min_score filter
// was applied and that negative-scoring pairs are therefore reported —
// preserving the nuance the deleted min_score line used to carry. When
// minScore is non-nil, the headline says nothing about it at all: the
// threshold now lives solely in the min_score json key
// (consolidateReportDoc.MinScore's pointer-plus-omitempty design already
// makes the key's presence-or-absence the signal in both lanes, per D-01
// — restating the value in the headline would be a second rendering
// rule).
func consolidateSummary(pairs []store.DuplicatePair, scope string, allScopes bool, minScore *float32, topK, scanned, queried uint64) string {
	target := fmt.Sprintf("scope=%q", scope)
	if allScopes {
		target = "all scopes"
	}
	headline := fmt.Sprintf("spine consolidate (%s) top_k=%d scanned=%d queried=%d: %d ranked candidate(s) — "+
		"NOT duplicates, never merged or mutated; a pair may cross scopes", target, topK, scanned, queried, len(pairs))
	if minScore == nil {
		headline += " — no min_score filter applied; negative-scoring pairs are reported"
	}
	return headline
}

func init() {
	addOperatorOutputFlag(spineReviewConsolidateCmd, &spineConsolidateOutput)
	spineReviewConsolidateCmd.Flags().StringVar(&spineConsolidateScope, "scope", "", "only consider records in this scope")
	spineReviewConsolidateCmd.Flags().BoolVar(&spineConsolidateAllScopes, "all-scopes", false,
		"span every scope; a candidate pair may then cross scopes, and each row names both (mutually exclusive with --scope)")
	spineReviewConsolidateCmd.Flags().DurationVar(&spineConsolidateTimeout, "timeout", 5*time.Minute,
		"max wall-clock for the sweep (0 disables); also cancellable via Ctrl-C")
	spineReviewConsolidateCmd.Flags().Uint64Var(&spineConsolidateTopK, "top-k", spineConsolidateDefaultTopK,
		"neighbours considered per record")
	spineReviewConsolidateCmd.Flags().StringVar(&spineConsolidateMinScore, "min-score", "",
		"minimum cosine score a pair must carry to be reported; absent (the default) means NO filter at all — "+
			"including pairs with a negative score")
	spineReviewConsolidateCmd.MarkFlagsMutuallyExclusive("scope", "all-scopes")
	spineReviewCmd.AddCommand(spineReviewConsolidateCmd)
}
