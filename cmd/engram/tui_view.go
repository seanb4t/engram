// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

const consoleNotFound = "not found: no record you can read has that id"

// consoleText makes server-supplied text safe to draw. Every C0 and C1
// control rune and DEL becomes a space, so no stored record (another
// actor's shared one included) can send the terminal an escape sequence.
// keepNewlines keeps line breaks for the detail view's content body. It
// runs on raw text, before any styling adds the console's own sequences.
func consoleText(s string, keepNewlines bool) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' && keepNewlines:
			return r
		case r < 0x20, r >= 0x7f && r <= 0x9f:
			return ' '
		}
		return r
	}, s)
}

func (m *consoleModel) View() tea.View {
	lines := make([]string, 0, m.height)
	lines = append(lines, m.tabLine(), m.inputLine(), m.filterLine(), m.resultsHeader())
	lines = append(lines, m.bodyLines()...)
	lines = append(lines, m.statusLine(), m.footerLine())
	for i, l := range lines {
		lines[i] = fitLine(l, m.width)
	}
	content := strings.Join(lines, "\n")
	if len(m.overlays) > 0 {
		content = m.composeOverlay(content)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	v.WindowTitle = "engram"
	if m.mouse {
		v.MouseMode = tea.MouseModeCellMotion
	}
	if len(m.overlays) == 0 {
		c := m.input.Cursor()
		if c == nil {
			c = m.narrowIn.Cursor()
		}
		if c != nil {
			c.Y = 1
			v.Cursor = c
		}
	}
	return v
}

// fitLine truncates a styled line to w cells and pads it to w.
func fitLine(s string, w int) string {
	s = ansi.Truncate(s, w, "")
	return s + strings.Repeat(" ", max(w-ansi.StringWidth(s), 0))
}

// joinRight puts right at the right edge of a w-wide line after left.
func joinRight(left, right string, w int) string {
	gap := w - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 1 {
		return left
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m *consoleModel) where() string {
	if m.crossSpine {
		return "every readable scope"
	}
	return consoleText(m.scope, false)
}

func (m *consoleModel) tabLine() string {
	st := m.st
	var b strings.Builder
	b.WriteString(st.accent.Bold(true).Render(" engram "))
	for i, name := range consoleTabNames {
		label := fmt.Sprintf("%d %s", i+1, name)
		if consoleTab(i) == m.tab {
			b.WriteString(st.tabActive.Render(label))
		} else {
			b.WriteString(st.tabInactive.Render(label))
		}
	}
	right := m.where()
	if m.crossSpine {
		right += fmt.Sprintf(" (%d)", len(m.scopes))
	}
	return joinRight(b.String(), st.faint.Render(right+" "), m.width)
}

// searchHint states what Enter will do with the text in the search box.
func searchHint(q string) string {
	q = strings.TrimSpace(q)
	switch {
	case q == "":
		return "enter: list"
	case isIDShaped(q):
		return "enter: open"
	}
	return "enter: search · 1 embedding"
}

func (m *consoleModel) inputLine() string {
	st := m.st
	switch {
	case m.narrowIn.Focused():
		return m.narrowIn.View()
	case m.tab == tabMemories && len(m.stack) == 0:
		hint := ""
		if m.input.Focused() {
			hint = st.faint.Render(searchHint(m.input.Value()) + " ")
		}
		return joinRight(m.input.View(), hint, m.width)
	}
	trail := []string{consoleTabNames[m.tab]}
	for i := range m.stack {
		trail = append(trail, m.screenTitle(&m.stack[i]))
	}
	return st.faint.Render(" " + strings.Join(trail, " › "))
}

func (m *consoleModel) screenTitle(s *screen) string {
	id := s.id
	if s.anchor != nil && s.anchor.GetShortId() != "" {
		id = s.anchor.GetShortId()
	}
	id = consoleText(id, false)
	if s.kind == screenRelated {
		return "related to " + id
	}
	return id
}

func (m *consoleModel) filterLine() string {
	st := m.st
	parts := []string{"scope: " + m.where()}
	var include []string
	if m.includeArchived {
		include = append(include, "archived")
	}
	if m.includeSuperseded {
		include = append(include, "superseded")
	}
	if m.includeScheduled {
		include = append(include, "scheduled and expired")
	}
	if len(include) > 0 {
		parts = append(parts, "include: "+strings.Join(include, ", "))
	}
	if len(m.chosenTags) > 0 {
		tags := make([]string, 0, len(m.chosenTags))
		for _, t := range m.chosenTags {
			tags = append(tags, "#"+consoleText(t, false))
		}
		parts = append(parts, "all of "+strings.Join(tags, " "))
	}
	left := st.muted.Render(" " + strings.Join(parts, " · "))
	right := st.faint.Render("f filters · = narrow · ctrl+k commands ")
	return joinRight(left, right, m.width)
}

// scopesPhrase is how many scopes a response covered, as the web
// console's results header words it.
func scopesPhrase(s *screen) string {
	switch {
	case !s.crossSpine:
		return "1 scope"
	case s.scopesUnknown:
		return "every readable scope"
	case s.scopesTruncated:
		return fmt.Sprintf("%d scopes (some readable scopes not searched)", len(s.searchedScopes))
	}
	return fmt.Sprintf("%d scopes", len(s.searchedScopes))
}

func hiddenTotal(s *screen) uint64 {
	if s.hidden == nil {
		return 0
	}
	return s.hidden.GetTotal()
}

// resultsHeader states, in words, what the active view searched.
func (m *consoleModel) resultsHeader() string {
	s := m.active()
	st := m.st
	n := s.count()
	hidden := ""
	if h := hiddenTotal(s); h > 0 {
		hidden = fmt.Sprintf(" · %d hidden by the recall gate", h)
	}
	query := fmt.Sprintf("%q", consoleText(s.query, false))
	var text string
	switch s.kind {
	case screenMemories:
		switch {
		case s.loading && s.query != "":
			text = fmt.Sprintf("Searching %s for %s…", m.where(), query)
		case s.loading && !s.loaded:
			text = "Loading memories in " + m.where() + "…"
		case s.narrow != "":
			text = fmt.Sprintf("%d of %d loaded match %q", n, s.itemCount(), consoleText(s.narrow, false))
		case s.searched && n == 0:
			text = fmt.Sprintf("No memories match %s in %s%s", query, m.where(), hidden)
		case s.searched:
			ranking := "ranked by cosine similarity"
			if slices.ContainsFunc(s.rows, func(r *engramv1.Memory) bool { return r.Relevance != nil }) {
				ranking = "reranked by jev"
			}
			text = fmt.Sprintf("%d hits across %s for %s · %s%s", n, scopesPhrase(s), query, ranking, hidden)
		case n == 0 && s.loaded:
			text = "No memories in " + m.where() + hidden
		default:
			text = fmt.Sprintf("Latest %d memories across %s · unranked%s", n, scopesPhrase(s), hidden)
		}
	case screenTags:
		text = fmt.Sprintf("%d tags in %s, most used first", n, m.where())
		if s.moreTags {
			text += " · more exist than shown"
		}
	case screenRules:
		text = fmt.Sprintf("%d rules across %s", n, strings.Replace(scopesPhrase(s), "scope", "rule scope", 1))
	case screenScheduled:
		state, other := "scheduled", "expired"
		if s.expired {
			state, other = other, state
		}
		text = fmt.Sprintf("%d %s records of yours in %s · w shows %s", n, state, m.where(), other)
	case screenRelated:
		text = fmt.Sprintf("%d %s", n, m.screenTitle(s))
		if s.relatedTruncated {
			text += " · more vector neighbours exist than shown"
		}
	case screenDetail:
		text = "memory " + m.screenTitle(s)
	}
	return st.bold.Render(" " + text)
}

// emptyLines say what was searched and the keys that would widen it.
func (m *consoleModel) emptyLines(s *screen) []string {
	st := m.st
	if s.kind != screenMemories {
		return []string{st.faint.Render("  nothing here")}
	}
	out := []string{"", st.muted.Render("  Nothing matched. To widen it:")}
	if !m.crossSpine {
		out = append(out, "  "+st.key.Render("f")+st.muted.Render("  search every readable scope"))
	}
	if !m.includeArchived {
		hint := "  include archived records"
		if s.hidden != nil && s.hidden.GetArchived() > 0 {
			hint += fmt.Sprintf(" (%d hidden)", s.hidden.GetArchived())
		}
		out = append(out, "  "+st.key.Render("f")+st.muted.Render(hint))
	}
	if len(m.chosenTags) > 0 {
		out = append(out, "  "+st.key.Render("f")+st.muted.Render("  clear the chosen tags"))
	}
	return out
}

func (m *consoleModel) bodyLines() []string {
	l := m.layout()
	s := m.active()
	h := m.bodyHeight()
	listLines := func(w, h int) []string {
		var out []string
		switch {
		case !s.loaded && s.loading:
			out = []string{m.st.faint.Render("  loading…")}
		case s.loaded && s.count() == 0:
			out = m.emptyLines(s)
		default:
			out = m.listLines(s, w, h)
		}
		for len(out) < h {
			out = append(out, "")
		}
		return out[:h]
	}
	divider := m.st.divider
	if m.focus == focusPreview {
		divider = m.st.dividerFocus
	}
	switch {
	case l.showList && l.showPreview && l.tier == tierSide:
		left, right := listLines(l.list.w, h), m.previewLines(l.preview.w-1, h)
		out := make([]string, h)
		for i := range h {
			out[i] = fitLine(left[i], l.list.w) + divider.Render("│") + " " + right[i]
		}
		return out
	case l.showList && l.showPreview:
		out := listLines(m.width, l.list.h)
		out = append(out, divider.Render(strings.Repeat("─", m.width)))
		return append(out, m.previewLines(m.width, l.preview.h)...)
	case l.showList:
		return listLines(m.width, h)
	}
	return m.previewLines(m.width, h)
}

// positionText is where the selection sits among the loaded rows.
func (m *consoleModel) positionText(s *screen) string {
	if s.kind == screenDetail {
		lines := len(m.previewContent(m.previewWidth()))
		if lines <= m.previewHeight() {
			return "all shown"
		}
		return fmt.Sprintf("%d%%", min(100, (m.preview.offset+m.previewHeight())*100/lines))
	}
	n := s.count()
	if n == 0 {
		return "0/0"
	}
	pos := s.cursor + 1
	switch {
	case s.nextToken != "" && s.narrow == "":
		return fmt.Sprintf("%d/%d+", pos, n)
	case s.kind == screenMemories && s.searched && s.itemCount() >= consolePageSize:
		return fmt.Sprintf("%d/%d · top %d", pos, n, consolePageSize)
	}
	return fmt.Sprintf("%d/%d", pos, n)
}

func (m *consoleModel) statusLine() string {
	st := m.st
	s := m.active()
	if m.statusErr {
		return st.errorFg.Render(" " + m.status)
	}
	left := " " + m.positionText(s)
	if s.hidden != nil && s.hidden.GetTotal() > 0 {
		var by []string
		for _, b := range []struct {
			n    uint64
			name string
		}{
			{s.hidden.GetArchived(), "archived"}, {s.hidden.GetSuperseded(), "superseded"},
			{s.hidden.GetExpired(), "expired"}, {s.hidden.GetScheduled(), "scheduled"},
		} {
			if b.n > 0 {
				by = append(by, fmt.Sprintf("%d %s", b.n, b.name))
			}
		}
		left += fmt.Sprintf(" · %d hidden (%s)", s.hidden.GetTotal(), strings.Join(by, ", "))
	}
	if s.narrow != "" {
		left += fmt.Sprintf(" · narrowed %d of %d loaded", s.count(), s.itemCount())
	}
	if s.advisory != "" {
		left += " · " + consoleText(s.advisory, false)
	}
	right := m.toast
	if right == "" {
		right = m.status
	}
	return joinRight(st.muted.Render(left), st.success.Render(right+" "), m.width)
}

// footerLine lists the focused pane's keys, dropping from the end when
// the line is narrow, and always ends with ? help.
func (m *consoleModel) footerLine() string {
	st := m.st
	render := func(b key.Binding) string {
		h := b.Help()
		return st.key.Render(h.Key) + " " + st.muted.Render(h.Desc)
	}
	focus := m.focus
	if m.active().kind == screenDetail {
		focus = focusPreview
	}
	bindings := m.keys.footer(focus)
	tail := render(m.keys.Help) + st.faint.Render(" · ") + render(m.keys.Quit)
	for len(bindings) > 0 {
		parts := make([]string, 0, len(bindings))
		for _, b := range bindings {
			parts = append(parts, render(b))
		}
		line := " " + strings.Join(parts, st.faint.Render(" · ")) + st.faint.Render(" · ") + tail
		if ansi.StringWidth(line) <= m.width {
			return line
		}
		bindings = bindings[:len(bindings)-1]
	}
	return " " + tail
}

// composeOverlay draws the topmost overlay over the base view.
func (m *consoleModel) composeOverlay(base string) string {
	o := m.overlays[len(m.overlays)-1]
	box := m.renderOverlay(o)
	x := max((m.width-lipgloss.Width(box))/2, 0)
	y := max(min(2, m.height-lipgloss.Height(box)), 0)
	return lipgloss.NewCanvas(m.width, m.height).Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(box).X(x).Y(y).Z(1),
	)).Render()
}
