// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"

	"github.com/seanb4t/engram/internal/store"
)

// runCLIWithEnv runs the built engram binary with args and extraEnv merged
// over the hermetic PATH/HOME baseline. runCLI's own childEnv(nil) baseline
// carries no ENGRAM_* vars at all, so it can only prove the pre-dial
// usage-error path (cli_exitcode_test.go's own callers); this variant is what
// lets a test point the binary at the ephemeral Qdrant this package's
// TestMain already brought up, so the destructive-command flip can be
// proven against a REAL collection rather than only against the built
// binary's pre-dial validation.
func runCLIWithEnv(t *testing.T, env map[string]string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, engramBin, args...)
	cmd.Env = childEnv(env)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("engram %v hung\nstdout:\n%s\nstderr:\n%s", args, outBuf.String(), errBuf.String())
	}
	if err == nil {
		return outBuf.String(), errBuf.String(), 0
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("engram %v: non-exit error: %v", args, err)
	}
	return outBuf.String(), errBuf.String(), exitErr.ExitCode()
}

// spineReviewQdrantClient dials the SAME ephemeral Qdrant the built-binary
// subprocess targets, over a bare client — used to seed a fixture record and
// to re-read the collection Subject-less (store.Store.Get), proving the
// preview path really left the record untouched rather than merely inferring
// it from the subprocess's exit code (memory dnanmnkqmg: green tests on the
// apply path alone are not evidence the preview path is safe).
func spineReviewQdrantClient(t *testing.T) *qdrant.Client {
	t.Helper()
	host, portStr, err := net.SplitHostPort(testQdrantAddr)
	if err != nil {
		t.Fatalf("invalid Qdrant address %q: %v", testQdrantAddr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("invalid Qdrant port %q (from %q): %v", portStr, testQdrantAddr, err)
	}
	c, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: port})
	if err != nil {
		t.Fatalf("qdrant client: %v", err)
	}
	return c
}

// newSpineReviewStore returns a Store over a fresh, per-test collection on
// the shared ephemeral Qdrant, deleted before (in case a prior run left it
// behind) and after (via t.Cleanup) — mirroring internal/store's own
// newSpineTestStore, reimplemented here since this package cannot import
// internal/store's unexported test helpers. collection must already carry
// this package's testCollectionPrefix (via testCollection()) — callers pass
// the prefixed value directly, rather than this helper applying the prefix
// itself, because the SAME value is also handed to pruneEnv for the built
// binary's ENGRAM_QDRANT_COLLECTION; the Go-side store and the subprocess
// must agree on one literal collection name.
func newSpineReviewStore(t *testing.T, collection string) *store.Store {
	t.Helper()
	if testQdrantAddr == "" {
		skipOrFailNoQdrant(t)
	}
	c := spineReviewQdrantClient(t)
	_ = c.DeleteCollection(context.Background(), collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })
	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(context.Background(), 3); err != nil {
		t.Fatalf("ensure collection %q: %v", collection, err)
	}
	return s
}

// pruneEnv builds the childEnv map StoreFromEnv (internal/server/tools.go)
// needs to dial collection on the shared ephemeral Qdrant. embed.* fields
// carry registry defaults StoreFromEnv's cfg.Validate() is satisfied by —
// prune-expired never calls the embedder, so no real OpenAI-compatible
// endpoint is needed.
func pruneEnv(collection string) map[string]string {
	return map[string]string{
		"ENGRAM_QDRANT_ADDR":       testQdrantAddr,
		"ENGRAM_QDRANT_COLLECTION": collection,
		"ENGRAM_EMBED_DIM":         "3",
	}
}

