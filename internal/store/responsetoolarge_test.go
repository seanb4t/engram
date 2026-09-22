// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file pins classifyResponseTooLarge's boundary (D-02) and its chain
// position (D-01) with synthetic gRPC statuses. The statuses below mirror
// grpc-go v1.83.2's receive- and send-side ResourceExhausted message shapes
// as INPUTS to OUR classifier — this file tests our own matching logic,
// never grpc-go's behavior (rule m45p2b4bp7).
// TestStoreListOverflowIsResponseTooLarge (responsetoolarge_oversized_test.go)
// is what proves the pinned grpc-go version's real traffic still matches one
// of these shapes.

package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeInvoker returns a grpc.UnaryInvoker that ignores every argument and
// always returns want, for driving classifyResponseTooLarge directly
// without a real *grpc.ClientConn.
func fakeInvoker(want error) grpc.UnaryInvoker {
	return func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		return want
	}
}

// TestClassifyResponseTooLarge pins the D-02 boundary: exactly the four
// grpc-go receive shapes classify (with exact Observed/Limit parsing), and
// every other input — both send shapes, a server-sent status, an
// unprefixed receive-worded message, a non-ResourceExhausted code carrying
// the receive shape, an unrelated code, a plain error, and a nil result —
// comes back unchanged.
func TestClassifyResponseTooLarge(t *testing.T) {
	const method = "/qdrant.Points/Scroll"

	classifyCases := []struct {
		name         string
		in           error
		wantObserved int
		wantLimit    int
	}{
		{
			name:         "receive shape: max length allowed on current machine",
			in:           status.Newf(grpccodes.ResourceExhausted, "grpc: received message larger than max length allowed on current machine (%d vs. %d)", 4294967296, 2147483647).Err(),
			wantObserved: 4294967296,
			wantLimit:    2147483647,
		},
		{
			name:         "receive shape: larger than max (pair)",
			in:           status.Newf(grpccodes.ResourceExhausted, "grpc: received message larger than max (%d vs. %d)", 5242880, 4194304).Err(),
			wantObserved: 5242880,
			wantLimit:    4194304,
		},
		{
			name:         "receive shape: after decompression larger than max (pair)",
			in:           status.Newf(grpccodes.ResourceExhausted, "grpc: message after decompression larger than max (%d vs. %d)", 5242880, 4194304).Err(),
			wantObserved: 5242880,
			wantLimit:    4194304,
		},
		{
			name:         "receive shape: after decompression larger than max (single)",
			in:           status.Newf(grpccodes.ResourceExhausted, "grpc: received message after decompression larger than max %d", 4194304).Err(),
			wantObserved: 0,
			wantLimit:    4194304,
		},
	}
	for _, tc := range classifyCases {
		t.Run(tc.name, func(t *testing.T) {
			err := classifyResponseTooLarge(context.Background(), method, nil, nil, nil, fakeInvoker(tc.in))
			if !errors.Is(err, ErrResponseTooLarge) {
				t.Fatalf("errors.Is(err, ErrResponseTooLarge) = false; err = %v", err)
			}
			st, ok := status.FromError(err)
			if !ok || st.Code() != grpccodes.ResourceExhausted {
				t.Fatalf("status.FromError(err): ok=%v code=%v, want ok=true code=ResourceExhausted", ok, st.Code())
			}
			var rtle *ResponseTooLargeError
			if !errors.As(err, &rtle) {
				t.Fatalf("errors.As(err, &rtle) = false; err = %v", err)
			}
			if rtle.Observed != tc.wantObserved {
				t.Errorf("Observed = %d, want %d", rtle.Observed, tc.wantObserved)
			}
			if rtle.Limit != tc.wantLimit {
				t.Errorf("Limit = %d, want %d", rtle.Limit, tc.wantLimit)
			}
			if !strings.Contains(err.Error(), method) {
				t.Errorf("err.Error() = %q, want it to contain the method %q for server-side logs", err.Error(), method)
			}
		})
	}

	passthroughCases := []struct {
		name string
		in   error
	}{
		{
			name: "send shape: grpc-prefixed trying to send",
			in:   status.Newf(grpccodes.ResourceExhausted, "grpc: trying to send message larger than max (%d vs. %d)", 5242880, 4194304).Err(),
		},
		{
			name: "send shape: unprefixed trying to send",
			in:   status.Newf(grpccodes.ResourceExhausted, "trying to send message larger than max (%d vs. %d)", 5242880, 4194304).Err(),
		},
		{
			name: "server-sent too many requests",
			in:   status.New(grpccodes.ResourceExhausted, "Too many requests").Err(),
		},
		{
			name: "empty message",
			in:   status.New(grpccodes.ResourceExhausted, "").Err(),
		},
		{
			name: "receive wording without grpc prefix",
			in:   status.New(grpccodes.ResourceExhausted, "server received message larger than max (1 vs. 2)").Err(),
		},
		{
			name: "non-ResourceExhausted code carrying the receive shape",
			in:   status.Newf(grpccodes.Internal, "grpc: received message larger than max (%d vs. %d)", 1, 2).Err(),
		},
		{
			name: "unavailable, unrelated message",
			in:   status.New(grpccodes.Unavailable, "connection refused").Err(),
		},
		{
			name: "plain error, not a gRPC status",
			in:   errors.New("boom"),
		},
	}
	for _, tc := range passthroughCases {
		t.Run(tc.name, func(t *testing.T) {
			err := classifyResponseTooLarge(context.Background(), method, nil, nil, nil, fakeInvoker(tc.in))
			if !errors.Is(err, tc.in) {
				t.Fatalf("errors.Is(err, tc.in) = false — the pass-through error changed identity; got %v, want %v", err, tc.in)
			}
			if err.Error() != tc.in.Error() {
				t.Fatalf("err.Error() = %q, want unchanged %q", err.Error(), tc.in.Error())
			}
			if errors.Is(err, ErrResponseTooLarge) {
				t.Fatalf("errors.Is(err, ErrResponseTooLarge) = true, want false: this row must not be relabeled")
			}
		})
	}

	t.Run("nil invoker result", func(t *testing.T) {
		err := classifyResponseTooLarge(context.Background(), method, nil, nil, nil, fakeInvoker(nil))
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	})
}

