// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestMountConnectDefaultOffWithoutUIOrHeadlessFlag is the composition-level
// assertion (REVIEWS.md LOW-11, D-12/SC5): the shape connectResolverFor
// produces when neither activation boolean is set (UI off, headless unset)
// is exactly NewConnectResolver(nil, nil) -- and passing THAT result to
// mountConnect registers no handler, so a POST to the EngramService
// procedure path gets a 404.
//
// This is deliberately NOT a restatement of TestMountConnectSkipsWhenResolverNil
// (connectapi_test.go), which already covers a hand-passed literal nil. This
// test's value is the composition -> mount link: it proves that the value
// cmd/engram's connectResolverFor would build in the both-off case, when fed
// through the real NewConnectResolver constructor, still ends up unmounted.
// Do not "simplify" the two tests back together -- they pin different
// obligations.
func TestMountConnectDefaultOffWithoutUIOrHeadlessFlag(t *testing.T) {
	d := &deps{} // no store needed; we never serve a request
	mux := http.NewServeMux()

	// The shape connectResolverFor(chain, false, nil) produces when
	// cookieResolve == nil && !headless -- both activation booleans off.
	resolve := NewConnectResolver(nil, nil)
	if resolve != nil {
		t.Fatal("NewConnectResolver(nil, nil) != nil, want nil")
	}

	if err := d.mountConnect(mux, resolve, nil, nil); err != nil {
		t.Fatalf("mountConnect: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/engram.v1.EngramService/ListScopes", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 (Connect must be unmounted when neither UI nor headless is active)", rec.Code)
	}
}
