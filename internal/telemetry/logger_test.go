// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestNewLoggerWritesJSONToStdout(t *testing.T) {
	var buf bytes.Buffer
	lg := newLoggerTo(&buf, Config{LogLevel: "info", LogFormat: "json", LogStdout: true}, nil)
	lg.Info("hello", "tool", "store_memory")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("expected JSON line, got %q (%v)", buf.String(), err)
	}
	if rec["msg"] != "hello" || rec["tool"] != "store_memory" {
		t.Errorf("missing fields: %v", rec)
	}
}

func TestNewLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	lg := newLoggerTo(&buf, Config{LogLevel: "warn", LogFormat: "json", LogStdout: true}, nil)
	lg.Info("suppressed")
	lg.Warn("shown")
	if strings.Contains(buf.String(), "suppressed") {
		t.Error("info should be filtered at warn level")
	}
	if !strings.Contains(buf.String(), "shown") {
		t.Error("warn should pass")
	}
}

func TestSilentProcessGuardForcesStdout(t *testing.T) {
	// stdout disabled AND no OTLP provider => must force stdout on, not go silent.
	var buf bytes.Buffer
	cfg := Config{LogLevel: "info", LogFormat: "json", LogStdout: false} // no endpoint => not enabled
	lg := newLoggerTo(&buf, cfg, nil)
	lg.Warn("must appear")
	if !strings.Contains(buf.String(), "must appear") {
		t.Error("guard must keep stdout when no log sink would otherwise exist")
	}
}

// recHandler is a test-only slog.Handler that records calls for assertions.
type recHandler struct {
	level     slog.Level
	enabled   bool
	records   []slog.Record
	attrs     []slog.Attr
	groups    []string
	handleErr error
}

func (r *recHandler) Enabled(_ context.Context, l slog.Level) bool { return r.enabled && l >= r.level }
func (r *recHandler) Handle(_ context.Context, rec slog.Record) error {
	r.records = append(r.records, rec)
	return r.handleErr
}
func (r *recHandler) WithAttrs(as []slog.Attr) slog.Handler {
	c := *r
	c.attrs = append(append([]slog.Attr(nil), r.attrs...), as...)
	return &c
}
func (r *recHandler) WithGroup(name string) slog.Handler {
	c := *r
	c.groups = append(append([]string(nil), r.groups...), name)
	return &c
}

func TestFanoutHandlerEnabled(t *testing.T) {
	off := &recHandler{enabled: false}
	on := &recHandler{enabled: true, level: slog.LevelInfo}
	f := fanout([]slog.Handler{off, on})

	if !f.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("Enabled must be true when at least one child is enabled")
	}

	allOff := fanout([]slog.Handler{off, off})
	if allOff.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("Enabled must be false when no child is enabled")
	}
}

func TestFanoutHandlerHandleDispatch(t *testing.T) {
	enabled := &recHandler{enabled: true, level: slog.LevelInfo}
	disabled := &recHandler{enabled: false}
	f := fanout([]slog.Handler{enabled, disabled})

	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
	_ = f.Handle(context.Background(), rec)

	if len(enabled.records) != 1 {
		t.Errorf("enabled child should have received 1 record, got %d", len(enabled.records))
	}
	if len(disabled.records) != 0 {
		t.Errorf("disabled child should receive no records, got %d", len(disabled.records))
	}
}

func TestFanoutHandlerHandleJoinsErrors(t *testing.T) {
	err1 := errors.New("handler-1 error")
	err2 := errors.New("handler-2 error")
	h1 := &recHandler{enabled: true, level: slog.LevelInfo, handleErr: err1}
	h2 := &recHandler{enabled: true, level: slog.LevelInfo, handleErr: err2}
	f := fanout([]slog.Handler{h1, h2})

	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	got := f.Handle(context.Background(), rec)
	if got == nil {
		t.Fatal("Handle should return an error when children fail")
	}
	if !errors.Is(got, err1) {
		t.Errorf("result should wrap err1; got %v", got)
	}
	if !errors.Is(got, err2) {
		t.Errorf("result should wrap err2; got %v", got)
	}
}

func TestFanoutHandlerWithAttrs(t *testing.T) {
	h := &recHandler{enabled: true, level: slog.LevelInfo}
	f := fanout([]slog.Handler{h})
	attr := slog.String("key", "val")
	f2 := f.WithAttrs([]slog.Attr{attr})

	fo, ok := f2.(*fanoutHandler)
	if !ok {
		t.Fatal("WithAttrs must return a *fanoutHandler")
	}
	child, ok := fo.handlers[0].(*recHandler)
	if !ok {
		t.Fatal("child must be a *recHandler")
	}
	if len(child.attrs) != 1 || child.attrs[0].Key != "key" {
		t.Errorf("attr not propagated; got %v", child.attrs)
	}
}

func TestFanoutHandlerWithGroup(t *testing.T) {
	h := &recHandler{enabled: true, level: slog.LevelInfo}
	f := fanout([]slog.Handler{h})
	f2 := f.WithGroup("grp")

	fo, ok := f2.(*fanoutHandler)
	if !ok {
		t.Fatal("WithGroup must return a *fanoutHandler")
	}
	child, ok := fo.handlers[0].(*recHandler)
	if !ok {
		t.Fatal("child must be a *recHandler")
	}
	if len(child.groups) != 1 || child.groups[0] != "grp" {
		t.Errorf("group not propagated; got %v", child.groups)
	}
}
