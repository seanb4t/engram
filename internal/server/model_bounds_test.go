// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"pgregory.net/rapid"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
)

// bound is one size limit of one write verb.
type bound struct {
	field string
	limit int  // 0 means the check is off
	count bool // a number of tags, not a byte length
}

// boundsFor lists each verb's effective limits: the test configuration for
// memory writes, the production constants for rules and discoveries.
func boundsFor(verb string, summaryLimit int) []bound {
	switch verb {
	case "store_rule":
		return []bound{{field: "content", limit: maxRuleContentBytes}, {field: "summary", limit: maxRuleSummaryBytes}}
	case "store_discovery":
		return []bound{{field: "content", limit: maxDiscoveryContentBytes}}
	}
	return []bound{
		{field: "content", limit: modelCaps.contentBytes},
		{field: "summary", limit: summaryLimit},
		{field: "tags", limit: modelCaps.tags, count: true},
		{field: "tag", limit: modelCaps.tagBytes},
	}
}

func (l *lanes) recordCount(t fataler, st *store.Store, scope string) uint64 {
	t.Helper()
	_, total, _, err := st.List(context.Background(), scope, store.Authenticated(l.owner), store.ListOptions{
		Limit: 1, IncludeArchived: true, IncludeSuperseded: true, IncludeScheduled: true})
	if err != nil {
		t.Fatalf("List(%s): %v", scope, err)
	}
	return total
}

// boundedWrite sends one verb with one sized field to both lanes.
func (l *lanes) boundedWrite(t fataler, verb, scope, field string, size int, mcpTarget, connTarget string) (outcome, outcome) {
	t.Helper()
	content, summary, tags := "content", "", []string(nil)
	if verb == "store_rule" {
		summary = "one line"
	}
	switch field {
	case "content":
		content = strings.Repeat("x", size)
	case "summary":
		summary = strings.Repeat("s", size)
	case "tags":
		tags = make([]string, size)
		for i := range tags {
			tags[i] = fmt.Sprintf("t%d", i)
		}
	case "tag":
		tags = []string{strings.Repeat("t", size)}
	}
	switch verb {
	case "store_rule":
		m := l.callMCP(t, "store_rule", map[string]any{"content": content, "scope": scope, "summary": summary})
		_, err := l.api.StoreRule(l.connCtx, connect.NewRequest(&engramv1.StoreRuleRequest{Content: content, Scope: scope, Summary: summary}))
		return m, connectOutcome(err)
	case "store_discovery":
		args := map[string]any{"content": content, "scope": scope, "kind": "fact",
			"citations": []map[string]any{{"kind": "file", "ref": "a.go"}}}
		m := l.callMCP(t, "store_discovery", args)
		_, err := l.api.StoreDiscovery(l.connCtx, connect.NewRequest(&engramv1.StoreDiscoveryRequest{
			Content: content, Scope: scope, Kind: "fact", Citations: []*engramv1.Citation{{Kind: "file", Ref: "a.go"}}}))
		return m, connectOutcome(err)
	}
	c := call{verb: verb, content: content, scope: scope, source: "user-said", cat: "decision", summary: summary, tags: tags}
	switch verb {
	case "schedule":
		at := l.now.Add(3600e9)
		c.notAfter = &at
	case "supersede":
		c.supersedes = []string{"target"}
	}
	tool, args := c.mcpArgs(mcpTarget)
	return l.callMCP(t, tool, args), connectOutcome(c.connect(l, connTarget))
}

func TestModelWriteBounds(t *testing.T) {
	l := newLanes(t, "bounds")
	run := uuid.NewString()
	n := 0
	rapid.Check(t, func(t *rapid.T) {
		n++
		verb := rapid.SampledFrom([]string{"store", "schedule", "supersede", "update", "store_rule", "store_discovery"}).Draw(t, "verb")
		summaryLimit := rapid.SampledFrom([]int{modelSummaryBytes, 0}).Draw(t, "summary_limit")
		l.mcpDeps.maxSummaryBytes, l.conn.maxSummaryBytes = summaryLimit, summaryLimit
		b := rapid.SampledFrom(boundsFor(verb, summaryLimit)).Draw(t, "bound")
		ref := b.limit
		if ref == 0 {
			ref = modelSummaryBytes
		}
		size := rapid.SampledFrom([]int{1, ref, ref + 1, ref * 3}).Draw(t, "size")

		scope := fmt.Sprintf("model-bounds:%s:%d", run, n)
		switch verb {
		case "store_rule":
			scope = fmt.Sprintf("rule:repo:model-bounds/%s/%d", run, n)
		case "store_discovery":
			scope = fmt.Sprintf("discovery:repo:model-bounds/%s/%d", run, n)
		}
		defer l.cleanup(scope)
		var mcpTarget, connTarget string
		if verb == "supersede" || verb == "update" {
			mcpTarget, connTarget = l.seed(t, scope)
		}
		beforeMCP, beforeConn := l.recordCount(t, l.mcpSt, scope), l.recordCount(t, l.connSt, scope)

		m, k := l.boundedWrite(t, verb, scope, b.field, size, mcpTarget, connTarget)
		over := b.limit > 0 && size > b.limit
		for lane, o := range map[string]outcome{"MCP": m, "Connect": k} {
			if !over {
				if !o.ok {
					t.Fatalf("%s %s with %s size %d (limit %d) rejected: %q", lane, verb, b.field, size, b.limit, o.text)
				}
				continue
			}
			field := map[bool]string{true: "tags", false: b.field}[b.field == "tag"]
			wantHint := map[bool]string{true: string(HintTooMany), false: string(HintTooLong)}[b.count]
			if o.ok || o.field != field || o.hint != wantHint {
				t.Fatalf("%s %s with %s size %d (limit %d) = %+v, want field=%s hint=%s", lane, verb, b.field, size, b.limit, o, field, wantHint)
			}
		}
		if !over {
			return
		}
		if got := l.recordCount(t, l.mcpSt, scope); got != beforeMCP {
			t.Fatalf("MCP %s over the %s bound changed the record count %d -> %d", verb, b.field, beforeMCP, got)
		}
		if got := l.recordCount(t, l.connSt, scope); got != beforeConn {
			t.Fatalf("Connect %s over the %s bound changed the record count %d -> %d", verb, b.field, beforeConn, got)
		}
		for _, target := range []struct {
			st *store.Store
			id string
		}{{l.mcpSt, mcpTarget}, {l.connSt, connTarget}} {
			if target.id == "" {
				continue
			}
			got, err := target.st.Get(context.Background(), target.id)
			if err != nil {
				t.Fatalf("Get(%s): %v", target.id, err)
			}
			if got.Content != "seed" || got.SupersededBy != nil || slices.Contains(got.Tags, "t0") {
				t.Fatalf("%s over the %s bound changed its target: %+v", verb, b.field, got)
			}
		}
	})
}
