// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"pgregory.net/rapid"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)

// modelCaps are the memory write limits the lane tests configure. They are
// small so a generated value crosses them cheaply.
var modelCaps = memoryWriteCaps{contentBytes: 64, tags: 3, tagBytes: 8}

const modelSummaryBytes = 16

// lanes holds one MCP session and one Connect API, each on its own real
// store, for the same owner and clock.
type lanes struct {
	owner           string
	now             time.Time
	mcpDeps, conn   *deps
	mcpSt, connSt   *store.Store
	mcpCtx, connCtx context.Context
	cs              *mcp.ClientSession
	api             *engramAPI
}

func newLanes(t *testing.T, name string) *lanes {
	t.Helper()
	l := &lanes{owner: "model-lanes-" + name, now: time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)}
	clock := func() time.Time { return l.now }
	mk := func(lane string) (*deps, *store.Store) {
		st := newTestStore(t, storetest.Dial(t, storetest.RecvLimit), testCollection("model_"+name+"_"+lane), store.WithClock(clock))
		if err := st.EnsureCollection(context.Background(), 3); err != nil {
			t.Fatalf("ensure %s: %v", lane, err)
		}
		return &deps{st: st, em: fakeEmbedder{}, now: clock, writeCaps: modelCaps, maxSummaryBytes: modelSummaryBytes}, st
	}
	l.mcpDeps, l.mcpSt = mk("mcp")
	l.conn, l.connSt = mk("connect")
	l.mcpCtx, l.cs = newLongMCPSession(t, l.mcpDeps, l.owner)
	l.connCtx = parityConnectCtx(l.owner)
	l.api = &engramAPI{d: l.conn}
	return l
}

