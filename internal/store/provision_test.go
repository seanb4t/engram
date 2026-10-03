// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// rpcCounter counts unary RPCs by full method name.
type rpcCounter struct {
	mu sync.Mutex
	n  map[string]int
}

func (r *rpcCounter) intercept(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	r.mu.Lock()
	r.n[method]++
	r.mu.Unlock()
	return invoker(ctx, method, req, reply, cc, opts...)
}

func (r *rpcCounter) count(method string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.n[method]
}

func (r *rpcCounter) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.n = map[string]int{}
}

// provisionFixture dials a counted client and returns a fresh, uniquely named
// collection that is dropped on cleanup.
func provisionFixture(t *testing.T, name string) (*qdrant.Client, *rpcCounter, string) {
	t.Helper()
	rc := &rpcCounter{n: map[string]int{}}
	c := dialTestClient(t, grpc.WithChainUnaryInterceptor(rc.intercept))
	collection := testCollection(name)
	ctx := context.Background()
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })
	return c, rc, collection
}

func quantizationOf(t *testing.T, c *qdrant.Client, collection string) *qdrant.QuantizationConfig {
	t.Helper()
	info, err := c.GetCollectionInfo(context.Background(), collection)
	if err != nil {
		t.Fatalf("GetCollectionInfo: %v", err)
	}
	return info.GetConfig().GetQuantizationConfig()
}

// setScalarQuantile hand-sets an int8 scalar config with a non-target quantile,
// standing in for an operator's manual tuning.
func setScalarQuantile(t *testing.T, c *qdrant.Client, collection string, quantile float32) {
	t.Helper()
	if err := c.UpdateCollection(context.Background(), &qdrant.UpdateCollection{
		CollectionName: collection,
		QuantizationConfig: qdrant.NewQuantizationDiffScalar(&qdrant.ScalarQuantization{
			Type: qdrant.QuantizationType_Int8, Quantile: qdrant.PtrOf(quantile),
		}),
	}); err != nil {
		t.Fatalf("hand-set quantization: %v", err)
	}
}

func TestCreateCollectionIfAbsentQuantization(t *testing.T) {
	for _, tc := range []struct {
		mode     QuantizationMode
		wantInt8 bool
	}{
		{QuantizationInt8, true},
		{QuantizationOff, false},
		{QuantizationUnmanaged, false},
		{"", false},
	} {
		t.Run(string(tc.mode)+"_mode", func(t *testing.T) {
			c, _, collection := provisionFixture(t, "provision_create_"+string(tc.mode))
			s := newTestStore(t, c, collection, WithQuantization(tc.mode))
			if err := s.CreateCollectionIfAbsent(context.Background(), 3); err != nil {
				t.Fatalf("CreateCollectionIfAbsent: %v", err)
			}
			got := quantizationOf(t, c, collection)
			if tc.wantInt8 && !isInt8Target(got) {
				t.Fatalf("quantization = %v, want int8 scalar quantile 0.99 pinned", got)
			}
			if !tc.wantInt8 && got != nil {
				t.Fatalf("quantization = %v, want none", got)
			}
		})
	}
}

func TestCreateCollectionIfAbsentLeavesExistingCollection(t *testing.T) {
	c, _, collection := provisionFixture(t, "provision_create_existing")
	ctx := context.Background()
	if err := newTestStore(t, c, collection).CreateCollectionIfAbsent(ctx, 3); err != nil {
		t.Fatalf("create unmanaged: %v", err)
	}
	// An int8 store must not touch the existing collection at create time;
	// changing it is EnsureSchema's job.
	if err := newTestStore(t, c, collection, WithQuantization(QuantizationInt8)).CreateCollectionIfAbsent(ctx, 3); err != nil {
		t.Fatalf("create int8 over existing: %v", err)
	}
	if got := quantizationOf(t, c, collection); got != nil {
		t.Fatalf("quantization = %v, want none (existing collection unchanged)", got)
	}
}

