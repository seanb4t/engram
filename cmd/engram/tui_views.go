// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/charmbracelet/x/ansi"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// relatedLanes are the related view's groups in the web console's order,
// each with the letter that marks it in every theme.
var relatedLanes = []struct {
	edge   engramv1.EdgeType
	letter string
	name   string
}{
	{engramv1.EdgeType_EDGE_TYPE_SUPERSESSION, "S", "supersession"},
	{engramv1.EdgeType_EDGE_TYPE_CITATION, "C", "shared citation"},
	{engramv1.EdgeType_EDGE_TYPE_TAG, "T", "shared tag"},
	{engramv1.EdgeType_EDGE_TYPE_VECTOR, "V", "vector neighbour"},
}

// applyRelated lays the related records out lane by lane. A record joined
// by several edge types appears once in each of their lanes.
func (s *screen) applyRelated(r *engramv1.RelatedMemoriesResponse) {
	if a := r.GetAnchor(); a != nil {
		s.anchor = a
	}
	s.rows, s.lanes, s.evidence = nil, nil, nil
	for _, lane := range relatedLanes {
		for _, rel := range r.GetRelated() {
			for _, e := range rel.GetEdges() {
				if e.GetType() == lane.edge {
					s.rows = append(s.rows, rel.GetMemory())
					s.lanes = append(s.lanes, lane.edge)
					s.evidence = append(s.evidence, edgeEvidence(e))
					break
				}
			}
		}
	}
	s.relatedTruncated = r.GetTruncated()
}

// edgeEvidence is why a record is related, in a few words.
func edgeEvidence(e *engramv1.RelatedEdge) string {
	switch {
	case e.GetSupersession() != nil:
		sup := e.GetSupersession()
		word := "predecessor"
		if sup.GetDirection() == engramv1.SupersessionDirection_SUPERSESSION_DIRECTION_SUCCESSOR {
			word = "successor"
		}
		if d := sup.GetDepth(); d > 1 {
			word += fmt.Sprintf(" %d away", d)
		}
		return word
	case e.GetCitation() != nil:
		refs := e.GetCitation().GetSharedCitations()
		if len(refs) == 0 {
			return "citation"
		}
		out := consoleText(path.Base(refs[0].GetRef()), false)
		if len(refs) > 1 {
			out += fmt.Sprintf(" +%d", len(refs)-1)
		}
		return out
	case e.GetTag() != nil:
		var tags []string
		for _, t := range e.GetTag().GetSharedTags() {
			tags = append(tags, "#"+consoleText(t.GetTag(), false))
		}
		return strings.Join(tags, " ")
	case e.GetVector() != nil:
		return fmt.Sprintf("%.2f", e.GetVector().GetScore())
	}
	return ""
}

func laneLabel(edge engramv1.EdgeType) (letter, name string) {
	for _, l := range relatedLanes {
		if l.edge == edge {
			return l.letter, l.name
		}
	}
	return "?", "related"
}

// listItem is one line of a list: a row in view (pos >= 0), or a lane
// heading or the paging line (pos < 0).
type listItem struct {
	pos  int
	text string
}

// listItems lays out every line of the list before windowing.
func (m *consoleModel) listItems(s *screen) []listItem {
	items := make([]listItem, 0, s.count()+5)
	lastLane := engramv1.EdgeType(-1)
	for pos := range s.count() {
		if s.kind == screenRelated {
			if lane := s.lanes[s.index(pos)]; lane != lastLane {
				lastLane = lane
				n := 0
				for p := range s.count() {
					if s.lanes[s.index(p)] == lane {
						n++
					}
				}
				letter, name := laneLabel(lane)
				items = append(items, listItem{pos: -1, text: fmt.Sprintf("%s %s (%d)", letter, name, n)})
			}
		}
		items = append(items, listItem{pos: pos})
	}
	if t := m.pagingLine(s); t != "" {
		items = append(items, listItem{pos: -1, text: t})
	}
	return items
}

// pagingLine is the line under a paged list: loading, more, the end, or a
// failed page.
func (m *consoleModel) pagingLine(s *screen) string {
	pageable := (s.kind == screenMemories && !s.searched) || s.kind == screenScheduled
	switch {
	case !pageable || !s.loaded || s.narrow != "":
		return ""
	case s.pageErr:
		return "couldn't load more · n to retry"
	case s.nextToken != "" && s.loading:
		return fmt.Sprintf("loading %d more…", consolePageSize)
	case s.nextToken != "":
		return "more below · n loads the next page"
	case s.count() > 0:
		noun := "memories"
		if s.kind == screenScheduled {
			noun = "records"
		}
		return fmt.Sprintf("end · %d %s", s.count(), noun)
	}
	return ""
}

// listLines renders the visible window of the list at w by h, keeping the
// selected row (and its lane heading) in view.
func (m *consoleModel) listLines(s *screen, w, h int) []string {
	items := m.listItems(s)
	cursorLine := 0
	for i, it := range items {
		if it.pos == s.cursor {
			cursorLine = i
			break
		}
	}
	bottom := cursorLine
	if cursorLine == len(items)-2 && items[len(items)-1].pos < 0 {
		bottom++ // keep the paging line in view at the last row
	}
	top := max(min(s.top, cursorLine), bottom-h+1, 0)
	if top == cursorLine && top > 0 && items[top-1].pos < 0 {
		top--
	}
	s.top = top
	var cols rowColumns
	maxCount := uint64(1)
	if s.kind == screenTags {
		for _, t := range s.tags {
			maxCount = max(maxCount, t.GetCount())
		}
	} else {
		cols = m.columns(s, w)
	}
	out := make([]string, 0, h)
	for i := top; i < len(items) && len(out) < h; i++ {
		it := items[i]
		switch {
		case it.pos < 0 && s.kind == screenRelated && i < len(items)-1:
			out = append(out, m.st.accent.Bold(true).Render(clip(it.text, w)))
		case it.pos < 0:
			out = append(out, m.st.faint.Render(clip("  "+it.text, w)))
		case s.kind == screenTags:
			out = append(out, m.tagLine(s, s.index(it.pos), w, maxCount, it.pos == s.cursor))
		default:
			out = append(out, m.rowLine(s, s.index(it.pos), w, cols, it.pos == s.cursor))
		}
	}
	return out
}

// tagLine renders one tag with its count as a bar sized against the most
// used tag.
func (m *consoleModel) tagLine(s *screen, idx, w int, maxCount uint64, selected bool) string {
	st := m.st
	t := s.tags[idx]
	bar := " "
	if selected {
		bar = st.barBlurred.Render("▌")
		if m.focus == focusList {
			bar = st.bar.Render("▌")
		}
	}
	check := "  "
	if slices.Contains(m.chosenTags, t.GetTag()) {
		check = st.success.Render("✓ ")
	}
	nameW := min(max(w/3, 12), 32)
	name := clip("#"+consoleText(t.GetTag(), false), nameW)
	count := fmt.Sprintf("%6d", t.GetCount())
	barW := max(w-2-2-nameW-1-ansi.StringWidth(count)-1, 1)
	n := max(int(float64(t.GetCount())/float64(maxCount)*float64(barW)), 1)
	nameStyle := st.text
	if selected {
		nameStyle = st.bold
	}
	return bar + " " + check + nameStyle.Render(name) + " " +
		st.accent.Render(strings.Repeat("■", n)) + strings.Repeat(" ", barW-n) + " " + st.muted.Render(count)
}
