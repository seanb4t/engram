// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"

	"github.com/seanb4t/engram/internal/config"
)

type overlayKind int

const (
	overlayHelp overlayKind = iota
	overlayPalette
	overlayTheme
	overlayFilters
)

// overlay is a panel drawn over the live view. Only the topmost one gets
// key presses, and Esc closes it.
type overlay struct {
	kind   overlayKind
	title  string
	input  textinput.Model
	cursor int
	// palette: every entry, and the ones matching the input, in order.
	entries []paletteEntry
	matches fuzzy.Matches
	// theme picker: the theme to restore on Esc.
	origTheme string
	// filters: the rows, and whether a filter changed.
	filters []filterRow
	changed bool
}

type paletteEntry struct {
	title, key string
	run        func(*consoleModel) tea.Cmd
}

type filterRowKind int

const (
	filterArchived filterRowKind = iota
	filterSuperseded
	filterScheduled
	filterCrossSpine
	filterScope
	filterTag
)

type filterRow struct {
	kind  filterRowKind
	value string
	label string
}

func newOverlayInput(placeholder string, st consoleStyles) textinput.Model {
	in := textinput.New()
	in.Prompt = "› "
	in.Placeholder = placeholder
	// A static virtual cursor: no blink timer.
	s := st.inputStyles(in.Styles())
	s.Cursor.Blink = false
	in.SetStyles(s)
	in.Focus()
	return in
}

func (m *consoleModel) openOverlay(o *overlay) {
	m.overlays = append(m.overlays, o)
}

func (m *consoleModel) closeOverlay() {
	m.overlays = m.overlays[:len(m.overlays)-1]
}

func newHelpOverlay() *overlay {
	return &overlay{kind: overlayHelp, title: "Keys"}
}

