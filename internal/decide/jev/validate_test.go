// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package jev

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/seanb4t/engram/internal/decide"
)

func tooManyChoicesRequest() decide.Request {
	options := make(map[string]string, 256)
	for i := range 256 {
		options[fmt.Sprintf("opt%d", i)] = "a description"
	}
	return decide.Request{Questions: map[string]decide.Question{
		"q": decide.Choice("pick one", options),
	}}
}

// TestJevValidatesBeforeNetwork proves D-09: an invalid request never
// reaches the network, for both Decide and DecideMany.
func TestJevValidatesBeforeNetwork(t *testing.T) {
	var requests int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt64(&requests, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL+"/api", "k", "")

	_, err := c.Decide(context.Background(), tooManyChoicesRequest())
	if !errors.Is(err, decide.ErrTooManyChoices) {
		t.Fatalf("Decide err = %v, want decide.ErrTooManyChoices", err)
	}
	if got := atomic.LoadInt64(&requests); got != 0 {
		t.Fatalf("requests after Decide = %d, want 0", got)
	}

	results := c.DecideMany(context.Background(), []decide.Request{
		tooManyChoicesRequest(),
		tooManyChoicesRequest(),
	})
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	for i, r := range results {
		if !errors.Is(r.Err, decide.ErrTooManyChoices) {
			t.Errorf("results[%d].Err = %v, want decide.ErrTooManyChoices", i, r.Err)
		}
	}
	if got := atomic.LoadInt64(&requests); got != 0 {
		t.Fatalf("requests after DecideMany = %d, want 0", got)
	}
}
