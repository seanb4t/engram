// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"regexp"
	"slices"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/seanb4t/engram/internal/surfaces"
)

// newConnectValidateInterceptor returns a unary interceptor that validates
// every request message against its buf.validate constraints via v. It must
// run AFTER the subject interceptor (D-10): an unauthenticated caller gets
// CodeUnauthenticated and never sees field-level validation detail.
//
// A *protovalidate.ValidationError becomes the shared field=/hint= envelope
// through violationError and connectError, so its Connect code follows the
// failure's class. Any other non-nil error maps to CodeInternal. A request
// payload that is not a proto.Message passes through defensively (every
// generated Connect request is one, but this avoids a panic if that ever
// changes).
func newConnectValidateInterceptor(v protovalidate.Validator) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			msg, ok := req.Any().(proto.Message)
			if !ok {
				return next(ctx, req)
			}
			if err := v.Validate(msg); err != nil {
				if valErr, ok := errors.AsType[*protovalidate.ValidationError](err); ok {
					return nil, connectError(ctx, violationError(valErr))
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			return next(ctx, req)
		}
	}
}

// requestPseudoField names a rejection that has no field of its own: a
// message rule with no mapped hint.
const requestPseudoField = "request"

// updateMaskRuleID is the id of UpdateMemoryRequest's message-level CEL rule.
const updateMaskRuleID = "update_memory.mask"

// violationMapping is the hint and class of one protovalidate rule.
type violationMapping struct {
	hint  HintCode
	class argClass
}

// fieldRuleMappings gives each protovalidate field rule id in engram.proto its
// hint and class. A rule id with no entry gets HintFormat and classMalformed.
var fieldRuleMappings = map[string]violationMapping{
	"string.min_len":     {HintRequired, classMalformed},
	"required":           {HintRequired, classMalformed},
	"repeated.min_items": {HintRequired, classMalformed},
	"string.in":          {HintEnum, classMalformed},
	"enum.not_in":        {HintEnum, classMalformed},
	"string.prefix":      {HintPrefix, classMalformed},
	"string.max_bytes":   {HintTooLong, classOutOfRange},
	"repeated.max_items": {HintTooMany, classOutOfRange},
}

// surfaceRuleClasses gives the class of each message-level CEL rule whose id
// is an internal/surfaces rule id. The classes match parseWindow's.
var surfaceRuleClasses = map[string]argClass{
	surfaces.RuleScheduleWindowAtLeastOne: classMalformed,
	surfaces.RuleWindowOrdering:           classPrecondition,
}

// indexRE matches a list index in a protovalidate field path.
var indexRE = regexp.MustCompile(`\[\d+\]`)

// violationError turns a ValidationError into the one *argError the caller
// sees. A field violation takes precedence over a message-level one, and the
// detail never holds the rejected value.
func violationError(ve *protovalidate.ValidationError) error {
	i := slices.IndexFunc(ve.Violations, func(v *protovalidate.Violation) bool {
		return len(v.Proto.GetField().GetElements()) > 0
	})
	if i >= 0 {
		return fieldViolationError(ve.Violations[i])
	}
	if len(ve.Violations) > 0 {
		return messageViolationError(ve.Violations[0])
	}
	return argErrf(classMalformed, HintFormat, requestPseudoField, "request failed validation")
}

// fieldViolationError names the core's spelling of the field, so that
// citations[0].kind becomes citations[i].kind.
func fieldViolationError(v *protovalidate.Violation) error {
	field := indexRE.ReplaceAllString(protovalidate.FieldPathString(v.Proto.GetField()), "[i]")
	m, ok := fieldRuleMappings[v.Proto.GetRuleId()]
	if !ok {
		m = violationMapping{HintFormat, classMalformed}
	}
	return argErrf(m.class, m.hint, field, "%s %s", field, v.Proto.GetMessage())
}

// messageViolationError renders a message-level CEL rule from its surface
// rule where it has one.
func messageViolationError(v *protovalidate.Violation) error {
	id := v.Proto.GetRuleId()
	if rule, ok := surfaces.RuleByID(id); ok {
		if class, ok := surfaceRuleClasses[id]; ok {
			return conditionalErrf(class, rule)
		}
	}
	if id == updateMaskRuleID {
		return argErrf(classMalformed, HintEnum, "update_mask", "%s", v.Proto.GetMessage())
	}
	return argErrf(classMalformed, HintFormat, requestPseudoField, "%s", v.Proto.GetMessage())
}
