// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package e2e holds binary-level smoke tests: they build the real `engram`
// binary, run it as a subprocess, and drive it over the real MCP transport.
//
// This tier exists to cover the one seam no other test reaches. The
// internal/server and internal/store suites exercise buildDepsFromEnv, the tool
// handlers, and the store directly — so a break in the wiring BETWEEN them
// (cobra flags/env -> runServe -> mux -> MCP transport -> tool registration)
// leaves the entire suite green while the shipped binary is broken. Keep this
// package small and focused on that seam; assert tool SEMANTICS in
// internal/server, where it is faster and more precise.
package e2e

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"maps"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"

	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)

// engramBin is the binary under test, built once by TestMain. Always non-empty
// once TestMain returns: a build failure is fatal, never a skip — a test tier
// that silently stops building the thing it tests is worse than no tier.
var engramBin string

// testCollectionPrefix namespaces this package's integration-test Qdrant
// collection names so a single shared Qdrant instance (CI's
// ENGRAM_QDRANT_TEST_ADDR path) can host every Qdrant-backed package's test
// suite concurrently without cross-package name collisions (CONTEXT.md D-16).
// This package already generated collision-safe per-port names before this
// constant existed (startServer's "e2e_" + port); the constant makes that
// namespace the single source of truth rather than a comment about a literal
// elsewhere in the line.
const testCollectionPrefix = "e2e_"

// testCollection returns name namespaced into this package's collection space
// on the shared test Qdrant instance.
func testCollection(name string) string {
	return testCollectionPrefix + name
}

// newTestStore is the prefix-enforcing construction seam for every test Store
// built against a real Qdrant instance in this package. It asserts name
// carries this package's testCollectionPrefix before constructing the store,
// so a collection name that skips testCollection() fails the test naming the
// offending value — a runtime assertion a source-level check could be routed
// around, but a t.Fatalf inside the one function every test store is built by
// cannot (CONTEXT.md D-16, plan 01-05). spine_review_test.go's
// newSpineReviewStore is this package's only direct store.New call site;
// startServer's ENGRAM_QDRANT_COLLECTION path constructs its store inside the
// built binary's own subprocess, outside this seam's reach.
func newTestStore(t testing.TB, c *qdrant.Client, name string) *store.Store {
	t.Helper()
	if !strings.HasPrefix(name, testCollectionPrefix) {
		t.Fatalf("collection name %q does not carry this package's prefix %q: route it through testCollection()", name, testCollectionPrefix)
	}
	return store.New(c, name)
}

// TestMain builds the engram binary under test (orthogonal to storetest —
// no other package in this milestone needs a subprocess binary), then
// delegates Qdrant container lifecycle to storetest.Run: the
// ENGRAM_QDRANT_TEST_ADDR fast path, the bounded testcontainer fallback, and
// the ENGRAM_REQUIRE_QDRANT fail-closed gate all live there (plan 01-01).
// This package also now inherits storetest's post-boot "required but empty
// address" check, which its own copy lacked — a deliberate normalization
// toward the store/server harness (plan 01-03). The early
// storetest.RequireQdrant parse happens BEFORE the build so a malformed
// ENGRAM_REQUIRE_QDRANT fails fast instead of paying the build cost first,
// preserving today's ordering.
func TestMain(m *testing.M) {
	if _, err := storetest.RequireQdrant(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}

	tmp, err := os.MkdirTemp("", "engram-e2e-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: temp dir: %v\n", err)
		os.Exit(1)
	}

	engramBin = filepath.Join(tmp, "engram")
	build := exec.Command("go", "build", "-o", engramBin, "github.com/seanb4t/engram/cmd/engram")
	if out, berr := build.CombinedOutput(); berr != nil {
		fmt.Fprintf(os.Stderr, "fatal: build engram: %v\n%s\n", berr, out)
		_ = os.RemoveAll(tmp)
		os.Exit(1)
	}

	code := storetest.Run(m)
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

// TestSharedQdrantAddressHonored proves this package took the CI shared-Qdrant
// fast path rather than booting its own testcontainer, whenever
// ENGRAM_QDRANT_TEST_ADDR is set. Delegates to storetest, which holds the
// booted-container state TestMain populated.
func TestSharedQdrantAddressHonored(t *testing.T) {
	storetest.AssertSharedAddressHonored(t)
}

