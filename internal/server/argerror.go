// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"errors"
	"fmt"

	"connectrpc.com/connect"

	"github.com/seanb4t/engram/internal/store"
)

// HintCode is a machine-stable remediation code an agent can branch on. It is
// string-backed rather than int-backed so it serialises to a legible token on
// both wire lanes (the MCP error string and, if ever needed, a Connect detail)
// with no lookup table, while still being a Go type the compiler checks at
// every authoring site (D-10).
type HintCode string

// The approved hint vocabulary (04-CONTEXT.md D-09/D-17 checkpoint). Do not
// add a code here without updating the checkpoint record in
// 04-01-SUMMARY.md — the vocabulary is a published wire contract on the MCP
// lane.
const (
	HintRequired            HintCode = "required"
	HintConditionalRequired HintCode = "conditional_required"
	HintTooLong             HintCode = "too_long"
	HintTooMany             HintCode = "too_many"
	HintEnum                HintCode = "enum"
	HintFormat              HintCode = "format"
	HintPrefix              HintCode = "prefix"
	HintOrdering            HintCode = "ordering"
	HintMutuallyExclusive   HintCode = "mutually_exclusive"
	HintNotApplicable       HintCode = "not_applicable"
)

// argClass is the failure CLASS that selects the Connect error code (D-11,
// D-20). Classification is by class, never by message text — the
// string-matching habit is what produced the #360 misattribution class in
// the first place.
type argClass int

const (
	// classMalformed: wrong shape/value — absent, unparseable, not in an
	// enum, wrong prefix. Maps to connect.CodeInvalidArgument.
	classMalformed argClass = iota
	// classOutOfRange: right shape, wrong magnitude. Maps to
	// connect.CodeOutOfRange.
	classOutOfRange
	// classPrecondition: a relationship or state constraint between two
	// individually-valid fields, not one value. Maps to
	// connect.CodeFailedPrecondition.
	classPrecondition
)

// argError is the ONE envelope carrying both the D-05 field attribution and
// the D-09 remediation hint — a single mechanism, not two. Fields is a slice
// rather than a single string so a relational rejection (not_before vs
// not_after, cursor_mode vs offset) has a home in the same envelope; a
// single-field rejection carries exactly one element.
//
// D-12: Detail states the constraint and names the bound — it never
// interpolates the caller's rejected VALUE. The one exception already
// present in the pre-conversion tree, `kind must be "map" or "fact", got %q`,
// is NOT carried forward: the enum's legal values are the useful, bounded
// half; the illegal value the caller just sent is the unbounded,
// caller-controlled half, and it is dropped.
type argError struct {
	Fields []string
	Hint   HintCode
	Detail string
	Class  argClass
}

// Error renders the field-first grammar approved in the D-17 checkpoint:
//
//	field=<f1>,<f2> hint=<code>: <detail>
//
// Per go-sdk@v1.6.1/mcp/server.go:340-354, the SDK discards the built
// *CallToolResult on a non-nil error and returns only err.Error() as text —
// this string IS the entire MCP wire payload for a rejected tool call, not a
// debug aid.
func (e *argError) Error() string {
	fields := ""
	for i, f := range e.Fields {
		if i > 0 {
			fields += ","
		}
		fields += f
	}
	return "field=" + fields + " hint=" + string(e.Hint) + ": " + e.Detail
}

// Unwrap returns store.ErrInvalidArgument. Load-bearing for back-compat:
// every existing errors.Is(err, store.ErrInvalidArgument) consumer keeps
// working unchanged across the whole D-06 sweep without being touched.
func (e *argError) Unwrap() error {
	return store.ErrInvalidArgument
}

// ConnectCode maps this error's Class to a Connect error code, per the D-11
// class table confirmed in the D-20 checkpoint resolution. All three arms sit
// inside the trio exitCodeForConnectErr (cmd/engram/client_common.go:237-249)
// already groups under exitUsage, so the shipped CLI exit-code contract is
// unchanged. The default arm returns CodeInvalidArgument so an unhandled
// future class degrades to today's behavior rather than to CodeInternal.
func (e *argError) ConnectCode() connect.Code {
	switch e.Class {
	case classOutOfRange:
		return connect.CodeOutOfRange
	case classPrecondition:
		return connect.CodeFailedPrecondition
	case classMalformed:
		return connect.CodeInvalidArgument
	default:
		return connect.CodeInvalidArgument
	}
}

// argErrf constructs a single-field argError. It returns error, not
// *argError, so a nil-typed-pointer-in-non-nil-interface mistake is
// impossible at call sites.
func argErrf(class argClass, hint HintCode, field, format string, a ...any) error {
	return &argError{
		Fields: []string{field},
		Hint:   hint,
		Detail: fmt.Sprintf(format, a...),
		Class:  class,
	}
}

// argErrFieldsf constructs a multi-field argError, for relational rejections
// (Pitfall 4) where more than one field participates in the violated
// constraint.
func argErrFieldsf(class argClass, hint HintCode, fields []string, format string, a ...any) error {
	return &argError{
		Fields: fields,
		Hint:   hint,
		Detail: fmt.Sprintf(format, a...),
		Class:  class,
	}
}

// argFieldsOf returns the field names attributed to err, or nil if err is not
// (or does not wrap) an *argError.
func argFieldsOf(err error) []string {
	var ae *argError
	if errors.As(err, &ae) {
		return ae.Fields
	}
	return nil
}

// argHintOf returns the hint code attributed to err, or "" if err is not (or
// does not wrap) an *argError.
func argHintOf(err error) HintCode {
	var ae *argError
	if errors.As(err, &ae) {
		return ae.Hint
	}
	return ""
}

// argClassOf returns the class attributed to err and true, or the zero class
// and false if err is not (or does not wrap) an *argError.
func argClassOf(err error) (argClass, bool) {
	var ae *argError
	if errors.As(err, &ae) {
		return ae.Class, true
	}
	return 0, false
}
