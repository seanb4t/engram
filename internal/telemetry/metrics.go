// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ToolMetrics holds the engram-specific instruments. Built from the global
// meter; records are no-ops when telemetry is disabled.
type ToolMetrics struct {
	calls        metric.Int64Counter
	duration     metric.Float64Histogram
	authFailures metric.Int64Counter
}

// NewToolMetrics constructs the instruments. Instrument creation errors (only
// possible on invalid names, which are constant here) are ignored: a nil
// instrument from the no-op provider still records safely.
func NewToolMetrics(m metric.Meter) *ToolMetrics {
	calls, _ := m.Int64Counter("engram.tool.calls")
	dur, _ := m.Float64Histogram("engram.tool.duration",
		metric.WithUnit("ms"), metric.WithDescription("tool handler latency"))
	auth, _ := m.Int64Counter("engram.auth.failures")
	return &ToolMetrics{calls: calls, duration: dur, authFailures: auth}
}

// Record logs one tool call's count and latency.
func (t *ToolMetrics) Record(ctx context.Context, tool, outcome string, ms float64) {
	attrs := metric.WithAttributes(attribute.String("tool", tool), attribute.String("outcome", outcome))
	t.calls.Add(ctx, 1, attrs)
	t.duration.Record(ctx, ms, attrs)
}

// RecordAuthFailure counts a rejected request by reason.
func (t *ToolMetrics) RecordAuthFailure(ctx context.Context, reason string) {
	t.authFailures.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", reason)))
}

// layerMetrics holds the per-operation latency instruments for the store,
// embed, and auth seams. It is package state so the Record* helpers below can
// be called from internal/store|embed|auth WITHOUT those packages importing the
// meter or threading an instrument handle through every call. The dependency is
// one-way: those packages import telemetry; telemetry imports none of them.
type layerMetrics struct {
	storeDur metric.Float64Histogram
	embedDur metric.Float64Histogram
	authDur  metric.Float64Histogram
}

// layer is nil until InitLayerMetrics runs (telemetry disabled => helpers no-op).
var layer *layerMetrics

// InitLayerMetrics builds the store/embed/auth latency histograms from m and
// installs them as package state. Called once from serve.go after the meter
// provider is registered. Instrument-creation errors are ignored: a nil
// instrument from the no-op provider still records safely.
func InitLayerMetrics(m metric.Meter) {
	storeDur, _ := m.Float64Histogram("engram.store.duration",
		metric.WithUnit("ms"), metric.WithDescription("store operation latency"))
	embedDur, _ := m.Float64Histogram("engram.embed.duration",
		metric.WithUnit("ms"), metric.WithDescription("embedder call latency"))
	authDur, _ := m.Float64Histogram("engram.auth.verify.duration",
		metric.WithUnit("ms"), metric.WithDescription("token verification latency"))
	layer = &layerMetrics{storeDur: storeDur, embedDur: embedDur, authDur: authDur}
}

