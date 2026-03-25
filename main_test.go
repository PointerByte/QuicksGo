// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PointerByte/QuicksGo/security/middlewares"
	"github.com/gin-gonic/gin"
)

func TestMainRegistersHelloRouteAndStartsServer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prevCreateApp := createAppFn
	prevGetRoute := getRouteFn
	prevStart := startFn
	t.Cleanup(func() {
		createAppFn = prevCreateApp
		getRouteFn = prevGetRoute
		startFn = prevStart
	})

	engine := gin.New()
	group := engine.Group("/api/v1")
	srv := &http.Server{Handler: engine}
	startCalled := false

	createAppFn = func(optionsJWT ...middlewares.JWTMiddlewareOption) (*http.Server, error) {
		return srv, nil
	}
	getRouteFn = func(string) *gin.RouterGroup {
		return group
	}
	startFn = func(input *http.Server) {
		startCalled = true
		if input != srv {
			t.Fatalf("expected server %p, got %p", srv, input)
		}
	}

	main()

	if !startCalled {
		t.Fatal("expected startFn to be called")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/hello", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if body := rec.Body.String(); body != "{\"message\":\"ok\"}" {
		t.Fatalf("unexpected response body %s", body)
	}
}

func TestMainPanicsWhenCreateAppFails(t *testing.T) {
	prevCreateApp := createAppFn
	prevGetRoute := getRouteFn
	prevStart := startFn
	t.Cleanup(func() {
		createAppFn = prevCreateApp
		getRouteFn = prevGetRoute
		startFn = prevStart
	})

	expectedErr := errors.New("create app failed")
	createAppFn = func(optionsJWT ...middlewares.JWTMiddlewareOption) (*http.Server, error) {
		return nil, expectedErr
	}
	getRouteFn = func(string) *gin.RouterGroup {
		t.Fatal("did not expect getRouteFn to be called")
		return nil
	}
	startFn = func(*http.Server) {
		t.Fatal("did not expect startFn to be called")
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected panic")
		}
		err, ok := recovered.(error)
		if !ok {
			t.Fatalf("expected panic error, got %T", recovered)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected panic %v, got %v", expectedErr, err)
		}
	}()

	main()
}
