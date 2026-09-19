// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves the byte-budget sweep primitive (scrollAllPoints's
// readView extension, boundedread.go) against both oversized fixture shapes
// storetest.SeedOversized supports, at the named storetest.RecvLimit — OUR
// request shape and OUR response-size accounting, via a recording
// interceptor chained inside the classifier NewQdrantClient installs (rule
// m45p2b4bp7: never asserting grpc-go's or Qdrant's own behavior as the test
// oracle).
package store_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/qdrant/go-client/qdrant"
)

// scrollRecordedCall is one intercepted Scroll RPC's request Limit and
// outcome, as scrollRecorder.intercept observed it.
type scrollRecordedCall struct {
	limit     uint32
	respBytes int
	code      codes.Code
}

// scrollRecorder is a mutex-guarded recording grpc.UnaryClientInterceptor,
// scoped to methods ending "/Scroll" (both ScrollAndOffset and Scroll issue
// this same RPC method — the interfaces block's own finding). reset/snapshot
// let a subtest isolate the calls belonging to its own scenario.
type scrollRecorder struct {
	mu    sync.Mutex
	calls []scrollRecordedCall
}

func (r *scrollRecorder) intercept(
	ctx context.Context, method string, req, reply any,
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
) error {
	if len(method) < len("/Scroll") || method[len(method)-len("/Scroll"):] != "/Scroll" {
		return invoker(ctx, method, req, reply, cc, opts...)
	}
	var limit uint32
	if sp, ok := req.(*qdrant.ScrollPoints); ok {
		limit = sp.GetLimit()
	}
	err := invoker(ctx, method, req, reply, cc, opts...)
	call := scrollRecordedCall{limit: limit}
	if err != nil {
		call.code = status.Code(err)
	} else if msg, ok := reply.(proto.Message); ok {
		call.respBytes = proto.Size(msg)
	}
	r.mu.Lock()
	r.calls = append(r.calls, call)
	r.mu.Unlock()
	return err
}

func (r *scrollRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = nil
}

func (r *scrollRecorder) snapshot() []scrollRecordedCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]scrollRecordedCall, len(r.calls))
	copy(out, r.calls)
	return out
}

// TestScrollAllPointsByteBudget runs one subtest per fixture shape
// (few-large, many-small), each dialing with a chained recording
// interceptor, seeding once, then sweeping both the full view and the
// summary view and asserting: every seeded id is visited exactly once, the
// recorder saw at least one Scroll call, every recorded request Limit
// equals store.SweepLimit(view), no recorded response exceeds
// store.RPCByteBudget(), no call ended ResourceExhausted, and the summary
// view's points carry neither content nor citations.
func TestScrollAllPointsByteBudget(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		shape := shape
		t.Run(shape.String(), func(t *testing.T) {
			rec := &scrollRecorder{}
			c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(rec.intercept))
			name := store.PrefixedTestCollection("oversized_boundedread_" + uuid.NewString())
			st := store.NewTestStore(t, c, name)
			ctx := context.Background()
			if err := st.EnsureCollection(ctx, 3); err != nil {
				t.Fatalf("EnsureCollection: %v", err)
			}
			t.Cleanup(func() {
				if err := c.DeleteCollection(ctx, name); err != nil {
					t.Errorf("DeleteCollection(%q): %v", name, err)
				}
			})

			fx := storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3}})

			// Pin the default per-view counts once (D-02/D-06's derived
			// values at DefaultRecordCaps()).
			if got := store.SweepLimit(st.FullView()); got != 2 {
				t.Errorf("SweepLimit(FullView()) = %d, want 2", got)
			}
			if got := store.SweepLimit(st.SummaryView()); got != 61 {
				t.Errorf("SweepLimit(SummaryView()) = %d, want 61", got)
			}

			subtests := []struct {
				name string
				view store.ReadView
			}{
				{"full", st.FullView()},
				{"summary", st.SummaryView()},
			}
			for _, sub := range subtests {
				sub := sub
				t.Run(sub.name, func(t *testing.T) {
					rec.reset()
					visited := map[string]int{}
					scanErr := st.ScrollAllPoints(ctx,
						&qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("scope", fx.Scope)}},
						sub.view,
						func(p *qdrant.RetrievedPoint) error {
							visited[p.Id.GetUuid()]++
							if sub.name == "summary" {
								if _, ok := p.Payload["content"]; ok {
									t.Errorf("summary view point %s carries content", p.Id.GetUuid())
								}
								if _, ok := p.Payload["citations"]; ok {
									t.Errorf("summary view point %s carries citations", p.Id.GetUuid())
								}
							}
							return nil
						})
					if scanErr != nil {
						t.Fatalf("ScrollAllPoints: %v", scanErr)
					}
					if len(visited) != len(fx.IDs) {
						t.Errorf("visited %d distinct ids, want %d", len(visited), len(fx.IDs))
					}
					for _, id := range fx.IDs {
						if visited[id] != 1 {
							t.Errorf("id %s visited %d time(s), want exactly 1", id, visited[id])
						}
					}

					calls := rec.snapshot()
					if len(calls) == 0 {
						t.Fatal("recorder observed zero Scroll calls")
					}
					wantLimit := uint32(store.SweepLimit(sub.view))
					for i, call := range calls {
						if call.limit != wantLimit {
							t.Errorf("call %d: Limit = %d, want %d", i, call.limit, wantLimit)
						}
						if call.code == codes.ResourceExhausted {
							t.Errorf("call %d: ended ResourceExhausted", i)
						}
						if call.respBytes > store.RPCByteBudget() {
							t.Errorf("call %d: response %d bytes exceeds RPCByteBudget %d", i, call.respBytes, store.RPCByteBudget())
						}
					}
				})
			}
		})
	}
}

