// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// consoleKeyMap is the console's one source of keys: the footer, the help
// overlay and the command palette are all built from it, so no key can
// show in one place and be missing from another. It uses the web
// console's key wherever one exists for the same action, and never reuses
// a web-console write key (a, A, S, s, e, #, x, X) for something else.
type consoleKeyMap struct {
	Up, Down, PageUp, PageDown, Home, End  key.Binding
	Open, Back, Focus                      key.Binding
	ScrollDown, ScrollUp, HalfDown, HalfUp key.Binding
	ScrollPane                             key.Binding
	Search, Narrow, Filters                key.Binding
	Memories, Tags, Rules, Scheduled       key.Binding
	Related, TrailBack, Choose, Window     key.Binding
	More, HidePreview, Maximise            key.Binding
	CopyShort, CopyID, Theme, Palette      key.Binding
	Help, Quit, ForceQuit                  key.Binding
}

func newConsoleKeyMap() consoleKeyMap {
	b := func(help, desc string, keys ...string) key.Binding {
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(help, desc))
	}
	return consoleKeyMap{
		Up:          b("k/↑", "up", "k", "up"),
		Down:        b("j/↓", "down", "j", "down"),
		PageUp:      b("pgup", "page up", "pgup"),
		PageDown:    b("pgdn", "page down", "pgdown"),
		Home:        b("home", "first", "home"),
		End:         b("end", "last", "end"),
		Open:        b("enter", "open", "enter"),
		Back:        b("esc", "back", "esc"),
		Focus:       b("tab", "switch pane", "tab", "shift+tab"),
		ScrollDown:  b("J", "scroll preview down", "J"),
		ScrollUp:    b("K", "scroll preview up", "K"),
		HalfDown:    b("ctrl+d", "preview half page down", "ctrl+d"),
		HalfUp:      b("ctrl+u", "preview half page up", "ctrl+u"),
		ScrollPane:  b("j/k", "scroll", "j", "k", "down", "up"),
		Search:      b("/", "search", "/"),
		Narrow:      b("=", "narrow loaded rows", "="),
		Filters:     b("f", "scope and filters", "f"),
		Memories:    b("1", "memories", "1"),
		Tags:        b("2", "tags", "2"),
		Rules:       b("3", "rules", "3"),
		Scheduled:   b("4", "scheduled", "4"),
		Related:     b("r", "related", "r"),
		TrailBack:   b("[", "back along trail", "["),
		Choose:      b("space", "choose tag", "space"),
		Window:      b("w", "scheduled/expired", "w"),
		More:        b("n", "next page", "n"),
		HidePreview: b("p", "hide preview", "p"),
		Maximise:    b("z", "maximise pane", "z"),
		CopyShort:   b("c", "copy short_id", "c"),
		CopyID:      b("C", "copy id", "C"),
		Theme:       b("T", "theme", "T"),
		Palette:     b("ctrl+k", "commands", "ctrl+k"),
		Help:        b("?", "help", "?"),
		Quit:        b("q", "quit", "q"),
		ForceQuit:   b("ctrl+c", "quit", "ctrl+c"),
	}
}

// groups are the help overlay's sections, every binding but ScrollPane
// (the preview's own j/k, listed in the footer when the preview has focus).
func (k consoleKeyMap) groups() []keyGroup {
	return []keyGroup{
		{"Move", []key.Binding{k.Up, k.Down, k.PageUp, k.PageDown, k.Home, k.End}},
		{"Open", []key.Binding{k.Open, k.Back, k.Focus, k.Related, k.TrailBack}},
		{"Preview", []key.Binding{k.ScrollDown, k.ScrollUp, k.HalfDown, k.HalfUp, k.HidePreview, k.Maximise}},
		{"Find", []key.Binding{k.Search, k.Narrow, k.Filters, k.More, k.Choose, k.Window}},
		{"Views", []key.Binding{k.Memories, k.Tags, k.Rules, k.Scheduled}},
		{"Other", []key.Binding{k.CopyShort, k.CopyID, k.Theme, k.Palette, k.Help, k.Quit, k.ForceQuit}},
	}
}

type keyGroup struct {
	title    string
	bindings []key.Binding
}

// all is every binding in the help overlay and the palette.
func (k consoleKeyMap) all() []key.Binding {
	var out []key.Binding
	for _, g := range k.groups() {
		out = append(out, g.bindings...)
	}
	return out
}

// footer is the bottom line's bindings for whatever has focus.
func (k consoleKeyMap) footer(f consoleFocus) []key.Binding {
	if f == focusPreview {
		return []key.Binding{k.ScrollPane, k.Back, k.HalfDown, k.HalfUp, k.Related, k.CopyShort, k.Focus, k.Maximise}
	}
	return []key.Binding{k.Open, k.Search, k.Narrow, k.Related, k.Filters, k.Palette}
}

// keyPress builds the key event a binding's first key sends, so running a
// palette entry is exactly pressing its key.
func keyPress(s string) tea.KeyPressMsg {
	special := map[string]rune{
		"enter": tea.KeyEnter, "esc": tea.KeyEscape, "tab": tea.KeyTab, "space": tea.KeySpace,
		"up": tea.KeyUp, "down": tea.KeyDown, "pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown,
		"home": tea.KeyHome, "end": tea.KeyEnd,
	}
	switch {
	case s == "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case strings.HasPrefix(s, "ctrl+"):
		return tea.KeyPressMsg{Code: []rune(strings.TrimPrefix(s, "ctrl+"))[0], Mod: tea.ModCtrl}
	}
	if code, ok := special[s]; ok {
		if code == tea.KeySpace {
			return tea.KeyPressMsg{Code: code, Text: " "}
		}
		return tea.KeyPressMsg{Code: code}
	}
	return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
}
