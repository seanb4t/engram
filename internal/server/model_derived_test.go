// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	validate "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
	"github.com/seanb4t/engram/internal/surfaces"
)

// wireActor is the header value that makes a callWrite request authenticate.
const wireActor = "model-lanes"

// connectOnlyMessageRules lists message rules that have no surface rule and
// only one lane. The scenario tests pin them.
var connectOnlyMessageRules = []string{"update_memory.mask"}

// fixtureInput is what a fixture builds a valid request from.
type fixtureInput struct {
	scope, target string
	l             *lanes
}

// parityFixture is one constrained request message: a valid Connect request,
// the valid arguments of the matching MCP tool, and how to send the request.
type parityFixture struct {
	tool       string
	seedTarget bool
	discovery  bool
	request    func(in fixtureInput) proto.Message
	args       func(in fixtureInput) map[string]any
	send       func(c engramv1connect.EngramServiceClient, m proto.Message) error
}

func writeArgs(in fixtureInput) map[string]any {
	return map[string]any{"content": "valid", "scope": in.scope, "source": "user-said", "category": "decision"}
}

func withArgs(base map[string]any, extra map[string]any) map[string]any {
	maps.Copy(base, extra)
	return base
}

func validCitationArgs() []map[string]any {
	return []map[string]any{{"kind": "file", "ref": "a.go"}}
}

func validCitations() []*engramv1.Citation {
	return []*engramv1.Citation{{Kind: "file", Ref: "a.go"}}
}

// parityFixtures is keyed by the request message's short name.
var parityFixtures = map[protoreflect.Name]parityFixture{
	"StoreMemoryRequest": {
		tool: "store_memory",
		request: func(in fixtureInput) proto.Message {
			return &engramv1.StoreMemoryRequest{Content: "valid", Scope: in.scope, Source: "user-said", Category: "decision"}
		},
		args: writeArgs,
		send: func(c engramv1connect.EngramServiceClient, m proto.Message) error {
			return callWrite(context.Background(), c.StoreMemory, m.(*engramv1.StoreMemoryRequest), wireActor)
		},
	},
	"ScheduleMemoryRequest": {
		tool: "schedule_memory",
		request: func(in fixtureInput) proto.Message {
			return &engramv1.ScheduleMemoryRequest{
				Content: "valid", Scope: in.scope, Source: "user-said", Category: "decision",
				NotAfter: timestamppb.New(in.l.now.Add(time.Hour)),
			}
		},
		args: func(in fixtureInput) map[string]any {
			at := in.l.now.Add(time.Hour)
			return withArgs(writeArgs(in), map[string]any{"not_after": rfc(&at)})
		},
		send: func(c engramv1connect.EngramServiceClient, m proto.Message) error {
			return callWrite(context.Background(), c.ScheduleMemory, m.(*engramv1.ScheduleMemoryRequest), wireActor)
		},
	},
	"SupersedeMemoryRequest": {
		tool:       "supersede_memory",
		seedTarget: true,
		request: func(in fixtureInput) proto.Message {
			return &engramv1.SupersedeMemoryRequest{
				Content: "valid", Scope: in.scope, Source: "user-said", Category: "decision",
				Supersedes: []string{in.target}, Citations: validCitations(),
			}
		},
		args: func(in fixtureInput) map[string]any {
			return withArgs(writeArgs(in), map[string]any{"supersedes": []string{in.target}, "citations": validCitationArgs()})
		},
		send: func(c engramv1connect.EngramServiceClient, m proto.Message) error {
			return callWrite(context.Background(), c.SupersedeMemory, m.(*engramv1.SupersedeMemoryRequest), wireActor)
		},
	},
	"UpdateMemoryRequest": {
		tool:       "update_memory",
		seedTarget: true,
		request: func(in fixtureInput) proto.Message {
			return &engramv1.UpdateMemoryRequest{Id: in.target, Content: "valid", UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"content"}}}
		},
		args: func(in fixtureInput) map[string]any {
			return map[string]any{"id": in.target, "content": "valid"}
		},
		send: func(c engramv1connect.EngramServiceClient, m proto.Message) error {
			return callWrite(context.Background(), c.UpdateMemory, m.(*engramv1.UpdateMemoryRequest), wireActor)
		},
	},
	"DeleteMemoryRequest": {
		tool:       "delete_memory",
		seedTarget: true,
		request: func(in fixtureInput) proto.Message {
			return &engramv1.DeleteMemoryRequest{Id: in.target}
		},
		args: func(in fixtureInput) map[string]any {
			return map[string]any{"id": in.target}
		},
		send: func(c engramv1connect.EngramServiceClient, m proto.Message) error {
			return callWrite(context.Background(), c.DeleteMemory, m.(*engramv1.DeleteMemoryRequest), wireActor)
		},
	},
	"SetVisibilityRequest": {
		tool:       "set_visibility",
		seedTarget: true,
		request: func(in fixtureInput) proto.Message {
			return &engramv1.SetVisibilityRequest{Id: in.target, Visibility: engramv1.Visibility_VISIBILITY_SHARED}
		},
		args: func(in fixtureInput) map[string]any {
			return map[string]any{"id": in.target, "shared": true}
		},
		send: func(c engramv1connect.EngramServiceClient, m proto.Message) error {
			return callWrite(context.Background(), c.SetVisibility, m.(*engramv1.SetVisibilityRequest), wireActor)
		},
	},
	"StoreDiscoveryRequest": {
		tool:      "store_discovery",
		discovery: true,
		request: func(in fixtureInput) proto.Message {
			return &engramv1.StoreDiscoveryRequest{Content: "valid", Kind: "fact", Scope: in.scope, Citations: validCitations()}
		},
		args: func(in fixtureInput) map[string]any {
			return map[string]any{"content": "valid", "kind": "fact", "scope": in.scope, "citations": validCitationArgs()}
		},
		send: func(c engramv1connect.EngramServiceClient, m proto.Message) error {
			return callWrite(context.Background(), c.StoreDiscovery, m.(*engramv1.StoreDiscoveryRequest), wireActor)
		},
	},
}