// newPaletteOverlay lists every key-map entry with its key, so running an
// entry is exactly pressing that key, then the actions that have no key.
func (m *consoleModel) newPaletteOverlay() *overlay {
	o := &overlay{kind: overlayPalette, title: "Commands", input: newOverlayInput("type to filter", m.st)}
	for _, b := range m.keys.all() {
		press := keyPress(b.Keys()[0])
		h := b.Help()
		o.entries = append(o.entries, paletteEntry{title: h.Desc, key: h.Key,
			run: func(m *consoleModel) tea.Cmd { return m.handleKey(press) }})
	}
	toggle := func(name string, flag *bool) paletteEntry {
		return paletteEntry{title: "Include " + name + ": turn " + onOff(!*flag), run: func(m *consoleModel) tea.Cmd {
			*flag = !*flag
			m.reloadFiltered()
			return m.openTab(m.tab)
		}}
	}
	o.entries = append(o.entries,
		toggle("archived records", &m.includeArchived),
		toggle("superseded records", &m.includeSuperseded),
		toggle("scheduled and expired records", &m.includeScheduled),
		paletteEntry{title: "Scope: every readable scope", run: func(m *consoleModel) tea.Cmd {
			return m.setScope(true, "")
		}},
		paletteEntry{title: "Mouse: turn " + onOff(!m.mouse), run: func(m *consoleModel) tea.Cmd {
			return m.setMouse(!m.mouse)
		}},
	)
	for _, sc := range m.scopes {
		name := sc.GetScope()
		o.entries = append(o.entries, paletteEntry{title: "Scope: " + consoleText(name, false),
			run: func(m *consoleModel) tea.Cmd { return m.setScope(false, name) }})
	}
	if len(m.chosenTags) > 0 {
		o.entries = append(o.entries, paletteEntry{title: "Clear the chosen tags", run: func(m *consoleModel) tea.Cmd {
			m.chosenTags = nil
			return m.applyTagFilter()
		}})
	}
	o.refilterPalette()
	return o
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func (o *overlay) refilterPalette() {
	titles := make([]string, len(o.entries))
	for i, e := range o.entries {
		titles[i] = e.title
	}
	if q := o.input.Value(); q != "" {
		o.matches = fuzzy.Find(q, titles)
	} else {
		o.matches = make(fuzzy.Matches, len(titles))
		for i, t := range titles {
			o.matches[i] = fuzzy.Match{Str: t, Index: i}
		}
	}
	o.cursor = 0
}

func (m *consoleModel) newThemeOverlay() *overlay {
	o := &overlay{kind: overlayTheme, title: "Theme", origTheme: m.themeName}
	o.cursor = max(slices.Index(config.TUIThemes, m.themeName), 0)
	return o
}

func (m *consoleModel) newFiltersOverlay() *overlay {
	o := &overlay{kind: overlayFilters, title: "Scope and filters", input: newOverlayInput("filter scopes", m.st)}
	m.refilterFilters(o)
	return o
}

// refilterFilters rebuilds the filter rows: the include options, the
// cross-spine choice, the scopes matching the input, and chosen tags.
func (m *consoleModel) refilterFilters(o *overlay) {
	o.filters = []filterRow{
		{kind: filterArchived, label: "include archived records"},
		{kind: filterSuperseded, label: "include superseded records"},
		{kind: filterScheduled, label: "include scheduled and expired records"},
		{kind: filterCrossSpine, label: "every readable scope (cross-spine)"},
	}
	names := make([]string, len(m.scopes))
	for i, sc := range m.scopes {
		names[i] = sc.GetScope()
	}
	approx := ""
	if m.scopesApproximate {
		approx = "~"
	}
	add := func(i int) {
		sc := m.scopes[i]
		o.filters = append(o.filters, filterRow{kind: filterScope, value: sc.GetScope(),
			label: fmt.Sprintf("%s  %s%d", consoleText(sc.GetScope(), false), approx, sc.GetCount())})
	}
	if q := o.input.Value(); q != "" {
		for _, mt := range fuzzy.FindNoSort(q, names) {
			add(mt.Index)
		}
	} else {
		for i := range m.scopes {
			add(i)
		}
	}
	for _, t := range m.chosenTags {
		o.filters = append(o.filters, filterRow{kind: filterTag, value: t, label: "#" + consoleText(t, false)})
	}
	o.cursor = min(o.cursor, max(len(o.filters)-1, 0))
}

func (m *consoleModel) setScope(cross bool, scope string) tea.Cmd {
	m.crossSpine, m.scope = cross, scope
	m.reloadFiltered()
	return m.openTab(m.tab)
}

// setMouse turns mouse support on or off and saves the choice.
func (m *consoleModel) setMouse(on bool) tea.Cmd {
	m.mouse = on
	m.settings.prefs.Mouse = &on
	return tea.Batch(m.savePrefs(), m.showToast("mouse "+onOff(on)))
}

func (m *consoleModel) savePrefs() tea.Cmd {
	path, prefs := m.settings.prefsPath, m.settings.prefs
	if path == "" {
		return nil
	}
	return func() tea.Msg { return prefsSavedMsg{err: saveConsolePrefs(path, prefs)} }
}

func (m *consoleModel) overlayKey(msg tea.KeyPressMsg) tea.Cmd {
	o := m.overlays[len(m.overlays)-1]
	k := m.keys
	switch o.kind {
	case overlayHelp:
		if key.Matches(msg, k.Back, k.Help, k.Quit) {
			m.closeOverlay()
		}
	case overlayPalette:
		switch msg.String() {
		case "esc":
			m.closeOverlay()
		case "up", "ctrl+p":
			o.cursor = max(o.cursor-1, 0)
		case "down", "ctrl+n":
			o.cursor = min(o.cursor+1, max(len(o.matches)-1, 0))
		case "enter":
			m.closeOverlay()
			if len(o.matches) > 0 {
				return o.entries[o.matches[o.cursor].Index].run(m)
			}
		default:
			var cmd tea.Cmd
			o.input, cmd = o.input.Update(msg)
			o.refilterPalette()
			return cmd
		}
	case overlayTheme:
		switch {
		case key.Matches(msg, k.Back):
			m.closeOverlay()
			m.setTheme(o.origTheme)
			return m.ensureRendered()
		case key.Matches(msg, k.Up), key.Matches(msg, k.Down):
			if key.Matches(msg, k.Up) {
				o.cursor = max(o.cursor-1, 0)
			} else {
				o.cursor = min(o.cursor+1, len(config.TUIThemes)-1)
			}
			m.setTheme(config.TUIThemes[o.cursor])
			return m.ensureRendered()
		case key.Matches(msg, k.Open):
			m.closeOverlay()
			m.settings.prefs.Theme = m.themeName
			note := "theme " + m.themeName + " saved"
			if m.settings.themeLocked {
				note += "; --theme or ENGRAM_TUI_THEME still wins at the next start"
			}
			return tea.Batch(m.savePrefs(), m.showToast(note))
		}
	case overlayFilters:
		switch msg.String() {
		case "esc":
			m.closeOverlay()
			if o.changed {
				m.reloadFiltered()
				return m.openTab(m.tab)
			}
		case "up":
			o.cursor = max(o.cursor-1, 0)
		case "down":
			o.cursor = min(o.cursor+1, max(len(o.filters)-1, 0))
		case "space":
			m.chooseFilter(o)
		case "enter":
			m.chooseFilter(o)
			m.closeOverlay()
			if o.changed {
				m.reloadFiltered()
				return m.openTab(m.tab)
			}
		default:
			var cmd tea.Cmd
			o.input, cmd = o.input.Update(msg)
			m.refilterFilters(o)
			return cmd
		}
	}
	return nil
}

func (m *consoleModel) chooseFilter(o *overlay) {
	if len(o.filters) == 0 {
		return
	}
	o.changed = true
	row := o.filters[o.cursor]
	switch row.kind {
	case filterArchived:
		m.includeArchived = !m.includeArchived
	case filterSuperseded:
		m.includeSuperseded = !m.includeSuperseded
	case filterScheduled:
		m.includeScheduled = !m.includeScheduled
	case filterCrossSpine:
		m.crossSpine, m.scope = true, ""
	case filterScope:
		m.crossSpine, m.scope = false, row.value
	case filterTag:
		m.toggleTag(row.value)
		m.refilterFilters(o)
	}
}

// renderOverlay draws an overlay's box.
func (m *consoleModel) renderOverlay(o *overlay) string {
	st := m.st
	w := min(max(m.width*2/3, 40), m.width-4, 96)
	h := max(min(m.height-6, 24), 6)
	var body []string
	line := func(selected bool, text string) string {
		if selected {
			return st.bar.Render("▌") + " " + st.bold.Render(clip(text, w-6))
		}
		return "  " + st.text.Render(clip(text, w-6))
	}
	switch o.kind {
	case overlayHelp:
		groups := m.keys.groups()
		cols := make([]string, 0, len(groups))
		for _, g := range groups {
			lines := []string{st.accent.Bold(true).Render(g.title)}
			for _, b := range g.bindings {
				lines = append(lines, st.key.Render(fmt.Sprintf("%-7s", b.Help().Key))+" "+st.muted.Render(b.Help().Desc))
			}
			cols = append(cols, strings.Join(lines, "\n")+"\n")
		}
		half := (len(cols) + 1) / 2
		left := lipgloss.JoinVertical(lipgloss.Left, cols[:half]...)
		right := lipgloss.JoinVertical(lipgloss.Left, cols[half:]...)
		body = strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, left, "   ", right), "\n")
		body = append(body, st.faint.Render("esc closes"))
	case overlayPalette:
		body = append(body, o.input.View(), "")
		top := max(o.cursor-(h-3), 0)
		for i := top; i < len(o.matches) && len(body) < h; i++ {
			mt := o.matches[i]
			e := o.entries[mt.Index]
			title := underlineMatches(e.title, mt.MatchedIndexes)
			text := title
			if e.key != "" {
				text = joinRight(title, st.key.Render(e.key), w-6)
			}
			if i == o.cursor {
				body = append(body, st.bar.Render("▌")+" "+text)
			} else {
				body = append(body, "  "+text)
			}
		}
		if len(o.matches) == 0 {
			body = append(body, st.faint.Render("  no command matches"))
		}
	case overlayTheme:
		for i, name := range config.TUIThemes {
			label := name
			if name == o.origTheme {
				label += " (started with)"
			}
			body = append(body, line(i == o.cursor, label))
		}
		body = append(body, "", st.faint.Render("↑/↓ preview · enter keeps and saves · esc reverts"))
	case overlayFilters:
		body = append(body, o.input.View(), "")
		top := max(o.cursor-(h-3), 0)
		for i := top; i < len(o.filters) && len(body) < h; i++ {
			row := o.filters[i]
			body = append(body, line(i == o.cursor, m.filterMark(row)+" "+row.label))
		}
		body = append(body, "", st.faint.Render("space toggles · enter chooses and closes · esc closes"))
	}
	title := st.accent.Bold(true).Render(o.title) + " " + st.divider.Render(strings.Repeat("─", max(w-6-ansi.StringWidth(o.title), 0)))
	return st.overlay.Width(w).Render(title + "\n" + strings.Join(body, "\n"))
}

