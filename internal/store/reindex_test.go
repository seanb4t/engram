// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

// l2normalize mirrors the normalization Qdrant applies when storing vectors in a
// Cosine-distance collection, so the test can compare a re-embedded vector
// against the embedder's raw output.
func l2normalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	n := float32(math.Sqrt(sum))
	if n == 0 {
		return v
	}
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = x / n
	}
	return out
}

// scrolledPoint is the verbatim shape a reindex test asserts against: the raw
// payload map and the stored vector. Payloads are compared by proto-text per
// key (reflect.DeepEqual on *qdrant.Value is unreliable across its internal
// state fields).
type scrolledPoint struct {
	payload map[string]*qdrant.Value
	vec     []float32
}

func scrollPoints(t *testing.T, c *qdrant.Client, collection string) map[string]scrolledPoint {
	t.Helper()
	ctx := context.Background()
	pts, err := c.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: collection,
		Limit:          qdrant.PtrOf(uint32(1000)),
		WithPayload:    qdrant.NewWithPayload(true),
		WithVectors:    qdrant.NewWithVectors(true),
	})
	if err != nil {
		t.Fatalf("scroll %s: %v", collection, err)
	}
	out := make(map[string]scrolledPoint, len(pts))
	for _, p := range pts {
		// Dense vectors arrive via GetDense().GetData(); GetData() on VectorOutput
		// is deprecated and empty for this server version.
		out[p.Id.GetUuid()] = scrolledPoint{
			payload: p.Payload,
			vec:     p.GetVectors().GetVector().GetDense().GetData(),
		}
	}
	return out
}

func payloadKeysEqual(t *testing.T, label string, want, got map[string]*qdrant.Value) {
	t.Helper()
	if len(want) != len(got) {
		t.Errorf("%s: payload key count: want %d, got %d (want=%v got=%v)", label, len(want), len(got), keys(want), keys(got))
		return
	}
	for k, wv := range want {
		gv, ok := got[k]
		if !ok {
			t.Errorf("%s: missing payload key %q", label, k)
			continue
		}
		if wv.String() != gv.String() {
			t.Errorf("%s: payload key %q: want %q, got %q", label, k, wv.String(), gv.String())
		}
	}
}

func keys(m map[string]*qdrant.Value) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// seedSource writes two points into the source collection at dim 3:
//   - a full Memory via the normal Upsert path (owner + shared + tags), and
//   - a raw point carrying an UNKNOWN payload key and NO owner key, written
//     straight through the client. The raw point is the crux: reindex must
//     preserve unknown keys and must NOT synthesize an owner key (which a
//     Memory round-trip would, dropping the record into the anonymous bucket).
func seedSource(t *testing.T, c *qdrant.Client, src string) (fullID, rawID string) {
	t.Helper()
	ctx := context.Background()
	s := New(c, src)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("ensure source: %v", err)
	}
	fullID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	full := Memory{
		ID:         fullID,
		Content:    "alpha content",
		Scope:      "eval-test:project:demo",
		Repo:       "demo",
		Source:     "user-said",
		Category:   "preference",
		Tags:       []string{"x", "y"},
		Owner:      "sub-123",
		Visibility: "shared",
		CreatedAt:  time.Now().UTC().Truncate(time.Second),
	}
	if err := s.Upsert(ctx, full, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed full: %v", err)
	}

	rawID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	if _, err := c.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: src,
		Wait:           qdrant.PtrOf(true),
		Points: []*qdrant.PointStruct{{
			Id:      qdrant.NewID(rawID),
			Vectors: qdrant.NewVectors(0.4, 0.5, 0.6),
			Payload: qdrant.NewValueMap(map[string]any{
				// Deliberately a different length than the full point's content so
				// the re-embedded vectors differ per point (embed4 keys off length),
				// making the per-point vector assertion discriminating.
				"content":      "bravo",
				"scope":        "eval-test:project:demo",
				"legacy_field": "keep-me-verbatim",
			}),
		}},
	}); err != nil {
		t.Fatalf("seed raw: %v", err)
	}
	return fullID, rawID
}

