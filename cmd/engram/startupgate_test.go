// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/seanb4t/engram/internal/config"
)

// countingTarget counts the requests the gate hands to it.
type countingTarget struct{ calls atomic.Int64 }

func (c *countingTarget) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	c.calls.Add(1)
	w.WriteHeader(http.StatusTeapot)
}

// gated is a gate behind the listener's cross-origin protection, as
// newHTTPServer builds it.
func gated(g *startupGate) http.Handler { return newCrossOriginProtection().Handler(g) }

func serve(h http.Handler, method, path string, header map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	for k, v := range header {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func assertStarting(t *testing.T, w *httptest.ResponseRecorder, what string) {
	t.Helper()
	if w.Code != http.StatusServiceUnavailable || w.Header().Get("Retry-After") == "" {
		t.Errorf("%s before provisioning: status=%d Retry-After=%q, want 503 with Retry-After", what, w.Code, w.Header().Get("Retry-After"))
	}
}

func TestStartupGate(t *testing.T) {
	crossSite := map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"}

	t.Run("primary gate before and after provisioning", func(t *testing.T) {
		target := &countingTarget{}
		health := &healthLoop{}
		g := &startupGate{health: health}
		h := gated(g)

		assertStarting(t, serve(h, http.MethodPost, "/mcp", map[string]string{"Content-Type": "application/json"}), "MCP tools/list POST")
		assertStarting(t, serve(h, http.MethodPost, "/engram.v1.EngramService/ListMemories", map[string]string{"Content-Type": "application/json"}), "Connect call")
		assertStarting(t, serve(h, http.MethodGet, "/ui/", nil), "console GET")
		if w := serve(h, http.MethodGet, "/livez", nil); w.Code != http.StatusOK {
			t.Errorf("GET /livez before provisioning = %d, want 200", w.Code)
		}
		if w := serve(h, http.MethodGet, "/readyz", nil); w.Code != http.StatusServiceUnavailable {
			t.Errorf("GET /readyz before provisioning = %d, want 503", w.Code)
		}
		if w := serve(h, http.MethodPost, "/mcp", crossSite); w.Code != http.StatusForbidden {
			t.Errorf("cross-site POST before provisioning = %d, want 403", w.Code)
		}
		if n := target.calls.Load(); n != 0 {
			t.Fatalf("target called %d times before provisioning, want 0", n)
		}

		g.setTarget(target)
		if w := serve(h, http.MethodPost, "/mcp", map[string]string{"Content-Type": "application/json"}); w.Code != http.StatusTeapot {
			t.Errorf("MCP POST after provisioning = %d, want the target's answer", w.Code)
		}
		if w := serve(h, http.MethodGet, "/livez", nil); w.Code != http.StatusOK {
			t.Errorf("GET /livez with a failed ping = %d, want 200", w.Code)
		}
		if w := serve(h, http.MethodPost, "/mcp", crossSite); w.Code != http.StatusForbidden {
			t.Errorf("cross-site POST after provisioning = %d, want 403", w.Code)
		}
	})

	t.Run("MCP-only gate before provisioning", func(t *testing.T) {
		target := &countingTarget{}
		g := &startupGate{}
		w := serve(gated(g), http.MethodGet, "/ui/", nil)
		assertStarting(t, w, "console GET on the MCP-only listener")
		if strings.Contains(w.Body.String(), "<html") {
			t.Errorf("console content served before provisioning: %q", w.Body.String())
		}
		if w := serve(gated(g), http.MethodGet, "/livez", nil); w.Code != http.StatusServiceUnavailable {
			t.Errorf("GET /livez on the MCP-only gate = %d, want 503: it serves no health route", w.Code)
		}
		if n := target.calls.Load(); n != 0 {
			t.Fatalf("target called %d times, want 0", n)
		}
	})
}

func TestPropertyStartupGate(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		target := &countingTarget{}
		g := &startupGate{health: &healthLoop{}}
		method := rapid.SampledFrom([]string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodDelete}).Draw(t, "method")
		path := rapid.SampledFrom([]string{"/", "/mcp", "/livez", "/readyz", "/ui/", "/auth/login", "/engram.v1.EngramService/GetMemory", "/.well-known/oauth-protected-resource/mcp", "/x"}).Draw(t, "path")
		site := rapid.SampledFrom([]string{"", "same-origin", "same-site", "cross-site", "none"}).Draw(t, "Sec-Fetch-Site")
		origin := rapid.SampledFrom([]string{"", "https://evil.example"}).Draw(t, "Origin")
		header := map[string]string{}
		if site != "" {
			header["Sec-Fetch-Site"] = site
		}
		if origin != "" {
			header["Origin"] = origin
		}
		r := httptest.NewRequest(method, path, nil)
		for k, v := range header {
			r.Header.Set(k, v)
		}
		refused := newCrossOriginProtection().Check(r) != nil
		w := serve(gated(g), method, path, header)
		switch {
		case refused:
			if w.Code != http.StatusForbidden {
				t.Fatalf("%s %s refused by the cross-origin check: status %d, want 403", method, path, w.Code)
			}
		case method == http.MethodGet && (path == "/livez" || path == "/readyz"):
		default:
			if w.Code != http.StatusServiceUnavailable || w.Header().Get("Retry-After") == "" {
				t.Fatalf("%s %s before provisioning: status %d, want 503 with Retry-After", method, path, w.Code)
			}
		}
		if n := target.calls.Load(); n != 0 {
			t.Fatalf("target saw %d calls before provisioning", n)
		}
	})
}

