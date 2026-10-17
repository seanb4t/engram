// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"connectrpc.com/connect"
	"github.com/charmbracelet/colorprofile"
	"github.com/sahilm/fuzzy"
	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/config"
)

// --- settings and themes ---

func TestConsoleSettingsResolution(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	prefsPath := filepath.Join(dir, "engram", "console.json")
	noEnv := func(string) string { return "" }

	s := loadConsoleSettings(config.ClientConfig{}, noEnv)
	if s.theme != "engram" || s.mouse || s.note != "" || s.prefsPath != prefsPath {
		t.Errorf("no file: %+v, want the defaults, no note and path %s", s, prefsPath)
	}
	if _, err := os.Stat(prefsPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("loading created %s (err=%v), want no file", prefsPath, err)
	}

	on := true
	if err := saveConsolePrefs(prefsPath, consolePrefs{Theme: "catppuccin", Mouse: &on}); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(prefsPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("preferences file mode = %v (err=%v), want 0600", info.Mode().Perm(), err)
	}
	s = loadConsoleSettings(config.ClientConfig{}, noEnv)
	if s.theme != "catppuccin" || !s.mouse || s.themeLocked {
		t.Errorf("saved preference: %+v, want catppuccin, mouse on, not locked", s)
	}
	s = loadConsoleSettings(config.ClientConfig{TUITheme: "tokyo-night", TUIMouse: "false"}, noEnv)
	if s.theme != "tokyo-night" || s.mouse || !s.themeLocked {
		t.Errorf("flag or environment: %+v, want tokyo-night, mouse off, locked", s)
	}

	if err := os.WriteFile(prefsPath, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s = loadConsoleSettings(config.ClientConfig{}, noEnv)
	if s.theme != "engram" || s.mouse || !strings.Contains(s.note, "ignored invalid console preferences") {
		t.Errorf("invalid file: %+v, want the defaults with a note", s)
	}
	h := newSizedConsoleHarness(t, &stubEngramService{listFn: listOf()}, 240, 40, s)
	h.wantView("ignored invalid console preferences")

	s = loadConsoleSettings(config.ClientConfig{}, func(k string) string {
		if k == "NO_COLOR" {
			return "1"
		}
		return ""
	})
	if !s.noColor {
		t.Error("NO_COLOR set, want noColor")
	}
}

func TestConsoleColorProfileHonoursNoColor(t *testing.T) {
	if p, ok := consoleColorProfile(func(string) string { return "1" }); !ok || p != colorprofile.Ascii {
		t.Errorf("NO_COLOR=1: profile %v, %v; want Ascii, true", p, ok)
	}
	if _, ok := consoleColorProfile(func(string) string { return "" }); ok {
		t.Error("NO_COLOR unset: a profile was forced")
	}
}

func TestConsoleThemesSetEveryRole(t *testing.T) {
	names := make([]string, 0, len(consoleThemes))
	for _, th := range consoleThemes {
		names = append(names, th.name)
		for variant, roles := range map[string]themeRoles{"dark": th.dark, "light": th.light} {
			for i, c := range roles.all() {
				if c == nil {
					t.Errorf("theme %s %s: role %d is unset", th.name, variant, i)
				}
			}
		}
	}
	if !slices.Equal(names, config.TUIThemes) {
		t.Errorf("themes = %v, want config.TUIThemes %v", names, config.TUIThemes)
	}
}

func TestTUIBackgroundPicksVariant(t *testing.T) {
	h := newConsoleHarness(t, &stubEngramService{listFn: listOf()})
	if h.m.st.roles.fg != engramTheme.dark.fg {
		t.Fatal("before the background is known, want the dark variant")
	}
	h.send(tea.BackgroundColorMsg{Color: color.White})
	if h.m.dark || h.m.st.roles.fg != engramTheme.light.fg {
		t.Error("a light background did not switch to the light variant")
	}
}

func TestTUINoColorDrawsNoColour(t *testing.T) {
	both := testMemory("aaaaaaaaa1", "archived and superseded")
	both.ArchivedAt = timestamppb.New(consoleNow)
	both.SupersededBy = stringPtr("id-x")
	h := newSizedConsoleHarness(t, &stubEngramService{listFn: listOf(both)}, 200, 40,
		consoleSettings{theme: "engram", noColor: true})
	raw := h.m.View().Content
	if strings.Contains(raw, "[38;") || strings.Contains(raw, "[48;") {
		t.Errorf("NO_COLOR view carries colour sequences:\n%q", raw)
	}
	h.wantView("archived superseded")
}

// --- layout, rows and the list ---

func TestTUILayoutTiers(t *testing.T) {
	rows := []*engramv1.Memory{testMemory("aaaaaaaaa1", "first"), testMemory("bbbbbbbbb2", "second")}
	for _, c := range []struct {
		w, h int
		want layoutTier
	}{{160, 40, tierSide}, {90, 45, tierStacked}, {70, 24, tierSingle}} {
		h := newSizedConsoleHarness(t, &stubEngramService{listFn: listOf(rows...), getFn: getFrom(nil, rows...)},
			c.w, c.h, consoleSettings{theme: "engram"})
		l := h.m.layout()
		if l.tier != c.want {
			t.Errorf("%dx%d: tier %v, want %v", c.w, c.h, l.tier, c.want)
		}
		v := h.view()
		switch c.want {
		case tierSide:
			if !l.showPreview || !strings.Contains(h.line("first"), "│") {
				t.Errorf("%dx%d: want the list and preview side by side:\n%s", c.w, c.h, v)
			}
		case tierStacked:
			if l.preview.y <= l.list.y || !strings.Contains(v, strings.Repeat("─", c.w)) {
				t.Errorf("%dx%d: want the preview below the list:\n%s", c.w, c.h, v)
			}
		case tierSingle:
			if l.showPreview {
				t.Errorf("%dx%d: want the list only", c.w, c.h)
			}
		}
	}
}

func TestTUISmallTerminalOpensRecordFullScreen(t *testing.T) {
	rows := []*engramv1.Memory{testMemory("aaaaaaaaa1", "first"), testMemory("bbbbbbbbb2", "second")}
	h := newSizedConsoleHarness(t, &stubEngramService{listFn: listOf(rows...), getFn: getFrom(nil, rows...)},
		70, 24, consoleSettings{theme: "engram"})
	h.press("j", "enter")
	h.wantView("second in full", "all shown", "esc back")
	h.noView("first", "0/0")
	h.press("esc")
	if got := h.m.active().selectedID; got != "id-bbbbbbbbb2" {
		t.Errorf("after Esc the selection is %q, want the same row", got)
	}
}

func TestTUIResizeKeepsSelection(t *testing.T) {
	rows := make([]*engramv1.Memory, 0, 8)
	for i := range 8 {
		rows = append(rows, testMemory(fmt.Sprintf("aaaaaaaa%02d", i), fmt.Sprintf("row %d", i)))
	}
	h := newSizedConsoleHarness(t, &stubEngramService{listFn: listOf(rows...), getFn: getFrom(nil, rows...)},
		160, 40, consoleSettings{theme: "engram"})
	h.press("j", "j", "j", "j")
	h.send(tea.WindowSizeMsg{Width: 70, Height: 24})
	if got := h.m.active().selectedID; got != "id-aaaaaaaa04" {
		t.Errorf("after resizing the selection is %q, want the fifth row", got)
	}
}

func TestTUIHideAndMaximisePreview(t *testing.T) {
	rows := []*engramv1.Memory{testMemory("aaaaaaaaa1", "first")}
	h := newConsoleHarness(t, &stubEngramService{listFn: listOf(rows...), getFn: getFrom(nil, rows...)})
	h.press("p")
	if l := h.m.layout(); l.showPreview || l.list.w != 200 {
		t.Errorf("after p: %+v, want the list at full width", l)
	}
	h.press("p", "tab", "z")
	if l := h.m.layout(); l.showList || l.preview.w != 200 {
		t.Errorf("after tab z: %+v, want the preview maximised", l)
	}
	h.press("z")
	if l := h.m.layout(); !l.showList || !l.showPreview {
		t.Errorf("after z again: %+v, want both panes", l)
	}
}

func TestTUIRowContent(t *testing.T) {
	r := testMemory("aaaaaaaaa1", "a decision with five tags")
	r.Tags = []string{"tag-aaaaaa", "tag-bbbbbb", "tag-cccccc", "tag-dddddd", "tag-eeeeee"}
	h := newConsoleHarness(t, &stubEngramService{listFn: listOf(r), getFn: getFrom(nil, r)})
	row := h.line("a decision with five tags")
	for _, want := range []string{"decision", "#tag-aaaaaa #tag-bbbbbb +3", "2d"} {
		if !strings.Contains(row, want) {
			t.Errorf("row %q does not contain %q", row, want)
		}
	}
	if strings.Contains(row, "tag-cccccc") {
		t.Errorf("row %q shows a third tag, want +3", row)
	}
}

func TestTUIPastStatesAreMuted(t *testing.T) {
	archived := testMemory("aaaaaaaaa1", "archived one")
	archived.ArchivedAt = timestamppb.New(consoleNow)
	superseded := testMemory("bbbbbbbbb2", "superseded one")
	superseded.SupersededBy = stringPtr("id-x")
	expired := testMemory("ccccccccc3", "expired one")
	expired.NotAfter = timestamppb.New(consoleNow.Add(-time.Hour))
	scheduled := testMemory("ddddddddd4", "scheduled one")
	scheduled.NotBefore = timestamppb.New(consoleNow.Add(time.Hour))
	lead := testMemory("eeeeeeeee5", "selected first")
	h := newConsoleHarness(t, &stubEngramService{listFn: listOf(lead, archived, superseded, expired, scheduled)})
	muted := sgrFor(engramTheme.dark.muted) + "decision"
	raw := strings.Split(h.m.View().Content, "\n")
	find := func(sub string) string {
		for _, l := range raw {
			if strings.Contains(ansiCSI.ReplaceAllString(l, ""), sub) {
				return l
			}
		}
		return ""
	}
	for _, sub := range []string{"archived one", "superseded one", "expired one"} {
		if !strings.Contains(find(sub), muted) {
			t.Errorf("row %q is not muted", sub)
		}
	}
	if strings.Contains(find("scheduled one"), muted) {
		t.Error("the scheduled row is muted, want it drawn normally")
	}
	for sub, word := range map[string]string{"archived one": "archived", "superseded one": "superseded",
		"expired one": "expired", "scheduled one": "scheduled"} {
		if !strings.Contains(h.line(sub), word) {
			t.Errorf("row %q lacks its state word %q", sub, word)
		}
	}
}

// sgrFor is the foreground sequence lipgloss writes for a truecolour value.
func sgrFor(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r>>8, g>>8, b>>8)
}