// fieldEdit is one value for one field: a string, or a list of copies of the
// fixture's first element.
type fieldEdit struct {
	text  string
	list  bool
	items int
}

func (e fieldEdit) applyProto(m protoreflect.Message, path []protoreflect.FieldDescriptor) {
	for _, fd := range path[:len(path)-1] {
		m = m.Get(fd).List().Get(0).Message()
	}
	fd := path[len(path)-1]
	if !e.list {
		m.Set(fd, protoreflect.ValueOfString(e.text))
		return
	}
	list := m.Mutable(fd).List()
	first := proto.Clone(list.Get(0).Message().Interface())
	list.Truncate(0)
	for range e.items {
		list.Append(protoreflect.ValueOfMessage(proto.Clone(first).ProtoReflect()))
	}
}

func (e fieldEdit) applyArgs(args map[string]any, path []string) {
	for _, name := range path[:len(path)-1] {
		args = args[name].([]map[string]any)[0]
	}
	name := path[len(path)-1]
	if !e.list {
		args[name] = e.text
		return
	}
	args[name] = slices.Repeat(args[name].([]map[string]any)[:1], e.items)
}

// declaredRule is one rule value on one field of a request message.
type declaredRule struct {
	message    protoreflect.Name
	path       []protoreflect.FieldDescriptor
	id         string
	value      protoreflect.Value
	ignoreZero bool
}

func (r declaredRule) names() []string {
	out := make([]string, len(r.path))
	for i, fd := range r.path {
		out[i] = string(fd.Name())
	}
	return out
}

func (r declaredRule) String() string {
	return fmt.Sprintf("%s.%s %s", r.message, strings.Join(r.names(), "."), r.id)
}