func TestMarkProvisioned(t *testing.T) {
	gates := []*startupGate{{health: &healthLoop{}}, {}}
	started := false
	markProvisioned(gates, []http.Handler{http.NotFoundHandler(), http.NotFoundHandler()}, func() {
		for i, g := range gates {
			if !g.provisioned() {
				t.Errorf("gate %d has no target when the health loop starts", i)
			}
		}
		started = true
	})
	if !started {
		t.Fatal("startHealth was not called")
	}
}

// boundServing binds one gated listener on a free port and starts serving it.
func boundServing(t *testing.T) (*serving, []*startupGate, string) {
	t.Helper()
	l := newListeners(config.ServerConfig{ListenAddr: "127.0.0.1:0"}, &healthLoop{}, func() {})
	bound, err := listenAll(l)
	if err != nil {
		t.Fatalf("listenAll: %v", err)
	}
	return startServing(bound), gatesOf(l), bound[0].ln.Addr().String()
}

func assertClosed(t *testing.T, addr string) {
	t.Helper()
	if conn, err := net.Dial("tcp", addr); err == nil {
		_ = conn.Close()
		t.Fatalf("%s still accepts connections", addr)
	}
}

func TestStartServeSignal(t *testing.T) {
	sv, _, addr := boundServing(t)
	sig, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := startServe(sig, sv, func(ctx context.Context) (func(context.Context), error) {
		cancel()
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("startServe = %v, want the context error", err)
	}
	if got := startupResult(sig, err); got != nil {
		t.Fatalf("startupResult after a signal = %v, want nil", got)
	}
	assertClosed(t, addr)
}

func TestStartupResult(t *testing.T) {
	startErr := errors.New("quantization rejected")
	done, cancel := context.WithCancel(context.Background())
	cancel()
	if got := startupResult(done, startErr); got != nil {
		t.Errorf("startupResult with a done signal = %v, want nil", got)
	}
	if got := startupResult(context.Background(), startErr); !errors.Is(got, startErr) {
		t.Errorf("startupResult without a signal = %v, want the error unchanged", got)
	}
}

func TestStartServeProvisioningError(t *testing.T) {
	sv, gates, addr := boundServing(t)
	startErr := errors.New("schema provisioning: reconcile quantization (int8): rejected")
	_, err := startServe(context.Background(), sv, func(context.Context) (func(context.Context), error) {
		return nil, startErr
	})
	if !errors.Is(err, startErr) {
		t.Fatalf("startServe = %v, want the register error unchanged", err)
	}
	for i, g := range gates {
		if g.provisioned() {
			t.Errorf("gate %d has a target after a provisioning error", i)
		}
	}
	assertClosed(t, addr)
}

func TestPropertyReadiness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		var pings, warns atomic.Int64
		pingOK := rapid.Bool().Draw(t, "initial ok")
		var next atomic.Bool
		next.Store(pingOK)
		checked := make(chan struct{})
		health := &healthLoop{
			timeout: time.Second,
			ping: func(context.Context) error {
				pings.Add(1)
				if next.Load() {
					return nil
				}
				return errors.New("down")
			},
		}
		lastOK, first := false, true
		health.checked = func() {
			ok := next.Load()
			if !ok && (first || lastOK) {
				warns.Add(1)
			}
			first, lastOK = false, ok
			checked <- struct{}{}
		}
		g := &startupGate{health: health}
		ticks := make(chan time.Time)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		readyz := func() int { return serve(gated(g), http.MethodGet, "/readyz", nil).Code }
		if got := readyz(); got != http.StatusServiceUnavailable {
			t.Fatalf("/readyz before provisioning = %d, want 503", got)
		}
		markProvisioned([]*startupGate{g}, []http.Handler{http.NotFoundHandler()}, func() { go health.run(ctx, ticks) })
		<-checked
		nTicks, wantWarns := 0, int64(0)
		if !pingOK {
			wantWarns = 1
		}
		prev := pingOK
		for range rapid.IntRange(0, 20).Draw(t, "steps") {
			switch rapid.SampledFrom([]string{"tick-ok", "tick-fail", "readyz"}).Draw(t, "action") {
			case "tick-ok":
				next.Store(true)
				ticks <- time.Time{}
				<-checked
				nTicks++
				prev = true
			case "tick-fail":
				next.Store(false)
				ticks <- time.Time{}
				<-checked
				nTicks++
				if prev {
					wantWarns++
				}
				prev = false
			case "readyz":
				want := http.StatusServiceUnavailable
				if prev {
					want = http.StatusOK
				}
				if got := readyz(); got != want {
					t.Fatalf("/readyz = %d, want %d", got, want)
				}
			}
			if p := pings.Load(); p != int64(nTicks+1) {
				t.Fatalf("pings = %d after %d ticks, want ticks+1", p, nTicks)
			}
		}
		if w := warns.Load(); w != wantWarns {
			t.Fatalf("warn transitions = %d, want %d", w, wantWarns)
		}
	})
}

