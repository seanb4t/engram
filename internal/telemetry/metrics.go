// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"

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
