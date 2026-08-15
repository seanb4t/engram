// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/spf13/cobra"
)

// TestWalkCommands is table-driven coverage of walkCommands and
// commandKey, built entirely over throwaway *cobra.Command trees — never
// rootCmd — so it exercises the walker's own contract independent of the
// live binary's registered commands.
func TestWalkCommands(t *testing.T) {
	t.Run("two-level tree returns parent-then-children in lexicographic order", func(t *testing.T) {
		root := &cobra.Command{Use: "root"}
		zChild := &cobra.Command{Use: "z-child"}
		aChild := &cobra.Command{Use: "a-child"}
		aGrandchild := &cobra.Command{Use: "grandchild"}
		aChild.AddCommand(aGrandchild)
		// Registered out of lexicographic order deliberately (z before a) —
		// the walker must sort each level by Name(), not registration order
		// (memory jb33frww29).
		root.AddCommand(zChild, aChild)

		got := walkCommands(root, commandWalkSkip)
		wantNames := []string{"a-child", "grandchild", "z-child"}
		if len(got) != len(wantNames) {
			t.Fatalf("walkCommands returned %d commands, want %d: %v", len(got), len(wantNames), got)
		}
		for i, cmd := range got {
			if cmd.Name() != wantNames[i] {
				t.Errorf("walkCommands()[%d].Name() = %q, want %q (parent-then-children, lexicographic)", i, cmd.Name(), wantNames[i])
			}
		}
	})

	t.Run("hidden parent removes its whole subtree", func(t *testing.T) {
		root := &cobra.Command{Use: "root"}
		hidden := &cobra.Command{Use: "hidden-group", Hidden: true}
		hiddenChild := &cobra.Command{Use: "should-not-appear"}
		hidden.AddCommand(hiddenChild)
		visible := &cobra.Command{Use: "visible"}
		root.AddCommand(hidden, visible)

		got := walkCommands(root, commandWalkSkip)
		if len(got) != 1 || got[0].Name() != "visible" {
			names := make([]string, len(got))
			for i, c := range got {
				names[i] = c.Name()
			}
			t.Errorf("walkCommands() = %v, want only [\"visible\"] — a hidden parent's subtree must not be visited at all", names)
		}
	})

	t.Run("help and completion are skipped at every depth", func(t *testing.T) {
		root := &cobra.Command{Use: "root"}
		group := &cobra.Command{Use: "group"}
		help := &cobra.Command{Use: "help"}
		completion := &cobra.Command{Use: "completion"}
		leaf := &cobra.Command{Use: "leaf"}
		group.AddCommand(help, completion, leaf)
		root.AddCommand(group)

		got := walkCommands(root, commandWalkSkip)
		wantNames := map[string]bool{"group": true, "leaf": true}
		if len(got) != len(wantNames) {
			names := make([]string, len(got))
			for i, c := range got {
				names[i] = c.Name()
			}
			t.Fatalf("walkCommands() = %v, want exactly %v", names, wantNames)
		}
		for _, c := range got {
			if !wantNames[c.Name()] {
				t.Errorf("walkCommands() unexpectedly included %q (help/completion scaffolding must be skipped at every depth, not just the top)", c.Name())
			}
		}
	})

	t.Run("commandKey returns bare name for a direct child of root", func(t *testing.T) {
		root := &cobra.Command{Use: "engram"}
		child := &cobra.Command{Use: "reindex"}
		root.AddCommand(child)

		if got, want := commandKey(child), "reindex"; got != want {
			t.Errorf("commandKey(child) = %q, want %q", got, want)
		}
	})

	t.Run("commandKey returns the space-joined path for a grandchild", func(t *testing.T) {
		root := &cobra.Command{Use: "engram"}
		group := &cobra.Command{Use: "spine-review"}
		leaf := &cobra.Command{Use: "scan"}
		group.AddCommand(leaf)
		root.AddCommand(group)

		if got, want := commandKey(leaf), "spine-review scan"; got != want {
			t.Errorf("commandKey(leaf) = %q, want %q", got, want)
		}
	})
}

