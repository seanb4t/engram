// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestNewToolMetricsRecordsWithoutPanic(t *testing.T) {
	t.Run("noop provider", func(_ *testing.T) {
		// With the global no-op MeterProvider, instruments are valid and record
		// silently — this proves the construction + record path is safe when
		// telemetry is disabled.
		m := NewToolMetrics(otel.Meter("test"))
		m.Record(context.Background(), "store_memory", "ok", 12.3)
		m.RecordAuthFailure(context.Background(), "unauthorized")
	})

	t.Run("real provider assertions", func(t *testing.T) {
		reader := metric.NewManualReader()
		mp := metric.NewMeterProvider(metric.WithReader(reader))
		m := NewToolMetrics(mp.Meter("test"))

		ctx := context.Background()
		m.Record(ctx, "store_memory", "ok", 12.3)
		m.RecordAuthFailure(ctx, "unauthorized")

		var rm metricdata.ResourceMetrics
		if err := reader.Collect(ctx, &rm); err != nil {
			t.Fatalf("Collect: %v", err)
		}

		byName := make(map[string]metricdata.Metrics)
		for _, sm := range rm.ScopeMetrics {
			for _, met := range sm.Metrics {
				byName[met.Name] = met
			}
		}

		// engram.tool.calls counter == 1 with tool/outcome attrs
		calls, ok := byName["engram.tool.calls"]
		if !ok {
			t.Fatal("engram.tool.calls not found")
		}
		sum, ok := calls.Data.(metricdata.Sum[int64])
		if !ok {
			t.Fatalf("engram.tool.calls data type: %T", calls.Data)
		}
		if len(sum.DataPoints) != 1 {
			t.Fatalf("engram.tool.calls data points: got %d want 1", len(sum.DataPoints))
		}
		if got := sum.DataPoints[0].Value; got != 1 {
			t.Errorf("engram.tool.calls value: got %d want 1", got)
		}
		attrs := sum.DataPoints[0].Attributes
		if v, _ := attrs.Value("tool"); v.AsString() != "store_memory" {
			t.Errorf("tool attr: got %q want store_memory", v.AsString())
		}
		if v, _ := attrs.Value("outcome"); v.AsString() != "ok" {
			t.Errorf("outcome attr: got %q want ok", v.AsString())
		}

		// engram.tool.duration histogram has 1 sample
		dur, ok := byName["engram.tool.duration"]
		if !ok {
			t.Fatal("engram.tool.duration not found")
		}
		hist, ok := dur.Data.(metricdata.Histogram[float64])
		if !ok {
			t.Fatalf("engram.tool.duration data type: %T", dur.Data)
		}
		if len(hist.DataPoints) != 1 {
			t.Fatalf("engram.tool.duration data points: got %d want 1", len(hist.DataPoints))
		}
		if got := hist.DataPoints[0].Count; got != 1 {
			t.Errorf("engram.tool.duration count: got %d want 1", got)
		}

		// engram.auth.failures == 1 with reason attr
		auth, ok := byName["engram.auth.failures"]
		if !ok {
			t.Fatal("engram.auth.failures not found")
		}
		authSum, ok := auth.Data.(metricdata.Sum[int64])
		if !ok {
			t.Fatalf("engram.auth.failures data type: %T", auth.Data)
		}
		if len(authSum.DataPoints) != 1 {
			t.Fatalf("engram.auth.failures data points: got %d want 1", len(authSum.DataPoints))
		}
		if got := authSum.DataPoints[0].Value; got != 1 {
			t.Errorf("engram.auth.failures value: got %d want 1", got)
		}
		authAttrs := authSum.DataPoints[0].Attributes
		if v, _ := authAttrs.Value("reason"); v.AsString() != "unauthorized" {
			t.Errorf("reason attr: got %q want unauthorized", v.AsString())
		}
	})
}

// collectStoreDuration reads the engram.store.duration histogram via a manual
// reader and returns its data points.
func collectStoreDuration(t *testing.T, rdr metric.Reader) []metricdata.HistogramDataPoint[float64] {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := rdr.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "engram.store.duration" {
				continue
			}
			h, ok := m.Data.(metricdata.Histogram[float64])
			if !ok {
				t.Fatalf("engram.store.duration is %T, want Histogram[float64]", m.Data)
			}
			return h.DataPoints
		}
	}
	return nil
}

func TestRecordStoreOpRecordsOperationAndOutcome(t *testing.T) {
	rdr := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(rdr))
	InitLayerMetrics(mp.Meter("test"))
	t.Cleanup(func() { layer = nil })

	RecordStoreOp(context.Background(), "Search", time.Now().Add(-5*time.Millisecond), nil)
	RecordStoreOp(context.Background(), "Upsert", time.Now(), errors.New("boom"))

	pts := collectStoreDuration(t, rdr)
	if len(pts) != 2 {
		t.Fatalf("got %d data points, want 2", len(pts))
	}
	got := map[string]string{}
	for _, p := range pts {
		op, _ := p.Attributes.Value("operation")
		out, _ := p.Attributes.Value("outcome")
		got[op.AsString()] = out.AsString()
	}
	if got["Search"] != "ok" {
		t.Errorf("Search outcome = %q, want ok", got["Search"])
	}
	if got["Upsert"] != "error" {
		t.Errorf("Upsert outcome = %q, want error", got["Upsert"])
	}
}

func TestRecordHelpersAreNilSafeWhenUninitialised(_ *testing.T) {
	layer = nil // telemetry disabled
	// must not panic
	RecordStoreOp(context.Background(), "Search", time.Now(), nil)
	RecordEmbed(context.Background(), time.Now(), nil)
	RecordAuthVerify(context.Background(), time.Now(), errors.New("x"))
}