func TestEnsureSchemaReconcilesQuantization(t *testing.T) {
	const update = "/qdrant.Collections/Update"
	ctx := context.Background()

	t.Run("int8 quantizes an unquantized collection", func(t *testing.T) {
		c, _, collection := provisionFixture(t, "provision_int8_from_none")
		if err := newTestStore(t, c, collection).CreateCollectionIfAbsent(ctx, 3); err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := newTestStore(t, c, collection, WithQuantization(QuantizationInt8)).EnsureSchema(ctx); err != nil {
			t.Fatalf("EnsureSchema: %v", err)
		}
		if got := quantizationOf(t, c, collection); !isInt8Target(got) {
			t.Fatalf("quantization = %v, want int8 target", got)
		}
	})

	t.Run("int8 replaces a different scalar config", func(t *testing.T) {
		c, _, collection := provisionFixture(t, "provision_int8_replace")
		if err := newTestStore(t, c, collection).CreateCollectionIfAbsent(ctx, 3); err != nil {
			t.Fatalf("create: %v", err)
		}
		setScalarQuantile(t, c, collection, 0.95)
		if err := newTestStore(t, c, collection, WithQuantization(QuantizationInt8)).EnsureSchema(ctx); err != nil {
			t.Fatalf("EnsureSchema: %v", err)
		}
		if got := quantizationOf(t, c, collection); !isInt8Target(got) {
			t.Fatalf("quantization = %v, want int8 target", got)
		}
	})

	t.Run("off removes quantization", func(t *testing.T) {
		c, _, collection := provisionFixture(t, "provision_off_removes")
		if err := newTestStore(t, c, collection, WithQuantization(QuantizationInt8)).CreateCollectionIfAbsent(ctx, 3); err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := newTestStore(t, c, collection, WithQuantization(QuantizationOff)).EnsureSchema(ctx); err != nil {
			t.Fatalf("EnsureSchema: %v", err)
		}
		if got := quantizationOf(t, c, collection); got != nil {
			t.Fatalf("quantization = %v, want none", got)
		}
	})

	t.Run("unmanaged leaves a hand-set config unchanged", func(t *testing.T) {
		c, rc, collection := provisionFixture(t, "provision_unmanaged")
		if err := newTestStore(t, c, collection).CreateCollectionIfAbsent(ctx, 3); err != nil {
			t.Fatalf("create: %v", err)
		}
		setScalarQuantile(t, c, collection, 0.95)
		rc.reset()
		if err := newTestStore(t, c, collection, WithQuantization(QuantizationUnmanaged)).EnsureSchema(ctx); err != nil {
			t.Fatalf("EnsureSchema: %v", err)
		}
		if n := rc.count(update); n != 0 {
			t.Fatalf("unmanaged EnsureSchema sent %d UpdateCollection RPCs, want 0", n)
		}
		if got := quantizationOf(t, c, collection).GetScalar().GetQuantile(); got != 0.95 {
			t.Fatalf("quantile = %v, want hand-set 0.95", got)
		}
	})

	for _, mode := range []QuantizationMode{QuantizationInt8, QuantizationOff} {
		t.Run(string(mode)+" second EnsureSchema sends no update", func(t *testing.T) {
			c, rc, collection := provisionFixture(t, "provision_idempotent_"+string(mode))
			s := newTestStore(t, c, collection, WithQuantization(mode))
			if err := s.EnsureCollection(ctx, 3); err != nil {
				t.Fatalf("EnsureCollection: %v", err)
			}
			rc.reset()
			if err := s.EnsureSchema(ctx); err != nil {
				t.Fatalf("second EnsureSchema: %v", err)
			}
			if n := rc.count(update); n != 0 {
				t.Fatalf("second EnsureSchema sent %d UpdateCollection RPCs, want 0", n)
			}
		})
	}
}

