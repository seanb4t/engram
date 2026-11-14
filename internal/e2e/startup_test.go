// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package e2e

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/seanb4t/engram/internal/store/storetest"
)

const (
	startupToken    = "e2e-startup-token-value"
	attemptFailedLn = "qdrant connect-and-create attempt failed"
	mcpToolsList    = `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
)

// gateProxy forwards TCP connections to target over IPv4 once it is opened.
// Until then it closes each connection as soon as it accepts it.
type gateProxy struct {
	ln     net.Listener
	target string
	open   atomic.Bool
	wg     sync.WaitGroup
}

func newGateProxy(t *testing.T, target string) *gateProxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("proxy listen: %v", err)
	}
	p := &gateProxy{ln: ln, target: target}
	p.wg.Go(p.serve)
	t.Cleanup(func() {
		_ = ln.Close()
		p.wg.Wait()
	})
	return p
}

func (p *gateProxy) serve() {
	for {
		c, err := p.ln.Accept()
		if err != nil {
			return
		}
		if !p.open.Load() {
			_ = c.Close()
			continue
		}
		p.wg.Go(func() { p.forward(c) })
	}
}

func (p *gateProxy) forward(c net.Conn) {
	defer c.Close()
	up, err := net.Dial("tcp4", p.target)
	if err != nil {
		return
	}
	defer up.Close()
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(up, c); done <- struct{}{} }()
	go func() { _, _ = io.Copy(c, up); done <- struct{}{} }()
	<-done
}

// waitForLog polls the server log until it holds at least n lines that
// contain substr.
func waitForLog(t *testing.T, srv *serverProc, substr string, n int) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for strings.Count(srv.logText(), substr) < n {
		select {
		case <-srv.done:
			t.Fatalf("engram serve exited while waiting for %d %q lines:\n%s", n, substr, srv.logText())
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("no %d %q lines within 30s:\n%s", n, substr, srv.logText())
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func get(t *testing.T, url string) int {
	t.Helper()
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

func TestServeWaitsForQdrant(t *testing.T) {
	if storetest.Addr() == "" {
		storetest.SkipOrFailNoQdrant(t)
	}
	proxy := newGateProxy(t, storetest.Addr())
	srv := launchServer(t, map[string]string{
		"ENGRAM_QDRANT_ADDR":                proxy.ln.Addr().String(),
		"ENGRAM_SERVICE_AUTH_STATIC_TOKENS": "e2e=" + startupToken,
	})
	waitForLog(t, srv, attemptFailedLn, 2)

	conn, err := net.DialTimeout("tcp", srv.addr, 2*time.Second)
	if err != nil {
		t.Fatalf("TCP connect during provisioning: %v", err)
	}
	_ = conn.Close()
	if got := get(t, srv.baseURL()+"/livez"); got != http.StatusOK {
		t.Errorf("GET /livez during provisioning = %d, want 200", got)
	}
	if got := get(t, srv.baseURL()+"/readyz"); got != http.StatusServiceUnavailable {
		t.Errorf("GET /readyz during provisioning = %d, want 503", got)
	}
	resp, _ := mcpRequest(t, http.MethodPost, srv.endpoint(), "", startupToken, mcpToolsList, nil)
	if resp.StatusCode != http.StatusServiceUnavailable || resp.Header.Get("Retry-After") == "" {
		t.Errorf("MCP tools/list during provisioning: status=%d Retry-After=%q, want 503 with Retry-After", resp.StatusCode, resp.Header.Get("Retry-After"))
	}

	proxy.open.Store(true)
	srv.waitReady(t, 10*time.Second)
	for line := range strings.SplitSeq(srv.logText(), "\n") {
		if strings.Contains(line, attemptFailedLn) && !strings.Contains(line, `"level":"WARN"`) {
			t.Errorf("failed attempt not logged at warn level: %s", line)
		}
	}
	checkDocumentedTools(t, mcpConnectAt(t, srv.endpoint(), "", startupToken))
}

func TestServeStartupTimeout(t *testing.T) {
	closed := fmt.Sprintf("127.0.0.1:%d", freePort(t))
	silent, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	var held sync.WaitGroup
	t.Cleanup(func() {
		_ = silent.Close()
		held.Wait()
	})
	held.Go(func() {
		var conns []net.Conn
		defer func() {
			for _, c := range conns {
				_ = c.Close()
			}
		}()
		for {
			c, err := silent.Accept()
			if err != nil {
				return
			}
			conns = append(conns, c)
		}
	})

	for name, addr := range map[string]string{"closed port": closed, "silent listener": silent.Addr().String()} {
		t.Run(name, func(t *testing.T) {
			srv := launchServer(t, map[string]string{
				"ENGRAM_QDRANT_ADDR":            addr,
				"ENGRAM_QDRANT_STARTUP_TIMEOUT": "2s",
			})
			if code := srv.exitCode(t, 15*time.Second); code != 5 {
				t.Fatalf("exit code = %d, want 5:\n%s", code, srv.logText())
			}
			log := srv.logText()
			for _, want := range []string{"connect-and-create", "ENGRAM_QDRANT_STARTUP_TIMEOUT"} {
				if !strings.Contains(log, want) {
					t.Errorf("log does not name %q:\n%s", want, log)
				}
			}
		})
	}
}

func TestServeSignalDuringStartup(t *testing.T) {
	srv := launchServer(t, map[string]string{
		"ENGRAM_QDRANT_ADDR":            fmt.Sprintf("127.0.0.1:%d", freePort(t)),
		"ENGRAM_QDRANT_STARTUP_TIMEOUT": "60s",
	})
	waitForLog(t, srv, attemptFailedLn, 1)
	if err := srv.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("SIGTERM: %v", err)
	}
	if code := srv.exitCode(t, 20*time.Second); code != 0 {
		t.Fatalf("exit code after SIGTERM during startup = %d, want 0:\n%s", code, srv.logText())
	}
}
