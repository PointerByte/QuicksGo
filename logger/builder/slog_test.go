// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/PointerByte/QuicksGo/logger/formatter"
	viperdata "github.com/PointerByte/QuicksGo/logger/viperData"
	"github.com/spf13/viper"
)

type errWriter struct {
	err error
}

func (w *errWriter) Write(_ []byte) (int, error) {
	return 0, w.err
}

type failOnNthWrite struct {
	n     int
	count int
	err   error
	buf   bytes.Buffer
}

func (w *failOnNthWrite) Write(p []byte) (int, error) {
	w.count++
	if w.count == w.n {
		return 0, w.err
	}
	return w.buf.Write(p)
}

func resetBuilderViper() {
	viper.Reset()
	viperdata.ResetViperDataSingleton()
}

func newTestCtx() *Context {
	ctx := New(context.Background())

	ctx.Set(traceIDKey, "trace-123")
	ctx.Set(detailsKey, formatter.KibanaData{
		System:   "loan-service",
		Client:   "mobile-app",
		Method:   "POST",
		Protocol: "HTTP",
		Path:     "/loan/simulate",
	})
	services := make([]formatter.Service, 0)
	ctx.Set(servicesKey, &services)

	return ctx
}

func TestNewHandler(t *testing.T) {
	h1 := newHandler(slog.LevelInfo, &bytes.Buffer{})
	if h1 == nil {
		t.Fatal("newHandler returned nil")
	}
	if h1.level != slog.LevelInfo {
		t.Fatalf("level = %v, want %v", h1.level, slog.LevelInfo)
	}
	if h1.w == nil {
		t.Fatal("writer is nil")
	}
	if len(h1.handlers) != 0 {
		t.Fatalf("handlers len = %d, want 0", len(h1.handlers))
	}

	dummy := slog.NewTextHandler(&bytes.Buffer{}, nil)
	h2 := newHandler(slog.LevelDebug, &bytes.Buffer{}, dummy)
	if len(h2.handlers) != 1 {
		t.Fatalf("handlers len = %d, want 1", len(h2.handlers))
	}
}

func TestJSONHandler_Enabled(t *testing.T) {
	h := newHandler(slog.LevelInfo, &bytes.Buffer{})

	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("Enabled(debug) = true, want false")
	}
	if !h.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("Enabled(info) = false, want true")
	}
	if !h.Enabled(context.Background(), slog.LevelWarn) {
		t.Fatal("Enabled(warn) = false, want true")
	}
}

func TestJSONHandler_Handle_JSON(t *testing.T) {
	resetBuilderViper()
	t.Cleanup(resetBuilderViper)

	viper.Set(string(viperdata.LoggerFormatDateAtribute), "2006-01-02T15:04:05.000")
	viper.Set(string(viperdata.LoggerFormatterAtribute), "json")
	viper.Set(string(viperdata.AppAtribute), "test-app")

	buf := &bytes.Buffer{}
	h := newHandler(slog.LevelDebug, buf)
	ctx := newTestCtx()

	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "hello-json", 0)

	if err := h.Handle(ctx, rec); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("output must end with newline, got %q", out)
	}

	line := strings.TrimSpace(out)

	var decoded map[string]any
	if err := json.Unmarshal([]byte(line), &decoded); err != nil {
		t.Fatalf("invalid json output: %v\noutput=%s", err, line)
	}

	if decoded["message"] != "hello-json" {
		t.Fatalf("message = %#v, want %#v", decoded["message"], "hello-json")
	}
	if decoded["traceID"] != "trace-123" {
		t.Fatalf("traceID = %#v, want %#v", decoded["traceID"], "trace-123")
	}

	detailsAny, ok := decoded["details"]
	if !ok {
		t.Fatal("details field not found")
	}

	details, ok := detailsAny.(map[string]any)
	if !ok {
		t.Fatalf("details has unexpected type %T", detailsAny)
	}

	if details["system"] != "loan-service" {
		t.Fatalf("details.system = %#v, want %#v", details["system"], "loan-service")
	}
	if details["client"] != "mobile-app" {
		t.Fatalf("details.client = %#v, want %#v", details["client"], "mobile-app")
	}
	if details["method"] != "POST" {
		t.Fatalf("details.method = %#v, want %#v", details["method"], "POST")
	}
	if details["protocol"] != "HTTP" {
		t.Fatalf("details.protocol = %#v, want %#v", details["protocol"], "HTTP")
	}
	if details["path"] != "/loan/simulate" {
		t.Fatalf("details.path = %#v, want %#v", details["path"], "/loan/simulate")
	}

	servicesAny, ok := decoded["services"]
	if !ok {
		t.Fatal("services field not found")
	}
	services, ok := servicesAny.([]any)
	if !ok {
		t.Fatalf("services has unexpected type %T", servicesAny)
	}
	if len(services) != 0 {
		t.Fatalf("services len = %d, want 0", len(services))
	}
}

