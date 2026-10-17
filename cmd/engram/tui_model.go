// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"regexp"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"connectrpc.com/connect"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
)

const (
	// consolePageSize is the list and scheduled page size and the search k:
	// well under the 1000 maximum, so a large scope never asks for a
	// response that runs into response_too_large.
	consolePageSize = 50
	consoleTagLimit = 100
)

// The console's id classifier mirrors the web console's
// (ui/src/lib/search/classify.ts): a single UUID- or short_id-shaped token
// opens that record by id and is never sent as a semantic search query.
var (
	consoleUUIDRe    = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	consoleShortIDRe = regexp.MustCompile(`(?i)^[0-9a-hjkmnp-tv-z]{10}$`)
)

type consoleTab int

const (
	tabMemories consoleTab = iota
	tabTags
	tabRules
	tabScheduled
)

var consoleTabNames = [...]string{"memories", "tags", "rules", "scheduled"}

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
// screens stack above the active tab, and Esc pops them.
type screen struct {
	kind screenKind
	// seq is the request this screen waits on. A result that carries any
	// other seq is stale and is dropped, so a slow response cannot
	// overwrite a newer one.
	seq     uint64
	loaded  bool
	loading bool
	cursor  int
	top     int

	rows  []*engramv1.Memory
	edges [][]engramv1.EdgeType // related: parallel to rows
	tags  []*engramv1.TagCount

	query     string // memories: the search query; "" lists
	searched  bool   // memories: the rows are search hits
	nextToken string
	footer    string

	id       string // detail and related: the record asked for
	record   *engramv1.Memory
	notFound bool
	view     viewport.Model

	expired bool // scheduled: lists expired records instead of scheduled ones
}

// rpcResult carries one RPC's outcome back to Update. apply writes a
// successful response into the screen waiting on seq.
type rpcResult struct {
	seq   uint64
	apply func(*screen)
	err   error
}

// consoleModel is the terminal console's Bubble Tea model. It only ever
// sends read RPCs.
type consoleModel struct {
	ctx     context.Context
	client  engramv1connect.EngramServiceClient
	timeout time.Duration

	keys          consoleKeyMap
	help          help.Model
	input         textinput.Model
	width, height int

	scopes            []*engramv1.ScopeCount
	scopesApproximate bool
	scope             string
	crossSpine        bool
	includeArchived   bool
	includeSuperseded bool
	includeScheduled  bool

	tab   consoleTab
	tabs  [4]screen
	stack []screen

	filtersOpen    bool
	filterCursor   int
	filtersChanged bool

	seq       uint64
	status    string
	statusErr bool
}

func newConsoleModel(ctx context.Context, client engramv1connect.EngramServiceClient, timeout time.Duration, scopes *engramv1.ListScopesResponse) *consoleModel {
	in := textinput.New()
	in.Prompt = "/ "
	in.Placeholder = "search, or a short_id or id to open"
	// A real terminal cursor instead of the virtual one: the virtual
	// cursor blinks on a timer, which the console does not need.
	in.SetVirtualCursor(false)
	m := &consoleModel{
		ctx:               ctx,
		client:            client,
		timeout:           timeout,
		keys:              newConsoleKeyMap(),
		help:              help.New(),
		input:             in,
		width:             80,
		height:            24,
		scopes:            scopes.GetScopes(),
		scopesApproximate: scopes.GetApproximate(),
		crossSpine:        true,
	}
	for i := range m.tabs {
		m.tabs[i].kind = screenKind(i)
	}
	return m
}

func (m *consoleModel) Init() tea.Cmd {
	return m.load(&m.tabs[tabMemories])
}

func (m *consoleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.SetWidth(msg.Width)
		m.input.SetWidth(max(msg.Width-lipgloss.Width(m.input.Prompt)-1, 10))
		for i := range m.stack {
			m.sizeViewport(&m.stack[i])
		}
	case rpcResult:
		m.applyResult(msg)
	case tea.KeyPressMsg:
		return m, m.handleKey(msg)
	}
	return m, nil
}

