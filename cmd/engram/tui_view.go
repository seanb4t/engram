// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// Basic ANSI colours only, so the console reads on light and dark
// terminals alike.
var (
	consoleSelected = lipgloss.NewStyle().Reverse(true)
	consolePast     = lipgloss.NewStyle().Faint(true)
	consoleTitle    = lipgloss.NewStyle().Bold(true)
	consoleError    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	consoleFaint    = lipgloss.NewStyle().Faint(true)
)

var edgeTypeWords = map[engramv1.EdgeType]string{
	engramv1.EdgeType_EDGE_TYPE_SUPERSESSION: "supersession",
	engramv1.EdgeType_EDGE_TYPE_TAG:          "shared tag",
	engramv1.EdgeType_EDGE_TYPE_CITATION:     "shared citation",
	engramv1.EdgeType_EDGE_TYPE_VECTOR:       "vector neighbour",
}

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
	s := m.active()
	top := []string{m.headerLine(), m.inputLine(), consoleTitle.Render(m.title(s))}
	bottom := []string{}
	if s.footer != "" {
		bottom = append(bottom, strings.Split(strings.TrimSuffix(s.footer, "\n"), "\n")...)
	}
	bottom = append(bottom, m.statusLine(), m.help.View(m.keys))

	h := m.listHeight()
	var body []string
	switch {
	case m.filtersOpen:
		body = m.filterLines(h)
	case s.kind == screenDetail:
		if s.notFound {
			body = []string{consoleNotFound}
		} else if s.record != nil {
			body = strings.Split(s.view.View(), "\n")
		}
	default:
		body = m.rowLines(s, h)
	}
	for len(body) < h {
		body = append(body, "")
	}

	lines := slices.Concat(top, body[:min(len(body), h)], bottom)
	clip := lipgloss.NewStyle().MaxWidth(m.width)
	for i, l := range lines {
		lines[i] = clip.Render(l)
	}
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	v.WindowTitle = "engram"
	if c := m.input.Cursor(); c != nil {
		c.Y = 1
		v.Cursor = c
	}
	return v
}

// listHeight is the number of lines left for rows once the header, input,
// title, footer, status and help lines are drawn.
func (m *consoleModel) listHeight() int {
	chrome := 4 + lipgloss.Height(m.help.View(m.keys)) + strings.Count(m.active().footer, "\n")
	return max(m.height-chrome, 1)
}

func (m *consoleModel) where() string {
	if m.crossSpine {
		return "every readable scope"
	}
	return consoleText(m.scope, false)
}

func (m *consoleModel) headerLine() string {
	var b strings.Builder
	b.WriteString("engram ")
	for i, name := range consoleTabNames {
		label := fmt.Sprintf(" %d %s ", i+1, name)
		if consoleTab(i) == m.tab {
			label = consoleSelected.Render(label)
		}
		b.WriteString(label)
	}
	include := []string{}
	if m.includeArchived {
		include = append(include, "archived")
	}
	if m.includeSuperseded {
		include = append(include, "superseded")
	}
	if m.includeScheduled {
		include = append(include, "scheduled and expired")
	}
	fmt.Fprintf(&b, "  scope: %s", m.where())
	if len(include) > 0 {
		fmt.Fprintf(&b, " · include: %s", strings.Join(include, ", "))
	}
	return b.String()
}

func (m *consoleModel) inputLine() string {
	if m.tab == tabMemories && len(m.stack) == 0 {
		return m.input.View()
	}
	trail := []string{consoleTabNames[m.tab]}
	for _, s := range m.stack {
		trail = append(trail, m.title(&s))
	}
	return consoleFaint.Render(strings.Join(trail, " › "))
}

func (m *consoleModel) statusLine() string {
	if m.statusErr {
		return consoleError.Render(m.status)
	}
	if m.active().loading {
		return consoleFaint.Render("loading…")
	}
	return m.status
}

func (m *consoleModel) title(s *screen) string {
	return consoleText(m.rawTitle(s), false)
}

func (m *consoleModel) rawTitle(s *screen) string {
	n := s.rowCount()
	more := ""
	if s.nextToken != "" {
		more = " · n: next page"
	}
	switch s.kind {
	case screenMemories:
		switch {
		case s.query == "" && s.loading && !s.loaded:
			return "Listing memories in " + m.where() + "…"
		case s.query != "" && s.loading:
			return fmt.Sprintf("Searching %s for %q…", m.where(), s.query)
		case !s.searched:
			return fmt.Sprintf("Latest %d memories in %s · unranked%s", n, m.where(), more)
		case n == 0:
			return fmt.Sprintf("No memories match %q in %s", s.query, m.where())
		}
		ranking := "ranked by cosine"
		if slices.ContainsFunc(s.rows, func(r *engramv1.Memory) bool { return r.Relevance != nil }) {
			ranking = "reranked by jev"
		}
		return fmt.Sprintf("%d hits for %q in %s · %s", n, s.query, m.where(), ranking)
	case screenTags:
		return fmt.Sprintf("%d tags in %s, most used first", n, m.where())
	case screenRules:
		return fmt.Sprintf("%d rules in your readable rule scopes", n)
	case screenScheduled:
		state, other := "scheduled", "expired"
		if s.expired {
			state, other = other, state
		}
		return fmt.Sprintf("%d %s records of yours in %s · tab: show %s%s", n, state, m.where(), other, more)
	case screenDetail:
		return "memory " + s.id
	default:
		anchor := s.id
		if s.record != nil && s.record.GetShortId() != "" {
			anchor = s.record.GetShortId()
		}
		return fmt.Sprintf("%d related to %s", n, anchor)
	}
}