// assertSamePreview asserts that a bare preview run and an explicit
// --apply=false run made the SAME decision — every field byte-identical except
// the one that legitimately cannot be.
//
// `before` is the expiry cutoff, computed as wall-clock now() at each
// invocation and serialized at SECOND precision. The two runs are separated by
// a round trip to Qdrant, so whenever they straddle a second boundary the
// cutoffs differ by 1s. Comparing raw stdout therefore made this assertion a
// coin flip: green whenever both landed inside the same second, red otherwise
// — and it went red in CI on a tree where `go test ./...` was green locally.
//
// Dropping `before` from the comparison is not the same as ignoring it: it is
// still required to be present and RFC3339-parseable in both, so a regression
// that omitted or corrupted the cutoff still fails here. What is deliberately
// NOT asserted is that two different invocations observed the same instant.
func assertSamePreview(t *testing.T, bare, applyFalse string) {
	t.Helper()

	decode := func(label, out string) map[string]any {
		var m map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &m); err != nil {
			t.Fatalf("%s stdout is not JSON: %v\nstdout: %q", label, err, out)
		}
		cutoff, ok := m["before"].(string)
		if !ok {
			t.Fatalf("%s stdout carries no string %q cutoff: %q", label, "before", out)
		}
		if _, err := time.Parse(time.RFC3339, cutoff); err != nil {
			t.Fatalf("%s %q cutoff is not RFC3339: %v (%q)", label, "before", err, cutoff)
		}
		delete(m, "before")
		return m
	}

	gotBare, gotApplyFalse := decode("bare", bare), decode("--apply=false", applyFalse)
	if !reflect.DeepEqual(gotBare, gotApplyFalse) {
		t.Errorf("bare vs --apply=false differ outside the time-varying cutoff:\nbare:          %v\n--apply=false: %v\n(raw bare: %q)\n(raw --apply=false: %q)",
			gotBare, gotApplyFalse, bare, applyFalse)
	}
}