// newLongMCPSession is newMCPSession without its ten-second deadline, which
// a rapid run outlasts.
func newLongMCPSession(t *testing.T, d *deps, owner string) (context.Context, *mcp.ClientSession) {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "engram-test", Version: "test"}, nil)
	if err := registerTools(s, d); err != nil {
		t.Fatalf("registerTools: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ss, err := s.Connect(authedContext(t, owner), serverTransport, nil)
	if err != nil {
		t.Fatalf("server Connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "engram-test-client", Version: "test"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client Connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return ctx, cs
}

func (l *lanes) cleanup(scope string) {
	ctx := context.Background()
	_ = l.mcpSt.DeleteAll(ctx, scope, store.Authenticated(l.owner))
	_ = l.connSt.DeleteAll(ctx, scope, store.Authenticated(l.owner))
}

// seed stores one record on each lane through the core, for verbs that need
// an existing target, and returns the two ids.
func (l *lanes) seed(t fataler, scope string) (mcpID, connID string) {
	t.Helper()
	a := storeArgs{Content: "seed", Scope: scope, Source: "user-said", Category: "decision"}
	c := callerForOwner(l.owner)
	mcpID, _, err := l.mcpDeps.storeMemory(context.Background(), c, a)
	if err != nil {
		t.Fatalf("seed MCP lane: %v", err)
	}
	connID, _, err = l.conn.storeMemory(context.Background(), c, a)
	if err != nil {
		t.Fatalf("seed Connect lane: %v", err)
	}
	return mcpID, connID
}

func callerForOwner(owner string) caller {
	c, err := callerFromTokenInfo(&mcpauth.TokenInfo{UserID: owner, Extra: map[string]any{"owner_claim": owner}})
	if err != nil {
		panic(err)
	}
	return c
}

type fataler interface {
	Helper()
	Fatalf(format string, args ...any)
}

// outcome is one lane's answer, reduced to what the lanes must agree on.
type outcome struct {
	ok          bool
	field, hint string
	text        string
	structured  map[string]any
}

var envelopeRE = regexp.MustCompile(`^field=([a-z_,]+) hint=([a-z_]+): `)

func outcomeOf(text string) outcome {
	o := outcome{text: text}
	if m := envelopeRE.FindStringSubmatch(text); m != nil {
		o.field, o.hint = m[1], m[2]
	}
	return o
}

func (l *lanes) callMCP(t fataler, tool string, args map[string]any) outcome {
	t.Helper()
	res, err := l.cs.CallTool(l.mcpCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", tool, err)
	}
	if !res.IsError {
		sc, _ := res.StructuredContent.(map[string]any)
		return outcome{ok: true, structured: sc}
	}
	var text string
	if len(res.Content) > 0 {
		if tc, ok := res.Content[0].(*mcp.TextContent); ok {
			text = tc.Text
		}
	}
	return outcomeOf(text)
}

func connectOutcome(err error) outcome {
	if err == nil {
		return outcome{ok: true}
	}
	var ce *connect.Error
	if errors.As(err, &ce) {
		return outcomeOf(ce.Message())
	}
	return outcomeOf(err.Error())
}

// call is one lane-neutral request. Each verb turns it into an MCP argument
// map and a Connect request.
type call struct {
	verb                        string
	content, scope, source, cat string
	summary                     string
	tags                        []string
	notBefore, notAfter         *time.Time
	supersedes                  []string // "target" stands for each lane's seeded id
	limit                       uint64
	createdAfter, createdBefore string
	idempotencyKey              string
}

func rfc(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func ts(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func targets(ids []string, target string) []string {
	if ids == nil {
		return nil
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = map[bool]string{true: target, false: id}[id == "target"]
	}
	return out
}

func putIf(m map[string]any, k string, v any) {
	switch x := v.(type) {
	case string:
		if x == "" {
			return
		}
	case []string:
		if x == nil {
			return
		}
	case uint64:
		if x == 0 {
			return
		}
	}
	m[k] = v
}

func (c call) mcpArgs(target string) (string, map[string]any) {
	m := map[string]any{}
	if c.verb == "list" {
		putIf(m, "scope", c.scope)
		putIf(m, "limit", c.limit)
		putIf(m, "created_after", c.createdAfter)
		putIf(m, "created_before", c.createdBefore)
		return "list_memory", m
	}
	if c.verb == "update" {
		m["id"] = target
		putIf(m, "content", c.content)
		putIf(m, "summary", c.summary)
		putIf(m, "tags", c.tags)
		return "update_memory", m
	}
	putIf(m, "content", c.content)
	putIf(m, "scope", c.scope)
	putIf(m, "source", c.source)
	putIf(m, "category", c.cat)
	putIf(m, "summary", c.summary)
	putIf(m, "tags", c.tags)
	putIf(m, "idempotency_key", c.idempotencyKey)
	switch c.verb {
	case "schedule":
		putIf(m, "not_before", rfc(c.notBefore))
		putIf(m, "not_after", rfc(c.notAfter))
		return "schedule_memory", m
	case "supersede":
		putIf(m, "supersedes", targets(c.supersedes, target))
		return "supersede_memory", m
	}
	return "store_memory", m
}

func (c call) connect(l *lanes, target string) error {
	ctx := l.connCtx
	var err error
	switch c.verb {
	case "store":
		_, err = l.api.StoreMemory(ctx, connect.NewRequest(&engramv1.StoreMemoryRequest{
			Content: c.content, Scope: c.scope, Source: c.source, Category: c.cat, Summary: c.summary, Tags: c.tags}))
	case "schedule":
		_, err = l.api.ScheduleMemory(ctx, connect.NewRequest(&engramv1.ScheduleMemoryRequest{
			Content: c.content, Scope: c.scope, Source: c.source, Category: c.cat, Summary: c.summary, Tags: c.tags,
			NotBefore: ts(c.notBefore), NotAfter: ts(c.notAfter), IdempotencyKey: c.idempotencyKey}))
	case "supersede":
		_, err = l.api.SupersedeMemory(ctx, connect.NewRequest(&engramv1.SupersedeMemoryRequest{
			Content: c.content, Scope: c.scope, Source: c.source, Category: c.cat, Summary: c.summary, Tags: c.tags,
			Supersedes: targets(c.supersedes, target), IdempotencyKey: c.idempotencyKey}))
	case "update":
		var paths []string
		for p, set := range map[string]bool{"content": c.content != "", "summary": c.summary != "", "tags": c.tags != nil} {
			if set {
				paths = append(paths, p)
			}
		}
		slices.Sort(paths)
		_, err = l.api.UpdateMemory(ctx, connect.NewRequest(&engramv1.UpdateMemoryRequest{
			Id: target, Content: c.content, Summary: c.summary, Tags: c.tags, UpdateMask: &fieldmaskpb.FieldMask{Paths: paths}}))
	case "list":
		_, err = l.api.ListMemories(ctx, connect.NewRequest(&engramv1.ListMemoriesRequest{
			Scope: c.scope, Limit: c.limit, CreatedAfter: c.createdAfter, CreatedBefore: c.createdBefore, CursorMode: true}))
	default:
		panic("unknown verb " + c.verb)
	}
	return err
}

// sized draws a string around limit: well under, at, one over, or far over.
func sized(t *rapid.T, label string, limit int) string {
	n := rapid.SampledFrom([]int{1, limit, limit + 1, limit * 3}).Draw(t, label)
	return strings.Repeat("x", n)
}

func drawTags(t *rapid.T) []string {
	switch rapid.IntRange(0, 3).Draw(t, "tags_class") {
	case 0:
		return nil
	case 1:
		return rapid.SliceOfN(rapid.StringMatching(`[a-z]{1,8}`), 1, modelCaps.tags).Draw(t, "tags")
	case 2:
		return slices.Repeat([]string{"t"}, modelCaps.tags+1)
	}
	return []string{sized(t, "tag", modelCaps.tagBytes)}
}

// drawCall draws a call inside both lanes' shared contract: each field holds
// a valid value or one of its invalid classes.
func (l *lanes) drawCall(t *rapid.T, scope string) call {
	c := call{verb: rapid.SampledFrom([]string{"store", "schedule", "supersede", "update", "list"}).Draw(t, "verb")}
	pick := func(label string, valid string, invalid ...string) string {
		return rapid.SampledFrom(append([]string{valid, valid}, invalid...)).Draw(t, label)
	}
	if c.verb == "list" {
		c.scope = pick("scope", scope, "")
		c.limit = rapid.SampledFrom([]uint64{0, 5, 1000, 1001}).Draw(t, "limit")
		c.createdAfter = pick("created_after", "", "2026-01-01T00:00:00Z", "yesterday")
		c.createdBefore = pick("created_before", "", "2027-01-01T00:00:00Z", "2026-13-01")
		return c
	}
	if c.verb == "update" {
		// MCP requires content on update and Connect does not, so content
		// is always present here. TestModelLaneParity checks that
		// difference on its own.
		c.content = sized(t, "content", modelCaps.contentBytes)
	} else {
		c.content = pick("content", sized(t, "content", modelCaps.contentBytes), "")
		c.scope = pick("scope", scope, "")
		c.source = pick("source", "user-said", "")
		c.cat = pick("category", "decision", "")
	}
	c.summary = pick("summary", "", sized(t, "summary", modelSummaryBytes))
	c.tags = drawTags(t)
	switch c.verb {
	case "schedule":
		at := func(label string, h int) *time.Time {
			v := l.now.Add(time.Duration(h) * time.Hour)
			return map[bool]*time.Time{true: &v, false: nil}[rapid.Bool().Draw(t, label)]
		}
		c.notBefore = at("has_not_before", rapid.IntRange(-2, 3).Draw(t, "not_before"))
		c.notAfter = at("has_not_after", rapid.IntRange(-2, 3).Draw(t, "not_after"))
	case "supersede":
		c.supersedes = rapid.SampledFrom([][]string{nil, {"target"}, {uuid.NewString()}}).Draw(t, "supersedes")
	}
	return c
}

func TestModelLaneParity(t *testing.T) {
	l := newLanes(t, "parity")
	hints := hintCodeConstants(t)
	run := uuid.NewString()

	// Update differs on purpose: MCP requires content, Connect applies a
	// partial update.
	scope := "model-parity:" + run + ":update"
	mcpID, connID := l.seed(t, scope)
	got := l.callMCP(t, "update_memory", map[string]any{"id": mcpID, "tags": []string{"a"}})
	if got.ok || got.field != "content" || got.hint != string(HintRequired) {
		t.Fatalf("MCP update without content = %+v, want field=content hint=required", got)
	}
	if err := (call{verb: "update", tags: []string{"a"}}).connect(l, connID); err != nil {
		t.Fatalf("Connect partial update: %v", err)
	}
	l.cleanup(scope)

	n := 0
	rapid.Check(t, func(t *rapid.T) {
		n++
		scope := fmt.Sprintf("model-parity:%s:%d", run, n)
		defer l.cleanup(scope)
		mcpTarget, connTarget := l.seed(t, scope)
		c := l.drawCall(t, scope)
		tool, args := c.mcpArgs(mcpTarget)
		m := l.callMCP(t, tool, args)
		k := connectOutcome(c.connect(l, connTarget))
		if m.ok != k.ok {
			t.Fatalf("%s: MCP ok=%v (%q), Connect ok=%v (%q)", c.verb, m.ok, m.text, k.ok, k.text)
		}
		if m.ok {
			return
		}
		if m.field != k.field || m.hint != k.hint {
			t.Fatalf("%s: MCP field=%s hint=%s (%q), Connect field=%s hint=%s (%q)", c.verb, m.field, m.hint, m.text, k.field, k.hint, k.text)
		}
		if m.hint != "" && !slices.Contains(hints, m.hint) {
			t.Fatalf("%s: hint %q is not a HintCode constant", c.verb, m.hint)
		}
	})
}
