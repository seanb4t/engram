// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"testing"
)

func TestBuildResourceHasStandardAttributes(t *testing.T) {
	res, err := buildResource(context.Background(), Config{ServiceName: "engram", ServiceVersion: "1.2.3"})
	if err != nil {
		t.Fatalf("buildResource: %v", err)
	}
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
