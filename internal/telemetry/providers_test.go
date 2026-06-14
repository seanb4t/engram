// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestBuildResourceHasStandardAttributes(t *testing.T) {
	res := buildResource(context.Background(), Config{ServiceName: "engram", ServiceVersion: "1.2.3"}, discardLogger())
	got := map[string]string{}
	for _, kv := range res.Attributes() {
		got[string(kv.Key)] = kv.Value.String()
	}
	for _, key := range []string{
		"service.name", "service.version", "service.instance.id",
		"telemetry.sdk.name", "telemetry.sdk.language", "telemetry.sdk.version",
		"process.runtime.name", "os.type",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing resource attribute %q", key)
		}
	}
	if got["service.name"] != "engram" {
		t.Errorf("service.name = %q, want engram", got["service.name"])
	}
	if got["service.version"] != "1.2.3" {
		t.Errorf("service.version = %q, want 1.2.3", got["service.version"])
	}
}

// failingDetector mimics an optional resource detector that cannot resolve its
// source — e.g. resource.WithHostID() on a distroless image with no
// /etc/machine-id, which returns a PLAIN error (not resource.ErrPartialResource).
type failingDetector struct{ err error }

func (d failingDetector) Detect(context.Context) (*resource.Resource, error) {
	return nil, d.err
}

// TestResourceFromOptionsToleratesDetectorFailure is the regression guard for
// issue #102: a failing optional detector must NOT disable telemetry. The
// distroless host.id failure surfaces as a plain error (not the
// ErrPartialResource sentinel), so tolerance cannot rely on errors.Is alone —
// resourceFromOptions must use the always-usable partial resource regardless.
func TestResourceFromOptionsToleratesDetectorFailure(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"plain host.id-style error", errors.New("host id not found in: /etc/machine-id or /var/lib/dbus/machine-id")},
		{"ErrPartialResource-wrapped", resource.ErrPartialResource},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := resourceFromOptions(context.Background(), discardLogger(),
				resource.WithDetectors(failingDetector{err: tc.err}),
				resource.WithAttributes(attribute.String("service.name", "engram")),
			)
			if res == nil {
				t.Fatal("resourceFromOptions returned nil; a failed detector must not nil out the resource")
			}
			got := map[string]string{}
			for _, kv := range res.Attributes() {
				got[string(kv.Key)] = kv.Value.String()
			}
			if got["service.name"] != "engram" {
				t.Errorf("service.name = %q, want engram (surviving attributes must be retained despite the failed detector)", got["service.name"])
			}
		})
	}
}

// TestResourceFromOptionsLogsPartialViaProvidedLogger guards finding engram-1id.1:
// the partial-resource warning must go through the caller-supplied bootstrap
// logger (which honours ENGRAM_LOG_LEVEL/FORMAT) rather than the global slog default
// (which, at telemetry.Setup time, is still the stock handler — slog.SetDefault
// runs later in serve.go).
func TestResourceFromOptionsLogsPartialViaProvidedLogger(t *testing.T) {
	var buf bytes.Buffer
	lg := slog.New(slog.NewJSONHandler(&buf, nil))

	resourceFromOptions(context.Background(), lg,
		resource.WithDetectors(failingDetector{err: errors.New("host id not found in: /etc/machine-id")}),
		resource.WithAttributes(attribute.String("service.name", "engram")),
	)

	if !strings.Contains(buf.String(), "partial telemetry resource") {
		t.Errorf("partial-resource warning was not routed to the provided logger; buffer = %q", buf.String())
	}
}