func TestTUISelectionStaysMarkedWithoutFocus(t *testing.T) {
	rows := []*engramv1.Memory{testMemory("aaaaaaaaa1", "first"), testMemory("bbbbbbbbb2", "second")}
	h := newConsoleHarness(t, &stubEngramService{listFn: listOf(rows...), getFn: getFrom(nil, rows...)})
	h.press("j", "tab")
	if h.m.focus != focusPreview {
		t.Fatal("tab did not move focus to the preview")
	}
	if !strings.HasPrefix(h.line("decision   second"), "▌") {
		t.Errorf("selected row %q is not marked while the preview has focus", h.line("decision   second"))
	}
}

func TestTUIResultsHeader(t *testing.T) {
	hits := make([]*engramv1.Memory, 0, 41)
	for i := range 41 {
		hits = append(hits, testMemory(fmt.Sprintf("aaaaaaaa%02d", i), fmt.Sprintf("hit %d", i)))
	}
	nine := []string{"s1", "s2", "s3", "s4", "s5", "s6", "s7", "s8", "s9"}
	svc := &stubEngramService{
		listFn: listOf(),
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{Memories: hits, SearchedScopes: nine,
				RecallGateHidden: &engramv1.RecallGateHidden{Total: 3, Archived: 3}}, nil
		},
	}
	h := newConsoleHarness(t, svc)
	h.press("/")
	h.typeText("helm")
	h.press("enter")
	h.wantView(`41 hits across 9 scopes for "helm" · ranked by cosine similarity · 3 hidden by the recall gate`)
}

