package logger

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/trace"
)

// ---- Helpers ----

func mustPanic(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic, but function did not panic")
		}
	}()
	f()
}

// ---- Tests ----

func TestSetOtelIds_Disabled(t *testing.T) {
	t.Parallel()

	// OTLP disabled path
	viper.Set("otlp.enable", false)
	ctx := New(context.Background())
	setOtelIds(ctx)

	// Should still insert empty keys
	if v, ok := ctx.Get(TraceIdOtel); !ok {
		t.Fatalf("expected empty trace_id, got %v", v)
	}
	if v, ok := ctx.Get(SpanIdOtel); !ok {
		t.Fatalf("expected empty span_id, got %v", v)
	}
}

func TestSetOtelIds_EnabledInvalidSpan(t *testing.T) {
	t.Parallel()

	viper.Set("otlp.enable", true)
	ctx := New(context.Background())
	setOtelIds(ctx)

	if v, ok := ctx.Get(TraceIdOtel); !ok {
		t.Fatalf("expected empty trace_id with invalid span, got %v", v)
	}
	if v, ok := ctx.Get(SpanIdOtel); !ok {
		t.Fatalf("expected empty span_id with invalid span, got %v", v)
	}
}

func TestSetOtelIds_EnabledValidSpan(t *testing.T) {
	t.Parallel()

	viper.Set("otlp.enable", true)
	traceID, _ := trace.TraceIDFromHex("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	spanID, _ := trace.SpanIDFromHex("bbbbbbbbbbbbbbbb")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})

	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	ctxLogger := New(ctx)
	setOtelIds(ctxLogger)

	if got, _ := ctxLogger.Get(TraceIdOtel); got != traceID.String() {
		t.Fatalf("expected trace_id=%v, got %v", traceID, got)
	}
	if got, _ := ctxLogger.Get(SpanIdOtel); got != spanID.String() {
		t.Fatalf("expected span_id=%v, got %v", spanID, got)
	}
}

func TestLoggerContext_All(t *testing.T) {
	t.Parallel()

	base := context.Background()
	c := New(base)
	c.Set("user", "alice")

	if v, ok := c.Get("user"); !ok || v != "alice" {
		t.Fatalf("expected Get('user')='alice', got (%v, %v)", v, ok)
	}

	if v := c.MustGet("user"); v != "alice" {
		t.Fatalf("expected MustGet('user')='alice', got %v", v)
	}

	mustPanic(t, func() { _ = c.MustGet("missing") })

	// Latency should grow
	d1 := c.GetLatency()
	time.Sleep(5 * time.Millisecond)
	d2 := c.GetLatency()
	if d2 <= d1 {
		t.Fatalf("expected latency to increase, got %v <= %v", d2, d1)
	}

	// WithCancel
	child, cancel := c.WithCancel()
	defer cancel()
	child.Set("id", 99)
	if _, ok := c.Get("id"); !ok {
		t.Fatalf("expected shared fields between parent and child")
	}
	cancel()
	select {
	case <-child.Done():
	default:
		t.Fatalf("expected cancelled child context")
	}

	// WithTimeout
	child2, cancel2 := c.WithTimeout(5 * time.Millisecond)
	defer cancel2()
	select {
	case <-child2.Done():
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timeout child did not finish in time")
	}
}
