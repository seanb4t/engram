// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package storetest provides shared test support for every Qdrant-backed
// test package in this module (the net/http/httptest idiom; see
// internal/testhttp for this repo's own non-_test.go precedent). It is a
// normal (non-_test.go) file in an internal package rather than a _test.go
// helper because Go cannot share a _test.go across package boundaries, and
// every Qdrant-backed test package needs the same container lifecycle,
// fail-closed ENGRAM_REQUIRE_QDRANT parser, dial helper, and (in seed.go)
// oversized-fixture seeder.
//
// Unlike internal/testhttp, storetest deliberately imports "testing" and
// testcontainers — it must never be imported by production code (a
// module-wide gate proves it never reaches cmd/engram's dependency graph).
// It reads exactly two environment variables — ENGRAM_QDRANT_TEST_ADDR and
// ENGRAM_REQUIRE_QDRANT — and never names the production Qdrant address
// variable, even conceptually: the instance a test dials must never be able
// to drift onto an operator's real deployment.
package storetest

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/store"
	tcqdrant "github.com/testcontainers/testcontainers-go/modules/qdrant"
	"google.golang.org/grpc"
)

// RecvLimit is the receive limit every oversized regression test in this
// milestone names explicitly and passes to both Dial and the seeder — the
// ceiling production effectively runs at today. Tests keep pinning this
// constant after the production backstop lands (REQ-recv-limit-backstop) so
// a passing test proves the bounded-read mechanism, not the backstop; never
// derived from grpc-go's own default (rule m45p2b4bp7).
const RecvLimit = 4 << 20

// QdrantImage is the one pinned Qdrant image tag used by every testcontainer
// fallback across this module. CI's services.qdrant.image must stay
// byte-identical to this value; bumping it requires re-verifying the TOCTOU
// contract guarded by internal/store's separate qdrantTOCTOUVerifiedVersion.
const QdrantImage = "qdrant/qdrant:v1.19.1"

// Package state set only by Run.
var (
	ran           bool
	ignoreRequire bool
	envAddr       string
	addr          string
	booted        bool
)

// runConfig holds Run's optional behavior, configured by the Options passed
// to Run.
type runConfig struct {
	ignoreRequire bool
}

// Option configures Run's behavior.
type Option func(*runConfig)

// IgnoreRequireQdrant configures Run to never parse or act on
// ENGRAM_REQUIRE_QDRANT. For a package whose pre-existing harness never
// consulted the env var (internal/retrievaleval): with this option, Run
// never parses it and SkipOrFailNoQdrant always skips rather than failing.
func IgnoreRequireQdrant() Option {
	return func(c *runConfig) { c.ignoreRequire = true }
}

// RequireQdrant is the single ENGRAM_REQUIRE_QDRANT parser. Unset or empty
// returns (false, nil) — local dev ergonomics unchanged, integration tests
// still skip without Qdrant. A truthy/falsey value parses via
// strconv.ParseBool. Any other value returns a non-nil error — it is NEVER
// coerced to false, which would silently re-enable skipping under a
// misconfigured CI job.
func RequireQdrant() (bool, error) {
	v := os.Getenv("ENGRAM_REQUIRE_QDRANT")
	if v == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("ENGRAM_REQUIRE_QDRANT: invalid value %q: %w", v, err)
	}
	return b, nil
}

// SkipOrFailNoQdrant skips t with a message naming how to get Qdrant, unless
// Run was configured with IgnoreRequireQdrant or ENGRAM_REQUIRE_QDRANT is
// set truthy — in which case it fails instead of skipping, so a
// Qdrant-backed test required by CI can never report green by skipping.
func SkipOrFailNoQdrant(t testing.TB) {
	t.Helper()
	if !ignoreRequire {
		required, err := RequireQdrant()
		if err != nil {
			t.Fatalf("%v", err)
		}
		if required {
			t.Fatal("no Qdrant available and ENGRAM_REQUIRE_QDRANT is set: failing instead of skipping")
		}
	}
	t.Skip("no Qdrant available: set ENGRAM_QDRANT_TEST_ADDR or start Docker (testcontainers)")
}