// embed4Vec is the deterministic dim-4 vector for a content string, keyed off
// its length so distinct contents yield distinct vectors.
func embed4Vec(content string) []float32 {
	return []float32{1, 0, 0, float32(len(content))}
}

// embed4 is a deterministic dim-4 embedder: the target dimension (4) differs
// from the source (3), so a passing test proves a dimension-changing reindex.
func embed4(_ context.Context, content string) ([]float32, error) {
	return embed4Vec(content), nil
}

func TestReindexOptionsValidate(t *testing.T) {
	cases := []struct {
		name    string
		opts    ReindexOptions
		wantErr bool
	}{
		{"valid", ReindexOptions{Target: "tgt", Dim: 4}, false},
		{"valid with source override", ReindexOptions{Target: "tgt", Source: "src", Dim: 4}, false},
		{"empty target", ReindexOptions{Target: "", Dim: 4}, true},
		{"zero dim", ReindexOptions{Target: "tgt", Dim: 0}, true},
		{"explicit source equals target", ReindexOptions{Target: "same", Source: "same", Dim: 4}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.opts.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("Validate(%+v) = nil, want error", tc.opts)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate(%+v) = %v, want nil", tc.opts, err)
			}
		})
	}
}

// TestReindexRejectsInvalidArgs exercises the validation guards that fire before
// any Qdrant call, so it runs even where no Qdrant container is available (a nil
// client is safe — the round-trip tests Skip in that case, leaving these guards
// otherwise uncovered).
func TestReindexRejectsInvalidArgs(t *testing.T) {
	ctx := context.Background()
	s := New(nil, "src")
	cases := []struct {
		name  string
		opts  ReindexOptions
		embed EmbedFunc
	}{
		{"empty target", ReindexOptions{Target: "", Dim: 4}, embed4},
		{"zero dim", ReindexOptions{Target: "tgt", Dim: 0}, embed4},
		{"target equals source", ReindexOptions{Target: "src", Dim: 4}, embed4},
		{"target equals source override", ReindexOptions{Target: "ov", Source: "ov", Dim: 4}, embed4},
		{"nil embed", ReindexOptions{Target: "tgt", Dim: 4}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.Reindex(ctx, tc.opts, tc.embed); err == nil {
				t.Fatalf("Reindex(%+v) = nil error, want a validation error", tc.opts)
			}
		})
	}
}

func TestReindexRoundtrip(t *testing.T) {
	c := dialTestClient(t)
	ctx := context.Background()
	const src, tgt = "reindex_src", "reindex_tgt"
	_ = c.DeleteCollection(ctx, src)
	_ = c.DeleteCollection(ctx, tgt)
	t.Cleanup(func() {
		_ = c.DeleteCollection(context.Background(), src)
		_ = c.DeleteCollection(context.Background(), tgt)
	})

	fullID, rawID := seedSource(t, c, src)
	srcBefore := scrollPoints(t, c, src)

	s := New(c, src)
	res, err := s.Reindex(ctx, ReindexOptions{Target: tgt, Dim: 4}, embed4)
	if err != nil {
		t.Fatalf("reindex: %v", err)
	}
	if res.Scanned != 2 || res.Upserted != 2 {
		t.Errorf("counts: want scanned=2 upserted=2, got %+v", res)
	}

	got := scrollPoints(t, c, tgt)
	if len(got) != 2 {
		t.Fatalf("target point count: want 2, got %d", len(got))
	}

	for id, want := range srcBefore {
		tp, ok := got[id]
		if !ok {
			t.Errorf("target missing id %s", id)
			continue
		}
		// Payload preserved verbatim.
		payloadKeysEqual(t, "id="+id, want.payload, tp.payload)
		// Vector re-embedded at the new dimension from content+tags (EmbedText).
		// Qdrant stores Cosine-distance vectors L2-normalized, so compare against
		// the normalized embedder output for the composed document text.
		m := fromPayload(id, tp.payload)
		wantVec := l2normalize(embed4Vec(EmbedText(m.Content, m.Tags)))
		if len(tp.vec) != 4 {
			t.Errorf("id=%s: target vector dim: want 4, got %d", id, len(tp.vec))
		}
		for i := range wantVec {
			if i >= len(tp.vec) || math.Abs(float64(tp.vec[i]-wantVec[i])) > 1e-4 {
				t.Errorf("id=%s: vector[%d]: want ~%v, got %v", id, i, wantVec[i], tp.vec[i])
			}
		}
	}

	// The raw point's unknown key survives, and no owner key is synthesized.
	if _, ok := got[rawID].payload["legacy_field"]; !ok {
		t.Errorf("raw point lost unknown payload key legacy_field")
	}
	if _, ok := got[rawID].payload["owner"]; ok {
		t.Errorf("raw point gained a synthesized owner key (would land it in the anonymous bucket)")
	}
	// The full point keeps its owner verbatim.
	if got[fullID].payload["owner"].GetStringValue() != "sub-123" {
		t.Errorf("full point owner not preserved: %q", got[fullID].payload["owner"].GetStringValue())
	}

	// Source is left untouched: original 3-dim vectors intact.
	srcAfter := scrollPoints(t, c, src)
	if len(srcAfter) != 2 {
		t.Errorf("source point count changed: want 2, got %d", len(srcAfter))
	}
	if got := srcAfter[rawID].vec; len(got) != 3 {
		t.Errorf("source vector dim changed: want 3, got %d", len(got))
	}
}

