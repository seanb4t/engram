// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"testing"

	"google.golang.org/grpc"
)

// This file deliberately tests ONLY two things about the production 64 MiB
// receive-limit backstop: that the configured value reaches the dial
// options (pass-through), and that the backstop is appended before any
// caller-supplied dial option (ordering). It deliberately does NOT dial
// Qdrant, send an RPC, or assert anything about how gRPC itself enforces a
// receive ceiling — that is third-party buffer handling this project does
// not own (rule m45p2b4bp7). A future reader tempted to add a test
// asserting that a response of some size fails, or that gRPC rejects an
// oversized message, should stop here: that assertion belongs to grpc-go's
// own test suite, not this one.
//
// This test is legitimately package store (a whitebox exception): it
// inspects qdrantDialOptions and productionCallOptions directly and never
// dials a real Qdrant, so it imports no test harness.

// TestQdrantRecvLimitBackstopPassThrough proves the configured receive
// limit reaches the production dial options. It compares against the
// literal 64 MiB value D-05 chose, not against the backstop's own constant
// — comparing productionRecvLimit to itself would be tautological and
// would not actually prove the value was configured correctly.
func TestQdrantRecvLimitBackstopPassThrough(t *testing.T) {
	opts := productionCallOptions()
	if len(opts) != 1 {
		t.Fatalf("productionCallOptions() has %d entries, want exactly 1", len(opts))
	}
	got, ok := opts[0].(grpc.MaxRecvMsgSizeCallOption)
	if !ok {
		t.Fatalf("productionCallOptions()[0] = %T, want grpc.MaxRecvMsgSizeCallOption", opts[0])
	}
	const wantBytes = 64 << 20 // 64 MiB, D-05's chosen value
	if got.MaxRecvMsgSize != wantBytes {
		t.Errorf("MaxRecvMsgSize = %d, want %d (64 MiB)", got.MaxRecvMsgSize, wantBytes)
	}
}

// TestQdrantRecvLimitBackstopPrecedesCallerOptions proves the backstop is
// appended BEFORE any caller-supplied dial option, so a caller that
// appends its own receive limit afterwards still wins. Getting this
// ordering backward would silently raise every oversized regression test's
// effective ceiling without any of them turning red — this is the one test
// in the repository that would notice.
func TestQdrantRecvLimitBackstopPrecedesCallerOptions(t *testing.T) {
	base := qdrantDialOptions(nil)

	probe := grpc.WithUserAgent("qdrantbackstop-test-probe-h4x8k2")
	withProbe := qdrantDialOptions([]grpc.DialOption{probe})

	if len(withProbe) != len(base)+1 {
		t.Fatalf("len(qdrantDialOptions([probe])) = %d, want %d (len(base)+1)", len(withProbe), len(base)+1)
	}

	last := withProbe[len(withProbe)-1]
	if last != probe {
		t.Error("last dial option is not the exact probe value (identity) — the backstop is not appended before caller options")
	}
}