// Run provisions Qdrant for the calling package's integration tests and
// returns the exit code the caller's TestMain should exit with — it never
// calls os.Exit itself, so a caller can run its own cleanup (e.g. removing a
// temp dir) before exiting. It prefers an existing instance named by
// ENGRAM_QDRANT_TEST_ADDR (the shared CI Qdrant fast path); otherwise it
// boots an ephemeral Qdrant via testcontainers and tears it down when the
// suite finishes. If neither is available, the suite still runs and every
// SkipOrFailNoQdrant-gated test skips — UNLESS ENGRAM_REQUIRE_QDRANT is set
// and IgnoreRequireQdrant was not passed, in which case Run returns 1
// instead of letting the suite run with Qdrant-backed tests silently
// skipped.
func Run(m *testing.M, opts ...Option) int {
	ran = true
	cfg := runConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	ignoreRequire = cfg.ignoreRequire

	// Capture the caller's env var BEFORE anything below mutates it (Run
	// exports the booted container's address onto the same name).
	envAddr = os.Getenv("ENGRAM_QDRANT_TEST_ADDR")

	var required bool
	if !ignoreRequire {
		var rerr error
		required, rerr = RequireQdrant()
		if rerr != nil {
			fmt.Fprintf(os.Stderr, "fatal: %v\n", rerr)
			return 1
		}
	}

	if envAddr != "" {
		addr = envAddr
		return m.Run()
	}

	// Bound startup so an unreachable daemon or a stalled image pull fails
	// fast instead of hanging the suite.
	startCtx, startCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	container, err := tcqdrant.Run(startCtx, QdrantImage)
	if err != nil {
		startCancel()
		fmt.Fprintf(os.Stderr, "qdrant testcontainer unavailable (%v); integration tests will skip — set ENGRAM_QDRANT_TEST_ADDR or start Docker\n", err)
		if required {
			fmt.Fprintln(os.Stderr, "fatal: ENGRAM_REQUIRE_QDRANT is set — failing instead of skipping")
			return 1
		}
		return m.Run()
	}
	addr, err = container.GRPCEndpoint(startCtx)
	startCancel()
	if err != nil {
		terminate(container)
		fmt.Fprintf(os.Stderr, "qdrant grpc endpoint: %v\n", err)
		return 1
	}
	booted = true
	if required && addr == "" {
		terminate(container)
		fmt.Fprintln(os.Stderr, "fatal: ENGRAM_REQUIRE_QDRANT is set but no Qdrant address resolved")
		return 1
	}
	// Export the booted address so in-package tests of Qdrant-backed
	// packages that cannot import storetest (an import cycle: storetest
	// imports store) still receive it through the pre-existing
	// ENGRAM_QDRANT_TEST_ADDR contract.
	if setErr := os.Setenv("ENGRAM_QDRANT_TEST_ADDR", addr); setErr != nil {
		terminate(container)
		fmt.Fprintf(os.Stderr, "fatal: export booted Qdrant address: %v\n", setErr)
		return 1
	}
	code := m.Run()
	terminate(container)
	return code
}

// terminate tears container down under a bounded context so a slow Docker
// shutdown cannot hang the suite. Its error is deliberately discarded: a
// teardown failure once the suite has already finished running is not
// actionable.
func terminate(c *tcqdrant.QdrantContainer) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = c.Terminate(ctx)
}

// Addr returns the Qdrant gRPC address Run resolved: either the value of
// ENGRAM_QDRANT_TEST_ADDR, or the address of the testcontainer Run booted.
// Empty when Run has not run, or ran but no Qdrant is available.
func Addr() string { return addr }

// EnvAddr returns the value ENGRAM_QDRANT_TEST_ADDR carried when Run
// started — captured before Run exports a booted container's address onto
// that same variable. Empty when the caller did not set it.
func EnvAddr() string { return envAddr }

