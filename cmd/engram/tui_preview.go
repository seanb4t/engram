// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	"charm.land/lipgloss/v2"
	"connectrpc.com/connect"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

const recordCacheSize = 128

// previewSettleMsg fires once the selection has rested; tag identifies the
// move it belongs to, so an earlier move's timer does nothing.
type previewSettleMsg struct {
	tag uint64
	id  string
}

// previewLoadingMsg fires when a fetch has taken long enough to say so.
type previewLoadingMsg struct {
	tag uint64
	id  string
}

// recordMsg is a fetched record and its rendered content.
type recordMsg struct {
	id       string
	mem      *engramv1.Memory
	notFound bool
	err      error
	key      string
	rendered string
	prefetch bool
}

// renderMsg is content rendered again for a new width or theme.
type renderMsg struct{ key, out string }

// previewState is the preview's data: fetched records, rendered content,
// the timers and fetch in flight, and the scroll position.
type previewState struct {
	records   *recordCache
	rendered  map[string]string
	renderers *rendererPool
	settleTag uint64
	loadingID string
	// errs are failed fetches by id, shown in the preview pane.
	errs     map[string]string
	cancel   context.CancelFunc
	prefetch []string
	offset   int
	shown    bool
}

func newPreviewState() previewState {
	return previewState{
		records:   newRecordCache(recordCacheSize),
		rendered:  map[string]string{},
		errs:      map[string]string{},
		renderers: &rendererPool{r: map[string]*glamour.TermRenderer{}},
	}
}

// recordCache keeps the most recently fetched records. A nil record means
// the server reported it not found.
type recordCache struct {
	capacity int
	order    *list.List
	items    map[string]*list.Element
}

type cacheEntry struct {
	id  string
	mem *engramv1.Memory
}

func newRecordCache(capacity int) *recordCache {
	return &recordCache{capacity: capacity, order: list.New(), items: map[string]*list.Element{}}
}

func (c *recordCache) get(id string) (*engramv1.Memory, bool) {
	e, ok := c.items[id]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(e)
	return e.Value.(*cacheEntry).mem, true
}

func (c *recordCache) put(id string, mem *engramv1.Memory) {
	if e, ok := c.items[id]; ok {
		e.Value.(*cacheEntry).mem = mem
		c.order.MoveToFront(e)
		return
	}
	c.items[id] = c.order.PushFront(&cacheEntry{id: id, mem: mem})
	if c.order.Len() > c.capacity {
		last := c.order.Back()
		c.order.Remove(last)
		delete(c.items, last.Value.(*cacheEntry).id)
	}
}

// rendererPool shares Glamour renderers by width and style. A renderer is
// not safe for concurrent use, so every render holds the lock.
type rendererPool struct {
	mu sync.Mutex
	r  map[string]*glamour.TermRenderer
}

func (p *rendererPool) render(content string, width int, style ansi.StyleConfig, styleKey string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	k := fmt.Sprintf("%s|%d", styleKey, width)
	tr, ok := p.r[k]
	if !ok {
		var err error
		tr, err = glamour.NewTermRenderer(glamour.WithStyles(style), glamour.WithWordWrap(width), glamour.WithPreservedNewLines())
		if err != nil {
			return "", err
		}
		if len(p.r) > 16 {
			p.r = map[string]*glamour.TermRenderer{}
		}
		p.r[k] = tr
	}
	out, err := tr.Render(content)
	return strings.Trim(out, "\n"), err
}

func (m *consoleModel) renderKey(id string, width int) string {
	return fmt.Sprintf("%s|%d|%s", id, width, m.styleKey())
}

// previewTarget is the id the preview or the full-screen record shows.
func (m *consoleModel) previewTarget() string {
	s := m.active()
	if s.kind == screenDetail {
		return s.id
	}
	if r := s.selectedRow(); r != nil {
		return r.GetId()
	}
	return ""
}

