// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/PointerByte/QuicksGo/logger/formatter"
	"github.com/PointerByte/QuicksGo/logger/utilities"
	viperdata "github.com/PointerByte/QuicksGo/logger/viperData"
	"github.com/spf13/viper"
)

func resetViperForTest() {
	viperdata.ResetViperDataSingleton()
	DisableModeTest()
}

func TestConvertStr(t *testing.T) {
	t.Run("string input", func(t *testing.T) {
		got := convertStr("plain-text")
		if got != "plain-text" {
			t.Fatalf("expected plain-text, got %q", got)
		}
	})

	t.Run("map input marshals to json", func(t *testing.T) {
		got := convertStr(map[string]any{
			"foo": "bar",
			"num": 123,
		})

		if !strings.Contains(got, `"foo":"bar"`) {
			t.Fatalf("expected marshaled json to contain foo, got %q", got)
		}
		if !strings.Contains(got, `"num":123`) {
			t.Fatalf("expected marshaled json to contain num, got %q", got)
		}
	})

	t.Run("struct input marshals to json", func(t *testing.T) {
		type payload struct {
			Name string `json:"name"`
		}

		got := convertStr(payload{Name: "sam"})
		if !strings.Contains(got, `"name":"sam"`) {
			t.Fatalf("expected marshaled json, got %q", got)
		}
	})
}

func helperTraceCaller() (string, int) {
	return utilities.TraceCaller(1)
}

func TestTraceCaller(t *testing.T) {
	funcName, line := helperTraceCaller()

	if funcName == "" || funcName == "unknown" {
		t.Fatalf("expected valid function name, got %q", funcName)
	}
	if !strings.Contains(funcName, "helperTraceCaller") {
		t.Fatalf("expected function name to contain helperTraceCaller, got %q", funcName)
	}
	if line <= 0 {
		t.Fatalf("expected line > 0, got %d", line)
	}
}

func TestClassifyStatus(t *testing.T) {
	tests := []struct {
		name     string
		code     int64
		initial  formatter.Status
		expected formatter.Status
	}{
		{
			name:     "success 2xx",
			code:     200,
			initial:  formatter.OTHER,
			expected: formatter.SUCCESS,
		},
		{
			name:     "error 4xx",
			code:     404,
			initial:  formatter.SUCCESS,
			expected: formatter.ERROR,
		},
		{
			name:     "error 5xx",
			code:     500,
			initial:  formatter.SUCCESS,
			expected: formatter.ERROR,
		},
		{
			name:     "other 3xx",
			code:     302,
			initial:  formatter.SUCCESS,
			expected: formatter.OTHER,
		},
		{
			name:     "code zero keeps previous status",
			code:     0,
			initial:  formatter.SUCCESS,
			expected: formatter.SUCCESS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			process := &formatter.Service{
				Code:   tt.code,
				Status: tt.initial,
			}

			classifyStatus(process)

			if process.Status != tt.expected {
				t.Fatalf("expected status %q, got %q", tt.expected, process.Status)
			}
		})
	}
}

func TestCustomLogFormat(t *testing.T) {
	resetViperForTest()

	ctx := New(context.Background())

	services := []formatter.Service{
		{
			System:   "unit-test-system",
			Process:  "custom-log-format",
			Status:   formatter.SUCCESS,
			Latency:  15,
			Method:   "GET",
			Protocol: "HTTP/1.1",
			Server:   "localhost",
			Code:     200,
			Path:     "/health",
		},
	}

	ctx.Set(traceIDKey, "trace-id-unit-test")
	ctx.Set(detailsKey, formatter.KibanaData{
		System:   "unit-test-system",
		Method:   "unit-test-method",
		Protocol: "HTTP/1.1",
		Client:   "unit-test-client",
	})
	ctx.Set(servicesKey, &services)

	ctx.startTime = time.Now().Add(-25 * time.Millisecond)

	got := ctx.customLogFormat()
	if got == nil {
		t.Fatal("expected non-nil formatted map")
	}

	if len(services) != 0 {
		t.Fatalf("expected services slice to be reset, got len=%d", len(services))
	}

	if time.Since(ctx.startTime) > time.Second {
		t.Fatal("expected startTime to be refreshed after customLogFormat")
	}
}