func TestJSONHandler_Handle_TextFallback(t *testing.T) {
	resetBuilderViper()
	t.Cleanup(resetBuilderViper)

	viper.Set(string(viperdata.LoggerFormatDateAtribute), "2006-01-02T15:04:05.000")
	viper.Set(string(viperdata.LoggerFormatterAtribute), "text")
	viper.Set(string(viperdata.AppAtribute), "test-app")

	buf := &bytes.Buffer{}
	h := newHandler(slog.LevelDebug, buf)
	ctx := newTestCtx()

	rec := slog.NewRecord(time.Now(), slog.LevelWarn, "hello-text", 0)

	if err := h.Handle(ctx, rec); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("output must end with newline, got %q", out)
	}
	line := strings.TrimSpace(out)

	if !strings.Contains(line, "hello-text") {
		t.Fatalf("output missing message: %s", line)
	}
	if !strings.Contains(line, "trace-123") {
		t.Fatalf("output missing trace id: %s", line)
	}
}

func TestJSONHandler_Handle_PanicsOnFormatterError(t *testing.T) {
	resetBuilderViper()
	t.Cleanup(resetBuilderViper)

	viper.Set(string(viperdata.LoggerFormatDateAtribute), "2006-01-02T15:04:05.000")
	viper.Set(string(viperdata.LoggerFormatterAtribute), "{{if}")
	viper.Set(string(viperdata.AppAtribute), "test-app")

	h := newHandler(slog.LevelDebug, &bytes.Buffer{})
	ctx := newTestCtx()
	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "boom", 0)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic but none occurred")
		}
	}()

	_ = h.Handle(ctx, rec)
}

func TestJSONHandler_Handle_PanicsOnWriteError_JSON(t *testing.T) {
	resetBuilderViper()
	t.Cleanup(resetBuilderViper)

	viper.Set(string(viperdata.LoggerFormatDateAtribute), "2006-01-02T15:04:05.000")
	viper.Set(string(viperdata.LoggerFormatterAtribute), "json")
	viper.Set(string(viperdata.AppAtribute), "test-app")

	h := newHandler(slog.LevelDebug, &errWriter{err: errors.New("write failed")})
	ctx := newTestCtx()
	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "write-json", 0)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic but none occurred")
		}
	}()

	_ = h.Handle(ctx, rec)
}

func TestJSONHandler_Handle_PanicsOnWriteError_TextFallback(t *testing.T) {
	resetBuilderViper()
	t.Cleanup(resetBuilderViper)

	viper.Set(string(viperdata.LoggerFormatDateAtribute), "2006-01-02T15:04:05.000")
	viper.Set(string(viperdata.LoggerFormatterAtribute), "text")
	viper.Set(string(viperdata.AppAtribute), "test-app")

	h := newHandler(slog.LevelDebug, &errWriter{err: errors.New("write failed")})
	ctx := newTestCtx()
	rec := slog.NewRecord(time.Now(), slog.LevelWarn, "write-text", 0)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic but none occurred")
		}
	}()

	_ = h.Handle(ctx, rec)
}

func TestJSONHandler_writeData(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		buf := &bytes.Buffer{}
		h := &jsonHandler{w: buf}

		if err := h.writeData([]byte(`{"ok":true}`)); err != nil {
			t.Fatalf("writeData() error = %v", err)
		}
		if got := buf.String(); got != "{\"ok\":true}\n" {
			t.Fatalf("writeData() = %q, want %q", got, "{\"ok\":true}\n")
		}
	})

	t.Run("first write error", func(t *testing.T) {
		h := &jsonHandler{w: &errWriter{err: errors.New("first write")}}

		if err := h.writeData([]byte(`{"ok":true}`)); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("second write error", func(t *testing.T) {
		w := &failOnNthWrite{n: 2, err: errors.New("newline write")}
		h := &jsonHandler{w: w}

		if err := h.writeData([]byte(`{"ok":true}`)); err == nil {
			t.Fatal("expected error, got nil")
		}
		if got := w.buf.String(); got != "{\"ok\":true}" {
			t.Fatalf("buffer before newline error = %q", got)
		}
	})
}

func TestJSONHandler_WithAttrs(t *testing.T) {
	orig := &jsonHandler{
		level: slog.LevelInfo,
		w:     &bytes.Buffer{},
		attrs: []slog.Attr{slog.String("a", "1")},
	}

	got := orig.WithAttrs([]slog.Attr{slog.String("b", "2")})
	clone, ok := got.(*jsonHandler)
	if !ok {
		t.Fatalf("WithAttrs() type = %T, want *jsonHandler", got)
	}

	if len(orig.attrs) != 1 {
		t.Fatalf("original attrs len = %d, want 1", len(orig.attrs))
	}
	if len(clone.attrs) != 2 {
		t.Fatalf("clone attrs len = %d, want 2", len(clone.attrs))
	}
	if clone.attrs[0].Key != "a" || clone.attrs[1].Key != "b" {
		t.Fatalf("unexpected attrs keys: %#v", clone.attrs)
	}
}

func TestJSONHandler_WithGroup(t *testing.T) {
	orig := &jsonHandler{
		level:  slog.LevelInfo,
		w:      &bytes.Buffer{},
		groups: []string{"root"},
	}

	got := orig.WithGroup("child")
	clone, ok := got.(*jsonHandler)
	if !ok {
		t.Fatalf("WithGroup() type = %T, want *jsonHandler", got)
	}

	if len(orig.groups) != 1 {
		t.Fatalf("original groups len = %d, want 1", len(orig.groups))
	}
	if len(clone.groups) != 2 {
		t.Fatalf("clone groups len = %d, want 2", len(clone.groups))
	}
	if clone.groups[0] != "root" || clone.groups[1] != "child" {
		t.Fatalf("unexpected groups: %#v", clone.groups)
	}
}