// TestReindexSourceOverride pins engram-orve: ReindexOptions.Source overrides the
// store's configured collection as the read source. The store is built pointing at
// a NON-EXISTENT env collection; with Source set to the real (seeded) collection,
// Reindex must read the override and succeed — proving it does not fall back to
// s.collection.
func TestReindexSourceOverride(t *testing.T) {
	c := dialTestClient(t)
	ctx := context.Background()
	const realSrc, envSrc, tgt = "reindex_srcov_real", "reindex_srcov_env", "reindex_srcov_tgt"
	cols := []string{realSrc, envSrc, tgt}
	for _, col := range cols {
		_ = c.DeleteCollection(ctx, col)
	}
	t.Cleanup(func() {
		for _, col := range cols {
			_ = c.DeleteCollection(context.Background(), col)
		}
	})

	seedSource(t, c, realSrc) // create + seed the override source only

	// Store points at envSrc, which is never created; Source=realSrc must win.
	s := New(c, envSrc)
	res, err := s.Reindex(ctx, ReindexOptions{Target: tgt, Source: realSrc, Dim: 4}, embed4)
	if err != nil {
		t.Fatalf("reindex with source override: %v", err)
	}
	if res.Scanned != 2 || res.Upserted != 2 {
		t.Errorf("counts: want scanned=2 upserted=2, got %+v", res)
	}
	if got := scrollPoints(t, c, tgt); len(got) != 2 {
		t.Fatalf("target point count: want 2, got %d", len(got))
	}
}

// TestReindexProgressCallback pins engram-xddn: ReindexOptions.Progress is
// invoked once per scanned batch with the running totals, so a long reindex can
// surface incremental feedback. Batch:1 over two points forces multiple pages.
func TestReindexProgressCallback(t *testing.T) {
	c := dialTestClient(t)
	ctx := context.Background()
	const src, tgt = "reindex_prog_src", "reindex_prog_tgt"
	_ = c.DeleteCollection(ctx, src)
	_ = c.DeleteCollection(ctx, tgt)
	t.Cleanup(func() {
		_ = c.DeleteCollection(context.Background(), src)
		_ = c.DeleteCollection(context.Background(), tgt)
	})
	seedSource(t, c, src) // two points

	s := New(c, src)
	var snaps []ReindexResult
	res, err := s.Reindex(ctx, ReindexOptions{
		Target: tgt, Dim: 4, Batch: 1,
		Progress: func(r ReindexResult) { snaps = append(snaps, r) },
	}, embed4)
	if err != nil {
		t.Fatalf("reindex: %v", err)
	}
	if len(snaps) < 2 {
		t.Fatalf("want >=2 progress callbacks (Batch:1 over 2 points), got %d", len(snaps))
	}
	for i := 1; i < len(snaps); i++ {
		if snaps[i].Scanned < snaps[i-1].Scanned {
			t.Errorf("progress Scanned went backwards: %+v", snaps)
		}
	}
	last := snaps[len(snaps)-1]
	if last.Scanned != res.Scanned || last.Scanned != 2 {
		t.Errorf("final progress %+v must match result %+v (scanned 2)", last, res)
	}
}