func TestTUIEmptyResultNamesFixes(t *testing.T) {
	svc := &stubEngramService{
		listFn: listOf(),
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{RecallGateHidden: &engramv1.RecallGateHidden{Total: 2, Archived: 2}}, nil
		},
	}
	h := newConsoleHarness(t, svc, "repo:example/one")
	h.m.crossSpine, h.m.scope = false, "repo:example/one"
	h.press("/")
	h.typeText("nothing")
	h.press("enter")
	h.wantView(`No memories match "nothing" in repo:example/one · 2 hidden by the recall gate`,
		"search every readable scope", "include archived records (2 hidden)")
}

func TestTUIIncludeArchivedReloads(t *testing.T) {
	var lists recorder[engramv1.ListMemoriesRequest]
	archived := testMemory("aaaaaaaaa1", "an archived record")
	archived.ArchivedAt = timestamppb.New(consoleNow)
	svc := &stubEngramService{
		listFn: func(_ context.Context, req *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			lists.add(req)
			if req.GetIncludeArchived() {
				return &engramv1.ListMemoriesResponse{Memories: []*engramv1.Memory{archived}}, nil
			}
			return &engramv1.ListMemoriesResponse{}, nil
		},
	}
	h := newConsoleHarness(t, svc)
	before := len(lists.all())
	h.press("f", "space", "esc")
	if got := len(lists.all()) - before; got != 1 {
		t.Errorf("turning on include archived sent %d list requests, want 1", got)
	}
	if req := lists.last(t); !req.GetIncludeArchived() || req.GetIncludeSuperseded() || req.GetIncludeScheduled() {
		t.Errorf("request = %v, want only include_archived", req)
	}
	h.wantView("include: archived", "an archived record")
	if !strings.Contains(h.line("an archived record"), "archived") {
		t.Error("the archived record is not marked archived")
	}
}

func TestTUIPagingAndPosition(t *testing.T) {
	var lists recorder[engramv1.ListMemoriesRequest]
	page := func(prefix string) []*engramv1.Memory {
		out := make([]*engramv1.Memory, 0, 50)
		for i := range 50 {
			out = append(out, testMemory(fmt.Sprintf("%s%07d", prefix, i), fmt.Sprintf("%s row %d", prefix, i)))
		}
		return out
	}
	svc := &stubEngramService{
		listFn: func(_ context.Context, req *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			lists.add(req)
			if req.GetPageToken() == "p2" {
				return &engramv1.ListMemoriesResponse{Memories: page("bbb")}, nil
			}
			return &engramv1.ListMemoriesResponse{Memories: page("aaa"), NextPageToken: "p2"}, nil
		},
		getFn: getFrom(nil, append(page("aaa"), page("bbb")...)...),
	}
	h := newConsoleHarness(t, svc)
	if first := lists.last(t); !first.GetCursorMode() || first.GetLimit() == 0 || first.GetLimit() >= 1000 {
		t.Errorf("first page = %v, want cursor_mode and a page size well under 1000", first)
	}
	for range 11 {
		h.press("j")
	}
	if len(lists.all()) != 1 {
		t.Fatalf("moving to row 12 sent %d requests, want only the first page", len(lists.all()))
	}
	h.wantView(" 12/50+")

	// Within one screen of the end: exactly one next-page request, even
	// with a second move while it is in flight.
	listH := h.m.listHeight()
	for h.m.active().cursor < 50-listH-1 {
		h.press("j")
	}
	pending := h.update(consoleKey("j"))
	if pending == nil {
		t.Fatal("reaching the end of the loaded rows sent no request")
	}
	if more := h.update(consoleKey("j")); more != nil {
		for _, msg := range runConsoleCmd(more) {
			if _, ok := msg.(rpcResult); ok {
				t.Error("a second move sent another page request while one was in flight")
			}
		}
	}
	h.run(pending)
	if req := lists.last(t); req.GetPageToken() != "p2" || req.GetCursorMode() {
		t.Errorf("next page = %v, want page_token p2 without cursor_mode", req)
	}
	if len(lists.all()) != 2 {
		t.Errorf("sent %d list requests, want 2", len(lists.all()))
	}
	selected := h.m.active().selectedID
	if got := h.m.active(); got.count() != 100 || got.selectedID != selected {
		t.Errorf("after the page: %d rows, selection %q, want 100 rows and the same selection", got.count(), got.selectedID)
	}
	h.press("end")
	h.wantView("100/100", "end · 100 memories")
}

func TestTUISearchPositionShowsTopK(t *testing.T) {
	hits := make([]*engramv1.Memory, 0, consolePageSize)
	for i := range consolePageSize {
		hits = append(hits, testMemory(fmt.Sprintf("aaaaaaaa%02d", i), fmt.Sprintf("hit %d", i)))
	}
	svc := &stubEngramService{
		listFn: listOf(),
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{Memories: hits}, nil
		},
		getFn: getFrom(nil, hits...),
	}
	h := newConsoleHarness(t, svc)
	h.press("/")
	h.typeText("q")
	h.press("enter")
	for range 11 {
		h.press("j")
	}
	h.wantView("12/50 · top 50")
}