// TestScrollAllPointsBatchOfOneFallback proves D-07's batch-of-1 fallback:
// three legacy over-cap records (written via a raw Store.Upsert, bypassing
// the internal/server content cap entirely — this package has none of its
// own) each carrying storetest.RecvLimit*5/8 bytes of content. A full-view
// sweep's first RPC (the computed byte-derived Limit) overflows; the
// fallback re-reads the SAME window one record at a time; the sweep
// completes with nil error, visiting every seeded id exactly once.
func TestScrollAllPointsBatchOfOneFallback(t *testing.T) {
	rec := &scrollRecorder{}
	c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(rec.intercept))
	name := store.PrefixedTestCollection("oversized_boundedread_fallback_" + uuid.NewString())
	st := store.NewTestStore(t, c, name)
	ctx := context.Background()
	if err := st.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	t.Cleanup(func() {
		if err := c.DeleteCollection(ctx, name); err != nil {
			t.Errorf("DeleteCollection(%q): %v", name, err)
		}
	})

	scope := "storetest-fallback:" + uuid.NewString()
	owner := "storetest-owner-" + uuid.NewString()
	base := time.Now().UTC().Truncate(time.Second)
	legacyContentBytes := storetest.RecvLimit * 5 / 8

	ids := make([]string, 3)
	for i := range ids {
		m := store.Memory{
			ID:        uuid.NewString(),
			Content:   strings.Repeat("x", legacyContentBytes),
			Scope:     scope,
			Owner:     owner,
			Actor:     owner,
			Category:  "decision",
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		}
		if err := st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert legacy record %d: %v", i, err)
		}
		ids[i] = m.ID
	}

	rec.reset()
	visited := map[string]int{}
	scanErr := st.ScrollAllPoints(ctx,
		&qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("scope", scope)}},
		st.FullView(),
		func(p *qdrant.RetrievedPoint) error {
			visited[p.Id.GetUuid()]++
			return nil
		})
	if scanErr != nil {
		t.Fatalf("ScrollAllPoints: %v", scanErr)
	}
	for _, id := range ids {
		if visited[id] != 1 {
			t.Errorf("id %s visited %d time(s), want exactly 1", id, visited[id])
		}
	}

	var sawOverflowAtComputedLimit bool
	var singleOKCount int
	var sawSingleOverflow bool
	for _, call := range rec.snapshot() {
		switch {
		case call.limit > 1 && call.code == codes.ResourceExhausted:
			sawOverflowAtComputedLimit = true
		case call.limit == 1 && call.code == codes.OK:
			singleOKCount++
		case call.limit == 1 && call.code == codes.ResourceExhausted:
			sawSingleOverflow = true
		}
	}
	if !sawOverflowAtComputedLimit {
		t.Error("recorder never observed a call at the computed Limit ending ResourceExhausted")
	}
	if singleOKCount < 2 {
		t.Errorf("recorder observed %d limit==1 OK call(s), want at least 2", singleOKCount)
	}
	if sawSingleOverflow {
		t.Error("recorder observed a limit==1 call ending ResourceExhausted, want none (the fallback should have covered every legacy record within one record)")
	}
}

