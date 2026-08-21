// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/authz"
	"github.com/seanb4t/engram/internal/migrate"
	"github.com/seanb4t/engram/internal/shortid"
	tcqdrant "github.com/testcontainers/testcontainers-go/modules/qdrant"
)

// qdrantImageTag is the Qdrant image the integration suite boots via
// testcontainers. The SetPayload-on-deleted-point fail-closed contract that
// TestSetVisibilityTOCTOU depends on was verified against this image; bumping it
// MUST be paired with re-verifying that contract and updating
// qdrantTOCTOUVerifiedVersion below (the version guard there will fail loudly
// until you do).
const qdrantImageTag = "qdrant/qdrant:v1.18.2"

// qdrantTOCTOUVerifiedVersion is the server version (as reported by HealthCheck)
// whose SetPayload point-ID NotFound semantics TestSetVisibilityTOCTOU was
// written against. Deliberately a SEPARATE constant from qdrantImageTag so a
// version bump trips the guard in TestSetVisibilityTOCTOU and forces a conscious
// re-verification rather than silently tracking the new image.
const qdrantTOCTOUVerifiedVersion = "1.18.2"

// testQdrantAddr is the gRPC host:port the integration tests run against. Set by
// TestMain: ENGRAM_QDRANT_TEST_ADDR if provided (fast path / override), else an
// ephemeral testcontainer. Empty when neither is available (Docker absent), in
// which case the integration tests skip.
var testQdrantAddr string

// testQdrantContainerBooted records whether TestMain booted its OWN
// testcontainer for this run, as opposed to taking the ENGRAM_QDRANT_TEST_ADDR
// fast path onto a shared instance. Set true only inside the testcontainer
// branch, immediately after the container's gRPC endpoint resolves; the env-var
// branch leaves it false. TestSharedQdrantAddressHonored asserts on it directly
// so "the CI test job uses one shared Qdrant" is a checkable claim rather than
// an inference from logs (CONTEXT.md D-20).
var testQdrantContainerBooted bool

// testCollectionPrefix namespaces this package's integration-test Qdrant
// collection names so a single shared Qdrant instance (CI's ENGRAM_QDRANT_TEST_ADDR
// path) can host internal/store's and internal/server's test suites
// concurrently without their previously-identical "mem_eval_test" collection
// names colliding (CONTEXT.md D-16).
const testCollectionPrefix = "store_"

// testCollection returns name namespaced into this package's collection space
// on the shared test Qdrant instance.
func testCollection(name string) string {
	return testCollectionPrefix + name
}

// newTestStore is the prefix-enforcing construction seam for every test Store
// built against a real Qdrant instance in this package. It asserts name
// carries this package's testCollectionPrefix before constructing the store,
// so a collection name that skips testCollection() fails the test naming the
// offending value. This is a runtime assertion on purpose: a source-level
// check alone could be routed around by assigning the raw name to a variable
// first, but a t.Fatalf inside the one function every test store is built by
// cannot (CONTEXT.md D-16, plan 01-05). opts is threaded through to New
// verbatim so newSpineTestStore's WithClock/WithAuthz callers stay covered
// by the same seam as every unopted construction.
func newTestStore(t testing.TB, c *qdrant.Client, name string, opts ...Option) *Store {
	t.Helper()
	if !strings.HasPrefix(name, testCollectionPrefix) {
		t.Fatalf("collection name %q does not carry this package's prefix %q: route it through testCollection()", name, testCollectionPrefix)
	}
	return New(c, name, opts...)
}

// requireQdrant is the SOLE place ENGRAM_REQUIRE_QDRANT is read/parsed in this
// package, mirroring internal/server/tools_test.go's requireQdrant: TestMain and
// dialTestClient act only on its result, never parsing the env var themselves.
// Unset/empty -> (false, nil): local dev ergonomics unchanged (integration tests
// still skip without Qdrant). A truthy/falsey value parses via
// strconv.ParseBool. Any non-empty invalid value returns a non-nil error rather
// than being coerced to false — coercing a parse error to false would silently
// re-enable skipping and defeat the fail-closed gate the CI `test` job relies on.
func requireQdrant() (bool, error) {
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

// TestMain provisions Qdrant for this package's integration tests. It prefers an
// existing instance via ENGRAM_QDRANT_TEST_ADDR; otherwise it boots an ephemeral
// Qdrant via testcontainers and tears it down afterward. If neither is available
// the suite still runs and the integration tests skip with a clear message —
// UNLESS ENGRAM_REQUIRE_QDRANT is set (mirrors internal/server/tools_test.go: the
// CI `test` job sets it), in which case TestMain exits non-zero instead of
// letting the suite run with the real-store authz gate silently skipped.
func TestMain(m *testing.M) {
	required, rerr := requireQdrant()
	if rerr != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", rerr)
		os.Exit(1)
	}
	if addr := os.Getenv("ENGRAM_QDRANT_TEST_ADDR"); addr != "" {
		testQdrantAddr = addr
		os.Exit(m.Run())
	}
	// Bound startup so an unreachable daemon or a stalled image pull fails fast
	// instead of hanging the suite. os.Exit skips defers, so cancel explicitly.
	startCtx, startCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	container, err := tcqdrant.Run(startCtx, qdrantImageTag)
	if err != nil {
		startCancel()
		fmt.Fprintf(os.Stderr, "qdrant testcontainer unavailable (%v); integration tests will skip — set ENGRAM_QDRANT_TEST_ADDR or start Docker\n", err)
		if required {
			fmt.Fprintln(os.Stderr, "fatal: ENGRAM_REQUIRE_QDRANT is set — failing instead of skipping")
			os.Exit(1)
		}
		os.Exit(m.Run())
	}
	testQdrantAddr, err = container.GRPCEndpoint(startCtx)
	startCancel()
	if err != nil {
		terminateQdrant(container)
		fmt.Fprintf(os.Stderr, "qdrant grpc endpoint: %v\n", err)
		os.Exit(1)
	}
	testQdrantContainerBooted = true
	if required && testQdrantAddr == "" {
		terminateQdrant(container)
		fmt.Fprintln(os.Stderr, "fatal: ENGRAM_REQUIRE_QDRANT is set but no Qdrant address resolved")
		os.Exit(1)
	}
	code := m.Run()
	terminateQdrant(container)
	os.Exit(code)
}

// terminateQdrant tears down the container under a bounded context so a slow
// Docker shutdown cannot hang the suite.
func terminateQdrant(c *tcqdrant.QdrantContainer) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = c.Terminate(ctx)
}

// dialTestClient dials the integration-test Qdrant and returns the bare client.
// Skips when no Qdrant is available. It is the single dialing primitive: testStore
// wraps it with a ready collection, and the reindex tests use it directly to drive
// two collections and read points back verbatim.
func dialTestClient(t *testing.T) *qdrant.Client {
	t.Helper()
	if testQdrantAddr == "" {
		required, err := requireQdrant()
		if err != nil {
			t.Fatalf("%v", err)
		}
		if required {
			t.Fatal("no Qdrant available and ENGRAM_REQUIRE_QDRANT is set: failing instead of skipping")
		}
		t.Skip("no Qdrant available: set ENGRAM_QDRANT_TEST_ADDR or start Docker (testcontainers)")
	}
	host, portStr, err := net.SplitHostPort(testQdrantAddr)
	if err != nil {
		t.Fatalf("invalid Qdrant address %q: %v", testQdrantAddr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		t.Fatalf("invalid Qdrant port %q (from %q): %v", portStr, testQdrantAddr, err)
	}
	c, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: port})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	return c
}

// TestRequireQdrant mirrors internal/server/tools_test.go's TestRequireQdrant:
// verifies parsing semantics for the env var this package's fail-closed gate
// relies on.
func TestRequireQdrant(t *testing.T) {
	cases := []struct {
		name    string
		val     string
		want    bool
		wantErr bool
	}{
		{name: "unset_or_empty", val: "", want: false},
		{name: "truthy_true", val: "true", want: true},
		{name: "truthy_1", val: "1", want: true},
		{name: "falsey_false", val: "false", want: false},
		{name: "falsey_0", val: "0", want: false},
		{name: "malformed", val: "treu", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ENGRAM_REQUIRE_QDRANT", tc.val)
			got, err := requireQdrant()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("requireQdrant() with %q = (%v, nil), want a non-nil error (must not coerce to false)", tc.val, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("requireQdrant() with %q: unexpected error: %v", tc.val, err)
			}
			if got != tc.want {
				t.Errorf("requireQdrant() with %q = %v, want %v", tc.val, got, tc.want)
			}
		})
	}
}

// TestSharedQdrantAddressHonored proves this package took the CI shared-Qdrant
// fast path rather than booting its own testcontainer, whenever
// ENGRAM_QDRANT_TEST_ADDR is set. Address equality alone is not enough — a
// package could boot a container and coincidentally resolve the same address —
// so the load-bearing assertion is testQdrantContainerBooted == false
// (CONTEXT.md D-20). Skips (does not fail) when the env var is unset: a
// developer running locally without it is not the case this test is about.
func TestSharedQdrantAddressHonored(t *testing.T) {
	addr := os.Getenv("ENGRAM_QDRANT_TEST_ADDR")
	if addr == "" {
		t.Skip("ENGRAM_QDRANT_TEST_ADDR not set: this test only asserts the shared-instance path")
	}
	if testQdrantAddr != addr {
		t.Errorf("testQdrantAddr = %q, want %q (shared CI Qdrant address not honored)", testQdrantAddr, addr)
	}
	if testQdrantContainerBooted {
		t.Error("testQdrantContainerBooted = true, want false: ENGRAM_QDRANT_TEST_ADDR was set but this package booted its own testcontainer anyway")
	}
}

// TestDialTestClientSkipsWhenNotRequired proves dialTestClient preserves its
// original skip-not-fail behavior with no Qdrant available and
// ENGRAM_REQUIRE_QDRANT unset, so local development without Docker still
// works. Runs against a saved/restored testQdrantAddr so it does not depend on
// whether TestMain actually provisioned a live Qdrant.
func TestDialTestClientSkipsWhenNotRequired(t *testing.T) {
	savedAddr := testQdrantAddr
	t.Cleanup(func() { testQdrantAddr = savedAddr })
	testQdrantAddr = ""
	t.Setenv("ENGRAM_REQUIRE_QDRANT", "")

	passed := t.Run("inner", func(t *testing.T) {
		dialTestClient(t)
		t.Fatal("dialTestClient(t) did not skip; reached past the skip call")
	})
	if !passed {
		t.Fatal("dialTestClient(t) failed instead of skipping with ENGRAM_REQUIRE_QDRANT unset; local dev ergonomics regressed")
	}
}

// TestDialTestClientFailsWhenRequiredAndUnavailable proves dialTestClient FAILS
// (not skips) when ENGRAM_REQUIRE_QDRANT is set and no Qdrant is available —
// the exact fail-closed gate this package previously lacked (internal/server
// already had it via failOrSkipNoQdrant). A subtest that is meant to fail
// cannot be nested directly (a failing subtest marks its whole parent test
// binary run as FAIL regardless of how the outer test reads t.Run's returned
// bool), so this re-execs the test binary as a subprocess and asserts on its
// exit code and output — the standard Go idiom for testing an intentional
// test failure.
func TestDialTestClientFailsWhenRequiredAndUnavailable(t *testing.T) {
	if os.Getenv("ENGRAM_STORE_TEST_DIAL_FAIL_HELPER") == "1" {
		testQdrantAddr = ""
		dialTestClient(t)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestDialTestClientFailsWhenRequiredAndUnavailable", "-test.v")
	cmd.Env = append(os.Environ(),
		"ENGRAM_STORE_TEST_DIAL_FAIL_HELPER=1",
		"ENGRAM_REQUIRE_QDRANT=1",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("subprocess unexpectedly succeeded (want failure: no Qdrant available and ENGRAM_REQUIRE_QDRANT=1); output:\n%s", out)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("subprocess failed for an unexpected reason (want a plain non-zero exit from t.Fatal): %v; output:\n%s", err, out)
	}
	if !strings.Contains(string(out), "no Qdrant available and ENGRAM_REQUIRE_QDRANT is set: failing instead of skipping") {
		t.Fatalf("subprocess exited non-zero but not via the expected fail-closed message; output:\n%s", out)
	}
	if strings.Contains(string(out), "--- SKIP:") {
		t.Fatalf("subprocess SKIPPED instead of FAILING with ENGRAM_REQUIRE_QDRANT=1; output:\n%s", out)
	}
}

func testStore(t *testing.T) *Store {
	t.Helper()
	s := newTestStore(t, dialTestClient(t), testCollection("mem_eval_test"))
	if err := s.EnsureCollection(context.Background(), 3); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	return s
}

// cleanupErr surfaces a deferred-cleanup failure so leftover records can't
// silently contaminate later tests in the run. ErrNotFound is tolerated: the
// record is already gone, which is exactly what cleanup wanted.
func cleanupErr(t *testing.T, what string, err error) {
	t.Helper()
	if err != nil && !errors.Is(err, ErrNotFound) {
		t.Errorf("cleanup %s: %v", what, err)
	}
}

func TestEnsureIndexesCreatesShortIDIndex(t *testing.T) {
	st := testStore(t) // ensureIndexes ran during construction
	info, err := st.client.GetCollectionInfo(context.Background(), st.collection)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := info.GetPayloadSchema()["short_id"]; !ok {
		t.Fatalf("short_id payload index not created; schema keys: %v", info.GetPayloadSchema())
	}
	// Idempotence: a second ensureIndexes is AlreadyExists-tolerant.
	if err := st.ensureIndexes(context.Background(), st.collection); err != nil {
		t.Fatalf("second ensureIndexes: %v", err)
	}
}

func TestUpsertGetDeleteRoundtrip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	m := Memory{
		ID:        "11111111-1111-1111-1111-111111111111",
		Content:   "uses jj for VCS",
		Scope:     "eval-test:project:selfhosted-cluster",
		Repo:      "selfhosted-cluster",
		Source:    "user-said",
		Category:  "preference",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := s.Get(ctx, m.ID)
	if err != nil || got.Content != m.Content {
		t.Fatalf("get: %v / %+v", err, got)
	}
	if got.Scope != m.Scope {
		t.Errorf("scope mismatch: got %q want %q", got.Scope, m.Scope)
	}
	if err := s.Delete(ctx, m.ID, Anonymous()); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, m.ID); err == nil {
		t.Fatalf("expected not-found after delete")
	}
}

// TestListAllLimitEmptyScope pins the offset-mode fetch==0 short-circuit
// (engram-3jo0.4): List with Limit:0 ("all") over a scope with no matching
// records must NOT hand Qdrant's Scroll a Limit=0 (which Qdrant rejects with
// "must be 1 or larger") — it returns an empty page cleanly. Previously covered
// only indirectly via the server package's list_rules test; this pins it in the
// store package where the fix lives.
func TestListAllLimitEmptyScope(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "eval-test:project:list-all-empty-scope"

	items, total, next, err := s.List(ctx, scope, Anonymous(), ListOptions{Limit: 0})
	if err != nil {
		t.Fatalf("List(all) over empty scope: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items over empty scope, want 0", len(items))
	}
	if total != 0 {
		t.Errorf("got total=%d over empty scope, want 0", total)
	}
	if next != "" {
		t.Errorf("got nextCursor=%q over empty scope, want empty", next)
	}
}

func TestDiscoveryRoundtrip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	m := Memory{
		ID:       "44444444-4444-4444-4444-444444444444",
		Content:  "retry logic lives in client.go, keyed off ctx deadline",
		Scope:    "discovery:repo:github.com/seanb4t/engram",
		Repo:     "github.com/seanb4t/engram",
		Source:   "agent-inferred",
		Category: "discovery",
		Kind:     "fact",
		Summary:  "retry = ctx deadline",
		Citations: []Citation{
			{Kind: "file", Ref: "internal/client.go", Locator: "200-240", Pin: "sha256:abc", Excerpt: "for ... { select { <-ctx.Done() } }"},
			{Kind: "repo", Ref: "github.com/qdrant/go-client", Pin: "@v1.18.2"},
		},
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := s.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Kind != "fact" || got.Summary != "retry = ctx deadline" {
		t.Errorf("kind/summary mismatch: %+v", got)
	}
	if len(got.Citations) != 2 {
		t.Fatalf("citations: got %d want 2: %+v", len(got.Citations), got.Citations)
	}
	c0 := got.Citations[0]
	if c0.Kind != "file" || c0.Ref != "internal/client.go" || c0.Locator != "200-240" || c0.Pin != "sha256:abc" || c0.Excerpt == "" {
		t.Errorf("citation[0] mismatch: %+v", c0)
	}
	if got.Citations[1].Ref != "github.com/qdrant/go-client" || got.Citations[1].Pin != "@v1.18.2" {
		t.Errorf("citation[1] mismatch: %+v", got.Citations[1])
	}
	cleanupErr(t, "Delete "+m.ID, s.Delete(ctx, m.ID, Anonymous()))
}

// TestPayloadCitations pins D-01: payload()'s citations write gate is
// independent of the discovery-only kind gate. A curated (non-discovery)
// record carrying citations round-trips them through a real Upsert/Get, a
// curated record with no citations produces a stored payload with no
// "citations" key at all (not an empty list), and "kind" stays
// discovery-exclusive regardless of whether citations are present.
func TestPayloadCitations(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// A `decision` record with citations round-trips them, in order, through
	// a real Upsert/Get — the store-level half of GAP 1.
	withCites := Memory{
		ID:       "99999999-9999-9999-9999-999999999991",
		Content:  "use jose for JWT, not golang-jwt",
		Scope:    "eval-test:project:citations",
		Source:   "agent-inferred",
		Category: "decision",
		Citations: []Citation{
			{Kind: "file", Ref: "internal/auth/verifier.go", Locator: "10-40", Pin: "sha256:abc", Excerpt: "jose.NewVerifier(...)"},
			{Kind: "url", Ref: "https://pkg.go.dev/github.com/go-jose/go-jose/v4"},
		},
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := s.Upsert(ctx, withCites, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert (with citations): %v", err)
	}
	defer func() { cleanupErr(t, "Delete "+withCites.ID, s.Delete(ctx, withCites.ID, Anonymous())) }()
	got, err := s.Get(ctx, withCites.ID)
	if err != nil {
		t.Fatalf("get (with citations): %v", err)
	}
	if len(got.Citations) != 2 {
		t.Fatalf("citations: got %d want 2: %+v", len(got.Citations), got.Citations)
	}
	if got.Citations[0] != withCites.Citations[0] {
		t.Errorf("citation[0] mismatch: got %+v want %+v", got.Citations[0], withCites.Citations[0])
	}
	if got.Citations[1] != withCites.Citations[1] {
		t.Errorf("citation[1] mismatch: got %+v want %+v", got.Citations[1], withCites.Citations[1])
	}
	// A `decision` record — even one carrying citations — never gets a "kind"
	// payload key; that stays discovery-exclusive.
	if _, ok := payload(withCites)["kind"]; ok {
		t.Error("decision record with citations must not write a kind payload key")
	}

	// A `decision` record with NO citations must produce a payload with no
	// "citations" key at all (not an empty list) — byte-identical to today.
	noCites := Memory{ID: "99999999-9999-9999-9999-999999999992", Content: "c", Category: "decision"}
	if _, ok := payload(noCites)["citations"]; ok {
		t.Error("citation-free decision record must not write a citations payload key")
	}
	if _, ok := payload(noCites)["kind"]; ok {
		t.Error("decision record must not write a kind payload key")
	}

	// A discovery record still writes its kind payload key, regardless of
	// citations — the discovery-only gate is unchanged.
	disco := Memory{ID: "99999999-9999-9999-9999-999999999993", Content: "c", Category: "discovery", Kind: "fact",
		Citations: []Citation{{Kind: "file", Ref: "f.go"}}}
	p := payload(disco)
	if _, ok := p["kind"]; !ok {
		t.Error("discovery record must write a kind payload key")
	}
	if _, ok := p["citations"]; !ok {
		t.Error("discovery record with citations must write a citations payload key")
	}
}

func TestSearchDiscoveryFilters(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scopeA := "discovery:repo:A"
	scopeB := "discovery:repo:B"
	curated := "repo:A"
	defer func() {
		cleanupErr(t, "DeleteAll "+scopeA, s.DeleteAll(ctx, scopeA, Anonymous()))
		cleanupErr(t, "DeleteAll "+scopeB, s.DeleteAll(ctx, scopeB, Anonymous()))
		cleanupErr(t, "DeleteAll "+curated, s.DeleteAll(ctx, curated, Anonymous()))
	}()

	mk := func(id, scope, cat, kind string, vec []float32) {
		m := Memory{ID: id, Content: "x", Scope: scope, Category: cat, Kind: kind,
			Source: "agent-inferred", CreatedAt: time.Now().UTC()}
		if cat == "discovery" {
			m.Citations = []Citation{{Kind: "file", Ref: "f.go"}}
		}
		if err := s.Upsert(ctx, m, vec); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	mk("55555555-5555-5555-5555-555555555555", scopeA, "discovery", "map", []float32{0.9, 0.1, 0.0})
	mk("66666666-6666-6666-6666-666666666666", scopeA, "discovery", "fact", []float32{0.8, 0.2, 0.0})
	mk("77777777-7777-7777-7777-777777777777", scopeB, "discovery", "map", []float32{0.7, 0.3, 0.0})
	mk("88888888-8888-8888-8888-888888888888", curated, "preference", "", []float32{0.9, 0.1, 0.0})

	q := []float32{0.9, 0.1, 0.0}

	// scope-constrained: only scopeA discoveries, not curated, not scopeB.
	hits, err := s.SearchDiscovery(ctx, scopeA, "", Anonymous(), q, 10)
	if err != nil {
		t.Fatalf("search scopeA: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("scopeA: got %d want 2", len(hits))
	}
	for _, h := range hits {
		if h.Category != "discovery" || h.Scope != scopeA {
			t.Errorf("leaked non-discovery/other-scope: %+v", h)
		}
	}

	// kind filter.
	maps, err := s.SearchDiscovery(ctx, scopeA, "map", Anonymous(), q, 10)
	if err != nil {
		t.Fatalf("search kind=map: %v", err)
	}
	if len(maps) != 1 || maps[0].Kind != "map" {
		t.Fatalf("kind=map: got %d %+v", len(maps), maps)
	}

	// cross-spine: empty scope spans scopeA + scopeB discoveries (3), still no curated.
	all, err := s.SearchDiscovery(ctx, "", "", Anonymous(), q, 10)
	if err != nil {
		t.Fatalf("search cross-spine: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("cross-spine: got %d want 3", len(all))
	}
}

func TestSearchDiscoveryOwnerIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scopeA := "discovery:repo:isoA"
	scopeB := "discovery:repo:isoB"
	defer func() {
		cleanupErr(t, "DeleteAllRaw "+scopeA, s.DeleteAllRaw(ctx, scopeA))
		cleanupErr(t, "DeleteAllRaw "+scopeB, s.DeleteAllRaw(ctx, scopeB))
	}()

	mk := func(id, scope, owner, vis string) {
		m := Memory{ID: id, Content: "d", Scope: scope, Category: "discovery", Kind: "fact",
			Owner: owner, Visibility: vis, CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	mk("cccccccc-0000-0000-0000-000000000001", scopeA, "sub-A", "")       // A private, scopeA
	mk("cccccccc-0000-0000-0000-000000000002", scopeA, "sub-B", "")       // B private, scopeA
	mk("cccccccc-0000-0000-0000-000000000003", scopeB, "sub-A", "")       // A private, scopeB
	mk("cccccccc-0000-0000-0000-000000000004", scopeB, "sub-B", "shared") // B shared, scopeB
	q := []float32{0.1, 0.2, 0.3}

	// Scoped: A in scopeA sees only A's record (1), not B's private.
	hits, err := s.SearchDiscovery(ctx, scopeA, "", Authenticated("sub-A"), q, 10)
	if err != nil {
		t.Fatalf("scoped: %v", err)
	}
	if len(hits) != 1 || hits[0].Owner != "sub-A" {
		t.Fatalf("scoped: got %d %+v want 1 A-owned", len(hits), hits)
	}
	// cross_spine (scope=""): A sees A across both scopes (2) + B's shared (1) = 3, never B's private.
	all, err := s.SearchDiscovery(ctx, "", "", Authenticated("sub-A"), q, 10)
	if err != nil {
		t.Fatalf("cross_spine: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("cross_spine: got %d want 3", len(all))
	}
	for _, h := range all {
		if h.Owner == "sub-B" && h.Visibility != "shared" {
			t.Errorf("cross_spine leaked B's private discovery: %s", h.ID)
		}
	}
}

// TestSearchDiscoverySupersededHidden (WR-01) pins that SearchDiscovery
// applies the same superseded_by soft-hide gate Search/List already carry —
// a superseded discovery must not remain visible via search_discovery.
func TestSearchDiscoverySupersededHidden(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "discovery:repo:superseded-hidden"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	supersededID := "cd000000-0000-0000-0000-000000000001"
	liveID := "cd000000-0000-0000-0000-000000000002"
	newID := "cd000000-0000-0000-0000-000000000003"
	vec := []float32{0.1, 0.2, 0.3}

	superseded := Memory{
		ID: supersededID, Content: "old discovery", Scope: scope, Category: "discovery",
		Kind: "fact", Owner: "sub-A", CreatedAt: time.Now().UTC(), SupersededBy: &newID,
	}
	if err := s.Upsert(ctx, superseded, vec); err != nil {
		t.Fatalf("upsert superseded: %v", err)
	}
	live := Memory{
		ID: liveID, Content: "live discovery", Scope: scope, Category: "discovery",
		Kind: "fact", Owner: "sub-A", CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, live, vec); err != nil {
		t.Fatalf("upsert live: %v", err)
	}

	hits, err := s.SearchDiscovery(ctx, scope, "", Authenticated("sub-A"), vec, 10)
	if err != nil {
		t.Fatalf("SearchDiscovery: %v", err)
	}
	if got := recordIDs(hits); slices.Contains(got, supersededID) {
		t.Errorf("SearchDiscovery: superseded record %s present, want excluded: %v", supersededID, got)
	}
	if got := recordIDs(hits); !slices.Contains(got, liveID) {
		t.Errorf("SearchDiscovery: live record %s absent, want present: %v", liveID, got)
	}
}

func TestFromPayloadSkipsNonStructCitation(t *testing.T) {
	// A malformed citations list (a stray non-struct item) must not yield an
	// empty Citation{}; the struct items still read back. Pure unit test — no Qdrant.
	p := qdrant.NewValueMap(map[string]any{
		"citations": []any{
			"not-a-struct",
			map[string]any{"kind": "file", "ref": "f.go", "locator": "1-2"},
		},
	})
	m := fromPayload("id-1", p)
	if len(m.Citations) != 1 {
		t.Fatalf("expected 1 citation (non-struct skipped), got %d: %+v", len(m.Citations), m.Citations)
	}
	if m.Citations[0].Ref != "f.go" || m.Citations[0].Kind != "file" {
		t.Errorf("citation mismatch: %+v", m.Citations[0])
	}
}

func TestSearchAndDeleteAll(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "eval-test:project:search-test"
	// A foreign-owned record survives the anon-scoped DeleteAll below, so clean
	// the whole scope raw regardless of owner.
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	m1 := Memory{
		ID: "22222222-2222-2222-2222-222222222222", Content: "prefers Go over Python",
		Scope: scope, Source: "user-said", Category: "preference", CreatedAt: time.Now().UTC(),
	}
	m2 := Memory{
		ID: "33333333-3333-3333-3333-333333333333", Content: "uses conventional commits",
		Scope: scope, Source: "agent-inferred", Category: "convention", CreatedAt: time.Now().UTC(),
	}
	// A record owned by a different sub, sharing m1's vector so it WOULD rank in
	// the query below if the owner filter regressed to matching everything. Both
	// the anon search (must exclude it) and the anon DeleteAll (must not delete
	// it) then exercise the owner-match branch — without this record an absent
	// owner filter would pass the test silently.
	mForeign := Memory{
		ID: "44444444-4444-4444-4444-444444444444", Content: "foreign-owned secret",
		Scope: scope, Owner: "sub-foreign", Source: "user-said", Category: "preference",
		CreatedAt: time.Now().UTC(),
	}

	if err := s.Upsert(ctx, m1, []float32{0.9, 0.1, 0.0}); err != nil {
		t.Fatalf("upsert m1: %v", err)
	}
	if err := s.Upsert(ctx, m2, []float32{0.0, 0.1, 0.9}); err != nil {
		t.Fatalf("upsert m2: %v", err)
	}
	if err := s.Upsert(ctx, mForeign, []float32{0.9, 0.1, 0.0}); err != nil {
		t.Fatalf("upsert mForeign: %v", err)
	}

	hits, err := s.Search(ctx, scope, Anonymous(), []float32{0.9, 0.1, 0.0}, 5, SearchOptions{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least 1 search hit")
	}
	// The owner-match branch must fire: an anonymous caller sees only the
	// anonymous bucket (owner==""), never the foreign-owned record.
	for _, h := range hits {
		if h.Owner != "" {
			t.Errorf("anonymous search returned a non-anon record: id=%s owner=%q", h.ID, h.Owner)
		}
	}

	if err := s.DeleteAll(ctx, scope, Anonymous()); err != nil {
		t.Fatalf("delete_all: %v", err)
	}

	hits2, err := s.Search(ctx, scope, Anonymous(), []float32{0.9, 0.1, 0.0}, 5, SearchOptions{})
	if err != nil {
		t.Fatalf("search after delete_all: %v", err)
	}
	if len(hits2) != 0 {
		t.Fatalf("expected 0 hits after delete_all, got %d", len(hits2))
	}
	// DeleteAll is owner-scoped: the foreign-owned record must survive an
	// anonymous DeleteAll.
	survivors, err := s.Search(ctx, scope, Authenticated("sub-foreign"), []float32{0.9, 0.1, 0.0}, 5, SearchOptions{})
	if err != nil {
		t.Fatalf("search as foreign owner after delete_all: %v", err)
	}
	if len(survivors) != 1 {
		t.Fatalf("foreign-owned record must survive anon delete_all: got %d want 1", len(survivors))
	}
}

// TestSearchAndListTagsFilter pins the tag filter on both recall paths: an
// optional tags filter narrows results to records carrying ALL requested tags
// (AND), an empty filter is a passthrough, and the filter composes with — never
// bypasses — the owner/visibility authz gate.
func TestSearchAndListTagsFilter(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:tags-filter"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	mk := func(id, owner string, tags ...string) {
		m := Memory{ID: id, Content: "x", Scope: scope, Owner: owner, Tags: tags,
			CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	// Anonymous-bucket (owner=="") records the anonymous caller can read.
	mk("cccccccc-0000-0000-0000-000000000001", "", "go", "perf") // both
	mk("cccccccc-0000-0000-0000-000000000002", "", "go")         // go only
	mk("cccccccc-0000-0000-0000-000000000003", "", "python")     // neither
	// Owned by another actor and carrying "go": the tag filter must NOT surface it
	// to the anonymous caller (filter narrows, authz still gates).
	mk("cccccccc-0000-0000-0000-000000000004", "sub-other", "go")

	q := []float32{0.1, 0.2, 0.3}
	ids := func(ms []Memory) []string {
		out := make([]string, len(ms))
		for i, m := range ms {
			out[i] = m.ID
		}
		slices.Sort(out)
		return out
	}

	cases := []struct {
		name string
		tags []string
		want []string
	}{
		{"no filter is passthrough", nil, []string{
			"cccccccc-0000-0000-0000-000000000001",
			"cccccccc-0000-0000-0000-000000000002",
			"cccccccc-0000-0000-0000-000000000003",
		}},
		{"single tag", []string{"go"}, []string{
			"cccccccc-0000-0000-0000-000000000001",
			"cccccccc-0000-0000-0000-000000000002",
		}},
		{"AND of two tags", []string{"go", "perf"}, []string{
			"cccccccc-0000-0000-0000-000000000001",
		}},
		{"non-matching tag", []string{"rust"}, []string{}},
		{"empty-string tag is passthrough", []string{""}, []string{
			"cccccccc-0000-0000-0000-000000000001",
			"cccccccc-0000-0000-0000-000000000002",
			"cccccccc-0000-0000-0000-000000000003",
		}},
		{"empty-string element is dropped", []string{"go", ""}, []string{
			"cccccccc-0000-0000-0000-000000000001",
			"cccccccc-0000-0000-0000-000000000002",
		}},
	}
	for _, tc := range cases {
		hits, err := s.Search(ctx, scope, Anonymous(), q, 10, SearchOptions{Tags: tc.tags})
		if err != nil {
			t.Fatalf("Search %s: %v", tc.name, err)
		}
		if got := ids(hits); !slices.Equal(got, tc.want) {
			t.Errorf("Search %s: got %v want %v", tc.name, got, tc.want)
		}
		lst, _, _, err := s.List(ctx, scope, Anonymous(), ListOptions{Limit: 10, Tags: tc.tags})
		if err != nil {
			t.Fatalf("List %s: %v", tc.name, err)
		}
		if got := ids(lst); !slices.Equal(got, tc.want) {
			t.Errorf("List %s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

// TestTagsFilterComposesWithWindow confirms the tag filter and the active-window
// recall gate compose: an expired record is dropped even when its tags match,
// because both are appended to the same Must envelope. The tag filter narrows;
// it never resurrects a windowed-out record.
func TestTagsFilterComposesWithWindow(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:tags-window"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	fixed := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixed } // white-box override of the recall clock

	active := fixed.Add(time.Hour)   // not_after in the future → still recalled
	expired := fixed.Add(-time.Hour) // not_after in the past → windowed out
	mk := func(id string, notAfter time.Time) {
		m := Memory{ID: id, Content: "x", Scope: scope, Owner: "", Tags: []string{"go"},
			CreatedAt: fixed, NotAfter: &notAfter}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	activeID := "cdcdcdcd-0000-0000-0000-000000000001"
	expiredID := "cdcdcdcd-0000-0000-0000-000000000002"
	mk(activeID, active)
	mk(expiredID, expired)

	// Tag "go" matches both records, but the window gate must drop the expired one.
	hits, err := s.Search(ctx, scope, Anonymous(), []float32{0.1, 0.2, 0.3}, 10, SearchOptions{Tags: []string{"go"}})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 || hits[0].ID != activeID {
		t.Errorf("Search tag+window: got %d (%v) want just %s", len(hits), hits, activeID)
	}
	lst, _, _, err := s.List(ctx, scope, Anonymous(), ListOptions{Limit: 10, Tags: []string{"go"}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(lst) != 1 || lst[0].ID != activeID {
		t.Errorf("List tag+window: got %d (%v) want just %s", len(lst), lst, activeID)
	}
}

func TestSearchListOwnerIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:search"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	mk := func(id, owner, vis string) {
		m := Memory{ID: id, Content: "x", Scope: scope, Owner: owner, Visibility: vis,
			CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	mk("bbbbbbbb-0000-0000-0000-000000000001", "sub-A", "")       // A private
	mk("bbbbbbbb-0000-0000-0000-000000000002", "sub-B", "")       // B private
	mk("bbbbbbbb-0000-0000-0000-000000000003", "sub-B", "shared") // B shared

	// A sees only A-private + B-shared (2), never B-private.
	hits, err := s.Search(ctx, scope, Authenticated("sub-A"), []float32{0.1, 0.2, 0.3}, 10, SearchOptions{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("Search: got %d want 2", len(hits))
	}
	for _, h := range hits {
		if h.Owner == "sub-B" && h.Visibility != "shared" {
			t.Errorf("leaked B's private record: %s", h.ID)
		}
	}
	// List honors the same filter.
	lst, _, _, err := s.List(ctx, scope, Authenticated("sub-A"), ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lst) != 2 {
		t.Errorf("List: got %d want 2", len(lst))
	}
}

func TestOwnerVisibilityRoundtrip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	m := Memory{
		ID:         "aaaaaaaa-0000-0000-0000-000000000001",
		Content:    "owned + shared record",
		Scope:      "iso-test:project:r",
		Owner:      "sub-abc",
		Visibility: "shared",
		CreatedAt:  time.Now().UTC().Truncate(time.Second),
	}
	defer func() {
		_, _ = s.client.Delete(ctx, &qdrant.DeletePoints{
			CollectionName: s.collection, Wait: qdrant.PtrOf(true),
			Points: qdrant.NewPointsSelector(qdrant.NewID(m.ID)),
		})
	}()
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := s.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Owner != "sub-abc" || got.Visibility != "shared" {
		t.Errorf("round-trip lost owner/visibility: owner=%q visibility=%q", got.Owner, got.Visibility)
	}
}

func TestGetReadableOwnerGate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:getr"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	priv := Memory{ID: "dddddddd-0000-0000-0000-000000000001", Content: "p", Scope: scope, Owner: "sub-B", CreatedAt: time.Now().UTC()}
	shar := Memory{ID: "dddddddd-0000-0000-0000-000000000002", Content: "s", Scope: scope, Owner: "sub-B", Visibility: "shared", CreatedAt: time.Now().UTC()}
	for _, m := range []Memory{priv, shar} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	// Owner reads own private record.
	if _, err := s.GetReadable(ctx, priv.ID, Authenticated("sub-B")); err != nil {
		t.Errorf("owner denied own record: %v", err)
	}
	// Non-owner reads a shared record.
	if _, err := s.GetReadable(ctx, shar.ID, Authenticated("sub-A")); err != nil {
		t.Errorf("shared record denied to other actor: %v", err)
	}
	// Non-owner denied a private record — and it looks like not-found.
	_, err := s.GetReadable(ctx, priv.ID, Authenticated("sub-A"))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-actor private read: want ErrNotFound, got %v", err)
	}
}

func TestDeleteOwnerGate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:del"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	// Even a SHARED record is not deletable by a non-owner.
	m := Memory{ID: "eeeeeeee-0000-0000-0000-000000000001", Content: "s", Scope: scope, Owner: "sub-B", Visibility: "shared", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := s.Delete(ctx, m.ID, Authenticated("sub-A")); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-owner delete: want ErrNotFound, got %v", err)
	}
	if _, err := s.Get(ctx, m.ID); err != nil {
		t.Errorf("record should survive non-owner delete: %v", err)
	}
	if err := s.Delete(ctx, m.ID, Authenticated("sub-B")); err != nil {
		t.Errorf("owner delete failed: %v", err)
	}
	if _, err := s.Get(ctx, m.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("owner delete did not remove record: %v", err)
	}
}

func TestUpdateOwnerGateAndSharedFlag(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:upd"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	m := Memory{ID: "ffffffff-0000-0000-0000-000000000001", Content: "v1", Scope: scope, Owner: "sub-B", Visibility: "shared", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	vec := []float32{0.4, 0.5, 0.6}
	// Non-owner cannot update even a shared record.
	if _, err := s.FetchForUpdate(ctx, m.ID, Authenticated("sub-A")); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-owner update: want ErrNotFound, got %v", err)
	}
	// Owner content-only update (shared == nil) PRESERVES visibility.
	cur, err := s.FetchForUpdate(ctx, m.ID, Authenticated("sub-B"))
	if err != nil {
		t.Fatalf("FetchForUpdate owner: %v", err)
	}
	if err := s.Update(ctx, cur, "v2", nil, nil, nil, vec); err != nil {
		t.Fatalf("owner update: %v", err)
	}
	got, err := s.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("get after content-only update: %v", err)
	}
	if got.Content != "v2" || got.Visibility != "shared" {
		t.Errorf("content-only update lost sharing: content=%q visibility=%q", got.Content, got.Visibility)
	}
	// Explicit unshare.
	no := false
	cur, err = s.FetchForUpdate(ctx, m.ID, Authenticated("sub-B"))
	if err != nil {
		t.Fatalf("FetchForUpdate before unshare: %v", err)
	}
	if err := s.Update(ctx, cur, "v3", &no, nil, nil, vec); err != nil {
		t.Fatalf("unshare update: %v", err)
	}
	got, err = s.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("get after unshare: %v", err)
	}
	if got.Visibility != "" {
		t.Errorf("unshare failed: visibility=%q", got.Visibility)
	}
}

// TestUpdateTags pins the store-level tag presence-signal at the layer that
// owns it (mirroring TestUpdateOwnerGateAndSharedFlag's coverage of shared):
// nil preserves, non-nil replaces, an empty slice clears — each surviving the
// full Qdrant payload round-trip.
func TestUpdateTags(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:upd-tags"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	id := "ffffffff-0000-0000-0000-0000000000a1"
	m := Memory{ID: id, Content: "v1", Scope: scope, Owner: "sub-T", Tags: []string{"old"}, CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	vec := []float32{0.4, 0.5, 0.6}

	// nil tags preserves the existing set.
	cur, err := s.FetchForUpdate(ctx, id, Authenticated("sub-T"))
	if err != nil {
		t.Fatalf("fetch (preserve): %v", err)
	}
	if err := s.Update(ctx, cur, "v2", nil, nil, nil, vec); err != nil {
		t.Fatalf("update nil tags: %v", err)
	}
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("get (preserve): %v", err)
	}
	if !slices.Equal(got.Tags, []string{"old"}) {
		t.Errorf("nil tags should preserve: got %v", got.Tags)
	}

	// non-nil replaces the full set.
	cur, err = s.FetchForUpdate(ctx, id, Authenticated("sub-T"))
	if err != nil {
		t.Fatalf("fetch (replace): %v", err)
	}
	repl := []string{"x", "y"}
	if err := s.Update(ctx, cur, "v3", nil, &repl, nil, vec); err != nil {
		t.Fatalf("update replace: %v", err)
	}
	got, err = s.Get(ctx, id)
	if err != nil {
		t.Fatalf("get (replace): %v", err)
	}
	if !slices.Equal(got.Tags, repl) {
		t.Errorf("non-nil tags should replace: got %v want %v", got.Tags, repl)
	}

	// empty slice clears.
	cur, err = s.FetchForUpdate(ctx, id, Authenticated("sub-T"))
	if err != nil {
		t.Fatalf("fetch (clear): %v", err)
	}
	empty := []string{}
	if err := s.Update(ctx, cur, "v4", nil, &empty, nil, vec); err != nil {
		t.Fatalf("update clear: %v", err)
	}
	got, err = s.Get(ctx, id)
	if err != nil {
		t.Fatalf("get (clear): %v", err)
	}
	if len(got.Tags) != 0 {
		t.Errorf("empty slice should clear tags: got %v", got.Tags)
	}
}

func TestSetVisibilityOwnerGate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:vis"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	m := Memory{ID: "a1a1a1a1-0000-0000-0000-000000000001", Content: "v", Scope: scope, Owner: "sub-B", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Non-owner denied.
	if err := s.SetVisibility(ctx, m.ID, Authenticated("sub-A"), true); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-owner set_visibility: want ErrNotFound, got %v", err)
	}
	// Owner shares, then unshares; vector is preserved (SetPayload, not Upsert).
	if err := s.SetVisibility(ctx, m.ID, Authenticated("sub-B"), true); err != nil {
		t.Fatalf("share: %v", err)
	}
	got, err := s.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("get after share: %v", err)
	}
	if got.Visibility != "shared" {
		t.Errorf("share failed: %q", got.Visibility)
	}
	if err := s.SetVisibility(ctx, m.ID, Authenticated("sub-B"), false); err != nil {
		t.Fatalf("unshare: %v", err)
	}
	got, err = s.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("get after unshare: %v", err)
	}
	if got.Visibility != "" {
		t.Errorf("unshare failed: %q", got.Visibility)
	}
}

// TestSetVisibilityTOCTOU verifies the TOCTOU behaviour of SetVisibility: a
// record deleted between the ownership gate (getWritable) and the SetPayload
// call must not cause SetVisibility to return nil.
//
// Qdrant v1.18.2 with a point-ID selector returns a NotFound gRPC error from
// SetPayload when the target ID does not exist — so the error propagates
// through SetVisibility without a separate re-fetch. Parts 1 and 2 are the
// load-bearing TOCTOU assertions: they confirm at the raw Qdrant level that
// SetPayload errors on a missing point-ID (the fail-closed contract we rely
// on), including when the record vanishes after the gate has passed. Part 3
// covers the simpler pre-entry case — the record is already gone when
// SetVisibility is called, so the getWritable gate rejects it. A version guard
// at the top of this function enforces the image-version coupling: bumping
// qdrantImageTag without updating qdrantTOCTOUVerifiedVersion will fail loudly
// here, requiring conscious re-verification of Parts 1–2 before proceeding.
func TestSetVisibilityTOCTOU(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Version guard: the SetPayload-on-deleted-point fail-closed contract asserted
	// in Parts 1-2 was verified against Qdrant qdrantTOCTOUVerifiedVersion. When the
	// suite boots the pinned testcontainer (no ENGRAM_QDRANT_TEST_ADDR override),
	// enforce that the running server matches — so bumping qdrantImageTag without
	// re-verifying Parts 1-2 and updating qdrantTOCTOUVerifiedVersion fails loudly
	// here. An operator-supplied external instance owns its own version, so the
	// check is skipped on that path to avoid a spurious failure.
	if os.Getenv("ENGRAM_QDRANT_TEST_ADDR") == "" {
		hc, err := s.client.HealthCheck(ctx)
		if err != nil {
			t.Fatalf("qdrant health check: %v", err)
		}
		if v := hc.GetVersion(); v != qdrantTOCTOUVerifiedVersion {
			t.Fatalf("Qdrant version %q != verified %q: re-verify SetPayload point-ID NotFound semantics (Parts 1-2), then update qdrantTOCTOUVerifiedVersion and qdrantImageTag together", v, qdrantTOCTOUVerifiedVersion)
		}
	}

	scope := "iso-test:project:toctou"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	// Part 1: verify Qdrant's SetPayload (point-ID selector) errors on a
	// missing ID — this is the contract that makes SetVisibility fail-closed.
	missingID := "f0f0f0f0-0000-0000-0000-000000000001"
	_, rawErr := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"visibility": "shared"}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(missingID)}),
	})
	if rawErr == nil {
		t.Fatal("qdrant SetPayload on missing point-ID returned nil — the fail-closed contract for SetVisibility is broken; review Qdrant behaviour for this version and update SetVisibility accordingly")
	}

	// Part 2: simulate the TOCTOU window at the raw level.
	// Insert → verify gate passes → delete (concurrent race) → SetPayload.
	// SetPayload must error because the ID no longer exists.
	id := "f0f0f0f0-0000-0000-0000-000000000002"
	m := Memory{
		ID: id, Content: "toctou-target", Scope: scope,
		Owner: "sub-owner", CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := s.getWritable(ctx, id, Authenticated("sub-owner"), authz.ActionShare); err != nil {
		t.Fatalf("getWritable pre-delete: %v", err)
	}
	// Concurrent delete: simulates what happens in the TOCTOU window.
	if _, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelector(qdrant.NewID(id)),
	}); err != nil {
		t.Fatalf("concurrent delete: %v", err)
	}
	_, setPayloadErr := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"visibility": "shared"}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	})
	if setPayloadErr == nil {
		t.Error("TOCTOU: SetPayload on deleted point-ID returned nil — the fail-closed contract for SetVisibility is broken; review Qdrant behaviour for this version and update SetVisibility accordingly")
	}

	// Part 3: end-to-end via the SetVisibility public API.
	// Insert → delete → SetVisibility must error. The record is absent at gate
	// entry, so this surfaces ErrNotFound from getWritable — the pre-entry
	// deletion case, not the TOCTOU window (which Part 2 covers).
	id2 := "f0f0f0f0-0000-0000-0000-000000000003"
	m2 := Memory{
		ID: id2, Content: "sv-target", Scope: scope,
		Owner: "sub-owner", CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, m2, []float32{0.4, 0.5, 0.6}); err != nil {
		t.Fatalf("upsert m2: %v", err)
	}
	if err := s.Delete(ctx, id2, Authenticated("sub-owner")); err != nil {
		t.Fatalf("delete m2: %v", err)
	}
	// SetVisibility on a missing record must not return nil.
	if err := s.SetVisibility(ctx, id2, Authenticated("sub-owner"), true); err == nil {
		t.Error("SetVisibility on deleted record returned nil — expected an error")
	}
}