func (m *consoleModel) active() *screen {
	if n := len(m.stack); n > 0 {
		return &m.stack[n-1]
	}
	return &m.tabs[m.tab]
}

func (m *consoleModel) applyResult(r rpcResult) {
	var s *screen
	for i := range m.tabs {
		if m.tabs[i].seq == r.seq {
			s = &m.tabs[i]
		}
	}
	for i := range m.stack {
		if m.stack[i].seq == r.seq {
			s = &m.stack[i]
		}
	}
	if s == nil {
		return
	}
	s.loading = false
	if r.err != nil {
		// The server's message goes out unchanged, field=/hint= envelope
		// included, and the session goes on.
		m.status, m.statusErr = "error: "+r.err.Error(), true
		return
	}
	s.loaded = true
	r.apply(s)
	if s.kind == screenDetail {
		m.sizeViewport(s)
		s.view.SetContent(m.renderDetail(s))
	}
	s.cursor = min(s.cursor, max(s.rowCount()-1, 0))
}

func (s *screen) rowCount() int {
	if s.kind == screenTags {
		return len(s.tags)
	}
	return len(s.rows)
}

func (s *screen) selected() *engramv1.Memory {
	if s.kind == screenDetail {
		return s.record
	}
	if s.kind == screenTags || s.cursor >= len(s.rows) {
		return nil
	}
	return s.rows[s.cursor]
}

func (m *consoleModel) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	if key.Matches(msg, m.keys.ForceQuit) {
		return tea.Quit
	}
	if m.input.Focused() {
		return m.handleInputKey(msg)
	}
	if m.filtersOpen {
		return m.handleFilterKey(msg)
	}
	s := m.active()
	switch {
	case key.Matches(msg, m.keys.Quit):
		return tea.Quit
	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
	case key.Matches(msg, m.keys.Back):
		switch {
		case m.help.ShowAll:
			m.help.ShowAll = false
		case len(m.stack) > 0:
			m.stack = m.stack[:len(m.stack)-1]
		}
	case key.Matches(msg, m.keys.Search):
		m.stack = nil
		m.tab = tabMemories
		m.input.Reset()
		return m.input.Focus()
	case key.Matches(msg, m.keys.Filters):
		m.filtersOpen, m.filterCursor, m.filtersChanged = true, 0, false
	case key.Matches(msg, m.keys.Memories):
		return m.openTab(tabMemories)
	case key.Matches(msg, m.keys.Tags):
		return m.openTab(tabTags)
	case key.Matches(msg, m.keys.Rules):
		return m.openTab(tabRules)
	case key.Matches(msg, m.keys.Scheduled):
		return m.openTab(tabScheduled)
	case key.Matches(msg, m.keys.Up):
		m.move(s, -1)
	case key.Matches(msg, m.keys.Down):
		m.move(s, 1)
	case key.Matches(msg, m.keys.PageUp):
		m.move(s, -m.listHeight())
	case key.Matches(msg, m.keys.PageDown):
		m.move(s, m.listHeight())
	case key.Matches(msg, m.keys.Home):
		s.cursor = 0
		s.view.GotoTop()
	case key.Matches(msg, m.keys.End):
		s.cursor = max(s.rowCount()-1, 0)
		s.view.GotoBottom()
	case key.Matches(msg, m.keys.Open):
		if s.kind != screenDetail {
			if r := s.selected(); r != nil {
				return m.push(screen{kind: screenDetail, id: r.GetId()})
			}
		}
	case key.Matches(msg, m.keys.Related):
		if r := s.selected(); r != nil {
			return m.push(screen{kind: screenRelated, id: r.GetId(), record: r})
		}
	case key.Matches(msg, m.keys.More):
		if s.nextToken != "" && !s.loading {
			return m.loadPage(s, s.nextToken)
		}
	case key.Matches(msg, m.keys.Switch):
		if s.kind == screenScheduled {
			s.expired = !s.expired
			return m.load(s)
		}
	}
	return nil
}

