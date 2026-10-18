// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package e2e

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/seanb4t/engram/internal/store"
)

// TestPrimeRecallGateAndIsolation runs the built binary's `engram prime`
// in a git repository against a real `engram serve` (headless Connect,
// two static-token principals) and a real store: archived, superseded,
// expired and not-yet-active records stay out of the digest, another
// owner's shared record appears and their private one does not.
func TestPrimeRecallGateAndIsolation(t *testing.T) {
	const (
		spine     = "repo:github.com/org/repo"
		ruleScope = "rule:repo:github.com/org/repo"
	)
	collection := testCollection("prime")
	c := spineReviewQdrantClient(t)
	_ = c.DeleteCollection(context.Background(), collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })
	srv := startServer(t, map[string]string{
		"ENGRAM_QDRANT_COLLECTION":          collection,
		"ENGRAM_CONNECT_HEADLESS":           "true",
		"ENGRAM_SERVICE_AUTH_STATIC_TOKENS": "me=tok-me,other=tok-other",
	})

	// A static-token principal's owner is namespaced by its lane
	// (internal/auth namespacedOwner).
	me, other := "12:static_token:2:me", "12:static_token:5:other"
	s := newTestStore(t, c, collection)
	now := time.Now().UTC()
	past, future := now.Add(-time.Hour), now.Add(24*time.Hour)
	liveID := fmt.Sprintf("e1000000-0000-4000-8000-%012d", 0)
	seeds := []store.Memory{
		{ID: liveID, Summary: "live-mine", Owner: me},
		{Summary: "archived-mine", Owner: me, ArchivedAt: &past},
		{Summary: "superseded-mine", Owner: me, SupersededBy: &liveID},
		{Summary: "expired-mine", Owner: me, NotAfter: &past},
		{Summary: "scheduled-mine", Owner: me, NotBefore: &future},
		{Summary: "shared-other", Owner: other, Visibility: "shared"},
		{Summary: "private-other", Owner: other},
		{Summary: "rule-mine", Owner: me, Scope: ruleScope, Category: "rule", Visibility: "shared"},
	}
	for i, m := range seeds {
		if m.ID == "" {
			m.ID = fmt.Sprintf("e1000000-0000-4000-8000-%012d", i)
		}
		m.ShortID = fmt.Sprintf("e2eprime%02d", i)
		m.Content = m.Summary
		m.Actor = m.Owner
		m.Scope = cmp.Or(m.Scope, spine)
		m.Category = cmp.Or(m.Category, "decision")
		m.CreatedAt = now.Add(-time.Duration(len(seeds)-i) * time.Minute)
		if err := s.Upsert(context.Background(), m, unitVector(m.ID, 1024)); err != nil {
			t.Fatalf("seed %s: %v", m.Summary, err)
		}
	}

	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitEnv := []string{"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1"}
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "https://github.com/org/repo.git"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), gitEnv...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, engramBin, "prime", "--server", srv.baseURL(), "--output", "json")
	cmd.Dir = repo
	cmd.Env = append(childEnv(map[string]string{"ENGRAM_TOKEN": "tok-me"}), gitEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("engram prime: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	var doc struct {
		Spine struct {
			Scope  string `json:"scope"`
			Source string `json:"source"`
		} `json:"spine"`
		Rules    []struct{ Summary string } `json:"rules"`
		Memories struct {
			Spine []struct{ Summary string } `json:"spine"`
		} `json:"memories"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("decode primer: %v\n%s", err, stdout.String())
	}
	if doc.Spine.Scope != spine || doc.Spine.Source != "git_origin_remote" {
		t.Errorf("spine = %+v, want %s from git_origin_remote", doc.Spine, spine)
	}
	got := make([]string, 0, len(doc.Memories.Spine))
	for _, m := range doc.Memories.Spine {
		got = append(got, m.Summary)
	}
	slices.Sort(got)
	if want := []string{"live-mine", "shared-other"}; !slices.Equal(got, want) {
		t.Errorf("digest summaries = %v, want %v", got, want)
	}
	if len(doc.Rules) != 1 || doc.Rules[0].Summary != "rule-mine" {
		t.Errorf("rules = %+v, want the one seeded rule", doc.Rules)
	}
}
