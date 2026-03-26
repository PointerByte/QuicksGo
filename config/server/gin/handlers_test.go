// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package gin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	clientHttp "github.com/PointerByte/QuicksGo/config/client/http"
	"github.com/PointerByte/QuicksGo/logger/builder"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func resetHandlersTestState(t *testing.T) {
	t.Helper()

	resetServerTestState(t)

	origFunctionsRefresh := functionsRefresh
	origHosts := hosts
	origGetGeneric := getGeneric
	origRestartJobs := restartJobs

	functionsRefresh = nil
	hosts = nil

	t.Cleanup(func() {
		functionsRefresh = origFunctionsRefresh
		hosts = origHosts
		getGeneric = origGetGeneric
		restartJobs = origRestartJobs
	})
}

func TestSetFunctionsRefresh(t *testing.T) {
	resetHandlersTestState(t)

	fn1 := func() error { return nil }
	fn2 := func() error { return nil }

	SetFunctionsRefresh(fn1)
	SetFunctionsRefresh(fn2)

	if len(functionsRefresh) != 2 {
		t.Fatalf("expected 2 refresh functions, got %d", len(functionsRefresh))
	}
	if reflect.ValueOf(functionsRefresh[0]).Pointer() != reflect.ValueOf(fn1).Pointer() {
		t.Fatal("expected first function to be preserved")
	}
	if reflect.ValueOf(functionsRefresh[1]).Pointer() != reflect.ValueOf(fn2).Pointer() {
		t.Fatal("expected second function to be appended")
	}
}

func TestSetHostsRefresh(t *testing.T) {
	resetHandlersTestState(t)

	SetHostsRefresh("10.0.0.1", "10.0.0.2")
	SetHostsRefresh("10.0.0.3")

	want := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}
	if !reflect.DeepEqual(hosts, want) {
		t.Fatalf("expected hosts %v, got %v", want, hosts)
	}
}

func TestNotFound(t *testing.T) {
	resetHandlersTestState(t)

	router := gin.New()
	router.NoRoute(notFound())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["message"] != "Path not found" {
		t.Fatalf("unexpected message: %q", body["message"])
	}
}

func TestNoMethod(t *testing.T) {
	resetHandlersTestState(t)

	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.NoMethod(noMethod())
	router.GET("/resource", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/resource", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["message"] != "Method not allow" {
		t.Fatalf("unexpected message: %q", body["message"])
	}
}

func TestRefreshGinBroadcastAlreadyUpdated(t *testing.T) {
	resetHandlersTestState(t)

	var calls int32
	var restartCalls int32
	getGeneric = func(context.Context, clientHttp.RequestGeneric) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}
	restartJobs = func() {
		atomic.AddInt32(&restartCalls, 1)
	}

	router := gin.New()
	router.GET("/api/v1/refresh", refresh())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/refresh", nil)
	req.Header.Set("broadcast-refresh", "true")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if atomic.LoadInt32(&calls) != 0 {
		t.Fatalf("expected no outbound refresh calls, got %d", calls)
	}
	if atomic.LoadInt32(&restartCalls) != 0 {
		t.Fatalf("expected no restart calls, got %d", restartCalls)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["action"] != "Tasks have been updated" {
		t.Fatalf("unexpected action: %q", body["action"])
	}
}