// TestE2EPruneExpiredPreviewsBeforeApply is REQ-destructive-preview-default's
// end-to-end proof against the BUILT binary (review finding #18: no operator
// command had ANY end-to-end coverage in this package before this test —
// internal/e2e execed only serve/search/list). It seeds one expired record
// directly through internal/store, execs `engram prune-expired` with no
// --apply and asserts BY RE-READING THE COLLECTION that the record is still
// present — never merely that the process exited 0 — execs again with
// --apply=false and asserts the same plus an equivalent preview verdict (see
// assertSamePreview), then execs with --apply and asserts the record is gone.
func TestE2EPruneExpiredPreviewsBeforeApply(t *testing.T) {
	if testQdrantAddr == "" {
		skipOrFailNoQdrant(t)
	}
	collection := testCollection("prune_expired")
	s := newSpineReviewStore(t, collection)
	ctx := context.Background()

	past := time.Now().UTC().Add(-time.Hour)
	const id = "d0000000-0000-0000-0000-000000000001"
	if err := s.Upsert(ctx, store.Memory{
		ID: id, Content: "expired", Scope: "e2e_prune_scope", Category: "note",
		Owner: "e2e-owner", CreatedAt: time.Now().UTC(), NotAfter: &past,
	}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed expired record: %v", err)
	}

	env := pruneEnv(collection)

	// Bare invocation: preview only, record survives.
	stdout1, stderr, code := runCLIWithEnv(t, env, "prune-expired")
	if code != 0 {
		t.Fatalf("prune-expired (bare) exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout1, stderr)
	}
	if _, err := s.Get(ctx, id); err != nil {
		t.Fatalf("record missing after a BARE (preview) prune-expired run: %v — the preview path must never delete", err)
	}

	// --apply=false: the value-not-supplied distinction — same as bare,
	// record survives, output byte-identical.
	stdout2, stderr, code := runCLIWithEnv(t, env, "prune-expired", "--apply=false")
	if code != 0 {
		t.Fatalf("prune-expired --apply=false exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout2, stderr)
	}
	if _, err := s.Get(ctx, id); err != nil {
		t.Fatalf("record missing after --apply=false: %v — --apply=false must behave exactly like an omitted flag", err)
	}
	assertSamePreview(t, stdout1, stdout2)

	// --apply: performs the deletion.
	stdout3, stderr, code := runCLIWithEnv(t, env, "prune-expired", "--apply")
	if code != 0 {
		t.Fatalf("prune-expired --apply exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout3, stderr)
	}
	if _, err := s.Get(ctx, id); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("record still present after --apply: err=%v, want %v", err, store.ErrNotFound)
	}
}

// TestE2EPruneExpiredPreviewZeroEligible is the empty-spine preview case: a
// preview run against a collection with no eligible records reports a zero
// count and exits 0.
func TestE2EPruneExpiredPreviewZeroEligible(t *testing.T) {
	if testQdrantAddr == "" {
		skipOrFailNoQdrant(t)
	}
	collection := testCollection("prune_expired_empty")
	newSpineReviewStore(t, collection)

	// --output text forces the human sentence regardless of the subprocess's
	// non-TTY stdout (which would otherwise default to json).
	stdout, stderr, code := runCLIWithEnv(t, pruneEnv(collection), "prune-expired", "--output", "text")
	if code != 0 {
		t.Fatalf("prune-expired exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "0 expired record") {
		t.Errorf("preview against an empty spine = %q, want it to report 0 eligible records", stdout)
	}
}

// seedFiller upserts a minimal, throwaway record used only to give scan/
// consolidate a real multi-page, multi-owner population to sweep — never
// touched by this test's own targeted assertions.
func seedFiller(t *testing.T, s *store.Store, id, scope, owner string) {
	t.Helper()
	// A tiny per-record vector perturbation so consolidate's near-duplicate
	// sweep does not treat all 270 filler records as one giant duplicate
	// cluster -- irrelevant to this test's assertions, which never inspect
	// consolidate's candidate list, only its exit code and unchanged-store
	// property.
	v := float32(len(id)%7) / 10.0
	if err := s.Upsert(context.Background(), store.Memory{
		ID: id, Content: "filler", Scope: scope, Category: "note",
		Owner: owner, CreatedAt: time.Now().UTC(),
	}, []float32{0.1 + v, 0.2, 0.3 - v}); err != nil {
		t.Fatalf("seed filler %s: %v", id, err)
	}
}

// fillerID deterministically derives a valid-shaped UUID from prefix and i --
// decimal digits are a subset of hex digits, so a zero-padded decimal
// counter is a valid (if unconventional) UUID suffix.
func fillerID(prefix string, i int) string {
	return fmt.Sprintf("%s000000-0000-4000-8000-%012d", prefix, i)
}

// verifyRepoScope derives a "repo:<identity>" scope segment matching what
// `spine-review verify` (cmd/engram/spine_review_verify.go's
// repoIdentityFromCWD/normalizeGitRemote) derives from THIS process's git
// origin remote -- so a citation seeded under this scope is classified
// "broken" (same repo, file genuinely missing) rather than "unverifiable"
// (different repo). A minimal, test-local duplicate of that normalisation
// rather than an import: cmd/engram's function is unexported and this
// package cannot reach it.
func verifyRepoScope(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		t.Skipf("no git origin remote available to derive a same-repo verify scope: %v", err)
	}
	remote := strings.TrimSpace(string(out))
	if i := strings.Index(remote, "://"); i >= 0 {
		remote = remote[i+3:]
	}
	if i := strings.Index(remote, "@"); i >= 0 {
		remote = remote[i+1:]
	}
	if i := strings.Index(remote, ":"); i >= 0 {
		remote = remote[:i] + "/" + remote[i+1:]
	}
	remote = strings.TrimSuffix(remote, ".git")
	remote = strings.TrimSuffix(remote, "/")
	return "repo:" + remote
}

// TestE2EPhaseAcceptance is the phase's ONE end-to-end acceptance run
// (03-07-PLAN.md Task 3, review finding #18 / Codex cross-plan): it execs
// the BUILT binary against a seeded multi-page, multi-owner collection and
// exercises all seven ROADMAP success criteria in one flow -- scan, verify,
// consolidate, archive then restore, prune-expired preview then --apply,
// and spine-review purge preview then --apply. Before this task,
// internal/e2e execed only serve/search/list (pre-phase) and prune-expired
// (plan 03-03); no other spine-review leaf had end-to-end coverage against
// the BUILT binary at all.
//
// Coverage map (which assertion below covers which of the seven ROADMAP
// success criteria):
//  1. scan (inventory/health signals)        -> the scan exit-0 + Owners>=2 assertion
//  2. verify (tiers, --fail-on)               -> the broken-citation + --fail-on exitFindings assertion
//  3. consolidate (ranked pairs)              -> the consolidate exit-0 + unchanged-store assertion
//  4. archive/restore                         -> the archive-then-restore round trip
//  5. prune-expired preview/apply             -> the prune preview-then-apply block
//  6. spine-review purge preview/apply        -> the purge preview-then-apply block
//  7. REQ-purge-extract-gated (this plan)     -> the purge block's extract-gate-satisfied path
//     (the superseded class self-satisfies its own gate, 03-07-PLAN.md's
//     named property)
func TestE2EPhaseAcceptance(t *testing.T) {
	if testQdrantAddr == "" {
		skipOrFailNoQdrant(t)
	}
	collection := testCollection("phase_acceptance")
	s := newSpineReviewStore(t, collection)
	ctx := context.Background()
	env := pruneEnv(collection)

	const fillerScopeA = "e2e_phase_acceptance_scope_a"
	const fillerScopeB = "e2e_phase_acceptance_scope_b"

	// 1. Seed a MULTI-PAGE (> one Qdrant scroll page; scrollAllPoints'
	// default batch is 256), multi-owner (2 distinct owners) population, so
	// scan's Owners count and consolidate's pagination claim are exercised
	// against real data rather than assumed.
	for i := 0; i < 200; i++ {
		seedFiller(t, s, fillerID("70", i), fillerScopeA, "owner-a")
	}
	for i := 0; i < 70; i++ {
		seedFiller(t, s, fillerID("71", i), fillerScopeB, "owner-b")
	}

	// --- Criterion 1: scan ---
	stdout, stderr, code := runCLIWithEnv(t, env, "spine-review", "scan", "--all-scopes", "--output", "json")
	if code != 0 {
		t.Fatalf("spine-review scan exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, `"total":270`) {
		t.Errorf("spine-review scan --output json = %q, want it to report total:270", stdout)
	}

	// --- Criterion 2: verify (tiers + --fail-on exitFindings) ---
	verifyScope := verifyRepoScope(t)
	brokenID := "72000000-0000-4000-8000-000000000001"
	if err := s.Upsert(ctx, store.Memory{
		ID: brokenID, Content: "cites a file that does not exist", Scope: verifyScope, Category: "decision",
		Owner: "owner-a", CreatedAt: time.Now().UTC(),
		Citations: []store.Citation{{Kind: "file", Ref: "this-file-does-not-exist.go", Excerpt: "func x() {}", Locator: "1"}},
	}, []float32{0.4, 0.4, 0.4}); err != nil {
		t.Fatalf("seed broken-citation record: %v", err)
	}
	stdout, stderr, code = runCLIWithEnv(t, env, "spine-review", "verify", "--scope", verifyScope, "--output", "json")
	if code != 0 {
		t.Fatalf("spine-review verify exit = %d, want 0 (no --fail-on)\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, `"broken":1`) {
		t.Errorf("spine-review verify --output json = %q, want broken:1", stdout)
	}
	_, stderr, code = runCLIWithEnv(t, env, "spine-review", "verify", "--scope", verifyScope, "--fail-on", "broken")
	if code != exitFindingsForTest {
		t.Errorf("spine-review verify --fail-on broken exit = %d, want %d (exitFindings)\nstderr:\n%s", code, exitFindingsForTest, stderr)
	}

	// --- Criterion 3: consolidate ---
	stdout, stderr, code = runCLIWithEnv(t, env, "spine-review", "consolidate", "--all-scopes", "--output", "json")
	if code != 0 {
		t.Fatalf("spine-review consolidate exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	// --- Criterion 4: archive / restore ---
	const archiveScope = "e2e_phase_acceptance_archive_scope"
	archiveID := "73000000-0000-4000-8000-000000000001"
	if err := s.Upsert(ctx, store.Memory{
		ID: archiveID, Content: "archive me", Scope: archiveScope, Category: "decision",
		Owner: "owner-a", CreatedAt: time.Now().UTC(),
	}, []float32{0.5, 0.5, 0.5}); err != nil {
		t.Fatalf("seed archive fixture: %v", err)
	}
	_, stderr, code = runCLIWithEnv(t, env, "spine-review", "archive", "--id", archiveID, "--output", "json")
	if code != 0 {
		t.Fatalf("spine-review archive exit = %d, want 0\nstderr:\n%s", code, stderr)
	}
	if got, gerr := s.Get(ctx, archiveID); gerr != nil || got.ArchivedAt == nil {
		t.Fatalf("record not archived after spine-review archive: err=%v archivedAt=%v", gerr, got.ArchivedAt)
	}
	_, stderr, code = runCLIWithEnv(t, env, "spine-review", "restore", "--id", archiveID, "--output", "json")
	if code != 0 {
		t.Fatalf("spine-review restore exit = %d, want 0\nstderr:\n%s", code, stderr)
	}
	if got, gerr := s.Get(ctx, archiveID); gerr != nil || got.ArchivedAt != nil {
		t.Fatalf("record still archived after spine-review restore: err=%v archivedAt=%v", gerr, got.ArchivedAt)
	}

	// --- Criterion 5: prune-expired preview then --apply ---
	const pruneScope = "e2e_phase_acceptance_prune_scope"
	past := time.Now().UTC().Add(-time.Hour)
	pruneID := "74000000-0000-4000-8000-000000000001"
	if err := s.Upsert(ctx, store.Memory{
		ID: pruneID, Content: "expired", Scope: pruneScope, Category: "decision",
		Owner: "owner-a", CreatedAt: time.Now().UTC(), NotAfter: &past,
	}, []float32{0.6, 0.6, 0.6}); err != nil {
		t.Fatalf("seed prune fixture: %v", err)
	}
	_, stderr, code = runCLIWithEnv(t, env, "prune-expired", "--output", "json")
	if code != 0 {
		t.Fatalf("prune-expired (preview) exit = %d, want 0\nstderr:\n%s", code, stderr)
	}
	if _, gerr := s.Get(ctx, pruneID); gerr != nil {
		t.Fatalf("record missing after a BARE prune-expired preview: %v", gerr)
	}
	_, stderr, code = runCLIWithEnv(t, env, "prune-expired", "--apply", "--output", "json")
	if code != 0 {
		t.Fatalf("prune-expired --apply exit = %d, want 0\nstderr:\n%s", code, stderr)
	}
	if _, gerr := s.Get(ctx, pruneID); !errors.Is(gerr, store.ErrNotFound) {
		t.Fatalf("record still present after prune-expired --apply: err=%v, want %v", gerr, store.ErrNotFound)
	}

	// --- Criteria 6 & 7: spine-review purge preview then --apply, gated on
	// the extract-before-delete precondition (REQ-purge-extract-gated). The
	// superseded class self-satisfies its own gate (03-07-PLAN.md's named
	// property): the successor IS the extraction link's target.
	const purgeScope = "e2e_phase_acceptance_purge_scope"
	successorID := "75000000-0000-4000-8000-000000000002"
	targetID := "75000000-0000-4000-8000-000000000001"
	if err := s.Upsert(ctx, store.Memory{
		ID: successorID, Content: "successor", Scope: purgeScope, Category: "decision",
		Owner: "owner-a", CreatedAt: time.Now().UTC().Add(-time.Minute),
	}, []float32{0.7, 0.7, 0.7}); err != nil {
		t.Fatalf("seed purge successor fixture: %v", err)
	}
	if err := s.Upsert(ctx, store.Memory{
		ID: targetID, Content: "superseded", Scope: purgeScope, Category: "decision",
		Owner: "owner-a", CreatedAt: time.Now().UTC().Add(-2 * time.Minute), SupersededBy: &successorID,
	}, []float32{0.75, 0.75, 0.75}); err != nil {
		t.Fatalf("seed purge target fixture: %v", err)
	}
	stdout, stderr, code = runCLIWithEnv(t, env, "spine-review", "purge", "--class", "superseded", "--scope", purgeScope, "--output", "json")
	if code != 0 {
		t.Fatalf("spine-review purge (preview) exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, targetID) {
		t.Errorf("spine-review purge preview = %q, want it to name the eligible target %s", stdout, targetID)
	}
	if _, gerr := s.Get(ctx, targetID); gerr != nil {
		t.Fatalf("target missing after a BARE spine-review purge preview: %v", gerr)
	}
	stdout, stderr, code = runCLIWithEnv(t, env, "spine-review", "purge", "--class", "superseded", "--scope", purgeScope, "--apply", "--output", "json")
	if code != 0 {
		t.Fatalf("spine-review purge --apply exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if _, gerr := s.Get(ctx, targetID); !errors.Is(gerr, store.ErrNotFound) {
		t.Fatalf("target still present after spine-review purge --apply: err=%v, want %v", gerr, store.ErrNotFound)
	}
	if _, gerr := s.Get(ctx, successorID); gerr != nil {
		t.Errorf("successor was deleted by purge (it was never a candidate): %v", gerr)
	}
}

// exitFindingsForTest mirrors cmd/engram's unexported exitFindings constant
// (client_common.go) -- this package cannot import cmd/engram's internal
// constants, so the value is pinned here and cross-checked against the
// pinned catalog.golden fixture by cmd/engram's own TestCatalogGolden.
const exitFindingsForTest = 7