// TestReindexResumeSkipsUnchanged pins engram-irhg: with Resume, a point already
// present in the target with identical content is skipped (counted Unchanged)
// instead of re-embedded, so an interrupted reindex resumes cheaply — while a
// point whose source content changed is re-embedded. Batch:1 forces the resume
// target lookup to run per page (one Get per point here), exercising the
// per-page scoping across a page boundary rather than in a single batch.
func TestReindexResumeSkipsUnchanged(t *testing.T) {
	c := dialTestClient(t)
	ctx := context.Background()
	const src, tgt = "reindex_resume_src", "reindex_resume_tgt"
	_ = c.DeleteCollection(ctx, src)
	_ = c.DeleteCollection(ctx, tgt)
	t.Cleanup(func() {
		_ = c.DeleteCollection(context.Background(), src)
		_ = c.DeleteCollection(context.Background(), tgt)
	})
	_, rawID := seedSource(t, c, src) // two points

	s := New(c, src)
	// First run: fresh target, nothing to skip — both re-embedded.
	res1, err := s.Reindex(ctx, ReindexOptions{Target: tgt, Dim: 4, Batch: 1, Resume: true}, embed4)
	if err != nil {
		t.Fatalf("run1: %v", err)
	}
	if res1.Upserted != 2 || res1.Unchanged != 0 {
		t.Errorf("run1: want upserted=2 unchanged=0, got %+v", res1)
	}

	// Second run: every point present with identical content — all skipped.
	res2, err := s.Reindex(ctx, ReindexOptions{Target: tgt, Dim: 4, Batch: 1, Resume: true}, embed4)
	if err != nil {
		t.Fatalf("run2: %v", err)
	}
	if res2.Scanned != 2 || res2.Upserted != 0 || res2.Unchanged != 2 {
		t.Errorf("run2: want scanned=2 upserted=0 unchanged=2, got %+v", res2)
	}

	// Mutate one source point's content; resume must re-embed only that one.
	if _, err := c.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: src, Wait: qdrant.PtrOf(true),
		Points: []*qdrant.PointStruct{{
			Id:      qdrant.NewID(rawID),
			Vectors: qdrant.NewVectors(0.4, 0.5, 0.6),
			Payload: qdrant.NewValueMap(map[string]any{
				"content": "bravo-changed", "scope": "eval-test:project:demo", "legacy_field": "keep-me-verbatim",
			}),
		}},
	}); err != nil {
		t.Fatalf("mutate source: %v", err)
	}

	res3, err := s.Reindex(ctx, ReindexOptions{Target: tgt, Dim: 4, Batch: 1, Resume: true}, embed4)
	if err != nil {
		t.Fatalf("run3: %v", err)
	}
	if res3.Upserted != 1 || res3.Unchanged != 1 {
		t.Errorf("run3: want upserted=1 unchanged=1, got %+v", res3)
	}
	if got := scrollPoints(t, c, tgt); got[rawID].payload["content"].GetStringValue() != "bravo-changed" {
		t.Errorf("changed point's target content not updated: %q", got[rawID].payload["content"].GetStringValue())
	}
}

