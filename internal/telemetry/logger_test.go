// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewLoggerWritesJSONToStdout(t *testing.T) {
	var buf bytes.Buffer
	lg := newLoggerTo(&buf, Config{LogLevel: "info", LogFormat: "json", LogStdout: true}, nil)
	lg.Info("hello", "tool", "store_memory")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("expected JSON line, got %q (%v)", buf.String(), err)
	}
	if rec["msg"] != "hello" || rec["tool"] != "store_memory" {
		t.Errorf("missing fields: %v", rec)
	}
}

func TestNewLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	lg := newLoggerTo(&buf, Config{LogLevel: "warn", LogFormat: "json", LogStdout: true}, nil)
	lg.Info("suppressed")
	lg.Warn("shown")
	if strings.Contains(buf.String(), "suppressed") {
		t.Error("info should be filtered at warn level")
	}
	if !strings.Contains(buf.String(), "shown") {
		t.Error("warn should pass")
	}
}

func TestSilentProcessGuardForcesStdout(t *testing.T) {
	// stdout disabled AND no OTLP provider => must force stdout on, not go silent.
	var buf bytes.Buffer
	cfg := Config{LogLevel: "info", LogFormat: "json", LogStdout: false} // no endpoint => not enabled
	lg := newLoggerTo(&buf, cfg, nil)
	lg.Warn("must appear")
	if !strings.Contains(buf.String(), "must appear") {
		t.Error("guard must keep stdout when no log sink would otherwise exist")
	}
}
