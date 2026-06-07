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

// Setup constructs the logger and (when enabled) the OTel providers, registers
// them as globals, and returns a shutdown that flushes them. When telemetry is
// disabled (no OTLP endpoint) it returns a stdout logger and a no-op shutdown.
//
// Phase 1: providers are not yet built; only the logger is wired. Phase 2
// replaces the body below with real provider construction behind this same
// signature, so callers never change.
func Setup(ctx context.Context, cfg Config) (*slog.Logger, ShutdownFunc, error) {
	if !cfg.Enabled() {
		lg := NewLogger(cfg, nil)
		if !cfg.LogStdout {
			lg.Warn("MEM_LOG_STDOUT=false but no OTLP endpoint configured; forcing stdout so logs are not silently dropped")
		}
		return lg, noopShutdown, nil
	}
	lp, shutdown, err := buildProviders(ctx, cfg)
	if err != nil {
		// Telemetry must never be a hard startup dependency: fall back to stdout.
		lg := NewLogger(cfg, nil)
		lg.Warn("telemetry setup failed; continuing with stdout logging only", "err", err)
		return lg, noopShutdown, nil
	}
	return NewLogger(cfg, lp), shutdown, nil
}