func TestOwnedOrAbsent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "discovery:repo:owned"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	id := "b2b2b2b2-0000-0000-0000-000000000001"
	// Absent id → ok (caller will create).
	if err := s.OwnedOrAbsent(ctx, id, Authenticated("sub-A")); err != nil {
		t.Errorf("absent id should be ok: %v", err)
	}
	m := Memory{ID: id, Content: "d", Scope: scope, Category: "discovery", Kind: "fact", Owner: "sub-A", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Owner re-supplies id → ok (replace).
	if err := s.OwnedOrAbsent(ctx, id, Authenticated("sub-A")); err != nil {
		t.Errorf("owner replace should be ok: %v", err)
	}
	// Other actor supplies id → ErrNotFound (refuse overwrite).
	if err := s.OwnedOrAbsent(ctx, id, Authenticated("sub-B")); !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-owner overwrite: want ErrNotFound, got %v", err)
	}
}

// TestFetchForUpdate mirrors TestOwnedOrAbsent for the FetchForUpdate gate:
// owner → record returned, non-owner → ErrNotFound, absent → ErrNotFound (record
// must exist to update), anonymous bucket (sub=="" on ownerless record) → record
// returned.
func TestFetchForUpdate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:fetch-for-update"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	id := "b2b2b2b2-0000-0000-0000-000000000002"

	// Absent record → ErrNotFound (nothing to update).
	if _, err := s.FetchForUpdate(ctx, id, Authenticated("sub-A")); !errors.Is(err, ErrNotFound) {
		t.Errorf("absent record: want ErrNotFound, got %v", err)
	}

	m := Memory{ID: id, Content: "d", Scope: scope, Category: "convention", Owner: "sub-A", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Owner → record returned (may proceed to embed + Update).
	got, err := s.FetchForUpdate(ctx, id, Authenticated("sub-A"))
	if err != nil {
		t.Errorf("owner: unexpected error: %v", err)
	}
	if got.ID != id {
		t.Errorf("owner: want ID %q, got %q", id, got.ID)
	}

	// Non-owner → ErrNotFound (early-exit before embed).
	if _, err := s.FetchForUpdate(ctx, id, Authenticated("sub-B")); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-owner: want ErrNotFound, got %v", err)
	}

	// Anonymous bucket: ownerless record (owner=="") with Anonymous() → record returned.
	ownerlessID := "b2b2b2b2-0000-0000-0000-000000000003"
	ownerless := Memory{ID: ownerlessID, Content: "x", Scope: scope, Category: "convention", Owner: "", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, ownerless, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert ownerless: %v", err)
	}
	if _, err := s.FetchForUpdate(ctx, ownerlessID, Anonymous()); err != nil {
		t.Errorf("anonymous bucket ownerless: unexpected error: %v", err)
	}
	// Stamped record (owner!="") with Anonymous() → ErrNotFound (fail-closed write isolation).
	if _, err := s.FetchForUpdate(ctx, id, Anonymous()); !errors.Is(err, ErrNotFound) {
		t.Errorf("anon on stamped record: want ErrNotFound, got %v", err)
	}
}

func TestDeleteAllOwnerScoped(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:delall"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	a := Memory{ID: "c3c3c3c3-0000-0000-0000-000000000001", Content: "a", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
	b := Memory{ID: "c3c3c3c3-0000-0000-0000-000000000002", Content: "b", Scope: scope, Owner: "sub-B", CreatedAt: time.Now().UTC()}
	for _, m := range []Memory{a, b} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	// A's teardown removes only A's record; B's survives.
	if err := s.DeleteAll(ctx, scope, Authenticated("sub-A")); err != nil {
		t.Fatalf("deleteAll: %v", err)
	}
	if _, err := s.Get(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("A's record should be gone: %v", err)
	}
	if _, err := s.Get(ctx, b.ID); err != nil {
		t.Errorf("B's record should survive A's teardown: %v", err)
	}
}

func TestMigrateSetOwner(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:migrate"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	// Refuses empty owner.
	if _, err := s.MigrateSetOwner(ctx, ""); err == nil {
		t.Error("empty owner: expected error")
	}

	// A record written WITHOUT the owner key (simulating a pre-isolation record):
	// build the payload, strip "owner", and upsert raw.
	id := "d4d4d4d4-0000-0000-0000-000000000001"
	p := payload(Memory{ID: id, Content: "legacy", Scope: scope, CreatedAt: time.Now().UTC()})
	delete(p, "owner")
	if _, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: []*qdrant.PointStruct{{
			Id: qdrant.NewID(id), Vectors: qdrant.NewVectors(0.1, 0.2, 0.3),
			Payload: qdrant.NewValueMap(p),
		}},
	}); err != nil {
		t.Fatalf("raw upsert: %v", err)
	}

	// An auth-disabled / anonymous-bucket record: the owner key is PRESENT but
	// empty (""). ownerlessFilter uses qdrant.NewIsEmpty("owner"), which matches a
	// MISSING key, not an empty-string value — so the backfill must leave this
	// record untouched. Without that distinction, enabling auth and running
	// migrate-set-owner would silently hijack every anonymous record into the
	// operator's sub.
	anonID := "d4d4d4d4-0000-0000-0000-000000000002"
	if err := s.Upsert(ctx, Memory{
		ID: anonID, Content: "anon-bucket", Scope: scope,
		Owner: "", CreatedAt: time.Now().UTC(),
	}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed anon-bucket record: %v", err)
	}

	n, err := s.MigrateSetOwner(ctx, "sub-OWNER")
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Exactly one record stamped: the missing-key legacy record. The explicit
	// owner=="" record is not matched by NewIsEmpty and must not be counted.
	if n != 1 {
		t.Fatalf("migrate stamped %d records, want exactly 1 (only the missing-key record; the explicit owner=='' bucket must be skipped)", n)
	}
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after migrate: %v", err)
	}
	if got.Owner != "sub-OWNER" {
		t.Errorf("owner not stamped: %q", got.Owner)
	}
	// The auth-disabled owner=="" record must be untouched by the backfill.
	anon, err := s.Get(ctx, anonID)
	if err != nil {
		t.Fatalf("Get anon-bucket after migrate: %v", err)
	}
	if anon.Owner != "" {
		t.Errorf("auth-disabled record hijacked: owner=%q, want \"\" (NewIsEmpty must not match empty-string owner)", anon.Owner)
	}
	// Idempotent: a second run stamps nothing (the record now has an owner).
	n2, err := s.MigrateSetOwner(ctx, "sub-OWNER")
	if err != nil {
		t.Fatalf("migrate rerun: %v", err)
	}
	if n2 != 0 {
		t.Errorf("idempotency: rerun stamped %d, want 0", n2)
	}
}

// TestMigrateSetOwnerHonorsCancel verifies the backfill propagates context
// cancellation to its Qdrant calls instead of running to completion — the
// property the CLI relies on for its --timeout / Ctrl-C bound (engram-027), so a
// hung Qdrant cannot block forever.
func TestMigrateSetOwnerHonorsCancel(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call: the first Count must observe it and bail.
	if _, err := s.MigrateSetOwner(ctx, "sub-x"); err == nil {
		t.Error("MigrateSetOwner with a cancelled context: expected error, got nil")
	}
}

// TestAnonBucketReadIsolation verifies Q1 (engram-99z.13): anonymous callers
// (sub=="") may only read the anonymous bucket (owner=="") — they cannot see
// another owner's shared record via Search, List, or GetReadable.
//
// Scope: this exercises the anonymous bucket (explicit owner=="", as written by
// auth-disabled deployments). It does NOT cover pre-isolation records (those
// with a MISSING owner key): NewMatch("owner","") does not match missing keys,
// so such records are intentionally invisible to every read until backfilled
// (see ownerlessFilter / MigrateSetOwner and the README "Upgrading" note). That
// invisibility is unchanged by this tightening.
func TestAnonBucketReadIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "anon-test:project:read-iso"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	// anonymous-bucket record (explicit owner==""): what an auth-disabled
	// deployment writes; readable by anonymous callers.
	ownerless := Memory{
		ID: "f1f1f1f1-0000-0000-0000-000000000001", Content: "anon-ownerless",
		Scope: scope, Owner: "", Visibility: "", CreatedAt: time.Now().UTC(),
	}
	// authenticated owner's shared record — must NOT be visible to anon callers.
	authShared := Memory{
		ID: "f1f1f1f1-0000-0000-0000-000000000002", Content: "auth-shared",
		Scope: scope, Owner: "sub-owner", Visibility: "shared", CreatedAt: time.Now().UTC(),
	}
	// authenticated owner's private record — must NOT be visible to anon callers.
	authPrivate := Memory{
		ID: "f1f1f1f1-0000-0000-0000-000000000003", Content: "auth-private",
		Scope: scope, Owner: "sub-owner", Visibility: "", CreatedAt: time.Now().UTC(),
	}
	for _, m := range []Memory{ownerless, authShared, authPrivate} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", m.ID, err)
		}
	}

	// Search: anonymous caller sees only the anonymous-bucket record (owner=="").
	hits, err := s.Search(ctx, scope, Anonymous(), []float32{0.1, 0.2, 0.3}, 10, SearchOptions{})
	if err != nil {
		t.Fatalf("Search anon: %v", err)
	}
	if len(hits) != 1 {
		t.Errorf("Search anon: got %d want 1 (ownerless only)", len(hits))
	} else if hits[0].ID != ownerless.ID {
		t.Errorf("Search anon: got record %q, want ownerless %q", hits[0].ID, ownerless.ID)
	}
	for _, h := range hits {
		if h.Owner != "" {
			t.Errorf("Search anon leaked owner-stamped record: %s owner=%q", h.ID, h.Owner)
		}
	}

	// List: same restriction.
	lst, _, _, err := s.List(ctx, scope, Anonymous(), ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("List anon: %v", err)
	}
	if len(lst) != 1 {
		t.Errorf("List anon: got %d want 1 (ownerless only)", len(lst))
	}
	for _, h := range lst {
		if h.Owner != "" {
			t.Errorf("List anon leaked owner-stamped record: %s owner=%q", h.ID, h.Owner)
		}
	}

	// GetReadable: anon caller reads the anonymous-bucket record (owner=="").
	got, err := s.GetReadable(ctx, ownerless.ID, Anonymous())
	if err != nil {
		t.Errorf("GetReadable anon on ownerless record: unexpected error: %v", err)
	}
	if got.ID != ownerless.ID {
		t.Errorf("GetReadable anon ownerless: got %q want %q", got.ID, ownerless.ID)
	}

	// GetReadable: anon caller denied another owner's shared record → ErrNotFound.
	_, err = s.GetReadable(ctx, authShared.ID, Anonymous())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetReadable anon on shared record: want ErrNotFound, got %v", err)
	}

	// GetReadable: anon caller denied another owner's private record → ErrNotFound.
	_, err = s.GetReadable(ctx, authPrivate.ID, Anonymous())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetReadable anon on private record: want ErrNotFound, got %v", err)
	}

	// Authenticated caller still reads shared records (no regression).
	got, err = s.GetReadable(ctx, authShared.ID, Authenticated("sub-other"))
	if err != nil {
		t.Errorf("GetReadable authenticated on shared record: unexpected error: %v", err)
	}
	if got.ID != authShared.ID {
		t.Errorf("GetReadable authenticated shared: got %q want %q", got.ID, authShared.ID)
	}

	// Authenticated Search: sub-owner sees own private + own shared (2), sub-other sees own (0) + shared (1).
	ownerHits, err := s.Search(ctx, scope, Authenticated("sub-owner"), []float32{0.1, 0.2, 0.3}, 10, SearchOptions{})
	if err != nil {
		t.Fatalf("Search sub-owner: %v", err)
	}
	if len(ownerHits) != 2 {
		t.Errorf("Search sub-owner: got %d want 2 (private+shared)", len(ownerHits))
	}
	otherHits, err := s.Search(ctx, scope, Authenticated("sub-other"), []float32{0.1, 0.2, 0.3}, 10, SearchOptions{})
	if err != nil {
		t.Fatalf("Search sub-other: %v", err)
	}
	if len(otherHits) != 1 {
		t.Errorf("Search sub-other: got %d want 1 (shared only)", len(otherHits))
	}
}

// TestAnonBucketWriteSemantics verifies Q2 (engram-99z.13): the anonymous
// bucket (explicit owner=="") is mutually writable when sub=="" — the
// auth-disabled deployment case. Owner-stamped records are NOT mutable by an
// anonymous caller (fail-closed write isolation).
func TestAnonBucketWriteSemantics(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "anon-test:project:write-sem"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	// Ownerless record — mutually writable by anonymous callers.
	ownerless := Memory{
		ID: "f1f1f1f1-0000-0000-0000-000000000011", Content: "v1",
		Scope: scope, Owner: "", Visibility: "", CreatedAt: time.Now().UTC(),
	}
	// Owner-stamped record — must NOT be writable by anonymous callers.
	stamped := Memory{
		ID: "f1f1f1f1-0000-0000-0000-000000000012", Content: "s1",
		Scope: scope, Owner: "sub-owner", Visibility: "", CreatedAt: time.Now().UTC(),
	}
	for _, m := range []Memory{ownerless, stamped} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", m.ID, err)
		}
	}

	// getWritable on ownerless record with Anonymous() → success (anon bucket mutually writable).
	if _, err := s.getWritable(ctx, ownerless.ID, Anonymous(), authz.ActionWrite); err != nil {
		t.Errorf("getWritable anon on ownerless record: unexpected error: %v", err)
	}

	// getWritable on owner-stamped record with Anonymous() → ErrNotFound (fail-closed write isolation).
	_, err := s.getWritable(ctx, stamped.ID, Anonymous(), authz.ActionWrite)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("getWritable anon on owner-stamped record: want ErrNotFound, got %v", err)
	}

	// Update on ownerless record with Anonymous() → success (anon bucket).
	cur, err := s.FetchForUpdate(ctx, ownerless.ID, Anonymous())
	if err != nil {
		t.Fatalf("FetchForUpdate anon on ownerless record: unexpected error: %v", err)
	}
	if err := s.Update(ctx, cur, "v2", nil, nil, nil, []float32{0.2, 0.3, 0.4}); err != nil {
		t.Errorf("Update anon on ownerless record: unexpected error: %v", err)
	}
	got, err := s.Get(ctx, ownerless.ID)
	if err != nil {
		t.Fatalf("Get after anon Update: %v", err)
	}
	if got.Content != "v2" {
		t.Errorf("anon Update: content not applied, got %q", got.Content)
	}

	// Delete on owner-stamped record with Anonymous() → ErrNotFound (fail-closed).
	if err := s.Delete(ctx, stamped.ID, Anonymous()); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete anon on owner-stamped record: want ErrNotFound, got %v", err)
	}
	// Record must still exist.
	if _, err := s.Get(ctx, stamped.ID); err != nil {
		t.Errorf("owner-stamped record should survive anon Delete: %v", err)
	}

	// Delete on ownerless record with Anonymous() → success.
	if err := s.Delete(ctx, ownerless.ID, Anonymous()); err != nil {
		t.Errorf("Delete anon on ownerless record: unexpected error: %v", err)
	}
	if _, err := s.Get(ctx, ownerless.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("ownerless record should be gone after anon Delete")
	}
}

// TestAnonBucketDiscoveryReadIsolation verifies that SearchDiscovery respects
// the same fail-closed anonymous read semantics: anon callers see only
// anonymous-bucket discovery records (owner==""), not authenticated owners'
// shared discoveries.
func TestAnonBucketDiscoveryReadIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "discovery:repo:anon-test"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	mk := func(id, owner, vis string) {
		m := Memory{
			ID: id, Content: "d", Scope: scope, Category: "discovery", Kind: "fact",
			Owner: owner, Visibility: vis,
			Citations: []Citation{{Kind: "file", Ref: "f.go"}},
			CreatedAt: time.Now().UTC(),
		}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	mk("f1f1f1f1-0000-0000-0000-000000000021", "", "")            // ownerless
	mk("f1f1f1f1-0000-0000-0000-000000000022", "sub-A", "shared") // A's shared

	q := []float32{0.1, 0.2, 0.3}

	// Anonymous caller sees only the ownerless discovery.
	hits, err := s.SearchDiscovery(ctx, scope, "", Anonymous(), q, 10)
	if err != nil {
		t.Fatalf("SearchDiscovery anon: %v", err)
	}
	if len(hits) != 1 {
		t.Errorf("SearchDiscovery anon: got %d want 1 (ownerless only)", len(hits))
	}
	for _, h := range hits {
		if h.Owner != "" {
			t.Errorf("SearchDiscovery anon leaked owner-stamped discovery: %s owner=%q", h.ID, h.Owner)
		}
	}

	// Authenticated caller sees own + shared.
	authHits, err := s.SearchDiscovery(ctx, scope, "", Authenticated("sub-A"), q, 10)
	if err != nil {
		t.Fatalf("SearchDiscovery sub-A: %v", err)
	}
	if len(authHits) != 1 {
		t.Errorf("SearchDiscovery sub-A: got %d want 1 (own shared only, no ownerless under sub-A)", len(authHits))
	}

	// sub-B sees sub-A's shared discovery (no regression on authenticated shared read).
	bHits, err := s.SearchDiscovery(ctx, scope, "", Authenticated("sub-B"), q, 10)
	if err != nil {
		t.Fatalf("SearchDiscovery sub-B: %v", err)
	}
	if len(bHits) != 1 {
		t.Errorf("SearchDiscovery sub-B: got %d want 1 (A's shared)", len(bHits))
	}
	if bHits[0].Owner != "sub-A" || bHits[0].Visibility != "shared" {
		t.Errorf("SearchDiscovery sub-B: unexpected record %+v", bHits[0])
	}
}

// TestNilSubjectFailsClosed pins the core guarantee of the typed-Subject
// refactor: a nil Subject (what a discarded subjectFromContext error yields)
// denies on every authz path — empty reads, ErrNotFound id-gates, and a rejected
// bulk delete — rather than silently resolving to the anonymous bucket.
func TestNilSubjectFailsClosed(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:nil-subject"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	// Seed an ownerless (anonymous-bucket) record and an owned record.
	anon := Memory{ID: "a0a0a0a0-0000-0000-0000-000000000001", Content: "anon", Scope: scope, Owner: "", CreatedAt: time.Now().UTC()}
	owned := Memory{ID: "a0a0a0a0-0000-0000-0000-000000000002", Content: "owned", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
	for _, m := range []Memory{anon, owned} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}

	var nilSubj Subject // zero value == nil: the discarded-error case

	// Reads return nothing.
	if hits, err := s.Search(ctx, scope, nilSubj, []float32{0.1, 0.2, 0.3}, 10, SearchOptions{}); err != nil || len(hits) != 0 {
		t.Errorf("Search(nil): want 0 hits nil err, got %d hits, %v", len(hits), err)
	}
	if mems, _, _, err := s.List(ctx, scope, nilSubj, ListOptions{Limit: 20}); err != nil || len(mems) != 0 {
		t.Errorf("List(nil): want 0 mems nil err, got %d, %v", len(mems), err)
	}
	if hits, err := s.SearchDiscovery(ctx, scope, "", nilSubj, []float32{0.1, 0.2, 0.3}, 10); err != nil || len(hits) != 0 {
		t.Errorf("SearchDiscovery(nil): want 0 hits nil err, got %d, %v", len(hits), err)
	}

	// Id-gates return ErrNotFound (even for the ownerless record).
	if _, err := s.GetReadable(ctx, anon.ID, nilSubj); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetReadable(nil): want ErrNotFound, got %v", err)
	}
	if _, err := s.FetchForUpdate(ctx, anon.ID, nilSubj); !errors.Is(err, ErrNotFound) {
		t.Errorf("FetchForUpdate(nil): want ErrNotFound, got %v", err)
	}
	if err := s.Delete(ctx, anon.ID, nilSubj); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete(nil): want ErrNotFound, got %v", err)
	}
	if err := s.SetVisibility(ctx, anon.ID, nilSubj, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("SetVisibility(nil): want ErrNotFound, got %v", err)
	}
	if err := s.OwnedOrAbsent(ctx, anon.ID, nilSubj); !errors.Is(err, ErrNotFound) {
		t.Errorf("OwnedOrAbsent(nil) on existing id: want ErrNotFound, got %v", err)
	}
	// ABSENT id short-circuits before the subject switch (Get→ErrNotFound):
	// nothing to overwrite, so OwnedOrAbsent returns nil even for a nil subject —
	// the create-new arm, distinct from the existing-id default-deny above.
	if err := s.OwnedOrAbsent(ctx, "00000000-0000-0000-0000-0000000000ab", nilSubj); err != nil {
		t.Errorf("OwnedOrAbsent(nil) on absent id: want nil (create-new, nothing to overwrite), got %v", err)
	}

	// Bulk delete is rejected and removes nothing.
	if err := s.DeleteAll(ctx, scope, nilSubj); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteAll(nil): want ErrNotFound, got %v", err)
	}
	if _, err := s.Get(ctx, anon.ID); err != nil {
		t.Errorf("DeleteAll(nil) must not delete: record gone, %v", err)
	}
}

// DeleteAllRaw removes every point in scope regardless of owner — test cleanup only.
func (s *Store) DeleteAllRaw(ctx context.Context, scope string) error {
	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelectorFilter(&qdrant.Filter{
			Must: []*qdrant.Condition{qdrant.NewMatch("scope", scope)},
		}),
	})
	return err
}