func (m *consoleModel) handleInputKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.input.Blur()
		return nil
	case "enter":
		m.input.Blur()
		q := strings.TrimSpace(m.input.Value())
		if consoleUUIDRe.MatchString(q) || consoleShortIDRe.MatchString(q) {
			return m.push(screen{kind: screenDetail, id: q})
		}
		s := &m.tabs[tabMemories]
		s.query = q
		return m.load(s)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

// filterEntries is the filter panel's row count: the three include
// toggles, the cross-spine choice, then one row per readable scope.
func (m *consoleModel) filterEntries() int { return 4 + len(m.scopes) }

func (m *consoleModel) handleFilterKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return tea.Quit
	case key.Matches(msg, m.keys.Back):
		return m.closeFilters()
	case key.Matches(msg, m.keys.Up):
		m.filterCursor = max(m.filterCursor-1, 0)
	case key.Matches(msg, m.keys.Down):
		m.filterCursor = min(m.filterCursor+1, m.filterEntries()-1)
	case key.Matches(msg, m.keys.Home):
		m.filterCursor = 0
	case key.Matches(msg, m.keys.End):
		m.filterCursor = m.filterEntries() - 1
	case key.Matches(msg, m.keys.Toggle):
		m.chooseFilter()
	case key.Matches(msg, m.keys.Open):
		m.chooseFilter()
		return m.closeFilters()
	}
	return nil
}

func (m *consoleModel) chooseFilter() {
	m.filtersChanged = true
	switch c := m.filterCursor; c {
	case 0:
		m.includeArchived = !m.includeArchived
	case 1:
		m.includeSuperseded = !m.includeSuperseded
	case 2:
		m.includeScheduled = !m.includeScheduled
	case 3:
		m.crossSpine, m.scope = true, ""
	default:
		m.crossSpine, m.scope = false, m.scopes[c-4].GetScope()
	}
}

// closeFilters reloads every tab the filters feed. The active tab reloads
// now; the others reload when next opened.
func (m *consoleModel) closeFilters() tea.Cmd {
	m.filtersOpen = false
	if !m.filtersChanged {
		return nil
	}
	m.tabs[tabMemories].loaded = false
	m.tabs[tabTags].loaded = false
	m.tabs[tabScheduled].loaded = false
	m.stack = nil
	return m.openTab(m.tab)
}

func (m *consoleModel) openTab(t consoleTab) tea.Cmd {
	m.stack = nil
	m.tab = t
	if s := &m.tabs[t]; !s.loaded && !s.loading {
		return m.load(s)
	}
	return nil
}

func (m *consoleModel) push(s screen) tea.Cmd {
	m.sizeViewport(&s)
	m.stack = append(m.stack, s)
	return m.load(&m.stack[len(m.stack)-1])
}

func (m *consoleModel) move(s *screen, delta int) {
	if s.kind == screenDetail {
		if delta < 0 {
			s.view.ScrollUp(-delta)
		} else {
			s.view.ScrollDown(delta)
		}
		return
	}
	s.cursor = max(min(s.cursor+delta, s.rowCount()-1), 0)
}

func (m *consoleModel) sizeViewport(s *screen) {
	if s.kind != screenDetail {
		return
	}
	if s.view.Width() == 0 {
		s.view = viewport.New()
		s.view.SoftWrap = true
	}
	s.view.SetWidth(m.width)
	s.view.SetHeight(m.listHeight())
}

// load starts a screen's first page.
func (m *consoleModel) load(s *screen) tea.Cmd {
	s.cursor, s.top = 0, 0
	return m.loadPage(s, "")
}