func TestReindexDryRunWritesNothing(t *testing.T) {
	c := dialTestClient(t)
	ctx := context.Background()
	const src, tgt = "reindex_dry_src", "reindex_dry_tgt"
	_ = c.DeleteCollection(ctx, src)
	_ = c.DeleteCollection(ctx, tgt)
	t.Cleanup(func() {
		_ = c.DeleteCollection(context.Background(), src)
		_ = c.DeleteCollection(context.Background(), tgt)
	})
	seedSource(t, c, src)

	s := New(c, src)
	res, err := s.Reindex(ctx, ReindexOptions{Target: tgt, Dim: 4, DryRun: true}, embed4)
	if err != nil {
		t.Fatalf("reindex dry-run: %v", err)
	}
	if res.Scanned != 2 || res.Upserted != 0 {
		t.Errorf("dry-run counts: want scanned=2 upserted=0, got %+v", res)
	}
	exists, err := c.CollectionExists(ctx, tgt)
	if err != nil {
		t.Fatalf("collection exists: %v", err)
	}
	if exists {
		t.Errorf("dry-run created the target collection")
	}
}

func TestReindexErrorsOnMissingSource(t *testing.T) {
	c := dialTestClient(t)
	ctx := context.Background()
	const src, tgt = "reindex_missing_src", "reindex_missing_tgt"
	_ = c.DeleteCollection(ctx, src)
	_ = c.DeleteCollection(ctx, tgt)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), tgt) })

	// Source never created — reindexing it must fail loudly, not silently scan
	// zero and report success (which would mask a typo'd ENGRAM_QDRANT_COLLECTION).
	s := New(c, src)
	_, err := s.Reindex(ctx, ReindexOptions{Target: tgt, Dim: 4}, embed4)
	if err == nil {
		t.Fatalf("want error for missing source collection, got nil")
	}
	if exists, _ := c.CollectionExists(ctx, tgt); exists {
		t.Errorf("target created despite missing source")
	}
}

func TestReindexSkipsEmptyContent(t *testing.T) {
	c := dialTestClient(t)
	ctx := context.Background()
	const src, tgt = "reindex_empty_src", "reindex_empty_tgt"
	_ = c.DeleteCollection(ctx, src)
	_ = c.DeleteCollection(ctx, tgt)
	t.Cleanup(func() {
		_ = c.DeleteCollection(context.Background(), src)
		_ = c.DeleteCollection(context.Background(), tgt)
	})

	s := New(c, src)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("ensure source: %v", err)
	}
	withContent := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	noContent := "dddddddd-dddd-dddd-dddd-dddddddddddd"
	if err := s.Upsert(ctx, Memory{ID: withContent, Content: "has content", Scope: "s"}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed with-content: %v", err)
	}
	// Raw point with NO content key at all.
	if _, err := c.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: src, Wait: qdrant.PtrOf(true),
		Points: []*qdrant.PointStruct{{
			Id:      qdrant.NewID(noContent),
			Vectors: qdrant.NewVectors(0.4, 0.5, 0.6),
			Payload: qdrant.NewValueMap(map[string]any{"scope": "s"}),
		}},
	}); err != nil {
		t.Fatalf("seed no-content: %v", err)
	}

	res, err := s.Reindex(ctx, ReindexOptions{Target: tgt, Dim: 4}, embed4)
	if err != nil {
		t.Fatalf("reindex: %v", err)
	}
	if res.Scanned != 2 || res.Upserted != 1 || res.Skipped != 1 {
		t.Errorf("counts: want scanned=2 upserted=1 skipped=1, got %+v", res)
	}
	got := scrollPoints(t, c, tgt)
	if _, ok := got[noContent]; ok {
		t.Errorf("empty-content point was upserted (garbage vector) instead of skipped")
	}
	if _, ok := got[withContent]; !ok {
		t.Errorf("with-content point missing from target")
	}
}

