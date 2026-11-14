// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/cenkalti/backoff/v7"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/seanb4t/engram/internal/store"
)

// The serve startup retry's backoff intervals. With backoff's default
// randomization factor of 0.5, the longest wait is 1.5 × serveRetryMaxInterval.
// serveRetryMaxInterval is also the serve client's gRPC reconnect cap.
const (
	serveRetryInitialInterval = 500 * time.Millisecond
	serveRetryMaxInterval     = 5 * time.Second
)

// startupPolicy configures the connect-and-create step. The zero value makes
// one attempt; a positive timeout makes `engram serve`'s retry.
type startupPolicy struct {
	timeout time.Duration
	// newBackOff builds the retry's BackOff; tests inject their own.
	newBackOff func() backoff.BackOff
	// extraDial is appended to the client's dial options; tests use it to
	// install interceptors.
	extraDial []grpc.DialOption
	// attempt bounds one connect-and-create attempt; zero means
	// collectionCreateBudget.
	attempt time.Duration
}

func servePolicy(timeout time.Duration) startupPolicy {
	return startupPolicy{timeout: timeout, newBackOff: newServeBackOff}
}

// newServeBackOff is the serve startup retry's production BackOff.
func newServeBackOff() backoff.BackOff {
	return newRetryBackOff(serveRetryInitialInterval, serveRetryMaxInterval)
}

func (p startupPolicy) retry() bool { return p.timeout > 0 }

func (p startupPolicy) attemptBudget() time.Duration {
	if p.attempt > 0 {
		return p.attempt
	}
	return collectionCreateBudget
}

// versionCheckTimeout bounds the client's construction-time version check by
// the smaller of the startup timeout and one attempt's budget.
func (p startupPolicy) versionCheckTimeout() time.Duration {
	return min(p.timeout, collectionCreateBudget)
}

func (p startupPolicy) dialOptions() []grpc.DialOption {
	return append([]grpc.DialOption{
		grpc.WithConnectParams(store.StartupConnectParams(serveRetryMaxInterval)),
		grpc.WithChainUnaryInterceptor(store.WaitForReadyInterceptor()),
	}, p.extraDial...)
}

// isTransientStartupErr reports whether a failed attempt is worth another:
// Qdrant is unavailable, or the attempt ran out of its own budget.
func isTransientStartupErr(err error) bool {
	if store.IsUnavailable(err) {
		return true
	}
	st, ok := status.FromError(err)
	return ok && st.Code() == codes.DeadlineExceeded
}

// retryConnectAndCreate runs attempt with backoff while Qdrant is unavailable.
// sigCtx is the signal context; parent is the startup budget derived from it.
// Each attempt runs under its own policy.attemptBudget(), capped by parent.
//
// The result is decided from the contexts, not from the error's cause: a
// done sigCtx returns sigCtx's error; a spent budget returns an Unavailable
// status naming the step and ENGRAM_QDRANT_STARTUP_TIMEOUT; any other stop
// returns the attempt's own error unchanged.
func retryConnectAndCreate(sigCtx, parent context.Context, policy startupPolicy, attempt func(context.Context) error) error {
	start := time.Now()
	n := 0
	_, err := backoff.Retry(parent, func() (struct{}, error) {
		n++
		actx, cancel := context.WithTimeout(parent, policy.attemptBudget())
		defer cancel()
		err := attempt(actx)
		if err == nil {
			return struct{}{}, nil
		}
		slog.Warn("qdrant connect-and-create attempt failed",
			"attempt", n, "elapsed", time.Since(start).Round(time.Millisecond), "err", err)
		if parent.Err() != nil || !isTransientStartupErr(err) {
			return struct{}{}, backoff.Permanent(err)
		}
		return struct{}{}, err
	},
		backoff.WithBackOff(policy.newBackOff()),
		backoff.WithMaxElapsedTime(policy.timeout),
	)
	if err == nil {
		return nil
	}
	if sigCtx.Err() != nil {
		return sigCtx.Err()
	}
	re := backoff.AsRetryError(err)
	lastErr := err
	if re != nil {
		lastErr = re.LastErr
	}
	if parent.Err() != nil || (re != nil && errors.Is(re.Cause, backoff.ErrMaxElapsedTime)) {
		return status.Error(codes.Unavailable, fmt.Sprintf(
			"connect-and-create: Qdrant still unavailable after ENGRAM_QDRANT_STARTUP_TIMEOUT=%s (%d attempts): %v",
			policy.timeout, n, lastErr))
	}
	return lastErr
}