func (r declaredRule) categoryAllowlist() bool {
	return r.id == "string.in" && slices.Equal(r.names(), []string{"category"})
}

// breakEdits returns values that break the rule. It reports false for a rule
// kind it cannot break.
func (r declaredRule) breakEdits() ([]fieldEdit, bool) {
	switch r.id {
	case "string.min_len":
		return []fieldEdit{{text: strings.Repeat("x", int(r.value.Uint())-1)}}, true
	case "string.in":
		edits := []fieldEdit{{text: "engram-not-listed"}}
		if !r.ignoreZero {
			edits = append(edits, fieldEdit{})
		}
		return edits, true
	case "string.prefix":
		return []fieldEdit{{text: "engram-wrong-prefix"}}, true
	case "string.max_bytes":
		return []fieldEdit{{text: strings.Repeat("x", int(r.value.Uint())+1)}}, true
	case "repeated.min_items":
		return []fieldEdit{{list: true, items: int(r.value.Uint()) - 1}}, true
	case "repeated.max_items":
		return []fieldEdit{{list: true, items: int(r.value.Uint()) + 1}}, true
	}
	return nil, false
}

// boundEdit returns the value exactly at the rule's bound, if it has one.
func (r declaredRule) boundEdit() (fieldEdit, bool) {
	switch r.id {
	case "string.max_bytes":
		return fieldEdit{text: strings.Repeat("x", int(r.value.Uint()))}, true
	case "repeated.max_items":
		return fieldEdit{list: true, items: int(r.value.Uint())}, true
	}
	return fieldEdit{}, false
}

// fieldRules flattens one field's rules into rule ids and values.
func fieldRules(fd protoreflect.FieldDescriptor) (rules map[string]protoreflect.Value, ignoreZero bool) {
	fr, ok := proto.GetExtension(fd.Options(), validate.E_Field).(*validate.FieldRules)
	if !ok || fr == nil {
		return nil, false
	}
	rules = map[string]protoreflect.Value{}
	fr.ProtoReflect().Range(func(kind protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case kind.Name() == "ignore":
			ignoreZero = validate.Ignore(v.Enum()) == validate.Ignore_IGNORE_IF_ZERO_VALUE
		case kind.Message() != nil:
			v.Message().Range(func(rule protoreflect.FieldDescriptor, rv protoreflect.Value) bool {
				rules[string(kind.Name())+"."+string(rule.Name())] = rv
				return true
			})
		default:
			rules[string(kind.Name())] = v
		}
		return true
	})
	return rules, ignoreZero
}

// walkRules lists the field rules of md and of the messages nested in it.
func walkRules(root protoreflect.Name, md protoreflect.MessageDescriptor, prefix []protoreflect.FieldDescriptor) []declaredRule {
	var out []declaredRule
	fields := md.Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		path := append(slices.Clone(prefix), fd)
		rules, ignoreZero := fieldRules(fd)
		for id, v := range rules {
			out = append(out, declaredRule{message: root, path: path, id: id, value: v, ignoreZero: ignoreZero})
		}
		if fd.Message() != nil && fd.Message().FullName().Parent() == md.FullName().Parent() {
			out = append(out, walkRules(root, fd.Message(), path)...)
		}
	}
	return out
}

func messageRules(md protoreflect.MessageDescriptor) []string {
	mr, ok := proto.GetExtension(md.Options(), validate.E_Message).(*validate.MessageRules)
	if !ok || mr == nil {
		return nil
	}
	ids := make([]string, 0, len(mr.GetCel()))
	for _, c := range mr.GetCel() {
		ids = append(ids, c.GetId())
	}
	return ids
}

// requestMessages returns each EngramService method's request message.
func requestMessages() []protoreflect.MessageDescriptor {
	methods := engramv1.File_engram_v1_engram_proto.Services().ByName("EngramService").Methods()
	out := make([]protoreflect.MessageDescriptor, 0, methods.Len())
	for i := range methods.Len() {
		out = append(out, methods.Get(i).Input())
	}
	return out
}