func TestReindexMultiPageCursor(t *testing.T) {
	c := dialTestClient(t)
	ctx := context.Background()
	const src, tgt = "reindex_page_src", "reindex_page_tgt"
	_ = c.DeleteCollection(ctx, src)
	_ = c.DeleteCollection(ctx, tgt)
	t.Cleanup(func() {
		_ = c.DeleteCollection(context.Background(), src)
		_ = c.DeleteCollection(context.Background(), tgt)
	})

	s := New(c, src)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("ensure source: %v", err)
	}
	ids := []string{
		"e0000000-0000-0000-0000-000000000001",
		"e0000000-0000-0000-0000-000000000002",
		"e0000000-0000-0000-0000-000000000003",
	}
	for i, id := range ids {
		if err := s.Upsert(ctx, Memory{ID: id, Content: id, Scope: "s"}, []float32{float32(i), 0.1, 0.2}); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	// Batch:1 forces the ScrollAndOffset cursor across multiple pages — the
	// next-page path that a single-page scroll never exercises.
	res, err := s.Reindex(ctx, ReindexOptions{Target: tgt, Dim: 4, Batch: 1}, embed4)
	if err != nil {
		t.Fatalf("reindex: %v", err)
	}
	if res.Scanned != 3 || res.Upserted != 3 {
		t.Errorf("counts: want scanned=3 upserted=3, got %+v", res)
	}
	got := scrollPoints(t, c, tgt)
	if len(got) != 3 {
		t.Errorf("target point count: want 3 (all pages migrated), got %d", len(got))
	}
}

func TestReindexPartialWriteOnLateEmbedError(t *testing.T) {
	c := dialTestClient(t)
	ctx := context.Background()
	const src, tgt = "reindex_partial_src", "reindex_partial_tgt"
	_ = c.DeleteCollection(ctx, src)
	_ = c.DeleteCollection(ctx, tgt)
	t.Cleanup(func() {
		_ = c.DeleteCollection(context.Background(), src)
		_ = c.DeleteCollection(context.Background(), tgt)
	})
	seedSource(t, c, src) // 2 points

	// Fail on the 2nd embed call regardless of scroll order: the first point is
	// upserted, the second aborts. Documents the no-rollback contract — the
	// target is left partially populated and the reported count matches.
	calls := 0
	boom := errors.New("embedder down on call 2")
	flaky := func(_ context.Context, content string) ([]float32, error) {
		calls++
		if calls == 2 {
			return nil, boom
		}
		return embed4Vec(content), nil
	}

	s := New(c, src)
	res, err := s.Reindex(ctx, ReindexOptions{Target: tgt, Dim: 4, Batch: 1}, flaky)
	if !errors.Is(err, boom) {
		t.Fatalf("want late embed error surfaced, got %v", err)
	}
	if res.Upserted != 1 {
		t.Errorf("want exactly 1 point upserted before the failure, got %d", res.Upserted)
	}
	got := scrollPoints(t, c, tgt)
	if len(got) != int(res.Upserted) {
		t.Errorf("target count %d disagrees with reported Upserted %d", len(got), res.Upserted)
	}
}

// sentinelIdentity is a fixed embedder-config-identity used by the
// reindex-stamps-identity tests below; it never needs to be a real
// config.EmbedderIdentity() output since Reindex just writes whatever
// opts.Identity carries.
const sentinelIdentity = "v1:deadbeefdeadbeef"

// staleIdentity is a different sentinel used to prove the resume path
// restamps a mismatched (not just an absent) identity.
const staleIdentity = "v1:oldoldoldoldold0"

