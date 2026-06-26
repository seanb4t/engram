// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/seanb4t/engram/internal/telemetry"
)

// SummarizeFunc compresses content into a one-line summary. Injected so the
// store never imports the summarizer package (matches Reindex's EmbedFunc).
type SummarizeFunc func(ctx context.Context, content string) (string, error)

// SummarizeOptions bounds a summarize-missing sweep.
type SummarizeOptions struct {
	Scope     string    // "" with AllScopes=false is a no-op guard (CLI requires one)
	AllScopes bool      // true sweeps every scope
	OlderThan time.Time // zero = no age filter; else only created_at before it
	Limit     int       // 0 = no cap on records scanned
	MaxChars  int       // eligibility threshold + summary cap
	Model     string    // stamped as summary_model on filled records
	DryRun    bool      // count eligible records without writing
}

// SummarizeResult is the operator-facing tally of a sweep.
type SummarizeResult struct {
	Scanned, Filled, Skipped, Failed int
}

// shouldSummarize is the per-record eligibility rule: no existing summary and a
// content body longer than the cap (short content is already recall-cheap).
func shouldSummarize(m Memory, maxChars int) bool {
	return m.Summary == "" && len([]rune(m.Content)) > maxChars
}

// SetSummary writes summary + provenance via a vector-preserving SetPayload
// (mirrors SetVisibility). Used by the auto path; always stamps source "auto".
func (s *Store) SetSummary(ctx context.Context, id, summary, model string) (err error) {
	ctx, span := tracer.Start(ctx, "store.SetSummary")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "SetSummary", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	_, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload: qdrant.NewValueMap(map[string]any{
			"summary": summary, "summary_source": "auto", "summary_model": model,
		}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	})
	return err
}

// FillSummary summarizes one record and persists it, idempotently. Returns
// filled=false (no error) when the record is ineligible. It is the reusable
// per-record unit the sweep below builds on.
func (s *Store) FillSummary(ctx context.Context, m Memory, summarize SummarizeFunc, model string, maxChars int) (filled bool, err error) {
	if !shouldSummarize(m, maxChars) {
		return false, nil
	}
	sum, err := summarize(ctx, m.Content)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(sum) == "" {
		return false, fmt.Errorf("summarize %s: empty summary", m.ID)
	}
	if err := s.SetSummary(ctx, m.ID, sum, model); err != nil {
		return false, err
	}
	return true, nil
}

// SummarizeMissing scrolls records (optionally scoped) and fills empty summaries
// best-effort. created_at age filtering is applied in-code (created_at is stored
// as an RFC3339 string, not a Qdrant-rangeable number). Per-record errors are
// counted, not fatal.
func (s *Store) SummarizeMissing(ctx context.Context, opts SummarizeOptions, summarize SummarizeFunc) (res SummarizeResult, err error) {
	ctx, span := tracer.Start(ctx, "store.SummarizeMissing")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "SummarizeMissing", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int("engram.result_count", res.Filled))
		}
	}()

	var must []*qdrant.Condition
	if opts.Scope != "" {
		must = append(must, qdrant.NewMatch("scope", opts.Scope))
	}
	var filter *qdrant.Filter
	if len(must) > 0 {
		filter = &qdrant.Filter{Must: must}
	}

	var offset *qdrant.PointId
	for {
		pts, next, serr := s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
			CollectionName: s.collection,
			Filter:         filter,
			Limit:          qdrant.PtrOf(uint32(256)),
			Offset:         offset,
			WithPayload:    qdrant.NewWithPayload(true),
		})
		if serr != nil {
			return res, serr
		}
		for _, p := range pts {
			if opts.Limit > 0 && res.Scanned >= opts.Limit {
				return res, nil
			}
			res.Scanned++
			m := fromPayload(p.Id.GetUuid(), p.Payload)
			if !opts.OlderThan.IsZero() && !m.CreatedAt.Before(opts.OlderThan) {
				res.Skipped++
				continue
			}
			if !shouldSummarize(m, opts.MaxChars) {
				res.Skipped++
				continue
			}
			if opts.DryRun {
				res.Filled++ // "would fill"
				continue
			}
			if _, ferr := s.FillSummary(ctx, m, summarize, opts.Model, opts.MaxChars); ferr != nil {
				slog.WarnContext(ctx, "summarize-missing: fill failed", "id", m.ID, "err", ferr)
				res.Failed++
				continue
			}
			res.Filled++
		}
		if next == nil {
			return res, nil
		}
		offset = next
	}
}
