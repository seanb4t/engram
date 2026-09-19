// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

// This file proves D-01 (memory content byte cap), D-09 (the caps are
// always enforced — Config.Validate rejects 0/negative rather than
// honoring them as "disabled") and D-10 (memory tags count/byte caps) end
// to end, on both the MCP and Connect lanes, against a spy-backed deps —
// every test here is hermetic (no Qdrant).

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
	"github.com/seanb4t/engram/internal/auth"
	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/store"
)

// newCapsMCPSession registers d's tools on a fresh in-memory MCP server and
// connects an authenticated in-memory client session as owner. Callers must
// close the returned session (t.Cleanup is sufficient); the underlying
// server session is cleaned up alongside it.
func newCapsMCPSession(t *testing.T, d *deps, owner string) *mcp.ClientSession {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "engram-test", Version: "test"}, nil)
	if err := registerTools(s, d); err != nil {
		t.Fatalf("registerTools: %v", err)
	}

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	ss, err := s.Connect(authedContext(t, owner), serverTransport, nil)
	if err != nil {
		t.Fatalf("server Connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	c := mcp.NewClient(&mcp.Implementation{Name: "engram-test-client", Version: "test"}, nil)
	cs, err := c.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client Connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	return cs
}

// callToolText calls the named tool with args and returns the first
// TextContent's text plus whether the result is an error result.
func callToolText(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): got Go error %v, want nil (the mapped result must still be a normal, non-erroring CallTool round trip)", name, err)
	}
	if len(res.Content) != 1 {
		t.Fatalf("CallTool(%s): len(Content) = %d, want 1 (content: %+v)", name, len(res.Content), res.Content)
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("CallTool(%s): Content[0] is %T, want *mcp.TextContent", name, res.Content[0])
	}
	return tc.Text, res.IsError
}