func TestTUISearchHint(t *testing.T) {
	var searches recorder[engramv1.SearchMemoriesRequest]
	svc := &stubEngramService{
		listFn: listOf(),
		searchFn: func(_ context.Context, req *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			searches.add(req)
			return &engramv1.SearchMemoriesResponse{}, nil
		},
		getFn: getFrom(nil),
	}
	h := newConsoleHarness(t, svc)
	h.press("/")
	h.wantView("enter: list")
	h.typeText("helm upgrade")
	h.wantView("enter: search · 1 embedding")
	if n := len(searches.all()); n != 0 {
		t.Fatalf("typing sent %d searches, want 0", n)
	}
	h.press("esc", "/")
	h.typeText("abcdefghjk")
	h.wantView("enter: open")
	h.press("enter")
	if n := len(searches.all()); n != 0 {
		t.Errorf("a short_id sent %d searches, want 0", n)
	}
	h.wantView(consoleNotFound)
}

func TestTUISearchRunsOnEnter(t *testing.T) {
	var searches recorder[engramv1.SearchMemoriesRequest]
	svc := &stubEngramService{
		listFn: listOf(),
		searchFn: func(_ context.Context, req *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			searches.add(req)
			return &engramv1.SearchMemoriesResponse{Memories: []*engramv1.Memory{testMemory("ccccccccc3", "a hit")}}, nil
		},
	}
	h := newConsoleHarness(t, svc, "repo:example/one")
	h.press("/")
	h.typeText("deploy notes")
	h.press("enter")
	all := searches.all()
	if len(all) != 1 || all[0].GetQuery() != "deploy notes" || !all[0].GetCrossSpine() {
		t.Fatalf("searches = %v, want one for \"deploy notes\" across every readable scope", all)
	}
}

// TestTUINewSearchStartsAtTheTop: a new query selects its best hit, not
// the row that happened to be selected before.
func TestTUINewSearchStartsAtTheTop(t *testing.T) {
	rows := []*engramv1.Memory{testMemory("aaaaaaaaa1", "best hit"), testMemory("bbbbbbbbb2", "second"),
		testMemory("ccccccccc3", "third")}
	svc := &stubEngramService{
		listFn: listOf(rows...),
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{Memories: rows}, nil
		},
		getFn: getFrom(nil, rows...),
	}
	h := newConsoleHarness(t, svc)
	h.press("j", "j", "/")
	h.typeText("q")
	h.press("enter")
	if got := h.m.active().selectedID; got != "id-aaaaaaaaa1" {
		t.Errorf("after a new search the selection is %q, want the best hit", got)
	}
}

func TestTUILocalNarrowing(t *testing.T) {
	var lists recorder[engramv1.ListMemoriesRequest]
	rows := make([]*engramv1.Memory, 0, 50)
	for i := range 50 {
		r := testMemory(fmt.Sprintf("zz%08d", i), "alpha beta gamma")
		r.Category, r.Tags = "rule", []string{"delta"}
		if i%7 == 0 {
			r.Tags = []string{"oci"}
		}
		rows = append(rows, r)
	}
	svc := &stubEngramService{listFn: func(_ context.Context, req *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
		lists.add(req)
		return &engramv1.ListMemoriesResponse{Memories: rows}, nil
	}}
	h := newConsoleHarness(t, svc)
	before := len(lists.all())
	h.press("=")
	h.typeText("oci")
	h.wantView(`8 of 50 loaded match "oci"`)
	if h.m.active().count() != 8 {
		t.Errorf("narrowed rows = %d, want 8", h.m.active().count())
	}
	if len(lists.all()) != before {
		t.Error("narrowing sent a request")
	}
	h.press("esc")
	if h.m.active().count() != 50 {
		t.Errorf("after Esc %d rows show, want all 50", h.m.active().count())
	}
}

// TestUnderlineMatchesKeepsNonASCII: underlining a fuzzy match keeps a
// non-ASCII title intact and underlines the matched characters.
func TestUnderlineMatchesKeepsNonASCII(t *testing.T) {
	title := "Scope: repo:example/café"
	matches := fuzzy.Find("café", []string{title})
	if len(matches) != 1 {
		t.Fatalf("fuzzy found %d matches, want 1", len(matches))
	}
	got := underlineMatches(title, matches[0].MatchedIndexes)
	if plain := ansiCSI.ReplaceAllString(got, ""); plain != title {
		t.Errorf("underlined title reads %q, want %q", plain, title)
	}
	if !strings.Contains(got, "é") || got == title {
		t.Errorf("underlined title %q lost the é or underlined nothing", got)
	}
}

// TestTUIPastedNarrowingMovesThePreview: a paste into the narrowing box
// that moves the selection makes the preview follow it.
func TestTUIPastedNarrowingMovesThePreview(t *testing.T) {
	// zulu is three rows down, out of the neighbour prefetch, so only a
	// selection change can load it.
	rows := []*engramv1.Memory{testMemory("aaaaaaaaa1", "alpha record"), testMemory("ccccccccc3", "beta record"),
		testMemory("ddddddddd4", "gamma record"), testMemory("bbbbbbbbb2", "zulu record")}
	h := newConsoleHarness(t, &stubEngramService{listFn: listOf(rows...), getFn: getFrom(nil, rows...)})
	h.fire()
	h.wantView("alpha record in full")
	h.press("=")
	h.send(tea.PasteMsg{Content: "zulu"})
	h.fire()
	if got := h.m.active().selectedID; got != "id-bbbbbbbbb2" {
		t.Fatalf("after pasting the selection is %q, want the zulu record", got)
	}
	h.wantView("zulu record in full")
	h.noView("alpha record in full")
}