func TestListScopes(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	a, b := "ls-test:project:a", "ls-test:project:b"
	defer func() {
		cleanupErr(t, "DeleteAllRaw "+a, s.DeleteAllRaw(ctx, a))
		cleanupErr(t, "DeleteAllRaw "+b, s.DeleteAllRaw(ctx, b))
	}()
	mk := func(id, scope, owner string) {
		m := Memory{ID: id, Content: "x", Scope: scope, Owner: owner, CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	mk("c1111111-0000-0000-0000-000000000001", a, "sub-A")
	mk("c1111111-0000-0000-0000-000000000002", a, "sub-A")
	mk("c1111111-0000-0000-0000-000000000003", b, "sub-A")
	mk("c1111111-0000-0000-0000-000000000004", a, "sub-B") // foreign: excluded for sub-A

	scopes, approx, err := s.ListScopes(ctx, Authenticated("sub-A"))
	if err != nil {
		t.Fatalf("ListScopes: %v", err)
	}
	if approx {
		t.Errorf("approximate=true for a tiny set, want false")
	}
	counts := map[string]uint64{}
	for _, sc := range scopes {
		counts[sc.Scope] = sc.Count
	}
	if counts[a] != 2 || counts[b] != 1 {
		t.Errorf("counts = %v, want {%s:2, %s:1} (sub-B's record excluded)", counts, a, b)
	}
}

func TestCountAnonymousBucket(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	before, err := s.CountAnonymousBucket(ctx)
	if err != nil {
		t.Fatalf("CountAnonymousBucket(before): %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	anon1 := Memory{ID: "a1111111-0000-0000-0000-000000000001", Content: "anon one", Scope: "anon-count-scope", Owner: "", Source: "agent-inferred", Category: "fact", CreatedAt: now}
	anon2 := Memory{ID: "a1111111-0000-0000-0000-000000000002", Content: "anon two", Scope: "anon-count-scope", Owner: "", Source: "agent-inferred", Category: "fact", CreatedAt: now}
	owned := Memory{ID: "a1111111-0000-0000-0000-000000000003", Content: "owned", Scope: "anon-count-scope", Owner: "sub-x", Source: "agent-inferred", Category: "fact", CreatedAt: now}
	for _, m := range []Memory{anon1, anon2, owned} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}
	defer func() {
		_ = s.Delete(ctx, anon1.ID, Anonymous())
		_ = s.Delete(ctx, anon2.ID, Anonymous())
		_ = s.Delete(ctx, owned.ID, Authenticated("sub-x"))
	}()

	after, err := s.CountAnonymousBucket(ctx)
	if err != nil {
		t.Fatalf("CountAnonymousBucket(after): %v", err)
	}
	if after-before != 2 {
		t.Fatalf("anonymous-bucket delta = %d, want 2", after-before)
	}
}

func TestListPagination(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "page-test:project:x"
	base := time.Now().UTC().Truncate(time.Second)
	// 5 owned records, descending CreatedAt so order is deterministic.
	for i := 0; i < 5; i++ {
		m := Memory{
			ID:      fmt.Sprintf("d0000000-0000-0000-0000-00000000000%d", i),
			Content: fmt.Sprintf("rec %d", i), Scope: scope, Owner: "owner-A",
			Visibility: "", Category: "convention", Source: "agent-inferred",
			CreatedAt: base.Add(time.Duration(-i) * time.Minute),
		}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	defer func() {
		for i := 0; i < 5; i++ {
			_ = s.Delete(ctx, fmt.Sprintf("d0000000-0000-0000-0000-00000000000%d", i), Authenticated("owner-A"))
		}
	}()
	subj := Authenticated("owner-A")

	// Page 1: limit 2, offset 0 -> 2 records, total 5.
	got, total, _, err := s.List(ctx, scope, subj, ListOptions{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 5 {
		t.Fatalf("total=%d, want 5", total)
	}
	if len(got) != 2 || got[0].Content != "rec 0" {
		t.Fatalf("page1 = %d records, first=%q", len(got), got[0].Content)
	}
	// Page 3: offset 4, limit 2 -> 1 record (the tail).
	got, _, _, _ = s.List(ctx, scope, subj, ListOptions{Limit: 2, Offset: 4})
	if len(got) != 1 {
		t.Fatalf("page3 = %d records, want 1", len(got))
	}
	// Offset past total -> empty page, no panic, real total.
	got, total, _, err = s.List(ctx, scope, subj, ListOptions{Limit: 2, Offset: 99})
	if err != nil || len(got) != 0 || total != 5 {
		t.Fatalf("oob: err=%v len=%d total=%d, want nil/0/5", err, len(got), total)
	}
}

// TestListExactTotalPastOldCap proves the scanCap ceiling is gone: with > 1000
// readable records, List returns an exact total (Count), not a capped 1000.
func TestListExactTotalPastOldCap(t *testing.T) {
	if testing.Short() {
		t.Skip("writes 1001 points; skipped in -short")
	}
	s := testStore(t)
	ctx := context.Background()
	scope := "cap-test:project:x"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	const n = 1001
	for i := 0; i < n; i++ {
		m := Memory{
			ID:      fmt.Sprintf("c0000000-0000-0000-0000-%012d", i),
			Content: "c", Scope: scope, Owner: "sub-A",
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert %d: %v", i, err)
		}
	}
	_, total, _, err := s.List(ctx, scope, Authenticated("sub-A"), ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != n {
		t.Errorf("exact total: got %d want %d (scanCap not retired?)", total, n)
	}
}

func TestListCategoryAndVisibilityFilter(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "filter-test:project:x"
	now := time.Now().UTC().Truncate(time.Second)
	// Private records are stored with Visibility=="" (canonical representation).
	seed := []Memory{
		{ID: "e0000000-0000-0000-0000-000000000001", Content: "conv shared", Scope: scope, Owner: "owner-A", Visibility: "shared", Category: "convention", Source: "agent-inferred", CreatedAt: now},
		{ID: "e0000000-0000-0000-0000-000000000002", Content: "gotcha private", Scope: scope, Owner: "owner-A", Visibility: "", Category: "gotcha", Source: "agent-inferred", CreatedAt: now},
		{ID: "e0000000-0000-0000-0000-000000000003", Content: "preference private", Scope: scope, Owner: "owner-A", Visibility: "", Category: "preference", Source: "agent-inferred", CreatedAt: now},
	}
	for _, m := range seed {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}
	defer func() {
		for _, m := range seed {
			_ = s.Delete(ctx, m.ID, Authenticated("owner-A"))
		}
	}()
	subj := Authenticated("owner-A")

	// Single-category filter.
	got, total, _, _ := s.List(ctx, scope, subj, ListOptions{Limit: 10, Categories: []string{"gotcha"}})
	if total != 1 || len(got) != 1 || got[0].Category != "gotcha" {
		t.Fatalf("category filter: total=%d len=%d", total, len(got))
	}

	// Multi-category OR filter (af5.14): both gotcha and preference must match.
	got, total, _, _ = s.List(ctx, scope, subj, ListOptions{Limit: 10, Categories: []string{"gotcha", "preference"}})
	if total != 2 || len(got) != 2 {
		t.Fatalf("multi-category OR filter: total=%d len=%d, want 2", total, len(got))
	}
	for _, r := range got {
		if r.Category != "gotcha" && r.Category != "preference" {
			t.Errorf("multi-category OR returned unexpected category: %q", r.Category)
		}
	}

	// Shared visibility filter: returns only the shared record.
	got, total, _, _ = s.List(ctx, scope, subj, ListOptions{Limit: 10, Visibility: "shared"})
	if total != 1 || len(got) != 1 || got[0].Visibility != "shared" {
		t.Fatalf("visibility=shared filter: total=%d len=%d", total, len(got))
	}

	// Private visibility filter: returns records with stored visibility=="" (2 private records),
	// never the shared one.
	got, total, _, _ = s.List(ctx, scope, subj, ListOptions{Limit: 10, Visibility: "private"})
	if total != 2 || len(got) != 2 {
		t.Fatalf("visibility=private filter: total=%d len=%d, want 2 private records", total, len(got))
	}
	for _, r := range got {
		if r.Visibility == "shared" {
			t.Errorf("visibility=private filter leaked shared record: id=%s", r.ID)
		}
		if r.ID == "e0000000-0000-0000-0000-000000000001" {
			t.Errorf("visibility=private filter returned the shared record")
		}
	}
}

// TestSearchCategoryFilter proves Store.Search's Categories filter (D-09):
// OR-composed across multiple values, an unknown value matches nothing
// (never an error, D-11), and nil/[""] are passthroughs — mirroring
// TestListCategoryAndVisibilityFilter's list-lane assertions on the search
// lane, since both now share categoryMatchCondition.
func TestSearchCategoryFilter(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "search-category-test:project:x"
	now := time.Now().UTC().Truncate(time.Second)
	seed := []Memory{
		{ID: "f0000000-0000-0000-0000-000000000001", Content: "a decision", Scope: scope, Owner: "owner-A", Category: "decision", Source: "agent-inferred", CreatedAt: now},
		{ID: "f0000000-0000-0000-0000-000000000002", Content: "a preference", Scope: scope, Owner: "owner-A", Category: "preference", Source: "agent-inferred", CreatedAt: now},
		{ID: "f0000000-0000-0000-0000-000000000003", Content: "a gotcha", Scope: scope, Owner: "owner-A", Category: "gotcha", Source: "agent-inferred", CreatedAt: now},
	}
	vecs := map[string][]float32{
		seed[0].ID: {0.9, 0.1, 0.0},
		seed[1].ID: {0.1, 0.9, 0.0},
		seed[2].ID: {0.0, 0.1, 0.9},
	}
	for _, m := range seed {
		if err := s.Upsert(ctx, m, vecs[m.ID]); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}
	defer func() {
		for _, m := range seed {
			_ = s.Delete(ctx, m.ID, Authenticated("owner-A"))
		}
	}()
	subj := Authenticated("owner-A")
	q := []float32{0.3, 0.3, 0.3}

	sortedIDs := func(ms []Memory) []string {
		out := recordIDs(ms)
		slices.Sort(out)
		return out
	}

	cases := []struct {
		name string
		cats []string
		want []string
	}{
		{"single value", []string{"decision"}, []string{seed[0].ID}},
		{"two-value OR", []string{"decision", "gotcha"}, []string{seed[0].ID, seed[2].ID}},
		{"unknown value returns empty, not error", []string{"nonexistent"}, []string{}},
		{"nil is passthrough", nil, []string{seed[0].ID, seed[1].ID, seed[2].ID}},
		{"[\"\"] is passthrough", []string{""}, []string{seed[0].ID, seed[1].ID, seed[2].ID}},
	}
	for _, tc := range cases {
		hits, err := s.Search(ctx, scope, subj, q, 10, SearchOptions{Categories: tc.cats})
		if err != nil {
			t.Fatalf("%s: Search: %v", tc.name, err)
		}
		want := slices.Clone(tc.want)
		slices.Sort(want)
		if got := sortedIDs(hits); !slices.Equal(got, want) {
			t.Errorf("%s: got %v want %v", tc.name, got, want)
		}
	}
}

// TestCategoryMatchConditionEdges is a pure (no-Qdrant) unit test over
// categoryMatchCondition's nil/empty/all-empty-string passthrough behavior
// and its non-nil condition for a mixed list.
func TestCategoryMatchConditionEdges(t *testing.T) {
	t.Parallel()
	if c := categoryMatchCondition(nil); c != nil {
		t.Errorf("categoryMatchCondition(nil) = %v, want nil", c)
	}
	if c := categoryMatchCondition([]string{}); c != nil {
		t.Errorf("categoryMatchCondition([]string{}) = %v, want nil", c)
	}
	if c := categoryMatchCondition([]string{""}); c != nil {
		t.Errorf(`categoryMatchCondition([""]) = %v, want nil (empty-string element skipped)`, c)
	}
	got := categoryMatchCondition([]string{"decision", ""})
	want := categoryMatchCondition([]string{"decision"})
	if got == nil || want == nil {
		t.Fatalf("categoryMatchCondition([\"decision\", \"\"]) or ([\"decision\"]) unexpectedly nil: got=%v want=%v", got, want)
	}
	if got.String() != want.String() {
		t.Errorf("categoryMatchCondition([\"decision\", \"\"]) = %v, want same as ([\"decision\"]) = %v", got, want)
	}
}

// TestSearchCategoryFilterPreRanking makes SC2's "applied before vector
// ranking" claim falsifiable rather than assumed: it first proves the
// preference record really would win the unfiltered ranking (it is at index
// 0), then proves the same search with Categories:["decision"] returns
// exactly the decision record and never the higher-scored preference one —
// the exclusion happens because Qdrant never returns the record to be
// ranked, not because a post-hoc trim removed it.
func TestSearchCategoryFilterPreRanking(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "search-category-preranking:project:x"
	now := time.Now().UTC().Truncate(time.Second)
	subj := Authenticated("owner-preranking")
	q := []float32{1, 0, 0}
	prefID := "f1000000-0000-0000-0000-000000000001"
	decID := "f1000000-0000-0000-0000-000000000002"
	pref := Memory{ID: prefID, Content: "near-exact match", Scope: scope, Owner: "owner-preranking", Category: "preference", Source: "agent-inferred", CreatedAt: now}
	dec := Memory{ID: decID, Content: "weak match", Scope: scope, Owner: "owner-preranking", Category: "decision", Source: "agent-inferred", CreatedAt: now}
	if err := s.Upsert(ctx, pref, []float32{1, 0, 0}); err != nil {
		t.Fatalf("seed pref: %v", err)
	}
	if err := s.Upsert(ctx, dec, []float32{0.5, 0.5, 0}); err != nil {
		t.Fatalf("seed dec: %v", err)
	}
	defer func() {
		_ = s.Delete(ctx, prefID, subj)
		_ = s.Delete(ctx, decID, subj)
	}()

	unfiltered, err := s.Search(ctx, scope, subj, q, 5, SearchOptions{})
	if err != nil {
		t.Fatalf("unfiltered Search: %v", err)
	}
	if len(unfiltered) == 0 || unfiltered[0].ID != prefID {
		t.Fatalf("unfiltered Search: want preference record ranked first, got %v", recordIDs(unfiltered))
	}

	filtered, err := s.Search(ctx, scope, subj, q, 5, SearchOptions{Categories: []string{"decision"}})
	if err != nil {
		t.Fatalf("filtered Search: %v", err)
	}
	if got := recordIDs(filtered); !slices.Equal(got, []string{decID}) {
		t.Errorf("filtered Search: got %v want [%s] (category pre-filter must exclude the higher-ranked preference record)", got, decID)
	}
}

// TestCategoryFilterDoesNotWidenVisibility is the D-16/SC4 assertion: the
// category filter composes strictly INSIDE ownerOrSharedCondition and can
// only narrow, never widen, what a caller may read. It also proves a shared
// read grant is not a write grant.
func TestCategoryFilterDoesNotWidenVisibility(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "search-category-widen-test:project:x"
	now := time.Now().UTC().Truncate(time.Second)
	ownerA := Authenticated("owner-catA")
	ownerB := Authenticated("owner-catB")
	vec := []float32{0.1, 0.2, 0.3}

	privID := "f2000000-0000-0000-0000-000000000001"
	priv := Memory{ID: privID, Content: "private decision", Scope: scope, Owner: "owner-catA", Category: "decision", Source: "agent-inferred", CreatedAt: now}
	if err := s.Upsert(ctx, priv, vec); err != nil {
		t.Fatalf("seed priv: %v", err)
	}
	defer func() { _ = s.Delete(ctx, privID, ownerA) }()

	// Owner B's category-filtered search must see zero results — the private
	// record stays invisible; the category filter cannot widen visibility.
	gotB, err := s.Search(ctx, scope, ownerB, vec, 10, SearchOptions{Categories: []string{"decision"}})
	if err != nil {
		t.Fatalf("owner B Search: %v", err)
	}
	if len(gotB) != 0 {
		t.Fatalf("owner B category-filtered Search: got %d results, want 0 (private record must stay invisible)", len(gotB))
	}

	sharedID := "f2000000-0000-0000-0000-000000000002"
	shared := Memory{ID: sharedID, Content: "shared decision", Scope: scope, Owner: "owner-catA", Visibility: "shared", Category: "decision", Source: "agent-inferred", CreatedAt: now}
	if err := s.Upsert(ctx, shared, vec); err != nil {
		t.Fatalf("seed shared: %v", err)
	}
	defer func() { _ = s.Delete(ctx, sharedID, ownerA) }()

	gotB2, err := s.Search(ctx, scope, ownerB, vec, 10, SearchOptions{Categories: []string{"decision"}})
	if err != nil {
		t.Fatalf("owner B Search (after shared): %v", err)
	}
	if got := recordIDs(gotB2); !slices.Contains(got, sharedID) {
		t.Fatalf("owner B category-filtered Search: got %v, want shared record %s present", got, sharedID)
	}

	// A readable record is not a writable one: owner B's write path must
	// still fail with the not-found-shaped error.
	if err := s.SetVisibility(ctx, sharedID, ownerB, false); !errors.Is(err, ErrNotFound) {
		t.Errorf("owner B SetVisibility on shared record: err=%v, want ErrNotFound (read grant must not become a write grant)", err)
	}
}

// TestSearchCategoryFilterOrderingUnchanged pins the ordering edge: a
// Categories filter that excludes nothing (every seeded record shares the one
// listed category) must not perturb the result order Search would otherwise
// return.
func TestSearchCategoryFilterOrderingUnchanged(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "search-category-ordering-test:project:x"
	now := time.Now().UTC().Truncate(time.Second)
	subj := Authenticated("owner-order-cat")
	q := []float32{0.4, 0.3, 0.2}
	vecs := [][]float32{
		{0.4, 0.3, 0.2},
		{0.35, 0.3, 0.25},
		{0.3, 0.35, 0.2},
		{0.2, 0.4, 0.3},
	}
	ids := []string{
		"f3000000-0000-0000-0000-000000000001",
		"f3000000-0000-0000-0000-000000000002",
		"f3000000-0000-0000-0000-000000000003",
		"f3000000-0000-0000-0000-000000000004",
	}
	for i, id := range ids {
		m := Memory{ID: id, Content: "shared-category record", Scope: scope, Owner: "owner-order-cat", Category: "gotcha", Source: "agent-inferred", CreatedAt: now}
		if err := s.Upsert(ctx, m, vecs[i]); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	defer func() {
		for _, id := range ids {
			_ = s.Delete(ctx, id, subj)
		}
	}()

	unfiltered, err := s.Search(ctx, scope, subj, q, 10, SearchOptions{})
	if err != nil {
		t.Fatalf("unfiltered Search: %v", err)
	}
	filtered, err := s.Search(ctx, scope, subj, q, 10, SearchOptions{Categories: []string{"gotcha"}})
	if err != nil {
		t.Fatalf("filtered Search: %v", err)
	}
	if got, want := recordIDs(filtered), recordIDs(unfiltered); !slices.Equal(got, want) {
		t.Errorf("filtered vs unfiltered Search ordering: got %v want %v (a filter matching everything must not reorder)", got, want)
	}
}

// TestListPrivateFilterCrossActorIsolation verifies that the private visibility
// filter preserves authz isolation: caller B must not see caller A's private
// records even when Visibility=="private" is specified in ListOptions.
func TestListPrivateFilterCrossActorIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-private-filter:project:x"
	now := time.Now().UTC().Truncate(time.Second)
	// A private, B private, B shared.
	aPriv := Memory{ID: "e1000000-0000-0000-0000-000000000001", Content: "A private", Scope: scope, Owner: "owner-A", Visibility: "", Category: "convention", Source: "agent-inferred", CreatedAt: now}
	bPriv := Memory{ID: "e1000000-0000-0000-0000-000000000002", Content: "B private", Scope: scope, Owner: "owner-B", Visibility: "", Category: "convention", Source: "agent-inferred", CreatedAt: now}
	bShared := Memory{ID: "e1000000-0000-0000-0000-000000000003", Content: "B shared", Scope: scope, Owner: "owner-B", Visibility: "shared", Category: "convention", Source: "agent-inferred", CreatedAt: now}
	for _, m := range []Memory{aPriv, bPriv, bShared} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}
	defer func() {
		_ = s.Delete(ctx, aPriv.ID, Authenticated("owner-A"))
		_ = s.Delete(ctx, bPriv.ID, Authenticated("owner-B"))
		_ = s.Delete(ctx, bShared.ID, Authenticated("owner-B"))
	}()

	// Caller A with Visibility=="private": sees only A's private record; never B's private or B's shared.
	got, total, _, err := s.List(ctx, scope, Authenticated("owner-A"), ListOptions{Limit: 10, Visibility: "private"})
	if err != nil {
		t.Fatalf("List owner-A private: %v", err)
	}
	if total != 1 || len(got) != 1 {
		t.Fatalf("owner-A private filter: total=%d len=%d, want 1", total, len(got))
	}
	if got[0].ID != aPriv.ID {
		t.Errorf("owner-A private filter: got record %q, want A's private %q", got[0].ID, aPriv.ID)
	}
	for _, r := range got {
		if r.Owner != "owner-A" {
			t.Errorf("private filter leaked another owner's record: id=%s owner=%q", r.ID, r.Owner)
		}
	}

	// Caller B with Visibility=="private": sees only B's private record; never A's private.
	got, total, _, err = s.List(ctx, scope, Authenticated("owner-B"), ListOptions{Limit: 10, Visibility: "private"})
	if err != nil {
		t.Fatalf("List owner-B private: %v", err)
	}
	if total != 1 || len(got) != 1 {
		t.Fatalf("owner-B private filter: total=%d len=%d, want 1", total, len(got))
	}
	if got[0].ID != bPriv.ID {
		t.Errorf("owner-B private filter: got record %q, want B's private %q", got[0].ID, bPriv.ID)
	}
}

func TestListFilterPreservesIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-filter:project:x"
	now := time.Now().UTC().Truncate(time.Second)
	// owner-A private + owner-B private, both convention.
	a := Memory{ID: "f0000000-0000-0000-0000-000000000001", Content: "A priv", Scope: scope, Owner: "owner-A", Visibility: "", Category: "convention", Source: "agent-inferred", CreatedAt: now}
	b := Memory{ID: "f0000000-0000-0000-0000-000000000002", Content: "B priv", Scope: scope, Owner: "owner-B", Visibility: "", Category: "convention", Source: "agent-inferred", CreatedAt: now}
	for _, m := range []Memory{a, b} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	defer func() {
		_ = s.Delete(ctx, a.ID, Authenticated("owner-A"))
		_ = s.Delete(ctx, b.ID, Authenticated("owner-B"))
	}()
	// Caller B with a category filter must still never see A's private record.
	got, total, _, _ := s.List(ctx, scope, Authenticated("owner-B"), ListOptions{Limit: 10, Categories: []string{"convention"}})
	if total != 1 || len(got) != 1 || got[0].Owner != "owner-B" {
		t.Fatalf("isolation breach: total=%d, %+v", total, got)
	}
}

func TestPayloadRoundTripWindow(t *testing.T) {
	nb := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	na := time.Date(2031, 6, 7, 8, 9, 10, 0, time.UTC)
	m := Memory{
		ID: "22222222-2222-2222-2222-222222222222", Content: "windowed",
		Scope: "win-test:project:x", Owner: "sub-A", CreatedAt: time.Now().UTC(),
		NotBefore: &nb, NotAfter: &na,
	}
	got := fromPayload(m.ID, qdrant.NewValueMap(payload(m)))
	if got.NotBefore == nil || !got.NotBefore.Equal(nb) {
		t.Errorf("NotBefore round-trip: got %v want %v", got.NotBefore, nb)
	}
	if got.NotAfter == nil || !got.NotAfter.Equal(na) {
		t.Errorf("NotAfter round-trip: got %v want %v", got.NotAfter, na)
	}
	// Unwindowed record: keys absent, pointers stay nil.
	plain := fromPayload("id", qdrant.NewValueMap(payload(Memory{ID: "id", Content: "x"})))
	if plain.NotBefore != nil || plain.NotAfter != nil {
		t.Errorf("unwindowed: want nil pointers, got nb=%v na=%v", plain.NotBefore, plain.NotAfter)
	}
}

func TestWithClockOverridesNow(t *testing.T) {
	fixed := time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)
	s := New(nil, "c", WithClock(func() time.Time { return fixed }))
	if got := s.now(); !got.Equal(fixed) {
		t.Errorf("WithClock: got %v want %v", got, fixed)
	}
	// Default clock is time.Now (non-zero, recent).
	d := New(nil, "c")
	if d.now().IsZero() {
		t.Error("default clock returned zero time")
	}
}

// TestWithAuthzOption exercises the WithAuthz Option's wiring itself (IN-02):
// it has no callers elsewhere in the repo (all authz-injection tests use the
// decideBucketHook/decideRecordHook function-var seams instead, since
// *authz.PDP has no exported constructor besides MustDefault, so WithAuthz
// today can only reinstall the same default policy corpus). This proves the
// Option correctly installs the given *authz.PDP rather than silently no-op'ing.
func TestWithAuthzOption(t *testing.T) {
	pdp := authz.MustDefault()
	s := New(nil, "c", WithAuthz(pdp))
	if s.authz != pdp {
		t.Error("WithAuthz did not install the given *authz.PDP")
	}
	// Default (no WithAuthz) also installs a non-nil PDP via authz.MustDefault().
	d := New(nil, "c")
	if d.authz == nil {
		t.Error("default authz is nil; want authz.MustDefault()")
	}
}

func TestRecallWindowGate(t *testing.T) {
	s := testStore(t)
	fixed := time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixed } // white-box override
	ctx := context.Background()
	scope := "gate-test:project:x"
	subj := Authenticated("sub-A")
	past := fixed.Add(-24 * time.Hour)
	future := fixed.Add(24 * time.Hour)

	mk := func(id string, nb, na *time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A",
			CreatedAt: fixed, NotBefore: nb, NotAfter: na}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, subj)) })
	}
	mkVis := func(id, owner, vis string, nb, na *time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: owner, Visibility: vis,
			CreatedAt: fixed, NotBefore: nb, NotAfter: na}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, Authenticated(owner))) })
	}
	mk("a0000000-0000-0000-0000-000000000001", nil, nil)       // unwindowed -> visible
	mk("a0000000-0000-0000-0000-000000000002", &past, &future) // active -> visible
	mk("a0000000-0000-0000-0000-000000000003", &future, nil)   // scheduled -> hidden
	mk("a0000000-0000-0000-0000-000000000004", nil, &past)     // expired -> hidden
	// sub-B's SHARED but scheduled record: must stay hidden from sub-A until active
	mkVis("a0000000-0000-0000-0000-000000000005", "sub-B", "shared", &future, nil)

	// Active set is exactly the unwindowed + active records; a count-only check
	// would pass a transposition (e.g. expired slipping in as active drops out).
	wantActive := []string{
		"a0000000-0000-0000-0000-000000000001",
		"a0000000-0000-0000-0000-000000000002",
	}
	assertActiveSet := func(label string, ms []Memory) {
		t.Helper()
		got := make(map[string]bool, len(ms))
		for _, m := range ms {
			got[m.ID] = true
		}
		if len(got) != len(wantActive) {
			t.Errorf("%s: got ids %v want exactly %v", label, got, wantActive)
			return
		}
		for _, id := range wantActive {
			if !got[id] {
				t.Errorf("%s: missing expected id %s (got %v)", label, id, got)
			}
		}
	}

	hits, err := s.Search(ctx, scope, subj, []float32{0.1, 0.2, 0.3}, 10, SearchOptions{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	assertActiveSet("Search", hits)
	lst, _, _, err := s.List(ctx, scope, subj, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	assertActiveSet("List", lst)
	// By-id is ungated: the scheduled record is still fetchable directly.
	if _, err := s.GetReadable(ctx, "a0000000-0000-0000-0000-000000000003", subj); err != nil {
		t.Errorf("GetReadable on scheduled record should be ungated, got %v", err)
	}
}

func TestListScheduledStates(t *testing.T) {
	s := testStore(t)
	fixed := time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixed }
	ctx := context.Background()
	scope := "sched-test:project:x"
	subj := Authenticated("sub-A")
	past := fixed.Add(-24 * time.Hour)
	future := fixed.Add(24 * time.Hour)

	mk := func(id string, nb, na *time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A",
			CreatedAt: fixed, NotBefore: nb, NotAfter: na}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, subj)) })
	}
	mk("b0000000-0000-0000-0000-000000000001", &future, nil)   // scheduled
	mk("b0000000-0000-0000-0000-000000000002", nil, &past)     // expired
	mk("b0000000-0000-0000-0000-000000000003", &past, &future) // active -> never listed

	sched, err := s.ListScheduled(ctx, scope, subj, ScheduledPending, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("scheduled: %v", err)
	}
	if len(sched) != 1 || sched[0].ID != "b0000000-0000-0000-0000-000000000001" {
		t.Errorf("ScheduledPending: got %d want 1 (the future record)", len(sched))
	}
	exp, err := s.ListScheduled(ctx, scope, subj, ScheduledExpired, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("expired: %v", err)
	}
	if len(exp) != 1 || exp[0].ID != "b0000000-0000-0000-0000-000000000002" {
		t.Errorf("ScheduledExpired: got %d want 1 (the past record)", len(exp))
	}
	all, err := s.ListScheduled(ctx, scope, subj, ScheduledAll, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("ScheduledAll: got %d want 2 (scheduled+expired, never active)", len(all))
	}
}

// TestListScheduledSupersededHidden (WR-02) pins that ListScheduled applies
// the same superseded_by soft-hide gate Search/List already carry: a
// scheduled record that has since been superseded must not still surface in
// the management view as a live pending/expired candidate.
func TestListScheduledSupersededHidden(t *testing.T) {
	s := testStore(t)
	fixed := time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixed }
	ctx := context.Background()
	scope := "sched-test:project:superseded-hidden"
	subj := Authenticated("sub-A")
	future := fixed.Add(24 * time.Hour)

	supersededID := "b1000000-0000-0000-0000-000000000001"
	liveID := "b1000000-0000-0000-0000-000000000002"
	newID := "b1000000-0000-0000-0000-000000000003"

	mk := func(id string, nb, na *time.Time, supersededBy *string) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A",
			CreatedAt: fixed, NotBefore: nb, NotAfter: na, SupersededBy: supersededBy}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, subj)) })
	}
	mk(supersededID, &future, nil, &newID) // scheduled but superseded -> excluded
	mk(liveID, &future, nil, nil)          // scheduled, live -> included

	sched, err := s.ListScheduled(ctx, scope, subj, ScheduledPending, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListScheduled: %v", err)
	}
	if got := recordIDs(sched); slices.Contains(got, supersededID) {
		t.Errorf("ListScheduled: superseded record %s present, want excluded: %v", supersededID, got)
	}
	if got := recordIDs(sched); !slices.Contains(got, liveID) {
		t.Errorf("ListScheduled: live record %s absent, want present: %v", liveID, got)
	}
}

// TestListScheduledRejectsInvalidState pins the store-layer guard (hr2.5): an
// unrecognized ScheduledState must be rejected outright, not silently treated as
// ScheduledPending. The handler validates already; this defends direct callers
// that bypass it.
func TestListScheduledRejectsInvalidState(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	_, err := s.ListScheduled(ctx, "any:project:x", Authenticated("sub-A"),
		ScheduledState("bogus"), ListOptions{Limit: 10})
	if err == nil {
		t.Fatal("ListScheduled with invalid state returned nil error; want a store-layer rejection")
	}
}

// TestListScheduledOwnerIsolation pins that ListScheduled is owner-only: a caller
// never sees another actor's scheduled/expired records — not even `shared` ones.
// A shared+scheduled memory must stay invisible to other actors until it becomes
// active (then normal recall surfaces it); the management view must not leak it
// early. Guards the deferred-reveal guarantee against an ownerOrShared regression.
func TestListScheduledOwnerIsolation(t *testing.T) {
	s := testStore(t)
	fixed := time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixed }
	ctx := context.Background()
	scope := "sched-iso:project:x"
	future := fixed.Add(24 * time.Hour)
	past := fixed.Add(-24 * time.Hour)

	mkVis := func(id, owner, vis string, nb, na *time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: owner, Visibility: vis,
			CreatedAt: fixed, NotBefore: nb, NotAfter: na}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, Authenticated(owner))) })
	}
	// sub-A's records: one private+scheduled, one shared+scheduled, one shared+expired.
	mkVis("d0000000-0000-0000-0000-000000000001", "sub-A", "", &future, nil)
	mkVis("d0000000-0000-0000-0000-000000000002", "sub-A", "shared", &future, nil)
	mkVis("d0000000-0000-0000-0000-000000000003", "sub-A", "shared", nil, &past)

	// sub-B must see NONE of sub-A's windowed records via any state — owner-only.
	subB := Authenticated("sub-B")
	for _, st := range []ScheduledState{ScheduledPending, ScheduledExpired, ScheduledAll} {
		got, err := s.ListScheduled(ctx, scope, subB, st, ListOptions{Limit: 10})
		if err != nil {
			t.Fatalf("ListScheduled(%s) for sub-B: %v", st, err)
		}
		if len(got) != 0 {
			t.Errorf("ListScheduled(%s): sub-B saw %d of sub-A's records (incl. shared); want 0 — owner-only", st, len(got))
		}
	}
	// sub-A still sees their own: 2 scheduled (pending) + 1 expired = 3 in `all`.
	own, err := s.ListScheduled(ctx, scope, Authenticated("sub-A"), ScheduledAll, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListScheduled(all) for sub-A: %v", err)
	}
	if len(own) != 3 {
		t.Errorf("ListScheduled(all) for owner sub-A: got %d want 3 (own records visible)", len(own))
	}
}

// TestNotAfterBoundaryInstant pins the epoch-second boundary semantics at the
// exact instant not_after == now (hr2.8). The three sites deliberately use
// different operators, and their interaction at the boundary is the subtle part:
//   - active recall gate:    not_after > now  (Gt)  → at ==now, NOT active (hidden)
//   - ListScheduled expired: not_after <= now (Lte) → at ==now, IS expired (shown)
//   - PruneExpired:          not_after < before (Lt) → at before==not_after, KEPT;
//     one second past the boundary, pruned.
func TestNotAfterBoundaryInstant(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	fixed := time.Date(2031, 6, 1, 12, 0, 0, 0, time.UTC) // epoch-second aligned
	s.now = func() time.Time { return fixed }             // white-box recall clock
	scope := "boundary:project:x"
	subj := Authenticated("sub-bnd")
	notAfter := fixed // record expires AT the comparison instant
	id := "e0000000-0000-0000-0000-000000000001"
	m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-bnd",
		CreatedAt: fixed.Add(-time.Hour), NotAfter: &notAfter}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, subj)) })

	// Active recall gate excludes it: Gt means not_after==now is already past the gate.
	items, _, _, err := s.List(ctx, scope, subj, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, it := range items {
		if it.ID == id {
			t.Error("not_after==now leaked into active recall; the gate (Gt) must hide it at the boundary")
		}
	}

	// ListScheduled(expired) includes it: Lte means not_after==now counts as expired.
	exp, err := s.ListScheduled(ctx, scope, subj, ScheduledExpired, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListScheduled: %v", err)
	}
	foundExpired := false
	for _, it := range exp {
		if it.ID == id {
			foundExpired = true
		}
	}
	if !foundExpired {
		t.Error("not_after==now not surfaced as expired; the expired clause (Lte) must include it at the boundary")
	}

	// PruneExpired keeps it at before==boundary (Lt is strict)...
	n, err := s.PruneExpired(ctx, fixed)
	if err != nil {
		t.Fatalf("prune at boundary: %v", err)
	}
	if n != 0 {
		t.Errorf("PruneExpired(before==not_after) deleted %d, want 0 (Lt is strict)", n)
	}
	if _, err := s.Get(ctx, id); err != nil {
		t.Errorf("record pruned at the boundary instant, want kept: %v", err)
	}

	// ...and removes it one second past the boundary.
	n, err = s.PruneExpired(ctx, fixed.Add(time.Second))
	if err != nil {
		t.Fatalf("prune past boundary: %v", err)
	}
	if n != 1 {
		t.Errorf("PruneExpired(before=not_after+1s) deleted %d, want 1", n)
	}
	if _, err := s.Get(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("record survived one second past the boundary, want pruned: %v", err)
	}
}

// TestPruneExpiredGracePeriod pins the partitioning a past cutoff produces
// (hr2.9): PruneExpired(before) with before in the past spares records that
// lapsed AFTER the cutoff (recently expired, still within the grace window) and
// removes only those that lapsed at or before it. This is the behavior the
// prune-expired --older-than flag yields via pruneCutoff(now, grace).
func TestPruneExpiredGracePeriod(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "prune-grace:project:x"
	subj := Authenticated("sub-A")
	now := time.Now().UTC()
	cutoff := now.Add(-24 * time.Hour) // grace = 24h
	longAgo := now.Add(-48 * time.Hour)
	recently := now.Add(-1 * time.Hour) // expired, but inside the grace window

	mk := func(id string, na time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A", CreatedAt: longAgo, NotAfter: &na}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, subj)) })
	}
	mk("f0000000-0000-0000-0000-000000000001", longAgo)  // lapsed before cutoff -> pruned
	mk("f0000000-0000-0000-0000-000000000002", recently) // lapsed within grace -> kept

	n, err := s.PruneExpired(ctx, cutoff)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 1 {
		t.Errorf("PruneExpired(now-24h grace): deleted %d, want 1 (only the record older than the grace window)", n)
	}
	if _, err := s.Get(ctx, "f0000000-0000-0000-0000-000000000001"); !errors.Is(err, ErrNotFound) {
		t.Errorf("record older than grace should be pruned, got %v", err)
	}
	if _, err := s.Get(ctx, "f0000000-0000-0000-0000-000000000002"); err != nil {
		t.Errorf("recently-expired record (within grace) should survive, got %v", err)
	}
}

