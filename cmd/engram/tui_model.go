// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
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
	// previewSettle is how long the selection must rest before the preview
	// fetches the full record; previewLoadingDelay is how long a fetch may
	// take before the preview says it is loading.
	previewSettle       = 100 * time.Millisecond
	previewLoadingDelay = 200 * time.Millisecond
	toastLifetime       = 3 * time.Second
)

// The console's id classifier mirrors the web console's
// (ui/src/lib/search/classify.ts): a single UUID- or short_id-shaped token
// opens that record by id and is never sent as a semantic search query.
var (
	consoleUUIDRe    = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	consoleShortIDRe = regexp.MustCompile(`(?i)^[0-9a-hjkmnp-tv-z]{10}$`)
)

func isIDShaped(q string) bool {
	return consoleUUIDRe.MatchString(q) || consoleShortIDRe.MatchString(q)
}

type consoleTab int

const (
	tabMemories consoleTab = iota
	tabTags
	tabRules
	tabScheduled
)

var consoleTabNames = [...]string{"Memories", "Tags", "Rules", "Scheduled"}

type consoleFocus int

const (
	focusList consoleFocus = iota
	focusPreview
)

// rpcResult carries one RPC's outcome back to Update. apply writes a
// successful response into the screen waiting on seq.
type rpcResult struct {
	seq   uint64
	apply func(*screen)
	err   error
}

type toastExpireMsg struct{ seq uint64 }

type prefsSavedMsg struct{ err error }

// consoleModel is the terminal console's Bubble Tea model. It only ever
// sends read RPCs.
type consoleModel struct {
	ctx     context.Context
	client  engramv1connect.EngramServiceClient
	timeout time.Duration

	keys          consoleKeyMap
	input         textinput.Model
	narrowIn      textinput.Model
	width, height int
	// now and after are the console's clock; a test replaces them so it
	// can deliver timer messages directly instead of waiting.
	now   func() time.Time
	after func(time.Duration, tea.Msg) tea.Cmd

	settings  consoleSettings
	themeName string
	dark      bool
	st        consoleStyles
	mouse     bool

	scopes            []*engramv1.ScopeCount
	scopesApproximate bool
	scope             string
	crossSpine        bool
	includeArchived   bool
	includeSuperseded bool
	includeScheduled  bool
	chosenTags        []string

	tab           consoleTab
	tabs          [4]screen
	stack         []screen
	focus         consoleFocus
	previewHidden bool
	maximised     bool
	overlays      []*overlay

	seq       uint64
	status    string
	statusErr bool
	toast     string
	toastSeq  uint64

	preview previewState
}

func defaultAfter(d time.Duration, msg tea.Msg) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return msg })
}

func newConsoleModel(ctx context.Context, client engramv1connect.EngramServiceClient, timeout time.Duration, scopes *engramv1.ListScopesResponse, settings consoleSettings) *consoleModel {
	newInput := func(prompt, placeholder string) textinput.Model {
		in := textinput.New()
		in.Prompt = prompt
		in.Placeholder = placeholder
		// A real terminal cursor instead of the virtual one: the virtual
		// cursor blinks on a timer, which the console does not need.
		in.SetVirtualCursor(false)
		return in
	}
	m := &consoleModel{
		ctx:               ctx,
		client:            client,
		timeout:           timeout,
		keys:              newConsoleKeyMap(),
		input:             newInput("/ ", "search, or a short_id or id to open"),
		narrowIn:          newInput("= ", "narrow the loaded rows"),
		width:             80,
		height:            24,
		now:               time.Now,
		after:             defaultAfter,
		settings:          settings,
		dark:              true,
		mouse:             settings.mouse,
		scopes:            scopes.GetScopes(),
		scopesApproximate: scopes.GetApproximate(),
		crossSpine:        true,
		preview:           newPreviewState(),
	}
	for i := range m.tabs {
		m.tabs[i].kind = screenKind(i)
	}
	m.setTheme(settings.theme)
	return m
}

func (m *consoleModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.load(&m.tabs[tabMemories]), tea.RequestBackgroundColor}
	if m.settings.note != "" {
		cmds = append(cmds, m.showToast(m.settings.note))
	}
	return tea.Batch(cmds...)
}

// setTheme switches to a named theme in the variant that matches the
// terminal background.
func (m *consoleModel) setTheme(name string) {
	m.themeName = name
	t := themeNamed(name)
	roles := t.dark
	if !m.dark {
		roles = t.light
	}
	if m.settings.noColor {
		roles = noColorRoles()
	}
	m.st = newConsoleStyles(roles)
	m.input.SetStyles(m.st.inputStyles(m.input.Styles()))
	m.narrowIn.SetStyles(m.st.inputStyles(m.narrowIn.Styles()))
	for _, o := range m.overlays {
		o.input.SetStyles(m.st.inputStyles(o.input.Styles()))
	}
}