// --- preview ---

func TestTUIPreviewFollowsAfterSettling(t *testing.T) {
	var gets recorder[engramv1.GetMemoryRequest]
	rows := make([]*engramv1.Memory, 0, 6)
	for i := range 6 {
		rows = append(rows, testMemory(fmt.Sprintf("aaaaaaaa%02d", i), fmt.Sprintf("row %d", i)))
	}
	h := newConsoleHarness(t, &stubEngramService{listFn: listOf(rows...), getFn: getFrom(&gets, rows...)})
	h.fire()
	h.wantView("row 0 in full")
	before := len(gets.all())
	h.press("j", "j", "j")
	if len(gets.all()) != before {
		t.Errorf("moving sent %v before the selection settled", getIDs(&gets)[before:])
	}
	h.wantView("row 3")
	h.fire()
	if ids := getIDs(&gets)[before:]; len(ids) == 0 || ids[0] != "id-aaaaaaaa03" {
		t.Errorf("after settling fetched %v, want the final row first", ids)
	}
	if slices.Contains(getIDs(&gets)[before:], "id-aaaaaaaa00") {
		t.Error("a cached record was fetched again")
	}
	h.wantView("row 3 in full")
	cached := len(gets.all())
	h.press("k", "k", "k")
	h.fire()
	if len(gets.all()) != cached {
		t.Errorf("moving back over cached rows fetched %v", getIDs(&gets)[cached:])
	}
}

func TestTUIPreviewLoadingTextIsDelayed(t *testing.T) {
	rows := []*engramv1.Memory{testMemory("aaaaaaaaa1", "first"), testMemory("bbbbbbbbb2", "neighbour"),
		testMemory("ccccccccc3", "second")}
	h := newConsoleHarness(t, &stubEngramService{listFn: listOf(rows...), getFn: getFrom(nil, rows...)})
	h.fire()
	h.press("j", "j")
	settle := h.ticks
	h.ticks = nil
	if len(settle) != 1 {
		t.Fatalf("queued timers = %v, want one settle timer", settle)
	}
	fetch := h.update(settle[0])
	h.noView("loading…")
	loading := h.ticks
	h.ticks = nil
	h.send(loading[0])
	h.wantView("loading…")
	h.run(fetch)
	h.noView("loading…")
	h.wantView("second in full")
}

func TestTUILateResultIsNeverDrawn(t *testing.T) {
	rows := []*engramv1.Memory{testMemory("aaaaaaaaa1", "first"), testMemory("bbbbbbbbb2", "neighbour"),
		testMemory("ccccccccc3", "second"), testMemory("ddddddddd4", "third")}
	rows[2].Content = "SECOND BODY"
	h := newConsoleHarness(t, &stubEngramService{listFn: listOf(rows...), getFn: getFrom(nil, rows...)})
	h.fire()
	h.press("j", "j")
	fetchSecond := h.update(h.ticks[0])
	h.ticks = nil
	h.press("j")
	for _, msg := range runConsoleCmd(fetchSecond) {
		h.send(msg)
	}
	h.noView("SECOND BODY")
}

func TestTUIPreviewRendersMarkdownSafely(t *testing.T) {
	r := testMemory("aaaaaaaaa1", "markdown")
	r.Content = "# Heading\n\nfirst line\nsecond line\n\n```go\nfmt.Println(1)\n```\n\nbad \x1b]52;c;cHduZWQ=\x07 \u009b2J end"
	h := newConsoleHarness(t, &stubEngramService{listFn: listOf(r), getFn: getFrom(nil, r)})
	h.fire()
	h.wantView("Heading", "fmt.Println(1)", "end")
	if h.line("first line") == h.line("second line") {
		t.Error("the content's line break was lost")
	}
	raw := h.m.View().Content
	for _, bad := range []string{"\x1b]", "\x07", "\u009b"} {
		if strings.Contains(raw, bad) {
			t.Errorf("the view carries %q from the record content", bad)
		}
	}
}

func TestTUIPreviewScrollAndFocusKeys(t *testing.T) {
	r := testMemory("aaaaaaaaa1", "long")
	lines := make([]string, 0, 120)
	for i := range 120 {
		lines = append(lines, fmt.Sprintf("content line %d", i))
	}
	r.Content = strings.Join(lines, "\n")
	h := newConsoleHarness(t, &stubEngramService{listFn: listOf(r), getFn: getFrom(nil, r)})
	h.fire()
	h.wantView("enter open")
	h.press("J")
	if h.m.preview.offset != 1 {
		t.Errorf("J scrolled to %d, want 1", h.m.preview.offset)
	}
	h.press("ctrl+d")
	if h.m.preview.offset <= 1 {
		t.Errorf("ctrl+d scrolled to %d, want half a page further", h.m.preview.offset)
	}
	h.press("tab")
	h.wantView("j/k scroll", "ctrl+d")
	h.noView("enter open")
	at := h.m.preview.offset
	h.press("j")
	if h.m.preview.offset != at+1 {
		t.Errorf("j in the preview scrolled to %d, want %d", h.m.preview.offset, at+1)
	}
}

