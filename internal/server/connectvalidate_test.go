// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"testing"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// fakeInternalErrorValidator implements protovalidate.Validator, always
// returning a sentinel error that is NOT a *protovalidate.ValidationError.
// A real protovalidate.Validator over valid generated constraints only ever
// returns nil or *ValidationError, so the CodeInternal branch is otherwise
// unreachable (review finding #5) — this fake exercises it directly.
type fakeInternalErrorValidator struct{}

func (fakeInternalErrorValidator) Validate(_ proto.Message, _ ...protovalidate.ValidationOption) error {
	return errors.New("boom")
}

func TestConnectValidateInterceptor(t *testing.T) {
	t.Run("valid_message_passes_through", func(t *testing.T) {
		v, err := protovalidate.New()
		if err != nil {
			t.Fatalf("protovalidate.New: %v", err)
		}
		nextCalled := false
		next := connect.UnaryFunc(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
			nextCalled = true
			return nil, nil
		})
		interceptor := newConnectValidateInterceptor(v)
		handler := interceptor(next)

		req := connect.NewRequest(&engramv1.SetVisibilityRequest{
			Id:         "some-id",
			Visibility: engramv1.Visibility_VISIBILITY_SHARED,
		})
		_, err = handler(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !nextCalled {
			t.Fatal("next handler must be called for a valid message")
		}
	})

	t.Run("invalid_message_returns_invalid_argument", func(t *testing.T) {
		v, err := protovalidate.New()
		if err != nil {
			t.Fatalf("protovalidate.New: %v", err)
		}
		nextCalled := false
		next := connect.UnaryFunc(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
			nextCalled = true
			return nil, nil
		})
		interceptor := newConnectValidateInterceptor(v)
		handler := interceptor(next)

		// Empty id (min_len=1) and unspecified visibility (enum not_in [0])
		// both violate constraints from proto/engram/v1/engram.proto.
		req := connect.NewRequest(&engramv1.SetVisibilityRequest{})
		_, err = handler(context.Background(), req)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
		}
		if nextCalled {
			t.Error("next handler must NOT be called when validation fails")
		}
	})

	t.Run("non_validation_error_maps_to_internal", func(t *testing.T) {
		nextCalled := false
		next := connect.UnaryFunc(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
			nextCalled = true
			return nil, nil
		})
		interceptor := newConnectValidateInterceptor(fakeInternalErrorValidator{})
		handler := interceptor(next)

		req := connect.NewRequest(&engramv1.SetVisibilityRequest{
			Id:         "some-id",
			Visibility: engramv1.Visibility_VISIBILITY_SHARED,
		})
		_, err := handler(context.Background(), req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Errorf("expected CodeInternal, got %v", connect.CodeOf(err))
		}
		if nextCalled {
			t.Error("next handler must NOT be called when validator returns a non-validation error")
		}
	})

	t.Run("non_proto_request_passes_through_defensively", func(t *testing.T) {
		v, err := protovalidate.New()
		if err != nil {
			t.Fatalf("protovalidate.New: %v", err)
		}
		nextCalled := false
		next := connect.UnaryFunc(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
			nextCalled = true
			return nil, nil
		})
		interceptor := newConnectValidateInterceptor(v)
		handler := interceptor(next)

		_, err = handler(context.Background(), connect.NewRequest(&struct{}{}))
		if err != nil {
			t.Fatalf("unexpected error for non-proto request: %v", err)
		}
		if !nextCalled {
			t.Fatal("next handler must be called for a non-proto request")
		}
	})
}
