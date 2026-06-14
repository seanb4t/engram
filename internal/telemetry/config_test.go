// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import "testing"

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317")
	t.Setenv("ENGRAM_LOG_LEVEL", "debug")
	t.Setenv("ENGRAM_LOG_FORMAT", "text")
	t.Setenv("ENGRAM_LOG_STDOUT", "false")

	c := ConfigFromEnv("engram", "1.2.3")

	if c.ServiceName != "engram" || c.ServiceVersion != "1.2.3" {
		t.Fatalf("service identity: got %q/%q", c.ServiceName, c.ServiceVersion)
	}
	if c.OTLPEndpoint != "otel-collector:4317" {
		t.Errorf("endpoint: got %q", c.OTLPEndpoint)
	}
	if c.LogLevel != "debug" || c.LogFormat != "text" || c.LogStdout {
		t.Errorf("log cfg: got level=%q format=%q stdout=%v", c.LogLevel, c.LogFormat, c.LogStdout)
	}
}

func TestConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("ENGRAM_LOG_LEVEL", "")
	t.Setenv("ENGRAM_LOG_FORMAT", "")
	t.Setenv("ENGRAM_LOG_STDOUT", "")

	c := ConfigFromEnv("engram", "dev")

	if c.OTLPEndpoint != "" {
		t.Errorf("endpoint default should be empty, got %q", c.OTLPEndpoint)
	}
	if c.LogLevel != "info" || c.LogFormat != "json" || !c.LogStdout {
		t.Errorf("defaults: got level=%q format=%q stdout=%v", c.LogLevel, c.LogFormat, c.LogStdout)
	}
	if c.Enabled() {
		t.Error("Enabled() should be false with no endpoint")
	}
}
