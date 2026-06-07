// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"errors"
	"testing"
	"time"

	otellog "go.opentelemetry.io/otel/log"
)

func TestSetupEnabledBuildsProvidersAndShutsDown(t *testing.T) {
	// A syntactically valid endpoint; otlptracegrpc.New does not dial eagerly,
	// so construction succeeds without a live collector.
	cfg := Config{ServiceName: "engram", ServiceVersion: "test",
		OTLPEndpoint: "localhost:4317", LogLevel: "info", LogFormat: "json", LogStdout: true}
	lg, shutdown, err := Setup(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Setup(enabled) error: %v", err)
	}
	if lg == nil {
		t.Fatal("logger must be non-nil")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}

func TestSetupBuildProvidersFallbackOnError(t *testing.T) {
	// Swap in a failing stub to exercise the error-fallback path without a
	// live OTLP endpoint.
	orig := buildProvidersFn
	t.Cleanup(func() { buildProvidersFn = orig })
	buildProvidersFn = func(_ context.Context, _ Config) (otellog.LoggerProvider, ShutdownFunc, error) {
		return nil, nil, errors.New("injected provider failure")
	}

	cfg := Config{ServiceName: "engram", ServiceVersion: "test",
		OTLPEndpoint: "localhost:4317", LogLevel: "info", LogFormat: "json", LogStdout: true}
	lg, shutdown, err := Setup(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Setup must never return a non-nil error; got %v", err)
	}
	if lg == nil {
		t.Fatal("Setup must return a usable logger even when provider build fails")
	}
	// shutdown must be the no-op variant — safe to call repeatedly.
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("fallback shutdown must be nil, got %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("second call to fallback shutdown must be nil, got %v", err)
	}
}

func TestSetupDisabledReturnsLoggerAndNoopShutdown(t *testing.T) {
	cfg := Config{ServiceName: "engram", ServiceVersion: "test",
		LogLevel: "info", LogFormat: "json", LogStdout: true} // no endpoint
	lg, shutdown, err := Setup(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Setup returned error when disabled: %v", err)
	}
	if lg == nil {
		t.Fatal("Setup must always return a usable logger")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("noop shutdown should be nil, got %v", err)
	}
	// idempotent
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("second shutdown should be nil, got %v", err)
	}
}
