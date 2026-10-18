// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// snapshotRows are the records every view snapshot shows: each category,
// tags that overflow, a superseded and a scheduled record, and markdown.
func snapshotRows() []*engramv1.Memory {
	mk := func(short, cat, summary string, age time.Duration, tags ...string) *engramv1.Memory {
		r := testMemory(short, summary)
		r.Category, r.Tags = cat, tags
		r.CreatedAt = timestamppb.New(consoleNow.Add(-age))
		r.Content = "# " + summary + "\n\nWhy it matters, with `inline code`.\n\n```sh\ngit diff --stat go.sum\n```\n\n- first point\n- second point\n"
		return r
	}
	rows := []*engramv1.Memory{
		mk("aaaaaaaaa1", "decision", "Use git only; jj is retired", 7*time.Hour, "vcs", "git"),
		mk("bbbbbbbbb2", "gotcha", "go list -m pollutes go.sum under -mod=mod", 3*time.Hour, "go-sum", "go-modules", "golang", "gotcha"),
		mk("ccccccccc3", "convention", "TestMain isolates XDG_CONFIG_HOME for cmd/engram tests", 26*time.Hour, "testing"),
		mk("ddddddddd4", "decision", "Old upgrade plan", 9*24*time.Hour),
		mk("eeeeeeeee5", "preference", "Prefer rg over grep for searches", 40*24*time.Hour, "search"),
		mk("fffffffff6", "decision", "Revisit the release cadence", time.Hour),
	}
	rows[3].SupersededBy = stringPtr("id-aaaaaaaaa1")
	rows[5].NotBefore = timestamppb.New(consoleNow.Add(72 * time.Hour))
	rows[4].Visibility, rows[4].Owner = "shared", "other@example.com"
	return rows
}

// TestTUIViewSnapshots pins the console's layout at the three tier sizes,
// as text with escape sequences stripped, plus the engram theme's colours
// at one size. Regenerate with:
//
//	go test ./cmd/engram -run TestTUIViewSnapshots -update
func TestTUIViewSnapshots(t *testing.T) {
	for _, c := range []struct {
		name string
		w, h int
		raw  bool
	}{
		{"side_160x40", 160, 40, false},
		{"stacked_90x45", 90, 45, false},
		{"single_70x24", 70, 24, false},
		{"side_160x40_truecolor", 160, 40, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			rows := snapshotRows()
			svc := &stubEngramService{
				listFn: func(_ context.Context, req *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
					if req.GetPageToken() != "" {
						return &engramv1.ListMemoriesResponse{}, nil
					}
					return &engramv1.ListMemoriesResponse{Memories: rows, NextPageToken: "more",
						SearchedScopes:   []string{"repo:example/one", "repo:example/two", "repo:example/three"},
						RecallGateHidden: &engramv1.RecallGateHidden{Total: 3, Archived: 2, Superseded: 1}}, nil
				},
				getFn: getFrom(nil, rows...),
			}
			h := newSizedConsoleHarness(t, svc, c.w, c.h, consoleSettings{theme: "engram"},
				"repo:example/one", "repo:example/two", "repo:example/three")
			h.press("j")
			h.fire()
			got := h.m.View().Content
			if !c.raw {
				got = h.view()
			}
			var lines []string
			for _, l := range strings.Split(got, "\n") {
				lines = append(lines, strings.TrimRight(l, " "))
			}
			got = strings.Join(lines, "\n") + "\n"
			path := filepath.Join("testdata", "tui", c.name+".golden")
			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v (regenerate with -update)", path, err)
			}
			if got != string(want) {
				t.Errorf("%s has drifted; review the change and regenerate with -update\n--- got ---\n%s", path, got)
			}
		})
	}
}