func TestTUIPreviewDetailRequirement(t *testing.T) {
	superseded := testMemory("aaaaaaaaa1", "an old decision")
	superseded.SupersededBy = stringPtr("id-successor")
	archived := testMemory("bbbbbbbbb2", "archived record")
	archived.ArchivedAt = timestamppb.New(consoleNow)
	expired := testMemory("ccccccccc3", "expired record")
	expired.NotAfter = timestamppb.New(consoleNow.Add(-time.Hour))
	scheduled := testMemory("ddddddddd4", "scheduled record")
	scheduled.NotBefore = timestamppb.New(consoleNow.Add(time.Hour))
	shared := testMemory("eeeeeeeee5", "a shared fact")
	shared.Visibility, shared.Owner = "shared", "other@example.com"
	rows := []*engramv1.Memory{superseded, archived, expired, scheduled, shared}
	h := newConsoleHarness(t, &stubEngramService{listFn: listOf(rows...), getFn: getFrom(nil, rows...)})
	h.fire()
	h.wantView("superseded by id-successor", "state: superseded")
	for _, want := range []string{"state: archived", "state: expired", "state: scheduled"} {
		h.press("j")
		h.fire()
		h.wantView(want)
	}
	h.press("j")
	h.fire()
	h.wantView("shared · owner other@example.com", "a shared fact in full")
	h.press("/")
	h.typeText("zzzzzzzzz9")
	h.press("enter")
	h.wantView(consoleNotFound)
}

// --- secondary views ---

func TestTUIRelatedLanes(t *testing.T) {
	var related recorder[engramv1.RelatedMemoriesRequest]
	anchor := testMemory("jjjjjjjjj1", "the anchor")
	edge := func(t engramv1.EdgeType) *engramv1.RelatedEdge { return &engramv1.RelatedEdge{Type: t} }
	rel := make([]*engramv1.RelatedMemory, 0, 7)
	for i := range 2 {
		rel = append(rel, &engramv1.RelatedMemory{Memory: testMemory(fmt.Sprintf("sssssssss%d", i), "chain member"),
			Edges: []*engramv1.RelatedEdge{edge(engramv1.EdgeType_EDGE_TYPE_SUPERSESSION)}})
	}
	for i := range 5 {
		rel = append(rel, &engramv1.RelatedMemory{Memory: testMemory(fmt.Sprintf("ttttttttt%d", i), "tag neighbour"),
			Edges: []*engramv1.RelatedEdge{edge(engramv1.EdgeType_EDGE_TYPE_TAG)}})
	}
	svc := &stubEngramService{
		listFn: listOf(anchor),
		getFn:  getFrom(nil, anchor),
		relatedFn: func(_ context.Context, req *engramv1.RelatedMemoriesRequest) (*engramv1.RelatedMemoriesResponse, error) {
			related.add(req)
			if req.GetId() == anchor.GetId() {
				return &engramv1.RelatedMemoriesResponse{Anchor: anchor, Related: rel}, nil
			}
			return &engramv1.RelatedMemoriesResponse{}, nil
		},
	}
	h := newConsoleHarness(t, svc)
	h.press("r")
	if got := related.last(t).GetId(); got != anchor.GetId() {
		t.Errorf("RelatedMemories id = %q, want %q", got, anchor.GetId())
	}
	v := h.view()
	s, tg := strings.Index(v, "S supersession (2)"), strings.Index(v, "T shared tag (5)")
	if s < 0 || tg < 0 || s > tg {
		t.Errorf("want S supersession (2) before T shared tag (5):\n%s", v)
	}
	h.press("enter")
	if got := related.last(t).GetId(); got != "id-sssssssss0" {
		t.Errorf("Enter walked to %q, want the selected record", got)
	}
	h.press("[")
	if h.m.active().id != anchor.GetId() {
		t.Error("[ did not walk back along the trail")
	}
}

func TestTUITagsChooseAndFilter(t *testing.T) {
	var lists recorder[engramv1.ListMemoriesRequest]
	svc := &stubEngramService{
		listFn: func(_ context.Context, req *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			lists.add(req)
			return &engramv1.ListMemoriesResponse{}, nil
		},
		listTagsFn: func(context.Context, *engramv1.ListTagsRequest) (*engramv1.ListTagsResponse, error) {
			return &engramv1.ListTagsResponse{Tags: []*engramv1.TagCount{{Tag: "helm", Count: 19}, {Tag: "oci", Count: 11}}}, nil
		},
	}
	h := newConsoleHarness(t, svc)
	h.press("2")
	h.wantView("#helm", "19", "■")
	h.press("space", "j", "space", "enter")
	if req := lists.last(t); !slices.Equal(req.GetTags(), []string{"helm", "oci"}) {
		t.Errorf("list request tags = %v, want [helm oci]", req.GetTags())
	}
	h.wantView("all of #helm #oci")
}

func TestTUIScheduledWindows(t *testing.T) {
	var scheduled recorder[engramv1.ListScheduledRequest]
	svc := &stubEngramService{
		listFn: listOf(),
		listScheduledFn: func(_ context.Context, req *engramv1.ListScheduledRequest) (*engramv1.ListScheduledResponse, error) {
			scheduled.add(req)
			r := testMemory("nnnnnnnnn4", req.GetState()+" record")
			if req.GetState() == "expired" {
				r.NotAfter = timestamppb.New(consoleNow.Add(-48 * time.Hour))
			} else {
				r.NotBefore = timestamppb.New(consoleNow.Add(72 * time.Hour))
			}
			return &engramv1.ListScheduledResponse{Memories: []*engramv1.Memory{r}}, nil
		},
	}
	h := newConsoleHarness(t, svc)
	h.press("4")
	if req := scheduled.last(t); req.GetState() != "scheduled" || !req.GetCrossSpine() {
		t.Errorf("ListScheduled = %v, want state scheduled across every readable scope", req)
	}
	h.wantView("reveals in 3d")
	h.press("w")
	if req := scheduled.last(t); req.GetState() != "expired" {
		t.Errorf("state after w = %q, want expired", req.GetState())
	}
	h.wantView("expired 2d ago")
}