func TestPruneExpired(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "prune-test:project:x"
	subj := Authenticated("sub-A")
	now := time.Now().UTC()
	old := now.Add(-48 * time.Hour)
	future := now.Add(48 * time.Hour)

	mk := func(id string, na *time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A", CreatedAt: now, NotAfter: na}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, subj)) })
	}
	mk("c0000000-0000-0000-0000-000000000001", &old)    // expired -> pruned
	mk("c0000000-0000-0000-0000-000000000002", &future) // not expired -> kept
	mk("c0000000-0000-0000-0000-000000000003", nil)     // no window -> kept

	n, err := s.PruneExpired(ctx, now)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 1 {
		t.Errorf("PruneExpired: deleted %d want 1", n)
	}
	if _, err := s.Get(ctx, "c0000000-0000-0000-0000-000000000001"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired record should be gone, got %v", err)
	}
	if _, err := s.Get(ctx, "c0000000-0000-0000-0000-000000000002"); err != nil {
		t.Errorf("future record should survive, got %v", err)
	}
	if _, err := s.Get(ctx, "c0000000-0000-0000-0000-000000000003"); err != nil {
		t.Errorf("unwindowed record (no not_after) should survive, got %v", err)
	}
}

func TestRemapOwner(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:remap"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	mk := func(id, owner string) {
		if err := s.Upsert(ctx, Memory{ID: id, Content: "c", Scope: scope, Owner: owner, CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	subID := "e0e0e0e0-0000-0000-0000-000000000001"
	emailID := "e0e0e0e0-0000-0000-0000-000000000002"
	mk(subID, "old-sub-123")
	mk(emailID, "old@example.com")

	if _, err := s.RemapOwner(ctx, RemapFrom("x"), "", false); err == nil {
		t.Error("empty to: expected error")
	}
	if _, err := s.RemapOwner(ctx, RemapFrom("a"), "a", false); err == nil {
		t.Error("From==to: expected error")
	}
	// Sealing eliminates the old multi-select combination (e.g. the former
	// OwnerRemapSource{Missing: true, From: "x"}) at compile time — there is
	// no way to construct it via RemapMissing()/RemapAnon()/RemapFrom(). The
	// interface's own zero value (nil) is still trivially constructible,
	// though, and RemapOwner's type switch must still reject it explicitly.
	if _, err := s.RemapOwner(ctx, nil, "to", false); err == nil {
		t.Error("nil source: expected error")
	}

	n, err := s.RemapOwner(ctx, RemapFrom("old-sub-123"), "sean@example.com", true)
	if err != nil || n != 1 {
		t.Fatalf("dry-run: n=%d err=%v, want 1,nil", n, err)
	}
	if got, _ := s.Get(ctx, subID); got.Owner != "old-sub-123" {
		t.Errorf("dry-run mutated owner to %q", got.Owner)
	}

	if n, err = s.RemapOwner(ctx, RemapFrom("old-sub-123"), "sean@example.com", false); err != nil || n != 1 {
		t.Fatalf("sub->email: n=%d err=%v", n, err)
	}
	if got, _ := s.Get(ctx, subID); got.Owner != "sean@example.com" {
		t.Errorf("sub->email owner = %q", got.Owner)
	}

	if n, err = s.RemapOwner(ctx, RemapFrom("old@example.com"), "new@example.com", false); err != nil || n != 1 {
		t.Fatalf("email->email: n=%d err=%v", n, err)
	}
	if got, _ := s.Get(ctx, emailID); got.Owner != "new@example.com" {
		t.Errorf("email->email owner = %q", got.Owner)
	}

	// Seed an owner-less (missing key, pre-isolation) record and an explicit
	// anonymous-bucket (owner=="") record to prove the Missing (IsEmpty) vs Anon
	// (Match "") filters target DIFFERENT record sets.
	missingID := "e0e0e0e0-0000-0000-0000-000000000003"
	pm := payload(Memory{ID: missingID, Content: "legacy", Scope: scope, CreatedAt: time.Now().UTC()})
	delete(pm, "owner")
	if _, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: []*qdrant.PointStruct{{
			Id: qdrant.NewID(missingID), Vectors: qdrant.NewVectors(0.1, 0.2, 0.3),
			Payload: qdrant.NewValueMap(pm),
		}},
	}); err != nil {
		t.Fatalf("raw upsert owner-less record: %v", err)
	}
	anonID := "e0e0e0e0-0000-0000-0000-000000000004"
	mk(anonID, "") // explicit owner==""

	// Missing matches ONLY the owner-less record, not the owner=="" bucket.
	if n, err = s.RemapOwner(ctx, RemapMissing(), "backfill@example.com", false); err != nil || n != 1 {
		t.Fatalf("missing->email: n=%d err=%v (want exactly 1; owner=='' must not match IsEmpty)", n, err)
	}
	if got, _ := s.Get(ctx, missingID); got.Owner != "backfill@example.com" {
		t.Errorf("missing->email owner = %q", got.Owner)
	}
	if got, _ := s.Get(ctx, anonID); got.Owner != "" {
		t.Errorf("anon bucket hijacked by Missing remap: owner=%q, want \"\"", got.Owner)
	}

	// Anon matches ONLY the explicit owner=="" record.
	if n, err = s.RemapOwner(ctx, RemapAnon(), "claimed@example.com", false); err != nil || n != 1 {
		t.Fatalf("anon->email: n=%d err=%v (want exactly 1)", n, err)
	}
	if got, _ := s.Get(ctx, anonID); got.Owner != "claimed@example.com" {
		t.Errorf("anon->email owner = %q", got.Owner)
	}
}

func TestRemapFromPanicsOnEmptyValue(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("RemapFrom(\"\") did not panic; an empty From must fail at construction, not silently collapse into a different source")
		}
	}()
	RemapFrom("")
}

// TestRemapOwnerIdempotentRerun verifies re-running an IDENTICAL RemapOwner
// call is a safe no-op: after the first run re-stamps the matching record, the
// From value no longer matches anything, so the rerun counts and mutates
// nothing rather than erroring or double-applying.
func TestRemapOwnerIdempotentRerun(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:remap-idempotent"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	id := "e0e0e0e0-0000-0000-0000-000000000005"
	if err := s.Upsert(ctx, Memory{ID: id, Content: "c", Scope: scope, Owner: "old-sub", CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	src := RemapFrom("old-sub")
	n1, err := s.RemapOwner(ctx, src, "new@example.com", false)
	if err != nil || n1 != 1 {
		t.Fatalf("first remap: n=%d err=%v, want 1,nil", n1, err)
	}

	n2, err := s.RemapOwner(ctx, src, "new@example.com", false)
	if err != nil || n2 != 0 {
		t.Fatalf("rerun: n=%d err=%v, want 0,nil (idempotent no-op)", n2, err)
	}
	if got, _ := s.Get(ctx, id); got.Owner != "new@example.com" {
		t.Errorf("owner drifted on rerun: %q, want unchanged from first remap", got.Owner)
	}
}

// TestMigrateSetOwnerEquivalentToRemapOwnerMissing pins the "deprecated alias"
// claim in migrateSetOwnerCmd.Deprecated ("use: migrate-remap-owner
// --from-missing --to <owner>"): MigrateSetOwner and
// RemapOwner(RemapMissing(), ...) are two independent store-layer
// implementations, so this regression-tests that they produce identical
// results against identical fixtures rather than assuming the doc comment
// stays true by construction.
func TestMigrateSetOwnerEquivalentToRemapOwnerMissing(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:migrate-alias"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	seedOwnerless := func(id string) {
		p := payload(Memory{ID: id, Content: "legacy", Scope: scope, CreatedAt: time.Now().UTC()})
		delete(p, "owner")
		if _, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: s.collection, Wait: qdrant.PtrOf(true),
			Points: []*qdrant.PointStruct{{
				Id: qdrant.NewID(id), Vectors: qdrant.NewVectors(0.1, 0.2, 0.3),
				Payload: qdrant.NewValueMap(p),
			}},
		}); err != nil {
			t.Fatalf("raw upsert owner-less record %s: %v", id, err)
		}
	}

	// Both sweeps match owner-less records across the WHOLE collection (no
	// scope filter — see ownerlessFilter), so the two fixtures cannot coexist:
	// each is seeded, swept, and verified before the next is created.
	viaMigrateID := "e0e0e0e0-0000-0000-0000-000000000006"
	seedOwnerless(viaMigrateID)
	nMigrate, err := s.MigrateSetOwner(ctx, "backfill@example.com")
	if err != nil {
		t.Fatalf("MigrateSetOwner: %v", err)
	}
	if nMigrate != 1 {
		t.Fatalf("MigrateSetOwner stamped %d, want exactly 1", nMigrate)
	}
	gotMigrate, err := s.Get(ctx, viaMigrateID)
	if err != nil {
		t.Fatalf("Get viaMigrate: %v", err)
	}

	viaRemapID := "e0e0e0e0-0000-0000-0000-000000000007"
	seedOwnerless(viaRemapID)
	nRemap, err := s.RemapOwner(ctx, RemapMissing(), "backfill@example.com", false)
	if err != nil {
		t.Fatalf("RemapOwner(RemapMissing()): %v", err)
	}
	if nRemap != 1 {
		t.Fatalf("RemapOwner(RemapMissing()) stamped %d, want exactly 1", nRemap)
	}
	gotRemap, err := s.Get(ctx, viaRemapID)
	if err != nil {
		t.Fatalf("Get viaRemap: %v", err)
	}

	if nMigrate != nRemap {
		t.Errorf("migrate-set-owner stamped %d, migrate-remap-owner --from-missing stamped %d, want equal", nMigrate, nRemap)
	}
	if gotMigrate.Owner != gotRemap.Owner {
		t.Errorf("owner mismatch: migrate-set-owner=%q, migrate-remap-owner --from-missing=%q", gotMigrate.Owner, gotRemap.Owner)
	}
}

func TestRemapOwnerHonorsCancel(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.RemapOwner(ctx, RemapFrom("x"), "y", false); err == nil {
		t.Error("cancelled context: expected error")
	}
}

func TestPayloadRoundTripSummaryProvenance(t *testing.T) {
	m := Memory{
		ID: "11111111-1111-1111-1111-111111111111", Content: "long original content",
		Scope: "repo:x", Category: "convention", Source: "agent-inferred",
		Summary: "terse line", SummarySource: SummarySourceAuto, SummaryModel: "summary-cheap",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	vm := qdrant.NewValueMap(payload(m))
	got := fromPayload(m.ID, vm)
	if got.Summary != "terse line" || got.SummarySource != SummarySourceAuto || got.SummaryModel != "summary-cheap" {
		t.Fatalf("summary provenance not round-tripped: %+v", got)
	}

	// A curated record with no summary must still round-trip cleanly (empty source).
	plain := Memory{ID: "22222222-2222-2222-2222-222222222222", Content: "c", Category: "gotcha"}
	g2 := fromPayload(plain.ID, qdrant.NewValueMap(payload(plain)))
	if g2.Summary != "" || g2.SummarySource != SummarySourceNone || g2.SummaryModel != "" {
		t.Fatalf("empty-summary record drifted: %+v", g2)
	}
}

func TestPayloadRoundTripsShortID(t *testing.T) {
	m := Memory{ID: "a0000000-0000-0000-0000-000000000001", ShortID: "j7k2m9p4x0", Content: "c", Scope: "s"}
	got := fromPayload(m.ID, qdrant.NewValueMap(payload(m)))
	if got.ShortID != "j7k2m9p4x0" {
		t.Fatalf("round-trip short_id = %q", got.ShortID)
	}
	// Empty ShortID MUST be omitted (not stamped as an explicit ""), so legacy /
	// reindexed records stay key-absent and the NewIsEmpty backfill filter matches them.
	if _, ok := payload(Memory{ID: "x"})["short_id"]; ok {
		t.Fatal("empty ShortID must be omitted from payload")
	}
}

// TestPayloadRoundTripsEmbedderIdentity pins D-05/D-06: EmbedderIdentity
// round-trips through payload()/fromPayload() under the shared
// embedderIdentityKey despite its `json:"-"` tag (the manual codec is the
// ONLY persistence path for this field), and a legacy payload missing the
// key decodes to the zero value with no backfill.
func TestPayloadRoundTripsEmbedderIdentity(t *testing.T) {
	m := Memory{ID: "a0000000-0000-0000-0000-000000000002", Content: "c", Scope: "s", EmbedderIdentity: "v1:deadbeefdeadbeef"}
	got := fromPayload(m.ID, qdrant.NewValueMap(payload(m)))
	if got.EmbedderIdentity != "v1:deadbeefdeadbeef" {
		t.Fatalf("round-trip embedder_identity = %q, want %q", got.EmbedderIdentity, "v1:deadbeefdeadbeef")
	}

	// A payload map missing the key (legacy record) must decode to "", not panic.
	legacy := map[string]*qdrant.Value{
		"content": qdrant.NewValueString("c"),
	}
	gotLegacy := fromPayload("legacy-id", legacy)
	if gotLegacy.EmbedderIdentity != "" {
		t.Fatalf("legacy payload missing embedder_identity must decode to \"\", got %q", gotLegacy.EmbedderIdentity)
	}
}

// TestPayloadRoundTripsIdempotencyFingerprint pins Phase 24 D-06:
// IdempotencyFingerprint round-trips through payload()/fromPayload() under the
// shared idempotencyFingerprintKey despite its `json:"-"` tag (the manual
// codec is the ONLY persistence path for this field), and a legacy payload
// missing the key decodes to the zero value with no backfill — the exact
// mirror of TestPayloadRoundTripsEmbedderIdentity above.
func TestPayloadRoundTripsIdempotencyFingerprint(t *testing.T) {
	m := Memory{ID: "a0000000-0000-0000-0000-000000000003", Content: "c", Scope: "s", IdempotencyFingerprint: "deadbeefdeadbeef"}
	got := fromPayload(m.ID, qdrant.NewValueMap(payload(m)))
	if got.IdempotencyFingerprint != "deadbeefdeadbeef" {
		t.Fatalf("round-trip idempotency_fingerprint = %q, want %q", got.IdempotencyFingerprint, "deadbeefdeadbeef")
	}

	// A payload map missing the key (legacy record) must decode to "", not panic.
	legacy := map[string]*qdrant.Value{
		"content": qdrant.NewValueString("c"),
	}
	gotLegacy := fromPayload("legacy-id", legacy)
	if gotLegacy.IdempotencyFingerprint != "" {
		t.Fatalf("legacy payload missing idempotency_fingerprint must decode to \"\", got %q", gotLegacy.IdempotencyFingerprint)
	}
}

// TestPayloadRoundTripsSchemaVersion pins D-05/D-09/D-10: the monotonic
// stamp, the absent-safe decode, the always-present payload key, the
// idempotent ordering, and the exact struct tag. Every expected value is
// derived from migrate.CurrentVersion, never a hard-coded literal, so this
// test keeps its meaning when a later phase raises the constant.
func TestPayloadRoundTripsSchemaVersion(t *testing.T) {
	cases := []struct {
		name string
		m    Memory
		want migrate.Version
	}{
		// A newer record's version survives an older binary's rewrite
		// undowngraded — the monotonic rule's whole reason to exist.
		{"above", Memory{ID: "a0000000-0000-0000-0000-000000000004", Content: "c", Scope: "s", SchemaVersion: migrate.CurrentVersion + 3}, migrate.CurrentVersion + 3},
		// An exact tie is a no-op: neither incremented nor lowered.
		{"equal", Memory{ID: "a0000000-0000-0000-0000-000000000005", Content: "c", Scope: "s", SchemaVersion: migrate.CurrentVersion}, migrate.CurrentVersion},
		// The zero-valued Memory{} — the empty-input edge — still stamps
		// current, and the key is never omitted.
		{"zero", Memory{}, migrate.CurrentVersion},
		// An OLDER record read by a NEWER binary is RAISED to current — the
		// arm the whole migration sweep depends on. Constructed one below
		// CurrentVersion rather than at a literal, so this case keeps
		// exercising the "below" branch whatever the constant's current
		// value is. At a low enough CurrentVersion, CurrentVersion-1 is a
		// synthetic value outside the normal non-negative range — which is
		// precisely why the assertion is written against the max() rule and
		// not against a fixed number.
		{"below", Memory{ID: "a0000000-0000-0000-0000-000000000006", Content: "c", Scope: "s", SchemaVersion: migrate.CurrentVersion - 1}, migrate.CurrentVersion},
	}
	// Derived, not enumerated: a hard-coded count silently stays "correct"
	// while a case goes missing. Assert on the case NAMES so a dropped or
	// renamed arm fails loudly instead of passing on cardinality alone.
	wantNames := []string{"above", "equal", "zero", "below"}
	gotNames := make([]string, 0, len(cases))
	for _, tc := range cases {
		gotNames = append(gotNames, tc.name)
	}
	if !slices.Equal(gotNames, wantNames) {
		t.Fatalf("case set = %v, want %v — every arm of the monotonic rule must stay covered", gotNames, wantNames)
	}
	ran := 0
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ran++
			p1 := payload(tc.m)
			v, ok := p1["schema_version"]
			if !ok {
				t.Fatal("schema_version key missing from payload() output")
			}
			if got := migrate.Version(v.(int)); got != tc.want {
				t.Fatalf("payload()[schema_version] = %d, want %d", got, tc.want)
			}

			got1 := fromPayload(tc.m.ID, qdrant.NewValueMap(p1))
			if got1.SchemaVersion != tc.want {
				t.Fatalf("fromPayload(payload(m)).SchemaVersion = %d, want %d", got1.SchemaVersion, tc.want)
			}

			// Idempotence: payload -> fromPayload -> payload again yields the
			// identical schema_version value — repeated rewrites in any
			// order converge and never oscillate.
			p2 := payload(got1)
			v2, ok := p2["schema_version"]
			if !ok {
				t.Fatal("schema_version key missing from second payload() output")
			}
			if got2 := migrate.Version(v2.(int)); got2 != tc.want {
				t.Fatalf("second payload()[schema_version] = %d, want %d (idempotence broken)", got2, tc.want)
			}
		})
	}
	// Derived from the table, not restated as a literal: a second hard-coded
	// count is a second place to forget when an arm is added.
	if ran != len(cases) {
		t.Fatalf("subtests executed = %d, want %d (one per case)", ran, len(cases))
	}

	// Legacy decode: a payload map with no schema_version key at all
	// decodes to migrate.Version(0) with no panic.
	legacy := map[string]*qdrant.Value{"content": qdrant.NewValueString("c")}
	gotLegacy := fromPayload("legacy-id", legacy)
	if gotLegacy.SchemaVersion != migrate.Version(0) {
		t.Fatalf("legacy payload missing schema_version must decode to 0, got %d", gotLegacy.SchemaVersion)
	}

	// Struct tag: the exact json tag, no omitempty, no hidden rename.
	field, ok := reflect.TypeOf(Memory{}).FieldByName("SchemaVersion")
	if !ok {
		t.Fatal("Memory has no SchemaVersion field")
	}
	if tag := field.Tag.Get("json"); tag != "schema_version" {
		t.Fatalf("Memory.SchemaVersion json tag = %q, want exactly %q", tag, "schema_version")
	}
}

// TestEnsureCollectionCreatesIndexes pins that EnsureCollection provisions the
// owner/scope/created_at payload indexes and is idempotent on a second call.
func TestEnsureCollectionCreatesIndexes(t *testing.T) {
	s := testStore(t) // testStore already calls EnsureCollection(ctx, 3) once
	ctx := context.Background()

	info, err := s.client.GetCollectionInfo(ctx, s.collection)
	if err != nil {
		t.Fatalf("GetCollectionInfo: %v", err)
	}
	schema := info.GetPayloadSchema()
	for _, field := range []string{"owner", "scope", "created_at"} {
		if _, ok := schema[field]; !ok {
			t.Errorf("payload index missing for %q; have %v", field, keysOf(schema))
		}
	}

	// Idempotent: a second EnsureCollection must not error on existing indexes.
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("second EnsureCollection: %v", err)
	}
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// recordIDs extracts ids in slice order — a shared id-extraction helper for
// table assertions. The existing TestSearchAndListTagsFilter defines a local
// `ids` closure; there is no package-level `ids`, so this lives here.
func recordIDs(ms []Memory) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return out
}

// TestListDateWindow pins half-open [after, before): a record AT created_after is
// included (gte); a record AT created_before is excluded (lt).
func TestListDateWindow(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "win-test:project:list"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	t0 := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	mk := func(id string, at time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A", CreatedAt: at}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert %s: %v", id, err)
		}
	}
	mk("a0000000-0000-0000-0000-000000000001", t0.Add(-time.Hour))  // before window
	mk("a0000000-0000-0000-0000-000000000002", t0)                  // == after  -> included
	mk("a0000000-0000-0000-0000-000000000003", t0.Add(time.Hour))   // inside
	mk("a0000000-0000-0000-0000-000000000004", t0.Add(2*time.Hour)) // == before -> excluded

	subj := Authenticated("sub-A")
	items, total, _, err := s.List(ctx, scope, subj, ListOptions{
		Limit: 10, CreatedAfter: t0, CreatedBefore: t0.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := recordIDs(items)
	want := []string{
		"a0000000-0000-0000-0000-000000000003",
		"a0000000-0000-0000-0000-000000000002",
	} // CreatedAt desc
	if !slices.Equal(got, want) {
		t.Errorf("window: got %v want %v", got, want)
	}
	if total != 2 {
		t.Errorf("window total: got %d want 2", total)
	}
}

// TestSearchDateWindow pins the same half-open window as a Search pre-filter.
func TestSearchDateWindow(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "win-test:project:search"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	t0 := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	mk := func(id string, at time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A", CreatedAt: at}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert %s: %v", id, err)
		}
	}
	mk("b0000000-0000-0000-0000-000000000001", t0.Add(-time.Hour))
	mk("b0000000-0000-0000-0000-000000000002", t0.Add(time.Hour))

	hits, err := s.Search(ctx, scope, Authenticated("sub-A"),
		[]float32{0.1, 0.2, 0.3}, 10, SearchOptions{CreatedAfter: t0})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if got := recordIDs(hits); !slices.Equal(got, []string{"b0000000-0000-0000-0000-000000000002"}) {
		t.Errorf("search window: got %v want [..002]", got)
	}
}

// TestSupersedeRecallGate pins the superseded_by IS EMPTY soft-hide gate at
// BOTH recall call sites (Search and List, mirroring TestListDateWindow /
// TestSearchDateWindow's dual-site pairing) — a record whose SupersededBy is
// non-empty must be absent from both, yet still fetchable via Get with its
// content intact. It also pins the Supersedes/SupersededBy payload codec
// round-trip: non-nil pointers survive a Get, and nil pointers stay nil (no
// panic on absent payload keys).
func TestSupersedeRecallGate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-test:project:recall-gate"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	supersededID := "c0000000-0000-0000-0000-000000000001"
	liveID := "c0000000-0000-0000-0000-000000000002"
	newID := "c0000000-0000-0000-0000-000000000003"

	superseded := Memory{
		ID: supersededID, Content: "old content", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), SupersededBy: &newID,
	}
	if err := s.Upsert(ctx, superseded, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert superseded: %v", err)
	}
	live := Memory{
		ID: liveID, Content: "live content", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, live, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert live: %v", err)
	}

	subj := Authenticated("sub-A")

	// Search: superseded record excluded, live record present.
	hits, err := s.Search(ctx, scope, subj, []float32{0.1, 0.2, 0.3}, 10, SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if got := recordIDs(hits); slices.Contains(got, supersededID) {
		t.Errorf("Search: superseded record %s present, want excluded: %v", supersededID, got)
	}
	if got := recordIDs(hits); !slices.Contains(got, liveID) {
		t.Errorf("Search: live record %s absent, want present: %v", liveID, got)
	}

	// List: same exclusion/inclusion pairing.
	items, _, _, err := s.List(ctx, scope, subj, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := recordIDs(items); slices.Contains(got, supersededID) {
		t.Errorf("List: superseded record %s present, want excluded: %v", supersededID, got)
	}
	if got := recordIDs(items); !slices.Contains(got, liveID) {
		t.Errorf("List: live record %s absent, want present: %v", liveID, got)
	}

	// Get: superseded record still fetchable, content intact.
	got, err := s.Get(ctx, supersededID)
	if err != nil {
		t.Fatalf("Get superseded: %v", err)
	}
	if got.Content != "old content" {
		t.Errorf("Get superseded: content = %q, want %q (should be untouched)", got.Content, "old content")
	}
	if got.SupersededBy == nil || *got.SupersededBy != newID {
		t.Errorf("Get superseded: SupersededBy = %v, want %q", got.SupersededBy, newID)
	}

	// Codec round-trip: nil pointers stay nil on the live record.
	gotLive, err := s.Get(ctx, liveID)
	if err != nil {
		t.Fatalf("Get live: %v", err)
	}
	if gotLive.Supersedes != nil {
		t.Errorf("Get live: Supersedes = %v, want nil", gotLive.Supersedes)
	}
	if gotLive.SupersededBy != nil {
		t.Errorf("Get live: SupersededBy = %v, want nil", gotLive.SupersededBy)
	}

	// Codec round-trip: a record with Supersedes set (forward link) survives.
	newRec := Memory{
		ID: newID, Content: "new content", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{supersededID},
	}
	if err := s.Upsert(ctx, newRec, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert new: %v", err)
	}
	gotNew, err := s.Get(ctx, newID)
	if err != nil {
		t.Fatalf("Get new: %v", err)
	}
	if len(gotNew.Supersedes) != 1 || gotNew.Supersedes[0] != supersededID {
		t.Errorf("Get new: Supersedes = %v, want [%q]", gotNew.Supersedes, supersededID)
	}
	if gotNew.SupersededBy != nil {
		t.Errorf("Get new: SupersededBy = %v, want nil", gotNew.SupersededBy)
	}
}

// TestSupersedeStamp (SC1) pins Store.Supersede's core contract: the target's
// SupersededBy is back-stamped to the new record's id via a single-key
// SetPayload, and the target's other payload fields (Content/Tags/Visibility)
// survive untouched — the proof that the back-stamp is a merge, not a
// re-Upsert (D-01). The new record carries Supersedes == target id (D-04).
func TestSupersedeStamp(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-test:project:stamp"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	targetID := "d0000000-0000-0000-0000-000000000001"
	newID := "d0000000-0000-0000-0000-000000000002"

	target := Memory{
		ID: targetID, Content: "original content", Scope: scope, Owner: "sub-A",
		Tags: []string{"t1"}, Visibility: "shared", CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, target, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert target: %v", err)
	}

	newMem := Memory{
		ID: newID, Content: "corrected content", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{targetID},
	}
	if err := s.Supersede(ctx, newMem, []float32{0.4, 0.5, 0.6}, []string{targetID}, Authenticated("sub-A")); err != nil {
		t.Fatalf("Supersede: %v", err)
	}

	gotTarget, err := s.Get(ctx, targetID)
	if err != nil {
		t.Fatalf("Get target: %v", err)
	}
	if gotTarget.SupersededBy == nil || *gotTarget.SupersededBy != newID {
		t.Errorf("target.SupersededBy = %v, want %q", gotTarget.SupersededBy, newID)
	}
	if gotTarget.Content != "original content" {
		t.Errorf("target.Content = %q, want unchanged %q", gotTarget.Content, "original content")
	}
	if !slices.Equal(gotTarget.Tags, []string{"t1"}) {
		t.Errorf("target.Tags = %v, want unchanged [t1]", gotTarget.Tags)
	}
	if gotTarget.Visibility != "shared" {
		t.Errorf("target.Visibility = %q, want unchanged %q", gotTarget.Visibility, "shared")
	}

	gotNew, err := s.Get(ctx, newID)
	if err != nil {
		t.Fatalf("Get new: %v", err)
	}
	if gotNew.Content != "corrected content" {
		t.Errorf("new.Content = %q, want %q", gotNew.Content, "corrected content")
	}
	if len(gotNew.Supersedes) != 1 || gotNew.Supersedes[0] != targetID {
		t.Errorf("new.Supersedes = %v, want [%q]", gotNew.Supersedes, targetID)
	}
}

// TestSupersedeVectorPreserved (IN-01) directly asserts the target's stored
// VECTOR survives Supersede byte-identical — closing the gap between
// Supersede's doc comment claim ("single-key SetPayload, vector-preserving")
// and TestSupersedeStamp, which only asserts payload fields. Store.Get omits
// vectors (WithPayload only), so this reads the raw Qdrant point directly
// with WithVectors, before and after, mirroring reindex_test.go's
// scrollPoints helper.
func TestSupersedeVectorPreserved(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-test:project:vector-preserved"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	targetID := "d6000000-0000-0000-0000-000000000001"
	newID := "d6000000-0000-0000-0000-000000000002"
	targetVec := []float32{0.11, 0.22, 0.33}

	target := Memory{ID: targetID, Content: "v", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, target, targetVec); err != nil {
		t.Fatalf("upsert target: %v", err)
	}

	rawVector := func(id string) []float32 {
		t.Helper()
		pts, err := s.client.Get(ctx, &qdrant.GetPoints{
			CollectionName: s.collection, Ids: []*qdrant.PointId{qdrant.NewID(id)},
			WithVectors: qdrant.NewWithVectors(true),
		})
		if err != nil {
			t.Fatalf("raw get %s: %v", id, err)
		}
		if len(pts) != 1 {
			t.Fatalf("raw get %s: got %d points, want 1", id, len(pts))
		}
		return pts[0].GetVectors().GetVector().GetDense().GetData()
	}

	// Qdrant normalizes vectors on insert for Cosine-distance collections, so
	// `before` is the normalized form of targetVec, not targetVec itself —
	// only its non-zero length is asserted here; the real assertion is
	// before == after below.
	before := rawVector(targetID)
	if len(before) == 0 {
		t.Fatalf("target vector before Supersede is empty, want a %d-dim vector", len(targetVec))
	}

	newMem := Memory{
		ID: newID, Content: "corrected", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{targetID},
	}
	if err := s.Supersede(ctx, newMem, []float32{0.9, 0.8, 0.7}, []string{targetID}, Authenticated("sub-A")); err != nil {
		t.Fatalf("Supersede: %v", err)
	}

	after := rawVector(targetID)
	if !slices.Equal(after, before) {
		t.Errorf("target vector after Supersede = %v, want unchanged %v", after, before)
	}
}

// TestSupersedeOwnerGate (SC3) pins that a non-owner cannot supersede a
// target they don't own — Supersede must use getWritable/ActionWrite (never
// GetReadable), so a caller without a write grant gets the same ErrNotFound
// as "doesn't exist" (no existence leak), and the target is left unstamped.
func TestSupersedeOwnerGate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-test:project:owner-gate"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	targetID := "d1000000-0000-0000-0000-000000000001"
	newID := "d1000000-0000-0000-0000-000000000002"

	target := Memory{ID: targetID, Content: "v", Scope: scope, Owner: "sub-B", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, target, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert target: %v", err)
	}

	newMem := Memory{
		ID: newID, Content: "corrected", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{targetID},
	}
	if err := s.Supersede(ctx, newMem, []float32{0.4, 0.5, 0.6}, []string{targetID}, Authenticated("sub-A")); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-owner Supersede: want ErrNotFound, got %v", err)
	}

	gotTarget, err := s.Get(ctx, targetID)
	if err != nil {
		t.Fatalf("Get target: %v", err)
	}
	if gotTarget.SupersededBy != nil {
		t.Errorf("target.SupersededBy = %v, want nil (unauthorized supersede must not stamp)", gotTarget.SupersededBy)
	}
}

// TestSupersedeAlreadySuperseded (SC4/D-05) pins the single-hop guard:
// superseding a target whose SupersededBy is already non-empty is rejected
// with store.ErrAlreadySuperseded, keeping a single live head per chain.
func TestSupersedeAlreadySuperseded(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-test:project:already-superseded"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")

	targetID := "d2000000-0000-0000-0000-000000000001"
	firstNewID := "d2000000-0000-0000-0000-000000000002"
	secondNewID := "d2000000-0000-0000-0000-000000000003"

	target := Memory{ID: targetID, Content: "v", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, target, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert target: %v", err)
	}
	firstNew := Memory{
		ID: firstNewID, Content: "first correction", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{targetID},
	}
	if err := s.Supersede(ctx, firstNew, []float32{0.2, 0.3, 0.4}, []string{targetID}, subj); err != nil {
		t.Fatalf("first Supersede: %v", err)
	}

	secondNew := Memory{
		ID: secondNewID, Content: "second correction", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{targetID},
	}
	if err := s.Supersede(ctx, secondNew, []float32{0.3, 0.4, 0.5}, []string{targetID}, subj); !errors.Is(err, ErrAlreadySuperseded) {
		t.Errorf("second Supersede on already-superseded target: want ErrAlreadySuperseded, got %v", err)
	}
}

// TestSupersedeConcurrent (CR-01) pins Store.Supersede's per-target lock: two
// goroutines racing to supersede the SAME target must not both succeed. Before
// the fix, both could observe SupersededBy == nil before either back-stamped,
// silently forking the correction chain with no error to either caller. Run
// with -race — the lock must also be free of data races on the shared
// sync.Map-backed *sync.Mutex.
func TestSupersedeConcurrent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-test:project:concurrent"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")

	targetID := "d5000000-0000-0000-0000-000000000001"
	firstNewID := "d5000000-0000-0000-0000-000000000002"
	secondNewID := "d5000000-0000-0000-0000-000000000003"

	target := Memory{ID: targetID, Content: "v", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, target, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert target: %v", err)
	}

	firstNew := Memory{
		ID: firstNewID, Content: "first correction", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{targetID},
	}
	secondNew := Memory{
		ID: secondNewID, Content: "second correction", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{targetID},
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs[0] = s.Supersede(ctx, firstNew, []float32{0.2, 0.3, 0.4}, []string{targetID}, subj)
	}()
	go func() {
		defer wg.Done()
		errs[1] = s.Supersede(ctx, secondNew, []float32{0.3, 0.4, 0.5}, []string{targetID}, subj)
	}()
	wg.Wait()

	successes, conflicts := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrAlreadySuperseded):
			conflicts++
		default:
			t.Fatalf("unexpected Supersede error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent Supersede: got %d successes, %d ErrAlreadySuperseded conflicts (errs=%v), want exactly 1 success and 1 conflict", successes, conflicts, errs)
	}

	gotTarget, err := s.Get(ctx, targetID)
	if err != nil {
		t.Fatalf("Get target: %v", err)
	}
	if gotTarget.SupersededBy == nil || *gotTarget.SupersededBy == "" {
		t.Fatalf("target.SupersededBy = %v, want set to the single winning correction's id", gotTarget.SupersededBy)
	}
	if *gotTarget.SupersededBy != firstNewID && *gotTarget.SupersededBy != secondNewID {
		t.Fatalf("target.SupersededBy = %q, want one of %q/%q", *gotTarget.SupersededBy, firstNewID, secondNewID)
	}
}

