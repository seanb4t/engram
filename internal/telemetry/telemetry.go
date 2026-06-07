// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"log/slog"
)

// ShutdownFunc flushes and closes all telemetry providers. Safe to call once;
// the no-op variant is safe to call repeatedly.
type ShutdownFunc func(context.Context) error

func noopShutdown(context.Context) error { return nil }

// buildProvidersFn is the production implementation; replaced in tests to inject
// a failing stub without touching the real exporter constructors.
var buildProvidersFn = buildProviders

// Setup constructs the logger and (when enabled) the OTel trace, metric, and
// log providers, registers them as globals, and returns a shutdown that flushes
// all three. When telemetry is disabled (no OTLP endpoint) it returns a stdout
// logger and a no-op shutdown. When provider construction fails it falls back to
// a stdout logger and no-op shutdown rather than propagating the error.
//
// Setup never returns a non-nil error. Telemetry is not a hard startup
// dependency (ADR engram-uxh); all failure modes degrade gracefully to stdout
// logging. The error return is kept in the signature so the API can evolve
// without breaking callers.
func Setup(ctx context.Context, cfg Config) (*slog.Logger, ShutdownFunc, error) {
	if !cfg.Enabled() {
		lg := NewLogger(cfg, nil)
		if !cfg.LogStdout {
			lg.Warn("MEM_LOG_STDOUT=false but no OTLP endpoint configured; forcing stdout so logs are not silently dropped")
		}
		return lg, noopShutdown, nil
	}
	lp, shutdown, err := buildProvidersFn(ctx, cfg)
	if err != nil {
		// Telemetry must never be a hard startup dependency: fall back to stdout.
		lg := NewLogger(cfg, nil)
		lg.Warn("telemetry setup failed; continuing with stdout logging only", "err", err)
		return lg, noopShutdown, nil
	}
	return NewLogger(cfg, lp), shutdown, nil
}