// TestReindexStampsEmbedderIdentity pins Phase 13 SC3: Reindex writes
// opts.Identity onto every reindexed record's payload under
// embedderIdentityKey via the guarded additive raw-map write, without
// disturbing the verbatim-payload owner-key invariant, and leaves the target
// free of the key when Identity is empty.
func TestReindexStampsEmbedderIdentity(t *testing.T) {
	c := dialTestClient(t)
	ctx := context.Background()
	const src, tgtStamped, tgtUnstamped = "reindex_identity_src", "reindex_identity_tgt", "reindex_identity_tgt_none"
	cols := []string{src, tgtStamped, tgtUnstamped}
	for _, col := range cols {
		_ = c.DeleteCollection(ctx, col)
	}
	t.Cleanup(func() {
		for _, col := range cols {
			_ = c.DeleteCollection(context.Background(), col)
		}
	})

	fullID, rawID := seedSource(t, c, src)

	s := New(c, src)
	res, err := s.Reindex(ctx, ReindexOptions{Target: tgtStamped, Dim: 4, Identity: sentinelIdentity}, embed4)
	if err != nil {
		t.Fatalf("reindex with identity: %v", err)
	}
	if res.Scanned != 2 || res.Upserted != 2 {
		t.Errorf("counts: want scanned=2 upserted=2, got %+v", res)
	}

	got := scrollPoints(t, c, tgtStamped)
	for _, id := range []string{fullID, rawID} {
		gotIdentity := got[id].payload[embedderIdentityKey].GetStringValue()
		if gotIdentity != sentinelIdentity {
			t.Errorf("id=%s: embedder_identity = %q, want %q", id, gotIdentity, sentinelIdentity)
		}
	}
	// The verbatim owner-key invariant holds alongside the additive stamp: the
	// raw point (no owner in the source) still has no owner in the target, and
	// the full point's owner survives untouched.
	if _, ok := got[rawID].payload["owner"]; ok {
		t.Errorf("raw point gained a synthesized owner key from the identity stamp")
	}
	if got[fullID].payload["owner"].GetStringValue() != "sub-123" {
		t.Errorf("full point owner not preserved: %q", got[fullID].payload["owner"].GetStringValue())
	}

	// Identity: "" (the default) never ADDS the key. rawID is a raw source
	// point with no embedder_identity key at all (seedSource writes it
	// straight through the client, bypassing payload()), so it proves the
	// no-op case cleanly. fullID went through the normal Upsert/payload()
	// path when seeded, which writes embedder_identity UNCONDITIONALLY
	// (D-05's "unconditional write, conditional read" precedent) — so its
	// source payload already carries the key as "" and the verbatim copy
	// preserves that as-is; it is not a case where Identity:"" ADDS the key.
	res2, err := s.Reindex(ctx, ReindexOptions{Target: tgtUnstamped, Dim: 4}, embed4)
	if err != nil {
		t.Fatalf("reindex without identity: %v", err)
	}
	if res2.Scanned != 2 || res2.Upserted != 2 {
		t.Errorf("counts: want scanned=2 upserted=2, got %+v", res2)
	}
	gotNone := scrollPoints(t, c, tgtUnstamped)
	if _, ok := gotNone[rawID].payload[embedderIdentityKey]; ok {
		t.Errorf("id=%s: embedder_identity key present with empty opts.Identity (Reindex added it)", rawID)
	}
	if v := gotNone[fullID].payload[embedderIdentityKey].GetStringValue(); v != "" {
		t.Errorf("id=%s: embedder_identity = %q, want unchanged empty value from source", fullID, v)
	}
}