// selectionChanged lets the preview follow the selection: a cached record
// shows at once, otherwise the fetch waits for the selection to settle.
func (m *consoleModel) selectionChanged() tea.Cmd {
	m.preview.offset, m.preview.loadingID, m.preview.prefetch = 0, "", nil
	id := m.previewTarget()
	if id == "" {
		return nil
	}
	delete(m.preview.errs, id)
	if _, ok := m.preview.records.get(id); ok {
		m.cancelFetch()
		return m.ensureRendered()
	}
	if !m.previewVisible() {
		return nil
	}
	m.preview.settleTag++
	if !m.preview.shown {
		m.preview.shown = true
		return m.fetch(id, false)
	}
	return m.after(previewSettle, previewSettleMsg{tag: m.preview.settleTag, id: id})
}

// showRecord fetches a record for the full-screen view at once.
func (m *consoleModel) showRecord(id string) tea.Cmd {
	if _, ok := m.preview.records.get(id); ok {
		return m.ensureRendered()
	}
	m.preview.settleTag++
	return tea.Batch(m.fetch(id, false),
		m.after(previewLoadingDelay, previewLoadingMsg{tag: m.preview.settleTag, id: id}))
}

func (m *consoleModel) cancelFetch() {
	if m.preview.cancel != nil {
		m.preview.cancel()
		m.preview.cancel = nil
	}
}

// fetch gets a record and renders its content off the update loop. A
// fetch for the selection cancels the one before it; a prefetch does not.
func (m *consoleModel) fetch(id string, prefetch bool) tea.Cmd {
	ctx, cancel := context.WithTimeout(m.ctx, m.timeout)
	if !prefetch {
		m.cancelFetch()
		m.preview.cancel = cancel
	}
	c, pool := m.client, m.preview.renderers
	w := m.markdownWidth()
	style, styleKey, key := markdownStyle(m.st.roles), m.styleKey(), m.renderKey(id, w)
	return func() tea.Msg {
		defer cancel()
		resp, err := c.GetMemory(ctx, connect.NewRequest(&engramv1.GetMemoryRequest{Id: id}))
		if connect.CodeOf(err) == connect.CodeNotFound {
			// Another actor's private record and a missing one read the
			// same, by design: one fixed message for both.
			return recordMsg{id: id, notFound: true, prefetch: prefetch}
		}
		if err != nil {
			return recordMsg{id: id, err: err, prefetch: prefetch}
		}
		mem := resp.Msg.GetMemory()
		out, _ := pool.render(consoleText(mem.GetContent(), true), w, style, styleKey)
		return recordMsg{id: id, mem: mem, key: key, rendered: out, prefetch: prefetch}
	}
}

// ensureRendered renders the target's content for the current width and
// theme when the cache does not hold it yet.
func (m *consoleModel) ensureRendered() tea.Cmd {
	m.clampPreviewOffset()
	id := m.previewTarget()
	mem, ok := m.preview.records.get(id)
	if !ok || mem == nil {
		return nil
	}
	w := m.markdownWidth()
	key := m.renderKey(id, w)
	if _, ok := m.preview.rendered[key]; ok {
		return nil
	}
	pool, style, styleKey := m.preview.renderers, markdownStyle(m.st.roles), m.styleKey()
	content := consoleText(mem.GetContent(), true)
	return func() tea.Msg {
		out, _ := pool.render(content, w, style, styleKey)
		return renderMsg{key: key, out: out}
	}
}

