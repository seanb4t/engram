// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"errors"
	"os"
	"testing"

	"connectrpc.com/connect"
)

// TestExitCodeForConnectErrTable is the D-10 mapper table test: every
// connect.Code from 1 through 16, plus an error that is not a
// *connect.Error at all.
func TestExitCodeForConnectErrTable(t *testing.T) {
	cases := []struct {
		code connect.Code
		want int
	}{
		{connect.CodeCanceled, exitUnavailable},
		{connect.CodeUnknown, exitGeneric},
		{connect.CodeInvalidArgument, exitUsage},
		{connect.CodeDeadlineExceeded, exitUnavailable},
		{connect.CodeNotFound, exitNotFound},
		{connect.CodeAlreadyExists, exitGeneric},
		{connect.CodePermissionDenied, exitAuth},
		{connect.CodeResourceExhausted, exitGeneric},
		{connect.CodeFailedPrecondition, exitUsage},
		{connect.CodeAborted, exitGeneric},
		{connect.CodeOutOfRange, exitUsage},
		{connect.CodeUnimplemented, exitGeneric},
		{connect.CodeInternal, exitGeneric},
		{connect.CodeUnavailable, exitUnavailable},
		{connect.CodeDataLoss, exitGeneric},
		{connect.CodeUnauthenticated, exitAuth},
	}
	if len(cases) != 16 {
		t.Fatalf("test table has %d entries, want 16 (one per connect.Code)", len(cases))
	}
	for _, c := range cases {
		t.Run(c.code.String(), func(t *testing.T) {
			err := connect.NewError(c.code, errors.New("boom"))
			if got := exitCodeForConnectErr(err); got != c.want {
				t.Errorf("exitCodeForConnectErr(%v) = %d, want %d", c.code, got, c.want)
			}
		})
	}
	t.Run("not a connect.Error", func(t *testing.T) {
		if got := exitCodeForConnectErr(errors.New("plain")); got != exitGeneric {
			t.Errorf("exitCodeForConnectErr(plain error) = %d, want %d", got, exitGeneric)
		}
	})
}

// TestResolveOutputFormat is the D-05/D-06 table test over the six
// TTY/flag combinations plus an invalid --output value.
func TestResolveOutputFormat(t *testing.T) {
	cases := []struct {
		name    string
		flagVal string
		isTTY   bool
		want    outputFormat
	}{
		{"empty tty=true", "", true, formatText},
		{"empty tty=false", "", false, formatJSON},
		{"json tty=true", "json", true, formatJSON},
		{"json tty=false", "json", false, formatJSON},
		{"text tty=true", "text", true, formatText},
		{"text tty=false", "text", false, formatText},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := resolveOutputFormat(c.flagVal, c.isTTY)
			if err != nil {
				t.Fatalf("resolveOutputFormat(%q, %v) returned error: %v", c.flagVal, c.isTTY, err)
			}
			if got != c.want {
				t.Errorf("resolveOutputFormat(%q, %v) = %v, want %v", c.flagVal, c.isTTY, got, c.want)
			}
		})
	}
	t.Run("invalid value", func(t *testing.T) {
		_, err := resolveOutputFormat("yaml", false)
		if err == nil {
			t.Fatal("expected an error for an invalid --output value")
		}
		var ec interface{ ExitCode() int }
		if !errors.As(err, &ec) {
			t.Fatalf("error %v does not carry ExitCode()", err)
		}
		if ec.ExitCode() != exitUsage {
			t.Errorf("ExitCode() = %d, want %d", ec.ExitCode(), exitUsage)
		}
	})
}

// TestIsTerminalOnNonTTY confirms isTerminal is false for a pipe and for a
// regular file — the only two non-pty cases a test can exercise. The
// positive branch (a real pty) cannot be exercised without one, which is
// why resolveOutputFormat takes the boolean as a parameter and is tested
// directly for isTTY=true above; this gap is recorded here, not hidden.
func TestIsTerminalOnNonTTY(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	if isTerminal(r) {
		t.Error("isTerminal(pipe read end) = true, want false")
	}

	f, err := os.CreateTemp(t.TempDir(), "isterm")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer f.Close()
	if isTerminal(f) {
		t.Error("isTerminal(regular file) = true, want false")
	}
}