// TestSupersedeVsUpdateConcurrent (CR-04) pins that Store.Update's
// whole-payload Upsert can no longer erase a concurrent Store.Supersede's
// superseded_by back-stamp. Before the fix, Update re-Upserted a
// FetchForUpdate snapshot taken BEFORE a racing Supersede landed its
// back-stamp, silently reverting it (empirically confirmed against real
// Qdrant: Get(target).SupersededBy went set->nil after the racing Update).
// The fix makes Update take the SAME per-target lock Supersede uses and
// re-read superseded_by/supersedes inside that lock before writing, so the
// back-stamp survives regardless of which of the two racing calls wins the
// lock first. Run with -race — the lock must also be free of data races.
func TestSupersedeVsUpdateConcurrent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-test:project:vs-update-concurrent"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")

	targetID := "d6000000-0000-0000-0000-000000000001"
	newID := "d6000000-0000-0000-0000-000000000002"

	target := Memory{ID: targetID, Content: "v1", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, target, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert target: %v", err)
	}

	// The stale snapshot: fetched BEFORE the racing Supersede below lands its
	// back-stamp, mirroring updateMemory's handler-layer FetchForUpdate call
	// that happens before its network-bound re-embed.
	cur, err := s.FetchForUpdate(ctx, targetID, subj)
	if err != nil {
		t.Fatalf("FetchForUpdate: %v", err)
	}
	if cur.SupersededBy != nil {
		t.Fatalf("precondition: cur.SupersededBy = %v, want nil before the race", cur.SupersededBy)
	}

	newMem := Memory{
		ID: newID, Content: "corrected", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{targetID},
	}

	var wg sync.WaitGroup
	var supersedeErr, updateErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		supersedeErr = s.Supersede(ctx, newMem, []float32{0.4, 0.5, 0.6}, []string{targetID}, subj)
	}()
	go func() {
		defer wg.Done()
		updateErr = s.Update(ctx, cur, "v2 content edit", nil, nil, nil, []float32{0.2, 0.3, 0.4})
	}()
	wg.Wait()

	if supersedeErr != nil {
		t.Fatalf("Supersede: %v", supersedeErr)
	}
	if updateErr != nil {
		t.Fatalf("Update: %v", updateErr)
	}

	gotTarget, err := s.Get(ctx, targetID)
	if err != nil {
		t.Fatalf("Get target: %v", err)
	}
	if gotTarget.SupersededBy == nil || *gotTarget.SupersededBy != newID {
		t.Fatalf("target.SupersededBy = %v, want %q (Update must not erase a concurrent Supersede's back-stamp)", gotTarget.SupersededBy, newID)
	}
	if gotTarget.Content != "v2 content edit" {
		t.Errorf("target.Content = %q, want %q (Update's content edit must still land)", gotTarget.Content, "v2 content edit")
	}
}

// TestSupersedeForwardChain (D-06) pins that forward chains are allowed: C
// supersedes A which superseded B. All three remain fetchable by id; only A
// and B (non-head records) are excluded from Search/List, C (the live head)
// stays present.
func TestSupersedeForwardChain(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-test:project:forward-chain"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")
	vec := []float32{0.1, 0.2, 0.3}

	bID := "d3000000-0000-0000-0000-000000000001"
	aID := "d3000000-0000-0000-0000-000000000002"
	cID := "d3000000-0000-0000-0000-000000000003"

	b := Memory{ID: bID, Content: "b", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, b, vec); err != nil {
		t.Fatalf("upsert b: %v", err)
	}
	a := Memory{ID: aID, Content: "a", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC(), Supersedes: []string{bID}}
	if err := s.Supersede(ctx, a, vec, []string{bID}, subj); err != nil {
		t.Fatalf("Supersede b->a: %v", err)
	}
	c := Memory{ID: cID, Content: "c", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC(), Supersedes: []string{aID}}
	if err := s.Supersede(ctx, c, vec, []string{aID}, subj); err != nil {
		t.Fatalf("Supersede a->c: %v", err)
	}

	for _, id := range []string{bID, aID, cID} {
		if _, err := s.Get(ctx, id); err != nil {
			t.Errorf("Get %s: %v", id, err)
		}
	}

	items, _, _, err := s.List(ctx, scope, subj, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := recordIDs(items); slices.Contains(got, bID) || slices.Contains(got, aID) {
		t.Errorf("List: superseded records present, want excluded: %v", got)
	} else if !slices.Contains(got, cID) {
		t.Errorf("List: C %s absent, want present (head): %v", cID, got)
	}

	hits, err := s.Search(ctx, scope, subj, vec, 10, SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if got := recordIDs(hits); slices.Contains(got, bID) || slices.Contains(got, aID) {
		t.Errorf("Search: superseded records present, want excluded: %v", got)
	} else if !slices.Contains(got, cID) {
		t.Errorf("Search: C %s absent, want present (head): %v", cID, got)
	}
}

// TestSupersedeMultiAlreadySuperseded (plan 03.1-02 Task 3, REQ-merge-
// supersession) generalizes TestSupersedeAlreadySuperseded to the multi-
// target rejection: naming an already-superseded target among a larger set
// is rejected, and — the part the single-target test never needed —
// naming it must leave no survivor behind (D-05: the preflight completes
// for every target before any write) and must name EVERY offender, not
// just the first.
func TestSupersedeMultiAlreadySuperseded(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-multi-test:project:already-superseded"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")

	target1 := "f4000000-0000-0000-0000-000000000001"
	target2 := "f4000000-0000-0000-0000-000000000002"
	target3 := "f4000000-0000-0000-0000-000000000003"
	preCorrection1ID := "f4000000-0000-0000-0000-000000000004"
	preCorrection2ID := "f4000000-0000-0000-0000-000000000005"
	mergeAttempt1ID := "f4000000-0000-0000-0000-000000000006"
	mergeAttempt2ID := "f4000000-0000-0000-0000-000000000007"

	for _, id := range []string{target1, target2, target3} {
		m := Memory{ID: id, Content: "v", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}

	// Single-offender case: target1 already superseded on its own first.
	preCorrection1 := Memory{
		ID: preCorrection1ID, Content: "pre-correction 1", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{target1},
	}
	if err := s.Supersede(ctx, preCorrection1, []float32{0.2, 0.3, 0.4}, []string{target1}, subj); err != nil {
		t.Fatalf("pre-correct target1: %v", err)
	}

	mergeAttempt1 := Memory{
		ID: mergeAttempt1ID, Content: "merge attempt 1", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{target1, target2, target3},
	}
	err := s.Supersede(ctx, mergeAttempt1, []float32{0.3, 0.4, 0.5}, []string{target1, target2, target3}, subj)
	if !errors.Is(err, ErrAlreadySuperseded) {
		t.Fatalf("merge attempt 1 err = %v, want ErrAlreadySuperseded", err)
	}
	var multiErr1 *MultiTargetError
	if !errors.As(err, &multiErr1) {
		t.Fatalf("errors.As(err, *MultiTargetError) failed on err = %v", err)
	}
	if !slices.Equal(multiErr1.IDs, []string{target1}) {
		t.Errorf("multiErr1.IDs = %v, want [%q] (single offender named)", multiErr1.IDs, target1)
	}
	if _, gerr := s.Get(ctx, mergeAttempt1ID); !errors.Is(gerr, ErrNotFound) {
		t.Errorf("Get merge-attempt-1 survivor: err = %v, want ErrNotFound (a rejected merge must leave no new record behind — D-05)", gerr)
	}

	// Two-offender case: target2 ALSO pre-corrected on its own — want both
	// already-superseded ids named, not just the first encountered.
	preCorrection2 := Memory{
		ID: preCorrection2ID, Content: "pre-correction 2", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{target2},
	}
	if err := s.Supersede(ctx, preCorrection2, []float32{0.3, 0.4, 0.5}, []string{target2}, subj); err != nil {
		t.Fatalf("pre-correct target2: %v", err)
	}

	mergeAttempt2 := Memory{
		ID: mergeAttempt2ID, Content: "merge attempt 2", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{target1, target2, target3},
	}
	err = s.Supersede(ctx, mergeAttempt2, []float32{0.4, 0.5, 0.6}, []string{target1, target2, target3}, subj)
	if !errors.Is(err, ErrAlreadySuperseded) {
		t.Fatalf("merge attempt 2 err = %v, want ErrAlreadySuperseded", err)
	}
	// Asserts through the TYPED error, not the rendered message: errors.As
	// yields *MultiTargetError and its IDs field holds both offending
	// canonical ids — want both already-superseded ids named.
	var multiErr2 *MultiTargetError
	if !errors.As(err, &multiErr2) {
		t.Fatalf("errors.As(err, *MultiTargetError) failed on err = %v", err)
	}
	gotIDs := slices.Clone(multiErr2.IDs)
	slices.Sort(gotIDs)
	wantIDs := []string{target1, target2}
	slices.Sort(wantIDs)
	if !slices.Equal(gotIDs, wantIDs) {
		t.Errorf("multiErr2.IDs = %v, want both offenders %v (want both already-superseded ids named, not just the first)", gotIDs, wantIDs)
	}
	// Secondary check on the rendered text, kept as a belt-and-suspenders
	// assertion alongside the typed-error check above.
	if !strings.Contains(err.Error(), target1) || !strings.Contains(err.Error(), target2) {
		t.Errorf("err.Error() = %q, want both already-superseded ids named", err.Error())
	}
	if _, gerr := s.Get(ctx, mergeAttempt2ID); !errors.Is(gerr, ErrNotFound) {
		t.Errorf("Get merge-attempt-2 survivor: err = %v, want ErrNotFound (a rejected merge must leave no new record behind — D-05)", gerr)
	}
}

// TestSupersedeMultiRecallGate (plan 03.1-02 Task 3) generalizes
// TestSupersedeRecallGate to N=3: after a successful three-target merge,
// all three targets are absent from List and Search (soft-hidden) and the
// survivor is present, while every target remains fetchable by id via Get
// with its content intact — the phase's "history preserved for all
// predecessors" claim stated as an executable assertion.
func TestSupersedeMultiRecallGate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-multi-test:project:recall-gate"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")

	target1 := "f5000000-0000-0000-0000-000000000001"
	target2 := "f5000000-0000-0000-0000-000000000002"
	target3 := "f5000000-0000-0000-0000-000000000003"
	newID := "f5000000-0000-0000-0000-000000000004"
	targets := []string{target1, target2, target3}

	for _, id := range targets {
		m := Memory{ID: id, Content: "original " + id, Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}

	newMem := Memory{
		ID: newID, Content: "merged content", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: targets,
	}
	if err := s.Supersede(ctx, newMem, []float32{0.4, 0.5, 0.6}, targets, subj); err != nil {
		t.Fatalf("Supersede: %v", err)
	}

	items, _, _, err := s.List(ctx, scope, subj, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	gotIDs := recordIDs(items)
	for _, id := range targets {
		if slices.Contains(gotIDs, id) {
			t.Errorf("List: target %s present, want excluded (soft-hidden by merge): %v", id, gotIDs)
		}
	}
	if !slices.Contains(gotIDs, newID) {
		t.Errorf("List: survivor %s absent, want present: %v", newID, gotIDs)
	}

	hits, err := s.Search(ctx, scope, subj, []float32{0.1, 0.2, 0.3}, 10, SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	gotHitIDs := recordIDs(hits)
	for _, id := range targets {
		if slices.Contains(gotHitIDs, id) {
			t.Errorf("Search: target %s present, want excluded (soft-hidden by merge): %v", id, gotHitIDs)
		}
	}
	if !slices.Contains(gotHitIDs, newID) {
		t.Errorf("Search: survivor %s absent, want present: %v", newID, gotHitIDs)
	}

	for _, id := range targets {
		got, gerr := s.Get(ctx, id)
		if gerr != nil {
			t.Fatalf("Get %s: %v (must still be fetchable by id)", id, gerr)
		}
		if got.Content != "original "+id {
			t.Errorf("Get %s: content = %q, want %q (must survive intact)", id, got.Content, "original "+id)
		}
		if got.SupersededBy == nil || *got.SupersededBy != newID {
			t.Errorf("Get %s: SupersededBy = %v, want %q", id, got.SupersededBy, newID)
		}
	}
}

// TestSupersedeMultiChainKeepsSingleHead (plan 03.1-02 Task 3, D-06) proves
// a merge does not disturb the forward-chain rule: A and B merge into C,
// then C is superseded by D. A further call naming the now-non-head A is
// rejected exactly as TestSupersedeForwardChain's single-target chain would
// reject a non-head record; a further call naming the current live head D
// succeeds, keeping one live head per chain.
func TestSupersedeMultiChainKeepsSingleHead(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-multi-test:project:chain-single-head"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")

	aID := "f6000000-0000-0000-0000-000000000001"
	bID := "f6000000-0000-0000-0000-000000000002"
	cID := "f6000000-0000-0000-0000-000000000003" // merges A+B
	dID := "f6000000-0000-0000-0000-000000000004" // supersedes C
	eID := "f6000000-0000-0000-0000-000000000005" // supersedes D
	rejectAttemptID := "f6000000-0000-0000-0000-000000000099"

	for _, id := range []string{aID, bID} {
		m := Memory{ID: id, Content: "v", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	c := Memory{
		ID: cID, Content: "merged a+b", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{aID, bID},
	}
	if err := s.Supersede(ctx, c, []float32{0.2, 0.3, 0.4}, []string{aID, bID}, subj); err != nil {
		t.Fatalf("Supersede a+b->c: %v", err)
	}
	d := Memory{
		ID: dID, Content: "d supersedes c", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{cID},
	}
	if err := s.Supersede(ctx, d, []float32{0.3, 0.4, 0.5}, []string{cID}, subj); err != nil {
		t.Fatalf("Supersede c->d: %v", err)
	}

	// A is a non-head record (folded into C, then C itself was superseded);
	// naming it must still be rejected, never resurrected.
	rejectAttempt := Memory{
		ID: rejectAttemptID, Content: "invalid", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{aID},
	}
	if err := s.Supersede(ctx, rejectAttempt, []float32{0.9, 0.9, 0.9}, []string{aID}, subj); !errors.Is(err, ErrAlreadySuperseded) {
		t.Errorf("Supersede naming non-head A: err = %v, want ErrAlreadySuperseded", err)
	}
	if _, gerr := s.Get(ctx, rejectAttemptID); !errors.Is(gerr, ErrNotFound) {
		t.Errorf("Get rejected merge's survivor: err = %v, want ErrNotFound", gerr)
	}

	// D is the current live head — a normal forward-chain target, must succeed.
	e := Memory{
		ID: eID, Content: "e supersedes d", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{dID},
	}
	if err := s.Supersede(ctx, e, []float32{0.4, 0.5, 0.6}, []string{dID}, subj); err != nil {
		t.Fatalf("Supersede d->e (forward chain over the live head must succeed): %v", err)
	}

	got, gerr := s.Get(ctx, eID)
	if gerr != nil {
		t.Fatalf("Get e: %v", gerr)
	}
	if got.SupersededBy != nil {
		t.Errorf("e.SupersededBy = %v, want nil (e is the current live head)", got.SupersededBy)
	}
}

// TestSupersedeTOCTOU (D-02) verifies Store.Supersede's TOCTOU behaviour: a
// target deleted between the getWritable ownership gate and the back-stamp
// SetPayload call must not cause Supersede to return nil. Mirrors
// TestSetVisibilityTOCTOU's three-part structure (raw-protocol confirmation,
// simulated TOCTOU window, end-to-end pre-entry-deletion), swapping the
// "visibility" payload key for "superseded_by". The version guard is the
// same qdrantTOCTOUVerifiedVersion coupling TestSetVisibilityTOCTOU enforces.
func TestSupersedeTOCTOU(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if os.Getenv("ENGRAM_QDRANT_TEST_ADDR") == "" {
		hc, err := s.client.HealthCheck(ctx)
		if err != nil {
			t.Fatalf("qdrant health check: %v", err)
		}
		if v := hc.GetVersion(); v != qdrantTOCTOUVerifiedVersion {
			t.Fatalf("Qdrant version %q != verified %q: re-verify SetPayload point-ID NotFound semantics, then update qdrantTOCTOUVerifiedVersion and qdrantImageTag together", v, qdrantTOCTOUVerifiedVersion)
		}
	}

	scope := "supersede-test:project:toctou"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	// Part 1: raw SetPayload on a missing point-ID errors — the protocol
	// contract Supersede's back-stamp relies on for fail-closed TOCTOU.
	missingID := "d4000000-0000-0000-0000-000000000001"
	_, rawErr := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"superseded_by": "whatever"}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(missingID)}),
	})
	if rawErr == nil {
		t.Fatal("qdrant SetPayload on missing point-ID returned nil — the fail-closed contract for Supersede's back-stamp is broken")
	}

	// Part 2: simulate the TOCTOU window — insert, gate passes, concurrent
	// delete, then the raw SetPayload Supersede's back-stamp step would issue.
	id := "d4000000-0000-0000-0000-000000000002"
	m := Memory{ID: id, Content: "toctou-target", Scope: scope, Owner: "sub-owner", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := s.getWritable(ctx, id, Authenticated("sub-owner"), authz.ActionWrite); err != nil {
		t.Fatalf("getWritable pre-delete: %v", err)
	}
	if _, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelector(qdrant.NewID(id)),
	}); err != nil {
		t.Fatalf("concurrent delete: %v", err)
	}
	_, setPayloadErr := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"superseded_by": "whatever"}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	})
	if setPayloadErr == nil {
		t.Error("TOCTOU: SetPayload on deleted point-ID returned nil — the fail-closed contract for Supersede's back-stamp is broken")
	}

	// Part 3: end-to-end via Supersede — target deleted before the call, so
	// the getWritable gate itself rejects it (pre-entry-deletion case).
	id2 := "d4000000-0000-0000-0000-000000000003"
	newID2 := "d4000000-0000-0000-0000-000000000004"
	m2 := Memory{ID: id2, Content: "supersede-target", Scope: scope, Owner: "sub-owner", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m2, []float32{0.4, 0.5, 0.6}); err != nil {
		t.Fatalf("upsert m2: %v", err)
	}
	if err := s.Delete(ctx, id2, Authenticated("sub-owner")); err != nil {
		t.Fatalf("delete m2: %v", err)
	}
	newMem := Memory{
		ID: newID2, Content: "correction", Scope: scope, Owner: "sub-owner",
		CreatedAt: time.Now().UTC(), Supersedes: []string{id2},
	}
	if err := s.Supersede(ctx, newMem, []float32{0.4, 0.5, 0.6}, []string{id2}, Authenticated("sub-owner")); err == nil {
		t.Error("Supersede on deleted target returned nil — expected an error")
	}
}

// TestSupersedeMultiTOCTOU (REQ-merge-atomicity, plan 03.1-02 Task 2)
// generalizes TestSupersedeTOCTOU to N=3, carrying the
// qdrantTOCTOUVerifiedVersion guard verbatim.
//
// Part 1 is the falsification, asserted directly against the server rather
// than assumed: a raw multi-ID payload write over three point ids — two
// real, one that does not exist — in ONE Qdrant call. It must error AND
// leave the two existing points mutated. This is the load-bearing evidence
// that partial application is reachable at this server version; if it ever
// starts passing "nothing was written", the whole reconciliation design is
// over-engineered and this test should say so, not silently pass.
//
// Part 2 is the end-to-end proof. The originally-planned trigger (deleting a
// target inside the window) does NOT work and is not attempted here: cross-AI
// review verified against the source that Store.Supersede's defer unlock()
// releases AFTER the back-stamp (so a locker-driven delete fires too late to
// create the window), and a pre-call delete is rejected by the method's own
// under-lock getWritable before Upsert ever runs — which is exactly what
// TestSupersedeTOCTOU's own end-to-end case already proves and all it
// proves. Instead: three owned live records are upserted against the real
// pinned Qdrant, then Supersede runs on a store whose setPayloadKeys seam
// wraps the production default — it forwards the write for target1/target2
// through defaultSetPayloadKeys (so Qdrant genuinely stamps them), then
// triggers the REAL missing-id error the server raises (proved reachable by
// Part 1 above), rather than an injected sentinel. Everything else — locks,
// gates, Upsert, the compensating deletePoint, and the reconciliation
// reads/clears — is production code hitting the real server; only the
// partiality itself is substituted.
//
// Every assertion is about the state AFTER the call has returned. This test
// does not and cannot prove anything about what a concurrent reader sees
// mid-call — Store.Get and every recall query are lock-free (see
// Store.Supersede's doc comment).
func TestSupersedeMultiTOCTOU(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if os.Getenv("ENGRAM_QDRANT_TEST_ADDR") == "" {
		hc, err := s.client.HealthCheck(ctx)
		if err != nil {
			t.Fatalf("qdrant health check: %v", err)
		}
		if v := hc.GetVersion(); v != qdrantTOCTOUVerifiedVersion {
			t.Fatalf("Qdrant version %q != verified %q: re-verify SetPayload point-ID NotFound semantics, then update qdrantTOCTOUVerifiedVersion and qdrantImageTag together", v, qdrantTOCTOUVerifiedVersion)
		}
	}

	scope := "supersede-multi-test:project:toctou"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	// Part 1: the falsification.
	real1 := "ed000000-0000-0000-0000-000000000001"
	real2 := "ed000000-0000-0000-0000-000000000002"
	missing := "ed000000-0000-0000-0000-000000000099"
	for _, id := range []string{real1, real2} {
		m := Memory{ID: id, Content: "falsification-target", Scope: scope, Owner: "sub-owner", CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	_, rawErr := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload: qdrant.NewValueMap(map[string]any{"superseded_by": "falsification-probe"}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{
			qdrant.NewID(real1), qdrant.NewID(real2), qdrant.NewID(missing),
		}),
	})
	if rawErr == nil {
		t.Fatal("qdrant multi-ID SetPayload with one missing id returned nil — if this ever passes, the whole reconciliation design is over-engineered and this test should say so, not silently pass")
	}
	for _, id := range []string{real1, real2} {
		got, gerr := s.client.Get(ctx, &qdrant.GetPoints{
			CollectionName: s.collection, Ids: []*qdrant.PointId{qdrant.NewID(id)},
			WithPayload: qdrant.NewWithPayload(true),
		})
		if gerr != nil || len(got) != 1 {
			t.Fatalf("Get %s after falsification write: %v / %d points", id, gerr, len(got))
		}
		if v, ok := got[0].Payload["superseded_by"]; !ok || v.GetStringValue() != "falsification-probe" {
			t.Fatalf("%s.superseded_by after falsification write = %v, want %q — the load-bearing evidence that partial application is reachable did not hold", id, v, "falsification-probe")
		}
	}
	if _, derr := s.client.DeletePayload(ctx, &qdrant.DeletePayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Keys:           []string{"superseded_by"},
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(real1), qdrant.NewID(real2)}),
	}); derr != nil {
		t.Fatalf("cleanup falsification payload: %v", derr)
	}

	// Part 2: the end-to-end proof, driven through the setPayloadKeys seam
	// against the real server.
	subj := Authenticated("sub-owner")
	target1 := "ed000000-0000-0000-0000-000000000011"
	target2 := "ed000000-0000-0000-0000-000000000012"
	target3 := "ed000000-0000-0000-0000-000000000013"
	newID := "ed000000-0000-0000-0000-000000000014"
	targets := []string{target1, target2, target3}
	for _, id := range targets {
		m := Memory{ID: id, Content: "toctou-target", Scope: scope, Owner: "sub-owner", CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	s.setPayloadKeys = func(ctx context.Context, ids []string, kv map[string]any) error {
		// Really stamp target1/target2 (ids[0], ids[1] — sorted) via the
		// production default, so Qdrant genuinely mutates them...
		if derr := s.defaultSetPayloadKeys(ctx, ids[:2], kv); derr != nil {
			return derr
		}
		// ...then trigger the REAL missing-id error, proved reachable by
		// Part 1 above, rather than an injected sentinel.
		return s.defaultSetPayloadKeys(ctx, []string{missing}, kv)
	}
	t.Cleanup(func() { s.setPayloadKeys = nil })

	newMem := Memory{
		ID: newID, Content: "merged", Scope: scope, Owner: "sub-owner",
		CreatedAt: time.Now().UTC(), Supersedes: targets,
	}
	err := s.Supersede(ctx, newMem, []float32{0.4, 0.5, 0.6}, targets, subj)
	if err == nil {
		t.Fatal("Supersede over a target set forced to partially apply returned nil, want a non-nil error")
	}

	for _, id := range []string{target1, target2} {
		got, gerr := s.Get(ctx, id)
		if gerr != nil {
			t.Fatalf("Get %s: %v", id, gerr)
		}
		if got.SupersededBy != nil {
			t.Errorf("%s.SupersededBy = %v, want nil (no dangling link after the failed merge returned)", id, got.SupersededBy)
		}
	}

	items, _, _, lerr := s.List(ctx, scope, subj, ListOptions{Limit: 10})
	if lerr != nil {
		t.Fatalf("List: %v", lerr)
	}
	gotIDs := recordIDs(items)
	for _, id := range []string{target1, target2, target3} {
		if !slices.Contains(gotIDs, id) {
			t.Errorf("List after failed merge = %v, want %s present", gotIDs, id)
		}
	}

	hits, serr := s.Search(ctx, scope, subj, []float32{0.1, 0.2, 0.3}, 10, SearchOptions{})
	if serr != nil {
		t.Fatalf("Search: %v", serr)
	}
	gotHitIDs := recordIDs(hits)
	for _, id := range []string{target1, target2, target3} {
		if !slices.Contains(gotHitIDs, id) {
			t.Errorf("Search after failed merge = %v, want %s present", gotHitIDs, id)
		}
	}

	if _, gerr := s.Get(ctx, newID); !errors.Is(gerr, ErrNotFound) {
		t.Errorf("Get survivor after failed merge: err = %v, want ErrNotFound (compensated survivor removed)", gerr)
	}
}

// TestSupersedeMultiTOCTOUNoDanglingLink is a focused companion to
// TestSupersedeMultiTOCTOU: it re-drives the identical trigger, then reads
// EVERY requested target (all three, not just the two the seam actually
// stamped) and asserts none has a superseded_by equal to the compensated
// survivor's id — the specific T-03.1-06 invariant, with its own named,
// greppable failure independent of TestSupersedeMultiTOCTOU's broader
// List/Search/Get assertions.
func TestSupersedeMultiTOCTOUNoDanglingLink(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if os.Getenv("ENGRAM_QDRANT_TEST_ADDR") == "" {
		hc, err := s.client.HealthCheck(ctx)
		if err != nil {
			t.Fatalf("qdrant health check: %v", err)
		}
		if v := hc.GetVersion(); v != qdrantTOCTOUVerifiedVersion {
			t.Fatalf("Qdrant version %q != verified %q: re-verify SetPayload point-ID NotFound semantics, then update qdrantTOCTOUVerifiedVersion and qdrantImageTag together", v, qdrantTOCTOUVerifiedVersion)
		}
	}

	scope := "supersede-multi-test:project:toctou-no-dangling"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-owner")

	target1 := "ee100000-0000-0000-0000-000000000001"
	target2 := "ee100000-0000-0000-0000-000000000002"
	target3 := "ee100000-0000-0000-0000-000000000003"
	missing := "ee100000-0000-0000-0000-000000000099"
	newID := "ee100000-0000-0000-0000-000000000004"
	targets := []string{target1, target2, target3}
	for _, id := range targets {
		m := Memory{ID: id, Content: "v", Scope: scope, Owner: "sub-owner", CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	s.setPayloadKeys = func(ctx context.Context, ids []string, kv map[string]any) error {
		if derr := s.defaultSetPayloadKeys(ctx, ids[:2], kv); derr != nil {
			return derr
		}
		return s.defaultSetPayloadKeys(ctx, []string{missing}, kv)
	}
	t.Cleanup(func() { s.setPayloadKeys = nil })

	newMem := Memory{
		ID: newID, Content: "merged", Scope: scope, Owner: "sub-owner",
		CreatedAt: time.Now().UTC(), Supersedes: targets,
	}
	if err := s.Supersede(ctx, newMem, []float32{0.4, 0.5, 0.6}, targets, subj); err == nil {
		t.Fatal("Supersede over a target set forced to partially apply returned nil, want a non-nil error")
	}

	for _, id := range targets {
		got, gerr := s.Get(ctx, id)
		if gerr != nil {
			t.Fatalf("Get %s: %v", id, gerr)
		}
		if got.SupersededBy != nil && *got.SupersededBy == newID {
			t.Errorf("%s.SupersededBy = %q, want anything but the compensated survivor's id %q (T-03.1-06 dangling-link invariant)", id, *got.SupersededBy, newID)
		}
	}
}

// TestSupersedeMultiStamp (D-01 promote, the phase's tracer proof) pins
// Store.Supersede's multi-target contract: a single call naming three owned
// live targets stores exactly one survivor and back-stamps ALL three
// targets to point at it. N=3 is deliberate — N=1 would pass under a
// reintroduced singular assumption (the assumption-delta checkpoint's
// companion invariant test).
func TestSupersedeMultiStamp(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-multi-test:project:stamp"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")

	target1 := "e1000000-0000-0000-0000-000000000001"
	target2 := "e1000000-0000-0000-0000-000000000002"
	target3 := "e1000000-0000-0000-0000-000000000003"
	newID := "e1000000-0000-0000-0000-000000000004"
	targets := []string{target1, target2, target3} // already lexically sorted

	for _, id := range targets {
		m := Memory{ID: id, Content: "original " + id, Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}

	newMem := Memory{
		ID: newID, Content: "merged content", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: targets,
	}
	if err := s.Supersede(ctx, newMem, []float32{0.4, 0.5, 0.6}, targets, subj); err != nil {
		t.Fatalf("Supersede: %v", err)
	}

	for _, id := range targets {
		got, err := s.Get(ctx, id)
		if err != nil {
			t.Fatalf("Get %s: %v", id, err)
		}
		if got.SupersededBy == nil || *got.SupersededBy != newID {
			t.Errorf("%s.SupersededBy = %v, want %q", id, got.SupersededBy, newID)
		}
	}

	gotNew, err := s.Get(ctx, newID)
	if err != nil {
		t.Fatalf("Get new: %v", err)
	}
	if !slices.Equal(gotNew.Supersedes, targets) {
		t.Errorf("new.Supersedes = %v, want %v (three-element list, sorted order)", gotNew.Supersedes, targets)
	}

	// Exactly one survivor: all three targets are soft-hidden, and there is
	// no fourth record beyond newID.
	items, _, _, err := s.List(ctx, scope, subj, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := recordIDs(items); len(got) != 1 || got[0] != newID {
		t.Errorf("List after merge = %v, want exactly [%q] (targets soft-hidden)", got, newID)
	}
}

// TestSupersedeMultiTypedErrorCarriesOffenders (PD-10) pins the typed
// multi-target rejection: superseding two of three targets first, then
// calling again with all three, must satisfy errors.Is against the
// ErrAlreadySuperseded sentinel AND errors.As must yield *MultiTargetError
// whose IDs holds BOTH offending canonical ids — not just the first — so
// the handler tier can recover them without parsing a formatted message
// (INV-01).
func TestSupersedeMultiTypedErrorCarriesOffenders(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-multi-test:project:typed-error"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")

	target1 := "e2000000-0000-0000-0000-000000000001"
	target2 := "e2000000-0000-0000-0000-000000000002"
	target3 := "e2000000-0000-0000-0000-000000000003"
	firstNewID := "e2000000-0000-0000-0000-000000000004"
	secondNewID := "e2000000-0000-0000-0000-000000000005"

	for _, id := range []string{target1, target2, target3} {
		m := Memory{ID: id, Content: "v", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}

	// Merge target1+target2 first, leaving target3 live.
	firstNew := Memory{
		ID: firstNewID, Content: "first merge", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{target1, target2},
	}
	if err := s.Supersede(ctx, firstNew, []float32{0.2, 0.3, 0.4}, []string{target1, target2}, subj); err != nil {
		t.Fatalf("first Supersede: %v", err)
	}

	// Attempt all three: target1 and target2 are now already-superseded.
	secondNew := Memory{
		ID: secondNewID, Content: "second merge", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{target1, target2, target3},
	}
	err := s.Supersede(ctx, secondNew, []float32{0.3, 0.4, 0.5}, []string{target1, target2, target3}, subj)
	if !errors.Is(err, ErrAlreadySuperseded) {
		t.Fatalf("second Supersede err = %v, want ErrAlreadySuperseded", err)
	}
	var multiErr *MultiTargetError
	if !errors.As(err, &multiErr) {
		t.Fatalf("errors.As(err, *MultiTargetError) failed on err = %v", err)
	}
	want := []string{target1, target2}
	got := slices.Clone(multiErr.IDs)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("multiErr.IDs = %v, want %v (both offenders)", got, want)
	}

	// target3 must not have been stamped by the rejected call.
	gotTarget3, err := s.Get(ctx, target3)
	if err != nil {
		t.Fatalf("Get target3: %v", err)
	}
	if gotTarget3.SupersededBy != nil {
		t.Errorf("target3.SupersededBy = %v, want nil (rejected call must not stamp)", gotTarget3.SupersededBy)
	}
}

// TestSupersedeMultiCompensatesInline (cycle-2 review HIGH) drives the
// back-stamp failure through the declared setPayloadKeys seam AFTER all
// three targets have passed preflight and the survivor has been written —
// the only way to reach the compensation branch. A nonexistent target is
// deliberately NOT used as the trigger: getWritable rejects the whole call
// before Upsert (store.go:1621-1625), so the survivor would never be
// written and the branch under test would never execute — the
// green-by-not-running class cycle-2 review found and withdrew.
func TestSupersedeMultiCompensatesInline(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-multi-test:project:compensate"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")

	target1 := "e3000000-0000-0000-0000-000000000001"
	target2 := "e3000000-0000-0000-0000-000000000002"
	target3 := "e3000000-0000-0000-0000-000000000003"
	newID := "e3000000-0000-0000-0000-000000000004"
	targets := []string{target1, target2, target3}

	for _, id := range targets {
		m := Memory{ID: id, Content: "v", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}

	// Script the seam: (a) count the call, (b) perform the REAL stamp via
	// the production default so all three targets genuinely land the
	// back-stamp, (c) return a sentinel error — proving compensation ran
	// against real dangling links, not against nothing-was-ever-written.
	injected := errors.New("injected setPayloadKeys failure")
	calls := 0
	s.setPayloadKeys = func(ctx context.Context, ids []string, kv map[string]any) error {
		calls++
		if derr := s.defaultSetPayloadKeys(ctx, ids, kv); derr != nil {
			return derr
		}
		return injected
	}
	t.Cleanup(func() { s.setPayloadKeys = nil })

	newMem := Memory{
		ID: newID, Content: "merged content", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: targets,
	}
	err := s.Supersede(ctx, newMem, []float32{0.4, 0.5, 0.6}, targets, subj)
	if !errors.Is(err, injected) {
		t.Fatalf("Supersede err = %v, want the exact injected error %v", err, injected)
	}
	if calls != 1 {
		t.Fatalf("setPayloadKeys called %d times, want exactly 1 (distinguishes compensated-cleanup from failed-before-back-stamp)", calls)
	}

	for _, id := range targets {
		got, gerr := s.Get(ctx, id)
		if gerr != nil {
			t.Fatalf("Get %s: %v (target must still exist — no delete_memory in the merge path)", id, gerr)
		}
		if got.SupersededBy != nil {
			t.Errorf("%s.SupersededBy = %v, want nil (compensation must clear the dangling link)", id, got.SupersededBy)
		}
	}

	if _, err := s.Get(ctx, newID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get survivor after compensation: err = %v, want ErrNotFound (survivor removed)", err)
	}
}

// TestSupersedeMultiCompensatesSurvivor (plan 03.1-02, D-15) scripts the
// setPayloadKeys seam to genuinely stamp only TWO of three targets (modelling
// the pinned server's chunk-then-error partial application, RESEARCH.md
// Pitfall 1) and then return an error — the trigger the classified
// reconciliation pass is proved against, not a nonexistent-target input
// (which dies in getWritable's preflight before the survivor is ever
// written; cycle-3 review's rejected alternative). Asserts the survivor is
// gone after the call returns.
func TestSupersedeMultiCompensatesSurvivor(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-multi-test:project:compensates-survivor"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")

	target1 := "e7000000-0000-0000-0000-000000000001"
	target2 := "e7000000-0000-0000-0000-000000000002"
	target3 := "e7000000-0000-0000-0000-000000000003"
	newID := "e7000000-0000-0000-0000-000000000004"
	targets := []string{target1, target2, target3} // already lexically sorted

	for _, id := range targets {
		m := Memory{ID: id, Content: "v", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}

	injected := errors.New("injected partial back-stamp failure")
	s.setPayloadKeys = func(ctx context.Context, ids []string, kv map[string]any) error {
		// Really stamp only the first two ids, then error — a genuine
		// partial write, not a simulated one.
		if derr := s.defaultSetPayloadKeys(ctx, ids[:2], kv); derr != nil {
			return derr
		}
		return injected
	}
	t.Cleanup(func() { s.setPayloadKeys = nil })

	newMem := Memory{
		ID: newID, Content: "merged content", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: targets,
	}
	err := s.Supersede(ctx, newMem, []float32{0.4, 0.5, 0.6}, targets, subj)
	if !errors.Is(err, injected) {
		t.Fatalf("Supersede err = %v, want the injected error %v", err, injected)
	}

	if _, gerr := s.Get(ctx, newID); !errors.Is(gerr, ErrNotFound) {
		t.Errorf("Get survivor after compensation: err = %v, want ErrNotFound (survivor removed)", gerr)
	}
}

// TestSupersedeMultiReconcilesDanglingLinks drives the identical
// two-of-three partial back-stamp as TestSupersedeMultiCompensatesSurvivor,
// but asserts the OTHER half of the D-15 claim: neither of the two targets
// that really were stamped still points at the removed survivor, and the
// error Supersede returns is the exact scripted sentinel — not a
// compensation error wrapping it (D-08: orphan/dangling detail goes to the
// structured log, never into the caller-facing error).
func TestSupersedeMultiReconcilesDanglingLinks(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-multi-test:project:reconciles-dangling"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")

	target1 := "e8000000-0000-0000-0000-000000000001"
	target2 := "e8000000-0000-0000-0000-000000000002"
	target3 := "e8000000-0000-0000-0000-000000000003"
	newID := "e8000000-0000-0000-0000-000000000004"
	targets := []string{target1, target2, target3}

	for _, id := range targets {
		m := Memory{ID: id, Content: "v", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}

	injected := errors.New("injected partial back-stamp failure")
	s.setPayloadKeys = func(ctx context.Context, ids []string, kv map[string]any) error {
		if derr := s.defaultSetPayloadKeys(ctx, ids[:2], kv); derr != nil {
			return derr
		}
		return injected
	}
	t.Cleanup(func() { s.setPayloadKeys = nil })

	newMem := Memory{
		ID: newID, Content: "merged content", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: targets,
	}
	err := s.Supersede(ctx, newMem, []float32{0.4, 0.5, 0.6}, targets, subj)
	if !errors.Is(err, injected) {
		t.Fatalf("errors.Is(err, injected) = false for err = %v", err)
	}
	// errors.Is alone would ALSO pass if err were wrapped (e.g.
	// fmt.Errorf("compensation: %w", injected)) — comparing rendered text
	// catches that a compensation error must never wrap the original (D-08).
	if err.Error() != injected.Error() {
		t.Fatalf("Supersede err = %q, want the EXACT injected error text %q (unwrapped, not decorated by a compensation error)", err.Error(), injected.Error())
	}

	for _, id := range targets {
		got, gerr := s.Get(ctx, id)
		if gerr != nil {
			t.Fatalf("Get %s: %v (target must still exist)", id, gerr)
		}
		if got.SupersededBy != nil {
			t.Errorf("%s.SupersededBy = %v, want nil (reconciliation must clear any dangling link)", id, got.SupersededBy)
		}
	}
}

// TestSupersedeMultiReconcileClassifiesFailures drives
// reconcileSupersedeFailure DIRECTLY (not through Supersede) over four
// targets, one of each classification outcome, so the classification logic
// itself has a dedicated, greppable proof independent of how Supersede
// wires it in:
//   - clearable: SupersededBy already equals the (nonexistent, deliberately
//     never-created) survivor id — clear must succeed and leave neither
//     ReadFailures, ClearFailures, nor Dangling.
//   - notFound: never upserted — s.Get fails with ErrNotFound, which must be
//     classified RESOLVED and appear in no field.
//   - transportFail: a malformed (non-UUID) id — the pinned server rejects
//     it with a protocol-level InvalidArgument error, NOT NotFound
//     (confirmed directly against a real v1.18.2 server before writing this
//     test), so it must land in BOTH ReadFailures and Dangling.
//   - pointsElsewhere: SupersededBy is set but to a DIFFERENT id — nothing
//     to do, must appear in no field.
func TestSupersedeMultiReconcileClassifiesFailures(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-multi-test:project:reconcile-classify"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	// Deliberately never upserted: deleting a nonexistent point is a no-op
	// success at the pinned server (confirmed directly before writing this
	// test), so RemoveErr is expected nil here.
	survivorID := "e9000000-0000-0000-0000-000000000099"

	clearable := "e9000000-0000-0000-0000-000000000001"
	notFound := "e9000000-0000-0000-0000-000000000002" // never upserted
	transportFail := "not-a-valid-uuid-transport-probe"
	pointsElsewhere := "e9000000-0000-0000-0000-000000000003"

	clearedBy := survivorID
	elsewhereBy := "e9000000-0000-0000-0000-000000000098"
	if err := s.Upsert(ctx, Memory{
		ID: clearable, Content: "v", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), SupersededBy: &clearedBy,
	}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert clearable: %v", err)
	}
	if err := s.Upsert(ctx, Memory{
		ID: pointsElsewhere, Content: "v", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), SupersededBy: &elsewhereBy,
	}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert pointsElsewhere: %v", err)
	}

	result := s.reconcileSupersedeFailure(ctx, survivorID, []string{clearable, notFound, transportFail, pointsElsewhere})

	if result.RemoveErr != nil {
		t.Errorf("RemoveErr = %v, want nil (deleting a nonexistent survivor point is a no-op)", result.RemoveErr)
	}
	if !slices.Equal(result.ReadFailures, []string{transportFail}) {
		t.Errorf("ReadFailures = %v, want exactly [%q]", result.ReadFailures, transportFail)
	}
	if len(result.ClearFailures) != 0 {
		t.Errorf("ClearFailures = %v, want empty (the one clear attempted must succeed)", result.ClearFailures)
	}
	if !slices.Equal(result.Dangling, []string{transportFail}) {
		t.Errorf("Dangling = %v, want exactly [%q] (the transport-error id; notFound is RESOLVED and pointsElsewhere/clearable have nothing to report)", result.Dangling, transportFail)
	}

	// The clearable target's link must actually be gone — proves the clear
	// branch ran, not just that it was classified as attempted.
	got, gerr := s.Get(ctx, clearable)
	if gerr != nil {
		t.Fatalf("Get clearable: %v", gerr)
	}
	if got.SupersededBy != nil {
		t.Errorf("clearable.SupersededBy = %v, want nil (clear must have run)", got.SupersededBy)
	}

	// pointsElsewhere's link must be untouched.
	got, gerr = s.Get(ctx, pointsElsewhere)
	if gerr != nil {
		t.Fatalf("Get pointsElsewhere: %v", gerr)
	}
	if got.SupersededBy == nil || *got.SupersededBy != elsewhereBy {
		t.Errorf("pointsElsewhere.SupersededBy = %v, want %q (untouched)", got.SupersededBy, elsewhereBy)
	}
}

// TestSupersedeMultiProductionDefaultsDoNotPanic (cycle-2 review HIGH) is
// the gate against a receiver-qualified invocation of a nil-defaulted
// function-var field: New populates no function field, so on a production
// Store every seam is nil, and a receiver-qualified call panics. Injecting a
// seam to test a branch is precisely what makes that field non-nil, so no
// seam-driven test can ever catch this class — only a Store built the way
// production builds it can. Drives BOTH a successful merge (defaultSetPayloadKeys
// only) and a compensated merge (defaultDeletePoint + defaultDeletePayloadKeys,
// via the local nil-fallback, exercised for real) to completion.
//
// The failing half uses a SECOND New-built instance with a t.Cleanup-scoped
// setPayloadKeys script — the ONLY field scripted there, so deletePoint and
// deletePayloadKeys stay nil exactly as in production and the compensation
// path really does resolve them from nil. Scripting the first instance's
// setPayloadKeys would leave it non-nil for the success case too. The
// trigger is deliberately NOT a nonexistent-target input: cycle-3 review
// caught that alternative as unreachable (it dies in getWritable's preflight
// before Upsert, so compensation never runs, and this test's "no panic"
// assertion would then pass without the compensated branch ever executing —
// a false verification record against this criterion).
func TestSupersedeMultiProductionDefaultsDoNotPanic(t *testing.T) {
	ctx := context.Background()
	subj := Authenticated("sub-A")

	// Success case: production store, every function field nil.
	s1 := newTestStore(t, dialTestClient(t), testCollection("mem_eval_test_prod_defaults_1"))
	if err := s1.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("ensure s1: %v", err)
	}
	scope1 := "supersede-multi-test:project:prod-defaults-success"
	defer func() { cleanupErr(t, "DeleteAllRaw s1 "+scope1, s1.DeleteAllRaw(ctx, scope1)) }()

	target1 := "ea000000-0000-0000-0000-000000000001"
	target2 := "ea000000-0000-0000-0000-000000000002"
	target3 := "ea000000-0000-0000-0000-000000000003"
	newID := "ea000000-0000-0000-0000-000000000004"
	targets := []string{target1, target2, target3}
	for _, id := range targets {
		m := Memory{ID: id, Content: "v", Scope: scope1, Owner: "sub-A", CreatedAt: time.Now().UTC()}
		if err := s1.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	newMem := Memory{
		ID: newID, Content: "merged", Scope: scope1, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: targets,
	}
	if err := s1.Supersede(ctx, newMem, []float32{0.4, 0.5, 0.6}, targets, subj); err != nil {
		t.Fatalf("Supersede (production defaults, success case): %v", err)
	}

	// Failing case: a SECOND New-built instance.
	s2 := newTestStore(t, dialTestClient(t), testCollection("mem_eval_test_prod_defaults_2"))
	if err := s2.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("ensure s2: %v", err)
	}
	scope2 := "supersede-multi-test:project:prod-defaults-compensated"
	defer func() { cleanupErr(t, "DeleteAllRaw s2 "+scope2, s2.DeleteAllRaw(ctx, scope2)) }()

	ftarget1 := "ea000000-0000-0000-0000-000000000011"
	ftarget2 := "ea000000-0000-0000-0000-000000000012"
	ftarget3 := "ea000000-0000-0000-0000-000000000013"
	fnewID := "ea000000-0000-0000-0000-000000000014"
	ftargets := []string{ftarget1, ftarget2, ftarget3}
	for _, id := range ftargets {
		m := Memory{ID: id, Content: "v", Scope: scope2, Owner: "sub-A", CreatedAt: time.Now().UTC()}
		if err := s2.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	injected := errors.New("injected production-default panic probe")
	s2.setPayloadKeys = func(ctx context.Context, ids []string, kv map[string]any) error {
		if derr := s2.defaultSetPayloadKeys(ctx, ids, kv); derr != nil {
			return derr
		}
		return injected
	}
	t.Cleanup(func() { s2.setPayloadKeys = nil })

	fnewMem := Memory{
		ID: fnewID, Content: "merged", Scope: scope2, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: ftargets,
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Supersede (production defaults, compensation path) panicked: %v", r)
			}
		}()
		if err := s2.Supersede(ctx, fnewMem, []float32{0.4, 0.5, 0.6}, ftargets, subj); !errors.Is(err, injected) {
			t.Fatalf("Supersede err = %v, want the injected error", err)
		}
	}()
}