// ContainerBooted reports whether Run booted its own testcontainer, as
// opposed to taking the ENGRAM_QDRANT_TEST_ADDR fast path onto a shared
// instance.
func ContainerBooted() bool { return booted }

// AssertSharedAddressHonored is the body every package's CI-counted
// shared-address test delegates to: it proves the calling package took the
// CI shared-Qdrant fast path rather than booting its own testcontainer,
// whenever ENGRAM_QDRANT_TEST_ADDR was set. Skips (rather than fails) when
// the env var is unset — a developer running locally without it is not the
// case this assertion is about.
func AssertSharedAddressHonored(t testing.TB) {
	t.Helper()
	if !ran {
		t.Fatal("storetest.Run has not run — AssertSharedAddressHonored must be called from a package whose TestMain calls storetest.Run")
	}
	if EnvAddr() == "" {
		t.Skip("ENGRAM_QDRANT_TEST_ADDR not set: this test only asserts the shared-instance path")
	}
	if Addr() != EnvAddr() {
		t.Errorf("Addr() = %q, want %q (shared CI Qdrant address not honored)", Addr(), EnvAddr())
	}
	if ContainerBooted() {
		t.Error("ContainerBooted() = true, want false: ENGRAM_QDRANT_TEST_ADDR was set but this package booted its own testcontainer anyway")
	}
}

// Dial dials the integration-test Qdrant and returns the bare client,
// applying recvLimit (in upstream grpc vocabulary) and any caller-supplied
// opts through store.NewQdrantClient — the same constructor production
// uses. Skips (or fails, per SkipOrFailNoQdrant) when no Qdrant is
// available. Dial registers no Close: every pre-convergence dial site left
// its client open for the process lifetime, and closing is a lifecycle
// change this phase does not make.
func Dial(t testing.TB, recvLimit int, opts ...grpc.DialOption) *qdrant.Client {
	t.Helper()
	if !ran {
		t.Fatal("storetest.Run has not run — call storetest.Run(m) from the package's TestMain before dialing")
	}
	dialOpts, err := dialOptions(recvLimit, opts)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if addr == "" {
		SkipOrFailNoQdrant(t)
	}
	host, port, err := splitAddr(addr)
	if err != nil {
		t.Fatalf("%v", err)
	}
	// An assignment, not a direct return: the client-holder gate's
	// never-writes check (D-13) needs a client-bound identifier in this
	// file to verify against.
	c, err := store.NewQdrantClient(host, port, dialOpts...)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	return c
}

// dialOptions returns opts, in their original order, followed LAST by the
// named receive limit as a grpc.WithDefaultCallOptions call wrapping
// grpc.MaxCallRecvMsgSize — so no caller-supplied option can widen the
// named limit. As of Phase 5, that includes store.NewQdrantClient's own
// 64 MiB productionRecvLimit backstop: this append-LAST ordering is what
// keeps every test dialed through Dial running at the limit it names
// instead of silently inheriting the wider production ceiling. recvLimit
// must be a positive byte count; it is passed through to that call
// unconverted — no unit conversion, rounding, or narrowing (rule
// m45p2b4bp7).
func dialOptions(recvLimit int, opts []grpc.DialOption) ([]grpc.DialOption, error) {
	if recvLimit <= 0 {
		return nil, fmt.Errorf("storetest: recvLimit must be a positive byte count naming the limit under test, got %d", recvLimit)
	}
	out := make([]grpc.DialOption, 0, len(opts)+1)
	out = append(out, opts...)
	out = append(out, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(recvLimit)))
	return out, nil
}

// splitAddr splits addr into a host and a validated 1..65535 port.
func splitAddr(addr string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid Qdrant address %q: %w", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid Qdrant port %q (from %q): %w", portStr, addr, err)
	}
	if port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("invalid Qdrant port %d (from %q): out of range 1..65535", port, addr)
	}
	return host, port, nil
}
