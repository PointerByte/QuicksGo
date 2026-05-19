// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package grpc

import (
	"context"
	"net/http"
	"testing"

	"github.com/PointerByte/GoForge/logger/builder"
	"github.com/PointerByte/GoForge/logger/formatter"
	"github.com/PointerByte/GoForge/logger/middlewares/common"
	viperdata "github.com/PointerByte/GoForge/logger/viperData"
	"github.com/spf13/viper"
)

func TestDisableBody(t *testing.T) {
	resetGRPCUtilitiesTestState(t)

	tests := []struct {
		name                string
		disableRequestBody  bool
		disableResponseBody bool
	}{
		{
			name:                "disables only request body",
			disableRequestBody:  true,
			disableResponseBody: false,
		},
		{
			name:                "disables only response body",
			disableRequestBody:  false,
			disableResponseBody: true,
		},
		{
			name:                "keeps both bodies",
			disableRequestBody:  false,
			disableResponseBody: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctxLogger := builder.New(context.Background())

			if _, ok := ctxLogger.Get(common.DisableRequestBodyKey); ok {
				t.Fatalf("did not expect %q before calling DisableBody", common.DisableRequestBodyKey)
			}
			if _, ok := ctxLogger.Get(common.DisableResponseBodyKey); ok {
				t.Fatalf("did not expect %q before calling DisableBody", common.DisableResponseBodyKey)
			}

			DisableBody(ctxLogger, tt.disableRequestBody, tt.disableResponseBody)

			requestFlag, ok := ctxLogger.Get(common.DisableRequestBodyKey)
			if !ok || requestFlag != tt.disableRequestBody {
				t.Fatalf("%q = %#v, want %#v", common.DisableRequestBodyKey, requestFlag, tt.disableRequestBody)
			}
			responseFlag, ok := ctxLogger.Get(common.DisableResponseBodyKey)
			if !ok || responseFlag != tt.disableResponseBody {
				t.Fatalf("%q = %#v, want %#v", common.DisableResponseBodyKey, responseFlag, tt.disableResponseBody)
			}
		})
	}
}

func TestDisableTraceBody(t *testing.T) {
	resetGRPCUtilitiesTestState(t)

	tests := []struct {
		name                string
		disableRequestBody  bool
		disableResponseBody bool
		wantRequest         any
		wantResponse        any
	}{
		{
			name:                "disables only trace request body",
			disableRequestBody:  true,
			disableResponseBody: false,
			wantRequest:         nil,
			wantResponse:        "trace-response",
		},
		{
			name:                "disables only trace response body",
			disableRequestBody:  false,
			disableResponseBody: true,
			wantRequest:         "trace-request",
			wantResponse:        nil,
		},
		{
			name:                "keeps both trace bodies",
			disableRequestBody:  false,
			disableResponseBody: false,
			wantRequest:         "trace-request",
			wantResponse:        "trace-response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctxLogger := builder.New(context.Background())

			if _, ok := ctxLogger.Get(string(common.DisableTraceRequestBodyKey)); ok {
				t.Fatalf("did not expect %q before calling DisableTraceBody", common.DisableTraceRequestBodyKey)
			}
			if _, ok := ctxLogger.Get(string(common.DisableTraceResponseBodyKey)); ok {
				t.Fatalf("did not expect %q before calling DisableTraceBody", common.DisableTraceResponseBodyKey)
			}

			DisableTraceBody(ctxLogger, tt.disableRequestBody, tt.disableResponseBody)

			requestFlag, ok := ctxLogger.Get(string(common.DisableTraceRequestBodyKey))
			if !ok || requestFlag != tt.disableRequestBody {
				t.Fatalf("%q = %#v, want %#v", common.DisableTraceRequestBodyKey, requestFlag, tt.disableRequestBody)
			}
			responseFlag, ok := ctxLogger.Get(string(common.DisableTraceResponseBodyKey))
			if !ok || responseFlag != tt.disableResponseBody {
				t.Fatalf("%q = %#v, want %#v", common.DisableTraceResponseBodyKey, responseFlag, tt.disableResponseBody)
			}

			process := &formatter.Service{
				System:   "test-service",
				Process:  "trace-process",
				Code:     http.StatusOK,
				Request:  "trace-request",
				Response: "trace-response",
			}
			ctxLogger.TraceInit(process)
			ctxLogger.TraceEnd(process)

			if process.Request != tt.wantRequest {
				t.Fatalf("process.Request = %#v, want %#v", process.Request, tt.wantRequest)
			}
			if process.Response != tt.wantResponse {
				t.Fatalf("process.Response = %#v, want %#v", process.Response, tt.wantResponse)
			}
		})
	}
}

func resetGRPCUtilitiesTestState(t *testing.T) {
	t.Helper()
	viper.Reset()
	viperdata.ResetViperDataSingleton()
	t.Cleanup(func() {
		viper.Reset()
		viperdata.ResetViperDataSingleton()
	})

	viper.Set(string(viperdata.AppAtribute), "test-service")
	viper.Set(string(viperdata.LoggerIgnoredHeadersAtribute), []string{})
	viper.Set(string(viperdata.LoggerModeTestAtribute), false)
}