// capturingLogHandler is a minimal slog.Handler that records every emitted
// slog.Record for inline assertion, without going through any formatting —
// TestSupersedeMultiCompensationFailureLogged reads structured field values
// directly rather than parsing rendered text.
type capturingLogHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *capturingLogHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *capturingLogHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *capturingLogHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *capturingLogHandler) WithGroup(string) slog.Handler      { return h }

// findAttr returns the value of the first attribute named key in r, and
// whether it was present.
func findAttr(r slog.Record, key string) (slog.Value, bool) {
	var v slog.Value
	found := false
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			v, found = a.Value, true
			return false
		}
		return true
	})
	return v, found
}

// TestSupersedeMultiCompensationFailureLogged (T-03.1-08) proves the
// operator-visible half of D-08: when compensation itself cannot fully
// complete, the ORIGINAL back-stamp error is still what the caller receives
// unchanged, and the orphan/dangling detail crosses ONLY into the
// structured log, never into the returned error. Two scripted scenarios,
// each severe enough that a "real" version of this test (driven by an
// actual Qdrant outage) could only ever be flaky or vacuous — an outage
// that breaks cleanup also breaks the setup and the observation.
func TestSupersedeMultiCompensationFailureLogged(t *testing.T) {
	handler := &capturingLogHandler{}
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(prevLogger) })

	t.Run("survivor_removal_fails", func(t *testing.T) {
		s := testStore(t)
		ctx := context.Background()
		scope := "supersede-multi-test:project:compensation-logged-remove"
		defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
		subj := Authenticated("sub-A")

		target1 := "eb000000-0000-0000-0000-000000000001"
		target2 := "eb000000-0000-0000-0000-000000000002"
		newID := "eb000000-0000-0000-0000-000000000003"
		targets := []string{target1, target2}
		for _, id := range targets {
			m := Memory{ID: id, Content: "v", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
			if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
				t.Fatalf("upsert %s: %v", id, err)
			}
		}

		injected := errors.New("injected back-stamp failure (remove-fails scenario)")
		s.setPayloadKeys = func(ctx context.Context, ids []string, kv map[string]any) error {
			if derr := s.defaultSetPayloadKeys(ctx, ids, kv); derr != nil {
				return derr
			}
			return injected
		}
		t.Cleanup(func() { s.setPayloadKeys = nil })
		removeErr := errors.New("injected survivor-removal failure")
		s.deletePoint = func(_ context.Context, _ string) error { return removeErr }
		t.Cleanup(func() { s.deletePoint = nil })

		newMem := Memory{
			ID: newID, Content: "merged", Scope: scope, Owner: "sub-A",
			CreatedAt: time.Now().UTC(), Supersedes: targets,
		}
		err := s.Supersede(ctx, newMem, []float32{0.4, 0.5, 0.6}, targets, subj)
		if !errors.Is(err, injected) || err.Error() != injected.Error() {
			t.Fatalf("Supersede err = %v, want the exact original back-stamp error, unchanged by the removal failure", err)
		}

		var rec slog.Record
		found := false
		handler.mu.Lock()
		for _, r := range handler.records {
			if r.Message == "supersede compensation incomplete" {
				if v, ok := findAttr(r, "survivor_id"); ok && v.String() == newID {
					rec, found = r, true
				}
			}
		}
		handler.mu.Unlock()
		if !found {
			t.Fatalf("no structured log record found with survivor_id=%q", newID)
		}
		if v, ok := findAttr(rec, "remove_err"); !ok || v.String() == "" {
			t.Errorf("remove_err field missing or empty in log record")
		}
		if _, ok := findAttr(rec, "dangling"); !ok {
			t.Errorf("dangling field missing from log record")
		}
	})

	t.Run("clear_fails_for_one_target", func(t *testing.T) {
		s := testStore(t)
		ctx := context.Background()
		scope := "supersede-multi-test:project:compensation-logged-clear"
		defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
		subj := Authenticated("sub-A")

		target1 := "ec000000-0000-0000-0000-000000000001"
		target2 := "ec000000-0000-0000-0000-000000000002"
		newID := "ec000000-0000-0000-0000-000000000003"
		targets := []string{target1, target2}
		for _, id := range targets {
			m := Memory{ID: id, Content: "v", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
			if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
				t.Fatalf("upsert %s: %v", id, err)
			}
		}

		injected := errors.New("injected back-stamp failure (clear-fails scenario)")
		s.setPayloadKeys = func(ctx context.Context, ids []string, kv map[string]any) error {
			if derr := s.defaultSetPayloadKeys(ctx, ids, kv); derr != nil {
				return derr
			}
			return injected
		}
		t.Cleanup(func() { s.setPayloadKeys = nil })
		clearErr := errors.New("injected clear failure for target2")
		s.deletePayloadKeys = func(ctx context.Context, id string, keys []string) error {
			if id == target2 {
				return clearErr
			}
			return s.defaultDeletePayloadKeys(ctx, id, keys)
		}
		t.Cleanup(func() { s.deletePayloadKeys = nil })

		newMem := Memory{
			ID: newID, Content: "merged", Scope: scope, Owner: "sub-A",
			CreatedAt: time.Now().UTC(), Supersedes: targets,
		}
		err := s.Supersede(ctx, newMem, []float32{0.4, 0.5, 0.6}, targets, subj)
		if !errors.Is(err, injected) || err.Error() != injected.Error() {
			t.Fatalf("Supersede err = %v, want the exact original back-stamp error, unchanged by the clear failure", err)
		}

		var rec slog.Record
		found := false
		handler.mu.Lock()
		for _, r := range handler.records {
			if r.Message == "supersede compensation incomplete" {
				if v, ok := findAttr(r, "survivor_id"); ok && v.String() == newID {
					rec, found = r, true
				}
			}
		}
		handler.mu.Unlock()
		if !found {
			t.Fatalf("no structured log record found with survivor_id=%q", newID)
		}
		v, ok := findAttr(rec, "dangling")
		if !ok {
			t.Fatalf("dangling field missing from log record")
		}
		dangling, ok := v.Any().([]string)
		if !ok || !slices.Contains(dangling, target2) {
			t.Errorf("dangling = %v, want a []string containing %q", v.Any(), target2)
		}

		// target1's link, cleared successfully, must be gone; target2's,
		// which failed to clear, must still dangle — proving the failure is
		// real, not merely reported.
		got1, gerr := s.Get(ctx, target1)
		if gerr != nil {
			t.Fatalf("Get target1: %v", gerr)
		}
		if got1.SupersededBy != nil {
			t.Errorf("target1.SupersededBy = %v, want nil (clear succeeded)", got1.SupersededBy)
		}
		got2, gerr := s.Get(ctx, target2)
		if gerr != nil {
			t.Fatalf("Get target2: %v", gerr)
		}
		if got2.SupersededBy == nil || *got2.SupersededBy != newID {
			t.Errorf("target2.SupersededBy = %v, want %q (clear failed, link still dangling)", got2.SupersededBy, newID)
		}
	})
}

// The next three tests close the class RESEARCH.md's Pitfall 4 names: an
// accessor that returns a ZERO VALUE instead of an error (Value.GetStringValue
// on a list-kind Value returns "" rather than erroring) makes a lost
// tolerance indistinguishable from a record with no link at all — every test
// on the pre-phase scalar shape would stay green while every record this
// phase writes silently decoded to empty. This repo has been bitten by this
// exact class twice; these tests pin the tolerance so a regression goes red.

// TestSupersedesFromPayloadTolerantDecode drives supersedesFromPayload
// directly with hand-built map[string]*qdrant.Value inputs — not through
// Upsert, because a write-then-read round trip only ever exercises the
// shape THIS phase writes and would leave the legacy scalar shape untested.
func TestSupersedesFromPayloadTolerantDecode(t *testing.T) {
	cases := []struct {
		name string
		p    map[string]*qdrant.Value
		want []string
	}{
		{
			name: "bare string value",
			p:    qdrant.NewValueMap(map[string]any{"supersedes": "legacy-target-id"}),
			want: []string{"legacy-target-id"},
		},
		{
			name: "list value of three strings",
			p:    qdrant.NewValueMap(map[string]any{"supersedes": []any{"a", "b", "c"}}),
			want: []string{"a", "b", "c"},
		},
		{
			name: "absent key",
			p:    qdrant.NewValueMap(map[string]any{"other_key": "x"}),
			want: nil,
		},
		{
			name: "list value containing an empty string",
			p:    qdrant.NewValueMap(map[string]any{"supersedes": []any{"a", "", "c"}}),
			want: []string{"a", "", "c"},
		},
		{
			name: "list value of length zero",
			p:    qdrant.NewValueMap(map[string]any{"supersedes": []any{}}),
			want: []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := supersedesFromPayload(tc.p)
			if !slices.Equal(got, tc.want) {
				t.Errorf("supersedesFromPayload(%+v) = %v, want %v", tc.p, got, tc.want)
			}
		})
	}
}

// TestSupersedesLegacyScalarRecordDecodes writes a record to Qdrant with the
// "supersedes" payload key set to a BARE STRING — the pre-phase record
// shape — bypassing payload()'s list write by issuing a raw client Upsert
// (mirroring TestMigrateSetOwner's raw-payload-override idiom), then reads
// it back through Store.Get and asserts Supersedes decodes to a one-element
// list holding that string. It must not read as empty.
func TestSupersedesLegacyScalarRecordDecodes(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-codec-test:project:legacy-scalar"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	id := "e4000000-0000-0000-0000-000000000001"
	p := payload(Memory{ID: id, Content: "legacy record", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()})
	p["supersedes"] = "legacy-target-id" // pre-phase scalar shape, overriding payload()'s (absent, this record has no Supersedes) list write.
	if _, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: []*qdrant.PointStruct{{
			Id: qdrant.NewID(id), Vectors: qdrant.NewVectors(0.1, 0.2, 0.3),
			Payload: qdrant.NewValueMap(p),
		}},
	}); err != nil {
		t.Fatalf("raw upsert: %v", err)
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Supersedes) != 1 || got.Supersedes[0] != "legacy-target-id" {
		t.Errorf("Supersedes = %v, want [%q] (legacy scalar must not read as empty)", got.Supersedes, "legacy-target-id")
	}
}

// TestSupersedesListRecordDecodes writes through the normal Upsert path with
// a three-element Supersedes and asserts Store.Get returns the same three in
// order — the shape every record written by this phase carries.
func TestSupersedesListRecordDecodes(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-codec-test:project:list-record"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	id := "e4000000-0000-0000-0000-000000000002"
	want := []string{"target-a", "target-b", "target-c"}
	m := Memory{ID: id, Content: "merged", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC(), Supersedes: want}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !slices.Equal(got.Supersedes, want) {
		t.Errorf("Supersedes = %v, want %v (same three, in order)", got.Supersedes, want)
	}
}

// TestSupersedeMultiDuplicateTargets (D-10, RESEARCH.md Pitfall 2) proves a
// repeated target completes rather than deadlocking: the in-process locker
// is a plain non-reentrant sync.Mutex, so acquiring the SAME resolved
// target twice on one goroutine would self-deadlock permanently — no panic,
// no timeout from the mutex itself, and Go's runtime deadlock detector does
// not catch a goroutine blocking on a lock it already holds. Guards the
// call with a bounded wait: a regression here would hang the whole test
// binary rather than fail, which is exactly the failure mode being guarded.
func TestSupersedeMultiDuplicateTargets(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-multi-test:project:duplicate-targets"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")

	targetID := "e5000000-0000-0000-0000-000000000001"
	newID := "e5000000-0000-0000-0000-000000000002"
	target := Memory{ID: targetID, Content: "v", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, target, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert target: %v", err)
	}

	newMem := Memory{
		ID: newID, Content: "merged", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{targetID},
	}

	done := make(chan error, 1)
	go func() {
		done <- s.Supersede(ctx, newMem, []float32{0.4, 0.5, 0.6}, []string{targetID, targetID, targetID}, subj)
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Supersede: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Supersede with a repeated target did not return within 5s — self-deadlock on the non-reentrant per-target lock")
	}

	got, err := s.Get(ctx, targetID)
	if err != nil {
		t.Fatalf("Get target: %v", err)
	}
	if got.SupersededBy == nil || *got.SupersededBy != newID {
		t.Errorf("target.SupersededBy = %v, want %q", got.SupersededBy, newID)
	}

	gotNew, err := s.Get(ctx, newID)
	if err != nil {
		t.Fatalf("Get new: %v", err)
	}
	if len(gotNew.Supersedes) != 1 || gotNew.Supersedes[0] != targetID {
		t.Errorf("new.Supersedes = %v, want [%q] (target stamped once, not three)", gotNew.Supersedes, targetID)
	}
}

// TestSupersedeMultiConcurrentOverlapping (D-10) proves sorted per-target
// lock acquisition: two goroutines superseding overlapping-but-different
// target sets ({A,B} and {B,C}) can never hold each other's next lock, so
// both terminate, and at most one succeeds for the shared target B. Copies
// TestSupersedeConcurrent's sync.WaitGroup + errs + classify-by-errors.Is
// shape, guarded by the same bounded-wait discipline as
// TestSupersedeMultiDuplicateTargets.
func TestSupersedeMultiConcurrentOverlapping(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-multi-test:project:concurrent-overlapping"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")

	aID := "e6000000-0000-0000-0000-000000000001"
	bID := "e6000000-0000-0000-0000-000000000002"
	cID := "e6000000-0000-0000-0000-000000000003"
	firstNewID := "e6000000-0000-0000-0000-000000000004"
	secondNewID := "e6000000-0000-0000-0000-000000000005"

	for _, id := range []string{aID, bID, cID} {
		m := Memory{ID: id, Content: "v", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}

	firstNew := Memory{
		ID: firstNewID, Content: "merge AB", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{aID, bID},
	}
	secondNew := Memory{
		ID: secondNewID, Content: "merge BC", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{bID, cID},
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs[0] = s.Supersede(ctx, firstNew, []float32{0.2, 0.3, 0.4}, []string{aID, bID}, subj)
	}()
	go func() {
		defer wg.Done()
		errs[1] = s.Supersede(ctx, secondNew, []float32{0.3, 0.4, 0.5}, []string{bID, cID}, subj)
	}()

	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-time.After(10 * time.Second):
		t.Fatal("two overlapping Supersede calls did not both terminate within 10s — sorted-lock-ordering deadlock (D-10 violated)")
	}

	successes, conflicts := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrAlreadySuperseded):
			conflicts++
		default:
			t.Fatalf("unexpected Supersede error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent overlapping Supersede: got %d successes, %d conflicts (errs=%v), want exactly 1 success and 1 conflict", successes, conflicts, errs)
	}

	gotB, err := s.Get(ctx, bID)
	if err != nil {
		t.Fatalf("Get B: %v", err)
	}
	if gotB.SupersededBy == nil {
		t.Fatal("B.SupersededBy = nil, want set to the winning call's survivor id")
	}
	winner := firstNewID
	if errs[0] != nil {
		winner = secondNewID
	}
	if *gotB.SupersededBy != winner {
		t.Errorf("B.SupersededBy = %q, want %q (the winning call's survivor)", *gotB.SupersededBy, winner)
	}

	// The losing call's non-shared target (A if AB lost, C if BC lost) must
	// not be stamped by the call that failed.
	if errs[0] != nil {
		gotA, err := s.Get(ctx, aID)
		if err != nil {
			t.Fatalf("Get A: %v", err)
		}
		if gotA.SupersededBy != nil {
			t.Errorf("A.SupersededBy = %v, want nil (the failing AB call must not stamp A)", gotA.SupersededBy)
		}
	} else {
		gotC, err := s.Get(ctx, cID)
		if err != nil {
			t.Fatalf("Get C: %v", err)
		}
		if gotC.SupersededBy != nil {
			t.Errorf("C.SupersededBy = %v, want nil (the failing BC call must not stamp C)", gotC.SupersededBy)
		}
	}
}

// TestSupersedeConcurrentKeyedDisjointTargetsCannotBothLand (03.1 cycle-4
// review CR-01) pins the fix for a data-integrity gap the per-target lock
// alone left open: two concurrent KEYED Supersede calls that share the same
// survivor id (simulating idempotencyPointID(owner, scope, key), which is
// deterministic and independent of the target set) but name DISJOINT target
// sets never contended on any per-target lock — both could pass their own
// preflight and both Upsert the same survivor id, the second whole-payload
// replacing the first with no error and no log (silent corruption: the
// first call's already-back-stamped target ends up pointing at a survivor
// whose persisted Supersedes omits it entirely).
//
// This asserts on GRAPH STATE, not merely a returned error: exactly one call
// must succeed and the other must observe store.ErrIdempotencyConflict
// (never both succeeding, and never any other error); the persisted
// survivor's Supersedes must equal exactly the WINNING call's target set;
// the winning target must be back-stamped to the survivor; and — the
// corruption this fix closes — the LOSING call's target must NOT be
// back-stamped at all, since the fix rejects the losing call under the lock
// BEFORE it ever reaches Upsert or the back-stamp. Run with -race.
func TestSupersedeConcurrentKeyedDisjointTargetsCannotBothLand(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "supersede-test:project:keyed-disjoint-race"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")

	aID := "ee000000-0000-0000-0000-000000000001"
	bID := "ee000000-0000-0000-0000-000000000002"
	// survivorID stands in for a deterministic idempotencyPointID: both
	// racing calls share it, exactly as two calls sharing an owner+scope+key
	// would at the server layer.
	survivorID := "ee000000-0000-0000-0000-0000000000f0"

	for _, id := range []string{aID, bID} {
		m := Memory{ID: id, Content: "v", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}

	fixA := Memory{
		ID: survivorID, Content: "fix A", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{aID},
		IdempotencyFingerprint: "fp-fix-a",
	}
	fixB := Memory{
		ID: survivorID, Content: "fix B", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), Supersedes: []string{bID},
		IdempotencyFingerprint: "fp-fix-b",
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs[0] = s.Supersede(ctx, fixA, []float32{0.2, 0.3, 0.4}, []string{aID}, subj)
	}()
	go func() {
		defer wg.Done()
		errs[1] = s.Supersede(ctx, fixB, []float32{0.3, 0.4, 0.5}, []string{bID}, subj)
	}()

	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-time.After(10 * time.Second):
		t.Fatal("two disjoint-target keyed Supersede calls did not both terminate within 10s — locking newMem.ID alongside targets must not deadlock")
	}

	successes, conflicts := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrIdempotencyConflict):
			conflicts++
		default:
			t.Fatalf("unexpected Supersede error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent keyed disjoint-target Supersede: got %d successes, %d idempotency conflicts (errs=%v), want exactly 1 success and 1 conflict — BOTH succeeding means the race this test guards against is back", successes, conflicts, errs)
	}

	winnerTarget, loserTarget := aID, bID
	if errs[0] != nil {
		winnerTarget, loserTarget = bID, aID
	}

	survivor, err := s.Get(ctx, survivorID)
	if err != nil {
		t.Fatalf("Get survivor: %v", err)
	}
	if len(survivor.Supersedes) != 1 || survivor.Supersedes[0] != winnerTarget {
		t.Fatalf("survivor.Supersedes = %v, want [%q] (only the winning call's target set — the losing call's whole-payload Upsert must never have landed)", survivor.Supersedes, winnerTarget)
	}

	gotWinner, err := s.Get(ctx, winnerTarget)
	if err != nil {
		t.Fatalf("Get winning target: %v", err)
	}
	if gotWinner.SupersededBy == nil || *gotWinner.SupersededBy != survivorID {
		t.Errorf("winning target %s: SupersededBy = %v, want %q", winnerTarget, gotWinner.SupersededBy, survivorID)
	}

	// The corruption this fix closes: pre-fix, the losing call still reached
	// its own back-stamp (its Upsert never conflicted, since nothing locked
	// the shared survivor id), leaving this target pointing at a survivor
	// whose persisted Supersedes omits it — a dangling forward link with no
	// error and no log. Post-fix the losing call must be rejected UNDER THE
	// LOCK before ever reaching Upsert or the back-stamp, so this target's
	// superseded_by must remain nil.
	gotLoser, err := s.Get(ctx, loserTarget)
	if err != nil {
		t.Fatalf("Get losing target: %v", err)
	}
	if gotLoser.SupersededBy != nil {
		t.Errorf("losing target %s: SupersededBy = %v, want nil (the losing keyed call must be rejected before it ever back-stamps its target)", loserTarget, *gotLoser.SupersededBy)
	}
}

// TestListCursorTraversal pins order-independent cursor paging at limit=1: N
// records sharing ONE timestamp plus M with distinct timestamps, paged to
// exhaustion, must yield the full set with no duplicates and no skips. At limit=1
// this only passes if resume over-fetches limit+len(seen).
func TestListCursorTraversal(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "cursor-test:project:x"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	tie := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	want := map[string]bool{}
	mk := func(id string, at time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A", CreatedAt: at}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert %s: %v", id, err)
		}
		want[id] = true
	}
	mk("d0000000-0000-0000-0000-000000000001", tie)
	mk("d0000000-0000-0000-0000-000000000002", tie)
	mk("d0000000-0000-0000-0000-000000000003", tie)
	mk("d0000000-0000-0000-0000-000000000004", tie)
	mk("d0000000-0000-0000-0000-000000000005", tie.Add(-time.Hour))
	mk("d0000000-0000-0000-0000-000000000006", tie.Add(-2*time.Hour))

	subj := Authenticated("sub-A")
	seen := map[string]int{}
	cursor := ""
	for steps := 0; steps < 100; steps++ {
		// CursorMode:true makes page 1 (Cursor:"") route through listByCursor, which
		// emits a nextCursor; without it the offset path runs and returns "" → break.
		items, _, next, err := s.List(ctx, scope, subj, ListOptions{Limit: 1, Cursor: cursor, CursorMode: true})
		if err != nil {
			t.Fatalf("List page: %v", err)
		}
		for _, m := range items {
			seen[m.ID]++
		}
		if next == "" {
			break
		}
		cursor = next
	}
	if len(seen) != len(want) {
		t.Errorf("traversal coverage: got %d distinct want %d", len(seen), len(want))
	}
	for id, nn := range seen {
		if nn != 1 {
			t.Errorf("record %s returned %d times (want 1) — dup/skip bug", id, nn)
		}
		if !want[id] {
			t.Errorf("unexpected id %s", id)
		}
	}
	for id := range want {
		if seen[id] == 0 {
			t.Errorf("record %s never returned — skip bug", id)
		}
	}
}

func TestListAscendingOrder(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "rule:repo:asc-order-test"
	subj := Authenticated("owner-asc")
	// Three records with distinct, increasing created_at.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mk := func(id string, mins int) {
		m := Memory{
			ID: id, Content: "c", Scope: scope, Category: "rule",
			Owner: "owner-asc", Visibility: "shared",
			Source: "user-said", Summary: "s", SummarySource: SummarySourceClient,
			CreatedAt: base.Add(time.Duration(mins) * time.Minute),
		}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	mk("c1000000-0000-0000-0000-000000000001", 0)
	mk("c1000000-0000-0000-0000-000000000002", 10)
	mk("c1000000-0000-0000-0000-000000000003", 20)
	t.Cleanup(func() { cleanupErr(t, "DeleteAll", s.DeleteAll(ctx, scope, subj)) })

	got, _, _, err := s.List(ctx, scope, subj, ListOptions{Ascending: true})
	if err != nil {
		t.Fatalf("List ascending: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d records, want 3", len(got))
	}
	if got[0].ID != "c1000000-0000-0000-0000-000000000001" ||
		got[2].ID != "c1000000-0000-0000-0000-000000000003" {
		t.Errorf("not ascending by created_at: first=%s last=%s", got[0].ID, got[2].ID)
	}
}

// TestListAscendingRejectedInCursorMode pins that Ascending is honored only in
// offset/all mode: combining it with cursor paging is a hard ErrInvalidArgument
// (mirroring the Cursor/Offset mutual-exclusion guard), not a silent desc order.
func TestListAscendingRejectedInCursorMode(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	_, _, _, err := s.List(ctx, "rule:repo:asc-cursor-guard", Authenticated("owner-x"),
		ListOptions{Ascending: true, CursorMode: true})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("want ErrInvalidArgument for Ascending+CursorMode, got %v", err)
	}
}

// TestListScheduledDateWindow pins the created_at window on the scheduled view.
func TestListScheduledDateWindow(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "sched-win:project:x"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	future := s.now().Add(48 * time.Hour)
	t0 := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	mk := func(id string, created time.Time) {
		nb := future
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A",
			NotBefore: &nb, CreatedAt: created}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert %s: %v", id, err)
		}
	}
	mk("e0000000-0000-0000-0000-000000000001", t0.Add(-time.Hour)) // before window
	mk("e0000000-0000-0000-0000-000000000002", t0.Add(time.Hour))  // inside

	got, err := s.ListScheduled(ctx, scope, Authenticated("sub-A"),
		ScheduledPending, ListOptions{Limit: 10, CreatedAfter: t0})
	if err != nil {
		t.Fatalf("ListScheduled: %v", err)
	}
	if len(got) != 1 || got[0].ID != "e0000000-0000-0000-0000-000000000002" {
		t.Errorf("scheduled window: got %v want just ..002", recordIDs(got))
	}
}

