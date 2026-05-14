// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package middlewares

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PointerByte/GoForge/logger/builder"
	"github.com/PointerByte/GoForge/logger/formatter"
	viperdata "github.com/PointerByte/GoForge/logger/viperData"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func TestInitLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		traceHeader string
		wantSet     bool
		wantValue   string
	}{
		{
			name:        "Success",
			traceHeader: "abc123-trace",
			wantSet:     true,
			wantValue:   "abc123-trace",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(InitLogger())

			r.GET("/test", func(ctx *gin.Context) {
				ctxLogger := builder.New(ctx.Request.Context())
				if v, ok := ctxLogger.Get(traceIDKey); ok {
					if s, ok := v.(string); ok {
						ctx.String(http.StatusOK, s)
						return
					}
					ctx.String(http.StatusOK, "")
					return
				}
				ctx.String(http.StatusOK, "")
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
			}
		})
	}
}

func TestMiddlewareCaptureBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name                string
		method              string
		body                string
		forceNilRequestBody bool
		wantRequestBody     string
		wantResponseBody    string
	}{
		{
			name:             "capture request and response body",
			method:           http.MethodPost,
			body:             `{"name":"chaos"}`,
			wantRequestBody:  `{"name":"chaos"}`,
			wantResponseBody: `{"message":"ok"}`,
		},
		{
			name:                "capture when request body is nil",
			method:              http.MethodGet,
			forceNilRequestBody: true,
			wantRequestBody:     "",
			wantResponseBody:    `plain-response`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotRequestBody any
			var gotResponseBody any
			var handlerSawBody string

			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Next()

				var ok bool
				gotRequestBody, ok = c.Get(requestBodyKey)
				if !ok {
					t.Fatalf("request body key %q was not set", requestBodyKey)
				}

				gotResponseBody, ok = c.Get(responseBodyKey)
				if !ok {
					t.Fatalf("response body key %q was not set", responseBodyKey)
				}
			})

			r.Use(CaptureBody())

			r.Handle(tt.method, "/test", func(c *gin.Context) {
				if c.Request.Body != nil {
					raw, err := io.ReadAll(c.Request.Body)
					if err != nil {
						t.Fatalf("handler failed to read request body: %v", err)
					}
					handlerSawBody = string(raw)
				}

				if tt.wantResponseBody == `plain-response` {
					c.String(http.StatusOK, tt.wantResponseBody)
					return
				}

				c.Data(http.StatusOK, "application/json", []byte(tt.wantResponseBody))
			})

			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, "/test", bytes.NewBufferString(tt.body))
			} else {
				req = httptest.NewRequest(tt.method, "/test", nil)
			}

			if tt.forceNilRequestBody {
				req.Body = nil
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
			}

			if handlerSawBody != tt.wantRequestBody {
				t.Fatalf("handlerSawBody = %q, want %q", handlerSawBody, tt.wantRequestBody)
			}

			if gotRequestBody != tt.wantRequestBody {
				t.Fatalf("request body captured = %#v, want %#v", gotRequestBody, tt.wantRequestBody)
			}

			if gotResponseBody != tt.wantResponseBody {
				t.Fatalf("response body captured = %#v, want %#v", gotResponseBody, tt.wantResponseBody)
			}
		})
	}
}

func TestDisableBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	ctxLogger := builder.New(c.Request.Context())
	c.Request = c.Request.WithContext(ctxLogger)

	if _, ok := c.Get(disableRequestBodyKey); ok {
		t.Fatalf("did not expect %q before calling DisableBody", disableRequestBodyKey)
	}
	if _, ok := c.Get(disableResponseBodyKey); ok {
		t.Fatalf("did not expect %q before calling DisableBody", disableResponseBodyKey)
	}

	DisableBody(c, true, false)

	gotRequest, ok := c.Get(disableRequestBodyKey)
	if !ok {
		t.Fatalf("expected %q to be set", disableRequestBodyKey)
	}
	disabledRequest, ok := gotRequest.(bool)
	if !ok {
		t.Fatalf("%q type = %T, want bool", disableRequestBodyKey, gotRequest)
	}
	if !disabledRequest {
		t.Fatalf("%q = %v, want true", disableRequestBodyKey, disabledRequest)
	}

	gotResponse, ok := c.Get(disableResponseBodyKey)
	if !ok {
		t.Fatalf("expected %q to be set", disableResponseBodyKey)
	}
	disabledResponse, ok := gotResponse.(bool)
	if !ok {
		t.Fatalf("%q type = %T, want bool", disableResponseBodyKey, gotResponse)
	}
	if disabledResponse {
		t.Fatalf("%q = %v, want false", disableResponseBodyKey, disabledResponse)
	}

	gotLoggerRequest, ok := ctxLogger.Get(string(disableRequestBodyKey))
	if !ok || gotLoggerRequest != true {
		t.Fatalf("logger %q = %#v, want true", disableRequestBodyKey, gotLoggerRequest)
	}
	gotLoggerResponse, ok := ctxLogger.Get(string(disableResponseBodyKey))
	if !ok || gotLoggerResponse != false {
		t.Fatalf("logger %q = %#v, want false", disableResponseBodyKey, gotLoggerResponse)
	}
}

func TestLoggerWithConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		enabled     bool
		path        string
		method      string
		requestBody string
		setupLog    func(*gin.Context)
	}{
		{
			name:        "info level",
			enabled:     true,
			path:        "/info",
			method:      http.MethodPost,
			requestBody: `{"kind":"info"}`,
			setupLog: func(c *gin.Context) {
				PrintInfo(c, "info message")
			},
		},
		{
			name:        "debug level",
			enabled:     true,
			path:        "/debug",
			method:      http.MethodPost,
			requestBody: `{"kind":"debug"}`,
			setupLog: func(c *gin.Context) {
				PrintDebug(c, "debug message")
			},
		},
		{
			name:        "warn level",
			enabled:     true,
			path:        "/warn",
			method:      http.MethodPost,
			requestBody: `{"kind":"warn"}`,
			setupLog: func(c *gin.Context) {
				PrintWarn(c, "warn message")
			},
		},
		{
			name:        "error level",
			enabled:     true,
			path:        "/error",
			method:      http.MethodPost,
			requestBody: `{"kind":"error"}`,
			setupLog: func(c *gin.Context) {
				PrintError(c, errors.New("boom"))
			},
		},
		{
			name:        "default branch without log level",
			enabled:     true,
			path:        "/default",
			method:      http.MethodPost,
			requestBody: `{"kind":"default"}`,
			setupLog:    nil,
		},
		{
			name:        "skip middleware when disabled",
			enabled:     false,
			path:        "/disabled",
			method:      http.MethodPost,
			requestBody: `{"kind":"disabled"}`,
			setupLog: func(c *gin.Context) {
				PrintInfo(c, "should be skipped")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viperdata.ResetViperDataSingleton()
			viper.Set(string(viperdata.GinLoggerWithConfigEnabledAtribute), tt.enabled)
			viper.Set(string(viperdata.GinLoggerWithConfigSkipPathsAtribute), []string{"/skip"})
			viper.Set(string(viperdata.GinLoggerWithConfigSkipQueryStringAtribute), false)

			r := gin.New()

			// Orden correcto:
			// LoggerWithConfig antes de CaptureBody para que el formatter
			// vea requestBody/responseBody ya seteados cuando regresa el flujo.
			r.Use(LoggerWithConfig())
			r.Use(CaptureBody())

			r.Handle(tt.method, tt.path, func(c *gin.Context) {
				if tt.setupLog != nil {
					tt.setupLog(c)
				}
				c.JSON(http.StatusOK, gin.H{"message": "Hello, World!"})
			})

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
			}

			if body := w.Body.String(); body != `{"message":"Hello, World!"}` {
				t.Fatalf("response body = %q, want %q", body, `{"message":"Hello, World!"}`)
			}
		})
	}
}

