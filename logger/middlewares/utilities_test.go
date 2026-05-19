// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PointerByte/GoForge/logger/builder"
	"github.com/PointerByte/GoForge/logger/formatter"
	"github.com/PointerByte/GoForge/logger/middlewares/common"
	viperdata "github.com/PointerByte/GoForge/logger/viperData"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func TestDisableBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

	if _, ok := c.Get(common.DisableRequestBodyKey); ok {
		t.Fatalf("did not expect %q before calling DisableBody", common.DisableRequestBodyKey)
	}
	if _, ok := c.Get(common.DisableResponseBodyKey); ok {
		t.Fatalf("did not expect %q before calling DisableBody", common.DisableResponseBodyKey)
	}

	DisableBody(c, true, false)

	gotRequest, ok := c.Get(common.DisableRequestBodyKey)
	if !ok {
		t.Fatalf("expected %q to be set", common.DisableRequestBodyKey)
	}
	disabledRequest, ok := gotRequest.(bool)
	if !ok {
		t.Fatalf("%q type = %T, want bool", common.DisableRequestBodyKey, gotRequest)
	}
	if !disabledRequest {
		t.Fatalf("%q = %v, want true", common.DisableRequestBodyKey, disabledRequest)
	}

	gotResponse, ok := c.Get(common.DisableResponseBodyKey)
	if !ok {
		t.Fatalf("expected %q to be set", common.DisableResponseBodyKey)
	}
	disabledResponse, ok := gotResponse.(bool)
	if !ok {
		t.Fatalf("%q type = %T, want bool", common.DisableResponseBodyKey, gotResponse)
	}
	if disabledResponse {
		t.Fatalf("%q = %v, want false", common.DisableResponseBodyKey, disabledResponse)
	}
}

func TestDisableTraceBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	viper.Reset()
	viperdata.ResetViperDataSingleton()
	t.Cleanup(func() {
		viper.Reset()
		viperdata.ResetViperDataSingleton()
	})

	viper.Set(string(viperdata.AppAtribute), "test-service")
	viper.Set(string(viperdata.LoggerModeTestAtribute), false)
	viper.Set(string(viperdata.LoggerIgnoredHeadersAtribute), []string{})

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
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

			ctxLogger := builder.New(c.Request.Context())
			c.Request = c.Request.WithContext(ctxLogger)

			if _, ok := ctxLogger.Get(string(common.DisableTraceRequestBodyKey)); ok {
				t.Fatalf("did not expect %q before calling DisableTraceBody", common.DisableTraceRequestBodyKey)
			}
			if _, ok := ctxLogger.Get(string(common.DisableTraceResponseBodyKey)); ok {
				t.Fatalf("did not expect %q before calling DisableTraceBody", common.DisableTraceResponseBodyKey)
			}

			DisableTraceBody(c, tt.disableRequestBody, tt.disableResponseBody)

			gotRequestFlag, ok := ctxLogger.Get(string(common.DisableTraceRequestBodyKey))
			if !ok || gotRequestFlag != tt.disableRequestBody {
				t.Fatalf("%q = %#v, want %#v", common.DisableTraceRequestBodyKey, gotRequestFlag, tt.disableRequestBody)
			}
			gotResponseFlag, ok := ctxLogger.Get(string(common.DisableTraceResponseBodyKey))
			if !ok || gotResponseFlag != tt.disableResponseBody {
				t.Fatalf("%q = %#v, want %#v", common.DisableTraceResponseBodyKey, gotResponseFlag, tt.disableResponseBody)
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
