// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// stallFirst delays the first call to method by d (or until ctx ends), then
// lets every call through.
func stallFirst(method string, d time.Duration) grpc.UnaryClientInterceptor {
	var seen atomic.Bool
	return func(ctx context.Context, m string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if m == method && !seen.Swap(true) {
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return invoker(ctx, m, req, reply, cc, opts...)
	}
}

// rejectMethod fails every call to method with InvalidArgument.
func rejectMethod(method string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, m string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if m == method {
			return status.Error(grpccodes.InvalidArgument, "rejected by test")
		}
		return invoker(ctx, m, req, reply, cc, opts...)
	}
}

// provisionCollection returns a fresh, uniquely named collection, dropped on
// cleanup.
func provisionCollection(t *testing.T, name string) string {
	t.Helper()
	if storetest.Addr() == "" {
		storetest.SkipOrFailNoQdrant(t)
	}
	collection := testCollection(name)
	c := storetest.Dial(t, storetest.RecvLimit)
	_ = c.DeleteCollection(context.Background(), collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })
	return collection
}

const createFieldIndex = "/qdrant.Points/CreateFieldIndex"

func TestProvisionStoreBudgets(t *testing.T) {
	t.Run("a slow index build within the schema budget succeeds although it outlasts the create budget", func(t *testing.T) {
		collection := provisionCollection(t, "provision_slow_index_ok")
		c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(stallFirst(createFieldIndex, 600*time.Millisecond)))
		st := newTestStore(t, c, collection)
		if err := provisionStore(st, 3, 300*time.Millisecond, 10*time.Second); err != nil {
			t.Fatalf("provisionStore: %v", err)
		}
	})

	t.Run("exceeding the schema budget names the step and the variable", func(t *testing.T) {
		collection := provisionCollection(t, "provision_schema_timeout")
		c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(stallFirst(createFieldIndex, 10*time.Second)))
		st := newTestStore(t, c, collection)
		err := provisionStore(st, 3, 10*time.Second, 300*time.Millisecond)
		if err == nil {
			t.Fatal("provisionStore = nil, want a schema deadline error")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("error %v does not wrap context.DeadlineExceeded", err)
		}
		for _, want := range []string{"schema provisioning", "ENGRAM_QDRANT_SCHEMA_TIMEOUT=300ms"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not name %q", err, want)
			}
		}
	})

	t.Run("a rejected quantization update fails provisioning naming the step", func(t *testing.T) {
		collection := provisionCollection(t, "provision_quant_rejected")
		plain := newTestStore(t, storetest.Dial(t, storetest.RecvLimit), collection)
		if err := plain.CreateCollectionIfAbsent(context.Background(), 3); err != nil {
			t.Fatalf("create unquantized collection: %v", err)
		}
		c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(rejectMethod("/qdrant.Collections/Update")))
		st := newTestStore(t, c, collection, store.WithQuantization(store.QuantizationInt8))
		err := provisionStore(st, 3, 10*time.Second, 10*time.Second)
		if err == nil {
			t.Fatal("provisionStore = nil, want the rejected quantization update")
		}
		for _, want := range []string{"schema provisioning", "reconcile quantization (int8)"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not name %q", err, want)
			}
		}
	})
}

// TestOnlyServerReconcilesQuantization drives the real construction paths
// from the environment: the server applies its mode, and a CLI verb's store
// (StoreFromEnv) leaves the collection alone whatever its own env says.
func TestOnlyServerReconcilesQuantization(t *testing.T) {
	collection := provisionCollection(t, "provision_server_vs_cli")
	t.Setenv("ENGRAM_QDRANT_ADDR", storetest.Addr())
	t.Setenv("ENGRAM_QDRANT_COLLECTION", collection)
	t.Setenv("ENGRAM_EMBED_DIM", "3")
	t.Setenv("ENGRAM_QDRANT_SCHEMA_TIMEOUT", "")
	// Same ambient-env isolation as TestBuildDepsFromEnvLoadsConfigOnce: no
	// summary queue, decider, or ranker for this build.
	t.Setenv("ENGRAM_SUMMARY_MODEL", "")
	t.Setenv("ENGRAM_SUMMARY_ON_WRITE", "")
	t.Setenv("ENGRAM_DECISIONS_PROVIDER", "")
	t.Setenv("ENGRAM_SEARCH_RANKER", "")

	reader := storetest.Dial(t, storetest.RecvLimit)
	quantization := func() *qdrant.QuantizationConfig {
		t.Helper()
		info, err := reader.GetCollectionInfo(context.Background(), collection)
		if err != nil {
			t.Fatalf("GetCollectionInfo: %v", err)
		}
		return info.GetConfig().GetQuantizationConfig()
	}

	t.Setenv("ENGRAM_QDRANT_QUANTIZATION", "off")
	if _, err := buildDepsFromEnv(nil, nil); err != nil {
		t.Fatalf("server build (off): %v", err)
	}
	if got := quantization(); got != nil {
		t.Fatalf("after server start in off mode: quantization = %v, want none", got)
	}

	// An operator's shell with the variable unset (default int8).
	t.Setenv("ENGRAM_QDRANT_QUANTIZATION", "")
	if _, err := StoreFromEnv(); err != nil {
		t.Fatalf("CLI StoreFromEnv: %v", err)
	}
	if got := quantization(); got != nil {
		t.Fatalf("after a CLI verb with the default env: quantization = %v, want none (CLI must not reconcile)", got)
	}

	if _, err := buildDepsFromEnv(nil, nil); err != nil {
		t.Fatalf("server build (default int8): %v", err)
	}
	sc := quantization().GetScalar()
	if sc.GetType() != qdrant.QuantizationType_Int8 || sc.GetQuantile() != 0.99 || sc.GetMemory() != qdrant.Memory_Pinned {
		t.Fatalf("after server start with the default mode: quantization = %v, want int8 scalar quantile 0.99 pinned", sc)
	}
}