// toolSchemaHas reports whether a tool's input schema holds a field path,
// following a list into its items.
func toolSchemaHas(schema any, path []string) bool {
	for _, name := range path {
		m, ok := schema.(map[string]any)
		if !ok {
			return false
		}
		if items, ok := m["items"].(map[string]any); ok {
			m = items
		}
		props, ok := m["properties"].(map[string]any)
		if !ok {
			return false
		}
		if schema, ok = props[name]; !ok {
			return false
		}
	}
	return true
}

// derivedSet is what the walk over the request messages produced.
type derivedSet struct {
	rules    []declaredRule
	fixtures []protoreflect.Name
}

// deriveRules walks the request messages and fails on anything the test
// cannot cover. A field that the matching tool lacks has only one lane, so it
// gets no rejection case.
func (l *lanes) deriveRules(t *testing.T) derivedSet {
	t.Helper()
	tools, err := l.cs.ListTools(l.mcpCtx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	schemas := map[string]any{}
	for _, tool := range tools.Tools {
		schemas[tool.Name] = tool.InputSchema
	}
	var set derivedSet
	for _, md := range requestMessages() {
		rules := walkRules(md.Name(), md, nil)
		cel := messageRules(md)
		if len(rules) == 0 && len(cel) == 0 {
			continue
		}
		f, ok := parityFixtures[md.Name()]
		if !ok {
			t.Errorf("%s is a constrained request message with no fixture", md.Name())
			continue
		}
		set.fixtures = append(set.fixtures, md.Name())
		for _, id := range cel {
			if _, ok := surfaces.RuleByID(id); !ok && !slices.Contains(connectOnlyMessageRules, id) {
				t.Errorf("%s: message rule %s is not a surface rule or a recorded Connect-only rule", md.Name(), id)
			}
		}
		for _, r := range rules {
			if !toolSchemaHas(schemas[f.tool], r.names()) {
				t.Logf("%s: Connect-only field, no rejection case", r)
				continue
			}
			if _, ok := r.breakEdits(); !ok {
				t.Errorf("%s: the test has no way to break this rule", r)
				continue
			}
			set.rules = append(set.rules, r)
		}
	}
	slices.SortFunc(set.rules, func(a, b declaredRule) int { return strings.Compare(a.String(), b.String()) })
	slices.Sort(set.fixtures)
	return set
}

// caseRun sends one case to both lanes, each on a seeded target of its own.
type caseRun struct {
	l   *lanes
	run string
	n   int
}

func (c *caseRun) send(t *testing.T, name protoreflect.Name, edit func(m protoreflect.Message, args map[string]any)) (mcpOut, connOut outcome) {
	t.Helper()
	c.n++
	f := parityFixtures[name]
	scope := fmt.Sprintf("model-derived:%s:%d", c.run, c.n)
	if f.discovery {
		scope = fmt.Sprintf("discovery:repo:model-derived/%s/%d", c.run, c.n)
	}
	defer c.l.cleanup(scope)
	var mcpTarget, connTarget string
	if f.seedTarget {
		mcpTarget, connTarget = c.l.seed(t, scope)
	}
	args := f.args(fixtureInput{scope: scope, target: mcpTarget, l: c.l})
	msg := f.request(fixtureInput{scope: scope, target: connTarget, l: c.l})
	if edit != nil {
		edit(msg.ProtoReflect(), args)
	}
	return c.l.callMCP(t, f.tool, args), connectOutcome(f.send(c.l.wire, msg))
}

// testDerivedParity runs the cases built from the declared constraints. The
// MCP lane is the oracle for each case.
func testDerivedParity(t *testing.T, l *lanes, hints []string) {
	set := l.deriveRules(t)
	if len(set.rules) == 0 || len(set.fixtures) == 0 {
		t.Fatal("the walk over the request messages derived no case")
	}
	run := &caseRun{l: l, run: uuid.NewString()}

	t.Run("A value that meets a constraint", func(t *testing.T) {
		for _, name := range set.fixtures {
			t.Run(string(name), func(t *testing.T) {
				m, k := run.send(t, name, nil)
				if !m.ok || !k.ok {
					t.Fatalf("a valid request was rejected: MCP %q, Connect %q", m.text, k.text)
				}
			})
		}
	})

	t.Run("A value at a declared bound", func(t *testing.T) {
		for _, r := range set.rules {
			edit, ok := r.boundEdit()
			if !ok {
				continue
			}
			t.Run(r.String(), func(t *testing.T) {
				m, k := run.send(t, r.message, func(msg protoreflect.Message, args map[string]any) {
					edit.applyProto(msg, r.path)
					edit.applyArgs(args, r.names())
				})
				if !m.ok || !k.ok {
					t.Fatalf("a value at the bound was rejected: MCP %q, Connect %q", m.text, k.text)
				}
			})
		}
	})

	t.Run("A value that breaks a constraint", func(t *testing.T) {
		for _, r := range set.rules {
			if r.categoryAllowlist() {
				continue
			}
			edits, _ := r.breakEdits()
			for i, edit := range edits {
				t.Run(fmt.Sprintf("%s %d", r, i), func(t *testing.T) {
					m, k := run.send(t, r.message, func(msg protoreflect.Message, args map[string]any) {
						edit.applyProto(msg, r.path)
						edit.applyArgs(args, r.names())
					})
					if m.ok || k.ok {
						t.Fatalf("%s: MCP ok=%v (%q), Connect ok=%v (%q), want both rejected", r, m.ok, m.text, k.ok, k.text)
					}
					if m.field != k.field || m.hint != k.hint {
						t.Fatalf("%s: MCP field=%s hint=%s (%q), Connect field=%s hint=%s (%q)", r, m.field, m.hint, m.text, k.field, k.hint, k.text)
					}
					if !slices.Contains(hints, k.hint) {
						t.Fatalf("%s: hint %q is not a HintCode constant", r, k.hint)
					}
					if edit.text != "" && (strings.Contains(m.text, edit.text) || strings.Contains(k.text, edit.text)) {
						t.Fatalf("%s: a message echoes the rejected value", r)
					}
				})
			}
		}
	})

	t.Run("The category allowlist exception", func(t *testing.T) {
		n := 0
		for _, r := range set.rules {
			if !r.categoryAllowlist() {
				continue
			}
			n++
			edit := fieldEdit{text: "engram-not-listed"}
			t.Run(r.String(), func(t *testing.T) {
				m, k := run.send(t, r.message, func(msg protoreflect.Message, args map[string]any) {
					edit.applyProto(msg, r.path)
					edit.applyArgs(args, r.names())
				})
				if !m.ok {
					t.Fatalf("MCP rejected an unlisted category: %q", m.text)
				}
				if k.ok || k.field != "category" || k.hint != string(HintEnum) {
					t.Fatalf("Connect = %+v, want field=category hint=enum", k)
				}
			})
		}
		if n == 0 {
			t.Fatal("no category allowlist rule was derived")
		}
	})

	t.Run("Declared surface rules", func(t *testing.T) {
		testSurfaceRuleParity(t, l, run)
	})
}

// surfaceBreak is a request that breaks one surface rule. tool is empty when
// the rule applies to the Connect lane alone.
type surfaceBreak struct {
	tool    string
	args    func(in fixtureInput) map[string]any
	connect func(l *lanes, in fixtureInput) error
}

func scheduleOver(in fixtureInput, mut func(*engramv1.ScheduleMemoryRequest)) error {
	req := &engramv1.ScheduleMemoryRequest{Content: "valid", Scope: in.scope, Source: "user-said", Category: "decision"}
	mut(req)
	return callWrite(context.Background(), in.l.wire.ScheduleMemory, req, wireActor)
}

var surfaceBreaks = map[string]surfaceBreak{
	surfaces.RuleScopeRequiredUnlessCrossSpine: {
		tool: "list_memory",
		args: func(fixtureInput) map[string]any { return map[string]any{} },
		connect: func(l *lanes, _ fixtureInput) error {
			return callWrite(context.Background(), l.wire.ListMemories, &engramv1.ListMemoriesRequest{CursorMode: true}, wireActor)
		},
	},
	surfaces.RuleScheduleWindowAtLeastOne: {
		tool: "schedule_memory",
		args: func(in fixtureInput) map[string]any { return writeArgs(in) },
		connect: func(_ *lanes, in fixtureInput) error {
			return scheduleOver(in, func(*engramv1.ScheduleMemoryRequest) {})
		},
	},
	surfaces.RuleDiscoveryNotSchedulable: {
		tool: "schedule_memory",
		args: func(in fixtureInput) map[string]any {
			at := in.l.now.Add(time.Hour)
			return withArgs(writeArgs(in), map[string]any{"category": "discovery", "not_after": rfc(&at)})
		},
		connect: func(_ *lanes, in fixtureInput) error {
			return scheduleOver(in, func(r *engramv1.ScheduleMemoryRequest) {
				r.Category, r.NotAfter = "discovery", timestamppb.New(in.l.now.Add(time.Hour))
			})
		},
	},
	surfaces.RuleWindowOrdering: {
		tool: "schedule_memory",
		args: func(in fixtureInput) map[string]any {
			early, late := in.l.now.Add(time.Hour), in.l.now.Add(2*time.Hour)
			return withArgs(writeArgs(in), map[string]any{"not_before": rfc(&late), "not_after": rfc(&early)})
		},
		connect: func(_ *lanes, in fixtureInput) error {
			return scheduleOver(in, func(r *engramv1.ScheduleMemoryRequest) {
				r.NotBefore, r.NotAfter = timestamppb.New(in.l.now.Add(2*time.Hour)), timestamppb.New(in.l.now.Add(time.Hour))
			})
		},
	},
	surfaces.RulePagingMutuallyExclusive: {
		connect: func(l *lanes, in fixtureInput) error {
			req := &engramv1.ListMemoriesRequest{Scope: in.scope, CursorMode: true, Offset: 1}
			return callWrite(context.Background(), l.wire.ListMemories, req, wireActor)
		},
	},
}

// ruleOnRequest reports whether a request message carries all of a rule's
// fields.
func ruleOnRequest(rule surfaces.ConditionalRule) bool {
	return slices.ContainsFunc(requestMessages(), func(md protoreflect.MessageDescriptor) bool {
		for _, f := range rule.Fields {
			if md.Fields().ByName(protoreflect.Name(f)) == nil {
				return false
			}
		}
		return true
	})
}

func testSurfaceRuleParity(t *testing.T, l *lanes, run *caseRun) {
	for _, rule := range surfaces.Rules() {
		if !ruleOnRequest(rule) {
			t.Logf("%s: no request message carries its fields, no case", rule.ID)
			continue
		}
		b, ok := surfaceBreaks[rule.ID]
		if !ok {
			t.Errorf("%s: the test has no request that breaks this rule", rule.ID)
			continue
		}
		t.Run(rule.ID, func(t *testing.T) {
			run.n++
			scope := fmt.Sprintf("model-derived:%s:%d", run.run, run.n)
			defer l.cleanup(scope)
			in := fixtureInput{scope: scope, l: l}
			wantField := strings.Join(rule.Fields, ",")
			got := map[string]outcome{"Connect": connectOutcome(b.connect(l, in))}
			if b.tool != "" {
				got["MCP"] = l.callMCP(t, b.tool, b.args(in))
			}
			for lane, o := range got {
				if o.ok || o.field != wantField || o.hint != rule.Hint {
					t.Errorf("%s = %+v, want field=%s hint=%s", lane, o, wantField, rule.Hint)
				}
			}
		})
	}
}
