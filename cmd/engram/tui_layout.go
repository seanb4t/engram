// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

// layoutTier is the arrangement the terminal's size allows.
type layoutTier int

const (
	tierSide    layoutTier = iota // list and preview side by side
	tierStacked                   // preview below the list
	tierSingle                    // list only; Enter opens a record full screen
)

// The console draws four lines above the body (tabs, search, filters,
// results header) and two below it (position and status, keys).
const (
	chromeTop    = 4
	chromeBottom = 2
	sideMinWidth = 100
	stackMinRows = 32
	listMinWidth = 48
)

type pane struct{ x, y, w, h int }

func (p pane) contains(x, y int) bool {
	return x >= p.x && x < p.x+p.w && y >= p.y && y < p.y+p.h
}

type consoleLayout struct {
	tier                  layoutTier
	list, preview         pane
	showList, showPreview bool
}

func (m *consoleModel) tier() layoutTier {
	switch {
	case m.width >= sideMinWidth:
		return tierSide
	case m.height >= stackMinRows:
		return tierStacked
	}
	return tierSingle
}

func (m *consoleModel) bodyHeight() int {
	return max(m.height-chromeTop-chromeBottom, 3)
}

// layout places the list and the preview for the current size, the
// active screen and the hide and maximise toggles.
func (m *consoleModel) layout() consoleLayout {
	h := m.bodyHeight()
	full := pane{0, chromeTop, m.width, h}
	l := consoleLayout{tier: m.tier()}
	if m.active().kind == screenDetail {
		l.showPreview, l.preview = true, full
		return l
	}
	l.showList, l.list = true, full
	if l.tier == tierSingle || m.previewHidden {
		return l
	}
	if m.maximised {
		if m.focus == focusPreview {
			l.showList, l.showPreview, l.preview = false, true, full
		}
		return l
	}
	l.showPreview = true
	switch l.tier {
	case tierSide:
		lw := max(listMinWidth, m.width/2)
		l.list = pane{0, chromeTop, lw, h}
		l.preview = pane{lw + 1, chromeTop, m.width - lw - 1, h}
	case tierStacked:
		lh := max(h*55/100, 3)
		l.list = pane{0, chromeTop, m.width, lh}
		l.preview = pane{0, chromeTop + lh + 1, m.width, max(h-lh-1, 1)}
	}
	return l
}

// previewAvailable reports whether the layout has a preview pane at all,
// shown or maximised away; Tab moves focus between it and the list.
func (m *consoleModel) previewAvailable() bool {
	return m.tier() != tierSingle && !m.previewHidden && m.active().kind != screenDetail
}

// previewVisible reports whether a preview pane sits beside the list.
func (m *consoleModel) previewVisible() bool {
	return m.layout().showPreview && m.active().kind != screenDetail
}

func (m *consoleModel) listHeight() int {
	if l := m.layout(); l.showList {
		return max(l.list.h, 1)
	}
	return m.bodyHeight()
}

func (m *consoleModel) previewHeight() int {
	if l := m.layout(); l.showPreview {
		return max(l.preview.h, 1)
	}
	return m.bodyHeight()
}

func (m *consoleModel) previewWidth() int {
	l := m.layout()
	switch {
	case l.showPreview && l.tier == tierSide && l.showList:
		return l.preview.w - 1 // a one-column gutter after the divider
	case l.showPreview:
		return l.preview.w
	}
	return m.width
}

// markdownWidth is the width content wraps at: the preview less a margin,
// and never wider than comfortable reading.
func (m *consoleModel) markdownWidth() int {
	return max(min(m.previewWidth()-2, 100), 20)
}