// childEnv builds the subprocess environment from SCRATCH rather than extending
// os.Environ().
//
// This is load-bearing, not tidiness. A developer shell commonly exports real
// ENGRAM_* values (ENGRAM_EMBED_DIM, ENGRAM_OPENAI_BASE_URL, ENGRAM_OPENAI_API_KEY
// pointing at a live gateway). Inheriting them makes the test assert against the
// machine instead of the code — and, worse, can send a real API key to whatever
// endpoint the test configures. Only PATH/HOME (needed for the Go toolchain and
// container discovery) plus the explicitly-passed ENGRAM_* vars cross into the child.
func childEnv(vars map[string]string) []string {
	env := make([]string, 0, 2+len(vars))
	env = append(env, "PATH="+os.Getenv("PATH"), "HOME="+os.Getenv("HOME"))
	for k, v := range vars {
		env = append(env, k+"="+v)
	}
	return env
}

// freePort reserves an ephemeral port and releases it. There is an inherent
// (tiny) race between release and the server's bind; the alternative — having
// engram report its own port — would require a production-code change purely
// for tests, which is the worse trade.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

// stubEmbedder serves an OpenAI-compatible POST /v1/embeddings, returning a
// deterministic unit vector per input so identical text embeds identically.
// In-process (httptest) rather than a second binary: the engram subprocess still
// reaches it over a real socket, so the outbound HTTP path is genuinely
// exercised, but lifecycle stays inside `go test`.
func stubEmbedder(t *testing.T, dim int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/embeddings", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Input any `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var inputs []string
		switch v := body.Input.(type) {
		case string:
			inputs = []string{v}
		case []any:
			for _, e := range v {
				s, _ := e.(string)
				inputs = append(inputs, s)
			}
		}
		if len(inputs) == 0 {
			inputs = []string{""}
		}
		data := make([]map[string]any, 0, len(inputs))
		for i, in := range inputs {
			data = append(data, map[string]any{"object": "embedding", "index": i, "embedding": unitVector(in, dim)})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data, "model": "stub"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// unitVector derives a deterministic L2-normalized vector from s. Qdrant
// normalizes vectors in cosine collections anyway (see g5pmdygqmv); normalizing
// here keeps the stub's output stable regardless.
func unitVector(s string, dim int) []float32 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	seed := h.Sum64()
	v := make([]float32, dim)
	var norm float64
	for i := range v {
		seed = seed*6364136223846793005 + 1442695040888963407
		x := float64(seed>>11) / float64(uint64(1)<<53)
		v[i] = float32(x)
		norm += x * x
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		norm = 1
	}
	for i := range v {
		v[i] = float32(float64(v[i]) / norm)
	}
	return v
}

// serverProc is a running engram subprocess.
type serverProc struct {
	addr string // host:port
}

// endpoint is the MCP transport URL.
func (s *serverProc) endpoint() string { return "http://" + s.addr + "/mcp" }

// baseURL is the server's HTTP origin, used by console_browser_test.go to
// reach the Connect API and the /ui/ static mount directly (neither goes
// through the MCP transport, so endpoint() does not apply).
func (s *serverProc) baseURL() string { return "http://" + s.addr }

// startServer launches `engram serve` with a controlled environment and waits
// for it to accept connections. extraEnv is merged over the baseline (and may
// override it) so a test can vary a single variable.
func startServer(t *testing.T, extraEnv map[string]string) *serverProc {
	t.Helper()
	qdrantAddr := storetest.Addr()
	if qdrantAddr == "" {
		storetest.SkipOrFailNoQdrant(t)
	}
	embed := stubEmbedder(t, 1024)
	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	env := map[string]string{
		"ENGRAM_QDRANT_ADDR":       qdrantAddr,
		"ENGRAM_QDRANT_COLLECTION": testCollection(strconv.FormatInt(int64(port), 10)),
		"ENGRAM_EMBED_DIM":         "1024",
		"ENGRAM_LISTEN_ADDR":       addr,
		"ENGRAM_OPENAI_BASE_URL":   embed.URL,
	}
	maps.Copy(env, extraEnv)

	logFile, err := os.CreateTemp(t.TempDir(), "serve-*.log")
	if err != nil {
		t.Fatalf("server log: %v", err)
	}
	cmd := exec.Command(engramBin, "serve")
	cmd.Env = childEnv(env)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		t.Fatalf("start engram serve: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		// Surface the server log on failure — a subprocess that died during
		// startup is otherwise invisible.
		if t.Failed() {
			if b, rerr := os.ReadFile(logFile.Name()); rerr == nil {
				t.Logf("engram serve log:\n%s", b)
			}
		}
	})

	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if c, derr := net.DialTimeout("tcp", addr, 500*time.Millisecond); derr == nil {
			_ = c.Close()
			return &serverProc{addr: addr}
		}
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			b, _ := os.ReadFile(logFile.Name())
			t.Fatalf("engram serve exited during startup:\n%s", b)
		}
		time.Sleep(100 * time.Millisecond)
	}
	b, _ := os.ReadFile(logFile.Name())
	t.Fatalf("engram serve did not listen on %s within 45s:\n%s", addr, b)
	return nil
}
