// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package jev

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/seanb4t/engram/internal/decide"
)

// oneNoulRequest is a minimal valid Request with one noul question named
// "q", matching fixtureNoulOK's answer key.
func oneNoulRequest() decide.Request {
	return decide.Request{
		State:     decide.State{"k": "v"},
		Questions: map[string]decide.Question{"q": decide.Noul("is it true?", "yes", "no")},
	}
}

// noRetryDelay makes a Client retry immediately, never sleeping the
// production jitter (per this plan's Executor notes).
func noRetryDelay(c *Client) { c.retryDelay = func() time.Duration { return 0 } }

// TestJevNoRetryOption proves D-09's client half: a Client built with
// WithNoRetry() makes exactly ONE HTTP attempt on a retryable failure and
// returns the classified error, even though the same handler would have
// succeeded on a second attempt; a default Client (no WithNoRetry) against
// the identical handler still retries once, per D-11.
func TestJevNoRetryOption(t *testing.T) {
	t.Run("503_once", func(t *testing.T) {
		var n int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if atomic.AddInt32(&n, 1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(fixtureOpenRouter503))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fixtureNoulOK))
		}))
		defer srv.Close()

		c := New(srv.URL, "k", "m", WithNoRetry())
		noRetryDelay(c)
		_, err := c.Decide(context.Background(), oneNoulRequest())
		if !errors.Is(err, decide.ErrDecisionUnavailable) {
			t.Errorf("err = %v, want ErrDecisionUnavailable", err)
		}
		if got := atomic.LoadInt32(&n); got != 1 {
			t.Errorf("requests = %d, want 1", got)
		}
	})

	t.Run("429_once", func(t *testing.T) {
		var n int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if atomic.AddInt32(&n, 1) == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(fixtureOpenRouter429))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fixtureNoulOK))
		}))
		defer srv.Close()

		c := New(srv.URL, "k", "m", WithNoRetry())
		noRetryDelay(c)
		_, err := c.Decide(context.Background(), oneNoulRequest())
		if !errors.Is(err, decide.ErrDecisionRateLimited) {
			t.Errorf("err = %v, want ErrDecisionRateLimited", err)
		}
		if got := atomic.LoadInt32(&n); got != 1 {
			t.Errorf("requests = %d, want 1", got)
		}
	})

	t.Run("dropped_connection_once", func(t *testing.T) {
		var n int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if atomic.AddInt32(&n, 1) == 1 {
				hj, ok := w.(http.Hijacker)
				if !ok {
					t.Fatal("ResponseWriter does not support Hijacker")
				}
				conn, _, herr := hj.Hijack()
				if herr != nil {
					t.Fatalf("Hijack: %v", herr)
				}
				_ = conn.Close() // abrupt close, no response written
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fixtureNoulOK))
		}))
		defer srv.Close()

		c := New(srv.URL, "k", "m", WithNoRetry())
		noRetryDelay(c)
		_, err := c.Decide(context.Background(), oneNoulRequest())
		if !errors.Is(err, decide.ErrDecisionUnavailable) {
			t.Errorf("err = %v, want ErrDecisionUnavailable", err)
		}
		if got := atomic.LoadInt32(&n); got != 1 {
			t.Errorf("requests = %d, want 1", got)
		}
	})

	t.Run("default_client_still_retries", func(t *testing.T) {
		var n int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if atomic.AddInt32(&n, 1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(fixtureOpenRouter503))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fixtureNoulOK))
		}))
		defer srv.Close()

		c := New(srv.URL, "k", "m")
		noRetryDelay(c)
		_, err := c.Decide(context.Background(), oneNoulRequest())
		if err != nil {
			t.Fatalf("Decide: %v", err)
		}
		if got := atomic.LoadInt32(&n); got != 2 {
			t.Errorf("requests = %d, want 2", got)
		}
	})
}

