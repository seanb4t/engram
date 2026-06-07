// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package telemetry owns engram's structured logger and OpenTelemetry SDK
// bootstrap. Telemetry export is enabled only when an OTLP endpoint is set;
// otherwise providers are no-ops and logs still go to stdout.
package telemetry

import "os"

// Config controls logging and OTLP export. It is env-first, matching engram's
// no-viper convention: OTEL_* vars are also read natively by the exporters.
type Config struct {
	ServiceName    string
	ServiceVersion string
	OTLPEndpoint   string // OTEL_EXPORTER_OTLP_ENDPOINT; empty disables export
	LogLevel       string // debug|info|warn|error
	LogFormat      string // json|text
	LogStdout      bool   // also write logs to stdout
}

// Enabled reports whether OTLP export should be wired up.
func (c Config) Enabled() bool { return c.OTLPEndpoint != "" }

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// ConfigFromEnv builds a Config from the environment. serviceName/serviceVersion
// are passed in (the version is ldflags-injected into main, not an env var).
func ConfigFromEnv(serviceName, serviceVersion string) Config {
	return Config{
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		OTLPEndpoint:   os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		LogLevel:       envOr("MEM_LOG_LEVEL", "info"),
		LogFormat:      envOr("MEM_LOG_FORMAT", "json"),
		LogStdout:      envOr("MEM_LOG_STDOUT", "true") != "false",
	}
}