// styleKey names the markdown style in use, for the render cache.
func (m *consoleModel) styleKey() string {
	switch {
	case m.settings.noColor:
		return "nocolor"
	case m.dark:
		return m.themeName + "/dark"
	}
	return m.themeName + "/light"
}

func (m *consoleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.SetWidth(max(m.width/2, 10))
		m.narrowIn.SetWidth(max(m.width/2, 10))
		m.clampPreviewOffset()
		return m, m.ensureRendered()
	case tea.BackgroundColorMsg:
		m.dark = msg.IsDark()
		m.setTheme(m.themeName)
		return m, m.ensureRendered()
	case rpcResult:
		return m, m.applyResult(msg)
	case previewSettleMsg, previewLoadingMsg, recordMsg, renderMsg:
		return m, m.updatePreview(msg)
	case toastExpireMsg:
		if msg.seq == m.toastSeq {
			m.toast = ""
		}
	case prefsSavedMsg:
		if msg.err != nil {
			m.status, m.statusErr = "error: "+consoleText(msg.err.Error(), false), true
		}
	case tea.MouseClickMsg:
		return m, m.handleClick(msg)
	case tea.MouseWheelMsg:
		return m, m.handleWheel(msg)
	case tea.PasteMsg:
		var cmd tea.Cmd
		switch {
		case m.input.Focused():
			m.input, cmd = m.input.Update(msg)
		case m.narrowIn.Focused():
			m.narrowIn, cmd = m.narrowIn.Update(msg)
			m.setNarrow(m.narrowIn.Value())
			cmd = tea.Batch(cmd, m.selectionChanged())
		}
		return m, cmd
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

// showToast puts a short note in the message slot and clears it later.
func (m *consoleModel) showToast(text string) tea.Cmd {
	m.toastSeq++
	m.toast = text
	return m.after(toastLifetime, toastExpireMsg{seq: m.toastSeq})
}

func (m *consoleModel) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	k := m.keys
	if key.Matches(msg, k.ForceQuit) {
		return tea.Quit
	}
	if len(m.overlays) > 0 {
		return m.overlayKey(msg)
	}
	if m.input.Focused() {
		return m.searchKey(msg)
	}
	if m.narrowIn.Focused() {
		return m.narrowKey(msg)
	}
	if m.focus == focusPreview && m.previewVisible() && m.previewKey(msg) {
		return nil
	}
	s := m.active()
	switch {
	case key.Matches(msg, k.Quit):
		return tea.Quit
	case key.Matches(msg, k.Help):
		m.openOverlay(newHelpOverlay())
	case key.Matches(msg, k.Palette):
		m.openOverlay(m.newPaletteOverlay())
	case key.Matches(msg, k.Theme):
		m.openOverlay(m.newThemeOverlay())
	case key.Matches(msg, k.Filters):
		m.openOverlay(m.newFiltersOverlay())
	case key.Matches(msg, k.Back):
		return m.back()
	case key.Matches(msg, k.Search):
		m.stack, m.tab, m.focus = nil, tabMemories, focusList
		m.input.Reset()
		return m.input.Focus()
	case key.Matches(msg, k.Narrow):
		if s.kind != screenDetail {
			m.narrowIn.SetValue(s.narrow)
			m.narrowIn.CursorEnd()
			return m.narrowIn.Focus()
		}
	case key.Matches(msg, k.Memories):
		return m.openTab(tabMemories)
	case key.Matches(msg, k.Tags):
		return m.openTab(tabTags)
	case key.Matches(msg, k.Rules):
		return m.openTab(tabRules)
	case key.Matches(msg, k.Scheduled):
		return m.openTab(tabScheduled)
	case key.Matches(msg, k.Focus):
		if m.previewAvailable() {
			if m.focus == focusList {
				m.focus = focusPreview
			} else {
				m.focus = focusList
			}
		}
	case key.Matches(msg, k.Up):
		return m.move(s, -1)
	case key.Matches(msg, k.Down):
		return m.move(s, 1)
	case key.Matches(msg, k.PageUp):
		return m.move(s, -m.listHeight())
	case key.Matches(msg, k.PageDown):
		return m.move(s, m.listHeight())
	case key.Matches(msg, k.Home):
		return m.move(s, -1<<20)
	case key.Matches(msg, k.End):
		return m.move(s, 1<<20)
	case key.Matches(msg, k.ScrollDown):
		m.scrollPreview(1)
	case key.Matches(msg, k.ScrollUp):
		m.scrollPreview(-1)
	case key.Matches(msg, k.HalfDown):
		m.scrollPreview(max(m.previewHeight()/2, 1))
	case key.Matches(msg, k.HalfUp):
		m.scrollPreview(-max(m.previewHeight()/2, 1))
	case key.Matches(msg, k.Open):
		return m.enter(s)
	case key.Matches(msg, k.Related):
		if r := m.selectedRecord(); r != nil {
			return m.push(screen{kind: screenRelated, id: r.GetId(), anchor: r})
		}
	case key.Matches(msg, k.TrailBack):
		if s.kind == screenRelated && len(m.stack) > 0 {
			return m.pop()
		}
	case key.Matches(msg, k.Choose):
		if s.kind == screenTags {
			if t := s.selectedTag(); t != "" {
				m.toggleTag(t)
			}
		}
	case key.Matches(msg, k.Window):
		if s.kind == screenScheduled {
			s.expired = !s.expired
			return m.load(s)
		}
	case key.Matches(msg, k.More):
		if s.nextToken != "" && !s.loading {
			return m.loadPage(s, s.nextToken)
		}
	case key.Matches(msg, k.HidePreview):
		if m.tier() != tierSingle {
			m.previewHidden = !m.previewHidden
			m.focus = focusList
			return m.ensureRendered()
		}
	case key.Matches(msg, k.Maximise):
		if m.tier() != tierSingle {
			m.maximised = !m.maximised
			return m.ensureRendered()
		}
	case key.Matches(msg, k.CopyShort):
		if r := m.selectedRecord(); r != nil && r.GetShortId() != "" {
			return tea.Batch(tea.SetClipboard(r.GetShortId()),
				m.showToast("copied short_id "+consoleText(r.GetShortId(), false)))
		}
	case key.Matches(msg, k.CopyID):
		if r := m.selectedRecord(); r != nil {
			return tea.Batch(tea.SetClipboard(r.GetId()),
				m.showToast("copied id "+consoleText(r.GetId(), false)))
		}
	}
	return nil
}

