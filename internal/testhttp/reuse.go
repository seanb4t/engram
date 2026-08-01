// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package testhttp provides connection-reuse test instrumentation shared by
// internal/embed and internal/summarize. It is a normal (non-_test.go) file
// in an internal package rather than a _test.go helper because Go cannot
// share a _test.go across package boundaries, and both provider clients'
// test packages need the same tracker.
//
// It imports no test framework and exposes only counters and accessors, so
// nothing test-only is pulled into a production import graph even though the
// package is importable from non-test code.
package testhttp

import (
	"context"
	"net/http/httptrace"
	"sync"
)

// ReuseTracker counts how many outbound HTTP requests, on contexts it
// wrapped via Context, reused a pooled connection versus opened a fresh one.
//
// It only observes requests whose context was produced by (*ReuseTracker).
// Context. Both internal/embed and internal/summarize build their requests
// with http.NewRequestWithContext, so wrapping the ctx passed into
// Embed/EmbedQuery/Summarize is what makes the tracker see anything —
// attaching it anywhere else (e.g. only on the *http.Client) silently
// observes nothing, and a reuse assertion built that way passes vacuously
// regardless of whether the code under test actually drains its responses.
type ReuseTracker struct {
	mu     sync.Mutex
	fresh  int
	reused int
}

// Context wraps parent with an httptrace.ClientTrace whose GotConn hook
// increments the tracker's fresh or reused count depending on
// httptrace.GotConnInfo.Reused.
func (r *ReuseTracker) Context(parent context.Context) context.Context {
	trace := &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			r.mu.Lock()
			defer r.mu.Unlock()
			if info.Reused {
				r.reused++
			} else {
				r.fresh++
			}
		},
	}
	return httptrace.WithClientTrace(parent, trace)
}

// Reused returns the number of observed requests that reused a pooled
// connection.
func (r *ReuseTracker) Reused() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reused
}

// Total returns the total number of observed requests (fresh + reused).
func (r *ReuseTracker) Total() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fresh + r.reused
}
