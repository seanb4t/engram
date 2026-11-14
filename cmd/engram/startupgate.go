// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

// startupRetryAfter is the Retry-After value, in seconds, of the startup
// gate's 503.
const startupRetryAfter = "5"

// startupGate answers every request until its target is set, then hands each
// request to the target. The primary listener's gate also serves the
// liveness and readiness routes. The target pointer is the only record of
// "provisioned": it is set once and never cleared.
type startupGate struct {
	target atomic.Pointer[http.Handler]
	// health is the primary listener's readiness source; nil on the
	// MCP-only listener, which serves no health route.
	health *healthLoop
}

func (g *startupGate) setTarget(h http.Handler) { g.target.Store(&h) }

func (g *startupGate) provisioned() bool { return g.target.Load() != nil }

func (g *startupGate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if g.health != nil && r.Method == http.MethodGet {
		switch r.URL.Path {
		case "/livez":
			w.WriteHeader(http.StatusOK)
			return
		case "/readyz":
			if g.provisioned() && g.health.ok.Load() {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
	}
	h := g.target.Load()
	if h == nil {
		w.Header().Set("Retry-After", startupRetryAfter)
		http.Error(w, "engram is starting: provisioning is not complete", http.StatusServiceUnavailable)
		return
	}
	(*h).ServeHTTP(w, r)
}

// healthLoop holds the result of the last Qdrant health check. It sends one
// ping when it starts and one per tick, each under its own timeout, so a
// readiness request never calls Qdrant.
type healthLoop struct {
	ping    func(context.Context) error
	timeout time.Duration
	ok      atomic.Bool
	// checked runs after each ping's result is stored; tests use it to
	// step the loop.
	checked func()
}

func (l *healthLoop) run(ctx context.Context, ticks <-chan time.Time) {
	first, last := true, false
	check := func() {
		pctx, cancel := context.WithTimeout(ctx, l.timeout)
		err := l.ping(pctx)
		cancel()
		switch {
		case err != nil && (first || last):
			slog.Warn("qdrant health check failed; not ready", "err", err)
		case err == nil && !first && !last:
			slog.Info("qdrant health check recovered; ready")
		}
		l.ok.Store(err == nil)
		first, last = false, err == nil
		if l.checked != nil {
			l.checked()
		}
	}
	check()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			check()
		}
	}
}

// markProvisioned sets the target of every gate, and only then calls
// startHealth, so a primary /readyz of 200 implies every listener serves.
func markProvisioned(gates []*startupGate, targets []http.Handler, startHealth func()) {
	for i, g := range gates {
		g.setTarget(targets[i])
	}
	startHealth()
}