func (m *consoleModel) updatePreview(msg tea.Msg) tea.Cmd {
	p := &m.preview
	switch msg := msg.(type) {
	case previewSettleMsg:
		if msg.tag != p.settleTag || msg.id != m.previewTarget() {
			return nil
		}
		if _, ok := p.records.get(msg.id); ok {
			return m.ensureRendered()
		}
		return tea.Batch(m.fetch(msg.id, false),
			m.after(previewLoadingDelay, previewLoadingMsg(msg)))
	case previewLoadingMsg:
		if _, ok := p.records.get(msg.id); !ok && msg.tag == p.settleTag && msg.id == m.previewTarget() {
			p.loadingID = msg.id
		}
	case renderMsg:
		p.rendered[msg.key] = msg.out
	case recordMsg:
		if msg.err != nil {
			if errors.Is(msg.err, context.Canceled) || connect.CodeOf(msg.err) == connect.CodeCanceled {
				return nil
			}
			if msg.prefetch {
				return m.nextPrefetch()
			}
			// The server's message goes out unchanged, field=/hint=
			// envelope included, in the pane that asked for it.
			p.errs[msg.id] = "error: " + consoleText(msg.err.Error(), false)
			if p.loadingID == msg.id {
				p.loadingID = ""
			}
			return nil
		}
		delete(p.errs, msg.id)
		p.records.put(msg.id, msg.mem)
		if msg.mem != nil && msg.mem.GetId() != msg.id {
			p.records.put(msg.mem.GetId(), msg.mem)
		}
		if msg.rendered != "" {
			p.rendered[msg.key] = msg.rendered
		}
		if p.loadingID == msg.id {
			p.loadingID = ""
		}
		if msg.prefetch {
			return m.nextPrefetch()
		}
		if msg.id != m.previewTarget() {
			return nil
		}
		m.queuePrefetch()
		return tea.Batch(m.ensureRendered(), m.nextPrefetch())
	}
	return nil
}

// queuePrefetch lines up the rows directly above and below the selection.
func (m *consoleModel) queuePrefetch() {
	s := m.active()
	m.preview.prefetch = nil
	if s.kind == screenDetail || s.kind == screenTags {
		return
	}
	for _, pos := range []int{s.cursor + 1, s.cursor - 1} {
		if pos < 0 || pos >= s.count() {
			continue
		}
		id := s.rows[s.index(pos)].GetId()
		if _, ok := m.preview.records.get(id); !ok {
			m.preview.prefetch = append(m.preview.prefetch, id)
		}
	}
}

// nextPrefetch fetches the next queued neighbour, one at a time.
func (m *consoleModel) nextPrefetch() tea.Cmd {
	for len(m.preview.prefetch) > 0 {
		id := m.preview.prefetch[0]
		m.preview.prefetch = m.preview.prefetch[1:]
		if _, ok := m.preview.records.get(id); !ok {
			return m.fetch(id, true)
		}
	}
	return nil
}

// previewKey handles the keys that scroll the preview while it has focus.
func (m *consoleModel) previewKey(msg tea.KeyPressMsg) bool {
	k := m.keys
	switch {
	case key.Matches(msg, k.ScrollPane):
		if key.Matches(msg, k.Down) {
			m.scrollPreview(1)
		} else {
			m.scrollPreview(-1)
		}
	case key.Matches(msg, k.PageDown):
		m.scrollPreview(m.previewHeight())
	case key.Matches(msg, k.PageUp):
		m.scrollPreview(-m.previewHeight())
	case key.Matches(msg, k.Home):
		m.preview.offset = 0
	case key.Matches(msg, k.End):
		m.scrollPreview(1 << 20)
	default:
		return false
	}
	return true
}

func (m *consoleModel) scrollPreview(delta int) {
	m.preview.offset += delta
	m.clampPreviewOffset()
}

func (m *consoleModel) clampPreviewOffset() {
	lines := len(m.previewContent(m.previewWidth()))
	m.preview.offset = max(min(m.preview.offset, lines-m.previewHeight()), 0)
}

// wrap breaks plain text into lines of at most w cells.
func wrap(s string, w int) []string {
	if s == "" {
		return nil
	}
	return strings.Split(lipgloss.NewStyle().Width(max(w, 1)).Render(s), "\n")
}

