// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v7"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"pgregory.net/rapid"

	"github.com/seanb4t/engram/internal/store/storetest"
)

// attemptLog captures the warn entries of failed connect-and-create attempts.
type attemptLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *attemptLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

// attempts returns the attempt numbers of the logged failures, in order.
func (l *attemptLog) attempts(t testing.TB) []int {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	var got []int
	for line := range strings.SplitSeq(strings.TrimSpace(l.buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec struct {
			Level   string `json:"level"`
			Msg     string `json:"msg"`
			Attempt int    `json:"attempt"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		if rec.Msg == "qdrant connect-and-create attempt failed" {
			if rec.Level != "WARN" {
				t.Errorf("attempt %d logged at %s, want WARN", rec.Attempt, rec.Level)
			}
			got = append(got, rec.Attempt)
		}
	}
	return got
}

func captureAttempts(t testing.TB) *attemptLog {
	l := &attemptLog{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(l, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return l
}

func zeroPolicy(timeout time.Duration) startupPolicy {
	return startupPolicy{timeout: timeout, newBackOff: func() backoff.BackOff { return &backoff.ZeroBackOff{} }}
}

// runRetry runs retryConnectAndCreate under a startup budget derived from sig.
// It also reports whether that budget was still live when the retry returned.
func runRetry(sig context.Context, policy startupPolicy, attempt func(context.Context) error) (parentLive bool, err error) {
	parent, cancel := context.WithTimeout(sig, policy.timeout)
	defer cancel()
	err = retryConnectAndCreate(sig, parent, policy, attempt)
	return parent.Err() == nil, err
}

func numbered(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i + 1
	}
	return out
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func isUnavailableStatus(err error) bool {
	st, ok := status.FromError(err)
	return ok && st.Code() == codes.Unavailable
}

func assertBudgetError(t *testing.T, err error) {
	t.Helper()
	if !isUnavailableStatus(err) {
		t.Fatalf("err = %v, want an Unavailable status", err)
	}
	for _, want := range []string{"connect-and-create", "ENGRAM_QDRANT_STARTUP_TIMEOUT"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

func TestServeProvisionRetry(t *testing.T) {
	unavailable := status.Error(codes.Unavailable, "connection refused")

	t.Run("unavailable then success logs each failure", func(t *testing.T) {
		log := captureAttempts(t)
		calls := 0
		_, err := runRetry(context.Background(), zeroPolicy(time.Minute), func(context.Context) error {
			calls++
			if calls <= 3 {
				return unavailable
			}
			return nil
		})
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if got := log.attempts(t); !equalInts(got, numbered(3)) {
			t.Errorf("logged attempts = %v, want [1 2 3]", got)
		}
	})

	t.Run("an attempt cut off by its own budget is retried", func(t *testing.T) {
		calls := 0
		_, err := runRetry(context.Background(), zeroPolicy(time.Minute), func(context.Context) error {
			calls++
			if calls == 1 {
				return status.Error(codes.DeadlineExceeded, "context deadline exceeded")
			}
			return nil
		})
		if err != nil || calls != 2 {
			t.Fatalf("err = %v after %d attempts, want nil after 2", err, calls)
		}
	})

	t.Run("an error that is not transient fails at once, unchanged", func(t *testing.T) {
		createErr := status.Error(codes.PermissionDenied, "forbidden")
		calls := 0
		_, err := runRetry(context.Background(), zeroPolicy(time.Minute), func(context.Context) error {
			calls++
			return createErr
		})
		if calls != 1 {
			t.Errorf("attempts = %d, want 1", calls)
		}
		if !errors.Is(err, createErr) || err.Error() != createErr.Error() {
			t.Errorf("err = %v, want the create error unchanged", err)
		}
	})

	t.Run("a blocked attempt at the end of the budget is a spent budget", func(t *testing.T) {
		_, err := runRetry(context.Background(), zeroPolicy(50*time.Millisecond), func(ctx context.Context) error {
			<-ctx.Done()
			return status.Error(codes.DeadlineExceeded, "context deadline exceeded")
		})
		assertBudgetError(t, err)
	})

	t.Run("backoff stopping on its elapsed-time limit is a spent budget", func(t *testing.T) {
		policy := startupPolicy{timeout: 100 * time.Millisecond, newBackOff: func() backoff.BackOff {
			return backoff.NewConstantBackOff(60 * time.Millisecond)
		}}
		parentLive, err := runRetry(context.Background(), policy, func(context.Context) error { return unavailable })
		if !parentLive {
			t.Fatalf("parent ended before Retry returned: the elapsed-time branch was not reached")
		}
		assertBudgetError(t, err)
	})

	t.Run("a signal during an attempt stops the retry", func(t *testing.T) {
		sig, cancel := context.WithCancel(context.Background())
		defer cancel()
		calls := 0
		_, err := runRetry(sig, zeroPolicy(time.Minute), func(ctx context.Context) error {
			calls++
			if calls == 2 {
				cancel()
				<-ctx.Done()
				return status.Error(codes.Canceled, "context canceled")
			}
			return unavailable
		})
		if calls != 2 {
			t.Errorf("attempts = %d, want 2", calls)
		}
		if !errors.Is(err, context.Canceled) || isUnavailableStatus(err) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	})

	t.Run("a signal during the backoff wait stops the retry", func(t *testing.T) {
		sig, cancel := context.WithCancel(context.Background())
		defer cancel()
		policy := startupPolicy{timeout: time.Minute, newBackOff: func() backoff.BackOff {
			return cancelingBackOff{cancel: cancel, wait: 10 * time.Second}
		}}
		calls := 0
		_, err := runRetry(sig, policy, func(context.Context) error {
			calls++
			return unavailable
		})
		if calls != 1 {
			t.Errorf("attempts = %d, want 1", calls)
		}
		if !errors.Is(err, context.Canceled) || isUnavailableStatus(err) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	})

	t.Run("Qdrant up on the last attempt the budget allows", func(t *testing.T) {
		policy := startupPolicy{timeout: time.Minute, newBackOff: func() backoff.BackOff {
			return &backoff.ZeroBackOff{}
		}}
		calls := 0
		_, err := runRetry(context.Background(), policy, func(context.Context) error {
			calls++
			if calls < 5 {
				return unavailable
			}
			return nil
		})
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	})
}

// cancelingBackOff cancels the signal context as backoff asks for the next
// wait, so the cancellation lands during that wait.
type cancelingBackOff struct {
	cancel context.CancelFunc
	wait   time.Duration
}

func (b cancelingBackOff) NextBackOff() time.Duration {
	b.cancel()
	return b.wait
}

func (cancelingBackOff) Reset() {}

// TestServeSchemaNotCutByStartupTimeout drives ensureStoreFromConfig: schema
// convergence runs under the signal context and its own budget, never the
// startup budget.
func TestServeSchemaNotCutByStartupTimeout(t *testing.T) {
	collection := provisionCollection(t, "provision_schema_not_startup")
	t.Setenv("ENGRAM_QDRANT_ADDR", storetest.Addr())
	t.Setenv("ENGRAM_QDRANT_COLLECTION", collection)
	t.Setenv("ENGRAM_EMBED_DIM", "3")
	t.Setenv("ENGRAM_QDRANT_SCHEMA_TIMEOUT", "10s")
	cfg, err := loadAndValidate()
	if err != nil {
		t.Fatalf("loadAndValidate: %v", err)
	}
	policy := servePolicy(100 * time.Millisecond)
	policy.extraDial = []grpc.DialOption{grpc.WithChainUnaryInterceptor(stallFirst(createFieldIndex, 300*time.Millisecond))}
	if _, err := ensureStoreFromConfig(context.Background(), cfg, policy); err != nil {
		t.Fatalf("ensureStoreFromConfig: %v", err)
	}
}

func TestStoreFromEnvSingleAttempt(t *testing.T) {
	log := captureAttempts(t)
	t.Setenv("ENGRAM_QDRANT_ADDR", "127.0.0.1:1")
	t.Setenv("ENGRAM_EMBED_DIM", "3")
	if _, err := StoreFromEnv(); err == nil {
		t.Fatal("StoreFromEnv against a closed port = nil, want an error")
	}
	if got := log.attempts(t); len(got) != 0 {
		t.Errorf("retry attempts logged = %v, want none: CLI verbs make one attempt", got)
	}
}

func TestPropertyServeBackOffBound(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		b := newServeBackOff()
		b.Reset()
		for i := range rapid.IntRange(1, 64).Draw(t, "draws") {
			if d := b.NextBackOff(); d > serveRetryMaxInterval*3/2 {
				t.Fatalf("draw %d = %s, want at most 7.5s", i, d)
			}
		}
	})
}

func TestPropertyServeProvisionRetryLog(t *testing.T) {
	unavailable := status.Error(codes.Unavailable, "connection refused")
	rapid.Check(t, func(rt *rapid.T) {
		log := captureAttempts(t)
		n := rapid.IntRange(0, 6).Draw(rt, "transient")
		outcome := rapid.SampledFrom([]string{"success", "permanent", "exhausted", "signal"}).Draw(rt, "outcome")
		createErr := status.Error(codes.PermissionDenied, "forbidden")
		sig, cancel := context.WithCancel(context.Background())
		defer cancel()
		calls := 0
		var wantFailed int
		attempt := func(ctx context.Context) error {
			calls++
			if calls <= n {
				return unavailable
			}
			switch outcome {
			case "success":
				return nil
			case "permanent":
				return createErr
			case "signal":
				cancel()
				return status.Error(codes.Canceled, "context canceled")
			default:
				<-ctx.Done()
				return status.Error(codes.DeadlineExceeded, "context deadline exceeded")
			}
		}
		timeout := time.Minute
		if outcome == "exhausted" {
			timeout = 20 * time.Millisecond
		}
		_, err := runRetry(sig, zeroPolicy(timeout), attempt)
		switch outcome {
		case "success":
			wantFailed = n
			if err != nil {
				rt.Fatalf("success: err = %v", err)
			}
		case "permanent":
			wantFailed = n + 1
			if !errors.Is(err, createErr) {
				rt.Fatalf("permanent: err = %v, want the create error", err)
			}
		case "signal":
			wantFailed = n + 1
			if !errors.Is(err, context.Canceled) {
				rt.Fatalf("signal: err = %v, want context.Canceled", err)
			}
		default:
			wantFailed = calls
			if !isUnavailableStatus(err) || !strings.Contains(err.Error(), "ENGRAM_QDRANT_STARTUP_TIMEOUT") {
				rt.Fatalf("exhausted: err = %v, want an Unavailable status naming the timeout", err)
			}
		}
		if got := log.attempts(t); !equalInts(got, numbered(wantFailed)) {
			rt.Fatalf("logged attempts = %v, want 1..%d", got, wantFailed)
		}
	})
}