func TestTraceInitAndTraceEnd_SuccessFlow(t *testing.T) {
	resetViperForTest()

	ctx := New(context.Background())
	services := make([]formatter.Service, 0)
	ctx.Set(servicesKey, &services)

	process := &formatter.Service{
		System:   "unit-test-system",
		Process:  "trace-success-flow",
		Protocol: "HTTP/1.1",
		Method:   "POST",
		Server:   "api.internal",
		Code:     201,
		Path:     "/v1/resource",
		Request:  map[string]any{"hello": "world"},
		Response: map[string]any{"ok": true},
	}

	ctx.TraceInit(process)
	if process.TimeInit.IsZero() {
		t.Fatal("expected TimeInit to be initialized")
	}

	time.Sleep(2 * time.Millisecond)
	ctx.TraceEnd(process)

	if process.Status != formatter.SUCCESS {
		t.Fatalf("expected SUCCESS, got %q", process.Status)
	}

	if process.Latency < 0 {
		t.Fatalf("expected non-negative latency, got %d", process.Latency)
	}

	if len(services) != 1 {
		t.Fatalf("expected 1 service appended, got %d", len(services))
	}
}

func TestTraceEnd_ErrorAndOtherFlow(t *testing.T) {
	resetViperForTest()

	t.Run("error 4xx", func(t *testing.T) {
		ctx := New(context.Background())
		services := make([]formatter.Service, 0)
		ctx.Set(servicesKey, &services)

		process := &formatter.Service{
			System:  "unit-test-system",
			Process: "trace-error-flow",
			Code:    404,
		}

		ctx.TraceInit(process)
		ctx.TraceEnd(process)

		if process.Status != formatter.ERROR {
			t.Fatalf("expected ERROR, got %q", process.Status)
		}

		if len(services) != 1 {
			t.Fatalf("expected 1 service appended, got %d", len(services))
		}
	})

	t.Run("other 3xx", func(t *testing.T) {
		ctx := New(context.Background())
		services := make([]formatter.Service, 0)
		ctx.Set(servicesKey, &services)

		process := &formatter.Service{
			System:  "unit-test-system",
			Process: "trace-other-flow",
			Code:    302,
		}

		ctx.TraceInit(process)
		ctx.TraceEnd(process)

		if process.Status != formatter.OTHER {
			t.Fatalf("expected OTHER, got %q", process.Status)
		}

		if len(services) != 1 {
			t.Fatalf("expected 1 service appended, got %d", len(services))
		}
	})
}

func TestTraceInitAndTraceEnd_DisableTrace(t *testing.T) {
	resetViperForTest()

	ctx := New(context.Background())
	ctx.disableTrace = true

	services := make([]formatter.Service, 0)
	ctx.Set(servicesKey, &services)

	process := &formatter.Service{
		System:  "unit-test-system",
		Process: "trace-disabled",
		Code:    200,
	}

	ctx.TraceInit(process)
	if process.TimeInit.IsZero() {
		t.Fatal("expected TimeInit to be initialized even when trace is disabled")
	}

	ctx.TraceEnd(process)

	if process.Status != formatter.SUCCESS {
		t.Fatalf("expected SUCCESS, got %q", process.Status)
	}

	if len(services) != 1 {
		t.Fatalf("expected 1 service appended, got %d", len(services))
	}
}

func TestSetTraceID_DisableTrace(t *testing.T) {
	resetViperForTest()

	ctx := New(context.Background())
	ctx.disableTrace = true

	process := &formatter.Service{
		System:  "unit-test-system",
		Process: "set-trace-id-disabled",
		TraceID: "",
	}

	ctx.setTraceID(process)

	if process.TraceID != "" {
		t.Fatalf("expected empty IdTrace when trace is disabled, got %q", process.TraceID)
	}
}