// previewContent is every line of the preview for width w, before
// scrolling: the row's data at once, then the record's rendered content.
func (m *consoleModel) previewContent(w int) []string {
	s := m.active()
	st := m.st
	if s.kind == screenTags {
		t := s.selectedTag()
		if t == "" {
			return nil
		}
		count := s.tags[s.index(s.cursor)].GetCount()
		return []string{
			st.bold.Render(clip("#"+consoleText(t, false), w)),
			st.muted.Render(clip(fmt.Sprintf("%d memories you can recall carry this tag", count), w)),
			"",
			st.muted.Render(clip("space chooses it · enter lists the memories with every chosen tag", w)),
		}
	}
	id := m.previewTarget()
	if id == "" {
		return nil
	}
	mem, cached := m.preview.records.get(id)
	if cached && mem == nil {
		return []string{st.warning.Render(clip(consoleNotFound, w))}
	}
	r := mem
	if r == nil {
		r = s.selectedRow()
	}
	if r == nil {
		r = s.anchor
	}
	if r == nil {
		if m.preview.loadingID == id {
			return []string{st.faint.Render("loading…")}
		}
		return nil
	}
	now := m.now()
	title := r.GetSummary()
	if title == "" {
		title = strings.SplitN(r.GetContent(), "\n", 2)[0]
	}
	var out []string
	for _, l := range wrap(consoleText(title, false), w) {
		out = append(out, st.bold.Render(l))
	}
	cat := consoleText(r.GetCategory(), false)
	visibility := r.GetVisibility()
	if visibility == "" {
		// The store keeps a private record's visibility empty.
		visibility = "private"
	}
	meta := []string{st.category(r.GetCategory()).Render("● " + cat), consoleText(r.GetScope(), false),
		consoleText(visibility, false), "owner " + consoleText(r.GetOwner(), false)}
	out = append(out, wrapStyled(strings.Join(meta, st.faint.Render(" · ")), w)...)
	ids := []string{consoleText(r.GetShortId(), false)}
	if c := r.GetCreatedAt(); c != nil {
		ids = append(ids, c.AsTime().UTC().Format("2006-01-02 15:04 UTC")+" ("+agoWords(now.Sub(c.AsTime()))+")")
	}
	out = append(out, st.muted.Render(clip(strings.Join(ids, " · "), w)))
	if words := memoryStateWords(r, now); len(words) > 0 {
		state := strings.Join(words, " ")
		if ww := windowWords(r, now); ww != "" {
			state += " · " + ww
		}
		out = append(out, st.warning.Render(clip("state: "+state, w)))
	}
	if sb := r.GetSupersededBy(); sb != "" {
		out = append(out, st.muted.Render(clip("superseded by "+consoleText(sb, false), w)))
	}
	if sp := r.GetSupersedes(); len(sp) > 0 {
		out = append(out, st.muted.Render(clip("supersedes "+consoleText(strings.Join(sp, ", "), false), w)))
	}
	if tags := r.GetTags(); len(tags) > 0 {
		var t []string
		for _, tag := range tags {
			t = append(t, "#"+consoleText(tag, false))
		}
		for _, l := range wrap(strings.Join(t, " "), w) {
			out = append(out, st.accent.Render(l))
		}
	}
	out = append(out, st.divider.Render("── content "+strings.Repeat("─", max(w-11, 0))))
	switch {
	case m.preview.errs[id] != "":
		for _, l := range wrap(m.preview.errs[id], w) {
			out = append(out, st.errorFg.Render(l))
		}
	case mem != nil:
		if rendered, ok := m.preview.rendered[m.renderKey(id, m.markdownWidth())]; ok {
			out = append(out, strings.Split(rendered, "\n")...)
		} else {
			for _, l := range wrap(consoleText(mem.GetContent(), true), m.markdownWidth()) {
				out = append(out, st.text.Render(l))
			}
		}
	case m.preview.loadingID == id:
		out = append(out, st.faint.Render("loading…"))
	}
	return out
}

// wrapStyled wraps a line that already carries styling.
func wrapStyled(s string, w int) []string {
	return strings.Split(lipgloss.NewStyle().Width(max(w, 1)).Render(s), "\n")
}

// previewLines is the visible window of the preview, padded to w by h.
func (m *consoleModel) previewLines(w, h int) []string {
	content := m.previewContent(w)
	off := max(min(m.preview.offset, len(content)-h), 0)
	out := make([]string, 0, h)
	for i := off; i < len(content) && len(out) < h; i++ {
		out = append(out, content[i])
	}
	for len(out) < h {
		out = append(out, "")
	}
	return out
}