// TestListOffsetCursorMutuallyExclusive pins the guard: a List that sets both a
// resume Cursor and a positive Offset is a client error tagged ErrInvalidArgument
// (so the Connect layer maps it to CodeInvalidArgument, not CodeInternal).
func TestListOffsetCursorMutuallyExclusive(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	tok := encodeCursor(listCursor{C: "2026-06-27T12:00:00Z", Seen: []string{"x"}})
	_, _, _, err := s.List(ctx, "iso:project:x", Authenticated("sub-A"), ListOptions{
		Cursor: tok, Offset: 1, Limit: 10,
	})
	if err == nil {
		t.Fatal("List with both Cursor and Offset accepted; want error")
	}
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("want ErrInvalidArgument, got %v", err)
	}
}

// TestDatetimeIndexBackfillsExistingRecords proves the no-migration claim: records
// written as RFC3339 strings BEFORE the created_at datetime index exists become
// range-filterable AFTER the index is created (Qdrant backfills; no re-stamp).
func TestDatetimeIndexBackfillsExistingRecords(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	coll := "migration-realism-" + t.Name() // unique bare collection
	// 1. Bare collection — NO indexes.
	if err := s.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: coll,
		VectorsConfig:  qdrant.NewVectorsConfig(&qdrant.VectorParams{Size: 3, Distance: qdrant.Distance_Cosine}),
	}); err != nil {
		t.Fatalf("create bare collection: %v", err)
	}
	t.Cleanup(func() { _ = s.client.DeleteCollection(ctx, coll) })
	// 2. Insert RFC3339-string created_at records BEFORE any index exists.
	old := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	for i, at := range []time.Time{old.Add(-time.Hour), old, old.Add(time.Hour)} {
		if _, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: coll, Wait: qdrant.PtrOf(true),
			Points: []*qdrant.PointStruct{{
				Id:      qdrant.NewID(fmt.Sprintf("a0000000-0000-0000-0000-00000000000%d", i+1)),
				Vectors: qdrant.NewVectors(0.1, 0.2, 0.3),
				Payload: qdrant.NewValueMap(map[string]any{"created_at": at.Format(time.RFC3339)}),
			}},
		}); err != nil {
			t.Fatalf("upsert pre-index %d: %v", i, err)
		}
	}
	// 3. NOW create the datetime index over the already-written string records.
	if _, err := s.client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
		CollectionName: coll, FieldName: "created_at",
		FieldType: qdrant.PtrOf(qdrant.FieldType_FieldTypeDatetime), Wait: qdrant.PtrOf(true),
	}); err != nil {
		t.Fatalf("create datetime index post-insert: %v", err)
	}
	// 4. A DatetimeRange query now returns the pre-index records — proves backfill, no migration.
	got, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: coll,
		Filter:         &qdrant.Filter{Must: []*qdrant.Condition{createdRangeCondition(old, time.Time{})}},
		Limit:          qdrant.PtrOf(uint32(10)), WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		t.Fatalf("range scroll post-index: %v", err)
	}
	if len(got) != 2 { // old (==after, gte) and old+1h; the -1h record is excluded
		t.Errorf("post-index range query: got %d records want 2 (pre-index records must be range-filterable)", len(got))
	}
}

func TestResolvePointID(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	defer func() { cleanupErr(t, "DeleteAllRaw s", st.DeleteAllRaw(ctx, "s")) }()
	vec := []float32{0.1, 0.2, 0.3}
	u := "a0000000-0000-0000-0000-000000000010"
	if err := st.Upsert(ctx, Memory{ID: u, ShortID: "j7k2m9p4x0", Content: "c", Scope: "s", Owner: "o"}, vec); err != nil {
		t.Fatal(err)
	}
	u2 := "a0000000-0000-0000-0000-000000000011"
	if err := st.Upsert(ctx, Memory{ID: u2, ShortID: "1a0bcdef23", Content: "c", Scope: "s", Owner: "o"}, vec); err != nil {
		t.Fatal(err)
	}

	check := func(name, in, wantID string, wantErr error) {
		got, err := st.ResolvePointID(ctx, in)
		if wantErr != nil {
			if !errors.Is(err, wantErr) {
				t.Fatalf("%s: err=%v want %v", name, err, wantErr)
			}
			return
		}
		if err != nil || got != wantID {
			t.Fatalf("%s: got %q err %v want %q", name, got, err, wantID)
		}
	}
	check("uuid fast path", u, u, nil)
	check("raw-hex uuid canonicalized", "a0000000000000000000000000000010", u, nil)
	check("short id exact", "j7k2m9p4x0", u, nil)
	check("short id upper+glyph+space", " IaObcdef23 ", u2, nil)                                   // I->1, O->0
	check("padded canonical uuid → fast path", "  a0000000-0000-0000-0000-000000000010  ", u, nil) // item 30
	check("nonexistent short id", "zzzzzzzzzz", "", ErrNotFound)
	check("8-char uuid prefix (original bug)", "a0000000", "", ErrNotFound)
	check("empty", "   ", "", ErrInvalidArgument)
}