func TestTUIRulesPreviewFullText(t *testing.T) {
	rule := testMemory("mmmmmmmmm3", "never push to main")
	rule.Category, rule.Scope, rule.Content = "rule", "rule:repo:example/one", "the full rule text"
	compact := testMemory("mmmmmmmmm3", "never push to main")
	compact.Category, compact.Scope, compact.Content = "rule", "rule:repo:example/one", ""
	svc := &stubEngramService{
		listFn: listOf(),
		getFn:  getFrom(nil, rule),
		listRulesFn: func(context.Context, *engramv1.ListRulesRequest) (*engramv1.ListRulesResponse, error) {
			return &engramv1.ListRulesResponse{Rules: []*engramv1.Memory{compact}, SearchedScopes: []string{"rule:repo:example/one"}}, nil
		},
	}
	h := newConsoleHarness(t, svc)
	h.press("3")
	h.fire()
	h.wantView("never push to main", "the full rule text")
}

func TestTUIRequestErrorStaysInSession(t *testing.T) {
	const envelope = "field=response hint=response_too_large: the result is too large to return in one response; retry with a smaller limit or k, or omit full"
	var tags recorder[engramv1.ListTagsRequest]
	svc := &stubEngramService{
		listFn: func(context.Context, *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			return nil, connect.NewError(connect.CodeResourceExhausted, errors.New(envelope))
		},
		listTagsFn: func(_ context.Context, req *engramv1.ListTagsRequest) (*engramv1.ListTagsResponse, error) {
			tags.add(req)
			return &engramv1.ListTagsResponse{}, nil
		},
	}
	h := newSizedConsoleHarness(t, svc, 240, 40, consoleSettings{theme: "engram"})
	h.wantView(envelope)
	h.press("2")
	if len(tags.all()) != 1 {
		t.Error("the key press after the error was not handled")
	}
}

func TestTUIDropsStaleResults(t *testing.T) {
	svc := &stubEngramService{
		listFn: listOf(),
		searchFn: func(_ context.Context, req *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{Memories: []*engramv1.Memory{testMemory("ppppppppp5", "result for "+req.GetQuery())}}, nil
		},
	}
	h := newConsoleHarness(t, svc)
	search := func(q string) tea.Cmd {
		h.press("/")
		h.m.input.SetValue(q)
		return h.update(consoleKey("enter"))
	}
	older, newer := search("older"), search("newer")
	h.run(newer)
	h.run(older)
	h.wantView("result for newer")
	h.noView("result for older")
}

// --- overlays ---

func TestTUIOverlayStack(t *testing.T) {
	h := newConsoleHarness(t, &stubEngramService{listFn: listOf()})
	h.m.openOverlay(newHelpOverlay())
	h.m.openOverlay(h.m.newThemeOverlay())
	h.press("esc")
	if len(h.m.overlays) != 1 || h.m.overlays[0].kind != overlayHelp {
		t.Errorf("after one Esc: %d overlays, want only the help overlay", len(h.m.overlays))
	}
}

func TestTUIPalette(t *testing.T) {
	var tags recorder[engramv1.ListTagsRequest]
	svc := &stubEngramService{
		listFn: listOf(),
		listTagsFn: func(_ context.Context, req *engramv1.ListTagsRequest) (*engramv1.ListTagsResponse, error) {
			tags.add(req)
			return &engramv1.ListTagsResponse{}, nil
		},
	}
	h := newConsoleHarness(t, svc)
	h.press("ctrl+k")
	o := h.m.overlays[0]
	for _, b := range h.m.keys.all() {
		if !slices.ContainsFunc(o.entries, func(e paletteEntry) bool {
			return e.key == b.Help().Key && e.title == b.Help().Desc
		}) {
			t.Errorf("the palette does not list %q (%s)", b.Help().Desc, b.Help().Key)
		}
	}
	h.typeText("tags")
	h.wantView("tags")
	if e := o.entries[o.matches[0].Index]; e.key != "2" {
		t.Errorf("first match = %+v, want the tags view entry with key 2", e)
	}
	h.press("enter")
	if h.m.tab != tabTags || len(tags.all()) != 1 {
		t.Errorf("tab = %v with %d tag requests, want the tags view loaded", h.m.tab, len(tags.all()))
	}
	h.press("ctrl+k", "esc")
	if len(h.m.overlays) != 0 || h.m.tab != tabTags {
		t.Error("Esc on the palette ran something or did not close it")
	}
}

func TestTUIHelpListsEveryKey(t *testing.T) {
	h := newSizedConsoleHarness(t, &stubEngramService{listFn: listOf()}, 200, 60, consoleSettings{theme: "engram"})
	h.press("?")
	for _, b := range h.m.keys.all() {
		h.wantView(b.Help().Desc)
	}
	h.press("esc")
	if len(h.m.overlays) != 0 {
		t.Error("Esc did not close the help")
	}
}

func TestTUIFiltersChooseScope(t *testing.T) {
	var lists recorder[engramv1.ListMemoriesRequest]
	svc := &stubEngramService{listFn: func(_ context.Context, req *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
		lists.add(req)
		return &engramv1.ListMemoriesResponse{}, nil
	}}
	h := newConsoleHarness(t, svc, "repo:example/one", "repo:example/two")
	before := len(lists.all())
	h.press("f")
	h.typeText("two")
	h.press("down", "down", "down", "down", "enter")
	if got := len(lists.all()) - before; got != 1 {
		t.Fatalf("choosing a scope sent %d requests, want 1", got)
	}
	if req := lists.last(t); req.GetScope() != "repo:example/two" || req.GetCrossSpine() {
		t.Errorf("request = %v, want scope repo:example/two", req)
	}
	h.wantView("scope: repo:example/two")
}