// TestClassifyResponseTooLargeIsIdempotent proves re-classifying an
// already-classified *ResponseTooLargeError returns it unchanged — the
// composed Error() text does not carry the "grpc: " prefix isRecvLimitMessage
// requires, so a doubled installation can never double-wrap.
func TestClassifyResponseTooLargeIsIdempotent(t *testing.T) {
	const method = "/qdrant.Points/Scroll"
	already := newResponseTooLargeError(method, "grpc: received message after decompression larger than max 4194304")

	got := classifyResponseTooLarge(context.Background(), method, nil, nil, nil, fakeInvoker(already))

	if got.Error() != already.Error() {
		t.Fatalf("got.Error() = %q, want unchanged %q", got.Error(), already.Error())
	}
	var rtle *ResponseTooLargeError
	if !errors.As(got, &rtle) {
		t.Fatalf("errors.As(got, &rtle) = false; got = %v", got)
	}
	if rtle.Method != already.Method || rtle.Limit != already.Limit || rtle.Observed != already.Observed {
		t.Fatalf("rtle = %+v, want %+v", rtle, already)
	}
}

// TestClassifyResponseTooLargeIsConcurrencySafe proves the classifier holds
// no mutable state: 64 concurrent classifications, each with its own method,
// each return a *ResponseTooLargeError carrying their own method and no
// other goroutine's. Run under -race.
func TestClassifyResponseTooLargeIsConcurrencySafe(t *testing.T) {
	const n = 64
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			method := fmt.Sprintf("/m/%d", i)
			in := status.New(grpccodes.ResourceExhausted, "grpc: received message after decompression larger than max 4194304").Err()
			errs[i] = classifyResponseTooLarge(context.Background(), method, nil, nil, nil, fakeInvoker(in))
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		var rtle *ResponseTooLargeError
		if !errors.As(err, &rtle) {
			t.Fatalf("goroutine %d: errors.As(err, &rtle) = false; err = %v", i, err)
		}
		want := fmt.Sprintf("/m/%d", i)
		if rtle.Method != want {
			t.Fatalf("goroutine %d: Method = %q, want %q", i, rtle.Method, want)
		}
	}
}

// TestResponseTooLargeClassifierSitsInsideCallerChain proves the D-01 chain
// position against real Qdrant: a caller interceptor added with
// grpc.WithChainUnaryInterceptor (never grpc.WithUnaryInterceptor, which the
// five existing in-package interceptor helpers use and which is ALWAYS
// outermost) sits INSIDE the base classifier, exactly where the network
// would. A receive-shaped injected status for Scroll reaches the caller as
// store.ErrResponseTooLarge; a server-sent ResourceExhausted (Too many
// requests) reaches the caller unrelabeled, code and message intact.
func TestResponseTooLargeClassifierSitsInsideCallerChain(t *testing.T) {
	cases := []struct {
		name         string
		injectedMsg  string
		wantSentinel bool
	}{
		{
			name:         "receive single-number shape becomes the sentinel",
			injectedMsg:  "grpc: received message after decompression larger than max 4194304",
			wantSentinel: true,
		},
		{
			name:         "server-sent too many requests passes through unrelabeled",
			injectedMsg:  "Too many requests",
			wantSentinel: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// inject must pass every non-Scroll RPC through to the real
			// invoker so EnsureCollection and any Count call hit real
			// Qdrant — only Scroll gets the synthetic status.
			inject := func(
				ctx context.Context, method string, req, reply any,
				cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
			) error {
				if !strings.HasSuffix(method, "/Scroll") {
					return invoker(ctx, method, req, reply, cc, opts...)
				}
				return status.New(grpccodes.ResourceExhausted, tc.injectedMsg).Err()
			}
			c := dialTestClient(t, grpc.WithChainUnaryInterceptor(inject))
			name := testCollection("classifier_chain_test")
			st := newTestStore(t, c, name)
			ctx := context.Background()
			if err := st.EnsureCollection(ctx, 3); err != nil {
				t.Fatalf("EnsureCollection: %v", err)
			}
			t.Cleanup(func() {
				if err := c.DeleteCollection(ctx, name); err != nil {
					t.Errorf("DeleteCollection(%q): %v", name, err)
				}
			})

			_, _, _, err := st.List(ctx, "chain-test-scope", Authenticated("chain-test-owner"), ListOptions{Limit: 10})
			if err == nil {
				t.Fatalf("List: got nil error, want the injected status")
			}
			st2, ok := status.FromError(err)
			if !ok || st2.Code() != grpccodes.ResourceExhausted {
				t.Fatalf("status.FromError(err): ok=%v code=%v, want ok=true code=ResourceExhausted", ok, st2.Code())
			}
			gotSentinel := errors.Is(err, ErrResponseTooLarge)
			if gotSentinel != tc.wantSentinel {
				t.Fatalf("errors.Is(err, ErrResponseTooLarge) = %v, want %v; err = %v", gotSentinel, tc.wantSentinel, err)
			}
			if !tc.wantSentinel && !strings.Contains(err.Error(), tc.injectedMsg) {
				t.Errorf("err.Error() = %q, want it to contain the injected message %q", err.Error(), tc.injectedMsg)
			}
		})
	}
}