func TestReadinessInitialPing(t *testing.T) {
	t.Run("ready after the initial ping, with no tick", func(t *testing.T) {
		checked := make(chan struct{}, 1)
		health := &healthLoop{timeout: time.Second, ping: func(context.Context) error { return nil }, checked: func() { checked <- struct{}{} }}
		g := &startupGate{health: health}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		markProvisioned([]*startupGate{g}, []http.Handler{http.NotFoundHandler()}, func() { go health.run(ctx, nil) })
		<-checked
		if got := serve(gated(g), http.MethodGet, "/readyz", nil).Code; got != http.StatusOK {
			t.Fatalf("/readyz after the initial ping = %d, want 200", got)
		}
	})

	t.Run("not ready while the initial ping is open", func(t *testing.T) {
		release, checked := make(chan struct{}), make(chan struct{}, 1)
		health := &healthLoop{timeout: time.Minute, ping: func(context.Context) error { <-release; return nil }, checked: func() { checked <- struct{}{} }}
		g := &startupGate{health: health}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		markProvisioned([]*startupGate{g}, []http.Handler{http.NotFoundHandler()}, func() { go health.run(ctx, nil) })
		if got := serve(gated(g), http.MethodGet, "/readyz", nil).Code; got != http.StatusServiceUnavailable {
			t.Fatalf("/readyz with the initial ping open = %d, want 503", got)
		}
		close(release)
		<-checked
		if got := serve(gated(g), http.MethodGet, "/readyz", nil).Code; got != http.StatusOK {
			t.Fatalf("/readyz after the initial ping = %d, want 200", got)
		}
	})
}
