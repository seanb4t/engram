// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestOperatorOutputFormat is table-driven coverage of operatorOutputFormat
// over a throwaway *cobra.Command (never rootCmd or a live operator
// command): a legal value resolves to the matching outputFormat, an
// illegal value is rejected with exitUsage through the SAME
// config.ValidateOutputFormat the client tier calls (never a second,
// unvalidated switch).
func TestOperatorOutputFormat(t *testing.T) {
	newCmdWithBuffer := func() *cobra.Command {
		cmd := &cobra.Command{Use: "throwaway"}
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		return cmd
	}

	t.Run("json resolves to formatJSON", func(t *testing.T) {
		got, err := operatorOutputFormat(newCmdWithBuffer(), "json")
		if err != nil {
			t.Fatalf("operatorOutputFormat(_, \"json\") returned error: %v", err)
		}
		if got != formatJSON {
			t.Errorf("operatorOutputFormat(_, \"json\") = %v, want formatJSON", got)
		}
	})

	t.Run("text resolves to formatText", func(t *testing.T) {
		got, err := operatorOutputFormat(newCmdWithBuffer(), "text")
		if err != nil {
			t.Fatalf("operatorOutputFormat(_, \"text\") returned error: %v", err)
		}
		if got != formatText {
			t.Errorf("operatorOutputFormat(_, \"text\") = %v, want formatText", got)
		}
	})

	t.Run("empty resolves to formatJSON for a non-TTY writer", func(t *testing.T) {
		got, err := operatorOutputFormat(newCmdWithBuffer(), "")
		if err != nil {
			t.Fatalf("operatorOutputFormat(_, \"\") returned error: %v", err)
		}
		if got != formatJSON {
			t.Errorf("operatorOutputFormat(_, \"\") = %v, want formatJSON (non-TTY *bytes.Buffer)", got)
		}
	})

	t.Run("illegal value exits exitUsage via the shared validator", func(t *testing.T) {
		_, err := operatorOutputFormat(newCmdWithBuffer(), "yaml")
		if err == nil {
			t.Fatal("operatorOutputFormat(_, \"yaml\") = nil error, want a usage error")
		}
		if got := exitCodeFromError(err); got != exitUsage {
			t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
		}
		if !strings.Contains(err.Error(), "--output") {
			t.Errorf("error %q does not name --output", err)
		}
	})
}

// TestRenderOperatorTextAndJSON pins renderOperator's own two-mode
// contract, independent of any specific operator command: text mode
// writes text plus exactly one trailing newline; json mode marshals doc as
// exactly one JSON document plus a trailing newline, parseable via
// json.Unmarshal.
func TestRenderOperatorTextAndJSON(t *testing.T) {
	type doc struct {
		Count int `json:"count"`
	}

	t.Run("text mode", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := &cobra.Command{Use: "throwaway"}
		cmd.SetOut(&buf)
		if err := renderOperator(cmd, formatText, "hello", doc{Count: 3}); err != nil {
			t.Fatalf("renderOperator: %v", err)
		}
		if got, want := buf.String(), "hello\n"; got != want {
			t.Errorf("renderOperator text mode wrote %q, want %q", got, want)
		}
	})

	t.Run("json mode", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := &cobra.Command{Use: "throwaway"}
		cmd.SetOut(&buf)
		if err := renderOperator(cmd, formatJSON, "hello", doc{Count: 3}); err != nil {
			t.Fatalf("renderOperator: %v", err)
		}
		var got doc
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("json.Unmarshal(%q): %v", buf.String(), err)
		}
		if got.Count != 3 {
			t.Errorf("decoded doc.Count = %d, want 3", got.Count)
		}
		if strings.Count(buf.String(), "\n") != 1 {
			t.Errorf("renderOperator json mode wrote %q, want exactly one trailing newline", buf.String())
		}
	})
}
