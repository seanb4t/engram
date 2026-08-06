// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package e2e

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os/exec"
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
// internal/store's unexported test helpers.
func newSpineReviewStore(t *testing.T, collection string) *store.Store {
	t.Helper()
	if testQdrantAddr == "" {
		skipOrFailNoQdrant(t)
	}
	c := spineReviewQdrantClient(t)
	_ = c.DeleteCollection(context.Background(), collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })
	s := store.New(c, collection)
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

// TestE2EPruneExpiredPreviewsBeforeApply is REQ-destructive-preview-default's
// end-to-end proof against the BUILT binary (review finding #18: no operator
// command had ANY end-to-end coverage in this package before this test —
// internal/e2e execed only serve/search/list). It seeds one expired record
// directly through internal/store, execs `engram prune-expired` with no
// --apply and asserts BY RE-READING THE COLLECTION that the record is still
// present — never merely that the process exited 0 — execs again with
// --apply=false and asserts the same plus byte-identical stdout, then execs
// with --apply and asserts the record is gone.
func TestE2EPruneExpiredPreviewsBeforeApply(t *testing.T) {
	if testQdrantAddr == "" {
		skipOrFailNoQdrant(t)
	}
	const collection = "e2e_prune_expired"
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
	if stdout1 != stdout2 {
		t.Errorf("bare vs --apply=false stdout differ:\nbare:          %q\n--apply=false: %q", stdout1, stdout2)
	}

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
	const collection = "e2e_prune_expired_empty"
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