// TestJevRetryAndBounds proves D-11 (exactly one jittered retry, only on
// 429/5xx or a non-timeout transport failure, inside the one timeout
// budget) and DEC-04's success/error body bounds, by request count.
func TestJevRetryAndBounds(t *testing.T) {
	t.Run("503_then_200_retries_once", func(t *testing.T) {
		var n int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if atomic.AddInt32(&n, 1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(fixtureOpenRouter503))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fixtureNoulOK))
		}))
		defer srv.Close()

		c := New(srv.URL, "k", "m")
		noRetryDelay(c)
		_, err := c.Decide(context.Background(), oneNoulRequest())
		if err != nil {
			t.Fatalf("Decide: %v", err)
		}
		if got := atomic.LoadInt32(&n); got != 2 {
			t.Errorf("requests = %d, want 2", got)
		}
	})

	t.Run("503_then_503_gives_unavailable", func(t *testing.T) {
		var n int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&n, 1)
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(fixtureOpenRouter503))
		}))
		defer srv.Close()

		c := New(srv.URL, "k", "m")
		noRetryDelay(c)
		_, err := c.Decide(context.Background(), oneNoulRequest())
		if !errors.Is(err, decide.ErrDecisionUnavailable) {
			t.Errorf("err = %v, want ErrDecisionUnavailable", err)
		}
		if got := atomic.LoadInt32(&n); got != 2 {
			t.Errorf("requests = %d, want 2", got)
		}
	})

	t.Run("429_then_200_retries_once", func(t *testing.T) {
		var n int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if atomic.AddInt32(&n, 1) == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(fixtureOpenRouter429))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fixtureNoulOK))
		}))
		defer srv.Close()

		c := New(srv.URL, "k", "m")
		noRetryDelay(c)
		_, err := c.Decide(context.Background(), oneNoulRequest())
		if err != nil {
			t.Fatalf("Decide: %v", err)
		}
		if got := atomic.LoadInt32(&n); got != 2 {
			t.Errorf("requests = %d, want 2", got)
		}
	})

	t.Run("429_then_429_gives_rate_limited", func(t *testing.T) {
		var n int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&n, 1)
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(fixtureOpenRouter429))
		}))
		defer srv.Close()

		c := New(srv.URL, "k", "m")
		noRetryDelay(c)
		_, err := c.Decide(context.Background(), oneNoulRequest())
		if !errors.Is(err, decide.ErrDecisionRateLimited) {
			t.Errorf("err = %v, want ErrDecisionRateLimited", err)
		}
		if got := atomic.LoadInt32(&n); got != 2 {
			t.Errorf("requests = %d, want 2", got)
		}
	})

	t.Run("400_gives_bad_request_no_retry", func(t *testing.T) {
		var n int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&n, 1)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(fixtureOpenRouter400Choices))
		}))
		defer srv.Close()

		c := New(srv.URL, "k", "m")
		noRetryDelay(c)
		_, err := c.Decide(context.Background(), oneNoulRequest())
		if !errors.Is(err, decide.ErrDecisionBadRequest) {
			t.Errorf("err = %v, want ErrDecisionBadRequest", err)
		}
		if got := atomic.LoadInt32(&n); got != 1 {
			t.Errorf("requests = %d, want 1", got)
		}
	})

	t.Run("401_gives_auth_no_retry", func(t *testing.T) {
		var n int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&n, 1)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(fixtureOpenRouter401))
		}))
		defer srv.Close()

		c := New(srv.URL, "k", "m")
		noRetryDelay(c)
		_, err := c.Decide(context.Background(), oneNoulRequest())
		if !errors.Is(err, decide.ErrDecisionAuth) {
			t.Errorf("err = %v, want ErrDecisionAuth", err)
		}
		if got := atomic.LoadInt32(&n); got != 1 {
			t.Errorf("requests = %d, want 1", got)
		}
	})

	t.Run("hijacked_connection_then_200_retries_once", func(t *testing.T) {
		var n int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if atomic.AddInt32(&n, 1) == 1 {
				hj, ok := w.(http.Hijacker)
				if !ok {
					t.Fatal("ResponseWriter does not support Hijacker")
				}
				conn, _, herr := hj.Hijack()
				if herr != nil {
					t.Fatalf("Hijack: %v", herr)
				}
				_ = conn.Close() // abrupt close, no response written
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fixtureNoulOK))
		}))
		defer srv.Close()

		c := New(srv.URL, "k", "m")
		noRetryDelay(c)
		_, err := c.Decide(context.Background(), oneNoulRequest())
		if err != nil {
			t.Fatalf("Decide: %v", err)
		}
		if got := atomic.LoadInt32(&n); got != 2 {
			t.Errorf("requests = %d, want 2", got)
		}
	})

	t.Run("short_budget_times_out_no_retry", func(t *testing.T) {
		var n int32
		release := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&n, 1)
			select {
			case <-release:
			case <-time.After(time.Second):
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer func() {
			close(release)
			srv.Close()
		}()

		c := New(srv.URL, "k", "m", WithTimeout(100*time.Millisecond))
		noRetryDelay(c)
		start := time.Now()
		_, err := c.Decide(context.Background(), oneNoulRequest())
		elapsed := time.Since(start)
		if !errors.Is(err, decide.ErrDecisionTimeout) {
			t.Errorf("err = %v, want ErrDecisionTimeout", err)
		}
		if elapsed >= 800*time.Millisecond {
			t.Errorf("elapsed = %v, want < 800ms", elapsed)
		}
		if got := atomic.LoadInt32(&n); got != 1 {
			t.Errorf("requests = %d, want 1", got)
		}
	})

	t.Run("response_too_large_no_retry", func(t *testing.T) {
		var n int32
		big := strings.Repeat("x", 4096)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&n, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(big))
		}))
		defer srv.Close()

		c := New(srv.URL, "k", "m", WithMaxResponseBytes(1024))
		noRetryDelay(c)
		_, err := c.Decide(context.Background(), oneNoulRequest())
		if !errors.Is(err, decide.ErrDecisionResponseTooLarge) {
			t.Errorf("err = %v, want ErrDecisionResponseTooLarge", err)
		}
		if got := atomic.LoadInt32(&n); got != 1 {
			t.Errorf("requests = %d, want 1", got)
		}
	})

	t.Run("budget_exhausted_skips_retry", func(t *testing.T) {
		var n int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&n, 1)
			time.Sleep(120 * time.Millisecond)
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(fixtureOpenRouter503))
		}))
		defer srv.Close()

		c := New(srv.URL, "k", "m", WithTimeout(150*time.Millisecond))
		c.retryDelay = func() time.Duration { return 100 * time.Millisecond }
		_, err := c.Decide(context.Background(), oneNoulRequest())
		if !errors.Is(err, decide.ErrDecisionUnavailable) {
			t.Errorf("err = %v, want ErrDecisionUnavailable", err)
		}
		if got := atomic.LoadInt32(&n); got != 1 {
			t.Errorf("requests = %d, want 1 (no retry once the budget can't cover the delay)", got)
		}
	})

	t.Run("large_error_body_bounded_and_drained", func(t *testing.T) {
		// A 20KiB error body on every attempt (500 is retryable) proves
		// classifyStatus's Detail stays bounded even when the caller never
		// limited its own read: attempt reads only maxErrorBodyBytes before
		// handing the rest to httpdrain.Drain, and every one of the
		// handler's writes completes normally rather than blocking on a
		// stalled reader.
		var wrote int32
		big := strings.Repeat("e", 20*1024)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			if _, werr := w.Write([]byte(big)); werr == nil {
				atomic.AddInt32(&wrote, 1)
			}
		}))
		defer srv.Close()

		c := New(srv.URL, "k", "m")
		noRetryDelay(c)
		_, err := c.Decide(context.Background(), oneNoulRequest())
		if !errors.Is(err, decide.ErrDecisionUnavailable) {
			t.Errorf("err = %v, want ErrDecisionUnavailable", err)
		}
		var de *decide.Error
		if errors.As(err, &de) && len(de.Detail) > maxErrorBodyBytes {
			t.Errorf("len(Detail) = %d, want <= %d", len(de.Detail), maxErrorBodyBytes)
		}
		if got := atomic.LoadInt32(&wrote); got != 2 {
			t.Errorf("handler writes completed = %d, want 2 (one retry)", got)
		}
	})

	t.Run("empty_api_key_omits_header_gives_auth", func(t *testing.T) {
		var sawAuth bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sawAuth = r.Header.Get("Authorization") != ""
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(fixtureOpenRouter401))
		}))
		defer srv.Close()

		c := New(srv.URL, "", "m")
		noRetryDelay(c)
		_, err := c.Decide(context.Background(), oneNoulRequest())
		if !errors.Is(err, decide.ErrDecisionAuth) {
			t.Errorf("err = %v, want ErrDecisionAuth", err)
		}
		if sawAuth {
			t.Error("server saw an Authorization header for an empty API key")
		}
	})

	t.Run("cost_decodes_within_tolerance", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fixtureNoulOK))
		}))
		defer srv.Close()

		c := New(srv.URL, "k", "m")
		noRetryDelay(c)
		resp, err := c.Decide(context.Background(), oneNoulRequest())
		if err != nil {
			t.Fatalf("Decide: %v", err)
		}
		if resp.Usage == nil || resp.Usage.CostUSD == nil {
			t.Fatal("Usage.CostUSD is nil")
		}
		const want = 0.000020874
		if got := *resp.Usage.CostUSD; got < want-1e-12 || got > want+1e-12 {
			t.Errorf("CostUSD = %v, want within 1e-12 of %v", got, want)
		}
	})
}