// loadPage starts the request that fills s, or with a page token, the next
// page that extends it.
func (m *consoleModel) loadPage(s *screen, token string) tea.Cmd {
	m.seq++
	s.seq, s.loading = m.seq, true
	m.status, m.statusErr = "", false
	call := m.call(s, token)
	ctx, timeout, seq := m.ctx, m.timeout, m.seq
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		apply, err := call(ctx)
		return rpcResult{seq: seq, apply: apply, err: err}
	}
}

// call builds the read RPC for s from the model's current settings. It
// copies everything it needs, so the returned function is safe to run off
// the update loop.
func (m *consoleModel) call(s *screen, token string) func(context.Context) (func(*screen), error) {
	c := m.client
	scope, cross := m.scope, m.crossSpine
	switch s.kind {
	case screenMemories:
		if s.query == "" {
			req := &engramv1.ListMemoriesRequest{
				Scope: scope, CrossSpine: cross, Limit: consolePageSize,
				IncludeArchived: m.includeArchived, IncludeSuperseded: m.includeSuperseded,
				IncludeScheduled: m.includeScheduled,
			}
			if token == "" {
				req.CursorMode = true
			} else {
				req.PageToken = token
			}
			return func(ctx context.Context) (func(*screen), error) {
				resp, err := c.ListMemories(ctx, connect.NewRequest(req))
				if err != nil {
					return nil, err
				}
				r := resp.Msg
				return func(s *screen) {
					if token == "" {
						s.rows = nil
					}
					s.rows = append(s.rows, r.GetMemories()...)
					s.searched, s.nextToken = false, r.GetNextPageToken()
					s.footer = recallFooter(cross, r.GetSearchedScopes(), r.GetScopesTruncated(), r.GetScopesUnknown(), r.GetRecallGateHidden())
				}, nil
			}
		}
		req := &engramv1.SearchMemoriesRequest{
			Query: s.query, Scope: scope, CrossSpine: cross, K: consolePageSize,
			IncludeArchived: m.includeArchived, IncludeSuperseded: m.includeSuperseded,
			IncludeScheduled: m.includeScheduled,
		}
		return func(ctx context.Context) (func(*screen), error) {
			resp, err := c.SearchMemories(ctx, connect.NewRequest(req))
			if err != nil {
				return nil, err
			}
			r := resp.Msg
			return func(s *screen) {
				s.rows, s.searched, s.nextToken = r.GetMemories(), true, ""
				s.footer = recallFooter(cross, r.GetSearchedScopes(), r.GetScopesTruncated(), r.GetScopesUnknown(), r.GetRecallGateHidden())
			}, nil
		}
	case screenTags:
		req := &engramv1.ListTagsRequest{Limit: consoleTagLimit}
		if !cross {
			req.Scope = scope
		}
		return func(ctx context.Context) (func(*screen), error) {
			resp, err := c.ListTags(ctx, connect.NewRequest(req))
			if err != nil {
				return nil, err
			}
			r := resp.Msg
			return func(s *screen) {
				s.tags, s.footer = r.GetTags(), ""
				if r.GetMore() {
					s.footer = "more: true (more tags exist than shown)\n"
				}
			}, nil
		}
	case screenRules:
		req := &engramv1.ListRulesRequest{}
		return func(ctx context.Context) (func(*screen), error) {
			resp, err := c.ListRules(ctx, connect.NewRequest(req))
			if err != nil {
				return nil, err
			}
			r := resp.Msg
			return func(s *screen) {
				s.rows = r.GetRules()
				s.footer = recallFooter(true, r.GetSearchedScopes(), r.GetScopesTruncated(), r.GetScopesUnknown(), nil)
				if a := r.GetAdvisory(); a != "" {
					s.footer += a + "\n"
				}
			}, nil
		}
	case screenScheduled:
		req := &engramv1.ListScheduledRequest{
			Scope: scope, CrossSpine: cross, Limit: consolePageSize, State: "scheduled", PageToken: token,
		}
		if s.expired {
			req.State = "expired"
		}
		return func(ctx context.Context) (func(*screen), error) {
			resp, err := c.ListScheduled(ctx, connect.NewRequest(req))
			if err != nil {
				return nil, err
			}
			r := resp.Msg
			return func(s *screen) {
				if token == "" {
					s.rows = nil
				}
				s.rows = append(s.rows, r.GetMemories()...)
				s.nextToken = r.GetNextPageToken()
				s.footer = recallFooter(cross, r.GetSearchedScopes(), r.GetScopesTruncated(), r.GetScopesUnknown(), nil)
			}, nil
		}
	case screenDetail:
		req := &engramv1.GetMemoryRequest{Id: s.id}
		return func(ctx context.Context) (func(*screen), error) {
			resp, err := c.GetMemory(ctx, connect.NewRequest(req))
			if connect.CodeOf(err) == connect.CodeNotFound {
				// Another actor's private record and a missing one read
				// the same, by design: one fixed message for both.
				return func(s *screen) { s.notFound = true }, nil
			}
			if err != nil {
				return nil, err
			}
			return func(s *screen) { s.record = resp.Msg.GetMemory() }, nil
		}
	default: // screenRelated
		req := &engramv1.RelatedMemoriesRequest{Id: s.id}
		return func(ctx context.Context) (func(*screen), error) {
			resp, err := c.RelatedMemories(ctx, connect.NewRequest(req))
			if err != nil {
				return nil, err
			}
			r := resp.Msg
			return func(s *screen) {
				if a := r.GetAnchor(); a != nil {
					s.record = a
				}
				s.rows, s.edges = nil, nil
				for _, rel := range r.GetRelated() {
					s.rows = append(s.rows, rel.GetMemory())
					var types []engramv1.EdgeType
					for _, e := range rel.GetEdges() {
						types = append(types, e.GetType())
					}
					s.edges = append(s.edges, types)
				}
				s.footer = ""
				if r.GetTruncated() {
					s.footer = "truncated: true (more vector neighbours exist than shown)\n"
				}
			}, nil
		}
	}
}