// back is Esc's tiered step: the narrowing first, then focus, then the
// screen stack.
func (m *consoleModel) back() tea.Cmd {
	s := m.active()
	switch {
	case s.narrow != "":
		m.setNarrow("")
		return m.selectionChanged()
	case m.focus == focusPreview:
		m.focus = focusList
	case len(m.stack) > 0:
		return m.pop()
	}
	return nil
}

func (m *consoleModel) searchKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.input.Blur()
		return nil
	case "enter":
		m.input.Blur()
		q := strings.TrimSpace(m.input.Value())
		if isIDShaped(q) {
			return m.push(screen{kind: screenDetail, id: q})
		}
		s := &m.tabs[tabMemories]
		s.query, s.narrow, s.selectedID = q, "", ""
		return m.load(s)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m *consoleModel) narrowKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.narrowIn.Blur()
		m.setNarrow("")
		return m.selectionChanged()
	case "enter":
		m.narrowIn.Blur()
		return nil
	}
	var cmd tea.Cmd
	m.narrowIn, cmd = m.narrowIn.Update(msg)
	m.setNarrow(m.narrowIn.Value())
	return tea.Batch(cmd, m.selectionChanged())
}

func (m *consoleModel) enter(s *screen) tea.Cmd {
	switch s.kind {
	case screenDetail:
		return nil
	case screenTags:
		if len(m.chosenTags) == 0 {
			if t := s.selectedTag(); t != "" {
				m.toggleTag(t)
			}
		}
		return m.applyTagFilter()
	case screenRelated:
		if m.focus == focusList {
			if r := m.selectedRecord(); r != nil {
				return m.push(screen{kind: screenRelated, id: r.GetId(), anchor: r})
			}
			return nil
		}
	}
	if r := m.selectedRecord(); r != nil {
		return m.push(screen{kind: screenDetail, id: r.GetId(), anchor: r})
	}
	return nil
}

func (m *consoleModel) toggleTag(t string) {
	if i := slices.Index(m.chosenTags, t); i >= 0 {
		m.chosenTags = slices.Delete(m.chosenTags, i, i+1)
		return
	}
	m.chosenTags = append(m.chosenTags, t)
}

// applyTagFilter returns to the memories list, filtered by every chosen
// tag.
func (m *consoleModel) applyTagFilter() tea.Cmd {
	m.reloadFiltered()
	m.tab, m.focus = tabMemories, focusList
	return m.load(&m.tabs[tabMemories])
}

// reloadFiltered marks every tab the filters feed as needing a reload.
func (m *consoleModel) reloadFiltered() {
	m.tabs[tabMemories].loaded = false
	m.tabs[tabTags].loaded = false
	m.tabs[tabScheduled].loaded = false
	m.stack = nil
}

