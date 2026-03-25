// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

// Package gin provides the HTTP bootstrap layer used by QuicksGo services.
//
// It is responsible for turning the framework configuration into a runnable Gin
// server with shared middleware, route groups, observability setup, refresh and
// health handlers, and graceful shutdown coordination.
//
// In a typical application flow this package:
//   - loads configuration through config/utilities
//   - initializes logger and OpenTelemetry
//   - creates the shared gin.Engine
//   - registers the common middleware stack
//   - creates the route groups declared in server.groups
//   - builds the final *http.Server returned by CreateApp
//
// Main entry points:
//   - CreateApp to initialize configuration and build the HTTP server
//   - GetEngine to access the shared gin.Engine after CreateApp
//   - GetRoute to obtain one of the configured route groups
//   - Start to run the HTTP server and coordinate shutdown
//
// The package also exposes helpers such as SetHostsRefresh and
// SetFunctionsRefresh, which are used by the built-in refresh propagation flow.
//
// Complete example from a main package:
//
//	package main
//
//	import (
//		"log"
//
//		servergin "github.com/PointerByte/QuicksGo/config/server/gin"
//		"github.com/gin-gonic/gin"
//	)
//
//	func main() {
//		srv, err := servergin.CreateApp()
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		api := servergin.GetRoute("/api/v1")
//		if api == nil {
//			log.Fatal("route group /api/v1 is not configured in server.groups")
//		}
//
//		api.GET("/hello", func(c *gin.Context) {
//			c.JSON(200, gin.H{
//				"message": "ok",
//			})
//		})
//
//		servergin.Start(srv)
//	}
//
// In that example, `/api/v1` must exist in the `server.groups` configuration.
// Once CreateApp succeeds, the package will also register `/health` and
// `/refresh` under each configured route group.
package gin
