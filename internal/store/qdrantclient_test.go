// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"sync/atomic"
	"testing"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

// TestNewQdrantClientAppliesBaseAndCallerOptions is REQ-test-client-parity's
// direct proof that a test client applies the same dial option as the
// production client (the otelgrpc stats handler) while still honoring caller
// options. It asserts through the observable span and interceptor rather
// than by inspecting grpc internals — it asserts our constructor's
// composition, never grpc-go's or Qdrant's own behavior (rule m45p2b4bp7).
//
// Does not opt into t.Parallel: the span recorder withSpanRecorder installs
// is process-wide (see instrument_test.go).
func TestNewQdrantClientAppliesBaseAndCallerOptions(t *testing.T) {
	sr := withSpanRecorder(t)
	before := len(sr.Ended())

	var calls atomic.Int64
	counting := func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		if len(method) >= len("/HealthCheck") && method[len(method)-len("/HealthCheck"):] == "/HealthCheck" {
			calls.Add(1)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}

	c := dialTestClient(t, grpc.WithUnaryInterceptor(counting))
	if _, err := c.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}

	if calls.Load() < 1 {
		t.Fatalf("counting interceptor never fired for HealthCheck — caller dial option did not take effect")
	}

	var healthSpan trace.SpanKind = -1
	for _, sp := range sr.Ended()[before:] {
		if len(sp.Name()) >= len("/HealthCheck") && sp.Name()[len(sp.Name())-len("/HealthCheck"):] == "/HealthCheck" {
			healthSpan = sp.SpanKind()
			break
		}
	}
	if healthSpan != trace.SpanKindClient {
		t.Fatalf("no client-kind HealthCheck span recorded — production base dial option (otelgrpc) did not take effect on this test client")
	}
}