// recallFooter renders the coverage and recall-gate footers the text lane
// of `engram list` and `engram search` prints, from the same helpers.
func recallFooter(crossSpine bool, searched []string, truncated, unknown bool, hidden *engramv1.RecallGateHidden) string {
	var b strings.Builder
	_ = renderCoverageFooter(&b, crossSpine, searched, truncated, unknown)
	_ = renderRecallHiddenFooter(&b, hidden)
	return b.String()
}

// consoleKeyMap uses the web console's key wherever it has one for the
// same action (engram-console-conventions).
type consoleKeyMap struct {
	Search, Open, Back, Up, Down, PageUp, PageDown, Home, End key.Binding
	Related, More, Filters, Toggle, Switch                    key.Binding
	Memories, Tags, Rules, Scheduled                          key.Binding
	Help, Quit, ForceQuit                                     key.Binding
}

func newConsoleKeyMap() consoleKeyMap {
	return consoleKeyMap{
		Search:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		Open:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Back:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Up:        key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:      key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		PageUp:    key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")),
		PageDown:  key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdown", "page down")),
		Home:      key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "first")),
		End:       key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "last")),
		Related:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "related")),
		More:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next page")),
		Filters:   key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "scope and filters")),
		Toggle:    key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle")),
		Switch:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "scheduled/expired")),
		Memories:  key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "memories")),
		Tags:      key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "tags")),
		Rules:     key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "rules")),
		Scheduled: key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "scheduled")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:      key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		ForceQuit: key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
	}
}

func (k consoleKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Search, k.Open, k.Back, k.Related, k.Filters, k.Help, k.Quit}
}

func (k consoleKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Search, k.Open, k.Back, k.Related, k.More},
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Home, k.End},
		{k.Memories, k.Tags, k.Rules, k.Scheduled, k.Switch},
		{k.Filters, k.Toggle, k.Help, k.Quit, k.ForceQuit},
	}
}