func (m *consoleModel) filterMark(row filterRow) string {
	check := func(on bool) string {
		if on {
			return "[x]"
		}
		return "[ ]"
	}
	radio := func(on bool) string {
		if on {
			return "(•)"
		}
		return "( )"
	}
	switch row.kind {
	case filterArchived:
		return check(m.includeArchived)
	case filterSuperseded:
		return check(m.includeSuperseded)
	case filterScheduled:
		return check(m.includeScheduled)
	case filterCrossSpine:
		return radio(m.crossSpine)
	case filterScope:
		return radio(!m.crossSpine && m.scope == row.value)
	}
	return check(slices.Contains(m.chosenTags, row.value))
}

// underlineMatches underlines the characters a fuzzy match hit.
func underlineMatches(s string, idx []int) string {
	if len(idx) == 0 {
		return s
	}
	u := lipgloss.NewStyle().Underline(true)
	var b strings.Builder
	for i, r := range []byte(s) {
		if slices.Contains(idx, i) {
			b.WriteString(u.Render(string(r)))
		} else {
			b.WriteByte(r)
		}
	}
	return b.String()
}

// handleClick selects the row under the pointer, or focuses the preview.
func (m *consoleModel) handleClick(msg tea.MouseClickMsg) tea.Cmd {
	if !m.mouse || len(m.overlays) > 0 || msg.Button != tea.MouseLeft {
		return nil
	}
	l := m.layout()
	s := m.active()
	switch {
	case l.showList && l.list.contains(msg.X, msg.Y):
		items := m.listItems(s)
		line := s.top + msg.Y - l.list.y
		if line < 0 || line >= len(items) || items[line].pos < 0 {
			return nil
		}
		m.focus = focusList
		before := s.selectedID
		s.setCursor(items[line].pos)
		if s.selectedID != before {
			return m.selectionChanged()
		}
	case l.showPreview && l.preview.contains(msg.X, msg.Y) && s.kind != screenDetail:
		m.focus = focusPreview
	}
	return nil
}

// handleWheel scrolls the pane under the pointer.
func (m *consoleModel) handleWheel(msg tea.MouseWheelMsg) tea.Cmd {
	if !m.mouse || len(m.overlays) > 0 {
		return nil
	}
	delta := 3
	if msg.Button == tea.MouseWheelUp {
		delta = -3
	} else if msg.Button != tea.MouseWheelDown {
		return nil
	}
	l := m.layout()
	if l.showPreview && l.preview.contains(msg.X, msg.Y) {
		m.scrollPreview(delta)
		return nil
	}
	if l.showList && l.list.contains(msg.X, msg.Y) {
		return m.move(m.active(), delta)
	}
	return nil
}
