// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
)

// newConnectValidateInterceptor returns a unary interceptor that validates
// every request message against its buf.validate constraints via v. It must
// run AFTER the subject interceptor (D-10): an unauthenticated caller gets
// CodeUnauthenticated and never sees field-level validation detail.
//
// A *protovalidate.ValidationError maps to CodeInvalidArgument; any other
// non-nil error maps to CodeInternal. A request payload that is not a
// proto.Message passes through defensively (every generated Connect request
// is one, but this avoids a panic if that ever changes).
func newConnectValidateInterceptor(v protovalidate.Validator) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			msg, ok := req.Any().(proto.Message)
			if !ok {
				return next(ctx, req)
			}
			if err := v.Validate(msg); err != nil {
				var valErr *protovalidate.ValidationError
				if errors.As(err, &valErr) {
					return nil, connect.NewError(connect.CodeInvalidArgument, valErr)
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			return next(ctx, req)
		}
	}
}
