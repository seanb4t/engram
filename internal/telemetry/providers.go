// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otellog "go.opentelemetry.io/otel/log"
	logglobal "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// buildProviders constructs the trace, metric, and log providers wired to OTLP
// gRPC exporters, registers them as OTel globals, and returns the log provider
// (for the otelslog bridge) plus a shutdown that flushes all three.
func buildProviders(ctx context.Context, cfg Config) (otellog.LoggerProvider, ShutdownFunc, error) {
	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
	))
	if err != nil {
		return nil, nil, err
	}

	traceExp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithTimeout(500*time.Millisecond),
		otlptracegrpc.WithRetry(otlptracegrpc.RetryConfig{Enabled: false}),
	)
	if err != nil {
		return nil, nil, err
	}
	tp := trace.NewTracerProvider(trace.WithBatcher(traceExp), trace.WithResource(res))

	metricExp, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithTimeout(500*time.Millisecond),
		otlpmetricgrpc.WithRetry(otlpmetricgrpc.RetryConfig{Enabled: false}),
	)
	if err != nil {
		return nil, nil, err
	}
	mp := metric.NewMeterProvider(metric.WithReader(metric.NewPeriodicReader(metricExp)), metric.WithResource(res))

	logExp, err := otlploggrpc.New(ctx,
		otlploggrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlploggrpc.WithInsecure(),
		otlploggrpc.WithTimeout(500*time.Millisecond),
		otlploggrpc.WithRetry(otlploggrpc.RetryConfig{Enabled: false}),
	)
	if err != nil {
		return nil, nil, err
	}
	lp := log.NewLoggerProvider(log.WithProcessor(log.NewBatchProcessor(logExp)), log.WithResource(res))

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	logglobal.SetLoggerProvider(lp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{}))

	shutdown := func(ctx context.Context) error {
		// Best-effort flush: telemetry is never a hard shutdown dependency
		// (ADR engram-uxh), so a failed flush (e.g. an unreachable collector)
		// is logged for the operator but never propagated. The bounded
		// per-export timeout (WithTimeout/no-retry above) keeps this fast.
		for _, fn := range []func(context.Context) error{
			tp.Shutdown, mp.Shutdown, lp.Shutdown,
		} {
			if err := fn(ctx); err != nil {
				slog.Warn("telemetry shutdown flush failed (best-effort)", "err", err)
			}
		}
		return nil
	}
	return lp, shutdown, nil
}
