// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/PointerByte/QuicksGo/logger/formatter"
	viperdata "github.com/PointerByte/QuicksGo/logger/viperData"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Context is a custom context similar to gin.Context.
type Context struct {
	context.Context // hereda Cancel, Deadline, Done, Value
	mux             sync.Mutex
	startTime       time.Time
	fields          map[any]any
	disableTrace    bool
	tracer          trace.Tracer // Trace from telemetry
	Method          string
	Line            int
	Details         formatter.KibanaData
}

// DisableTrace disables trace logging for a specific process or trace.
func (c *Context) DisableTrace() {
	c.disableTrace = true
}

// private key to store the *logger.Context* within context.Context.
type ctxKey int

const loggerCtxKey ctxKey = iota

// From attempts to extract a *logger.Context* from a context.Context.
// Returns (*Context, true) if it exists; otherwise (nil, false).
func From(parent context.Context) (*Context, bool) {
	if parent == nil {
		return nil, false
	}
	// Case 1: The parent is already a *logger.Context
	if lc, ok := parent.(*Context); ok && lc != nil {
		return lc, true
	}
	// Case 2: The parent has saved the *logger.Context* in the values
	if v := parent.Value(loggerCtxKey); v != nil {
		if lc, ok := v.(*Context); ok && lc != nil {
			return lc, true
		}
	}
	return nil, false
}

// New creates or reuses a *logger.Context*.
//
//   - If the parent context already contains a *logger.Context*, it reuses it.
//   - Otherwise, it creates a new one, initializes the traceID and the list of services,
//     and saves it in the context values for later use.
func New(parent context.Context) *Context {
	if parent == nil {
		parent = context.Background()
	}

	// Reuse the context if it already exists
	if existing, ok := From(parent); ok {
		return existing
	}

	appName := viperdata.GetViperData(string(viperdata.AppAtribute)).(string)
	newContext := &Context{
		Context:   parent,
		startTime: time.Now(),
		fields:    make(map[any]any),
		tracer:    otel.Tracer(appName),
	}

	// Initialize traceID
	traceID := strings.ReplaceAll(uuid.NewString(), "-", "")
	newContext.Set(traceIDKey, traceID)

	// Initialize service collection
	services := make([]formatter.Service, 0)
	newContext.Set(servicesKey, &services)

	// Save the *logger.Context* within the context for future retrieval
	newContext.Context = context.WithValue(parent, loggerCtxKey, newContext)
	return newContext
}

// Set adds a key-value pair to the context.
func (c *Context) Set(key any, value any) {
	c.fields[key] = value
}

// Get retrieves a value from the context.
func (c *Context) Get(key any) (any, bool) {
	v, ok := c.fields[key]
	return v, ok
}

// MustGet returns a value or throws an exception if one does not exist.
func (c *Context) MustGet(key string) any {
	if v, ok := c.Get(key); ok {
		return v
	}
	panic(fmt.Sprintf("logger.Context: key '%s' not found", key))
}

// GetLatency returns the elapsed time since the context was created.
// It measures how long the operation associated with this context has been running.
func (c *Context) GetLatency() int64 {
	return time.Since(c.startTime).Milliseconds()
}

// WithCancel creates a copy of logger.Context with support for manual cancellation.
func (c *Context) WithCancel() (*Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(c.Context)
	newCtx := &Context{
		Context:      ctx,
		startTime:    c.startTime,
		fields:       c.fields,
		tracer:       c.tracer,
		disableTrace: c.disableTrace,
		Method:       c.Method,
		Line:         c.Line,
	}
	return newCtx, cancel
}

// WithTimeout creates a copy of logger.Context that automatically expires
// after the specified duration.
func (c *Context) WithTimeout(d time.Duration) (*Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(c.Context, d)
	newCtx := &Context{
		Context:      ctx,
		startTime:    c.startTime,
		fields:       c.fields,
		tracer:       c.tracer,
		disableTrace: c.disableTrace,
		Method:       c.Method,
		Line:         c.Line,
	}
	return newCtx, cancel
}
