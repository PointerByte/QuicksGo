// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package server_Gin

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PointerByte/QuicksGo/config/clientHttp"
	"github.com/PointerByte/QuicksGo/config/utilities/jobs"
	"github.com/PointerByte/QuicksGo/logger/builder"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

type HandlerFunctionsRefresh func(ctx context.Context) error

var functionsRefresh []HandlerFunctionsRefresh

// SetFunctionsRefresh registers refresh callbacks used by the intra-service
// synchronization flow.
func SetFunctionsRefresh(input ...HandlerFunctionsRefresh) {
	functionsRefresh = append(functionsRefresh, input...)
}

var hosts []string

// SetHostsRefresh appends the remote hosts that should receive a refresh
// notification when this node propagates an update.
func SetHostsRefresh(input ...string) {
	hosts = append(hosts, input...)
}

func notFound() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"message": "Path not found",
		})
	}
}

func noMethod() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"message": "Method not allow",
		})
	}
}

var restartJobs = jobs.RestartJobs

// refreshGin builds the `/refresh` endpoint handler.
//
// When the request already carries the broadcast marker it responds without
// forwarding the call again, preventing loops between nodes. Otherwise it
// restarts package-level jobs, propagates the refresh to the configured hosts,
// and returns the consolidated result.
//
// The handler is registered automatically under each route group created by
// `server_Gin.CreateApp()`, so for a group like `/api/v1` the resulting path is
// `/api/v1/refresh`.
func refreshGin() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ctxLogger := builder.New(ctx)
		flag := c.Request.Header.Get("broadcast-refresh")

		if flag == "true" {
			ctxLogger.Info("Se valida que la tarea ya fue actualizada")
			c.JSON(http.StatusOK, gin.H{
				"action":  "Tasks have been updated",
				"mensaje": "It is verified that the task has already been updated",
			})
			return
		}

		restartJobs()
		basePath := strings.TrimSuffix(c.FullPath(), "/refresh")
		if err := sendRefreshToTask(c, ctxLogger, basePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"action":  "Internal server error; please check",
				"mensaje": "Error retrieving the IP addresses of the tasks",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"action":  "The hosts have been updated",
			"mensaje": "All tasks have been updated",
		})
	}
}

// getGeneric wraps the HTTP client used to notify the refresh endpoint on
// remote hosts and can be replaced in tests.
var getGeneric = func(ctx context.Context, input clientHttp.RequestGeneric) error {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	newRestGeneric := clientHttp.NewGenericRest(nil, time.Minute, tr)
	return newRestGeneric.GetGeneric(ctx, input)
}

// sendRefreshToTask forwards the refresh request to all registered hosts in
// parallel, preserving the original headers and adding
// `broadcast-refresh=true` to avoid infinite fan-out loops.
//
// The propagated target URL follows the same route group as the incoming
// request, ending with `/refresh` on each configured host.
func sendRefreshToTask(c *gin.Context, ctxLogger *builder.Context, basePath string) error {
	port := viper.GetString("server.gin.port")
	var wg sync.WaitGroup
	errChan := make(chan error, len(hosts))

	for _, t := range hosts {
		wg.Add(1)
		go func(host string) {
			defer wg.Done()
			headers := http.Header{
				"Content-Type":      []string{"application/json"},
				"Broadcast-Refresh": []string{"true"},
			}
			for k, vals := range c.Request.Header {
				for _, v := range vals {
					headers.Add(k, v)
				}
			}
			var resp any
			url := fmt.Sprintf("http://%s%s%s/refresh", host, port, basePath)
			errChan <- getGeneric(ctxLogger, clientHttp.RequestGeneric{
				System:   "Refresh",
				Process:  "All tasks are updated",
				Url:      url,
				Header:   headers,
				Response: &resp,
			})
		}(t)
	}
	wg.Wait()

	close(errChan)
	for err := range errChan {
		if err == nil {
			continue
		}
		return err
	}
	return nil
}
