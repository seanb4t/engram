// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
)

// consoleNow is the console's clock in every model test.
var consoleNow = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

// consoleHarness drives the console model against a stub server: it sends
// messages to the model and runs every command the model returns, feeding
// each result back, so a test sees what a session would. Timers never
// run: the model's clock queues each timer message in ticks, and a test
// delivers them with fire.
type consoleHarness struct {
	t     *testing.T
	m     *consoleModel
	ticks []tea.Msg
}

func newConsoleHarness(t *testing.T, svc *stubEngramService, scopes ...string) *consoleHarness {
	t.Helper()
	return newSizedConsoleHarness(t, svc, 200, 40, consoleSettings{theme: "engram"}, scopes...)
}

func newSizedConsoleHarness(t *testing.T, svc *stubEngramService, w, h int, settings consoleSettings, scopes ...string) *consoleHarness {
	t.Helper()
	svc.readOnly = t
	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, startStubServer(t, svc))
	resp := &engramv1.ListScopesResponse{}
	for _, s := range scopes {
		resp.Scopes = append(resp.Scopes, &engramv1.ScopeCount{Scope: s, Count: 1})
	}
	hr := &consoleHarness{t: t}
	hr.m = newConsoleModel(context.Background(), client, 5*time.Second, resp, settings)
	hr.m.now = func() time.Time { return consoleNow }
	hr.m.after = func(_ time.Duration, msg tea.Msg) tea.Cmd {
		hr.ticks = append(hr.ticks, msg)
		return nil
	}
	hr.send(tea.WindowSizeMsg{Width: w, Height: h})
	hr.run(hr.m.Init())
	return hr
}

func (h *consoleHarness) update(msg tea.Msg) tea.Cmd {
	_, cmd := h.m.Update(msg)
	return cmd
}

func (h *consoleHarness) send(msg tea.Msg) { h.run(h.update(msg)) }

func (h *consoleHarness) run(cmd tea.Cmd) {
	for _, msg := range runConsoleCmd(cmd) {
		h.send(msg)
	}
}

// fire delivers every queued timer message, as if each timer had run out.
func (h *consoleHarness) fire() {
	for len(h.ticks) > 0 {
		pending := h.ticks
		h.ticks = nil
		for _, msg := range pending {
			h.send(msg)
		}
	}
}

// runConsoleCmd runs cmd and returns the messages it produced, with
// batches flattened.
func runConsoleCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	switch msg := cmd().(type) {
	case nil:
		return nil
	case tea.BatchMsg:
		var out []tea.Msg
		for _, c := range msg {
			out = append(out, runConsoleCmd(c)...)
		}
		return out
	default:
		return []tea.Msg{msg}
	}
}

func consoleKey(k string) tea.KeyPressMsg { return keyPress(k) }

func (h *consoleHarness) press(keys ...string) {
	for _, k := range keys {
		h.send(consoleKey(k))
	}
}

func (h *consoleHarness) typeText(s string) {
	for _, r := range s {
		h.send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

var ansiCSI = regexp.MustCompile("\x1b\\[[0-9;:?]*[ -/]*[@-~]")

// view is the screen with escape sequences stripped.
func (h *consoleHarness) view() string {
	return ansiCSI.ReplaceAllString(h.m.View().Content, "")
}

func (h *consoleHarness) wantView(substrs ...string) {
	h.t.Helper()
	v := h.view()
	for _, s := range substrs {
		if !strings.Contains(v, s) {
			h.t.Errorf("view does not contain %q:\n%s", s, v)
		}
	}
}

func (h *consoleHarness) noView(substrs ...string) {
	h.t.Helper()
	v := h.view()
	for _, s := range substrs {
		if strings.Contains(v, s) {
			h.t.Errorf("view contains %q, want it absent:\n%s", s, v)
		}
	}
}

// line is the first screen line holding sub, or "".
func (h *consoleHarness) line(sub string) string {
	for _, l := range strings.Split(h.view(), "\n") {
		if strings.Contains(l, sub) {
			return l
		}
	}
	return ""
}

// recorder keeps every request one RPC received.
type recorder[Req any] struct {
	mu   sync.Mutex
	reqs []*Req
}

func (r *recorder[Req]) add(req *Req) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reqs = append(r.reqs, req)
}

func (r *recorder[Req]) all() []*Req {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*Req(nil), r.reqs...)
}

func (r *recorder[Req]) last(t *testing.T) *Req {
	t.Helper()
	all := r.all()
	if len(all) == 0 {
		t.Fatal("no request received")
	}
	return all[len(all)-1]
}

func stringPtr(s string) *string { return &s }

func testMemory(shortID, summary string) *engramv1.Memory {
	return &engramv1.Memory{
		Id: "id-" + shortID, ShortId: shortID, Summary: summary, Content: summary + " in full",
		Category: "decision", Scope: "repo:example/one", Owner: "me@example.com",
		CreatedAt: timestamppb.New(consoleNow.Add(-48 * time.Hour)),
	}
}

func listOf(mems ...*engramv1.Memory) func(context.Context, *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
	return func(context.Context, *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
		return &engramv1.ListMemoriesResponse{Memories: mems}, nil
	}
}

// getFrom answers GetMemory from mems by id or short_id, and not found
// for anything else.
func getFrom(gets *recorder[engramv1.GetMemoryRequest], mems ...*engramv1.Memory) func(context.Context, *engramv1.GetMemoryRequest) (*engramv1.GetMemoryResponse, error) {
	return func(_ context.Context, req *engramv1.GetMemoryRequest) (*engramv1.GetMemoryResponse, error) {
		if gets != nil {
			gets.add(req)
		}
		for _, m := range mems {
			if m.GetId() == req.GetId() || m.GetShortId() == req.GetId() {
				return &engramv1.GetMemoryResponse{Memory: m}, nil
			}
		}
		return nil, connect.NewError(connect.CodeNotFound, errors.New("memory not found"))
	}
}

func getIDs(gets *recorder[engramv1.GetMemoryRequest]) []string {
	all := gets.all()
	ids := make([]string, 0, len(all))
	for _, r := range all {
		ids = append(ids, r.GetId())
	}
	return ids
}
