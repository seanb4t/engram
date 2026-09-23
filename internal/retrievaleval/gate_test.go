// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package retrievaleval

import (
	"strings"
	"testing"
)

// TestEvalGate is hermetic: every row injects its own environment through
// resolveEvalGate's environ argument, never t.Setenv, so it needs no gate of
// its own and always runs in ordinary CI (D-15). It must NOT carry the
// TestRetrievalEval name prefix.
func TestEvalGate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		environ    []string
		want       bool
		wantErrAll []string // substrings the error must contain; nil means no error
	}{
		{"empty environ", nil, false, nil},
		{"empty value keeps default", []string{"ENGRAM_RETRIEVAL_EVAL="}, false, nil},
		{"1 enables", []string{"ENGRAM_RETRIEVAL_EVAL=1"}, true, nil},
		{"true enables", []string{"ENGRAM_RETRIEVAL_EVAL=true"}, true, nil},
		{"TRUE enables", []string{"ENGRAM_RETRIEVAL_EVAL=TRUE"}, true, nil},
		{"0 disables", []string{"ENGRAM_RETRIEVAL_EVAL=0"}, false, nil},
		{"false disables", []string{"ENGRAM_RETRIEVAL_EVAL=false"}, false, nil},
		{"malformed value errors", []string{"ENGRAM_RETRIEVAL_EVAL=yes"}, false, []string{"ENGRAM_RETRIEVAL_EVAL", "yes"}},
		{
			"unrelated malformed vars with gate unset are ignored",
			[]string{"ENGRAM_EMBED_DIM=abc", "ENGRAM_QDRANT_ADDR=::bad"},
			false, nil,
		},
		{
			"unrelated malformed vars with gate set are still ignored",
			[]string{"ENGRAM_EMBED_DIM=abc", "ENGRAM_QDRANT_ADDR=::bad", "ENGRAM_RETRIEVAL_EVAL=1"},
			true, nil,
		},
		{"look-alike missing prefix is ignored", []string{"RETRIEVAL_EVAL=1"}, false, nil},
		{"look-alike wrong prefix is ignored", []string{"XENGRAM_RETRIEVAL_EVAL=1"}, false, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveEvalGate(func() []string { return tc.environ })
			if tc.wantErrAll != nil {
				if err == nil {
					t.Fatalf("resolveEvalGate(%v) = (%v, nil), want an error", tc.environ, got)
				}
				for _, want := range tc.wantErrAll {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("resolveEvalGate(%v) error %q missing %q", tc.environ, err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveEvalGate(%v) unexpected error: %v", tc.environ, err)
			}
			if got != tc.want {
				t.Errorf("resolveEvalGate(%v) = %v, want %v", tc.environ, got, tc.want)
			}
		})
	}
}