// TestReindexResumeRestampsStaleIdentity pins the review MEDIUM fix: under
// Resume:true, a target point whose content matches the source but whose
// embedder_identity is absent or mismatched must be re-embedded and
// restamped, not skipped as Unchanged — otherwise resume silently leaves a
// record untraceable to its embedder config (Phase 13 SC3).
func TestReindexResumeRestampsStaleIdentity(t *testing.T) {
	c := dialTestClient(t)
	ctx := context.Background()
	const src, tgt = "reindex_restamp_src", "reindex_restamp_tgt"
	_ = c.DeleteCollection(ctx, src)
	_ = c.DeleteCollection(ctx, tgt)
	t.Cleanup(func() {
		_ = c.DeleteCollection(context.Background(), src)
		_ = c.DeleteCollection(context.Background(), tgt)
	})

	fullID, rawID := seedSource(t, c, src)

	// Pre-create the target collection at the reindex dimension so the raw
	// pre-population upserts below have somewhere to land (normally Reindex
	// itself ensures the target; here we're seeding state that predates it).
	if err := New(c, tgt).EnsureCollection(ctx, 4); err != nil {
		t.Fatalf("pre-create target collection: %v", err)
	}

	// Pre-populate the target directly (bypassing Reindex) with points whose
	// CONTENT matches the source but whose embedder_identity is ABSENT
	// (fullID) or MISMATCHED (rawID) — the two ways a stamp can be stale.
	if err := s2Upsert(ctx, c, tgt, fullID, "alpha content", map[string]any{
		"content": "alpha content", "scope": "eval-test:project:demo",
	}); err != nil {
		t.Fatalf("pre-populate target (absent identity): %v", err)
	}
	if err := s2Upsert(ctx, c, tgt, rawID, "bravo", map[string]any{
		"content": "bravo", "legacy_field": "keep-me-verbatim", embedderIdentityKey: staleIdentity,
	}); err != nil {
		t.Fatalf("pre-populate target (mismatched identity): %v", err)
	}

	s := New(c, src)
	res, err := s.Reindex(ctx, ReindexOptions{Target: tgt, Dim: 4, Resume: true, Identity: sentinelIdentity}, embed4)
	if err != nil {
		t.Fatalf("reindex resume-restamp: %v", err)
	}
	// Both points have matching content but a stale/absent stamp, so BOTH must
	// be re-embedded+restamped (Upserted), not skipped (Unchanged).
	if res.Upserted != 2 || res.Unchanged != 0 {
		t.Errorf("resume-restamp counts: want upserted=2 unchanged=0, got %+v", res)
	}

	got := scrollPoints(t, c, tgt)
	for _, id := range []string{fullID, rawID} {
		gotIdentity := got[id].payload[embedderIdentityKey].GetStringValue()
		if gotIdentity != sentinelIdentity {
			t.Errorf("id=%s: embedder_identity = %q after resume-restamp, want %q", id, gotIdentity, sentinelIdentity)
		}
	}

	// Complementary case: run again with the target now carrying the matching
	// identity — this time both points must be skipped as Unchanged.
	res2, err := s.Reindex(ctx, ReindexOptions{Target: tgt, Dim: 4, Resume: true, Identity: sentinelIdentity}, embed4)
	if err != nil {
		t.Fatalf("reindex resume (already stamped): %v", err)
	}
	if res2.Upserted != 0 || res2.Unchanged != 2 {
		t.Errorf("resume (already stamped) counts: want upserted=0 unchanged=2, got %+v", res2)
	}
}

// s2Upsert writes a single raw point directly into collection, merging an
// explicit content field with extra payload keys (which may include a
// pre-set embedder_identity) — used to pre-populate a reindex target outside
// the Reindex path itself, simulating a prior run's leftover state.
func s2Upsert(ctx context.Context, c *qdrant.Client, collection, id, content string, extra map[string]any) error {
	payload := map[string]any{"content": content}
	for k, v := range extra {
		payload[k] = v
	}
	_, err := c.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collection,
		Wait:           qdrant.PtrOf(true),
		Points: []*qdrant.PointStruct{{
			Id:      qdrant.NewID(id),
			Vectors: qdrant.NewVectors(0.1, 0.1, 0.1, 0.1),
			Payload: qdrant.NewValueMap(payload),
		}},
	})
	return err
}

func TestReindexFailsClosedOnEmbedError(t *testing.T) {
	c := dialTestClient(t)
	ctx := context.Background()
	const src, tgt = "reindex_fail_src", "reindex_fail_tgt"
	_ = c.DeleteCollection(ctx, src)
	_ = c.DeleteCollection(ctx, tgt)
	t.Cleanup(func() {
		_ = c.DeleteCollection(context.Background(), src)
		_ = c.DeleteCollection(context.Background(), tgt)
	})
	seedSource(t, c, src)

	boom := errors.New("embedder down")
	failing := func(_ context.Context, _ string) ([]float32, error) { return nil, boom }

	s := New(c, src)
	_, err := s.Reindex(ctx, ReindexOptions{Target: tgt, Dim: 4}, failing)
	if !errors.Is(err, boom) {
		t.Fatalf("want embedder error surfaced, got %v", err)
	}
	// Fail-closed: no garbage/zero vector written to the target.
	if exists, _ := c.CollectionExists(ctx, tgt); exists {
		got := scrollPoints(t, c, tgt)
		if len(got) != 0 {
			t.Errorf("fail-closed violated: %d point(s) written to target", len(got))
		}
	}
}
