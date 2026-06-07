// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
)

func TestNewToolMetricsRecordsWithoutPanic(_ *testing.T) {
	// With the global no-op MeterProvider, instruments are valid and record
	// silently — this proves the construction + record path is safe when
	// telemetry is disabled.
	m := NewToolMetrics(otel.Meter("test"))
	m.Record(context.Background(), "store_memory", "ok", 12.3)
	m.RecordAuthFailure(context.Background(), "unauthorized")
}