func TestRefreshGin(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		resetHandlersTestState(t)
		hosts = []string{"host-a", "host-b"}
		viper.Set("server.gin.port", ":8080")

		var (
			mu     sync.Mutex
			inputs []clientHttp.RequestGeneric
		)
		var restartCalls int32
		getGeneric = func(_ context.Context, input clientHttp.RequestGeneric) error {
			mu.Lock()
			inputs = append(inputs, input)
			mu.Unlock()
			return nil
		}
		restartJobs = func() {
			atomic.AddInt32(&restartCalls, 1)
		}

		router := gin.New()
		router.GET("/api/v1/refresh", refresh())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/refresh", nil)
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if body["mensaje"] != "All tasks have been updated" {
			t.Fatalf("unexpected message: %q", body["mensaje"])
		}

		if len(inputs) != 2 {
			t.Fatalf("expected 2 outbound requests, got %d", len(inputs))
		}
		if atomic.LoadInt32(&restartCalls) != 1 {
			t.Fatalf("expected 1 restart call, got %d", restartCalls)
		}
		for _, input := range inputs {
			if input.Url != "http://host-a:8080/api/v1/refresh" && input.Url != "http://host-b:8080/api/v1/refresh" {
				t.Fatalf("unexpected outbound url: %q", input.Url)
			}
			if got := input.Header.Get("broadcast-refresh"); got != "true" {
				t.Fatalf("expected broadcast-refresh header, got %q", got)
			}
			if got := input.Header.Get("Authorization"); got != "Bearer token" {
				t.Fatalf("expected Authorization header to propagate, got %q", got)
			}
			if input.System != "Refresh" {
				t.Fatalf("unexpected system: %q", input.System)
			}
		}
	})

	t.Run("error", func(t *testing.T) {
		resetHandlersTestState(t)
		hosts = []string{"host-a"}
		viper.Set("server.gin.port", ":8080")

		wantErr := errors.New("boom")
		var restartCalls int32
		getGeneric = func(context.Context, clientHttp.RequestGeneric) error {
			return wantErr
		}
		restartJobs = func() {
			atomic.AddInt32(&restartCalls, 1)
		}

		router := gin.New()
		router.GET("/api/v1/refresh", refresh())

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/refresh", nil))

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
		if atomic.LoadInt32(&restartCalls) != 1 {
			t.Fatalf("expected 1 restart call, got %d", restartCalls)
		}

		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if body["mensaje"] != "Error retrieving the hosts from tasks" {
			t.Fatalf("unexpected message: %q", body["mensaje"])
		}
	})

	t.Run("refresh function error", func(t *testing.T) {
		resetHandlersTestState(t)
		hosts = []string{"host-a"}
		viper.Set("server.gin.port", ":8080")

		wantErr := errors.New("refresh function failed")
		SetFunctionsRefresh(func() error {
			return wantErr
		})

		var outboundCalls int32
		var restartCalls int32
		getGeneric = func(context.Context, clientHttp.RequestGeneric) error {
			atomic.AddInt32(&outboundCalls, 1)
			return nil
		}
		restartJobs = func() {
			atomic.AddInt32(&restartCalls, 1)
		}

		router := gin.New()
		router.GET("/api/v1/refresh", refresh())

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/refresh", nil))

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
		if atomic.LoadInt32(&restartCalls) != 1 {
			t.Fatalf("expected 1 restart call, got %d", restartCalls)
		}
		if atomic.LoadInt32(&outboundCalls) != 0 {
			t.Fatalf("expected no outbound calls when refresh function fails, got %d", outboundCalls)
		}

		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if body["mensaje"] != "Error to refresh start functions" {
			t.Fatalf("unexpected message: %q", body["mensaje"])
		}
	})
}

func TestSendRefreshToTask(t *testing.T) {
	t.Run("no hosts", func(t *testing.T) {
		resetHandlersTestState(t)
		viper.Set("server.gin.port", ":8080")

		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/refresh", nil)

		if err := sendRefreshToTask(c, builder.New(context.Background()), "/api/v1"); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("returns first non nil error", func(t *testing.T) {
		resetHandlersTestState(t)
		hosts = []string{"host-a", "host-b"}
		viper.Set("server.gin.port", ":8080")

		wantErr := errors.New("outbound error")
		getGeneric = func(_ context.Context, input clientHttp.RequestGeneric) error {
			if input.Url == "http://host-b:8080/api/v1/refresh" {
				return wantErr
			}
			return nil
		}

		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/refresh", nil)
		c.Request.Header.Set("X-Request-ID", "123")

		err := sendRefreshToTask(c, builder.New(context.Background()), "/api/v1")
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected %v, got %v", wantErr, err)
		}
	})
}

func TestGetGeneric(t *testing.T) {
	resetHandlersTestState(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	var response any
	err := getGeneric(context.Background(), clientHttp.RequestGeneric{
		System:   "Refresh",
		Process:  "All tasks are updated",
		Url:      srv.URL,
		Response: &response,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