func TestTraceEnd_IgnoreHeaders(t *testing.T) {
	viper.Reset()
	viperdata.ResetViperDataSingleton()
	t.Cleanup(func() {
		viper.Reset()
		viperdata.ResetViperDataSingleton()
	})

	viper.Set(string(viperdata.LoggerIgnoredHeadersAtribute), []string{
		"Authorization",
		"Cookie",
	})

	ctx := New(context.Background())
	services := make([]formatter.Service, 0)
	ctx.Set(servicesKey, &services)

	headers := http.Header{
		"Content-Type":  {"application/json"},
		"X-Trace-Id":    {"abc-123", "def-456"},
		"Authorization": {"Bearer secret"},
		"Cookie":        {"session=123"},
	}

	process := &formatter.Service{
		System:  "unit-test-system",
		Process: "trace-ignore-headers",
		Code:    200,
		Headers: &headers,
	}

	ctx.TraceInit(process)
	time.Sleep(2 * time.Millisecond)
	ctx.TraceEnd(process)

	if process.Status != formatter.SUCCESS {
		t.Fatalf("expected SUCCESS, got %q", process.Status)
	}

	if process.Headers == nil {
		t.Fatal("expected Headers to remain initialized")
	}

	gotHeaders := *process.Headers

	if got := gotHeaders.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/json")
	}

	if got := gotHeaders.Values("X-Trace-Id"); !reflect.DeepEqual(got, []string{"abc-123", "def-456"}) {
		t.Fatalf("X-Trace-Id = %#v, want %#v", got, []string{"abc-123", "def-456"})
	}

	if got := gotHeaders.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q, want empty because it must be ignored", got)
	}

	if got := gotHeaders.Get("Cookie"); got != "" {
		t.Fatalf("Cookie = %q, want empty because it must be ignored", got)
	}

	// Verify deep copy after TraceEnd/ignoreHeaders.
	headers["Content-Type"][0] = "text/plain"
	headers["X-Trace-Id"][0] = "mutated"

	if got := gotHeaders.Get("Content-Type"); got != "application/json" {
		t.Fatalf("after source mutation, Content-Type = %q, want %q", got, "application/json")
	}

	if got := gotHeaders.Values("X-Trace-Id"); !reflect.DeepEqual(got, []string{"abc-123", "def-456"}) {
		t.Fatalf("after source mutation, X-Trace-Id = %#v, want %#v", got, []string{"abc-123", "def-456"})
	}

	if len(services) != 1 {
		t.Fatalf("expected 1 service appended, got %d", len(services))
	}

	if services[0].Headers == nil {
		t.Fatal("expected appended service to include filtered headers")
	}

	appendedHeaders := *services[0].Headers
	if got := appendedHeaders.Get("Authorization"); got != "" {
		t.Fatalf("appended Authorization = %q, want empty because it must be ignored", got)
	}
	if got := appendedHeaders.Get("Cookie"); got != "" {
		t.Fatalf("appended Cookie = %q, want empty because it must be ignored", got)
	}
	if got := appendedHeaders.Get("Content-Type"); got != "application/json" {
		t.Fatalf("appended Content-Type = %q, want %q", got, "application/json")
	}
}

func TestInfoDebugWarnAndError(t *testing.T) {
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	defer slog.SetDefault(oldLogger)

	tests := []struct {
		name       string
		enableTest bool
		infoMsg    string
		debugMsg   string
		warnMsg    string
		err        error
	}{
		{
			name:       "info normal mode",
			enableTest: false,
			infoMsg:    "unit test info log",
		},
		{
			name:       "debug normal mode",
			enableTest: false,
			debugMsg:   "unit test debug log",
		},
		{
			name:       "warn normal mode",
			enableTest: false,
			warnMsg:    "unit test warn log",
		},
		{
			name:       "error normal mode",
			enableTest: false,
			err:        errors.New("unit test error log"),
		},
		{
			name:       "mode test skips logs",
			enableTest: true,
			infoMsg:    "skipped info log",
			debugMsg:   "skipped debug log",
			warnMsg:    "skipped warn log",
			err:        errors.New("skipped error log"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetViperForTest()

			if tt.enableTest {
				EnableModeTest()
				defer DisableModeTest()
			}

			ctx := New(context.Background())
			ctx.Set(detailsKey, formatter.KibanaData{
				System:   "system-from-context",
				Client:   "client-from-context",
				Method:   "method-from-context",
				Protocol: "protocol-from-context",
			})

			process := &formatter.Service{
				System:  "unit-test",
				Process: "test-trace",
				Status:  formatter.SUCCESS,
				Code:    200,
			}

			services := make([]formatter.Service, 0)
			ctx.Set(servicesKey, &services)

			ctx.TraceInit(process)
			defer ctx.TraceEnd(process)

			if tt.infoMsg != "" {
				ctx.Info(tt.infoMsg)
			}

			if tt.debugMsg != "" {
				ctx.Debug(tt.debugMsg)
			}

			if tt.warnMsg != "" {
				ctx.Warn(tt.warnMsg)
			}

			if tt.err != nil {
				ctx.Error(tt.err)
			}

			got, ok := ctx.Get(detailsKey)
			if !ok {
				t.Fatal("detailsKey not found")
			}
			details := got.(formatter.KibanaData)

			if details.System != "system-from-context" {
				t.Fatalf("System = %q, want %q", details.System, "system-from-context")
			}
			if details.Client != "client-from-context" {
				t.Fatalf("Client = %q, want %q", details.Client, "client-from-context")
			}
			if details.Method != "method-from-context" {
				t.Fatalf("Method = %q, want %q", details.Method, "method-from-context")
			}
			if details.Protocol != "protocol-from-context" {
				t.Fatalf("Protocol = %q, want %q", details.Protocol, "protocol-from-context")
			}
		})
	}
}
