// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

type screenKind int

const (
	screenMemories screenKind = iota
	screenTags
	screenRules
	screenScheduled
	screenDetail
	screenRelated
)

// screen is one view's state. Each tab keeps one; detail and related
// screens stack above the active tab, and Esc pops them. The selection is
// held by id as well as position, so a reload or resize keeps it.
type screen struct {
	kind screenKind
	// seq is the request this screen waits on. A result that carries any
	// other seq is stale and is dropped, so a slow response cannot
	// overwrite a newer one.
	seq                      uint64
	loaded, loading, pageErr bool
	cursor, top              int
	selectedID               string

	rows     []*engramv1.Memory
	lanes    []engramv1.EdgeType // related: each row's lane
	evidence []string            // related: each row's evidence
	tags     []*engramv1.TagCount

	query     string // memories: the search query; "" lists
	searched  bool   // memories: the rows are search hits
	nextToken string

	crossSpine                     bool
	searchedScopes                 []string
	scopesTruncated, scopesUnknown bool
	hidden                         *engramv1.RecallGateHidden
	advisory                       string
	moreTags, relatedTruncated     bool

	narrow  string
	visible []int // the rows that match narrow; nil when not narrowed

	id      string           // detail and related: the record asked for
	anchor  *engramv1.Memory // detail and related: the record as last seen
	expired bool             // scheduled: lists expired records instead
}

func (s *screen) itemCount() int {
	if s.kind == screenTags {
		return len(s.tags)
	}
	return len(s.rows)
}

// count is the number of rows in view, after narrowing.
func (s *screen) count() int {
	if s.visible != nil {
		return len(s.visible)
	}
	return s.itemCount()
}

// index maps a position in view to an index into rows or tags.
func (s *screen) index(pos int) int {
	if s.visible != nil {
		return s.visible[pos]
	}
	return pos
}

func (s *screen) itemID(idx int) string {
	switch s.kind {
	case screenTags:
		return "#" + s.tags[idx].GetTag()
	case screenRelated:
		return fmt.Sprintf("%s|%d", s.rows[idx].GetId(), s.lanes[idx])
	}
	return s.rows[idx].GetId()
}

func (s *screen) setCursor(pos int) {
	n := s.count()
	if n == 0 {
		s.cursor, s.selectedID = 0, ""
		return
	}
	s.cursor = max(min(pos, n-1), 0)
	s.selectedID = s.itemID(s.index(s.cursor))
}

// refilter recomputes the narrowed rows and puts the selection back on the
// same item when it is still in view.
func (s *screen) refilter() {
	s.visible = nil
	if s.narrow != "" {
		matches := fuzzy.FindNoSort(s.narrow, s.narrowSource())
		s.visible = make([]int, 0, len(matches))
		for _, mt := range matches {
			s.visible = append(s.visible, mt.Index)
		}
	}
	for pos := range s.count() {
		if s.itemID(s.index(pos)) == s.selectedID {
			s.setCursor(pos)
			return
		}
	}
	s.setCursor(0)
}

// narrowSource is the text local narrowing matches for each row.
func (s *screen) narrowSource() []string {
	if s.kind == screenTags {
		out := make([]string, len(s.tags))
		for i, t := range s.tags {
			out[i] = t.GetTag()
		}
		return out
	}
	out := make([]string, len(s.rows))
	for i, r := range s.rows {
		out[i] = strings.Join([]string{r.GetSummary(), r.GetCategory(), strings.Join(r.GetTags(), " "), r.GetShortId()}, " ")
	}
	return out
}

func (s *screen) selectedRow() *engramv1.Memory {
	if s.kind == screenTags || s.kind == screenDetail || s.count() == 0 {
		return nil
	}
	return s.rows[s.index(s.cursor)]
}

func (s *screen) selectedTag() string {
	if s.kind != screenTags || s.count() == 0 {
		return ""
	}
	return s.tags[s.index(s.cursor)].GetTag()
}

func (s *screen) coverage(cross bool, searched []string, truncated, unknown bool) {
	s.crossSpine, s.searchedScopes, s.scopesTruncated, s.scopesUnknown = cross, searched, truncated, unknown
}

func (m *consoleModel) setNarrow(q string) {
	s := m.active()
	s.narrow = q
	s.refilter()
}

// selectedRecord is the record the preview, copy and related keys act on.
func (m *consoleModel) selectedRecord() *engramv1.Memory {
	s := m.active()
	if s.kind == screenDetail {
		if r, ok := m.preview.records.get(s.id); ok && r != nil {
			return r
		}
		return s.anchor
	}
	return s.selectedRow()
}

// compactAge is a duration in the units rows use: now, 5m, 3h, 2d, 4mo, 1y.
func compactAge(d time.Duration) string {
	day := 24 * time.Hour
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	case d < day:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	case d < 30*day:
		return fmt.Sprintf("%dd", int(d/day))
	case d < 365*day:
		return fmt.Sprintf("%dmo", int(d/(30*day)))
	}
	return fmt.Sprintf("%dy", int(d/(365*day)))
}

// agoWords is a past age in words: "just now" or "3h ago".
func agoWords(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	return compactAge(d) + " ago"
}

// windowWords states a scheduled record's window: when it reveals, when it
// expired, or when it expires.
func windowWords(r *engramv1.Memory, now time.Time) string {
	if na := r.GetNotAfter(); na != nil && !na.AsTime().After(now) {
		return "expired " + compactAge(now.Sub(na.AsTime())) + " ago"
	}
	if nb := r.GetNotBefore(); nb != nil && nb.AsTime().After(now) {
		return "reveals in " + compactAge(nb.AsTime().Sub(now))
	}
	if na := r.GetNotAfter(); na != nil {
		return "expires in " + compactAge(na.AsTime().Sub(now))
	}
	return ""
}

