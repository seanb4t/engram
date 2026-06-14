// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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
	// Shared exporter option values — the three exporter blocks below must stay
	// value-identical; update this const and the RetryConfig together.
	const exportTimeout = 500 * time.Millisecond
	insecure := os.Getenv("OTEL_EXPORTER_OTLP_INSECURE") == "true"

	// boot is the configured stdout logger (no OTLP bridge yet — providers are
	// being built). It carries any partial-resource warning on the reliable
	// stdout channel, honouring ENGRAM_LOG_LEVEL/FORMAT, before slog.SetDefault runs.
	boot := NewLogger(cfg, nil)
	res := buildResource(ctx, cfg, boot)

	traceOpts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithTimeout(exportTimeout),
		otlptracegrpc.WithRetry(otlptracegrpc.RetryConfig{Enabled: false}),
	}
	if insecure {
		traceOpts = append(traceOpts, otlptracegrpc.WithInsecure())
	}
	traceExp, err := otlptracegrpc.New(ctx, traceOpts...)
	if err != nil {
		return nil, nil, err
	}
	tp := trace.NewTracerProvider(trace.WithBatcher(traceExp), trace.WithResource(res))

	metricOpts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlpmetricgrpc.WithTimeout(exportTimeout),
		otlpmetricgrpc.WithRetry(otlpmetricgrpc.RetryConfig{Enabled: false}),
	}
	if insecure {
		metricOpts = append(metricOpts, otlpmetricgrpc.WithInsecure())
	}
	metricExp, err := otlpmetricgrpc.New(ctx, metricOpts...)
	if err != nil {
		return nil, nil, err
	}
	mp := metric.NewMeterProvider(metric.WithReader(metric.NewPeriodicReader(metricExp)), metric.WithResource(res))

	logOpts := []otlploggrpc.Option{
		otlploggrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlploggrpc.WithTimeout(exportTimeout),
		otlploggrpc.WithRetry(otlploggrpc.RetryConfig{Enabled: false}),
	}
	if insecure {
		logOpts = append(logOpts, otlploggrpc.WithInsecure())
	}
	logExp, err := otlploggrpc.New(ctx, logOpts...)
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

// buildResource assembles the OTel resource from the full idiomatic detector
// set plus engram's service identity. resource.New is opt-in per detector and
// does NOT include resource.Default(), so every standard attribute group is
// requested explicitly. WithFromEnv is first so OTEL_RESOURCE_ATTRIBUTES /
// OTEL_SERVICE_NAME are honoured; WithAttributes is last so engram's
// service.name/version/instance.id win on conflict. WithAttributes is schemaless,
// so it never conflicts with the detectors' bundled semconv schema URL.
func buildResource(ctx context.Context, cfg Config, lg *slog.Logger) *resource.Resource {
	return resourceFromOptions(ctx, lg,
		resource.WithFromEnv(),      // OTEL_RESOURCE_ATTRIBUTES + OTEL_SERVICE_NAME
		resource.WithTelemetrySDK(), // telemetry.sdk.name|language|version
		// WithProcess captures ALL of os.Args onto process.command_args, exported
		// on every signal. engram's flags are non-secret today; if a future flag
		// carries a token/key/secret path, swap this for WithProcessRuntimeName()
		// + WithProcessRuntimeVersion() (which omit command_args).
		resource.WithProcess(),   // process.pid, process.executable.*, process.runtime.*, process.owner, process.command_args
		resource.WithHost(),      // host.name
		resource.WithHostID(),    // host.id
		resource.WithOS(),        // os.type, os.description
		resource.WithContainer(), // container.id (docker/k8s cgroup)
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("service.instance.id", uuid.New().String()),
		),
	)
}

// resourceFromOptions builds an OTel resource, tolerating partial detection
// failures. resource.New ALWAYS returns a usable, non-nil Resource: it merges
// every detector that succeeds and skips only the one that failed, so the
// surviving attributes (service.name/version/instance.id, process, os,
// container, …) are still present. Resource detection is therefore best-effort
// *metadata* and must NEVER disable telemetry export (ADR engram-uxh).
//
// The tolerance cannot rely on errors.Is alone: WithHostID on a distroless
// image (no /etc/machine-id) returns a PLAIN error, not the documented
// resource.ErrPartialResource sentinel (only WithFromEnv wraps that sentinel).
// That plain error — propagated as fatal — was issue #102: a single optional
// host.id detector took down traces + metrics together. So this swallows every
// detection error and returns the partial resource regardless.
//
// The error is logged through lg, the caller's bootstrap stdout logger, NOT
// the slog default: at telemetry.Setup time slog.SetDefault has not run (it
// runs later in serve.go), so a bare slog.Warn would bypass ENGRAM_LOG_LEVEL/FORMAT.
func resourceFromOptions(ctx context.Context, lg *slog.Logger, opts ...resource.Option) *resource.Resource {
	res, err := resource.New(ctx, opts...)
	if err != nil {
		// No span at startup, so the context-less Warn is correct
		// (logger.go traceContextHandler convention).
		lg.Warn("partial telemetry resource; exporting with detected attributes", "err", err)
	}
	return res
}