func TestTUIThemePicker(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	settings := loadConsoleSettings(config.ClientConfig{}, func(string) string { return "" })
	h := newSizedConsoleHarness(t, &stubEngramService{listFn: listOf()}, 200, 40, settings)
	h.press("T", "down")
	if h.m.themeName != "terminal" {
		t.Errorf("highlighting terminal shows %q, want terminal", h.m.themeName)
	}
	h.press("esc")
	if h.m.themeName != "engram" {
		t.Errorf("after Esc the theme is %q, want engram", h.m.themeName)
	}
	h.press("T", "down", "down", "enter")
	if h.m.themeName != "catppuccin" {
		t.Fatalf("after Enter the theme is %q, want catppuccin", h.m.themeName)
	}
	again := loadConsoleSettings(config.ClientConfig{}, func(string) string { return "" })
	if again.theme != "catppuccin" {
		t.Errorf("after a restart the theme is %q, want catppuccin", again.theme)
	}
}

// --- mouse and copy ---

func TestTUIMouse(t *testing.T) {
	rows := []*engramv1.Memory{testMemory("aaaaaaaaa1", "first"), testMemory("bbbbbbbbb2", "second"),
		testMemory("ccccccccc3", "third")}
	h := newConsoleHarness(t, &stubEngramService{listFn: listOf(rows...), getFn: getFrom(nil, rows...)})
	if mode := h.m.View().MouseMode; mode != tea.MouseModeNone {
		t.Errorf("default mouse mode = %v, want none", mode)
	}
	h.send(tea.MouseClickMsg{X: 5, Y: chromeTop + 2, Button: tea.MouseLeft})
	if h.m.active().cursor != 0 {
		t.Error("a click selected a row while the mouse was off")
	}
	on := newSizedConsoleHarness(t, &stubEngramService{listFn: listOf(rows...), getFn: getFrom(nil, rows...)},
		200, 40, consoleSettings{theme: "engram", mouse: true})
	if mode := on.m.View().MouseMode; mode != tea.MouseModeCellMotion {
		t.Errorf("mouse on: mode %v, want cell motion", mode)
	}
	on.send(tea.MouseClickMsg{X: 5, Y: chromeTop + 2, Button: tea.MouseLeft})
	on.fire()
	if on.m.active().selectedID != "id-ccccccccc3" {
		t.Errorf("clicking the third row selected %q", on.m.active().selectedID)
	}
	on.wantView("third in full")
}

func TestTUICopyShortID(t *testing.T) {
	r := testMemory("aaaaaaaaa1", "first")
	h := newConsoleHarness(t, &stubEngramService{listFn: listOf(r), getFn: getFrom(nil, r)})
	msgs := runConsoleCmd(h.update(consoleKey("c")))
	if !slices.ContainsFunc(msgs, func(m tea.Msg) bool {
		return fmt.Sprintf("%T", m) == "tea.setClipboardMsg" && fmt.Sprint(m) == "aaaaaaaaa1"
	}) {
		t.Errorf("c produced %v, want the clipboard set to the short_id", msgs)
	}
	h.wantView("copied short_id aaaaaaaaa1")
	stale := toastExpireMsg{seq: h.m.toastSeq - 1}
	h.send(stale)
	h.wantView("copied short_id aaaaaaaaa1")
	h.fire()
	h.noView("copied short_id")
}

// --- whole session ---

func TestTUIQuitKeys(t *testing.T) {
	m := newConsoleModel(context.Background(), nil, time.Second, &engramv1.ListScopesResponse{}, consoleSettings{theme: "engram"})
	for _, k := range []string{"q", "ctrl+c"} {
		_, cmd := m.Update(consoleKey(k))
		if cmd == nil {
			t.Fatalf("%s returned no command", k)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%s did not return the quit command", k)
		}
	}
	m.input.Focus()
	if _, cmd := m.Update(consoleKey("q")); cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Error("q quit while typing a search")
		}
	}
}

// TestTUIReadOnlySession runs a session through every view to quit. The
// harness fails the test on any write RPC the server receives.
func TestTUIReadOnlySession(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	rec := testMemory("qqqqqqqqq6", "a record")
	svc := &stubEngramService{
		listFn: listOf(rec),
		searchFn: func(context.Context, *engramv1.SearchMemoriesRequest) (*engramv1.SearchMemoriesResponse, error) {
			return &engramv1.SearchMemoriesResponse{Memories: []*engramv1.Memory{rec}}, nil
		},
		getFn: getFrom(nil, rec),
		relatedFn: func(context.Context, *engramv1.RelatedMemoriesRequest) (*engramv1.RelatedMemoriesResponse, error) {
			return &engramv1.RelatedMemoriesResponse{Anchor: rec}, nil
		},
		listTagsFn: func(context.Context, *engramv1.ListTagsRequest) (*engramv1.ListTagsResponse, error) {
			return &engramv1.ListTagsResponse{Tags: []*engramv1.TagCount{{Tag: "t", Count: 1}}}, nil
		},
		listRulesFn: func(context.Context, *engramv1.ListRulesRequest) (*engramv1.ListRulesResponse, error) {
			return &engramv1.ListRulesResponse{}, nil
		},
		listScheduledFn: func(context.Context, *engramv1.ListScheduledRequest) (*engramv1.ListScheduledResponse, error) {
			return &engramv1.ListScheduledResponse{}, nil
		},
	}
	h := newSizedConsoleHarness(t, svc, 200, 40, loadConsoleSettings(config.ClientConfig{}, func(string) string { return "" }), "repo:example/one")
	h.press("/")
	h.typeText("anything")
	h.press("enter", "enter", "esc", "r", "esc", "?", "esc", "ctrl+k", "esc", "T", "down", "enter",
		"f", "space", "esc", "=", "esc", "2", "space", "enter", "3", "4", "w", "1", "n", "j", "k", "tab", "J", "c", "C", "p", "z")
	h.fire()
	if cmd := h.update(consoleKey("q")); cmd == nil {
		t.Fatal("q returned no command")
	} else if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q did not quit")
	}
	if h.m.statusErr {
		t.Errorf("session ended with an error: %s", h.m.status)
	}
}
