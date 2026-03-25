// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

// Package main is the root entry point of QuicksGo.
//
// It represents the framework shell that groups the main QuicksGo building
// blocks: configuration loading, HTTP and gRPC bootstrap, structured logging,
// OpenTelemetry tracing, and JWT-based security helpers.
package main

import (
	"net/http"

	serverGin "github.com/PointerByte/QuicksGo/config/server/gin"
	"github.com/PointerByte/QuicksGo/security/middlewares"
	"github.com/gin-gonic/gin"
)

var (
	createAppFn = func(optionsJWT ...middlewares.JWTMiddlewareOption) (*http.Server, error) {
		return serverGin.CreateApp(optionsJWT...)
	}
	getRouteFn = serverGin.GetRoute
	startFn    = serverGin.Start
)

func main() {
	srv, err := createAppFn()
	if err != nil {
		panic(err)
	}

	api := getRouteFn("/api/v1")
	api.GET("/hello", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	startFn(srv)
}