// wantOperatorCommandKeys is the hand-known expected membership of
// operatorCommands() over the LIVE rootCmd tree: the six pre-existing
// operator commands this plan backfills plus spine-review scan (plan
// 03-01's first operator-tier leaf), spine-review verify (plan 03-04's
// leaf), spine-review consolidate (plan 03-05's leaf), spine-review
// archive/restore (plan 03-06's leaves), and migrate / migrate status /
// migrate revert (04-03-PLAN.md's three migrate-family leaves, landed
// across Tasks 2 and 3) — never search/list/store (the client tier, excluded by the
// "server" flag check) and never spine-review or migrate itself as a
// non-runnable parent (migrate DOES have its own RunE via
// registerDestructive, so it IS a member; only spine-review, the
// non-runnable group, is excluded by the RunE check) or serve/version (the
// named exclusion set).
var wantOperatorCommandKeys = map[string]bool{
	"reindex":                  true,
	"prune-expired":            true,
	"summarize-missing":        true,
	"backfill-short-ids":       true,
	"migrate-remap-owner":      true,
	"migrate-set-owner":        true,
	"spine-review scan":        true,
	"spine-review verify":      true,
	"spine-review consolidate": true,
	"spine-review archive":     true,
	"spine-review restore":     true,
	"spine-review purge":       true,
	"migrate":                  true,
	"migrate status":           true,
	"migrate revert":           true,
}

// commandKeySet is a small helper turning a []*cobra.Command into a
// commandKey set for both-directions comparison.
func commandKeySet(cmds []*cobra.Command) map[string]bool {
	set := make(map[string]bool, len(cmds))
	for _, c := range cmds {
		set[commandKey(c)] = true
	}
	return set
}

// TestOperatorCommands is the phase's structural-predicate gate (review
// #19): operatorCommands() is a concrete definition of operator-tier
// membership, not an informal reading of walkCommands' own (tier-agnostic)
// traversal. Two claims, both both-directions:
//
//  1. operatorCommands()'s commandKey set equals the hand-known expected
//     set over the live tree, in both directions — a command silently
//     dropped from (or added to) the predicate is caught either way.
//  2. The named exclusion set is EXACTLY {"serve", "version"}, and both
//     names resolve to a live command in the tree — a future command
//     added to the exclusion set without also existing in the tree (or a
//     stale entry left behind after a rename) is caught.
func TestOperatorCommands(t *testing.T) {
	got := commandKeySet(operatorCommands())

	for key := range wantOperatorCommandKeys {
		if !got[key] {
			t.Errorf("operatorCommands() is missing %q", key)
		}
	}
	for key := range got {
		if !wantOperatorCommandKeys[key] {
			t.Errorf("operatorCommands() unexpectedly includes %q", key)
		}
	}

	wantExclusions := map[string]bool{"serve": true, "version": true}
	if len(operatorCommandExclusions) != len(wantExclusions) {
		t.Fatalf("operatorCommandExclusions has %d entries, want %d: %v", len(operatorCommandExclusions), len(wantExclusions), operatorCommandExclusions)
	}
	for name := range wantExclusions {
		if !operatorCommandExclusions[name] {
			t.Errorf("operatorCommandExclusions is missing %q", name)
		}
	}

	liveKeys := commandKeySet(walkCommands(rootCmd, commandWalkSkip))
	for name := range operatorCommandExclusions {
		if !liveKeys[name] {
			t.Errorf("operatorCommandExclusions names %q, which does not resolve to a live command in the tree", name)
		}
	}
}

// TestCatalogOutputFlagMatchesOperatorCommandsUnionClientVerbs decodes the
// committed catalog.golden fixture and asserts SET EQUALITY between the
// set of command names whose flag list carries "output" and
// operatorCommands()'s commandKey set unioned with the three client verbs
// (search, list, store). This is the behavioural gate the plan's
// acceptance criteria call for — an rg-based literal count over the golden
// text would pass on two offsetting errors; decoding the JSON and
// comparing sets cannot.
func TestCatalogOutputFlagMatchesOperatorCommandsUnionClientVerbs(t *testing.T) {
	b, err := os.ReadFile("testdata/catalog.golden")
	if err != nil {
		t.Fatalf("reading testdata/catalog.golden: %v", err)
	}
	var doc catalogDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("json.Unmarshal(catalog.golden): %v", err)
	}

	golden := make(map[string]bool)
	for _, c := range doc.Commands {
		for _, f := range c.Flags {
			if f.Name == "output" {
				golden[c.Name] = true
				break
			}
		}
	}

	want := commandKeySet(operatorCommands())
	want["search"] = true
	want["list"] = true
	want["store"] = true

	for key := range want {
		if !golden[key] {
			t.Errorf("catalog.golden's --output-bearing command set is missing %q", key)
		}
	}
	for key := range golden {
		if !want[key] {
			t.Errorf("catalog.golden's --output-bearing command set unexpectedly includes %q (not in operatorCommands() union the three client verbs)", key)
		}
	}
}