func TestLoggerWithConfig_BodyHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name                     string
		disableRequestBody       bool
		disableResponseBody      bool
		wantDisableRequestKey    bool
		wantDisableRequestValue  bool
		wantDisableResponseKey   bool
		wantDisableResponseValue bool
		wantRequest              any
		wantResponse             any
	}{
		{
			name:                   "bodies are added to details when disable flags are false",
			disableRequestBody:     false,
			disableResponseBody:    false,
			wantDisableRequestKey:  true,
			wantDisableResponseKey: true,
			wantRequest:            `{"kind":"info"}`,
			wantResponse:           `{"message":"Hello, World!"}`,
		},
		{
			name:                     "bodies are omitted when DisableBody is used",
			disableRequestBody:       true,
			disableResponseBody:      true,
			wantDisableRequestKey:    true,
			wantDisableRequestValue:  true,
			wantDisableResponseKey:   true,
			wantDisableResponseValue: true,
			wantRequest:              nil,
			wantResponse:             nil,
		},
		{
			name:                    "request and response bodies are controlled independently",
			disableRequestBody:      true,
			disableResponseBody:     false,
			wantDisableRequestKey:   true,
			wantDisableRequestValue: true,
			wantDisableResponseKey:  true,
			wantRequest:             nil,
			wantResponse:            `{"message":"Hello, World!"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viperdata.ResetViperDataSingleton()
			t.Cleanup(func() {
				viper.Reset()
				viperdata.ResetViperDataSingleton()
			})

			viper.Set(string(viperdata.AppAtribute), "test-service")
			viper.Set(string(viperdata.GinLoggerWithConfigEnabledAtribute), true)
			viper.Set(string(viperdata.GinLoggerWithConfigSkipPathsAtribute), []string{})
			viper.Set(string(viperdata.GinLoggerWithConfigSkipQueryStringAtribute), false)
			viper.Set(string(viperdata.LoggerIgnoredHeadersAtribute), []string{})
			viper.Set(string(viperdata.LoggerModeTestAtribute), false)

			var gotDisableRequestKey bool
			var gotDisableRequestValue bool
			var gotDisableResponseKey bool
			var gotDisableResponseValue bool
			var gotDetails formatter.KibanaData

			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Next()

				v, ok := c.Get(disableRequestBodyKey)
				gotDisableRequestKey = ok
				if ok {
					boolValue, typeOK := v.(bool)
					if !typeOK {
						t.Fatalf("%q type = %T, want bool", disableRequestBodyKey, v)
					}
					gotDisableRequestValue = boolValue
				}

				v, ok = c.Get(disableResponseBodyKey)
				gotDisableResponseKey = ok
				if ok {
					boolValue, typeOK := v.(bool)
					if !typeOK {
						t.Fatalf("%q type = %T, want bool", disableResponseBodyKey, v)
					}
					gotDisableResponseValue = boolValue
				}

				ctxLogger := builder.New(c.Request.Context())
				detailsAny, ok := ctxLogger.Get(detailsKey)
				if !ok {
					t.Fatalf("expected %q in logger context", detailsKey)
				}

				var castOK bool
				gotDetails, castOK = detailsAny.(formatter.KibanaData)
				if !castOK {
					t.Fatalf("%q type = %T, want formatter.KibanaData", detailsKey, detailsAny)
				}
			})
			r.Use(InitLogger())
			r.Use(LoggerWithConfig())
			r.Use(CaptureBody())

			r.POST("/test", func(c *gin.Context) {
				DisableBody(c, tt.disableRequestBody, tt.disableResponseBody)

				PrintInfo(c, "info message")
				c.JSON(http.StatusOK, gin.H{"message": "Hello, World!"})
			})

			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(`{"kind":"info"}`))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
			}

			if !gotDisableRequestKey && tt.wantDisableRequestKey {
				t.Fatalf("expected %q to be present", disableRequestBodyKey)
			}
			if gotDisableRequestKey != tt.wantDisableRequestKey {
				t.Fatalf("%q presence = %v, want %v", disableRequestBodyKey, gotDisableRequestKey, tt.wantDisableRequestKey)
			}
			if gotDisableRequestValue != tt.wantDisableRequestValue {
				t.Fatalf("%q value = %v, want %v", disableRequestBodyKey, gotDisableRequestValue, tt.wantDisableRequestValue)
			}
			if !gotDisableResponseKey && tt.wantDisableResponseKey {
				t.Fatalf("expected %q to be present", disableResponseBodyKey)
			}
			if gotDisableResponseKey != tt.wantDisableResponseKey {
				t.Fatalf("%q presence = %v, want %v", disableResponseBodyKey, gotDisableResponseKey, tt.wantDisableResponseKey)
			}
			if gotDisableResponseValue != tt.wantDisableResponseValue {
				t.Fatalf("%q value = %v, want %v", disableResponseBodyKey, gotDisableResponseValue, tt.wantDisableResponseValue)
			}
			if gotDetails.Request != tt.wantRequest {
				t.Fatalf("details.request = %#v, want %#v", gotDetails.Request, tt.wantRequest)
			}
			if gotDetails.Response != tt.wantResponse {
				t.Fatalf("details.response = %#v, want %#v", gotDetails.Response, tt.wantResponse)
			}
		})
	}
}

func TestPrintInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)

	PrintInfo(ctx, "info message")

	method, ok := ctx.Get(methodKey)
	if !ok {
		t.Fatalf("expected %q to be set", methodKey)
	}
	methodValue, ok := method.(string)
	if !ok {
		t.Fatalf("%q type = %T, want string", methodKey, method)
	}
	if !strings.HasSuffix(methodValue, "PrintInfo") {
		t.Fatalf("%q = %q, want suffix %q", methodKey, methodValue, "PrintInfo")
	}

	line, ok := ctx.Get(lineKey)
	if !ok {
		t.Fatalf("expected %q to be set", lineKey)
	}
	lineValue, ok := line.(int)
	if !ok {
		t.Fatalf("%q type = %T, want int", lineKey, line)
	}
	if lineValue <= 0 {
		t.Fatalf("%q = %d, want > 0", lineKey, lineValue)
	}

	level, ok := ctx.Get(formatter.InfoLevel)
	if !ok {
		t.Fatalf("expected %q to be set", formatter.InfoLevel)
	}
	if level != "info message" {
		t.Fatalf("%q = %#v, want %#v", formatter.InfoLevel, level, "info message")
	}
}

func TestPrintDebug(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)

	PrintDebug(ctx, "debug message")

	method, ok := ctx.Get(methodKey)
	if !ok {
		t.Fatalf("expected %q to be set", methodKey)
	}
	methodValue, ok := method.(string)
	if !ok {
		t.Fatalf("%q type = %T, want string", methodKey, method)
	}
	if !strings.HasSuffix(methodValue, "PrintDebug") {
		t.Fatalf("%q = %q, want suffix %q", methodKey, methodValue, "PrintDebug")
	}

	line, ok := ctx.Get(lineKey)
	if !ok {
		t.Fatalf("expected %q to be set", lineKey)
	}
	lineValue, ok := line.(int)
	if !ok {
		t.Fatalf("%q type = %T, want int", lineKey, line)
	}
	if lineValue <= 0 {
		t.Fatalf("%q = %d, want > 0", lineKey, lineValue)
	}

	level, ok := ctx.Get(formatter.DebugLevel)
	if !ok {
		t.Fatalf("expected %q to be set", formatter.DebugLevel)
	}
	if level != "debug message" {
		t.Fatalf("%q = %#v, want %#v", formatter.DebugLevel, level, "debug message")
	}
}

func TestPrintWarn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)

	PrintWarn(ctx, "warn message")

	method, ok := ctx.Get(methodKey)
	if !ok {
		t.Fatalf("expected %q to be set", methodKey)
	}
	methodValue, ok := method.(string)
	if !ok {
		t.Fatalf("%q type = %T, want string", methodKey, method)
	}
	if !strings.HasSuffix(methodValue, "PrintWarn") {
		t.Fatalf("%q = %q, want suffix %q", methodKey, methodValue, "PrintWarn")
	}

	line, ok := ctx.Get(lineKey)
	if !ok {
		t.Fatalf("expected %q to be set", lineKey)
	}
	lineValue, ok := line.(int)
	if !ok {
		t.Fatalf("%q type = %T, want int", lineKey, line)
	}
	if lineValue <= 0 {
		t.Fatalf("%q = %d, want > 0", lineKey, lineValue)
	}

	level, ok := ctx.Get(formatter.WarnLevel)
	if !ok {
		t.Fatalf("expected %q to be set", formatter.WarnLevel)
	}
	if level != "warn message" {
		t.Fatalf("%q = %#v, want %#v", formatter.WarnLevel, level, "warn message")
	}
}

func TestPrintError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	wantErr := fmt.Errorf("boom")

	PrintError(ctx, wantErr)

	method, ok := ctx.Get(methodKey)
	if !ok {
		t.Fatalf("expected %q to be set", methodKey)
	}
	methodValue, ok := method.(string)
	if !ok {
		t.Fatalf("%q type = %T, want string", methodKey, method)
	}
	if !strings.HasSuffix(methodValue, "PrintError") {
		t.Fatalf("%q = %q, want suffix %q", methodKey, methodValue, "PrintError")
	}

	line, ok := ctx.Get(lineKey)
	if !ok {
		t.Fatalf("expected %q to be set", lineKey)
	}
	lineValue, ok := line.(int)
	if !ok {
		t.Fatalf("%q type = %T, want int", lineKey, line)
	}
	if lineValue <= 0 {
		t.Fatalf("%q = %d, want > 0", lineKey, lineValue)
	}

	level, ok := ctx.Get(formatter.ErrorLevel)
	if !ok {
		t.Fatalf("expected %q to be set", formatter.ErrorLevel)
	}
	gotErr, ok := level.(error)
	if !ok {
		t.Fatalf("%q type = %T, want error", formatter.ErrorLevel, level)
	}
	if gotErr != wantErr {
		t.Fatalf("%q = %#v, want %#v", formatter.ErrorLevel, gotErr, wantErr)
	}
}