func outcomeOf(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

func msSince(start time.Time) float64 {
	return float64(time.Since(start).Microseconds()) / 1000.0
}

// RecordStoreOp records one store method's latency keyed by operation+outcome.
func RecordStoreOp(ctx context.Context, op string, start time.Time, err error) {
	if layer == nil {
		return
	}
	layer.storeDur.Record(ctx, msSince(start), metric.WithAttributes(
		attribute.String("operation", op), attribute.String("outcome", outcomeOf(err))))
}

// RecordEmbed records one embedder call's latency keyed by outcome.
func RecordEmbed(ctx context.Context, start time.Time, err error) {
	if layer == nil {
		return
	}
	layer.embedDur.Record(ctx, msSince(start),
		metric.WithAttributes(attribute.String("outcome", outcomeOf(err))))
}

// RecordAuthVerify records one token verification's latency keyed by outcome.
func RecordAuthVerify(ctx context.Context, start time.Time, err error) {
	if layer == nil {
		return
	}
	layer.authDur.Record(ctx, msSince(start),
		metric.WithAttributes(attribute.String("outcome", outcomeOf(err))))
}

// SummaryQueueMetrics holds the STATIC instruments for the async-on-write
// summary worker: per-outcome counters plus a fill-latency histogram. Built
// once from the global meter in serve.go, mirroring NewToolMetrics.
//
// This type deliberately does NOT own the queue-depth observable gauge — see
// RegisterSummaryQueueDepth below for why that instrument is registered
// separately, later, from buildDepsFromEnv (Wave 3).
type SummaryQueueMetrics struct {
	enqueued metric.Int64Counter
	dropped  metric.Int64Counter
	failed   metric.Int64Counter
	retried  metric.Int64Counter
	fillDur  metric.Float64Histogram
}

// NewSummaryQueueMetrics constructs the static enqueue/drop/fail/retry
// counters and the fill-latency histogram from m. It takes ONLY the meter —
// no depth callback — because at the point this is called (serve.go startup,
// before the queue exists) there is no live queue to sample. Instrument
// creation errors are ignored: a nil instrument from the no-op provider still
// records safely (same idiom as NewToolMetrics).
func NewSummaryQueueMetrics(m metric.Meter) *SummaryQueueMetrics {
	enqueued, _ := m.Int64Counter("engram.summary_queue.enqueued")
	dropped, _ := m.Int64Counter("engram.summary_queue.dropped")
	failed, _ := m.Int64Counter("engram.summary_queue.failed")
	retried, _ := m.Int64Counter("engram.summary_queue.retried")
	fillDur, _ := m.Float64Histogram("engram.summary_queue.fill.duration",
		metric.WithUnit("s"), metric.WithDescription("async summary fill latency"))
	return &SummaryQueueMetrics{enqueued: enqueued, dropped: dropped, failed: failed, retried: retried, fillDur: fillDur}
}

// Enqueued counts one record successfully enqueued for async summary fill.
func (s *SummaryQueueMetrics) Enqueued(ctx context.Context) {
	s.enqueued.Add(ctx, 1)
}

// Dropped counts one record that could not be enqueued (queue full).
func (s *SummaryQueueMetrics) Dropped(ctx context.Context) {
	s.dropped.Add(ctx, 1)
}

// Failed counts one fill that exhausted its retry budget without success.
func (s *SummaryQueueMetrics) Failed(ctx context.Context) {
	s.failed.Add(ctx, 1)
}

// Retried counts one fill attempt that failed and was retried.
func (s *SummaryQueueMetrics) Retried(ctx context.Context) {
	s.retried.Add(ctx, 1)
}

// RecordFill records one fill attempt's latency in seconds.
func (s *SummaryQueueMetrics) RecordFill(ctx context.Context, seconds float64) {
	s.fillDur.Record(ctx, seconds)
}

// RegisterSummaryQueueDepth registers the async summary queue's depth as an
// Int64ObservableGauge, sampled via depth on each collection pass.
//
// This is a STANDALONE helper, deliberately separate from
// NewSummaryQueueMetrics: it must be called AFTER the summary queue is
// constructed (from buildDepsFromEnv in Wave 3), because only there does the
// depth() closure have a live queue (e.g. len(ch)) to read. Calling it from
// serve.go — where NewSummaryQueueMetrics runs — would have nothing to close
// over, since the queue does not exist yet at that point in startup. Both
// symbols' doc comments describe this split so 11-01 (static instruments)
// and 11-03 (depth gauge wiring) stay compile-consistent.
func RegisterSummaryQueueDepth(m metric.Meter, depth func() int64) {
	_, _ = m.Int64ObservableGauge("engram.summary_queue.depth",
		metric.WithUnit("{item}"),
		metric.WithDescription("async summary queue depth"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			o.Observe(depth())
			return nil
		}),
	)
}