// rowLines renders the visible window of s's rows, keeping the cursor in
// view.
func (m *consoleModel) rowLines(s *screen, h int) []string {
	n := s.rowCount()
	s.top = max(min(s.top, s.cursor), s.cursor-h+1, 0)
	end := min(s.top+h, n)
	now := time.Now()
	stateW, edgeW := 0, 0
	for i := s.top; i < end && s.kind != screenTags; i++ {
		stateW = max(stateW, len(strings.Join(memoryStateWords(s.rows[i], now), " ")))
		if s.kind == screenRelated {
			edgeW = max(edgeW, len(edgeWords(s.edges[i])))
		}
	}
	var out []string
	for i := s.top; i < end; i++ {
		var line string
		past := false
		if s.kind == screenTags {
			t := s.tags[i]
			line = fmt.Sprintf("%7d  %s", t.GetCount(), t.GetTag())
		} else {
			r := s.rows[i]
			words := memoryStateWords(r, now)
			past = slices.ContainsFunc(words, func(w string) bool { return w != "scheduled" })
			cols := []string{fmt.Sprintf("%-10s", r.GetShortId())}
			if s.kind == screenRelated {
				cols = append(cols, fmt.Sprintf("%-*s", edgeW, edgeWords(s.edges[i])))
			}
			cols = append(cols, fmt.Sprintf("%-10s", r.GetCategory()), fmt.Sprintf("%-*s", stateW, strings.Join(words, " ")))
			if m.crossSpine || s.kind == screenRules || s.kind == screenRelated {
				cols = append(cols, r.GetScope())
			}
			if r.GetVisibility() == "shared" {
				cols = append(cols, "shared · "+r.GetOwner())
			}
			text := r.GetSummary()
			if text == "" {
				text = r.GetContent()
			}
			cols = append(cols, truncateSummary(text, max(m.width, 20)))
			line = strings.Join(cols, "  ")
		}
		line = consoleText(line, false)
		switch {
		case i == s.cursor:
			line = consoleSelected.Render(line)
		case past:
			line = consolePast.Render(line)
		}
		out = append(out, line)
	}
	if n == 0 && s.loaded {
		out = append(out, consoleFaint.Render("nothing here"))
	}
	return out
}

func edgeWords(types []engramv1.EdgeType) string {
	words := make([]string, 0, len(types))
	for _, t := range types {
		words = append(words, edgeTypeWords[t])
	}
	return strings.Join(words, ", ")
}

func (m *consoleModel) filterLines(h int) []string {
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
	entries := make([]string, 0, 4+len(m.scopes))
	entries = append(entries,
		check(m.includeArchived)+" archived",
		check(m.includeSuperseded)+" superseded",
		check(m.includeScheduled)+" scheduled and expired",
		radio(m.crossSpine)+" every readable scope (cross-spine)",
	)
	approx := ""
	if m.scopesApproximate {
		approx = "~"
	}
	for _, sc := range m.scopes {
		entries = append(entries, fmt.Sprintf("%s %s  %s%d", radio(!m.crossSpine && m.scope == sc.GetScope()), sc.GetScope(), approx, sc.GetCount()))
	}
	for i := range entries {
		entries[i] = consoleText(entries[i], false)
		if i == m.filterCursor {
			entries[i] = consoleSelected.Render(entries[i])
		}
		entries[i] = "  " + entries[i]
	}
	lines := []string{
		consoleFaint.Render("space: toggle · enter: choose and close · esc: close"),
		"Include records the recall gate hides",
	}
	lines = append(lines, entries[:3]...)
	lines = append(lines, "Scope")
	scopes := entries[3:]
	room := max(h-len(lines), 1)
	first := max(min(m.filterCursor-3, len(scopes)-room), 0)
	return append(lines, scopes[first:min(first+room, len(scopes))]...)
}

// renderDetail is the detail view's text: every field the spec names, by
// its proto field name, then the full content.
func (m *consoleModel) renderDetail(s *screen) string {
	r := s.record
	if r == nil {
		return ""
	}
	var b strings.Builder
	field := func(name, value string) {
		if value != "" {
			fmt.Fprintf(&b, "%-15s %s\n", name+":", consoleText(value, false))
		}
	}
	ts := func(name string, t *timestamppb.Timestamp) {
		if t != nil {
			field(name, t.AsTime().UTC().Format(time.RFC3339))
		}
	}
	state := strings.Join(memoryStateWords(r, time.Now()), " ")
	if state == "" {
		state = "live"
	}
	field("short_id", r.GetShortId())
	field("id", r.GetId())
	field("state", state)
	field("superseded_by", r.GetSupersededBy())
	field("supersedes", strings.Join(r.GetSupersedes(), ", "))
	field("scope", r.GetScope())
	field("category", r.GetCategory())
	visibility := r.GetVisibility()
	if visibility == "" {
		// The store keeps a private record's visibility empty.
		visibility = "private"
	}
	field("visibility", visibility)
	field("owner", r.GetOwner())
	field("tags", strings.Join(r.GetTags(), ", "))
	ts("created_at", r.GetCreatedAt())
	ts("not_before", r.GetNotBefore())
	ts("not_after", r.GetNotAfter())
	ts("archived_at", r.GetArchivedAt())
	field("schema_version", strconv.FormatUint(uint64(r.GetSchemaVersion()), 10))
	field("summary", r.GetSummary())
	b.WriteString("\n")
	b.WriteString(consoleText(r.GetContent(), true))
	return b.String()
}