// clip cuts plain text to w cells with an ellipsis and pads it to w.
func clip(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(w-ansi.StringWidth(s), 0))
}

// tagsText fits as many tags as budget allows, then a +N for the rest.
func tagsText(tags []string, budget int) string {
	var b strings.Builder
	for i, t := range tags {
		word := "#" + consoleText(t, false)
		rest := len(tags) - i - 1
		suffix := ""
		if rest > 0 {
			suffix = fmt.Sprintf(" +%d", rest)
		}
		sep := ""
		if b.Len() > 0 {
			sep = " "
		}
		if ansi.StringWidth(b.String()+sep+word+suffix) > budget {
			if b.Len() == 0 {
				return fmt.Sprintf("+%d", len(tags))
			}
			return b.String() + fmt.Sprintf(" +%d", len(tags)-i)
		}
		b.WriteString(sep + word)
	}
	return b.String()
}

// rowColumns are the widths one render of a list gives each column.
type rowColumns struct {
	state, tags, scope, right int
}

// columns sizes the row columns for width w from the rows in view: tags
// drop out first, then scope, as width runs short.
func (m *consoleModel) columns(s *screen, w int) rowColumns {
	now := m.now()
	var c rowColumns
	scopes := map[string]bool{}
	longestScope := 0
	for pos := range s.count() {
		r := s.rows[s.index(pos)]
		c.state = max(c.state, ansi.StringWidth(strings.Join(memoryStateWords(r, now), " ")))
		scopes[r.GetScope()] = true
		longestScope = max(longestScope, ansi.StringWidth(r.GetScope()))
		if s.kind == screenScheduled {
			c.right = max(c.right, ansi.StringWidth(windowWords(r, now)))
		}
		if s.kind == screenRelated {
			c.right = max(c.right, ansi.StringWidth(s.evidence[s.index(pos)]))
		}
	}
	if s.kind != screenScheduled && s.kind != screenRelated {
		c.right = 4
	}
	c.right = min(c.right, 24)
	const fixed = 2 + 2 + 11 // selection bar, marker, category word
	summary := func() int {
		n := w - fixed - c.right - 1
		for _, col := range []int{c.state, c.tags, c.scope} {
			if col > 0 {
				n -= col + 1
			}
		}
		return n
	}
	if w >= 90 {
		c.tags = 28
	}
	if len(scopes) > 1 && w >= 110 {
		c.scope = min(longestScope, 28)
	}
	if summary() < 24 {
		c.tags = 0
	}
	if summary() < 24 {
		c.scope = 0
	}
	return c
}

// rowLine renders one record row at width w.
func (m *consoleModel) rowLine(s *screen, idx int, w int, c rowColumns, selected bool) string {
	r := s.rows[idx]
	now := m.now()
	st := m.st
	words := memoryStateWords(r, now)
	past := slices.ContainsFunc(words, func(w string) bool { return w != "scheduled" })
	text, cat := st.text, st.category(r.GetCategory())
	if past {
		text, cat = st.muted, st.muted
	}
	bar := " "
	if selected {
		bar = st.barBlurred.Render("▌")
		if m.focus == focusList {
			bar = st.bar.Render("▌")
		}
	}
	summaryText := r.GetSummary()
	if summaryText == "" {
		summaryText = r.GetContent()
	}
	summaryText = consoleText(strings.SplitN(summaryText, "\n", 2)[0], false)
	summaryStyle := text
	if selected && !past {
		summaryStyle = st.bold
	}

	used := 2 + 2 + 11 + c.right + 1
	parts := make([]string, 0, 8)
	parts = append(parts,
		bar+" ",
		cat.Render("●")+" ",
		cat.Render(clip(consoleText(r.GetCategory(), false), 10))+" ",
	)
	tail := []string{}
	if c.state > 0 {
		used += c.state + 1
		stateStyle := st.muted
		switch {
		case slices.Contains(words, "expired"):
			stateStyle = st.warning
		case len(words) == 1 && words[0] == "scheduled":
			stateStyle = st.accent
		}
		tail = append(tail, stateStyle.Render(clip(strings.Join(words, " "), c.state)))
	}
	if c.tags > 0 {
		used += c.tags + 1
		tail = append(tail, st.muted.Render(clip(tagsText(r.GetTags(), c.tags), c.tags)))
	}
	if c.scope > 0 {
		used += c.scope + 1
		tail = append(tail, st.faint.Render(clip(consoleText(r.GetScope(), false), c.scope)))
	}
	right := compactAge(now.Sub(r.GetCreatedAt().AsTime()))
	switch s.kind {
	case screenScheduled:
		right = windowWords(r, now)
	case screenRelated:
		right = s.evidence[idx]
	}
	if r.GetVisibility() == "shared" && w-used >= 40 {
		shared := "shared · " + consoleText(r.GetOwner(), false)
		tail = append([]string{st.accent.Render(clip(shared, min(ansi.StringWidth(shared), 32)))}, tail...)
		used += min(ansi.StringWidth(shared), 32) + 1
	}
	parts = append(parts, summaryStyle.Render(clip(summaryText, max(w-used, 1))))
	for _, t := range tail {
		parts = append(parts, " "+t)
	}
	parts = append(parts, " "+st.faint.Render(fmt.Sprintf("%*s", c.right, ansi.Truncate(right, c.right, "…"))))
	return strings.Join(parts, "")
}
