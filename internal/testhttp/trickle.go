// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package testhttp

import (
	"bytes"
	"net/http"
	"time"
)

// TrickleHandler returns an http.Handler that writes status 200 and prelude
// immediately (flushed at once), then writes count chunks of chunkSize
// padding bytes, pausing pause between each chunk — simulating a slow
// provider tail after a complete, decodable response has already arrived.
// The caller supplies prelude as a whole, valid response value (e.g. a
// complete JSON document) so a decoder reading only the prelude succeeds
// immediately; the trailing padding exists solely so a caller's own
// time-bounded drain has something slow to abandon.
//
// The total trickle is BOUNDED BY CONSTRUCTION: count*pause is the absolute
// ceiling on how long this handler can hold a connection open, and it
// always finishes on its own. An open-ended trickle would turn a regression
// from a clean assertion failure into a go test timeout — a materially
// worse and much slower proof, so every caller must pass a genuinely
// bounded count and pause, never a value meant to run indefinitely.
//
// It also returns early once the request context is done, so a client that
// abandons the body (e.g. httpdrain.Drain closing it) releases this handler
// immediately rather than waiting out the full budget — httptest.Server's
// Close does not hang on it. Both exits terminate the handler; neither is
// relied on alone.
func TrickleHandler(prelude []byte, chunkSize, count int, pause time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(prelude)
		flusher, canFlush := w.(http.Flusher)
		if canFlush {
			flusher.Flush()
		}

		chunk := bytes.Repeat([]byte{'p'}, chunkSize)
		for range count {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(pause):
			}
			if _, err := w.Write(chunk); err != nil {
				return
			}
			if canFlush {
				flusher.Flush()
			}
		}
	})
}