func (m *consoleModel) openTab(t consoleTab) tea.Cmd {
	m.stack, m.tab, m.focus = nil, t, focusList
	if s := &m.tabs[t]; !s.loaded && !s.loading {
		return m.load(s)
	}
	return m.selectionChanged()
}

func (m *consoleModel) push(s screen) tea.Cmd {
	m.stack = append(m.stack, s)
	m.focus = focusList
	top := &m.stack[len(m.stack)-1]
	if top.kind == screenDetail {
		m.preview.offset = 0
		return m.showRecord(top.id)
	}
	return m.load(top)
}

func (m *consoleModel) pop() tea.Cmd {
	m.stack = m.stack[:len(m.stack)-1]
	m.focus = focusList
	return m.selectionChanged()
}

// move shifts the selection and lets the preview and paging follow it.
func (m *consoleModel) move(s *screen, delta int) tea.Cmd {
	if s.kind == screenDetail {
		m.scrollPreview(delta)
		return nil
	}
	before := s.selectedID
	s.setCursor(s.cursor + delta)
	cmds := []tea.Cmd{m.autoPage(s)}
	if s.selectedID != before {
		cmds = append(cmds, m.selectionChanged())
	}
	return tea.Batch(cmds...)
}

// autoPage requests the next page once the selection is within one
// screen of the last loaded row.
func (m *consoleModel) autoPage(s *screen) tea.Cmd {
	if s.nextToken == "" || s.loading || s.narrow != "" {
		return nil
	}
	if s.cursor < s.count()-m.listHeight() {
		return nil
	}
	return m.loadPage(s, s.nextToken)
}

func (m *consoleModel) applyResult(r rpcResult) tea.Cmd {
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
		return nil
	}
	s.loading = false
	if r.err != nil {
		s.pageErr = s.loaded
		// The server's message goes out unchanged, field=/hint= envelope
		// included, and the session goes on.
		m.status, m.statusErr = "error: "+consoleText(r.err.Error(), false), true
		return nil
	}
	s.loaded, s.pageErr = true, false
	r.apply(s)
	s.refilter()
	if s != m.active() {
		return nil
	}
	return m.selectionChanged()
}

// load starts a screen's first page.
func (m *consoleModel) load(s *screen) tea.Cmd {
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
	tags := slices.Clone(m.chosenTags)
	switch s.kind {
	case screenMemories:
		if s.query == "" {
			req := &engramv1.ListMemoriesRequest{
				Scope: scope, CrossSpine: cross, Limit: consolePageSize, Tags: tags,
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
					s.searched, s.nextToken = false, r.GetNextPageToken()
					if token == "" {
						// Coverage describes the listing; a later page
						// adds rows and its own hidden counts.
						s.rows, s.hidden = nil, nil
						s.coverage(cross, r.GetSearchedScopes(), r.GetScopesTruncated(), r.GetScopesUnknown())
					}
					s.rows = append(s.rows, r.GetMemories()...)
					s.hidden = addHidden(s.hidden, r.GetRecallGateHidden())
				}, nil
			}
		}
		req := &engramv1.SearchMemoriesRequest{
			Query: s.query, Scope: scope, CrossSpine: cross, K: consolePageSize, Tags: tags,
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
				s.coverage(cross, r.GetSearchedScopes(), r.GetScopesTruncated(), r.GetScopesUnknown())
				s.hidden = r.GetRecallGateHidden()
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
			return func(s *screen) { s.tags, s.moreTags = r.GetTags(), r.GetMore() }, nil
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
				s.coverage(true, r.GetSearchedScopes(), r.GetScopesTruncated(), r.GetScopesUnknown())
				s.advisory = r.GetAdvisory()
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
					s.coverage(cross, r.GetSearchedScopes(), r.GetScopesTruncated(), r.GetScopesUnknown())
				}
				s.rows = append(s.rows, r.GetMemories()...)
				s.nextToken = r.GetNextPageToken()
			}, nil
		}
	default: // screenRelated
		req := &engramv1.RelatedMemoriesRequest{Id: s.id}
		return func(ctx context.Context) (func(*screen), error) {
			resp, err := c.RelatedMemories(ctx, connect.NewRequest(req))
			if err != nil {
				return nil, err
			}
			r := resp.Msg
			return func(s *screen) { s.applyRelated(r) }, nil
		}
	}
}

// addHidden sums the recall gate's hidden counts across loaded pages.
func addHidden(a, b *engramv1.RecallGateHidden) *engramv1.RecallGateHidden {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return &engramv1.RecallGateHidden{
		Total: a.GetTotal() + b.GetTotal(), Archived: a.GetArchived() + b.GetArchived(),
		Superseded: a.GetSuperseded() + b.GetSuperseded(), Expired: a.GetExpired() + b.GetExpired(),
		Scheduled: a.GetScheduled() + b.GetScheduled(),
	}
}