// TestScrollAllPointsSingleOversizedRecordFailsNamed proves D-07's failure
// half: one record larger than storetest.RecvLimit, alongside two small
// records, fails the full-view sweep with errors.Is(err,
// store.ErrResponseTooLarge) after the fallback's Limit-1 retry was
// attempted — never silently skipped.
func TestScrollAllPointsSingleOversizedRecordFailsNamed(t *testing.T) {
	rec := &scrollRecorder{}
	c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(rec.intercept))
	name := store.PrefixedTestCollection("oversized_boundedread_singlefail_" + uuid.NewString())
	st := store.NewTestStore(t, c, name)
	ctx := context.Background()
	if err := st.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	t.Cleanup(func() {
		if err := c.DeleteCollection(ctx, name); err != nil {
			t.Errorf("DeleteCollection(%q): %v", name, err)
		}
	})

	scope := "storetest-singlefail:" + uuid.NewString()
	owner := "storetest-owner-" + uuid.NewString()
	base := time.Now().UTC().Truncate(time.Second)
	hugeContentBytes := storetest.RecvLimit + storetest.RecvLimit/4

	contents := []string{strings.Repeat("x", hugeContentBytes), strings.Repeat("s", 64), strings.Repeat("s", 64)}
	for i, content := range contents {
		m := store.Memory{
			ID:        uuid.NewString(),
			Content:   content,
			Scope:     scope,
			Owner:     owner,
			Actor:     owner,
			Category:  "decision",
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		}
		if err := st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert record %d: %v", i, err)
		}
	}

	rec.reset()
	scanErr := st.ScrollAllPoints(ctx,
		&qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("scope", scope)}},
		st.FullView(),
		func(_ *qdrant.RetrievedPoint) error { return nil })
	if scanErr == nil {
		t.Fatal("ScrollAllPoints: got nil error, want a non-nil error wrapping store.ErrResponseTooLarge")
	}
	if !errors.Is(scanErr, store.ErrResponseTooLarge) {
		t.Fatalf("errors.Is(scanErr, store.ErrResponseTooLarge) = false; err = %v", scanErr)
	}

	var sawSingleOverflow bool
	for _, call := range rec.snapshot() {
		if call.limit == 1 && call.code == codes.ResourceExhausted {
			sawSingleOverflow = true
		}
	}
	if !sawSingleOverflow {
		t.Error("recorder never observed a limit==1 call ending ResourceExhausted — the fallback did not run before the sweep failed")
	}
}

// maxCapCitations builds n citations each at DefaultRecordCaps()'s excerpt
// cap, with Ref/Locator/Pin at RESEARCH.md's assumed sizes (256/64/64
// bytes) — Ref/Locator/Pin carry no write cap; these are representative
// values, not an enforced bound.
func maxCapCitations(n, excerptBytes int) []store.Citation {
	out := make([]store.Citation, n)
	for i := range out {
		out[i] = store.Citation{
			Kind:    "file",
			Ref:     strings.Repeat("r", 256),
			Locator: strings.Repeat("l", 64),
			Pin:     strings.Repeat("p", 64),
			Excerpt: strings.Repeat("e", excerptBytes),
		}
	}
	return out
}

