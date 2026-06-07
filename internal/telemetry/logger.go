// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	otellog "go.opentelemetry.io/otel/log"
)

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// NewLogger builds the application logger. When lp is non-nil, log records are
// also bridged to OpenTelemetry via otelslog. When stdout is disabled and there
// is no OTLP log provider, stdout is forced back on so the process is never
// left with no log sink (silent-process guard).
func NewLogger(cfg Config, lp otellog.LoggerProvider) *slog.Logger {
	return newLoggerTo(os.Stdout, cfg, lp)
}

func newLoggerTo(w io.Writer, cfg Config, lp otellog.LoggerProvider) *slog.Logger {
	level := parseLevel(cfg.LogLevel)
	stdout := cfg.LogStdout
	if !stdout && lp == nil {
		// No sink would remain — force stdout on. Caller logs the degradation.
		stdout = true
	}

	var handlers []slog.Handler
	if stdout {
		opts := &slog.HandlerOptions{Level: level}
		if cfg.LogFormat == "text" {
			handlers = append(handlers, slog.NewTextHandler(w, opts))
		} else {
			handlers = append(handlers, slog.NewJSONHandler(w, opts))
		}
	}
	if lp != nil {
		handlers = append(handlers, otelslog.NewHandler("github.com/seanb4t/engram",
			otelslog.WithLoggerProvider(lp)))
	}

	if len(handlers) == 1 {
		return slog.New(handlers[0])
	}
	return slog.New(fanout(handlers))
}

// fanout dispatches each record to every wrapped handler. Used to write both
// stdout and the OTLP bridge from one *slog.Logger.
type fanoutHandler struct {
	handlers []slog.Handler
}

func fanout(hs []slog.Handler) slog.Handler { return &fanoutHandler{handlers: hs} }

func (f *fanoutHandler) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (f *fanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range f.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (f *fanoutHandler) WithAttrs(as []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithAttrs(as)
	}
	return &fanoutHandler{handlers: next}
}

func (f *fanoutHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithGroup(name)
	}
	return &fanoutHandler{handlers: next}
}
