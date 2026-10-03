// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/telemetry"
	"go.opentelemetry.io/otel/codes"
)

// QuantizationMode selects how a Store manages its collection's vector
// quantization (#698).
type QuantizationMode string

const (
	// QuantizationUnmanaged never reads or changes the collection's
	// quantization config. It is the Store's zero-value behavior, so every
	// construction path except `engram serve` leaves quantization alone.
	QuantizationUnmanaged QuantizationMode = "unmanaged"
	// QuantizationInt8 keeps an int8 scalar copy of every vector pinned in
	// Qdrant's RAM, so a cold search does not page evicted mmap'd vectors
	// back in from disk. Rescoring stays at Qdrant's default (on), so result
	// scores are still computed from the original vectors.
	QuantizationInt8 QuantizationMode = "int8"
	// QuantizationOff actively removes any quantization from the collection.
	QuantizationOff QuantizationMode = "off"
)

// ParseQuantizationMode maps a config value onto a QuantizationMode.
func ParseQuantizationMode(s string) (QuantizationMode, error) {
	switch m := QuantizationMode(s); m {
	case QuantizationUnmanaged, QuantizationInt8, QuantizationOff:
		return m, nil
	}
	return "", fmt.Errorf("unknown quantization mode %q: want int8, off, or unmanaged", s)
}

// WithQuantization sets the mode CreateCollectionIfAbsent creates with and
// EnsureSchema reconciles to. Only the server's construction path sets it.
func WithQuantization(m QuantizationMode) Option {
	return func(s *Store) { s.quantization = m }
}

// int8Quantile is the quantile int8 scalar quantization clips outliers at.
const int8Quantile float32 = 0.99

func int8Scalar() *qdrant.ScalarQuantization {
	return &qdrant.ScalarQuantization{
		Type:     qdrant.QuantizationType_Int8,
		Quantile: qdrant.PtrOf(int8Quantile),
		Memory:   qdrant.PtrOf(qdrant.Memory_Pinned),
	}
}

// CreateCollectionIfAbsent creates the store's collection at the given vector
// size (distance Cosine) when it does not exist, carrying the int8
// quantization config at creation when the mode is QuantizationInt8. It never
// changes an existing collection.
func (s *Store) CreateCollectionIfAbsent(ctx context.Context, dim uint64) (err error) {
	ctx, span := tracer.Start(ctx, "store.CreateCollectionIfAbsent")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "CreateCollectionIfAbsent", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	return s.createCollectionIfAbsent(ctx, s.collection, dim, s.quantization)
}

// createCollectionIfAbsent is CreateCollectionIfAbsent for a named collection
// and an explicit mode, so reindex can create its target with the configured
// mode while its own Store stays unmanaged.
func (s *Store) createCollectionIfAbsent(ctx context.Context, name string, dim uint64, mode QuantizationMode) error {
	exists, err := s.client.CollectionExists(ctx, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	req := &qdrant.CreateCollection{
		CollectionName: name,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size: dim, Distance: qdrant.Distance_Cosine,
		}),
	}
	if mode == QuantizationInt8 {
		req.QuantizationConfig = qdrant.NewQuantizationScalar(int8Scalar())
	}
	return s.client.CreateCollection(ctx, req)
}

// EnsureSchema reconciles the collection's quantization to the store's mode,
// then ensures the recall payload indexes. The reconcile runs first: it is
// cheap (Qdrant applies it in milliseconds and re-optimizes in the
// background), and a rejected config should fail before the slow index builds.
func (s *Store) EnsureSchema(ctx context.Context) (err error) {
	ctx, span := tracer.Start(ctx, "store.EnsureSchema")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "EnsureSchema", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	if err := s.reconcileQuantization(ctx); err != nil {
		return fmt.Errorf("reconcile quantization (%s): %w", s.quantization, err)
	}
	// Indexes are ensured on every boot (idempotently) so existing collections
	// gain them without a data migration: Qdrant backfills the index over the
	// already-stored RFC3339 created_at strings and keyword payloads.
	return s.ensureIndexes(ctx, s.collection)
}

// reconcileQuantization brings the collection's quantization config to the
// store's mode, sending an update only when the current config differs.
func (s *Store) reconcileQuantization(ctx context.Context) error {
	if s.quantization != QuantizationInt8 && s.quantization != QuantizationOff {
		return nil // unmanaged: never read, never written
	}
	info, err := s.client.GetCollectionInfo(ctx, s.collection)
	if err != nil {
		return err
	}
	current := info.GetConfig().GetQuantizationConfig()

	var diff *qdrant.QuantizationConfigDiff
	switch s.quantization {
	case QuantizationInt8:
		if isInt8Target(current) {
			slog.DebugContext(ctx, "quantization already int8", "collection", s.collection)
			return nil
		}
		diff = qdrant.NewQuantizationDiffScalar(int8Scalar())
	case QuantizationOff:
		if current == nil {
			slog.DebugContext(ctx, "quantization already off", "collection", s.collection)
			return nil
		}
		diff = qdrant.NewQuantizationDiffDisabled()
	}
	if err := s.client.UpdateCollection(ctx, &qdrant.UpdateCollection{
		CollectionName:     s.collection,
		QuantizationConfig: diff,
	}); err != nil {
		return err
	}
	slog.InfoContext(ctx, "updated collection quantization",
		"collection", s.collection, "mode", string(s.quantization), "before", current.String())
	return nil
}

// isInt8Target compares only the three fields the int8 target sets. Qdrant
// may echo other fields (such as the deprecated always_ram) on read, so a
// whole-message comparison would report drift on every boot.
func isInt8Target(c *qdrant.QuantizationConfig) bool {
	sc := c.GetScalar()
	return sc != nil &&
		sc.GetType() == qdrant.QuantizationType_Int8 &&
		sc.GetQuantile() == int8Quantile &&
		sc.GetMemory() == qdrant.Memory_Pinned
}
