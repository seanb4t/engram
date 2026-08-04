// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// startHungServer returns the base URL of a real HTTP server that accepts
// every connection and never writes a response. The handler primarily
// blocks on the request context's Done channel -- never a fixed sleep --
// which is what lets it unblock immediately if the client's own connection
// teardown ever cancels it. A bare net.Listener that only accepts would not
// do: the Connect client needs a real HTTP endpoint for the deadline to
// surface as connect.CodeDeadlineExceeded rather than as a raw transport
// error (see exitCodeForConnectErr's own comment in client_common.go).
//
// It also selects on a release channel closed from t.Cleanup, BEFORE
// httptest.Server.Close() is called. This is required, not decorative:
// connectrpc.com/connect's duplex HTTP call writes the request body through
// an io.Pipe in a background goroutine, and confirmed empirically here (a
// throwaway repro against a bare net/http client vs. the generated Connect
// client against this exact handler shape) that connect-go's client
// returning context.DeadlineExceeded to the caller does NOT reliably close
// the underlying TCP connection -- unlike a plain http.Client, whose
// Transport does close it on ctx cancellation. Relying on
// r.Context().Done() alone left the handler goroutine blocked indefinitely,
// which made httptest.Server.Close() (which waits for every outstanding
// request to finish) hang the whole test binary well past the 60s budget.
// The release channel is the deterministic unblock this suite needs
// regardless of what the client library's connection-teardown timing turns
// out to be.
func startHungServer(t *testing.T) string {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() {
		close(release)
		srv.Close()
	})
	return srv.URL
}

// closedPortURL opens a listener on 127.0.0.1:0, records the OS-assigned
// address, and closes it immediately: the address is real but nothing is
// listening on it, so a dial against it fails fast with connection-refused
// rather than hanging. The port is never hard-coded, so this cannot
// collide with anything else running on the machine.
func closedPortURL(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatalf("closing listener: %v", err)
	}
	return "http://" + addr
}

// TestTimeoutHungServerExitsTimeout proves the deadline this plan wires
// into every client RPC call site actually bounds it: a server that
// accepts the connection and never answers must not block the invocation
// forever. Each subtest supplies a short explicit --timeout so the hung
// rows stay well under a couple of seconds each, keeping the whole package
// suite inside its 60s feedback budget (01-VALIDATION.md).
func TestTimeoutHungServerExitsTimeout(t *testing.T) {
	url := startHungServer(t)

	t.Run("search", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, searchCmd)
		start := time.Now()
		_, _, err := runClient(t, "search", "--server", url, "--scope", "s", "--query", "q", "--timeout", "300ms")
		elapsed := time.Since(start)
		if elapsed > 2*time.Second {
			t.Errorf("search against a hung server took %s, want well under 2s", elapsed)
		}
		if got := exitCodeFromError(err); got != exitTimeout {
			t.Errorf("exitCodeFromError(err) = %d, want exitTimeout (%d); err=%v", got, exitTimeout, err)
		}
	})

	t.Run("list", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, listCmd)
		start := time.Now()
		_, _, err := runClient(t, "list", "--server", url, "--scope", "s", "--timeout", "300ms")
		elapsed := time.Since(start)
		if elapsed > 2*time.Second {
			t.Errorf("list against a hung server took %s, want well under 2s", elapsed)
		}
		if got := exitCodeFromError(err); got != exitTimeout {
			t.Errorf("exitCodeFromError(err) = %d, want exitTimeout (%d); err=%v", got, exitTimeout, err)
		}
	})

	t.Run("store", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, storeCmd)
		start := time.Now()
		_, _, err := runClient(t, "store", "--server", url, "--content", "c", "--scope", "s", "--timeout", "300ms")
		elapsed := time.Since(start)
		if elapsed > 2*time.Second {
			t.Errorf("store against a hung server took %s, want well under 2s", elapsed)
		}
		if got := exitCodeFromError(err); got != exitTimeout {
			t.Errorf("exitCodeFromError(err) = %d, want exitTimeout (%d); err=%v", got, exitTimeout, err)
		}
	})
}

// TestTimeoutDistinctFromUnavailable proves the hung-server case (exit 6)
// and a closed-port case (exit 5) are DISTINCT process exit codes, observed
// in the same test run -- not merely "both nonzero". Per memory
// nczgrtfec2 / 667p88n2be, a loose assertion that both codes are "as
// expected" would still pass on a silent classification collapse; this
// asserts the concrete values and their inequality.
func TestTimeoutDistinctFromUnavailable(t *testing.T) {
	hungURL := startHungServer(t)
	closedURL := closedPortURL(t)

	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	_, _, hungErr := runClient(t, "search", "--server", hungURL, "--scope", "s", "--query", "q", "--timeout", "300ms")
	hungCode := exitCodeFromError(hungErr)
	if hungCode != exitTimeout {
		t.Fatalf("hung-server exit code = %d, want exitTimeout (%d); err=%v", hungCode, exitTimeout, hungErr)
	}

	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	_, _, closedErr := runClient(t, "search", "--server", closedURL, "--scope", "s", "--query", "q", "--timeout", "5s")
	closedCode := exitCodeFromError(closedErr)
	if closedCode != exitUnavailable {
		t.Fatalf("closed-port exit code = %d, want exitUnavailable (%d); err=%v", closedCode, exitUnavailable, closedErr)
	}

	if hungCode == closedCode {
		t.Errorf("hung-server code (%d) and closed-port code (%d) must be DISTINCT, got equal", hungCode, closedCode)
	}
}

// TestTimeoutSuccessInsideDeadline proves the deadline adds nothing to the
// success path: a call well inside its --timeout still exits 0, and its
// rendered output is byte-for-byte identical to the same call made with the
// default --timeout.
func TestTimeoutSuccessInsideDeadline(t *testing.T) {
	svc := &stubEngramService{
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{
				Memories: []*engramv1.Memory{{ShortId: "AAAA111111", Scope: "s"}},
			}, nil
		},
	}
	url := startStubServer(t, svc)

	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	stdoutWithTimeout, _, err := runClient(t, "search", "--server", url, "--scope", "s", "--query", "q", "--timeout", "5s")
	if got := exitCodeFromError(err); got != exitOK {
		t.Fatalf("exitCodeFromError(err) = %d, want exitOK (%d); err=%v", got, exitOK, err)
	}

	resetClientFlags(t)
	resetCommandFlagState(t, searchCmd)
	stdoutDefault, _, err := runClient(t, "search", "--server", url, "--scope", "s", "--query", "q")
	if got := exitCodeFromError(err); got != exitOK {
		t.Fatalf("exitCodeFromError(err) = %d, want exitOK (%d); err=%v", got, exitOK, err)
	}

	if stdoutWithTimeout == "" {
		t.Fatal("expected non-empty stdout")
	}
	if stdoutWithTimeout != stdoutDefault {
		t.Errorf("output with an explicit --timeout differs from the default-timeout output:\nwith --timeout: %q\ndefault:        %q",
			stdoutWithTimeout, stdoutDefault)
	}
}
