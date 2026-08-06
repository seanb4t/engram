// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
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
