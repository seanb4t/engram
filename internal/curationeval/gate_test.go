// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package curationeval

import (
	"strings"
	"testing"
)

// TestResolveCurationEvalGate is hermetic: every row injects its own
// environment through resolveEvalGate's environ argument, never t.Setenv,
// so it needs no gate of its own and always runs in ordinary CI (D-15). It
// must NOT carry the TestCurationEval name prefix.
func TestResolveCurationEvalGate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		environ    []string
		wantOn     bool
		wantPath   string
		wantErrAll []string // substrings the error must contain; nil means no error
	}{
		{"unset gives false and empty path", nil, false, "", nil},
		{"empty value keeps default", []string{"ENGRAM_CURATION_EVAL="}, false, "", nil},
		{"1 enables", []string{"ENGRAM_CURATION_EVAL=1"}, true, "", nil},
		{"true enables", []string{"ENGRAM_CURATION_EVAL=true"}, true, "", nil},
		{"0 disables", []string{"ENGRAM_CURATION_EVAL=0"}, false, "", nil},
		{"false disables", []string{"ENGRAM_CURATION_EVAL=false"}, false, "", nil},
		{"malformed value errors naming the var", []string{"ENGRAM_CURATION_EVAL=maybe"}, false, "", []string{"ENGRAM_CURATION_EVAL", "maybe"}},
		{
			"unrelated malformed var is ignored",
			[]string{"ENGRAM_CURATION_EVAL=1", "ENGRAM_EMBED_DIM=abc"},
			true, "", nil,
		},
		{
			"pairs path is returned verbatim",
			[]string{"ENGRAM_CURATION_EVAL_PAIRS=/tmp/x.jsonl"},
			false, "/tmp/x.jsonl", nil,
		},
		{
			"gate and pairs path together",
			[]string{"ENGRAM_CURATION_EVAL=1", "ENGRAM_CURATION_EVAL_PAIRS=/tmp/x.jsonl"},
			true, "/tmp/x.jsonl", nil,
		},
		{"look-alike missing prefix is ignored", []string{"CURATION_EVAL=1"}, false, "", nil},
		{"look-alike wrong prefix is ignored", []string{"XENGRAM_CURATION_EVAL=1"}, false, "", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotOn, gotPath, err := resolveEvalGate(func() []string { return tc.environ })
			if tc.wantErrAll != nil {
				if err == nil {
					t.Fatalf("resolveEvalGate(%v) = (%v, %v, nil), want an error", tc.environ, gotOn, gotPath)
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
			if gotOn != tc.wantOn {
				t.Errorf("resolveEvalGate(%v) enabled = %v, want %v", tc.environ, gotOn, tc.wantOn)
			}
			if gotPath != tc.wantPath {
				t.Errorf("resolveEvalGate(%v) path = %q, want %q", tc.environ, gotPath, tc.wantPath)
			}
		})
	}
}