// TestQuantizationChangesLeaveRecordsUntouched switches a seeded collection
// off → int8 → off and checks every point's payload and vector read back
// identical: the reconcile is a config update, never a rewrite of the data.
func TestQuantizationChangesLeaveRecordsUntouched(t *testing.T) {
	c, _, collection := provisionFixture(t, "provision_records_untouched")
	ctx := context.Background()
	off := newTestStore(t, c, collection, WithQuantization(QuantizationOff))
	if err := off.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	ids := []string{
		"d0000000-0000-0000-0000-000000000001",
		"d0000000-0000-0000-0000-000000000002",
		"d0000000-0000-0000-0000-000000000003",
	}
	for i, id := range ids {
		m := Memory{ID: id, Content: "record " + id, Scope: "provision:project:untouched", Category: "gotcha", Tags: []string{"t"}}
		if err := off.Upsert(ctx, m, []float32{0.1 * float32(i+1), 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert %s: %v", id, err)
		}
	}
	read := func() []*qdrant.RetrievedPoint {
		qids := make([]*qdrant.PointId, len(ids))
		for i, id := range ids {
			qids[i] = qdrant.NewID(id)
		}
		pts, err := c.Get(ctx, &qdrant.GetPoints{
			CollectionName: collection, Ids: qids,
			WithPayload: qdrant.NewWithPayload(true), WithVectors: qdrant.NewWithVectors(true),
		})
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		return pts
	}
	before := read()

	for _, mode := range []QuantizationMode{QuantizationInt8, QuantizationOff} {
		if err := newTestStore(t, c, collection, WithQuantization(mode)).EnsureSchema(ctx); err != nil {
			t.Fatalf("EnsureSchema(%s): %v", mode, err)
		}
		after := read()
		if len(after) != len(before) {
			t.Fatalf("after %s: %d points, want %d", mode, len(after), len(before))
		}
		for i := range before {
			if !proto.Equal(before[i], after[i]) {
				t.Fatalf("after %s: point %s changed:\nbefore %v\nafter  %v", mode, ids[i], before[i], after[i])
			}
		}
	}
}

func TestEnsureSchemaLogsEachIndex(t *testing.T) {
	c, _, collection := provisionFixture(t, "provision_index_logs")
	ctx := context.Background()
	s := newTestStore(t, c, collection)
	if err := s.CreateCollectionIfAbsent(ctx, 3); err != nil {
		t.Fatalf("create: %v", err)
	}

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	// logged runs EnsureSchema and returns, per field, the level each index
	// entry was logged at.
	logged := func() map[string][]string {
		buf.Reset()
		if err := s.EnsureSchema(ctx); err != nil {
			t.Fatalf("EnsureSchema: %v", err)
		}
		got := map[string][]string{}
		for line := range bytes.Lines(buf.Bytes()) {
			var e struct {
				Level      string `json:"level"`
				Msg        string `json:"msg"`
				Collection string `json:"collection"`
				Field      string `json:"field"`
				Duration   *int64 `json:"duration"`
			}
			if err := json.Unmarshal(line, &e); err != nil || e.Msg != "ensured payload index" || e.Collection != collection {
				continue
			}
			if e.Duration == nil {
				t.Errorf("index %q logged without a duration", e.Field)
			}
			got[e.Field] = append(got[e.Field], e.Level)
		}
		return got
	}
	want := []string{"owner", "scope", "created_at", "short_id", schemaVersionKey, "tags",
		"visibility", "superseded_by", "archived_at", "not_before", "not_after"}
	check := func(phase string, got map[string][]string, level string) {
		t.Helper()
		for _, f := range want {
			if len(got[f]) != 1 || got[f][0] != level {
				t.Errorf("%s: index %q logged at %v, want exactly one %s entry", phase, f, got[f], level)
			}
		}
		if len(got) != len(want) {
			t.Errorf("%s: logged indexes %v, want exactly %v", phase, got, want)
		}
	}
	check("first provisioning (builds)", logged(), "INFO")
	check("second provisioning (already present)", logged(), "DEBUG")
}

func TestParseQuantizationMode(t *testing.T) {
	for _, s := range []string{"int8", "off", "unmanaged"} {
		if m, err := ParseQuantizationMode(s); err != nil || string(m) != s {
			t.Errorf("ParseQuantizationMode(%q) = %q, %v", s, m, err)
		}
	}
	for _, s := range []string{"", "INT8", "binary"} {
		if _, err := ParseQuantizationMode(s); err == nil {
			t.Errorf("ParseQuantizationMode(%q) = nil error, want rejection", s)
		}
	}
}

// TestReindexTargetQuantization covers both halves of the reindex contract: a
// target reindex creates carries the configured mode, and a target that
// already exists keeps its quantization through a resumed reindex.
func TestReindexTargetQuantization(t *testing.T) {
	ctx := context.Background()

	t.Run("fresh target created with int8", func(t *testing.T) {
		c, _, src := provisionFixture(t, "reindex_q_src_fresh")
		tgt := testCollection("reindex_q_tgt_fresh")
		_ = c.DeleteCollection(ctx, tgt)
		t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), tgt) })
		seedSource(t, c, src)

		s := newTestStore(t, c, src)
		if _, err := s.Reindex(ctx, ReindexOptions{Target: tgt, Dim: 4, Quantization: QuantizationInt8}, embed4); err != nil {
			t.Fatalf("Reindex: %v", err)
		}
		if got := quantizationOf(t, c, tgt); !isInt8Target(got) {
			t.Fatalf("target quantization = %v, want int8 target", got)
		}
		if got := quantizationOf(t, c, src); got != nil {
			t.Fatalf("source quantization = %v, want none (reindex must not touch its source)", got)
		}
	})

	t.Run("resumed reindex leaves an existing target alone", func(t *testing.T) {
		c, rc, src := provisionFixture(t, "reindex_q_src_resume")
		tgt := testCollection("reindex_q_tgt_resume")
		_ = c.DeleteCollection(ctx, tgt)
		t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), tgt) })
		seedSource(t, c, src)

		s := newTestStore(t, c, src)
		if _, err := s.Reindex(ctx, ReindexOptions{Target: tgt, Dim: 4}, embed4); err != nil {
			t.Fatalf("first Reindex: %v", err)
		}
		rc.reset()
		if _, err := s.Reindex(ctx, ReindexOptions{Target: tgt, Dim: 4, Resume: true, Quantization: QuantizationInt8}, embed4); err != nil {
			t.Fatalf("resumed Reindex: %v", err)
		}
		if n := rc.count("/qdrant.Collections/Update"); n != 0 {
			t.Fatalf("resumed reindex sent %d UpdateCollection RPCs, want 0", n)
		}
		if got := quantizationOf(t, c, tgt); got != nil {
			t.Fatalf("target quantization = %v, want none (existing target unchanged)", got)
		}
	})
}
