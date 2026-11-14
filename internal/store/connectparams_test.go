// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestStartupConnectParams(t *testing.T) {
	p := StartupConnectParams()
	if p.Backoff.MaxDelay != 5*time.Second {
		t.Errorf("MaxDelay = %s, want 5s", p.Backoff.MaxDelay)
	}
	if p.Backoff.BaseDelay <= 0 || p.Backoff.Multiplier <= 0 || p.MinConnectTimeout <= 0 {
		t.Errorf("connect params %+v: BaseDelay, Multiplier and MinConnectTimeout must be non-zero", p)
	}
}

func TestWaitForReadyInterceptor(t *testing.T) {
	icpt := WaitForReadyInterceptor()
	for name, tc := range map[string]struct {
		ctx  context.Context
		want bool
	}{
		"marked":   {WithWaitForReady(context.Background()), true},
		"unmarked": {context.Background(), false},
	} {
		t.Run(name, func(t *testing.T) {
			var got bool
			err := icpt(tc.ctx, "/m", nil, nil, nil, func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, opts ...grpc.CallOption) error {
				for _, o := range opts {
					if _, ok := o.(grpc.FailFastCallOption); ok {
						got = true
					}
				}
				return nil
			})
			if err != nil || got != tc.want {
				t.Fatalf("WaitForReady option present = %v (err %v), want %v", got, err, tc.want)
			}
		})
	}
}