// maxCapTags builds n distinct tags, each exactly tagBytes long.
func maxCapTags(n, tagBytes int) []string {
	out := make([]string, n)
	for i := range out {
		prefix := fmt.Sprintf("tag%04d", i)
		out[i] = prefix + strings.Repeat("x", tagBytes-len(prefix))
	}
	return out
}

// TestRecordCeilingHoldsForMaxCapRecord writes one record at every default
// cap (D-02's provability half) and proves, in both views: proto.Size(p)
// does not exceed store.ViewMaxRecordBytes(view), the retrieved point
// carries nil Vectors (RESEARCH.md Assumption A1), and the summary view
// excludes content and citations while carrying every tag and the summary.
func TestRecordCeilingHoldsForMaxCapRecord(t *testing.T) {
	c := storetest.Dial(t, storetest.RecvLimit)
	name := store.PrefixedTestCollection("oversized_boundedread_maxcap_" + uuid.NewString())
	st := store.NewTestStore(t, c, name)
	ctx := context.Background()
	if err := st.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	t.Cleanup(func() {
		if err := c.DeleteCollection(ctx, name); err != nil {
			t.Errorf("DeleteCollection(%q): %v", name, err)
		}
	})

	caps := store.DefaultRecordCaps()
	tags := maxCapTags(caps.Tags, caps.TagBytes)
	citations := maxCapCitations(caps.Citations, caps.CitationExcerptBytes)
	supersedes := make([]string, 16)
	for i := range supersedes {
		supersedes[i] = uuid.NewString()
	}

	scope := "storetest-maxcap:" + uuid.NewString()
	owner := "storetest-owner-" + uuid.NewString()
	m := store.Memory{
		ID:         uuid.NewString(),
		Content:    strings.Repeat("c", caps.ContentBytes),
		Scope:      scope,
		Repo:       strings.Repeat("g", 256),
		Workspace:  strings.Repeat("w", 256),
		Worktree:   strings.Repeat("t", 256),
		BaseDir:    strings.Repeat("b", 256),
		Category:   "decision",
		Tags:       tags,
		Owner:      owner,
		Actor:      owner,
		CreatedAt:  time.Now().UTC(),
		Summary:    strings.Repeat("s", caps.SummaryBytes),
		Citations:  citations,
		Supersedes: supersedes,
	}
	if err := st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("Upsert max-cap record: %v", err)
	}

	filter := &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("scope", scope)}}

	checkView := func(t *testing.T, name string, view store.ReadView, wantContent, wantCitations bool) {
		t.Helper()
		var visits int
		scanErr := st.ScrollAllPoints(ctx, filter, view, func(p *qdrant.RetrievedPoint) error {
			visits++
			if got, ceiling := proto.Size(p), store.ViewMaxRecordBytes(view); got > ceiling {
				t.Errorf("%s view: proto.Size(p) = %d, exceeds ViewMaxRecordBytes %d", name, got, ceiling)
			}
			if v := p.GetVectors(); v != nil {
				t.Errorf("%s view: GetVectors() = %v, want nil", name, v)
			}
			if _, has := p.Payload["content"]; has != wantContent {
				t.Errorf("%s view: content present = %v, want %v", name, has, wantContent)
			}
			if _, has := p.Payload["citations"]; has != wantCitations {
				t.Errorf("%s view: citations present = %v, want %v", name, has, wantCitations)
			}
			if name == "summary" {
				if got := len(p.Payload["tags"].GetListValue().GetValues()); got != len(tags) {
					t.Errorf("summary view: tags count = %d, want %d", got, len(tags))
				}
				if got := p.Payload["summary"].GetStringValue(); got != m.Summary {
					t.Errorf("summary view: summary mismatch (len %d vs %d)", len(got), len(m.Summary))
				}
			}
			return nil
		})
		if scanErr != nil {
			t.Fatalf("%s view ScrollAllPoints: %v", name, scanErr)
		}
		if visits != 1 {
			t.Errorf("%s view: visited %d time(s), want 1", name, visits)
		}
	}

	checkView(t, "full", st.FullView(), true, true)
	checkView(t, "summary", st.SummaryView(), false, false)
}