func TestResolvePointIDAmbiguous(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	defer func() { cleanupErr(t, "DeleteAllRaw s", st.DeleteAllRaw(ctx, "s")) }()
	vec := []float32{0.1, 0.2, 0.3}
	// Force two records sharing a short id by writing them directly (bypassing MintShortID).
	for _, id := range []string{"a0000000-0000-0000-0000-000000000020", "a0000000-0000-0000-0000-000000000021"} {
		if err := st.Upsert(ctx, Memory{ID: id, ShortID: "dupdupdup0", Content: "c", Scope: "s", Owner: "o"}, vec); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.ResolvePointID(ctx, "dupdupdup0"); !errors.Is(err, ErrAmbiguousShortID) {
		t.Fatalf("want ErrAmbiguousShortID, got %v", err)
	}
}

func TestMintShortIDUnique(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	defer func() { cleanupErr(t, "DeleteAllRaw s", st.DeleteAllRaw(ctx, "s")) }()
	a, err := st.MintShortID(ctx, nil)
	if err != nil || len(a) != shortid.Length {
		t.Fatalf("mint a: %q err %v", a, err)
	}
	if err := st.Upsert(ctx, Memory{ID: "a0000000-0000-0000-0000-000000000030", ShortID: a, Content: "c", Scope: "s", Owner: "o"}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatal(err)
	}
	b, err := st.MintShortID(ctx, nil)
	if err != nil || b == a {
		t.Fatalf("mint b collided/errored: %q err %v", b, err)
	}
}

func TestMintShortIDRetriesOnCollision(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	defer func() { cleanupErr(t, "DeleteAllRaw s", st.DeleteAllRaw(ctx, "s")) }()
	// Persist "collidecol" so the first candidate collides, forcing the retry branch.
	if err := st.Upsert(ctx, Memory{ID: "a0000000-0000-0000-0000-000000000031", ShortID: "collidecol", Content: "c", Scope: "s", Owner: "o"}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	st.mintCandidate = func() (string, error) {
		calls++
		if calls == 1 {
			return "collidecol", nil // taken → must retry
		}
		return "freshfresh", nil
	}
	got, err := st.MintShortID(ctx, nil)
	if err != nil || got != "freshfresh" || calls != 2 {
		t.Fatalf("got %q err %v calls %d (want freshfresh / 2)", got, err, calls)
	}
}

// TestMintShortIDExhaustsAfterCap forces every candidate to collide (a real
// Qdrant Count() check on each attempt) and asserts MintShortID gives up with
// an errors.Is-checkable ErrShortIDExhausted after exactly maxMintAttempts
// real collision checks (D-04).
func TestMintShortIDExhaustsAfterCap(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	defer func() { cleanupErr(t, "DeleteAllRaw s", st.DeleteAllRaw(ctx, "s")) }()
	if err := st.Upsert(ctx, Memory{ID: "a0000000-0000-0000-0000-000000000032", ShortID: "alwaystaken", Content: "c", Scope: "s", Owner: "o"}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	st.mintCandidate = func() (string, error) {
		calls++
		return "alwaystaken", nil // every candidate collides — forces exhaustion
	}
	_, err := st.MintShortID(ctx, nil)
	if !errors.Is(err, ErrShortIDExhausted) {
		t.Fatalf("err = %v, want ErrShortIDExhausted", err)
	}
	if calls != maxMintAttempts {
		t.Fatalf("calls = %d, want %d", calls, maxMintAttempts)
	}
}

// TestMintShortIDSeenMapDoesNotConsumeBudget proves (D-05) that seen-map dup
// hits are skipped for free — a pre-populated seen map does not shrink the
// real-collision-check budget below maxMintAttempts before exhaustion.
func TestMintShortIDSeenMapDoesNotConsumeBudget(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	defer func() { cleanupErr(t, "DeleteAllRaw s", st.DeleteAllRaw(ctx, "s")) }()
	if err := st.Upsert(ctx, Memory{ID: "a0000000-0000-0000-0000-000000000033", ShortID: "alwaystaken", Content: "c", Scope: "s", Owner: "o"}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatal(err)
	}
	seen := map[string]struct{}{
		"dup1": {}, "dup2": {}, "dup3": {},
	}
	dups := []string{"dup1", "dup2", "dup3"}
	genIdx := 0
	calls := 0
	st.mintCandidate = func() (string, error) {
		if genIdx < len(dups) {
			c := dups[genIdx]
			genIdx++
			return c, nil // seen-map hit — must not consume the real-check budget
		}
		calls++
		return "alwaystaken", nil // every real candidate collides — forces exhaustion
	}
	_, err := st.MintShortID(ctx, seen)
	if !errors.Is(err, ErrShortIDExhausted) {
		t.Fatalf("err = %v, want ErrShortIDExhausted", err)
	}
	if calls != maxMintAttempts {
		t.Fatalf("real collision-check calls = %d, want %d (seen-map dups must not consume the budget)", calls, maxMintAttempts)
	}
}

// TestMintShortIDDegenerateGeneratorTerminates proves the absolute spin cap
// (maxMintSpins) guarantees termination even when the injectable mintCandidate
// seam returns ONLY already-seen candidates forever. Because seen-map skips do
// not consume the maxMintAttempts real-collision-check budget (D-05), without
// the spin cap this loop would never terminate. The real Qdrant Count() check
// is never reached (every candidate is a seen-map hit), so this exercises the
// spin cap in isolation.
func TestMintShortIDDegenerateGeneratorTerminates(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	seen := map[string]struct{}{"onlyvalue0": {}}
	calls := 0
	st.mintCandidate = func() (string, error) {
		calls++
		return "onlyvalue0", nil // always a seen-map hit — never reaches the real Count check
	}
	_, err := st.MintShortID(ctx, seen)
	if !errors.Is(err, ErrShortIDExhausted) {
		t.Fatalf("err = %v, want ErrShortIDExhausted (spin cap must halt a seen-only generator)", err)
	}
	if calls != maxMintSpins {
		t.Fatalf("generator called %d times, want maxMintSpins=%d (spin cap must bound and terminate the seen-only loop)", calls, maxMintSpins)
	}
}

// floatsEqual reports whether two float32 slices are element-wise equal.
func floatsEqual(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestUpdatePayloadPreservesVectorBumpsUsageAndClearsProvenance is the
// round-3/5/7/8 payload-only-update regression guard. It seeds a record with
// stale auto-summary provenance (SummarySourceAuto + a non-empty SummaryModel
// + a non-zero SummaryEgressAt) and asserts, across two UpdatePayload calls
// (a client-summary write, then a summary clear):
//
//	(a) the stored vector is byte-identical before/after, verified via the raw
//	    Qdrant client with WithVectors(true) (Store.Get omits vectors, so it
//	    cannot prove preservation — round-3 MED-4);
//	(b) AccessCount increments and LastAccessedAt advances on every call (the
//	    usage-signal regression guard, review finding 6);
//	(c) summary_model/summary_egress_at are DELETED from the raw payload (not
//	    merely zeroed in the decoded struct) after both the client-summary
//	    write and the summary clear (round-3 MED-3, round-5 MED grok, round-7
//	    HIGH — the targeted DeletePayload, not a whole-payload overwrite).
func TestUpdatePayloadPreservesVectorBumpsUsageAndClearsProvenance(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:update-payload"
	owner := "sub-updatepayload"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, st.DeleteAllRaw(ctx, scope)) }()

	id := "c0000000-0000-0000-0000-000000000001"
	vec := []float32{0.3, 0.4, 0.5}
	seeded := Memory{
		ID: id, Content: "v", Scope: scope, Owner: owner,
		CreatedAt:       time.Now().UTC(),
		Summary:         "auto summary",
		SummarySource:   SummarySourceAuto,
		SummaryModel:    "gpt-x",
		SummaryEgressAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := st.Upsert(ctx, seeded, vec); err != nil {
		t.Fatalf("seed upsert: %v", err)
	}

	before := scrollPoints(t, st.client, st.collection)[id]
	if len(before.vec) == 0 {
		t.Fatal("seed: no vector persisted")
	}
	for _, k := range provenanceKeys {
		if _, ok := before.payload[k]; !ok {
			t.Fatalf("seed: expected provenance key %q present", k)
		}
	}

	// --- Call 1: shared=true + a CLIENT summary write. ---
	cur, err := st.FetchForUpdate(ctx, id, Authenticated(owner))
	if err != nil {
		t.Fatalf("FetchForUpdate: %v", err)
	}
	shared := true
	clientSummary := "client summary"
	if err := st.UpdatePayload(ctx, cur, &shared, &clientSummary); err != nil {
		t.Fatalf("UpdatePayload (client summary): %v", err)
	}

	afterClient := scrollPoints(t, st.client, st.collection)[id]
	if !floatsEqual(before.vec, afterClient.vec) {
		t.Errorf("vector changed after UpdatePayload: before=%v after=%v", before.vec, afterClient.vec)
	}
	for _, k := range provenanceKeys {
		if _, ok := afterClient.payload[k]; ok {
			t.Errorf("raw payload still carries provenance key %q after client-summary write", k)
		}
	}

	got, err := st.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.AccessCount != 1 {
		t.Errorf("AccessCount = %d, want 1", got.AccessCount)
	}
	if got.LastAccessedAt == nil {
		t.Error("LastAccessedAt not stamped")
	}
	if got.Visibility != visibilityShared {
		t.Errorf("Visibility = %q, want %q", got.Visibility, visibilityShared)
	}
	if got.Summary != clientSummary || got.SummarySource != SummarySourceClient {
		t.Errorf("summary not persisted as client: summary=%q source=%q", got.Summary, got.SummarySource)
	}
	if got.SummaryModel != "" || !got.SummaryEgressAt.IsZero() {
		t.Errorf("decoded provenance not cleared: model=%q egress=%v", got.SummaryModel, got.SummaryEgressAt)
	}

	// Re-seed auto-provenance directly via the raw client (simulating a later
	// auto-fill sweep) so call 2 can prove a summary CLEAR also deletes it.
	if _, err := st.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: st.collection, Wait: qdrant.PtrOf(true),
		Payload: qdrant.NewValueMap(map[string]any{
			"summary_source":    string(SummarySourceAuto),
			"summary_model":     "gpt-y",
			"summary_egress_at": time.Now().UTC().Format(time.RFC3339),
		}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	}); err != nil {
		t.Fatalf("re-seed provenance: %v", err)
	}

	// --- Call 2: summary CLEAR (empty string). ---
	cur2, err := st.FetchForUpdate(ctx, id, Authenticated(owner))
	if err != nil {
		t.Fatalf("FetchForUpdate 2: %v", err)
	}
	empty := ""
	if err := st.UpdatePayload(ctx, cur2, nil, &empty); err != nil {
		t.Fatalf("UpdatePayload (clear): %v", err)
	}

	got2, err := st.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get 2: %v", err)
	}
	if got2.Summary != "" || got2.SummarySource != SummarySourceNone {
		t.Errorf("summary not cleared: summary=%q source=%q", got2.Summary, got2.SummarySource)
	}
	if got2.AccessCount != 2 {
		t.Errorf("AccessCount = %d, want 2 (bumped again)", got2.AccessCount)
	}

	afterClear := scrollPoints(t, st.client, st.collection)[id]
	for _, k := range provenanceKeys {
		if _, ok := afterClear.payload[k]; ok {
			t.Errorf("raw payload still carries provenance key %q after summary clear", k)
		}
	}
	if !floatsEqual(before.vec, afterClear.vec) {
		t.Errorf("vector changed after second UpdatePayload: before=%v after=%v", before.vec, afterClear.vec)
	}
}

// TestUpdatePayloadInjectedDeletePayloadFailure is the round-8 injected-failure
// test: it forces the SECOND op (DeletePayload, the provenance clear) to fail
// via the deletePayloadKeys function-var seam (the real *qdrant.Client has no
// interface seam, so this mirrors the existing mintCandidate override
// pattern) while the FIRST op (SetPayload) has already succeeded, and asserts
// the documented partial-success contract: the exact injected error is
// surfaced, the primary mutation (summary/summary_source/access_count/
// last_accessed_at) is COMMITTED (readable on a fresh Get), and the
// provenance keys remain STALE (present, not deleted) — never content/vector
// corruption.
func TestUpdatePayloadInjectedDeletePayloadFailure(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:update-payload-fail"
	owner := "sub-updatepayload-fail"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, st.DeleteAllRaw(ctx, scope)) }()

	id := "c0000000-0000-0000-0000-000000000002"
	seeded := Memory{
		ID: id, Content: "v", Scope: scope, Owner: owner,
		CreatedAt:       time.Now().UTC(),
		Summary:         "auto summary",
		SummarySource:   SummarySourceAuto,
		SummaryModel:    "gpt-x",
		SummaryEgressAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := st.Upsert(ctx, seeded, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cur, err := st.FetchForUpdate(ctx, id, Authenticated(owner))
	if err != nil {
		t.Fatalf("FetchForUpdate: %v", err)
	}

	injected := errors.New("injected DeletePayload failure")
	st.deletePayloadKeys = func(context.Context, string, []string) error { return injected }
	t.Cleanup(func() { st.deletePayloadKeys = nil })

	clientSummary := "new client summary"
	if err := st.UpdatePayload(ctx, cur, nil, &clientSummary); !errors.Is(err, injected) {
		t.Fatalf("UpdatePayload error = %v, want the exact injected error %v", err, injected)
	}

	// Primary mutation COMMITTED despite the DeletePayload failure (documented
	// partial success).
	got, err := st.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Summary != clientSummary || got.SummarySource != SummarySourceClient {
		t.Errorf("primary mutation not committed: summary=%q source=%q", got.Summary, got.SummarySource)
	}
	if got.AccessCount != 1 {
		t.Errorf("AccessCount not bumped despite committed primary mutation: %d", got.AccessCount)
	}

	// Provenance keys remain STALE: the injected failure means DeletePayload
	// never actually ran against Qdrant.
	raw := scrollPoints(t, st.client, st.collection)[id]
	for _, k := range provenanceKeys {
		if _, ok := raw.payload[k]; !ok {
			t.Errorf("expected stale provenance key %q to remain after the injected DeletePayload failure", k)
		}
	}
}

// TestSearchAuthzCallCount proves the bulk recall path calls DecideBucket
// O(buckets-per-request) — at most one per candidate bucket (own, shared) —
// and NEVER a count that scales with the number of stored/returned records
// (SC3, no per-record Cedar evaluation on the hot path). *authz.PDP is a
// sealed concrete type with no exported constructor besides MustDefault, so
// the counting probe is injected via decideBucketHook (a same-package field,
// mirroring the deletePayloadKeys injection above) rather than a custom PDP.
func TestSearchAuthzCallCount(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "authz-test:project:call-count"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	var calls int
	s.decideBucketHook = func(owner, kind string, action authz.Action, bucket authz.Bucket) authz.Decision {
		calls++
		return s.authz.DecideBucket(owner, kind, action, bucket)
	}
	t.Cleanup(func() { s.decideBucketHook = nil })

	const n = 12
	for i := 0; i < n; i++ {
		m := Memory{
			ID: fmt.Sprintf("cccccccc-0000-0000-0000-%012d", i), Content: "x",
			Scope: scope, Owner: "sub-count", CreatedAt: time.Now().UTC(),
		}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}

	calls = 0
	hits, err := s.Search(ctx, scope, Authenticated("sub-count"), []float32{0.1, 0.2, 0.3}, uint64(n), SearchOptions{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != n {
		t.Fatalf("search: got %d hits, want %d", len(hits), n)
	}
	// Exactly one DecideBucket call per candidate bucket (own, shared) — bounded
	// by bucket count, never by the 12 stored/returned records.
	if calls != 2 {
		t.Errorf("Search: DecideBucket called %d times, want 2 (own+shared, not per-record)", calls)
	}
}

// TestBulkFilterOwnAndSharedAdjacency (edge 1) proves a record that is
// simultaneously owner==caller AND visibility=="shared" is returned exactly
// once for the authenticated owner — the own and shared bucket clauses
// compose as Should conditions, never a conflict or a duplicate.
func TestBulkFilterOwnAndSharedAdjacency(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "authz-test:project:adjacency"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	m := Memory{
		ID: "dddddddd-0000-0000-0000-000000000001", Content: "own+shared",
		Scope: scope, Owner: "sub-adj", Visibility: "shared", CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	hits, err := s.Search(ctx, scope, Authenticated("sub-adj"), []float32{0.1, 0.2, 0.3}, 10, SearchOptions{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("Search: got %d hits, want exactly 1 (own+shared record returned once)", len(hits))
	}
	if hits[0].ID != m.ID {
		t.Errorf("Search: got record %q, want %q", hits[0].ID, m.ID)
	}
}

// TestBulkFilterZeroBucketFailsClosed (edge 5) proves that a decision
// allowing zero buckets — an all-deny PDP injected via decideBucketHook —
// compiles to a match-nothing filter, never an unfiltered Qdrant query, for
// an authenticated caller who owns records under the scope.
func TestBulkFilterZeroBucketFailsClosed(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "authz-test:project:zero-bucket"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	m := Memory{
		ID: "eeeeeeee-0000-0000-0000-000000000001", Content: "owned",
		Scope: scope, Owner: "sub-deny", CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	s.decideBucketHook = func(_, _ string, _ authz.Action, _ authz.Bucket) authz.Decision {
		return authz.Decision{Allow: false}
	}
	t.Cleanup(func() { s.decideBucketHook = nil })

	hits, err := s.Search(ctx, scope, Authenticated("sub-deny"), []float32{0.1, 0.2, 0.3}, 10, SearchOptions{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("Search with all-deny PDP: got %d hits, want 0 (fail-closed, never unfiltered)", len(hits))
	}

	lst, _, _, err := s.List(ctx, scope, Authenticated("sub-deny"), ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lst) != 0 {
		t.Errorf("List with all-deny PDP: got %d items, want 0 (fail-closed, never unfiltered)", len(lst))
	}
}

// TestBulkFilterOrderIndependent (edge 6) proves the composed filter result
// is stable regardless of which bucket decision is evaluated first — the
// authz condition stays the outer Must in every composed filter, and the
// authenticated own+shared result is identical whether own or shared is
// decided first.
func TestBulkFilterOrderIndependent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "authz-test:project:order-independent"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	own := Memory{
		ID: "ffffffff-0000-0000-0000-000000000001", Content: "own",
		Scope: scope, Owner: "sub-order", CreatedAt: time.Now().UTC(),
	}
	shared := Memory{
		ID: "ffffffff-0000-0000-0000-000000000002", Content: "shared",
		Scope: scope, Owner: "sub-other-order", Visibility: "shared", CreatedAt: time.Now().UTC(),
	}
	for _, m := range []Memory{own, shared} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", m.ID, err)
		}
	}

	search := func() map[string]bool {
		hits, err := s.Search(ctx, scope, Authenticated("sub-order"), []float32{0.1, 0.2, 0.3}, 10, SearchOptions{})
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		got := make(map[string]bool, len(hits))
		for _, h := range hits {
			got[h.ID] = true
		}
		return got
	}

	// Default (own decided before shared, per s.decideBucket call order).
	want := search()
	if len(want) != 2 || !want[own.ID] || !want[shared.ID] {
		t.Fatalf("baseline: got %v, want {own, shared}", want)
	}

	// Force the SIBLING bucket to be decided first on every call (regardless of
	// the production code's fixed own-then-shared call order), then return the
	// requested bucket's decision. If DecideBucket held any hidden state shared
	// across calls, this interleaved reversal would perturb the result; it must
	// not — decisions are a pure function of (owner, kind, action, bucket).
	s.decideBucketHook = func(owner, kind string, action authz.Action, bucket authz.Bucket) authz.Decision {
		sibling := authz.BucketOwn
		if bucket == authz.BucketOwn {
			sibling = authz.BucketShared
		}
		_ = s.authz.DecideBucket(owner, kind, action, sibling)
		return s.authz.DecideBucket(owner, kind, action, bucket)
	}
	t.Cleanup(func() { s.decideBucketHook = nil })

	got := search()
	if len(got) != len(want) || !got[own.ID] || !got[shared.ID] {
		t.Errorf("order-independence: got %v, want %v (unchanged regardless of decision order)", got, want)
	}
}

// TestCrossSpineAuthzIsolation is the non-vacuous two-owner proof for
// ROADMAP criterion 2 (03-AUTHZ-GATE.md), landed BEFORE the cross-spine
// feature exists (D-18).
//
// It deliberately does NOT go through Store.Search or Store.List. Today
// scope=="" means "the scope payload is literally the empty string", which
// matches essentially nothing — a test that passed an empty scope to either
// method would report "owner B never appeared" because *nothing* appeared,
// which is a vacuous green and proves nothing about authorization
// (03-AUTHZ-GATE.md "The correction"). Instead this test builds the exact
// *qdrant.Filter shape the post-D-05 cross-spine path will produce — a Must
// slice containing ONLY s.ownerOrSharedCondition, no scope element — and
// scrolls it directly. That is not a hypothetical shape: it is byte-for-byte
// the filter Store.ListScopes already runs in production today
// (store.go:1396-1401), so this test pins a composition that is already
// live, not one that will exist only after 03-02 lands.
//
// Both owners seed records under the SAME scope name (D-16): overlap is what
// makes a dropped owner clause visible as a leak of the other owner's
// records, rather than silently returning nothing.
func TestCrossSpineAuthzIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:cross-spine-overlap"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	mk := func(id, owner, vis string) {
		m := Memory{ID: id, Content: "x", Scope: scope, Owner: owner, Visibility: vis,
			CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	const (
		aPrivateID = "c5c50000-0000-0000-0000-000000000001"
		bPrivateID = "c5c50000-0000-0000-0000-000000000002" // must never appear for A
		bSharedID  = "c5c50000-0000-0000-0000-000000000003"
	)
	mk(aPrivateID, "sub-xspine-A", "")      // A private
	mk(bPrivateID, "sub-xspine-B", "")      // B private
	mk(bSharedID, "sub-xspine-B", "shared") // B shared

	// The cross-spine-shaped filter: authz clause only, no scope element.
	f := &qdrant.Filter{Must: []*qdrant.Condition{
		s.ownerOrSharedCondition(ctx, Authenticated("sub-xspine-A")),
	}}

	const limit = 10000
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         f,
		Limit:          qdrant.PtrOf(uint32(limit)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		t.Fatalf("scroll: %v", err)
	}

	// Falsifiability guard: mem_eval_test is a package-shared collection and
	// other tests (e.g. TestListExactTotalPastOldCap) seed thousands of points
	// into it. If the page came back full, "owner B's private record is
	// absent" would be satisfiable by truncation rather than by the authz
	// clause — the same vacuous green in a new costume. Fail loudly instead.
	if len(pts) >= limit {
		t.Fatalf("scroll page filled (%d >= %d limit): cannot conclude anything about exclusion — truncation, not authz, would explain absence", len(pts), limit)
	}

	seen := make(map[string]bool, len(pts))
	for _, p := range pts {
		seen[p.Id.GetUuid()] = true
	}

	if seen[bPrivateID] {
		t.Fatalf("leaked owner B's private record across the cross-spine-shaped filter: %s", bPrivateID)
	}
	if !seen[aPrivateID] {
		t.Errorf("owner A's own private record missing from cross-spine-shaped read: %s", aPrivateID)
	}
	if !seen[bSharedID] {
		t.Errorf("owner B's shared record missing from cross-spine-shaped read: %s", bSharedID)
	}
}

// TestSearchCrossSpine is the WIRING proof for Store.Search's now-conditional
// scope clause (ownerScopeFilter, store.go:752): an empty scope spans every
// scope in the collection the caller may read, and naming a scope still
// confines to it. TestCrossSpineAuthzIsolation (above) is the AUTHZ proof —
// they are not redundant. This test would have been vacuous if written
// before ownerScopeFilter's conditional-scope edit: pre-edit, scope=="" is a
// literal-string match against a payload field no record carries, so an
// empty-scope Search returns zero hits regardless of what else is true.
//
// One owner, two distinct scopes, at least two records per scope (so a
// single dropped record cannot silently collapse the multi-scope
// assertion), every record carrying one tag unique to this test — mem_eval_
// test is a collection the whole package shares (including the 1001 points
// TestListExactTotalPastOldCap seeds), so SearchOptions.Tags narrows the
// results to just this fixture without touching the scope or authz
// conditions it is appended after (store.go:894).
func TestSearchCrossSpine(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	const (
		scopeA     = "iso-test:project:cross-spine-wiring-a"
		scopeB     = "iso-test:project:cross-spine-wiring-b"
		owner      = "sub-xspine-wiring"
		fixtureTag = "xspine-wiring-fixture-7c1d"
	)
	defer func() {
		cleanupErr(t, "DeleteAllRaw "+scopeA, s.DeleteAllRaw(ctx, scopeA))
		cleanupErr(t, "DeleteAllRaw "+scopeB, s.DeleteAllRaw(ctx, scopeB))
	}()

	mk := func(id, scope string) {
		m := Memory{ID: id, Content: "x", Scope: scope, Owner: owner,
			Tags: []string{fixtureTag}, CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	const (
		a1 = "c5c50002-0000-0000-0000-000000000001"
		a2 = "c5c50002-0000-0000-0000-000000000002"
		b1 = "c5c50002-0000-0000-0000-000000000003"
		b2 = "c5c50002-0000-0000-0000-000000000004"
	)
	mk(a1, scopeA)
	mk(a2, scopeA)
	mk(b1, scopeB)
	mk(b2, scopeB)

	opts := SearchOptions{Tags: []string{fixtureTag}}
	subj := Authenticated(owner)

	// Empty scope spans both scopes.
	hits, err := s.Search(ctx, "", subj, []float32{0.1, 0.2, 0.3}, 10, opts)
	if err != nil {
		t.Fatalf("cross-spine search: %v", err)
	}
	gotScopes := map[string]bool{}
	gotIDs := map[string]bool{}
	for _, h := range hits {
		gotScopes[h.Scope] = true
		gotIDs[h.ID] = true
	}
	if len(gotScopes) <= 1 {
		t.Fatalf("cross-spine search spans %d distinct scope(s), want >1: %v", len(gotScopes), gotScopes)
	}
	for _, id := range []string{a1, a2, b1, b2} {
		if !gotIDs[id] {
			t.Errorf("cross-spine search missing %s", id)
		}
	}

	// Naming scopeA still confines to it.
	scoped, err := s.Search(ctx, scopeA, subj, []float32{0.1, 0.2, 0.3}, 10, opts)
	if err != nil {
		t.Fatalf("scope-confined search: %v", err)
	}
	scopedScopes := map[string]bool{}
	scopedIDs := map[string]bool{}
	for _, h := range scoped {
		scopedScopes[h.Scope] = true
		scopedIDs[h.ID] = true
	}
	if len(scopedScopes) != 1 || !scopedScopes[scopeA] {
		t.Errorf("scope-confined search: distinct scopes = %v, want exactly {%s}", scopedScopes, scopeA)
	}
	if scopedIDs[b1] || scopedIDs[b2] {
		t.Errorf("scope-confined search leaked scopeB records: %v", scopedIDs)
	}
}

// TestListCrossSpine is Store.List's wiring proof, the list analog of
// TestSearchCrossSpine: listFilter's now-conditional scope clause spans every
// scope in the collection the owner may read when scope=="", and naming a
// scope still confines to it. One owner, two distinct scopes, one fixture tag
// unique to this test (mem_eval_test is package-shared).
func TestListCrossSpine(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	const (
		scopeA     = "iso-test:project:cross-spine-list-wiring-a"
		scopeB     = "iso-test:project:cross-spine-list-wiring-b"
		owner      = "sub-xspine-list-wiring"
		fixtureTag = "xspine-list-wiring-fixture-4e9a"
	)
	defer func() {
		cleanupErr(t, "DeleteAllRaw "+scopeA, s.DeleteAllRaw(ctx, scopeA))
		cleanupErr(t, "DeleteAllRaw "+scopeB, s.DeleteAllRaw(ctx, scopeB))
	}()

	mk := func(id, scope string) {
		m := Memory{ID: id, Content: "x", Scope: scope, Owner: owner,
			Tags: []string{fixtureTag}, CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	const (
		a1 = "c5c50005-0000-0000-0000-000000000001"
		a2 = "c5c50005-0000-0000-0000-000000000002"
		b1 = "c5c50005-0000-0000-0000-000000000003"
		b2 = "c5c50005-0000-0000-0000-000000000004"
	)
	mk(a1, scopeA)
	mk(a2, scopeA)
	mk(b1, scopeB)
	mk(b2, scopeB)

	opts := ListOptions{Limit: 10000, Tags: []string{fixtureTag}}
	subj := Authenticated(owner)

	// Empty scope spans both scopes.
	items, _, _, err := s.List(ctx, "", subj, opts)
	if err != nil {
		t.Fatalf("cross-spine list: %v", err)
	}
	gotScopes := map[string]bool{}
	gotIDs := map[string]bool{}
	for _, m := range items {
		gotScopes[m.Scope] = true
		gotIDs[m.ID] = true
	}
	if len(gotScopes) <= 1 {
		t.Fatalf("cross-spine list spans %d distinct scope(s), want >1: %v", len(gotScopes), gotScopes)
	}
	for _, id := range []string{a1, a2, b1, b2} {
		if !gotIDs[id] {
			t.Errorf("cross-spine list missing %s", id)
		}
	}

	// Naming scopeA still confines to it.
	scoped, _, _, err := s.List(ctx, scopeA, subj, opts)
	if err != nil {
		t.Fatalf("scope-confined list: %v", err)
	}
	scopedScopes := map[string]bool{}
	scopedIDs := map[string]bool{}
	for _, m := range scoped {
		scopedScopes[m.Scope] = true
		scopedIDs[m.ID] = true
	}
	if len(scopedScopes) != 1 || !scopedScopes[scopeA] {
		t.Errorf("scope-confined list: distinct scopes = %v, want exactly {%s}", scopedScopes, scopeA)
	}
	if scopedIDs[b1] || scopedIDs[b2] {
		t.Errorf("scope-confined list leaked scopeB records: %v", scopedIDs)
	}
}

// TestListCrossSpineTotal pins D-09: a cross-spine List's total is the exact
// server-side Count across every readable scope, strictly greater than the
// scope-confined total. Uses its own fixture tag and owner (distinct from
// TestListCrossSpine's) so the two tests' exact-count assertions cannot
// contaminate each other against the package-shared collection.
func TestListCrossSpineTotal(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	const (
		scopeA     = "iso-test:project:cross-spine-list-total-a"
		scopeB     = "iso-test:project:cross-spine-list-total-b"
		owner      = "sub-xspine-list-total"
		fixtureTag = "xspine-list-total-fixture-1a7c"
	)
	defer func() {
		cleanupErr(t, "DeleteAllRaw "+scopeA, s.DeleteAllRaw(ctx, scopeA))
		cleanupErr(t, "DeleteAllRaw "+scopeB, s.DeleteAllRaw(ctx, scopeB))
	}()

	mk := func(id, scope string) {
		m := Memory{ID: id, Content: "x", Scope: scope, Owner: owner,
			Tags: []string{fixtureTag}, CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	// 3 records in scopeA, 2 in scopeB — 5 total, tagged so the count is
	// deterministic against the shared collection.
	for i := 0; i < 3; i++ {
		mk(fmt.Sprintf("c5c50006-0000-0000-0000-00000000000%d", i+1), scopeA)
	}
	for i := 0; i < 2; i++ {
		mk(fmt.Sprintf("c5c50006-0000-0000-0000-00000000001%d", i+1), scopeB)
	}

	opts := ListOptions{Limit: 1, Tags: []string{fixtureTag}}
	subj := Authenticated(owner)

	_, crossTotal, _, err := s.List(ctx, "", subj, opts)
	if err != nil {
		t.Fatalf("cross-spine list: %v", err)
	}
	if crossTotal != 5 {
		t.Errorf("cross-spine total = %d, want exact 5 (3 in scopeA + 2 in scopeB)", crossTotal)
	}

	_, scopedTotal, _, err := s.List(ctx, scopeA, subj, opts)
	if err != nil {
		t.Fatalf("scope-confined list: %v", err)
	}
	if scopedTotal != 3 {
		t.Errorf("scope-confined total = %d, want exact 3", scopedTotal)
	}
	if crossTotal <= scopedTotal {
		t.Errorf("cross-spine total (%d) must be strictly greater than scope-confined total (%d)", crossTotal, scopedTotal)
	}
}

// TestGetReadableDenyMapsToNotFound proves a Cedar Deny on an id-addressed
// gate is indistinguishable from a genuinely missing id: even though the
// record EXISTS and is owned by the caller, an all-deny decideRecordHook
// forces GetReadable to return the exact same fmt.Errorf ErrNotFound form
// used for an absent id (DEC-xa6) — the error carries no policy-id/reason
// text and its message equals the plain missing-id form, proving the
// authz.Decision's unexported Diagnostic never leaks into the caller-facing
// error (SC4, T-22-08). *authz.PDP is a sealed concrete type with no
// exported constructor besides MustDefault, so the all-deny probe is
// injected via decideRecordHook (mirroring decideBucketHook), not a custom
// *authz.PDP built through WithAuthz.
func TestGetReadableDenyMapsToNotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "authz-test:project:record-deny"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	m := Memory{
		ID: "11111111-2222-0000-0000-000000000001", Content: "owned",
		Scope: scope, Owner: "sub-record-deny", CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	s.decideRecordHook = func(_, _ string, _ authz.Action, _, _, _, _ string) authz.Decision {
		return authz.Decision{Allow: false}
	}
	t.Cleanup(func() { s.decideRecordHook = nil })

	_, err := s.GetReadable(ctx, m.ID, Authenticated("sub-record-deny"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetReadable with all-deny PDP: want ErrNotFound, got %v", err)
	}
	want := fmt.Errorf("%w: %s", ErrNotFound, m.ID).Error()
	if err.Error() != want {
		t.Errorf("GetReadable error leaked non-uniform content: got %q, want %q (plain missing-id form, no Diagnostic)", err.Error(), want)
	}
}

// TestGetWritableAndOwnedOrAbsentDenyMapsToNotFound is the write-path analogue
// of TestGetReadableDenyMapsToNotFound (WR-02): it proves the same D-10
// Deny->ErrNotFound uniformity holds for getWritable and OwnedOrAbsent on an
// owned, EXISTING record — not just the real-PDP cross-owner path exercised by
// TestDeleteOwnerGate/TestSetVisibilityOwnerGate/TestUpdateOwnerGateAndSharedFlag,
// and not just the absent-id short-circuit covered by
// TestIdAddressedAbsentShortCircuit. Under the current policy corpus this
// branch (an owner denied by Cedar on their own record) is unreachable in
// production — own_records permits every action unconditionally for the
// owner — but the injected all-deny decideRecordHook forces it, guarding
// against a future write-path change that accidentally threads the
// Diagnostic into the caller-facing error.
func TestGetWritableAndOwnedOrAbsentDenyMapsToNotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "authz-test:project:write-record-deny"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	m := Memory{
		ID: "11111111-2222-0000-0000-000000000002", Content: "owned",
		Scope: scope, Owner: "sub-write-record-deny", CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	s.decideRecordHook = func(_, _ string, _ authz.Action, _, _, _, _ string) authz.Decision {
		return authz.Decision{Allow: false}
	}
	t.Cleanup(func() { s.decideRecordHook = nil })

	want := fmt.Errorf("%w: %s", ErrNotFound, m.ID).Error()

	_, err := s.getWritable(ctx, m.ID, Authenticated("sub-write-record-deny"), authz.ActionWrite)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("getWritable with all-deny PDP: want ErrNotFound, got %v", err)
	}
	if err.Error() != want {
		t.Errorf("getWritable error leaked non-uniform content: got %q, want %q (plain missing-id form, no Diagnostic)", err.Error(), want)
	}

	err = s.OwnedOrAbsent(ctx, m.ID, Authenticated("sub-write-record-deny"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("OwnedOrAbsent with all-deny PDP: want ErrNotFound, got %v", err)
	}
	if err.Error() != want {
		t.Errorf("OwnedOrAbsent error leaked non-uniform content: got %q, want %q (plain missing-id form, no Diagnostic)", err.Error(), want)
	}
}

// TestDeleteAllDeniedBucketDeletesNothing (WR-04) covers the new bucket-denial
// branch the WR-01 fix added to DeleteAll: when decideBucket(ActionDelete,
// BucketOwn) returns Deny, DeleteAll must return nil (nothing to delete)
// rather than silently deleting or erroring. Under the current policy corpus
// this branch is unreachable in production (own_records permits delete
// unconditionally for the owner) — same accepted-risk class as the sibling
// denial tests (TestBulkFilterZeroBucketFailsClosed via decideBucketHook) —
// but it guards against a future edit that flips the nil-return to something
// that silently deletes on Deny.
func TestDeleteAllDeniedBucketDeletesNothing(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "authz-test:project:deleteall-deny"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	m := Memory{
		ID: "abababab-0000-0000-0000-000000000001", Content: "owned",
		Scope: scope, Owner: "sub-deleteall-deny", CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	s.decideBucketHook = func(_, _ string, _ authz.Action, _ authz.Bucket) authz.Decision {
		return authz.Decision{Allow: false}
	}
	t.Cleanup(func() { s.decideBucketHook = nil })

	if err := s.DeleteAll(ctx, scope, Authenticated("sub-deleteall-deny")); err != nil {
		t.Fatalf("DeleteAll with all-deny PDP: want nil (nothing to delete), got %v", err)
	}
	if _, err := s.Get(ctx, m.ID); err != nil {
		t.Errorf("DeleteAll with all-deny PDP must not delete: record gone, %v", err)
	}
}

// TestIdAddressedAbsentShortCircuit proves the s.Get -> ErrNotFound
// short-circuit precedes DecideRecord for every id-addressed gate: even
// under an all-deny PDP, an id that does NOT exist yields the SAME
// GetReadable/getWritable ErrNotFound and OwnedOrAbsent's absent->nil
// contract, because Cedar is never consulted for a record that was never
// fetched (Pattern 4, T-22-10). A Deny-everything decideRecordHook must not
// change the absent-id contract.
func TestIdAddressedAbsentShortCircuit(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.decideRecordHook = func(_, _ string, _ authz.Action, _, _, _, _ string) authz.Decision {
		return authz.Decision{Allow: false}
	}
	t.Cleanup(func() { s.decideRecordHook = nil })

	const missing = "11111111-2222-0000-0000-00000000dead"

	if _, err := s.GetReadable(ctx, missing, Authenticated("sub-absent")); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetReadable absent id under all-deny PDP: want ErrNotFound, got %v", err)
	}
	if _, err := s.getWritable(ctx, missing, Authenticated("sub-absent"), authz.ActionWrite); !errors.Is(err, ErrNotFound) {
		t.Errorf("getWritable absent id under all-deny PDP: want ErrNotFound, got %v", err)
	}
	if err := s.OwnedOrAbsent(ctx, missing, Authenticated("sub-absent")); err != nil {
		t.Errorf("OwnedOrAbsent absent id under all-deny PDP: want nil, got %v", err)
	}
}

// --- Archive/Restore (D-12, plan 03-06) ---

// TestActiveWindowConditionsExcludesArchivedAt (D-12) pins that the
// archived_at soft-hide is a SIBLING of activeWindowConditions, never folded
// into it: the helper's returned conditions must never reference
// archived_at. Pure unit test — no Qdrant round-trip — asserted directly on
// the helper's return value rather than a source grep any comment could
// satisfy or defeat.
func TestActiveWindowConditionsExcludesArchivedAt(t *testing.T) {
	conds := activeWindowConditions(time.Now())
	if len(conds) == 0 {
		t.Fatal("activeWindowConditions returned no conditions")
	}
	for _, c := range conds {
		if strings.Contains(c.String(), "archived_at") {
			t.Fatalf("activeWindowConditions condition references archived_at, want it excluded entirely: %s", c.String())
		}
	}
}

// TestArchiveRecallGateSearchAndList (T-03-03's mitigation) mirrors
// TestSupersedeRecallGate: an archived record is excluded from Search and
// List but stays fetchable, content intact, via Get.
func TestArchiveRecallGateSearchAndList(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "archive-test:project:recall-search-list"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")
	vec := []float32{0.1, 0.2, 0.3}

	archivedID := "e1000000-0000-0000-0000-000000000001"
	liveID := "e1000000-0000-0000-0000-000000000002"
	if err := s.Upsert(ctx, Memory{ID: archivedID, Content: "old", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}, vec); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := s.Archive(ctx, archivedID); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if err := s.Upsert(ctx, Memory{ID: liveID, Content: "live", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}, vec); err != nil {
		t.Fatalf("upsert live: %v", err)
	}

	hits, err := s.Search(ctx, scope, subj, vec, 10, SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if got := recordIDs(hits); slices.Contains(got, archivedID) {
		t.Errorf("Search: archived record %s present, want excluded: %v", archivedID, got)
	} else if !slices.Contains(got, liveID) {
		t.Errorf("Search: live record %s absent, want present: %v", liveID, got)
	}

	items, _, _, err := s.List(ctx, scope, subj, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := recordIDs(items); slices.Contains(got, archivedID) {
		t.Errorf("List: archived record %s present, want excluded: %v", archivedID, got)
	} else if !slices.Contains(got, liveID) {
		t.Errorf("List: live record %s absent, want present: %v", liveID, got)
	}

	got, err := s.Get(ctx, archivedID)
	if err != nil {
		t.Fatalf("Get archived: %v", err)
	}
	if got.ArchivedAt == nil {
		t.Errorf("Get archived: ArchivedAt = nil, want set (Get is never gated)")
	}
	if got.Content != "old" {
		t.Errorf("Get archived: content = %q, want %q (untouched)", got.Content, "old")
	}
}

// TestArchiveRecallGateIncludeArchived (phase 07, D-01/D-02) proves
// ListOptions.IncludeArchived is a REVEAL of the existing archived_at hide
// gate — an archived record becomes reachable via Store.List when the flag
// is set, a live record in the same scope stays reachable alongside it, the
// flag maps 1:1 onto exactly the archived_at condition (it does not reveal a
// superseded or windowed-inactive record), and authorization stays
// orthogonal to state (D-04): it never reveals another owner's private
// record.
func TestArchiveRecallGateIncludeArchived(t *testing.T) {
	s := testStore(t)
	fixed := time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixed }
	ctx := context.Background()
	scope := "archive-test:project:include-archived"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")
	vec := []float32{0.1, 0.2, 0.3}
	future := fixed.Add(24 * time.Hour)

	archivedID := "e3000000-0000-0000-0000-000000000001"
	liveID := "e3000000-0000-0000-0000-000000000002"
	supersededID := "e3000000-0000-0000-0000-000000000003"
	supersessorID := "e3000000-0000-0000-0000-000000000004"
	scheduledID := "e3000000-0000-0000-0000-000000000005"
	otherOwnerID := "e3000000-0000-0000-0000-000000000006"

	if err := s.Upsert(ctx, Memory{ID: archivedID, Content: "archived", Scope: scope, Owner: "sub-A", CreatedAt: fixed}, vec); err != nil {
		t.Fatalf("upsert archived: %v", err)
	}
	if _, err := s.Archive(ctx, archivedID); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if err := s.Upsert(ctx, Memory{ID: liveID, Content: "live", Scope: scope, Owner: "sub-A", CreatedAt: fixed}, vec); err != nil {
		t.Fatalf("upsert live: %v", err)
	}
	if err := s.Upsert(ctx, Memory{ID: supersededID, Content: "old", Scope: scope, Owner: "sub-A", CreatedAt: fixed, SupersededBy: &supersessorID}, vec); err != nil {
		t.Fatalf("upsert superseded: %v", err)
	}
	if err := s.Upsert(ctx, Memory{ID: scheduledID, Content: "future", Scope: scope, Owner: "sub-A", CreatedAt: fixed, NotBefore: &future}, vec); err != nil {
		t.Fatalf("upsert scheduled: %v", err)
	}
	if err := s.Upsert(ctx, Memory{ID: otherOwnerID, Content: "not mine", Scope: scope, Owner: "sub-B", CreatedAt: fixed}, vec); err != nil {
		t.Fatalf("upsert other-owner: %v", err)
	}
	if _, err := s.Archive(ctx, otherOwnerID); err != nil {
		t.Fatalf("Archive other-owner: %v", err)
	}

	// ListOptions{} (default, zero value): archived record absent — today's
	// behavior, unchanged.
	items, _, _, err := s.List(ctx, scope, subj, ListOptions{Limit: 20})
	if err != nil {
		t.Fatalf("List default: %v", err)
	}
	if got := recordIDs(items); slices.Contains(got, archivedID) {
		t.Errorf("List default: archived record %s present, want excluded: %v", archivedID, got)
	}

	// ListOptions{IncludeArchived: true}: the archived record is present, a
	// live record in the same scope is still present, and the flag maps 1:1
	// onto exactly one gate — it does not reveal superseded or
	// windowed-inactive records, and it never reveals another owner's
	// private record (authz stays orthogonal, D-04).
	items, _, _, err = s.List(ctx, scope, subj, ListOptions{Limit: 20, IncludeArchived: true})
	if err != nil {
		t.Fatalf("List IncludeArchived: %v", err)
	}
	got := recordIDs(items)
	if !slices.Contains(got, archivedID) {
		t.Errorf("List IncludeArchived: archived record %s absent, want present: %v", archivedID, got)
	}
	if !slices.Contains(got, liveID) {
		t.Errorf("List IncludeArchived: live record %s absent, want present: %v", liveID, got)
	}
	if slices.Contains(got, supersededID) {
		t.Errorf("List IncludeArchived: superseded record %s present, want excluded: %v", supersededID, got)
	}
	if slices.Contains(got, scheduledID) {
		t.Errorf("List IncludeArchived: windowed-inactive record %s present, want excluded: %v", scheduledID, got)
	}
	if slices.Contains(got, otherOwnerID) {
		t.Errorf("List IncludeArchived: other owner's private record %s present, want excluded (D-04): %v", otherOwnerID, got)
	}
}

// TestListIncludeSupersededAndScheduled (phase 07, D-01/D-02) completes the
// three-flag opt-in proof on the List lane: IncludeSuperseded and
// IncludeScheduled each map 1:1 onto their own gate condition,
// IncludeScheduled relaxes BOTH halves of activeWindowConditions together
// (proven by one call returning both a not-yet-active and an already-expired
// record), the flags compose (archived AND superseded revealed together only
// when both flags are set), and the all-false default path is unchanged.
func TestListIncludeSupersededAndScheduled(t *testing.T) {
	s := testStore(t)
	fixed := time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixed }
	ctx := context.Background()
	scope := "list-test:project:include-superseded-scheduled"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")
	vec := []float32{0.1, 0.2, 0.3}
	future := fixed.Add(24 * time.Hour)
	past := fixed.Add(-24 * time.Hour)

	liveID := "e4000000-0000-0000-0000-000000000001"
	supersededID := "e4000000-0000-0000-0000-000000000002"
	supersessorID := "e4000000-0000-0000-0000-000000000003"
	archivedID := "e4000000-0000-0000-0000-000000000004"
	notYetActiveID := "e4000000-0000-0000-0000-000000000005"
	expiredID := "e4000000-0000-0000-0000-000000000006"
	bothArchivedAndSupersededID := "e4000000-0000-0000-0000-000000000007"

	if err := s.Upsert(ctx, Memory{ID: liveID, Content: "live", Scope: scope, Owner: "sub-A", CreatedAt: fixed}, vec); err != nil {
		t.Fatalf("upsert live: %v", err)
	}
	if err := s.Upsert(ctx, Memory{ID: supersededID, Content: "old", Scope: scope, Owner: "sub-A", CreatedAt: fixed, SupersededBy: &supersessorID}, vec); err != nil {
		t.Fatalf("upsert superseded: %v", err)
	}
	if err := s.Upsert(ctx, Memory{ID: archivedID, Content: "archived", Scope: scope, Owner: "sub-A", CreatedAt: fixed}, vec); err != nil {
		t.Fatalf("upsert archived: %v", err)
	}
	if _, err := s.Archive(ctx, archivedID); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if err := s.Upsert(ctx, Memory{ID: notYetActiveID, Content: "future", Scope: scope, Owner: "sub-A", CreatedAt: fixed, NotBefore: &future}, vec); err != nil {
		t.Fatalf("upsert not-yet-active: %v", err)
	}
	if err := s.Upsert(ctx, Memory{ID: expiredID, Content: "expired", Scope: scope, Owner: "sub-A", CreatedAt: fixed, NotAfter: &past}, vec); err != nil {
		t.Fatalf("upsert expired: %v", err)
	}
	if err := s.Upsert(ctx, Memory{
		ID: bothArchivedAndSupersededID, Content: "both", Scope: scope, Owner: "sub-A",
		CreatedAt: fixed, SupersededBy: &supersessorID,
	}, vec); err != nil {
		t.Fatalf("upsert both: %v", err)
	}
	if _, err := s.Archive(ctx, bothArchivedAndSupersededID); err != nil {
		t.Fatalf("Archive both: %v", err)
	}

	// IncludeSuperseded reveals ONLY the superseded record, not the archived
	// one.
	items, _, _, err := s.List(ctx, scope, subj, ListOptions{Limit: 20, IncludeSuperseded: true})
	if err != nil {
		t.Fatalf("List IncludeSuperseded: %v", err)
	}
	got := recordIDs(items)
	if !slices.Contains(got, supersededID) {
		t.Errorf("List IncludeSuperseded: superseded record %s absent, want present: %v", supersededID, got)
	}
	if slices.Contains(got, archivedID) {
		t.Errorf("List IncludeSuperseded: archived record %s present, want excluded: %v", archivedID, got)
	}

	// IncludeScheduled reveals BOTH halves of the window in ONE call: the
	// not-yet-active record AND the already-expired record. It does not
	// reveal archived or superseded records.
	items, _, _, err = s.List(ctx, scope, subj, ListOptions{Limit: 20, IncludeScheduled: true})
	if err != nil {
		t.Fatalf("List IncludeScheduled: %v", err)
	}
	got = recordIDs(items)
	if !slices.Contains(got, notYetActiveID) {
		t.Errorf("List IncludeScheduled: not-yet-active record %s absent, want present: %v", notYetActiveID, got)
	}
	if !slices.Contains(got, expiredID) {
		t.Errorf("List IncludeScheduled: expired record %s absent, want present: %v", expiredID, got)
	}
	if slices.Contains(got, archivedID) {
		t.Errorf("List IncludeScheduled: archived record %s present, want excluded: %v", archivedID, got)
	}
	if slices.Contains(got, supersededID) {
		t.Errorf("List IncludeScheduled: superseded record %s present, want excluded: %v", supersededID, got)
	}

	// The flags compose: a record that is both archived and superseded is
	// returned only when BOTH flags are set.
	items, _, _, err = s.List(ctx, scope, subj, ListOptions{Limit: 20, IncludeArchived: true})
	if err != nil {
		t.Fatalf("List IncludeArchived only: %v", err)
	}
	if got := recordIDs(items); slices.Contains(got, bothArchivedAndSupersededID) {
		t.Errorf("List IncludeArchived only: archived+superseded record %s present, want excluded (still superseded-hidden): %v", bothArchivedAndSupersededID, got)
	}
	items, _, _, err = s.List(ctx, scope, subj, ListOptions{Limit: 20, IncludeSuperseded: true})
	if err != nil {
		t.Fatalf("List IncludeSuperseded only: %v", err)
	}
	if got := recordIDs(items); slices.Contains(got, bothArchivedAndSupersededID) {
		t.Errorf("List IncludeSuperseded only: archived+superseded record %s present, want excluded (still archived-hidden): %v", bothArchivedAndSupersededID, got)
	}
	items, _, _, err = s.List(ctx, scope, subj, ListOptions{Limit: 20, IncludeArchived: true, IncludeSuperseded: true})
	if err != nil {
		t.Fatalf("List IncludeArchived+IncludeSuperseded: %v", err)
	}
	if got := recordIDs(items); !slices.Contains(got, bothArchivedAndSupersededID) {
		t.Errorf("List IncludeArchived+IncludeSuperseded: archived+superseded record %s absent, want present (both flags compose): %v", bothArchivedAndSupersededID, got)
	}

	// Default path (all three false): exactly the live records, matching
	// today's behavior over this same corpus.
	items, _, _, err = s.List(ctx, scope, subj, ListOptions{Limit: 20})
	if err != nil {
		t.Fatalf("List default: %v", err)
	}
	got = recordIDs(items)
	if !slices.Contains(got, liveID) {
		t.Errorf("List default: live record %s absent, want present: %v", liveID, got)
	}
	for _, hidden := range []string{supersededID, archivedID, notYetActiveID, expiredID, bothArchivedAndSupersededID} {
		if slices.Contains(got, hidden) {
			t.Errorf("List default: hidden record %s present, want excluded: %v", hidden, got)
		}
	}
}

// TestArchiveRecallGateSearchDiscovery mirrors
// TestSearchDiscoverySupersededHidden for the archived_at soft-hide.
func TestArchiveRecallGateSearchDiscovery(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "discovery:repo:archive-recall-gate"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	vec := []float32{0.1, 0.2, 0.3}

	archivedID := "e1000000-0000-0000-0000-000000000003"
	liveID := "e1000000-0000-0000-0000-000000000004"
	if err := s.Upsert(ctx, Memory{
		ID: archivedID, Content: "old discovery", Scope: scope, Category: "discovery",
		Kind: "fact", Owner: "sub-A", CreatedAt: time.Now().UTC(),
	}, vec); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := s.Archive(ctx, archivedID); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if err := s.Upsert(ctx, Memory{
		ID: liveID, Content: "live discovery", Scope: scope, Category: "discovery",
		Kind: "fact", Owner: "sub-A", CreatedAt: time.Now().UTC(),
	}, vec); err != nil {
		t.Fatalf("upsert live: %v", err)
	}

	hits, err := s.SearchDiscovery(ctx, scope, "", Authenticated("sub-A"), vec, 10)
	if err != nil {
		t.Fatalf("SearchDiscovery: %v", err)
	}
	if got := recordIDs(hits); slices.Contains(got, archivedID) {
		t.Errorf("SearchDiscovery: archived record %s present, want excluded: %v", archivedID, got)
	} else if !slices.Contains(got, liveID) {
		t.Errorf("SearchDiscovery: live record %s absent, want present: %v", liveID, got)
	}
}

// TestArchiveRecallGateListScheduled mirrors TestListScheduledSupersededHidden
// for the archived_at soft-hide.
func TestArchiveRecallGateListScheduled(t *testing.T) {
	s := testStore(t)
	fixed := time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixed }
	ctx := context.Background()
	scope := "sched-test:project:archive-recall-gate"
	subj := Authenticated("sub-A")
	future := fixed.Add(24 * time.Hour)

	archivedID := "e1000000-0000-0000-0000-000000000005"
	liveID := "e1000000-0000-0000-0000-000000000006"
	mk := func(id string) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A", CreatedAt: fixed, NotBefore: &future}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, subj)) })
	}
	mk(archivedID)
	mk(liveID)
	if _, err := s.Archive(ctx, archivedID); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	sched, err := s.ListScheduled(ctx, scope, subj, ScheduledPending, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListScheduled: %v", err)
	}
	if got := recordIDs(sched); slices.Contains(got, archivedID) {
		t.Errorf("ListScheduled: archived record %s present, want excluded: %v", archivedID, got)
	} else if !slices.Contains(got, liveID) {
		t.Errorf("ListScheduled: live record %s absent, want present: %v", liveID, got)
	}
}

// TestArchivedAndSupersededHideIndependently (T-03-03's mitigation) pins that
// a record carrying BOTH superseded_by and archived_at stays hidden from
// recall while EITHER condition holds, and resurfaces only once BOTH are
// cleared — proving the two soft-hides are independently maintained, not a
// single combined flag one Restore call alone could clear.
func TestArchivedAndSupersededHideIndependently(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "archive-test:project:both-states"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")
	vec := []float32{0.1, 0.2, 0.3}

	id := "e2000000-0000-0000-0000-000000000001"
	newID := "e2000000-0000-0000-0000-000000000002"
	if err := s.Upsert(ctx, Memory{
		ID: id, Content: "v1", Scope: scope, Owner: "sub-A",
		CreatedAt: time.Now().UTC(), SupersededBy: &newID,
	}, vec); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := s.Archive(ctx, id); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	assertHidden := func(t *testing.T, why string) {
		t.Helper()
		items, _, _, err := s.List(ctx, scope, subj, ListOptions{Limit: 10})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if got := recordIDs(items); slices.Contains(got, id) {
			t.Errorf("List: record %s present (%s), want excluded", id, why)
		}
	}
	assertHidden(t, "both superseded_by and archived_at set")

	// Restore clears archived_at; supersession alone still hides it.
	if _, err := s.Restore(ctx, id); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	assertHidden(t, "superseded_by still set after Restore")

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ArchivedAt != nil {
		t.Errorf("Get after Restore: ArchivedAt = %v, want nil", got.ArchivedAt)
	}
	if got.SupersededBy == nil {
		t.Fatalf("Get after Restore: SupersededBy = nil, want still set")
	}

	// Re-archive while still superseded, then clear supersession alone (no
	// "un-supersede" verb exists; a direct targeted delete stands in) to
	// prove the REVERSE order too: clearing superseded_by alone must not
	// resurface a still-archived record.
	if _, err := s.Archive(ctx, id); err != nil {
		t.Fatalf("re-Archive: %v", err)
	}
	assertHidden(t, "archived_at set again, superseded_by still set")

	if err := s.defaultDeletePayloadKeys(ctx, id, []string{"superseded_by"}); err != nil {
		t.Fatalf("clear superseded_by: %v", err)
	}
	assertHidden(t, "archived_at still set after clearing superseded_by")

	if _, err := s.Restore(ctx, id); err != nil {
		t.Fatalf("final Restore: %v", err)
	}
	items, _, _, err := s.List(ctx, scope, subj, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := recordIDs(items); !slices.Contains(got, id) {
		t.Errorf("List: record %s absent after both conditions cleared, want present: %v", id, got)
	}
}

// TestArchiveIdempotent pins that a second Archive on an already-archived
// record reports ArchiveOutcomeAlready and leaves the original stamp
// unchanged — idempotent by value, not by re-stamping.
func TestArchiveIdempotent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "archive-test:project:idempotent"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	id := "e4000000-0000-0000-0000-000000000001"
	if err := s.Upsert(ctx, Memory{ID: id, Content: "v", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	res1, err := s.Archive(ctx, id)
	if err != nil {
		t.Fatalf("first Archive: %v", err)
	}
	if res1.Outcome != ArchiveOutcomeChanged {
		t.Fatalf("first Archive: Outcome = %q, want %q", res1.Outcome, ArchiveOutcomeChanged)
	}
	first, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after first Archive: %v", err)
	}
	if first.ArchivedAt == nil {
		t.Fatalf("Get after first Archive: ArchivedAt = nil, want set")
	}

	res2, err := s.Archive(ctx, id)
	if err != nil {
		t.Fatalf("second Archive: %v", err)
	}
	if res2.Outcome != ArchiveOutcomeAlready {
		t.Errorf("second Archive: Outcome = %q, want %q", res2.Outcome, ArchiveOutcomeAlready)
	}
	second, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after second Archive: %v", err)
	}
	if !second.ArchivedAt.Equal(*first.ArchivedAt) {
		t.Errorf("second Archive changed the stamp: first=%v second=%v", first.ArchivedAt, second.ArchivedAt)
	}
}

// TestRestoreNoOpWhenNeverArchived pins that Restore on a never-archived
// record reports ArchiveOutcomeAlready and mutates nothing.
func TestRestoreNoOpWhenNeverArchived(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "archive-test:project:restore-noop"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	id := "e4000000-0000-0000-0000-000000000002"
	if err := s.Upsert(ctx, Memory{ID: id, Content: "v", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	res, err := s.Restore(ctx, id)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if res.Outcome != ArchiveOutcomeAlready {
		t.Errorf("Restore never-archived: Outcome = %q, want %q", res.Outcome, ArchiveOutcomeAlready)
	}
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Content != "v" {
		t.Errorf("Restore never-archived mutated content: got %q, want %q", got.Content, "v")
	}
	if got.ArchivedAt != nil {
		t.Errorf("Restore never-archived: ArchivedAt = %v, want nil", got.ArchivedAt)
	}
}

// TestArchiveUnknownID and TestRestoreUnknownID pin that both verbs report
// the SAME not-found-class error and the SAME ArchiveOutcomeNotFound value
// for an id that does not exist — never a silent success on either side
// (T-03-29's mitigation).
func TestArchiveUnknownID(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	res, err := s.Archive(ctx, "ffffffff-ffff-ffff-ffff-ffffffffffff")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Archive(unknown): err = %v, want ErrNotFound-class", err)
	}
	if res.Outcome != ArchiveOutcomeNotFound {
		t.Errorf("Archive(unknown): Outcome = %q, want %q", res.Outcome, ArchiveOutcomeNotFound)
	}
}

func TestRestoreUnknownID(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	res, err := s.Restore(ctx, "ffffffff-ffff-ffff-ffff-ffffffffffff")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Restore(unknown): err = %v, want ErrNotFound-class", err)
	}
	if res.Outcome != ArchiveOutcomeNotFound {
		t.Errorf("Restore(unknown): Outcome = %q, want %q", res.Outcome, ArchiveOutcomeNotFound)
	}
}

// TestArchiveSurvivesWholePayloadUpdate pins that a record archived and then
// updated through Store.Update's whole-payload Upsert path is still
// archived afterwards — the sequential half of T-03-17's mitigation.
func TestArchiveSurvivesWholePayloadUpdate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "archive-test:project:survives-update"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")

	id := "e3000000-0000-0000-0000-000000000001"
	if err := s.Upsert(ctx, Memory{ID: id, Content: "v1", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := s.Archive(ctx, id); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	cur, err := s.FetchForUpdate(ctx, id, subj)
	if err != nil {
		t.Fatalf("FetchForUpdate: %v", err)
	}
	if cur.ArchivedAt == nil {
		t.Fatalf("precondition: FetchForUpdate.ArchivedAt = nil, want set")
	}
	if err := s.Update(ctx, cur, "v2", nil, nil, nil, []float32{0.2, 0.3, 0.4}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ArchivedAt == nil {
		t.Errorf("Get after Update: ArchivedAt = nil, want still set (whole-payload Upsert must preserve it)")
	}
	if got.Content != "v2" {
		t.Errorf("Get after Update: Content = %q, want %q", got.Content, "v2")
	}
}

// TestArchiveSurvivesConcurrentUpdate (T-03-17's mitigation) proves Update's
// whole-payload Upsert cannot erase a concurrent Archive, via a
// DETERMINISTIC barrier-controlled interleaving through the
// updateAfterReadHook seam — a single iteration, never a repeated
// unsynchronized race. The hook fires inside Update's lock, after its
// in-lock re-read and before its Upsert; the test holds Update there until
// Archive either completes or a 2s bound expires, then asserts the record
// is still archived. Run with -race.
func TestArchiveSurvivesConcurrentUpdate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "archive-test:project:concurrent-update"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")

	id := "e5000000-0000-0000-0000-000000000001"
	if err := s.Upsert(ctx, Memory{ID: id, Content: "v1", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	cur, err := s.FetchForUpdate(ctx, id, subj)
	if err != nil {
		t.Fatalf("FetchForUpdate: %v", err)
	}
	if cur.ArchivedAt != nil {
		t.Fatalf("precondition: cur.ArchivedAt = %v, want nil before the race", cur.ArchivedAt)
	}

	inWindow := make(chan struct{})
	archiveDone := make(chan struct{})
	updateAfterReadHook = func() {
		close(inWindow)
		select {
		case <-archiveDone:
		case <-time.After(2 * time.Second):
		}
	}
	t.Cleanup(func() { updateAfterReadHook = nil })

	var wg sync.WaitGroup
	var updateErr, archiveErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		updateErr = s.Update(ctx, cur, "v2", nil, nil, nil, []float32{0.2, 0.3, 0.4})
	}()
	<-inWindow

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, archiveErr = s.Archive(ctx, id)
		close(archiveDone)
	}()
	wg.Wait()

	if updateErr != nil {
		t.Fatalf("Update: %v", updateErr)
	}
	if archiveErr != nil {
		t.Fatalf("Archive: %v", archiveErr)
	}
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ArchivedAt == nil {
		t.Fatalf("ArchivedAt = nil, want set (Update must not erase a concurrent Archive under the same-lock implementation)")
	}
	if got.Content != "v2" {
		t.Errorf("Content = %q, want %q (Update's content edit must still land)", got.Content, "v2")
	}
}

// TestRestoreSurvivesConcurrentUpdate is TestArchiveSurvivesConcurrentUpdate's
// mirror: an already-archived record, racing Restore against Update via the
// same deterministic barrier, must end up NOT archived.
func TestRestoreSurvivesConcurrentUpdate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "archive-test:project:restore-concurrent-update"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	subj := Authenticated("sub-A")

	id := "e5000000-0000-0000-0000-000000000002"
	if err := s.Upsert(ctx, Memory{ID: id, Content: "v1", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := s.Archive(ctx, id); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	cur, err := s.FetchForUpdate(ctx, id, subj)
	if err != nil {
		t.Fatalf("FetchForUpdate: %v", err)
	}
	if cur.ArchivedAt == nil {
		t.Fatalf("precondition: cur.ArchivedAt = nil, want set before the race")
	}

	inWindow := make(chan struct{})
	restoreDone := make(chan struct{})
	updateAfterReadHook = func() {
		close(inWindow)
		select {
		case <-restoreDone:
		case <-time.After(2 * time.Second):
		}
	}
	t.Cleanup(func() { updateAfterReadHook = nil })

	var wg sync.WaitGroup
	var updateErr, restoreErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		updateErr = s.Update(ctx, cur, "v2", nil, nil, nil, []float32{0.2, 0.3, 0.4})
	}()
	<-inWindow

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, restoreErr = s.Restore(ctx, id)
		close(restoreDone)
	}()
	wg.Wait()

	if updateErr != nil {
		t.Fatalf("Update: %v", updateErr)
	}
	if restoreErr != nil {
		t.Fatalf("Restore: %v", restoreErr)
	}
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ArchivedAt != nil {
		t.Fatalf("ArchivedAt = %v, want nil (Restore must complete after Update releases the lock)", got.ArchivedAt)
	}
}
