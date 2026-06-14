// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package telemetry owns engram's structured logger and OpenTelemetry SDK
// bootstrap. Telemetry export is enabled only when an OTLP endpoint is set;
// otherwise providers are no-ops and logs still go to stdout.
package telemetry

import (
	"os"

	"github.com/seanb4t/engram/internal/config"
)

// Config controls logging and OTLP export. It is env-first, matching engram's
// no-viper convention: ENGRAM_LOG_* vars are read via internal/config;
// OTEL_* vars are also read natively by the exporters.
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

// ConfigFromEnv builds a Config from the environment. serviceName/serviceVersion
// are passed in (the version is ldflags-injected into main, not an env var). Log
// fields come from internal/config (ENGRAM_LOG_*); the OTLP endpoint is read
// natively (OTEL_* vars are consumed directly by the exporters).
func ConfigFromEnv(serviceName, serviceVersion string) Config {
	c, err := config.Load(nil)
	if err != nil {
		panic("telemetry config load: " + err.Error()) // static registry: cannot fail
	}
	return Config{
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		OTLPEndpoint:   os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		LogLevel:       c.Log.Level,
		LogFormat:      c.Log.Format,
		LogStdout:      c.Log.Stdout != "false",
	}
}