// newCapsConnectClient mounts d's Connect handlers on a fresh httptest
// server, authenticated as owner via the bearer lane, and returns a client.
// The server is closed in t.Cleanup.
func newCapsConnectClient(t *testing.T, d *deps, owner string) engramv1connect.EngramServiceClient {
	t.Helper()
	resolve := func(_ context.Context, _ connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error) {
		return &mcpauth.TokenInfo{Extra: map[string]any{auth.OwnerClaimExtraKey: owner}}, auth.LaneBearer, nil
	}
	csrfVerify := func(_, _ string) bool { return true }
	mux := http.NewServeMux()
	if err := d.mountConnect(mux, resolve, csrfVerify, nil); err != nil {
		t.Fatalf("mountConnect: %v", err)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
}

// repeatTags builds n tags, each exactly byteLen ASCII bytes (a zero-padded
// decimal index), for boundary-testing the count and per-tag byte caps
// independently.
func repeatTags(n, byteLen int) []string {
	tags := make([]string, n)
	for i := range tags {
		tags[i] = fmt.Sprintf("%0*d", byteLen, i)
	}
	return tags
}

// seedTarget writes a minimal, owned record directly into sp's map so a
// supersede_memory call in this file's tests has a resolvable target. It is
// inert for the cap-violation subtests below — validateStoreArgs rejects
// before target resolution ever runs — but keeps the call shaped like a
// realistic invocation.
func seedTarget(sp *spyStore, owner string) string {
	id := uuid.NewString()
	sp.mu.Lock()
	sp.records[id] = store.Memory{ID: id, Owner: owner, Content: "target", Scope: "tool:cap", Source: "user-said", Category: "decision"}
	sp.mu.Unlock()
	return id
}

// TestMemoryContentCapFlowsFromConfig proves D-01/D-09 end to end: a
// configured ENGRAM_MEMORY_MAX_CONTENT_BYTES reaches the MCP store_memory
// rejection unchanged, using the existing field=content hint=too_long
// envelope, and an at-cap write still succeeds and is actually stored.
func TestMemoryContentCapFlowsFromConfig(t *testing.T) {
	t.Setenv("ENGRAM_MEMORY_MAX_CONTENT_BYTES", "10")

	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	d, sp := newSpyDeps()
	d.writeCaps = memoryWriteCapsFromConfig(cfg)

	cs := newCapsMCPSession(t, d, "cap-owner")

	t.Run("over", func(t *testing.T) {
		text, isError := callToolText(t, cs, "store_memory", map[string]any{
			"content":  "01234567890", // 11 bytes — one over the configured cap of 10
			"scope":    "repo:cap",
			"source":   "user-said",
			"category": "decision",
		})
		if !isError {
			t.Fatalf("store_memory(11-byte content, cap 10): IsError = false, want true (text: %q)", text)
		}
		const wantPrefix = "field=content hint=too_long: "
		if !strings.HasPrefix(text, wantPrefix) {
			t.Errorf("store_memory(11-byte content): text = %q, want prefix %q", text, wantPrefix)
		}
		if !strings.Contains(text, "(max 10)") {
			t.Errorf("store_memory(11-byte content): text = %q, want it to contain %q", text, "(max 10)")
		}
		sp.mu.Lock()
		n := len(sp.records)
		sp.mu.Unlock()
		if n != 0 {
			t.Errorf("store_memory(11-byte content) rejected, but spy holds %d record(s), want 0", n)
		}
	})

	t.Run("at", func(t *testing.T) {
		const content = "0123456789" // exactly 10 bytes — at the configured cap
		text, isError := callToolText(t, cs, "store_memory", map[string]any{
			"content":  content,
			"scope":    "repo:cap",
			"source":   "user-said",
			"category": "decision",
		})
		if isError {
			t.Fatalf("store_memory(10-byte content, cap 10): IsError = true, want false (text: %q)", text)
		}
		sp.mu.Lock()
		defer sp.mu.Unlock()
		if len(sp.records) != 1 {
			t.Fatalf("store_memory(10-byte content) accepted, but spy holds %d record(s), want 1", len(sp.records))
		}
		for _, m := range sp.records {
			if m.Content != content {
				t.Errorf("stored record Content = %q, want %q", m.Content, content)
			}
		}
	})
}

// TestMemoryWriteCapsRejectOnEveryCreateLane proves D-01/D-10 on every
// create-style write path, on both the MCP and Connect lanes: content-cap,
// tag-count-cap, and per-tag-byte-cap violations are all rejected with the
// existing envelopes, and nothing is stored.
func TestMemoryWriteCapsRejectOnEveryCreateLane(t *testing.T) {
	const owner = "cap-lane-owner"

	type violation struct {
		name       string
		content    string
		tags       []string
		wantPrefix string
	}
	violations := []violation{
		{
			name:       "content-too-long",
			content:    strings.Repeat("a", defaultMaxContentBytes+1),
			wantPrefix: "field=content hint=too_long: ",
		},
		{
			name:       "too-many-tags",
			content:    "valid content",
			tags:       repeatTags(defaultMaxTags+1, 4),
			wantPrefix: "field=tags hint=too_many: ",
		},
		{
			name:       "tag-too-long",
			content:    "valid content",
			tags:       []string{strings.Repeat("a", defaultMaxTagBytes+1)},
			wantPrefix: "field=tags hint=too_long: ",
		},
	}

	t.Run("mcp", func(t *testing.T) {
		d, sp := newSpyDeps()
		targetID := seedTarget(sp, owner)
		cs := newCapsMCPSession(t, d, owner)

		mcpPaths := []struct {
			tool  string
			extra map[string]any
		}{
			{tool: "store_memory"},
			{tool: "schedule_memory", extra: map[string]any{"not_after": "2099-01-01T00:00:00Z"}},
			{tool: "supersede_memory", extra: map[string]any{"supersedes": []string{targetID}}},
		}

		for _, p := range mcpPaths {
			for _, v := range violations {
				t.Run(p.tool+"/"+v.name, func(t *testing.T) {
					args := map[string]any{
						"content": v.content, "scope": "tool:cap", "source": "user-said", "category": "decision",
					}
					if v.tags != nil {
						args["tags"] = v.tags
					}
					for k, val := range p.extra {
						args[k] = val
					}
					sp.mu.Lock()
					before := len(sp.records)
					sp.mu.Unlock()

					text, isError := callToolText(t, cs, p.tool, args)
					if !isError {
						t.Fatalf("%s(%s): IsError = false, want true (text: %q)", p.tool, v.name, text)
					}
					if !strings.HasPrefix(text, v.wantPrefix) {
						t.Errorf("%s(%s): text = %q, want prefix %q", p.tool, v.name, text, v.wantPrefix)
					}
					sp.mu.Lock()
					after := len(sp.records)
					sp.mu.Unlock()
					if after != before {
						t.Errorf("%s(%s): spy record count changed %d -> %d, want unchanged", p.tool, v.name, before, after)
					}
				})
			}
		}
	})

	t.Run("connect", func(t *testing.T) {
		d, sp := newSpyDeps()
		client := newCapsConnectClient(t, d, owner)
		notAfter := timestamppb.New(time.Now().Add(24 * time.Hour))

		for _, v := range violations {
			t.Run("StoreMemory/"+v.name, func(t *testing.T) {
				sp.mu.Lock()
				before := len(sp.records)
				sp.mu.Unlock()

				_, err := client.StoreMemory(context.Background(), connect.NewRequest(&engramv1.StoreMemoryRequest{
					Content: v.content, Scope: "tool:cap", Source: "user-said", Category: "decision", Tags: v.tags,
				}))
				if err == nil {
					t.Fatal("StoreMemory: err = nil, want CodeOutOfRange")
				}
				if code := connect.CodeOf(err); code != connect.CodeOutOfRange {
					t.Errorf("StoreMemory: code = %v, want CodeOutOfRange (err: %v)", code, err)
				}
				var cerr *connect.Error
				if errors.As(err, &cerr) && !strings.HasPrefix(cerr.Message(), v.wantPrefix) {
					t.Errorf("StoreMemory: message = %q, want prefix %q", cerr.Message(), v.wantPrefix)
				}
				sp.mu.Lock()
				after := len(sp.records)
				sp.mu.Unlock()
				if after != before {
					t.Errorf("StoreMemory(%s): spy record count changed %d -> %d, want unchanged", v.name, before, after)
				}
			})

			t.Run("ScheduleMemory/"+v.name, func(t *testing.T) {
				sp.mu.Lock()
				before := len(sp.records)
				sp.mu.Unlock()

				_, err := client.ScheduleMemory(context.Background(), connect.NewRequest(&engramv1.ScheduleMemoryRequest{
					Content: v.content, Scope: "tool:cap", Source: "user-said", Category: "decision", Tags: v.tags,
					NotAfter: notAfter,
				}))
				if err == nil {
					t.Fatal("ScheduleMemory: err = nil, want CodeOutOfRange")
				}
				if code := connect.CodeOf(err); code != connect.CodeOutOfRange {
					t.Errorf("ScheduleMemory: code = %v, want CodeOutOfRange (err: %v)", code, err)
				}
				var cerr *connect.Error
				if errors.As(err, &cerr) && !strings.HasPrefix(cerr.Message(), v.wantPrefix) {
					t.Errorf("ScheduleMemory: message = %q, want prefix %q", cerr.Message(), v.wantPrefix)
				}
				sp.mu.Lock()
				after := len(sp.records)
				sp.mu.Unlock()
				if after != before {
					t.Errorf("ScheduleMemory(%s): spy record count changed %d -> %d, want unchanged", v.name, before, after)
				}
			})
		}
	})
}

// TestMemoryWriteCapBoundaries drives d.storeMemory directly against a
// spy-backed deps, proving exact boundaries, byte-vs-rune precision,
// empty-input handling, rejection ORDER, configured (non-default) caps, and
// that a zero-value memoryWriteCaps is never uncapped (D-01/D-09/D-10).
func TestMemoryWriteCapBoundaries(t *testing.T) {
	type wantEnvelope struct {
		fields []string
		hint   HintCode
	}

	cases := []struct {
		name       string
		content    string
		tags       []string
		summary    string
		caps       memoryWriteCaps // zero value resolves to the defaults
		wantErr    bool
		want       wantEnvelope
		wantMaxSub string // substring expected in the rejection detail
	}{
		{name: "content-at-cap", content: strings.Repeat("a", defaultMaxContentBytes)},
		{
			name: "content-over-cap", content: strings.Repeat("a", defaultMaxContentBytes+1),
			wantErr: true, want: wantEnvelope{[]string{"content"}, HintTooLong},
		},
		{name: "tags-at-cap", content: "valid content", tags: repeatTags(defaultMaxTags, defaultMaxTagBytes)},
		{
			name: "tags-over-count", content: "valid content", tags: repeatTags(defaultMaxTags+1, 4),
			wantErr: true, want: wantEnvelope{[]string{"tags"}, HintTooMany},
		},
		{
			name: "tag-over-bytes", content: "valid content", tags: []string{strings.Repeat("a", defaultMaxTagBytes+1)},
			wantErr: true, want: wantEnvelope{[]string{"tags"}, HintTooLong},
		},
		{
			// 21846 copies of a 3-byte rune (€) = 65538 bytes, 21846 runes:
			// rejected because the cap counts BYTES, not runes.
			name: "multibyte-over", content: strings.Repeat("€", 21846),
			wantErr: true, want: wantEnvelope{[]string{"content"}, HintTooLong},
		},
		{
			// 21845 copies of € = 65535 bytes — under the 65536-byte cap.
			name: "multibyte-under", content: strings.Repeat("€", 21845),
		},
		{
			name: "empty-content", content: "",
			wantErr: true, want: wantEnvelope{[]string{"content"}, HintRequired},
		},
		{name: "empty-tags-nil", content: "valid content", tags: nil},
		{name: "empty-tags-empty-slice", content: "valid content", tags: []string{}},
		{
			name: "order-summary-before-content", content: strings.Repeat("a", defaultMaxContentBytes+1), summary: strings.Repeat("s", 1000),
			wantErr: true, want: wantEnvelope{[]string{"summary"}, HintTooLong},
		},
		{
			name: "order-content-before-tags", content: strings.Repeat("a", defaultMaxContentBytes+1), tags: repeatTags(defaultMaxTags+1, 4),
			wantErr: true, want: wantEnvelope{[]string{"content"}, HintTooLong},
		},
		{
			name: "order-count-before-bytes", content: "valid content", tags: repeatTags(defaultMaxTags+1, defaultMaxTagBytes+1),
			wantErr: true, want: wantEnvelope{[]string{"tags"}, HintTooMany},
		},
		{
			name: "configured-caps-content", content: strings.Repeat("a", 11),
			caps:    memoryWriteCaps{contentBytes: 10, tags: 2, tagBytes: 3},
			wantErr: true, want: wantEnvelope{[]string{"content"}, HintTooLong}, wantMaxSub: "(max 10)",
		},
		{
			name: "configured-caps-tags-count", content: "short", tags: []string{"a", "b", "c"},
			caps:    memoryWriteCaps{contentBytes: 10, tags: 2, tagBytes: 3},
			wantErr: true, want: wantEnvelope{[]string{"tags"}, HintTooMany}, wantMaxSub: "(max 2)",
		},
		{
			name: "configured-caps-tag-bytes", content: "short", tags: []string{"abcd"},
			caps:    memoryWriteCaps{contentBytes: 10, tags: 2, tagBytes: 3},
			wantErr: true, want: wantEnvelope{[]string{"tags"}, HintTooLong}, wantMaxSub: "(max 3)",
		},
		{
			// caps left at its zero value: resolved() must still cap at the
			// documented default, so a bare &deps{} is never uncapped.
			name: "zero-value-deps-is-capped", content: strings.Repeat("a", defaultMaxContentBytes+1),
			wantErr: true, want: wantEnvelope{[]string{"content"}, HintTooLong},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, sp := newSpyDeps()
			d.maxSummaryBytes = 512 // the approved default (D-18)
			d.writeCaps = tc.caps
			a := storeArgs{
				Content: tc.content, Scope: "tool:cap", Source: "user-said", Category: "decision",
				Tags: tc.tags, Summary: tc.summary,
			}
			_, _, err := d.storeMemory(context.Background(), zzCaller(), a)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("storeMemory(%s): want error, got nil", tc.name)
				}
				if fields := argFieldsOf(err); !reflect.DeepEqual(fields, tc.want.fields) {
					t.Errorf("storeMemory(%s): argFieldsOf(err) = %v, want %v (err: %v)", tc.name, fields, tc.want.fields, err)
				}
				if hint := argHintOf(err); hint != tc.want.hint {
					t.Errorf("storeMemory(%s): argHintOf(err) = %v, want %v (err: %v)", tc.name, hint, tc.want.hint, err)
				}
				if tc.wantMaxSub != "" && !strings.Contains(err.Error(), tc.wantMaxSub) {
					t.Errorf("storeMemory(%s): err = %q, want it to contain %q", tc.name, err.Error(), tc.wantMaxSub)
				}
				if len(sp.records) != 0 {
					t.Errorf("storeMemory(%s): rejected, but spy holds %d record(s), want 0", tc.name, len(sp.records))
				}
				return
			}

			if err != nil {
				t.Fatalf("storeMemory(%s): want nil, got %v", tc.name, err)
			}
			if len(sp.records) != 1 {
				t.Fatalf("storeMemory(%s): accepted, but spy holds %d record(s), want 1", tc.name, len(sp.records))
			}
			for _, m := range sp.records {
				if m.Content != tc.content {
					t.Errorf("storeMemory(%s): stored Content len = %d, want %d", tc.name, len(m.Content), len(tc.content))
				}
				if !slices.Equal(m.Tags, tc.tags) {
					t.Errorf("storeMemory(%s): stored Tags = %v, want %v", tc.name, m.Tags, tc.tags)
				}
			}
		})
	}
}

// TestMemoryWriteCapDefaultsMatchRegistry proves the server-side default
// consts equal the registry's documented defaults (65536/128/128) by test,
// not by reading (D-01/D-10).
func TestMemoryWriteCapDefaultsMatchRegistry(t *testing.T) {
	t.Setenv("ENGRAM_MEMORY_MAX_CONTENT_BYTES", "")
	t.Setenv("ENGRAM_MEMORY_MAX_TAGS", "")
	t.Setenv("ENGRAM_MEMORY_MAX_TAG_BYTES", "")
	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	got := memoryWriteCapsFromConfig(cfg)
	want := memoryWriteCaps{contentBytes: defaultMaxContentBytes, tags: defaultMaxTags, tagBytes: defaultMaxTagBytes}
	if got != want {
		t.Errorf("memoryWriteCapsFromConfig(defaults) = %+v, want %+v", got, want)
	}
	if defaultMaxContentBytes != 65536 || defaultMaxTags != 128 || defaultMaxTagBytes != 128 {
		t.Errorf("default consts = %d/%d/%d, want 65536/128/128", defaultMaxContentBytes, defaultMaxTags, defaultMaxTagBytes)
	}
}
